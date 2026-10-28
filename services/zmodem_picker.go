package services

import (
	"context"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/ilaziness/vela/internal/i18n"
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
		SetTitle(i18n.T("dialog.zmodem_save_dir")).
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
		SetTitle(i18n.T("dialog.zmodem_select_upload")).
		PromptForSingleSelection()
	if dialogCancelled(err) || path == "" {
		return "", zmodem.ErrCancelled
	}
	if err != nil {
		return "", err
	}
	return path, nil
}
