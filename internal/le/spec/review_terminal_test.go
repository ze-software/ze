// Design: docs/contributing/spec-workflow.md -- owner authorization issuance at every round.
package spec

import (
	"errors"
	"maps"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/le/human"
	"github.com/ze-software/ze/internal/le/le/human/humantest"
)

// TestOwnerReviewTerminalBoundary drives recordReview below and above the cap,
// including replacement artifacts, and proves refusal leaves the tree untouched.
func TestOwnerReviewTerminalBoundary(t *testing.T) {
	for _, mode := range []string{"terminal", "piped-stdin", "piped-stdout", "piped-both", "closed-stdin", "closed-stdout"} {
		t.Run(mode, func(t *testing.T) {
			humantest.Streams(t, mode)
			for _, rounds := range []int{1, 6} {
				name := "below-cap"
				if rounds == 6 {
					name = "above-cap"
				}
				t.Run(name, func(t *testing.T) {
					for _, existing := range []bool{false, true} {
						name := "new"
						if existing {
							name = "replacement"
						}
						t.Run(name, func(t *testing.T) {
							root := reviewFixture(t)
							request := reviewRecord{Spec: "demo", Verdict: "clean", Rounds: 1,
								Files: []string{"pkg/a.go"}, Model: "fixture", SessionID: "session"}
							if existing {
								if _, err := recordReview(root, request); err != nil {
									t.Fatal(err)
								}
							}
							request.Rounds = rounds
							request.RoundsReason = "an extra pass found a product defect"
							request.OwnerAuthorised = "Thomas asked for this pass"
							before := humantest.Snapshot(t, root)
							artifact, err := recordReview(root, request)
							if mode == "terminal" {
								if err != nil {
									t.Fatal(err)
								}
								if artifact.OwnerAuthorised != request.OwnerAuthorised || artifact.Rounds != rounds {
									t.Fatalf("recorded artifact = %+v", artifact)
								}
								return
							}
							var refusal *human.Refusal
							if !errors.As(err, &refusal) {
								t.Errorf("owner review answered %v, want terminal refusal", err)
							} else if strings.Contains(err.Error(), "full verification") || !strings.Contains(err.Error(), "./le spec review record") {
								t.Errorf("authorization refusal = %v", err)
							}
							if !maps.Equal(before, humantest.Snapshot(t, root)) {
								t.Error("refused owner review changed files or directories")
							}
						})
					}
				})
			}
		})
	}
}

// TestOrdinaryReviewAndRecordedAuthorizationRemainNoninteractive proves normal
// recording and consumption do not inherit the owner issuance gate.
func TestOrdinaryReviewAndRecordedAuthorizationRemainNoninteractive(t *testing.T) {
	root := reviewFixture(t)
	request := reviewRecord{Spec: "demo", Verdict: "clean", Rounds: 1,
		Files: []string{"pkg/a.go"}, Model: "fixture", SessionID: "session"}
	t.Run("record-owner", func(t *testing.T) {
		humantest.Streams(t, "terminal")
		request.OwnerAuthorised = "Thomas asked for this pass"
		if _, err := recordReview(root, request); err != nil {
			t.Fatal(err)
		}
	})
	humantest.Streams(t, "piped-both")
	check, err := CheckReview(root, "demo", "session", request.Files)
	if err != nil || check.Blocked {
		t.Fatalf("recorded owner review consumption: %+v, %v", check, err)
	}
	request.OwnerAuthorised = ""
	if _, err := recordReview(root, request); err != nil {
		t.Fatalf("ordinary noninteractive review: %v", err)
	}
}
