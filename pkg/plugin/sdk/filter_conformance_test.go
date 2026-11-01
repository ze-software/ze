package sdk

import (
	"encoding/json"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// recordingReporter captures what the harness reports.
type recordingReporter struct{ errors []string }

func (r *recordingReporter) Helper() {}
func (r *recordingReporter) Errorf(format string, args ...any) {
	b, _ := json.Marshal(args) //nolint:errcheck // test capture of the arguments only
	r.errors = append(r.errors, format+" "+string(b))
}

func conformanceRoutes() []ConformanceRoute {
	return []ConformanceRoute{
		{Input: rpc.FilterUpdateInput{Filter: "f", Peer: "a", Fields: json.RawMessage(`{"med":7}`)}},
		{Input: rpc.FilterUpdateInput{Filter: "f", Peer: "b", Fields: json.RawMessage(`{"med":9}`)}},
		{Input: rpc.FilterUpdateInput{Filter: "f", Peer: "c"}},
	}
}

// snapshotEvaluator is a conforming evaluator: it reads its threshold from a
// snapshot its own goroutines swap atomically.
type snapshotEvaluator struct{ threshold atomic.Pointer[uint32] }

func (s *snapshotEvaluator) eval(route *FilterRoute) (*FilterUpdateOutput, error) {
	limit := *s.threshold.Load()
	fields, err := route.Fields()
	if err != nil {
		return nil, err
	}
	var read struct {
		MED uint32 `json:"med"`
	}
	if len(fields) > 0 {
		if err := json.Unmarshal(fields, &read); err != nil {
			return nil, err
		}
	}
	if read.MED > limit {
		return &FilterUpdateOutput{Action: rpc.FilterReject}, nil
	}
	return &FilterUpdateOutput{Action: rpc.FilterModify, Sets: []rpc.FilterSet{
		{Field: "local-preference", Action: rpc.FilterSetActionSet, Value: json.RawMessage(`200`)},
	}}, nil
}

// TestFastPathConformance proves AC-36: the harness refuses an evaluator
// that blocks past the per-call deadline, one that keeps the view past its
// return, and one that writes shared state without a snapshot, each naming
// the filter and the clause; a conforming evaluator passes, its concurrent
// calls run under -race while its snapshot is swapped. Method: run the
// harness with a recording reporter over each evaluator.
func TestFastPathConformance(t *testing.T) {
	t.Run("conforming", func(t *testing.T) {
		s := &snapshotEvaluator{}
		limit := uint32(8)
		s.threshold.Store(&limit)
		stop := make(chan struct{})
		swapped := make(chan struct{})
		go func() { // the plugin's own goroutine swapping the snapshot
			defer close(swapped)
			same := uint32(8)
			for {
				select {
				case <-stop:
					return
				default:
					s.threshold.Store(&same)
				}
			}
		}()
		var rep recordingReporter
		ok := CheckFilterConformance(&rep, "med-limit", s.eval, conformanceRoutes())
		close(stop)
		<-swapped
		if !ok || len(rep.errors) > 0 {
			t.Fatalf("a conforming evaluator was refused: %v", rep.errors)
		}
	})

	refused := []struct {
		name   string
		clause string
		eval   FilterEvaluator
	}{
		{"sleeper", "non-blocking clause", func(*FilterRoute) (*FilterUpdateOutput, error) {
			time.Sleep(2 * FilterCallDeadline)
			return &FilterUpdateOutput{Action: rpc.FilterAccept}, nil
		}},
		{"keeper", "borrowed-view clause", keeperEvaluator()},
		{"counter", "snapshot clause", counterEvaluator()},
	}
	for _, tc := range refused {
		t.Run(tc.name, func(t *testing.T) {
			var rep recordingReporter
			if CheckFilterConformance(&rep, tc.name, tc.eval, conformanceRoutes()) {
				t.Fatalf("%s passed the harness", tc.name)
			}
			joined := strings.Join(rep.errors, "\n")
			if !strings.Contains(joined, tc.clause) || !strings.Contains(joined, tc.name) {
				t.Fatalf("report %q does not name %s and the %s", joined, tc.name, tc.clause)
			}
		})
	}
}

// keeperEvaluator keeps each route and reads the previous one on its next call.
func keeperEvaluator() FilterEvaluator {
	var previous *FilterRoute
	return func(route *FilterRoute) (*FilterUpdateOutput, error) {
		if previous != nil {
			previous.Fields() //nolint:errcheck // the read itself is the defect under test
		}
		previous = route
		return &FilterUpdateOutput{Action: rpc.FilterAccept}, nil
	}
}

// counterEvaluator writes shared state and answers by it: every other call rejects.
func counterEvaluator() FilterEvaluator {
	calls := 0
	return func(*FilterRoute) (*FilterUpdateOutput, error) {
		calls++
		if calls%2 == 0 {
			return &FilterUpdateOutput{Action: rpc.FilterReject}, nil
		}
		return &FilterUpdateOutput{Action: rpc.FilterAccept}, nil
	}
}
