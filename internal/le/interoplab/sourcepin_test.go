package interoplab

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// sourcePinNow is the clock every case judges against.
var sourcePinNow = time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)

// writeDockerfile writes body to a Dockerfile in a temporary directory and
// answers its path.
func writeDockerfile(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "Dockerfile.peer")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

const pinnedDockerfile = `FROM alpine:3.24.3 AS build
ARG ZE_SOURCE_REPOSITORY=MarcJHuber/event-driven-servers
ARG ZE_SOURCE_COMMIT=ed3c66a89d5cb71db04d514a330081fbb2595139
ARG ZE_SOURCE_DATE=@DATE@
RUN curl https://codeload.github.com/${ZE_SOURCE_REPOSITORY}/tar.gz/${ZE_SOURCE_COMMIT}
`

// VALIDATES: a pinned commit less than a month old builds, and one more than a
// month old is refused before docker build runs, naming the pin, its age and
// the bump command.
// PREVENTS: a source-built peer built from source older than M14 allows.
func TestBuildRefusesAStaleSourcePin(t *testing.T) {
	cases := []struct {
		name  string
		date  string
		stale bool
	}{
		{"three days old", "2026-10-19T14:39:08Z", false},
		{"exactly one month old", "2026-09-22T12:00:00Z", false},
		{"one month and a second", "2026-09-22T11:59:59Z", true},
		{"two months old", "2026-08-01T00:00:00Z", true},
	}
	for _, one := range cases {
		t.Run(one.name, func(t *testing.T) {
			path := writeDockerfile(t, strings.ReplaceAll(pinnedDockerfile, "@DATE@", one.date))
			runner := &recordingRunner{}
			docker := newDocker(runner)
			err := docker.RequireFreshSource(context.Background(), ImageBuild{Dockerfile: path}, sourcePinNow)
			if !one.stale {
				if err != nil {
					t.Fatalf("a fresh pin was refused: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("a stale pin was not refused")
			}
			for _, want := range []string{path, "ed3c66a89d5c", "MarcJHuber/event-driven-servers", "days ago", "M14", "Fix: ./le lab pin bump dockerfile " + path} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("refusal %q does not name %q", err, want)
				}
			}
			if len(runner.commands) != 0 {
				t.Errorf("a refused pin still ran %d commands", len(runner.commands))
			}
		})
	}
}

// VALIDATES: Build itself refuses the stale pin, so no `docker build` runs.
// PREVENTS: a caller of Build reaching Docker around the rule.
func TestBuildRunsNoDockerForAStalePin(t *testing.T) {
	path := writeDockerfile(t, strings.ReplaceAll(pinnedDockerfile, "@DATE@", "2020-01-01T00:00:00Z"))
	runner := &recordingRunner{}
	docker := newDocker(runner)
	_, err := docker.Build(context.Background(), ImageBuild{Tag: "ze-test:peer", Dockerfile: path, Context: filepath.Dir(path)})
	if err == nil || !strings.Contains(err.Error(), "lab pin bump") {
		t.Fatalf("Build answered %v, want the stale-pin refusal", err)
	}
	if len(runner.commands) != 0 {
		t.Fatalf("Build ran %v before refusing", runner.commands[0].Arguments)
	}
}

// VALIDATES: a half-declared pin is refused, and a Dockerfile with no source
// declaration is not judged at all.
// PREVENTS: a pin missing its date building with no freshness check.
func TestParseSourcePinDeclarations(t *testing.T) {
	if _, declared, err := ParseSourcePin("Dockerfile", "FROM alpine:3.24.3\nRUN true\n"); declared || err != nil {
		t.Errorf("a plain Dockerfile answered declared=%v err=%v", declared, err)
	}
	noDate := "ARG ZE_SOURCE_REPOSITORY=mc36/freeRtr\nARG ZE_SOURCE_COMMIT=96368b02da1030148d44fc1821e844deeba0a2f0\n"
	if _, _, err := ParseSourcePin("Dockerfile", noDate); err == nil || !strings.Contains(err.Error(), "ZE_SOURCE_DATE") {
		t.Errorf("a pin with no date answered %v", err)
	}
	if _, _, err := ParseSourcePin("Dockerfile", "ARG ZE_SOURCE_REPOSITORY=mc36/freeRtr\n"); err == nil {
		t.Error("a repository with neither pin nor checkout was accepted")
	}
	badDate := noDate + "ARG ZE_SOURCE_DATE=yesterday\n"
	if _, _, err := ParseSourcePin("Dockerfile", badDate); err == nil {
		t.Error("a date that is not RFC 3339 was accepted")
	}
}

const checkoutDockerfile = "ARG ZE_SOURCE_REPOSITORY=mc36/freeRtr\nARG ZE_SOURCE_CHECKOUT=freertr\nFROM scratch\n"

// checkoutRunner answers git for a checkout whose HEAD date and status the
// case chooses, and records every command.
func checkoutRunner(head, status string) *recordingRunner {
	return &recordingRunner{run: func(command ProcessCommand) (ProcessResult, error) {
		switch command.Arguments[3] {
		case "log":
			return ProcessResult{Stdout: head + "\n"}, nil
		case "status":
			return ProcessResult{Stdout: status}, nil
		}
		return ProcessResult{}, nil
	}}
}

// VALIDATES: a followed checkout whose HEAD is fresh is left alone; a stale
// clean one is fetched and fast-forwarded; a stale dirty one is refused with
// no fetch and no merge.
// PREVENTS: the perf DUT built from a checkout months behind upstream, or an
// operator's uncommitted work in the checkout moved under them.
func TestBuildFreshensAFollowedCheckout(t *testing.T) {
	path := writeDockerfile(t, checkoutDockerfile)
	build := ImageBuild{Dockerfile: path, NamedContexts: []NamedBuildContext{{Name: "freertr", Path: "/src/rtr"}}}

	fresh := checkoutRunner("2026-10-20T09:00:00+02:00", "")
	if err := newDocker(fresh).RequireFreshSource(context.Background(), build, sourcePinNow); err != nil {
		t.Fatalf("a fresh checkout was refused: %v", err)
	}
	if len(fresh.commands) != 1 {
		t.Errorf("a fresh checkout ran %d git commands, want only the HEAD date", len(fresh.commands))
	}

	clean := checkoutRunner("2026-07-01T09:00:00Z", "")
	if err := newDocker(clean).RequireFreshSource(context.Background(), build, sourcePinNow); err != nil {
		t.Fatalf("a stale clean checkout was refused: %v", err)
	}
	var verbs []string
	for _, command := range clean.commands {
		if command.Arguments[1] != "-C" || command.Arguments[2] != "/src/rtr" {
			t.Errorf("git ran outside the checkout: %v", command.Arguments)
		}
		verbs = append(verbs, command.Arguments[3])
	}
	if strings.Join(verbs, " ") != "log status fetch merge" {
		t.Errorf("a stale clean checkout ran %v, want log status fetch merge", verbs)
	}

	dirty := checkoutRunner("2026-07-01T09:00:00Z", " M src/org/freertr/rtr/rtrBgp.java\n")
	err := newDocker(dirty).RequireFreshSource(context.Background(), build, sourcePinNow)
	if err == nil || !strings.Contains(err.Error(), "uncommitted changes") || !strings.Contains(err.Error(), "git -C /src/rtr pull --ff-only") {
		t.Fatalf("a stale dirty checkout answered %v, want a refusal naming the fix", err)
	}
	if len(dirty.commands) != 2 {
		t.Errorf("a dirty checkout ran %d git commands, want log and status only", len(dirty.commands))
	}

	unpassed := ImageBuild{Dockerfile: path}
	if err := newDocker(checkoutRunner("", "")).RequireFreshSource(context.Background(), unpassed, sourcePinNow); err == nil {
		t.Error("a build that does not pass the followed context was accepted")
	}
}
