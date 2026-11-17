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

func TestTrackerRemoteOpFlowsToProgressData(t *testing.T) {
	var events []ProgressData
	r := NewRegistry(func(d ProgressData) { events = append(events, d) })
	tr, err := r.NewWith("sess", TypeUpload, "/remote/a", "/remote/b", 10, NewOpts{ID: "p1", RemoteOp: true})
	if err != nil {
		t.Fatal(err)
	}
	if !tr.RemoteOp() {
		t.Fatal("RemoteOp() = false, want true")
	}
	tr.Stop(nil)
	var saw bool
	for _, d := range events {
		if d.ID == "p1" && d.RemoteOp {
			saw = true
		}
	}
	if !saw {
		t.Fatalf("no progress event carried RemoteOp: %+v", events)
	}
	// Regular transfers default to RemoteOp=false.
	tr2, err := r.New("sess", TypeDownload, "l", "r", 1)
	if err != nil {
		t.Fatal(err)
	}
	defer tr2.Stop(nil)
	if tr2.RemoteOp() {
		t.Fatal("plain transfer RemoteOp() = true, want false")
	}
}
