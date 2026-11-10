package env

import (
	"os"
	"testing"
)

// TestLookupInReadsWhatGetReads proves LookupIn answers a child environment
// the way Get answers the same entries in a process of its own.
// Method: each row sets its entries in this process, resets the cache and
// compares Get with LookupIn over the same entries, for every spelling and
// both orders of a dotted and an uppercase spelling.
func TestLookupInReadsWhatGetReads(t *testing.T) {
	const key = "ze.test.lookup.in"
	_ = MustRegister(EnvEntry{Key: key, Type: "string", Description: "LookupIn test key (tests)"})

	rows := []struct {
		name    string
		entries [][2]string
		want    string
	}{
		{"dotted", [][2]string{{"ze.test.lookup.in", "a"}}, "a"},
		{"uppercase", [][2]string{{"ZE_TEST_LOOKUP_IN", "b"}}, "b"},
		{"mixed case and hyphen", [][2]string{{"Ze.Test-Lookup.In", "c"}}, "c"},
		{"uppercase after dotted", [][2]string{{"ze.test.lookup.in", "dot"}, {"ZE_TEST_LOOKUP_IN", "upper"}}, "dot"},
		{"dotted after uppercase", [][2]string{{"ZE_TEST_LOOKUP_IN", "upper"}, {"ze.test.lookup.in", "dot"}}, "dot"},
		{"absent", nil, ""},
	}
	// MUTATION: drop the dotted-wins rule from effectiveValues, and LookupIn
	// answers "upper" in the uppercase-after-dotted row.
	for _, row := range rows {
		environ := make([]string, 0, len(row.entries))
		for _, name := range []string{"ze.test.lookup.in", "ZE_TEST_LOOKUP_IN", "Ze.Test-Lookup.In"} {
			t.Setenv(name, "")
			os.Unsetenv(name) //nolint:errcheck // t.Setenv restores it
		}
		for _, entry := range row.entries {
			t.Setenv(entry[0], entry[1])
			environ = append(environ, entry[0]+"="+entry[1])
		}
		ResetCache()
		if got := Get(key); got != row.want {
			t.Fatalf("%s: Get = %q, want %q: the row no longer describes Get", row.name, got, row.want)
		}
		if got := LookupIn(environ, key); got != row.want {
			t.Errorf("%s: LookupIn = %q, Get = %q", row.name, got, row.want)
		}
	}
	ResetCache()
}
