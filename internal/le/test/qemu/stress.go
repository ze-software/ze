// Design: docs/architecture/testing/qemu-integration.md -- running the BGP stress harness in the guest
// Related: run.go -- the guest this action boots
// Related: ../integration/stress.go -- the harness the guest runs, and the report it writes
//
// stress.go carries `le test qemu stress`: the one host command that boots the
// guest, runs `le test integration stress` inside it as root, and copies the
// JSON report, every profile the report lists and the DUT binary to a host
// directory. The harness needs root and network namespaces, which a macOS host
// has neither of, so the BGP perf measurement runs here on the owner's Mac.

package testqemu

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	leaction "github.com/ze-software/ze/internal/le/le/action"
	lepath "github.com/ze-software/ze/internal/le/le/path"
	testintegration "github.com/ze-software/ze/internal/le/test/integration"
)

// The stress action's verb and keywords. keywordTimeout is shared with run.
const (
	stressVerb      = "stress"
	keywordScenario = "scenario"
	keywordPrefixes = "prefixes"
	keywordPprof    = "pprof"
	keywordOutput   = "output"
)

// stressGuestPackages is what the harness preflight looks for: `ip` and
// `ethtool`. Its own fallback installs them with apt-get, which Alpine has not.
const stressGuestPackages = "iproute2 ethtool"

// stressGuestReportRel is where the guest writes the harness JSON, under the
// shared checkout, so the host reads the file the guest wrote.
const stressGuestReportRel = "tmp/qemu/stress-report.json"

// stressReportName is the report's name in the output directory.
const stressReportName = "report.json"

// stressDefaultTimeout bounds the guest command: a cold DUT build inside the
// guest and every round of the scenario, each of which carries its own timeout.
const stressDefaultTimeout = time.Hour

// stressRequest is one `le test qemu stress` invocation after its keywords are
// parsed. Prefixes zero keeps the registry's counts, which is what a
// measurement records; above zero it is a smoke run.
type stressRequest struct {
	Scenario string
	Prefixes int
	Pprof    bool
	Output   string
	Timeout  time.Duration
}

// StressGuestReport is the answer of one guest stress run: the guest run, where
// the evidence went, and what was copied there.
type StressGuestReport struct {
	Run     *RunReport `json:"run"`
	Output  string     `json:"output"`
	Files   []string   `json:"files,omitempty"`
	Failure string     `json:"failure,omitempty"`
}

// Text renders the guest verdict and the copied files for a person.
func (r *StressGuestReport) Text() string {
	var b textbuf.Buffer
	b.Str(r.Run.Text())
	if r.Failure != "" {
		b.Str("\nevidence: ").Str(r.Failure)
	}
	if len(r.Files) != 0 {
		b.Str("\ncopied to ").Str(r.Output).Byte(':')
		for _, file := range r.Files {
			b.Str("\n  ").Str(file)
		}
	}
	return b.String()
}

// stressAction is the table row. The scenario placeholder is the harness
// registry itself, so a new scenario is discoverable here with no edit.
func stressAction() leaction.Action {
	return leaction.Action{
		Verb: stressVerb,
		Why: "boot the guest, run `le test integration stress` for one scenario inside it as root," +
			" and copy the JSON report, every profile it lists and the DUT binary into the output" +
			" directory, which must be empty. `prefixes` shortens every round for a smoke run, which" +
			" is never a measurement; `pprof` captures the CPU, heap and goroutine profiles",
		Parameters: []leaction.Parameter{
			{Keyword: keywordScenario, Value: strings.Join(testintegration.StressScenarios(), ","), Requirement: leaction.Required},
			{Keyword: keywordOutput, Value: "directory", Requirement: leaction.Required},
			{Keyword: keywordPrefixes, Value: "count", Requirement: leaction.Optional},
			{Keyword: keywordPprof},
			{Keyword: keywordTimeout, Value: "duration", Requirement: leaction.Optional},
		},
		AnswerArgs: runStressHere,
	}
}

func runStressHere(args leaction.Arguments) (any, int) {
	working, err := os.Getwd()
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	request, err := parseStressArguments(args, working)
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	root, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	return runStressGuest(root, request)
}

// parseStressArguments refuses every malformed invocation on the host, before a
// guest boots for minutes only to fail. A relative output is read against
// working, the directory the operator typed it in.
func parseStressArguments(args leaction.Arguments, working string) (stressRequest, error) {
	request := stressRequest{
		Scenario: args.One(keywordScenario), Pprof: args.Has(keywordPprof),
		Output: args.One(keywordOutput), Timeout: stressDefaultTimeout,
	}
	if request.Scenario == "" {
		return request, errors.New("qemu stress requires scenario <name>, one of " + strings.Join(testintegration.StressScenarios(), ", "))
	}
	if !slices.Contains(testintegration.StressScenarios(), request.Scenario) {
		return request, fmt.Errorf("qemu stress has no scenario %q; the harness registry holds %s",
			request.Scenario, strings.Join(testintegration.StressScenarios(), ", "))
	}
	if request.Output == "" {
		return request, errors.New("qemu stress requires output <directory>, the host directory the report and profiles are copied to")
	}
	if !filepath.IsAbs(request.Output) {
		request.Output = filepath.Join(working, request.Output)
	}
	if named := args.One(keywordPrefixes); named != "" {
		prefixes, err := strconv.Atoi(named)
		if err != nil {
			return request, fmt.Errorf("qemu stress prefixes %q is not a count: %w", named, err)
		}
		if prefixes <= 0 {
			return request, fmt.Errorf("qemu stress prefixes must be above zero, got %d; omit it for the registry counts", prefixes)
		}
		request.Prefixes = prefixes
	}
	if named := args.One(keywordTimeout); named != "" {
		timeout, err := positiveWholeSeconds(keywordTimeout, named)
		if err != nil {
			return request, err
		}
		request.Timeout = timeout
	}
	return request, nil
}

// stressGuestCommand is what the guest runs from /workspace: the guest le,
// which is the BGP peer the harness starts, with the harness's own knobs.
func stressGuestCommand(request stressRequest, goarch string) string {
	var b textbuf.Buffer
	b.Str("STRESS_SCENARIO=").Str(shellQuote(request.Scenario))
	if request.Prefixes > 0 {
		b.Str(" STRESS_PREFIXES=").Int(int64(request.Prefixes))
	}
	if request.Pprof {
		b.Str(" ZE_PPROF=1")
	}
	return b.Byte(' ').Str(filepath.ToSlash(GuestLeRel(goarch))).
		Str(" test integration stress '|' json > ").Str(stressGuestReportRel).String()
}

// runStressGuest builds the guest le, boots the guest, and copies the evidence.
// The report a previous run left is removed first, so a guest that writes none
// is an error rather than an old run read as this one.
func runStressGuest(root string, request stressRequest) (any, int) {
	goarch := GuestArch()
	reportPath := filepath.Join(root, stressGuestReportRel)
	if err := os.Remove(reportPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		leaction.ReportError(fmt.Errorf("remove the previous guest stress report %s: %w", reportPath, err))
		return nil, 1
	}
	options, err := parseRunArguments(leaction.Arguments{
		keywordCommand: {stressGuestCommand(request, goarch)},
		"packages":     {stressGuestPackages},
		keywordTimeout: {request.Timeout.String()},
	})
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	options.HardwareOnly = true
	if _, err := buildGuestLe(root, goarch); err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	ctx, cancel := guestContext()
	defer cancel()
	run, err := NewRun(root, options).Execute(ctx)
	if err != nil {
		leaction.ReportError(err)
		return &run, 1
	}
	report := &StressGuestReport{Run: &run, Output: request.Output}
	raw, err := os.ReadFile(reportPath)
	if err != nil {
		report.Failure = "the guest wrote no stress report: " + err.Error()
		return report, 1
	}
	report.Files, err = copyStressEvidence(root, raw, request.Output)
	if err != nil {
		report.Failure = err.Error()
		return report, 1
	}
	return report, runExitCode(&run)
}

// copyStressEvidence writes the report into output, then copies every profile
// and the DUT binary the report names. The paths come from the report, the
// harness's own record of what it wrote, never from a second list here. A guest
// path is under /workspace, the shared checkout at root; any other path is on
// the guest's own disk, gone with the guest, and refused. output must be empty
// or absent, so one directory never mixes two runs.
func copyStressEvidence(root string, raw []byte, output string) ([]string, error) {
	var report testintegration.StressReport
	if err := json.Unmarshal(raw, &report); err != nil {
		return nil, fmt.Errorf("the guest stress report is not the harness JSON: %w", err)
	}
	if len(report.Scenarios) == 0 {
		return nil, errors.New("the guest stress report holds no scenario: " + report.Failure)
	}
	if err := emptyOutput(output); err != nil {
		return nil, err
	}
	reportCopy := filepath.Join(output, stressReportName)
	if err := os.WriteFile(reportCopy, raw, 0o644); err != nil { //nolint:gosec // evidence the owner reads and pastes
		return nil, err
	}
	files := []string{reportCopy}
	for i := range report.Scenarios {
		scenario := &report.Scenarios[i]
		guestPaths := make([]string, 0, len(scenario.Profiles)+1)
		for _, profile := range scenario.Profiles {
			guestPaths = append(guestPaths, profile.Path)
		}
		if scenario.Binary != "" {
			guestPaths = append(guestPaths, scenario.Binary)
		}
		for _, guestPath := range guestPaths {
			copied, err := copyGuestFile(root, guestPath, output)
			if err != nil {
				return files, err
			}
			files = append(files, copied)
		}
	}
	return files, nil
}

// emptyOutput creates output, or refuses one that already holds a file.
func emptyOutput(output string) error {
	entries, err := os.ReadDir(output)
	if err == nil && len(entries) != 0 {
		return fmt.Errorf("output directory %s is not empty; name a new one so two runs never mix", output)
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return os.MkdirAll(output, 0o750)
}

// copyGuestFile copies the file the guest wrote at guestPath into output.
func copyGuestFile(root, guestPath, output string) (string, error) {
	relative, shared := strings.CutPrefix(guestPath, GuestWorkspace+"/")
	if !shared {
		return "", fmt.Errorf("the guest wrote %s outside %s, so the host cannot read it", guestPath, GuestWorkspace)
	}

	source, err := os.Open(filepath.Join(root, filepath.FromSlash(relative)))
	if err != nil {
		return "", fmt.Errorf("the report names %s, which the host cannot read: %w", guestPath, err)
	}
	defer source.Close() //nolint:errcheck // read-only file

	target := filepath.Join(output, filepath.Base(relative))
	destination, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o755) //nolint:gosec // the DUT binary stays executable for pprof
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(destination, source); err != nil {
		destination.Close() //nolint:errcheck,gosec // the copy error is the one reported
		return "", fmt.Errorf("copy %s to %s: %w", guestPath, target, err)
	}
	if err := destination.Close(); err != nil {
		return "", err
	}
	return target, nil
}
