// Design: docs/architecture/testing/ci-format.md -- declaration vocabulary ancestry

package runner

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestBuildTagDeclarationUsesCICheckoutManifest declares a synthetic feature
// absent from the host checkout, then changes only the CI ancestor manifest.
func TestBuildTagDeclarationUsesCICheckoutManifest(t *testing.T) {
	initial, parsed := discoverBuildTagParseCI(t, "option=needs-build-tag:value=ze_fixture_only:state=present\n")
	if parsed.ParseError == nil {
		t.Fatal("unknown-feature negative control was accepted")
	}
	if !strings.Contains(parsed.ParseError.Error(), "unknown feature build tag") {
		t.Fatalf("negative control failed before feature validation: %v", parsed.ParseError)
	}
	manifest := filepath.Join(initial.baseDir, featureGatesFile)
	if err := os.WriteFile(manifest, []byte("ze_fixture_only internal/component/fixture\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	tests := NewParsingTests(initial.baseDir)
	if err := tests.Discover(filepath.Dir(parsed.File)); err != nil {
		t.Fatal(err)
	}
	if test := tests.Registered()[0]; test.ParseError != nil {
		t.Fatalf("parse reader ignored the CI ancestor manifest: %v", test.ParseError)
	}
	generic := NewEncodingTests(initial.baseDir)
	if err := generic.Discover(filepath.Dir(parsed.File)); err != nil {
		t.Fatal(err)
	}
	if record := generic.Registered()[0]; record.ParseFailed {
		t.Fatalf("generic reader ignored the CI ancestor manifest: %v", record.Error)
	}
}

// TestBuildTagWholeOptionBoundary retains the declaration's complete name.
// A suffix must not be read as a present/absent field of needs-build-tag.
func TestBuildTagWholeOptionBoundary(t *testing.T) {
	for _, suffix := range []string{"ged", "=ze_vpp", "-extra"} {
		_, parsed := discoverBuildTagParseCI(t, "option=needs-build-tag"+suffix+":value=ze_vpp:state=absent\n")
		if parsed.ParseError == nil {
			t.Fatalf("accepted option name suffix %q", suffix)
		}
		if !strings.Contains(parsed.ParseError.Error(), "unknown directive") {
			t.Fatalf("suffix was interpreted as a known option: %v", parsed.ParseError)
		}
	}
}

// TestBuildTagDeclarationRefusesMalformedManifestTag checks the name itself,
// because the shared manifest parser does not validate its tag column.
func TestBuildTagDeclarationRefusesMalformedManifestTag(t *testing.T) {
	initial, parsed := discoverBuildTagParseCI(t, "option=needs-build-tag:value=ze_bad!:state=present\n")
	manifest := filepath.Join(initial.baseDir, featureGatesFile)
	if err := os.WriteFile(manifest, []byte("ze_bad! internal/component/fixture\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	tests := NewParsingTests(initial.baseDir)
	if err := tests.Discover(filepath.Dir(parsed.File)); err != nil {
		t.Fatal(err)
	}
	got := tests.Registered()[0]
	if got.ParseError == nil {
		t.Fatal("accepted malformed tag merely because the manifest listed it")
	}
	if !strings.Contains(got.ParseError.Error(), "invalid feature build tag") {
		t.Fatalf("failure did not identify the malformed tag: %v", got.ParseError)
	}
}
