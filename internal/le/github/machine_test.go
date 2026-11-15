// Design: docs/contributing/github-integration.md -- observation never executes a handoff.

package github

import (
	"errors"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/job"
	leaction "github.com/ze-software/ze/internal/le/le/action"
)

func machineFixture() machineSources {
	return machineSources{
		root:      func() (string, error) { return "/fixture/main", nil },
		head:      func(string) string { return testRevision },
		inputHash: func(string) (string, error) { return strings.Repeat("a", 64), nil },
		dirty:     func(string) (bool, error) { return false, nil },
		branch:    func(string) (string, error) { return "main", nil },
		linked:    func(string) (bool, error) { return false, nil },
		hostname:  func() (string, error) { return "fixture-machine", nil },
		capacity: func(string) (*job.Overview, error) {
			return &job.Overview{Slots: 4, Free: 4, ProcessTable: "read", Waiters: []job.WaiterRow{},
				Machine: job.Machine{MemoryAvailable: job.Measured(1 << 30)}}, nil
		},
	}
}

// TestGitHubMachineReportReachesLocalObservation drives the same registered
// action factory with actual observation dependencies replaced, not a claimed
// previous test record. Its payload cannot say that any test was executed.
func TestGitHubMachineReportReachesLocalObservation(t *testing.T) {
	observations := 0
	source := machineFixture()
	source.head = func(string) string { observations++; return testRevision }
	payload, code := reportArea(func(args leaction.Arguments) (any, int) {
		if !args.Has("machine") {
			t.Fatal("machine keyword was not parsed")
		}
		report, err := observeMachine(args.One("revision"), source)
		if err != nil {
			t.Fatal(err)
		}
		return report, 0
	}).Answer([]string{"report", "machine", "revision", testRevision})
	if code != 0 {
		t.Fatalf("machine action returned %d", code)
	}
	report, ok := payload.(machineReport)
	if !ok {
		t.Fatalf("payload = %T", payload)
	}
	if !report.Ready {
		t.Fatalf("ready facts were rejected: %+v", report)
	}
	if observations != 2 {
		t.Errorf("HEAD observations = %d, want before and after", observations)
	}
	if report.Revision != testRevision {
		t.Error("actual HEAD was lost")
	}
	if report.Host != "fixture-machine" {
		t.Error("actual machine identity was lost")
	}
	if report.Execution != "not-started" {
		t.Error("observation claimed to have executed a test")
	}
	if report.Capacity.Free != 4 {
		t.Error("actual capacity was lost")
	}
}

// TestGitHubMachineHandoffRefusesUnboundInputs proves every readiness failure
// remains visible without running a test, transport or destructive command.
func TestGitHubMachineHandoffRefusesUnboundInputs(t *testing.T) {
	for _, test := range []struct {
		name      string
		change    func(*machineSources)
		requested string
		want      string
	}{
		{name: "dirty", change: func(source *machineSources) { source.dirty = func(string) (bool, error) { return true, nil } }, want: "dirty"},
		{name: "mismatched HEAD", requested: strings.Repeat("f", 40), want: "differs"},
		{name: "missing HEAD", change: func(source *machineSources) { source.head = func(string) string { return job.Unknown } }, want: "HEAD"},
		{name: "unreachable local source", change: func(source *machineSources) {
			source.root = func() (string, error) { return "", errors.New("source unreadable") }
		}, want: "unreadable"},
		{name: "input read failure", change: func(source *machineSources) {
			source.inputHash = func(string) (string, error) { return "", errors.New("input unavailable") }
		}, want: "unavailable"},
		{name: "input race", change: func(source *machineSources) {
			reads := 0
			source.inputHash = func(string) (string, error) {
				reads++
				if reads == 1 {
					return "before", nil
				}
				return "after", nil
			}
		}, want: "changed during"},
		{name: "HEAD race", change: func(source *machineSources) {
			reads := 0
			source.head = func(string) string {
				reads++
				if reads == 1 {
					return testRevision
				}
				return strings.Repeat("f", 40)
			}
		}, want: "HEAD changed"},
		{name: "linked worktree", change: func(source *machineSources) { source.linked = func(string) (bool, error) { return true, nil } }, want: "linked worktree"},
		{name: "wrong branch", change: func(source *machineSources) { source.branch = func(string) (string, error) { return "topic", nil } }, want: "main checkout"},
		{name: "busy", change: func(source *machineSources) {
			source.capacity = func(string) (*job.Overview, error) { return &job.Overview{ProcessTable: "read", Free: 0}, nil }
		}, want: "no native job slot"},
		{name: "capacity unknown", change: func(source *machineSources) {
			source.capacity = func(string) (*job.Overview, error) { return nil, errors.New("capacity unreadable") }
		}, want: "capacity unreadable"},
		{name: "process unknown", change: func(source *machineSources) {
			source.capacity = func(string) (*job.Overview, error) { return &job.Overview{ProcessTable: "unavailable", Free: 4}, nil }
		}, want: "liveness is unavailable"},
		{name: "waiter", change: func(source *machineSources) {
			source.capacity = func(string) (*job.Overview, error) {
				return &job.Overview{ProcessTable: "read", Free: 4, Waiters: []job.WaiterRow{{Label: "other"}}}, nil
			}
		}, want: "already waiting"},
	} {
		t.Run(test.name, func(t *testing.T) {
			source := machineFixture()
			if test.change != nil {
				test.change(&source)
			}
			report, err := observeMachine(test.requested, source)
			if err == nil {
				t.Fatal("unbound input was declared ready")
			}
			if report.Ready {
				t.Fatal("failed observation claimed readiness")
			}
			if report.Execution != "not-started" {
				t.Fatal("failed observation started a test")
			}
			if !strings.Contains(strings.Join(report.Problems, ";"), test.want) {
				t.Fatalf("problems = %v", report.Problems)
			}
		})
	}
}
