// Design: docs/architecture/api/process-protocol.md -- Stage 5 precedes bridge activation

package l2tpauthradius

import (
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"

	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// authReadyReadGate holds bytes already copied by the real net.Pipe reader.
// It delays the SDK's receipt without delaying the engine's successful write.
type authReadyReadGate struct {
	armed    atomic.Bool
	held     chan struct{}
	release  chan struct{}
	released sync.Once
}

func newAuthReadyReadGate() *authReadyReadGate {
	return &authReadyReadGate{held: make(chan struct{}), release: make(chan struct{})}
}

func (gate *authReadyReadGate) releaseRead() {
	gate.released.Do(func() { close(gate.release) })
}

type authReadyReadConn struct {
	net.Conn
	gate *authReadyReadGate
}

func (conn *authReadyReadConn) Read(buf []byte) (int, error) {
	count, err := conn.Conn.Read(buf)
	if count > 0 && conn.gate.armed.Swap(false) {
		close(conn.gate.held)
		<-conn.gate.release
	}
	return count, err
}

// Close MUST release the held reader before joining the SDK during cleanup.
func (conn *authReadyReadConn) Close() error {
	conn.gate.releaseRead()
	return conn.Conn.Close()
}

// The registered engine must become callable before its startup fixture returns,
// even when the SDK receives the successful Stage 5 answer after the sender does.
func TestAuthProviderStartupWaitsForSDKActivation(t *testing.T) {
	// The first subscription starts os/signal's process-wide watcher. Initialize
	// it outside the bubble; cancel this ordinary subscription immediately.
	// The real plugin still obtains and releases its own SignalContext below.
	_, stopSignals := sdk.SignalContext()
	stopSignals()
	synctest.Test(t, func(t *testing.T) {
		gate := newAuthReadyReadGate()
		t.Cleanup(gate.releaseRead)
		observed := make(chan *rpc.DirectBridge, 1)
		started := make(chan *rpc.DirectBridge, 1)
		pipe := func() (net.Conn, net.Conn) {
			pluginEnd, engineEnd := net.Pipe()
			return &authReadyReadConn{Conn: pluginEnd, gate: gate}, engineEnd
		}
		beforeReadyReply := func(bridge *rpc.DirectBridge) {
			gate.armed.Store(true)
			observed <- bridge
		}
		go func() {
			defer close(started)
			_, bridge := startAuthProviderWithPipe(t, Name, "", pipe, beforeReadyReply)
			started <- bridge
		}()
		<-gate.held
		bridge := <-observed
		if bridge.Ready() {
			t.Fatal("SDK activated before receiving its held Stage 5 result")
		}
		synctest.Wait()
		select {
		case <-started:
			t.Fatal("provider fixture returned before SDK callback activation")
		default:
		}
		gate.releaseRead()
		returned := <-started
		if returned != bridge {
			t.Fatal("startup returned a replacement bridge")
		}
		if !returned.Ready() {
			t.Fatal("provider fixture returned without SDK callback activation")
		}
	})
}
