// Design: docs/architecture/wire/nlri.md -- NLRI receive validation, the per-family syntax registry
// RFC: rfc/short/rfc7606.md -- Sections 3(j) and 5.3; rfc/short/rfc9552.md -- Section 8.2.2; rfc/short/rfc9830.md -- Section 5
// Related: ../nlritype/nlritype.go -- the per-family TYPE ruling this registry sits beside
// Related: ../nlrisplit/nlrisplit.go -- the per-family framing
//
// Package nlrisyntax holds a registry of per-family NLRI SYNTAX rulings.
//
// RFC 7606 Section 5.3 judges the NLRI framing of the families whose NLRI is a
// plain prefix list. Some typed families carry their own syntax rules in their own
// specification: RFC 9552 Section 8.2.2 for BGP-LS, RFC 7432 Section 7.1 for the
// fixed Ethernet A-D length, RFC 9830 Section 5 for SR Policy. Each such family
// registers one Rule here, from the package that owns its layout, and the receive
// path reads the Rule by family. No receive-path file names a family.
//
// A Rule has two answers, each optional, because the receive path asks two
// questions at two passes:
//
//   - Judge answers for the whole NLRI section of one MP attribute, at the RFC 7606
//     Section 5.3 walk (message.validateMPNLRISyntax). Its Judgement is the
//     UPDATE's verdict: well-formed, treat-as-withdraw, or session reset.
//   - Retain answers per NLRI, at the per-route pass (reactor.typedNLRIEdit): it
//     returns the section with the malformed NLRIs removed, so neither the RIB nor
//     a synthesized withdrawal nor a relay carries them.
package nlrisyntax

import (
	"errors"
	"fmt"
	"maps"
	"sync"

	"github.com/ze-software/ze/internal/core/family"
)

// Registration errors.
var (
	// ErrDuplicate reports a second ruling for a family that already has one.
	ErrDuplicate = errors.New("nlrisyntax: duplicate rule for family")
	// ErrEmpty reports a Rule with neither answer, which rules on nothing.
	ErrEmpty = errors.New("nlrisyntax: rule has neither Judge nor Retain")
	// ErrNoCitation reports a Rule that names no specification. The receive path
	// logs the citation on every verdict, so an operator can find the rule.
	ErrNoCitation = errors.New("nlrisyntax: rule names no citation")
	// ErrNoRemovedMessage reports a Rule with a Retain answer and no RemovedMessage.
	ErrNoRemovedMessage = errors.New("nlrisyntax: rule with Retain names no removed-NLRI message")
)

// Action is the verdict a Judge answers for one NLRI section.
type Action uint8

const (
	// ActionUnspecified is the zero value, never an answer. A Judge that returns it
	// has a defect, and the receive path treats it as one (session reset, logged).
	ActionUnspecified Action = iota
	// ActionWellFormed: every NLRI in the section passed the family's syntax rule.
	ActionWellFormed
	// ActionTreatAsWithdraw: the section is walkable, so every NLRI boundary is known,
	// and at least one NLRI is malformed in a way the family answers with
	// treat-as-withdraw (RFC 7606 Section 2).
	ActionTreatAsWithdraw
	// ActionSessionReset: the section cannot be walked, so no NLRI boundary after the
	// failure is known and treat-as-withdraw is impossible (RFC 7606 Section 3(j)).
	ActionSessionReset
)

// Judgement is a Judge's answer. Reason is the operator-facing line for any
// Action other than ActionWellFormed, and names the specification and the
// offending value.
type Judgement struct {
	Action Action
	Reason string
}

// WellFormed is the Judgement for a section every NLRI of which passed.
var WellFormed = Judgement{Action: ActionWellFormed}

// Rule is one family's NLRI syntax ruling. At least one of Judge and Retain is set.
type Rule struct {
	// Citation names the specification section the rule implements, for log lines
	// the receive path writes on the rule's behalf (for example "RFC 9830 Section 5").
	Citation string
	// RemovedMessage is the log message the receive path writes, with the count under
	// "discarded", when Retain removed NLRIs. It names the specification, as RFC 9552
	// Section 8.2.2 asks: "An implementation SHOULD log a message for any errors found
	// during syntax validation for further analysis." Required when Retain is set.
	RemovedMessage string
	// Judge answers for a whole NLRI section, which starts at the first NLRI of an MP
	// attribute and runs to its end. addPath is the RFC 7911 negotiation for the
	// family: each NLRI is then preceded by a 4-octet path identifier. Judge MUST
	// NOT retain section, and MUST bound every read by len(section).
	Judge func(section []byte, addPath bool) Judgement
	// Retain returns section with every NLRI the rule finds malformed removed, and
	// how many it removed. It returns section itself, sharing its backing array,
	// when it removed nothing. framed is false when the section cannot be walked,
	// and the caller then resets the session.
	Retain func(section []byte, addPath bool) (kept []byte, dropped int, framed bool)
}

var (
	mu    sync.RWMutex
	rules = map[family.Family]Rule{}
)

// Register installs rule for fam. A family has one ruling: a second
// registration returns ErrDuplicate. A rule with neither answer returns ErrEmpty
// one with no citation ErrNoCitation, and one with Retain but no RemovedMessage
// ErrNoRemovedMessage. The caller MUST treat any error as a
// startup failure.
func Register(fam family.Family, rule Rule) error {
	if rule.Judge == nil && rule.Retain == nil {
		return fmt.Errorf("%w: %s", ErrEmpty, fam)
	}
	if rule.Citation == "" {
		return fmt.Errorf("%w: %s", ErrNoCitation, fam)
	}
	if rule.Retain != nil && rule.RemovedMessage == "" {
		return fmt.Errorf("%w: %s", ErrNoRemovedMessage, fam)
	}
	mu.Lock()
	defer mu.Unlock()
	if _, ok := rules[fam]; ok {
		return fmt.Errorf("%w: %s", ErrDuplicate, fam)
	}
	rules[fam] = rule
	return nil
}

// Get returns the rule registered for fam. ok is false when no family package
// ruled on fam, and the caller then applies no family syntax rule.
func Get(fam family.Family) (rule Rule, ok bool) {
	mu.RLock()
	defer mu.RUnlock()
	rule, ok = rules[fam]
	return rule, ok
}

// SnapshotForTest empties the registry and returns a function that restores
// exactly what was registered. NOT for production use, and not safe under
// t.Parallel. Same contract as nlritype.SnapshotForTest.
func SnapshotForTest() func() {
	mu.Lock()
	defer mu.Unlock()
	saved := make(map[family.Family]Rule, len(rules))
	maps.Copy(saved, rules)
	rules = map[family.Family]Rule{}
	return func() {
		mu.Lock()
		defer mu.Unlock()
		rules = saved
	}
}
