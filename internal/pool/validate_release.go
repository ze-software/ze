//go:build !debug

package pool

// validateHandle is a no-op in release builds.
func (p *Pool) validateHandle(h Handle, op string) {}

// validateHandleForRelease is a no-op in release builds.
func (p *Pool) validateHandleForRelease(h Handle, op string) {}
