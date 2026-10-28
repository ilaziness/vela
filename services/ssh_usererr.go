package services

import (
	"errors"
	"strings"

	"github.com/ilaziness/vela/internal/i18n"
	"github.com/ilaziness/vela/internal/ssh"
	"github.com/ilaziness/vela/internal/usererr"
)

// mapSSHUserError converts SSH engine failures into the RPC user-error contract.
func mapSSHUserError(err error) error {
	if err == nil {
		return nil
	}
	if _, ok := usererr.As(err); ok {
		return err
	}
	switch {
	case errors.Is(err, ssh.ErrAuthCancelled):
		return usererr.New("ssh.auth_cancelled", i18n.TFor(i18n.LangEn, "ssh.auth_cancelled"))
	case errors.Is(err, ssh.ErrAuthTimeout):
		return usererr.New("ssh.auth_timeout", i18n.TFor(i18n.LangEn, "ssh.auth_timeout"))
	case errors.Is(err, ssh.ErrNoAuthMethods):
		return usererr.New("ssh.auth_failed", i18n.TFor(i18n.LangEn, "ssh.auth_failed"))
	case errors.Is(err, ssh.ErrSessionNotFound):
		return usererr.New("ssh.connection_not_found", i18n.TFor(i18n.LangEn, "ssh.connection_not_found"))
	}

	msg := err.Error()
	lower := strings.ToLower(msg)
	if strings.Contains(lower, "unable to authenticate") ||
		strings.Contains(lower, "no supported methods remain") {
		return usererr.New("ssh.auth_failed", i18n.TFor(i18n.LangEn, "ssh.auth_failed"))
	}
	return err
}
