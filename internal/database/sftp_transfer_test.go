package database

import (
	"os"
	"path/filepath"
	"testing"

	"go.uber.org/zap"

	"github.com/ilaziness/vela/internal/transfer"
)

func TestSftpTransferRepositoryCRUD(t *testing.T) {
	dir := t.TempDir()
	db := NewDatabase(dir, zap.NewNop())
	if err := db.Initialize(); err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	repo := db.SftpTransferRepo
	rec := transfer.QueueRecord{
		ID:           "t1",
		OwnerKey:     "bookmark-a",
		TransferType: transfer.TypeDownload,
		LocalFile:    "/tmp/a",
		RemoteFile:   "/remote/a",
		TotalSize:    100,
		Status:       transfer.StatusFailed,
		Error:        "boom",
		Overwrite:    true,
	}
	if err := repo.Upsert(rec); err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	got, err := repo.Get("t1")
	if err != nil || got == nil {
		t.Fatalf("Get: %v %#v", err, got)
	}
	if got.OwnerKey != "bookmark-a" || got.Status != transfer.StatusFailed || !got.Overwrite {
		t.Fatalf("unexpected record: %+v", got)
	}
	list, err := repo.ListByOwner("bookmark-a")
	if err != nil || len(list) != 1 || !list[0].Overwrite {
		t.Fatalf("ListByOwner: %v %d", err, len(list))
	}
	if err := repo.SetStatus("t1", transfer.StatusInterrupted, "interrupted"); err != nil {
		t.Fatalf("SetStatus: %v", err)
	}
	if err := repo.Delete("t1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	gone, err := repo.Get("t1")
	if err != nil || gone != nil {
		t.Fatalf("expected deleted, got %#v err=%v", gone, err)
	}
	// ensure db file created under user data dir
	if _, err := os.Stat(filepath.Join(dir, "vela.db")); err != nil {
		t.Fatalf("db file missing: %v", err)
	}
}
