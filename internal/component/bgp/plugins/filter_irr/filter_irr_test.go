package filter_irr

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"sync"
	"sync/atomic"
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
	initial := mustPrefixListFromIRR(t, irr.PrefixList{
		IPv4: []netip.Prefix{netip.MustParsePrefix("10.0.0.0/24")},
	})
	plug := &irrPlugin{
		byASN: map[uint32]*asnState{
			65001: {asn: 65001, configured: "AS-TEST", asSet: "AS-TEST", list: &irrPrefixList{entries: initial}},
		},
		stopCh: make(chan struct{}),
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
		// Past OnStarted, so handleConfigure starts the resolution itself.
		started: true,
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
		// Past OnStarted, so handleConfigure starts the resolution itself.
		started: true,
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
				"filter": map[string]any{"import": map[string]any{"filters": []any{"bgp-filter-irr:65001"}, "final": "accept"}},
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
	// than on a duration. The product bounds this wait itself: refreshASN gives
	// the lookup a perASNRefreshTimeout context, the IRR client puts that
	// deadline on the connection, and every completion path, the timeout
	// included, closes firstDone. A test-side deadline shorter than that one
	// failed on a loaded machine while the lookup was still inside its own
	// budget; a lookup that overruns it closes firstDone with lastErr set, which
	// the check below reports. The test's own bound sits past the product's, so
	// it fires only when firstDone is never closed, never on a slow lookup.
	select {
	case <-st.firstDone:
	case <-time.After(perASNRefreshTimeout + 10*time.Second):
		t.Fatal("first resolution did not complete within the product's refresh bound")
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

	if !list.evaluateUpdate("nlri add 10.0.0.0/24") {
		t.Error("matching prefix should be accepted")
	}
	if list.evaluateUpdate("nlri add 192.168.0.0/24") {
		t.Error("non-matching prefix should be rejected")
	}
}

// VALIDATES: AC-4 -- empty list rejects all (fail-closed).
func TestPrefixListFromIRREmpty(t *testing.T) {
	entries := mustPrefixListFromIRR(t, irr.PrefixList{})
	list := &irrPrefixList{entries: entries}

	if list.evaluateUpdate("nlri add 10.0.0.0/24") {
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
	existing := &irrPrefixList{entries: mustPrefixListFromIRR(t, irr.PrefixList{
		IPv4: []netip.Prefix{netip.MustParsePrefix("10.0.0.0/24")},
	})}
	plug := &irrPlugin{
		byASN: map[uint32]*asnState{
			65010: {asn: 65010, asSet: "", list: existing},
		},
		// Unreachable endpoints: PeeringDB returns nothing, IRR connection refused.
		// The store falls back to AS65010 but the IRR query fails.
		prefixStore: store.New(irr.NewIRR("127.0.0.1:1"), peeringdb.NewPeeringDB("http://127.0.0.1:1"), nil),
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
		65010: {asn: 65010, configured: "AS-OTHER", asSet: "AS-OTHER"},
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
	list := &irrPrefixList{entries: mustPrefixListFromIRR(t, irr.PrefixList{
		IPv4: []netip.Prefix{netip.MustParsePrefix("10.0.0.0/24")},
	})}
	p := list.partitionUpdate("nlri add 10.0.0.0/24 192.168.0.0/24")
	if len(p.accepted) != 1 || p.accepted[0] != "10.0.0.0/24" {
		t.Errorf("accepted = %v, want [10.0.0.0/24]", p.accepted)
	}
	if len(p.rejected) != 1 || p.rejected[0] != "192.168.0.0/24" {
		t.Errorf("rejected = %v, want [192.168.0.0/24]", p.rejected)
	}
	if got := buildModifyDelta(p); got != "nlri add 10.0.0.0/24" {
		t.Errorf("delta = %q, want %q", got, "nlri add 10.0.0.0/24")
	}
}

// VALIDATES: an UPDATE whose prefixes are all out-of-list yields an empty
// accepted set (whole-update reject), and an all-in-list UPDATE yields no
// rejected entries (accept unmodified).
func TestPartitionUpdateAllOrNone(t *testing.T) {
	list := &irrPrefixList{entries: mustPrefixListFromIRR(t, irr.PrefixList{
		IPv4: []netip.Prefix{netip.MustParsePrefix("10.0.0.0/24")},
	})}
	allBad := list.partitionUpdate("nlri add 192.168.0.0/24 203.0.113.0/24")
	if len(allBad.accepted) != 0 || len(allBad.rejected) != 2 {
		t.Errorf("all-out-of-list: accepted=%v rejected=%v, want 0 accepted / 2 rejected", allBad.accepted, allBad.rejected)
	}
	allGood := list.partitionUpdate("nlri add 10.0.0.0/24")
	if len(allGood.accepted) != 1 || len(allGood.rejected) != 0 {
		t.Errorf("all-in-list: accepted=%v rejected=%v, want 1 accepted / 0 rejected", allGood.accepted, allGood.rejected)
	}
}

// fakeIRRv4 starts a TCP whois server answering "!a4<name>" queries from the
// given per-name prefix table.
func fakeIRRv4(t *testing.T, v4 map[string]string) string {
	t.Helper()
	return fakeIRRv4Hook(t, v4, nil)
}

// fakeIRRv4Hook is fakeIRRv4 with onQuery, when non-nil, called with each
// query before it is answered, so a test can count the queries or act while a
// lookup is in flight.
func fakeIRRv4Hook(t *testing.T, v4 map[string]string, onQuery func(query string)) string {
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
				if onQuery != nil {
					onQuery(query)
				}
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

// VALIDATES: TestFilterIRRUsesStore -- refreshASN delegates resolution to the
// shared in-memory PrefixStore; the resolved prefixes land in the
// plugin's per-ASN state and in the store under the ASN identity.
func TestFilterIRRUsesStore(t *testing.T) {
	addr := fakeIRRv4(t, map[string]string{"AS-TEST": "10.0.0.0/24"})
	plug := &irrPlugin{
		byASN:       map[uint32]*asnState{65001: {asn: 65001, configured: "AS-TEST", asSet: "AS-TEST"}},
		prefixStore: store.New(irr.NewIRR(addr), nil, nil),
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
		prefixStore: store.New(irr.NewIRR(addr), nil, nil),
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
		Update: "ipv4/unicast announce nlri add 10.0.0.0/24",
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

// VALIDATES: AC-1 at the BGP consumer -- an IRR server that answers "key not
// found" leaves this filter enforcing the prefixes it already had, so the peer's
// UPDATEs keep being evaluated on their merits.
// PREVENTS: an empty answer emptying a live filter. refreshASNCtx keeps st.list
// only on its error branch, so the property depends on store.Refresh reporting
// ErrNoPrefixes for an answer that learned nothing
// (internal/component/resolve/irr/store/store.go). Without that error the list
// is replaced with an empty one, handleFilterUpdate finds no entries, and every
// UPDATE from the peer is rejected.
func TestRefreshEmptyAnswerKeepsFilterList(t *testing.T) {
	good := fakeIRRv4(t, map[string]string{"AS-TEST": "10.0.0.0/24"})
	plug := &irrPlugin{
		byASN:       map[uint32]*asnState{65001: newASNState(65001, "AS-TEST")},
		prefixStore: store.New(irr.NewIRR(good), nil, nil),
		stopCh:      make(chan struct{}),
	}
	plug.refreshASN(65001)
	seeded := plug.byASN[65001].list
	if seeded == nil || len(seeded.entries) != 1 {
		t.Fatalf("seed refresh did not populate the list: %+v", seeded)
	}

	// The same AS-SET, answered now by a server that holds no key for it. A new
	// store and a new client, because the client caches a non-empty answer for an
	// hour and would replay the seed.
	plug.prefixStore = store.New(irr.NewIRR(fakeIRRv4(t, nil)), nil, nil)
	plug.refreshASN(65001)

	st := plug.byASN[65001]
	if st.list != seeded {
		t.Fatalf("the empty answer replaced the prefix list: %+v", st.list)
	}
	if st.lastErr == "" {
		t.Error("a refresh that learned nothing must be recorded, not reported as success")
	}

	out := plug.handleFilterUpdate(&sdk.FilterUpdateInput{
		Filter: "bgp-filter-irr:65001",
		Peer:   "127.0.0.1",
		Update: "ipv4/unicast announce nlri add 10.0.0.0/24",
	})
	if out.Action != sdk.FilterAccept {
		t.Errorf("action = %v, want FilterAccept: the peer's prefix is still in the last-known-good list", out.Action)
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
		prefixStore: store.New(irr.NewIRR("127.0.0.1:1"), nil, nil),
		stopCh:      make(chan struct{}),
	}

	// First resolution completes (failing) and closes firstDone, so the filter's
	// wait returns immediately and re-checks the still-empty list.
	plug.initialResolve()

	out := plug.handleFilterUpdate(&sdk.FilterUpdateInput{
		Filter: "bgp-filter-irr:65002",
		Peer:   "127.0.0.1",
		Update: "ipv4/unicast announce nlri add 10.0.0.0/24",
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
			Update: "ipv4/unicast announce nlri add 10.0.0.0/24",
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
				ps := store.New(irr.NewIRR("127.0.0.1:1"), nil, nil)
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
	existing := &irrPrefixList{entries: mustPrefixListFromIRR(t, irr.PrefixList{
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

// irrConfigFor builds the bgp subtree handleConfigure reads: one peer of AS
// 65001 importing bgp-filter-irr:65001 with the given as-set, resolved against
// server, or, with an empty asn, a config that enrolls nothing. PeeringDB is
// unreachable, so a discovery falls back.
func irrConfigFor(server, asn, asSet string) map[string]any {
	return irrConfigWithPeeringDB(server, "http://127.0.0.1:1", asn, asSet)
}

// irrConfigWithPeeringDB is irrConfigFor with the PeeringDB URL a discovery
// queries.
func irrConfigWithPeeringDB(server, peeringDBURL, asn, asSet string) map[string]any {
	policy := map[string]any{"irr": map[string]any{"server": server, "peeringdb-url": peeringDBURL}}
	if asn == "" {
		return map[string]any{"policy": policy}
	}
	return map[string]any{
		"policy": policy,
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

// seedEntry resolves AS65001 for asSet through the store's own Refresh, so the
// entry carries the as-set the store recorded rather than one a test wrote.
func seedEntry(t *testing.T, plug *irrPlugin, asSet string) {
	t.Helper()
	if _, err := plug.prefixStore.Refresh(context.Background(), "AS65001", asSet); err != nil {
		t.Fatalf("seed %s: %v", asSet, err)
	}
	if plug.prefixStore.Get("AS65001") == nil {
		t.Fatalf("seed %s: no entry", asSet)
	}
}

// VALIDATES: the prefix store is created once and kept across every later
// configure, so an entry resolved before a reload is still the base the
// store's per-family last-known-good merges the next refresh into.
// PREVENTS: a reload emptying the in-memory store. With the disk copy gone,
// a store built per configure would hold nothing after each reload, and the
// next refresh that answered one family only would drop the other.
// Method: configure, resolve an entry, reconfigure with the same as-set, and
// check the store is the same one and still holds the entry.
func TestHandleConfigureKeepsStoreAcrossReload(t *testing.T) {
	addr := fakeIRRv4(t, map[string]string{"AS-ONE": "10.0.0.0/24"})
	plug := &irrPlugin{byASN: make(map[uint32]*asnState), stopCh: make(chan struct{})}
	t.Cleanup(func() { close(plug.stopCh) })

	plug.handleConfigure(irrConfigFor(addr, "65001", "AS-ONE"))
	first := plug.prefixStore
	if first == nil {
		t.Fatal("configure built no prefix store")
	}
	seedEntry(t, plug, "AS-ONE")

	plug.handleConfigure(irrConfigFor(addr, "65001", "AS-ONE"))
	if plug.prefixStore != first {
		t.Fatal("reconfigure replaced the prefix store: the resolved entries were lost")
	}
	if first.Get("AS65001") == nil {
		t.Error("reconfigure with the same as-set dropped the resolved entry")
	}
}

// VALIDATES: a reconfigure drops from the store the entry of an ASN whose
// configured as-set changed, and the entry of an ASN no longer enrolled.
// PREVENTS: an entry resolved for a set the config no longer names, or for an
// unenrolled ASN, living in memory forever.
func TestHandleConfigureForgetsUnconfiguredEntries(t *testing.T) {
	addr := fakeIRRv4(t, map[string]string{"AS-ONE": "10.0.0.0/24"})
	plug := &irrPlugin{byASN: make(map[uint32]*asnState), stopCh: make(chan struct{})}
	t.Cleanup(func() { close(plug.stopCh) })

	plug.handleConfigure(irrConfigFor(addr, "65001", "AS-ONE"))
	seedEntry(t, plug, "AS-ONE")
	plug.handleConfigure(irrConfigFor(addr, "65001", "AS-TWO"))
	if plug.prefixStore.Get("AS65001") != nil {
		t.Error("as-set changed from AS-ONE to AS-TWO and the AS-ONE entry is still in the store")
	}

	plug.handleConfigure(irrConfigFor(addr, "65001", "AS-ONE"))
	seedEntry(t, plug, "AS-ONE")
	plug.handleConfigure(irrConfigFor(addr, "", ""))
	if plug.prefixStore.Get("AS65001") != nil {
		t.Error("AS65001 is no longer enrolled and its entry is still in the store")
	}
}

// VALIDATES: a lookup a reload superseded by changing the ASN's as-set is
// dropped, and the canceled AS-OLD lookup commits nothing: the live state
// keeps the new as-set and no list, and the store holds no AS-OLD entry, also
// after a second refresh for AS-NEW that learns nothing.
// PREVENTS: the in-flight refresh applying the old set's answer to the new
// state (the filter then enforces a set the config no longer names, with
// firstDone closed), or committing an AS-OLD entry the reload no longer wants.
// The store tests own the guard that a failed lookup never relabels an entry.
// Method: the fake IRR reconfigures to AS-NEW while it is answering the
// AS-OLD query, so the reload lands between the lookup and its apply. A second
// refresh then queries AS-NEW, which the fake IRR holds nothing for.
func TestRefreshDropsLookupAReloadSuperseded(t *testing.T) {
	plug := &irrPlugin{byASN: make(map[uint32]*asnState), stopCh: make(chan struct{})}
	t.Cleanup(func() { close(plug.stopCh) })
	var addr string
	var once sync.Once
	addr = fakeIRRv4Hook(t, map[string]string{"AS-OLD": "10.0.0.0/24"}, func(query string) {
		if query != "!a4AS-OLD" {
			return
		}
		once.Do(func() { plug.handleConfigure(irrConfigFor(addr, "65001", "AS-NEW")) })
	})
	plug.handleConfigure(irrConfigFor(addr, "65001", "AS-OLD"))

	plug.refreshASN(65001)

	plug.mu.RLock()
	st := plug.byASN[65001]
	asSet, list := st.asSet, st.list
	plug.mu.RUnlock()
	if asSet != "AS-NEW" {
		t.Errorf("live as-set = %q, want AS-NEW: the superseded lookup overwrote it", asSet)
	}
	if list != nil {
		t.Errorf("live list = %v, want none: the AS-OLD answer was applied to the AS-NEW state", list.entries)
	}

	plug.refreshASN(65001)

	plug.mu.RLock()
	asSet, list = st.asSet, st.list
	plug.mu.RUnlock()
	if asSet != "AS-NEW" {
		t.Errorf("live as-set = %q after a failed AS-NEW refresh, want AS-NEW", asSet)
	}
	if list != nil {
		t.Errorf("live list = %v after a failed AS-NEW refresh, want none", list.entries)
	}
	if entry := plug.prefixStore.Get("AS65001"); entry != nil {
		t.Errorf("store entry = %+v, want none: the reload canceled the AS-OLD lookup, so it commits nothing", entry)
	}
}

// VALIDATES: a lookup whose state a reload replaced while keeping the ASN's
// as-set is applied: the live state carries the answer, and the store keeps
// the entry.
// PREVENTS: refreshASN dropping a lookup because the state pointer changed.
// Every reload replaces the state, so a pointer-only drop would discard every
// answer that races a reconfigure, even one for the set the config still names.
// Method: as TestRefreshDropsLookupAReloadSuperseded, but the hook reconfigures
// with the same as-set, AS-OLD.
func TestRefreshAppliesLookupAReloadKeptItsASSet(t *testing.T) {
	plug := &irrPlugin{byASN: make(map[uint32]*asnState), stopCh: make(chan struct{})}
	t.Cleanup(func() { close(plug.stopCh) })
	var addr string
	var once sync.Once
	addr = fakeIRRv4Hook(t, map[string]string{"AS-OLD": "10.0.0.0/24"}, func(query string) {
		if query != "!a4AS-OLD" {
			return
		}
		once.Do(func() { plug.handleConfigure(irrConfigFor(addr, "65001", "AS-OLD")) })
	})
	plug.handleConfigure(irrConfigFor(addr, "65001", "AS-OLD"))
	captured := plug.byASN[65001]

	plug.refreshASN(65001)

	plug.mu.RLock()
	st := plug.byASN[65001]
	asSet, list := st.asSet, st.list
	plug.mu.RUnlock()
	if st == captured {
		t.Fatal("the reload did not replace the state, so the test cannot tell a pointer-only drop apart")
	}
	if asSet != "AS-OLD" {
		t.Errorf("live as-set = %q, want AS-OLD", asSet)
	}
	if list == nil || len(list.entries) != 1 {
		t.Fatalf("live list = %v, want the AS-OLD answer applied: a reload that kept the as-set dropped it", list)
	}
	if entry := plug.prefixStore.Get("AS65001"); entry == nil || entry.ASSet != "AS-OLD" {
		t.Errorf("store entry = %+v, want the AS-OLD entry kept", entry)
	}
}

// VALIDATES: a configure that runs before OnStarted, inside the startup
// handshake, sends no query to the IRR server.
// PREVENTS: startup depending on IRR reachability, which the started gate
// exists to stop: the first configure leaves resolution to OnStarted.
// Method: count the queries the fake IRR server receives, then give any
// stray resolution goroutine time to reach it.
func TestConfigureBeforeStartedQueriesNothing(t *testing.T) {
	var queries atomic.Int32
	addr := fakeIRRv4Hook(t, map[string]string{"AS-ONE": "10.0.0.0/24"}, func(string) { queries.Add(1) })
	plug := &irrPlugin{byASN: make(map[uint32]*asnState), stopCh: make(chan struct{})}
	t.Cleanup(func() { close(plug.stopCh) })

	plug.handleConfigure(irrConfigFor(addr, "65001", "AS-ONE"))
	time.Sleep(200 * time.Millisecond)

	if n := queries.Load(); n != 0 {
		t.Errorf("IRR queries = %d before OnStarted, want 0", n)
	}
	if plug.byASN[65001] == nil {
		t.Error("ASN 65001 was not enrolled: the test would pass with nothing to resolve")
	}
}

// fakePeeringDBCounting answers /api/net?asn=<n> with the AS-SET asSetByASN
// holds for n, or with no record, and counts the lookups it answers.
func fakePeeringDBCounting(t *testing.T, asSetByASN map[string]string, lookups *atomic.Int32) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lookups.Add(1)
		set, ok := asSetByASN[r.URL.Query().Get("asn")]
		if !ok {
			if _, err := w.Write([]byte(`{"data":[]}`)); err != nil {
				return
			}
			return
		}
		if _, err := fmt.Fprintf(w, `{"data":[{"irr_as_set":%q}]}`, set); err != nil {
			return
		}
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

// VALIDATES: deleting the configured as-set hands the choice back to
// PeeringDB: the reload carries no list over, and the next refresh asks
// PeeringDB and filters against the set it names.
// PREVENTS: the removed as-set riding the carry-over as the query hint, so
// discovery never runs and the ASN keeps filtering against a set the operator
// deleted until the plugin restarts.
// Method: resolve AS-OLD from config, reload with no as-set, refresh, and
// check PeeringDB was asked and its AS-PDB answer is live.
func TestRemovingConfiguredASSetRediscoversThroughPeeringDB(t *testing.T) {
	var lookups atomic.Int32
	pdbURL := fakePeeringDBCounting(t, map[string]string{"65001": "AS-PDB"}, &lookups)
	addr := fakeIRRv4(t, map[string]string{"AS-OLD": "10.0.0.0/24", "AS-PDB": "192.0.2.0/24"})
	plug := &irrPlugin{byASN: make(map[uint32]*asnState), stopCh: make(chan struct{})}
	t.Cleanup(func() { close(plug.stopCh) })

	plug.handleConfigure(irrConfigWithPeeringDB(addr, pdbURL, "65001", "AS-OLD"))
	plug.refreshASN(65001)
	if got := liveASSet(t, plug); got != "AS-OLD" {
		t.Fatalf("as-set after the first refresh = %q, want AS-OLD", got)
	}

	plug.handleConfigure(irrConfigWithPeeringDB(addr, pdbURL, "65001", ""))

	plug.mu.RLock()
	st := plug.byASN[65001]
	asSet, list := st.asSet, st.list
	plug.mu.RUnlock()
	if asSet != "" {
		t.Errorf("as-set after the reload = %q, want none: the removed set is still the query hint", asSet)
	}
	if list != nil {
		t.Errorf("list after the reload = %v, want none: the removed set's list was carried over", list.entries)
	}
	if entry := plug.prefixStore.Get("AS65001"); entry != nil {
		t.Errorf("store entry after the reload = %+v, want none: the removed set's entry stays cached", entry)
	}

	plug.refreshASN(65001)

	if n := lookups.Load(); n != 1 {
		t.Errorf("PeeringDB lookups = %d, want 1: the refresh after the removal did not rediscover", n)
	}
	plug.mu.RLock()
	asSet, list = st.asSet, st.list
	plug.mu.RUnlock()
	if asSet != "AS-PDB" {
		t.Errorf("as-set after the refresh = %q, want AS-PDB from PeeringDB", asSet)
	}
	if list == nil || !list.evaluateUpdate("nlri add 192.0.2.0/24") {
		t.Error("192.0.2.0/24 is not accepted: the AS-PDB list is not live")
	}
	if list != nil && list.evaluateUpdate("nlri add 10.0.0.0/24") {
		t.Error("10.0.0.0/24 is accepted: the removed AS-OLD list is still live")
	}
}

// VALIDATES: after the configured as-set is deleted, a PeeringDB outage falls
// back to the ASN itself, never to the deleted set.
// PREVENTS: store.discoverASSet keeping the cached AS-SET on a PeeringDB
// failure, which would hand the deleted set back if its entry stayed cached.
// Method: PeeringDB unreachable; resolve AS-OLD, reload with no as-set,
// refresh, and read the as-set the refresh queried.
func TestRemovedASSetIsNotThePeeringDBFallback(t *testing.T) {
	addr := fakeIRRv4(t, map[string]string{"AS-OLD": "10.0.0.0/24"})
	plug := &irrPlugin{byASN: make(map[uint32]*asnState), stopCh: make(chan struct{})}
	t.Cleanup(func() { close(plug.stopCh) })

	plug.handleConfigure(irrConfigFor(addr, "65001", "AS-OLD"))
	plug.refreshASN(65001)
	plug.handleConfigure(irrConfigFor(addr, "65001", ""))
	plug.refreshASN(65001)

	if got := liveASSet(t, plug); got != "AS65001" {
		t.Errorf("as-set after PeeringDB failed = %q, want AS65001: the deleted set came back as the fallback", got)
	}
}

// VALIDATES: a reload that configures no as-set before and after keeps the
// discovered list and set, and the next refresh asks PeeringDB as every
// refresh with no configured set does.
// PREVENTS: the configured/discovered split turning every reload into an
// empty list.
// Method: count PeeringDB lookups across discover, reload, refresh.
func TestReloadKeepsADiscoveredASSet(t *testing.T) {
	var lookups atomic.Int32
	pdbURL := fakePeeringDBCounting(t, map[string]string{"65001": "AS-PDB"}, &lookups)
	addr := fakeIRRv4(t, map[string]string{"AS-PDB": "192.0.2.0/24"})
	plug := &irrPlugin{byASN: make(map[uint32]*asnState), stopCh: make(chan struct{})}
	t.Cleanup(func() { close(plug.stopCh) })

	plug.handleConfigure(irrConfigWithPeeringDB(addr, pdbURL, "65001", ""))
	plug.refreshASN(65001)
	plug.handleConfigure(irrConfigWithPeeringDB(addr, pdbURL, "65001", ""))

	plug.mu.RLock()
	st := plug.byASN[65001]
	asSet, list := st.asSet, st.list
	plug.mu.RUnlock()
	if asSet != "AS-PDB" {
		t.Errorf("as-set after the reload = %q, want the discovered AS-PDB", asSet)
	}
	if list == nil {
		t.Error("list after the reload is none, want the discovered list carried over")
	}

	plug.refreshASN(65001)

	if n := lookups.Load(); n != 2 {
		t.Errorf("PeeringDB lookups = %d, want 2: one per refresh, with no configured set", n)
	}
	if got := liveASSet(t, plug); got != "AS-PDB" {
		t.Errorf("as-set after the refresh = %q, want AS-PDB", got)
	}
}

// liveASSet reads AS 65001's live as-set under the plugin lock.
func liveASSet(t *testing.T, plug *irrPlugin) string {
	t.Helper()
	plug.mu.RLock()
	defer plug.mu.RUnlock()
	st := plug.byASN[65001]
	if st == nil {
		t.Fatal("ASN 65001 is not enrolled")
	}
	return st.asSet
}

// VALIDATES: a lookup for a configured as-set still in flight when a reload
// deletes it commits nothing, so a PeeringDB outage afterwards falls back to
// the ASN itself rather than to the deleted set.
// PREVENTS: the superseded lookup installing AS-OLD after the reload purged
// it, which store.discoverASSet would hand back as the cached AS-SET.
// Method: the fake IRR holds the AS-OLD query until the reload is done;
// PeeringDB is unreachable.
func TestInFlightLookupForADeletedSetCommitsNothing(t *testing.T) {
	reached := make(chan struct{})
	gate := make(chan struct{})
	var once sync.Once
	addr := fakeIRRv4Hook(t, map[string]string{"AS-OLD": "10.0.0.0/24"}, func(query string) {
		if query != "!a4AS-OLD" {
			return
		}
		once.Do(func() { close(reached) })
		<-gate
	})
	plug := &irrPlugin{byASN: make(map[uint32]*asnState), stopCh: make(chan struct{})}
	t.Cleanup(func() { close(plug.stopCh) })
	plug.handleConfigure(irrConfigFor(addr, "65001", "AS-OLD"))

	done := make(chan struct{})
	go func() {
		defer close(done)
		plug.refreshASN(65001)
	}()
	<-reached
	plug.handleConfigure(irrConfigFor(addr, "65001", ""))
	close(gate)
	<-done

	if entry := plug.prefixStore.Get("AS65001"); entry != nil {
		t.Errorf("store entry = %+v, want none: the superseded AS-OLD lookup committed after the purge", entry)
	}
	plug.refreshASN(65001)
	if got := liveASSet(t, plug); got != "AS65001" {
		t.Errorf("as-set after PeeringDB failed = %q, want AS65001: the deleted set came back", got)
	}
}

// switchablePeeringDB is a fake PeeringDB whose answer and availability a
// test changes between refreshes.
type switchablePeeringDB struct {
	mu    sync.Mutex
	down  bool
	asSet string
}

func (f *switchablePeeringDB) set(down bool, asSet string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.down, f.asSet = down, asSet
}

func (f *switchablePeeringDB) serve(t *testing.T) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		f.mu.Lock()
		down, asSet := f.down, f.asSet
		f.mu.Unlock()
		if down {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		if _, err := fmt.Fprintf(w, `{"data":[{"irr_as_set":%q}]}`, asSet); err != nil {
			return
		}
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

// VALIDATES: with no configured as-set every refresh asks PeeringDB: after an
// outage at the first lookup the next refresh picks PeeringDB's set up, an
// upstream AS-SET change is followed once the PeeringDB client's one-hour
// answer cache no longer holds the old one, and an outage after that keeps
// the cached set.
// PREVENTS: the first answer, or the AS<asn> literal an outage produced,
// becoming the query hint for the plugin's life.
// Method: switch the fake PeeringDB between refreshes and read the live set.
// A fresh PeeringDB client (UseClients, as a reload installs) stands for the
// client's cache expiring, which a test cannot wait an hour for.
func TestDiscoveryFollowsPeeringDB(t *testing.T) {
	pdb := &switchablePeeringDB{}
	pdbURL := pdb.serve(t)
	addr := fakeIRRv4(t, map[string]string{
		"AS65001": "198.51.100.0/24", "AS-ONE": "192.0.2.0/24", "AS-TWO": "203.0.113.0/24",
	})
	plug := &irrPlugin{byASN: make(map[uint32]*asnState), stopCh: make(chan struct{})}
	t.Cleanup(func() { close(plug.stopCh) })
	plug.handleConfigure(irrConfigWithPeeringDB(addr, pdbURL, "65001", ""))

	steps := []struct {
		down    bool
		asSet   string
		expired bool
		want    string
	}{
		{down: true, want: "AS65001"},
		{asSet: "AS-ONE", want: "AS-ONE"},
		{asSet: "AS-TWO", expired: true, want: "AS-TWO"},
		{down: true, expired: true, want: "AS-TWO"},
	}
	for i, step := range steps {
		pdb.set(step.down, step.asSet)
		if step.expired {
			plug.prefixStore.UseClients(irr.NewIRR(addr), peeringdb.NewPeeringDB(pdbURL))
		}
		plug.refreshASN(65001)
		if got := liveASSet(t, plug); got != step.want {
			t.Errorf("step %d (PeeringDB down=%v, as-set %q): live as-set = %q, want %q", i, step.down, step.asSet, got, step.want)
		}
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
