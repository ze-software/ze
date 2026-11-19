package dnsserver

import (
	"errors"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/opererr"
)

// VALIDATES: a DNS over TLS listen-port of 53 is refused when the config is
// parsed, with an operator error that names the port and the fix.
// METHOD: ParseSecureLeaves over a tls container with listen-port 53, then
// read the operator error's problem and fix.
// PREVENTS: a DoT listener on the cleartext DNS port, where a client cannot
// tell TLS from cleartext and an attacker can downgrade it.
// RFC requirement: RFC7858-3.1-3 negative -- a tls listen-port of 53 is refused by ParseSecureLeaves with an operator error whose problem names listen-port 53 and whose fix is `set service test tls listen-port 853`, and the DoT port is left at its default.
func TestDoTListenPort53Refused(t *testing.T) {
	sc := DefaultSecureConfig()
	node := map[string]any{"tls": map[string]any{"enabled": "true", "listen-port": "53"}}
	err := ParseSecureLeaves(node, &sc, "test")
	if err == nil {
		t.Fatalf("tls listen-port 53 accepted (DoTPort %d), want refused", sc.DoTPort)
	}
	var oe *opererr.Error
	if !errors.As(err, &oe) {
		t.Fatalf("refusal is not an operator error: %v", err)
	}
	if !strings.Contains(oe.Problem(), "listen-port 53") {
		t.Fatalf("problem %q does not name listen-port 53", oe.Problem())
	}
	if oe.Fix() != "set service test tls listen-port 853" {
		t.Fatalf("fix = %q, want %q", oe.Fix(), "set service test tls listen-port 853")
	}
	if sc.DoTPort != DefaultDoTPort {
		t.Fatalf("refused port written: DoTPort %d, want the default %d", sc.DoTPort, DefaultDoTPort)
	}
}

// VALIDATES: the refusal is tied to port 53 alone: the ports on either side
// of it, and the default 853, parse to the port written.
// METHOD: ParseSecureLeaves over a tls container with each port in turn.
// PREVENTS: a refusal of a range (any port at or below 53, any well-known
// port) passing as the RFC 7858 rule.
// RFC requirement: RFC7858-3.1-3 positive -- a tls listen-port of 52, 54 or 853 is accepted by ParseSecureLeaves and parses to that DoT port.
func TestDoTListenPortBesides53Accepted(t *testing.T) {
	for _, tc := range []struct {
		text string
		port uint16
	}{
		{"52", 52},
		{"54", 54},
		{"853", 853},
	} {
		sc := DefaultSecureConfig()
		node := map[string]any{"tls": map[string]any{"enabled": "true", "listen-port": tc.text}}
		if err := ParseSecureLeaves(node, &sc, "test"); err != nil {
			t.Fatalf("tls listen-port %s refused: %v", tc.text, err)
		}
		if sc.DoTPort != tc.port {
			t.Fatalf("tls listen-port %s parsed to DoTPort %d, want %d", tc.text, sc.DoTPort, tc.port)
		}
	}
}
