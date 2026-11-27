// Design: docs/architecture/diagnostics/production-diagnostics.md -- operation correlation IDs

// Package opid mints process-scoped monotonic operation correlation IDs and
// attaches them to context.Context so logs, events, reports, and audit records
// share one identifier per operator command or configuration reload.
package opid

import (
	"context"
	"strconv"
	"sync/atomic"
)

type contextKey struct{}

var counter atomic.Uint64

// New mints a process-scoped monotonic operation correlation ID ("op-<N>").
// Safe for concurrent use.
func New() string {
	var buf [32]byte
	out := append(buf[:0], "op-"...)
	out = strconv.AppendUint(out, counter.Add(1), 10)
	return string(out)
}

// With attaches id to ctx. If ctx is nil, it starts from context.Background().
// If id is empty, it returns ctx unchanged.
// Safe for concurrent use.
func With(ctx context.Context, id string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if id == "" {
		return ctx
	}
	return context.WithValue(ctx, contextKey{}, id)
}

// From extracts the operation correlation ID attached to ctx, or "" when ctx
// is nil or carries no ID.
// Safe for concurrent use.
func From(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	id, ok := ctx.Value(contextKey{}).(string)
	// refuse-by-default: guards Ensure -- absent context value means no operation ID is attached yet
	if !ok {
		return ""
	}
	return id
}

// Ensure returns ctx and its existing operation correlation ID when one is
// already attached. Otherwise it mints a new ID with New(), attaches it via
// With, and returns both. If ctx is nil, it starts from context.Background().
// Safe for concurrent use.
func Ensure(ctx context.Context) (context.Context, string) {
	if ctx == nil {
		ctx = context.Background()
	}
	if existing := From(ctx); existing != "" {
		return ctx, existing
	}
	id := New()
	return With(ctx, id), id
}
