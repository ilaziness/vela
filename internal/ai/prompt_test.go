package ai

import (
	"strings"
	"testing"
	"time"

	"github.com/ilaziness/vela/internal/ssh"
)

func TestBuildSystemPromptNoSSH(t *testing.T) {
	got := BuildSystemPromptOpts(PromptOptions{})
	if !strings.Contains(got, time.Now().Format("2006年1月")) {
		t.Fatalf("expected current year-month in prompt, got:\n%s", got)
	}
	if !strings.Contains(got, "不要调用 run_ssh_command") {
		t.Fatalf("expected no-SSH tool ban, got:\n%s", got)
	}
	if !strings.Contains(got, "不要假设仍可访问聊天历史中曾出现的其他主机") {
		t.Fatalf("expected history-host warning, got:\n%s", got)
	}
	if strings.Contains(got, "可以使用 run_ssh_command") {
		t.Fatalf("unexpected SSH-enabled wording:\n%s", got)
	}
	if strings.Contains(got, "run_ssh_command 在当前已连接主机") {
		t.Fatalf("SSH capability should be omitted when disabled:\n%s", got)
	}
}

func TestBuildSystemPromptWithSSH(t *testing.T) {
	ctx := &SSHPromptContext{Host: "10.0.0.2", Port: 22, User: "root"}
	got := BuildSystemPromptOpts(PromptOptions{SSH: ctx, EnableSSH: true})
	if !strings.Contains(got, "root@10.0.0.2:22") {
		t.Fatalf("expected connection line, got:\n%s", got)
	}
	if !strings.Contains(got, "以本轮系统提示中的连接为准") {
		t.Fatalf("expected current-host rule, got:\n%s", got)
	}
	remote := &ssh.RemoteSystemInfo{Ready: true, Hostname: "box"}
	got2 := BuildSystemPromptOpts(PromptOptions{SSH: ctx, Remote: remote, EnableSSH: true})
	if !strings.Contains(got2, "主机名：box") {
		t.Fatalf("expected remote hostname, got:\n%s", got2)
	}
}

func TestBuildSystemPromptOptsFSAndSearch(t *testing.T) {
	got := BuildSystemPromptOpts(PromptOptions{
		EnableFS:        true,
		AllowDirs:       []string{`D:\work`},
		EnableWebSearch: true,
	})
	if !strings.Contains(got, "read_local_file") {
		t.Fatalf("expected FS tools in prompt:\n%s", got)
	}
	if !strings.Contains(got, `D:\work`) {
		t.Fatalf("expected allow dir in prompt:\n%s", got)
	}
	if !strings.Contains(got, "web_search") {
		t.Fatalf("expected web_search in prompt:\n%s", got)
	}
	off := BuildSystemPromptOpts(PromptOptions{})
	if strings.Contains(off, "web_search") || strings.Contains(off, "read_local_file") {
		t.Fatalf("disabled tools must not appear:\n%s", off)
	}
}

func TestBuildSystemPromptCurrentPlanLastSection(t *testing.T) {
	plan := `{"title":"Fix nginx","tasks":[{"id":"t1","title":"Check config","status":"running"},{"id":"t2","title":"Reload","status":"pending"}]}`
	got := BuildSystemPromptOpts(PromptOptions{PlanJSON: plan})
	idxPlan := strings.Index(got, "## 当前计划")
	idxSSH := strings.Index(got, "## 当前 SSH 上下文")
	if idxPlan < 0 {
		t.Fatalf("missing ## 当前计划:\n%s", got)
	}
	if idxSSH < 0 || idxPlan < idxSSH {
		t.Fatalf("## 当前计划 must follow SSH section: ssh=%d plan=%d\n%s", idxSSH, idxPlan, got)
	}
	if !strings.Contains(got, "权威计划快照") {
		t.Fatalf("expected authoritative wording:\n%s", got)
	}
	if !strings.Contains(got, "[running] Check config（id=t1）") {
		t.Fatalf("expected formatted task:\n%s", got)
	}
	if !strings.Contains(got, "禁止多步做完再一次批量改状态") {
		t.Fatalf("expected step-update rule:\n%s", got)
	}
	if !strings.Contains(got, "变更必须再调用 upsert_plan") {
		t.Fatalf("expected plan mutation rule in section:\n%s", got)
	}
}

func TestBuildSystemPromptEmptyOrInvalidPlanSkipped(t *testing.T) {
	if strings.Contains(BuildSystemPromptOpts(PromptOptions{}), "## 当前计划") {
		t.Fatal("empty plan must omit section")
	}
	if strings.Contains(BuildSystemPromptOpts(PromptOptions{PlanJSON: `{`}), "## 当前计划") {
		t.Fatal("invalid JSON must omit section")
	}
	if strings.Contains(BuildSystemPromptOpts(PromptOptions{PlanJSON: `{"title":"x","tasks":[]}`}), "## 当前计划") {
		t.Fatal("empty tasks must omit section")
	}
}
