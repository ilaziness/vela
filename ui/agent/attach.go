package agent

import (
	"fmt"
	"strings"

	"github.com/go-gui-org/go-gui/gui"
	_ "golang.org/x/image/webp" // register WebP so image previews decode

	"github.com/ilaziness/vela/internal/i18n"
	"github.com/ilaziness/vela/ui/agent/model"
	"github.com/ilaziness/vela/ui/app"
	"github.com/ilaziness/vela/ui/layout"
)

const (
	attachChipSize = 32
	// Message chips sit under a one-line bubble, so the tile hugs the glyph.
	messageAttachChipSize = 20
)

func attachGlyph(mime string) string {
	switch {
	case strings.HasPrefix(strings.ToLower(mime), "image/"):
		return gui.IconFileImage
	case strings.EqualFold(mime, "application/pdf"):
		return gui.IconFile
	default:
		return gui.IconDocument
	}
}

func isImageMIME(mime string) bool {
	return strings.HasPrefix(strings.ToLower(mime), "image/")
}

// draftAttachRow is the icon strip above the composer. Nil when nothing is queued.
func draftAttachRow(ctrl *app.Controller, atts []model.Attachment) gui.View {
	if len(atts) == 0 {
		return nil
	}
	chips := make([]gui.View, len(atts))
	for i, att := range atts {
		chips[i] = draftAttachChip(i, att, ctrl)
	}
	return gui.Row(layout.Flush(gui.ContainerCfg{
		ID:      "agent-draft-atts",
		Sizing:  gui.FillFit,
		HAlign:  gui.HAlignLeft,
		VAlign:  gui.VAlignMiddle,
		Spacing: gui.SpacingSmall,
		Content: chips,
	}))
}

func draftAttachChip(i int, att model.Attachment, ctrl *app.Controller) gui.View {
	th := gui.CurrentTheme()
	id := fmt.Sprintf("agent-draft-att-%d", i)
	path := att.Path
	glyph := gui.Text(gui.TextCfg{
		ID:        id + "-icon",
		Text:      attachGlyph(att.MIME),
		TextStyle: th.TextStyleIconSmall,
	})
	closeBtn := gui.Button(gui.ButtonCfg{
		ID:            id + "-x",
		Variant:       gui.ButtonGhost,
		FocusDisabled: true,
		Padding:       gui.PadAll(0),
		Sizing:        gui.FixedFixed,
		Width:         12,
		Height:        12,
		A11YCfg:       gui.A11YCfg{A11YLabel: i18n.T("ai.attach.remove")},
		Content: []gui.View{
			gui.Text(gui.TextCfg{Text: gui.IconClose, TextStyle: th.TextStyleIconXSmall}),
		},
		OnClick: func(ctx gui.EventCtx) {
			ctx.Consume()
			ctrl.AgentRemoveAttachment(ctx.Window, path)
		},
	})
	return gui.Interactive(id, func(is gui.InteractionState) gui.View {
		layers := []gui.View{
			gui.Column(layout.Flush(gui.ContainerCfg{
				Sizing:  gui.FillFill,
				HAlign:  gui.HAlignCenter,
				VAlign:  gui.VAlignMiddle,
				Padding: gui.NewPadding(1, 8, 0, 0),
				Content: []gui.View{glyph},
			})),
			gui.Column(layout.Flush(gui.ContainerCfg{
				Sizing:  gui.FillFill,
				HAlign:  gui.HAlignRight,
				VAlign:  gui.VAlignTop,
				Content: []gui.View{closeBtn},
			})),
		}
		if is.Hovered && att.Filename != "" {
			layers = append(layers, layout.Tooltip(id+"-tip", att.Filename,
				gui.FloatBottomCenter, gui.FloatTopCenter, 0, 4))
		}
		return gui.Stack(layout.Flush(gui.ContainerCfg{
			ID:      id,
			Sizing:  gui.FixedFixed,
			Width:   attachChipSize,
			Height:  attachChipSize,
			Radius:  gui.RadiusSmall,
			Color:   th.ColorInterior,
			Content: layers,
		}))
	})
}

// messageAttachRow draws sent-file icons under a user bubble. Image chips open a preview.
func messageAttachRow(ctrl *app.Controller, m *model.Message) gui.View {
	if m == nil || len(m.Attachments) == 0 {
		return nil
	}
	chips := make([]gui.View, len(m.Attachments))
	for i, att := range m.Attachments {
		chips[i] = messageAttachChip(m.ID, i, att, ctrl)
	}
	return gui.Row(layout.Flush(gui.ContainerCfg{
		ID:      "agent-msg-atts-" + m.ID,
		Sizing:  gui.FillFit,
		HAlign:  gui.HAlignRight,
		VAlign:  gui.VAlignMiddle,
		Spacing: gui.SpacingTight,
		Content: chips,
	}))
}

func messageAttachChip(msgID string, i int, att model.Attachment, ctrl *app.Controller) gui.View {
	th := gui.CurrentTheme()
	id := fmt.Sprintf("agent-msg-att-%s-%d", msgID, i)
	glyph := gui.Text(gui.TextCfg{Text: attachGlyph(att.MIME), TextStyle: th.TextStyleIconSmall})
	var face gui.View
	if isImageMIME(att.MIME) {
		preview := att
		face = gui.Button(gui.ButtonCfg{
			ID:            id + "-hit",
			Variant:       gui.ButtonGhost,
			FocusDisabled: true,
			Padding:       gui.PadAll(0),
			Sizing:        gui.FitFit,
			A11YCfg:       gui.A11YCfg{A11YLabel: i18n.Tf("ai.chat.preview_image", att.Filename)},
			Content:       []gui.View{glyph},
			OnClick: func(ctx gui.EventCtx) {
				ctx.Consume()
				ctrl.AgentPreviewImage(ctx.Window, preview)
			},
		})
	} else {
		face = glyph
	}
	return gui.Interactive(id, func(is gui.InteractionState) gui.View {
		layers := []gui.View{
			gui.Column(layout.Flush(gui.ContainerCfg{
				Sizing:  gui.FillFill,
				HAlign:  gui.HAlignCenter,
				VAlign:  gui.VAlignMiddle,
				Content: []gui.View{face},
			})),
		}
		if is.Hovered && att.Filename != "" {
			layers = append(layers, layout.Tooltip(id+"-tip", att.Filename,
				gui.FloatBottomCenter, gui.FloatTopCenter, 0, 4))
		}
		return gui.Stack(layout.Flush(gui.ContainerCfg{
			ID:      id,
			Sizing:  gui.FixedFixed,
			Width:   messageAttachChipSize,
			Height:  messageAttachChipSize,
			Radius:  gui.RadiusSmall,
			Color:   th.ColorInterior,
			Content: layers,
		}))
	})
}
