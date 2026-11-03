package services

import (
	"context"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/ilaziness/vela/internal/config"
	"github.com/ilaziness/vela/internal/i18n"
	"github.com/ilaziness/vela/internal/zmodem"
)

type zmodemPicker struct {
	app   *application.App
	store *config.Store
}

func newZmodemPicker(app *application.App, store *config.Store) *zmodemPicker {
	return &zmodemPicker{app: app, store: store}
}

func (p *zmodemPicker) PickSaveDirectory(_ context.Context, _ string) (string, error) {
	path, err := promptOpenDirectory(p.app, p.store, i18n.T("dialog.zmodem_save_dir"))
	if dialogCancelled(err) || path == "" {
		return "", zmodem.ErrCancelled
	}
	if err != nil {
		return "", err
	}
	return path, nil
}

func (p *zmodemPicker) PickOpenFile(_ context.Context, _ string) (string, error) {
	path, err := promptOpenFile(p.app, p.store, i18n.T("dialog.zmodem_select_upload"))
	if dialogCancelled(err) || path == "" {
		return "", zmodem.ErrCancelled
	}
	if err != nil {
		return "", err
	}
	return path, nil
}
