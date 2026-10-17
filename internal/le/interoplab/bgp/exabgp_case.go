// Design: docs/architecture/testing/interop.md -- the mock ExaBGP speaker
// Related: exabgp_server.go -- serving the sessions a case names
// Related: exabgp_case_test.go -- the case reader's refusals and its sequence order
// RFC: rfc/short/rfc9003.md Section 2
//
// The case reader turns a `.ci` fixture into the script the mock walks. It
// reads the fixture the way upstream's own mock does (Checker.group_messages
// and the option loop of test/exabgp-compat/bin/bgp), and refuses before any
// session opens a fixture that could pass without asserting anything.

//go:build ze_bgp

package bgp

import (
	"bufio"
	"cmp"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// readExaBGPCase reads a `.ci` fixture: the script each connection owes, the AS
// this mock presents, and whether the fixture STATED that AS or took the default.
//
// The caller needs the third answer because the default is a guess. Upstream's
// own runner never opened a session at all, so no fixture states an AS unless
// its author had a reason to, and 65000 is right for none of them until the
// config happens to name it.
func readExaBGPCase(path string) (exabgpCase, error) {
	file, err := os.Open(path) //nolint:gosec // the case file is the .ci fixture this helper is pointed at by the tracked lab runner
	if err != nil {
		return exabgpCase{}, err
	}
	defer func() { _ = file.Close() }()
	spec := exabgpCase{steps: make(map[int][]exabgpStep), asn: 65000, messageTimeout: exabgpSessionLength}
	// The frame a `json:` line attaches to. A zero connection means no frame
	// has been read yet, which makes a leading `json:` line an error.
	var lastDocumentable struct{ connection, index int }
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "option=") {
			if err := readExaBGPOption(line, &spec); err != nil {
				return exabgpCase{}, fmt.Errorf("%s: %w", path, err)
			}
			continue
		}
		// Every other line is `<position>:<kind>:<content>`. A line of another
		// shape, or of a kind nobody reads, used to be skipped in silence, which
		// is how a mistyped expectation became no expectation.
		fields := strings.SplitN(line, ":", 3)
		if len(fields) < 3 {
			return exabgpCase{}, fmt.Errorf("line %q is not a <position>:<kind>:<content> step", line)
		}
		kind := fields[1]
		content := strings.TrimSpace(fields[2])
		// `cmd:` is the API line a `raw:` frame was generated from, and
		// `jsv4:`/`jsv6:` are documents for other ExaBGP versions. Upstream's
		// mock skips all three, because the frame is the expectation.
		if slices.Contains(exabgpDocumentationKinds, kind) {
			continue
		}
		// A `json:` line states the ExaBGP document the frame ABOVE it owes, so
		// it is attached to that frame rather than being a step of its own.
		// Until 2026-09-20 this loop dropped every one of them, which is why
		// two defects in ze's own ExaBGP rendering reached a verification sweep
		// unseen (plan/journal/gate-excludes-part-of-its-population.md).
		//
		// Its leading number is never read. Upstream's own reader (parse_ci_file,
		// qa/bin/test_json) attaches an expectation to "the immediately preceding
		// non-EOR raw UPDATE message", and conf-watchdog follows a `1:raw:` frame
		// with a `5:json:` line.
		if kind == "json" {
			if lastDocumentable.connection == 0 {
				return exabgpCase{}, fmt.Errorf("json expectation names no frame above it: %q", line)
			}
			spec.steps[lastDocumentable.connection][lastDocumentable.index].wantJSON = content
			continue
		}
		connection, sequence, err := exabgpCasePosition(fields[0])
		if err != nil {
			return exabgpCase{}, fmt.Errorf("invalid position in %q: %w", line, err)
		}
		step, err := readExaBGPStep(kind, content)
		if err != nil {
			return exabgpCase{}, fmt.Errorf("%q: %w", line, err)
		}
		step.sequence = sequence
		spec.steps[connection] = append(spec.steps[connection], step)
		// An EOR and a KEEPALIVE carry no document, so a `json:` line below one
		// belongs to the UPDATE before it. Upstream skips both when it decides
		// which frame an expectation attaches to.
		if step.kind == exabgpStepFrame && documentableFrame(step.frame) {
			lastDocumentable.connection = connection
			lastDocumentable.index = len(spec.steps[connection]) - 1
		}
	}
	if err := scanner.Err(); err != nil {
		return exabgpCase{}, err
	}
	if err := orderExaBGPCase(&spec); err != nil {
		return exabgpCase{}, fmt.Errorf("%s: %w", path, err)
	}
	return spec, nil
}

// exabgpDocumentationKinds are the step kinds the mock skips: `cmd:` is the
// API line a `raw:` frame was generated from, and `jsv4:`/`jsv6:` are
// documents for other ExaBGP versions.
var exabgpDocumentationKinds = []string{"cmd", "jsv4", "jsv6"}

// readExaBGPOption reads one `option=` line. Every option a fixture may carry is
// named here, either as one this mock acts on or as one the runner reads, and
// any other is refused: an option skipped in silence is a fixture that runs as
// if it said nothing, which is how conf-unknowncap passed without the mock ever
// sending the capability its name promises.
func readExaBGPOption(line string, spec *exabgpCase) error {
	option := strings.TrimPrefix(line, "option=")
	// The runner reads these (exabgpCLI in internal/test/cli/cmd_exabgp.go):
	// the configs ze runs, how many sessions ze opens, and whether the case
	// runs alone.
	if strings.HasPrefix(option, "file:") {
		return nil
	}
	if strings.HasPrefix(option, "tcp_connections:") {
		return nil
	}
	if option == "serial" {
		return nil
	}
	if value, ok := strings.CutPrefix(option, "asn:"); ok {
		parsed, err := strconv.ParseUint(value, 10, 32)
		if err != nil {
			return fmt.Errorf("option=asn: %q is not an AS number: %w", value, err)
		}
		spec.asn = uint32(parsed)
		spec.asnStated = true
		return nil
	}
	// `option=update:` names an UPDATE this mock sends to the speaker once the
	// session is up. It was read as nothing until 2026-09-06, so a case whose
	// script waits for that route waited for ever.
	if value, ok := strings.CutPrefix(option, "update:"); ok {
		if value != "send-default-route" {
			return fmt.Errorf("unknown option=update: %q: the one UPDATE this mock sends is send-default-route", value)
		}
		spec.sendDefaultRoute = true
		return nil
	}
	// `option=timeout:<seconds>` is upstream's per-message limit: the mock
	// fails a session when the speaker sends nothing for that long. The session
	// deadline still bounds the whole session, so a longer limit would be cut
	// short in silence and is refused.
	if value, ok := strings.CutPrefix(option, "timeout:"); ok {
		seconds, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return fmt.Errorf("option=timeout: %q is not a number of seconds: %w", value, err)
		}
		timeout := time.Duration(seconds * float64(time.Second))
		if timeout <= 0 {
			return fmt.Errorf("option=timeout: %q is not a positive number of seconds", value)
		}
		if timeout > exabgpSessionLength {
			return fmt.Errorf("option=timeout: %q is longer than the %s the mock gives a whole session: state at most %s",
				value, exabgpSessionLength, exabgpSessionLength)
		}
		spec.messageTimeout = timeout
		return nil
	}
	// `option=frames:none` states that the case asserts no frame from the
	// speaker. Without it such a case is refused, because it would pass
	// whatever the speaker sent.
	if option == "frames:none" {
		spec.framesNoneStated = true
		return nil
	}
	if option == "open:send-unknown-capability" {
		spec.sendUnknownCapability = true
		return nil
	}
	if option == "open:inspect-open-message" {
		spec.inspectOpen = true
		return nil
	}
	if option == "bind:ipv6" {
		spec.bindIPv6 = true
		return nil
	}
	return fmt.Errorf("option %q is read by neither this mock nor the runner, so the case would run as if it were absent: "+
		"remove it, or teach readExaBGPOption (internal/le/interoplab/bgp/exabgp_case.go) what it means", line)
}

// readExaBGPStep reads the content of one `raw:`, `signal:` or `notification:`
// step.
func readExaBGPStep(kind, content string) (exabgpStep, error) {
	if kind == "raw" {
		// Upstream's fixtures write a frame both with colons between its header
		// fields and without them, and its mock strips them. Until 2026-10-17
		// this reader skipped a frame written without them, so api-sr-policy's
		// announce and its End-of-RIB were never awaited.
		wire, err := hex.DecodeString(strings.ReplaceAll(strings.ReplaceAll(content, ":", ""), " ", ""))
		if err != nil {
			return exabgpStep{}, fmt.Errorf("decode raw frame: %w", err)
		}
		if len(wire) < bgpHeaderLength {
			return exabgpStep{}, fmt.Errorf("raw frame is %d octets, shorter than a BGP header", len(wire))
		}
		return exabgpStep{kind: exabgpStepFrame, frame: wire}, nil
	}
	// `signal:` names a point in the script where the RUNNER must signal the
	// speaker. It was read as nothing until 2026-09-06, so api-reload's reload
	// never happened and its post-reload frames never arrived.
	if kind == "signal" {
		if content == "" {
			return exabgpStep{}, errors.New("signal step names no signal")
		}
		return exabgpStep{kind: exabgpStepSignal, signal: content}, nil
	}
	// `notification:` makes this mock close the session with a Shutdown
	// Communication carrying the text, as upstream's mock does. It was read as
	// nothing until 2026-10-17, so api-notification's session was never closed.
	if kind == "notification" {
		return readExaBGPNotification(content)
	}
	return exabgpStep{}, fmt.Errorf("unknown step kind %q: a step is raw, json, signal, notification, cmd, jsv4 or jsv6", kind)
}

// readExaBGPNotification reads the text a notification step sends, the way
// upstream's mock reads it: the content after its last colon
// (_send_notification_if_requested splits on ':'), lowercased, because
// group_messages lowercases every step that is not a raw frame.
func readExaBGPNotification(content string) (exabgpStep, error) {
	text := content[strings.LastIndex(content, ":")+1:]
	// RFC 9003 Section 2: "To support international characters, the Shutdown
	// Communication field MUST be encoded using UTF-8."
	if !utf8.ValidString(text) {
		return exabgpStep{}, fmt.Errorf("shutdown communication %q is not valid UTF-8, which RFC 9003 Section 2 requires: write the text in UTF-8", text)
	}
	text = strings.ToLower(text)
	if len(text) > exabgpShutdownCommunicationOctetsMax {
		return exabgpStep{}, fmt.Errorf("shutdown communication is %d octets, the limit is %d",
			len(text), exabgpShutdownCommunicationOctetsMax)
	}
	return exabgpStep{kind: exabgpStepNotification, notification: text}, nil
}

// orderExaBGPCase puts each session's steps in sequence order and refuses a
// script that could pass without asserting anything.
//
// The sort is stable, so steps that share a sequence number keep the order the
// fixture wrote them in, and they form one group whose frames match in any
// order. A group holding a signal or a notification holds nothing else, because
// an action has no defined place among frames that may arrive in any order.
func orderExaBGPCase(spec *exabgpCase) error {
	spec.connections = len(spec.steps)
	if spec.connections == 0 {
		return errors.New("the script has no step, so the case would pass without asserting anything: " +
			"add the `<n>:raw:` frames the speaker owes")
	}
	frames := 0
	for connection := 1; connection <= spec.connections; connection++ {
		steps, ok := spec.steps[connection]
		if !ok {
			return exabgpSessionGap(spec.steps, connection)
		}
		slices.SortStableFunc(steps, func(left, right exabgpStep) int { return cmp.Compare(left.sequence, right.sequence) })
		if err := checkExaBGPOpenSteps(steps, connection, spec.inspectOpen); err != nil {
			return err
		}
		for index, step := range steps {
			if step.kind == exabgpStepFrame {
				frames++
				continue
			}
			if index > 0 && steps[index-1].sequence == step.sequence {
				return fmt.Errorf("session %s sequence %d holds an action beside other steps: give the action a sequence of its own",
					exabgpSessionLetter(connection), step.sequence)
			}
			if index+1 < len(steps) && steps[index+1].sequence == step.sequence {
				return fmt.Errorf("session %s sequence %d holds an action beside other steps: give the action a sequence of its own",
					exabgpSessionLetter(connection), step.sequence)
			}
		}
	}
	if frames == 0 && !spec.framesNoneStated {
		return errors.New("the script expects no frame from the speaker, so the case would pass without asserting anything: " +
			"add the `<n>:raw:` frames it owes, or state `option=frames:none` with a comment saying why the case asserts no frame")
	}
	if frames > 0 && spec.framesNoneStated {
		return errors.New("the script states `option=frames:none` and also expects frames: remove the option")
	}
	return nil
}

// checkExaBGPOpenSteps refuses an OPEN expectation the mock would never check.
// Upstream's mock compares the speaker's OPEN only under
// `option=open:inspect-open-message`, and then as the first message of the
// session, so an OPEN step without the option, or after another step, is one
// no run can match, and the option without an OPEN step asserts nothing.
func checkExaBGPOpenSteps(steps []exabgpStep, connection int, inspectOpen bool) error {
	session := exabgpSessionLetter(connection)
	for index, step := range steps {
		if !exabgpOpenStep(step) {
			continue
		}
		if !inspectOpen {
			return fmt.Errorf("session %s sequence %d expects the speaker's OPEN, which the mock checks only under "+
				"`option=open:inspect-open-message`: add the option", session, step.sequence)
		}
		if index > 0 {
			return fmt.Errorf("session %s sequence %d expects the speaker's OPEN after other steps, but the OPEN is the first "+
				"message of a session: move the OPEN step first or give it a sequence of its own", session, step.sequence)
		}
	}
	if inspectOpen && !exabgpOpenStep(steps[0]) {
		return fmt.Errorf("the script states `option=open:inspect-open-message` but session %s does not open with an OPEN "+
			"frame: add the speaker's OPEN as its first `raw:` step, or remove the option", session)
	}
	return nil
}

// exabgpOpenStep answers whether a step is a frame expectation for an OPEN.
// readExaBGPStep refused a frame shorter than a BGP header, so its type octet
// is there.
func exabgpOpenStep(step exabgpStep) bool {
	return step.kind == exabgpStepFrame && step.frame[18] == bgpOpen
}

// exabgpSessionGap is the refusal for a script whose session letters skip one:
// the mock accepts sessions in order, so a missing session would assert
// nothing and every session after it would be matched against the wrong wire.
func exabgpSessionGap(steps map[int][]exabgpStep, missing int) error {
	present := make([]int, 0, len(steps))
	for connection := range steps {
		present = append(present, connection)
	}
	slices.Sort(present)
	letters := make([]string, 0, len(present))
	rename := 0
	for _, connection := range present {
		letters = append(letters, exabgpSessionLetter(connection))
		if rename == 0 && connection > missing {
			rename = connection
		}
	}
	listed := letters[len(letters)-1]
	noun := "session"
	if len(letters) > 1 {
		listed = strings.Join(letters[:len(letters)-1], ", ") + " and " + listed
		noun = "sessions"
	}
	want := exabgpSessionLetter(missing)
	return fmt.Errorf("the script has steps for %s %s but none for %s: rename %s to %s, or add %s's steps",
		noun, listed, want, exabgpSessionLetter(rename), want, want)
}

// exabgpSessionLetter is the letter a fixture names a session by: 1 is A.
// exabgpCasePosition reads only letters, so a session is at most Z.
func exabgpSessionLetter(connection int) string {
	return string(rune('A' + connection - 1))
}

// documentableFrame answers whether a frame is one an ExaBGP JSON document is
// written for: an UPDATE that is not an End-of-RIB marker.
//
// RFC 4271 Section 4.1 puts the type octet at offset 18, after the 16-octet
// marker and the 2-octet length.
func documentableFrame(wire []byte) bool {
	const (
		headerLength = 19
		typeOffset   = 18
		typeUpdate   = 2
	)
	if len(wire) <= typeOffset {
		return false
	}
	if wire[typeOffset] != typeUpdate {
		return false
	}
	return !exabgpEndOfRIB(wire[headerLength:])
}

// exabgpEndOfRIB answers whether an UPDATE body is an End-of-RIB marker, in
// either of its two forms.
//
// RFC 4724 Section 2: "For the IPv4 unicast address family, the End-of-RIB
// marker is an UPDATE message with the minimum length [BGP-4].  For any other
// address family, it is an UPDATE message that contains only the
// MP_UNREACH_NLRI attribute [BGP-MP] with no withdrawn routes for that
// <AFI, SAFI>."
//
//	+-----------------------------+
//	| Withdrawn Routes Length (2) |  octets 0-1, zero
//	+-----------------------------+
//	| Total Path Attr Length (2)  |  octets 2-3, zero or the one attribute
//	+-----------------------------+
//	| MP_UNREACH_NLRI (type 15)   |  flags, type, length (1 or 2 octets),
//	|   AFI (2), SAFI (1)         |  then 3 octets and no withdrawn route
//	+-----------------------------+
//
// Upstream's is_eor takes any UPDATE whose body is 4 or 11 octets for an
// End-of-RIB, by length alone, which admits the MP form only when its
// attribute length is written in two octets; this reads both encodings.
func exabgpEndOfRIB(body []byte) bool {
	const (
		flagExtendedLength = 0x10
		typeMPUnreach      = 15
		afiSAFIOctets      = 3
	)
	if len(body) < 4 {
		return false
	}
	if binary.BigEndian.Uint16(body[0:2]) != 0 {
		return false
	}
	attributes := body[4:]
	if int(binary.BigEndian.Uint16(body[2:4])) != len(attributes) {
		return false
	}
	if len(attributes) == 0 {
		return true
	}
	if len(attributes) < 3 {
		return false
	}
	if attributes[1] != typeMPUnreach {
		return false
	}
	valueOffset, length := 3, int(attributes[2])
	if attributes[0]&flagExtendedLength != 0 {
		if len(attributes) < 4 {
			return false
		}
		valueOffset, length = 4, int(binary.BigEndian.Uint16(attributes[2:4]))
	}
	return length == afiSAFIOctets && len(attributes) == valueOffset+length
}

// exabgpStepKind says what one entry of a connection's script is. Zero is
// Unspecified, so a step nobody wrote is never read as a frame the speaker owes.
type exabgpStepKind uint8

const (
	exabgpStepUnspecified exabgpStepKind = iota
	exabgpStepFrame
	exabgpStepSignal
	exabgpStepNotification
)

// exabgpShutdownCommunicationOctetsMax is the longest text a notification step
// may carry: RFC 9003 Section 2 gives its length a single octet.
const exabgpShutdownCommunicationOctetsMax = 255

// exabgpStep is one entry of a connection's script, in sequence order.
//
// A frame step is an expectation the speaker owes. A signal step is an
// instruction to the RUNNER, which owns the ze process this mock cannot reach,
// and a notification step makes this mock close the session. Each action has a
// sequence of its own, so every frame of the groups before it has matched when
// it runs, and the frames after it are matched only once it has.
type exabgpStep struct {
	kind exabgpStepKind
	// sequence is the number the fixture's prefix carries. Steps that share it
	// form one group, whose frames match in any order.
	sequence     int
	frame        []byte
	signal       string
	notification string
	// wantJSON is the ExaBGP document this frame owes, as the fixture's `json:`
	// line states it. Empty when the fixture states none, which is every frame
	// that is not an UPDATE and most that are.
	wantJSON string
}

// exabgpCase is what one `.ci` fixture states: the script each connection owes,
// the AS this mock presents and whether the fixture named it, and the UPDATEs
// the mock sends of its own accord.
type exabgpCase struct {
	steps map[int][]exabgpStep
	// connections is the number of sessions the script names, which is how
	// many this mock accepts. orderExaBGPCase sets it and checks the sessions
	// run from A with none missing.
	connections int
	// framesNoneStated is `option=frames:none`: the fixture states it asserts
	// no frame from the speaker, which is refused unless stated.
	framesNoneStated bool
	asn              uint32
	// asnStated says the fixture named the AS. The caller needs it because the
	// default is a guess: upstream's runner opened no session at all, so no
	// fixture states an AS unless its author had a reason to.
	asnStated bool
	// sendDefaultRoute is `option=update:send-default-route`, which makes this
	// mock announce 0.0.0.0/32 once the session is up.
	sendDefaultRoute bool
	// sendUnknownCapability is `option=open:send-unknown-capability`: the
	// mock's OPEN carries capability 66, which ze does not implement.
	sendUnknownCapability bool
	// inspectOpen is `option=open:inspect-open-message`: each session's first
	// step is the speaker's OPEN, compared byte for byte.
	inspectOpen bool
	// bindIPv6 is `option=bind:ipv6`: the mock listens on IPv6 alone, so
	// the case passes only when ze reaches it over IPv6.
	bindIPv6 bool
	// messageTimeout is how long the mock waits for the speaker's next
	// message: `option=timeout:`, or the whole session's length when the
	// fixture states none.
	messageTimeout time.Duration
}

// exabgpCasePosition reads where a `.ci` step sits: the session it belongs to,
// and its sequence number within that session.
//
// The prefix comes in two shapes and both are upstream's, read the way
// upstream's mock reads them (Checker.group_messages, test/exabgp-compat/bin/bgp).
// A LETTER names the session and the digits after it are the sequence, so
// `A1:raw:` and `A2:raw:` are the first and second groups of session A and
// `B1:raw:` opens session B. A bare NUMBER is a sequence of the FIRST session,
// so `1:raw:` and `2:raw:` are two ordered groups on one wire.
//
// The number was read as a session until 2026-10-17. A case whose groups ran
// `1:` to `5:` then owed five sessions, the mock served one, and the frames of
// groups 2 to 5 were never awaited: api-rib, numbered from `10:`, asserted
// nothing at all. Zero is a sequence like any other, and api-broken-flow.ci
// opens with it.
func exabgpCasePosition(prefix string) (connection, sequence int, err error) {
	if prefix == "" {
		return 0, 0, errors.New("empty position prefix")
	}
	connection = 1
	digits := prefix
	first := prefix[0]
	if first >= 'A' && first <= 'Z' {
		connection = int(first-'A') + 1
		digits = prefix[1:]
	}
	if first >= 'a' && first <= 'z' {
		connection = int(first-'a') + 1
		digits = prefix[1:]
	}
	sequence, err = strconv.Atoi(digits)
	if err != nil {
		return 0, 0, fmt.Errorf("prefix %q carries no sequence number: %w", prefix, err)
	}
	if sequence < 0 {
		return 0, 0, fmt.Errorf("prefix %q carries a negative sequence number", prefix)
	}
	return connection, sequence, nil
}
