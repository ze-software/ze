// Design: docs/guide/dataplane-features.md -- command-line plugin doors and static named-table data-plane selection.
// Related: plugin_fixture_01_api.go -- REST client helpers shared with api-config-enum-refusal.
// Related: plugin_fixture_config_refusal.go -- gNMI TLS client helpers shared with gnmi-set-enum-refusal.
// Related: plugin_fixture_session_editor.go -- PTY session editor driver shared with SSH editor tests.
// Related: misc_fixture_managed.go -- fake hub and managed store helpers shared with config-push-transactional.
// Related: static_named_table_fixture.go -- kernel route reader shared with static-named-table-unchanged.

package fixture

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/creack/pty"
	gpb "github.com/openconfig/gnmi/proto/gnmi"
	gossh "golang.org/x/crypto/ssh"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/core/configupload"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

const (
	dpDoorCLIPluginArg       = "ze.flowspec-firewall"
	dpDoorPluginFlagText     = "--plugin ze.flowspec-firewall"
	dpDoorFixFirewallVPP     = "set firewall backend vpp"
	dpDoorBackendKernelText  = "backend kernel"
	dpDoorBackendVPPText     = "backend vpp"
	dpDoorInterfaceBackend   = "interface.backend"
	dpDoorCandidateVPP       = "interface {\n\tbackend vpp;\n}\n"
	dpDoorStaticVPPCode      = "doctor-static-vpp-channel"
	dpDoorAdminPasswordHash  = "$2a$04$UlwuiuH82Unfsq.XEMPGJeDkXwbm3KW.nvVaVXOd/JeFK8VjMjrQO" // #nosec G101 -- bcrypt hash of testpass matching session-editor fixtures
	dpDoorSSHPassword        = "testpass"
	dpDoorCommitBlocked      = "commit blocked:"
	dpDoorCommitReloadErr    = "commit failed: reload:"
	dpDoorEnvManagedTLSInsec = "ZE_MANAGED_TLS_INSECURE"
)

// dpDoorAPIDriver proves a REST config session commit on a daemon started with
// `--plugin ze.flowspec-firewall` is refused at validation (`commit validation:`),
// not at runtime reload, names the command-line plugin and fix, and leaves the
// running configuration unchanged.
func dpDoorAPIDriver(ctx context.Context, args []string) error {
	if len(args) != 1 {
		return errors.New("plugin/dataplane-door-api needs a REST endpoint URL")
	}
	return Observe(ctx, "dataplane-door-api", &sdk.Registration{}, func(ctx context.Context, plugin *sdk.Plugin) error {
		return runDPDoorAPI(ctx, plugin, args[0])
	})
}

func runDPDoorAPI(ctx context.Context, plugin *sdk.Plugin, endpoint string) error {
	if !Poll(ctx, 40, 250*time.Millisecond, func() bool {
		return plugin01RESTReadyAt(ctx, endpoint)
	}) {
		return errors.New("REST API did not respond")
	}
	beforeConfig, err := plugin01RequireDone(ctx, plugin, "show config running")
	if err != nil {
		return err
	}

	created, err := plugin01RESTAt(ctx, endpoint, http.MethodPost, "/config/sessions", nil)
	if err != nil {
		return err
	}
	sessionID, _ := created["session-id"].(string)
	if sessionID == "" {
		return fmt.Errorf("create session response=%v", created)
	}
	session := "/config/sessions/" + sessionID

	if _, err := plugin01RESTAt(ctx, endpoint, http.MethodPut, session,
		map[string]string{fieldPath: dpDoorInterfaceBackend, fieldValue: actionVPPTest}); err != nil {
		return err
	}
	refusal, err := plugin01RESTRefusalAt(ctx, endpoint, http.MethodPost, session+"/commit", nil)
	if err != nil {
		return err
	}
	if err := plugin01RefusalHolds("REST dataplane commit", refusal,
		[]string{"commit validation:", dpDoorPluginFlagText, dpDoorFixFirewallVPP},
		[]string{"commit runtime reload failed:"}); err != nil {
		return err
	}
	afterConfig, err := plugin01RequireDone(ctx, plugin, "show config running")
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(beforeConfig, afterConfig) {
		return errors.New("refused REST commit changed the running configuration")
	}
	fmt.Fprintln(os.Stderr, "OK: REST commit validation refused --plugin ze.flowspec-firewall and kept running config unchanged") //nolint:errcheck // fixture progress line
	return nil
}

// dpDoorGNMIDriver proves a gNMI Set on a daemon started with
// `--plugin ze.flowspec-firewall` is refused at validation
// (`commit: commit validation:`), not at runtime reload, names the command-line
// plugin and fix, and leaves the running configuration unchanged.
func dpDoorGNMIDriver(ctx context.Context, args []string) error {
	if len(args) != 2 {
		return errors.New("plugin/dataplane-door-gnmi needs listener address and certificate path")
	}
	return Observe(ctx, "dataplane-door-gnmi", &sdk.Registration{}, func(ctx context.Context, plugin *sdk.Plugin) error {
		return runDPDoorGNMI(ctx, plugin, args[0], args[1])
	})
}

func runDPDoorGNMI(ctx context.Context, plugin *sdk.Plugin, address, certificate string) error {
	conn, err := configRefusalClient(address, certificate)
	if err != nil {
		return err
	}
	defer conn.Close() //nolint:errcheck // fixture client cleanup
	callCtx, cancel := context.WithTimeout(ctx, WaitBudget(70, 20*time.Second))
	defer cancel()
	callCtx = metadata.AppendToOutgoingContext(callCtx, "authorization", "Bearer secret")
	client := gpb.NewGNMIClient(conn)

	var lastCapErr error
	if !Poll(callCtx, WaitAttempts(40, plugin01PollDelay, 40), plugin01PollDelay, func() bool {
		_, lastCapErr = client.Capabilities(callCtx, &gpb.CapabilityRequest{})
		// refuse-by-default: guards runDPDoorGNMI -- Poll retries while the listener binds; lastCapErr is returned if the deadline expires
		if lastCapErr != nil {
			return false
		}
		return true
	}) {
		return fmt.Errorf("TLS gNMI listener did not become ready: %w", lastCapErr)
	}
	beforeConfig, err := plugin01RequireDone(callCtx, plugin, "show config running")
	if err != nil {
		return err
	}

	_, setErr := client.Set(callCtx, &gpb.SetRequest{Update: []*gpb.Update{{
		Path: &gpb.Path{Elem: []*gpb.PathElem{{Name: "interface"}, {Name: "backend"}}},
		Val:  &gpb.TypedValue{Value: &gpb.TypedValue_StringVal{StringVal: "vpp"}},
	}}})
	if setErr == nil {
		return errors.New("gNMI Set was accepted, a validation refusal was expected")
	}
	if status.Code(setErr) != codes.Aborted {
		return fmt.Errorf("gNMI Set code=%s, want Aborted: %w", status.Code(setErr), setErr)
	}
	msg := status.Convert(setErr).Message()
	if err := plugin01RefusalHolds("gNMI dataplane Set", msg,
		[]string{"commit: commit validation:", dpDoorPluginFlagText, dpDoorFixFirewallVPP},
		[]string{"commit: commit runtime reload failed:"}); err != nil {
		return err
	}

	afterConfig, err := plugin01RequireDone(callCtx, plugin, "show config running")
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(beforeConfig, afterConfig) {
		return errors.New("refused gNMI Set changed the running configuration")
	}
	fmt.Fprintln(os.Stderr, "OK: gNMI Set validation refused --plugin ze.flowspec-firewall and kept running config unchanged") //nolint:errcheck // fixture progress line
	return nil
}

// dpDoorUploadDriver proves an SSH config upload on a daemon started with
// `--plugin ze.flowspec-firewall` is refused at validation before publication,
// names the `--plugin` argument in its local validation fix line, and leaves
// the running configuration unchanged.
func dpDoorUploadDriver(ctx context.Context, args []string) error {
	if len(args) != 1 {
		return errors.New("plugin/dataplane-door-upload needs SSH host:port")
	}
	return Observe(ctx, "dataplane-door-upload", &sdk.Registration{}, func(ctx context.Context, plugin *sdk.Plugin) error {
		return runDPDoorUpload(ctx, plugin, args[0])
	})
}

func runDPDoorUpload(ctx context.Context, plugin *sdk.Plugin, address string) error {
	beforeConfig, err := plugin01RequireDone(ctx, plugin, "show config running")
	if err != nil {
		return err
	}

	sshCfg := &gossh.ClientConfig{
		User:            valueTestUser,
		Auth:            []gossh.AuthMethod{gossh.Password(dpDoorSSHPassword)},
		HostKeyCallback: gossh.InsecureIgnoreHostKey(), //nolint:gosec // local test daemon with ephemeral key
		Timeout:         5 * time.Second,
	}

	var client *gossh.Client
	if !Poll(ctx, 50, 200*time.Millisecond, func() bool {
		c, dialErr := gossh.Dial("tcp", address, sshCfg)
		if dialErr != nil {
			return false
		}
		client = c
		return true
	}) {
		return fmt.Errorf("SSH server at %s did not become reachable", address)
	}
	defer client.Close() //nolint:errcheck // fixture SSH client cleanup

	exitCode, output, err := sendSSHConfigUpload(client, dpDoorCandidateVPP)
	if err != nil {
		return err
	}
	if exitCode == 0 {
		return fmt.Errorf("SSH config upload exited 0, expected validation refusal; output=%q", output)
	}
	wantFix := "ze " + dpDoorPluginFlagText + " config validate <file>"
	if err := plugin01RefusalHolds("SSH upload", output,
		[]string{"configuration upload failed validation", "refused before publication", wantFix},
		[]string{"configuration upload was not accepted"}); err != nil {
		return err
	}

	afterConfig, err := plugin01RequireDone(ctx, plugin, "show config running")
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(beforeConfig, afterConfig) {
		return errors.New("refused SSH upload changed the running configuration")
	}
	fmt.Fprintln(os.Stderr, "OK: SSH upload validation refused --plugin ze.flowspec-firewall before publication and kept running config unchanged") //nolint:errcheck // fixture progress line
	return nil
}

func readSSHExitStatus(requests <-chan *gossh.Request, statusCh chan<- uint32) {
	for req := range requests {
		if req.Type == "exit-status" && len(req.Payload) >= 4 {
			statusCh <- binary.BigEndian.Uint32(req.Payload[:4])
		}
		if req.WantReply {
			_ = req.Reply(false, nil) //nolint:errcheck // best-effort SSH request reply
		}
	}
	close(statusCh)
}

func copyStreamToBuffer(dst *bytes.Buffer, src io.Reader, done chan<- struct{}) {
	_, _ = io.Copy(dst, src) //nolint:errcheck // fixture stderr capture
	close(done)
}

func waitProcessExit(cmd *exec.Cmd, done chan<- error) {
	done <- cmd.Wait()
}

func sendSSHConfigUpload(client *gossh.Client, payload string) (uint32, string, error) {
	ch, requests, err := client.OpenChannel("session", nil)
	if err != nil {
		return 0, "", fmt.Errorf("open SSH session channel: %w", err)
	}
	defer ch.Close() //nolint:errcheck // fixture channel cleanup

	statusCh := make(chan uint32, 1)
	go readSSHExitStatus(requests, statusCh)

	subsystemPayload := gossh.Marshal(struct{ Name string }{Name: configupload.Subsystem})
	ok, err := ch.SendRequest("subsystem", true, subsystemPayload)
	if err != nil {
		return 0, "", fmt.Errorf("request subsystem %q: %w", configupload.Subsystem, err)
	}
	if !ok {
		return 0, "", fmt.Errorf("SSH server rejected subsystem %q", configupload.Subsystem)
	}

	if err := configupload.Write(ch, payload); err != nil {
		return 0, "", fmt.Errorf("write upload payload: %w", err)
	}
	if err := ch.CloseWrite(); err != nil {
		return 0, "", fmt.Errorf("close write half of upload channel: %w", err)
	}

	var out bytes.Buffer
	var errBuf bytes.Buffer
	errDone := make(chan struct{})
	go copyStreamToBuffer(&errBuf, ch.Stderr(), errDone)
	_, _ = io.Copy(&out, ch) //nolint:errcheck // fixture stdout capture
	<-errDone

	exitCode, received := <-statusCh
	if !received {
		return 0, "", errors.New("SSH upload channel ended without exit-status")
	}
	combined := out.String() + "\n" + errBuf.String()
	return exitCode, combined, nil
}

// dpDoorEphemeralEditorDriver proves `ze --plugin ze.flowspec-firewall config
// edit` with no daemon running forwards `--plugin` to its ephemeral daemon so
// committing `interface backend vpp` is refused at validation with `commit
// blocked:`, leaving the stored configuration unchanged, while the same session
// without `--plugin` commits cleanly.
func dpDoorEphemeralEditorDriver(ctx context.Context, args []string) error {
	if len(args) != 1 {
		return errors.New("plugin/dataplane-door-ephemeral-editor needs an SSH port")
	}
	sshPort := args[0]

	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	workDir, err := os.MkdirTemp(cwd, "ephemeral-door-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(workDir) //nolint:errcheck // fixture cleanup

	env := overrideEnv04(os.Environ(),
		"ZE_CONFIG_DIR="+workDir,
		"ZE_CONFIG_STORAGE=disk",
		"ZE_SSH_HOST=127.0.0.1",
		"ZE_SSH_PORT="+sshPort,
		"ZE_SSH_USERNAME="+valueTestUser,
		"ZE_SSH_PASSWORD="+dpDoorSSHPassword,
		"NO_COLOR=1",
		"TERM=xterm",
	)

	initInput := valueTestUser + "\n" + dpDoorSSHPassword + "\n127.0.0.1\n" + sshPort + "\n\n"
	if _, err := runCommandProcess04(ctx, env, strings.NewReader(initInput), "init"); err != nil {
		return fmt.Errorf("ze init: %w", err)
	}

	var seedBuf textbuf.Buffer
	seedBuf.Str("system {\n\tauthentication {\n\t\tuser ").Str(valueTestUser).Str(" {\n\t\t\tpassword \"").
		Str(dpDoorAdminPasswordHash).Str("\";\n\t\t}\n\t}\n}\n").
		Str("environment {\n\tssh {\n\t\tenabled true;\n\t\tserver main {\n\t\t\tip 127.0.0.1;\n\t\t\tport ").
		Str(sshPort).Str(";\n\t\t}\n\t}\n}\n").
		Str("interface {\n\t").Str(dpDoorBackendKernelText).Str(";\n}\n")

	confName := "ephemeral-door.conf"
	seedPath := filepath.Join(workDir, confName)
	if err := os.WriteFile(seedPath, []byte(seedBuf.String()), 0o600); err != nil {
		return err
	}
	dbPath := filepath.Join(workDir, "database")
	if _, err := runCommandProcess04(ctx, env, nil, "data", "--path", dbPath, "import", seedPath); err != nil {
		return fmt.Errorf("import seed config: %w", err)
	}

	// Session 1: with --plugin ze.flowspec-firewall, committing interface
	// backend vpp must be blocked at validation and leave stored config intact.
	refusedSteps := []sessionEditorStep{
		{verb: sessionEditorSend, text: "set interface backend vpp"},
		{verb: sessionEditorSend, text: "commit now"},
		{verb: sessionEditorWait, text: dpDoorCommitBlocked},
		{verb: sessionEditorSend, text: "show | errors"},
		{verb: sessionEditorWait, text: dpDoorPluginFlagText},
		{verb: sessionEditorWait, text: dpDoorFixFirewallVPP},
		{verb: sessionEditorSend, text: "discard all"},
		{verb: sessionEditorWait, text: "discarded"},
	}
	transcript1, err := runEphemeralEditorPTY(ctx, env, workDir, []string{"--plugin", dpDoorCLIPluginArg, commandOutcomeConfigDirectory, "edit", confName}, refusedSteps)
	if err != nil {
		return fmt.Errorf("ephemeral session with --plugin: %w", err)
	}
	if strings.Contains(transcript1, dpDoorCommitReloadErr) {
		return fmt.Errorf("ephemeral session with --plugin failed at reload instead of validation:\n%s", transcript1)
	}

	storedAfterRefusal, err := runCommandProcess04(ctx, env, nil, "data", "--path", dbPath, "cat", "file/active/"+confName)
	if err != nil {
		return fmt.Errorf("read stored active config after refusal: %w", err)
	}
	if !strings.Contains(storedAfterRefusal, dpDoorBackendKernelText) {
		return fmt.Errorf("stored config lost %s after refused ephemeral commit:\n%s", dpDoorBackendKernelText, storedAfterRefusal)
	}
	if strings.Contains(storedAfterRefusal, dpDoorBackendVPPText) {
		return fmt.Errorf("stored config unexpectedly contains %s after refused ephemeral commit:\n%s", dpDoorBackendVPPText, storedAfterRefusal)
	}
	fmt.Fprintln(os.Stderr, "OK: ephemeral config edit with --plugin ze.flowspec-firewall refused commit and kept stored config unchanged") //nolint:errcheck // fixture progress line

	// Session 2: without --plugin, the exact same edit commits cleanly.
	acceptedSteps := []sessionEditorStep{
		{verb: sessionEditorSend, text: "set interface backend vpp"},
		{verb: sessionEditorSend, text: "commit now"},
		{verb: sessionEditorWait, text: "Session committed:"},
	}
	if _, err := runEphemeralEditorPTY(ctx, env, workDir, []string{commandOutcomeConfigDirectory, "edit", confName}, acceptedSteps); err != nil {
		return fmt.Errorf("ephemeral session without --plugin: %w", err)
	}

	storedAfterCommit, err := runCommandProcess04(ctx, env, nil, "data", "--path", dbPath, "cat", "file/active/"+confName)
	if err != nil {
		return fmt.Errorf("read stored active config after commit: %w", err)
	}
	if !strings.Contains(storedAfterCommit, dpDoorBackendVPPText) {
		return fmt.Errorf("stored config does not contain %s after accepted ephemeral commit:\n%s", dpDoorBackendVPPText, storedAfterCommit)
	}
	fmt.Fprintln(os.Stderr, "OK: ephemeral config edit without --plugin committed interface backend vpp") //nolint:errcheck // fixture progress line
	return nil
}

func runEphemeralEditorPTY(ctx context.Context, env []string, dir string, zeArgs []string, steps []sessionEditorStep) (string, error) {
	stepCtx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()
	cmd := exec.CommandContext(stepCtx, "ze", zeArgs...) //nolint:gosec // fixture runs ze binary under test
	cmd.Env = env
	cmd.Dir = dir
	terminal, err := pty.StartWithSize(cmd, &pty.Winsize{Rows: 30, Cols: 160})
	if err != nil {
		return "", err
	}
	defer terminal.Close() //nolint:errcheck // fixture PTY cleanup
	defer func() {
		if cmd.ProcessState == nil {
			_, _ = killSessionEditor(cmd, terminal) //nolint:errcheck // teardown after failed step
		}
	}()

	var transcript bytes.Buffer
	pending, err := sessionEditorWaitFor(stepCtx, terminal, nil, &transcript, "\x1b[?1049h")
	if err != nil {
		return transcript.String(), fmt.Errorf("ephemeral editor did not draw its first frame: %w\n%s", err, transcript.String())
	}
	for i, step := range steps {
		switch step.verb {
		case sessionEditorSend:
			_, err = terminal.WriteString(step.text + "\r")
		case sessionEditorWait:
			pending, err = sessionEditorWaitFor(stepCtx, terminal, pending, &transcript, step.text)
		case sessionEditorUnspecified, sessionEditorCLI, sessionEditorHas, sessionEditorLacks, sessionEditorKey, sessionEditorKill, sessionEditorStop, sessionEditorInput:
			return transcript.String(), fmt.Errorf("step %d uses unsupported ephemeral step verb %v", i+1, step.verb)
		}
		if err != nil {
			return transcript.String(), fmt.Errorf("step %d (%q): %w\n%s", i+1, step.text, err, transcript.String())
		}
	}
	if _, err := terminal.WriteString("quit\r"); err != nil {
		return transcript.String(), fmt.Errorf("quit: %w\n%s", err, transcript.String())
	}
	tail, err := readPTYUntil04(terminal, nil, true)
	transcript.WriteString(tail)
	if err != nil {
		return transcript.String(), fmt.Errorf("ephemeral editor did not exit after quit: %w\n%s", err, transcript.String())
	}
	if err := cmd.Wait(); err != nil {
		return transcript.String(), fmt.Errorf("ephemeral editor exited: %w\n%s", err, transcript.String())
	}
	return transcript.String(), nil
}

// dpDoorManagedPushedDriver proves both halves of AC-18 on the managed door:
//  1. Pre-boot (`judgeConfigBeforeBoot`): a managed client started with
//     `--plugin ze.flowspec-firewall` refuses a hub-fetched candidate setting
//     `interface backend vpp` before caching or booting it.
//  2. Live push (`managed.Handler.Validate`): a running managed client started
//     with `--plugin ze.flowspec-firewall` refuses the same pushed candidate
//     with `ConfigAck.Error` starting with `validate config:`, keeping the
//     running configuration on `backend kernel` with no lingering candidate.
func dpDoorManagedPushedDriver(ctx context.Context, args []string) error {
	flags := flag.NewFlagSet("managed/dataplane-door-pushed-config", flag.ContinueOnError)
	hubPort := flags.Int("hub-port", 0, "fake hub listen port")
	pluginPort := flags.Int("plugin-port", 0, "plugin hub listen port")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *hubPort == 0 {
		return errors.New("hub-port is required")
	}
	if *pluginPort == 0 {
		return errors.New("plugin-port is required")
	}
	if err := runDPDoorManagedPreBoot(ctx, *hubPort, *pluginPort); err != nil {
		return fmt.Errorf("pre-boot pushed candidate: %w", err)
	}
	if err := runDPDoorManagedLivePush(ctx, *hubPort, *pluginPort); err != nil {
		return fmt.Errorf("live pushed candidate: %w", err)
	}
	fmt.Fprintln(os.Stderr, "OK: managed pre-boot and live pushed config doors both refused --plugin ze.flowspec-firewall") //nolint:errcheck // fixture progress line
	return nil
}

func dpDoorManagedConfig(backend string, hubPort, pluginPort int) string {
	var tb textbuf.Buffer
	tb.Str("bus {\n\thub {\n\t\tclient ").Str(managedClientName).Str(" { host 127.0.0.1; port ").
		Int(int64(hubPort)).Str("; secret \"").Str(managedToken).Str("\"; }\n\t\tserver local { ip 127.0.0.1; port ").
		Int(int64(pluginPort)).Str("; secret \"").Str(managedServerSecret).Str("\"; }\n\t}\n}\n").
		Str("interface {\n\tbackend ").Str(backend).Str(";\n}\n")
	return tb.String()
}

func runDPDoorManagedPreBoot(ctx context.Context, hubPort, pluginPort int) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	cfgDir, err := os.MkdirTemp(cwd, "managed-dp-preboot-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(cfgDir) //nolint:errcheck // fixture cleanup

	dbPath := filepath.Join(cfgDir, "database")
	env := childEnvironment(daemonEnvironment(os.Environ(), cfgDir, daemonStoreDisk), map[string]string{
		dpDoorEnvManagedTLSInsec: valueTrue,
		"ZE_MANAGED_SERVER":      "127.0.0.1:" + strconv.Itoa(hubPort),
		"ZE_MANAGED_NAME":        managedClientName,
		"ZE_MANAGED_TOKEN":       managedToken,
		envNoColor:               "1",
		envTestBGPPort:           strconv.Itoa(pluginPort + 1000),
	})
	if _, err := managedRunCommand(ctx, env, cfgDir, "admin\nsecret123\n127.0.0.1\n2222\n"+managedClientName+"\n", "init", "--managed"); err != nil {
		return err
	}

	pushed := dpDoorManagedConfig(actionVPPTest, hubPort, pluginPort)
	_, _, closeHub, err := startManagedFakeHub(hubPort, managedToken, []byte(pushed))
	if err != nil {
		return err
	}
	defer closeHub()

	var zeOutput bytes.Buffer
	zeCmd := exec.CommandContext(ctx, "ze", "--plugin", dpDoorCLIPluginArg, "start")
	zeCmd.Dir = cfgDir
	zeCmd.Env = env
	zeCmd.Stdout = &zeOutput
	zeCmd.Stderr = &zeOutput
	runErr := zeCmd.Run()
	if runErr == nil {
		return fmt.Errorf("expected pre-boot refusal, but ze exited 0:\n%s", zeOutput.String())
	}
	output := zeOutput.String()
	if err := plugin01RefusalHolds("managed pre-boot", output,
		[]string{"hub config failed validation", dpDoorPluginFlagText, dpDoorFixFirewallVPP},
		[]string{"interface backend loaded"}); err != nil {
		return err
	}

	listing, err := managedRunCommand(ctx, env, cfgDir, "", "data", "--path", dbPath, "list")
	if err != nil {
		return err
	}
	if strings.Contains(string(listing), "file/active/"+managedClientName+".conf") {
		return fmt.Errorf("refused pre-boot hub config was cached in store:\n%s", string(listing))
	}
	fmt.Fprintln(os.Stderr, "OK: managed pre-boot judgeConfigBeforeBoot refused --plugin ze.flowspec-firewall before caching") //nolint:errcheck // fixture progress line
	return nil
}

func runDPDoorManagedLivePush(ctx context.Context, hubPort, pluginPort int) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	cfgDir, err := os.MkdirTemp(cwd, "managed-dp-live-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(cfgDir) //nolint:errcheck // fixture cleanup

	dbPath := filepath.Join(cfgDir, "database")
	env := childEnvironment(daemonEnvironment(os.Environ(), cfgDir, daemonStoreDisk), map[string]string{
		dpDoorEnvManagedTLSInsec: valueTrue,
		envNoColor:               "1",
		envTestBGPPort:           strconv.Itoa(pluginPort + 1000),
	})
	if _, err := managedRunCommand(ctx, env, cfgDir, "admin\nsecret123\n127.0.0.1\n2222\n"+managedClientName+"\n", "init", "--managed"); err != nil {
		return err
	}

	active := dpDoorManagedConfig("kernel", hubPort, pluginPort)
	pushed := dpDoorManagedConfig(actionVPPTest, hubPort, pluginPort)
	activePath := filepath.Join(cfgDir, managedClientName+".conf")
	if err := os.WriteFile(activePath, []byte(active), 0o600); err != nil {
		return err
	}
	if _, err := managedRunCommand(ctx, env, cfgDir, "", "data", "--path", dbPath, "import", activePath); err != nil {
		return err
	}

	ackCh, hubErrCh, closeHub, err := startManagedFakeHub(hubPort, managedToken, []byte(pushed))
	if err != nil {
		return err
	}
	defer closeHub()

	var zeOutput bytes.Buffer
	zeCmd := exec.CommandContext(ctx, "ze", "--plugin", dpDoorCLIPluginArg, "start")
	zeCmd.Dir = cfgDir
	zeCmd.Env = env
	zeCmd.Stdout = &zeOutput
	zeCmd.Stderr = &zeOutput
	zeCmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := zeCmd.Start(); err != nil {
		return fmt.Errorf("start ze: %w", err)
	}
	zeDone := make(chan error, 1)
	go waitProcessExit(zeCmd, zeDone)

	var ack managedConfigAck
	select {
	case ack = <-ackCh:
	case hubErr := <-hubErrCh:
		stopManagedProcess(zeCmd, zeDone)
		return fmt.Errorf("fake hub: %w\nze output:\n%s", hubErr, zeOutput.String())
	case exitErr := <-zeDone:
		return fmt.Errorf("ze exited before config ack: %w\n%s", exitErr, zeOutput.String())
	case <-time.After(35 * time.Second):
		stopManagedProcess(zeCmd, zeDone)
		return fmt.Errorf("timed out waiting for config ack\nze output:\n%s", zeOutput.String())
	case <-ctx.Done():
		stopManagedProcess(zeCmd, zeDone)
		return ctx.Err()
	}
	stopManagedProcess(zeCmd, zeDone)

	expectedVersion := managedVersionHash([]byte(pushed))
	if ack.Version != expectedVersion {
		return fmt.Errorf("ack version %q, want %q", ack.Version, expectedVersion)
	}
	if ack.OK {
		return errors.New("live pushed mismatch ACK was OK, expected validation refusal")
	}
	if err := plugin01RefusalHolds("managed live push ACK", ack.Error,
		[]string{"validate config:", dpDoorPluginFlagText, dpDoorFixFirewallVPP},
		[]string{"reload:"}); err != nil {
		return err
	}

	activeData, err := managedRunCommand(ctx, env, cfgDir, "", "data", "--path", dbPath, "cat", "file/active/"+managedClientName+".conf")
	if err != nil {
		return err
	}
	activeText := string(activeData)
	if !strings.Contains(activeText, dpDoorBackendKernelText) {
		return fmt.Errorf("active config lost %s after refused live push:\n%s", dpDoorBackendKernelText, activeText)
	}
	if strings.Contains(activeText, dpDoorBackendVPPText) {
		return fmt.Errorf("active config contains %s after refused live push:\n%s", dpDoorBackendVPPText, activeText)
	}

	store, err := openDaemonStoreReadOnly(env, cfgDir)
	if err != nil {
		return err
	}
	_, _, candidate, readErr := storage.ReadCandidateConfig(store, managedClientName+".conf")
	closeErr := store.Close()
	if err := errors.Join(readErr, closeErr); err != nil {
		return fmt.Errorf("inspect candidate after live push: %w", err)
	}
	if candidate {
		return errors.New("candidate pointer remains after refused live push")
	}
	fmt.Fprintln(os.Stderr, "OK: managed live push handler refused --plugin ze.flowspec-firewall and kept running config unchanged") //nolint:errcheck // fixture progress line
	return nil
}

// dpDoorStaticDataPlaneDriver proves AC-7: under `interface { backend vpp; }`
// with no VPP running, the static plugin keeps programming main-table routes
// into the Loc-RIB while refusing to fall back to the Linux kernel for
// named-table routes, raising doctor-static-vpp-channel instead.
func dpDoorStaticDataPlaneDriver(ctx context.Context, _ []string) error {
	return Observe(ctx, "static-dataplane-observer", &sdk.Registration{}, runDPDoorStaticDataPlane)
}

func runDPDoorStaticDataPlane(ctx context.Context, plugin *sdk.Plugin) error {
	var winner, ribRaw string
	if !Poll(ctx, namedTablePolls, 100*time.Millisecond, func() bool {
		winner, ribRaw = ribWinner02(ctx, plugin, mainTablePrefix)
		return winner == protocolStatic
	}) {
		return fmt.Errorf("main-table static %s never reached the system RIB (winner %q); show rib: %s",
			mainTablePrefix, winner, ribRaw)
	}

	var doctorRaw map[string]any
	if !Poll(ctx, namedTablePolls, 100*time.Millisecond, func() bool {
		data, statusStr, err := plugin01DispatchMap(ctx, plugin, "show doctor")
		if err != nil {
			return false
		}
		if statusStr != rpc.StatusDone {
			return false
		}
		doctorRaw = data
		for _, item := range plugin01Array(data["diagnostics"]) {
			if plugin01Object(item)["code"] == dpDoorStaticVPPCode {
				return true
			}
		}
		return false
	}) {
		return fmt.Errorf("show doctor never reported %s: %v", dpDoorStaticVPPCode, doctorRaw)
	}

	var staticRaw map[string]any
	if !Poll(ctx, namedTablePolls, 100*time.Millisecond, func() bool {
		data, statusStr, err := plugin01DispatchMap(ctx, plugin, "show static")
		if err != nil {
			return false
		}
		if statusStr != rpc.StatusDone {
			return false
		}
		staticRaw = data
		encoded := fmt.Sprint(data)
		return strings.Contains(encoded, dpDoorStaticVPPCode) && strings.Contains(encoded, namedTablePrefix)
	}) {
		return fmt.Errorf("show static never reported %s for %s: %v", dpDoorStaticVPPCode, namedTablePrefix, staticRaw)
	}

	if leaked := kernelRoute(ctx, namedTableID, namedTablePrefix); leaked != "" {
		return fmt.Errorf("named-table static %s leaked into kernel table %s under interface backend vpp: %q",
			namedTablePrefix, namedTableID, leaked)
	}
	fmt.Fprintln(os.Stderr, "OK: named-table static route followed interface backend vpp, raised "+dpDoorStaticVPPCode+", and never fell back to kernel table "+namedTableID) //nolint:errcheck // fixture progress line
	return nil
}
