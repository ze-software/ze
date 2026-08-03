// Design: docs/research/l2tpv2-ze-integration.md -- RADIUS auth handler
// RFC: rfc/short/rfc2865.md -- Access-Request attributes
// RFC: rfc/short/rfc2869.md -- NAS-Port-Id (Section 5.17)
// Related: l2tpauthradius.go -- atomic logger, Name constant
// Related: nasportid.go -- NAS-Port-Id template resolution

package l2tpauthradius

import (
	"context"
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

// handle is the AuthHandler registered with the l2tp package.
// It spawns a goroutine per request for async RADIUS I/O, returning
// Handled=true so the drain goroutine skips its own AuthResponse call.
func (a *radiusAuth) handle(req ppp.EventAuthRequest, respond l2tp.AuthRespondFunc) l2tp.AuthResult {
	a.mu.RLock()
	client := a.client
	nasID := a.nasID
	sAddr := a.serverAddr
	srcAddr := a.sourceAddress
	portIDFormat := a.nasPortIDFormat
	a.mu.RUnlock()

	if client == nil {
		if req.Method == ppp.AuthMethodNone {
			return l2tp.AuthResult{Accept: true, Message: "no-auth accepted (no RADIUS client)"}
		}
		logger().Warn("l2tp-auth-radius: no RADIUS client configured; rejecting",
			"tunnel", req.TunnelID, "session", req.SessionID)
		return l2tp.AuthResult{Accept: false, Message: "no RADIUS client"}
	}

	incAuthSent(sAddr, sAddr)
	go a.doRADIUS(req, client, nasID, srcAddr, portIDFormat, respond)
	return l2tp.AuthResult{Handled: true}
}

func (a *radiusAuth) doRADIUS(req ppp.EventAuthRequest, client *radius.Client, nasID string, sourceAddr net.IP, nasPortIDFormat string, respond l2tp.AuthRespondFunc) {
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

	pkt := &radius.Packet{
		Code:          radius.CodeAccessRequest,
		Authenticator: auth,
		Attrs:         buildAccessRequestAttrs(req, nasID, sourceAddr, nasPortIDFormat),
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
// the RFC 2865 set every request carries, plus the operator's NAS-Port-Id.
//
// The two are built separately because they answer to different sources.
// buildAuthAttrs carries what the RFC and the credential method require, and
// nasPortIDFormat carries what the operator configured. An empty format adds
// nothing, so an unconfigured deployment sends exactly today's attributes.
func buildAccessRequestAttrs(req ppp.EventAuthRequest, nasID string, sourceAddr net.IP, nasPortIDFormat string) []radius.Attr {
	attrs := buildAuthAttrs(req, nasID, sourceAddr)

	// RFC 2869 Section 5.17: NAS-Port-Id names the access port in text, for a
	// NAS that cannot conveniently number its ports. An LNS port is the tunnel
	// and session pair, so the operator's template composes it.
	if attr, ok := nasPortIDAttr(nasPortIDFormat, nasPortIDFacts{
		nasID:     nasID,
		tunnelID:  req.TunnelID,
		sessionID: req.SessionID,
	}); ok {
		attrs = append(attrs, attr)
	}
	return attrs
}

// RFC 2865 Section 5.2: User-Password stored as cleartext here;
// the client XOR-encodes it per-server in Exchange().
func buildAuthAttrs(req ppp.EventAuthRequest, nasID string, sourceAddr net.IP) []radius.Attr {
	attrs := []radius.Attr{
		{Type: radius.AttrUserName, Value: radius.AttrString(req.Username)},
		{Type: radius.AttrServiceType, Value: radius.AttrUint32(radius.ServiceTypeFramed)},
		{Type: radius.AttrFramedProtocol, Value: radius.AttrUint32(radius.FramedProtocolPPP)},
		{Type: radius.AttrNASPortType, Value: radius.AttrUint32(radius.NASPortTypeVirtual)},
		{Type: radius.AttrNASPort, Value: radius.AttrUint32(uint32(req.SessionID))},
	}

	if v4 := sourceAddr.To4(); v4 != nil {
		attrs = append(attrs, radius.Attr{Type: radius.AttrNASIPAddress, Value: v4})
	}

	if nasID != "" {
		attrs = append(attrs, radius.Attr{Type: radius.AttrNASIdentifier, Value: radius.AttrString(nasID)})
	}

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
		if len(req.Response) >= 40 {
			peerChallenge := req.Response[:16]
			ntResponse := req.Response[16:40]
			if vsaResp, err := radius.EncodeMSCHAP2Response(req.Identifier, peerChallenge, ntResponse); err == nil {
				attrs = append(attrs, radius.Attr{Type: radius.AttrVendorSpecific, Value: vsaResp[2:]})
			}
			if vsaChal, err := radius.EncodeMSCHAPChallenge(req.Challenge); err == nil {
				attrs = append(attrs, radius.Attr{Type: radius.AttrVendorSpecific, Value: vsaChal[2:]})
			}
		}

	case ppp.AuthMethodNone:
	}

	return attrs
}

func extractMSCHAP2Success(resp *radius.Packet) []byte {
	for _, val := range resp.FindAllAttr(radius.AttrVendorSpecific) {
		vendorID, vendorType, data, err := radius.DecodeVSA(val)
		if err != nil {
			continue
		}
		if vendorID == radius.VendorMicrosoft && vendorType == radius.MSCHAP2Success {
			return data
		}
	}
	return nil
}
