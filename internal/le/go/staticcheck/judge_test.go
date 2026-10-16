package gostaticcheck

import (
	"slices"
	"strings"
	"testing"
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
