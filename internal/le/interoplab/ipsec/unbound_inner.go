// Design: docs/architecture/testing/interop.md -- unbound-inner-address-uniqueness, two clients bound to no pool choosing one inner address.
// Related: virtualip.go -- the pool-bound sibling scenario, its client table and the inner-route helper.
// Related: helpers.go -- ESP direction counters, the Ze CLI query and the XFRM policy read.
// RFC: rfc/short/rfc3948.md -- Section 5.1, two SAs that lead to one inner address.
// RFC: rfc/short/rfc7296.md -- Sections 2.9 and 2.21.2, TS_UNACCEPTABLE keeps the IKE SA.
package ipsec

import (
	"context"
	"fmt"
	"strings"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// unboundInner is the inner address both clients of unbound-inner-address-uniqueness
// propose as their own selector (swanctl `local_ts = 10.1.2.3/32`).
const unboundInner = "10.1.2.3"

// innerOutPolicyTunnels reads `ip xfrm policy` and answers the tunnel endpoint (the
// template destination) of every outbound ESP policy whose destination selector is
// inner. One answer per policy, in the order the kernel lists them, so two policies
// to one inner address answer two endpoints even when they name the same one.
func innerOutPolicyTunnels(output, inner string) []string {
	var tunnels []string
	matched := false
	outbound := false
	for line := range strings.SplitSeq(output, "\n") {
		if match := srcDstPattern.FindStringSubmatch(line); match != nil {
			matched = match[2] == inner
			outbound = false
			continue
		}
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[0] == "dir" {
			outbound = fields[1] == "out"
			continue
		}
		if !matched {
			continue
		}
		if !outbound {
			continue
		}
		if len(fields) >= 5 && fields[0] == "tmpl" && fields[3] == "dst" {
			tunnels = append(tunnels, fields[4])
		}
	}
	return tunnels
}

// requireOneInnerTunnel refuses unless Ze's kernel holds exactly one outbound ESP
// policy to the inner address, and its tunnel leads to the first client's public
// address. Two policies are the RFC 3948 Section 5.1 confusion itself.
func (l *scenarioLab) requireOneInnerTunnel(ctx context.Context, stage string) error {
	output, err := l.xfrmPolicy(ctx, zePeer)
	if err != nil {
		return err
	}
	tunnels := innerOutPolicyTunnels(output, unboundInner+"/32")
	if len(tunnels) != 1 {
		return fmt.Errorf("%s: ze's kernel holds %d outbound ESP policies to %s/32 (tunnels %v), want exactly one; policies:\n%s",
			stage, len(tunnels), unboundInner, tunnels, interoplab.OutputExcerpt(output))
	}
	if tunnels[0] != swanPublicIP {
		return fmt.Errorf("%s: ze's one outbound ESP policy to %s/32 tunnels to %s, want the first client %s",
			stage, unboundInner, tunnels[0], swanPublicIP)
	}
	return nil
}

// zeChildOf answers the child-sa of the one established IKE SA of peer, and false when
// that IKE SA holds none. No established IKE SA, or two, is refused.
func zeChildOf(records []map[string]any, peer string) (map[string]any, bool, error) {
	var found []map[string]any
	for _, record := range records {
		if record["peer-name"] != peer {
			continue
		}
		if record["state"] != zeStateEstablished {
			continue
		}
		found = append(found, record)
	}
	if len(found) != 1 {
		return nil, false, fmt.Errorf("show vpn ipsec sa reports %d established IKE SAs for peer %s, want 1", len(found), peer)
	}
	child, ok := found[0]["child-sa"].(map[string]any)
	return child, ok, nil
}

// requireZeRefusedSecond refuses unless Ze reports the first client's Child SA with
// the inner address as its remote selector, and the second client's IKE SA
// established with no Child SA (RFC 7296 Section 2.21.2).
func (l *scenarioLab) requireZeRefusedSecond(ctx context.Context) error {
	records, answer, err := l.zeIKESAs(ctx)
	if err != nil {
		return err
	}
	first, held, err := zeChildOf(records, vipClients[0].peer)
	if err != nil {
		return fmt.Errorf("%w; answer: %s", err, answer)
	}
	if !held {
		return fmt.Errorf("ze reports no Child SA for peer %s, the first client; answer: %s", vipClients[0].peer, answer)
	}
	if first["ts-remote"] != unboundInner+"/32" {
		return fmt.Errorf("ze's Child SA of peer %s has ts-remote %v, want %s/32; answer: %s",
			vipClients[0].peer, first["ts-remote"], unboundInner, answer)
	}
	_, held, err = zeChildOf(records, vipClients[1].peer)
	if err != nil {
		return fmt.Errorf("%w; answer: %s", err, answer)
	}
	if held {
		return fmt.Errorf("ze holds a Child SA for peer %s, the second client to %s: two SAs lead to one inner address (RFC 3948 Section 5.1); answer: %s",
			vipClients[1].peer, unboundInner, answer)
	}
	return nil
}

// checkUnboundInnerAddressUniqueness proves RFC 3948 Section 5.1 for two strongSwan
// clients behind NAT that are bound to no pool and both propose 10.1.2.3/32 as their
// own selector.
//
// Each observation is a different event:
//  1. the first client's Child SA installs, and Ze's kernel holds one outbound policy
//     to 10.1.2.3/32, tunneled to that client;
//  2. strongSwan reports TS_UNACCEPTABLE for the second client and keeps its IKE SA
//     ESTABLISHED with no Child SA: the refusal reached the wire as RFC 7296 Section
//     2.9 names it, and Section 2.21.2's IKE SA survived it;
//  3. Ze reports the same: the first peer's Child SA, the second peer childless;
//  4. Ze's kernel still holds exactly the one policy, the first client's;
//  5. a ping from 10.1.2.3 to Ze's inner address moves ESP on all four simplex SAs of
//     the first client: its tunnel still carries traffic.
func checkUnboundInnerAddressUniqueness(ctx context.Context, lab *scenarioLab) error {
	if _, err := lab.exec(ctx, swanPeer, "ip", ipObjectAddress, "add", vipSecondClient+"/32", ipArgDev, containerInterface); err != nil {
		return fmt.Errorf("add the second client address %s to strongSwan: %w", vipSecondClient, err)
	}
	if err := lab.natInnerAddress(ctx, swanPeer, unboundInner, vipZeInner); err != nil {
		return err
	}
	if err := lab.natInnerAddress(ctx, zePeer, vipZeInner, unboundInner); err != nil {
		return err
	}
	first, second := vipClients[0], vipClients[1]
	if _, err := lab.exec(ctx, swanPeer, cmdSwanctl, "--initiate", "--child", first.child); err != nil {
		return fmt.Errorf("strongSwan could not bring up %s: %w", first.ike, err)
	}
	if _, err := lab.waitSwanIKE(ctx, first.ike); err != nil {
		return fmt.Errorf("%s: %w", first.ike, err)
	}
	if err := lab.requireOneInnerTunnel(ctx, "after "+first.ike); err != nil {
		return err
	}
	// The refused initiation exits non-zero by design, so its error is the expected
	// outcome and its log is what is judged.
	initiate, initiateErr := lab.exec(ctx, swanPeer, cmdSwanctl, "--initiate", "--child", second.child)
	// The kernel is judged first: two outbound policies to the inner address are the
	// RFC 3948 Section 5.1 confusion itself, and they are the failure an absent guard
	// shows, so that is the failure this scenario reports rather than the notify.
	if err := lab.requireOneInnerTunnel(ctx, "after "+second.ike); err != nil {
		return err
	}
	if err := requireContains(initiate, "TS_UNACCEPTABLE",
		"strongSwan's second client was not refused with TS_UNACCEPTABLE"); err != nil {
		return fmt.Errorf("%w (initiate error: %w)", err, initiateErr)
	}
	ike, err := lab.exec(ctx, swanPeer, cmdSwanctl, "--list-sas", "--ike", second.ike)
	if err != nil {
		return err
	}
	if err := requireContains(ike, "ESTABLISHED", "strongSwan did not keep the second client's IKE SA"); err != nil {
		return err
	}
	if strings.Contains(ike, "INSTALLED") {
		return fmt.Errorf("strongSwan installed a Child SA for %s although Ze refused it:\n%s", second.ike, interoplab.OutputExcerpt(ike))
	}
	if err := lab.requireZeRefusedSecond(ctx); err != nil {
		return err
	}
	return lab.verifyESPDirectionsToward(ctx, "the first client's tunnel stopped carrying traffic",
		pingProbe{peer: swanPeer, target: vipZeInner, source: unboundInner}, first.directions)
}
