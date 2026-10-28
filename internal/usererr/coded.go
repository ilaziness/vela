package usererr

import "fmt"

// Coded builds a user-facing error with an English message fallback.
// params are both sprintf args for messageFmt and Payload.Params for FE translation.
func Coded(code, messageFmt string, params ...string) *Error {
	args := make([]any, len(params))
	for i, p := range params {
		args[i] = p
	}
	msg := messageFmt
	if len(args) > 0 {
		msg = fmt.Sprintf(messageFmt, args...)
	}
	return New(code, msg, params...)
}
