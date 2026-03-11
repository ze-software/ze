// Design: docs/architecture/config/yang-config-design.md — config editor

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"time"

	rpc "codeberg.org/thomas-mangin/ze/pkg/plugin/rpc"
)

// reloadTimeout is the maximum time to wait for a daemon reload response.
const reloadTimeout = 5 * time.Second

// NewSocketReloadNotifier creates a ReloadNotifier that triggers config reload
// via the daemon's API socket. It sends the "ze-system:daemon-reload" RPC using
// NUL-framed JSON, the same protocol the CLI uses.
//
// If the socket does not exist or the daemon is not running, the returned
// function returns an error (which cmdCommit handles gracefully).
func NewSocketReloadNotifier(socketPath string) ReloadNotifier {
	return func() error {
		ctx, cancel := context.WithTimeout(context.Background(), reloadTimeout)
		defer cancel()

		// Connect to daemon API socket
		var d net.Dialer
		conn, err := d.DialContext(ctx, "unix", socketPath)
		if err != nil {
			return fmt.Errorf("daemon not reachable: %w", err)
		}
		defer func() { _ = conn.Close() }()

		// Set deadline from context
		if deadline, ok := ctx.Deadline(); ok {
			_ = conn.SetDeadline(deadline)
		}

		// Send reload request
		req := rpc.Request{Method: "ze-system:daemon-reload"}
		reqBytes, err := json.Marshal(req)
		if err != nil {
			return fmt.Errorf("marshal reload request: %w", err)
		}

		writer := rpc.NewFrameWriter(conn)
		if err := writer.Write(reqBytes); err != nil {
			return fmt.Errorf("send reload request: %w", err)
		}

		// Read response
		reader := rpc.NewFrameReader(conn)
		respBytes, err := reader.Read()
		if err != nil {
			return fmt.Errorf("read reload response: %w", err)
		}

		// Check for error response
		var errResp struct {
			Error  string          `json:"error"`
			Params json.RawMessage `json:"params,omitempty"`
		}
		if err := json.Unmarshal(respBytes, &errResp); err == nil && errResp.Error != "" {
			if msg := rpc.ExtractMessage(errResp.Params); msg != "" {
				return fmt.Errorf("daemon reload failed: %s", msg)
			}
			return fmt.Errorf("daemon reload failed: %s", errResp.Error)
		}

		return nil
	}
}
