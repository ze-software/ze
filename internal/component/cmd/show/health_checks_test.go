package show

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ze-software/ze/internal/core/health"

	// Health rows are registered by their OWNING components at init
	// (ai/rules/plugin-self-containment.md). Linking the owners here mirrors
	// the production wiring in internal/component/plugin/all. The probe
	// behavior tests moved with the probes to
	// internal/core/report/health_probe_test.go.
	_ "github.com/ze-software/ze/internal/component/bgp/plugin"
	_ "github.com/ze-software/ze/internal/component/iface"
	_ "github.com/ze-software/ze/internal/component/l2tp"
	_ "github.com/ze-software/ze/internal/component/plugin/process"
	_ "github.com/ze-software/ze/internal/core/report"
	_ "github.com/ze-software/ze/internal/plugins/fib/kernel"
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
