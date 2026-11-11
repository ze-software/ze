// Design: docs/architecture/testing/runner-architecture.md -- peer stdin routing.
// Related: one_off_proof_test.go -- TestUnusedPeerBlockRefused, the refusal classes.

package runner

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestNegativeOrphanPeerBlockRefusalNamesIt proves the refusal of a negative
// peer block nothing reads names that block. Method: parse a .ci whose
// `stdin=peer` block carries a reject=bgp and has no ze-peer consumer, once
// with cmd= orchestration and once as an ordinary file, and read the error.
//
// VALIDATES: validatePeerBlockRejects opens its orphan refusal with the
// block's name, `stdin=peer block line`, on both routes into it.
// PREVENTS: a refusal that says a reject=bgp reaches nothing without saying
// which of several stdin= blocks in the file is the orphan.
func TestNegativeOrphanPeerBlockRefusalNamesIt(t *testing.T) {
	const rejectBlock = "stdin=peer:terminator=EOF_PEER\n" + unusedPeerExpect + "\n" +
		"reject=bgp:conn=1:pattern=deadbeef\nEOF_PEER\n"
	const helper = "cmd=foreground:seq=2:exec=true\n"

	for _, tc := range []struct {
		name    string
		content string
	}{
		{"orchestrated negative orphan", rejectBlock + helper},
		{"ordinary negative orphan", rejectBlock},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parseUnusedPeerCI(t, NewEncodingTests(t.TempDir()), tc.content)
			require.Error(t, err, "a negative peer block nothing reads must fail the file")
			require.Contains(t, err.Error(), "reject=bgp is read by ze-peer",
				"the file must be refused as an orphan negative block")
			require.Contains(t, err.Error(), "stdin=peer block line",
				"the refusal must name the orphan block")
		})
	}
}
