// Design: docs/architecture/testing/interop.md -- owned-child cleanup guards.

package interoplab

import (
	"slices"
	"strings"
	"testing"
)

// TestOwnedCleanupRechecksInspectedLabels checks the label boundary even when
// a fake daemon returns a row outside the requested run filter. A matching
// nonce alone must not bypass kind, client identity or live-owner protection.
func TestOwnedCleanupRechecksInspectedLabels(t *testing.T) {
	for _, row := range []struct {
		name   string
		start  string
		mutate func(map[string]string)
	}{
		{name: "foreign nonce despite matching daemon filter", mutate: func(labels map[string]string) {
			labels[labelLaunch] += "-foreign"
		}},
		{name: "same nonce live owner", start: "100"},
		{name: "same nonce foreign machine", mutate: func(labels map[string]string) {
			labels[labelMachineID] = otherMachineID
		}},
		{name: "same nonce wrong resource kind", mutate: func(labels map[string]string) {
			labels[labelKind] = resourceImageTag.labelValue()
		}},
	} {
		t.Run(row.name, func(t *testing.T) {
			labels := fixtureLabels(nil)
			labels[labelLaunch] = fixtureOwner().run
			if row.mutate != nil {
				row.mutate(labels)
			}
			runner := &recordingRunner{run: orphanDaemon(labels, func(command ProcessCommand) (ProcessResult, bool) {
				joined := strings.Join(command.Arguments, " ")
				if strings.HasPrefix(joined, "docker network ls ") {
					return ProcessResult{}, true
				}
				if strings.HasPrefix(joined, "docker image ls ") {
					return ProcessResult{}, true
				}
				return ProcessResult{}, false
			})}
			docker := newDocker(runner)
			docker.judge.lookup = lookupAnswering(row.start)
			report, err := docker.CleanupOwnedOrphans(t.Context())
			if err != nil {
				t.Fatalf("CleanupOwnedOrphans: %v", err)
			}
			if len(report.Skipped) != 1 {
				t.Errorf("skipped = %#v, want the retained resource and its reason", report.Skipped)
			}
			if len(report.Removed) != 0 {
				t.Errorf("removed = %#v, want none", report.Removed)
			}
			commands := recordedCommands(runner.commands)
			if strings.Contains(commands, "docker rm -f ") {
				t.Errorf("owned cleanup removed a protected resource:\n%s", commands)
			}
			if !strings.Contains(commands, "--filter label="+labelLaunch+"="+docker.owner.run) {
				t.Errorf("owned cleanup did not restrict its listing to this run:\n%s", commands)
			}
		})
	}
}

// TestOwnedCleanupRemovesAllOwnedKindsKeepsCache preserves actual non-dry
// cleanup for containers, networks and retained tags, in dependency order.
func TestOwnedCleanupRemovesAllOwnedKindsKeepsCache(t *testing.T) {
	labels := fixtureLabels(func(labels map[string]string) { labels[labelLaunch] = fixtureOwner().run })
	runner := &recordingRunner{run: orphanDaemon(labels, nil)}
	docker := newDocker(runner)
	report, err := docker.CleanupOwnedOrphans(t.Context())
	if err != nil {
		t.Fatalf("CleanupOwnedOrphans: %v", err)
	}
	if report.Dry {
		t.Error("owned cleanup only inspected resources")
	}
	if len(report.Removed) != 3 {
		t.Errorf("removed = %#v, want all three owned kinds", report.Removed)
	}
	commands := recordedCommands(runner.commands)
	assertOrder(t, commands,
		"docker rm -f "+fixtureContainerID,
		"docker network rm "+fixtureNetworkID,
		"docker image rm --no-prune ze-interop-retained:abc",
	)
	if strings.Contains(commands, "docker image rm --no-prune ze-interop-peer:latest") {
		t.Errorf("owned cleanup removed the shared cache tag:\n%s", commands)
	}
}

// TestChildEnvironmentReplacesEveryInheritedIdentity compares the complete
// resulting environment, so a duplicate alias cannot override the child's run
// identity and a similarly prefixed unrelated key is preserved.
func TestChildEnvironmentReplacesEveryInheritedIdentity(t *testing.T) {
	docker := newDocker(&recordingRunner{})
	got, err := docker.ChildEnvironment([]string{
		"PATH=/keep", "ze.lab.launch=dotted", "ZE_LAB_LAUNCH=upper", "Ze_Lab_Launch=mixed",
		"ZE.LAB.LAUNCH=upper-dotted", "ze_lab_launch=lower", "ZE_LAB_LAUNCH_CHILD=keep",
	})
	if err != nil {
		t.Fatalf("ChildEnvironment: %v", err)
	}
	want := []string{"PATH=/keep", "ZE_LAB_LAUNCH_CHILD=keep", runIdentityVariable + "=" + docker.owner.run}
	if !slices.Equal(got, want) {
		t.Errorf("child environment = %q, want %q", got, want)
	}
	docker.owner.launch = "root-launch-fixture"
	nested, err := docker.ChildEnvironment([]string{"PATH=/keep", "ZE_LAB_LAUNCH=wrong-parent"})
	if err != nil {
		t.Fatalf("nested ChildEnvironment: %v", err)
	}
	wantNested := []string{"PATH=/keep", runIdentityVariable + "=root-launch-fixture"}
	if !slices.Equal(nested, wantNested) {
		t.Errorf("nested child environment = %q, want %q", nested, wantNested)
	}
	if docker.owner.run != fixtureOwner().run {
		t.Errorf("inherited launch replaced the process run identity: %q", docker.owner.run)
	}
}

// TestMissingRunIdentityRefusesOwnedCleanup prevents an uninitialized client
// from converting a scoped cleanup request into the explicit global sweep.
func TestMissingRunIdentityRefusesOwnedCleanup(t *testing.T) {
	runner := &recordingRunner{}
	docker := &Docker{runner: runner}
	if _, err := docker.ChildEnvironment(nil); err == nil {
		t.Error("ChildEnvironment accepted an absent run identity")
	}
	if _, err := docker.CleanupOwnedOrphans(t.Context()); err == nil {
		t.Error("CleanupOwnedOrphans accepted an absent run identity")
	}
	if len(runner.commands) != 0 {
		t.Errorf("an uninitialized client's cleanup ran commands: %s", recordedCommands(runner.commands))
	}
}
