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

func tacacsMockCmd() int {
	var (
		port    int
		key     string
		users   tacacsUserList
		addrOut string
		logAll  bool
	)

	fs := flag.NewFlagSet("ze-test tacacs-mock", flag.ExitOnError)
	fs.IntVar(&port, "port", 0, "TCP listen port (0 = auto)")
	fs.StringVar(&key, "key", "", "TACACS+ shared secret (required)")
	fs.Var(&users, "user", "credential: name:pass[:privlvl] (repeatable, priv-lvl default 15)")
	fs.StringVar(&addrOut, "addr-file", "", "write listening host:port to this file")
	fs.BoolVar(&logAll, "log-packets", true, "log every received packet to stderr")

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
		go tacacsMockHandle(conn, keyBytes, users, logAll)
	}
}

// tacacsMockHandle reads one packet from the connection, replies per type, and
// closes. TACACS+ single-connect is not negotiated; every session is its own
// TCP connection in this mock.
func tacacsMockHandle(conn net.Conn, key []byte, users tacacsUserList, logPackets bool) {
	defer func() { _ = conn.Close() }()

	hdrBuf := make([]byte, 12)
	if _, err := io.ReadFull(conn, hdrBuf); err != nil {
		return
	}
	hdr, err := tacacs.UnmarshalPacketHeader(hdrBuf)
	if err != nil {
		return
	}

	body := make([]byte, hdr.Length)
	if _, err := io.ReadFull(conn, body); err != nil {
		return
	}
	tacacs.Encrypt(body, hdr.SessionID, key, hdr.Version, hdr.SeqNo)

	switch hdr.Type {
	case 0x01: // AUTHEN
		tacacsMockReplyAuthen(conn, hdr, body, key, users, logPackets)
	case 0x02: // AUTHOR
		tacacsMockReplyAuthor(conn, hdr, key, logPackets)
	case 0x03: // ACCT
		tacacsMockReplyAcct(conn, hdr, body, key, logPackets)
	default:
		fmt.Fprintf(os.Stderr, "tacacs-mock: unknown packet type 0x%02x\n", hdr.Type)
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

func tacacsMockReplyAuthen(conn net.Conn, hdr tacacs.PacketHeader, body, key []byte, users tacacsUserList, logPackets bool) {
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

	tacacsMockSendReply(conn, hdr, reply, key)
	if logPackets {
		fmt.Fprintf(os.Stderr, "tacacs-mock: AUTHEN reply user=%q status=0x%02x priv-lvl=%d\n", user, status, privLvl)
	}
}

func tacacsMockReplyAuthor(conn net.Conn, hdr tacacs.PacketHeader, key []byte, logPackets bool) {
	// Always reply PASS_ADD. Body: status(1) arg-count(1) server-msg-len(2) data-len(2)
	reply := []byte{0x01, 0x00, 0x00, 0x00, 0x00, 0x00}
	tacacsMockSendReply(conn, hdr, reply, key)
	if logPackets {
		fmt.Fprintf(os.Stderr, "tacacs-mock: AUTHOR reply status=PASS_ADD\n")
	}
}

func tacacsMockReplyAcct(conn net.Conn, hdr tacacs.PacketHeader, body, key []byte, logPackets bool) {
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
	tacacsMockSendReply(conn, hdr, reply, key)
}

// tacacsMockSendReply encrypts and writes a reply packet. SeqNo increments
// from the client's value per RFC 8907 Section 4.1 (client odd, server even).
func tacacsMockSendReply(conn net.Conn, hdr tacacs.PacketHeader, body, key []byte) {
	replyHdr := tacacs.PacketHeader{
		Version:   hdr.Version,
		Type:      hdr.Type,
		SeqNo:     hdr.SeqNo + 1,
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
