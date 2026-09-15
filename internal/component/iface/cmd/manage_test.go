package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"

	// The iface command tree is what binds `vid`, and DefaultLoader sees it
	// only once this package's init has registered the module.
	_ "github.com/ze-software/ze/internal/component/iface/yang"
)

// TestUnitVIDBoundIsTheModels drives `create interface <name> unit <vid>` and
// `delete interface <name> unit <vid>` through the real dispatcher, with the
// real handlers registered under the argument definitions the command tree
// declares.
//
// VALIDATES: the VLAN ID bound, 1 to 4094, is enforced by the model on the
// dispatch path. A value outside it is refused with the dispatcher's own range
// message before the handler runs, and a value inside it reaches the handler,
// which then fails on the one thing this test host lacks, a loaded backend.
// PREVENTS: a second copy of the bound in handleUnitAdd or handleUnitDel. The
// handlers used to compare the parsed value against 1 and 4094 themselves,
// which was dead once the `vid` leaf declared its range, and a dead check is
// the copy that drifts when the model changes (ai/rules/principles.md).
func TestUnitVIDBoundIsTheModels(t *testing.T) {
	loader, err := yang.DefaultLoader()
	require.NoError(t, err)
	argDefs := yang.PathToArgDefs(loader)

	cases := []struct {
		path    string
		handler pluginserver.Handler
		input   string
		// refused is the dispatcher's own message for a value outside the
		// model's range; empty means the handler must be reached.
		refused string
	}{
		{
			path:    "delete interface name unit",
			handler: handleUnitDel,
			input:   "delete interface name eth0 unit 4095",
			refused: "value 4095 out of range 1..4094",
		},
		{
			path:    "delete interface name unit",
			handler: handleUnitDel,
			input:   "delete interface name eth0 unit 0",
			refused: "value 0 out of range 1..4094",
		},
		{
			path:    "delete interface name unit",
			handler: handleUnitDel,
			input:   "delete interface name eth0 unit 4094",
		},
		{
			path:    "create interface unit",
			handler: handleUnitAdd,
			input:   "create interface eth0 unit 4095",
			refused: "value 4095 out of range 1..4094",
		},
		{
			path:    "create interface unit",
			handler: handleUnitAdd,
			input:   "create interface eth0 unit 1",
		},
	}

	for _, c := range cases {
		t.Run(c.input, func(t *testing.T) {
			defs := argDefs[c.path]
			require.NotEmpty(t, defs, "the model declares no argument for %q", c.path)

			d := pluginserver.NewDispatcher()
			d.RegisterWithOptions(c.path, c.handler, "under test", pluginserver.RegisterOptions{ArgDefs: defs})

			resp, dispatchErr := d.Dispatch(&pluginserver.CommandContext{}, c.input)
			require.NotNil(t, resp)
			assert.Equal(t, plugin.StatusError, resp.Status)
			if c.refused != "" {
				require.Error(t, dispatchErr)
				assert.Contains(t, resp.Error, c.refused, "the model's range refuses the value")
				return
			}
			// The value passed the model, so the handler ran and reached the
			// backend seam. No backend is loaded in a unit test, and that is
			// the error it answers with: not a VLAN ID complaint.
			require.NoError(t, dispatchErr)
			assert.Equal(t, "iface: no backend loaded", resp.Error)
		})
	}
}
