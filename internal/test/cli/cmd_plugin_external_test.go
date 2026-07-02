package cli

import "testing"

// VALIDATES: cmdPluginExternal's argument and registry-lookup validation
// (the branches reachable without a live TLS server -- the TLS dial itself
// is exercised end-to-end by the .ci functional tests, not a Go unit test,
// matching NewFromTLSEnv's own untested-in-isolation precedent).
// PREVENTS: a usage or lookup error being silently swallowed instead of
// returning a clear nonzero exit.
func TestCmdPluginExternal_ArgValidation(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{"no args", nil},
		{"too many args", []string{"as112", "extra"}},
		{"unknown plugin", []string{"not-a-real-registered-plugin-name"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if code := cmdPluginExternal(tc.args); code != 1 {
				t.Fatalf("cmdPluginExternal(%v) = %d, want 1", tc.args, code)
			}
		})
	}
}
