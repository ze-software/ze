// Design: docs/architecture/dns/geodns.md -- the SRV target check at config parse
// RFC: rfc/short/rfc2782.md -- SRV records

package geodns

import (
	"strings"
	"testing"
)

// srvConfig serves t.example. with one nameserver, an SRV host
// _sip._udp.t.example. in the host set "web" pointing at target, and the
// extra hosts given, as JSON members of web's host map.
func srvConfig(target, webHosts, labHosts string) string {
	srv := `"_sip._udp.t.example.":{"type":"SRV","srv":{"priority":"10","weight":"5","port":"5060","target":"` + target + `"}}`
	if webHosts != "" {
		srv += "," + webHosts
	}
	return `{"service":{"geodns":{"enabled":"true","zone":["t.example."],"nameserver":["192.0.2.53"],` +
		`"host-set":{"web":{"host":{` + srv + `}},"lab":{"host":{` + labHosts + `}}},` +
		`"source":{"0.0.0.0/0":{"host-set":"web"}}}}}`
}

// VALIDATES: a commit is refused when an SRV record's target lies in a zone
// geodns serves and has no address record in the SRV record's own host set:
// the target is configured nowhere, only in another host set, or is itself an
// SRV host. It is accepted when the target has an A record in the same set, is
// a synthesized ns<n>.<zone> name, or lies outside every served zone (RFC 2782,
// "Target"). Method: parseConfig, the one parser the verifier and the apply
// share.
// PREVENTS: geodns publishing an SRV record whose target answers no address,
// which sends every client to a host it cannot reach.
func TestRFC2782_SRVTargetNeedsAnAddress(t *testing.T) {
	const sipA = `"sip.t.example.":{"address":["10.0.0.7"]}`
	refused := []struct {
		name string
		cfg  string
	}{
		{"target configured nowhere", srvConfig("sip.t.example.", "", "")},
		{"target only in another host set", srvConfig("sip.t.example.", "", sipA)},
		{"target is an SRV host", srvConfig("_sip._udp.t.example.", "", "")},
	}
	for _, tc := range refused {
		_, err := parseConfig(tc.cfg)
		// RFC requirement: RFC2782-Target-1 positive -- an SRV target inside a served zone with no A or AAAA record in the SRV record's host set (configured nowhere, only in another set, or itself an SRV host) is refused at config parse, naming the target and the fix.
		if err == nil || !strings.Contains(err.Error(), `srv target "`) || !strings.Contains(err.Error(), "with an address to host-set") {
			t.Errorf("%s: parseConfig error %v; want the SRV target refused with its fix", tc.name, err)
		}
	}

	accepted := []struct {
		name string
		cfg  string
	}{
		{"target with an A record in the same set", srvConfig("sip.t.example.", sipA, "")},
		{"target is the synthesized ns1 glue name", srvConfig("ns1.t.example.", "", "")},
		{"target outside every served zone", srvConfig("sip.other.example.", "", "")},
	}
	for _, tc := range accepted {
		_, err := parseConfig(tc.cfg)
		// RFC requirement: RFC2782-Target-1 negative -- an SRV target with an A record in the same host set, a synthesized ns1.<zone> glue name, or a name outside every served zone is accepted.
		if err != nil {
			t.Errorf("%s: parseConfig refused a target that has an address: %v", tc.name, err)
		}
	}
}
