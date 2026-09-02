// Design: docs/contributing/testing.md -- a gate accounts for every member it does not walk
// Related: ../verify/lint/verifylint.go -- the first gate that carried this accounting
//
// Package population turns a gate's claim about the set it governs into an
// accounting that fails when a member is neither walked nor excused.
//
// The failure this answers is the largest recorded class in plan/journal: a gate
// walks part of the set it claims, and the shorter walk reads as a pass. A count
// floor catches only the empty case, and comparing sizes instead of sets is the
// same defect one level up. So the accounting is a set difference, and every
// member the walk missed owes a stated reason that is itself rechecked.
package population

import (
	"fmt"
	"sort"
)

// defaultUnexcused is the reason recorded against a member that no walk covered
// and no excuse names. It is shouted because it IS the finding, not a note
// beside one.
const defaultUnexcused = "NOT WALKED AND NOT EXCUSED"

// Blind is one population member the walk did not cover, with the reason it did
// not. A member the gate cannot explain still gets a row here, so the report
// names it rather than dropping it.
type Blind struct {
	Member string `json:"member"`
	Reason string `json:"reason"`
}

// Coverage is the accounting a gate publishes over the set it claims to govern.
// Code is non-zero when the accounting does not balance.
//
// The two ways it fails to balance are deliberately symmetric. An unexcused
// member is the gate walking less than it claims. A healed excuse is a stated
// exception that has stopped being one, and leaving it in place hides the next
// member to land on that same path.
type Coverage struct {
	Population int      `json:"population"`
	Walked     int      `json:"walked"`
	Blind      []Blind  `json:"blind,omitempty"`
	Unexcused  []string `json:"unexcused,omitempty"`
	Healed     []string `json:"healed-excuses,omitempty"`
	Code       int      `json:"code"`
}

// Claim is what a gate says about the set it governs, before Assess turns it
// into the accounting.
type Claim struct {
	// Subject names the population in the refusal message, so the reader of a
	// failing gate learns WHICH set came back empty.
	Subject string
	// Population is every member the gate claims to govern.
	Population map[string]bool
	// Walked is the members the gate covered. A member here that is absent from
	// Population is ignored, because the claim is what is under test.
	Walked map[string]bool
	// Excused maps a member to the reason the gate deliberately does not walk
	// it. An entry that has stopped being needed is a finding, not a comment.
	//
	// An entry whose reason is empty excuses nothing. The reason is the whole
	// value of the exception: it is what a later reader checks to decide whether
	// the exception still holds, and a blank one asks them to take it on trust.
	Excused map[string]string
	// UnexcusedReason replaces the wording recorded against an unexcused
	// member, so each gate names the walk a reader must go and extend. Empty
	// takes the package default.
	UnexcusedReason string
}

// Exemptions accounts for a gate's exemption rules against the members each rule
// matched during the walk.
//
// This is the accounting a file-level one cannot give. A gate whose walk either
// scans a file or exempts it balances by construction, so it proves nothing
// about the exemptions themselves. Here the RULES are the population, and the
// walk is the only producer that can say which of them still match anything.
//
// A rule that matches nothing is not untidiness. It states that some behavior
// is legitimate at a path. It keeps stating that for whatever code arrives at
// the path next, with nobody having judged it.
//
// rules maps each rule to the reason it exists. matched holds the rules the
// walk used. A rule the walk never used comes back in Coverage.Unexcused.
//
// No rules at all is a valid state, and it answers a clean Coverage rather than
// the refusal Assess gives an empty population. The two empties are different
// facts. A walked population that came back empty is a walk that found nothing
// and looks like a healthy tree. A rule set that is empty was WRITTEN empty, in
// source a reader can see, and it says the gate exempts nobody.
func Exemptions(subject string, rules map[string]string, matched map[string]bool) (Coverage, error) {
	if len(rules) == 0 {
		return Coverage{}, nil
	}
	declared := make(map[string]bool, len(rules))
	for rule := range rules {
		declared[rule] = true
	}
	claim := Claim{
		Subject:         subject,
		Population:      declared,
		Walked:          matched,
		UnexcusedReason: "MATCHES NOTHING IN THE TREE THE WALK READ",
	}
	return claim.Assess()
}

// Assess accounts for every member of the claimed population against the set the
// gate walked.
//
// An empty population is an error rather than a clean report. A gate whose walk
// found nothing prints what a gate over a healthy tree prints, and telling those
// two apart is why this package exists.
func (c Claim) Assess() (Coverage, error) {
	if len(c.Population) == 0 {
		return Coverage{}, fmt.Errorf("%s population is empty", c.Subject)
	}

	blind := make([]string, 0, len(c.Population))
	for member := range c.Population {
		if !c.Walked[member] {
			blind = append(blind, member)
		}
	}
	sort.Strings(blind)

	unexcused := c.UnexcusedReason
	if unexcused == "" {
		unexcused = defaultUnexcused
	}

	coverage := Coverage{Population: len(c.Population), Walked: len(c.Population) - len(blind)}
	isBlind := make(map[string]bool, len(blind))
	for _, member := range blind {
		isBlind[member] = true
		reason := c.Excused[member]
		if reason == "" {
			reason = unexcused
			coverage.Unexcused = append(coverage.Unexcused, member)
		}
		coverage.Blind = append(coverage.Blind, Blind{Member: member, Reason: reason})
	}
	// An excuse for a member that is no longer blind, and an excuse for a member
	// that has left the population entirely, are the same defect: a statement
	// nobody rechecked. Both land here.
	for member := range c.Excused {
		if !isBlind[member] {
			coverage.Healed = append(coverage.Healed, member)
		}
	}
	sort.Strings(coverage.Healed)

	if len(coverage.Unexcused) != 0 || len(coverage.Healed) != 0 {
		coverage.Code = 1
	}
	return coverage, nil
}
