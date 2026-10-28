// Package usererr defines the RPC user-visible error contract:
// stable code + params, with an English message fallback for clients
// that cannot translate yet.
package usererr

import (
	"encoding/json"
	"errors"
	"fmt"
)

// Payload is the JSON shape returned via error.Error() for Wails RPC.
type Payload struct {
	Code    string   `json:"code"`
	Params  []string `json:"params,omitempty"`
	Message string   `json:"message"`
}

// Error is a user-facing RPC error. Error() returns JSON so the frontend
// can parse code/params and translate locally.
type Error struct {
	Code    string
	Params  []string
	Message string
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	b, err := json.Marshal(Payload{
		Code:    e.Code,
		Params:  e.Params,
		Message: e.Message,
	})
	if err != nil {
		return e.Message
	}
	return string(b)
}

// New builds a coded user error. message should be the English fallback.
func New(code, message string, params ...string) *Error {
	return &Error{Code: code, Message: message, Params: params}
}

// Newf is New with fmt.Sprintf for the English message.
func Newf(code, messageFmt string, args ...any) *Error {
	return &Error{Code: code, Message: fmt.Sprintf(messageFmt, args...)}
}

// As extracts *Error from err's chain.
func As(err error) (*Error, bool) {
	var ue *Error
	if errors.As(err, &ue) {
		return ue, true
	}
	return nil, false
}

// CodeOf returns the user error code if present.
func CodeOf(err error) string {
	if ue, ok := As(err); ok {
		return ue.Code
	}
	return ""
}

// Parse tries to decode a Payload from a string (e.g. err.Error() or FE message).
func Parse(s string) (Payload, bool) {
	var p Payload
	if err := json.Unmarshal([]byte(s), &p); err != nil {
		return Payload{}, false
	}
	if p.Code == "" && p.Message == "" {
		return Payload{}, false
	}
	return p, true
}
