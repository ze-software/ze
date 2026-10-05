// Design: docs/guide/graceful-restart.md -- OPEN capability values and family-native retention.
// Related: internal/component/bgp/format/decode.go -- normalized OPEN producer.
// Related: real_rib_test.go -- registered RIB engine command consumer.

package gr

import (
	"encoding/hex"
	"net/netip"
	"slices"
	"testing"
	"testing/synctest"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/format"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestNormalizedOpenRetainsNativeFamilies delivers real wire capability values
// through the JSON producer and structured OPEN paths, including mixed transport
// directions. It reads the real received RIB before and after the GR deadline,
// and records the production family-specific readvertisement commands.
func TestNormalizedOpenRetainsNativeFamilies(t *testing.T) {
	for _, shape := range []struct {
		name string
		caps string
		both bool
	}{
		{"ipv4-only", "400600030001018047070001018000003c", false},
		{"both-families", "400a00030001018000020180470e0001018000003c0002018000003c", true},
	} {
		for _, received := range rfc9494Paths {
			for _, sent := range rfc9494Paths {
				t.Run(shape.name+"/received-"+received.name+"/sent-"+sent.name, func(t *testing.T) {
					synctest.Test(t, func(t *testing.T) {
						gp, rib := newGRWithRealRIB(t)
						var rec dispatchRecorder
						record := rec.hook()
						gp.dispatchHook = func(command string, args ...string) {
							record(command, args...)
							rib.command(command, args...)
						}
						body := normalizedGRTestOpen(t, shape.caps)
						deliverNormalizedGRTestOpen(t, gp, body, rpc.DirectionReceived, received.structured)
						deliverNormalizedGRTestOpen(t, gp, body, rpc.DirectionSent, sent.structured)
						wantFamilies := []family.Family{family.IPv4Unicast}
						if shape.both {
							wantFamilies = append(wantFamilies, family.IPv6Unicast)
						}
						cap := gp.peerCaps[testPeer]
						if cap == nil {
							t.Fatal("received OPEN did not publish GR capability")
						}
						if cap.RestartTime != 3 {
							t.Errorf("received restart time = %d, want 3", cap.RestartTime)
						}
						gotFamilies := make([]family.Family, 0, len(cap.Families))
						for _, entry := range cap.Families {
							gotFamilies = append(gotFamilies, entry.Family)
						}
						if !slices.Equal(gotFamilies, wantFamilies) {
							t.Errorf("received GR families = %v, want %v", gotFamilies, wantFamilies)
						}
						rib.received("18c63364", grReceivedAttrs)
						rib.received("18c63365", grReceivedAttrs+"c00804ffff0007")
						rib.command("request bgp rib inject", testPeer, "ipv6/unicast", "2001:db8::/32", "nexthop", "::1")
						rfc9494Down(gp, received.structured)
						rib.down()
						rib.requireStale("198.51.100.0/24", 1)
						rib.requireStale("198.51.101.0/24", 1)
						if shape.both {
							rib.requireStale("2001:db8::/32", 1)
						} else if routes := rib.routes("2001:db8::/32"); len(routes) != 0 {
							t.Fatalf("nonretained IPv6 routes survived DOWN: %v", routes)
						}
						time.Sleep(3 * time.Second)
						synctest.Wait()
						rib.requireStale("198.51.100.0/24", 2)
						if !slices.Contains(rib.communities("198.51.100.0/24"), uint32(0xffff0006)) {
							t.Error("retained route lacks LLGR_STALE after GR expiry")
						}
						if routes := rib.routes("198.51.101.0/24"); len(routes) != 0 {
							t.Errorf("NO_LLGR route survived LLGR entry: %v", routes)
						}
						if shape.both {
							rib.requireStale("2001:db8::/32", 2)
						}
						for _, fam := range wantFamilies {
							command := "clear bgp rib out !" + testPeer + " " + fam.String()
							if !slices.Contains(rec.all(), command) {
								t.Errorf("family-native readvertisement missing: %s", command)
							}
						}
					})
				})
			}
		}
	}
}

// normalizedGRTestOpen frames the same code-64/code-71 values the stress peer
// sends: three seconds of GR, sixty seconds of LLGR, and preserved forwarding.
func normalizedGRTestOpen(t *testing.T, capabilities string) []byte {
	t.Helper()
	caps, err := hex.DecodeString(capabilities)
	if err != nil {
		t.Fatal(err)
	}
	body := []byte{4, 0, 1, 0, 90, 192, 0, 2, 1, byte(2 + len(caps)), 2, byte(len(caps))}
	return append(body, caps...)
}

// deliverNormalizedGRTestOpen keeps the JSON producer in the test: hand-written
// value strings would bypass the capability TLV/value boundary under regression.
func deliverNormalizedGRTestOpen(t *testing.T, gp *grPlugin, body []byte, direction rpc.MessageDirection, structured bool) {
	t.Helper()
	if structured {
		gp.handleStructuredEvent(&rpc.StructuredEvent{
			PeerAddress: testPeer, EventType: rpc.EventKindOpen, Direction: direction,
			RawMessage: &bgptypes.RawMessage{RawBytes: body},
		})
		return
	}
	peer := plugin.PeerInfo{Address: netip.MustParseAddr(testPeer), PeerAS: 1, LocalAS: 1}
	encoder := format.NewJSONEncoder("test")
	if err := gp.handleEvent(encoder.Open(&peer, format.DecodeOpen(body), direction, 1)); err != nil {
		t.Fatal(err)
	}
}
