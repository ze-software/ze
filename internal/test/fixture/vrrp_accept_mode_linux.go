// Design: docs/architecture/vrrp/vrrp-first-hop-redundancy.md -- Accept_Mode dataplane enforcement
// RFC: rfc/short/rfc9568.md (VRRPv3) -- Section 6.1 Accept_Mode, Section 6.4.3 Active
// Related: routing_fixture_linux.go -- registers both fixtures below, beside the other VRRP ones
//
// The fixtures behind test/vrrp/vrrp-accept-mode.ci. They prove against a real
// kernel what the unit tests prove against the model: an Active router that is
// neither the address owner nor configured with Accept_Mode True does not accept
// packets addressed to its virtual address, and does accept them the moment the
// operator sets the leaf.
//
// The probe is a UDP datagram sent to the virtual address from this host. A
// packet addressed to a local address is delivered through the input hook, which
// is where the filter sits, so a listener bound to the virtual address either
// receives the datagram or does not. That is the RFC's own word, "accept",
// observed rather than inferred. The kernel rules are read as well, because that
// is the only way to see the Section 6.1 Neighbor Discovery carve-out on a host
// with no IPv6 peer to solicit it.

//go:build linux

package fixture

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"syscall"
	"time"

	"github.com/google/nftables"
	"github.com/google/nftables/expr"
	"github.com/vishvananda/netlink"

	"github.com/ze-software/ze/internal/core/textbuf"
)

const (
	// vrrpAcceptParent is the throwaway veth this group's macvlan hangs off, so
	// the virtual address lands there and never on the VM's own interface.
	vrrpAcceptParent     = "zeptam0"
	vrrpAcceptParentPeer = "zeptam0p"
	vrrpAcceptRealCIDR   = "198.51.100.251/24"
	// vrrpAcceptVIP is the virtual address. It is NOT a real address of the
	// parent, so this router is not the address owner and Accept_Mode decides.
	vrrpAcceptVIP = "198.51.100.1"
	// vrrpAcceptVirtualMAC is 00:00:5e:00:01:{vrid} for VRID 11.
	vrrpAcceptVirtualMAC = "00:00:5e:00:01:0b"
	// vrrpAcceptTable is the kernel table the plugin owns (acceptFilterTableName,
	// internal/plugins/vrrp/acceptfilter.go).
	vrrpAcceptTable = "ze_vrrp"
	// vrrpAcceptProbePort carries the probe datagram. Above 1024, so the probe
	// needs no privilege of its own.
	vrrpAcceptProbePort = 19999
	// vrrpAcceptProbeWait bounds the wait for a datagram a conforming router
	// MUST NOT deliver. It is generous for a loopback delivery, which lands in
	// microseconds, and it is the whole cost of one negative probe.
	vrrpAcceptProbeWait = time.Second
	// The two ICMPv6 types RFC 9568 Section 6.1 keeps out of the filter.
	vrrpAcceptNDSolicit = 135
	vrrpAcceptNDAdvert  = 136
)

func vrrpAcceptModeSetup(context.Context, []string) error {
	if err := addRoutingLink(&netlink.Veth{Name: vrrpAcceptParent, PeerName: vrrpAcceptParentPeer}); err != nil {
		return fmt.Errorf("add %s veth: %w", vrrpAcceptParent, err)
	}
	parent, err := netlink.LinkByName(vrrpAcceptParent)
	if err != nil {
		return fmt.Errorf("find %s: %w", vrrpAcceptParent, err)
	}
	if err := addRoutingAddress(parent, vrrpAcceptRealCIDR); err != nil {
		return fmt.Errorf("address %s: %w", vrrpAcceptParent, err)
	}
	if err := netlink.LinkSetUp(parent); err != nil {
		return fmt.Errorf("bring %s up: %w", vrrpAcceptParent, err)
	}
	peer, err := netlink.LinkByName(vrrpAcceptParentPeer)
	if err != nil {
		return fmt.Errorf("find %s: %w", vrrpAcceptParentPeer, err)
	}
	if err := netlink.LinkSetUp(peer); err != nil {
		return fmt.Errorf("bring %s up: %w", vrrpAcceptParentPeer, err)
	}
	return nil
}

// vrrpAcceptConfig builds the test config with the group's accept-mode set as
// asked. group false drops the vrrp block entirely, which is the teardown case.
func vrrpAcceptConfig(group, acceptMode bool) string {
	var tb textbuf.Buffer
	tb.Str("interface {\n\tbackend netlink;\n\tethernet ").Str(vrrpAcceptParent).
		Str(" {\n\t\tunit 0 {\n\t\t\tipv4 {\n\t\t\t\taddress [ ").Str(vrrpAcceptRealCIDR).Str(" ];\n")
	if group {
		accept := "false"
		if acceptMode {
			accept = "true"
		}
		tb.Str("\t\t\t\tvrrp {\n\t\t\t\t\tgroup lab {\n\t\t\t\t\t\tvrid 11;\n").
			Str("\t\t\t\t\t\tvirtual-address [ ").Str(vrrpAcceptVIP).Str(" ];\n").
			Str("\t\t\t\t\t\tpriority 200;\n").
			Str("\t\t\t\t\t\taccept-mode ").Str(accept).Str(";\n").
			Str("\t\t\t\t\t\tadvertise-interval-milliseconds 1000;\n").
			Str("\t\t\t\t\t}\n\t\t\t\t}\n")
	}
	return tb.Str("\t\t\t}\n\t\t}\n\t}\n}\n").String()
}

// vrrpAcceptReload writes a new config and signals the daemon to read it.
func vrrpAcceptReload(pid int, config string) error {
	if err := os.WriteFile("ze-bgp.conf", []byte(config), 0o600); err != nil {
		return fmt.Errorf("rewrite config: %w", err)
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("find daemon: %w", err)
	}
	if err := process.Signal(syscall.SIGHUP); err != nil {
		return fmt.Errorf("reload daemon: %w", err)
	}
	return nil
}

// vrrpAcceptRuleOrder reports what the kernel holds in the plugin's table: the
// rule index of the first drop of the virtual address, and of the accept for
// each Neighbor Discovery type. A rule that is not there is reported as -1, and
// an absent table as three of them, so a caller can tell "not yet" from "wrong".
func vrrpAcceptRuleOrder() (firstDrop, ndSolicit, ndAdvert int, err error) {
	firstDrop, ndSolicit, ndAdvert = -1, -1, -1

	connection := new(nftables.Conn)
	tables, err := connection.ListTables()
	if err != nil {
		return firstDrop, ndSolicit, ndAdvert, fmt.Errorf("list tables: %w", err)
	}
	victim := net.ParseIP(vrrpAcceptVIP).To4()
	for _, table := range tables {
		if table.Name != vrrpAcceptTable {
			continue
		}
		chains, chainErr := connection.ListChainsOfTableFamily(table.Family)
		if chainErr != nil {
			return firstDrop, ndSolicit, ndAdvert, fmt.Errorf("list chains: %w", chainErr)
		}
		for _, chain := range chains {
			if chain.Table == nil || chain.Table.Name != table.Name {
				continue
			}
			rules, ruleErr := connection.GetRules(table, chain)
			if ruleErr != nil {
				return firstDrop, ndSolicit, ndAdvert, fmt.Errorf("read rules: %w", ruleErr)
			}
			for index, rule := range rules {
				if vrrpAcceptRuleDrops(rule, victim) && firstDrop < 0 {
					firstDrop = index
				}
				if vrrpAcceptRuleAcceptsICMPv6(rule, vrrpAcceptNDSolicit) {
					ndSolicit = index
				}
				if vrrpAcceptRuleAcceptsICMPv6(rule, vrrpAcceptNDAdvert) {
					ndAdvert = index
				}
			}
		}
	}
	return firstDrop, ndSolicit, ndAdvert, nil
}

// vrrpAcceptRuleDrops reports whether a kernel rule compares against the virtual
// address and ends in a drop.
func vrrpAcceptRuleDrops(rule *nftables.Rule, victim net.IP) bool {
	matched := false
	for _, expression := range rule.Exprs {
		if comparison, ok := expression.(*expr.Cmp); ok && bytes.Equal(comparison.Data, victim) {
			matched = true
		}
		verdict, ok := expression.(*expr.Verdict)
		if ok && matched && verdict.Kind == expr.VerdictDrop {
			return true
		}
	}
	return false
}

// vrrpAcceptRuleAcceptsICMPv6 reports whether a kernel rule compares against
// both the ICMPv6 protocol number and the given message type, and accepts.
func vrrpAcceptRuleAcceptsICMPv6(rule *nftables.Rule, icmpType byte) bool {
	sawProtocol, sawType := false, false
	for _, expression := range rule.Exprs {
		if comparison, ok := expression.(*expr.Cmp); ok && len(comparison.Data) == 1 {
			if comparison.Data[0] == syscall.IPPROTO_ICMPV6 {
				sawProtocol = true
			}
			if comparison.Data[0] == icmpType {
				sawType = true
			}
		}
		verdict, ok := expression.(*expr.Verdict)
		if ok && sawProtocol && sawType && verdict.Kind == expr.VerdictAccept {
			return true
		}
	}
	return false
}

// vrrpAcceptProbe sends one UDP datagram to the virtual address and reports
// whether a listener bound to that address received it. That is the RFC's own
// word "accept": the packet is delivered locally, or it is not.
func vrrpAcceptProbe() (bool, error) {
	address := &net.UDPAddr{IP: net.ParseIP(vrrpAcceptVIP), Port: vrrpAcceptProbePort}
	listener, err := net.ListenUDP("udp4", address)
	if err != nil {
		return false, fmt.Errorf("listen on the virtual address: %w", err)
	}
	defer listener.Close() //nolint:errcheck // the probe owns this socket and returns immediately after

	sender, err := net.DialUDP("udp4", nil, address)
	if err != nil {
		return false, fmt.Errorf("dial the virtual address: %w", err)
	}
	defer sender.Close() //nolint:errcheck // the probe owns this socket and returns immediately after

	if _, err := sender.Write([]byte("vrrp-accept-mode-probe")); err != nil {
		return false, fmt.Errorf("send the probe: %w", err)
	}
	if err := listener.SetReadDeadline(time.Now().Add(vrrpAcceptProbeWait)); err != nil {
		return false, fmt.Errorf("set the probe deadline: %w", err)
	}
	buffer := make([]byte, 64)
	if _, _, err := listener.ReadFrom(buffer); err != nil {
		if errors.Is(err, os.ErrDeadlineExceeded) {
			return false, nil
		}
		return false, fmt.Errorf("read the probe: %w", err)
	}
	return true, nil
}

// vrrpAcceptWaitProbe waits for the probe outcome to settle on want. A config
// reload is asynchronous, so the first probe after a SIGHUP can still meet the
// rules the previous config left behind.
func vrrpAcceptWaitProbe(ctx context.Context, want bool) error {
	var probeErr error
	if Poll(ctx, 100, 200*time.Millisecond, func() bool {
		accepted, err := vrrpAcceptProbe()
		probeErr = err
		return err == nil && accepted == want
	}) {
		return nil
	}
	if probeErr != nil {
		return probeErr
	}
	return fmt.Errorf("the virtual address never settled on accepted=%t", want)
}

// vrrpAcceptSay prints one marker line the .ci expectations match on.
func vrrpAcceptSay(marker, detail string) {
	var tb textbuf.Buffer
	tb.Str(marker).Byte(' ').Str(detail).Byte('\n').StdOut() //nolint:errcheck // CLI output
}

func vrrpAcceptModeDriver(ctx context.Context, _ []string) error {
	pid, err := routingDaemonPID(ctx)
	if err != nil {
		return err
	}

	var macvlan netlink.Link
	var lookupErr error
	if !Poll(ctx, 100, 100*time.Millisecond, func() bool {
		macvlan, lookupErr = routingLinkByMAC(vrrpAcceptVirtualMAC)
		if lookupErr != nil || macvlan == nil {
			return false
		}
		installed, addrErr := routingIPv4Addresses(macvlan)
		lookupErr = addrErr
		return addrErr == nil && installed[vrrpAcceptVIP]
	}) {
		if lookupErr != nil && !errors.Is(lookupErr, errLinkAbsent) {
			return fmt.Errorf("wait for the Active router: %w", lookupErr)
		}
		return fmt.Errorf("virtual address %s never appeared on a macvlan carrying %s", vrrpAcceptVIP, vrrpAcceptVirtualMAC)
	}
	// RFC 9568 Section 6.4.3 requires the Active router to answer ARP for the
	// virtual address, which on Linux follows from the address being installed.
	// So the address IS present here even though this router must not accept
	// packets addressed to it, and its presence is the first thing proven.
	vrrpAcceptSay("ACTIVE-WITH-VIP", macvlan.Attrs().Name)

	// RFC 9568 Section 6.1: the two Neighbor Discovery types must be accepted
	// ahead of the address drop, because the first matching rule decides.
	var firstDrop, ndSolicit, ndAdvert int
	if !Poll(ctx, 100, 100*time.Millisecond, func() bool {
		firstDrop, ndSolicit, ndAdvert, lookupErr = vrrpAcceptRuleOrder()
		return lookupErr == nil && firstDrop >= 0
	}) {
		if lookupErr != nil {
			return fmt.Errorf("read the kernel filter: %w", lookupErr)
		}
		return fmt.Errorf("no kernel rule drops %s: a non-owner Active with accept-mode false must not accept packets addressed to it", vrrpAcceptVIP)
	}
	if ndSolicit < 0 || ndAdvert < 0 {
		return fmt.Errorf("neighbor discovery carve-out missing: solicit rule %d, advert rule %d", ndSolicit, ndAdvert)
	}
	if ndSolicit >= firstDrop || ndAdvert >= firstDrop {
		return fmt.Errorf("neighbor discovery accepted after the drop at rule %d (solicit %d, advert %d): the first matching rule decides", firstDrop, ndSolicit, ndAdvert)
	}
	vrrpAcceptSay("ND-CARVE-OUT-BEFORE-DROP", vrrpAcceptTable)

	if err := vrrpAcceptWaitProbe(ctx, false); err != nil {
		return fmt.Errorf("accept-mode false: %w", err)
	}
	vrrpAcceptSay("VIP-NOT-ACCEPTED", "accept-mode-false")

	if err := vrrpAcceptReload(pid, vrrpAcceptConfig(true, true)); err != nil {
		return err
	}
	if err := vrrpAcceptWaitProbe(ctx, true); err != nil {
		return fmt.Errorf("accept-mode true: %w", err)
	}
	vrrpAcceptSay("VIP-ACCEPTED", "accept-mode-true")

	if err := vrrpAcceptReload(pid, vrrpAcceptConfig(true, false)); err != nil {
		return err
	}
	if err := vrrpAcceptWaitProbe(ctx, false); err != nil {
		return fmt.Errorf("accept-mode false again: %w", err)
	}
	vrrpAcceptSay("VIP-NOT-ACCEPTED", "accept-mode-false-again")

	// The group goes away while it is still suppressing, so the teardown has a
	// rule to leave behind if it forgets to withdraw one.
	if err := vrrpAcceptReload(pid, vrrpAcceptConfig(false, false)); err != nil {
		return err
	}
	var vipPresent bool
	if !Poll(ctx, 200, 100*time.Millisecond, func() bool {
		firstDrop, _, _, lookupErr = vrrpAcceptRuleOrder()
		if lookupErr != nil {
			return false
		}
		vipPresent, lookupErr = routingAnyAddress(map[string]bool{vrrpAcceptVIP: true})
		return lookupErr == nil && firstDrop < 0 && !vipPresent
	}) {
		if lookupErr != nil {
			return fmt.Errorf("read the teardown state: %w", lookupErr)
		}
		return fmt.Errorf("teardown left state behind: drop rule at %d, virtual address present %t", firstDrop, vipPresent)
	}
	vrrpAcceptSay("TEARDOWN-COMPLETE", vrrpAcceptTable)

	process, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("find daemon: %w", err)
	}
	if err := process.Signal(syscall.SIGTERM); err != nil && !errors.Is(err, os.ErrProcessDone) {
		return fmt.Errorf("stop daemon: %w", err)
	}
	Poll(ctx, 100, 50*time.Millisecond, func() bool { return process.Signal(syscall.Signal(0)) != nil })
	return nil
}
