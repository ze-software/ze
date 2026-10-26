//go:build ze_bgp && ze_ospf && ze_isis && ze_ldp && ze_rsvpte

// Design: docs/architecture/plugin/plugin-system.md -- config-root auto-load
//
// RFC 7084 Section 4.2 WPD-7: by default an IPv6 CE router starts no dynamic
// routing protocol on its WAN interface.
//
// VALIDATES: a configuration that holds only the interface root, where the
// WAN unit and its DHCPv6 client live, auto-loads none of the plugins that a
// routing protocol config root loads, while each such root loads at least one.
// PREVENTS: a routing protocol plugin started by a configuration that never
// named its protocol.

package server

import (
	"testing"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/registry"
)

// TestRFC7084InterfaceOnlyConfigStartsNoRoutingProtocol resolves the
// auto-load set of an interface-only configuration.
func TestRFC7084InterfaceOnlyConfigStartsNoRoutingProtocol(t *testing.T) {
	// RFC requirement: RFC7084-4.2-24 negative -- a configuration holding only the interface root auto-loads none of the plugins the bgp, ospf, isis, ldp and rsvp-te config roots load.
	routing := make(map[string]string)
	for _, root := range []string{"bgp", "ospf", "isis", "ldp", "rsvp-te"} {
		names := registry.PluginsForConfigPaths([]string{root})
		if len(names) == 0 {
			t.Fatalf("no plugin claims the %s config root, so its absence below would prove nothing", root)
		}
		for _, name := range names {
			routing[name] = root
		}
	}

	s := &Server{
		config: &ServerConfig{
			ConfigNeedHolds: holdsForEmptyConfig,
			ConfiguredPaths: []string{"interface"},
		},
		registry:      plugin.NewPluginRegistry(),
		loadedPlugins: make(map[string]bool),
	}
	got, err := s.getConfigPathPlugins()
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range got {
		if root, ok := routing[p.Name]; ok {
			t.Errorf("an interface-only configuration auto-loads %s, a plugin of the %s routing root", p.Name, root)
		}
	}
}
