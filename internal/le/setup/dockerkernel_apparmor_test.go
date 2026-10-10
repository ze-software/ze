// Design: docs/architecture/testing/interop.md -- the Docker host kernel check
// Related: dockerkernel_apparmor.go -- the action these tests drive
//
// `le setup docker-kernel apparmor` loads any registered Ze AppArmor profile on
// a Linux Docker host. Its root steps never run here: the tests read the plan,
// the refusals and the commands the lab refusals name. interoplab registers the
// generic lab profile ze-lab-net-sysctl, so the registry holds more than the
// probe's profile, as it does in the le binary.

package setup

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/kernelcap"
	"github.com/ze-software/ze/internal/le/interoplab"
	leaction "github.com/ze-software/ze/internal/le/le/action"
)

// VALIDATES: AC-19 (D-7). The command every lab AppArmor refusal names is this
// action, spelled with its own verb, keyword and the profile's name, for every
// registered profile, and the action is registered.
// PREVENTS: a refusal naming a command that does not exist.
func TestAppArmorLoadCommandIsThisAction(t *testing.T) {
	for _, name := range interoplab.AppArmorProfileNames() {
		want := "./le setup docker-kernel " + dockerKernelAppArmorVerb + " " + dockerKernelConfirmKeyword + " " + name
		if got := interoplab.AppArmorLoadCommandFor(name); got != want {
			t.Errorf("the refusal names %q, the action is %q", got, want)
		}
	}
	if !slices.ContainsFunc(DockerKernelActions().Actions, func(action leaction.Row) bool { return action.Verb == dockerKernelAppArmorVerb }) {
		t.Errorf("verb %q is not registered", dockerKernelAppArmorVerb)
	}
}

// VALIDATES: AC-19. Off Linux the action refuses naming the macOS route; on a
// host without AppArmor enabled, or without apparmor_parser, it refuses naming
// what is missing; a host with both is ready.
// PREVENTS: root steps that cannot succeed, run before the reason is known.
func TestAppArmorLoadRefusesAHostThatCannotLoadIt(t *testing.T) {
	if err := appArmorLoadPlatform("darwin"); err == nil || !strings.Contains(err.Error(), "Linux") {
		t.Errorf("darwin: %v, want a Linux-only refusal", err)
	}
	if err := appArmorLoadPlatform("linux"); err != nil {
		t.Errorf("linux refused: %v", err)
	}
	for name, tc := range map[string]struct {
		enabled string
		parser  string
		want    string
	}{
		"disabled":         {"N\n", "/usr/sbin/apparmor_parser", "not enabled"},
		"no module":        {"", "/usr/sbin/apparmor_parser", "not enabled"},
		"no parser":        {"Y\n", "", "apparmor_parser"},
		"enabled, parsing": {"Y\n", "/usr/sbin/apparmor_parser", ""},
	} {
		t.Run(name, func(t *testing.T) {
			err := appArmorHostReady(tc.enabled, tc.parser)
			if tc.want == "" {
				if err != nil {
					t.Errorf("a ready host refused: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("got %v, want a refusal naming %q", err, tc.want)
			}
		})
	}
}

// VALIDATES: AC-19. The plan installs the profile into /etc/apparmor.d, so it
// loads again at boot, then replaces the loaded copy and writes the cache; the
// file it installs holds the text the profile registers, for every profile.
// PREVENTS: a profile loaded for this boot only, and a second copy of the text.
func TestAppArmorLoadSteps(t *testing.T) {
	for _, name := range []string{kernelcap.ProbeAppArmorProfileName, interoplab.NetSysctlAppArmorProfileName} {
		t.Run(name, func(t *testing.T) {
			profile, registered := interoplab.LookupAppArmorProfile(name)
			if !registered {
				t.Fatalf("profile %s is not registered", name)
			}
			dir := t.TempDir()
			staged, err := stageAppArmorProfile(dir, profile)
			if err != nil {
				t.Fatalf("stage the profile: %v", err)
			}
			data, err := os.ReadFile(staged)
			if err != nil {
				t.Fatalf("read the staged profile: %v", err)
			}
			if string(data) != profile.Text() {
				t.Error("the staged profile is not the registered text")
			}
			if !strings.Contains(string(data), "profile "+name+" ") {
				t.Errorf("the staged text does not declare profile %s", name)
			}
			target := filepath.Join("/etc/apparmor.d", name)
			steps := appArmorLoadSteps(profile, staged, "/usr/sbin/apparmor_parser")
			got := make([]string, 0, len(steps))
			for _, step := range steps {
				if step.Why == "" {
					t.Errorf("step %v carries no reason", step.Argv)
				}
				got = append(got, strings.Join(step.Argv, " "))
			}
			want := []string{
				"sudo install -m 0644 " + staged + " " + target,
				"sudo /usr/sbin/apparmor_parser -r -W " + target,
			}
			if !slices.Equal(got, want) {
				t.Errorf("steps:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
			}
		})
	}
}

// VALIDATES: AC-19. Without `confirm <profile>` naming a registered profile
// nothing runs, and the refusal names the command that loads each registered
// profile; every registered profile's own name is accepted and answers that
// profile.
// PREVENTS: a lab refusal naming `confirm ze-lab-net-sysctl`, which this action then
// refuses.
func TestAppArmorLoadNeedsConfirmation(t *testing.T) {
	for _, value := range []string{"", "docker-default", "ze-kernel-probe-old"} {
		_, err := appArmorLoadConfirmed(value)
		if err == nil {
			t.Errorf("confirm %q ran the steps", value)
			continue
		}
		for _, name := range interoplab.AppArmorProfileNames() {
			if !strings.Contains(err.Error(), interoplab.AppArmorLoadCommandFor(name)) {
				t.Errorf("confirm %q: the refusal does not name the command for %s: %v", value, name, err)
			}
		}
	}
	for _, name := range []string{kernelcap.ProbeAppArmorProfileName, interoplab.NetSysctlAppArmorProfileName} {
		profile, err := appArmorLoadConfirmed(name)
		if err != nil {
			t.Errorf("the profile's own name %s refused: %v", name, err)
			continue
		}
		if profile.Name != name {
			t.Errorf("confirm %s answered profile %q", name, profile.Name)
		}
	}
}
