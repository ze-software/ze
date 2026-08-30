// Design: docs/architecture/testing/interop.md -- independent RADIUS wire peer for the L2TP accounting scenario.
// Related: RFC 2865 section 3 -- packet framing and response authenticator.
// Related: RFC 2866 section 3 -- Accounting-Response packet framing.
package main

import (
	"crypto/md5" //nolint:gosec // RADIUS response authenticators require MD5 in RFC 2865 section 3.
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

const (
	codeAccessRequest      = 1
	codeAccessAccept       = 2
	codeAccountingRequest  = 4
	codeAccountingResponse = 5

	attributeUserName         = 1
	attributeNASIPAddress     = 4
	attributeFramedIPAddress  = 8
	attributeAccountingStatus = 40
	attributeAccountingID     = 44
	attributeNASPortID        = 87

	packetHeaderOctets = 20
	packetOctetsMax    = 4096
)

type attribute struct {
	kind  byte
	value []byte
}

func main() {
	port, err := environmentPort()
	if err != nil {
		exitError(err)
		return
	}
	secret := []byte(os.Getenv("RADIUS_KEY"))
	if len(secret) == 0 {
		secret = []byte("testing123")
	}
	if err := serve(port, secret); err != nil {
		exitError(err)
	}
}

func serve(port int, secret []byte) (result error) {
	address := &net.UDPAddr{IP: net.IPv4zero, Port: port}
	connection, err := net.ListenUDP("udp4", address)
	if err != nil {
		return fmt.Errorf("listen for RADIUS: %w", err)
	}
	defer func() {
		err := connection.Close()
		if err == nil {
			return
		}
		if result == nil {
			result = fmt.Errorf("close RADIUS socket: %w", err)
		}
	}()
	if _, err := fmt.Fprintf(os.Stdout, "radius-mock listening on 0.0.0.0:%d\n", port); err != nil {
		return fmt.Errorf("write RADIUS readiness: %w", err)
	}

	packet := make([]byte, packetOctetsMax)
	// The daemon serves packets for the container lifetime. Docker owns the bound.
	for {
		octets, peer, err := connection.ReadFromUDP(packet)
		if err != nil {
			return fmt.Errorf("read RADIUS packet: %w", err)
		}
		response, description, ok := handlePacket(packet[:octets], secret)
		if !ok {
			continue
		}
		if _, err := fmt.Fprintln(os.Stdout, description); err != nil {
			return fmt.Errorf("write RADIUS observation: %w", err)
		}
		if _, err := connection.WriteToUDP(response, peer); err != nil {
			return fmt.Errorf("write RADIUS response: %w", err)
		}
	}
}

func exitError(err error) {
	if _, writeErr := fmt.Fprintln(os.Stderr, err); writeErr != nil {
		os.Exit(1)
	}
	os.Exit(1)
}

func handlePacket(packet, secret []byte) ([]byte, string, bool) {
	if len(packet) < packetHeaderOctets {
		return nil, "", false
	}
	length := int(binary.BigEndian.Uint16(packet[2:4]))
	if length < packetHeaderOctets || length > len(packet) {
		return nil, "", false
	}
	code := packet[0]
	var responseCode byte
	switch code {
	case codeAccessRequest:
		responseCode = codeAccessAccept
	case codeAccountingRequest:
		responseCode = codeAccountingResponse
	default:
		return nil, "", false
	}
	attributes := parseAttributes(packet[packetHeaderOctets:length])
	response := make([]byte, packetHeaderOctets)
	response[0] = responseCode
	response[1] = packet[1]
	binary.BigEndian.PutUint16(response[2:4], packetHeaderOctets)
	authenticator := responseAuthenticator(responseCode, packet[1], packet[4:20], secret)
	copy(response[4:20], authenticator[:])
	return response, describe(code, attributes), true
}

// RFC 2865 section 5: Type(1), Length(1), Value(Length-2), repeated.
func parseAttributes(data []byte) []attribute {
	attributes := make([]attribute, 0, 8)
	for offset := 0; offset+2 <= len(data); {
		length := int(data[offset+1])
		if length < 2 || offset+length > len(data) {
			break
		}
		attributes = append(attributes, attribute{kind: data[offset], value: data[offset+2 : offset+length]})
		offset += length
	}
	return attributes
}

func responseAuthenticator(code, identifier byte, requestAuthenticator, secret []byte) [md5.Size]byte {
	material := make([]byte, 0, packetHeaderOctets+len(secret))
	material = append(material, code, identifier, 0, packetHeaderOctets)
	material = append(material, requestAuthenticator...)
	material = append(material, secret...)
	return md5.Sum(material) //nolint:gosec // RFC 2865 section 3 fixes this algorithm.
}

func describe(code byte, attributes []attribute) string {
	fields := make([]string, 0, len(attributes))
	var tb textbuf.Buffer
	for _, attribute := range attributes {
		switch attribute.kind {
		case attributeNASPortID:
			fields = append(fields, tb.Reset().Str("NAS-Port-Id=").
				Str(validText(attribute.value)).String())
		case attributeFramedIPAddress:
			if len(attribute.value) == net.IPv4len {
				fields = append(fields, tb.Reset().Str("Framed-IP-Address=").
					Str(net.IP(attribute.value).String()).String())
			}
		case attributeNASIPAddress:
			if len(attribute.value) == net.IPv4len {
				fields = append(fields, tb.Reset().Str("NAS-IP-Address=").
					Str(net.IP(attribute.value).String()).String())
			}
		case attributeAccountingStatus:
			if len(attribute.value) == 4 {
				fields = append(fields, tb.Reset().Str("Acct-Status-Type=").
					Str(accountingStatus(binary.BigEndian.Uint32(attribute.value))).String())
			}
		case attributeAccountingID:
			fields = append(fields, tb.Reset().Str("Acct-Session-Id=").
				Str(validText(attribute.value)).String())
		case attributeUserName:
			fields = append(fields, tb.Reset().Str("User-Name=").
				Str(validText(attribute.value)).String())
		}
	}
	name := tb.Reset().Str("code-").Uint8(code).String()
	if code == codeAccessRequest {
		name = "Access-Request"
	}
	if code == codeAccountingRequest {
		name = "Accounting-Request"
	}
	return tb.Reset().Str("RADIUS-RX ").Str(name).Byte(' ').Join(fields, " ").String()
}

func accountingStatus(status uint32) string {
	switch status {
	case 1:
		return "Start"
	case 2:
		return "Stop"
	case 3:
		return "Interim-Update"
	default:
		return textbuf.StringUint32(status)
	}
}

func validText(value []byte) string { return strings.ToValidUTF8(string(value), "�") }

func environmentPort() (int, error) {
	value := os.Getenv("RADIUS_PORT")
	if value == "" {
		return 1812, nil
	}
	port, err := strconv.Atoi(value)
	if err != nil || port < 1 || port > 65535 {
		return 0, errors.New("RADIUS_PORT must be between 1 and 65535")
	}
	return port, nil
}
