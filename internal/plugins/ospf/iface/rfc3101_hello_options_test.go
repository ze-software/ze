// Design: docs/architecture/ospf/ospf-11-stub-nssa.md -- NSSA Hello option agreement.
//
// VALIDATES: RFC 3101 sec 2.1 on the receive check every Hello passes (ReceiveHello ->
// validateHelloLocked) and on the Hello Ze builds. Both the N-bit and the E-bit of a
// received Hello must match the area type of the receiving interface, for a normal, a stub
// and an NSSA interface alike; on an NSSA a Hello carrying N and E together is refused; and
// the Hello Ze itself sends on an NSSA interface carries N set and E clear.
// PREVENTS: an N-bit compared only on NSSA interfaces (a normal or stub interface forming an
// adjacency with an NSSA router), and an E-bit check skipped whenever N is set.
package iface

import (
	"testing"
	"time"

	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// helloOptionsCase is one received Hello option byte on one interface area type.
type helloOptionsCase struct {
	name     string
	areaType string
	options  types.Options
	want     string
}

// receiveHelloOptions returns the drop reason ReceiveHello gives a Hello carrying options.
func receiveHelloOptions(t *testing.T, tc helloOptionsCase) string {
	t.Helper()
	cfg := baseConfig(t)
	cfg.AreaType = tc.areaType
	ifc := New(cfg, &fakeSender{}, NopMetrics())
	h := helloFor(cfg)
	h.Options = tc.options
	return ifc.ReceiveHello(rid(t, "10.0.0.2"), h, time.Now())
}

// RFC requirement: RFC3101-2.1-1 positive -- a received Hello whose N-bit and E-bit both
// match the area type of the receiving interface is accepted on each area type: normal
// (E set, N clear), stub (both clear) and NSSA (N set, E clear).
func TestRFC3101HelloOptionsMatchingAreaTypeAccepted(t *testing.T) {
	cases := []helloOptionsCase{
		{name: "normal", areaType: types.AreaTypeNormal, options: types.OptionE, want: ""},
		{name: "stub", areaType: types.AreaTypeStub, options: 0, want: ""},
		{name: "nssa", areaType: types.AreaTypeNSSA, options: types.OptionNP, want: ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := receiveHelloOptions(t, tc); got != tc.want {
				t.Fatalf("%s interface, options %#02x: drop reason %q, want accepted", tc.areaType, uint8(tc.options), got)
			}
		})
	}
}

// RFC requirement: RFC3101-2.1-1 negative -- a received Hello whose N-bit or E-bit
// disagrees with the receiving interface's area type is dropped on EVERY area type: a
// normal interface drops N set (options-n) and E clear (options-e); a stub interface drops
// N set (options-n) and E set (options-e); an NSSA interface drops N clear (options-n).
func TestRFC3101HelloOptionsMismatchDroppedOnEveryAreaType(t *testing.T) {
	cases := []helloOptionsCase{
		{name: "normal-n-set", areaType: types.AreaTypeNormal, options: types.OptionE | types.OptionNP, want: DropReasonOptionsN},
		{name: "normal-e-clear", areaType: types.AreaTypeNormal, options: 0, want: DropReasonOptionsE},
		{name: "stub-n-set", areaType: types.AreaTypeStub, options: types.OptionNP, want: DropReasonOptionsN},
		{name: "stub-e-set", areaType: types.AreaTypeStub, options: types.OptionE, want: DropReasonOptionsE},
		{name: "nssa-n-clear", areaType: types.AreaTypeNSSA, options: 0, want: DropReasonOptionsN},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := receiveHelloOptions(t, tc); got != tc.want {
				t.Fatalf("%s interface, options %#02x: drop reason %q, want %q", tc.areaType, uint8(tc.options), got, tc.want)
			}
		})
	}
}

// RFC requirement: RFC3101-x-1 positive -- the Hello Ze builds for an NSSA interface
// (buildHelloPacketLocked, the packet every Hello send uses) carries the N-bit (0x08) set
// and the E-bit (0x02) clear in the Options octet at offset 30 of the OSPFv2 packet.
func TestRFC3101NSSAHelloSentWithNSetAndEClear(t *testing.T) {
	cfg := baseConfig(t)
	cfg.AreaType = types.AreaTypeNSSA
	ifc := New(cfg, &fakeSender{}, NopMetrics())
	// OSPFv2 header (24 octets), then Network Mask (4) and HelloInterval (2): Options at 30.
	const helloOptionsOffset = 30
	wire := ifc.buildHelloPacketLocked()
	if len(wire) <= helloOptionsOffset {
		t.Fatalf("Hello is %d octets, too short to carry Options", len(wire))
	}
	options := wire[helloOptionsOffset]
	if options&0x08 == 0 {
		t.Fatalf("NSSA Hello Options %#02x: N-bit (0x08) clear, want set", options)
	}
	if options&0x02 != 0 {
		t.Fatalf("NSSA Hello Options %#02x: E-bit (0x02) set while N is set, want clear", options)
	}
}

// RFC requirement: RFC3101-x-1 negative -- on an NSSA interface a received Hello with the
// N-bit set AND the E-bit set (N set => E clear violated) is dropped (options-e), even
// though its N-bit alone matches the area.
func TestRFC3101NSSAHelloWithNAndESetRefused(t *testing.T) {
	tc := helloOptionsCase{name: "nssa-n-and-e", areaType: types.AreaTypeNSSA, options: types.OptionNP | types.OptionE, want: DropReasonOptionsE}
	if got := receiveHelloOptions(t, tc); got != tc.want {
		t.Fatalf("NSSA Hello with N and E set: drop reason %q, want %q", got, tc.want)
	}
}
