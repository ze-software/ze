// Design: docs/architecture/testing/interop.md -- ze against a real VPP daemon
// Overview: actions.go -- the area table that reaches this run
// Detail: vppifacescenarios.go -- the five features and the configuration each needs
// Detail: vppifacereport.go -- the payload this run answers
// Related: deployment.go -- the container and the collector
// Related: daemonbuild.go -- the daemon this proof builds
// Related: vppevidence.go -- the eight-scenario proof that shares this VPP lab
//
// vppiface.go proves that ze programs interface features on a VPP daemon that
// somebody else built. ze sends a GRE tunnel, a SPAN mirror, a wireguard
// interface and an LCP pair through ze's GoVPP binary API. It then asks vppctl
// -- VPP's own command line, not ze's -- whether each object exists. A stub
// proves only that ze agrees with ze. Only the real daemon's own answer proves
// that ze spoke the binary API correctly.
//
// The image determines how much the run can prove. wireguard and linux-cp are
// VPP plugins that the base image CAN ship or omit. Each scenario asks `show
// plugins` first. It records an evidence-backed SKIP when its plugin is absent.
//
// It MUST NOT treat a FAILED plugin query as an absent plugin. That error turns
// a broken container into two silent skips and a passing gate. This port fixes
// that defect. The Python still carries it
// (plan/journal/zero-value-as-valid-answer.md).

package testdeployment

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/interoplab"
)

// The dot-notation spellings of ZE_VPP_DOCKER_IMAGE, ZE_VPP_DOCKER_PLATFORM
// and ZE_VPP_DOCKER_GOARCH. env.Get matches case-insensitively and treats a dot
// and an underscore as the same character, so these keys read the variables the
// Python original read.
const (
	VPPImageKey    = "ze.vpp.docker.image"
	VPPPlatformKey = "ze.vpp.docker.platform"
	VPPGoarchKey   = "ze.vpp.docker.goarch"
)

// What the run uses when the operator names nothing.
//
// The tag names ligato's 26.06 release build, the latest stable VPP, so
// the VPP under the proof changes only when this line does. The plugin probes
// stay because a ligato build ships only some of VPP's plugins.
const (
	VPPImage    = "ligato/vpp-base:26.06-release"
	VPPPlatform = "linux/amd64"
	VPPGoarch   = "amd64"
)

var (
	vppImageEntry = stringSetting(VPPImageKey, VPPImage,
		"the container image the VPP interface proof runs the VPP daemon in")
	vppPlatformEntry = stringSetting(VPPPlatformKey, VPPPlatform,
		"the container platform the VPP interface proof runs in")
	vppGoarchEntry = stringSetting(VPPGoarchKey, VPPGoarch,
		"the GOARCH the VPP interface proof cross-compiles the daemon for")
)

// The four plugins the run reports on, whether or not a scenario needs them.
// The report names all four because "the image limits what can be proven" is
// the fact a reader of a green run has to be able to check.
const (
	WireguardPlugin = "wireguard_plugin.so"
	URPFPlugin      = "urpf_plugin.so"
	LinuxCPPlugin   = "linux_cp_plugin.so"
	LinuxNLPlugin   = "linux_nl_plugin.so"
)

// vppReportedPlugins is the order the report lists the four plugins in.
var vppReportedPlugins = []string{WireguardPlugin, LinuxCPPlugin, LinuxNLPlugin, URPFPlugin}

// VPPReportedPlugins names the plugins the interface proof reports on, in report
// order. The returned slice is a copy, so a caller cannot change the run's list.
func VPPReportedPlugins() []string {
	return slices.Clone(vppReportedPlugins)
}

// Where the container sees the run's scratch directory, and the CLI socket VPP
// creates in it. The checkout is mounted separately at /src, because the daemon
// is executed out of it.
const (
	vppMount   = "/run/vpp"
	vppCLISock = "/run/vpp/cli.sock"
	// zeRunDir is the container-private root under which each daemon's config
	// directory is staged, so the root daemon owns every ancestor of its store.
	zeRunDir = "/run/ze"
)

// The two bounds control the container's own startup. One bounds the time that
// VPP has to create its sockets. The other bounds the response time for one
// vppctl query.
//
// The socket wait is the Python's 30 seconds. Without the query bound, a vppctl
// command that does not return holds the scenario wait open. No process would
// detect it. A local CLI socket round trip takes much less than 30 seconds.
const (
	VPPSocketWait = 30 * time.Second
	vppctlTimeout = 30 * time.Second
)

// VPPScenarioWait bounds one scenario: how long ze gets to program the feature
// and VPP to show it. A feature that has not appeared 25 seconds after the
// daemon started has failed rather than stalled, and it is the Python's bound.
const VPPScenarioWait = 25 * time.Second

// The two poll intervals. Socket appearance is a filesystem event, so the run
// checks it often. A scenario probe costs a container exec. Thus, the run checks
// it less often. Both intervals are the Python's.
const (
	socketPoll   = 100 * time.Millisecond
	scenarioPoll = 500 * time.Millisecond
)

// vppIface is one run of the VPP interface-feature proof.
type vppIface struct {
	// Tree is the checkout: the daemon is built from it, and it is mounted
	// into the container at /src.
	Tree string
	// Image, Platform and Goarch say what VPP runs in and what the daemon is
	// built for.
	Image    string
	Platform string
	Goarch   string
	// SocketWait bounds the wait for VPP's own sockets. ScenarioWait bounds
	// the wait for each feature's appearance.
	SocketWait   time.Duration
	ScenarioWait time.Duration
	// Progress receives the daemon's output as it arrives, and the run's
	// narration. It is stderr for an operator, because the answer is the report
	// and a pipe operator must be able to carry it. It MUST be safe for
	// concurrent use: os.Stderr and io.Discard both are.
	Progress io.Writer
	// Docker is the client every container, exec and removal goes through.
	Docker *interoplab.Docker
	// ctx is the verb's signal root, set by Run (or by an integration test
	// that drives the steps itself): every docker call is bound to it, so a
	// SIGINT ends the run and the deferred removal still runs.
	ctx context.Context
}

// newVPPIface answers the run the command performs over tree, with every
// setting taken from the environment or from its default.
func newVPPIface(tree string) *vppIface {
	return &vppIface{
		Tree:         tree,
		Image:        setting(vppImageEntry.Key, VPPImage),
		Platform:     setting(vppPlatformEntry.Key, VPPPlatform),
		Goarch:       setting(vppGoarchEntry.Key, VPPGoarch),
		SocketWait:   VPPSocketWait,
		ScenarioWait: VPPScenarioWait,
		Progress:     os.Stderr,
		Docker:       interoplab.NewDocker(),
		ctx:          context.Background(),
	}
}

// containerSpec answers the container VPP and the daemon both run in.
//
// It holds NET_ADMIN and the host's /dev/net/tun and /dev/vhost-net, never
// --privileged: VPP's tap interfaces (the LCP pair) open a TUN device and a
// vhost-net backend, and ze programs the container's own network namespace.
// It holds no IPC_LOCK: on an amd64 Linux host VPP started, created its tap and
// moved traffic without it, and ze's memlock plugin fails soft. VPP needs no hugepages: with none
// reserved its buffer pool sits on default pages. It holds the checkout at /src and the
// run's scratch directory at /run/vpp. The entry point is replaced by a sleep
// so the image's own VPP is not started before its configuration has been
// written.
func (v *vppIface) containerSpec(name, work string) interoplab.DetachedContainer {
	return interoplab.DetachedContainer{
		Name:             name,
		Image:            v.Image,
		Platform:         v.Platform,
		Capabilities:     []string{"NET_ADMIN"},
		Devices:          []string{"/dev/net/tun", "/dev/vhost-net"},
		Mounts:           []interoplab.Mount{{Source: v.Tree, Target: srcMount}, {Source: work, Target: vppMount}},
		WorkingDirectory: srcMount,
		Entrypoint:       "sleep",
		Command:          []string{"infinity"},
	}
}

// vppDaemonCommand is VPP itself, started detached inside the container so the run
// keeps the container's foreground for the queries that follow.
var vppDaemonCommand = []string{"vpp", "-c", "/run/vpp/startup.conf"}

// daemonExec answers ze, started inside the container reading the named
// configuration file out of a private container-owned directory.
//
// Each scenario stages its input from the scratch mount before launch.
// The `start` keyword is explicit because the bare `ze <config>` launch form
// was removed from the CLI (learned 1248).
func (v *vppIface) daemonExec(name, binaryRel, configFile string) interoplab.ContainerExec {
	var tb textbuf.Buffer
	binary := tb.Str("/src/").Str(filepath.ToSlash(binaryRel)).String()
	config := filepath.Join(zeRunDir, configFile, "ze.conf")

	return interoplab.ContainerExec{
		Container: name,
		Command:   []string{binary, "start", config},
		Environment: []interoplab.EnvironmentVariable{
			{Name: "ZE_LOG_VPP", Value: zeLogInfo},
			{Name: "ZE_LOG_INTERFACE", Value: zeLogDebug},
			{Name: "ZE_LOG_BGP", Value: zeLogInfo},
			{Name: zeConfigDirVariable, Value: filepath.Dir(config)},
		},
		Interactive: true,
	}
}

// stageConfig copies the explicit input out of the host-owned scratch mount.
// The container's root daemon must own every ancestor of its database folder.
func (v *vppIface) stageConfig(container, configFile string) error {
	directory := filepath.Join(zeRunDir, configFile)
	if output, ok := v.containerText(container, "mkdir", "-p", directory); !ok {
		return errors.New("create private ze config directory: " + output)
	}
	source := filepath.Join(vppMount, "ze", configFile, "ze.conf")
	if output, ok := v.containerText(container, "cp", source, filepath.Join(directory, "ze.conf")); !ok {
		return errors.New("stage explicit ze configuration: " + output)
	}
	return nil
}

// vppctlArgs answers one query put to VPP's own command line, as the words
// docker is given. The query's words are separate arguments because vppctl
// takes a command as argv rather than as one string.
func vppctlArgs(query string) []string {
	words := strings.Fields(query)
	argv := make([]string, 0, len(words)+3)
	argv = append(argv, "vppctl", "-s", vppCLISock)
	return append(argv, words...)
}

// writeScratch creates VPP's startup file and the inputs stageConfig copies
// into each scenario's private configuration directory.
func (v *vppIface) writeScratch(work string) error {
	if err := os.MkdirAll(filepath.Join(work, "ze"), 0o750); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(work, "startup.conf"), []byte(VPPStartupConfig), 0o644); err != nil { //nolint:gosec // a scratch file VPP reads inside a container
		return err
	}
	for i := range vppScenarios {
		one := &vppScenarios[i]
		directory := filepath.Join(work, "ze", one.file)
		if err := os.MkdirAll(directory, 0o750); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(directory, "ze.conf"), []byte(one.config), 0o644); err != nil { //nolint:gosec // a scratch file ze reads inside a container
			return err
		}
	}
	return nil
}

// prepareDaemon readies the linux daemon and refuses a Docker kernel that
// lacks an enrolled feature, then readies the VPP image: the steps a proof that
// starts one VPP container and Ze in it takes first, vpp-iface-test and
// TestVPPSRv6ServiceRoute (build tag integration). The check comes before the
// image, so a refused host pulls nothing.
func (v *vppIface) prepareDaemon() error {
	if err := buildCheckedDaemon(v.ctx, v.Tree, v.Goarch, func() error { return buildDaemon(v.ctx, v.Tree, v.Goarch, v.Progress) }); err != nil {
		return err
	}
	return ensureImage(v.ctx, v.Docker, v.Image, v.Progress)
}

// Run performs the proof and answers what happened.
//
// A step that the run cannot perform is an error. The operator has something to
// fix, and the run reached no verdict. A feature that did not appear is NOT an
// error. It is the verdict. The report includes that verdict, the query's own
// output and the daemon's last lines.
//
// ctx is the verb's signal root: a SIGINT ends the run, and the container is
// still removed.
func (v *vppIface) Run(ctx context.Context) (report VPPIfaceReport, err error) {
	v.ctx = ctx
	report = VPPIfaceReport{Image: v.Image}

	if err := look("docker", "go"); err != nil {
		return report, err
	}
	if err := v.prepareDaemon(); err != nil {
		return report, err
	}

	work, err := scratchDir(v.Tree, "vpp-iface-")
	if err != nil {
		return report, err
	}
	if err := v.writeScratch(work); err != nil {
		return report, err
	}

	report.Container = vppContainerName()
	remove, err := v.startContainer(report.Container, work)
	if remove != nil {
		defer func() { report.ContainerRemoval = remove() }()
	}
	if err != nil {
		return report, err
	}

	if err := v.startVPP(report.Container, work); err != nil {
		return report, err
	}

	version, ok := v.vppctl(report.Container, "show version")
	if !ok {
		return report, errors.New("vppctl `show version` failed, so VPP is not answering its own command line")
	}
	report.Version = strings.TrimSpace(version)
	v.note(report.Version)

	return v.proveScenarios(report)
}

// proveScenarios probes the plugins, then runs each scenario until one fails.
//
// It stops at the first failure, as the Python did. The scenarios share one VPP
// daemon. A feature that the run was unable to program leaves that daemon in a state
// that the next scenario would use for its verdict.
func (v *vppIface) proveScenarios(report VPPIfaceReport) (VPPIfaceReport, error) {
	loaded := make(map[string]bool, len(vppReportedPlugins))
	for _, plugin := range vppReportedPlugins {
		state, err := v.pluginLoaded(report.Container, plugin)
		if err != nil {
			return report, err
		}
		loaded[plugin] = state
		report.Plugins = append(report.Plugins, PluginState{Name: plugin, Loaded: state})
	}

	for i := range vppScenarios {
		result, err := v.proveOne(report.Container, &vppScenarios[i], loaded)
		if err != nil {
			return report, err
		}
		report.Scenarios = append(report.Scenarios, result)
		if result.Outcome == OutcomeFail {
			return report, nil
		}
	}

	report.Passed = true
	return report, nil
}

// proveOne runs one scenario: start ze on its configuration, ask VPP whether
// the object exists, and answer what happened.
func (v *vppIface) proveOne(container string, one *vppScenario, loaded map[string]bool) (ScenarioResult, error) {
	result := ScenarioResult{Feature: one.feature}

	for _, plugin := range one.needsPlugins {
		if loaded[plugin] {
			continue
		}
		result.Outcome = OutcomeSkip
		result.Detail = one.skipDetail
		return result, nil
	}

	if err := v.stageConfig(container, one.file); err != nil {
		return result, err
	}

	// The collector does not watch for a verdict. The verdict comes from VPP's
	// own command line, not from what ze logged. The collector forwards the
	// daemon's output to the operator, drains its pipe and retains the last lines
	// that the daemon wrote for a failure report.
	seen := newCollector()
	ze, err := startWatchedExec(v.ctx, v.Docker, v.daemonExec(container, daemonRel(v.Goarch), one.file), "ze> ", seen, v.Progress)
	if err != nil {
		return result, err
	}
	defer seen.wait()
	defer ze.stop()

	text, matched, err := v.awaitFeature(container, one)
	if err != nil {
		return result, fmt.Errorf("the run stopped while VPP was asked for %s: %w", one.feature, err)
	}
	if !matched {
		result.Outcome = OutcomeFail
		result.Detail = one.missingDetail
		result.Evidence = tailLines(text)
		result.LogTail = seen.tailLines()
		return result, nil
	}

	result.Outcome = OutcomePass
	result.Detail = one.provenDetail
	if one.hostLinks {
		links, ok := v.containerText(container, "ip", "link", "show")
		if !ok {
			return result, errors.New("the LCP pair was created but the container's link listing could not be read")
		}
		result.Evidence = tailLines(links)
	}
	return result, nil
}

// awaitFeature polls VPP for the scenario's object until the bound. If the first
// query never matches, it asks the scenario's second query when the scenario has
// one.
//
// Only a query that SUCCEEDED contributes evidence. The Python matched the
// failed query's own error text. Thus, `show gre tunnel: unknown input` supplied
// the needle `gre`, and the fallback reported a tunnel that was never created
// (plan/journal/zero-value-as-valid-answer.md). awaitFeature always answers the
// last text it saw. Thus, a failure still reports what VPP said.
//
// An interrupted run answers the interruption as its error, at once: after a
// Ctrl-C every query fails, so polling on until the bound and answering "not
// matched" would turn the interruption into a FAIL verdict for a feature the run
// never finished probing.
func (v *vppIface) awaitFeature(container string, one *vppScenario) (string, bool, error) {
	deadline := time.Now().Add(v.ScenarioWait)
	last := ""
	for {
		text, ok := v.vppctl(container, one.probe)
		if ok {
			last = text
			if containsAny(text, one.needles) {
				return text, true, nil
			}
		}
		if err := v.ctx.Err(); err != nil {
			return last, false, err
		}
		if !time.Now().Before(deadline) {
			break
		}
		select {
		case <-v.ctx.Done():
			return last, false, v.ctx.Err()
		case <-time.After(scenarioPoll):
		}
	}

	if one.fallback == "" {
		return last, false, nil
	}

	text, ok := v.vppctl(container, one.fallback)
	if !ok {
		if err := v.ctx.Err(); err != nil {
			return last, false, err
		}
		return last, false, nil
	}
	return text, containsAny(text, one.fallbackNeedles), nil
}

// pluginLoaded reports whether VPP has loaded the named plugin.
//
// A query that FAILED is an error, not an absent plugin. If code reads those two
// results as one value, a container whose CLI socket has gone away answers
// "not loaded" for every plugin. The run then skips both gated scenarios and
// reports a pass. `plugin_loaded` (internal/le/test/deployment/vppiface.go) has
// this defect because it ignores the exit status and tests the error text for
// the plugin's name.
func (v *vppIface) pluginLoaded(container, plugin string) (bool, error) {
	text, ok := v.vppctl(container, "show plugins")
	if !ok {
		var tb textbuf.Buffer
		return false, errors.New(tb.Str("vppctl `show plugins` failed, so no plugin can be reported present or absent: ").
			Str(strings.TrimSpace(text)).String())
	}
	return strings.Contains(text, plugin), nil
}

// vppctl puts one query to VPP's own command line and answers its output and
// whether it succeeded.
func (v *vppIface) vppctl(container, query string) (string, bool) {
	return v.containerText(container, vppctlArgs(query)...)
}

// containerText runs one command inside the container and answers its combined
// output and whether it succeeded.
//
// containerText captures both streams. vppctl reports an unknown command on
// standard error and the answer to a known one on standard output. A reader of
// a failure needs the stream that contains the reason.
func (v *vppIface) containerText(container string, argv ...string) (string, bool) {
	return v.containerTextWith(container, nil, argv...)
}

// containerTextWith is containerText with an environment for the command.
func (v *vppIface) containerTextWith(container string, environ []interoplab.EnvironmentVariable, argv ...string) (string, bool) {
	var output textbuf.Buffer
	_, err := v.Docker.ExecAttached(v.ctx, interoplab.ContainerExec{
		Container: container, Command: argv, Environment: environ,
		Stdout: &output, Stderr: &output, Timeout: vppctlTimeout,
	})
	if err != nil {
		var tb textbuf.Buffer
		return tb.Str(output.String()).Str(err.Error()).String(), false
	}
	return output.String(), true
}

// startContainer starts the container the proof runs in, and answers the
// removal the caller MUST defer whenever docker may have created it, a failed
// start included (startProofContainer).
func (v *vppIface) startContainer(name, work string) (func() string, error) {
	remove, err := startProofContainer(v.ctx, v.Docker, v.containerSpec(name, work))
	if err != nil {
		return remove, fmt.Errorf("failed to start the VPP evidence container: %w", err)
	}
	return remove, nil
}

// startVPP starts VPP inside the container and waits for the two sockets it
// creates.
//
// The process waits for the sockets on the HOST side of the bind mount. This
// keeps the wait cheap. The scratch directory is the container's /run/vpp. When
// a socket appears there, this process can see the filesystem event.
//
// If VPP never creates its API socket, the container's own log is the only
// evidence of the cause. Thus, the error includes the log.
func (v *vppIface) startVPP(name, work string) error {
	if err := v.Docker.ExecDetached(v.ctx, name, vppDaemonCommand, nil); err != nil {
		return errors.New("failed to start VPP inside the evidence container: " + err.Error())
	}

	if !waitForPath(filepath.Join(work, "api.sock"), v.SocketWait) {
		var tb textbuf.Buffer
		return errors.New(tb.Str("the VPP API socket did not appear: ").
			Str(strings.TrimSpace(v.containerLog(name))).String())
	}
	if !waitForPath(filepath.Join(work, "cli.sock"), v.SocketWait) {
		return errors.New("the VPP CLI socket did not appear")
	}
	return nil
}

// containerLogLines bounds the container log a failed start reports.
const containerLogLines = 200

// containerLog answers what the container has written for a failure with no
// other evidence. If this process cannot read the log, containerLog says so
// with the reason: the caller already holds the failure the report describes,
// and an empty log would read as a container that wrote nothing.
func (v *vppIface) containerLog(name string) string {
	log, err := v.Docker.Logs(v.ctx, name, containerLogLines)
	if err != nil {
		return "(the container log could not be read: " + err.Error() + ")"
	}
	return log.Text
}

// note writes one line of narration to the progress stream. The report is the
// answer, so everything a person watches the run by goes to the other stream.
func (v *vppIface) note(line string) {
	if v.Progress == nil || line == "" {
		return
	}
	var tb textbuf.Buffer
	io.WriteString(v.Progress, tb.Str(line).Byte('\n').String()) //nolint:errcheck // progress output
}

// waitForPath reports whether path exists before the bound. It polls the
// filesystem instead of using a watch. Another process creates the file inside
// a container. Docker bind-mount implementations do not provide a portable
// inotify watch on the host.
func waitForPath(path string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for {
		if _, err := os.Stat(path); err == nil {
			return true
		}
		if !time.Now().Before(deadline) {
			return false
		}
		time.Sleep(socketPoll)
	}
}

// containsAny reports whether text carries any needle, without regard to case.
// Each needle is a VPP object name or a port number. The case of VPP command
// headings varies between releases.
func containsAny(text string, needles []string) bool {
	lowered := strings.ToLower(text)
	for _, needle := range needles {
		if strings.Contains(lowered, strings.ToLower(needle)) {
			return true
		}
	}
	return false
}

// tailLines answers the last lines of text, at most LogTailLines of them.
//
// Every tail in this package is bounded. A VPP interface listing grows with the
// interface count, so an unbounded tail CAN fill a JSON document with that
// listing. The lines that explain a failure are the last ones.
func tailLines(text string) []string {
	trimmed := strings.TrimRight(text, "\n")
	if trimmed == "" {
		return nil
	}
	lines := strings.Split(trimmed, "\n")
	if len(lines) > LogTailLines {
		lines = lines[len(lines)-LogTailLines:]
	}
	return lines
}

// vppContainerName answers a name that no other run on this machine will pick.
// Thus, two developers or two runs from one developer do not collide.
func vppContainerName() string {
	var tb textbuf.Buffer
	return tb.Str("ze-vpp-iface-").Int(int64(os.Getpid())).String()
}
