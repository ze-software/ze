package fixture

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

func init() {
	Register("runner/verify-scope-freshness-scoped", verifyScopeFreshnessDriver)
	Register("runner/verify-scope-selector", verifyScopeSelectorDriver)
	Register("runner/verify-scope-suite-map", verifyScopeSuiteMapDriver)
}

func nativeLEBinary() (string, error) {
	root := os.Getenv("ZE_REPO_ROOT")
	if root == "" {
		return "", errors.New("ZE_REPO_ROOT is not set")
	}
	path := filepath.Join(root, "bin", "le")
	info, err := os.Stat(path)
	if err != nil || info.Mode()&0o111 == 0 {
		return "", fmt.Errorf("native le binary is not executable: %s", path)
	}
	return path, nil
}

func verifyScopeFreshnessDriver(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return errors.New("freshness fixture takes no arguments")
	}
	repo, err := os.MkdirTemp("", "ze-verify-scope-freshness-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(repo) //nolint:errcheck // fixture cleanup
	if err := gitFixture(ctx, repo, map[string]string{fileGoMod: "module fixture/freshness\n\ngo 1.24\n", fileFeatureGates: contentFeatureGate, fileGitIgnore: contentGitIgnoreTmp, "mine.txt": "mine\n", "theirs.txt": "theirs\n"}); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(repo, "mine.txt"), []byte("mine\nmy edit\n"), 0o600); err != nil {
		return err
	}
	le, err := nativeLEBinary()
	if err != nil {
		return err
	}
	env := envRootedAt(repo)
	run := func(arguments ...string) (string, int, error) { return rawCommand(ctx, repo, env, le, arguments...) }
	if out, code, err := run("verify status", "write", "exit-code", "0", "mode", "full"); err != nil || code != 0 {
		return fmt.Errorf("write status exit=%d: %w %s", code, err, out)
	}
	fmt.Fprintln(os.Stdout, "scratch-repo-ready") //nolint:errcheck // progress output
	if err := os.WriteFile(filepath.Join(repo, "theirs.txt"), []byte("theirs\ntheir edit\n"), 0o600); err != nil {
		return err
	}
	if out, code, err := run("verify status", "check", "path", "mine.txt"); err != nil || code != 0 || !strings.Contains(out, "FRESH") {
		return fmt.Errorf("scoped mine check exit=%d: %w %s", code, err, out)
	}
	fmt.Fprintln(os.Stdout, "scoped-fresh-for-my-path") //nolint:errcheck // progress output
	if _, code, err := run("verify status", "check"); err != nil || code == 0 {
		return fmt.Errorf("unscoped check exit=%d: %w", code, err)
	}
	fmt.Fprintln(os.Stdout, "unscoped-still-whole-tree") //nolint:errcheck // progress output
	if _, code, err := run("verify status", "check", "path", "theirs.txt"); err != nil || code == 0 {
		return fmt.Errorf("theirs scoped check exit=%d: %w", code, err)
	}
	fmt.Fprintln(os.Stdout, "scoped-stale-for-their-path") //nolint:errcheck // progress output
	if err := os.WriteFile(filepath.Join(repo, "mine.txt"), []byte("mine\nmy second edit\n"), 0o600); err != nil {
		return err
	}
	if _, code, err := run("verify status", "check", "path", "mine.txt"); err != nil || code == 0 {
		return fmt.Errorf("own edit check exit=%d: %w", code, err)
	}
	fmt.Fprintln(os.Stdout, "scoped-stale-for-my-own-edit") //nolint:errcheck // progress output
	if err := os.WriteFile(filepath.Join(repo, "mine.txt"), []byte("mine\nmy edit\n"), 0o600); err != nil {
		return err
	}
	if _, code, err := run("verify status", "check", "path", "mine.txt"); err != nil || code != 0 {
		return fmt.Errorf("reverted edit check exit=%d: %w", code, err)
	}
	manifest := filepath.Join(repo, "tmp", "ze-verify-manifest.txt")
	body, err := os.ReadFile(manifest) //nolint:gosec // the path is the fixture's own scratch file
	if err != nil {
		return err
	}
	lines := strings.Split(string(body), "\n")
	found := false
	for index, line := range lines {
		if strings.HasSuffix(line, " mine.txt") {
			lines[index] = "MOVED-DURING-RUN mine.txt"
			found = true
		}
	}
	if !found {
		return errors.New("mine.txt missing from verify manifest")
	}
	if err := os.WriteFile(manifest, []byte(strings.Join(lines, "\n")), 0o600); err != nil {
		return err
	}
	out, code, err := run("verify status", "check", "path", "mine.txt")
	if err != nil || code == 0 || !strings.Contains(out, "moved while the run was in flight") {
		return fmt.Errorf("moved check exit=%d: %w %s", code, err, out)
	}
	fmt.Fprintln(os.Stdout, "moved-path-stays-stale") //nolint:errcheck // progress output
	if out, code, err := run("verify status", "write", "exit-code", "1", "mode", "full"); err != nil || code != 0 {
		return fmt.Errorf("write red status exit=%d: %w %s", code, err, out)
	}
	if _, code, err := run("verify status", "check", "path", "mine.txt"); err != nil || code == 0 {
		return fmt.Errorf("red run scoped check exit=%d: %w", code, err)
	}
	fmt.Fprintln(os.Stdout, "red-run-stale-for-every-scope") //nolint:errcheck // progress output
	return nil
}

func verifyScopeSelectorDriver(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return errors.New("selector fixture takes no arguments")
	}
	le, err := nativeLEBinary()
	if err != nil {
		return err
	}
	root := os.Getenv("ZE_REPO_ROOT")
	work, err := os.MkdirTemp("", "ze-verify-scope-selector-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(work) //nolint:errcheck // fixture cleanup
	// envRootedAt over the REAL root, not os.Environ(): this scenario asks the
	// selector about this checkout, so ZE_REPO_ROOT keeps its value, and the
	// other two inherited variables still have to go (inheritedDropped).
	env := envRootedAt(root)
	run := func(path, printing string) (string, string, int, error) {
		input := filepath.Join(work, "scope.paths")
		if err := os.WriteFile(input, []byte(path+"\n"), 0o600); err != nil {
			return "", "", -1, err
		}
		return rawCommandStreams(ctx, root, env, le, "changed", "scope", "print", printing, "paths-from", input)
	}
	// Both streams: a refusal from le itself prints on stderr and leaves stdout
	// empty, so reporting stdout alone answered "exit=2" with nothing after it.
	out, diagnostics, code, err := run("internal/component/ssh/ssh.go", "both")
	if err != nil || code != 0 {
		return fmt.Errorf("SSH selector exit=%d: %w\n%s%s", code, err, out, diagnostics)
	}
	sections := strings.Split(out, "# tags\n")
	if len(sections) != 2 {
		return fmt.Errorf("bad selector output: %s", out)
	}
	packages := strings.TrimSpace(strings.TrimPrefix(sections[0], "# packages\n"))
	if packages != "./cmd/ze\n./cmd/ze/hub\n./internal/component/ssh" {
		return fmt.Errorf("SSH packages changed: %s", packages)
	}
	if strings.TrimSpace(sections[1]) != "ze_ssh" {
		return fmt.Errorf("SSH tags changed: %s", sections[1])
	}
	fmt.Fprintln(os.Stdout, "ssh-selects-its-gated-importer") //nolint:errcheck // progress output
	fmt.Fprintln(os.Stdout, "ssh-reaches-one-feature")        //nolint:errcheck // progress output
	const unclassified = "demos/terminal/rpki/demo.cast"
	out, diagnostics, code, err = run(unclassified, "packages")
	if err != nil || code != 0 {
		return fmt.Errorf("unclassified selector exit=%d: %w\n%s%s", code, err, out, diagnostics)
	}
	// The stderr line is the whole guarantee for a kind no rule names: the
	// answer is never silently narrow, and the operator holds the path that
	// would need a rule. Reading stdout alone would let the naming disappear
	// with this scenario still green.
	if !strings.Contains(diagnostics, "no rule names "+unclassified) {
		return fmt.Errorf("the selector did not name %s on stderr: %s", unclassified, diagnostics)
	}
	fmt.Fprintln(os.Stdout, "unclassified-path-is-named") //nolint:errcheck // progress output
	if strings.TrimSpace(out) == "./..." {
		return errors.New("unclassified path selected every package")
	}
	fmt.Fprintln(os.Stdout, "unclassified-path-narrows-to-its-readers") //nolint:errcheck // progress output
	for _, path := range []string{fileGoMod, "go.sum", "vendor/example.com/dep/dep.go"} {
		out, diagnostics, code, err = run(path, "packages")
		if err != nil || code != 0 || strings.TrimSpace(out) != "./..." {
			return fmt.Errorf("dependency %s did not widen: exit=%d %w %s", path, code, err, out)
		}
		if !strings.Contains(diagnostics, path+" changed, so a dependency moved") {
			return fmt.Errorf("the selector did not name %s as the move: %s", path, diagnostics)
		}
	}
	fmt.Fprintln(os.Stdout, "dependency-move-widens-and-names-the-path") //nolint:errcheck // progress output
	return nil
}

// The suite names and the package this scenario reasons about. Each is written
// six times below, in the map the fixture records and in the answers it judges,
// so each is named once here.
const (
	suiteMapEncode     = "encode"
	suiteMapParse      = "parse"
	suiteMapUI         = "ui"
	suiteMapWeb        = "web"
	suiteMapSSHPackage = "./internal/component/ssh"
)

// suiteMapScenario is one question put to `le functional select`: the change
// set a verify run published, and what the answer must say about it.
type suiteMapScenario struct {
	// name is the progress word the .ci asserts on.
	name string
	// packages is the change-set answer this scenario publishes.
	packages []string
	// skipSuites is the operator override, empty for every scenario but one.
	skipSuites string
	// narrowed says whether the map must have ruled a suite out.
	narrowed bool
	// runs and ruledOut name suites the answer must place, and says means the
	// printed reason must hold that text.
	runs     []string
	ruledOut []string
	says     string
}

// verifyScopeSuiteMapDriver drives the REAL `le functional select` over a
// scratch checkout holding a suite map this fixture wrote.
//
// What it proves is the SELECTION: a recorded map plus a change set, in, and
// the run list a gating run would start, out. It deliberately proves nothing
// about the RECORDING, which is a whole instrumented functional run and is
// asserted by the reduction's own tests. A map this fixture writes by hand is
// not a recording and never stands in for one.
//
// The change set arrives through ZE_VERIFY_SCOPE_PACKAGES, which is the file a
// verify run publishes before its first stage. That keeps the scenario about
// the intersection: which packages a change reaches is the change-set
// selector's own question, and runner/verify-scope-selector already drives it.
func verifyScopeSuiteMapDriver(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return errors.New("suite map fixture takes no arguments")
	}
	le, err := nativeLEBinary()
	if err != nil {
		return err
	}
	repo, err := os.MkdirTemp("", "ze-verify-scope-suite-map-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(repo) //nolint:errcheck // fixture cleanup

	if err := gitFixture(ctx, repo, map[string]string{
		fileGoMod:                       "module fixture/suitemap\n\ngo 1.24\n",
		fileFeatureGates:                contentFeatureGate,
		fileGitIgnore:                   contentGitIgnoreTmp,
		"internal/component/ssh/ssh.go": "package ssh\n",
		"internal/component/cli/cli.go": "package cli\n",
	}); err != nil {
		return err
	}
	head, err := headOf(ctx, repo)
	if err != nil {
		return err
	}

	// encode reaches a package this change set never names, ui and parse reach
	// the one it does, and every other gating suite is absent from the map.
	// An absent suite is UNKNOWN and always runs, so encode is the suite the
	// selection has to rule out.
	if err := writeSuiteMapFixture(repo, head, map[string][]string{
		suiteMapEncode: {"./internal/component/cli"},
		suiteMapUI:     {suiteMapSSHPackage},
		suiteMapParse:  {suiteMapSSHPackage},
	}); err != nil {
		return err
	}

	for _, scenario := range []suiteMapScenario{
		{
			name: "map-narrows-to-the-suites-that-reached-the-package", narrowed: true,
			packages: []string{suiteMapSSHPackage},
			runs:     []string{suiteMapUI, suiteMapParse, suiteMapWeb}, ruledOut: []string{suiteMapEncode},
		},
		{
			name:     "unknown-package-runs-every-suite-and-is-named",
			packages: []string{"./internal/component/nowhere"},
			runs:     []string{suiteMapUI, suiteMapParse, suiteMapEncode, suiteMapWeb},
			says:     "records no suite reaching ./internal/component/nowhere",
		},
		{
			name: "operator-skip-outranks-the-map", narrowed: true,
			packages: []string{suiteMapSSHPackage}, skipSuites: suiteMapParse,
			runs: []string{suiteMapUI}, ruledOut: []string{suiteMapEncode},
		},
	} {
		if err := checkSuiteMapScenario(ctx, le, repo, scenario); err != nil {
			return err
		}
	}

	// A commit after the recording moves the package out of the map's reach,
	// and the answer must widen and name it.
	if err := os.WriteFile(filepath.Join(repo, "internal", "component", "ssh", "ssh.go"),
		[]byte("package ssh\n\nfunc Listen() {}\n"), 0o600); err != nil {
		return err
	}
	if err := commitFixture(ctx, repo, "ssh moved under the map"); err != nil {
		return err
	}
	if err := checkSuiteMapScenario(ctx, le, repo, suiteMapScenario{
		name:     "a-commit-since-the-recording-makes-the-package-unknown",
		packages: []string{suiteMapSSHPackage},
		runs:     []string{suiteMapUI, suiteMapParse, suiteMapEncode},
		says:     "touched ./internal/component/ssh",
	}); err != nil {
		return err
	}

	// No map at all is the state of every fresh checkout and every CI shard.
	if err := os.Remove(filepath.Join(repo, suiteMapArtifact)); err != nil {
		return err
	}
	return checkSuiteMapScenario(ctx, le, repo, suiteMapScenario{
		name:     "an-absent-map-runs-every-suite",
		packages: []string{suiteMapSSHPackage},
		runs:     []string{suiteMapUI, suiteMapParse, suiteMapEncode, suiteMapWeb},
		says:     "records no suite map",
	})
}

// suiteMapArtifact is where the map lives inside a checkout. The fixture writes
// this path because it is the one the tool reads (suiteMapPath,
// internal/le/functional/suitemap.go).
const suiteMapArtifact = "tmp/ze-suite-map.json"

// checkSuiteMapScenario publishes one change set, asks the real `le functional
// select` what a gating run would start, and judges the answer.
func checkSuiteMapScenario(ctx context.Context, le, repo string, scenario suiteMapScenario) error {
	scopeFile := filepath.Join(repo, "tmp", "scope-packages.txt")
	if err := os.WriteFile(scopeFile, []byte(strings.Join(scenario.packages, "\n")+"\n"), 0o600); err != nil {
		return err
	}
	environment := suiteMapEnvironment(repo, scopeFile, scenario.skipSuites)

	out, _, code, err := rawCommandStreams(ctx, repo, environment, le, "functional", "select", "|", "json")
	if err != nil || code != 0 {
		return fmt.Errorf("%s: le functional select exit=%d: %w %s", scenario.name, code, err, out)
	}

	var answer struct {
		Reason   string   `json:"reason"`
		Narrowed bool     `json:"narrowed"`
		Running  []string `json:"running"`
		Skipped  []string `json:"skipped"`
		RuledOut []string `json:"ruled-out"`
	}
	if err := json.Unmarshal([]byte(out), &answer); err != nil {
		return fmt.Errorf("%s: the answer is not JSON: %w %s", scenario.name, err, out)
	}

	if answer.Narrowed != scenario.narrowed {
		return fmt.Errorf("%s: narrowed=%v, want %v: %s", scenario.name, answer.Narrowed, scenario.narrowed, answer.Reason)
	}
	for _, suite := range scenario.runs {
		if !slices.Contains(answer.Running, suite) {
			return fmt.Errorf("%s: the run list is %v, and it must hold %s", scenario.name, answer.Running, suite)
		}
	}
	for _, suite := range scenario.ruledOut {
		if !slices.Contains(answer.RuledOut, suite) {
			return fmt.Errorf("%s: the ruled-out suites are %v, and they must hold %s",
				scenario.name, answer.RuledOut, suite)
		}
		if slices.Contains(answer.Running, suite) {
			return fmt.Errorf("%s: %s both runs and is ruled out", scenario.name, suite)
		}
	}
	if scenario.says != "" && !strings.Contains(answer.Reason, scenario.says) {
		return fmt.Errorf("%s: the reason is %q, which does not say %q", scenario.name, answer.Reason, scenario.says)
	}
	if scenario.skipSuites != "" {
		if !slices.Contains(answer.Skipped, scenario.skipSuites) {
			return fmt.Errorf("%s: the operator skips are %v, want %s",
				scenario.name, answer.Skipped, scenario.skipSuites)
		}
		if slices.Contains(answer.Running, scenario.skipSuites) {
			return fmt.Errorf("%s: the map added back the suite the operator skipped", scenario.name)
		}
	}

	fmt.Fprintln(os.Stdout, scenario.name) //nolint:errcheck // progress output
	return nil
}

// suiteMapEnvironment answers the environment one scenario runs `le` under.
//
// Every variable the selection reads is REPLACED rather than appended. The
// harness runs inside a checkout whose session may already export any of them,
// and a second copy in the child's environment leaves which one wins to the C
// library. ZE_COVER decides it outright: a recording run runs every suite, so
// an inherited one would widen every scenario here.
func suiteMapEnvironment(repo, scopeFile, skip string) []string {
	replaced := []string{"ZE_VERIFY_SCOPE_PACKAGES", "ZE_VERIFY_SCOPE_TAGS", "ZE_SKIP_SUITES", "ZE_COVER"}
	inherited := envRootedAt(repo)
	kept := make([]string, 0, len(inherited)+2)
	for _, entry := range inherited {
		name, _, found := strings.Cut(entry, "=")
		if found && slices.ContainsFunc(replaced, func(key string) bool {
			return strings.EqualFold(strings.ReplaceAll(name, ".", "_"), key)
		}) {
			continue
		}
		kept = append(kept, entry)
	}
	return append(kept, "ZE_VERIFY_SCOPE_PACKAGES="+scopeFile, "ZE_SKIP_SUITES="+skip)
}

// writeSuiteMapFixture writes the map a whole recording run would have
// published at head.
func writeSuiteMapFixture(repo, head string, reached map[string][]string) error {
	body, err := json.Marshal(map[string]any{"head": head, "reached": reached})
	if err != nil {
		return err
	}
	path := filepath.Join(repo, suiteMapArtifact)
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	return os.WriteFile(path, body, 0o600)
}

// headOf answers the commit the scratch checkout sits on.
func headOf(ctx context.Context, repo string) (string, error) {
	out, code, err := rawCommand(ctx, repo, os.Environ(), "git", "rev-parse", "HEAD")
	if err != nil || code != 0 {
		return "", fmt.Errorf("git rev-parse exit=%d: %w %s", code, err, out)
	}
	return strings.TrimSpace(out), nil
}

// commitFixture commits everything in the scratch checkout under one message.
func commitFixture(ctx context.Context, repo, message string) error {
	for _, args := range [][]string{{argAdd, "-A"}, {argCommit, "-q", "-m", message}} {
		if out, code, err := rawCommand(ctx, repo, os.Environ(), "git", args...); err != nil || code != 0 {
			return fmt.Errorf("git %s exit=%d: %w %s", strings.Join(args, " "), code, err, out)
		}
	}
	return nil
}
