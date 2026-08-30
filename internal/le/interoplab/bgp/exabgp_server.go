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
	"strconv"
	"strings"
	"time"
)

func runExaBGPServer(args []string, output io.Writer) error {
	flags := flag.NewFlagSet("exabgp-server", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	port := flags.Int("port", 0, "listen port")
	_ = flags.Bool("terse", false, "terse output")
	_ = flags.String("save", "", "result directory")
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
		err = serveExaBGPConnection(connection, asn, expected[connectionIndex])
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
		connection, err := strconv.Atoi(parts[0])
		if err != nil || connection <= 0 {
			return nil, 0, fmt.Errorf("invalid raw connection in %q", line)
		}
		wire, err := hex.DecodeString(strings.Join(parts[2:], ""))
		if err != nil {
			return nil, 0, fmt.Errorf("decode raw directive: %w", err)
		}
		result[connection] = append(result[connection], wire)
	}
	return result, asn, scanner.Err()
}

func serveExaBGPConnection(connection net.Conn, asn uint32, expected [][]byte) error {
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
		found := -1
		for index, wanted := range remaining {
			if bytes.Equal(actual, wanted) {
				found = index
				break
			}
		}
		if found < 0 {
			return fmt.Errorf("unexpected message = %X; %d expected frames remain", actual, len(remaining))
		}
		remaining = append(remaining[:found], remaining[found+1:]...)
	}
	return nil
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
