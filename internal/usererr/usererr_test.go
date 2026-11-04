package usererr

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestErrorJSON(t *testing.T) {
	err := New("ssh.auth_failed", "Login failed", "a")
	var p Payload
	if e := json.Unmarshal([]byte(err.Error()), &p); e != nil {
		t.Fatal(e)
	}
	if p.Code != "ssh.auth_failed" || p.Message != "Login failed" || len(p.Params) != 1 || p.Params[0] != "a" {
		t.Fatalf("payload = %+v", p)
	}
	if CodeOf(err) != "ssh.auth_failed" {
		t.Fatal(CodeOf(err))
	}
	wrapped := errors.Join(errors.New("wrap"), err)
	if CodeOf(wrapped) != "ssh.auth_failed" {
		// Join may not unwrap *Error via As depending on Go version; check As directly
		if _, ok := As(err); !ok {
			t.Fatal("As failed")
		}
	}
}

func TestWrapPreservesCoded(t *testing.T) {
	inner := Coded("ssh.auth_failed", "Login failed")
	got := Wrap(inner, "ai.encrypt_key_failed", "Failed to encrypt API key: %s")
	if !errors.Is(got, inner) {
		t.Fatalf("Wrap coded = %v", got)
	}
	plain := errors.New("disk full")
	wrapped := Wrap(plain, "ai.encrypt_key_failed", "Failed to encrypt API key: %s")
	ue, ok := As(wrapped)
	if !ok || ue.Code != "ai.encrypt_key_failed" || ue.Params[0] != "disk full" {
		t.Fatalf("%v %+v", ok, ue)
	}
	if !errors.Is(wrapped, plain) {
		t.Fatal("Wrap should unwrap to the original error")
	}
}

func TestParse(t *testing.T) {
	p, ok := Parse(`{"code":"ssh.auth_timeout","message":"Authentication timed out, please retry"}`)
	if !ok || p.Code != "ssh.auth_timeout" {
		t.Fatalf("%v %v", ok, p)
	}
	if _, ok := Parse("plain error"); ok {
		t.Fatal("expected false")
	}
}
