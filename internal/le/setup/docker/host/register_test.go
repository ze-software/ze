package dockerhost

import (
	"slices"
	"testing"

	leroot "github.com/ze-software/ze/internal/le/le/root"
)

// VALIDATES: `./le setup docker host` is registered with le's dispatcher and
// lists build, start, stop and check, with build's arch, size and pins
// keywords.
// PREVENTS: an area that compiles but that `./le` cannot reach.
func TestDockerHostAreaIsRegistered(t *testing.T) {
	if !leroot.Owns(dockerHostArea) {
		t.Fatalf("leroot does not own %q", dockerHostArea)
	}
	list, found := leroot.ActionsOf(dockerHostArea)
	if !found {
		t.Fatalf("no actions registered for %q", dockerHostArea)
	}
	var verbs []string
	for _, row := range list.Actions {
		verbs = append(verbs, row.Verb)
		if row.Verb != "build" {
			continue
		}
		var keywords []string
		for _, parameter := range row.Parameters {
			keywords = append(keywords, parameter.Keyword)
		}
		if !slices.Equal(keywords, []string{archKeyword, sizeKeyword, pinsKeyword}) {
			t.Errorf("build keywords %v, want arch, size, pins", keywords)
		}
	}
	if !slices.Equal(verbs, []string{"build", "start", "stop", "check"}) {
		t.Errorf("verbs %v, want build, start, stop, check", verbs)
	}
}
