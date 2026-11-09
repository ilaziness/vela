package ai

import (
	"strings"
	"testing"
	"time"

	"github.com/ilaziness/vela/internal/ssh"
)

func TestBuildSystemPromptNoSSH(t *testing.T) {
	got := BuildSystemPromptOpts(PromptOptions{UILanguage: "en"})
	ym := time.Now().Format("2006-01")
	if !strings.Contains(got, ym) {
		t.Fatalf("expected current year-month in prompt, got:\n%s", got)
	}
	if !strings.Contains(got, "Respond in the user's UI language: en") {
		t.Fatalf("expected UI language rule:\n%s", got)
	}
	if !strings.Contains(got, "Current session") {
		t.Fatalf("expected Current session section:\n%s", got)
	}
	if strings.Contains(got, "run_ssh_command is available") {
		t.Fatalf("did not expect connected SSH tools blurb:\n%s", got)
	}
	if !strings.Contains(got, "No session bound") {
		t.Fatalf("expected unbound session wording:\n%s", got)
	}
}

func TestBuildSystemPromptWithSSH(t *testing.T) {
	ctx := &SSHPromptContext{Host: "h.example", Port: 22, User: "u"}
	got := BuildSystemPromptOpts(PromptOptions{SSH: ctx, EnableSSH: true, UILanguage: "zh-CN"})
	if !strings.Contains(got, "run_ssh_command") {
		t.Fatalf("expected SSH capability:\n%s", got)
	}
	if !strings.Contains(got, "protocol=ssh") {
		t.Fatalf("expected protocol=ssh:\n%s", got)
	}
	if !strings.Contains(got, "u@h.example:22") {
		t.Fatalf("expected connection line:\n%s", got)
	}
	if !strings.Contains(got, "Respond in the user's UI language: zh-CN") {
		t.Fatalf("expected zh-CN language rule:\n%s", got)
	}
	remote := &ssh.RemoteSystemInfo{Ready: true, Hostname: "box", OSPretty: "Ubuntu", OSID: "ubuntu", OSVersion: "22.04"}
	got2 := BuildSystemPromptOpts(PromptOptions{SSH: ctx, Remote: remote, EnableSSH: true, UILanguage: "en"})
	if !strings.Contains(got2, "Hostname: box") {
		t.Fatalf("expected remote info:\n%s", got2)
	}
}

func TestBuildSystemPromptWithSerial(t *testing.T) {
	got := BuildSystemPromptOpts(PromptOptions{
		Serial:     &SerialPromptContext{Device: "COM3", BaudRate: 115200},
		UILanguage: "en",
	})
	if !strings.Contains(got, "protocol=serial") {
		t.Fatalf("expected protocol=serial:\n%s", got)
	}
	if !strings.Contains(got, "COM3") {
		t.Fatalf("expected device:\n%s", got)
	}
	if !strings.Contains(got, "115200") {
		t.Fatalf("expected baud:\n%s", got)
	}
	if strings.Contains(got, "run_ssh_command is available") {
		t.Fatalf("serial must not enable SSH exec blurb:\n%s", got)
	}
	if strings.Contains(got, "Remote system info") {
		t.Fatalf("serial must not include remote info:\n%s", got)
	}
}

func TestBuildSystemPromptWithSerialBoundNoDevice(t *testing.T) {
	got := BuildSystemPromptOpts(PromptOptions{
		Serial:     &SerialPromptContext{},
		UILanguage: "en",
	})
	if !strings.Contains(got, "protocol=serial") {
		t.Fatalf("bound serial must still set protocol=serial:\n%s", got)
	}
	if strings.Contains(got, "No session bound") {
		t.Fatalf("must not treat empty device as unbound:\n%s", got)
	}
	if strings.Contains(got, "run_ssh_command is available") {
		t.Fatalf("serial must not enable SSH exec blurb:\n%s", got)
	}
}

func TestBuildSystemPromptOptsFSAndSearch(t *testing.T) {
	got := BuildSystemPromptOpts(PromptOptions{
		EnableFS:        true,
		AllowDirs:       []string{`D:\data`},
		EnableWebSearch: true,
		UILanguage:      "en",
	})
	if !strings.Contains(got, "read_local_file") {
		t.Fatalf("expected FS tools in prompt:\n%s", got)
	}
	if !strings.Contains(got, `D:\data`) {
		t.Fatalf("expected allow dir in prompt:\n%s", got)
	}
	if !strings.Contains(got, "File tools may only use paths under these directories") {
		t.Fatalf("expected allowlist rule:\n%s", got)
	}
	if !strings.Contains(got, "web_search") {
		t.Fatalf("expected web_search in prompt:\n%s", got)
	}
	off := BuildSystemPromptOpts(PromptOptions{UILanguage: "en"})
	if strings.Contains(off, "Local file allowlist") {
		t.Fatalf("unexpected allowlist when FS off:\n%s", off)
	}
}

func TestFormatRemoteOS(t *testing.T) {
	t.Parallel()
	if got := formatRemoteOS("Ubuntu", "ubuntu", "22.04"); got != "Ubuntu (ubuntu 22.04)" {
		t.Fatalf("got %q", got)
	}
	if got := formatRemoteOS("Ubuntu", "", ""); got != "Ubuntu" {
		t.Fatalf("got %q", got)
	}
	if got := formatRemoteOS("", "ubuntu", "22.04"); got != "ubuntu 22.04" {
		t.Fatalf("got %q", got)
	}
	if got := formatRemoteOS("", "", ""); got != "" {
		t.Fatalf("got %q", got)
	}
}

func TestBuildSystemPromptCurrentPlanLastSection(t *testing.T) {
	plan := `{"title":"T","tasks":[{"id":"1","title":"A","status":"pending"}]}`
	got := BuildSystemPromptOpts(PromptOptions{PlanJSON: plan, UILanguage: "en"})
	idxPlan := strings.Index(got, "## Current plan")
	if idxPlan < 0 {
		t.Fatalf("expected Current plan:\n%s", got)
	}
	if !strings.Contains(got, "[pending] A (id=1)") {
		t.Fatalf("expected task line:\n%s", got)
	}
}

func TestBuildSystemPromptEmptyOrInvalidPlanSkipped(t *testing.T) {
	if strings.Contains(BuildSystemPromptOpts(PromptOptions{UILanguage: "en"}), "## Current plan") {
		t.Fatal("empty plan")
	}
	if strings.Contains(BuildSystemPromptOpts(PromptOptions{PlanJSON: `{`, UILanguage: "en"}), "## Current plan") {
		t.Fatal("invalid plan")
	}
	if strings.Contains(BuildSystemPromptOpts(PromptOptions{PlanJSON: `{"title":"x","tasks":[]}`, UILanguage: "en"}), "## Current plan") {
		t.Fatal("empty tasks")
	}
}

func TestBuildSystemPromptScopeLimitsToOperations(t *testing.T) {
	got := BuildSystemPromptOpts(PromptOptions{UILanguage: "en"})
	for _, want := range []string{
		"Only answer operations-related questions",
		"unrelated to operations",
		"do not use tools",
		"cannot answer",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected scope wording %q in prompt:\n%s", want, got)
		}
	}
}

func TestFallbackSystemPromptCoversHardRules(t *testing.T) {
	got := fallbackSystemPrompt(PromptOptions{
		EnableSSH: true, EnableFS: true, EnableWebSearch: true,
	}, "zh-CN")
	for _, want := range []string{
		"Respond in the user's UI language: zh-CN",
		"Only answer operations-related questions",
		"do not use tools",
		"cannot answer",
		"run_ssh_command",
		"read_local_file",
		"web_search",
		"passwords",
		"credentials",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected fallback wording %q:\n%s", want, got)
		}
	}
	off := fallbackSystemPrompt(PromptOptions{}, "en")
	if strings.Contains(off, "run_ssh_command") || strings.Contains(off, "web_search") || strings.Contains(off, "read_local_file") {
		t.Fatalf("disabled tools must stay out of fallback:\n%s", off)
	}
}

func TestBuildSystemPromptSecurityBansSecrets(t *testing.T) {
	got := BuildSystemPromptOpts(PromptOptions{UILanguage: "en"})
	for _, want := range []string{
		"Never request, output, or guess passwords",
		"API keys",
		"credentials",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected security wording %q in prompt:\n%s", want, got)
		}
	}
}
