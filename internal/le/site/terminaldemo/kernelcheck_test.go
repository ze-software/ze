package siteterminaldemo

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// kernelLackingDocker is a docker on PATH whose daemon kernel lacks MOBIKE's
// XFRM_MSG_MIGRATE_STATE. It records every argv and answers the kernel probe
// with one absent row.
const kernelLackingDocker = `#!/bin/sh
echo "$*" >> "$DOCKER_RECORD"
case "$*" in
*KernelVersion*) echo 6.8.0-117-generic ;;
*SecurityOptions*) echo '["name=seccomp,profile=builtin","name=cgroupns"]' ;;
*kernel-capabilities*)
  echo '{"ready": false, "capabilities": [{"subsystem": "ipsec-mobike", "kernel": "CONFIG_XFRM_MIGRATE", "state": "absent", "reason": "XFRM_MSG_MIGRATE_STATE: invalid argument"}]}'
  exit 1 ;;
esac
exit 0
`

// VALIDATES: AC-3 through the terminal-demo render and the renderer's validate
// mode, with the production check (no KernelCheck option). A daemon kernel
// lacking an enrolled feature refuses the render or the validation, naming the feature and the kernel release, before any validator
// or recorder container starts.
// PREVENTS: a demo recorded on a kernel Ze does not support, whose transcript
// then shows a host fact as product behavior.
func TestRenderRefusesMissingKernelFeature(t *testing.T) {
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "docker"), []byte(kernelLackingDocker), 0o755); err != nil { //nolint:gosec // a stub on a test's own PATH must be executable
		t.Fatalf("write the docker stub: %v", err)
	}
	record := filepath.Join(bin, "record")
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("DOCKER_RECORD", record)

	fixture := newDemoFixture(t)
	var output bytes.Buffer
	engine := New(Options{
		Root:         fixture.root,
		ArtifactRoot: fixture.artifacts,
		Executor:     fixture.execute,
		Lookup:       func(string) (string, error) { return "fixture-ffmpeg", nil },
		Output:       &output,
		LockWait:     30 * time.Millisecond,
		LockPoll:     time.Millisecond,
	})

	for _, render := range []struct {
		name string
		run  func() error
	}{
		{"all", func() error { _, err := engine.RenderAll("26.10.10"); return err }},
		{"one", func() error { _, err := engine.RenderOne("term", "26.10.10"); return err }},
		// The renderer's validate mode runs ze-demo in containers too.
		{"validate", func() error { _, err := engine.validationCheckAll(); return err }},
	} {
		err := render.run()
		if err == nil {
			t.Fatalf("render %s proceeded on a kernel lacking XFRM_MSG_MIGRATE_STATE", render.name)
		}
		for _, want := range []string{"ipsec-mobike", "CONFIG_XFRM_MIGRATE", "6.8.0-117-generic"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("render %s refusal does not name %q: %v", render.name, want, err)
			}
		}
	}
	for _, command := range fixture.commands {
		if len(command.Args) > 1 && command.Args[1] == "run" {
			t.Errorf("a container started after the refusal: %v", command.Args)
		}
	}
	recorded, err := os.ReadFile(record) //nolint:gosec // the test's own temp file
	if err != nil {
		t.Fatalf("read the docker record: %v", err)
	}
	if !strings.Contains(string(recorded), filepath.Join(fixture.root, "tmp", "terminal-demos", "bin", "ze")) {
		t.Errorf("the probe did not mount the demo binary:\n%s", recorded)
	}
}

// VALIDATES: a render whose kernel check passes probes with the demo binary
// once per render and then proceeds to the validators.
// PREVENTS: a check wired after the first container, or against another binary.
func TestRenderChecksKernelWithTheDemoBinary(t *testing.T) {
	fixture := newDemoFixture(t)
	var output bytes.Buffer
	if _, err := fixture.engine(&output).RenderOne("term", "26.10.10"); err != nil {
		t.Fatalf("render: %v\n%s", err, output.String())
	}
	want := filepath.Join(fixture.root, "tmp", "terminal-demos", "bin", "ze")
	if len(fixture.kernelChecked) != 1 || fixture.kernelChecked[0] != want {
		t.Fatalf("kernel checks = %v, want one with %s", fixture.kernelChecked, want)
	}
}
