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

// Wrap returns err if it is already a coded user error; otherwise Coded with
// params followed by err.Error() as the last sprintf argument.
func Wrap(err error, code, messageFmt string, params ...string) error {
	if err == nil {
		return nil
	}
	if _, ok := As(err); ok {
		return err
	}
	all := make([]string, 0, len(params)+1)
	all = append(all, params...)
	all = append(all, err.Error())
	ue := Coded(code, messageFmt, all...)
	ue.cause = err
	return ue
}
