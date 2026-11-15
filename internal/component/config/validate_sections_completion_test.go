package config

import (
	"testing"

	"github.com/ze-software/ze/internal/component/config/yang"
)

// TestCompletionOnlyAnnotationOwesNoWalk decides which ze:validate names make
// a section "declaring" for TestEveryValidatorSectionIsWalkedOrExcused.
//
// Goal: a section whose only annotation is a SUGGESTION (a CompleteFn and no
// ValidateFn, the RegisterSuggestion shape) owes neither a walk nor an excuse,
// because the walk would run nothing for it, while a real rule, a name nobody
// registered, and a union that holds one rule still count.
// Method: annotationRefuses over a registry holding one rule and one
// suggestion, through every shape a ze:validate argument takes.
// PREVENTS: the firewall section, whose only annotations are the irr owner's
// AS-SET suggestion, reading as an annotation that silently does nothing.
func TestCompletionOnlyAnnotationOwesNoWalk(t *testing.T) {
	reg := yang.NewValidatorRegistry()
	reg.Register("a-rule", yang.CustomValidator{ValidateFn: func(string, any) error { return nil }})
	reg.Register("a-hint", yang.CustomValidator{CompleteFn: func() []string { return []string{"X"} }})

	cases := []struct {
		arg  string
		want bool
	}{
		{"", false},
		{"a-hint", false},
		{"a-rule", true},
		{"nobody-registered-this", true},
		{"a-hint|a-rule", true},
		{"a-hint|nobody-registered-this", true},
	}
	for _, c := range cases {
		if got := annotationRefuses(c.arg, reg); got != c.want {
			t.Errorf("annotationRefuses(%q) = %v, want %v", c.arg, got, c.want)
		}
	}
}
