// Design: docs/research/vpp-deployment-reference.md -- pinned native transports
// Related: process_peer.go -- retained resource leases; stats_conn.go -- stats socket
package vpp

import (
	"sync"

	"go.fd.io/govpp/core"
)

type processAPIConnection struct {
	*core.Connection
	peer *processWitness
	once sync.Once
}

func (c *processAPIConnection) processPeer() *processWitness { return c.peer }
func (c *processAPIConnection) Disconnect() {
	c.once.Do(func() {
		c.Connection.Disconnect()
		if err := c.peer.release(); err != nil {
			reportProcessFailure(c.peer, "descriptor-close", err)
		}
	})
}

type processStatsConnection struct {
	*core.StatsConnection
	peer *processWitness
	once sync.Once
}

func (c *processStatsConnection) processPeer() *processWitness { return c.peer }
func (c *processStatsConnection) Disconnect() {
	c.once.Do(func() {
		c.StatsConnection.Disconnect()
		if err := c.peer.release(); err != nil {
			reportProcessFailure(c.peer, "descriptor-close", err)
		}
	})
}
