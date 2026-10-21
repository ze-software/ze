// Design: docs/architecture/testing/interop.md -- the Docker host kernel check
// Related: dockerkernel_apparmor_colima.go -- the macOS half these tests drive
//
// On macOS `le setup docker-kernel apparmor` loads a Ze profile into the Colima
// VM that runs the Docker daemon, through `colima ssh`. Every command it runs
// before the root steps goes through a scripted runner here, and the root
// steps are read as a plan, never run.

package dockerkernel

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/kernelcap"
	"github.com/ze-software/ze/internal/le/interoplab"
)

// colimaVMSSH is the prefix every command into the default profile's VM takes.
const colimaVMSSH = "colima ssh --profile default -- "

// scriptedVM answers each argv, joined by spaces, from replies, records every
// argv it was asked, and fails a test on an argv it holds no reply for. A
// `test -r` of any path is answered by readable when it is set, because the
// action stages under a directory whose name it draws at random.
type scriptedVM struct {
	t        *testing.T
	replies  map[string]vmResult
	readable *vmResult
	asked    []string
}

func (s *scriptedVM) run(_ context.Context, argv []string) (vmResult, error) {
	line := strings.Join(argv, " ")
	s.asked = append(s.asked, line)
	if s.readable != nil && strings.HasPrefix(line, colimaVMSSH+"test -r ") {
		return *s.readable, nil
	}
	reply, scripted := s.replies[line]
	if !scripted {
		s.t.Errorf("unscripted command: %s", line)
		return vmResult{}, errors.New("unscripted command")
	}
	return reply, nil
}

// readyColimaReplies answers a running default Colima VM with AppArmor enabled
// and apparmor_parser in /usr/sbin.
func readyColimaReplies() map[string]vmResult {
	return map[string]vmResult{
		dockerEndpointCommand:                                               {Stdout: "unix:///Users/me/.colima/default/docker.sock\n"},
		colimaVMSSH + "true":                                                {},
		colimaVMSSH + "cat " + appArmorEnabledPath:                          {Stdout: "Y\n"},
		colimaVMSSH + "test -x /usr/sbin/apparmor_parser":                   {},
		colimaVMSSH + "test -r /Users/me/ze/tmp/apparmor-1/ze-kernel-probe": {},
	}
}

// colimaHome answers true for the default profile's colima.yaml under
// /Users/me/.colima, the one Colima home these tests script.
func colimaHome(path string) bool { return path == "/Users/me/.colima/default/colima.yaml" }

// VALIDATES: owner decision 2026-10-21 ("find the colima issue so that setup
// can have it working"). On macOS the action finds the Colima VM behind the
// client's Docker endpoint and asks that VM, not the Mac, whether AppArmor is
// enabled and where apparmor_parser is; it refuses an endpoint that is not
// Colima's, a VM it cannot reach, a VM kernel without AppArmor and a VM
// without the parser, each naming the fix.
// METHOD: a scripted runner answers every command for each case.
// PREVENTS: a profile loaded into a VM the daemon does not run in, and root
// steps that cannot succeed.
func TestAppArmorColimaHost(t *testing.T) {
	ready := &scriptedVM{t: t, replies: readyColimaReplies()}
	host, err := appArmorDarwinHost(t.Context(), ready.run, colimaHome)
	if err != nil {
		t.Fatalf("a ready Colima VM refused: %v", err)
	}
	if got, want := strings.Join(host.via, " "), strings.TrimSuffix(colimaVMSSH, " "); got != want {
		t.Errorf("the steps run through %q, want %q", got, want)
	}
	if host.profile != "default" {
		t.Errorf("profile %q, want default", host.profile)
	}
	if host.parser != "/usr/sbin/apparmor_parser" {
		t.Errorf("parser %q, want /usr/sbin/apparmor_parser", host.parser)
	}
	wantAsked := []string{
		dockerEndpointCommand,
		colimaVMSSH + "true",
		colimaVMSSH + "cat " + appArmorEnabledPath,
		colimaVMSSH + "test -x /usr/sbin/apparmor_parser",
	}
	if !slices.Equal(ready.asked, wantAsked) {
		t.Errorf("asked:\n%s\nwant:\n%s", strings.Join(ready.asked, "\n"), strings.Join(wantAsked, "\n"))
	}

	for name, tc := range map[string]struct {
		edit func(map[string]vmResult)
		want []string
	}{
		"Docker Desktop": {
			edit: func(r map[string]vmResult) {
				r[dockerEndpointCommand] = vmResult{Stdout: "unix:///Users/me/.docker/run/docker.sock\n"}
			},
			want: []string{"unix:///Users/me/.docker/run/docker.sock", "Colima", "\n  ./le setup docker-host start"},
		},
		"no endpoint": {
			edit: func(r map[string]vmResult) {
				r[dockerEndpointCommand] = vmResult{Exit: 1, Stderr: "context not found"}
			},
			want: []string{dockerEndpointCommand, "context not found", "\n  ./le setup docker-host start"},
		},
		"VM stopped": {
			edit: func(r map[string]vmResult) {
				r[colimaVMSSH+"true"] = vmResult{Exit: 1, Stderr: "colima is not running"}
			},
			want: []string{"could not reach", "colima is not running", "colima start --profile default"},
		},
		"no AppArmor module": {
			edit: func(r map[string]vmResult) {
				r[colimaVMSSH+"cat "+appArmorEnabledPath] = vmResult{Exit: 1, Stderr: "No such file or directory"}
			},
			want: []string{"AppArmor is not enabled"},
		},
		"AppArmor disabled": {
			edit: func(r map[string]vmResult) {
				r[colimaVMSSH+"cat "+appArmorEnabledPath] = vmResult{Stdout: "N\n"}
			},
			want: []string{"not enabled"},
		},
		"no parser": {
			edit: func(r map[string]vmResult) {
				r[colimaVMSSH+"test -x /usr/sbin/apparmor_parser"] = vmResult{Exit: 1}
				r[colimaVMSSH+"test -x /sbin/apparmor_parser"] = vmResult{Exit: 1}
			},
			want: []string{"apparmor_parser"},
		},
	} {
		t.Run(name, func(t *testing.T) {
			replies := readyColimaReplies()
			tc.edit(replies)
			vm := &scriptedVM{t: t, replies: replies}
			_, err := appArmorDarwinHost(t.Context(), vm.run, colimaHome)
			if err == nil {
				t.Fatal("the action proceeded")
			}
			for _, want := range tc.want {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("the refusal lacks %q:\n%v", want, err)
				}
			}
		})
	}
}

// VALIDATES: the two root steps run inside the Colima VM, each prefixed by the
// VM's `colima ssh`, and install the staged file the VM reads through Colima's
// mount of the home directory; a staged file the VM cannot read refuses before
// any root step, naming the mount.
// METHOD: the plan is read for both profiles; the visibility check runs over a
// scripted runner that answers readable, then unreadable.
// PREVENTS: a sudo step run on the Mac, which has no AppArmor, and an install
// that fails half way because the VM cannot see the file.
func TestAppArmorLoadStepsThroughColima(t *testing.T) {
	host := appArmorHost{via: strings.Fields(colimaVMSSH), parser: "/usr/sbin/apparmor_parser", profile: "default"}
	staged := "/Users/me/ze/tmp/apparmor-1/" + kernelcap.ProbeAppArmorProfileName
	profile, registered := interoplab.LookupAppArmorProfile(kernelcap.ProbeAppArmorProfileName)
	if !registered {
		t.Fatal("ze-kernel-probe is not registered")
	}
	got := make([]string, 0, 2)
	for _, step := range appArmorLoadSteps(profile, staged, host) {
		got = append(got, strings.Join(step.Argv, " "))
	}
	want := []string{
		colimaVMSSH + "sudo install -m 0644 " + staged + " /etc/apparmor.d/ze-kernel-probe",
		colimaVMSSH + "sudo /usr/sbin/apparmor_parser -r -W /etc/apparmor.d/ze-kernel-probe",
	}
	if !slices.Equal(got, want) {
		t.Errorf("steps:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}

	visible := &scriptedVM{t: t, replies: readyColimaReplies()}
	if err := appArmorStagedVisible(t.Context(), host, staged, visible.run); err != nil {
		t.Errorf("a file the VM reads refused: %v", err)
	}
	hidden := readyColimaReplies()
	hidden[colimaVMSSH+"test -r "+staged] = vmResult{Exit: 1}
	vm := &scriptedVM{t: t, replies: hidden}
	err := appArmorStagedVisible(t.Context(), host, staged, vm.run)
	if err == nil {
		t.Fatal("a file the VM cannot read did not refuse")
	}
	for _, want := range []string{staged, "mount", "colima start --profile default --edit"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal lacks %q:\n%v", want, err)
		}
	}
	local := &scriptedVM{t: t, replies: map[string]vmResult{}}
	if err := appArmorStagedVisible(t.Context(), appArmorHost{parser: "/usr/sbin/apparmor_parser"}, staged, local.run); err != nil {
		t.Errorf("a Linux host, which reads its own file, refused: %v", err)
	}
	if len(local.asked) != 0 {
		t.Errorf("a Linux host was asked %q", local.asked)
	}
}

// VALIDATES: a question whose program is not installed names the Homebrew
// formula, and one that hangs names the command and the status command of the
// profile it asked, or `colima list` for the Docker client's question.
// PREVENTS: a raw exec error or a bare timeout with no next step.
func TestVMCommandRefusals(t *testing.T) {
	for program, argv := range map[string][]string{
		"colima": append(strings.Fields(colimaVMSSH), "true"),
		"docker": dockerEndpointArgv,
	} {
		if err := vmCommandMissing(argv); !strings.Contains(err.Error(), "\n  brew install "+program) {
			t.Errorf("%s missing: %v", program, err)
		}
	}
	hung := vmCommandHung(append(strings.Fields(colimaVMSSH), "cat", appArmorEnabledPath))
	for _, want := range []string{colimaVMSSH + "cat " + appArmorEnabledPath, "\n  colima status --profile default"} {
		if !strings.Contains(hung.Error(), want) {
			t.Errorf("the hung refusal lacks %q: %v", want, hung)
		}
	}
	if hung := vmCommandHung(dockerEndpointArgv); !strings.Contains(hung.Error(), "\n  colima list") {
		t.Errorf("the hung endpoint question does not name colima list: %v", hung)
	}
}

// VALIDATES: the action's entry point takes the darwin route through the
// runners it is given: without `confirm` it asks the Colima VM, prints the
// plan and runs no root step; a Docker endpoint that is not Colima's refuses
// before any step; a staged profile the VM cannot read refuses before any
// step; a confirmed profile the VM reads runs the two root steps inside the
// VM, in order.
// METHOD: appArmorLoad runs with goos darwin, a scripted VM and a step runner
// that records each root step instead of running it.
// PREVENTS: a darwin action that bypasses the Colima checks, or runs sudo
// before the VM can read the file it installs.
func TestAppArmorLoadOnDarwin(t *testing.T) {
	load := func(t *testing.T, confirm string, edit func(*scriptedVM)) ([]string, error) {
		t.Helper()
		vm := &scriptedVM{t: t, replies: readyColimaReplies(), readable: &vmResult{}}
		if edit != nil {
			edit(vm)
		}
		var steps []string
		record := func(_ context.Context, step installStep) error {
			steps = append(steps, strings.Join(step.Argv, " "))
			return nil
		}
		_, err := appArmorLoad(t.Context(), confirm, appArmorEnv{goos: osDarwin, run: vm.run, exists: colimaHome, step: record})
		return steps, err
	}
	probe := kernelcap.ProbeAppArmorProfileName

	steps, err := load(t, "", nil)
	if err == nil || !strings.Contains(err.Error(), interoplab.AppArmorLoadCommandFor(probe)) {
		t.Errorf("without confirm: %v, want the refusal naming the load command", err)
	}
	if len(steps) != 0 {
		t.Errorf("without confirm the action ran %q", steps)
	}

	steps, err = load(t, probe, func(vm *scriptedVM) {
		vm.replies[dockerEndpointCommand] = vmResult{Stdout: "unix:///Users/me/.docker/run/docker.sock\n"}
	})
	if err == nil || !strings.Contains(err.Error(), "not a Colima VM's socket") {
		t.Errorf("Docker Desktop endpoint: %v, want the Colima refusal", err)
	}
	if len(steps) != 0 {
		t.Errorf("a non-Colima endpoint ran %q", steps)
	}

	steps, err = load(t, probe, func(vm *scriptedVM) { vm.readable = &vmResult{Exit: 1} })
	if err == nil || !strings.Contains(err.Error(), "cannot read the staged profile") {
		t.Errorf("hidden staged file: %v, want the mount refusal", err)
	}
	if len(steps) != 0 {
		t.Errorf("a staged file the VM cannot read ran %q", steps)
	}

	steps, err = load(t, probe, nil)
	if err != nil {
		t.Fatalf("a confirmed load refused: %v", err)
	}
	if len(steps) != 2 {
		t.Fatalf("a confirmed load ran %d steps, want 2: %q", len(steps), steps)
	}
	for i, want := range []string{"sudo install -m 0644 ", "sudo /usr/sbin/apparmor_parser -r -W /etc/apparmor.d/ze-kernel-probe"} {
		if !strings.HasPrefix(steps[i], colimaVMSSH) || !strings.Contains(steps[i], want) {
			t.Errorf("step %d is %q, want %q inside the VM", i, steps[i], want)
		}
	}
}
