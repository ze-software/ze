package dnsserver

import (
	"errors"
	"net"
	"net/netip"
	"strconv"
	"strings"
	"testing"
)

// occupyUDP holds 127.0.0.1:port, so a bind of it fails with "address
// already in use", until the caller closes the answer.
func occupyUDP(t *testing.T, port uint16) net.PacketConn {
	t.Helper()
	pc, err := (&net.ListenConfig{}).ListenPacket(t.Context(), "udp", net.JoinHostPort("127.0.0.1", strconv.Itoa(int(port))))
	if err != nil {
		t.Fatalf("occupy port %d: %v", port, err)
	}
	t.Cleanup(func() { _ = pc.Close() })
	return pc
}

// VALIDATES: R-13 for the DNS listeners. A strict (reload) apply whose new
// listener cannot be bound fails with an error naming the endpoint, the
// cause and the fix, and the listener of the running set still answers. A
// later strict apply, once the port is free, binds it: the refusal left no
// stale signature behind.
// PREVENTS: a reload reported as committed while the new listener is down,
// the failure only in the log.
// METHOD: bind port a leniently (boot), hold port b, apply a+b strictly.
func TestApplyStrictRefusesUnbindableListenerAndKeepsRunning(t *testing.T) {
	mgr := New(testLogger(), echoHandler("10.0.0.1"), Options{})
	t.Cleanup(mgr.Stop)
	a, b := freePort(t), freePort(t)
	loopback := netip.MustParseAddr("127.0.0.1")
	running := []Endpoint{{IP: loopback, Port: a}}
	if err := mgr.ApplyWithSecure(true, running, SecureConfig{}, BindLenient, testLogger()); err != nil {
		t.Fatalf("boot apply: %v", err)
	}

	holder := occupyUDP(t, b)
	candidate := []Endpoint{{IP: loopback, Port: a}, {IP: loopback, Port: b}}
	err := mgr.ApplyWithSecure(true, candidate, SecureConfig{}, BindStrict, testLogger())
	if err == nil {
		t.Fatal("strict apply to an occupied port succeeded")
	}
	busy := net.JoinHostPort("127.0.0.1", strconv.Itoa(int(b)))
	for _, want := range []string{"reload refused", "cannot listen on " + busy, "running listeners are kept", "commit again"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not contain %q", err, want)
		}
	}
	resp := exchangeA(t, "udp", net.JoinHostPort("127.0.0.1", strconv.Itoa(int(a))), "x.example.")
	if len(resp.Answer) != 1 {
		t.Fatalf("running listener after the refusal: %d answers, want 1", len(resp.Answer))
	}

	if err := holder.Close(); err != nil {
		t.Fatalf("release port %d: %v", b, err)
	}
	if err := mgr.ApplyWithSecure(true, candidate, SecureConfig{}, BindStrict, testLogger()); err != nil {
		t.Fatalf("strict apply once the port is free: %v", err)
	}
	if resp := exchangeA(t, "udp", busy, "x.example."); len(resp.Answer) != 1 {
		t.Fatalf("new listener: %d answers, want 1", len(resp.Answer))
	}
}

// VALIDATES: R-13 for DoT/DoH TLS material. A strict (reload) apply naming a
// certificate the store lacks fails, naming the certificate, the cause and
// the command that lists the store, and the running cleartext listener still
// answers. The same config under the boot policy is accepted, with the
// secure listeners left out (logged).
// PREVENTS: a DoT reload reported as committed while DoT is not served.
func TestApplyStrictRefusesMissingCertificate(t *testing.T) {
	errNoEntry := errors.New("no such certificate")
	mgr := New(testLogger(), echoHandler("10.0.0.2"), Options{
		TLSMaterialResolver: func(string) ([]byte, []byte, error) { return nil, nil, errNoEntry },
	})
	t.Cleanup(mgr.Stop)
	a := freePort(t)
	plain := []Endpoint{{IP: netip.MustParseAddr("127.0.0.1"), Port: a}}
	if err := mgr.ApplyWithSecure(true, plain, SecureConfig{}, BindLenient, testLogger()); err != nil {
		t.Fatalf("boot apply: %v", err)
	}

	secure := SecureConfig{DoTEnabled: true, DoTPort: freePort(t), Certificate: "absent"}
	err := mgr.ApplyWithSecure(true, plain, secure, BindStrict, testLogger())
	if !errors.Is(err, errNoEntry) {
		t.Fatalf("strict apply with a missing certificate: %v, want it to wrap the resolver error", err)
	}
	for _, want := range []string{"reload refused", `"absent"`, "ze show pki certificates", "commit again"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not contain %q", err, want)
		}
	}
	resp := exchangeA(t, "udp", net.JoinHostPort("127.0.0.1", strconv.Itoa(int(a))), "x.example.")
	if len(resp.Answer) != 1 {
		t.Fatalf("running listener after the refusal: %d answers, want 1", len(resp.Answer))
	}

	if err := mgr.ApplyWithSecure(true, plain, secure, BindLenient, testLogger()); err != nil {
		t.Fatalf("boot policy with a missing certificate: %v, want accepted with DoT left out", err)
	}
}

// portThief occupies 127.0.0.1:port over UDP on the first down-edge Stop
// fires once armed, so the port is taken in the window between the strict
// apply's Stop and its rebind of the previous listeners.
type portThief struct {
	t      *testing.T
	port   uint16
	armed  bool
	stolen net.PacketConn
}

func (p *portThief) onListenerChange(_, _ string, up bool) {
	if up {
		return
	}
	if !p.armed {
		return
	}
	if p.stolen != nil {
		return
	}
	pc, err := (&net.ListenConfig{}).ListenPacket(p.t.Context(), "udp", net.JoinHostPort("127.0.0.1", strconv.Itoa(int(p.port))))
	if err != nil {
		p.t.Errorf("steal port %d: %v", p.port, err)
		return
	}
	p.stolen = pc
}

func (p *portThief) release() {
	if p.stolen != nil {
		_ = p.stolen.Close()
	}
}

// VALIDATES: ISSUE-5. A strict apply refused for an unbindable listener,
// whose previous listener is taken while it is down, says which previous
// listener is not serving and why, wraps ErrPreviousListenersLost, and never
// claims the running listeners are kept; the previous listener that could be
// bound again answers.
// PREVENTS: "the running listeners are kept" reported over a port another
// process took in the stop and rebind window.
// METHOD: bind a and c leniently; steal a on the first down-edge of the
// strict apply to occupied b.
func TestApplyStrictRefusalNamesAPreviousListenerItCouldNotRebind(t *testing.T) {
	thief := &portThief{t: t}
	mgr := New(testLogger(), echoHandler("10.0.0.3"), Options{OnListenerChange: thief.onListenerChange})
	t.Cleanup(mgr.Stop)
	t.Cleanup(thief.release)
	a, b, c := freePort(t), freePort(t), freePort(t)
	thief.port = a
	loopback := netip.MustParseAddr("127.0.0.1")
	running := []Endpoint{{IP: loopback, Port: a}, {IP: loopback, Port: c}}
	if err := mgr.ApplyWithSecure(true, running, SecureConfig{}, BindLenient, testLogger()); err != nil {
		t.Fatalf("boot apply: %v", err)
	}

	occupyUDP(t, b)
	thief.armed = true
	err := mgr.ApplyWithSecure(true, []Endpoint{{IP: loopback, Port: b}}, SecureConfig{}, BindStrict, testLogger())
	if !errors.Is(err, ErrPreviousListenersLost) {
		t.Fatalf("strict apply with a stolen previous port: %v, want ErrPreviousListenersLost", err)
	}
	stolen := net.JoinHostPort("127.0.0.1", strconv.Itoa(int(a)))
	busy := net.JoinHostPort("127.0.0.1", strconv.Itoa(int(b)))
	for _, want := range []string{"reload refused", "cannot listen on " + busy, "1 of the 2 running listener(s)",
		"cannot listen on " + stolen, "so they are not serving"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not contain %q", err, want)
		}
	}
	if strings.Contains(err.Error(), "running listeners are kept") {
		t.Errorf("error %q claims the running listeners are kept", err)
	}
	if strings.Contains(err.Error(), "commit the running config again") {
		t.Errorf("error %q names a commit of the running config as the fix, which has no diff", err)
	}
	resp := exchangeA(t, "udp", net.JoinHostPort("127.0.0.1", strconv.Itoa(int(c))), "x.example.")
	if len(resp.Answer) != 1 {
		t.Fatalf("previous listener bound again: %d answers, want 1", len(resp.Answer))
	}
}

// VALIDATES: ISSUE-5. A strict apply refused for an unbindable listener whose
// only previous listener cannot be bound again wraps ErrPreviousListenersLost,
// and applying the previous listener set again, once its port is free, binds
// it: the refusal left no signature that makes that apply a no-op.
// PREVENTS: DNS down after a refused reload until a restart, because the
// re-commit of the running config matched the stale signature.
func TestApplyStrictTotalRebindFailureThenReapplyBinds(t *testing.T) {
	thief := &portThief{t: t}
	mgr := New(testLogger(), echoHandler("10.0.0.4"), Options{OnListenerChange: thief.onListenerChange})
	t.Cleanup(mgr.Stop)
	t.Cleanup(thief.release)
	a, b := freePort(t), freePort(t)
	thief.port = a
	loopback := netip.MustParseAddr("127.0.0.1")
	running := []Endpoint{{IP: loopback, Port: a}}
	if err := mgr.ApplyWithSecure(true, running, SecureConfig{}, BindLenient, testLogger()); err != nil {
		t.Fatalf("boot apply: %v", err)
	}

	occupyUDP(t, b)
	thief.armed = true
	err := mgr.ApplyWithSecure(true, []Endpoint{{IP: loopback, Port: b}}, SecureConfig{}, BindStrict, testLogger())
	if !errors.Is(err, ErrPreviousListenersLost) {
		t.Fatalf("strict apply whose previous port is stolen: %v, want ErrPreviousListenersLost", err)
	}
	if !strings.Contains(err.Error(), "1 of the 1 running listener(s)") {
		t.Errorf("error %q does not count the one lost listener", err)
	}

	thief.release()
	if err := mgr.ApplyWithSecure(true, running, SecureConfig{}, BindStrict, testLogger()); err != nil {
		t.Fatalf("applying the running config again: %v", err)
	}
	resp := exchangeA(t, "udp", net.JoinHostPort("127.0.0.1", strconv.Itoa(int(a))), "x.example.")
	if len(resp.Answer) != 1 {
		t.Fatalf("running listener after the re-apply: %d answers, want 1", len(resp.Answer))
	}
}

// VALIDATES: NOTE-4. A strict apply whose DoT config and addresses are those
// of the running apply is accepted when the certificate can no longer be
// loaded, keeping the material DoT serves; a strict apply that changes the
// DoT listeners with the same unloadable certificate is refused.
// PREVENTS: a reload of host data alone refused because a certificate left
// the store, although no listener would change.
func TestApplyStrictUnchangedSecureKeepsMaterialWhenItCannotLoad(t *testing.T) {
	chain, key := resolverChain(t, "dot.example")
	errGone := errors.New("certificate removed from the store")
	var gone bool
	mgr := New(testLogger(), echoHandler("10.0.0.5"), Options{
		TLSMaterialResolver: func(string) ([]byte, []byte, error) {
			if gone {
				return nil, nil, errGone
			}
			return chain, key, nil
		},
	})
	t.Cleanup(mgr.Stop)
	plain := []Endpoint{{IP: netip.MustParseAddr("127.0.0.1"), Port: freePort(t)}}
	secure := SecureConfig{DoTEnabled: true, DoTPort: freePort(t), Certificate: "dot"}
	if err := mgr.ApplyWithSecure(true, plain, secure, BindLenient, testLogger()); err != nil {
		t.Fatalf("boot apply: %v", err)
	}
	if mgr.running.TLSConfig == nil || len(mgr.running.DoT) != 1 {
		t.Fatalf("boot apply serves DoT %v, want one listener with material", mgr.running.DoT)
	}

	gone = true
	if err := mgr.ApplyWithSecure(true, plain, secure, BindStrict, testLogger()); err != nil {
		t.Fatalf("strict apply with unchanged listeners and an unloadable certificate: %v", err)
	}
	if mgr.running.TLSConfig == nil || len(mgr.running.DoT) != 1 {
		t.Fatalf("DoT after the unchanged reload: %v, want the one listener kept", mgr.running.DoT)
	}

	moved := secure
	moved.DoTPort = freePort(t)
	err := mgr.ApplyWithSecure(true, plain, moved, BindStrict, testLogger())
	if !errors.Is(err, errGone) {
		t.Fatalf("strict apply moving DoT with an unloadable certificate: %v, want refused", err)
	}
}

// VALIDATES: review round 3 ISSUE-8. A strict apply whose DoT config and
// addresses are those of the running apply is refused when the certificate
// cannot be loaded and the running apply holds no TLS material either: a
// lenient boot whose own load failed serves no DoT listener, so there is no
// material to keep.
// PREVENTS: a host-data reload accepted with DoT configured and not served,
// logged as keeping material that nothing serves.
// METHOD: boot leniently with the certificate missing, then apply the same
// inputs strictly with it still missing.
func TestApplyStrictUnchangedSecureRefusedWhenBootLoadedNoMaterial(t *testing.T) {
	errGone := errors.New("certificate not in the store")
	mgr := New(testLogger(), echoHandler("10.0.0.6"), Options{
		TLSMaterialResolver: func(string) ([]byte, []byte, error) {
			return nil, nil, errGone
		},
	})
	t.Cleanup(mgr.Stop)
	plain := []Endpoint{{IP: netip.MustParseAddr("127.0.0.1"), Port: freePort(t)}}
	secure := SecureConfig{DoTEnabled: true, DoTPort: freePort(t), Certificate: "dot"}
	if err := mgr.ApplyWithSecure(true, plain, secure, BindLenient, testLogger()); err != nil {
		t.Fatalf("lenient boot apply with a missing certificate: %v", err)
	}
	if mgr.running.TLSConfig != nil || len(mgr.running.DoT) != 0 {
		t.Fatalf("lenient boot without material serves DoT %v, want none", mgr.running.DoT)
	}

	err := mgr.ApplyWithSecure(true, plain, secure, BindStrict, testLogger())
	if !errors.Is(err, errGone) {
		t.Fatalf("strict apply of unchanged DoT with no material running: %v, want refused", err)
	}
	if !strings.Contains(err.Error(), "reload refused") {
		t.Errorf("error %q does not say the reload was refused", err)
	}
}
