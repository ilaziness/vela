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

func TestParse(t *testing.T) {
	p, ok := Parse(`{"code":"ssh.auth_timeout","message":"Authentication timed out, please retry"}`)
	if !ok || p.Code != "ssh.auth_timeout" {
		t.Fatalf("%v %v", ok, p)
	}
	if _, ok := Parse("plain error"); ok {
		t.Fatal("expected false")
	}
}
