// Design: docs/contributing/rfc-conformance-gates.md -- test filename scope.
package rfc

import (
	"strings"
	"testing"
)

// TestNamingScopeExcludesCatalogUnits keeps development-tool naming outside
// the naming gate even when a known catalog's Go test uses the unit carrier.
// An ordinary misnamed unit must still produce its naming finding.
func TestNamingScopeExcludesCatalogUnits(t *testing.T) {
	catalogUnit := "internal/le/interoplab/bgp/check_local_pref_negative_test.go"
	ordinaryUnit := "internal/sample/preference_test.go"
	carriers := carriersFor(nil, nil)
	carrier, held := CarrierFor(catalogUnit, carriers)
	if !held {
		t.Fatal("known catalog unit has no carrier; naming scope control is vacuous")
	}
	if carrier.Kind != kindUnit {
		t.Fatalf("known catalog test has kind %q, want unit", carrier.Kind)
	}
	root := namingTree(t, map[string]string{
		catalogUnit:  namingBody,
		ordinaryUnit: namingBody,
	})
	tags := []Tag{
		{RID: "RFC7311-4-1", File: catalogUnit},
		{RID: "RFC7311-4-1", File: ordinaryUnit},
	}
	findings, err := checkTestFileNames(root, carriers, tags, namingRequirements(), namingStems())
	if err != nil {
		t.Fatalf("checkTestFileNames: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("naming findings=%v, want only the ordinary misnamed unit", findings)
	}
	if !strings.HasPrefix(findings[0], ordinaryUnit+":") {
		t.Errorf("naming finding=%q, want the ordinary misnamed unit", findings[0])
	}
	if !strings.Contains(findings[0], "internal/sample/rfc7311_preference_test.go") {
		t.Errorf("naming finding=%q, want the ordinary unit's RFC naming repair", findings[0])
	}
}
