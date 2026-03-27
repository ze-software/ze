// Design: docs/architecture/fleet-config.md — managed client connection lifecycle
// Related: handler.go — processes config responses from hub
// Related: reconnect.go — backoff for connection retries
// Related: heartbeat.go — liveness detection

package managed

import (
	"context"
	"encoding/json"
	"fmt"

	"codeberg.org/thomas-mangin/ze/pkg/fleet"
	"codeberg.org/thomas-mangin/ze/pkg/plugin/rpc"
)

// FetchConfig sends a config-fetch RPC to the hub and returns the response.
// version is the client's current config hash (empty on first boot).
// Uses MuxConn.CallRPC for request/response correlation.
func FetchConfig(ctx context.Context, mc *rpc.MuxConn, version string) (fleet.ConfigFetchResponse, error) {
	req := fleet.ConfigFetchRequest{Version: version}

	result, err := mc.CallRPC(ctx, fleet.VerbConfigFetch, req)
	if err != nil {
		return fleet.ConfigFetchResponse{}, fmt.Errorf("config-fetch RPC: %w", err)
	}

	var resp fleet.ConfigFetchResponse
	if err := json.Unmarshal(result, &resp); err != nil {
		return fleet.ConfigFetchResponse{}, fmt.Errorf("unmarshal config-fetch response: %w", err)
	}

	return resp, nil
}

// SendConfigAck sends a config-ack RPC to the hub confirming receipt.
func SendConfigAck(ctx context.Context, mc *rpc.MuxConn, ack fleet.ConfigAck) error {
	_, err := mc.CallRPC(ctx, fleet.VerbConfigAck, ack)
	if err != nil {
		return fmt.Errorf("config-ack RPC: %w", err)
	}
	return nil
}

// SendPing sends a ping RPC to the hub for liveness.
func SendPing(ctx context.Context, mc *rpc.MuxConn) error {
	_, err := mc.CallRPC(ctx, fleet.VerbPing, struct{}{})
	if err != nil {
		return fmt.Errorf("ping RPC: %w", err)
	}
	return nil
}
