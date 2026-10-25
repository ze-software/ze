// Design: docs/architecture/appliance/docker-host.md -- `./le setup docker-host`
// Related: register.go -- the area registration
// Related: build.go -- the image build
// Related: colima.go -- the ze Colima profile

package dockerhost

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"syscall"

	leaction "github.com/ze-software/ze/internal/le/le/action"
	lepath "github.com/ze-software/ze/internal/le/le/path"
)

// dockerHostArea is the name this area is typed as.
const dockerHostArea = "setup docker-host"

const (
	archKeyword  = "arch"
	pinsKeyword  = "pins"
	sizeKeyword  = "size"
	darwinOS     = "darwin"
	verdictPass  = "pass"
	verdictFail  = "fail"
	checkImage   = "image"
	checkProfile = "profile ze"
	checkSocket  = "socket"
)

var dockerHostActions = leaction.New(dockerHostArea,
	leaction.Action{
		Verb:   "build",
		Writes: true,
		Why: "build Ze's docker-host image (Ze's runtime kernel, dockerd, containerd, sshd, sudo, iptables) from the" +
			" pins in " + PinFile + " into ~/.cache/ze/docker-host/<arch>/<key>/ and print its path; the same inputs" +
			" reuse the cached image. `arch` defaults to this host's, `size` (GiB, 2-1024) to 16, `pins` names another pin file",
		Parameters: []leaction.Parameter{
			{Keyword: archKeyword, Kind: leaction.KindEnum, Help: "the architecture to build for; this host's when absent", Value: "arch", Values: []string{archAMD64, archARM64},
				ValueHelp: map[string]string{
					archAMD64: "the x86-64 image, for Intel and AMD hosts",
					archARM64: "the 64-bit Arm image, for Apple silicon and Arm hosts",
				},
				Requirement: leaction.Optional},
			{Keyword: sizeKeyword, Kind: leaction.KindUint, Help: "the image disk size in GiB", Value: "GiB", Requirement: leaction.Optional},
			{Keyword: pinsKeyword, Kind: leaction.KindPath, Help: "the pin file the image is built from", Value: "pin-file", Requirement: leaction.Optional},
		},
		AnswerArgs: runBuild,
	},
	leaction.Action{
		Verb:   "start",
		Writes: true,
		Why: "macOS: build the image if needed, then run it as the `ze` Colima profile, whose Docker socket" +
			" (ze/docker.sock under Colima's home: COLIMA_HOME, else ~/.colima) the labs use;" +
			" a changed image stops, deletes and recreates the profile." +
			" Colima's `default` profile is never named",
		Answer: runStart,
	},
	leaction.Action{
		Verb:   "stop",
		Writes: true,
		Why:    "macOS: stop the `ze` Colima profile when it runs",
		Answer: runStop,
	},
	leaction.Action{
		Verb: "check",
		Why: "report the tools the docker-host image needs (colima, limactl, docker, mkfs.ext4), the image the `ze`" +
			" profile was started with, whether the profile runs and whether its Docker socket exists; exits 1 naming each fix",
		Answer: runCheck,
	},
)

// DockerHostActions answers the area's command surface as data.
func DockerHostActions() leaction.List { return dockerHostActions.Actions() }

// DockerHostSubs is the one-line hint help renders under the command.
func DockerHostSubs() string { return dockerHostActions.Subs() }

// DockerHostAnswer is the `le setup docker-host` command.
func DockerHostAnswer(args []string) (any, int) { return dockerHostActions.Answer(args) }

// runBuild builds the image for the named arch under the setup lock.
func runBuild(args leaction.Arguments) (any, int) {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	req, err := parseBuildRequest(args.One(archKeyword), args.One(sizeKeyword), args.One(pinsKeyword))
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	image, err := lockedBuild(ctx, req)
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	return image, 0
}

// lockedBuild builds under the setup lock.
func lockedBuild(ctx context.Context, req buildRequest) (builtImage, error) {
	cacheDir, err := dockerHostCacheDir()
	if err != nil {
		return builtImage{}, err
	}
	lock, err := takeLock(cacheDir)
	if err != nil {
		return builtImage{}, err
	}
	defer lock.release() //nolint:errcheck // process exit drops the hold as well
	return systemBuilder(cacheDir, Fetcher{CacheDir: cacheDir, Client: http.DefaultClient}).build(ctx, req)
}

// EnsureImage answers the raw docker-host image for arch built from the
// checkout at repo, with the default size and the repository's pin file, from
// the cache when the inputs' key already holds it, under the setup lock.
// `./le test qemu docker-lab` boots it (spec D-6), so the QEMU guest and the
// ze Colima profile run one image.
func EnsureImage(ctx context.Context, repo, arch string) (string, error) {
	if !slices.Contains(pinArches, arch) {
		return "", fmt.Errorf("arch %q is not one the docker-host image is built for. Fix: run on a host whose arch is one of %s",
			arch, strings.Join(pinArches, ", "))
	}
	pins, err := LoadPins(filepath.Join(repo, PinFile))
	if err != nil {
		return "", err
	}
	image, err := lockedBuild(ctx, buildRequest{repo: repo, arch: arch, pins: pins, sizeGiB: imageSizeGiBDefault})
	if err != nil {
		return "", err
	}
	return image.Path, nil
}

// runStart builds the host arch's image when needed and runs it as the ze
// profile, all under one hold of the setup lock.
func runStart() (any, int) {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	outcome, err := start(ctx)
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	return outcome, 0
}

// start is runStart's work, returning its error.
func start(ctx context.Context) (startOutcome, error) {
	if err := refuseOffDarwin("start"); err != nil {
		return startOutcome{}, err
	}
	req, err := parseBuildRequest("", "", "")
	if err != nil {
		return startOutcome{}, err
	}
	cacheDir, err := dockerHostCacheDir()
	if err != nil {
		return startOutcome{}, err
	}
	lock, err := takeLock(cacheDir)
	if err != nil {
		return startOutcome{}, err
	}
	defer lock.release() //nolint:errcheck // process exit drops the hold as well

	image, err := systemBuilder(cacheDir, Fetcher{CacheDir: cacheDir, Client: http.DefaultClient}).build(ctx, req)
	if err != nil {
		return startOutcome{}, err
	}
	return systemColimaHost(cacheDir).start(ctx, image, req.arch)
}

// runStop stops the ze profile under the setup lock.
func runStop() (any, int) {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	answer, err := stopProfile(ctx)
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	return answer, 0
}

// stopProfile is runStop's work, returning its error.
func stopProfile(ctx context.Context) (map[string]string, error) {
	if err := refuseOffDarwin("stop"); err != nil {
		return nil, err
	}
	cacheDir, err := dockerHostCacheDir()
	if err != nil {
		return nil, err
	}
	lock, err := takeLock(cacheDir)
	if err != nil {
		return nil, err
	}
	defer lock.release() //nolint:errcheck // process exit drops the hold as well
	return systemColimaHost(cacheDir).stop(ctx)
}

// checkRow is one line of `check`.
type checkRow struct {
	Check   string `json:"check"`
	Verdict string `json:"verdict"`
	Detail  string `json:"detail"`
}

// runCheck reports each dependency and the profile's state.
func runCheck() (any, int) {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	rows := checkRows(ctx)
	code := 0
	for _, row := range rows {
		if row.Verdict != verdictPass {
			code = 1
		}
	}
	return rows, code
}

// checkRows answers every check, each failing row naming its fix.
func checkRows(ctx context.Context) []checkRow {
	var rows []checkRow
	for _, tool := range []struct{ name, fix string }{
		{"colima", "brew install colima"},
		{"limactl", "brew install lima"},
		{dockerCLI, "brew install docker"},
	} {
		if path, err := exec.LookPath(tool.name); err == nil {
			rows = append(rows, checkRow{tool.name, verdictPass, path})
		} else {
			rows = append(rows, checkRow{tool.name, verdictFail, "not on PATH. Fix: " + tool.fix})
		}
	}
	if path, err := findMkfs(); err == nil {
		rows = append(rows, checkRow{"mkfs.ext4", verdictPass, path})
	} else {
		rows = append(rows, checkRow{"mkfs.ext4", verdictFail, err.Error()})
	}
	rows = append(rows, imageRow(), profileRow(ctx), socketRow())
	return rows
}

// imageRow checks the image the profile was last started with.
func imageRow() checkRow {
	cacheDir, err := dockerHostCacheDir()
	if err != nil {
		return checkRow{checkImage, verdictFail, err.Error()}
	}
	key, image, err := readColimaRecord(filepath.Join(cacheDir, colimaRecordFileName))
	if err != nil {
		return checkRow{checkImage, verdictFail, err.Error()}
	}
	if key == "" {
		return checkRow{checkImage, verdictFail, "the ze profile was never started by setup. Fix: ./le setup docker-host start"}
	}
	if _, err := os.Stat(image); err != nil {
		return checkRow{checkImage, verdictFail, "the started image " + image + " is gone (" + err.Error() + "). Fix: ./le setup docker-host start"}
	}
	return checkRow{checkImage, verdictPass, key + " " + image}
}

// profileRow checks that the ze profile runs.
func profileRow(ctx context.Context) checkRow {
	state, err := colimaProfileState(ctx)
	if err != nil {
		return checkRow{checkProfile, verdictFail, err.Error()}
	}
	if !state.running {
		return checkRow{checkProfile, verdictFail, "not running. Fix: ./le setup docker-host start"}
	}
	return checkRow{checkProfile, verdictPass, "running"}
}

// socketRow checks the ze profile's Docker socket.
func socketRow() checkRow {
	socket, err := zeSocketPath()
	if err != nil {
		return checkRow{checkSocket, verdictFail, err.Error()}
	}
	info, err := os.Stat(socket)
	if errors.Is(err, fs.ErrNotExist) {
		return checkRow{checkSocket, verdictFail, socket + " does not exist. Fix: ./le setup docker-host start"}
	}
	if err != nil {
		return checkRow{checkSocket, verdictFail, err.Error()}
	}
	if info.Mode()&fs.ModeSocket == 0 {
		return checkRow{checkSocket, verdictFail, socket + " is not a socket. Fix: colima delete --profile ze --force, then ./le setup docker-host start"}
	}
	return checkRow{checkSocket, verdictPass, socket}
}

// parseBuildRequest checks the build's keywords and reads the pins. Empty
// values take the host arch, the default size and the repository pin file.
func parseBuildRequest(arch, size, pinPath string) (buildRequest, error) {
	repo, err := lepath.Root()
	if err != nil {
		return buildRequest{}, fmt.Errorf("find the repository root: %w", err)
	}
	if arch == "" {
		arch = runtime.GOARCH
	}
	if !slices.Contains(pinArches, arch) {
		return buildRequest{}, fmt.Errorf("arch %q is not one the image is built for. Fix: name one of %s", arch, strings.Join(pinArches, ", "))
	}
	sizeGiB := imageSizeGiBDefault
	if size != "" {
		sizeGiB, err = strconv.Atoi(size)
		if err != nil {
			return buildRequest{}, fmt.Errorf("size %q is not a whole number of GiB. Fix: name one from %d to %d", size, imageSizeGiBMin, imageSizeGiBMax)
		}
	}
	if sizeGiB < imageSizeGiBMin {
		return buildRequest{}, fmt.Errorf("size %d GiB is below %d, which the boot partitions and /perm need. Fix: name %d to %d", sizeGiB, imageSizeGiBMin, imageSizeGiBMin, imageSizeGiBMax)
	}
	if sizeGiB > imageSizeGiBMax {
		return buildRequest{}, fmt.Errorf("size %d GiB is above %d. Fix: name %d to %d", sizeGiB, imageSizeGiBMax, imageSizeGiBMin, imageSizeGiBMax)
	}
	if pinPath == "" {
		pinPath = filepath.Join(repo, PinFile)
	}
	pins, err := LoadPins(pinPath)
	if err != nil {
		return buildRequest{}, err
	}
	return buildRequest{repo: repo, arch: arch, pins: pins, sizeGiB: sizeGiB}, nil
}

// refuseOffDarwin refuses a Colima action on a host that is not a Mac.
func refuseOffDarwin(verb string) error {
	if runtime.GOOS == darwinOS {
		return nil
	}
	return fmt.Errorf("`./le setup docker-host %s` drives the ze Colima profile, which is the macOS lab host; on %s, labs use "+
		"this host's Docker daemon booted on Ze's kernel. Fix: ./le setup docker-kernel install", verb, runtime.GOOS)
}

// dockerHostCacheDir is the docker-host cache root, ~/.cache/ze/docker-host.
func dockerHostCacheDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("find the home directory: %w", err)
	}
	return filepath.Join(home, ".cache", "ze", "docker-host"), nil
}
