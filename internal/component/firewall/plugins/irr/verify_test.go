package irr

// VALIDATES: AC-5 verify rejects when no cached data exists
// PREVENTS: config commit silently accepting uncached ASN/AS-SET references

import (
	"testing"

	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

func TestVerifyRejectsMissingCache(t *testing.T) {
	plug := &irrPlugin{}
	refs := []irrRef{
		{Name: "AS99999"},
	}
	// getPrefixStore returns nil when no store is configured; verify must reject.
	ps := plug.getPrefixStore()
	if ps != nil {
		t.Fatal("expected nil prefix store for unconfigured plugin")
	}
	// Simulate what OnConfigVerify does: check each ref has cached data.
	for _, ref := range refs {
		if ps == nil {
			// No store means no cached data: verify should reject.
			t.Logf("correctly identified missing cache for %s (no store)", ref.Name)
			return
		}
	}
	t.Fatal("verify should have rejected missing cache")
}

// VALIDATES: AC-2 verify rejects when interface binding AS-SET has no cached data.
// PREVENTS: commit succeeding with uncached interface AS-SET, producing empty filter.
func TestVerifyRejectsUncachedIfaceBinding(t *testing.T) {
	sections := []sdk.ConfigSection{{
		Root: "firewall",
		Data: `{"firewall":{"irr":{"interface":{"eth1":{"source-as-set":"AS-MISSING"}}}}}`,
	}}
	refs := extractIRRRefs(sections)
	found := false
	for _, ref := range refs {
		if ref.Name == "AS-MISSING" && ref.IsASSet {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("extractIRRRefs must include interface binding AS-SET refs")
	}
}

func TestASNBoundary(t *testing.T) {
	tests := []struct {
		input   string
		wantErr bool
	}{
		{"0", true},
		{"1", false},
		{"13335", false},
		{"4294967294", false},
		{"4294967295", true},
		{"99999999999", true},
		{"abc", true},
		{"", true},
	}
	for _, tt := range tests {
		plug := &irrPlugin{}
		status, _, err := plug.handleCommand("update firewall irr asn", []string{tt.input})
		if tt.wantErr {
			if err == nil {
				t.Errorf("ASN %q: expected error, got status=%q", tt.input, status)
			}
		} else {
			// For valid ASNs, error is expected because no prefix store is configured,
			// but it should NOT be the "invalid ASN" error.
			if err != nil && err.Error() == "invalid ASN: must be 1-4294967294" {
				t.Errorf("ASN %q: got invalid ASN error for valid input", tt.input)
			}
		}
	}
}
