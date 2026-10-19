// Design: docs/features/interfaces.md -- IPv6 reverse-path check
// Overview: backend_linux.go -- netlinkBackend

//go:build linux

package ifacenetlink

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/google/nftables"
	"github.com/google/nftables/expr"
	"github.com/mdlayher/netlink"

	"github.com/ze-software/ze/internal/component/iface"
)

// Linux has no IPv6 rp_filter sysctl, so the IPv6 reverse-path check is an
// nftables fib lookup in prerouting, in a table this backend owns alone:
//
//	table ip6 ze_iface_rpf {
//	    set strict { type ifname }
//	    set loose  { type ifname }
//	    chain prerouting {
//	        type filter hook prerouting priority raw; policy accept;
//	        ip6 saddr fe80::/10 accept
//	        ip6 saddr :: accept
//	        iifname @strict fib saddr . iif oif missing counter drop
//	        iifname @loose  fib saddr oif missing counter drop
//	    }
//	}
//
// The interfaces are set members, so one rule serves every interface of a mode
// and the per-packet cost does not grow with the interface count. The table is
// rebuilt whole on every call, in one netlink batch, so the kernel never holds
// a half-built table and never keeps a member the config no longer names.
const (
	rpfTableName   = "ze_iface_rpf"
	rpfChainName   = "prerouting"
	rpfStrictSet   = "strict"
	rpfLooseSet    = "loose"
	rpfNetlinkWait = 10 * time.Second
)

// ip6SaddrOffset is the offset of the source address in the IPv6 header:
// version/class/flow (4), payload length (2), next header (1), hop limit (1).
const ip6SaddrOffset = 8

// ReplaceIPv6RPF rebuilds the ze_iface_rpf table so it enforces exactly
// checks. An empty slice deletes the table.
func (b *netlinkBackend) ReplaceIPv6RPF(checks []iface.RPFCheck) error {
	conn, err := nftables.New(nftables.WithSockOptions(func(c *netlink.Conn) error {
		return c.SetDeadline(time.Now().Add(rpfNetlinkWait))
	}))
	if err != nil {
		return fmt.Errorf("ipv6 rpf-check: open nftables: %w", err)
	}
	if err := queueIPv6RPF(conn, checks); err != nil {
		return err
	}
	err = conn.Flush()
	if err == nil {
		return nil
	}
	if len(checks) == 0 && errors.Is(err, os.ErrPermission) {
		// Guard: an empty set asks only that no table remain, and every
		// config without an IPv6 rpf-check sends one. nftables refuses even
		// the delete without CAP_NET_ADMIN, so refusing here would abort
		// every interface apply of an unprivileged ze over a check nobody
		// configured. Such a process cannot remove a table a privileged run
		// left behind either, so the warning names the residue it cannot see.
		logger().Warn("ipv6 rpf-check: cannot confirm no stale nftables table remains, "+
			"no check is configured and removing one needs CAP_NET_ADMIN; "+
			"if a privileged run left it, remove it with `nft delete table ip6 "+rpfTableName+"`",
			"error", err)
		return nil
	}
	return fmt.Errorf("ipv6 rpf-check: program nftables table %s: %w", rpfTableName, err)
}

// ReplaceIPv4RPF accepts the IPv4 set and installs nothing for it, because
// this backend's traffic is forwarded by the Linux kernel, and the kernel
// already enforces each check: the same leaf reaches
// net.ipv4.conf.<dev>.rp_filter through the iface component's sysctl phase,
// which judges every packet this backend forwards.
func (b *netlinkBackend) ReplaceIPv4RPF(_ []iface.RPFCheck) error {
	return nil
}

// queueIPv6RPF queues the batch that replaces the table. Adding the table
// before deleting it makes the delete succeed whether or not the table exists,
// which is the nft idiom for "replace" without a separate read.
func queueIPv6RPF(conn *nftables.Conn, checks []iface.RPFCheck) error {
	table := &nftables.Table{Family: nftables.TableFamilyIPv6, Name: rpfTableName}
	conn.AddTable(table)
	conn.DelTable(table)
	if len(checks) == 0 {
		return nil
	}

	var strict, loose []nftables.SetElement
	for _, c := range checks {
		switch c.Mode {
		case iface.RPFStrict:
			strict = append(strict, nftables.SetElement{Key: ifnameKey(c.Interface)})
		case iface.RPFLoose:
			loose = append(loose, nftables.SetElement{Key: ifnameKey(c.Interface)})
		case iface.RPFUnspecified:
			// desiredIPv6RPF never emits Unspecified; a check without a mode is
			// a caller defect, refused rather than read as either mode.
			return fmt.Errorf("ipv6 rpf-check: interface %q has no mode", c.Interface)
		}
	}

	conn.AddTable(table)
	chain := conn.AddChain(&nftables.Chain{
		Name:     rpfChainName,
		Table:    table,
		Type:     nftables.ChainTypeFilter,
		Hooknum:  nftables.ChainHookPrerouting,
		Priority: nftables.ChainPriorityRaw,
		Policy:   new(nftables.ChainPolicyAccept),
	})
	for _, exprs := range rpfExemptRules() {
		conn.AddRule(&nftables.Rule{Table: table, Chain: chain, Exprs: exprs})
	}
	if err := addRPFModeRule(conn, table, chain, rpfStrictSet, strict, true); err != nil {
		return err
	}
	return addRPFModeRule(conn, table, chain, rpfLooseSet, loose, false)
}

// addRPFModeRule queues one mode's interface set and the rule that drops what
// fails that mode's lookup. A mode no interface uses gets neither.
func addRPFModeRule(conn *nftables.Conn, table *nftables.Table, chain *nftables.Chain, name string, members []nftables.SetElement, strict bool) error {
	if len(members) == 0 {
		return nil
	}
	set := &nftables.Set{Table: table, Name: name, KeyType: nftables.TypeIFName}
	if err := conn.AddSet(set, members); err != nil {
		return fmt.Errorf("ipv6 rpf-check: set %s: %w", name, err)
	}
	conn.AddRule(&nftables.Rule{Table: table, Chain: chain, Exprs: rpfDropExprs(set, strict)})
	return nil
}

// rpfExemptRules returns the rules that accept the two sources a reverse-path
// lookup cannot judge. A link-local source (fe80::/10) is only meaningful on
// its own link and a router never forwards it. The unspecified source (::) is
// what duplicate address detection sends from. RFC 4862 Section 5.4.2: "the IP
// source is set to the unspecified address". Neither has a route that a
// reverse-path lookup can judge in every case. Measured on Linux 7.2 the
// kernel's fib expression skips the lookup for the multicast-destined DAD and
// router solicitation, and a link-local source resolves through the
// interface's fe80::/64 route. Unicast neighbor probes get no such skip, an
// interface can run without the fe80::/64 route, and older kernels skip less,
// so the exemptions are what keep neighbor discovery and DHCPv6 working
// whatever the kernel does.
func rpfExemptRules() [][]expr.Any {
	return [][]expr.Any{
		{
			&expr.Payload{DestRegister: 1, Base: expr.PayloadBaseNetworkHeader, Offset: ip6SaddrOffset, Len: 2},
			&expr.Bitwise{SourceRegister: 1, DestRegister: 1, Len: 2, Mask: []byte{0xff, 0xc0}, Xor: []byte{0, 0}},
			&expr.Cmp{Op: expr.CmpOpEq, Register: 1, Data: []byte{0xfe, 0x80}},
			&expr.Verdict{Kind: expr.VerdictAccept},
		},
		{
			&expr.Payload{DestRegister: 1, Base: expr.PayloadBaseNetworkHeader, Offset: ip6SaddrOffset, Len: 16},
			&expr.Cmp{Op: expr.CmpOpEq, Register: 1, Data: make([]byte, 16)},
			&expr.Verdict{Kind: expr.VerdictAccept},
		},
	}
}

// rpfDropExprs is `iifname @set fib saddr [. iif] oif missing counter drop`.
//
// RFC 3704 Section 2.2: "the source address is looked up in the Forwarding
// Information Base (FIB) - and if the packet is received on the interface
// which would be used to forward the traffic to the source of the packet, it
// passes the check." The iif flag restricts the lookup to the arrival
// interface, so a missing result is that failure.
//
// RFC 3704 Section 2.4: Loose RPF "checks only for the existence of a route
// (even a default route, if applicable), not where the route points to." The
// loose lookup carries no iif flag, so any route, the default one included,
// passes.
func rpfDropExprs(set *nftables.Set, strict bool) []expr.Any {
	return []expr.Any{
		&expr.Meta{Key: expr.MetaKeyIIFNAME, Register: 1},
		&expr.Lookup{SourceRegister: 1, SetName: set.Name, SetID: set.ID},
		&expr.Fib{Register: 1, FlagSADDR: true, FlagIIF: strict, ResultOIF: true, FlagPRESENT: true},
		&expr.Cmp{Op: expr.CmpOpEq, Register: 1, Data: make([]byte, 4)},
		&expr.Counter{},
		&expr.Verdict{Kind: expr.VerdictDrop},
	}
}

// ifnameKey is name as an nftables ifname set key: IFNAMSIZ bytes, NUL padded.
func ifnameKey(name string) []byte {
	key := make([]byte, 16)
	copy(key, name)
	return key
}
