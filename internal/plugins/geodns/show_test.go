package geodns

import (
	"testing"

	"github.com/ze-software/ze/internal/component/plugin"
)

// VALIDATES: `show geodns` reports status fields from the published snapshot
// (enabled, bind, zones, host-set/source counts, SOA serial).
// PREVENTS: the status command drifting from the actual running server state.
func TestHandleShowGeoDNS(t *testing.T) {
	cfg, err := parseConfig(fullConfig)
	if err != nil {
		t.Fatalf("parseConfig: %v", err)
	}
	storeApplied(cfg, 42)

	resp, err := handleShowGeoDNS(nil, nil)
	if err != nil {
		t.Fatalf("handleShowGeoDNS: %v", err)
	}
	data, ok := resp.Data.(plugin.Map)
	if !ok {
		t.Fatalf("resp.Data is %T, want plugin.Map", resp.Data)
	}
	if data["enabled"] != true {
		t.Errorf("enabled = %v, want true", data["enabled"])
	}
	listeners, ok := data["listeners"].([]string)
	if !ok || len(listeners) != 2 || listeners[0] != "127.0.0.1:5300" {
		t.Errorf("listeners = %v, want [127.0.0.1:5300 [::1]:5300]", data["listeners"])
	}
	if data["soa-serial"] != uint32(42) {
		t.Errorf("soa-serial = %v, want 42", data["soa-serial"])
	}
	if data["host-sets"] != 2 {
		t.Errorf("host-sets = %v, want 2", data["host-sets"])
	}
}
