package yang

import (
	"strings"
	"testing"
)

// TestRibPoolStatsSchemaOwnsPoolStats asserts the BGP RIB command cluster owns
// the "metrics pool" command node (ze-bgp:pool-stats). pool-stats reads the BGP
// RIB attribute pools (bgp/plugins/rib/pool); the central metrics verb keeps
// only the generic Prometheus-registry commands. This is the owner half of the
// self-containment invariant. See ai/rules/plugins.md.
func TestRibPoolStatsSchemaOwnsPoolStats(t *testing.T) {
	if !strings.Contains(ZeRibPoolStatsCmdYANG, `ze:command "ze-bgp:pool-stats"`) {
		t.Error("ze-rib-poolstats-cmd.yang must declare the ze-bgp:pool-stats command")
	}
	if !strings.Contains(ZeRibPoolStatsCmdYANG, "container metrics") ||
		!strings.Contains(ZeRibPoolStatsCmdYANG, "container pool") {
		t.Error("ze-rib-poolstats-cmd.yang must container-merge metrics > pool")
	}
}
