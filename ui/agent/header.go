package agent

import (
	"time"

	"github.com/go-gui-org/go-gui/gui"

	"github.com/ilaziness/vela/internal/agent"
	"github.com/ilaziness/vela/internal/i18n"
	"github.com/ilaziness/vela/ui/app"
	"github.com/ilaziness/vela/ui/layout"
)

// header shows the session title, bound-terminal status and panel actions.
func header(w *gui.Window, ctrl *app.Controller, st *app.State) gui.View {
	th := gui.CurrentTheme()
	label, status, _ := ctrl.AgentBindInfo(st)
	bindText := i18n.T("ai.session.bind_none")
	switch {
	case status == "connected" && label != "":
		bindText = i18n.Tf("ai.session.bind_connected", label)
	case status == "connecting" && label != "":
		bindText = i18n.Tf("ai.session.bind_connecting", label)
	case label != "":
		bindText = i18n.Tf("ai.session.bind_disconnected", label)
	}

	title := ""
	if st.AgentChat != nil {
		title = st.AgentChat.SessionTitle
	}
	if title == "" || title == "__new_session__" {
		title = i18n.T("ai.session.new")
	}
	title = layout.Ellipsize(title, 140, func(s string) float32 {
		return w.TextWidth(s, th.TextStyleLabel)
	})

	btn := func(id, glyph, tip string, active bool, onClick func(gui.EventCtx)) gui.View {
		cellID := "agent-hdr-" + id
		return gui.Interactive(cellID, func(is gui.InteractionState) gui.View {
			views := []gui.View{
				gui.Button(gui.ButtonCfg{
					ID:            cellID + "-hit",
					Variant:       gui.ButtonGhost,
					FocusDisabled: true,
					Color:         map[bool]gui.Color{true: th.ColorActive}[active],
					Content: []gui.View{
						gui.Text(gui.TextCfg{Text: glyph, TextStyle: th.TextStyleIconSmall}),
					},
					OnClick: onClick,
				}),
			}
			if is.Hovered && tip != "" {
				views = append(views, layout.Tip(cellID+"-tip", tip,
					gui.FloatBottomLeft, gui.FloatTopLeft, 0, 4))
			}
			return gui.Column(layout.Flush(gui.ContainerCfg{
				ID:      cellID,
				Sizing:  gui.FitFit,
				Content: views,
			}))
		})
	}

	return gui.Column(layout.Flush(gui.ContainerCfg{
		ID:      "agent-header",
		Sizing:  gui.FillFit,
		Color:   th.ColorPanel,
		Spacing: gui.SpacingTight,
		Padding: gui.NewPadding(6, 8, 4, 8),
		Content: []gui.View{
			gui.Row(layout.Flush(gui.ContainerCfg{
				Sizing:  gui.FillFit,
				VAlign:  gui.VAlignMiddle,
				Spacing: gui.SpacingTight,
				Content: []gui.View{
					gui.Text(gui.TextCfg{Text: gui.IconStar, TextStyle: th.TextStyleIconSmall, Sizing: gui.FitFit}),
					gui.Text(gui.TextCfg{
						Text:      title,
						TextStyle: th.TextStyleLabel,
						Sizing:    gui.FillFit,
					}),
					btn("agent-history", gui.IconComment, i18n.T("ai.session.history"), st.AgentHistoryOpen, func(ctx gui.EventCtx) {
						ctx.Consume()
						s := gui.State[app.State](ctx.Window)
						s.AgentHistoryOpen = !s.AgentHistoryOpen
					}),
					btn("agent-new", gui.IconPlus, i18n.T("ai.session.new"), false, func(ctx gui.EventCtx) {
						ctx.Consume()
						ctrl.AgentNewSession(ctx.Window)
					}),
					btn("agent-del", gui.IconTrash, i18n.T("ai.session.delete_title"), false, func(ctx gui.EventCtx) {
						ctx.Consume()
						confirmDeleteSession(ctx.Window, ctrl, st, st.AgentSessionID)
					}),
					btn("agent-close", gui.IconClose, i18n.T("ai.session.close_assistant"), false, func(ctx gui.EventCtx) {
						ctx.Consume()
						ctrl.AgentToggle(ctx.Window)
					}),
				},
			})),
			gui.Text(gui.TextCfg{
				Text:      bindText,
				TextStyle: th.TextStyleSecondary,
				Sizing:    gui.FillFit,
				Mode:      gui.TextModeWrap,
			}),
		},
	}))
}

// historyOverlay is the full-panel overlay layer: a FillFill column that
// left-aligns the fixed-width history panel so it covers the whole panel.
func historyOverlay(w *gui.Window, ctrl *app.Controller, st *app.State) gui.View {
	return gui.Column(layout.Flush(gui.ContainerCfg{
		ID:      "agent-history-overlay",
		Sizing:  gui.FillFill,
		HAlign:  gui.HAlignLeft,
		Content: []gui.View{historyPanel(w, ctrl, st)},
	}))
}

// historyPanel is the session-history sub-column docked on the left of the
// assistant panel, with a right-edge divider.
func historyPanel(w *gui.Window, ctrl *app.Controller, st *app.State) gui.View {
	th := gui.CurrentTheme()

	var rows []gui.View
	for _, sess := range st.AgentSessions {
		rows = append(rows, historyRow(w, ctrl, st, sess))
	}
	if len(rows) == 0 {
		rows = append(rows, gui.Text(gui.TextCfg{
			Text:      i18n.T("ai.session.history"),
			TextStyle: th.TextStyleSecondary,
			Sizing:    gui.FillFit,
		}))
	}

	panel := gui.Column(layout.Flush(gui.ContainerCfg{
		ID:     "agent-history",
		Sizing: gui.FixedFill,
		Width:  200,
		Color:  th.ColorPanel,
		Content: []gui.View{
			gui.Row(layout.Flush(gui.ContainerCfg{
				Sizing:  gui.FillFit,
				VAlign:  gui.VAlignMiddle,
				Spacing: gui.SpacingTight,
				Padding: gui.NewPadding(6, 8, 4, 8),
				Content: []gui.View{
					gui.Text(gui.TextCfg{Text: i18n.T("ai.session.history"), TextStyle: th.TextStyleLabel, Sizing: gui.FillFit}),
					gui.Button(gui.ButtonCfg{
						ID:            "agent-history-close",
						Variant:       gui.ButtonGhost,
						FocusDisabled: true,
						Content: []gui.View{
							gui.Text(gui.TextCfg{Text: gui.IconClose, TextStyle: th.TextStyleIconSmall}),
						},
						OnClick: func(ctx gui.EventCtx) {
							ctx.Consume()
							gui.State[app.State](ctx.Window).AgentHistoryOpen = false
						},
					}),
				},
			})),
			gui.Column(layout.Flush(gui.ContainerCfg{
				ID:           "agent-history-list",
				Sizing:       gui.FillFill,
				Scrollable:   true,
				ScrollMode:   gui.ScrollVerticalOnly,
				Clip:         true,
				ClipContents: true,
				Spacing:      gui.SpacingTight,
				Padding:      gui.NewPadding(4, 8, 8, 8),
				Content:      rows,
			})),
		},
	}))
	return gui.Row(layout.Flush(gui.ContainerCfg{
		ID:      "agent-history-row",
		Sizing:  gui.FixedFill,
		Width:   201,
		Content: []gui.View{panel, layout.VRule()},
	}))
}

// confirmDeleteSession asks for confirmation, then deletes the session.
func confirmDeleteSession(w *gui.Window, ctrl *app.Controller, st *app.State, id string) {
	if id == "" {
		return
	}
	title := ""
	for _, sess := range st.AgentSessions {
		if sess.ID == id {
			title = sess.Title
			break
		}
	}
	if title == "" || title == "__new_session__" {
		title = i18n.T("ai.session.new")
	}
	w.Dialog(gui.DialogCfg{
		DialogType:    gui.DialogConfirm,
		Title:         i18n.T("ai.session.delete_title"),
		Body:          i18n.Tf("ai.session.delete_confirm", title),
		DefaultButton: gui.DialogButtonNo,
		OnOkYes: func(w *gui.Window) {
			ctrl.AgentDeleteSession(w, id)
		},
	})
}

// historyRow is one selectable session entry: title + relative time.
func historyRow(w *gui.Window, ctrl *app.Controller, st *app.State, sess *agent.AISession) gui.View {
	th := gui.CurrentTheme()
	title := sess.Title
	if title == "" || title == "__new_session__" {
		title = i18n.T("ai.session.new")
	}
	when := relTime(sess.UpdatedAt)
	tip := title + " · " + when
	active := sess.ID == st.AgentSessionID
	short := layout.Ellipsize(title, 150, func(s string) float32 {
		return w.TextWidth(s, th.TextStyleBody)
	})

	rowID := "agent-hist-" + sess.ID
	return gui.Interactive(rowID, func(is gui.InteractionState) gui.View {
		bg := gui.Color{}
		if active {
			bg = th.ColorActive
		} else if is.Hovered {
			bg = th.ColorHover
		}
		views := []gui.View{
			gui.Row(layout.Flush(gui.ContainerCfg{
				Sizing:  gui.FillFit,
				VAlign:  gui.VAlignMiddle,
				Spacing: gui.SpacingTight,
				Content: []gui.View{
					gui.Button(gui.ButtonCfg{
						ID:            rowID + "-hit",
						Variant:       gui.ButtonGhost,
						Sizing:        gui.FillFill,
						Padding:       gui.PadAll(6),
						Color:         bg,
						Radius:        gui.RadiusSmall,
						HAlign:        gui.Some(gui.HAlignLeft),
						VAlign:        gui.Some(gui.VAlignMiddle),
						FocusDisabled: true,
						Content: []gui.View{
							gui.Column(layout.Flush(gui.ContainerCfg{
								Sizing:  gui.FillFit,
								Spacing: gui.SpacingTight,
								Content: []gui.View{
									gui.Text(gui.TextCfg{Text: short, TextStyle: th.TextStyleBody, Sizing: gui.FillFit}),
									gui.Text(gui.TextCfg{Text: when, TextStyle: th.TextStyleSecondary, Sizing: gui.FillFit}),
								},
							})),
						},
						OnClick: func(ctx gui.EventCtx) {
							ctx.Consume()
							s := gui.State[app.State](ctx.Window)
							s.AgentHistoryOpen = false
							ctrl.AgentSelectSession(ctx.Window, sess.ID)
						},
					}),
					gui.Button(gui.ButtonCfg{
						ID:            rowID + "-del",
						Variant:       gui.ButtonGhost,
						FocusDisabled: true,
						Content: []gui.View{
							gui.Text(gui.TextCfg{Text: gui.IconTrash, TextStyle: th.TextStyleIconSmall}),
						},
						OnClick: func(ctx gui.EventCtx) {
							ctx.Consume()
							confirmDeleteSession(ctx.Window, ctrl, st, sess.ID)
						},
					}),
				},
			})),
		}
		if is.Hovered {
			views = append(views, layout.Tip(rowID+"-tip", tip,
				gui.FloatMiddleRight, gui.FloatMiddleLeft, 8, 0))
		}
		return gui.Column(layout.Flush(gui.ContainerCfg{
			ID:      rowID,
			Sizing:  gui.FillFit,
			Content: views,
		}))
	})
}

// relTime renders a unix timestamp as "just now / N min ago / N hr ago /
// N days ago" under 30 days, else a short datetime.
func relTime(ts int64) string {
	if ts <= 0 {
		return ""
	}
	d := time.Since(time.Unix(ts, 0))
	switch {
	case d < time.Minute:
		return i18n.T("ai.session.time.now")
	case d < time.Hour:
		return i18n.Tf("ai.session.time.min_ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return i18n.Tf("ai.session.time.hour_ago", int(d.Hours()))
	case d < 30*24*time.Hour:
		return i18n.Tf("ai.session.time.day_ago", int(d.Hours()/24))
	default:
		return time.Unix(ts, 0).Format("2006-01-02 15:04")
	}
}
