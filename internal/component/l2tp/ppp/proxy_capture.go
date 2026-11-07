// Design: docs/architecture/l2tp/followup-l2tp-call.md -- realm relay, proxy LCP capture at the LAC
// RFC: rfc/short/rfc2661.md -- RFC 2661 Section 4.4.5 (Proxy LCP AVPs 26 to 28)
// Related: session_run.go -- handleLCPPacket, sendConfigureAck and sendConfigureNakOrReject feed it
// Related: auth.go -- awaitAuthDecision copies it into EventAuthRequest

package ppp

import "bytes"

// lcpCapture records the LCP exchange a LAC relays to the LNS when the
// subscriber's realm names a relay binding: the option bodies of the first
// Configure-Request received, and of the Configure-Request pair that was
// Acked. "Final" in RFC 2661 Section 4.4.5 means the requests that completed
// the negotiation, so each Ack overwrites the pair and the last Ack before
// Opened wins. Owned by the session goroutine; not safe for concurrent use.
type lcpCapture struct {
	initialReceived []byte
	initialSet      bool
	lastSent        []byte
	lastReceived    []byte
}

// noteRequestAnswered records a Configure-Request the session answered (Ack,
// Nak or Reject), so a request the automaton discarded never becomes AVP 26.
//
// RFC 2661 Section 4.4.5: "LCP CONFREQ is a copy of the body of the initial
// CONFREQ received, starting at the first option within the body of the LCP
// message."
func (c *lcpCapture) noteRequestAnswered(req LCPPacket) {
	if c == nil || c.initialSet {
		return
	}
	c.initialReceived = bytes.Clone(req.Data)
	c.initialSet = true
}

// noteRequestAcked records the subscriber's Configure-Request Ze Acked.
//
// RFC 2661 Section 4.4.5: "The LCP CONFREQ is a copy of the body of the
// final CONFREQ received from the client to complete LCP negotiation,
// starting at the first option within the body of the LCP message."
func (c *lcpCapture) noteRequestAcked(req LCPPacket) {
	if c == nil {
		return
	}
	c.lastReceived = bytes.Clone(req.Data)
}

// noteOwnRequestAcked records Ze's own Configure-Request the subscriber
// Acked. sent is the option body of the request the Ack matched.
//
// RFC 2661 Section 4.4.5: "The LCP CONFREQ is a copy of the body of the
// final CONFREQ sent to the client to complete LCP negotiation, starting at
// the first option within the body of the LCP message."
func (c *lcpCapture) noteOwnRequestAcked(sent []byte) {
	if c == nil {
		return
	}
	c.lastSent = bytes.Clone(sent)
}

// fill copies the capture into an authentication request. Each slice is a
// fresh copy, so the consumer may keep it after the session moves on.
func (c *lcpCapture) fill(req *EventAuthRequest) {
	if c == nil {
		return
	}
	req.ProxyLCP = &ProxyLCPCapture{
		InitialReceived: bytes.Clone(c.initialReceived),
		LastSent:        bytes.Clone(c.lastSent),
		LastReceived:    bytes.Clone(c.lastReceived),
	}
}

// ProxyLCPCapture is the LCP exchange of a session started with
// StartSession.CaptureProxyLCP, in the form RFC 2661 Section 4.4.5 carries
// it: each field is the option body of one Configure-Request.
type ProxyLCPCapture struct {
	InitialReceived []byte
	LastSent        []byte
	LastReceived    []byte
}
