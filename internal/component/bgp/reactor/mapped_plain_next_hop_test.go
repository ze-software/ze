// Design: docs/architecture/update-building.md -- family-specific next-hop admission.
package reactor

import (
	"bytes"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/rib"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/selector"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

const mappedPlainHop = "00000000000000000000ffffc0000209"
const mappedPlainGlobal = "20010db8000100000000000000000009"
const mappedPlainLinkLocal = "fe80000000000000000000000000000a"
const mappedPlainNLRI = "4020010db800070000"

// mappedPlainNegotiate deliberately has no reverse capability-5 tuple. RFC 8950
// Section 1 recognizes the existing mapped sixteen-octet AFI 2 field itself.
func mappedPlainNegotiate(peer *Peer, families ...family.Family) {
	caps := []capability.Capability{&capability.ASN4{ASN: peer.settings.LocalAS}}
	for _, fam := range families {
		caps = append(caps, &capability.Multiprotocol{AFI: fam.AFI, SAFI: fam.SAFI})
	}
	neg := capability.Negotiate(caps, caps, capability.PeerIdentity{LocalASN: peer.settings.LocalAS, PeerASN: peer.settings.PeerAS})
	peer.session.negotiated = neg
	peer.negotiated.Store(NewNegotiatedCapabilities(neg))
	peer.sendCtx.Store(bgpctx.NewEncodingContext(neg.Identity, neg.Encoding, bgpctx.DirectionSend))
}

func mappedPlainReach(t *testing.T, fam family.Family, hop, raw string) []byte {
	t.Helper()
	field := mustHex(t, hop)
	value := append([]byte{0, 2, byte(fam.SAFI), byte(len(field))}, field...)
	value = append(value, 0)
	return append(value, mustHex(t, raw)...)
}

// TestMappedPlainNextHopBuilder checks emitted builder fields, including an
// ignored LL hint, not just a predicate result. A native four-octet IPv4 input
// must not be automatically mapped. No dataplane or transport is exercised.
func TestMappedPlainNextHopBuilder(t *testing.T) {
	for _, safi := range []family.SAFI{1, 2} {
		fam := family.Family{AFI: 2, SAFI: safi}
		for _, tc := range []struct {
			name, address, wire string
			linkLocal           bool
		}{
			{"mapped", "::ffff:192.0.2.9", mappedPlainHop, false},
			{"mapped-unused-hint", "::ffff:192.0.2.9", mappedPlainHop, true},
			{"global", "2001:db8:1::9", mappedPlainGlobal, false},
			{"native-four-octets", "192.0.2.9", "", false},
		} {
			t.Run(fam.String()+"/"+tc.name, func(t *testing.T) {
				params := message.UnicastParams{Prefix: netip.MustParsePrefix("2001:db8:7::/64"), SAFI: attribute.SAFI(safi), NextHop: netip.MustParseAddr(tc.address)}
				if tc.linkLocal {
					params.LinkLocalNextHop = netip.MustParseAddr("fe80::a")
				}
				builder := message.NewUpdateBuilder(65000, false, true, false)
				update, err := builder.BuildUnicast(&params)
				if tc.wire == "" {
					if err == nil || update != nil {
						t.Fatalf("four-octet AFI 2 input: update=%v error=%v", update, err)
					}
					return
				}
				if err != nil || update == nil {
					t.Fatalf("valid field refused: %v", err)
				}
				_, _, reach, found := attribute.AttrFind(update.PathAttributes, attribute.AttrMPReachNLRI)
				want := mappedPlainReach(t, fam, tc.wire, mappedPlainNLRI)
				if !found || !bytes.Equal(reach, want) {
					t.Fatalf("MP_REACH=%x want=%x", reach, want)
				}
			})
		}
	}
}

// TestMappedPlainNextHopOrigination observes actual Session output for explicit
// mapped input and final export replacement. Direct/default are existing
// unicast-only APIs; multicast configured input uses the generic plugin route
// carrier. Single mapped fields require no cap5; mapped pairs and native IPv4
// fields stay refused. Literal field bytes, NLRI, and EOR are the oracle.
func TestMappedPlainNextHopOrigination(t *testing.T) {
	for _, safi := range []family.SAFI{1, 2} {
		fam := family.Family{AFI: 2, SAFI: safi}
		for _, rail := range []string{"ordinary", "direct", "default", "configured", "grouped-configured", "batch", "queued", "commit"} {
			if safi != 1 && (rail == "direct" || rail == "default") {
				continue
			}
			for _, postPolicy := range []bool{false, true} {
				if rail == "direct" && postPolicy {
					continue // The legacy direct writer is covered at its producer boundary.
				}
				stage := "producer"
				if postPolicy {
					stage = "export"
				}
				for _, tc := range []struct {
					name, address, wire string
					valid               bool
				}{
					{"mapped", "::ffff:192.0.2.9", mappedPlainHop, true},
					{"global", "2001:db8:1::9", mappedPlainGlobal, true},
					{"native-four-octets", "192.0.2.9", "c0000209", false},
					{"mapped-pair", "::ffff:192.0.2.9", mappedPlainHop + mappedPlainLinkLocal, false},
				} {
					if !postPolicy && tc.name == "mapped-pair" && rail != "ordinary" && rail != "direct" {
						continue // These producers take one address, not an on-wire pair.
					}
					t.Run(fam.String()+"/"+rail+"/"+stage+"/"+tc.name, func(t *testing.T) {
						peer, conn := newInitialSyncPeer(t, true, fam)
						peer.settings.Address = netip.MustParseAddr("2001:db8:2::2")
						mappedPlainNegotiate(peer, fam)
						peer.sendingInitialRoutes.Store(0)
						peer.refreshLinkScopeFrom(nil)
						api := groupUpdatesReactor([]*Peer{peer}, false)
						raw := mappedPlainNLRI
						if rail == "default" {
							raw = "00"
						}
						want := mappedPlainReach(t, fam, tc.wire, raw)
						attrs := mixedAttrs(append([]byte{0x80, 14, byte(len(want))}, want...))
						hop := netip.MustParseAddr(tc.address)
						if postPolicy {
							hop = netip.MustParseAddr("2001:db8:1::9")
							body := buildUpdatePayload(attrs, nil)
							peer.session.egressRouteFilter = func([]byte) (bool, []byte) { return false, body }
						}
						routeNLRI, err := nlri.NewWireNLRI(fam, mustHex(t, raw), false)
						if err != nil {
							t.Fatal(err)
						}
						prefix := netip.MustParsePrefix("2001:db8:7::/64")
						switch rail {
						case "ordinary":
							err = peer.SendUpdate(&message.Update{PathAttributes: attrs})
						case "direct":
							route := bgptypes.RouteSpec{Prefix: prefix, NextHop: bgptypes.NewNextHopExplicit(hop)}
							if tc.name == "mapped-pair" && !postPolicy {
								err = peer.session.SendAnnounce(route, netip.MustParseAddr("fe80::a"), 65000, false, true, false)
							} else {
								err = peer.SendAnnounce(route, 65000)
							}
						case "default", "configured", "grouped-configured":
							peer.sendingInitialRoutes.Store(1)
							peer.initialSyncEOROwed.Store(true)
							if rail == "default" {
								peer.settings.DefaultOriginate = map[string]bool{fam.String(): true}
								peer.settings.LocalAddress = hop
							} else if safi == 1 {
								peer.settings.GroupUpdates = rail == "grouped-configured"
								peer.settings.StaticRoutes = []StaticRoute{{Prefix: prefix, NextHop: bgptypes.NewNextHopExplicit(hop)}}
							} else {
								peer.settings.PluginRoutes = []PluginRoute{{Family: fam.String(), IsIPv6: true, NLRI: mustHex(t, raw), NextHop: hop, Group: rail == "grouped-configured"}}
							}
							peer.sendInitialRoutes()
						case "batch", "queued":
							if rail == "queued" {
								peer.sendingInitialRoutes.Store(1)
								peer.initialSyncEOROwed.Store(true)
							}
							err = api.AnnounceNLRIBatch(t.Context(), selector.All(), bgptypes.NLRIBatch{Family: fam, NLRIs: []nlri.NLRI{routeNLRI}, NextHop: bgptypes.NewNextHopExplicit(hop)}, plugin.OperatorSender())
							if rail == "queued" {
								peer.sendInitialRoutes()
							}
						case "commit":
							result, commitErr := api.SendRoutes(selector.All(), []*rib.Route{rib.NewRouteWithASPath(routeNLRI, hop, nil, nil)}, nil, true, plugin.OperatorSender())
							err = commitErr
							count := 0
							if tc.valid {
								count = 1
							}
							if result.RoutesAnnounced != count {
								t.Errorf("announced=%d want=%d", result.RoutesAnnounced, count)
							}
						}
						if tc.valid && err != nil {
							t.Errorf("valid mapped/global field refused: %v", err)
						}
						announcements, eors := 0, 0
						for _, update := range recoveryWrittenUpdates(t, conn.written()) {
							if update.IsEndOfRIBAnyFamily() {
								eors++
								continue
							}
							announcements++
							_, _, reach, found := attribute.AttrFind(update.PathAttributes, attribute.AttrMPReachNLRI)
							_, _, _, legacy := attribute.AttrFind(update.PathAttributes, attribute.AttrNextHop)
							if !found || !bytes.Equal(reach, want) || legacy || len(update.NLRI) != 0 {
								t.Errorf("reach=%x want=%x legacy=%v NLRI=%x", reach, want, legacy, update.NLRI)
							}
						}
						count := 0
						if tc.valid {
							count = 1
						}
						if announcements != count {
							t.Errorf("delivered announcements=%d want=%d", announcements, count)
						}
						if (rail == "default" || rail == "configured" || rail == "grouped-configured" || rail == "queued") && eors != 1 {
							t.Errorf("EOR count=%d want=1", eors)
						}
					})
				}
			}
		}
	}
}

// TestMappedPlainNextHopForwarding drives real receive plus both forwarding
// consumers, with and without an effective export rewrite. The independent
// legacy sibling survives forwarding refusal, but a malformed received width
// resets the whole session under RFC 7606 Section 7.11 before publication.
// A mapped-plus-LL pair is invalid even when an off-link second address would
// normally be trimmed.
func TestMappedPlainNextHopForwarding(t *testing.T) {
	for _, safi := range []family.SAFI{1, 2} {
		fam := family.Family{AFI: 2, SAFI: safi}
		for _, rail := range []string{"cached", "rs"} {
			for _, rewrite := range []bool{false, true} {
				stage := "received"
				if rewrite {
					stage = "export"
				}
				for _, tc := range []struct {
					name, wire string
					valid      bool
				}{
					{"mapped", mappedPlainHop, true},
					{"global", mappedPlainGlobal, true},
					{"native-four-octets", "c0000209", false},
					{"mapped-pair", mappedPlainHop + mappedPlainLinkLocal, false},
				} {
					t.Run(fam.String()+"/"+rail+"/"+stage+"/"+tc.name, func(t *testing.T) {
						f := rfc2545ReceiveFixture(t, false, false, true)
						for _, peer := range []*Peer{f.source, f.destination} {
							mappedPlainNegotiate(peer, fam, family.IPv4Unicast)
						}
						f.destination.fwdFacts.Store(f.destination.buildForwardFacts())
						input := tc.wire
						if rewrite {
							input = mappedPlainGlobal
							replace := func(_, _ filterapi.PeerFilterInfo, payload []byte, _ map[string]any, mods *filterapi.ModAccumulator) bool {
								if payloadNextHop(payload).mpFamily == fam {
									mods.Op(14, filterapi.AttrModSet, mustHex(t, tc.wire))
								}
								return true
							}
							f.r.orderedEgressSteps = orderedEgressStepsFromFuncs(replace)
							f.r.egressFilters = []filterapi.EgressFilterFunc{replace}
						}
						reach := mappedPlainReach(t, fam, input, mappedPlainNLRI)
						attrs := mixedAttrs(append([]byte{0x80, 14, byte(len(reach))}, reach...))
						attrs = append(attrs, 0x40, 3, 4)
						attrs = append(attrs, f.source.Settings().Address.AsSlice()...)
						legacy := mustHex(t, "180a0900")
						if !rewrite && tc.name == "native-four-octets" {
							// The shared forwarding helper requires an intact announcement,
							// but RFC 7606 Section 7.11 forbids publishing this UPDATE.
							sourceConn := f.source.session.conn.(*recordingConn)
							publications := 0
							f.source.session.onMessageReceived = func(_ netip.Addr, typ msgtype.MessageType, _ []byte,
								_ *wireu.WireUpdate, _ bgpctx.ContextID, _ rpc.MessageDirection,
								_ BufHandle, _ map[string]any, _ string, _ uint64) bool {
								if typ == msgtype.TypeUPDATE {
									publications++
								}
								return false
							}
							body := buildUpdatePayload(attrs, legacy)
							original := bytes.Clone(body)
							header := message.Header{Type: msgtype.TypeUPDATE, Length: uint16(message.HeaderLen + len(body))}
							err, kept := f.source.session.processMessage(&header, body, BufHandle{ID: noPoolBufID, Buf: body})
							if err == nil || kept || f.source.session.State() != fsm.StateIdle {
								t.Fatalf("malformed receive: error=%v kept=%v state=%v, want reset", err, kept, f.source.session.State())
							}
							if publications != 0 || len(f.conn.written()) != 0 || !bytes.Equal(body, original) {
								t.Fatalf("reset published=%d recipient=%x original changed=%v", publications, f.conn.written(), !bytes.Equal(body, original))
							}
							wantNotification := mustHex(t, "ffffffffffffffffffffffffffffffff0015030301")
							if got := sourceConn.written(); !bytes.Equal(got, wantNotification) {
								t.Fatalf("reset notification=%x want=%x", got, wantNotification)
							}
							return
						}
						updates := rfc2545ReceiveForward(t, f, rail, buildUpdatePayload(attrs, legacy))
						nativeCount, legacyCount := 0, 0
						for _, update := range updates {
							if len(update.NLRI) != 0 {
								legacyCount++
								if !bytes.Equal(update.NLRI, legacy) {
									t.Errorf("legacy NLRI=%x want=%x", update.NLRI, legacy)
								}
								continue
							}
							nativeCount++
							_, _, got, announced := attribute.AttrFind(update.PathAttributes, attribute.AttrMPReachNLRI)
							_, _, withdrawn, withdrawal := attribute.AttrFind(update.PathAttributes, attribute.AttrMPUnreachNLRI)
							if tc.valid {
								want := mappedPlainReach(t, fam, tc.wire, mappedPlainNLRI)
								if !announced || withdrawal || !bytes.Equal(got, want) {
									t.Errorf("reach=%x withdrawal=%x want=%x", got, withdrawn, want)
								}
							} else {
								want := append([]byte{0, 2, byte(safi)}, mustHex(t, mappedPlainNLRI)...)
								if announced || !withdrawal || !bytes.Equal(withdrawn, want) {
									t.Errorf("reach=%x withdrawal=%x want withdrawal=%x", got, withdrawn, want)
								}
							}
						}
						if nativeCount != 1 || legacyCount != 1 {
							t.Errorf("native/legacy decisions=%d/%d want=1/1", nativeCount, legacyCount)
						}
					})
				}
			}
		}
	}
}
