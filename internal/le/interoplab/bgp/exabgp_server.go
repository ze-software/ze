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
	expected, asn, err := readExaBGPCase(flags.Arg(0))
	if err != nil {
		return err
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
		err = serveExaBGPConnection(connection, asn, expected[connectionIndex], recorder)
		recorder.close()
		_ = connection.Close()
		if err != nil {
			return fmt.Errorf("connection %d: %w", connectionIndex, err)
		}
	}
	_, err = fmt.Fprintln(output, "successful")
	return err
}

func readExaBGPCase(path string) (map[int][][]byte, uint32, error) {
	file, err := os.Open(path) //nolint:gosec // the case file is the .ci fixture this helper is pointed at by the tracked lab runner
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = file.Close() }()
	result := make(map[int][][]byte)
	asn := uint32(65000)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if value, ok := strings.CutPrefix(line, "option=asn:"); ok {
			parsed, err := strconv.ParseUint(value, 10, 32)
			if err != nil {
				return nil, 0, err
			}
			asn = uint32(parsed)
			continue
		}
		parts := strings.Split(line, ":")
		if len(parts) < 4 || parts[1] != "raw" {
			continue
		}
		connection, err := exabgpCaseConnection(parts[0])
		if err != nil {
			return nil, 0, fmt.Errorf("invalid raw connection in %q: %w", line, err)
		}
		wire, err := hex.DecodeString(strings.Join(parts[2:], ""))
		if err != nil {
			return nil, 0, fmt.Errorf("decode raw directive: %w", err)
		}
		result[connection] = append(result[connection], wire)
	}
	return result, asn, scanner.Err()
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
	if connection <= 0 {
		return 0, fmt.Errorf("connection %d is not a session", connection)
	}
	return connection, nil
}

func serveExaBGPConnection(connection net.Conn, asn uint32, expected [][]byte, recorder *frameRecorder) error {
	_ = connection.SetDeadline(time.Now().Add(60 * time.Second))
	messageType, body, err := readBGPWireMessage(connection)
	if err != nil {
		return err
	}
	if messageType != bgpOpen {
		return fmt.Errorf("first message type = %d, want OPEN", messageType)
	}
	matched := 0
	if len(expected) > 0 && len(expected[0]) >= bgpHeaderLength && expected[0][18] == bgpOpen {
		actualOpen := speakerMessage(bgpOpen, body)
		if !bytes.Equal(actualOpen, expected[0]) {
			return fmt.Errorf("OPEN = %X, want %X", actualOpen, expected[0])
		}
		matched++
	}
	openBody := append([]byte(nil), body...)
	if len(openBody) < 10 {
		return errors.New("truncated OPEN")
	}
	peerAS := uint16(asn)
	if asn > 0xffff {
		peerAS = 23456
	}
	binary.BigEndian.PutUint16(openBody[1:3], peerAS)
	routerID := []byte{10, 0, 0, 1}
	if bytes.Equal(openBody[5:9], routerID) {
		routerID[3] = 2
	}
	copy(openBody[5:9], routerID)
	rewriteAS4Capability(openBody, asn)
	if _, err := connection.Write(speakerMessage(bgpOpen, openBody)); err != nil {
		return err
	}
	if _, err := connection.Write(speakerKeepalive()); err != nil {
		return err
	}
	remaining := append([][]byte(nil), expected[matched:]...)
	for len(remaining) > 0 {
		messageType, body, err = readBGPWireMessage(connection)
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
			if bytes.Equal(actual, wanted) {
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
	if count := recorder.mismatches(); count > 0 {
		return fmt.Errorf("%d frame(s) the speaker sent match no expectation", count)
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
