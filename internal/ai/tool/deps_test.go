package tool

import (
	"context"
	"testing"
)

func TestFromCtxIsolatesRuns(t *testing.T) {
	a := Deps{LinkID: "sess-a"}
	b := Deps{LinkID: "sess-b"}
	ctxA := WithDeps(context.Background(), a)
	ctxB := WithDeps(context.Background(), b)
	if got := FromCtx(ctxA).LinkID; got != "sess-a" {
		t.Fatalf("ctxA LinkID=%q", got)
	}
	if got := FromCtx(ctxB).LinkID; got != "sess-b" {
		t.Fatalf("ctxB LinkID=%q", got)
	}
	if got := FromCtx(context.Background()).LinkID; got != "" {
		t.Fatalf("empty ctx LinkID=%q", got)
	}
}
