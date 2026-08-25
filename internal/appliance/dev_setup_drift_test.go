package appliance

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

// The names as `le` spells them: `ApplianceCheck(name='appliance-xorriso', ...)`.
// Single quotes, because scripts/le is formatted with them (pyproject.toml,
// [tool.ruff.format] quote-style). A double-quoted spelling would match
// nothing and this test would pass vacuously, so the quote is part of the
// pattern rather than tolerated by it.
var applianceCheckKeyRE = regexp.MustCompile(`name='(appliance-\w+)'`)

func TestDevSetupMatchesDoctor(t *testing.T) {
	checks := applianceDoctorChecks()

	buildArtifactChecks := map[string]bool{
		"appliance-kernel": true,
		"appliance-initrd": true,
	}

	var installableNames []string
	for _, c := range checks {
		if buildArtifactChecks[c.Name] {
			continue
		}
		installableNames = append(installableNames, c.Name)
	}

	if len(installableNames) == 0 {
		t.Fatal("no installable checks found in applianceDoctorChecks()")
	}

	_, thisFile, _, _ := runtime.Caller(0)
	repoRoot := filepath.Join(filepath.Dir(thisFile), "..", "..")
	scriptPath := filepath.Join(repoRoot, "scripts", "le", "devtools", "tools.py")

	data, err := os.ReadFile(scriptPath)
	if err != nil {
		t.Fatalf("reading scripts/le/devtools/tools.py: %v", err)
	}
	scriptContent := string(data)

	matches := applianceCheckKeyRE.FindAllStringSubmatch(scriptContent, -1)
	// An empty parse is indistinguishable from "the table is empty", and both
	// would make every assertion below vacuous. The table has three entries, so
	// a format change is what would empty this.
	if len(matches) == 0 {
		t.Fatalf("no APPLIANCE_CHECKS names parsed from %s: did the table format change? "+
			"This test must not pass vacuously.", scriptPath)
	}
	scriptKeys := make(map[string]bool)
	for _, m := range matches {
		scriptKeys[m[1]] = true
	}

	for _, name := range installableNames {
		if !scriptKeys[name] {
			t.Errorf("applianceDoctorChecks() has %q but APPLIANCE_CHECKS in scripts/le/devtools/tools.py does not", name)
		}
	}

	for key := range scriptKeys {
		found := false
		for _, c := range checks {
			if c.Name == key {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("APPLIANCE_CHECKS in scripts/le/devtools/tools.py has %q but applianceDoctorChecks() does not", key)
		}
	}

	if t.Failed() {
		t.Log("installable doctor checks:", strings.Join(installableNames, ", "))
		var keys []string
		for k := range scriptKeys {
			keys = append(keys, k)
		}
		t.Log("script APPLIANCE_CHECKS keys:", strings.Join(keys, ", "))
	}
}
