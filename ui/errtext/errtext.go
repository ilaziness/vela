// Package errtext renders errors for native-UI toasts and inline labels.
package errtext

import (
	"go.uber.org/zap"

	"github.com/ilaziness/vela/internal/applog"
	"github.com/ilaziness/vela/internal/database"
	"github.com/ilaziness/vela/internal/i18n"
	"github.com/ilaziness/vela/internal/usererr"
)

// Text formats err for display. usererr coded errors are translated via
// i18n; storage-layer failures are logged with full detail and replaced by
// a generic message; everything else falls back to err.Error().
func Text(err error) string {
	if err == nil {
		return ""
	}
	if ue, ok := usererr.As(err); ok {
		if ue.Code != "" {
			if len(ue.Params) == 0 {
				return i18n.T(ue.Code)
			}
			args := make([]any, len(ue.Params))
			for i, p := range ue.Params {
				args[i] = p
			}
			return i18n.Tf(ue.Code, args...)
		}
		return ue.Message
	}
	if database.IsStorageError(err) {
		applog.L().Warn("database error surfaced to UI", zap.Error(err))
		return i18n.T(usererr.StorageErrorCode)
	}
	return err.Error()
}
