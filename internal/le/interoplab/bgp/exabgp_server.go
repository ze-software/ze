package bgp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
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
	connections := 1
	if value, conversionErr := strconv.Atoi(os.Getenv("exabgp_tcp_connections")); conversionErr == nil && value > 0 {
		connections = value
	}
	var config net.ListenConfig
	listener, err := config.Listen(context.Background(), "tcp", fmt.Sprintf(":%d", *port))
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
	spec := exabgpCase{steps: make(map[int][]exabgpStep), asn: 65000}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if value, ok := strings.CutPrefix(line, "option=asn:"); ok {
			parsed, err := strconv.ParseUint(value, 10, 32)
			if err != nil {
				return exabgpCase{}, err
			}
			spec.asn = uint32(parsed)
			spec.asnStated = true
			continue
		}
		// `option=update:` names an UPDATE this mock sends to the speaker once
		// the session is up. It was read as nothing until 2026-09-06, so a case
		// whose script waits for that route waited for ever.
		if value, ok := strings.CutPrefix(line, "option=update:"); ok {
			if value != "send-default-route" {
				return exabgpCase{}, fmt.Errorf("unknown option=update: %q", value)
			}
			spec.sendDefaultRoute = true
			continue
		}
		parts := strings.Split(line, ":")
		if len(parts) < 3 {
			continue
		}
		// `<prefix>:signal:<NAME>` names a point in the script where the RUNNER
		// must signal the speaker. It was read as nothing until 2026-09-06, so
		// api-reload's reload never happened and its post-reload frames never
		// arrived.
		if parts[1] == "signal" {
			connection, err := exabgpCaseConnection(parts[0])
			if err != nil {
				return exabgpCase{}, fmt.Errorf("invalid signal connection in %q: %w", line, err)
			}
			name := strings.TrimSpace(parts[2])
			if name == "" {
				return exabgpCase{}, fmt.Errorf("signal directive names no signal: %q", line)
			}
			spec.steps[connection] = append(spec.steps[connection], exabgpStep{kind: exabgpStepSignal, signal: name})
			continue
		}
		if len(parts) < 4 || parts[1] != "raw" {
			continue
		}
		connection, err := exabgpCaseConnection(parts[0])
		if err != nil {
			return exabgpCase{}, fmt.Errorf("invalid raw connection in %q: %w", line, err)
		}
		wire, err := hex.DecodeString(strings.Join(parts[2:], ""))
		if err != nil {
			return exabgpCase{}, fmt.Errorf("decode raw directive: %w", err)
		}
		spec.steps[connection] = append(spec.steps[connection], exabgpStep{kind: exabgpStepFrame, frame: wire})
	}
	return spec, scanner.Err()
}

// exabgpStepKind says what one entry of a connection's script is. Zero is
// Unspecified, so a step nobody wrote is never read as a frame the speaker owes.
type exabgpStepKind uint8

const (
	exabgpStepUnspecified exabgpStepKind = iota
	exabgpStepFrame
	exabgpStepSignal
)

// exabgpStep is one entry of a connection's script, in the order the fixture
// wrote it.
//
// A frame step is an expectation the speaker owes. A signal step is an
// instruction to the RUNNER, which owns the ze process this mock cannot reach,
// and it divides the script into ordered segments: every frame before it must
// match before the signal is reported, and the frames after it are matched only
// once it has been.
type exabgpStep struct {
	kind   exabgpStepKind
	frame  []byte
	signal string
}

// exabgpSignalMarker opens the line this mock writes on its stdout when a
// script reaches a signal step. It follows the shape of the `PORT <n>` line the
// runner already reads off that same stream, and a frame dump is unspaced
// uppercase hex, so neither line can be read as the other.
const exabgpSignalMarker = "SIGNAL"

// exabgpCase is what one `.ci` fixture states: the script each connection owes,
// the AS this mock presents and whether the fixture named it, and the UPDATEs
// the mock sends of its own accord.
type exabgpCase struct {
	steps map[int][]exabgpStep
	asn   uint32
	// asnStated says the fixture named the AS. The caller needs it because the
	// default is a guess: upstream's runner opened no session at all, so no
	// fixture states an AS unless its author had a reason to.
	asnStated bool
	// sendDefaultRoute is `option=update:send-default-route`, which makes this
	// mock announce 0.0.0.0/32 once the session is up.
	sendDefaultRoute bool
}

// exabgpCaseConnection reads the connection a `.ci` expectation belongs to.
//
// The prefix comes in two shapes and both are upstream's. A NUMBER is the
// connection itself, so `1:raw:` is the first session. A LETTER names the
// connection and the digits after it are the sequence within that connection,
// so `A1:raw:` and `A2:raw:` are the first and second frames of connection A,
// and `B1:raw:` opens connection B. The lettered shape is what the multi-session
// cases use: api-teardown, api-reload, api-peer-lifecycle and api-notification.
//
// A parser that read the whole prefix as an integer refused every lettered case
// by name, which is how it was found, and one that read only the leading digits
// would silently merge two connections into one and assert their frames in the
// wrong order.
func exabgpCaseConnection(prefix string) (int, error) {
	if prefix == "" {
		return 0, errors.New("empty connection prefix")
	}
	first := prefix[0]
	if first >= 'A' && first <= 'Z' {
		return int(first-'A') + 1, nil
	}
	if first >= 'a' && first <= 'z' {
		return int(first-'a') + 1, nil
	}
	connection, err := strconv.Atoi(prefix)
	if err != nil {
		return 0, err
	}
	if connection < 0 {
		return 0, fmt.Errorf("connection %d is not a session", connection)
	}
	// Zero is the FIRST session, not an absent one. Upstream's own runner never
	// reads this number -- `_read_expected_messages` in exabgp's qa/bin/functional
	// splits the line on ':' and keeps only the `raw` payload -- so its fixtures
	// number from zero and from one interchangeably, and api-broken-flow.ci uses
	// BOTH for the same session: `0:` for the End-of-RIB and `1:` for the routes
	// that follow it on that same wire.
	if connection == 0 {
		return 1, nil
	}
	return connection, nil
}

func serveExaBGPConnection(connection net.Conn, spec exabgpCase, connectionIndex int, recorder *frameRecorder, output io.Writer) error {
	steps := spec.steps[connectionIndex]
	_ = connection.SetDeadline(time.Now().Add(60 * time.Second))
	messageType, body, err := readBGPWireMessage(connection)
	if err != nil {
		return err
	}
	if messageType != bgpOpen {
		return fmt.Errorf("first message type = %d, want OPEN", messageType)
	}
	matched := 0
	if len(steps) > 0 && steps[0].kind == exabgpStepFrame && len(steps[0].frame) >= bgpHeaderLength && steps[0].frame[18] == bgpOpen {
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
	if _, err := connection.Write(speakerMessage(bgpOpen, openBody)); err != nil {
		return err
	}
	if _, err := connection.Write(speakerKeepalive()); err != nil {
		return err
	}
	if spec.sendDefaultRoute {
		if _, err := connection.Write(exabgpDefaultRoute()); err != nil {
			return err
		}
	}
	// The script is walked one segment at a time, a segment being the frames
	// that stand between two signal steps. A fixture naming no signal has one
	// segment, which is the whole script and the behavior every case had before
	// signal steps existed.
	position := matched
	for position < len(steps) {
		end := position
		for end < len(steps) && steps[end].kind == exabgpStepFrame {
			end++
		}
		if err := matchExaBGPFrames(connection, steps[position:end], recorder); err != nil {
			return err
		}
		position = end
		if position == len(steps) {
			break
		}
		// Every frame the fixture wrote before this step has matched, so the
		// runner can act on it now. This mock runs in a separate process from
		// the speaker and holds only the TCP session, so reporting the step is
		// all it can do about it.
		if _, err := fmt.Fprintf(output, "%s %s\n", exabgpSignalMarker, steps[position].signal); err != nil {
			return err
		}
		position++
	}
	if count := recorder.mismatches(); count > 0 {
		return fmt.Errorf("%d frame(s) the speaker sent match no expectation", count)
	}
	return nil
}

// matchExaBGPFrames reads from the speaker until every frame of one segment has
// matched. Inside a segment the frames match in any order, because a fixture
// states what the speaker owes rather than the order its encoder picks.
func matchExaBGPFrames(connection net.Conn, segment []exabgpStep, recorder *frameRecorder) error {
	remaining := make([][]byte, 0, len(segment))
	for _, step := range segment {
		remaining = append(remaining, step.frame)
	}
	for len(remaining) > 0 {
		messageType, body, err := readBGPWireMessage(connection)
		if err != nil {
			return err
		}
		if messageType == bgpKeepalive {
			continue
		}
		actual := speakerMessage(messageType, body)
		recorder.record(actual)
		found := -1
		for index, wanted := range remaining {
			if bgpFrameEqual(actual, wanted) {
				found = index
				break
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
			return fmt.Errorf("unexpected message = %X; %d expected frames remain", actual, len(remaining))
		}
		remaining = append(remaining[:found], remaining[found+1:]...)
	}
	return nil
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
		parsed, parseErr := strconv.ParseUint(strings.TrimSuffix(fields[1], ";"), 10, 32)
		if parseErr != nil {
			return 0, false, fmt.Errorf("peer-as in %s: %w", path, parseErr)
		}
		return uint32(parsed), true, nil
	}
	return 0, false, scanner.Err()
}

// exabgpDefaultRoute is the UPDATE `option=update:send-default-route` sends: the
// prefix 0.0.0.0/32 with ORIGIN igp, an empty AS_PATH, NEXT_HOP 127.0.0.1 and
// LOCAL_PREF 100.
//
// The bytes are upstream's, from qa/sbin/bgp-3.6 in the ExaBGP repository, where
// the same option writes this literal. api-check's script waits for ze to render
// exactly this route back to it as a text event, so a byte invented here would
// be a route the case was never written about.
func exabgpDefaultRoute() []byte {
	return speakerMessage(bgpUpdate, []byte{
		0x00, 0x00, // Withdrawn Routes Length
		0x00, 0x15, // Total Path Attribute Length
		0x40, 0x01, 0x01, 0x00, // ORIGIN igp
		0x40, 0x02, 0x00, // AS_PATH, empty
		0x40, 0x03, 0x04, 0x7F, 0x00, 0x00, 0x01, // NEXT_HOP 127.0.0.1
		0x40, 0x05, 0x04, 0x00, 0x00, 0x00, 0x64, // LOCAL_PREF 100
		0x20, 0x00, 0x00, 0x00, 0x00, // NLRI 0.0.0.0/32
	})
}
