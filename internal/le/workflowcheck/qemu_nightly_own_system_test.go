package workflowcheck

import (
	"strings"
	"testing"

	testqemu "github.com/ze-software/ze/internal/le/test/qemu"
)

// TestQEMUNightlyRunsAllInZeSystem pins the needs-linux job to the route `test
// qemu all` accepts.
// VALIDATES: the job boots Ze's image through `docker-lab` with the harness, fills
// the module cache the image build reads, and gives the guest a Go tree it can
// reach, because Ze's image carries none and three phases compile in the guest.
// PREVENTS: the step stopping before any suite, because the guest action refuses
// any guest whose PID 1 is not /gokrazy/init.
// METHOD: read the needs-linux job of qemu-nightly.yml with comments stripped.
func TestQEMUNightlyRunsAllInZeSystem(t *testing.T) {
	const name = "qemu-nightly.yml"
	var body string
	for _, job := range jobBlocks(t, name) {
		if job.name == "needs-linux" {
			body = stripComments(job.body)
		}
	}
	if body == "" {
		t.Fatalf("%s has no needs-linux job", name)
	}
	if strings.Contains(body, "./le test qemu run") {
		t.Errorf("%s needs-linux still boots the Alpine route", name)
	}
	for _, want := range []string{
		"./le test qemu docker-lab",
		"kernel tmp/kernel/build/vmlinuz",
		testqemu.KeywordHarness,
		"./le setup appliance-modules",
		"cp -RL \"$GOROOT\" tmp/qemu/_guest-go",
		"GOROOT=/workspace/tmp/qemu/_guest-go",
		// le finds the toolchain through exec.LookPath, and `go test` runs by name.
		"PATH=/workspace/tmp/qemu/_guest-go/bin:$PATH",
		"tmp/qemu/linux-amd64/le test qemu all",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("%s needs-linux lacks %q", name, want)
		}
	}
}

// TestQEMUNightlyNamespaceStepsRunInZeSystem pins the two namespace steps whose
// tools Ze's image carries to the route that boots it.
// VALIDATES: each step's nearest `./le test qemu` route is `docker-lab` with the
// harness, and the job fills the module cache the image build reads.
// PREVENTS: a guest moved to Ze's system going back to the Alpine route unnoticed,
// and the image build refusing because the module cache is empty.
// METHOD: read the runtime-kernel-labs job of qemu-nightly.yml with comments
// stripped, and take the route text before each step's guest command.
func TestQEMUNightlyNamespaceStepsRunInZeSystem(t *testing.T) {
	const name = "qemu-nightly.yml"
	var body string
	for _, job := range jobBlocks(t, name) {
		if job.name == "runtime-kernel-labs" {
			body = stripComments(job.body)
		}
	}
	if body == "" {
		t.Fatalf("%s has no runtime-kernel-labs job", name)
	}
	if !strings.Contains(body, "./le setup appliance-modules") {
		t.Errorf("%s runtime-kernel-labs does not fill the module cache", name)
	}
	const route = "./le test qemu "
	for _, proof := range []string{"le test qemu pppoe'", "le test qemu netns suites plugin'"} {
		at := strings.Index(body, proof)
		if at < 0 {
			t.Errorf("%s runtime-kernel-labs lacks %q", name, proof)
			continue
		}
		start := strings.LastIndex(body[:at], route)
		if start < 0 {
			t.Errorf("%s runtime-kernel-labs has no route before %q", name, proof)
			continue
		}
		words := strings.Fields(body[start:at])
		if len(words) < 4 || words[3] != "docker-lab" {
			t.Errorf("%s step %q runs through %q, want docker-lab", name, proof, words)
		}
		if !strings.Contains(body[start:at], testqemu.KeywordHarness) {
			t.Errorf("%s step %q lacks %s", name, proof, testqemu.KeywordHarness)
		}
	}
}
