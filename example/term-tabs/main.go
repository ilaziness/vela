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
	panes    []*pane
	active   int
	leftInput string
	leftText string
	rightInput string
	rightText string
}

func tabID(i int) string { return fmt.Sprintf("term-%d", i) }

func main() {
	gui.SetTheme(gui.ThemeDark)

	app := gui.NewApp()
	st := &appState{}

	win := gui.NewWindow(gui.WindowCfg{
		Title:  "go-term tabs repro",
		Width:  1100,
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
			gui.Row(gui.ContainerCfg{
				Sizing:  gui.FillFill,
				Spacing: gui.SpacingPx(4),
				Content: []gui.View{
					leftSidebar(w),
					gui.Column(gui.ContainerCfg{
						Sizing:  gui.FillFill,
						Spacing: gui.SpacingPx(4),
						Content: []gui.View{
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
					}),
					rightSidebar(w),
				},
			}),
		},
	})
}

func leftSidebar(w *gui.Window) gui.View {
	st := gui.State[appState](w)
	th := gui.CurrentTheme()
	return gui.Column(gui.ContainerCfg{
		Sizing:  gui.FitFill,
		Spacing: gui.SpacingPx(4),
		Content: []gui.View{
			gui.Label("Left Sidebar", th.TextStyleBody.Bold()),
			gui.Label("Input:", th.TextStyleBody),
			gui.Input(gui.InputCfg{
				ID:          "left-input",
				Text:        st.leftInput,
				Placeholder: "Type here...",
				Sizing:      gui.FillFit,
				OnTextChanged: func(s string, ctx gui.EventCtx) {
					ctx.Consume()
					st := gui.State[appState](ctx.Window)
					st.leftInput = s
				},
			}),
			gui.Label("Textarea:", th.TextStyleBody),
			gui.Input(gui.InputCfg{
				ID:          "left-textarea",
				Text:        st.leftText,
				Placeholder: "Type multiline text...",
				Mode:        gui.InputMultiline,
				Sizing:      gui.FillFill,
				OnTextChanged: func(s string, ctx gui.EventCtx) {
					ctx.Consume()
					st := gui.State[appState](ctx.Window)
					st.leftText = s
				},
			}),
			gui.Label("Sample Buttons:", th.TextStyleBody),
			gui.Button(gui.ButtonCfg{
				ID:    "left-btn-1",
				Label: "Button 1",
				OnClick: func(ctx gui.EventCtx) {
					ctx.Consume()
					st := gui.State[appState](ctx.Window)
					st.leftInput = "Clicked Button 1"
				},
			}),
			gui.Button(gui.ButtonCfg{
				ID:    "left-btn-2",
				Label: "Button 2",
				OnClick: func(ctx gui.EventCtx) {
					ctx.Consume()
					st := gui.State[appState](ctx.Window)
					st.leftText = "Clicked Button 2"
				},
			}),
			gui.Button(gui.ButtonCfg{
				ID:    "left-btn-3",
				Label: "Clear",
				OnClick: func(ctx gui.EventCtx) {
					ctx.Consume()
					st := gui.State[appState](ctx.Window)
					st.leftInput = ""
					st.leftText = ""
				},
			}),
		},
	})
}

func rightSidebar(w *gui.Window) gui.View {
	st := gui.State[appState](w)
	th := gui.CurrentTheme()
	return gui.Column(gui.ContainerCfg{
		Sizing:  gui.FitFill,
		Spacing: gui.SpacingPx(4),
		Content: []gui.View{
			gui.Label("Right Sidebar", th.TextStyleBody.Bold()),
			gui.Label("Input:", th.TextStyleBody),
			gui.Input(gui.InputCfg{
				ID:          "right-input",
				Text:        st.rightInput,
				Placeholder: "Type here...",
				Sizing:      gui.FillFit,
				OnTextChanged: func(s string, ctx gui.EventCtx) {
					ctx.Consume()
					st := gui.State[appState](ctx.Window)
					st.rightInput = s
				},
			}),
			gui.Label("Textarea:", th.TextStyleBody),
			gui.Input(gui.InputCfg{
				ID:          "right-textarea",
				Text:        st.rightText,
				Placeholder: "Type multiline text...",
				Mode:        gui.InputMultiline,
				Sizing:      gui.FillFill,
				OnTextChanged: func(s string, ctx gui.EventCtx) {
					ctx.Consume()
					st := gui.State[appState](ctx.Window)
					st.rightText = s
				},
			}),
			gui.Label("Sample Buttons:", th.TextStyleBody),
			gui.Button(gui.ButtonCfg{
				ID:    "right-btn-1",
				Label: "Button 1",
				OnClick: func(ctx gui.EventCtx) {
					ctx.Consume()
					st := gui.State[appState](ctx.Window)
					st.rightInput = "Clicked Button 1"
				},
			}),
			gui.Button(gui.ButtonCfg{
				ID:    "right-btn-2",
				Label: "Button 2",
				OnClick: func(ctx gui.EventCtx) {
					ctx.Consume()
					st := gui.State[appState](ctx.Window)
					st.rightText = "Clicked Button 2"
				},
			}),
			gui.Button(gui.ButtonCfg{
				ID:    "right-btn-3",
				Label: "Clear",
				OnClick: func(ctx gui.EventCtx) {
					ctx.Consume()
					st := gui.State[appState](ctx.Window)
					st.rightInput = ""
					st.rightText = ""
				},
			}),
		},
	})
}
