// Design: docs/architecture/api/process-protocol.md -- policy dry-run exposes the effective decision.
// RFC naming: untagged -- red real-entry policy probes; requirement tags await implementation and discrimination proof.
package reactor

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"net/netip"
	"reflect"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/component/plugin"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/core/bgp/wire"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestRFC8212PolicyDryRunWiring drives the existing PolicyDryRunner entry point,
// not a proposed verdict signature. JSON assertions also cover the consumer
// boundary without requiring fields that production has not implemented yet.
func TestRFC8212PolicyDryRunWiring(t *testing.T) {
	cases := []struct {
		name       string
		remoteAS   uint32
		dynamic    bool
		final      filterapi.FinalDisposition
		wantAction string
		wantFinal  string
		wantSource string
	}{
		{name: "absent-ebgp", remoteAS: 65001, wantAction: "reject", wantFinal: "reject", wantSource: "rfc8212"},
		{name: "absent-ibgp", remoteAS: 65000, wantAction: "accept"},
		{name: "explicit-accept", remoteAS: 65001, final: filterapi.FinalAccept, wantAction: "accept", wantFinal: "accept"},
		{name: "explicit-reject", remoteAS: 65001, final: filterapi.FinalReject, wantAction: "reject", wantFinal: "reject"},
		{name: "unresolved-dynamic", dynamic: true, wantAction: "undetermined", wantFinal: "undetermined"},
	}
	for _, tc := range cases {
		for _, direction := range []string{directionImport, directionExport} {
			t.Run(tc.name+"/"+direction, func(t *testing.T) {
				var policy filterapi.Policy
				if tc.final != 0 {
					policy = mustPolicy(t, nil, tc.final)
				}
				addr := netip.MustParseAddr("192.0.2.1")
				peer := NewPeer(&PeerSettings{
					Address: addr, LocalAS: 65000, PeerAS: tc.remoteAS,
					IsDynamic: tc.dynamic, ImportPolicy: policy, ExportPolicy: policy,
				})
				r := &Reactor{peers: map[netip.AddrPort]*Peer{peer.Settings().PeerKey(): peer}}
				api := &reactorAPIAdapter{r: r}
				result, err := api.PolicyDryRun(addr.String(), direction, "", policyChainAdvertisingBody(), true)
				if err != nil {
					t.Fatal(err)
				}
				if result.Action != tc.wantAction {
					t.Errorf("action = %q, want %q", result.Action, tc.wantAction)
				}
				if tc.wantAction != "accept" {
					if result.TextAfter != "" {
						t.Errorf("non-accepting prediction produced route %q", result.TextAfter)
					}
				}
				if tc.dynamic {
					if len(result.Trace) != 0 {
						t.Errorf("unresolved session executed a filter trace: %+v", result.Trace)
					}
				}
				encoded, err := json.Marshal(result)
				if err != nil {
					t.Fatal(err)
				}
				var response struct {
					Policy *struct {
						Final  string `json:"final"`
						Source string `json:"final-source"`
					} `json:"policy"`
				}
				if err := json.Unmarshal(encoded, &response); err != nil {
					t.Fatal(err)
				}
				if tc.wantFinal == "" {
					if response.Policy != nil {
						t.Errorf("empty IBGP direction must remain omitted: %s", encoded)
					}
					return
				}
				if response.Policy == nil {
					t.Fatalf("missing effective policy context: %s", encoded)
				}
				if response.Policy.Final != tc.wantFinal {
					t.Errorf("policy final = %q, want %q", response.Policy.Final, tc.wantFinal)
				}
				if response.Policy.Source != tc.wantSource {
					t.Errorf("policy source = %q, want %q", response.Policy.Source, tc.wantSource)
				}
			})
		}
	}
}

// TestRFC8212ChainBoundaries compares real and traced chain execution on both
// session types. Neither an automatic reference nor a terminal raw rewrite can
// grant an absent EBGP policy acceptance, and failure/teardown remain terminal.
func TestRFC8212ChainBoundaries(t *testing.T) {
	automatic := filterapi.BindInProcess(filterapi.FilterRef{Name: "AUTO"}, "loop-detection")
	if !automatic.InProcess() {
		t.Fatal("loop registration did not bind the automatic reference")
	}
	for _, external := range []bool{false, true} {
		for _, final := range []filterapi.FinalDisposition{0, filterapi.FinalAccept, filterapi.FinalReject} {
			for _, tc := range []struct {
				name     string
				refs     []filterapi.FilterRef
				response PolicyResponse
				calls    int
				text     string
			}{
				{name: "empty", text: "origin igp"},
				{name: "inactive", refs: frefs("inactive:plug:SKIP"), text: "origin igp"},
				{name: "automatic", refs: []filterapi.FilterRef{automatic}, text: "origin igp"},
				{name: "accept", refs: frefs("plug:FIRST", "plug:LAST"), response: PolicyResponse{Action: PolicyAccept}, calls: 2, text: "origin igp"},
				{name: "modify", refs: frefs("plug:FIRST", "plug:LAST"), response: PolicyResponse{Action: PolicyModify, Delta: "med 200"}, calls: 2, text: "origin igp med 200"},
				{name: "inactive-external", refs: frefs("inactive:plug:SKIP", "plug:KEEP"), response: PolicyResponse{Action: PolicyAccept}, calls: 1, text: "origin igp"},
				{name: "raw", refs: frefs("plug:FIRST", "plug:LAST"), response: PolicyResponse{Action: PolicyModify, Raw: policyChainAdvertisingBody()}, calls: 1, text: "origin igp"},
				{name: "reject", refs: frefs("plug:FIRST", "plug:LAST"), response: PolicyResponse{Action: PolicyReject}, calls: 1},
				{name: "failure", refs: frefs("plug:FIRST", "plug:LAST"), response: PolicyResponse{Action: PolicyReject, Failed: true}, calls: 1},
				{name: "teardown", refs: frefs("plug:FIRST", "plug:LAST"), response: PolicyResponse{Action: PolicyAccept, Teardown: true, NotifyCode: 6, NotifySubcode: 2}, calls: 1},
			} {
				t.Run(tc.name, func(t *testing.T) {
					policy := filterapi.InternalPolicy(tc.refs)
					if final != 0 {
						policy = mustPolicy(t, tc.refs, final)
					}
					calls := 0
					call := func(_, _, _, _ string, _ uint32, _ string) PolicyResponse {
						calls++
						return tc.response
					}
					got := PolicyFilterChain(policy, external, directionImport, "192.0.2.1", 65001, "origin igp", call)
					if calls != tc.calls {
						t.Fatalf("runtime calls=%d want=%d", calls, tc.calls)
					}
					calls = 0
					traced, trace := tracePolicyFilterChain(policy, external, directionImport, "192.0.2.1", 65001, "origin igp", call)
					if calls != tc.calls || !reflect.DeepEqual(got, traced) {
						t.Fatalf("trace differs: runtime=%+v traced=%+v calls=%d want=%d", got, traced, calls, tc.calls)
					}
					wantTrace := tc.calls
					if tc.name == "automatic" {
						wantTrace = 1
						if len(trace) != 1 || trace[0].Action != "not-evaluated" || trace[0].TextAfter != "" {
							t.Fatalf("automatic check claimed execution: %+v", trace)
						}
					}
					if len(trace) != wantTrace {
						t.Fatalf("trace=%+v want %d entries", trace, wantTrace)
					}
					accept := final == filterapi.FinalAccept || final == 0 && !external
					reject := !accept || tc.response.Action == PolicyReject || tc.response.Teardown
					if (got.Action == PolicyReject) != reject || got.Failed != tc.response.Failed || got.Teardown != tc.response.Teardown {
						t.Fatalf("external=%v final=%d result=%+v", external, final, got)
					}
					if reject {
						if got.Text != "" || len(got.Raw) != 0 {
							t.Fatalf("rejection retained a route: %+v", got)
						}
					} else {
						if got.Text != tc.text || !bytes.Equal(got.Raw, tc.response.Raw) {
							t.Fatalf("acceptance changed result: %+v want text=%q", got, tc.text)
						}
					}
				})
			}
		}
	}
}

// TestRFC8212Section3AbsentImportRejectsOnEBGP checks the actual publication
// boundary: denied announcements never reach consumers, while explicit IPv4/MP
// withdrawals and EOR survive. Automatic references do not supply presence.
func TestRFC8212Section3AbsentImportRejectsOnEBGP(t *testing.T) {
	automatic := filterapi.BindInProcess(filterapi.FilterRef{Name: "AUTO"}, "loop-detection")
	if !automatic.InProcess() {
		t.Fatal("loop registration missing")
	}
	for _, refs := range [][]filterapi.FilterRef{nil, {automatic}, {{Name: "off", Inactive: true}}} {
		for _, tc := range []struct{ name, body, want string }{
			{"announcement", "0000000018c00003", ""},
			{"mp-announcement", "0000001f800e1c0002011020010db8000000000000000000000001003020010db80001", ""},
			{"withdrawal", "000418c000020000", "000418c000020000"},
			{"mixed", "000418c00002000018c00003", "000418c000020000"},
			{"mp-withdrawal", "0000000d800f0a0002013020010db80001", "0000000d800f0a0002013020010db80001"},
			{"mp-mixed", "0000001d800f0a0002013020010db80001800e0d00010104010101010018c00003", "0000000d800f0a0002013020010db80001"},
			{"eor", "00000000", "00000000"},
			{"mp-eor", "00000006800f03000201", "00000006800f03000201"},
		} {
			t.Run(tc.name, func(t *testing.T) {
				addr := netip.MustParseAddr("203.0.113.9")
				source := NewPeer(&PeerSettings{Address: addr, LocalAS: 65000, PeerAS: 65001,
					ImportPolicy: filterapi.InternalPolicy(refs)})
				source.state.Store(int32(PeerStateEstablished))
				var delivered [][]byte
				r := &Reactor{clock: source.clock, config: &Config{LocalAS: 65000},
					peers:               map[netip.AddrPort]*Peer{source.Settings().PeerKey(): source},
					recentUpdates:       newRecentUpdateCache(100),
					orderedIngressSteps: []orderedIngressStep{{name: policyChainStepName, policyChain: true}},
					messageReceiver: &testDeliveryReceiver{consumerCount: 1, onReceived: func(_ plugin.PeerInfo, msg bgptypes.RawMessage) {
						delivered = append(delivered, bytes.Clone(msg.RawBytes))
					}}}
				body := mustHex(t, tc.body)
				wu := wireu.NewWireUpdate(body, 0)
				wu.SetSourceID(source.SourceID())
				r.notifyMessageReceiver(addr, msgtype.TypeUPDATE, body, wu, 0, rpc.DirectionReceived, BufHandle{ID: noPoolBufID, Buf: body}, nil, "", 0)
				if tc.want == "" {
					if len(delivered) != 0 {
						t.Fatalf("denied announcement published: %x", delivered)
					}
					return
				}
				if len(delivered) != 1 || !bytes.Equal(delivered[0], mustHex(t, tc.want)) {
					t.Fatalf("published %x want %s", delivered, tc.want)
				}
			})
		}
	}
}

// TestRFC8212Section3AbsentExportRejectsOnEBGP exercises originated admission
// before and after snapshot publication, and the general forwarding policy step.
func TestRFC8212Section3AbsentExportRejectsOnEBGP(t *testing.T) {
	for _, refs := range [][]filterapi.FilterRef{nil, {{Name: "off", Inactive: true}}} {
		for _, snapshot := range []bool{false, true} {
			settings := NewPeerSettings(netip.MustParseAddr("192.0.2.2"), 65000, 65001, 1)
			settings.ExportPolicy = filterapi.InternalPolicy(refs)
			peer := NewPeer(settings)
			if snapshot {
				peer.refreshForwardFacts()
			}
			r := &Reactor{}
			body := policyChainAdvertisingBody()
			if suppress, override := r.exportFilterForBody(peer, body); !suppress || override != nil {
				t.Fatalf("absent EBGP snapshot=%v exported %x", snapshot, override)
			}
			got := r.runEgressPolicyChain(settings.ExportPolicy, "192.0.2.2", 65001, 65000, false, testWireUpdate(body))
			if got.accept || got.failed {
				t.Fatalf("general forwarding did not give policy rejection: %+v", got)
			}
			for _, tc := range []struct{ body, want string }{
				{"000418c000020000", "000418c000020000"},
				{"000418c00002000018c00003", "000418c000020000"},
				{"0000000d800f0a0002013020010db80001", "0000000d800f0a0002013020010db80001"},
				{"00000000", "00000000"},
			} {
				suppress, override := r.exportFilterForBody(peer, mustHex(t, tc.body))
				if suppress || !bytes.Equal(override, mustHex(t, tc.want)) {
					t.Fatalf("withdrawal/EOR snapshot=%v suppress=%v body=%x want=%s", snapshot, suppress, override, tc.want)
				}
			}
		}
	}
}

// TestRFC8212AbsentPolicyAcceptsOnIBGP preserves ordinary and migration-AS IBGP
// on ingress and destination export, independent of the source AS-path encoding.
func TestRFC8212AbsentPolicyAcceptsOnIBGP(t *testing.T) {
	for _, migration := range []uint32{0, 65001} {
		remote := uint32(65000)
		if migration != 0 {
			remote = migration
		}
		settings := NewPeerSettings(netip.MustParseAddr("192.0.2.2"), 65000, remote, 1)
		settings.MigrationAS = migration
		peer := NewPeer(settings)
		r := &Reactor{}
		body := policyChainAdvertisingBody()
		update := testWireUpdate(body)
		if got := r.runIngressPolicyChain(peer, settings.Address, remote, update, body); !got.accept || got.modifiedPayload != nil {
			t.Fatalf("IBGP ingress migration=%d changed: %+v", migration, got)
		}
		for _, snapshot := range []bool{false, true} {
			if snapshot {
				peer.refreshForwardFacts()
			}
			if suppress, override := r.exportFilterForBody(peer, body); suppress || override != nil {
				t.Fatalf("IBGP export migration=%d snapshot=%v changed: %v %x", migration, snapshot, suppress, override)
			}
		}
		for _, asn4 := range []bool{false, true} {
			got := r.runEgressPolicyChainASN4(settings.ExportPolicy, "192.0.2.2", remote, 65000, settings.IsIBGP(), update, asn4)
			if !got.accept || got.failed || got.wireOverride != nil {
				t.Fatalf("source ASN4=%v changed IBGP destination: %+v", asn4, got)
			}
		}
	}
}

// TestRFC8212NativeRSSkipsAbsentExport checks both the initial scan and selected
// retry. A denied destination must return to the policy-aware general rail.
func TestRFC8212NativeRSSkipsAbsentExport(t *testing.T) {
	ctx := bgpctx.EncodingContextForASN4(true)
	ctxID, err := bgpctx.Registry.Register(ctx)
	if err != nil {
		t.Fatal(err)
	}
	source := makeRSPeer(t, "10.0.0.1", 65001, ctx, ctxID)
	destination := makeRSPeer(t, "10.0.0.2", 65002, ctx, ctxID)
	destination.settings.ExportPolicy = filterapi.Policy{}
	destination.refreshForwardFacts()
	update := &ReceivedUpdate{WireUpdate: wireu.NewWireUpdate(policyChainAdvertisingBody(), ctxID), SourcePeerIP: source.Settings().Address}
	r := &Reactor{peers: map[netip.AddrPort]*Peer{source.Settings().PeerKey(): source, destination.Settings().PeerKey(): destination}}
	skipped, dispatched := reactorForwardRS(r, update, 51, source.Settings().Address, source)
	if dispatched != 0 || len(skipped) != 1 || skipped[0] != destination.Settings().PeerKey() {
		t.Fatalf("absent final escaped fast path: skipped=%v dispatched=%d", skipped, dispatched)
	}
	skipped, dispatched, retry := reactorForwardRSSection(r, update, update.WireUpdate, []*Peer{destination}, 51, source.Settings().Address, source)
	if dispatched != 0 || len(retry) != 0 || len(skipped) != 1 || skipped[0] != destination.Settings().PeerKey() {
		t.Fatalf("absent final escaped retry: skipped=%v dispatched=%d retry=%v", skipped, dispatched, retry)
	}
}

// TestRFC8212DefaultOriginateGuard keeps the synthetic condition separate from
// operator presence, using a real external destination and filter callback.
func TestRFC8212DefaultOriginateGuard(t *testing.T) {
	for _, final := range []filterapi.FinalDisposition{0, filterapi.FinalAccept, filterapi.FinalReject} {
		for _, action := range []PolicyAction{PolicyAccept, PolicyReject} {
			settings := NewPeerSettings(netip.MustParseAddr("192.0.2.2"), 65000, 65001, 1)
			if final != 0 {
				settings.ExportPolicy = mustPolicy(t, nil, final)
			}
			peer := NewPeer(settings)
			calls := 0
			peer.reactor = &Reactor{api: liveFilterServer(t), policyFilterSeam: func(_, _, _, _ string, asn uint32, _ string) PolicyResponse {
				calls++
				if asn != 65001 {
					t.Errorf("guard changed destination AS: %d", asn)
				}
				return PolicyResponse{Action: action}
			}}
			got := peer.defaultOriginateFilterAccepts("test:guard", family.IPv4Unicast, netip.MustParsePrefix("0.0.0.0/0"), netip.MustParseAddr("192.0.2.254"))
			if got != (final == filterapi.FinalAccept && action == PolicyAccept) || calls != 1 {
				t.Fatalf("final=%d action=%d accepts=%v calls=%d", final, action, got, calls)
			}
		}
	}
	t.Run("unsupported-export-ownership", func(t *testing.T) {
		ref := filterapi.BindInProcess(filterapi.FilterRef{Name: "AUTO"}, "loop-detection")
		if !ref.InProcess() {
			t.Fatal("loop registration did not bind")
		}
		settings := NewPeerSettings(netip.MustParseAddr("192.0.2.2"), 65000, 65001, 1)
		settings.ExportPolicy = mustPolicy(t, []filterapi.FilterRef{ref}, filterapi.FinalAccept)
		peer := NewPeer(settings)
		peer.reactor = &Reactor{api: liveFilterServer(t), policyFilterSeam: func(_, _, _, _ string, _ uint32, _ string) PolicyResponse {
			t.Fatal("synthetic guard hid unsupported export ownership before its callback")
			return PolicyResponse{Action: PolicyAccept}
		}}
		if peer.defaultOriginateFilterAccepts("test:guard", family.IPv4Unicast, netip.MustParsePrefix("0.0.0.0/0"), netip.MustParseAddr("192.0.2.254")) {
			t.Fatal("synthetic condition bypassed unsupported export ownership")
		}
	})
}

// TestRFC8212UnknownDryRunNeverExecutes proves unknown type is not an EBGP guess,
// even with explicit policy or a single-filter override.
func TestRFC8212UnknownDryRunNeverExecutes(t *testing.T) {
	addr := netip.MustParseAddr("192.0.2.2")
	policy := mustPolicy(t, frefs("test:guard"), filterapi.FinalAccept)
	peer := NewPeer(&PeerSettings{Address: addr, LocalAS: 65000, IsDynamic: true, ImportPolicy: policy, ExportPolicy: policy})
	r := &Reactor{peers: map[netip.AddrPort]*Peer{peer.Settings().PeerKey(): peer}, policyFilterSeam: func(_, _, _, _ string, _ uint32, _ string) PolicyResponse {
		t.Fatal("unresolved session called a filter")
		return PolicyResponse{Action: PolicyReject, Failed: true}
	}}
	api := &reactorAPIAdapter{r: r}
	for _, direction := range []string{directionImport, directionExport} {
		for _, override := range []string{"", "guard"} {
			got, err := api.PolicyDryRun(addr.String(), direction, override, policyChainAdvertisingBody(), true)
			if err != nil {
				t.Fatal(err)
			}
			if got.Action != "undetermined" || got.Reason == "" || len(got.Trace) != 0 || got.TextBefore != "" || got.TextAfter != "" || len(got.ChangedAttrs) != 0 || len(got.WireChanges) != 0 {
				t.Fatalf("unknown session fabricated a result: %+v", got)
			}
		}
	}
}

// TestRFC8212GeneralForwardOwnedRejection drives cached acceptance, the general
// policy rail, and the actual writer. Absence withdraws only this source's old
// advertisement; another owner and unrelated destination slots survive.
func TestRFC8212GeneralForwardOwnedRejection(t *testing.T) {
	for _, owner := range []string{"same", "other", "none"} {
		t.Run(owner, func(t *testing.T) {
			f := ownershipRailNew(t, nil)
			f.r.orderedEgressSteps = []orderedEgressStep{{name: policyChainStepName, policyChain: true}}
			f.destination.mu.Lock()
			f.destination.settings.ExportPolicy = mustPolicy(t, nil, filterapi.FinalAccept)
			f.destination.settings.PeerAS = 65002
			f.destination.settings.RSClient = true
			f.destination.mu.Unlock()
			f.destination.refreshForwardFacts()
			f.destination.session.egressFinalAccepts = func() bool {
				facts := f.destination.forwardFacts()
				return facts.exportPolicy.AcceptsAtEnd(facts.isEBGP)
			}
			if owner != "none" {
				source := f.source
				if owner == "other" {
					source = f.other
				}
				f.send(t, "cached", source, f.ctxID, ownershipRailBody(family.IPv4Unicast, false, syncOrderPrefixWire, 1))
				f.await(t, 1, syncOrderPrefixWire)
			}
			unrelated := []byte{24, 203, 0, 113}
			f.send(t, "cached", f.source, f.ctxID, ownershipRailBody(family.IPv4Unicast, false, unrelated, 7))
			f.await(t, 7, unrelated)
			before := len(ownershipRailRead(t, f.conn.written(), f.destination.sendCtx.Load()))
			f.destination.mu.Lock()
			f.destination.settings.ExportPolicy = filterapi.Policy{}
			f.destination.mu.Unlock()
			f.destination.refreshForwardFacts()
			update := f.publish(t, f.source, f.ctxID, ownershipRailBody(family.IPv4Unicast, false, syncOrderPrefixWire, 2))
			api := &reactorAPIAdapter{r: f.r}
			if err := api.forwardUpdateCore(update, f.id, []*Peer{f.destination}, replayFenceSource(f.source, false)); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			if err := f.r.fwdPool.Barrier(ctx); err != nil {
				t.Fatal(err)
			}
			events := ownershipRailRead(t, f.conn.written(), f.destination.sendCtx.Load())[before:]
			if owner == "same" {
				if len(events) != 1 || !events[0].withdraw || events[0].key != string(syncOrderPrefixWire) {
					t.Fatalf("owned rejection did not withdraw only P: %+v", events)
				}
			} else if len(events) != 0 {
				t.Fatalf("rejection touched %s ownership or unrelated routes: %+v", owner, events)
			}
		})
	}
}

// TestRFC8212RawAPIAdmission drives both authorized raw API forms through the
// real writer and checks wire bytes and committed ownership, not API success.
func TestRFC8212RawAPIAdmission(t *testing.T) {
	set := append(rfc9774Segment(rfc9774SegSequence, 2, 65000), rfc9774Segment(rfc9774SegSet, 2, 65010)...)
	setBody := makeUpdateBody(nil, rfc9774Attrs(set), []byte{24, 203, 0, 113})
	for _, packet := range []bool{false, true} {
		mode := "body"
		if packet {
			mode = "packet"
		}
		for _, policyCase := range []struct {
			name     string
			internal bool
			final    filterapi.FinalDisposition
			accept   bool
		}{
			{name: "absent-ebgp"},
			{name: "explicit-reject", final: filterapi.FinalReject},
			{name: "explicit-accept", final: filterapi.FinalAccept, accept: true},
			{name: "absent-ibgp", internal: true, accept: true},
		} {
			for _, tc := range []struct {
				name      string
				body      []byte
				denied    []byte
				announces bool
				opaque    bool
			}{
				{name: "announce", body: syncOrderAnnounceBody, announces: true},
				{name: "as-set", body: setBody, announces: true},
				{name: "withdraw", body: mustHex(t, "000418c000020000"), denied: mustHex(t, "000418c000020000")},
				{name: "mixed", body: mustHex(t, "000418c00002000018c00003"), denied: mustHex(t, "000418c000020000"), announces: true},
				{name: "mp-announce", body: mustHex(t, "0000001f800e1c0002011020010db8000000000000000000000001003020010db80001"), announces: true},
				{name: "mp-withdraw", body: mustHex(t, "0000000d800f0a0002013020010db80001"), denied: mustHex(t, "0000000d800f0a0002013020010db80001")},
				{name: "mp-mixed", body: mustHex(t, "0000001d800f0a0002013020010db80001800e0d00010104010101010018c00003"), denied: mustHex(t, "0000000d800f0a0002013020010db80001"), announces: true},
				{name: "eor", body: mustHex(t, "00000000"), denied: mustHex(t, "00000000")},
				{name: "mp-eor", body: mustHex(t, "00000006800f03000201"), denied: mustHex(t, "00000006800f03000201")},
				{name: "opaque", body: mustHex(t, "deadbeef"), opaque: true},
				{name: "attribute-tail", body: mustHex(t, "0000000140"), opaque: true},
				{name: "withdrawal-tail", body: mustHex(t, "0001210000"), opaque: true},
				{name: "mp-withdrawal-tail", body: mustHex(t, "00000007800f0400020181"), opaque: true},
				{name: "nlri-tail", body: append(bytes.Clone(syncOrderAnnounceBody), 33), opaque: true},
			} {
				t.Run(mode+"/"+policyCase.name+"/"+tc.name, func(t *testing.T) {
					peer, conn := rawOwnershipPeer(t)
					if policyCase.internal {
						peer.settings.PeerAS = peer.settings.LocalAS
					}
					if policyCase.final != 0 {
						peer.settings.ExportPolicy = mustPolicy(t, nil, policyCase.final)
					}
					peer.refreshForwardFacts()
					// Match runOnce's coherent published session-final callback.
					peer.session.egressFinalAccepts = func() bool {
						facts := peer.forwardFacts()
						return facts.exportPolicy.AcceptsAtEnd(facts.isEBGP)
					}
					kind, payload := uint8(msgtype.TypeUPDATE), tc.body
					if packet {
						kind, payload = 0, rawOwnershipPacket(tc.body, msgtype.TypeUPDATE)
					}
					api := newSendPermissionReactor(peer)
					if err := api.SendRawMessage(peer.settings.Address, kind, payload, plugin.ProcessSender(railGranted)); err != nil {
						t.Fatal(err)
					}
					want := tc.denied
					if policyCase.accept {
						want = tc.body
					}
					var frame []byte
					if want != nil {
						frame = rawOwnershipPacket(want, msgtype.TypeUPDATE)
					}
					if !bytes.Equal(conn.written(), frame) {
						t.Errorf("wire=%x want=%x", conn.written(), frame)
					}
					wantOwner := policyCase.accept && tc.announces
					if got := len(peer.adjOut.routes) != 0; got != wantOwner {
						t.Errorf("advertised owner=%v want=%v", got, wantOwner)
					}
					wantRetired := policyCase.accept && tc.opaque
					if conn.closed != wantRetired || peer.session.tearingDown.Load() != wantRetired {
						t.Errorf("retired=%v teardown=%v want=%v", conn.closed, peer.session.tearingDown.Load(), wantRetired)
					}
				})
			}
		}
	}
}

// TestRFC8212RawControlUnaffected preserves provably non-UPDATE diagnostics but
// requires accepting export policy for opaque frames that could hide UPDATEs.
func TestRFC8212RawControlUnaffected(t *testing.T) {
	keepalive := rawOwnershipPacket(nil, msgtype.TypeKEEPALIVE)
	trailing := append(bytes.Clone(keepalive), rawOwnershipPacket(syncOrderAnnounceBody, msgtype.TypeUPDATE)...)
	for _, accept := range []bool{false, true} {
		for _, tc := range []struct {
			name    string
			kind    uint8
			payload []byte
			want    []byte
			opaque  bool
		}{
			{name: "keepalive-body", kind: uint8(msgtype.TypeKEEPALIVE), want: keepalive},
			{name: "keepalive-packet", payload: keepalive, want: keepalive},
			{name: "empty"},
			{name: "control-trailing-update", payload: trailing, want: trailing, opaque: true},
			{name: "unknown-body", kind: 255, payload: []byte{1}, want: rawOwnershipPacket([]byte{1}, msgtype.MessageType(255)), opaque: true},
			{name: "short-packet", payload: []byte{0xff}, want: []byte{0xff}, opaque: true},
		} {
			t.Run(tc.name, func(t *testing.T) {
				peer, conn := rawOwnershipPeer(t)
				if accept {
					peer.settings.ExportPolicy = mustPolicy(t, nil, filterapi.FinalAccept)
				}
				peer.refreshForwardFacts()
				peer.session.egressFinalAccepts = func() bool {
					facts := peer.forwardFacts()
					return facts.exportPolicy.AcceptsAtEnd(facts.isEBGP)
				}
				api := newSendPermissionReactor(peer)
				if err := api.SendRawMessage(peer.settings.Address, tc.kind, tc.payload, plugin.ProcessSender(railGranted)); err != nil {
					t.Fatal(err)
				}
				want := tc.want
				if tc.opaque && !accept {
					want = nil
				}
				if !bytes.Equal(conn.written(), want) {
					t.Errorf("accept=%v control wire=%x want=%x", accept, conn.written(), want)
				}
				retired := tc.opaque && accept
				if conn.closed != retired || len(peer.adjOut.routes) != 0 {
					t.Errorf("accept=%v control retired=%v want=%v owners=%d", accept, conn.closed, retired, len(peer.adjOut.routes))
				}
			})
		}
	}
}

// TestRFC8212ImportEpochFences holds genuinely admitted work across a policy
// epoch change, then observes the production direct writer and queued worker.
// A later wire marker settles the negative assertion; the old owner's explicit
// withdrawal and a fresh announcement remain independent positive controls.
func TestRFC8212ImportEpochFences(t *testing.T) {
	for _, rail := range []string{"direct", "cached", "pool-fallback"} {
		t.Run(rail, func(t *testing.T) {
			gate := ownershipRailNewGate()
			f := ownershipRailNew(t, gate)
			f.r.clock = f.source.clock
			f.r.messageReceiver = &testDeliveryReceiver{consumerCount: 1}
			first := rfc8212EpochAdmit(t, f, ownershipRailBody(family.IPv4Unicast, false, syncOrderPrefixWire, 1))
			rfc8212EpochDispatch(t, f, rail, first)
			f.await(t, 1, syncOrderPrefixWire)

			stale := rfc8212EpochAdmit(t, f, ownershipRailBody(family.IPv4Unicast, false, syncOrderPrefixWire, 2))
			if rail != "direct" {
				gate.armed.Store(true)
				rfc8212EpochDispatch(t, f, rail, stale)
				awaitEntered(t, gate.entered)
			}
			generation := f.source.forwardGeneration.Load()
			f.source.advanceSourcePolicyEpoch()
			if f.source.forwardGeneration.Load() != generation {
				t.Fatal("policy change replaced the dispatch session")
			}
			if rail == "direct" {
				rfc8212EpochDispatch(t, f, rail, stale)
			} else {
				gate.release()
			}
			rfc8212EpochMarker(t, f, rail, 90)
			f.assertHistory(t, []byte{1}, 0)

			withdraw := rfc8212EpochAdmit(t, f, syncOrderWithdrawBody)
			rfc8212EpochDispatch(t, f, rail, withdraw)
			rfc8212EpochMarker(t, f, rail, 91)
			f.assertHistory(t, []byte{1}, 1)

			fresh := rfc8212EpochAdmit(t, f, ownershipRailBody(family.IPv4Unicast, false, syncOrderPrefixWire, 3))
			rfc8212EpochDispatch(t, f, rail, fresh)
			f.await(t, 3, syncOrderPrefixWire)
		})
	}
}

// TestRFC8212StoredReplayEpochFences reconstructs retained bytes only after the
// policy changes. A reconnect alone still permits original policy provenance;
// a policy change or a replacement Peer at the same address does not.
func TestRFC8212StoredReplayEpochFences(t *testing.T) {
	for _, transition := range []string{"reconnect", "policy", "replacement", "unrecorded"} {
		t.Run(transition, func(t *testing.T) {
			f := ownershipRailNew(t, nil)
			f.r.clock = f.source.clock
			f.r.messageReceiver = &testDeliveryReceiver{consumerCount: 1}
			f.source.recvCtxID = f.ctxID
			update := rfc8212EpochAdmit(t, f, ownershipRailBody(family.IPv4Unicast, false, syncOrderPrefixWire, 7))
			sections, err := wire.ParseUpdateSections(update.WireUpdate.Payload())
			if err != nil {
				t.Fatal(err)
			}
			route := rpc.StoredRoute{
				SourcePeer: f.source.addrString, Family: family.IPv4Unicast.String(),
				MsgID: update.WireUpdate.MessageID(), SourcePolicyEpoch: update.SourcePolicyEpoch,
				NLRIFraming: rpc.NLRIFramingPrefixOnly,
				NLRIHex:     hex.EncodeToString(sections.NLRI(update.WireUpdate.Payload())),
				AttrHex:     hex.EncodeToString(sections.Attrs(update.WireUpdate.Payload())),
				NextHopHex:  "0a000001",
			}
			switch transition {
			case "reconnect":
				f.source.advanceForwardGeneration()
			case "policy":
				f.source.advanceSourcePolicyEpoch()
			case "replacement":
				prior := f.source
				replacement := NewPeer(prior.Settings())
				replacement.state.Store(int32(PeerStateEstablished))
				replacement.advanceForwardGeneration()
				replacement.recvCtxID = f.ctxID
				replacement.sendCtxID = f.ctxID
				replacement.sendCtx.Store(prior.sendCtx.Load())
				replacement.negotiated.Store(prior.negotiated.Load())
				replacement.refreshForwardFacts()
				f.r.peers[prior.Settings().PeerKey()] = replacement
				f.source = replacement
			case "unrecorded":
				route.SourcePolicyEpoch = 0
			}
			api := &reactorAPIAdapter{r: f.r}
			if err := api.RelayStoredRoute(f.destination.Settings().Address, []rpc.StoredRoute{route}, plugin.OperatorSender()); err != nil {
				t.Fatal(err)
			}
			rfc8212EpochMarker(t, f, "cached", 90)
			var origins []byte
			for _, event := range ownershipRailRead(t, f.conn.written(), f.destination.sendCtx.Load()) {
				if event.key == string(syncOrderPrefixWire) && !event.withdraw {
					origins = append(origins, event.origin)
				}
			}
			var want []byte
			if transition == "reconnect" {
				want = []byte{7}
			}
			if !bytes.Equal(origins, want) {
				t.Fatalf("%s replay reached recipient with origins %v, want %v", transition, origins, want)
			}
		})
	}
}

// rfc8212EpochAdmit uses actual ingress admission, not a manually stamped cache
// object. The fixture's existing consumer owns the cache reference until cleanup.
func rfc8212EpochAdmit(t *testing.T, f *ownershipRailFixture, body []byte) *ReceivedUpdate {
	t.Helper()
	update := wireu.NewWireUpdate(body, f.ctxID)
	update.SetSourceID(f.source.SourceID())
	if !f.r.notifyMessageReceiver(f.source.Settings().Address, msgtype.TypeUPDATE, body, update,
		f.ctxID, rpc.DirectionReceived, BufHandle{ID: noPoolBufID, Buf: body}, nil, "", 0) {
		t.Fatal("ingress did not admit the UPDATE into the cache")
	}
	cached, ok := f.r.recentUpdates.Get(update.MessageID())
	if !ok {
		t.Fatal("admitted UPDATE disappeared before dispatch")
	}
	return cached
}

// rfc8212EpochDispatch drives the production cache command or fast-path writer.
func rfc8212EpochDispatch(t *testing.T, f *ownershipRailFixture, rail string, update *ReceivedUpdate) {
	t.Helper()
	if rail == "cached" {
		api := &reactorAPIAdapter{r: f.r}
		if err := api.ForwardUpdatesDirect([]uint64{update.WireUpdate.MessageID()},
			[]netip.AddrPort{f.destination.Settings().PeerKey()}, "route-server", plugin.OperatorSender()); err != nil {
			t.Fatal(err)
		}
		return
	}
	if rail == "pool-fallback" {
		f.destination.session.writeMu.Lock()
		reactorForwardRS(f.r, update, update.WireUpdate.MessageID(), f.source.Settings().Address, f.source)
		f.destination.session.writeMu.Unlock()
		return
	}
	reactorForwardRS(f.r, update, update.WireUpdate.MessageID(), f.source.Settings().Address, f.source)
	f.flush(t)
}

// rfc8212EpochMarker settles all earlier work on the same destination rail.
func rfc8212EpochMarker(t *testing.T, f *ownershipRailFixture, rail string, value byte) {
	t.Helper()
	raw := []byte{24, 198, 51, value}
	update := rfc8212EpochAdmit(t, f, ownershipRailBody(family.IPv4Unicast, false, raw, value))
	rfc8212EpochDispatch(t, f, rail, update)
	f.await(t, value, raw)
}

// TestRFC8212ExpectedOwnerEpochFence proves the exact sent receipt cannot
// reannounce stale received ownership, while retaining authority to remove it.
// A direct local announcement remains independent of the source's policy epoch.
func TestRFC8212ExpectedOwnerEpochFence(t *testing.T) {
	f := ownershipRailNew(t, nil)
	f.r.clock = f.source.clock
	f.r.messageReceiver = &testDeliveryReceiver{consumerCount: 1}
	update := rfc8212EpochAdmit(t, f, ownershipRailBody(family.IPv4Unicast, false, syncOrderPrefixWire, 1))
	rfc8212EpochDispatch(t, f, "direct", update)
	f.await(t, 1, syncOrderPrefixWire)
	session := f.destination.currentSession()
	var revision uint64
	f.destination.adjOut.mu.Lock()
	for _, entry := range f.destination.adjOut.routes {
		revision = entry.revision
	}
	f.destination.adjOut.mu.Unlock()
	if revision == 0 {
		t.Fatal("writer did not issue an exact sent receipt")
	}
	f.source.advanceSourcePolicyEpoch()
	item := fwdItem{peer: f.destination, session: session, authority: adjOutExpected,
		expectedMessage: revision,
		rawBodies:       [][]byte{ownershipRailBody(family.IPv4Unicast, false, syncOrderPrefixWire, 2)}}
	handled, _, _ := tryDirectWriteNoFlush(&item)
	if !handled {
		t.Fatal("exact-owner writer did not handle the replay")
	}
	f.flush(t)
	f.assertHistory(t, []byte{1}, 0)
	item.rawBodies = [][]byte{syncOrderWithdrawBody}
	handled, _, _ = tryDirectWriteNoFlush(&item)
	if !handled {
		t.Fatal("exact-owner writer did not handle the removal")
	}
	f.flush(t)
	f.assertHistory(t, []byte{1}, 1)
	// RFC 4271 Section 4.3: local origination is a separate authority.
	if err := session.SendUpdate(ownershipWriterUpdate(t,
		ownershipRailBody(family.IPv4Unicast, false, syncOrderPrefixWire, 3))); err != nil {
		t.Fatal(err)
	}
	f.await(t, 3, syncOrderPrefixWire)
}
