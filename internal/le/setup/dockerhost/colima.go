// Design: docs/architecture/appliance/docker-host.md -- the ze Colima profile
// Related: build.go -- the image start boots
// Related: lock.go -- the hold start and stop run under

package dockerhost

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	labhost "github.com/ze-software/ze/internal/le/dockerhost"
)

// archARM64 is the Go name of the arch Rosetta and aarch64 belong to.
const archARM64 = "arm64"

// colimaRecordFileName holds the image key and path the ze profile was last
// started with, under the docker-host cache root.
const colimaRecordFileName = "colima-ze.started"

// colimaArgv is the only builder of a colima command line. Every argv it
// answers names the ze profile right after the verb, so no path in setup can
// reach Colima's default profile by omitting the flag (spec R-6).
func colimaArgv(verb string, options ...string) []string {
	return append([]string{"colima", verb, "--profile", labhost.ZeProfile}, options...)
}

// colimaStartArgv is the argv that boots the image as the ze profile.
// `--activate=false` keeps Colima from switching the Docker context itself:
// start switches it with useContext on every path, the no-op one included,
// where no colima command runs, so one place owns the switch and its error.
// Colima still creates the `colima-ze` context while it provisions the
// runtime, activate or not. `--force-disk-image` is needed because Colima
// accepts only its own Ubuntu disk images without it.
func colimaStartArgv(image, arch string) []string {
	options := []string{
		"--vm-type", "vz",
		"--arch", colimaArch(arch),
		"--runtime", dockerCLI,
		"--disk-image", image,
		"--force-disk-image",
		"--mount-type", "virtiofs",
		"--cpus", "4",
		"--memory", "8",
		"--activate=false",
	}
	if arch == archARM64 {
		options = append(options, "--vz-rosetta")
	}
	return colimaArgv("start", options...)
}

// zeDockerContext is the Docker context Colima creates for the ze profile:
// Colima names a profile's context `colima-<profile>`.
func zeDockerContext() string {
	return "colima-" + labhost.ZeProfile
}

// colimaArch is Colima's name for a Go architecture.
func colimaArch(arch string) string {
	if arch == archARM64 {
		return "aarch64"
	}
	return "x86_64"
}

// profileState is what setup knows of the ze profile before it acts.
type profileState struct {
	// exists is true when Lima holds an instance for the profile.
	exists bool
	// running is true when `colima status --profile ze` succeeds.
	running bool
}

// colimaHost drives the ze profile. Each external effect is a field so a
// test records the argv sequence instead of running Colima.
type colimaHost struct {
	// run runs one colima argv, streaming its output; it fails on a non-zero exit.
	run func(ctx context.Context, argv []string) error
	// state answers whether the profile exists and runs.
	state func(ctx context.Context) (profileState, error)
	// useContext makes the named Docker context the operator's current one.
	useContext func(ctx context.Context, name string) error
	// recordPath is the file holding the started image key and path.
	recordPath string
	// progress receives one line per colima argv before it runs.
	progress io.Writer
}

// startOutcome says what start did.
type startOutcome struct {
	Profile  string `json:"profile"`
	Image    string `json:"image"`
	Key      string `json:"key"`
	Action   string `json:"action"`
	Recorded string `json:"recorded-key"`
	// Context is the Docker context start left current.
	Context string `json:"context"`
}

// Start actions, as start reports them.
const (
	startActionNone     = "none: the profile already runs this image"
	startActionStart    = "started"
	startActionRecreate = "recreated: the image changed"
)

// start makes the ze profile run image. With the recorded key equal to the
// image's and the profile running it starts nothing. When the profile exists
// with another image it stops it, deletes it and starts it afresh, because
// Colima keeps an existing disk and would ignore the new image (spec AC-7,
// R-5). The key is recorded only after the start succeeded. Every path that
// succeeds then makes `colima-ze` the current Docker context, so a plain
// `docker` in the operator's shell reaches the ze VM.
func (h colimaHost) start(ctx context.Context, image builtImage, arch string) (startOutcome, error) {
	recorded, _, err := readColimaRecord(h.recordPath)
	if err != nil {
		return startOutcome{}, err
	}
	state, err := h.state(ctx)
	if err != nil {
		return startOutcome{}, err
	}
	outcome := startOutcome{Profile: labhost.ZeProfile, Image: image.Path, Key: image.Key, Recorded: recorded}
	if recorded == image.Key && state.running {
		outcome.Action = startActionNone
		return h.activate(ctx, outcome)
	}
	outcome.Action = startActionStart
	if state.exists && recorded != image.Key {
		outcome.Action = startActionRecreate
		if state.running {
			if err := h.runShown(ctx, colimaArgv("stop")); err != nil {
				return startOutcome{}, err
			}
		}
		if err := h.runShown(ctx, colimaArgv("delete", "--force")); err != nil {
			return startOutcome{}, err
		}
	}
	if err := h.runShown(ctx, colimaStartArgv(image.Path, arch)); err != nil {
		return startOutcome{}, err
	}
	if err := writeColimaRecord(h.recordPath, image); err != nil {
		return startOutcome{}, err
	}
	return h.activate(ctx, outcome)
}

// activate makes the ze profile's Docker context current and records it in
// the outcome.
func (h colimaHost) activate(ctx context.Context, outcome startOutcome) (startOutcome, error) {
	name := zeDockerContext()
	if _, err := io.WriteString(h.progress, "runs: docker context use "+name+"\n"); err != nil {
		return startOutcome{}, err
	}
	if err := h.useContext(ctx, name); err != nil {
		return startOutcome{}, err
	}
	outcome.Context = name
	return outcome, nil
}

// stop stops the ze profile when it runs, and does nothing otherwise.
func (h colimaHost) stop(ctx context.Context) (map[string]string, error) {
	state, err := h.state(ctx)
	if err != nil {
		return nil, err
	}
	if !state.running {
		return map[string]string{"profile": labhost.ZeProfile, "action": "none: the profile is not running"}, nil
	}
	if err := h.runShown(ctx, colimaArgv("stop")); err != nil {
		return nil, err
	}
	return map[string]string{"profile": labhost.ZeProfile, "action": "stopped"}, nil
}

// runShown prints the argv, then runs it.
func (h colimaHost) runShown(ctx context.Context, argv []string) error {
	if _, err := io.WriteString(h.progress, "runs: "+strings.Join(argv, " ")+"\n"); err != nil {
		return err
	}
	return h.run(ctx, argv)
}

// readColimaRecord answers the key and image path the profile was last
// started with, both empty when it never was.
func readColimaRecord(path string) (key, image string, err error) {
	data, err := os.ReadFile(path) //nolint:gosec // the record in the docker-host cache
	if errors.Is(err, fs.ErrNotExist) {
		return "", "", nil
	}
	if err != nil {
		return "", "", fmt.Errorf("read %s: %w", path, err)
	}
	key, image, found := strings.Cut(strings.TrimSpace(string(data)), " ")
	if !found {
		return "", "", fmt.Errorf("%s holds %q, not `<key> <image path>`. Fix: delete it, then run `./le setup docker-host start`", path, string(data))
	}
	return key, image, nil
}

// writeColimaRecord records the image the profile now runs.
func writeColimaRecord(path string, image builtImage) error {
	if err := os.WriteFile(path, []byte(image.Key+" "+image.Path+"\n"), 0o644); err != nil { //nolint:gosec // the record in the docker-host cache
		return fmt.Errorf("record the started image in %s: %w", path, err)
	}
	return nil
}

// zeSocketPath is the Docker socket Colima forwards for the ze profile.
func zeSocketPath() (string, error) {
	root, err := labhost.SystemColimaHome()
	if err != nil {
		return "", err
	}
	return labhost.ZeSocket(root), nil
}

// systemColimaHost is the colimaHost that runs the real colima binary.
func systemColimaHost(cacheDir string) colimaHost {
	return colimaHost{
		run:        runColima,
		state:      colimaProfileState,
		useContext: useDockerContext,
		recordPath: filepath.Join(cacheDir, colimaRecordFileName),
		progress:   os.Stderr,
	}
}

// runColima runs argv with its output on stderr and no stdin, so a prompt
// fails instead of waiting.
func runColima(ctx context.Context, argv []string) error {
	command := exec.CommandContext(ctx, argv[0], argv[1:]...) //nolint:gosec // argv from colimaArgv
	command.Stdout = os.Stderr
	command.Stderr = os.Stderr
	if err := command.Run(); err != nil {
		return fmt.Errorf("`%s` failed: %w (its output is above). Fix: read Colima's message, then run "+
			"`colima delete --profile ze --force` and `./le setup docker-host start` if the profile is broken", strings.Join(argv, " "), err)
	}
	return nil
}

// useDockerContext runs `docker context use name` with its output on stderr
// and no stdin.
func useDockerContext(ctx context.Context, name string) error {
	command := exec.CommandContext(ctx, dockerCLI, "context", "use", name) //nolint:gosec // name from zeDockerContext
	command.Stdout = os.Stderr
	command.Stderr = os.Stderr
	if err := command.Run(); err != nil {
		return fmt.Errorf("the ze profile runs, but `docker context use %s` failed: %w (its output is above), "+
			"so a plain `docker` still talks to another daemon. Colima creates that context when it starts the profile. "+
			"Fix: run `docker context use %s`; if docker says the context does not exist, run "+
			"`colima stop --profile %s` and `./le setup docker-host start`", name, err, name, labhost.ZeProfile)
	}
	return nil
}

// colimaProfileState asks Lima's instance directory whether the profile
// exists and `colima status` whether it runs.
func colimaProfileState(ctx context.Context) (profileState, error) {
	root, err := labhost.SystemColimaHome()
	if err != nil {
		return profileState{}, err
	}
	instance := filepath.Join(root, "_lima", "colima-"+labhost.ZeProfile)
	if _, err := os.Stat(instance); errors.Is(err, fs.ErrNotExist) {
		return profileState{}, nil
	} else if err != nil {
		return profileState{}, fmt.Errorf("inspect %s: %w", instance, err)
	}
	argv := colimaArgv("status")
	statusErr := exec.CommandContext(ctx, argv[0], argv[1:]...).Run() //nolint:gosec // argv from colimaArgv
	if _, exited := errors.AsType[*exec.ExitError](statusErr); exited {
		return profileState{exists: true}, nil
	}
	if statusErr != nil {
		return profileState{}, fmt.Errorf("run `%s`: %w. Fix: install Colima (brew install colima)", strings.Join(argv, " "), statusErr)
	}
	return profileState{exists: true, running: true}, nil
}
