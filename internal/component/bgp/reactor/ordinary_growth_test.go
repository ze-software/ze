// Design: docs/architecture/api/architecture.md -- ordinary route injection and commits.
// Related: rfc2545_ordinary_joint_subnet_test.go -- real ordinary egress topology.
package reactor

import (
	"bytes"
	"context"
	"encoding/binary"
	"net/netip"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/rib"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/wire"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/selector"
)

// TestOrdinaryIPv6GrowthBoundary sends one legal original chunk with less than
// sixteen spare octets through the actual ordinary egress callback. Adding the
// owned link-local next hop must split delivery, not lose the legal routes or
// reapply export policy. The API build slot is only 4096 bytes: the extended
// boundary deliberately uses Session.SendUpdate, not an unsupported API batch.
func TestOrdinaryIPv6GrowthBoundary(t *testing.T) {
	for _, tc := range []struct {
		name    string
		ceiling int
		routes  int
		queued  bool
		commit  bool
	}{
		{name: "api-4096", ceiling: 4096, routes: 237},
		{name: "queued-maintenance-4096", ceiling: 4096, routes: 237, queued: true},
		{name: "named-commit-4096", ceiling: 4096, routes: 237, commit: true},
		{name: "session-65535", ceiling: 65535, routes: 3851},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, peer, conn := newOrdinaryJointPeer(t, netip.MustParseAddr("2001:db8:a::2"), ordinaryJointPolicyPair)
			session := peer.session
			t.Cleanup(session.timers.StopAll)
			peer.settings.GroupUpdates = true
			if tc.ceiling == 65535 {
				caps := []capability.Capability{&capability.ExtendedMessage{}}
				session.negotiated = capability.Negotiate(caps, caps, capability.PeerIdentity{LocalASN: 65000, PeerASN: 65001})
				session.writeBuf.Resize(session.negotiated.ExtendedMessageSend)
			}
			r.api = &pluginserver.Server{}
			ctxID, err := bgpctx.Registry.Register(peer.sendCtx.Load())
			if err != nil {
				t.Fatal(err)
			}
			session.sendCtxID = ctxID
			session.adjOut = &peer.adjOut
			peer.sendCtxID = ctxID
			// The fixture is single-threaded. Republish its final configuration
			// before any API build or ordinary export callback observes it.
			peer.fwdFacts.Store(peer.buildForwardFacts())
			policyCalls := 0
			r.policyFilterSeam = func(_, _, _, _ string, _ uint32, _ string) PolicyResponse {
				policyCalls++
				return PolicyResponse{Action: PolicyAccept}
			}

			update, wantNLRI := ordinaryGrowthUpdate(t, tc.routes)
			// Header 19 + section lengths 4 + ORIGIN 4 + AS_PATH 9 +
			// extended MP header 4 + AFI/SAFI/NH/reserved 21 + 17 per /128.
			if got := update.Len(nil); got != 61+17*tc.routes {
				t.Fatalf("fixture length = %d, want %d", got, 61+17*tc.routes)
			}
			spare := tc.ceiling - update.Len(nil)
			if spare < 0 {
				t.Fatalf("original fixture exceeds ceiling by %d", -spare)
			}
			if spare >= 16 {
				t.Fatalf("fixture leaves %d octets; normalization would not cross ceiling", spare)
			}

			api := &reactorAPIAdapter{r: r}
			batch := bgptypes.NLRIBatch{
				Family:  family.IPv6Unicast,
				NLRIs:   manyIPv6Host(tc.routes),
				NextHop: bgptypes.NewNextHopExplicit(peer.settings.LocalAddress),
			}
			var eors uint32
			var commit bgptypes.TransactionResult
			var seedOctets int
			var seedStats PeerStats
			var pool *fwdPool
			if tc.queued {
				// A captured-owner maintenance replacement stays one grouped
				// API operation; initial opQueue draining instead sends each
				// route individually and does not reach this size boundary.
				seed := batch
				seed.NextHop = bgptypes.NewNextHopExplicit(netip.MustParseAddr("2001:db8:c::1"))
				if err := api.AnnounceNLRIBatch(t.Context(), selector.All(), seed, plugin.OperatorSender()); err != nil {
					t.Fatalf("foreign-global seed was refused: %v", err)
				}
				batch.SentOwnerMessage = session.sentReceipt.MessageID()
				if batch.SentOwnerMessage == 0 {
					t.Fatal("seed lacks a real sent-ownership receipt")
				}
				seedOctets = len(conn.written())
				if seedOctets != update.Len(nil) {
					t.Fatalf("seed wire length = %d, want one %d-octet original chunk", seedOctets, update.Len(nil))
				}
				seedStats = peer.Stats()
				if seedStats.UpdatesSent != 1 {
					t.Fatalf("seed UPDATE count = %d, want 1", seedStats.UpdatesSent)
				}
				policyCalls = 0
				pool = newFwdPool(fwdBatchHandler, fwdPoolConfig{chanSize: 8, idleTimeout: time.Second})
				r.fwdPool = pool
				peer.sendingInitialRoutes.Store(1)
				t.Cleanup(func() {
					// Release ordering before Stop, including fatal exits.
					peer.sendingInitialRoutes.Store(0)
					peer.wakeForwardOverflow()
					pool.Stop()
				})
			}
			if tc.commit {
				routes := make([]*rib.Route, len(batch.NLRIs))
				for i, n := range batch.NLRIs {
					routes[i] = rib.NewRouteWithASPath(n, peer.settings.LocalAddress, nil, nil)
				}
				var err error
				commit, err = api.SendRoutes(selector.All(), routes, nil, true, plugin.OperatorSender())
				if err != nil {
					t.Errorf("legal named commit lost after next-hop growth: %v", err)
				}
				eors = 1
			} else if tc.ceiling == 65535 {
				if err := session.SendUpdate(update); err != nil {
					t.Errorf("legal extended chunk lost after next-hop growth: %v", err)
				}
			} else if err := api.AnnounceNLRIBatch(t.Context(), selector.All(), batch, plugin.OperatorSender()); err != nil {
				t.Errorf("legal API chunk lost after next-hop growth: %v", err)
			}
			if tc.queued {
				if len(conn.written()) != seedOctets {
					t.Fatal("maintenance replacement escaped its ordering hold")
				}
				if policyCalls != 0 {
					t.Fatalf("queued policy ran before release: %d", policyCalls)
				}
				peer.sendingInitialRoutes.Store(0)
				peer.wakeForwardOverflow()
				ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
				defer cancel()
				if err := pool.Barrier(ctx); err != nil {
					t.Fatal(err)
				}
			}
			frames := ordinaryGrowthWire(t, conn.written()[seedOctets:], tc.ceiling, wantNLRI, eors)
			if frames < 2 {
				t.Errorf("grown chunk delivered in %d route frames, want at least two", frames)
			}
			if policyCalls != 1 {
				t.Errorf("export policy applied %d times, want once for original chunk", policyCalls)
			}
			if stats := peer.Stats(); stats.UpdatesSent-seedStats.UpdatesSent != uint32(frames)+eors || stats.EORSent-seedStats.EORSent != eors {
				t.Errorf("subject updates/EOR = %d/%d, want %d/%d", stats.UpdatesSent-seedStats.UpdatesSent, stats.EORSent-seedStats.EORSent, uint32(frames)+eors, eors)
			}
			if !peer.hasAdvertised() {
				t.Error("delivered routes not recorded as advertised")
			}
			if tc.commit {
				if commit.RoutesQueued != tc.routes || commit.RoutesAnnounced != tc.routes || commit.RoutesWithdrawn != 0 || commit.UpdatesSent != frames+int(eors) || commit.EORSent != int(eors) {
					t.Errorf("commit result = %+v, want %d routes and %d actual UPDATEs including EOR", commit, tc.routes, frames+int(eors))
				}
				if len(commit.Peers) != 1 {
					t.Fatalf("commit peer rows = %d, want 1", len(commit.Peers))
				}
				row := commit.Peers[0]
				if row.Address != peer.settings.Address.String() || row.RoutesAnnounced != tc.routes || row.RoutesWithdrawn != 0 || row.UpdatesSent != frames+int(eors) || row.EORSent != int(eors) || len(row.Reasons) != 0 {
					t.Errorf("commit peer result = %+v, want complete delivery and actual frame accounting", row)
				}
			}

			// A different prefix proves the same session remains usable; a repeat
			// could disappear into Adj-RIB-Out duplicate suppression instead.
			before := len(conn.written())
			batch.SentOwnerMessage = 0
			batch.NLRIs = manyIPv6Host(tc.routes + 1)[tc.routes:]
			if err := api.AnnounceNLRIBatch(t.Context(), selector.All(), batch, plugin.OperatorSender()); err != nil {
				t.Fatalf("subsequent ordinary send failed: %v", err)
			}
			_, allNLRI := ordinaryGrowthUpdate(t, tc.routes+1)
			ordinaryGrowthWire(t, conn.written()[before:], tc.ceiling, allNLRI[len(wantNLRI):], 0)
			if policyCalls != 2 {
				t.Errorf("policy calls after subsequent send = %d, want 2", policyCalls)
			}
			if session.writeFailed != nil {
				t.Errorf("session write failed: %v", session.writeFailed)
			}
			if session.fsm.State() != fsm.StateEstablished {
				t.Errorf("session state = %v, want Established", session.fsm.State())
			}
			if got := len(peer.adjOut.routes); got != tc.routes+1 {
				t.Errorf("owned routes after subsequent send = %d, want %d", got, tc.routes+1)
			}
		})
	}
}

// TestOrdinaryIPv6GrowthPartialCommit accounts for the output that actually
// leaves before a later section is refused. Unlike two NLRIs in one MP section
// (which are prevalidated together), Splitter emits legacy withdrawals before
// attempting MP_REACH. A real raw export replacement exercises that distinction.
func TestOrdinaryIPv6GrowthPartialCommit(t *testing.T) {
	r, peer, conn := newOrdinaryJointPeer(t, netip.MustParseAddr("2001:db8:a::2"), ordinaryJointPolicyPair)
	session := peer.session
	t.Cleanup(session.timers.StopAll)
	ctxID, err := bgpctx.Registry.Register(peer.sendCtx.Load())
	if err != nil {
		t.Fatal(err)
	}
	session.sendCtxID = ctxID
	session.adjOut = &peer.adjOut
	peer.sendCtxID = ctxID
	peer.fwdFacts.Store(peer.buildForwardFacts())

	// 1001 ordinary communities are valid, unique 65000:value pairs. Their
	// extended-length attribute occupies 4008 octets, all below MP_REACH's
	// type code, so Splitter does not charge a high-attribute stash.
	communities := make([]byte, 4008)
	copy(communities, []byte{0xd0, 8, 0x0f, 0xa4})
	for i := range 1001 {
		binary.BigEndian.PutUint16(communities[4+i*4:], 65000)
		binary.BigEndian.PutUint16(communities[6+i*4:], uint16(i))
	}
	small, wantNLRI := ordinaryGrowthUpdate(t, 1)
	attrs := append(bytes.Clone(small.PathAttributes[:13]), communities...)
	mp := small.PathAttributes[17:]
	attrs = append(attrs, 0x80, 14, byte(len(mp)))
	attrs = append(attrs, mp...)
	replacement := message.Update{
		WithdrawnRoutes: mustHex(t, "18c63364"),
		PathAttributes:  attrs,
	}
	// 19 header + 4 section lengths + 4 withdrawn /24 + 13 mandatory attrs
	// + 4008 communities + 3 MP header + 21 MP fixed fields + 17 /128 = 4089.
	// After adding the LL, the splitter reserves four MP-header octets:
	// 4096 - 23 - (13+4008) - 4 - 37 = 11 NLRI octets, fewer than 17.
	if replacement.Len(nil) != 4089 {
		t.Fatalf("replacement frame length = %d, want 4089", replacement.Len(nil))
	}
	encoded := make([]byte, replacement.Len(nil))
	if got := replacement.WriteTo(encoded, 0, nil); got != len(encoded) {
		t.Fatalf("replacement encoded length = %d, want %d", got, len(encoded))
	}
	r.api = &pluginserver.Server{}
	policyCalls := 0
	r.policyFilterSeam = func(_, _, _, _ string, _ uint32, _ string) PolicyResponse {
		policyCalls++
		if policyCalls == 1 {
			return PolicyResponse{Action: PolicyModify, Raw: encoded[19:]}
		}
		return PolicyResponse{Action: PolicyAccept}
	}
	api := &reactorAPIAdapter{r: r}
	routes := []*rib.Route{
		rib.NewRouteWithASPath(manyIPv6Host(1)[0], peer.settings.LocalAddress, nil, nil),
	}
	result, err := api.SendRoutes(selector.All(), routes, nil, false, plugin.OperatorSender())
	// SendRoutes reports a per-peer refusal in its result; it need not return
	// an aggregate error for a peer that emitted only part of the operation.
	if err != nil {
		t.Logf("partial commit also returned an aggregate error: %v", err)
	}
	wantWithdrawal := mustHex(t, "ffffffffffffffffffffffffffffffff001b02000418c633640000")
	if got := conn.written(); !bytes.Equal(got, wantWithdrawal) {
		t.Errorf("partial commit wire = %x, want exactly one complete withdrawal %x", got, wantWithdrawal)
	}
	if result.RoutesQueued != 1 || result.RoutesAnnounced != 0 || result.RoutesWithdrawn != 1 || result.UpdatesSent != 1 || result.EORSent != 0 {
		t.Errorf("partial commit result = %+v, want one actual withdrawal UPDATE, no announcement or EOR", result)
	}
	if len(result.Peers) != 1 {
		t.Fatalf("partial commit peer rows = %d, want 1", len(result.Peers))
	}
	row := result.Peers[0]
	if row.RoutesAnnounced != 0 || row.RoutesWithdrawn != 1 || row.UpdatesSent != 1 || row.EORSent != 0 {
		t.Errorf("partial commit peer result = %+v, want actual withdrawal-only accounting", row)
	}
	refused := false
	for _, reason := range row.Reasons {
		if reason == bgptypes.CommitReasonAnnounceRefused {
			refused = true
		}
	}
	if !refused {
		t.Errorf("partial commit reasons = %v, want announce-refused", row.Reasons)
	}
	if stats := peer.Stats(); stats.UpdatesSent != 1 || stats.EORSent != 0 {
		t.Errorf("partial wire counters = %d/%d, want 1/0", stats.UpdatesSent, stats.EORSent)
	}
	if session.writePending || peer.adjOut.pending {
		t.Error("partial commit left a pending write or ownership receipt")
	}
	if len(peer.adjOut.routes) != 0 {
		t.Errorf("refused IPv6 route earned ownership: %d entries", len(peer.adjOut.routes))
	}
	if policyCalls != 1 {
		t.Errorf("partial commit policy calls = %d, want 1", policyCalls)
	}

	before := len(conn.written())
	batch := bgptypes.NLRIBatch{
		Family:  family.IPv6Unicast,
		NLRIs:   manyIPv6Host(1),
		NextHop: bgptypes.NewNextHopExplicit(peer.settings.LocalAddress),
	}
	if err := api.AnnounceNLRIBatch(t.Context(), selector.All(), batch, plugin.OperatorSender()); err != nil {
		t.Fatalf("healthy subsequent announcement refused: %v", err)
	}
	ordinaryGrowthWire(t, conn.written()[before:], 4096, wantNLRI, 0)
	if policyCalls != 2 {
		t.Errorf("policy calls after subsequent send = %d, want 2", policyCalls)
	}
	if session.writeFailed != nil || session.tearingDown.Load() || session.fsm.State() != fsm.StateEstablished {
		t.Errorf("route-scoped split refusal damaged session: error=%v teardown=%v state=%v", session.writeFailed, session.tearingDown.Load(), session.fsm.State())
	}
	if len(peer.adjOut.routes) != 1 {
		t.Errorf("subsequent route ownership count = %d, want 1", len(peer.adjOut.routes))
	}
}

// ordinaryGrowthUpdate encodes the same hosts as manyIPv6Host without using the
// writer or next-hop rewriter as the expected-wire oracle. There is no ADD-PATH.
func ordinaryGrowthUpdate(t *testing.T, routes int) (*message.Update, []byte) {
	t.Helper()
	nlri := make([]byte, 17*routes)
	for i := range routes {
		off := 17 * i
		nlri[off] = 128
		copy(nlri[off+1:off+5], []byte{0x20, 0x01, 0x0d, 0xb8})
		binary.BigEndian.PutUint16(nlri[off+15:off+17], uint16(i))
	}
	value := append([]byte{0, 2, 1, 16}, netip.MustParseAddr("2001:db8:a::1").AsSlice()...)
	value = append(value, 0)
	value = append(value, nlri...)
	attrs := mustHex(t, "4001010040020602010000fde8")
	attrs = append(attrs, 0x90, 14, byte(len(value)>>8), byte(len(value)))
	attrs = append(attrs, value...)
	return &message.Update{PathAttributes: attrs}, nlri
}

// ordinaryGrowthWire validates every complete frame independently, preserving
// ordered NLRI bytes so a missing, duplicated or reordered route cannot pass.
func ordinaryGrowthWire(t *testing.T, stream []byte, ceiling int, wantNLRI []byte, wantEOR uint32) int {
	t.Helper()
	var gotNLRI []byte
	var eors uint32
	frames := 0
	for len(stream) != 0 {
		if len(stream) < 19 {
			t.Fatalf("trailing partial header: %d octets", len(stream))
		}
		if !bytes.Equal(stream[:16], bytes.Repeat([]byte{0xff}, 16)) {
			t.Fatal("invalid BGP marker")
		}
		length := int(binary.BigEndian.Uint16(stream[16:18]))
		if length < 23 {
			t.Fatalf("UPDATE frame length = %d", length)
		}
		if length > ceiling {
			t.Fatalf("frame length %d exceeds negotiated ceiling %d", length, ceiling)
		}
		if length > len(stream) {
			t.Fatalf("frame length %d exceeds remaining stream %d", length, len(stream))
		}
		if stream[18] != 2 {
			t.Fatalf("message type = %d, want UPDATE", stream[18])
		}
		frame := stream[:length]
		stream = stream[length:]
		if bytes.Equal(frame, eorWire(family.IPv6Unicast)) {
			eors++
			continue
		}
		body := frame[19:]
		sections, err := wire.ParseUpdateSections(body)
		if err != nil {
			t.Fatalf("invalid emitted UPDATE: %v", err)
		}
		if len(sections.Withdrawn(body)) != 0 {
			t.Fatal("unexpected withdrawn routes")
		}
		if len(sections.NLRI(body)) != 0 {
			t.Fatal("unexpected trailing IPv4 NLRI")
		}
		attrs := sections.Attrs(body)
		fixed := mustHex(t, "4001010040020602010000fde8")
		if !bytes.HasPrefix(attrs, fixed) {
			t.Fatal("ORIGIN or local-AS prepend changed")
		}
		_, _, mp, found := attribute.AttrFind(attrs, attribute.AttrMPReachNLRI)
		if !found {
			t.Fatal("emitted route frame lacks MP_REACH")
		}
		wantPrefix := append([]byte{0, 2, 1, 32}, netip.MustParseAddr("2001:db8:a::1").AsSlice()...)
		wantPrefix = append(wantPrefix, netip.MustParseAddr("fe80::1").AsSlice()...)
		wantPrefix = append(wantPrefix, 0)
		if !bytes.HasPrefix(mp, wantPrefix) {
			t.Fatal("MP_REACH lacks exact IPv6 global + owned link-local + reserved field")
		}
		header := 3
		if attrs[len(fixed)]&0x10 != 0 {
			header = 4
		}
		if len(attrs) != len(fixed)+header+len(mp) {
			t.Fatal("unexpected extra or incomplete path attributes")
		}
		if len(mp) == len(wantPrefix) {
			t.Fatal("split emitted empty reachable chunk")
		}
		gotNLRI = append(gotNLRI, mp[len(wantPrefix):]...)
		frames++
	}
	if !bytes.Equal(gotNLRI, wantNLRI) {
		t.Errorf("delivered NLRI differs: got %d octets, want %d (every intended route exactly once)", len(gotNLRI), len(wantNLRI))
	}
	if eors != wantEOR {
		t.Errorf("wire EOR count = %d, want %d", eors, wantEOR)
	}
	return frames
}
