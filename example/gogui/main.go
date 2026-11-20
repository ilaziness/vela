package main

import (
	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend"
)

type appState struct{}

const sampleText = `Hello, World! 你好，世界！ 🌍✨
English 中文 emoji 🚀🎉 漢字繁體
Third line: café naïve résumé — ﬁ ﬂ ligatures
第四行：混合 Mixed 12345 テスト
Ωμέγα δελτα Σίγμα — façade jalapeño
Last line: 你好 👋🏽 goodbye 🌙`

func main() {
	gui.SetTheme(gui.ThemeDark)
	app := gui.NewApp()
	win := gui.NewWindow(gui.WindowCfg{
		Title:  "go-gui text example",
		Width:  640,
		Height: 400,
		State:  &appState{},
		OnInit: func(w *gui.Window) {
			w.SetView(func(w *gui.Window) gui.View {
				th := gui.CurrentTheme()
				style := th.TextStyleBody
				style.Align = gui.TextAlignCenter
				style.Color = gui.RGB(255, 255, 255)
				return gui.Column(gui.ContainerCfg{
					Sizing: gui.FillFill,
					HAlign: gui.HAlignCenter,
					VAlign: gui.VAlignMiddle,
					Content: []gui.View{
						gui.Text(gui.TextCfg{
							Text:      sampleText,
							TextStyle: style,
							Sizing:    gui.FillFit,
						}),
					},
				})
			})
		},
	})
	backend.RunApp(app, win)
}
