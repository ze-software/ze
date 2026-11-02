// Design: plan/post-release/engineering/spec-filter-wire-0-umbrella.md -- Data Flow step 2, every BGP chain compiles to the library chain
// Related: filter_library.go -- runLibraryChain and rewriteLibrary, which serve and apply the text members
// Related: ../policychain/external_text.go -- TextCaller, the chain's side
// Related: filter_ordered.go -- the text chain steps, which share textDeltaOps
//
// TRANSITIONAL, deleted in phase 5 with the rest of the text machinery (AC-8):
// a plugin filter that declares no reads still answers the text update, and
// the library chain serves it here with the text chain's own pieces (the
// text rendering, policyFilterFunc, applyFilterDelta, the text delta
// extractors), so its verdict and the bytes it produces are the text chain's.

package reactor

import (
	"errors"
	"unsafe"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/policychain"
	"github.com/ze-software/ze/internal/component/bgp/policyview"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
)

// libraryText is one library run's text: the received UPDATE rendered once,
// at the first text member, and the running text each text member's delta
// folds into, as PolicyFilterChain folds it. It lives in the pooled
// libraryRun, so its rendering buffer is kept between runs. Not safe for
// concurrent use: one run owns it.
type libraryText struct {
	reactor    *Reactor
	wireUpdate *wireu.WireUpdate
	payload    []byte
	session    policyview.Session
	direction  string
	peer       string
	peerAS     uint32

	call     PolicyFilterFunc
	buf      []byte
	original string
	current  string
	rendered bool
}

// reasonTextBesideNLRISets refuses a run where a text member's delta and a
// member's NLRI sets both rewrite the NLRI.
const reasonTextBesideNLRISets = "a text filter and a filter's NLRI sets both rewrite the NLRI"

var (
	errTextFilterFailed = errors.New("the text filter failed, and its on-error rejects the route")
	errTextFilterAction = errors.New("the text filter answered an action that is neither accept, reject nor modify")
)

// begin readies t for one run. It renders nothing: a chain with no text
// member pays nothing. The caller MUST call clear when the run ends.
func (t *libraryText) begin(r *Reactor, wireUpdate *wireu.WireUpdate, payload []byte, session policyview.Session, direction, peer string, peerAS uint32) {
	t.reactor, t.wireUpdate, t.payload, t.session = r, wireUpdate, payload, session
	t.direction, t.peer, t.peerAS = direction, peer, peerAS
}

// clear drops the run's references and keeps the buffer. It MUST be called
// after begin, once the run's result no longer reads the text.
func (t *libraryText) clear() {
	buf := t.buf[:0]
	*t = libraryText{buf: buf}
}

// changed reports whether a text member's delta changed the running text; a
// nil t has served no run.
func (t *libraryText) changed() bool {
	return t != nil && t.rendered && t.current != t.original
}

// CallTextFilter sends the filter the running text through policyFilterFunc,
// which applies the filter's declared on-error to a failure as the text chain
// does, and folds a modify's delta (applyFilterDelta).
func (t *libraryText) CallTextFilter(plugin, filter string) (policychain.TextAnswer, error) {
	if !t.rendered {
		attrs, _ := t.wireUpdate.Attrs() //nolint:errcheck // a malformed section renders as the text chain renders it
		t.buf = AppendUpdateForFilter(t.buf[:0], attrs, t.wireUpdate, nil)
		t.original = unsafe.String(unsafe.SliceData(t.buf), len(t.buf)) //nolint:gosec // audited: buf outlives the run, cleared after it
		t.current = t.original
		t.call = t.reactor.policyFilterFunc(t.wireUpdate, t.payload, t.session)
		t.rendered = true
	}
	res := t.call(plugin, filter, t.direction, t.peer, t.peerAS, t.current)
	if res.Teardown {
		// The text chain rejects a teardown in both directions; the chain
		// honors the teardown itself on import only.
		return policychain.TextAnswer{Reject: true, Teardown: true, NotifyCode: res.NotifyCode, NotifySubcode: res.NotifySubcode}, nil
	}
	switch res.Action {
	case PolicyReject:
		if res.Failed {
			return policychain.TextAnswer{}, errTextFilterFailed
		}
		return policychain.TextAnswer{Reject: true}, nil
	case PolicyModify:
		if len(res.Raw) > 0 {
			return policychain.TextAnswer{Raw: res.Raw}, nil
		}
		t.current = applyFilterDelta(t.current, res.Delta)
		return policychain.TextAnswer{}, nil
	case PolicyAccept:
		return policychain.TextAnswer{}, nil
	}
	return policychain.TextAnswer{}, errTextFilterAction
}

// textDeltaOps writes into mods the operations that turn the original
// filter text into modified, through the extractors the text chain uses, and
// answers the NLRI override sectionNLRIDelta computes (false with the refusal
// when the answer drops NLRI the engine cannot drop). The AS-path directives
// apply only where intent allows an AS_PATH edit, and the MED removal only on
// import. values MUST stay held until buildModifiedPayload has copied the
// operations' values.
func textDeltaOps(values *valueScratch, original, modified string, wireUpdate *wireu.WireUpdate, intent libraryIntentContext, medRemove bool, mods *filterapi.ModAccumulator) ([]byte, nlriRefusal, bool) {
	// The two parses write into storage this call owns, so the structs live
	// on its stack rather than on the heap; each text is parsed once and the
	// extractors share the parse read-only (spec filter-delta-parse-once).
	var origAttrs, modAttrs filterAttrs
	parseFilterAttrsInto(&origAttrs, original)
	parseFilterAttrsInto(&modAttrs, modified)
	textDeltaToModOps(values, &origAttrs, &modAttrs, mods)
	attrsWire, _ := wireUpdate.Attrs() //nolint:errcheck // a nil section is what both AS-path cores refuse to edit
	if intent.asPathEditable {
		ExtractRemovePrivateASOps(values, &modAttrs, attrsWire, intent.asn4, intent.peerAS, mods)
		ExtractASPathPrependOps(values, &modAttrs, attrsWire, intent.asn4, intent.localAS, mods)
	} else {
		fwdLogger().Debug("export AS_PATH edits not applied toward an internal peer (RFC 4271 Section 5.1.2)")
	}
	// RFC 4271 Section 5.1.4's configured removal, and the ONLY site that
	// converts the directive. The rewritten payload replaces the WireUpdate
	// before the RIB plugin runs Decision Process phases 1 and 2, which is the
	// ordering that section requires of a removal.
	if medRemove && medRemoveHasWork(&modAttrs) {
		ExtractMEDRemoveOps(&modAttrs, mods)
	}
	return sectionNLRIDelta(values, original, modified, wireUpdate, mods)
}
