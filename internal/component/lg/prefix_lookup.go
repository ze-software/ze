// Design: docs/architecture/api/birdwatcher-compat.md -- the prefix lookup, an address or a prefix
// Related: handler_ui.go -- handleUISearch sends the lookup with the other search filters
// Related: handler_api.go -- routes/prefix and routes/search send the lookup alone
// Related: handler_graph.go -- the graph sends the lookup for the routes it draws

package lg

import (
	"net/netip"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// The two bgp-rib source selectors a prefix field becomes
// (internal/component/bgp/plugins/rib/rib_pipeline.go). Each one chooses the
// received routes the plugin reads from its prefix trie, and the plugin takes
// either only on the `received` scope.
//
// The text filter `prefix` is not one of them. It keeps a row whose prefix
// text starts with the value, so `prefix 192.0.0.0/2` also lists
// 192.0.0.0/24: a different question from the one a prefix with its length
// asks.
const (
	// filterExact answers the routes of exactly one prefix. The plugin
	// refuses a prefix with host bits set, so the value is always masked.
	filterExact = "exact"
	// filterCovering answers every route whose prefix contains one address,
	// the most specific prefix first within each peer.
	filterCovering = "covering"
)

// prefixLookup is the question one prefix field asks bgp-rib: a filter
// keyword and its value.
//
// Only parsePrefixLookup builds one, so value is always the text net/netip
// wrote for an address or a prefix it parsed. No text the user typed reaches
// the command line, so a space, a pipe or a keyword inside the field cannot
// add a filter of its own.
type prefixLookup struct {
	keyword string
	value   string
}

// parsePrefixLookup reads what a user typed in a prefix field.
//
// A prefix with its length asks for the routes of exactly that prefix,
// through `exact`. Its host bits are cleared first, so `10.0.0.1/8` asks for
// 10.0.0.0/8, the prefix a route carrying that length holds. A bare address
// asks for every route whose prefix contains it, through `covering`. Anything
// else is refused with an operator error that repeats the value, says why,
// and shows a valid form.
//
// The refusal is declared *opererr.Error, not error, so every writer a caller
// hands err.Error() to is seen carrying the fix (./le arch error-inventory),
// and a caller cannot be handed a plain error in its place. A caller keeps it
// in a variable of this type: put in an error variable, a nil refusal is a
// non-nil interface.
func parsePrefixLookup(text string) (prefixLookup, *opererr.Error) {
	if strings.Contains(text, "/") {
		prefix, err := netip.ParsePrefix(text)
		if err != nil {
			return prefixLookup{}, prefixRefusal(text)
		}
		return prefixLookup{keyword: filterExact, value: prefix.Masked().String()}, nil
	}

	addr, err := netip.ParseAddr(text)
	if err != nil {
		return prefixLookup{}, prefixRefusal(text)
	}
	// A zone names a link on this host. No route prefix carries one, so an
	// address with a zone can match nothing.
	if addr.Zone() != "" {
		return prefixLookup{}, opererr.New(
			"address "+text+" carries the zone "+addr.Zone(),
			"a route prefix has no zone, so no route can cover this address",
			addr.WithZone("").String())
	}
	return prefixLookup{keyword: filterCovering, value: addr.String()}, nil
}

// parseExactPrefixLookup reads a field that names one prefix, such as the
// prefix of a route detail: parsePrefixLookup's rule, with a bare address
// refused, because an address names no single prefix. The refusal suggests
// the address as a host prefix, the one prefix it can be written as.
func parseExactPrefixLookup(text string) (prefixLookup, *opererr.Error) {
	if strings.Contains(text, "/") {
		return parsePrefixLookup(text)
	}
	addr, err := netip.ParseAddr(text)
	if err != nil {
		return prefixLookup{}, prefixRefusal(text)
	}
	host := netip.PrefixFrom(addr.WithZone(""), addr.BitLen())
	return prefixLookup{}, opererr.New(
		"route detail for "+text+" names an address, not a prefix",
		"a route detail shows the routes of one prefix, written with its length",
		host.String())
}

// appendTo writes the lookup as one source selector pipe operation,
// " | <keyword> <value>", after the scope operation the caller already wrote.
func (l prefixLookup) appendTo(tb *textbuf.Buffer) {
	tb.Str(" | ").Str(l.keyword).Byte(' ').Str(l.value)
}

// receivedCommand is the command that asks bgp-rib for the routes learned
// from peers that answer the lookup: the `received` scope (cmdRIBReceived),
// the only scope the plugin reads a source selector on.
func (l prefixLookup) receivedCommand() string {
	var tb textbuf.Buffer
	tb.Str(cmdRIBReceived)
	l.appendTo(&tb)
	return tb.String()
}

// prefixRefusal is the operator error for a prefix field that is neither an
// address nor a prefix. A length past the family's longest gets a refusal of
// its own, because the address is right and only the number is wrong.
func prefixRefusal(text string) *opererr.Error {
	if addr, tooLong := prefixLengthTooLong(text); tooLong {
		family := "IPv4"
		if addr.Is6() {
			family = "IPv6"
		}
		longest := strconv.Itoa(addr.BitLen())
		return opererr.New(
			"prefix "+text+" has a length "+family+" does not allow",
			"an "+family+" prefix length is 0 to "+longest,
			addr.String()+"/"+longest)
	}
	return opererr.New(
		"prefix "+text+" is not an IP address or a prefix",
		"the field takes an address, such as 192.0.2.1, which lists every route that covers it, "+
			"or a prefix with its length, which lists exactly that prefix",
		"192.0.2.0/24")
}

// prefixLengthTooLong reports whether text is an address followed by a length
// longer than the address's family allows, and answers that address. Any other
// shape answers false, and prefixRefusal gives the general refusal.
func prefixLengthTooLong(text string) (netip.Addr, bool) {
	slash := strings.LastIndexByte(text, '/')
	if slash < 1 {
		return netip.Addr{}, false
	}
	addr, addrErr := netip.ParseAddr(text[:slash])
	if addrErr != nil {
		return netip.Addr{}, false
	}
	if addr.Zone() != "" {
		return netip.Addr{}, false
	}
	length, lengthErr := strconv.Atoi(text[slash+1:])
	if lengthErr != nil {
		return netip.Addr{}, false
	}
	return addr, length > addr.BitLen()
}
