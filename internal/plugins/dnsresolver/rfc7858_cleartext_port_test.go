package dnsresolver

import (
	"errors"
	"testing"

	"github.com/ze-software/ze/internal/core/opererr"
)

// resolverListenerConfig is a resolver section with one listener lan on port.
func resolverListenerConfig(port string) string {
	return "service {\n dns-resolver {\n  listener lan {\n   ip 127.0.0.1\n   port " + port + "\n  }\n }\n}\n"
}

// resolverForwardConfig is a resolver section with one forward server
// 192.0.2.1 on port.
func resolverForwardConfig(port string) string {
	return "service {\n dns-resolver {\n  forward {\n   server 192.0.2.1 {\n    port " + port + "\n   }\n  }\n }\n}\n"
}

// requireFix asserts err is an operator error whose fix is want.
func requireFix(t *testing.T, err error, want string) {
	t.Helper()
	var oe *opererr.Error
	if !errors.As(err, &oe) {
		t.Fatalf("refusal is not an operator error: %v", err)
	}
	if oe.Fix() != want {
		t.Fatalf("fix = %q, want %q", oe.Fix(), want)
	}
}

// VALIDATES: a resolver listener on port 853 is refused when the config is
// parsed, because the resolver answers only cleartext DNS.
// METHOD: parse a listener lan on 127.0.0.1 port 853 through the daemon's
// config delivery pipeline.
// PREVENTS: cleartext DNS served on the DNS over TLS port.
// RFC requirement: RFC7858-3.1-6 negative -- (server role) a dns-resolver listener on port 853 is refused at config parse with an operator error whose fix is `set service dns-resolver listener lan port 53`; the resolver has no DoT listener, so it would answer cleartext DNS on 853.
func TestResolverListenerOn853Refused(t *testing.T) {
	_, err := parseDelivered(t, resolverListenerConfig("853"))
	if err == nil {
		t.Fatal("listener port 853 accepted, want refused")
	}
	requireFix(t, err, "set service dns-resolver listener lan port 53")
}

// VALIDATES: the listener refusal is tied to port 853 alone: the ports on
// either side of it, and the default 53, parse to the port written.
// METHOD: parse a listener lan on each port in turn.
// PREVENTS: a refusal of a port range passing as the RFC 7858 rule.
// RFC requirement: RFC7858-3.1-6 positive -- (server role) a dns-resolver listener on port 53, 852 or 854 parses to a listener on that port.
func TestResolverListenerOffDoTPortAccepted(t *testing.T) {
	for _, tc := range []struct {
		text string
		port uint16
	}{
		{"53", 53},
		{"852", 852},
		{"854", 854},
	} {
		cfg, err := parseDelivered(t, resolverListenerConfig(tc.text))
		if err != nil {
			t.Fatalf("listener port %s refused: %v", tc.text, err)
		}
		if len(cfg.Listeners) != 1 || cfg.Listeners[0].Port != tc.port {
			t.Fatalf("listener port %s parsed to %+v, want one listener on port %d", tc.text, cfg.Listeners, tc.port)
		}
	}
}

// VALIDATES: a forward server on port 853 is refused when the config is
// parsed, because the forwarder sends only cleartext DNS (exchange in
// forward.go).
// METHOD: parse a forward server 192.0.2.1 port 853 through the daemon's
// config delivery pipeline.
// PREVENTS: Ze sending cleartext queries to an upstream's DNS over TLS port.
// RFC requirement: RFC7858-3.1-6 negative -- (client role) a dns-resolver forward server on port 853 is refused at config parse with an operator error whose fix is `set service dns-resolver forward server 192.0.2.1 port 53`.
// RFC requirement: RFC7858-3.1-7 negative -- (client role) the forwarder sends cleartext DNS only, so a forward server on 853, the port used for DNS over TLS by default, is refused at config parse rather than configured.
func TestForwardServerOn853Refused(t *testing.T) {
	_, err := parseDelivered(t, resolverForwardConfig("853"))
	if err == nil {
		t.Fatal("forward server port 853 accepted, want refused")
	}
	requireFix(t, err, "set service dns-resolver forward server 192.0.2.1 port 53")
}

// VALIDATES: the forward refusal is tied to port 853 alone: an upstream on
// either side of it, or on 53, parses to the port written.
// METHOD: parse a forward server 192.0.2.1 on each port in turn.
// PREVENTS: a refusal of a port range passing as the RFC 7858 rule.
// RFC requirement: RFC7858-3.1-6 positive -- (client role) a dns-resolver forward server on port 53, 852 or 854 parses to an upstream on that port.
// RFC requirement: RFC7858-3.1-7 positive -- (client role) a forward server on a port other than 853 (53, 852, 854) is accepted as a cleartext upstream on that port.
func TestForwardServerOffDoTPortAccepted(t *testing.T) {
	for _, tc := range []struct {
		text string
		port uint16
	}{
		{"53", 53},
		{"852", 852},
		{"854", 854},
	} {
		cfg, err := parseDelivered(t, resolverForwardConfig(tc.text))
		if err != nil {
			t.Fatalf("forward server port %s refused: %v", tc.text, err)
		}
		if len(cfg.Forward) != 1 || cfg.Forward[0].Port != tc.port {
			t.Fatalf("forward server port %s parsed to %+v, want one upstream on port %d", tc.text, cfg.Forward, tc.port)
		}
	}
}
