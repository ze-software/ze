// Design: docs/architecture/testing/interop.md -- independent RADIUS wire peer for the L2TP accounting scenario.
// Related: RFC 2865 section 3 -- packet framing and response authenticator.
// Related: RFC 2866 section 3 -- Accounting-Response packet framing.
// Related: RFC 2869 section 5.14 -- Message-Authenticator on an Access-Accept.
package main

import (
	"crypto/hmac"
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

	attributeUserName             = 1
	attributeNASIPAddress         = 4
	attributeFramedIPAddress      = 8
	attributeCallingStationID     = 31
	attributeAccountingStatus     = 40
	attributeAccountingDelay      = 41
	attributeAccountingID         = 44
	attributeTerminateCause       = 49
	attributeEventTimestamp       = 55
	attributeMessageAuthenticator = 80
	attributeNASPortID            = 87

	packetHeaderOctets = 20
	packetOctetsMax    = 4096

	// messageAuthenticatorAttributeOctets is the Type, Length and 16-octet
	// HMAC-MD5 of a Message-Authenticator (RFC 2869 section 5.14).
	messageAuthenticatorAttributeOctets = 18
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
	response := make([]byte, packetHeaderOctets, packetHeaderOctets+messageAuthenticatorAttributeOctets)
	response[0] = responseCode
	response[1] = packet[1]
	binary.BigEndian.PutUint16(response[2:4], packetHeaderOctets)
	// Both authenticators of a response are computed with the Request
	// Authenticator in the authenticator field, so it holds the request's
	// value until the response is complete.
	copy(response[4:20], packet[4:20])
	if responseCode == codeAccessAccept {
		response = appendMessageAuthenticator(response, secret)
	}
	authenticator := responseAuthenticator(response, secret)
	copy(response[4:20], authenticator[:])
	return response, describe(code, attributes), true
}

// appendMessageAuthenticator signs an Access-Accept whose authenticator field
// still holds the Request Authenticator. Ze discards an unsigned answer to an
// Access-Request unless the operator accepts a legacy server (BlastRADIUS,
// CVE-2024-3596), so this peer answers as a server updated for it does.
//
// RFC 2869 Section 5.14: "For Access-Challenge, Access-Accept, and
// Access-Reject packets, the Message-Authenticator is calculated as follows,
// using the Request-Authenticator from the Access-Request this packet is in
// reply to:"
//
// RFC 2869 Section 5.14: "When the checksum is calculated the signature string
// should be considered to be sixteen octets of zero."
//
//	 0                   1                   2
//	 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	|  Type (80)    |  Length (18)  |  HMAC-MD5 (16 octets) ...
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	offset 20 (after the header): Type, 21: Length, 22..37: String
func appendMessageAuthenticator(response, secret []byte) []byte {
	valueOffset := len(response) + 2
	response = append(response, attributeMessageAuthenticator, messageAuthenticatorAttributeOctets)
	response = append(response, make([]byte, md5.Size)...)
	binary.BigEndian.PutUint16(response[2:4], packetHeaderOctets+messageAuthenticatorAttributeOctets)
	mac := hmac.New(md5.New, secret)
	mac.Write(response)
	copy(response[valueOffset:], mac.Sum(nil))
	return response
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

// responseAuthenticator computes the Response Authenticator of a response
// whose authenticator field holds the Request Authenticator.
//
// RFC 2865 Section 3: "ResponseAuth =
// MD5(Code+ID+Length+RequestAuth+Attributes+Secret)"
func responseAuthenticator(response, secret []byte) [md5.Size]byte {
	digest := md5.New() //nolint:gosec // RFC 2865 section 3 fixes this algorithm.
	digest.Write(response)
	digest.Write(secret)
	var sum [md5.Size]byte
	digest.Sum(sum[:0])
	return sum
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
		case attributeCallingStationID:
			fields = append(fields, tb.Reset().Str("Calling-Station-Id=").
				Str(validText(attribute.value)).String())
		case attributeAccountingDelay:
			if len(attribute.value) == 4 {
				fields = append(fields, tb.Reset().Str("Acct-Delay-Time=").
					Uint32(binary.BigEndian.Uint32(attribute.value)).String())
			}
		case attributeTerminateCause:
			if len(attribute.value) == 4 {
				fields = append(fields, tb.Reset().Str("Acct-Terminate-Cause=").
					Uint32(binary.BigEndian.Uint32(attribute.value)).String())
			}
		case attributeEventTimestamp:
			if len(attribute.value) == 4 {
				fields = append(fields, tb.Reset().Str("Event-Timestamp=").
					Uint32(binary.BigEndian.Uint32(attribute.value)).String())
			}
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
