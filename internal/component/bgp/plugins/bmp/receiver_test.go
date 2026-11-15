// Design: docs/guide/bmp.md -- receiver listener changes are all or none.
// Related: receiver.go -- the installer exercised with real TCP sockets.

package bmp

import (
	"errors"
	"net"
	"strings"
	"syscall"
	"testing"
	"time"
)

// TestReceiverBindFailureClosesStagedListeners asks whether a failed set can
// leave a successful sibling bound. The available endpoint is staged first.
func TestReceiverBindFailureClosesStagedListeners(t *testing.T) {
	blocked, available := receiverBindOrderedConflict(t)
	bp := receiverBindPlugin(t)
	err := bp.applyReceiverConfig(receiverBindConfig(t, "9", map[string]string{
		"available": available,
		"blocked":   blocked.Addr().String(),
	}))
	if err == nil {
		t.Error("initial receiver install accepted an occupied endpoint")
	}
	if len(bp.listeners) != 0 {
		t.Errorf("failed initial bind left %d receiver listeners, want none", len(bp.listeners))
	}
	var lc net.ListenConfig
	rebound, err := lc.Listen(t.Context(), "tcp", available)
	if err != nil {
		t.Fatalf("failed initial bind retained the available sibling %s: %v", available, err)
	}
	closeLog(rebound, "test-rebound")
}

// TestReceiverFailedApplyPreservesListenersAndCap checks that an unavailable
// addition cannot retire an old listener or publish a candidate session cap.
func TestReceiverFailedApplyPreservesListenersAndCap(t *testing.T) {
	bp := receiverBindPlugin(t)
	if err := bp.applyReceiverConfig(receiverBindConfig(t, "4", map[string]string{"old": "127.0.0.1:0"})); err != nil {
		t.Fatal(err)
	}
	old := bp.listeners["127.0.0.1:0"]
	if old == nil {
		t.Fatal("initial receiver was not bound")
	}
	blocked := receiverBindListen(t)
	if err := bp.applyReceiverConfig(receiverBindConfig(t, "8", map[string]string{"blocked": blocked.Addr().String()})); err == nil {
		t.Error("receiver install accepted an occupied endpoint")
	}
	if bp.listeners["127.0.0.1:0"] != old {
		t.Error("failed apply removed the previously serving listener")
	}
	if cap := bp.maxSessions.Load(); cap != 4 {
		t.Errorf("failed apply published session cap %d, want previous cap 4", cap)
	}
	receiverBindDial(t, old.Addr().String())
}

// TestReceiverSuccessfulReconcileKeepsSessions proves the useful counterpart:
// additions, removals and cap changes work without replacing a retained socket
// or stopping an accepted session of a removed listener.
func TestReceiverSuccessfulReconcileKeepsSessions(t *testing.T) {
	bp := receiverBindPlugin(t)
	removedAddr := receiverBindAvailableAddress(t)
	if err := bp.applyReceiverConfig(receiverBindConfig(t, "4", map[string]string{
		"kept": "127.0.0.1:0", "removed": removedAddr,
	})); err != nil {
		t.Fatal(err)
	}
	kept := bp.listeners["127.0.0.1:0"]
	if kept == nil {
		t.Fatal("initial kept listener was not bound")
	}
	keptConn := receiverBindDial(t, kept.Addr().String())
	removedConn := receiverBindDial(t, removedAddr)
	receiverBindSendName(t, bp, keptConn, "kept-before")
	receiverBindSendName(t, bp, removedConn, "removed-before")
	addedAddr := receiverBindAvailableAddress(t)
	if err := bp.applyReceiverConfig(receiverBindConfig(t, "6", map[string]string{
		"kept": "127.0.0.1:0", "added": addedAddr,
	})); err != nil {
		t.Fatal(err)
	}
	if bp.listeners["127.0.0.1:0"] != kept {
		t.Fatal("successful apply replaced an unchanged listener")
	}
	if cap := bp.maxSessions.Load(); cap != 6 {
		t.Fatalf("successful apply session cap %d, want 6", cap)
	}
	receiverBindDial(t, addedAddr)
	receiverBindSendName(t, bp, keptConn, "kept-after")
	receiverBindSendName(t, bp, removedConn, "removed-after")
	var lc net.ListenConfig
	reclaimed, err := lc.Listen(t.Context(), "tcp", removedAddr)
	if err != nil {
		t.Fatalf("removed listener still owns %s: %v", removedAddr, err)
	}
	closeLog(reclaimed, "test-reclaimed")
	if err := bp.applyReceiverConfig(&receiverConfig{}); err != nil {
		t.Fatal(err)
	}
	if len(bp.listeners) != 0 {
		t.Fatalf("disabled receiver kept %d listeners", len(bp.listeners))
	}
	receiverBindSendName(t, bp, keptConn, "kept-disabled")
}

// TestReceiverOccupiedReplacementPreservesOldListener forces an exact-address
// collision: the old socket's configured port-zero key differs from its actual
// endpoint, so replacing it requires a bind while that exact endpoint is held.
func TestReceiverOccupiedReplacementPreservesOldListener(t *testing.T) {
	bp := receiverBindPlugin(t)
	if err := bp.applyReceiverConfig(receiverBindConfig(t, "4", map[string]string{"old": "127.0.0.1:0"})); err != nil {
		t.Fatal(err)
	}
	old := bp.listeners["127.0.0.1:0"]
	if old == nil {
		t.Fatal("initial receiver was not bound")
	}
	target := old.Addr().String()
	if err := bp.applyReceiverConfig(receiverBindConfig(t, "8", map[string]string{"replacement": target})); err == nil {
		t.Fatal("receiver install accepted an endpoint its old socket still holds")
	}
	if bp.listeners["127.0.0.1:0"] != old {
		t.Fatal("failed exact-address replacement closed the running listener")
	}
	if cap := bp.maxSessions.Load(); cap != 4 {
		t.Fatalf("failed exact-address replacement changed cap to %d, want 4", cap)
	}
	receiverBindDial(t, target)
}

// TestReceiverOverlappingReplacementPreservesOldListener checks the kernel's
// own coexistence verdict first. Ze must preserve the old listener when the
// kernel refuses overlap, and install the wanted replacement when it permits
// overlap. An unexpected probe failure is an error, never evidence of refusal.
func TestReceiverOverlappingReplacementPreservesOldListener(t *testing.T) {
	bp := receiverBindPlugin(t)
	if err := bp.applyReceiverConfig(receiverBindConfig(t, "4", map[string]string{"old": "127.0.0.1:0"})); err != nil {
		t.Fatal(err)
	}
	old := bp.listeners["127.0.0.1:0"]
	if old == nil {
		t.Fatal("initial receiver was not bound")
	}
	_, port, err := net.SplitHostPort(old.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	target := net.JoinHostPort("0.0.0.0", port)
	var lc net.ListenConfig
	probe, probeErr := lc.Listen(t.Context(), "tcp", target)
	canCoexist := probeErr == nil
	if probeErr != nil {
		if !errors.Is(probeErr, syscall.EADDRINUSE) {
			t.Fatalf("kernel overlap probe could not decide coexistence: %v", probeErr)
		}
	} else {
		if err := probe.Close(); err != nil {
			t.Fatalf("close kernel overlap probe: %v", err)
		}
	}
	err = bp.applyReceiverConfig(receiverBindConfig(t, "4", map[string]string{"wildcard": target}))
	if canCoexist {
		if err != nil {
			t.Fatalf("kernel permits overlap but receiver refused replacement: %v", err)
		}
		if bp.listeners[target] == nil {
			t.Fatal("accepted wildcard replacement has no listener")
		}
		if bp.listeners["127.0.0.1:0"] != nil {
			t.Fatal("accepted wildcard replacement retained the removed listener")
		}
		receiverBindDial(t, net.JoinHostPort("127.0.0.1", port))
		return
	}
	if err == nil {
		t.Fatal("kernel refused overlap but receiver accepted replacement")
	}
	if bp.listeners["127.0.0.1:0"] != old {
		t.Fatal("overlapping replacement closed the running listener instead of refusing")
	}
	receiverBindDial(t, old.Addr().String())
}

// TestReceiverRefusalQuotesServerName requires the repair to name the actual
// list key as one command word, even when that key contains command syntax.
func TestReceiverRefusalQuotesServerName(t *testing.T) {
	for _, test := range []struct {
		name string
		word string
	}{
		{name: "blocked", word: "blocked"},
		{name: "blocked server", word: `"blocked server"`},
		{name: "|", word: `"|"`},
	} {
		t.Run(test.name, func(t *testing.T) {
			bp := receiverBindPlugin(t)
			holder := receiverBindListen(t)
			err := bp.applyReceiverConfig(receiverBindConfig(t, "4", map[string]string{test.name: holder.Addr().String()}))
			if err == nil {
				t.Fatal("receiver install accepted an occupied endpoint")
			}
			want := "fix: delete environment bmp server " + test.word
			if !strings.HasSuffix(err.Error(), want) {
				t.Fatalf("receiver refusal %q does not end with %q", err, want)
			}
		})
	}
}

func receiverBindPlugin(t *testing.T) *BMPPlugin {
	t.Helper()
	bp := &BMPPlugin{state: newBMPState(), stopCh: make(chan struct{})}
	t.Cleanup(func() {
		close(bp.stopCh)
		bp.stopListeners()
		bp.sessions.Wait()
	})
	return bp
}

func receiverBindListen(t *testing.T) net.Listener {
	t.Helper()
	var lc net.ListenConfig
	listener, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { closeLog(listener, "test-listener") })
	return listener
}

func receiverBindAvailableAddress(t *testing.T) string {
	t.Helper()
	listener := receiverBindListen(t)
	address := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	return address
}

// receiverBindOrderedConflict holds the lexically later endpoint and releases
// the earlier one, forcing the sorted installer to stage a socket before it
// reaches the failing bind.
func receiverBindOrderedConflict(t *testing.T) (net.Listener, string) {
	t.Helper()
	first := receiverBindListen(t)
	second := receiverBindListen(t)
	if first.Addr().String() > second.Addr().String() {
		first, second = second, first
	}
	available := first.Addr().String()
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	return second, available
}

func receiverBindConfig(t *testing.T, cap string, addresses map[string]string) *receiverConfig {
	t.Helper()
	cfg := &receiverConfig{Enabled: yangTrue, MaxSessions: cap, Servers: make(map[string]listenerConfig, len(addresses))}
	for name, address := range addresses {
		ip, port, err := net.SplitHostPort(address)
		if err != nil {
			t.Fatal(err)
		}
		cfg.Servers[name] = listenerConfig{IP: ip, Port: port}
	}
	return cfg
}

func receiverBindDial(t *testing.T, address string) net.Conn {
	t.Helper()
	dialer := net.Dialer{Timeout: time.Second}
	conn, err := dialer.DialContext(t.Context(), "tcp", address)
	if err != nil {
		t.Fatalf("receiver %s does not accept: %v", address, err)
	}
	t.Cleanup(func() { closeLog(conn, "test-router") })
	return conn
}

// receiverBindSendName waits for a parsed message on this exact session, so
// TCP send buffering cannot make a closed session look alive.
func receiverBindSendName(t *testing.T, bp *BMPPlugin, conn net.Conn, name string) {
	t.Helper()
	var wire [256]byte
	n := writeInitiation(wire[:], 0, &Initiation{TLVs: []TLV{
		makeStringTLV(InitTLVSysName, name),
		makeStringTLV(InitTLVSysDescr, "receiver bind test"),
	}})
	if err := conn.SetWriteDeadline(time.Now().Add(3 * time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Write(wire[:n]); err != nil {
		t.Fatal(err)
	}
	deadline := time.NewTimer(3 * time.Second)
	defer deadline.Stop()
	tick := time.NewTicker(time.Millisecond)
	defer tick.Stop()
	for {
		bp.state.mu.RLock()
		router := bp.state.routers[conn.LocalAddr().String()]
		seen := router != nil && router.SysName == name
		bp.state.mu.RUnlock()
		if seen {
			return
		}
		select {
		case <-tick.C:
		case <-deadline.C:
			t.Fatalf("session %s did not receive identity %q", conn.LocalAddr(), name)
		}
	}
}
