package bookmark

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestImportOpenSSHProxyJump(t *testing.T) {
	svc, _ := testBookmarkService(t, "", "")
	if err := svc.ensureGroup(ImportedGroupName); err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	cfg := filepath.Join(dir, "config")
	content := `
Host jump
  HostName 10.0.0.1
  User j
  Port 22

Host target
  HostName 10.0.0.2
  User t
  ProxyJump jump
  SetEnv FOO=bar
  SetEnv TERM=vt100
  RemoteCommand cd /data
`
	if err := os.WriteFile(cfg, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	res, err := svc.ImportOpenSSH(cfg, ImportedGroupName)
	if err != nil {
		t.Fatal(err)
	}
	if res.Created < 2 {
		t.Fatalf("created=%d warnings=%v", res.Created, res.Warnings)
	}
	items, err := svc.ListItems()
	if err != nil {
		t.Fatal(err)
	}
	var jumpID, targetID string
	for _, it := range items {
		if it.GroupName != ImportedGroupName {
			continue
		}
		switch it.Title {
		case "jump":
			jumpID = it.ID
		case "target":
			targetID = it.ID
		}
	}
	if jumpID == "" || targetID == "" {
		t.Fatalf("missing bookmarks: %+v", items)
	}
	target, err := svc.Get(targetID)
	if err != nil {
		t.Fatal(err)
	}
	if target.ProxyJumpID != jumpID {
		t.Fatalf("proxy_jump_id=%q want %q warnings=%v", target.ProxyJumpID, jumpID, res.Warnings)
	}
	if target.StartupCmd != "cd /data" {
		t.Fatalf("startup=%q", target.StartupCmd)
	}
	if target.Term != "vt100" {
		t.Fatalf("term=%q", target.Term)
	}
	env, err := ParseEnvVars(target.EnvVars)
	if err != nil || env["FOO"] != "bar" {
		t.Fatalf("env=%v err=%v", target.EnvVars, err)
	}
	if _, ok := env["TERM"]; ok {
		t.Fatalf("TERM should not be in env_vars: %v", env)
	}
	if len(target.Tags) != 0 || target.Color != "" {
		t.Fatalf("import should not set tags/color: tags=%v color=%q", target.Tags, target.Color)
	}

	out, warns, err := svc.ExportOpenSSH(ImportedGroupName)
	if err != nil {
		t.Fatal(err)
	}
	if out == "" {
		t.Fatalf("empty export warns=%v", warns)
	}
	for _, part := range []string{"Host target", "ProxyJump jump", `RemoteCommand "cd /data"`, "SetEnv TERM=vt100", "SetEnv FOO=bar"} {
		if !strings.Contains(out, part) {
			t.Fatalf("missing %q in export:\n%s", part, out)
		}
	}
}
