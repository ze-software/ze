//go:build docker

package interoplab

import (
	"bufio"
	"context"
	"net/netip"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// orphanChildVariable makes a re-exec of this test binary act as a lab run
// that prints its owner run id, creates one labeled network, container and
// retained image tag, prints their names, and then blocks until it is killed.
const orphanChildVariable = "ZE_ORPHAN_TEST_CHILD"

// orphanTestImage is the base the child's container and build use.
const orphanTestImage = "alpine:3.21"

// VALIDATES: the wiring row "a lab run SIGKILLed, then any lab start ->
// labels on create + Docker.CleanupOrphans", against the real daemon: a run
// killed with SIGKILL leaves its network, container and run-owned tag, and the
// next sweep removes exactly that set while a live run's set beside it stays,
// reported as kept. Once the second run dies too, the next sweep removes its
// set as well.
// PREVENTS: R-1 (a live run's resources removed) and V-1 (a killed run's
// resources kept forever).
func TestKilledLabRunIsCleanedByNextStart(t *testing.T) {
	if suffix := os.Getenv(orphanChildVariable); suffix != "" {
		runOrphanChild(t, suffix)
		return
	}
	if _, err := exec.LookPath(DockerExecutable); err != nil {
		if os.Getenv("ZE_REQUIRE_DOCKER") == "1" {
			t.Fatalf("docker is not installed, and ZE_REQUIRE_DOCKER=1 says this run must exercise it. Fix: install Docker")
		}
		t.Skip("docker is not installed; install Docker to run this test")
	}
	ctx := t.Context()
	if err := NewDocker().Probe(ctx); err != nil {
		t.Fatalf("Docker does not answer: %v. Fix: start Docker, or point DOCKER_HOST at a daemon", err)
	}

	stamp := strconv.FormatInt(time.Now().UnixNano(), 36)
	killed := startOrphanChild(t, "killed-"+stamp, 0)
	live := startOrphanChild(t, "live-"+stamp, 1)

	if err := killed.process.Process.Signal(syscall.SIGKILL); err != nil {
		t.Fatalf("SIGKILL the first run: %v", err)
	}
	_ = killed.process.Wait() //nolint:errcheck // a SIGKILLed child exits non-zero by design
	for _, name := range killed.resources() {
		if !resourceExists(t, name) {
			t.Fatalf("%s vanished with its SIGKILLed owner, so this run cannot prove the sweep removes it", name)
		}
	}

	report, err := NewDocker().CleanupOrphans(ctx, false)
	if err != nil {
		t.Fatalf("CleanupOrphans: %v", err)
	}
	t.Logf("first sweep:\n%s", report.Text())
	for _, name := range killed.resources() {
		if resourceExists(t, name) {
			t.Errorf("the killed run's %s survived the sweep", name)
		}
	}
	for _, name := range live.resources() {
		if !resourceExists(t, name) {
			t.Errorf("the sweep removed the live run's %s", name)
		}
	}
	kept := false
	for _, resource := range report.Skipped {
		if resource.Name == live.container && strings.Contains(resource.Reason, "is still running") {
			kept = true
		}
	}
	if !kept {
		t.Errorf("the live run's container %s is not reported as kept: %#v", live.container, report.Skipped)
	}

	if err := live.process.Process.Signal(syscall.SIGKILL); err != nil {
		t.Fatalf("SIGKILL the second run: %v", err)
	}
	_ = live.process.Wait() //nolint:errcheck // a SIGKILLed child exits non-zero by design
	report, err = NewDocker().CleanupOrphans(ctx, false)
	if err != nil {
		t.Fatalf("CleanupOrphans: %v", err)
	}
	t.Logf("second sweep:\n%s", report.Text())
	for _, name := range live.resources() {
		if resourceExists(t, name) {
			t.Errorf("the second run's %s survived the sweep after it was killed", name)
		}
	}
}

// orphanChild is one re-exec'd lab run and the names of what it created.
type orphanChild struct {
	process   *exec.Cmd
	run       string
	network   string
	container string
	retained  string
	cacheTag  string
}

func (c orphanChild) resources() []string {
	return []string{"container:" + c.container, "network:" + c.network, "image:" + c.retained}
}

func startOrphanChild(t *testing.T, suffix string, subnet int) orphanChild {
	t.Helper()
	command := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestKilledLabRunIsCleanedByNextStart$", "-test.timeout=10m") //nolint:gosec // this test binary
	command.Env = append(os.Environ(), orphanChildVariable+"="+suffix, "ZE_ORPHAN_TEST_SUBNET="+strconv.Itoa(subnet))
	command.Stderr = os.Stderr
	stdout, err := command.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := command.Start(); err != nil {
		t.Fatalf("start the %s run: %v", suffix, err)
	}
	child := orphanChild{process: command, cacheTag: "ze-orphan-test:" + suffix}
	// Every exit path, a child whose setup failed included, removes what the
	// child's run created: the sweep may never have run, or may have kept a
	// set whose owner was still alive when it looked.
	t.Cleanup(func() {
		_ = command.Process.Kill() //nolint:errcheck // already dead on the normal path
		_ = command.Wait()         //nolint:errcheck // reaped for the cleanup only
		removeRunResources(t, child.run)
		// The sweep never removes a cache tag, so the test removes the one it made.
		_ = exec.Command(DockerExecutable, "image", "rm", "--no-prune", child.cacheTag).Run() //nolint:errcheck,gosec,noctx // best-effort removal of this test's own tag
	})
	lines := bufio.NewScanner(stdout)
	deadline := time.AfterFunc(5*time.Minute, func() { _ = command.Process.Kill() }) //nolint:errcheck // a wedged child
	defer deadline.Stop()
	for lines.Scan() {
		fields := strings.Fields(lines.Text())
		if len(fields) == 2 && fields[0] == "orphan-run" {
			child.run = fields[1]
			continue
		}
		if len(fields) == 4 && fields[0] == "orphan-ready" {
			child.network, child.container, child.retained = fields[1], fields[2], fields[3]
			return child
		}
		t.Logf("%s run: %s", suffix, lines.Text())
	}
	t.Fatalf("the %s run exited before it created its resources", suffix)
	return child
}

func runOrphanChild(t *testing.T, suffix string) {
	ctx := context.Background()
	docker := NewDocker()
	// The run id goes out before anything is created, so the parent can remove
	// this run's resources whichever step below fails.
	os.Stdout.WriteString("orphan-run " + docker.owner.run + "\n") //nolint:errcheck // the parent reads it
	subnet, _ := strconv.Atoi(os.Getenv("ZE_ORPHAN_TEST_SUBNET"))  //nolint:errcheck // the parent always sets it
	candidates := make([]Subnet, 0, 8)
	for third := 240 + 8*subnet; third < 248+8*subnet; third++ {
		candidates = append(candidates, Subnet{IPv4: netip.MustParsePrefix("172.31." + strconv.Itoa(third) + ".0/24")})
	}
	network, err := docker.createNetwork(ctx, NetworkSpec{Name: "ze-orphan-net-" + suffix, Candidates: candidates})
	if err != nil {
		t.Fatalf("child network: %v", err)
	}
	container := "ze-orphan-peer-" + suffix
	peer := PeerConfig{Name: "peer", Container: container, Image: orphanTestImage, Host: 2, Command: []string{"sleep", "3600"}}
	if err := docker.runContainer(ctx, network, peer); err != nil {
		t.Fatalf("child container: %v", err)
	}
	buildContext := t.TempDir()
	if err := os.WriteFile(filepath.Join(buildContext, "Dockerfile"), []byte("FROM "+orphanTestImage+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	image, err := docker.Build(ctx, ImageBuild{Name: "orphan", Tag: "ze-orphan-test:" + suffix,
		Dockerfile: filepath.Join(buildContext, "Dockerfile"), Context: buildContext})
	if err != nil {
		t.Fatalf("child build: %v", err)
	}
	os.Stdout.WriteString("orphan-ready " + network.Name + " " + container + " " + image.retainedTag + "\n") //nolint:errcheck // the parent reads it
	select {}
}

// removeRunResources removes, through the wrapper, every container, network
// and run-owned tag labeled with one run id, in the sweep's order. A run id
// that never reached the parent means the child created nothing.
func removeRunResources(t *testing.T, run string) {
	t.Helper()
	if run == "" {
		return
	}
	// t.Context is already cancelled when a cleanup runs; each daemon call
	// below keeps the wrapper's own bound.
	ctx := context.Background()
	docker := NewDocker()
	for _, kind := range []resourceKind{resourceContainer, resourceNetwork, resourceImageTag} {
		rows, err := docker.listResources(ctx, kind, "label="+labelRun+"="+run)
		if err != nil {
			t.Errorf("list run %s's %s resources to remove them: %v", run, kind, err)
			continue
		}
		for _, row := range rows {
			if kind == resourceImageTag && !strings.HasPrefix(row.name, retainedTagRepository+":") {
				continue
			}
			if err := docker.removeResource(ctx, kind, row); err != nil {
				t.Errorf("remove run %s's %s %s: %v", run, kind, row.name, err)
			}
		}
	}
}

// resourceExists asks the daemon whether "kind:name" still exists.
func resourceExists(t *testing.T, resource string) bool {
	t.Helper()
	kind, name, _ := strings.Cut(resource, ":")
	argv := []string{kind, "inspect", name}
	if kind == "image" {
		argv = []string{"image", "inspect", name}
	}
	output, err := exec.CommandContext(t.Context(), DockerExecutable, argv...).CombinedOutput() //nolint:gosec // fixed inspect
	if err == nil {
		return true
	}
	text := strings.ToLower(string(output))
	if strings.Contains(text, "no such") || strings.Contains(text, "not found") {
		return false
	}
	t.Fatalf("docker %s: %v: %s", strings.Join(argv, " "), err, output)
	return false
}
