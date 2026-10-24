package fixture

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

// verifyFirstRedRefusesACutDriver types `le verify worktree part 1 of 2
// first-red` against a throwaway checkout and asserts the refusal: exit 2,
// the reason (a piece is dealt for debt clearing), the two spellings that
// work, and no worktree created.
func verifyFirstRedRefusesACutDriver(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return errors.New("verify-first-red-refuses-a-cut fixture takes no arguments")
	}
	le, err := nativeLEBinary()
	if err != nil {
		return err
	}
	repo, err := os.MkdirTemp("", "ze-verify-first-red-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(repo) //nolint:errcheck // fixture cleanup

	// A whole-tree verb, so it runs at a terminal, as Thomas runs it.
	out, code, err := terminalCommand(ctx, repo, envRootedAt(repo), le,
		"verify", "worktree", "part", "1", "of", "2", "first-red")
	if err != nil {
		return fmt.Errorf("run le verify worktree: %w", err)
	}
	if code != 2 {
		return fmt.Errorf("le verify worktree part 1 of 2 first-red exit=%d, want 2: %s", code, out)
	}
	for _, want := range []string{
		"first-red cannot be combined with part",
		"debt clearing",
		"le verify worktree first-red",
		"le verify worktree part <i> of <n>",
	} {
		if !strings.Contains(out, want) {
			return fmt.Errorf("refusal does not say %q: %s", want, out)
		}
	}
	fmt.Fprintln(os.Stdout, "first-red-with-part-refused") //nolint:errcheck // progress output
	if _, err := os.Stat(filepath.Join(repo, "tmp", "verify-worktree")); !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("the refused line created a worktree directory (%w)", err)
	}
	fmt.Fprintln(os.Stdout, "no-worktree-created") //nolint:errcheck // progress output
	return nil
}

// firstRedCheapRedStage is the fast stage the stop scenario makes cheapest and
// red. `arch tier check` reads internal/component, which the throwaway
// checkout does not hold, so it exits non-zero in well under a second.
const firstRedCheapRedStage = "arch tier/check"

// verifyFirstRedStopsDriver runs `le verify current fast first-red` against a
// throwaway checkout whose stage-time history makes one red stage the
// cheapest, and asserts the run stopped after it: the progress line, the
// combined log and the failure index name the stop and the stages not run, the
// history gained a line for the stage that ran and none for the rest, and
// `le verify status check` answers STALE for the stopped run.
func verifyFirstRedStopsDriver(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return errors.New("verify-first-red-stops fixture takes no arguments")
	}
	le, err := nativeLEBinary()
	if err != nil {
		return err
	}
	repo, err := os.MkdirTemp("", "ze-verify-first-red-stops-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(repo) //nolint:errcheck // fixture cleanup

	if err := gitFixture(ctx, repo, map[string]string{
		fileGoMod: "module fixture/firstred\n\ngo 1.24\n", fileFeatureGates: contentFeatureGate,
		fileGitIgnore: contentGitIgnoreTmp,
	}); err != nil {
		return err
	}
	run := func(arguments ...string) (string, int, error) {
		return rawCommand(ctx, repo, envRootedAt(repo), le, arguments...)
	}

	stages, err := firstRedFastStages(run)
	if err != nil {
		return err
	}
	seeded, err := seedFirstRedHistory(repo, stages)
	if err != nil {
		return err
	}
	fmt.Fprintln(os.Stdout, "cheap-red-stage-seeded") //nolint:errcheck // progress output

	stop := "stopped at " + firstRedCheapRedStage + ", " + strconv.Itoa(len(stages)-1) + " stages not run"
	// A whole-tree verb, so it runs at a terminal, as Thomas runs it.
	out, code, err := terminalCommand(ctx, repo, envRootedAt(repo), le, "verify", "current", "fast", "first-red")
	if err != nil {
		return fmt.Errorf("run le verify current fast first-red: %w", err)
	}
	if code == 0 {
		return fmt.Errorf("le verify current fast first-red exit=0 over a red stage: %s", out)
	}
	if !strings.Contains(out, stop) {
		return fmt.Errorf("the run did not say %q: %s", stop, out)
	}
	fmt.Fprintln(os.Stdout, "run-stopped-at-first-red") //nolint:errcheck // progress output

	if err := firstRedLogsNameTheStop(repo, stop, stages); err != nil {
		return err
	}
	fmt.Fprintln(os.Stdout, "log-names-unreached-stages") //nolint:errcheck // progress output

	if err := firstRedHistoryGainedOneLine(repo, seeded); err != nil {
		return err
	}
	fmt.Fprintln(os.Stdout, "history-holds-only-the-reached-stage") //nolint:errcheck // progress output

	out, code, err = run("verify", "status", "check")
	if err != nil {
		return fmt.Errorf("run le verify status check: %w", err)
	}
	if code == 0 {
		return fmt.Errorf("le verify status check exit=0 after a stopped run: %s", out)
	}
	if !strings.Contains(out, "STALE: last verify failed") {
		return fmt.Errorf("le verify status check does not answer the stopped run as STALE: %s", out)
	}
	fmt.Fprintln(os.Stdout, "status-answers-stale") //nolint:errcheck // progress output
	return nil
}

// firstRedFastStages answers the fast population in declared order, from the
// real `le verify list fast`, so the scenario follows the population as it
// changes. The list prints one stage name per line, and a name holds spaces.
func firstRedFastStages(run func(...string) (string, int, error)) ([]string, error) {
	listed, code, err := run("verify", "list", "fast")
	if err != nil {
		return nil, fmt.Errorf("run le verify list fast: %w", err)
	}
	if code != 0 {
		return nil, fmt.Errorf("le verify list fast exit=%d: %s", code, listed)
	}
	stages := strings.Split(strings.TrimSpace(listed), "\n")
	if !slices.Contains(stages, firstRedCheapRedStage) {
		return nil, fmt.Errorf("the fast population does not hold %q: %s", firstRedCheapRedStage, listed)
	}
	if len(stages) < 2 {
		return nil, fmt.Errorf("the fast population leaves no stage to stop before: %s", listed)
	}
	return stages, nil
}

// seedFirstRedHistory writes a stage-time history that records every fast
// stage at 100 seconds and the red stage at a millisecond, so the cost order
// runs the red stage first. It answers the seeded bytes.
func seedFirstRedHistory(repo string, stages []string) (string, error) {
	var history strings.Builder
	for _, stage := range stages {
		seconds := "100.000"
		if stage == firstRedCheapRedStage {
			seconds = "0.001"
		}
		history.WriteString(stage + "\t" + seconds + "\t0\tfast\tseed\t2026-10-21T00:00:00Z\n")
	}
	path := filepath.Join(repo, "tmp", "ze-verify-stage-times.tsv")
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return "", err
	}
	return history.String(), os.WriteFile(path, []byte(history.String()), 0o600)
}

// firstRedLogsNameTheStop asserts the combined log records the stop after the
// one stage that ran, and the failure index names a stage the run never
// reached.
func firstRedLogsNameTheStop(repo, stop string, stages []string) error {
	logs, err := filepath.Glob(filepath.Join(repo, "tmp", "verify", "*", "ze-verify.log"))
	if err != nil {
		return fmt.Errorf("find the combined log: %w", err)
	}
	if len(logs) != 1 {
		return fmt.Errorf("want one combined log, found %v", logs)
	}
	combined, err := os.ReadFile(logs[0]) //nolint:gosec // the fixture's own throwaway checkout
	if err != nil {
		return err
	}
	if !strings.Contains(string(combined), "### Run "+strings.Replace(stop, "stopped at ", "stopped at first red: ", 1)) {
		return fmt.Errorf("the combined log does not record the stop: %s", combined)
	}
	if strings.Count(string(combined), "### Stage result: ") != 1 {
		return fmt.Errorf("the combined log shows other than the one stage run: %s", combined)
	}
	index, err := os.ReadFile(filepath.Join(repo, "tmp", "ze-verify-failures.log")) //nolint:gosec // the fixture's own throwaway checkout
	if err != nil {
		return err
	}
	if !strings.Contains(string(index), "Stopped at first red: "+firstRedCheapRedStage+"\n") {
		return fmt.Errorf("the failure index does not record the stop: %s", index)
	}
	unreached := stages[0]
	if unreached == firstRedCheapRedStage {
		unreached = stages[1]
	}
	stopLine := ""
	for line := range strings.SplitSeq(string(index), "\n") {
		if rest, found := strings.CutPrefix(line, "Not reached, the run stopped at its first red: "); found {
			stopLine = rest
		}
	}
	if !slices.Contains(strings.Split(stopLine, ", "), unreached) {
		return fmt.Errorf("the failure index does not name %q as not reached: %s", unreached, index)
	}
	return nil
}

// firstRedHistoryGainedOneLine asserts the run appended exactly one history
// line, for the stage it ran, and none for a stage it never reached.
func firstRedHistoryGainedOneLine(repo, seeded string) error {
	recorded, err := os.ReadFile(filepath.Join(repo, "tmp", "ze-verify-stage-times.tsv")) //nolint:gosec // the fixture's own throwaway checkout
	if err != nil {
		return err
	}
	appended, found := strings.CutPrefix(string(recorded), seeded)
	if !found {
		return fmt.Errorf("the run rewrote the seeded history instead of appending: %q", recorded)
	}
	if !strings.HasPrefix(appended, firstRedCheapRedStage+"\t") {
		return fmt.Errorf("the history did not gain a line for the stage that ran: %q", appended)
	}
	if strings.Count(appended, "\n") != 1 {
		return fmt.Errorf("the history gained lines for stages the run never reached: %q", appended)
	}
	return nil
}
