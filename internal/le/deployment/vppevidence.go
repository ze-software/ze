// Design: docs/architecture/testing/interop.md -- ze against a real VPP daemon
// Overview: actions.go -- the deployment action that reaches this run
// Detail: vppevidencescenarios.go -- the eight scenario inputs
// Detail: vppevidencerun.go -- the eight scenario producers
// Detail: vppevidencereport.go -- the structured result of each scenario
// Related: vppiface.go -- the shared VPP container and command machinery
//
// This runner ports the legacy effective-VPP harness without starting it. One VPP container is
// shared by eight ordered scenarios. A proof failure stops the sequence because
// the next scenario would inherit state that the failed scenario did not prove.
package deployment

import (
	"context"
	"errors"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/featuretags"
)

const (
	vppPeerWait       = 5 * time.Second
	vppApplyWait      = 25 * time.Second
	vppQueryWait      = 15 * time.Second
	vppWithdrawWait   = 15 * time.Second
	vppReconcileWait  = 25 * time.Second
	vppQueryPoll      = 500 * time.Millisecond
	vppPeerReadyLine  = "listening on"
	vppTrafficLogLine = "traffic-control config applied"
	vppFirewallLine   = "firewall config applied"
)

// VPP is one run of the eight legacy deployment scenarios. It
// embeds the existing VPP runner because both proofs use the same image,
// container, sockets, vppctl path, daemon settings, and bounded process helpers.
//
// Not safe for concurrent use.
type VPP struct {
	*vppIface
	iface string
}

type vppEvidenceRunStep struct {
	name string
	run  func(string, string) (VPPScenarioReport, error)
}

// newVPP answers the real VPP deployment run over tree.
func newVPP(tree string) *VPP {
	return &VPP{vppIface: newVPPIface(tree)}
}

// Run performs all eight scenarios and returns the completed report.
// Operating failures return an error. A VPP answer that disproves a claim is a
// failed report with a nil error.
func (v *VPP) Run() (VPPReport, error) {
	report := VPPReport{Image: v.Image}
	if err := look("docker", "go"); err != nil {
		return report, err
	}
	if err := ensureImage(v.Image, v.Progress); err != nil {
		return report, err
	}
	if err := v.buildBinaries(); err != nil {
		return report, err
	}

	work, err := scratchDir(v.Tree, "vpp-real-")
	if err != nil {
		return report, err
	}
	if err := v.writeEvidenceScratch(work); err != nil {
		return report, err
	}

	report.Container = vppEvidenceContainerName()
	if err := v.startContainer(report.Container, work); err != nil {
		return report, err
	}
	defer removeContainer(report.Container)

	if err := v.startVPP(report.Container, work); err != nil {
		return report, err
	}
	version, err := v.query(report.Container, "show version")
	if err != nil {
		return report, err
	}
	report.Version = strings.TrimSpace(version)
	v.note(report.Version)

	iface, err := v.createLoopback(report.Container)
	if err != nil {
		return report, err
	}
	report.Interface = iface
	v.iface = iface

	for _, step := range v.scenarioRuns() {
		scenario, runErr := step.run(report.Container, work)
		if runErr != nil {
			return report, runErr
		}
		if scenario.Scenario != step.name {
			var tb textbuf.Buffer
			return report, errors.New(tb.Str("VPP scenario ").Quoted(step.name).
				Str(" answered as ").Quoted(scenario.Scenario).String())
		}
		if appendVPPScenario(&report, scenario) {
			return report, nil
		}
	}
	report.Passed = true
	return report, nil
}

func (v *VPP) scenarioRuns() []vppEvidenceRunStep {
	return []vppEvidenceRunStep{
		{name: VPPScenarioIPsec, run: v.runIPsec},
		{name: VPPScenarioIPv4FIB, run: v.runIPv4FIB},
		{name: VPPScenarioMPLSFIB, run: v.runMPLSFIB},
		{name: VPPScenarioTrafficInterface, run: v.runTrafficInterface},
		{name: VPPScenarioTrafficProtocol, run: v.runTrafficProtocol},
		{name: VPPScenarioTrafficDSCP, run: v.runTrafficDSCP},
		{name: VPPScenarioTrafficMultiClass, run: v.runTrafficMultiClass},
		{name: VPPScenarioFirewall, run: v.runFirewall},
	}
}

func appendVPPScenario(report *VPPReport, scenario VPPScenarioReport) bool {
	report.Scenarios = append(report.Scenarios, scenario)
	return scenario.Verdict != VPPProofPass
}

func (v *VPP) buildBinaries() error {
	if err := buildDaemon(v.Tree, v.Goarch, v.Progress); err != nil {
		return err
	}
	tags, err := featuretags.DaemonTags(v.Tree)
	if err != nil {
		return err
	}
	var tb textbuf.Buffer
	testTags := tb.Str("ze_test").Byte(' ').Join(tags, " ").String()
	if err := v.runBuild([]string{
		"build", goBuildTagsArg, testTags, "-o", filepath.Join(v.Tree, vppTestRel(v.Goarch)), "./cmd/ze",
	}, errors.New("go build ze-test (-tags ze_test ./cmd/ze) failed")); err != nil {
		return err
	}
	return v.runBuild([]string{
		"test", "-c", goBuildTagsArg, "ze_core ze_vpp integration",
		"-o", filepath.Join(v.Tree, vppIPsecProbeRel(v.Goarch)),
		"./internal/component/ike/dataplane",
	}, errors.New("go test -c ./internal/component/ike/dataplane failed"))
}

func (v *VPP) runBuild(argv []string, failed error) error {
	out := ""
	for i := 0; i+1 < len(argv); i++ {
		if argv[i] == "-o" {
			out = argv[i+1]
			break
		}
	}
	if out != "" {
		if err := os.MkdirAll(filepath.Dir(out), 0o750); err != nil {
			return err
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), buildTimeout)
	defer cancel()

	build := exec.CommandContext(ctx, "go", argv...) //nolint:gosec // the argv is package data and manifest tags
	build.Dir = v.Tree
	build.Stdout = v.Progress
	build.Stderr = v.Progress
	var tb textbuf.Buffer
	build.Env = append(os.Environ(), "GOOS=linux", tb.Str("GOARCH=").Str(v.Goarch).String(), "CGO_ENABLED=0")
	if os.Getenv("GOCACHE") == "" {
		build.Env = append(build.Env, tb.Reset().Str("GOCACHE=").Str(filepath.Join(v.Tree, "tmp", "go-cache")).String())
	}
	if err := build.Run(); err != nil {
		return failed
	}
	return nil
}

func (v *VPP) writeEvidenceScratch(work string) error {
	if err := os.MkdirAll(filepath.Join(work, "ze"), 0o750); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(work, "startup.conf"), []byte(VPPStartupConfig), 0o644) //nolint:gosec // a scratch file VPP reads
}

func (v *VPP) query(container, command string) (string, error) {
	text, ok := v.vppctl(container, command)
	return requireVPPQuery(text, ok, command)
}

func requireVPPQuery(text string, ok bool, command string) (string, error) {
	if ok {
		return text, nil
	}
	var tb textbuf.Buffer
	return "", errors.New(tb.Str("vppctl ").Quoted(command).Str(" failed: ").Str(strings.TrimSpace(text)).String())
}

func (v *VPP) createLoopback(container string) (string, error) {
	text, err := v.query(container, "create loopback interface")
	if err != nil {
		return "", err
	}
	iface := parseVPPInterface(text)
	if _, err := v.query(container, vppCommand("set interface state", iface, "up")); err != nil {
		return "", err
	}
	interfaces, err := v.query(container, "show interface")
	if err != nil {
		return "", err
	}
	if !strings.Contains(interfaces, iface) {
		var tb textbuf.Buffer
		return "", errors.New(tb.Str("created VPP loopback ").Quoted(iface).
			Str(" not visible in show interface: ").Str(interfaces).String())
	}
	return iface, nil
}

func parseVPPInterface(text string) string {
	for token := range strings.FieldsSeq(text) {
		if !strings.HasPrefix(token, "loop") {
			continue
		}
		if _, err := strconv.Atoi(strings.TrimPrefix(token, "loop")); err == nil {
			return token
		}
	}
	return "loop0"
}

func parseVPPInterfaceIndex(text, iface string) (int, error) {
	for line := range strings.SplitSeq(text, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		if fields[0] != iface {
			continue
		}
		index, err := strconv.Atoi(fields[1])
		if err != nil {
			var tb textbuf.Buffer
			return 0, errors.New(tb.Str("interface ").Quoted(iface).Str(" has an invalid index in show interface: ").Str(text).String())
		}
		return index, nil
	}
	var tb textbuf.Buffer
	return 0, errors.New(tb.Str("interface ").Quoted(iface).Str(" has no index in show interface: ").Str(text).String())
}

func vppCommand(parts ...string) string {
	return textbuf.Join(parts, " ")
}

func vppTestRel(goarch string) string {
	var tb textbuf.Buffer
	return filepath.Join("tmp", "evidence", "bin", tb.Str("ze-test-linux-").Str(goarch).String())
}

func vppIPsecProbeRel(goarch string) string {
	var tb textbuf.Buffer
	return filepath.Join("tmp", "evidence", "bin", tb.Str("ipsec-vpp-linux-").Str(goarch).String())
}

func vppEvidenceContainerName() string {
	var tb textbuf.Buffer
	return tb.Str("ze-vpp-evidence-").Int(int64(os.Getpid())).String()
}

func (v *VPP) freePort() (int, error) {
	var listen net.ListenConfig
	listener, err := listen.Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer listener.Close() //nolint:errcheck // the operating result is the allocated port
	addr, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		return 0, errors.New("the free-port listener did not return a TCP address")
	}
	return addr.Port, nil
}

func passVPPCheck(check, detail string, evidence ...string) VPPCheck {
	return VPPCheck{Check: check, Verdict: VPPProofPass, Detail: detail, Evidence: evidence}
}

func failVPPCheck(check, detail, evidence string, seen *collector) VPPCheck {
	result := VPPCheck{Check: check, Verdict: VPPProofFail, Detail: detail}
	if evidence != "" {
		result.Evidence = tailLines(evidence)
	}
	if seen != nil {
		result.LogTail = seen.tailLines()
	}
	return result
}

func finishVPPScenario(scenario string, checks []VPPCheck) VPPScenarioReport {
	result := VPPScenarioReport{Scenario: scenario, Verdict: VPPProofFail, Checks: checks}
	if len(checks) == 0 {
		return result
	}
	result.Verdict = VPPProofPass
	for i := range checks {
		if checks[i].Verdict != VPPProofPass {
			result.Verdict = VPPProofFail
			break
		}
	}
	return result
}

func (v *VPP) awaitQuery(container, command string, want bool, timeout time.Duration, present func(string) bool) (bool, string, error) {
	deadline := time.Now().Add(timeout)
	last := ""
	for {
		text, err := v.query(container, command)
		if err != nil {
			return false, last, err
		}
		last = text
		if present(text) == want {
			return true, text, nil
		}
		if !time.Now().Before(deadline) {
			return false, last, nil
		}
		time.Sleep(vppQueryPoll)
	}
}

func (v *VPP) writeConfig(work, name, content string) error {
	return os.WriteFile(filepath.Join(work, name), []byte(content), 0o644) //nolint:gosec // a scratch config ze reads
}

func (v *VPP) startEvidenceDaemon(container, configFile string, port int) (*running, *collector, error) {
	seen := newCollector(vppTrafficLogLine, vppFirewallLine)
	argv := v.evidenceDaemonArgs(container, configFile, port)
	cmd := exec.CommandContext(context.Background(), "docker", argv...) //nolint:gosec // the argv is package data
	daemon, err := startWatched(cmd, "ze> ", seen, v.Progress)
	if err != nil {
		return nil, nil, err
	}
	return daemon, seen, nil
}

func (v *VPP) evidenceDaemonArgs(container, configFile string, port int) []string {
	var tb textbuf.Buffer
	binary := tb.Str("/src/").Str(filepath.ToSlash(daemonRel(v.Goarch))).String()
	config := tb.Reset().Str(vppMount).Byte('/').Str(configFile).String()
	argv := []string{
		dockerExec, dockerInteractiveArg,
		dockerEnv, "ZE_LOG_VPP=info",
		dockerEnv, "ZE_LOG_FIB_VPP=debug",
		dockerEnv, "ZE_LOG_TRAFFIC=debug",
		dockerEnv, "ZE_LOG_TRAFFIC_VPP=debug",
		dockerEnv, "ZE_LOG_FIREWALL=debug",
		dockerEnv, "ZE_LOG_FIREWALL_VPP=debug",
		dockerEnv, "ZE_LOG_BGP=info",
		dockerEnv, storageBlobDisabledEnv,
		dockerEnv, "ZE_CONFIG_DIR=/run/vpp/ze",
	}
	if port != 0 {
		argv = append(argv, dockerEnv, tb.Reset().Str("ZE_TEST_BGP_PORT=").Int(int64(port)).String())
	}
	return append(argv, container, binary, "start", config)
}

func stopVPPProcess(proc *running, seen *collector) {
	proc.stop()
	seen.wait()
}

func (v *VPP) waitDaemonLine(seen *collector, proc *running, line string) bool {
	return await(seen, line, proc, v.vppApplyWait())
}

func (v *VPP) vppApplyWait() time.Duration {
	if v.ScenarioWait > 0 {
		return v.ScenarioWait
	}
	return vppApplyWait
}
