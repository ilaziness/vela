package ai

import (
	"strings"
	"testing"

	"github.com/ilaziness/vela/internal/ssh"
)

func TestBuildSystemPromptNoSSH(t *testing.T) {
	got := BuildSystemPrompt(nil, nil, false)
	if !strings.Contains(got, "不要调用 run_ssh_command") {
		t.Fatalf("expected no-SSH tool ban, got:\n%s", got)
	}
	if !strings.Contains(got, "不要假设仍可访问聊天历史中曾出现的其他主机") {
		t.Fatalf("expected history-host warning, got:\n%s", got)
	}
	if strings.Contains(got, "可以使用 run_ssh_command") {
		t.Fatalf("unexpected SSH-enabled wording:\n%s", got)
	}
}

func TestBuildSystemPromptWithSSH(t *testing.T) {
	ctx := &SSHPromptContext{Host: "10.0.0.2", Port: 22, User: "root"}
	got := BuildSystemPrompt(ctx, nil, true)
	if !strings.Contains(got, "root@10.0.0.2:22") {
		t.Fatalf("expected connection line, got:\n%s", got)
	}
	if !strings.Contains(got, "以本轮系统提示中的连接为准") {
		t.Fatalf("expected current-host rule, got:\n%s", got)
	}
	remote := &ssh.RemoteSystemInfo{Ready: true, Hostname: "box"}
	got2 := BuildSystemPrompt(ctx, remote, true)
	if !strings.Contains(got2, "主机名：box") {
		t.Fatalf("expected remote hostname, got:\n%s", got2)
	}
}
