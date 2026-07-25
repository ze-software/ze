// Design: plan/learned/913-firewall-irr.md -- interval set generation from cached prefixes

package irr

import (
	"net/netip"

	"github.com/ze-software/ze/internal/component/firewall"
	"github.com/ze-software/ze/internal/component/resolve/irr/store"
	"github.com/ze-software/ze/internal/core/textbuf"
)

const (
	maxPrefixEntries = 500_000
	ifaceTableName   = "ze_irr_iface"
	ifaceChainName   = "irr_iface_ingress"
	ifaceChainPrio   = -10
)

func setNames(name string) (v4, v6 string) {
	var tb textbuf.Buffer
	v4 = tb.Str("irr_v4_").Str(name).String()
	tb.Reset()
	v6 = tb.Str("irr_v6_").Str(name).String()
	return v4, v6
}

func buildSets(name string, v4, v6 []netip.Prefix) []firewall.Set {
	v4Name, v6Name := setNames(name)
	var sets []firewall.Set

	if len(v4) > 0 {
		elements := prefixesToIntervalElements(v4, maxPrefixEntries)
		sets = append(sets, firewall.Set{
			Name:     v4Name,
			Type:     firewall.SetTypeIPv4,
			Flags:    firewall.SetFlagInterval,
			Elements: elements,
		})
	}
	if len(v6) > 0 {
		remaining := max(maxPrefixEntries-len(v4), 0)
		elements := prefixesToIntervalElements(v6, remaining)
		sets = append(sets, firewall.Set{
			Name:     v6Name,
			Type:     firewall.SetTypeIPv6,
			Flags:    firewall.SetFlagInterval,
			Elements: elements,
		})
	}
	return sets
}

func prefixesToIntervalElements(prefixes []netip.Prefix, limit int) []firewall.SetElement {
	cap := min(len(prefixes), limit) * 2
	elements := make([]firewall.SetElement, 0, cap)
	for _, p := range prefixes {
		if len(elements)/2 >= limit {
			logger().Warn("firewall-irr: prefix list exceeds cap, truncating",
				"count", len(prefixes), "cap", limit)
			break
		}
		if p.Bits() == 0 {
			continue
		}
		start, end := prefixRange(p)
		elements = append(elements,
			firewall.SetElement{Value: start.String()},
			firewall.SetElement{Value: end.String(), IntervalEnd: true},
		)
	}
	return elements
}

func prefixRange(p netip.Prefix) (start, exclusiveEnd netip.Addr) {
	start = p.Masked().Addr()
	bits := p.Bits()
	if p.Addr().Is4() {
		a := start.As4()
		hostBits := 32 - bits
		mask := uint32(1)<<hostBits - 1
		base := (uint32(a[0])<<24 | uint32(a[1])<<16 | uint32(a[2])<<8 | uint32(a[3]))
		end := base | mask
		end++
		a[0] = byte(end >> 24)
		a[1] = byte(end >> 16)
		a[2] = byte(end >> 8)
		a[3] = byte(end)
		return start, netip.AddrFrom4(a)
	}
	a := start.As16()
	hostBits := 128 - bits
	carry := uint16(1)
	byteIdx := 15
	bitsRemaining := hostBits
	for bitsRemaining >= 8 && byteIdx >= 0 {
		a[byteIdx] = 0
		byteIdx--
		bitsRemaining -= 8
	}
	if bitsRemaining > 0 && byteIdx >= 0 {
		mask := byte(0xFF >> (8 - bitsRemaining))
		a[byteIdx] |= mask
		sum := uint16(a[byteIdx]) + carry
		a[byteIdx] = byte(sum)
		carry = sum >> 8
		byteIdx--
	}
	for carry > 0 && byteIdx >= 0 {
		sum := uint16(a[byteIdx]) + carry
		a[byteIdx] = byte(sum)
		carry = sum >> 8
		byteIdx--
	}
	return start, netip.AddrFrom16(a)
}

func buildIfaceTables(ps *store.PrefixStore, bindings []ifaceBinding) []firewall.Table {
	if len(bindings) == 0 {
		return nil
	}

	seenSets := make(map[string]bool)
	var sets []firewall.Set
	var terms []firewall.Term
	for _, ib := range bindings {
		entry := ps.Get(ib.ASSet)
		if entry == nil {
			continue
		}
		if !seenSets[ib.ASSet] {
			seenSets[ib.ASSet] = true
			sets = append(sets, buildSets(ib.ASSet, entry.IPv4, entry.IPv6)...)
		}

		v4Name, v6Name := setNames(ib.ASSet)
		if len(entry.IPv4) > 0 {
			terms = append(terms, firewall.Term{
				Name: ifaceTermName(ib.Interface, "v4"),
				Matches: []firewall.Match{
					firewall.MatchInputInterface{Name: ib.Interface},
					firewall.MatchInSet{SetName: v4Name, MatchField: firewall.SetFieldSourceAddr},
				},
				Actions: []firewall.Action{firewall.Accept{}},
			})
		}
		if len(entry.IPv6) > 0 {
			terms = append(terms, firewall.Term{
				Name: ifaceTermName(ib.Interface, "v6"),
				Matches: []firewall.Match{
					firewall.MatchInputInterface{Name: ib.Interface},
					firewall.MatchInSet{SetName: v6Name, MatchField: firewall.SetFieldSourceAddr},
				},
				Actions: []firewall.Action{firewall.Accept{}},
			})
		}
		terms = append(terms, firewall.Term{
			Name: ifaceTermName(ib.Interface, "drop"),
			Matches: []firewall.Match{
				firewall.MatchInputInterface{Name: ib.Interface},
			},
			Actions: []firewall.Action{firewall.Drop{}},
		})
	}

	if len(terms) == 0 {
		return nil
	}

	return []firewall.Table{{
		Name:   ifaceTableName,
		Family: firewall.FamilyInet,
		Sets:   sets,
		Chains: []firewall.Chain{{
			Name:     ifaceChainName,
			IsBase:   true,
			Type:     firewall.ChainFilter,
			Hook:     firewall.HookPrerouting,
			Priority: ifaceChainPrio,
			Policy:   firewall.PolicyAccept,
			Terms:    terms,
		}},
	}}
}

func ifaceTermName(iface, suffix string) string {
	var tb textbuf.Buffer
	return tb.Str("iface_").Str(iface).Byte('_').Str(suffix).String()
}

func buildIRRTables(ps *store.PrefixStore, refs []irrRef) []firewall.Table {
	byTable := make(map[string]*firewall.Table)
	var order []string
	for _, ref := range refs {
		entry := ps.Get(ref.Name)
		if entry == nil {
			continue
		}
		sets := buildSets(ref.Name, entry.IPv4, entry.IPv6)
		if len(sets) == 0 {
			continue
		}
		tbl, ok := byTable[ref.TableName]
		if !ok {
			tbl = &firewall.Table{
				Name:   ref.TableName,
				Family: firewall.FamilyInet,
			}
			byTable[ref.TableName] = tbl
			order = append(order, ref.TableName)
		}
		tbl.Sets = append(tbl.Sets, sets...)
	}
	if len(order) == 0 {
		return nil
	}
	tables := make([]firewall.Table, len(order))
	for i, name := range order {
		tables[i] = *byTable[name]
	}
	return tables
}
