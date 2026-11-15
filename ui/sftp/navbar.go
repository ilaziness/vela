package sftpui

import (
	"github.com/go-gui-org/go-gui/gui"

	"github.com/ilaziness/vela/internal/i18n"
	"github.com/ilaziness/vela/ui/app"
	"github.com/ilaziness/vela/ui/layout"
)

const (
	navBarHeight   float32 = 30
	paneHeadHeight float32 = 24
	rowHeight      float32 = 24
	toolBtn        float32 = 24
)

// paneView builds one file pane (local or remote).
func paneView(w *gui.Window, ctrl *app.Controller, tab *app.Tab, s *app.SFTPState, remote bool) gui.View {
	th := gui.CurrentTheme()
	sessID := tab.SessID

	var list gui.View
	var head gui.View
	var nav gui.View
	var onDrop func(gui.EventCtx)
	if remote {
		onDrop = func(ctx gui.EventCtx) {
			ctx.Consume()
			if p := ctx.Event.FilePath; p != "" {
				ctrl.SFTPUploadLocalPaths(ctx.Window, sessID, []string{p})
			}
		}
		nav = remoteNavBar(ctrl, sessID, s)
		head = fileHeader(true)
		list = remoteFileList(w, ctrl, tab, s)
	} else {
		nav = localNavBar(ctrl, sessID, s)
		head = fileHeader(false)
		list = localFileList(w, ctrl, tab, s)
	}

	return gui.Column(layout.Flush(gui.ContainerCfg{
		ID:         paneID(remote),
		Sizing:     gui.FillFill,
		Color:      th.ColorPanel,
		Spacing:    gui.NoSpacing,
		OnFileDrop: onDrop,
		Content:    []gui.View{nav, head, list, widthFloor(w)},
	}))
}

// widthFloor pins the pane's measured width to a value far wider than
// any content. go-gui's Fill sizing starts from each child's measured
// (content) width and only water-fills the remainder, so two panes
// with different content widths drift apart once space runs short.
// Equal measured widths keep the two Fill panes in lockstep at every
// window size. The sizer is zero-height and invisible; its Width seeds
// Shape.Width while leaving MinWidth at ~0, so it never constrains
// the arranged size.
func widthFloor(w *gui.Window) gui.View {
	winW, _ := w.WindowSize()
	return gui.Column(layout.Flush(gui.ContainerCfg{
		Sizing: gui.FitFit,
		Width:  float32(winW) + 2048,
		Height: 0,
	}))
}

func paneID(remote bool) string {
	if remote {
		return "sftp-pane-remote"
	}
	return "sftp-pane-local"
}

// remoteNavBar is the remote pane's path/navigation row.
func remoteNavBar(ctrl *app.Controller, sessID string, s *app.SFTPState) gui.View {
	th := gui.CurrentTheme()
	atRoot := s.RemoteDir == "/" || s.RemoteDir == ""
	return gui.Row(layout.Flush(gui.ContainerCfg{
		Sizing:  gui.FillFixed,
		Height:  navBarHeight,
		VAlign:  gui.VAlignMiddle,
		Spacing: gui.SpacingTight,
		Padding: gui.NewPadding(0, 4, 0, 4),
		Content: []gui.View{
			iconBtn("sftp-remote-up", gui.IconArrowUp, i18n.T("sftp.go_up"), th, atRoot, func(ctx gui.EventCtx) {
				ctx.Consume()
				ctrl.SFTPGoUpRemote(ctx.Window, sessID)
			}),
			gui.Input(gui.InputCfg{
				ID:          "sftp-remote-path",
				Text:        s.RemoteInput,
				Placeholder: "/",
				Sizing:      gui.FillFit,
				OnTextChanged: func(v string, ctx gui.EventCtx) {
					s.RemoteInput = v
				},
				// Enter fires both OnTextCommit(Enter) and OnEnter;
				// handling the commit reason once avoids a double
				// navigation. Blur commits are ignored.
				OnTextCommit: func(v string, reason gui.InputCommitReason, ctx gui.EventCtx) {
					if reason == gui.InputCommitEnter {
						ctrl.SFTPGoToRemote(ctx.Window, sessID, v)
					}
				},
			}),
			iconBtn("sftp-remote-go", gui.IconArrowRight, i18n.T("sftp.navigate_to_path"), th, false, func(ctx gui.EventCtx) {
				ctx.Consume()
				ctrl.SFTPGoToRemote(ctx.Window, sessID, s.RemoteInput)
			}),
			iconBtn("sftp-remote-refresh", gui.IconSync, i18n.T("common.refresh"), th, false, func(ctx gui.EventCtx) {
				ctx.Consume()
				ctrl.SFTPRefreshRemote(ctx.Window, sessID)
			}),
			iconBtn("sftp-remote-hidden", gui.IconEye, hiddenTip(s.RemoteShowHidden), th, s.RemoteShowHidden, func(ctx gui.EventCtx) {
				ctx.Consume()
				ctrl.SFTPToggleHiddenRemote(ctx.Window, sessID)
			}),
		},
	}))
}

// localNavBar is the local pane's path/navigation row.
func localNavBar(ctrl *app.Controller, sessID string, s *app.SFTPState) gui.View {
	th := gui.CurrentTheme()
	return gui.Row(layout.Flush(gui.ContainerCfg{
		Sizing:  gui.FillFixed,
		Height:  navBarHeight,
		VAlign:  gui.VAlignMiddle,
		Spacing: gui.SpacingTight,
		Padding: gui.NewPadding(0, 4, 0, 4),
		Content: []gui.View{
			iconBtn("sftp-local-up", gui.IconArrowUp, i18n.T("sftp.go_up"), th, s.LocalAtRoot, func(ctx gui.EventCtx) {
				ctx.Consume()
				ctrl.SFTPGoUpLocal(ctx.Window, sessID)
			}),
			gui.Input(gui.InputCfg{
				ID:          "sftp-local-path",
				Text:        s.LocalInput,
				Placeholder: "~/",
				Sizing:      gui.FillFit,
				OnTextChanged: func(v string, ctx gui.EventCtx) {
					s.LocalInput = v
				},
				OnTextCommit: func(v string, reason gui.InputCommitReason, ctx gui.EventCtx) {
					if reason == gui.InputCommitEnter {
						ctrl.SFTPGoToLocal(ctx.Window, sessID, v)
					}
				},
			}),
			iconBtn("sftp-local-go", gui.IconArrowRight, i18n.T("sftp.navigate_to_path"), th, false, func(ctx gui.EventCtx) {
				ctx.Consume()
				ctrl.SFTPGoToLocal(ctx.Window, sessID, s.LocalInput)
			}),
			iconBtn("sftp-local-refresh", gui.IconSync, i18n.T("common.refresh"), th, false, func(ctx gui.EventCtx) {
				ctx.Consume()
				ctrl.SFTPRefreshLocal(ctx.Window, sessID)
			}),
			iconBtn("sftp-local-hidden", gui.IconEye, hiddenTip(s.LocalShowHidden), th, s.LocalShowHidden, func(ctx gui.EventCtx) {
				ctx.Consume()
				ctrl.SFTPToggleHiddenLocal(ctx.Window, sessID)
			}),
		},
	}))
}

// hiddenTip picks the dotfile toggle tooltip.
func hiddenTip(show bool) string {
	if show {
		return i18n.T("sftp.hide_dotfiles")
	}
	return i18n.T("sftp.show_dotfiles")
}

// iconBtn is a small square icon button with a hover tip.
func iconBtn(id, glyph, tip string, th gui.Theme, active bool, onClick func(gui.EventCtx)) gui.View {
	return tipBtn(id, glyph, tip, th, toolBtn, gui.ButtonGhost, active, false, onClick)
}

// tipBtn is a fixed-size icon button with a hover tip, built on
// gui.Interactive like session.toolBtn. WithTooltip wraps its content
// in a default-padded column, which breaks the flush bars' vertical
// alignment, so the tip floats manually on hover instead.
func tipBtn(id, glyph, tip string, th gui.Theme, size float32, variant gui.ButtonVariant, active, disabled bool, onClick func(gui.EventCtx)) gui.View {
	cellID := "tipbtn-" + id
	return gui.Interactive(cellID, func(is gui.InteractionState) gui.View {
		bg := gui.Color{}
		if active {
			bg = th.ColorActive
		}
		iconStyle := th.TextStyleIconSmall
		if size > 26 {
			iconStyle = th.TextStyleIconMedium
		}
		content := []gui.View{
			gui.Button(gui.ButtonCfg{
				ID:            cellID + "-hit",
				Variant:       variant,
				Sizing:        gui.FillFill,
				Padding:       gui.PaddingNone,
				Color:         bg,
				Radius:        gui.RadiusSmall,
				Disabled:      disabled,
				FocusDisabled: true,
				HAlign:        gui.Some(gui.HAlignCenter),
				VAlign:        gui.Some(gui.VAlignMiddle),
				Content: []gui.View{
					gui.Text(gui.TextCfg{Text: glyph, TextStyle: iconStyle}),
				},
				OnClick: onClick,
			}),
		}
		if is.Hovered && tip != "" {
			content = append(content, TipBubble(tip, th))
		}
		return gui.Column(layout.Flush(gui.ContainerCfg{
			ID:      cellID,
			Sizing:  gui.FixedFixed,
			Width:   size,
			Height:  size,
			Radius:  gui.RadiusSmall,
			HAlign:  gui.HAlignCenter,
			VAlign:  gui.VAlignMiddle,
			Content: content,
		}))
	})
}

// TipBubble is the floating label shown while a tipBtn or file name
// is hovered.
func TipBubble(text string, th gui.Theme) gui.View {
	return gui.Column(layout.Flush(gui.ContainerCfg{
		Float:         true,
		FloatAutoFlip: true,
		FloatAnchor:   gui.FloatBottomCenter,
		FloatTieOff:   gui.FloatTopCenter,
		FloatOffsetY:  4,
		FloatZIndex:   10,
		Color:         th.ColorInterior,
		ColorBorder:   th.ColorBorder,
		SizeBorder:    gui.BorderThin,
		Radius:        gui.RadiusSmall,
		Padding:       gui.PaddingXSmall,
		Content: []gui.View{
			gui.Text(gui.TextCfg{Text: text, TextStyle: th.TextStyleSecondary}),
		},
	}))
}
