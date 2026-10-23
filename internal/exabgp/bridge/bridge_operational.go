// Design: docs/architecture/exabgp-bridge.md -- what the bridge refuses
// Related: bridge_command.go -- Translator.Line, the caller

package bridge

import (
	"errors"
	"fmt"
	"strings"
)

// operationalKinds are the OPERATIONAL message kinds ExaBGP knows
// (OPERATIONAL_KINDS, src/exabgp/reactor/api/command/announce.py): advisory
// (asm, adm), and the RPC, ADJ-RIB and LOC-RIB prefix count query and reply
// pairs of draft-ietf-idr-operational-message.
var operationalKinds = map[string]bool{
	"asm": true, "adm": true, "rpcq": true, "rpcp": true,
	"apcq": true, "apcp": true, "lpcq": true, "lpcp": true,
}

var (
	// errOperationalUnknown answers an `announce operational` line whose kind
	// ExaBGP does not know, as its announce_operational does ("unknown
	// operational message: <command>").
	errOperationalUnknown = errors.New("unknown operational message")
	// errOperationalNotSent answers a known kind: Ze does not originate the
	// OPERATIONAL message, which Ze does not support (owner decision,
	// ai/META-DECISIONS.md; docs/exabgp/exabgp-differences.md).
	errOperationalNotSent = errors.New("the OPERATIONAL message is not sent by Ze")
)

// operationalWord is the command word of `[announce] operational <kind> ...`.
const operationalWord = "operational"

// convertOperational refuses ExaBGP's `announce operational <kind> ...`, the
// API 4 form, and `operational <kind> ...`, the API 6 form after its selector.
// ok is false for a line that is neither, which the caller translates on.
//
// Both refusals name the command. An unknown kind is refused as ExaBGP refuses
// it; a known kind is a message ExaBGP would send and Ze cannot, so the reason
// says so and names what the operator can do instead.
func convertOperational(rest string) (ok bool, err error) {
	words := strings.Fields(strings.ToLower(rest))
	position := 1
	switch {
	case len(words) >= 2 && words[0] == announceVerb && words[1] == operationalWord:
		position = 2
	case len(words) >= 1 && words[0] == operationalWord:
	default:
		return false, nil
	}
	kind := ""
	if len(words) > position {
		kind = words[position]
	}
	if !operationalKinds[kind] {
		return true, fmt.Errorf("%w: %s; the kinds are asm, adm, rpcq, rpcp, apcq, apcp, lpcq and lpcp",
			errOperationalUnknown, rest)
	}
	return true, fmt.Errorf("%w (%s): %s; remove the line from the script, or send it from ExaBGP",
		errOperationalNotSent, kind, rest)
}
