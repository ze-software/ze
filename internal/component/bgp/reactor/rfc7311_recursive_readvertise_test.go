package reactor

import (
	"bytes"
	"context"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/internal/core/rib/igpcost"
	"github.com/ze-software/ze/internal/core/rib/locrib"
)

// TestRFC7311RecursiveAIGPChangeReachesWire changes the received AIGP of a
// recursive route in the actual Loc-RIB, without manually invoking replay.
// RFC 7311 Section 3.4.3: "Any change due to (a) in any of these values
// MUST trigger a new AIGP computation for that route."
// RFC requirement: RFC7311-3.4.3-7 positive -- changing a recursive next hop's received AIGP triggers a new wire advertisement without a new source UPDATE.
// RFC requirement: RFC7311-3.4.3-7 negative -- after a saturated advertisement, a finite recursive metric is recomputed from the original received value, not the previous advertisement.
// RFC requirement: RFC7311-3.4.3-2 positive -- both recursive accumulation and the subsequent source-metric addition saturate at the uint64 maximum on the wire.
// RFC requirement: RFC7311-3.4.3-2 negative -- finite recursive sums remain exact rather than being clamped or accumulated from prior sent metrics.
func TestRFC7311RecursiveAIGPChangeReachesWire(t *testing.T) {
	f := newAIGPReplayFixture(t, nil)
	loc := locrib.Default()
	protocol := redistevents.RegisterProtocol("aigp-readvertise-test")
	terminal := netip.MustParsePrefix("198.18.233.3/32")
	loc.Insert(family.IPv4Unicast, terminal, locrib.Path{Source: protocol, Metric: 7})
	t.Cleanup(func() { loc.Remove(family.IPv4Unicast, terminal, protocol, 0) })
	recursive := locrib.Path{Source: protocol, IsBGP: true, AIGPPresent: true, AIGP: 200, NextHop: terminal.Addr()}
	prefix := netip.PrefixFrom(f.source.Settings().Address, 32)
	loc.Insert(family.IPv4Unicast, prefix, recursive)
	id := f.receive(t, f.body(t, 100))
	f.forward(t, id)
	f.waitBatch(t)
	assertMetric := func(want uint64) {
		t.Helper()
		bodies := aigpSocketBodies(t, f.conn)
		if len(bodies) == 0 {
			t.Fatal("no AIGP advertisement")
		}
		got, present := aigpReceivedMetric(t, bodies[len(bodies)-1])
		if !present || got != want {
			t.Fatalf("wire metric = %d present=%v, want %d", got, present, want)
		}
	}
	assertMetric(307)
	ctx, cancel := context.WithCancel(context.Background())
	stopped := make(chan struct{})
	go func() { defer close(stopped); f.r.runAIGPAdvertisements(ctx) }()
	t.Cleanup(func() { cancel(); <-stopped })
	for _, metric := range []uint64{^uint64(0) - 5, 400, ^uint64(0) - 50, 400} {
		recursive.AIGP = metric
		loc.Insert(family.IPv4Unicast, prefix, recursive)
		f.waitBatch(t)
		want := ^uint64(0)
		if metric == 400 {
			want = 507
		}
		assertMetric(want)
	}
}

// TestRFC7311ReceivedRecursiveAIGPChangeReachesWire enters the selected-route
// producer through established sessions and the registered received-route RIB
// plugin. Only the terminal interior route is installed directly in the Loc-RIB.
// RFC 7311 Section 3.4.3: "Any change due to (a) in any of these values
// MUST trigger a new AIGP computation for that route."
// MUTATION: In rib.checkRouteBestChange, suppress publication before either
// mirrorToLocRIB call when the selected peer, next hop and MED are unchanged
// but received AIGP differs. The dependent wire metric must then stop changing.
// RFC requirement: RFC7311-3.4.3-7 positive -- received AIGP-only replacements of the selected immediate or deeper recursive BGP next hop automatically reach the dependent route's destination wire.
// RFC requirement: RFC7311-3.4.3-7 negative -- finite recovery after saturation uses the dependent route's original received AIGP without another source UPDATE or manual replay, while an unrelated control route stays unchanged.
func TestRFC7311ReceivedRecursiveAIGPChangeReachesWire(t *testing.T) {
	for _, depth := range []string{"one-bgp-hop", "two-bgp-hops"} {
		t.Run(depth, func(t *testing.T) {
			igpcost.Set(nil)
			t.Cleanup(func() { igpcost.Set(nil) })
			loc := locrib.Default()
			require.NotNil(t, loc)
			protocol := redistevents.RegisterProtocol("aigp-readvertise-test")
			bgpProtocol := redistevents.RegisterProtocol("bgp")
			terminal := netip.MustParsePrefix("198.18.234.3/32")
			first := netip.MustParsePrefix("198.18.234.1/32")
			second := netip.MustParsePrefix("198.18.234.2/32")
			dependent := []byte{24, 10, 20, 0}
			control := []byte{24, 10, 21, 0}
			loc.Insert(family.IPv4Unicast, terminal, locrib.Path{Source: protocol, Metric: 7})
			t.Cleanup(func() {
				loc.Remove(family.IPv4Unicast, terminal, protocol, 0)
				for _, prefix := range []netip.Prefix{first, second, netip.MustParsePrefix("10.20.0.0/24"), netip.MustParsePrefix("10.21.0.0/24")} {
					loc.Remove(family.IPv4Unicast, prefix, bgpProtocol, 0)
				}
			})

			source := lowLiveSettings("192.0.2.1", 65000, 65002)
			destination := lowLiveSettings("192.0.2.2", 65000, 65003)
			localPort := uint16(freePort(t)) //nolint:gosec // Kernel-selected TCP port.
			for _, settings := range []*PeerSettings{source, destination} {
				settings.AIGPSession = new(true)
				settings.LocalAddress = netip.MustParseAddr("127.0.0.1")
				settings.LocalPort = localPort
			}
			destination.NextHopMode = NextHopSelf
			peers := lowLiveRouter(t, source, destination)
			update := func(prefix []byte, nextHop netip.Addr, metric uint64) *message.Update {
				// All replacements have the same peer, prefix, AS_PATH, next hop,
				// and MED. Only the first AIGP TLV's uint64 value changes.
				attrs := []byte{0x40, 1, 1, 0, 0x40, 2, 6, 2, 1, 0, 0, 0xfd, 0xea, 0x40, 3, 4}
				attrs = append(attrs, nextHop.AsSlice()...)
				attrs = append(attrs, 0x80, 4, 4, 0, 0, 0, 19)
				attrs = append(attrs, 0x80, byte(attribute.AttrAIGP), 11)
				var tlv [11]byte
				attribute.WriteAIGPMetric(tlv[:], 0, metric)
				attrs = append(attrs, tlv[:]...)
				return &message.Update{PathAttributes: attrs, NLRI: prefix}
			}
			receiveRecursive := func(prefix netip.Prefix, nextHop netip.Addr, metric uint64) {
				t.Helper()
				nlri := append([]byte{32}, prefix.Addr().AsSlice()...)
				peers[0].send(t, message.PackTo(update(nlri, nextHop, metric), nil))
				lowEventually(t, func() bool {
					path, foundPrefix, found := loc.LPM(family.IPv4Unicast, prefix.Addr())
					return found && foundPrefix == prefix && path.IsBGP && path.AIGPPresent &&
						path.AIGP == metric && path.NextHop == nextHop && path.Metric == 19
				}, "received recursive AIGP published by the best-path producer")
			}
			firstNextHop := terminal.Addr()
			initial := uint64(307)
			if depth == "two-bgp-hops" {
				receiveRecursive(second, terminal.Addr(), 300)
				firstNextHop = second.Addr()
				initial = 607
			}
			receiveRecursive(first, firstNextHop, 200)

			// Capture only the destination's actual UPDATE bodies, not events or
			// the output mirror. Poll the required wire value rather than a
			// generic completion token belonging to an earlier advertisement.
			wireBodies := func(prefix []byte) [][]byte {
				t.Helper()
				peers[1].mu.Lock()
				defer peers[1].mu.Unlock()
				var bodies [][]byte
				for _, frame := range peers[1].frames {
					if frame[18] != 2 {
						continue
					}
					u, err := message.UnpackUpdate(frame[19:])
					require.NoError(t, err)
					require.NotEqual(t, prefix, u.WithdrawnRoutes, "a resolved route must not be withdrawn")
					if bytes.Equal(u.NLRI, prefix) {
						bodies = append(bodies, bytes.Clone(frame[19:]))
					}
				}
				return bodies
			}
			waitMetric := func(prefix []byte, want uint64) {
				t.Helper()
				lowEventually(t, func() bool {
					bodies := wireBodies(prefix)
					if len(bodies) == 0 {
						return false
					}
					got, present := aigpReceivedMetric(t, bodies[len(bodies)-1])
					return present && got == want
				}, "exact dependent AIGP on the destination wire")
			}
			sourceUpdate := update(dependent, first.Addr(), 100)
			controlUpdate := update(control, terminal.Addr(), 23)
			peers[0].send(t, message.PackTo(sourceUpdate, nil))
			waitMetric(dependent, initial)
			peers[0].send(t, message.PackTo(controlUpdate, nil))
			waitMetric(control, 30)
			controlWire := wireBodies(control)
			require.Len(t, controlWire, 1)
			lowEventually(t, func() bool {
				return bytes.Equal(lowInstalledAttributes(dependent), sourceUpdate.PathAttributes) &&
					bytes.Equal(lowInstalledAttributes(control), controlUpdate.PathAttributes)
			}, "original dependent and control attributes retained in the received RIB")

			changedPrefix, changedNextHop := first, firstNextHop
			finite, sourceSaturation := uint64(507), ^uint64(0)-50
			if depth == "two-bgp-hops" {
				changedPrefix, changedNextHop = second, terminal.Addr()
				finite, sourceSaturation = 707, ^uint64(0)-250
			}
			wants := []uint64{initial}
			for _, change := range []struct {
				received uint64
				wire     uint64
			}{
				{^uint64(0) - 5, ^uint64(0)}, // Recursive accumulation saturates.
				{400, finite},
				{sourceSaturation, ^uint64(0)}, // Only addition of source 100 saturates.
				{400, finite},
			} {
				receiveRecursive(changedPrefix, changedNextHop, change.received)
				waitMetric(dependent, change.wire)
				wants = append(wants, change.wire)
				require.Equal(t, controlWire, wireBodies(control), "unrelated control route must not be re-advertised or edited")
				require.Equal(t, sourceUpdate.PathAttributes, lowInstalledAttributes(dependent), "recomputation must not replace the received source generation")
				require.Equal(t, controlUpdate.PathAttributes, lowInstalledAttributes(control))
			}
			if depth == "two-bgp-hops" {
				// The same deeper chain must also react to its immediate BGP
				// hop, without replacing the dependent or deeper received route.
				receiveRecursive(first, firstNextHop, 600)
				waitMetric(dependent, 1107)
				wants = append(wants, 1107)
				require.Equal(t, controlWire, wireBodies(control))
				require.Equal(t, sourceUpdate.PathAttributes, lowInstalledAttributes(dependent))
			}
			var transitions []uint64
			for _, body := range wireBodies(dependent) {
				metric, present := aigpReceivedMetric(t, body)
				require.True(t, present)
				if len(transitions) == 0 || transitions[len(transitions)-1] != metric {
					transitions = append(transitions, metric)
				}
			}
			require.Equal(t, wants, transitions, "all observed wire transitions must use the original received metric")
		})
	}
}
