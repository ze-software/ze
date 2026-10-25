// Related: kind.go -- the closed kind set these tests pin

package leaction

import (
	"errors"
	"testing"
)

// VALIDATES: D-Q3 -- every kind in the set has a name, a meaning and a check,
// and no two kinds share a name.
// PREVENTS: a kind a parameter can declare that a help page renders blank, or
// that a completion callback cannot tell from another.
func TestEveryKindIsDeclaredWithAMeaningAndValidator(t *testing.T) {
	all := Kinds()
	if len(all) == 0 {
		t.Fatal("the kind set is empty")
	}
	names := make(map[string]Kind, len(all))
	for _, kind := range all {
		if kind == KindUnspecified {
			t.Fatal("KindUnspecified is in the declared set, so a parameter could publish it")
		}
		declaration := declarationOf(kind)
		if declaration.name == "" {
			t.Errorf("kind %d has no name", kind)
		}
		if declaration.meaning == "" {
			t.Errorf("kind %s has no meaning", declaration.name)
		}
		if declaration.check == nil {
			t.Errorf("kind %s has no syntax check", declaration.name)
		}
		if other, taken := names[declaration.name]; taken {
			t.Errorf("kinds %d and %d share the name %q", other, kind, declaration.name)
		}
		names[declaration.name] = kind
	}
	for _, want := range []string{"switch", "string", "enum", "uint", "duration", "path", "revision", "spec", "label", "area"} {
		if _, declared := names[want]; !declared {
			t.Errorf("the kind set lacks %q, which D-Q3 names", want)
		}
	}
}

// VALIDATES: R-17 -- a kind checks syntax only: uint, duration and enum refuse
// a malformed value, and path and spec accept one naming nothing on disk.
// PREVENTS: a check that asks whether a path exists, which refuses every
// output path a caller names before the action creates it.
func TestKindValidatorsCheckSyntaxOnly(t *testing.T) {
	members := []string{"build", "test"}
	cases := []struct {
		kind  Kind
		value string
		ok    bool
	}{
		{KindUint, "0", true},
		{KindUint, "18446744073709551615", true},
		{KindUint, "18446744073709551616", false},
		{KindUint, "-1", false},
		{KindUint, "auto", false},
		{KindDuration, "90s", true},
		{KindDuration, "2h30m", true},
		{KindDuration, "ninety", false},
		{KindEnum, "build", true},
		{KindEnum, "deploy", false},
		{KindList, "build,test", true},
		{KindList, "build,,test", false},
		{KindList, "build,deploy", false},
		{KindPath, "tmp/does/not/exist/yet.json", true},
		{KindPath, "", false},
		{KindSpec, "plan/no-such-spec.md", true},
		{KindRevision, "HEAD~2", true},
		{KindRevision, "two words", false},
		{KindLabel, "unit-pkg", true},
		{KindLabel, "unit pkg", false},
		{KindString, "", true},
		{KindSwitch, "", true},
		{KindSwitch, "on", false},
	}
	for _, one := range cases {
		err := one.kind.Check(one.value, members)
		if one.ok && err != nil {
			t.Errorf("%s refused %q: %v", one.kind, one.value, err)
		}
		if !one.ok && err == nil {
			t.Errorf("%s accepted %q", one.kind, one.value)
		}
		if !one.ok && err != nil && !errors.Is(err, errValue) {
			t.Errorf("%s refused %q with an error that does not wrap errValue: %v", one.kind, one.value, err)
		}
	}
}
