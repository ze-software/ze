package show

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"codeberg.org/thomas-mangin/ze/internal/core/health"

	// Health rows are registered by their OWNING components at init
	// (ai/rules/plugin-self-containment.md). Linking the owners here mirrors
	// the production wiring in internal/component/plugin/all. The probe
	// behavior tests moved with the probes to
	// internal/core/report/health_probe_test.go.
	_ "codeberg.org/thomas-mangin/ze/internal/component/bgp/plugin"
	_ "codeberg.org/thomas-mangin/ze/internal/component/iface"
	_ "codeberg.org/thomas-mangin/ze/internal/component/l2tp"
	_ "codeberg.org/thomas-mangin/ze/internal/component/plugin/process"
	_ "codeberg.org/thomas-mangin/ze/internal/core/report"
	_ "codeberg.org/thomas-mangin/ze/internal/plugins/fib/kernel"
)

// VALIDATES: AC-20 -- every component health row that `show health` reports
// is registered by its owning component, not by this package.
// PREVENTS: deleting a component leaving a dangling health row, or a health
// row silently disappearing because no owner registers it.
func TestHealthRowsRegisteredByOwners(t *testing.T) {
	rpt := health.Check()
	names := make(map[string]bool, len(rpt.Components))
	for i := range rpt.Components {
		names[rpt.Components[i].Name] = true
	}
	for _, want := range []string{"bgp", "fib", "iface", "l2tp", "plugins", "report-bus"} {
		assert.True(t, names[want], "health row %q must be registered by its owner", want)
	}
}
