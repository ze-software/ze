// Design: docs/architecture/testing/ci-format.md -- selected-DUT metadata admission

package runner

import (
	"errors"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// TestBuildTagMetadataRefusesUnknown checks malformed embedded settings without
// a compiler per case. Unknown metadata must never establish feature absence.
func TestBuildTagMetadataRefusesUnknown(t *testing.T) {
	for _, question := range []struct {
		name  string
		info  *debug.BuildInfo
		cause string
	}{
		{"nil", nil, "no Go build information"},
		{"missing", &debug.BuildInfo{}, "no explicit Go -tags"},
		{"empty", buildTagInfo(""), "empty Go -tags"},
		{"duplicate-setting", &debug.BuildInfo{Settings: []debug.BuildSetting{{Key: "-tags", Value: "ze_l2tp"}, {Key: "-tags", Value: "ze_vpp"}}}, "duplicate Go -tags"},
		{"duplicate-tag", buildTagInfo("ze_l2tp,ze_l2tp"), "repeats Go build tag"},
		{"empty-member", buildTagInfo("ze_l2tp,,ze_vpp"), "malformed Go build tag"},
		{"space", buildTagInfo("ze_l2tp ze_vpp"), "malformed Go build tag"},
		{"expression", buildTagInfo("ze_l2tp,!ze_vpp"), "malformed Go build tag"},
	} {
		t.Run(question.name, func(t *testing.T) {
			_, parsed := discoverBuildTagParseCI(t, "option=needs-build-tag:value=ze_vpp:state=absent\n")
			if parsed.ParseError != nil {
				t.Fatal(parsed.ParseError)
			}
			dut := newDUTBuildTags("selected-artifact", func(string) (*debug.BuildInfo, error) { return question.info, nil })
			reason, err := dut.admit(parsed.buildTags)
			if err == nil {
				t.Fatal("unknown metadata was admitted")
			}
			if reason != "" {
				t.Fatalf("unknown metadata became a skip: %s", reason)
			}
			for _, needle := range []string{"DUT build tags", "selected-artifact", question.cause} {
				if !strings.Contains(err.Error(), needle) {
					t.Fatalf("error %v does not name %q", err, needle)
				}
			}
		})
	}
}

// buildTagInfo models the actual Go setting, not a manifest-derived artifact.
// MAIN's full/no-VPP CLI runs separately prove the real ReadFile boundary.
func buildTagInfo(tags string) *debug.BuildInfo {
	return &debug.BuildInfo{Settings: []debug.BuildSetting{{Key: "-tags", Value: tags}}}
}

// TestBuildTagMetadataMatchesPredicates includes ordinary personality/custom
// tags and no ze_core requirement. Both present/absent mismatches must skip.
func TestBuildTagMetadataMatchesPredicates(t *testing.T) {
	for _, question := range []struct {
		name     string
		options  string
		tags     string
		mismatch string
	}{
		{"present", "option=needs-build-tag:value=ze_l2tp:state=present\n", "ze_l2tp,zetest,ze_setup,custom.1", ""},
		{"absent", "option=needs-build-tag:value=ze_vpp:state=absent\n", "ze_l2tp,ze_core,ze_distro", ""},
		{"missing-present", "option=needs-build-tag:value=ze_l2tp:state=present\n", "custom", "value=ze_l2tp:state=present"},
		{"present-absent", "option=needs-build-tag:value=ze_vpp:state=absent\n", "ze_l2tp,ze_vpp", "value=ze_vpp:state=absent"},
		{"conjunction", "option=needs-build-tag:value=ze_l2tp:state=present\noption=needs-build-tag:value=ze_vpp:state=absent\n", "ze_l2tp,zetest", ""},
		{"conjunction-second-mismatch", "option=needs-build-tag:value=ze_l2tp:state=present\noption=needs-build-tag:value=ze_vpp:state=absent\n", "ze_l2tp,ze_vpp", "value=ze_vpp:state=absent"},
	} {
		t.Run(question.name, func(t *testing.T) {
			_, parsed := discoverBuildTagParseCI(t, question.options)
			if parsed.ParseError != nil {
				t.Fatal(parsed.ParseError)
			}
			dut := newDUTBuildTags("selected-artifact", func(string) (*debug.BuildInfo, error) { return buildTagInfo(question.tags), nil })
			reason, err := dut.admit(parsed.buildTags)
			if err != nil {
				t.Fatal(err)
			}
			if question.mismatch == "" {
				if reason != "" {
					t.Fatalf("matching artifact skipped: %s", reason)
				}
				return
			}
			for _, needle := range []string{"selected-artifact", question.mismatch} {
				if !strings.Contains(reason, needle) {
					t.Fatalf("skip %q does not identify %q", reason, needle)
				}
			}
		})
	}
}

// TestBuildTagMetadataSnapshotKeepsReadFailure verifies one read per run, zero
// reads for ordinary cases, and retained uncertainty for every declaring case.
func TestBuildTagMetadataSnapshotKeepsReadFailure(t *testing.T) {
	_, parsed := discoverBuildTagParseCI(t, "option=needs-build-tag:value=ze_vpp:state=absent\n")
	if parsed.ParseError != nil {
		t.Fatal(parsed.ParseError)
	}
	var reads int
	cause := errors.New("cannot read artifact")
	dut := newDUTBuildTags("selected-artifact", func(string) (*debug.BuildInfo, error) { reads++; return nil, cause })
	if reason, err := dut.admit(nil); reason != "" || err != nil {
		t.Fatalf("ordinary case changed: %s %v", reason, err)
	}
	if reads != 0 {
		t.Fatalf("ordinary case read metadata %d times", reads)
	}
	for range 2 {
		reason, err := dut.admit(parsed.buildTags)
		if !errors.Is(err, cause) {
			t.Fatalf("read failure lost its cause: %v", err)
		}
		if reason != "" {
			t.Fatalf("read failure became a skip: %s", reason)
		}
	}
	if reads != 1 {
		t.Fatalf("one selected DUT read %d times", reads)
	}
}

// TestBuildTagBothRunnersUseSelectedDUT drives both real Run entry points with
// a modeled metadata reader and a synthetic command witness. These prove the
// admission wiring; real Go artifact/command proof remains a separate CLI run.
func TestBuildTagBothRunnersUseSelectedDUT(t *testing.T) {
	for _, dialect := range []string{"generic", "parse"} {
		for _, question := range []struct {
			name     string
			options  string
			tags     string
			wantSkip bool
		}{
			{"match", "option=needs-build-tag:value=ze_l2tp:state=present\noption=needs-build-tag:value=ze_vpp:state=absent\n", "ze_l2tp,zetest,custom", false},
			{"mismatch", "option=needs-build-tag:value=ze_vpp:state=absent\n", "ze_vpp,zetest", true},
			{"ordinary", "", "", false},
		} {
			t.Run(dialect+"/"+question.name, func(t *testing.T) {
				tests, parsed := discoverBuildTagParseCI(t, question.options)
				if parsed.ParseError != nil {
					t.Fatal(parsed.ParseError)
				}
				binary := filepath.Join(tests.baseDir, "synthetic-selected-dut")
				if err := os.WriteFile(binary, []byte("#!/bin/sh\nprintf 'configuration valid\\n'\n"), 0o700); err != nil {
					t.Fatal(err)
				}
				var reads atomic.Int32
				read := func(path string) (*debug.BuildInfo, error) {
					reads.Add(1)
					if path != binary {
						return nil, errors.New("runner inspected a different DUT: " + path)
					}
					return buildTagInfo(question.tags), nil
				}
				var skip string
				if dialect == "parse" {
					parsed.SetActive(true)
					runner := NewParsingRunner(tests, tests.baseDir, binary)
					runner.readDUTBuildInfo = read
					if !runner.Run(t.Context(), false, true) {
						t.Fatalf("parse Run: %v", parsed.Error)
					}
					skip = parsed.SkipReason
					if !question.wantSkip {
						if !strings.Contains(parsed.Output, "configuration valid") {
							t.Fatal("admitted parse command did not produce its witness")
						}
					}
				} else {
					generic := NewEncodingTests(tests.baseDir)
					if err := generic.Discover(filepath.Dir(parsed.File)); err != nil {
						t.Fatal(err)
					}
					record := generic.Registered()[0]
					if record.ParseFailed {
						t.Fatal(record.Error)
					}
					record.Activate()
					runner, err := NewRunner(generic, tests.baseDir)
					if err != nil {
						t.Fatal(err)
					}
					t.Cleanup(runner.Cleanup)
					runner.zePath = binary
					runner.readDUTBuildInfo = read
					if !runner.Run(t.Context(), &RunOptions{Quiet: true, SkipTimings: true, Timeout: time.Second}) {
						t.Fatalf("generic Run: %v", record.Error)
					}
					skip = record.SkipReason
					if !question.wantSkip {
						if !strings.Contains(record.ClientStdout, "configuration valid") {
							t.Fatal("admitted generic command did not produce its witness")
						}
					}
				}
				if question.wantSkip {
					for _, needle := range []string{binary, "value=ze_vpp:state=absent"} {
						if !strings.Contains(skip, needle) {
							t.Fatalf("skip %q does not name %q", skip, needle)
						}
					}
				} else if skip != "" {
					t.Fatalf("permitted case skipped: %s", skip)
				}
				wantReads := int32(1)
				if question.options == "" {
					wantReads = 0
				}
				if reads.Load() != wantReads {
					t.Fatalf("metadata reads %d, want %d", reads.Load(), wantReads)
				}
			})
		}
	}
}

// TestBuildTagChangedDUTRechecksPreviousMismatch runs the same selected case
// twice. A previous build skip is a result, not a permanent host requirement.
func TestBuildTagChangedDUTRechecksPreviousMismatch(t *testing.T) {
	for _, dialect := range []string{"generic", "parse"} {
		t.Run(dialect, func(t *testing.T) {
			tests, parsed := discoverBuildTagParseCI(t, "option=needs-build-tag:value=ze_vpp:state=absent\n")
			if parsed.ParseError != nil {
				t.Fatal(parsed.ParseError)
			}
			first := filepath.Join(tests.baseDir, "first-selected-dut")
			second := filepath.Join(tests.baseDir, "second-selected-dut")
			if err := os.WriteFile(second, []byte("#!/bin/sh\nprintf 'configuration valid\\n'\n"), 0o700); err != nil {
				t.Fatal(err)
			}
			var reads atomic.Int32
			read := func(path string) (*debug.BuildInfo, error) {
				reads.Add(1)
				if path == first {
					return buildTagInfo("ze_vpp"), nil
				}
				if path == second {
					return buildTagInfo("custom"), nil
				}
				return nil, errors.New("unselected artifact: " + path)
			}
			if dialect == "parse" {
				parsed.SetActive(true)
				runner := NewParsingRunner(tests, tests.baseDir, first)
				runner.readDUTBuildInfo = read
				if !runner.Run(t.Context(), false, true) {
					t.Fatal(parsed.Error)
				}
				if parsed.SkipReason == "" {
					t.Fatal("first artifact did not establish the mismatch control")
				}
				runner.zePath = second
				if !runner.Run(t.Context(), false, true) {
					t.Fatal(parsed.Error)
				}
				if parsed.SkipReason != "" {
					t.Fatalf("new artifact inherited the old build skip: %s", parsed.SkipReason)
				}
				if !strings.Contains(parsed.Output, "configuration valid") {
					t.Fatal("second artifact's command did not run")
				}
			} else {
				generic := NewEncodingTests(tests.baseDir)
				if err := generic.Discover(filepath.Dir(parsed.File)); err != nil {
					t.Fatal(err)
				}
				record := generic.Registered()[0]
				if record.ParseFailed {
					t.Fatal(record.Error)
				}
				record.Activate()
				runner, err := NewRunner(generic, tests.baseDir)
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(runner.Cleanup)
				runner.readDUTBuildInfo = read
				opts := &RunOptions{Quiet: true, SkipTimings: true, Timeout: time.Second}
				runner.zePath = first
				if !runner.Run(t.Context(), opts) {
					t.Fatal(record.Error)
				}
				if record.State != StateSkip {
					t.Fatal("first artifact did not establish the mismatch control")
				}
				runner.zePath = second
				if !runner.Run(t.Context(), opts) {
					t.Fatal(record.Error)
				}
				if record.State != StateSuccess {
					t.Fatalf("new artifact inherited the old verdict: %v", record.State)
				}
				if record.SkipReason != "" {
					t.Fatalf("new artifact retained the old skip: %s", record.SkipReason)
				}
			}
			if reads.Load() != 2 {
				t.Fatalf("two selected artifacts caused %d reads", reads.Load())
			}
		})
	}
}
