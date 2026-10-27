package bmp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/crashlog"
	"github.com/ze-software/ze/internal/core/taskguard"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// swapBMPPanics replaces the BMP panic boundary with one that logs JSON into the
// returned buffer, points crash records at a temporary directory, and restores both
// when t ends.
func swapBMPPanics(t *testing.T) (*bmpPanicTasks, *lockedBuffer) {
	t.Helper()
	restoreCrashDir := crashlog.SetCrashDirForTest(t.TempDir())
	prior := bmpPanics
	logs := &lockedBuffer{}
	guard := taskguard.New("bmp", slog.New(slog.NewJSONHandler(logs, nil)))
	tasks := &bmpPanicTasks{guard: guard, receive: guard.Task("receive", receiveRestart)}
	bmpPanics = func() *bmpPanicTasks { return tasks }
	t.Cleanup(func() {
		bmpPanics = prior
		restoreCrashDir()
	})
	return tasks, logs
}

// lockedBuffer is a bytes.Buffer the guard writes from a session goroutine while the
// test reads it.
type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// fakeEngine is the engine side of the plugin's DirectBridge: it records every
// injected route and every dispatched command, and panics on an injection from the
// router whose address prefix is armed.
type fakeEngine struct {
	mu        sync.Mutex
	armed     string
	injected  []string
	withdrawn []string
	injection chan struct{}
}

func (e *fakeEngine) inject(_, peerKey string, _ []byte) error {
	e.mu.Lock()
	armed := e.armed
	e.injected = append(e.injected, peerKey)
	e.mu.Unlock()
	e.injection <- struct{}{}
	if armed != "" && strings.HasPrefix(peerKey, armed) {
		panic("boom")
	}
	return nil
}

func (e *fakeEngine) dispatch(_ context.Context, command string, args []string, _ string) (*rpc.DispatchCommandOutput, error) {
	if command == "request bgp rib withdraw-router" && len(args) == 2 {
		e.mu.Lock()
		e.withdrawn = append(e.withdrawn, args[1])
		e.mu.Unlock()
	}
	return nil, errors.New("fake engine answers no command")
}

// routeMonitoring is one BMP Route Monitoring message carrying an empty BGP UPDATE.
func routeMonitoring() []byte {
	update := make([]byte, 23) // marker, length, type, withdrawn length, attributes length
	for i := range 16 {
		update[i] = 0xff
	}
	update[17], update[18] = 23, 2
	buf := make([]byte, 256)
	n := writeRouteMonitoring(buf, 0, &RouteMonitoring{Peer: testPeerHeader(), BGPUpdate: update})
	return buf[:n]
}

// TestBMPReceiverPanicContained proves the BMP receive task's panic boundary through the
// receiver's real entry: a TCP listener served by acceptLoop, with two monitored routers
// connected. Router A's Route Monitoring panics inside the route injection; the plugin
// must survive, end only A's session through its own teardown (close, withdraw-router,
// state removal), keep router B's session receiving, and log, count and crash-record the
// panic.
//
// The seam is the engine's typed inject-wire-route handler on the plugin's DirectBridge,
// the interface processRouteMonitoring already calls (SetInjectWireRoute). Nothing is
// added to production code.
func TestBMPReceiverPanicContained(t *testing.T) {
	tasks, logs := swapBMPPanics(t)

	engine := &fakeEngine{injection: make(chan struct{}, 8)}
	bridge := rpc.NewDirectBridge()
	bridge.SetInjectWireRoute(engine.inject)
	bridge.SetDispatchCommandArgs(engine.dispatch)
	bridge.SetReady()
	pluginSide, engineSide := net.Pipe()
	t.Cleanup(func() {
		closeLog(pluginSide, "plugin-side")
		closeLog(engineSide, "engine-side")
	})

	bp := &BMPPlugin{
		plugin: sdk.NewWithConn("bgp-bmp", rpc.NewBridgedConn(pluginSide, bridge)),
		state:  newBMPState(),
		stopCh: make(chan struct{}),
	}
	var lc net.ListenConfig
	ln, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	bp.listeners = map[string]net.Listener{ln.Addr().String(): ln}
	bp.maxSessions.Store(10)
	bp.sessions.Go(func() { bp.acceptLoop(ln.Addr().String(), ln) })
	t.Cleanup(func() {
		close(bp.stopCh)
		bp.stopListeners()
		bp.sessions.Wait()
	})

	dial := func() net.Conn {
		conn, dialErr := (&net.Dialer{Timeout: time.Second}).DialContext(t.Context(), "tcp", ln.Addr().String())
		if dialErr != nil {
			t.Fatalf("dial: %v", dialErr)
		}
		t.Cleanup(func() { closeLog(conn, "router") })
		return conn
	}
	send := func(conn net.Conn, why string) {
		if _, writeErr := conn.Write(routeMonitoring()); writeErr != nil {
			t.Fatalf("%s: %v", why, writeErr)
		}
		select {
		case <-engine.injection:
		case <-time.After(5 * time.Second):
			t.Fatalf("%s: the Route Monitoring never reached the engine", why)
		}
	}

	routerB := dial()
	send(routerB, "router B's first message")
	routerA := dial()
	remoteA := routerA.LocalAddr().String()
	engine.mu.Lock()
	engine.armed = remoteA + ":"
	engine.mu.Unlock()
	send(routerA, "router A's message")

	// Router A's session ends: ze closes the connection.
	if err := routerA.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatalf("deadline: %v", err)
	}
	if n, readErr := routerA.Read(make([]byte, 1)); !errors.Is(readErr, io.EOF) {
		t.Fatalf("router A read = %d, %v, want EOF: ze did not close the session the panic ended", n, readErr)
	}

	// Router B's session continues.
	send(routerB, "router B's message after A's panic")

	// A's teardown ran: its routes were withdrawn and its state removed; B's stays.
	deadline := time.Now().Add(5 * time.Second)
	for {
		engine.mu.Lock()
		withdrawn := append([]string(nil), engine.withdrawn...)
		engine.mu.Unlock()
		if len(withdrawn) == 1 && withdrawn[0] == remoteA {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("withdraw-router calls = %q, want exactly router A %q", withdrawn, remoteA)
		}
		time.Sleep(10 * time.Millisecond)
	}
	bp.state.mu.RLock()
	_, aKept := bp.state.routers[remoteA]
	_, bKept := bp.state.routers[routerB.LocalAddr().String()]
	bp.state.mu.RUnlock()
	if aKept {
		t.Error("router A is still in the BMP state after its session ended")
	}
	if !bKept {
		t.Error("router B left the BMP state: the panic ended a session it did not belong to")
	}

	// The boundary is the session's first defer, so it reports after the teardown above;
	// its log line is the last thing it writes.
	for logs.String() == "" {
		if time.Now().After(deadline) {
			t.Fatal("the guard never logged the contained panic")
		}
		time.Sleep(10 * time.Millisecond)
	}
	requireReceiveContained(t, tasks, logs, "bmp router "+remoteA)
}

// requireReceiveContained asserts exactly one contained receive panic with value "boom"
// on unit: one ERROR line naming the component, the task, the unit and the Route
// Monitoring in flight, one count, and one recovered crash record.
func requireReceiveContained(t *testing.T, tasks *bmpPanicTasks, logs *lockedBuffer, unit string) {
	t.Helper()
	if count, declared := tasks.guard.Panics("receive"); count != 1 || !declared {
		t.Errorf("Panics(receive) = %d, %v, want 1, true: the panic was not counted", count, declared)
	}
	lines := strings.Split(strings.TrimSpace(logs.String()), "\n")
	if len(lines) != 1 {
		t.Fatalf("got %d log lines, want 1: %q", len(lines), logs.String())
	}
	var line map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &line); err != nil {
		t.Fatalf("log line is not JSON: %v: %q", err, lines[0])
	}
	want := map[string]string{
		"level": "ERROR", "component": "bmp", "task": "receive", "unit": unit,
		"message-type": "Route Monitoring", "panic": "boom",
	}
	for key, value := range want {
		if line[key] != value {
			t.Errorf("log field %s = %v, want %q", key, line[key], value)
		}
	}
	crashes := crashlog.ListCrashes()
	if len(crashes) != 1 {
		t.Fatalf("ListCrashes returned %d records, want 1", len(crashes))
	}
	if crashes[0].Kind != crashlog.KindRecovered {
		t.Errorf("crash record kind %q, want %q", crashes[0].Kind, crashlog.KindRecovered)
	}
}
