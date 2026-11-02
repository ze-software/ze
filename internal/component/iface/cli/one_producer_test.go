package cli

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"testing"

	"github.com/ze-software/ze/internal/component/command/commandtest"
	ifacepkg "github.com/ze-software/ze/internal/component/iface"
	_ "github.com/ze-software/ze/internal/component/iface/cmd" // registers the daemon RPCs compared here
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

// showBackend answers ListInterfaces and GetInterface from a fixed set, so the
// test knows what the producer holds. Every other Backend method is the nil
// embedded interface: a route that reached one would panic the test.
type showBackend struct {
	ifacepkg.Backend
	ifs []ifacepkg.InterfaceInfo
}

func (b *showBackend) ListInterfaces() ([]ifacepkg.InterfaceInfo, error) {
	return append([]ifacepkg.InterfaceInfo(nil), b.ifs...), nil
}

func (b *showBackend) GetInterface(name string) (*ifacepkg.InterfaceInfo, error) {
	for i := range b.ifs {
		if b.ifs[i].Name == name {
			info := b.ifs[i]
			return &info, nil
		}
	}
	return nil, errors.New("interface not found: " + name)
}

func (b *showBackend) Close() error { return nil }

// TestShowInterfaceOneProducer proves AC-14 for show interface: the no-daemon
// route (cmdShow, which `ze show interface` reaches through Run) and the
// daemon RPCs (ze-iface:show-interface, ze-iface:show-interface-detail) answer
// from one producer, iface.ListInterfaces and iface.GetInterface.
//
// Method: load a backend holding interfaces no host has, then compare the
// local --json output with the daemon payload, for the list and for one
// interface. A route that read anything but the loaded backend would answer
// the host's interfaces, or none, and differ.
func TestShowInterfaceOneProducer(t *testing.T) {
	backend := &showBackend{ifs: []ifacepkg.InterfaceInfo{
		{Name: "one-producer-a", Index: 901, Type: "dummy", State: "up", MTU: 1500, MAC: "02:00:00:00:09:01"},
		{Name: "one-producer-b", Index: 902, Type: "veth", State: "down", MTU: 9000},
	}}
	const name = "one-producer-test"
	if err := ifacepkg.RegisterBackend(name, func() (ifacepkg.Backend, error) { return backend, nil }); err != nil {
		t.Fatal(err)
	}
	// The package's init loads the netlink backend, and the parallel tests
	// (TestCmdShowSuccess) run after this serial one and read it: put it back.
	previous := ifacepkg.ActiveBackendName()
	if err := ifacepkg.LoadBackend(name); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if previous == "" {
			if err := ifacepkg.CloseBackend(); err != nil {
				t.Errorf("close backend: %v", err)
			}
			return
		}
		if err := ifacepkg.LoadBackend(previous); err != nil {
			t.Errorf("restore backend %q: %v", previous, err)
		}
	})

	daemonList := callShowInterfaceRPC(t, "ze-iface:show-interface")
	localList := captureShow(t, "--json")
	if daemonList != localList {
		t.Errorf("show interface differs:\ndaemon %s\nlocal  %s", daemonList, localList)
	}

	daemonOne := callShowInterfaceRPC(t, "ze-iface:show-interface-detail", "one-producer-a")
	localOne := captureShow(t, "--json", "one-producer-a")
	if daemonOne != localOne {
		t.Errorf("show interface one-producer-a differs:\ndaemon %s\nlocal  %s", daemonOne, localOne)
	}
}

// callShowInterfaceRPC runs the registered daemon handler of wire and answers
// its payload re-encoded as compact JSON.
func callShowInterfaceRPC(t *testing.T, wire string, words ...string) string {
	t.Helper()
	for _, rpc := range pluginserver.AllBuiltinRPCs() {
		if rpc.WireMethod != wire {
			continue
		}
		resp, err := rpc.Handler(nil, commandtest.Args(words...))
		if err != nil {
			t.Fatalf("%s: %v", wire, err)
		}
		if resp.Status != plugin.StatusDone {
			t.Fatalf("%s: status %q: %s", wire, resp.Status, resp.Error)
		}
		return compactJSON(t, resp.Data)
	}
	t.Fatalf("%s: no builtin RPC registered", wire)
	return ""
}

// captureShow runs cmdShow with args and answers its stdout re-encoded as
// compact JSON.
func captureShow(t *testing.T, args ...string) string {
	t.Helper()
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	saved := os.Stdout
	os.Stdout = writer
	code := cmdShow(args)
	os.Stdout = saved
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	out, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	if code != 0 {
		t.Fatalf("cmdShow %q exit %d: %s", args, code, out)
	}
	var decoded any
	if err := json.Unmarshal(out, &decoded); err != nil {
		t.Fatalf("cmdShow %q: %v: %s", args, err, out)
	}
	return compactJSON(t, decoded)
}

func compactJSON(t *testing.T, v any) string {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var decoded any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	out, err := json.Marshal(decoded)
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}
