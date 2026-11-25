package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/go-gui-org/go-gui/gui"
	"go.uber.org/zap"

	"github.com/ilaziness/vela/internal/i18n"
	"github.com/ilaziness/vela/internal/system"
	"github.com/ilaziness/vela/internal/transfer"
	"github.com/ilaziness/vela/internal/usererr"
	"github.com/ilaziness/vela/ui/errtext"
)

const (
	editTempRoot     = "vela-sftp-edit"
	editPollInterval = time.Second
	editMtimeStable  = 800 * time.Millisecond
)

// remoteEdit tracks one remote file opened in an external editor.
type remoteEdit struct {
	sessionID   string
	remote      string
	local       string
	baseline    time.Time
	snapSize    int64
	snapMod     string
	downloading bool
	opened      bool
	closing     bool
	discard     bool
	cancel      context.CancelFunc
	mu          sync.Mutex
}

// sftpEditBook owns the per-session remote-edit watchers. It is safe
// for concurrent use; all window/dialog interaction goes through the
// Controller.
type sftpEditBook struct {
	mu      sync.Mutex
	by      map[string]map[string]*remoteEdit
	prompts map[string]*sync.Mutex
	// dlgMu serializes edit-confirm dialogs across sessions: go-gui
	// has a single dialog slot, so two prompts shown together would
	// replace each other and strand the loser's watcher goroutine.
	dlgMu sync.Mutex
}

func newEditBook() *sftpEditBook {
	return &sftpEditBook{
		by:      map[string]map[string]*remoteEdit{},
		prompts: map[string]*sync.Mutex{},
	}
}

// The book methods tolerate a nil receiver: tests build a bare
// &Controller{} without NewController's edits.

func (b *sftpEditBook) promptLock(sessionID string) *sync.Mutex {
	if b == nil {
		return &sync.Mutex{}
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	m, ok := b.prompts[sessionID]
	if !ok {
		m = &sync.Mutex{}
		b.prompts[sessionID] = m
	}
	return m
}

// begin registers ed as the session's current edit for its path and
// reports whether the caller should download. A download already in
// flight wins (false: the caller does nothing); an already-open edit
// is replaced — its watcher is cancelled so the new download
// overwrites the temp copy and reopens it.
func (b *sftpEditBook) begin(ed *remoteEdit) bool {
	if b == nil {
		return true
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	files := b.by[ed.sessionID]
	if files == nil {
		files = map[string]*remoteEdit{}
		b.by[ed.sessionID] = files
	}
	if cur := files[ed.remote]; cur != nil {
		cur.mu.Lock()
		downloading := cur.downloading
		if !downloading {
			cur.closing = true
		}
		cancel := cur.cancel
		cur.mu.Unlock()
		if downloading {
			return false
		}
		if cancel != nil {
			cancel()
		}
	}
	files[ed.remote] = ed
	return true
}

// remoteDirty reports whether an already-open edit for remote has
// local changes that were not uploaded yet. A missing temp file counts
// as clean: the re-download recreates it.
func (b *sftpEditBook) remoteDirty(sessionID, remote string) bool {
	if b == nil {
		return false
	}
	b.mu.Lock()
	ed := b.by[sessionID][remote]
	b.mu.Unlock()
	return ed.localDirty()
}

// localDirty reports unsaved edits in the temp file. Downloads leave
// opened false until they land, so an in-flight download is clean.
func (ed *remoteEdit) localDirty() bool {
	if ed == nil {
		return false
	}
	ed.mu.Lock()
	opened := ed.opened
	local := ed.local
	baseline := ed.baseline
	ed.mu.Unlock()
	if !opened {
		return false
	}
	st, err := os.Stat(local)
	if err != nil {
		return false
	}
	return st.ModTime().After(baseline)
}

// remove drops ed only when it is still the session's current edit for
// that path; a newer EditRemote may already have replaced it.
func (b *sftpEditBook) remove(ed *remoteEdit) {
	if b == nil {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	files := b.by[ed.sessionID]
	if files == nil || files[ed.remote] != ed {
		return
	}
	delete(files, ed.remote)
	if len(files) == 0 {
		delete(b.by, ed.sessionID)
	}
}

func (b *sftpEditBook) snapshot(sessionID string) []*remoteEdit {
	if b == nil {
		return nil
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	files := b.by[sessionID]
	out := make([]*remoteEdit, 0, len(files))
	for _, ed := range files {
		out = append(out, ed)
	}
	return out
}

// dialogLock/dialogUnlock serialize edit-confirm dialogs across
// sessions (nil-tolerant like the other book methods).
func (b *sftpEditBook) dialogLock() {
	if b != nil {
		b.dlgMu.Lock()
	}
}

func (b *sftpEditBook) dialogUnlock() {
	if b != nil {
		b.dlgMu.Unlock()
	}
}

func (b *sftpEditBook) dropSession(sessionID string) {
	if b == nil {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.by, sessionID)
	delete(b.prompts, sessionID)
}

func (b *sftpEditBook) unsaved(sessionID string) []*remoteEdit {
	var out []*remoteEdit
	for _, ed := range b.snapshot(sessionID) {
		if ed.localDirty() {
			out = append(out, ed)
		}
	}
	return out
}

// EditRemoteFile downloads a remote file into a temp directory and
// opens it with the OS default application. Every call re-downloads,
// so a stale temp copy can never be edited by mistake; when the temp
// copy has unsaved local changes the user confirms the overwrite
// first. A call while the same download runs does nothing. Ported from
// v1 services/sftp_edit.go.
func (c *Controller) EditRemoteFile(sessionID, remotePath string) error {
	remotePath = path.Clean(remotePath)
	if sessionID == "" || remotePath == "" || remotePath == "." {
		return usererr.Coded("sftp.invalid_remote_path", "Invalid remote path")
	}
	if c.SFTPManager() == nil {
		return usererr.Coded("sftp.unsupported", "SFTP is not available")
	}
	info, err := c.SFTPManager().Stat(sessionID, remotePath)
	if err != nil {
		c.logger.Error("stat remote file for edit failed",
			zap.String("sessionID", sessionID), zap.String("path", remotePath), zap.Error(err))
		return err
	}
	if info.IsDir {
		return usererr.Coded("sftp.cannot_edit_dir", "Cannot edit a directory")
	}
	local, err := editTempFile(sessionID, remotePath)
	if err != nil {
		return err
	}
	if c.edits.remoteDirty(sessionID, remotePath) {
		c.edits.dialogLock()
		choice := c.askEditYes(context.Background(),
			i18n.Tf("sftp.confirm_redownload", path.Base(remotePath)))
		c.edits.dialogUnlock()
		if choice != editYes {
			return nil
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	ed := &remoteEdit{
		sessionID:   sessionID,
		remote:      remotePath,
		local:       local,
		snapSize:    info.Size,
		snapMod:     info.ModTime,
		downloading: true,
		cancel:      cancel,
	}
	if !c.edits.begin(ed) {
		cancel()
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(local), 0o755); err != nil {
		c.logger.Error("create edit temp dir failed", zap.String("path", local), zap.Error(err))
		c.abandonEdit(ed)
		return err
	}
	tracker, err := c.SFTPManager().StartOverwriteDownload(sessionID, local, remotePath, info.Size)
	if err != nil {
		c.logger.Error("start edit download failed", zap.String("path", remotePath), zap.Error(err))
		c.abandonEdit(ed)
		return err
	}
	go c.finishEditDownload(ctx, ed, tracker)
	return nil
}

// UploadSessionEdits uploads every unsaved edit without asking again;
// the caller already confirmed. Runs on a goroutine.
func (c *Controller) UploadSessionEdits(sessionID string) {
	lock := c.edits.promptLock(sessionID)
	lock.Lock()
	defer lock.Unlock()
	for _, ed := range c.edits.unsaved(sessionID) {
		ed.mu.Lock()
		ed.closing = true
		cancel := ed.cancel
		ed.mu.Unlock()
		if cancel != nil {
			cancel()
		}
		if _, err := c.uploadEdit(ed); err != nil {
			c.logger.Error("edit upload on session close failed",
				zap.String("sessionID", sessionID), zap.String("path", ed.remote), zap.Error(err))
		}
	}
	c.edits.dropSession(sessionID)
}

// DiscardSessionEdits cancels watchers without prompting or uploading.
func (c *Controller) DiscardSessionEdits(sessionID string) {
	for _, ed := range c.edits.snapshot(sessionID) {
		ed.mu.Lock()
		ed.closing = true
		ed.discard = true
		cancel := ed.cancel
		ed.mu.Unlock()
		if cancel != nil {
			cancel()
		}
	}
	c.edits.dropSession(sessionID)
}

// UnsavedEditCount reports opened edits with local changes.
func (c *Controller) UnsavedEditCount(sessionID string) int {
	return len(c.edits.unsaved(sessionID))
}

// CleanupSessionEditDir deletes that session's temp directory. Call
// after watchers are cancelled and the SFTP client is closed; watchers
// hold no file handles between polls, so removal cannot race a write.
func (c *Controller) CleanupSessionEditDir(sessionID string) {
	c.removeEditSessionDir(sessionID)
}

// CloseAllEdits cancels watchers without a dialog (app shutdown).
func (c *Controller) CloseAllEdits() {
	if c.edits == nil {
		return
	}
	c.edits.mu.Lock()
	for _, files := range c.edits.by {
		for _, ed := range files {
			ed.mu.Lock()
			ed.closing = true
			ed.discard = true
			cancel := ed.cancel
			ed.mu.Unlock()
			if cancel != nil {
				cancel()
			}
		}
	}
	c.edits.by = map[string]map[string]*remoteEdit{}
	c.edits.prompts = map[string]*sync.Mutex{}
	c.edits.mu.Unlock()
}

// CleanupAllEditDirs deletes the whole edit temp root.
func (c *Controller) CleanupAllEditDirs() {
	root := filepath.Join(os.TempDir(), editTempRoot)
	if err := os.RemoveAll(root); err != nil {
		c.logger.Warn("delete sftp edit temp dir failed", zap.String("path", root), zap.Error(err))
	}
}

func (c *Controller) removeEditSessionDir(sessionID string) {
	dir, ok := editSessionDir(sessionID)
	if !ok {
		c.logger.Warn("skip delete of sftp edit temp dir", zap.String("sessionID", sessionID))
		return
	}
	if err := os.RemoveAll(dir); err != nil {
		c.logger.Warn("delete sftp edit temp dir failed", zap.String("path", dir), zap.Error(err))
	}
}

func (c *Controller) abandonEdit(ed *remoteEdit) {
	if ed.cancel != nil {
		ed.cancel()
	}
	c.edits.remove(ed)
}

func (c *Controller) editStopped(ed *remoteEdit) bool {
	ed.mu.Lock()
	defer ed.mu.Unlock()
	return ed.closing || ed.discard
}

func (c *Controller) finishEditDownload(ctx context.Context, ed *remoteEdit, tracker *transfer.Tracker) {
	err := tracker.Wait()
	if ctx.Err() != nil || c.editStopped(ed) {
		return
	}
	if err != nil {
		c.logger.Error("edit download failed",
			zap.String("sessionID", ed.sessionID), zap.String("path", ed.remote), zap.Error(err))
		c.abandonEdit(ed)
		c.showEditError(i18n.Tf("sftp.download_failed", path.Base(ed.remote), errtext.Text(err)))
		return
	}
	st, err := os.Stat(ed.local)
	if err != nil {
		c.logger.Error("stat edit temp file failed", zap.String("path", ed.local), zap.Error(err))
		c.abandonEdit(ed)
		c.showEditError(i18n.Tf("sftp.download_failed", path.Base(ed.remote), errtext.Text(err)))
		return
	}
	ed.mu.Lock()
	ed.baseline = st.ModTime()
	ed.downloading = false
	ed.opened = true
	ed.mu.Unlock()
	if err := system.OpenPath(ed.local); err != nil {
		c.logger.Error("open editor failed", zap.String("path", ed.local), zap.Error(err))
		ed.mu.Lock()
		ed.opened = false
		ed.mu.Unlock()
		c.abandonEdit(ed)
		c.showEditError(i18n.Tf("sftp.open_failed", path.Base(ed.remote), errtext.Text(err)))
		return
	}
	c.watchEdit(ctx, ed)
}

func (c *Controller) watchEdit(ctx context.Context, ed *remoteEdit) {
	ticker := time.NewTicker(editPollInterval)
	defer ticker.Stop()
	var gate mtimeGate
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			ed.mu.Lock()
			if ed.closing || ed.discard || !ed.opened {
				ed.mu.Unlock()
				gate.reset()
				continue
			}
			baseline := ed.baseline
			local := ed.local
			ed.mu.Unlock()
			st, err := os.Stat(local)
			if err != nil {
				c.logger.Warn("stat edit temp while watching failed", zap.String("path", local), zap.Error(err))
				continue
			}
			if !gate.observe(now, st.ModTime(), baseline, editMtimeStable) {
				continue
			}
			c.promptEditUpload(ctx, ed, st.ModTime())
			gate.reset()
		}
	}
}

func (c *Controller) promptEditUpload(ctx context.Context, ed *remoteEdit, triggered time.Time) {
	lock := c.edits.promptLock(ed.sessionID)
	lock.Lock()
	defer lock.Unlock()

	ed.mu.Lock()
	if ed.closing || ed.discard || !ed.opened || !triggered.After(ed.baseline) {
		ed.mu.Unlock()
		return
	}
	ed.mu.Unlock()

	changed := c.remoteDiffers(ed)
	c.edits.dialogLock()
	if c.editStopped(ed) {
		c.edits.dialogUnlock()
		return
	}
	choice := c.askEditYes(ctx, editConfirmMessage(ed.remote, changed))
	c.edits.dialogUnlock()
	switch choice {
	case editInterrupted:
		return
	case editNo:
		if !c.discardedOnly(ed) {
			c.setEditBaseline(ed, currentMtime(ed.local, triggered))
		}
		return
	}
	if c.discardedOnly(ed) {
		return
	}

	started, err := c.uploadEdit(ed)
	c.setEditBaseline(ed, nextEditBaseline(triggered, started, err == nil))
	if err != nil {
		c.logger.Error("edit upload failed",
			zap.String("sessionID", ed.sessionID), zap.String("path", ed.remote), zap.Error(err))
		if !c.editStopped(ed) {
			c.showEditError(i18n.Tf("sftp.upload_failed", path.Base(ed.remote), errtext.Text(err)))
		}
	}
	c.refreshRemoteAfterEdit(ed.sessionID)
}

// uploadEdit returns the local mtime observed before the upload reads
// the file.
func (c *Controller) uploadEdit(ed *remoteEdit) (time.Time, error) {
	st, err := os.Stat(ed.local)
	if err != nil {
		return time.Time{}, err
	}
	started := st.ModTime()
	tracker, err := c.SFTPManager().StartOverwriteUpload(ed.sessionID, ed.local, ed.remote, st.Size())
	if err != nil {
		return time.Time{}, err
	}
	if err := tracker.Wait(); err != nil {
		return time.Time{}, err
	}
	info, statErr := c.SFTPManager().Stat(ed.sessionID, ed.remote)
	if statErr != nil {
		c.logger.Warn("stat remote after edit upload failed", zap.String("path", ed.remote), zap.Error(statErr))
		return started, nil
	}
	ed.mu.Lock()
	ed.snapSize = info.Size
	ed.snapMod = info.ModTime
	ed.mu.Unlock()
	return started, nil
}

// nextEditBaseline is the mtime to treat as already uploaded: on
// success the mtime at upload start (a save mid-upload stays newer);
// on failure the mtime that opened the prompt, so the dialog does not
// repeat until the file changes again.
func nextEditBaseline(triggered, started time.Time, uploadOK bool) time.Time {
	if !uploadOK {
		return triggered
	}
	return started
}

func (c *Controller) remoteDiffers(ed *remoteEdit) bool {
	info, err := c.SFTPManager().Stat(ed.sessionID, ed.remote)
	if err != nil {
		c.logger.Warn("stat remote before edit confirm failed", zap.String("path", ed.remote), zap.Error(err))
		return false
	}
	ed.mu.Lock()
	size, mod := ed.snapSize, ed.snapMod
	ed.mu.Unlock()
	return info.Size != size || info.ModTime != mod
}

func (c *Controller) setEditBaseline(ed *remoteEdit, mt time.Time) {
	ed.mu.Lock()
	ed.baseline = mt
	ed.mu.Unlock()
}

func (c *Controller) discardedOnly(ed *remoteEdit) bool {
	ed.mu.Lock()
	defer ed.mu.Unlock()
	return ed.discard
}

type editChoice int

const (
	editNo editChoice = iota
	editYes
	editInterrupted
)

// askEditYes shows a Yes/No dialog and waits for the answer on the
// calling (watcher) goroutine.
func (c *Controller) askEditYes(ctx context.Context, message string) editChoice {
	w := c.Window()
	if w == nil {
		c.logger.Error("edit confirm unavailable", zap.String("message", message))
		return editNo
	}
	if ctx == nil {
		ctx = context.Background()
	}
	result := make(chan bool, 1)
	send := func(yes bool) {
		select {
		case result <- yes:
		default:
		}
	}
	w.QueueCommand(func(w *gui.Window) {
		w.Dialog(gui.DialogCfg{
			DialogType:    gui.DialogConfirm,
			Title:         i18n.T("dialog.edit_title"),
			Body:          message,
			DefaultButton: gui.DialogButtonNo,
			OnOkYes:       func(w *gui.Window) { send(true) },
			OnCancelNo:    func(w *gui.Window) { send(false) },
		})
	})
	select {
	case yes := <-result:
		if yes {
			return editYes
		}
		return editNo
	case <-ctx.Done():
		return editInterrupted
	case <-c.shutdownDone():
		return editInterrupted
	}
}

// showEditError reports watcher failures. It uses a toast rather than
// a dialog on purpose: a Dialog() call would evict a pending
// edit-confirm dialog (single slot) and strand its watcher.
func (c *Controller) showEditError(message string) {
	c.toast(gui.ToastError, "", message)
}

func editConfirmMessage(remotePath string, remoteChanged bool) string {
	msg := i18n.Tf("sftp.confirm_upload", path.Base(remotePath))
	if remoteChanged {
		msg += "\n" + i18n.T("sftp.confirm_remote_changed")
	}
	return msg
}

func currentMtime(local string, fallback time.Time) time.Time {
	st, err := os.Stat(local)
	if err != nil {
		return fallback
	}
	return st.ModTime()
}

// mtimeGate debounces a changed mtime until it stays stable.
type mtimeGate struct {
	pending time.Time
	since   time.Time
}

func (g *mtimeGate) reset() {
	g.pending = time.Time{}
	g.since = time.Time{}
}

// observe reports whether mod is newer than baseline and has stayed
// unchanged for stable.
func (g *mtimeGate) observe(now, mod, baseline time.Time, stable time.Duration) bool {
	if !mod.After(baseline) {
		g.reset()
		return false
	}
	if g.pending.IsZero() || !mod.Equal(g.pending) {
		g.pending = mod
		g.since = now
		return false
	}
	return !now.Before(g.since.Add(stable))
}

// editTempFile maps a remote path to a unique local temp file inside
// the session's edit dir.
func editTempFile(sessionID, remotePath string) (string, error) {
	dir, ok := editSessionDir(sessionID)
	if !ok {
		return "", usererr.Coded("sftp.invalid_session", "Invalid session")
	}
	sum := sha256.Sum256([]byte(remotePath))
	digest := hex.EncodeToString(sum[:])
	return filepath.Join(dir, digest, sanitizeEditBase(remotePath)), nil
}

// editSessionDir validates and returns the temp dir for one session.
func editSessionDir(sessionID string) (string, bool) {
	if sessionID == "" || strings.Contains(sessionID, "..") || strings.ContainsAny(sessionID, `/\`) {
		return "", false
	}
	root := filepath.Join(os.TempDir(), editTempRoot)
	dir := filepath.Join(root, sessionID)
	rel, err := filepath.Rel(root, dir)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false
	}
	return dir, true
}

func sanitizeEditBase(name string) string {
	base := path.Base(strings.ReplaceAll(name, "\\", "/"))
	if base == "." || base == ".." || base == "" {
		return "file"
	}
	var b strings.Builder
	for _, r := range base {
		if illegalEditRune(r) {
			b.WriteByte('_')
			continue
		}
		b.WriteRune(r)
	}
	cleaned := strings.TrimRight(b.String(), " .")
	if cleaned == "" || cleaned == "." || cleaned == ".." {
		cleaned = "file"
	}
	if runtime.GOOS == "windows" && isWindowsReserved(cleaned) {
		cleaned = "_" + cleaned
	}
	return cleaned
}

func illegalEditRune(r rune) bool {
	if r == 0 || r < 32 || r == 127 {
		return true
	}
	if runtime.GOOS == "windows" && strings.ContainsRune(`<>:"/\|?*`, r) {
		return true
	}
	return false
}

func isWindowsReserved(name string) bool {
	stem := name
	if i := strings.IndexByte(name, '.'); i >= 0 {
		stem = name[:i]
	}
	switch strings.ToUpper(stem) {
	case "CON", "PRN", "AUX", "NUL":
		return true
	}
	if len(stem) == 4 && (strings.HasPrefix(strings.ToUpper(stem), "COM") || strings.HasPrefix(strings.ToUpper(stem), "LPT")) {
		d := stem[3]
		return d >= '1' && d <= '9'
	}
	return false
}
