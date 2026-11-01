package filter_community_match

import (
	"errors"
	"testing"

	"github.com/ze-software/ze/internal/component/policy"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// definitionsSection builds a bgp delivery defining one community-match entry per
// name.
func definitionsSection(names ...string) []sdk.ConfigSection {
	data := `{"policy":{"community-match":{`
	for i, name := range names {
		if i > 0 {
			data += ","
		}
		data += `"` + name + `":{"entry":[{"community":"65001:100","action":"reject"}]}`
	}
	data += `}}}`
	return []sdk.ConfigSection{{Root: policy.RootName, Data: data}}
}

// TestConfigTxRefusesAMalformedReload proves what the config transaction of
// bgp-filter-community-match still owns. The definitions are no state the plugin keeps: a
// BGP chain judges them through a match instance compiled from the policy
// tree, so the transaction's job is the verify. Method: a boot configure, a
// verified edit applied and rolled back, then a candidate that is not JSON,
// whose verify is refused and whose apply is then refused as unverified.
// VALIDATES: spec-plugin-reload-applies-or-refuses AC-1 to AC-4 for
// bgp-filter-community-match, as far as the plugin holds them.
// PREVENTS: a malformed reload candidate reaching apply.
func TestConfigTxRefusesAMalformedReload(t *testing.T) {
	tx := newCommunityMatchConfigTx()
	if err := tx.Configure(definitionsSection("A")); err != nil {
		t.Fatalf("configure: %v", err)
	}
	if err := tx.Verify(definitionsSection("A", "B")); err != nil {
		t.Fatalf("verify: %v", err)
	}
	if err := tx.Apply(); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatalf("rollback: %v", err)
	}

	if err := tx.Verify([]sdk.ConfigSection{{Root: policy.RootName, Data: "{not json"}}); err == nil {
		t.Fatal("verify of invalid JSON: err nil, want a refusal")
	}
	if err := tx.Apply(); !errors.Is(err, sdk.ErrConfigApplyUnverified) {
		t.Fatalf("apply after a refused verify: %v, want ErrConfigApplyUnverified", err)
	}
}
