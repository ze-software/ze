// Design: docs/architecture/meta/role.md -- learned state belongs to the selected connection.
// Related: role.go -- applyValidateOpen; otc.go -- selected relationship consumers.
package role

import (
	"encoding/json"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestProvisionalOpenPreservesSelectedRole checks the real role validator and
// OTC export decision after a competing OPEN, including rejection and absence.
// The JSON is the external plugin boundary, so the pre-fix DTO still compiles.
func TestProvisionalOpenPreservesSelectedRole(t *testing.T) {
	for _, candidate := range []struct {
		name   string
		caps   string
		accept bool
	}{
		{"mismatched-provider", `[{"code":9,"hex":"00"}]`, false},
		{"roleless", `[]`, true},
		{"valid-customer", `[{"code":9,"hex":"03"}]`, true},
	} {
		t.Run(candidate.name, func(t *testing.T) {
			clearFilterState(t)
			configs := map[string]*peerRoleConfig{
				"192.0.2.1": {role: roleProvider},
				"192.0.2.2": {role: roleProvider, export: []string{"customer"}, resolvedExport: resolveExport(roleProvider, []string{"customer"})},
			}
			setFilterState(configs, nil)
			if out := applyValidateOpen(configs, nil, openWithRole("192.0.2.1", 3)); !out.Accept {
				t.Fatal("valid incumbent role was refused")
			}
			source := filterapi.PeerFilterInfo{Address: netip.MustParseAddr("192.0.2.2")}
			target := filterapi.PeerFilterInfo{Address: netip.MustParseAddr("192.0.2.1")}
			payload := buildTestPayload(buildTestAttrs(0), nil)
			if !OTCEgressFilter(source, target, payload, nil, nil) {
				t.Fatal("incumbent Customer was not usable before competing OPEN")
			}
			var input rpc.ValidateOpenInput
			wire := `{"peer":"192.0.2.1","provisional":true,"local":{},"remote":{"capabilities":` + candidate.caps + `}}`
			if err := json.Unmarshal([]byte(wire), &input); err != nil {
				t.Fatal(err)
			}
			out := applyValidateOpen(configs, nil, &input)
			if out.Accept != candidate.accept {
				t.Fatalf("provisional validation accept=%v, want %v", out.Accept, candidate.accept)
			}
			if !out.Accept {
				if out.NotifyCode != 2 {
					t.Fatalf("mismatched OPEN returned %d/%d, want 2/11", out.NotifyCode, out.NotifySubcode)
				}
				if out.NotifySubcode != 11 {
					t.Fatalf("mismatched OPEN returned %d/%d, want 2/11", out.NotifyCode, out.NotifySubcode)
				}
			}
			_, learned := getFilterConfig("192.0.2.1", "", "")
			if learned != roleCustomer {
				t.Fatalf("unselected OPEN replaced incumbent Customer with %q", learned)
			}
			if !OTCEgressFilter(source, target, payload, nil, nil) {
				t.Fatal("competing OPEN changed the incumbent's useful export decision")
			}
		})
	}
}
