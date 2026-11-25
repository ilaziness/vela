package errtext

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"testing"

	"go.uber.org/zap"

	"github.com/ilaziness/vela/internal/applog"
	"github.com/ilaziness/vela/internal/i18n"
	"github.com/ilaziness/vela/internal/usererr"
)

func TestMain(m *testing.M) {
	// Render-only tests must not create the process log file.
	applog.SetDefault(zap.NewNop())
	os.Exit(m.Run())
}

func TestTextNil(t *testing.T) {
	if got := Text(nil); got != "" {
		t.Fatalf("Text(nil) = %q, want empty", got)
	}
}

func TestTextPlainError(t *testing.T) {
	if got := Text(errors.New("plain failure")); got != "plain failure" {
		t.Fatalf("Text(plain) = %q", got)
	}
}

func TestTextStorageError(t *testing.T) {
	i18n.SetLanguage(i18n.LangEn)
	err := fmt.Errorf("insert bookmark failed: %w", sql.ErrNoRows)
	if got, want := Text(err), i18n.T(usererr.StorageErrorCode); got != want {
		t.Fatalf("Text(storage) = %q, want %q", got, want)
	}
}

func TestTextUserErrMessage(t *testing.T) {
	ue := usererr.New("", "fallback message")
	if got := Text(ue); got != "fallback message" {
		t.Fatalf("Text(usererr) = %q", got)
	}
}

func TestTextUserErrPrecedence(t *testing.T) {
	// A coded user error must render its i18n message even when wrapped
	// inside another error, and never degrade to the DB fallback.
	i18n.SetLanguage(i18n.LangEn)
	wrapped := fmt.Errorf("outer: %w", usererr.New("session.not_found", "Session not found"))
	if got, want := Text(wrapped), "Session not found"; got != want {
		t.Fatalf("Text(wrapped usererr) = %q, want %q", got, want)
	}
}
