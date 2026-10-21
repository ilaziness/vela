package termcharset

import (
	"bytes"
	"errors"
	"io"
	"testing"

	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

func TestNormalize(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in, want string
		ok       bool
	}{
		{"", UTF8, true},
		{"utf-8", UTF8, true},
		{"UTF8", UTF8, true},
		{"gbk", GBK, true},
		{"GB18030", GB18030, true},
		{"latin1", "", false},
	}
	for _, c := range cases {
		got, err := Normalize(c.in)
		if c.ok {
			if err != nil || got != c.want {
				t.Fatalf("Normalize(%q)=%q,%v want %q", c.in, got, err, c.want)
			}
		} else if err == nil {
			t.Fatalf("Normalize(%q) expected error", c.in)
		}
	}
}

func mustGBK(t *testing.T, s string) []byte {
	t.Helper()
	out, _, err := transform.Bytes(simplifiedchinese.GBK.NewEncoder(), []byte(s))
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestDecoderRoundTrip(t *testing.T) {
	t.Parallel()
	dec, err := NewDecoder(GBK)
	if err != nil || dec == nil {
		t.Fatalf("dec=%v err=%v", dec, err)
	}
	raw := mustGBK(t, "你好世界")
	got := dec.Bytes(raw)
	if string(got) != "你好世界" {
		t.Fatalf("got %q", got)
	}
}

func TestDecoderCrossChunk(t *testing.T) {
	t.Parallel()
	dec, err := NewDecoder(GBK)
	if err != nil || dec == nil {
		t.Fatal(err)
	}
	raw := mustGBK(t, "中文")
	if len(raw) < 2 {
		t.Fatal("expected multi-byte")
	}
	var out []byte
	out = append(out, dec.Bytes(raw[:1])...)
	out = append(out, dec.Bytes(raw[1:])...)
	if string(out) != "中文" {
		t.Fatalf("cross-chunk got %q (raw=%x)", out, raw)
	}
}

func TestEncoder(t *testing.T) {
	t.Parallel()
	enc, err := NewEncoder(GBK)
	if err != nil || enc == nil {
		t.Fatal(err)
	}
	got := enc.Bytes([]byte("测试"))
	want := mustGBK(t, "测试")
	if !bytes.Equal(got, want) {
		t.Fatalf("got %x want %x", got, want)
	}
}

func TestGB18030RoundTrip(t *testing.T) {
	t.Parallel()
	const text = "欧元"
	raw, _, err := transform.Bytes(simplifiedchinese.GB18030.NewEncoder(), []byte(text))
	if err != nil {
		t.Fatal(err)
	}
	dec, err := NewDecoder(GB18030)
	if err != nil || dec == nil {
		t.Fatal(err)
	}
	if got := string(dec.Bytes(raw)); got != text {
		t.Fatalf("decode got %q", got)
	}
	enc, err := NewEncoder(GB18030)
	if err != nil || enc == nil {
		t.Fatal(err)
	}
	if !bytes.Equal(enc.Bytes([]byte(text)), raw) {
		t.Fatalf("encode mismatch")
	}
}

func TestUTF8PassthroughNil(t *testing.T) {
	t.Parallel()
	dec, err := NewDecoder(UTF8)
	if err != nil || dec != nil {
		t.Fatalf("utf8 decoder should be nil: %v %v", dec, err)
	}
	enc, err := NewEncoder("")
	if err != nil || enc != nil {
		t.Fatalf("empty encoder should be nil: %v %v", enc, err)
	}
	var s *Stream
	in := []byte("abc")
	if out := s.Bytes(in); string(out) != "abc" {
		t.Fatal(out)
	}
}

func TestWriteCloser(t *testing.T) {
	t.Parallel()
	enc, err := NewEncoder(GBK)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	w := WrapWriteCloser(nopCloser{&buf}, enc)
	n, err := w.Write([]byte("好"))
	if err != nil || n != len("好") {
		t.Fatalf("n=%d err=%v", n, err)
	}
	want := mustGBK(t, "好")
	if !bytes.Equal(buf.Bytes(), want) {
		t.Fatalf("got %x want %x", buf.Bytes(), want)
	}
	if WrapWriteCloser(nopCloser{&buf}, nil) == nil {
		t.Fatal("nil enc should return underlying")
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte("x")); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("Write after Close: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("idempotent Close: %v", err)
	}
}

func TestDecoderFlush(t *testing.T) {
	t.Parallel()
	dec, err := NewDecoder(GBK)
	if err != nil || dec == nil {
		t.Fatal(err)
	}
	raw := mustGBK(t, "中")
	_ = dec.Bytes(raw[:1])
	got := append([]byte{}, dec.Flush()...)
	// Incomplete trailing byte at EOF should not panic; may be empty or replacement.
	_ = got
	dec2, _ := NewDecoder(GBK)
	out := append(dec2.Bytes(raw[:1]), dec2.Bytes(raw[1:])...)
	if string(out) != "中" {
		t.Fatalf("got %q", out)
	}
}

type nopCloser struct{ *bytes.Buffer }

func (nopCloser) Close() error { return nil }
