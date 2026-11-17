package env

import (
	"slices"
	"testing"
)

// TestWithDefaultsFillsOnlyWhatTheEnvironmentLeavesUnset proves the priority
// WithDefaults keeps: the environment it is given wins, under any spelling of
// the key, and a default fills only a key that environment gives no value.
// Method: each row is an environment and the defaults laid under it; the
// result is read back through LookupIn, which reads as a child's Get does,
// and through ShellSafe, which is what a forked plugin is handed.
func TestWithDefaultsFillsOnlyWhatTheEnvironmentLeavesUnset(t *testing.T) {
	const key = "ze.test.with.defaults"
	_ = MustRegister(EnvEntry{Key: key, Type: "string", Description: "WithDefaults test key (tests)"})

	rows := []struct {
		name     string
		environ  []string
		defaults []string
		want     string
	}{
		{"absent key is filled", []string{"PATH=/bin"}, []string{key + "=config"}, "config"},
		{"dotted spelling wins over the default", []string{key + "=operator"}, []string{key + "=config"}, "operator"},
		{"shell spelling wins over the default", []string{"ZE_TEST_WITH_DEFAULTS=operator"}, []string{key + "=config"}, "operator"},
		{"hyphen spelling wins over the default", []string{"ze-test-with-defaults=operator"}, []string{key + "=config"}, "operator"},
		{"a key set empty is not set, dotted", []string{key + "="}, []string{key + "=config"}, "config"},
		{"a key set empty is not set, shell spelling", []string{"ZE_TEST_WITH_DEFAULTS="}, []string{key + "=config"}, "config"},
		{"the first default of a key is kept", nil, []string{key + "=first", key + "=second"}, "first"},
		{"an empty default sets nothing", nil, []string{key + "="}, ""},
		{"no default", []string{key + "=operator"}, nil, "operator"},
	}
	// MUTATION: drop the `values[norm] != ""` guard, and the three "wins over
	// the default" rows answer "config".
	for _, row := range rows {
		before := slices.Clone(row.environ)
		filled := WithDefaults(row.environ, row.defaults)

		if got := LookupIn(filled, key); got != row.want {
			t.Errorf("%s: a child reads %q, want %q (environment %q)", row.name, got, row.want, filled)
		}
		if got := LookupIn(ShellSafe(filled), key); got != row.want {
			t.Errorf("%s: a child started through a shell reads %q, want %q", row.name, got, row.want)
		}
		if !slices.Equal(row.environ, before) {
			t.Errorf("%s: the environment given was changed: %q, was %q", row.name, row.environ, before)
		}
	}
}

// TestWithDefaultsKeepsEveryOtherVariable proves the environment it is given
// passes whole and in order, with the defaults after it.
func TestWithDefaultsKeepsEveryOtherVariable(t *testing.T) {
	environ := []string{"PATH=/bin", "HOME=/root", "not-an-entry"}

	filled := WithDefaults(environ, []string{"ze.log=debug"})

	want := []string{"PATH=/bin", "HOME=/root", "not-an-entry", "ze.log=debug"}
	if !slices.Equal(filled, want) {
		t.Errorf("WithDefaults = %q, want %q", filled, want)
	}
}
