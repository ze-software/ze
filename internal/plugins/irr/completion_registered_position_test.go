// Design: docs/architecture/irr-owner.md -- registered command argument completion
package irr

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
	// The real system completion handler is loaded only when its owned
	// command/API modules participate in NewServer's resolved schema.
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	_ "github.com/ze-software/ze/internal/core/ipc/yang"
)

// TestRegisteredASSetCompletionStopsAfterValue reaches both owners through
// the real server's system command completion RPC. The keyword awaits a
// stored or referenced name, and a completed name permits no further value.
// No plugin process exists, so executing either target instead of completing
// it would fail rather than return the required successful candidate payload.
func TestRegisteredASSetCompletionStopsAfterValue(t *testing.T) {
	seedASSetState(t, useDaemonStore(t))
	server, err := pluginserver.NewServer(&pluginserver.ServerConfig{}, nil)
	require.NoError(t, err)
	t.Cleanup(server.Stop)
	require.NotNil(t, server.Dispatcher().Lookup("system command complete"), "the real system completion RPC must be loaded before either owner query")

	for _, entry := range []struct {
		wire string
		path string
	}{
		{wire: "ze-irr:update-irr-as-set", path: "update irr as-set"},
		{wire: "ze-irr:clear-irr-as-set", path: "clear irr as-set"},
	} {
		t.Run(entry.path, func(t *testing.T) {
			// Compare the actual loaded carrier with init's registration. The
			// test must not register a private copy of its completion guard.
			loaded := server.Dispatcher().Lookup(entry.path)
			require.NotNil(t, loaded)
			found := false
			for _, registration := range pluginserver.AllBuiltinRPCs() {
				if registration.WireMethod != entry.wire {
					continue
				}
				found = true
				require.NotNil(t, registration.Complete)
				require.Equal(t, reflect.ValueOf(registration.Complete).Pointer(), reflect.ValueOf(loaded.Complete).Pointer())
				require.Equal(t, reflect.ValueOf(registration.Handler).Pointer(), reflect.ValueOf(loaded.Handler).Pointer())
			}
			require.True(t, found, "the actual command owner registration is required")
			ctx := &pluginserver.CommandContext{Server: server, RequestContext: t.Context(), Sender: plugin.OperatorSender()}
			response, err := server.Dispatcher().Dispatch(ctx, `system command complete args "`+entry.path+`" as-set ""`)
			require.NoError(t, err)
			require.NotNil(t, response)
			require.Equal(t, plugin.StatusDone, response.Status)
			data, ok := response.Data.(plugin.Map)
			require.True(t, ok)
			candidates, ok := data["completions"].([]pluginserver.Completion)
			require.True(t, ok)
			got := make(map[string]string, len(candidates))
			for _, candidate := range candidates {
				got[candidate.Value] = candidate.ShortHelp
			}
			require.Equal(t, map[string]string{
				"AS-FETCHED": asSetHelpStored,
				"AS-STORED":  asSetHelpStored,
				"AS-WANTED":  asSetHelpReferenced,
			}, got)

			response, err = server.Dispatcher().Dispatch(ctx, `system command complete args "`+entry.path+`" as-set AS-STORED ""`)
			require.NoError(t, err)
			require.NotNil(t, response)
			require.Equal(t, plugin.StatusDone, response.Status)
			data, ok = response.Data.(plugin.Map)
			require.True(t, ok)
			candidates, ok = data["completions"].([]pluginserver.Completion)
			require.True(t, ok)
			require.Empty(t, candidates, "a completed AS-SET leaves no second argument")
		})
	}
}
