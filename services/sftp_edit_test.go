package services

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"
)

func TestMtimeUnchangedDoesNotPrompt(t *testing.T) {
	base := time.Unix(1_700_000_000, 0)
	var gate mtimeGate
	now := base.Add(time.Second)
	if gate.observe(now, base, base, editMtimeStable) {
		t.Fatal("unchanged mtime prompted")
	}
	if gate.observe(now.Add(2*time.Second), base, base, editMtimeStable) {
		t.Fatal("unchanged mtime prompted on a later poll")
	}
}

func TestMtimeStablePrompts(t *testing.T) {
	base := time.Unix(1_700_000_000, 0)
	changed := base.Add(time.Minute)
	var gate mtimeGate
	start := base.Add(time.Second)
	if gate.observe(start, changed, base, editMtimeStable) {
		t.Fatal("new mtime prompted before it was stable")
	}
	if !gate.observe(start.Add(editMtimeStable), changed, base, editMtimeStable) {
		t.Fatal("stable mtime did not prompt")
	}
}

func TestEditConfirmMentionsRemoteChange(t *testing.T) {
	plain := editConfirmMessage("/var/a.txt", false)
	if !strings.Contains(plain, "a.txt") || strings.Contains(plain, "不一致") {
		t.Fatalf("plain message = %q", plain)
	}
	changed := editConfirmMessage("/var/a.txt", true)
	if !strings.Contains(changed, "大小或修改时间") {
		t.Fatalf("changed message = %q", changed)
	}
}

func TestNextEditBaselineKeepsUploadStart(t *testing.T) {
	triggered := time.Unix(1_700_000_000, 0)
	started := triggered
	later := started.Add(2 * time.Second)
	got := nextEditBaseline(triggered, started, true)
	if !got.Equal(started) || !later.After(got) {
		t.Fatalf("baseline=%s later=%s", got, later)
	}
	if nextEditBaseline(triggered, started, false) != triggered {
		t.Fatal("failed upload should keep the prompted mtime")
	}
}

func TestDialogDismissIsNo(t *testing.T) {
	if choiceFromButton(false) != editNo || choiceFromButton(true) != editYes {
		t.Fatal("button result did not map to a choice")
	}
}

func TestEditSessionDirRejectsTraversal(t *testing.T) {
	for _, id := range []string{"", "..", "../x", `a\b`, "a/b"} {
		if _, ok := editSessionDir(id); ok {
			t.Fatalf("accepted %q", id)
		}
	}
	dir, ok := editSessionDir("abc123")
	if !ok {
		t.Fatal("hex session id rejected")
	}
	root := filepath.Join(os.TempDir(), editTempRoot)
	rel, err := filepath.Rel(root, dir)
	if err != nil || strings.HasPrefix(rel, "..") {
		t.Fatalf("dir=%s rel=%s err=%v", dir, rel, err)
	}
}

func TestRemoveEditKeepsReplacement(t *testing.T) {
	book := newEditBook()
	old := &remoteEdit{sessionID: "s", remote: "/a", local: "old"}
	if action, _ := book.begin(old); action != beginStarted {
		t.Fatalf("begin old = %v", action)
	}
	neu := &remoteEdit{sessionID: "s", remote: "/a", local: "new", downloading: true}
	if action, _ := book.begin(neu); action != beginStarted {
		t.Fatalf("begin replacement = %v", action)
	}
	book.remove(old)
	got := book.snapshot("s")
	if len(got) != 1 || got[0] != neu {
		t.Fatalf("replacement missing: %+v", got)
	}
}

func TestCleanupSessionEditDirWaitsThenDeletes(t *testing.T) {
	s := &SftpService{logger: zap.NewNop(), edits: newEditBook()}
	id := "sesseditcleanup"
	dir, ok := editSessionDir(id)
	if !ok {
		t.Fatal("session dir")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	s.edits.setClosingDone(id, []chan struct{}{done})
	finished := make(chan struct{})
	go func() {
		s.CleanupSessionEditDir(id)
		close(finished)
	}()
	select {
	case <-finished:
		t.Fatal("cleanup returned before the edit goroutine finished")
	case <-time.After(50 * time.Millisecond):
	}
	close(done)
	select {
	case <-finished:
	case <-time.After(time.Second):
		t.Fatal("cleanup did not return")
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("temp dir remains: %v", err)
	}
}

func TestSanitizeEditBaseKeepsExtension(t *testing.T) {
	got := sanitizeEditBase("/remote/my file.txt")
	if !strings.HasSuffix(got, ".txt") {
		t.Fatalf("extension lost: %q", got)
	}
	if strings.ContainsAny(got, `<>:"/\|?*`) {
		t.Fatalf("illegal character remained: %q", got)
	}
}
