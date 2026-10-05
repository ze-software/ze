// Design: docs/architecture/bgp/structural-forwarding.md -- treatment-sensitive body reuse.
package reactor

import (
	"bytes"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/message"
	_ "github.com/ze-software/ze/internal/component/bgp/plugins/rs"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/selector"
)

// Both destinations have the same encoding and no pre-body attribute edits.
// Grouping must not let ordinary normalization change a route-server client's
// attributes, or let the client's transparent body escape to an ordinary peer.
func TestForwardBodyCacheSeparatesClientRoles(t *testing.T) {
	testForwardOpaqueTreatment(t, [2]bool{true, false}, false)
}

// Plugin absence leaves RFC 4271 Section 5 treatment even for a configured client.
func TestForwardOpaqueTreatmentWithoutPlugin(t *testing.T) {
	snapshot := filterapi.Snapshot()
	t.Cleanup(func() { filterapi.Restore(snapshot) })
	filterapi.ResetForTest()
	testForwardOpaqueTreatment(t, [2]bool{false, false}, false)
	testForwardOpaqueTreatment(t, [2]bool{false, false}, true)
}

// A generic selector deliberately preserves the ordinary destination rather than
// the client. Exact recipient bytes prove neither rail hardcodes the client role.
func TestForwardOpaqueTreatmentUsesPluginSelection(t *testing.T) {
	snapshot := filterapi.Snapshot()
	t.Cleanup(func() { filterapi.Restore(snapshot) })
	filterapi.ResetForTest()
	destination := netip.MustParseAddr("198.18.232.3")
	if err := filterapi.Register(filterapi.Filter{
		Name: "opaque-test",
		PreserveOpaqueAttributes: func(peer filterapi.PeerFilterInfo) bool {
			return peer.Address == destination
		},
	}); err != nil {
		t.Fatal(err)
	}
	testForwardOpaqueTreatment(t, [2]bool{false, true}, false)
	testForwardOpaqueTreatment(t, [2]bool{false, true}, true)
}

// Identical policy edits must materialize separately when the effective opaque
// treatment differs, before either destination publishes bytes into dedup.
func TestForwardOpaqueMaterializationSeparatesTreatment(t *testing.T) {
	testForwardOpaqueTreatment(t, [2]bool{true, false}, true)
}

// RFC 4271 Section 5 and RFC 7947 Section 2.2: compare complete attributes after
// delivery, with both cache settings and both forwarding rails.
func testForwardOpaqueTreatment(t *testing.T, preserve [2]bool, modify bool) {
	t.Helper()
	for _, fast := range []bool{false, true} {
		for _, grouped := range []bool{false, true} {
			name := "general/ungrouped"
			if fast {
				name = "fast/ungrouped"
			}
			if grouped {
				name = "general/grouped"
				if fast {
					name = "fast/grouped"
				}
			}
			if modify {
				name += "/modified"
			}
			t.Run(name, func(t *testing.T) {
				f := newAIGPReplayFixture(t, nil)
				other, otherConn := newAnnouncePeer(t, "198.18.232.3")
				f.r.peers[other.Settings().PeerKey()] = other
				f.r.fwdPool.registerOutgoingPool(fwdKey{peerAddr: other.Settings().PeerKey()}, 4096)
				f.r.updateGroups = newUpdateGroupIndex(grouped)
				f.source.settings.RSClient = true
				f.source.refreshForwardFacts()
				for i, peer := range []*Peer{f.destination, other} {
					peer.settings.GlobalLocalAS = 65000
					peer.settings.PeerAS = 65000
					peer.settings.RSClient = i == 0
					peer.settings.NextHopMode = NextHopUnchanged
					peer.settings.ProcessBindings = f.source.settings.ProcessBindings
					peer.sendCtx.Store(f.source.sendCtx.Load())
					peer.sendCtxID, peer.recvCtxID = f.ctxID, f.ctxID
					peer.negotiated.Store(f.source.negotiated.Load())
					peer.session.sendCtxID = f.ctxID
					peer.session.negotiated = f.source.session.negotiated
					peer.refreshForwardFacts()
				}
				if modify {
					edit := func(_, _ filterapi.PeerFilterInfo, _ []byte, _ map[string]any, mods *filterapi.ModAccumulator) bool {
						mods.Op(5, filterapi.AttrModSet, []byte{0, 0, 0, 101})
						return true
					}
					f.r.orderedEgressSteps = orderedEgressStepsFromFuncs(edit)
					f.r.egressFilters = []filterapi.EgressFilterFunc{edit}
				}
				attrs := []byte{
					0x40, 1, 1, 2,
					0x40, 2, 6, 2, 1, 0, 0, 0xfd, 0xe9,
					0x40, 3, 4, 192, 0, 2, 1,
					0x40, 5, 4, 0, 0, 0, 100,
					0xc0, 241, 3, 0x11, 0x22, 0x33,
					0x80, 242, 3, 0x44, 0x55, 0x66,
				}
				prefix := []byte{24, 203, 0, 115}
				id := f.receive(t, buildUpdatePayload(attrs, prefix))
				if fast {
					update, ok := f.r.recentUpdates.Get(id)
					if !ok {
						t.Fatal("received UPDATE missing")
					}
					skipped, sent := reactorForwardRS(f.r, update, id, f.source.Settings().Address, f.source)
					if len(skipped) != 0 || sent != 2 {
						t.Fatalf("forwarded=%d skipped=%v", sent, skipped)
					}
					f.source.session.flushFwdDirty()
				} else if err := (&reactorAPIAdapter{r: f.r}).ForwardUpdate(selector.All(), id, "aigp-forwarder", plugin.ProcessSender("aigp-forwarder")); err != nil {
					t.Fatal(err)
				}
				forwardSocketBarrier(t, f.r)
				wantAttrs := bytes.Clone(attrs)
				if modify {
					_, _, localPref, found := attribute.AttrFind(wantAttrs, attribute.AttrLocalPref)
					if !found {
						t.Fatal("fixture missing LOCAL_PREF")
					}
					localPref[3] = 101
				}
				for i, conn := range []*recordingConn{f.conn, otherConn} {
					bodies := aigpSocketBodies(t, conn)
					if len(bodies) != 1 {
						t.Fatalf("destination%d UPDATE count=%d", i, len(bodies))
					}
					update, err := message.UnpackUpdate(bodies[0])
					if err != nil {
						t.Fatal(err)
					}
					if !bytes.Equal(update.NLRI, prefix) {
						t.Fatalf("destination%d NLRI=%x", i, update.NLRI)
					}
					if preserve[i] {
						if !bytes.Equal(update.PathAttributes, wantAttrs) {
							t.Errorf("preserved destination%d attributes=%x, want %x", i, update.PathAttributes, wantAttrs)
						}
						continue
					}
					if _, _, _, found := attribute.AttrFind(update.PathAttributes, attribute.AttributeCode(242)); found {
						t.Error("ordinary treatment passed unknown non-transitive attribute")
					}
					_, flags, value, found := attribute.AttrFind(update.PathAttributes, attribute.AttributeCode(241))
					if !found || flags != 0xe0 || !bytes.Equal(value, []byte{0x11, 0x22, 0x33}) {
						t.Errorf("ordinary transitive attribute flags=%x value=%x present=%v", flags, value, found)
					}
					want := bytes.Clone(wantAttrs[:len(wantAttrs)-6])
					want[len(want)-6] |= byte(attribute.FlagPartial)
					if !bytes.Equal(update.PathAttributes, want) {
						t.Errorf("sanitized destination%d attributes=%x, want %x", i, update.PathAttributes, want)
					}
				}
			})
		}
	}
}
