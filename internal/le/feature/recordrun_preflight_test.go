// VALIDATES: feature run refuses on a host that cannot run its .ci, before any item runs.
// PREVENTS: a feature run that builds and runs every Go test before the first .ci fails on a missing loopback address.

package feature

import (
	"errors"
	"testing"
)

// hostFine is the preflight of a host that can run every item.
func hostFine([]string) error { return nil }

// TestRecordRunRefusesBeforeAnyItemRuns: a preflight that refuses stops the
// record before the first item runs, writes nothing, and answers the
// preflight's own refusal. The method counts the item runner's calls.
func TestRecordRunRefusesBeforeAnyItemRuns(t *testing.T) {
	tree := recordedTree(t)
	missing := errors.New("this host is missing loopback addresses")
	ran := 0
	item := func(item string) (observation, error) {
		ran++
		return fakeRunner(passOutputs)(item)
	}
	var judged []string
	preflight := func(items []string) error {
		judged = items
		return missing
	}
	_, err := recordRun(tree, "widget", runners{item: item, preflight: preflight}, "abc", "2026-10-07")
	if !errors.Is(err, missing) {
		t.Fatalf("error %v, want the preflight's refusal", err)
	}
	if ran != 0 {
		t.Fatalf("%d items ran on a host the preflight refused", ran)
	}
	if len(judged) != 2 {
		t.Fatalf("the preflight judged %v, want both real-path items", judged)
	}
	requireNoRecord(t, tree)
}

// TestRepoRunnerPreflightChecksOnlyAHostCI: only a .ci that runs on this host
// consults the host check. A Go test, an editor test and a .ci whose home is
// the QEMU guest start on any host. The method counts the calls to a host
// check that always refuses.
func TestRepoRunnerPreflightChecksOnlyAHostCI(t *testing.T) {
	missing := errors.New("this host is missing loopback addresses")
	cases := []struct {
		name  string
		items []string
		guest bool
		calls int
	}{
		{"go test only", []string{"internal/widget/widget_test.go::TestWidget"}, false, 0},
		{"editor test only", []string{editorItem}, false, 0},
		{"guest ci", []string{"test/plugin/widget.ci"}, true, 0},
		{"host ci after a go test", []string{"internal/widget/widget_test.go::TestWidget",
			"test/plugin/widget.ci"}, false, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			runner := &repoRunner{tree: t.TempDir(),
				needsGuest: func(string) (bool, error) { return tc.guest, nil },
				hostReady: func() error {
					calls++
					return missing
				}}
			err := runner.preflight(tc.items)
			if calls != tc.calls {
				t.Fatalf("the host was checked %d times, want %d", calls, tc.calls)
			}
			if tc.calls > 0 && !errors.Is(err, missing) {
				t.Fatalf("error %v, want the host's refusal", err)
			}
			if tc.calls == 0 && err != nil {
				t.Fatalf("refused a run that binds no loopback address: %v", err)
			}
		})
	}
}
