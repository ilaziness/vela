package services

import (
	"context"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/ilaziness/vela/internal/ai"
	"github.com/ilaziness/vela/internal/database"
)

func openAITestDB(t *testing.T) *database.Database {
	t.Helper()
	db := database.NewDatabase(t.TempDir(), zap.NewNop())
	if err := db.Initialize(); err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func TestTurnPersisterStreamCheckpointFinalizeSameIDs(t *testing.T) {
	db := openAITestDB(t)
	repo := db.AISessionRepo
	ctx := context.Background()

	sess := &database.AISession{Title: "t"}
	if err := repo.CreateSession(ctx, sess); err != nil {
		t.Fatal(err)
	}
	user := &database.AIMessage{
		SessionID: sess.ID, Role: ai.RoleUser, Content: "q", Parts: ai.UserTextPartsJSON("q"), RunID: "run-x",
	}
	if err := repo.CreateMessage(ctx, user); err != nil {
		t.Fatal(err)
	}

	p := newTurnPersister(repo, zap.NewNop(), sess.ID, "run-x", user.ID)
	p.UpsertStreamingText("hel")
	p.UpsertStreamingText("lo")
	// Force flush path via Checkpoint after stream was written.
	time.Sleep(streamFlushInterval + 20*time.Millisecond)
	p.UpsertStreamingText("!")

	mid, err := repo.ListMessagesByRunID(ctx, sess.ID, "run-x")
	if err != nil {
		t.Fatal(err)
	}
	var modelBefore string
	for _, m := range mid {
		if m.Role == ai.RoleModel {
			modelBefore = m.ID
			if m.Content != "hello!" {
				t.Fatalf("stream content = %q", m.Content)
			}
		}
	}
	if modelBefore == "" {
		t.Fatal("expected provisional model row")
	}

	p.Checkpoint([]ai.ChatMessage{{
		Role: ai.RoleModel, Content: "hello!", Parts: ai.UserTextPartsJSON("hello!"),
	}})
	afterCP, err := repo.ListMessagesByRunID(ctx, sess.ID, "run-x")
	if err != nil {
		t.Fatal(err)
	}
	var cpModelID string
	for _, m := range afterCP {
		if m.Role == ai.RoleModel {
			cpModelID = m.ID
		}
	}
	if cpModelID == "" {
		t.Fatal("expected checkpoint model")
	}

	last, err := p.Finalize(&ai.AgentResponse{
		Turn:    []ai.ChatMessage{{Role: ai.RoleModel, Content: "hello!", Parts: ai.UserTextPartsJSON("hello!")}},
		Content: "hello!",
	}, false)
	if err != nil {
		t.Fatal(err)
	}
	if last == nil || last.ID != cpModelID {
		t.Fatalf("finalize should reuse checkpoint id, last=%+v cp=%s", last, cpModelID)
	}
	all, err := repo.ListMessages(ctx, sess.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range all {
		if m.RunID != "" {
			t.Fatalf("expected sealed, got %+v", m)
		}
	}
	// Second finalize is no-op.
	if _, err := p.Finalize(&ai.AgentResponse{Turn: []ai.ChatMessage{{Role: ai.RoleModel, Content: "x", Parts: "[]"}}}, false); err != nil {
		t.Fatal(err)
	}
	all2, _ := repo.ListMessages(ctx, sess.ID)
	models := 0
	for _, m := range all2 {
		if m.Role == ai.RoleModel {
			models++
			if m.Content != "hello!" {
				t.Fatalf("duplicate finalize changed content: %q", m.Content)
			}
		}
	}
	if models != 1 {
		t.Fatalf("want 1 model, got %d", models)
	}
}

func TestTurnPersisterFinalizeMergesStreamWhenRespEmpty(t *testing.T) {
	db := openAITestDB(t)
	repo := db.AISessionRepo
	ctx := context.Background()

	sess := &database.AISession{Title: "t"}
	if err := repo.CreateSession(ctx, sess); err != nil {
		t.Fatal(err)
	}
	user := &database.AIMessage{
		SessionID: sess.ID, Role: ai.RoleUser, Content: "q", Parts: "[]", RunID: "run-merge",
	}
	if err := repo.CreateMessage(ctx, user); err != nil {
		t.Fatal(err)
	}
	p := newTurnPersister(repo, zap.NewNop(), sess.ID, "run-merge", user.ID)
	p.UpsertStreamingText("only-stream")
	last, err := p.Finalize(nil, true)
	if err != nil {
		t.Fatal(err)
	}
	if last == nil || last.Content != "only-stream" {
		t.Fatalf("expected merged stream, got %+v", last)
	}
	all, _ := repo.ListMessages(ctx, sess.ID)
	for _, m := range all {
		if m.RunID != "" {
			t.Fatalf("unsealed: %+v", m)
		}
	}
}

func TestMergeStreamSegmentAppendsAfterCheckpoint(t *testing.T) {
	resp := &ai.AgentResponse{
		Turn:    []ai.ChatMessage{{Role: ai.RoleModel, Content: "hello", Parts: ai.UserTextPartsJSON("hello")}},
		Content: "hello",
	}
	got := mergeStreamSegment(resp, " world")
	if got.Turn[0].Content != "hello world" {
		t.Fatalf("got %q", got.Turn[0].Content)
	}
	// Dedupe when segment already applied.
	got2 := mergeStreamSegment(got, " world")
	if got2.Turn[0].Content != "hello world" {
		t.Fatalf("dedupe failed: %q", got2.Turn[0].Content)
	}
	// Full segment supersedes prefix.
	got3 := mergeStreamSegment(&ai.AgentResponse{
		Turn: []ai.ChatMessage{{Role: ai.RoleModel, Content: "hi", Parts: ai.UserTextPartsJSON("hi")}},
	}, "hi there")
	if got3.Turn[0].Content != "hi there" {
		t.Fatalf("prefix replace: %q", got3.Turn[0].Content)
	}
}

func TestFlushExtendsCheckpointModelNotSplit(t *testing.T) {
	db := openAITestDB(t)
	repo := db.AISessionRepo
	ctx := context.Background()

	sess := &database.AISession{Title: "t"}
	if err := repo.CreateSession(ctx, sess); err != nil {
		t.Fatal(err)
	}
	user := &database.AIMessage{
		SessionID: sess.ID, Role: ai.RoleUser, Content: "q", Parts: "[]", RunID: "run-ext",
	}
	if err := repo.CreateMessage(ctx, user); err != nil {
		t.Fatal(err)
	}
	p := newTurnPersister(repo, zap.NewNop(), sess.ID, "run-ext", user.ID)
	p.Checkpoint([]ai.ChatMessage{{
		Role: ai.RoleModel, Content: "hello", Parts: ai.UserTextPartsJSON("hello"),
	}})
	p.UpsertStreamingText(" world")
	msgs, err := repo.ListMessagesByRunID(ctx, sess.ID, "run-ext")
	if err != nil {
		t.Fatal(err)
	}
	models := 0
	for _, m := range msgs {
		if m.Role == ai.RoleModel {
			models++
			if m.Content != "hello world" {
				t.Fatalf("want extended single model, got %q", m.Content)
			}
		}
	}
	if models != 1 {
		t.Fatalf("want 1 model row, got %d", models)
	}
	last, err := p.Finalize(&ai.AgentResponse{
		Turn: []ai.ChatMessage{{Role: ai.RoleModel, Content: "hello", Parts: ai.UserTextPartsJSON("hello")}},
	}, true)
	if err != nil {
		t.Fatal(err)
	}
	if last.Content != "hello world" {
		t.Fatalf("finalize merge: %q", last.Content)
	}
}

func TestCheckpointEmptyDoesNotWipeTurn(t *testing.T) {
	db := openAITestDB(t)
	repo := db.AISessionRepo
	ctx := context.Background()

	sess := &database.AISession{Title: "t"}
	if err := repo.CreateSession(ctx, sess); err != nil {
		t.Fatal(err)
	}
	user := &database.AIMessage{
		SessionID: sess.ID, Role: ai.RoleUser, Content: "q", Parts: "[]", RunID: "run-empty-cp",
	}
	if err := repo.CreateMessage(ctx, user); err != nil {
		t.Fatal(err)
	}
	p := newTurnPersister(repo, zap.NewNop(), sess.ID, "run-empty-cp", user.ID)
	p.Checkpoint([]ai.ChatMessage{{
		Role: ai.RoleModel, Content: "keep", Parts: ai.UserTextPartsJSON("keep"),
	}})
	p.Checkpoint(nil)
	p.Checkpoint([]ai.ChatMessage{{Role: ai.RoleUser, Content: "leak", Parts: "[]"}})
	msgs, err := repo.ListMessagesByRunID(ctx, sess.ID, "run-empty-cp")
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, m := range msgs {
		if m.Role == ai.RoleModel && m.Content == "keep" {
			found = true
		}
		if m.Role == ai.RoleUser && m.Content == "leak" {
			t.Fatal("user must not be written by checkpoint")
		}
	}
	if !found {
		t.Fatal("empty checkpoint wiped prior turn")
	}
}

func TestMergeErrorIntoResponseNoDup(t *testing.T) {
	msg := "boom"
	resp := &ai.AgentResponse{
		Turn: []ai.ChatMessage{{Role: ai.RoleModel, Content: msg, Parts: ai.UserTextPartsJSON(msg)}},
	}
	got := mergeErrorIntoResponse(resp, msg)
	if len(got.Turn) != 1 {
		t.Fatalf("want 1 turn, got %d", len(got.Turn))
	}
}

func TestForceFinalizeSessionRunStealsHungRun(t *testing.T) {
	db := openAITestDB(t)
	repo := db.AISessionRepo
	ctx := context.Background()

	sess := &database.AISession{Title: "t"}
	if err := repo.CreateSession(ctx, sess); err != nil {
		t.Fatal(err)
	}
	user := &database.AIMessage{
		SessionID: sess.ID, Role: ai.RoleUser, Content: "q", Parts: "[]", RunID: "run-steal",
	}
	if err := repo.CreateMessage(ctx, user); err != nil {
		t.Fatal(err)
	}
	p := newTurnPersister(repo, zap.NewNop(), sess.ID, "run-steal", user.ID)
	p.UpsertStreamingText("partial")
	_, cancel := context.WithCancel(context.Background())
	s := &AIService{
		logger:      zap.NewNop(),
		sessionRepo: repo,
		runs: map[string]*agentRun{
			sess.ID: {runID: "run-steal", cancel: cancel, persister: p, approvals: map[string]chan approvalDecision{}},
		},
		cancelled: make(map[string]struct{}),
	}
	s.forceFinalizeSessionRun(sess.ID)
	if _, ok := s.runs[sess.ID]; ok {
		t.Fatal("run should be removed from map")
	}
	// Late Finalize from the stolen Chat must be a no-op.
	if _, err := p.Finalize(nil, true); err != nil {
		t.Fatal(err)
	}
	refs, err := repo.ListUnsealedRuns(ctx, sess.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(refs) != 0 {
		t.Fatalf("unsealed left: %v", refs)
	}
	all, _ := repo.ListMessages(ctx, sess.ID)
	found := false
	for _, m := range all {
		if m.Role == ai.RoleModel && m.Content == "partial" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected partial model persisted")
	}
}

func TestIsChatStopped(t *testing.T) {
	if isChatStopped(nil, context.Background()) {
		t.Fatal("background ctx is not stopped")
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if !isChatStopped(context.Canceled, canceled) {
		t.Fatal("canceled ctx should stop")
	}
	deadline, cancelDeadline := context.WithTimeout(context.Background(), 0)
	defer cancelDeadline()
	<-deadline.Done()
	if !isChatStopped(context.DeadlineExceeded, deadline) {
		t.Fatal("deadline ctx should stop")
	}
}

func TestWaitSessionRunIdle(t *testing.T) {
	s := &AIService{
		runs:      make(map[string]*agentRun),
		cancelled: make(map[string]struct{}),
	}
	start := time.Now()
	s.waitSessionRunIdle("sess", 200*time.Millisecond)
	if time.Since(start) > 100*time.Millisecond {
		t.Fatal("idle session should return immediately")
	}

	_, cancel := context.WithCancel(context.Background())
	defer cancel()
	s.runs["sess"] = &agentRun{runID: "r", cancel: cancel}
	go func() {
		time.Sleep(80 * time.Millisecond)
		s.runsMu.Lock()
		delete(s.runs, "sess")
		s.runsMu.Unlock()
	}()
	start = time.Now()
	s.waitSessionRunIdle("sess", time.Second)
	if time.Since(start) < 50*time.Millisecond {
		t.Fatal("should have waited for run to clear")
	}
	if _, ok := s.runs["sess"]; ok {
		t.Fatal("run still present")
	}
}

func TestTurnPersisterForceFinalizeCancelled(t *testing.T) {
	db := openAITestDB(t)
	repo := db.AISessionRepo
	ctx := context.Background()

	sess := &database.AISession{Title: "t"}
	if err := repo.CreateSession(ctx, sess); err != nil {
		t.Fatal(err)
	}
	user := &database.AIMessage{
		SessionID: sess.ID, Role: ai.RoleUser, Content: "q", Parts: "[]", RunID: "run-f",
	}
	if err := repo.CreateMessage(ctx, user); err != nil {
		t.Fatal(err)
	}
	p := newTurnPersister(repo, zap.NewNop(), sess.ID, "run-f", user.ID)
	p.Checkpoint([]ai.ChatMessage{{
		Role: ai.RoleModel, Content: "partial", Parts: ai.UserTextPartsJSON("partial"),
	}})
	p.UpsertStreamingText(" more")
	last, err := p.ForceFinalize()
	if err != nil {
		t.Fatal(err)
	}
	if last == nil || last.Content == "" {
		t.Fatalf("expected forced content, got %+v", last)
	}
	all, err := repo.ListMessages(ctx, sess.ID)
	if err != nil {
		t.Fatal(err)
	}
	var userStatus string
	for _, m := range all {
		if m.RunID != "" {
			t.Fatalf("unsealed after force: %+v", m)
		}
		if m.Role == ai.RoleUser {
			userStatus = m.Status
		}
		if m.Role == ai.RoleModel && m.Status != database.MessageStatusCancelled {
			t.Fatalf("model should be cancelled: %+v", m)
		}
	}
	if userStatus != database.MessageStatusCancelled {
		t.Fatalf("user status = %q", userStatus)
	}
}

func TestSealOrphanRuns(t *testing.T) {
	db := openAITestDB(t)
	repo := db.AISessionRepo
	ctx := context.Background()

	sess := &database.AISession{Title: "t"}
	if err := repo.CreateSession(ctx, sess); err != nil {
		t.Fatal(err)
	}
	if err := repo.CreateMessage(ctx, &database.AIMessage{
		SessionID: sess.ID, Role: ai.RoleUser, Content: "q", Parts: "[]", RunID: "orphan",
	}); err != nil {
		t.Fatal(err)
	}
	if err := repo.ReplaceRunMessages(ctx, sess.ID, "orphan", []*database.AIMessage{
		{ID: "om1", Role: ai.RoleModel, Content: "stuck", Parts: ai.UserTextPartsJSON("stuck")},
	}); err != nil {
		t.Fatal(err)
	}

	s := &AIService{
		logger:      zap.NewNop(),
		sessionRepo: repo,
		runs:        make(map[string]*agentRun),
		cancelled:   make(map[string]struct{}),
	}
	s.RecoverOrphanTurns()

	all, err := repo.ListMessages(ctx, sess.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) < 2 {
		t.Fatalf("want user+model, got %d", len(all))
	}
	for _, m := range all {
		if m.RunID != "" {
			t.Fatalf("still unsealed: %+v", m)
		}
		if m.Status != database.MessageStatusCancelled {
			t.Fatalf("expected cancelled, got %+v", m)
		}
	}
}

func TestFlushAndStopForceFinalizes(t *testing.T) {
	db := openAITestDB(t)
	repo := db.AISessionRepo
	ctx := context.Background()

	sess := &database.AISession{Title: "t"}
	if err := repo.CreateSession(ctx, sess); err != nil {
		t.Fatal(err)
	}
	user := &database.AIMessage{
		SessionID: sess.ID, Role: ai.RoleUser, Content: "q", Parts: "[]", RunID: "run-flush",
	}
	if err := repo.CreateMessage(ctx, user); err != nil {
		t.Fatal(err)
	}
	p := newTurnPersister(repo, zap.NewNop(), sess.ID, "run-flush", user.ID)
	p.UpsertStreamingText("in flight")
	_, cancel := context.WithCancel(context.Background())
	s := &AIService{
		logger:      zap.NewNop(),
		sessionRepo: repo,
		runs: map[string]*agentRun{
			sess.ID: {runID: "run-flush", cancel: cancel, persister: p, approvals: map[string]chan approvalDecision{}},
		},
		cancelled: make(map[string]struct{}),
	}
	s.FlushAndStop()
	if len(s.runs) != 0 {
		t.Fatal("runs should be cleared")
	}
	refs, err := repo.ListUnsealedRuns(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(refs) != 0 {
		t.Fatalf("unsealed left: %v", refs)
	}
	all, _ := repo.ListMessages(ctx, sess.ID)
	found := false
	for _, m := range all {
		if m.Role == ai.RoleModel && m.Content != "" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected model content after FlushAndStop")
	}
}
