package interoplab

import (
	"context"
	"net/netip"
	"strings"
	"testing"
)

// settlingDaemon models the D-e race (2026-10-19): the docker client of a
// `docker run` is canceled after the daemon accepted the create, so the
// container appears only once the client is gone. Here it appears after the
// first `rm -f`, which therefore answers that it does not exist. stuck makes
// the appeared container refuse every removal, and inProgress makes that
// refusal docker's "already in progress" for a removal that never ends
// (D-f). silentAbsent is Docker 29.9
// (checked on 2026-10-19): `rm -f` of an absent name exits 0 with
// no output, and only a removal prints the name; otherwise it is the older
// "No such container" refusal. Not safe for concurrent use.
type settlingDaemon struct {
	silentAbsent bool
	present      bool
	appeared     bool
	stuck        bool
	inProgress   bool
	removals     int
}

func (s *settlingDaemon) run(command ProcessCommand) (ProcessResult, error) {
	argv := command.Arguments
	name := argv[len(argv)-1]
	switch {
	case argv[1] == "run":
		return ProcessResult{ExitCode: timeoutExitCode}, context.Canceled
	case argv[1] == "rm":
		s.removals++
		if s.present && s.stuck && s.inProgress {
			return ProcessResult{ExitCode: 1, Stderr: "Error response from daemon: removal of container " + name + " is already in progress"}, nil
		}
		if s.present && s.stuck {
			return ProcessResult{ExitCode: 1, Stderr: "Error response from daemon: cannot remove container \"/" + name + "\": could not kill: tried to kill container, but did not receive an exit event"}, nil
		}
		if s.present {
			s.present = false
			return ProcessResult{Stdout: name + "\n"}, nil
		}
		if !s.appeared {
			s.appeared = true
			s.present = true
		}
		if s.silentAbsent {
			return ProcessResult{}, nil
		}
		return ProcessResult{ExitCode: 1, Stderr: "Error response from daemon: No such container: " + name}, nil
	case argv[1] == dockerContainerObject && argv[2] == dockerInspectVerb:
		if s.present {
			return ProcessResult{Stdout: "abc\n"}, nil
		}
		return ProcessResult{ExitCode: 1, Stderr: "Error: No such object: " + name}, nil
	}
	return ProcessResult{}, nil
}

// unsettledStarts is every start path that issues a `docker run` this client
// later removes by name: a detached run, a lab peer's run, and a one-shot.
func unsettledStarts() map[string]func(context.Context, *Docker) (string, error) {
	return map[string]func(context.Context, *Docker) (string, error){
		"detached": func(ctx context.Context, d *Docker) (string, error) {
			const name = "ze-l2tp-evidence-1"
			err := d.RunDetached(ctx, DetachedContainer{Name: name, Image: "alpine:3", Command: []string{"sleep", "infinity"}})
			if !ContainerMayExist(err) {
				return name, err
			}
			return name, d.RemoveContainer(context.WithoutCancel(ctx), name)
		},
		"lab peer": func(ctx context.Context, d *Docker) (string, error) {
			const name = "ze-interop-frr-1"
			network := Network{Name: "ze-interop-net-1", IPv4: netip.MustParsePrefix("172.30.0.0/24")}
			err := d.runContainer(ctx, network, PeerConfig{Name: "frr", Container: name, Image: "frr:test", Host: 2})
			if !ContainerMayExist(err) {
				return name, err
			}
			return name, d.RemoveContainer(context.WithoutCancel(ctx), name)
		},
		"one-shot": func(ctx context.Context, d *Docker) (string, error) {
			_, err := d.RunOneShot(ctx, OneShotContainer{Image: "alpine:3", Timeout: dockerCommandTimeout, Command: []string{"true"}})
			return fixtureOneShotName(), removalOutcome(err)
		},
	}
}

// startDialect is one start path under one `rm -f` absent-answer dialect.
type startDialect struct {
	name         string
	silentAbsent bool
}

// dialectStarts is every start path under both absent-answer dialects.
func dialectStarts() map[startDialect]func(context.Context, *Docker) (string, error) {
	starts := make(map[startDialect]func(context.Context, *Docker) (string, error))
	for path, start := range unsettledStarts() {
		starts[startDialect{name: path + "/no-such-container", silentAbsent: false}] = start
		starts[startDialect{name: path + "/silent-exit-0", silentAbsent: true}] = start
	}
	return starts
}

// removalOutcome is the removal part of a one-shot's joined error: the run's
// own cancel is expected, so only an error naming the removal counts.
func removalOutcome(err error) error {
	if err == nil {
		return nil
	}
	if strings.Contains(err.Error(), "remove") {
		return err
	}
	return nil
}

// VALIDATES: G-2, D-e. A start whose docker client was canceled is removed
// even when the daemon creates its container after the first removal said it
// did not exist, on every start path (RunDetached, a lab peer, a one-shot).
// PREVENTS: a Ctrl-C during `docker run` leaving a Created container behind
// while the report says it was removed.
// METHOD: settlingDaemon answers the run with a cancel and makes the container
// appear only after the first `rm -f`; the test reads that it was removed.
func TestCanceledStartRemovesAContainerThatAppearsLate(t *testing.T) {
	for path, start := range dialectStarts() {
		t.Run(path.name, func(t *testing.T) {
			daemon := &settlingDaemon{silentAbsent: path.silentAbsent}
			docker := newDocker(&recordingRunner{run: daemon.run})
			name, err := start(t.Context(), docker)
			if err != nil {
				t.Fatalf("removal of %s failed: %v", name, err)
			}
			if !daemon.appeared {
				t.Fatal("the fixture never made the container appear: the removal ran no rm -f")
			}
			if daemon.present {
				t.Fatalf("%s appeared after the first rm -f answered it absent, and was left behind (%d removals)", name, daemon.removals)
			}
		})
	}
}

// VALIDATES: G-2, D-e. A container that appears after a canceled start and
// then cannot be removed is reported by name with the command that removes it.
// PREVENTS: a left-behind container hidden behind an early "No such container".
func TestCanceledStartNamesAnAppearedContainerItCannotRemove(t *testing.T) {
	for path, start := range dialectStarts() {
		t.Run(path.name, func(t *testing.T) {
			daemon := &settlingDaemon{stuck: true, silentAbsent: path.silentAbsent}
			docker := newDocker(&recordingRunner{run: daemon.run})
			name, err := start(t.Context(), docker)
			if err == nil {
				t.Fatalf("%s is still present and the removal answered success", name)
			}
			for _, want := range []string{name, cleanupCommand} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("removal error %q does not name %q", err, want)
				}
			}
		})
	}
}

// VALIDATES: D-f. A container that appears after a canceled start, and whose
// removal docker then says is already in progress and never ends, is reported
// once, by name, with the command that removes it.
// PREVENTS: the settle error wrapping the in-progress one, so the reader is
// told the same fix twice.
func TestCanceledStartWithARemovalStuckInProgressNamesTheCleanupOnce(t *testing.T) {
	daemon := &settlingDaemon{stuck: true, inProgress: true, silentAbsent: true}
	docker := newDocker(&recordingRunner{run: daemon.run})
	name, err := unsettledStarts()["one-shot"](t.Context(), docker)
	if err == nil {
		t.Fatalf("%s is still present and the removal answered success", name)
	}
	for _, want := range []string{name, "still present", cleanupCommand} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("removal error %q does not name %q", err, want)
		}
	}
	if fixes := strings.Count(err.Error(), "Fix:"); fixes != 1 {
		t.Errorf("removal error %q gives %d fixes, want 1", err, fixes)
	}
}

// VALIDATES: the settle wait is paid only after a failed start: removing a
// container whose start succeeded, and that is already gone, issues one rm -f.
// PREVENTS: every pre-clean and teardown waiting out the settle window.
func TestRemovalAfterASuccessfulStartDoesNotWait(t *testing.T) {
	removals := 0
	runner := &recordingRunner{run: func(command ProcessCommand) (ProcessResult, error) {
		if command.Arguments[1] == "rm" {
			removals++
			return ProcessResult{ExitCode: 1, Stderr: "Error response from daemon: No such container: ze-peer-1"}, nil
		}
		return ProcessResult{}, nil
	}}
	docker := newDocker(runner)
	if err := docker.RunDetached(t.Context(), DetachedContainer{Name: "ze-peer-1", Image: "alpine:3"}); err != nil {
		t.Fatal(err)
	}
	if err := docker.RemoveContainer(t.Context(), "ze-peer-1"); err != nil {
		t.Fatal(err)
	}
	if removals != 1 {
		t.Fatalf("removal after a successful start issued %d rm -f, want 1", removals)
	}
}
