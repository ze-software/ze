// Design: docs/architecture/firewall/fw-6-firewall-vpp.md -- NAT44-ED integration

//go:build linux

package firewallvpp

import (
	"fmt"
	"maps"
	"net/netip"
	"slices"
	"strings"

	"go.fd.io/govpp/binapi/interface_types"
	"go.fd.io/govpp/binapi/ip_types"

	"github.com/ze-software/ze/internal/component/firewall"
)

const natTagPrefix = "ze/"

func (b *backend) applyNATChains(
	ops vppOps,
	desired []firewall.Table,
	nameIndex map[string]interface_types.InterfaceIndex,
) error {
	var hasNAT bool
	for i := range desired {
		for j := range desired[i].Chains {
			if desired[i].Chains[j].Type == firewall.ChainNAT {
				hasNAT = true
				break
			}
		}
	}
	if !hasNAT {
		return nil
	}

	if err := ops.nat44Enable(); err != nil {
		if !strings.Contains(err.Error(), "already enabled") &&
			!strings.Contains(err.Error(), "instance already") {
			return fmt.Errorf("firewall-vpp: nat44 enable: %w", err)
		}
	}

	if err := b.cleanupNATOrphans(ops, desired); err != nil {
		return fmt.Errorf("firewall-vpp: nat cleanup: %w", err)
	}

	for i := range desired {
		tbl := &desired[i]
		for j := range tbl.Chains {
			ch := &tbl.Chains[j]
			if ch.Type != firewall.ChainNAT {
				continue
			}
			if err := b.applyNATChain(ops, tbl, ch, nameIndex); err != nil {
				return fmt.Errorf("firewall-vpp: table %q chain %q: %w", tbl.Name, ch.Name, err)
			}
		}
	}
	return nil
}

func (b *backend) applyNATChain(
	ops vppOps,
	tbl *firewall.Table,
	ch *firewall.Chain,
	nameIndex map[string]interface_types.InterfaceIndex,
) error {
	var undo []func()
	for i := range ch.Terms {
		term := &ch.Terms[i]
		for _, a := range term.Actions {
			switch v := a.(type) {
			case firewall.Masquerade:
				if err := b.applyMasquerade(ops, nameIndex); err != nil {
					undoNAT(undo)
					return fmt.Errorf("term %q masquerade: %w", term.Name, err)
				}
				captured := maps.Clone(nameIndex)
				undo = append(undo, func() {
					for _, idx := range captured {
						_ = ops.nat44AddDelOutputInterface(idx, false)
					}
				})
			case firewall.SNAT:
				if err := b.applySNAT(ops, v, nameIndex); err != nil {
					undoNAT(undo)
					return fmt.Errorf("term %q snat: %w", term.Name, err)
				}
				capturedSNAT := v
				capturedIfaces := maps.Clone(nameIndex)
				undo = append(undo, func() {
					first := addrToIP4(capturedSNAT.Address)
					last := first
					if capturedSNAT.AddressEnd.IsValid() && capturedSNAT.AddressEnd != capturedSNAT.Address {
						last = addrToIP4(capturedSNAT.AddressEnd)
					}
					_ = ops.nat44AddDelAddressRange(first, last, false)
					for _, idx := range capturedIfaces {
						_ = ops.nat44AddDelInterfaceFeature(idx, true, false)
					}
				})
			case firewall.DNAT:
				tag := natTag(tbl.Name, ch.Name, term.Name)
				mapping := buildDNATMapping(v, term, tag)
				if err := b.applyDNAT(ops, mapping); err != nil {
					undoNAT(undo)
					return fmt.Errorf("term %q dnat: %w", term.Name, err)
				}
				undoMapping := mapping
				undoMapping.IsAdd = false
				undo = append(undo, func() {
					_ = ops.nat44AddDelStaticMapping(undoMapping)
				})
			}
		}
	}
	return nil
}

func undoNAT(undo []func()) {
	for _, step := range slices.Backward(undo) {
		step()
	}
}

// applyMasquerade marks all VPP interfaces as NAT44 output interfaces.
// This is the VPP equivalent of an nftables postrouting masquerade chain,
// which also applies to all outgoing traffic. The verifier rejects
// per-interface match constraints for SNAT/masquerade.
func (b *backend) applyMasquerade(
	ops vppOps,
	nameIndex map[string]interface_types.InterfaceIndex,
) error {
	for _, swIfIndex := range nameIndex {
		if err := ops.nat44AddDelOutputInterface(swIfIndex, true); err != nil {
			if !strings.Contains(err.Error(), "already") {
				return err
			}
		}
	}
	return nil
}

func (b *backend) applySNAT(
	ops vppOps,
	snat firewall.SNAT,
	nameIndex map[string]interface_types.InterfaceIndex,
) error {
	first := addrToIP4(snat.Address)
	last := first
	if snat.AddressEnd.IsValid() && snat.AddressEnd != snat.Address {
		last = addrToIP4(snat.AddressEnd)
	}
	if err := ops.nat44AddDelAddressRange(first, last, true); err != nil {
		return err
	}
	for _, swIfIndex := range nameIndex {
		if err := ops.nat44AddDelInterfaceFeature(swIfIndex, true, true); err != nil {
			if !strings.Contains(err.Error(), "already") {
				return err
			}
		}
	}
	return nil
}

func buildDNATMapping(dnat firewall.DNAT, term *firewall.Term, tag string) natStaticMapping {
	var proto uint8
	var extPort uint16
	for _, m := range term.Matches {
		switch v := m.(type) {
		case firewall.MatchProtocol:
			// Unknown protocols leave proto at 0; known protocols use the
			// shared IANA table (the former inline switch handled only tcp/udp
			// and silently programmed 0 for everything else).
			if num, ok := firewall.ProtocolNumber(v.Protocol); ok {
				proto = num
			}
		case firewall.MatchDestinationPort:
			if len(v.Ranges) > 0 {
				extPort = v.Ranges[0].Lo
			}
		}
	}

	localPort := dnat.Port
	if localPort == 0 {
		localPort = extPort
	}

	return natStaticMapping{
		IsAdd:             true,
		Tag:               tag,
		Protocol:          proto,
		LocalAddr:         addrToIP4(dnat.Address),
		LocalPort:         localPort,
		ExternalAddr:      ip_types.IP4Address{},
		ExternalPort:      extPort,
		ExternalSwIfIndex: ^interface_types.InterfaceIndex(0),
	}
}

func (b *backend) applyDNAT(ops vppOps, m natStaticMapping) error {
	return ops.nat44AddDelStaticMapping(m)
}

func (b *backend) cleanupNATOrphans(ops vppOps, desired []firewall.Table) error {
	desiredTags := make(map[string]bool)
	for i := range desired {
		for j := range desired[i].Chains {
			ch := &desired[i].Chains[j]
			if ch.Type != firewall.ChainNAT {
				continue
			}
			for k := range ch.Terms {
				desiredTags[natTag(desired[i].Name, ch.Name, ch.Terms[k].Name)] = true
			}
		}
	}

	existing, err := ops.nat44StaticMappingDump()
	if err != nil {
		return err
	}
	for _, m := range existing {
		if !isNATTag(m.Tag) {
			continue
		}
		if desiredTags[m.Tag] {
			continue
		}
		m.IsAdd = false
		if err := ops.nat44AddDelStaticMapping(m); err != nil {
			lg := logger()
			lg.Warn("firewall-vpp: delete orphan NAT mapping failed", "tag", m.Tag, "err", err)
		}
	}
	return nil
}

// isNATTag returns true if tag matches the NAT format ze/<table>/<chain>/<term>
// (4 segments), distinguishing from ACL tags ze/<table>/<chain> (3 segments)
// and classify policer names ze/fw/<table>/<chain>/<term> (5 segments).
func isNATTag(tag string) bool {
	return strings.HasPrefix(tag, natTagPrefix) &&
		!strings.HasPrefix(tag, "ze/fw/") &&
		strings.Count(tag, "/") >= 3
}

func addrToIP4(a netip.Addr) ip_types.IP4Address {
	if !a.IsValid() || !a.Is4() {
		return ip_types.IP4Address{}
	}
	return ip_types.IP4Address(a.As4())
}
