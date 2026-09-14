// Design: auth.go -- AuthMode is the typed form of the auth-mode leaf
//
// Goal: prove the auth modes an operator can write at environment/mcp/auth-mode
// are the modes ParseAuthMode reads and String writes, so a word cannot exist
// on one side alone. Method: read the enumeration out of the loaded model with
// configyang.EnumValues, which fails on a leaf that declares no enumeration,
// round-trip each word through the parser, and walk every typed mode back to
// the model.

package mcp

import (
	"slices"
	"testing"

	configyang "github.com/ze-software/ze/internal/component/config/yang"

	// The blank import registers ze-mcp-conf with the loader, which declares
	// the leaf read below.
	_ "github.com/ze-software/ze/internal/component/mcp/yang"
)

const authModeLeaf = "environment/mcp/auth-mode"

// authModeMax bounds the walk over the typed modes: String answers
// "unspecified" past the last one, and the walk stops there.
const authModeMax = 16

// TestAuthModesMatchTheModel holds AuthMode to the model in both directions.
func TestAuthModesMatchTheModel(t *testing.T) {
	model, err := configyang.EnumValues(authModeLeaf)
	if err != nil {
		t.Fatalf("read the enumeration at %s: %v", authModeLeaf, err)
	}

	for _, word := range model {
		mode, err := ParseAuthMode(word)
		if err != nil {
			t.Errorf("%q is offered at %s and ParseAuthMode refuses it: %v", word, authModeLeaf, err)
			continue
		}
		if mode.String() != word {
			t.Errorf("%q parses to %d, which prints as %q rather than the word an operator wrote", word, mode, mode.String())
		}
	}

	typed := []string{}
	for mode := AuthMode(1); mode < authModeMax; mode++ {
		word := mode.String()
		if word == AuthUnspecified.String() {
			break
		}
		typed = append(typed, word)
	}
	slices.Sort(typed)
	if !slices.Equal(model, typed) {
		t.Errorf("the auth modes disagree: the model at %s holds %v and AuthMode prints %v. "+
			"A word only Go carries is a mode no operator can ask for",
			authModeLeaf, model, typed)
	}
}
