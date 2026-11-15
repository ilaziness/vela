package transferui

import (
	"testing"

	"github.com/ilaziness/vela/internal/transfer"
)

func TestTransferDisplayName(t *testing.T) {
	up := transfer.ProgressData{TransferType: transfer.TypeUpload, LocalFile: "/tmp/a.txt", RemoteFile: "/r/a.txt"}
	if got := transferDisplayName(up); got != "a.txt" {
		t.Fatalf("upload name = %q", got)
	}
	down := transfer.ProgressData{TransferType: transfer.TypeDownload, RemoteFile: "/r/b.bin"}
	if got := transferDisplayName(down); got != "b.bin" {
		t.Fatalf("download name = %q", got)
	}
}

func TestTransferStatusText(t *testing.T) {
	active := transfer.ProgressData{TotalSize: 100, Transferred: 50, Rate: 1024}
	if got := transferStatusText(active); got == "" {
		t.Fatal("active transfer has empty status")
	}
	done := transfer.ProgressData{Done: true}
	if got := transferStatusText(done); got == "" {
		t.Fatal("done transfer has empty status")
	}
	failed := transfer.ProgressData{Done: true, Error: "boom"}
	if got := transferStatusText(failed); got != "boom" {
		t.Fatalf("failed status = %q", got)
	}
}
