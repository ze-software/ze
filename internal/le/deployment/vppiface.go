// Design: docs/architecture/testing/interop.md -- ze against a real VPP daemon
// Overview: actions.go -- the area table that reaches this run
// Detail: vppifacescenarios.go -- the four features and the configuration each needs
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

package deployment

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
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
// The tag floats because ligato publishes no pinned VPP release tag. This is
// the image's own publishing choice, not this proof's choice. The plugin probes
// exist because the contents of this image are not fixed.
const (
	VPPImage    = "ligato/vpp-base:latest"
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

// The three plugins the run reports on, whether or not a scenario needs them.
// The report names all three because "the image limits what can be proven" is
// the fact a reader of a green run has to be able to check.
const (
	WireguardPlugin = "wireguard_plugin.so"
	LinuxCPPlugin   = "linux_cp_plugin.so"
	LinuxNLPlugin   = "linux_nl_plugin.so"
)

// Where the container sees the run's scratch directory, and the CLI socket VPP
// creates in it. The checkout is mounted separately at /src, because the daemon
// is executed out of it.
const (
	vppMount   = "/run/vpp"
	vppCLISock = "/run/vpp/cli.sock"
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
	}
}

// containerArgs answers the container VPP and the daemon both run in.
//
// It is privileged because VPP claims hugepages and creates devices, and it
// holds the checkout at /src and the run's scratch directory at /run/vpp. The
// entry point is replaced by a sleep so the image's own VPP is not started
// before its configuration has been written.
func (v *vppIface) containerArgs(name, work string) []string {
	var tb textbuf.Buffer
	src := tb.Str(v.Tree).Str(":/src").String()
	tb.Reset()
	scratch := tb.Str(work).Byte(':').Str(vppMount).String()

	return []string{
		"run", "--rm", dockerDetach, "--privileged",
		"--platform", v.Platform,
		"--name", name,
		"-v", src,
		"-v", scratch,
		"-w", "/src",
		"--entrypoint", "sleep",
		v.Image,
		"infinity",
	}
}

// vppArgs answers VPP itself, started detached inside the container so the run
// keeps the container's foreground for the queries that follow.
func (v *vppIface) vppArgs(name string) []string {
	return []string{dockerExec, dockerDetach, name, "vpp", "-c", "/run/vpp/startup.conf"}
}

// daemonArgs answers ze, started inside the container reading the named
// configuration file out of the scratch mount.
//
// Blob storage is off and the configuration directory is inside the mount, so
// the run leaves nothing behind in the checkout. The `start` keyword is
// explicit because the bare `ze <config>` launch form was removed from the CLI
// (learned 1248) and a positional path now dies with "unknown command".
func (v *vppIface) daemonArgs(name, binaryRel, configFile string) []string {
	var tb textbuf.Buffer
	binary := tb.Str("/src/").Str(filepath.ToSlash(binaryRel)).String()
	tb.Reset()
	config := tb.Str(vppMount).Byte('/').Str(configFile).String()

	return []string{
		dockerExec, dockerInteractiveArg,
		dockerEnv, "ZE_LOG_VPP=info",
		dockerEnv, "ZE_LOG_INTERFACE=debug",
		dockerEnv, "ZE_LOG_BGP=info",
		dockerEnv, storageBlobDisabledEnv,
		dockerEnv, "ZE_CONFIG_DIR=/run/vpp/ze",
		name,
		binary,
		"start", config,
	}
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

// writeScratch creates the directory that VPP and ze both read. It contains
// VPP's own startup file, each scenario's configuration, and the empty directory
// where ze writes its configuration store.
func (v *vppIface) writeScratch(work string) error {
	if err := os.MkdirAll(filepath.Join(work, "ze"), 0o750); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(work, "startup.conf"), []byte(VPPStartupConfig), 0o644); err != nil { //nolint:gosec // a scratch file VPP reads inside a container
		return err
	}
	for i := range vppScenarios {
		one := &vppScenarios[i]
		if err := os.WriteFile(filepath.Join(work, one.file), []byte(one.config), 0o644); err != nil { //nolint:gosec // a scratch file ze reads inside a container
			return err
		}
	}
	return nil
}

// Run performs the proof and answers what happened.
//
// A step that the run cannot perform is an error. The operator has something to
// fix, and the run reached no verdict. A feature that did not appear is NOT an
// error. It is the verdict. The report includes that verdict, the query's own
// output and the daemon's last lines.
func (v *vppIface) Run() (VPPIfaceReport, error) {
	report := VPPIfaceReport{Image: v.Image}

	if err := look("docker", "go"); err != nil {
		return report, err
	}
	if err := ensureImage(v.Image, v.Progress); err != nil {
		return report, err
	}
	if err := buildDaemon(v.Tree, v.Goarch, v.Progress); err != nil {
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
	if err := v.startContainer(report.Container, work); err != nil {
		return report, err
	}
	defer removeContainer(report.Container)

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
	reported := []string{WireguardPlugin, LinuxCPPlugin, LinuxNLPlugin}
	loaded := make(map[string]bool, len(reported))
	for _, plugin := range reported {
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

	// The collector does not watch for a verdict. The verdict comes from VPP's
	// own command line, not from what ze logged. The collector forwards the
	// daemon's output to the operator, drains its pipe and retains the last lines
	// that the daemon wrote for a failure report.
	seen := newCollector()
	daemon := exec.CommandContext(context.Background(), "docker", //nolint:gosec // the argv is built above, never by an operator
		v.daemonArgs(container, daemonRel(v.Goarch), one.file)...)

	ze, err := startWatched(daemon, "ze> ", seen, v.Progress)
	if err != nil {
		return result, err
	}
	defer seen.wait()
	defer ze.stop()

	text, matched := v.awaitFeature(container, one)
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
func (v *vppIface) awaitFeature(container string, one *vppScenario) (string, bool) {
	deadline := time.Now().Add(v.ScenarioWait)
	last := ""
	for {
		text, ok := v.vppctl(container, one.probe)
		if ok {
			last = text
			if containsAny(text, one.needles) {
				return text, true
			}
		}
		if !time.Now().Before(deadline) {
			break
		}
		time.Sleep(scenarioPoll)
	}

	if one.fallback == "" {
		return last, false
	}

	text, ok := v.vppctl(container, one.fallback)
	if !ok {
		return last, false
	}
	return text, containsAny(text, one.fallbackNeedles)
}

// pluginLoaded reports whether VPP has loaded the named plugin.
//
// A query that FAILED is an error, not an absent plugin. If code reads those two
// results as one value, a container whose CLI socket has gone away answers
// "not loaded" for every plugin. The run then skips both gated scenarios and
// reports a pass. `plugin_loaded` (internal/le/deployment/vppiface.go) has
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
	full := make([]string, 0, len(argv)+2)
	full = append(full, dockerExec, container)
	full = append(full, argv...)
	return v.dockerText(full...)
}

// dockerText runs one complete docker argv and returns its combined output.
// It exists because docker exec flags can appear before the container name.
func (v *vppIface) dockerText(argv ...string) (string, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), vppctlTimeout)
	defer cancel()

	out, err := exec.CommandContext(ctx, "docker", argv...).CombinedOutput() //nolint:gosec // the argv is built by this package, never by an operator
	return string(out), err == nil
}

// startContainer starts the container the proof runs in.
func (v *vppIface) startContainer(name, work string) error {
	ctx, cancel := context.WithTimeout(context.Background(), dockerTimeout)
	defer cancel()

	start := exec.CommandContext(ctx, "docker", v.containerArgs(name, work)...) //nolint:gosec // the argv is built above, never by an operator
	start.Stderr = v.Progress
	if err := start.Run(); err != nil {
		return errors.New("failed to start the VPP evidence container")
	}
	return nil
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
	ctx, cancel := context.WithTimeout(context.Background(), dockerTimeout)
	defer cancel()

	start := exec.CommandContext(ctx, "docker", v.vppArgs(name)...) //nolint:gosec // the argv is built above, never by an operator
	start.Stderr = v.Progress
	if err := start.Run(); err != nil {
		return errors.New("failed to start VPP inside the evidence container")
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

// containerLog answers what the container has written for a failure with no
// other evidence. If this process cannot read the log, containerLog answers
// nothing. The caller already holds the failure that the report describes.
func (v *vppIface) containerLog(name string) string {
	ctx, cancel := context.WithTimeout(context.Background(), vppctlTimeout)
	defer cancel()

	out, err := exec.CommandContext(ctx, "docker", "logs", name).CombinedOutput() //nolint:gosec // the container name is this package's own
	if err != nil {
		return ""
	}
	return string(out)
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
