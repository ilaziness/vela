package session

import (
	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-term/term"

	"github.com/ilaziness/vela/internal/i18n"
	"github.com/ilaziness/vela/ui/app"
	"github.com/ilaziness/vela/ui/layout"
)

// TerminalPane embeds the go-term view flush in the main pane, or the
// reconnect pane when the session behind the tab has dropped.
func TerminalPane(w *gui.Window, ctrl *app.Controller, tab *app.Tab) gui.View {
	if tab == nil || tab.Kind != app.TabSession {
		return gui.Label(i18n.T("shell.session.none"), gui.CurrentTheme().TextStyleBody)
	}
	if tab.SessID == "" {
		// The session ended but the tab was kept: offer a reconnect.
		return ReconnectPane(w, ctrl, tab)
	}
	tm := ctrl.TermFor(tab.SessID)
	if tm == nil {
		// SessID without a term is a broken invariant, not a disconnect.
		return gui.Label(i18n.T("shell.session.term_not_ready"), gui.CurrentTheme().TextStyleBody)
	}
	body := gui.Column(layout.Flush(gui.ContainerCfg{
		Sizing:  gui.FillFill,
		Content: []gui.View{tm.View(w)},
	}))
	return terminalMenu(w, ctrl, tab, tm, body)
}

// terminalMenu wraps the terminal view in a right-click context menu.
// Copy/Paste/Find dispatch through Term.RunAction so they honor the same
// conditional-passthrough rules as the keyboard bindings; the record item
// toggles a .gtr session recording via the controller.
func terminalMenu(w *gui.Window, ctrl *app.Controller, tab *app.Tab, tm *term.Term, body gui.View) gui.View {
	sessID := tab.SessID
	recordText := i18n.T("terminal.start_logging")
	if tm.Recording() {
		recordText = i18n.T("terminal.stop_logging")
	}
	return gui.ContextMenu(w, gui.ContextMenuCfg{
		ID:      "term-ctx-" + sessID,
		Sizing:  gui.FillFill,
		Padding: gui.PaddingNone,
		Items: []gui.MenuItemCfg{
			{ID: "copy", Text: i18n.T("common.copy"), Action: func(_ *gui.MenuItemCfg, ctx gui.EventCtx) {
				tm.RunAction(term.ActionCopy, ctx.Window)
			}},
			{ID: "paste", Text: i18n.T("terminal.paste"), Action: func(_ *gui.MenuItemCfg, ctx gui.EventCtx) {
				tm.RunAction(term.ActionPaste, ctx.Window)
			}},
			{ID: "find", Text: i18n.T("terminal.find"), Action: func(_ *gui.MenuItemCfg, ctx gui.EventCtx) {
				tm.RunAction(term.ActionFind, ctx.Window)
			}},
			gui.MenuSeparator(),
			{ID: "record", Text: recordText, Action: func(_ *gui.MenuItemCfg, ctx gui.EventCtx) {
				ctrl.ToggleSessionRecording(ctx.Window, sessID)
			}},
		},
		Content: []gui.View{body},
	})
}
