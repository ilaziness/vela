package main

import (
	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend"
)

type appState struct {
	app    *gui.App
	input1 string
	input2 string
	input3 string
}

func main() {
	gui.SetTheme(gui.ThemeDark)

	app := gui.NewApp()
	st := &appState{app: app}

	win := gui.NewWindow(gui.WindowCfg{
		Title:  "window focus repro",
		Width:  560,
		Height: 420,
		State:  st,
		OnInit: func(w *gui.Window) {
			w.SetView(mainView)
		},
	})

	backend.RunApp(app, win)
}

func mainView(w *gui.Window) gui.View {
	st := gui.State[appState](w)
	return gui.Column(gui.ContainerCfg{
		Sizing: gui.FillFill,
		Content: []gui.View{
			gui.Text(gui.TextCfg{Text: "Hello 你好 こんにちは 안녕하세요  مرحبا mixed 文本 123"}),
			gui.Text(gui.TextCfg{Text: "focus: " + w.FocusID()}),
			gui.Input(gui.InputCfg{
				ID:          "input-1",
				Sizing:      gui.FillFit,
				Placeholder: "input 1",
				Text:        st.input1,
				OnTextChanged: func(text string, ctx gui.EventCtx) {
					gui.State[appState](ctx.Window).input1 = text
				},
			}),
			gui.Input(gui.InputCfg{
				ID:          "input-2",
				Sizing:      gui.FillFit,
				Placeholder: "input 2",
				Text:        st.input2,
				OnTextChanged: func(text string, ctx gui.EventCtx) {
					gui.State[appState](ctx.Window).input2 = text
				},
			}),
			gui.Input(gui.InputCfg{
				ID:          "input-3",
				Sizing:      gui.FillFit,
				Placeholder: "input 3",
				Text:        st.input3,
				OnTextChanged: func(text string, ctx gui.EventCtx) {
					gui.State[appState](ctx.Window).input3 = text
				},
			}),
			gui.Button(gui.ButtonCfg{
				ID:    "open-child",
				Label: "Open child",
				OnClick: func(ctx gui.EventCtx) {
					ctx.Consume()
					openChild(st)
				},
			}),
		},
	})
}

func openChild(st *appState) {
	st.app.OpenWindow(gui.WindowCfg{
		Title:  "child window",
		Width:  360,
		Height: 240,
		OnInit: func(w *gui.Window) {
			w.SetView(childView)
		},
	})
}

func childView(w *gui.Window) gui.View {
	return gui.Text(gui.TextCfg{Text: "child window sample text"})
}
