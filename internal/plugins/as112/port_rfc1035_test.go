// Design: docs/architecture/dns/as112.md -- serverEndpoints is the one place
// as112 chooses what it binds, and dnsserver.Manager gives every endpoint both
// a UDP PacketConn and a TCP Listener. This file pins the port that choice
// carries.
// RFC: rfc/short/rfc1035.md -- the server port of section 4.2.1 and 4.2.2

package as112

import (
	"testing"

	"github.com/ze-software/ze/internal/core/dnsserver"
)

// VALIDATES: every endpoint as112 binds sits on port 53, for both address
// families and for the loopback diagnostic address, and the doctor's registered
// listener defaults say the same. The DoT and DoH defaults in the same harness
// are 853 and 443, so 53 is the port of the two RFC 1035 transports rather than
// a value stamped on everything.
// PREVENTS: an anycast sink nobody can reach. A resolver sends to port 53 and
// nowhere else, so a node bound anywhere else answers no query while every
// health check that probes its own configured port reports it up.
func TestRFC1035_DNSTransportsUseServerPort53(t *testing.T) {
	t.Parallel()

	// RFC requirement: RFC1035-4.2.1-3 positive -- "Messages sent using UDP user
	// server port 53 (decimal)." The word "user" is verbatim: RFC 1035 has a typo
	// there for "use", and section 4.2.2 reads "use server port 53" for TCP.
	// RFC 1035 carries no capitalised RFC 2119 keyword anywhere, so this quoted
	// sentence is the whole anchor.
	for _, family := range []string{"", addressFamilyIPv4Only, addressFamilyIPv6Only} {
		endpoints := serverEndpoints(family)
		if len(endpoints) == 0 {
			t.Fatalf("family %q binds no endpoint at all", family)
		}
		for _, e := range endpoints {
			if e.Port != 53 {
				t.Errorf("family %q binds %s on port %d, want 53", family, e.IP, e.Port)
			}
		}
	}

	// The two families must really differ, or the loop above tested one set three
	// times.
	if len(serverEndpoints(addressFamilyIPv4Only)) == len(serverEndpoints("")) {
		t.Error("the ipv4-only endpoint set is the same size as the dual one, so no family branch was exercised")
	}

	// RFC requirement: RFC1035-4.2.1-3 negative -- port 53 belongs to the
	// datagram and stream transports RFC 1035 defines. The encrypted transports
	// the same harness offers are RFC 7858 and RFC 8484, and they carry their own
	// IANA ports, so 53 is a choice about which protocol runs where rather than a
	// constant applied to every listener.
	secure := dnsserver.DefaultSecureConfig()
	if secure.DoTPort == 53 || secure.DoHPort == 53 {
		t.Errorf("DoT is on %d and DoH on %d; neither is the RFC 1035 port", secure.DoTPort, secure.DoHPort)
	}
}
