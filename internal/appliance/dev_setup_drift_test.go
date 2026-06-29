package appliance

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

var applianceCheckKeyRE = regexp.MustCompile(`"(appliance-\w+)"`)

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
	scriptPath := filepath.Join(repoRoot, "scripts", "dev", "dev-setup.py")

	data, err := os.ReadFile(scriptPath)
	if err != nil {
		t.Fatalf("reading dev-setup.py: %v", err)
	}
	scriptContent := string(data)

	matches := applianceCheckKeyRE.FindAllStringSubmatch(scriptContent, -1)
	scriptKeys := make(map[string]bool)
	for _, m := range matches {
		scriptKeys[m[1]] = true
	}

	for _, name := range installableNames {
		if !scriptKeys[name] {
			t.Errorf("applianceDoctorChecks() has %q but APPLIANCE_CHECKS in dev-setup.py does not", name)
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
			t.Errorf("APPLIANCE_CHECKS in dev-setup.py has %q but applianceDoctorChecks() does not", key)
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
