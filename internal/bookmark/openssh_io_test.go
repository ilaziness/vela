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

func TestExportOpenSSHSkipsSerialWithWarning(t *testing.T) {
	svc, _ := testBookmarkService(t, "", "")
	if _, err := svc.Save(Bookmark{
		GroupName: DefaultGroupName, Title: "com1", Protocol: "serial",
		SerialDevice: "COM1", BaudRate: 9600,
	}); err != nil {
		t.Fatal(err)
	}
	out, warns, err := svc.ExportOpenSSH(DefaultGroupName)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "Host com1") {
		t.Fatalf("serial bookmark exported:\n%s", out)
	}
	found := false
	for _, w := range warns {
		if strings.Contains(w, "skipped non-SSH bookmark") {
			found = true
		}
	}
	if !found {
		t.Fatalf("missing non-SSH warning: %v", warns)
	}
}

func TestExportOpenSSHUniqueAliases(t *testing.T) {
	svc, _ := testBookmarkService(t, "", "")
	if err := svc.AddGroup("Prod"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Save(Bookmark{
		GroupName: DefaultGroupName, Title: "web", Protocol: "ssh",
		Host: "10.0.0.1", Port: 22, User: "u",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Save(Bookmark{
		GroupName: "Prod", Title: "web", Protocol: "ssh",
		Host: "10.0.0.2", Port: 22, User: "u",
	}); err != nil {
		t.Fatal(err)
	}

	out, warns, err := svc.ExportOpenSSH("")
	if err != nil {
		t.Fatal(err)
	}
	// Titles are unique per group only; exporting both groups must not emit
	// two identical Host blocks.
	if !strings.Contains(out, "Host web\n") || !strings.Contains(out, "Host web-2\n") {
		t.Fatalf("expected unique aliases:\n%s\nwarns=%v", out, warns)
	}
	if !strings.Contains(out, "HostName 10.0.0.1") || !strings.Contains(out, "HostName 10.0.0.2") {
		t.Fatalf("missing host entries:\n%s", out)
	}
}

func TestExportOpenSSHIncludesCrossGroupJump(t *testing.T) {
	svc, _ := testBookmarkService(t, "", "")
	if err := svc.AddGroup("Infra"); err != nil {
		t.Fatal(err)
	}
	jumpID, err := svc.Save(Bookmark{
		GroupName: "Infra", Title: "bastion", Protocol: "ssh",
		Host: "10.1.0.1", Port: 22, User: "j",
	})
	if err != nil {
		t.Fatal(err)
	}
	targetID, err := svc.Save(Bookmark{
		GroupName: DefaultGroupName, Title: "app", Protocol: "ssh",
		Host: "10.2.0.1", Port: 22, User: "a",
	})
	if err != nil {
		t.Fatal(err)
	}
	target, err := svc.Get(targetID)
	if err != nil {
		t.Fatal(err)
	}
	target.ProxyJumpID = jumpID
	if _, err := svc.Save(*target); err != nil {
		t.Fatal(err)
	}

	out, warns, err := svc.ExportOpenSSH(DefaultGroupName)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Host bastion\n") {
		t.Fatalf("jump host not included:\n%s\nwarns=%v", out, warns)
	}
	if !strings.Contains(out, "ProxyJump bastion") {
		t.Fatalf("missing ProxyJump:\n%s", out)
	}
}
