// Design: docs/architecture/api/wire-format.md — wire-method prefix derived from the registering package

// Package callsite answers which package a registration call came from, so a
// registry can stamp each entry with a fact about the call rather than a claim
// the caller makes.
package callsite

import (
	"runtime"
	"strings"
)

// Package answers the import path of a function on the stack: Package(1)
// names the package of its own caller, Package(2) that caller's caller. It
// answers "" when the stack is that short.
// A function name is "<import path>.<symbol>", and the symbol part holds no
// slash, so the path ends at the first dot after the last slash.
func Package(skip int) string {
	pcs := make([]uintptr, 1)
	if runtime.Callers(skip+1, pcs) == 0 {
		return ""
	}
	frame, _ := runtime.CallersFrames(pcs).Next()
	name := frame.Function
	slash := strings.LastIndexByte(name, '/')
	dot := strings.IndexByte(name[slash+1:], '.')
	if dot < 0 {
		return name
	}
	return name[:slash+1+dot]
}
