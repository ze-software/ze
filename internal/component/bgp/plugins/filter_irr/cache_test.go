package filter_irr

import (
	"net/netip"
	"os"
	"path/filepath"
	"testing"

	"codeberg.org/thomas-mangin/ze/pkg/zefs"
)

// VALIDATES: AC-16 -- resolved prefix-lists stored in zefs and survive restart.
// PREVENTS: IRR prefix data lost on restart.

func TestCacheRoundTrip(t *testing.T) {
	dir := t.TempDir()
	storePath := filepath.Join(dir, "test.zefs")

	store, err := zefs.Create(storePath)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("close store: %v", err)
	}

	plug := &irrPlugin{
		byASN:  make(map[uint32]*asnState),
		stopCh: make(chan struct{}),
	}

	plug.byASN[65001] = &asnState{
		asn:   65001,
		asSet: "AS-TEST",
		list: &irrPrefixList{entries: []prefixEntry{
			{prefix: netip.MustParsePrefix("10.0.0.0/24"), ge: 24, le: 32},
			{prefix: netip.MustParsePrefix("2001:db8::/32"), ge: 32, le: 128},
		}},
		v4Count: 1,
		v6Count: 1,
	}

	plug.saveCacheTo(storePath)

	plug2 := &irrPlugin{
		byASN:  make(map[uint32]*asnState),
		stopCh: make(chan struct{}),
	}
	plug2.byASN[65001] = &asnState{asn: 65001}

	plug2.loadCacheFrom(storePath)

	st := plug2.byASN[65001]
	if st.list == nil {
		t.Fatal("cached list not loaded")
	}
	if len(st.list.entries) != 2 {
		t.Fatalf("entries = %d, want 2", len(st.list.entries))
	}
	if st.asSet != "AS-TEST" {
		t.Errorf("asSet = %q, want AS-TEST", st.asSet)
	}
	if st.v4Count != 1 || st.v6Count != 1 {
		t.Errorf("counts v4=%d v6=%d, want 1,1", st.v4Count, st.v6Count)
	}
}

func TestCacheLoadMissingFile(t *testing.T) {
	plug := &irrPlugin{
		byASN:  make(map[uint32]*asnState),
		stopCh: make(chan struct{}),
	}
	plug.loadCacheFrom(filepath.Join(os.TempDir(), "nonexistent-irr-test.zefs"))
}
