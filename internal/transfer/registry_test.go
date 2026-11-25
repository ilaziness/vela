package transfer

import (
	"errors"
	"testing"
)

func TestTrackerStopIdempotent(t *testing.T) {
	var n int
	r := NewRegistry(func(ProgressData) { n++ })
	tr, err := r.New("sess", TypeUpload, "local", "remote", 100)
	if err != nil {
		t.Fatal(err)
	}
	tr.Stop(nil)
	tr.Stop(nil)
	if n < 2 {
		t.Fatalf("expected at least create+done events, got %d", n)
	}
}

func TestNewWithRejectsDuplicateActiveID(t *testing.T) {
	r := NewRegistry(nil)
	tr, err := r.NewWith("sess", TypeUpload, "a", "b", 10, NewOpts{ID: "same"})
	if err != nil {
		t.Fatal(err)
	}
	defer tr.Stop(nil)
	if _, err := r.NewWith("sess", TypeUpload, "c", "d", 10, NewOpts{ID: "same"}); err == nil {
		t.Fatal("expected duplicate active id error")
	}
	if !r.HasActive("same") {
		t.Fatal("expected HasActive")
	}
}

func TestTrackerStopRecordsErrorAndWait(t *testing.T) {
	r := NewRegistry(nil)
	tr, err := r.New("sess", TypeDownload, "local", "remote", 10)
	if err != nil {
		t.Fatal(err)
	}
	want := errors.New("download failed")
	tr.Stop(want)
	if got := tr.Wait(); got == nil || got.Error() != want.Error() {
		t.Fatalf("Wait()=%v, want %v", got, want)
	}
}
