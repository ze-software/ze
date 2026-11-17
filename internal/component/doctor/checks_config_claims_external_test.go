package doctor

import (
	"strings"
	"testing"

	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

// unclaimedRoots answers the roots diags reports as delivered to nobody.
func unclaimedRoots(diags []diagnostic.Diagnostic, roots ...string) []string {
	var reported []string
	for _, root := range roots {
		for _, d := range diags {
			if d.Code == diagnostic.CodeDoctorConfigRootUnclaimed && strings.Contains(d.Message, "config under "+root+" ") {
				reported = append(reported, root)
			}
		}
	}
	return reported
}

// TestDoctorClaimsCountExternalRoots drives the claims check from its entry
// point over a config that holds a section under an external plugin's own
// root and a section nobody wants. With the plugin running, its root is
// delivered and only the other is reported; once the plugin is gone, both are.
//
// VALIDATES: AC-14 of spec-plugin-external-yang-at-startup -- no unclaimed-root
// finding for the root a running external plugin declared, and the check keeps
// its meaning for a root no running plugin wants.
// PREVENTS: the check building its population from the plugins compiled into
// ze alone, so every external plugin's section is reported as having no
// effect while the plugin is running with it.
func TestDoctorClaimsCountExternalRoots(t *testing.T) {
	tree := treeWith("acme-monitor", "nobody-claims-this")

	t.Cleanup(pluginserver.SetExternalConfigRootsForTest(map[string][]string{"acme-monitor": {"acme-monitor"}}))
	running := unclaimedRoots(checkConfigClaims(tree), "acme-monitor", "nobody-claims-this")
	if len(running) != 1 || running[0] != "nobody-claims-this" {
		t.Errorf("with the plugin running, want only nobody-claims-this reported, got %v", running)
	}

	restore := pluginserver.SetExternalConfigRootsForTest(nil)
	defer restore()
	ended := unclaimedRoots(checkConfigClaims(tree), "acme-monitor", "nobody-claims-this")
	if len(ended) != 2 {
		t.Errorf("with no external plugin running, want both roots reported, got %v", ended)
	}
}
