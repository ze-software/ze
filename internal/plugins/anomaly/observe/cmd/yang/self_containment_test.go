// VALIDATES: the `show anomaly observe` command surface is declared inside the
// anomaly-observe plugin's own YANG, not in a central schema.
// PREVENTS: the command outliving the plugin -- removing
// internal/plugins/anomaly/observe/ must remove the command node, its handler and
// this schema together (ai/rules/plugins.md).

package yang

import (
	"strings"
	"testing"
)

func TestAnomalyObserveCmdSchemaOwnsShowObserve(t *testing.T) {
	for _, want := range []string{
		`ze:command "ze-show:anomaly-observe"`,
		"container show",
		"container anomaly",
		"container observe",
	} {
		if !strings.Contains(ZeAnomalyObserveCmdYANG, want) {
			t.Errorf("ze-anomaly-observe-cmd.yang must declare %q so removing the anomaly-observe plugin removes the show anomaly observe command surface", want)
		}
	}
}
