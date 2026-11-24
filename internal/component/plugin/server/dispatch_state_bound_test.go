// Design: docs/architecture/api/process-protocol.md -- per-key state bounds and read grants.
package server

import (
	"bytes"
	"encoding/json"
	"slices"
	"testing"

	"github.com/ze-software/ze/internal/component/config/storage"
	plugin "github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/core/statestore"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/zefs"
)

// useStateTestStore gives the test a fresh daemon store and removes it when the
// test ends, so no state reaches the next test.
func useStateTestStore(t *testing.T) {
	t.Helper()
	statestore.SetStore(nil)
	store, err := storage.Create(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		statestore.SetStore(nil)
		if err := store.Close(); err != nil {
			t.Error(err)
		}
	})
	statestore.SetStore(store)
}

// TestPluginStateDeclaredBoundBothTransports proves a key's declared bound
// (zefs.IRRDatasetValueMax on the IRR dataset key) carries a value of exactly
// that size through the SDK, the dispatcher and ZeFS on both transports, and
// that a value one byte larger is refused by the daemon as well as the SDK.
//
// Method: over the socket and over DirectBridge, put and get a value of the
// declared size and compare it byte for byte. Then hand the daemon's request
// check (stateInput) the bound plus one byte, which the SDK never sends, and a
// pointer key, which declares no bound, StateDataMax plus one byte: both are
// refused, and the pointer at StateDataMax passes.
func TestPluginStateDeclaredBoundBothTransports(t *testing.T) {
	const owner = "state-bound-test"
	statestore.RegisterPluginKeys(owner, zefs.KeyIRRDataset, zefs.KeyIRRSet)
	dataset := zefs.KeyIRRDataset.Key("whois.radb.net", "AS-TEST", "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	value := bytes.Repeat([]byte{0xa5, 0x5a, 0x00, 0xff}, zefs.IRRDatasetValueMax/4)
	for _, direct := range []bool{false, true} {
		name := "socket"
		if direct {
			name = "direct"
		}
		t.Run(name, func(t *testing.T) {
			p := stateRPCClientAs(t, direct, owner)
			useStateTestStore(t)
			ctx, cancel := stateRPCContext(t)
			defer cancel()
			if err := p.StatePut(ctx, dataset, value); err != nil {
				t.Fatalf("put of %d bytes, the declared bound: %v", len(value), err)
			}
			read, found, err := p.StateGet(ctx, dataset)
			if err != nil || !found {
				t.Fatalf("get after put: found=%v error=%v", found, err)
			}
			if !bytes.Equal(read, value) {
				t.Fatalf("get returned %d bytes, not the %d written", len(read), len(value))
			}
			if err := p.StatePut(ctx, dataset, append(value, 0)); err == nil {
				t.Fatal("SDK sent a value one byte over the declared bound")
			}
		})
	}
	proc := process.NewProcess(plugin.PluginConfig{Name: owner, CompiledPlugin: owner, Internal: true})
	requestOf := func(key string, size int) json.RawMessage {
		raw, err := json.Marshal(rpc.StateInput{Key: key, Data: make([]byte, size)})
		if err != nil {
			t.Fatal(err)
		}
		return raw
	}
	if _, err := stateInput(proc, requestOf(dataset, zefs.IRRDatasetValueMax+1), stateAccessWrite); err == nil {
		t.Fatal("daemon accepted a dataset one byte over its declared bound")
	}
	pointer := zefs.KeyIRRSet.Key("AS-TEST")
	if _, err := stateInput(proc, requestOf(pointer, rpc.StateDataMax), stateAccessWrite); err != nil {
		t.Fatalf("daemon refused a pointer at the default bound: %v", err)
	}
	if _, err := stateInput(proc, requestOf(pointer, rpc.StateDataMax+1), stateAccessWrite); err == nil {
		t.Fatal("daemon accepted a pointer one byte over the default bound")
	}
}

// TestPluginStateReadGrantBothTransports proves a plugin holding a read grant
// reads and lists another plugin's keys and can write, remove or increment
// none of them, on both transports; the refused mutations change nothing.
//
// Method: write a pointer and the generation counter into the store as the
// owner would, then, as the reader, get and list them, then try put, remove
// and increment, each of which must fail, and read both values back unchanged.
func TestPluginStateReadGrantBothTransports(t *testing.T) {
	const reader = "state-read-test"
	statestore.RegisterPluginReadKeys(reader, zefs.KeyIRRSet, zefs.KeyIRRGeneration)
	pointer := zefs.KeyIRRSet.Key("AS-TEST")
	generation := zefs.KeyIRRGeneration.Key()
	stored := []byte(`{"server":"whois.radb.net","generation":7}`)
	for _, direct := range []bool{false, true} {
		name := "socket"
		if direct {
			name = "direct"
		}
		t.Run(name, func(t *testing.T) {
			p := stateRPCClientAs(t, direct, reader)
			useStateTestStore(t)
			if err := statestore.Write(pointer, stored); err != nil {
				t.Fatal(err)
			}
			if err := statestore.Write(generation, []byte{0, 0, 0, 7}); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := stateRPCContext(t)
			defer cancel()
			read, found, err := p.StateGet(ctx, pointer)
			if err != nil || !found || !bytes.Equal(read, stored) {
				t.Fatalf("reader get: data=%q found=%v error=%v", read, found, err)
			}
			keys, err := p.StateList(ctx, zefs.KeyIRRSet.Prefix())
			if err != nil || !slices.Equal(keys, []string{pointer}) {
				t.Fatalf("reader list: keys=%v error=%v", keys, err)
			}
			if err := p.StatePut(ctx, pointer, []byte(`{}`)); err == nil {
				t.Fatal("reader wrote a key it may only read")
			}
			if err := p.StateRemove(ctx, pointer); err == nil {
				t.Fatal("reader removed a key it may only read")
			}
			if _, err := p.StateIncrement(ctx, generation); err == nil {
				t.Fatal("reader incremented a counter it may only read")
			}
			read, found, err = p.StateGet(ctx, pointer)
			if err != nil || !found || !bytes.Equal(read, stored) {
				t.Fatalf("pointer changed under refused mutations: data=%q found=%v error=%v", read, found, err)
			}
			counter, found, err := p.StateGet(ctx, generation)
			if err != nil || !found || !bytes.Equal(counter, []byte{0, 0, 0, 7}) {
				t.Fatalf("generation changed under a refused increment: data=%x found=%v error=%v", counter, found, err)
			}
		})
	}
}
