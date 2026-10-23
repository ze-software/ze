// Design: docs/architecture/exabgp-bridge.md -- the neighbor lifecycle vocabulary
// RFC: rfc/short/rfc4271.md -- Section 4.5, the NOTIFICATION subcode is one octet
// RFC: rfc/short/rfc4486.md -- the Cease subcodes a teardown chooses from
// RFC: rfc/short/rfc9003.md -- the Shutdown Communication a teardown carries, and its 128-octet limit
// Overview: bridge_command.go -- the line translator that calls this converter
// Related: bridge_event.go -- the event encoder, which writes JSON and not text
//
// ExaBGP's API drives a neighbor as well as its routes: it creates one, deletes
// one, tears one down, and reports what one received. The four verbs land in
// three different places, and this file is where that is decided rather than
// guessed. Three are ze commands, and one is not a command at all.

package bridge

import (
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

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
// bridge_event.go declares zebgpToExabgpJSON and nothing else, and no code in
// the bridge reads the `encoder text` setting a script's config can carry.
var errNeighborReceive = errors.New("`receive` names an event ze sends up to the script, not a command a script sends down")

// The two NOTIFICATION values the teardown grammar names. The bridge is always
// linked and the BGP message package is gated by ze_bgp, so the values are
// stated here with their sources rather than imported.
const (
	// RFC 4271 Section 4.5, the Error Code table row: "6 Cease".
	teardownCodeCease uint8 = 6
	// RFC 4486 Section 4, the subcode table row: "2 Administrative Shutdown",
	// the pin's default
	// subcode (_TEARDOWN_DEFAULT, reactor/api/command/neighbor.py).
	teardownSubcodeAdminShutdown uint8 = 2
	// RFC 4486 Section 4, the subcode table row: "4 Administrative Reset".
	teardownSubcodeAdminReset uint8 = 4
	// The most octets of Shutdown Communication the pin sends
	// (Notification.SHUTDOWN_COMM_MAX_LEGACY, bgp/message/notification.py).
	// RFC 9003 Section 3 says: "Otherwise, a Shutdown Communication MAY be
	// sent, but it SHOULD NOT be longer than 128 octets." That is the limit.
	teardownCommunicationMax = 128
	// The most octets of Data the pin sends (Notify.DATA_MAX_OCTETS,
	// bgp/message/notification.py): RFC 4271 Section 4.1 says "The maximum
	// message size is 4096 octets.", less the 19-octet header and the two
	// code octets.
	teardownDataMax = 4096 - 19 - 2
)

// convertNeighborControl translates the ExaBGP neighbor lifecycle commands
// that are not routes. It reports false when the line is not one of them.
//
//	neighbor <ip> teardown [<code>] [<subcode>] [<text>]
//	                                  -> request peer <ip> notify <code> <subcode> [hex <octets>]
//	neighbor <ip> disable [<text>]    -> request peer <ip> disable [<text>]
//	neighbor <ip> enable              -> request peer <ip> enable
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
func convertNeighborControl(selector, rest string) (Translation, bool, error) {
	fields := strings.Fields(rest)
	if len(fields) == 0 {
		return Translation{}, false, nil
	}
	verb := strings.ToLower(fields[0])
	names := len(fields) > 1 && strings.EqualFold(fields[1], "neighbor")

	// enumeration: exempt (these are ExaBGP's own neighbor control words, not the CLI verb vocabulary)
	switch {
	case verb == "teardown":
		return convertTeardown(selector, rest)
	case verb == "disable":
		return convertDisable(selector, rest)
	case verb == "enable":
		return convertEnable(selector, fields)
	case verb == "create" && names:
		return convertNeighborCreate(fields[2:])
	case verb == "delete" && names:
		return convertNeighborDelete(fields[2:])
	case verb == "receive":
		return Translation{}, true, fmt.Errorf("%w: %q", errNeighborReceive, rest)
	}
	return Translation{}, false, nil
}

// convertTeardown writes the ze command that closes a session with the
// NOTIFICATION code, subcode and text the script chose.
//
// The grammar is the pinned ExaBGP's teardown_notification
// (reactor/api/command/neighbor.py): `teardown [<code>] [<subcode>] [<text>]`.
// The words after the verb are shlex-split; up to two leading all-digit words
// are numbers, each one octet; no number is Cease, Administrative Shutdown
// (6, 2), one is a Cease subcode, two are a code and a subcode; the remaining
// words joined with one space are the text. The refusals carry the pin's own
// texts, which a script may match.
//
// Every form goes to ze's `notify`, never its `teardown`: ze's teardown is
// Cease-only and writes the subcode's name when no text is given, so it would
// not send the pin's octets. The bridge builds the Data field the pin sends
// (teardownWireData, the pin's Notify._wire_text) and hands it to notify's hex
// form, which sends it octet for octet. The ExaBGP text therefore never passes
// through ze's command tokenizer (tokenize,
// internal/component/plugin/server/command.go), which would drop a `"` and
// refuse a backslash: what the script wrote is what the peer receives.
func convertTeardown(selector, rest string) (Translation, bool, error) {
	_, after, _ := strings.Cut(strings.TrimSpace(rest), " ")
	words, err := SplitCommandLine(after)
	if err != nil {
		return Translation{}, true, err
	}

	// RFC 4271 Section 4.5: "This 1-octet unsigned integer provides more
	// specific information about the nature of the reported error." Two numbers
	// at most, each one octet, as the pin reads them.
	numbers := make([]uint8, 0, 2)
	for len(words) > 0 && len(numbers) < 2 && allDigits(words[0]) {
		value, err := strconv.ParseUint(words[0], 10, 8)
		if err != nil {
			return Translation{}, true, fmt.Errorf("%s is not a value from 0 to 255", pyRepr(words[0]))
		}
		numbers = append(numbers, uint8(value))
		words = words[1:]
	}
	if len(numbers) == 0 && len(words) > 0 {
		return Translation{}, true, fmt.Errorf("expected a code or a subcode, got %s", pyRepr(words[0]))
	}

	code, subcode := teardownCodeCease, teardownSubcodeAdminShutdown
	switch len(numbers) {
	case 1:
		subcode = numbers[0]
	case 2:
		code, subcode = numbers[0], numbers[1]
	}

	// A wildcard selector reaches ze's own refusal rather than a second copy of
	// it here. ResolveSinglePeer (internal/component/plugin/server/command.go)
	// answers "notify requires one specific peer", which states the rule where
	// it is decided; restating it in the bridge would be a copy that can drift.
	// RFC 4271 Section 4.5, RFC 9003 Section 2
	data := teardownWireData(code, subcode, strings.Join(words, " "))
	var tb textbuf.Buffer
	tb.Str("request peer ").Str(selector).Str(" notify ").Uint8(code).Str(" ").Uint8(subcode)
	if len(data) > 0 {
		tb.Str(" hex ").Str(hex.EncodeToString(data))
	}
	return Translation{Commands: textCommands(tb.String()), Selector: selector}, true, nil
}

// teardownWireData is the Data field the pin's Notify._wire_text builds for
// code, subcode and text (bgp/message/notification.py), every case cut to
// the pin's DATA_MAX_OCTETS as Notify.__init__ cuts it.
//
// Under Cease, Administrative Shutdown or Administrative Reset the text is an
// RFC 9003 Shutdown Communication: its UTF-8 octets cut to 128, then back to
// the last whole character, behind a length octet; no text is an empty Data
// field. Under every other pair the text is `detail.encode('ascii',
// 'backslashreplace')`: an ASCII character as it is, and any other character
// as \xNN, \uNNNN or \UNNNNNNNN by its code point, lowercase hex.
//
// The pin reads a script's output as UTF-8, an octet that is not UTF-8 read as
// U+FFFD (2f909aa14, Processes._decoded), and so does the bridge
// (scriptLineText), so text holds whole characters only: a U+FFFD is its
// three UTF-8 octets in a Shutdown Communication and � under every other
// pair, as the pin sends it. backslashReplace still escapes a raw octet \xNN
// for a caller that hands it one.
func teardownWireData(code, subcode uint8, text string) []byte {
	var data []byte
	// RFC 9003 Section 2.
	if isTeardownCommunication(code, subcode) {
		data = teardownCommunication(text)
	} else {
		data = backslashReplace(text)
	}
	if len(data) > teardownDataMax {
		data = data[:teardownDataMax]
	}
	return data
}

// teardownCommunication is the RFC 9003 Shutdown Communication the pin sends
// for text: a length octet and at most 128 octets of whole UTF-8 characters,
// or nothing for an empty text.
//
// RFC 9003 Section 2: "the Shutdown Communication field MUST be encoded using
// UTF-8." RFC 9003 Section 3: "Otherwise, a Shutdown Communication MAY be
// sent, but it SHOULD NOT be longer than 128 octets."
func teardownCommunication(text string) []byte {
	if text == "" {
		return nil
	}
	communication := make([]byte, 1, 1+teardownCommunicationMax)
	for i := 0; i < len(text); {
		r, width := utf8.DecodeRuneInString(text[i:])
		i += width
		if r == utf8.RuneError && width == 1 {
			continue
		}
		if len(communication)-1+width > teardownCommunicationMax {
			break
		}
		communication = utf8.AppendRune(communication, r)
	}
	communication[0] = byte(len(communication) - 1)
	return communication
}

// backslashReplace is Python's text.encode('ascii', 'backslashreplace'): an
// ASCII character as itself, a code point below 0x100 as \xNN, below 0x10000
// as \uNNNN, and above as \UNNNNNNNN, the digits in lowercase. An octet that
// is not UTF-8 is written \xNN.
func backslashReplace(text string) []byte {
	out := make([]byte, 0, len(text))
	for i := 0; i < len(text); {
		r, width := utf8.DecodeRuneInString(text[i:])
		if r == utf8.RuneError && width == 1 {
			out = appendEscape(out, `\x`, uint32(text[i]), 2)
			i++
			continue
		}
		i += width
		if r < utf8.RuneSelf {
			out = append(out, byte(r))
			continue
		}
		if r < 0x100 {
			out = appendEscape(out, `\x`, uint32(r), 2)
			continue
		}
		if r < 0x10000 {
			out = appendEscape(out, `\u`, uint32(r), 4)
			continue
		}
		out = appendEscape(out, `\U`, uint32(r), 8)
	}
	return out
}

// appendEscape appends prefix and value as digits lowercase hex digits,
// zero-padded on the left.
func appendEscape(out []byte, prefix string, value uint32, digits int) []byte {
	const hexDigits = "0123456789abcdef"
	out = append(out, prefix...)
	for shift := (digits - 1) * 4; shift >= 0; shift -= 4 {
		out = append(out, hexDigits[(value>>uint(shift))&0xf])
	}
	return out
}

// isTeardownCommunication reports whether code and subcode are Cease,
// Administrative Shutdown or Administrative Reset, the pin's SHUTDOWN_SUBCODES:
// the pairs whose Data field is an RFC 9003 Shutdown Communication.
//
// RFC 9003 Section 2: "The Error Subcode value MUST be one of the following
// values: 2 ("Administrative Shutdown") or 4 ("Administrative Reset")."
func isTeardownCommunication(code, subcode uint8) bool {
	if code != teardownCodeCease {
		return false
	}
	if subcode == teardownSubcodeAdminShutdown {
		return true
	}
	return subcode == teardownSubcodeAdminReset
}

// allDigits reports whether word is one or more ASCII digits, the test the
// pin's str.isdigit makes for the words an ExaBGP script writes.
func allDigits(word string) bool {
	if word == "" {
		return false
	}
	for i := range len(word) {
		if word[i] < '0' || word[i] > '9' {
			return false
		}
	}
	return true
}

// pyRepr quotes word as Python's repr quotes a str, so a refusal reads as the
// pin's: single quotes, or double quotes when the word holds a single quote and
// no double one.
func pyRepr(word string) string {
	quote := "'"
	if strings.Contains(word, "'") && !strings.Contains(word, `"`) {
		quote = `"`
	}
	return quote + word + quote
}

// errEnableArgument is the answer for an enable that carries a word, which
// ExaBGP refuses with the same text (neighbor.py, enable).
var errEnableArgument = errors.New("enable takes no argument")

// errUnclosedQuote is Python shlex's answer for an unbalanced quote, which is
// what ExaBGP's disable answers with (neighbor.py, disable: shlex.split raises
// ValueError and the handler answers str(exc)).
var errUnclosedQuote = errors.New("No closing quotation") //nolint:staticcheck // ST1005: ExaBGP's exact text, which a script may match

// convertDisable writes the ze command that holds a peer administratively down.
//
// ExaBGP 6.0 (reactor/api/command/neighbor.py, disable) reads the words after
// the verb with shlex.split and joins them with one space; that text is the RFC
// 9003 Shutdown Communication of the Cease, Administrative Shutdown it sends.
// The words go to ze unquoted: ze's handler joins its tokens with one space,
// so the communication on the wire is the one ExaBGP would send.
func convertDisable(selector, rest string) (Translation, bool, error) {
	_, after, _ := strings.Cut(strings.TrimSpace(rest), " ")
	words, err := SplitCommandLine(after)
	if err != nil {
		return Translation{}, true, err
	}
	var tb textbuf.Buffer
	tb.Str("request peer ").Str(selector).Str(" disable")
	for _, word := range words {
		tb.Str(" ").Str(word)
	}
	return Translation{Commands: textCommands(tb.String()), Selector: selector}, true, nil
}

// convertEnable writes the ze command that releases an administrative hold.
// ExaBGP's enable takes no argument and refuses one.
func convertEnable(selector string, fields []string) (Translation, bool, error) {
	if len(fields) > 1 {
		return Translation{}, true, fmt.Errorf("%w, got %q", errEnableArgument, strings.Join(fields[1:], " "))
	}
	var tb textbuf.Buffer
	return Translation{
		Commands: textCommands(tb.Str("request peer ").Str(selector).Str(" enable").String()),
		Selector: selector,
	}, true, nil
}

// SplitCommandLine splits text as Python's shlex.split does in POSIX mode, for the
// forms an ExaBGP script writes: words separated by blanks, a single-quoted
// run taken as written, a double-quoted run where a backslash escapes `"` or
// `\` and before any other character stays a backslash, and a backslash
// outside quotes escaping the next character. An unclosed quote is refused
// with shlex's own text.
//
// Python's shlex (Lib/shlex.py, read_token, POSIX mode) writes the escape
// character itself when, inside quotes, the next character is neither the
// quote nor the escape character, so `"a\tb"` is the four characters a, \, t,
// b, which ExaBGP 6 then sends as its teardown text.
func SplitCommandLine(text string) ([]string, error) {
	var words []string
	var word strings.Builder
	inWord := false
	quote := rune(0)
	escaped := false
	quotedEscape := false
	for _, r := range text {
		if quotedEscape {
			if r != '"' && r != '\\' {
				word.WriteRune('\\')
			}
			word.WriteRune(r)
			quotedEscape = false
			continue
		}
		if escaped {
			word.WriteRune(r)
			escaped = false
			continue
		}
		if quote == '\'' {
			if r == '\'' {
				quote = 0
				continue
			}
			word.WriteRune(r)
			continue
		}
		if quote == '"' {
			if r == '"' {
				quote = 0
				continue
			}
			if r == '\\' {
				quotedEscape = true
				continue
			}
			word.WriteRune(r)
			continue
		}
		if r == ' ' || r == '\t' {
			if inWord {
				words = append(words, word.String())
				word.Reset()
				inWord = false
			}
			continue
		}
		inWord = true
		if r == '\'' || r == '"' {
			quote = r
			continue
		}
		if r == '\\' {
			escaped = true
			continue
		}
		word.WriteRune(r)
	}
	if quote != 0 {
		return nil, errUnclosedQuote
	}
	if escaped {
		return nil, errNoEscapedCharacter
	}
	if inWord {
		words = append(words, word.String())
	}
	return words, nil
}

// errNoEscapedCharacter is shlex's answer for a line ending in a backslash.
var errNoEscapedCharacter = errors.New("No escaped character") //nolint:staticcheck // ST1005: shlex's exact text

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

// PluginName is the registry name of the ze plugin that runs every ExaBGP
// script. A script is a child of that plugin and never a ze process of its own,
// so a peer serves a script by attaching this plugin.
const PluginName = "exabgp-bridge"

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
//	create bgp peer 127.0.0.1 asn 1 local-as 1 local-address 127.0.0.1 attach exabgp-bridge
//
// The peer ze builds lives in the reactor alone, which is what ExaBGP's own
// dynamic peer does: neither writes the configuration file.
func convertNeighborCreate(fields []string) (Translation, bool, error) {
	if len(fields) == 0 {
		return Translation{}, true, errNeighborCreateAddress
	}
	address := fields[0]

	values := make(map[string]string, len(fields)/2)
	attached := false

	for i := 1; i < len(fields); i++ {
		name := strings.ToLower(fields[i])
		if i+1 >= len(fields) {
			return Translation{}, true, fmt.Errorf("%w: %q takes a value", errNeighborParameter, name)
		}
		value := fields[i+1]
		i++

		// `api` is the one repeating parameter: ExaBGP writes the keyword once
		// per process. Every ExaBGP process is a script the bridge plugin
		// runs, so whatever the names, the peer attaches that one plugin.
		if name == "api" {
			attached = true
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

	// Attaching the plugin by the SCRIPT's name named a process ze does not
	// run, so the created peer attached nothing and refused every route the
	// script then sent it (api-peer-lifecycle).
	if attached {
		values[zeKeywordAttach] = PluginName
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
