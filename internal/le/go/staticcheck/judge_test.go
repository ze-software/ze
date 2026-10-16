package gostaticcheck

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// TestJudgeRunsWithPathIndependentCompiles proves the environment Staticcheck
// runs under adds -trimpath to GOFLAGS, keeps every inherited flag, never adds
// it twice, and pins the gate's own cache.
//
// Method: build the environment from three inherited GOFLAGS shapes and read
// the entry the child sees, which is the LAST one for a key. Without -trimpath
// every compile action ID carries the tree's absolute directory, so a verify
// run in a fresh tree compiles the whole module cold and the matrix exceeds
// its deadline (judgeEnvironment).
func TestJudgeRunsWithPathIndependentCompiles(t *testing.T) {
	cases := []struct {
		name    string
		environ []string
		want    string
	}{
		{"no GOFLAGS", []string{"HOME=/h"}, "-trimpath"},
		{"inherited flags kept", []string{"GOFLAGS=-mod=vendor"}, "-mod=vendor -trimpath"},
		{"already present", []string{"GOFLAGS=-trimpath -mod=vendor"}, "-trimpath -mod=vendor"},
		{"last inherited entry wins", []string{"GOFLAGS=-race", "GOFLAGS=-mod=vendor"}, "-mod=vendor -trimpath"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := judgeEnvironment(tc.environ, "/c")
			if flags := lastValue(got, goFlagsKey); flags != tc.want {
				t.Errorf("GOFLAGS = %q, want %q", flags, tc.want)
			}
			if cache := lastValue(got, cacheKey); cache != "/c" {
				t.Errorf("STATICCHECK_CACHE = %q, want /c", cache)
			}
			if !slices.Equal(got[:len(tc.environ)], tc.environ) {
				t.Errorf("inherited environment changed: %q", got[:len(tc.environ)])
			}
		})
	}
}

// TestJudgeCompilesBeforeTheDeadlineStarts proves Judge compiles the widest row
// Staticcheck will load before the Staticcheck deadline starts, with the
// arguments Staticcheck's own load passes and the environment it runs under.
//
// Method: a fake `go` on PATH records its arguments and GOFLAGS, then takes
// longer than the deadline Judge is given; a fake `staticcheck` answers clean at
// once. A Judge that compiled inside the deadline, or not at all, reports the
// matrix unjudged or leaves the record without the widest row's line. A change to a
// package most of the module imports leaves every dependent to compile again,
// and piece 1 of the next verify paid that compile inside a bound sized for the
// type check alone (warmBuildCache).
func TestJudgeCompilesBeforeTheDeadlineStarts(t *testing.T) {
	bin := t.TempDir()
	record := filepath.Join(t.TempDir(), "go-calls.txt")
	// sleep(external): the fake compiler must outlast the Staticcheck deadline
	// to prove the deadline does not include the compile; no condition exists
	// to wait on because the delay IS the property under test.
	fakeGo := "#!/bin/sh\necho \"GOFLAGS=$GOFLAGS $*\" >> " + record + "\nsleep 1.5\n"
	fakeStaticcheck := "#!/bin/sh\ncat > /dev/null\nexit 0\n"
	for name, body := range map[string]string{"go": fakeGo, "staticcheck": fakeStaticcheck} {
		if err := os.WriteFile(filepath.Join(bin, name), []byte(body), 0o700); err != nil { //nolint:gosec // an executable test double
			t.Fatalf("write the fake %s: %v", name, err)
		}
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("GOFLAGS", "")

	matrix := Matrix{
		{Name: "core_only", Tags: []string{"ze_a"}},
		{Name: "without_ze_c", Tags: []string{"ze_a", "ze_b"}, Omits: "ze_c"},
		{Name: "without_ze_b", Tags: []string{"ze_a", "ze_c"}, Omits: "ze_b"},
	}
	verdict, judged, err := Judge(t.TempDir(), matrix, time.Second)
	if !judged {
		t.Fatalf("matrix unjudged (%v): the compile ran inside the Staticcheck deadline", err)
	}
	if !verdict.Passed {
		t.Fatalf("verdict failed over a clean fake Staticcheck: %+v", verdict)
	}

	raw, err := os.ReadFile(record)
	if err != nil {
		t.Fatalf("no compile ran before Staticcheck: %v", err)
	}
	const load = "list -e -compiled=true -test=true -export=true -deps=true -find=false -buildvcs=false -pgo=off "
	want := []string{"GOFLAGS=-trimpath " + load + "-tags=ze_a,ze_b -- ./..."}
	if got := strings.Split(strings.TrimSpace(string(raw)), "\n"); !slices.Equal(got, want) {
		t.Errorf("compiles before Staticcheck:\n got %q\nwant %q", got, want)
	}
}

// lastValue answers the value the child process reads for key: the last entry.
func lastValue(environ []string, key string) string {
	value := ""
	for _, entry := range environ {
		if rest, found := strings.CutPrefix(entry, key+"="); found {
			value = rest
		}
	}
	return value
}
