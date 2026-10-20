package archtier

import (
	"strings"
	"testing"
)

// VALIDATES: AC-16: the tier check refuses an exec of docker outside
// internal/le/interoplab by a literal (a bare name or a path, through an
// aliased os/exec), by a constant of the file's own package declared in
// another file, by the wrapper's exported constant, by a constant defined from
// another constant (local or the wrapper's), through a package whose name
// differs from its directory, by a package-level var holding docker, by a
// function-scoped const, `:=`, `=` or var (chained through a local to the
// wrapper's constant too), and by a const repeating a docker value implicitly,
// naming each file and line; the wrapper's own exec, an exec of another
// program, and a local shadowing a package docker constant pass.
// PREVENTS: a test or tool starting containers the cleanup cannot find,
// because they carry no owner labels.
func TestNoDockerExecOutsideInteroplab(t *testing.T) {
	clean, err := checkFixture("docker-exec-clean", dockerExecFixture(false))
	if err != nil {
		t.Fatal(err)
	}
	if clean.Failed != 0 {
		t.Fatalf("the clean fixture failed: %s", clean.Diagnosis())
	}
	bad, err := checkFixture("docker-exec-bad", dockerExecFixture(true))
	if err != nil {
		t.Fatal(err)
	}
	if bad.Failed != 2 {
		t.Fatalf("the violating fixture answered %d, want 2: %s", bad.Failed, bad.Diagnosis())
	}
	diagnosis := bad.Diagnosis()
	for _, want := range []string{
		dockerExecViolators[0] + ":6 runs \"/usr/bin/docker\"",
		dockerExecViolators[1] + ":3 runs binary",
		dockerExecViolators[2] + ":6 runs DockerExecutable",
		dockerExecViolators[3] + ":3 runs chainedBinary",
		dockerExecViolators[4] + ":7 runs viaWrapper",
		dockerExecViolators[5] + ":6 runs Path",
		dockerExecViolators[6] + ":3 holds the docker program in var dockerBin",
		dockerExecViolators[6] + ":4 holds the docker program in var wrapperBin",
		dockerExecViolators[7] + ":5 runs program",
		dockerExecViolators[8] + ":5 runs program",
		dockerExecViolators[8] + ":10 runs tool",
		dockerExecViolators[9] + ":9 runs program",
		dockerExecViolators[10] + ":3 runs repeatedBin",
		"interoplab.Docker", "labtest.Start",
	} {
		if !strings.Contains(diagnosis, want) {
			t.Errorf("diagnosis lacks %q:\n%s", want, diagnosis)
		}
	}
	if strings.Contains(diagnosis, "filterapi/ip.go") {
		t.Errorf("a local shadowing a package docker constant was refused:\n%s", diagnosis)
	}
	if strings.Contains(diagnosis, dockerExecRoute) {
		t.Errorf("the wrapper's own exec was refused:\n%s", diagnosis)
	}
}

// VALIDATES: the docker route gate does not read the top-level build cache,
// where le keeps a patched GOROOT whose standard library generators exec
// docker, and it still reads a source package named cache deeper in the tree.
// PREVENTS: the tier check failing on a machine that has run a build and
// passing on one that has not, over code that is not Ze's.
func TestDockerExecGateSkipsTheBuildCache(t *testing.T) {
	const stdlibGenerator = "cache/stdpatch/0123/goroot/src/crypto/internal/gen/generate.go"
	const nestedCache = "internal/component/bgp/cache/run.go"
	fixture := dockerExecFixture(false)
	fixture[stdlibGenerator] = "package main\nimport \"os/exec\"\n" +
		"func main() { _ = exec.Command(\"docker\", \"run\") }\n"
	report, err := checkFixture("docker-exec-cache", fixture)
	if err != nil {
		t.Fatal(err)
	}
	if report.Failed != 0 {
		t.Fatalf("an exec inside the build cache was refused: %s", report.Diagnosis())
	}

	fixture[nestedCache] = "package cache\nimport \"os/exec\"\n" +
		"func run() { _ = exec.Command(\"docker\", \"ps\") }\n"
	report, err = checkFixture("docker-exec-nested-cache", fixture)
	if err != nil {
		t.Fatal(err)
	}
	diagnosis := report.Diagnosis()
	if !strings.Contains(diagnosis, nestedCache+":3 runs \"docker\"") {
		t.Errorf("a source package named cache was not scanned:\n%s", diagnosis)
	}
	if strings.Contains(diagnosis, stdlibGenerator) {
		t.Errorf("the build cache was scanned:\n%s", diagnosis)
	}
}
