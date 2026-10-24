package zmodem

import (
	"bytes"
	"context"
	"testing"
	"time"
)

func TestFindHexAutostartZRQINIT(t *testing.T) {
	frame := []byte{0x2a, 0x2a, 0x18, 0x42, '0', '0', '0', '0'}
	data := append([]byte("hello"), frame...)
	det, ok := findHexAutostart(data)
	if !ok {
		t.Fatal("expected detect")
	}
	if det.index != 5 {
		t.Fatalf("index=%d", det.index)
	}
	if det.dir != DirectionReceive {
		t.Fatalf("dir=%v", det.dir)
	}
}

func TestFindHexAutostartZRINIT(t *testing.T) {
	frame := []byte{0x2a, 0x2a, 0x18, 0x42, '0', '1', '0', '0'}
	det, ok := findHexAutostart(frame)
	if !ok || det.dir != DirectionSend {
		t.Fatalf("got ok=%v dir=%v", ok, det.dir)
	}
}

func TestFindHexAutostartIgnoresOtherTypes(t *testing.T) {
	frame := []byte{0x2a, 0x2a, 0x18, 0x42, '0', '4', '0', '0'}
	if _, ok := findHexAutostart(frame); ok {
		t.Fatal("should ignore ZFILE as autostart")
	}
}

func TestFindHexAutostartSingleCANNotEnough(t *testing.T) {
	if _, ok := findHexAutostart([]byte{0x18, 'B', '0', '0'}); ok {
		t.Fatal("single CAN must not trigger")
	}
}

func TestFeedOutPassthroughIdle(t *testing.T) {
	f := NewFilter("s1", Deps{Picker: &stubPicker{}})
	in := []byte("hello top output\r\n")
	out := f.FeedOut(in)
	if !bytes.Equal(out, in) {
		t.Fatalf("idle must passthrough exactly: got %q", out)
	}
}

func TestFeedOutStarEchoesImmediately(t *testing.T) {
	f := NewFilter("s1", Deps{Picker: &stubPicker{}})
	out := f.FeedOut([]byte("*"))
	if string(out) != "*" {
		t.Fatalf("single * must echo immediately, got %q", out)
	}
	out = f.FeedOut([]byte("**"))
	if string(out) != "**" {
		t.Fatalf("** must echo immediately, got %q", out)
	}
}

func TestFeedOutNilPickerPassthrough(t *testing.T) {
	f := NewFilter("s1", Deps{})
	in := []byte{0x2a, 0x2a, 0x18, 0x42, '0', '0'}
	out := f.FeedOut(in)
	if !bytes.Equal(out, in) {
		t.Fatal("nil picker must not filter")
	}
}

func TestFeedOutSplitsOnDetect(t *testing.T) {
	f := NewFilter("s1", Deps{Picker: &stubPicker{err: ErrCancelled}})
	f.BindStdin(nopWriteCloser{new(bytes.Buffer)})
	prefix := []byte("prompt$ ")
	frame := []byte{0x2a, 0x2a, 0x18, 0x42, '0', '0', '0', '0', '0', '0'}
	out := f.FeedOut(append(append([]byte{}, prefix...), frame...))
	if !bytes.Equal(out, prefix) {
		t.Fatalf("echo=%q want %q", out, prefix)
	}
	waitIdle(t, f)
}

func TestFeedOutCrossChunkDetectWithLookbehind(t *testing.T) {
	f := NewFilter("s1", Deps{Picker: &stubPicker{err: ErrCancelled}})
	f.BindStdin(nopWriteCloser{new(bytes.Buffer)})

	// First chunk ends mid-header; those bytes are echoed (lookbehind only).
	echo1 := f.FeedOut([]byte("hi**\x18"))
	if string(echo1) != "hi**\x18" {
		t.Fatalf("echo1=%q", echo1)
	}
	// Completes header — no further echo; protocol starts.
	echo2 := f.FeedOut([]byte{'B', '0', '0'})
	if len(echo2) != 0 {
		t.Fatalf("echo2 should be empty, got %q", echo2)
	}
	waitIdle(t, f)
}

func TestFeedOutCrossChunkStarThenHeader(t *testing.T) {
	f := NewFilter("s1", Deps{Picker: &stubPicker{err: ErrCancelled}})
	f.BindStdin(nopWriteCloser{new(bytes.Buffer)})

	if got := f.FeedOut([]byte("*")); string(got) != "*" {
		t.Fatalf("got %q", got)
	}
	// Remaining header bytes after the already-echoed leading '*'.
	rest := []byte{0x2a, 0x18, 0x42, '0', '0'}
	echo := f.FeedOut(rest)
	if len(echo) != 0 {
		t.Fatalf("protocol remainder should not echo, got %q", echo)
	}
	waitIdle(t, f)
}

func TestTrailingPartialAutostartKeepsFullPrefix(t *testing.T) {
	// Raw "last 5 bytes" of "x**\x18B0" would be "*\x18B0" and break detection.
	got := trailingPartialAutostart([]byte{'x', 0x2a, 0x2a, 0x18, 0x42, '0'})
	want := []byte{0x2a, 0x2a, 0x18, 0x42, '0'}
	if !bytes.Equal(got, want) {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestFeedOutSplitAfterNoise(t *testing.T) {
	f := NewFilter("s1", Deps{Picker: &stubPicker{err: ErrCancelled}})
	f.BindStdin(nopWriteCloser{new(bytes.Buffer)})

	echo1 := f.FeedOut([]byte{'x', 0x2a, 0x2a, 0x18, 0x42, '0'})
	if !bytes.Equal(echo1, []byte{'x', 0x2a, 0x2a, 0x18, 0x42, '0'}) {
		t.Fatalf("echo1=%q", echo1)
	}
	echo2 := f.FeedOut([]byte{'0'})
	if len(echo2) != 0 {
		t.Fatalf("echo2 should be empty, got %q", echo2)
	}
	waitIdle(t, f)
}

func TestAbortClearsLookbehind(t *testing.T) {
	f := NewFilter("s1", Deps{Picker: &stubPicker{err: ErrCancelled}})
	f.BindStdin(nopWriteCloser{new(bytes.Buffer)})
	_ = f.FeedOut([]byte{0x2a, 0x2a, 0x18, 0x42, '0'}) // partial → lookbehind
	f.Reset()
	out := f.FeedOut([]byte{'1'}) // must not complete a stale ZRINIT
	if string(out) != "1" {
		t.Fatalf("echo=%q", out)
	}
	f.mu.Lock()
	lb := len(f.lookbehind)
	st := f.state
	f.mu.Unlock()
	if lb != 0 || st != stateIdle {
		t.Fatalf("after Reset: lookbehind=%d state=%v", lb, st)
	}
}

func waitIdle(t *testing.T, f *Filter) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		f.mu.Lock()
		st := f.state
		f.mu.Unlock()
		if st == stateIdle {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	f.Reset()
	t.Fatal("filter did not return to idle")
}

type stubPicker struct {
	err  error
	dir  string
	file string
}

func (p *stubPicker) PickSaveDirectory(ctx context.Context, sessionID string) (string, error) {
	if p.err != nil {
		return "", p.err
	}
	return p.dir, nil
}

func (p *stubPicker) PickOpenFile(ctx context.Context, sessionID string) (string, error) {
	if p.err != nil {
		return "", p.err
	}
	return p.file, nil
}

type nopWriteCloser struct {
	buf *bytes.Buffer
}

func (n nopWriteCloser) Write(p []byte) (int, error) { return n.buf.Write(p) }
func (n nopWriteCloser) Close() error                { return nil }
