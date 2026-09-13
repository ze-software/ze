// Design: docs/architecture/exabgp-bridge.md -- naming the sessions a line reaches
// Overview: bridge_command.go -- the line translator that calls this
//
// ExaBGP lets a command name its destination in more than one way, and the
// bridge read only the simplest of them until 2026-09-05: `neighbor <ip>`
// followed immediately by a verb. A line that qualified the neighbor further,
// or named several, matched no verb after the address and was refused whole.
//
// ExaBGP spells the keyword two ways and this file reads both. `neighbor` is
// its v4 API, `peer` its v6 one, and its own parser takes either.

package bridge

import (
	"slices"
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

// bridgeSelectorKeyword reports whether a token is the word ExaBGP writes
// before a peer address.
//
// There are two, and they are the same word in two API versions: v4 writes
// `neighbor`, v6 writes `peer`, and ExaBGP's own parser takes either ("Accept
// both 'neighbor' (v4) and 'peer' (v6) prefixes", extract_neighbors,
// src/exabgp/reactor/api/command/limit.py). Its healthcheck application writes
// the v6 one, so a bridge that read `neighbor` alone refused every line that
// application produced.
func bridgeSelectorKeyword(field string) bool {
	return strings.EqualFold(field, "neighbor") || strings.EqualFold(field, "peer")
}

// splitNeighborSelector reads the destination off the front of an ExaBGP line
// and answers it as a ze peer selector, with the command that follows it.
//
// everyPeer is the selector for a line that names no neighbor. The caller
// supplies it because the answer belongs to the SCRIPT rather than to the line:
// a script every neighbor feeds sends to every peer, and a script two neighbors
// name sends to those two (Translator.everyPeer).
//
// ExaBGP writes the destination five ways, and all five occur in its own test
// corpus:
//
//	announce route ...                                  every session
//	neighbor 127.0.0.1 announce route ...               one session
//	neighbor 127.0.0.1 local-as 1 peer-as 1 announce .. one session, qualified
//	neighbor A router-id X, neighbor B announce ...     two sessions
//	peer [A router-id X, B] announce route ...          two sessions, v6
//
// The keyword is `neighbor` or `peer` (bridgeSelectorKeyword), and the bracket
// list is v6's spelling of the comma list. The two spellings name the same
// sessions, so they answer the same selector.
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
	if len(fields) == 0 || !bridgeSelectorKeyword(fields[0]) {
		return everyPeer, line, false
	}
	if len(fields) > 1 && strings.HasPrefix(fields[1], "[") {
		return splitBracketSelector(line, everyPeer)
	}

	var addresses []string
	named := 0
	i := 0
	for i < len(fields) && bridgeSelectorKeyword(fields[i]) {
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

// splitBracketSelector reads ExaBGP's v6 destination list, which holds every
// address in ONE bracket and repeats the keyword for none of them:
//
//	peer [10.0.0.1 router-id 1.2.3.4, 10.0.0.2] announce route ...
//
// Each entry is an address or `*`, followed by the qualifiers the comma form
// takes, and the entries are separated by commas
// (_extract_bracket_selectors and _parse_single_selector,
// src/exabgp/reactor/api/command/limit.py).
//
// A list with no closing bracket names nothing the bridge can read, so the
// whole line is answered unchanged and the caller refuses it by name. ExaBGP
// reaches the same end by another road: its parser answers no selector and
// leaves the bracket in the command, which its dispatcher then fails to read.
//
// line is the whitespace-collapsed line, as the caller already computed it.
func splitBracketSelector(line, everyPeer string) (selector, rest string, unmatched bool) {
	open := strings.Index(line, "[")
	if open < 0 {
		return everyPeer, line, false
	}
	closing := strings.Index(line[open:], "]")
	if closing < 0 {
		return everyPeer, line, false
	}
	inside := line[open+1 : open+closing]
	rest = strings.TrimSpace(line[open+closing+1:])

	var addresses []string
	named := 0
	for entry := range strings.SplitSeq(inside, ",") {
		words := strings.Fields(entry)
		if len(words) == 0 {
			continue
		}
		named++
		if !bracketEntryExcluded(words[1:]) {
			addresses = append(addresses, words[0])
		}
	}

	if len(addresses) == 0 {
		if named > 0 {
			// Every address the list named carried a qualifier no ze session
			// can satisfy, so the line reaches nothing. This is the answer the
			// comma form gives for the same list.
			return "", rest, true
		}
		// An empty list names no address, so the line goes where a line that
		// names none goes. ExaBGP answers it the same way: no selector means
		// every peer the process feeds (match_neighbors, limit.py).
		return everyPeer, rest, false
	}
	return selectorForAddresses(addresses), rest, false
}

// bracketEntryExcluded reports whether the qualifiers of one bracket entry name
// something no ze session can be. words holds the entry's tokens after its
// address, as key and value pairs.
//
// A key the qualifier set does not hold ENDS the entry, which is what ExaBGP's
// own parser does with it (_parse_single_selector breaks on an unknown key).
// The tokens after it are dropped with the qualifiers, because a bracket entry
// holds a selector and nothing else.
func bracketEntryExcluded(words []string) bool {
	for i := 0; i+1 < len(words); i += 2 {
		key := strings.ToLower(words[i])
		if !bridgeSelectorKeys[key] {
			return false
		}
		if selectorExcludes(key, words[i+1]) {
			return true
		}
	}
	return false
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
//
// A list holding the WILDCARD is the wildcard. ExaBGP's own corpus writes
// `peer [10.0.0.1 local-as 65000, *]`, and it unions the matches, so the entry
// that matches every session decides the answer. ze's selector parser reads a
// comma list as a list of ADDRESSES (parseMultiIP, internal/core/selector), so
// writing `10.0.0.1,*` would fail to parse, fall back to a peer NAME no session
// carries, and send the route nowhere.
func selectorForAddresses(addresses []string) string {
	if slices.Contains(addresses, bridgeEveryPeer) {
		return bridgeEveryPeer
	}

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
