// Design: rfc/short/rfc7606.md -- Section 5.4, typed NLRI
// RFC: rfc/short/rfc7606.md -- Section 5.4 discard of unrecognized NLRI types
// Related: ../nlrisplit/nlrisplit.go -- the per-family framing this registry carves with
//
// Package nlritype holds a registry of per-family NLRI route-type recognizers.
//
// RFC 7606 Section 5.4: "A BGP speaker advertising support for such a typed
// address family MUST handle routes with unrecognized NLRI types within that
// address family by discarding them, unless the relevant specification for that
// address family specifies otherwise."
//
// The escape clause is the whole design. It is per family, and at least one
// family uses it: RFC 9552 Section 5.2 deviates from Section 5.4 for BGP-LS and
// requires unknown Link-State NLRI types to be preserved and propagated. A
// blanket discard would violate that MUST. So the ruling is registered per
// family by the plugin that owns the family, and a family nobody has ruled on
// discards nothing.
//
// Registration is via init() in the owning NLRI plugin, beside that plugin's
// family.MustRegister call. The recognizer's presence therefore tracks the
// family's advertisement: compile the plugin out and ze neither advertises the
// family nor owes Section 5.4 for it.
package nlritype

import (
	"errors"
	"fmt"
	"maps"
	"sync"

	"github.com/ze-software/ze/internal/core/bgp/nlri/nlrisplit"
	"github.com/ze-software/ze/internal/core/family"
)

// Registration errors.
var (
	// ErrNoSplitter reports a recognizer for a family whose NLRI section cannot
	// be carved into individual NLRIs.
	ErrNoSplitter = errors.New("nlritype: no NLRI splitter registered for family")
	// ErrDuplicate reports a second ruling for a family that already has one.
	ErrDuplicate = errors.New("nlritype: duplicate recognizer for family")
)

// Verdict is a recognizer's answer about one carved NLRI.
type Verdict uint8

const (
	// Keep: the NLRI carries a type this speaker implements and, where the family's
	// recognizer judges the body, a well-formed one.
	Keep Verdict = iota
	// Discard: the NLRI carries a type this speaker does not implement. RFC 7606
	// Section 5.4: "MUST handle routes with unrecognized NLRI types within that
	// address family by discarding them".
	Discard
	// Withdraw: the NLRI's type is implemented but its body is malformed in a way
	// the family's specification answers with treat-as-withdraw (RFC 7606
	// Section 2), so the route leaves the announcement and becomes a withdrawal.
	Withdraw
)

// Recognizer judges one carved NLRI. nlriBytes is a single NLRI exactly as it
// appears on the wire; when addPath is true it starts with the 4-byte RFC 7911
// path identifier, which the recognizer must skip before reading the type.
//
// Every recognizer answers about the TYPE. A family whose specification prescribes
// treat-as-withdraw for a malformed body may also judge the body and answer
// Withdraw; one that does not answers only Keep or Discard. Framing the NLRI is not
// the recognizer's job: the family's splitter carved it, and a framing failure is
// an RFC 7606 Section 5.3 concern Retain reports as an error.
type Recognizer func(nlriBytes []byte, addPath bool) Verdict

var (
	mu          sync.RWMutex
	recognizers = map[family.Family]Recognizer{}
	// bodyJudged holds the families whose recognizer was installed by
	// RegisterBodyRecognizer.
	bodyJudged = map[family.Family]bool{}
)

// Register installs fn as Section 5.4's recognizer for fam.
//
// Returns ErrNoSplitter when fam has no registered nlrisplit splitter. Judging
// route types requires carving the NLRI section into individual NLRIs, so a
// recognizer without a splitter is a contradiction with no permitted runtime
// resolution: failing open would violate Section 5.4, and failing closed would
// discard every route in the family. Reporting it here makes it a startup
// failure the caller must act on rather than a silent wire-visible one. Package
// initialization order guarantees the splitter is present first, because this
// package imports nlrisplit.
//
// Returns ErrDuplicate on a second registration. A family has one ruling.
// A nil fn is a no-op, so a family whose specification overrides Section 5.4 can
// say so at its own registration site without a special case here.
func Register(fam family.Family, fn Recognizer) error {
	if fn == nil {
		return nil
	}
	if !nlrisplit.Supported(fam) {
		return fmt.Errorf("%w: %s", ErrNoSplitter, fam)
	}
	mu.Lock()
	defer mu.Unlock()
	if _, ok := recognizers[fam]; ok {
		return fmt.Errorf("%w: %s", ErrDuplicate, fam)
	}
	recognizers[fam] = fn
	return nil
}

// RegisterBodyRecognizer installs fn as Register does, for a family whose
// recognizer also judges the NLRI body and answers Withdraw for a malformed one.
//
// The family says so because a malformed NLRI cannot name its own route on the
// wire. Its route key forms, so the Adj-RIB-In entry it replaces is known, but a
// receiver decodes every field of a withdrawn NLRI, and the bytes the peer sent
// are the bytes Ze's own recognizer refused. A receiver that relays the
// treat-as-withdraw therefore needs the earlier, well-formed generation of the
// same route to withdraw it by, and JudgesBody tells it which families to keep
// that generation for.
//
// Errors are those of Register.
func RegisterBodyRecognizer(fam family.Family, fn Recognizer) error {
	if err := Register(fam, fn); err != nil {
		return err
	}
	if fn == nil {
		return nil
	}
	mu.Lock()
	defer mu.Unlock()
	bodyJudged[fam] = true
	return nil
}

// JudgesBody reports whether fam's recognizer was installed by
// RegisterBodyRecognizer, so it can answer Withdraw for an NLRI whose bytes are
// malformed.
func JudgesBody(fam family.Family) bool {
	mu.RLock()
	defer mu.RUnlock()
	return bodyJudged[fam]
}

// Get returns the registered recognizer for fam, or nil when no ruling exists.
func Get(fam family.Family) Recognizer {
	mu.RLock()
	defer mu.RUnlock()
	return recognizers[fam]
}

// Retained is what Retain decided about one MP NLRI section.
type Retained struct {
	// Kept is the section with every discarded and withdrawn NLRI removed. It is the
	// input itself, sharing the caller's backing array, whenever nothing was removed.
	Kept []byte
	// Withdrawn holds, in wire order and with their path identifiers, the NLRIs of an
	// announcement the recognizer answered Withdraw for and whose route key forms.
	// Always nil for a withdrawal section, where such an NLRI is already a withdrawal
	// and stays in Kept.
	Withdrawn []byte
	// Unkeyable holds the NLRIs answered Withdraw whose route key cannot form. No
	// withdrawal can name such a route, so it is discarded instead of withdrawn.
	Unkeyable []byte
	// Discarded counts the NLRIs removed as unrecognized types plus the unkeyable ones.
	Discarded int
	// WithdrawnCount counts the NLRIs in Withdrawn.
	WithdrawnCount int
	// UnkeyableCount counts the NLRIs in Unkeyable, which Discarded includes.
	UnkeyableCount int
}

// Removed reports whether any NLRI left the section.
func (r Retained) Removed() bool { return r.Discarded+r.WithdrawnCount > 0 }

// Retain applies fn to every NLRI of one MP NLRI section and reports the routes
// that survive, the routes to withdraw, and the routes discarded.
//
// fn is the family's recognizer, which the caller has already looked up with Get.
// Passing it rather than looking it up again is what lets the receive path answer
// "does Section 5.4 bind this family at all" and "which routes survive" from ONE
// registry read. A nil fn means no ruling, and the answer is to change nothing.
//
// announce says data is an MP_REACH_NLRI section. There a Withdraw verdict moves the
// NLRI to Withdrawn, provided the family's route key (nlrisplit.GetPrefixKey) forms
// over it: a withdrawal that names no route removes nothing anywhere and cannot be
// relayed as a route, so an unkeyable NLRI is discarded and reported in Unkeyable.
// scratch is the caller-owned key scratch, nlrisplit.PrefixKeyScratchSize octets, so
// the key check allocates nothing. In an MP_UNREACH_NLRI section a Withdraw verdict
// keeps the NLRI: a malformed route withdrawn is still the withdrawal of that route.
//
// The common "nothing removed" answer costs no allocation. Removing a route
// allocates the rewritten section, which happens only on input a peer got wrong.
//
// A non-nil error means the NLRI framing could not be trusted, and data is
// returned unchanged. When the length fields do not agree with the section, the
// boundaries between NLRIs are unknowable, so no decision can be made and
// inventing one would rewrite the wire from a guess. Framing errors belong to
// RFC 7606 Section 5.3, which runs before this.
func Retain(fn Recognizer, fam family.Family, data []byte, addPath, announce bool, scratch []byte) (Retained, error) {
	return RetainUpdate(fn, nil, nil, fam, data, addPath, announce, scratch)
}

// RetainUpdate is Retain with the family's UPDATE-level check (update_check.go)
// applied as well: an announced NLRI fn keeps is passed to check with attrs, the
// path attributes of the UPDATE carrying it, and a Withdraw answer withdraws it
// exactly as a Withdraw from fn does. check is never called for a withdrawal
// section, nor for an NLRI fn discarded or withdrew. A nil fn with a non-nil check
// keeps every type; nil for both changes nothing. attrs may be nil only when check is.
func RetainUpdate(
	fn Recognizer, check UpdateCheck, attrs *UpdateAttrs,
	fam family.Family, data []byte, addPath, announce bool, scratch []byte,
) (Retained, error) {
	if len(data) == 0 {
		return Retained{Kept: data}, nil
	}
	if fn == nil && check == nil {
		return Retained{Kept: data}, nil
	}
	r := rulings{recognize: fn, check: check, attrs: attrs}

	nlris, splitErr := nlrisplit.Split(fam, data, addPath)
	if splitErr != nil {
		return Retained{Kept: data}, splitErr
	}

	// A nil key means a withdrawal section, where Withdraw keeps the NLRI.
	var key nlrisplit.PrefixKeyFunc
	if announce {
		if len(scratch) < nlrisplit.PrefixKeyScratchSize {
			panic("BUG: nlritype.Retain: key scratch shorter than nlrisplit.PrefixKeyScratchSize")
		}
		key = nlrisplit.GetPrefixKey(fam)
	}

	// One pass to count, so the common "nothing removed" answer costs no
	// allocation and the rewrite below is sized exactly.
	var out Retained
	keepBytes := 0
	for _, one := range nlris {
		switch judge(r, key, one, addPath, scratch) {
		case Keep:
			keepBytes += len(one)
		case Withdraw:
			out.WithdrawnCount++
		case Discard:
			out.Discarded++
		case unkeyable:
			out.Discarded++
			out.UnkeyableCount++
		}
	}
	if !out.Removed() {
		return Retained{Kept: data}, nil
	}

	if keepBytes > 0 {
		out.Kept = make([]byte, 0, keepBytes)
	}
	for _, one := range nlris {
		switch judge(r, key, one, addPath, scratch) {
		case Keep:
			out.Kept = append(out.Kept, one...)
		case Withdraw:
			out.Withdrawn = append(out.Withdrawn, one...)
		case unkeyable:
			out.Unkeyable = append(out.Unkeyable, one...)
		case Discard:
		}
	}
	return out, nil
}

// rulings is what one RetainUpdate call judges with: the family's recognizer, its
// UPDATE-level check, and the attributes that check reads.
type rulings struct {
	recognize Recognizer
	check     UpdateCheck
	attrs     *UpdateAttrs
}

// verdict is r's answer for one NLRI. The check runs only for an announcement
// (key non-nil) the recognizer kept: in a withdrawal the route is already leaving.
func (r rulings) verdict(key nlrisplit.PrefixKeyFunc, one []byte, addPath bool) Verdict {
	v := Keep
	if r.recognize != nil {
		v = r.recognize(one, addPath)
	}
	if v != Keep {
		return v
	}
	if r.check == nil {
		return Keep
	}
	if key == nil {
		return Keep
	}
	return r.check(one, addPath, r.attrs)
}

// judge applies r to one NLRI and, for an announcement (key non-nil), turns a
// Withdraw verdict whose route key cannot form into unkeyable.
func judge(r rulings, key nlrisplit.PrefixKeyFunc, one []byte, addPath bool, scratch []byte) Verdict {
	v := r.verdict(key, one, addPath)
	if v != Withdraw {
		return v
	}
	if key == nil {
		return Keep
	}
	raw := one
	if addPath {
		raw = one[4:]
	}
	if _, err := key(raw, scratch, true); err != nil {
		return unkeyable
	}
	return Withdraw
}

// unkeyable is Retain's private fourth answer: a Withdraw verdict over an NLRI
// whose route key cannot form. It never leaves this package.
const unkeyable Verdict = Withdraw + 1

// ResetForTest clears every registered recognizer and UPDATE-level check. Tests call it to start from a
// clean slate. NOT for production use.
//
// Prefer SnapshotForTest in a package whose test binary links real NLRI plugins: this
// leaves the registry EMPTY, so a later test in the same binary finds no recognizer for a
// family the daemon really does rule on, its Section 5.4 filter silently does nothing, and
// the test passes proving nothing.
func ResetForTest() {
	mu.Lock()
	defer mu.Unlock()
	recognizers = map[family.Family]Recognizer{}
	bodyJudged = map[family.Family]bool{}
	updateChecks = map[family.Family]UpdateCheck{}
}

// SnapshotForTest empties the registry and returns a function that puts back exactly what
// was registered. NOT for production use.
//
// Registration happens in plugin init(), so what a test binary starts with depends on which
// plugins it links, and that changes when an unrelated file adds an import. Restoring is the
// only teardown that cannot leave the next test in a state no daemon is ever in.
//
// Two limits, both deliberate and neither hidden. It does not NEST: a second call inside one
// test empties the registry again, so the first caller's registrations are gone for the rest
// of that test (the restores themselves are correct, since t.Cleanup runs last-in-first-out).
// And it is not safe under t.Parallel: two overlapping snapshots make the second save an
// already-empty map, and restoring it loses what the plugins registered. Register everything
// one test needs through ONE call, and do not mark such a test parallel.
func SnapshotForTest() func() {
	mu.Lock()
	defer mu.Unlock()
	saved := make(map[family.Family]Recognizer, len(recognizers))
	maps.Copy(saved, recognizers)
	savedJudged := make(map[family.Family]bool, len(bodyJudged))
	maps.Copy(savedJudged, bodyJudged)
	savedChecks := make(map[family.Family]UpdateCheck, len(updateChecks))
	maps.Copy(savedChecks, updateChecks)
	recognizers = map[family.Family]Recognizer{}
	bodyJudged = map[family.Family]bool{}
	updateChecks = map[family.Family]UpdateCheck{}
	return func() {
		mu.Lock()
		defer mu.Unlock()
		recognizers = saved
		bodyJudged = savedJudged
		updateChecks = savedChecks
	}
}
