//go:build linux

// Design: docs/architecture/cli/plugin-modes.md -- explicit local purge respects package ownership.

package local

import (
	"os"
	"strings"
	"testing"
)

func explicitLocalPurgeCases() []nativePurgeCase {
	return []nativePurgeCase{
		{name: "explicit-shared-package-state-without-standalone", prefix: "/usr/local", marker: "/usr/share/ze/install-method", explicitLocal: true, purge: true, refused: true},
		{name: "explicit-shared-package-state-with-standalone", prefix: "/usr/local", marker: "/usr/share/ze/install-method", binaryPresent: true, explicitLocal: true, purge: true, refused: true},
		{name: "explicit-shared-package-state-relocated-package-bin", prefix: "/usr/local", marker: "/usr/share/ze/install-method", packageBinLink: true, explicitLocal: true, purge: true, refused: true},
		{name: "explicit-selected-package-prefix", prefix: "/usr/local", marker: "/usr/local/share/ze/install-method", binaryPresent: true, explicitLocal: true, purge: true, refused: true},
		{name: "explicit-standalone-purge", prefix: "/usr/local", binaryPresent: true, explicitLocal: true, purge: true},
		{name: "explicit-standalone-state-only-purge", prefix: "/usr/local", explicitLocal: true, purge: true},
		{name: "explicit-custom-prefix-independent-of-package", prefix: "/opt/ze", marker: "/usr/share/ze/install-method", binaryPresent: true, explicitLocal: true, purge: true},
		{name: "explicit-ordinary-removal-preserves-shared-state", prefix: "/usr/local", marker: "/usr/share/ze/install-method", binaryPresent: true, explicitLocal: true},
		{name: "explicit-dry-run-preserves-package-state", prefix: "/usr/local", marker: "/usr/share/ze/install-method", binaryPresent: true, explicitLocal: true, purge: true, dryRun: true},
	}
}

// TestExplicitLocalPurgeRespectsSharedPackageState uses the shared child-only
// filesystem fixture to exercise the explicit local command's flag parser.
func TestExplicitLocalPurgeRespectsSharedPackageState(t *testing.T) {
	nativePurgeRunCases(t, false, true)
}

// explicitLocalPurgeOutput MUST run only after TestNativePurgeWorker enters its
// private filesystem. The command writes directly to the process stderr.
func explicitLocalPurgeOutput(t *testing.T, args []string) (int, string) {
	t.Helper()
	const path = "/explicit-local-command-output"
	output, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	stderr := os.Stderr
	os.Stderr = output
	code := cmdUninstall(args)
	os.Stderr = stderr
	if err := output.Close(); err != nil {
		t.Fatal(err)
	}
	text, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return code, string(text)
}

func explicitLocalPurgeRefusal(t *testing.T, output, marker string) {
	t.Helper()
	if !strings.Contains(output, "package-managed installation marker "+marker+" exists") {
		t.Fatalf("explicit local removal did not report package ownership: %s", output)
	}
	if !strings.HasSuffix(strings.TrimSpace(output), "ze help uninstall") {
		t.Fatalf("explicit local refusal did not provide the uninstall help command: %s", output)
	}
}
