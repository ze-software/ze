// VALIDATES: spec-fixit-ike-resource-lifetime-leaks AC-1 -- when XfrmStateDel returns an
// error inside RemoveSA, the SPI is absent from the ESP form registry and the error still
// reaches the caller unchanged.
// PREVENTS: the host-wide raw ESP socket that a forgotten SPI holds open. The Forget call
// sat BELOW the delete's error return, and the second teardown of one Child SA (RFC 7296
// Section 1.4 Delete processing, engine/delete.go) is ordinary rather than exotic: the
// state is already gone, the kernel answers ESRCH, and the SPI stayed watched forever.
// The receiver then keeps its raw IPPROTO_ESP socket, which makes the kernel clone every
// bare ESP packet on the box, long after the last tunnel closed.
//
// No kernel state is set up: a delete for a state that was never installed is REFUSED by
// every kernel (ESRCH as root, EPERM unprivileged), which is exactly the case under test.
// So this needs no netns and no CAP_NET_ADMIN, and is merge-gated rather than nightly.

//go:build linux

package dataplane

import (
	"net"
	"net/netip"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/slogutil"
)

func TestRemoveSAForgetsESPFormWhenStateDeleteFails(t *testing.T) {
	const spi uint32 = 0xfeed1234

	b := &xfrmBackend{espForms: newESPFormReceiver(slogutil.DiscardLogger())}
	// The registry is seeded directly rather than through Watch: Watch opens the raw
	// sockets, which needs CAP_NET_RAW, and the membership is what RemoveSA must drop.
	b.espForms.reg.watch(spi, netip.MustParseAddr("198.51.100.7"), netip.MustParseAddr("203.0.113.9"))

	err := b.RemoveSA(spi, net.ParseIP("203.0.113.9"), ProtoESP)

	if err == nil {
		t.Fatal("the kernel accepted a delete for a state that was never installed, so this test exercises the success path and proves nothing")
	}
	if !strings.Contains(err.Error(), "xfrm: state del") {
		t.Errorf("RemoveSA returned %v, want the state-del failure reported unchanged to the caller", err)
	}
	if _, watched := b.espForms.reg.target(spi); watched {
		t.Errorf("spi %#x is still watched after a failed RemoveSA: the receiver holds a host-wide raw ESP socket for a tunnel that is gone", spi)
	}
}
