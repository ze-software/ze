// Design: docs/architecture/wire/attributes.md -- RFC 9774 Section 3, received AS_SET
// RFC: rfc/short/rfc9774.md
// Related: session_validation_receive.go -- validateRFC7606Attrs reads AcceptASSet

package reactor

import "fmt"

// Values of the peer leaf `session as-set` (ze-bgp-conf.yang).
const (
	asSetPolicyWithdraw = "withdraw"
	asSetPolicyAccept   = "accept"
)

// applyASSetPolicy reads the peer leaf `session as-set` into AcceptASSet.
//
// RFC 9774 Section 3: "Unless explicitly configured by a network operator to do
// otherwise (e.g., during a transition phase), BGP speakers:" ... "MUST use the
// "treat-as-withdraw" error handling behavior per [RFC7606] upon reception of BGP
// UPDATE messages containing AS_SETs or AS_CONFED_SETs in the AS_PATH or AS4_PATH
// [RFC6793]." `accept` is that configuration: the operator sets it, and
// `ze exabgp migrate` writes it for an ExaBGP neighbor that said `as-set accept`.
// An absent leaf is `withdraw`, the YANG default and the RFC's behavior.
func applyASSetPolicy(ps *PeerSettings, name string, session map[string]any) error {
	raw, present := session["as-set"]
	if !present {
		return nil
	}
	value := fmt.Sprint(raw)
	if value == asSetPolicyAccept {
		ps.AcceptASSet = true
		return nil
	}
	if value == asSetPolicyWithdraw {
		ps.AcceptASSet = false
		return nil
	}
	return fmt.Errorf("peer %s: invalid session as-set %q: use withdraw or accept", name, value)
}
