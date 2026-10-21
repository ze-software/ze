// Tests for the docker-buildx step: the BuildKit plugin the interop labs need
// to build an image with a named build context.
//
// Each case drives the step through a fake Shell whose `docker buildx version`
// answers from the docker config file the step writes, the way the real docker
// CLI finds a plugin only once `cliPluginsExtraDirs` names its directory. No
// case starts a process or reads the real ~/.docker.

package setup

import (
	"bytes"
	"context"
	"encoding/json"
	"net/netip"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// buildxFake is a host with docker on PATH and, optionally, Homebrew, whose
// buildx plugin answers once the docker config names pluginDir.
type buildxFake struct {
	brew         bool
	prefix       string // what `brew --prefix` prints
	installed    bool   // whether `brew list --versions docker-buildx` succeeds
	dockerCE     string // the dpkg status line for docker-ce; empty means dpkg does not know it
	installFails bool   // brew install fails
	prefixFails  bool   // brew --prefix fails
	neverAnswers bool   // the plugin never answers, whatever the config says
	config       string // the docker config file the CLI reads
	calls        []string
}

// shell builds the seam the step runs through.
func (f *buildxFake) shell() *Shell {
	return &Shell{
		Look: func(name string) (string, bool) {
			if name == toolDocker {
				return "/test/bin/docker", true
			}
			if name == brewBin && f.brew {
				return f.prefix + "/bin/brew", true
			}
			return "", false
		},
		Exec: func(_ context.Context, cmd Cmd) Result {
			line := strings.Join(cmd.Argv, " ")
			f.calls = append(f.calls, line)
			switch line {
			case "docker buildx version":
				if f.pluginVisible() {
					return Result{Argv: cmd.Argv, Out: "github.com/docker/buildx v0.38.0 abc\n"}
				}
				return Result{Argv: cmd.Argv, Code: 1, Err: "unknown command: docker buildx"}
			case "brew --prefix":
				if f.prefixFails {
					return Result{Argv: cmd.Argv, Code: 1, Err: "Error: brew is broken"}
				}
				return Result{Argv: cmd.Argv, Out: f.prefix + "\n"}
			case "brew list --versions docker-buildx":
				if f.installed {
					return Result{Argv: cmd.Argv, Out: "docker-buildx 0.38.0\n"}
				}
				return Result{Argv: cmd.Argv, Code: 1}
			case "brew install docker-buildx":
				if f.installFails {
					return Result{Argv: cmd.Argv, Code: 1, Err: "Error: No such keg"}
				}
				f.installed = true
				return Result{Argv: cmd.Argv}
			case "dpkg-query -W -f=${Status} docker-ce":
				if f.dockerCE == "" {
					return Result{Argv: cmd.Argv, Code: 1, Err: "dpkg-query: no packages found matching docker-ce"}
				}
				return Result{Argv: cmd.Argv, Out: f.dockerCE}
			}
			return Result{Argv: cmd.Argv, Code: 127, Err: "unexpected command in test: " + line}
		},
		Euid: func() int { return 1000 },
		Tty:  func() bool { return false },
	}
}

// pluginVisible reports whether the docker CLI would find the plugin: brew has
// installed it and the config names its directory.
func (f *buildxFake) pluginVisible() bool {
	if f.neverAnswers {
		return false
	}
	if !f.installed {
		return false
	}
	data, err := os.ReadFile(f.config)
	if err != nil {
		return false
	}
	var config struct {
		Dirs []string `json:"cliPluginsExtraDirs"`
	}
	if json.Unmarshal(data, &config) != nil {
		return false
	}
	return slices.Contains(config.Dirs, f.prefix+"/lib/docker/cli-plugins")
}

// ran reports whether the step ran this exact command line.
func (f *buildxFake) ran(line string) bool { return slices.Contains(f.calls, line) }

// newBuildxCase builds a Setup over the fake, with its home in a temporary
// directory so the docker config lives there.
func newBuildxCase(t *testing.T, goos string, check bool, fake *buildxFake) *Setup {
	t.Helper()
	home := t.TempDir()
	fake.config = filepath.Join(home, ".docker", "config.json")
	return &Setup{
		Check: check,
		GOOS:  goos,
		Home:  home,
		Env:   func(string) string { return "" },
		Shell: fake.shell(),
	}
}

// writeConfig puts content in the fake's docker config file.
func writeConfig(t *testing.T, fake *buildxFake, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(fake.config), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fake.config, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

// readConfig answers the fake's docker config, decoded key by key.
func readConfig(t *testing.T, fake *buildxFake) map[string]json.RawMessage {
	t.Helper()
	data, err := os.ReadFile(fake.config)
	if err != nil {
		t.Fatalf("the docker config was not written: %v", err)
	}
	config := map[string]json.RawMessage{}
	if err := json.Unmarshal(data, &config); err != nil {
		t.Fatalf("the docker config is not JSON after the step: %v\n%s", err, data)
	}
	return config
}

// pluginDirs answers the cliPluginsExtraDirs list of a decoded config.
func pluginDirs(t *testing.T, config map[string]json.RawMessage) []string {
	t.Helper()
	var dirs []string
	if err := json.Unmarshal(config["cliPluginsExtraDirs"], &dirs); err != nil {
		t.Fatalf("cliPluginsExtraDirs is not a list of strings: %v", err)
	}
	return dirs
}

// TestBuildxInstallPreservesOtherConfigKeys proves the config edit is a
// read-modify-write.
// VALIDATES: on macOS the step installs the formula, appends the Homebrew
// plugin directory after an entry already there, keeps credentials, contexts
// and every other key with its value, and ends with a plugin that answers.
// PREVENTS: an edit that rewrites ~/.docker/config.json from scratch and logs
// the owner out of every registry.
func TestBuildxInstallPreservesOtherConfigKeys(t *testing.T) {
	fake := &buildxFake{brew: true, prefix: "/opt/homebrew"}
	s := newBuildxCase(t, osDarwin, false, fake)
	writeConfig(t, fake, `{
	"auths": {"ghcr.io": {"auth": "c2VjcmV0"}},
	"credsStore": "osxkeychain",
	"currentContext": "colima",
	"cliPluginsExtraDirs": ["/elsewhere/cli-plugins"]
}`)

	report := &Report{}
	outcome := s.visitBuildx(report)

	if outcome.State != StateInstalled {
		t.Fatalf("state %q (%s), want %q\n%s", outcome.State, outcome.Detail, StateInstalled, report.Text())
	}
	if !fake.ran("brew install docker-buildx") {
		t.Errorf("the formula was not installed; ran %q", fake.calls)
	}
	config := readConfig(t, fake)
	for key, want := range map[string]string{
		"auths":          `{"ghcr.io": {"auth": "c2VjcmV0"}}`,
		"credsStore":     `"osxkeychain"`,
		"currentContext": `"colima"`,
	} {
		// The value is compared, not its whitespace: the file is re-indented,
		// as the docker CLI itself does when it writes the file.
		var got, wanted bytes.Buffer
		if err := json.Compact(&got, config[key]); err != nil {
			t.Fatalf("key %q: %v", key, err)
		}
		if err := json.Compact(&wanted, []byte(want)); err != nil {
			t.Fatal(err)
		}
		if got.String() != wanted.String() {
			t.Errorf("key %q is %s after the edit, want %s", key, got.String(), wanted.String())
		}
	}
	if len(config) != 4 {
		t.Errorf("the config holds %d keys after the edit, want 4: %v", len(config), config)
	}
	want := []string{"/elsewhere/cli-plugins", "/opt/homebrew/lib/docker/cli-plugins"}
	if got := pluginDirs(t, config); !slices.Equal(got, want) {
		t.Errorf("cliPluginsExtraDirs = %q, want %q", got, want)
	}
	info, err := os.Stat(fake.config)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("the config mode is %v after the edit, want the original 0600", info.Mode().Perm())
	}
}

// TestBuildxEntryAlreadyPresentIsNotDuplicated proves the edit is idempotent.
// VALIDATES: when the config already names the plugin directory, the file is
// left byte for byte as it was, and a second install run changes nothing.
// PREVENTS: a list that grows one duplicate entry on every `setup install`.
func TestBuildxEntryAlreadyPresentIsNotDuplicated(t *testing.T) {
	fake := &buildxFake{brew: true, prefix: "/opt/homebrew"}
	s := newBuildxCase(t, osDarwin, false, fake)
	original := `{"cliPluginsExtraDirs": ["/opt/homebrew/lib/docker/cli-plugins"], "credsStore": "desktop"}`
	writeConfig(t, fake, original)

	outcome := s.visitBuildx(&Report{})
	if outcome.State != StateInstalled {
		t.Fatalf("state %q (%s), want %q", outcome.State, outcome.Detail, StateInstalled)
	}
	data, err := os.ReadFile(fake.config)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != original {
		t.Errorf("the config was rewritten although it already named the directory:\n%s", data)
	}

	// A second run finds the plugin answering and touches nothing.
	fake.calls = nil
	if again := s.visitBuildx(&Report{}); again.State != StatePresent {
		t.Errorf("second run: state %q (%s), want %q", again.State, again.Detail, StatePresent)
	}
	if !slices.Equal(fake.calls, []string{"docker buildx version"}) {
		t.Errorf("second run ran %q, want only the probe", fake.calls)
	}
}

// TestBuildxWithoutHomebrew proves the step refuses before it writes.
// VALIDATES: on a Mac with no brew the step changes nothing, fails the run,
// and names Homebrew and the formula as the fix.
// PREVENTS: a config entry pointing at a plugin directory nothing installed.
func TestBuildxWithoutHomebrew(t *testing.T) {
	fake := &buildxFake{brew: false}
	s := newBuildxCase(t, osDarwin, false, fake)

	report := &Report{}
	outcome := s.visitBuildx(report)

	if outcome.State != StatePending {
		t.Fatalf("state %q (%s), want %q", outcome.State, outcome.Detail, StatePending)
	}
	if _, err := os.Stat(fake.config); !os.IsNotExist(err) {
		t.Errorf("the docker config was written with no Homebrew to install the plugin: %v", err)
	}
	for _, line := range fake.calls {
		if strings.HasPrefix(line, "brew ") {
			t.Errorf("ran %q with no brew on PATH", line)
		}
	}
	text := report.Text()
	for _, want := range []string{"https://brew.sh", "brew install docker-buildx"} {
		if !strings.Contains(text, want) {
			t.Errorf("the report does not name %q:\n%s", want, text)
		}
	}
}

// TestBuildxIntelPrefix proves the plugin directory follows `brew --prefix`.
// VALIDATES: an Intel Mac, whose Homebrew lives under /usr/local, gets
// /usr/local/lib/docker/cli-plugins, and a missing config file is created.
// PREVENTS: the Apple silicon path written on a Mac where it does not exist.
func TestBuildxIntelPrefix(t *testing.T) {
	fake := &buildxFake{brew: true, prefix: "/usr/local", installed: true}
	s := newBuildxCase(t, osDarwin, false, fake)

	outcome := s.visitBuildx(&Report{})
	if outcome.State != StateInstalled {
		t.Fatalf("state %q (%s), want %q", outcome.State, outcome.Detail, StateInstalled)
	}
	if fake.ran("brew install docker-buildx") {
		t.Error("the formula was installed again although brew already holds it")
	}
	want := []string{"/usr/local/lib/docker/cli-plugins"}
	if got := pluginDirs(t, readConfig(t, fake)); !slices.Equal(got, want) {
		t.Errorf("cliPluginsExtraDirs = %q, want %q", got, want)
	}
}

// TestBuildxCheckChangesNothing proves the check mode is probe only.
// VALIDATES: `setup check` on a Mac without buildx reports MISSING, so the
// verdict points at `setup install`, and runs nothing but the probe.
// PREVENTS: a check that installs a formula or edits the docker config.
func TestBuildxCheckChangesNothing(t *testing.T) {
	fake := &buildxFake{brew: true, prefix: "/opt/homebrew"}
	s := newBuildxCase(t, osDarwin, true, fake)

	outcome := s.visitBuildx(&Report{})
	if outcome.State != StateMissing {
		t.Fatalf("state %q (%s), want %q", outcome.State, outcome.Detail, StateMissing)
	}
	if !slices.Equal(fake.calls, []string{"docker buildx version"}) {
		t.Errorf("check ran %q, want only the probe", fake.calls)
	}
	if _, err := os.Stat(fake.config); !os.IsNotExist(err) {
		t.Errorf("check wrote the docker config: %v", err)
	}
}

// TestBuildxLinuxNamesThePackage proves the Linux branch runs no root step.
// VALIDATES: on Linux the step names the package that matches the installed
// docker (docker-buildx-plugin beside Docker's docker-ce, docker-buildx beside
// the distribution's docker.io) and the sudo command, and runs neither.
// PREVENTS: setup running sudo for a package the owner did not ask it to add.
func TestBuildxLinuxNamesThePackage(t *testing.T) {
	for _, tc := range []struct {
		dockerCE string
		want     string
	}{
		{dockerCE: "install ok installed", want: "sudo apt-get install docker-buildx-plugin"},
		{dockerCE: "", want: "sudo apt-get install docker-buildx"},
		// A removed docker-ce leaves its config files: dpkg-query still
		// answers, and the package is not installed.
		{dockerCE: "deinstall ok config-files", want: "sudo apt-get install docker-buildx"},
	} {
		fake := &buildxFake{dockerCE: tc.dockerCE}
		s := newBuildxCase(t, osLinux, false, fake)

		report := &Report{}
		outcome := s.visitBuildx(report)
		if outcome.State != StatePending {
			t.Fatalf("docker-ce=%q: state %q (%s), want %q", tc.dockerCE, outcome.State, outcome.Detail, StatePending)
		}
		if !strings.Contains(report.Text(), tc.want+"\n") {
			t.Errorf("docker-ce=%q: the report does not name %q:\n%s", tc.dockerCE, tc.want, report.Text())
		}
		for _, line := range fake.calls {
			if strings.Contains(line, "sudo") || strings.Contains(line, "apt-get") {
				t.Errorf("docker-ce=%q: ran %q", tc.dockerCE, line)
			}
		}
	}
}

// TestBuildxNoDockerIsSkipped proves the step follows the optional docker row.
// VALIDATES: with no docker CLI the step is skipped and runs nothing.
// PREVENTS: a failed run on a host that never uses the labs.
func TestBuildxNoDockerIsSkipped(t *testing.T) {
	fake := &buildxFake{}
	s := newBuildxCase(t, osDarwin, false, fake)
	s.Shell.Look = func(string) (string, bool) { return "", false }

	if outcome := s.visitBuildx(&Report{}); outcome.State != StateSkipped {
		t.Errorf("state %q (%s), want %q", outcome.State, outcome.Detail, StateSkipped)
	}
	if len(fake.calls) != 0 {
		t.Errorf("ran %q with no docker on PATH", fake.calls)
	}
}

// TestBuildxRefusesAConfigItCannotRead proves an unreadable config is left
// alone.
// VALIDATES: a config that is not a JSON object is not overwritten, and the
// report names the file and the entry to add by hand.
// PREVENTS: replacing a file the owner can still repair with a fresh one.
func TestBuildxRefusesAConfigItCannotRead(t *testing.T) {
	fake := &buildxFake{brew: true, prefix: "/opt/homebrew", installed: true}
	s := newBuildxCase(t, osDarwin, false, fake)
	writeConfig(t, fake, `{"auths": {`)

	report := &Report{}
	outcome := s.visitBuildx(report)
	if outcome.State != StatePending {
		t.Fatalf("state %q (%s), want %q", outcome.State, outcome.Detail, StatePending)
	}
	data, err := os.ReadFile(fake.config)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != `{"auths": {` {
		t.Errorf("the unreadable config was overwritten:\n%s", data)
	}
	if !strings.Contains(report.Text(), `"cliPluginsExtraDirs": ["/opt/homebrew/lib/docker/cli-plugins"]`) {
		t.Errorf("the report does not give the entry to add:\n%s", report.Text())
	}
}

// TestBuildxHonoursDockerConfig proves the edit lands where docker reads.
// VALIDATES: DOCKER_CONFIG moves the config directory, and the step edits the
// file there rather than ~/.docker/config.json.
// PREVENTS: an entry written to a file this docker CLI never opens.
func TestBuildxHonoursDockerConfig(t *testing.T) {
	fake := &buildxFake{brew: true, prefix: "/opt/homebrew", installed: true}
	s := newBuildxCase(t, osDarwin, false, fake)
	dir := t.TempDir()
	s.Env = func(key string) string {
		if key == "DOCKER_CONFIG" {
			return dir
		}
		return ""
	}
	fake.config = filepath.Join(dir, "config.json")

	if outcome := s.visitBuildx(&Report{}); outcome.State != StateInstalled {
		t.Fatalf("state %q (%s), want %q", outcome.State, outcome.Detail, StateInstalled)
	}
	if _, err := os.Stat(filepath.Join(s.Home, ".docker", "config.json")); !os.IsNotExist(err) {
		t.Errorf("~/.docker/config.json was written although DOCKER_CONFIG names another directory: %v", err)
	}
}

// TestBuildxCheckRunFailsAndNamesTheStep proves Run visits the step.
// VALIDATES: `setup check` on a Mac whose docker has no buildx exits 1, and its
// verdict names docker-buildx and the install action.
// PREVENTS: a step that exists and that the command never runs.
func TestBuildxCheckRunFailsAndNamesTheStep(t *testing.T) {
	fake := &buildxFake{brew: true, prefix: "/opt/homebrew"}
	s := newBuildxCase(t, osDarwin, true, fake)
	answer := s.Shell.Exec
	// Every other tool is on PATH and every other command succeeds, so the
	// verdict is about this step.
	s.Shell.Look = func(name string) (string, bool) {
		if name == toolClaude {
			return "", false
		}
		return "/test/bin/" + name, true
	}
	s.Shell.Exec = func(ctx context.Context, cmd Cmd) Result {
		if strings.Join(cmd.Argv, " ") == "docker buildx version" {
			return answer(ctx, cmd)
		}
		return Result{Argv: cmd.Argv}
	}
	s.Gopls = func() Result { return Result{Out: "clock.go"} }
	s.LoopbackPrefixes = func() ([]netip.Prefix, error) { return nil, nil }
	s.Bindable = func(string) bool { return true }

	report, code := s.Run()
	if code != 1 {
		t.Fatalf("check answered %d with buildx missing, want 1\n%s", code, report.Text())
	}
	found := false
	for _, outcome := range report.Outcomes {
		if outcome.Name == buildxName {
			found = true
			if outcome.State != StateMissing {
				t.Errorf("docker-buildx is %q in the run, want %q", outcome.State, StateMissing)
			}
		}
	}
	if !found {
		t.Fatalf("the run has no docker-buildx row:\n%s", report.Text())
	}
	text := report.Text()
	for _, want := range []string{"Missing required tools: ", buildxName, "Install them with: " + BuildKitSetupCommand} {
		if !strings.Contains(text, want) {
			t.Errorf("the verdict does not name %q:\n%s", want, text)
		}
	}
}

// TestBuildxFailureNotesNameWhyAndFix proves every darwin failure is actionable.
// VALIDATES: a failed brew install, a brew that names no prefix, and a plugin
// that still does not answer each leave the run pending with a note stating
// the consequence and the command to run next.
// PREVENTS: a bare `FAIL` line the owner has to decode on a broken host.
func TestBuildxFailureNotesNameWhyAndFix(t *testing.T) {
	for _, tc := range []struct {
		name string
		fake *buildxFake
		next string
	}{
		{"brew install fails", &buildxFake{brew: true, prefix: "/opt/homebrew", installFails: true}, "brew install docker-buildx"},
		{"brew --prefix fails", &buildxFake{brew: true, prefix: "/opt/homebrew", installed: true, prefixFails: true}, "brew doctor"},
		{"plugin never answers", &buildxFake{brew: true, prefix: "/opt/homebrew", installed: true, neverAnswers: true}, "brew reinstall docker-buildx"},
	} {
		s := newBuildxCase(t, osDarwin, false, tc.fake)
		report := &Report{}
		outcome := s.visitBuildx(report)
		if outcome.State != StatePending {
			t.Errorf("%s: state %q (%s), want %q", tc.name, outcome.State, outcome.Detail, StatePending)
		}
		text := report.Text()
		for _, want := range []string{whyBuildx, tc.next + "\n", BuildKitSetupCommand} {
			if !strings.Contains(text, want) {
				t.Errorf("%s: the report does not name %q:\n%s", tc.name, want, text)
			}
		}
	}
}

// TestBuildxRefusesADanglingConfigLink proves a broken link is not replaced.
// VALIDATES: a config path that is a symbolic link to nothing is refused, the
// link stays a link, and the report names the path.
// PREVENTS: a regular file silently taking the place of a link the owner keeps
// to a dotfiles checkout.
func TestBuildxRefusesADanglingConfigLink(t *testing.T) {
	fake := &buildxFake{brew: true, prefix: "/opt/homebrew", installed: true}
	s := newBuildxCase(t, osDarwin, false, fake)
	if err := os.MkdirAll(filepath.Dir(fake.config), 0o700); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(t.TempDir(), "absent.json")
	if err := os.Symlink(target, fake.config); err != nil {
		t.Fatal(err)
	}

	report := &Report{}
	if outcome := s.visitBuildx(report); outcome.State != StatePending {
		t.Fatalf("state %q (%s), want %q", outcome.State, outcome.Detail, StatePending)
	}
	info, err := os.Lstat(fake.config)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Error("the dangling link was replaced by a regular file")
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Errorf("the link's target was created: %v", err)
	}
	if !strings.Contains(report.Text(), target) {
		t.Errorf("the report does not name the missing target %q:\n%s", target, report.Text())
	}
}

// TestBuildxRefusesWithoutAHome proves no relative config is written.
// VALIDATES: with no home directory and no DOCKER_CONFIG the step refuses,
// and writes nothing under the working directory.
// PREVENTS: a .docker/config.json created in the checkout.
func TestBuildxRefusesWithoutAHome(t *testing.T) {
	if _, err := DockerConfigPath(func(string) string { return "" }, ""); err == nil {
		t.Error("an empty home answered a config path")
	}
	got, err := DockerConfigPath(func(key string) string {
		if key == "DOCKER_CONFIG" {
			return "/cfg"
		}
		return ""
	}, "")
	if err != nil || got != "/cfg/config.json" {
		t.Errorf("DOCKER_CONFIG with no home = %q, %v; want /cfg/config.json", got, err)
	}

	fake := &buildxFake{brew: true, prefix: "/opt/homebrew", installed: true}
	s := newBuildxCase(t, osDarwin, false, fake)
	s.Home = ""
	s.Env = func(key string) string {
		if key == "HOME" {
			return ""
		}
		return ""
	}
	t.Setenv("HOME", "")
	t.Chdir(t.TempDir())

	report := &Report{}
	if outcome := s.visitBuildx(report); outcome.State != StatePending {
		t.Fatalf("state %q (%s), want %q", outcome.State, outcome.Detail, StatePending)
	}
	if _, err := os.Stat(".docker"); !os.IsNotExist(err) {
		t.Errorf("a relative .docker was written under the working directory: %v", err)
	}
}
