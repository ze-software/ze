package ipsec

import (
	"errors"
	"net/netip"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/interoplab"
)

// vtiIPv6PolicyDump is `ip xfrm policy` in the shape iproute2 prints it for the
// scenario's narrowed Child SA: one ::/0 policy per direction on if_id 0x7, beside the
// control-plane bypass Ze installs on if_id 0, which prints no if_id line.
const vtiIPv6PolicyDump = `src ::/0 dst ::/0
	dir out priority 2000 ptype main
	tmpl src 172.28.0.2 dst 172.28.0.3
		proto esp reqid 1 mode tunnel
	if_id 0x7
src ::/0 dst ::/0
	dir fwd priority 2000 ptype main
	tmpl src 172.28.0.3 dst 172.28.0.2
		proto esp reqid 1 mode tunnel
	if_id 0x7
src ::/0 dst ::/0
	dir in priority 2000 ptype main
	tmpl src 172.28.0.3 dst 172.28.0.2
		proto esp reqid 1 mode tunnel
	if_id 0x7
src 0.0.0.0/0 dst 0.0.0.0/0 proto udp sport 500
	dir out priority 100 ptype main
`

// vtiIPv6StateDump is `ip xfrm state` for the same Child SA, both states flagged
// af-unspec as child 20 installs them.
const vtiIPv6StateDump = `src 172.28.0.2 dst 172.28.0.3
	proto esp spi 0xc1a2b3c4 reqid 1 mode tunnel
	replay-window 0 flag af-unspec
	aead rfc4106(gcm(aes)) 0x00 128
	if_id 0x7
src 172.28.0.3 dst 172.28.0.2
	proto esp spi 0xd1a2b3c4 reqid 1 mode tunnel
	replay-window 32 flag af-unspec
	aead rfc4106(gcm(aes)) 0x00 128
	if_id 0x7
`

// VALIDATES: the scenario's policy assertion accepts the narrowed set, and refuses an
// IPv4 policy on the if_id, a missing direction, and a duplicated one.
// PREVENTS: a checker that reads a dump it cannot parse as "no IPv4 policy".
func TestRequireIPv6PoliciesOnIfID(t *testing.T) {
	if err := requireIPv6PoliciesOnIfID(parseXFRMBlocks(vtiIPv6PolicyDump), vtiIPv6IfID); err != nil {
		t.Fatalf("the narrowed policy set was refused: %v", err)
	}

	ipv4 := vtiIPv6PolicyDump + "src 0.0.0.0/0 dst 0.0.0.0/0\n\tdir out priority 2000 ptype main\n\tif_id 0x7\n"
	if err := requireIPv6PoliciesOnIfID(parseXFRMBlocks(ipv4), vtiIPv6IfID); err == nil {
		t.Error("an IPv4 policy on the if_id was accepted")
	}

	withoutFwd := strings.Replace(vtiIPv6PolicyDump, "dir fwd", "dir out", 1)
	if err := requireIPv6PoliciesOnIfID(parseXFRMBlocks(withoutFwd), vtiIPv6IfID); err == nil {
		t.Error("a set with two out policies and no fwd policy was accepted")
	}

	if err := requireIPv6PoliciesOnIfID(parseXFRMBlocks(""), vtiIPv6IfID); err == nil {
		t.Error("an empty dump was accepted")
	}
}

// vtiInterfaceCatchAllDump is the final entry Ze installs at vti0's if_id since the SPD
// selection work, as iproute2 printed it in the lab: a BLOCK at the last priority for
// in, out and fwd, in both families.
const vtiInterfaceCatchAllDump = `src 0.0.0.0/0 dst 0.0.0.0/0
	dir in action block priority 4294967295
	if_id 0x7
src 0.0.0.0/0 dst 0.0.0.0/0
	dir fwd action block priority 4294967295
	if_id 0x7
src 0.0.0.0/0 dst 0.0.0.0/0
	dir out action block priority 4294967295
	if_id 0x7
src ::/0 dst ::/0
	dir in action block priority 4294967295
	if_id 0x7
src ::/0 dst ::/0
	dir fwd action block priority 4294967295
	if_id 0x7
src ::/0 dst ::/0
	dir out action block priority 4294967295
	if_id 0x7
`

// VALIDATES: the policy assertion accepts the narrowed set beside vti0's own final
// entry (BLOCK at the last priority, both families), and still refuses an IPv4 policy
// that admits traffic at the if_id, whether it prints no action at the last priority
// or a BLOCK at another priority.
// Method: the narrowed dump plus the lab's catch-all records, then each near miss.
// PREVENTS: vti-ipv6-inner-ipv4-outer refusing Ze's interface catch-all as a negotiated
// IPv4 selector, and an IPv4 allow policy hiding behind the catch-all's exemption.
func TestRequireIPv6PoliciesOnIfIDBesideTheInterfaceCatchAll(t *testing.T) {
	withCatchAll := vtiIPv6PolicyDump + vtiInterfaceCatchAllDump
	if err := requireIPv6PoliciesOnIfID(parseXFRMBlocks(withCatchAll), vtiIPv6IfID); err != nil {
		t.Fatalf("the narrowed policy set beside the interface catch-all was refused: %v", err)
	}
	if err := requireInterfaceSPDCatchAll(parseXFRMBlocks(withCatchAll), vtiIPv6IfID); err != nil {
		t.Fatalf("the fixture is not the final entry requireInterfaceSPDCatchAll accepts: %v", err)
	}

	allowAtLast := withCatchAll + "src 0.0.0.0/0 dst 0.0.0.0/0\n\tdir out priority 4294967295\n\tif_id 0x7\n"
	if err := requireIPv6PoliciesOnIfID(parseXFRMBlocks(allowAtLast), vtiIPv6IfID); err == nil {
		t.Error("an IPv4 allow policy at the last priority on the if_id was accepted")
	}

	blockElsewhere := withCatchAll + "src 0.0.0.0/0 dst 0.0.0.0/0\n\tdir out action block priority 2000\n\tif_id 0x7\n"
	if err := requireIPv6PoliciesOnIfID(parseXFRMBlocks(blockElsewhere), vtiIPv6IfID); err == nil {
		t.Error("an IPv4 policy at the Child SA priority on the if_id was accepted because it blocks")
	}

	ipv4Child := withCatchAll + "src 0.0.0.0/0 dst 0.0.0.0/0\n\tdir out priority 2000 ptype main\n\tif_id 0x7\n"
	if err := requireIPv6PoliciesOnIfID(parseXFRMBlocks(ipv4Child), vtiIPv6IfID); err == nil {
		t.Error("an IPv4 Child SA policy beside the catch-all was accepted")
	}
}

// VALIDATES: leakZeWAN and leakSwanWAN are the addresses Docker gives Ze (host 2) and
// strongSwan (host 3) on leakWANLink, so the checker's own address steps restate what
// the lab network assigns rather than adding a second address.
// PREVENTS: the leak scenario's WAN addresses drifting from its lab network.
func TestLeakWANAddressesAreOnTheLabNetwork(t *testing.T) {
	for _, address := range []struct {
		value string
		host  byte
	}{{leakZeWAN, 2}, {leakSwanWAN, 3}} {
		addr := netip.MustParseAddr(address.value)
		if !leakWANLink.Contains(addr) {
			t.Errorf("%s is not on %s", addr, leakWANLink)
		}
		if addr.As16()[15] != address.host {
			t.Errorf("%s is not host %d of %s", addr, address.host, leakWANLink)
		}
	}
}

// VALIDATES: the state assertion accepts two af-unspec states on the if_id, and refuses
// a state without the flag and a dump holding fewer than two bound states.
// PREVENTS: AC-6's red walk going green because the flag check read the wrong line.
func TestRequireAFUnspecStates(t *testing.T) {
	if err := requireAFUnspecStates(parseXFRMBlocks(vtiIPv6StateDump), vtiIPv6IfID); err != nil {
		t.Fatalf("two af-unspec states were refused: %v", err)
	}

	unflagged := strings.Replace(vtiIPv6StateDump, "replay-window 32 flag af-unspec", "replay-window 32", 1)
	if err := requireAFUnspecStates(parseXFRMBlocks(unflagged), vtiIPv6IfID); err == nil {
		t.Error("a state without af-unspec was accepted")
	}

	first, _, found := strings.Cut(vtiIPv6StateDump, "src 172.28.0.3")
	if !found {
		t.Fatal("the fixture lost its second state")
	}
	if err := requireAFUnspecStates(parseXFRMBlocks(first), vtiIPv6IfID); err == nil {
		t.Error("one bound state was accepted as a Child SA")
	}
}

// VALIDATES: the if-id read finds the Child SA's number in the decoded JSON answer and
// refuses an answer whose child carries another one.
func TestRequireChildIfID(t *testing.T) {
	records, err := decodeIKESAs(`[{"child-sa": {"if-id": 7, "ts-local": "::/0", "ts-remote": "::/0"}}]`)
	if err != nil {
		t.Fatalf("decodeIKESAs: %v", err)
	}
	if err := requireChildIfID(records, vtiIPv6IfIDValue); err != nil {
		t.Errorf("if-id 7 was refused: %v", err)
	}
	if err := requireChildIfID(records, 8); err == nil {
		t.Error("if-id 7 was accepted as 8")
	}
}

// TestVTIIPv6EvidenceReads proves the evidence a lost ping prints covers the two
// places the packet can stop before it reaches the tunnel. Method: walk the read list.
// Each peer must answer its full policy dump (the if_id 0 lookup decides before vti0
// does) and /proc/net/xfrm_stat, and the list must repeat the probe's ping from vti0's
// address, whose stderr carries the EPERM a blocking policy answers.
func TestVTIIPv6EvidenceReads(t *testing.T) {
	wanted := map[string][]string{
		zePeer:   {"ip xfrm policy", "cat /proc/net/xfrm_stat", "ip -6 route show table all"},
		swanPeer: {"ip xfrm policy", "cat /proc/net/xfrm_stat", "ip -6 route show table all"},
	}
	seen := make(map[string]map[string]bool, len(wanted))
	pinged := false
	for _, read := range vtiIPv6EvidenceReads {
		line := strings.Join(read.command, " ")
		if seen[read.peer] == nil {
			seen[read.peer] = make(map[string]bool)
		}
		seen[read.peer][line] = true
		if read.peer == zePeer && read.command[0] == cmdPing && strings.Contains(line, "-I "+vtiIPv6ZeInner) {
			pinged = true
		}
	}
	for peer, lines := range wanted {
		for _, line := range lines {
			if !seen[peer][line] {
				t.Errorf("%s evidence lacks %q", peer, line)
			}
		}
	}
	if !pinged {
		t.Errorf("the evidence does not repeat the ping from %s", vtiIPv6ZeInner)
	}
}

// TestWriteVTIIPv6Read proves one read is reported whole. Method: format a read that
// printed on both streams and failed, and require the peer, the command, the exit
// code, stdout, stderr and the error in the report.
func TestWriteVTIIPv6Read(t *testing.T) {
	var tb textbuf.Buffer
	read := vtiIPv6Read{peer: zePeer, command: []string{cmdPing, "-c", "1", vtiIPv6SwanInner}}
	result := interoplab.CommandResult{
		Stdout:   "1 packets transmitted, 0 received\n",
		Stderr:   "ping: sendmsg: Operation not permitted\n",
		ExitCode: 1,
	}
	writeVTIIPv6Read(&tb, read, result, errors.New("exit status 1"))
	report := tb.String()
	for _, want := range []string{
		"== " + zePeer + ": ping -c 1 " + vtiIPv6SwanInner + " (exit 1)",
		"1 packets transmitted, 0 received",
		"stderr: ping: sendmsg: Operation not permitted",
		"error: exit status 1",
	} {
		if !strings.Contains(report, want) {
			t.Errorf("report lacks %q:\n%s", want, report)
		}
	}
}
