package filter_irr

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/netip"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/resolve/irr"
	"github.com/ze-software/ze/internal/component/resolve/irr/store"
	"github.com/ze-software/ze/internal/component/resolve/peeringdb"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// VALIDATES: AC-5 -- concurrent filter evaluation during refresh sees consistent state.
// PREVENTS: Torn reads when refresh goroutine swaps the prefix-list while filter
// evaluation is in progress.
func TestRefreshSwapsAtomically(t *testing.T) {
	initial := prefixListFromIRR(irr.PrefixList{
		IPv4: []netip.Prefix{netip.MustParsePrefix("10.0.0.0/24")},
	})
	plug := &irrPlugin{
		byASN: map[uint32]*asnState{
			65001: {asn: 65001, asSet: "AS-TEST", list: &irrPrefixList{entries: initial}},
		},
		stopCh: make(chan struct{}),
	}

	replacement := prefixListFromIRR(irr.PrefixList{
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
		now := time.Now()
		plug.mu.Lock()
		st := plug.byASN[65001]
		if i%2 == 0 {
			st.list = &irrPrefixList{entries: replacement}
		} else {
			st.list = &irrPrefixList{entries: initial}
		}
		st.lastOK = now
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
	cfg := parseIRRConfig(bgpCfg)

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
	cfg := parseIRRConfig(bgpCfg)

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
// for a peer that never opted into IRR filtering, so initialResolve has nothing
// to resolve and the plugin issues no PeeringDB or IRR whois request.
// PREVENTS: the unsolicited live lookup that any `bgp { peer ... }` config used
// to trigger. Enrollment keyed on "peer has a remote ASN" alone, so every BGP
// config -- and therefore every functional test with a BGP peer -- opened an
// HTTPS connection to www.peeringdb.com and a whois connection to the IRR server
// (default whois.radb.net) at startup. Driven through handleConfigure, not the
// parser, because the enrollment decision is what gates the network I/O.
func TestHandleConfigureDoesNotEnrollPeerWithoutIRR(t *testing.T) {
	plug := &irrPlugin{
		byASN:  make(map[uint32]*asnState),
		stopCh: make(chan struct{}),
	}
	t.Cleanup(func() { close(plug.stopCh) })

	// A peer with a remote ASN and nothing else: no filter chain, no as-set.
	// Point the server at an address nothing listens on, so if this test ever
	// regresses into resolving, it fails here rather than reaching the internet.
	plug.handleConfigure(map[string]any{
		"policy": map[string]any{
			"irr": map[string]any{"server": "127.0.0.1:1", "peeringdb-url": "http://127.0.0.1:1"},
		},
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

// VALIDATES: handleConfigure DOES enroll and resolve a peer whose filter chain
// names bgp-filter-irr, against a fake IRR server -- the gate above rejects the
// unsolicited case without disabling the feature it gates.
// PREVENTS: the enrollment gate silently disabling IRR filtering for a correctly
// configured peer (fail-closed guards must still let the intended path through).
func TestHandleConfigureEnrollsIRRFilteredPeer(t *testing.T) {
	addr := fakeIRRv4(t, map[string]string{"AS-CUSTOMER1": "10.0.0.0/24"})
	plug := &irrPlugin{
		byASN:  make(map[uint32]*asnState),
		stopCh: make(chan struct{}),
	}
	t.Cleanup(func() { close(plug.stopCh) })

	// An explicit as-set keeps this hermetic: store.resolve only calls PeeringDB
	// when the as-set is empty, so the fake IRR server is the only endpoint used.
	plug.handleConfigure(map[string]any{
		"policy": map[string]any{"irr": map[string]any{"server": addr}},
		"peer": map[string]any{
			"10.0.0.1": map[string]any{
				"session": map[string]any{
					"asn": map[string]any{"remote": "65001"},
					"irr": map[string]any{"as-set": "AS-CUSTOMER1"},
				},
				"filter": map[string]any{"import": []any{"bgp-filter-irr:65001"}},
			},
		},
	})

	plug.mu.RLock()
	st := plug.byASN[65001]
	plug.mu.RUnlock()
	if st == nil {
		t.Fatal("ASN 65001 not enrolled: an IRR-filtered peer must be resolved")
	}
	// initialResolve runs detached; wait on its per-ASN completion signal rather
	// than on a duration.
	select {
	case <-st.firstDone:
	case <-time.After(10 * time.Second):
		t.Fatal("first resolution did not complete")
	}
	plug.mu.RLock()
	defer plug.mu.RUnlock()
	if st.lastErr != "" {
		t.Fatalf("unexpected lastErr: %s", st.lastErr)
	}
	if st.list == nil || len(st.list.entries) != 1 {
		t.Fatalf("prefix-list not populated from the fake IRR server: %+v", st.list)
	}
}

// VALIDATES: AC-21 -- show bgp irr includes last-refresh and next-refresh timestamps.
func TestShowIRRIncludesTimestamps(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	plug := &irrPlugin{
		byASN:       map[uint32]*asnState{},
		config:      &irrConfig{Server: "whois.radb.net"},
		lastRefresh: now,
		nextRefresh: now.Add(time.Hour),
	}

	_, data, err := plug.showIRR()
	if err != nil {
		t.Fatalf("showIRR error: %v", err)
	}
	raw, ok := data.(json.RawMessage)
	if !ok {
		t.Fatalf("data is %T, want json.RawMessage", data)
	}
	var parsed map[string]any
	if err := json.Unmarshal(raw, &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, ok := parsed["last-refresh"]; !ok {
		t.Error("show bgp irr output missing last-refresh")
	}
	if _, ok := parsed["next-refresh"]; !ok {
		t.Error("show bgp irr output missing next-refresh")
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

	entries := prefixListFromIRR(pl)

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
	entries := prefixListFromIRR(pl)
	list := &irrPrefixList{entries: entries}

	if !list.evaluateUpdate("ipv4/unicast add 10.0.0.0/24") {
		t.Error("matching prefix should be accepted")
	}
	if list.evaluateUpdate("ipv4/unicast add 192.168.0.0/24") {
		t.Error("non-matching prefix should be rejected")
	}
}

// VALIDATES: AC-4 -- empty list rejects all (fail-closed).
func TestPrefixListFromIRREmpty(t *testing.T) {
	entries := prefixListFromIRR(irr.PrefixList{})
	list := &irrPrefixList{entries: entries}

	if list.evaluateUpdate("ipv4/unicast add 10.0.0.0/24") {
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

// VALIDATES: when PeeringDB returns no AS-SET and IRR is unreachable,
// `update bgp irr asn` fails (returns statusError) and leaves the existing
// prefix-list untouched. PREVENTS: a failed refresh wiping last-known-good data.
func TestUpdateASNFailurePreservesState(t *testing.T) {
	existing := &irrPrefixList{entries: prefixListFromIRR(irr.PrefixList{
		IPv4: []netip.Prefix{netip.MustParsePrefix("10.0.0.0/24")},
	})}
	plug := &irrPlugin{
		byASN: map[uint32]*asnState{
			65010: {asn: 65010, asSet: "", list: existing},
		},
		// Unreachable endpoints: PeeringDB returns nothing, IRR connection refused.
		// The store falls back to AS65010 but the IRR query fails.
		prefixStore: store.New(irr.NewIRR("127.0.0.1:1"), peeringdb.NewPeeringDB("http://127.0.0.1:1"), ""),
	}

	status, _, err := plug.updateASN([]string{"65010"})
	if status != statusError || err == nil {
		t.Fatalf("updateASN status=%q err=%v, want statusError + non-nil err", status, err)
	}

	plug.mu.RLock()
	got := plug.byASN[65010].list
	lastErr := plug.byASN[65010].lastErr
	asSet := plug.byASN[65010].asSet
	plug.mu.RUnlock()
	if got != existing {
		t.Error("prefix-list was replaced on a failed refresh; want last-known-good preserved")
	}
	if lastErr == "" {
		t.Error("lastErr not recorded for a failed refresh")
	}
	if asSet != "AS65010" {
		t.Errorf("asSet = %q, want AS65010 (fallback from empty PeeringDB)", asSet)
	}
}

// VALIDATES: `update bgp irr asn <asn>` for an ASN with no IRR-filtered peer
// fails rather than silently reporting success.
func TestUpdateASNUnknownASN(t *testing.T) {
	plug := &irrPlugin{byASN: map[uint32]*asnState{}}
	status, _, err := plug.updateASN([]string{"65099"})
	if status != statusError || err == nil {
		t.Fatalf("updateASN unknown ASN status=%q err=%v, want statusError + non-nil err", status, err)
	}
}

// VALIDATES: `update bgp irr as-set <as-set>` for an AS-SET used by no peer fails.
func TestUpdateASSetUnused(t *testing.T) {
	plug := &irrPlugin{byASN: map[uint32]*asnState{
		65010: {asn: 65010, asSet: "AS-OTHER"},
	}}
	status, _, err := plug.updateASSet([]string{"AS-NOBODY"})
	if status != statusError || err == nil {
		t.Fatalf("updateASSet unused AS-SET status=%q err=%v, want statusError + non-nil err", status, err)
	}
}

// VALIDATES: a mixed UPDATE (one in-list prefix, one out-of-list) is partitioned
// so only the in-list prefix is kept and the modify delta carries just that
// subset. PREVENTS: a single unauthorized prefix collaterally dropping the
// legitimate routes that share the same UPDATE (all-or-nothing reject).
func TestPartitionUpdateMixed(t *testing.T) {
	list := &irrPrefixList{entries: prefixListFromIRR(irr.PrefixList{
		IPv4: []netip.Prefix{netip.MustParsePrefix("10.0.0.0/24")},
	})}
	p := list.partitionUpdate("ipv4/unicast add 10.0.0.0/24 192.168.0.0/24")
	if len(p.accepted) != 1 || p.accepted[0] != "10.0.0.0/24" {
		t.Errorf("accepted = %v, want [10.0.0.0/24]", p.accepted)
	}
	if len(p.rejected) != 1 || p.rejected[0] != "192.168.0.0/24" {
		t.Errorf("rejected = %v, want [192.168.0.0/24]", p.rejected)
	}
	if got := buildModifyDelta(p); got != "nlri ipv4/unicast add 10.0.0.0/24" {
		t.Errorf("delta = %q, want %q", got, "nlri ipv4/unicast add 10.0.0.0/24")
	}
}

// VALIDATES: an UPDATE whose prefixes are all out-of-list yields an empty
// accepted set (whole-update reject), and an all-in-list UPDATE yields no
// rejected entries (accept unmodified).
func TestPartitionUpdateAllOrNone(t *testing.T) {
	list := &irrPrefixList{entries: prefixListFromIRR(irr.PrefixList{
		IPv4: []netip.Prefix{netip.MustParsePrefix("10.0.0.0/24")},
	})}
	allBad := list.partitionUpdate("ipv4/unicast add 192.168.0.0/24 203.0.113.0/24")
	if len(allBad.accepted) != 0 || len(allBad.rejected) != 2 {
		t.Errorf("all-out-of-list: accepted=%v rejected=%v, want 0 accepted / 2 rejected", allBad.accepted, allBad.rejected)
	}
	allGood := list.partitionUpdate("ipv4/unicast add 10.0.0.0/24")
	if len(allGood.accepted) != 1 || len(allGood.rejected) != 0 {
		t.Errorf("all-in-list: accepted=%v rejected=%v, want 1 accepted / 0 rejected", allGood.accepted, allGood.rejected)
	}
}

// fakeIRRv4 starts a TCP whois server answering "!a4<name>" queries from the
// given per-name prefix table.
func fakeIRRv4(t *testing.T, v4 map[string]string) string {
	t.Helper()
	lc := net.ListenConfig{}
	ln, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		for {
			conn, acceptErr := ln.Accept()
			if acceptErr != nil {
				return
			}
			go func(c net.Conn) {
				defer func() { _ = c.Close() }()
				buf := make([]byte, 4096)
				n, readErr := c.Read(buf)
				if readErr != nil {
					return
				}
				query := strings.TrimSpace(string(buf[:n]))
				if prefixes, ok := v4[strings.TrimPrefix(query, "!a4")]; ok && strings.HasPrefix(query, "!a4") && prefixes != "" {
					if _, err := fmt.Fprintf(c, "A1\n%s\nC\n", prefixes); err != nil {
						return
					}
					return
				}
				if _, err := fmt.Fprint(c, "D\n"); err != nil {
					return
				}
			}(conn)
		}
	}()
	return ln.Addr().String()
}

// VALIDATES: TestFilterIRRUsesStore -- refreshASN delegates resolution and
// persistence to the shared PrefixStore; the resolved prefixes land in the
// plugin's per-ASN state and in the store under the ASN identity.
func TestFilterIRRUsesStore(t *testing.T) {
	addr := fakeIRRv4(t, map[string]string{"AS-TEST": "10.0.0.0/24"})
	plug := &irrPlugin{
		byASN:       map[uint32]*asnState{65001: {asn: 65001, asSet: "AS-TEST"}},
		prefixStore: store.New(irr.NewIRR(addr), nil, ""),
	}

	plug.refreshASN(65001)

	st := plug.byASN[65001]
	if st.lastErr != "" {
		t.Fatalf("unexpected lastErr: %s", st.lastErr)
	}
	if st.list == nil || len(st.list.entries) != 1 {
		t.Fatalf("refreshASN did not populate list from store: %+v", st.list)
	}
	if st.v4Count != 1 {
		t.Errorf("v4Count = %d, want 1", st.v4Count)
	}
	if plug.prefixStore.Get("AS65001") == nil {
		t.Error("store has no entry for AS65001 after refresh")
	}
}

// VALIDATES: an IRR-filtered UPDATE that arrives BEFORE the first background
// resolution has populated the prefix-list waits on the ASN's firstDone signal
// (rather than rejecting with "no-prefix-list") and is then filtered on its
// merits once the background resolution completes. This is the startup-resolution
// race the filter-layer wait closes: configure resolves IRR asynchronously, so
// the very first UPDATE for a configured-IRR peer can race the resolution.
// PREVENTS: a configured-IRR peer's first UPDATE being spuriously rejected during
// the background-resolution window.
func TestFilterWaitsForFirstResolution(t *testing.T) {
	addr := fakeIRRv4(t, map[string]string{"AS-TEST": "10.0.0.0/24"})
	plug := &irrPlugin{
		// firstDone armed (open) and list nil: exactly the state configure leaves
		// an enrolled ASN in before the detached resolution runs.
		byASN:       map[uint32]*asnState{65001: newASNState(65001, "AS-TEST")},
		prefixStore: store.New(irr.NewIRR(addr), nil, ""),
		stopCh:      make(chan struct{}),
	}

	// Detach the first resolution, exactly as handleConfigure now does. A short
	// delay makes the filter UPDATE below genuinely arrive while the list is still
	// nil, so it must exercise the bounded firstDone wait rather than reading a
	// pre-populated list.
	go func() {
		time.Sleep(50 * time.Millisecond)
		plug.initialResolve()
	}()

	// The filter UPDATE blocks on firstDone (bounded by firstResolveWait), then
	// re-reads the now-populated list and accepts the in-list prefix.
	out := plug.handleFilterUpdate(&sdk.FilterUpdateInput{
		Filter: "bgp-filter-irr:65001",
		Peer:   "127.0.0.1",
		Update: "ipv4/unicast announce nlri ipv4/unicast add 10.0.0.0/24",
	})
	if out.Action == sdk.FilterReject {
		st := plug.byASN[65001]
		t.Fatalf("in-list prefix rejected while/after first resolution (list=%v lastErr=%q); want accept after wait",
			st.list, st.lastErr)
	}
	if out.Action != sdk.FilterAccept {
		t.Errorf("action = %v, want FilterAccept", out.Action)
	}
}

// VALIDATES: the firstDone wait does NOT turn a genuinely-empty resolution into
// an indefinite hang. When the first resolution completes but yields nothing
// (IRR unreachable, no cached data), it closes firstDone with an empty list; the
// filter's re-check then falls through to the unchanged fail-closed reject.
// PREVENTS: regressing the fail-closed semantics for a configured-IRR peer whose
// IRR legitimately resolves to no prefixes.
func TestFilterFailClosedAfterEmptyResolution(t *testing.T) {
	plug := &irrPlugin{
		// IRR endpoint refused: resolution fails, no list is populated, firstDone
		// is closed by the failed attempt.
		byASN:       map[uint32]*asnState{65002: newASNState(65002, "AS-NONE")},
		prefixStore: store.New(irr.NewIRR("127.0.0.1:1"), nil, ""),
		stopCh:      make(chan struct{}),
	}

	// First resolution completes (failing) and closes firstDone, so the filter's
	// wait returns immediately and re-checks the still-empty list.
	plug.initialResolve()

	out := plug.handleFilterUpdate(&sdk.FilterUpdateInput{
		Filter: "bgp-filter-irr:65002",
		Peer:   "127.0.0.1",
		Update: "ipv4/unicast announce nlri ipv4/unicast add 10.0.0.0/24",
	})
	if out.Action != sdk.FilterReject {
		t.Errorf("action = %v, want FilterReject (fail-closed on empty list)", out.Action)
	}
}

// VALIDATES: the firstDone wait is bounded -- an IRR-filtered UPDATE whose ASN
// resolution never completes (firstDone never closed, e.g. resolution still in
// flight far longer than firstResolveWait) is not blocked forever; it falls
// through to the fail-closed reject once the wait elapses. Uses a deliberately
// short stub by closing stopCh, which also releases the wait, but the key
// property under test is that handleFilterUpdate returns rather than hanging.
func TestFilterWaitIsBounded(t *testing.T) {
	plug := &irrPlugin{
		// firstDone armed and never closed; list stays nil (no resolution runs).
		byASN:  map[uint32]*asnState{65003: newASNState(65003, "AS-SLOW")},
		stopCh: make(chan struct{}),
	}

	// Release the wait promptly via stopCh so the test stays fast while still
	// proving handleFilterUpdate returns (does not block on a never-closed
	// firstDone) and fails closed when no list ever materializes.
	go func() {
		time.Sleep(50 * time.Millisecond)
		close(plug.stopCh)
	}()

	done := make(chan *sdk.FilterUpdateOutput, 1)
	go func() {
		done <- plug.handleFilterUpdate(&sdk.FilterUpdateInput{
			Filter: "bgp-filter-irr:65003",
			Peer:   "127.0.0.1",
			Update: "ipv4/unicast announce nlri ipv4/unicast add 10.0.0.0/24",
		})
	}()

	select {
	case out := <-done:
		if out.Action != sdk.FilterReject {
			t.Errorf("action = %v, want FilterReject (fail-closed, no list resolved)", out.Action)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("handleFilterUpdate did not return; firstDone wait is not bounded")
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

// VALIDATES: refreshASN reads plug.prefixStore AND re-reads plug.byASN under
// plug.mu, so a reconfigure that swaps both (as handleConfigure does) cannot
// data-race with an in-flight refresh. PREVENTS: a torn read of the store
// pointer, and writing a refresh result into an orphaned asnState after byASN
// is replaced. Run with -race.
func TestRefreshASNStoreFieldRace(t *testing.T) {
	plug := &irrPlugin{
		byASN:  map[uint32]*asnState{65001: {asn: 65001}},
		stopCh: make(chan struct{}),
	}
	var wg sync.WaitGroup
	stop := make(chan struct{})

	// Writer: reassign the store and byASN under plug.mu, as handleConfigure does.
	wg.Go(func() {
		for {
			select {
			case <-stop:
				return
			default:
				ps := store.New(irr.NewIRR("127.0.0.1:1"), nil, "")
				plug.mu.Lock()
				plug.prefixStore = ps
				plug.byASN = map[uint32]*asnState{65001: {asn: 65001}}
				plug.mu.Unlock()
			}
		}
	})

	// Reader: refreshASN reads the store pointer and byASN; must be under plug.mu.
	for range 200 {
		plug.refreshASN(65001)
	}
	close(stop)
	wg.Wait()
}

// VALIDATES: refreshASN is a safe no-op when prefixStore is nil (config never
// ran, or store open failed) -- no panic, no list mutation.
func TestRefreshASNNilStore(t *testing.T) {
	existing := &irrPrefixList{entries: prefixListFromIRR(irr.PrefixList{
		IPv4: []netip.Prefix{netip.MustParsePrefix("10.0.0.0/24")},
	})}
	plug := &irrPlugin{
		byASN:  map[uint32]*asnState{65001: {asn: 65001, list: existing}},
		stopCh: make(chan struct{}),
		// prefixStore intentionally nil
	}
	plug.refreshASN(65001) // must not panic
	if plug.byASN[65001].list != existing {
		t.Error("nil prefixStore: existing list must be left untouched")
	}
}

// VALIDATES: showIRR escapes the configured server, so a server value with a
// quote cannot inject extra JSON keys into `show bgp irr` output.
func TestShowIRRServerEscaped(t *testing.T) {
	plug := &irrPlugin{
		byASN:  map[uint32]*asnState{},
		config: &irrConfig{Server: `whois.example","injected":"x`},
	}
	_, data, err := plug.showIRR()
	if err != nil {
		t.Fatalf("showIRR: %v", err)
	}
	raw, ok := data.(json.RawMessage)
	if !ok {
		t.Fatalf("data is %T, want json.RawMessage", data)
	}
	var parsed map[string]any
	if err := json.Unmarshal(raw, &parsed); err != nil {
		t.Fatalf("output is not valid JSON (injection): %v", err)
	}
	if _, injected := parsed["injected"]; injected {
		t.Error("server value injected an extra JSON key; not escaped")
	}
	if parsed["server"] != `whois.example","injected":"x` {
		t.Errorf("server = %v, want the raw value preserved as one string", parsed["server"])
	}
}
