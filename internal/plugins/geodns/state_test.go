package geodns

import (
	"net/netip"
	"testing"
)

// VALIDATES: buildState builds a longest-prefix matcher from the config's
// sources, and the atomic holder round-trips a published snapshot.
// PREVENTS: a reload publishing a state the server/show path cannot read, or a
// matcher that ignores the configured sources.
func TestBuildStateAndStore(t *testing.T) {
	cfg, err := parseConfig(fullConfig)
	if err != nil {
		t.Fatalf("parseConfig: %v", err)
	}
	st := buildState(cfg)
	if st.matcher == nil {
		t.Fatal("buildState produced a nil matcher")
	}
	if len(st.cfg.Listeners) != 2 {
		t.Errorf("state cfg listeners = %d, want 2", len(st.cfg.Listeners))
	}
	set, ok := st.matcher.lookup(netip.MustParseAddr("82.219.4.10"))
	if !ok || set != "internal" {
		t.Errorf("lookup(82.219.4.10) = (%q,%v), want (internal,true)", set, ok)
	}

	storeState(st)
	if loadState() != st {
		t.Error("loadState did not return the stored snapshot")
	}
}
