package filter_irr

import (
	"context"
	"net/netip"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/resolve/irr"
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

// oversizedIRRList builds an IRR answer whose IPv4 half alone fills the cap and
// whose IPv6 half carries one prefix.
func oversizedIRRList() irr.PrefixList {
	pl := irr.PrefixList{IPv6: []netip.Prefix{netip.MustParsePrefix("2001:db8::/32")}}
	pl.IPv4 = make([]netip.Prefix, 0, maxPrefixEntries)
	for i := range maxPrefixEntries {
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

// mustPrefixListFromIRR is prefixListFromIRR for a list under the cap.
func mustPrefixListFromIRR(t *testing.T, pl irr.PrefixList) []prefixEntry {
	t.Helper()
	entries, err := prefixListFromIRR(pl)
	if err != nil {
		t.Fatalf("prefixListFromIRR: %v", err)
	}
	return entries
}
