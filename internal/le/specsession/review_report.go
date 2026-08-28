// Design: docs/architecture/core-design.md -- native spec lifecycle support
// Related: review.go -- review artifact validation and persistence

package specsession

import "github.com/ze-software/ze/internal/core/textbuf"

// ReviewedFile pins one reviewed path to its content hash.
type ReviewedFile struct {
	Path string `json:"path"`
	Hash string `json:"hash"`
}

// reviewArtifact is the structured form of one recorded review.
type reviewArtifact struct {
	Path            string         `json:"path"`
	Spec            string         `json:"spec"`
	Verdict         string         `json:"verdict"`
	Rounds          int            `json:"rounds"`
	Reviewers       string         `json:"reviewers"`
	Model           string         `json:"model"`
	Timestamp       string         `json:"timestamp"`
	Files           []ReviewedFile `json:"files"`
	Findings        string         `json:"findings,omitempty"`
	RoundsReason    string         `json:"rounds-reason,omitempty"`
	OwnerAuthorised string         `json:"owner-authorised,omitempty"` //nolint:misspell // owner-authorised is a CLI keyword and a JSON key, not prose. It is named in ai/rules/planning.md, ai/skills/ze-close.md, ai/skills/ze-review.md and plan/TEMPLATE-CLOSURE.md, so the spelling is a contract and renaming it is the owner's decision
	ModelOverride   string         `json:"model-override,omitempty"`
	Warnings        []string       `json:"warnings,omitempty"`
}

// Document renders the persisted artifact format.
func (a reviewArtifact) Document() string {
	var tb textbuf.Buffer
	tb.Str("<!-- ze-review spec=").Str(a.Spec).Str(" verdict=").Str(a.Verdict).
		Str(" rounds=").Int(int64(a.Rounds)).Str(" reviewers=").Str(a.Reviewers).
		Str(" model=").Str(firstNonemptyString(a.Model, "unknown")).Str(" ts=").Str(a.Timestamp).Str(" -->\n").
		Str("# Independent review — ").Str(a.Spec).Str("\n\nfiles:\n")
	for _, file := range a.Files {
		tb.Str("  ").Str(file.Hash).Str("  ").Str(file.Path).Byte('\n')
	}
	if a.ModelOverride != "" {
		tb.Str("\nmodel-override: ").Str(a.ModelOverride).Byte('\n')
	}
	if a.RoundsReason != "" {
		tb.Str("\nrounds-reason: ").Str(a.RoundsReason).Byte('\n')
	}
	if a.OwnerAuthorised != "" {
		tb.Str("\nowner-authorised: ").Str(a.OwnerAuthorised).Byte('\n') //nolint:misspell // owner-authorised is a CLI keyword and a JSON key, not prose. It is named in ai/rules/planning.md, ai/skills/ze-close.md, ai/skills/ze-review.md and plan/TEMPLATE-CLOSURE.md, so the spelling is a contract and renaming it is the owner's decision
	}
	tb.Str("\n## Findings\n\n").Str(firstNonemptyString(a.Findings, "(none recorded)")).Str("\n")
	return tb.String()
}

// Text renders the record result for a person.
func (a reviewArtifact) Text() string {
	var tb textbuf.Buffer
	return tb.Str("review_gate: wrote ").Str(a.Path).Str(" (").Int(int64(len(a.Files))).
		Str(" files, verdict=").Str(a.Verdict).Str(")\n").String()
}

// ReviewCheck is the structured review gate verdict.
type ReviewCheck struct {
	Spec       string   `json:"spec"`
	Path       string   `json:"path"`
	Verdict    string   `json:"verdict,omitempty"`
	CodeFiles  int      `json:"code-files"`
	Blocked    bool     `json:"blocked"`
	Reason     string   `json:"reason,omitempty"`
	Unreviewed []string `json:"unreviewed,omitempty"`
	Stale      []string `json:"stale,omitempty"`
	Warnings   []string `json:"warnings,omitempty"`
}

// Text renders the review gate verdict.
func (c ReviewCheck) Text() string {
	var tb textbuf.Buffer
	if !c.Blocked {
		return tb.Str("review_gate: OK (").Int(int64(c.CodeFiles)).Str(" code files, clean, hashes match ").Str(c.Path).Str(")\n").String()
	}
	switch c.Reason {
	case reasonMissing:
		tb.Str("review-gate: BLOCKED — no independent-review artifact at ").Str(c.Path).Str("\n").
			Str("  Run an INDEPENDENT critical review (subagents / fresh session, never your own inline reasoning) and record it with le spec-session review record. See ai/rules/planning.md.\n")
	case keywordVerdict:
		tb.Str("review-gate: BLOCKED — review artifact ").Str(c.Path).Str(" verdict is '").Str(c.Verdict).Str("', not clean\n").
			Str("  Fix every BLOCKER/ISSUE, then re-run the independent review to a clean pass.\n")
	case "unreviewed":
		tb.Str("review-gate: BLOCKED — ").Int(int64(len(c.Unreviewed))).Str(" code file(s) in the commit were not covered by the review\n  Unreviewed: ")
		joinReviewPaths(&tb, c.Unreviewed)
		tb.Str("\n  Re-run the independent review over the FULL changeset and re-record.\n")
	case "stale":
		tb.Str("review-gate: BLOCKED — ").Int(int64(len(c.Stale))).Str(" reviewed file(s) changed AFTER the review (stale review)\n  Changed since review: ")
		joinReviewPaths(&tb, c.Stale)
		tb.Str("\n  Every fix is new code that needs a fresh review. Re-review and re-record.\n")
	}
	return tb.String()
}

// modelReport is the transcript-model command answer.
type modelReport struct {
	Transcript string `json:"transcript,omitempty"`
	Model      string `json:"model,omitempty"`
	ReviewTier bool   `json:"review-tier"`
	Readable   bool   `json:"readable"`
}

// Text renders the historical running-model CLI contract.
func (m modelReport) Text() string {
	if m.Model == "" {
		return "unknown\n"
	}
	return m.Model + "\n"
}

func joinReviewPaths(tb *textbuf.Buffer, paths []string) {
	for index, path := range paths {
		if index > 0 {
			tb.Str(", ")
		}
		tb.Str(path)
	}
}

func firstNonemptyString(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
