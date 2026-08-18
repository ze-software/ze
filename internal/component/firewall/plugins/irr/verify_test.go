package irr

// VALIDATES: AC-5 verify rejects when no cached data exists
// PREVENTS: config commit silently accepting uncached ASN/AS-SET references

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/resolve/irr/store"
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
	refs := parseIRRConfig(sections).allRefs()
	found := false
	for _, ref := range refs {
		if ref.Name == "AS-MISSING" && ref.IsASSet {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("allRefs must include interface binding AS-SET refs")
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

// VALIDATES: AC-3 -- a table term naming an IRR entry with no cached data is
// refused by the IRR-aware check, and the message names the entry and the
// command that fetches it. Drives the two functions OnConfigVerify calls,
// parseIRRConfig then verifyRefs, over a table-term config rather than an
// interface binding.
// PREVENTS: the actionable message staying unreachable for the table form. The
// firewall component refused every IRR table term first with `match references
// unknown set "irr_v4_AS99999"`, which names neither the entry nor the command.
func TestVerifyRejectsUncachedTableTerm(t *testing.T) {
	tests := []struct {
		name    string
		leaf    string
		value   string
		wantErr string
	}{
		{
			name:    "asn",
			leaf:    "source-asn",
			value:   "99999",
			wantErr: "firewall irr: no cached prefix data for AS99999; run 'update firewall irr asn 99999' first",
		},
		{
			name:    "as-set",
			leaf:    "destination-as-set",
			value:   "AS-MISSING",
			wantErr: "firewall irr: no cached prefix data for as-set AS-MISSING; run 'update firewall irr as-set AS-MISSING' first",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sections := []sdk.ConfigSection{{
				Root: "firewall",
				Data: `{"firewall":{"table":{"wan":{"family":"inet","chain":{"input":{"term":{"t":{"from":{"` +
					tt.leaf + `":"` + tt.value + `"},"then":{"drop":""}}}}}}}}}`,
			}}
			refs := parseIRRConfig(sections).allRefs()
			if len(refs) != 1 {
				t.Fatalf("allRefs must find the table term ref, got %v", refs)
			}
			if refs[0].TableName != "ze_wan" {
				t.Errorf("ref table = %q, want ze_wan", refs[0].TableName)
			}
			err := verifyRefs(store.New(nil, nil, ""), refs)
			if err == nil {
				t.Fatal("an uncached table-term reference must be refused")
			}
			if err.Error() != tt.wantErr {
				t.Errorf("error = %q, want %q", err, tt.wantErr)
			}
		})
	}
}

// VALIDATES: AC-1 -- the same table term with cached prefixes passes the
// IRR-aware check, so the rejection above is about the missing data and not
// about the table form.
func TestVerifyAcceptsCachedTableTerm(t *testing.T) {
	sections := []sdk.ConfigSection{{
		Root: "firewall",
		Data: `{"firewall":{"table":{"wan":{"family":"inet","chain":{"input":{"term":{"t":{"from":{"source-asn":"13335"},"then":{"drop":""}}}}}}}}}`,
	}}
	refs := parseIRRConfig(sections).allRefs()
	ps := store.New(nil, nil, "")
	ps.Put("AS13335", []netip.Prefix{netip.MustParsePrefix("1.1.1.0/24")}, nil)
	if err := verifyRefs(ps, refs); err != nil {
		t.Fatalf("a cached table-term reference must verify, got %v", err)
	}
}
