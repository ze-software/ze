// Design: docs/research/l2tpv2-ze-integration.md -- transport-specific configuration refusals.

package pppconfig

import (
	"errors"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/opererr"
)

// TestLCPProviderRefusalNamesAWorkingRepair checks the operator error contract,
// including exact integer commands for both callers of the shared parser.
func TestLCPProviderRefusalNamesAWorkingRepair(t *testing.T) {
	for _, transport := range []string{"l2tp", "pppoe"} {
		for _, tc := range []struct {
			name string
			raw  any
			fix  string
		}{
			{"block", "500", "delete " + transport + " lcp"},
			{"type", map[string]any{"max-failure": 5}, "set " + transport + " lcp max-failure 5"},
			{"integer", map[string]any{"max-terminate": "invalid"}, "set " + transport + " lcp max-terminate 2"},
			{"timer bounds", map[string]any{"restart-timer": "50"}, "set " + transport + " lcp restart-timer 3000"},
			{"timer overflow", map[string]any{"restart-timer": "9223372036854775807"}, "set " + transport + " lcp restart-timer 3000"},
			{"counter bounds", map[string]any{"max-configure": "0"}, "set " + transport + " lcp max-configure 10"},
		} {
			t.Run(transport+"/"+tc.name, func(t *testing.T) {
				_, err := ParseLCP(transport, tc.raw)
				var operatorError *opererr.Error
				if !errors.As(err, &operatorError) {
					t.Fatalf("refusal = %v, want opererr.Error", err)
				}
				if operatorError.Fix() != tc.fix {
					t.Fatalf("fix = %q, want %q", operatorError.Fix(), tc.fix)
				}
				if !strings.HasSuffix(err.Error(), "\nfix: "+tc.fix) {
					t.Fatalf("repair is not the final line: %v", err)
				}
			})
		}
	}
}

// TestLCPMissingSchemaDoesNotSuggestEditingASetting distinguishes a defective
// build from invalid operator input: changing a leaf cannot restore a module.
func TestLCPMissingSchemaDoesNotSuggestEditingASetting(t *testing.T) {
	defaults := lcpDefaults
	lcpDefaults = func() ([4]int, error) { return [4]int{}, errors.New("shared PPP LCP schema is not registered") }
	t.Cleanup(func() { lcpDefaults = defaults })
	_, err := ParseLCP("pppoe", nil)
	var operatorError *opererr.Error
	if !errors.As(err, &operatorError) {
		t.Fatalf("refusal = %v, want opererr.Error", err)
	}
	if strings.HasPrefix(operatorError.Fix(), "set ") {
		t.Fatalf("schema repair suggests changing config: %q", operatorError.Fix())
	}
	if !strings.Contains(operatorError.Fix(), "ze-ppp-lcp") {
		t.Fatalf("schema repair does not name missing module: %q", operatorError.Fix())
	}
}
