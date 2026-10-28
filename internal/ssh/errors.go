package ssh

import "errors"

// Stable sentinels for user-visible SSH failures. Services map these to usererr codes.
var (
	ErrAuthCancelled = errors.New("ssh: auth cancelled")
	ErrAuthTimeout   = errors.New("ssh: auth timeout")
	ErrNoAuthMethods = errors.New("ssh: no auth methods")
)
