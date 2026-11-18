// Design: docs/architecture/testing/runner-architecture.md -- shared fixture schema publication.

package externalyang

import (
	"slices"
	"testing"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestOutcomeQuestionsKeepRecordBoundary pins the actual request order and
// useful healthy controls on both sides of the record writer's own threshold.
func TestOutcomeQuestionsKeepRecordBoundary(t *testing.T) {
	for _, outcome := range []string{"partial", "failed", "warning"} {
		t.Run(outcome, func(t *testing.T) {
			want := []OutcomeQuestion{
				{Command: "show answer-outcome healthy short", Outcome: "healthy", Count: 2},
				{Command: "show answer-outcome " + outcome + " short", Outcome: outcome, Count: 2},
				{Command: "show answer-outcome healthy long", Outcome: "healthy", Count: rpc.AnswerBufferThreshold + 3},
				{Command: "show answer-outcome " + outcome + " long", Outcome: outcome, Count: rpc.AnswerBufferThreshold + 3},
			}
			got := OutcomeQuestions(outcome)
			if !slices.Equal(got, want) {
				t.Fatalf("outcome requests = %+v, want %+v", got, want)
			}
			ids := make([]string, 0, len(got))
			for _, question := range got {
				ids = append(ids, OutcomeToolID(question))
			}
			if wantIDs := []string{"healthy-short", outcome + "-short", "healthy-long", outcome + "-long"}; !slices.Equal(ids, wantIDs) {
				t.Errorf("outcome tool identifiers = %v, want %v", ids, wantIDs)
			}
		})
	}
}
