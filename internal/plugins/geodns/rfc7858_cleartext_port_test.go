package geodns

import (
	"errors"
	"testing"

	"github.com/ze-software/ze/internal/core/opererr"
)

// geodnsListenerConfig is a geodns section with one listener a on 127.0.0.1
// port, and tls is the tls container's JSON members ("" for no container).
func geodnsListenerConfig(port, tls string) string {
	section := `"listener":{"a":{"ip":"127.0.0.1","port":"` + port + `"}}`
	if tls != "" {
		section += `,"tls":{` + tls + `}`
	}
	return `{"service":{"geodns":{` + section + `}}}`
}

// requireGeoDNSFix asserts err is an operator error whose fix is want.
func requireGeoDNSFix(t *testing.T, err error, want string) {
	t.Helper()
	var oe *opererr.Error
	if !errors.As(err, &oe) {
		t.Fatalf("refusal is not an operator error: %v", err)
	}
	if oe.Fix() != want {
		t.Fatalf("fix = %q, want %q", oe.Fix(), want)
	}
}

// VALIDATES: a geodns listener on port 853 is refused when the config is
// parsed, because a geodns listener answers cleartext DNS.
// METHOD: parseConfig over a listener a on 127.0.0.1 port 853.
// PREVENTS: cleartext DNS served on the DNS over TLS port.
// RFC requirement: RFC7858-3.1-6 negative -- (server role) a geodns listener on port 853 is refused at config parse with an operator error whose fix is `set service geodns listener a port 5300`.
func TestGeoDNSListenerOn853Refused(t *testing.T) {
	_, err := parseConfig(geodnsListenerConfig("853", ""))
	if err == nil {
		t.Fatal("listener port 853 accepted, want refused")
	}
	requireGeoDNSFix(t, err, "set service geodns listener a port 5300")
}

// VALIDATES: the refusal is tied to port 853 alone: a listener on either side
// of it parses to the port written.
// METHOD: parseConfig over a listener a on each port in turn.
// PREVENTS: a refusal of a port range passing as the RFC 7858 rule.
// RFC requirement: RFC7858-3.1-6 positive -- (server role) a geodns listener on port 852 or 854 parses to a listener on that port.
func TestGeoDNSListenerOffDoTPortAccepted(t *testing.T) {
	for _, tc := range []struct {
		text string
		port uint16
	}{
		{"852", 852},
		{"854", 854},
	} {
		cfg, err := parseConfig(geodnsListenerConfig(tc.text, ""))
		if err != nil {
			t.Fatalf("listener port %s refused: %v", tc.text, err)
		}
		if len(cfg.Listeners) != 1 || cfg.Listeners[0].Port != tc.port {
			t.Fatalf("listener port %s parsed to %+v, want one listener on port %d", tc.text, cfg.Listeners, tc.port)
		}
	}
}

// VALIDATES: while DoT is enabled on a port other than 853, a cleartext
// listener on that same port is refused: the DoT listener binds every
// listener IP on its port, so the listener would answer cleartext UDP on the
// port used for DNS over TLS.
// METHOD: parseConfig over tls enabled on listen-port 8853 and a listener a on
// 8853.
// PREVENTS: a geodns that answers cleartext DNS on its own DoT port.
// RFC requirement: RFC7858-3.1-8 negative -- with tls enabled on listen-port 8853, a geodns listener on port 8853 is refused at config parse with an operator error whose fix is `set service geodns tls listen-port 853`.
func TestGeoDNSListenerOnDoTListenPortRefused(t *testing.T) {
	_, err := parseConfig(geodnsListenerConfig("8853", `"enabled":"true","listen-port":"8853"`))
	if err == nil {
		t.Fatal("listener on the DoT listen-port accepted, want refused")
	}
	requireGeoDNSFix(t, err, "set service geodns tls listen-port 853")
}

// VALIDATES: the default listeners (127.0.0.1:5300 and ::1:5300), used when
// no listener is written, are checked against the DoT listen-port too.
// METHOD: parseConfig over tls enabled on listen-port 5300 with no listener.
// PREVENTS: the default list escaping the shared-port refusal.
func TestGeoDNSDefaultListenersOnDoTListenPortRefused(t *testing.T) {
	_, err := parseConfig(`{"service":{"geodns":{"tls":{"enabled":"true","listen-port":"5300"}}}}`)
	if err == nil {
		t.Fatal("default listeners on the DoT listen-port 5300 accepted, want refused")
	}
	requireGeoDNSFix(t, err, "set service geodns tls listen-port 853")
}

// VALIDATES: the shared-port refusal is tied to DoT using the port: with DoT
// disabled the same listener on 8853 parses, and with DoT enabled on 8853 a
// listener on 5300 parses.
// METHOD: parseConfig over each pair of tls container and listener port.
// PREVENTS: a refusal of any port that merely appears as a tls listen-port.
// RFC requirement: RFC7858-3.1-8 positive -- a geodns listener on port 8853 parses while tls is disabled with listen-port 8853, and a listener on port 5300 parses while tls is enabled on listen-port 8853.
func TestGeoDNSListenerOffDoTListenPortAccepted(t *testing.T) {
	for _, tc := range []struct {
		name string
		port string
		tls  string
		want uint16
	}{
		{"dot disabled", "8853", `"enabled":"false","listen-port":"8853"`, 8853},
		{"other port", "5300", `"enabled":"true","listen-port":"8853"`, 5300},
	} {
		cfg, err := parseConfig(geodnsListenerConfig(tc.port, tc.tls))
		if err != nil {
			t.Fatalf("%s: refused: %v", tc.name, err)
		}
		if len(cfg.Listeners) != 1 || cfg.Listeners[0].Port != tc.want {
			t.Fatalf("%s: parsed to %+v, want one listener on port %d", tc.name, cfg.Listeners, tc.want)
		}
	}
}
