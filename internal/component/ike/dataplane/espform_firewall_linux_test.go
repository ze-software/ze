// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- one Child SA receives both ESP forms
// Related: espform_firewall_linux.go -- the analyzer, the rule decoder and the watch these tests drive

//go:build linux

package dataplane

import (
	"encoding/binary"
	"errors"
	"testing"
	"time"

	"github.com/google/nftables"
	"github.com/google/nftables/expr"
	"github.com/google/nftables/userdata"
	"github.com/mdlayher/netlink"
	linknl "github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
)

// espFWChainKey names a chain the way the kernel does: by family, table and chain name.
type espFWChainKey struct {
	family nftables.TableFamily
	table  string
	chain  string
}

// espFWRuleset is a fake espFormNFTReader holding tables, chains and rules shaped
// as the kernel returns them: each chain and rule carries its own copy of its
// table's name and family, as chainFromMsg and ruleFromMsg build them.
type espFWRuleset struct {
	tables    []*nftables.Table
	chains    []*nftables.Chain
	rules     map[espFWChainKey][]*nftables.Rule
	dumps     map[espFWChainKey]int // GetRules calls, per chain
	chainsErr error
	rulesErr  error
	// generations answers each Generation call in turn, then repeats its last value;
	// empty answers generation 1 to every call. generationErr fails every call.
	generations     []uint32
	generationReads int
	generationErr   error
}

var _ espFormNFTReader = (*espFWRuleset)(nil)

func newESPFWRuleset() *espFWRuleset {
	return &espFWRuleset{rules: map[espFWChainKey][]*nftables.Rule{}, dumps: map[espFWChainKey]int{}}
}

func (f *espFWRuleset) Generation() (uint32, error) {
	if f.generationErr != nil {
		return 0, f.generationErr
	}
	read := f.generationReads
	f.generationReads++
	if len(f.generations) == 0 {
		return 1, nil
	}
	return f.generations[min(read, len(f.generations)-1)], nil
}

func (f *espFWRuleset) ListTables() ([]*nftables.Table, error) { return f.tables, nil }

func (f *espFWRuleset) ListChains() ([]*nftables.Chain, error) {
	if f.chainsErr != nil {
		return nil, f.chainsErr
	}
	return f.chains, nil
}

func (f *espFWRuleset) GetRules(t *nftables.Table, c *nftables.Chain) ([]*nftables.Rule, error) {
	if f.rulesErr != nil {
		return nil, f.rulesErr
	}
	key := espFWChainKey{family: t.Family, table: t.Name, chain: c.Name}
	f.dumps[key]++
	return f.rules[key], nil
}

func (f *espFWRuleset) table(family nftables.TableFamily, name string) *nftables.Table {
	t := &nftables.Table{Name: name, Family: family}
	f.tables = append(f.tables, t)
	return t
}

func (f *espFWRuleset) base(t *nftables.Table, name string, hook uint32, policy nftables.ChainPolicy) *nftables.Chain {
	c := &nftables.Chain{
		Name:     name,
		Table:    &nftables.Table{Name: t.Name, Family: t.Family},
		Hooknum:  new(nftables.ChainHook(hook)),
		Priority: nftables.ChainPriorityFilter,
		Type:     nftables.ChainTypeFilter,
		Policy:   &policy,
	}
	f.chains = append(f.chains, c)
	return c
}

func (f *espFWRuleset) regular(t *nftables.Table, name string) *nftables.Chain {
	c := &nftables.Chain{Name: name, Table: &nftables.Table{Name: t.Name, Family: t.Family}}
	f.chains = append(f.chains, c)
	return c
}

// rule appends a rule carrying Ze's UserData encoding: the term name as raw bytes
// (internal/plugins/firewall/nft/backend_linux.go, addChainRules).
func (f *espFWRuleset) rule(c *nftables.Chain, handle uint64, comment string, exprs ...[]expr.Any) {
	f.ruleData(c, handle, []byte(comment), exprs...)
}

func (f *espFWRuleset) ruleData(c *nftables.Chain, handle uint64, data []byte, exprs ...[]expr.Any) {
	var all []expr.Any
	for _, part := range exprs {
		all = append(all, part...)
	}
	key := espFWChainKey{family: c.Table.Family, table: c.Table.Name, chain: c.Name}
	f.rules[key] = append(f.rules[key], &nftables.Rule{
		Table:    &nftables.Table{Name: c.Table.Name, Family: c.Table.Family},
		Chain:    &nftables.Chain{Name: c.Name},
		Handle:   handle,
		Exprs:    all,
		UserData: data,
	})
}

// The expression sequences below are the ones nft and Ze's lowering
// (internal/plugins/firewall/nft/lower_linux.go) put in the kernel.

// espFWL4Proto is `meta l4proto <op> <proto>`: a Meta load into r1, then a Cmp.
func espFWL4Proto(op expr.CmpOp, proto byte) []expr.Any {
	return []expr.Any{
		&expr.Meta{Key: expr.MetaKeyL4PROTO, Register: 1},
		&expr.Cmp{Op: op, Register: 1, Data: []byte{proto}},
	}
}

// espFWIPProto is `ip protocol <op> <proto>`: network-header byte 9, then a Cmp.
func espFWIPProto(op expr.CmpOp, proto byte) []expr.Any {
	return []expr.Any{
		&expr.Payload{DestRegister: 1, Base: expr.PayloadBaseNetworkHeader, Offset: 9, Len: 1},
		&expr.Cmp{Op: op, Register: 1, Data: []byte{proto}},
	}
}

// espFWNFProto is the `meta nfproto` guard Ze and nft put ahead of an address match in an inet table.
func espFWNFProto(nfproto byte) []expr.Any {
	return []expr.Any{
		&expr.Meta{Key: expr.MetaKeyNFPROTO, Register: 1},
		&expr.Cmp{Op: expr.CmpOpEq, Register: 1, Data: []byte{nfproto}},
	}
}

// espFWDport is a destination-port match with no protocol guard (lowerPortMatch).
func espFWDport(port uint16) []expr.Any {
	data := make([]byte, 2)
	binary.BigEndian.PutUint16(data, port)
	return []expr.Any{
		&expr.Payload{DestRegister: 1, Base: expr.PayloadBaseTransportHeader, Offset: 2, Len: 2},
		&expr.Cmp{Op: expr.CmpOpEq, Register: 1, Data: data},
	}
}

// espFWSaddr is an IPv4 source-address match (lowerAddrMatch in an ip table).
func espFWSaddr(a, b, c, d byte) []expr.Any {
	return []expr.Any{
		&expr.Payload{DestRegister: 1, Base: expr.PayloadBaseNetworkHeader, Offset: 12, Len: 4},
		&expr.Cmp{Op: expr.CmpOpEq, Register: 1, Data: []byte{a, b, c, d}},
	}
}

// espFWSaddr8 is `ip saddr <a>.0.0.0/8`: the source address, masked to its first
// octet by a bitwise, then a Cmp.
func espFWSaddr8(a byte) []expr.Any {
	return []expr.Any{
		&expr.Payload{DestRegister: 1, Base: expr.PayloadBaseNetworkHeader, Offset: 12, Len: 4},
		&expr.Bitwise{SourceRegister: 1, DestRegister: 1, Len: 4, Mask: []byte{0xff, 0, 0, 0}, Xor: []byte{0, 0, 0, 0}},
		&expr.Cmp{Op: expr.CmpOpEq, Register: 1, Data: []byte{a, 0, 0, 0}},
	}
}

// espFWIifname is `iifname "<name>"`: the input interface name, a 16-octet Meta
// load, then a Cmp against the name padded with NULs.
func espFWIifname(name string) []expr.Any {
	data := make([]byte, 16)
	copy(data, name)
	return []expr.Any{
		&expr.Meta{Key: expr.MetaKeyIIFNAME, Register: 1},
		&expr.Cmp{Op: expr.CmpOpEq, Register: 1, Data: data},
	}
}

// espFWEtherType is `ether type <op> <type>`: link-layer octets 12-13, then a Cmp.
// nft emits the 0x8100 form first for every `vlan` match.
func espFWEtherType(op expr.CmpOp, ethertype uint16) []expr.Any {
	data := make([]byte, 2)
	binary.BigEndian.PutUint16(data, ethertype)
	return []expr.Any{
		&expr.Payload{DestRegister: 1, Base: expr.PayloadBaseLLHeader, Offset: 12, Len: 2},
		&expr.Cmp{Op: op, Register: 1, Data: data},
	}
}

// espFWMetaProtocol is `meta protocol <op> <type>`: the skb's protocol, two octets,
// which nft emits ahead of an `ip protocol` match in a netdev or bridge table.
func espFWMetaProtocol(op expr.CmpOp, ethertype uint16) []expr.Any {
	data := make([]byte, 2)
	binary.BigEndian.PutUint16(data, ethertype)
	return []expr.Any{
		&expr.Meta{Key: expr.MetaKeyPROTOCOL, Register: 1},
		&expr.Cmp{Op: op, Register: 1, Data: data},
	}
}

// espFWARPSender is Ze's ARP sender match (lowerARPSenderAddressMatch): the ARP
// layout octets from network-header offset 0, then the sender address at 14.
func espFWARPSender(a, b, c, d byte) []expr.Any {
	return []expr.Any{
		&expr.Payload{DestRegister: 1, Base: expr.PayloadBaseNetworkHeader, Offset: 0, Len: 6},
		&expr.Cmp{Op: expr.CmpOpEq, Register: 1, Data: []byte{0x00, 0x01, 0x08, 0x00, 0x06, 0x04}},
		&expr.Payload{DestRegister: 1, Base: expr.PayloadBaseNetworkHeader, Offset: 14, Len: 4},
		&expr.Cmp{Op: expr.CmpOpEq, Register: 1, Data: []byte{a, b, c, d}},
	}
}

func espFWCounter() []expr.Any { return []expr.Any{&expr.Counter{}} }

func espFWVerdict(kind expr.VerdictKind) []expr.Any {
	return []expr.Any{&expr.Verdict{Kind: kind}}
}

func espFWJump(kind expr.VerdictKind, chain string) []expr.Any {
	return []expr.Any{&expr.Verdict{Kind: kind, Chain: chain}}
}

func espFWOne(e expr.Any) []expr.Any { return []expr.Any{e} }

const (
	espFWHookPrerouting = unix.NF_INET_PRE_ROUTING
	espFWHookInput      = unix.NF_INET_LOCAL_IN
	espFWHookForward    = unix.NF_INET_FORWARD
	espFWHookOutput     = unix.NF_INET_LOCAL_OUT
	espFWHookIngress    = unix.NF_NETDEV_INGRESS
)

// TestESPFormFirewallBlocks proves the analyzer's verdict for each ruleset shape
// the design names. Method: each row builds a ruleset as the kernel returns it,
// through the fake reader, and asserts whether the fast path is blocked and, when
// it is, the first site the walk reports: family, table, base chain, hook, rule
// handle with its comment (or "policy drop"), and the reason class.
func TestESPFormFirewallBlocks(t *testing.T) {
	tests := []struct {
		name    string
		build   func(f *espFWRuleset)
		blocked bool
		site    espFormFirewallSite
	}{
		{
			name: "input policy drop blocks",
			build: func(f *espFWRuleset) {
				f.base(f.table(nftables.TableFamilyIPv4, "filter"), "input", espFWHookInput, nftables.ChainPolicyDrop)
			},
			blocked: true,
			site:    espFormFirewallSite{Family: "ip", Table: "filter", Chain: "input", Hook: "input", Rule: "policy drop", Reason: espFormReasonPolicyDrop},
		},
		{
			name: "tcp-scoped drop is safe",
			build: func(f *espFWRuleset) {
				in := f.base(f.table(nftables.TableFamilyINet, "ze_filter"), "input", espFWHookInput, nftables.ChainPolicyAccept)
				f.rule(in, 4, "ssh-drop", espFWL4Proto(expr.CmpOpEq, unix.IPPROTO_TCP), espFWDport(22), espFWCounter(), espFWVerdict(expr.VerdictDrop))
			},
		},
		{
			name: "port-only drop blocks: the port payload carries no l4proto guard",
			build: func(f *espFWRuleset) {
				in := f.base(f.table(nftables.TableFamilyINet, "ze_filter"), "input", espFWHookInput, nftables.ChainPolicyAccept)
				f.rule(in, 7, "port-drop", espFWDport(22), espFWCounter(), espFWVerdict(expr.VerdictDrop))
			},
			blocked: true,
			site:    espFormFirewallSite{Family: "inet", Table: "ze_filter", Chain: "input", Hook: "input", Rule: `handle 7 "port-drop"`, Reason: espFormReasonDrop},
		},
		{
			name: "l4proto esp accept before policy drop is safe",
			build: func(f *espFWRuleset) {
				in := f.base(f.table(nftables.TableFamilyINet, "ze_filter"), "input", espFWHookInput, nftables.ChainPolicyDrop)
				f.rule(in, 3, "allow-esp", espFWL4Proto(expr.CmpOpEq, unix.IPPROTO_ESP), espFWCounter(), espFWVerdict(expr.VerdictAccept))
				f.rule(in, 5, "drop-all", espFWCounter(), espFWVerdict(expr.VerdictDrop))
			},
		},
		{
			// MUTATION: drop the protocol-load check from espFormAcceptsESP.
			name: "ip protocol esp accept in an ip table is a definite accept",
			build: func(f *espFWRuleset) {
				in := f.base(f.table(nftables.TableFamilyIPv4, "fw"), "input", espFWHookInput, nftables.ChainPolicyDrop)
				f.rule(in, 3, "", espFWIPProto(expr.CmpOpEq, unix.IPPROTO_ESP), espFWCounter(), espFWVerdict(expr.VerdictAccept))
			},
		},
		{
			// nft's inet form of `ip protocol esp accept` puts the nfproto guard first,
			// so the protocol pair is not the rule's first two expressions.
			// MUTATION: look for the definite accept's pair past the first expression.
			name: "ip protocol esp accept behind the nfproto ipv4 guard is no definite accept",
			build: func(f *espFWRuleset) {
				in := f.base(f.table(nftables.TableFamilyINet, "fw"), "input", espFWHookInput, nftables.ChainPolicyDrop)
				f.rule(in, 3, "", espFWNFProto(unix.NFPROTO_IPV4), espFWIPProto(expr.CmpOpEq, unix.IPPROTO_ESP), espFWVerdict(expr.VerdictAccept))
			},
			blocked: true,
			site:    espFormFirewallSite{Family: "inet", Table: "fw", Chain: "input", Hook: "input", Rule: "policy drop", Reason: espFormReasonPolicyDrop},
		},
		{
			// MUTATION: allow a log beside the counter in espFormAcceptsESP.
			name: "a log on the esp accept is no definite accept",
			build: func(f *espFWRuleset) {
				in := f.base(f.table(nftables.TableFamilyINet, "filter"), "input", espFWHookInput, nftables.ChainPolicyDrop)
				f.rule(in, 1, "", espFWL4Proto(expr.CmpOpEq, unix.IPPROTO_ESP), espFWOne(&expr.Log{}), espFWVerdict(expr.VerdictAccept))
			},
			blocked: true,
			site:    espFormFirewallSite{Family: "inet", Table: "filter", Chain: "input", Hook: "input", Rule: "policy drop", Reason: espFormReasonPolicyDrop},
		},
		{
			name: "an accept narrowed by more than the esp guard is no definite accept",
			build: func(f *espFWRuleset) {
				in := f.base(f.table(nftables.TableFamilyIPv4, "filter"), "input", espFWHookInput, nftables.ChainPolicyDrop)
				f.rule(in, 3, "esp-from-peer", espFWL4Proto(expr.CmpOpEq, unix.IPPROTO_ESP), espFWSaddr(192, 0, 2, 1), espFWVerdict(expr.VerdictAccept))
			},
			blocked: true,
			site:    espFormFirewallSite{Family: "ip", Table: "filter", Chain: "input", Hook: "input", Rule: "policy drop", Reason: espFormReasonPolicyDrop},
		},
		{
			name: "a jump to a chain that drops blocks at the jump",
			build: func(f *espFWRuleset) {
				tab := f.table(nftables.TableFamilyIPv4, "filter")
				in := f.base(tab, "input", espFWHookInput, nftables.ChainPolicyAccept)
				f.regular(tab, "blocklist")
				f.rule(in, 2, "to-blocklist", espFWJump(expr.VerdictJump, "blocklist"))
				f.rule(f.chains[1], 9, "bad-host", espFWSaddr(198, 51, 100, 7), espFWVerdict(expr.VerdictDrop))
			},
			blocked: true,
			site:    espFormFirewallSite{Family: "ip", Table: "filter", Chain: "input", Hook: "input", Rule: `handle 2 "to-blocklist"`, Reason: espFormReasonJump},
		},
		{
			// ip filter input, policy drop:
			//   udp dport 4500 accept; ip saddr 10.0.0.0/8 return; meta l4proto esp accept
			// The kernel sends bare ESP from 10/8 from the return to the policy, which
			// drops it, so the later esp accept must not make the chain safe.
			name: "a return that can carry esp blocks: in a base chain it skips to the policy",
			build: func(f *espFWRuleset) {
				in := f.base(f.table(nftables.TableFamilyIPv4, "filter"), "input", espFWHookInput, nftables.ChainPolicyDrop)
				f.rule(in, 1, "nat-t", espFWL4Proto(expr.CmpOpEq, unix.IPPROTO_UDP), espFWDport(4500), espFWVerdict(expr.VerdictAccept))
				f.rule(in, 2, "rfc1918", espFWSaddr8(10), espFWVerdict(expr.VerdictReturn))
				f.rule(in, 3, "esp", espFWL4Proto(expr.CmpOpEq, unix.IPPROTO_ESP), espFWVerdict(expr.VerdictAccept))
			},
			blocked: true,
			site:    espFormFirewallSite{Family: "ip", Table: "filter", Chain: "input", Hook: "input", Rule: `handle 2 "rfc1918"`, Reason: espFormReasonReturn},
		},
		{
			// jump c, where c holds `ip saddr 10.0.0.0/8 return; meta l4proto esp
			// accept`, and the caller drops after the jump. ESP from 10/8 returns to the
			// rule after the jump and is dropped, so the accept in c proves nothing.
			name: "a jump into a chain that returns blocks at the jump",
			build: func(f *espFWRuleset) {
				tab := f.table(nftables.TableFamilyIPv4, "filter")
				in := f.base(tab, "input", espFWHookInput, nftables.ChainPolicyAccept)
				c := f.regular(tab, "c")
				f.rule(in, 2, "to-c", espFWJump(expr.VerdictJump, "c"))
				f.rule(c, 5, "", espFWSaddr8(10), espFWVerdict(expr.VerdictReturn))
				f.rule(c, 6, "", espFWL4Proto(expr.CmpOpEq, unix.IPPROTO_ESP), espFWVerdict(expr.VerdictAccept))
				f.rule(in, 3, "drop-rest", espFWVerdict(expr.VerdictDrop))
			},
			blocked: true,
			site:    espFormFirewallSite{Family: "ip", Table: "filter", Chain: "input", Hook: "input", Rule: `handle 2 "to-c"`, Reason: espFormReasonJump},
		},
		{
			// Zone dispatch: iifname "wan" goto wan-zone, where wan-zone accepts only
			// udp dport 4500, then a later esp accept. ESP from wan falls off the end
			// of wan-zone; a goto from a base chain pushed nothing, so it meets the
			// policy, which drops it, and never reaches the later accept.
			name: "a narrowed goto blocks at the goto",
			build: func(f *espFWRuleset) {
				tab := f.table(nftables.TableFamilyINet, "filter")
				in := f.base(tab, "input", espFWHookInput, nftables.ChainPolicyDrop)
				zone := f.regular(tab, "wan-zone")
				f.rule(in, 2, "wan", espFWIifname("wan"), espFWJump(expr.VerdictGoto, "wan-zone"))
				f.rule(zone, 7, "", espFWL4Proto(expr.CmpOpEq, unix.IPPROTO_UDP), espFWDport(4500), espFWVerdict(expr.VerdictAccept))
				f.rule(in, 3, "esp", espFWL4Proto(expr.CmpOpEq, unix.IPPROTO_ESP), espFWVerdict(expr.VerdictAccept))
			},
			blocked: true,
			site:    espFormFirewallSite{Family: "inet", Table: "filter", Chain: "input", Hook: "input", Rule: `handle 2 "wan"`, Reason: espFormReasonGoto},
		},
		{
			name: "a tcp-guarded jump is safe",
			build: func(f *espFWRuleset) {
				tab := f.table(nftables.TableFamilyIPv4, "filter")
				in := f.base(tab, "input", espFWHookInput, nftables.ChainPolicyAccept)
				f.regular(tab, "tcp-checks")
				f.rule(in, 2, "", espFWL4Proto(expr.CmpOpEq, unix.IPPROTO_TCP), espFWJump(expr.VerdictJump, "tcp-checks"))
				f.rule(f.chains[1], 9, "", espFWVerdict(expr.VerdictDrop))
			},
		},
		{
			// The analyzer reads base chains only, so even a jump whose target
			// accepts every packet is refused: the target is never read.
			name: "an unconditional jump to a chain that accepts everything blocks at the jump",
			build: func(f *espFWRuleset) {
				tab := f.table(nftables.TableFamilyIPv4, "filter")
				in := f.base(tab, "input", espFWHookInput, nftables.ChainPolicyDrop)
				f.regular(tab, "allow")
				f.rule(in, 2, "", espFWJump(expr.VerdictJump, "allow"))
				f.rule(f.chains[1], 5, "", espFWVerdict(expr.VerdictAccept))
				f.rule(in, 3, "", espFWVerdict(expr.VerdictDrop))
			},
			blocked: true,
			site:    espFormFirewallSite{Family: "ip", Table: "filter", Chain: "input", Hook: "input", Rule: "handle 2", Reason: espFormReasonJump},
		},
		{
			name: "netdev ingress drop blocks",
			build: func(f *espFWRuleset) {
				ing := f.base(f.table(nftables.TableFamilyNetdev, "edge"), "ingress", espFWHookIngress, nftables.ChainPolicyAccept)
				f.rule(ing, 1, "", espFWVerdict(expr.VerdictDrop))
			},
			blocked: true,
			site:    espFormFirewallSite{Family: "netdev", Table: "edge", Chain: "ingress", Hook: "ingress", Rule: "handle 1", Reason: espFormReasonDrop},
		},
		{
			name: "netdev egress shares the input hook number and is ignored",
			build: func(f *espFWRuleset) {
				f.base(f.table(nftables.TableFamilyNetdev, "edge"), "egress", unix.NF_NETDEV_EGRESS, nftables.ChainPolicyDrop)
			},
		},
		{
			name: "inet ingress drop blocks",
			build: func(f *espFWRuleset) {
				ing := f.base(f.table(nftables.TableFamilyINet, "edge"), "ingress", espFormHookInetIngress, nftables.ChainPolicyAccept)
				f.rule(ing, 1, "", espFWVerdict(expr.VerdictDrop))
			},
			blocked: true,
			site:    espFormFirewallSite{Family: "inet", Table: "edge", Chain: "ingress", Hook: "ingress", Rule: "handle 1", Reason: espFormReasonDrop},
		},
		{
			name: "bridge input policy drop blocks",
			build: func(f *espFWRuleset) {
				f.base(f.table(nftables.TableFamilyBridge, "br"), "input", espFormHookBridgeInput, nftables.ChainPolicyDrop)
			},
			blocked: true,
			site:    espFormFirewallSite{Family: "bridge", Table: "br", Chain: "input", Hook: "input", Rule: "policy drop", Reason: espFormReasonPolicyDrop},
		},
		{
			name: "ip6 family, forward and output hooks are ignored",
			build: func(f *espFWRuleset) {
				f.base(f.table(nftables.TableFamilyIPv6, "filter6"), "input", espFWHookInput, nftables.ChainPolicyDrop)
				tab := f.table(nftables.TableFamilyINet, "filter")
				f.base(tab, "forward", espFWHookForward, nftables.ChainPolicyDrop)
				out := f.base(tab, "output", espFWHookOutput, nftables.ChainPolicyAccept)
				f.rule(out, 1, "", espFWVerdict(expr.VerdictDrop))
			},
		},
		{
			name: "unguarded prerouting DNAT blocks",
			build: func(f *espFWRuleset) {
				pre := f.base(f.table(nftables.TableFamilyIPv4, "nat"), "prerouting", espFWHookPrerouting, nftables.ChainPolicyAccept)
				f.rule(pre, 6, "dnat-web", espFWDport(8080),
					espFWOne(&expr.Immediate{Register: 1, Data: []byte{10, 0, 0, 1}}),
					espFWOne(&expr.NAT{Type: expr.NATTypeDestNAT, Family: unix.NFPROTO_IPV4, RegAddrMin: 1}))
			},
			blocked: true,
			site:    espFormFirewallSite{Family: "ip", Table: "nat", Chain: "prerouting", Hook: "prerouting", Rule: `handle 6 "dnat-web"`, Reason: espFormReasonNAT},
		},
		{
			name: "an xt compat target blocks",
			build: func(f *espFWRuleset) {
				in := f.base(f.table(nftables.TableFamilyIPv4, "filter"), "INPUT", espFWHookInput, nftables.ChainPolicyAccept)
				f.rule(in, 11, "", espFWCounter(), espFWOne(&expr.Target{Name: "REJECT"}))
			},
			blocked: true,
			site:    espFormFirewallSite{Family: "ip", Table: "filter", Chain: "INPUT", Hook: "input", Rule: "handle 11", Reason: espFormReasonXT},
		},
		{
			name: "a verdict map blocks",
			build: func(f *espFWRuleset) {
				in := f.base(f.table(nftables.TableFamilyINet, "filter"), "input", espFWHookInput, nftables.ChainPolicyAccept)
				f.rule(in, 12, "",
					espFWOne(&expr.Payload{DestRegister: 1, Base: expr.PayloadBaseNetworkHeader, Offset: 12, Len: 4}),
					espFWOne(&expr.Lookup{SourceRegister: 1, DestRegister: unix.NFT_REG_VERDICT, IsDestRegSet: true, SetName: "__map0"}))
			},
			blocked: true,
			site:    espFormFirewallSite{Family: "inet", Table: "filter", Chain: "input", Hook: "input", Rule: "handle 12", Reason: espFormReasonVerdictMap},
		},
		{
			name: "a fwd the library cannot decode blocks",
			build: func(f *espFWRuleset) {
				ing := f.base(f.table(nftables.TableFamilyNetdev, "edge"), "ingress", espFWHookIngress, nftables.ChainPolicyAccept)
				f.rule(ing, 13, "", espFWOne(&expr.Immediate{Register: 1, Data: []byte{2, 0, 0, 0}}), espFWOne(&espFormOpaqueExpr{name: "fwd"}))
			},
			blocked: true,
			site:    espFormFirewallSite{Family: "netdev", Table: "edge", Chain: "ingress", Hook: "ingress", Rule: "handle 13", Reason: espFormReasonForward},
		},
		{
			name: "an expression the classifier does not name blocks",
			build: func(f *espFWRuleset) {
				in := f.base(f.table(nftables.TableFamilyINet, "filter"), "input", espFWHookInput, nftables.ChainPolicyAccept)
				f.rule(in, 14, "", espFWOne(&expr.SecMark{Ctx: "system_u:object_r:ipsec_t:s0"}))
			},
			blocked: true,
			site:    espFormFirewallSite{Family: "inet", Table: "filter", Chain: "input", Hook: "input", Rule: "handle 14", Reason: espFormReasonUnknown + " *expr.SecMark"},
		},
		{
			name: "an expression the kernel holds and the decoder does not know blocks",
			build: func(f *espFWRuleset) {
				in := f.base(f.table(nftables.TableFamilyINet, "filter"), "input", espFWHookInput, nftables.ChainPolicyAccept)
				f.rule(in, 15, "", espFWOne(&espFormOpaqueExpr{name: "osf"}))
			},
			blocked: true,
			site:    espFormFirewallSite{Family: "inet", Table: "filter", Chain: "input", Hook: "input", Rule: "handle 15", Reason: espFormReasonUnknown + " osf"},
		},
		{
			name: "a limit with no verdict is safe",
			build: func(f *espFWRuleset) {
				in := f.base(f.table(nftables.TableFamilyINet, "filter"), "input", espFWHookInput, nftables.ChainPolicyAccept)
				f.rule(in, 1, "", espFWOne(&expr.Limit{Type: expr.LimitTypePkts, Rate: 10, Unit: expr.LimitTimeSecond}), espFWCounter())
			},
		},
		{
			name: "counter-only is safe",
			build: func(f *espFWRuleset) {
				in := f.base(f.table(nftables.TableFamilyINet, "filter"), "input", espFWHookInput, nftables.ChainPolicyAccept)
				f.rule(in, 1, "", espFWCounter())
			},
		},
		{
			name: "a jump into a loop blocks at the first jump",
			build: func(f *espFWRuleset) {
				tab := f.table(nftables.TableFamilyIPv4, "filter")
				in := f.base(tab, "input", espFWHookInput, nftables.ChainPolicyAccept)
				a := f.regular(tab, "a")
				b := f.regular(tab, "b")
				f.rule(in, 2, "", espFWJump(expr.VerdictJump, "a"))
				f.rule(a, 3, "", espFWJump(expr.VerdictGoto, "b"))
				f.rule(b, 4, "back", espFWJump(expr.VerdictJump, "a"))
			},
			blocked: true,
			site:    espFormFirewallSite{Family: "ip", Table: "filter", Chain: "input", Hook: "input", Rule: "handle 2", Reason: espFormReasonJump},
		},
		{
			name: "a jump to a missing chain blocks at the jump",
			build: func(f *espFWRuleset) {
				in := f.base(f.table(nftables.TableFamilyIPv4, "filter"), "input", espFWHookInput, nftables.ChainPolicyAccept)
				f.rule(in, 2, "", espFWJump(expr.VerdictJump, "gone"))
			},
			blocked: true,
			site:    espFormFirewallSite{Family: "ip", Table: "filter", Chain: "input", Hook: "input", Rule: "handle 2", Reason: espFormReasonJump},
		},
		{
			// MUTATION: answer false from espFormExcludesESP.
			name: "ip protocol and l4proto first pairs that exclude esp are safe",
			build: func(f *espFWRuleset) {
				in := f.base(f.table(nftables.TableFamilyIPv4, "filter"), "input", espFWHookInput, nftables.ChainPolicyAccept)
				f.rule(in, 1, "", espFWIPProto(expr.CmpOpEq, unix.IPPROTO_UDP), espFWVerdict(expr.VerdictDrop))
				f.rule(in, 3, "", espFWL4Proto(expr.CmpOpEq, unix.IPPROTO_ICMP), espFWVerdict(expr.VerdictDrop))
			},
		},
		{
			// Only `eq X`, X not 50, excludes.
			// MUTATION: also exclude on `!= 50`.
			name: "an l4proto != esp first pair excludes nothing",
			build: func(f *espFWRuleset) {
				in := f.base(f.table(nftables.TableFamilyIPv4, "filter"), "input", espFWHookInput, nftables.ChainPolicyAccept)
				f.rule(in, 2, "", espFWL4Proto(expr.CmpOpNeq, unix.IPPROTO_ESP), espFWVerdict(expr.VerdictDrop))
			},
			blocked: true,
			site:    espFormFirewallSite{Family: "ip", Table: "filter", Chain: "input", Hook: "input", Rule: "handle 2", Reason: espFormReasonDrop},
		},
		{
			// MUTATION: also exclude on `!= 50`.
			name: "an ip protocol != esp first pair excludes nothing",
			build: func(f *espFWRuleset) {
				in := f.base(f.table(nftables.TableFamilyIPv4, "filter"), "input", espFWHookInput, nftables.ChainPolicyAccept)
				f.rule(in, 2, "", espFWIPProto(expr.CmpOpNeq, unix.IPPROTO_ESP), espFWVerdict(expr.VerdictDrop))
			},
			blocked: true,
			site:    espFormFirewallSite{Family: "ip", Table: "filter", Chain: "input", Hook: "input", Rule: "handle 2", Reason: espFormReasonDrop},
		},
		{
			// The ip protocol load excludes in inet and ip tables only.
			// MUTATION: take the ip protocol load in every family.
			name: "an ip protocol first pair in a netdev table excludes nothing",
			build: func(f *espFWRuleset) {
				ing := f.base(f.table(nftables.TableFamilyNetdev, "edge"), "ingress", espFWHookIngress, nftables.ChainPolicyAccept)
				f.rule(ing, 4, "", espFWIPProto(expr.CmpOpEq, unix.IPPROTO_TCP), espFWVerdict(expr.VerdictDrop))
			},
			blocked: true,
			site:    espFormFirewallSite{Family: "netdev", Table: "edge", Chain: "ingress", Hook: "ingress", Rule: "handle 4", Reason: espFormReasonDrop},
		},
		{
			// MUTATION: look for the excluding pair past the first expression.
			name: "an excluding pair that is not the rule's first two expressions excludes nothing",
			build: func(f *espFWRuleset) {
				in := f.base(f.table(nftables.TableFamilyINet, "filter"), "input", espFWHookInput, nftables.ChainPolicyAccept)
				f.rule(in, 5, "", espFWCounter(), espFWL4Proto(expr.CmpOpEq, unix.IPPROTO_TCP), espFWVerdict(expr.VerdictDrop))
			},
			blocked: true,
			site:    espFormFirewallSite{Family: "inet", Table: "filter", Chain: "input", Hook: "input", Rule: "handle 5", Reason: espFormReasonDrop},
		},
		{
			// Round-4 B1. A map lookup writes up to 64 octets (nft_lookup_eval copies
			// set->dlen), so the register model forgot a stale l4proto it had
			// overwritten. The load is not followed by its compare, so nothing
			// excludes, and the map output is off the harmless list.
			// MUTATION: exclude on an l4proto load and a later compare of its register
			// (the deleted register model), and this row reads safe.
			name: "a map lookup between the l4proto load and its compare excludes nothing",
			build: func(f *espFWRuleset) {
				in := f.base(f.table(nftables.TableFamilyINet, "filter"), "input", espFWHookInput, nftables.ChainPolicyAccept)
				f.rule(in, 6, "",
					espFWOne(&expr.Meta{Key: expr.MetaKeyL4PROTO, Register: unix.NFT_REG32_04}),
					espFWOne(&expr.Lookup{SourceRegister: unix.NFT_REG32_03, DestRegister: unix.NFT_REG32_00, IsDestRegSet: true, SetName: "m"}),
					espFWOne(&expr.Cmp{Op: expr.CmpOpEq, Register: unix.NFT_REG32_04, Data: []byte{unix.IPPROTO_UDP}}),
					espFWVerdict(expr.VerdictDrop))
			},
			blocked: true,
			site:    espFormFirewallSite{Family: "inet", Table: "filter", Chain: "input", Hook: "input", Rule: "handle 6", Reason: espFormReasonMap},
		},
		{
			// Round-5. A membership lookup is no pure selector: a hit runs the
			// element's own stateful expressions (nft_set_elem_update_expr), which
			// the analyzer never reads, so a connlimit on an element can drop ESP
			// from `ip saddr @trusted counter accept`. This row once read safe.
			// MUTATION: answer "" for a lookup with no output in espFormLookup.
			name: "a set lookup with no output blocks",
			build: func(f *espFWRuleset) {
				in := f.base(f.table(nftables.TableFamilyINet, "filter"), "input", espFWHookInput, nftables.ChainPolicyAccept)
				f.rule(in, 7, "",
					espFWOne(&expr.Payload{DestRegister: 1, Base: expr.PayloadBaseNetworkHeader, Offset: 12, Len: 4}),
					espFWOne(&expr.Lookup{SourceRegister: 1, SetName: "trusted"}),
					espFWCounter(), espFWVerdict(expr.VerdictAccept))
			},
			blocked: true,
			site:    espFormFirewallSite{Family: "inet", Table: "filter", Chain: "input", Hook: "input", Rule: "handle 7", Reason: espFormReasonSet},
		},
		{
			// The round-5 attack: `ip saddr @s counter accept` in an ip table, the
			// set's element carrying a connlimit the rule does not show. Judged at
			// the lookup, ahead of the accept.
			// MUTATION: answer espFormReasonMap for a lookup with no output: the
			// site's reason then names a map the rule does not use.
			name: "a membership lookup ahead of an accept blocks as a set lookup",
			build: func(f *espFWRuleset) {
				in := f.base(f.table(nftables.TableFamilyIPv4, "filter"), "input", espFWHookInput, nftables.ChainPolicyAccept)
				f.rule(in, 8, "",
					espFWOne(&expr.Payload{DestRegister: 1, Base: expr.PayloadBaseNetworkHeader, Offset: 12, Len: 4}),
					espFWOne(&expr.Lookup{SourceRegister: 1, SetName: "s"}),
					espFWCounter(), espFWVerdict(expr.VerdictAccept))
			},
			blocked: true,
			site:    espFormFirewallSite{Family: "ip", Table: "filter", Chain: "input", Hook: "input", Rule: "handle 8", Reason: espFormReasonSet},
		},
		{
			// Round-5. An exthdr with neither register is the TCP option strip
			// (nft_exthdr_select_ops answers nft_exthdr_tcp_strip_ops), which writes
			// the packet and drops it when the option cannot be removed.
			// MUTATION: test only SourceRegister in espFormHarmless's Exthdr case.
			name: "an exthdr with no register blocks",
			build: func(f *espFWRuleset) {
				in := f.base(f.table(nftables.TableFamilyINet, "filter"), "input", espFWHookInput, nftables.ChainPolicyAccept)
				f.rule(in, 9, "", espFWOne(&expr.Exthdr{Op: expr.ExthdrOpTcpopt, Type: 2}))
			},
			blocked: true,
			site:    espFormFirewallSite{Family: "inet", Table: "filter", Chain: "input", Hook: "input", Rule: "handle 9", Reason: espFormReasonRewrite},
		},
		{
			// The exthdr load stays harmless: a destination register, no source.
			// MUTATION: refuse every exthdr in espFormHarmless.
			name: "an exthdr load is harmless",
			build: func(f *espFWRuleset) {
				in := f.base(f.table(nftables.TableFamilyINet, "filter"), "input", espFWHookInput, nftables.ChainPolicyAccept)
				f.rule(in, 10, "",
					espFWOne(&expr.Exthdr{Op: expr.ExthdrOpTcpopt, Type: 2, Offset: 2, Len: 2, DestRegister: 1}),
					espFWOne(&expr.Cmp{Op: expr.CmpOpEq, Register: 1, Data: []byte{5, 0xb4}}),
					espFWCounter())
			},
		},
		{
			name: "an l4proto compare that does not exclude esp blocks",
			build: func(f *espFWRuleset) {
				in := f.base(f.table(nftables.TableFamilyIPv4, "filter"), "input", espFWHookInput, nftables.ChainPolicyAccept)
				f.rule(in, 1, "", espFWL4Proto(expr.CmpOpNeq, unix.IPPROTO_TCP), espFWVerdict(expr.VerdictDrop))
			},
			blocked: true,
			site:    espFormFirewallSite{Family: "ip", Table: "filter", Chain: "input", Hook: "input", Rule: "handle 1", Reason: espFormReasonDrop},
		},
		{
			name: "a compare on a register the l4proto load did not fill excludes nothing",
			build: func(f *espFWRuleset) {
				in := f.base(f.table(nftables.TableFamilyIPv4, "filter"), "input", espFWHookInput, nftables.ChainPolicyAccept)
				f.rule(in, 1, "",
					espFWOne(&expr.Meta{Key: expr.MetaKeyL4PROTO, Register: 1}),
					espFWOne(&expr.Cmp{Op: expr.CmpOpEq, Register: unix.NFT_REG_2, Data: []byte{unix.IPPROTO_TCP}}),
					espFWVerdict(expr.VerdictDrop))
			},
			blocked: true,
			site:    espFormFirewallSite{Family: "ip", Table: "filter", Chain: "input", Hook: "input", Rule: "handle 1", Reason: espFormReasonDrop},
		},
		{
			name: "a load that overwrites the l4proto register excludes nothing",
			build: func(f *espFWRuleset) {
				in := f.base(f.table(nftables.TableFamilyIPv4, "filter"), "input", espFWHookInput, nftables.ChainPolicyAccept)
				f.rule(in, 1, "",
					espFWOne(&expr.Meta{Key: expr.MetaKeyL4PROTO, Register: 1}),
					espFWOne(&expr.Meta{Key: expr.MetaKeyMARK, Register: 1}),
					espFWOne(&expr.Cmp{Op: expr.CmpOpEq, Register: 1, Data: []byte{unix.IPPROTO_TCP}}),
					espFWVerdict(expr.VerdictDrop))
			},
			blocked: true,
			site:    espFormFirewallSite{Family: "ip", Table: "filter", Chain: "input", Hook: "input", Rule: "handle 1", Reason: espFormReasonDrop},
		},
		{
			name: "an ipv6 guard in an inet table excludes the rule",
			build: func(f *espFWRuleset) {
				in := f.base(f.table(nftables.TableFamilyINet, "filter"), "input", espFWHookInput, nftables.ChainPolicyAccept)
				f.rule(in, 1, "", espFWNFProto(unix.NFPROTO_IPV6), espFWVerdict(expr.VerdictDrop))
			},
		},
		{
			name: "a statement ahead of an excluding guard still acts on esp",
			build: func(f *espFWRuleset) {
				in := f.base(f.table(nftables.TableFamilyINet, "filter"), "input", espFWHookInput, nftables.ChainPolicyAccept)
				f.rule(in, 1, "", espFWOne(&expr.Queue{Num: 1}), espFWL4Proto(expr.CmpOpEq, unix.IPPROTO_TCP))
			},
			blocked: true,
			site:    espFormFirewallSite{Family: "inet", Table: "filter", Chain: "input", Hook: "input", Rule: "handle 1", Reason: espFormReasonQueue},
		},
		{
			// MUTATION: classify the log off the harmless list.
			name: "a log with no verdict is safe",
			build: func(f *espFWRuleset) {
				pre := f.base(f.table(nftables.TableFamilyIPv4, "mangle"), "prerouting", espFWHookPrerouting, nftables.ChainPolicyAccept)
				f.rule(pre, 3, "", espFWOne(&expr.Log{}), espFWCounter())
			},
		},
		{
			// Round-4 B2. nft_payload_set_init accepts any csum_offset up to U8_MAX,
			// and nft_payload_set_eval writes the checksum delta there, so a
			// one-octet DSCP write with csum_offset 8 rewrites the TTL and protocol.
			// MUTATION: name the network-header octet-1 write harmless again (the
			// round-3 DSCP entry), and this row reads safe.
			name: "a dscp write whose checksum lands on the ttl blocks",
			build: func(f *espFWRuleset) {
				pre := f.base(f.table(nftables.TableFamilyIPv4, "mangle"), "prerouting", espFWHookPrerouting, nftables.ChainPolicyAccept)
				f.rule(pre, 2, "dscp",
					espFWOne(&expr.Immediate{Register: 1, Data: []byte{46 << 2}}),
					espFWOne(&expr.Payload{OperationType: expr.PayloadWrite, SourceRegister: 1, Base: expr.PayloadBaseNetworkHeader, Offset: 1, Len: 1, CsumType: expr.CsumTypeInet, CsumOffset: 8}))
			},
			blocked: true,
			site:    espFormFirewallSite{Family: "ip", Table: "mangle", Chain: "prerouting", Hook: "prerouting", Rule: `handle 2 "dscp"`, Reason: espFormReasonRewrite},
		},
		{
			// setDSCPv4's own sequence, checksum at offset 10: off the list too, because
			// no stock Ze config puts a DSCP write on a base chain ESP crosses.
			// MUTATION: name the network-header octet-1 write harmless again.
			name: "the lowering's dscp set blocks",
			build: func(f *espFWRuleset) {
				pre := f.base(f.table(nftables.TableFamilyIPv4, "mangle"), "prerouting", espFWHookPrerouting, nftables.ChainPolicyAccept)
				f.rule(pre, 2, "",
					espFWOne(&expr.Payload{DestRegister: 1, Base: expr.PayloadBaseNetworkHeader, Offset: 1, Len: 1}),
					espFWOne(&expr.Bitwise{SourceRegister: 1, DestRegister: 1, Len: 1, Mask: []byte{0x03}, Xor: []byte{46 << 2}}),
					espFWOne(&expr.Payload{OperationType: expr.PayloadWrite, SourceRegister: 1, Base: expr.PayloadBaseNetworkHeader, Offset: 1, Len: 1, CsumType: expr.CsumTypeInet, CsumOffset: 10}))
			},
			blocked: true,
			site:    espFormFirewallSite{Family: "ip", Table: "mangle", Chain: "prerouting", Hook: "prerouting", Rule: "handle 2", Reason: espFormReasonRewrite},
		},
		{
			// MUTATION: name meta mark set harmless again.
			name: "a meta mark set blocks",
			build: func(f *espFWRuleset) {
				pre := f.base(f.table(nftables.TableFamilyIPv4, "mangle"), "prerouting", espFWHookPrerouting, nftables.ChainPolicyAccept)
				f.rule(pre, 1, "", espFWOne(&expr.Immediate{Register: 1, Data: []byte{1, 0, 0, 0}}), espFWOne(&expr.Meta{Key: expr.MetaKeyMARK, SourceRegister: true, Register: 1}))
			},
			blocked: true,
			site:    espFormFirewallSite{Family: "ip", Table: "mangle", Chain: "prerouting", Hook: "prerouting", Rule: "handle 1", Reason: espFormReasonStateSet},
		},
		{
			// MUTATION: name ct mark set harmless again.
			name: "a ct mark set blocks",
			build: func(f *espFWRuleset) {
				pre := f.base(f.table(nftables.TableFamilyIPv4, "mangle"), "prerouting", espFWHookPrerouting, nftables.ChainPolicyAccept)
				f.rule(pre, 1, "", espFWOne(&expr.Immediate{Register: 1, Data: []byte{1, 0, 0, 0}}), espFWOne(&expr.Ct{Key: expr.CtKeyMARK, SourceRegister: true, Register: 1}))
			},
			blocked: true,
			site:    espFormFirewallSite{Family: "ip", Table: "mangle", Chain: "prerouting", Hook: "prerouting", Rule: "handle 1", Reason: espFormReasonStateSet},
		},
		{
			// MUTATION: name notrack harmless again.
			name: "a notrack blocks",
			build: func(f *espFWRuleset) {
				pre := f.base(f.table(nftables.TableFamilyIPv4, "raw"), "prerouting", espFWHookPrerouting, nftables.ChainPolicyAccept)
				f.rule(pre, 1, "", espFWOne(&expr.Notrack{}))
			},
			blocked: true,
			site:    espFormFirewallSite{Family: "ip", Table: "raw", Chain: "prerouting", Hook: "prerouting", Rule: "handle 1", Reason: espFormReasonStateSet},
		},
		{
			// MUTATION: name dup harmless again.
			name: "a dup blocks",
			build: func(f *espFWRuleset) {
				pre := f.base(f.table(nftables.TableFamilyIPv4, "mangle"), "prerouting", espFWHookPrerouting, nftables.ChainPolicyAccept)
				f.rule(pre, 1, "", espFWOne(&expr.Immediate{Register: 1, Data: []byte{192, 0, 2, 9}}), espFWOne(&expr.Dup{RegAddr: 1}))
			},
			blocked: true,
			site:    espFormFirewallSite{Family: "ip", Table: "mangle", Chain: "prerouting", Hook: "prerouting", Rule: "handle 1", Reason: espFormReasonDup},
		},
		{
			// A dynset writes an element a later lookup reads, so what that lookup
			// matches for ESP depends on this rule.
			// MUTATION: name dynset harmless again.
			name: "a dynset blocks",
			build: func(f *espFWRuleset) {
				in := f.base(f.table(nftables.TableFamilyINet, "filter"), "input", espFWHookInput, nftables.ChainPolicyAccept)
				f.rule(in, 1, "",
					espFWOne(&expr.Payload{DestRegister: 1, Base: expr.PayloadBaseNetworkHeader, Offset: 12, Len: 4}),
					espFWOne(&expr.Dynset{SrcRegKey: 1, SetName: "seen", Operation: uint32(unix.NFT_DYNSET_OP_ADD)}))
			},
			blocked: true,
			site:    espFormFirewallSite{Family: "inet", Table: "filter", Chain: "input", Hook: "input", Rule: "handle 1", Reason: espFormReasonSetUpdate},
		},
		{
			// MUTATION: name the inline quota harmless, as the limit is.
			name: "an inline quota blocks",
			build: func(f *espFWRuleset) {
				in := f.base(f.table(nftables.TableFamilyINet, "filter"), "input", espFWHookInput, nftables.ChainPolicyAccept)
				f.rule(in, 1, "", espFWOne(&expr.Quota{Bytes: 1 << 20}))
			},
			blocked: true,
			site:    espFormFirewallSite{Family: "inet", Table: "filter", Chain: "input", Hook: "input", Rule: "handle 1", Reason: espFormReasonUnlisted + " *expr.Quota"},
		},
		{
			name: "a TTL payload write blocks",
			build: func(f *espFWRuleset) {
				pre := f.base(f.table(nftables.TableFamilyIPv4, "mangle"), "prerouting", espFWHookPrerouting, nftables.ChainPolicyAccept)
				f.rule(pre, 5, "ttl", espFWOne(&expr.Immediate{Register: 1, Data: []byte{64}}),
					espFWOne(&expr.Payload{OperationType: expr.PayloadWrite, SourceRegister: 1, Base: expr.PayloadBaseNetworkHeader, Offset: 8, Len: 1, CsumType: expr.CsumTypeInet, CsumOffset: 10}))
			},
			blocked: true,
			site:    espFormFirewallSite{Family: "ip", Table: "mangle", Chain: "prerouting", Hook: "prerouting", Rule: `handle 5 "ttl"`, Reason: espFormReasonRewrite},
		},
		{
			name: "rules are walked in the order the kernel returns them, not by handle",
			build: func(f *espFWRuleset) {
				in := f.base(f.table(nftables.TableFamilyIPv4, "filter"), "input", espFWHookInput, nftables.ChainPolicyAccept)
				// `nft insert` puts a new rule, with a higher handle, at the head.
				f.rule(in, 9, "inserted-esp", espFWL4Proto(expr.CmpOpEq, unix.IPPROTO_ESP), espFWVerdict(expr.VerdictAccept))
				f.rule(in, 1, "drop-all", espFWVerdict(expr.VerdictDrop))
			},
		},
		{
			name: "an nft comment in TLV user data is reported",
			build: func(f *espFWRuleset) {
				in := f.base(f.table(nftables.TableFamilyIPv4, "filter"), "input", espFWHookInput, nftables.ChainPolicyAccept)
				f.ruleData(in, 8, userdata.AppendString(nil, userdata.TypeComment, "drop by hand"), espFWVerdict(expr.VerdictDrop))
			},
			blocked: true,
			site:    espFormFirewallSite{Family: "ip", Table: "filter", Chain: "input", Hook: "input", Rule: `handle 8 "drop by hand"`, Reason: espFormReasonDrop},
		},
		{
			// `ether type 8021q meta l4proto esp drop` on a VLAN's parent link. The
			// kernel rebuilds the tag the device took off into the link-layer load, so
			// the rule matches bare ESP of that VLAN.
			// MUTATION: restore the Excludes answer for an ether type load at LL 12.
			name: "an 802.1Q ether type compare on netdev ingress excludes nothing",
			build: func(f *espFWRuleset) {
				ing := f.base(f.table(nftables.TableFamilyNetdev, "edge"), "ingress", espFWHookIngress, nftables.ChainPolicyAccept)
				f.rule(ing, 21, "vlan-esp", espFWEtherType(expr.CmpOpEq, unix.ETH_P_8021Q), espFWL4Proto(expr.CmpOpEq, unix.IPPROTO_ESP), espFWVerdict(expr.VerdictDrop))
			},
			blocked: true,
			site:    espFormFirewallSite{Family: "netdev", Table: "edge", Chain: "ingress", Hook: "ingress", Rule: `handle 21 "vlan-esp"`, Reason: espFormReasonDrop},
		},
		{
			// MUTATION: restore the Excludes answer for an ether type load at LL 12.
			name: "an 802.1Q ether type compare in a bridge prerouting chain excludes nothing",
			build: func(f *espFWRuleset) {
				pre := f.base(f.table(nftables.TableFamilyBridge, "br"), "prerouting", espFormHookBridgePrerouting, nftables.ChainPolicyAccept)
				f.rule(pre, 22, "", espFWEtherType(expr.CmpOpEq, unix.ETH_P_8021Q), espFWL4Proto(expr.CmpOpEq, unix.IPPROTO_ESP), espFWVerdict(expr.VerdictDrop))
			},
			blocked: true,
			site:    espFormFirewallSite{Family: "bridge", Table: "br", Chain: "prerouting", Hook: "prerouting", Rule: "handle 22", Reason: espFormReasonDrop},
		},
		{
			// MUTATION: restore the Excludes answer for an ether type load at LL 12.
			name: "an ether type != ip compare excludes nothing",
			build: func(f *espFWRuleset) {
				ing := f.base(f.table(nftables.TableFamilyNetdev, "edge"), "ingress", espFWHookIngress, nftables.ChainPolicyAccept)
				f.rule(ing, 23, "", espFWEtherType(expr.CmpOpNeq, unix.ETH_P_IP), espFWVerdict(expr.VerdictDrop))
			},
			blocked: true,
			site:    espFormFirewallSite{Family: "netdev", Table: "edge", Chain: "ingress", Hook: "ingress", Rule: "handle 23", Reason: espFormReasonDrop},
		},
		{
			// The l4proto pair is not the rule's first two expressions.
			// MUTATION: look for the excluding pair past the first expression.
			name: "an l4proto compare after an ether type compare excludes nothing",
			build: func(f *espFWRuleset) {
				ing := f.base(f.table(nftables.TableFamilyNetdev, "edge"), "ingress", espFWHookIngress, nftables.ChainPolicyAccept)
				f.rule(ing, 24, "", espFWEtherType(expr.CmpOpEq, unix.ETH_P_8021Q), espFWL4Proto(expr.CmpOpEq, unix.IPPROTO_TCP), espFWVerdict(expr.VerdictDrop))
			},
			blocked: true,
			site:    espFormFirewallSite{Family: "netdev", Table: "edge", Chain: "ingress", Hook: "ingress", Rule: "handle 24", Reason: espFormReasonDrop},
		},
		{
			// MUTATION: answer false from espFormExcludesESP.
			name: "an l4proto first pair on netdev ingress excludes the rule",
			build: func(f *espFWRuleset) {
				ing := f.base(f.table(nftables.TableFamilyNetdev, "edge"), "ingress", espFWHookIngress, nftables.ChainPolicyAccept)
				f.rule(ing, 29, "", espFWL4Proto(expr.CmpOpEq, unix.IPPROTO_TCP), espFWEtherType(expr.CmpOpEq, unix.ETH_P_8021Q), espFWVerdict(expr.VerdictDrop))
			},
		},
		{
			// MUTATION: restore the Excludes answer for a meta protocol load.
			name: "a meta protocol compare excludes nothing",
			build: func(f *espFWRuleset) {
				ing := f.base(f.table(nftables.TableFamilyNetdev, "edge"), "ingress", espFWHookIngress, nftables.ChainPolicyAccept)
				f.rule(ing, 25, "", espFWMetaProtocol(expr.CmpOpEq, unix.ETH_P_IPV6), espFWVerdict(expr.VerdictDrop))
			},
			blocked: true,
			site:    espFormFirewallSite{Family: "netdev", Table: "edge", Chain: "ingress", Hook: "ingress", Rule: "handle 25", Reason: espFormReasonDrop},
		},
		{
			// nft's netdev form of `ip protocol esp accept`. meta protocol narrows, so
			// this is no definite accept and the policy drop is reached.
			// MUTATION: answer Guards for a meta protocol ip compare.
			name: "meta protocol ip ahead of the esp match is no definite accept",
			build: func(f *espFWRuleset) {
				ing := f.base(f.table(nftables.TableFamilyNetdev, "edge"), "ingress", espFWHookIngress, nftables.ChainPolicyDrop)
				f.rule(ing, 26, "", espFWMetaProtocol(expr.CmpOpEq, unix.ETH_P_IP), espFWIPProto(expr.CmpOpEq, unix.IPPROTO_ESP), espFWVerdict(expr.VerdictAccept))
			},
			blocked: true,
			site:    espFormFirewallSite{Family: "netdev", Table: "edge", Chain: "ingress", Hook: "ingress", Rule: "policy drop", Reason: espFormReasonPolicyDrop},
		},
		{
			// nft's netdev form of `ip protocol tcp drop`: the meta protocol pair comes
			// first, and the ip protocol load is not taken in a netdev table.
			// MUTATION: look for the excluding pair past the first expression, and
			// take the ip protocol load in every family.
			name: "meta protocol ip ahead of an ip protocol compare excludes nothing",
			build: func(f *espFWRuleset) {
				ing := f.base(f.table(nftables.TableFamilyNetdev, "edge"), "ingress", espFWHookIngress, nftables.ChainPolicyAccept)
				f.rule(ing, 27, "", espFWMetaProtocol(expr.CmpOpEq, unix.ETH_P_IP), espFWIPProto(expr.CmpOpEq, unix.IPPROTO_TCP), espFWVerdict(expr.VerdictDrop))
			},
			blocked: true,
			site:    espFormFirewallSite{Family: "netdev", Table: "edge", Chain: "ingress", Hook: "ingress", Rule: "handle 27", Reason: espFormReasonDrop},
		},
		{
			// MUTATION: restore the Excludes answer for a network-header load at offset 0.
			name: "an ARP layout compare on the network header excludes nothing",
			build: func(f *espFWRuleset) {
				ing := f.base(f.table(nftables.TableFamilyNetdev, "edge"), "ingress", espFWHookIngress, nftables.ChainPolicyAccept)
				f.rule(ing, 28, "arp-guard", espFWARPSender(192, 0, 2, 9), espFWVerdict(expr.VerdictDrop))
			},
			blocked: true,
			site:    espFormFirewallSite{Family: "netdev", Table: "edge", Chain: "ingress", Hook: "ingress", Rule: `handle 28 "arp-guard"`, Reason: espFormReasonDrop},
		},
		{
			// MUTATION: answer Accepts (or Excluded) for VerdictBreak.
			name: "a break sends esp on to the next rule",
			build: func(f *espFWRuleset) {
				in := f.base(f.table(nftables.TableFamilyIPv4, "filter"), "input", espFWHookInput, nftables.ChainPolicyAccept)
				f.rule(in, 1, "", espFWL4Proto(expr.CmpOpEq, unix.IPPROTO_ESP), espFWVerdict(expr.VerdictBreak))
				f.rule(in, 2, "after-break", espFWVerdict(expr.VerdictDrop))
			},
			blocked: true,
			site:    espFormFirewallSite{Family: "ip", Table: "filter", Chain: "input", Hook: "input", Rule: `handle 2 "after-break"`, Reason: espFormReasonDrop},
		},
		{
			// MUTATION: answer unsafe for VerdictBreak.
			name: "a break before an accept policy is safe",
			build: func(f *espFWRuleset) {
				in := f.base(f.table(nftables.TableFamilyIPv4, "filter"), "input", espFWHookInput, nftables.ChainPolicyAccept)
				f.rule(in, 1, "", espFWCounter(), espFWVerdict(expr.VerdictBreak))
			},
		},
		{
			// MUTATION: classify the counter objref harmless.
			name: "a counter objref blocks",
			build: func(f *espFWRuleset) {
				in := f.base(f.table(nftables.TableFamilyINet, "filter"), "input", espFWHookInput, nftables.ChainPolicyAccept)
				f.rule(in, 1, "", espFWOne(&expr.Objref{Type: unix.NFT_OBJECT_COUNTER, Name: "esp"}))
			},
			blocked: true,
			site:    espFormFirewallSite{Family: "inet", Table: "filter", Chain: "input", Hook: "input", Rule: "handle 1", Reason: espFormReasonObjref + " 1"},
		},
		{
			// MUTATION: classify the quota objref harmless.
			name: "a quota objref blocks",
			build: func(f *espFWRuleset) {
				in := f.base(f.table(nftables.TableFamilyINet, "filter"), "input", espFWHookInput, nftables.ChainPolicyAccept)
				f.rule(in, 1, "", espFWOne(&expr.Objref{Type: unix.NFT_OBJECT_QUOTA, Name: "q"}))
			},
			blocked: true,
			site:    espFormFirewallSite{Family: "inet", Table: "filter", Chain: "input", Hook: "input", Rule: "handle 1", Reason: espFormReasonObjref + " 2"},
		},
		{
			// `ct helper set "<name>"`, as lowerSetCtHelper emits it: a ct write.
			// MUTATION: classify the ct helper objref harmless.
			name: "a ct helper objref blocks",
			build: func(f *espFWRuleset) {
				pre := f.base(f.table(nftables.TableFamilyINet, "filter"), "prerouting", espFWHookPrerouting, nftables.ChainPolicyAccept)
				f.rule(pre, 1, "", espFWOne(&expr.Objref{Type: unix.NFT_OBJECT_CT_HELPER, Name: "ftp"}))
			},
			blocked: true,
			site:    espFormFirewallSite{Family: "inet", Table: "filter", Chain: "prerouting", Hook: "prerouting", Rule: "handle 1", Reason: espFormReasonObjref + " 3"},
		},
		{
			// The map form (`counter name ip saddr map @m`) carries NFTA_OBJREF_SET_*,
			// which the vendored decoder does not read, so it decodes to Type 0 and no
			// name: the object it picks per packet is unknown.
			// MUTATION: classify objref Type 0 harmless.
			name: "an objref map, decoded as Type 0, blocks",
			build: func(f *espFWRuleset) {
				in := f.base(f.table(nftables.TableFamilyINet, "filter"), "input", espFWHookInput, nftables.ChainPolicyAccept)
				f.rule(in, 31, "", espFWOne(&expr.Objref{}))
			},
			blocked: true,
			site:    espFormFirewallSite{Family: "inet", Table: "filter", Chain: "input", Hook: "input", Rule: "handle 31", Reason: espFormReasonObjref + " 0"},
		},
		{
			// MUTATION: classify the synproxy objref harmless.
			name: "a synproxy objref blocks",
			build: func(f *espFWRuleset) {
				in := f.base(f.table(nftables.TableFamilyINet, "filter"), "input", espFWHookInput, nftables.ChainPolicyAccept)
				f.rule(in, 32, "", espFWOne(&expr.Objref{Type: unix.NFT_OBJECT_SYNPROXY, Name: "sp"}))
			},
			blocked: true,
			site:    espFormFirewallSite{Family: "inet", Table: "filter", Chain: "input", Hook: "input", Rule: "handle 32", Reason: espFormReasonSynProxy},
		},
		{
			// MUTATION: classify every meta set harmless, as the mark set is.
			name: "a meta set other than mark blocks",
			build: func(f *espFWRuleset) {
				pre := f.base(f.table(nftables.TableFamilyIPv4, "mangle"), "prerouting", espFWHookPrerouting, nftables.ChainPolicyAccept)
				f.rule(pre, 33, "prio", espFWOne(&expr.Immediate{Register: 1, Data: []byte{0, 1, 0, 1}}), espFWOne(&expr.Meta{Key: expr.MetaKeyPRIORITY, SourceRegister: true, Register: 1}))
			},
			blocked: true,
			site:    espFormFirewallSite{Family: "ip", Table: "mangle", Chain: "prerouting", Hook: "prerouting", Rule: `handle 33 "prio"`, Reason: espFormReasonStateSet},
		},

		// Stock Ze configs, each row the expression sequence the lowering emits for
		// the producer named (internal/plugins/firewall/nft/lower_linux.go: matches
		// in term order, nfprotoGuard first in an address match of an inet table,
		// lowerProtoMatch as meta l4proto).
		{
			// copp, translatePolicy (internal/plugins/copp/translate.go): the
			// established accept, then the trusted, neighbor and unknown-source terms,
			// each opening with `matchTCP`.
			// MUTATION: answer false from espFormExcludesESP.
			name: "stock copp keeps the fast path on",
			build: func(f *espFWRuleset) {
				in := f.base(f.table(nftables.TableFamilyINet, "ze_copp"), "input", espFWHookInput, nftables.ChainPolicyAccept)
				f.rule(in, 1, "established",
					espFWOne(&expr.Ct{Key: expr.CtKeySTATE, Register: 1}),
					espFWOne(&expr.Bitwise{SourceRegister: 1, DestRegister: 1, Len: 4, Mask: []byte{6, 0, 0, 0}, Xor: make([]byte, 4)}),
					espFWOne(&expr.Cmp{Op: expr.CmpOpNeq, Register: 1, Data: make([]byte, 4)}),
					espFWCounter(), espFWVerdict(expr.VerdictAccept))
				f.rule(in, 2, "trusted", espFWL4Proto(expr.CmpOpEq, unix.IPPROTO_TCP), espFWDport(179), espFWNFProto(unix.NFPROTO_IPV4), espFWSaddr(192, 0, 2, 1), espFWCounter(), espFWVerdict(expr.VerdictAccept))
				f.rule(in, 3, "neighbor-v4", espFWL4Proto(expr.CmpOpEq, unix.IPPROTO_TCP), espFWDport(179),
					espFWOne(&expr.Payload{DestRegister: 1, Base: expr.PayloadBaseNetworkHeader, Offset: 12, Len: 4}),
					espFWOne(&expr.Lookup{SourceRegister: 1, SetName: "neighbor-v4"}),
					espFWJump(expr.VerdictJump, "bgp-neighbor"))
				f.rule(in, 4, "unknown-source-drop", espFWL4Proto(expr.CmpOpEq, unix.IPPROTO_TCP), espFWDport(179), espFWCounter(), espFWVerdict(expr.VerdictDrop))
			},
		},
		// gtsm has no row here: TestESPFormFirewallStockGTSMKeepsTheFastPathOn
		// (espform_firewall_stock_linux_test.go) judges the rules gtsm.FilterTables
		// yields after the nft backend lowered and committed them.
		{
			// vrrp, acceptFilterTables (internal/plugins/vrrp/acceptfilter.go): the
			// neighbor discovery carve-outs, then a drop per IPv6 virtual address.
			// MUTATION: refuse the nfproto load.
			name: "stock vrrp accept filter with an ipv6 address keeps the fast path on",
			build: func(f *espFWRuleset) {
				in := f.base(f.table(nftables.TableFamilyINet, "ze_vrrp_accept"), "input", espFWHookInput, nftables.ChainPolicyAccept)
				f.rule(in, 1, "", espFWL4Proto(expr.CmpOpEq, unix.IPPROTO_ICMPV6),
					espFWOne(&expr.Payload{DestRegister: 1, Base: expr.PayloadBaseTransportHeader, Offset: 0, Len: 1}),
					espFWOne(&expr.Cmp{Op: expr.CmpOpEq, Register: 1, Data: []byte{135}}), espFWVerdict(expr.VerdictAccept))
				f.rule(in, 2, "", espFWNFProto(unix.NFPROTO_IPV6),
					espFWOne(&expr.Payload{DestRegister: 1, Base: expr.PayloadBaseNetworkHeader, Offset: 24, Len: 16}),
					espFWOne(&expr.Cmp{Op: expr.CmpOpEq, Register: 1, Data: []byte{0x20, 0x01, 0x0d, 0xb8, 15: 1}}),
					espFWVerdict(expr.VerdictDrop))
			},
		},
		{
			// The same filter with an IPv4 virtual address drops ESP sent to it.
			// MUTATION: exclude on the nfproto ipv4 compare.
			name: "stock vrrp accept filter with an ipv4 address closes the fast path",
			build: func(f *espFWRuleset) {
				in := f.base(f.table(nftables.TableFamilyINet, "ze_vrrp_accept"), "input", espFWHookInput, nftables.ChainPolicyAccept)
				f.rule(in, 3, "", espFWNFProto(unix.NFPROTO_IPV4),
					espFWOne(&expr.Payload{DestRegister: 1, Base: expr.PayloadBaseNetworkHeader, Offset: 16, Len: 4}),
					espFWOne(&expr.Cmp{Op: expr.CmpOpEq, Register: 1, Data: []byte{192, 0, 2, 100}}),
					espFWVerdict(expr.VerdictDrop))
			},
			blocked: true,
			site:    espFormFirewallSite{Family: "inet", Table: "ze_vrrp_accept", Chain: "input", Hook: "input", Rule: "handle 3", Reason: espFormReasonDrop},
		},
		// ddos has no row here: TestESPFormFirewallStockDDoSNamedProtocolKeepsTheFastPathOn
		// and TestESPFormFirewallStockDDoSUnnamedProtocolClosesTheFastPath
		// (espform_firewall_stock_linux_test.go) judge the table local.MitigationTable
		// yields after the nft backend lowered and committed it.
		{
			// policyroute, buildPolicyMatch then buildActions
			// (internal/plugins/policyroute/translate.go): a rule naming only a
			// protocol opens with it, and its mark set is never reached by ESP.
			// MUTATION: answer false from espFormExcludesESP.
			name: "stock policy route on a protocol keeps the fast path on",
			build: func(f *espFWRuleset) {
				pre := f.base(f.table(nftables.TableFamilyINet, "ze_policyroute"), "prerouting", espFWHookPrerouting, nftables.ChainPolicyAccept)
				f.rule(pre, 1, "", espFWL4Proto(expr.CmpOpEq, unix.IPPROTO_TCP), espFWDport(443), espFWCounter(),
					espFWOne(&expr.Immediate{Register: 1, Data: []byte{0, 1, 0, 0}}),
					espFWOne(&expr.Meta{Key: expr.MetaKeyMARK, SourceRegister: true, Register: 1}),
					espFWVerdict(expr.VerdictAccept))
			},
		},
		{
			// A policy route naming a source address opens with the address match,
			// so its mark set can reach ESP.
			// MUTATION: name meta mark set harmless again.
			name: "stock policy route on a source address closes the fast path at its mark set",
			build: func(f *espFWRuleset) {
				pre := f.base(f.table(nftables.TableFamilyINet, "ze_policyroute"), "prerouting", espFWHookPrerouting, nftables.ChainPolicyAccept)
				f.rule(pre, 2, "", espFWNFProto(unix.NFPROTO_IPV4), espFWSaddr8(10), espFWCounter(),
					espFWOne(&expr.Immediate{Register: 1, Data: []byte{0, 1, 0, 0}}),
					espFWOne(&expr.Meta{Key: expr.MetaKeyMARK, SourceRegister: true, Register: 1}),
					espFWVerdict(expr.VerdictAccept))
			},
			blocked: true,
			site:    espFormFirewallSite{Family: "inet", Table: "ze_policyroute", Chain: "prerouting", Hook: "prerouting", Rule: "handle 2", Reason: espFormReasonStateSet},
		},
		{
			// flowexport's conntrack setup
			// (internal/plugins/flowexport/conntrack_setup_appliance_linux.go): a ct
			// state load and a counter.
			// MUTATION: classify a ct load off the harmless list.
			name: "stock flowexport conntrack rule keeps the fast path on",
			build: func(f *espFWRuleset) {
				pre := f.base(f.table(nftables.TableFamilyINet, "ze_flowexport"), "prerouting", espFWHookPrerouting, nftables.ChainPolicyAccept)
				f.rule(pre, 1, "", espFWOne(&expr.Ct{Key: expr.CtKeySTATE, Register: 1}), espFWCounter())
			},
		},
		{
			// flowspec-firewall, the flowspec-in base chain
			// (internal/plugins/flowspec-firewall/state.go, rule_chains.go): each
			// rule is an unconditional jump to its match chain.
			// MUTATION: answer Passes for a jump.
			name: "stock flowspec with a rule closes the fast path at its jump",
			build: func(f *espFWRuleset) {
				tab := f.table(nftables.TableFamilyINet, "ze_flowspec")
				in := f.base(tab, "flowspec-in", espFWHookInput, nftables.ChainPolicyAccept)
				f.regular(tab, "fs-0-match")
				f.rule(in, 1, "fs-0", espFWJump(expr.VerdictJump, "fs-0-match"))
			},
			blocked: true,
			site:    espFormFirewallSite{Family: "inet", Table: "ze_flowspec", Chain: "flowspec-in", Hook: "input", Rule: `handle 1 "fs-0"`, Reason: espFormReasonJump},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newESPFWRuleset()
			tt.build(f)
			blocked, site, err := espFormFirewallBlocks(f)
			if err != nil {
				t.Fatalf("espFormFirewallBlocks: unexpected error %v", err)
			}
			if blocked != tt.blocked {
				t.Fatalf("blocked = %v, want %v (site %+v)", blocked, tt.blocked, site)
			}
			if site != tt.site {
				t.Fatalf("site = %+v, want %+v", site, tt.site)
			}
		})
	}
}

// TestESPFormFirewallReaderError proves a ruleset the reader cannot read is an
// error, never a verdict. Method: each read the walk makes fails in turn.
func TestESPFormFirewallReaderError(t *testing.T) {
	cause := errors.New("netlink: operation not permitted")

	f := newESPFWRuleset()
	f.chainsErr = cause
	if _, _, err := espFormFirewallBlocks(f); !errors.Is(err, cause) {
		t.Fatalf("list chains failure: err = %v, want it to wrap %v", err, cause)
	}

	f = newESPFWRuleset()
	f.base(f.table(nftables.TableFamilyIPv4, "filter"), "input", espFWHookInput, nftables.ChainPolicyAccept)
	f.rulesErr = cause
	blocked, _, err := espFormFirewallBlocks(f)
	if !errors.Is(err, cause) {
		t.Fatalf("get rules failure: err = %v, want it to wrap %v", err, cause)
	}
	if blocked {
		t.Fatalf("get rules failure: blocked = true beside the error, want the error alone")
	}
}

// TestESPFormFirewallGenerationBracket proves a judgment stands only when the ruleset
// generation read before its first dump equals the one read after its last: a commit
// between two dumps is a splice, refused whether the walk read the ruleset as safe or
// as blocked, and a generation that cannot be read is an error. Method: a fake
// reader answering a scripted generation sequence over a safe and a blocking ruleset.
// MUTATION: skip the compare of the second Generation read with the first in
// espFormFirewallBlocks, and the two moved-generation cases answer no error.
func TestESPFormFirewallGenerationBracket(t *testing.T) {
	safe := func() *espFWRuleset {
		f := newESPFWRuleset()
		f.base(f.table(nftables.TableFamilyINet, "a"), "input", espFWHookInput, nftables.ChainPolicyAccept)
		return f
	}
	blocking := func() *espFWRuleset {
		f := newESPFWRuleset()
		in := f.base(f.table(nftables.TableFamilyINet, "b"), "input", espFWHookInput, nftables.ChainPolicyAccept)
		f.rule(in, 1, "", espFWL4Proto(expr.CmpOpEq, unix.IPPROTO_ESP), espFWVerdict(expr.VerdictDrop))
		return f
	}

	f := safe()
	f.generations = []uint32{7, 7}
	blocked, _, err := espFormFirewallBlocks(f)
	if err != nil || blocked {
		t.Fatalf("one generation, safe ruleset: blocked %v, err %v, want neither", blocked, err)
	}
	if f.generationReads != 2 {
		t.Fatalf("one judgment read the generation %d times, want 2", f.generationReads)
	}

	f = safe()
	f.generations = []uint32{7, 8}
	blocked, _, err = espFormFirewallBlocks(f)
	if !errors.Is(err, errESPFormRulesetChanged) {
		t.Fatalf("moved generation, safe read: err = %v, want %v", err, errESPFormRulesetChanged)
	}
	if blocked {
		t.Fatalf("moved generation, safe read: blocked beside the error, want the error alone")
	}

	f = blocking()
	f.generations = []uint32{7, 8}
	blocked, site, err := espFormFirewallBlocks(f)
	if !errors.Is(err, errESPFormRulesetChanged) {
		t.Fatalf("moved generation, blocking read: err = %v, want %v", err, errESPFormRulesetChanged)
	}
	if blocked || site != (espFormFirewallSite{}) {
		t.Fatalf("moved generation, blocking read: blocked %v, site %v, want the error alone", blocked, site)
	}

	cause := errors.New("netlink: operation not permitted")
	f = safe()
	f.generationErr = cause
	if _, _, err := espFormFirewallBlocks(f); !errors.Is(err, cause) {
		t.Fatalf("unreadable generation: err = %v, want it to wrap %v", err, cause)
	}
}

// TestESPFormGenerationFromMessage proves the GETGEN reply decodes to its NFTA_GEN_ID,
// and that a reply of another type, one flagged NLM_F_DUMP_INTR, or one with no
// generation id is an error rather than generation zero.
// MUTATION: answer generation 0 with no error when NFTA_GEN_ID is absent.
func TestESPFormGenerationFromMessage(t *testing.T) {
	id := []netlink.Attribute{{Type: unix.NFTA_GEN_ID, Data: espFWU32(0x01020304)}}
	got, err := espFormGenerationFromMessage(espFWDumpMessage(t, unix.NFT_MSG_NEWGEN, nftables.TableFamilyUnspecified, 0, id))
	if err != nil || got != 0x01020304 {
		t.Fatalf("NEWGEN with an id: %#x, %v, want 0x01020304 and no error", got, err)
	}
	pid := []netlink.Attribute{{Type: unix.NFTA_GEN_PROC_PID, Data: espFWU32(42)}}
	if _, err := espFormGenerationFromMessage(espFWDumpMessage(t, unix.NFT_MSG_NEWGEN, nftables.TableFamilyUnspecified, 0, pid)); err == nil {
		t.Fatalf("NEWGEN with no id: no error, want one")
	}
	if _, err := espFormGenerationFromMessage(espFWDumpMessage(t, unix.NFT_MSG_NEWTABLE, nftables.TableFamilyUnspecified, 0, id)); err == nil {
		t.Fatalf("a NEWTABLE message: no error, want one")
	}
	_, err = espFormGenerationFromMessage(espFWDumpMessage(t, unix.NFT_MSG_NEWGEN, nftables.TableFamilyUnspecified, netlink.DumpInterrupted, id))
	if !errors.Is(err, errESPFormDumpInterrupted) {
		t.Fatalf("a flagged NEWGEN: err = %v, want %v", err, errESPFormDumpInterrupted)
	}
}

// TestESPFormFirewallChainWithoutTable proves a chain the reader answered with no
// table, or a nil entry in either list, fails the judgement rather than being
// skipped: a skipped base chain is a rule the walk never saw. Method: a ruleset whose
// only base chain would block, beside each malformed entry in turn.
// MUTATION: skip a chain with no table (the round-3 behavior) and the first case
// goes green with blocked = false and no error.
func TestESPFormFirewallChainWithoutTable(t *testing.T) {
	cases := []struct {
		name  string
		build func(f *espFWRuleset)
	}{
		{name: "a chain with no table", build: func(f *espFWRuleset) {
			f.chains = append(f.chains, &nftables.Chain{Name: "orphan", Hooknum: new(nftables.ChainHook(espFWHookInput))})
		}},
		{name: "a nil chain", build: func(f *espFWRuleset) { f.chains = append(f.chains, nil) }},
		{name: "a nil table", build: func(f *espFWRuleset) { f.tables = append(f.tables, nil) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newESPFWRuleset()
			f.base(f.table(nftables.TableFamilyIPv4, "filter"), "input", espFWHookInput, nftables.ChainPolicyDrop)
			tc.build(f)
			blocked, site, err := espFormFirewallBlocks(f)
			if err == nil {
				t.Fatalf("err = nil (blocked %v, site %+v), want an error", blocked, site)
			}
			if blocked {
				t.Fatalf("blocked = true beside the error, want the error alone")
			}
		})
	}
}

// espFWDumpMessage builds one nftables dump message of type msg and family, its
// attributes marshaled after the nfgenmsg header, with the header flags given.
func espFWDumpMessage(t *testing.T, msg uint16, family nftables.TableFamily, flags netlink.HeaderFlags, attrs []netlink.Attribute) netlink.Message {
	t.Helper()
	return netlink.Message{
		Header: netlink.Header{Type: netlink.HeaderType(unix.NFNL_SUBSYS_NFTABLES<<8 | msg), Flags: flags},
		Data:   append([]byte{byte(family), unix.NFNETLINK_V0, 0, 0}, espFWMarshal(t, attrs)...),
	}
}

// espFWChainAttrs is the attribute list the kernel dumps for a base chain of table
// "filter", chain "input": table, name, the nested hook and the policy. hook is the
// hook attribute's payload, so a case can break it.
func espFWChainAttrs(t *testing.T, hook []netlink.Attribute) []netlink.Attribute {
	t.Helper()
	return []netlink.Attribute{
		{Type: unix.NFTA_CHAIN_TABLE, Data: []byte("filter\x00")},
		{Type: unix.NFTA_CHAIN_NAME, Data: []byte("input\x00")},
		{Type: unix.NLA_F_NESTED | unix.NFTA_CHAIN_HOOK, Data: espFWMarshal(t, hook)},
		{Type: unix.NFTA_CHAIN_POLICY, Data: espFWU32(uint32(nftables.ChainPolicyDrop))},
	}
}

// TestESPFormTableAndChainFromMessage proves the table and chain dumps are judged as
// strictly as the rule dump: a message flagged NLM_F_DUMP_INTR, a message of another
// type, a chain with no table, a hook with no hook number and an attribute of the
// wrong size each fail the read. The vendored ListTables and ListChains hand the flag
// through, and chainFromMsg ignores both its decoder's error and hookFromMsg's.
// Method: decode well-formed NEWTABLE and NEWCHAIN messages, then each broken form.
func TestESPFormTableAndChainFromMessage(t *testing.T) {
	hook := []netlink.Attribute{
		{Type: unix.NFTA_HOOK_HOOKNUM, Data: espFWU32(unix.NF_INET_LOCAL_IN)},
		{Type: unix.NFTA_HOOK_PRIORITY, Data: espFWU32(0)},
	}
	tableAttrs := []netlink.Attribute{{Type: unix.NFTA_TABLE_NAME, Data: []byte("filter\x00")}}

	t.Run("a table decodes", func(t *testing.T) {
		table, err := espFormTableFromMessage(espFWDumpMessage(t, unix.NFT_MSG_NEWTABLE, nftables.TableFamilyINet, netlink.Multi, tableAttrs))
		if err != nil {
			t.Fatalf("espFormTableFromMessage: %v", err)
		}
		if table.Name != "filter" || table.Family != nftables.TableFamilyINet {
			t.Fatalf("table = %+v, want inet table filter", table)
		}
	})
	t.Run("a base chain decodes", func(t *testing.T) {
		c, err := espFormChainFromMessage(espFWDumpMessage(t, unix.NFT_MSG_NEWCHAIN, nftables.TableFamilyIPv4, netlink.Multi, espFWChainAttrs(t, hook)))
		if err != nil {
			t.Fatalf("espFormChainFromMessage: %v", err)
		}
		if c.Name != "input" || c.Table == nil || c.Table.Name != "filter" || c.Table.Family != nftables.TableFamilyIPv4 {
			t.Fatalf("chain = %+v (table %+v), want chain input of ip table filter", c, c.Table)
		}
		if c.Hooknum == nil || *c.Hooknum != unix.NF_INET_LOCAL_IN {
			t.Fatalf("hook = %v, want input (%d)", c.Hooknum, unix.NF_INET_LOCAL_IN)
		}
		if c.Policy == nil || *c.Policy != nftables.ChainPolicyDrop {
			t.Fatalf("policy = %v, want drop", c.Policy)
		}
	})
	t.Run("a regular chain decodes with no hook", func(t *testing.T) {
		c, err := espFormChainFromMessage(espFWDumpMessage(t, unix.NFT_MSG_NEWCHAIN, nftables.TableFamilyIPv4, netlink.Multi, []netlink.Attribute{
			{Type: unix.NFTA_CHAIN_TABLE, Data: []byte("filter\x00")},
			{Type: unix.NFTA_CHAIN_NAME, Data: []byte("blocklist\x00")},
		}))
		if err != nil {
			t.Fatalf("espFormChainFromMessage: %v", err)
		}
		if c.Hooknum != nil || c.Policy != nil {
			t.Fatalf("chain = %+v, want a regular chain: no hook, no policy", c)
		}
	})

	t.Run("a flagged table message fails", func(t *testing.T) {
		table, err := espFormTableFromMessage(espFWDumpMessage(t, unix.NFT_MSG_NEWTABLE, nftables.TableFamilyIPv4, netlink.Multi|netlink.DumpInterrupted, tableAttrs))
		if !errors.Is(err, errESPFormDumpInterrupted) {
			t.Fatalf("err = %v, want %v", err, errESPFormDumpInterrupted)
		}
		if table != nil {
			t.Fatalf("decoded a table %+v beside the error, want none", table)
		}
	})
	t.Run("a flagged chain message fails", func(t *testing.T) {
		c, err := espFormChainFromMessage(espFWDumpMessage(t, unix.NFT_MSG_NEWCHAIN, nftables.TableFamilyIPv4, netlink.Multi|netlink.DumpInterrupted, espFWChainAttrs(t, hook)))
		if !errors.Is(err, errESPFormDumpInterrupted) {
			t.Fatalf("err = %v, want %v", err, errESPFormDumpInterrupted)
		}
		if c != nil {
			t.Fatalf("decoded a chain %+v beside the error, want none", c)
		}
	})

	broken := []struct {
		name  string
		table bool // decode as a table, not a chain
		msg   func(t *testing.T) netlink.Message
		want  error // nil: any error
	}{
		{name: "a chain message read as a table", table: true, msg: func(t *testing.T) netlink.Message {
			return espFWDumpMessage(t, unix.NFT_MSG_NEWCHAIN, nftables.TableFamilyIPv4, 0, tableAttrs)
		}},
		{name: "a chain with no table", msg: func(t *testing.T) netlink.Message {
			return espFWDumpMessage(t, unix.NFT_MSG_NEWCHAIN, nftables.TableFamilyIPv4, 0, espFWChainAttrs(t, hook)[1:])
		}},
		{name: "a hook with no hook number", msg: func(t *testing.T) netlink.Message {
			return espFWDumpMessage(t, unix.NFT_MSG_NEWCHAIN, nftables.TableFamilyIPv4, 0, espFWChainAttrs(t, hook[1:]))
		}, want: errESPFormChainNoHookNum},
		{name: "a hook number of two octets", msg: func(t *testing.T) netlink.Message {
			return espFWDumpMessage(t, unix.NFT_MSG_NEWCHAIN, nftables.TableFamilyIPv4, 0, espFWChainAttrs(t, []netlink.Attribute{{Type: unix.NFTA_HOOK_HOOKNUM, Data: []byte{0, 1}}}))
		}},
		{name: "a policy of two octets", msg: func(t *testing.T) netlink.Message {
			attrs := espFWChainAttrs(t, hook)
			attrs[3].Data = []byte{0, 0}
			return espFWDumpMessage(t, unix.NFT_MSG_NEWCHAIN, nftables.TableFamilyIPv4, 0, attrs)
		}},
	}
	for _, tc := range broken {
		t.Run(tc.name+" fails", func(t *testing.T) {
			m := tc.msg(t)
			var err error
			if tc.table {
				var table *nftables.Table
				table, err = espFormTableFromMessage(m)
				if table != nil {
					t.Fatalf("decoded a table %+v beside the error, want none", table)
				}
			} else {
				var c *nftables.Chain
				c, err = espFormChainFromMessage(m)
				if c != nil {
					t.Fatalf("decoded a chain %+v beside the error, want none", c)
				}
			}
			if err == nil {
				t.Fatalf("err = nil, want an error")
			}
			if tc.want != nil && !errors.Is(err, tc.want) {
				t.Fatalf("err = %v, want %v", err, tc.want)
			}
		})
	}
}

// TestESPFormFirewallDumpsEachBaseChainOnce proves one judgment dumps each base
// chain it judges once and no other chain at all: no jump target, no chain on a
// hook bare ESP does not cross. Method: a ruleset with two judged base chains, one
// of them holding a TCP-guarded jump (excluded, so the walk reads on past it), the
// jump's target, and an output chain; the fake reader counts its GetRules calls.
func TestESPFormFirewallDumpsEachBaseChainOnce(t *testing.T) {
	f := newESPFWRuleset()
	filter := f.table(nftables.TableFamilyIPv4, "filter")
	in := f.base(filter, "input", espFWHookInput, nftables.ChainPolicyAccept)
	f.regular(filter, "tcp-checks")
	f.rule(in, 1, "", espFWL4Proto(expr.CmpOpEq, unix.IPPROTO_TCP), espFWJump(expr.VerdictJump, "tcp-checks"))
	f.rule(in, 2, "", espFWCounter())
	f.rule(f.chains[1], 3, "", espFWVerdict(expr.VerdictDrop))
	mangle := f.table(nftables.TableFamilyINet, "mangle")
	pre := f.base(mangle, "prerouting", espFWHookPrerouting, nftables.ChainPolicyAccept)
	f.rule(pre, 1, "", espFWCounter())
	f.base(mangle, "output", espFWHookOutput, nftables.ChainPolicyAccept)

	blocked, site, err := espFormFirewallBlocks(f)
	if err != nil {
		t.Fatalf("espFormFirewallBlocks: unexpected error %v", err)
	}
	if blocked {
		t.Fatalf("blocked = true (site %+v), want false", site)
	}
	want := map[espFWChainKey]int{
		{family: nftables.TableFamilyIPv4, table: "filter", chain: "input"}:      1,
		{family: nftables.TableFamilyINet, table: "mangle", chain: "prerouting"}: 1,
	}
	if len(f.dumps) != len(want) {
		t.Fatalf("dumped %d chains %v, want %d: %v", len(f.dumps), f.dumps, len(want), want)
	}
	for key, n := range want {
		if f.dumps[key] != n {
			t.Fatalf("chain %+v dumped %d times, want %d (all dumps %v)", key, f.dumps[key], n, f.dumps)
		}
	}
}

// TestESPFormFirewallSiteString proves the site reads as one line naming every
// field. Method: format a rule site and a policy site.
func TestESPFormFirewallSiteString(t *testing.T) {
	rule := espFormFirewallSite{Family: "inet", Table: "ze_filter", Chain: "input -> blocklist", Hook: "input", Rule: `handle 9 "bad-host"`, Reason: espFormReasonDrop}
	if got, want := rule.String(), `inet table ze_filter, chain input -> blocklist, hook input, handle 9 "bad-host": drop`; got != want {
		t.Fatalf("String() = %q, want %q", got, want)
	}
	policy := espFormFirewallSite{Family: "ip", Table: "filter", Chain: "input", Hook: "input", Rule: "policy drop", Reason: espFormReasonPolicyDrop}
	if got, want := policy.String(), "ip table filter, chain input, hook input, policy drop: policy drop"; got != want {
		t.Fatalf("String() = %q, want %q", got, want)
	}
}

// espFWElement marshals one expression as the kernel's list element carries it.
func espFWElement(t *testing.T, e expr.Any) netlink.Attribute {
	t.Helper()
	data, err := expr.Marshal(byte(nftables.TableFamilyNetdev), e)
	if err != nil {
		t.Fatalf("marshal %T: %v", e, err)
	}
	return netlink.Attribute{Type: unix.NLA_F_NESTED | unix.NFTA_LIST_ELEM, Data: data}
}

func espFWMarshal(t *testing.T, attrs []netlink.Attribute) []byte {
	t.Helper()
	b, err := netlink.MarshalAttributes(attrs)
	if err != nil {
		t.Fatalf("marshal attributes: %v", err)
	}
	return b
}

// TestESPFormRuleFromMessage proves the reader keeps every expression the kernel
// holds. The vendored library's GetRules drops an expression it has no type for
// (expr.go, exprsFromBytes: "continue // unsupported expression type"), so a
// netdev `fwd` would read as a rule with no verdict. Method: decode a NEWRULE
// message holding meta, cmp, dup (which the library's own decoder also drops),
// fwd (which it has no type for) and an accept verdict, and assert each one, in
// order, with the handle and the user data.
func TestESPFormRuleFromMessage(t *testing.T) {
	fwdData := espFWMarshal(t, []netlink.Attribute{{Type: 1, Data: []byte{0, 0, 0, 1}}})
	fwd := espFWMarshal(t, []netlink.Attribute{
		{Type: unix.NFTA_EXPR_NAME, Data: []byte("fwd\x00")},
		{Type: unix.NLA_F_NESTED | unix.NFTA_EXPR_DATA, Data: fwdData},
	})
	list := espFWMarshal(t, []netlink.Attribute{
		espFWElement(t, &expr.Meta{Key: expr.MetaKeyL4PROTO, Register: 1}),
		espFWElement(t, &expr.Cmp{Op: expr.CmpOpEq, Register: 1, Data: []byte{unix.IPPROTO_ESP}}),
		espFWElement(t, &expr.Dup{RegAddr: 1, RegDev: 2, IsRegDevSet: true}),
		{Type: unix.NLA_F_NESTED | unix.NFTA_LIST_ELEM, Data: fwd},
		espFWElement(t, &expr.Verdict{Kind: expr.VerdictAccept}),
	})
	handle := make([]byte, 8)
	binary.BigEndian.PutUint64(handle, 42)
	attrs := espFWMarshal(t, []netlink.Attribute{
		{Type: unix.NFTA_RULE_TABLE, Data: []byte("edge\x00")},
		{Type: unix.NFTA_RULE_CHAIN, Data: []byte("ingress\x00")},
		{Type: unix.NFTA_RULE_HANDLE, Data: handle},
		{Type: unix.NLA_F_NESTED | unix.NFTA_RULE_EXPRESSIONS, Data: list},
		{Type: unix.NFTA_RULE_USERDATA, Data: []byte("steer")},
	})
	msg := netlink.Message{
		Header: netlink.Header{Type: netlink.HeaderType(unix.NFNL_SUBSYS_NFTABLES<<8 | unix.NFT_MSG_NEWRULE)},
		Data:   append([]byte{byte(nftables.TableFamilyNetdev), unix.NFNETLINK_V0, 0, 0}, attrs...),
	}

	table := &nftables.Table{Name: "edge", Family: nftables.TableFamilyNetdev}
	chain := &nftables.Chain{Name: "ingress", Table: table}
	rule, err := espFormRuleFromMessage(table, chain, msg)
	if err != nil {
		t.Fatalf("espFormRuleFromMessage: %v", err)
	}
	if rule.Handle != 42 {
		t.Fatalf("handle = %d, want 42", rule.Handle)
	}
	if string(rule.UserData) != "steer" {
		t.Fatalf("user data = %q, want %q", rule.UserData, "steer")
	}
	if len(rule.Exprs) != 5 {
		t.Fatalf("decoded %d expressions, want 5: %#v", len(rule.Exprs), rule.Exprs)
	}
	if m, ok := rule.Exprs[0].(*expr.Meta); !ok || m.Key != expr.MetaKeyL4PROTO || m.Register != 1 {
		t.Fatalf("expression 0 = %#v, want meta l4proto into r1", rule.Exprs[0])
	}
	if c, ok := rule.Exprs[1].(*expr.Cmp); !ok || c.Op != expr.CmpOpEq || len(c.Data) != 1 || c.Data[0] != unix.IPPROTO_ESP {
		t.Fatalf("expression 1 = %#v, want cmp eq 50", rule.Exprs[1])
	}
	if _, ok := rule.Exprs[2].(*expr.Dup); !ok {
		t.Fatalf("expression 2 = %#v, want dup", rule.Exprs[2])
	}
	if o, ok := rule.Exprs[3].(*espFormOpaqueExpr); !ok || o.name != "fwd" {
		t.Fatalf("expression 3 = %#v, want the opaque fwd", rule.Exprs[3])
	}
	if v, ok := rule.Exprs[4].(*expr.Verdict); !ok || v.Kind != expr.VerdictAccept {
		t.Fatalf("expression 4 = %#v, want verdict accept", rule.Exprs[4])
	}

	if _, err := espFormRuleFromMessage(table, chain, netlink.Message{
		Header: netlink.Header{Type: netlink.HeaderType(unix.NFNL_SUBSYS_NFTABLES<<8 | unix.NFT_MSG_NEWCHAIN)},
		Data:   msg.Data,
	}); err == nil {
		t.Fatalf("a chain message decoded as a rule, want an error")
	}
}

// espFWRuleMessage wraps an expression list in a NEWRULE message of the netdev
// table "edge", chain "ingress", handle 7, carrying the header flags given.
func espFWRuleMessage(t *testing.T, list []byte, flags netlink.HeaderFlags) netlink.Message {
	t.Helper()
	handle := make([]byte, 8)
	binary.BigEndian.PutUint64(handle, 7)
	attrs := espFWMarshal(t, []netlink.Attribute{
		{Type: unix.NFTA_RULE_TABLE, Data: []byte("edge\x00")},
		{Type: unix.NFTA_RULE_CHAIN, Data: []byte("ingress\x00")},
		{Type: unix.NFTA_RULE_HANDLE, Data: handle},
		{Type: unix.NLA_F_NESTED | unix.NFTA_RULE_EXPRESSIONS, Data: list},
	})
	return netlink.Message{
		Header: netlink.Header{Type: netlink.HeaderType(unix.NFNL_SUBSYS_NFTABLES<<8 | unix.NFT_MSG_NEWRULE), Flags: flags},
		Data:   append([]byte{byte(nftables.TableFamilyNetdev), unix.NFNETLINK_V0, 0, 0}, attrs...),
	}
}

// espFWCtElement is a ct list element built from raw attributes, as the kernel
// dumps them, not through expr.Marshal.
func espFWCtElement(t *testing.T, attrs []netlink.Attribute) netlink.Attribute {
	t.Helper()
	ct := espFWMarshal(t, []netlink.Attribute{
		{Type: unix.NFTA_EXPR_NAME, Data: []byte("ct\x00")},
		{Type: unix.NLA_F_NESTED | unix.NFTA_EXPR_DATA, Data: espFWMarshal(t, attrs)},
	})
	return netlink.Attribute{Type: unix.NLA_F_NESTED | unix.NFTA_LIST_ELEM, Data: ct}
}

func espFWU32(v uint32) []byte {
	b := make([]byte, 4)
	binary.BigEndian.PutUint32(b, v)
	return b
}

// TestESPFormRuleFromMessageCtSet proves a kernel `ct zone set` decodes as a set
// and blocks. The vendored Ct decoder reads NFTA_CT_DREG and never NFTA_CT_SREG, so
// a set would read as a load into register 0, which the judgement counts a
// harmless selector. Method: a NEWRULE message whose ct element carries exactly
// what nft_ct_set_dump (net/netfilter/nft_ct.c, v7.0) emits, NFTA_CT_SREG then
// NFTA_CT_KEY, behind an immediate; the decoded ct must be a set from register 1,
// and the rule must judge unsafe as a state set. A ct load (NFTA_CT_DREG) must
// stay a load, and a ct naming both registers must decode as opaque.
func TestESPFormRuleFromMessageCtSet(t *testing.T) {
	table := &nftables.Table{Name: "edge", Family: nftables.TableFamilyNetdev}
	chain := &nftables.Chain{Name: "ingress", Table: table}
	decode := func(t *testing.T, attrs []netlink.Attribute) *nftables.Rule {
		t.Helper()
		list := espFWMarshal(t, []netlink.Attribute{
			espFWElement(t, &expr.Immediate{Register: 1, Data: []byte{0, 1}}),
			espFWCtElement(t, attrs),
		})
		rule, err := espFormRuleFromMessage(table, chain, espFWRuleMessage(t, list, 0))
		if err != nil {
			t.Fatalf("espFormRuleFromMessage: %v", err)
		}
		if len(rule.Exprs) != 2 {
			t.Fatalf("decoded %d expressions, want 2: %#v", len(rule.Exprs), rule.Exprs)
		}
		return rule
	}

	t.Run("ct zone set is a set and blocks", func(t *testing.T) {
		rule := decode(t, []netlink.Attribute{
			{Type: unix.NFTA_CT_SREG, Data: espFWU32(unix.NFT_REG_1)},
			{Type: unix.NFTA_CT_KEY, Data: espFWU32(unix.NFT_CT_ZONE)},
		})
		ct, ok := rule.Exprs[1].(*expr.Ct)
		if !ok {
			t.Fatalf("expression 1 = %#v, want ct", rule.Exprs[1])
		}
		if !ct.SourceRegister || ct.Register != unix.NFT_REG_1 || ct.Key != expr.CtKeyZONE {
			t.Fatalf("ct = %+v, want a zone set from register 1", ct)
		}
		j := espFormJudgeRule(table.Family, rule.Exprs)
		if j.kind != espFormRuleUnsafe || j.reason != espFormReasonStateSet {
			t.Fatalf("judgement = %+v, want unsafe %q", j, espFormReasonStateSet)
		}
	})

	t.Run("ct mark load stays a load", func(t *testing.T) {
		rule := decode(t, []netlink.Attribute{
			{Type: unix.NFTA_CT_DREG, Data: espFWU32(unix.NFT_REG_1)},
			{Type: unix.NFTA_CT_KEY, Data: espFWU32(unix.NFT_CT_MARK)},
		})
		ct, ok := rule.Exprs[1].(*expr.Ct)
		if !ok {
			t.Fatalf("expression 1 = %#v, want ct", rule.Exprs[1])
		}
		if ct.SourceRegister || ct.Register != unix.NFT_REG_1 || ct.Key != expr.CtKeyMARK {
			t.Fatalf("ct = %+v, want a mark load into register 1", ct)
		}
	})

	t.Run("ct naming both registers is opaque", func(t *testing.T) {
		rule := decode(t, []netlink.Attribute{
			{Type: unix.NFTA_CT_DREG, Data: espFWU32(unix.NFT_REG_1)},
			{Type: unix.NFTA_CT_SREG, Data: espFWU32(unix.NFT_REG_1)},
			{Type: unix.NFTA_CT_KEY, Data: espFWU32(unix.NFT_CT_MARK)},
		})
		o, ok := rule.Exprs[1].(*espFormOpaqueExpr)
		if !ok || !errors.Is(o.err, errESPFormCtRegisters) {
			t.Fatalf("expression 1 = %#v, want the opaque ct carrying %v", rule.Exprs[1], errESPFormCtRegisters)
		}
	})
}

// TestESPFormRuleFromMessageDumpInterrupted proves a rule message the kernel
// flagged NLM_F_DUMP_INTR fails the read: a dump spliced across a commit is never
// a ruleset judged safe. Method: decode one well-formed rule message without the
// flag, then the same message with it.
func TestESPFormRuleFromMessageDumpInterrupted(t *testing.T) {
	table := &nftables.Table{Name: "edge", Family: nftables.TableFamilyNetdev}
	chain := &nftables.Chain{Name: "ingress", Table: table}
	list := espFWMarshal(t, []netlink.Attribute{espFWElement(t, &expr.Verdict{Kind: expr.VerdictAccept})})

	if _, err := espFormRuleFromMessage(table, chain, espFWRuleMessage(t, list, netlink.Multi)); err != nil {
		t.Fatalf("an unflagged rule message: %v, want it decoded", err)
	}
	rule, err := espFormRuleFromMessage(table, chain, espFWRuleMessage(t, list, netlink.Multi|netlink.DumpInterrupted))
	if !errors.Is(err, errESPFormDumpInterrupted) {
		t.Fatalf("a flagged rule message: err = %v, want %v", err, errESPFormDumpInterrupted)
	}
	if rule != nil {
		t.Fatalf("a flagged rule message decoded a rule %+v beside the error, want none", rule)
	}
}

// espFWWait fails the test when ch neither yields nor closes in time. It is a
// guard against a hang, never a wait for a condition: every send the test makes
// has already been taken by the worker when it is called.
func espFWWait(t *testing.T, ch <-chan struct{}, what string) (open bool) {
	t.Helper()
	select {
	case _, open = <-ch:
		return open
	case <-time.After(10 * time.Second):
		t.Fatalf("%s: nothing after 10s", what)
		return false
	}
}

// espFWMonitor is the seam the watch reads: the generational monitor's channel,
// and its Close, which ends the channel as the library does once its socket closes.
type espFWMonitor struct {
	events chan *nftables.MonitorEvents
	stops  int
}

func (m *espFWMonitor) stop() error {
	m.stops++
	if m.stops == 1 {
		close(m.events)
	}
	return nil
}

func espFWCommit() *nftables.MonitorEvents {
	return &nftables.MonitorEvents{GeneratedBy: &nftables.MonitorEvent{Type: nftables.MonitorEventType(unix.NFT_MSG_NEWGEN)}}
}

// TestESPFormNFTWatch proves the watch turns commits into one coalesced signal and
// closes its channel when the monitor ends. Method: drive the monitor's channel
// directly. The channel is unbuffered, so a send returns only once the worker took
// it, and each send that follows proves the worker finished the one before.
func TestESPFormNFTWatch(t *testing.T) {
	t.Run("commits coalesce and Close ends the channel", func(t *testing.T) {
		m := &espFWMonitor{events: make(chan *nftables.MonitorEvents)}
		w := newESPFormNFTWatch(m.events, m.stop)
		m.events <- espFWCommit()
		m.events <- espFWCommit()
		m.events <- espFWCommit()
		w.Close()
		if !espFWWait(t, w.signal, "first read") {
			t.Fatalf("three commits gave no signal, want one")
		}
		if espFWWait(t, w.signal, "second read") {
			t.Fatalf("three commits gave two signals, want one")
		}
		if err := w.Err(); err != nil {
			t.Fatalf("Err after Close = %v, want nil", err)
		}
		if m.stops != 1 {
			t.Fatalf("monitor stopped %d times, want 1", m.stops)
		}
		w.Close()
		if m.stops != 1 {
			t.Fatalf("a second Close stopped the monitor again (%d stops)", m.stops)
		}
	})

	t.Run("a read signal leaves room for the next commit", func(t *testing.T) {
		m := &espFWMonitor{events: make(chan *nftables.MonitorEvents)}
		w := newESPFormNFTWatch(m.events, m.stop)
		m.events <- espFWCommit()
		m.events <- espFWCommit()
		if !espFWWait(t, w.signal, "first commit") {
			t.Fatalf("channel closed, want a signal")
		}
		m.events <- espFWCommit()
		m.events <- espFWCommit()
		if !espFWWait(t, w.signal, "commit after the read") {
			t.Fatalf("channel closed, want a signal for the commit after the read")
		}
		w.Close()
		// The last commit may have signaled after the read above: at most one
		// signal is left, then the channel is closed.
		if espFWWait(t, w.signal, "after Close") && espFWWait(t, w.signal, "after the last signal") {
			t.Fatalf("two signals after Close, want at most one and then the channel closed")
		}
	})

	t.Run("a monitor failure closes the channel and names the cause", func(t *testing.T) {
		m := &espFWMonitor{events: make(chan *nftables.MonitorEvents)}
		w := newESPFormNFTWatch(m.events, m.stop)
		cause := errors.New("netlink receive: no buffer space available")
		m.events <- &nftables.MonitorEvents{GeneratedBy: &nftables.MonitorEvent{Type: nftables.MonitorEventTypeOOB, Error: cause}}
		close(m.events) // the library closes its channel after the failure batch
		m.stops = 1     // the channel is already closed: stop must not close it again
		if espFWWait(t, w.signal, "after the failure") {
			t.Fatalf("the failure gave a signal, want the channel closed")
		}
		if err := w.Err(); !errors.Is(err, cause) {
			t.Fatalf("Err = %v, want %v", err, cause)
		}
		w.Close()
	})

	t.Run("a monitor that ends with no failure still names a cause", func(t *testing.T) {
		m := &espFWMonitor{events: make(chan *nftables.MonitorEvents)}
		w := newESPFormNFTWatch(m.events, m.stop)
		close(m.events)
		m.stops = 1
		if espFWWait(t, w.signal, "after the end") {
			t.Fatalf("a signal, want the channel closed")
		}
		if err := w.Err(); !errors.Is(err, errESPFormNFTWatchEnded) {
			t.Fatalf("Err = %v, want %v", err, errESPFormNFTWatchEnded)
		}
		w.Close()
	})
}

// TestESPFormNFTWatchReceiveBuffer proves the watch's socket takes the receive buffer
// a large commit needs, past net.core.rmem_max. Method: a NETLINK_NETFILTER socket,
// the option the watch dials with, and the size the kernel reports back, which it
// doubles from what was set.
func TestESPFormNFTWatchReceiveBuffer(t *testing.T) {
	conn, err := netlink.Dial(unix.NETLINK_NETFILTER, nil)
	if err != nil {
		t.Fatalf("open a netfilter socket: %v", err)
	}
	defer conn.Close() //nolint:errcheck // the test is done with the socket

	err = forceESPFormNFTWatchBuffer(conn)
	if errors.Is(err, unix.EPERM) {
		t.Skip("SO_RCVBUFFORCE needs CAP_NET_ADMIN, which this run does not hold; `./le test qemu all` runs it as root")
	}
	if err != nil {
		t.Fatalf("set the watch's receive buffer: %v", err)
	}
	got, err := conn.ReadBuffer()
	if err != nil {
		t.Fatalf("read the receive buffer back: %v", err)
	}
	if got < espFormNFTWatchBufferOctets {
		t.Errorf("the receive buffer is %d octets, want at least %d", got, espFormNFTWatchBufferOctets)
	}
	if err := espFormNFTWatchBuffer(conn); err != nil {
		t.Errorf("the watch's socket option answered %v, want nil: it must never fail the dial", err)
	}
}

// espFWCloseAfter closes updates once done is closed, as the link subscription's
// library does after stop closed done.
func espFWCloseAfter(done <-chan struct{}, updates chan<- linknl.LinkUpdate) {
	<-done
	close(updates)
}

// TestESPFormFastPathStopDuringJudgment proves a judgment that is mid-dump when stop
// starts, and finds the ruleset safe, does not leave the gate open once stop
// returns: the watch is closed then, so nothing would see a rule committed after.
// stop steps the epoch with its first shut, so that judgment publishes nothing, and
// shuts the gate again once the worker returned, so a judgment that read the stepped
// epoch before it saw done is undone too.
//
// Method: the worker serves as start leaves it, with stop's done and exited
// installed. The judge seam says it is reading, then answers safe only once stop has
// closed done. The test reads the gate's writes and the state after stop.
// MUTATION: drop both the epoch step and the second shut from stop, and the last
// write is 1 with GateOpen true. Either one alone keeps this test green: in this
// interleaving the judgment read the epoch before the step.
func TestESPFormFastPathStopDuringJudgment(t *testing.T) {
	f, writes := newGateTestFastPath(t, &gateTestReader{}, nil)
	done, exited := make(chan struct{}), make(chan struct{})
	updates := make(chan linknl.LinkUpdate)
	reading := make(chan struct{})
	judge := &gateTestJudge{pass: func(call int) gateTestVerdict {
		if call == 0 {
			close(reading)
			<-done // stop has shut the gate and closed done
		}
		return gateTestVerdict{}
	}}
	f.judge = judge.judge
	f.mu.Lock()
	f.done, f.exited = done, exited
	f.mu.Unlock()
	go espFWCloseAfter(done, updates)
	go gateTestServe(f, updates, nil, done, gateTestNoRewatch(t), exited)

	espFWWait(t, reading, "the judgment's dump")
	// Bounded, so a worker that never returns fails here by name rather than hang the
	// test binary in stop's wait for it.
	stopped := make(chan struct{})
	go gateTestStopInto(f.stop, stopped)
	select {
	case <-stopped:
	case <-time.After(gateTestWait):
		t.Fatalf("stop did not return within %s of a judgment it interrupted: the worker kept running", gateTestWait)
	}

	got := writes.all()
	if len(got) == 0 {
		t.Fatalf("stop wrote nothing into the gate, want it shut")
	}
	if last := got[len(got)-1]; last != espFormFastPathGateClosed {
		t.Fatalf("gate writes %v end with %d, want %d: stop left the gate open with nothing watching", got, last, espFormFastPathGateClosed)
	}
	if st := gateTestState(t, f); st.GateOpen {
		t.Fatalf("GateOpen after stop, want false: %+v", st)
	}
	if n := judge.count(); n != 1 {
		t.Fatalf("the ruleset was judged %d times, want 1: the worker judged again after stop closed done", n)
	}
}
