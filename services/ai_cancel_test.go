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

func TestCancelSSHExecDoesNotCancelRun(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	execCtx, execCancel := context.WithCancel(context.Background())
	defer execCancel()
	run := &agentRun{
		runID:      "run-1",
		cancel:     cancel,
		approvals:  make(map[string]chan approvalDecision),
		execCancel: execCancel,
	}
	s := &AIService{
		runs:      map[string]*agentRun{"sess": run},
		cancelled: make(map[string]struct{}),
	}
	if err := s.CancelSSHExec("sess"); err != nil {
		t.Fatal(err)
	}
	if ctx.Err() != nil {
		t.Fatal("CancelSSHExec must not cancel the agent run")
	}
	if execCtx.Err() == nil {
		t.Fatal("CancelSSHExec should cancel the in-flight Exec context")
	}
	run.mu.Lock()
	abort := run.execAbort
	run.mu.Unlock()
	if abort {
		t.Fatal("with active execCancel, abort flag should not be set")
	}
}

func TestCancelSSHExecBeforeExecSetsAbort(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	run := &agentRun{
		runID:       "run-1",
		cancel:      cancel,
		approvals:   make(map[string]chan approvalDecision),
		execPending: true,
	}
	s := &AIService{
		runs:      map[string]*agentRun{"sess": run},
		cancelled: make(map[string]struct{}),
	}
	if err := s.CancelSSHExec("sess"); err != nil {
		t.Fatal(err)
	}
	if ctx.Err() != nil {
		t.Fatal("CancelSSHExec must not cancel the agent run")
	}
	run.mu.Lock()
	abort := run.execAbort
	run.mu.Unlock()
	if !abort {
		t.Fatal("expected execAbort when Exec pending but not yet registered")
	}
}

func TestCancelSSHExecIdleIsNoop(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	run := &agentRun{
		runID:     "run-1",
		cancel:    cancel,
		approvals: make(map[string]chan approvalDecision),
	}
	s := &AIService{
		runs:      map[string]*agentRun{"sess": run},
		cancelled: make(map[string]struct{}),
	}
	if err := s.CancelSSHExec("sess"); err != nil {
		t.Fatal(err)
	}
	if ctx.Err() != nil {
		t.Fatal("CancelSSHExec must not cancel the agent run")
	}
	run.mu.Lock()
	abort := run.execAbort
	run.mu.Unlock()
	if abort {
		t.Fatal("idle CancelSSHExec must not set execAbort (would poison next Exec)")
	}
}

func TestArmSSHExecCancelSetsPending(t *testing.T) {
	run := &agentRun{
		runID:     "run-1",
		cancel:    func() {},
		approvals: make(map[string]chan approvalDecision),
	}
	h := &aiHost{run: run}
	h.ArmSSHExecCancel()
	run.mu.Lock()
	pending := run.execPending
	run.mu.Unlock()
	if !pending {
		t.Fatal("ArmSSHExecCancel should set execPending")
	}
}

func TestToolDepsArmSSHExecCancelWiring(t *testing.T) {
	run := &agentRun{
		runID:     "run-1",
		cancel:    func() {},
		approvals: make(map[string]chan approvalDecision),
	}
	h := &aiHost{run: run}
	// Same wiring as Chat(): callback on ToolDeps, not ApprovalGate.
	arm := h.ArmSSHExecCancel
	arm()
	run.mu.Lock()
	pending := run.execPending
	run.mu.Unlock()
	if !pending {
		t.Fatal("ToolDeps ArmSSHExecCancel callback should arm execPending")
	}
}

func TestRespondToolApprovalArmsSSHExecPending(t *testing.T) {
	run := &agentRun{
		runID:        "run-1",
		cancel:       func() {},
		approvals:    map[string]chan approvalDecision{"appr-1": make(chan approvalDecision, 1)},
		sshApprovals: map[string]struct{}{"appr-1": {}},
	}
	s := &AIService{
		runs:      map[string]*agentRun{"sess": run},
		cancelled: make(map[string]struct{}),
	}
	if err := s.RespondToolApproval(&ToolApprovalRequest{
		SessionID: "sess", ApprovalID: "appr-1", Approved: true,
	}); err != nil {
		t.Fatal(err)
	}
	run.mu.Lock()
	pending := run.execPending
	_, stillMarked := run.sshApprovals["appr-1"]
	run.mu.Unlock()
	if !pending {
		t.Fatal("approving an SSH-marked approval should arm execPending immediately")
	}
	if stillMarked {
		t.Fatal("sshApprovals mark should be cleared after respond")
	}
}

func TestRespondToolApprovalMCPDoesNotArm(t *testing.T) {
	run := &agentRun{
		runID:     "run-1",
		cancel:    func() {},
		approvals: map[string]chan approvalDecision{"appr-mcp": make(chan approvalDecision, 1)},
		// no sshApprovals entry — MCP
	}
	s := &AIService{
		runs:      map[string]*agentRun{"sess": run},
		cancelled: make(map[string]struct{}),
	}
	if err := s.RespondToolApproval(&ToolApprovalRequest{
		SessionID: "sess", ApprovalID: "appr-mcp", Approved: true,
	}); err != nil {
		t.Fatal(err)
	}
	run.mu.Lock()
	pending := run.execPending
	run.mu.Unlock()
	if pending {
		t.Fatal("MCP approve must not arm execPending")
	}
}

// Mirrors aiHost.Exec's consume of execAbort so a pending Stop cannot stick.
func TestExecAbortIsSingleShot(t *testing.T) {
	run := &agentRun{
		runID:       "run-1",
		cancel:      func() {},
		approvals:   make(map[string]chan approvalDecision),
		execPending: true,
	}
	s := &AIService{
		runs:      map[string]*agentRun{"sess": run},
		cancelled: make(map[string]struct{}),
	}
	if err := s.CancelSSHExec("sess"); err != nil {
		t.Fatal(err)
	}
	run.mu.Lock()
	if !run.execAbort {
		run.mu.Unlock()
		t.Fatal("expected abort after CancelSSHExec while pending")
	}
	// aiHost.Exec registration:
	run.execPending = false
	abort := run.execAbort
	run.execAbort = false
	run.mu.Unlock()
	if !abort {
		t.Fatal("first Exec should see abort")
	}
	run.mu.Lock()
	leftover := run.execAbort
	run.mu.Unlock()
	if leftover {
		t.Fatal("execAbort must be cleared after first consume")
	}
}
