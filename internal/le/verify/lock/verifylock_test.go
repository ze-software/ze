// VALIDATES: the verify-lock command keeps the action-before-identifier grammar,
// delegates contention to lejob, and returns a child's exit status unchanged.
// PREVENTS: a second verify-class lock or a wrapper that flattens every failure to 1.
package verifylock

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/env"
)

func TestParseRequiresClosedKeywordsBeforeLabelAndCommand(t *testing.T) {
	label, argv, err := parse([]string{"run", "label", "full-verify", "command", "./le", "verify", "current", "mode", "full"})
	if err != nil {
		t.Fatal(err)
	}
	if label != "full-verify" || strings.Join(argv, " ") != "./le verify current mode full" {
		t.Fatalf("parse = label %q argv %q", label, argv)
	}
	for _, args := range [][]string{
		{"full-verify", "go"},
		{"run", "full-verify", "command", "go"},
		{"run", "label", "full-verify", "go"},
		{"run", "label", "full-verify", "command"},
	} {
		if _, _, err := parse(args); err == nil {
			t.Errorf("parse(%q) accepted an open positional", args)
		}
	}
}

func TestConcurrentRunSharesOneAdmittedChildAndItsExit(t *testing.T) {
	if os.Getenv("ZE_VERIFY_LOCK_HELPER") == "1" {
		verifyLockHelperProcess()
		return
	}
	t.Setenv("ZE_RUN_SLOTS", "1")
	t.Setenv("MAY_ATTACH", "1")
	// This test calls Run in-process, so it admits its own job. Without this
	// line, a test binary invoked through a wrapping job (`./le job run`,
	// `./le test-unit`) inherits ZE_RUN_JOB from that wrapper. insideParent
	// then reports both concurrent calls as nested in the same parent slot,
	// and neither reaches the claim/attach path this test exercises. See the
	// same fix in internal/le/job/contention_test.go's detach helper.
	t.Setenv("ZE_RUN_JOB", "")
	env.ResetCache()
	t.Cleanup(env.ResetCache)
	root := t.TempDir()
	marker := filepath.Join(root, "child-runs")
	started := filepath.Join(root, "child-started")
	argv := []string{os.Args[0], "-test.run=^TestConcurrentRunSharesOneAdmittedChildAndItsExit$", "--", marker, started, "37"}
	t.Setenv("ZE_VERIFY_LOCK_HELPER", "1")

	type outcome struct {
		admission string
		code      int
		err       error
	}
	results := make(chan outcome, 2)
	run := func() {
		var out bytes.Buffer
		report, code, err := Run(root, "full-verify", argv, &out, &out)
		results <- outcome{admission: report.Admission, code: code, err: err}
	}
	go run()
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := os.Stat(started); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("admitted child did not start")
		}
		time.Sleep(10 * time.Millisecond)
	}
	go run()

	first := <-results
	second := <-results
	for _, result := range []outcome{first, second} {
		if result.err != nil || result.code != 37 {
			t.Fatalf("run = admission %q code %d err %v", result.admission, result.code, result.err)
		}
	}
	admissions := first.admission + " " + second.admission
	if !strings.Contains(admissions, "claimed") || !strings.Contains(admissions, "attached") {
		t.Fatalf("admissions = %q, want one claimed and one attached", admissions)
	}
	content, err := os.ReadFile(marker)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "run\n" {
		t.Fatalf("child executions = %q, want one", content)
	}
}

func verifyLockHelperProcess() {
	args := os.Args
	if len(args) < 4 {
		os.Exit(98)
	}
	marker := args[len(args)-3]
	started := args[len(args)-2]
	code, err := strconv.Atoi(args[len(args)-1])
	if err != nil {
		os.Exit(97)
	}
	if err := os.WriteFile(started, []byte("started\n"), 0o600); err != nil {
		os.Exit(96)
	}
	time.Sleep(200 * time.Millisecond)
	file, err := os.OpenFile(marker, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		os.Exit(95)
	}
	if _, err := file.WriteString("run\n"); err != nil {
		_ = file.Close()
		os.Exit(94)
	}
	if err := file.Close(); err != nil {
		os.Exit(93)
	}
	os.Exit(code)
}
