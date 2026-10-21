// Design: docs/guide/developer-setup.md -- Docker labs: BuildKit for the docker CLI
// Related: setup.go -- Run visits this step after the tool table installs docker.
// Related: ../interoplab/buildkit.go -- the lab refusal that names BuildKitSetupCommand.
//
// The interop labs build images with a named build context, which only BuildKit
// accepts, and the docker CLI reaches BuildKit through its buildx plugin. A
// Homebrew docker-buildx is installed outside every directory the docker CLI
// searches, so the formula alone leaves `docker buildx` an unknown command:
// the CLI finds it only once ~/.docker/config.json names the directory in
// cliPluginsExtraDirs. This step owns both halves, and the verdict is the
// plugin answering, never the package being present.

package setup

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// BuildKitSetupCommand is the command that makes BuildKit available to the
// docker CLI. The interop lab's BuildKit refusal prints it as the fix, so the
// refusal and this step name one answer.
const BuildKitSetupCommand = "./le setup install"

const (
	// buildxName is what the report calls the step, and the Homebrew formula.
	buildxName = "docker-buildx"
	// buildxPluginSubdir is where the formula puts the plugin, under the
	// Homebrew prefix. Homebrew's own caveat names the same path.
	buildxPluginSubdir = "lib/docker/cli-plugins"
	// dockerConfigEnv moves the docker CLI's config directory away from
	// ~/.docker, so the edit follows it.
	dockerConfigEnv = "DOCKER_CONFIG"
	// pluginDirsKey is the config key the docker CLI reads extra plugin
	// directories from.
	pluginDirsKey = "cliPluginsExtraDirs"
	// buildxProbeTimeout bounds one `docker buildx version`. The probe reads
	// no daemon, so a reply past this is a hung CLI.
	buildxProbeTimeout = 30 * time.Second
	// buildxCheckCommand is the probe, printed for the human who finishes a
	// step by hand.
	buildxCheckCommand = "docker buildx version"
	// whyBuildx is the consequence every failing row states.
	whyBuildx = "the interop labs cannot build their images (named build contexts need BuildKit)"
)

// visitBuildx reports whether the docker CLI reaches BuildKit, and on an
// install run makes it so where setup can.
//
// SKIPPED with no docker CLI: docker is an optional row, and a host with no
// docker runs no lab. MISSING on a check run, so the verdict points at the
// install action. PENDING when only a human can finish it: Linux, where the
// plugin is a root package and setup runs no sudo for it, or a Mac with no
// Homebrew.
func (s *Setup) visitBuildx(report *Report) Outcome {
	if !s.Shell.Present(toolDocker) {
		return Outcome{Name: buildxName, State: StateSkipped, Detail: "no docker CLI, so no lab to build for"}
	}
	probe := s.probeBuildx()
	if probe.OK() {
		return Outcome{Name: buildxName, State: StatePresent, Detail: firstLine(probe.Out)}
	}
	if s.Check {
		return Outcome{Name: buildxName, State: StateMissing, Detail: whyBuildx}
	}
	switch s.goos() {
	case osDarwin:
		return s.installBuildxDarwin(report)
	case osLinux:
		return s.noteBuildxLinux(report)
	}
	return Outcome{Name: buildxName, State: StateSkipped, Detail: "no buildx route for this platform"}
}

// probeBuildx asks the docker CLI itself whether it finds the plugin.
func (s *Setup) probeBuildx() Result {
	return s.Shell.Run(Cmd{Argv: []string{toolDocker, "buildx", "version"}, Timeout: buildxProbeTimeout})
}

// installBuildxDarwin installs the formula when brew lacks it, names its
// plugin directory in the docker config, and judges the run by the probe.
func (s *Setup) installBuildxDarwin(report *Report) Outcome {
	if !s.Shell.Present(brewBin) {
		report.Note("  docker-buildx: Homebrew is not installed, so setup cannot add the BuildKit plugin")
		report.Note("    and " + whyBuildx + ".")
		report.Note("    Install Homebrew from https://brew.sh, then run:")
		report.Note("    " + BuildKitSetupCommand)
		report.Note("    or install the plugin by hand with: brew install " + buildxName)
		return Outcome{Name: buildxName, State: StatePending, Detail: "no Homebrew to install the plugin"}
	}

	if !s.Shell.Run(Cmd{Argv: []string{brewBin, "list", "--versions", buildxName}}).OK() {
		report.Note("  brew install " + buildxName)
		if result := s.Shell.Run(Cmd{Argv: []string{brewBin, installSubcommand, buildxName}}); !result.OK() {
			report.Note("  docker-buildx: brew could not install the plugin, so " + whyBuildx + ".")
			report.Note("    brew answered: " + result.complaint())
			report.Note("    Fix what brew reports, then install the plugin and run setup again:")
			report.Note("    brew install " + buildxName)
			report.Note("    " + BuildKitSetupCommand)
			return Outcome{Name: buildxName, State: StatePending, Detail: "brew install " + buildxName + " failed"}
		}
	}

	// The prefix is asked of brew itself: this step needs brew to install, and
	// Homebrew's caveat for the formula names $(brew --prefix).
	answer := s.Shell.Run(Cmd{Argv: []string{brewBin, "--prefix"}})
	prefix := strings.TrimSpace(answer.Out)
	if !answer.OK() || prefix == "" {
		report.Note("  docker-buildx: `brew --prefix` did not name the Homebrew prefix, so setup cannot tell")
		report.Note("    which plugin directory to add to the docker config, and " + whyBuildx + ".")
		report.Note("    brew answered: " + answer.complaint())
		report.Note("    Ask Homebrew what is wrong, fix it, then run setup again:")
		report.Note("    brew doctor")
		report.Note("    " + BuildKitSetupCommand)
		return Outcome{Name: buildxName, State: StatePending, Detail: "Homebrew prefix unknown"}
	}
	pluginDir := filepath.Join(prefix, buildxPluginSubdir)
	entry := `"` + pluginDirsKey + `": ["` + pluginDir + `"]`

	config, err := s.dockerConfigPath()
	if err != nil {
		var tb textbuf.Buffer
		report.Note(tb.Str("  docker-buildx: ").Err(err).Str(", so ").Str(whyBuildx).Str(".").String())
		report.Note("    Set HOME, or DOCKER_CONFIG to the docker config directory, then run:")
		report.Note("    " + BuildKitSetupCommand)
		return Outcome{Name: buildxName, State: StatePending, Detail: "docker config location unknown"}
	}
	added, err := addPluginDir(config, pluginDir)
	if err != nil {
		var tb textbuf.Buffer
		report.Note(tb.Str("  docker-buildx: cannot add the plugin directory to ").Str(config).Str(": ").Err(err).String())
		report.Note("    The docker CLI does not search " + pluginDir + " until that file names it,")
		report.Note("    so " + whyBuildx + ". Repair the file or add this entry by hand, then run: " + buildxCheckCommand)
		report.Note("    " + entry)
		return Outcome{Name: buildxName, State: StatePending, Detail: "docker config not updated"}
	}
	if added {
		report.Note("  added " + pluginDir + " to " + pluginDirsKey + " in " + config)
	}

	probe := s.probeBuildx()
	if !probe.OK() {
		report.Note("  docker-buildx: brew holds the plugin and " + config + " names " + pluginDir + ",")
		report.Note("    and `" + buildxCheckCommand + "` still fails, so " + whyBuildx + ".")
		report.Note("    docker answered: " + probe.complaint())
		report.Note("    The plugin file " + filepath.Join(pluginDir, buildxName) + " may be missing or broken.")
		report.Note("    Reinstall it, then run setup again:")
		report.Note("    brew reinstall " + buildxName)
		report.Note("    " + BuildKitSetupCommand)
		return Outcome{Name: buildxName, State: StatePending, Detail: "the plugin does not answer"}
	}
	return Outcome{Name: buildxName, State: StateInstalled, Detail: firstLine(probe.Out)}
}

// dpkgInstalled is the status dpkg-query prints for an installed package. A
// removed package whose config files remain answers "deinstall ok
// config-files", and dpkg-query still exits 0 for it.
const dpkgInstalled = "install ok installed"

// noteBuildxLinux names the package that matches the installed docker, and
// the root command that installs it. Setup runs no sudo for it: the owner
// asked for the step to name the package, not to add one.
//
// Docker's own repository ships docker-ce beside docker-buildx-plugin; Debian
// and Ubuntu ship docker.io beside docker-buildx. Mixing the two families
// conflicts, so the package follows what dpkg holds installed.
func (s *Setup) noteBuildxLinux(report *Report) Outcome {
	pkg := "docker-buildx"
	status := s.Shell.Run(Cmd{Argv: []string{"dpkg-query", "-W", "-f=${Status}", "docker-ce"}})
	if status.OK() && strings.TrimSpace(status.Out) == dpkgInstalled {
		pkg = "docker-buildx-plugin"
	}
	report.Note("  docker-buildx: the docker CLI has no buildx plugin, so " + whyBuildx + ".")
	report.Note("    Install the package as root, then run: ./le setup check")
	report.Note("    sudo apt-get install " + pkg)
	return Outcome{Name: buildxName, State: StatePending, Detail: "install " + pkg + " as root"}
}

// dockerConfigPath answers the config file the docker CLI reads, through
// DockerConfigPath.
func (s *Setup) dockerConfigPath() (string, error) {
	return DockerConfigPath(s.env, s.home())
}

// DockerConfigPath answers the config file the docker CLI reads:
// $DOCKER_CONFIG/config.json when getenv names the variable, else
// <home>/.docker/config.json. An empty home with no DOCKER_CONFIG is an error,
// never a path relative to the working directory. The setup step edits this
// file and the interop lab's BuildKit refusal names it, so both read it here.
func DockerConfigPath(getenv func(string) string, home string) (string, error) {
	if dir := getenv(dockerConfigEnv); dir != "" {
		return filepath.Join(dir, "config.json"), nil
	}
	if home == "" {
		return "", errors.New("cannot find the docker config: neither HOME nor DOCKER_CONFIG is set")
	}
	return filepath.Join(home, ".docker", "config.json"), nil
}

// addPluginDir appends dir to cliPluginsExtraDirs in the docker config at
// path, and answers whether the file changed.
//
// The edit is a read-modify-write over the top-level keys: every other key
// keeps its value, because the file holds registry credentials and contexts.
// Ze writes the keys sorted (encoding/json orders map keys) and tab-indented.
// A file that already names dir is not touched. A file that is not a JSON
// object, or whose entry is not a list of strings, is refused rather than
// replaced. A symbolic link is followed to the file it names, and a link that
// names nothing is refused: replacing it would turn the link into a file. The
// new content is synced and replaces the old by rename, so a failed write
// leaves the original whole.
func addPluginDir(path, dir string) (bool, error) {
	if link, err := os.Lstat(path); err == nil && link.Mode()&fs.ModeSymlink != 0 {
		resolved, err := filepath.EvalSymlinks(path)
		if err != nil {
			target, _ := os.Readlink(path) //nolint:errcheck // the target only enriches the refusal
			return false, fmt.Errorf("it is a symbolic link to %s, which does not exist; create that file or remove the link: %w", target, err)
		}
		path = resolved
	}

	config := map[string]json.RawMessage{}
	mode := fs.FileMode(0o600)
	data, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return false, err
	}
	if err == nil {
		info, statErr := os.Stat(path)
		if statErr != nil {
			return false, statErr
		}
		mode = info.Mode().Perm()
		if len(bytes.TrimSpace(data)) > 0 {
			if err := json.Unmarshal(data, &config); err != nil {
				return false, fmt.Errorf("it is not a JSON object: %w", err)
			}
		}
	}
	// A file holding `null` decodes to a nil map.
	if config == nil {
		config = map[string]json.RawMessage{}
	}

	var dirs []string
	if raw, found := config[pluginDirsKey]; found {
		if err := json.Unmarshal(raw, &dirs); err != nil {
			return false, fmt.Errorf("its %s is not a list of directories: %w", pluginDirsKey, err)
		}
	}
	if slices.Contains(dirs, dir) {
		return false, nil
	}
	encoded, err := json.Marshal(append(dirs, dir))
	if err != nil {
		return false, err
	}
	config[pluginDirsKey] = encoded

	out, err := json.MarshalIndent(config, "", "\t")
	if err != nil {
		return false, err
	}
	return true, replaceFile(path, append(out, '\n'), mode)
}

// replaceFile writes content beside path and renames it over path, so a
// reader sees the old file or the new one and never half of either.
func replaceFile(path string, content []byte, mode fs.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	temp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".*")
	if err != nil {
		return err
	}

	name := temp.Name()
	defer os.Remove(name) //nolint:errcheck // gone after a successful rename; a leftover only on failure

	if _, err := temp.Write(content); err != nil {
		temp.Close() //nolint:errcheck // the write error is the one reported
		return err
	}
	if err := temp.Chmod(mode); err != nil {
		temp.Close() //nolint:errcheck // the chmod error is the one reported
		return err
	}
	if err := temp.Sync(); err != nil {
		temp.Close() //nolint:errcheck // the sync error is the one reported
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	return os.Rename(name, path)
}
