package ipsec

import (
	"strings"
	"testing"
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
