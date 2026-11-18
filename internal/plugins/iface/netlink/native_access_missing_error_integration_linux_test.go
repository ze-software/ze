// Design: docs/features/interfaces.md -- original native VLAN custody
// Related: native_access_transport_linux.go -- original GETLINK errors
//go:build integration && linux

package ifacenetlink

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/test/userns"
)

// Goal: an absent original GETLINK result renders safely and retains its actual
// kernel cause, while the same transport can still return a healthy link.
// Method: issue real missing-index and loopback lookups on one route socket.
func TestNativeAccessMissingLookupRetainsKernelCause(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	withVLANQoSNetNS(t, func() {
		transport := newNativeTransactionTestTransport(t)
		ctx, cancel := context.WithTimeout(t.Context(), nativeVLANTransactionTestGuard)
		defer cancel()
		if err := transport.bind(ctx); err != nil {
			t.Fatal(err)
		}
		link, err := transport.byIndex(nativeAccessIndexMax)
		if err == nil {
			t.Fatal("missing original lookup returned no failure")
		}
		if link != nil {
			t.Fatal("missing original lookup returned a link")
		}
		if actual := err.Error(); actual != unix.ENODEV.Error() {
			t.Fatalf("original kernel error = %q, want %q", actual, unix.ENODEV.Error())
		}
		if !errors.Is(err, unix.ENODEV) {
			t.Fatal("original lookup discarded the kernel ENODEV cause")
		}
		retained := errors.Join(context.Canceled, fmt.Errorf("original lookup: %w", err))
		if retained.Error() == "" {
			t.Fatal("joined original cause did not render")
		}
		if !errors.Is(retained, unix.ENODEV) {
			t.Fatal("joined original lookup discarded ENODEV")
		}
		if !errors.Is(retained, context.Canceled) {
			t.Fatal("joined original lookup discarded the caller cause")
		}
		if !nativeVLANLinkAbsent(retained) {
			t.Fatal("joined native ENODEV was not recognized as a missing link")
		}
		healthy, err := transport.byName("lo")
		if err != nil {
			t.Fatalf("healthy original lookup failed: %v", err)
		}
		if healthy.Attrs().Name != "lo" {
			t.Fatal("healthy original lookup returned another interface")
		}
		if healthy.Attrs().Index <= 0 {
			t.Fatal("healthy original lookup returned no positive kernel index")
		}
		if transport.pending != nil {
			t.Fatal("completed original lookups left a pending transaction")
		}
	})
}

// Goal: library-owned missing-link wrappers remain valid absence evidence.
// Method: obtain an initialized wrapper from a real library GETLINK and check
// its classification through wrapping and joining without fabricating its type.
func TestNativeAccessMissingLibraryLookupRetainsClassification(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	withVLANQoSNetNS(t, func() {
		link, err := netlink.LinkByIndex(nativeAccessIndexMax)
		if err == nil {
			t.Fatal("missing library lookup returned no failure")
		}
		if link != nil {
			t.Fatal("missing library lookup returned a link")
		}
		if _, absent := errors.AsType[netlink.LinkNotFoundError](err); !absent {
			t.Fatalf("library lookup did not return its initialized missing-link error: %v", err)
		}
		if err.Error() == "" {
			t.Fatal("library missing-link error did not render")
		}
		retained := errors.Join(context.Canceled, fmt.Errorf("library lookup: %w", err))
		if !nativeVLANLinkAbsent(retained) {
			t.Fatal("joined library missing-link error lost its classification")
		}
		if !errors.Is(retained, context.Canceled) {
			t.Fatal("joined library missing-link error lost the caller cause")
		}
	})
}
