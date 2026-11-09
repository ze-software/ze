// Design: docs/architecture/testing/interop.md -- the access chain.
// Related: check_access.go -- checkAccessUp, which runs this after waitResolvers.
// Related: register_access_chain.go -- the pass-through subnet and lan-res-c.
package topology

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// commandDHCPCD is the DHCP client every dhcpcd node runs (the Knot image's
// dhcpcd 10.5.2), and option6BindSeconds bounds one bind of it, inside the
// option 6 wait's own timeout.
const (
	commandDHCPCD      = "dhcpcd"
	option6BindSeconds = "30"
)

// checkOption6PassThrough proves spec-home-cpe-23b story 1b on a real DHCP
// client: lan-res-c, reserved in the pass-through subnet, which sets no
// dns-server and no dns-source, leases its reservation from the CPE and is
// handed in option 6 exactly the two servers the CPE learned through IPCP,
// primary first: the D-5 default (configured, then received) reaching the
// received class.
//
// waitResolvers runs first, so the PPPoE client already reports the servers.
// The DHCP server learns them through the system-dns event, which can trail
// that report, and until then the received class holds the host fallback
// 192.0.2.53 alone (liveAddresses, internal/core/dnssource/record.go). So each
// attempt stops the dhcpcd the previous one left running and binds again,
// reading a fresh ACK, and the verdict is held to the pair: a server that
// sends no option 6 or only the fallback fails at the timeout, naming what
// the last lease carried.
func checkOption6PassThrough(ctx context.Context, check *interoplab.CheckContext) error {
	lan, err := check.Member(nodeLANResC, homeLANLink)
	if err != nil {
		return err
	}
	want := []string{dnsPrimary, dnsSecondary}
	var verdict error
	_, _, err = interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout: 90 * time.Second, Interval: 2 * time.Second,
		Description: "lan-res-c's lease carrying ze-cpe's IPCP DNS servers in option 6",
	}, func(probe context.Context) (dhcpcdLease, error) {
		return rebindLease(probe, check, nodeLANResC, lan.Device)
	}, func(lease dhcpcdLease) bool {
		verdict = option6Verdict(lease, resCAddress, want)
		return verdict == nil
	})
	if err != nil {
		return waitFailure("lan-res-c was never handed ze-cpe's learned resolvers in option 6", verdict, err)
	}
	return nil
}

// rebindLease stops client's dhcpcd on device, binds again and answers the
// lease the running daemon dumps. dhcpcd 10 answers --dumplease only from a
// running daemon, so the bind uses -w (return once bound, keep running), not
// the -1 one-shot checkLease uses.
func rebindLease(ctx context.Context, check *interoplab.CheckContext, client, device string) (dhcpcdLease, error) {
	// With no daemon running, -x exits non-zero, which is the state this step
	// wants; only a failure to run it at all is an error.
	if _, err := check.Lab.Exec(ctx, client, []string{commandDHCPCD, "-4", "-x", device}, nil); err != nil {
		return dhcpcdLease{}, fmt.Errorf("%s: stop dhcpcd on %s: %w", client, device, err)
	}
	if _, err := execOK(ctx, check, client, []string{commandDHCPCD, "-4", "-w", "-t", option6BindSeconds, device}); err != nil {
		return dhcpcdLease{}, fmt.Errorf("%s's dhcpcd bound no lease from the CPE: %w", client, err)
	}
	dump, err := execOK(ctx, check, client, []string{commandDHCPCD, "-4", "--dumplease", device})
	if err != nil {
		return dhcpcdLease{}, err
	}
	return parseDhcpcdLease(dump)
}

// dhcpcdLease is the part of `dhcpcd --dumplease` the option 6 clause reads.
// servers is empty when the ACK carried no option 6: dhcpcd's dhcp_env
// (src/dhcp.c) prints an option only when get_option finds it in the stored
// message, and names option 6 domain_name_servers.
type dhcpcdLease struct {
	address string
	servers []string
}

// errNoLeaseAddress says a lease dump holds no ip_address: dhcpcd prints
// nothing for an interface with no lease.
var errNoLeaseAddress = errors.New("dhcpcd's lease dump holds no ip_address")

// parseDhcpcdLease reads dhcpcd's lease dump, one name=value line per option,
// values possibly single-quoted.
func parseDhcpcdLease(output string) (dhcpcdLease, error) {
	var lease dhcpcdLease
	for line := range strings.SplitSeq(output, "\n") {
		name, value, ok := strings.Cut(strings.TrimSpace(line), "=")
		if !ok {
			continue
		}
		value = strings.Trim(value, "'\"")
		switch name {
		case "ip_address":
			lease.address = value
		case "domain_name_servers":
			lease.servers = strings.Fields(value)
		}
	}
	if lease.address == "" {
		return dhcpcdLease{}, fmt.Errorf("%w: %q", errNoLeaseAddress, strings.TrimSpace(output))
	}
	return lease, nil
}

// option6Verdict holds a lease to the reserved address and to exactly want in
// option 6, in order.
func option6Verdict(lease dhcpcdLease, address string, want []string) error {
	if lease.address != address {
		return fmt.Errorf("leased %s, want its reservation %s", lease.address, address)
	}
	if len(lease.servers) == 0 {
		return fmt.Errorf("the lease carried no option 6, want %v: the subnet's default list never reached the received class", want)
	}
	if !slices.Equal(lease.servers, want) {
		return fmt.Errorf("option 6 carried %v, want %v, the servers ze-cpe learned through IPCP, primary first", lease.servers, want)
	}
	return nil
}
