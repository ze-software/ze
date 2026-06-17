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

	"codeberg.org/thomas-mangin/ze/internal/component/resolve/irr"
	"codeberg.org/thomas-mangin/ze/internal/component/resolve/irr/store"
	"codeberg.org/thomas-mangin/ze/internal/component/resolve/peeringdb"
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
