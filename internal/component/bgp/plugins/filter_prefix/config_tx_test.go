package filter_prefix

import (
	"errors"
	"testing"

	"github.com/ze-software/ze/internal/component/policy"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// prefixSection builds a bgp delivery defining prefix-list L with one entry
// for 10.0.0.0/8 and the given action.
func prefixSection(entryAction string) []sdk.ConfigSection {
	data := `{"policy":{"prefix-list":{"L":{"entry":{"10.0.0.0/8":{"action":"` +
		entryAction + `"}}}}}}`
	return []sdk.ConfigSection{{Root: policy.RootName, Data: data}}
}

// TestPrefixConfigTxRefusesAMalformedReload proves what the config
// transaction of bgp-filter-prefix still owns. The prefix lists are no state
// the plugin keeps: a BGP chain judges them through a match instance compiled
// from the policy tree, so the transaction's job is the verify. Method: a boot
// configure, a verified edit applied and rolled back, then an entry whose
// action the parser does not know, whose verify is refused and whose apply is
// then refused as unverified.
// VALIDATES: spec-plugin-reload-applies-or-refuses AC-1, AC-3, AC-4 for
// bgp-filter-prefix, as far as the plugin holds them.
// PREVENTS: a candidate the parser refuses reaching apply.
func TestPrefixConfigTxRefusesAMalformedReload(t *testing.T) {
	tx := newPrefixConfigTx()
	if err := tx.Configure(prefixSection("accept")); err != nil {
		t.Fatalf("configure: %v", err)
	}
	if err := tx.Verify(prefixSection("reject")); err != nil {
		t.Fatalf("verify: %v", err)
	}
	if err := tx.Apply(); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatalf("rollback: %v", err)
	}

	if err := tx.Verify(prefixSection("maybe")); err == nil {
		t.Fatal("verify of an unknown action: err nil, want a parse refusal")
	}
	if err := tx.Apply(); !errors.Is(err, sdk.ErrConfigApplyUnverified) {
		t.Fatalf("apply after a refused verify: %v, want ErrConfigApplyUnverified", err)
	}
}

// TestPrefixConfigTxRemovalAndAbsentRoot proves AC-2 for bgp-filter-prefix as
// far as the plugin holds it: a reload whose policy section defines no
// prefix-list, and one that carries no policy section at all, both verify and
// apply.
// PREVENTS: a list removal refused by the plugin, which would hold the reload.
func TestPrefixConfigTxRemovalAndAbsentRoot(t *testing.T) {
	tx := newPrefixConfigTx()
	if err := tx.Configure(prefixSection("accept")); err != nil {
		t.Fatalf("configure: %v", err)
	}

	if err := tx.Verify([]sdk.ConfigSection{{Root: "interface", Data: `{}`}}); err != nil {
		t.Fatalf("verify without policy: %v", err)
	}
	if err := tx.Apply(); err != nil {
		t.Fatalf("apply without policy: %v", err)
	}

	if err := tx.Verify([]sdk.ConfigSection{{Root: policy.RootName, Data: `{"bgp":{}}`}}); err != nil {
		t.Fatalf("verify of a removal: %v", err)
	}
	if err := tx.Apply(); err != nil {
		t.Fatalf("apply of a removal: %v", err)
	}
}
