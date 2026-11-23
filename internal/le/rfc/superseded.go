// Design: docs/contributing/rfc-conformance-gates.md -- superseded obligation resolution
// Related: profilecoverage.go -- figuresByStem calls resolveSuperseded when classifying rows
// Related: next.go -- rowOwes calls resolveSuperseded so satisfied superseded rows are not owed
package rfc

// kindSuperseded is the evidence kind MandatoryByKind counts a MUST row under
// when an obsoleted document's requirement is satisfied through its implemented
// successor (restated onto a proven requirement, or dropped by an implemented
// successor).
const kindSuperseded = "superseded"

// supersededOutcome states whether a row's {superseded} marker resolves it
// through its successor RFC.
type supersededOutcome uint8

const (
	supersededNone supersededOutcome = iota
	supersededProven
	supersededNotOwed
)

// indexRequirements maps requirement IDs to pointers inside requirements so
// superseded pointers resolve in O(1) without rescanning the corpus.
func indexRequirements(requirements []Requirement) map[string]*Requirement {
	out := make(map[string]*Requirement, len(requirements))
	for index := range requirements {
		req := &requirements[index]
		out[req.RID] = req
	}
	return out
}

// implementedStemsOf answers the published RFC stems Ze implements
// (ImplementsRFC) for resolving `{superseded: dropped}` rows whose obsoleting
// successor is built. Internet-Drafts never satisfy a published RFC.
func implementedStemsOf(metas map[string]Meta) map[string]bool {
	out := make(map[string]bool, len(metas))
	for stem := range metas {
		meta := metas[stem]
		if ImplementsRFC(stem, &meta) {
			out[stem] = true
		}
	}
	return out
}

// resolveSuperseded answers whether a row of an obsoleted RFC is already
// validated and complete (or excluded as not owed) through its implemented
// successor.
//
// A restated row is validated and complete when its successor row is proven
// (Ze proven or OS verified), and not owed when its successor row is excluded
// with checked evidence (not applicable, declined, or Linux limit). Chained
// obsoletions (such as RFC 3768 -> RFC 5798 -> RFC 9568) resolve recursively
// with a bound equal to the corpus size.
//
// A dropped row is validated and complete when Ze implements the obsoleting
// successor RFC that dropped the old obligation. Unextracted and unresolved
// markers stay debt and resolve to supersededNone.
func (e *figureEvidence) resolveSuperseded(req *Requirement, proven map[string]bool) (supersededOutcome, string) {
	return e.resolveSupersededDepth(req, proven, 0)
}

// resolveSupersededDepth implements resolveSuperseded with recursion bounded by
// len(e.byRID) so cyclic or malformed fixtures cannot loop.
func (e *figureEvidence) resolveSupersededDepth(req *Requirement, proven map[string]bool, depth int) (supersededOutcome, string) {
	if req.Superseded == nil {
		return supersededNone, ""
	}
	if depth > len(e.byRID) {
		return supersededNone, ""
	}
	switch req.Superseded.Disposition {
	case successorRestated:
		target := e.byRID[req.Superseded.Target]
		if target == nil {
			return supersededNone, ""
		}
		_, targetMet := rowCounts(target, proven)
		if targetMet {
			return supersededProven, ""
		}
		if osVerified(target, e.osProofs) {
			return supersededProven, ""
		}
		if !osMet(target) && e.excluded(target) {
			return supersededNotOwed, target.Annotation.Kind
		}
		return e.resolveSupersededDepth(target, proven, depth+1)
	case successorDropped:
		successor := e.successors[req.RFC]
		if successor == "" {
			return supersededNone, ""
		}
		if e.implemented[successor] {
			return supersededProven, ""
		}
		return supersededNone, ""
	case successorUnextracted, successorUnresolved:
		return supersededNone, ""
	default:
		// Open string disposition from synthetic fixtures: fail closed.
		return supersededNone, ""
	}
}
