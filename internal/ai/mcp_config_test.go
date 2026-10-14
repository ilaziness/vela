package ai

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestMCPServerEntryValidate(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		entry   MCPServerEntry
		wantErr bool
	}{
		{"ok stdio inferred", MCPServerEntry{Name: "a", Command: "uvx"}, false},
		{"ok http inferred", MCPServerEntry{Name: "a", URL: "https://x"}, false},
		{"ok stdio typed", MCPServerEntry{Name: "a", Type: "stdio", Command: "uvx"}, false},
		{"ok http typed", MCPServerEntry{Name: "a", Type: "http", URL: "https://x"}, false},
		{"missing name", MCPServerEntry{Command: "x"}, true},
		{"missing command", MCPServerEntry{Name: "a", Type: "stdio"}, true},
		{"missing url", MCPServerEntry{Name: "a", Type: "http"}, true},
		{"neither command nor url", MCPServerEntry{Name: "a"}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.entry.validate()
			if tc.wantErr && err == nil {
				t.Fatal("expected error")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected: %v", err)
			}
		})
	}
}

func TestLoadSaveMCPConfig(t *testing.T) {
	dir := t.TempDir()
	path, err := EnsureMCPConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(path) != mcpConfigFileName {
		t.Fatalf("path=%s", path)
	}
	cfg, err := LoadMCPConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Entries()) != 0 {
		t.Fatalf("servers=%d", len(cfg.Entries()))
	}
	en := true
	cfg.MCPServers = map[string]MCPServerEntry{
		"demo": {
			Enabled: &en,
			Command: "echo",
			Env:     map[string]string{"FOO": "bar"},
		},
	}
	if err := writeMCPConfig(MCPConfigPath(dir), cfg); err != nil {
		t.Fatal(err)
	}
	if err := SaveMCPEnabled(dir, "demo", false); err != nil {
		t.Fatal(err)
	}
	cfg2, err := LoadMCPConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	entries := cfg2.Entries()
	if len(entries) != 1 || entries[0].IsEnabled() {
		t.Fatalf("enabled still true: %+v", entries)
	}
	if entries[0].Name != "demo" || entries[0].Env["FOO"] != "bar" {
		t.Fatalf("entry: %+v", entries[0])
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(MCPConfigPath(dir))
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm()&0o077 != 0 {
			t.Fatalf("mcp.json should not be group/world accessible: %v", info.Mode())
		}
	}
}

func TestSaveMCPEnabledMissing(t *testing.T) {
	dir := t.TempDir()
	if _, err := EnsureMCPConfig(dir); err != nil {
		t.Fatal(err)
	}
	if err := SaveMCPEnabled(dir, "nope", true); err == nil {
		t.Fatal("expected not found")
	}
}

func TestMCPNamespacedToolName(t *testing.T) {
	t.Parallel()
	got := mcpNamespacedToolName("mui-mcp", "fetchDocs")
	want := "mcp__mui-mcp__fetchDocs"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	got = mcpNamespacedToolName("my server", "a.b/c")
	want = "mcp__my_server__a_b_c"
	if got != want {
		t.Fatalf("sanitize: got %q want %q", got, want)
	}
}

func TestMCPServerEntryEnvSlice(t *testing.T) {
	t.Parallel()
	e := MCPServerEntry{Env: map[string]string{"B": "2", "A": "1"}}
	got := e.EnvSlice()
	if len(got) != 2 || got[0] != "A=1" || got[1] != "B=2" {
		t.Fatalf("got %#v", got)
	}
}
