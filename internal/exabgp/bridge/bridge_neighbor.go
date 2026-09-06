// Design: docs/architecture/exabgp-bridge.md -- the neighbor lifecycle vocabulary
// RFC: rfc/short/rfc4271.md -- Section 4.5, the NOTIFICATION subcode is one octet
// RFC: rfc/short/rfc4486.md -- the Cease subcodes a teardown chooses from
// RFC: rfc/short/rfc8203.md -- the shutdown communication ze supplies by default
// Overview: bridge_command.go -- the line translator that calls this converter
// Related: bridge_event.go -- the event encoder, which writes JSON and not text
//
// ExaBGP's API drives a neighbor as well as its routes: it creates one, tears
// one down, and reports what one received. The three verbs land in three
// different places, and this file is where that is decided rather than guessed.
// One is a ze command, one is a command ze does not have, and one is not a
// command at all.

package bridge

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// errNeighborCreate is the answer for `create neighbor`. ze has no command that
// creates a BGP peer while it runs: `ze-bgp:peer-add` is declared in
// internal/component/bgp/yang/ze-bgp-api.yang and no handler registers it, so
// `./le command list` shows no CLI path that reaches it. A peer is created by
// editing the config tree and committing, which is not one line a script writes.
//
// The refusal is named rather than approximated. `delete bgp peer` exists and
// `request peer <sel> teardown` exists, and neither one creates anything, so a
// mapping to either would acknowledge the script for a session that was never
// created (ai/rules/principles.md).
var errNeighborCreate = errors.New("ze creates no BGP peer at runtime: ze-bgp:peer-add is declared with no handler and no CLI path, so a peer is created by config and commit")

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
//	create neighbor <ip> ...          -> refused, ze has no such command
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
func ConvertNeighborControl(selector, rest string) (Translation, bool, error) {
	fields := strings.Fields(strings.ToLower(rest))
	if len(fields) == 0 {
		return Translation{}, false, nil
	}

	switch {
	case fields[0] == "teardown":
		return convertTeardown(selector, fields)
	case fields[0] == "create" && len(fields) > 1 && fields[1] == "neighbor":
		return Translation{}, true, fmt.Errorf("%w: %q", errNeighborCreate, rest)
	case fields[0] == "receive":
		return Translation{}, true, fmt.Errorf("%w: %q", errNeighborReceive, rest)
	}
	return Translation{}, false, nil
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
		Commands: []string{tb.Str("request peer ").Str(selector).Str(" teardown ").Uint8(uint8(subcode)).String()},
		Selector: selector,
	}, true, nil
}
