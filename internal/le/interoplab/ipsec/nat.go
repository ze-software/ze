// Design: docs/architecture/testing/interop.md -- the address-translating middlebox of the IPsec lab.
// Detail: ipsec.go -- the conditional peer this file configures.
// Related: test/interop-ipsec/Dockerfile.nat -- the image the rules are installed in.
// RFC: rfc/short/rfc7296.md -- transport mode NAT traversal (Section 2.23.1)
package ipsec

import (
	"fmt"
	"net/netip"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// natConfigName is the scenario file that turns the NAT box on, exactly as
// swanctl.conf turns strongSwan on and frr.conf turns FRR on.
const natConfigName = "nat.conf"

// natTranslationsMax bounds a scenario's nat.conf. The lab has three peers and only
// two of them can sit behind a translation, so a longer file is a mistake rather
// than a topology.
const natTranslationsMax = 2

// natTranslation is one address pair: the address a peer's own stack holds, and the
// address the OTHER peer sees it at.
//
// RFC 7296 Section 2.23.1 names them IP1 and IPN1 for the client, IP2 and IPN2 for
// the server, and the whole section exists because the two differ.
type natTranslation struct {
	real   netip.Addr
	public netip.Addr
}

// readNATConfig reads a scenario's nat.conf.
//
// The format is one translation per line, `<real> <public>`, with `#` comments and
// blank lines ignored. It is declarative on purpose: a scenario directory carries
// inputs, and every assertion about them is a typed Go checker
// (docs/architecture/testing/interop.md).
func readNATConfig(directory string) ([]natTranslation, error) {
	content, err := readFileUnder(directory, natConfigName)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", natConfigName, err)
	}
	translations := make([]natTranslation, 0, natTranslationsMax)
	for lineNumber, raw := range strings.Split(string(content), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 2 {
			return nil, fmt.Errorf("%s line %d is not `<real> <public>`: %q", natConfigName, lineNumber+1, line)
		}
		real, realErr := netip.ParseAddr(fields[0])
		if realErr != nil {
			return nil, fmt.Errorf("%s line %d: %w", natConfigName, lineNumber+1, realErr)
		}
		public, publicErr := netip.ParseAddr(fields[1])
		if publicErr != nil {
			return nil, fmt.Errorf("%s line %d: %w", natConfigName, lineNumber+1, publicErr)
		}
		if !networkPrefix.Contains(real) || !networkPrefix.Contains(public) {
			return nil, fmt.Errorf("%s line %d: %s and %s must both sit inside the lab network %s",
				natConfigName, lineNumber+1, real, public, networkPrefix)
		}
		if real == public {
			return nil, fmt.Errorf("%s line %d translates %s to itself, so nothing is translated and the scenario would prove nothing",
				natConfigName, lineNumber+1, real)
		}
		translations = append(translations, natTranslation{real: real, public: public})
		if len(translations) > natTranslationsMax {
			return nil, fmt.Errorf("%s declares more than %d translations", natConfigName, natTranslationsMax)
		}
	}
	if len(translations) == 0 {
		return nil, fmt.Errorf("%s declares no translation", natConfigName)
	}
	return translations, nil
}

// natSetupScript builds the shell the NAT container runs as its whole life.
//
// Each translation gets a SECONDARY ADDRESS on eth0 and one rule in each direction.
// The secondary address is what removes every route from the peers: both public
// addresses sit inside the lab bridge's own prefix, so the other peer resolves them
// by ARP and this container answers. A design that put the NAT on its own segment
// would need a second Docker network, and interoplab.ScenarioPlan carries one that
// the BGP suite shares.
//
// DNAT runs in PREROUTING and SNAT in POSTROUTING, so one datagram crossing this box
// has both its addresses rewritten: this is the two-NAT figure of RFC 7296 Section
// 2.23.1 drawn with one box, and it is what makes all four NAT_DETECTION comparisons
// mismatch. Return traffic is reversed by conntrack, so no rule states it.
//
// The rules carry no port and no protocol. IKE floats from UDP 500 to UDP 4500 mid
// exchange (RFC 7296 Section 2.23), and a port-scoped rule would translate the
// handshake and drop the ESP that follows.
//
// It never returns: the container is infrastructure, and the scenario's verdict is
// read from the two daemons.
func natSetupScript(translations []natTranslation) string {
	var script textbuf.Buffer
	script.Str("set -e\n")
	// The box forwards between two addresses on ONE interface, so the packet leaves
	// by the interface it arrived on. Linux answers that with an ICMP redirect, which
	// would teach a peer to bypass the very translation under test.
	script.Str("sysctl -w net.ipv4.ip_forward=1\n")
	script.Str("sysctl -w net.ipv4.conf.all.send_redirects=0\n")
	script.Str("sysctl -w net.ipv4.conf.all.accept_redirects=0\n")
	for _, translation := range translations {
		script.Str("ip addr add ").Str(translation.public.String()).Byte('/').
			Int(int64(networkPrefix.Bits())).Str(" dev eth0\n")
	}
	for _, translation := range translations {
		script.Str("iptables -t nat -A PREROUTING -d ").Str(translation.public.String()).
			Str(" -j DNAT --to-destination ").Str(translation.real.String()).Byte('\n')
		script.Str("iptables -t nat -A POSTROUTING -s ").Str(translation.real.String()).
			Str(" -j SNAT --to-source ").Str(translation.public.String()).Byte('\n')
	}
	script.Str("while true; do sleep 3600; done\n")
	return script.String()
}
