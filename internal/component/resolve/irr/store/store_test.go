package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/ze-software/ze/internal/component/resolve/irr"
	"github.com/ze-software/ze/internal/component/resolve/peeringdb"
	"github.com/ze-software/ze/pkg/zefs"
)

// fakeIRR starts a TCP whois server answering "!a4<name>" / "!a6<name>" queries
// from the given per-name prefix tables. Returns the server address.
func fakeIRR(t *testing.T, v4, v6 map[string]string) string {
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
				var table map[string]string
				var name string
				switch {
				case strings.HasPrefix(query, "!a4"):
					table, name = v4, query[3:]
				case strings.HasPrefix(query, "!a6"):
					table, name = v6, query[3:]
				default:
					if _, err := fmt.Fprint(c, "D\n"); err != nil {
						return
					}
					return
				}
				if prefixes, ok := table[name]; ok && prefixes != "" {
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

// fakePeeringDB returns an AS-SET for known ASNs via the /api/net endpoint.
func fakePeeringDB(t *testing.T, asSetByASN map[string]string) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asn := r.URL.Query().Get("asn")
		set, ok := asSetByASN[asn]
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

func tempStorePath(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "database.zefs")
	bs, err := zefs.Create(path)
	if err != nil {
		t.Fatalf("create zefs: %v", err)
	}
	if err := bs.Close(); err != nil {
		t.Fatalf("close zefs: %v", err)
	}
	return path
}

// VALIDATES: AC-1, AC-2 -- Refresh resolves an AS-SET via IRR, persists to zefs,
// and updates the in-memory cache.
func TestPrefixStoreRefresh(t *testing.T) {
	addr := fakeIRR(t,
		map[string]string{"AS-TEST": "10.0.0.0/24"},
		map[string]string{"AS-TEST": "2001:db8::/32"},
	)
	path := tempStorePath(t)
	s := New(irr.NewIRR(addr), nil, path)

	entry, err := s.Refresh(context.Background(), "AS-TEST", "")
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if len(entry.IPv4) != 1 || len(entry.IPv6) != 1 {
		t.Fatalf("entry v4=%d v6=%d, want 1,1", len(entry.IPv4), len(entry.IPv6))
	}
	if s.Get("AS-TEST") == nil {
		t.Fatal("Get returned nil after Refresh")
	}

	// Persisted under meta/irr/AS-TEST.
	bs, err := zefs.Open(path)
	if err != nil {
		t.Fatalf("open zefs: %v", err)
	}
	defer func() { _ = bs.Close() }()
	if !bs.Has(zefs.KeyIRRPrefixCache.Key("AS-TEST")) {
		t.Error("entry not persisted under meta/irr/AS-TEST")
	}
}

// VALIDATES: AC-4, AC-5 -- Get returns the cached entry, or nil when absent.
func TestPrefixStoreGet(t *testing.T) {
	addr := fakeIRR(t, map[string]string{"AS-TEST": "10.0.0.0/24"}, nil)
	s := New(irr.NewIRR(addr), nil, "")

	if s.Get("AS-TEST") != nil {
		t.Fatal("Get before Refresh should be nil")
	}
	if _, err := s.Refresh(context.Background(), "AS-TEST", ""); err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if s.Get("AS-TEST") == nil {
		t.Fatal("Get after Refresh should be non-nil")
	}
	if s.Get("AS99999") != nil {
		t.Fatal("Get for uncached name should be nil")
	}
}

// TestPrefixStoreRefreshAll (AC-6) and TestPrefixStoreList (AC-11)
// removed. List and RefreshAll were descoped from this spec (no in-tree consumer
// -- the BGP plugin uses Get/Refresh/Open and its own per-ASN refreshAllNow) and
// moved to spec-firewall-irr, which will re-add them with their consumer and
// tests. No coverage lost for delivered code.

// VALIDATES: data survives store close and reopen via zefs persistence.
func TestPrefixStorePersistence(t *testing.T) {
	addr := fakeIRR(t, map[string]string{"AS-TEST": "10.0.0.0/24"}, nil)
	path := tempStorePath(t)

	s1 := New(irr.NewIRR(addr), nil, path)
	if _, err := s1.Refresh(context.Background(), "AS-TEST", ""); err != nil {
		t.Fatalf("Refresh: %v", err)
	}

	s2 := New(irr.NewIRR(addr), nil, path)
	if err := s2.Open(); err != nil {
		t.Fatalf("Open: %v", err)
	}
	got := s2.Get("AS-TEST")
	if got == nil || len(got.IPv4) != 1 {
		t.Fatalf("reopened entry = %+v, want IPv4 len 1", got)
	}
}

// VALIDATES: AC-7 -- a failed refresh preserves existing cached data.
// Uses a fresh client pointed at an unreachable server so the IRR client's 1h
// in-memory cache cannot mask the failure (see spec Review Gate note #5).
func TestPrefixStoreRefreshError(t *testing.T) {
	addr := fakeIRR(t, map[string]string{"AS-TEST": "10.0.0.0/24"}, nil)
	path := tempStorePath(t)

	good := New(irr.NewIRR(addr), nil, path)
	if _, err := good.Refresh(context.Background(), "AS-TEST", ""); err != nil {
		t.Fatalf("seed Refresh: %v", err)
	}

	// Fresh store, unreachable IRR, loading the seeded data from disk.
	bad := New(irr.NewIRR("127.0.0.1:1"), nil, path)
	if err := bad.Open(); err != nil {
		t.Fatalf("Open: %v", err)
	}
	if _, err := bad.Refresh(context.Background(), "AS-TEST", "AS-TEST"); err == nil {
		t.Fatal("Refresh against unreachable IRR should error")
	}
	got := bad.Get("AS-TEST")
	if got == nil || len(got.IPv4) != 1 {
		t.Fatalf("cached entry not preserved after failed refresh: %+v", got)
	}
}

// VALIDATES: AC-3 -- a bare ASN with no AS-SET triggers PeeringDB discovery.
func TestPrefixStorePeeringDBFallback(t *testing.T) {
	addr := fakeIRR(t, map[string]string{"AS-CLOUDFLARE": "1.1.1.0/24"}, nil)
	pdbURL := fakePeeringDB(t, map[string]string{"13335": "AS-CLOUDFLARE"})
	s := New(irr.NewIRR(addr), peeringdb.NewPeeringDB(pdbURL), "")

	entry, err := s.Refresh(context.Background(), "AS13335", "")
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if entry.ASSet != "AS-CLOUDFLARE" {
		t.Errorf("entry.ASSet = %q, want AS-CLOUDFLARE (discovered via PeeringDB)", entry.ASSet)
	}
	if len(entry.IPv4) != 1 {
		t.Errorf("entry.IPv4 = %d, want 1", len(entry.IPv4))
	}
}

// VALIDATES: AC-12 -- a bare ASN with no AS-SET and no PeeringDB result falls
// back to querying IRR with the literal "AS<asn>" name.
func TestPrefixStoreLiteralFallback(t *testing.T) {
	addr := fakeIRR(t, map[string]string{"AS65001": "65.0.0.0/24"}, nil)
	s := New(irr.NewIRR(addr), nil, "") // no PeeringDB

	entry, err := s.Refresh(context.Background(), "AS65001", "")
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if entry.ASSet != "AS65001" {
		t.Errorf("entry.ASSet = %q, want AS65001 (literal fallback)", entry.ASSet)
	}
	if len(entry.IPv4) != 1 {
		t.Errorf("entry.IPv4 = %d, want 1", len(entry.IPv4))
	}
}

// VALIDATES: AC-10 -- the legacy single-blob cache is migrated to per-entry
// keys and the legacy key is removed.
func TestMigrateOldCache(t *testing.T) {
	path := tempStorePath(t)

	legacy := []legacyEntry{{
		ASN:   13335,
		ASSet: "AS-CLOUDFLARE",
		IPv4:  []string{"1.1.1.0/24"},
		IPv6:  []string{"2606:4700::/32"},
	}}
	blob, err := json.Marshal(legacy)
	if err != nil {
		t.Fatalf("marshal legacy: %v", err)
	}
	bs, err := zefs.Open(path)
	if err != nil {
		t.Fatalf("open zefs: %v", err)
	}
	if err := bs.WriteFile(zefs.KeyIRRCache.Pattern, blob, 0); err != nil {
		t.Fatalf("write legacy: %v", err)
	}
	if err := bs.Close(); err != nil {
		t.Fatalf("close zefs: %v", err)
	}

	s := New(irr.NewIRR("127.0.0.1:1"), nil, path)
	if err := s.Open(); err != nil {
		t.Fatalf("Open: %v", err)
	}

	got := s.Get("AS13335")
	if got == nil {
		t.Fatal("migrated entry AS13335 not found")
	}
	if got.ASSet != "AS-CLOUDFLARE" || len(got.IPv4) != 1 || len(got.IPv6) != 1 {
		t.Fatalf("migrated entry = %+v, want AS-CLOUDFLARE with 1 v4 + 1 v6", got)
	}

	bs2, err := zefs.Open(path)
	if err != nil {
		t.Fatalf("reopen zefs: %v", err)
	}
	defer func() { _ = bs2.Close() }()
	if bs2.Has(zefs.KeyIRRCache.Pattern) {
		t.Error("legacy key not removed after migration")
	}
	if !bs2.Has(zefs.KeyIRRPrefixCache.Key("AS13335")) {
		t.Error("per-entry key meta/irr/AS13335 not written")
	}
}

// VALIDATES: A-5 -- AS-SET names containing ':' are valid zefs keys and survive
// persistence (hierarchical IRR names like RIPE::AS-FOO).
func TestPrefixStoreColonKey(t *testing.T) {
	const name = "AS-FOO:AS-BAR"
	addr := fakeIRR(t, map[string]string{name: "203.0.113.0/24"}, nil)
	path := tempStorePath(t)

	s1 := New(irr.NewIRR(addr), nil, path)
	if _, err := s1.Refresh(context.Background(), name, ""); err != nil {
		t.Fatalf("Refresh: %v", err)
	}

	s2 := New(irr.NewIRR(addr), nil, path)
	if err := s2.Open(); err != nil {
		t.Fatalf("Open: %v", err)
	}
	if s2.Get(name) == nil {
		t.Fatalf("entry %q with ':' not persisted/reloaded", name)
	}
}

// VALIDATES: ASN boundary handling in parseBareASN. Valid uint32 range is
// 1..4294967295; 0 and overflow are not bare ASNs, and AS-SET names (hyphen,
// colon) are never bare ASNs.
func TestParseBareASNBoundaries(t *testing.T) {
	tests := []struct {
		name    string
		wantASN uint32
		wantOK  bool
	}{
		{"0", 0, false}, // below valid range
		{"AS0", 0, false},
		{"1", 1, true}, // first valid
		{"AS1", 1, true},
		{"4294967294", 4294967294, true},
		{"AS4294967295", 4294967295, true}, // max uint32
		{"4294967296", 0, false},           // overflow uint32
		{"", 0, false},
		{"AS", 0, false},
		{"AS-CLOUDFLARE", 0, false}, // AS-SET, not a bare ASN
		{"RIPE::AS-FOO", 0, false},
	}
	for _, tt := range tests {
		asn, ok := parseBareASN(tt.name)
		if ok != tt.wantOK || asn != tt.wantASN {
			t.Errorf("parseBareASN(%q) = (%d, %v), want (%d, %v)", tt.name, asn, ok, tt.wantASN, tt.wantOK)
		}
	}
}

// VALIDATES: names that are not valid zefs path segments (".", "..", "a..b") are
// rejected, so they can never poison the shared store file -- a "meta/irr/."
// key fails fs.ValidPath at decode and would make the whole file unreadable.
func TestPrefixStoreRejectsBadName(t *testing.T) {
	addr := fakeIRR(t, map[string]string{"AS-OK": "10.0.0.0/24"}, nil)
	path := tempStorePath(t)
	s := New(irr.NewIRR(addr), nil, path)

	for _, bad := range []string{".", "..", "a..b"} {
		if _, err := s.Refresh(context.Background(), bad, ""); err == nil {
			t.Errorf("Refresh(%q) should be rejected", bad)
		}
	}

	// A good entry still persists and the store reopens cleanly afterwards.
	if _, err := s.Refresh(context.Background(), "AS-OK", ""); err != nil {
		t.Fatalf("Refresh(AS-OK): %v", err)
	}
	s2 := New(irr.NewIRR(addr), nil, path)
	if err := s2.Open(); err != nil {
		t.Fatalf("Open after bad-name attempts: %v", err)
	}
	if s2.Get("AS-OK") == nil {
		t.Error("store unreadable or AS-OK missing after bad-name attempts")
	}
}

// VALIDATES: concurrent in-process Refreshes on distinct names do not lose each
// other's persisted entry. Each persist opens the file, adds its key, and
// atomic-renames the whole file; without fileMu serialization a racing writer's
// flush would clobber a key. Run with -race.
func TestPrefixStoreConcurrentPersist(t *testing.T) {
	names := []string{"AS-A", "AS-B", "AS-C", "AS-D", "AS-E", "AS-F"}
	v4 := make(map[string]string, len(names))
	for i, n := range names {
		v4[n] = fmt.Sprintf("10.%d.0.0/24", i)
	}
	addr := fakeIRR(t, v4, nil)
	path := tempStorePath(t)
	s := New(irr.NewIRR(addr), nil, path)

	var wg sync.WaitGroup
	for _, n := range names {
		wg.Add(1)
		go func(name string) {
			defer wg.Done()
			if _, err := s.Refresh(context.Background(), name, ""); err != nil {
				t.Errorf("Refresh(%s): %v", name, err)
			}
		}(n)
	}
	wg.Wait()

	// Every entry must have survived to disk (reopen from a fresh store).
	s2 := New(irr.NewIRR(addr), nil, path)
	if err := s2.Open(); err != nil {
		t.Fatalf("Open: %v", err)
	}
	for _, n := range names {
		if s2.Get(n) == nil {
			t.Errorf("entry %s lost on disk after concurrent Refresh", n)
		}
	}
}

// VALIDATES: migration of a legacy entry with multiple prefixes per family
// preserves every valid prefix and silently drops unparseable ones.
func TestMigrateMultiPrefix(t *testing.T) {
	path := tempStorePath(t)
	legacy := []legacyEntry{{
		ASN:   13335,
		ASSet: "AS-CF",
		IPv4:  []string{"1.1.1.0/24", "JUNK", "1.0.0.0/24", "1.2.3.0/24"},
		IPv6:  []string{"2606:4700::/32", "2400:cb00::/32"},
	}}
	blob, err := json.Marshal(legacy)
	if err != nil {
		t.Fatalf("marshal legacy: %v", err)
	}
	bs, err := zefs.Open(path)
	if err != nil {
		t.Fatalf("open zefs: %v", err)
	}
	if err := bs.WriteFile(zefs.KeyIRRCache.Pattern, blob, 0); err != nil {
		t.Fatalf("write legacy: %v", err)
	}
	if err := bs.Close(); err != nil {
		t.Fatalf("close zefs: %v", err)
	}

	s := New(irr.NewIRR("127.0.0.1:1"), nil, path)
	if err := s.Open(); err != nil {
		t.Fatalf("Open: %v", err)
	}
	got := s.Get("AS13335")
	if got == nil {
		t.Fatal("migrated entry missing")
	}
	if len(got.IPv4) != 3 { // "JUNK" dropped, three valid kept
		t.Errorf("IPv4 = %d, want 3 (one malformed dropped)", len(got.IPv4))
	}
	if len(got.IPv6) != 2 {
		t.Errorf("IPv6 = %d, want 2", len(got.IPv6))
	}
}

// VALIDATES: Open skips an entry whose JSON Name does not match its zefs key
// segment, so a corrupt/tampered blob cannot poison another name's slot.
func TestPrefixStoreOpenNameKeyMismatch(t *testing.T) {
	path := tempStorePath(t)
	bs, err := zefs.Open(path)
	if err != nil {
		t.Fatalf("open zefs: %v", err)
	}
	// Blob under key meta/irr/AS13335 whose Name falsely claims AS99999.
	mb, err := json.Marshal(&CachedEntry{Name: "AS99999", ASSet: "AS-X", IPv4: []netip.Prefix{netip.MustParsePrefix("10.0.0.0/24")}})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := bs.WriteFile(zefs.KeyIRRPrefixCache.Key("AS13335"), mb, 0); err != nil {
		t.Fatalf("write mismatch: %v", err)
	}
	// A correctly-named entry to prove valid ones still load.
	ob, err := json.Marshal(&CachedEntry{Name: "AS64500", IPv4: []netip.Prefix{netip.MustParsePrefix("10.1.0.0/24")}})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := bs.WriteFile(zefs.KeyIRRPrefixCache.Key("AS64500"), ob, 0); err != nil {
		t.Fatalf("write ok: %v", err)
	}
	if err := bs.Close(); err != nil {
		t.Fatalf("close zefs: %v", err)
	}

	s := New(irr.NewIRR("127.0.0.1:1"), nil, path)
	if err := s.Open(); err != nil {
		t.Fatalf("Open: %v", err)
	}
	if s.Get("AS99999") != nil {
		t.Error("tampered entry (name != key) loaded under its claimed name")
	}
	if s.Get("AS13335") != nil {
		t.Error("tampered entry loaded under its key segment")
	}
	if s.Get("AS64500") == nil {
		t.Error("correctly-named entry should still load")
	}
}

// fakeIRRReply starts a TCP whois server that answers each exact query string
// with the raw RPSL reply mapped to it. An unmapped query gets "D\n" (key not
// found), which is what a real server sends for a name it does not hold.
func fakeIRRReply(t *testing.T, replies map[string]string) string {
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
				reply, ok := replies[strings.TrimSpace(string(buf[:n]))]
				if !ok {
					reply = "D\n"
				}
				if _, wErr := fmt.Fprint(c, reply); wErr != nil {
					return
				}
			}(conn)
		}
	}()
	return ln.Addr().String()
}

// seedStore refreshes name from a server holding v4Prefix and returns the zefs
// path the entry was persisted to.
func seedStore(t *testing.T, name, v4Prefix string) string {
	t.Helper()
	addr := fakeIRR(t, map[string]string{name: v4Prefix}, nil)
	path := tempStorePath(t)
	good := New(irr.NewIRR(addr), nil, path)
	if _, err := good.Refresh(context.Background(), name, ""); err != nil {
		t.Fatalf("seed Refresh: %v", err)
	}
	return path
}

// VALIDATES: AC-1 -- an IRR answer carrying no prefixes never overwrites a
// populated entry, in memory or in the persisted cache.
// PREVENTS: a bad minute upstream emptying a live prefix filter for an hour.
func TestRefreshKeepsLastGoodOnEmptyAnswer(t *testing.T) {
	for _, tc := range []struct {
		name  string
		reply string
	}{
		{"key-not-found", "D\n"},
		{"query-ok-no-records", "C\n"},
		{"server-error", "F query failed\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := seedStore(t, "AS-TEST", "10.0.0.0/24")
			addr := fakeIRRReply(t, map[string]string{
				"!a4AS-TEST": tc.reply,
				"!a6AS-TEST": tc.reply,
			})

			s := New(irr.NewIRR(addr), nil, path)
			if err := s.Open(); err != nil {
				t.Fatalf("Open: %v", err)
			}
			if _, err := s.Refresh(context.Background(), "AS-TEST", "AS-TEST"); err == nil {
				t.Fatal("an answer carrying no prefixes must not report success")
			}

			got := s.Get("AS-TEST")
			if got == nil || len(got.IPv4) != 1 {
				t.Fatalf("in-memory entry not preserved: %+v", got)
			}
			if !got.Stale() {
				t.Error("an entry enforced after a refresh learned nothing must report itself stale")
			}

			reopened := New(irr.NewIRR(addr), nil, path)
			if err := reopened.Open(); err != nil {
				t.Fatalf("reopen: %v", err)
			}
			persisted := reopened.Get("AS-TEST")
			if persisted == nil || len(persisted.IPv4) != 1 {
				t.Fatalf("persisted entry not preserved: %+v", persisted)
			}
			if !persisted.Stale() {
				t.Error("staleness must survive a restart")
			}
		})
	}
}

// seedBothFamilies caches one IPv4 and one IPv6 prefix for name and returns the
// zefs path they were persisted to. seedStore above seeds IPv4 alone, so it
// cannot show a family being lost.
func seedBothFamilies(t *testing.T, name string) string {
	t.Helper()
	addr := fakeIRR(t,
		map[string]string{name: "10.0.0.0/24"},
		map[string]string{name: "2001:db8::/32"},
	)
	path := tempStorePath(t)
	seed := New(irr.NewIRR(addr), nil, path)
	if _, err := seed.Refresh(context.Background(), name, name); err != nil {
		t.Fatalf("seed Refresh: %v", err)
	}
	return path
}

// VALIDATES: AC-1 -- last-known-good is decided per FAMILY. An answer that
// carries one family and nothing for the other keeps the cached prefixes of the
// silent family, dates the entry by them, and marks it stale.
// PREVENTS: the interface binding losing one family's accept term while the
// drop term that closes the whitelist stays, which drops every packet of that
// family (internal/component/firewall/plugins/irr/sets.go, buildIfaceTables).
// A server answers "D" both for a family it does not hold and for a family with
// no route objects, so an empty family is never evidence that prefixes are gone.
func TestRefreshKeepsLastGoodPerFamily(t *testing.T) {
	for _, tc := range []struct {
		name    string
		replies map[string]string
		wantV4  int
		wantV6  int
	}{
		{
			name:    "ipv6-answers-nothing",
			replies: map[string]string{"!a4AS-TEST": "A2\n10.0.0.0/24\n10.1.0.0/24\nC\n"},
			wantV4:  2,
			wantV6:  1,
		},
		{
			name:    "ipv4-answers-nothing",
			replies: map[string]string{"!a6AS-TEST": "A2\n2001:db8::/32\n2001:dba::/32\nC\n"},
			wantV4:  1,
			wantV6:  2,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := seedBothFamilies(t, "AS-TEST")
			addr := fakeIRRReply(t, tc.replies) // every unmapped query answers "D"

			s := New(irr.NewIRR(addr), nil, path)
			if err := s.Open(); err != nil {
				t.Fatalf("Open: %v", err)
			}
			seeded := s.Get("AS-TEST")
			if seeded == nil {
				t.Fatal("seeded entry not loaded")
			}

			entry, err := s.Refresh(context.Background(), "AS-TEST", "AS-TEST")
			if err != nil {
				t.Fatalf("a refresh that learned one family must not report a failure: %v", err)
			}
			if len(entry.IPv4) != tc.wantV4 || len(entry.IPv6) != tc.wantV6 {
				t.Fatalf("entry v4=%d v6=%d, want %d,%d", len(entry.IPv4), len(entry.IPv6), tc.wantV4, tc.wantV6)
			}
			if !entry.Stale() {
				t.Error("an entry enforcing a family nobody confirmed must report itself stale")
			}
			if !entry.RefreshedAt.Equal(seeded.RefreshedAt) {
				t.Errorf("RefreshedAt = %v, want the kept family's date %v: the age of the oldest data is what an operator reads",
					entry.RefreshedAt, seeded.RefreshedAt)
			}

			reopened := New(irr.NewIRR(addr), nil, path)
			if err := reopened.Open(); err != nil {
				t.Fatalf("reopen: %v", err)
			}
			persisted := reopened.Get("AS-TEST")
			if persisted == nil || len(persisted.IPv4) != tc.wantV4 || len(persisted.IPv6) != tc.wantV6 {
				t.Fatalf("persisted entry not preserved: %+v", persisted)
			}
			if !persisted.Stale() {
				t.Error("staleness must survive a restart")
			}
		})
	}
}

// VALIDATES: AC-1 -- an empty answer for a name that was never cached stores
// nothing, so no consumer sees an entry that exists and holds no prefixes.
// PREVENTS: a zero-prefix entry reading as a valid answer (ai/rules/evidence.md).
func TestRefreshStoresNothingOnFirstEmptyAnswer(t *testing.T) {
	addr := fakeIRRReply(t, nil) // every query answers "D"
	s := New(irr.NewIRR(addr), nil, tempStorePath(t))

	if _, err := s.Refresh(context.Background(), "AS-TEST", "AS-TEST"); !errors.Is(err, ErrNoPrefixes) {
		t.Fatalf("Refresh error = %v, want ErrNoPrefixes", err)
	}
	if got := s.Get("AS-TEST"); got != nil {
		t.Fatalf("empty answer cached an entry: %+v", got)
	}
}

// VALIDATES: AC-1 -- a successful refresh clears the staleness a previous empty
// answer recorded.
// PREVENTS: an entry reporting stale data forever after one bad answer.
func TestRefreshClearsStaleness(t *testing.T) {
	path := seedStore(t, "AS-TEST", "10.0.0.0/24")

	empty := New(irr.NewIRR(fakeIRRReply(t, nil)), nil, path)
	if err := empty.Open(); err != nil {
		t.Fatalf("Open: %v", err)
	}
	if _, err := empty.Refresh(context.Background(), "AS-TEST", "AS-TEST"); err == nil {
		t.Fatal("expected an error for an empty answer")
	}
	if got := empty.Get("AS-TEST"); got == nil || !got.Stale() {
		t.Fatalf("entry should be stale: %+v", got)
	}

	good := New(irr.NewIRR(fakeIRR(t, map[string]string{"AS-TEST": "10.0.0.0/24"}, nil)), nil, path)
	if err := good.Open(); err != nil {
		t.Fatalf("Open: %v", err)
	}
	if _, err := good.Refresh(context.Background(), "AS-TEST", "AS-TEST"); err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if got := good.Get("AS-TEST"); got == nil || got.Stale() {
		t.Fatalf("a refresh that learned prefixes must clear staleness: %+v", got)
	}
}

// VALIDATES: AC-6 -- Purge is the deliberate exit from last-known-good, removing
// the entry from memory and from the persisted cache.
// PREVENTS: a deregistered AS-SET being enforced forever because empty answers
// no longer clear it.
func TestPurgeRemovesEntry(t *testing.T) {
	path := seedStore(t, "AS-TEST", "10.0.0.0/24")

	s := New(irr.NewIRR(fakeIRRReply(t, nil)), nil, path)
	if err := s.Open(); err != nil {
		t.Fatalf("Open: %v", err)
	}
	if s.Get("AS-TEST") == nil {
		t.Fatal("seeded entry not loaded")
	}
	if !s.Purge("AS-TEST") {
		t.Fatal("Purge reported nothing removed")
	}
	if got := s.Get("AS-TEST"); got != nil {
		t.Fatalf("entry still in memory after Purge: %+v", got)
	}
	if s.Purge("AS-TEST") {
		t.Error("Purge of an absent name must report nothing removed")
	}
	// A name the store can never hold must not reach zefs key substitution,
	// which panics on "..".
	for _, bad := range []string{"", "..", ".", "RIPE::AS-FOO/../x"} {
		if s.Purge(bad) {
			t.Errorf("Purge(%q) reported a removal", bad)
		}
	}

	reopened := New(irr.NewIRR(fakeIRRReply(t, nil)), nil, path)
	if err := reopened.Open(); err != nil {
		t.Fatalf("reopen: %v", err)
	}
	if got := reopened.Get("AS-TEST"); got != nil {
		t.Fatalf("entry still persisted after Purge: %+v", got)
	}
}
