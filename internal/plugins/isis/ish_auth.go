// Design: docs/architecture/isis/isis-10-auth.md -- ISO 9542 per-link passwords.
// Related: auth_wiring.go -- installs the ISH hooks beside the IIH hooks.
// RFC 1195 Annex D.2: "IS-IS Hello and 9542 IS Hello packets shall contain the
// per-link password" when simple-password authentication is used.
//
// ISO 9542 option layout: code 133 | length | auth type 1 | password octets.
// RFC 5304/5310 define IS-IS crypto authentication, not an ES-IS HMAC encoding.
// Crypto-only chains therefore protect IIHs, while ISHs remain discovery only.

package isis

import (
	"fmt"
	"time"

	"github.com/ze-software/ze/internal/plugins/isis/packet"
	"github.com/ze-software/ze/internal/plugins/isis/transport"
)

// signISHPDU selects an active per-link cleartext password. Absence of a current
// key in a configured cleartext chain is an error, never an unsigned fallback.
func (e *engine) signISHPDU(iface string, pdu []byte) ([]byte, error) {
	e.ksMu.RLock()
	defer e.ksMu.RUnlock()
	now := time.Now()
	required := false
	for _, level := range [...]lsdbLevel{levelOne, levelTwo} {
		chain := e.keystore.helloChain(iface, level)
		if chain == nil {
			continue
		}
		if !chain.cleartext {
			continue
		}
		required = true
		for _, candidate := range chain.keys {
			if candidate.key.Algorithm != packet.AuthAlgoCleartext {
				continue
			}
			if candidate.send.contains(now) {
				return packet.SignISH(pdu, candidate.key)
			}
		}
	}
	if required {
		return nil, fmt.Errorf("isis: no active ISO 9542 password on interface %s", iface)
	}
	return pdu, nil
}

// verifyISHFrame checks the per-link password before discovery changes state.
// The ISH has no routing level, so the bounded key set includes both link chains.
func (e *engine) verifyISHFrame(rf transport.RawFrame) bool {
	iface := e.ifaceNameFor(rf.IfIndex)
	var keys [2 * maxKeysPerChain]packet.Key
	count := 0
	required := false
	now := time.Now()
	e.ksMu.RLock()
	for _, level := range [...]lsdbLevel{levelOne, levelTwo} {
		chain := e.keystore.helloChain(iface, level)
		if chain == nil {
			continue
		}
		if !chain.cleartext {
			continue
		}
		required = true
		accepted := 0
		for _, candidate := range chain.keys {
			if candidate.key.Algorithm != packet.AuthAlgoCleartext {
				continue
			}
			if !candidate.accept.contains(now) {
				continue
			}
			if accepted == maxKeysPerChain {
				break
			}
			keys[count] = candidate.key
			count++
			accepted++
		}
	}
	fail := e.authFailures
	e.ksMu.RUnlock()
	if !required {
		return true
	}
	if count > 0 {
		if err := packet.VerifyISH(rf.PDU, keys[:count]); err == nil {
			return true
		}
	}
	fail.With("l1", iface).Inc()
	e.log.Debug("isis: ISO 9542 password verification failed", "interface", iface)
	return false
}
