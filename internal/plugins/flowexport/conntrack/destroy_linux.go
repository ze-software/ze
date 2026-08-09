//go:build linux

// Design: docs/architecture/flowexport/flow-export-2-flow-records.md -- conntrack destroy-event listener
// Related: destroy.go -- ctnetlink event parser

package conntrack

import (
	"fmt"

	"github.com/mdlayher/netlink"
	"golang.org/x/sys/unix"
)

// DestroyListener subscribes to the kernel's NFNLGRP_CONNTRACK_DESTROY
// multicast group and decodes each flow-teardown event. Destroy events carry
// the flow's final cumulative counters, so exporting on destroy captures the
// residual traffic of short-lived flows that begin and end between two periodic
// table dumps -- the dump-only path would miss them entirely.
type DestroyListener struct {
	conn *netlink.Conn
}

// NewDestroyListener opens a NETLINK_NETFILTER socket and joins the conntrack
// destroy multicast group. It does not send a dump request; it only receives
// asynchronous teardown events.
func NewDestroyListener() (*DestroyListener, error) {
	conn, err := netlink.Dial(unix.NETLINK_NETFILTER, nil)
	if err != nil {
		return nil, fmt.Errorf("conntrack destroy: dial netfilter: %w", err)
	}
	if err := conn.JoinGroup(unix.NFNLGRP_CONNTRACK_DESTROY); err != nil {
		closeErr := conn.Close()
		if closeErr != nil {
			return nil, fmt.Errorf("conntrack destroy: join group: %w (close: %w)", err, closeErr)
		}
		return nil, fmt.Errorf("conntrack destroy: join group: %w", err)
	}
	return &DestroyListener{conn: conn}, nil
}

// Read blocks until one or more destroy events arrive and returns the flows
// that parsed successfully. Events whose tuple cannot be decoded (e.g. a
// protocol family we do not track) are skipped rather than failing the batch.
func (l *DestroyListener) Read() ([]FlowEntry, error) {
	msgs, err := l.conn.Receive()
	if err != nil {
		return nil, fmt.Errorf("conntrack destroy: receive: %w", err)
	}

	entries := make([]FlowEntry, 0, len(msgs))
	for i := range msgs {
		e, ok := parseConntrackEvent(msgs[i].Data)
		if !ok {
			continue
		}
		entries = append(entries, e)
	}
	return entries, nil
}

// Close shuts down the netlink socket, unblocking any in-flight Read.
func (l *DestroyListener) Close() error {
	return l.conn.Close()
}
