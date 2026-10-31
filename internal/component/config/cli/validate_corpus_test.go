//go:build ze_bgp

package cli

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/exabgp/migration"
)

// sweepSkippedDirs are the directories the sweep never enters: version
// control, vendored and cached module trees (a Go toolchain under
// gokrazy/modcache ships resolv.conf fixtures), session scratch, and test/draft,
// which every repo-wide gate skips.
var sweepSkippedDirs = map[string]bool{
	".git": true, "vendor": true, "modcache": true, "node_modules": true, "tmp": true, "draft": true,
}

// hostVerdictCodes are diagnostics that judge the machine running the sweep,
// not the config: the kernel capability gate answers about THIS host
// (runValidation), so a Linux-only config refused on a macOS workstation is
// not a config defect.
var hostVerdictCodes = map[string]bool{"config-kernel-capability": true}

// refusedExit matches a .ci expectation of a non-zero exit, the marker of a
// test that feeds Ze a config it must refuse.
var refusedExit = regexp.MustCompile(`(?m)(^expect=exit:code=[1-9]|:exit=[1-9])`)

// sweepCarrier is one config the sweep validates.
type sweepCarrier struct {
	source string // file path, the .ci block name, or "migrate <input>"
	body   string
	// refused is true when the test carrying the body states that Ze refuses it.
	refused bool
	// rfc8212 is true when the test carrying the body is about RFC 8212, so a
	// direction left without policy is the subject under test.
	rfc8212 bool
}

// TestRepositoryConfigsValidate runs `ze config validate`'s own path
// (runValidation) over every Ze config the repository uses or generates: each
// `.conf` file outside vendored and cached trees, each `.ci` tmpfs `.conf` and
// stdin=config body, and the `ze exabgp migrate` output of every ExaBGP config
// the repository carries. A file is a Ze config when its first keyword is a Ze
// config root, so FRR, BIRD, strongSwan and ExaBGP inputs are told apart by the
// schema, not by a list.
//
// VALIDATES: every Ze config validates with no error, and no EBGP direction
// is left closed by RFC 8212 unless the test carrying it is about RFC 8212.
// A `.ci` that expects a non-zero exit carries a config it must refuse, and is
// skipped by that expectation, not by a list.
// PREVENTS: a config no test loads going stale when validation tightens, as
// test/perf/configs/ze.conf did when RFC 8212 enforcement landed.
func TestRepositoryConfigsValidate(t *testing.T) {
	schema, err := config.YANGSchema()
	if err != nil {
		t.Fatalf("YANG schema: %v", err)
	}
	root := filepath.Join("..", "..", "..", "..")
	carriers := collectSweepCarriers(t, root)

	var zeConfigs, refused, migrated int
	var absentRoots []string
	for i := range carriers {
		c := &carriers[i]
		first := firstConfigKeyword(c.body)
		if schema.Get(first) == nil {
			if requirements, gated := registry.GatedConfigRootRequirements(first); gated {
				absentRoots = append(absentRoots, c.source+" (root "+first+" needs "+joinRequirements(requirements)+")")
				continue
			}
			if body, ok := migrateExaBGP(c.body); ok {
				migrated++
				checkSweepCarrier(t, sweepCarrier{source: "migrate " + c.source, body: body, rfc8212: c.rfc8212})
			}
			continue
		}
		zeConfigs++
		if c.refused {
			refused++
			continue
		}
		checkSweepCarrier(t, *c)
	}
	if len(absentRoots) > 0 {
		t.Errorf("%d configs use a root this test binary was built without; run the test under the feature tag set (./le test unit config):\n  %s",
			len(absentRoots), strings.Join(absentRoots, "\n  "))
	}
	// R-6: a sweep that finds nothing proves nothing.
	if zeConfigs == 0 {
		t.Fatal("the sweep found no Ze config")
	}
	if migrated == 0 {
		t.Fatal("the sweep migrated no ExaBGP config")
	}
	t.Logf("validated %d Ze configs (%d skipped as refused by their test) and %d migrated ExaBGP configs",
		zeConfigs-refused, refused, migrated)
}

// checkSweepCarrier validates one body and fails naming each error and each
// RFC 8212 absence the carrying test is not about.
func checkSweepCarrier(t *testing.T, c sweepCarrier) {
	t.Helper()
	result := runValidation(c.body, c.source)
	for i := range result.Diagnostics {
		d := &result.Diagnostics[i]
		switch {
		case hostVerdictCodes[d.Code]:
			continue
		case d.Severity == diagnostic.SeverityError:
			t.Errorf("%s: %s: %s", c.source, d.Code, lineMessage(d))
		case d.Code == rfc8212WarningCode && !c.rfc8212:
			t.Errorf("%s: %s: %s", c.source, d.Code, d.Message)
		}
	}
}

// lineMessage prefixes a diagnostic's message with its line when it has one.
func lineMessage(d *diagnostic.Diagnostic) string {
	if d.Line > 0 {
		return "line " + strconv.Itoa(d.Line) + ": " + d.Message
	}
	return d.Message
}

// migrateExaBGP answers `ze exabgp migrate`'s output for an ExaBGP config, by
// the three calls the command makes, and false for text that is not one.
func migrateExaBGP(body string) (string, bool) {
	tree, err := migration.ParseExaBGPConfig(body)
	if err != nil {
		return "", false
	}
	result, err := migration.MigrateFromExaBGP(tree)
	if err != nil {
		return "", false
	}
	return migration.SerializeTree(result.Tree), true
}

// joinRequirements renders gated root requirements as "ze_a+ze_b or ze_c".
func joinRequirements(requirements [][]string) string {
	alternatives := make([]string, 0, len(requirements))
	for _, tags := range requirements {
		alternatives = append(alternatives, strings.Join(tags, "+"))
	}
	return strings.Join(alternatives, " or ")
}

// collectSweepCarriers walks root for `.conf` files and the `.ci` bodies
// collectCorpusCarriers reads, marking each with what its carrying test says.
func collectSweepCarriers(t *testing.T, root string) []sweepCarrier {
	t.Helper()
	var carriers []sweepCarrier
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if sweepSkippedDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		switch filepath.Ext(path) {
		case ".conf":
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			carriers = append(carriers, sweepCarrier{source: path, body: string(data), rfc8212: directoryIsAboutRFC8212(path)})
		case ".ci":
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			text := string(data)
			for _, c := range collectCorpusCarriers(t, path) {
				carriers = append(carriers, sweepCarrier{
					source:  c.source,
					body:    expandRecordPorts(c.body),
					refused: refusedExit.MatchString(text),
					rfc8212: rfc8212Expectation.MatchString(text),
				})
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
	return carriers
}

// rfc8212Expectation matches a `.ci` expectation that names RFC 8212: the
// test asserts what RFC 8212 does to a direction left without policy.
var rfc8212Expectation = regexp.MustCompile(`(?mi)^expect=.*8212`)

// sweepRecordPort stands for the port the functional runner allocates a test.
const sweepRecordPort = 1790

// expandRecordPorts expands $PORT2, then $PORT, in a `.ci` body exactly as the
// functional runner does before a test uses its tmpfs (tmpfsForRecord,
// internal/test/runner/runner_exec.go): this is a copy of that rule, and that
// function is its source.
func expandRecordPorts(body string) string {
	body = strings.ReplaceAll(body, "$PORT2", strconv.Itoa(sweepRecordPort+1))
	return strings.ReplaceAll(body, "$PORT", strconv.Itoa(sweepRecordPort))
}

// directoryIsAboutRFC8212 reports whether the scenario holding a `.conf` is
// about RFC 8212 by its own name, as bgp-rfc8212-default-reject-frr is.
func directoryIsAboutRFC8212(path string) bool {
	return strings.Contains(strings.ToLower(filepath.Base(filepath.Dir(path))), "8212")
}

// firstConfigKeyword answers the first top-level keyword of a config body, in
// either the hierarchical or the set form, and "" for a body with none.
func firstConfigKeyword(body string) string {
	for raw := range strings.SplitSeq(body, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "//") {
			continue
		}
		fields := strings.FieldsFunc(line, func(r rune) bool { return r == ' ' || r == '\t' || r == '{' || r == ';' })
		if len(fields) == 0 {
			continue
		}
		if fields[0] == "set" && len(fields) > 1 {
			return fields[1]
		}
		return fields[0]
	}
	return ""
}
