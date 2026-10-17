package server

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// VALIDATES: Stage 1 accepts the known resolver caches and refuses an unknown
// or repeated one, naming the plugin and the fix.
// PREVENTS: a misspelled cache leaving a plugin out of every clear in silence.
func TestValidateResolverCacheDecls(t *testing.T) {
	if err := validateResolverCacheDecls("p", []string{rpc.ResolverCacheIRR, rpc.ResolverCachePeeringDB, rpc.ResolverCacheCymru}); err != nil {
		t.Fatalf("known caches refused: %v", err)
	}
	if err := validateResolverCacheDecls("p", nil); err != nil {
		t.Fatalf("no declaration refused: %v", err)
	}
	cases := map[string][]string{
		"which ze does not have": {"irrr"},
		"twice":                  {rpc.ResolverCacheIRR, rpc.ResolverCacheIRR},
	}
	for want, caches := range cases {
		err := validateResolverCacheDecls("bgp-filter-irr", caches)
		if err == nil {
			t.Errorf("%v accepted, want a refusal", caches)
			continue
		}
		if !strings.Contains(err.Error(), want) || !strings.Contains(err.Error(), "bgp-filter-irr") {
			t.Errorf("%v: error %q does not say %q and name the plugin", caches, err, want)
		}
	}
	err := validateResolverCacheDecls("p", []string{"irrr"})
	if err == nil || !strings.Contains(err.Error(), "declare one of "+strings.Join(rpc.ResolverCacheNames(), ", ")) {
		t.Errorf("unknown cache: error %v does not list every cache ze has", err)
	}
}
