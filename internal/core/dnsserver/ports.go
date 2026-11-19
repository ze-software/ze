// Design: docs/architecture/dns/secure-transports.md -- the RFC 7858 port rules for every DNS listener and client
// Related: secure.go -- ParseSecureLeaves, which refuses DNS over TLS on port 53
// RFC: rfc/short/rfc7858.md -- RFC7858-3.1-3, RFC7858-3.1-6, RFC7858-3.1-7, RFC7858-3.1-8

package dnsserver

import (
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// CleartextDNSPort is the IANA "domain" port that cleartext DNS uses over UDP
// and TCP (RFC 1035 Section 4.2).
const CleartextDNSPort uint16 = 53

// refuseDoTOnCleartextPort refuses a DNS over TLS listen-port of 53. svc is
// the plugin whose `service <svc> tls listen-port` leaf holds the port. It
// answers nil for every other port.
//
// RFC 7858 Section 3.1: "Such another port MUST NOT be port 53 but MAY be
// from the "first-come, first-served" port range."
func refuseDoTOnCleartextPort(svc string, port uint16) error {
	if port != CleartextDNSPort {
		return nil
	}
	return opererr.New(
		svc+": tls listen-port 53 is the cleartext DNS port, and DNS over TLS cannot listen on it",
		"RFC 7858 Section 3.1 forbids DNS over TLS on port 53: a client could not tell the TLS "+
			"listener from a cleartext one, and an attacker could downgrade it to cleartext",
		"set service "+svc+" tls listen-port 853",
	)
}

// RefuseCleartextOnDoTPort refuses a cleartext DNS endpoint on port 853, the
// IANA "domain-s" port of DNS over TLS. endpoint names the configured entry as
// the operator wrote it (`listener "lan"`, `forward server 192.0.2.1`), and fix
// is the exact config statement that moves it to another port. It answers nil
// for every other port.
//
// The caller decides which role the endpoint plays: a listener is the server
// role, a forward server the client role. The why is the same for both.
//
// RFC 7858 Section 3.1: "DNS clients and servers MUST NOT use port 853 to
// transport cleartext DNS messages."
func RefuseCleartextOnDoTPort(endpoint string, port uint16, fix string) error {
	if port != DefaultDoTPort {
		return nil
	}
	return opererr.New(
		endpoint+" port 853 is the DNS over TLS port, and Ze speaks cleartext DNS to this endpoint",
		"RFC 7858 Section 3.1 reserves port 853 for DNS over TLS and forbids cleartext DNS on it, "+
			"so an exchange there would carry an unprotected query where a TLS session is expected",
		fix,
	)
}

// RefuseCleartextOnConfiguredDoTPort refuses a cleartext listener that shares
// the port the plugin's own DNS over TLS listener binds. The DoT listener binds
// every IP of the cleartext listeners (Manager.ApplyWithSecure), so a cleartext
// listener on the DoT port would answer cleartext UDP on the port used for DNS
// over TLS. It answers nil when DoT is disabled or the ports differ.
//
// The fix moves DoT back to 853: the caller refuses a cleartext listener on 853
// first (RefuseCleartextOnDoTPort), so 853 is then free of cleartext listeners.
//
// RFC 7858 Section 3.1: "DNS clients MUST NOT send and DNS servers MUST NOT
// respond to cleartext DNS messages on any port used for DNS over TLS
// (including, for example, after a failed TLS handshake)."
func RefuseCleartextOnConfiguredDoTPort(svc, listener string, port uint16, sc SecureConfig) error {
	if !sc.DoTEnabled {
		return nil
	}
	if port != sc.DoTPort {
		return nil
	}
	var portText textbuf.Buffer
	portText.Uint(uint64(port))
	return opererr.New(
		svc+": listener \""+listener+"\" port "+portText.String()+" is also the tls listen-port, so cleartext DNS "+
			"would be answered on the DNS over TLS port",
		"RFC 7858 Section 3.1 forbids a server to answer cleartext DNS on any port used for DNS over TLS",
		"set service "+svc+" tls listen-port 853",
	)
}
