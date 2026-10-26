package interoplab

import (
	"context"
	"errors"
	"maps"
	"net/netip"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/le/procscan"
)

const (
	fixtureContainerID = "c000000000000000000000000000000000000000000000000000000000000001"
	fixtureNetworkID   = "a000000000000000000000000000000000000000000000000000000000000002"
	fixtureImageID     = "sha256:b000000000000000000000000000000000000000000000000000000000000003"
	otherMachineID     = "fedcba9876543210fedcba9876543210"
	otherBootID        = "76543210-89ab-cdef-0123-456789abcdef"
)

// fixtureLabels answers the label set of a resource owned by pid 4242 on the
// fixture machine, in the fixture boot, with start token "100".
func fixtureLabels(mutate func(map[string]string)) map[string]string {
	labels := map[string]string{
		labelMarker:       labelMarkerValue,
		labelRun:          "run-fixture",
		labelPID:          "4242",
		labelPIDStart:     "100",
		labelBootID:       fixtureBootID,
		labelMachineID:    fixtureMachineID,
		labelStarted:      "2026-10-18T10:00:00Z",
		labelPurpose:      "le test integration interop",
		labelKind:         "container",
		labelUID:          "1000",
		labelOS:           hostGOOSLinux,
		labelPIDNamespace: fixturePIDNamespace,
	}
	if mutate != nil {
		mutate(labels)
	}
	return labels
}

func labelJSON(labels map[string]string) string {
	keys := make([]string, 0, len(labels))
	for key := range labels {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	var text strings.Builder
	text.WriteByte('{')
	for index, key := range keys {
		if index > 0 {
			text.WriteByte(',')
		}
		text.WriteString(`"` + key + `":"` + labels[key] + `"`)
	}
	text.WriteByte('}')
	return text.String()
}

// lookupAnswering answers pid 4242 with the given start token; every other pid is absent.
func lookupAnswering(start string) func(int) (procscan.Process, bool, error) {
	return func(pid int) (procscan.Process, bool, error) {
		if pid == 4242 && start != "" {
			return procscan.Process{PID: pid, Start: start, Argv: []string{"le", "test", "integration", "interop"}}, true, nil
		}
		return procscan.Process{}, false, nil
	}
}

// orphanDaemon answers the labeled listings with one container, one network
// and one image carrying the cache tag and the run-owned tag, each owned by
// the labels given.
func orphanDaemon(labels map[string]string, extra func(ProcessCommand) (ProcessResult, bool)) func(ProcessCommand) (ProcessResult, error) {
	return func(command ProcessCommand) (ProcessResult, error) {
		if extra != nil {
			if result, handled := extra(command); handled {
				return result, nil
			}
		}
		joined := strings.Join(command.Arguments, " ")
		switch {
		case strings.HasPrefix(joined, "docker ps -a"):
			return ProcessResult{Stdout: fixtureContainerID + "\tlab-peer-4242\n"}, nil
		case strings.HasPrefix(joined, "docker network ls"):
			return ProcessResult{Stdout: fixtureNetworkID + "\tlab-net-4242\n"}, nil
		case strings.HasPrefix(joined, "docker image ls"):
			return ProcessResult{Stdout: fixtureImageID + "\tze-interop-peer:latest\n" +
				fixtureImageID + "\tze-interop-retained:abc\n"}, nil
		case strings.HasPrefix(joined, "docker container inspect"):
			return ProcessResult{Stdout: labelJSON(labels) + "\n"}, nil
		case strings.HasPrefix(joined, "docker network inspect"):
			return ProcessResult{Stdout: labelJSON(createdAs(labels, resourceNetwork)) + "\n"}, nil
		case strings.HasPrefix(joined, "docker image inspect"):
			return ProcessResult{Stdout: labelJSON(createdAs(labels, resourceImageTag)) + "\n"}, nil
		}
		return ProcessResult{}, nil
	}
}

// createdAs answers labels as the wrapper writes them on a resource of kind:
// the fixture's default container kind becomes kind's, and any other kind a
// test set stays, so a test can label every object with one inherited kind.
func createdAs(labels map[string]string, kind resourceKind) map[string]string {
	if labels[labelKind] != resourceContainer.labelValue() {
		return labels
	}
	created := maps.Clone(labels)
	created[labelKind] = kind.labelValue()
	return created
}

// VALIDATES: AC-1, AC-2, AC-3: every create path the wrapper owns carries the
// marker and every owner label, from the one helper.
// PREVENTS: a create path whose resource a later sweep cannot attribute.
func TestOwnerLabelsOnEveryCreate(t *testing.T) {
	runner := &recordingRunner{run: func(ProcessCommand) (ProcessResult, error) {
		return ProcessResult{Stdout: "sha256:feed\n"}, nil
	}}
	docker := newDocker(runner)
	ctx := t.Context()
	network := Network{Name: "lab-net", IPv4: netip.MustParsePrefix("172.29.0.0/24")}
	if err := docker.runContainer(ctx, network, PeerConfig{Name: "peer", Container: "lab-peer", Image: "peer-image", Host: 3}); err != nil {
		t.Fatalf("runContainer: %v", err)
	}
	if _, err := docker.RunOneShot(ctx, OneShotContainer{Image: "alpine", Timeout: dockerRunTimeout}); err != nil {
		t.Fatalf("RunOneShot: %v", err)
	}
	if _, err := docker.CreateNetwork(ctx, NetworkSpec{Name: "lab-net", Candidates: []Subnet{{IPv4: network.IPv4}}}); err != nil {
		t.Fatalf("CreateNetwork: %v", err)
	}
	if _, err := docker.Build(ctx, ImageBuild{Name: "peer", Tag: "peer-image", Dockerfile: "Dockerfile", Context: "."}); err != nil {
		t.Fatalf("Build: %v", err)
	}
	probe, err := docker.oneShotArguments(kernelProbeContainer("/staged/ze", false), fixtureOneShotName())
	if err != nil {
		t.Fatalf("oneShotArguments: %v", err)
	}

	creates := make([][]string, 0, 5)
	for _, command := range runner.commands {
		creates = append(creates, command.Arguments)
	}
	creates = append(creates, probe)
	if len(creates) != 5 {
		t.Fatalf("recorded %d create commands, want 5", len(creates))
	}
	owner := fixtureOwner()
	want := map[string]string{
		labelMarker:       labelMarkerValue,
		labelRun:          owner.run,
		labelPID:          "4242",
		labelPIDStart:     owner.pidStart,
		labelBootID:       owner.bootID,
		labelMachineID:    owner.machineID,
		labelStarted:      owner.started.Format(timeLayoutRFC3339),
		labelPurpose:      owner.purpose,
		labelUID:          "1000",
		labelOS:           owner.goos,
		labelPIDNamespace: owner.pidNamespace,
	}
	for _, argv := range creates {
		joined := strings.Join(argv, " ")
		for key, value := range want {
			if !strings.Contains(joined, "--label "+key+"="+value) {
				t.Errorf("%s carries no --label %s=%s", joined, key, value)
			}
		}
		kind := "container"
		switch {
		case strings.HasPrefix(joined, "docker build"):
			kind = "image"
		case strings.HasPrefix(joined, "docker network create"):
			kind = "network"
		}
		if !strings.Contains(joined, "--label "+labelKind+"="+kind) {
			t.Errorf("%s carries no --label %s=%s (AC-9)", joined, labelKind, kind)
		}
	}
}

// VALIDATES: AC-4: the owner facts and the verdict are taken on the client;
// no Docker command asks the daemon for a pid, a boot id or a machine id.
// PREVENTS: judging a Mac run by the Colima VM's pid namespace.
func TestOwnerIdentityNeverReadsTheDaemon(t *testing.T) {
	runner := &recordingRunner{run: orphanDaemon(fixtureLabels(nil), nil)}
	docker := newDocker(runner)
	if _, err := docker.CleanupOrphans(t.Context(), false); err != nil {
		t.Fatalf("CleanupOrphans: %v", err)
	}
	if len(runner.commands) == 0 {
		t.Fatal("CleanupOrphans ran no command, so the assertion below is vacuous")
	}
	for _, command := range runner.commands {
		joined := strings.Join(command.Arguments, " ")
		for _, daemonFact := range []string{"docker info", "docker version", "State.Pid", "/proc", "boot_id", "machine-id"} {
			if strings.Contains(joined, daemonFact) {
				t.Errorf("%q asks the daemon for an owner fact (%s)", joined, daemonFact)
			}
		}
	}
}

// VALIDATES: A-2, A-3: the host facts come from the client's own files on
// Linux and its own commands on macOS, and an unreadable or malformed answer
// is a named problem, never an empty identity read as a match.
func TestOwnerIdentityReadsBootID(t *testing.T) {
	linux := hostReaders{
		goos: "linux",
		readFile: func(path string) ([]byte, error) {
			switch path {
			case machineIDPath:
				return []byte("0123456789ABCDEF0123456789abcdef\n"), nil
			case bootIDPath:
				return []byte("01234567-89ab-cdef-0123-456789abcdef\n"), nil
			case procStatPath:
				return []byte("cpu  1 2 3\nbtime 1760781600\nprocesses 9\n"), nil
			}
			return nil, os.ErrNotExist
		},
		readLink: func(path string) (string, error) {
			if path == pidNamespacePath {
				return "pid:[4026531836]", nil
			}
			return "", os.ErrNotExist
		},
		uid: 1000,
	}
	facts := readHostFacts(linux)
	if facts.problem != "" {
		t.Fatalf("linux facts carry a problem: %s", facts.problem)
	}
	if facts.machineID != fixtureMachineID {
		t.Errorf("machine id = %q, want %q (lower-cased)", facts.machineID, fixtureMachineID)
	}
	if facts.bootID != fixtureBootID {
		t.Errorf("boot id = %q, want %q", facts.bootID, fixtureBootID)
	}
	if facts.pidNamespace != fixturePIDNamespace {
		t.Errorf("pid namespace = %q, want %q", facts.pidNamespace, fixturePIDNamespace)
	}
	if facts.uid != 1000 {
		t.Errorf("uid = %d, want 1000", facts.uid)
	}
	if want := time.Unix(1760781600, 0).UTC(); !facts.bootStarted.Equal(want) || facts.bootStartedProblem != "" {
		t.Errorf("boot started = %v (%q), want %v", facts.bootStarted, facts.bootStartedProblem, want)
	}

	darwin := hostReaders{
		goos: "darwin",
		command: func(name string, arguments ...string) ([]byte, error) {
			if name == "ioreg" {
				return []byte(`    "IOPlatformUUID" = "0123ABCD-0000-1111-2222-333344445555"` + "\n"), nil
			}
			if name == "sysctl" && slices.Contains(arguments, "kern.bootsessionuuid") {
				return []byte("01234567-89AB-CDEF-0123-456789ABCDEF\n"), nil
			}
			if name == "sysctl" && slices.Contains(arguments, "kern.boottime") {
				return []byte("{ sec = 1760781600, usec = 123 } Sat Oct 18 10:00:00 2026\n"), nil
			}
			return nil, errors.New("unexpected command")
		},
	}
	facts = readHostFacts(darwin)
	if facts.problem != "" {
		t.Fatalf("darwin facts carry a problem: %s", facts.problem)
	}
	if facts.machineID != "0123abcd-0000-1111-2222-333344445555" {
		t.Errorf("darwin machine id = %q", facts.machineID)
	}
	if facts.bootID != fixtureBootID {
		t.Errorf("darwin boot id = %q", facts.bootID)
	}
	if facts.pidNamespace != "" {
		t.Errorf("darwin pid namespace = %q, want none", facts.pidNamespace)
	}
	if want := time.Unix(1760781600, 0).UTC(); !facts.bootStarted.Equal(want) {
		t.Errorf("darwin boot started = %v (%q), want %v", facts.bootStarted, facts.bootStartedProblem, want)
	}
	hidden := linux
	hidden.readLink = func(string) (string, error) { return "", os.ErrPermission }
	if facts := readHostFacts(hidden); !strings.Contains(facts.problem, pidNamespacePath) {
		t.Errorf("an unreadable pid namespace gave problem %q, want one naming %s", facts.problem, pidNamespacePath)
	}

	unreadable := hostReaders{goos: "linux", readFile: func(string) ([]byte, error) { return nil, os.ErrPermission }}
	if facts := readHostFacts(unreadable); facts.problem == "" || !strings.Contains(facts.problem, machineIDPath) {
		t.Errorf("an unreadable machine id gave problem %q, want one naming %s", facts.problem, machineIDPath)
	}
	malformed := hostReaders{goos: "linux", readFile: func(string) ([]byte, error) { return []byte("not-an-id\n"), nil }}
	if facts := readHostFacts(malformed); facts.problem == "" {
		t.Error("a malformed machine id was accepted")
	}
	if facts := readHostFacts(hostReaders{goos: "plan9"}); facts.problem == "" {
		t.Error("a host with no identity source answered an identity")
	}
}

// VALIDATES: AC-5 (a, b, c) gone, AC-6 (a, b, c, d) skipped, and the pid label
// boundary, over injected identity and process scans.
// PREVENTS: R-1, a live run's resource judged gone.
func TestCleanupOrphansVerdictTable(t *testing.T) {
	host := fixtureJudge().host
	macOS := host
	macOS.goos = hostGOOSDarwin
	macOS.pidNamespace = ""
	bootTimeUnread := host
	bootTimeUnread.bootStartedProblem = "/proc/stat is unreadable"
	for _, row := range []struct {
		name   string
		host   hostFacts
		lookup func(int) (procscan.Process, bool, error)
		mutate func(map[string]string)
		want   ownerVerdict
	}{
		{"AC-5a same boot, pid absent", host, lookupAnswering(""), nil, ownerGone},
		{"AC-5b same boot, pid reused", host, lookupAnswering("999"), nil, ownerGone},
		{"AC-5c different boot", host, lookupAnswering("100"), func(l map[string]string) { l[labelBootID] = otherBootID }, ownerGone},
		{"AC-6a alive with the same token", host, lookupAnswering("100"), nil, ownerAlive},
		{"AC-6b another machine", host, lookupAnswering(""), func(l map[string]string) { l[labelMachineID] = otherMachineID }, ownerUnknown},
		{"AC-6c missing pid", host, lookupAnswering(""), func(l map[string]string) { delete(l, labelPID) }, ownerUnknown},
		{"AC-6c missing start token", host, lookupAnswering(""), func(l map[string]string) { delete(l, labelPIDStart) }, ownerUnknown},
		{"AC-6c malformed machine id", host, lookupAnswering(""), func(l map[string]string) { l[labelMachineID] = "x;rm -rf" }, ownerUnknown},
		{"AC-6c malformed boot id", host, lookupAnswering(""), func(l map[string]string) { l[labelBootID] = "" }, ownerUnknown},
		{"AC-6d own machine id unreadable", hostFacts{problem: "machine id unreadable"}, lookupAnswering(""), nil, ownerUnknown},
		{"pid scan failed", host, func(int) (procscan.Process, bool, error) { return procscan.Process{}, false, errors.New("ps failed") }, nil, ownerUnknown},
		{"pid 2147483647 is the last valid", host, lookupAnswering(""), func(l map[string]string) { l[labelPID] = "2147483647" }, ownerGone},
		{"pid 0 is malformed", host, lookupAnswering(""), func(l map[string]string) { l[labelPID] = "0" }, ownerUnknown},
		{"pid 2147483648 is malformed", host, lookupAnswering(""), func(l map[string]string) { l[labelPID] = "2147483648" }, ownerUnknown},
		{"pid -1 is malformed", host, lookupAnswering(""), func(l map[string]string) { l[labelPID] = "-1" }, ownerUnknown},
		{"R-1b another uid, pid hidden by hidepid", host, lookupAnswering(""), func(l map[string]string) { l[labelUID] = "1001" }, ownerUnknown},
		{"R-1a another pid namespace on one daemon", host, lookupAnswering(""), func(l map[string]string) { l[labelPIDNamespace] = "4026532999" }, ownerUnknown},
		{"linux owner with no pid namespace label", host, lookupAnswering(""), func(l map[string]string) { delete(l, labelPIDNamespace) }, ownerUnknown},
		{"no uid label", host, lookupAnswering(""), func(l map[string]string) { delete(l, labelUID) }, ownerUnknown},
		{"no os label", host, lookupAnswering(""), func(l map[string]string) { delete(l, labelOS) }, ownerUnknown},
		{"R-1c another boot id, owner started after this boot (a clone)", host, lookupAnswering(""), func(l map[string]string) {
			l[labelBootID] = otherBootID
			l[labelStarted] = "2026-10-18T12:00:00Z"
		}, ownerUnknown},
		{"another boot id, boot time unreadable", bootTimeUnread, lookupAnswering(""), func(l map[string]string) { l[labelBootID] = otherBootID }, ownerUnknown},
		{"macOS owner carries no pid namespace", macOS, lookupAnswering(""), func(l map[string]string) {
			l[labelOS] = hostGOOSDarwin
			delete(l, labelPIDNamespace)
		}, ownerGone},
		{"another boot id, owner start unreadable", host, lookupAnswering(""), func(l map[string]string) {
			l[labelBootID] = otherBootID
			l[labelStarted] = "yesterday"
		}, ownerUnknown},
	} {
		t.Run(row.name, func(t *testing.T) {
			judge := ownerJudge{host: row.host, lookup: row.lookup}
			got := judge.judge(resourceContainer, fixtureLabels(row.mutate))
			if got.verdict != row.want {
				t.Errorf("verdict = %v (%s), want %v", got.verdict, got.reason, row.want)
			}
			if got.reason == "" {
				t.Error("the verdict states no reason")
			}
			if got.verdict != ownerGone && got.fix == "" {
				t.Error("a skip names no fix")
			}
		})
	}
}

// VALIDATES: AC-5 order: containers, then networks, then the run-owned tags,
// each by the id the filtered listing gave; and R-7, a network a foreign
// endpoint still holds is a reported skip, not a failure.
func TestCleanupOrphansOrderContainersNetworksTags(t *testing.T) {
	runner := &recordingRunner{run: orphanDaemon(fixtureLabels(nil), nil)}
	docker := newDocker(runner)
	report, err := docker.CleanupOrphans(t.Context(), false)
	if err != nil {
		t.Fatalf("CleanupOrphans: %v", err)
	}
	assertOrder(t, recordedCommands(runner.commands),
		"docker ps -a --no-trunc --filter label=ze.lab",
		"docker rm -f "+fixtureContainerID,
		"docker network rm "+fixtureNetworkID,
		"docker image rm --no-prune ze-interop-retained:abc",
	)
	if len(report.Removed) != 3 {
		t.Errorf("removed %d resources, want 3: %#v", len(report.Removed), report.Removed)
	}
	if report.Code() != 0 {
		t.Errorf("code = %d, want 0", report.Code())
	}

	busy := &recordingRunner{run: orphanDaemon(fixtureLabels(nil), func(command ProcessCommand) (ProcessResult, bool) {
		if strings.HasPrefix(strings.Join(command.Arguments, " "), "docker network rm") {
			return ProcessResult{ExitCode: 1, Stderr: "Error response from daemon: error while removing network: network lab-net-4242 id " + fixtureNetworkID + " has active endpoints"}, true
		}
		return ProcessResult{}, false
	})}
	report, err = newDocker(busy).CleanupOrphans(t.Context(), false)
	if err != nil {
		t.Fatalf("CleanupOrphans: %v", err)
	}
	if len(report.Failed) != 0 {
		t.Errorf("a network with a foreign endpoint failed the sweep: %#v", report.Failed)
	}
	skipped := false
	for _, resource := range report.Skipped {
		if resource.ID == fixtureNetworkID && strings.Contains(resource.Reason, "endpoint") {
			skipped = true
		}
	}
	if !skipped {
		t.Errorf("the attached network is not a reported skip: %#v", report.Skipped)
	}
}

// VALIDATES: AC-9: the sweep lists only by the marker label, never removes a
// volume, never prunes, and never removes a tag other than the run-owned one.
func TestCleanupOrphansNeverTouchesUnlabeledVolumesOrCacheTags(t *testing.T) {
	for _, dry := range []bool{false, true} {
		runner := &recordingRunner{run: orphanDaemon(fixtureLabels(nil), nil)}
		if _, err := newDocker(runner).CleanupOrphans(t.Context(), dry); err != nil {
			t.Fatalf("CleanupOrphans(dry=%v): %v", dry, err)
		}
		for _, command := range runner.commands {
			joined := strings.Join(command.Arguments, " ")
			if slices.Contains(command.Arguments, "volume") || slices.Contains(command.Arguments, "prune") {
				t.Errorf("dry=%v ran %q", dry, joined)
			}
			if strings.Contains(joined, "ze-interop-peer:latest") {
				t.Errorf("dry=%v touched the shared cache tag: %q", dry, joined)
			}
			listing := strings.HasPrefix(joined, "docker ps") || strings.HasPrefix(joined, "docker network ls")
			if listing && !strings.Contains(joined, "--filter label="+labelMarker) {
				t.Errorf("dry=%v listed without the marker filter: %q", dry, joined)
			}
			if dry && slices.Contains(command.Arguments, "rm") {
				t.Errorf("dry run removed something: %q", joined)
			}
		}
	}
}

// VALIDATES: R-6: a resource another sweep already removed is success.
func TestCleanupOrphansConcurrentAbsentIsSuccess(t *testing.T) {
	runner := &recordingRunner{run: orphanDaemon(fixtureLabels(nil), func(command ProcessCommand) (ProcessResult, bool) {
		joined := strings.Join(command.Arguments, " ")
		switch {
		case strings.HasPrefix(joined, "docker rm -f"):
			return ProcessResult{ExitCode: 1, Stderr: "Error response from daemon: No such container: " + fixtureContainerID}, true
		case strings.HasPrefix(joined, "docker network rm"):
			return ProcessResult{ExitCode: 1, Stderr: "Error response from daemon: network " + fixtureNetworkID + " not found"}, true
		case strings.HasPrefix(joined, "docker image rm"):
			return ProcessResult{ExitCode: 1, Stderr: "Error response from daemon: No such image: ze-interop-retained:abc"}, true
		}
		return ProcessResult{}, false
	})}
	report, err := newDocker(runner).CleanupOrphans(t.Context(), false)
	if err != nil {
		t.Fatalf("CleanupOrphans: %v", err)
	}
	if len(report.Failed) != 0 || report.Code() != 0 {
		t.Errorf("an already-removed resource failed the sweep: code %d, %#v", report.Code(), report.Failed)
	}
}

// VALIDATES: AC-6 messages: each skip names the resource, why it stays, and
// the command that resolves it; a removal failure names the failing command.
func TestCleanupOrphansSkipMessagesNameFix(t *testing.T) {
	foreign := fixtureLabels(func(l map[string]string) { l[labelMachineID] = otherMachineID })
	docker := newDocker(&recordingRunner{run: orphanDaemon(foreign, nil)})
	report, err := docker.CleanupOrphans(t.Context(), false)
	if err != nil {
		t.Fatalf("CleanupOrphans: %v", err)
	}
	if len(report.Skipped) == 0 {
		t.Fatal("a foreign owner's resources were not skipped")
	}
	for _, resource := range report.Skipped {
		if !strings.Contains(resource.Fix, "./le test integration clean") || !strings.Contains(resource.Fix, otherMachineID) {
			t.Errorf("foreign skip fix %q names neither the command nor the machine", resource.Fix)
		}
	}
	text := report.Text()
	if !strings.Contains(text, "lab-peer-4242") {
		t.Errorf("the text report does not name the container:\n%s", text)
	}

	alive := newDocker(&recordingRunner{run: orphanDaemon(fixtureLabels(nil), nil)})
	alive.judge.lookup = lookupAnswering("100")
	report, err = alive.CleanupOrphans(t.Context(), true)
	if err != nil {
		t.Fatalf("CleanupOrphans: %v", err)
	}
	for _, resource := range report.Skipped {
		if !strings.Contains(resource.Reason, "4242") || !strings.Contains(resource.Reason, "le test integration interop") {
			t.Errorf("a live-owner skip %q names neither the pid nor its command line", resource.Reason)
		}
	}

	failing := newDocker(&recordingRunner{run: orphanDaemon(fixtureLabels(nil), func(command ProcessCommand) (ProcessResult, bool) {
		if strings.HasPrefix(strings.Join(command.Arguments, " "), "docker rm -f") {
			return ProcessResult{ExitCode: 1, Stderr: "Error response from daemon: device busy"}, true
		}
		return ProcessResult{}, false
	})})
	report, err = failing.CleanupOrphans(t.Context(), false)
	if err != nil {
		t.Fatalf("CleanupOrphans: %v", err)
	}
	if report.Code() != 1 || len(report.Failed) != 1 || !strings.Contains(report.Failed[0].Error, "docker rm -f") ||
		!strings.Contains(report.Failed[0].Fix, "docker rm -f "+fixtureContainerID) {
		t.Errorf("a failed removal is not named with its command: code %d, %#v", report.Code(), report.Failed)
	}
}

// VALIDATES: Q-3: the dry run lists an unlabeled run-owned tag left from
// before the labels existed, and no mode removes it.
func TestCleanupOrphansListsUnlabeledRetainedTags(t *testing.T) {
	runner := &recordingRunner{run: func(command ProcessCommand) (ProcessResult, error) {
		joined := strings.Join(command.Arguments, " ")
		if strings.HasPrefix(joined, "docker image ls") && strings.Contains(joined, "reference=") {
			return ProcessResult{Stdout: "sha256:dd\tze-interop-retained:old\n"}, nil
		}
		return ProcessResult{}, nil
	}}
	report, err := newDocker(runner).CleanupOrphans(context.Background(), true)
	if err != nil {
		t.Fatalf("CleanupOrphans: %v", err)
	}
	if len(report.Unlabeled) != 1 || report.Unlabeled[0].Name != "ze-interop-retained:old" {
		t.Errorf("unlabeled = %#v, want the old retained tag", report.Unlabeled)
	}
	for _, command := range runner.commands {
		if slices.Contains(command.Arguments, "rm") {
			t.Errorf("the dry run removed: %v", command.Arguments)
		}
	}
}

// presenceAnswer is what the re-observation after a failed removal answers.
type presenceAnswer int

const (
	presenceGone presenceAnswer = iota + 1
	presenceStill
	presenceNoAnswer
)

// timedOutRemovalDaemon answers the labeled listings and label reads like
// orphanDaemon, with an owner pid that is absent, times out every removal the
// way a loaded daemon outlasts the client bound, and answers the `{{.Id}}`
// re-observation with presence.
func timedOutRemovalDaemon(presence presenceAnswer) func(ProcessCommand) (ProcessResult, error) {
	listings := orphanDaemon(fixtureLabels(nil), nil)
	return func(command ProcessCommand) (ProcessResult, error) {
		joined := strings.Join(command.Arguments, " ")
		removal := strings.HasPrefix(joined, "docker rm -f") ||
			strings.HasPrefix(joined, "docker network rm") ||
			strings.HasPrefix(joined, "docker image rm")
		if removal {
			return ProcessResult{ExitCode: -1}, context.DeadlineExceeded
		}
		if !strings.Contains(joined, dockerFormatFlag+" {{.Id}}") {
			return listings(command)
		}
		switch presence {
		case presenceGone:
			return ProcessResult{ExitCode: 1, Stderr: "Error response from daemon: No such object: " +
				command.Arguments[len(command.Arguments)-1]}, nil
		case presenceStill:
			return ProcessResult{Stdout: fixtureContainerID + "\n"}, nil
		case presenceNoAnswer:
			return ProcessResult{ExitCode: -1}, context.DeadlineExceeded
		}
		return ProcessResult{}, errors.New("BUG: timedOutRemovalDaemon without a presence answer")
	}
}

// timedOutRemovalDocker is a Docker whose judge finds the fixture owner gone.
func timedOutRemovalDocker(presence presenceAnswer) *Docker {
	docker := newDocker(&recordingRunner{run: timedOutRemovalDaemon(presence)})
	docker.judge.lookup = lookupAnswering("")
	return docker
}

// VALIDATES: a removal whose call outlasts the client bound, while the daemon
// completes it, is reported removed: the sweep re-observes the resource by id
// and finds it gone.
// PREVENTS: "1 removed, 3 failed" and exit 1 for four resources the daemon
// removed (the 2026-10-18 run under load ~30).
func TestCleanupOrphansTimedOutRemovalOfGoneResourceIsRemoved(t *testing.T) {
	report, err := timedOutRemovalDocker(presenceGone).CleanupOrphans(t.Context(), false)
	if err != nil {
		t.Fatalf("CleanupOrphans: %v", err)
	}
	if len(report.Removed) != 3 || len(report.Failed) != 0 || len(report.Unknown) != 0 || report.Code() != 0 {
		t.Fatalf("removed %d, failed %#v, unknown %#v, code %d; want 3 removed and code 0",
			len(report.Removed), report.Failed, report.Unknown, report.Code())
	}
}

// VALIDATES: a removal that timed out while the resource is still there is a
// failure that carries the removal's own error.
// PREVENTS: the re-observation turning a real failure into "removed".
func TestCleanupOrphansTimedOutRemovalOfPresentResourceFails(t *testing.T) {
	report, err := timedOutRemovalDocker(presenceStill).CleanupOrphans(t.Context(), false)
	if err != nil {
		t.Fatalf("CleanupOrphans: %v", err)
	}
	if len(report.Failed) != 3 || len(report.Removed) != 0 || report.Code() != 1 {
		t.Fatalf("failed %d, removed %#v, code %d; want 3 failed and code 1", len(report.Failed), report.Removed, report.Code())
	}
	for _, resource := range report.Failed {
		if !strings.Contains(resource.Error, "did not finish within") {
			t.Errorf("%s %s: error %q does not carry the removal's timeout", resource.Kind, resource.Name, resource.Error)
		}
	}
}

// VALIDATES: when the removal and its re-observation both fail, the outcome is
// unknown, named with both errors, and the command exits 1.
// PREVENTS: a guess in either direction when the daemon answered neither.
func TestCleanupOrphansRecheckFailureIsUnknown(t *testing.T) {
	report, err := timedOutRemovalDocker(presenceNoAnswer).CleanupOrphans(t.Context(), false)
	if err != nil {
		t.Fatalf("CleanupOrphans: %v", err)
	}
	if len(report.Unknown) != 3 || len(report.Removed) != 0 || len(report.Failed) != 0 || report.Code() != 1 {
		t.Fatalf("unknown %d, removed %#v, failed %#v, code %d; want 3 unknown and code 1",
			len(report.Unknown), report.Removed, report.Failed, report.Code())
	}
	for _, resource := range report.Unknown {
		if strings.Count(resource.Error, "did not finish within") != 2 {
			t.Errorf("%s %s: error %q does not carry both the removal and the re-check error", resource.Kind, resource.Name, resource.Error)
		}
	}
	if !strings.Contains(report.Text(), "0 failed, 3 unknown") {
		t.Errorf("summary does not count the unknown outcomes:\n%s", report.Text())
	}
}

// VALIDATES: RemoveContainer, which Lab.cleanup calls for every peer, answers
// success for a timed-out removal whose container is gone, the removal's error
// while it stays, and an error naming both failures when the re-check fails.
// PREVENTS: a scenario teardown listing a removed container as a cleanup error.
func TestRemoveContainerTimedOutReobservesPresence(t *testing.T) {
	if err := timedOutRemovalDocker(presenceGone).RemoveContainer(t.Context(), "lab-peer"); err != nil {
		t.Errorf("gone after a timed-out removal: %v, want nil", err)
	}
	err := timedOutRemovalDocker(presenceStill).RemoveContainer(t.Context(), "lab-peer")
	if err == nil {
		t.Fatal("still present after a timed-out removal: nil, want the removal's timeout")
	}
	if strings.Count(err.Error(), "did not finish within") != 1 {
		t.Errorf("still present after a timed-out removal: %v, want the removal's timeout alone", err)
	}
	err = timedOutRemovalDocker(presenceNoAnswer).RemoveContainer(t.Context(), "lab-peer")
	if err == nil {
		t.Fatal("re-check failed: nil, want an error naming both failures")
	}
	if strings.Count(err.Error(), "did not finish within") != 2 {
		t.Errorf("re-check failed: %v, want the removal and the re-check errors", err)
	}
}

// VALIDATES: AC-9: a container that carries only the owner labels it
// inherited from a lab-built image (kind image, as a container a person
// starts by hand from that image does) is never removed, and is reported as a
// skip naming why, while the run-owned tag the same labels sit on still goes.
// PREVENTS: the sweep removing a hand-started container once the builder of
// its image has exited.
func TestCleanupOrphansSkipsContainerWithInheritedImageLabels(t *testing.T) {
	inherited := fixtureLabels(func(labels map[string]string) { labels["ze.lab.kind"] = "image" })
	runner := &recordingRunner{run: orphanDaemon(inherited, nil)}
	report, err := newDocker(runner).CleanupOrphans(t.Context(), false)
	if err != nil {
		t.Fatalf("CleanupOrphans: %v", err)
	}
	joined := recordedCommands(runner.commands)
	if strings.Contains(joined, "docker rm -f "+fixtureContainerID) {
		t.Fatalf("a container with inherited image labels was removed:\n%s", joined)
	}
	if !strings.Contains(joined, "docker image rm --no-prune ze-interop-retained:abc") {
		t.Errorf("the run-owned tag whose image carries the labels was not removed:\n%s", joined)
	}
	skipped := false
	for _, resource := range report.Skipped {
		if resource.Kind == resourceContainer && strings.Contains(resource.Reason, "image labels only") &&
			strings.Contains(resource.Reason, "not created by a lab run") && resource.Fix != "" {
			skipped = true
		}
	}
	if !skipped {
		t.Errorf("no container skip names inherited image labels: %#v", report.Skipped)
	}
}

// VALIDATES: the client's operating system survives an identity it cannot
// read: every problem answer of readHostFacts still carries readers.goos, so
// an AppArmor refusal on a Linux host whose machine id is unreadable still
// names the Linux load command, and an empty system reaching a refusal
// builder is a Ze defect that panics rather than picking the non-Linux text.
// PREVENTS: "cannot be loaded from : ..." and "On  the labs run inside" on a
// host with no /etc/machine-id, such as a minimal guest.
func TestHostFactsKeepTheSystemWhenTheIdentityIsUnreadable(t *testing.T) {
	failing := func(string) ([]byte, error) { return nil, os.ErrPermission }
	for _, readers := range []hostReaders{
		{goos: hostGOOSLinux, readFile: failing},
		{goos: hostGOOSDarwin, command: func(string, ...string) ([]byte, error) { return nil, os.ErrPermission }},
		{goos: "plan9"},
	} {
		facts := readHostFacts(readers)
		if facts.problem == "" {
			t.Fatalf("%s: an unreadable identity gave no problem", readers.goos)
		}
		if facts.goos != readers.goos {
			t.Errorf("%s: the problem answer lost the system: goos %q", readers.goos, facts.goos)
		}
	}
	host := readHostFacts(hostReaders{goos: hostGOOSLinux, readFile: failing})
	registerTestLabProfile(t)
	profile, _ := LookupAppArmorProfile(testLabProfile)
	refusal := labAppArmorRefusal(host.goos, profile, "x")
	requireCommandOnItsOwnLine(t, refusal, AppArmorLoadCommandFor(testLabProfile))

	for name, build := range map[string]func(){
		"lab":    func() { _ = labAppArmorRefusal("", profile, "x") },
		"probe":  func() { _ = appArmorRefusal("", "6.8", "x") },
		"denied": func() { _ = deniedFix("", true, false) },
	} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("%s: an empty client system chose a refusal text", name)
				}
			}()
			build()
		}()
	}
}
