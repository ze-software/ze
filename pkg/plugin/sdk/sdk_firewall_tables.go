// Design: docs/architecture/api/process-protocol.md -- firewall table contribution
// Related: ../rpc/firewall_tables.go -- the public value contract
// Related: sdk_owned_devices.go -- engineOperationResult

package sdk

import (
	"context"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// FirewallTablesReplace replaces the caller's complete set of firewall tables.
// The firewall component accepts only kernel table names the calling plugin
// reserved from its init().
func (p *Plugin) FirewallTablesReplace(ctx context.Context, input *rpc.FirewallTablesReplaceInput) (*rpc.FirewallTablesReplaceOutput, error) {
	return engineOperationResult[rpc.FirewallTablesReplaceOutput](p, ctx, rpc.MethodFirewallTablesReplace, input)
}
