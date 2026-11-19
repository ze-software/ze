// Design: docs/research/vpp-deployment-reference.md -- VPP stats segment connection
// Related: telemetry.go -- stats poller consuming the connection
// Related: conn.go -- binary API connection (separate socket)

package vpp

import (
	"errors"
	"fmt"
	"net"

	"go.fd.io/govpp/adapter/statsclient"
	"go.fd.io/govpp/core"
)

// connectStats creates a stats connection to VPP's stats segment.
// Returns one immutable mapping with its actual exchange-socket peer, or an error.
// The stats segment is separate from the binary API socket.
// Caller MUST call Disconnect on the returned connection when done.
func connectStats(socketPath string, owner *Connector, generation uint64) (*processStatsConnection, error) {
	if owner == nil {
		return nil, errors.New("vpp: statistics require the original API owner")
	}
	var peer *processWitness
	client := statsclient.NewStatsClient(socketPath, statsclient.WithPinnedConnection(func(connection *net.UnixConn) error {
		var err error
		peer, err = captureProcessWitness(connection)
		if err != nil {
			return err
		}
		// This same original socket must be admitted BEFORE receiving or
		// mapping its segment. Bad foreign segment bytes retain their cause.
		return owner.checkStatsPeer(generation, peer)
	}))
	conn, err := core.ConnectStats(client)
	if err != nil {
		if peer != nil {
			err = errors.Join(err, peer.release())
		}
		return nil, fmt.Errorf("stats connect %s: %w", socketPath, err)
	}
	if peer == nil {
		conn.Disconnect()
		return nil, errors.New("vpp: stats mapping has no physical process witness")
	}
	return &processStatsConnection{StatsConnection: conn, peer: peer}, nil
}
