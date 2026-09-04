// Design: docs/architecture/testing/ci-format.md -- mock RADIUS server for AAA testing
// RFC: rfc/short/rfc2865.md -- Access-Request/Accept/Reject, User-Password (5.2)
//
// A minimal RFC 2865 RADIUS authentication server for functional tests. It
// answers Access-Request with Access-Accept for a configured user and
// Access-Reject otherwise, optionally attaching Filter-Id reply attributes so
// the admin backend's profile mapping can be exercised. It reuses the
// production radius wire package so the test exercises the real encode/decode
// path.
//
// It verifies either credential RFC 2865 Section 4.1 permits: a User-Password,
// decoded per Section 5.2, or a CHAP-Password, verified per Section 2.2 against
// the cleartext password it holds. A request carrying both is rejected, because
// Section 4.1 states that "An Access-Request MUST NOT contain both a
// User-Password and a CHAP-Password."

package radius

import (
	"crypto/md5" //nolint:gosec // RFC 2865 Section 5.2 mandates MD5 for User-Password
	"crypto/subtle"
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
	eap := newEAPServer()
	buf := make([]byte, radius.MaxPacketLen)
	for {
		n, from, err := conn.ReadFromUDP(buf)
		if err != nil {
			return 0
		}
		reply := handleRequest(buf[:n], keyBytes, users, eap, logAll)
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
func handleRequest(data, key []byte, users userList, eap *eapServer, logPackets bool) []byte {
	pkt, err := radius.Decode(data)
	if err != nil || pkt.Code != radius.CodeAccessRequest {
		return nil
	}

	// RFC 3579 Section 3.1 gives an EAP conversation its own shape: several
	// rounds, an Access-Challenge between them, and a Message-Authenticator on
	// every packet. eap.go answers it, and nothing below is reached for it.
	if pkt.FindAttr(radius.AttrEAPMessage) != nil {
		return eap.handle(data, pkt, key, users, logPackets)
	}

	name := string(pkt.FindAttr(radius.AttrUserName))
	userPassword := pkt.FindAttr(radius.AttrUserPassword)
	chapPassword := pkt.FindAttr(radius.AttrCHAPPassword)

	code := uint8(radius.CodeAccessReject)
	var reply []radius.Attr
	method := "pap"
	switch {
	case userPassword != nil && chapPassword != nil:
		// RFC 2865 Section 4.1: "An Access-Request MUST NOT contain both a
		// User-Password and a CHAP-Password." A request carrying both is
		// malformed, so it is rejected without reading either credential. This
		// is what makes the mock able to fail a client that APPENDS the CHAP
		// credential to the PAP one instead of selecting between them.
		method = "both"
	case chapPassword != nil:
		method = "chap"
		// RFC 2865 Section 5.3: "The CHAP challenge value is found in the
		// CHAP-Challenge Attribute (60) if present in the packet, otherwise in
		// the Request Authenticator field."
		challenge := pkt.FindAttr(radius.AttrCHAPChallenge)
		if challenge == nil {
			challenge = pkt.Authenticator[:]
		}
		for _, u := range users {
			if u.name == name && verifyCHAP(chapPassword, u.pass, challenge) {
				code = radius.CodeAccessAccept
				reply = profileAttrs(u)
				break
			}
		}
	default:
		pass := decodeUserPassword(userPassword, key, pkt.Authenticator)
		for _, u := range users {
			if u.name == name && u.pass == pass {
				code = radius.CodeAccessAccept
				reply = profileAttrs(u)
				break
			}
		}
	}
	if logPackets {
		slog.Info("radius-mock: Access-Request", "user", name, "method", method, "reply", codeName(code))
	}
	return buildResponse(code, pkt.Identifier, pkt.Authenticator, reply, key)
}

// profileAttrs returns the Filter-Id attributes an Access-Accept carries for a
// user, one per profile name.
func profileAttrs(u mockUser) []radius.Attr {
	reply := make([]radius.Attr, 0, len(u.profiles))
	for _, p := range u.profiles {
		reply = append(reply, radius.Attr{Type: radius.AttrFilterID, Value: []byte(p)})
	}
	return reply
}

// verifyCHAP performs the check RFC 2865 Section 2.2 gives the server: it
// "encrypts the challenge using MD5 on the CHAP ID octet, that password, and
// the CHAP challenge ... and compares that result to the CHAP-Password".
//
// The comparison is constant-time so the mock does not model a server that
// leaks the response one octet at a time.
func verifyCHAP(chapPassword []byte, password string, challenge []byte) bool {
	// RFC 2865 Section 5.3: Length 19, so the value is one CHAP Ident octet
	// followed by a 16-octet String.
	if len(chapPassword) != 17 {
		return false
	}
	h := md5.New() //nolint:gosec // RFC 2865 Section 2.2 mandates MD5
	h.Write(chapPassword[:1])
	h.Write([]byte(password))
	h.Write(challenge)
	return subtle.ConstantTimeCompare(h.Sum(nil), chapPassword[1:]) == 1
}

func codeName(code uint8) string {
	switch code {
	case radius.CodeAccessAccept:
		return "Access-Accept"
	case radius.CodeAccessChallenge:
		return "Access-Challenge"
	default:
		return "Access-Reject"
	}
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
