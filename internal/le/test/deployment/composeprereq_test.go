package testdeployment

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// composeVersionAnswer answers `docker compose version --short` with result
// and fails every other command, so a test sees a step run past the check.
func composeVersionAnswer(t *testing.T, result interoplab.ProcessResult) *argvRecorder {
	t.Helper()
	return &argvRecorder{answer: func(argv []string) (interoplab.ProcessResult, error) {
		if strings.Join(argv, " ") != "docker compose version --short" {
			t.Errorf("ran %v past the Compose check", argv)
		}
		return result, nil
	}}
}

// VALIDATES: a docker CLI without the Compose plugin is refused up front with
// what is missing, why the proof needs it and the install command per host.
// PREVENTS: Docker's raw "unknown flag: --project-name" at build, down and logs.
func TestComposeRefusesAMissingPlugin(t *testing.T) {
	for goos, fix := range map[string]string{
		"darwin": "brew install docker-compose",
		"linux":  "docker-compose-plugin",
	} {
		recorder := composeVersionAnswer(t, interoplab.ProcessResult{ExitCode: 125, Stderr: "docker: unknown command: docker compose"})
		err := requireCompose(t.Context(), interoplab.NewFixtureDocker(recorder), goos)
		if err == nil {
			t.Fatalf("%s: a missing plugin was accepted", goos)
		}
		for _, want := range []string{"Docker Compose plugin is missing", "needs the Compose plugin", "Fix: ", fix} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("%s: refusal %q lacks %q", goos, err, want)
			}
		}
		if len(recorder.commands) != 1 {
			t.Errorf("%s: ran %d commands, want only the version check", goos, len(recorder.commands))
		}
	}
}

// VALIDATES: a plugin older than 2.24.4 is refused by name; 2.24.4 and newer,
// with or without a leading v or a suffix, pass.
func TestComposeRefusesAnOldPlugin(t *testing.T) {
	recorder := composeVersionAnswer(t, interoplab.ProcessResult{Stdout: "2.24.3\n"})
	err := requireCompose(t.Context(), interoplab.NewFixtureDocker(recorder), "linux")
	if err == nil || !strings.Contains(err.Error(), "Docker Compose plugin 2.24.3 is older than 2.24.4") {
		t.Errorf("2.24.3 answered %v", err)
	}
	for _, version := range []string{"2.24.4", "v2.29.1", "2.30.0-desktop.1", "3.0.0"} {
		recorder := composeVersionAnswer(t, interoplab.ProcessResult{Stdout: version + "\n"})
		if err := requireCompose(t.Context(), interoplab.NewFixtureDocker(recorder), "linux"); err != nil {
			t.Errorf("%s refused: %v", version, err)
		}
	}
	recorder = composeVersionAnswer(t, interoplab.ProcessResult{Stdout: "dev\n"})
	if err := requireCompose(t.Context(), interoplab.NewFixtureDocker(recorder), "linux"); err == nil {
		t.Error("an unreadable version was accepted")
	}
}
