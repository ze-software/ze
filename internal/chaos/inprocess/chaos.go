// Design: docs/architecture/chaos-web-dashboard.md — in-process chaos scheduling

package inprocess

import (
	"context"
	"net"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/chaos/engine"
	"github.com/ze-software/ze/internal/chaos/guard"
	"github.com/ze-software/ze/internal/chaos/mocknet"
	"github.com/ze-software/ze/internal/chaos/route"
)

// establishedState tracks which peers are in Established state.
// Written by the event-processing goroutine, read by schedulers.
type establishedState struct {
	mu    sync.RWMutex
	peers []bool
}

func newEstablishedState(n int) *establishedState {
	return &establishedState{peers: make([]bool, n)}
}

func (es *establishedState) Set(idx int, val bool) {
	es.mu.Lock()
	es.peers[idx] = val
	es.mu.Unlock()
}

func (es *establishedState) Snapshot() []bool {
	es.mu.RLock()
	snap := make([]bool, len(es.peers))
	copy(snap, es.peers)
	es.mu.RUnlock()
	return snap
}

// reconnectDialer creates mock TCP connection pairs on demand for chaos
// reconnection in in-process mode. Each DialContext call creates a new
// pair, wraps the reactor end with proper TCP addresses, and queues it
// on the MockListener.
type reconnectDialer struct {
	cpm       *mocknet.ConnPairManager
	ml        *mocknet.MockListener
	localAddr *net.TCPAddr
	peerIP    net.IP
}

func (rd *reconnectDialer) DialContext(_ context.Context, _, _ string) (net.Conn, error) {
	peerEnd, reactorEnd, err := rd.cpm.NewPair()
	if err != nil {
		return nil, err
	}
	remoteTCPAddr := &net.TCPAddr{IP: rd.peerIP, Port: 0}
	wrappedEnd := mocknet.NewConnWithAddr(reactorEnd, rd.localAddr, remoteTCPAddr)
	rd.ml.QueueConn(wrappedEnd)
	return peerEnd, nil
}

// chaosSchedulerLoop ticks the chaos scheduler using virtual time and
// dispatches actions to per-peer channels. Runs until ctx is canceled.
func chaosSchedulerLoop(ctx context.Context, sched *engine.Scheduler, g *guard.Guard, es *establishedState, channels []chan engine.ChaosAction, tickCh <-chan time.Time) {
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-tickCh:
			actions := sched.Tick(now, es.Snapshot())
			for _, a := range actions {
				if ok, _ := g.AllowChaos(a.PeerIndex, a.Action.Type); !ok {
					continue
				}
				select {
				case channels[a.PeerIndex] <- a.Action:
					if a.Action.Type == engine.ActionHoldTimerExpiry {
						g.OnHoldTimerExpiry(a.PeerIndex)
					}
				default: // non-blocking: drop action if peer is busy processing previous one
				}
			}
		}
	}
}

// routeSchedulerLoop ticks the route scheduler using virtual time and
// dispatches actions to per-peer channels. Runs until ctx is canceled.
func routeSchedulerLoop(ctx context.Context, sched *route.Scheduler, g *guard.Guard, es *establishedState, channels []chan route.Action, tickCh <-chan time.Time) {
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-tickCh:
			actions := sched.Tick(now, es.Snapshot())
			for _, a := range actions {
				if ok, _ := g.AllowRoute(a.PeerIndex, a.Action.Type); !ok {
					continue
				}
				select {
				case channels[a.PeerIndex] <- a.Action:
					if a.Action.Type == route.ActionFullWithdraw {
						g.OnFullWithdraw(a.PeerIndex)
					}
				default: // non-blocking: drop action if peer is busy processing previous one
				}
			}
		}
	}
}
