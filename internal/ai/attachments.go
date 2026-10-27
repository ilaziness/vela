package ai

import (
	"encoding/base64"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	genkitAI "github.com/firebase/genkit/go/ai"
)

const (
	// AttURIPrefix is the scheme for pasted files copied into the user data dir.
	AttURIPrefix = "vela-att://"
	// PathURIPrefix stores the original local path of picker/drop attachments (not copied).
	PathURIPrefix = "vela-path:"

	MaxAttachBytes = 5 << 20 // 5 MiB
	MaxAttachCount = 5

	metaFilename = "filename"
)

var attachMIMEAllow = map[string]struct{}{
	"image/png":       {},
	"image/jpeg":      {},
	"image/webp":      {},
	"image/gif":       {},
	"application/pdf": {},
	"text/plain":      {},
	"text/markdown":   {},
	"text/csv":        {},
}

func allowedAttachMIME(mime string) bool {
	_, ok := attachMIMEAllow[normalizeMIME(mime)]
	return ok
}

func isTextAttachMIME(mime string) bool {
	switch normalizeMIME(mime) {
	case "text/plain", "text/markdown", "text/csv":
		return true
	default:
		return false
	}
}

func normalizeMIME(mime string) string {
	mime = strings.TrimSpace(strings.ToLower(mime))
	if i := strings.IndexByte(mime, ';'); i >= 0 {
		mime = strings.TrimSpace(mime[:i])
	}
	if mime == "image/jpg" {
		return "image/jpeg"
	}
	return mime
}

func attachmentsRoot(userDataDir string) string {
	return filepath.Join(userDataDir, "ai", "attachments")
}

func sessionAttachmentsDir(userDataDir, sessionID string) string {
	return filepath.Join(attachmentsRoot(userDataDir), sessionID)
}

// FormatAttURI builds vela-att://{sessionID}/{attID}.
func FormatAttURI(sessionID, attID string) string {
	return AttURIPrefix + sessionID + "/" + attID
}

// ParseAttURI parses vela-att://{sessionID}/{attID}.
func ParseAttURI(uri string) (sessionID, attID string, ok bool) {
	if !strings.HasPrefix(uri, AttURIPrefix) {
		return "", "", false
	}
	rest := strings.TrimPrefix(uri, AttURIPrefix)
	sessionID, attID, found := strings.Cut(rest, "/")
	if !found || sessionID == "" || attID == "" || strings.Contains(attID, "/") {
		return "", "", false
	}
	if strings.Contains(sessionID, "..") || strings.Contains(attID, "..") {
		return "", "", false
	}
	return sessionID, attID, true
}

func attachmentPath(userDataDir, sessionID, attID string) (string, error) {
	if userDataDir == "" || sessionID == "" || attID == "" {
		return "", fmt.Errorf("invalid attachment path")
	}
	if strings.Contains(sessionID, "..") || strings.Contains(attID, "..") ||
		strings.ContainsAny(sessionID, `/\`) || strings.ContainsAny(attID, `/\`) {
		return "", fmt.Errorf("invalid attachment id")
	}
	return filepath.Join(sessionAttachmentsDir(userDataDir, sessionID), attID), nil
}

func saveAttachment(userDataDir, sessionID, attID string, data []byte) error {
	path, err := attachmentPath(userDataDir, sessionID, attID)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

func readAttachment(userDataDir, sessionID, attID string) ([]byte, error) {
	path, err := attachmentPath(userDataDir, sessionID, attID)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(path)
}

// RemoveSessionAttachments deletes all files for a session.
func RemoveSessionAttachments(userDataDir, sessionID string) error {
	if userDataDir == "" || sessionID == "" {
		return nil
	}
	if strings.Contains(sessionID, "..") || strings.ContainsAny(sessionID, `/\`) {
		return fmt.Errorf("invalid session id")
	}
	return os.RemoveAll(sessionAttachmentsDir(userDataDir, sessionID))
}

// DecodeDataURL parses a data:[mime];base64,... URL.
func DecodeDataURL(dataURL string) (mime string, data []byte, err error) {
	dataURL = strings.TrimSpace(dataURL)
	if !strings.HasPrefix(dataURL, "data:") {
		return "", nil, fmt.Errorf("not a data URL")
	}
	rest := strings.TrimPrefix(dataURL, "data:")
	header, payload, ok := strings.Cut(rest, ",")
	if !ok {
		return "", nil, fmt.Errorf("invalid data URL")
	}
	header = strings.ToLower(header)
	if !strings.HasSuffix(header, ";base64") {
		return "", nil, fmt.Errorf("data URL must be base64")
	}
	mime = normalizeMIME(strings.TrimSuffix(header, ";base64"))
	if mime == "" {
		return "", nil, fmt.Errorf("missing MIME in data URL")
	}
	data, err = base64.StdEncoding.DecodeString(payload)
	if err != nil {
		data, err = base64.RawStdEncoding.DecodeString(payload)
		if err != nil {
			return "", nil, fmt.Errorf("invalid base64 data")
		}
	}
	return mime, data, nil
}

// EncodeDataURL builds a data URL from mime and raw bytes.
func EncodeDataURL(mime string, data []byte) string {
	mime = normalizeMIME(mime)
	if mime == "" {
		mime = "application/octet-stream"
	}
	return "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(data)
}

// ChatAttachmentInput is one composer file: Path (picker/drop) or DataURL (paste).
type ChatAttachmentInput struct {
	DataURL  string
	Path     string
	Filename string
	MIME     string
}

// PersistUserAttachments validates inputs and returns parts ready for SQLite.
// Pasted files (DataURL) are copied into the session attachments dir; picker/drop
// files (Path) store a vela-path: ref to the original file only.
func PersistUserAttachments(userDataDir, sessionID, text string, atts []ChatAttachmentInput, newID func() string) ([]*genkitAI.Part, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, fmt.Errorf("message text is required")
	}
	if len(atts) > MaxAttachCount {
		return nil, fmt.Errorf("at most %d attachments allowed", MaxAttachCount)
	}
	parts := []*genkitAI.Part{genkitAI.NewTextPart(text)}
	for i, att := range atts {
		part, err := persistOneAttachment(userDataDir, sessionID, att, i+1, newID)
		if err != nil {
			return nil, err
		}
		parts = append(parts, part)
	}
	return parts, nil
}

func persistOneAttachment(userDataDir, sessionID string, att ChatAttachmentInput, index int, newID func() string) (*genkitAI.Part, error) {
	if p := strings.TrimSpace(att.Path); p != "" {
		abs, mime, filename, err := inspectLocalAttach(p, att.MIME, att.Filename, index)
		if err != nil {
			return nil, fmt.Errorf("attachment %d: %w", index, err)
		}
		return newMediaRefPart(mime, formatPathURI(abs), filename), nil
	}

	mime, data, err := DecodeDataURL(att.DataURL)
	if err != nil {
		return nil, fmt.Errorf("attachment %d: %w", index, err)
	}
	if !allowedAttachMIME(mime) {
		return nil, fmt.Errorf("attachment %d: unsupported type %s", index, mime)
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("attachment %d: empty file", index)
	}
	if len(data) > MaxAttachBytes {
		return nil, fmt.Errorf("attachment %d: exceeds size limit", index)
	}
	filename := sanitizeAttachFilename(att.Filename)
	if filename == "" {
		filename = defaultFilename(mime, index)
	}
	attID := attachmentDiskID(newID(), filename, mime)
	if err := saveAttachment(userDataDir, sessionID, attID, data); err != nil {
		return nil, fmt.Errorf("attachment %d: save: %w", index, err)
	}
	return newMediaRefPart(mime, FormatAttURI(sessionID, attID), filename), nil
}

func inspectLocalAttach(rawPath, mimeHint, filenameHint string, index int) (abs, mime, filename string, err error) {
	abs, err = validateLocalAttachPath(rawPath)
	if err != nil {
		return "", "", "", err
	}
	st, err := os.Stat(abs)
	if err != nil {
		return "", "", "", fmt.Errorf("file not found")
	}
	if !st.Mode().IsRegular() {
		return "", "", "", fmt.Errorf("not a file")
	}
	if st.Size() == 0 {
		return "", "", "", fmt.Errorf("empty file")
	}
	if st.Size() > MaxAttachBytes {
		return "", "", "", fmt.Errorf("exceeds size limit")
	}
	mime = normalizeMIME(mimeHint)
	if mime == "" {
		mime = mimeFromExt(filepath.Ext(abs))
	}
	if !allowedAttachMIME(mime) {
		return "", "", "", fmt.Errorf("unsupported type %s", mime)
	}
	filename = sanitizeAttachFilename(filenameHint)
	if filename == "" {
		filename = sanitizeAttachFilename(filepath.Base(abs))
	}
	if filename == "" {
		filename = defaultFilename(mime, index)
	}
	return abs, mime, filename, nil
}

func validateLocalAttachPath(p string) (string, error) {
	p = strings.TrimSpace(p)
	if p == "" || !filepath.IsAbs(p) {
		return "", fmt.Errorf("invalid path")
	}
	return filepath.Clean(p), nil
}

func formatPathURI(abs string) string {
	return PathURIPrefix + url.QueryEscape(filepath.Clean(abs))
}

func parsePathURI(uri string) (string, bool) {
	if !strings.HasPrefix(uri, PathURIPrefix) {
		return "", false
	}
	raw, err := url.QueryUnescape(strings.TrimPrefix(uri, PathURIPrefix))
	if err != nil || raw == "" {
		return "", false
	}
	abs, err := validateLocalAttachPath(raw)
	if err != nil {
		return "", false
	}
	return abs, true
}

func sanitizeAttachFilename(name string) string {
	name = filepath.Base(strings.TrimSpace(name))
	if name == "." || name == ".." {
		return ""
	}
	return name
}

// attachmentDiskID keeps a file extension so the OS opener can pick an app.
func attachmentDiskID(id, filename, mime string) string {
	ext := filepath.Ext(filename)
	if ext == "" {
		ext = extFromMIME(mime)
	}
	if ext != "" && !strings.EqualFold(filepath.Ext(id), ext) {
		return id + ext
	}
	return id
}

func extFromMIME(mime string) string {
	switch normalizeMIME(mime) {
	case "image/png":
		return ".png"
	case "image/jpeg":
		return ".jpg"
	case "image/webp":
		return ".webp"
	case "image/gif":
		return ".gif"
	case "application/pdf":
		return ".pdf"
	case "text/markdown":
		return ".md"
	case "text/csv":
		return ".csv"
	case "text/plain":
		return ".txt"
	default:
		return ""
	}
}

func mediaFilename(p *genkitAI.Part) string {
	if p.Metadata == nil {
		return ""
	}
	if v, ok := p.Metadata[metaFilename]; ok {
		return sanitizeAttachFilename(fmt.Sprint(v))
	}
	return ""
}

func newMediaRefPart(mime, uri, filename string) *genkitAI.Part {
	p := genkitAI.NewMediaPart(mime, uri)
	if filename != "" {
		p.Metadata = map[string]any{metaFilename: filename}
	}
	return p
}

func formatAttachedText(filename, body string) string {
	return fmt.Sprintf("Attached file %q:\n```\n%s\n```", filename, body)
}

func defaultFilename(mime string, index int) string {
	ext := extFromMIME(mime)
	if ext == "" {
		ext = ".txt"
	}
	prefix := "file"
	if strings.HasPrefix(normalizeMIME(mime), "image/") {
		prefix = "image"
	}
	return fmt.Sprintf("%s-%d%s", prefix, index, ext)
}

func isAttachRef(uri string) bool {
	return strings.HasPrefix(uri, AttURIPrefix) || strings.HasPrefix(uri, PathURIPrefix)
}

func readAttachBytes(userDataDir, uri string) ([]byte, error) {
	if sessionID, attID, ok := ParseAttURI(uri); ok {
		return readAttachment(userDataDir, sessionID, attID)
	}
	if path, ok := parsePathURI(uri); ok {
		st, err := os.Stat(path)
		if err != nil {
			return nil, err
		}
		if st.Size() > MaxAttachBytes {
			return nil, fmt.Errorf("exceeds size limit")
		}
		return os.ReadFile(path)
	}
	return nil, fmt.Errorf("invalid attachment ref")
}

func mimeFromURI(uri, fallback string) string {
	if mime := normalizeMIME(fallback); mime != "" {
		return mime
	}
	if _, id, ok := ParseAttURI(uri); ok {
		return mimeFromExt(filepath.Ext(id))
	}
	if path, ok := parsePathURI(uri); ok {
		return mimeFromExt(filepath.Ext(path))
	}
	return ""
}

// ResolvePartsForModel expands attachment refs for the provider: text MIME → TextPart,
// everything else → MediaPart with a data URL.
func ResolvePartsForModel(userDataDir string, parts []*genkitAI.Part) ([]*genkitAI.Part, error) {
	out := make([]*genkitAI.Part, 0, len(parts))
	for _, p := range parts {
		if p == nil {
			continue
		}
		if !p.IsMedia() || !isAttachRef(p.Text) {
			out = append(out, p)
			continue
		}
		data, err := readAttachBytes(userDataDir, p.Text)
		if err != nil {
			return nil, fmt.Errorf("read attachment: %w", err)
		}
		mime := mimeFromURI(p.Text, p.ContentType)
		if isTextAttachMIME(mime) {
			filename := mediaFilename(p)
			if filename == "" {
				filename = defaultFilename(mime, 1)
			}
			out = append(out, genkitAI.NewTextPart(formatAttachedText(filename, string(data))))
			continue
		}
		resolved := genkitAI.NewMediaPart(mime, EncodeDataURL(mime, data))
		if p.Metadata != nil {
			resolved.Metadata = p.Metadata
		}
		out = append(out, resolved)
	}
	return out, nil
}

// OpenChatAttachment opens a chat attachment with the OS default application.
func OpenChatAttachment(userDataDir, uri, filename string) error {
	if sessionID, attID, ok := ParseAttURI(uri); ok {
		path, err := attachmentPath(userDataDir, sessionID, attID)
		if err != nil {
			return err
		}
		return OpenPath(path)
	}
	if path, ok := parsePathURI(uri); ok {
		return OpenPath(path)
	}
	if strings.HasPrefix(uri, "data:") {
		mime, data, err := DecodeDataURL(uri)
		if err != nil {
			return err
		}
		name := sanitizeAttachFilename(filename)
		if name == "" {
			name = defaultFilename(mime, 1)
		}
		ext := filepath.Ext(name)
		if ext == "" {
			ext = extFromMIME(mime)
		}
		tmp, err := os.CreateTemp("", "vela-att-*"+ext)
		if err != nil {
			return err
		}
		tmpPath := tmp.Name()
		if _, err := tmp.Write(data); err != nil {
			_ = tmp.Close()
			_ = os.Remove(tmpPath)
			return err
		}
		if err := tmp.Close(); err != nil {
			_ = os.Remove(tmpPath)
			return err
		}
		return OpenPath(tmpPath)
	}
	return fmt.Errorf("unsupported attachment uri")
}

// AttachmentDataURL loads a ref as a data URL for in-app image preview.
func AttachmentDataURL(userDataDir, uri, mime string) (string, error) {
	data, err := readAttachBytes(userDataDir, uri)
	if err != nil {
		return "", err
	}
	return EncodeDataURL(mimeFromURI(uri, mime), data), nil
}

// LocalFileDataURL reads a local file for composer preview (not copied into the user data dir).
func LocalFileDataURL(path string) (string, error) {
	abs, mime, _, err := inspectLocalAttach(path, "", "", 1)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return "", err
	}
	return EncodeDataURL(mime, data), nil
}

func mimeFromExt(ext string) string {
	switch strings.ToLower(ext) {
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".webp":
		return "image/webp"
	case ".gif":
		return "image/gif"
	case ".pdf":
		return "application/pdf"
	case ".md", ".markdown":
		return "text/markdown"
	case ".csv":
		return "text/csv"
	case ".txt":
		return "text/plain"
	default:
		return ""
	}
}
