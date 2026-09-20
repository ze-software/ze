// Design: docs/research/l2tpv2-ze-integration.md -- RADIUS auth handler
// RFC: rfc/short/rfc2865.md -- Access-Request attributes
// RFC: rfc/short/rfc2869.md -- NAS-Port-Id (Section 5.17)
// Related: l2tpauthradius.go -- atomic logger, Name constant
// Related: nasportid.go -- NAS-Port-Id template resolution

package l2tpauthradius

import (
	"bytes"
	"context"
	"encoding/hex"
	"net"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/component/l2tp"
	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/component/radius"
)

// radiusAuth holds the RADIUS client and implements the auth handler.
type radiusAuth struct {
	mu              sync.RWMutex
	client          *radius.Client
	nasID           string
	serverAddr      string
	sourceAddress   net.IP
	nasPortIDFormat string
	exclusions      attributeExclusions
}

// attributePolicy is what the operator configured about the attribute list of
// one Access-Request: the template that ADDS NAS-Port-Id, and the exclusions
// that REMOVE attributes. The two travel together because one request answers
// to both, and a function that took them apart would need a parameter for each.
type attributePolicy struct {
	nasPortIDFormat string
	exclusions      attributeExclusions
}

func newRADIUSAuth() *radiusAuth {
	return &radiusAuth{}
}

// swapClient replaces the client and returns the old one (caller closes it).
// The NAS-Port-Id format travels with the client so one reload applies both at
// once: a request can never pick up the new client with the old format.
func (a *radiusAuth) swapClient(c *radius.Client, nasID, serverAddr string, sourceAddr net.IP, nasPortIDFormat string) *radius.Client {
	a.mu.Lock()
	defer a.mu.Unlock()
	old := a.client
	a.client = c
	a.nasID = nasID
	a.serverAddr = serverAddr
	a.sourceAddress = sourceAddr
	a.nasPortIDFormat = nasPortIDFormat
	return old
}

// setExclusions installs the attributes this deployment holds back. It is
// separate from swapClient because the two answer different questions: which
// server a request goes to, and which attributes the request carries. A reload
// applies to every request built after it.
func (a *radiusAuth) setExclusions(exclusions attributeExclusions) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.exclusions = exclusions
}

// handle is the AuthHandler registered with the l2tp package.
// It spawns a goroutine per request for async RADIUS I/O, returning
// Handled=true so the drain goroutine skips its own AuthResponse call.
func (a *radiusAuth) handle(req ppp.EventAuthRequest, respond l2tp.AuthRespondFunc) l2tp.AuthResult {
	a.mu.RLock()
	client := a.client
	nasID := a.nasID
	sAddr := a.serverAddr
	srcAddr := a.sourceAddress
	policy := attributePolicy{nasPortIDFormat: a.nasPortIDFormat, exclusions: a.exclusions}
	a.mu.RUnlock()

	// RFC 2865 Section 4.1: "An Access-Request MUST contain either a
	// User-Password or a CHAP-Password or a State." A session the operator
	// configured for no authentication carries none of the three, so there is
	// no conformant Access-Request for this handler to send and no server
	// answer to wait for. The operator already decided the question the
	// Access-Request would have asked.
	//
	// Admitting it HERE is what lets a RADIUS server be configured for
	// accounting alone. activateRadiusConfig (register.go) claims the single
	// auth slot for every RADIUS deployment, accounting-only included, so a
	// refusal here refuses every session on a wholesale LNS that bills and
	// never authenticates. The guard below still holds for every method that
	// does carry a credential: buildAccessRequestAttrs refuses to build a
	// request without one.
	if req.Method == ppp.AuthMethodNone {
		return l2tp.AuthResult{Accept: true, Message: "no-auth accepted (RADIUS carries no credential for it)"}
	}

	if client == nil {
		logger().Warn("l2tp-auth-radius: no RADIUS client configured; rejecting",
			"tunnel", req.TunnelID, "session", req.SessionID)
		return l2tp.AuthResult{Accept: false, Message: "no RADIUS client"}
	}

	incAuthSent(sAddr, sAddr)
	go a.doRADIUS(req, client, nasID, srcAddr, policy, respond)
	return l2tp.AuthResult{Handled: true}
}

func (a *radiusAuth) doRADIUS(req ppp.EventAuthRequest, client *radius.Client, nasID string, sourceAddr net.IP, policy attributePolicy, respond l2tp.AuthRespondFunc) {
	defer func() {
		if r := recover(); r != nil {
			logger().Error("l2tp-auth-radius: goroutine panic",
				"tunnel", req.TunnelID, "session", req.SessionID, "panic", r)
			if err := respond(false, "internal error", nil); err != nil {
				logger().Warn("l2tp-auth-radius: respond failed after panic", "error", err)
			}
		}
	}()

	auth, err := radius.RandomAuthenticator()
	if err != nil {
		logger().Error("l2tp-auth-radius: random authenticator failed", "error", err)
		if respErr := respond(false, "internal error", nil); respErr != nil {
			logger().Warn("l2tp-auth-radius: respond failed", "error", respErr)
		}
		return
	}

	attrs, ok := buildAccessRequestAttrs(req, nasID, sourceAddr, policy)
	if !ok {
		// RFC 2865 Section 4.1: "An Access-Request MUST contain either a
		// User-Password or a CHAP-Password or a State." The peer supplied no
		// credential this LNS can carry, so there is no conformant Access-Request
		// to send and no server to ask. Deny the session.
		logger().Warn("l2tp-auth-radius: no credential attribute for this method; rejecting",
			"tunnel", req.TunnelID, "session", req.SessionID, "method", req.Method)
		if respErr := respond(false, "no usable credential", nil); respErr != nil {
			logger().Warn("l2tp-auth-radius: respond failed", "error", respErr)
		}
		return
	}

	pkt := &radius.Packet{
		Code:          radius.CodeAccessRequest,
		Authenticator: auth,
		Attrs:         attrs,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	resp, err := client.SendToServers(ctx, pkt)

	a.mu.RLock()
	srvAddr := a.serverAddr
	a.mu.RUnlock()

	if err != nil {
		setRadiusUp(srvAddr, srvAddr, false)
		logger().Warn("l2tp-auth-radius: RADIUS request failed",
			"tunnel", req.TunnelID, "session", req.SessionID, "error", err)
		if respErr := respond(false, "RADIUS unreachable", nil); respErr != nil {
			logger().Warn("l2tp-auth-radius: respond failed", "error", respErr)
		}
		return
	}
	setRadiusUp(srvAddr, srvAddr, true)

	switch resp.Code {
	case radius.CodeAccessAccept:
		// RFC 2865 Section 5.6: "A NAS is not required to implement all of these
		// service types, and MUST treat unknown or unsupported Service-Types as
		// though an Access-Reject had been received instead."
		// RFC 2865 Section 1.1: "A NAS MUST treat a RADIUS access-accept
		// authorizing an unavailable service as an access-reject instead."
		//
		// The LNS provides framed PPP access and asks for Framed-User, so an
		// Accept naming any other service authorizes something this NAS cannot
		// bring up. Denying is what the RFC requires, and it also stops a session
		// coming up under an authorization ze cannot honor.
		if !radius.AcceptedServiceType(resp, radius.ServiceTypeFramed) {
			logger().Warn("l2tp-auth-radius: Access-Accept names an unsupported Service-Type; rejecting",
				"tunnel", req.TunnelID, "session", req.SessionID, "username", req.Username)
			if respErr := respond(false, "unsupported Service-Type", nil); respErr != nil {
				logger().Warn("l2tp-auth-radius: respond failed", "error", respErr)
			}
			return
		}

		// RFC 2865: extract subscriber profile attributes from Access-Accept.
		if meta := extractAuthMetadata(resp); meta != nil {
			l2tp.StoreSessionMetadata(req.TunnelID, req.SessionID, meta)
			logger().Info("l2tp-auth-radius: stored session metadata",
				"tunnel", req.TunnelID, "session", req.SessionID,
				"framed-ip", meta.FramedIP, "framed-pool", meta.FramedPool)
		}
		var authBlob []byte
		if req.Method == ppp.AuthMethodMSCHAPv2 {
			authBlob = extractMSCHAP2Success(resp)
			if authBlob == nil {
				logger().Warn("l2tp-auth-radius: Access-Accept carries no readable MS-CHAP2-Success; rejecting",
					"tunnel", req.TunnelID, "session", req.SessionID, "username", req.Username)
				if respErr := respond(false, "no MS-CHAP2-Success in Access-Accept", nil); respErr != nil {
					logger().Warn("l2tp-auth-radius: respond failed", "error", respErr)
				}
				return
			}
		}
		logger().Info("l2tp-auth-radius: accepted",
			"tunnel", req.TunnelID, "session", req.SessionID, "username", req.Username)
		if respErr := respond(true, "", authBlob); respErr != nil {
			logger().Warn("l2tp-auth-radius: respond failed", "error", respErr)
		}

	case radius.CodeAccessReject:
		msg := "RADIUS rejected"
		if reply := resp.FindAttr(radius.AttrReplyMessage); reply != nil {
			msg = string(reply)
		}
		logger().Info("l2tp-auth-radius: rejected",
			"tunnel", req.TunnelID, "session", req.SessionID, "username", req.Username, "reason", msg)
		if respErr := respond(false, msg, nil); respErr != nil {
			logger().Warn("l2tp-auth-radius: respond failed", "error", respErr)
		}

	default:
		logger().Warn("l2tp-auth-radius: unexpected response code",
			"tunnel", req.TunnelID, "session", req.SessionID, "code", resp.Code)
		if respErr := respond(false, "unexpected RADIUS response", nil); respErr != nil {
			logger().Warn("l2tp-auth-radius: respond failed", "error", respErr)
		}
	}
}

// buildAccessRequestAttrs is the full attribute list of one Access-Request:
// the RFC 2865 set every request carries, plus the operator's NAS-Port-Id, less
// whatever the operator held back.
//
// The parts are built separately because they answer to different sources.
// buildAuthAttrs carries what the RFC and the credential method require, and
// the policy carries what the operator configured. An empty policy adds nothing
// and removes nothing, so an unconfigured deployment sends exactly the
// attributes it sent before this feature existed.
func buildAccessRequestAttrs(req ppp.EventAuthRequest, nasID string, sourceAddr net.IP, policy attributePolicy) ([]radius.Attr, bool) {
	attrs, ok := buildAuthAttrs(req, nasID, sourceAddr)
	if !ok {
		return nil, false
	}

	// RFC 2869 Section 5.17: NAS-Port-Id names the access port in text, for a
	// NAS that cannot conveniently number its ports. An LNS port is the tunnel
	// and session pair, so the operator's template composes it.
	if attr, ok := nasPortIDAttr(policy.nasPortIDFormat, nasPortIDFacts{
		nasID:     nasID,
		tunnelID:  req.TunnelID,
		sessionID: req.SessionID,
	}); ok {
		attrs = append(attrs, attr)
	}

	// The exclusions are applied HERE, to the finished list, and never as a
	// condition on one of the appends above. RFC 2865 Section 4.1: "It MUST
	// contain either a NAS-IP-Address attribute or a NAS-Identifier attribute
	// (or both)", and "An Access-Request MUST contain either a User-Password or
	// a CHAP-Password or a State". The schema names neither the NAS identity nor a
	// credential attribute, so what survives is still a conformant
	// Access-Request. NAS-Port-Id is the one attribute an operator can remove
	// from this list, and the same section makes User-Name a SHOULD.
	return policy.exclusions.filter(attrs, packetAccessRequest), true
}

// buildAuthAttrs builds the RFC 2865 attribute set for one Access-Request, and
// reports false when the peer's credential cannot produce one of the three
// credential attributes RFC 2865 Section 4.1 admits. A false result means no
// Access-Request is sent at all.
//
// RFC 2865 Section 5.2: User-Password stored as cleartext here;
// the client XOR-encodes it per-server in Exchange().
func buildAuthAttrs(req ppp.EventAuthRequest, nasID string, sourceAddr net.IP) ([]radius.Attr, bool) {
	attrs := []radius.Attr{
		{Type: radius.AttrServiceType, Value: radius.AttrUint32(radius.ServiceTypeFramed)},
		{Type: radius.AttrFramedProtocol, Value: radius.AttrUint32(radius.FramedProtocolPPP)},
		{Type: radius.AttrNASPortType, Value: radius.AttrUint32(radius.NASPortTypeVirtual)},
		{Type: radius.AttrNASPort, Value: radius.AttrUint32(uint32(req.SessionID))},
	}

	// RFC 2865 Section 5: "Text of length zero (0) MUST NOT be sent; omit the
	// entire attribute instead." A PAP peer may send Peer-ID-Length 0 and a
	// CHAP peer an empty Name, so User-Name is text whose length the peer picks.
	attrs = radius.AppendTextAttr(attrs, radius.AttrUserName, req.Username)

	// RFC 2865 Section 4.1: "Either NAS-IP-Address or NAS-Identifier MUST be
	// present in an Access-Request."
	attrs = appendNASIdentity(attrs, nasID, sourceAddr)

	// RFC 2865 Section 4.1: "An Access-Request MUST contain either a
	// User-Password or a CHAP-Password or a State." Ze holds no State for a
	// subscriber, so each method below either produces one of the other two or
	// reports that no Access-Request can be built.
	switch req.Method {
	case ppp.AuthMethodPAP:
		attrs = append(attrs, radius.Attr{Type: radius.AttrUserPassword, Value: req.Response})

	case ppp.AuthMethodCHAPMD5:
		attrs = append(attrs,
			radius.Attr{
				Type:  radius.AttrCHAPPassword,
				Value: radius.EncodeCHAPPassword(req.Identifier, req.Response),
			},
			radius.Attr{
				Type:  radius.AttrCHAPChallenge,
				Value: req.Challenge,
			},
		)

	case ppp.AuthMethodMSCHAPv2:
		// The MS-CHAPv2 response carries the credential in a vendor-specific
		// attribute, which stands in for CHAP-Password. A response shorter than
		// the 16-octet peer challenge plus the 24-octet NT response yields
		// neither, and so yields no credential at all.
		if len(req.Response) < 40 {
			return nil, false
		}
		vsaResp, err := radius.EncodeMSCHAP2Response(req.Identifier, req.Response[:16], req.Response[16:40])
		if err != nil {
			return nil, false
		}
		vsaChal, err := radius.EncodeMSCHAPChallenge(req.Challenge)
		if err != nil {
			return nil, false
		}
		attrs = append(attrs,
			radius.Attr{Type: radius.AttrVendorSpecific, Value: vsaResp[2:]},
			radius.Attr{Type: radius.AttrVendorSpecific, Value: vsaChal[2:]},
		)

	case ppp.AuthMethodNone:
		// The peer authenticated with nothing, so there is no credential to put
		// in an Access-Request and no request to send.
		return nil, false
	}

	return attrs, true
}

func extractMSCHAP2Success(resp *radius.Packet) []byte {
	for _, val := range resp.FindAllAttr(radius.AttrVendorSpecific) {
		vendorID, vendorType, data, err := radius.DecodeVSA(val)
		if err != nil {
			continue
		}
		if vendorID != radius.VendorMicrosoft || vendorType != radius.MSCHAP2Success {
			continue
		}
		return decodeMSCHAP2Success(data)
	}
	return nil
}

// mschapv2AuthenticatorResponseLen is the raw Authenticator Response the PPP
// authenticator declares it needs. runMSCHAPv2AuthPhase
// (internal/component/l2tp/ppp/mschapv2.go) fails the session on any other
// length and hex-encodes these octets itself into the Success packet.
const mschapv2AuthenticatorResponseLen = 20

// mschap2SuccessPrefix opens the authenticator string RFC 2548 carries.
var mschap2SuccessPrefix = []byte("S=")

// decodeMSCHAP2Success turns one MS-CHAP2-Success attribute value into the raw
// Authenticator Response its consumer requires, and returns nil for a value
// this format does not describe.
//
// RFC 2548 Section 2.3.3 gives the attribute value two fields: an "Ident"
// octet "Identical to the PPP MS-CHAP v2 Identifier", then "String: The
// 42-octet authenticator string". RFC 2759 Section 5 says of that string: "The
// <auth_string> quantity is a 20 octet number encoded in ASCII as 40
// hexadecimal digits."
//
// So the attribute carries 43 octets where the consumer requires 20, and
// handing it over verbatim failed the consumer's length guard on every
// RADIUS-backed MS-CHAPv2 session. The Ident octet is dropped and the hex is
// decoded here, in the one function that knows the attribute's format.
//
// A value that does not match the format is refused rather than passed on
// half-read. The caller then rejects the session, which is the honest answer:
// a Success packet built from octets ze could not read carries an S= field the
// peer verifies and discards (RFC 2759 Section 5: "If the authenticator
// response is either missing or incorrect, the peer MUST" terminate).
func decodeMSCHAP2Success(data []byte) []byte {
	// The Ident octet is not part of the authenticator string.
	if len(data) < 1 {
		return nil
	}
	authString := data[1:]
	if !bytes.HasPrefix(authString, mschap2SuccessPrefix) {
		return nil
	}
	digits := authString[len(mschap2SuccessPrefix):]
	if len(digits) < 2*mschapv2AuthenticatorResponseLen {
		return nil
	}
	raw := make([]byte, mschapv2AuthenticatorResponseLen)
	if _, err := hex.Decode(raw, digits[:2*mschapv2AuthenticatorResponseLen]); err != nil {
		return nil
	}
	return raw
}
