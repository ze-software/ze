//go:build !linux

// Design: plan/spec-vrrp-4-transport.md -- non-Linux backend stub test

package transport

import (
	"errors"
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/plugins/vrrp/packet"
)

func TestOpenInstanceUnsupportedPlatform(t *testing.T) {
	// VALIDATES: AC-13 -- off Linux the backend returns the typed
	// unsupported-platform error and never panics; frame builders still work.
	b := NewBackend()
	h, err := b.OpenInstance(InstanceSpec{Family: packet.V4, VRID: 10, Parent: "eth0"}, rxSink{})
	if h != nil {
		t.Fatalf("handle = %v, want nil", h)
	}
	if !errors.Is(err, ErrUnsupportedPlatform) {
		t.Fatalf("err = %v, want ErrUnsupportedPlatform", err)
	}
}
