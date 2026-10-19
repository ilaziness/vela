package services

import (
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

type Windows struct {
	app       *application.App
	Main      *application.WebviewWindow
	Setting   *application.WebviewWindow
	Command   *application.WebviewWindow
	Tool      *application.WebviewWindow
	frameless bool
}

func NewWindows(app *application.App, main *application.WebviewWindow, frameless bool) *Windows {
	w := &Windows{app: app, Main: main, frameless: frameless}
	RegisterFileDropHandler(app, main)
	main.OnWindowEvent(events.Common.WindowClosing, func(event *application.WindowEvent) {
		for _, window := range app.Window.GetAll() {
			window.Close()
		}
	})
	return w
}

// IsFrameless reports the frame mode chosen when this process created its windows.
func (w *Windows) IsFrameless() bool {
	return w.frameless
}

func (w *Windows) newSettingWindow() *application.WebviewWindow {
	return w.app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title: "设置", URL: "/#/setting", Width: 1200, Height: 800,
		MinWidth: 1200, MinHeight: 800, DefaultContextMenuDisabled: true, Hidden: true, Frameless: w.frameless,
	})
}

func (w *Windows) newCommandWindow() *application.WebviewWindow {
	return w.app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title: "命令面板", URL: "/#/command", Width: 1600, Height: 900,
		MinWidth: 1600, MinHeight: 900, DefaultContextMenuDisabled: true, Hidden: true, Frameless: w.frameless,
	})
}

func (w *Windows) newToolWindow() *application.WebviewWindow {
	return w.app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title: "运维工具", URL: "/#/tools", Width: 1200, Height: 800,
		MinWidth: 800, MinHeight: 600, DefaultContextMenuDisabled: true, Hidden: true, Frameless: w.frameless,
	})
}

func (w *Windows) NewMainWindow() {
	win := w.app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title: "Vela", URL: "/#/submainwindow", Width: 1440, Height: 800,
		MinWidth: 1440, MinHeight: 800, DefaultContextMenuDisabled: true, Frameless: w.frameless, EnableFileDrop: true,
	})
	RegisterFileDropHandler(w.app, win)
	win.Show()
	win.Focus()
}

func (w *Windows) ShowSetting() {
	if w.Setting == nil {
		w.Setting = w.newSettingWindow()
		w.Setting.OnWindowEvent(events.Common.WindowClosing, func(event *application.WindowEvent) {
			w.Setting = nil
		})
	}
	w.Setting.Show()
	w.Setting.Focus()
}

func (w *Windows) CloseSetting() {
	if w.Setting != nil {
		w.Setting.Close()
		w.Setting = nil
	}
}

func (w *Windows) ToggleSetting() {
	if w.Setting != nil && w.Setting.IsVisible() {
		w.CloseSetting()
		return
	}
	w.ShowSetting()
}

func (w *Windows) ShowCommand() {
	if w.Command == nil {
		w.Command = w.newCommandWindow()
		w.Command.OnWindowEvent(events.Common.WindowClosing, func(event *application.WindowEvent) {
			w.Command = nil
		})
	}
	w.Command.Show()
	w.Command.Focus()
}

func (w *Windows) CloseCommand() {
	if w.Command != nil {
		w.Command.Close()
		w.Command = nil
	}
}

func (w *Windows) ToggleCommand() {
	if w.Command != nil && w.Command.IsVisible() {
		w.CloseCommand()
		return
	}
	w.ShowCommand()
}

func (w *Windows) ShowTool() {
	if w.Tool == nil {
		w.Tool = w.newToolWindow()
		w.Tool.OnWindowEvent(events.Common.WindowClosing, func(event *application.WindowEvent) {
			w.Tool = nil
		})
	}
	w.Tool.Show()
	w.Tool.Focus()
}

func (w *Windows) CloseTool() {
	if w.Tool != nil {
		w.Tool.Close()
		w.Tool = nil
	}
}

func (w *Windows) ToggleTool() {
	if w.Tool != nil && w.Tool.IsVisible() {
		w.CloseTool()
		return
	}
	w.ShowTool()
}
