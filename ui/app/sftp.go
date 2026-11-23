package app

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/go-gui-org/go-gui/gui"
	"go.uber.org/zap"

	"github.com/ilaziness/vela/internal/i18n"
	"github.com/ilaziness/vela/internal/localfs"
	"github.com/ilaziness/vela/internal/sftp"
	"github.com/ilaziness/vela/internal/system"
	"github.com/ilaziness/vela/internal/termio"
	"github.com/ilaziness/vela/internal/transfer"
	"github.com/ilaziness/vela/ui/errtext"
)

// SFTPPanelID is the go-gui focus/effective ID of the SFTP panel
// root. OpenSFTP moves keyboard focus here: without it go-gui's
// focus fixup would park focus on the first tab stop (the quick
// connect refresh button) once the terminal releases focus.
const SFTPPanelID = "sftp-panel"

// SetSFTP binds the SFTP manager. edits is created by NewController.
func (c *Controller) SetSFTP(mgr *sftp.Manager) {
	c.mu.Lock()
	c.sftpMgr = mgr
	c.mu.Unlock()
}

// SFTPManager exposes the manager for tests and wiring.
func (c *Controller) SFTPManager() *sftp.Manager {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.sftpMgr
}

// OnTerminalCwd follows the interactive shell's cwd for the remote
// pane. Called from the SSH layer; hops to the UI thread.
func (c *Controller) OnTerminalCwd(sessionID, dir string) {
	w := c.Window()
	if w == nil {
		return
	}
	w.QueueCommand(func(w *gui.Window) {
		st := gui.State[State](w)
		s := st.SFTP[sessionID]
		if s == nil || !s.RemoteReady || s.Status != SFTPReady {
			return
		}
		tab := st.TabBySessID(sessionID)
		if tab == nil || !tab.SFTPOpen {
			return
		}
		if dir == "" || dir == s.RemoteDir || s.RemoteNav != "" {
			return
		}
		if !strings.HasPrefix(dir, "/") {
			return
		}
		c.refreshRemote(w, sessionID, dir)
	})
}

// OnSFTPQueued toasts after transfers were enqueued.
func (c *Controller) OnSFTPQueued(sessionID string, count int) {
	c.toast(gui.ToastInfo, "", i18n.Tf("transfer.added_n", count))
}

// OnTransferProgress merges live transfer progress into the panel
// state and refreshes panes when a transfer finishes.
func (c *Controller) OnTransferProgress(p transfer.ProgressData) {
	w := c.Window()
	if w == nil {
		return
	}
	w.QueueCommand(func(w *gui.Window) {
		st := gui.State[State](w)
		if st.TabBySessID(p.SessionID) == nil {
			return
		}
		c.mu.Lock()
		c.transfers[p.SessionID] = transferUpsert(c.transfers[p.SessionID], p)
		c.mu.Unlock()
		if p.Done {
			c.refreshAfterTransfer(w, p.SessionID)
		}
	})
}

// ToggleSFTP opens or closes the panel for a tab (tool rail entry).
func (c *Controller) ToggleSFTP(w *gui.Window, tabID string) {
	st := gui.State[State](w)
	tab := st.TabByID(tabID)
	if tab == nil || tab.Kind != TabSession || tab.SessID == "" {
		return
	}
	if tab.Protocol != termio.ProtocolSSH {
		c.SetStatus(i18n.T("sftp.unsupported"))
		return
	}
	if tab.SFTPOpen {
		c.CloseSFTP(w, tabID)
		return
	}
	c.OpenSFTP(w, tabID)
}

// SFTPControls reports whether the tab may show the SFTP panel.
func SFTPControls(tab *Tab) bool {
	return tab != nil && tab.Kind == TabSession && tab.SessID != "" && tab.Protocol == termio.ProtocolSSH
}

// OpenSFTP shows the panel for the tab's session. The terminal keeps
// running underneath; only its focus is released.
func (c *Controller) OpenSFTP(w *gui.Window, tabID string) {
	st := gui.State[State](w)
	tab := st.TabByID(tabID)
	if tab == nil || !SFTPControls(tab) {
		return
	}
	sessID := tab.SessID
	s := st.SFTP[sessID]
	if s == nil {
		s = newSFTPState(tab.OwnerKey, c.initialLocalDir())
		st.SFTP[sessID] = s
	}
	tab.SFTPOpen = true
	if tm := c.TermFor(sessID); tm != nil {
		tm.SetFocused(false)
	}
	// The unfocused terminal drops its focusable ID, so go-gui's
	// per-frame fixup would move focus to the first tab stop in the
	// window (the quick-connect refresh button). Claim the panel
	// instead; it is Focusable and keeps Escape working.
	w.SetFocus(SFTPPanelID)
	ownerKey := s.OwnerKey
	go c.initSFTP(w, sessID, ownerKey)
}

// CloseSFTP hides the panel and returns focus to the terminal.
func (c *Controller) CloseSFTP(w *gui.Window, tabID string) {
	st := gui.State[State](w)
	tab := st.TabByID(tabID)
	if tab == nil {
		return
	}
	tab.SFTPOpen = false
	if tab.SessID != "" {
		c.focusActiveTerm(st)
	}
}

// initSFTP connects the SFTP client and loads both panes. Runs on a
// goroutine; results land via QueueCommand.
func (c *Controller) initSFTP(w *gui.Window, sessID, ownerKey string) {
	mgr := c.SFTPManager()
	if mgr == nil {
		return
	}
	if err := mgr.Connect(sessID, ownerKey); err != nil {
		w.QueueCommand(func(w *gui.Window) {
			st := gui.State[State](w)
			s := st.SFTP[sessID]
			if s == nil {
				return
			}
			s.Status = SFTPError
			s.Err = errtext.Text(err)
		})
		return
	}
	remoteDir := ""
	if c.sshMgr != nil {
		remoteDir = c.sshMgr.Cwd(sessID)
	}
	if remoteDir == "" {
		if wd, err := mgr.GetWd(sessID); err == nil {
			remoteDir = wd
		}
	}
	if remoteDir == "" {
		remoteDir = "/"
	}
	// State reads/writes must stay on the UI thread. On reopen the
	// pane keeps the directory the user last browsed; only a fresh
	// panel (RemoteDir empty) seeds from the session cwd.
	w.QueueCommand(func(w *gui.Window) {
		st := gui.State[State](w)
		if s := st.SFTP[sessID]; s != nil && s.RemoteDir != "" {
			remoteDir = s.RemoteDir
		}
		c.refreshRemote(w, sessID, remoteDir)
		c.refreshLocal(w, sessID, "")
		c.refreshTransfers(w, sessID)
	})
}

// initialLocalDir seeds the local pane from persisted state or home.
func (c *Controller) initialLocalDir() string {
	c.mu.Lock()
	store := c.store
	c.mu.Unlock()
	if store != nil {
		if d := store.LastSftpLocalDir(); d != "" {
			return d
		}
	}
	return ""
}

// lastDialogDir returns the remembered native-dialog directory, if any.
func (c *Controller) lastDialogDir() string {
	c.mu.Lock()
	store := c.store
	c.mu.Unlock()
	if store == nil {
		return ""
	}
	return store.LastDialogDir()
}

// rememberDialogPath persists the directory of a picked dialog path.
func (c *Controller) rememberDialogPath(selected string) {
	c.mu.Lock()
	store := c.store
	c.mu.Unlock()
	if store != nil {
		store.RememberDialogPath(selected)
	}
}

// --- pane refresh ---

// refreshRemote lists a remote dir on a goroutine and merges the
// result into panel state on the UI thread.
func (c *Controller) refreshRemote(w *gui.Window, sessID, dir string) {
	mgr := c.SFTPManager()
	if mgr == nil {
		return
	}
	st := gui.State[State](w)
	s := st.SFTP[sessID]
	if s == nil {
		return
	}
	s.RemoteSeq++
	seq := s.RemoteSeq
	s.RemoteNav = dir
	showHidden := s.RemoteShowHidden
	go func() {
		files, err := mgr.ListFiles(sessID, dir, showHidden)
		w.QueueCommand(func(w *gui.Window) {
			st := gui.State[State](w)
			s := st.SFTP[sessID]
			if s == nil || s.RemoteSeq != seq {
				return
			}
			s.RemoteNav = ""
			if err != nil {
				if s.Status == SFTPLoading {
					s.Status = SFTPError
				}
				c.toast(gui.ToastError, "", errtext.Text(err))
				return
			}
			sortRemoteEntries(files)
			s.RemoteEntries = files
			s.RemoteDir = dir
			s.RemoteInput = dir
			s.PruneRemoteSelection()
			if s.Status == SFTPLoading {
				s.Status = SFTPReady
				s.RemoteReady = true
			}
		})
	}()
}

// refreshLocal lists a local dir on a goroutine.
func (c *Controller) refreshLocal(w *gui.Window, sessID, dir string) {
	st := gui.State[State](w)
	s := st.SFTP[sessID]
	if s == nil {
		return
	}
	if dir == "" {
		dir = s.LocalDir
	}
	s.LocalSeq++
	seq := s.LocalSeq
	s.LocalNav = dir
	showHidden := s.LocalShowHidden
	go func() {
		listing, err := localfs.List(dir, showHidden)
		w.QueueCommand(func(w *gui.Window) {
			st := gui.State[State](w)
			s := st.SFTP[sessID]
			if s == nil || s.LocalSeq != seq {
				return
			}
			s.LocalNav = ""
			if err != nil {
				c.toast(gui.ToastError, "", errtext.Text(err))
				return
			}
			s.LocalDir = listing.Dir
			s.LocalParent = listing.Parent
			s.LocalAtRoot = listing.AtRoot
			s.LocalEntries = listing.Entries
			s.LocalInput = listing.Dir
			s.PruneLocalSelection()
			c.mu.Lock()
			store := c.store
			c.mu.Unlock()
			if store != nil {
				if err := store.SetLastSftpLocalDir(listing.Dir); err != nil {
					c.logger.Warn("persist sftp local dir failed", zap.Error(err))
				}
			}
		})
	}()
}

// navigateRemote moves the remote pane to dir.
func (c *Controller) navigateRemote(w *gui.Window, sessID, dir string) {
	st := gui.State[State](w)
	s := st.SFTP[sessID]
	if s == nil || s.RemoteNav != "" || dir == "" {
		return
	}
	c.refreshRemote(w, sessID, dir)
}

// navigateLocal moves the local pane to dir.
func (c *Controller) navigateLocal(w *gui.Window, sessID, dir string) {
	st := gui.State[State](w)
	s := st.SFTP[sessID]
	if s == nil || s.LocalNav != "" || dir == "" {
		return
	}
	c.refreshLocal(w, sessID, dir)
}

// SFTPGoUpRemote / Local step to the parent directory.
func (c *Controller) SFTPGoUpRemote(w *gui.Window, sessID string) {
	st := gui.State[State](w)
	s := st.SFTP[sessID]
	if s == nil || s.RemoteDir == "" || s.RemoteDir == "/" {
		return
	}
	c.navigateRemote(w, sessID, remoteParent(s.RemoteDir))
}

func (c *Controller) SFTPGoUpLocal(w *gui.Window, sessID string) {
	st := gui.State[State](w)
	s := st.SFTP[sessID]
	if s == nil || s.LocalDir == "" || s.LocalAtRoot {
		return
	}
	c.navigateLocal(w, sessID, s.LocalParent)
}

// SFTPGoToRemote / Local navigate to the path typed in the bar.
func (c *Controller) SFTPGoToRemote(w *gui.Window, sessID, dir string) {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return
	}
	if !strings.HasPrefix(dir, "/") {
		dir = "/" + dir
	}
	c.navigateRemote(w, sessID, pathCleanRemote(dir))
}

func (c *Controller) SFTPGoToLocal(w *gui.Window, sessID, dir string) {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return
	}
	if !filepath.IsAbs(dir) {
		home, herr := os.UserHomeDir()
		switch {
		case herr == nil && dir == "~":
			dir = home
		case herr == nil && strings.HasPrefix(dir, "~/"):
			dir = filepath.Join(home, dir[2:])
		default:
			dir = filepath.Join(c.localDirFor(sessID), dir)
		}
	}
	c.navigateLocal(w, sessID, filepath.Clean(dir))
}

// localDirFor returns the pane's current local dir ("" if none).
func (c *Controller) localDirFor(sessID string) string {
	w := c.Window()
	if w == nil {
		return ""
	}
	st := gui.State[State](w)
	if s := st.SFTP[sessID]; s != nil {
		return s.LocalDir
	}
	return ""
}

// SFTPOpenRemoteDir opens a remote subdirectory by name.
func (c *Controller) SFTPOpenRemoteDir(w *gui.Window, sessID, name string) {
	st := gui.State[State](w)
	s := st.SFTP[sessID]
	if s == nil || s.RemoteDir == "" {
		return
	}
	c.navigateRemote(w, sessID, remoteJoin(s.RemoteDir, name))
}

// SFTPOpenLocalDir opens a local subdirectory by path.
func (c *Controller) SFTPOpenLocalDir(w *gui.Window, sessID, dir string) {
	c.navigateLocal(w, sessID, dir)
}

// SFTPRefresh* re-lists the pane in place.
func (c *Controller) SFTPRefreshRemote(w *gui.Window, sessID string) {
	st := gui.State[State](w)
	s := st.SFTP[sessID]
	if s == nil || s.RemoteDir == "" {
		return
	}
	c.refreshRemote(w, sessID, s.RemoteDir)
}

func (c *Controller) SFTPRefreshLocal(w *gui.Window, sessID string) {
	st := gui.State[State](w)
	s := st.SFTP[sessID]
	if s == nil {
		return
	}
	c.refreshLocal(w, sessID, s.LocalDir)
}

// SFTPToggleHidden flips the dotfile filter and refreshes.
func (c *Controller) SFTPToggleHiddenRemote(w *gui.Window, sessID string) {
	st := gui.State[State](w)
	s := st.SFTP[sessID]
	if s == nil {
		return
	}
	s.RemoteShowHidden = !s.RemoteShowHidden
	if s.RemoteDir != "" {
		c.refreshRemote(w, sessID, s.RemoteDir)
	}
}

func (c *Controller) SFTPToggleHiddenLocal(w *gui.Window, sessID string) {
	st := gui.State[State](w)
	s := st.SFTP[sessID]
	if s == nil {
		return
	}
	s.LocalShowHidden = !s.LocalShowHidden
	c.refreshLocal(w, sessID, s.LocalDir)
}

// refreshAfterTransfer reloads both panes once a transfer ends.
func (c *Controller) refreshAfterTransfer(w *gui.Window, sessID string) {
	st := gui.State[State](w)
	tab := st.TabBySessID(sessID)
	if tab == nil || !tab.SFTPOpen {
		return
	}
	s := st.SFTP[sessID]
	if s == nil {
		return
	}
	if s.RemoteDir != "" {
		c.refreshRemote(w, sessID, s.RemoteDir)
	}
	c.refreshLocal(w, sessID, s.LocalDir)
}

// refreshRemoteAfterEdit reloads the remote pane after an edit upload.
func (c *Controller) refreshRemoteAfterEdit(sessID string) {
	w := c.Window()
	if w == nil {
		return
	}
	w.QueueCommand(func(w *gui.Window) {
		st := gui.State[State](w)
		s := st.SFTP[sessID]
		if s == nil || s.RemoteDir == "" {
			return
		}
		c.refreshRemote(w, sessID, s.RemoteDir)
	})
}

// --- transfers ---

// transfersBusy reports whether a pane navigation is in flight.
func transfersBusy(s *SFTPState) bool {
	return s.LocalNav != "" || s.RemoteNav != ""
}

// SFTPUploadSelected uploads the local pane's selection to the remote
// pane's current directory.
func (c *Controller) SFTPUploadSelected(w *gui.Window, sessID string) {
	st := gui.State[State](w)
	s := st.SFTP[sessID]
	if s == nil {
		return
	}
	paths := s.LocalPaths()
	if len(paths) == 0 {
		return
	}
	c.SFTPUploadLocalPaths(w, sessID, paths)
}

// SFTPDownloadSelected downloads the remote pane's selection into the
// local pane's current directory.
func (c *Controller) SFTPDownloadSelected(w *gui.Window, sessID string) {
	st := gui.State[State](w)
	s := st.SFTP[sessID]
	if s == nil {
		return
	}
	paths := s.RemotePaths()
	if len(paths) == 0 {
		return
	}
	c.SFTPDownloadRemotePaths(w, sessID, paths)
}

// SFTPUploadLocalPaths enqueues uploads of local paths into the remote
// pane dir, prompting on conflicts.
func (c *Controller) SFTPUploadLocalPaths(w *gui.Window, sessID string, localPaths []string) {
	mgr := c.SFTPManager()
	st := gui.State[State](w)
	s := st.SFTP[sessID]
	if mgr == nil || s == nil || len(localPaths) == 0 {
		return
	}
	if transfersBusy(s) {
		c.toast(gui.ToastInfo, "", i18n.T("sftp.please_wait"))
		return
	}
	remoteDir := s.RemoteDir
	if remoteDir == "" {
		remoteDir = "/"
	}
	go func() {
		conflicts, err := mgr.UploadConflicts(sessID, remoteDir, localPaths)
		w.QueueCommand(func(w *gui.Window) {
			st := gui.State[State](w)
			s := st.SFTP[sessID]
			if s == nil {
				return
			}
			if err != nil {
				c.toast(gui.ToastError, "", errtext.Text(err))
				return
			}
			if len(conflicts) > 0 {
				w.Dialog(gui.DialogCfg{
					DialogType:    gui.DialogConfirm,
					Title:         i18n.T("sftp.confirm_overwrite_title"),
					Body:          overwritePrompt(conflicts),
					DefaultButton: gui.DialogButtonNo,
					OnOkYes: func(w *gui.Window) {
						c.enqueueUpload(w, sessID, localPaths, remoteDir)
					},
				})
				return
			}
			c.enqueueUpload(w, sessID, localPaths, remoteDir)
		})
	}()
}

func (c *Controller) enqueueUpload(w *gui.Window, sessID string, localPaths []string, remoteDir string) {
	mgr := c.SFTPManager()
	if mgr == nil {
		return
	}
	paths := append([]string(nil), localPaths...)
	go func() {
		if err := mgr.UploadPaths(sessID, remoteDir, paths); err != nil {
			w.QueueCommand(func(w *gui.Window) {
				c.toast(gui.ToastError, "", errtext.Text(err))
			})
		}
	}()
}

// SFTPDownloadRemotePaths enqueues downloads of remote paths into the
// local pane dir, prompting on conflicts.
func (c *Controller) SFTPDownloadRemotePaths(w *gui.Window, sessID string, remotePaths []string) {
	st := gui.State[State](w)
	s := st.SFTP[sessID]
	if s == nil || len(remotePaths) == 0 {
		return
	}
	localDir := s.LocalDir
	if localDir == "" {
		localDir = c.initialLocalDir()
	}
	c.downloadRemotePathsTo(w, sessID, remotePaths, localDir)
}

// SFTPDownloadRemotePathsDialog asks for a target folder, then downloads
// the remote paths there. The transfer column keeps using the local pane
// dir via SFTPDownloadRemotePaths.
func (c *Controller) SFTPDownloadRemotePathsDialog(w *gui.Window, sessID string, remotePaths []string) {
	st := gui.State[State](w)
	s := st.SFTP[sessID]
	if s == nil || len(remotePaths) == 0 {
		return
	}
	startDir := s.LocalDir
	if startDir == "" {
		startDir = c.lastDialogDir()
	}
	paths := append([]string(nil), remotePaths...)
	w.NativeFolderDialog(gui.NativeFolderDialogCfg{
		Title:    i18n.T("sftp.choose_save_dir"),
		StartDir: startDir,
		OnDone: func(res gui.NativeDialogResult, w *gui.Window) {
			dirs := res.PathStrings()
			if res.Status != gui.DialogOK || len(dirs) == 0 {
				return
			}
			c.rememberDialogPath(dirs[0])
			c.downloadRemotePathsTo(w, sessID, paths, dirs[0])
		},
	})
}

// downloadRemotePathsTo enqueues downloads of remote paths into an
// explicit local dir, prompting on conflicts.
func (c *Controller) downloadRemotePathsTo(w *gui.Window, sessID string, remotePaths []string, localDir string) {
	mgr := c.SFTPManager()
	st := gui.State[State](w)
	s := st.SFTP[sessID]
	if mgr == nil || s == nil || len(remotePaths) == 0 {
		return
	}
	if transfersBusy(s) {
		c.toast(gui.ToastInfo, "", i18n.T("sftp.please_wait"))
		return
	}
	go func() {
		conflicts, err := mgr.DownloadConflicts(sessID, localDir, remotePaths)
		w.QueueCommand(func(w *gui.Window) {
			st := gui.State[State](w)
			s := st.SFTP[sessID]
			if s == nil {
				return
			}
			if err != nil {
				c.toast(gui.ToastError, "", errtext.Text(err))
				return
			}
			if len(conflicts) > 0 {
				w.Dialog(gui.DialogCfg{
					DialogType:    gui.DialogConfirm,
					Title:         i18n.T("sftp.confirm_overwrite_title"),
					Body:          overwritePrompt(conflicts),
					DefaultButton: gui.DialogButtonNo,
					OnOkYes: func(w *gui.Window) {
						c.enqueueDownload(w, sessID, remotePaths, localDir)
					},
				})
				return
			}
			c.enqueueDownload(w, sessID, remotePaths, localDir)
		})
	}()
}

func (c *Controller) enqueueDownload(w *gui.Window, sessID string, remotePaths []string, localDir string) {
	mgr := c.SFTPManager()
	if mgr == nil {
		return
	}
	paths := append([]string(nil), remotePaths...)
	go func() {
		if err := mgr.DownloadPaths(sessID, localDir, paths); err != nil {
			w.QueueCommand(func(w *gui.Window) {
				c.toast(gui.ToastError, "", errtext.Text(err))
			})
		}
	}()
}

// --- native pickers ---

// SFTPUploadFileDialog opens the OS file picker for upload sources.
func (c *Controller) SFTPUploadFileDialog(w *gui.Window, sessID string) {
	st := gui.State[State](w)
	s := st.SFTP[sessID]
	startDir := ""
	if s != nil {
		startDir = s.LocalDir
	}
	if startDir == "" {
		startDir = c.lastDialogDir()
	}
	w.NativeOpenDialog(gui.NativeOpenDialogCfg{
		Title:         i18n.T("sftp.upload_file"),
		StartDir:      startDir,
		AllowMultiple: true,
		OnDone: func(res gui.NativeDialogResult, w *gui.Window) {
			if res.Status != gui.DialogOK || len(res.PathStrings()) == 0 {
				return
			}
			c.rememberDialogPath(res.PathStrings()[0])
			c.SFTPUploadLocalPaths(w, sessID, res.PathStrings())
		},
	})
}

// SFTPUploadFolderDialog opens the OS folder picker for upload.
func (c *Controller) SFTPUploadFolderDialog(w *gui.Window, sessID string) {
	st := gui.State[State](w)
	s := st.SFTP[sessID]
	startDir := ""
	if s != nil {
		startDir = s.LocalDir
	}
	if startDir == "" {
		startDir = c.lastDialogDir()
	}
	w.NativeFolderDialog(gui.NativeFolderDialogCfg{
		Title:    i18n.T("sftp.upload_folder"),
		StartDir: startDir,
		OnDone: func(res gui.NativeDialogResult, w *gui.Window) {
			paths := res.PathStrings()
			if res.Status != gui.DialogOK || len(paths) == 0 {
				return
			}
			c.rememberDialogPath(paths[0])
			c.SFTPUploadLocalPaths(w, sessID, paths)
		},
	})
}

// --- remote file operations ---

// SFTPDeleteRemote confirms then deletes remote paths.
func (c *Controller) SFTPDeleteRemote(w *gui.Window, sessID string, remotePaths []string) {
	mgr := c.SFTPManager()
	st := gui.State[State](w)
	s := st.SFTP[sessID]
	if mgr == nil || s == nil || len(remotePaths) == 0 {
		return
	}
	w.Dialog(gui.DialogCfg{
		DialogType:    gui.DialogConfirm,
		Title:         i18n.T("sftp.confirm_delete_title"),
		Body:          deletePrompt(remotePaths),
		DefaultButton: gui.DialogButtonNo,
		OnOkYes: func(w *gui.Window) {
			paths := append([]string(nil), remotePaths...)
			go func() {
				err := mgr.DeleteFiles(sessID, paths)
				w.QueueCommand(func(w *gui.Window) {
					if err != nil {
						c.toast(gui.ToastError, "", errtext.Text(err))
					}
					c.SFTPRefreshRemote(w, sessID)
				})
			}()
		},
	})
}

// SFTPEditRemote opens a remote file in the OS editor with upload-on-save.
func (c *Controller) SFTPEditRemote(w *gui.Window, sessID, remotePath string) {
	go func() {
		if err := c.EditRemoteFile(sessID, remotePath); err != nil {
			w.QueueCommand(func(w *gui.Window) {
				c.toast(gui.ToastError, "", errtext.Text(err))
			})
		}
	}()
}

// SFTPOpenLocal opens a local path with the OS default handler.
func (c *Controller) SFTPOpenLocal(localPath string) {
	if err := system.OpenPath(localPath); err != nil {
		c.toast(gui.ToastError, "", errtext.Text(err))
	}
}

// SFTPStateFor returns the panel state for rendering.
func (c *Controller) SFTPStateFor(w *gui.Window, sessID string) *SFTPState {
	st := gui.State[State](w)
	return st.SFTP[sessID]
}

// --- transfer list actions ---

// refreshTransfers merges persisted queue rows into the panel.
func (c *Controller) refreshTransfers(w *gui.Window, sessID string) {
	mgr := c.SFTPManager()
	st := gui.State[State](w)
	tab := st.TabBySessID(sessID)
	if mgr == nil || tab == nil {
		return
	}
	ownerKey := tab.OwnerKey
	if ownerKey == "" {
		ownerKey = c.sftpOwnerKey(reconnectReq{draft: tab.Draft})
	}
	go func() {
		pending, err := mgr.ListPendingTransfers(ownerKey)
		if err != nil {
			c.logger.Warn("list pending transfers failed", zap.Error(err))
			return
		}
		w.QueueCommand(func(w *gui.Window) {
			c.mu.Lock()
			// Merge: live rows win over persisted ones with the same ID.
			live := make(map[string]bool, len(c.transfers[sessID]))
			for _, p := range c.transfers[sessID] {
				live[p.ID] = true
			}
			for _, p := range pending {
				p.SessionID = sessID
				if !live[p.ID] {
					c.transfers[sessID] = transferUpsert(c.transfers[sessID], p)
				}
			}
			c.mu.Unlock()
		})
	}()
}

// SFTPCancelTransfer cancels an active transfer.
func (c *Controller) SFTPCancelTransfer(w *gui.Window, sessID, id string) {
	mgr := c.SFTPManager()
	if mgr == nil {
		return
	}
	go func() {
		if err := mgr.CancelTransfer(id); err != nil {
			w.QueueCommand(func(w *gui.Window) {
				c.toast(gui.ToastError, "", errtext.Text(err))
			})
		}
	}()
}

// SFTPRetryTransfer re-queues a failed/interrupted transfer.
func (c *Controller) SFTPRetryTransfer(w *gui.Window, sessID, id string) {
	mgr := c.SFTPManager()
	if mgr == nil {
		return
	}
	go func() {
		if err := mgr.RetryTransfer(sessID, id); err != nil {
			w.QueueCommand(func(w *gui.Window) {
				c.toast(gui.ToastError, "", errtext.Text(err))
			})
		}
	}()
}

// SFTPDismissTransfer removes a finished transfer record.
func (c *Controller) SFTPDismissTransfer(w *gui.Window, sessID, id string) {
	mgr := c.SFTPManager()
	if mgr == nil {
		return
	}
	c.mu.Lock()
	c.transfers[sessID] = transferRemove(c.transfers[sessID], id)
	c.mu.Unlock()
	go func() {
		if err := mgr.DismissTransfer(id); err != nil {
			c.logger.Warn("dismiss transfer failed", zap.String("id", id), zap.Error(err))
		}
	}()
}

// SFTPClearFinished removes completed/failed/cancelled/interrupted
// rows from the list and the queue.
func (c *Controller) SFTPClearFinished(w *gui.Window, sessID string, onlyFailed bool) {
	mgr := c.SFTPManager()
	if mgr == nil {
		return
	}
	c.mu.Lock()
	var ids []string
	kept := c.transfers[sessID][:0]
	for _, p := range c.transfers[sessID] {
		done := p.Done
		failed := p.Error != ""
		match := done && (!onlyFailed || failed)
		if match {
			ids = append(ids, p.ID)
			continue
		}
		kept = append(kept, p)
	}
	c.transfers[sessID] = kept
	c.mu.Unlock()
	go func() {
		for _, id := range ids {
			if err := mgr.DismissTransfer(id); err != nil {
				c.logger.Warn("dismiss transfer failed", zap.String("id", id), zap.Error(err))
			}
		}
	}()
}

// --- session lifecycle ---

// closeSFTPSession tears down SFTP state for a dying session. When
// the SSH session is already dead there is nothing to upload to, so
// edits are discarded without prompting.
func (c *Controller) closeSFTPSession(sessID string) {
	c.DiscardSessionEdits(sessID)
	if mgr := c.SFTPManager(); mgr != nil {
		mgr.CloseSession(sessID)
	}
	go c.CleanupSessionEditDir(sessID)
}

// requestCloseSession runs after-func once unsaved remote edits are
// resolved: uploads them on Yes, discards on No.
func (c *Controller) requestCloseSession(w *gui.Window, sessID string, after func()) {
	if c.UnsavedEditCount(sessID) == 0 {
		after()
		return
	}
	w.Dialog(gui.DialogCfg{
		DialogType:    gui.DialogConfirm,
		Title:         i18n.T("dialog.edit_title"),
		Body:          i18n.T("sftp.confirm_upload_session"),
		DefaultButton: gui.DialogButtonNo,
		OnOkYes: func(w *gui.Window) {
			go func() {
				c.UploadSessionEdits(sessID)
				w.QueueCommand(func(w *gui.Window) { after() })
			}()
		},
		OnCancelNo: func(w *gui.Window) {
			c.DiscardSessionEdits(sessID)
			w.QueueCommand(func(w *gui.Window) { after() })
		},
	})
}

// CloseAllSFTP shuts down every SFTP client and edit watcher.
func (c *Controller) CloseAllSFTP() {
	c.CloseAllEdits()
	if mgr := c.SFTPManager(); mgr != nil {
		mgr.CloseAll()
	}
	c.CleanupAllEditDirs()
}

// --- helpers ---

func pathCleanRemote(p string) string {
	cleaned := path.Clean("/" + strings.TrimPrefix(p, "/"))
	if cleaned == "." || cleaned == "/" {
		return "/"
	}
	return cleaned
}

func pathBaseRemote(p string) string {
	return path.Base(strings.TrimSuffix(p, "/"))
}

func formatModeBits(bits uint32) string {
	if bits <= 0o777 {
		return fmt.Sprintf("%03o", bits)
	}
	return fmt.Sprintf("%04o", bits)
}

// parseID parses a required numeric id field (uid/gid).
func parseID(s string) (int, bool) {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	return n, err == nil
}
