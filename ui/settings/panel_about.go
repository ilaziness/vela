package settings

import (
	"strconv"
	"time"

	"github.com/go-gui-org/go-gui/gui"

	"github.com/ilaziness/vela/internal/buildinfo"
	"github.com/ilaziness/vela/internal/i18n"
)

func panelAbout() gui.View {
	rows := []gui.View{
		gui.Text(gui.TextCfg{Text: "Vela", TextStyle: gui.CurrentTheme().TextStyleTitle}),
		gui.Label(i18n.T("settings.about.tagline"), gui.CurrentTheme().TextStyleSecondary),
		gui.Label(i18n.Tf("settings.about.version", buildinfo.Version), gui.CurrentTheme().TextStyleBody),
	}
	if buildinfo.GitInfo != "" {
		rows = append(rows, gui.Label(i18n.Tf("settings.about.git", buildinfo.GitInfo), gui.CurrentTheme().TextStyleSecondary))
	}
	if buildinfo.BuildTime != "" {
		rows = append(rows, gui.Label(i18n.Tf("settings.about.build_time", buildinfo.BuildTime), gui.CurrentTheme().TextStyleSecondary))
	}
	rows = append(rows,
		gui.Label(i18n.T("settings.about.description"), gui.CurrentTheme().TextStyleBody),
		gui.Label(i18n.Tf("settings.about.copyright", strconv.Itoa(time.Now().Year())), gui.CurrentTheme().TextStyleSecondary),
		gui.Button(gui.ButtonCfg{
			ID:      "settings-about-update",
			Variant: gui.ButtonSecondary,
			Label:   i18n.T("settings.about.check_update"),
			OnClick: func(ctx gui.EventCtx) {
				ctx.Consume()
				toastInfo(ctx.Window, i18n.T("settings.about.update_unavailable"))
			},
		}),
	)
	return panelColumn(i18n.T("settings.about.title"), rows)
}
