package filter_irr

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/netip"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/ze-software/ze/internal/component/plugin/issuequeue"
	"github.com/ze-software/ze/internal/component/resolve/irr"
	"github.com/ze-software/ze/internal/component/resolve/irr/store"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// VALIDATES: AC-5 -- concurrent filter evaluation during refresh sees consistent state.
// PREVENTS: Torn reads when refresh goroutine swaps the prefix-list while filter
// evaluation is in progress.
func TestRefreshSwapsAtomically(t *testing.T) {
	initial := mustPrefixListFromIRR(t, irr.PrefixList{
		IPv4: []netip.Prefix{netip.MustParsePrefix("10.0.0.0/24")},
	})
	plug := &irrPlugin{
		byASN: map[uint32]*asnState{
			65001: {asn: 65001, configured: "AS-TEST", asSet: "AS-TEST", list: &irrPrefixList{entries: initial}},
		},
	}

	replacement := mustPrefixListFromIRR(t, irr.PrefixList{
		IPv4: []netip.Prefix{
			netip.MustParsePrefix("10.0.0.0/24"),
			netip.MustParsePrefix("172.16.0.0/16"),
		},
	})

	var wg sync.WaitGroup
	stop := make(chan struct{})

	wg.Go(func() {
		for {
			select {
			case <-stop:
				return
			default:
				plug.mu.RLock()
				st := plug.byASN[65001]
				list := st.list
				plug.mu.RUnlock()
				if list != nil {
					for _, e := range list.entries {
						_ = e.prefix.String()
					}
				}
			}
		}
	})

	for i := range 100 {
		plug.mu.Lock()
		st := plug.byASN[65001]
		if i%2 == 0 {
			st.list = &irrPrefixList{entries: replacement}
		} else {
			st.list = &irrPrefixList{entries: initial}
		}
		st.checkedAt = time.Now()
		plug.mu.Unlock()
	}

	close(stop)
	wg.Wait()
}

// VALIDATES: AC-10 -- multiple peers with the same remote ASN share a single
// IRR query and prefix-list via the byASN map.
func TestMultiplePeersSameASN(t *testing.T) {
	bgpCfg := map[string]any{
		"peer": map[string]any{
			"10.0.0.1": map[string]any{
				"session": map[string]any{
					"asn": map[string]any{"remote": "65001"},
					"irr": map[string]any{"as-set": "AS-SHARED"},
				},
			},
			"10.0.0.2": map[string]any{
				"session": map[string]any{
					"asn": map[string]any{"remote": "65001"},
				},
			},
		},
	}
	cfg := parseIRRConfigOf(bgpCfg)

	asnMap := make(map[uint32]*asnState)
	for _, peer := range cfg.Peers {
		if peer.Disabled {
			continue
		}
		st, exists := asnMap[peer.RemoteASN]
		if !exists {
			st = &asnState{asn: peer.RemoteASN, asSet: peer.ASSet}
			asnMap[peer.RemoteASN] = st
		}
		st.peerAddrs = append(st.peerAddrs, peer.PeerAddr)
		if peer.ASSet != "" && st.asSet == "" {
			st.asSet = peer.ASSet
		}
	}

	if len(asnMap) != 1 {
		t.Fatalf("asnMap has %d entries, want 1 (shared ASN)", len(asnMap))
	}
	st := asnMap[65001]
	if len(st.peerAddrs) != 2 {
		t.Errorf("peerAddrs = %d, want 2", len(st.peerAddrs))
	}
	if st.asSet != "AS-SHARED" {
		t.Errorf("asSet = %q, want AS-SHARED", st.asSet)
	}
}

// VALIDATES: AC-14 -- peer with irr enable disable is excluded from filter evaluation.
func TestHandleConfigureDisabledPeerSkipped(t *testing.T) {
	bgpCfg := map[string]any{
		"peer": map[string]any{
			"10.0.0.1": map[string]any{
				"session": map[string]any{
					"asn": map[string]any{"remote": "65001"},
					"irr": map[string]any{"enable": "disable"},
				},
			},
			"10.0.0.2": map[string]any{
				"session": map[string]any{
					"asn": map[string]any{"remote": "65002"},
					"irr": map[string]any{"as-set": "AS-ACTIVE"},
				},
			},
		},
	}
	cfg := parseIRRConfigOf(bgpCfg)

	byASN := make(map[uint32]*asnState)
	for _, peer := range cfg.Peers {
		if peer.Disabled {
			continue
		}
		st, exists := byASN[peer.RemoteASN]
		if !exists {
			st = &asnState{asn: peer.RemoteASN, asSet: peer.ASSet}
			byASN[peer.RemoteASN] = st
		}
		st.peerAddrs = append(st.peerAddrs, peer.PeerAddr)
	}

	if _, found := byASN[65001]; found {
		t.Error("disabled peer ASN 65001 should not be in byASN")
	}
	if _, found := byASN[65002]; !found {
		t.Error("enabled peer ASN 65002 should be in byASN")
	}
}

// VALIDATES: handleConfigure -- the real configure entry point -- enrolls NO ASN
// for a peer that never opted into IRR filtering, so its AS number is no
// interest the irr plugin fetches for.
// PREVENTS: the unsolicited live lookup that any `bgp { peer ... }` config used
// to trigger. Enrollment keyed on "peer has a remote ASN" alone, so every BGP
// config -- and therefore every functional test with a BGP peer -- opened an
// HTTPS connection to www.peeringdb.com and a whois connection to the IRR server
// (default whois.radb.net) at startup. Driven through handleConfigure, not the
// parser, because the enrollment decision is what gates the network I/O.
func TestHandleConfigureDoesNotEnrollPeerWithoutIRR(t *testing.T) {
	plug := &irrPlugin{
		byASN: make(map[uint32]*asnState),
		// Past OnStarted, so handleConfigure asks the reader itself.
		started: true,
	}

	// A peer with a remote ASN and nothing else: no filter chain, no as-set.
	plug.configureOf(map[string]any{
		"peer": map[string]any{
			"10.0.0.1": map[string]any{
				"session": map[string]any{"asn": map[string]any{"remote": "65001"}},
			},
		},
	})

	plug.mu.RLock()
	enrolled := len(plug.byASN)
	plug.mu.RUnlock()
	if enrolled != 0 {
		t.Errorf("byASN has %d enrolled ASN(s), want 0: a peer with no IRR filter "+
			"reference and no as-set must not be resolved", enrolled)
	}
}

// VALIDATES: handleConfigure DOES enroll a peer whose filter chain names
// bgp-filter-irr, and a reload after the start reads its stored list -- the
// gate above rejects the unsolicited case without disabling the feature it
// gates.
// PREVENTS: the enrollment gate silently disabling IRR filtering for a correctly
// configured peer (fail-closed guards must still let the intended path through).
func TestHandleConfigureEnrollsIRRFilteredPeer(t *testing.T) {
	state := newFakeConsumerState()
	state.storeList(t, "AS-CUSTOMER1", "10.0.0.0/24")
	plug := storedPlugin(state, map[uint32]string{})
	plug.started = true // past OnStarted, so handleConfigure asks the reader itself

	plug.configureOf(map[string]any{
		"peer": map[string]any{
			"10.0.0.1": map[string]any{
				"session": map[string]any{
					"asn": map[string]any{"remote": "65001"},
					"irr": map[string]any{"as-set": "AS-CUSTOMER1"},
				},
				"filter": map[string]any{"import": map[string]any{"filters": []any{"bgp-filter-irr:65001"}, "final": "accept"}},
			},
		},
	})
	select {
	case <-plug.reader.wake:
	default:
		t.Fatal("a reload after the start did not ask the reader for the stored lists")
	}
	plug.reader.pass(context.Background())

	plug.mu.RLock()
	defer plug.mu.RUnlock()
	st := plug.byASN[65001]
	if st == nil {
		t.Fatal("ASN 65001 not enrolled: an IRR-filtered peer must be filtered")
	}
	if st.readErr != "" {
		t.Fatalf("unexpected read error: %s", st.readErr)
	}
	if st.list == nil || len(st.list.entries) != 1 {
		t.Fatalf("prefix-list not read from the store: %+v", st.list)
	}
}

// VALIDATES: AC-2 -- prefix in IRR list accepted.
// VALIDATES: AC-3 -- prefix NOT in IRR list rejected.
// VALIDATES: AC-13 -- aggregated prefixes preserved from IRR.
// PREVENTS: IRR-to-prefix-list conversion drops or corrupts entries.

func TestPrefixListFromIRR(t *testing.T) {
	pl := irr.PrefixList{
		IPv4: []netip.Prefix{
			netip.MustParsePrefix("10.0.0.0/24"),
			netip.MustParsePrefix("172.16.0.0/16"),
		},
		IPv6: []netip.Prefix{
			netip.MustParsePrefix("2001:db8::/32"),
		},
	}

	entries := mustPrefixListFromIRR(t, pl)

	if len(entries) != 3 {
		t.Fatalf("entries = %d, want 3", len(entries))
	}

	if entries[0].prefix != netip.MustParsePrefix("10.0.0.0/24") {
		t.Errorf("entry[0].prefix = %s, want 10.0.0.0/24", entries[0].prefix)
	}
	if entries[0].ge != 24 || entries[0].le != 32 {
		t.Errorf("entry[0] ge=%d le=%d, want ge=24 le=32", entries[0].ge, entries[0].le)
	}

	if entries[2].prefix != netip.MustParsePrefix("2001:db8::/32") {
		t.Errorf("entry[2].prefix = %s, want 2001:db8::/32", entries[2].prefix)
	}
	if entries[2].ge != 32 || entries[2].le != 128 {
		t.Errorf("entry[2] ge=%d le=%d, want ge=32 le=128", entries[2].ge, entries[2].le)
	}
}

func TestPrefixListFromIRRAcceptReject(t *testing.T) {
	pl := irr.PrefixList{
		IPv4: []netip.Prefix{
			netip.MustParsePrefix("10.0.0.0/24"),
			netip.MustParsePrefix("10.0.1.0/24"),
		},
	}
	entries := mustPrefixListFromIRR(t, pl)
	list := &irrPrefixList{entries: entries}

	if !admitsAll(t, list, "10.0.0.0/24") {
		t.Error("matching prefix should be accepted")
	}
	if admitsAll(t, list, "192.168.0.0/24") {
		t.Error("non-matching prefix should be rejected")
	}
}

// VALIDATES: AC-4 -- empty list rejects all (fail-closed).
func TestPrefixListFromIRREmpty(t *testing.T) {
	entries := mustPrefixListFromIRR(t, irr.PrefixList{})
	list := &irrPrefixList{entries: entries}

	if admitsAll(t, list, "10.0.0.0/24") {
		t.Error("empty IRR result should reject all")
	}
}

// VALIDATES: AC-2, AC-3 -- filter name -> ASN extraction.
func TestExtractASNFromFilter(t *testing.T) {
	tests := []struct {
		filter string
		want   uint32
	}{
		{"bgp-filter-irr:65001", 65001},
		{"65001", 65001},
		{"bgp-filter-irr:", 0},
		{"", 0},
		{"bgp-filter-irr:abc", 0},
	}
	for _, tt := range tests {
		got := extractASNFromFilter(tt.filter)
		if got != tt.want {
			t.Errorf("extractASNFromFilter(%q) = %d, want %d", tt.filter, got, tt.want)
		}
	}
}

// VALIDATES: a mixed UPDATE (one in-list prefix, one out-of-list) is judged
// so only the in-list prefix is kept: one accepted, one rejected, and the
// answer's Sets remove just the out-of-list prefix, in the shape the prefix
// field carried it. PREVENTS: a single unauthorized prefix collaterally
// dropping the legitimate routes that share the same UPDATE (all-or-nothing
// reject).
func TestJudgeNLRIMixed(t *testing.T) {
	list := &irrPrefixList{entries: mustPrefixListFromIRR(t, irr.PrefixList{
		IPv4: []netip.Prefix{netip.MustParsePrefix("10.0.0.0/24")},
	})}
	judged, err := judgeNLRI(announceFields("10.0.0.0/24", "192.168.0.0/24"), list)
	if err != nil {
		t.Fatalf("judgeNLRI: %v", err)
	}
	if judged.accepted != 1 || judged.rejected != 1 {
		t.Errorf("accepted=%d rejected=%d, want 1 and 1", judged.accepted, judged.rejected)
	}
	out := judgeListed(&sdk.FilterUpdateInput{Filter: "bgp-filter-irr:65001"}, &judged)
	if out.Action != sdk.FilterModify {
		t.Fatalf("action = %v, want FilterModify", out.Action)
	}
	if got := removedValues(t, out); !slices.Equal(got, []string{`"192.168.0.0/24"`}) {
		t.Errorf("removes = %v, want [\"192.168.0.0/24\"]", got)
	}
}

// VALIDATES: an UPDATE whose prefixes are all out-of-list yields no accepted
// prefix and answers a whole-update reject, and an all-in-list UPDATE yields no
// rejected prefix and answers accept unmodified.
func TestJudgeNLRIAllOrNone(t *testing.T) {
	list := &irrPrefixList{entries: mustPrefixListFromIRR(t, irr.PrefixList{
		IPv4: []netip.Prefix{netip.MustParsePrefix("10.0.0.0/24")},
	})}
	in := &sdk.FilterUpdateInput{Filter: "bgp-filter-irr:65001"}
	allBad, err := judgeNLRI(announceFields("192.168.0.0/24", "203.0.113.0/24"), list)
	if err != nil {
		t.Fatalf("judgeNLRI: %v", err)
	}
	if allBad.accepted != 0 || allBad.rejected != 2 {
		t.Errorf("all-out-of-list: accepted=%d rejected=%d, want 0 accepted / 2 rejected", allBad.accepted, allBad.rejected)
	}
	if out := judgeListed(in, &allBad); out.Action != sdk.FilterReject || len(out.Sets) != 0 {
		t.Errorf("all-out-of-list: answer %+v, want FilterReject with no Sets", out)
	}
	allGood, err := judgeNLRI(announceFields("10.0.0.0/24"), list)
	if err != nil {
		t.Fatalf("judgeNLRI: %v", err)
	}
	if allGood.accepted != 1 || allGood.rejected != 0 {
		t.Errorf("all-in-list: accepted=%d rejected=%d, want 1 accepted / 0 rejected", allGood.accepted, allGood.rejected)
	}
	if out := judgeListed(in, &allGood); out.Action != sdk.FilterAccept || len(out.Sets) != 0 {
		t.Errorf("all-in-list: answer %+v, want FilterAccept with no Sets", out)
	}
}

// VALIDATES: an IRR-filtered UPDATE that arrives BEFORE the first background
// resolution has populated the prefix-list is judged at once against the
// unresolved list (FR-10, AC-32): the in-list prefix takes the filter's
// on-error reject, and once the resolution publishes the list the same
// UPDATE, sent again, is accepted on its merits. The wait this test proved
// before D-12 is deleted (R-8); TestIRRUnresolvedTakesOnError proves the
// hold and the ROUTE-REFRESH that brings the route back.
// PREVENTS: a configured-IRR peer's first UPDATE blocking an engine goroutine
// on the background resolution, and the list never reaching the evaluator.
func TestFilterJudgesBeforeFirstResolution(t *testing.T) {
	state := newFakeConsumerState()
	state.storeList(t, "AS-TEST", "10.0.0.0/24")
	// The state configure leaves: enrolled, nothing read yet.
	plug := storedPlugin(state, map[uint32]string{65001: "AS-TEST"})

	in := &sdk.FilterUpdateInput{
		Filter: "bgp-filter-irr:65001",
		Peer:   "127.0.0.1",
		Fields: announceFields("10.0.0.0/24"),
	}
	if out := plug.handleFilterUpdate(in); out.Action != sdk.FilterReject {
		t.Fatalf("before the first resolution: action = %v, want FilterReject (on-error, no wait)", out.Action)
	}

	readStored(plug)
	if out := plug.handleFilterUpdate(in); out.Action != sdk.FilterAccept {
		st := plug.byASN[65001]
		t.Fatalf("after the first read: action = %v (list=%v readErr=%q), want FilterAccept",
			out.Action, st.list, st.readErr)
	}
}

// VALIDATES: an IRR-filtered UPDATE whose ASN resolution never completes
// (resolution still in flight, or never started) is answered at once with the
// fail-closed reject: the evaluator reads the published snapshot and has
// nothing to wait on.
// PREVENTS: handleFilterUpdate blocking the engine goroutine that calls it.
func TestFilterAnswersWithoutWaiting(t *testing.T) {
	plug := &irrPlugin{
		// Enrolled and never read; no reader runs.
		byASN: map[uint32]*asnState{65003: newASNState(65003, "AS-SLOW")},
	}
	plug.mu.Lock()
	plug.publishLocked()
	plug.mu.Unlock()

	done := make(chan *sdk.FilterUpdateOutput, 1)
	go func() {
		done <- plug.handleFilterUpdate(&sdk.FilterUpdateInput{
			Filter: "bgp-filter-irr:65003",
			Peer:   "127.0.0.1",
			Fields: announceFields("10.0.0.0/24"),
		})
	}()

	select {
	case out := <-done:
		if out.Action != sdk.FilterReject {
			t.Errorf("action = %v, want FilterReject (fail-closed, no list resolved)", out.Action)
		}
	case <-time.After(sdk.FilterCallDeadline * 20):
		t.Fatal("handleFilterUpdate did not answer at once: the evaluator waited")
	}
}

// VALIDATES: ASN boundary handling in filter name -> ASN extraction.
// Range 1..4294967295 (uint32); below (0) and above (>0xFFFFFFFF) yield 0.
func TestExtractASNFromFilterBoundaries(t *testing.T) {
	tests := []struct {
		filter string
		want   uint32
	}{
		{"bgp-filter-irr:0", 0}, // below valid range
		{"bgp-filter-irr:1", 1}, // first valid
		{"bgp-filter-irr:4294967294", 4294967294},
		{"bgp-filter-irr:4294967295", 4294967295}, // max uint32
		{"bgp-filter-irr:4294967296", 0},          // overflow uint32
	}
	for _, tt := range tests {
		if got := extractASNFromFilter(tt.filter); got != tt.want {
			t.Errorf("extractASNFromFilter(%q) = %d, want %d", tt.filter, got, tt.want)
		}
	}
}

// irrConfigFor builds the bgp subtree handleConfigure reads: one peer of AS
// 65001 importing bgp-filter-irr:65001 with the given as-set ("" lets
// the irr plugin's PeeringDB discovery choose it). The PeeringDB URL a
// discovery queries is the irr plugin's config, so the bgp root carries none.
func irrConfigFor(asSet string) map[string]any {
	const asn = "65001"
	return map[string]any{
		"peer": map[string]any{
			"10.0.0.1": map[string]any{
				"session": map[string]any{
					"asn": map[string]any{"remote": asn},
					"irr": map[string]any{"as-set": asSet},
				},
				"filter": map[string]any{"import": map[string]any{"filters": []any{"bgp-filter-irr:" + asn}, "final": "accept"}},
			},
		},
	}
}

// VALIDATES: a configure that runs before OnStarted, inside the startup
// handshake, where no state RPC is served, asks the reader for nothing.
// PREVENTS: a state read refused inside the handshake: the first configure
// leaves the store read to OnStarted.
func TestConfigureBeforeStartedQueriesNothing(t *testing.T) {
	state := newFakeConsumerState()
	plug := storedPlugin(state, map[uint32]string{})

	plug.configureOf(irrConfigFor("AS-ONE"))

	select {
	case <-plug.reader.wake:
		t.Error("a configure before OnStarted asked the reader for a store read")
	default:
	}
	if state.readCount() != 0 {
		t.Errorf("store reads = %d before OnStarted, want 0", state.readCount())
	}
	if plug.byASN[65001] == nil {
		t.Error("ASN 65001 was not enrolled: the test would pass with nothing to read")
	}
}

// VALIDATES: a reload that keeps a peer's as-set keeps the list it enforces
// until the reader reads again: an unrelated edit never drops a list.
func TestReloadKeepingTheSetKeepsItsList(t *testing.T) {
	state := newFakeConsumerState()
	state.storeList(t, "AS-ONE", "10.0.0.0/24")
	plug := storedPlugin(state, map[uint32]string{})
	plug.configureOf(irrConfigFor("AS-ONE"))
	readStored(plug)
	plug.configureOf(irrConfigFor("as-one"))
	if got := announceFor(plug, "65001", "10.0.0.0/24"); got != sdk.FilterAccept {
		t.Fatalf("a reload keeping the set dropped its list: action = %v", got)
	}
}

// oversizedIRRList builds an IRR answer whose IPv4 half alone is one prefix
// over the per-family bound and whose IPv6 half carries one prefix.
func oversizedIRRList() irr.PrefixList {
	pl := irr.PrefixList{IPv6: []netip.Prefix{netip.MustParsePrefix("2001:db8::/32")}}
	pl.IPv4 = make([]netip.Prefix, 0, store.DatasetFamilyMax+1)
	for i := range store.DatasetFamilyMax + 1 {
		pl.IPv4 = append(pl.IPv4, netip.PrefixFrom(netip.AddrFrom4([4]byte{10, byte(i >> 16), byte(i >> 8), byte(i)}), 32))
	}
	return pl
}

// TestPrefixListFromIRRRefusesOversizedList checks that a list over the cap is
// refused rather than cut. The cut filled the budget with IPv4 first, so the
// IPv6 half was lost and every IPv6 route rejected while the refresh counted
// success (plan/journal/guard-addition-drops-what-it-refuses.md, 2026-10-08).
func TestPrefixListFromIRRRefusesOversizedList(t *testing.T) {
	entries, err := prefixListFromIRR(oversizedIRRList())
	if err == nil {
		t.Fatalf("oversized list answered %d entries, want a refusal", len(entries))
	}
	if !strings.Contains(err.Error(), "1 IPv6") {
		t.Errorf("refusal %q does not count the IPv6 half", err)
	}
}

// irrListOf builds an IRR answer of v4 distinct IPv4 /32s and v6 distinct
// IPv6 /128s.
func irrListOf(v4, v6 int) irr.PrefixList {
	pl := irr.PrefixList{
		IPv4: make([]netip.Prefix, 0, v4),
		IPv6: make([]netip.Prefix, 0, v6),
	}
	for i := range v4 {
		pl.IPv4 = append(pl.IPv4, netip.PrefixFrom(netip.AddrFrom4([4]byte{10, byte(i >> 16), byte(i >> 8), byte(i)}), 32))
	}
	for i := range v6 {
		a := [16]byte{0x20, 0x01, 0x0d, 0xb8}
		a[13], a[14], a[15] = byte(i>>16), byte(i>>8), byte(i)
		pl.IPv6 = append(pl.IPv6, netip.PrefixFrom(netip.AddrFrom16(a), 128))
	}
	return pl
}

// TestPrefixListFromIRRRefusesOversizedIPv6 proves the IPv6 family is bounded
// on its own: one prefix over store.DatasetFamilyMax is refused whole, with
// both counts named, while IPv4 holds one prefix.
func TestPrefixListFromIRRRefusesOversizedIPv6(t *testing.T) {
	entries, err := prefixListFromIRR(irrListOf(1, store.DatasetFamilyMax+1))
	if err == nil {
		t.Fatalf("oversized IPv6 list answered %d entries, want a refusal", len(entries))
	}
	if !strings.Contains(err.Error(), "1 IPv4") {
		t.Errorf("refusal %q does not count the IPv4 half", err)
	}
}

// TestPrefixListFromIRREnforcesWhatTheStoreHolds proves the bound is per
// family, the store's: 400,000 IPv4 and 200,000 IPv6 prefixes, over the
// 500,000 the two families once shared, is a list the store holds and the
// firewall enforces, so BGP must enforce it too (AC-8, AC-12).
// PREVENTS: a combined cap that refused a stored list and named a fix no
// store action could satisfy.
func TestPrefixListFromIRREnforcesWhatTheStoreHolds(t *testing.T) {
	const v4, v6 = 400_000, 200_000
	entries, err := prefixListFromIRR(irrListOf(v4, v6))
	if err != nil {
		t.Fatalf("a list the store holds was refused: %v", err)
	}
	if len(entries) != v4+v6 {
		t.Fatalf("entries = %d, want %d", len(entries), v4+v6)
	}
	if got := entries[v4].prefix; !got.Addr().Is6() {
		t.Errorf("entry %d = %v, want the first IPv6 prefix", v4, got)
	}
}

// mustPrefixListFromIRR is prefixListFromIRR for a list under the cap.
func mustPrefixListFromIRR(t *testing.T, pl irr.PrefixList) []prefixEntry {
	t.Helper()
	entries, err := prefixListFromIRR(pl)
	if err != nil {
		t.Fatalf("prefixListFromIRR: %v", err)
	}
	return entries
}

type irrTestPublisher struct {
	mu     sync.Mutex
	raises []sdk.Issue
}

func (p *irrTestPublisher) RaiseIssue(_ context.Context, issue sdk.Issue) error {
	p.mu.Lock()
	p.raises = append(p.raises, issue)
	p.mu.Unlock()
	return nil
}

func (p *irrTestPublisher) ClearIssue(_ context.Context, _, _ string) error {
	return nil
}

type irrSyncWriter struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (w *irrSyncWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buf.Write(p)
}

func (w *irrSyncWriter) String() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buf.String()
}

// TestFilterIRRInvalidPeerAndReadmitConfirmRaiseDoctorCodes verifies that an
// invalid peer address in trackVerdict and a post-refresh session confirmation
// failure in readmitter.readmit both emit WARN logs with their registered
// doctor codes and enqueue report-issue RPCs via issuequeue.
//
// VALIDATES: AC-2, AC-4 (filter_irr.go trackVerdict, readmit.go readmit).
// PREVENTS: invalid peer names and unconfirmed readmission refreshes hiding below WARN.
func TestFilterIRRInvalidPeerAndReadmitConfirmRaiseDoctorCodes(t *testing.T) {
	logs := &irrSyncWriter{}
	testLog := slog.New(slog.NewTextHandler(logs, &slog.HandlerOptions{Level: slog.LevelWarn}))
	prevLogger := logger
	logger = func() *slog.Logger { return testLog }
	t.Cleanup(func() { logger = prevLogger })

	pub := &irrTestPublisher{}
	q := issuequeue.Start(pub)
	t.Cleanup(q.Stop)

	eng := newFakeReadmitEngine()
	eng.afterRefreshErr = errors.New("session query timed out")

	allowed := mustPrefixListFromIRR(t, irr.PrefixList{
		IPv4: []netip.Prefix{netip.MustParsePrefix("203.0.113.0/24")},
	})
	plug := &irrPlugin{
		issues: q,
		byASN: map[uint32]*asnState{
			65001: {asn: 65001, list: &irrPrefixList{entries: allowed}},
		},
	}
	plug.readmit = newReadmitter(eng, plug, time.Hour)
	plug.readmit.issues = q

	// 1. Invalid peer address in trackVerdict (called twice -> rate-limited to 1 WARN/raise).
	judged := &routeNLRI{rejected: 1}
	plug.trackVerdict("not-an-ip", 65001, true, judged)
	plug.trackVerdict("not-an-ip", 65001, true, judged)

	// 2. Post-refresh session confirmation failure in readmitter.readmit.
	peerAddr := netip.MustParseAddr("192.0.2.50")
	pfx := netip.MustParsePrefix("203.0.113.0/24")
	plug.readmit.track(peerAddr, 65001, true, &familyNLRI{
		family:   "ipv4/unicast",
		prefixes: []netip.Prefix{pfx},
		admitted: []bool{false},
	})
	plug.readmit.readmit(peerFamily{peer: peerAddr, family: "ipv4/unicast"}, []heldKey{{
		peer: peerAddr, family: "ipv4/unicast", prefix: pfx,
	}})

	q.Flush()

	out := logs.String()
	for _, want := range []string{
		"level=WARN",
		"code=" + codeIRRInvalidPeer,
		"code=" + codeIRRReadmitConfirmFailed,
		fixShowBGPPeerList,
		"ze show bgp peer 192.0.2.50 detail",
		"report=queued",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("WARN log missing %q:\n%s", want, out)
		}
	}

	pub.mu.Lock()
	raises := append([]sdk.Issue(nil), pub.raises...)
	pub.mu.Unlock()
	if len(raises) != 2 {
		t.Fatalf("issuequeue raises = %d, want 2 (1 invalid-peer, 1 readmit-confirm): %+v", len(raises), raises)
	}
	var sawInvalidPeer, sawReadmitConfirm bool
	for _, issue := range raises {
		if issue.Severity != rpc.ReportError {
			t.Errorf("issue %q severity = %v, want error", issue.Code, issue.Severity)
		}
		switch issue.Code {
		case codeIRRInvalidPeer:
			sawInvalidPeer = true
			if issue.Fix != fixShowBGPPeerList {
				t.Errorf("invalid-peer fix = %q, want %q", issue.Fix, fixShowBGPPeerList)
			}
		case codeIRRReadmitConfirmFailed:
			sawReadmitConfirm = true
			if issue.Fix != "ze show bgp peer 192.0.2.50 detail" {
				t.Errorf("readmit-confirm fix = %q, want %q", issue.Fix, "ze show bgp peer 192.0.2.50 detail")
			}
		}
	}
	if !sawInvalidPeer {
		t.Errorf("missing issuequeue raise for %s", codeIRRInvalidPeer)
	}
	if !sawReadmitConfirm {
		t.Errorf("missing issuequeue raise for %s", codeIRRReadmitConfirmFailed)
	}
}

// TestFilterIRRInvalidPeerBoundsLongAndControlInput verifies that an oversized
// peer identifier carrying control bytes stays sanitized and within
// rpc.ReportSubjectMax and rpc.ReportMessageMax in both WARN logs and
// report-issue payloads (never echoing raw netip.ParseAddr input).
//
// VALIDATES: Security Review Checklist (bounded/sanitized report payloads) and AC-4.
// PREVENTS: raw netip.ParseAddr errors bypassing sanitizePeerLabel and exceeding rpc.ReportMessageMax.
func TestFilterIRRInvalidPeerBoundsLongAndControlInput(t *testing.T) {
	logs := &irrSyncWriter{}
	testLog := slog.New(slog.NewTextHandler(logs, &slog.HandlerOptions{Level: slog.LevelWarn}))
	prevLogger := logger
	logger = func() *slog.Logger { return testLog }
	t.Cleanup(func() { logger = prevLogger })

	pub := &irrTestPublisher{}
	q := issuequeue.Start(pub)
	t.Cleanup(q.Stop)

	plug := &irrPlugin{
		issues: q,
		byASN:  make(map[uint32]*asnState),
	}
	plug.readmit = newReadmitter(newFakeReadmitEngine(), plug, time.Hour)
	plug.readmit.issues = q

	sentinel := "UNBOUNDED_TAIL_MARKER"
	oversized := "bad\x00\x1b\npeer-" + strings.Repeat("A", 2048) + sentinel
	plug.trackVerdict(oversized, 65001, true, &routeNLRI{rejected: 1})
	q.Flush()

	pub.mu.Lock()
	raises := append([]sdk.Issue(nil), pub.raises...)
	pub.mu.Unlock()
	if len(raises) != 1 {
		t.Fatalf("issuequeue raises = %d, want 1", len(raises))
	}
	issue := raises[0]
	if err := rpc.ValidateReportIssue(&rpc.ReportIssueInput{
		Action:   rpc.ReportRaise,
		Severity: issue.Severity,
		Code:     issue.Code,
		Subject:  issue.Subject,
		Message:  issue.Message,
		Fix:      issue.Fix,
	}); err != nil {
		t.Fatalf("ValidateReportIssue rejected oversized invalid-peer report: %v", err)
	}
	if strings.Contains(issue.Message, sentinel) || strings.Contains(logs.String(), sentinel) {
		t.Fatalf("unbounded peer input leaked past sanitizePeerLabel:\nmsg=%s\nlog=%s", issue.Message, logs.String())
	}
	if strings.ContainsAny(issue.Message, "\x00\x1b\n") {
		t.Fatalf("raw control byte leaked into report message: %q", issue.Message)
	}
	if !utf8.ValidString(issue.Subject) || !utf8.ValidString(issue.Message) {
		t.Fatalf("report contains invalid UTF-8: subject=%q msg=%q", issue.Subject, issue.Message)
	}

	// Case 2: IPv6-like malformed peer hitting netip.ParseAddr's "(at ...)" suffix
	// containing `"): ` + sentinel must neither leak sentinel nor drop the static reason.
	atSentinel := "AT_SUFFIX_LEAK_SENTINEL"
	withAt := "1:2:3:4:5:6:7:8" + strings.Repeat("A", 2048) + `"): ` + atSentinel
	plug.trackVerdict(withAt, 65001, true, &routeNLRI{rejected: 1})

	// Case 3: C1 control (\u009b) inside the first 64 bytes AND multi-byte UTF-8 rune
	// straddling byte 64 must produce strictly valid printable ASCII with no C1 control
	// or mid-rune UTF-8 split.
	utf8AndC1 := "a\u009b" + strings.Repeat("x", 60) + "é"
	plug.trackVerdict(utf8AndC1, 65001, true, &routeNLRI{rejected: 1})
	q.Flush()

	pub.mu.Lock()
	raises = append([]sdk.Issue(nil), pub.raises...)
	pub.mu.Unlock()
	if len(raises) != 3 {
		t.Fatalf("issuequeue raises = %d, want 3", len(raises))
	}
	if strings.Contains(raises[1].Message, atSentinel) || strings.Contains(raises[1].Subject, atSentinel) || strings.Contains(logs.String(), atSentinel) {
		t.Fatalf("err.at suffix leaked sentinel:\nsubject=%q\nmsg=%q\nlog=%s", raises[1].Subject, raises[1].Message, logs.String())
	}
	if strings.Contains(raises[1].Message, "(at ") {
		t.Fatalf("raw netip.ParseAddr (at ...) suffix leaked into report message: %q", raises[1].Message)
	}
	if !strings.Contains(raises[1].Message, "(each group must have 4 or less digits)") || strings.Contains(raises[1].Message, "(unable to parse IP)") {
		t.Fatalf("parseAddrReason dropped static IPv6 reason and fell back: %q", raises[1].Message)
	}
	for idx, got := range raises {
		if !utf8.ValidString(got.Subject) || !utf8.ValidString(got.Message) {
			t.Fatalf("raise[%d] contains invalid UTF-8: subject=%q msg=%q", idx, got.Subject, got.Message)
		}
		for i := range len(got.Subject) {
			if b := got.Subject[i]; b < 0x20 || b >= 0x7f {
				t.Fatalf("raise[%d] subject contains non-printable-ASCII byte 0x%02x: %q", idx, b, got.Subject)
			}
		}
	}

	// Case 4: fitReportText handles negative and small maxBytes on multi-byte UTF-8 without panicking or splitting runes.
	multiByte := "éééé"
	for _, bound := range []int{-1, 0, 1, 2, 3, 4} {
		got := fitReportText(multiByte, bound)
		if bound <= 0 && got != "" {
			t.Fatalf("fitReportText(%q, %d) = %q, want empty", multiByte, bound, got)
		}
		if bound > 0 && len(got) > bound {
			t.Fatalf("fitReportText(%q, %d) len = %d > %d (%q)", multiByte, bound, len(got), bound, got)
		}
		if !utf8.ValidString(got) {
			t.Fatalf("fitReportText(%q, %d) split UTF-8 rune: %q", multiByte, bound, got)
		}
	}
}
