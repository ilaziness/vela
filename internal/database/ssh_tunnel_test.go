package database

import (
	"testing"

	"go.uber.org/zap"
)

func TestSSHTunnelRepositoryCRUD(t *testing.T) {
	dir := t.TempDir()
	db := NewDatabase(dir, zap.NewNop())
	if err := db.Initialize(); err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	repo := db.SSHTunnelRepo
	if repo == nil {
		t.Fatal("SSHTunnelRepo not initialized")
	}

	rules := []SSHTunnelDB{
		{ID: "r1", BookmarkID: "bm-a", TunnelType: "local", LocalAddr: "127.0.0.1:8080", RemoteAddr: "db:3306"},
		{ID: "r2", BookmarkID: "bm-a", TunnelType: "remote", LocalAddr: "localhost:6379", RemotePort: 6380},
		{ID: "r3", BookmarkID: "bm-a", TunnelType: "dynamic", LocalAddr: "127.0.0.1:1080"},
		{ID: "r4", BookmarkID: "bm-b", TunnelType: "dynamic", LocalAddr: "127.0.0.1:1081"},
	}
	for i := range rules {
		if err := repo.Insert(&rules[i]); err != nil {
			t.Fatalf("Insert %s: %v", rules[i].ID, err)
		}
	}

	list, err := repo.ListByBookmark("bm-a")
	if err != nil {
		t.Fatalf("ListByBookmark: %v", err)
	}
	if len(list) != 3 {
		t.Fatalf("ListByBookmark(bm-a) len = %d, want 3", len(list))
	}
	if list[0].ID != "r1" || list[2].ID != "r3" {
		t.Fatalf("unexpected order: %+v", list)
	}
	if list[1].RemotePort != 6380 || list[1].RemoteAddr != "" {
		t.Fatalf("unexpected remote rule: %+v", list[1])
	}

	// Delete is scoped to the bookmark id.
	if err := repo.Delete("r4", "bm-a"); err != nil {
		t.Fatalf("Delete wrong bookmark: %v", err)
	}
	if err := repo.Delete("r4", "bm-b"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	list, err = repo.ListByBookmark("bm-b")
	if err != nil || len(list) != 0 {
		t.Fatalf("expected bm-b empty, got %d err=%v", len(list), err)
	}

	if err := repo.DeleteByBookmark("bm-a"); err != nil {
		t.Fatalf("DeleteByBookmark: %v", err)
	}
	list, err = repo.ListByBookmark("bm-a")
	if err != nil || len(list) != 0 {
		t.Fatalf("expected bm-a empty after DeleteByBookmark, got %d err=%v", len(list), err)
	}
}
