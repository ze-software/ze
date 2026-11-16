// Design: docs/architecture/irr-owner.md -- command argument completion
package irr

import (
	"github.com/stretchr/testify/require"
	"testing"
)

// TestASSetCompletionStopsAfterName proves a populated store cannot leak its
// names into a second argument position which neither command accepts.
func TestASSetCompletionStopsAfterName(t *testing.T) {
	seedASSetState(t, useDaemonStore(t))
	require.NotEmpty(t, completeASSetArgument(nil, []string{leafASSet}))
	require.Empty(t, completeASSetArgument(nil, []string{"AS-STORED"}))
}
