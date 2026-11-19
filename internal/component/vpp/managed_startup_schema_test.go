//go:build linux || darwin

// Design: docs/guide/vpp.md -- startup-file candidate validation
// Related: managed_startup.go -- pure path refusal; register.go -- verifier
package vpp

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config"
)

// The loaded native schema rejects relative paths and native token separators
// before component delivery. Optional, future, Unicode and long paths remain
// valid syntax; filesystem availability belongs to managed preflight.
func TestManagedStartupCLINativeSchemaRefusesTokenInjection(t *testing.T) {
	validator, err := config.YANGValidator()
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{
		"relative.cli", "/tmp/a b", "/tmp/a\tmore", "/tmp/a\rmore", "/tmp/a\nmore",
		"/tmp/{other}", "/tmp/a#comment", "/tmp/a;other", "/tmp/\"a\"", "/tmp/'a'", "/tmp/a\\b",
	} {
		if err := validator.Validate("vpp/startup-cli-file", path); err == nil {
			t.Errorf("native schema admitted invalid startup token %q", path)
		}
	}
	for _, path := range []string{
		"", "/", "/future/bootstrap.cli", "/tmp/é/测试.cli", "/tmp/../bootstrap.cli",
		"/future/" + strings.Repeat("a", 8192),
	} {
		if err := validator.Validate("vpp/startup-cli-file", path); err != nil {
			t.Errorf("native schema invented a restriction on syntactically valid path %q: %v", path, err)
		}
	}
}

// The registered offline verifier checks the remaining Unicode token rules
// before apply, even for disabled or external VPP. It must not open a future
// command file or invent a filesystem path length during candidate checking.
func TestManagedStartupCLIRegisteredVerifierRefusesUnicodeTokens(t *testing.T) {
	for _, mode := range []map[string]any{
		{"enabled": "false"},
		{"enabled": "true", "external": "true"},
	} {
		for _, path := range []string{"/tmp/a\u00a0b", "/tmp/a\u200bb", "/tmp/a\u2028b", "/tmp/a\x00b"} {
			mode["startup-cli-file"] = path
			errs := config.VerifyPluginConfigMap(map[string]any{"vpp": mode})
			if len(errs) == 0 {
				t.Errorf("registered verifier admitted invalid startup token %q", path)
			}
		}
		for _, path := range []string{"", "/future/bootstrap.cli", "/tmp/é/测试.cli", "/future/" + strings.Repeat("a", 8192)} {
			mode["startup-cli-file"] = path
			if errs := config.VerifyPluginConfigMap(map[string]any{"vpp": mode}); len(errs) != 0 {
				t.Errorf("registered verifier inspected or restricted future path %q: %v", path, errs)
			}
		}
	}
}
