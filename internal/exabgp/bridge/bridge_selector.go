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
var bridgeSelectorKeys = map[string]bool{
	"local-ip":       true,
	"local-address":  true,
	"local-as":       true,
	"peer-as":        true,
	"router-id":      true,
	"family-allowed": true,
}

// splitNeighborSelector reads the destination off the front of an ExaBGP line
// and answers it as a ze peer selector, with the command that follows it.
//
// ExaBGP writes the destination four ways, and all four occur in its own test
// corpus:
//
//	announce route ...                                  every session
//	neighbor 127.0.0.1 announce route ...               one session
//	neighbor 127.0.0.1 local-as 1 peer-as 1 announce .. one session, qualified
//	neighbor A router-id X, neighbor B announce ...     two sessions
//
// The qualifiers are PARSED and do not reach ze. ze's selector names a peer by
// address, name, ASN or glob and cannot conjoin a predicate onto an address, so
// there is nothing to translate them into. What that costs is bounded and
// stated rather than hidden: an address already resolves to at most one session
// in ze, so a qualifier can only have REJECTED that session, and ze sends where
// ExaBGP would have stayed silent. Conjunctive selectors are a change to the
// command grammar, which is not the bridge's to make.
func splitNeighborSelector(line string) (selector, rest string) {
	fields := strings.Fields(line)
	if len(fields) == 0 || !strings.EqualFold(fields[0], "neighbor") {
		return bridgeEveryPeer, line
	}

	var addresses []string
	i := 0
	for i < len(fields) && strings.EqualFold(fields[i], "neighbor") {
		if i+1 >= len(fields) {
			return bridgeEveryPeer, line
		}
		addresses = append(addresses, strings.TrimSuffix(fields[i+1], ","))
		i += 2

		// The qualifiers, each a key and a value.
		for i+1 < len(fields) && bridgeSelectorKeys[strings.ToLower(strings.TrimSuffix(fields[i], ","))] {
			commaAfterValue := strings.HasSuffix(fields[i+1], ",")
			i += 2
			if commaAfterValue {
				break
			}
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
		return bridgeEveryPeer, line
	}
	return selectorForAddresses(addresses), strings.Join(fields[i:], " ")
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
