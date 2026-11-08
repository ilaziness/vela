package ssh

import (
	"bytes"
	"testing"
	"time"
)

func TestFilterEnv(t *testing.T) {
	out := filterEnv(map[string]string{"FOO": "1", "TERM": "xterm", "": "x"})
	if out["FOO"] != "1" {
		t.Fatalf("%v", out)
	}
	if _, ok := out["TERM"]; ok {
		t.Fatal("TERM should be stripped")
	}
	if filterEnv(nil) != nil {
		t.Fatal("nil env")
	}
}

func TestBuildStdinBootstrapEmpty(t *testing.T) {
	if buildStdinBootstrap(nil, "") != "" {
		t.Fatal("expected empty")
	}
	if buildStdinBootstrap(map[string]string{"TERM": "x"}, "  ") != "" {
		t.Fatal("TERM-only and blank startup should be empty")
	}
}

func TestBuildStdinBootstrapEnvAndStartup(t *testing.T) {
	got := buildStdinBootstrap(map[string]string{"abc": "123", "Z": "z"}, "pwd && whoami")
	want := "export 'Z'='z'; export 'abc'='123'; pwd && whoami"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestBashSingleQuote(t *testing.T) {
	if bashSingleQuote(`a'b`) != `'a'\''b'` {
		t.Fatal(bashSingleQuote(`a'b`))
	}
}

func TestWriteStdinLine(t *testing.T) {
	var buf bytes.Buffer
	if err := writeStdinLine(&buf, "cd /tmp"); err != nil {
		t.Fatal(err)
	}
	if buf.String() != "cd /tmp\n" {
		t.Fatalf("%q", buf.String())
	}
	if err := writeStdinLine(nil, "x"); err == nil {
		t.Fatal("expected error")
	}
}

func TestWaitShellReadyQuiet(t *testing.T) {
	sc := &Session{}
	sc.noteOutput()
	start := time.Now()
	sc.waitShellReady(2 * time.Second)
	elapsed := time.Since(start)
	if elapsed < shellReadyQuiet {
		t.Fatalf("returned too early: %v", elapsed)
	}
	if elapsed > shellReadyQuiet+200*time.Millisecond {
		t.Fatalf("took too long: %v", elapsed)
	}
}

func TestDefaultTermType(t *testing.T) {
	if defaultTermType != "xterm-256color" {
		t.Fatal(defaultTermType)
	}
}
