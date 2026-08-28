// Package netlab checks the repository's netlab daemon integration.
package netlab

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

const tempPrefix = "ze-netlab-render-"

var errNoRenderedConfiguration = errors.New("netlab create rendered no ze configuration")

// Command is one child process invocation. Env is the complete inherited
// environment. An empty Dir means the caller's current directory.
type Command struct {
	Argv []string
	Env  []string
	Dir  string
}

// CommandResult is the complete captured result of one child process.
type CommandResult struct {
	Stdout string
	Stderr string
	Code   int
	Err    error
}

// ExecuteFunc runs one command. Tests replace it without starting netlab or ze.
type ExecuteFunc func(Command) CommandResult

// FileSystem is every filesystem operation a Check needs. Tests may replace an
// individual function while retaining the real implementation for the rest.
type FileSystem struct {
	Stat      func(string) (fs.FileInfo, error)
	ReadDir   func(string) ([]fs.DirEntry, error)
	ReadFile  func(string) ([]byte, error)
	WriteFile func(string, []byte, fs.FileMode) error
	MkdirAll  func(string, fs.FileMode) error
	MkdirTemp func(string, string) (string, error)
	RemoveAll func(string) error
	CopyFile  func(string, string) error
	CopyTree  func(string, string) error
}

// Report is the structured answer shared by the default prose renderer and the
// json, yaml and table pipe operators.
type Report struct {
	Netlab   string   `json:"netlab,omitempty" yaml:"netlab,omitempty"`
	Nodes    []string `json:"nodes,omitempty" yaml:"nodes,omitempty"`
	Failures []string `json:"failures,omitempty" yaml:"failures,omitempty"`
	Problems int      `json:"problems" yaml:"problems"`
	Updated  bool     `json:"updated" yaml:"updated"`
	Clean    bool     `json:"clean" yaml:"clean"`

	stdout strings.Builder
	stderr strings.Builder
}

// Text preserves the producer's stdout when no pipe operator is selected.
func (r *Report) Text() string { return r.stdout.String() }

// errorText preserves the producer's stderr separately from structured data.
func (r *Report) errorText() string { return r.stderr.String() }

// Checker owns one render and validation pass.
type Checker struct {
	Root       string
	Env        []string
	TempParent string
	FS         FileSystem
	Execute    ExecuteFunc
}

// newChecker returns the production checker for one repository root.
func newChecker(root string) *Checker {
	checker := &Checker{
		Root:    root,
		Env:     os.Environ(),
		FS:      osFileSystem(),
		Execute: execute,
	}
	// These closures read checker.FS at invocation time. Replacing ReadFile or
	// another primitive in a test therefore also changes copies performed by
	// CopyFile and CopyTree instead of leaving them bound to an older FS copy.
	checker.FS.CopyFile = func(source, target string) error {
		return copyFile(checker.FS, source, target)
	}
	checker.FS.CopyTree = func(source, target string) error {
		return copyTree(checker.FS, source, target)
	}
	return checker
}

// Run renders the topology, checks or updates its goldens, and validates every
// rendered node. It returns the producer's exit code.
func (c *Checker) Run(update bool) (report *Report, code int) {
	report = &Report{Updated: update}
	netlab, err := c.findNetlab()
	if err != nil {
		return c.fail(report, err.Error())
	}
	report.Netlab = netlab
	fmt.Fprintf(&report.stdout, "netlab: %s\n", netlab)

	temporary, err := c.FS.MkdirTemp(c.TempParent, tempPrefix)
	if err != nil {
		return c.fail(report, fmt.Sprintf("could not create temporary lab: %v", err))
	}
	removed := false
	defer func() {
		if removed {
			return
		}
		if cleanupErr := c.FS.RemoveAll(temporary); cleanupErr != nil {
			message := fmt.Sprintf("could not remove %s: %v", temporary, cleanupErr)
			report.Failures = append(report.Failures, message)
			fmt.Fprintf(&report.stderr, "error: %s\n", message)
			code = 1
		}
	}()

	lab := filepath.Join(temporary, "lab")
	if err := c.buildLab(lab); err != nil {
		return c.fail(report, err.Error())
	}
	if failed := c.runNetlabCreate(report, netlab, lab); failed {
		return report, 1
	}
	rendered, err := c.renderedConfigs(lab)
	if err != nil {
		if errors.Is(err, errNoRenderedConfiguration) {
			return c.fail(report, err.Error(),
				"contrib/netlab/ze.yml must map the `ze` daemon_config key to a real file.")
		}
		return c.fail(report, err.Error())
	}

	removed = true
	if err := c.FS.RemoveAll(temporary); err != nil {
		return c.fail(report, fmt.Sprintf("could not remove %s: %v", temporary, err))
	}

	problems, err := c.compare(report, rendered, update)
	if err != nil {
		return c.fail(report, err.Error())
	}

	ze, err := c.findZe()
	if err != nil {
		return c.fail(report, err.Error(),
			"Build a canonical daemon with `ZE_TEST_CANONICAL=1 ./le functional parse`, or set ZE_BIN=/path/to/ze.",
			"The render is only evidence if the daemon accepts what it produced.")
	}
	problems += c.validateGolden(report, ze, rendered)
	report.Problems = problems

	if problems != 0 {
		fmt.Fprintf(&report.stderr, "\n./le netlab render-check FAILED (%d problem(s))\n", problems)
		if !update {
			report.stderr.WriteString("If the render is right and the golden is stale, run `./le netlab render-update` and review the diff.\n")
		}
		return report, 1
	}
	report.Clean = true
	fmt.Fprintf(&report.stdout, "\n./le netlab render-check OK (%d node(s))\n", len(rendered))
	return report, 0
}

func (c *Checker) findNetlab() (string, error) {
	if override := envValue(c.Env, "NETLAB"); override != "" {
		if !c.executableFile(override) {
			return "", fmt.Errorf("NETLAB=%s is not an executable file", override)
		}
		return override, nil
	}
	found := c.findOnPath("netlab")
	if found == "" {
		return "", errors.New("netlab not found on PATH")
	}
	return found, nil
}

func (c *Checker) findZe() (string, error) {
	if override := envValue(c.Env, "ZE_BIN"); override != "" {
		if !c.regularFile(override) {
			return "", fmt.Errorf("ZE_BIN=%s does not exist", override)
		}
		return override, nil
	}
	candidate := filepath.Join(c.Root, "bin", "ze")
	if c.regularFile(candidate) {
		return candidate, nil
	}
	if found := c.findOnPath("ze"); found != "" {
		return found, nil
	}
	return "", errors.New("no ze binary to validate the golden files with")
}

func (c *Checker) findOnPath(name string) string {
	for _, directory := range filepath.SplitList(envValue(c.Env, "PATH")) {
		if directory == "" {
			directory = "."
		}
		candidate := filepath.Join(directory, name)
		if c.executableFile(candidate) {
			return candidate
		}
	}
	return ""
}

func (c *Checker) executableFile(path string) bool {
	info, err := c.FS.Stat(path)
	return err == nil && info.Mode().IsRegular() && info.Mode().Perm()&0o111 != 0
}

func (c *Checker) regularFile(path string) bool {
	info, err := c.FS.Stat(path)
	return err == nil && info.Mode().IsRegular()
}

func (c *Checker) buildLab(lab string) error {
	if err := c.FS.MkdirAll(filepath.Join(lab, "templates"), 0o777); err != nil {
		return fmt.Errorf("could not create netlab templates directory: %w", err)
	}
	source := filepath.Join(c.Root, "contrib", "netlab", "ze.yml")
	target := filepath.Join(lab, "topology-defaults.yml")
	if err := c.writeDaemonDefaults(source, target); err != nil {
		return fmt.Errorf("could not read %s: %w", source, err)
	}
	if err := c.FS.CopyTree(filepath.Join(c.Root, "contrib", "netlab", "ze"), filepath.Join(lab, "templates", "ze")); err != nil {
		return fmt.Errorf("could not copy netlab templates: %w", err)
	}
	if err := c.FS.CopyFile(filepath.Join(c.Root, "contrib", "netlab", "topology.yml"), filepath.Join(lab, "topology.yml")); err != nil {
		return fmt.Errorf("could not copy netlab topology: %w", err)
	}
	return nil
}

func (c *Checker) writeDaemonDefaults(source, target string) error {
	body, err := c.FS.ReadFile(source)
	if err != nil {
		return err
	}
	var daemon any
	if err := yaml.Unmarshal(body, &daemon); err != nil {
		return err
	}
	wrapped, err := yaml.Marshal(map[string]any{"daemons": map[string]any{"ze": daemon}})
	if err != nil {
		return err
	}
	return c.FS.WriteFile(target, wrapped, 0o666)
}

func (c *Checker) runNetlabCreate(report *Report, netlab, lab string) bool {
	result := c.Execute(Command{Argv: []string{netlab, "create"}, Env: c.Env, Dir: lab})
	if result.Code == 0 && result.Err == nil && !strings.Contains(result.Stdout, "Errors encountered") {
		return false
	}
	fmt.Fprintln(&report.stderr, result.Stdout)
	fmt.Fprintln(&report.stderr, result.Stderr)
	message := "`netlab create` failed on contrib/netlab/topology.yml"
	report.Failures = append(report.Failures, message)
	fmt.Fprintf(&report.stderr, "error: %s\n", message)
	report.stderr.WriteString("  Every module the topology declares needs a daemon_config key in\n")
	report.stderr.WriteString("  contrib/netlab/ze.yml and a template in contrib/netlab/ze/.\n")
	return true
}

func (c *Checker) renderedConfigs(lab string) (map[string]string, error) {
	nodeFiles := filepath.Join(lab, "node_files")
	info, err := c.FS.Stat(nodeFiles)
	if err != nil || !info.IsDir() {
		return nil, fmt.Errorf("netlab create wrote no %s", nodeFiles)
	}
	entries, err := c.FS.ReadDir(nodeFiles)
	if err != nil {
		return nil, fmt.Errorf("could not enumerate %s: %w", nodeFiles, err)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	rendered := make(map[string]string, len(entries))
	for _, entry := range entries {
		config := filepath.Join(nodeFiles, entry.Name(), "ze")
		if !c.regularFile(config) {
			continue
		}
		body, err := c.FS.ReadFile(config)
		if err != nil {
			return nil, fmt.Errorf("could not read %s: %w", config, err)
		}
		rendered[entry.Name()] = string(body)
	}
	if len(rendered) == 0 {
		return nil, errNoRenderedConfiguration
	}
	return rendered, nil
}

func (c *Checker) compare(report *Report, rendered map[string]string, update bool) (int, error) {
	golden := filepath.Join(c.Root, "contrib", "netlab", "golden")
	if err := c.FS.MkdirAll(golden, 0o777); err != nil {
		return 0, fmt.Errorf("could not create %s: %w", golden, err)
	}
	names := sortedNames(rendered)
	report.Nodes = append(report.Nodes, names...)
	problems := 0
	for _, name := range names {
		text := rendered[name]
		path := filepath.Join(golden, name+".conf")
		relative := filepath.Join("contrib", "netlab", "golden", name+".conf")
		if update {
			if err := c.FS.WriteFile(path, []byte(text), 0o666); err != nil {
				return problems, fmt.Errorf("could not update %s: %w", path, err)
			}
			fmt.Fprintf(&report.stdout, "updated %s\n", relative)
			continue
		}
		want, err := c.FS.ReadFile(path)
		if err != nil {
			if !errors.Is(err, fs.ErrNotExist) {
				return problems, fmt.Errorf("could not read %s: %w", path, err)
			}
			problems++
			message := fmt.Sprintf("no golden file for node %s at %s", name, path)
			report.Failures = append(report.Failures, message)
			fmt.Fprintf(&report.stderr, "FAIL: %s\n%s\n", message, text)
			continue
		}
		if string(want) != text {
			problems++
			message := fmt.Sprintf("%s does not match the render", relative)
			report.Failures = append(report.Failures, message)
			fmt.Fprintf(&report.stderr, "FAIL: %s\n", message)
			report.stderr.WriteString(unifiedDiffText(linesKeepingEnds(string(want)), linesKeepingEnds(text), "golden/"+name+".conf", "rendered/"+name))
			continue
		}
		fmt.Fprintf(&report.stdout, "ok: %s matches the render\n", relative)
	}

	entries, err := c.FS.ReadDir(golden)
	if err != nil {
		return problems, fmt.Errorf("could not enumerate %s: %w", golden, err)
	}
	var stale []string
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".conf" {
			continue
		}
		name := strings.TrimSuffix(entry.Name(), ".conf")
		if _, ok := rendered[name]; !ok {
			stale = append(stale, name)
		}
	}
	sort.Strings(stale)
	for _, name := range stale {
		problems++
		message := fmt.Sprintf("golden/%s.conf has no node in contrib/netlab/topology.yml", name)
		report.Failures = append(report.Failures, message)
		fmt.Fprintf(&report.stderr, "FAIL: %s\n", message)
	}
	return problems, nil
}

func (c *Checker) validateGolden(report *Report, ze string, rendered map[string]string) int {
	problems := 0
	for _, name := range sortedNames(rendered) {
		path := filepath.Join(c.Root, "contrib", "netlab", "golden", name+".conf")
		result := c.Execute(Command{Argv: []string{ze, "config", "validate", path}, Env: c.Env})
		if result.Code != 0 || result.Err != nil {
			problems++
			message := fmt.Sprintf("%s is not valid ze configuration", path)
			report.Failures = append(report.Failures, message)
			fmt.Fprintf(&report.stderr, "FAIL: %s\n%s\n", message, result.Stdout+result.Stderr)
			continue
		}
		relative := filepath.Join("contrib", "netlab", "golden", name+".conf")
		fmt.Fprintf(&report.stdout, "ok: %s validates\n", relative)
	}
	return problems
}

func (c *Checker) fail(report *Report, message string, hints ...string) (*Report, int) {
	report.Failures = append(report.Failures, message)
	fmt.Fprintf(&report.stderr, "error: %s\n", message)
	if message == "netlab not found on PATH" {
		hints = []string{
			"This check renders the contrib/netlab templates with a real netlab;",
			"it cannot verify them without one, and it will not pass silently.",
			"Install networklab from https://netlab.tools/install/",
			"Or point at an existing install: NETLAB=/path/to/netlab ./le netlab render-check",
		}
	}
	for _, hint := range hints {
		fmt.Fprintf(&report.stderr, "  %s\n", hint)
	}
	return report, 1
}

func sortedNames(rendered map[string]string) []string {
	names := make([]string, 0, len(rendered))
	for name := range rendered {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func envValue(environment []string, key string) string {
	prefix := key + "="
	for i := len(environment) - 1; i >= 0; i-- {
		if strings.HasPrefix(environment[i], prefix) {
			return strings.TrimPrefix(environment[i], prefix)
		}
	}
	return ""
}

func execute(spec Command) CommandResult {
	if len(spec.Argv) == 0 {
		return CommandResult{Code: -1, Err: errors.New("empty command")}
	}
	command := exec.Command(spec.Argv[0], spec.Argv[1:]...) //nolint:gosec // argv comes from repository paths and fixed keywords
	command.Dir = spec.Dir
	command.Env = append([]string(nil), spec.Env...)
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	err := command.Run()
	result := CommandResult{Stdout: stdout.String(), Stderr: stderr.String(), Err: err}
	if err == nil {
		return result
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		result.Code = exitErr.ExitCode()
		result.Err = nil
		return result
	}
	result.Code = -1
	return result
}

func osFileSystem() FileSystem {
	return FileSystem{
		Stat:      os.Stat,
		ReadDir:   os.ReadDir,
		ReadFile:  os.ReadFile,
		WriteFile: os.WriteFile,
		MkdirAll:  os.MkdirAll,
		MkdirTemp: os.MkdirTemp,
		RemoveAll: os.RemoveAll,
	}
}

func copyFile(filesystem FileSystem, source, target string) error {
	body, err := filesystem.ReadFile(source)
	if err != nil {
		return err
	}
	info, err := filesystem.Stat(source)
	if err != nil {
		return err
	}
	return filesystem.WriteFile(target, body, info.Mode().Perm())
}

func copyTree(filesystem FileSystem, source, target string) error {
	info, err := filesystem.Stat(source)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("%s is not a directory", source)
	}
	if err := filesystem.MkdirAll(target, info.Mode().Perm()); err != nil {
		return err
	}
	entries, err := filesystem.ReadDir(source)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		from := filepath.Join(source, entry.Name())
		to := filepath.Join(target, entry.Name())
		if entry.IsDir() {
			if err := copyTree(filesystem, from, to); err != nil {
				return err
			}
			continue
		}
		if err := copyFile(filesystem, from, to); err != nil {
			return err
		}
	}
	return nil
}
