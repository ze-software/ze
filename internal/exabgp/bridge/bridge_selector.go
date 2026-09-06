// Design: docs/architecture/exabgp-bridge.md -- naming the sessions a line reaches
// Overview: bridge_command.go -- the line translator that calls this
//
// ExaBGP lets a command name its destination in more than one way, and the
// bridge read only the simplest of them until 2026-09-05: `neighbor <ip>`
// followed immediately by a verb. A line that qualified the neighbor further,
// or named several, matched no verb after the address and was refused whole.

package bridge

import (
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// bridgeSelectorKeys are the qualifiers ExaBGP accepts after a neighbor
// address, each taking one value. They NARROW the set of sessions a command
// reaches; they never widen it.
//
// Source: ExaBGP src/exabgp/reactor/api/command/limit.py, SELECTOR_KEYS.
//
// bridgeFamilyAllowed is named because three places read the same keyword: the
// key set below, the one value that excludes (selectorExcludes), and the
// `create neighbor` parameter table (bridge_neighbor.go).
const bridgeFamilyAllowed = "family-allowed"

var bridgeSelectorKeys = map[string]bool{
	"local-ip":          true,
	"local-address":     true,
	"local-as":          true,
	"peer-as":           true,
	"router-id":         true,
	bridgeFamilyAllowed: true,
}

// splitNeighborSelector reads the destination off the front of an ExaBGP line
// and answers it as a ze peer selector, with the command that follows it.
//
// everyPeer is the selector for a line that names no neighbor. The caller
// supplies it because the answer belongs to the SCRIPT rather than to the line:
// a script every neighbor feeds sends to every peer, and a script two neighbors
// name sends to those two (Translator.everyPeer).
//
// ExaBGP writes the destination four ways, and all four occur in its own test
// corpus:
//
//	announce route ...                                  every session
//	neighbor 127.0.0.1 announce route ...               one session
//	neighbor 127.0.0.1 local-as 1 peer-as 1 announce .. one session, qualified
//	neighbor A router-id X, neighbor B announce ...     two sessions
//
// The qualifiers are PARSED and, with ONE exception, do not reach ze. ze's
// selector names a peer by address, name, ASN or glob and cannot conjoin a
// predicate onto an address, so there is nothing to translate them into. What
// that costs is bounded and stated rather than hidden: an address already
// resolves to at most one session in ze, so a qualifier can only have REJECTED
// that session, and ze sends where ExaBGP would have stayed silent.
// Conjunctive selectors are a change to the command grammar, which is not the
// bridge's to make.
//
// The exception is the qualifier that can never match anything: unmatched says
// every address the line named was excluded, so the line reaches no session.
// The caller acks it and dispatches nothing, which is what ExaBGP does with a
// command whose selector matches no session.
func splitNeighborSelector(line, everyPeer string) (selector, rest string, unmatched bool) {
	fields := strings.Fields(line)
	if len(fields) == 0 || !strings.EqualFold(fields[0], "neighbor") {
		return everyPeer, line, false
	}

	var addresses []string
	named := 0
	i := 0
	for i < len(fields) && strings.EqualFold(fields[i], "neighbor") {
		if i+1 >= len(fields) {
			return everyPeer, line, false
		}
		address := strings.TrimSuffix(fields[i+1], ",")
		named++
		i += 2

		// The qualifiers, each a key and a value.
		excluded := false
		for i+1 < len(fields) && bridgeSelectorKeys[strings.ToLower(strings.TrimSuffix(fields[i], ","))] {
			if selectorExcludes(strings.ToLower(strings.TrimSuffix(fields[i], ",")), strings.TrimSuffix(fields[i+1], ",")) {
				excluded = true
			}
			commaAfterValue := strings.HasSuffix(fields[i+1], ",")
			i += 2
			if commaAfterValue {
				break
			}
		}
		if !excluded {
			addresses = append(addresses, address)
		}

		// A comma continues the list with another neighbor. It may be attached
		// to the previous token or stand alone.
		if i < len(fields) && fields[i] == "," {
			i++
			continue
		}
		if i > 0 && strings.HasSuffix(fields[i-1], ",") {
			continue
		}
		break
	}

	if len(addresses) == 0 {
		if named > 0 {
			// Every address the line named carried a qualifier no ze session
			// can satisfy, so the line reaches nothing.
			return "", strings.Join(fields[i:], " "), true
		}
		return everyPeer, line, false
	}
	return selectorForAddresses(addresses), strings.Join(fields[i:], " "), false
}

// selectorExcludes reports whether a qualifier's value names something no ze
// session can be, so that an address carrying it reaches no session.
//
// There is exactly one such value. `family-allowed in-open` is ExaBGP's
// "negotiate the families in the OPEN", and its own parser answers an empty
// family list for it. ze STATES the families its OPEN offers and has no mode
// that defers the choice, so no ze session is ever the one that qualifier
// names. bridge_neighbor.go refuses the same word on `create neighbor`, for the
// same reason and from the same reading.
//
// Every other value CAN name ze's session, and ze has at most one per address,
// so the qualifier is discarded rather than evaluated (see the note above).
func selectorExcludes(key, value string) bool {
	return key == bridgeFamilyAllowed && strings.EqualFold(value, "in-open")
}

// selectorForAddresses renders one or more neighbor addresses as the ze
// selector that names them. ze spells a set as a comma-separated list.
//
// A repeated address is written once. ExaBGP's own corpus writes
// `neighbor 127.0.0.1 router-id 1.2.3.4, neighbor 127.0.0.1 announce ...`,
// where the two entries differ only in a qualifier the ze selector cannot
// carry, so keeping both would name one peer twice.
func selectorForAddresses(addresses []string) string {
	var tb textbuf.Buffer
	seen := make(map[string]bool, len(addresses))
	for _, address := range addresses {
		if seen[address] {
			continue
		}
		seen[address] = true
		if len(seen) > 1 {
			tb.Byte(',')
		}
		tb.Str(address)
	}
	return tb.String()
}
