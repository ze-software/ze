// Design: docs/architecture/config/yang-config-design.md -- protocol RPC publication.
// Related: internal/core/ipc/yang/ze-plugin-engine.yang -- selected output and cancellation.
// Related: internal/component/bgp/yang/ze-bgp-api.yang -- original source leases.
package docyangcontract

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestSelectedOutputRPCsPublishTheirProtocolMethods loads the registered schema
// and checks that plugins can discover the exact methods their SDK sends,
// without turning authenticated process operations into operator commands.
// MUTATION: removing any ze:method leaves that RPC unnamed and fails this test.
func TestSelectedOutputRPCsPublishTheirProtocolMethods(t *testing.T) {
	schema, err := yang.DefaultLoader()
	require.NoError(t, err)
	publication, err := yang.PublishedRPCs(schema)
	require.NoError(t, err)
	for _, test := range []struct {
		module, name, method string
	}{
		{"ze-bgp-api", "source-lease", rpc.MethodSourceLease},
		{"ze-plugin-engine", "cancel-request", rpc.MethodCancelRequest},
		{"ze-plugin-engine", "select-routes", rpc.MethodSelectRoutes},
	} {
		t.Run(test.name, func(t *testing.T) {
			found := 0
			for _, operation := range publication.Protocol {
				if operation.Module != test.module {
					continue
				}
				if operation.Name != test.name {
					continue
				}
				found++
				require.Equal(t, test.method, operation.WireMethod)
			}
			require.Equal(t, 1, found, "the protocol declares its exact method once")
			for _, operation := range publication.Unnamed {
				if operation.Module == test.module {
					require.NotEqual(t, test.name, operation.Name, "plugins need a published wire method")
				}
			}
			for _, command := range publication.Commands {
				require.NotEqual(t, test.method, command.WireMethod, "a process operation grants no operator command")
			}
		})
	}

	// A real operator command stays in the command publication. Protocol
	// classification must not provide an escape from command-handler checks.
	operator := false
	for _, command := range publication.Commands {
		if command.WireMethod == "ze-bgp:peer-list" {
			operator = true
		}
	}
	require.True(t, operator, "the peer-list operator command remains published")
	for _, operation := range publication.Protocol {
		require.NotEqual(t, "ze-bgp:peer-list", operation.WireMethod)
	}
}
