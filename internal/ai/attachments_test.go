package ai

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPersistAndResolveAttachment(t *testing.T) {
	dir := t.TempDir()
	sessionID := "sess-1"
	png := []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a}
	dataURL := EncodeDataURL("image/png", png)
	n := 0
	newID := func() string {
		n++
		return fmt.Sprintf("att-%d", n)
	}

	parts, err := PersistUserAttachments(dir, sessionID, "see image", []ChatAttachmentInput{
		{DataURL: dataURL, Filename: "a.png"},
		{DataURL: EncodeDataURL("text/plain", []byte("hello")), Filename: "note.txt"},
	}, newID)
	if err != nil {
		t.Fatal(err)
	}
	if len(parts) != 3 {
		t.Fatalf("parts=%d", len(parts))
	}
	if !parts[0].IsText() || parts[0].Text != "see image" {
		t.Fatalf("text part: %+v", parts[0])
	}
	if !parts[1].IsMedia() || !strings.HasPrefix(parts[1].Text, AttURIPrefix) {
		t.Fatalf("media part: %+v", parts[1])
	}
	if !parts[2].IsMedia() || !strings.HasPrefix(parts[2].Text, AttURIPrefix) {
		t.Fatalf("text file media part: %+v", parts[2])
	}
	if !strings.HasSuffix(parts[2].Text, ".txt") {
		t.Fatalf("text file should keep extension: %s", parts[2].Text)
	}

	resolved, err := ResolvePartsForModel(dir, parts)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(resolved[1].Text, "data:image/png;base64,") {
		t.Fatalf("resolved image=%q", resolved[1].Text)
	}
	if !resolved[2].IsText() || !strings.Contains(resolved[2].Text, "note.txt") || !strings.Contains(resolved[2].Text, "hello") {
		t.Fatalf("resolved text file: %+v", resolved[2])
	}

	dataURL, err = AttachmentDataURL(dir, parts[1].Text, "image/png")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(dataURL, "data:image/png;base64,") {
		t.Fatalf("preview data url: %s", dataURL)
	}

	if err := RemoveSessionAttachments(dir, sessionID); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(sessionAttachmentsDir(dir, sessionID)); !os.IsNotExist(err) {
		t.Fatalf("session dir still exists: %v", err)
	}
}

func TestPersistPathAttachmentDoesNotCopy(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "orig.md")
	if err := os.WriteFile(src, []byte("# hi"), 0o600); err != nil {
		t.Fatal(err)
	}
	parts, err := PersistUserAttachments(dir, "sess-2", "see md", []ChatAttachmentInput{
		{Path: src, Filename: "orig.md", MIME: "text/markdown"},
	}, func() string { return "unused" })
	if err != nil {
		t.Fatal(err)
	}
	if len(parts) != 2 || !parts[1].IsMedia() {
		t.Fatalf("parts=%+v", parts)
	}
	if !strings.HasPrefix(parts[1].Text, PathURIPrefix) {
		t.Fatalf("want path uri, got %s", parts[1].Text)
	}
	if _, err := os.Stat(sessionAttachmentsDir(dir, "sess-2")); !os.IsNotExist(err) {
		t.Fatalf("should not copy into user data dir: %v", err)
	}
	resolved, err := ResolvePartsForModel(dir, parts)
	if err != nil {
		t.Fatal(err)
	}
	if !resolved[1].IsText() || !strings.Contains(resolved[1].Text, "# hi") {
		t.Fatalf("resolved: %+v", resolved[1])
	}
	if err := os.Remove(src); err != nil {
		t.Fatal(err)
	}
	if _, err := ResolvePartsForModel(dir, parts); err == nil {
		t.Fatal("expected missing file error")
	}
}

func TestPersistRejectsEmptyText(t *testing.T) {
	_, err := PersistUserAttachments(t.TempDir(), "s", "  ", nil, func() string { return "x" })
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestParseAttURI(t *testing.T) {
	sid, id, ok := ParseAttURI(FormatAttURI("s1", "a1"))
	if !ok || sid != "s1" || id != "a1" {
		t.Fatalf("%q %q %v", sid, id, ok)
	}
	if _, _, ok := ParseAttURI("http://x"); ok {
		t.Fatal("expected fail")
	}
	sid, id, ok = ParseAttURI(FormatAttURI("s1", "uuid.md"))
	if !ok || sid != "s1" || id != "uuid.md" {
		t.Fatalf("ext id: %q %q %v", sid, id, ok)
	}
}
