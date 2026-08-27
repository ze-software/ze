// Design: docs/architecture/testing/interop.md -- fail-open command helpers
// Detail: dockerexec_python.go -- Python tokenization and call classification
//
// dockerexec.go finds Python test helpers that return docker_exec_quiet results.
// The empty result means that Docker failed. A caller MUST test a bound result
// for emptiness before it reads that result.
package functional

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/ze-software/ze/internal/core/textbuf"
)

const (
	dockerExecSeed         = "docker_exec_quiet"
	dockerExecBaselineRel  = "test/health/docker-exec-baseline.json"
	dockerExecBaselineKey  = "unchecked"
	dockerExecVerdictCheck = "checked"
	dockerExecVerdictDrop  = "discarded"
	dockerExecVerdictAllow = "exempt"
	dockerExecVerdictOpen  = "unchecked"
)

// DockerExecSite is one call of a function whose empty result means failure.
type DockerExecSite struct {
	File     string `json:"file"`
	Line     int    `json:"line"`
	Function string `json:"function"`
	Member   string `json:"member"`
	Verdict  string `json:"verdict"`
}

// DockerExecCounts groups the four mutually exclusive call-site verdicts.
type DockerExecCounts struct {
	Checked   int `json:"checked"`
	Discarded int `json:"discarded"`
	Exempt    int `json:"exempt"`
	Unchecked int `json:"unchecked"`
}

// DockerExecAnalysis is the analyzer result before the committed floor is read.
type DockerExecAnalysis struct {
	FailOpenFunctions []string         `json:"fail-open-functions"`
	Sites             []DockerExecSite `json:"sites"`
	FilesScanned      int              `json:"files-scanned"`
	Counts            DockerExecCounts `json:"counts"`
}

// DockerExecUncheckedSite is the site shape of the producer's JSON report.
type DockerExecUncheckedSite struct {
	File     string `json:"file"`
	Line     int    `json:"line"`
	Function string `json:"function"`
	Member   string `json:"member"`
}

// DockerExecReport is the structured answer of ze-functional-docker-exec-check.
type DockerExecReport struct {
	SchemaVersion     int                       `json:"schema-version"`
	Seed              string                    `json:"seed"`
	FailOpenFunctions []string                  `json:"fail-open-functions"`
	FilesScanned      int                       `json:"files-scanned"`
	Counts            DockerExecCounts          `json:"counts"`
	Baseline          int                       `json:"baseline"`
	UncheckedSites    []DockerExecUncheckedSite `json:"unchecked-sites"`
}

// Code reports whether the unchecked count exceeded the committed floor.
func (r DockerExecReport) Code() int {
	if r.Counts.Unchecked > r.Baseline {
		return 1
	}
	return 0
}

// Text returns the deterministic success text of the Python producer.
func (r DockerExecReport) Text() string {
	var out textbuf.Buffer
	out.Str("docker-exec-check: OK (").Int(int64(r.Counts.Unchecked)).
		Str(" unchecked <= floor ").Int(int64(r.Baseline)).Str("; ").
		Int(int64(len(r.FailOpenFunctions))).Str(" fail-open functions, ").
		Int(int64(r.siteCount())).Str(" call sites in ").Int(int64(r.FilesScanned)).Str(" files)\n")
	if r.Counts.Unchecked < r.Baseline {
		out.Str("  The count fell to ").Int(int64(r.Counts.Unchecked)).
			Str(": lower the baseline in ").Str(dockerExecBaselineRel).
			Str(" in this change to keep the floor tight.\n")
	}
	return out.String()
}

func (r DockerExecReport) siteCount() int {
	return r.Counts.Checked + r.Counts.Discarded + r.Counts.Exempt + r.Counts.Unchecked
}

// DockerExecSelftestReport is the structured answer of the fixture selftest.
type DockerExecSelftestReport struct {
	Status            string           `json:"status"`
	Seed              string           `json:"seed"`
	FailOpenFunctions []string         `json:"fail-open-functions"`
	Verdicts          []DockerExecSite `json:"verdicts"`
}

// Text returns the producer's selftest success line.
func (r DockerExecSelftestReport) Text() string {
	if r.Status == "OK" {
		return "docker-exec-check selftest: OK\n"
	}
	return ""
}

// CheckDockerExec scans root/test and applies the committed unchecked floor.
func CheckDockerExec(root string) (DockerExecReport, error) {
	analysis, err := AnalyzeDockerExec(root)
	if err != nil {
		return DockerExecReport{}, err
	}
	baseline, err := readDockerExecBaseline(root)
	if err != nil {
		return DockerExecReport{}, err
	}
	unchecked := make([]DockerExecUncheckedSite, 0, analysis.Counts.Unchecked)
	for _, site := range analysis.Sites {
		if site.Verdict != dockerExecVerdictOpen {
			continue
		}
		unchecked = append(unchecked, DockerExecUncheckedSite{
			File: site.File, Line: site.Line, Function: site.Function, Member: site.Member,
		})
	}
	return DockerExecReport{
		SchemaVersion: 1, Seed: dockerExecSeed, FailOpenFunctions: analysis.FailOpenFunctions,
		FilesScanned: analysis.FilesScanned, Counts: analysis.Counts, Baseline: baseline,
		UncheckedSites: unchecked,
	}, nil
}

// AnalyzeDockerExec parses the complete Python population and classifies each call.
func AnalyzeDockerExec(root string) (DockerExecAnalysis, error) {
	sources, err := readDockerExecSources(root)
	if err != nil {
		return DockerExecAnalysis{}, err
	}
	return analyzeDockerExecSources(sources)
}

func readDockerExecSources(root string) (map[string]string, error) {
	base := filepath.Join(root, "test")
	sourceRoot, err := os.OpenRoot(base)
	if err != nil {
		return nil, fmt.Errorf("%s: cannot be read: %w", dockerExecRel(root, base), err)
	}
	sources := make(map[string]string)
	walkErr := fs.WalkDir(sourceRoot.FS(), ".", func(path string, entry fs.DirEntry, err error) error {
		fullPath := filepath.Join(base, filepath.FromSlash(path))
		if err != nil {
			return fmt.Errorf("%s: cannot be read: %w", dockerExecRel(root, fullPath), err)
		}
		if path == "." {
			return nil
		}
		parts := strings.Split(path, "/")
		if entry.IsDir() {
			if skipDockerExecDir(parts) {
				return fs.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".py" {
			return nil
		}
		body, err := sourceRoot.ReadFile(path)
		if err != nil {
			return fmt.Errorf("%s: cannot be read: %w", dockerExecRel(root, fullPath), err)
		}
		if !utf8.Valid(body) {
			return fmt.Errorf("%s: cannot be read: invalid UTF-8", dockerExecRel(root, fullPath))
		}
		sources[dockerExecRel(root, fullPath)] = string(body)
		return nil
	})
	closeErr := sourceRoot.Close()
	if walkErr != nil {
		return nil, walkErr
	}
	if closeErr != nil {
		return nil, fmt.Errorf("%s: cannot be closed: %w", dockerExecRel(root, base), closeErr)
	}
	return sources, nil
}

func skipDockerExecDir(parts []string) bool {
	for _, part := range parts {
		switch part {
		case "tmp", ".claude", ".git", "__pycache__", "node_modules", "vendor":
			return true
		}
	}
	return len(parts) == 1 && parts[0] == "draft"
}

func dockerExecRel(root, path string) string {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return filepath.ToSlash(path)
	}
	return filepath.ToSlash(rel)
}

func readDockerExecBaseline(root string) (int, error) {
	checkoutRoot, err := os.OpenRoot(root)
	if err != nil {
		return 0, fmt.Errorf("open checkout root: %w", err)
	}
	body, readErr := checkoutRoot.ReadFile(filepath.FromSlash(dockerExecBaselineRel))
	closeErr := checkoutRoot.Close()
	if errors.Is(readErr, fs.ErrNotExist) {
		return 0, fmt.Errorf("%s does not exist. Restore it from git rather than letting this run mint today's count as the new floor", dockerExecBaselineRel)
	}
	if readErr != nil {
		return 0, fmt.Errorf("%s: %w", dockerExecBaselineRel, readErr)
	}
	if closeErr != nil {
		return 0, fmt.Errorf("close checkout root: %w", closeErr)
	}
	var document map[string]any
	if err := json.Unmarshal(body, &document); err != nil {
		return 0, fmt.Errorf("%s: %w", dockerExecBaselineRel, err)
	}
	value, exists := document[dockerExecBaselineKey]
	if !exists {
		return 0, fmt.Errorf("%s: no `%s` key; a missing key would silently disable the ratchet", dockerExecBaselineRel, dockerExecBaselineKey)
	}
	switch number := value.(type) {
	case float64:
		return int(number), nil
	case string:
		baseline, err := strconv.Atoi(number)
		if err == nil {
			return baseline, nil
		}
	case bool:
		if number {
			return 1, nil
		}
		return 0, nil
	}
	return 0, fmt.Errorf("%s: `%s` is not a number: int() argument has type %T", dockerExecBaselineRel, dockerExecBaselineKey, value)
}

const dockerExecSelftestSource = `
def docker_exec_quiet(container, cmd): return ""
def _vtysh_quiet(container, command): return docker_exec_quiet(container, ["vtysh", "-c", command])
def route(container, prefix):
    output = _vtysh_quiet(container, "show bgp %s json" % prefix)
    if not output.strip(): return None
    return output
def is_dis(container):
    out = _vtysh_quiet(container, "show isis interface detail")
    return "DIS" in out or "Designated" in out
def warm(container): _vtysh_quiet(container, "show version")
def dump(container): print(_vtysh_quiet(container, "show isis neighbor")[:500])  # fail-open-ok: diag
def unmarked(container): print(_vtysh_quiet(container, "show isis neighbor")[:500])
def bare_marker(container):
    # fail-open-ok:
    print(_vtysh_quiet(container, "show isis neighbor")[:500])
`

// SelftestDockerExec proves every verdict and the wrapper derivation on fixtures.
func SelftestDockerExec() (DockerExecSelftestReport, error) {
	analysis, err := analyzeDockerExecSources(map[string]string{"selftest.py": dockerExecSelftestSource})
	if err != nil {
		return DockerExecSelftestReport{}, err
	}
	wantMembers := []string{dockerExecSeed, "_vtysh_quiet"}
	sort.Strings(wantMembers)
	if !slices.Equal(analysis.FailOpenFunctions, wantMembers) {
		return DockerExecSelftestReport{}, fmt.Errorf("fail-open set is %v, want %v", analysis.FailOpenFunctions, wantMembers)
	}
	want := map[string]string{
		"_vtysh_quiet": dockerExecVerdictCheck, "route": dockerExecVerdictCheck,
		"is_dis": dockerExecVerdictOpen, "warm": dockerExecVerdictDrop,
		"dump": dockerExecVerdictAllow, "unmarked": dockerExecVerdictOpen,
		"bare_marker": dockerExecVerdictOpen,
	}
	got := make(map[string]string, len(analysis.Sites))
	for _, site := range analysis.Sites {
		got[site.Function] = site.Verdict
	}
	if len(got) != len(want) {
		return DockerExecSelftestReport{}, fmt.Errorf("verdicts are %v, want %v", got, want)
	}
	for function, verdict := range want {
		if got[function] != verdict {
			return DockerExecSelftestReport{}, fmt.Errorf("%s verdict is %s, want %s", function, got[function], verdict)
		}
	}
	return DockerExecSelftestReport{
		Status: "OK", Seed: dockerExecSeed,
		FailOpenFunctions: analysis.FailOpenFunctions, Verdicts: analysis.Sites,
	}, nil
}
