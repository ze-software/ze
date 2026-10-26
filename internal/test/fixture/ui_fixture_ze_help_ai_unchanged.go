package fixture

// Related: register_ze_help_ai_unchanged.go -- the registration.
// Related: ui_fixture_le_help_ai.go -- le's own help ai, which shares the reference type with ze's.

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// zeHelpAiGoldenDir holds ze's help ai output captured before le gained its
// own help ai, relative to the repository root. The goldens are per platform:
// ze links linux-only plugins (iface-dhcp, iface-ra, memlock; `platformOnly`
// in internal/component/plugin/all/all_test.go), so one feature set prints a
// different plugin list on Linux than on darwin.
const zeHelpAiGoldenDir = "internal/test/fixture/testdata"

// zeHelpAiAll is the ze help ai section that prints every page, and the
// name of its golden.
const zeHelpAiAll = "all"

// zeHelpAiForm is one output form of ze help ai.
type zeHelpAiForm struct {
	name string   // the golden is ze-help-ai-<name>-<GOOS>.golden
	args []string // after "help ai"
}

// zeHelpAiForms are the three forms AC-41 pins, each compared byte for byte:
// the reference orders its RPCs, notifications and services itself, so two
// runs of one binary print the same bytes.
var zeHelpAiForms = []zeHelpAiForm{
	{name: "summary"},
	{name: zeHelpAiAll, args: []string{zeHelpAiAll}},
	{name: renderJSON, args: []string{"--json"}},
}

// zeHelpAiUnchanged runs the ze binary this run built, under the full feature
// set the functional runner builds it with, and compares `ze help ai`,
// `ze help ai all` and `ze help ai --json` with their goldens (AC-41). The pin
// lives here rather than in a cmd/ze unit test because the output depends on
// which features the binary links, and only this layer builds ze with all of
// them.
func zeHelpAiUnchanged(ctx context.Context) error {
	root := os.Getenv(envRepoRoot)
	if root == "" {
		return errors.New(envRepoRoot + " is not set")
	}
	zePath, err := uiZEBinary()
	if err != nil {
		return err
	}
	for _, form := range zeHelpAiForms {
		if err := zeHelpAiCompare(ctx, root, zePath, form); err != nil {
			return err
		}
	}
	// The .ci reads this line; an exit code of 0 alone would also pass a
	// driver that returned before its checks ran.
	fmt.Println("OK")
	return nil
}

// zeHelpAiCompare runs one form and refuses any difference from its golden
// for the platform this run is on. A platform with no golden is a refusal,
// never a pass: a pin that skipped where it had nothing to compare would read
// green on a host it never judged.
func zeHelpAiCompare(ctx context.Context, root, zePath string, form zeHelpAiForm) error {
	goldenName := "ze-help-ai-" + form.name + "-" + runtime.GOOS + ".golden"
	goldenPath := filepath.Join(root, zeHelpAiGoldenDir, goldenName)
	argv := append([]string{leHelpPagesHelpWord, "ai"}, form.args...)
	want, err := os.ReadFile(goldenPath) //nolint:gosec // the path is the fixture's own golden under the repository root
	if errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("no %s golden for %s: %s does not exist; capture it on a %s host from the ze this run built, "+
			"with `%s %s > %s`, then have an independent ze-test-judge agent judge the new golden before it is committed",
			form.name, runtime.GOOS, goldenPath, runtime.GOOS, zePath, strings.Join(argv, " "), goldenPath)
	}
	if err != nil {
		return fmt.Errorf("read the %s golden %s: %w", form.name, goldenPath, err)
	}
	cmd := exec.CommandContext(ctx, zePath, argv...) //nolint:gosec // the fixture chooses the program and its arguments
	cmd.Dir = root
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ze help ai %s: %w, stderr %q", strings.Join(form.args, " "), err, stderr.String())
	}
	if !bytes.Equal(stdout.Bytes(), want) {
		line, got, golden := zeHelpAiFirstDifference(stdout.String(), string(want))
		return fmt.Errorf("ze help ai %s differs from %s byte for byte, first at line %d: now %q, golden %q",
			strings.Join(form.args, " "), goldenPath, line, got, golden)
	}
	return nil
}

// zeHelpAiFirstDifference answers the first line, counted from 1, where got
// and want differ, and both lines there. A text that ends first answers "" for
// its line, so a missing or extra trailing line is named too.
func zeHelpAiFirstDifference(got, want string) (int, string, string) {
	gotLines, wantLines := strings.Split(got, "\n"), strings.Split(want, "\n")
	for index := range max(len(gotLines), len(wantLines)) {
		var gotLine, wantLine string
		if index < len(gotLines) {
			gotLine = gotLines[index]
		}
		if index < len(wantLines) {
			wantLine = wantLines[index]
		}
		if gotLine != wantLine {
			return index + 1, gotLine, wantLine
		}
	}
	return 0, "", ""
}
