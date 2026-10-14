package ai

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseSkillFrontmatter(t *testing.T) {
	t.Parallel()
	fm := parseSkillFrontmatter([]byte("---\nname: x\ndescription: Hello skill\n---\n\n# Body\n"))
	if fm.Description != "Hello skill" {
		t.Fatalf("desc=%q", fm.Description)
	}
	empty := parseSkillFrontmatter([]byte("# no frontmatter\n"))
	if empty.Description != "" {
		t.Fatalf("expected empty, got %q", empty.Description)
	}
}

func TestListSkillsAndSetEnabled(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, skillsDirName, "demo-skill")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := "---\ndescription: Demo desc\n---\n\n# Demo\n"
	if err := os.WriteFile(filepath.Join(skillDir, skillMarkdownName), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	list, err := ListSkills(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].Name != "demo-skill" || !list[0].Enabled {
		t.Fatalf("list=%+v", list)
	}
	if list[0].Description != "Demo desc" {
		t.Fatalf("desc=%q", list[0].Description)
	}

	if err := SetSkillEnabled(dir, "demo-skill", false); err != nil {
		t.Fatal(err)
	}
	list, err = ListSkills(dir)
	if err != nil {
		t.Fatal(err)
	}
	if list[0].Enabled {
		t.Fatal("expected disabled")
	}
	paths, err := EnabledSkillPaths(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 0 {
		t.Fatalf("paths=%v", paths)
	}

	if err := SetSkillEnabled(dir, "demo-skill", true); err != nil {
		t.Fatal(err)
	}
	paths, err = EnabledSkillPaths(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 1 {
		t.Fatalf("paths=%v", paths)
	}

	if err := SetSkillEnabled(dir, "missing", false); err == nil {
		t.Fatal("expected error for missing skill")
	}
	if err := SetSkillEnabled(dir, "../x", false); err == nil {
		t.Fatal("expected error for invalid name")
	}
}
