// Design: docs/plugin-development/protocol.md -- The Route Fields of filter-update
package reactor

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"net/netip"
	"testing"

	_ "github.com/ze-software/ze/internal/component/bgp/plugins/nlri/evpn"
	_ "github.com/ze-software/ze/internal/component/bgp/plugins/nlri/flowspec"
	_ "github.com/ze-software/ze/internal/component/bgp/plugins/nlri/ls"
	_ "github.com/ze-software/ze/internal/component/bgp/plugins/nlri/mup"
	_ "github.com/ze-software/ze/internal/component/bgp/plugins/nlri/mvpn"
	_ "github.com/ze-software/ze/internal/component/bgp/plugins/nlri/rtc"
	_ "github.com/ze-software/ze/internal/component/bgp/plugins/nlri/srpolicy"
	_ "github.com/ze-software/ze/internal/component/bgp/plugins/nlri/vpls"
	_ "github.com/ze-software/ze/internal/component/bgp/plugins/nlri/vpn"
	"github.com/ze-software/ze/internal/component/bgp/policychain"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// opaqueFamilyBody frames MP NLRI without interpreting it. The deliberately
// undecodable tail is not a CIDR or a valid family-specific NLRI: raw filters
// must receive it unchanged, not be prevented from judging it by a prefix read.
func opaqueFamilyBody(fam family.Family, withdrawn bool) []byte {
	value := binary.BigEndian.AppendUint16(nil, uint16(fam.AFI))
	value = append(value, byte(fam.SAFI))
	code := byte(15)
	if !withdrawn {
		code = 14
		value = append(value, 0, 0) // Empty next-hop, reserved.
	}
	value = append(value, 0xff, 0, 0xff)
	attrs := []byte{0x80, code, byte(len(value))}
	attrs = append(attrs, value...)
	body := []byte{0, 0}
	body = binary.BigEndian.AppendUint16(body, uint16(len(attrs)))
	return append(body, attrs...)
}

// TestDeclaredRawReachesEveryNonCIDRFamily proves A-8 and AC-2/3: for every
// registered non-CIDR family, both MP sections reach a declared raw filter with
// the exact body and no implicit prefix field. Its accept and reject decisions
// control the import result even though the NLRI cannot be decoded as prefixes.
// Registering the family owners includes SR-Policy, which RegisterTestFamilies
// does not install; enumeration makes new linked families join the same proof.
func TestDeclaredRawReachesEveryNonCIDRFamily(t *testing.T) {
	family.RegisterTestFamilies()
	addr := netip.MustParseAddr("192.0.2.4")
	families := 0
	for _, name := range family.RegisteredFamilyNames() {
		fam, ok := family.LookupFamily(name)
		if !ok {
			t.Fatalf("registered family %q cannot be looked up", name)
		}
		if fam.PlainCIDR() {
			continue
		}
		families++
		for _, withdrawn := range []bool{false, true} {
			section := "announce"
			if withdrawn {
				section = "withdraw"
			}
			for _, action := range []rpc.FilterAction{rpc.FilterAccept, rpc.FilterReject} {
				t.Run(name+"/"+section+"/"+action.String(), func(t *testing.T) {
					body := opaqueFamilyBody(fam, withdrawn)
					transport := &externalTransport{reads: []string{rpc.FilterFieldRaw},
						reply: rpc.FilterUpdateOutput{Action: action}}
					reactor := &Reactor{filterTransportSeam: transport}
					peer := NewPeer(&PeerSettings{Address: addr, LocalAS: 65000, PeerAS: 65001,
						ImportPolicy: mixedPolicy(t, "", policychain.Member{Name: "f", Plugin: "probe"})})
					got := reactor.runIngressPolicyChain(peer, addr, nil, contextUpdate(t, body), body)
					if got.accept != (action == rpc.FilterAccept) {
						t.Fatalf("import accept = %v, filter answered %s", got.accept, action)
					}
					if got.modifiedPayload != nil {
						t.Fatalf("raw-only decision unexpectedly rewrote the UPDATE: %x", got.modifiedPayload)
					}
					if len(transport.sent) != 1 {
						t.Fatalf("filter calls = %d, want 1 (no implicit prefix parse)", len(transport.sent))
					}
					input := transport.sent[0]
					if !bytes.Equal(input.Raw, body) {
						t.Fatalf("raw = %x, want %x", input.Raw, body)
					}
					var fields map[string]json.RawMessage
					if len(input.Fields) > 0 {
						if err := json.Unmarshal(input.Fields, &fields); err != nil {
							t.Fatal(err)
						}
					}
					if len(fields) != 0 {
						t.Fatalf("raw-only declaration received decoded fields: %s", input.Fields)
					}
				})
			}
		}
	}
	if families < 17 {
		t.Fatalf("covered %d non-CIDR families, want all 17 standard registrations or more", families)
	}
}
