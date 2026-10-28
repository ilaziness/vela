package services

import (
	"errors"
	"fmt"
	"testing"

	"github.com/ilaziness/vela/internal/ssh"
	"github.com/ilaziness/vela/internal/usererr"
)

func TestMapSSHUserError(t *testing.T) {
	if mapSSHUserError(nil) != nil {
		t.Fatal("nil")
	}
	err := mapSSHUserError(ssh.ErrAuthCancelled)
	if usererr.CodeOf(err) != "ssh.auth_cancelled" {
		t.Fatalf("got %v", err)
	}
	err = mapSSHUserError(ssh.ErrAuthTimeout)
	if usererr.CodeOf(err) != "ssh.auth_timeout" {
		t.Fatalf("got %v", err)
	}
	err = mapSSHUserError(ssh.ErrNoAuthMethods)
	if usererr.CodeOf(err) != "ssh.auth_failed" {
		t.Fatalf("got %v", err)
	}
	err = mapSSHUserError(errors.New("ssh: handshake failed: unable to authenticate, attempted methods [none], no supported methods remain"))
	if usererr.CodeOf(err) != "ssh.auth_failed" {
		t.Fatalf("got %v", err)
	}
	err = mapSSHUserError(fmt.Errorf("%w: abc", ssh.ErrSessionNotFound))
	if usererr.CodeOf(err) != "ssh.connection_not_found" {
		t.Fatalf("got %v", err)
	}
	plain := errors.New("dial tcp: connection refused")
	got := mapSSHUserError(plain)
	if !errors.Is(got, plain) || usererr.CodeOf(got) != "" {
		t.Fatalf("unexpected remap: %v", got)
	}
}
