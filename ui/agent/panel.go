// Package agent renders the AI assistant side panel: bind status, session
// picker, streaming transcript (markdown / reasoning / tool cards /
// approvals) and the composer.
package agent

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/highlight"

	"github.com/ilaziness/vela/internal/i18n"
	"github.com/ilaziness/vela/ui/agent/model"
	"github.com/ilaziness/vela/ui/app"
	"github.com/ilaziness/vela/ui/layout"
)

// Panel is the right-edge assistant column.
func Panel(w *gui.Window, ctrl *app.Controller) gui.View {
	st := gui.State[app.State](w)
	th := gui.CurrentTheme()
	chat := st.AgentChat

	content := []gui.View{header(w, ctrl, st)}
	// The Stack wrapper is unconditional: it contributes a segment to
	// the transcript's effective ID, and the scroll offset is keyed by
	// that ID. Wrapping only while the jump button is visible would move
	// the offset to a fresh key, so the transcript would flip between
	// two scroll positions on every frame.
	transcriptLayers := []gui.View{transcript(w, ctrl, st, chat)}
	if v := jumpToBottom(w); v != nil {
		transcriptLayers = append(transcriptLayers, v)
	}
	transcriptView := gui.Stack(layout.Flush(gui.ContainerCfg{
		ID:      "agent-transcript-stack",
		Sizing:  gui.FillFill,
		Content: transcriptLayers,
	}))
	body := []gui.View{transcriptView}
	if dock := planDock(w, st, chat); dock != nil {
		body = append(body, dock)
	}
	body = append(body, composer(w, ctrl, st, chat))
	content = append(content, gui.Column(layout.Flush(gui.ContainerCfg{
		ID:      "agent-body",
		Sizing:  gui.FillFill,
		Content: body,
	})))

	main := gui.Column(layout.Flush(gui.ContainerCfg{
		ID:      "agent-panel",
		Sizing:  gui.FillFill,
		Color:   th.ColorPanel,
		Content: content,
	}))
	layers := []gui.View{main}
	if st.AgentHistoryOpen {
		layers = append(layers, historyOverlay(w, ctrl, st))
	}
	// Unconditional for the same reason as the transcript's Stack: the
	// history overlay adds a layer, and an ID-bearing ancestor appearing
	// mid-run would re-key every scroll offset below it.
	return gui.Stack(layout.Flush(gui.ContainerCfg{
		ID:      "agent-panel-stack",
		Sizing:  gui.FillFill,
		Content: layers,
	}))
}

// transcriptScrollID resolves the transcript's effective scroll ID: its
// ancestors carry IDs, so the leaf alone does not address the scrollable.
// ok is false until a frame carrying the transcript has been laid out.
func transcriptScrollID(w *gui.Window) (string, bool) {
	if ids := w.ResolveID("agent-transcript"); len(ids) > 0 {
		return ids[0], true
	}
	return "", false
}

// jumpToBottom is the floating scroll-to-bottom button shown while the
// transcript is scrolled away from the end. It is a Float rather than a
// full-size Stack layer, so it only takes the pointer over the button
// itself and leaves the transcript's hover and wheel handling intact.
func jumpToBottom(w *gui.Window) gui.View {
	sid, ok := transcriptScrollID(w)
	if !ok {
		return nil
	}
	pct := w.ScrollVerticalPct(sid)
	if pct <= 0.02 || pct >= 0.92 {
		return nil
	}
	th := gui.CurrentTheme()
	return gui.Column(layout.Flush(gui.ContainerCfg{
		ID:           "agent-jump-bottom",
		Sizing:       gui.FitFit,
		Float:        true,
		FloatAnchor:  gui.FloatBottomRight,
		FloatTieOff:  gui.FloatBottomRight,
		FloatOffsetX: -12,
		FloatOffsetY: -12,
		FloatZIndex:  10,
		Content: []gui.View{
			gui.Button(gui.ButtonCfg{
				ID:            "agent-jump-bottom-btn",
				Variant:       gui.ButtonSecondary,
				FocusDisabled: true,
				Radius:        gui.RadiusLarge,
				Content: []gui.View{
					gui.Text(gui.TextCfg{Text: gui.IconArrowDown, TextStyle: th.TextStyleIconSmall}),
				},
				OnClick: func(ctx gui.EventCtx) {
					ctx.Consume()
					if sid, ok := transcriptScrollID(ctx.Window); ok {
						ctx.Window.ScrollVerticalToPct(sid, 1)
					}
				},
			}),
		},
	}))
}

// planTask mirrors the upsert_plan payload (internal/ai/tool.PlanSnapshot).
type planTask struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Status string `json:"status"`
}
type planSnapshot struct {
	Title string     `json:"title"`
	Tasks []planTask `json:"tasks"`
}

// latestPlan returns the most recent upsert_plan payload in the transcript.
func latestPlan(chat *model.Chat) *planSnapshot {
	if chat == nil {
		return nil
	}
	var snap *planSnapshot
	for _, m := range chat.Messages {
		for _, b := range m.Blocks {
			if b.Kind != model.BlockTool || b.Tool == nil || b.Tool.Name != "upsert_plan" || len(b.Tool.Input) == 0 {
				continue
			}
			var ps planSnapshot
			if json.Unmarshal(b.Tool.Input, &ps) == nil && len(ps.Tasks) > 0 {
				cp := ps
				snap = &cp
			}
		}
	}
	return snap
}

// planDock is the collapsible plan panel above the composer. Collapsed it
// shows only the plan title; expanded it lists tasks with live statuses.
func planDock(w *gui.Window, st *app.State, chat *model.Chat) gui.View {
	plan := latestPlan(chat)
	if plan == nil {
		return nil
	}
	th := gui.CurrentTheme()
	key := "agent-plan"
	if st.AgentExpanded == nil {
		st.AgentExpanded = map[string]bool{}
	}
	open := st.AgentExpanded[key]

	title := plan.Title
	if title == "" {
		title = i18n.T("ai.plan.title")
	}
	head := gui.Row(layout.Flush(gui.ContainerCfg{
		Sizing:  gui.FillFit,
		VAlign:  gui.VAlignMiddle,
		Spacing: gui.SpacingTight,
		Content: []gui.View{
			gui.Text(gui.TextCfg{Text: disclosureGlyph(open), TextStyle: th.TextStyleIconSmall, Sizing: gui.FitFit}),
			gui.Text(gui.TextCfg{Text: gui.IconListBullet, TextStyle: th.TextStyleIconSmall, Sizing: gui.FitFit}),
			gui.Text(gui.TextCfg{
				Text:      layout.Ellipsize(title, 200, func(s string) float32 { return w.TextWidth(s, th.TextStyleLabel) }),
				TextStyle: th.TextStyleLabel,
				Sizing:    gui.FillFit,
			}),
		},
	}))

	var tasks []gui.View
	if open {
		for _, t := range plan.Tasks {
			tasks = append(tasks, planTaskRow(th, t))
		}
	}
	return gui.Column(layout.Flush(gui.ContainerCfg{
		ID:      "agent-plan-dock",
		Sizing:  gui.FillFit,
		Spacing: gui.SpacingTight,
		Padding: gui.NewPadding(4, 10, 4, 10),
		Color:   th.ColorInterior,
		Content: []gui.View{
			gui.ExpandPanel(gui.ExpandPanelCfg{
				ID:      "agent-plan-expand",
				Head:    head,
				Open:    open,
				Sizing:  gui.FillFit,
				Content: gui.Column(layout.Flush(gui.ContainerCfg{Sizing: gui.FillFit, Spacing: gui.SpacingTight, Content: tasks})),
				OnToggle: func(ctx gui.EventCtx) {
					ctx.Consume()
					s := gui.State[app.State](ctx.Window)
					if s.AgentExpanded == nil {
						s.AgentExpanded = map[string]bool{}
					}
					s.AgentExpanded[key] = !open
				},
			}),
		},
	}))
}

func planTaskRow(th gui.Theme, t planTask) gui.View {
	glyph := gui.IconClock
	style := th.TextStyleSecondary
	label := i18n.T("ai.plan.pending")
	switch t.Status {
	case "running":
		glyph = gui.IconClock
		style = th.TextStyleLabel
		label = i18n.T("ai.plan.running")
	case "done":
		glyph = gui.IconCheck
		style = th.TextStyleSecondary
		label = i18n.T("ai.plan.done")
	case "failed":
		glyph = gui.IconClose
		style = th.TextStyleError
		label = i18n.T("ai.plan.failed")
	}
	return gui.Row(layout.Flush(gui.ContainerCfg{
		Sizing:  gui.FillFit,
		VAlign:  gui.VAlignTop,
		Spacing: gui.SpacingTight,
		Content: []gui.View{
			gui.Text(gui.TextCfg{Text: glyph, TextStyle: th.TextStyleIconXSmall, Sizing: gui.FitFit}),
			gui.Text(gui.TextCfg{Text: t.Title, TextStyle: th.TextStyleBodySmall, Mode: gui.TextModeWrap, Sizing: gui.FillFit}),
			gui.Text(gui.TextCfg{Text: label, TextStyle: style, Sizing: gui.FitFit}),
		},
	}))
}

// transcript is the scrollable message list.
func transcript(w *gui.Window, ctrl *app.Controller, st *app.State, chat *model.Chat) gui.View {
	th := gui.CurrentTheme()
	sid, laidOut := transcriptScrollID(w)
	switch {
	case chat != nil && st.AgentScrollChat != chat:
		// A transcript shown for the first time (panel opened or session
		// switched) starts at the end. The scroll ID only resolves after
		// a laid-out frame, and every pull reads the height laid out on
		// the previous frame — right after a load that is still the old
		// content. So the first pull is never taken as final: the next
		// frame says whether it landed, and a longer chat needs another.
		if laidOut {
			if st.AgentScrollPushed && w.ScrollVerticalPct(sid) > 0.99 {
				st.AgentScrollChat = chat
				st.AgentScrollPushed = false
				st.AgentSeenRev = chat.Rev
			} else {
				w.ScrollVerticalToPct(sid, 1)
				st.AgentScrollPushed = true
			}
		}
	case chat != nil && chat.Rev != st.AgentSeenRev:
		st.AgentSeenRev = chat.Rev
		if laidOut && (chat.Running || w.ScrollVerticalPct(sid) > 0.7) {
			w.ScrollVerticalToPct(sid, 1)
		}
	}

	var msgs []gui.View
	switch {
	case chat == nil:
		msgs = append(msgs, hint(th, i18n.T("common.loading")))
	case len(chat.Messages) == 0:
		msgs = append(msgs, hint(th, i18n.T("ai.chat.empty")))
	default:
		for _, m := range chat.Messages {
			msgs = append(msgs, messageView(w, ctrl, st, chat, m))
		}
		// Approvals whose tool block is gone (edge cases) still surface
		// inline at the end of the transcript.
		for _, ap := range chat.Approvals {
			if !approvalInTranscript(chat, ap) {
				msgs = append(msgs, approvalCard(w, ctrl, ap))
			}
		}
	}

	return gui.Column(layout.Flush(gui.ContainerCfg{
		ID:           "agent-transcript",
		Sizing:       gui.FillFill,
		Scrollable:   true,
		ScrollMode:   gui.ScrollVerticalOnly,
		Clip:         true,
		ClipContents: true,
		Spacing:      gui.SpacingMedium,
		Padding:      gui.NewPadding(8, 12, 8, 8),
		Content:      msgs,
	}))
}

func hint(th gui.Theme, text string) gui.View {
	return gui.Text(gui.TextCfg{
		Text:      text,
		TextStyle: th.TextStyleSecondary,
		Mode:      gui.TextModeWrap,
		Sizing:    gui.FillFit,
	})
}

// messageView renders one chat turn in messenger style: avatar + name +
// bubble + meta row. User turns sit on the right (avatar outermost),
// assistant turns on the left.
func messageView(w *gui.Window, ctrl *app.Controller, st *app.State, chat *model.Chat, m *model.Message) gui.View {
	th := gui.CurrentTheme()
	isUser := m.Role == model.RoleUser

	var blocks []gui.View
	if isUser {
		// The bubble spans the row, so the text itself carries the right
		// alignment: go-gui only distributes Fill children, so a wrapping
		// box cannot shrink back to its content.
		style := th.TextStyleBody
		style.Align = gui.TextAlignRight
		blocks = append(blocks, gui.Text(gui.TextCfg{
			Text:      m.Text(),
			TextStyle: style,
			Mode:      gui.TextModeWrap,
			Sizing:    gui.FillFit,
		}))
	} else {
		for i, b := range m.Blocks {
			if v := blockView(w, ctrl, st, chat, m, b, i); v != nil {
				blocks = append(blocks, v)
			}
		}
		if m.Status == model.MsgCancelled {
			blocks = append(blocks, gui.Text(gui.TextCfg{
				Text:      i18n.T("ai.chat.stopped"),
				TextStyle: th.TextStyleSecondary,
				Sizing:    gui.FillFit,
			}))
		}
		if m.Status == model.MsgRunning {
			blocks = append(blocks, orbRow(m))
		}
	}

	bubbleColor := th.ColorInterior
	avatarBg := th.ColorActive
	avatarGlyph := gui.IconStar
	name := i18n.T("ai.chat.user_assistant")
	if isUser {
		bubbleColor = th.ColorAccentSubtle
		avatarBg = th.ColorSelect
		avatarGlyph = gui.IconUser
		name = i18n.T("ai.chat.user_you")
	}

	align := gui.HAlignLeft
	if isUser {
		align = gui.HAlignRight
	}
	aligned := func(v gui.View) gui.View {
		return gui.Row(layout.Flush(gui.ContainerCfg{
			Sizing:  gui.FillFit,
			HAlign:  align,
			Content: []gui.View{v},
		}))
	}

	bubble := gui.Column(layout.Flush(gui.ContainerCfg{
		Sizing:  gui.FillFit,
		Color:   bubbleColor,
		Radius:  gui.RadiusMedium,
		Padding: gui.NewPadding(8, 10, 8, 10),
		Spacing: gui.SpacingTight,
		Content: blocks,
	}))

	meta := gui.Row(layout.Flush(gui.ContainerCfg{
		Sizing:  gui.FillFit,
		HAlign:  align,
		VAlign:  gui.VAlignMiddle,
		Spacing: gui.SpacingTight,
		Content: []gui.View{
			gui.Text(gui.TextCfg{
				Text:      msgTime(m.Time),
				TextStyle: th.TextStyleCaptionSmall,
				Sizing:    gui.FitFit,
			}),
			gui.Button(gui.ButtonCfg{
				ID:            "agent-copy-" + m.ID,
				Variant:       gui.ButtonGhost,
				FocusDisabled: true,
				Content: []gui.View{
					gui.Text(gui.TextCfg{Text: i18n.T("ai.chat.copy"), TextStyle: th.TextStyleCaptionSmall}),
				},
				OnClick: func(ctx gui.EventCtx) {
					ctx.Consume()
					ctx.Window.SetClipboard(m.Text())
				},
			}),
		},
	}))

	body := gui.Column(layout.Flush(gui.ContainerCfg{
		Sizing:  gui.FillFit,
		Spacing: gui.SpacingTight,
		Content: []gui.View{
			aligned(gui.Text(gui.TextCfg{
				Text:      name,
				TextStyle: th.TextStyleCaption,
				Sizing:    gui.FitFit,
			})),
			bubble,
			meta,
		},
	}))

	avatar := gui.Column(layout.Flush(gui.ContainerCfg{
		Sizing: gui.FixedFixed,
		Width:  30,
		Height: 30,
		Color:  avatarBg,
		Radius: gui.RadiusLarge,
		HAlign: gui.HAlignCenter,
		VAlign: gui.VAlignMiddle,
		Content: []gui.View{
			gui.Text(gui.TextCfg{Text: avatarGlyph, TextStyle: th.TextStyleIconSmall}),
		},
	}))

	rowContent := []gui.View{avatar, body}
	if isUser {
		rowContent = []gui.View{body, avatar}
	}
	return gui.Row(layout.Flush(gui.ContainerCfg{
		ID:      "agent-msg-" + m.ID,
		Sizing:  gui.FillFit,
		VAlign:  gui.VAlignTop,
		Spacing: gui.SpacingMedium,
		Content: rowContent,
	}))
}

// orbRow is the streaming indicator at the bottom of a running assistant
// bubble: a thinking orb while reasoning streams, a breathing orb while
// waiting for the model otherwise.
func orbRow(m *model.Message) gui.View {
	design := gui.ThinkingOrbBreathing
	text := i18n.T("ai.chat.waiting")
	if n := len(m.Blocks); n > 0 && m.Blocks[n-1].Kind == model.BlockReasoning {
		design = gui.ThinkingOrbSolving
		text = i18n.T("ai.chat.thinking")
	}
	return gui.ThinkingOrbLabel(gui.ThinkingOrbLabelCfg{
		ID:        "agent-orb-" + m.ID,
		Text:      text,
		Design:    design,
		Size:      gui.ThinkingOrbSmall,
		TextStyle: gui.CurrentTheme().TextStyleSecondary,
		Sizing:    gui.FillFit,
	})
}

// msgTime formats a unix timestamp: HH:MM today, else short date+time.
func msgTime(ts int64) string {
	if ts <= 0 {
		return ""
	}
	t := time.Unix(ts, 0)
	now := time.Now()
	if t.Year() == now.Year() && t.YearDay() == now.YearDay() {
		return t.Format("15:04")
	}
	return t.Format("2006-01-02 15:04")
}

// blockView renders one ordered piece of an assistant message.
func blockView(w *gui.Window, ctrl *app.Controller, st *app.State, chat *model.Chat, m *model.Message, b *model.Block, idx int) gui.View {
	th := gui.CurrentTheme()
	switch b.Kind {
	case model.BlockReasoning:
		key := fmt.Sprintf("reasoning-%s-%d", m.ID, idx)
		open := st.AgentExpanded[key]
		label := i18n.T("ai.chat.thinking")
		if idx < len(m.Blocks)-1 || m.Status != model.MsgRunning {
			label = i18n.T("ai.chat.thought")
		}
		head := gui.Row(layout.Flush(gui.ContainerCfg{
			Sizing:  gui.FillFit,
			VAlign:  gui.VAlignMiddle,
			Spacing: gui.SpacingTight,
			Content: []gui.View{
				gui.Text(gui.TextCfg{Text: disclosureGlyph(open), TextStyle: th.TextStyleIconSmall, Sizing: gui.FitFit}),
				gui.Text(gui.TextCfg{
					Text:      label,
					TextStyle: th.TextStyleSecondary,
					Sizing:    gui.FillFit,
				}),
			},
		}))
		return gui.ExpandPanel(gui.ExpandPanelCfg{
			ID:      "agent-" + key,
			Head:    head,
			Open:    open,
			Sizing:  gui.FillFit,
			Content: gui.Text(gui.TextCfg{Text: b.Text, TextStyle: th.TextStyleSecondary, Mode: gui.TextModeWrap, Sizing: gui.FillFit}),
			OnToggle: func(ctx gui.EventCtx) {
				ctx.Consume()
				s := gui.State[app.State](ctx.Window)
				if s.AgentExpanded == nil {
					s.AgentExpanded = map[string]bool{}
				}
				s.AgentExpanded[key] = !open
			},
		})
	case model.BlockTool:
		if b.Tool != nil && b.Tool.Status == model.ToolAwaiting {
			if ap := findApproval(chat, b.Tool.ID); ap != nil {
				return approvalCard(w, ctrl, ap)
			}
		}
		return toolCard(st, b.Tool)
	default:
		if strings.TrimSpace(b.Text) == "" {
			return nil
		}
		style := gui.DefaultMarkdownStyle()
		style.CodeHighlighter = highlight.Default()
		return w.Markdown(gui.MarkdownCfg{
			ID:                  "agent-md-" + m.ID + "-" + fmt.Sprint(idx),
			Source:              b.Text,
			Style:               style,
			DisableExternalAPIs: true,
			AllowRemoteImages:   false,
			AllowLocalImages:    false,
		})
	}
}

func disclosureGlyph(open bool) string {
	if open {
		return gui.IconDropDown
	}
	return gui.IconDropRight
}

// toolCard renders one tool invocation: name, status, args, output.
func toolCard(st *app.State, t *model.ToolCall) gui.View {
	th := gui.CurrentTheme()
	if t == nil {
		return nil
	}

	statusText, statusStyle := toolStatusText(th, t.Status)
	header := gui.Row(layout.Flush(gui.ContainerCfg{
		Sizing:  gui.FillFit,
		VAlign:  gui.VAlignMiddle,
		Spacing: gui.SpacingTight,
		Content: []gui.View{
			gui.Text(gui.TextCfg{Text: gui.IconCode, TextStyle: th.TextStyleIconSmall, Sizing: gui.FitFit}),
			gui.Text(gui.TextCfg{Text: toolLabel(t.Name), TextStyle: th.TextStyleBody, Sizing: gui.FitFit}),
			gui.Text(gui.TextCfg{Text: statusText, TextStyle: statusStyle, Sizing: gui.FillFit}),
		},
	}))

	content := []gui.View{header}
	if summary := toolInputSummary(t); summary != "" {
		content = append(content, codeBlock(th, summary))
	}
	if t.ErrorText != "" {
		content = append(content, gui.Text(gui.TextCfg{
			Text:      t.ErrorText,
			TextStyle: th.TextStyleError,
			Mode:      gui.TextModeWrap,
			Sizing:    gui.FillFit,
		}))
	}
	if len(t.Output) > 0 {
		key := "tool-" + t.ID + "-out"
		open := st.AgentExpanded[key]
		outHead := gui.Row(layout.Flush(gui.ContainerCfg{
			Sizing:  gui.FillFit,
			VAlign:  gui.VAlignMiddle,
			Spacing: gui.SpacingTight,
			Content: []gui.View{
				gui.Text(gui.TextCfg{Text: disclosureGlyph(open), TextStyle: th.TextStyleIconSmall, Sizing: gui.FitFit}),
				gui.Text(gui.TextCfg{Text: i18n.T("ai.chat.tool_output"), TextStyle: th.TextStyleSecondary, Sizing: gui.FillFit}),
			},
		}))
		content = append(content, gui.ExpandPanel(gui.ExpandPanelCfg{
			ID:      "agent-" + key,
			Head:    outHead,
			Open:    open,
			Sizing:  gui.FillFit,
			Content: codeBlock(th, rawText(t.Output)),
			OnToggle: func(ctx gui.EventCtx) {
				ctx.Consume()
				s := gui.State[app.State](ctx.Window)
				if s.AgentExpanded == nil {
					s.AgentExpanded = map[string]bool{}
				}
				s.AgentExpanded[key] = !open
			},
		}))
	}

	return gui.Column(layout.Flush(gui.ContainerCfg{
		ID:      "agent-tool-" + t.ID,
		Sizing:  gui.FillFit,
		Spacing: gui.SpacingTight,
		Padding: gui.NewPadding(4, 6, 4, 6),
		Color:   th.ColorBackground,
		Radius:  gui.RadiusSmall,
		Content: content,
	}))
}

func codeBlock(th gui.Theme, text string) gui.View {
	return gui.Text(gui.TextCfg{
		Text:      text,
		TextStyle: th.TextStyleCodeSmall,
		Mode:      gui.TextModeWrap,
		Sizing:    gui.FillFit,
	})
}

func toolStatusText(th gui.Theme, s model.ToolStatus) (string, gui.TextStyle) {
	switch s {
	case model.ToolAwaiting:
		return i18n.T("ai.chat.state.awaiting_approval"), th.TextStyleSecondary
	case model.ToolSuccess:
		return i18n.T("ai.chat.state.done"), th.TextStyleSecondary
	case model.ToolError:
		return i18n.T("ai.chat.state.failed"), th.TextStyleError
	case model.ToolDenied:
		return i18n.T("ai.chat.state.denied"), th.TextStyleSecondary
	default:
		return i18n.T("ai.chat.state.running"), th.TextStyleSecondary
	}
}

func toolLabel(name string) string {
	if name == "" {
		return "tool"
	}
	return name
}

// toolInputSummary renders tool args compactly; run_ssh_command shows the
// command first for readability.
func toolInputSummary(t *model.ToolCall) string {
	if len(t.Input) == 0 {
		return ""
	}
	var m map[string]any
	if err := json.Unmarshal(t.Input, &m); err == nil {
		if cmd, ok := m["command"].(string); ok && cmd != "" {
			if t.Name == "run_ssh_command" {
				return "$ " + cmd
			}
		}
	}
	return rawText(t.Input)
}

func rawText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return string(raw)
	}
	if s, ok := v.(string); ok {
		return s
	}
	var sb strings.Builder
	enc := json.NewEncoder(&sb)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
	return strings.TrimRight(sb.String(), "\n")
}

// findApproval returns the pending approval bound to a tool call id.
func findApproval(chat *model.Chat, callID string) *model.Approval {
	if chat == nil {
		return nil
	}
	for _, ap := range chat.Approvals {
		if ap.ToolCallID == callID {
			return ap
		}
	}
	return nil
}

// approvalInTranscript reports whether the approval's tool block is still
// present and awaiting inside the transcript.
func approvalInTranscript(chat *model.Chat, ap *model.Approval) bool {
	for _, m := range chat.Messages {
		for _, b := range m.Blocks {
			if b.Kind == model.BlockTool && b.Tool != nil &&
				b.Tool.ID == ap.ToolCallID && b.Tool.Status == model.ToolAwaiting {
				return true
			}
		}
	}
	return false
}

// approvalCard is the approve/deny card rendered inline in the transcript.
func approvalCard(w *gui.Window, ctrl *app.Controller, ap *model.Approval) gui.View {
	th := gui.CurrentTheme()
	return gui.Column(layout.Flush(gui.ContainerCfg{
		ID:      "agent-approval-" + ap.ID,
		Sizing:  gui.FillFit,
		Spacing: gui.SpacingTight,
		Padding: gui.NewPadding(6, 8, 6, 8),
		Color:   th.ColorActive,
		Radius:  gui.RadiusSmall,
		Content: []gui.View{
			gui.Row(layout.Flush(gui.ContainerCfg{
				Sizing:  gui.FillFit,
				VAlign:  gui.VAlignMiddle,
				Spacing: gui.SpacingTight,
				Content: []gui.View{
					gui.Text(gui.TextCfg{Text: gui.IconShield, TextStyle: th.TextStyleIconSmall, Sizing: gui.FitFit}),
					gui.Text(gui.TextCfg{
						Text:      fmt.Sprintf("%s: %s", i18n.T("ai.chat.state.awaiting_approval"), toolLabel(ap.ToolName)),
						TextStyle: th.TextStyleLabel,
						Sizing:    gui.FillFit,
					}),
				},
			})),
			codeBlock(th, rawText(ap.Input)),
			gui.Row(layout.Flush(gui.ContainerCfg{
				Sizing:  gui.FillFit,
				Spacing: gui.SpacingSmall,
				Content: []gui.View{
					gui.Button(gui.ButtonCfg{
						ID:      "agent-approve-" + ap.ID,
						Label:   i18n.T("ai.chat.approve"),
						Variant: gui.ButtonPrimary,
						OnClick: func(ctx gui.EventCtx) {
							ctx.Consume()
							ctrl.AgentRespondApproval(ap.ID, true, "")
						},
					}),
					gui.Button(gui.ButtonCfg{
						ID:      "agent-deny-" + ap.ID,
						Label:   i18n.T("ai.chat.deny"),
						Variant: gui.ButtonGhost,
						OnClick: func(ctx gui.EventCtx) {
							ctx.Consume()
							ctrl.AgentRespondApproval(ap.ID, false, "")
						},
					}),
				},
			})),
		},
	}))
}

// composer is the bottom input row: multiline draft + send/stop.
func composer(w *gui.Window, ctrl *app.Controller, st *app.State, chat *model.Chat) gui.View {
	th := gui.CurrentTheme()
	running := chat != nil && chat.Running
	svcReady := ctrl.AgentServiceReady()

	placeholder := i18n.T("ai.chat.placeholder")
	if !svcReady {
		placeholder = i18n.T("ai.chat.disabled_hint")
	}
	input := gui.Input(gui.InputCfg{
		ID:          "agent-composer",
		Mode:        gui.InputMultiline,
		Scrollable:  true,
		Text:        st.AgentDraft,
		Placeholder: placeholder,
		Sizing:      gui.FillFit,
		Height:      44,
		Disabled:    !svcReady,
		OnTextChanged: func(text string, ctx gui.EventCtx) {
			gui.State[app.State](ctx.Window).AgentDraft = text
		},
	})

	action := gui.Button(gui.ButtonCfg{
		ID:            "agent-send",
		Variant:       gui.ButtonPrimary,
		FocusDisabled: true,
		Disabled:      !svcReady || (!running && strings.TrimSpace(st.AgentDraft) == ""),
		Content: []gui.View{
			gui.Text(gui.TextCfg{
				Text:      map[bool]string{true: gui.IconStop, false: gui.IconPaperPlane}[running],
				TextStyle: th.TextStyleIconSmall,
			}),
		},
		OnClick: func(ctx gui.EventCtx) {
			ctx.Consume()
			s := gui.State[app.State](ctx.Window)
			if running {
				ctrl.AgentStop()
				return
			}
			text := strings.TrimSpace(s.AgentDraft)
			if text == "" {
				return
			}
			s.AgentDraft = ""
			ctrl.AgentSend(text)
		},
	})

	return gui.Column(layout.Flush(gui.ContainerCfg{
		ID:      "agent-composer-row",
		Sizing:  gui.FillFit,
		Spacing: gui.SpacingTight,
		Padding: gui.NewPadding(6, 8, 8, 8),
		Content: []gui.View{
			input,
			gui.Row(layout.Flush(gui.ContainerCfg{
				Sizing:  gui.FillFit,
				HAlign:  gui.HAlignRight,
				Content: []gui.View{action},
			})),
		},
	}))
}
