// Design: docs/architecture/ike/ipsec-9-ikev2-eap-nat.md -- which EAP method an IKEv2
// peer configuration selects
// RFC: rfc/short/rfc3748.md -- Section 7.10, the key-deriving-method constraint on IKEv2
//
// VALIDATES: the two producers that turn an operator's ipsec.AuthMode into an EAP method
// for an IKEv2 exchange reach a method that derives no key only through a mode the
// operator wrote, and that mode announces itself when the configuration is adopted.
// newEAPSession (eap_auth.go) is the producer for the authenticator, the switch in
// startEAPExchange (fsm.go) is the producer for the peer, and warnKeylessEAPModes
// (eap_auth.go) is the adoption warning.
// PREVENTS: an EAP method with no MSK arriving in an IKEv2 exchange by accident. The eap
// package supports MD5-Challenge, which RFC 3748 Section 5.4 records as deriving no key,
// so the framework constructor eap.NewSession accepts Type 4 and cannot be the place this
// is judged. Only these producers can, and the ipsec.AuthMode they read is the one thing
// an operator chooses.
//
// The rule these cases carry is RFC 7296 Section 2.16 (rfc/full/rfc7296.txt:2958): "EAP
// methods that do not establish a shared key SHOULD NOT be used, as they are subject to a
// number of man-in-the-middle attacks". It is a SHOULD NOT about USE, and the sentence
// after it says what to do when such a method IS used, so the method is the operator's to
// select and the selection is one ze reports.

package engine

import (
	"bytes"
	"log/slog"
	"strconv"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/core/eap"
)

// authModeSweepMax bounds the sweep over ipsec.AuthMode. The type is an iota enum
// (internal/component/ike/ipsec/types.go), so the values ipsec names are contiguous from
// zero and every value above them answers "unknown" from String(). The sweep runs to this
// bound so it covers the named modes and the unnamed values above them, and
// declaredAuthModes below fails when the bound would stop inside the enum.
const authModeSweepMax = 32

// methodSelectionPeer is the peer name every case configures. It is asserted in the
// adoption warning, so an operator reading that line learns which peer carries the mode.
const methodSelectionPeer = "method-selection"

// keylessMethodSentence is the RFC 7296 Section 2.16 sentence the adoption warning MUST
// carry, quoted from rfc/full/rfc7296.txt:2958. It is written out here rather than read
// from the warning, because a test that took the words from the code it judges would pass
// over any words at all.
const keylessMethodSentence = "EAP methods that do not establish a shared key SHOULD NOT be used, " +
	"as they are subject to a number of man-in-the-middle attacks"

// declaredAuthModes returns every ipsec.AuthMode the ipsec package names, read from that
// package's own String() rather than from a list copied here. A second copy of the enum
// would disagree with it the day a mode is added (ai/rules/principles.md).
func declaredAuthModes(t *testing.T) []ipsec.AuthMode {
	t.Helper()

	modes := make([]ipsec.AuthMode, 0, authModeSweepMax)
	for raw := range authModeSweepMax {
		mode := ipsec.AuthMode(raw)
		if authModeIsDeclared(mode) {
			modes = append(modes, mode)
		}
	}
	if len(modes) == 0 {
		t.Fatal("the ipsec package names no auth mode, so a sweep over the named modes asserts nothing")
	}
	if last := int(modes[len(modes)-1]); last >= authModeSweepMax-1 {
		t.Fatalf("the highest named auth mode is %d and the sweep stops at %d, so the sweep may end inside the enum",
			last, authModeSweepMax-1)
	}
	return modes
}

// authModeIsDeclared reports whether the ipsec package names this value, which is the
// same question as "can an operator write it in a configuration": ipsec.ParseAuthMode
// reads the same table String() answers from.
func authModeIsDeclared(mode ipsec.AuthMode) bool {
	return mode.String() != ipsec.AuthUnknown.String()
}

// authModeCaseName names a sub-test after the mode it runs. The numeric value leads,
// because every unnamed value in the sweep spells itself "unknown".
func authModeCaseName(mode ipsec.AuthMode) string {
	return strconv.Itoa(int(mode)) + "-" + mode.String()
}

// ikev2MethodSelectionConfig is a method configuration that satisfies every EAP method
// the eap package builds: a password for MD5-Challenge and MS-CHAPv2, and a server
// certificate for EAP-TLS.
//
// It is deliberately permissive. A mode that selects MD5-Challenge must reach a live
// session and be read through DerivesKey, rather than be hidden by a constructor that
// failed for want of a credential.
func ikev2MethodSelectionConfig(t *testing.T) eap.MethodConfig {
	t.Helper()

	pki := newESCPKI(t)
	return eap.MethodConfig{
		Password:      "ikev2-method-selection-secret",
		ServerCertPEM: pki.certPEM,
		ServerKeyPEM:  pki.keyPEM,
		CACertPEM:     pki.caPEM,
	}
}

// TestRFC3748IKEv2EAPModesSelectAKeyDerivingMethod checks that every auth mode ze offers
// as an EAP mode is usable inside IKEv2, and that all but one of them key the AUTH
// payload from an MSK.
//
// The method is a sub-test for each mode ipsec.IsEAPMode names among the modes the ipsec
// package declares. Each one is handed to newEAPSession with a configuration that
// satisfies every EAP method, and the session it returns is asked DerivesKey.
func TestRFC3748IKEv2EAPModesSelectAKeyDerivingMethod(t *testing.T) {
	// RFC requirement: RFC3748-7.10-3 positive -- every ipsec.AuthMode that
	// ipsec.IsEAPMode names is accepted by newEAPSession (eap_auth.go), the producer
	// that picks the EAP method of an IKEv2 exchange for the authenticator, and the
	// eap.Session it returns answers true to DerivesKey for every one of those modes
	// except ipsec.AuthEAPMD5, which is required only to be accepted because RFC 3748
	// Section 5.4 gives MD5-Challenge no key derivation.
	config := ikev2MethodSelectionConfig(t)

	eapModes, keyDerivingModes := 0, 0
	for _, mode := range declaredAuthModes(t) {
		if !ipsec.IsEAPMode(mode) {
			continue
		}
		eapModes++
		if mode != ipsec.AuthEAPMD5 {
			keyDerivingModes++
		}
		t.Run(authModeCaseName(mode), func(t *testing.T) {
			session, err := newEAPSession(mode, config)
			if err != nil {
				t.Fatalf("newEAPSession refused the EAP auth mode %s: %v", mode, err)
			}
			t.Cleanup(session.Close)
			if mode == ipsec.AuthEAPMD5 {
				return
			}
			if !session.DerivesKey() {
				t.Fatalf("auth mode %s selected an EAP method that derives no key", mode)
			}
		})
	}
	if eapModes == 0 {
		t.Error("no auth mode reports ipsec.IsEAPMode, so no sub-test above ran")
	}
	// Without this the DerivesKey assertion is satisfied by a tree in which every EAP
	// mode is the excluded one.
	if keyDerivingModes == 0 {
		t.Error("every EAP auth mode is the excluded ipsec.AuthEAPMD5, so no DerivesKey assertion ran")
	}
}

// TestRFC3748IKEv2NoAuthModeSelectsAKeylessMethod checks that an EAP method with no MSK
// enters an IKEv2 exchange only through a mode the operator named, and that adopting such
// a configuration says so.
//
// The method is a sweep over every value of ipsec.AuthMode up to authModeSweepMax, past
// the last value the ipsec package names, with one sub-test per value. Each value is
// handed to both producers that choose a method for IKEv2, and the value that reaches a
// method deriving no key must be a mode the ipsec package names and must draw the
// adoption warning. Every other value must draw none.
func TestRFC3748IKEv2NoAuthModeSelectsAKeylessMethod(t *testing.T) {
	// RFC requirement: RFC3748-7.10-3 negative -- the body sweeps ipsec.AuthMode from
	// zero to authModeSweepMax, which declaredAuthModes checks is past the last value
	// ipsec names, and asks both producers what each value selects: the authenticator
	// producer newEAPSession (eap_auth.go) and the peer producer startEAPExchange
	// (fsm.go). A value the ipsec package does not name starts no session at either
	// producer. A value whose session answers false to DerivesKey is a value the ipsec
	// package names, and warnKeylessEAPModes (eap_auth.go) writes one line for it
	// carrying the peer name, the mode name and the RFC 7296 Section 2.16 sentence
	// quoted in keylessMethodSentence; every other value draws no line at all. The body
	// also counts the sessions each producer started and the keyless selections it saw,
	// so a producer that selects nothing, and a sweep that never reaches a keyless
	// method, do not satisfy it.
	config := ikev2MethodSelectionConfig(t)

	authenticatorSessions, peerSessions, keylessSelections := 0, 0, 0
	for raw := range authModeSweepMax {
		mode := ipsec.AuthMode(raw)
		t.Run(authModeCaseName(mode), func(t *testing.T) {
			authenticatorKeyless, authenticatorStarted := authenticatorSelection(t, mode, config)
			peerKeyless, peerStarted := peerSelection(t, mode)
			if authenticatorStarted {
				authenticatorSessions++
			}
			if peerStarted {
				peerSessions++
			}

			assertUndeclaredModeStartsNoSession(t, mode, authenticatorStarted, peerStarted)

			keyless := authenticatorKeyless || peerKeyless
			if keyless {
				keylessSelections++
			}
			assertKeylessModeIsNamedAndAnnounced(t, mode, keyless)
		})
	}

	// Each producer must have started at least one session. Without this the assertions
	// above are satisfied by a producer that selects no method for any mode at all.
	if authenticatorSessions == 0 {
		t.Error("newEAPSession started no EAP session for any auth mode in the sweep")
	}
	if peerSessions == 0 {
		t.Error("startEAPExchange started no EAP peer session for any auth mode in the sweep")
	}
	// And one mode must have reached a method that derives no key. Without this the
	// warning assertion never runs and the case proves only that nothing is warned about.
	if keylessSelections == 0 {
		t.Error("no auth mode in the sweep selected an EAP method that derives no key, " +
			"so the adoption warning was never asserted")
	}
}

// assertUndeclaredModeStartsNoSession checks that a value the ipsec package does not
// name reaches no EAP method at either producer.
//
// It is the half of the sweep that assertKeylessModeIsNamedAndAnnounced cannot carry.
// That helper branches on whether a session turned out to be keyless, so a value the
// ipsec package never named, that nevertheless started a KEY-DERIVING session, walks
// past it unremarked. Such a value is an unconfigurable EAP exchange: nothing an
// operator wrote chose the method, and RFC 3748 Section 7.10 makes the method choice
// the thing that decides whether the exchange carries an MSK at all.
func assertUndeclaredModeStartsNoSession(t *testing.T, mode ipsec.AuthMode, authenticatorStarted, peerStarted bool) {
	t.Helper()

	if authModeIsDeclared(mode) {
		return
	}
	if authenticatorStarted {
		t.Errorf("auth mode %d is a value the ipsec package does not name and newEAPSession still "+
			"started an EAP session for it", int(mode))
	}
	if peerStarted {
		t.Errorf("auth mode %d is a value the ipsec package does not name and startEAPExchange still "+
			"started an EAP peer session for it", int(mode))
	}
}

// assertKeylessModeIsNamedAndAnnounced checks the two things ze owes for a mode that
// selects a method deriving no key: the mode is one an operator can write, and adopting a
// configuration that carries it writes the warning. For every other mode it checks that
// the adoption writes nothing.
func assertKeylessModeIsNamedAndAnnounced(t *testing.T, mode ipsec.AuthMode, keyless bool) {
	t.Helper()

	logged := adoptionLog(t, mode)
	if !keyless {
		if logged != "" {
			t.Fatalf("auth mode %s selects no keyless EAP method and the adoption still warned: %s", mode, logged)
		}
		return
	}
	if !authModeIsDeclared(mode) {
		t.Fatalf("auth mode %d selected an EAP method that derives no key and the ipsec package names no such mode, "+
			"so nothing an operator wrote chose it", int(mode))
	}
	if logged == "" {
		t.Fatalf("auth mode %s selected an EAP method that derives no key and adopting it warned about nothing", mode)
	}
	for _, want := range []string{methodSelectionPeer, mode.String(), keylessMethodSentence} {
		if !strings.Contains(logged, want) {
			t.Errorf("the adoption warning for auth mode %s does not carry %q: %s", mode, want, logged)
		}
	}
}

// adoptionLog runs the adoption warning over a configuration whose one peer carries mode,
// and answers what it wrote at warning level.
func adoptionLog(t *testing.T, mode ipsec.AuthMode) string {
	t.Helper()

	var logged bytes.Buffer
	warnKeylessEAPModes(&ipsec.IPsecConfig{
		Peers: map[string]ipsec.SiteToSitePeer{
			methodSelectionPeer: {Auth: ipsec.AuthConfig{Mode: mode}},
		},
	}, slog.New(slog.NewTextHandler(&logged, &slog.HandlerOptions{Level: slog.LevelWarn})))
	return logged.String()
}

// authenticatorSelection asks newEAPSession for the method mode selects. It reports
// whether that method derives no key, and whether a session was created at all.
func authenticatorSelection(t *testing.T, mode ipsec.AuthMode, config eap.MethodConfig) (bool, bool) {
	t.Helper()

	session, err := newEAPSession(mode, config)
	if err != nil {
		return false, false
	}
	t.Cleanup(session.Close)
	return !session.DerivesKey(), true
}

// peerSelection runs the peer's method switch for mode. It reports whether the method it
// started derives no key, and whether a session was started at all.
//
// The EAP payload carries an EAP-Success, which RFC 3748 Section 4.2 makes the peer
// discard at the start of a conversation. The switch has already chosen the method and
// stored the session on the SA by then, and no response leaves the peer, so no transport
// is needed to read the choice back.
//
// EAP-TLS starts no session here, because buildPeerTLSConfig (fsm.go) needs a client
// certificate in the PKI store and this SA names none. That is the "no method at all"
// outcome, which this test accepts; the EAP-TLS peer path is exercised by
// TestForgetKeysClosesInitiatorEAPSession.
func peerSelection(t *testing.T, mode ipsec.AuthMode) (bool, bool) {
	t.Helper()

	sa := &SA{
		PeerName: methodSelectionPeer,
		PeerCfg: ipsec.SiteToSitePeer{
			Auth: ipsec.AuthConfig{
				Mode:    mode,
				PSK:     "ikev2-method-selection-secret",
				LocalID: "method-selection-user",
			},
		},
	}
	startEAPExchange(sa, &wire.PayloadEAP{Code: eap.CodeSuccess, Identifier: 1}, nil, slog.New(slog.DiscardHandler))
	if sa.EAPSession == nil {
		return false, false
	}

	peer, ok := sa.EAPSession.(*eap.PeerSession)
	if !ok {
		t.Fatalf("auth mode %s left a %T on the SA, and only an *eap.PeerSession can say which method it runs",
			mode, sa.EAPSession)
	}
	t.Cleanup(peer.Close)
	return !peer.DerivesKey(), true
}
