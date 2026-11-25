package app

import (
	"strconv"
	"strings"

	"github.com/go-gui-org/go-gui/gui"

	"github.com/ilaziness/vela/internal/i18n"
	"github.com/ilaziness/vela/ui/errtext"
)

// Remote file-operation dialogs: rename, create, chmod, chown. Native
// prompt/confirm dialogs capture their arguments in closures; only chown
// needs a custom view because it has two inputs.

// SFTPRenameRemote prompts for a new name then renames.
func (c *Controller) SFTPRenameRemote(w *gui.Window, sessID, remotePath string) {
	st := gui.State[State](w)
	s := st.SFTP[sessID]
	if s == nil {
		return
	}
	name := pathBaseRemote(remotePath)
	w.Dialog(gui.DialogCfg{
		DialogType: gui.DialogPrompt,
		Title:      i18n.T("common.rename"),
		Body:       i18n.T("sftp.new_name"),
		Reply:      name,
		OnReply: func(reply string, w *gui.Window) {
			c.doRenameRemote(w, sessID, remotePath, reply)
		},
	})
}

func (c *Controller) doRenameRemote(w *gui.Window, sessID, remotePath, newName string) {
	mgr := c.SFTPManager()
	st := gui.State[State](w)
	s := st.SFTP[sessID]
	if mgr == nil || s == nil {
		return
	}
	newName = strings.TrimSpace(newName)
	if !validRemoteName(newName) {
		c.toast(gui.ToastError, "", i18n.T("sftp.invalid_remote_path"))
		return
	}
	if newName == pathBaseRemote(remotePath) {
		return
	}
	newPath := remoteJoin(s.RemoteDir, newName)
	go func() {
		err := mgr.RenameFile(sessID, remotePath, newPath)
		w.QueueCommand(func(w *gui.Window) {
			if err != nil {
				c.toast(gui.ToastError, "", errtext.Text(err))
			}
			c.SFTPRefreshRemote(w, sessID)
		})
	}()
}

// SFTPCreateRemote prompts for a name then creates a file or folder.
func (c *Controller) SFTPCreateRemote(w *gui.Window, sessID string, isDir bool) {
	mgr := c.SFTPManager()
	st := gui.State[State](w)
	s := st.SFTP[sessID]
	if mgr == nil || s == nil || s.RemoteDir == "" {
		return
	}
	title := i18n.T("sftp.new_file")
	label := i18n.T("sftp.filename")
	if isDir {
		title = i18n.T("sftp.new_folder")
		label = i18n.T("sftp.folder_name")
	}
	w.Dialog(gui.DialogCfg{
		DialogType: gui.DialogPrompt,
		Title:      i18n.T("sftp.create"),
		Body:       title + " — " + label,
		OnReply: func(reply string, w *gui.Window) {
			c.doCreateRemote(w, sessID, isDir, reply)
		},
	})
}

func (c *Controller) doCreateRemote(w *gui.Window, sessID string, isDir bool, name string) {
	mgr := c.SFTPManager()
	st := gui.State[State](w)
	s := st.SFTP[sessID]
	if mgr == nil || s == nil {
		return
	}
	name = strings.TrimSpace(name)
	if !validRemoteName(name) {
		c.toast(gui.ToastError, "", i18n.T("sftp.invalid_remote_path"))
		return
	}
	target := remoteJoin(s.RemoteDir, name)
	go func() {
		var err error
		if isDir {
			err = mgr.CreateDirectory(sessID, target)
		} else {
			err = mgr.CreateFile(sessID, target)
		}
		w.QueueCommand(func(w *gui.Window) {
			if err != nil {
				c.toast(gui.ToastError, "", errtext.Text(err))
			}
			c.SFTPRefreshRemote(w, sessID)
		})
	}()
}

// SFTPChmodRemote prompts for octal/symbolic mode then applies it.
func (c *Controller) SFTPChmodRemote(w *gui.Window, sessID, remotePath string) {
	mgr := c.SFTPManager()
	st := gui.State[State](w)
	s := st.SFTP[sessID]
	if mgr == nil || s == nil {
		return
	}
	// Pre-fill with the current octal mode when known.
	reply := ""
	if e, ok := s.RemoteEntryByName(pathBaseRemote(remotePath)); ok && e.ModeBits != 0 {
		reply = formatModeBits(e.ModeBits)
	}
	w.Dialog(gui.DialogCfg{
		DialogType: gui.DialogPrompt,
		Title:      i18n.T("sftp.chmod_title"),
		Body:       i18n.T("sftp.chmod_example"),
		Reply:      reply,
		OnReply: func(reply string, w *gui.Window) {
			c.doChmodRemote(w, sessID, remotePath, reply)
		},
	})
}

func (c *Controller) doChmodRemote(w *gui.Window, sessID, remotePath, input string) {
	mgr := c.SFTPManager()
	if mgr == nil {
		return
	}
	bits, ok := parseModeInput(input)
	if !ok {
		c.toast(gui.ToastError, "", i18n.T("sftp.invalid_chmod"))
		return
	}
	go func() {
		err := mgr.Chmod(sessID, remotePath, bits)
		w.QueueCommand(func(w *gui.Window) {
			if err != nil {
				c.toast(gui.ToastError, "", errtext.Text(err))
			}
			c.SFTPRefreshRemote(w, sessID)
		})
	}()
}

// SFTPChownRemote prompts for uid/gid via a custom dialog.
func (c *Controller) SFTPChownRemote(w *gui.Window, sessID, remotePath string) {
	mgr := c.SFTPManager()
	st := gui.State[State](w)
	s := st.SFTP[sessID]
	if mgr == nil || s == nil {
		return
	}
	// Pre-fill with the entry's current uid/gid like v1 did.
	dlg := sftpDialog{path: remotePath}
	if e, ok := s.RemoteEntryByName(pathBaseRemote(remotePath)); ok {
		dlg.input = strconv.Itoa(e.UID)
		dlg.input2 = strconv.Itoa(e.GID)
	}
	s.dlg = dlg
	w.Dialog(gui.DialogCfg{
		DialogType: gui.DialogCustom,
		Title:      i18n.T("sftp.chown_title"),
		Body:       i18n.Tf("sftp.chown_requires_privilege", pathBaseRemote(remotePath)),
		CustomView: func(w *gui.Window) gui.View {
			return c.chownDialogBody(w, sessID)
		},
	})
}

// chownDialogBody builds the uid/gid inputs for the chown dialog.
// Runs during dialog layout generation; reads/writes s.dlg.
func (c *Controller) chownDialogBody(w *gui.Window, sessID string) gui.View {
	st := gui.State[State](w)
	s := st.SFTP[sessID]
	dlg := sftpDialog{}
	if s != nil {
		dlg = s.dlg
	}
	return gui.Column(gui.ContainerCfg{
		Sizing:  gui.FillFit,
		Spacing: gui.SpacingMedium,
		Content: []gui.View{
			gui.Input(gui.InputCfg{
				ID:    "sftp-chown-uid",
				Label: "UID",
				Text:  dlg.input,
				OnTextChanged: func(text string, ctx gui.EventCtx) {
					if s := gui.State[State](ctx.Window).SFTP[sessID]; s != nil {
						s.dlg.input = text
					}
				},
			}),
			gui.Input(gui.InputCfg{
				ID:    "sftp-chown-gid",
				Label: "GID",
				Text:  dlg.input2,
				OnTextChanged: func(text string, ctx gui.EventCtx) {
					if s := gui.State[State](ctx.Window).SFTP[sessID]; s != nil {
						s.dlg.input2 = text
					}
				},
			}),
			gui.Row(gui.ContainerCfg{
				Sizing:  gui.FillFit,
				HAlign:  gui.HAlignEnd,
				Spacing: gui.SpacingSmall,
				Content: []gui.View{
					gui.Button(gui.ButtonCfg{
						ID:      "sftp-chown-ok",
						Label:   i18n.T("common.ok"),
						Variant: gui.ButtonPrimary,
						OnClick: func(ctx gui.EventCtx) {
							ctx.Consume()
							ww := ctx.Window
							st := gui.State[State](ww)
							s := st.SFTP[sessID]
							if s == nil {
								return
							}
							d := s.dlg
							s.dlg = sftpDialog{}
							ww.DialogDismiss()
							c.doChownRemote(ww, sessID, d.path, d.input, d.input2)
						},
					}),
					gui.Button(gui.ButtonCfg{
						ID:      "sftp-chown-cancel",
						Label:   i18n.T("common.cancel"),
						Variant: gui.ButtonSecondary,
						OnClick: func(ctx gui.EventCtx) {
							ctx.Consume()
							ww := ctx.Window
							st := gui.State[State](ww)
							if s := st.SFTP[sessID]; s != nil {
								s.dlg = sftpDialog{}
							}
							ww.DialogDismiss()
						},
					}),
				},
			}),
		},
	})
}

func (c *Controller) doChownRemote(w *gui.Window, sessID, remotePath, uidStr, gidStr string) {
	mgr := c.SFTPManager()
	if mgr == nil {
		return
	}
	uid, uidOK := parseID(uidStr)
	gid, gidOK := parseID(gidStr)
	if !uidOK || !gidOK || uid < 0 || gid < 0 {
		c.toast(gui.ToastError, "", i18n.T("sftp.invalid_uid_gid"))
		return
	}
	go func() {
		err := mgr.Chown(sessID, remotePath, uid, gid)
		w.QueueCommand(func(w *gui.Window) {
			if err != nil {
				c.toast(gui.ToastError, "", errtext.Text(err))
			}
			c.SFTPRefreshRemote(w, sessID)
		})
	}()
}
