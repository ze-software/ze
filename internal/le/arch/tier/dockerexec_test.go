package archtier

import (
	"strings"
	"testing"
)

// VALIDATES: AC-16: the tier check refuses an exec of docker outside
// internal/le/interoplab by a literal (a bare name or a path, through an
// aliased os/exec), by a constant of the file's own package declared in
// another file, and by the wrapper's exported constant, naming each file and
// line; the wrapper's own exec and an exec of another program pass.
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
		"interoplab.Docker", "labtest.Start",
	} {
		if !strings.Contains(diagnosis, want) {
			t.Errorf("diagnosis lacks %q:\n%s", want, diagnosis)
		}
	}
	if strings.Contains(diagnosis, dockerExecRoute) {
		t.Errorf("the wrapper's own exec was refused:\n%s", diagnosis)
	}
}
