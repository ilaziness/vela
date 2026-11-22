package main

import (
	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend"
)

var app *gui.App

func main() {
	gui.SetTheme(gui.ThemeDark)

	app = gui.NewApp()

	win := gui.NewWindow(gui.WindowCfg{
		Title:  "zoom issue repro",
		Width:  480,
		Height: 360,
		OnInit: func(w *gui.Window) {
			w.SetView(mainView)
		},
	})

	backend.RunApp(app, win)
}

func mainView(w *gui.Window) gui.View {
	th := gui.CurrentTheme()
	return gui.Column(gui.ContainerCfg{
		Sizing:  gui.FillFill,
		Padding: gui.PadAll(16),
		Content: []gui.View{
			gui.Button(gui.ButtonCfg{
				ID:      "open-child",
				Variant: gui.ButtonGhost,
				Width:   36,
				Height:  36,
				A11YCfg: gui.A11YCfg{A11YLabel: "open child window"},
				Content: []gui.View{
					gui.Text(gui.TextCfg{Text: gui.IconAppMenu, TextStyle: th.TextStyleIconMedium}),
				},
				OnClick: func(ctx gui.EventCtx) {
					ctx.Consume()
					openChildWindow()
				},
			}),
			gui.Button(gui.ButtonCfg{
				ID:      "open-text",
				Variant: gui.ButtonGhost,
				Width:   36,
				Height:  36,
				A11YCfg: gui.A11YCfg{A11YLabel: "open text window"},
				Content: []gui.View{
					gui.Text(gui.TextCfg{Text: gui.IconFile, TextStyle: th.TextStyleIconMedium}),
				},
				OnClick: func(ctx gui.EventCtx) {
					ctx.Consume()
					openTextWindow()
				},
			}),
		},
	})
}

func openChildWindow() {
	app.OpenWindow(gui.WindowCfg{
		Title:  "child window",
		Width:  520,
		Height: 400,
		OnInit: func(w *gui.Window) {
			w.SetView(childView)
		},
	})
}

func openTextWindow() {
	app.OpenWindow(gui.WindowCfg{
		Title:  "text window",
		Width:  360,
		Height: 240,
		OnInit: func(w *gui.Window) {
			w.SetView(textView)
		},
	})
}

func textView(w *gui.Window) gui.View {
	th := gui.CurrentTheme()
	return gui.Column(gui.ContainerCfg{
		Sizing:  gui.FillFill,
		Padding: gui.PadAll(16),
		Spacing: gui.SpacingMedium,
		Content: []gui.View{
			gui.Text(gui.TextCfg{Text: "plain text sample", TextStyle: th.TextStyleBody}),
			gui.Text(gui.TextCfg{Text: "second line of text", TextStyle: th.TextStyleSecondary}),
		},
	})
}

// childView puts the earlier menu-item list on the left and the
// grab-bag of stock widgets on the right.
func childView(w *gui.Window) gui.View {
	th := gui.CurrentTheme()

	menuItems := []string{"Item 1", "Item 2", "Item 3", "Item 4", "Item 5"}
	menuContent := make([]gui.View, 0, len(menuItems))
	for _, label := range menuItems {
		menuContent = append(menuContent, gui.Button(gui.ButtonCfg{
			ID:            "menu-item-" + label,
			Variant:       gui.ButtonGhost,
			Sizing:        gui.FillFit,
			A11YCfg:       gui.A11YCfg{A11YLabel: label},
			HAlign:        gui.Some(gui.HAlignLeft),
			Label:         label,
			FocusDisabled: true,
			OnClick: func(ctx gui.EventCtx) {
				ctx.Consume()
			},
		}))
	}
	menu := gui.Column(gui.ContainerCfg{
		Sizing:  gui.FixedFill,
		Width:   160,
		Color:   th.ColorPanel,
		Padding: gui.PadAll(8),
		Spacing: gui.NoSpacing,
		Content: menuContent,
	})

	body := gui.Column(gui.ContainerCfg{
		Sizing:  gui.FillFill,
		Padding: gui.PadAll(16),
		Spacing: gui.SpacingMedium,
		Content: []gui.View{
			gui.Text(gui.TextCfg{Text: "child window", TextStyle: th.TextStyleTitle}),
			gui.Input(gui.InputCfg{
				ID:          "child-input",
				Sizing:      gui.FillFit,
				Placeholder: "input field",
			}),
			gui.Checkbox(gui.ToggleCfg{
				ID:       "child-check",
				Label:    "checkbox",
				Selected: true,
			}),
			gui.Select(gui.SelectCfg{
				ID:          "child-select",
				Sizing:      gui.FillFit,
				Placeholder: "select one",
				Items:       []string{"alpha", "beta", "gamma"},
			}),
			gui.Slider(gui.SliderCfg{
				ID:    "child-slider",
				Min:   0,
				Max:   100,
				Value: 40,
			}),
			gui.Button(gui.ButtonCfg{
				ID:      "child-btn",
				Variant: gui.ButtonSecondary,
				Label:   "a button",
				OnClick: func(ctx gui.EventCtx) {
					ctx.Consume()
				},
			}),
		},
	})

	return gui.Row(gui.ContainerCfg{
		Sizing:  gui.FillFill,
		Spacing: gui.NoSpacing,
		Content: []gui.View{menu, body},
	})
}
