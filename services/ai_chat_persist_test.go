package services

import (
	"context"
	"strings"
	"testing"

	genkitAI "github.com/firebase/genkit/go/ai"
	"go.uber.org/zap"

	"github.com/ilaziness/vela/internal/ai"
	"github.com/ilaziness/vela/internal/database"
	"github.com/ilaziness/vela/internal/usererr"
)

func TestPersistableChatErrorTextNotJSON(t *testing.T) {
	err := usererr.Coded("ai.not_enabled", "AI assistant is not enabled. Open Settings → AI assistant to configure and enable it.")
	got := persistableChatErrorText(err)
	if got == "" {
		t.Fatal("expected persistable text")
	}
	if strings.HasPrefix(strings.TrimSpace(got), "{") {
		t.Fatalf("stored JSON: %q", got)
	}
}

func TestPersistAndSealFailedTurnKeepsUser(t *testing.T) {
	db := openAITestDB(t)
	s := &AIService{logger: zap.NewNop(), sessionRepo: db.AISessionRepo}
	ctx := context.Background()
	sess := &database.AISession{Title: "t"}
	if err := s.sessionRepo.CreateSession(ctx, sess); err != nil {
		t.Fatal(err)
	}
	user, err := s.insertUserChatMessage(sess.ID, "hello world", "run-fail", nil, false)
	if err != nil {
		t.Fatal(err)
	}
	s.persistAndSealFailedTurn(sess.ID, "run-fail", ErrAINotEnabled)
	msgs, err := s.sessionRepo.ListMessages(ctx, sess.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 2 {
		t.Fatalf("got %d messages", len(msgs))
	}
	if msgs[0].ID != user.ID || msgs[0].Role != ai.RoleUser || msgs[0].RunID != "" {
		t.Fatalf("user not sealed: %+v", msgs[0])
	}
	if msgs[1].Role != ai.RoleModel || msgs[1].RunID != "" || msgs[1].Content == "" {
		t.Fatalf("model not sealed: %+v", msgs[1])
	}
	if strings.HasPrefix(strings.TrimSpace(msgs[1].Content), "{") {
		t.Fatalf("model stored JSON: %q", msgs[1].Content)
	}
}

func TestPersistAndSealFailedTurnStopHasNoAssistant(t *testing.T) {
	db := openAITestDB(t)
	s := &AIService{logger: zap.NewNop(), sessionRepo: db.AISessionRepo}
	ctx := context.Background()
	sess := &database.AISession{Title: "t"}
	if err := s.sessionRepo.CreateSession(ctx, sess); err != nil {
		t.Fatal(err)
	}
	if _, err := s.insertUserChatMessage(sess.ID, "hello", "run-stop", nil, false); err != nil {
		t.Fatal(err)
	}
	s.persistAndSealFailedTurn(sess.ID, "run-stop", ErrGenerationStopped)
	msgs, err := s.sessionRepo.ListMessages(ctx, sess.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 1 || msgs[0].Role != ai.RoleUser || msgs[0].RunID != "" {
		t.Fatalf("stop should seal user only: %+v", msgs)
	}
}

func TestChatDoesNotPersistWhenApprovalPending(t *testing.T) {
	db := openAITestDB(t)
	repo := db.AISessionRepo
	ctx := context.Background()
	s := &AIService{
		logger:      zap.NewNop(),
		sessionRepo: repo,
		runs:        map[string]*agentRun{},
		cancelled:   map[string]struct{}{},
	}
	sess := &database.AISession{Title: "t"}
	if err := repo.CreateSession(ctx, sess); err != nil {
		t.Fatal(err)
	}
	if err := repo.CreateMessage(ctx, &database.AIMessage{
		SessionID: sess.ID, Role: ai.RoleUser, Content: "q", Parts: ai.UserTextPartsJSON("q"),
	}); err != nil {
		t.Fatal(err)
	}
	part := genkitAI.NewToolRequestPart(&genkitAI.ToolRequest{
		Name: ai.ToolRunSSHCommand, Ref: "c1", Input: map[string]any{"command": "ls"},
	})
	part.Interrupt = &genkitAI.ToolInterrupt{}
	if err := repo.CreateMessage(ctx, &database.AIMessage{
		SessionID: sess.ID, Role: ai.RoleModel, Content: "", Parts: ai.MarshalParts([]*genkitAI.Part{part}),
	}); err != nil {
		t.Fatal(err)
	}
	_, err := s.Chat(&ChatRequest{SessionID: sess.ID, NewMessage: "next", RunID: "run-new"})
	if usererr.CodeOf(err) != "ai.approval_pending" {
		t.Fatalf("err = %v", err)
	}
	msgs, err := repo.ListMessages(ctx, sess.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 2 {
		t.Fatalf("stray persist: %d messages", len(msgs))
	}
}

func TestChatDoesNotPersistWhenOrphanInterruptUnsealed(t *testing.T) {
	db := openAITestDB(t)
	repo := db.AISessionRepo
	ctx := context.Background()
	s := &AIService{
		logger:      zap.NewNop(),
		sessionRepo: repo,
		runs:        map[string]*agentRun{},
		cancelled:   map[string]struct{}{},
	}
	sess := &database.AISession{Title: "t"}
	if err := repo.CreateSession(ctx, sess); err != nil {
		t.Fatal(err)
	}
	if err := repo.CreateMessage(ctx, &database.AIMessage{
		SessionID: sess.ID, Role: ai.RoleUser, Content: "q", Parts: ai.UserTextPartsJSON("q"), RunID: "run-old",
	}); err != nil {
		t.Fatal(err)
	}
	part := genkitAI.NewToolRequestPart(&genkitAI.ToolRequest{
		Name: ai.ToolRunSSHCommand, Ref: "c1", Input: map[string]any{"command": "ls"},
	})
	part.Interrupt = &genkitAI.ToolInterrupt{}
	if err := repo.CreateMessage(ctx, &database.AIMessage{
		SessionID: sess.ID, Role: ai.RoleModel, Content: "", Parts: ai.MarshalParts([]*genkitAI.Part{part}), RunID: "run-old",
	}); err != nil {
		t.Fatal(err)
	}
	_, err := s.Chat(&ChatRequest{SessionID: sess.ID, NewMessage: "next", RunID: "run-new"})
	if usererr.CodeOf(err) != "ai.approval_pending" {
		t.Fatalf("err = %v", err)
	}
	msgs, err := repo.ListMessages(ctx, sess.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range msgs {
		if m.Role == ai.RoleUser && m.Content == "next" {
			t.Fatal("new user should not persist over orphan interrupt")
		}
	}
}
