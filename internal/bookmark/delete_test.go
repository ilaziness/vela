package bookmark

import (
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/ilaziness/vela/internal/database"
)

func newDeleteTestDB(t *testing.T) *database.Database {
	t.Helper()
	db := database.NewDatabase(t.TempDir(), zap.NewNop())
	if err := db.Initialize(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func mustGroup(t *testing.T, db *database.Database, name string) *database.BookmarkGroupDB {
	t.Helper()
	if err := db.BookmarkRepo.InsertGroup(&database.BookmarkGroupDB{Name: name}); err != nil {
		t.Fatal(err)
	}
	g, err := db.BookmarkRepo.GetGroupByName(name)
	if err != nil {
		t.Fatal(err)
	}
	return g
}

func mustBookmark(t *testing.T, db *database.Database, id, title, jump string, groupID int) {
	t.Helper()
	now := time.Now()
	if err := db.BookmarkRepo.InsertBookmark(&database.BookmarkDB{
		ID: id, GroupID: groupID, Title: title, Protocol: "ssh",
		Host: "h", Port: 22, User: "u", ProxyJumpID: jump,
		CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}
}

func assertProxyJumpCleared(t *testing.T, db *database.Database, id string) {
	t.Helper()
	b, err := db.BookmarkRepo.GetBookmarkByID(id)
	if err != nil {
		t.Fatal(err)
	}
	if b.ProxyJumpID != "" {
		t.Fatalf("bookmark %s still references a deleted ProxyJump target: %q", id, b.ProxyJumpID)
	}
}

func TestDeleteBookmarkCleansRelatedData(t *testing.T) {
	db := newDeleteTestDB(t)
	main := mustGroup(t, db, DefaultGroupName)
	other := mustGroup(t, db, "other")
	mustBookmark(t, db, "jump-1", "jump", "", main.ID)
	mustBookmark(t, db, "target-1", "target", "jump-1", other.ID)
	if err := db.SSHTunnelRepo.Insert(&database.SSHTunnelDB{
		ID: "t1", BookmarkID: "jump-1", TunnelType: "local",
		LocalAddr: "127.0.0.1:8080", RemoteAddr: "db:3306",
	}); err != nil {
		t.Fatal(err)
	}

	svc := New(zap.NewNop(), db, nil, nil, nil, nil, nil)
	if err := svc.Delete("jump-1"); err != nil {
		t.Fatal(err)
	}

	if _, err := db.BookmarkRepo.GetBookmarkByID("jump-1"); err == nil {
		t.Fatal("deleted bookmark still present")
	}
	if tunnels, err := db.SSHTunnelRepo.ListByBookmark("jump-1"); err != nil || len(tunnels) != 0 {
		t.Fatalf("tunnels not cleaned: %v, err=%v", tunnels, err)
	}
	assertProxyJumpCleared(t, db, "target-1")
}

func TestDeleteGroupCleansRelatedData(t *testing.T) {
	db := newDeleteTestDB(t)
	doomed := mustGroup(t, db, "doomed")
	main := mustGroup(t, db, DefaultGroupName)
	mustBookmark(t, db, "jump-1", "jump", "", doomed.ID)
	mustBookmark(t, db, "jump-2", "jump2", "", doomed.ID)
	mustBookmark(t, db, "target-1", "target", "jump-1", main.ID)
	if err := db.SSHTunnelRepo.Insert(&database.SSHTunnelDB{
		ID: "t1", BookmarkID: "jump-2", TunnelType: "local",
		LocalAddr: "127.0.0.1:8080", RemoteAddr: "db:3306",
	}); err != nil {
		t.Fatal(err)
	}

	svc := New(zap.NewNop(), db, nil, nil, nil, nil, nil)
	if err := svc.DeleteGroup("doomed"); err != nil {
		t.Fatal(err)
	}

	if _, err := db.BookmarkRepo.GetGroupByName("doomed"); err == nil {
		t.Fatal("group still present")
	}
	for _, id := range []string{"jump-1", "jump-2"} {
		if _, err := db.BookmarkRepo.GetBookmarkByID(id); err == nil {
			t.Fatalf("group bookmark %s still present", id)
		}
	}
	if tunnels, err := db.SSHTunnelRepo.ListByBookmark("jump-2"); err != nil || len(tunnels) != 0 {
		t.Fatalf("group tunnels not cleaned: %v, err=%v", tunnels, err)
	}
	assertProxyJumpCleared(t, db, "target-1")
}
