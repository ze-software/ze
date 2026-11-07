// Design: docs/architecture/testing/interop.md -- vti-ipv6-inner-ipv4-outer, IPv6 inside an IPv4 Child SA on an xfrm interface
// Related: checkers.go -- scenarioCheckers names this checker
// Related: helpers.go -- the ESP counter, ping and selector reads it composes

package ipsec

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/interoplab"
)

const (
	// vtiIPv6Interface is the xfrm interface ze.conf binds the peer to.
	vtiIPv6Interface = "vti0"
	// vtiIPv6IfID is vti0's if_id as iproute2 prints it ("if_id %#x", ip/ipxfrm.c).
	vtiIPv6IfID = "0x7"
	// vtiIPv6IfIDValue is the same if_id as `show vpn ipsec sa | json` answers it.
	vtiIPv6IfIDValue = 7
	// vtiIPv6ZeInner is the address ze.conf puts on vti0.
	vtiIPv6ZeInner = "fd00:db8:1::1"
	// vtiIPv6SwanInner is the management address the checker puts on strongSwan.
	vtiIPv6SwanInner = "fd00:db8:1::2"
	// vtiIPv6Prefix is the management prefix both ends route into the tunnel.
	vtiIPv6Prefix = "fd00:db8:1::/112"
	// vtiIPv6Wildcard is the one selector strongSwan's narrowed answer holds.
	vtiIPv6Wildcard = "::/0"
	// vtiIPv6SwanChild is the child config swanctl.conf names.
	vtiIPv6SwanChild = "ze-child"
	// xfrmDirOut, xfrmDirIn and xfrmDirFwd are the policy directions `ip xfrm policy`
	// prints after "dir".
	xfrmDirOut = "out"
	xfrmDirIn  = "in"
	xfrmDirFwd = "fwd"
	// xfrmHeaderTarget is the word between the two addresses of a dump record's header.
	xfrmHeaderTarget = "dst"
	// vtiIPv6AFUnspec is iproute2's name for XFRM_STATE_AF_UNSPEC in a state's flag list.
	vtiIPv6AFUnspec = "af-unspec"
)

// vtiIPv6PeerSysctls are the sysctls the checker sets to vtiIPv6Enabled to enable IPv6
// on strongSwan's lab interface. The lab network is IPv4-only, and a policy-based tunnel
// decapsulates onto the interface the ESP arrived on, so ipv6_rcv would drop every
// decrypted packet on an interface whose IPv6 is disabled. Ze needs no such write: its
// decrypted packets arrive on vti0, which takes the namespace default when ze creates
// it. The profile strongSwan runs under grants each one (privilege_test.go).
var vtiIPv6PeerSysctls = []string{
	"net.ipv6.conf.all.disable_ipv6",
	"net.ipv6.conf.eth0.disable_ipv6",
}

// vtiIPv6Enabled is the disable_ipv6 value that enables IPv6 on an interface.
const vtiIPv6Enabled = "0"

// checkVTIIPv6InnerIPv4Outer proves the home CPE's management path against strongSwan:
// Ze proposes both families in one Child SA over an IPv4 outer path, strongSwan narrows
// the answer to ::/0 === ::/0, and IPv6 then crosses the IPv4 SA through vti0, before
// and after a Child SA rekey.
//
// Each step names what it proves:
//
//  1. strongSwan's installed Child SA holds ::/0 on both halves and no IPv4 selector.
//     Ze's outer path is IPv4, so its wildcard would have been 0.0.0.0/0 alone, which this
//     child config cannot accept: an established ::/0 child is proof that Ze's proposal
//     carried the IPv6 entry (AC-1, AC-2).
//  2. Ze reports the same selectors and vti0's if_id (AC-7), installs IPv6 policies only,
//     each on that if_id, and no IPv4 policy on it (AC-3).
//  3. Every ESP state Ze bound to vti0 carries af-unspec, child 20's fix, without which
//     the IPv4 state never matches an IPv6 flow (AC-6 reddens here and at the ping).
//  4. The route to strongSwan's address leaves through vti0, and a ping from vti0's
//     address completes while all four simplex SAs count bytes (AC-5). xfrmi_xmit2
//     (net/xfrm/xfrm_interface_core.c) refuses a packet whose lookup finds no xfrm
//     state, so vti0 has no clear exit and the reply cannot have bypassed the tunnel.
//  5. After strongSwan rekeys the Child SA, steps 1 to 4 hold again (AC-8).
func checkVTIIPv6InnerIPv4Outer(ctx context.Context, lab *scenarioLab) error {
	if err := lab.vtiIPv6PeerInner(ctx); err != nil {
		return err
	}
	if err := establish(ctx, lab); err != nil {
		return err
	}
	if err := lab.vtiIPv6Negotiated(ctx); err != nil {
		return err
	}
	if err := lab.vtiIPv6Carries(ctx, "IPv6 did not cross the IPv4 Child SA on vti0"); err != nil {
		return err
	}

	before, err := lab.espSPIs(ctx, swanPeer)
	if err != nil {
		return err
	}
	if len(before) == 0 {
		return errors.New("strongSwan holds no ESP SPI before the Child SA rekey")
	}
	if err := lab.swanRekeyChild(ctx, vtiIPv6SwanChild); err != nil {
		return err
	}
	_, _, err = interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout: childWaitTimeout, Interval: 2 * time.Second, Description: "new ESP SA after the Child SA rekey",
	}, func(probe context.Context) (map[string]struct{}, error) {
		return lab.espSPIs(probe, swanPeer)
	}, func(after map[string]struct{}) bool {
		return newStrings(before, after)
	})
	if err != nil {
		return err
	}
	if err := lab.vtiIPv6Negotiated(ctx); err != nil {
		return fmt.Errorf("after the Child SA rekey: %w", err)
	}
	return lab.vtiIPv6Carries(ctx, "IPv6 stopped crossing vti0 after the Child SA rekey")
}

// vtiIPv6PeerInner gives strongSwan the management address and the route that sends
// the prefix toward the tunnel. charon's ::/0 policy then takes the packet.
func (l *scenarioLab) vtiIPv6PeerInner(ctx context.Context) error {
	for _, key := range vtiIPv6PeerSysctls {
		setting := key + "=" + vtiIPv6Enabled
		if _, err := l.exec(ctx, swanPeer, "sysctl", "-w", setting); err != nil {
			return fmt.Errorf("set strongSwan's %s: %w (Docker mounts /proc/sys read-only: the scenario's "+
				"scenarioCheckers entry sets writesPeerSysctl, which runs strongSwan under %s)",
				setting, err, interoplab.NetSysctlAppArmorProfileName)
		}
	}
	if _, err := l.exec(ctx, swanPeer, "ip", "-6", ipObjectAddress, "replace", vtiIPv6SwanInner+"/128", ipArgDev, "lo"); err != nil {
		return fmt.Errorf("install inner address %s on strongSwan: %w", vtiIPv6SwanInner, err)
	}
	if _, err := l.exec(ctx, swanPeer, "ip", "-6", "route", "replace", vtiIPv6Prefix, ipArgDev, containerInterface, "src", vtiIPv6SwanInner); err != nil {
		return fmt.Errorf("install inner route to %s on strongSwan: %w", vtiIPv6Prefix, err)
	}
	return nil
}

// vtiIPv6Negotiated asserts the narrowed selectors on both ends and the kernel state Ze
// installed for them.
func (l *scenarioLab) vtiIPv6Negotiated(ctx context.Context) error {
	if err := l.waitChildSelectors(ctx, vtiIPv6Wildcard, vtiIPv6Wildcard); err != nil {
		return err
	}
	listing, err := l.listSAs(ctx)
	if err != nil {
		return err
	}
	if strings.Contains(listing, "0.0.0.0/0") {
		return fmt.Errorf("strongSwan installed an IPv4 selector its child config does not name; swanctl --list-sas:\n%s",
			interoplab.OutputExcerpt(listing))
	}
	if err := l.waitZeSelectors(ctx, vtiIPv6Wildcard, vtiIPv6Wildcard); err != nil {
		return err
	}
	records, answer, err := l.zeIKESAs(ctx)
	if err != nil {
		return err
	}
	if err := requireChildIfID(records, vtiIPv6IfIDValue); err != nil {
		return fmt.Errorf("%w; answer: %s", err, answer)
	}
	policies, err := l.xfrmPolicy(ctx, zePeer)
	if err != nil {
		return err
	}
	if err := requireIPv6PoliciesOnIfID(parseXFRMBlocks(policies), vtiIPv6IfID); err != nil {
		return fmt.Errorf("ze: %w; ip xfrm policy:\n%s", err, interoplab.OutputExcerpt(policies))
	}
	states, err := l.xfrmState(ctx, zePeer)
	if err != nil {
		return err
	}
	if err := requireAFUnspecStates(parseXFRMBlocks(states), vtiIPv6IfID); err != nil {
		return fmt.Errorf("ze: %w; ip xfrm state:\n%s", err, interoplab.OutputExcerpt(states))
	}
	return nil
}

// vtiIPv6Carries asserts the route into vti0, then one lossless ping from vti0's address
// whose round trip all four simplex SAs count.
func (l *scenarioLab) vtiIPv6Carries(ctx context.Context, message string) error {
	var tb textbuf.Buffer
	viaVTI := tb.Str("dev ").Str(vtiIPv6Interface).Byte(' ').String()
	description := tb.Reset().Str("ze route to ").Str(vtiIPv6SwanInner).Str(" through ").Str(vtiIPv6Interface).String()
	command := []string{"ip", "-6", "route", "get", vtiIPv6SwanInner}
	if _, err := l.waitOutput(ctx, zePeer, command, xfrmWaitTimeout, description, func(output string) bool {
		return strings.Contains(output+" ", viaVTI)
	}); err != nil {
		return fmt.Errorf("%s: the static route %s via %s never reached the kernel: %w", message, vtiIPv6Prefix, vtiIPv6Interface, err)
	}
	probe := pingProbe{peer: zePeer, target: vtiIPv6SwanInner, source: vtiIPv6ZeInner}
	return l.verifyESPDirectionsToward(ctx, message, probe, espBothDirections)
}

// xfrmBlock is one record of an `ip xfrm policy` or `ip xfrm state` dump: the src and dst
// of its header line, and the lines under it.
type xfrmBlock struct {
	source string
	target string
	body   []string
}

// field answers the value after key on the first body line that holds key as a word,
// and false when no line does.
func (b *xfrmBlock) field(key string) (string, bool) {
	for _, line := range b.body {
		fields := strings.Fields(line)
		for i := 0; i+1 < len(fields); i++ {
			if fields[i] == key {
				return fields[i+1], true
			}
		}
	}
	return "", false
}

// parseXFRMBlocks splits a dump into records. A record starts at a line that begins,
// unindented, with "src "; iproute2 indents every other line of the record. A header
// that does not read "src X dst Y" leaves the record's addresses empty, which no
// assertion below accepts.
func parseXFRMBlocks(output string) []xfrmBlock {
	var blocks []xfrmBlock
	for line := range strings.SplitSeq(output, "\n") {
		if strings.HasPrefix(line, "src ") {
			fields := strings.Fields(line)
			block := xfrmBlock{}
			if len(fields) >= 4 && fields[2] == xfrmHeaderTarget {
				block.source = fields[1]
				block.target = fields[3]
			}
			blocks = append(blocks, block)
			continue
		}
		if len(blocks) == 0 {
			continue
		}
		last := &blocks[len(blocks)-1]
		last.body = append(last.body, strings.TrimSpace(line))
	}
	return blocks
}

// requireIPv6PoliciesOnIfID refuses unless the policies bound to ifID are exactly one
// ::/0 === ::/0 policy for each of out, in and fwd. Linux needs fwd beside in for a
// tunnel (RequiresForwardPolicy, internal/component/ike/dataplane/xfrm_linux.go).
func requireIPv6PoliciesOnIfID(blocks []xfrmBlock, ifID string) error {
	directions := map[string]int{xfrmDirOut: 0, xfrmDirIn: 0, xfrmDirFwd: 0}
	for i := range blocks {
		block := &blocks[i]
		bound, ok := block.field("if_id")
		if !ok {
			continue
		}
		if bound != ifID {
			continue
		}
		direction, ok := block.field("dir")
		if !ok {
			return fmt.Errorf("a policy on if_id %s prints no direction", ifID)
		}
		if block.source != vtiIPv6Wildcard {
			return fmt.Errorf("a %s policy on if_id %s selects src %s dst %s; only %s was negotiated",
				direction, ifID, block.source, block.target, vtiIPv6Wildcard)
		}
		if block.target != vtiIPv6Wildcard {
			return fmt.Errorf("a %s policy on if_id %s selects src %s dst %s; only %s was negotiated",
				direction, ifID, block.source, block.target, vtiIPv6Wildcard)
		}
		if _, known := directions[direction]; !known {
			return fmt.Errorf("a policy on if_id %s has direction %q", ifID, direction)
		}
		directions[direction]++
	}
	for _, direction := range []string{xfrmDirOut, xfrmDirIn, xfrmDirFwd} {
		if directions[direction] != 1 {
			return fmt.Errorf("if_id %s holds %d %s policies for %s, want 1", ifID, directions[direction], direction, vtiIPv6Wildcard)
		}
	}
	return nil
}

// requireAFUnspecStates refuses unless at least the two states of one Child SA are bound
// to ifID, and every state bound to it carries af-unspec in its flag list.
func requireAFUnspecStates(blocks []xfrmBlock, ifID string) error {
	bound := 0
	for i := range blocks {
		block := &blocks[i]
		value, ok := block.field("if_id")
		if !ok {
			continue
		}
		if value != ifID {
			continue
		}
		bound++
		if !blockHasFlag(block, vtiIPv6AFUnspec) {
			return fmt.Errorf("the state %s -> %s on if_id %s lacks %s, so the kernel gives its selector the outer family and no IPv6 flow finds it",
				block.source, block.target, ifID, vtiIPv6AFUnspec)
		}
	}
	if bound < 2 {
		return fmt.Errorf("%d states are bound to if_id %s, want the two of the Child SA", bound, ifID)
	}
	return nil
}

// blockHasFlag reports whether a state's "flag" list names flag. iproute2 prints the
// list as the words after "flag" on the replay-window line, up to the line's end.
func blockHasFlag(block *xfrmBlock, flag string) bool {
	for _, line := range block.body {
		fields := strings.Fields(line)
		listed := false
		for _, word := range fields {
			if word == "flag" {
				listed = true
				continue
			}
			if listed && word == flag {
				return true
			}
		}
	}
	return false
}

// requireChildIfID refuses unless one Child SA of `show vpn ipsec sa | json` reports
// ifID. A JSON number decodes as float64.
func requireChildIfID(records []map[string]any, ifID float64) error {
	for _, record := range records {
		child, ok := record["child-sa"].(map[string]any)
		if !ok {
			continue
		}
		got, ok := child["if-id"].(float64)
		if !ok {
			continue
		}
		if got == ifID {
			return nil
		}
	}
	return fmt.Errorf("show vpn ipsec sa reports no child sa with if-id %v", ifID)
}
