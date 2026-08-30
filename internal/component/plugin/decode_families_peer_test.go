// Related: registration.go — DecodeFamiliesForPlugins, GetDecodeFamilies
//
// VALIDATES: a peer is offered the decode families of ITS attached processes,
// and a peer with none is offered nothing.
// PREVENTS: a peer whose config names no family block taking every loaded
// plugin's families into its OPEN. That offered link-state and the rest to an
// ordinary peer, and left the set non-empty, so the implicit ipv4/unicast
// default could never fire and the most ordinary configuration in BGP
// negotiated nothing an operator wanted.
package plugin

import (
	"maps"
	"testing"
)

// registryWithFamilies returns a registry whose family table maps each family to
// the plugin that decodes it.
func registryWithFamilies(t *testing.T, families map[string]string) *PluginRegistry {
	t.Helper()

	r := &PluginRegistry{families: map[string]string{}}
	maps.Copy(r.families, families)

	return r
}

// TestAPeerIsOfferedOnlyItsOwnProcessesFamilies is the defect. The whole-process
// answer and the per-peer answer differ, and only the second is right for an
// OPEN.
func TestAPeerIsOfferedOnlyItsOwnProcessesFamilies(t *testing.T) {
	r := registryWithFamilies(t, map[string]string{
		"ipv6/unicast":  "watcher",
		"bgp-ls/bgp-ls": "topology",
		"l2vpn/evpn":    "fabric",
	})

	got := r.DecodeFamiliesForPlugins([]string{"watcher"})

	if len(got) != 1 || got[0] != "ipv6/unicast" {
		t.Fatalf("a peer attaching only watcher was offered %v; it must be offered "+
			"what watcher decodes and nothing else", got)
	}

	if all := r.GetDecodeFamilies(); len(all) != 3 {
		t.Fatalf("the whole-process answer changed: %v. It is still the right answer "+
			"for startup validation, and the wrong one for a peer", all)
	}
}

// TestAPeerWithNoProcessIsOfferedNothing is what lets the implicit ipv4/unicast
// default fire. capability.Negotiate supplies it only when the local family set
// is EMPTY, so a non-empty set is what suppressed it.
func TestAPeerWithNoProcessIsOfferedNothing(t *testing.T) {
	r := registryWithFamilies(t, map[string]string{
		"ipv6/unicast":  "watcher",
		"bgp-ls/bgp-ls": "topology",
	})

	if got := r.DecodeFamiliesForPlugins(nil); got != nil {
		t.Fatalf("a peer attaching no process was offered %v; the empty answer is "+
			"what lets the implicit ipv4/unicast default fire", got)
	}
	if got := r.DecodeFamiliesForPlugins([]string{}); got != nil {
		t.Fatalf("an empty name list answered %v, want nil", got)
	}
}

// TestSeveralProcessesContributeEachTheirOwn keeps the union working, so the fix
// cannot be read as "one process only".
func TestSeveralProcessesContributeEachTheirOwn(t *testing.T) {
	r := registryWithFamilies(t, map[string]string{
		"ipv6/unicast":  "watcher",
		"bgp-ls/bgp-ls": "topology",
		"l2vpn/evpn":    "fabric",
	})

	got := r.DecodeFamiliesForPlugins([]string{"watcher", "fabric"})

	want := []string{"ipv6/unicast", "l2vpn/evpn"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v (sorted, for a deterministic OPEN)", got, want)
		}
	}
}

// TestAnUnknownProcessNameContributesNothing keeps the lookup honest: a name
// that decodes nothing must not widen the set.
func TestAnUnknownProcessNameContributesNothing(t *testing.T) {
	r := registryWithFamilies(t, map[string]string{"ipv6/unicast": "watcher"})

	if got := r.DecodeFamiliesForPlugins([]string{"stranger"}); got != nil {
		t.Fatalf("a process that decodes no family contributed %v", got)
	}
}
