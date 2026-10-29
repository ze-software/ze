package dhcpserver

import (
	"errors"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

// resolveKernelDevice is the kernel device the test backend binds every
// logical name to, so a bind on the logical name itself is visible.
const resolveKernelDevice = "enp9s0"

// resolveBackend answers GetLink for one logical name with another kernel
// device, the way an os-name or mac-match selector binds it. Every other
// Backend method is left to the embedded nil interface: the resolver calls
// none of them here.
type resolveBackend struct {
	iface.Backend
	logical string
}

func (b *resolveBackend) GetLink(name string) (*iface.InterfaceInfo, error) {
	if name != b.logical {
		return nil, fmt.Errorf("resolve backend: no interface %q", name)
	}
	return &iface.InterfaceInfo{Name: name, OsName: resolveKernelDevice, Index: 9, State: iface.LinkStateUp}, nil
}

func (b *resolveBackend) Close() error { return nil }

var (
	resolveBackendOnce    sync.Once
	resolveBackendErr     error
	resolveBackendCurrent atomic.Pointer[resolveBackend]
	resolveNameSeq        atomic.Int64
)

// TestListenInterfaceBindsTheResolvedDevice configures a logical
// listen-interface the iface backend binds to another kernel device, and
// records the device dhcpListen is asked to bind.
//
// VALIDATES: the configured name goes through iface.ResolveDevice before
// SO_BINDTODEVICE, and the listener carries the kernel device that serveMulti
// and the neighbor writes then use, beside the configured name.
// PREVENTS: a listen-interface bound by name, which fails, or binds the wrong
// device, whenever the logical name is not the kernel device's name.
func TestListenInterfaceBindsTheResolvedDevice(t *testing.T) {
	const backendName = "dhcpserver-resolve-test"
	resolveBackendOnce.Do(func() {
		resolveBackendErr = iface.RegisterBackend(backendName, func() (iface.Backend, error) {
			return resolveBackendCurrent.Load(), nil
		})
	})
	if resolveBackendErr != nil {
		t.Fatalf("register resolve backend: %v", resolveBackendErr)
	}
	// A fresh logical name per run: the resolver caches bindings by name.
	logical := fmt.Sprintf("lan%d", resolveNameSeq.Add(1))
	resolveBackendCurrent.Store(&resolveBackend{logical: logical})
	if err := iface.LoadBackend(backendName); err != nil {
		t.Fatalf("load resolve backend: %v", err)
	}
	t.Cleanup(func() { _ = iface.CloseBackend() })

	orig := dhcpListen
	t.Cleanup(func() { dhcpListen = orig })
	var bound []string
	dhcpListen = func(device string) (*net.UDPConn, error) {
		bound = append(bound, device)
		return ephemeralUDP(t), nil
	}

	log, _ := captureLogger()
	got, err := bindListeners(serverConfig{Enabled: true, ListenInterfaces: []string{logical}}, nil, log, true)
	if err != nil {
		t.Fatalf("bindListeners: %v", err)
	}
	if len(bound) != 1 || bound[0] != resolveKernelDevice {
		t.Fatalf("dhcpListen bound %q, want only the kernel device %q", bound, resolveKernelDevice)
	}
	if len(got) != 1 {
		t.Fatalf("got %d listeners, want 1", len(got))
	}
	if got[0].iface != logical || got[0].device != resolveKernelDevice {
		t.Errorf("listener carries iface %q device %q, want %q and %q", got[0].iface, got[0].device, logical, resolveKernelDevice)
	}
}

// stubResolver stands in iface.ResolveDevice with a table: a name in devices
// resolves to its kernel device, and any other name is refused the way
// ResolveDevice refuses a name whose selector matches no device.
func stubResolver(t *testing.T, devices map[string]string) {
	t.Helper()
	previous := dhcpResolveDevice
	dhcpResolveDevice = func(name string) (string, error) {
		if device, ok := devices[name]; ok {
			return device, nil
		}
		return "", fmt.Errorf("iface: interface %q is bound to hardware by a selector that resolves to no device", name)
	}
	t.Cleanup(func() { dhcpResolveDevice = previous })
}

// recordListen stands in the bind with an ephemeral socket and records the
// device each bind was asked for, in order.
func recordListen(t *testing.T) *[]string {
	t.Helper()
	previous := dhcpListen
	bound := &[]string{}
	dhcpListen = func(device string) (*net.UDPConn, error) {
		*bound = append(*bound, device)
		return ephemeralUDP(t), nil
	}
	t.Cleanup(func() { dhcpListen = previous })
	return bound
}

// recordServe stands in the request loop with one that reports the device it
// was started on and returns, so no goroutine outlives the test.
func recordServe(t *testing.T) <-chan string {
	t.Helper()
	previous := dhcpServe
	served := make(chan string, 8)
	dhcpServe = func(_ *net.UDPConn, device string, _ []*dhcpHandler, _ *slog.Logger) {
		served <- device
	}
	t.Cleanup(func() { dhcpServe = previous })
	return served
}

// awaitServed answers the device the next request loop started on, failing
// the test when none starts within a few seconds.
func awaitServed(t *testing.T, served <-chan string) string {
	t.Helper()
	select {
	case device := <-served:
		return device
	case <-time.After(5 * time.Second):
		t.Fatal("no request loop started")
		return ""
	}
}

// TestBindListenersServesTheResolvedDevice binds a logical listen-interface
// and records the device the request loop is started on.
//
// VALIDATES: serveMulti receives the kernel device, so a reply's neighbor
// entry is read and written on the device the socket is bound to.
// PREVENTS: the loop handed the logical name, which writes neighbor entries on
// whatever device wears that name.
func TestBindListenersServesTheResolvedDevice(t *testing.T) {
	stubResolver(t, map[string]string{"lan": resolveKernelDevice})
	recordListen(t)
	served := recordServe(t)

	log, _ := captureLogger()
	if _, err := bindListeners(serverConfig{Enabled: true, ListenInterfaces: []string{"lan"}}, nil, log, true); err != nil {
		t.Fatalf("bindListeners: %v", err)
	}
	if device := awaitServed(t, served); device != resolveKernelDevice {
		t.Fatalf("request loop started on %q, want the kernel device %q", device, resolveKernelDevice)
	}
}

// TestRestoreBindsAndServesTheResolvedDevice restores a listener a refused
// reload closed.
//
// VALIDATES: restore binds and serves the device the listener was bound to
// before, and keeps both names on the listener it records.
// PREVENTS: a rollback that rebinds the logical name, putting the server back
// on a device the operator's selector never chose.
func TestRestoreBindsAndServesTheResolvedDevice(t *testing.T) {
	bound := recordListen(t)
	served := recordServe(t)
	log, _ := captureLogger()
	s := &dhcpServer{log: log}
	cause := errors.New("the new listener failed")

	err := s.restore([]dhcpListener{{iface: "lan", device: resolveKernelDevice}}, cause)
	if !errors.Is(err, cause) {
		t.Fatalf("restore answered %v, want the install cause", err)
	}
	if len(*bound) != 1 || (*bound)[0] != resolveKernelDevice {
		t.Fatalf("restore bound %q, want only the kernel device %q", *bound, resolveKernelDevice)
	}
	if device := awaitServed(t, served); device != resolveKernelDevice {
		t.Fatalf("restore served %q, want the kernel device %q", device, resolveKernelDevice)
	}
	if len(s.listeners) != 1 || s.listeners[0].iface != "lan" || s.listeners[0].device != resolveKernelDevice {
		t.Fatalf("restored listeners %+v, want lan on %q", s.listeners, resolveKernelDevice)
	}
}

// TestBindListenersRefusesAnUnresolvedSelector configures a listen-interface
// whose selector resolves to no device, in both bind modes.
//
// VALIDATES: nothing is bound, a reload is refused with a fix naming the
// os-name or mac match selector, and boot serves nothing on that interface.
// PREVENTS: the logical name reaching SO_BINDTODEVICE after a failed
// resolution, binding a device that only wears that name.
func TestBindListenersRefusesAnUnresolvedSelector(t *testing.T) {
	stubResolver(t, map[string]string{})
	bound := recordListen(t)
	recordServe(t)
	log, _ := captureLogger()
	cfg := serverConfig{Enabled: true, ListenInterfaces: []string{"lan"}}

	_, err := bindListeners(cfg, nil, log, true)
	if err == nil {
		t.Fatal("reload bound an interface whose selector resolves to no device")
	}
	if !strings.Contains(err.Error(), `Fix: make the os-name or mac match selector of interface "lan"`) {
		t.Fatalf("refusal %q does not name the selector fix", err)
	}
	listeners, err := bindListeners(cfg, nil, log, false)
	if err != nil {
		t.Fatalf("boot bind: %v", err)
	}
	if len(listeners) != 0 {
		t.Fatalf("boot bound %d listeners, want none", len(listeners))
	}
	if len(*bound) != 0 {
		t.Fatalf("dhcpListen was asked for %q, want no bind", *bound)
	}
}

// TestCheckDHCPInterfacesProbesTheResolvedDevice runs the doctor check over a
// config tree whose ethernet entries bind the listen interfaces by selector:
// "lan" by os-name to a kernel device of another name, and "wan" by mac match
// to an address no device carries. The check runs in the CLI process, so
// nothing but the tree can answer which device each name binds.
//
// VALIDATES: the probe asks the host for the os-name device, so "lan" is
// clean on a host that holds only that device, and an unmatched mac match is
// reported at the warning severity the iface check gives a deferred binding.
// PREVENTS: a selector-bound interface reported as "not found" by a probe of
// its logical name, or a lookalike device of that name passing the check.
func TestCheckDHCPInterfacesProbesTheResolvedDevice(t *testing.T) {
	withHostInterfaces(t, resolveKernelDevice, "wan")

	tree := dhcpServerTree(true, "lan", "wan")
	ifaceBlock := tree.GetOrCreateContainer("interface")
	lan := config.NewTree()
	lan.Set("os-name", resolveKernelDevice)
	ifaceBlock.AddListEntry("ethernet", "lan", lan)
	wan := config.NewTree()
	wan.GetOrCreateContainer("mac").Set("match", "02:00:5e:00:fe:42")
	ifaceBlock.AddListEntry("ethernet", "wan", wan)

	diags := checkDHCPInterfaces(diagnostic.DoctorCheckContext{Tree: tree})
	if len(diags) != 1 {
		t.Fatalf("diagnostics = %d, want 1 for wan only: %+v", len(diags), diags)
	}
	if diags[0].Severity != diagnostic.SeverityWarning {
		t.Fatalf("severity = %s, want warning for an unmatched selector", diags[0].Severity)
	}
	// The selector text differs by platform (off Linux no address is read, so
	// the MAC is reported unchecked), but it always names the interface and
	// the MAC the operator configured.
	if !strings.Contains(diags[0].Message, "listen interface wan has no single device to bind: ethernet wan: ") {
		t.Fatalf("message = %q, want the unmatched interface named", diags[0].Message)
	}
	if !strings.Contains(diags[0].Message, "MAC 02:00:5e:00:fe:42") {
		t.Fatalf("message = %q, want the unmatched mac match named", diags[0].Message)
	}
}

// TestRunDHCPServerPluginRefusesExternalProcess starts the plugin on a plain
// net.Pipe end, which is what an external process's connection looks like to
// the SDK (sdk.Plugin.IsInternal).
//
// VALIDATES: the plugin exits 1 at once, without waiting for an engine, and
// its log line names the in-process fix.
// PREVENTS: a forked dhcpserver whose empty selector table binds every
// listen-interface to whatever device wears the logical name.
func TestRunDHCPServerPluginRefusesExternalProcess(t *testing.T) {
	log, buf := captureLogger()
	previous := loggerPtr.Load()
	loggerPtr.Store(log)
	t.Cleanup(func() { loggerPtr.Store(previous) })

	pluginEnd, engineEnd := net.Pipe()
	t.Cleanup(func() {
		pluginEnd.Close() //nolint:errcheck // test cleanup
		engineEnd.Close() //nolint:errcheck // test cleanup
	})

	exit := make(chan int, 1)
	go func() { exit <- runDHCPServerPlugin(pluginEnd) }()
	select {
	case code := <-exit:
		if code != 1 {
			t.Fatalf("runDHCPServerPlugin(external conn) = %d, want 1", code)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("runDHCPServerPlugin(external conn) did not refuse; it waited for an engine")
	}
	if !strings.Contains(buf.String(), "internal dhcpserver") {
		t.Fatalf("log %q does not name the in-process fix", buf.String())
	}
}
