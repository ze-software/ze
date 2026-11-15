package filter_irr

import (
	"testing"
)

// VALIDATES: AC-15 -- the filter serves no fetch command: `update bgp irr all`,
// `update bgp irr asn` and `update bgp irr as-set` are gone from its
// declarations and its handler, because the irr plugin's `update irr ...`
// fetches for every consumer.
// PREVENTS: a second fetch path for one operation (no-layering), and a command
// declared to the catalog that the plugin no longer answers.
func TestUpdateCommandsAreGone(t *testing.T) {
	removed := []string{"update bgp irr all", "update bgp irr asn", "update bgp irr as-set"}
	for _, decl := range commandDecls() {
		for _, name := range removed {
			if decl.Name == name {
				t.Errorf("commandDecls still declares %q", name)
			}
		}
	}
	plug := &irrPlugin{byASN: map[uint32]*asnState{}}
	for _, name := range removed {
		status, _, err := plug.handleCommand(name, []string{"65001"})
		if err == nil || status != statusError {
			t.Errorf("handleCommand(%q) = %q, %v; want the unknown-command refusal", name, status, err)
		}
	}
}
