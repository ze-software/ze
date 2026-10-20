// Design: docs/architecture/testing/interop.md -- the mock ExaBGP speaker
// Related: exabgp_server_nobgp.go -- what the personality answers without the gate
// Related: exabgp_case_test.go -- the case reader's refusals and its sequence order
// RFC: rfc/short/rfc9003.md Section 2
//
// The personality renders each UPDATE it receives as ExaBGP JSON, through
// bridge.WireUpdateToExabgpJSON, which reads the message with the BGP engine's
// own types. Those live behind ze_bgp (feature-gates.txt), so this file does
// too and a build without BGP keeps the stub beside it.

//go:build ze_bgp

package bgp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/netip"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/asn"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/exabgp/bridge"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

func runExaBGPServer(args []string, output io.Writer) error {
	flags := flag.NewFlagSet("exabgp-server", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	port := flags.Int("port", 0, "listen port")
	_ = flags.Bool("terse", false, "terse output")
	// --save was accepted and ignored until 2026-09-05, which is a flag that
	// answers nothing: a caller passing it got the same run as a caller who did
	// not. It now captures every frame the speaker sent, which is what makes an
	// expectation adaptable by evidence rather than by eye.
	saveDir := flags.String("save", "", "directory to write the frames the speaker sent")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 1 || *port < 0 {
		return errors.New("exabgp-server wants [--port N] CASE.ci")
	}
	spec, err := readExaBGPCase(flags.Arg(0))
	if err != nil {
		return err
	}
	if !spec.asnStated {
		configured, found, asErr := peerASFromExaBGPConfig(os.Getenv("EXABGP_TEST_CONFIG"))
		if asErr != nil {
			return asErr
		}
		if found {
			spec.asn = configured
		}
	}
	// The script names the sessions it owes, so the script is what says how
	// many to accept. The count used to come from `exabgp_tcp_connections`,
	// which the runner sets for ze and never for this process, so every case
	// was served one session: api-teardown's B and C scripts were never read.
	connections := spec.connections
	// `option=bind:ipv6` makes the mock listen on IPv6 alone, so the case
	// passes only when the speaker reached it over IPv6. Upstream's mock binds
	// ::1, but conf-llnh-update and conf-llnh-lla-only name fd00::3 as their
	// neighbor, which a ::1 bind never accepts, so this one takes every IPv6
	// address. Without the option upstream listens on 127.0.0.1 alone, and this
	// mock on every address, which admits a speaker upstream's would refuse:
	// it is the looser of the two on that point.
	const (
		networkEveryFamily = "tcp"
		networkIPv6        = "tcp6"
	)
	network, host := networkEveryFamily, ""
	if spec.bindIPv6 {
		network, host = networkIPv6, "::"
	}
	var config net.ListenConfig
	listener, err := config.Listen(context.Background(), network, net.JoinHostPort(host, strconv.Itoa(*port)))
	if err != nil {
		return err
	}
	defer func() { _ = listener.Close() }()
	tcpAddress, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		return fmt.Errorf("TCP listener has address type %T", listener.Addr())
	}
	if _, err := fmt.Fprintf(output, "PORT %d\n", tcpAddress.Port); err != nil {
		return err
	}
	for connectionIndex := 1; connectionIndex <= connections; connectionIndex++ {
		connection, err := listener.Accept()
		if err != nil {
			return err
		}
		recorder, recorderErr := newFrameRecorder(*saveDir, flags.Arg(0), connectionIndex)
		if recorderErr != nil {
			return recorderErr
		}
		err = serveExaBGPConnection(connection, spec, connectionIndex, recorder, output)
		recorder.close()
		_ = connection.Close()
		if err != nil {
			return fmt.Errorf("connection %d: %w", connectionIndex, err)
		}
	}
	_, err = fmt.Fprintln(output, "successful")
	return err
}

// exabgpSignalMarker opens the line this mock writes on its stdout when a
// script reaches a signal step. It follows the shape of the `PORT <n>` line the
// runner already reads off that same stream, and a frame dump is unspaced
// uppercase hex, so neither line can be read as the other.
const exabgpSignalMarker = "SIGNAL"

// exabgpSessionLength bounds one session from its accept to its last step. A
// case that `option=timeout:` gives a per-message limit is still bounded by it.
const exabgpSessionLength = 60 * time.Second

// exabgpWire reads one session's messages, each under the earlier of the
// session's end and the case's per-message limit, so a speaker that sends
// keepalives and never the frame it owes still fails on the limit.
// Not safe for concurrent use: one session's walk owns it.
type exabgpWire struct {
	connection     net.Conn
	sessionEnd     time.Time
	messageTimeout time.Duration
}

// read reads the next whole message off the session.
func (w exabgpWire) read() (byte, []byte, error) {
	deadline := time.Now().Add(w.messageTimeout)
	if w.sessionEnd.Before(deadline) {
		deadline = w.sessionEnd
	}
	if err := w.connection.SetReadDeadline(deadline); err != nil {
		return 0, nil, err
	}
	return readBGPWireMessage(w.connection)
}

// addExaBGPUnknownCapability is upstream's _add_capa66: it appends to the
// mock's OPEN a Capabilities Optional Parameter carrying capability 66, whose
// value is the ten octets of "loremipsum", so the case proves the speaker
// keeps the session when its peer advertises a capability it does not know.
//
// RFC 4271 Section 4.2: "Optional Parameters Length: This 1-octet unsigned
// integer indicates the total length of the Optional Parameters field in
// octets." The body is Version (1), My AS (2), Hold Time (2), BGP Identifier
// (4), so that length is octet 9 and the parameters follow it.
//
//	+---------+----------+--------+-------------+------------------+
//	| type 2  | length 12| code 66| length 10   | "loremipsum"     |
//	+---------+----------+--------+-------------+------------------+
func addExaBGPUnknownCapability(openBody []byte) ([]byte, error) {
	const (
		parametersLengthOffset = 9
		parameterCapabilities  = 2
		capabilityUnknown      = 66
		parametersOctetsMax    = 255
	)
	value := []byte("loremipsum")
	parameter := append([]byte{parameterCapabilities, byte(2 + len(value)), capabilityUnknown, byte(len(value))}, value...)
	// RFC 9072 Section 2 marks the extended form with a length of 255, and
	// adding to a one-octet length past 255 would wrap it.
	length := int(openBody[parametersLengthOffset])
	if length+len(parameter) >= parametersOctetsMax {
		return nil, fmt.Errorf("the speaker's OPEN carries %d octets of optional parameters, and capability 66 would take "+
			"them to %d, past the one-octet length upstream's mock rewrites: shorten the case's config or drop "+
			"option=open:send-unknown-capability", length, length+len(parameter))
	}
	rewritten := append(slices.Clone(openBody), parameter...)
	rewritten[parametersLengthOffset] = byte(length + len(parameter))
	return rewritten, nil
}

func serveExaBGPConnection(connection net.Conn, spec exabgpCase, connectionIndex int, recorder *frameRecorder, output io.Writer) error {
	steps := spec.steps[connectionIndex]
	wire := exabgpWire{
		connection:     connection,
		sessionEnd:     time.Now().Add(exabgpSessionLength),
		messageTimeout: spec.messageTimeout,
	}
	if err := connection.SetWriteDeadline(wire.sessionEnd); err != nil {
		return err
	}
	messageType, body, err := wire.read()
	if err != nil {
		return fmt.Errorf("the speaker sent no OPEN: %w", err)
	}
	if messageType != bgpOpen {
		return fmt.Errorf("first message type = %d, want OPEN", messageType)
	}
	// orderExaBGPCase made the speaker's OPEN the first step of every session
	// under `option=open:inspect-open-message`, and refused it anywhere else.
	matched := 0
	if spec.inspectOpen {
		actualOpen := speakerMessage(bgpOpen, body)
		if !bytes.Equal(actualOpen, steps[0].frame) {
			return fmt.Errorf("OPEN = %X, want %X", actualOpen, steps[0].frame)
		}
		matched++
	}
	openBody := append([]byte(nil), body...)
	if len(openBody) < 10 {
		return errors.New("truncated OPEN")
	}
	// Read off the session itself, not off the fixture's ze config. Everything
	// an ExaBGP document states about the neighbor is here: ze's OPEN carries
	// its AS and its router id, and the socket carries both addresses. A
	// fixture that changed its config would otherwise need this mock changed
	// with it.
	facts := exabgpSessionFacts(connection, openBody, spec.asn)
	peerAS := uint16(spec.asn)
	if spec.asn > 0xffff {
		peerAS = 23456
	}
	binary.BigEndian.PutUint16(openBody[1:3], peerAS)
	routerID := []byte{10, 0, 0, 1}
	if bytes.Equal(openBody[5:9], routerID) {
		routerID[3] = 2
	}
	copy(openBody[5:9], routerID)
	rewriteAS4Capability(openBody, spec.asn)
	if spec.sendUnknownCapability {
		openBody, err = addExaBGPUnknownCapability(openBody)
		if err != nil {
			return err
		}
	}
	if _, err := connection.Write(speakerMessage(bgpOpen, openBody)); err != nil {
		return err
	}
	if _, err := connection.Write(speakerKeepalive()); err != nil {
		return err
	}
	if spec.sendUnknownMessage {
		// The 5.0 mock's unknown message: a header alone, of type 255.
		if _, err := connection.Write(speakerMessage(exabgpUnknownMessageType, nil)); err != nil {
			return err
		}
	}
	if spec.sendDefaultRoute {
		if _, err := connection.Write(exabgpDefaultRoute()); err != nil {
			return err
		}
	}
	// Under the 4.2 send-notification option the script is not walked: the
	// 4.2 mock answers each message with a Cease and reports success when the
	// connection closes, whatever its script still expected.
	if spec.notifyEachMessage {
		return notifyExaBGPUntilClosed(wire)
	}
	// The script is walked one sequence group at a time, in the order the
	// fixture's sequence numbers give, which is how upstream's own mock walks
	// it (Checker.group_messages, test/exabgp-compat/bin/bgp). Every group must
	// complete before the next starts, so a frame that arrives a group early is
	// refused, and a frame that never arrives leaves the read to fail on the
	// message timeout, the session's deadline or its close: none ends in success.
	replay := &exabgpReplay{}
	decoded := &exabgpDecodedRoutes{}
	position := matched
	for position < len(steps) {
		end := position + 1
		for end < len(steps) && steps[end].sequence == steps[position].sequence {
			end++
		}
		if err := runExaBGPGroup(wire, steps[position:end], recorder, facts, output, replay, decoded); err != nil {
			return err
		}
		position = end
	}
	if count := recorder.mismatches(); count > 0 {
		return fmt.Errorf("%d frame(s) the speaker sent match no expectation", count)
	}
	return nil
}

// runExaBGPGroup performs one sequence group of a session's script.
// orderExaBGPCase made an action the only step of its group, so a group is
// either frames or one action.
func runExaBGPGroup(wire exabgpWire, group []exabgpStep, recorder *frameRecorder, facts bridge.SessionFacts, output io.Writer, replay *exabgpReplay,
	decoded *exabgpDecodedRoutes,
) error {
	switch group[0].kind {
	case exabgpStepFrame:
		return matchExaBGPFrames(wire, group, recorder, facts, replay)
	case exabgpStepRoute:
		return matchExaBGPRoutes(wire, group, recorder, decoded)
	case exabgpStepSignal:
		// Every frame the fixture wrote before this step has matched, so the
		// runner can act on it now. This mock runs in a separate process from
		// the speaker and holds only the TCP session, so reporting the step is
		// all it can do about it.
		_, err := fmt.Fprintln(output, exabgpSignalMarker, group[0].signal)
		return err
	case exabgpStepNotification:
		_, err := wire.connection.Write(exabgpShutdownNotification(group[0].notification))
		return err
	case exabgpStepUnspecified:
	}
	panic("BUG: the case reader built a step of no kind")
}

// exabgpShutdownNotification builds the Cease NOTIFICATION a notification step
// sends, carrying its text as a Shutdown Communication. readExaBGPStep refused
// a text longer than exabgpShutdownCommunicationOctetsMax.
//
// RFC 9003 Section 2: "The Cease NOTIFICATION message with a Shutdown
// Communication is encoded as below":
//
//	 0                   1                   2                   3
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	| Error Code 6  |    Subcode    |    Length     |     ...       \
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+               /
//	/                 ... Shutdown Communication ...                /
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//
// Octet 0 is the code, octet 1 the subcode, octet 2 the length, and the text
// starts at octet 3.
func exabgpShutdownNotification(communication string) []byte {
	const (
		errorCodeCease = 6
		// RFC 9003 Section 2: "The Error Subcode value MUST be one of the
		// following values: 2 ("Administrative Shutdown") or 4 ("Administrative
		// Reset")."
		subcodeAdministrativeShutdown = 2
	)
	body := make([]byte, 3+len(communication))
	body[0] = errorCodeCease
	body[1] = subcodeAdministrativeShutdown
	body[2] = byte(len(communication))
	copy(body[3:], communication)
	return speakerMessage(bgpNotification, body)
}

// matchExaBGPFrames reads from the speaker until every frame of one group has
// matched. Inside a group the frames match in any order, because a fixture
// states what the speaker owes rather than the order its encoder picks.
func matchExaBGPFrames(wire exabgpWire, segment []exabgpStep, recorder *frameRecorder, facts bridge.SessionFacts, replay *exabgpReplay) error {
	remaining := make([]exabgpStep, 0, len(segment))
	remaining = append(remaining, segment...)
	for len(remaining) > 0 {
		messageType, body, err := wire.read()
		// A read that fails here is a group the speaker stalled on, so the
		// error says what it still owed, not only that the socket ended.
		if err != nil {
			return fmt.Errorf("sequence %d: %d expected frame(s) never arrived (first: %X): %w",
				segment[0].sequence, len(remaining), remaining[0].frame, err)
		}
		if messageType == bgpKeepalive {
			continue
		}
		actual := speakerMessage(messageType, body)
		recorder.record(actual)
		found := -1
		for index, wanted := range remaining {
			if bgpFrameEqual(actual, wanted.frame) {
				found = index
				break
			}
		}
		// The frame matched, so the fixture's document for it is now owed.
		// Checked here rather than beside the hex because a `.ci` states the
		// two as one expectation: the bytes, and what a script attached to
		// this peer reads when those bytes arrive.
		if found >= 0 && remaining[found].wantJSON != "" {
			if err := matchExaBGPDocument(body, remaining[found].wantJSON, facts); err != nil {
				return err
			}
		}
		if found < 0 {
			// Capturing beats stopping when a recorder is open: an adaptation
			// needs every frame the speaker sent, not the first one that
			// disagreed. The run still fails, at the end, with the count.
			if recorder.capturing() {
				recorder.mismatch()
				continue
			}
			// The first frame still owed is named as the read failure above
			// names it, so one allowance text covers a case whichever way the
			// missing frame shows: a timeout, or a frame that arrived instead.
			return fmt.Errorf("unexpected message = %X; %d expected frames remain (first: %X)",
				actual, len(remaining), remaining[0].frame)
		}
		remaining = append(remaining[:found], remaining[found+1:]...)
		if err := replay.matched(wire.connection, messageType, actual); err != nil {
			return err
		}
	}
	return nil
}

// bgpRouteRefresh is the ROUTE-REFRESH message type (RFC 2918 Section 3).
const bgpRouteRefresh = 5

// exabgpReplay is upstream's answer to a ROUTE-REFRESH from the speaker: the
// mock sends back every UPDATE the speaker sent it, then a KEEPALIVE
// (Checker._send_rr_if_required, test/exabgp-compat/bin/bgp). api-rr's script
// reads those routes back and announces its next route only when both arrive,
// so a mock without the replay can never complete that case.
//
// The replay is per session: serveExaBGPConnection makes one for each
// connection, where upstream's updates_seen lives on the checker and so
// spans every session of the run. No fixture sends a ROUTE-REFRESH on a
// second session, so the two agree on every case today. An End-of-RIB, in
// either RFC 4724 form, is no route and is not replayed, as upstream's
// is_eor excludes it.
//
// It holds the matched UPDATEs of one session, which the fixture bounds.
// Not safe for concurrent use: one session's walk owns it.
type exabgpReplay struct {
	updates [][]byte
}

// matched records one matched frame, and answers a ROUTE-REFRESH with the
// replay.
func (r *exabgpReplay) matched(connection net.Conn, messageType byte, frame []byte) error {
	if messageType == bgpUpdate && documentableFrame(frame) {
		r.updates = append(r.updates, frame)
		return nil
	}
	if messageType != bgpRouteRefresh {
		return nil
	}
	for _, update := range r.updates {
		if _, err := connection.Write(update); err != nil {
			return err
		}
	}
	_, err := connection.Write(speakerKeepalive())
	return err
}

// matchExaBGPDocument compares the ExaBGP document one UPDATE renders to
// against the one its fixture states.
//
// Both sides are put through bridge.DropVolatile first: it removes the members
// no two runs agree on, which is upstream's own _cleanup list, and folds the
// direction vocabulary, because the ExaBGP DAEMON writes receive/send while the
// fixtures carry the in/out its test harness writes.
func matchExaBGPDocument(payload []byte, want string, facts bridge.SessionFacts) error {
	var expected map[string]any
	if err := json.Unmarshal([]byte(want), &expected); err != nil {
		return fmt.Errorf("the fixture's json expectation does not parse: %w", err)
	}
	rendered, err := bridge.WireUpdateToExabgpJSON(payload, facts, rpc.DirectionReceived)
	if err != nil {
		return fmt.Errorf("rendering the frame as ExaBGP json: %w", err)
	}
	bridge.DropVolatile(expected)
	bridge.DropVolatile(rendered)

	gotText, err := json.Marshal(rendered)
	if err != nil {
		return err
	}
	wantText, err := json.Marshal(expected)
	if err != nil {
		return err
	}
	if !bytes.Equal(canonicalJSON(gotText), canonicalJSON(wantText)) {
		return fmt.Errorf("json expectation:\n  want %s\n  got  %s", wantText, gotText)
	}
	return nil
}

// exabgpSessionFacts reads the neighbor facts an ExaBGP document states off the
// session, so no fixture has to repeat them here.
//
// The perspective is ZE's, because the document is what a process attached to
// ze would read: `local` is ze, which is this mock's REMOTE, and `peer` is this
// mock. ze's AS and router id come out of the OPEN it just sent.
func exabgpSessionFacts(connection net.Conn, openBody []byte, mockASN uint32) bridge.SessionFacts {
	facts := bridge.SessionFacts{PeerAS: mockASN}
	if host, _, err := net.SplitHostPort(connection.RemoteAddr().String()); err == nil {
		facts.Local, _ = netip.ParseAddr(host)
	}
	if host, _, err := net.SplitHostPort(connection.LocalAddr().String()); err == nil {
		facts.Peer, _ = netip.ParseAddr(host)
	}
	if len(openBody) >= 9 {
		facts.LocalAS = uint32(binary.BigEndian.Uint16(openBody[1:3]))
		facts.RouterID = binary.BigEndian.Uint32(openBody[5:9])
	}
	facts.Encoding = exabgpNegotiatedEncoding(openBody)
	return facts
}

// exabgpNegotiatedEncoding reads what the speaker's OPEN advertised, so the
// renderer decodes the UPDATEs that follow the way the session encoded them.
//
// Only ADD-PATH is read, and only because RFC 7911 Section 3 changes the NLRI's
// own LAYOUT: without it a decoder reads the 4-octet Path Identifier as prefix
// bytes, and one route comes out as five. Every other capability changes what
// an UPDATE may contain rather than how its octets are cut, so a document
// rendered without them is still the right document.
//
// This mock mirrors the speaker's OPEN back to it, so what the speaker
// advertised is what the session negotiated.
func exabgpNegotiatedEncoding(openBody []byte) *capability.EncodingCaps {
	open, err := message.UnpackOpen(openBody)
	if err != nil {
		return nil
	}
	// RFC 9072 Section 2: ExaBGP sends the extended form above 255 octets, and
	// the OPEN's own framing decides the Parameter Length width.
	caps, err := capability.ParseFromOptionalParams(open.OptionalParams, open.ExtendedParams)
	if err != nil {
		return nil
	}
	for _, advertised := range caps {
		addPath, isAddPath := advertised.(*capability.AddPath)
		if !isAddPath {
			continue
		}
		modes := make(map[family.Family]capability.AddPathMode, len(addPath.Families))
		for _, entry := range addPath.Families {
			modes[family.Family{AFI: entry.AFI, SAFI: entry.SAFI}] = entry.Mode
		}
		return &capability.EncodingCaps{ASN4: true, AddPathMode: modes}
	}
	return nil
}

// canonicalJSON re-marshals a document so two equal documents compare equal
// whatever order their members were written in.
func canonicalJSON(document []byte) []byte {
	var value any
	if err := json.Unmarshal(document, &value); err != nil {
		return document
	}
	canonical, err := json.Marshal(value)
	if err != nil {
		return document
	}
	return canonical
}

// frameRecorder writes every frame the speaker sent, so an expectation can be
// adapted against evidence. A nil-valued recorder is the ordinary run: record
// and close do nothing and capturing answers false, so the serve loop keeps its
// stop-at-the-first-mismatch behavior.
type frameRecorder struct {
	file  *os.File
	wrong int
}

func newFrameRecorder(directory, casePath string, connection int) (*frameRecorder, error) {
	if directory == "" {
		return &frameRecorder{}, nil
	}
	if err := os.MkdirAll(directory, 0o750); err != nil {
		return nil, err
	}
	stem := strings.TrimSuffix(filepath.Base(casePath), filepath.Ext(casePath))
	name := filepath.Join(directory, fmt.Sprintf("%s.%d.frames", stem, connection))
	file, err := os.Create(name) //nolint:gosec // the directory is named on this helper's own command line
	if err != nil {
		return nil, err
	}
	return &frameRecorder{file: file}, nil
}

func (r *frameRecorder) capturing() bool { return r != nil && r.file != nil }

func (r *frameRecorder) record(frame []byte) {
	if !r.capturing() {
		return
	}
	_, _ = fmt.Fprintf(r.file, "%X\n", frame)
}

func (r *frameRecorder) mismatch() {
	if r != nil {
		r.wrong++
	}
}

func (r *frameRecorder) mismatches() int {
	if r == nil {
		return 0
	}
	return r.wrong
}

func (r *frameRecorder) close() {
	if r.capturing() {
		_ = r.file.Close()
	}
}

func readBGPWireMessage(reader io.Reader) (byte, []byte, error) {
	header := make([]byte, bgpHeaderLength)
	if _, err := io.ReadFull(reader, header); err != nil {
		return 0, nil, err
	}
	length := int(binary.BigEndian.Uint16(header[16:18]))
	if length < bgpHeaderLength || length > 65535 {
		return 0, nil, fmt.Errorf("invalid BGP length %d", length)
	}
	body := make([]byte, length-bgpHeaderLength)
	_, err := io.ReadFull(reader, body)
	return header[18], body, err
}

func rewriteAS4Capability(openBody []byte, asn uint32) {
	for index := 10; index+2 <= len(openBody); {
		parameterLength := int(openBody[index+1])
		parameterEnd := index + 2 + parameterLength
		if parameterEnd > len(openBody) {
			return
		}
		if openBody[index] == 2 {
			for capability := index + 2; capability+2 <= parameterEnd; {
				capabilityLength := int(openBody[capability+1])
				capabilityEnd := capability + 2 + capabilityLength
				if capabilityEnd > parameterEnd {
					return
				}
				if openBody[capability] == 65 && capabilityLength == 4 {
					binary.BigEndian.PutUint32(openBody[capability+2:capabilityEnd], asn)
				}
				capability = capabilityEnd
			}
		}
		index = parameterEnd
	}
}

// peerASFromExaBGPConfig reads the AS this mock must present, out of the ExaBGP
// config the case under test runs.
//
// ze reads `peer-as` as the AS it REQUIRES the far end to open with, and RFC
// 4271 Section 6.2 makes any other AS a Bad Peer AS. The mock is that far end,
// so opening with a fixed 65000 made ze refuse every session whose config named
// a different AS. It went unnoticed while ze accepted an OPEN from any AS at
// all; the moment the check existed, 31 of the 42 encoding cases went red on a
// NOTIFICATION rather than on a frame.
//
// The value is read from the ExaBGP config rather than added to 42 `.ci` files,
// because the config is where it is already stated and a second copy would be a
// second thing to keep true (ai/rules/principles.md). An explicit
// `option=asn:` in the case still wins: a case that states its own AS is stating
// it for a reason.
//
// It answers found=false rather than an error for a config it cannot read, so a
// case run without EXABGP_TEST_CONFIG keeps the old default instead of failing
// on an environment variable.
func peerASFromExaBGPConfig(path string) (uint32, bool, error) {
	if path == "" {
		return 0, false, nil
	}
	file, err := os.Open(path) //nolint:gosec // the path is the fixture config the tracked runner points this helper at
	if err != nil {
		return 0, false, nil
	}
	defer func() { _ = file.Close() }()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Fields(strings.TrimSpace(scanner.Text()))
		if len(fields) < 2 || fields[0] != "peer-as" {
			continue
		}
		parsed, parseErr := asn.Parse(strings.TrimSuffix(fields[1], ";"))
		if parseErr != nil {
			return 0, false, fmt.Errorf("peer-as in %s: %w", path, parseErr)
		}
		return parsed, true, nil
	}
	return 0, false, scanner.Err()
}

// exabgpDefaultRoute is the UPDATE `option=update:send-default-route` sends: the
// prefix 0.0.0.0/32 with ORIGIN igp, an empty AS_PATH, NEXT_HOP 192.0.2.1 and
// LOCAL_PREF 100.
//
// The upstream fixture used loopback as NEXT_HOP. Keep its prefix and other
// attributes, but use a non-local host address so api-check exercises an
// announcement rather than RFC 7606 treat-as-withdraw.
func exabgpDefaultRoute() []byte {
	return speakerMessage(bgpUpdate, []byte{
		0x00, 0x00, // Withdrawn Routes Length
		0x00, 0x15, // Total Path Attribute Length
		0x40, 0x01, 0x01, 0x00, // ORIGIN igp
		0x40, 0x02, 0x00, // AS_PATH, empty
		0x40, 0x03, 0x04, 0xc0, 0x00, 0x02, 0x01, // NEXT_HOP 192.0.2.1
		0x40, 0x05, 0x04, 0x00, 0x00, 0x00, 0x64, // LOCAL_PREF 100
		0x20, 0x00, 0x00, 0x00, 0x00, // NLRI 0.0.0.0/32
	})
}
