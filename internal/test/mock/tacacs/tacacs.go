// Design: docs/architecture/testing/ci-format.md -- mock TACACS+ server for AAA testing

package tacacs

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

	"github.com/ze-software/ze/internal/component/tacacs"
	"github.com/ze-software/ze/internal/core/textbuf"
)

type tacacsMockUser struct {
	name    string
	pass    string
	privLvl uint8
}

type tacacsUserList []tacacsMockUser

func (u *tacacsUserList) String() string { return textbuf.IntStr(int64(len(*u)), " users") }

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

var acctCounter atomic.Uint64

var connCounter atomic.Uint64

type stringSliceFlag []string

func (s *stringSliceFlag) String() string { return textbuf.Join(*s, ",") }

func (s *stringSliceFlag) Set(v string) error { *s = append(*s, v); return nil }

func Run(args []string) int {
	var (
		port        int
		key         string
		users       tacacsUserList
		addrOut     string
		logAll      bool
		authorDeny  stringSliceFlag
		authorAllow stringSliceFlag
	)

	fs := flag.NewFlagSet("le test tacacs-mock", flag.ExitOnError)
	fs.IntVar(&port, "port", 0, "TCP listen port (0 = auto)")
	fs.StringVar(&key, "key", "", "TACACS+ shared secret (required)")
	fs.Var(&users, "user", "credential: name:pass[:privlvl] (repeatable, priv-lvl default 15)")
	fs.StringVar(&addrOut, "addr-file", "", "write listening host:port to this file")
	fs.BoolVar(&logAll, "log-packets", true, "log every received packet to stderr")
	fs.Var(&authorDeny, "author-deny", "deny AUTHOR REQUEST when cmd contains this substring (repeatable)")
	fs.Var(&authorAllow, "author-allow", "allow an exact AUTHOR username=command pair without granting authentication (repeatable; deny rules still apply)")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: le test tacacs-mock [flags]\n\nMock TACACS+ server for AAA testing.\n\nFlags:\n")
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
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
	ln, err := lc.Listen(context.Background(), "tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: listen: %v\n", err)
		return 1
	}
	defer func() { _ = ln.Close() }()

	addr := ln.Addr().String()
	fmt.Fprintf(os.Stderr, "le test tacacs-mock: listening on %s\n", addr)
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
			return 0
		}
		n := connCounter.Add(1)
		if logAll {
			fmt.Fprintf(os.Stderr, "tacacs-mock: connection #%d from %s\n", n, conn.RemoteAddr())
		}
		go tacacsMockHandle(conn, keyBytes, users, authorDeny, authorAllow, logAll)
	}
}

func tacacsMockHandle(conn net.Conn, key []byte, users tacacsUserList, authorDeny, authorAllow []string, logPackets bool) {
	defer func() { _ = conn.Close() }()

	singleConnect := false
	for i := 0; ; i++ {
		hdrBuf := make([]byte, 12)
		if _, err := io.ReadFull(conn, hdrBuf); err != nil {
			return
		}
		hdr, err := tacacs.UnmarshalPacketHeader(hdrBuf)
		if err != nil {
			return
		}

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
		case 0x01:
			tacacsMockReplyAuthen(conn, hdr, body, key, users, replyFlags, logPackets)
		case 0x02:
			tacacsMockReplyAuthor(conn, hdr, body, key, users, authorDeny, authorAllow, replyFlags, logPackets)
		case 0x03:
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

	var status uint8 = 0x02
	for _, u := range users {
		if u.name == user && u.pass == data {
			status = 0x01
			break
		}
	}

	msg := "mock-reply"
	// PAP authenticates credentials; privilege is returned only by session
	// authorization (RFC 8907 Sections 5.4.2.2 and 9).
	reply := make([]byte, 6+len(msg))
	reply[0] = status
	reply[1] = 0x00
	binary.BigEndian.PutUint16(reply[2:4], uint16(len(msg)))
	copy(reply[6:], msg)

	tacacsMockSendReply(conn, hdr, reply, key, replyFlags)
	if logPackets {
		fmt.Fprintf(os.Stderr, "tacacs-mock: AUTHEN reply user=%q status=0x%02x\n", user, status)
	}
}

func tacacsMockReplyAuthor(conn net.Conn, hdr tacacs.PacketHeader, body, key []byte, users tacacsUserList, authorDeny, authorAllow []string, replyFlags uint8, logPackets bool) {
	user, cmd, valid := parseAuthorRequest(body)
	status := uint8(tacacs.AuthorStatusFail)
	statusName := "FAIL"
	var privilege string
	if valid {
		for _, configured := range users {
			if configured.name != user {
				continue
			}
			status = tacacs.AuthorStatusPassAdd
			statusName = "PASS_ADD"
			if cmd == "" {
				privilege = "priv-lvl=" + strconv.Itoa(int(configured.privLvl))
			}
			break
		}
		for _, allowed := range authorAllow {
			allowedUser, allowedCommand, ok := strings.Cut(allowed, "=")
			if !ok || allowedUser != user || allowedCommand != cmd {
				continue
			}
			status = tacacs.AuthorStatusPassAdd
			statusName = "PASS_ADD"
			break
		}
	}
	for _, deny := range authorDeny {
		if deny != "" && strings.Contains(cmd, deny) {
			status = tacacs.AuthorStatusFail
			statusName = "FAIL"
			privilege = ""
			break
		}
	}

	// RFC 8907 Section 6.2: status[0], arg_cnt[1], message/data lengths[2:6],
	// then AV lengths and AV bytes. Only shell session policy assigns a level.
	reply := []byte{status, 0x00, 0x00, 0x00, 0x00, 0x00}
	if privilege != "" {
		reply[1] = 1
		reply = append(reply, uint8(len(privilege)))
		reply = append(reply, privilege...)
	}
	tacacsMockSendReply(conn, hdr, reply, key, replyFlags)
	if logPackets {
		fmt.Fprintf(os.Stderr, "tacacs-mock: AUTHOR cmd=%q reply=%s user=%q privilege=%q\n",
			cmd, statusName, user, privilege)
	}
}

func parseAuthorRequest(body []byte) (string, string, bool) {
	if len(body) < 8 {
		return "", "", false
	}
	userLen := int(body[4])
	portLen := int(body[5])
	remLen := int(body[6])
	argCount := int(body[7])
	off := 8 + argCount
	if off+userLen+portLen+remLen > len(body) {
		return "", "", false
	}
	user := string(body[off : off+userLen])
	off += userLen + portLen + remLen
	var cmd string
	var cmdArgs []string
	serviceSeen, commandSeen := false, false
	for i := range argCount {
		argLen := int(body[8+i])
		if off+argLen > len(body) {
			return "", "", false
		}
		arg := string(body[off : off+argLen])
		off += argLen
		switch {
		case arg == "service=shell":
			serviceSeen = true
		case strings.HasPrefix(arg, "cmd="):
			cmd = arg[4:]
			commandSeen = true
		case strings.HasPrefix(arg, "cmd-arg="):
			cmdArgs = append(cmdArgs, arg[8:])
		default:
			return "", "", false
		}
	}
	if !serviceSeen || !commandSeen || off != len(body) {
		return "", "", false
	}
	if len(cmdArgs) == 0 {
		return user, cmd, true
	}
	if cmd == "" {
		return "", "", false
	}
	var tb textbuf.Buffer
	return user, tb.Str(cmd).Byte(' ').Join(cmdArgs, " ").String(), true
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

	reply := []byte{0x00, 0x00, 0x00, 0x00, 0x01}
	tacacsMockSendReply(conn, hdr, reply, key, replyFlags)
}

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
		return
	}
}
