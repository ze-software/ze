// Design: docs/guide/radius.md -- RADIUS/EAP admin login
// Overview: authenticator.go -- exchange and result, which this loop reuses
// Related: eap.go -- the EAP-Message split and concatenation
// Related: internal/core/eap -- the EAP peer this drives
// RFC: rfc/short/rfc3579.md -- Sections 2.1, 2.6.3, 2.6.4, 3.1
// RFC: rfc/short/rfc2865.md -- Section 5.24 State

package radius

import (
	"context"
	"errors"
	"fmt"

	"github.com/ze-software/ze/internal/component/aaa"
	"github.com/ze-software/ze/internal/core/eap"
)

// errEAPNoResponse reports a challenge the peer answered with nothing: it
// discarded the packet, so there is no EAP-Response to put in the next
// Access-Request and the conversation cannot continue.
var errEAPNoResponse = errors.New("radius: the EAP peer discarded the challenge and has no response to send")

// authenticateEAP runs one RADIUS/EAP conversation for an operator login.
//
// Ze is both the NAS and the EAP peer. RFC 3579 Section 2.1 describes a NAS
// that "initially sends an EAP-Request/Identity message to the peer" and then
// forwards the peer's Response to the RADIUS server; here the peer is
// internal/core/eap, running in this process on the password the operator
// typed. Nothing is manufactured: the Identity Request below is the one the NAS
// sends its peer, and every EAP packet that reaches the wire came out of
// (*eap.PeerSession).Process.
//
// The loop is bounded on both axes, and a server can disable neither. The peer
// counts every Process call against its own maxEAPRounds and returns
// ErrTooManyRounds past it (internal/core/eap/peer.go), and ctx carries the
// authenticator's time budget, which SendToServers cannot outlive. A server
// that challenges forever therefore ends the login with an error, and the AAA
// chain tries the next backend.
func (a *radiusAuthenticator) authenticateEAP(ctx context.Context, request aaa.AuthRequest) (aaa.AuthResult, error) {
	method, ok := a.method.EAPType()
	if !ok {
		return aaa.AuthResult{}, fmt.Errorf("radius: auth method %s runs no EAP conversation", a.method)
	}

	session := eap.NewPeerSession(method, request.Username, request.Password)
	defer session.Close()

	// RFC 3579 Section 2.1: "if the NAS initially sends an EAP-Request/Identity
	// message to the peer, the NAS MUST copy the contents of the Type-Data field
	// of the EAP-Response/Identity received from the peer into the User-Name
	// attribute and MUST include the Type-Data field of the
	// EAP-Response/Identity in the User-Name attribute in every subsequent
	// Access-Request."
	//
	// Identifier 0 opens the conversation. RFC 3748 Section 4.1 binds a
	// Response's Identifier to the Request it answers, and this Request is ze's
	// own, so the pair is consistent by construction. The server's own Requests
	// arrive with its Identifiers, and the peer echoes each one.
	identity := session.Process(&eap.Packet{
		Code:       eap.CodeRequest,
		Identifier: 0,
		Type:       eap.TypeIdentity,
	})
	if identity.Err != nil {
		return aaa.AuthResult{}, fmt.Errorf("radius: EAP identity: %w", identity.Err)
	}
	if identity.Response == nil {
		return aaa.AuthResult{}, errEAPNoResponse
	}
	// The User-Name is the Type-Data the peer put in its own Identity Response,
	// read back from the packet rather than from request.Username, so the two can
	// never drift.
	username := string(identity.Response.TypeData)
	outbound := identity.Response.Encode()

	// State is the server's, opaque, and copied byte for byte. RFC 2865
	// Section 5.24: it "MUST be sent unmodified from the client to the server in
	// the new Access-Request reply to that challenge", and "the client MUST NOT
	// interpret the attribute locally". Nothing below parses it, logs it, or
	// keeps it past the login.
	var state []byte

	for {
		if err := ctx.Err(); err != nil {
			return aaa.AuthResult{}, fmt.Errorf("radius: EAP login budget: %w", err)
		}

		credential, err := eapCredential(outbound, state)
		if err != nil {
			return aaa.AuthResult{}, err
		}
		resp, err := a.exchange(ctx, username, credential)
		if err != nil {
			return aaa.AuthResult{}, err
		}

		// RFC 3579 Section 2.6.4: "the NAS MUST first process the attributes,
		// including the EAP-Message attribute(s), prior to processing the
		// Accept/Reject indication." The peer sees the encapsulated packet here,
		// on EVERY code, before anything below reads resp.Code.
		result, processed, eapErr := a.processEAPMessage(session, resp)

		if resp.Code != CodeAccessChallenge {
			// RFC 3579 Section 2.6.3: "The NAS MUST make its access control
			// decision based solely on the RADIUS Packet Type
			// (Access-Accept/Access-Reject)", and "The access control decision MUST
			// NOT be based on the contents of the EAP packet encapsulated in one or
			// more EAP-Message attributes, if present."
			//
			// So eapErr is REPORTED and dropped. The two can disagree, and the
			// document is written because they do: an Access-Accept carrying an
			// EAP-Failure, or one the peer discarded, is still an Access-Accept, and
			// an Access-Reject carrying an EAP-Success is still a rejection.
			// Returning eapErr here would put the encapsulated packet back in charge
			// of the decision through the error path.
			if eapErr != nil {
				a.logger.Info("RADIUS admin EAP: the concluding EAP packet did not satisfy the peer; the RADIUS code decides",
					"username", username, "code", resp.Code, "error", eapErr)
			}
			return a.result(resp, username)
		}

		if eapErr != nil {
			return aaa.AuthResult{}, eapErr
		}

		if !processed || result.Response == nil {
			// The peer read the challenge and owes no Response: RFC 3748 Section 4.2
			// silently discards several packet shapes, and a discard leaves nothing
			// to send. There is no retransmission to wait for on this path, because
			// the RADIUS reply already arrived, so the login ends and the chain tries
			// the next backend.
			a.logger.Warn("RADIUS admin EAP login ended: the peer has no response to the challenge",
				"username", username, "method", a.method.String())
			return aaa.AuthResult{}, errEAPNoResponse
		}
		outbound = result.Response.Encode()
		state = resp.FindAttr(AttrState)
	}
}

// processEAPMessage hands the reply's encapsulated EAP packet to the peer. The
// second return names the absence: false says the reply carried no EAP-Message
// at all, which is a legal reply and a different thing from a peer that read
// one and answered nothing.
//
// RFC 3579 Section 2.2: "the NAS MUST validate the EAP header fields (Code,
// Identifier, Length) prior to forwarding an EAP packet to or from the RADIUS
// server." eap.DecodePacket is that validation: it refuses a packet shorter
// than four octets, a Length below four, a Length past the octets received, and
// a Request or Response too short to carry its Type field. A packet that fails
// it never reaches the peer.
func (a *radiusAuthenticator) processEAPMessage(session *eap.PeerSession, resp *Packet) (eap.PeerResult, bool, error) {
	encoded, err := eapPacketFrom(resp)
	if err != nil {
		return eap.PeerResult{}, false, err
	}
	if encoded == nil {
		return eap.PeerResult{}, false, nil
	}
	packet, err := eap.DecodePacket(encoded)
	if err != nil {
		return eap.PeerResult{}, false, fmt.Errorf("radius: EAP header from the server: %w", err)
	}
	result := session.Process(packet)
	if result.Err != nil {
		return eap.PeerResult{}, true, fmt.Errorf("radius: EAP exchange: %w", result.Err)
	}
	if result.Notified {
		// RFC 3579 Section 1.2: a displayable message "MUST NOT affect operation of
		// the protocol". It is logged as a value and nothing branches on it, and
		// the peer has already produced the Notification Response the RFC owes.
		a.logger.Info("RADIUS admin EAP notification", "message", result.Notification)
	}
	return result, true, nil
}

// eapCredential builds the credential attributes of one EAP Access-Request:
// the EAP packet in consecutive EAP-Message attributes, a Message-Authenticator
// placeholder, and the server's State when it sent one.
//
// RFC 3579 Section 3.1: "the Message-Authenticator attribute MUST be used to
// protect all Access-Request, Access-Challenge, Access-Accept, and
// Access-Reject packets containing an EAP-Message attribute." The sixteen zero
// octets here are a placeholder: the value cannot be computed until the packet
// is encoded, so encodeRequest (client.go) signs it over the finished bytes and
// refuses to send an EAP-Message packet in which it found nothing to sign.
//
// The State attribute goes LAST, after the EAP-Message run, because RFC 3579
// Section 3.1 requires the EAP-Message attributes to be consecutive and a State
// placed between them would break the run.
func eapCredential(packet, state []byte) ([]Attr, error) {
	attrs, err := appendEAPMessage(make([]Attr, 0, 4), packet)
	if err != nil {
		return nil, err
	}
	attrs = append(attrs, Attr{Type: AttrMessageAuthenticator, Value: make([]byte, AuthenticatorLen)})
	if len(state) > 0 {
		// RFC 2865 Section 5.24: "A packet must have only zero or one State
		// Attribute." One append, of the value the challenge carried, unread.
		attrs = append(attrs, Attr{Type: AttrState, Value: state})
	}
	return attrs, nil
}
