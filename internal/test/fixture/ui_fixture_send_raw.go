// Design: docs/architecture/api/commands.md -- the raw injection command this drives
// Related: ui_fixture_cli_announce.go -- startCLIWireSession, the daemon and peer harness this reuses
//
// raw is the one send form that reaches exactly ONE session. Everything else in
// the family fans out, so raw is where the arity guard lives and where a
// functional test earns the most: the guard is now two tokens further from the
// operator's eye than it was under the old grammar, and a refusal that stopped
// firing would send unvalidated bytes to every peer.
//
// The peer holds the assertion. Its script states the octets it must receive,
// and ze-test peer exits zero only when each one arrived, so this proves the
// operator's bytes reached the wire rather than only that the command exited
// zero.

package fixture

import (
	"context"
	"fmt"
	"strings"
	"time"
)

func init() {
	registerFixture("ui/send-raw-reaches-one-peer", sendRawReachesOnePeer)
}

// sendRawPacketHex is the whole BGP message this fixture injects, marker and
// header included, because no type keyword is typed.
//
// It is an UPDATE that withdraws 10.9.9.0/24: a two-octet withdrawn-routes
// length of 4, the prefix in the length-then-significant-octets form RFC 4271
// Section 4.3 states, and a two-octet path-attribute length of zero. The
// withdrawal is what makes it distinguishable at the peer. An UPDATE with an
// empty body is byte-identical to the End-of-RIB marker the session already
// sends, so it would prove nothing.
const sendRawPacketHex = "FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001B020004180A09090000"

// sendRawForm is the form word this fixture types. It names the send form, not
// the pipe operator of the same spelling, so it is a constant of its own rather
// than a reuse of renderRaw.
const sendRawForm = "raw"

// sendRawHexEncoding is the encoding word the data below is written in. The
// model declares it as an enumeration of hex and b64.
const sendRawHexEncoding = "hex"

// sendRawPeer is the selector that names the one session this fixture injects
// into. It is the address the daemon's configuration dials.
const sendRawPeer = "127.0.0.1"

// sendRawReachesOnePeer proves that `send bgp <selector> raw <encoding> <data>`
// puts the operator's octets on one peer's wire, and that every selector which
// does not name exactly one session is refused by name instead.
func sendRawReachesOnePeer(ctx context.Context, args []string) error {
	session, err := startCLIWireSession(ctx, args)
	if err != nil {
		return err
	}
	defer session.stop()

	// The refusals run FIRST. Each one must send nothing, and the peer script
	// expects exactly one injected message, so a refusal that leaked would
	// arrive before the accepted injection and fail the script on the octets.
	refusals := []struct {
		argv []string
		says string
	}{
		// The wildcard. This is the whole arity property: raw acts on one
		// session, and "every peer" is the selector that must never reach it.
		{[]string{"send", "bgp", "*", sendRawForm, sendRawHexEncoding, sendRawPacketHex}, "one specific peer"},
		// An exclusion names a set, and which peer it lands on depends on how
		// many are configured.
		{[]string{"send", "bgp", "!peer1", sendRawForm, sendRawHexEncoding, sendRawPacketHex}, "exclusion selector"},
		// The path this grammar left. It matches nothing, so no alias survives.
		{[]string{"peer", sendRawPeer, sendRawForm, sendRawHexEncoding, sendRawPacketHex}, ""},
	}
	for _, refusal := range refusals {
		line := strings.Join(refusal.argv, " ")
		result := session.run(ctx, refusal.argv...)
		if result.code == 0 {
			return fmt.Errorf("ze %s exit=0, want a refusal: %s", line, result.out)
		}
		if refusal.says == "" {
			continue
		}
		if !strings.Contains(result.out, refusal.says) {
			return fmt.Errorf("ze %s did not say %q: %s", line, refusal.says, result.out)
		}
	}

	injected := session.run(ctx, "send", "bgp", sendRawPeer, sendRawForm, sendRawHexEncoding, sendRawPacketHex)
	if injected.code != 0 {
		return fmt.Errorf("ze send bgp %s raw hex exit=%d, want 0: %s", sendRawPeer, injected.code, injected.out)
	}

	if err := waitFixtureProcess(ctx, session.peer, 20*time.Second); err != nil {
		return fmt.Errorf("the peer did not receive the injected message: %w\n%s", err, session.peer.output.String())
	}

	fmt.Println("OK")
	return nil
}
