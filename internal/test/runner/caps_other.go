// Design: docs/architecture/testing/ci-format.md -- option=needs-linux capability gating
// Related: caps_linux.go -- the real probe; record_parse.go -- the needs-linux option

//go:build !linux

package runner

// probeCaps is never consulted off Linux: option=needs-linux already skips on
// a non-Linux GOOS before the capability check is reached. It exists so the
// needs-linux parser compiles on every platform.
func probeCaps(_ ...int) bool { return false }
