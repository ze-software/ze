// Design: docs/architecture/api/process-protocol.md -- owned interface devices
// Related: ../rpc/owned_devices.go -- the public value contract
// Related: sdk_firewall_tables.go -- the firewall table contribution

package sdk

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// OwnedBridgesReplace replaces the caller's complete set of owned bridges. The
// interface component accepts it only from a plugin compiled into the ze
// binary that granted itself device names from its init().
func (p *Plugin) OwnedBridgesReplace(ctx context.Context, input *rpc.OwnedBridgesReplaceInput) (*rpc.OwnedBridgesReplaceOutput, error) {
	return engineOperationResult[rpc.OwnedBridgesReplaceOutput](p, ctx, rpc.MethodOwnedBridgesReplace, input)
}

// OwnedLinkCreate creates one link from an owned bridge into the network
// namespace of a process, and answers the host end's name.
func (p *Plugin) OwnedLinkCreate(ctx context.Context, input *rpc.OwnedLinkCreateInput) (*rpc.OwnedLinkCreateOutput, error) {
	return engineOperationResult[rpc.OwnedLinkCreateOutput](p, ctx, rpc.MethodOwnedLinkCreate, input)
}

// OwnedLinkRemove removes every link the caller recorded under one key.
func (p *Plugin) OwnedLinkRemove(ctx context.Context, input *rpc.OwnedLinkRemoveInput) (*rpc.OwnedLinkRemoveOutput, error) {
	return engineOperationResult[rpc.OwnedLinkRemoveOutput](p, ctx, rpc.MethodOwnedLinkRemove, input)
}

// engineOperationResult calls one owner-registered engine operation and
// decodes its result. DirectBridge has no frame decoder, so the reply bound is
// kept equal to the one an RPC reply frame allows; the budget function carries
// the route view's name because that operation needed it first.
func engineOperationResult[T any](p *Plugin, ctx context.Context, method string, input any) (*T, error) {
	result, err := p.callEngineWithResult(ctx, method, input)
	if err != nil {
		return nil, err
	}
	if len(result) > rpc.RouteViewResultBudget() {
		return nil, fmt.Errorf("%s result exceeds maximum message size", method)
	}
	var out T
	if err := json.Unmarshal(result, &out); err != nil {
		return nil, fmt.Errorf("decode %s result: %w", method, err)
	}
	return &out, nil
}
