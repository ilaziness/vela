package ssh

import (
	"errors"
	"fmt"
	"testing"

	"github.com/ilaziness/vela/internal/usererr"
)

func TestMapUserError(t *testing.T) {
	if MapUserError(nil) != nil {
		t.Fatal("nil")
	}
	err := MapUserError(ErrAuthCancelled)
	if usererr.CodeOf(err) != "ssh.auth_cancelled" {
		t.Fatalf("got %v", err)
	}
	err = MapUserError(ErrAuthTimeout)
	if usererr.CodeOf(err) != "ssh.auth_timeout" {
		t.Fatalf("got %v", err)
	}
	err = MapUserError(ErrNoAuthMethods)
	if usererr.CodeOf(err) != "ssh.auth_failed" {
		t.Fatalf("got %v", err)
	}
	err = MapUserError(errors.New("ssh: handshake failed: unable to authenticate, attempted methods [none], no supported methods remain"))
	if usererr.CodeOf(err) != "ssh.auth_failed" {
		t.Fatalf("got %v", err)
	}
	err = MapUserError(fmt.Errorf("%w: abc", ErrSessionNotFound))
	if usererr.CodeOf(err) != "ssh.connection_not_found" {
		t.Fatalf("got %v", err)
	}
	plain := errors.New("dial tcp: connection refused")
	got := MapUserError(plain)
	if !errors.Is(got, plain) || usererr.CodeOf(got) != "" {
		t.Fatalf("unexpected remap: %v", got)
	}
}
