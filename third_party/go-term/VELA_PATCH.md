# Vela patch

Based on upstream `github.com/go-gui-org/go-term` **v0.17.1**.

Local addition (only):

- `term/stream.go` — exports `Stream` and `NewWithStream` so remote sessions
  (SSH/serial) can drive the emulator without a local PTY.

When upgrading, re-copy upstream and re-apply `term/stream.go`.
