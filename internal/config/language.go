package config

import (
	"strings"

	"go.uber.org/zap"

	"github.com/ilaziness/vela/internal/i18n"
)

// BootstrapLanguage detects and persists language on first launch, then applies it.
// Call after Load. Kept here so internal/i18n stays a leaf (no config import).
func BootstrapLanguage(store *Store, log *zap.Logger) {
	if store == nil || store.Config == nil {
		return
	}
	if log == nil {
		log = zap.NewNop()
	}
	stored := strings.TrimSpace(store.Config.General.Language)
	detected := i18n.DetectSystem()
	lang := stored
	if lang == "" {
		lang = detected
	}
	lang = i18n.Normalize(lang)
	log.Debug("bootstrap language",
		zap.String("stored", stored),
		zap.String("detected", detected),
		zap.String("applied", lang),
	)
	if lang != stored {
		store.Config.General.Language = lang
		if err := store.Save(); err != nil {
			log.Error("persist language failed", zap.Error(err))
		} else {
			log.Debug("persisted language", zap.String("language", lang))
		}
	}
	i18n.SetLanguage(lang)
}
