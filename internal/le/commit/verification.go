// Design: docs/architecture/testing/verify-freshness-scope.md -- what the certificate proves at commit preparation
package commit

import (
	"strings"

	verifyengine "github.com/ze-software/ze/internal/le/verify/engine"
)

// VerificationState records what the latest native verify status proves for
// the prospective commit's explicit paths.
type VerificationState struct {
	State     string `json:"state"`
	Detail    string `json:"detail"`
	Mode      string `json:"mode,omitempty"`
	Timestamp string `json:"timestamp,omitempty"`
	Commit    string `json:"commit,omitempty"`
}

// The two VerificationState values the commit flow branches on: the
// certificate covers the commit's paths, or no native gate judges this tree.
const (
	verifyFresh         = "fresh"
	verifyNotApplicable = "not-applicable"
)

// verificationState answers what the verification certificate proves for the
// commit's paths. It is reported, never a refusal: a red prevents a release,
// never a commit (ai/META-DECISIONS.md, "Ze is pre-release"), so commit
// preparation reads no failure index and charges no red.
func verificationState(root string, paths []string) VerificationState {
	freshness := verifyengine.CheckCertificate(root, paths)
	state := VerificationState{
		State:     "stale",
		Detail:    freshness.Reason,
		Mode:      freshness.Mode,
		Timestamp: freshness.Timestamp,
		Commit:    freshness.GitSHA,
	}
	if freshness.Fresh {
		state.State = verifyFresh
	}
	return state
}

// ciDetail is what the create report says when no local verify record covers
// the commit's paths: nothing is owed, because no agent runs ./le verify and
// GitHub CI verifies the pushed commit (ai/META-DECISIONS.md).
const ciDetail = "GitHub CI verifies the pushed commit; an agent's evidence is its scoped test runs"

// label renders the create report's `verify=` value. A fresh record is named
// with its detail. A stale or absent one is CI's to answer, so the report
// never reads as a verify an agent owes.
func (v VerificationState) label() string {
	switch v.State {
	case verifyFresh, verifyNotApplicable:
		return strings.ToUpper(v.State) + " (" + v.Detail + ")"
	}
	return "CI (" + ciDetail + ")"
}
