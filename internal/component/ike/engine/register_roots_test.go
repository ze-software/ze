package engine

import (
	"slices"
	"testing"

	"github.com/ze-software/ze/internal/component/plugin/registry"
)

// TestPKIAloneDoesNotSelectTheEngine proves which config roots start the ike plugin.
// Method: the registry's one answer to "which plugins does this config select"
// (registry.PluginsForConfigPaths, read by boot auto-load and by the validate and
// commit doors) is asked for a config holding only `pki`, then one holding `vpn`.
//
// A config with PKI and no IPsec has nothing for the engine to protect. An engine
// started for it applied an empty configuration, installed the SPD catch-all, and
// refused the daemon's start wherever the kernel refused that entry (no CAP_NET_ADMIN,
// no XFRM). The engine still RECEIVES `pki` beside `vpn` (ConfigSectionRoots), so its
// verifier resolves the certificates a peer cites.
// MUTATION: put configRootPKI back in ConfigRoots (register.go) and the first check fails.
func TestPKIAloneDoesNotSelectTheEngine(t *testing.T) {
	if slices.Contains(registry.PluginsForConfigPaths([]string{configRootPKI}), "ike") {
		t.Fatal("a config holding only `pki` selects the ike plugin, want it left unloaded")
	}
	if !slices.Contains(registry.PluginsForConfigPaths([]string{configRootVPN}), "ike") {
		t.Fatal("a config holding `vpn` does not select the ike plugin")
	}

	reg := registry.Lookup("ike")
	if reg == nil {
		t.Fatal("the ike plugin is not registered")
	}
	roots := reg.ConfigSectionRoots()
	for _, want := range []string{configRootVPN, configRootPKI} {
		if !slices.Contains(roots, want) {
			t.Errorf("the ike plugin no longer receives `%s` (ConfigSectionRoots %v)", want, roots)
		}
	}
}
