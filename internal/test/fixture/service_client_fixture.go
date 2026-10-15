// Design: docs/functional-tests.md -- service clients that read what a daemon serves
// Related: register_service_client.go -- the registration of these clients

package fixture

import (
	"context"
	"crypto/tls"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	mdns "github.com/miekg/dns"
)

// A service client asks the daemon for one thing until the answer carries the
// text the .ci expects, then prints `OK: <what> served <text> from <server>`.
// The .ci awaits that line, so a reload test proves the service answers with
// the NEW config, which a log line written beside the install cannot prove.
//
// The client polls because it starts beside the reload trigger: it cannot know
// when the reload lands, only that the answer must change before the bound.
// A daemon that keeps serving the old config never matches, and the client
// fails with the last answer it read.
const (
	// serviceClientAttempts times serviceClientDelay bounds the wait at 15s,
	// inside the 20s daemon timeout every reload .ci sets.
	serviceClientAttempts = 75
	serviceClientDelay    = 200 * time.Millisecond

	// serviceClientExchangeTimeout bounds one request: a datagram, a TFTP
	// block, or an HTTP round trip.
	serviceClientExchangeTimeout = time.Second

	// serviceClientBodyOctetsMax bounds what one answer may carry. The files a
	// .ci serves are a line of text.
	serviceClientBodyOctetsMax = 64 << 10
)

// serviceClientAwait calls fetch until its answer contains want, or the
// attempts run out. It answers an error naming the last failure or the last
// answer read, never a success the server did not give.
func serviceClientAwait(ctx context.Context, what, server, want string, fetch func(context.Context) (string, error)) error {
	var answer string
	var fetchErr error
	served := Poll(ctx, serviceClientAttempts, serviceClientDelay, func() bool {
		answer, fetchErr = fetch(ctx)
		if fetchErr != nil {
			return false
		}
		return strings.Contains(answer, want)
	})
	if served {
		fmt.Fprintln(os.Stderr, "OK:", what, "served", want, "from", server)
		return nil
	}
	if fetchErr != nil {
		return fmt.Errorf("%s from %s never served %q; the last attempt failed: %w", what, server, want, fetchErr)
	}
	return fmt.Errorf("%s from %s never served %q; the last answer was %q", what, server, want, answer)
}

// dnsOverTLSScheme prefixes a server the client reaches over DNS over TLS
// (RFC 7858) instead of UDP.
const dnsOverTLSScheme = "tls://"

// serviceClientDNSQuery runs `client/dns-query [tls://]<address:port> <name>
// <type> <text>`: it sends one query per attempt and matches <text> against
// the rcode and the answer section, one record per line in presentation form.
// A bare address is asked over UDP. A tls:// address is asked over DNS over
// TLS, for a server that answers on an unprivileged port only there.
func serviceClientDNSQuery(ctx context.Context, args []string) error {
	if len(args) != 4 {
		return errors.New("client/dns-query needs four arguments: [tls://]<address:port> <name> <type> <expected text>")
	}
	server, name, typeName, want := args[0], mdns.Fqdn(args[1]), strings.ToUpper(args[2]), args[3]
	client := &mdns.Client{Net: "udp", Timeout: serviceClientExchangeTimeout}
	address, overTLS := strings.CutPrefix(server, dnsOverTLSScheme)
	if overTLS {
		// The server under test presents the ephemeral self-signed certificate
		// Ze generates when no cert-file is configured, so no chain can verify.
		// The client proves what the server answers, never who it is.
		client.Net = "tcp-tls"
		client.TLSConfig = &tls.Config{MinVersion: tls.VersionTLS12, InsecureSkipVerify: true} //nolint:gosec // self-signed test server; see above
	}
	if _, _, err := net.SplitHostPort(address); err != nil {
		return fmt.Errorf("client/dns-query: %q is not an address:port: %w", server, err)
	}
	qtype, known := mdns.StringToType[typeName]
	if !known {
		return fmt.Errorf("client/dns-query: %q is not a DNS record type; use one such as A, TXT or SOA", args[2])
	}
	return serviceClientAwait(ctx, "dns "+typeName+" "+name, server, want, func(ctx context.Context) (string, error) {
		query := new(mdns.Msg)
		query.SetQuestion(name, qtype)
		reply, _, err := client.ExchangeContext(ctx, query, address)
		if err != nil {
			return "", err
		}
		return dnsReplyText(reply), nil
	})
}

// dnsReplyText answers the rcode on the first line, then each answer record.
func dnsReplyText(reply *mdns.Msg) string {
	var text strings.Builder
	text.WriteString(mdns.RcodeToString[reply.Rcode])
	for _, record := range reply.Answer {
		text.WriteByte('\n')
		text.WriteString(record.String())
	}
	return text.String()
}

// serviceClientHTTPGet runs `client/http-get <url> <text>`: it fetches <url>
// once per attempt and matches <text> against a 200 answer's body. Any other
// status is a failed attempt.
func serviceClientHTTPGet(ctx context.Context, args []string) error {
	if len(args) != 2 {
		return errors.New("client/http-get needs two arguments: <url> <expected text>")
	}
	target, want := args[0], args[1]
	if _, err := url.ParseRequestURI(target); err != nil {
		return fmt.Errorf("client/http-get: %q is not a URL: %w", target, err)
	}
	client := &http.Client{Timeout: serviceClientExchangeTimeout}
	return serviceClientAwait(ctx, "http GET", target, want, func(ctx context.Context) (string, error) {
		return httpGetBody(ctx, client, target)
	})
}

// httpGetBody answers the body of a 200 answer to GET target.
func httpGetBody(ctx context.Context, client *http.Client, target string) (string, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, target, http.NoBody)
	if err != nil {
		return "", err
	}
	response, err := client.Do(request)
	if err != nil {
		return "", err
	}

	defer response.Body.Close() //nolint:errcheck // the body was read; a close failure changes no answer

	body, err := io.ReadAll(io.LimitReader(response.Body, serviceClientBodyOctetsMax))
	if err != nil {
		return "", fmt.Errorf("read body: %w", err)
	}
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("status %s, body %q", response.Status, body)
	}
	return string(body), nil
}

// TFTP opcodes and sizes, RFC 1350 Section 5.
const (
	tftpOpcodeRRQ   = 1
	tftpOpcodeDATA  = 3
	tftpOpcodeACK   = 4
	tftpOpcodeERROR = 5

	tftpHeaderOctets = 4
	tftpBlockOctets  = 512
	tftpBlocksMax    = serviceClientBodyOctetsMax / tftpBlockOctets
)

// serviceClientTFTPGet runs `client/tftp-get <address:port> <file> <text>`: it
// reads <file> in octet mode once per attempt and matches <text> against the
// content. An ERROR packet, such as file not found, is a failed attempt.
func serviceClientTFTPGet(ctx context.Context, args []string) error {
	if len(args) != 3 {
		return errors.New("client/tftp-get needs three arguments: <address:port> <file> <expected text>")
	}
	server, file, want := args[0], args[1], args[2]
	address, err := net.ResolveUDPAddr("udp", server)
	if err != nil {
		return fmt.Errorf("client/tftp-get: %q is not an address:port: %w", server, err)
	}
	return serviceClientAwait(ctx, "tftp RRQ "+file, server, want, func(context.Context) (string, error) {
		return tftpRead(address, file)
	})
}

// tftpRead transfers file from server and answers its content.
//
//	RRQ   | 01 | filename | 0 | "octet" | 0 |
//	DATA  | 03 | block (2 octets) | 0..512 octets of data |
//	ACK   | 04 | block (2 octets) |
//	ERROR | 05 | error code (2 octets) | message | 0 |
//
// The server answers from a new port, its transfer identifier, so every ACK
// goes to the address the DATA came from.
func tftpRead(server *net.UDPAddr, file string) (string, error) {
	conn, err := net.ListenUDP("udp", nil)
	if err != nil {
		return "", fmt.Errorf("open client socket: %w", err)
	}

	defer conn.Close() //nolint:errcheck // a client socket; the transfer already answered

	// RFC 1350 Section 5: "The file name is a sequence of bytes in netascii
	// terminated by a zero byte."
	request := make([]byte, 0, 2+len(file)+1+len("octet")+1)
	request = binary.BigEndian.AppendUint16(request, tftpOpcodeRRQ)
	request = append(request, file...)
	request = append(request, 0)
	request = append(request, "octet"...)
	request = append(request, 0)
	if _, err := conn.WriteToUDP(request, server); err != nil {
		return "", fmt.Errorf("send RRQ: %w", err)
	}

	var content []byte
	packet := make([]byte, tftpHeaderOctets+tftpBlockOctets)
	ack := make([]byte, tftpHeaderOctets)
	for block := uint16(1); int(block) <= tftpBlocksMax; block++ {
		if err := conn.SetReadDeadline(time.Now().Add(serviceClientExchangeTimeout)); err != nil {
			return "", fmt.Errorf("set read deadline: %w", err)
		}
		octets, peer, err := conn.ReadFromUDP(packet)
		if err != nil {
			return "", fmt.Errorf("await block %d: %w", block, err)
		}
		data, err := tftpDataBlock(packet[:octets], block)
		if err != nil {
			return "", err
		}
		content = append(content, data...)

		// RFC 1350 Section 5: "The block number in an ACK echoes the block
		// number of the DATA packet being acknowledged."
		binary.BigEndian.PutUint16(ack, tftpOpcodeACK)
		binary.BigEndian.PutUint16(ack[2:], block)
		if _, err := conn.WriteToUDP(ack, peer); err != nil {
			return "", fmt.Errorf("send ACK %d: %w", block, err)
		}
		// RFC 1350 Section 2: "A data packet of less than 512 bytes signals
		// termination of a transfer."
		if len(data) < tftpBlockOctets {
			return string(content), nil
		}
	}
	return "", fmt.Errorf("%s is longer than %d blocks", file, tftpBlocksMax)
}

// tftpDataBlock answers the data of packet, which MUST be DATA for block.
func tftpDataBlock(packet []byte, block uint16) ([]byte, error) {
	if len(packet) < tftpHeaderOctets {
		return nil, fmt.Errorf("a %d-octet packet is shorter than a TFTP header", len(packet))
	}
	opcode := binary.BigEndian.Uint16(packet)
	if opcode == tftpOpcodeERROR {
		return nil, fmt.Errorf("server answered ERROR %d: %s",
			binary.BigEndian.Uint16(packet[2:]), strings.TrimRight(string(packet[tftpHeaderOctets:]), "\x00"))
	}
	if opcode != tftpOpcodeDATA {
		return nil, fmt.Errorf("server answered opcode %d where DATA was due", opcode)
	}
	// RFC 1350 Section 5: "The block numbers on data packets begin with one
	// and increase by one for each new block of data."
	if got := binary.BigEndian.Uint16(packet[2:]); got != block {
		return nil, fmt.Errorf("server sent block %d where block %d was due", got, block)
	}
	return packet[tftpHeaderOctets:], nil
}
