// Design: docs/architecture/core-design.md -- conditional defaults and stale readvertisement.
package reactor

import (
	"bytes"
	"context"
	"encoding/json"
	"net/netip"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	plugipc "github.com/ze-software/ze/internal/component/plugin/ipc"
	routepolicy "github.com/ze-software/ze/internal/component/policy"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/bgp/wire"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// writerBoundaryTransport keeps declarations local while calls use the actual
// typed bridge or encoded filter-update dispatcher. Only stale opts in.
type writerBoundaryTransport struct {
	*answerTransport
	conn *plugipc.PluginConn
}

// FilterReadvertise opts "stale" and "stale-too" in, as llgr-egress does, and
// "rail-only", a third party's filter that asks for the stale rail and no more.
func (w *writerBoundaryTransport) FilterReadvertise(plugin, filter string) (bool, bool) {
	return filter == "stale" || filter == "stale-too" || filter == "rail-only", plugin == answerPlugin
}

// FilterJudgesFinal has "stale" and "stale-too" run again over a rewritten body.
func (w *writerBoundaryTransport) FilterJudgesFinal(plugin, filter string) (bool, bool) {
	return filter == "stale" || filter == "stale-too", plugin == answerPlugin
}

func (w *writerBoundaryTransport) CallFilterUpdate(ctx context.Context, _ string, route *rpc.FilterRoute) (*rpc.FilterUpdateOutput, error) {
	return w.conn.CallFilter(ctx, route)
}

// writerBoundaryPeer binds the exact same send encoding to peer and session,
// as OPEN completion does, and installs the chain and normalization of the originated
// export gate through exportFilterForBody. Production installs Peer.egressGate,
// which also stamps the write.
func writerBoundaryPeer(t *testing.T, r *Reactor) (*Peer, *recordingConn) {
	t.Helper()
	peer, conn := newGroupUpdatesPeer(t, "10.0.0.2", "absent")
	encoding := bgpctx.EncodingContextForASN4(true)
	id, err := bgpctx.Registry.Register(encoding)
	if err != nil {
		t.Fatal(err)
	}
	peer.sendCtx.Store(encoding)
	peer.sendCtxID = id
	peer.session.setSendCtxID(id)
	peer.session.adjOut = &peer.adjOut
	peer.settings.LocalAddress = netip.MustParseAddr("10.0.0.1")
	peer.reactor = r
	peer.session.egressRouteFilter = func(body []byte) (bool, []byte, []byte) {
		return r.exportFilterForBody(peer, body)
	}
	return peer, conn
}

// TestDefaultConditionSetsGateWithoutMutatingDefault proves a condition's valid
// ordinary sets can be realized without suppressing the default, but remain a
// condition, not an attribute policy. The actual default reaches the socket
// under its separate full export chain. Reject and call failure still withhold.
func TestDefaultConditionSetsGateWithoutMutatingDefault(t *testing.T) {
	for _, typed := range []bool{false, true} {
		transportName := "forked"
		if typed {
			transportName = "typed"
		}
		for _, verdict := range []string{"sets", "reject", "error"} {
			t.Run(transportName+"/"+verdict, func(t *testing.T) {
				var calls []string
				var callsMu sync.Mutex
				judge := func(input rpc.FilterUpdateInput) (*rpc.FilterUpdateOutput, error) {
					callsMu.Lock()
					calls = append(calls, input.Filter)
					callsMu.Unlock()
					if input.Filter == "export" {
						out := answerSet("med", "42")
						return &out, nil
					}
					if input.Filter != "condition" {
						return &rpc.FilterUpdateOutput{Action: rpc.FilterAccept, Error: "unexpected condition member"}, nil
					}
					if string(input.Fields) != `{"prefix":{"ipv4/unicast":["0.0.0.0/0"]}}` {
						return &rpc.FilterUpdateOutput{Action: rpc.FilterAccept, Error: "condition did not read the actual default prefix"}, nil
					}
					if verdict == "error" {
						return &rpc.FilterUpdateOutput{Action: rpc.FilterAccept, Error: "condition unavailable"}, nil
					}
					if verdict == "reject" {
						return &rpc.FilterUpdateOutput{Action: rpc.FilterReject}, nil
					}
					return &rpc.FilterUpdateOutput{Action: rpc.FilterModify, Sets: []rpc.FilterSet{
						{Field: "local-preference", Action: rpc.FilterSetActionSet, Value: json.RawMessage("222")},
						{Field: "med", Action: rpc.FilterSetActionSet, Value: json.RawMessage("77")},
						{Field: "community", Action: rpc.FilterSetActionAdd, Value: json.RawMessage(`"65000:77"`)},
					}}, nil
				}
				transport := &writerBoundaryTransport{answerTransport: &answerTransport{
					reads: []string{"prefix"}, sets: []string{"local-preference", "med", "community"}},
					conn: errorVerdictConnection(t, typed, judge)}
				r := &Reactor{api: liveFilterServer(t), attrModHandlers: attrModHandlersWithDefaults(), filterTransportSeam: transport}
				peer, conn := writerBoundaryPeer(t, r)
				peer.settings.ExportPolicy = answerPolicyAt(t, "bgp export", routepolicy.FinalAccept, "export")
				peer.settings.DefaultOriginate = map[string]bool{"ipv4/unicast": true}
				peer.settings.DefaultOriginateFilter = map[string]string{"ipv4/unicast": "test:condition"}
				peer.sendDefaultOriginateRoutes(peer.testInitialSync(), peer.negotiated.Load())
				callsMu.Lock()
				defer callsMu.Unlock()
				bodies := updateBodies(t, conn.written())
				if verdict != "sets" {
					if len(bodies) != 0 || !slices.Equal(calls, []string{"condition"}) {
						t.Fatalf("refused condition emitted %x with calls %v", bodies, calls)
					}
					return
				}
				if len(bodies) != 1 {
					t.Fatalf("valid condition sets emitted %d defaults, want 1; calls=%v", len(bodies), calls)
				}
				wantBody := nlriHex(t, "0000001b4001010040020602010000fde84003040a0000018004040000002a00")
				if !bytes.Equal(bodies[0], wantBody) {
					t.Fatalf("default UPDATE=%x, want exact export-only attributes %x", bodies[0], wantBody)
				}
				sections, err := wire.ParseUpdateSections(bodies[0])
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(sections.NLRI(bodies[0]), []byte{0}) || len(sections.Withdrawn(bodies[0])) != 0 {
					t.Fatalf("default reachability = %x", bodies[0])
				}
				attrs := sections.Attrs(bodies[0])
				_, _, med, found := attribute.AttrFind(attrs, attribute.AttrMED)
				if !found || !bytes.Equal(med, []byte{0, 0, 0, 42}) {
					t.Fatalf("default MED=%x, want separate export set 42", med)
				}
				for _, code := range []attribute.AttributeCode{attribute.AttrLocalPref, attribute.AttrCommunity} {
					if _, _, value, found := attribute.AttrFind(attrs, code); found {
						t.Fatalf("condition-only attribute %d leaked: %x", code, value)
					}
				}
				if !slices.Equal(calls, []string{"condition", "export"}) {
					t.Fatalf("filter calls=%v, want condition then full export", calls)
				}
			})
		}
	}
}

// TestStaleWriterDoesNotReenterExport drives the full stale send branch through
// both transports and the session writer. The opted-in prepend runs once, the
// non-opted rejecting member never runs, and keep/withdraw preserve exact NLRI.
func TestStaleWriterDoesNotReenterExport(t *testing.T) {
	for _, typed := range []bool{false, true} {
		transportName := "forked"
		if typed {
			transportName = "typed"
		}
		for _, verdict := range []string{"prepend", "keep", "withdraw"} {
			t.Run(transportName+"/"+verdict, func(t *testing.T) {
				var calls []string
				var callsMu sync.Mutex
				judge := func(input rpc.FilterUpdateInput) (*rpc.FilterUpdateOutput, error) {
					callsMu.Lock()
					calls = append(calls, input.Filter)
					callsMu.Unlock()
					if input.Filter != "stale" {
						return &rpc.FilterUpdateOutput{Action: rpc.FilterReject}, nil
					}
					if verdict == "withdraw" {
						return &rpc.FilterUpdateOutput{Action: rpc.FilterWithdraw}, nil
					}
					if verdict == "keep" {
						return &rpc.FilterUpdateOutput{Action: rpc.FilterAccept}, nil
					}
					return &rpc.FilterUpdateOutput{Action: rpc.FilterModify, Sets: []rpc.FilterSet{
						{Field: "as-path", Action: rpc.FilterSetActionPrepend, Value: json.RawMessage("1")},
					}}, nil
				}
				transport := &writerBoundaryTransport{answerTransport: &answerTransport{
					reads: []string{"prefix"}, sets: []string{"as-path"}}, conn: errorVerdictConnection(t, typed, judge)}
				r := &Reactor{attrModHandlers: attrModHandlersWithDefaults(), filterTransportSeam: transport}
				peer, conn := writerBoundaryPeer(t, r)
				peer.settings.ExportPolicy = answerPolicyAt(t, "bgp export", routepolicy.FinalAccept, "stale", "ordinary")
				batch := staleReadvertiseBatch(t)
				a := &reactorAPIAdapter{r: r}
				target := announceTarget{peer: peer, session: peer.session}
				// A withdrawal can remove only an advertisement held on this session.
				if verdict == "withdraw" {
					peer.session.egressRouteFilter = nil
					facts := announceFactsFor(peer, batch.Family, netip.MustParseAddr("10.0.0.1"), false, peer.negotiated.Load())
					initial, err := a.buildBatchAnnounceUpdate(make([]byte, 4096), make([]byte, 4096), batch, facts)
					if err != nil {
						t.Fatal(err)
					}
					if err := target.sendBatchUpdate(t.Context(), initial, 4096, false, batch); err != nil {
						t.Fatal(err)
					}
					peer.session.egressRouteFilter = func(body []byte) (bool, []byte, []byte) { return r.exportFilterForBody(peer, body) }
				}
				before := len(conn.written())
				sent, err := a.sendStaleReadvertise(t.Context(), target, batch, netip.MustParseAddr("10.0.0.1"), false, peer.negotiated.Load())
				callsMu.Lock()
				defer callsMu.Unlock()
				if err != nil || sent != 1 {
					t.Fatalf("send=%d err=%v, want one accepted stale output", sent, err)
				}
				bodies := updateBodies(t, conn.written()[before:])
				if len(bodies) != 1 {
					t.Fatalf("stale output=%x calls=%v, want one UPDATE", bodies, calls)
				}
				wantBody := "000000184001010040020a02020000fde80000fde94003040a000001180a0000"
				if verdict == "prepend" {
					wantBody = "0000001c4001010040020e02030000fde80000fde80000fde94003040a000001180a0000"
				}
				if verdict == "withdraw" {
					wantBody = "0004180a00000000"
				}
				if !bytes.Equal(bodies[0], nlriHex(t, wantBody)) {
					t.Fatalf("stale UPDATE=%x, want exact survivor attributes and reachability %s", bodies[0], wantBody)
				}
				sections, err := wire.ParseUpdateSections(bodies[0])
				if err != nil {
					t.Fatal(err)
				}
				if verdict == "withdraw" {
					if !bytes.Equal(sections.Withdrawn(bodies[0]), tenSlash24NLRI) || len(sections.NLRI(bodies[0])) != 0 {
						t.Fatalf("stale withdrawal=%x", bodies[0])
					}
				} else {
					if !bytes.Equal(sections.NLRI(bodies[0]), tenSlash24NLRI) || len(sections.Withdrawn(bodies[0])) != 0 {
						t.Fatalf("stale survivor reachability=%x", bodies[0])
					}
					attrs := sections.Attrs(bodies[0])
					_, _, path, found := attribute.AttrFind(attrs, attribute.AttrASPath)
					want := "02020000fde80000fde9"
					if verdict == "prepend" {
						want = "02030000fde80000fde80000fde9"
					}
					if !found || !bytes.Equal(path, nlriHex(t, want)) {
						t.Fatalf("AS_PATH=%x, want exactly %s", path, want)
					}
					_, _, nextHop, found := attribute.AttrFind(attrs, attribute.AttrNextHop)
					if !found || !bytes.Equal(nextHop, []byte{10, 0, 0, 1}) {
						t.Fatalf("stale survivor next hop=%x", nextHop)
					}
				}
				if !slices.Equal(calls, []string{"stale"}) {
					t.Fatalf("filter calls=%v, want exactly one opted-in invocation", calls)
				}
			})
		}
	}
}

// TestStaleWriterRetainsNextHopAdmission keeps the prefiltered callback honest:
// raw stale output still trims a link-local address without a proven common
// subnet, retains a permitted pair, and never hides a peer-owned next hop.
func TestStaleWriterRetainsNextHopAdmission(t *testing.T) {
	const global = "20010db8000100000000000000000001"
	for _, typed := range []bool{false, true} {
		transportName := "forked"
		if typed {
			transportName = "typed"
		}
		for _, tc := range []struct {
			name      string
			peer      string
			linkLocal string
			common    bool
			withheld  bool
		}{
			{"unproven subnet trims link-local", "2001:db8:2::2", "fe800000000000000000000000000001", false, false},
			{"common subnet preserves pair", "2001:db8:1::2", "fe800000000000000000000000000001", true, false},
			{"peer-owned link-local is withheld", "fe80::2", "fe800000000000000000000000000002", false, true},
		} {
			t.Run(transportName+"/"+tc.name, func(t *testing.T) {
				raw := nlriHex(t, "0000003e800e2e00020120"+global+tc.linkLocal+"004020010db8000700004001010040020602010000fde8")
				var calls atomic.Int32
				transport := &writerBoundaryTransport{answerTransport: &answerTransport{reads: []string{rpc.FilterFieldRaw}},
					conn: errorVerdictConnection(t, typed, func(rpc.FilterUpdateInput) (*rpc.FilterUpdateOutput, error) {
						calls.Add(1)
						return &rpc.FilterUpdateOutput{Action: rpc.FilterModify, Raw: raw}, nil
					})}
				r := &Reactor{attrModHandlers: attrModHandlersWithDefaults(), filterTransportSeam: transport}
				peer, conn := writerBoundaryPeer(t, r)
				peer.settings.Address = netip.MustParseAddr(tc.peer)
				peer.settings.LocalAddress = netip.MustParseAddr("2001:db8:1::1")
				peer.settings.ExportPolicy = answerPolicyAt(t, "bgp export", routepolicy.FinalAccept, "stale")
				var connected []netip.Prefix
				if tc.common {
					connected = []netip.Prefix{netip.MustParsePrefix("2001:db8:1::/64")}
				}
				peer.llScope.Store(newLinkScopeWith(connected, neighborView{}, peer.settings.Address))
				peer.negotiated.Load().families[family.IPv6Unicast] = true
				batch := staleReadvertiseBatch(t)
				batch.Family = family.IPv6Unicast
				batch.NLRIs = []nlri.NLRI{nlri.NewINET(family.IPv6Unicast, netip.MustParsePrefix("2001:db8:7::/64"), 0, false)}
				a := &reactorAPIAdapter{r: r}
				_, err := a.sendStaleReadvertise(t.Context(), announceTarget{peer: peer, session: peer.session},
					batch, netip.MustParseAddr("2001:db8:1::1"), false, peer.negotiated.Load())
				if err != nil {
					t.Fatal(err)
				}
				bodies := updateBodies(t, conn.written())
				if tc.withheld {
					if len(bodies) != 0 {
						t.Fatalf("peer-owned next hop was hidden or advertised: %x", bodies)
					}
				} else {
					if len(bodies) != 1 {
						t.Fatalf("permitted stale route emitted %d UPDATEs, want 1", len(bodies))
					}
					want := nlriHex(t, "0000002e800e1e00020110"+global+"004020010db8000700004001010040020602010000fde8")
					if tc.common {
						want = raw
					}
					if !bytes.Equal(bodies[0], want) {
						t.Fatalf("final stale UPDATE=%x, want exact scoped output %x", bodies[0], want)
					}
				}
				if calls.Load() != 1 {
					t.Fatalf("stale raw member ran %d times, want once", calls.Load())
				}
			})
		}
	}
}

// TestStaleWriterShapesRawRewrite retains the policy-output shape contract:
// a mixed raw result withdraws a previously advertised route and announces its
// permitted survivor in separate messages without evaluating the member twice.
func TestStaleWriterShapesRawRewrite(t *testing.T) {
	for _, typed := range []bool{false, true} {
		name := "forked"
		if typed {
			name = "typed"
		}
		t.Run(name, func(t *testing.T) {
			raw := plainSetsBody(t, []string{"40010100", "40020602010000fde8", "4003040a000001"})
			var calls atomic.Int32
			transport := &writerBoundaryTransport{answerTransport: &answerTransport{reads: []string{rpc.FilterFieldRaw}},
				conn: errorVerdictConnection(t, typed, func(rpc.FilterUpdateInput) (*rpc.FilterUpdateOutput, error) {
					calls.Add(1)
					return &rpc.FilterUpdateOutput{Action: rpc.FilterModify, Raw: raw}, nil
				})}
			r := &Reactor{attrModHandlers: attrModHandlersWithDefaults(), filterTransportSeam: transport}
			peer, conn := writerBoundaryPeer(t, r)
			peer.settings.ExportPolicy = answerPolicyAt(t, "bgp export", routepolicy.FinalAccept, "stale")
			gate := peer.session.egressRouteFilter
			peer.session.egressRouteFilter = nil
			seed := ownershipWriterUpdate(t, nlriHex(t, "000000144001010040020602010000fde84003040a00000118cb0071"))
			if err := peer.session.SendUpdate(seed); err != nil {
				t.Fatal(err)
			}
			peer.session.egressRouteFilter = gate
			before := len(conn.written())
			batch := staleReadvertiseBatch(t)
			a := &reactorAPIAdapter{r: r}
			_, err := a.sendStaleReadvertise(t.Context(), announceTarget{peer: peer, session: peer.session},
				batch, netip.MustParseAddr("10.0.0.1"), false, peer.negotiated.Load())
			if err != nil {
				t.Fatal(err)
			}
			bodies := updateBodies(t, conn.written()[before:])
			want := []string{"000418cb00710000", "000000144001010040020602010000fde84003040a00000118c63364"}
			if len(bodies) != len(want) {
				t.Fatalf("raw stale output=%x, want withdrawal and survivor", bodies)
			}
			for i, body := range bodies {
				if !bytes.Equal(body, nlriHex(t, want[i])) {
					t.Fatalf("raw stale output[%d]=%x, want %s", i, body, want[i])
				}
			}
			if calls.Load() != 1 {
				t.Fatalf("raw stale member ran %d times, want once", calls.Load())
			}
		})
	}
}

// TestStaleWriterQueuedOutput preserves captured sent-owner authority and the
// overflow byte-ownership boundary. A held real worker releases the decided
// prepend or withdrawal only after the initial-sync gate opens.
func TestStaleWriterQueuedOutput(t *testing.T) {
	for _, typed := range []bool{false, true} {
		name := "forked"
		if typed {
			name = "typed"
		}
		for _, withdraw := range []bool{false, true} {
			verdict := "prepend"
			if withdraw {
				verdict = "withdraw"
			}
			t.Run(name+"/"+verdict, func(t *testing.T) {
				var calls atomic.Int32
				transport := &writerBoundaryTransport{answerTransport: &answerTransport{
					reads: []string{"prefix"}, sets: []string{"as-path"}},
					conn: errorVerdictConnection(t, typed, func(input rpc.FilterUpdateInput) (*rpc.FilterUpdateOutput, error) {
						calls.Add(1)
						if input.Filter != "stale" {
							return &rpc.FilterUpdateOutput{Action: rpc.FilterReject}, nil
						}
						if withdraw {
							return &rpc.FilterUpdateOutput{Action: rpc.FilterWithdraw}, nil
						}
						return &rpc.FilterUpdateOutput{Action: rpc.FilterModify, Sets: []rpc.FilterSet{
							{Field: "as-path", Action: rpc.FilterSetActionPrepend, Value: json.RawMessage("1")},
						}}, nil
					})}
				r := &Reactor{attrModHandlers: attrModHandlersWithDefaults(), filterTransportSeam: transport}
				peer, conn := writerBoundaryPeer(t, r)
				peer.settings.ExportPolicy = answerPolicyAt(t, "bgp export", routepolicy.FinalAccept, "stale", "ordinary")
				a := &reactorAPIAdapter{r: r}
				batch := staleReadvertiseBatch(t)
				facts := announceFactsFor(peer, batch.Family, netip.MustParseAddr("10.0.0.1"), false, peer.negotiated.Load())
				seed, err := a.buildBatchAnnounceUpdate(make([]byte, 4096), make([]byte, 4096), batch, facts)
				if err != nil {
					t.Fatal(err)
				}
				gate := peer.session.egressRouteFilter
				peer.session.egressRouteFilter = nil
				if err := peer.session.SendUpdate(seed); err != nil {
					t.Fatal(err)
				}
				peer.session.egressRouteFilter = gate
				batch.SentOwnerMessage = peer.session.sentReceipt.MessageID()
				if batch.SentOwnerMessage == 0 {
					t.Fatal("actual seed write produced no ownership receipt")
				}
				batch.Replay = true
				done := make(chan struct{}, 1)
				pool := newFwdPool(func(key fwdKey, items []fwdItem) {
					fwdBatchHandler(key, items)
					for i := range items {
						if len(items[i].rawBodies) != 0 {
							done <- struct{}{}
							return
						}
					}
				}, fwdPoolConfig{chanSize: 8, idleTimeout: time.Second})
				r.fwdPool = pool
				peer.testInitialSync().sendingInitialRoutes.Store(1)
				t.Cleanup(func() {
					peer.testInitialSync().sendingInitialRoutes.Store(0)
					peer.wakeForwardOverflow()
					pool.Stop()
				})
				before := len(conn.written())
				sent, err := a.sendStaleReadvertise(t.Context(), announceTarget{peer: peer, session: peer.session},
					batch, netip.MustParseAddr("10.0.0.1"), false, peer.negotiated.Load())
				if err != nil || sent != 1 {
					t.Fatalf("queued stale send=%d err=%v, want one accepted unit", sent, err)
				}
				if len(conn.written()) != before {
					t.Fatal("stale body bypassed the held forward order")
				}
				peer.testInitialSync().sendingInitialRoutes.Store(0)
				peer.wakeForwardOverflow()
				select {
				case <-done:
				case <-time.After(5 * time.Second):
					t.Fatal("released stale overflow never completed its real writer")
				}
				bodies := updateBodies(t, conn.written()[before:])
				want := "0000001c4001010040020e02030000fde80000fde80000fde94003040a000001180a0000"
				if withdraw {
					want = "0004180a00000000"
				}
				if len(bodies) != 1 {
					t.Fatalf("queued stale output=%x, want one UPDATE", bodies)
				}
				if !bytes.Equal(bodies[0], nlriHex(t, want)) {
					t.Fatalf("queued stale output=%x, want exact %s", bodies[0], want)
				}
				if calls.Load() != 1 {
					t.Fatalf("queued stale filter calls=%d, want one opted member", calls.Load())
				}
			})
		}
	}
}
