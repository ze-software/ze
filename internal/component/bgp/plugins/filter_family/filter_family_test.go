package filter_family

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// TestConfigureToHandlerEndToEnd validates the package wiring: parse instances,
// publish them via instancesByName, and have the runtime handler resolve a
// reference end to end.
//
// VALIDATES: config -> instancesByName -> handleFilterUpdate runtime path.
// PREVENTS: a parsed instance not being visible to the dispatch handler.
func TestConfigureToHandlerEndToEnd(t *testing.T) {
	cfg := map[string]any{"policy": map[string]any{"family-filter": map[string]any{
		"NoFlow": map[string]any{"family": "ipv4/flow", "action": "remove"},
	}}}
	instances, err := parseFamilyFilters(cfg)
	require.NoError(t, err)
	instancesByName.Store(&instances)
	t.Cleanup(func() { instancesByName.Store(nil) })

	out := handleFilterUpdate(&sdk.FilterUpdateInput{
		Filter:    "NoFlow",
		Direction: "import",
		Raw:       buildUpdate(mpFlowReachAttr(), nil),
	})
	assert.Equal(t, sdk.FilterReject, out.Action)
}
