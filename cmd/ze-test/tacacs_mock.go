// Design: docs/architecture/testing/ci-format.md -- mock TACACS+ server for AAA testing
//
// ze-test tacacs-mock is a lightweight TACACS+ (RFC 8907) server for functional tests.
// It listens on TCP, accepts one session per connection, and replies based on the
// configured credentials. AUTHEN, AUTHOR, and ACCT packet types are supported.
//
// Usage:
//
//	ze-test tacacs-mock --port 0 --key secret --user admin:testpass:15 --user guest:gpass:1

package main

import (
	"context"
	"encoding/binary"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"sync/atomic"

	"codeberg.org/thomas-mangin/ze/internal/component/tacacs"
)

// tacacsMockUser describes an accepted credential with its privilege level.
type tacacsMockUser struct {
	name    string
	pass    string
	privLvl uint8
}

type tacacsUserList []tacacsMockUser

func (u *tacacsUserList) String() string { return fmt.Sprintf("%d users", len(*u)) }

func (u *tacacsUserList) Set(s string) error {
	parts := strings.Split(s, ":")
	if len(parts) < 2 || len(parts) > 3 {
		return fmt.Errorf("expected name:pass[:privlvl] got %q", s)
	}
	user := tacacsMockUser{name: parts[0], pass: parts[1], privLvl: 15}
	if len(parts) == 3 {
		n, err := strconv.ParseUint(parts[2], 10, 8)
		if err != nil || n > 15 {
			return fmt.Errorf("invalid priv-lvl %q: must be 0-15", parts[2])
		}
		user.privLvl = uint8(n) //nolint:gosec // bounded above
	}
	*u = append(*u, user)
	return nil
}

// acctCounter is incremented for every ACCT REQUEST received so tests can
// observe accounting traffic via an atomic snapshot in the log.
var acctCounter atomic.Uint64

// connCounter is incremented for every accepted TCP connection so tests can
// verify single-connect mode keeps the connection count low across many
// sessions.
var connCounter atomic.Uint64

// stringSliceFlag captures repeatable string flags such as --author-deny.
type stringSliceFlag []string

func (s *stringSliceFlag) String() string { return strings.Join(*s, ",") }

func (s *stringSliceFlag) Set(v string) error { *s = append(*s, v); return nil }

var _ = register("tacacs-mock", "Mock TACACS+ server (RFC 8907) for AAA testing", tacacsMockCmd)

func tacacsMockCmd() int {
	var (
		port       int
		key        string
		users      tacacsUserList
		addrOut    string
		logAll     bool
		authorDeny stringSliceFlag
	)

	fs := flag.NewFlagSet("ze-test tacacs-mock", flag.ExitOnError)
	fs.IntVar(&port, "port", 0, "TCP listen port (0 = auto)")
	fs.StringVar(&key, "key", "", "TACACS+ shared secret (required)")
	fs.Var(&users, "user", "credential: name:pass[:privlvl] (repeatable, priv-lvl default 15)")
	fs.StringVar(&addrOut, "addr-file", "", "write listening host:port to this file")
	fs.BoolVar(&logAll, "log-packets", true, "log every received packet to stderr")
	fs.Var(&authorDeny, "author-deny", "deny AUTHOR REQUEST when cmd contains this substring (repeatable)")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: ze-test tacacs-mock [flags]\n\nMock TACACS+ server for AAA testing.\n\nFlags:\n")
		fs.PrintDefaults()
	}

	if err := fs.Parse(os.Args[1:]); err != nil {
		return 1
	}
	if key == "" {
		fmt.Fprintf(os.Stderr, "error: --key is required\n")
		return 1
	}
	if len(users) == 0 {
		fmt.Fprintf(os.Stderr, "error: at least one --user is required\n")
		return 1
	}

	lc := &net.ListenConfig{}
	ln, err := lc.Listen(context.Background(), "tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: listen: %v\n", err)
		return 1
	}
	defer func() { _ = ln.Close() }()

	addr := ln.Addr().String()
	fmt.Fprintf(os.Stderr, "ze-test tacacs-mock: listening on %s\n", addr)
	if addrOut != "" {
		if err := os.WriteFile(addrOut, []byte(addr), 0o600); err != nil {
			fmt.Fprintf(os.Stderr, "error: write addr-file: %v\n", err)
			return 1
		}
	}

	keyBytes := []byte(key)
	for {
		conn, err := ln.Accept()
		if err != nil {
			return 0 // listener closed
		}
		n := connCounter.Add(1)
		if logAll {
			fmt.Fprintf(os.Stderr, "tacacs-mock: connection #%d from %s\n", n, conn.RemoteAddr())
		}
		go tacacsMockHandle(conn, keyBytes, users, authorDeny, logAll)
	}
}

// tacacsMockHandle reads packets from the connection and replies per type.
//
// If the first packet has FlagSingleConnect (0x04) set, the mock echoes it
// on the reply and keeps the connection open for subsequent sessions. If
// the flag is absent, the mock closes the connection after one exchange
// (the historical per-session-TCP behavior).
func tacacsMockHandle(conn net.Conn, key []byte, users tacacsUserList, authorDeny []string, logPackets bool) {
	defer func() { _ = conn.Close() }()

	singleConnect := false
	for i := 0; ; i++ {
		hdrBuf := make([]byte, 12)
		if _, err := io.ReadFull(conn, hdrBuf); err != nil {
			return // client closed, or read error
		}
		hdr, err := tacacs.UnmarshalPacketHeader(hdrBuf)
		if err != nil {
			return
		}

		// Cap at the same 65535 ceiling the production client enforces
		// (packet.go::maxBodyLen). Without this, a rogue client could
		// advertise a 4 GB body in the header and OOM the mock.
		if hdr.Length > 65535 {
			fmt.Fprintf(os.Stderr, "tacacs-mock: rejecting oversized body length %d\n", hdr.Length)
			return
		}
		body := make([]byte, hdr.Length)
		if _, err := io.ReadFull(conn, body); err != nil {
			return
		}
		tacacs.Encrypt(body, hdr.SessionID, key, hdr.Version, hdr.SeqNo)

		replyFlags := uint8(0)
		if i == 0 && hdr.Flags&tacacs.FlagSingleConnect != 0 {
			singleConnect = true
			replyFlags |= tacacs.FlagSingleConnect
			if logPackets {
				fmt.Fprintf(os.Stderr, "tacacs-mock: single-connect accepted on %s\n", conn.RemoteAddr())
			}
		}

		switch hdr.Type {
		case 0x01: // AUTHEN
			tacacsMockReplyAuthen(conn, hdr, body, key, users, replyFlags, logPackets)
		case 0x02: // AUTHOR
			tacacsMockReplyAuthor(conn, hdr, body, key, authorDeny, replyFlags, logPackets)
		case 0x03: // ACCT
			tacacsMockReplyAcct(conn, hdr, body, key, replyFlags, logPackets)
		default:
			fmt.Fprintf(os.Stderr, "tacacs-mock: unknown packet type 0x%02x\n", hdr.Type)
			return
		}

		if !singleConnect {
			return
		}
	}
}

// parseAuthenStart decodes the fields this mock cares about: user + data.
// Returns empty strings if the body is malformed.
func parseAuthenStart(body []byte) (user, data string) {
	if len(body) < 8 {
		return "", ""
	}
	userLen := int(body[4])
	portLen := int(body[5])
	remLen := int(body[6])
	dataLen := int(body[7])
	off := 8
	if off+userLen+portLen+remLen+dataLen > len(body) {
		return "", ""
	}
	user = string(body[off : off+userLen])
	off += userLen + portLen + remLen
	data = string(body[off : off+dataLen])
	return user, data
}

func tacacsMockReplyAuthen(conn net.Conn, hdr tacacs.PacketHeader, body, key []byte, users tacacsUserList, replyFlags uint8, logPackets bool) {
	user, data := parseAuthenStart(body)
	if logPackets {
		fmt.Fprintf(os.Stderr, "tacacs-mock: AUTHEN user=%q data-len=%d\n", user, len(data))
	}

	var status uint8 = 0x02 // FAIL
	var privLvl uint8
	for _, u := range users {
		if u.name == user && u.pass == data {
			status = 0x01 // PASS
			privLvl = u.privLvl
			break
		}
	}

	// AUTHEN REPLY body: status(1) flags(1) server-msg-len(2) data-len(2)
	// data contains one byte: priv-lvl (only for PASS).
	msg := "mock-reply"
	var dataField []byte
	if status == 0x01 {
		dataField = []byte{privLvl}
	}
	reply := make([]byte, 6+len(msg)+len(dataField))
	reply[0] = status
	reply[1] = 0x00
	binary.BigEndian.PutUint16(reply[2:4], uint16(len(msg)))
	binary.BigEndian.PutUint16(reply[4:6], uint16(len(dataField)))
	copy(reply[6:], msg)
	copy(reply[6+len(msg):], dataField)

	tacacsMockSendReply(conn, hdr, reply, key, replyFlags)
	if logPackets {
		fmt.Fprintf(os.Stderr, "tacacs-mock: AUTHEN reply user=%q status=0x%02x priv-lvl=%d\n", user, status, privLvl)
	}
}

func tacacsMockReplyAuthor(conn net.Conn, hdr tacacs.PacketHeader, body, key []byte, authorDeny []string, replyFlags uint8, logPackets bool) {
	cmd := parseAuthorCmd(body)
	status := uint8(0x01) // PASS_ADD
	statusName := "PASS_ADD"
	for _, deny := range authorDeny {
		if deny != "" && strings.Contains(cmd, deny) {
			status = 0x10 // FAIL
			statusName = "FAIL"
			break
		}
	}

	// Body: status(1) arg-count(1) server-msg-len(2) data-len(2)
	reply := []byte{status, 0x00, 0x00, 0x00, 0x00, 0x00}
	tacacsMockSendReply(conn, hdr, reply, key, replyFlags)
	if logPackets {
		fmt.Fprintf(os.Stderr, "tacacs-mock: AUTHOR cmd=%q reply=%s\n", cmd, statusName)
	}
}

// parseAuthorCmd extracts the value of the cmd= arg from an AUTHOR REQUEST
// body. Returns "" if the body is malformed or no cmd arg is present. Mirrors
// `tacacs.AuthorRequest.MarshalBinary` layout (RFC 8907 §6.1).
func parseAuthorCmd(body []byte) string {
	if len(body) < 8 {
		return ""
	}
	userLen := int(body[4])
	portLen := int(body[5])
	remLen := int(body[6])
	argCount := int(body[7])
	if len(body) < 8+argCount {
		return ""
	}
	argLens := make([]int, argCount)
	for i := range argCount {
		argLens[i] = int(body[8+i])
	}
	off := 8 + argCount + userLen + portLen + remLen
	for i, alen := range argLens {
		if off+alen > len(body) {
			return ""
		}
		arg := string(body[off : off+alen])
		off += alen
		if v, ok := strings.CutPrefix(arg, "cmd="); ok {
			return v
		}
		_ = i
	}
	return ""
}

func tacacsMockReplyAcct(conn net.Conn, hdr tacacs.PacketHeader, body, key []byte, replyFlags uint8, logPackets bool) {
	n := acctCounter.Add(1)
	var flags uint8
	if len(body) > 0 {
		flags = body[0]
	}
	kind := "OTHER"
	switch {
	case flags&0x02 != 0:
		kind = "START"
	case flags&0x04 != 0:
		kind = "STOP"
	case flags&0x08 != 0:
		kind = "WATCHDOG"
	}
	if logPackets {
		fmt.Fprintf(os.Stderr, "tacacs-mock: ACCT %s seq=%d total=%d\n", kind, hdr.SeqNo, n)
	}

	// ACCT REPLY body: server-msg-len(2) data-len(2) status(1)
	reply := []byte{0x00, 0x00, 0x00, 0x00, 0x01} // status SUCCESS
	tacacsMockSendReply(conn, hdr, reply, key, replyFlags)
}

// tacacsMockSendReply encrypts and writes a reply packet. SeqNo increments
// from the client's value per RFC 8907 Section 4.1 (client odd, server even).
// replyFlags is ORed into the header flags so single-connect is echoed.
func tacacsMockSendReply(conn net.Conn, hdr tacacs.PacketHeader, body, key []byte, replyFlags uint8) {
	replyHdr := tacacs.PacketHeader{
		Version:   hdr.Version,
		Type:      hdr.Type,
		SeqNo:     hdr.SeqNo + 1,
		Flags:     replyFlags,
		SessionID: hdr.SessionID,
		Length:    uint32(len(body)),
	}
	wire := replyHdr.MarshalBinary()
	encrypted := make([]byte, len(body))
	copy(encrypted, body)
	tacacs.Encrypt(encrypted, replyHdr.SessionID, key, replyHdr.Version, replyHdr.SeqNo)
	wire = append(wire, encrypted...)
	if _, err := conn.Write(wire); err != nil {
		return // best effort
	}
}
