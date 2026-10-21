// Design: docs/architecture/core-design.md — ExaBGP text command to ZeBGP translation
// Overview: bridge.go — startup protocol, bridge runtime
// Related: bridge_event.go — ZeBGP to ExaBGP JSON event translation
// Related: bridge_muxconn.go — MuxConn wire format parsing for post-startup I/O

package bridge

import (
	"errors"
	"fmt"
	"net/netip"
	"strings"

	"github.com/ze-software/ze/internal/component/bgp/textparse"
	"github.com/ze-software/ze/internal/core/bgp/nlri/flowspec"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// ExaBGP's own word for each path attribute. One declaration serves both
// directions: the route grammar a script writes DOWN (bridgeRouteAttrs) and the
// text rendering of an event that goes UP (textAttributes). They are the same
// vocabulary, and two copies of it would disagree the first time ExaBGP renamed
// one.
const (
	bridgeAttrNextHop          = "next-hop"
	bridgeAttrOrigin           = "origin"
	bridgeAttrMED              = "med"
	bridgeAttrLocalPreference  = "local-preference"
	bridgeAttrASPath           = "as-path"
	bridgeAttrCommunity        = "community"
	bridgeAttrLargeCommunity   = "large-community"
	bridgeAttrExtCommunity     = "extended-community"
	bridgeAttrExtCommunityIPv6 = "extended-community-ipv6"
	bridgeAttrAtomicAggregate  = "atomic-aggregate"
	bridgeAttrAggregator       = "aggregator"
	bridgeAttrOriginatorID     = "originator-id"
	bridgeAttrClusterList      = "cluster-list"
	bridgeAttrAIGP             = "aigp"
	bridgeAttrLabel            = "label"
	bridgeAttrRouteDist        = "route-distinguisher"
	bridgeAttrPathInformation  = "path-information"
	bridgeAttrWatchdog         = "watchdog"
)

// The two keys an UPDATE's body carries in ze JSON: its path attributes and its
// NLRI, by family (docs/architecture/api/json-format.md).
const (
	bridgeUpdateAttr = "attr"
	bridgeUpdateNLRI = "nlri"
)

// The two verbs an ExaBGP route line starts with, and the two-word forms
// convertRoute reads first.
const (
	announceRoute = "announce route"
	withdrawRoute = "withdraw route"
	announceVerb  = "announce"
	withdrawVerb  = "withdraw"
)

// bridgeEveryPeer is the ze peer selector for every configured session. An
// ExaBGP line that names no neighbor goes to every neighbor, so the bridge
// translates it with this selector.
const bridgeEveryPeer = "*"

// Translation is what the bridge makes of one ExaBGP line: the ze command to
// dispatch, and the selector whose forward pool the caller flushes once that
// command is acknowledged.
//
// The selector travels WITH the command because the builder that wrote the
// command already knew it. Reading it back out of the finished string stated one
// fact twice, and the two statements disagreed in silence: the route test
// answered on a substring that survives any change of leading token, while the
// selector test answered on that token, so a moved grammar left every flush
// skipped with no error and no log line (ai/rules/principles.md).
type Translation struct {
	// Commands are the lines to hand to ze's dispatcher, in order. Most ExaBGP
	// lines become exactly one.
	//
	// One line becomes SEVERAL because ExaBGP puts each prefix on its OWN
	// UPDATE. `announce attributes med 100 next-hop 101.1.101.1 nlri
	// 1.0.0.1/32 1.0.0.2/32` is two UPDATEs on the wire, and a route carrying
	// `split /23` is one per piece. A single ze command naming both prefixes is
	// ONE update, which is a different thing on the wire from what the script
	// asked for, and it is what api-attributes and api-announcement caught.
	Commands []Command
	// Selector names the peers Command addresses. It is set whenever Route is,
	// and no caller reads it otherwise.
	Selector string
	// Route says Command puts an UPDATE on a wire, so a per-peer flush is owed
	// after the dispatch is acknowledged.
	Route bool
	// Local is set when the bridge answers the line itself. Command is empty
	// then, and a caller MUST read this before it reads Nothing: a local action
	// carries no command and is not an empty line.
	Local LocalAction
	// APIVersion is the `api version` line when Local is LocalAPIVersion. The
	// answer depends on the script's version and the fleet's setting, which the
	// runner holds, so the runner performs it (APIVersionRequest.Answer).
	APIVersion APIVersionRequest
}

// Command is one ze command line, with the route it carries named by the
// translator that wrote it.
//
// The key travels WITH the command for the reason the selector does: the
// builder that wrote the command already held the family, the verb and the
// NLRI. Reading them back out of the finished string would state one fact
// twice, and the second statement drifts the moment the `send bgp` grammar
// moves, with no test red (ai/rules/principles.md).
type Command struct {
	// Text is the ze command line to dispatch.
	Text string
	// Key names the routes Text carries, for the batch netting. It is zero for
	// every command that carries no route.
	Key RouteKey
	// Withdrawal is the ze command that withdraws what Text announces, written
	// by the builder that wrote Text, from the same tokens. The runner
	// dispatches it when the script that announced the route exits with its
	// routes withdrawn (ExaBGP's withdraw_owner). It is empty for a withdrawal
	// and for every command that carries no route.
	Withdrawal string
	// WithdrawsAdjRIBOut says Text withdraws every route the Adj-RIB-Out of
	// its line's selector holds: ExaBGP's `clear adj-rib out`. Net cancels the
	// route commands written for that selector earlier in the batch and keeps
	// every command written after it after it (bridge_batch.go).
	WithdrawsAdjRIBOut bool
}

// RouteKey names the routes one command carries.
//
// It is the family and the NLRI tokens as the command writes them, which is
// the key ExaBGP nets on: `Change.index` is the family plus `NLRI.index()` and
// carries no action, so one route's announce and its withdrawal share a key
// (src/exabgp/rib/change.py, upstream exa-networks/exabgp).
type RouteKey struct {
	// Family is the ze family word: `ipv4/unicast`, `ipv4/flow` and the rest.
	Family string
	// NLRI is the command's NLRI tokens, joined by one space, in the order the
	// command writes them. A command that carries several prefixes carries them
	// all, so a withdrawal cancels an announce only when it names exactly the
	// same routes. That is coarser than ExaBGP's per-prefix index and it errs
	// the safe way: it can leave a frame on the wire, never remove one.
	NLRI string
	// Withdraw says the command's NLRI verb is `del`.
	Withdraw bool
}

// Route reports whether this key names a route the netting can act on.
//
// A command that carries no route -- an End-of-RIB, a flush, a teardown -- has
// the zero key, and the zero key is NOT a route rather than a route with an
// empty name: two commands that name nothing must never cancel each other
// (ai/rules/principles.md).
func (k RouteKey) Route() bool { return k.Family != "" && k.NLRI != "" }

// routeIdentity is one route without the action, which is what an announce and
// its withdrawal share.
type routeIdentity struct {
	family string
	nlri   string
}

func (k RouteKey) identity() routeIdentity {
	return routeIdentity{family: k.Family, nlri: k.NLRI}
}

// textCommands wraps command lines that carry no route.
func textCommands(texts ...string) []Command {
	commands := make([]Command, 0, len(texts))
	for _, text := range texts {
		commands = append(commands, Command{Text: text})
	}
	return commands
}

// LocalAction is a command the bridge performs ITSELF rather than dispatching.
//
// The ExaBGP API's ack control is the whole set. A script turns acks off, sends
// commands it does not want answered, and turns them back on, and the three
// words differ in WHEN the silence starts: `disable-ack` is acked and silences
// what follows, `silence-ack` is not acked at all, and `enable-ack` is acked and
// resumes.
type LocalAction uint8

const (
	// LocalNone is the ordinary line: it carries a command to dispatch.
	LocalNone LocalAction = iota
	// LocalAckEnable resumes acks, and this command is acked.
	LocalAckEnable
	// LocalAckDisableAfter stops acks after this command, which is acked.
	LocalAckDisableAfter
	// LocalAckDisableNow stops acks immediately, so this command is not acked.
	LocalAckDisableNow
	// LocalPing answers ExaBGP's health check as JSON, then done.
	LocalPing
	// LocalPingText answers the health check as text, then done: the line
	// carried the `text` keyword.
	LocalPingText
	// LocalAPIVersion is `api version [<value>]`: the runner writes its answer
	// line, then done, or error when the value is refused.
	LocalAPIVersion
	// LocalStatus is `status` or `daemon status`: the runner reads every
	// peer's state from ze and writes ExaBGP's daemon status, then done
	// (StatusPeers, StatusReply).
	LocalStatus
	// LocalGroup is a `group` line, or a route line an open group holds: the
	// runner writes the group's answer line, if any, then done, or error when
	// the group refused the line (Group, ScriptLines.Batch).
	LocalGroup
)

// statusWord is ExaBGP's daemon status command, `status` in API 4 and
// `daemon status` in API 6.
const statusWord = "status"

// shutdownWord and pingWord are ExaBGP's v4 spellings of `daemon shutdown` and
// `session ping` (v6ControlSpellings).
const (
	shutdownWord = "shutdown"
	pingWord     = "ping"
)

// Nothing reports a line that carries no command: a blank line, or a comment.
// It is an ANSWER rather than a failure, and it is named so that no caller has
// to read an empty Command as one.
func (t Translation) Nothing() bool { return len(t.Commands) == 0 }

// ErrLineNotTranslated is what TranslateLine answers for a line the bridge does
// not read. The bridge refuses such a line rather than forwarding it, because a
// forwarded line dies at ze's dispatcher as an unknown command with no mention
// of the bridge that sent it.
var ErrLineNotTranslated = errors.New("the ExaBGP bridge does not translate this line")

// bridgePassthrough is the ONE ExaBGP line that still reaches ze's dispatcher
// unchanged. `help` is the bridge's own word rather than a route, ze declares it
// as ze-meta:bgp-help, and it is the single member bridgeSurface keeps
// (internal/component/command/grammar/checker.go).
const bridgePassthrough = "help"

// TranslateLine converts one ExaBGP text command into the ze command that sends
// it, and names the peers that command addresses.
//
// ExaBGP: neighbor <ip> announce route <prefix> next-hop <nh> [origin <o>] ...
// ZeBGP:  send bgp <ip> update text nhop <nh> origin <o> nlri add <prefix>.
//
// A line that names no neighbor names no destination, and ExaBGP sends such a
// line to the peers that attach the process that wrote it. This package
// function carries no process, so it sends to every peer: `announce route
// <prefix>` becomes `send bgp * update text nlri add <prefix>`.
// Translator.Peers is how a script narrows that.
//
// Every other line is REFUSED by name. The passthrough that forwarded it used to
// carry ze's own announce and withdraw spellings, which have moved under
// `send bgp <selector>` and are translator output now, so what it carried is
// gone and what remains of it is an untyped path from a script's stdout to ze's
// dispatcher.
func TranslateLine(line string) (Translation, error) {
	return Translator{}.Line(line)
}

// Translator carries what a translation needs beyond the line itself.
//
// A line that names no neighbor is every peer that attaches THIS script, and
// which peers those are is the neighbor's `api { processes [ ... ] }` list
// rather than anything the script writes. A zero Translator sends an
// unaddressed line to every peer, which is what one process serving every
// neighbor means.
type Translator struct {
	// Families are the families this bridge declared, in ze spelling
	// ("ipv4/unicast"). No translation reads them since a bare End-of-RIB
	// became IPv4 unicast alone, as ExaBGP's api_eor reads it. The field and
	// its writers (bridge.go, bridgerun.New) are owed a removal.
	Families []string
	// Peers are the addresses of the peers that attach the script this
	// translator serves. An empty list is every peer.
	//
	// ExaBGP scopes a process's commands the same way: Reactor.peers(service)
	// answers every peer when the service is empty, and otherwise the peers
	// whose api block names that process
	// (src/exabgp/reactor/loop.py, Reactor.peers).
	Peers []string
}

// errSelectorMatchesNoPeer refuses a line whose selector names no peer this
// script feeds: every address it named is outside Translator.Peers, or carried
// a qualifier no ze session can satisfy (splitNeighborSelector).
var errSelectorMatchesNoPeer = errors.New("the selector matches no peer")

// servedSelector narrows a selector of addresses to the peers this script
// feeds, as ExaBGP matches a selector against Reactor.peers(service) only
// (match_neighbors, reactor/api/command/limit.py). unmatched is set when none
// is left. A translator serving every peer, and the `*` selector, pass whole:
// ze's own dispatcher refuses an address it holds no peer for
// (route.ErrNoPeersMatch).
func (t Translator) servedSelector(selector string) (narrowed string, unmatched bool) {
	if len(t.Peers) == 0 || selector == bridgeEveryPeer || selector == t.everyPeer() {
		return selector, false
	}
	served := make(map[netip.Addr]bool, len(t.Peers))
	for _, peer := range t.Peers {
		if address, err := netip.ParseAddr(peer); err == nil {
			served[address] = true
		}
	}
	var kept []string
	for named := range strings.SplitSeq(selector, ",") {
		address, err := netip.ParseAddr(named)
		if err != nil {
			// Not an address: ze's dispatcher reads it and answers for it.
			kept = append(kept, named)
			continue
		}
		if served[address] {
			kept = append(kept, named)
		}
	}
	if len(kept) == 0 {
		return "", true
	}
	return selectorForAddresses(kept), false
}

// everyPeer answers the selector for a line that names no neighbor: the peers
// this script serves, or the wildcard when it serves them all.
func (t Translator) everyPeer() string {
	if len(t.Peers) == 0 {
		return bridgeEveryPeer
	}
	return selectorForAddresses(t.Peers)
}

// Line converts one ExaBGP text command, as TranslateLine does, with the
// translator's own context available to it.
func (t Translator) Line(line string) (Translation, error) {
	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, "#") {
		return Translation{}, nil
	}

	// `api version` reads or moves the bridge's own setting and reaches no
	// peer, so it is read before any selector.
	if request, ok := apiVersionRequest(line); ok {
		return Translation{Local: LocalAPIVersion, APIVersion: request}, nil
	}

	// ExaBGP pads a command to align it in a script, and its own qa/api corpus
	// writes `announce` and `route` with 38 spaces between them. Every match
	// below is on token text, so the run of spaces is collapsed once here
	// rather than guarded against at each site. A v6 control line is read in
	// its v4 spelling from here on (v4ControlSpelling), so both spellings of a
	// word reach one translation, and one refusal, which still quotes the line
	// as the script wrote it.
	collapsed := v4ControlSpelling(strings.Join(strings.Fields(line), " "))
	selector, rest, unmatched := splitNeighborSelector(collapsed, t.everyPeer())
	if !unmatched {
		selector, unmatched = t.servedSelector(selector)
	}
	if unmatched {
		// X-24: ExaBGP's dispatcher refuses a selector that matches no peer
		// (NoMatchingPeers, reactor/api/dispatch/common.py) and answers the
		// error marker; it sends nothing.
		return Translation{}, fmt.Errorf("%w: %q; name a peer this process feeds, or write no selector",
			errSelectorMatchesNoPeer, line)
	}

	if translation, ok := convertControl(selector, rest); ok {
		return translation, nil
	}
	if translation, ok, err := convertNeighborControl(selector, rest); ok {
		if err != nil {
			return Translation{}, fmt.Errorf("%w: %q", err, line)
		}
		return translation, nil
	}
	if translation, ok, err := convertRouteRefresh(selector, rest); ok {
		if err != nil {
			return Translation{}, fmt.Errorf("%w: %q", err, line)
		}
		return translation, nil
	}
	if refused, err := convertOperational(rest); refused {
		return Translation{}, err
	}
	if commands, translated, err := t.convertRoute(selector, rest); translated {
		if err != nil {
			return Translation{}, fmt.Errorf("%w: %q", err, line)
		}
		return Translation{Commands: commands, Selector: selector, Route: true}, nil
	}
	if strings.EqualFold(rest, bridgePassthrough) {
		return Translation{Commands: textCommands(bridgePassthrough)}, nil
	}
	return Translation{}, fmt.Errorf("%w: %q", ErrLineNotTranslated, line)
}

// convertControl translates the ExaBGP API commands that are not routes.
//
// An ExaBGP script drives more than announcements: it clears and flushes the
// adj-RIB, turns the command acknowledgement on and off, drives a watchdog
// group, and asks the daemon to stop. The bridge answered none of these until
// 2026-09-05, so every script that used one stopped there, and 29 of the 40
// ported qa/api tests ended after their first frame.
//
// Each mapping is to a command ze declares, checked against the live registry
// rather than assumed:
//
//	clear adj-rib in            -> clear bgp rib in <sel>         (ze-bgp:rib-clear-in)
//	clear adj-rib out           -> clear bgp rib advertised <sel> (ze-bgp:rib-clear-advertised)
//	flush adj-rib out           -> clear bgp rib out <sel>        (ze-bgp:rib-clear-out)
//	announce watchdog <name>    -> request bgp watchdog announce <name>
//	withdraw watchdog <name>    -> request bgp watchdog withdraw <name>
//	shutdown, request shutdown  -> request shutdown           (ze-plugin:system-daemon-shutdown)
//	enable-ack, disable-ack, silence-ack -> answered by the bridge itself
//	ping [text]                 -> answered by the bridge itself (bridge_ping.go)
//
// Each v6 spelling (`daemon shutdown`, `session ack disable`, `session ping`,
// `rib clear out`, ...) arrives here already in its v4 spelling
// (v4ControlSpelling in Translator.Line).
//
// The watchdog forms are read BEFORE convertRoute although they start with the
// announce and withdraw verbs. convertRoute would refuse them anyway, because
// `watchdog <name>` states no family, but a reader should not have to know that
// to see which branch takes them.
func convertControl(selector, rest string) (Translation, bool) {
	fields := strings.Fields(strings.ToLower(rest))
	if len(fields) == 0 {
		return Translation{}, false
	}
	var tb textbuf.Buffer
	// enumeration: exempt (these are ExaBGP's own API control words, not the CLI verb vocabulary)
	switch {
	case len(fields) == 1 && fields[0] == "enable-ack":
		return Translation{Local: LocalAckEnable}, true
	case len(fields) == 1 && fields[0] == "disable-ack":
		return Translation{Local: LocalAckDisableAfter}, true
	case len(fields) == 1 && fields[0] == "silence-ack":
		return Translation{Local: LocalAckDisableNow}, true
	case fields[0] == pingWord:
		return Translation{Local: pingAction(fields)}, true
	case fields[0] == statusWord:
		return Translation{Local: LocalStatus}, true
	case len(fields) == 1 && fields[0] == shutdownWord,
		len(fields) == 2 && fields[0] == "request" && fields[1] == shutdownWord:
		return Translation{Commands: textCommands("request shutdown"), Selector: selector}, true
	case len(fields) == 3 && fields[0] == "clear" && fields[1] == "adj-rib" && fields[2] == "in":
		// ze refuses `clear bgp rib in|out|advertised` without a selector,
		// because the all-peers effect must be typed. ExaBGP's neighbor-less
		// form means every peer the process serves, which is what selector
		// already holds (everyPeer), so the selector is always written in.
		return Translation{
			Commands: textCommands(tb.Str("clear bgp rib in ").Str(selector).String()),
			Selector: selector,
		}, true
	case len(fields) == 3 && fields[0] == "clear" && fields[1] == "adj-rib" && fields[2] == "out":
		// ExaBGP withdraws every route the adj-rib-out holds
		// (OutgoingRIB.withdraw at the pin): ze's engine-side withdrawal of
		// the session's Adj-RIB-Out, not bgp-rib's resend.
		command := Command{Text: tb.Str("clear bgp rib advertised ").Str(selector).String(), WithdrawsAdjRIBOut: true}
		return Translation{Commands: []Command{command}, Selector: selector}, true
	case len(fields) == 3 && fields[0] == "flush" && fields[1] == "adj-rib" && fields[2] == "out":
		// ExaBGP resends the adj-rib-out (OutgoingRIB.resend at the pin),
		// bracketed by BoRR and EoRR when Enhanced Route Refresh was
		// negotiated: bgp-rib's resend does both.
		return Translation{
			Commands: textCommands(tb.Str("clear bgp rib out ").Str(selector).String()),
			Selector: selector,
		}, true
	case len(fields) == 3 && fields[1] == bridgeAttrWatchdog &&
		(fields[0] == announceVerb || fields[0] == withdrawVerb):
		name := strings.Fields(rest)[2]
		return Translation{
			Commands: textCommands(tb.Str("request bgp watchdog ").Str(fields[0]).Byte(' ').Str(name).String()),
			Selector: selector,
		}, true
	}
	return Translation{}, false
}

// convertRoute translates one ExaBGP announce or withdraw into the ze command
// that sends it to the peers the selector names. It reports false when the line
// names no form the bridge translates, and it writes nothing then.
//
// The caller decides what an untranslated line becomes, because the answer
// differs between a line that names a neighbor and a line that does not.
func (t Translator) convertRoute(selector, rest string) ([]Command, bool, error) {
	restLower := strings.ToLower(rest)

	// The three forms that are not `<verb> <route>`: an End-of-RIB, which
	// carries no route; the attributes form, which carries several; and the
	// braced flow route, whose body is a nested grammar rather than a token
	// run (bridge_flow.go).
	if commands, matched, err := convertEOR(selector, rest); matched {
		return commands, true, err
	}
	if strings.HasPrefix(restLower, announceVerb) || strings.HasPrefix(restLower, withdrawVerb) {
		verb := nlriAdd
		body := strings.TrimSpace(rest[len(announceVerb):])
		if strings.HasPrefix(restLower, withdrawVerb) {
			verb = nlriDel
			body = strings.TrimSpace(rest[len(withdrawVerb):])
		}
		if commands, matched, err := convertAttributesForm(selector, rest, verb); matched {
			return commands, true, err
		}
		// ExaBGP writes a flowspec route with no family word, `flow route
		// ...`, as well as the family-qualified `ipv4 flow ...` that
		// convertFamilyRoute reads below.
		if command, matched, err := convertFlowRoute(selector, flowFamilyDefault, verb, body); matched {
			if err != nil {
				return nil, true, err
			}
			return []Command{command}, true, nil
		}
	}

	if strings.HasPrefix(restLower, announceRoute) {
		command, err := convertAnnounce(selector, rest[len(announceRoute):])
		return command, true, err
	}
	if strings.HasPrefix(restLower, withdrawRoute) {
		command, err := convertWithdraw(selector, rest[len(withdrawRoute):])
		return command, true, err
	}
	if strings.HasPrefix(restLower, announceVerb) {
		return convertAnnounceFamily(selector, rest[len(announceVerb):])
	}
	if strings.HasPrefix(restLower, withdrawVerb) {
		return convertWithdrawFamily(selector, rest[len(withdrawVerb):])
	}
	return nil, false, nil
}

// buildRouteCommand renders one ze update-text command from a family, an NLRI
// and the attribute tail that follows it.
//
// It is the ONE place a route command is assembled, so the two ExaBGP spellings
// -- `announce route <prefix>` and `announce <afi> <safi> <nlri>` -- cannot
// disagree about which attributes they carry, which is exactly what they did
// until 2026-09-05 (bridge_attribute.go).
func buildRouteCommand(selector, family, verb, body string) ([]Command, error) {
	nlriTokens, attrTokens := splitRouteBodyForFamily(family, strings.Fields(strings.TrimSpace(body)))
	nlriTokens = shapePluginNLRI(family, nlriTokens)
	nlriTokens, err := defaultPrefixLengths(family, nlriTokens)
	if err != nil {
		return nil, err
	}
	attrs, err := parseRouteAttributes(attrTokens)
	if err != nil {
		return nil, err
	}
	// ExaBGP's `withdraw` flag announces the route in its withdrawn state, so
	// it flips the NLRI verb rather than adding an attribute.
	if attrs.Withdraw {
		verb = nlriDel
	}
	// `split /<n>` announces the PIECES and not the prefix that was written, so
	// the one NLRI becomes several before the command is built.
	if attrs.Split > 0 {
		if len(nlriTokens) != 1 {
			return nil, fmt.Errorf("%w: split needs exactly one prefix", errSplitLength)
		}
		pieces, whole, err := splitPrefix(nlriTokens[0], attrs.Split)
		if err != nil {
			return nil, err
		}
		if whole {
			var tb textbuf.Buffer
			tb.Str("split /").Int(int64(attrs.Split)).Str(" is shorter than the prefix ").
				Str(nlriTokens[0]).Str(", so the prefix is sent whole")
			attrs.Warnings = append(attrs.Warnings, tb.String())
		}
		nlriTokens = pieces
	}
	warnRouteLine(attrs)

	return oneCommandPerNLRI(selector, family, verb, attrs, nlriTokens, false), nil
}

// oneCommandPerNLRI renders the ze commands for one ExaBGP route line.
//
// perNLRI is FALSE for every caller today, and the parameter is kept because
// the question it asks is real: are these tokens several routes or the several
// tokens of one? What it must NOT be used for is framing. Whether two prefixes
// share an UPDATE is the destination peer's `behavior { group-updates }` leaf,
// and splitting here decided it for ze: api-attributes-path leaves the leaf at
// its default and expects ONE frame carrying both prefixes, while
// api-attributes sets it false and expects two.
func oneCommandPerNLRI(selector, family, verb string, attrs routeAttributes, nlriTokens []string, perNLRI bool) []Command {
	// The key is stated HERE, beside the tokens it names, so no consumer reads
	// it back out of the command text (ai/rules/principles.md). An announce and
	// a withdrawal of one route reach this function with the same family and
	// the same tokens, and differ in the verb alone, so the two carry one
	// identity and the batch netting can cancel one against the other.
	write := func(tokens []string, verb string) Command {
		var tb textbuf.Buffer
		tb.Str("send bgp ").Str(selector).Str(" update text")
		if command := attrs.Command(); command != "" {
			tb.Byte(' ').Str(command)
		}
		// ze names the UPDATE section with its keyword: `nlri` for IPv4 unicast
		// in the legacy fields (RFC 4271 Section 4.3), `mp <family>` otherwise.
		if textparse.LegacyNLRISection(family, verb == nlriAdd && attrs.nextHopIPv6()) {
			tb.Str(" nlri ").Str(verb)
		} else {
			tb.Str(" mp ").Str(family).Byte(' ').Str(verb)
		}
		for _, token := range tokens {
			tb.Byte(' ').Str(token)
		}
		return Command{
			Text: tb.String(),
			Key: RouteKey{
				Family:   family,
				NLRI:     textbuf.Join(tokens, " "),
				Withdraw: verb == nlriDel,
			},
		}
	}
	// An announce carries its own withdrawal: the same attributes and NLRI
	// tokens under `del`, which is what `withdraw route` with the same body
	// writes.
	announce := func(tokens []string) Command {
		command := write(tokens, verb)
		if verb == nlriAdd {
			command.Withdrawal = write(tokens, nlriDel).Text
		}
		return command
	}

	if !perNLRI {
		return []Command{announce(nlriTokens)}
	}
	commands := make([]Command, 0, len(nlriTokens))
	for _, token := range nlriTokens {
		commands = append(commands, announce([]string{token}))
	}
	return commands
}

const (
	nlriAdd = "add"
	nlriDel = "del"
)

// flowFamilyDefault is the family a family-less `flow route ...` starts in.
// ExaBGP's own grammar states no family on that form and reads the AFI off the
// first address in the body, which convertFlowRoute does too: it promotes to
// ipv6 on an IPv6 match component and to flow-vpn on a route distinguisher.
const flowFamilyDefault = "ipv4/" + bridgeFlowSAFI

// convertAnnounceFamily translates an ExaBGP announce that states its family,
// which is every announce apart from a plain `announce route`. It reports false
// when the text after the verb states no family the bridge reads.
func convertAnnounceFamily(selector, rest string) ([]Command, bool, error) {
	return convertFamilyRoute(selector, rest, nlriAdd)
}

// convertWithdrawFamily translates an ExaBGP withdraw that states its family,
// which is every withdraw apart from a plain `withdraw route`. It reports false
// when the text after the verb states no family the bridge reads.
func convertWithdrawFamily(selector, rest string) ([]Command, bool, error) {
	return convertFamilyRoute(selector, rest, nlriDel)
}

// convertFamilyRoute translates a route whose line states its own family.
//
// The announce and the withdraw differ in ONE token, the NLRI verb, so they are
// one function taking that verb. They were two until 2026-09-05, and the two
// had diverged: the announce read four attribute keywords and the withdraw read
// none, because the withdraw took `strings.Fields(routeStr)[0]` as the whole
// route and discarded everything after it.
func convertFamilyRoute(selector, rest, verb string) ([]Command, bool, error) {
	rest = strings.TrimSpace(rest)

	// RFC 4761 Section 3.2.2: ExaBGP's `vpls` route (bridge_vpls.go).
	if commands, matched, err := convertVPLSRoute(selector, rest, verb); matched {
		return commands, true, err
	}

	if match := bridgeSRPolicyRE.FindStringSubmatch(rest); match != nil {
		afi := strings.ToLower(match[1])
		if verb == nlriDel {
			command, err := convertWithdrawSRPolicy(selector, afi, match[2])
			if err != nil {
				return nil, true, err
			}
			return []Command{command}, true, nil
		}
		command, err := convertAnnounceSRPolicy(selector, afi, match[2])
		if err != nil {
			return nil, true, err
		}
		return []Command{command}, true, nil
	}

	match := bridgeFamilyRE.FindStringSubmatch(rest)
	if match == nil {
		return nil, false, nil
	}

	afi := strings.ToLower(match[1])
	safi := canonicalExabgpSAFI(strings.ToLower(match[2]))
	routeStr := match[3]

	var tb textbuf.Buffer
	fam := tb.Str(afi).Byte('/').Str(safi).String()
	if safi == bridgeFlowSAFI || safi == bridgeFlowVPNSAFI {
		command, err := convertFlowSpec(selector, fam, routeStr, verb)
		if err != nil {
			return nil, true, err
		}
		return []Command{command}, true, nil
	}

	command, err := buildRouteCommand(selector, fam, verb, routeStr)
	return command, true, err
}

// convertAnnounce translates `announce route <prefix> ...`, the ExaBGP spelling
// that states no family and takes it from the prefix.
func convertAnnounce(selector, routeStr string) ([]Command, error) {
	return buildRouteCommand(selector, familyOfRoute(routeStr), nlriAdd, routeStr)
}

// convertWithdraw translates `withdraw route <prefix> ...`.
func convertWithdraw(selector, routeStr string) ([]Command, error) {
	return buildRouteCommand(selector, familyOfRoute(routeStr), nlriDel, routeStr)
}

// familyOfRoute reads the family out of the prefix a family-less ExaBGP route
// begins with. A colon in the prefix is IPv6; everything else is IPv4, which is
// also the answer for an empty body, where ze names the family in its refusal.
func familyOfRoute(routeStr string) string {
	parts := strings.Fields(strings.TrimSpace(routeStr))
	afi := bridgeAFIv4
	if len(parts) > 0 && strings.Contains(parts[0], ":") {
		afi = bridgeAFIv6
	}

	// The SAFI is read off the attributes, exactly as familyForAttributes does
	// for the `announce attributes` form: a route distinguisher makes it a VPN
	// route and a bare label makes it a labeled one.
	//
	// Reading the prefix alone sent every `announce route <p> rd <rd> label <l>`
	// as ipv4/unicast, a family the peer had not negotiated, so the UPDATE was
	// dropped with no error and the bridge acked the script. api-vpnv4 is that
	// line (ai/rules/principles.md).
	_, attrTokens := splitRouteBody(parts)
	attrs, err := parseRouteAttributes(attrTokens)
	if err != nil {
		return safiFamily(afi, bridgeUnicastSAFI)
	}
	switch {
	case attrs.Has("rd"):
		return safiFamily(afi, bridgeVPNSAFI)
	case attrs.Has(bridgeAttrLabel):
		return safiFamily(afi, bridgeLabeledSAFI)
	}
	return safiFamily(afi, bridgeUnicastSAFI)
}

// convertAnnounceSRPolicy translates ExaBGP SR-Policy announce to Ze's update text format.
//
// ExaBGP: announce ipv4 sr-policy distinguisher 0 color 100 endpoint 10.0.0.1 next-hop 1.2.3.4 preference 100 ...
// Ze:     send bgp <ip> update text nhop 1.2.3.4 mp ipv4/sr-policy add distinguisher 0 color 100 endpoint 10.0.0.1 preference 100 ...
//
// Extracts next-hop and the three NLRI fields (distinguisher, color, endpoint),
// lifts the communities ExaBGP accepts among the sub-TLVs to the attributes
// that precede `mp`, then appends all remaining tunnel-encap tokens verbatim.
//
// It reports an extended-community item ExaBGP refuses
// (CheckExtendedCommunityList), so the route is not sent.
func convertAnnounceSRPolicy(selector, afi, rest string) (Command, error) {
	rest = strings.TrimSpace(rest)
	parts := strings.Fields(rest)

	var nhop, distinguisher, color, endpoint string
	var attrs, extra []string
	for i := 0; i < len(parts); i++ {
		key := strings.ToLower(parts[i])
		switch key {
		// RFC 9830 Section 4.2.1: "The SR Policy update MUST have either the
		// NO_ADVERTISE community, at least one Route Target extended community
		// in IPv4-address format, or both." ExaBGP reads them anywhere among
		// the sub-TLVs; update text refuses an attribute after an nlri section.
		case bridgeAttrCommunity, bridgeAttrLargeCommunity, bridgeAttrExtCommunity:
			value, next := collectBridgeAttrValue(parts, i+1)
			if key == bridgeAttrExtCommunity {
				if err := CheckExtendedCommunityList(value); err != nil {
					return Command{}, err
				}
			}
			attrs = append(attrs, key+" "+value)
			i = next - 1
		case bridgeAttrNextHop, srPolicyDistinguisher, srPolicyColor, srPolicyEndpoint:
			if i+1 >= len(parts) {
				break
			}
			switch key {
			case bridgeAttrNextHop:
				nhop = parts[i+1]
			case srPolicyDistinguisher:
				distinguisher = parts[i+1]
			case srPolicyColor:
				color = parts[i+1]
			case srPolicyEndpoint:
				endpoint = parts[i+1]
			}
			i++
		default:
			extra = append(extra, parts[i])
		}
	}

	var tb textbuf.Buffer
	tb.Str("send bgp ").Str(selector).Str(" update text")
	if nhop != "" {
		tb.Str(" nhop ").Str(nhop)
	}
	for _, attr := range attrs {
		tb.Byte(' ').Str(attr)
	}
	// RFC 9830: SR Policy NLRI travels in MP_REACH_NLRI, ze's `mp` section.
	tb.Str(" mp ").Str(afi).Str("/sr-policy add")
	tb.Byte(' ').Str(srPolicyDistinguisher).Byte(' ').Str(distinguisher)
	tb.Byte(' ').Str(srPolicyColor).Byte(' ').Str(color)
	tb.Byte(' ').Str(srPolicyEndpoint).Byte(' ').Str(endpoint)
	for _, tok := range extra {
		tb.Str(" ").Str(tok)
	}
	// The three NLRI fields are the policy's identity, so the key names them
	// and not the tunnel-encap tail that follows.
	var withdrawal textbuf.Buffer
	withdrawal.Str("send bgp ").Str(selector).Str(" update text mp ").Str(afi).Str("/sr-policy del ").
		Str(srPolicyDistinguisher).Byte(' ').Str(distinguisher).
		Byte(' ').Str(srPolicyColor).Byte(' ').Str(color).
		Byte(' ').Str(srPolicyEndpoint).Byte(' ').Str(endpoint)
	return Command{
		Text:       tb.String(),
		Key:        srPolicyKey(afi, distinguisher, color, endpoint, false),
		Withdrawal: withdrawal.String(),
	}, nil
}

// srPolicyKey names one SR-Policy candidate path, stated where the command is
// written.
func srPolicyKey(afi, distinguisher, color, endpoint string, withdraw bool) RouteKey {
	var fam textbuf.Buffer
	var nlri textbuf.Buffer
	return RouteKey{
		Family: fam.Str(afi).Str("/sr-policy").String(),
		NLRI: nlri.Str(srPolicyDistinguisher).Byte(' ').Str(distinguisher).
			Byte(' ').Str(srPolicyColor).Byte(' ').Str(color).
			Byte(' ').Str(srPolicyEndpoint).Byte(' ').Str(endpoint).String(),
		Withdraw: withdraw,
	}
}

// convertWithdrawSRPolicy translates ExaBGP SR-Policy withdraw to Ze's update text format.
//
// ExaBGP parses the attributes of a withdrawal as it parses an announce
// (sr_policy.py `_values`), so an extended-community item ExaBGP refuses is
// reported here too (CheckExtendedCommunityList).
func convertWithdrawSRPolicy(selector, afi, rest string) (Command, error) {
	rest = strings.TrimSpace(rest)

	var tb textbuf.Buffer
	tb.Str("send bgp ").Str(selector).Str(" update text mp ").Str(afi).Str("/sr-policy del ").Str(rest)

	// The withdraw form writes the policy fields through unchanged, so the key
	// is read off the same three keywords convertAnnounceSRPolicy writes. A
	// field the line does not state leaves the key without it, and a key naming
	// a different set of fields matches no announce, which leaves both on the
	// wire rather than canceling the wrong one.
	fields := strings.Fields(rest)
	var distinguisher, color, endpoint string
	for i := 0; i+1 < len(fields); i++ {
		switch strings.ToLower(fields[i]) {
		case bridgeAttrExtCommunity:
			value, next := collectBridgeAttrValue(fields, i+1)
			if err := CheckExtendedCommunityList(value); err != nil {
				return Command{}, err
			}
			i = next - 1
		case srPolicyDistinguisher:
			distinguisher = fields[i+1]
		case srPolicyColor:
			color = fields[i+1]
		case srPolicyEndpoint:
			endpoint = fields[i+1]
		}
	}
	return Command{Text: tb.String(), Key: srPolicyKey(afi, distinguisher, color, endpoint, true)}, nil
}

// The three keywords that name an SR-Policy candidate path. One declaration
// serves the announce form, the withdraw form and the key both write.
const (
	srPolicyDistinguisher = "distinguisher"
	srPolicyColor         = "color"
	srPolicyEndpoint      = "endpoint"
)

// convertFlowSpec translates one ExaBGP `flow route { ... }` into the ze command
// that puts it on the wire, under the NLRI verb the caller names.
//
// The announce and the withdraw differ in that verb alone, so they are one
// function taking it -- the shape convertFamilyRoute already took, for the same
// reason. They were two, and the two had diverged: the withdraw discarded the
// attrs parseFlowSpecBridgeRoute reads out of the `then { ... }` block, so
// `withdraw flow route { match { ... } then { rate-limit 1; } }` reached ze with
// no rate-limit extended community and left one out of the withdrawal ExaBGP
// carries it on (api-broken-flow.ci).
//
// It reports an attribute ExaBGP itself refuses, so the route is not sent.
func convertFlowSpec(selector, family, routeStr, verb string) (Command, error) {
	fam, attrs, rd, nlri, err := parseFlowSpecBridgeRoute(family, routeStr)
	if err != nil {
		return Command{}, err
	}
	cmdParts := make([]string, 1, len(attrs)+2)
	cmdParts[0] = "send bgp " + selector + " update text"
	cmdParts = append(cmdParts, attrs...)

	var nlriPart textbuf.Buffer
	// RFC 8955 Section 4: FlowSpec NLRI travels in MP_REACH_NLRI, ze's `mp` section.
	nlriPart.Str("mp ").Str(fam).Byte(' ').Str(verb)
	if rd != "" {
		nlriPart.Str(" rd ").Str(rd)
	}
	if nlri != "" {
		nlriPart.Byte(' ').Str(nlri)
	}
	cmdParts = append(cmdParts, nlriPart.String())

	// The key names the rd and the match components, which is what the NLRI
	// part above writes. The attributes are left out: a withdrawal of one route
	// carries fewer of them than its announce did, and a key that read them
	// would never match (bridge_batch.go, Net).
	var keyNLRI textbuf.Buffer
	if rd != "" {
		keyNLRI.Str("rd ").Str(rd)
		if nlri != "" {
			keyNLRI.Byte(' ')
		}
	}
	keyNLRI.Str(nlri)
	command := Command{
		Text: textbuf.Join(cmdParts, " "),
		Key:  RouteKey{Family: fam, NLRI: keyNLRI.String(), Withdraw: verb == nlriDel},
	}
	if verb == nlriAdd {
		withdrawal, err := convertFlowSpec(selector, family, routeStr, nlriDel)
		if err != nil {
			return Command{}, err
		}
		command.Withdrawal = withdrawal.Text
	}
	return command, nil
}

// parseFlowSpecBridgeRoute splits a family-form flow route into its family, its
// update-text attributes, its route distinguisher and its NLRI text. It reports
// an extended-community item ExaBGP refuses (CheckExtendedCommunityList), and
// a route no packet can match (flowRouteMatchable).
func parseFlowSpecBridgeRoute(family, routeStr string) (string, []string, string, string, error) {
	parts := strings.Fields(strings.TrimSpace(routeStr))
	attrs := make([]string, 0, 4)
	nlri := make([]string, 0, len(parts))
	types := make([]flowspec.FlowComponentType, 0, len(parts))
	rd := ""
	fam := family
	currentComponent := ""
	inList := false

	for i := 0; i < len(parts); {
		key := strings.ToLower(parts[i])
		switch key {
		case bridgeAttrNextHop:
			if i+1 < len(parts) {
				attrs = append(attrs, "nhop "+parts[i+1])
				i += 2
			} else {
				i++
			}
		case bridgeAttrOrigin:
			if i+1 < len(parts) {
				attrs = append(attrs, "origin "+strings.ToLower(parts[i+1]))
				i += 2
			} else {
				i++
			}
		case bridgeAttrCommunity, bridgeAttrLargeCommunity, bridgeAttrExtCommunity:
			if i+1 < len(parts) {
				value, next := collectBridgeAttrValue(parts, i+1)
				if key == bridgeAttrExtCommunity {
					if err := CheckExtendedCommunityList(value); err != nil {
						return "", nil, "", "", err
					}
					value = normalizeFlowSpecExtCommunityValue(value)
				}
				attrs = append(attrs, key+" "+value)
				i = next
			} else {
				i++
			}
		case "rd":
			if i+1 < len(parts) {
				rd = parts[i+1]
				fam = flowSpecVPNFamily(fam)
				i += 2
			} else {
				i++
			}
		default:
			token := normalizeFlowSpecComponentToken(fam, parts[i])
			if componentType, ok := flowSpecComponentType(token); ok {
				types = append(types, componentType)
				currentComponent = strings.ToLower(token)
				nlri = append(nlri, token)
			} else {
				nlri = append(nlri, normalizeFlowSpecComponentValue(currentComponent, token))
				switch {
				case strings.Contains(parts[i], "[") && !strings.Contains(parts[i], "]"):
					inList = true
				case strings.Contains(parts[i], "]"):
					inList = false
					currentComponent = ""
				case !inList:
					currentComponent = ""
				}
			}
			i++
		}
	}

	// RFC 8955 Section 4.2
	if err := flowRouteMatchable(routeStr, types); err != nil {
		return "", nil, "", "", err
	}
	return fam, attrs, rd, textbuf.Join(nlri, " "), nil
}

// flowSpecComponentType answers the component type a token names, and false
// for a token that names no match component. The core vocabulary is the one
// statement of the keyword-to-type mapping (flowspec.KeywordComponentType).
//
// Its callers test the token AFTER normalizeFlowSpecComponentToken has run,
// so the bare aliases never reach it and the qualified spellings always do.
func flowSpecComponentType(token string) (flowspec.FlowComponentType, bool) {
	return flowspec.KeywordComponentType(strings.ToLower(token))
}

func normalizeFlowSpecComponentValue(component, token string) string {
	switch component {
	case "protocol", "next-header":
		return strings.TrimPrefix(token, "=")
	default:
		return token
	}
}

func flowSpecVPNFamily(family string) string {
	if strings.HasPrefix(family, "ipv6/") {
		return "ipv6/flow-vpn"
	}
	return "ipv4/flow-vpn"
}

func collectBridgeAttrValue(parts []string, start int) (string, int) {
	if start >= len(parts) {
		return "", start
	}
	if strings.Contains(parts[start], "[") && !strings.Contains(parts[start], "]") {
		valueParts := []string{parts[start]}
		i := start + 1
		for i < len(parts) {
			valueParts = append(valueParts, parts[i])
			if strings.Contains(parts[i], "]") {
				return textbuf.Join(valueParts, " "), i + 1
			}
			i++
		}
		return textbuf.Join(valueParts, " "), i
	}
	return parts[start], start + 1
}

func normalizeFlowSpecExtCommunityValue(value string) string {
	if value == "" {
		return value
	}
	if strings.HasPrefix(value, "[") && strings.HasSuffix(value, "]") {
		inner := strings.TrimSpace(value[1 : len(value)-1])
		if inner == "" {
			return value
		}
		fields := strings.Fields(inner)
		for i := range fields {
			fields[i] = normalizeFlowSpecExtCommunityToken(fields[i])
		}
		return "[" + textbuf.Join(fields, " ") + "]"
	}
	return normalizeFlowSpecExtCommunityToken(value)
}

func normalizeFlowSpecExtCommunityToken(value string) string {
	if rate, ok := strings.CutPrefix(value, "rate-limit-packets:"); ok {
		var tb textbuf.Buffer
		return tb.Str("rate-limit:").Str(rate).Str(":packets").String()
	}
	if strings.HasSuffix(value, ":bytes") && strings.HasPrefix(value, "rate-limit:") {
		return strings.TrimSuffix(value, ":bytes")
	}
	return value
}

// normalizeFlowSpecComponentToken rewrites ExaBGP's bare prefix keywords into
// the family-qualified ones ze speaks.
//
// ExaBGP accepts `source` and `destination` as aliases of the qualified
// spellings on input, and emits only the qualified ones
// (src/exabgp/configuration/announce/flow.py and
// src/exabgp/bgp/message/update/nlri/flow.py, both on 5.0 and main). An operator
// config written by hand may still use the alias, so the bare word arrives here
// and the route's family says which spelling it meant.
func normalizeFlowSpecComponentToken(family, token string) string {
	v6 := strings.HasPrefix(family, "ipv6/")
	switch strings.ToLower(token) {
	case "source":
		if v6 {
			return "source-ipv6"
		}
		return "source-ipv4"
	case "destination":
		if v6 {
			return "destination-ipv6"
		}
		return "destination-ipv4"
	}
	return token
}
