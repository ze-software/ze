// VALIDATES: OnBGPMessage builds an MRT BGP4MP record for a maximum-size RFC 8654
// extended BGP message (65535 bytes) without overflowing the pooled record buffer.
// PREVENTS: a peer sending a near-max extended UPDATE from panicking the session
// goroutine (slice bounds out of range) and crashing ze when MRT recording is
// enabled -- a remotely-triggerable crash. See internal/plugins/mrt/dump.go
// (maxRecordLen) and component.go OnBGPMessage.
package mrt

import (
	"net/netip"
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/core/bgp/msgtype"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	mrtfmt "codeberg.org/thomas-mangin/ze/internal/mrt"
)

func TestOnBGPMessageMaxSizeExtendedMessageNoOverflow(t *testing.T) {
	// Worst case: extended timestamp (16-byte MRT header) + IPv6/AS4 (44-byte
	// BGP4MP common header) + a maximum 65535-byte BGP message = 65595 bytes,
	// which must fit the pooled record buffer.
	c := New(Config{ExtendedTimestamp: true}, nil)
	w := mrtfmt.NewWriter(t.TempDir() + "/all-%Y%m%d.mrt")
	c.allMsgs = newAsyncWriter(w, c.logger)

	// Deferred LIFO: recover runs first (turns the crash into a clear failure),
	// then the writer is closed.
	defer func() { _ = c.allMsgs.Close() }()
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("OnBGPMessage panicked on a max-size extended message "+
				"(record buffer too small): %v", r)
		}
	}()

	peer := &plugin.PeerInfo{
		Address:      netip.MustParseAddr("2001:db8::1"),
		LocalAddress: netip.MustParseAddr("2001:db8::2"),
		PeerAS:       65001,
		LocalAS:      65002,
	}
	raw := make([]byte, 65535) // maximum BGP message length (16-bit length field)

	c.OnBGPMessage(peer, msgtype.TypeUPDATE, false, raw)
}
