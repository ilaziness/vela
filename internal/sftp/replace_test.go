package sftp

import (
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	pkgsftp "github.com/pkg/sftp"
	"go.uber.org/zap"
)

type fakeInfo struct {
	dir bool
}

func (f fakeInfo) Name() string       { return "f" }
func (f fakeInfo) Size() int64        { return 1 }
func (f fakeInfo) Mode() os.FileMode  { return 0 }
func (f fakeInfo) ModTime() time.Time { return time.Time{} }
func (f fakeInfo) IsDir() bool        { return f.dir }
func (f fakeInfo) Sys() any           { return nil }

type fakeReplacer struct {
	hasPosix  bool
	posixErr  error
	renameErr error
	missing   bool
	dir       bool
	statErr   error
	gone      map[string]bool
	ops       []string
}

func (f *fakeReplacer) HasExtension(name string) (string, bool) {
	if name == posixRenameExt && f.hasPosix {
		return "1", true
	}
	return "", false
}

func (f *fakeReplacer) PosixRename(oldname, newname string) error {
	f.ops = append(f.ops, "posix:"+oldname+"->"+newname)
	return f.posixErr
}

func (f *fakeReplacer) Rename(oldname, newname string) error {
	f.ops = append(f.ops, "rename:"+oldname+"->"+newname)
	return f.renameErr
}

func (f *fakeReplacer) Stat(p string) (os.FileInfo, error) {
	f.ops = append(f.ops, "stat:"+p)
	if f.statErr != nil {
		return nil, f.statErr
	}
	if f.missing || f.gone[p] {
		return nil, os.ErrNotExist
	}
	return fakeInfo{dir: f.dir}, nil
}

func (f *fakeReplacer) Remove(path string) error {
	f.ops = append(f.ops, "remove:"+path)
	if f.gone == nil {
		f.gone = map[string]bool{}
	}
	f.gone[path] = true
	return nil
}

func TestReplaceUsesPosixRenameWithoutRemovingOriginal(t *testing.T) {
	client := &fakeReplacer{hasPosix: true}
	mgr := NewManager(zap.NewNop(), nil, nil, nil)
	if err := mgr.replaceStagedFile(client, "/tmp/.a.txt.vela-edit", "/tmp/a.txt"); err != nil {
		t.Fatal(err)
	}
	if strings.Join(client.ops, ",") != "posix:/tmp/.a.txt.vela-edit->/tmp/a.txt" {
		t.Fatalf("ops=%v", client.ops)
	}
}

func TestReplacePosixFailureKeepsOriginal(t *testing.T) {
	client := &fakeReplacer{hasPosix: true, posixErr: errors.New("busy")}
	mgr := NewManager(zap.NewNop(), nil, nil, nil)
	err := mgr.replaceStagedFile(client, "/tmp/.a.txt.vela-edit", "/tmp/a.txt")
	if err == nil {
		t.Fatal("expected error")
	}
	joined := strings.Join(client.ops, ",")
	if strings.Contains(joined, "remove:/tmp/a.txt") {
		t.Fatalf("original removed: %v", client.ops)
	}
	if !strings.Contains(joined, "remove:/tmp/.a.txt.vela-edit") {
		t.Fatalf("staging not removed: %v", client.ops)
	}
}

func TestReplaceFallsBackWhenPosixUnsupported(t *testing.T) {
	client := &fakeReplacer{
		hasPosix: true,
		posixErr: &pkgsftp.StatusError{Code: uint32(pkgsftp.ErrSSHFxOpUnsupported)},
	}
	mgr := NewManager(zap.NewNop(), nil, nil, nil)
	if err := mgr.replaceStagedFile(client, "/tmp/.a.txt.vela-edit", "/tmp/a.txt"); err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(client.ops, ",")
	if !strings.Contains(joined, "remove:/tmp/a.txt") || !strings.Contains(joined, "rename:") {
		t.Fatalf("ops=%v", client.ops)
	}
}

func TestReplaceRemoveThenRenameFailureIsExplicit(t *testing.T) {
	client := &fakeReplacer{renameErr: errors.New("denied")}
	mgr := NewManager(zap.NewNop(), nil, nil, nil)
	err := mgr.replaceStagedFile(client, "/tmp/.a.txt.vela-edit", "/tmp/a.txt")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "was removed") || !strings.Contains(err.Error(), "/tmp/.a.txt.vela-edit") {
		t.Fatalf("error=%v", err)
	}
	joined := strings.Join(client.ops, ",")
	if strings.Contains(joined, "remove:/tmp/.a.txt.vela-edit") {
		t.Fatalf("staging deleted after original was removed: %v", client.ops)
	}
}

func TestReplaceRejectsDirectoryBeforeRemove(t *testing.T) {
	client := &fakeReplacer{dir: true}
	mgr := NewManager(zap.NewNop(), nil, nil, nil)
	err := mgr.replaceStagedFile(client, "/tmp/.a.vela-edit", "/tmp/a")
	if err == nil || !strings.Contains(err.Error(), "directory") {
		t.Fatalf("error=%v", err)
	}
	if strings.Contains(strings.Join(client.ops, ","), "remove:/tmp/a,") || strings.HasSuffix(strings.Join(client.ops, ","), "remove:/tmp/a") {
		t.Fatalf("directory removed: %v", client.ops)
	}
}
