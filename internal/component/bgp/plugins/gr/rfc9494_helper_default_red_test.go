// Design: docs/guide/graceful-restart.md -- Long-Lived Graceful Restart, per-family configuration
// RFC: rfc/short/rfc9494.md -- Section 5, procedures not enabled by default
// Overview: gr_llgr_exchange.go -- exchangedLLGRLocked, the families both OPENs declared
// Related: rfc9494_gr_event_test.go -- buildOpenBody, buildCapabilityParam, buildGRCapTLV, buildLLGRCapTLV
// Related: rfc9494_llgr_entry_test.go -- rfc9494Paths, the two production event paths

package gr

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"

	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// LLGR Capability tuples (RFC 9494 Section 3): AFI, SAFI, flags (F set), 24-bit LLST
// of 3600 seconds.
var (
	llgrTupleIPv4 = [7]byte{0x00, 0x01, 0x01, 0x80, 0x00, 0x0E, 0x10}
	llgrTupleIPv6 = [7]byte{0x00, 0x02, 0x01, 0x80, 0x00, 0x0E, 0x10}
)

// rfc9494Opens delivers the OPEN Ze sent to testPeer and the OPEN it received from it,
// on the JSON or the structured path, then a TCP-failure session drop. Both OPENs carry
// a Graceful Restart Capability with Restart Time 0 for IPv4 and IPv6 unicast, so the
// LLGR procedures are the only ones that can retain anything, and either family can
// enter the LLGR period when both OPENs list it in code 71. sentLLGR and receivedLLGR
// are the LLGR Capability tuples of each OPEN; an empty list leaves code 71 out.
func rfc9494Opens(gp *grPlugin, structured bool, sentLLGR, receivedLLGR [][7]byte) {
	gr := buildGRCapTLV(0, 0, [][4]byte{{0x00, 0x01, 0x01, 0x80}, {0x00, 0x02, 0x01, 0x80}})
	for _, open := range []struct {
		direction rpc.MessageDirection
		tuples    [][7]byte
	}{{rpc.DirectionSent, sentLLGR}, {rpc.DirectionReceived, receivedLLGR}} {
		caps := [][]byte{gr}
		if len(open.tuples) > 0 {
			caps = append(caps, buildLLGRCapTLV(open.tuples))
		}
		if structured {
			gp.handleStructuredEvent(&rpc.StructuredEvent{PeerAddress: testPeer,
				EventType: rpc.EventKindOpen, Direction: open.direction,
				RawMessage: &bgptypes.RawMessage{RawBytes: buildOpenBody(buildCapabilityParam(caps...))}})
			continue
		}
		capsJSON := `{"code":64,"value":"00000001018000020180"}`
		if len(open.tuples) > 0 {
			value := ""
			for _, tuple := range open.tuples {
				value += hex.EncodeToString(tuple[:])
			}
			capsJSON += `,{"code":71,"value":"` + value + `"}`
		}
		event := `{"type":"bgp","bgp":{"message":{"type":"open","direction":"` + open.direction.String() +
			`"},"peer":{"remote":{"address":"` + testPeer + `","as":65001}},"open":{"asn":65001,` +
			`"router-id":"1.1.1.1","hold-time":90,"capabilities":[` + capsJSON + `]}}}`
		if err := gp.handleEvent(event); err != nil {
			panic("BUG: test OPEN event refused: " + err.Error())
		}
	}
	rfc9494Down(gp, structured)
}

// TestRFC9494HelperProceduresOffWithoutLocalConfig drops a session whose peer advertised
// the LLGR Capability for IPv4 unicast (Long-Lived Stale Time 3600) while Ze's own OPEN
// carried no LLGR Capability, which is what a peer with no long-lived-stale-time
// configured sends. It then repeats the drop with Ze declaring LLGR for IPv6 unicast
// only, the other family.
//
// VALIDATES: RFC 9494 Section 5: "Implementations MUST NOT enable these procedures by
// default. They MUST require affirmative configuration per AFI/SAFI in order to enable
// them." Without that configuration for IPv4 unicast, on either event path, the LLGR
// entry does not run: no NO_LLGR sweep (ffff0007), no LLGR_STALE attach (ffff0006).
// PREVENTS: a peer alone turning on long-lived retention of its routes, for up to
// 2^24 - 1 seconds, on a Ze router whose operator never configured LLGR for the family.
//
// RFC requirement: RFC9494-5-1 negative -- with Ze's OPEN carrying no LLGR Capability, or one for IPv6 unicast only, a peer's LLGR Capability for IPv4 unicast followed by a TCP failure dispatches no delete-with-community ffff0007 and no attach-community ffff0006, on both event paths.
func TestRFC9494HelperProceduresOffWithoutLocalConfig(t *testing.T) {
	for _, path := range rfc9494Paths {
		for _, local := range []struct {
			name   string
			tuples [][7]byte
		}{{"no-local-llgr", nil}, {"local-ipv6-only", [][7]byte{llgrTupleIPv6}}} {
			t.Run(path.name+"/"+local.name, func(t *testing.T) {
				gp, rec := newRecordedGRPlugin()
				rfc9494Opens(gp, path.structured, local.tuples, [][7]byte{llgrTupleIPv4})

				sent := rec.all()
				assert.NotEmpty(t, sent, "the session drop dispatches the base procedures")
				for _, command := range sent {
					assert.NotContains(t, command, "ffff0006", "LLGR_STALE attached although LLGR was not configured for IPv4")
					assert.NotContains(t, command, "ffff0007", "NO_LLGR sweep run although LLGR was not configured for IPv4")
				}
			})
		}
	}
}

// TestRFC9494HelperProceduresOnPerConfiguredFamily drops a session where Ze's OPEN
// declared LLGR for IPv4 unicast and the peer's declared it for IPv4 and IPv6 unicast.
// Both OPENs list both families in the Graceful Restart Capability, so IPv6 unicast
// reaches the LLGR decision and only Ze's missing configuration keeps it out.
//
// VALIDATES: RFC 9494 Section 5: "They MUST require affirmative configuration per
// AFI/SAFI in order to enable them." The configured family enters the LLGR period
// (NO_LLGR sweep and LLGR_STALE attach for IPv4 unicast) and the family Ze did not
// configure does not: no NO_LLGR sweep or LLGR_STALE attach for IPv6 unicast, whose
// stale routes are purged at the zero Restart Time instead, on both event paths.
// PREVENTS: an all-or-nothing gate, where configuring LLGR for one family turns the
// procedures on for every family the peer lists.
//
// RFC requirement: RFC9494-5-1 positive -- with LLGR configured for IPv4 unicast only (Ze's OPEN declares it) and both families in both GR Capabilities, a TCP failure dispatches delete-with-community ffff0007 and attach-community ffff0006 for ipv4/unicast, neither for ipv6/unicast, which the peer also listed in LLGR, and purge-stale for ipv6/unicast, on both event paths.
func TestRFC9494HelperProceduresOnPerConfiguredFamily(t *testing.T) {
	ipv4 := family.IPv4Unicast.String()
	ipv6 := family.IPv6Unicast.String()
	for _, path := range rfc9494Paths {
		t.Run(path.name, func(t *testing.T) {
			gp, rec := newRecordedGRPlugin()
			rfc9494Opens(gp, path.structured, [][7]byte{llgrTupleIPv4}, [][7]byte{llgrTupleIPv4, llgrTupleIPv6})

			sent := rec.all()
			assert.Contains(t, sent, "request bgp rib delete-with-community "+testPeer+" "+ipv4+" ffff0007")
			assert.Contains(t, sent, "request bgp rib attach-community "+testPeer+" "+ipv4+" ffff0006")
			assert.NotContains(t, sent, "request bgp rib delete-with-community "+testPeer+" "+ipv6+" ffff0007",
				"LLGR entered for a family Ze did not configure")
			assert.NotContains(t, sent, "request bgp rib attach-community "+testPeer+" "+ipv6+" ffff0006",
				"LLGR entered for a family Ze did not configure")
			assert.Contains(t, sent, "request bgp rib purge-stale "+testPeer+" "+ipv6)
		})
	}
}
