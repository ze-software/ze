package runner

import (
	"strconv"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/env"
)

// TestNestedRunnerRetainsParentContention checks both consumers of an enclosing
// runner's factor: the measured child deadline and the child's published factor.
// A serial debug run stays tight; nested parallel runs apply the factor once.
func TestNestedRunnerRetainsParentContention(t *testing.T) {
	for _, test := range []struct {
		name   string
		parent string
		local  int
		want   int
	}{
		{"isolated single case", "1", 1, 1},
		{"stress siblings with one selected case", "3", 1, 3},
		{"parallel cases inside parallel stress", "3", 8, 3},
		{"local parallel cases", "1", 8, 3},
		{"malformed parent keeps floor", "invalid", 1, 1},
		{"negative parent keeps floor", "-2", 1, 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Cleanup(env.ResetCache)
			t.Setenv(ParallelFactorEnv, test.parent)
			env.ResetCache()
			r := &Runner{concurrency: test.local}
			if got := r.parallelFactor(); got != test.want {
				t.Errorf("factor = %d, want %d", got, test.want)
			}
			if got := r.withParallelHeadroom(10 * time.Second); got != time.Duration(test.want)*10*time.Second {
				t.Errorf("measured child budget = %s, want %ds", got, 10*test.want)
			}
			if got := env.LookupIn([]string{r.parallelFactorEnv()}, ParallelFactorEnv); got != strconv.Itoa(test.want) {
				t.Errorf("published child factor = %q, want %d", got, test.want)
			}
			if got := env.LookupIn([]string{r.testBudgetEnv(10 * time.Second)}, TestBudgetEnv); got != (time.Duration(test.want) * 10 * time.Second).String() {
				t.Errorf("published child budget = %q, want %ds", got, 10*test.want)
			}
		})
	}
}
