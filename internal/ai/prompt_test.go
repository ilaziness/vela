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
