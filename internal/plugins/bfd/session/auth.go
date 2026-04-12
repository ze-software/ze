// Design: rfc/short/rfc5880.md -- Section 6.7 (authentication integration)
// Related: session.go -- Machine state and identity
// Related: fsm.go -- reception procedure that drives Verify
//
// Authentication plumbing for the session Machine. The Machine holds
// a signer (outbound) and a verifier (inbound) plus the RFC 5880
// Section 6.8.1 state variables bfd.XmitAuthSeq and bfd.RcvAuthSeq.
// The concrete crypto lives in internal/plugins/bfd/auth; this file
// wires those into the session's Build and Receive paths.
package session

import (
	"codeberg.org/thomas-mangin/ze/internal/plugins/bfd/auth"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bfd/packet"
)

// AuthPair bundles the signer and verifier built from a session's
// configuration plus the optional persistence helper for the TX
// sequence number. A nil AuthPair means the session runs unauthenticated.
type AuthPair struct {
	Signer    auth.Signer
	Verifier  auth.Verifier
	Persister *auth.SeqPersister
}

// SetAuth installs the authentication state on the Machine. Called
// from Init by the engine when the session config carries an
// `auth { ... }` block. A nil pair clears any previously-installed
// state. Safe to call only before the session's first send or
// receive (normally right after Init).
func (m *Machine) SetAuth(pair *AuthPair) {
	m.authPair = pair
	if pair != nil {
		m.vars.AuthType = pair.Signer.AuthType()
		if pair.Persister != nil {
			m.vars.XmitAuthSeq = pair.Persister.Start()
		}
	} else {
		m.vars.AuthType = 0
	}
}

// HasAuth reports whether the session has an authentication pair
// installed. Called by the engine to decide whether to invoke
// Sign and Verify around the wire path.
func (m *Machine) HasAuth() bool { return m.authPair != nil }

// AuthBodyLen reports the size of the authentication section the
// signer emits, or zero when no signer is installed. Used by
// engine.sendLocked to set Control.Length before calling WriteTo.
func (m *Machine) AuthBodyLen() int {
	if m.authPair == nil {
		return 0
	}
	return m.authPair.Signer.BodyLen()
}

// Sign writes the authentication section at buf[off:] using the
// machine's configured signer and advances bfd.XmitAuthSeq. The
// caller MUST have called Build to populate the Control's Auth bit
// and then written the mandatory section into buf via Control.WriteTo
// BEFORE calling Sign so the digest can cover the full packet.
//
// Returns the number of bytes written. A machine without an
// installed signer returns zero.
func (m *Machine) Sign(buf []byte, off int) int {
	if m.authPair == nil {
		return 0
	}
	seq := m.vars.XmitAuthSeq
	// RFC 5880 §6.7.3: non-meticulous variants leave the sequence
	// unchanged for back-to-back retransmits; the engine's periodic
	// TX loop is the right place to bump it. Meticulous variants
	// advance on every TX via the session; the engine calls
	// AdvanceAuthSeq after Sign.
	n := m.authPair.Signer.Sign(buf, off, seq)
	return n
}

// AdvanceAuthSeq bumps bfd.XmitAuthSeq after a Sign and publishes
// the new value to the persister. Called once per periodic TX from
// engine.sendLocked. RFC 5880 §6.7.3 allows the non-meticulous
// variants to keep the same sequence across back-to-back packets,
// but ze advances on every TX because the extra monotony does not
// break any peer and makes the replay window more conservative.
func (m *Machine) AdvanceAuthSeq() {
	if m.authPair == nil {
		return
	}
	m.vars.XmitAuthSeq++
	if m.authPair.Persister != nil {
		m.authPair.Persister.Store(m.vars.XmitAuthSeq)
	}
}

// Verify runs the verifier against an incoming packet and advances
// bfd.RcvAuthSeq on success. Returns nil when the session has no
// authentication installed (in which case the engine must still
// reject packets whose Auth bit differs from the configured one --
// that check is in Receive, not here).
func (m *Machine) Verify(data []byte, c packet.Control) error {
	if m.authPair == nil {
		return nil
	}
	return m.authPair.Verifier.Verify(data, c, &m.rcvAuthSeq)
}

// CloseAuth releases the persister, if any. Called from the engine's
// session teardown path.
func (m *Machine) CloseAuth() error {
	if m.authPair == nil || m.authPair.Persister == nil {
		return nil
	}
	return m.authPair.Persister.Close()
}
