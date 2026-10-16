package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/ilaziness/vela/internal/system"
	"github.com/ilaziness/vela/internal/transfer"
)

const (
	editTempRoot     = "vela-sftp-edit"
	editPollInterval = time.Second
	editMtimeStable  = 800 * time.Millisecond
)

type beginAction int

const (
	beginStarted beginAction = iota
	beginDownloading
	beginReopen
)

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
	done        chan struct{}
	mu          sync.Mutex
}

type editBook struct {
	mu          sync.Mutex
	by          map[string]map[string]*remoteEdit
	prompts     map[string]*sync.Mutex
	closingDone map[string][]chan struct{}
}

func newEditBook() *editBook {
	return &editBook{
		by:          map[string]map[string]*remoteEdit{},
		prompts:     map[string]*sync.Mutex{},
		closingDone: map[string][]chan struct{}{},
	}
}

func (b *editBook) promptLock(sessionID string) *sync.Mutex {
	b.mu.Lock()
	defer b.mu.Unlock()
	m, ok := b.prompts[sessionID]
	if !ok {
		m = &sync.Mutex{}
		b.prompts[sessionID] = m
	}
	return m
}

func (b *editBook) begin(ed *remoteEdit) (beginAction, string) {
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
		opened := cur.opened
		local := cur.local
		cancel := cur.cancel
		cur.mu.Unlock()
		if downloading {
			return beginDownloading, local
		}
		if opened {
			return beginReopen, local
		}
		if cancel != nil {
			cancel()
		}
	}
	files[ed.remote] = ed
	return beginStarted, ed.local
}

// remove drops ed only when it is still the session's current edit for that path.
// A newer EditRemoteFile may already have replaced it.
func (b *editBook) remove(ed *remoteEdit) {
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

func (b *editBook) snapshot(sessionID string) []*remoteEdit {
	b.mu.Lock()
	defer b.mu.Unlock()
	files := b.by[sessionID]
	out := make([]*remoteEdit, 0, len(files))
	for _, ed := range files {
		out = append(out, ed)
	}
	return out
}

func (b *editBook) dropSession(sessionID string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.by, sessionID)
	delete(b.prompts, sessionID)
}

func (b *editBook) setClosingDone(sessionID string, dones []chan struct{}) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(dones) == 0 {
		delete(b.closingDone, sessionID)
		return
	}
	b.closingDone[sessionID] = dones
}

func (b *editBook) takeClosingDone(sessionID string) []chan struct{} {
	b.mu.Lock()
	defer b.mu.Unlock()
	dones := b.closingDone[sessionID]
	delete(b.closingDone, sessionID)
	return dones
}

// EditRemoteFile downloads a remote file into a temp directory and opens it.
// A second call while that download is running does nothing. If the file is
// already open, the existing temp file is opened again.
func (s *SftpService) EditRemoteFile(sessionID, remotePath string) error {
	remotePath = path.Clean(remotePath)
	if sessionID == "" || remotePath == "" || remotePath == "." {
		return fmt.Errorf("无效的远程路径")
	}
	info, err := s.mgr.Stat(sessionID, remotePath)
	if err != nil {
		s.logger.Error("stat remote file for edit failed", zap.String("sessionID", sessionID), zap.String("path", remotePath), zap.Error(err))
		return err
	}
	if info.IsDir {
		return fmt.Errorf("不能编辑目录")
	}
	local, err := editTempFile(sessionID, remotePath)
	if err != nil {
		return err
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
		done:        make(chan struct{}),
	}
	started := false
	defer func() {
		if !started {
			close(ed.done)
		}
	}()
	switch action, existing := s.edits.begin(ed); action {
	case beginDownloading:
		cancel()
		return nil
	case beginReopen:
		cancel()
		if err := system.OpenPath(existing); err != nil {
			s.logger.Error("reopen editor failed", zap.String("path", existing), zap.Error(err))
			return err
		}
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(local), 0o755); err != nil {
		s.logger.Error("create edit temp dir failed", zap.String("path", local), zap.Error(err))
		s.abandon(ed)
		return err
	}
	tracker, err := s.mgr.StartOverwriteDownload(sessionID, local, remotePath, info.Size)
	if err != nil {
		s.logger.Error("start edit download failed", zap.String("path", remotePath), zap.Error(err))
		s.abandon(ed)
		return err
	}
	started = true
	go func() {
		defer close(ed.done)
		s.finishEditDownload(ctx, ed, tracker)
	}()
	return nil
}

// CloseSessionEdits asks about unsaved edits and may upload before the SFTP client closes.
// Temp files are removed by CleanupSessionEditDir after the client closes.
func (s *SftpService) CloseSessionEdits(sessionID string) {
	edits := s.edits.snapshot(sessionID)
	for _, ed := range edits {
		ed.mu.Lock()
		ed.closing = true
		cancel := ed.cancel
		ed.mu.Unlock()
		// Cancel before taking the prompt lock. A watcher blocked in the
		// question dialog must be able to notice tab close without the closer
		// already holding that lock.
		if cancel != nil {
			cancel()
		}
	}
	lock := s.edits.promptLock(sessionID)
	lock.Lock()
	defer lock.Unlock()
	dones := make([]chan struct{}, 0, len(edits))
	for _, ed := range edits {
		if ed.done != nil {
			dones = append(dones, ed.done)
		}
		s.flushEdit(ed)
	}
	s.edits.dropSession(sessionID)
	s.edits.setClosingDone(sessionID, dones)
}

// CleanupSessionEditDir waits until edit goroutines leave, then deletes that
// session's temp directory. Call it after the SFTP client is closed so a
// download is no longer writing the file. Delete failures are logged only.
func (s *SftpService) CleanupSessionEditDir(sessionID string) {
	for _, done := range s.edits.takeClosingDone(sessionID) {
		s.waitEditDone(done)
	}
	s.removeEditSessionDir(sessionID)
}

// CloseAllEdits cancels watchers without a dialog. It does not delete temp
// files and must not show a dialog: app shutdown runs on the UI thread.
func (s *SftpService) CloseAllEdits() {
	s.edits.mu.Lock()
	for _, files := range s.edits.by {
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
	s.edits.by = map[string]map[string]*remoteEdit{}
	s.edits.prompts = map[string]*sync.Mutex{}
	s.edits.mu.Unlock()
}

// CleanupAllEditDirs deletes the edit temp root. Call it after SFTP clients
// are closed. It does not show a dialog and does not wait on a confirm.
func (s *SftpService) CleanupAllEditDirs() {
	root := filepath.Join(os.TempDir(), editTempRoot)
	if err := os.RemoveAll(root); err != nil {
		s.logger.Warn("delete sftp edit temp dir failed", zap.String("path", root), zap.Error(err))
	}
}

const editDoneWait = 5 * time.Second

func (s *SftpService) waitEditDone(done <-chan struct{}) {
	timer := time.NewTimer(editDoneWait)
	defer timer.Stop()
	select {
	case <-done:
	case <-timer.C:
		s.logger.Warn("sftp edit goroutine still running; deleting temp dir anyway")
	}
}

func (s *SftpService) removeEditSessionDir(sessionID string) {
	dir, ok := editSessionDir(sessionID)
	if !ok {
		s.logger.Warn("skip delete of sftp edit temp dir", zap.String("sessionID", sessionID))
		return
	}
	if err := os.RemoveAll(dir); err != nil {
		s.logger.Warn("delete sftp edit temp dir failed", zap.String("path", dir), zap.Error(err))
	}
}

func (s *SftpService) abandon(ed *remoteEdit) {
	if ed.cancel != nil {
		ed.cancel()
	}
	s.edits.remove(ed)
}

func (s *SftpService) editStopped(ed *remoteEdit) bool {
	ed.mu.Lock()
	defer ed.mu.Unlock()
	return ed.closing || ed.discard
}

func (s *SftpService) finishEditDownload(ctx context.Context, ed *remoteEdit, tracker *transfer.Tracker) {
	err := tracker.Wait()
	if ctx.Err() != nil || s.editStopped(ed) {
		return
	}
	if err != nil {
		s.logger.Error("edit download failed", zap.String("sessionID", ed.sessionID), zap.String("path", ed.remote), zap.Error(err))
		s.abandon(ed)
		s.showEditError(fmt.Sprintf("下载 %s 失败：%v", path.Base(ed.remote), err))
		return
	}
	st, err := os.Stat(ed.local)
	if err != nil {
		s.logger.Error("stat edit temp file failed", zap.String("path", ed.local), zap.Error(err))
		s.abandon(ed)
		s.showEditError(fmt.Sprintf("下载 %s 失败：%v", path.Base(ed.remote), err))
		return
	}
	ed.mu.Lock()
	ed.baseline = st.ModTime()
	ed.downloading = false
	ed.opened = true
	ed.mu.Unlock()
	if err := system.OpenPath(ed.local); err != nil {
		s.logger.Error("open editor failed", zap.String("path", ed.local), zap.Error(err))
		ed.mu.Lock()
		ed.opened = false
		ed.mu.Unlock()
		s.abandon(ed)
		s.showEditError(fmt.Sprintf("打开 %s 失败：%v", path.Base(ed.remote), err))
		return
	}
	s.watchEdit(ctx, ed)
}

func (s *SftpService) watchEdit(ctx context.Context, ed *remoteEdit) {
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
				s.logger.Warn("stat edit temp while watching failed", zap.String("path", local), zap.Error(err))
				continue
			}
			if !gate.observe(now, st.ModTime(), baseline, editMtimeStable) {
				continue
			}
			s.promptEditUpload(ctx, ed, st.ModTime())
			gate.reset()
		}
	}
}

func (s *SftpService) promptEditUpload(ctx context.Context, ed *remoteEdit, triggered time.Time) {
	lock := s.edits.promptLock(ed.sessionID)
	lock.Lock()
	defer lock.Unlock()

	ed.mu.Lock()
	if ed.closing || ed.discard || !ed.opened || !triggered.After(ed.baseline) {
		ed.mu.Unlock()
		return
	}
	ed.mu.Unlock()

	changed := s.remoteDiffers(ed)
	switch s.askEditYes(ctx, editConfirmMessage(ed.remote, changed)) {
	case editInterrupted:
		return
	case editNo:
		if !s.discardedOnly(ed) {
			s.setBaseline(ed, currentMtime(ed.local, triggered))
		}
		return
	}
	if s.discardedOnly(ed) {
		return
	}

	started, err := s.uploadEdit(ed)
	s.setBaseline(ed, nextEditBaseline(triggered, started, err == nil))
	if err != nil {
		s.logger.Error("edit upload failed", zap.String("sessionID", ed.sessionID), zap.String("path", ed.remote), zap.Error(err))
		if !s.editStopped(ed) {
			s.showEditError(fmt.Sprintf("上传 %s 失败：%v", path.Base(ed.remote), err))
		}
	}
}

func (s *SftpService) flushEdit(ed *remoteEdit) {
	ed.mu.Lock()
	opened := ed.opened
	downloading := ed.downloading
	baseline := ed.baseline
	local := ed.local
	ed.mu.Unlock()
	if !opened || downloading {
		return
	}
	st, err := os.Stat(local)
	if err != nil {
		s.logger.Warn("stat edit temp on close failed", zap.String("path", local), zap.Error(err))
		return
	}
	if !st.ModTime().After(baseline) {
		return
	}
	if s.askEditYes(context.Background(), editConfirmMessage(ed.remote, s.remoteDiffers(ed))) != editYes {
		return
	}
	if s.discardedOnly(ed) {
		return
	}
	if _, err := s.uploadEdit(ed); err != nil {
		s.logger.Error("edit upload on close failed", zap.String("sessionID", ed.sessionID), zap.String("path", ed.remote), zap.Error(err))
		s.showEditError(fmt.Sprintf("上传 %s 失败：%v", path.Base(ed.remote), err))
	}
}

// uploadEdit returns the local mtime observed before the upload reads the file.
func (s *SftpService) uploadEdit(ed *remoteEdit) (time.Time, error) {
	st, err := os.Stat(ed.local)
	if err != nil {
		return time.Time{}, err
	}
	started := st.ModTime()
	tracker, err := s.mgr.StartOverwriteUpload(ed.sessionID, ed.local, ed.remote, st.Size())
	if err != nil {
		return time.Time{}, err
	}
	if err := tracker.Wait(); err != nil {
		return time.Time{}, err
	}
	info, statErr := s.mgr.Stat(ed.sessionID, ed.remote)
	if statErr != nil {
		s.logger.Warn("stat remote after edit upload failed", zap.String("path", ed.remote), zap.Error(statErr))
		return started, nil
	}
	ed.mu.Lock()
	ed.snapSize = info.Size
	ed.snapMod = info.ModTime
	ed.mu.Unlock()
	return started, nil
}

// nextEditBaseline is the mtime to treat as already uploaded.
// On success that is the mtime from the start of the upload: a save while the
// upload was reading the file must stay newer than the baseline.
// On failure it stays at the mtime that opened the prompt, so the dialog does
// not repeat until the file changes again.
func nextEditBaseline(triggered, started time.Time, uploadOK bool) time.Time {
	if !uploadOK {
		return triggered
	}
	return started
}

func (s *SftpService) remoteDiffers(ed *remoteEdit) bool {
	info, err := s.mgr.Stat(ed.sessionID, ed.remote)
	if err != nil {
		s.logger.Warn("stat remote before edit confirm failed", zap.String("path", ed.remote), zap.Error(err))
		return false
	}
	ed.mu.Lock()
	size, mod := ed.snapSize, ed.snapMod
	ed.mu.Unlock()
	return info.Size != size || info.ModTime != mod
}

func (s *SftpService) setBaseline(ed *remoteEdit, mt time.Time) {
	ed.mu.Lock()
	ed.baseline = mt
	ed.mu.Unlock()
}

func (s *SftpService) discardedOnly(ed *remoteEdit) bool {
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

func (s *SftpService) askEditYes(ctx context.Context, message string) editChoice {
	if s.app == nil {
		s.logger.Error("edit confirm unavailable", zap.String("message", message))
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
	dlg := s.app.Dialog.Question().SetTitle("Vela - 编辑").SetMessage(message)
	yes := dlg.AddButton("Yes")
	no := dlg.AddButton("No")
	yes.OnClick(func() { send(true) })
	no.OnClick(func() { send(false) })
	// Windows MessageBox is MB_YESNO. Title-bar close reports "Cancel", which is
	// not Yes or No, so a Windows-only button with that label receives it.
	// Linux and macOS render every added button, and their Show returns before
	// the click. GTK title-bar close does not run a button callback.
	if runtime.GOOS == "windows" {
		cancel := dlg.AddButton("Cancel")
		cancel.OnClick(func() { send(false) })
	}
	dlg.SetDefaultButton(no)
	dlg.SetCancelButton(no)
	dlg.Show()
	if runtime.GOOS == "windows" {
		select {
		case yes := <-result:
			return choiceFromButton(yes)
		default:
			// Title-bar close is MessageBox Cancel and matches neither Yes nor No
			// unless the extra Cancel button above ran. Treat that as No.
			return editNo
		}
	}
	// Linux and macOS return from Show before the click. Closing the GTK dialog
	// from the title bar does not run a button callback, so also stop waiting
	// when the edit context is cancelled (tab close) instead of holding the
	// prompt lock until the process exits.
	select {
	case yes := <-result:
		return choiceFromButton(yes)
	case <-ctx.Done():
		return editInterrupted
	}
}

func choiceFromButton(yes bool) editChoice {
	if yes {
		return editYes
	}
	return editNo
}

func (s *SftpService) showEditError(message string) {
	if s.app == nil {
		return
	}
	s.app.Dialog.Error().SetTitle("Vela - 编辑").SetMessage(message).Show()
}

func editConfirmMessage(remotePath string, remoteChanged bool) string {
	msg := fmt.Sprintf("文件 %s 已在本地修改，是否上传并覆盖远程文件？", path.Base(remotePath))
	if remoteChanged {
		msg += "\n远程文件的大小或修改时间与下载时不一致。"
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

type mtimeGate struct {
	pending time.Time
	since   time.Time
}

func (g *mtimeGate) reset() {
	g.pending = time.Time{}
	g.since = time.Time{}
}

// observe reports whether mod is newer than baseline and has stayed unchanged for stable.
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

func editTempFile(sessionID, remotePath string) (string, error) {
	dir, ok := editSessionDir(sessionID)
	if !ok {
		return "", fmt.Errorf("无效的会话")
	}
	sum := sha256.Sum256([]byte(remotePath))
	digest := hex.EncodeToString(sum[:])
	return filepath.Join(dir, digest, sanitizeEditBase(remotePath)), nil
}

// editSessionDir is the temp directory for one session, under the edit root.
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
