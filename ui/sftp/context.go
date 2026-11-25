package sftpui

import (
	"path"
	"strings"
	"time"

	"github.com/go-gui-org/go-gui/gui"

	"github.com/ilaziness/vela/internal/i18n"
	"github.com/ilaziness/vela/internal/localfs"
	"github.com/ilaziness/vela/internal/sftp"
	"github.com/ilaziness/vela/ui/app"
)

// copyPaths puts paths on the clipboard one per line and confirms it.
func copyPaths(ctx gui.EventCtx, paths []string) {
	ctx.Window.SetClipboard(strings.Join(paths, "\n"))
	ctx.Window.Toast(gui.ToastCfg{
		Body:     i18n.T("common.copied"),
		Severity: gui.ToastSuccess,
		Duration: 2 * time.Second,
	})
}

// --- remote menu items shared by the row and blank-area menus ---

// remotePasteItem is the Paste entry; callers add it only while the
// remote clipboard holds paths.
func remotePasteItem(ctrl *app.Controller, sessID string) gui.MenuItemCfg {
	return gui.MenuItemCfg{ID: "paste", Text: i18n.T("sftp.paste"), Action: func(_ *gui.MenuItemCfg, ctx gui.EventCtx) {
		ctrl.SFTPPasteRemote(ctx.Window, sessID)
	}}
}

func remoteDeselectItem(s *app.SFTPState) gui.MenuItemCfg {
	return gui.MenuItemCfg{ID: "deselect", Text: i18n.T("sftp.deselect"), Action: func(_ *gui.MenuItemCfg, _ gui.EventCtx) {
		s.ClearRemoteSelection()
	}}
}

func remoteRefreshItem(ctrl *app.Controller, sessID string) gui.MenuItemCfg {
	return gui.MenuItemCfg{ID: "refresh", Text: i18n.T("common.refresh"), Action: func(_ *gui.MenuItemCfg, ctx gui.EventCtx) {
		ctrl.SFTPRefreshRemote(ctx.Window, sessID)
	}}
}

// remoteNewItems returns the New File / New Folder entries.
func remoteNewItems(ctrl *app.Controller, sessID string) []gui.MenuItemCfg {
	return []gui.MenuItemCfg{
		{ID: "newfile", Text: i18n.T("sftp.new_file"), Action: func(_ *gui.MenuItemCfg, ctx gui.EventCtx) {
			ctrl.SFTPCreateRemote(ctx.Window, sessID, false)
		}},
		{ID: "newfolder", Text: i18n.T("sftp.new_folder"), Action: func(_ *gui.MenuItemCfg, ctx gui.EventCtx) {
			ctrl.SFTPCreateRemote(ctx.Window, sessID, true)
		}},
	}
}

// remoteUploadItems returns the Upload File / Upload Folder entries.
func remoteUploadItems(ctrl *app.Controller, sessID string) []gui.MenuItemCfg {
	return []gui.MenuItemCfg{
		{ID: "uploadfile", Text: i18n.T("sftp.upload_file"), Action: func(_ *gui.MenuItemCfg, ctx gui.EventCtx) {
			ctrl.SFTPUploadFileDialog(ctx.Window, sessID)
		}},
		{ID: "uploadfolder", Text: i18n.T("sftp.upload_folder"), Action: func(_ *gui.MenuItemCfg, ctx gui.EventCtx) {
			ctrl.SFTPUploadFolderDialog(ctx.Window, sessID)
		}},
	}
}

// remoteItemMenu builds the right-click menu for one remote row.
func remoteItemMenu(ctrl *app.Controller, tab *app.Tab, s *app.SFTPState, e sftp.FileInfo) gui.ContextMenuCfg {
	sessID := tab.SessID
	path := remotePathOf(s, e.Name)
	// targetPaths returns the selected paths, falling back to the row
	// the menu was opened on.
	targetPaths := func() []string {
		paths := s.RemotePaths()
		if len(paths) == 0 {
			paths = []string{path}
		}
		return paths
	}
	var items []gui.MenuItemCfg
	// Directories cannot be opened in the editor.
	if !e.IsDir {
		items = append(items, gui.MenuItemCfg{ID: "edit", Text: i18n.T("common.edit"), Action: func(_ *gui.MenuItemCfg, ctx gui.EventCtx) {
			ctrl.SFTPEditRemote(ctx.Window, sessID, path)
		}})
	}
	items = append(items,
		gui.MenuItemCfg{ID: "copy", Text: i18n.T("common.copy"), Action: func(_ *gui.MenuItemCfg, ctx gui.EventCtx) {
			ctrl.SFTPCopyRemote(ctx.Window, sessID, targetPaths())
		}},
		gui.MenuItemCfg{ID: "cut", Text: i18n.T("sftp.cut"), Action: func(_ *gui.MenuItemCfg, ctx gui.EventCtx) {
			ctrl.SFTPCutRemote(ctx.Window, sessID, targetPaths())
		}},
	)
	// Paste only appears while the remote clipboard holds paths.
	if s.HasClipboard() {
		items = append(items, remotePasteItem(ctrl, sessID))
	}
	items = append(items,
		gui.MenuSeparator(),
		gui.MenuItemCfg{ID: "compress", Text: i18n.T("sftp.compress"), Action: func(_ *gui.MenuItemCfg, ctx gui.EventCtx) {
			ctrl.SFTPCompressRemote(ctx.Window, sessID, entryNames(targetPaths()))
		}},
		gui.MenuItemCfg{ID: "decompress", Text: i18n.T("sftp.decompress"), Action: func(_ *gui.MenuItemCfg, ctx gui.EventCtx) {
			ctrl.SFTPDecompressRemote(ctx.Window, sessID, targetPaths())
		}},
		gui.MenuSeparator(),
		gui.MenuItemCfg{ID: "download", Text: i18n.T("sftp.download"), Action: func(_ *gui.MenuItemCfg, ctx gui.EventCtx) {
			// The row menu asks for a target folder; the transfer column
			// button keeps downloading into the local pane dir.
			ctrl.SFTPDownloadRemotePathsDialog(ctx.Window, sessID, targetPaths())
		}},
		gui.MenuItemCfg{ID: "copy_path", Text: i18n.T("sftp.copy_path"), Action: func(_ *gui.MenuItemCfg, ctx gui.EventCtx) {
			copyPaths(ctx, targetPaths())
		}},
		gui.MenuItemCfg{ID: "delete", Text: i18n.T("common.delete"), Action: func(_ *gui.MenuItemCfg, ctx gui.EventCtx) {
			ctrl.SFTPDeleteRemote(ctx.Window, sessID, targetPaths())
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
	)
	items = append(items, gui.MenuSeparator(),
		remoteDeselectItem(s),
		remoteRefreshItem(ctrl, sessID),
		gui.MenuSeparator(),
	)
	items = append(items, remoteNewItems(ctrl, sessID)...)
	items = append(items, remoteUploadItems(ctrl, sessID)...)

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

// entryNames reduces full remote paths to basenames for archive members.
func entryNames(paths []string) []string {
	out := make([]string, 0, len(paths))
	for _, p := range paths {
		out = append(out, path.Base(p))
	}
	return out
}

// localItemMenu builds the right-click menu for one local row.
func localItemMenu(ctrl *app.Controller, tab *app.Tab, s *app.SFTPState, e localfs.Entry) gui.ContextMenuCfg {
	sessID := tab.SessID
	// targetPaths returns the selected paths, falling back to the row
	// the menu was opened on.
	targetPaths := func() []string {
		paths := s.LocalPaths()
		if len(paths) == 0 {
			paths = []string{e.Path}
		}
		return paths
	}
	return gui.ContextMenuCfg{
		ID:     "sftp-local-item-menu",
		Sizing: gui.FillFit, // width fills the pane, height hugs the row
		// The wrapper Column otherwise takes the theme's medium
		// padding, which inflates every row's spacing.
		Padding: gui.PaddingNone,
		Items: []gui.MenuItemCfg{
			{ID: "copy_path", Text: i18n.T("sftp.copy_path"), Action: func(_ *gui.MenuItemCfg, ctx gui.EventCtx) {
				copyPaths(ctx, targetPaths())
			}},
			gui.MenuSeparator(),
			{ID: "refresh", Text: i18n.T("common.refresh"), Action: func(_ *gui.MenuItemCfg, ctx gui.EventCtx) {
				ctrl.SFTPRefreshLocal(ctx.Window, sessID)
			}},
		},
		Content: []gui.View{},
		OnAnyClick: func(ctx gui.EventCtx) {
			// Right-click on an unselected row selects only that row
			// so menu actions apply to it.
			ev := ctx.Event
			if ev != nil && ev.Type == gui.EventMouseDown && ev.MouseButton == gui.MouseRight {
				if !s.LocalSelected[e.Path] {
					s.LocalSelected = map[string]bool{e.Path: true}
				}
			}
		},
	}
}

// localBlankMenu is the right-click menu on empty local pane area.
func localBlankMenu(ctrl *app.Controller, tab *app.Tab) gui.ContextMenuCfg {
	sessID := tab.SessID
	return gui.ContextMenuCfg{
		ID:      "sftp-local-blank-menu",
		Sizing:  gui.FillFill,
		Padding: gui.PaddingNone,
		Items: []gui.MenuItemCfg{
			{ID: "refresh", Text: i18n.T("common.refresh"), Action: func(_ *gui.MenuItemCfg, ctx gui.EventCtx) {
				ctrl.SFTPRefreshLocal(ctx.Window, sessID)
			}},
		},
	}
}

// remoteBlankMenu is the right-click menu on empty remote pane area.
func remoteBlankMenu(ctrl *app.Controller, tab *app.Tab, s *app.SFTPState) gui.ContextMenuCfg {
	sessID := tab.SessID
	items := []gui.MenuItemCfg{}
	if s.HasClipboard() {
		items = append(items, remotePasteItem(ctrl, sessID))
	}
	items = append(items,
		remoteDeselectItem(s),
		remoteRefreshItem(ctrl, sessID),
		gui.MenuSeparator(),
	)
	items = append(items, remoteNewItems(ctrl, sessID)...)
	items = append(items, gui.MenuSeparator())
	items = append(items, remoteUploadItems(ctrl, sessID)...)
	return gui.ContextMenuCfg{
		ID:      "sftp-remote-blank-menu",
		Sizing:  gui.FillFill,
		Padding: gui.PaddingNone,
		Items:   items,
	}
}
