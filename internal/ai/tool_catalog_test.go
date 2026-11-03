package ai

import (
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"

	"go.uber.org/zap"

	"github.com/ilaziness/vela/internal/ai/tool"
)

func TestPublishBuiltinDoesNotTouchMCPTools(t *testing.T) {
	e := NewAIEngine(zap.NewNop())
	e.SetUserDataDir(t.TempDir())

	// Seed skills via publish path (no real MCP).
	e.catalogMu.Lock()
	e.storeSnapLocked(CatalogSnapshot{
		Skills: []EnabledSkill{
			{Name: "keep-me", Description: "d", Path: "/tmp/keep"},
		},
	})
	e.catalogMu.Unlock()

	dir := t.TempDir()
	cfg := &Config{
		Filesystem: FileSystemConfig{AllowedDirs: []string{dir}},
		WebSearch:  WebSearchConfig{Provider: tool.SearchTavily, APIKeyEncrypted: "cipher"},
	}
	e.PublishBuiltinFromConfig(cfg)

	snap := e.Snapshot()
	if !snap.EnableFS || len(snap.AllowDirs) != 1 {
		t.Fatalf("builtin snap=%+v", snap)
	}
	if !snap.EnableWebSearch {
		t.Fatal("expected EnableWebSearch from encrypted key")
	}
	if len(snap.Skills) != 1 || snap.Skills[0].Name != "keep-me" {
		t.Fatalf("skills should be untouched: %+v", snap.Skills)
	}
}

func TestPublishSkillsDoesNotClearBuiltin(t *testing.T) {
	root := t.TempDir()
	skillDir := filepath.Join(root, skillsDirName, "demo")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := "---\ndescription: Demo\n---\n\n# Body\n"
	if err := os.WriteFile(filepath.Join(skillDir, skillMarkdownName), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	e := NewAIEngine(zap.NewNop())
	e.SetUserDataDir(root)
	allow := t.TempDir()
	e.PublishBuiltinFromConfig(&Config{
		Filesystem: FileSystemConfig{AllowedDirs: []string{allow}},
	})
	if err := e.PublishSkills(); err != nil {
		t.Fatal(err)
	}

	snap := e.Snapshot()
	if !snap.EnableFS || len(snap.AllowDirs) != 1 {
		t.Fatalf("builtin cleared: %+v", snap)
	}
	if len(snap.Skills) != 1 || snap.Skills[0].Name != "demo" {
		t.Fatalf("skills=%+v", snap.Skills)
	}
	if snap.Skills[0].Description != "Demo" {
		t.Fatalf("desc=%q", snap.Skills[0].Description)
	}
}

func TestSnapshotIsolationConcurrent(t *testing.T) {
	e := NewAIEngine(zap.NewNop())
	dir1 := t.TempDir()
	dir2 := t.TempDir()
	e.PublishBuiltinFromConfig(&Config{Filesystem: FileSystemConfig{AllowedDirs: []string{dir1}}})
	sanitized2 := (&Config{Filesystem: FileSystemConfig{AllowedDirs: []string{dir2}}}).EffectiveAllowedDirs()
	if len(sanitized2) != 1 {
		t.Fatalf("sanitize dir2: %v", sanitized2)
	}

	var wg sync.WaitGroup
	var sawComplete atomic.Int32
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s := e.Snapshot()
			if len(s.AllowDirs) == 1 && s.EnableFS {
				sawComplete.Add(1)
			}
		}()
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		e.PublishBuiltinFromConfig(&Config{Filesystem: FileSystemConfig{AllowedDirs: []string{dir2}}})
	}()
	wg.Wait()

	final := e.Snapshot()
	if len(final.AllowDirs) != 1 || final.AllowDirs[0] != sanitized2[0] {
		t.Fatalf("final=%+v want %q", final, sanitized2[0])
	}
	if sawComplete.Load() == 0 {
		t.Fatal("expected readers to observe at least one complete snapshot")
	}
}

func TestClearMCPToolsPreservesSkills(t *testing.T) {
	e := NewAIEngine(zap.NewNop())
	e.catalogMu.Lock()
	e.storeSnapLocked(CatalogSnapshot{
		Skills:    []EnabledSkill{{Name: "a", Path: "/a"}},
		EnableFS:  true,
		AllowDirs: []string{"/x"},
	})
	e.catalogMu.Unlock()
	e.clearMCPTools()
	snap := e.Snapshot()
	if len(snap.Skills) != 1 || !snap.EnableFS {
		t.Fatalf("snap=%+v", snap)
	}
	if len(snap.MCPTools) != 0 {
		t.Fatalf("mcp tools=%v", snap.MCPTools)
	}
}

func TestSetConfigDisableKeepsBuiltinCatalog(t *testing.T) {
	dir := t.TempDir()
	e := NewAIEngine(zap.NewNop())
	e.SetUserDataDir(dir)
	e.PublishBuiltinFromConfig(&Config{
		Filesystem: FileSystemConfig{AllowedDirs: []string{dir}},
		WebSearch:  WebSearchConfig{Provider: tool.SearchTavily, APIKeyEncrypted: "x"},
	})
	if err := e.SetConfig(&Config{
		Enabled:    false,
		Filesystem: FileSystemConfig{AllowedDirs: []string{dir}},
		WebSearch:  WebSearchConfig{Provider: tool.SearchTavily, APIKeyEncrypted: "x"},
	}); err != nil {
		t.Fatal(err)
	}
	snap := e.Snapshot()
	if !snap.EnableFS || !snap.EnableWebSearch || len(snap.MCPTools) != 0 {
		t.Fatalf("snap after disable=%+v", snap)
	}
}
