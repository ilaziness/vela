package sftpui

import (
	"fmt"
	"time"

	"github.com/go-gui-org/go-gui/gui"

	"github.com/ilaziness/vela/internal/i18n"
	"github.com/ilaziness/vela/internal/localfs"
	"github.com/ilaziness/vela/internal/sftp"
	"github.com/ilaziness/vela/ui/app"
	"github.com/ilaziness/vela/ui/layout"
)

// fileHeader renders the column header row.
func fileHeader(remote bool) gui.View {
	th := gui.CurrentTheme()
	return gui.Row(layout.Flush(gui.ContainerCfg{
		ID:      headerID(remote),
		Sizing:  gui.FillFixed,
		Height:  paneHeadHeight,
		VAlign:  gui.VAlignMiddle,
		Spacing: gui.SpacingTight,
		Padding: gui.NewPadding(0, 6, 0, 6),
		Color:   th.ColorPanel,
		Content: []gui.View{
			gui.Text(gui.TextCfg{Text: i18n.T("sftp.name"), TextStyle: th.TextStyleLabel, Sizing: gui.FillFit}),
			fixedCell(i18n.T("sftp.size"), 90, th),
			fixedCell(i18n.T("sftp.modified"), 150, th),
		},
	}))
}

func headerID(remote bool) string {
	if remote {
		return "sftp-remote-head"
	}
	return "sftp-local-head"
}

// remoteFileList renders the remote pane's scrollable rows. The whole
// list is wrapped in the blank-area context menu; rows carry their own
// menu which wins on right-click.
func remoteFileList(w *gui.Window, ctrl *app.Controller, tab *app.Tab, s *app.SFTPState) gui.View {
	th := gui.CurrentTheme()
	rows := make([]gui.View, 0, len(s.RemoteEntries)+1)
	for i, e := range s.RemoteEntries {
		rows = append(rows, remoteRow(w, ctrl, tab, s, e, i))
	}
	if len(rows) == 0 {
		rows = append(rows, emptyRow(th))
	}
	list := gui.Column(layout.Flush(gui.ContainerCfg{
		ID:         "sftp-remote-list",
		Sizing:     gui.FillFill,
		Scrollable: true,
		// No ClipContents: its stencil masks the overlaid scrollbar too,
		// so the list scrolled with no bar. Clip still scissors the rows.
		Clip: true,
		// Right gutter keeps rows clear of the overlay scrollbar.
		Padding: gui.NewPadding(0, 12, 0, 0),
		Spacing: gui.NoSpacing,
		Content: rows,
	}))
	cfg := remoteBlankMenu(ctrl, tab, s)
	cfg.Content = []gui.View{list}
	return gui.ContextMenu(w, cfg)
}

// localFileList renders the local pane's scrollable rows.
func localFileList(w *gui.Window, ctrl *app.Controller, tab *app.Tab, s *app.SFTPState) gui.View {
	th := gui.CurrentTheme()
	rows := make([]gui.View, 0, len(s.LocalEntries)+1)
	for i, e := range s.LocalEntries {
		rows = append(rows, localRow(w, ctrl, tab, s, e, i))
	}
	if len(rows) == 0 {
		rows = append(rows, emptyRow(th))
	}
	return gui.Column(layout.Flush(gui.ContainerCfg{
		ID:         "sftp-local-list",
		Sizing:     gui.FillFill,
		Scrollable: true,
		// No ClipContents: its stencil masks the overlaid scrollbar too,
		// so the list scrolled with no bar. Clip still scissors the rows.
		Clip: true,
		// Right gutter keeps rows clear of the overlay scrollbar.
		Padding: gui.NewPadding(0, 12, 0, 0),
		Spacing: gui.NoSpacing,
		Content: rows,
	}))
}

func emptyRow(th gui.Theme) gui.View {
	return gui.Text(gui.TextCfg{
		Text:      " ",
		TextStyle: th.TextStyleSecondary,
		Sizing:    gui.FillFit,
	})
}

// remoteRow is one remote file row wrapped in its context menu.
func remoteRow(w *gui.Window, ctrl *app.Controller, tab *app.Tab, s *app.SFTPState, e sftp.FileInfo, idx int) gui.View {
	var onActivate func(gui.EventCtx)
	if e.IsDir {
		onActivate = func(ctx gui.EventCtx) {
			onRowActivate(&s.LastRemoteClick, &s.LastRemoteClickAt, e.Name, func() {
				ctrl.SFTPOpenRemoteDir(ctx.Window, tab.SessID, e.Name)
			})
		}
	}
	row := fileRow(
		w,
		fmt.Sprintf("sftp-r-%d", idx),
		s.RemoteSelected[e.Name],
		e.IsDir,
		e.Name,
		e.Mode,
		formatSize(e.Size),
		formatModTime(e.ModTime),
		func(ctx gui.EventCtx) {
			ctx.Consume()
			s.ToggleRemote(e.Name)
		},
		onActivate,
	)
	cfg := remoteItemMenu(ctrl, tab, s, e)
	// Menu open-state is keyed by ID: every row needs its own or one
	// right-click opens a popup in every row's wrapper at once.
	cfg.ID = fmt.Sprintf("sftp-rmenu-%d", idx)
	cfg.Content = []gui.View{row}
	return gui.ContextMenu(w, cfg)
}

// localRow is one local file row (no context menu, like v1).
func localRow(w *gui.Window, ctrl *app.Controller, tab *app.Tab, s *app.SFTPState, e localfs.Entry, idx int) gui.View {
	return fileRow(
		w,
		fmt.Sprintf("sftp-l-%d", idx),
		s.LocalSelected[e.Path],
		e.IsDir,
		e.Name,
		"",
		formatSize(e.Size),
		formatModTime(e.ModTime),
		func(ctx gui.EventCtx) {
			ctx.Consume()
			s.ToggleLocal(e.Path)
		},
		func(ctx gui.EventCtx) {
			onRowActivate(&s.LastLocalClick, &s.LastLocalClickAt, e.Path, func() {
				if e.IsDir {
					ctrl.SFTPOpenLocalDir(ctx.Window, tab.SessID, e.Path)
				} else {
					ctrl.SFTPOpenLocal(e.Path)
				}
			})
		},
	)
}

// nameBudget approximates the pixel width left for the name column:
// half the window minus the rail, transfer column, fixed cells and
// row chrome. Ellipsis is cosmetic; the cell's Clip is the hard bound.
func nameBudget(w *gui.Window) float32 {
	winW, _ := w.WindowSize()
	paneW := float32(winW)/2 - transferColWidth - 48
	budget := paneW - 300
	if budget < 60 {
		budget = 60
	}
	return budget
}

// fileRow builds the common row chrome: icon+name, size, modified.
// onToggle handles row clicks (selection); onActivate fires on
// double-click.
func fileRow(w *gui.Window, id string, selected, isDir bool, name, mode, size, mod string, onToggle, onActivate func(gui.EventCtx)) gui.View {
	th := gui.CurrentTheme()
	icon := gui.IconFile
	iconColor := th.TextStyleSuccess.Color
	if isDir {
		icon = gui.IconFolderOpen
		iconColor = th.TextStyleWarning.Color
	}
	nameStyle := th.TextStyleBody
	// Icons take semantic colors so they read apart from the name
	// and stay visible on the select-fill row background.
	iconStyle := th.TextStyleIconSmall
	iconStyle.Color = iconColor
	budget := nameBudget(w)
	if mode != "" {
		// The remote mode suffix shares the name row; reserve its
		// width so the ellipsis is not clipped by the cell edge.
		budget -= w.TextWidth(mode, th.TextStyleSecondary) + th.SpacingSmall
	}
	display := layout.Ellipsize(name, budget, func(s string) float32 {
		return w.TextWidth(s, nameStyle)
	})
	return gui.Row(layout.Flush(gui.ContainerCfg{
		ID:      id,
		Sizing:  gui.FillFixed,
		Height:  rowHeight,
		VAlign:  gui.VAlignMiddle,
		Spacing: gui.SpacingTight,
		Padding: gui.NewPadding(0, 6, 0, 6),
		Color:   rowBg(th, selected),
		// Only the left button toggles selection; right-click is the
		// context menu's job (it picks the row to act on).
		OnAnyClick: func(ctx gui.EventCtx) {
			if e := ctx.Event; e != nil && e.MouseButton != gui.MouseLeft {
				return
			}
			onToggle(ctx)
		},
		OnMouseDown:  onActivate,
		Clip:         true,
		ClipContents: true,
		Content: []gui.View{
			gui.Row(layout.Flush(gui.ContainerCfg{
				Sizing:  gui.FillFit,
				VAlign:  gui.VAlignMiddle,
				Spacing: gui.SpacingSmall,
				Clip:    true,
				Content: []gui.View{
					gui.Text(gui.TextCfg{Text: icon, TextStyle: iconStyle, Sizing: gui.FitFit}),
					nameCell(id+"-name", name, display, nameStyle),
					gui.Text(gui.TextCfg{Text: mode, TextStyle: th.TextStyleSecondary, Sizing: gui.FitFit, Clip: true}),
				},
			})),
			fixedCell(size, 90, th),
			fixedCell(mod, 150, th),
		},
	}))
}

// doubleClickWindow is the max gap between two activations that count
// as a double-click.
const doubleClickWindow = 400 * time.Millisecond

// onRowActivate implements double-click detection: two activations of
// the same row within doubleClickWindow fire fn.
func onRowActivate(lastName *string, lastAt *time.Time, name string, fn func()) {
	now := time.Now()
	if *lastName == name && now.Sub(*lastAt) <= doubleClickWindow {
		*lastName = ""
		*lastAt = time.Time{}
		fn()
		return
	}
	*lastName = name
	*lastAt = now
}

func rowBg(th gui.Theme, selected bool) gui.Color {
	if selected {
		return th.ColorSelect
	}
	return gui.Color{}
}

// fixedCell renders a fixed-width right-aligned row cell.
func fixedCell(text string, width float32, th gui.Theme) gui.View {
	style := th.TextStyleSecondary
	style.Align = gui.TextAlignRight
	return gui.Row(layout.Flush(gui.ContainerCfg{
		Sizing:       gui.FixedFixed,
		Width:        width,
		VAlign:       gui.VAlignMiddle,
		Clip:         true,
		ClipContents: true,
		Content:      []gui.View{gui.Text(gui.TextCfg{Text: text, TextStyle: style, Sizing: gui.FillFit, Clip: true})},
	}))
}

// nameCell renders the file name and floats the full name as a
// tooltip on hover. WithTooltip is avoided: its wrapper Column takes
// the theme's default padding and would break the flush row.
// Interactive plus the shared layout.Tip float keeps the cell flush.
func nameCell(id, full, display string, style gui.TextStyle) gui.View {
	return gui.Interactive(id, func(is gui.InteractionState) gui.View {
		content := []gui.View{
			gui.Text(gui.TextCfg{Text: display, TextStyle: style, Sizing: gui.FillFit, Clip: true}),
		}
		if is.Hovered {
			content = append(content, layout.Tip(id+"-tip", full,
				gui.FloatBottomCenter, gui.FloatTopCenter, 0, 4))
		}
		return gui.Row(layout.Flush(gui.ContainerCfg{
			ID:      id,
			Sizing:  gui.FillFit,
			VAlign:  gui.VAlignMiddle,
			Clip:    true,
			Content: content,
		}))
	})
}

// remotePathOf joins the remote dir and entry name.
func remotePathOf(s *app.SFTPState, name string) string {
	if s.RemoteDir == "" || s.RemoteDir == "/" {
		return "/" + name
	}
	return trimSlash(s.RemoteDir) + "/" + name
}

func trimSlash(p string) string {
	for len(p) > 1 && p[len(p)-1] == '/' {
		p = p[:len(p)-1]
	}
	return p
}
