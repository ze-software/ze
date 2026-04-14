// Design: docs/research/vpp-deployment-reference.md -- VPP stats segment connection
// Related: telemetry.go -- stats poller consuming the connection
// Related: conn.go -- binary API connection (separate socket)

package vpp

import (
	"fmt"

	"go.fd.io/govpp/adapter/statsclient"
	"go.fd.io/govpp/core"
)

// connectStats creates a stats connection to VPP's stats segment.
// Returns a *core.StatsConnection that satisfies statsProvider, or an error.
// The stats segment is separate from the binary API socket.
// Caller MUST call Disconnect on the returned connection when done.
func connectStats(socketPath string) (*core.StatsConnection, error) {
	client := statsclient.NewStatsClient(socketPath)
	conn, err := core.ConnectStats(client)
	if err != nil {
		return nil, fmt.Errorf("stats connect %s: %w", socketPath, err)
	}
	return conn, nil
}
