// Design: docs/architecture/wire/capabilities.md -- WalkOptionalParams, the one reader of the parameter framing
// Overview: gr.go -- handleStructuredOpen

package gr

import (
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/message"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	"github.com/ze-software/ze/internal/core/family"
)

// TestGRReadsExtendedOpen proves the GR plugin reads a received OPEN's Graceful
// Restart Capability under both framings: an OPEN whose parameters stay at most
// 255 octets (RFC 4271 form) and one whose parameters exceed 255 octets (the RFC
// 9072 Section 2 form, two-octet Parameter Lengths, the form ze itself sends from
// a capability total of 254 octets) store the same restart time and family.
//
// VALIDATES: spec-bgp-open-session-rfc-defects D1, the gr received-OPEN reader:
// handleStructuredOpen walks the parameters with capability.WalkOptionalParams
// and the OPEN's own ExtendedParams.
// PREVENTS: a one-octet Parameter Length walk misframing the extended OPEN, so
// the peer's GR capability is missed and its routes are purged instead of
// retained across a restart.
func TestGRReadsExtendedOpen(t *testing.T) {
	for _, tc := range []struct {
		name   string
		filler int
	}{{"base form", 10}, {"RFC 9072 form", 200}} {
		t.Run(tc.name, func(t *testing.T) {
			gp := &grPlugin{
				peerCaps:     make(map[string]*grPeerCap),
				peerLLGRCaps: make(map[string]*llgrPeerCap),
				state:        newGRStateManager(nil),
			}
			params := []capability.OptionalParam{
				// Graceful Restart: restart time 120, IPv4 unicast with F set.
				capability.RawParam(2, []byte{64, 6, 0x00, 0x78, 0x00, 0x01, 0x01, 0x80}),
			}
			for range 2 {
				params = append(params, capability.RawParam(2, append([]byte{0xee, byte(tc.filler)}, make([]byte, tc.filler)...)))
			}
			optional, extended, err := capability.PackOptionalParams(params)
			if err != nil {
				t.Fatalf("pack: %v", err)
			}
			if extended != (tc.filler == 200) {
				t.Fatalf("the test OPEN's framing is not the case's (extended=%v)", extended)
			}
			open := message.Open{Version: 4, MyAS: 65001, HoldTime: 90, BGPIdentifier: 0x01020304, OptionalParams: optional, ExtendedParams: extended}
			buf := make([]byte, open.Len(nil))
			open.WriteTo(buf, 0, nil)

			gp.handleStructuredOpen(testPeer, &bgptypes.RawMessage{RawBytes: buf[message.HeaderLen:]})

			gp.mu.Lock()
			peerCap, ok := gp.peerCaps[testPeer]
			gp.mu.Unlock()
			if !ok {
				t.Fatal("the peer's Graceful Restart Capability was not stored")
			}
			if peerCap.RestartTime != 120 {
				t.Errorf("restart time %d, want 120", peerCap.RestartTime)
			}
			if len(peerCap.Families) != 1 {
				t.Fatalf("families %v, want one", peerCap.Families)
			}
			if peerCap.Families[0].Family != family.IPv4Unicast {
				t.Errorf("family %v, want %v", peerCap.Families[0].Family, family.IPv4Unicast)
			}
			if !peerCap.Families[0].ForwardState {
				t.Error("forwarding state not set")
			}
		})
	}
}
