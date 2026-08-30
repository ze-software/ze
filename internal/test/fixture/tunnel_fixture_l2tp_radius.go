package fixture

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/md5" //nolint:gosec // L2TP CHAP and RADIUS require MD5 on the wire
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

func tunnelRadiusAttr(attribute byte, value []byte) []byte {
	return append([]byte{attribute, byte(len(value) + 2)}, value...)
}

func tunnelRadiusUint32Attr(attribute byte, value uint32) []byte {
	return tunnelRadiusAttr(attribute, tunnelL2TPU32(value))
}

func tunnelRadiusDriver(defaultPort int, accessCode byte, accessAttrs []byte) Driver {
	return func(ctx context.Context, args []string) error {
		port := defaultPort
		if configured := os.Getenv("RADIUS_PORT"); configured != "" {
			parsed, err := strconv.Atoi(configured)
			if err != nil || parsed < 1 || parsed > 65535 {
				return fmt.Errorf("invalid RADIUS_PORT %q", configured)
			}
			port = parsed
		}
		if len(args) > 0 {
			parsed, err := tunnelArgPort(args, 0)
			if err != nil {
				return err
			}
			port = parsed
		}
		secret := []byte(os.Getenv("RADIUS_KEY"))
		if len(secret) == 0 {
			secret = []byte("testing123")
		}
		if len(args) > 1 {
			secret = []byte(args[1])
		}
		conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: port})
		if err != nil {
			return err
		}
		defer conn.Close() //nolint:errcheck // fixture teardown
		fmt.Printf("RADIUS mock listening on 127.0.0.1:%d\n", port)
		buffer := make([]byte, 4096)
		for {
			_ = conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
			n, address, err := conn.ReadFromUDP(buffer)
			if err != nil {
				if ctx.Err() != nil {
					return nil
				}
				if netErr, ok := errors.AsType[net.Error](err); ok && netErr.Timeout() {
					continue
				}
				return err
			}
			if n < 20 {
				continue
			}
			code := buffer[0]
			var responseCode byte
			var attrs []byte
			switch code {
			case 1:
				responseCode, attrs = accessCode, accessAttrs
			case 4:
				responseCode = 5
			default:
				continue
			}
			response := make([]byte, 20+len(attrs))
			response[0], response[1] = responseCode, buffer[1]
			binary.BigEndian.PutUint16(response[2:4], uint16(len(response)))
			copy(response[20:], attrs)
			hash := md5.New() //nolint:gosec // RADIUS response authenticator is MD5
			hash.Write(response[:4])
			hash.Write(buffer[4:20])
			hash.Write(attrs)
			hash.Write(secret)
			copy(response[4:20], hash.Sum(nil))
			if _, err := conn.WriteToUDP(response, address); err != nil {
				return err
			}
		}
	}
}

func tunnelRadiusCoA(ctx context.Context, args []string) error {
	port, err := tunnelArgPort(args, 0)
	if err != nil {
		return err
	}
	if len(args) < 2 {
		return errors.New("missing shared secret")
	}
	secret := []byte(args[1])
	attrs := append(tunnelRadiusAttr(44, []byte("no-such-session")), tunnelRadiusUint32Attr(55, uint32(time.Now().Unix()))...)
	attrs = append(attrs, tunnelRadiusAttr(80, make([]byte, 16))...)
	request := make([]byte, 20+len(attrs))
	request[0], request[1] = 40, 7
	binary.BigEndian.PutUint16(request[2:4], uint16(len(request)))
	copy(request[20:], attrs)
	hash := md5.New() //nolint:gosec // RFC 5176 request authenticator
	hash.Write(request[:4])
	hash.Write(make([]byte, 16))
	hash.Write(attrs)
	hash.Write(secret)
	copy(request[4:20], hash.Sum(nil))
	mac := hmac.New(md5.New, secret)
	mac.Write(request)
	copy(request[len(request)-16:], mac.Sum(nil))
	requestAuthenticator := append([]byte(nil), request[4:20]...)

	conn, target, err := tunnelL2TPDial(port)
	if err != nil {
		return err
	}
	defer conn.Close() //nolint:errcheck // fixture teardown
	response, _, err := tunnelL2TPExchange(ctx, conn, target, request, 40, 500*time.Millisecond)
	if err != nil {
		return fmt.Errorf("no answer from CoA listener on port %d", port)
	}
	if len(response) < 20 {
		return fmt.Errorf("response is %d octets, less than a RADIUS header", len(response))
	}
	length := int(binary.BigEndian.Uint16(response[2:4]))
	if length > len(response) {
		return fmt.Errorf("response length %d exceeds datagram", length)
	}
	verify := md5.New() //nolint:gosec // RFC 5176 response authenticator
	verify.Write(response[:4])
	verify.Write(requestAuthenticator)
	verify.Write(response[20:length])
	verify.Write(secret)
	if !bytes.Equal(response[4:20], verify.Sum(nil)) || response[0] != 42 || response[1] != 7 {
		return errors.New("Disconnect-NAK header or response authenticator did not verify")
	}
	var cause []byte
	for offset := 20; offset+2 <= length; {
		attributeLength := int(response[offset+1])
		if attributeLength < 2 || offset+attributeLength > length {
			break
		}
		if response[offset] == 101 {
			cause = response[offset+2 : offset+attributeLength]
			break
		}
		offset += attributeLength
	}
	if len(cause) != 4 || binary.BigEndian.Uint32(cause) != 503 {
		return errors.New("Disconnect-NAK missing Error-Cause 503")
	}
	fmt.Println("OK: Disconnect-NAK with Error-Cause 503 from the coa-port listener")
	return nil
}

func tunnelRadiusAccounting(ctx context.Context, args []string) error {
	port, err := tunnelArgPort(args, 0)
	if err != nil {
		return err
	}
	secret := []byte("testing123")
	if len(args) > 1 {
		secret = []byte(args[1])
	}
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: port})
	if err != nil {
		return err
	}
	defer conn.Close() //nolint:errcheck // fixture teardown
	fmt.Printf("radius-mock listening on 127.0.0.1:%d\n", port)
	buffer := make([]byte, 4096)
	for {
		_ = conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
		n, address, readErr := conn.ReadFromUDP(buffer)
		if readErr != nil {
			if ctx.Err() != nil {
				return nil
			}
			continue
		}
		if n < 20 || (buffer[0] != 1 && buffer[0] != 4) {
			continue
		}
		line := tunnelRadiusDescribe(buffer[:n])
		fmt.Println(line)
		file, openErr := os.OpenFile("radius-rx.log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
		if openErr != nil {
			return openErr
		}
		_, writeErr := fmt.Fprintln(file, line)
		closeErr := file.Close()
		if writeErr != nil || closeErr != nil {
			return errors.Join(writeErr, closeErr)
		}
		responseCode := byte(2)
		if buffer[0] == 4 {
			responseCode = 5
		}
		response := make([]byte, 20)
		response[0], response[1] = responseCode, buffer[1]
		binary.BigEndian.PutUint16(response[2:4], 20)
		hash := md5.New() //nolint:gosec // RADIUS response authenticator
		hash.Write(response[:4])
		hash.Write(buffer[4:20])
		hash.Write(secret)
		copy(response[4:20], hash.Sum(nil))
		_, _ = conn.WriteToUDP(response, address)
	}
}

func tunnelRadiusDescribe(packet []byte) string {
	name := "Access-Request"
	if packet[0] == 4 {
		name = "Accounting-Request"
	}
	fields := make([]string, 0)
	for offset := 20; offset+2 <= len(packet); {
		length := int(packet[offset+1])
		if length < 2 || offset+length > len(packet) {
			break
		}
		value := packet[offset+2 : offset+length]
		switch packet[offset] {
		case 1:
			fields = append(fields, "User-Name="+string(value))
		case 4:
			if len(value) == 4 {
				fields = append(fields, "NAS-IP-Address="+net.IP(value).String())
			}
		case 8:
			if len(value) == 4 {
				fields = append(fields, "Framed-IP-Address="+net.IP(value).String())
			}
		case 40:
			if len(value) == 4 {
				status := map[uint32]string{1: "Start", 2: "Stop", 3: "Interim-Update"}[binary.BigEndian.Uint32(value)]
				fields = append(fields, "Acct-Status-Type="+status)
			}
		case 44:
			fields = append(fields, "Acct-Session-Id="+string(value))
		case 87:
			fields = append(fields, "NAS-Port-Id="+string(value))
		}
		offset += length
	}
	return "RADIUS-RX " + name + " " + strings.Join(fields, " ")
}
