// Design: docs/architecture/testing/interop.md -- the L2TP subscriber of Ze's LAN DNS resolver.
// Related: l2tp.go -- knotPeer and the dns-resolver-l2tp-subscriber peer set.
// Related: checkers.go -- waitPeerLog, the shared Ze log probe.
package l2tp

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"slices"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
)

const (
	// peerKnot is Knot DNS, authoritative for the lab zone and the only
	// server that holds it, so a name answered through Ze's resolver was
	// forwarded there.
	peerKnot = "knot"

	// subscriberResolvConf is where pppd 2.5 writes the DNS addresses IPCP
	// negotiated when `usepeerdns` is set (pppd/pathnames.h PPP_PATH_RESOLV).
	subscriberResolvConf = "/etc/ppp/resolv.conf"
	// subscriberQueryName holds two A records in lab.example.zone.
	subscriberQueryName = "www.lab.example"

	subscriberWait = 30 * time.Second
	subscriberPoll = time.Second
)

// subscriberAddresses is the A RRset Knot serves for subscriberQueryName.
var subscriberAddresses = []netip.Addr{netip.MustParseAddr("192.0.2.10"), netip.MustParseAddr("192.0.2.11")}

// checkDNSResolverSubscriber proves AC-33 of spec-lan-dns-resolver: an
// L2TP subscriber learns Ze's resolver through IPCP and resolves through it.
//
//  1. Ze brings the PPP session up.
//  2. RFC 1877: pppd, asking with `usepeerdns`, was given the pool's
//     dns-primary, which is Ze's resolver address, and wrote it to its
//     resolv.conf. The scenario's dns-secondary is the same address, and
//     pppd writes a second nameserver line only when it differs, so this
//     step observes the primary alone.
//  3. The LAC reaches that address over its PPP interface, so the query
//     leaves from the subscriber's pool address.
//  4. The name resolves through that server to exactly Knot's RRset.
func checkDNSResolverSubscriber(ctx context.Context, lab labOperations, _ time.Duration) error {
	if err := waitPeerLog(ctx, lab, peerZe, "L2TP listener bound", 10*time.Second); err != nil {
		return fmt.Errorf("assertion 1: %w", err)
	}
	if err := waitPeerLog(ctx, lab, peerZe, "PPP session up", 60*time.Second); err != nil {
		return fmt.Errorf("assertion 1: %w", err)
	}
	resolver, err := netip.ParseAddr(localPPPAddress)
	if err != nil {
		return fmt.Errorf("assertion 2: resolver address: %w", err)
	}
	if err := waitSubscriberDNS(ctx, lab, resolver); err != nil {
		return fmt.Errorf("assertion 2: %w", err)
	}
	route, err := execOutput(ctx, lab, peerLAC, []string{"ip", "route", "get", localPPPAddress})
	if err != nil {
		return fmt.Errorf("assertion 3: %w", err)
	}
	if err := routeLeavesPPP(route); err != nil {
		return fmt.Errorf("assertion 3: %w", err)
	}
	if err := waitSubscriberAnswer(ctx, lab, resolver); err != nil {
		return fmt.Errorf("assertion 4: %w", err)
	}
	return nil
}

// waitSubscriberDNS polls until pppd has written its resolv.conf, then
// requires every server in it to be the resolver.
func waitSubscriberDNS(ctx context.Context, lab labOperations, resolver netip.Addr) error {
	deadline := time.Now().Add(subscriberWait)
	for {
		result, err := lab.Exec(ctx, peerLAC, []string{"cat", subscriberResolvConf}, nil)
		if err == nil {
			err = peerDNSIsResolver(result.Stdout, resolver)
			if err == nil {
				return nil
			}
		}
		if time.Now().After(deadline) {
			return err
		}
		if sleepErr := sleepContext(ctx, subscriberPoll); sleepErr != nil {
			return sleepErr
		}
	}
}

// waitSubscriberAnswer polls the subscriber's lookup until it answers, so
// the check starts once Knot has loaded its zone, and judges the last reply.
func waitSubscriberAnswer(ctx context.Context, lab labOperations, resolver netip.Addr) error {
	argv := []string{"nslookup", "-type=a", subscriberQueryName, resolver.String()}
	deadline := time.Now().Add(subscriberWait)
	for {
		// A failed lookup exits non-zero and still prints the reason (REFUSED,
		// SERVFAIL, a timeout), which nslookupAnswer turns into the error.
		result, execErr := lab.Exec(ctx, peerLAC, argv, nil)
		addresses, err := nslookupAnswer(result.Stdout+result.Stderr, resolver, subscriberQueryName)
		if err == nil {
			return requireAddresses(addresses, subscriberAddresses)
		}
		if execErr != nil {
			err = fmt.Errorf("%w (nslookup: %w)", err, execErr)
		}
		if time.Now().After(deadline) {
			return err
		}
		if sleepErr := sleepContext(ctx, subscriberPoll); sleepErr != nil {
			return sleepErr
		}
	}
}

// peerDNSIsResolver reads the nameserver lines pppd wrote: the first is the
// RFC 1877 Primary-DNS-Address and the second, when present, the
// Secondary-DNS-Address. Both must be the resolver, because the scenario's
// pool names no other server.
func peerDNSIsResolver(resolvConf string, resolver netip.Addr) error {
	servers := make([]netip.Addr, 0, 2)
	for line := range strings.SplitSeq(resolvConf, "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		if fields[0] != "nameserver" {
			continue
		}
		server, err := netip.ParseAddr(fields[1])
		if err != nil {
			return fmt.Errorf("%s names %q, not an address", subscriberResolvConf, fields[1])
		}
		servers = append(servers, server)
	}
	if len(servers) == 0 {
		return interoplab.WithSearchedOutput(
			fmt.Errorf("%s names no DNS server: IPCP gave the subscriber none", subscriberResolvConf), resolvConf)
	}
	for index, server := range servers {
		if server != resolver {
			return fmt.Errorf("%s DNS server %d is %s, want Ze's resolver %s", subscriberResolvConf, index+1, server, resolver)
		}
	}
	return nil
}

// routeLeavesPPP requires the LAC's route to the resolver to use its PPP
// interface, so the lookup is the subscriber's and not the LAC's lab address.
func routeLeavesPPP(route string) error {
	fields := strings.Fields(route)
	for index, field := range fields {
		if field != "dev" {
			continue
		}
		if index+1 == len(fields) {
			break
		}
		if strings.HasPrefix(fields[index+1], pppLinkType) {
			return nil
		}
		return fmt.Errorf("route to the resolver leaves %s, not the PPP interface: %q", fields[index+1], strings.TrimSpace(route))
	}
	return fmt.Errorf("route to the resolver names no device: %q", strings.TrimSpace(route))
}

// nslookupAnswer reads busybox nslookup's reply: the server block
// ("Server:", then "Address: <server>:53"), then one "Name:" and
// "Address:" pair per record. It refuses a reply from another server, a
// failure line ("** server can't find <name>: <RCODE>") and a reply with no
// record for name.
func nslookupAnswer(output string, server netip.Addr, name string) ([]netip.Addr, error) {
	if _, reason, failed := strings.Cut(output, "** server can't find "); failed {
		line, _, _ := strings.Cut(reason, "\n")
		// The line is "<name>: <RCODE>"; the RCODE is what the reader needs.
		if _, rcode, named := strings.Cut(line, ": "); named {
			line = rcode
		}
		return nil, fmt.Errorf("%s did not resolve %s: %s", server, name, strings.TrimSpace(line))
	}
	serverSeen := false
	recordName := ""
	addresses := make([]netip.Addr, 0, len(subscriberAddresses))
	for line := range strings.SplitSeq(output, "\n") {
		key, value, found := strings.Cut(line, ":")
		if !found {
			continue
		}
		value = strings.TrimSpace(value)
		switch strings.TrimSpace(key) {
		case "Server":
			serverSeen = true
		case "Name":
			recordName = strings.TrimSuffix(value, ".")
		case "Address":
			if recordName == "" {
				if !answeringServer(value, server) {
					return nil, fmt.Errorf("answer came from %s, want %s port 53", value, server)
				}
				continue
			}
			if recordName != name {
				continue
			}
			address, err := netip.ParseAddr(value)
			if err != nil {
				return nil, fmt.Errorf("record for %s carries %q, not an address", name, value)
			}
			addresses = append(addresses, address)
		}
	}
	if !serverSeen {
		return nil, interoplab.WithSearchedOutput(errors.New("nslookup printed no server: no reply was read"), output)
	}
	if len(addresses) == 0 {
		return nil, interoplab.WithSearchedOutput(fmt.Errorf("%s answered no address for %s", server, name), output)
	}
	return addresses, nil
}

// answeringServer reads the server block's address, which busybox prints
// with the port after a colon (10.100.0.1:53) and BIND's nslookup after a
// hash (10.100.0.1#53), and requires it to be server on port 53.
func answeringServer(printed string, server netip.Addr) bool {
	host, found := strings.CutSuffix(printed, ":53")
	if !found {
		host, found = strings.CutSuffix(printed, "#53")
	}
	if !found {
		return false
	}
	address, err := netip.ParseAddr(host)
	if err != nil {
		return false
	}
	return address == server
}

// requireAddresses holds an answer to exactly the expected RRset, in any
// order and with no record repeated or missing.
func requireAddresses(got, want []netip.Addr) error {
	sortedGot := slices.Clone(got)
	slices.SortFunc(sortedGot, netip.Addr.Compare)
	sortedWant := slices.Clone(want)
	slices.SortFunc(sortedWant, netip.Addr.Compare)
	if slices.Equal(sortedGot, sortedWant) {
		return nil
	}
	return fmt.Errorf("%s resolved to %v, want Knot's RRset %v", subscriberQueryName, sortedGot, sortedWant)
}
