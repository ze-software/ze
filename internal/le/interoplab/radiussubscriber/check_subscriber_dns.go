// Design: docs/architecture/testing/interop.md -- the FreeRADIUS subscriber suite, radius-subscriber-dns
// Related: checkers.go -- waitStart, the server's own record of the session
// RFC 2548 Sections 2.7.6 and 2.7.7, RFC 1877 Section 1.1 -- see rfc/short/rfc2548.md.
package radiussubscriber

import (
	"context"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
)

const (
	// lacPPPOptions is where xl2tpd's pppd reads the LAC's options.
	lacPPPOptions = "/etc/ppp/options.l2tpd.client"
	// lacResolvConf is pppd's PPP_PATH_RESOLV (pppd/pathnames.h, ppp 2.5.5):
	// create_resolv in pppd/ipcp.c writes one "nameserver" line per resolver
	// IPCP acknowledged, when usepeerdns is set.
	lacResolvConf = "/etc/ppp/resolv.conf"
	// catCommand reads a peer file whole.
	catCommand = "cat"
)

// The resolvers the scenario's users file names, and the pool's own, which
// MUST lose to them under the pool's built-in list (radius first).
var (
	subscriberDNSWant   = []string{"nameserver 198.51.100.53", "nameserver 198.51.100.54"}
	subscriberDNSRefuse = "203.0.113.53"
)

// configureSubscriberDNSPeers gives this scenario its own users file, whose
// Access-Accept carries MS-Primary-DNS-Server and MS-Secondary-DNS-Server
// (FreeRADIUS dictionary.microsoft, attributes 28 and 29; rlm_files'
// file_common moves a users entry's reply items into the reply), and its own
// pppd options, which add usepeerdns so pppd asks IPCP for both resolvers.
func configureSubscriberDNSPeers(source interoplab.ScenarioSource, server, lac *interoplab.PeerConfig) {
	overrideMount(server, serverUsers, filepath.Join(source.Directory, "users"))
	overrideMount(lac, lacPPPOptions, filepath.Join(source.Directory, "ppp-options"))
}

// overrideMount points the mount at target to source.
func overrideMount(peer *interoplab.PeerConfig, target, source string) {
	for i := range peer.Mounts {
		if peer.Mounts[i].Target == target {
			peer.Mounts[i].Source = source
		}
	}
}

// checkSubscriberDNS proves spec-home-cpe-23b AC-9 on the wire: FreeRADIUS
// answers alice's Access-Request with the two MS DNS attributes, ze's pool
// (no dns-source, so radius, configured, received) offers them in its IPCP
// Configure-Nak, and pppd, which asked with 0.0.0.0, writes both to its
// resolv.conf in order. The pool's dns-primary MUST NOT appear.
// MUTATION: empty candidates[ClassRadius] in selectSubscriberDNS
// (internal/component/l2tp/plugins/pool/dns_source.go); pppd then writes the
// pool's 203.0.113.53 and this checker fails.
func checkSubscriberDNS(ctx context.Context, lab *scenarioLab) error {
	if _, err := lab.waitStart(ctx, ""); err != nil {
		return err
	}
	// RFC 1877 Section 1.1: "If local peer requests an invalid
	// server address (which it will typically do intentionally) the
	// remote peer specifies the address by NAKing this option, and
	// returning the IP address of a valid DNS server."
	_, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout: lab.timeout, Interval: time.Second, Description: "pppd's resolv.conf naming the RADIUS resolvers",
	}, func(probeCtx context.Context) (string, error) {
		return lab.lab.Query(probeCtx, peerLAC, []string{catCommand, lacResolvConf}, nil)
	}, func(text string) bool {
		var lines []string
		for line := range strings.SplitSeq(text, "\n") {
			if line = strings.TrimSpace(line); line != "" {
				lines = append(lines, line)
			}
		}
		if strings.Contains(text, subscriberDNSRefuse) {
			return false
		}
		return slices.Equal(lines, subscriberDNSWant)
	})
	if err != nil {
		return err
	}
	return lab.subscriberPing(ctx)
}
