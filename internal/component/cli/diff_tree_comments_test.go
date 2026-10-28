package cli

import (
	"testing"

	"github.com/ze-software/ze/internal/component/config"
)

// TestTreeDiffIgnoresComments checks that the editor's tree diff reports no
// change between two configurations that differ only in comments, as
// PruneUnchanged does. Phase 3 made Serialize print comments, so a container
// that inlines on the comment-free side (`bgp router-id ...`) was compared as
// text against the block form the comment forces on the other side, and read
// as modified. Method: diff each pair both ways; every line must be unchanged.
func TestTreeDiffIgnoresComments(t *testing.T) {
	schema, err := config.YANGSchema()
	if err != nil {
		t.Fatalf("YANG schema: %v", err)
	}
	for _, tc := range []struct{ name, commented, bare string }{
		{"inline container", "bgp {\n\trouter-id 192.0.2.1 # trail\n}\n", "bgp {\n\trouter-id 192.0.2.1\n}\n"},
		{"every slot", "# head\n\nbgp { # open\n\t# lead\n\trouter-id 192.0.2.1 # trail\n\tpeer A { # entry\n\t}\n\t# closing\n} # after\n# end\n",
			"bgp {\n\trouter-id 192.0.2.1\n\tpeer A {\n\t}\n}\n"},
	} {
		for _, pair := range [][2]string{{tc.commented, tc.bare}, {tc.bare, tc.commented}} {
			lines, err := computeTreeAnnotatedDiff(pair[0], pair[1], schema)
			if err != nil {
				t.Fatalf("%s: %v", tc.name, err)
			}
			for _, line := range lines {
				if line.Marker != diffUnchanged {
					t.Errorf("%s: comment-only difference reported as %q on %q", tc.name, string(line.Marker), line.Text)
				}
			}
		}
	}
}
