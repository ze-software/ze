// Smoke + selftest for scripts/checks/config_string_coercion.go (the //go:build
// ignore config value-coercion guard). The checker is ignore-tagged so it is
// excluded from normal compilation; this test gives the package a buildable
// target and runs the checker as a subprocess, asserting the current tree
// passes -- no config.go coerces a delivered config value with a native-type
// assertion (v.(bool)/v.(float64)) or a numeric/bool type switch lacking a
// `case string:` arm. Regression guard for the ddos-detect-disabled bug
// (session 6503): the framework delivers YANG leaf values as JSON strings.

package main

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"
)

// TestNoNativeTypeConfigCoercion runs the guard against the live tree and
// asserts every config parser handles the string form the framework delivers.
func TestNoNativeTypeConfigCoercion(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), checkTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "run", "scripts/checks/config_string_coercion.go")
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	cmd.Dir = repoRoot(t)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("config-string-coercion gate failed (a config parser coerces a delivered string config value with a native-type assertion or a type switch missing `case string:`):\n%s", out)
	}
	if !strings.Contains(string(out), "config-string-coercion: OK") {
		t.Fatalf("config_string_coercion.go did not report OK:\n%s", out)
	}
}

// TestConfigStringCoercionSelftest proves the AST detection actually fires --
// isolated temp-dir fixtures (a numeric type switch without a string case and a
// direct v.(bool) are flagged; the string-aware equivalents are not) -- so a
// regression in the detector is caught even while the live tree stays clean.
func TestConfigStringCoercionSelftest(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), checkTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "run", "scripts/checks/config_string_coercion.go", "--selftest")
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	cmd.Dir = repoRoot(t)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("config-string-coercion --selftest failed:\n%s", out)
	}
	if !strings.Contains(string(out), "config-string-coercion selftest OK") {
		t.Fatalf("config_string_coercion.go --selftest did not report OK:\n%s", out)
	}
}
