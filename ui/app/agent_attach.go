package app

import (
	"image"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-gui-org/go-gui/gui"
	"go.uber.org/zap"

	"github.com/ilaziness/vela/internal/ai"
	"github.com/ilaziness/vela/internal/i18n"
	agentmodel "github.com/ilaziness/vela/ui/agent/model"
	"github.com/ilaziness/vela/ui/layout"
)

// attachExts are the extensions large-model chats accept. They match
// internal/ai mimeFromExt; the picker filter uses the same set.
var attachExts = []string{"png", "jpg", "jpeg", "webp", "gif", "pdf", "txt", "md", "markdown", "csv"}

// AgentPickAttachments opens the OS file picker and queues accepted files
// on the composer. Paths are stored as-is; nothing is copied.
func (c *Controller) AgentPickAttachments(w *gui.Window) {
	if w == nil {
		return
	}
	st := gui.State[State](w)
	if len(st.AgentAttachments) >= ai.MaxAttachCount {
		c.toast(gui.ToastError, "", i18n.Tf("ai.attach.max_count", ai.MaxAttachCount))
		return
	}
	w.NativeOpenDialog(gui.NativeOpenDialogCfg{
		Title:         i18n.T("dialog.select_attachment"),
		StartDir:      c.lastDialogDir(),
		AllowMultiple: true,
		Filters: []gui.NativeFileFilter{{
			Name:       i18n.T("dialog.attachment_filter"),
			Extensions: attachExts,
		}},
		OnDone: func(res gui.NativeDialogResult, w *gui.Window) {
			paths := res.PathStrings()
			if res.Status != gui.DialogOK || len(paths) == 0 {
				return
			}
			c.rememberDialogPath(paths[0])
			c.agentAddAttachmentPaths(w, paths)
		},
	})
}

// AgentRemoveAttachment drops one queued composer file by absolute path.
func (c *Controller) AgentRemoveAttachment(w *gui.Window, path string) {
	if w == nil || path == "" {
		return
	}
	st := gui.State[State](w)
	kept := make([]agentmodel.Attachment, 0, len(st.AgentAttachments))
	for _, a := range st.AgentAttachments {
		if a.Path != path {
			kept = append(kept, a)
		}
	}
	st.AgentAttachments = kept
}

// AgentPreviewImage opens a dialog for an image attachment. A missing
// source file is reported; the engine still errors if that path is sent.
func (c *Controller) AgentPreviewImage(w *gui.Window, att agentmodel.Attachment) {
	if w == nil {
		return
	}
	if _, err := os.Stat(att.Path); err != nil {
		c.logger.Warn("agent attachment preview", zap.String("path", att.Path), zap.Error(err))
		c.toast(gui.ToastError, "", i18n.T("ai.attach.missing"))
		return
	}
	name := att.Filename
	if name == "" {
		name = filepath.Base(att.Path)
	}
	path := att.Path
	// Image with no width/height lays out at 100×100 and stretches. Fit the
	// real pixel size into the dialog instead.
	iw, ih := fitPreviewImage(path, 600, 420)
	w.Dialog(gui.DialogCfg{
		DialogType: gui.DialogCustom,
		Title:      i18n.Tf("ai.chat.preview_image", name),
		Width:      iw + 32,
		CustomView: func(win *gui.Window) gui.View {
			return gui.Column(layout.Flush(gui.ContainerCfg{
				Sizing:  gui.FitFit,
				Spacing: gui.SpacingSmall,
				HAlign:  gui.HAlignCenter,
				Content: []gui.View{
					gui.Image(gui.ImageCfg{
						ID:     "agent-attach-preview",
						Src:    path,
						Width:  iw,
						Height: ih,
					}),
					gui.Row(layout.Flush(gui.ContainerCfg{
						Sizing: gui.FillFit,
						HAlign: gui.HAlignRight,
						Content: []gui.View{
							gui.Button(gui.ButtonCfg{
								ID:      "agent-attach-preview-close",
								Label:   i18n.T("common.close"),
								Variant: gui.ButtonGhost,
								OnClick: func(ctx gui.EventCtx) {
									ctx.Consume()
									ctx.Window.DialogDismiss()
								},
							}),
						},
					})),
				},
			}))
		},
	})
}

// fitPreviewImage returns the image's pixel size scaled down to fit maxW×maxH.
// A file that cannot be measured falls back to that box.
func fitPreviewImage(path string, maxW, maxH float32) (float32, float32) {
	f, err := os.Open(path)
	if err != nil {
		return maxW, maxH
	}
	defer f.Close()
	cfg, _, err := image.DecodeConfig(f)
	if err != nil || cfg.Width <= 0 || cfg.Height <= 0 {
		return maxW, maxH
	}
	w := float32(cfg.Width)
	h := float32(cfg.Height)
	if w <= maxW && h <= maxH {
		return w, h
	}
	if w/maxW > h/maxH {
		return maxW, h * maxW / w
	}
	return w * maxH / h, maxH
}

func (c *Controller) agentAddAttachmentPaths(w *gui.Window, paths []string) {
	st := gui.State[State](w)
	have := make(map[string]struct{}, len(st.AgentAttachments))
	for _, a := range st.AgentAttachments {
		have[a.Path] = struct{}{}
	}
	var added []agentmodel.Attachment
	toldSize, toldType := false, false
	for _, raw := range paths {
		if len(st.AgentAttachments)+len(added) >= ai.MaxAttachCount {
			c.toast(gui.ToastError, "", i18n.Tf("ai.attach.max_count", ai.MaxAttachCount))
			break
		}
		abs, mime, name, reason := inspectComposerAttachment(raw)
		if reason != "" {
			c.logger.Debug("agent attachment rejected", zap.String("path", raw), zap.String("reason", reason))
			switch reason {
			case "size":
				if !toldSize {
					c.toast(gui.ToastError, "", i18n.Tf("ai.attach.max_size", int(ai.MaxAttachBytes>>20)))
					toldSize = true
				}
			default:
				if !toldType {
					c.toast(gui.ToastError, "", i18n.T("ai.attach.unsupported_type"))
					toldType = true
				}
			}
			continue
		}
		if _, ok := have[abs]; ok {
			continue
		}
		added = append(added, agentmodel.Attachment{Path: abs, Filename: name, MIME: mime})
		have[abs] = struct{}{}
	}
	if len(added) == 0 {
		return
	}
	st.AgentAttachments = append(st.AgentAttachments, added...)
	c.logger.Debug("agent attachments added", zap.Int("count", len(added)))
}

// inspectComposerAttachment checks a picker path against the same limits
// the engine applies on send. reason is "", "size", or "type".
func inspectComposerAttachment(raw string) (abs, mime, name, reason string) {
	abs = strings.TrimSpace(raw)
	if abs == "" {
		return "", "", "", "type"
	}
	var err error
	abs, err = filepath.Abs(abs)
	if err != nil {
		return "", "", "", "type"
	}
	abs = filepath.Clean(abs)
	mime, ok := mimeFromAttachExt(filepath.Ext(abs))
	if !ok {
		return "", "", "", "type"
	}
	st, err := os.Stat(abs)
	if err != nil || !st.Mode().IsRegular() || st.Size() == 0 {
		return "", "", "", "type"
	}
	if st.Size() > ai.MaxAttachBytes {
		return "", "", "", "size"
	}
	name = filepath.Base(abs)
	return abs, mime, name, ""
}

func mimeFromAttachExt(ext string) (string, bool) {
	switch strings.ToLower(ext) {
	case ".png":
		return "image/png", true
	case ".jpg", ".jpeg":
		return "image/jpeg", true
	case ".webp":
		return "image/webp", true
	case ".gif":
		return "image/gif", true
	case ".pdf":
		return "application/pdf", true
	case ".md", ".markdown":
		return "text/markdown", true
	case ".csv":
		return "text/csv", true
	case ".txt":
		return "text/plain", true
	default:
		return "", false
	}
}
