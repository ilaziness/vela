package main

import (
	"log"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

const sampleText = `Hello, World! 你好，世界！ 🌍✨
English 中文 emoji 🚀🎉 漢字繁體
Third line: café naïve résumé — ﬁ ﬂ ligatures
第四行：混合 Mixed 12345 テスト
Ωμέγα δελτα Σίγμα — façade jalapeño
Last line: 你好 👋🏽 goodbye 🌙`

type app struct{}

func (a *app) view(c *ui.Context) {
	c.SetTheme(ui.DarkTheme())
	ui.Column(c).Fill().Center().Gap(12).Children(func() {
		ui.Text(c, sampleText).FillWidth().FontSize(14).TextAlign(ui.Center).TextColor(ui.RGB(255, 255, 255))
	})
}

func main() {
	app := &app{}
	mygo.App.WhenReady(func() {
		mygo.NewWindow(mygo.WindowOptions{
			Title:   "mygo text example",
			Width:   640,
			Height:  400,
			Content: ui.View(app.view),
		})
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
