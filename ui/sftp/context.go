package sftpui

import (
	"github.com/go-gui-org/go-gui/gui"

	"github.com/ilaziness/vela/internal/i18n"
	"github.com/ilaziness/vela/internal/sftp"
	"github.com/ilaziness/vela/ui/app"
)

// remoteItemMenu builds the right-click menu for one remote row.
func remoteItemMenu(ctrl *app.Controller, tab *app.Tab, s *app.SFTPState, e sftp.FileInfo) gui.ContextMenuCfg {
	sessID := tab.SessID
	path := remotePathOf(s, e.Name)
	items := []gui.MenuItemCfg{
		{ID: "edit", Text: i18n.T("common.edit"), Action: func(_ *gui.MenuItemCfg, ctx gui.EventCtx) {
			ctrl.SFTPEditRemote(ctx.Window, sessID, path)
		}},
		{ID: "download", Text: i18n.T("sftp.download"), Action: func(_ *gui.MenuItemCfg, ctx gui.EventCtx) {
			paths := s.RemotePaths()
			if len(paths) == 0 {
				paths = []string{path}
			}
			ctrl.SFTPDownloadRemotePaths(ctx.Window, sessID, paths)
		}},
		{ID: "delete", Text: i18n.T("common.delete"), Action: func(_ *gui.MenuItemCfg, ctx gui.EventCtx) {
			paths := s.RemotePaths()
			if len(paths) == 0 {
				paths = []string{path}
			}
			ctrl.SFTPDeleteRemote(ctx.Window, sessID, paths)
		}},
		gui.MenuItemCfg{ID: "rename", Text: i18n.T("common.rename"), Action: func(_ *gui.MenuItemCfg, ctx gui.EventCtx) {
			ctrl.SFTPRenameRemote(ctx.Window, sessID, path)
		}},
		gui.MenuItemCfg{ID: "chmod", Text: i18n.T("sftp.chmod"), Action: func(_ *gui.MenuItemCfg, ctx gui.EventCtx) {
			ctrl.SFTPChmodRemote(ctx.Window, sessID, path)
		}},
		gui.MenuItemCfg{ID: "chown", Text: i18n.T("sftp.chown"), Action: func(_ *gui.MenuItemCfg, ctx gui.EventCtx) {
			ctrl.SFTPChownRemote(ctx.Window, sessID, path)
		}},
	}
	items = append(items, gui.MenuSeparator(),
		gui.MenuItemCfg{ID: "deselect", Text: i18n.T("sftp.deselect"), Action: func(_ *gui.MenuItemCfg, ctx gui.EventCtx) {
			s.ClearRemoteSelection()
		}},
		gui.MenuItemCfg{ID: "refresh", Text: i18n.T("common.refresh"), Action: func(_ *gui.MenuItemCfg, ctx gui.EventCtx) {
			ctrl.SFTPRefreshRemote(ctx.Window, sessID)
		}},
		gui.MenuSeparator(),
		gui.MenuItemCfg{ID: "newfile", Text: i18n.T("sftp.new_file"), Action: func(_ *gui.MenuItemCfg, ctx gui.EventCtx) {
			ctrl.SFTPCreateRemote(ctx.Window, sessID, false)
		}},
		gui.MenuItemCfg{ID: "newfolder", Text: i18n.T("sftp.new_folder"), Action: func(_ *gui.MenuItemCfg, ctx gui.EventCtx) {
			ctrl.SFTPCreateRemote(ctx.Window, sessID, true)
		}},
		gui.MenuItemCfg{ID: "uploadfile", Text: i18n.T("sftp.upload_file"), Action: func(_ *gui.MenuItemCfg, ctx gui.EventCtx) {
			ctrl.SFTPUploadFileDialog(ctx.Window, sessID)
		}},
		gui.MenuItemCfg{ID: "uploadfolder", Text: i18n.T("sftp.upload_folder"), Action: func(_ *gui.MenuItemCfg, ctx gui.EventCtx) {
			ctrl.SFTPUploadFolderDialog(ctx.Window, sessID)
		}},
	)

	return gui.ContextMenuCfg{
		ID:     "sftp-remote-menu",
		Sizing: gui.FillFit, // width fills the pane, height hugs the row
		// The wrapper Column otherwise takes the theme's medium
		// padding, which inflates every row's spacing.
		Padding: gui.PaddingNone,
		Items:   items,
		Content: []gui.View{},
		OnAnyClick: func(ctx gui.EventCtx) {
			// Right-click on an unselected row selects only that row
			// so menu actions apply to it.
			ev := ctx.Event
			if ev != nil && ev.Type == gui.EventMouseDown && ev.MouseButton == gui.MouseRight {
				name := e.Name
				if !s.RemoteSelected[name] {
					s.ClearRemoteSelection()
					s.RemoteSelected[name] = true
				}
			}
		},
	}
}

// remoteBlankMenu is the right-click menu on empty remote pane area.
func remoteBlankMenu(ctrl *app.Controller, tab *app.Tab, s *app.SFTPState) gui.ContextMenuCfg {
	sessID := tab.SessID
	return gui.ContextMenuCfg{
		ID:      "sftp-remote-blank-menu",
		Sizing:  gui.FillFill,
		Padding: gui.PaddingNone,
		Items: []gui.MenuItemCfg{
			{ID: "deselect", Text: i18n.T("sftp.deselect"), Action: func(_ *gui.MenuItemCfg, ctx gui.EventCtx) {
				s.ClearRemoteSelection()
			}},
			{ID: "refresh", Text: i18n.T("common.refresh"), Action: func(_ *gui.MenuItemCfg, ctx gui.EventCtx) {
				ctrl.SFTPRefreshRemote(ctx.Window, sessID)
			}},
			gui.MenuSeparator(),
			{ID: "newfile", Text: i18n.T("sftp.new_file"), Action: func(_ *gui.MenuItemCfg, ctx gui.EventCtx) {
				ctrl.SFTPCreateRemote(ctx.Window, sessID, false)
			}},
			{ID: "newfolder", Text: i18n.T("sftp.new_folder"), Action: func(_ *gui.MenuItemCfg, ctx gui.EventCtx) {
				ctrl.SFTPCreateRemote(ctx.Window, sessID, true)
			}},
			gui.MenuSeparator(),
			{ID: "uploadfile", Text: i18n.T("sftp.upload_file"), Action: func(_ *gui.MenuItemCfg, ctx gui.EventCtx) {
				ctrl.SFTPUploadFileDialog(ctx.Window, sessID)
			}},
			{ID: "uploadfolder", Text: i18n.T("sftp.upload_folder"), Action: func(_ *gui.MenuItemCfg, ctx gui.EventCtx) {
				ctrl.SFTPUploadFolderDialog(ctx.Window, sessID)
			}},
		},
	}
}
