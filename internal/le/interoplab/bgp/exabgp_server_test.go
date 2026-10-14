// Tests for the exabgp interop mock's OPEN reader.
//
// VALIDATES: AC-1r of spec-bgp-open-session-rfc-defects for the mock:
// exabgpNegotiatedEncoding reads ADD-PATH from an RFC 9072 extended OPEN as from
// a base one.
// PREVENTS: the mock reading an extended OPEN's 255/255 marker as a one-octet
// Optional Parameters length and missing ADD-PATH.
// Related: exabgp_server.go -- exabgpNegotiatedEncoding

//go:build ze_bgp

package bgp

import (
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	"github.com/ze-software/ze/internal/core/family"
)

// TestExabgpMockReadsExtendedOpen builds the same ADD-PATH OPEN in the base
// form and, with filler parameters past 255 octets, in the RFC 9072 form, and
// reads the encoding the mock derives from each.
func TestExabgpMockReadsExtendedOpen(t *testing.T) {
	for _, tc := range []struct {
		name    string
		fillers int
	}{{"base form", 0}, {"RFC 9072 form", 2}} {
		t.Run(tc.name, func(t *testing.T) {
			// ADD-PATH (69) for ipv4/unicast, send and receive (3).
			params := []capability.OptionalParam{capability.RawParam(2, []byte{69, 4, 0, 1, 1, 3})}
			for range tc.fillers {
				params = append(params, capability.RawParam(2, append([]byte{0xee, 200}, make([]byte, 200)...)))
			}
			optional, extended, err := capability.PackOptionalParams(params)
			if err != nil {
				t.Fatalf("pack: %v", err)
			}
			if extended != (tc.fillers > 0) {
				t.Fatalf("the test OPEN's framing is not the case's (extended=%v)", extended)
			}
			open := message.Open{Version: 4, MyAS: 65001, HoldTime: 90, BGPIdentifier: 1, OptionalParams: optional, ExtendedParams: extended}
			buf := make([]byte, open.Len(nil))
			open.WriteTo(buf, 0, nil)

			encoding := exabgpNegotiatedEncoding(buf[message.HeaderLen:])
			if encoding == nil {
				t.Fatal("no ADD-PATH read from the OPEN")
			}
			if mode := encoding.AddPathMode[family.IPv4Unicast]; mode != capability.AddPathBoth {
				t.Fatalf("ipv4/unicast ADD-PATH mode %v, want both", mode)
			}
		})
	}
}
