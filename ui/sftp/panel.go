// Package sftpui renders the native SFTP file panel that overlays the
// terminal stage of an SSH session tab. It is a pure view layer: all
// backend work goes through *app.Controller methods.
package sftpui

import (
	"github.com/go-gui-org/go-gui/gui"

	"github.com/ilaziness/vela/internal/i18n"
	"github.com/ilaziness/vela/ui/app"
	"github.com/ilaziness/vela/ui/layout"
)

// View builds the full-panel overlay. It must be placed as the top
// layer of a gui.Stack covering only the terminal stage.
func View(w *gui.Window, ctrl *app.Controller, tab *app.Tab) gui.View {
	sessID := tab.SessID
	s := ctrl.SFTPStateFor(w, sessID)
	th := gui.CurrentTheme()

	var body gui.View
	switch {
	case s == nil:
		body = gui.Label(i18n.T("common.loading"), th.TextStyleSecondary)
	case s.Status == app.SFTPLoading && len(s.RemoteEntries) == 0 && len(s.LocalEntries) == 0:
		body = gui.Label(i18n.T("common.loading"), th.TextStyleSecondary)
	case s.Status == app.SFTPError && len(s.RemoteEntries) == 0:
		body = gui.Column(gui.ContainerCfg{
			Sizing:  gui.FillFill,
			VAlign:  gui.VAlignMiddle,
			HAlign:  gui.HAlignCenter,
			Spacing: gui.SpacingMedium,
			Content: []gui.View{
				gui.Text(gui.TextCfg{Text: s.Err, TextStyle: th.TextStyleError}),
				gui.Button(gui.ButtonCfg{
					ID:      "sftp-retry",
					Label:   i18n.T("common.retry"),
					Variant: gui.ButtonSecondary,
					OnClick: func(ctx gui.EventCtx) {
						ctx.Consume()
						s.Status = app.SFTPLoading
						s.Err = ""
						ctrl.OpenSFTP(ctx.Window, tab.ID)
					},
				}),
			},
		})
	default:
		body = panesRow(w, ctrl, tab, s)
	}

	return gui.Column(layout.Flush(gui.ContainerCfg{
		// Focusable so OpenSFTP can park keyboard focus here (Escape
		// closes the panel); app.SFTPPanelID is the contract.
		ID:           app.SFTPPanelID,
		Focusable:    true,
		Sizing:       gui.FillFill,
		Color:        th.ColorBackground,
		Padding:      gui.PaddingSmall,
		Clip:         true,
		ClipContents: true,
		OnAnyClick:   swallowUnhandled,
		OnMouseDown:  swallowUnhandled,
		OnMouseUp:    swallowUnhandled,
		OnMouseMove:  swallowUnhandled,
		OnChar:       swallowUnhandled,
		OnKeyDown:    func(ctx gui.EventCtx) { panelKey(ctx, ctrl, tab) },
		OnKeyUp:      swallowUnhandled,
		Content: []gui.View{
			body,
		},
	}))
}

// swallowUnhandled eats events no child claimed so nothing leaks
// toward the terminal layer or shell chrome.
func swallowUnhandled(ctx gui.EventCtx) {
	if !ctx.Handled() {
		ctx.Consume()
	}
}

// panelKey closes the panel on Escape when no dialog is open.
func panelKey(ctx gui.EventCtx, ctrl *app.Controller, tab *app.Tab) {
	e := ctx.Event
	if e != nil && e.Type == gui.EventKeyDown && e.KeyCode == gui.KeyEscape && !ctx.Window.DialogIsVisible() {
		ctrl.CloseSFTP(ctx.Window, tab.ID)
		ctx.Consume()
		return
	}
	swallowUnhandled(ctx)
}

// panesRow lays out local | transfer column | remote, with a thin
// separator on each side of the transfer column.
func panesRow(w *gui.Window, ctrl *app.Controller, tab *app.Tab, s *app.SFTPState) gui.View {
	th := gui.CurrentTheme()
	return gui.Row(layout.Flush(gui.ContainerCfg{
		ID:      "sftp-panes",
		Sizing:  gui.FillFill,
		Spacing: gui.NoSpacing,
		Content: []gui.View{
			paneView(w, ctrl, tab, s, false),
			vDivider(th),
			transferColumn(ctrl, tab, s),
			vDivider(th),
			paneView(w, ctrl, tab, s, true),
		},
	}))
}

// vDivider is the 1px separator between a file pane and the transfer
// column. ColorInterior reads clearly against the panel surface,
// unlike the faint default border color.
func vDivider(th gui.Theme) gui.View {
	return gui.Column(layout.Flush(gui.ContainerCfg{
		Sizing: gui.FixedFill,
		Width:  1,
		Color:  th.ColorInterior,
	}))
}
