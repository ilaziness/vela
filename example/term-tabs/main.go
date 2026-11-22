// Minimal repro: multiple go-term terminal tabs inside a go-gui window,
// using only the published upstream APIs (no local patches, no Stream —
// each tab is a real local PTY shell via term.New).
package main

import (
	"fmt"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend"
	"github.com/go-gui-org/go-term/term"
)

type pane struct {
	tm    *term.Term
	title string
}

type appState struct {
	panes  []*pane
	active int
}

func tabID(i int) string { return fmt.Sprintf("term-%d", i) }

func main() {
	gui.SetTheme(gui.ThemeDark)

	app := gui.NewApp()
	st := &appState{}

	win := gui.NewWindow(gui.WindowCfg{
		Title:  "go-term tabs repro",
		Width:  900,
		Height: 600,
		State:  st,
		OnInit: func(w *gui.Window) {
			// Route unhandled window events (focus reports, momentum
			// cancel, drag safety) to the active terminal only.
			w.OnEvent = func(e *gui.Event, win *gui.Window) {
				if p := activePane(gui.State[appState](win)); p != nil {
					p.tm.HandleWindowEvent(e)
				}
			}
			addPane(w, st)
			w.SetView(mainView)
		},
		OnCloseRequest: func(w *gui.Window) {
			for _, p := range gui.State[appState](w).panes {
				_ = p.tm.Close()
			}
			w.Close()
		},
	})

	backend.RunApp(app, win)
}

func activePane(st *appState) *pane {
	if st.active >= 0 && st.active < len(st.panes) {
		return st.panes[st.active]
	}
	return nil
}

// addPane spawns a new local-shell Term and makes it the active tab.
func addPane(w *gui.Window, st *appState) {
	p := &pane{title: "shell"}
	idx := len(st.panes)
	tm, err := term.New(w, term.Cfg{
		// We dispatch window events ourselves in w.OnEvent.
		NoWindowHandler: true,
		OnTitle: func(title string) {
			w.QueueCommand(func(w *gui.Window) {
				p.title = title
			})
		},
		OnExit: func() {
			// Reader goroutine: hop to the main thread to mutate state.
			w.QueueCommand(func(w *gui.Window) {
				removePane(gui.State[appState](w), p)
			})
		},
	})
	if err != nil {
		return
	}
	p.tm = tm
	st.panes = append(st.panes, p)
	st.active = idx
	focusActive(w, st)
}

func removePane(st *appState, p *pane) {
	for i, q := range st.panes {
		if q == p {
			st.panes = append(st.panes[:i], st.panes[i+1:]...)
			if st.active >= len(st.panes) {
				st.active = len(st.panes) - 1
			}
			break
		}
	}
}

// focusActive mirrors the embedder contract: only the visible pane is
// flagged focused, and go-gui focus is re-asserted on its focus ID.
func focusActive(w *gui.Window, st *appState) {
	for i, p := range st.panes {
		p.tm.SetFocused(i == st.active)
	}
	if p := activePane(st); p != nil {
		if ids := w.ResolveID(p.tm.FocusID()); len(ids) > 0 {
			w.SetFocus(ids[0])
		}
	}
}

func mainView(w *gui.Window) gui.View {
	st := gui.State[appState](w)

	items := make([]gui.TabItemCfg, 0, len(st.panes))
	for i, p := range st.panes {
		tm := p.tm
		title := p.title
		if title == "" {
			title = fmt.Sprintf("Term %d", i+1)
		}
		items = append(items, gui.NewTabItem(tabID(i), title,
			[]gui.View{gui.ViewFunc(func(vw *gui.Window) gui.View {
				return tm.View(vw)
			})}))
	}

	selected := ""
	if p := activePane(st); p != nil {
		selected = tabID(st.active)
	}

	return gui.Column(gui.ContainerCfg{
		Sizing:  gui.FillFill,
		Spacing: gui.SpacingPx(4),
		Content: []gui.View{
			gui.Row(gui.ContainerCfg{
				Sizing:  gui.FillFit,
				Spacing: gui.SpacingPx(4),
				Content: []gui.View{
					gui.Button(gui.ButtonCfg{
						ID:    "new-tab",
						Label: "+ New tab",
						OnClick: func(ctx gui.EventCtx) {
							ctx.Consume()
							addPane(ctx.Window, gui.State[appState](ctx.Window))
						},
					}),
					gui.Button(gui.ButtonCfg{
						ID:    "close-tab",
						Label: "Close tab",
						OnClick: func(ctx gui.EventCtx) {
							ctx.Consume()
							s := gui.State[appState](ctx.Window)
							if p := activePane(s); p != nil {
								removePane(s, p)
								_ = p.tm.Close()
								focusActive(ctx.Window, s)
							}
						},
					}),
				},
			}),
			gui.TabControl(gui.TabControlCfg{
				ID:       "tabs",
				Sizing:   gui.FillFill,
				Selected: selected,
				Items:    items,
				OnSelect: func(id string, ctx gui.EventCtx) {
					ctx.Consume()
					s := gui.State[appState](ctx.Window)
					var idx int
					if _, err := fmt.Sscanf(id, "term-%d", &idx); err == nil {
						s.active = idx
						focusActive(ctx.Window, s)
					}
				},
			}),
		},
	})
}
