// Design: docs/functional-tests.md -- verify failure routing protocol
// Related: scripts/dev/verify-status.sh -- freshness status consumer
package main

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/core/hostload"
	"codeberg.org/thomas-mangin/ze/internal/core/textbuf"
)

const (
	combinedLogPath  = "tmp/ze-verify.log"
	failuresLogPath  = "tmp/ze-verify-failures.log"
	failuresJSONPath = "tmp/ze-verify-failures.json"
	statusPath       = "tmp/ze-verify.status"
	stageLogDir      = "tmp/verify"

	maxInlineMembers = 8
	maxExcerptLines  = 20
)

type stage struct {
	Name    string
	Command []string
	Rerun   string
	Env     []string
}

type failureGroup struct {
	Stage     string   `json:"stage"`
	GroupID   string   `json:"group-id"`
	Kind      string   `json:"kind"`
	Related   []string `json:"related"`
	Summary   string   `json:"summary"`
	Rerun     string   `json:"rerun"`
	DetailLog string   `json:"detail-log"`
	Parallel  string   `json:"parallel"`
	Excerpt   []string `json:"excerpt,omitempty"`
}

type stageResult struct {
	Stage     string         `json:"stage"`
	ExitCode  int            `json:"exit_code"`
	DetailLog string         `json:"detail-log"`
	Groups    []failureGroup `json:"groups,omitempty"`
}

type verifyIndex struct {
	Mode        string        `json:"mode"`
	ExitCode    int           `json:"exit_code"`
	CombinedLog string        `json:"combined_log"`
	GeneratedAt string        `json:"generated_at"`
	Stages      []stageResult `json:"stages"`
}

type verifyConfig struct {
	Root        string
	Mode        string
	Stages      []stage
	Now         func() time.Time
	Out         io.Writer
	RunStage    func(context.Context, string, stage, io.Writer) int
	WriteStatus func(root string, exitCode int, mode, skipped string, now time.Time) error
}

// makeNoExecFlags are the GNU make short options under which recipes are not
// really executed, so every stage would "succeed" without doing anything:
//
//	n  -n / --dry-run / --just-print / --recon -- prints recipes, runs nothing
//	t  -t / --touch                            -- touches targets; a .PHONY
//	                                              target reports "Nothing to be
//	                                              done" and exits 0
//	q  -q / --question                         -- stage sub-makes no-op and
//	                                              exit 1 (the top-level make
//	                                              still runs a $(MAKE) line)
//
// All three share the property that matters here: make still EXECUTES recipe
// lines containing $(MAKE) (that is how recursive make participates in these
// modes), so this runner really starts, while the stage sub-makes it invokes do
// nothing. -n and -t then report success; -q reports failure, which is not a
// forgery but would still overwrite a good verify record with a false red.
const makeNoExecFlags = "ntq"

// makeDryRun reports whether the invoking make was run in one of those modes,
// by inspecting MAKEFLAGS.
//
// GNU make puts the concatenated single-letter flags in the FIRST whitespace
// -separated field, with no leading dash ("n", "rRn", "Bn",
// "n -j4 --jobserver-auth=…", "n -- FOO=bar"). If that field starts with a dash
// there are no short flags at all -- that is the case for "-j8 …" and for
// long-only options such as "--no-print-directory", both of which must NOT
// match. Only that first field is examined, so a target name or a variable
// value containing an "n"/"t"/"q" cannot cause a false positive.
//
// Verified against real GNU make 4.3 output rather than assumed; the observed
// strings are pinned in TestMakeDryRunDetectsDashN.
func makeDryRun(makeflags string) bool {
	fields := strings.Fields(makeflags)
	if len(fields) == 0 || strings.HasPrefix(fields[0], "-") {
		return false
	}
	// A short-flags word never contains '='. GNU make 3.81 (the macOS system
	// make) writes a command-line variable override as the FIRST word with no
	// `--` separator (`make ze-verify ZE_VERIFY_LOG=tmp/x.log` ->
	// MAKEFLAGS="ZE_VERIFY_LOG=tmp/x.log"); reading that as flags would refuse
	// the run on the 't' in "tmp". Newer makes separate overrides with `--`,
	// already handled by the leading-dash check above.
	if strings.ContainsRune(fields[0], '=') {
		return false
	}
	return strings.ContainsAny(fields[0], makeNoExecFlags)
}

// modeFullVerify is the default mode name, shared by main, --list and the
// Makefile targets.
const modeFullVerify = "ze-verify"

func main() {
	mode := modeFullVerify
	if len(os.Args) > 1 {
		mode = os.Args[1]
	}

	// `--list` prints the stage list and runs nothing. It exists so that
	// "what does ze-verify run?" has a SAFE answer -- see the dry-run refusal
	// below for why `make -n ze-verify` is not one.
	if mode == "--list" {
		listMode := modeFullVerify
		if len(os.Args) > 2 {
			listMode = os.Args[2]
		}
		stages := stagesForMode(listMode, "make")
		if len(stages) == 0 {
			fmt.Fprintf(os.Stderr, "verify runner: unknown mode %q (want ze-verify or ze-verify-changed)\n", listMode)
			os.Exit(2)
		}
		for _, st := range stages {
			fmt.Println(st.Name) //nolint:errcheck // stdout
		}
		return
	}

	// REFUSE to run under make's no-execute modes (-n, -t, -q). This is a
	// fail-closed guard on the commit gate, not a convenience.
	//
	// `ze-verify`'s recipe contains $(MAKE), and GNU make executes such lines
	// even in those modes (it is how recursive make participates in them). So
	// `make -n ze-verify` really starts this program, with the flag propagated
	// through MAKEFLAGS to every stage sub-make -- and each stage then does
	// nothing. Under -n it echoes its recipe and exits 0; under -t a .PHONY
	// stage prints "Nothing to be done" and exits 0, which is quieter still.
	// Without this guard the run would collect a full sweep of green stages, write an
	// all-green tmp/ze-verify-failures.json, and write tmp/ze-verify.status with
	// exit=0 and the CURRENT tree hash -- after which
	// scripts/dev/verify-status.sh reports FRESH and commit_helper.py sees no
	// structural-gate reds. One `make -n` or `make -t` would certify a
	// completely unverified tree as fully verified. Refuse loudly instead.
	if makeDryRun(os.Getenv("MAKEFLAGS")) {
		fmt.Fprintln(os.Stderr, "verify runner: refusing to run under `make -n` / -t / -q (no-execute modes).")
		fmt.Fprintln(os.Stderr, "  This recipe contains $(MAKE), so make executes it even in those modes, while")
		fmt.Fprintln(os.Stderr, "  every stage sub-make does nothing. Under -n and -t the stages all report")
		fmt.Fprintln(os.Stderr, "  success, so writing tmp/ze-verify.status would forge a FRESH verify record")
		fmt.Fprintln(os.Stderr, "  for a completely unverified tree.")
		fmt.Fprintln(os.Stderr, "  To see the stage list without running anything: make ze-verify-list")
		os.Exit(2)
	}

	cfg := defaultVerifyConfig(mode, os.Stdout)
	code, err := runVerify(context.Background(), cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "verify runner failed: %v\n", err)
		if code == 0 {
			code = 1
		}
	}
	os.Exit(code)
}

func defaultVerifyConfig(mode string, out io.Writer) verifyConfig {
	makeCmd := os.Getenv("ZE_VERIFY_MAKE")
	if makeCmd == "" {
		makeCmd = "make"
	}
	return verifyConfig{
		Root:        ".",
		Mode:        mode,
		Stages:      stagesForMode(mode, makeCmd),
		Now:         time.Now,
		Out:         out,
		RunStage:    execStage,
		WriteStatus: writeVerifyStatus,
	}
}

// stagesForMode is the SINGLE SOURCE OF TRUTH for what `make ze-verify` and
// `make ze-verify-changed` run. Both Makefile targets shell out to this runner,
// and .github/workflows/verify.yml's only step is `make ze-verify` -- so a gate that
// is not listed here runs NOWHERE, in CI or locally.
//
// Add a new gate to BOTH branches. The two lists are hand-duplicated on
// purpose (the changed-mode variants differ), which is precisely how they drift;
// TestStagesForModeMatchesGolden pins each against a committed golden so a
// one-branch edit fails loudly.
//
// A pair of duplicate `_ze-verify-impl` / `_ze-verify-changed-impl` Makefile
// targets used to shadow this list. They had zero callers, drifted for an
// unknown period, and were deleted by plan/spec-fixit-verify-stage-ssot.md.
// Do not reintroduce a second copy.
func stagesForMode(mode, makeCmd string) []stage {
	mk := func(name string) stage {
		return stage{
			Name:    name,
			Command: []string{makeCmd, "--no-print-directory", name},
			Rerun:   "make " + shellQuote(name),
		}
	}
	switch mode {
	case "ze-verify-changed":
		return []stage{
			mk("ze-lint-changed"),
			mk("ze-tier-check"),
			mk("ze-rfc-check"),
			mk("ze-iface-resolution-check"),
			mk("ze-plugin-boundary-check"),
			mk("ze-config-coercion-check"),
			mk("ze-fs-persistence-check"),
			mk("ze-dash-stdio-check"),
			mk("ze-port-defaults-check"),
			mk("ze-test-sensitivity-check"),
			mk("ze-platform-vet"),
			mk("ze-verify-wiring-docs"),
			mk("ze-doc-test"),
			mk("ze-doc-links"),
			mk("ze-regen-check-readonly"),
			mk("ze-hook-test"),
			mk("ze-unit-test-changed"),
			mk("ze-functional-test"),
			mk("ze-exabgp-test"),
		}
	case modeFullVerify:
		return []stage{
			mk("ze-lint"),
			mk("ze-tier-check"),
			mk("ze-rfc-check"),
			mk("ze-iface-resolution-check"),
			mk("ze-plugin-boundary-check"),
			mk("ze-config-coercion-check"),
			mk("ze-fs-persistence-check"),
			mk("ze-dash-stdio-check"),
			mk("ze-port-defaults-check"),
			mk("ze-test-sensitivity-check"),
			mk("ze-platform-vet"),
			mk("ze-verify-wiring-docs"),
			mk("ze-doc-test"),
			mk("ze-doc-links"),
			mk("ze-regen-check-readonly"),
			mk("ze-vet-evidence"),
			mk("ze-hook-test"),
			mk("ze-unit-test-cached"),
			mk("ze-unit-test-race-changed"),
			mk("ze-alloc-gate"),
			mk("ze-functional-test"),
			mk("ze-exabgp-test"),
		}
	default:
		// FAIL CLOSED on an unknown mode. This used to be the `default` branch
		// returning the FULL list, so a typo (`ze-verify-chnaged`) silently ran
		// full verify and then wrote mode=ze-verify-chnaged into
		// tmp/ze-verify.status, which verify-status.sh rendered as
		// FRESH(ze-verify-chnaged) -- a verified-looking record for a mode
		// nobody asked for. runVerify turns the empty list into exit 2 with
		// "no verify stages configured".
		return nil
	}
}

func runVerify(ctx context.Context, cfg verifyConfig) (int, error) {
	if cfg.Root == "" {
		cfg.Root = "."
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	if cfg.Out == nil {
		cfg.Out = io.Discard
	}
	if cfg.RunStage == nil {
		cfg.RunStage = execStage
	}
	if cfg.WriteStatus == nil {
		cfg.WriteStatus = writeVerifyStatus
	}
	if len(cfg.Stages) == 0 {
		return 2, errors.New("no verify stages configured")
	}

	if err := os.MkdirAll(filepath.Join(cfg.Root, stageLogDir), 0o750); err != nil {
		return 2, fmt.Errorf("create stage log dir: %w", err)
	}
	if err := os.MkdirAll(filepath.Join(cfg.Root, "tmp"), 0o750); err != nil {
		return 2, fmt.Errorf("create tmp dir: %w", err)
	}

	combinedPath := filepath.Join(cfg.Root, combinedLogPath)
	combined, err := openControlledFile(combinedPath)
	if err != nil {
		return 2, fmt.Errorf("open combined log: %w", err)
	}
	defer combined.Close() //nolint:errcheck // best-effort close at process exit

	generatedAt := cfg.Now().UTC().Format(time.RFC3339)
	out := io.MultiWriter(cfg.Out, combined)
	writef(out, "Ze verify protocol run: %s\nMode: %s\nCombined log: %s\n\n", generatedAt, cfg.Mode, combinedLogPath)

	if warn := contendedWarning(); warn != "" {
		writef(out, "WARNING: %s\n\n", warn)
	}

	results := make([]stageResult, 0, len(cfg.Stages))
	exitCode := 0
	for i, st := range cfg.Stages {
		logRel := filepath.ToSlash(filepath.Join(stageLogDir, fmt.Sprintf("%02d-%s.log", i+1, safeStageLogName(st.Name))))
		logPath := filepath.Join(cfg.Root, filepath.FromSlash(logRel))
		stageLog, openErr := openControlledFile(logPath)
		if openErr != nil {
			return 2, fmt.Errorf("open stage log %s: %w", logRel, openErr)
		}

		writer := io.MultiWriter(cfg.Out, combined, stageLog)
		writef(writer, "\n### Stage %02d/%02d: %s\n", i+1, len(cfg.Stages), st.Name)
		writef(writer, "$ %s\n", strings.Join(quoteCommand(st.Command), " "))

		code := cfg.RunStage(ctx, cfg.Root, st, writer)
		writef(writer, "\n### Stage result: %s exit=%d\n", st.Name, code)
		stageLog.Close() //nolint:errcheck // subsequent read reports the real error if close failed

		res := stageResult{Stage: st.Name, ExitCode: code, DetailLog: logRel}
		if code != 0 {
			if exitCode == 0 {
				exitCode = 1
			}
			content, readErr := readControlledFile(logPath)
			if readErr != nil {
				res.Groups = []failureGroup{genericGroup(st, logRel, fmt.Sprintf("stage failed and log could not be read: %v", readErr), nil)}
			} else {
				res.Groups = classifyStage(st, logRel, string(content))
			}
		}
		results = append(results, res)
	}

	index := verifyIndex{
		Mode:        cfg.Mode,
		ExitCode:    exitCode,
		CombinedLog: combinedLogPath,
		GeneratedAt: generatedAt,
		Stages:      results,
	}
	if err := writeFailureArtifacts(cfg.Root, index); err != nil {
		return 2, err
	}
	if err := cfg.WriteStatus(cfg.Root, exitCode, cfg.Mode, os.Getenv("ZE_SKIP_SUITES"), cfg.Now()); err != nil {
		return 2, fmt.Errorf("write verify status: %w", err)
	}

	printFinalSummary(io.MultiWriter(cfg.Out, combined), index)
	return exitCode, nil
}

func execStage(ctx context.Context, root string, st stage, w io.Writer) int {
	if len(st.Command) == 0 {
		writeln(w, "stage has no command")
		return 2
	}
	cmd := exec.CommandContext(ctx, st.Command[0], st.Command[1:]...) //nolint:gosec // command list is fixed by repository code
	cmd.Dir = root
	cmd.Stdout = w
	cmd.Stderr = w
	cmd.Env = append(os.Environ(), "ZE_VERIFY_MODE=1")
	cmd.Env = append(cmd.Env, st.Env...)
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return exitErr.ExitCode()
		}
		writef(w, "stage command failed: %v\n", err)
		return 1
	}
	return 0
}

func writeFailureArtifacts(root string, index verifyIndex) error {
	var text strings.Builder
	writef(&text, "# Ze verify failure index\n")
	writef(&text, "Generated: %s\n", index.GeneratedAt)
	writef(&text, "Mode: %s\n", index.Mode)
	writef(&text, "Combined log: %s\n\n", index.CombinedLog)

	failedStages := 0
	for i := range index.Stages {
		st := &index.Stages[i]
		if st.ExitCode == 0 {
			continue
		}
		failedStages++
		writef(&text, "## Stage: %s\n", st.Stage)
		writef(&text, "Exit: %d\n", st.ExitCode)
		writef(&text, "Detail log: %s\n\n", st.DetailLog)
		for j := range st.Groups {
			g := &st.Groups[j]
			writef(&text, "### Group: %s\n", g.GroupID)
			writef(&text, "Stage: %s\n", g.Stage)
			writef(&text, "Kind: %s\n", g.Kind)
			writef(&text, "Related: %s\n", formatInlineMembers(g.Related))
			writef(&text, "Summary: %s\n", g.Summary)
			writef(&text, "Rerun: %s\n", g.Rerun)
			writef(&text, "Detail log: %s\n", g.DetailLog)
			writef(&text, "Parallel: %s\n", g.Parallel)
			if len(g.Excerpt) > 0 {
				writeln(&text, "Excerpt:")
				for _, line := range cappedStrings(g.Excerpt, maxExcerptLines) {
					writef(&text, "  %s\n", line)
				}
				if len(g.Excerpt) > maxExcerptLines {
					writef(&text, "  ... %d more line(s); see detail log\n", len(g.Excerpt)-maxExcerptLines)
				}
			}
			writeln(&text)
		}
	}
	if failedStages == 0 {
		writeln(&text, "No failures.")
	}
	if err := os.WriteFile(filepath.Join(root, failuresLogPath), []byte(text.String()), 0o600); err != nil {
		return fmt.Errorf("write failure log: %w", err)
	}

	data, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal failure json: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(filepath.Join(root, failuresJSONPath), data, 0o600); err != nil {
		return fmt.Errorf("write failure json: %w", err)
	}
	return nil
}

func printFinalSummary(w io.Writer, index verifyIndex) {
	failed := 0
	for _, st := range index.Stages {
		if st.ExitCode != 0 {
			failed++
		}
	}
	writeln(w)
	writeln(w, "════════════════════════════════════════")
	if failed == 0 {
		writef(w, "PASS  all %d verify stage(s)\n", len(index.Stages))
		writef(w, "Artifacts: %s, %s, %s\n", combinedLogPath, failuresLogPath, statusPath)
		return
	}
	writef(w, "FAIL  %d verify stage(s) failed\n", failed)
	writef(w, "Read first: %s\n", failuresLogPath)
	for i := range index.Stages {
		st := &index.Stages[i]
		if st.ExitCode == 0 {
			continue
		}
		writef(w, "  %s: %d group(s), detail %s\n", st.Stage, len(st.Groups), st.DetailLog)
		for j := range st.Groups {
			g := &st.Groups[j]
			writef(w, "    %s: %s\n", g.GroupID, g.Rerun)
		}
	}
}

func classifyStage(st stage, detailLog, text string) []failureGroup {
	var groups []failureGroup
	switch st.Name {
	case "ze-unit-test-cached", "ze-unit-test-race-changed", "ze-unit-test-changed":
		groups = classifyGoTest(st, detailLog, text)
	case "ze-lint", "ze-lint-changed":
		groups = classifyLint(st, detailLog, text)
	case "ze-vet-evidence", "ze-platform-vet":
		groups = classifyVet(st, detailLog, text)
	case "ze-verify-wiring-docs":
		groups = classifyWiringDocs(st, detailLog, text)
	case "ze-functional-test":
		groups = classifyFunctional(detailLog, text)
	case "ze-exabgp-test":
		groups = classifyExabgp(st, detailLog, text)
	}
	if len(groups) == 0 {
		groups = []failureGroup{genericGroup(st, detailLog, firstUsefulLine(text), excerptFromText(text))}
	}
	for i := range groups {
		if groups[i].Stage == "" {
			groups[i].Stage = st.Name
		}
		if groups[i].DetailLog == "" {
			groups[i].DetailLog = detailLog
		}
		if groups[i].Rerun == "" {
			groups[i].Rerun = st.Rerun
		}
		if groups[i].Parallel == "" {
			groups[i].Parallel = "stage"
		}
		groups[i].Related = uniqueStrings(groups[i].Related)
		groups[i].Excerpt = cappedStrings(groups[i].Excerpt, maxExcerptLines+1)
	}
	return groups
}

func classifyGoTest(st stage, detailLog, text string) []failureGroup {
	lines := splitLines(text)
	groupsByPkg := map[string]*failureGroup{}
	order := []string{}
	var pendingTests []string
	var pendingExcerpt []string
	var compilePkg string
	pkgHeaderRE := regexp.MustCompile(`^#\s+(\S+)`)
	failPkgRE := regexp.MustCompile(`^FAIL\s+(\S+)(?:\s|$)`)
	testRE := regexp.MustCompile(`^--- FAIL: ([^\s(]+)`) // standard go test failure line
	for _, line := range lines {
		if m := pkgHeaderRE.FindStringSubmatch(line); m != nil {
			compilePkg = m[1]
			appendCapped(&pendingExcerpt, line)
			continue
		}
		if m := testRE.FindStringSubmatch(line); m != nil {
			pendingTests = append(pendingTests, m[1])
			appendCapped(&pendingExcerpt, line)
			continue
		}
		if strings.HasPrefix(line, "    ") || strings.Contains(line, ".go:") || strings.Contains(line, "Error") {
			appendCapped(&pendingExcerpt, line)
		}
		m := failPkgRE.FindStringSubmatch(line)
		if len(m) != 2 || m[1] == "FAIL" {
			continue
		}
		pkg := m[1]
		if compilePkg != "" {
			pkg = compilePkg
		}
		const kindBuild = "build"
		kind := "package"
		if strings.Contains(line, "[build failed]") || compilePkg != "" {
			kind = kindBuild
		}
		g, ok := groupsByPkg[pkg]
		if !ok {
			g = &failureGroup{Stage: st.Name, GroupID: "package:" + pkg, Kind: kind, Summary: strings.TrimSpace(line), DetailLog: detailLog, Parallel: "group"}
			groupsByPkg[pkg] = g
			order = append(order, pkg)
		}
		if kind == kindBuild {
			g.Kind = kind
		}
		if len(pendingTests) == 0 {
			g.Related = append(g.Related, pkg)
		} else {
			g.Related = append(g.Related, pendingTests...)
		}
		g.Excerpt = append(g.Excerpt, pendingExcerpt...)
		g.Excerpt = append(g.Excerpt, line)
		g.Rerun = goTestRerun(st.Name, pkg, pendingTests)
		pendingTests = nil
		pendingExcerpt = nil
		compilePkg = ""
	}
	groups := make([]failureGroup, 0, len(order))
	for _, pkg := range order {
		groups = append(groups, *groupsByPkg[pkg])
	}
	return groups
}

func classifyLint(st stage, detailLog, text string) []failureGroup {
	lineRE := regexp.MustCompile(`^([^:\s][^:]*\.go):\d+:\d+:\s*(.*?)(?:\s+\(([^)]+)\))?$`)
	groups := map[string]*failureGroup{}
	order := []string{}
	for _, line := range splitLines(text) {
		m := lineRE.FindStringSubmatch(stripANSI(line))
		if m == nil {
			continue
		}
		file := filepath.ToSlash(m[1])
		linter := m[3]
		if linter == "" {
			linter = "lint"
		}
		dir := filepath.ToSlash(filepath.Dir(file))
		if dir == "." {
			dir = "."
		}
		key := dir + ":" + linter
		g, ok := groups[key]
		if !ok {
			pkg := "./" + dir
			if dir == "." {
				pkg = "."
			}
			g = &failureGroup{Stage: st.Name, GroupID: "lint:" + key, Kind: linter, Summary: strings.TrimSpace(m[2]), Rerun: "golangci-lint run " + shellQuote(pkg), DetailLog: detailLog, Parallel: "group"}
			groups[key] = g
			order = append(order, key)
		}
		g.Related = append(g.Related, file)
		g.Excerpt = append(g.Excerpt, line)
	}
	return orderedGroups(groups, order)
}

func classifyVet(st stage, detailLog, text string) []failureGroup {
	pkgRE := regexp.MustCompile(`^#\s+(\S+)`)
	groups := map[string]*failureGroup{}
	order := []string{}
	current := "./scripts/evidence/..."
	for _, line := range splitLines(text) {
		if m := pkgRE.FindStringSubmatch(line); m != nil {
			current = importPathToPattern(m[1])
		}
		if !strings.Contains(line, ".go:") && !strings.HasPrefix(line, "# ") {
			continue
		}
		g, ok := groups[current]
		if !ok {
			g = &failureGroup{Stage: st.Name, GroupID: "vet:" + current, Kind: "package", Summary: strings.TrimSpace(line), Rerun: "GOOS=linux go vet " + shellQuote(current), DetailLog: detailLog, Parallel: "group"}
			groups[current] = g
			order = append(order, current)
		}
		g.Related = append(g.Related, current)
		g.Excerpt = append(g.Excerpt, line)
	}
	return orderedGroups(groups, order)
}

func classifyWiringDocs(st stage, detailLog, text string) []failureGroup {
	var groups []failureGroup
	current := ""
	runningRE := regexp.MustCompile(`^Running ([A-Za-z0-9_-]+)\.{3}`)
	failedRE := regexp.MustCompile(`([A-Za-z0-9_-]+) failed`)
	for _, line := range splitLines(text) {
		if strings.Contains(line, "Wiring check FAILED") {
			groups = append(groups, failureGroup{Stage: st.Name, GroupID: "subcheck:wiring", Kind: "subcheck", Related: []string{"wiring"}, Summary: strings.TrimSpace(line), Rerun: "python3 scripts/dev/verify_wiring_docs.py", DetailLog: detailLog, Parallel: "group", Excerpt: []string{line}})
			current = "wiring"
			continue
		}
		if m := runningRE.FindStringSubmatch(line); m != nil {
			current = m[1]
			continue
		}
		if m := failedRE.FindStringSubmatch(line); m != nil {
			target := m[1]
			if current != "" && current != target {
				target = current
			}
			groups = append(groups, failureGroup{Stage: st.Name, GroupID: "subcheck:" + target, Kind: "subcheck", Related: []string{target}, Summary: strings.TrimSpace(line), Rerun: "make " + shellQuote(target), DetailLog: detailLog, Parallel: "group", Excerpt: []string{line}})
		}
	}
	return groups
}

func classifyFunctional(detailLog, text string) []failureGroup {
	const prefix = "VERIFY FAILURE GROUP:"
	var groups []failureGroup
	seenSuite := map[string]bool{}
	for _, line := range splitLines(text) {
		_, payload, ok := strings.Cut(line, prefix)
		if !ok {
			continue
		}
		var g failureGroup
		if err := json.Unmarshal([]byte(strings.TrimSpace(payload)), &g); err != nil {
			continue
		}
		if g.DetailLog == "" {
			g.DetailLog = detailLog
		}
		if g.Parallel == "" {
			g.Parallel = "group"
		}
		groups = append(groups, g)
		seenSuite[g.Stage] = true
	}

	summaryRE := regexp.MustCompile(`FAIL\s+\d+ suite\(s\) failed:\s+(.+)`)
	for _, line := range splitLines(stripANSI(text)) {
		m := summaryRE.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		for suite := range strings.FieldsSeq(m[1]) {
			if seenSuite[suite] {
				continue
			}
			groups = append(groups, failureGroup{Stage: suite, GroupID: "suite:" + suite, Kind: "suite", Related: []string{suite}, Summary: "functional suite failed", Rerun: functionalSuiteRerun(suite), DetailLog: detailLog, Parallel: "stage", Excerpt: []string{line}})
			seenSuite[suite] = true
		}
	}
	return groups
}

func functionalSuiteRerun(suite string) string {
	if suite == "install" {
		return "bin/ze-test install --all"
	}
	return "make ze-" + suite + "-test"
}

func classifyExabgp(st stage, detailLog, text string) []failureGroup {
	lines := splitLines(stripANSI(text))
	var failed []string
	var timedOut []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "failed") && strings.Contains(line, "[") {
			failed = append(failed, parseBracketMembers(line)...)
		}
		if strings.HasPrefix(line, "timed out") && strings.Contains(line, "[") {
			timedOut = append(timedOut, parseBracketMembers(line)...)
		}
	}
	var groups []failureGroup
	if len(failed) > 0 {
		groups = append(groups, failureGroup{Stage: st.Name, GroupID: "exabgp:failed", Kind: "mismatch", Related: failed, Summary: fmt.Sprintf("%d ExaBGP encoding test(s) failed", len(failed)), Rerun: exabgpRerun(failed), DetailLog: detailLog, Parallel: "group", Excerpt: excerptFromText(text)})
	}
	if len(timedOut) > 0 {
		groups = append(groups, failureGroup{Stage: st.Name, GroupID: "exabgp:timeout", Kind: "timeout", Related: timedOut, Summary: fmt.Sprintf("%d ExaBGP encoding test(s) timed out", len(timedOut)), Rerun: exabgpRerun(timedOut), DetailLog: detailLog, Parallel: "group", Excerpt: excerptFromText(text)})
	}
	return groups
}

func genericGroup(st stage, detailLog, summary string, excerpt []string) failureGroup {
	if summary == "" {
		summary = "stage failed"
	}
	return failureGroup{Stage: st.Name, GroupID: "stage:" + st.Name, Kind: "stage", Related: []string{st.Name}, Summary: summary, Rerun: st.Rerun, DetailLog: detailLog, Parallel: "stage", Excerpt: excerpt}
}

func goTestRerun(stageName, pkg string, tests []string) string {
	pkgPattern := importPathToPattern(pkg)
	args := []string{"go", "test"}
	if stageName == "ze-unit-test-race-changed" {
		args = append(args, "-race")
	}
	args = append(args, pkgPattern)
	if len(tests) == 1 {
		args = append(args, "-run", "^"+regexp.QuoteMeta(tests[0])+"$")
	}
	return strings.Join(quoteCommand(args), " ")
}

func exabgpRerun(nicks []string) string {
	args := make([]string, 0, maxInlineMembers+9)
	args = append(args, "uv", "run", "--with", "psutil", "--with", "paramiko", "./test/exabgp-compat/bin/functional", "encoding", "--timeout", "180")
	args = append(args, cappedStrings(nicks, maxInlineMembers)...)
	return strings.Join(quoteCommand(args), " ")
}

func importPathToPattern(pkg string) string {
	const module = "codeberg.org/thomas-mangin/ze"
	if pkg == module {
		return "."
	}
	if suffix, ok := strings.CutPrefix(pkg, module+"/"); ok {
		return "./" + suffix
	}
	return pkg
}

func safeStageLogName(name string) string {
	var b strings.Builder
	for _, r := range name {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' || r == '.' {
			b.WriteRune(r)
		} else {
			b.WriteByte('-')
		}
	}
	if b.Len() == 0 {
		return "stage"
	}
	return b.String()
}

func splitLines(text string) []string {
	s := bufio.NewScanner(strings.NewReader(text))
	lines := []string{}
	for s.Scan() {
		lines = append(lines, s.Text())
	}
	return lines
}

func firstUsefulLine(text string) string {
	for _, line := range splitLines(stripANSI(text)) {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "$ ") || strings.HasPrefix(trimmed, "### Stage") {
			continue
		}
		return trimmed
	}
	return "stage failed"
}

func excerptFromText(text string) []string {
	var excerpt []string
	for _, line := range splitLines(stripANSI(text)) {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "$ ") || strings.HasPrefix(trimmed, "### Stage") {
			continue
		}
		appendCapped(&excerpt, trimmed)
	}
	return excerpt
}

func appendCapped(dst *[]string, value string) {
	if strings.TrimSpace(value) == "" {
		return
	}
	if len(*dst) < maxExcerptLines+1 {
		*dst = append(*dst, value)
	}
}

func cappedStrings(values []string, cap int) []string {
	if len(values) <= cap {
		return values
	}
	out := make([]string, cap)
	copy(out, values[:cap])
	return out
}

func formatInlineMembers(values []string) string {
	if len(values) == 0 {
		return "-"
	}
	shown := cappedStrings(values, maxInlineMembers)
	text := strings.Join(shown, ", ")
	if len(values) > maxInlineMembers {
		text += fmt.Sprintf(", +%d more", len(values)-maxInlineMembers)
	}
	return text
}

func uniqueStrings(values []string) []string {
	seen := map[string]bool{}
	out := values[:0]
	for _, value := range values {
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func orderedGroups(groups map[string]*failureGroup, order []string) []failureGroup {
	out := make([]failureGroup, 0, len(order))
	for _, key := range order {
		out = append(out, *groups[key])
	}
	return out
}

func parseBracketMembers(line string) []string {
	start := strings.IndexByte(line, '[')
	end := strings.LastIndexByte(line, ']')
	if start < 0 || end <= start {
		return nil
	}
	parts := strings.Split(line[start+1:end], ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if nick := strings.TrimSpace(part); nick != "" {
			out = append(out, nick)
		}
	}
	return out
}

var ansiRE = regexp.MustCompile(`\x1b\[[0-9;]*[A-Za-z]`)

func stripANSI(s string) string {
	return ansiRE.ReplaceAllString(s, "")
}

var safeShellRE = regexp.MustCompile(`^[A-Za-z0-9_./:=@%+,-]+$`)

func shellQuote(s string) string {
	if safeShellRE.MatchString(s) {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

func quoteCommand(args []string) []string {
	quoted := make([]string, len(args))
	for i, arg := range args {
		quoted[i] = shellQuote(arg)
	}
	return quoted
}

func writef(w io.Writer, format string, args ...any) {
	_, _ = fmt.Fprintf(w, format, args...) //nolint:errcheck // output
}

func writeln(w io.Writer, args ...any) {
	_, _ = fmt.Fprintln(w, args...) //nolint:errcheck // output
}

func writeVerifyStatus(root string, exitCode int, mode, skipped string, now time.Time) error {
	if err := os.MkdirAll(filepath.Join(root, "tmp"), 0o750); err != nil {
		return err
	}
	hash := computeTreeHash(root)
	sha := gitOutput(root, "rev-parse", "HEAD")
	if strings.TrimSpace(sha) == "" {
		sha = "unknown"
	}
	var content strings.Builder
	writef(&content, "exit=%d\n", exitCode)
	writef(&content, "timestamp=%s\n", now.UTC().Format(time.RFC3339))
	// mode distinguishes FRESH(ze-verify) from the weaker FRESH(ze-verify-changed);
	// skipped records ZE_SKIP_SUITES so a partial pass cannot read as a full one.
	writef(&content, "mode=%s\n", mode)
	writef(&content, "skipped=%s\n", skipped)
	writef(&content, "git_sha=%s\n", strings.TrimSpace(sha))
	writef(&content, "tree_hash=%s\n", hash)
	return os.WriteFile(filepath.Join(root, statusPath), []byte(content.String()), 0o600)
}

func computeTreeHash(root string) string {
	h := sha256.New()
	io.WriteString(h, gitOutput(root, "rev-parse", "HEAD")) //nolint:errcheck // hash writer never fails
	io.WriteString(h, gitOutput(root, "diff", "HEAD"))      //nolint:errcheck // hash writer never fails
	untracked := nonEmptyLines(gitOutput(root, "ls-files", "-o", "--exclude-standard"))
	sort.Strings(untracked)
	for _, rel := range untracked {
		io.WriteString(h, rel+"\n") //nolint:errcheck // hash writer never fails
		data, err := readControlledFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			io.WriteString(h, "MISSING\n") //nolint:errcheck // hash writer never fails
			continue
		}
		fileHash := sha256.Sum256(data)
		io.WriteString(h, hex.EncodeToString(fileHash[:])+"\n") //nolint:errcheck // hash writer never fails
	}
	return hex.EncodeToString(h.Sum(nil))
}

func gitOutput(root string, args ...string) string {
	cmd := exec.CommandContext(context.Background(), "git", args...) //nolint:gosec // fixed binary and caller-controlled arguments
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return string(out)
}

func nonEmptyLines(text string) []string {
	var lines []string
	for line := range strings.SplitSeq(text, "\n") {
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

func readControlledFile(path string) ([]byte, error) {
	//nolint:gosec // path is generated by the verify runner or enumerated by git within the repo root
	return os.ReadFile(path)
}

func openControlledFile(path string) (*os.File, error) {
	//nolint:gosec // path is generated by the verify runner within the repo root
	return os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
}

// contendedWarning warns when the verify run is CPU-contended. It uses the same
// hostload.Contended definition as the functional-test runner (single source of
// truth), so the two surfaces cannot disagree: process concurrency ALONE is not
// enough -- load must also exceed CPU count before the run is called contended.
func contendedWarning() string {
	load := hostload.Snapshot()
	if !load.Contended() {
		return ""
	}
	var tb textbuf.Buffer
	tb.Str("contended run detected: ")
	tb.Int(int64(load.ZeProcs))
	tb.Str(" ze-test, ")
	tb.Int(int64(load.GoTestProcs))
	tb.Str(" go-test processes; results may include environmental failures")
	return tb.String()
}
