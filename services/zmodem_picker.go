package services

import (
	"context"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/ilaziness/vela/internal/zmodem"
)

type zmodemPicker struct {
	app *application.App
}

func newZmodemPicker(app *application.App) *zmodemPicker {
	return &zmodemPicker{app: app}
}

func (p *zmodemPicker) PickSaveDirectory(_ context.Context, _ string) (string, error) {
	path, err := p.app.Dialog.OpenFile().
		SetTitle("Zmodem 保存目录").
		CanChooseDirectories(true).
		CanChooseFiles(false).
		CanCreateDirectories(true).
		PromptForSingleSelection()
	if dialogCancelled(err) || path == "" {
		return "", zmodem.ErrCancelled
	}
	if err != nil {
		return "", err
	}
	return path, nil
}

func (p *zmodemPicker) PickOpenFile(_ context.Context, _ string) (string, error) {
	path, err := p.app.Dialog.OpenFile().
		SetTitle("Zmodem 选择要上传的文件").
		PromptForSingleSelection()
	if dialogCancelled(err) || path == "" {
		return "", zmodem.ErrCancelled
	}
	if err != nil {
		return "", err
	}
	return path, nil
}
