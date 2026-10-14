package services

import (
	"context"
	"testing"
)

func TestCancelChatRequiresIDs(t *testing.T) {
	s := &AIService{
		runs:      make(map[string]*agentRun),
		cancelled: make(map[string]struct{}),
	}
	if err := s.CancelChat("", "run1"); err == nil {
		t.Fatal("expected error for empty session id")
	}
	if err := s.CancelChat("sess", ""); err == nil {
		t.Fatal("expected error for empty run id")
	}
}

func TestCancelChatMarksBeforeRegister(t *testing.T) {
	s := &AIService{
		runs:      make(map[string]*agentRun),
		cancelled: make(map[string]struct{}),
	}
	if err := s.CancelChat("sess", "run1"); err != nil {
		t.Fatal(err)
	}
	if !s.isRunCancelled("run1") {
		t.Fatal("expected run1 marked cancelled")
	}
}

func TestCancelChatOnlyStopsMatchingRun(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	run := &agentRun{
		runID:     "run-b",
		cancel:    cancel,
		approvals: make(map[string]chan approvalDecision),
	}
	s := &AIService{
		runs:      map[string]*agentRun{"sess": run},
		cancelled: make(map[string]struct{}),
	}

	if err := s.CancelChat("sess", "run-a"); err != nil {
		t.Fatal(err)
	}
	if ctx.Err() != nil {
		t.Fatal("CancelChat for a different runID must not cancel the active run")
	}
	if !s.isRunCancelled("run-a") {
		t.Fatal("expected run-a marked cancelled for pre-register race")
	}

	if err := s.CancelChat("sess", "run-b"); err != nil {
		t.Fatal(err)
	}
	if ctx.Err() == nil {
		t.Fatal("CancelChat for matching runID should cancel the active run")
	}
	if s.isRunCancelled("run-b") {
		t.Fatal("matching active run should be cancelled in-place, not left in cancelled map")
	}
}

func TestClearCancelledRun(t *testing.T) {
	s := &AIService{
		runs:      make(map[string]*agentRun),
		cancelled: map[string]struct{}{"run1": {}},
	}
	s.clearCancelledRun("run1")
	s.runsMu.Lock()
	_, ok := s.cancelled["run1"]
	s.runsMu.Unlock()
	if ok {
		t.Fatal("expected cancelled mark cleared")
	}
}
