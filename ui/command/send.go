package commandui

import (
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/go-gui-org/go-gui/gui"

	"github.com/ilaziness/vela/internal/i18n"
	"github.com/ilaziness/vela/ui/layout"
)

// sendPanel is the right column: command history, target sessions, and
// the command input with its send action.
func sendPanel(w *gui.Window, d *Deps, st *State) gui.View {
	th := gui.CurrentTheme()

	return gui.Column(layout.Flush(gui.ContainerCfg{
		ID:      "cmd-send",
		Sizing:  gui.FillFill,
		Color:   th.ColorBackground,
		Padding: gui.PaddingSmall,
		Spacing: gui.SpacingSmall,
		Content: []gui.View{
			historySection(w, d, st),
			gui.Row(layout.Flush(gui.ContainerCfg{
				Sizing: gui.FillFixed,
				Height: 1,
				Color:  th.ColorBorder,
			})),
			sessionSection(w, d, st),
			inputSection(d, st),
		},
	}))
}

// historySection lists recently sent commands; clicking one refills the
// input. Clearing asks for confirmation first.
func historySection(w *gui.Window, d *Deps, st *State) gui.View {
	th := gui.CurrentTheme()
	hist := st.hist

	// Command text gets whatever the row leaves after the timestamp;
	// ellipsize against the measured time width so it never overflows.
	winW, _ := w.WindowSize()
	availW := float32(winW) - libraryWidth - 96
	rows := make([]gui.View, 0, len(hist))
	for i, h := range hist {
		cmd := h.Command
		when := time.UnixMilli(h.Timestamp).Format("2006-01-02 15:04")
		timeW := w.TextWidth(when, th.TextStyleSecondary)
		budget := availW - timeW
		label := layout.Ellipsize(cmd, budget, func(s string) float32 {
			return w.TextWidth(s, th.TextStyleBody)
		})
		rows = append(rows, gui.Row(layout.Flush(gui.ContainerCfg{
			ID:      "cmd-hist-" + strconv.Itoa(i),
			Sizing:  gui.FillFit,
			Spacing: gui.SpacingTight,
			HAlign:  gui.HAlignStart,
			OnClick: func(ctx gui.EventCtx) {
				ctx.Consume()
				gui.State[State](ctx.Window).input = cmd
			},
			Content: []gui.View{
				gui.Button(gui.ButtonCfg{
					ID:            "cmd-hist-del-" + strconv.Itoa(i),
					Variant:       gui.ButtonGhost,
					Sizing:        gui.FitFit,
					FocusDisabled: true,
					Content: []gui.View{
						gui.Text(gui.TextCfg{Text: gui.IconTrash, TextStyle: th.TextStyleIconSmall}),
					},
					OnClick: func(ctx gui.EventCtx) {
						ctx.Consume()
						s := gui.State[State](ctx.Window)
						if err := d.Commands.DeleteHistory(cmd); err != nil {
							toastErr(ctx.Window, i18n.Tf("command.delete_failed", errText(err)))
							return
						}
						s.reload(d)
					},
				}),
				gui.Text(gui.TextCfg{
					Text:      label,
					TextStyle: th.TextStyleBody,
					Sizing:    gui.FillFit,
				}),
				gui.Text(gui.TextCfg{
					Text:      when,
					TextStyle: th.TextStyleSecondary,
					Sizing:    gui.FitFit,
				}),
			},
		})))
	}
	if len(rows) == 0 {
		rows = append(rows, gui.Text(gui.TextCfg{
			Text:      i18n.T("common.none"),
			TextStyle: th.TextStyleSecondary,
		}))
	}

	return gui.Column(layout.Flush(gui.ContainerCfg{
		ID:      "cmd-history",
		Sizing:  gui.FillFill,
		Color:   th.ColorPanel,
		Padding: gui.PaddingSmall,
		Spacing: gui.SpacingTight,
		Content: []gui.View{
			gui.Row(layout.Flush(gui.ContainerCfg{
				Sizing:  gui.FillFit,
				Spacing: gui.SpacingSmall,
				Content: []gui.View{
					gui.Text(gui.TextCfg{Text: i18n.T("command.history"), TextStyle: th.TextStyleBody, Sizing: gui.FillFit}),
					gui.Button(gui.ButtonCfg{
						ID:      "cmd-hist-clear",
						Variant: gui.ButtonGhost,
						Label:   i18n.T("command.clear_history"),
						OnClick: func(ctx gui.EventCtx) {
							ctx.Consume()
							confirmClearHistory(ctx.Window, d)
						},
					}),
				},
			})),
			// The list owns the scroll so the header stays pinned; the
			// right padding reserves the overlay scrollbar's footprint
			// so timestamps are not covered.
			gui.Column(layout.Flush(gui.ContainerCfg{
				ID:         "cmd-history-list",
				Sizing:     gui.FillFill,
				Padding:    gui.NewPadding(0, th.ScrollbarStyle.GapEdge+th.ScrollbarStyle.Size, 0, 0),
				Scrollable: true,
				Content:    rows,
			})),
		},
	}))
}

// sessionSection lists live terminal sessions with checkboxes; checked
// sessions receive the sent command.
func sessionSection(w *gui.Window, d *Deps, st *State) gui.View {
	th := gui.CurrentTheme()
	infos := d.Hub.Active()
	// Hub.Active() walks a map, so order is random frame to frame;
	// sort by display label for a stable list (no flicker).
	sort.Slice(infos, func(i, j int) bool {
		li, lj := infos[i].Label, infos[j].Label
		if li == "" {
			li = infos[i].ID
		}
		if lj == "" {
			lj = infos[j].ID
		}
		return strings.ToLower(li) < strings.ToLower(lj)
	})
	rows := make([]gui.View, 0, len(infos))
	for _, info := range infos {
		id := info.ID
		label := info.Label
		if label == "" {
			label = id
		}
		rows = append(rows, gui.Toggle(gui.ToggleCfg{
			ID:       "cmd-sess-" + sanitizeID(id),
			Label:    label + " (" + string(info.Protocol) + ")",
			Selected: st.selectedSess[id],
			OnClick: func(ctx gui.EventCtx) {
				ctx.Consume()
				s := gui.State[State](ctx.Window)
				if s.selectedSess[id] {
					delete(s.selectedSess, id)
				} else {
					s.selectedSess[id] = true
				}
			},
		}))
	}
	if len(rows) == 0 {
		rows = append(rows, gui.Text(gui.TextCfg{
			Text:      i18n.T("common.none"),
			TextStyle: th.TextStyleSecondary,
		}))
	}

	// Cap the session list at ~40% of the window so a long list scrolls
	// internally instead of squeezing the history pane.
	_, winH := w.WindowSize()
	maxSessH := float32(winH) * 0.4
	return gui.Column(layout.Flush(gui.ContainerCfg{
		ID:         "cmd-sessions",
		Sizing:     gui.FillFit,
		MaxHeight:  maxSessH,
		Color:      th.ColorPanel,
		Padding:    gui.PaddingSmall,
		Spacing:    gui.SpacingTight,
		Scrollable: true,
		Content: []gui.View{
			gui.Row(layout.Flush(gui.ContainerCfg{
				Sizing:  gui.FillFit,
				Spacing: gui.SpacingSmall,
				Content: []gui.View{
					gui.Text(gui.TextCfg{Text: i18n.T("command.select_session_label"), TextStyle: th.TextStyleLabel}),
				},
			})),
			gui.Column(layout.Flush(gui.ContainerCfg{
				Sizing:  gui.FillFit,
				Spacing: gui.SpacingSmall,
				Padding: gui.NewPadding(0, th.ScrollbarStyle.GapEdge+th.ScrollbarStyle.Size, 0, 0),
				Content: rows,
			})),
		},
	}))
}

// inputSection is the command composer: input plus send.
func inputSection(d *Deps, st *State) gui.View {
	th := gui.CurrentTheme()

	input := gui.Input(gui.InputCfg{
		ID: "cmd-input",
		// Multiline like the AI composer: it grows with the draft and
		// scrolls past the cap. Enter inserts a newline (multiline
		// fields own that key), so sending is the button's job.
		Mode:        gui.InputMultiline,
		Text:        st.input,
		Placeholder: i18n.T("command.input_placeholder"),
		Sizing:      gui.FillFit,
		MinHeight:   44,
		MaxHeight:   140,
		Scrollable:  true,
		OnTextChanged: func(v string, ctx gui.EventCtx) {
			gui.State[State](ctx.Window).input = v
			ctx.Consume()
		},
	})

	return gui.Column(layout.Flush(gui.ContainerCfg{
		ID:      "cmd-input-area",
		Sizing:  gui.FillFit,
		Spacing: gui.SpacingTight,
		Content: []gui.View{
			gui.Text(gui.TextCfg{
				Text:      i18n.T("command.input_helper"),
				TextStyle: th.TextStyleSecondary,
			}),
			input,
			gui.Row(layout.Flush(gui.ContainerCfg{
				Sizing:  gui.FillFit,
				HAlign:  gui.HAlignEnd,
				Spacing: gui.SpacingSmall,
				Content: []gui.View{
					gui.Button(gui.ButtonCfg{
						ID:       "cmd-send-btn",
						Variant:  gui.ButtonPrimary,
						Label:    i18n.T("command.send"),
						Sizing:   gui.FitFit,
						Disabled: len(st.selectedSess) == 0,
						OnClick: func(ctx gui.EventCtx) {
							ctx.Consume()
							sendCommand(ctx.Window, d)
						},
					}),
				},
			})),
		},
	}))
}

// sendCommand delivers the drafted command to every selected session.
func sendCommand(w *gui.Window, d *Deps) {
	st := gui.State[State](w)
	cmd := strings.TrimSpace(st.input)
	if cmd == "" {
		toastErr(w, i18n.T("command.empty"))
		return
	}
	ids := st.selectedIDs()
	if len(ids) == 0 {
		toastErr(w, i18n.T("command.select_session"))
		return
	}
	if err := d.Commands.Send(cmd, ids); err != nil {
		toastErr(w, i18n.Tf("command.send_failed", errText(err)))
		return
	}
	st.input = ""
	st.reload(d)
}

// confirmClearHistory asks before wiping the command history table.
func confirmClearHistory(w *gui.Window, d *Deps) {
	w.Dialog(gui.DialogCfg{
		DialogType:    gui.DialogConfirm,
		Title:         i18n.T("command.confirm_clear_title"),
		Body:          i18n.T("command.confirm_clear_message"),
		DefaultButton: gui.DialogButtonNo,
		OnOkYes: func(win *gui.Window) {
			if err := d.Commands.ClearHistory(); err != nil {
				toastErr(win, i18n.Tf("command.clear_history_failed", errText(err)))
				return
			}
			gui.State[State](win).reload(d)
			toastOK(win, i18n.T("command.clear_history_success"))
		},
	})
}

// sanitizeID keeps session IDs safe inside element IDs (they may contain
// '@', ':', '|').
func sanitizeID(s string) string {
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_' {
			out = append(out, c)
		} else {
			out = append(out, '_')
		}
	}
	return string(out)
}
