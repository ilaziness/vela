package tool

import "context"

type callIDKey struct{}

// WithCallID stores the tool call id on ctx so tool handlers can correlate
// per-call decisions (e.g. terminal-exec marks) without a ToolContext field.
func WithCallID(ctx context.Context, callID string) context.Context {
	if ctx == nil || callID == "" {
		return ctx
	}
	return context.WithValue(ctx, callIDKey{}, callID)
}

// CallIDFromCtx returns the call id stored by WithCallID, or "".
func CallIDFromCtx(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	id, _ := ctx.Value(callIDKey{}).(string)
	return id
}
