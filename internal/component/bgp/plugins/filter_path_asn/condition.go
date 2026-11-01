// Design: docs/architecture/bgp/filter-path-asn.md -- the reject-asn match condition
// Related: register.go -- registers the reject-asn condition and its obligation
// Related: config.go -- parseOneList, the list parse the condition shares
// Related: match.go -- matchPositions and matchPattern, the judgement it shares
//
// The reject-asn match condition: a match instance's `reject-asn NAME` passes
// a route the named reject-asn list does not reject. It reads the list's own
// definition and judges the same AS-path text with the same position and
// pattern arms the list's filter uses, with the sender read off the view's
// session, which is known on import only, as senderOf reads it.

package filter_path_asn

import (
	"errors"
	"unsafe"

	"github.com/ze-software/ze/internal/component/bgp/policychain"
	"github.com/ze-software/ze/internal/component/policy"
)

// compileBGPCondition compiles a match instance's reject-asn value, the name
// of one reject-asn instance, into its check over a BGP UPDATE. The route
// passes when the list rejects it on neither arm; a reject is counted under
// the direction and the slot that decided it, as handleFilterUpdate counts
// it. An AS_PATH the view refuses as malformed is an error, so the member
// fails closed.
//
// RFC 7454 Section 9: "This loose policy could be combined with filters for
// specific 2-byte or 4-byte AS paths that must not be accepted if advertised by
// the customer, such as upstream transit providers or peer ASNs."
func compileBGPCondition(value any, resolve policy.InstanceResolver) (policy.ConditionCheck[policychain.MatchRoute], error) {
	list, err := conditionList(value, resolve)
	if err != nil {
		return nil, err
	}
	return func(r *policychain.MatchRoute) (bool, error) {
		text, err := r.View.ASPathText()
		if err != nil {
			return false, err
		}
		// The text is the view's own buffer, unchanged until the run ends, and
		// no judgement below keeps the string past its return.
		asPath := unsafe.String(unsafe.SliceData(text), len(text)) //nolint:gosec // audited: the view's buffer outlives this synchronous check
		session := r.View.Session()
		direction := directionExport
		sender := senderASN{}
		if session.SenderKnown {
			direction = directionImport
			sender = senderASN{asn: session.Sender, known: true}
		}
		if found, ok := matchPositions(asPath, sender, list.positions, list.nth); ok {
			recordReject(direction, rejectSlot(found.at))
			return false, nil
		}
		if _, ok := matchPattern(asPath, list.patterns); ok {
			recordReject(direction, slotPattern)
			return false, nil
		}
		return true, nil
	}, nil
}

// conditionList reads the reject-asn list the condition value names.
func conditionList(value any, resolve policy.InstanceResolver) (*rejectList, error) {
	name, ok := value.(string)
	if !ok {
		return nil, errors.New("must name one reject-asn instance")
	}
	if name == "" {
		return nil, errors.New("must name one reject-asn instance")
	}
	if resolve == nil {
		return nil, errors.New("BUG: compiled with no instance resolver")
	}
	instance, err := resolve(name, filterTypeName)
	if err != nil {
		return nil, err
	}
	return parseOneList(name, instance)
}
