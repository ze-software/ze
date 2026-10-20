package perfrunner

import (
	"context"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/netip"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/interoplab"
	"github.com/ze-software/ze/internal/le/linuxle"
)

// The benchmark network: one fixed /24 with the sender, the receiver address
// the sender adds to its own interface, and one fixed address per DUT.
var (
	perfSubnet = netip.MustParsePrefix("172.31.0.0/24")
	senderIP   = netip.MustParseAddr("172.31.0.10")
	receiverIP = netip.MustParseAddr("172.31.0.11")
)

type DUT struct {
	Name, Image                    string
	IP                             netip.Addr
	Port, SenderPort, ReceiverPort int
	Passive                        bool
}

// The DUT names. Each one is the container name, the Dockerfile suffix and the
// key every per-DUT switch in this file branches on.
const (
	dutZE       = "ze"
	dutFRR      = "frr"
	dutBIRD     = "bird"
	dutGoBGP    = "gobgp"
	dutRustBGPd = "rustbgpd"
	dutRustyBGP = "rustybgp"
	dutFreeRtr  = "freertr"
	dutOpenBGPd = "openbgpd"
)

// containerLe is where the sender container finds le. The file name MUST be
// `le`: cmd/ze selects the personality from the name it was started as
// (defaultDispatch), and only that name reaches the `perf send` command.
const containerLe = "/usr/local/bin/le"

// senderImage is the image the sender container runs: le is mounted into it,
// so it needs nothing but ip, arping and a shell.
const senderImage = "alpine:3.21"

// linuxBuildTimeout bounds the cross-build of le. It links every feature the
// launcher's le links, which is more than the perf personality it replaced.
const linuxBuildTimeout = 10 * time.Minute

// imageBuildTimeout bounds one DUT image build.
const imageBuildTimeout = 10 * time.Minute

// cleanupCommand is what an operator runs to remove a container or network
// this run could not remove: every one carries the owner labels it matches.
const cleanupCommand = "./le test integration cleanup"

// Steps selects what one suite run does. The runner refuses a run that selects
// neither, because it would do nothing and answer success.
type Steps struct {
	// Build makes the Docker image of every selected DUT.
	Build bool
	// Test measures every selected DUT whose image exists.
	Test bool
}

func DUTs() []DUT {
	frr := envOr("FRR_IMAGE", "quay.io/frrouting/frr:10.3.1")
	rust := envOr("RUSTBGPD_IMAGE", "rustbgpd-interop")
	return []DUT{
		{Name: dutZE, Image: "ze-interop", IP: netip.MustParseAddr("172.31.0.2"), Port: 179, SenderPort: 1790, ReceiverPort: 1791},
		{Name: dutFRR, Image: frr, IP: netip.MustParseAddr("172.31.0.3"), Port: 179},
		{Name: dutBIRD, Image: "bird-interop", IP: netip.MustParseAddr("172.31.0.4"), Port: 179},
		{Name: dutGoBGP, Image: "gobgp-interop", IP: netip.MustParseAddr("172.31.0.5"), Port: 179},
		{Name: dutRustBGPd, Image: rust, IP: netip.MustParseAddr("172.31.0.6"), Port: 179, Passive: true},
		{Name: dutRustyBGP, Image: "rustybgp-interop", IP: netip.MustParseAddr("172.31.0.7"), Port: 179},
		{Name: dutFreeRtr, Image: "freertr-interop", IP: netip.MustParseAddr("172.31.0.8"), Port: 179},
		{Name: dutOpenBGPd, Image: "openbgpd-interop", IP: netip.MustParseAddr("172.31.0.9"), Port: 179},
	}
}

func envOr(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

func unmeasuredDUTs(resultFiles []string) []string {
	measured := make(map[string]bool, len(resultFiles))
	for _, path := range resultFiles {
		measured[strings.TrimSuffix(filepath.Base(path), ".json")] = true
	}
	missing := make([]string, 0)
	for _, dut := range DUTs() {
		if !measured[dut.Name] {
			missing = append(missing, dut.Name)
		}
	}
	return missing
}

// say writes one line made of parts to w. A write to the operator's terminal
// that fails has nowhere left to be reported, so its error is dropped.
func say(w io.Writer, parts ...string) {
	var tb textbuf.Buffer
	for _, part := range parts {
		tb.Str(part)
	}
	tb.Byte('\n')
	_, _ = io.WriteString(w, tb.String())
}

// CommandRunner runs one host program: the cross-build of le and the reporter.
// Every Docker command goes through Runner.Docker instead.
type CommandRunner func(context.Context, io.Writer, io.Writer, string, []string, []string) error

func systemCommand(ctx context.Context, stdout, stderr io.Writer, dir string, env, args []string) error {
	command := exec.CommandContext(ctx, args[0], args[1:]...) // #nosec G204 -- the runner passes the Go toolchain and the reporter it was given.
	command.Dir, command.Stdout, command.Stderr = dir, stdout, stderr
	if env != nil {
		command.Env = env
	}
	return command.Run()
}

// generateToFile atomically replaces dest only after generator success.
func generateToFile(ctx context.Context, run CommandRunner, command []string, dest string, stderr io.Writer) bool {
	if err := os.MkdirAll(filepath.Dir(dest), 0o750); err != nil {
		say(stderr, "warning: ", err.Error())
		return false
	}
	temporary := dest + ".new"
	file, err := os.Create(temporary) // #nosec G304 -- destination is runner-owned scratch output.
	if err != nil {
		say(stderr, "warning: ", err.Error())
		return false
	}
	commandCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	err = run(commandCtx, file, stderr, "", nil, command)
	cancel()
	closeErr := file.Close()
	if err != nil || closeErr != nil {
		_ = os.Remove(temporary)
		say(stderr, "  warning: ", filepath.Base(dest), " NOT updated: generator failed; existing file left untouched")
		return false
	}
	if err := os.Rename(temporary, dest); err != nil {
		_ = os.Remove(temporary)
		say(stderr, "warning: publish ", dest, ": ", err.Error())
		return false
	}
	return true
}

// Runner is one multi-DUT benchmark over a checkout. Not safe for concurrent
// use: one Runner runs one suite at a time.
//
// The sender inside the container is a linux le, cross-built at LinuxBinary
// with LinuxTags and mounted at containerLe. Reporter is the argv prefix of
// the host program that renders the results: `le perf run` passes the le that
// is running. Docker runs every container, build and removal with the owner
// labels, so a run this process did not finish is removed by the next lab
// start or by cleanupCommand.
type Runner struct {
	Root, LinuxBinary, LinuxTags, ConfigOverlay                string
	Reporter                                                   []string
	Routes, Seed, Repeat                                       int
	NoBuild, PProf, GCTrace                                    bool
	PProfPort, PProfCPUSeconds                                 int
	PProfDir                                                   string
	Stdout, Stderr                                             io.Writer
	Run                                                        CommandRunner
	Docker                                                     *interoplab.Docker
	suffix, network, resultsDir, runDir, interopDir, configDir string
	// containers holds every container this run started and has not removed
	// yet; cleanup removes what is left.
	containers map[string]struct{}
	// networkCreated says cleanup owes the network's removal.
	networkCreated bool
	// removalFailed records that a container, a network or an image tag this
	// run created could not be removed, which fails the run.
	removalFailed bool
}

// New answers a Runner over the checkout at root. reporter is the argv prefix
// that renders a result set, for example the running le followed by
// `perf report`. It MUST NOT be empty: Execute renders every result through it.
func New(root string, reporter []string, stdout, stderr io.Writer) *Runner {
	if len(reporter) == 0 {
		panic("BUG: perfrunner.New needs the argv that renders a result set")
	}
	routes, _ := strconv.Atoi(envOr("DUT_ROUTES", "100000"))
	seed, _ := strconv.Atoi(envOr("DUT_SEED", "42"))
	repeat, _ := strconv.Atoi(envOr("DUT_REPEAT", "3"))
	pprofPort, _ := strconv.Atoi(envOr("PPROF_PORT", "6060"))
	cpu, _ := strconv.Atoi(envOr("PPROF_CPU_SECONDS", "30"))
	suffix := strconv.Itoa(os.Getpid())
	runDir := filepath.Join(root, "tmp", "perf-run")
	// The linux le lives under tmp/, never under bin/: bin/le-<name> is the
	// launcher's namespace, and a perf artifact there would claim a name.
	linux := filepath.Join(runDir, "linux-"+runtime.GOARCH, linuxle.Name)
	return &Runner{Root: root, Reporter: reporter, LinuxBinary: linux, ConfigOverlay: os.Getenv("PERF_CONFIGS_DIR"), Routes: routes, Seed: seed, Repeat: repeat, NoBuild: os.Getenv("NO_BUILD") != "", PProf: os.Getenv("PPROF") != "", GCTrace: os.Getenv("GCTRACE") != "", PProfPort: pprofPort, PProfCPUSeconds: cpu, PProfDir: envOr("PPROF_DIR", filepath.Join(root, "tmp", "perf-run", "pprof")), Stdout: stdout, Stderr: stderr, Run: systemCommand, Docker: interoplab.NewDocker(), suffix: suffix, network: "le-perf-" + suffix, resultsDir: filepath.Join(root, "test", "perf", "results"), runDir: runDir, interopDir: filepath.Join(root, "test", "interop"), configDir: filepath.Join(root, "test", "perf", "configs"), containers: make(map[string]struct{})}
}

func (runner *Runner) config(name string) string {
	if runner.ConfigOverlay != "" {
		candidate := filepath.Join(runner.ConfigOverlay, name)
		if info, err := os.Stat(candidate); err == nil && info.Mode().IsRegular() {
			return candidate
		}
	}
	return filepath.Join(runner.configDir, name)
}
func (runner *Runner) containerName(name string) string {
	return "le-perf-" + name + "-" + runner.suffix
}
func (runner *Runner) senderName() string {
	return "le-perf-runner-" + runner.suffix
}

// pause waits d, or less when ctx ends first, and then answers ctx's error.
func pause(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// buildLinuxBinary cross-builds le for the container by the linuxle recipe, for
// the host's architecture because Docker runs the image natively. It prints the
// build time, which is the cost this run pays before its first measurement.
func (runner *Runner) buildLinuxBinary(ctx context.Context) error {
	if runner.LinuxTags == "" {
		return errors.New("no build tags for the linux le: the caller MUST set LinuxTags")
	}
	if err := os.MkdirAll(filepath.Dir(runner.LinuxBinary), 0o750); err != nil {
		return err
	}
	env := append(os.Environ(), linuxle.Overrides(runtime.GOARCH)...)
	buildCtx, cancel := context.WithTimeout(ctx, linuxBuildTimeout)
	defer cancel()

	started := time.Now()
	argv := linuxle.Argv(runner.LinuxTags, runner.LinuxBinary)
	if err := runner.Run(buildCtx, runner.Stdout, runner.Stderr, runner.Root, env, argv); err != nil {
		return err
	}
	say(runner.Stdout, "Built the linux le in ", time.Since(started).Round(time.Second).String())
	return nil
}

// buildImages makes every selected DUT's image: FRR is pulled, the others are
// built. A failed build is a warning, because the test step skips a DUT whose
// image is absent; a build tag this run could not release fails the run.
func (runner *Runner) buildImages(ctx context.Context, duts []DUT) {
	if runner.NoBuild {
		say(runner.Stdout, "  skipping image builds (NO_BUILD=1)")
		return
	}
	for _, dut := range duts {
		if ctx.Err() != nil {
			return
		}
		say(runner.Stdout, "Building ", dut.Name, " image...")
		if dut.Name == dutFRR {
			if err := runner.Docker.Pull(ctx, dut.Image, 0); err != nil {
				say(runner.Stderr, "  warning: ", dut.Name, " image pull failed: ", err.Error())
			}
			continue
		}
		build, err := runner.imageBuild(dut)
		if err != nil {
			say(runner.Stderr, "  warning: ", err.Error())
			continue
		}
		image, err := runner.Docker.Build(ctx, build)
		if err != nil {
			say(runner.Stderr, "  warning: ", dut.Name, " image build failed: ", err.Error())
			continue
		}
		if err := runner.Docker.ReleaseImage(context.WithoutCancel(ctx), image); err != nil {
			runner.removalFailed = true
			say(runner.Stderr, "  error: release the build tag of ", dut.Name, ": ", err.Error(), "; remove it with ", cleanupCommand)
		}
	}
}

// imageBuild answers the build of one DUT image other than FRR, which is pulled.
func (runner *Runner) imageBuild(dut DUT) (interoplab.ImageBuild, error) {
	build := interoplab.ImageBuild{Name: dut.Name, Tag: dut.Image, Dockerfile: filepath.Join(runner.interopDir, "Dockerfile."+dut.Name), Timeout: imageBuildTimeout}
	switch dut.Name {
	case dutZE:
		build.Context = runner.Root
	case dutBIRD, dutGoBGP, dutRustBGPd, dutRustyBGP:
		build.Context = runner.interopDir
	case dutFreeRtr:
		build.Context = runner.Root
		source := envOr("FREERTR_DIR", filepath.Join(filepath.Dir(runner.Root), "rtr"))
		build.NamedContexts = []interoplab.NamedBuildContext{{Name: "freertr", Path: source}}
	case dutOpenBGPd:
		build.Context = envOr("OPENBGPD_DIR", filepath.Join(filepath.Dir(runner.Root), "openbgpd-portable"))
	default:
		return build, errors.New("no image build is declared for DUT " + dut.Name)
	}
	return build, nil
}

// dutMounts answers the read-only configuration files one DUT reads.
func (runner *Runner) dutMounts(name string) []interoplab.Mount {
	mount := func(source, target string) interoplab.Mount {
		return interoplab.Mount{Source: source, Target: target, ReadOnly: true}
	}
	switch name {
	case dutZE:
		return []interoplab.Mount{mount(runner.config("ze.conf"), "/etc/ze/bgp.conf")}
	case dutFRR:
		return []interoplab.Mount{mount(runner.config("frr.conf"), "/etc/frr/frr.conf"), mount(filepath.Join(runner.interopDir, "daemons"), "/etc/frr/daemons"), mount(filepath.Join(runner.interopDir, "vtysh.conf"), "/etc/frr/vtysh.conf")}
	case dutBIRD:
		return []interoplab.Mount{mount(runner.config("bird.conf"), "/etc/bird/bird.conf")}
	case dutGoBGP:
		return []interoplab.Mount{mount(runner.config("gobgp.toml"), "/etc/gobgp/gobgp.toml")}
	case dutRustBGPd:
		return []interoplab.Mount{mount(runner.config("rustbgpd.toml"), "/etc/rustbgpd/config.toml")}
	case dutRustyBGP:
		return []interoplab.Mount{mount(runner.config("rustybgp.toml"), "/etc/rustybgp/config.toml")}
	case dutFreeRtr:
		return []interoplab.Mount{mount(runner.config("freertr-hw.txt"), "/etc/freertr/freertr-hw.txt"), mount(runner.config("freertr-sw.txt"), "/etc/freertr/freertr-sw.txt")}
	case dutOpenBGPd:
		return []interoplab.Mount{mount(runner.config("openbgpd.conf"), "/etc/bgpd.conf")}
	}
	return nil
}

// dutContainer answers the detached container one DUT runs in.
func (runner *Runner) dutContainer(dut DUT) interoplab.DetachedContainer {
	spec := interoplab.DetachedContainer{Name: runner.containerName(dut.Name), Image: dut.Image, Network: runner.network, Address: dut.IP, Capabilities: []string{"NET_ADMIN"}, Mounts: runner.dutMounts(dut.Name)}
	switch dut.Name {
	case dutFRR:
		spec.Capabilities = append(spec.Capabilities, "SYS_ADMIN")
	case dutFreeRtr:
		spec.Capabilities = append(spec.Capabilities, "NET_RAW")
	case dutOpenBGPd:
		spec.Capabilities = append(spec.Capabilities, "SYS_CHROOT")
	case dutZE:
		if runner.GCTrace {
			spec.Environment = []interoplab.EnvironmentVariable{{Name: "GODEBUG", Value: "gctrace=1"}}
		}
		if runner.PProf {
			spec.Command = append(spec.Command, "--pprof", "127.0.0.1:"+strconv.Itoa(runner.PProfPort))
		}
		spec.Command = append(spec.Command, "start", "/etc/ze/bgp.conf")
	}
	return spec
}

// startContainer runs spec detached and records it for removal whenever the
// run may have left a container (interoplab.ContainerMayExist), so a run that
// the daemon created but could not start is removed too, while a run refused
// before docker ran, or refused because another container holds the name, is
// not. That last guard is the one cleanup relies on: the names carry this
// process's pid, and the single fixed perfSubnet makes a second concurrent
// perf run fail CreateNetwork on overlap before it starts any container, but
// neither stops a leftover of an earlier process that had the same pid.
func (runner *Runner) startContainer(ctx context.Context, spec interoplab.DetachedContainer) error {
	err := runner.Docker.RunDetached(ctx, spec)
	if interoplab.ContainerMayExist(err) {
		runner.containers[spec.Name] = struct{}{}
	}
	return err
}

// removeContainer removes one container this run started, on a context the
// run's cancellation does not reach, so an interrupted run still removes it.
// A name startContainer never recorded is not this run's, and is left alone.
// A removal that fails is reported and fails the run.
func (runner *Runner) removeContainer(ctx context.Context, name string) {
	if _, started := runner.containers[name]; !started {
		return
	}
	delete(runner.containers, name)
	if err := runner.Docker.RemoveContainer(context.WithoutCancel(ctx), name); err != nil {
		runner.removalFailed = true
		// A container still settling or still being removed already names the
		// cleanup command, so the line names it once.
		if interoplab.RemovalNamesCleanup(err) {
			say(runner.Stderr, "  error: remove container ", name, ": ", err.Error())
			return
		}
		say(runner.Stderr, "  error: remove container ", name, ": ", err.Error(), "; remove it with ", cleanupCommand)
	}
}

func (runner *Runner) startDUT(ctx context.Context, dut DUT) error {
	return runner.startContainer(ctx, runner.dutContainer(dut))
}
func (runner *Runner) waitDUT(ctx context.Context, name string) error {
	limit := 30 * time.Second
	if name == dutFreeRtr {
		limit = 45 * time.Second
	}
	container := runner.containerName(name)
	deadline := time.Now().Add(limit)
	var last error
	for time.Now().Before(deadline) {
		running, err := runner.Docker.ContainerRunning(ctx, container)
		if err == nil && running {
			return pause(ctx, 5*time.Second)
		}
		last = err
		if err := pause(ctx, time.Second); err != nil {
			return err
		}
	}
	if logs, err := runner.Docker.Logs(ctx, container, 20); err == nil {
		_, _ = io.WriteString(runner.Stdout, logs.Text)
	} else {
		say(runner.Stderr, "  warning: read the logs of ", container, ": ", err.Error())
	}
	if last != nil {
		return errors.New(name + " did not start within " + limit.String() + ": " + last.Error())
	}
	return errors.New(name + " did not start within " + limit.String())
}
func (runner *Runner) stop(ctx context.Context, name string) {
	runner.removeContainer(ctx, runner.containerName(name))
}

// prepareSender runs one setup command in the sender. A failure is a warning:
// the measurement that follows says whether the sender works.
func (runner *Runner) prepareSender(ctx context.Context, container string, argv ...string) {
	if _, err := runner.Docker.Exec(ctx, container, argv, nil); err != nil {
		say(runner.Stderr, "  warning: sender setup: ", err.Error())
	}
}

// senderContainer answers the sender: le mounted read-only, the results
// directory writable, and a sleep that keeps it up for the execs.
func (runner *Runner) senderContainer() interoplab.DetachedContainer {
	return interoplab.DetachedContainer{
		Name: runner.senderName(), Image: senderImage, Network: runner.network, Address: senderIP,
		Capabilities: []string{"NET_ADMIN"},
		Mounts: []interoplab.Mount{
			{Source: runner.LinuxBinary, Target: containerLe, ReadOnly: true},
			{Source: runner.resultsDir, Target: "/results"},
		},
		Command: []string{"sleep", "3600"},
	}
}

func (runner *Runner) runPerf(ctx context.Context, dut DUT, senderFirst bool) bool {
	container := runner.senderName()
	if err := runner.startContainer(ctx, runner.senderContainer()); err != nil {
		say(runner.Stderr, "  error: start the sender: ", err.Error())
		runner.removeContainer(ctx, container)
		return false
	}
	defer runner.removeContainer(ctx, container)
	runner.prepareSender(ctx, container, "ip", "addr", "add", netip.PrefixFrom(receiverIP, perfSubnet.Bits()).String(), "dev", "eth0")
	runner.prepareSender(ctx, container, "arping", "-U", "-c", "1", "-I", "eth0", receiverIP.String())
	if dut.Name == dutFreeRtr {
		runner.prepareSender(ctx, container, "apk", "add", "--no-cache", "ethtool")
		runner.prepareSender(ctx, container, "ethtool", "-K", "eth0", "tx", "off")
	}
	convergence := max(30, (runner.Routes/1000)*4+30)
	result := dut.Name + ".json"
	if senderFirst {
		result = dut.Name + "-propagation.json"
	}
	args := []string{containerLe, "perf", "send", "--dut-addr", dut.IP.String(), "--dut-port", strconv.Itoa(dut.Port), "--dut-asn", "65000", "--dut-name", dut.Name, "--sender-addr", senderIP.String(), "--sender-asn", "65001", "--receiver-addr", receiverIP.String(), "--receiver-asn", "65002", "--routes", strconv.Itoa(runner.Routes), "--seed", strconv.Itoa(runner.Seed), "--repeat", strconv.Itoa(runner.Repeat), "--warmup-runs", "1", "--iter-delay", "8s", "--warmup", "2s", "--connect-timeout", "20s", "--duration", strconv.Itoa(convergence) + "s", "--output", "/results/" + result}
	if dut.SenderPort != 0 {
		args = append(args, "--sender-port", strconv.Itoa(dut.SenderPort))
	}
	if dut.ReceiverPort != 0 {
		args = append(args, "--receiver-port", strconv.Itoa(dut.ReceiverPort))
	}
	if dut.Passive {
		args = append(args, "--passive-listen")
	}
	if senderFirst {
		args = append(args, "-sender-first")
	}
	timeout := time.Duration((1+runner.Repeat)*(convergence+40)+60) * time.Second
	send := interoplab.ContainerExec{Container: container, Command: args, Stdout: runner.Stdout, Stderr: runner.Stderr, Timeout: timeout}
	if _, err := runner.Docker.ExecAttached(ctx, send); err != nil {
		say(runner.Stderr, "  error: measure ", dut.Name, ": ", err.Error())
		return false
	}
	return true
}

// captureProfile writes one pprof endpoint of the running container to path.
func (runner *Runner) captureProfile(ctx context.Context, container, endpoint, path string, timeout time.Duration) error {
	file, err := os.Create(path) //nolint:gosec // path is PProfDir joined with one of this file's fixed profile names
	if err != nil {
		return err
	}
	url := "http://127.0.0.1:" + strconv.Itoa(runner.PProfPort) + "/debug/pprof/" + endpoint
	capture := interoplab.ContainerExec{Container: container, Command: []string{"le", "test", "http-get", url}, Stdout: file, Stderr: runner.Stderr, Timeout: timeout}
	_, runErr := runner.Docker.ExecAttached(ctx, capture)
	closeErr := file.Close()
	if runErr != nil {
		return runErr
	}
	return closeErr
}

// captureProfiles collects the CPU and the memory profiles. Profiling is
// best-effort: a failed capture is reported and the benchmark continues.
func (runner *Runner) captureProfiles(ctx context.Context, container, directory string) {
	if err := os.MkdirAll(directory, 0o750); err != nil {
		say(runner.Stderr, "perf: create profile directory ", directory, ": ", err.Error())
		return
	}
	if pause(ctx, 5*time.Second) != nil {
		return
	}
	cpu := "profile?seconds=" + strconv.Itoa(runner.PProfCPUSeconds)
	if err := runner.captureProfile(ctx, container, cpu, filepath.Join(directory, "cpu.pprof"), time.Duration(runner.PProfCPUSeconds+30)*time.Second); err != nil {
		say(runner.Stderr, "perf: capture cpu profile: ", err.Error())
	}
	for _, name := range []string{"heap", "allocs", "goroutine"} {
		if err := runner.captureProfile(ctx, container, name, filepath.Join(directory, name+".pprof"), 15*time.Second); err != nil {
			say(runner.Stderr, "perf: capture ", name, " profile: ", err.Error())
		}
	}
}

// cleanup removes every container this run started and still holds, then the
// network, on a context the run's cancellation does not reach.
func (runner *Runner) cleanup(ctx context.Context) {
	for _, name := range slices.Sorted(maps.Keys(runner.containers)) {
		runner.removeContainer(ctx, name)
	}
	if !runner.networkCreated {
		return
	}
	runner.networkCreated = false
	if err := runner.Docker.RemoveNetwork(context.WithoutCancel(ctx), runner.network); err != nil {
		runner.removalFailed = true
		say(runner.Stderr, "  error: remove network ", runner.network, ": ", err.Error(), "; remove it with ", cleanupCommand)
	}
}

// measurement is what the test step found over the selected DUTs.
type measurement struct {
	passed  int
	failed  []string
	skipped []string
	results []string
}

// Execute runs the selected steps over the named DUTs, or over every DUT when
// names is empty. It answers 2 when steps selects nothing, 1 when no DUT
// matches, a measurement fails, ctx ends the run, or a container, network or
// build tag this run created could not be removed, and 0 otherwise. A DUT
// whose image is absent is skipped and reported, never measured. ctx is the
// command's root: canceling it stops the run, and the removals still run.
func (runner *Runner) Execute(ctx context.Context, steps Steps, names []string) int {
	if !steps.Build && !steps.Test {
		say(runner.Stderr, "error: a suite run needs the build step, the test step, or both")
		return 2
	}
	requested := make(map[string]bool)
	for _, name := range names {
		requested[name] = true
	}
	duts := make([]DUT, 0)
	for _, dut := range DUTs() {
		if len(requested) == 0 || requested[dut.Name] {
			duts = append(duts, dut)
		}
	}
	if len(duts) == 0 {
		say(runner.Stderr, "error: no matching DUTs")
		return 1
	}
	if steps.Build {
		runner.buildImages(ctx, duts)
	}
	if !steps.Test {
		return runner.verdict(ctx)
	}
	if err := runner.buildLinuxBinary(ctx); err != nil {
		say(runner.Stderr, "build Linux binary: ", err.Error())
		return 1
	}
	if err := os.MkdirAll(runner.resultsDir, 0o750); err != nil {
		say(runner.Stderr, "error: create the results directory: ", err.Error())
		return 1
	}
	found, err := runner.measure(ctx, duts)
	runner.cleanup(ctx)
	if err != nil {
		say(runner.Stderr, "error: ", err.Error())
		return 1
	}
	if ctx.Err() == nil {
		runner.render(ctx, found.results)
	}
	runner.summarize(found)
	if len(found.failed) > 0 {
		return 1
	}
	return runner.verdict(ctx)
}

// verdict answers 0, or 1 when ctx ended the run or a removal failed:
// either one leaves the run's outcome untrusted or a resource behind.
func (runner *Runner) verdict(ctx context.Context) int {
	if err := ctx.Err(); err != nil {
		say(runner.Stderr, "FAIL  interrupted: ", err.Error())
		return 1
	}
	if runner.removalFailed {
		say(runner.Stderr, "FAIL  a resource of this run was not removed; run ", cleanupCommand)
		return 1
	}
	return 0
}

// measure creates the network and measures each DUT whose image exists. It
// answers an error when the network cannot be created or ctx ends the run
// between two DUTs; a DUT under way when ctx ends fails. The caller MUST
// call cleanup after it, whatever it answers.
func (runner *Runner) measure(ctx context.Context, duts []DUT) (measurement, error) {
	var found measurement
	network := interoplab.NetworkSpec{Name: runner.network, Candidates: []interoplab.Subnet{{IPv4: perfSubnet}}}
	if _, err := runner.Docker.CreateNetwork(ctx, network); err != nil {
		return found, err
	}
	runner.networkCreated = true
	for _, dut := range duts {
		if err := ctx.Err(); err != nil {
			return found, fmt.Errorf("the run was interrupted before %s: %w", dut.Name, err)
		}
		say(runner.Stdout, "-- ", dut.Name, " --")
		runner.measureIfPresent(ctx, dut, &found)
	}
	return found, nil
}

// measureIfPresent measures one DUT into found when its image exists, skips
// it when the image is absent, and fails it when the daemon cannot say.
func (runner *Runner) measureIfPresent(ctx context.Context, dut DUT, found *measurement) {
	present, err := runner.Docker.ImagePresent(ctx, dut.Image)
	if err != nil {
		say(runner.Stderr, "  error: inspect image ", dut.Image, ": ", err.Error())
		found.failed = append(found.failed, dut.Name)
		return
	}
	if !present {
		found.skipped = append(found.skipped, dut.Name)
		return
	}
	if !runner.measureDUT(ctx, dut, found) {
		found.failed = append(found.failed, dut.Name)
	}
	runner.stop(ctx, dut.Name)
}

// measureDUT starts one DUT, waits for it, and measures it into found. It
// answers false when the DUT did not start or its measurement failed; the
// caller removes the DUT's container either way.
func (runner *Runner) measureDUT(ctx context.Context, dut DUT, found *measurement) bool {
	if err := runner.startDUT(ctx, dut); err != nil {
		say(runner.Stderr, "  error: start ", dut.Name, ": ", err.Error())
		return false
	}
	if err := runner.waitDUT(ctx, dut.Name); err != nil {
		say(runner.Stderr, "  error: ", err.Error())
		return false
	}
	var profile sync.WaitGroup
	if dut.Name == dutZE && runner.PProf {
		profile.Go(func() {
			runner.captureProfiles(ctx, runner.containerName(dut.Name), filepath.Join(runner.PProfDir, strconv.Itoa(runner.Routes)))
		})
	}
	ok := runner.runPerf(ctx, dut, false)
	profile.Wait()
	if !ok {
		return false
	}
	found.passed++
	found.results = append(found.results, filepath.Join(runner.resultsDir, dut.Name+".json"))
	if dut.Name == dutZE && runner.runPerf(ctx, dut, true) {
		found.results = append(found.results, filepath.Join(runner.resultsDir, "ze-propagation.json"))
	}
	return true
}

// render prints the comparison and writes the HTML report and the doc snapshot.
func (runner *Runner) render(ctx context.Context, results []string) {
	if len(results) == 0 {
		return
	}
	if err := runner.Run(ctx, runner.Stdout, runner.Stderr, runner.Root, nil, runner.reportArgv("--md", results)); err != nil {
		say(runner.Stderr, "  warning: render the comparison: ", err.Error())
	}
	generateToFile(ctx, runner.Run, runner.reportArgv("--html", results), filepath.Join(runner.resultsDir, "report.html"), runner.Stderr)
	snapshot := filepath.Join(runner.runDir, "performance.md")
	if generateToFile(ctx, runner.Run, runner.reportArgv("--doc", results), snapshot, runner.Stderr) {
		if missing := unmeasuredDUTs(results); len(missing) > 0 {
			say(runner.Stdout, "  covers this run only; no result for: ", strings.Join(missing, ", "))
		}
		say(runner.Stdout, "Performance doc snapshot: ", snapshot)
		say(runner.Stdout, filepath.Join(runner.Root, "docs", "performance.md"), " is NOT written by a benchmark run")
	}
}

// summarize prints the one verdict line over what the test step found.
func (runner *Runner) summarize(found measurement) {
	parts := []string{}
	if found.passed > 0 {
		parts = append(parts, strconv.Itoa(found.passed)+" passed")
	}
	if len(found.failed) > 0 {
		parts = append(parts, strconv.Itoa(len(found.failed))+" failed: "+strings.Join(found.failed, " "))
	}
	if len(found.skipped) > 0 {
		parts = append(parts, strconv.Itoa(len(found.skipped))+" skipped: "+strings.Join(found.skipped, " "))
	}
	if len(parts) == 0 {
		parts = append(parts, "no DUTs ran")
	}
	verdict := "PASS"
	if len(found.failed) > 0 {
		verdict = "FAIL"
	}
	say(runner.Stdout, verdict, "  ", strings.Join(parts, ", "))
}

// reportArgv answers the Reporter command that renders results in one format.
// It builds a fresh slice, so no call shares a backing array with Reporter.
func (runner *Runner) reportArgv(format string, results []string) []string {
	argv := make([]string, 0, len(runner.Reporter)+1+len(results))
	argv = append(argv, runner.Reporter...)
	argv = append(argv, format)
	return append(argv, results...)
}

func FindRoot(start string) (string, error) {
	path, err := filepath.Abs(start)
	if err != nil {
		return "", err
	}
	for {
		if info, statErr := os.Stat(filepath.Join(path, "go.mod")); statErr == nil && info.Mode().IsRegular() {
			return path, nil
		}
		parent := filepath.Dir(path)
		if parent == path {
			return "", errors.New("repository root not found")
		}
		path = parent
	}
}
