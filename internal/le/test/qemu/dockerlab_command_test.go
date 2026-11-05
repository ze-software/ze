// Design: docs/architecture/testing/qemu-integration.md -- One command in Ze's own system

package testqemu

import (
	"bytes"
	"context"
	"encoding/binary"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/appliance/dockerhost"
	"github.com/ze-software/ze/internal/thirdparty/fat"
)

// TestDockerLabCommandRefusesIncompatibleModes reaches the registered consumer
// and requires its specific mode error, not an unknown-keyword or tool failure.
func TestDockerLabCommandRefusesIncompatibleModes(t *testing.T) {
	for _, test := range []struct {
		name string
		args []string
		want string
	}{
		{"command and lab", []string{keywordKernel, "/unused", keywordCommand, "true", keywordLab, "test integration"}, "command and lab are mutually exclusive"},
		{"missing kernel", []string{keywordCommand, "true"}, "command requires kernel"},
		{"kernel without command", []string{keywordKernel, "/unused"}, "kernel is only valid with command"},
	} {
		t.Run(test.name, func(t *testing.T) {
			stderr, err := os.CreateTemp(t.TempDir(), "refusal-")
			if err != nil {
				t.Fatal(err)
			}
			saved := os.Stderr
			os.Stderr = stderr
			t.Cleanup(func() {
				os.Stderr = saved
				if err := stderr.Close(); err != nil {
					t.Error(err)
				}
			})
			answer, code := Answer(append([]string{dockerLabVerb}, test.args...))
			output, err := os.ReadFile(stderr.Name())
			if err != nil {
				t.Fatal(err)
			}
			if code != 1 || !strings.Contains(string(output), test.want) {
				t.Fatalf("consumer answered %d (%v), stderr %q; want mode refusal %q", code, answer, output, test.want)
			}
		})
	}
}

// TestDockerLabCommandKernelRejectsDifferentBytes uses equal filenames and
// lengths so only the actual byte identity can discriminate the bad request.
func TestDockerLabCommandKernelRejectsDifferentBytes(t *testing.T) {
	tree := dockerLabRuntimeFixture(t)
	requested := filepath.Join(t.TempDir(), "vmlinuz")
	if err := os.WriteFile(requested, []byte("foreign-kernel"), 0o600); err != nil {
		t.Fatal(err)
	}
	selected, err := dockerLabCommandKernel(requested, tree)
	if selected != nil || err == nil || !strings.Contains(err.Error(), "not image runtime kernel") {
		t.Fatalf("different kernel bytes selected %+v, error %v", selected, err)
	}
}

// TestDockerLabCommandKernelRefusesUnknownModules makes the kernel bytes match
// but supplies two releases, then removes the known index. Neither may execute.
func TestDockerLabCommandKernelRefusesUnknownModules(t *testing.T) {
	tree := dockerLabRuntimeFixture(t)
	second := filepath.Join(tree, "lib", "modules", "unknown")
	if err := os.Mkdir(second, 0o700); err != nil {
		t.Fatal(err)
	}
	if selected, err := dockerLabCommandKernel(filepath.Join(tree, "vmlinuz"), tree); selected != nil || err == nil || !strings.Contains(err.Error(), "not one known set") {
		t.Fatalf("unknown module release selected %+v, error %v", selected, err)
	}
	if err := os.Remove(second); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(tree, "lib", "modules", "7.3.0-ze", "modules.dep")); err != nil {
		t.Fatal(err)
	}
	if selected, err := dockerLabCommandKernel(filepath.Join(tree, "vmlinuz"), tree); selected != nil || err == nil || !strings.Contains(err.Error(), "module index") {
		t.Fatalf("missing module index selected %+v, error %v", selected, err)
	}
}

// TestDockerLabCommandRefusesMismatchedImage drives the VM consumer with a real
// FAT boot partition carrying a foreign kernel. No QEMU child may start.
func TestDockerLabCommandRefusesMismatchedImage(t *testing.T) {
	tree := dockerLabRuntimeFixture(t)
	kernel, err := dockerLabCommandKernel(filepath.Join(tree, "vmlinuz"), tree)
	if err != nil {
		t.Fatal(err)
	}
	image := dockerLabImageFixture(t, "foreign-kernel", "root=PARTUUID=fixture-02 ro init=/gokrazy/init\n")
	run := NewRun(t.TempDir(), RunOptions{Kernel: kernel.Selected, Command: "true"})
	run.ops.Start = func(*exec.Cmd) error {
		t.Fatal("a mismatched image started a child")
		return nil
	}
	_, err = run.dockerLabCommand(t.Context(), image, kernel)
	if err == nil || !strings.Contains(err.Error(), "not selected runtime") {
		t.Fatalf("consumer accepted foreign embedded kernel: %v", err)
	}
}

// TestDockerLabCommandUsesImageRootIdentity reads an image-authored PARTUUID
// through the actual boot consumer and checks QEMU directly boots the request.
func TestDockerLabCommandUsesImageRootIdentity(t *testing.T) {
	tree := dockerLabRuntimeFixture(t)
	kernel, err := dockerLabCommandKernel(filepath.Join(tree, "vmlinuz"), tree)
	if err != nil {
		t.Fatal(err)
	}
	const root = "root=PARTUUID=60c24cc1-f3f9-427a-8199-123456780001/PARTNROFF=1"
	image := dockerLabImageFixture(t, "runtime-kernel", root+" ro init=/gokrazy/init\n")
	cmdline, err := dockerLabImageBoot(image, kernel)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(cmdline, root) || strings.Contains(cmdline, "/dev/vda2") {
		t.Fatalf("root identity was guessed or dropped: %q", cmdline)
	}
	vm := dockerLabVM{goarch: "amd64", kernel: kernel.Selected, cmdline: cmdline}
	argv := strings.Join(vm.argv(), " ")
	if !strings.Contains(argv, "-kernel "+kernel.Selected+" -append "+cmdline) {
		t.Fatalf("QEMU did not directly boot the selected kernel and image command line: %s", argv)
	}
	withoutRoot := dockerLabImageFixture(t, "runtime-kernel", "root=/dev/sda2 ro init=/gokrazy/init\n")
	if _, err := dockerLabImageBoot(withoutRoot, kernel); err == nil || !strings.Contains(err.Error(), "no root PARTUUID") {
		t.Fatalf("unknown image root mapping was accepted: %v", err)
	}
}

// TestDockerLabCommandCidataUsesOwnerWithoutDocker passes command cidata through
// the guest's real parser, then compares the normal lab's unchanged UID.
func TestDockerLabCommandCidataUsesOwnerWithoutDocker(t *testing.T) {
	for _, command := range []bool{false, true} {
		guest := dockerLabGuest{instanceID: "command-fixture", publicKey: "ssh-ed25519 AAAAtest", command: command}
		dir := t.TempDir()
		var boot string
		for _, file := range guest.cidataFiles() {
			if err := os.WriteFile(filepath.Join(dir, file.name), file.body, 0o600); err != nil {
				t.Fatal(err)
			}
			if file.name == "boot.sh" {
				boot = string(file.body)
			}
		}
		// The parser correctly refuses host root as a command UID. Native proof
		// explicitly requires the ordinary owner; normal lab UID is always 1001.
		if command && os.Getuid() == 0 {
			if _, err := dockerhost.ParseCidata(dir); err == nil {
				t.Fatal("root command UID was accepted")
			}
			continue
		}
		parsed, err := dockerhost.ParseCidata(dir)
		if err != nil {
			t.Fatal(err)
		}
		want := 1001
		if command {
			want = os.Getuid()
		}
		if parsed.UID != uint32(want) {
			t.Fatalf("command=%v UID=%d want %d", command, parsed.UID, want)
		}
		if strings.Contains(boot, "systemctl start") == command {
			t.Fatalf("command=%v daemon startup script %q", command, boot)
		}
	}
}

// dockerLabRuntimeFixture supplies a known byte identity and one module index.
func dockerLabRuntimeFixture(t *testing.T) string {
	t.Helper()
	tree := t.TempDir()
	modules := filepath.Join(tree, "lib", "modules", "7.3.0-ze")
	if err := os.MkdirAll(modules, 0o700); err != nil {
		t.Fatal(err)
	}
	for path, body := range map[string]string{
		filepath.Join(tree, "vmlinuz"):        "runtime-kernel",
		filepath.Join(modules, "modules.dep"): "kernel/fixture.ko:\n",
	} {
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return tree
}

// TestDockerLabCommandTransportKeepsUnprivilegedInterrupt executes the existing
// command lifecycle and its interrupt sender over recorded SSH commands. Both
// sessions must select the same per-run key and non-root user.
func TestDockerLabCommandTransportKeepsUnprivilegedInterrupt(t *testing.T) {
	run := NewRun(t.TempDir(), RunOptions{Command: "exit 23", Timeout: time.Minute})
	session := dockerLabSession{run: run, key: "/run/owned-key", port: 2222}
	report := RunReport{Plan: RunPlan{
		SSHPort: 2222, SetupCommand: guestRunParentSetup(guestRunJobParent),
		commandSession: &session,
	}}
	calls := 0
	run.ops.Run = func(_ context.Context, spec commandSpec) (commandResult, error) {
		calls++
		line := strings.Join(spec.Args, " ")
		for _, want := range []string{"-i /run/owned-key", "-p 2222", "ze@127.0.0.1"} {
			if !strings.Contains(line, want) {
				t.Errorf("SSH transport lacks %q: %s", want, line)
			}
		}
		if strings.Contains(line, "root@") {
			t.Errorf("command or interrupt used a privileged SSH identity: %s", line)
		}
		if calls == 1 {
			for _, want := range []string{"ZE_RUN_JOB", "exit 23"} {
				if !strings.Contains(line, want) {
					t.Errorf("command setup lacks %q: %s", want, line)
				}
			}
			return commandResult{Code: 23}, nil
		}
		if !strings.Contains(line, "kill -INT -$pgid") {
			t.Errorf("second SSH session did not forward the group interrupt: %s", line)
		}
		return commandResult{Stdout: "42\n"}, nil
	}
	ended, err := run.runGuestCommand(t.Context(), &report)
	if err != nil || ended.GuestExitCode != 23 {
		t.Fatalf("guest command status was lost: %+v, %v", ended, err)
	}
	if group, err := run.interruptGuest(t.Context(), &report.Plan); err != nil || group != "42" {
		t.Fatalf("unprivileged interrupt failed: group=%q err=%v", group, err)
	}
	if calls != 2 {
		t.Fatalf("got %d SSH commands, want command and interrupt", calls)
	}
}

// dockerLabImageFixture puts the real FAT writer's output at GPT partition 1.
func dockerLabImageFixture(t *testing.T, kernel, cmdline string) string {
	t.Helper()
	var partition bytes.Buffer
	writer, err := fat.NewWriter(&partition)
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range []struct{ name, body string }{{"/vmlinuz", kernel}, {"/cmdline.txt", cmdline}} {
		out, err := writer.File(file.name, time.Unix(0, 0))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := out.Write([]byte(file.body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Flush(); err != nil {
		t.Fatal(err)
	}
	image := filepath.Join(t.TempDir(), "docker-host.img")
	writeGPTFixture(t, image, 0)
	file, err := os.OpenFile(image, os.O_RDWR, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close() //nolint:errcheck // test fixture
	if _, err := file.WriteAt(partition.Bytes(), 2048*512); err != nil {
		t.Fatal(err)
	}
	var last [8]byte
	binary.LittleEndian.PutUint64(last[:], 2048+uint64((partition.Len()+511)/512)-1)
	if _, err := file.WriteAt(last[:], 1024+40); err != nil {
		t.Fatal(err)
	}
	return image
}
