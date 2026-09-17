// Design: docs/architecture/api/process-protocol.md -- plugin state transport parity.
package server

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/config/storage"
	plugin "github.com/ze-software/ze/internal/component/plugin"
	plugipc "github.com/ze-software/ze/internal/component/plugin/ipc"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/core/statestore"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
	"github.com/ze-software/ze/pkg/zefs"
)

func stateRPCClient(t *testing.T, direct bool) *sdk.Plugin {
	t.Helper()
	const owner = "state-rpc-test"
	statestore.RegisterPluginKeys(owner, zefs.KeyOSPFAuthBootCount, zefs.KeyOSPFGRFact)
	s := &Server{}
	s.ctx, s.cancel = context.WithCancel(context.Background())
	t.Cleanup(s.cancel)
	proc := process.NewProcess(plugin.PluginConfig{Name: owner})
	client, engine := net.Pipe()
	conn := client
	if direct {
		bridge := rpc.NewDirectBridge()
		bridge.SetDeliverEvents(func([]string) error { return nil })
		bridge.SetDispatchRPC(func(method string, params json.RawMessage) (json.RawMessage, error) {
			return s.dispatchPluginRPCDirect(proc, method, params)
		})
		bridge.SetReady()
		conn = rpc.NewBridgedConn(client, bridge)
	}
	p := sdk.NewWithConn(owner, conn)
	done := make(chan struct{})
	if !direct {
		pc := plugipc.NewPluginConn(engine, engine)
		go func() {
			defer close(done)
			for {
				req, err := pc.ReadRequest(s.ctx)
				if err != nil {
					return
				}
				s.serveEngineOpJSON(proc, pc, req, lookupEngineOp(req.Method))
			}
		}()
	} else {
		close(done)
	}
	t.Cleanup(func() {
		if err := p.Close(); err != nil {
			t.Error(err)
		}
		if err := engine.Close(); err != nil {
			t.Error(err)
		}
		<-done
	})
	return p
}

func requireStateStatus(t *testing.T, err error, want rpc.StateStatus) {
	t.Helper()
	var stateErr *rpc.StateError
	if !errors.As(err, &stateErr) {
		t.Fatalf("state error = %v, want %s", err, want)
	}
	if stateErr.Status != want {
		t.Fatalf("state status = %s, want %s", stateErr.Status, want)
	}
}

// Both transports must expose persisted bytes and explicit failures. The owner
// grant must protect credentials even when a plugin supplies a valid raw key.
func TestPluginStateTransportParity(t *testing.T) {
	for _, direct := range []bool{false, true} {
		name := "socket"
		if direct {
			name = "direct"
		}
		t.Run(name, func(t *testing.T) {
			p := stateRPCClient(t, direct)
			// The store is created before the deadline starts: seeding a
			// tree takes seconds under the race detector on a loaded
			// machine, and the deadline bounds the RPC round trips only.
			statestore.SetStore(nil)
			t.Cleanup(func() { statestore.SetStore(nil) })
			dir := t.TempDir()
			store, err := storage.Create(dir)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				statestore.SetStore(nil)
				if err := store.Close(); err != nil {
					t.Error(err)
				}
			})
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			key := zefs.KeyOSPFAuthBootCount.Key()
			requireStateStatus(t, p.StatePut(ctx, key, []byte{1}), rpc.StateUnavailable)
			_, _, err = p.StateGet(ctx, key)
			requireStateStatus(t, err, rpc.StateUnavailable)
			statestore.SetStore(store)
			if _, found, err := p.StateGet(ctx, key); err != nil || found {
				t.Fatalf("absent key: found=%v error=%v", found, err)
			}
			if err := p.StatePut(ctx, key, []byte{0, 0, 0, 9}); err != nil {
				t.Fatal(err)
			}
			if value, err := p.StateIncrement(ctx, key); err != nil || value != 10 {
				t.Fatalf("increment: value=%d error=%v", value, err)
			}
			data, found, err := p.StateGet(ctx, key)
			if err != nil || !found || string(data) != "\x00\x00\x00\x0a" {
				t.Fatalf("read persisted counter: data=%x found=%v error=%v", data, found, err)
			}
			keys, err := p.StateList(ctx, "meta/ospf/")
			if err != nil || len(keys) != 1 || keys[0] != key {
				t.Fatalf("owned list: %v %v", keys, err)
			}
			if _, _, err := p.StateGet(ctx, "meta/auth/local/password"); err == nil {
				t.Fatal("plugin read credentials outside its grant")
			}
			if err := p.StatePut(ctx, key, []byte{1}); err != nil {
				t.Fatal(err)
			}
			_, err = p.StateIncrement(ctx, key)
			requireStateStatus(t, err, rpc.StateCorrupt)
			if err := p.StateRemove(ctx, key); err != nil {
				t.Fatal(err)
			}
			if _, found, err := p.StateGet(ctx, key); err != nil || found {
				t.Fatalf("remove: %v %v", found, err)
			}
			statestore.SetStore(corruptStateStore{Storage: store})
			_, _, err = p.StateGet(ctx, key)
			requireStateStatus(t, err, rpc.StateCorrupt)
			readonly, err := storage.OpenReadOnly(dir)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				statestore.SetStore(nil)
				if err := readonly.Close(); err != nil {
					t.Error(err)
				}
			})
			statestore.SetStore(readonly)
			requireStateStatus(t, p.StatePut(ctx, key, []byte{1}), rpc.StatePersistFailed)
			canceled, stop := context.WithCancel(ctx)
			stop()
			if err := p.StatePut(canceled, key, nil); !errors.Is(err, context.Canceled) {
				t.Fatalf("cancellation: %v", err)
			}
		})
	}
}

type corruptStateStore struct{ storage.Storage }

func (corruptStateStore) ReadKey(string) ([]byte, error) {
	return nil, storage.ErrCorrupt
}
