// Design: docs/architecture/config/yang-config-design.md -- config diff annotation.

package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestComputeAnnotatedDiffFlatStatementsPreserveRemovals proves the line
// fallback cannot consume a deleted statement as a different path's new value.
// A flat statement has no indentation that proves configuration identity.
func TestComputeAnnotatedDiffFlatStatementsPreserveRemovals(t *testing.T) {
	for _, verb := range []string{"set", "nop", "inactive"} {
		t.Run(verb, func(t *testing.T) {
			removed := verb + " bgp group stable-revision description selected-baseline-only"
			previous := verb + " bgp router-id 192.0.2.41"
			current := verb + " bgp router-id 192.0.2.43"
			assert.Equal(t, []diffLine{
				{diffRemoved, removed},
				{diffRemoved, previous},
				{diffAdded, current},
			}, computeAnnotatedDiff(removed+"\n"+previous+"\n", current+"\n"))
		})
	}
}
