package sim_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestNoDirectTimeCalls verifies that production code in reactor and fsm
// packages uses sim.Clock instead of direct time package calls.
//
// VALIDATES: All time.Now/After/Sleep/AfterFunc/NewTimer calls replaced with sim.Clock.
// PREVENTS: Regression where direct time calls bypass simulation injection.
func TestNoDirectTimeCalls(t *testing.T) {
	root := findRepoRoot(t)

	dirs := []string{
		filepath.Join(root, "internal", "plugins", "bgp", "reactor"),
		filepath.Join(root, "internal", "plugins", "bgp", "fsm"),
	}

	forbidden := []string{
		"time.Now()",
		"time.After(",
		"time.Sleep(",
		"time.AfterFunc(",
		"time.NewTimer(",
	}

	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatalf("reading %s: %v", dir, err)
		}

		for _, entry := range entries {
			name := entry.Name()
			// Skip test files and non-Go files
			if strings.HasSuffix(name, "_test.go") || !strings.HasSuffix(name, ".go") {
				continue
			}

			path := filepath.Join(dir, name)
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("reading %s: %v", path, err)
			}
			content := string(data)

			for _, pattern := range forbidden {
				if strings.Contains(content, pattern) {
					t.Errorf("%s contains direct %s call — use sim.Clock instead", name, pattern)
				}
			}
		}
	}
}

// TestNoDirectNetCalls verifies that production code in reactor and listener
// uses sim.Dialer/ListenerFactory instead of direct net package calls.
//
// VALIDATES: All net.Listen/net.Dial calls replaced with sim interfaces.
// PREVENTS: Regression where direct net calls bypass simulation injection.
func TestNoDirectNetCalls(t *testing.T) {
	root := findRepoRoot(t)

	dir := filepath.Join(root, "internal", "plugins", "bgp", "reactor")

	forbidden := []string{
		"net.Listen(",
		"net.Dial(",
		"net.DialTimeout(",
		"net.Dialer{",
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading %s: %v", dir, err)
	}

	for _, entry := range entries {
		name := entry.Name()
		if strings.HasSuffix(name, "_test.go") || !strings.HasSuffix(name, ".go") {
			continue
		}

		path := filepath.Join(dir, name)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("reading %s: %v", path, err)
		}
		content := string(data)

		for _, pattern := range forbidden {
			if strings.Contains(content, pattern) {
				t.Errorf("%s contains direct %s call — use sim.Dialer/ListenerFactory instead", name, pattern)
			}
		}
	}
}

// findRepoRoot walks up from the working directory to find the go.mod file.
func findRepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find go.mod in any parent directory")
		}
		dir = parent
	}
}
