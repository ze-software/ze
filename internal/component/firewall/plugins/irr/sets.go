// Design: docs/architecture/firewall/firewall-irr.md -- interval set generation from cached prefixes

package irr

import (
	"errors"
	"net/netip"

	"github.com/ze-software/ze/internal/component/firewall"
	"github.com/ze-software/ze/internal/component/resolve/irr/store"
	"github.com/ze-software/ze/internal/core/textbuf"
)

const (
	ifaceTableName = "ze_irr_iface"
	ifaceChainName = "irr_iface_ingress"
	ifaceChainPrio = -10
)

// maxPrefixesPerFamily bounds ONE family's prefix list, because one family is
// one nftables set. IPv4 and IPv6 are two objects in the kernel with two
// element counts, and neither spends the other's budget. The two families
// shared one budget until 2026-09-20, which made the size of the IPv6 set
// depend on the length of the IPv4 list: a full IPv4 list left IPv6 with a set
// of no elements, and that set matches no address.
//
// It is a var so a test can lower it. One IRR reply is cut at 4 MB
// (maxResponse, internal/component/resolve/irr/client.go), so 500000 prefixes
// reach the store only through the many member queries of a large AS-SET, and
// a test that drives the whole refresh cannot afford to build them.
var maxPrefixesPerFamily = 500_000

func setNames(name string) (v4, v6 string) {
	var tb textbuf.Buffer
	v4 = tb.Str("irr_v4_").Str(name).String()
	tb.Reset()
	v6 = tb.Str("irr_v6_").Str(name).String()
	return v4, v6
}

// familySet builds one family's interval set. A family with no prefixes yields
// a set with no elements, which is a valid nftables set and matches no
// address.
//
// The list arrives bounded. oversizedEntryError refuses an entry that does not
// fit before any of this runs, so this function never shortens one: a set that
// holds part of a prefix list is a filter the operator did not write.
func familySet(name string, setType firewall.SetType, prefixes []netip.Prefix) firewall.Set {
	return firewall.Set{
		Name:     name,
		Type:     setType,
		Flags:    firewall.SetFlagInterval,
		Elements: prefixesToIntervalElements(prefixes),
	}
}

// oversizedEntryError refuses a cached entry that holds more prefixes in one
// family than a firewall set takes, and answers nil for an entry that fits.
//
// It is a refusal because the bound used to TRUNCATE. The builder stopped at
// the cap, logged one WARN and returned the short list, so refreshName and
// refreshAllNow programmed part of the operator's rule and reported a refresh
// that worked. A firewall set is read in two directions and nothing here can
// see which one applies: a set a term accepts from drops the traffic the
// missing prefixes carry, and a set a term drops on forwards that traffic.
// Neither may be reached in silence, so an entry that does not fit keeps the
// sets already registered instead of replacing them. That is how the IRR
// client already answers its own read cap, where a reply cut short is an error
// rather than a shorter prefix list (parseReply,
// internal/component/resolve/irr/client.go).
//
// Two paths carry the check. verifyRefs refuses the commit, and applyTables
// refuses the apply for data that reached the cache another way: a refresh, a
// restart reading the persisted store, or the BGP IRR filter writing the same
// shared entry.
func oversizedEntryError(name string, entry *store.CachedEntry) error {
	if entry == nil {
		return nil
	}
	if len(entry.IPv4) > maxPrefixesPerFamily {
		return oversizedMessage(name, "IPv4", len(entry.IPv4))
	}
	if len(entry.IPv6) > maxPrefixesPerFamily {
		return oversizedMessage(name, "IPv6", len(entry.IPv6))
	}
	return nil
}

// oversizedMessage says which family is too long, by how much, and what the
// operator does about it. The rules naming the entry are not programmed, which
// is the fact an operator needs before the count.
func oversizedMessage(name, family string, count int) error {
	var tb textbuf.Buffer
	tb.Str("firewall irr: ").Str(name).Str(" holds ").Int(int64(count)).Byte(' ').Str(family)
	tb.Str(" prefixes and a firewall set holds ").Int(int64(maxPrefixesPerFamily))
	tb.Str(", so the rules naming it are not programmed; narrow the reference, or run 'clear firewall irr ")
	if isASSetName(name) {
		tb.Str("as-set ").Str(name)
	} else {
		tb.Str("asn ").Str(bareASN(name))
	}
	tb.Str("' to remove it")
	return errors.New(tb.String())
}

// refuseOversizedRefs answers the first reference whose cached prefix list does
// not fit a firewall set. One reference stops the whole apply: the alternative
// leaves out the sets of that one entry, and dropTablesMissingAProvidedSet
// (internal/component/firewall/registry.go) then holds back its whole table,
// which takes a filter that was working out of the kernel. Registering nothing
// keeps every table exactly as it is.
func refuseOversizedRefs(ps *store.PrefixStore, refs []irrRef) error {
	for _, ref := range refs {
		if err := oversizedEntryError(ref.Name, ps.Get(ref.Name)); err != nil {
			return err
		}
	}
	return nil
}

// buildSets returns a set for each family the entry announces, and nothing for
// a family that announces none. buildIfaceTables is the caller: it emits an
// accept term per family under the same condition, so it never names a set it
// did not declare. A table term needs buildTermSets instead.
func buildSets(name string, v4, v6 []netip.Prefix) []firewall.Set {
	v4Name, v6Name := setNames(name)
	var sets []firewall.Set

	if len(v4) > 0 {
		sets = append(sets, familySet(v4Name, firewall.SetTypeIPv4, v4))
	}
	if len(v6) > 0 {
		sets = append(sets, familySet(v6Name, firewall.SetTypeIPv6, v6))
	}
	return sets
}

// buildTermSets returns the sets a table TERM needs for one entry, which is
// both families or neither.
//
// The parser cannot see the prefix data. expandProvidedTermV6
// (internal/component/firewall/config.go) emits an IPv6 twin for every IRR
// term, whatever the entry announces. An ASN or AS-SET announcing only IPv4 is
// ordinary, and buildSets answers with one set for it. The twin would then name
// a set no owner declares. dropTablesMissingAProvidedSet
// (internal/component/firewall/registry.go) holds back the operator's WHOLE
// table for it, and the commit reports success with nothing in the kernel.
//
// The family that announced nothing is declared with no elements. That is what
// its term must read: no address of that family belongs to this entry.
//
// An entry announcing nothing at all still yields no set. A cold cache
// therefore keeps holding the table back until prefixes arrive, which
// test/plugin/firewall-irr-cold-cache-recovers.ci asserts.
func buildTermSets(name string, v4, v6 []netip.Prefix) []firewall.Set {
	if len(v4) == 0 && len(v6) == 0 {
		return nil
	}
	v4Name, v6Name := setNames(name)
	return []firewall.Set{
		familySet(v4Name, firewall.SetTypeIPv4, v4),
		familySet(v6Name, firewall.SetTypeIPv6, v6),
	}
}

// prefixesToIntervalElements encodes every prefix as a start element and an
// exclusive end element. It drops nothing but the default route, which an
// interval set cannot carry, and it shortens nothing: the whole list reaches
// the kernel or none of it does (oversizedEntryError).
func prefixesToIntervalElements(prefixes []netip.Prefix) []firewall.SetElement {
	elements := make([]firewall.SetElement, 0, len(prefixes)*2)
	for _, p := range prefixes {
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

		v4Name, v6Name := setNames(ib.ASSet)
		var accepts []firewall.Term
		if len(entry.IPv4) > 0 {
			accepts = append(accepts, firewall.Term{
				Name: ifaceTermName(ib.Interface, "v4"),
				Matches: []firewall.Match{
					firewall.MatchInputInterface{Name: ib.Interface},
					firewall.MatchInSet{SetName: v4Name, MatchField: firewall.SetFieldSourceAddr},
				},
				Actions: []firewall.Action{firewall.Accept{}},
			})
		}
		if len(entry.IPv6) > 0 {
			accepts = append(accepts, firewall.Term{
				Name: ifaceTermName(ib.Interface, "v6"),
				Matches: []firewall.Match{
					firewall.MatchInputInterface{Name: ib.Interface},
					firewall.MatchInSet{SetName: v6Name, MatchField: firewall.SetFieldSourceAddr},
				},
				Actions: []firewall.Action{firewall.Accept{}},
			})
		}

		// The drop term is the whitelist's closing rule: it drops what the
		// accept terms above did not match. Emitted on its own it drops every
		// packet arriving on the interface, so a binding with no prefixes
		// produces nothing at all. A filter with no data is not a filter, and
		// an unfiltered port beats a blackholed one.
		if len(accepts) == 0 {
			logger().Warn("firewall-irr: interface binding has no prefixes, leaving the interface unfiltered",
				"interface", ib.Interface, "as-set", ib.ASSet)
			continue
		}

		if !seenSets[ib.ASSet] {
			seenSets[ib.ASSet] = true
			sets = append(sets, buildSets(ib.ASSet, entry.IPv4, entry.IPv6)...)
		}
		terms = append(terms, accepts...)
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
		sets := buildTermSets(ref.Name, entry.IPv4, entry.IPv6)
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
