package dockerhost

import (
	"maps"
	"os"
	"strings"
	"testing"
)

// environMap answers the process environment as a name to value map.
func environMap() map[string]string {
	environ := make(map[string]string)
	for _, entry := range os.Environ() {
		name, value, _ := strings.Cut(entry, "=")
		environ[name] = value
	}
	return environ
}

// TestDockerHostGokLeavesTheEnvironmentAsFound proves the image build gives
// back the process environment it was handed. `./le test qemu docker-lab`
// builds the image in its own process and then spawns the host le with
// os.Environ(), so a GOOS=linux left behind by gok made that le's launcher
// build a linux binary on macOS. Method: fix a baseline with t.Setenv, run
// runGokWith with a runner that changes, adds and removes variables the way
// gok does, once succeeding and once failing, and compare the environment
// afterwards with the baseline.
func TestDockerHostGokLeavesTheEnvironmentAsFound(t *testing.T) {
	for _, exit := range []int{0, 1} {
		t.Setenv("GOOS", "darwin")
		t.Setenv("GOARCH", "arm64")
		t.Setenv("CGO_ENABLED", "1")
		t.Setenv("GOFLAGS", "-mod=vendor")
		t.Setenv("ZE_TEST_GOK_REMOVED", "kept")
		t.Setenv("ZE_TEST_GOK_ADDED", "")
		if err := os.Unsetenv("ZE_TEST_GOK_ADDED"); err != nil {
			t.Fatal(err)
		}
		before := environMap()

		run := func([]string) int {
			for name, value := range map[string]string{
				"GOOS":              "linux",
				"CGO_ENABLED":       "0",
				"GOFLAGS":           "-mod=mod",
				"ZE_TEST_GOK_ADDED": "set by gok",
			} {
				if err := os.Setenv(name, value); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.Unsetenv("ZE_TEST_GOK_REMOVED"); err != nil {
				t.Fatal(err)
			}
			return exit
		}
		err := runGokWith(run, "amd64", nil)
		if (err != nil) != (exit != 0) {
			t.Fatalf("gok exit %d: runGokWith answered %v", exit, err)
		}

		after := environMap()
		if maps.Equal(before, after) {
			continue
		}
		for name, value := range after {
			if old, ok := before[name]; !ok || old != value {
				t.Errorf("gok exit %d: %s=%q after the build, want %q (present before: %v)", exit, name, value, old, ok)
			}
		}
		for name, value := range before {
			if _, ok := after[name]; !ok {
				t.Errorf("gok exit %d: %s=%q was removed by the build", exit, name, value)
			}
		}
	}
}
