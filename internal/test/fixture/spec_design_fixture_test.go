package fixture

import (
	"slices"
	"testing"
)

func TestWithRepoRootEnvReplacesBothSpellings(t *testing.T) {
	got := withRepoRootEnv([]string{
		"PATH=/usr/bin",
		"ZE_REPO_ROOT=/old",
		"ze.repo.root=/older",
		"KEEP=1",
	}, "/fixture/repo")

	want := []string{"PATH=/usr/bin", "KEEP=1", "ZE_REPO_ROOT=/fixture/repo"}
	if !slices.Equal(got, want) {
		t.Fatalf("withRepoRootEnv = %v, want %v", got, want)
	}
}
