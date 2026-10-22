//go:build ze_bgp

// Design: docs/architecture/exabgp-bridge.md -- the lines a 4.2 or 5.0 script reads
// Related: old_commands_test.go -- the commands a 4.2 or 5.0 script writes
//
// X-35, AC-38. ExaBGP 6.0 recorded what 4.2 and 5.0 told a helper for each
// session state, FSM change, signal and message of the neighbors of
// qa/old/configs-<release>.json (qa/old/responses-<release>.json.gz at the pin,
// carried as is in testdata/exabgp-old). This judge gives each recorded event
// to ze the way the daemon meets it: the message bytes go through the engine's
// own formatter for a JSON subscriber (formatMessageForSubscription,
// internal/component/bgp/server/events.go: format.AppendMessage for an UPDATE,
// the JSONEncoder for the other messages, format.AppendStateChange for a
// state), and the ze document goes through the bridge exactly as a script's
// line does (ReadEvent, then Event.Line in the API 4 text and JSON formats).
// A JSON line is compared parsed, without its time, host, process ids and
// counter, and the `exabgp` field is reported once; a text line is compared
// as it is. This is the pin's qa/bin/test_old_responses, judge for judge.
//
// An event ze has no producer for (an FSM change, a signal, an OPERATIONAL
// message, the daemon's shutdown, the `packets` and `consolidate` renderings)
// is ze writing no line, and it is compared as that: a helper the release told
// something is told nothing.

package bridge

import (
	"compress/gzip"
	"encoding/hex"
	"encoding/json"
	"io"
	"log/slog"
	"net/netip"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/format"
	"github.com/ze-software/ze/internal/component/bgp/message"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// oldResponseRow exempts a difference: the releases it holds for, the encoder,
// a pattern on the event as the recorder named it, and a pattern on what
// differs (a JSON path, or the release's text line).
type oldResponseRow struct {
	releases string
	encoder  string
	event    *regexp.Regexp
	where    *regexp.Regexp
	reason   string
}

func oldResponseAllow(releases, encoder, event, where, reason string) oldResponseRow {
	return oldResponseRow{
		releases: releases, encoder: encoder,
		event: regexp.MustCompile(event), where: regexp.MustCompile(where), reason: reason,
	}
}

// oldResponsesAllowed is the pin's ALLOWED table of qa/bin/test_old_responses
// (ExaBGP 685f772e), reason and patterns as written there: each is a line a 4.2
// or 5.0 helper sees change on upgrade to ExaBGP 6.0, so it is the behavior ze
// owes (M3).
var oldResponsesAllowed = []oldResponseRow{
	oldResponseAllow("4.2 5.0", "json", `^open `, `/address-family-flags/`,
		"the Forwarding and Restart State of a Graceful Restart family had their names swapped (RFC 4724 3); this tree names them right, for API 4 too (CHANGELOG)"),
	oldResponseAllow("4.2 5.0", "json", `^update `, `/extended-community/\d+/string$`,
		"a route origin community with a two octet AS was printed as AS:IPv4, and 4.2 marked a four octet AS with an L; this tree prints the number, as configuration reads it (CHANGELOG)"),
	oldResponseAllow("4.2 5.0", "text", `^update `, `extended-community .*\b(origin|target):(\d+L:|\d+:\d+\.\d+\.\d+\.\d+)`,
		"the route origin community and the four octet AS, as in JSON (CHANGELOG)"),
	oldResponseAllow("4.2 5.0", "text", `^update `, `as-path \( `,
		"an AS_SET was written in ( ) and an AS_SEQUENCE in [ ], the other way round from the configuration; and a route with an AS_SET is treated as withdrawn (RFC 9774, CHANGELOG)"),
	oldResponseAllow("5.0", "text", `^operational `, `advisory "b'`,
		`5.0 wrote an advisory as the representation of Python bytes, "b'a message'"; this tree writes the message`),
	oldResponseAllow("4.2 5.0", "json", `^notification `, `^/neighbor/notification/message$`,
		`a Data field which is not printable ASCII is written in hexadecimal, "0xFFFF": 5.0 decoded it as UTF-8 with replacement characters, and a control character the peer chose reached the helper (CHANGELOG)`),
	oldResponseAllow("4.2 5.0", "json", `^update \S+ \S*800E[0-9A-F]{2}00020104`, `.`,
		"an IPv6 route with a four octet next hop and no extended next hop negotiated is malformed (RFC 2545 3, RFC 8950 3): the release took it, this tree treats it as an error"),
	oldResponseAllow("4.2 5.0", "text", `^update \S+ \S*800E[0-9A-F]{2}00020104`, `.`,
		"the IPv6 route with a four octet next hop, as in JSON"),
	oldResponseAllow("5.0", "text", `^update `, `bgp-prefix-sid \[ l[23]-service \[`,
		"5.0 wrote an SRv6 prefix SID one closing bracket short; this tree closes every one"),
	oldResponseAllow("4.2", "json", `^refresh packets `, `^/type$`,
		`4.2 wrote "route refresh", 5.0 "route-refresh"`),
	oldResponseAllow("4.2", "json", `^update `, `/attribute/as-path$`,
		"4.2 wrote the AS path as a list of numbers, 5.0 as its segments with their type"),
	oldResponseAllow("4.2", "json", `^update `, `/attribute/confederation-path$`,
		"4.2 wrote the confederation segments apart, as an empty list when there were none; 5.0 writes them in the AS path segments"),
	oldResponseAllow("4.2", "json", `^update `, `/attribute/attribute-0x19-0xC0$`,
		"4.2 did not know the IPv6 address specific extended community (RFC 5701), 5.0 does"),
	oldResponseAllow("4.2", "text", `^update `, `attribute \[ 0x19 0xC0 `,
		"the IPv6 extended community 4.2 did not know"),
	oldResponseAllow("4.2", "text", `^update `, `cluster-list \[ [^ ]+ \]`,
		"4.2 wrote a cluster list of one in brackets, 5.0 without"),
	oldResponseAllow("4.2", "json", `^update \S+ \S*800E[0-9A-F]{2}000285`, `/announce$`,
		"4.2 packed an IPv6 flow route a peer can not decode (test_old_commands counts them), and read its own bytes back: this tree finds no route in them"),
	oldResponseAllow("4.2", "text", `^update \S+ \S*800E[0-9A-F]{2}000285`, `announced flow `,
		"the IPv6 flow route of 4.2, as in JSON"),
	// End of the rows copied from the pin. Ze's own rows follow.
}

func (r *oldResponseRow) allows(release, encoder, event, where string) bool {
	if !slices.Contains(strings.Fields(r.releases), release) {
		return false
	}
	if r.encoder != encoder {
		return false
	}
	return r.event.MatchString(event) && r.where.MatchString(where)
}

func oldResponseAllowed(release, encoder, event, where string) bool {
	for i := range oldResponsesAllowed {
		if oldResponsesAllowed[i].allows(release, encoder, event, where) {
			return true
		}
	}
	return false
}

// oldResponseVolatile is what changes by nature from one run to the next.
var oldResponseVolatile = []string{"time", "host", "pid", "ppid", "counter"}

// oldShutdownPids is the text line of a shutdown, which names the daemon's
// process ids.
var oldShutdownPids = regexp.MustCompile(`^shutdown \d+ \d+`)

func TestOldResponses50(t *testing.T) { runOldResponses(t, "5.0") }

func TestOldResponses42(t *testing.T) { runOldResponses(t, "4.2") }

// runOldResponses judges every recorded event of one release.
//
// VALIDATES: AC-38, every line a 4.2 or 5.0 helper was written for an event
// it reads from ze's bridge as the release wrote it, or the difference is an
// ALLOWED row carrying its reason.
// PREVENTS: a 4.2 or 5.0 helper misreading, or never reading, an event after
// its move to ze.
func runOldResponses(t *testing.T, release string) {
	t.Helper()
	var recorded map[string]map[string]map[string]any
	loadOldResponseFile(t, "responses-"+release+".json.gz", &recorded)
	sessions := loadOldSessions(t, release, recorded)

	texts := make([]string, 0, len(recorded))
	for text := range recorded {
		texts = append(texts, text)
	}
	slices.Sort(texts)

	versions := map[string]bool{}
	classes := map[string]int{}
	var failures []string
	lines, broken := 0, 0
	for _, text := range texts {
		session, ok := sessions[text]
		if !ok {
			t.Fatalf("%s: no recorded session for a configuration of the responses file", release)
		}
		events := recorded[text]
		names := make([]string, 0, len(events))
		for name := range events {
			names = append(names, name)
		}
		slices.Sort(names)
		for _, event := range names {
			for _, encoder := range []string{"json", "text"} {
				theirs, present := events[event][encoder]
				if !present {
					continue
				}
				lines++
				ours := session.render(encoder, event)
				theirMap, theirIsMap := theirs.(map[string]any)
				ourMap, ourIsMap := ours.(map[string]any)
				if theirIsMap && ourIsMap {
					if version, has := theirMap["exabgp"]; has {
						versions[oldJSONText(version)+" -> "+oldJSONText(ourMap["exabgp"])] = true
						delete(theirMap, "exabgp")
						delete(ourMap, "exabgp")
					}
				}
				if oldResponseBroken(theirs) {
					broken++
					continue
				}
				for _, where := range oldResponseDifferences(ours, theirs, "") {
					if oldResponseAllowed(release, encoder, event, where) {
						continue
					}
					failures = append(failures, encoder+" "+oldShort(event)+" "+where+
						"\n    release "+oldShort(oldJSONText(theirs))+"\n    ze      "+oldShort(oldJSONText(ours)))
					classes[oldResponseClass(encoder, event, where, ours)]++
					break
				}
			}
		}
	}
	for version := range versions {
		t.Logf("%s: the \"exabgp\" field of a JSON line was %s", release, version)
	}
	t.Logf("%s: %d/%d helper lines as %s wrote them (%d %s could not write)",
		release, lines-len(failures), lines, release, broken, release)
	classNames := make([]string, 0, len(classes))
	for name := range classes {
		classNames = append(classNames, name)
	}
	slices.Sort(classNames)
	for _, name := range classNames {
		t.Logf("%s: class %5d  %s", release, classes[name], name)
	}
	for i, failure := range failures {
		if i == 40 && os.Getenv("ZE_OLD_VERBOSE") == "" {
			t.Logf("... %d more", len(failures)-i)
			break
		}
		t.Errorf("%s: %s", release, failure)
	}
}

// oldResponseClass groups a failure for the summary: the encoder, the event's
// kind and how it was rendered, and where the line differs, its numbers and
// addresses folded.
func oldResponseClass(encoder, event, where string, ours any) string {
	words := strings.Fields(event)
	kind := words[0]
	if len(words) > 1 && (words[1] == "packets" || words[1] == "consolidate") {
		kind += " " + words[1]
	}
	if ours == nil {
		return encoder + " " + kind + " (ze writes no line)"
	}
	if strings.HasPrefix(where, "{") {
		where = "(the whole line)"
	}
	if len(where) > 70 {
		where = where[:70]
	}
	return encoder + " " + kind + " " + oldDigits.ReplaceAllString(where, "N")
}

var oldDigits = regexp.MustCompile(`[0-9A-Fa-f]*\d[0-9A-Fa-f.:]*`)

func oldShort(text string) string {
	if len(text) > 400 && os.Getenv("ZE_OLD_VERBOSE") == "" {
		return text[:400] + "..."
	}
	return text
}

func oldJSONText(value any) string {
	encoded, err := json.Marshal(value)
	if err != nil {
		return "unencodable"
	}
	return string(encoded)
}

// oldResponseBroken reports a line the release could not write: its
// exception, or JSON no helper could parse (test_old_responses, broken).
func oldResponseBroken(theirs any) bool {
	if text, isText := theirs.(string); isText {
		return strings.HasPrefix(text, "raised ")
	}
	if found, isMap := theirs.(map[string]any); isMap {
		_, invalid := found["invalid-json"]
		return invalid
	}
	return false
}

// oldResponseDifferences answers where a helper reading what the release
// wrote finds something else in ze's line. A key ze adds is not read by a
// helper written before it, so only what the release wrote is held to. A JSON
// difference is named by its path, a text one by the release's first line ze
// changed (test_old_responses, differences).
//
// The recursion follows the recorded document, a bounded structure the
// release wrote.
func oldResponseDifferences(ours, theirs any, path string) []string {
	switch recorded := theirs.(type) {
	case map[string]any:
		if mine, isMap := ours.(map[string]any); isMap {
			var found []string
			for _, key := range sortedKeys(recorded) {
				value, has := mine[key]
				if !has {
					found = append(found, path+"/"+key)
					continue
				}
				found = append(found, oldResponseDifferences(value, recorded[key], path+"/"+key)...)
			}
			return found
		}
	case []any:
		if mine, isList := ours.([]any); isList && len(mine) == len(recorded) {
			var found []string
			for i := range recorded {
				found = append(found, oldResponseDifferences(mine[i], recorded[i], path+"/"+strconv.Itoa(i))...)
			}
			return found
		}
	case string:
		if mine, isText := ours.(string); isText && path == "" {
			return oldTextDifference(mine, recorded)
		}
	}
	if reflect.DeepEqual(ours, theirs) {
		return nil
	}
	if path == "" {
		return []string{oldJSONText(theirs)}
	}
	return []string{path}
}

func oldTextDifference(ours, theirs string) []string {
	mine := oldSplitLines(oldShutdownPids.ReplaceAllString(ours, "shutdown"))
	written := oldSplitLines(oldShutdownPids.ReplaceAllString(theirs, "shutdown"))
	for i, line := range written {
		if i >= len(mine) || mine[i] != line {
			return []string{line}
		}
	}
	if len(mine) == len(written) {
		return nil
	}
	return []string{"(more lines)"}
}

// oldSplitLines is Python's str.splitlines for "\n": no empty last line.
func oldSplitLines(text string) []string {
	lines := strings.Split(text, "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func loadOldResponseFile(t *testing.T, name string, into any) {
	t.Helper()
	file, err := os.Open(filepath.Join("testdata", "exabgp-old", name))
	if err != nil {
		t.Fatalf("open %s: %v", name, err)
	}
	defer func() { _ = file.Close() }()
	reader, err := gzip.NewReader(file)
	if err != nil {
		t.Fatalf("gunzip %s: %v", name, err)
	}
	data, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	if err := json.Unmarshal(data, into); err != nil {
		t.Fatalf("parse %s: %v", name, err)
	}
}

// oldSession is the session one configuration's events were recorded on: the
// first neighbor of the configuration, as the release parsed it, and the OPEN
// it sent, taken as received too (test_old_responses, shared).
type oldSession struct {
	peer       plugin.PeerInfo
	facts      SessionFacts
	negotiated format.DecodedNegotiated
	encoder    *format.JSONEncoder
}

// loadOldSessions reads configs-<release>.json.gz: each configuration's
// neighbors in the order the release listed them, named as it parsed them
// (`neighbor <peer> local-ip <local> local-as <n> peer-as <n> router-id <id>
// ...`), with the OPEN each one sent.
func loadOldSessions(t *testing.T, release string, recorded map[string]map[string]map[string]any) map[string]*oldSession {
	t.Helper()
	var configs map[string]struct {
		Neighbors json.RawMessage `json:"neighbors"`
	}
	loadOldResponseFile(t, "configs-"+release+".json.gz", &configs)
	sessions := make(map[string]*oldSession, len(recorded))
	for text := range recorded {
		config, ok := configs[text]
		if !ok {
			t.Fatalf("%s: a configuration of the responses file is not in the configs file", release)
		}
		name, open, err := oldFirstNeighbor(config.Neighbors)
		if err != nil {
			t.Fatalf("%s: neighbors of a configuration: %v", release, err)
		}
		session, err := newOldSession(name, open)
		if err != nil {
			t.Fatalf("%s: %s: %v", release, name, err)
		}
		sessions[text] = session
	}
	return sessions
}

// oldFirstNeighbor answers the first member of the neighbors object, in the
// file's order, which a Go map does not keep.
func oldFirstNeighbor(neighbors json.RawMessage) (string, string, error) {
	decoder := json.NewDecoder(strings.NewReader(string(neighbors)))
	if _, err := decoder.Token(); err != nil {
		return "", "", err
	}
	token, err := decoder.Token()
	if err != nil {
		return "", "", err
	}
	name, isName := token.(string)
	if !isName {
		return "", "", io.ErrUnexpectedEOF
	}
	var neighbor struct {
		Open string `json:"open"`
	}
	if err := decoder.Decode(&neighbor); err != nil {
		return "", "", err
	}
	return name, neighbor.Open, nil
}

func newOldSession(name, openHex string) (*oldSession, error) {
	words := strings.Fields(name)
	field := map[string]string{}
	for i := 0; i+1 < len(words); i += 2 {
		field[words[i]] = words[i+1]
	}
	peer, err := netip.ParseAddr(field["neighbor"])
	if err != nil {
		return nil, err
	}
	local, err := netip.ParseAddr(field["local-ip"])
	if err != nil {
		return nil, err
	}
	routerID, err := netip.ParseAddr(field["router-id"])
	if err != nil {
		return nil, err
	}
	localAS, err := strconv.ParseUint(field["local-as"], 10, 32)
	if err != nil {
		return nil, err
	}
	peerAS, err := strconv.ParseUint(field["peer-as"], 10, 32)
	if err != nil {
		return nil, err
	}
	raw, err := hex.DecodeString(openHex)
	if err != nil {
		return nil, err
	}
	open, err := message.UnpackOpen(raw[message.HeaderLen:])
	if err != nil {
		return nil, err
	}
	caps, err := capability.ParseFromOptionalParams(open.OptionalParams, open.ExtendedParams)
	if err != nil {
		return nil, err
	}
	id := routerID.As4()
	identity := capability.PeerIdentity{
		LocalASN: uint32(localAS), PeerASN: uint32(peerAS),
		LocalRouterID: uint32(id[0])<<24 | uint32(id[1])<<16 | uint32(id[2])<<8 | uint32(id[3]),
		PeerRouterID:  open.BGPIdentifier,
	}
	negotiated := capability.Negotiate(caps, caps, identity)
	session := &oldSession{
		peer: plugin.PeerInfo{
			Address: peer, LocalAddress: local,
			AddressStr: peer.String(), LocalAddressStr: local.String(),
			LocalAS: uint32(localAS), PeerAS: uint32(peerAS), RouterID: identity.LocalRouterID,
		},
		facts: SessionFacts{
			Local: local, Peer: peer, LocalAS: uint32(localAS), PeerAS: uint32(peerAS),
			RouterID: identity.LocalRouterID, Encoding: negotiated.Encoding,
		},
		negotiated: format.NegotiatedToDecoded(negotiated),
		encoder:    format.NewJSONEncoder(""),
	}
	return session, nil
}

var oldDiscard = slog.New(slog.DiscardHandler)

// render is ze's line for one event, as the recorder named it: parsed JSON
// without its volatile keys, a text line as it is, or nil when ze writes the
// script nothing.
func (s *oldSession) render(encoder, event string) (line any) {
	defer func() {
		if caught := recover(); caught != nil {
			line = "ze panicked: " + oldJSONText(caught)
		}
	}()
	document := s.zeDocument(strings.Fields(event))
	if document == "" {
		return nil
	}
	var zebgp map[string]any
	if err := json.Unmarshal([]byte(document), &zebgp); err != nil {
		return "the ze document does not parse: " + err.Error()
	}
	read := ReadEvent(zebgp)
	if encoder == "text" {
		written := read.Line(FormatV4Text, oldDiscard)
		if written == nil {
			return nil
		}
		return string(written)
	}
	written := read.Line(FormatV4JSON, oldDiscard)
	if written == nil {
		return nil
	}
	var found map[string]any
	if err := json.Unmarshal(written, &found); err != nil {
		return map[string]any{"invalid-json": string(written)}
	}
	for _, key := range oldResponseVolatile {
		delete(found, key)
	}
	return found
}

// zeDocument is the event ze's engine hands a JSON subscriber for the
// recorded event, or "" when ze has no such event.
func (s *oldSession) zeDocument(words []string) string {
	switch words[0] {
	case "up":
		return string(format.AppendStateChange(nil, &s.peer, rpc.SessionStateUp, "", nil, plugin.EncodingJSON))
	case "down":
		// ExaBGP's `peer reset` is the peer closing the TCP session, which ze
		// names tcp-failure (rpc.ReasonTCPFailure).
		return string(format.AppendStateChange(nil, &s.peer, rpc.SessionStateDown, rpc.ReasonTCPFailure, nil, plugin.EncodingJSON))
	case "negotiated":
		return s.encoder.Negotiated(&s.peer, s.negotiated)
	case "connected", "shutdown", "fsm", "signal":
		// Ze has no producer for these events.
		return ""
	}
	if words[1] == "packets" || words[1] == "consolidate" {
		// Ze has no rendering of a message as its packet.
		return ""
	}
	raw, err := hex.DecodeString(words[len(words)-1])
	if err != nil || len(raw) < message.HeaderLen {
		return ""
	}
	direction := rpc.DirectionReceived
	if words[1] == "send" {
		direction = rpc.DirectionSent
	}
	return s.messageDocument(msgtype.MessageType(raw[18]), raw[message.HeaderLen:], direction)
}

// messageDocument is formatMessageForSubscription for a JSON subscriber of
// the parsed format, the one the bridge's `*` subscription reads.
func (s *oldSession) messageDocument(kind msgtype.MessageType, body []byte, direction rpc.MessageDirection) string {
	switch kind {
	case msgtype.TypeUPDATE:
		if oldSessionReset(body, s.facts) {
			// The reactor resets the session rather than delivering the UPDATE.
			return ""
		}
		contextID, err := sessionContextID(s.facts)
		if err != nil {
			return ""
		}
		var wire wireu.WireUpdate
		wireu.InitWireUpdate(&wire, body, contextID)
		attrs, err := wire.Attrs()
		if err != nil {
			return ""
		}
		raw := bgptypes.RawMessage{
			Type: msgtype.TypeUPDATE, RawBytes: body, Direction: direction,
			WireUpdate: &wire, AttrsWire: attrs,
		}
		content := bgptypes.ContentConfig{Encoding: plugin.EncodingJSON, Format: plugin.FormatParsed}
		if direction == rpc.DirectionSent {
			return string(format.AppendSentMessage(nil, &s.peer, raw, content))
		}
		return string(format.AppendMessage(nil, &s.peer, raw, content))
	case msgtype.TypeOPEN:
		return s.encoder.Open(&s.peer, format.DecodeOpen(body), direction, 0)
	case msgtype.TypeNOTIFICATION:
		return s.encoder.Notification(&s.peer, format.DecodeNotification(body), direction, 0)
	case msgtype.TypeKEEPALIVE:
		return s.encoder.Keepalive(&s.peer, direction, 0)
	case msgtype.TypeROUTEREFRESH:
		return s.encoder.RouteRefresh(&s.peer, format.DecodeRouteRefresh(body), direction, 0)
	}
	// OPERATIONAL (type 6) and any other code: ze delivers no event for it.
	return ""
}
