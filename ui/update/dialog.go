package update

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/go-gui-org/go-gui/gui"

	"github.com/ilaziness/vela/internal/i18n"
	"github.com/ilaziness/vela/internal/updater"
)

// Show displays the cached update dialog when a newer release is known.
func Show(w *gui.Window, d *Deps) {
	if w == nil || d == nil || d.Manager == nil {
		return
	}
	rel := d.Manager.Latest()
	if rel == nil {
		return
	}
	d.Prompted.Store(true)
	st := d.Manager.InstallStatus()
	showDialog(w, d, rel, st.Busy())
}

// NotifyAvailable is the background-check entry point; same as Show.
func NotifyAvailable(w *gui.Window, d *Deps) { Show(w, d) }

func showProgress(w *gui.Window, d *Deps, rel *updater.Release) {
	showDialog(w, d, rel, true)
}

func showDialog(w *gui.Window, d *Deps, rel *updater.Release, progress bool) {
	w.Dialog(gui.DialogCfg{
		Title:          i18n.T("update.title"),
		DialogType:     gui.DialogCustom,
		Width:          560,
		MaxHeight:      640,
		EscapeDisabled: progress,
		CustomView: func(vw *gui.Window) gui.View {
			return dialogBody(vw, d, rel, progress)
		},
	})
}

func dialogBody(w *gui.Window, d *Deps, rel *updater.Release, forceProgress bool) gui.View {
	st := d.Manager.InstallStatus()
	busy := forceProgress || st.Busy()
	if busy {
		return progressBody(w, d, rel, st)
	}
	return infoBody(w, d, rel)
}

func infoBody(w *gui.Window, d *Deps, rel *updater.Release) gui.View {
	th := gui.CurrentTheme()
	content := []gui.View{
		gui.Text(gui.TextCfg{Text: rel.Tag, TextStyle: th.TextStyleTitle}),
	}
	if strings.TrimSpace(rel.Notes) == "" {
		content = append(content, gui.Label(i18n.T("update.no_notes"), th.TextStyleSecondary))
	} else {
		content = append(content, gui.Column(gui.ContainerCfg{
			ID:         "update-notes",
			Sizing:     gui.FillFit,
			Scrollable: true,
			MaxHeight:  320,
			Padding:    gui.PaddingNone,
			SizeBorder: gui.NoBorder,
			Content: []gui.View{
				w.Markdown(gui.MarkdownCfg{
					ID:                  "update-notes-md",
					Source:              rel.Notes,
					DisableExternalAPIs: true,
				}),
			},
		}))
	}
	content = append(content, gui.Row(gui.ContainerCfg{
		Sizing:     gui.FillFit,
		HAlign:     gui.HAlignEnd,
		Spacing:    gui.SpacingMedium,
		SizeBorder: gui.NoBorder,
		Content: []gui.View{
			gui.Button(gui.ButtonCfg{
				ID:      "update-close",
				Variant: gui.ButtonSecondary,
				Label:   i18n.T("common.close"),
				OnClick: func(ctx gui.EventCtx) {
					ctx.Consume()
					ctx.Window.DialogDismiss()
				},
			}),
			gui.Button(gui.ButtonCfg{
				ID:      "update-open-page",
				Variant: gui.ButtonSecondary,
				Label:   i18n.T("update.open_download_page"),
				OnClick: func(ctx gui.EventCtx) {
					ctx.Consume()
					if err := OpenHTTPURL(ctx.Window, rel.URL); err != nil {
						toast(ctx.Window, gui.ToastError, i18n.T("update.open_failed"))
					}
				},
			}),
			gui.Button(gui.ButtonCfg{
				ID:       "update-install",
				Variant:  gui.ButtonPrimary,
				Label:    i18n.T("update.install"),
				Disabled: rel.Asset.URL == "",
				OnClick: func(ctx gui.EventCtx) {
					ctx.Consume()
					startInstall(ctx.Window, d, rel)
				},
			}),
		},
	}))
	return gui.Column(gui.ContainerCfg{
		ID:         "update-dialog",
		Sizing:     gui.FillFit,
		Spacing:    gui.SpacingMedium,
		Padding:    gui.PaddingSmall,
		SizeBorder: gui.NoBorder,
		Content:    content,
	})
}

func progressBody(w *gui.Window, d *Deps, rel *updater.Release, st updater.InstallStatus) gui.View {
	th := gui.CurrentTheme()
	phase := phaseLabel(st.Phase)
	pct := float32(0)
	indef := true
	if st.Phase == updater.PhaseDownload && st.Total > 0 {
		pct = float32(st.Written) / float32(st.Total)
		if pct > 1 {
			pct = 1
		}
		indef = false
	}
	statusText := phase
	if st.Phase == updater.PhaseDownload && st.Total > 0 {
		statusText = i18n.Tf("update.download_progress", formatBytes(st.Written), formatBytes(st.Total))
	}
	content := []gui.View{
		gui.Text(gui.TextCfg{Text: rel.Tag, TextStyle: th.TextStyleTitle}),
		gui.Label(statusText, th.TextStyleBody),
		gui.ProgressBar(gui.ProgressBarCfg{
			ID:         "update-progress",
			Percent:    pct,
			Indefinite: indef,
			Sizing:     gui.FillFit,
			TextShow:   false,
			MinHeight:  8,
			MaxHeight:  8,
		}),
	}
	if st.Err != nil {
		content = append(content, gui.Label(i18n.T("update.install_failed"), th.TextStyleSecondary))
	}
	canCancel := st.Phase == updater.PhaseDownload && st.Err == nil
	failed := st.Err != nil || st.Phase == updater.PhaseIdle
	content = append(content, gui.Row(gui.ContainerCfg{
		Sizing:     gui.FillFit,
		HAlign:     gui.HAlignEnd,
		Spacing:    gui.SpacingMedium,
		SizeBorder: gui.NoBorder,
		Content: []gui.View{
			gui.Button(gui.ButtonCfg{
				ID:       "update-progress-cancel",
				Variant:  gui.ButtonSecondary,
				Label:    i18n.T("common.cancel"),
				Disabled: !canCancel,
				OnClick: func(ctx gui.EventCtx) {
					ctx.Consume()
					d.Manager.CancelInstall()
				},
			}),
			gui.Button(gui.ButtonCfg{
				ID:       "update-progress-close",
				Variant:  gui.ButtonSecondary,
				Label:    i18n.T("common.close"),
				Disabled: !failed,
				OnClick: func(ctx gui.EventCtx) {
					ctx.Consume()
					ctx.Window.DialogDismiss()
				},
			}),
		},
	}))
	return gui.Column(gui.ContainerCfg{
		ID:         "update-progress-dialog",
		Sizing:     gui.FillFit,
		Spacing:    gui.SpacingMedium,
		Padding:    gui.PaddingSmall,
		SizeBorder: gui.NoBorder,
		Content:    content,
	})
}

func phaseLabel(p updater.Phase) string {
	switch p {
	case updater.PhaseDownload:
		return i18n.T("update.downloading")
	case updater.PhaseVerify:
		return i18n.T("update.verifying")
	case updater.PhaseInstall, updater.PhaseDone:
		return i18n.T("update.installing")
	default:
		return i18n.T("update.installing")
	}
}

func startInstall(w *gui.Window, d *Deps, rel *updater.Release) {
	go func() {
		var mu sync.Mutex
		var last time.Time
		progress := func(written, total int64) {
			now := time.Now()
			mu.Lock()
			if now.Sub(last) < 100*time.Millisecond {
				mu.Unlock()
				return
			}
			last = now
			mu.Unlock()
			w.QueueCommand(func(win *gui.Window) { win.InvalidateLayout() })
		}
		err := d.Manager.Install(context.Background(), rel, progress)
		if err != nil {
			w.QueueCommand(func(win *gui.Window) {
				if errors.Is(err, context.Canceled) {
					toast(win, gui.ToastInfo, i18n.T("update.canceled"))
				} else {
					toast(win, gui.ToastError, i18n.T("update.install_failed"))
				}
				win.DialogDismiss()
				showDialog(win, d, rel, false)
			})
			return
		}
		if d.OnInstalled != nil {
			d.OnInstalled()
		}
	}()
	w.DialogDismiss()
	showProgress(w, d, rel)
}

func toast(w *gui.Window, sev gui.ToastSeverity, body string) {
	if w == nil || body == "" {
		return
	}
	w.Toast(gui.ToastCfg{Body: body, Severity: sev, Duration: 4 * time.Second})
}

func formatBytes(n int64) string {
	if n < 0 {
		return ""
	}
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for x := n / unit; x >= unit; x /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGTPE"[exp])
}
