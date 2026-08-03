// VALIDATES: the show ddos command surface is declared inside the ddos-observe
// plugin's own YANG, not a central schema.
// PREVENTS: the command outliving the plugin -- removing internal/plugins/ddos/
// observe/ must remove the command nodes, their handlers, and this schema
// together (ai/rules/plugins.md).

package yang

import (
	"strings"
	"testing"
)

func TestDdosCmdSchemaOwnsShowDdos(t *testing.T) {
	for _, want := range []string{
		`ze:command "ze-show:ddos-status"`,
		`ze:command "ze-show:ddos-incidents"`,
		"container show",
		"container ddos",
		"container status",
		"container incidents",
	} {
		if !strings.Contains(ZeDdosCmdYANG, want) {
			t.Errorf("ze-ddos-cmd.yang must declare %q so removing the ddos-observe plugin removes the show ddos command surface", want)
		}
	}
}
