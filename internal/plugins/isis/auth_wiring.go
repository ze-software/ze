// Design: plan/learned/935-isis-10-auth.md -- engine <-> authentication wiring.
// Related: auth_keystore.go -- the key store this builds from resolved config
// Related: server.go -- the engine struct, dispatcher, and the metric this owns
//
// The sign/verify crypto backend lives in the packet subpackage
// (packet.SignPDU / packet.VerifyPDU); this file only selects the chain/key and
// routes bytes.
//
// RFC: rfc/short/rfc5304.md -- HMAC-MD5; LSP zeroing; authenticated purges (sec 2)
// RFC: rfc/short/rfc5310.md -- generic crypto / HMAC-SHA; per-SA Key ID (sec 3)
//
// This file is the root-package glue between the resolved config (config.go key
// chains, owned by isis-4) and the two shared hook points the spec calls for:
// sign on TX (per PDU class) and verify on RX (one chokepoint at the dispatcher).
// It builds the key store, installs the per-level LSP/SNP signer (Originator +
// Flooder) and the per-interface IIH signer (each Circuit), and installs the
// dispatcher verify hook that rejects a failed PDU and increments
// ze_isis_auth_failures_total. All crypto lives in the packet backend; this file
// only selects the chain/key and routes the bytes.

package isis

import (
	"time"

	"github.com/ze-software/ze/internal/plugins/isis/adjacency"
	"github.com/ze-software/ze/internal/plugins/isis/circuit"
	"github.com/ze-software/ze/internal/plugins/isis/lsdb"
	"github.com/ze-software/ze/internal/plugins/isis/packet"
	"github.com/ze-software/ze/internal/plugins/isis/transport"
)

// setKeyStore rebuilds the authentication key store from cfg and installs (or
// updates) the sign/verify hooks. Called from setConfig on startup and on every
// reload so a key-chain change takes effect without a restart (hitless rotation,
// spec AC-4). When no key chain is configured the verify hook is detached and the
// signers are nil (unauthenticated operation, the default).
func (e *engine) setKeyStore(cfg Config) {
	ks := newKeyStore(cfg)
	e.ksMu.Lock()
	e.keystore = ks
	e.ksMu.Unlock()

	// Install the per-level signers on the LSDB subsystems (LSP at origination,
	// CSNP/PSNP at build). These pick the area key (L1) / domain key (L2).
	if e.originator != nil {
		if ks.configured() {
			e.originator.SetSigner(e.signLevelPDU)
		} else {
			e.originator.SetSigner(nil)
		}
	}
	if e.flooder != nil {
		if ks.configured() {
			e.flooder.SetSigner(e.signLevelPDU)
		} else {
			e.flooder.SetSigner(nil)
		}
	}

	// Install the RX verify hook (one chokepoint at the dispatcher). Detach it
	// when no auth is configured so the receive path stays allocation-free.
	if ks.configured() {
		e.dispatch.setVerify(e.verifyFrame)
	} else {
		e.dispatch.setVerify(nil)
	}
}

// installCircuitSigner wires the per-interface (IIH) signer onto a freshly built
// circuit (called from buildCircuit). The signer picks the circuit's per-level
// IIH chain. A no-op when no auth is configured.
func (e *engine) installCircuitSigner(c *circuit.Circuit) {
	if c == nil {
		return
	}
	e.ksMu.RLock()
	configured := e.keystore.configured()
	e.ksMu.RUnlock()
	if !configured {
		return
	}
	name := c.Name()
	c.SetSigner(func(level adjacency.Level, pdu []byte) []byte {
		return e.signHelloPDU(name, level, pdu)
	})
}

// signLevelPDU signs an LSP/CSNP/PSNP using the per-level chain (area key for L1,
// domain key for L2). The level and PDU class are derived from the PDU type in
// the bytes. When no chain is set or no key is currently valid for signing, the
// PDU is returned unchanged (a peer under auth will then reject it -- the operator
// sees the failure; we never send a half-signed PDU).
func (e *engine) signLevelPDU(pdu []byte) []byte {
	level, ok := levelOfPDU(pdu)
	if !ok {
		return pdu
	}
	e.ksMu.RLock()
	chain := e.keystore.levelChain(level)
	key, has := e.keystore.signKey(chain, time.Now())
	e.ksMu.RUnlock()
	if !has {
		return pdu
	}
	return signOrUnchanged(pdu, key)
}

// signHelloPDU signs an IIH using the per-interface chain for the adjacency level
// (the Link Level Authentication string, RFC 5304 sec 2). Returns the PDU
// unchanged when no IIH chain is set on this circuit/level.
func (e *engine) signHelloPDU(iface string, level adjacency.Level, pdu []byte) []byte {
	e.ksMu.RLock()
	chain := e.keystore.helloChain(iface, adjToKSLevel(level))
	key, has := e.keystore.signKey(chain, time.Now())
	e.ksMu.RUnlock()
	if !has {
		return pdu
	}
	return signOrUnchanged(pdu, key)
}

// signOrUnchanged signs pdu with key; on any signing error it returns the PDU
// unchanged (a malformed/unsupported case must never panic the TX path -- the
// peer then rejects the unsigned PDU and the operator sees the failure).
func signOrUnchanged(pdu []byte, key packet.Key) []byte {
	signed, err := packet.SignPDU(pdu, key)
	if err != nil {
		return pdu
	}
	return signed
}

// verifyFrame authenticates a received frame against the relevant chain and
// returns true to accept. On rejection it increments ze_isis_auth_failures_total
// {level,interface} and returns false (the dispatcher drops the frame before any
// adjacency/LSDB/SNP processing). A PDU class with no configured chain accepts
// (VerifyPDU with no candidate keys returns nil).
func (e *engine) verifyFrame(rf transport.RawFrame) bool {
	if len(rf.PDU) <= offPDUType {
		return false
	}
	pt := packet.PDUType(rf.PDU[offPDUType] & pduTypeMask)
	level, ok := levelOfPDUType(pt)
	if !ok {
		// A type we cannot classify by level (should not reach here): let the
		// handler's own decode reject it; do not authenticate.
		return true
	}
	iface := e.ifaceNameFor(rf.IfIndex)

	e.ksMu.RLock()
	var keys []packet.Key
	switch {
	case pt == packet.PDUTypeP2PHello:
		// RFC 5303 sec 3: a P2P IIH is level-agnostic on the wire (one PDU type, no
		// level bit), so the receiver cannot know the negotiated level from the
		// bytes. The sender (sendP2PHello) signs with the NEGOTIATED level's IIH
		// chain, which on an L1L2 circuit may be L1 or L2 and the two chains may
		// differ. Accept keys from BOTH per-interface IIH chains so a correctly
		// signed P2P Hello verifies regardless of which level negotiated; the
		// per-key HMAC still rejects a forged or wrong-secret PDU.
		now := time.Now()
		keys = e.keystore.verifyKeys(e.keystore.helloChain(iface, levelOne), now)
		keys = append(keys, e.keystore.verifyKeys(e.keystore.helloChain(iface, levelTwo), now)...)
	case isIIHType(pt):
		keys = e.keystore.verifyKeys(e.keystore.helloChain(iface, level), time.Now())
	default:
		keys = e.keystore.verifyKeys(e.keystore.levelChain(level), time.Now())
	}
	e.ksMu.RUnlock()

	if err := packet.VerifyPDU(rf.PDU, keys); err != nil {
		e.ksMu.RLock()
		fail := e.authFailures
		e.ksMu.RUnlock()
		fail.With(ksLevelToken(level), iface).Inc()
		e.log.Debug("isis: auth verification failed",
			"interface", iface, "level", ksLevelToken(level), "reason", err.Error())
		return false
	}
	return true
}

// ifaceNameFor maps a received frame's source ifindex to the circuit name (for
// the per-interface IIH chain and the auth-failure metric label). Returns "" when
// the ifindex has no live circuit.
func (e *engine) ifaceNameFor(ifindex int) string {
	e.circuitsMu.RLock()
	c := e.circuits[ifindex]
	e.circuitsMu.RUnlock()
	if c == nil {
		return ""
	}
	return c.Name()
}

// ---- small helpers ----

// levelOfPDU derives the key-store level from a fully-encoded PDU's type octet.
func levelOfPDU(pdu []byte) (lsdbLevel, bool) {
	if len(pdu) <= offPDUType {
		return levelOne, false
	}
	return levelOfPDUType(packet.PDUType(pdu[offPDUType] & pduTypeMask))
}

// levelOfPDUType maps a PDU type to the key-store level for the per-level chains
// (LSP/CSNP/PSNP) and the LAN IIH. A P2P Hello is level-agnostic on the wire and
// returns levelOne only as the metric label; verifyFrame does NOT use this level
// to pick the P2P IIH chain (it tries both L1 and L2 IIH chains, matching the
// sign side which signs a P2P Hello with the NEGOTIATED adjacency level).
func levelOfPDUType(pt packet.PDUType) (lsdbLevel, bool) {
	switch pt {
	case packet.PDUTypeL1LANHello, packet.PDUTypeL1LSP, packet.PDUTypeL1CSNP, packet.PDUTypeL1PSNP:
		return levelOne, true
	case packet.PDUTypeL2LANHello, packet.PDUTypeL2LSP, packet.PDUTypeL2CSNP, packet.PDUTypeL2PSNP:
		return levelTwo, true
	case packet.PDUTypeP2PHello:
		return levelOne, true
	default:
		return levelOne, false
	}
}

// isIIHType reports whether a PDU type is an IS-IS Hello (per-interface auth) as
// opposed to an LSP/CSNP/PSNP (per-level auth).
func isIIHType(pt packet.PDUType) bool {
	switch pt {
	case packet.PDUTypeL1LANHello, packet.PDUTypeL2LANHello, packet.PDUTypeP2PHello:
		return true
	default:
		return false
	}
}

// adjToKSLevel maps an adjacency.Level to the key-store level.
func adjToKSLevel(l adjacency.Level) lsdbLevel {
	if l == adjacency.Level2 {
		return levelTwo
	}
	return levelOne
}

// ksLevelToken renders a key-store level as the "l1"/"l2" metric label.
func ksLevelToken(l lsdbLevel) string {
	if l == levelTwo {
		return lsdb.Level2.String()
	}
	return lsdb.Level1.String()
}
