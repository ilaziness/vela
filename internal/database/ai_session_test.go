package database

import (
	"context"
	"testing"
	"time"

	"go.uber.org/zap"
)

func openTestDB(t *testing.T) *Database {
	t.Helper()
	db := NewDatabase(t.TempDir(), zap.NewNop())
	if err := db.Initialize(); err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func TestReplaceRunMessagesKeepsUserAndIsIdempotent(t *testing.T) {
	db := openTestDB(t)
	repo := db.AISessionRepo
	ctx := context.Background()

	sess := &AISession{Title: "t"}
	if err := repo.CreateSession(ctx, sess); err != nil {
		t.Fatal(err)
	}
	user := &AIMessage{
		SessionID: sess.ID,
		Role:      "user",
		Content:   "hi",
		Parts:     "[]",
		RunID:     "run-1",
		Timestamp: time.Now(),
	}
	if err := repo.CreateMessage(ctx, user); err != nil {
		t.Fatal(err)
	}

	m1 := &AIMessage{ID: "m1", Role: "model", Content: "a", Parts: "[]"}
	if err := repo.ReplaceRunMessages(ctx, sess.ID, "run-1", []*AIMessage{m1}); err != nil {
		t.Fatal(err)
	}
	m1b := &AIMessage{ID: "m1", Role: "model", Content: "ab", Parts: "[]"}
	m2 := &AIMessage{ID: "m2", Role: "tool", Content: "out", Parts: "[]"}
	if err := repo.ReplaceRunMessages(ctx, sess.ID, "run-1", []*AIMessage{m1b, m2}); err != nil {
		t.Fatal(err)
	}

	all, err := repo.ListMessages(ctx, sess.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 3 {
		t.Fatalf("got %d msgs, want 3 (user+model+tool)", len(all))
	}
	if all[0].Role != "user" || all[0].Content != "hi" || all[0].RunID != "run-1" {
		t.Fatalf("user row corrupted: %+v", all[0])
	}
	if all[1].ID != "m1" || all[1].Content != "ab" {
		t.Fatalf("model = %+v", all[1])
	}
	if all[2].ID != "m2" {
		t.Fatalf("tool = %+v", all[2])
	}
}

func TestSetMessagesRunID(t *testing.T) {
	db := openTestDB(t)
	repo := db.AISessionRepo
	ctx := context.Background()
	sess := &AISession{Title: "t"}
	if err := repo.CreateSession(ctx, sess); err != nil {
		t.Fatal(err)
	}
	m := &AIMessage{SessionID: sess.ID, Role: "model", Content: "x", Parts: "[]", Timestamp: time.Now()}
	if err := repo.CreateMessage(ctx, m); err != nil {
		t.Fatal(err)
	}
	if m.ID == "" {
		t.Fatal("expected CreateMessage to assign id")
	}
	if err := repo.SetMessagesRunID(ctx, "cold-run", []string{m.ID}); err != nil {
		t.Fatal(err)
	}
	all, err := repo.ListMessages(ctx, sess.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 1 || all[0].RunID != "cold-run" {
		t.Fatalf("got %+v", all)
	}
}

func TestSealRunMessagesClearsRunID(t *testing.T) {
	db := openTestDB(t)
	repo := db.AISessionRepo
	ctx := context.Background()

	sess := &AISession{Title: "t"}
	if err := repo.CreateSession(ctx, sess); err != nil {
		t.Fatal(err)
	}
	if err := repo.CreateMessage(ctx, &AIMessage{
		SessionID: sess.ID, Role: "user", Content: "q", Parts: "[]", RunID: "r1",
	}); err != nil {
		t.Fatal(err)
	}
	if err := repo.ReplaceRunMessages(ctx, sess.ID, "r1", []*AIMessage{
		{ID: "m1", Role: "model", Content: "a", Parts: "[]"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := repo.SealRunMessages(ctx, sess.ID, "r1"); err != nil {
		t.Fatal(err)
	}
	all, err := repo.ListMessages(ctx, sess.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range all {
		if m.RunID != "" {
			t.Fatalf("expected sealed run_id empty, got %+v", m)
		}
	}
	refs, err := repo.ListUnsealedRuns(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(refs) != 0 {
		t.Fatalf("expected no unsealed runs, got %v", refs)
	}
}

func TestUpdateMessageContent(t *testing.T) {
	db := openTestDB(t)
	repo := db.AISessionRepo
	ctx := context.Background()

	sess := &AISession{Title: "t"}
	if err := repo.CreateSession(ctx, sess); err != nil {
		t.Fatal(err)
	}
	msg := &AIMessage{SessionID: sess.ID, Role: "model", Content: "a", Parts: "[]", RunID: "r"}
	if err := repo.CreateMessage(ctx, msg); err != nil {
		t.Fatal(err)
	}
	if err := repo.UpdateMessageContent(ctx, msg.ID, "abc", `[{"text":"abc"}]`); err != nil {
		t.Fatal(err)
	}
	all, err := repo.ListMessages(ctx, sess.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 1 || all[0].Content != "abc" {
		t.Fatalf("got %+v", all)
	}
}

func TestUpdateSessionPlanPreservedAcrossTitleUpdate(t *testing.T) {
	db := openTestDB(t)
	repo := db.AISessionRepo
	ctx := context.Background()

	sess := &AISession{Title: "t"}
	if err := repo.CreateSession(ctx, sess); err != nil {
		t.Fatal(err)
	}
	plan := `{"title":"p","tasks":[{"id":"1","title":"a","status":"pending"}]}`
	if err := repo.UpdateSessionPlan(ctx, sess.ID, plan); err != nil {
		t.Fatal(err)
	}
	got, err := repo.GetSession(ctx, sess.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.PlanJSON != plan {
		t.Fatalf("plan=%q", got.PlanJSON)
	}
	got.Title = "renamed"
	if err := repo.UpdateSession(ctx, got); err != nil {
		t.Fatal(err)
	}
	got2, err := repo.GetSession(ctx, sess.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got2.Title != "renamed" || got2.PlanJSON != plan {
		t.Fatalf("after title update: %+v", got2)
	}
	if err := repo.UpdateSessionPlan(ctx, sess.ID, ""); err != nil {
		t.Fatal(err)
	}
	got3, err := repo.GetSession(ctx, sess.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got3.PlanJSON != "" {
		t.Fatalf("clear failed: %q", got3.PlanJSON)
	}
	// Idempotent rewrite must not fail (SQLite RowsAffected may be 0).
	if err := repo.UpdateSessionPlan(ctx, sess.ID, ""); err != nil {
		t.Fatalf("idempotent clear: %v", err)
	}
}

func TestDeleteSessionRemovesMessages(t *testing.T) {
	db := openTestDB(t)
	repo := db.AISessionRepo
	ctx := context.Background()

	sess := &AISession{Title: "t"}
	if err := repo.CreateSession(ctx, sess); err != nil {
		t.Fatal(err)
	}
	keep := &AISession{Title: "keep"}
	if err := repo.CreateSession(ctx, keep); err != nil {
		t.Fatal(err)
	}
	if err := repo.CreateMessage(ctx, &AIMessage{
		SessionID: sess.ID, Role: "user", Content: "q", Parts: "[]",
	}); err != nil {
		t.Fatal(err)
	}
	if err := repo.CreateMessage(ctx, &AIMessage{
		SessionID: sess.ID, Role: "model", Content: "a", Parts: "[]",
	}); err != nil {
		t.Fatal(err)
	}
	if err := repo.CreateMessage(ctx, &AIMessage{
		SessionID: keep.ID, Role: "user", Content: "other", Parts: "[]",
	}); err != nil {
		t.Fatal(err)
	}

	if err := repo.DeleteSession(ctx, sess.ID); err != nil {
		t.Fatal(err)
	}
	left, err := repo.ListMessages(ctx, sess.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(left) != 0 {
		t.Fatalf("deleted session still has messages: %+v", left)
	}
	kept, err := repo.ListMessages(ctx, keep.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(kept) != 1 || kept[0].Content != "other" {
		t.Fatalf("other session messages: %+v", kept)
	}
	// Recreate leftover messages without a session row (pre-fix CASCADE miss).
	if err := repo.CreateMessage(ctx, &AIMessage{
		SessionID: sess.ID, Role: "user", Content: "orphan", Parts: "[]",
	}); err != nil {
		t.Fatal(err)
	}
	if err := repo.DeleteSession(ctx, sess.ID); err == nil {
		t.Fatal("expected session not found")
	}
	orphans, err := repo.ListMessages(ctx, sess.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(orphans) != 0 {
		t.Fatalf("orphan messages left: %+v", orphans)
	}
}
