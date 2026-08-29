// Design: docs/architecture/core-design.md -- native spec lifecycle checks
// Related: closure.go -- closure evidence derivation

package specstatus

import "github.com/ze-software/ze/internal/core/textbuf"

// closureSpec is one active spec and every signal used to classify it.
type closureSpec struct {
	Spec               string `json:"spec"`
	Stem               string `json:"stem"`
	Status             string `json:"status"`
	CompletedNotClosed bool   `json:"completed-not-closed"`
	NeedsVerification  bool   `json:"needs-verification"`
	IsUmbrella         bool   `json:"is-umbrella"`
	Evidence           string `json:"evidence-learned,omitempty"`
	LearnedExact       string `json:"learned-exact,omitempty"`
	JournalMatch       string `json:"journal-match,omitempty"`
	LearnedStem        string `json:"learned-stem,omitempty"`
	LearnedRef         string `json:"learned-ref,omitempty"`
	ReviewGatePresent  bool   `json:"review-gate-present"`
	UncheckedCloseBox  bool   `json:"unchecked-close-box"`
	GateFinished       bool   `json:"review-gate-finished"`
	Acknowledged       bool   `json:"acknowledged,omitempty"`
}

// ClosureReport is the closure inventory. Its rows are specs, so the shared
// command engine can count and filter them.
type ClosureReport []closureSpec

// Blocked reports whether a single-spec check must exit 3.
func (r ClosureReport) Blocked() bool {
	if len(r) != 1 {
		return false
	}
	if !r[0].CompletedNotClosed {
		return false
	}
	return !r[0].Acknowledged
}

// Text renders the producer-compatible two-tier closure advisory.
func (r ClosureReport) Text() string {
	if len(r) == 1 {
		if r[0].Acknowledged {
			var tb textbuf.Buffer
			return tb.Str("closure-ack present for ").Str(r[0].Stem).Str("; not blocking\n").String()
		}
	}
	if r.Blocked() {
		one := r[0]
		var tb textbuf.Buffer
		return tb.Str("Spec '").Str(one.Spec).Str("' is COMPLETED BUT NOT CLOSED.\n").
			Str("  Evidence: committed closure artifact ").Str(one.Evidence).Str(" exists,\n").
			Str("  but the spec is still in plan/ with Status=in-progress.\n").
			Str("  Close it (ai/rules/planning.md Spec Closure):\n").
			Str("    1. Finalize the Review Gate (0 BLOCKER, 0 ISSUE) via /ze-review.\n").
			Str("    2. Prepare closure commit: git rm ").Str(one.Spec).Str("\n").
			Str("  If it is genuinely still open, record why and proceed:\n").
			Str("    echo '<reason>' > tmp/session/.closure-ack-").Str(one.Stem).Byte('\n').String()
	}

	var flagged ClosureReport
	var possible ClosureReport
	for _, one := range r {
		if one.CompletedNotClosed {
			flagged = append(flagged, one)
		}
		if one.NeedsVerification {
			possible = append(possible, one)
		}
	}
	if len(flagged) == 0 {
		if len(possible) == 0 {
			return "No completed-but-not-closed specs.\n"
		}
	}
	var tb textbuf.Buffer
	if len(flagged) > 0 {
		tb.Str("Completed but not closed (").Int(int64(len(flagged))).Str(") -- high confidence:\n\n")
		for _, one := range flagged {
			tb.Str("  ").Str(one.Spec).Str("\n      evidence: own closure artifact ").Str(one.Evidence).Byte('\n')
		}
		tb.Str("\nClose each via ai/rules/planning.md Spec Closure (finalize gate, git rm spec).\n")
	}
	if len(possible) > 0 {
		tb.Str("\nPossibly closable -- NEEDS VERIFICATION (").Int(int64(len(possible))).Str("):\n").
			Str("  Weak signal (umbrella, or a child/sibling/predecessor learned\n").
			Str("  summary). Audit before closing -- most of these are false positives.\n\n")
		for _, one := range possible {
			why := "weak-match"
			if one.IsUmbrella {
				why = "umbrella"
			}
			tb.Str("  ").Str(one.Spec).Str("  [").Str(why).Str("]\n      signal: ").Str(one.Evidence).Byte('\n')
		}
	}
	return tb.String()
}
