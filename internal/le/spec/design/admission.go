// Design: docs/contributing/spec-workflow.md -- what lets a ready spec start: Thomas's approval, or his away admission
// Related: approval.go -- the one approval lookup this file asks
// Related: away.go -- Thomas's recorded absence
//
// admission.go answers the question every gate on a spec's start asks: may
// this design text start implementation? Claim asks it before the WIP cap,
// the ready-edit hook before a spec becomes ready, and `./le commit create`
// to write the Design-approved: trailer that carries the answer to every
// checkout (spec-design-needs-owner-approval,
// AC-21 to AC-29, AC-31). One function judges it, so no gate admits a design
// another refuses.
package specdesign

import (
	"errors"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/le/doc/readrecord"
	specpath "github.com/ze-software/ze/internal/le/spec/path"
)

// Admission is what lets a spec's design text start implementation, or why
// nothing does.
type Admission struct {
	// Approval is Thomas's approval of this text, or, with Approval.Away set,
	// the admission of a judged fix spec while he is away. It is valid only
	// when Admitted.
	Approval Approval `json:"approval"`
	Admitted bool     `json:"admitted"`
	// ApprovedBefore is the day Thomas approved an earlier text of this spec,
	// "" when he approved none (AC-24).
	ApprovedBefore string `json:"approved-before,omitempty"`
	// Away is Thomas's absence when he is recorded away, nil otherwise.
	Away *Away `json:"away,omitempty"`
	// AwayProblems names each condition of the away admission the spec fails,
	// while he is away (AC-29).
	AwayProblems []string `json:"away-problems,omitempty"`
}

// Admit answers whether the design text of the spec at relative, a
// checkout-relative path, may start implementation on the day now falls on:
// Thomas's approval of this text (the local record or a reachable
// Design-approved: line), his approval of an earlier text carried to this one
// (carry.go), else, while he is away, a fix spec with no departs
// row whose check passes with the judge's current approval. An error is a
// record or a history that could not be read, or a file name no record can
// key on, never "not admitted".
//
// An away admission a trailer recorded admits only while Thomas is still
// away: once he is back, the design waits for him ("he reviews it on return").
func Admit(root, relative, text string, now time.Time) (Admission, error) {
	spec, keyed := specText(relative, text)
	if !keyed {
		return Admission{}, errors.New(relative + errNotASpec)
	}
	approval, found, err := Approved(root, spec.stem, text)
	if err != nil {
		return Admission{}, err
	}
	if found && !approval.Away {
		return Admission{Approval: approval, Admitted: true}, nil
	}
	away, problems, err := awayAdmission(root, spec, now)
	if err != nil {
		return Admission{}, err
	}
	if away != nil && len(problems) == 0 {
		admitted := Approval{Stem: spec.stem, Digest: DesignDigest(text), Date: away.From,
			Away: true, From: away.From, Until: away.Until}
		return Admission{Approval: admitted, Admitted: true, Away: away}, nil
	}
	admission := Admission{Away: away, AwayProblems: problems}
	record, recorded, err := ReadApproval(root, spec.stem)
	if err != nil {
		return Admission{}, err
	}
	if recorded {
		admission.ApprovedBefore = record.Date
	}
	return admission, nil
}

// Reason answers why the design cannot start, for a refusal's headline
// (example 5, AC-24).
func (a Admission) Reason() string {
	if a.ApprovedBefore != "" {
		return "its design changed after Thomas approved it on " + a.ApprovedBefore
	}
	return "its design has no owner approval, and only Thomas records one"
}

// Refusal answers the lines under a refusal's headline for the spec at
// relative: why, the away rule and what fails it when Thomas is away, and the
// commands that check, judge and approve the design (example 5, AC-29).
func (a Admission) Refusal(relative string) []string {
	lines := []string{`  why: a design reaches implementation only once Thomas approved it (ai/META-DECISIONS.md, "How agents work")`}
	if a.Away != nil {
		lines = append(lines, "  while Thomas is away, only a judged fix spec with no departure starts without his approval ("+
			a.Away.From+" to "+a.Away.Until+"):")
		for _, problem := range a.AwayProblems {
			lines = append(lines, "    - "+problem)
		}
	}
	lines = append(lines,
		"  if the change only adds rows for decisions Thomas made, or follows a rename on a page a row quotes, "+
			"his earlier approval carries over: have ze-design-judge judge it, then run",
		"    ./le spec design carry spec "+relative)
	return append(lines,
		"  fix: otherwise run ./le spec design check spec "+relative+", have ze-design-judge judge it, present its ## "+
			presentationHeading+" to Thomas; he runs, from his own terminal:",
		"    ./le spec design approve spec "+relative+` words "<what he said>"`)
}

// Report answers the line a started spec's claim report carries: Thomas's
// approval (AC-23), or the away admission he reviews on return (AC-28).
func (a Admission) Report() string {
	if a.Approval.Away {
		return "admitted while Thomas is away (" + a.Approval.From + " to " + a.Approval.Until +
			"): no design approval; he reviews it on return"
	}
	approved := "approved: " + a.Approval.Date + ` "` + a.Approval.Words + `"`
	if a.Approval.CarriedFrom == "" {
		return approved
	}
	return approved + ", carried on " + a.Approval.CarriedOn + " to this text, which adds only his own decisions (judge: " +
		a.Approval.JudgeSentence + ")"
}

// OwedApprovalTrailer answers the Design-approved: line a commit carrying the
// spec at relative, with text, owes on the day now falls on: Thomas's approval
// from the local record whose digest matches, his own or the one carry wrote
// (AC-31, carry.go), else the away admission
// of the text (AC-28), and "" when no reachable commit is to carry it: one
// already carries his approval of this stem and digest, an away line already
// carries it and he has not approved it since, or nothing admits the text.
// An away line of the same digest does not stop his approval, so a later
// commit carries it to every checkout (AC-30, AC-31). A file
// name no record can key on owes none, since no approval of it can exist.
func OwedApprovalTrailer(root, relative, text string, now time.Time) (string, error) {
	spec, keyed := specText(relative, text)
	if !keyed {
		return "", nil
	}
	digest := DesignDigest(text)
	lookup := approvalLookup{root: root}
	trailers, err := lookup.load()
	if err != nil {
		return "", err
	}
	awayCarried := false
	for _, trailer := range trailers {
		if trailer.stem != spec.stem || trailer.digest != digest {
			continue
		}
		if !strings.HasPrefix(trailer.value, awayWord) {
			return "", nil
		}
		awayCarried = true
	}
	record, found, err := localApproval(root, spec.stem, digest)
	if err != nil {
		return "", err
	}
	// Thomas's approval, of this text or carried to it, is owed even over an
	// away trailer of the same text: the away line is no approval, and without
	// his trailer every other checkout keeps listing the spec as waiting for him.
	if found {
		return FormatApprovalTrailer(record.approval(spec.stem)), nil
	}
	if awayCarried {
		return "", nil
	}
	away, problems, err := awayAdmission(root, spec, now)
	if err != nil {
		return "", err
	}
	if away == nil || len(problems) != 0 {
		return "", nil
	}
	return FormatApprovalTrailer(Approval{Stem: spec.stem, Digest: digest, Away: true, From: away.From, Until: away.Until}), nil
}

// Departures answers the departs rows of a spec's Design documents table, as
// an approval lists them, so a closure can ask whether Thomas approved each.
func Departures(text string) []Departure {
	return departsRows(specFile{text: text, sections: readrecord.Sections(text)})
}

// specText is the spec at relative holding text. keyed is false for a file
// name the records cannot key on (specStem).
func specText(relative, text string) (spec specFile, keyed bool) {
	cleaned := path.Clean(filepath.ToSlash(relative))
	stem, isMarkdown := strings.CutSuffix(path.Base(cleaned), ".md")
	if !isMarkdown {
		return specFile{}, false
	}
	if !specStem.MatchString(stem) {
		return specFile{}, false
	}
	return specFile{relative: cleaned, stem: stem, text: text, sections: readrecord.Sections(text)}, true
}

// awayAdmission answers Thomas's absence on the day now falls on, nil when he
// is not away, and each condition of the away admission spec fails: a fix
// spec, no departs row, and a check that passes with the judge's current
// approval (AC-28, AC-29). Nothing is derived while he is not away.
func awayAdmission(root string, spec specFile, now time.Time) (*Away, []string, error) {
	away, found, err := ActiveAway(root, now)
	if err != nil {
		return nil, nil, err
	}
	if !found {
		return nil, nil, nil
	}
	var problems []string
	if !isFixSpec(spec.relative) {
		dir, err := fixesDir()
		if err != nil {
			return nil, nil, err
		}
		problems = append(problems, spec.name()+" is not a fix spec: only a spec under "+dir+"/ starts while Thomas is away")
	}
	for _, departure := range departsRows(spec) {
		problems = append(problems, spec.name()+" departs from "+departure.Page+" ("+departure.Section+")")
	}
	derived, err := derive(root, spec)
	if err != nil {
		return nil, nil, err
	}
	checked, err := checkSpec(root, spec, derived, judgeStepOwed)
	if err != nil {
		return nil, nil, err
	}
	return &away, append(problems, checked...), nil
}

// isFixSpec reports whether relative is a spec of the fixes category: the
// away rule covers "a fix spec's design" (ai/META-DECISIONS.md).
func isFixSpec(relative string) bool {
	bucket, placed := specpath.Bucket(relative)
	if !placed {
		return false
	}
	return bucket == specpath.Fixes
}

// fixesDir is the fixes category's directory, from its declaration. It
// refuses, rather than guess a directory, when specpath no longer declares
// the category.
func fixesDir() (string, error) {
	for _, category := range specpath.Categories() {
		if category.Name == specpath.Fixes {
			return category.Dir, nil
		}
	}
	return "", errors.New("specpath declares no " + specpath.Fixes + " category, so the away rule cannot name a fix spec")
}
