// VALIDATES: the show traffic-usage command surface is declared inside this
// plugin's own YANG, not a central schema.
// PREVENTS: the command outliving the plugin -- removing internal/plugins/
// trafficusage/ must remove the command node, its handler, and this schema
// together (ai/rules/plugin-self-containment.md).

package yang

import (
	"strings"
	"testing"
)

func TestTrafficUsageCmdSchemaOwnsShowTrafficUsage(t *testing.T) {
	for _, want := range []string{
		`ze:command "ze-show:traffic-usage"`,
		"container traffic-usage",
		"container show",
	} {
		if !strings.Contains(ZeTrafficUsageCmdYANG, want) {
			t.Errorf("ze-traffic-usage-cmd.yang must declare %q so removing the traffic-usage plugin removes the show traffic-usage command surface", want)
		}
	}
}

func TestTrafficUsageConfSchemaOwnsConfigRoot(t *testing.T) {
	for _, want := range []string{
		"container traffic-usage",
		"leaf enabled",
		"container interfaces",
		"list interface",
		"leaf track-ip",
	} {
		if !strings.Contains(ZeTrafficUsageConfYANG, want) {
			t.Errorf("ze-traffic-usage-conf.yang must declare %q so the config surface lives with the plugin", want)
		}
	}
}
