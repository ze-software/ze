// Design: docs/architecture/testing/ci-format.md -- mock RADIUS server for AAA testing
// Overview: radius.go -- the mock server whose Access-Request branch this extends
// Related: docs/guide/radius.md -- the auth-method values this branch answers
// RFC: rfc/short/rfc3579.md -- EAP-Message (3.1), Message-Authenticator (3.2)
// RFC: rfc/short/rfc2865.md -- Section 5.24 State
//
// The EAP half of the mock RADIUS server. It answers an Access-Request bearing
// EAP-Message with a real Access-Challenge: its own EAP-Message run, its own
// State, and a Message-Authenticator computed here from the shared secret.
//
// Two decisions decide what a green functional test proves, so both are stated
// rather than left for a reader to infer.
//
// The Message-Authenticator is computed and verified in this file, with
// crypto/hmac and crypto/md5, and never by calling ze's SignMessageAuthenticator
// or its verifiers. A server that signed with ze's signer would agree with ze
// whatever either of them wrote, and the signature would be unproven.
//
// The MS-CHAPv2 arithmetic is the opposite trade: eap.GenerateNTResponse and
// eap.GenerateAuthenticatorResponse are the same functions ze's peer runs, so
// this file cannot catch a defect in them. Hand-copying MD4, DES and the
// RFC 2759 hash chain into a fixture would prove the copy rather than the
// product. The proof against arithmetic ze did not write is the interop
// scenario radius-admin-eap-freeradius (internal/le/interoplab/radius).
package radius

import (
	"crypto/hmac"
	"crypto/md5" //nolint:gosec // RFC 3579 Section 3.2 mandates HMAC-MD5 for the Message-Authenticator
	"crypto/rand"
	"crypto/subtle"
	"encoding/binary"
	"encoding/hex"
	"log/slog"
	"strings"

	radius "github.com/ze-software/ze/internal/component/radius"
	"github.com/ze-software/ze/internal/core/eap"
	"github.com/ze-software/ze/internal/core/textbuf"
)

const (
	// The EAP Codes. RFC 3748 Section 4: "1 Request, 2 Response, 3 Success,
	// 4 Failure".
	eapCodeRequest  = 1
	eapCodeResponse = 2
	eapCodeSuccess  = 3
	eapCodeFailure  = 4

	// eapHeaderLen is Code(1) + Identifier(1) + Length(2). RFC 3748 Section 4.2
	// gives a Success and a Failure exactly that and nothing more.
	eapHeaderLen = 4

	// maxEAPMessageValue is the largest String an EAP-Message attribute carries.
	// RFC 2865 Section 5 gives every attribute a one-octet Length counting its
	// own two header octets, so 255 minus 2.
	maxEAPMessageValue = radius.MaxAttrLen - 2

	// The MS-CHAPv2 OpCodes, RFC 2759 Section 2.
	mschapv2OpChallenge = 1
	mschapv2OpResponse  = 2
	mschapv2OpSuccess   = 3

	// mschapv2ChallengeLen is the Value-Size of a Challenge packet, and
	// mschapv2ResponseLen the Value-Size of a Response. RFC 2759 Sections 3 and 4
	// fix both.
	mschapv2ChallengeLen = 16
	mschapv2ResponseLen  = 49

	// mschapv2ResponseFloor is the shortest MS-CHAPv2 Response: OpCode(1) +
	// MS-ID(1) + MS-Length(2) + Value-Size(1) + Response(49), before the Name.
	mschapv2ResponseFloor = 5 + mschapv2ResponseLen
)

// eapStage names what the server is waiting for on a conversation it challenged.
type eapStage uint8

const (
	// stageUnspecified is the zero value, so a session that reached the map
	// without a stage can never be read as one waiting for a Response.
	stageUnspecified eapStage = iota
	stageResponse
	stageSuccessAck
)

// eapSession is one EAP conversation in flight, held between the challenge the
// server sent and the answer it expects.
type eapSession struct {
	user          mockUser
	peerName      string
	authChallenge [mschapv2ChallengeLen]byte
	peerChallenge [mschapv2ChallengeLen]byte
	ntResponse    [24]byte
	msID          uint8
	stage         eapStage
}

// eapServer holds every EAP conversation in flight, keyed by the State value it
// issued for that conversation.
//
// Not safe for concurrent use. Run reads one datagram at a time on one
// goroutine, and this server is reached from nowhere else.
type eapServer struct {
	sessions map[string]*eapSession
	issued   int64
}

func newEAPServer() *eapServer {
	return &eapServer{sessions: make(map[string]*eapSession)}
}

// eapReply is what one round answers with, before it becomes RADIUS attributes.
// The state field is empty on a reply that ends the conversation, which is what
// keeps a State attribute off an Access-Accept and an Access-Reject.
type eapReply struct {
	code  uint8
	eap   []byte
	state string
	extra []radius.Attr
}

// eapPacket is a decoded EAP packet. A Success and a Failure carry no Type, so
// eapType is zero for both and no caller reads it for them.
type eapPacket struct {
	code       uint8
	identifier uint8
	eapType    uint8
	data       []byte
}

// handle answers one Access-Request carrying EAP-Message, or returns nil for a
// packet the server discards.
//
// RFC 3579 Section 3.1: "A NAS supporting the EAP-Message attribute MUST
// calculate the correct value of the Message-Authenticator and MUST silently
// discard the packet if it does not match the value sent." The discard is the
// nil return: the client's request stays outstanding and it retransmits, which
// is what tells a broken signer apart from a refused login.
func (s *eapServer) handle(data []byte, pkt *radius.Packet, key []byte, users userList, logPackets bool) []byte {
	name := string(pkt.FindAttr(radius.AttrUserName))
	if !verifyRequestSignature(data, key) {
		slog.Warn("radius-mock: EAP Access-Request discarded", "user", name, "reason", "message-authenticator does not verify")
		return nil
	}
	encoded, err := concatenateEAPMessage(pkt)
	if err != nil {
		slog.Warn("radius-mock: EAP Access-Request discarded", "user", name, "error", err)
		return nil
	}
	request, err := decodeEAP(encoded)
	if err != nil {
		slog.Warn("radius-mock: EAP Access-Request discarded", "user", name, "error", err)
		return nil
	}

	reply, err := s.round(pkt, request, users)
	if err != nil {
		slog.Warn("radius-mock: EAP round failed", "user", name, "error", err)
		return nil
	}
	if logPackets {
		slog.Info("radius-mock: Access-Request", "user", name, "method", "eap",
			"eap-type", request.eapType, "reply", codeName(reply.code))
	}
	response, err := buildEAPResponse(reply, pkt.Identifier, pkt.Authenticator, key)
	if err != nil {
		slog.Warn("radius-mock: EAP reply could not be built", "user", name, "error", err)
		return nil
	}
	return response
}

// round advances one EAP conversation by one packet. It owns the control flow:
// every branch it takes ends in an eapReply, and the helpers it calls compute
// one answer each.
func (s *eapServer) round(pkt *radius.Packet, request eapPacket, users userList) (eapReply, error) {
	// RFC 3748 Section 4: an authenticator receives Responses. A Request, a
	// Success or a Failure from the peer is not part of this conversation.
	if request.code != eapCodeResponse {
		return eapReply{}, errStr("radius-mock: EAP Code is not a Response")
	}
	if request.eapType == eap.TypeIdentity {
		return s.identity(request, users)
	}

	// RFC 2865 Section 5.24: State "MUST be sent unmodified from the client to
	// the server in the new Access-Request reply to that challenge". Every round
	// after the identity answers a challenge, so a request without the State this
	// server issued is refused rather than answered.
	state := string(pkt.FindAttr(radius.AttrState))
	session, known := s.sessions[state]
	if !known {
		return eapReply{code: radius.CodeAccessReject, eap: encodeEAP(eapCodeFailure, request.identifier, nil)}, nil
	}
	if request.eapType != eap.TypeMSCHAPv2 || len(request.data) == 0 {
		delete(s.sessions, state)
		return eapReply{code: radius.CodeAccessReject, eap: encodeEAP(eapCodeFailure, request.identifier, nil)}, nil
	}

	opCode := request.data[0]
	if session.stage == stageResponse && opCode == mschapv2OpResponse {
		return s.verifyResponse(session, request, state)
	}
	if session.stage == stageSuccessAck && opCode == mschapv2OpSuccess {
		delete(s.sessions, state)
		return eapReply{
			code:  radius.CodeAccessAccept,
			eap:   encodeEAP(eapCodeSuccess, request.identifier, nil),
			extra: profileAttrs(session.user),
		}, nil
	}
	delete(s.sessions, state)
	return eapReply{code: radius.CodeAccessReject, eap: encodeEAP(eapCodeFailure, request.identifier, nil)}, nil
}

// identity opens a conversation from the peer's EAP-Response/Identity and
// challenges it with MS-CHAPv2.
func (s *eapServer) identity(request eapPacket, users userList) (eapReply, error) {
	name := stripDomain(string(request.data))
	user, known := findUser(users, name)
	if !known {
		return eapReply{code: radius.CodeAccessReject, eap: encodeEAP(eapCodeFailure, request.identifier, nil)}, nil
	}

	session := &eapSession{user: user, peerName: name, msID: request.identifier + 1, stage: stageResponse}
	if _, err := rand.Read(session.authChallenge[:]); err != nil {
		return eapReply{}, err
	}
	s.issued++
	var tb textbuf.Buffer
	state := tb.Str("ze-mock-eap-").Int(s.issued).String()
	s.sessions[state] = session

	return eapReply{
		code:  radius.CodeAccessChallenge,
		eap:   encodeEAP(eapCodeRequest, request.identifier+1, mschapv2Challenge(session, name)),
		state: state,
	}, nil
}

// verifyResponse checks the peer's NT-Response against the password the server
// holds, and answers a verified one with the mutual proof RFC 2759 Section 5
// requires the peer to check in its turn.
func (s *eapServer) verifyResponse(session *eapSession, request eapPacket, state string) (eapReply, error) {
	td := request.data
	if len(td) < mschapv2ResponseFloor || td[4] != mschapv2ResponseLen {
		delete(s.sessions, state)
		return eapReply{code: radius.CodeAccessReject, eap: encodeEAP(eapCodeFailure, request.identifier, nil)}, nil
	}
	copy(session.peerChallenge[:], td[5:21])
	copy(session.ntResponse[:], td[29:53])
	// RFC 2759 Section 4: the Name follows the 49-octet Response, and Section 8
	// hashes the user name without its domain.
	peerName := stripDomain(string(td[mschapv2ResponseFloor:]))

	expected := eap.GenerateNTResponse(session.authChallenge, session.peerChallenge, peerName, session.user.pass)
	if subtle.ConstantTimeCompare(expected[:], session.ntResponse[:]) != 1 {
		delete(s.sessions, state)
		return eapReply{code: radius.CodeAccessReject, eap: encodeEAP(eapCodeFailure, request.identifier, nil)}, nil
	}

	authResponse := eap.GenerateAuthenticatorResponse(session.user.pass, session.ntResponse,
		session.peerChallenge, session.authChallenge, peerName)
	session.peerName = peerName
	session.stage = stageSuccessAck
	// RFC 2759 Section 5: the Message field is "S=<auth_string> M=<message>",
	// where the auth_string is "encoded in ASCII as 40 hexadecimal digits".
	var tb textbuf.Buffer
	message := tb.Str("S=").Str(strings.ToUpper(hex.EncodeToString(authResponse[:]))).
		Str(" M=Authentication succeeded").String()
	return eapReply{
		code:  radius.CodeAccessChallenge,
		eap:   encodeEAP(eapCodeRequest, request.identifier+1, mschapv2Success(session.msID, message)),
		state: state,
	}, nil
}

// mschapv2Challenge builds the EAP Type-Data of an MS-CHAPv2 Challenge.
//
// RFC 2759 Section 3, over the draft-kamath EAP encapsulation:
//
//	 0        1        2        3        4        5                 21
//	+--------+--------+--------+--------+--------+-----------------+------+
//	| OpCode | MS-ID  |    MS-Length    |Val-Size|  Challenge (16) | Name |
//	+--------+--------+--------+--------+--------+-----------------+------+
//	    1                                   16
func mschapv2Challenge(session *eapSession, name string) []byte {
	msLen := 5 + mschapv2ChallengeLen + len(name)
	td := make([]byte, 1+msLen)
	td[0] = eap.TypeMSCHAPv2
	td[1] = mschapv2OpChallenge
	td[2] = session.msID
	binary.BigEndian.PutUint16(td[3:5], uint16(msLen))
	td[5] = mschapv2ChallengeLen
	copy(td[6:], session.authChallenge[:])
	copy(td[6+mschapv2ChallengeLen:], name)
	return td
}

// mschapv2Success builds the EAP Type-Data of an MS-CHAPv2 Success, whose
// Message carries the authenticator response the peer verifies.
func mschapv2Success(msID uint8, message string) []byte {
	msLen := 4 + len(message)
	td := make([]byte, 1+msLen)
	td[0] = eap.TypeMSCHAPv2
	td[1] = mschapv2OpSuccess
	td[2] = msID
	binary.BigEndian.PutUint16(td[3:5], uint16(msLen))
	copy(td[5:], message)
	return td
}

// encodeEAP writes one EAP packet. RFC 3748 Section 4 puts the Code, the
// Identifier and the Length in the first four octets, and Section 4.2 gives a
// Success and a Failure nothing after them, which is what a nil typeData writes.
func encodeEAP(code, identifier uint8, typeData []byte) []byte {
	packet := make([]byte, eapHeaderLen+len(typeData))
	packet[0] = code
	packet[1] = identifier
	binary.BigEndian.PutUint16(packet[2:4], uint16(len(packet)))
	copy(packet[eapHeaderLen:], typeData)
	return packet
}

// decodeEAP reads one EAP packet out of the concatenated EAP-Message values.
//
// RFC 3748 Section 4: "The Length field is two octets and indicates the length
// of the EAP packet including the Code, Identifier, Length, and Data fields",
// and "Octets outside the range of the Length field should be treated as Data
// Link Layer padding and MUST be ignored on reception."
func decodeEAP(packet []byte) (eapPacket, error) {
	if len(packet) < eapHeaderLen {
		return eapPacket{}, errStr("radius-mock: EAP packet shorter than its header")
	}
	length := int(binary.BigEndian.Uint16(packet[2:4]))
	if length < eapHeaderLen || length > len(packet) {
		return eapPacket{}, errStr("radius-mock: EAP Length does not match the octets received")
	}
	decoded := eapPacket{code: packet[0], identifier: packet[1]}
	if decoded.code == eapCodeSuccess || decoded.code == eapCodeFailure {
		return decoded, nil
	}
	// RFC 3748 Section 4.1: a Request and a Response both carry a Type field, so
	// four octets is a malformed one rather than an empty one.
	if length == eapHeaderLen {
		return eapPacket{}, errStr("radius-mock: EAP Request or Response carries no Type")
	}
	decoded.eapType = packet[4]
	decoded.data = packet[5:length]
	return decoded, nil
}

// concatenateEAPMessage joins the EAP-Message attributes of a request into the
// one EAP packet they encode, and refuses a run that is not consecutive.
//
// RFC 3579 Section 3.1: "If multiple EAP-Message attributes are contained within
// an Access-Request or Access-Challenge packet, they MUST be in order and they
// MUST be consecutive attributes in the Access-Request or Access-Challenge
// packet", and "Multiple EAP packets MUST NOT be encoded within EAP-Message
// attributes contained within a single Access-Challenge, Access-Accept,
// Access-Reject or Access-Request packet."
//
// The walk is the mock's own rather than ze's, so a client that broke the run
// is refused by a reader that did not write it.
func concatenateEAPMessage(pkt *radius.Packet) ([]byte, error) {
	first, count := -1, 0
	for index, attr := range pkt.Attrs {
		if attr.Type != radius.AttrEAPMessage {
			continue
		}
		if first < 0 {
			first, count = index, 1
			continue
		}
		if index != first+count {
			return nil, errStr("radius-mock: the EAP-Message attributes are not consecutive")
		}
		count++
	}
	if first < 0 {
		return nil, errStr("radius-mock: no EAP-Message attribute")
	}
	packet := make([]byte, 0, radius.MaxAttrLen)
	for _, attr := range pkt.Attrs[first : first+count] {
		packet = append(packet, attr.Value...)
	}
	return packet, nil
}

// appendEAPMessage splits one EAP packet into consecutive EAP-Message
// attributes, at the 253-octet limit RFC 3579 Section 3.1 describes: "this
// allows EAP packets longer than 253 octets to be transported by RADIUS".
func appendEAPMessage(attrs []radius.Attr, packet []byte) []radius.Attr {
	for off := 0; off < len(packet); off += maxEAPMessageValue {
		end := min(off+maxEAPMessageValue, len(packet))
		attrs = append(attrs, radius.Attr{Type: radius.AttrEAPMessage, Value: packet[off:end]})
	}
	return attrs
}

// buildEAPResponse encodes one reply and signs it.
//
// RFC 3579 Section 3.2: "Message-Authenticator = HMAC-MD5 (Type, Identifier,
// Length, Request Authenticator, Attributes)", and "When the message integrity
// check is calculated the signature string should be considered to be sixteen
// octets of zero." So the Request Authenticator sits in the authenticator field
// while the HMAC runs, and the Response Authenticator of RFC 2865 Section 3 is
// computed afterwards, over the attributes the signature is now part of.
func buildEAPResponse(reply eapReply, id uint8, requestAuth [radius.AuthenticatorLen]byte, key []byte) ([]byte, error) {
	attrs := appendEAPMessage(make([]radius.Attr, 0, 4), reply.eap)
	attrs = append(attrs, radius.Attr{Type: radius.AttrMessageAuthenticator, Value: make([]byte, radius.AuthenticatorLen)})
	if reply.state != "" {
		attrs = append(attrs, radius.Attr{Type: radius.AttrState, Value: []byte(reply.state)})
	}
	attrs = append(attrs, reply.extra...)

	body := make([]byte, 0, 128)
	for _, attr := range attrs {
		body = append(body, attr.Type, byte(2+len(attr.Value)))
		body = append(body, attr.Value...)
	}
	total := radius.HeaderLen + len(body)
	packet := make([]byte, total)
	packet[0] = reply.code
	packet[1] = id
	binary.BigEndian.PutUint16(packet[2:4], uint16(total))
	copy(packet[4:], requestAuth[:])
	copy(packet[radius.HeaderLen:], body)

	off, found := messageAuthenticatorOffset(packet)
	if !found {
		return nil, errStr("radius-mock: the reply carries no Message-Authenticator to sign")
	}
	mac := hmac.New(md5.New, key) //nolint:gosec // RFC 3579 Section 3.2 mandates HMAC-MD5
	mac.Write(packet)
	copy(packet[off:off+radius.AuthenticatorLen], mac.Sum(nil))

	auth := radius.ResponseAuthenticator(reply.code, id, uint16(total), requestAuth, packet[radius.HeaderLen:], key)
	copy(packet[4:4+radius.AuthenticatorLen], auth[:])
	return packet, nil
}

// verifyRequestSignature recomputes the Message-Authenticator of an
// Access-Request over the octets that arrived, and answers whether the client
// signed them with the shared secret.
//
// It is computed here rather than through ze's signer, because a server that
// signed with the client's own code would agree with it whatever either wrote.
func verifyRequestSignature(data, key []byte) bool {
	if len(data) < radius.MinPacketLen {
		return false
	}
	length := int(binary.BigEndian.Uint16(data[2:4]))
	if length < radius.MinPacketLen || length > len(data) {
		return false
	}
	packet := make([]byte, length)
	copy(packet, data[:length])
	off, found := messageAuthenticatorOffset(packet)
	if !found {
		return false
	}
	var received [radius.AuthenticatorLen]byte
	copy(received[:], packet[off:off+radius.AuthenticatorLen])
	clear(packet[off : off+radius.AuthenticatorLen])
	mac := hmac.New(md5.New, key) //nolint:gosec // RFC 3579 Section 3.2 mandates HMAC-MD5
	mac.Write(packet)
	return hmac.Equal(received[:], mac.Sum(nil))
}

// messageAuthenticatorOffset walks the attribute list and answers where the
// Message-Authenticator value starts. An attribute list that does not walk, and
// one whose Message-Authenticator is not the 18 octets RFC 3579 Section 3.2
// gives it, both answer false: neither can be signed or verified.
func messageAuthenticatorOffset(packet []byte) (int, bool) {
	if len(packet) < radius.HeaderLen {
		return 0, false
	}
	length := int(binary.BigEndian.Uint16(packet[2:4]))
	if length < radius.HeaderLen || length > len(packet) {
		return 0, false
	}
	for off := radius.HeaderLen; off+2 <= length; {
		attrLen := int(packet[off+1])
		if attrLen < 2 || off+attrLen > length {
			return 0, false
		}
		if packet[off] == radius.AttrMessageAuthenticator {
			if attrLen != 2+radius.AuthenticatorLen {
				return 0, false
			}
			return off + 2, true
		}
		off += attrLen
	}
	return 0, false
}

// findUser answers the configured credential for a name.
func findUser(users userList, name string) (mockUser, bool) {
	for _, user := range users {
		if user.name == name {
			return user, true
		}
	}
	return mockUser{}, false
}

// stripDomain drops a DOMAIN\ prefix from a user name, which is what RFC 2759
// Section 8 hashes and what ze's peer sends in its Name field.
func stripDomain(name string) string {
	if _, user, found := strings.Cut(name, "\\"); found {
		return user
	}
	return name
}
