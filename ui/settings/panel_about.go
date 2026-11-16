package settings

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/go-gui-org/go-gui/gui"

	"github.com/ilaziness/vela/internal/buildinfo"
	"github.com/ilaziness/vela/internal/i18n"
	updateui "github.com/ilaziness/vela/ui/update"
)

const githubURL = "https://github.com/ilaziness/vela"

func panelAbout(d *Deps, st *State) gui.View {
	th := gui.CurrentTheme()
	versionText := i18n.Tf("settings.about.version", buildinfo.Version)
	var meta []string
	if built := strings.TrimSpace(buildinfo.BuildTime); built != "" {
		if len(built) >= 10 {
			built = built[:10]
		}
		meta = append(meta, built)
	}
	if git := strings.TrimSpace(buildinfo.GitInfo); git != "" {
		meta = append(meta, git)
	}
	if len(meta) > 0 {
		versionText += " · " + strings.Join(meta, " · ")
	}

	rows := []gui.View{
		gui.Row(gui.ContainerCfg{
			Sizing:     gui.FillFit,
			HAlign:     gui.HAlignStart,
			VAlign:     gui.VAlignMiddle,
			Spacing:    gui.SpacingSmall,
			SizeBorder: gui.NoBorder,
			Content: []gui.View{
				gui.Text(gui.TextCfg{Text: "Vela", TextStyle: th.TextStyleTitle}),
				gui.Button(gui.ButtonCfg{
					ID:      "settings-about-github",
					Variant: gui.ButtonGhost,
					Padding: gui.PaddingXSmall,
					A11YCfg: gui.A11YCfg{A11YLabel: i18n.T("settings.about.github")},
					Content: []gui.View{
						gui.Text(gui.TextCfg{Text: gui.IconGithub, TextStyle: th.TextStyleIconLarge}),
					},
					OnClick: func(ctx gui.EventCtx) {
						ctx.Consume()
						if err := updateui.OpenHTTPURL(ctx.Window, githubURL); err != nil {
							toastInfo(ctx.Window, i18n.T("settings.about.open_failed"))
						}
					},
				}),
			},
		}),
		gui.Label(i18n.T("settings.about.tagline"), th.TextStyleSecondary),
		gui.Label(versionText, th.TextStyleBody),
		gui.Label(i18n.T("settings.about.description"), th.TextStyleBody),
		gui.Label(i18n.Tf("settings.about.copyright", strconv.Itoa(time.Now().Year())), th.TextStyleSecondary),
	}

	checkLabel := i18n.T("settings.about.check_update")
	if st.UpdateChecking {
		checkLabel = i18n.T("settings.about.update_checking")
	}
	rows = append(rows, gui.Button(gui.ButtonCfg{
		ID:       "settings-about-update",
		Variant:  gui.ButtonSecondary,
		Label:    checkLabel,
		Disabled: st.UpdateChecking,
		OnClick: func(ctx gui.EventCtx) {
			ctx.Consume()
			if d.Update == nil || d.Update.Manager == nil {
				toastInfo(ctx.Window, i18n.T("settings.about.update_unavailable"))
				return
			}
			status := d.Update.Manager.InstallStatus()
			if d.Update.Manager.Latest() != nil || status.Busy() {
				updateui.Show(ctx.Window, d.Update)
				return
			}
			s := gui.State[State](ctx.Window)
			s.UpdateChecking = true
			ctx.Window.InvalidateLayout()
			go func() {
				rel, err := d.Update.Manager.Check(context.Background())
				ctx.Window.QueueCommand(func(w *gui.Window) {
					s2 := gui.State[State](w)
					s2.UpdateChecking = false
					switch {
					case err != nil:
						toastInfo(w, i18n.T("settings.about.update_failed"))
					case rel != nil:
						updateui.Show(w, d.Update)
					default:
						toastInfo(w, i18n.T("settings.about.up_to_date"))
					}
					w.InvalidateLayout()
				})
			}()
		},
	}))
	return panelColumn(i18n.T("settings.about.title"), rows)
}
