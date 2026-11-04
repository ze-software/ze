// Design: docs/guide/bgp-role.md -- source-keyed OTC export insertion
package bgpconfig

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/plugin/registry"
)

// TestOTCExportInsertionReachesRolelessDestination proves a source's role
// installs the required export procedure on a destination with no role, while
// ingress stays per-peer. Method: resolve real config with the compiled role
// registration, including an all-roleless control and a forbidden deactivation.
func TestOTCExportInsertionReachesRolelessDestination(t *testing.T) {
	registry.SetRuntimeLoadedPlugins(func() []string { return []string{"bgp-role"} })
	t.Cleanup(func() { registry.SetRuntimeLoadedPlugins(nil) })
	const text = `bgp {
    router-id 192.0.2.254;
    session { asn { local 65000; } }
    peer source {
        connection { remote { ip 192.0.2.1; } local { ip auto; } }
        session { asn { remote 65001; } }
        SOURCE
    }
    peer destination {
        connection { remote { ip 192.0.2.2; } local { ip auto; } }
        session { asn { remote 65002; } }
        DESTINATION
    }
}`
	for _, tc := range []struct {
		name        string
		source      string
		destination string
		wantExport  bool
		wantRefusal bool
	}{
		{name: "source-role", source: "role { import provider; export [ customer ]; }", wantExport: true},
		{name: "no-roles"},
		{name: "required-on-roleless-destination", source: "role { import provider; }", destination: "filter { export { filters [ inactive:bgp-role:otc-egress ]; final accept; } }", wantRefusal: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input := strings.ReplaceAll(strings.ReplaceAll(text, "SOURCE", tc.source), "DESTINATION", tc.destination)
			tree, err := config.ParseTreeWithYANG(input, nil)
			if err != nil {
				t.Fatal(err)
			}
			peers, err := PeersFromConfigTree(tree)
			if tc.wantRefusal {
				if err == nil || !strings.Contains(err.Error(), "otc-egress") || !strings.Contains(err.Error(), "The operator MUST NOT") {
					t.Fatalf("required egress deactivation: got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if len(peers) != 2 {
				t.Fatalf("loaded %d peers, want 2", len(peers))
			}
			for _, peer := range peers {
				exports := refNames(peer.ExportPolicy.Filters())
				gotExport := false
				for _, ref := range exports {
					gotExport = gotExport || ref == "bgp-role:otc-egress"
				}
				if gotExport != tc.wantExport {
					t.Errorf("%s export list %v: OTC presence = %v, want %v", peer.Name, exports, gotExport, tc.wantExport)
				}
				if peer.Name == "destination" {
					for _, ref := range peer.ImportPolicy.Filters() {
						if ref.Name == "bgp-role:otc-ingress" {
							t.Error("roleless destination has an OTC ingress member")
						}
					}
				}
			}
		})
	}
}
