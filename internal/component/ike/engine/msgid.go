// Design: plan/learned/1069-ipsec-13-rekey-wire.md -- RFC 7296 §2.3 message-ID handling
// RFC: rfc/short/rfc7296.md -- Message IDs and windows (Section 2.3)

package engine

import (
	"time"

	"github.com/ze-software/ze/internal/component/ike/crypto"
)

// rekeyKind distinguishes a Child SA rekey (CREATE_CHILD_SA with REKEY_SA on an
// ESP SA) from an IKE SA rekey (CREATE_CHILD_SA rekeying the IKE SA itself).
type rekeyKind int

const (
	rekeyChild rekeyKind = iota
	rekeyIKE
)

// pendingRekey tracks a CREATE_CHILD_SA exchange this side initiated and is
// awaiting a response for. It is owned exclusively by the maintainSA goroutine,
// so it needs no lock. RFC 7296 §2.3 allows one outstanding request per SA, so
// there is at most one pendingRekey at a time.
type pendingRekey struct {
	kind        rekeyKind
	messageID   uint32 // message ID of the request we sent (response must echo it)
	sentMsg     []byte // full wire bytes, for retransmit
	sentAt      time.Time
	retransmits int
	localNonce  []byte // Ni we sent

	// Child SA rekey.
	newInboundSPI uint32   // our proposed ESP SPI
	oldChild      *ChildSA // the Child SA being replaced

	// IKE SA rekey.
	newInitiatorSPI [8]byte            // our proposed new IKE SPI
	dh              *crypto.DHExchange // our DH half, kept until the response supplies KEr
}

// clear releases any DH key material held by a pending IKE SA rekey.
func (p *pendingRekey) clear() {
	if p == nil {
		return
	}
	if p.dh != nil {
		p.dh.Clear()
		p.dh = nil
	}
}

// inboundClass classifies an established-SA message against the RFC 7296 §2.3
// window (size 1): the next expected peer request, a retransmit of the previous
// request, a response to our outstanding request, or an out-of-window message.
type inboundClass int

const (
	inboundInvalid    inboundClass = iota // out-of-window / unexpected -> drop
	inboundNewRequest                     // process, advance ExpectedMsgID, cache response
	inboundRetransmit                     // duplicate request -> resend cached response
	inboundResponse                       // response to our outstanding request
)

// classifyInbound decides how to treat an inbound established-SA message.
// RFC 7296 §2.3: a response is matched to the outstanding request by message ID;
// a request is accepted only at ExpectedMsgID, and a repeat of the previous
// request ID is a retransmit answered from cache.
func classifyInbound(sa *SA, msgID uint32, isResponse bool, pending *pendingRekey) inboundClass {
	if isResponse {
		if pending != nil && msgID == pending.messageID {
			return inboundResponse
		}
		return inboundInvalid
	}
	if msgID == sa.ExpectedMsgID {
		return inboundNewRequest
	}
	if sa.lastResponseSet && msgID == sa.lastResponseID {
		return inboundRetransmit
	}
	return inboundInvalid
}

// cacheResponse records the response we sent for request msgID and advances the
// expected request counter. A later retransmit of msgID replays sa.lastResponse
// without reprocessing (RFC 7296 §2.3).
func cacheResponse(sa *SA, msgID uint32, resp []byte) {
	sa.lastResponse = resp
	sa.lastResponseID = msgID
	sa.lastResponseSet = true
	sa.ExpectedMsgID = msgID + 1
}
