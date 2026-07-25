package filter_irr

import (
	"encoding/json"
	"net/netip"
	"path/filepath"
	"testing"

	"github.com/ze-software/ze/internal/component/resolve/irr"
	"github.com/ze-software/ze/internal/component/resolve/irr/store"
	"github.com/ze-software/ze/pkg/zefs"
)

// seedStore writes the given entries to a fresh zefs file and returns its path.
func seedStore(t *testing.T, entries ...store.CachedEntry) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "database.zefs")
	bs, err := zefs.Create(path)
	if err != nil {
		t.Fatalf("create zefs: %v", err)
	}
	for _, e := range entries {
		entry := e
		blob, mErr := json.Marshal(&entry)
		if mErr != nil {
			t.Fatalf("marshal: %v", mErr)
		}
		if wErr := bs.WriteFile(zefs.KeyIRRPrefixCache.Key(entry.Name), blob, 0); wErr != nil {
			t.Fatalf("write %s: %v", entry.Name, wErr)
		}
	}
	if err := bs.Close(); err != nil {
		t.Fatalf("close zefs: %v", err)
	}
	return path
}

// VALIDATES: AC-8/AC-9 wiring -- loadFromStore applies cached prefixes only to
// enrolled ASNs, ignoring store entries the plugin never enrolled (the
// enrollment gate the legacy single-blob cache enforced).
// PREVENTS: IRR prefix data lost on restart, and a shared store leaking other
// consumers' entries into the BGP filter.
func TestLoadFromStoreEnrolledOnly(t *testing.T) {
	path := seedStore(t,
		store.CachedEntry{Name: "AS65001", ASSet: "AS-ONE", IPv4: []netip.Prefix{netip.MustParsePrefix("10.0.0.0/24")}},
		store.CachedEntry{Name: "AS65999", ASSet: "AS-OTHER", IPv4: []netip.Prefix{netip.MustParsePrefix("10.9.0.0/24")}},
	)

	plug := &irrPlugin{
		byASN: map[uint32]*asnState{
			65001: {asn: 65001}, // enrolled, no list -> filled from store
			65002: {asn: 65002}, // enrolled, no store entry -> stays empty
		},
		prefixStore: store.New(irr.NewIRR("127.0.0.1:1"), nil, path),
	}
	if err := plug.prefixStore.Open(); err != nil {
		t.Fatalf("Open: %v", err)
	}
	plug.loadFromStore()

	st := plug.byASN[65001]
	if st.list == nil || len(st.list.entries) != 1 {
		t.Fatalf("AS65001 list not loaded from store: %+v", st.list)
	}
	if st.asSet != "AS-ONE" {
		t.Errorf("AS65001 asSet = %q, want AS-ONE", st.asSet)
	}
	if plug.byASN[65002].list != nil {
		t.Error("AS65002 has no store entry; list should remain nil")
	}
	if _, found := plug.byASN[65999]; found {
		t.Error("non-enrolled AS65999 leaked into byASN")
	}
}

// VALIDATES: loadFromStore on a missing zefs file is a no-op (no panic, no list).
func TestLoadFromStoreMissingFile(t *testing.T) {
	plug := &irrPlugin{
		byASN:       map[uint32]*asnState{65001: {asn: 65001}},
		prefixStore: store.New(irr.NewIRR("127.0.0.1:1"), nil, filepath.Join(t.TempDir(), "absent.zefs")),
	}
	if err := plug.prefixStore.Open(); err != nil {
		t.Fatalf("Open: %v", err)
	}
	plug.loadFromStore()
	if plug.byASN[65001].list != nil {
		t.Error("no store file: list should remain nil")
	}
}

// VALIDATES: loadFromStore does not clobber an already-populated list -- a
// second load (e.g. after a refresh) must preserve the live list even when the
// store has its own entry for that ASN.
func TestLoadFromStorePreservesExistingList(t *testing.T) {
	path := seedStore(t,
		store.CachedEntry{Name: "AS65001", ASSet: "AS-STORE", IPv4: []netip.Prefix{netip.MustParsePrefix("10.9.0.0/24")}},
	)
	existing := &irrPrefixList{entries: prefixListFromIRR(irr.PrefixList{
		IPv4: []netip.Prefix{netip.MustParsePrefix("10.0.0.0/24")},
	})}
	plug := &irrPlugin{
		byASN:       map[uint32]*asnState{65001: {asn: 65001, asSet: "AS-LIVE", list: existing}},
		prefixStore: store.New(irr.NewIRR("127.0.0.1:1"), nil, path),
	}
	if err := plug.prefixStore.Open(); err != nil {
		t.Fatalf("Open: %v", err)
	}
	plug.loadFromStore()
	if plug.byASN[65001].list != existing {
		t.Error("loadFromStore clobbered an already-populated list")
	}
}
