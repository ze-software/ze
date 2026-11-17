// Design: docs/architecture/testing/ci-format.md -- strict actual-DUT conditions

package runner

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// TestBuildTagGrammarRefusesMalformedConditions uses both discoverers, so a
// malformed condition cannot become an unsupported-option skip in one dialect.
func TestBuildTagGrammarRefusesMalformedConditions(t *testing.T) {
	for _, question := range []struct {
		name    string
		options string
		cause   string
	}{
		{"missing-value", "option=needs-build-tag:state=present\n", "value must name a feature build tag"},
		{"empty-value", "option=needs-build-tag:value=:state=present\n", "value must name a feature build tag"},
		{"unknown-name", "option=needs-build-tag:value=ze_vp:state=absent\n", "unknown feature build tag"},
		{"missing-state", "option=needs-build-tag:value=ze_vpp\n", "state must be present or absent"},
		{"empty-state", "option=needs-build-tag:value=ze_vpp:state=\n", "state must be present or absent"},
		{"wrong-state", "option=needs-build-tag:value=ze_vpp:state=missing\n", "state must be present or absent"},
		{"extra-key", "option=needs-build-tag:value=ze_vpp:state=absent:typo=yes\n", "typo"},
		{"duplicate-key", "option=needs-build-tag:value=ze_vpp:state=absent:state=present\n", "twice"},
		{"duplicate-condition", "option=needs-build-tag:value=ze_vpp:state=present\noption=needs-build-tag:value=ze_vpp:state=present\n", "already declares"},
		{"contradiction", "option=needs-build-tag:value=ze_vpp:state=present\noption=needs-build-tag:value=ze_vpp:state=absent\n", "already declares"},
	} {
		t.Run(question.name, func(t *testing.T) {
			tests, test := discoverBuildTagParseCI(t, question.options)
			if test.ParseError == nil {
				t.Fatal("parse dialect accepted malformed condition")
			}
			if !strings.Contains(test.ParseError.Error(), question.cause) {
				t.Fatalf("parse cause %v, want %q", test.ParseError, question.cause)
			}
			generic := NewEncodingTests(tests.baseDir)
			if err := generic.Discover(filepath.Dir(test.File)); err != nil {
				t.Fatal(err)
			}
			record := generic.Registered()[0]
			if !record.ParseFailed {
				t.Fatal("generic dialect accepted malformed condition")
			}
			if !strings.Contains(record.Error.Error(), question.cause) {
				t.Fatalf("generic cause %v, want %q", record.Error, question.cause)
			}
		})
	}
}

// TestBuildTagGenericUnknownArtifactFails reaches the generic Runner with a
// valid conditional question. Missing metadata must fail before any child runs.
func TestBuildTagGenericUnknownArtifactFails(t *testing.T) {
	tests, test := discoverBuildTagParseCI(t, "option=needs-build-tag:value=ze_vpp:state=absent\n")
	generic := NewEncodingTests(tests.baseDir)
	if err := generic.Discover(filepath.Dir(test.File)); err != nil {
		t.Fatal(err)
	}
	record := generic.Registered()[0]
	if record.ParseFailed {
		t.Fatalf("valid generic condition failed discovery: %v", record.Error)
	}
	record.Activate()
	runner, err := NewRunner(generic, tests.baseDir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(runner.Cleanup)
	runner.zePath = filepath.Join(tests.baseDir, "missing-selected-dut")
	if runner.Run(t.Context(), &RunOptions{Quiet: true, SkipTimings: true, Timeout: time.Second}) {
		t.Fatal("generic runner admitted unknown DUT metadata")
	}
	if record.Error == nil {
		t.Fatal("generic runner lost metadata error")
	}
	for _, needle := range []string{"DUT build tags", runner.zePath} {
		if !strings.Contains(record.Error.Error(), needle) {
			t.Fatalf("metadata error %v does not name %q", record.Error, needle)
		}
	}
	if record.State != StateFail {
		t.Fatalf("state %v, want failure", record.State)
	}
}

// TestBuildTagHostSkipPrecedesUnknownArtifact keeps an eligible declaration's
// unreadable artifact from replacing the existing host-skip reason.
func TestBuildTagHostSkipPrecedesUnknownArtifact(t *testing.T) {
	tests, test := discoverBuildTagParseCI(t, "option=skip-os:value="+runtime.GOOS+"\n"+
		"option=needs-build-tag:value=ze_vpp:state=absent\n")
	if test.ParseError != nil {
		t.Fatal(test.ParseError)
	}
	test.SetActive(true)
	before := test.SkipReason
	runner := NewParsingRunner(tests, tests.baseDir, filepath.Join(tests.baseDir, "missing-selected-dut"))
	if !runner.Run(t.Context(), false, true) {
		t.Fatalf("host-skip became metadata failure: %v", test.Error)
	}
	if test.SkipReason != before {
		t.Fatalf("host-skip cause changed from %q to %q", before, test.SkipReason)
	}
}

// TestBuildTagResourceOnlyConsumerRefuses uses the shared editor/web reader.
// Recognizing a DUT predicate there must fail explicitly, never discard it.
func TestBuildTagResourceOnlyConsumerRefuses(t *testing.T) {
	var record Record
	handled, err := ParseResourceOption(&record, "needs-build-tag:value=ze_vpp:state=absent")
	if !handled {
		t.Fatal("resource-only consumer silently ignored a DUT build condition")
	}
	if err == nil {
		t.Fatal("resource-only consumer accepted an unsupported DUT build condition")
	}
	if !strings.Contains(err.Error(), "selected DUT") {
		t.Fatalf("refusal did not identify the missing artifact contract: %v", err)
	}
}
