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
	pc, err := net.ListenPacket("udp", net.JoinHostPort("127.0.0.1", strconv.Itoa(int(port))))
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
