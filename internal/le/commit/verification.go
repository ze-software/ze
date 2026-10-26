// Design: docs/architecture/testing/verify-freshness-scope.md -- what the certificate proves at commit preparation
package commit

import (
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
