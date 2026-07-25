// Design: docs/architecture/testing/ci-format.md -- mock RADIUS server for AAA testing
// RFC: rfc/short/rfc2865.md -- Access-Request/Accept/Reject, User-Password (5.2)
//
// A minimal RFC 2865 RADIUS authentication server for functional tests. It
// answers Access-Request with Access-Accept for a configured user (decoding the
// hidden User-Password per Section 5.2), optionally attaching Filter-Id reply
// attributes so the admin backend's profile mapping can be exercised, and
// Access-Reject otherwise. It reuses the production radius wire package so the
// test exercises the real encode/decode path.

package radius

import (
	"crypto/md5" //nolint:gosec // RFC 2865 Section 5.2 mandates MD5 for User-Password
	"encoding/binary"
	"flag"
	"log/slog"
	"net"
	"os"
	"strings"

	radius "github.com/ze-software/ze/internal/component/radius"
	"github.com/ze-software/ze/internal/core/textbuf"
)

type mockUser struct {
	name     string
	pass     string
	profiles []string // returned as Filter-Id reply attributes on Accept
}

type userList []mockUser

func (u *userList) String() string { return textbuf.IntStr(int64(len(*u)), " users") }

func (u *userList) Set(s string) error {
	// Format: name:pass[:profile1,profile2]
	parts := strings.SplitN(s, ":", 3)
	if len(parts) < 2 {
		var tb textbuf.Buffer
		return errStr(tb.Str("expected name:pass[:profiles] got ").Str(s).String())
	}
	user := mockUser{name: parts[0], pass: parts[1]}
	if len(parts) == 3 && parts[2] != "" {
		user.profiles = strings.Split(parts[2], ",")
	}
	*u = append(*u, user)
	return nil
}

type errStr string

func (e errStr) Error() string { return string(e) }

// Run is the `ze-test radius-mock` entry point.
func Run(args []string) int {
	var (
		port    int
		key     string
		users   userList
		addrOut string
		logAll  bool
	)

	fs := flag.NewFlagSet("ze-test radius-mock", flag.ExitOnError)
	fs.IntVar(&port, "port", 0, "UDP listen port (0 = auto)")
	fs.StringVar(&key, "key", "", "RADIUS shared secret (required)")
	fs.Var(&users, "user", "credential: name:pass[:profile1,profile2] (repeatable)")
	fs.StringVar(&addrOut, "addr-file", "", "write listening host:port to this file")
	fs.BoolVar(&logAll, "log-packets", true, "log every received packet")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if key == "" {
		slog.Error("radius-mock: --key is required")
		return 1
	}
	if len(users) == 0 {
		slog.Error("radius-mock: at least one --user is required")
		return 1
	}

	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: port})
	if err != nil {
		slog.Error("radius-mock: listen failed", "error", err)
		return 1
	}
	defer func() { _ = conn.Close() }()

	addr := conn.LocalAddr().String()
	slog.Info("radius-mock: listening", "addr", addr)
	if addrOut != "" {
		if err := os.WriteFile(addrOut, []byte(addr), 0o600); err != nil {
			slog.Error("radius-mock: write addr-file failed", "error", err)
			return 1
		}
	}

	keyBytes := []byte(key)
	buf := make([]byte, radius.MaxPacketLen)
	for {
		n, from, err := conn.ReadFromUDP(buf)
		if err != nil {
			return 0
		}
		reply := handleRequest(buf[:n], keyBytes, users, logAll)
		if reply == nil {
			continue
		}
		if _, err := conn.WriteToUDP(reply, from); err != nil {
			slog.Warn("radius-mock: write reply failed", "error", err)
		}
	}
}

// handleRequest decodes an Access-Request and returns the encoded response, or
// nil if the packet is not a decodable Access-Request.
func handleRequest(data, key []byte, users userList, logPackets bool) []byte {
	pkt, err := radius.Decode(data)
	if err != nil || pkt.Code != radius.CodeAccessRequest {
		return nil
	}

	name := string(pkt.FindAttr(radius.AttrUserName))
	pass := decodeUserPassword(pkt.FindAttr(radius.AttrUserPassword), key, pkt.Authenticator)

	code := uint8(radius.CodeAccessReject)
	var reply []radius.Attr
	for _, u := range users {
		if u.name == name && u.pass == pass {
			code = radius.CodeAccessAccept
			for _, p := range u.profiles {
				reply = append(reply, radius.Attr{Type: radius.AttrFilterID, Value: []byte(p)})
			}
			break
		}
	}
	if logPackets {
		slog.Info("radius-mock: Access-Request", "user", name, "reply", codeName(code))
	}
	return buildResponse(code, pkt.Identifier, pkt.Authenticator, reply, key)
}

func codeName(code uint8) string {
	if code == radius.CodeAccessAccept {
		return "Access-Accept"
	}
	return "Access-Reject"
}

// decodeUserPassword reverses the RFC 2865 Section 5.2 User-Password hiding.
func decodeUserPassword(enc, secret []byte, auth [radius.AuthenticatorLen]byte) string {
	if len(enc) == 0 || len(enc)%radius.AuthenticatorLen != 0 {
		return ""
	}
	out := make([]byte, len(enc))
	prev := auth[:]
	for off := 0; off < len(enc); off += radius.AuthenticatorLen {
		h := md5.New() //nolint:gosec // RFC 2865 mandates MD5
		h.Write(secret)
		h.Write(prev)
		block := h.Sum(nil)
		for i := range radius.AuthenticatorLen {
			out[off+i] = enc[off+i] ^ block[i]
		}
		prev = enc[off : off+radius.AuthenticatorLen]
	}
	return string(trimTrailingNUL(out))
}

func trimTrailingNUL(b []byte) []byte {
	end := len(b)
	for end > 0 && b[end-1] == 0 {
		end--
	}
	return b[:end]
}

// buildResponse encodes a response packet with a valid Response Authenticator
// (RFC 2865 Section 3) computed over the reply attributes.
func buildResponse(code, id uint8, reqAuth [radius.AuthenticatorLen]byte, attrs []radius.Attr, secret []byte) []byte {
	body := make([]byte, 0, 64)
	for _, a := range attrs {
		body = append(body, a.Type, byte(2+len(a.Value)))
		body = append(body, a.Value...)
	}
	total := radius.HeaderLen + len(body)
	resp := make([]byte, total)
	resp[0] = code
	resp[1] = id
	binary.BigEndian.PutUint16(resp[2:4], uint16(total))
	copy(resp[radius.HeaderLen:], body)
	auth := radius.ResponseAuthenticator(code, id, uint16(total), reqAuth, body, secret)
	copy(resp[4:4+radius.AuthenticatorLen], auth[:])
	return resp
}
