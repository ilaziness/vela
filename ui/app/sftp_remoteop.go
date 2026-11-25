package app

import (
	"context"
	"errors"
	"path"

	"github.com/go-gui-org/go-gui/gui"
	"go.uber.org/zap"

	"github.com/ilaziness/vela/internal/i18n"
	"github.com/ilaziness/vela/internal/sftp"
	"github.com/ilaziness/vela/ui/errtext"
	"github.com/ilaziness/vela/ui/layout"
)

// Remote clipboard operations and server-side archive commands for the
// SFTP panel. All heavy work runs through ssh.Manager.Exec on the
// remote host; the UI shows a busy dialog with a cancel button.

// SFTPCopyRemote puts remote paths on the clipboard (copy).
func (c *Controller) SFTPCopyRemote(w *gui.Window, sessID string, remotePaths []string) {
	st := gui.State[State](w)
	s := st.SFTP[sessID]
	if s == nil || len(remotePaths) == 0 {
		return
	}
	s.ClipOp = clipCopy
	s.ClipPaths = append([]string(nil), remotePaths...)
	c.toast(gui.ToastSuccess, "", i18n.T("common.copied"))
}

// SFTPCutRemote puts remote paths on the clipboard (move).
func (c *Controller) SFTPCutRemote(w *gui.Window, sessID string, remotePaths []string) {
	st := gui.State[State](w)
	s := st.SFTP[sessID]
	if s == nil || len(remotePaths) == 0 {
		return
	}
	s.ClipOp = clipCut
	s.ClipPaths = append([]string(nil), remotePaths...)
	c.toast(gui.ToastSuccess, "", i18n.T("sftp.cut_done"))
}

// SFTPPasteRemote pastes the clipboard into the remote pane's current
// directory, prompting on name conflicts.
func (c *Controller) SFTPPasteRemote(w *gui.Window, sessID string) {
	mgr := c.SFTPManager()
	st := gui.State[State](w)
	s := st.SFTP[sessID]
	if mgr == nil || s == nil || !s.HasClipboard() {
		return
	}
	op := s.ClipOp
	paths := append([]string(nil), s.ClipPaths...)
	destDir := s.RemoteDir
	if destDir == "" {
		destDir = "/"
	}
	label := i18n.T("sftp.busy_copy")
	if op == clipCut {
		label = i18n.T("sftp.busy_move")
	}
	go func() {
		conflicts, err := mgr.RemotePasteConflicts(sessID, destDir, paths)
		w.QueueCommand(func(w *gui.Window) {
			if err != nil {
				c.logger.Warn("sftp paste conflict check failed",
					zap.String("session", sessID), zap.Error(err))
				c.toast(gui.ToastError, "", errtext.Text(err))
				return
			}
			run := func(ww *gui.Window) {
				c.runRemoteOp(ww, sessID, label, func() error {
					if op == clipCut {
						return mgr.MoveRemotePaths(sessID, paths, destDir)
					}
					return mgr.CopyRemotePaths(sessID, paths, destDir)
				}, func(ww *gui.Window) {
					// A successful cut consumes the clipboard; copy keeps
					// it so the same paths can be pasted again.
					if op != clipCut {
						return
					}
					if cs := gui.State[State](ww).SFTP[sessID]; cs != nil {
						cs.ClipOp = clipNone
						cs.ClipPaths = nil
					}
				})
			}
			if len(conflicts) > 0 {
				w.Dialog(gui.DialogCfg{
					DialogType:    gui.DialogConfirm,
					Title:         i18n.T("sftp.confirm_overwrite_title"),
					Body:          overwritePrompt(conflicts),
					DefaultButton: gui.DialogButtonNo,
					OnOkYes:       run,
				})
				return
			}
			run(w)
		})
	}()
}

// SFTPCompressRemote opens the format picker for the selected remote
// entries (basenames inside the remote pane's current directory).
func (c *Controller) SFTPCompressRemote(w *gui.Window, sessID string, names []string) {
	mgr := c.SFTPManager()
	st := gui.State[State](w)
	s := st.SFTP[sessID]
	if mgr == nil || s == nil || len(names) == 0 {
		return
	}
	dir := s.RemoteDir
	if dir == "" {
		dir = "/"
	}
	formats := sftp.CompressFormats()
	buttons := make([]gui.View, 0, len(formats))
	for _, f := range formats {
		format := f
		buttons = append(buttons, gui.Button(gui.ButtonCfg{
			ID:      "sftp-compress-" + format,
			Label:   "." + format,
			Variant: gui.ButtonSecondary,
			OnClick: func(ctx gui.EventCtx) {
				ctx.Consume()
				ww := ctx.Window
				ww.DialogDismiss()
				c.runRemoteOp(ww, sessID, i18n.T("sftp.busy_compress"), func() error {
					_, err := mgr.CompressRemote(sessID, dir, names, format)
					return err
				}, nil)
			},
		}))
	}
	w.Dialog(gui.DialogCfg{
		DialogType: gui.DialogCustom,
		Title:      i18n.T("sftp.compress_title"),
		Width:      280,
		CustomView: func(*gui.Window) gui.View {
			return gui.Column(layout.Flush(gui.ContainerCfg{
				Sizing:  gui.FillFit,
				Spacing: gui.SpacingSmall,
				Content: buttons,
			}))
		},
	})
}

// SFTPDecompressRemote extracts the selected remote archives after a
// generic overwrite confirmation.
func (c *Controller) SFTPDecompressRemote(w *gui.Window, sessID string, remotePaths []string) {
	mgr := c.SFTPManager()
	st := gui.State[State](w)
	s := st.SFTP[sessID]
	if mgr == nil || s == nil || len(remotePaths) == 0 {
		return
	}
	var archives []string
	for _, p := range remotePaths {
		if sftp.IsArchiveName(path.Base(p)) {
			archives = append(archives, p)
		}
	}
	if len(archives) == 0 {
		c.toast(gui.ToastError, "", i18n.T("sftp.no_archive_items"))
		return
	}
	w.Dialog(gui.DialogCfg{
		DialogType:    gui.DialogConfirm,
		Title:         i18n.T("sftp.decompress"),
		Body:          i18n.T("sftp.confirm_extract_overwrite"),
		DefaultButton: gui.DialogButtonNo,
		OnOkYes: func(ww *gui.Window) {
			paths := append([]string(nil), archives...)
			c.runRemoteOp(ww, sessID, i18n.T("sftp.busy_extract"), func() error {
				return mgr.ExtractRemote(sessID, paths)
			}, nil)
		},
	})
}

// runRemoteOp single-flights a remote exec op behind the busy dialog.
// fn runs on a goroutine; completion dismisses the dialog, toasts the
// result and refreshes the remote pane. onSuccess (optional) runs on the
// UI thread only when fn succeeded.
func (c *Controller) runRemoteOp(w *gui.Window, sessID, label string, fn func() error, onSuccess func(*gui.Window)) {
	c.mu.Lock()
	if c.remoteOpRunning {
		c.mu.Unlock()
		c.toast(gui.ToastInfo, "", i18n.T("sftp.please_wait"))
		return
	}
	c.remoteOpRunning = true
	c.mu.Unlock()
	mgr := c.SFTPManager()
	layout.BusyDialog(w, label, func() {
		if mgr != nil {
			mgr.CancelRemoteOp(sessID)
		}
	})
	go func() {
		err := fn()
		w.QueueCommand(func(w *gui.Window) {
			c.mu.Lock()
			c.remoteOpRunning = false
			c.mu.Unlock()
			w.DialogDismiss()
			switch {
			case err == nil:
				if onSuccess != nil {
					onSuccess(w)
				}
				c.toast(gui.ToastSuccess, "", i18n.T("sftp.op_done"))
			case errors.Is(err, context.Canceled):
				c.toast(gui.ToastInfo, "", i18n.T("sftp.op_cancelled"))
			default:
				c.logger.Warn("sftp remote op failed",
					zap.String("session", sessID), zap.String("op", label), zap.Error(err))
				c.toast(gui.ToastError, "", errtext.Text(err))
			}
			c.SFTPRefreshRemote(w, sessID)
		})
	}()
}
