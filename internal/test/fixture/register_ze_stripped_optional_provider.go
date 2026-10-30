// Design: docs/architecture/plugin/plugin-system.md -- registration fields, config needs
// Related: ui_fixture_ze_stripped_no_bgp.go -- validateZEStrippedConfig, the validate door this fixture shares

package fixture

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

func init() {
	Register("ui/ze-stripped-optional-provider", uiDriver(runZEStrippedOptionalProvider))
}

// strippedOptionalProviderBootPolls bounds the boot wait: 150 polls of 200ms
// is the 30 seconds a loaded machine has needed to start the daemon.
const (
	strippedOptionalProviderBootPolls = 150
	strippedOptionalProviderPollDelay = 200 * time.Millisecond
)

// strippedOptionalProviderBootVerdict is what one boot attempt showed.
type strippedOptionalProviderBootVerdict struct {
	refused bool   // the daemon exited by itself before it reported running
	log     string // everything the daemon wrote
}

// runZEStrippedOptionalProvider drives ze-stripped, which is built without
// ze_vpp, through the two doors a config passes before it is applied: `ze
// config validate` (VerifyPluginConfigMapTransition) and daemon boot
// (LoadConfig). A config that sets `traffic control backend vpp` needs the vpp
// component, so each door must refuse it naming the leaf and the build tag,
// and the same root with the tc backend must be accepted.
func runZEStrippedOptionalProvider(ctx context.Context) error {
	// 1. The validate door refuses the leaf that needs the absent provider.
	refused, err := validateZEStrippedConfig(ctx, "traffic-vpp.conf")
	if err != nil {
		return err
	}
	if refused.exitCode == 0 {
		return fmt.Errorf("ze config validate ACCEPTED traffic control backend vpp on a build without ze_vpp: out=%q", refused.output)
	}
	if err := requireOptionalProviderRefusal("validate", refused.output); err != nil {
		return err
	}

	// 2. The same root with the leaf off is accepted. Without this, step 1
	// would pass just as well on a binary that refused every traffic root.
	accepted, err := validateZEStrippedConfig(ctx, "traffic-tc.conf")
	if err != nil {
		return err
	}
	if accepted.exitCode != 0 {
		return fmt.Errorf("traffic control backend tc rejected by ze-stripped: rc=%d out=%q", accepted.exitCode, accepted.output)
	}

	// 3. The boot door refuses the same config before the daemon runs.
	boot, err := bootZEStrippedConfig(ctx, "traffic-vpp.conf")
	if err != nil {
		return err
	}
	if !boot.refused {
		return fmt.Errorf("ze-stripped BOOTED with traffic control backend vpp on a build without ze_vpp:\n%s", boot.log)
	}
	if err := requireOptionalProviderRefusal("boot", boot.log); err != nil {
		return err
	}
	// The refusal comes before any plugin starts. Before this door existed the
	// daemon did refuse, but only once the traffic plugin failed its config
	// stage, and that late failure is the one logged as a plugin startup failure.
	if strings.Contains(boot.log, "plugin startup failed") {
		return fmt.Errorf("ze-stripped refused the config only after a plugin started:\n%s", boot.log)
	}

	// 4. The interface and firewall backend leaves, steps 1 to 3 for each.
	if err := runStrippedVPPBackendSteps(ctx); err != nil {
		return err
	}

	_, err = fmt.Fprintln(os.Stdout, "ze-stripped: absent optional provider refused at validate and boot, leaf off accepted")
	return err
}

// requireOptionalProviderRefusal checks that a refusal names what the operator
// typed and the build tag that would carry the provider.
func requireOptionalProviderRefusal(door, output string) error {
	if !strings.Contains(output, "traffic control backend") {
		return fmt.Errorf("%s refusal does not name the leaf `traffic control backend`: %q", door, output)
	}
	if !strings.Contains(output, "ze_vpp") {
		return fmt.Errorf("%s refusal does not name the build tag ze_vpp: %q", door, output)
	}
	return nil
}

// bootZEStrippedConfig boots ze-stripped with the config at path
// (bootStrippedConfig).
func bootZEStrippedConfig(ctx context.Context, path string) (strippedOptionalProviderBootVerdict, error) {
	return bootStrippedConfig(ctx, "ze-stripped", path)
}

// strippedDaemonRunning is the line a YANG-config daemon prints once it runs
// (cmd/ze/hub/main.go, runYANGConfig).
const strippedDaemonRunning = "Ze running"

// strippedReloadComplete is the line a finished SIGHUP reload prints
// (cmd/ze/hub/main_reload.go, reloadCompleteLine).
const strippedReloadComplete = "sighup reload complete"

// bootStrippedConfig starts `<binary> -` with the config on stdin and answers
// whether the daemon refused it: it exited by itself before logging that it
// runs. A daemon that reports running, or is still up when the wait ends, is
// stopped, and that is not a refusal.
func bootStrippedConfig(ctx context.Context, binary, path string) (strippedOptionalProviderBootVerdict, error) {
	config, err := os.Open(path) //nolint:gosec // the path is the fixture's own scratch file
	if err != nil {
		return strippedOptionalProviderBootVerdict{}, err
	}
	defer config.Close() //nolint:errcheck // read-only fixture input

	var log bytes.Buffer
	command := exec.CommandContext(ctx, binary, "-") //nolint:gosec // the fixture chooses the program and its arguments
	command.Stdin = config
	command.Stdout = &log
	command.Stderr = &log
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := command.Start(); err != nil {
		return strippedOptionalProviderBootVerdict{}, fmt.Errorf("start %s boot of %q: %w", binary, path, err)
	}
	done := make(chan error, 1)
	go waitStrippedOptionalProviderBoot(command, done)

	exited := Poll(ctx, strippedOptionalProviderBootPolls, strippedOptionalProviderPollDelay, func() bool {
		select {
		case <-done:
			return true
		default:
			return false
		}
	})
	if !exited {
		stopStrippedOptionalProviderBoot(command, done)
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		return strippedOptionalProviderBootVerdict{}, ctxErr
	}

	// The buffer is read only once Wait has returned, because until then its
	// copy goroutine can still write to it.
	output := log.String()
	if strings.Contains(output, strippedDaemonRunning) {
		return strippedOptionalProviderBootVerdict{refused: false, log: output}, nil
	}
	return strippedOptionalProviderBootVerdict{refused: exited, log: output}, nil
}

// waitStrippedOptionalProviderBoot reports the daemon's exit on done. It is
// the one goroutine bootZEStrippedConfig starts, and it ends when the process
// does.
func waitStrippedOptionalProviderBoot(command *exec.Cmd, done chan<- error) {
	done <- command.Wait()
}

// stopStrippedOptionalProviderBoot ends a daemon that is still running, with
// its whole process group. It MUST be called only while done has not fired,
// and it returns once the process has exited.
func stopStrippedOptionalProviderBoot(command *exec.Cmd, done <-chan error) {
	if err := syscall.Kill(-command.Process.Pid, syscall.SIGTERM); err != nil && !errors.Is(err, syscall.ESRCH) {
		_ = command.Process.Kill() //nolint:errcheck // best effort after a failed SIGTERM; the wait below bounds it
	}
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		_ = syscall.Kill(-command.Process.Pid, syscall.SIGKILL) //nolint:errcheck // the process may already be gone
		<-done
	}
}

// strippedVPPBackendLeaf is one backend leaf whose vpp value needs the vpp
// component: the leaf as the operator types it, the config that sets it to
// vpp, and the config that sets it to the always-built backend.
type strippedVPPBackendLeaf struct {
	leaf     string
	vppConf  string
	homeConf string
}

// runStrippedVPPBackendSteps runs steps 1 to 3 of runZEStrippedOptionalProvider
// for `interface backend vpp` and `firewall backend vpp` (AC-8, AC-9). Both
// roots are ungated, so ze-stripped parses them, while the vpp component and
// each VPP backend are compiled out with ze_vpp. Before the spec, validate
// accepted both configs, because neither verifier refuses an unregistered
// backend, and boot failed late in LoadBackend naming no tag.
func runStrippedVPPBackendSteps(ctx context.Context) error {
	leaves := []strippedVPPBackendLeaf{
		{leaf: "interface backend", vppConf: "interface-vpp.conf", homeConf: "interface-netlink.conf"},
		{leaf: "firewall backend", vppConf: "firewall-vpp.conf", homeConf: "firewall-nft.conf"},
	}
	for _, step := range leaves {
		refused, err := validateZEStrippedConfig(ctx, step.vppConf)
		if err != nil {
			return err
		}
		if refused.exitCode == 0 {
			return fmt.Errorf("ze config validate ACCEPTED %s vpp on a build without ze_vpp: out=%q", step.leaf, refused.output)
		}
		if err := requireVPPBackendRefusal("validate", step.leaf, refused.output); err != nil {
			return err
		}

		accepted, err := validateZEStrippedConfig(ctx, step.homeConf)
		if err != nil {
			return err
		}
		if accepted.exitCode != 0 {
			return fmt.Errorf("%s with the always-built backend rejected by ze-stripped: rc=%d out=%q", step.leaf, accepted.exitCode, accepted.output)
		}

		boot, err := bootZEStrippedConfig(ctx, step.vppConf)
		if err != nil {
			return err
		}
		if !boot.refused {
			return fmt.Errorf("ze-stripped BOOTED with %s vpp on a build without ze_vpp:\n%s", step.leaf, boot.log)
		}
		if err := requireVPPBackendRefusal("boot", step.leaf, boot.log); err != nil {
			return err
		}
		if strings.Contains(boot.log, "plugin startup failed") {
			return fmt.Errorf("ze-stripped refused %s vpp only after a plugin started:\n%s", step.leaf, boot.log)
		}
	}
	return nil
}

// requireVPPBackendRefusal checks that a refusal names the backend leaf the
// operator typed and the build tag ze_vpp.
func requireVPPBackendRefusal(door, leaf, output string) error {
	if !strings.Contains(output, leaf) {
		return fmt.Errorf("%s refusal does not name the leaf `%s`: %q", door, leaf, output)
	}
	if !strings.Contains(output, "ze_vpp") {
		return fmt.Errorf("%s refusal of `%s vpp` does not name the build tag ze_vpp: %q", door, leaf, output)
	}
	return nil
}

func init() {
	Register("ui/ze-stripped-flow-optional-provider", uiDriver(runZEStrippedFlowOptionalProvider))
}

// strippedFlowBinary is the functional build that carries flow export, DDoS
// and anomaly detection and no ze_bgp (internal/le/test/functional/binaries.go,
// strippedFlowTags), so a present plugin's setting can need the absent BGP
// subsystem.
const strippedFlowBinary = "ze-stripped-flow"

// strippedFlowReloadConfig is the file the reload step boots from and rewrites.
const strippedFlowReloadConfig = "ddos-reload.conf"

// runZEStrippedFlowOptionalProvider drives ze-stripped-flow through every door
// a config passes before it is applied. `flow-export enrichment bgp true`
// needs bgp-rib and `ddos flowspec response-level enforce` needs bgp, and the
// build carries neither: validate, boot and SIGHUP reload each refuse the
// setting naming it and ze_bgp, and the same roots with the setting off are
// accepted. Each accept step is what makes its refusal step discriminate: a
// binary that refused every flow-export or ddos root would fail it.
//
// The reload step boots DDoS FlowSpec, not flow export: flow export
// hard-depends on the interface component, which has no backend on darwin, so
// a flow-export daemon cannot boot there. Flow export with enrichment off is
// therefore proven by validate only here (AC-2); its boot needs Linux.
func runZEStrippedFlowOptionalProvider(ctx context.Context) error {
	// 1. AC-1: the validate door refuses BGP enrichment.
	refused, err := validateStrippedConfig(ctx, strippedFlowBinary, "flow-enrich-on.conf")
	if err != nil {
		return err
	}
	if refused.exitCode == 0 {
		return fmt.Errorf("ze config validate ACCEPTED flow-export enrichment bgp true on a build without ze_bgp: out=%q", refused.output)
	}
	if err := requireBGPNeedRefusal("validate", refused.output, "flow-export enrichment bgp"); err != nil {
		return err
	}

	// 2. AC-2: the same root with enrichment off is accepted.
	accepted, err := validateStrippedConfig(ctx, strippedFlowBinary, "flow-enrich-off.conf")
	if err != nil {
		return err
	}
	if accepted.exitCode != 0 {
		return fmt.Errorf("flow-export with enrichment off rejected by %s: rc=%d out=%q", strippedFlowBinary, accepted.exitCode, accepted.output)
	}

	// 3. AC-3: the boot door refuses BGP enrichment before any plugin starts.
	boot, err := bootStrippedConfig(ctx, strippedFlowBinary, "flow-enrich-on.conf")
	if err != nil {
		return err
	}
	if !boot.refused {
		return fmt.Errorf("%s BOOTED with flow-export enrichment bgp true on a build without ze_bgp:\n%s", strippedFlowBinary, boot.log)
	}
	if err := requireBGPNeedRefusal("boot", boot.log, "flow-export enrichment bgp"); err != nil {
		return err
	}
	if strings.Contains(boot.log, "plugin startup failed") {
		return fmt.Errorf("%s refused enrichment only after a plugin started:\n%s", strippedFlowBinary, boot.log)
	}

	// 4. AC-12: DDoS FlowSpec enforcement needs the BGP engine; alert does not.
	enforce, err := validateStrippedConfig(ctx, strippedFlowBinary, "ddos-enforce.conf")
	if err != nil {
		return err
	}
	if enforce.exitCode == 0 {
		return fmt.Errorf("ze config validate ACCEPTED ddos flowspec response-level enforce on a build without ze_bgp: out=%q", enforce.output)
	}
	if err := requireBGPNeedRefusal("validate", enforce.output, "ddos flowspec response-level"); err != nil {
		return err
	}
	alert, err := validateStrippedConfig(ctx, strippedFlowBinary, "ddos-alert.conf")
	if err != nil {
		return err
	}
	if alert.exitCode != 0 {
		return fmt.Errorf("ddos flowspec response-level alert rejected by %s: rc=%d out=%q", strippedFlowBinary, alert.exitCode, alert.output)
	}

	// 5. AC-4: the daemon boots with FlowSpec alert, refuses a SIGHUP reload
	// to enforce, keeps running, and still reloads.
	if err := reloadStrippedFlowSpecEnforce(ctx); err != nil {
		return err
	}

	_, err = fmt.Fprintln(os.Stdout, "ze-stripped-flow: BGP-needing settings refused at validate, boot and reload, settings off accepted")
	return err
}

// requireBGPNeedRefusal checks that a refusal names the setting the operator
// typed and that its fix stands on its own line naming ze_bgp.
func requireBGPNeedRefusal(door, output, setting string) error {
	if !strings.Contains(output, setting) {
		return fmt.Errorf("%s refusal does not name the setting `%s`: %q", door, setting, output)
	}
	for line := range strings.SplitSeq(output, "\n") {
		fix, isFix := strings.CutPrefix(strings.TrimSpace(line), "fix:")
		if !isFix {
			continue
		}
		if !strings.Contains(fix, "ze_bgp") {
			return fmt.Errorf("%s refusal's fix line does not name the build tag ze_bgp: %q", door, line)
		}
		return nil
	}
	return fmt.Errorf("%s refusal carries no fix line: %q", door, output)
}

// reloadStrippedFlowSpecEnforce boots ze-stripped-flow from a file with DDoS
// FlowSpec `response-level alert`, rewrites the file to `enforce` and sends
// SIGHUP: the reload MUST be refused naming the setting and ze_bgp, MUST NOT
// report completion, and MUST leave the daemon running. The file then goes
// back to alert and a second SIGHUP MUST complete, which proves the first
// refusal was this setting's and not a reload path that fails for everything.
//
// Between the two reloads the step asks the daemon, over SSH, for the
// configuration it applied (`show config running`): it MUST still answer
// alert. The file holds enforce at that moment, so only an answer from the
// daemon's memory can say what the refusal left in force.
func reloadStrippedFlowSpecEnforce(ctx context.Context) error {
	login, err := strippedFlowLoginBlock(ctx)
	if err != nil {
		return err
	}
	alert, err := os.ReadFile("ddos-alert.conf")
	if err != nil {
		return err
	}
	alert = append(alert, login...)
	enforce, err := os.ReadFile("ddos-enforce.conf")
	if err != nil {
		return err
	}
	enforce = append(enforce, login...)
	if err := os.WriteFile(strippedFlowReloadConfig, alert, 0o600); err != nil {
		return err
	}
	configDir, err := os.MkdirTemp("", "stripped-flow-reload-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(configDir) //nolint:errcheck // fixture cleanup
	sshAddressPath := filepath.Join(configDir, "ssh.addr")

	var daemon lockedBuffer
	command := exec.CommandContext(ctx, strippedFlowBinary, "start", strippedFlowReloadConfig) //nolint:gosec // the fixture chooses the program and its arguments
	command.Env = miscEnvironment(map[string]string{envConfigDir: configDir, envSSHEphemeral: sshAddressPath})
	command.Stdout = &daemon
	command.Stderr = &daemon
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := command.Start(); err != nil {
		return fmt.Errorf("start %s with ddos flowspec response-level alert: %w", strippedFlowBinary, err)
	}
	done := make(chan error, 1)
	go waitStrippedOptionalProviderBoot(command, done)
	defer stopManagedProcess(command, done)

	running := func() bool { return strings.Contains(daemon.String(), strippedDaemonRunning) }
	if !Poll(ctx, strippedOptionalProviderBootPolls, strippedOptionalProviderPollDelay, running) {
		return fmt.Errorf("%s did not boot with ddos flowspec response-level alert:\n%s", strippedFlowBinary, daemon.String())
	}

	before := len(daemon.String())
	if err := os.WriteFile(strippedFlowReloadConfig, enforce, 0o600); err != nil {
		return err
	}
	if err := syscall.Kill(command.Process.Pid, syscall.SIGHUP); err != nil {
		return err
	}
	refusedReload := func() bool { return strings.Contains(daemon.String()[before:], "ze_bgp") }
	if !Poll(ctx, strippedOptionalProviderBootPolls, strippedOptionalProviderPollDelay, refusedReload) {
		return fmt.Errorf("SIGHUP reload to ddos flowspec response-level enforce was not refused naming ze_bgp:\n%s", daemon.String())
	}
	refusal := daemon.String()[before:]
	if strings.Contains(refusal, strippedReloadComplete) {
		return fmt.Errorf("SIGHUP reload reported complete over a refused setting:\n%s", refusal)
	}
	if err := requireBGPNeedRefusal("reload", refusal, "ddos flowspec response-level enforce"); err != nil {
		return err
	}
	select {
	case <-done:
		return fmt.Errorf("%s exited after refusing the reload:\n%s", strippedFlowBinary, daemon.String())
	default:
	}
	level, err := strippedFlowRunningLevel(ctx, sshAddressPath, configDir)
	if err != nil {
		return fmt.Errorf("%w\ndaemon:\n%s", err, daemon.String())
	}
	if level != "alert" {
		return fmt.Errorf("show config running answered ddos flowspec response-level %q after the refused reload to enforce, want alert", level)
	}

	before = len(daemon.String())
	if err := os.WriteFile(strippedFlowReloadConfig, alert, 0o600); err != nil {
		return err
	}
	if err := syscall.Kill(command.Process.Pid, syscall.SIGHUP); err != nil {
		return err
	}
	reloaded := func() bool { return strings.Contains(daemon.String()[before:], strippedReloadComplete) }
	if !Poll(ctx, strippedOptionalProviderBootPolls, strippedOptionalProviderPollDelay, reloaded) {
		return fmt.Errorf("SIGHUP reload back to ddos flowspec response-level alert did not complete after the refusal:\n%s", daemon.String())
	}
	return nil
}

// strippedFlowUser is the login the reload step reads the running config as.
const strippedFlowUser = "ci"

// strippedFlowLoginBlock answers the `system authentication` block that lets
// the fixture log in over SSH as strippedFlowUser with valueSecret. The hash is
// made by the binary under test (`ze passwd`), so the block is one that build
// accepts.
func strippedFlowLoginBlock(ctx context.Context) ([]byte, error) {
	passwd := exec.CommandContext(ctx, strippedFlowBinary, "passwd") //nolint:gosec // the fixture chooses the program and its arguments
	passwd.Stdin = strings.NewReader(valueSecret + "\n")
	var hash bytes.Buffer
	passwd.Stdout = &hash
	passwd.Stderr = os.Stderr
	if err := passwd.Run(); err != nil {
		return nil, fmt.Errorf("%s passwd: %w", strippedFlowBinary, err)
	}
	block := "\nsystem {\n    authentication {\n        user " + strippedFlowUser + " {\n" +
		"            password \"" + strings.TrimSpace(hash.String()) + "\"\n" +
		"            profile [ admin ]\n        }\n    }\n" +
		"    authorization {\n        profile admin { run { default-action allow; } edit { default-action allow; } }\n    }\n}\n"
	return []byte(block), nil
}

// strippedFlowRunningLevel asks the running daemon, over the SSH CLI, for the
// configuration it applied and answers its ddos flowspec response-level. The
// daemon writes its SSH address to sshAddressPath once it listens.
func strippedFlowRunningLevel(ctx context.Context, sshAddressPath, configDir string) (string, error) {
	// The address file is absent until the daemon listens, so only a missing
	// file keeps polling. Any other stat failure ends the wait and is answered.
	var statErr error
	listening := func() bool {
		_, statErr = os.Stat(sshAddressPath)
		return !errors.Is(statErr, os.ErrNotExist)
	}
	if !Poll(ctx, strippedOptionalProviderBootPolls, strippedOptionalProviderPollDelay, listening) {
		return "", fmt.Errorf("%s never wrote its SSH address to %s", strippedFlowBinary, sshAddressPath)
	}
	if statErr != nil {
		return "", fmt.Errorf("SSH address file %s: %w", sshAddressPath, statErr)
	}
	address, err := os.ReadFile(sshAddressPath) //nolint:gosec // the path is the fixture's own scratch file
	if err != nil {
		return "", err
	}
	host, port, err := net.SplitHostPort(strings.TrimSpace(string(address)))
	if err != nil {
		return "", fmt.Errorf("SSH address %q: %w", address, err)
	}
	cli := exec.CommandContext(ctx, strippedFlowBinary, "cli", "-c", "show config running | json") //nolint:gosec // the fixture chooses the program and its arguments
	cli.Env = miscEnvironment(map[string]string{
		envSSHHost: host, envSSHPort: port, envSSHUsername: strippedFlowUser,
		envSSHPassword: valueSecret, envConfigDir: configDir,
	})
	// The answer is stdout alone: the client logs (slog's default handler) go
	// to stderr, so merging the streams would put log lines before the JSON.
	var diagnostics bytes.Buffer
	cli.Stderr = &diagnostics
	output, err := cli.Output()
	if err != nil {
		return "", fmt.Errorf("%s cli -c \"show config running | json\": %w: %s%s", strippedFlowBinary, err, output, diagnostics.Bytes())
	}
	var running struct {
		DDoS struct {
			FlowSpec struct {
				Level string `json:"response-level"`
			} `json:"flowspec"`
		} `json:"ddos"`
	}
	if err := json.Unmarshal(output, &running); err != nil {
		return "", fmt.Errorf("show config running answered no JSON: %w: %s", err, output)
	}
	if running.DDoS.FlowSpec.Level == "" {
		return "", fmt.Errorf("show config running answered no ddos flowspec response-level: %s", output)
	}
	return running.DDoS.FlowSpec.Level, nil
}
