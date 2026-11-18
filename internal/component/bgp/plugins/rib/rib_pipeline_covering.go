// Design: docs/architecture/plugin/rib-storage-design.md — covering and exact reads for `show bgp rib received covering <address>` and `exact <prefix>`
// Overview: rib_pipeline.go — iterator pipeline for RIB show commands
// Related: storage/peerrib.go — PeerRIB.IterateCovering and PeerRIB.IterateExact, the trie reads this file selects
package rib

import (
	"net/netip"
	"strings"

	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/storage"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// receivedWalk names which of a peer's received routes a source reads: every
// route, only the routes whose prefix contains one address (covering), or only
// the routes of one prefix (exact).
//
// `covering <address>` and `exact <prefix>` select the routes a source reads,
// the way `peer <selector>` selects the peers, so they are parsed into the walk
// rather than into a filter stage. A filter stage sees every row the source
// yields, and a covering or exact question answered by a filter would read the
// whole table to keep a handful of rows, on a looking glass endpoint anyone
// can reach.
//
// The zero value is the whole-table walk. That zero is a guard: isCovering
// and isExact read it, and coveringWalk and exactWalk are the only ways to
// build the other two walks, from a value parseCoveringAddress or
// parseExactPrefix has already accepted. At most one of the two fields is
// valid: each constructor sets one, and parsePipelineArgs refuses a command
// that names both keywords (walkConflictRefusal).
type receivedWalk struct {
	// covering is the address a covering walk reads the routes of. It is
	// valid only in a covering walk.
	covering netip.Addr
	// exact is the masked prefix an exact walk reads the routes of. It is
	// valid only in an exact walk.
	exact netip.Prefix
}

// coveringWalk builds the walk that reads, from each peer's prefix trie, only
// the routes whose prefix contains addr. addr MUST come from
// parseCoveringAddress.
func coveringWalk(addr netip.Addr) receivedWalk {
	return receivedWalk{covering: addr}
}

// exactWalk builds the walk that reads, from each peer's prefix trie, only the
// routes of pfx itself. pfx MUST come from parseExactPrefix, which refuses a
// prefix with host bits set.
func exactWalk(pfx netip.Prefix) receivedWalk {
	return receivedWalk{exact: pfx}
}

// isCovering reports whether the walk reads covering routes only.
func (w receivedWalk) isCovering() bool {
	return w.covering.IsValid()
}

// isExact reports whether the walk reads the routes of one prefix only.
func (w receivedWalk) isExact() bool {
	return w.exact.IsValid()
}

// read hands one peer's selected routes to fn. A whole-table walk visits every
// route in sorted prefix order. A covering walk visits only the trie entries on
// the path to the address, most specific prefix first within each family, with
// every ADD-PATH path of each prefix (PeerRIB.IterateCovering). An exact walk
// visits only the entry keyed by the prefix, with every ADD-PATH path of it
// (PeerRIB.IterateExact).
//
// fn runs under the PeerRIB read lock, so it MUST NOT call a PeerRIB method
// that takes it again (PeerRIB.IsAddPath states the deadlock).
func (w receivedWalk) read(peerRIB *storage.PeerRIB, fn func(fam family.Family, nlriBytes []byte, entry storage.RouteEntry) bool) {
	if w.isCovering() {
		peerRIB.IterateCovering(w.covering, fn)
		return
	}
	if w.isExact() {
		peerRIB.IterateExact(w.exact, fn)
		return
	}
	peerRIB.IterateSorted(fn)
}

// parseCoveringAddress reads the value of `covering`: one IP address, IPv4 or
// IPv6, with no mask and no zone. It answers the refusal text for any other
// value, naming the value and a form that works.
//
// A value with a mask is refused rather than read as its address. `covering
// 10.0.0.0/8` would otherwise answer the routes containing 10.0.0.0, a
// different question from the one the operator wrote, and `exact` answers the
// routes of a prefix.
func parseCoveringAddress(value string) (netip.Addr, string) {
	addr, err := netip.ParseAddr(value)
	if err != nil {
		return netip.Addr{}, coveringValueRefusal(value)
	}
	if addr.Zone() != "" {
		// A route's prefix carries no zone, so a zoned address names no route.
		var tb textbuf.Buffer
		return netip.Addr{}, tb.Str("covering takes an IP address with no zone, such as ").
			Str(addr.WithZone("").String()).Str(": ").Quoted(value).Str(" names a zone").String()
	}
	return addr, ""
}

// coveringValueRefusal words the refusal of a covering value that is not a
// bare address. A value that is a prefix gets the address it was written with
// as the example, and the keyword that answers the question it looks like.
func coveringValueRefusal(value string) string {
	var tb textbuf.Buffer
	if pfx, err := netip.ParsePrefix(value); err == nil {
		return tb.Str("covering takes an IP address with no mask, such as ").Str(pfx.Addr().String()).
			Str(": ").Quoted(value).Str(" carries a mask; for the routes of that prefix itself, use exact ").
			Str(pfx.Masked().String()).String()
	}
	example := "192.0.2.1"
	if strings.Contains(value, ":") {
		example = "2001:db8::1"
	}
	return tb.Str("covering takes an IP address with no mask, such as ").Str(example).
		Str(": ").Quoted(value).Str(" is not an IP address").String()
}

// parseExactPrefix reads the value of `exact`: one IPv4 or IPv6 prefix with its
// mask, no host bits set and no zone. It answers the refusal text for any other
// value, naming the value and a form that works.
//
// A prefix with host bits set is refused rather than masked. `exact
// 10.0.0.1/8` would otherwise answer the routes of 10.0.0.0/8, a prefix the
// operator did not write, and the refusal names that masked form so the
// operator can confirm it.
func parseExactPrefix(value string) (netip.Prefix, string) {
	pfx, err := netip.ParsePrefix(value)
	if err != nil {
		return netip.Prefix{}, exactValueRefusal(value)
	}
	masked := pfx.Masked()
	if pfx != masked {
		var tb textbuf.Buffer
		return netip.Prefix{}, tb.Str("exact takes a prefix with no host bits set, such as ").Str(masked.String()).
			Str(": ").Quoted(value).Str(" sets bits past its mask").String()
	}
	return pfx, ""
}

// exactValueRefusal words the refusal of an exact value that netip cannot read
// as a prefix: a zoned value, a bare address, or text that is neither. A bare
// address gets its host prefix as the example and the keyword that answers the
// question it looks like.
func exactValueRefusal(value string) string {
	var tb textbuf.Buffer
	if strings.Contains(value, "%") {
		// netip refuses a zone in a prefix, and a route's prefix carries none.
		return tb.Str("exact takes a prefix with no zone, such as ").Str(exactZonelessExample(value)).
			Str(": ").Quoted(value).Str(" names a zone").String()
	}
	if addr, err := netip.ParseAddr(value); err == nil {
		return tb.Str("exact takes a prefix with its mask, such as ").Str(netip.PrefixFrom(addr, addr.BitLen()).String()).
			Str(": ").Quoted(value).Str(" is an address with no mask; for the routes containing that address, use covering ").
			Str(value).String()
	}
	return tb.Str("exact takes a prefix with its mask, such as ").Str(exactExample(value)).
		Str(": ").Quoted(value).Str(" is not a prefix").String()
}

// exactZonelessExample answers the form a zoned exact value names once its zone
// is dropped, the mask kept: fe80::%eth0/64 gives fe80::/64, and a zoned
// address gives its host prefix. A value that reads as neither gets the
// generic example of its IP version.
func exactZonelessExample(value string) string {
	// The caller found a '%' in value, so the cut always splits it.
	address, rest, _ := strings.Cut(value, "%")
	if _, mask, hasMask := strings.Cut(rest, "/"); hasMask {
		if pfx, err := netip.ParsePrefix(address + "/" + mask); err == nil {
			return pfx.Masked().String()
		}
		return exactExample(value)
	}
	if addr, err := netip.ParseAddr(address); err == nil {
		return netip.PrefixFrom(addr, addr.BitLen()).String()
	}
	return exactExample(value)
}

// exactExample is the prefix an exact refusal offers when the value itself
// gives none, of the IP version the value looks like.
func exactExample(value string) string {
	if strings.Contains(value, ":") {
		return "2001:db8::/32"
	}
	return "10.0.0.0/8"
}

// walkConflictRefusal refuses a command that names both covering and exact.
// They select different routes, the ones containing an address and the ones
// of one prefix, and a walk reads one selection, so the refusal names what
// each keyword alone would answer and lets the operator drop the other.
func walkConflictRefusal(addr netip.Addr, pfx netip.Prefix) string {
	var tb textbuf.Buffer
	return tb.Str("covering and exact select different routes, so one lookup takes one of them: drop exact to list the routes containing ").
		Str(addr.String()).Str(", or drop covering to list the routes of ").Str(pfx.String()).String()
}

// refuseCoveringScope refuses a covering walk asked of the sent routes. The
// Adj-RIB-Out is held in maps keyed by route, with no prefix trie to read the
// path to an address from, so answering would mean reading every sent route;
// the received scope answers it.
func refuseCoveringScope(addr netip.Addr) pipelineRefusal {
	var fix textbuf.Buffer
	fix.Str("show bgp rib received covering ").Str(addr.String())
	return refusePipelineArgs("covering reads received routes only, not sent routes", fix.String())
}

// refuseExactScope refuses an exact walk asked of the sent routes. The sent
// sources read the Adj-RIB-Out whole and take no walk, so on a scope holding
// sent routes `exact` would either be ignored there, listing every sent route,
// or answer the received half alone; refusing names the scope that answers it.
func refuseExactScope(pfx netip.Prefix) pipelineRefusal {
	var fix textbuf.Buffer
	fix.Str("show bgp rib received exact ").Str(pfx.String())
	return refusePipelineArgs("exact reads received routes only, not sent routes", fix.String())
}

// coveringOnBestRefusal is the refusal `show bgp rib best` answers for the
// covering keyword. The best-path answer runs one election per prefix, over
// prefixes newBestSource collects with a whole-table walk of every peer, so a
// covering best-path answer would read every route this keyword exists not to
// read. The received scope answers the question from the trie.
const coveringOnBestRefusal = "covering is not available on show bgp rib best: " +
	"it reads received routes, so use show bgp rib received covering <address>"

// exactOnBestRefusal is the refusal `show bgp rib best` answers for the exact
// keyword, for the reason coveringOnBestRefusal states: newBestSource collects
// its prefixes with a whole-table walk of every peer and takes no walk.
const exactOnBestRefusal = "exact is not available on show bgp rib best: " +
	"it reads received routes, so use show bgp rib received exact <prefix>"
