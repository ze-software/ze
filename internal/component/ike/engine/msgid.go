// Design: plan/learned/1069-ipsec-13-rekey-wire.md -- RFC 7296 §2.3 message-ID handling
// RFC: rfc/short/rfc7296.md -- Message IDs and windows (Section 2.3)

package engine

import (
	"math"
	"time"

	"github.com/ze-software/ze/internal/component/ike/crypto"
	"github.com/ze-software/ze/internal/component/ike/wire"
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

// requestWindowTimeout bounds how long one self-initiated request holds the window
// of RFC 7296 §2.3. A Delete is best-effort and is never resent. A lost answer to
// one would otherwise stop every later request on this SA. It would stop the DPD
// probe with them.
//
// A rekey bounds its own hold through its retransmissions. A DPD probe bounds its
// own hold in two ways. The liveness budget ends it. An authenticated inbound also
// retires the probe, and maintainSA frees the window in the same step
// (retireRequest, below). Only a Delete therefore reaches this value.
const requestWindowTimeout = 30 * time.Second

// msgIDRekeyThreshold is the Message ID at which an SA asks for an IKE SA rekey
// rather than waiting for its counter to run out. RFC 7296 §2.2 allows the SA to be
// closed OR rekeyed, and a rekey keeps the tunnel up. The 4096 ids of headroom leave
// room for the rekey request, its retransmissions, and the Delete that follows it,
// so none of them meets the ceiling.
const msgIDRekeyThreshold uint32 = math.MaxUint32 - 4096

// advanceMsgID moves the outbound Message ID past the request the caller just built.
//
// RFC 7296 §2.2 MUST: "In the unlikely event that Message IDs grow too large to fit
// in 32 bits, the IKE SA MUST be closed or rekeyed." At the ceiling the counter stays
// where it is and the SA is marked exhausted. It never wraps to 0, so no id is ever
// spent twice under one set of keys. The maintainSA ticker closes the SA on the flag.
func (sa *SA) advanceMsgID() {
	if sa.NextMsgID == math.MaxUint32 {
		sa.msgIDExhausted = true
		return
	}
	sa.NextMsgID++
}

// resumeRequestsAfter points the outbound request counter past a peer request this side
// has just answered, and is the responder's counterpart to advanceMsgID.
//
// The responder raises no request of its own during the handshake. Its IKE_SA_INIT and
// IKE_AUTH messages are responses. §2.2 makes a response carry the Message ID of the
// request it answers, so NextMsgID is SET here rather than incremented. The first
// self-initiated request on the new SA then takes the next free id. That request is a DPD
// probe, a Delete, or a rekey. §2.2: "the first pair of IKE_AUTH messages will have an ID
// of 1, the second (when EAP is used) will be 2, and so on."
//
// It shares advanceMsgID's ceiling. That is the whole point of routing the write through
// here. §2.2 MUST: "In the unlikely event that Message IDs grow too large to fit in 32
// bits, the IKE SA MUST be closed or rekeyed." A plain `NextMsgID = msgID + 1` wraps to 0
// for an authenticated peer that answers at math.MaxUint32. That spends id 0 a second time
// under one set of keys. It hands back the replay protection §2.2 names. The counter
// freezes at the ceiling and the SA is marked exhausted instead. maintainSA closes it on
// the flag.
func (sa *SA) resumeRequestsAfter(msgID uint32) {
	if msgID == math.MaxUint32 {
		sa.NextMsgID = math.MaxUint32
		sa.msgIDExhausted = true
		return
	}
	sa.NextMsgID = msgID + 1
}

// advanceExpectedMsgID moves the inbound Message ID past the peer request the caller
// just answered. It uses the same ceiling as advanceMsgID, for the same reason. The
// peer drives this counter, so a wrap here lets it replay its whole request sequence
// from id 0 under the same keys. RFC 7296 §2.2 calls the Message ID replay protection.
func (sa *SA) advanceExpectedMsgID(msgID uint32) {
	if msgID == math.MaxUint32 {
		sa.msgIDExhausted = true
		return
	}
	sa.ExpectedMsgID = msgID + 1
}

// msgIDNearExhaustion reports whether either counter has climbed into the headroom
// below the ceiling. maintainSA answers it with an IKE SA rekey, and RFC 7296 §2.18
// starts the replacement SA with both counters at 0.
func (sa *SA) msgIDNearExhaustion() bool {
	return sa.NextMsgID >= msgIDRekeyThreshold || sa.ExpectedMsgID >= msgIDRekeyThreshold
}

// reserveRequestWindow claims the one outstanding-request slot for the request the
// caller is about to build. RFC 7296 §2.3 allows one request in flight per SA, so a
// caller that reads false MUST defer its request rather than send it. The window
// expects an answer at the current NextMsgID, which is the id the caller's request
// carries. Reserve immediately before the build, and release the window when the
// build fails. Owned by the maintainSA loop, so it needs no lock.
//
// An exhausted SA reads false as well. RFC 7296 §2.2 leaves it no id to carry a
// request, so no later request is built on it. That alone would leave the SA quiet
// rather than closed, so maintainSA reads the same flag and closes the SA.
func (sa *SA) reserveRequestWindow() bool {
	if sa.requestOutstanding || sa.msgIDExhausted {
		return false
	}
	sa.requestOutstanding = true
	sa.requestMsgID = sa.NextMsgID
	sa.requestSentAt = time.Now()
	return true
}

// releaseRequestWindow frees the window without an answer. A caller uses it when its
// build failed, or when it abandons its own exchange.
func (sa *SA) releaseRequestWindow() {
	sa.requestOutstanding = false
}

// answerAuthenticatedResponse frees the window when msgID is the id of the
// outstanding request. A stale or replayed answer names another id, so it leaves the
// window held.
//
// The caller MUST have authenticated the response first. RFC 7296 §2.3 lets the next
// request go out once this returns, so a datagram that only resembles an answer must
// never reach it. strongSwan orders it the same way: process_response clears its slot
// after parse_body verifies the message.
//
// Two call sites exist, and both sit after a successful decryptAndParse in
// handleOwnedInbound (inbound.go). One serves a response that leaves no pendingRekey,
// which is a DPD probe or a Delete. The other serves the rekey response. A third
// response path MUST call this too, or its request holds the window until
// serviceRequestWindow frees it.
func (sa *SA) answerAuthenticatedResponse(msgID uint32) {
	if !sa.requestOutstanding || sa.requestMsgID != msgID {
		return
	}
	sa.requestOutstanding = false
}

// retireRequest frees the window when msgID is the id of the outstanding request and
// the caller has abandoned that request. Unlike answerAuthenticatedResponse it is not
// driven by an answer, so the caller MUST have decided the request has no further
// purpose.
//
// One caller exists: maintainSA retires the DPD probe when an authenticated inbound
// message proves the peer alive (RFC 7296 §2.4). handleDPDResponse already drops the
// stored datagram there, which makes a retransmission impossible, so the probe is
// abandoned either way. This makes the window bookkeeping match that abandonment.
// Without it, a peer REQUEST arriving while our probe is unanswered strands the window
// for the whole requestWindowTimeout, and the SA raises no request at all in that time.
func (sa *SA) retireRequest(msgID uint32) {
	if !sa.requestOutstanding || sa.requestMsgID != msgID {
		return
	}
	sa.requestOutstanding = false
}

// requestWindowStale reports whether the outstanding request has waited past
// requestWindowTimeout. Only a caller that knows no other timer covers the holder
// acts on it (serviceRequestWindow, established.go).
func (sa *SA) requestWindowStale(now time.Time) bool {
	return sa.requestOutstanding && now.Sub(sa.requestSentAt) >= requestWindowTimeout
}

// recordPeerWindowSize stores the window the peer promised to keep, read from the
// SET_WINDOW_SIZE notify of its IKE_AUTH message. A nil notify leaves PeerWindowSize
// at zero, which RFC 7296 §2.3 reads as a window of one.
//
// RFC 7296 §2.3 gives the notification a fixed body. The data MUST be 4 octets long,
// and it MUST hold the big-endian count of messages the sender promises to keep. The
// checklist row RFC7296-2.3-7 carries the sentence verbatim.
//
// A body of any other length is refused rather than ignored. Ze never acts on a
// notification whose length the RFC fixes (ai/rules/exact-or-reject.md). The caller
// ends the IKE_AUTH on the error.
//
// The value bounds what Ze MAY SEND and never what Ze ACCEPTS. Ze sends no
// SET_WINDOW_SIZE of its own, so its declared window stays one and classifyInbound
// keeps accepting exactly one request id.
func recordPeerWindowSize(sa *SA, notify *wire.PayloadNotify) error {
	if notify == nil {
		return nil
	}
	window, err := wire.ParseSetWindowSize(notify.NotificationData)
	if err != nil {
		return err
	}
	sa.PeerWindowSize = window
	return nil
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
		// This classifier runs before the message is authenticated, so it never frees
		// the request window of RFC 7296 §2.3. answerAuthenticatedResponse does that,
		// from the two post-decrypt sites in handleOwnedInbound (inbound.go).
		if pending != nil && msgID == pending.messageID {
			return inboundResponse
		}
		return inboundInvalid
	}
	// The retransmit test runs first. Under normal operation the two ids differ by one,
	// because advanceExpectedMsgID sets ExpectedMsgID to lastResponseID plus one, so
	// the order changes nothing. They become equal at the 32-bit ceiling, where the
	// counter freezes (RFC 7296 §2.2). A peer retransmit of that last request must
	// still replay the cached response there. RFC 7296 §2.1 forbids reprocessing it.
	if sa.lastResponseSet && msgID == sa.lastResponseID {
		return inboundRetransmit
	}
	if msgID == sa.ExpectedMsgID {
		return inboundNewRequest
	}
	return inboundInvalid
}

// cacheResponse records the response we sent for request msgID and advances the
// expected request counter. A later retransmit of msgID replays sa.lastResponse
// without reprocessing (RFC 7296 §2.3).
//
// The counter advances through advanceExpectedMsgID, so a peer that drives it to the
// 32-bit ceiling marks the SA exhausted instead of wrapping it to 0 (RFC 7296 §2.2).
// The cached response stays, so the peer's retransmit of that last request is still
// answered.
func cacheResponse(sa *SA, msgID uint32, resp []byte) {
	sa.lastResponse = resp
	sa.lastResponseID = msgID
	sa.lastResponseSet = true
	sa.advanceExpectedMsgID(msgID)
}
