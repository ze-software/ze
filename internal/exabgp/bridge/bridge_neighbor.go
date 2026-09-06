// Design: docs/architecture/exabgp-bridge.md -- the neighbor lifecycle vocabulary
// RFC: rfc/short/rfc4271.md -- Section 4.5, the NOTIFICATION subcode is one octet
// RFC: rfc/short/rfc4486.md -- the Cease subcodes a teardown chooses from
// RFC: rfc/short/rfc8203.md -- the shutdown communication ze supplies by default
// Overview: bridge_command.go -- the line translator that calls this converter
// Related: bridge_event.go -- the event encoder, which writes JSON and not text
//
// ExaBGP's API drives a neighbor as well as its routes: it creates one, deletes
// one, tears one down, and reports what one received. The four verbs land in
// three different places, and this file is where that is decided rather than
// guessed. Three are ze commands, and one is not a command at all.

package bridge

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// The refusals `create neighbor` and `delete neighbor` answer with.
//
// Each one names the ExaBGP parameter ze cannot honor, rather than dropping it
// and acking the script for a peer that is not the one it asked for
// (ai/rules/principles.md).
var (
	// ExaBGP refuses its own line when one of these three is absent
	// (_parse_neighbor_params, src/exabgp/reactor/api/command/peer.py), so the
	// bridge refuses it here and the script gets the same answer it would get
	// from ExaBGP rather than a ze error about a different leaf.
	errNeighborCreateRequired = errors.New("create neighbor requires local-address, local-as and peer-as")

	errNeighborCreateAddress = errors.New("create neighbor takes the peer address after the neighbor keyword")

	errNeighborParameter = errors.New("create neighbor does not take this parameter")

	errNeighborDuplicate = errors.New("create neighbor takes this parameter once")

	// `family-allowed in-open` is ExaBGP's "negotiate the families in the
	// OPEN": its parser answers an empty family list for it. ze states the
	// families its OPEN offers and has no mode that defers the choice, and an
	// omitted `family` is ipv4/unicast rather than "whatever the peer sends".
	// So the two are different sessions and the word is refused.
	errNeighborFamilyInOpen = errors.New("ze states the families its OPEN offers, so `family-allowed in-open` names no ze behavior")

	errNeighborFamilyForm = errors.New("a family is <afi>-<safi>, and several are separated by /")

	// ExaBGP's `peer delete <selector> local-as 1` filters which peers the
	// delete reaches. `delete bgp peer` takes the selector alone, so a filter
	// would delete a peer the script did not name.
	errNeighborDeleteFilter = errors.New("delete neighbor takes one address, and ze's delete carries no filter")

	errNeighborDeleteAddress = errors.New("delete neighbor takes one address")
)

// errNeighborReceive is the answer for `receive`. It names ExaBGP's own EVENT
// vocabulary, written by the text response encoder
// (src/exabgp/reactor/api/response/text.py, Response.Text.update, with
// direction `receive`), so it travels UP from the daemon to the script. A
// script never sends one down, and api-check reads it on its stdin.
//
// It is refused by name so that a reader of the refusal opens the event encoder
// rather than the translator. ze has no text event encoder at all today:
// bridge_event.go declares ZebgpToExabgpJSON and nothing else, and no code in
// the bridge reads the `encoder text` setting a script's config can carry.
var errNeighborReceive = errors.New("`receive` names an event ze sends up to the script, not a command a script sends down")

// errTeardownSubcode is the answer for a teardown whose subcode the
// NOTIFICATION field cannot carry.
var errTeardownSubcode = errors.New("teardown takes one BGP cease subcode, 0 to 255")

// ConvertNeighborControl translates the ExaBGP neighbor lifecycle commands
// that are not routes. It reports false when the line is not one of them.
//
//	neighbor <ip> teardown <subcode>  -> request peer <ip> teardown <subcode>
//	create neighbor <ip> ...          -> create bgp peer <ip> asn <asn> ...
//	delete neighbor <ip>              -> delete bgp peer <ip>
//	neighbor <ip> receive ...         -> refused, this is an event, not a command
//
// The three answers are three different things, and a caller MUST read them
// apart. ok false with a nil error says the line belongs to another converter.
// ok true with a nil error says the Translation carries the ze command. ok true
// with an error says the line IS one of these commands and the bridge will not
// translate it, which is the answer that MUST reach the script rather than
// being turned into a command that does something else.
//
// rest is the line with any `neighbor <address>` prefix already removed, and
// selector names the peers it addresses. A line that names no neighbor arrives
// with the wildcard selector, the same as every other converter here.
//
// A create and a delete line name their OWN peer, in the token after
// `neighbor`, so both ignore the selector they arrive with: the line begins
// with the verb rather than with `neighbor`, so splitNeighborSelector had
// nothing to read and answered the wildcard.
func ConvertNeighborControl(selector, rest string) (Translation, bool, error) {
	fields := strings.Fields(rest)
	if len(fields) == 0 {
		return Translation{}, false, nil
	}
	verb := strings.ToLower(fields[0])
	names := len(fields) > 1 && strings.EqualFold(fields[1], "neighbor")

	switch {
	case verb == "teardown":
		return convertTeardown(selector, lowerFields(fields))
	case verb == "create" && names:
		return convertNeighborCreate(fields[2:])
	case verb == "delete" && names:
		return convertNeighborDelete(fields[2:])
	case verb == "receive":
		return Translation{}, true, fmt.Errorf("%w: %q", errNeighborReceive, rest)
	}
	return Translation{}, false, nil
}

// lowerFields answers a lowercased copy, for the converters that compare every
// token and carry none of them through.
func lowerFields(fields []string) []string {
	lowered := make([]string, len(fields))
	for i, field := range fields {
		lowered[i] = strings.ToLower(field)
	}
	return lowered
}

// convertTeardown writes the ze command that closes a session with the cease
// subcode the script chose.
//
// ze sends Cease (RFC 4271 error code 6) with this subcode, and it supplies the
// RFC 8203 shutdown communication itself when none is given: Session.teardown
// (internal/component/bgp/reactor/session_connection.go) defaults the message to
// CeaseSubcodeString(subcode). So the ExaBGP grammar, which carries a subcode
// and nothing else, needs no message invented for it here.
//
// The subcode is converted at this boundary rather than passed on as the token
// the script wrote. ze's handler checks the range again
// (handleTeardown, internal/component/bgp/plugins/cmd/peer/peer.go), and the
// pair is deliberate: this one names the ExaBGP line in the refusal, and that
// one guards every other caller.
func convertTeardown(selector string, fields []string) (Translation, bool, error) {
	// ExaBGP's teardown carries the subcode alone: its handler reads the rest of
	// the line as one string and refuses it unless every character is a digit
	// (src/exabgp/reactor/api/command/neighbor.py, teardown). A trailing word is
	// therefore not a shutdown message a script can write, however well ze's own
	// command would carry one.
	if len(fields) != 2 {
		return Translation{}, true, fmt.Errorf("%w, got %d words", errTeardownSubcode, len(fields)-1)
	}

	// RFC 4271 Section 4.5: "This 1-octet unsigned integer provides more
	// specific information about the nature of the reported error." One octet is
	// what the wire carries, so a value outside 0 to 255 names no subcode and is
	// refused here rather than sent on as text.
	subcode, err := strconv.ParseUint(fields[1], 10, 8)
	if err != nil {
		return Translation{}, true, fmt.Errorf("%w, got %q", errTeardownSubcode, fields[1])
	}

	// A wildcard selector reaches ze's own refusal rather than a second copy of
	// it here. ResolveSinglePeer (internal/component/plugin/server/command.go)
	// answers "teardown requires one specific peer", which states the rule where
	// it is decided; restating it in the bridge would be a copy that can drift.
	var tb textbuf.Buffer
	return Translation{
		Commands: textCommands(tb.Str("request peer ").Str(selector).Str(" teardown ").Uint8(uint8(subcode)).String()),
		Selector: selector,
	}, true, nil
}

// The `create bgp peer` keywords this converter writes. Each one is a leaf of
// the same name in ze-peer-cmd.yang, and each is named three times here: in the
// mapping table, in the order the command is written in, and in the required
// set. A constant is what keeps the three agreeing.
const (
	zeKeywordASN             = "asn"
	zeKeywordLocalAS         = "local-as"
	zeKeywordLocalAddress    = "local-address"
	zeKeywordRouterID        = "router-id"
	zeKeywordAccept          = "accept"
	zeKeywordFamily          = "family"
	zeKeywordGracefulRestart = "graceful-restart"
	zeKeywordGroupUpdates    = "group-updates"
	zeKeywordAttach          = "attach"
)

// neighborParameter is one ExaBGP `create neighbor` parameter: the
// `create bgp peer` keyword that carries it, and the conversion its value
// needs on the way.
type neighborParameter struct {
	keyword string
	convert func(value string) (string, error)
}

// neighborCreateParameters is every parameter ExaBGP's `create neighbor` reads
// (_parse_neighbor_params, src/exabgp/reactor/api/command/peer.py), except
// `api`, which repeats and is collected on its own.
//
// A parameter ze cannot honor is REFUSED by name rather than dropped, and
// there is exactly one: `family-allowed in-open`. Every other parameter has a
// `create bgp peer` keyword that means the same thing, so the table is the whole
// mapping and a reader needs no second list (ai/rules/evidence.md).
//
// The VALUES pass through unconverted where the two grammars agree on the
// spelling. ze's handler types each one and names the offending keyword
// (peerCreateKeywords, internal/component/bgp/plugins/cmd/peer/create.go), so a
// second range check here would be a copy that can drift.
var neighborCreateParameters = map[string]neighborParameter{
	// ExaBGP accepts two spellings for the local address, and they are one
	// parameter: a line naming both is a duplicate, as it is in ExaBGP.
	"local-address":     {keyword: zeKeywordLocalAddress, convert: neighborValueAsIs},
	"local-ip":          {keyword: zeKeywordLocalAddress, convert: neighborValueAsIs},
	"local-as":          {keyword: zeKeywordLocalAS, convert: neighborValueAsIs},
	"peer-as":           {keyword: zeKeywordASN, convert: neighborValueAsIs},
	"router-id":         {keyword: zeKeywordRouterID, convert: neighborValueAsIs},
	bridgeFamilyAllowed: {keyword: zeKeywordFamily, convert: neighborFamilies},
	"graceful-restart":  {keyword: zeKeywordGracefulRestart, convert: neighborValueAsIs},
	"group-updates":     {keyword: zeKeywordGroupUpdates, convert: neighborValueAsIs},
}

// neighborCreateOrder is the order the keywords are written in, so one line
// always produces one command text. A map iterates at random, and a command
// that changes shape between two runs is one no test can assert on.
var neighborCreateOrder = []string{
	zeKeywordASN, zeKeywordLocalAS, zeKeywordLocalAddress, zeKeywordRouterID,
	zeKeywordAccept, zeKeywordFamily, zeKeywordGracefulRestart,
	zeKeywordGroupUpdates, zeKeywordAttach,
}

// neighborCreateRequired is the set ExaBGP itself demands. The bridge demands
// the same three, so a script missing one gets the answer it would get from
// ExaBGP rather than a ze error naming a config leaf it never wrote.
var neighborCreateRequired = []string{zeKeywordASN, zeKeywordLocalAS, zeKeywordLocalAddress}

// convertNeighborCreate writes the ze command that creates a BGP peer while the
// daemon runs.
//
// fields is the line after `create neighbor`, so fields[0] is the peer address
// and the rest are parameter pairs.
//
//	create neighbor 127.0.0.1 local-address 127.0.0.1 local-as 1 peer-as 1 api peer-lifecycle
//	create bgp peer 127.0.0.1 asn 1 local-as 1 local-address 127.0.0.1 attach peer-lifecycle
//
// The peer ze builds lives in the reactor alone, which is what ExaBGP's own
// dynamic peer does: neither writes the configuration file.
func convertNeighborCreate(fields []string) (Translation, bool, error) {
	if len(fields) == 0 {
		return Translation{}, true, errNeighborCreateAddress
	}
	address := fields[0]

	values := make(map[string]string, len(fields)/2)
	var processes []string

	for i := 1; i < len(fields); i++ {
		name := strings.ToLower(fields[i])
		if i+1 >= len(fields) {
			return Translation{}, true, fmt.Errorf("%w: %q takes a value", errNeighborParameter, name)
		}
		value := fields[i+1]
		i++

		// `api` is the one repeating parameter: ExaBGP writes the keyword once
		// per process. ze names them all in one `attach`, because the
		// dispatcher binds exactly one token to a keyword.
		if name == "api" {
			processes = append(processes, value)
			continue
		}

		parameter, known := neighborCreateParameters[name]
		if !known {
			return Translation{}, true, fmt.Errorf("%w: %q", errNeighborParameter, name)
		}
		if _, seen := values[parameter.keyword]; seen {
			return Translation{}, true, fmt.Errorf("%w: %q", errNeighborDuplicate, name)
		}
		converted, err := parameter.convert(value)
		if err != nil {
			return Translation{}, true, err
		}
		values[parameter.keyword] = converted
	}

	if len(processes) > 0 {
		values[zeKeywordAttach] = textbuf.Join(processes, ",")
	}

	// The created neighbor DIALS and does not listen.
	//
	// ExaBGP's neighbor_create builds a Neighbor and leaves `passive` at its
	// default of false, so the peer connects out (_build_neighbor,
	// src/exabgp/reactor/api/command/peer.py). It opens no listening socket for
	// it either: ExaBGP has ONE listener, declared statically in the
	// configuration file, and a neighbor created over the API never adds a
	// second one.
	//
	// ze binds a listener per peer address that accepts (startMultiListeners,
	// internal/component/bgp/reactor/reactor.go), so a created peer carrying
	// `local-address` and ze's default `accept true` would open a socket ExaBGP
	// never opens. Stating `accept false` is what makes the two daemons build
	// the same session out of the same line.
	values[zeKeywordAccept] = "false"

	for _, required := range neighborCreateRequired {
		if _, stated := values[required]; !stated {
			return Translation{}, true, errNeighborCreateRequired
		}
	}

	var tb textbuf.Buffer
	tb.Str("create bgp peer ").Str(address)
	for _, keyword := range neighborCreateOrder {
		value, stated := values[keyword]
		if !stated {
			continue
		}
		tb.Byte(' ').Str(keyword).Byte(' ').Str(value)
	}

	return Translation{Commands: textCommands(tb.String()), Selector: address}, true, nil
}

// convertNeighborDelete writes the ze command that removes a peer from the
// running daemon.
//
// ExaBGP takes a filter after the selector (`peer delete 127.0.0.2 local-as 1`)
// and `delete bgp peer` takes the selector alone, so a filtered line is refused
// rather than widened into a delete the script did not ask for.
func convertNeighborDelete(fields []string) (Translation, bool, error) {
	if len(fields) == 0 {
		return Translation{}, true, errNeighborDeleteAddress
	}
	if len(fields) > 1 {
		return Translation{}, true, fmt.Errorf("%w: %q", errNeighborDeleteFilter, textbuf.Join(fields[1:], " "))
	}

	var tb textbuf.Buffer
	return Translation{
		Commands: textCommands(tb.Str("delete bgp peer ").Str(fields[0]).String()),
		Selector: fields[0],
	}, true, nil
}

// neighborValueAsIs carries a value ze spells the same way ExaBGP does.
func neighborValueAsIs(value string) (string, error) { return value, nil }

// neighborFamilies rewrites ExaBGP's family list into ze's.
//
// ExaBGP separates families with `/` and joins the AFI to the SAFI with `-`
// (_parse_families). ze writes one family as `afi/safi` and separates them with
// a comma, so `ipv4-unicast/ipv6-unicast` becomes `ipv4/unicast,ipv6/unicast`.
// The NAMES are not translated: ze validates each one against its family
// registry and refuses the line naming the family it did not find.
func neighborFamilies(value string) (string, error) {
	if strings.EqualFold(value, "in-open") {
		return "", errNeighborFamilyInOpen
	}

	families := strings.Split(value, "/")
	converted := make([]string, 0, len(families))
	for _, exabgp := range families {
		afi, safi, split := strings.Cut(exabgp, "-")
		if !split || afi == "" || safi == "" || strings.Contains(safi, "-") {
			return "", fmt.Errorf("%w: %q", errNeighborFamilyForm, exabgp)
		}
		var tb textbuf.Buffer
		converted = append(converted, tb.Str(afi).Byte('/').Str(safi).String())
	}
	return textbuf.Join(converted, ","), nil
}
