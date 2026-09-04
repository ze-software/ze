// Design: docs/architecture/fleet-config.md -- how a managed client authenticates its hub

package fixture

import (
	"bytes"
	"context"
	"encoding/pem"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// The two daemons this scenario runs, and the strings they agree on. The hub
// serves the client's config; the client validates the hub's chain against the
// root it was given.
const (
	caTrustHubName        = "fleet-hub"
	caTrustClientName     = "edge-01"
	caTrustClientSecret   = "edge-ca-trust-secret-that-is-at-least-32"
	caTrustHubSecret      = "hub-ca-trust-secret-that-is-at-least-32c"
	caTrustPluginSecret   = "hub-plugin-secret-that-is-at-least-32ch"
	caTrustAnchorName     = "fleet-hub-root"
	caTrustBootRouterID   = "1.1.1.1"
	caTrustServedRouterID = "2.2.2.2"
	caTrustExportFile     = "exported-ca-root.pem"
	caTrustScenarioAll    = "both"
	caTrustScenarioTrust  = "trusted"
	caTrustScenarioForeig = "foreign"
)

// managedHubCATrustDriver runs spec-local-ca AC-12 and user story 2: the
// operator exports the hub's certificate authority root, pastes it into a
// client's `pki ca` block, and the client fetches its config over a chain it
// validated against that root.
//
// Two REAL daemons, both `ze start`. A fake hub would prove the client's half
// and nothing about what the hub serves, and the leaf the hub presents here is
// one its own authority issued at startup.
//
// The `foreign` scenario is the control. Its client is given a root the hub did
// not use, everything else held equal, and it MUST NOT fetch. Without it a
// client that skipped verification entirely would pass the `trusted` half.
func managedHubCATrustDriver(ctx context.Context, args []string) error {
	flags := flag.NewFlagSet("managed/hub-ca-trust", flag.ContinueOnError)
	scenario := flags.String("scenario", caTrustScenarioAll, "trusted, foreign, or both")
	pluginPort := flags.Int("hub-plugin-port", 0, "hub plugin transport port")
	managedPort := flags.Int("hub-managed-port", 0, "hub managed-client listen port")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *pluginPort == 0 || *managedPort == 0 {
		return errors.New("hub-plugin-port and hub-managed-port are required")
	}

	scenarios := []string{*scenario}
	if *scenario == caTrustScenarioAll {
		scenarios = []string{caTrustScenarioTrust, caTrustScenarioForeig}
	}
	for _, name := range scenarios {
		if err := runManagedCATrustScenario(ctx, name, *pluginPort, *managedPort); err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
	}
	fmt.Fprintln(os.Stderr, "OK: the managed client fetched its config over a chain validated against the exported hub root, and refused a foreign root")
	return nil
}

// runManagedCATrustScenario starts one hub, exports its root, starts one client
// against that root or against a foreign one, and reads the verdict out of the
// client's own active config.
func runManagedCATrustScenario(ctx context.Context, scenario string, pluginPort, managedPort int) (retErr error) {
	if scenario != caTrustScenarioTrust && scenario != caTrustScenarioForeig {
		return fmt.Errorf("unknown scenario %q", scenario)
	}
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	base, err := os.MkdirTemp(cwd, "ca-trust-"+scenario+"-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(base) //nolint:errcheck // fixture cleanup

	hub, err := prepareCATrustHub(ctx, base, pluginPort, managedPort)
	if err != nil {
		return err
	}
	// The hub is stopped while its store is written, then started again. Two
	// processes writing one blob replace each other's state (pkg/zefs takes no
	// file lock), and the restart is worth having anyway: the leaf the client
	// validates below is issued AFTER the root was exported, which is the whole
	// point of trusting an issuer.
	if err := caTrustImportConfig(ctx, hub, "client-"+caTrustClientName+".conf",
		caTrustClientConfig(managedPort, caTrustServedRouterID, string(hub.rootPEM))); err != nil {
		return err
	}
	if err := caTrustStart(ctx, hub); err != nil {
		return err
	}
	defer func() {
		stopManagedProcess(hub.command, hub.done)
		if retErr != nil {
			retErr = fmt.Errorf("%w\nhub output:\n%s", retErr, hub.output.String())
		}
	}()

	anchor := hub.rootPEM
	if scenario == caTrustScenarioForeig {
		if anchor, err = caTrustForeignRootPEM(); err != nil {
			return err
		}
	}

	client, err := startCATrustClient(ctx, base, managedPort, anchor)
	if err != nil {
		return err
	}
	defer func() {
		stopManagedProcess(client.command, client.done)
		if retErr != nil {
			retErr = fmt.Errorf("%w\nclient output:\n%s", retErr, client.output.String())
		}
	}()

	if scenario == caTrustScenarioTrust {
		return caTrustRequireFetch(ctx, client)
	}
	return caTrustRequireRefusal(ctx, client)
}

// caTrustDaemon is one running `ze start` and everything needed to read what it
// did.
type caTrustDaemon struct {
	command *exec.Cmd
	done    chan error
	output  *bytes.Buffer
	dir     string
	dbPath  string
	env     []string
	rootPEM []byte
}

// prepareCATrustHub initializes the hub's store, runs the daemon once so its
// certificate authority generates a root, reads that root back through the
// export command, and stops the daemon again. The returned hub is NOT running.
func prepareCATrustHub(ctx context.Context, base string, pluginPort, managedPort int) (*caTrustDaemon, error) {
	dir := filepath.Join(base, "hub")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, err
	}
	hub := &caTrustDaemon{
		dir:    dir,
		dbPath: filepath.Join(dir, "database.zefs"),
		env: miscEnvironment(map[string]string{
			envConfigDir:   dir,
			envNoColor:     "1",
			envTestBGPPort: strconv.Itoa(managedPort + 1000),
		}),
	}

	initInput := "admin\nsecret123\n127.0.0.1\n2222\n" + caTrustHubName + "\n"
	if _, err := managedRunCommand(ctx, hub.env, dir, initInput, "init"); err != nil {
		return nil, err
	}

	exportPath := filepath.Join(dir, caTrustExportFile)
	if err := caTrustImportConfig(ctx, hub, caTrustHubName+".conf",
		caTrustHubConfig(pluginPort, managedPort, exportPath)); err != nil {
		return nil, err
	}

	if err := caTrustStart(ctx, hub); err != nil {
		return nil, err
	}
	found := waitForFile(ctx, exportPath, 300, 100*time.Millisecond)
	stopManagedProcess(hub.command, hub.done)
	if !found {
		return nil, fmt.Errorf("the hub never exported its certificate authority root\nhub output:\n%s", hub.output.String())
	}

	rootPEM, err := os.ReadFile(exportPath) //nolint:gosec // the path is the fixture's own scratch file
	if err != nil {
		return nil, err
	}
	if block, _ := pem.Decode(rootPEM); block == nil {
		return nil, fmt.Errorf("the exported root is not PEM: %q", rootPEM)
	}
	hub.rootPEM = rootPEM
	return hub, nil
}

// startCATrustClient initializes a managed client whose active config names the
// given anchor, then starts it.
func startCATrustClient(ctx context.Context, base string, managedPort int, anchorPEM []byte) (*caTrustDaemon, error) {
	dir := filepath.Join(base, "client")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, err
	}
	client := &caTrustDaemon{
		dir:    dir,
		dbPath: filepath.Join(dir, "database.zefs"),
		// ZE_MANAGED_TLS_INSECURE is deliberately ABSENT. Setting it would make
		// every scenario here pass against any certificate at all.
		env: miscEnvironment(map[string]string{
			envConfigDir:   dir,
			envNoColor:     "1",
			envTestBGPPort: strconv.Itoa(managedPort + 2000),
		}),
	}

	initInput := "admin\nsecret123\n127.0.0.1\n2222\n" + caTrustClientName + "\n"
	if _, err := managedRunCommand(ctx, client.env, dir, initInput, "init", "--managed"); err != nil {
		return nil, err
	}
	if err := caTrustImportConfig(ctx, client, caTrustClientName+".conf",
		caTrustClientConfig(managedPort, caTrustBootRouterID, string(anchorPEM))); err != nil {
		return nil, err
	}
	if err := caTrustStart(ctx, client); err != nil {
		return nil, err
	}
	return client, nil
}

// caTrustStart launches `ze start` in the daemon's own directory and collects
// its output, which is where a refused handshake is reported.
func caTrustStart(ctx context.Context, daemon *caTrustDaemon) error {
	daemon.output = &bytes.Buffer{}
	command := exec.CommandContext(ctx, "ze", "start") //nolint:gosec // the fixture chooses the program
	command.Dir = daemon.dir
	command.Env = daemon.env
	command.Stdout = daemon.output
	command.Stderr = daemon.output
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := command.Start(); err != nil {
		return fmt.Errorf("start ze in %s: %w", daemon.dir, err)
	}
	daemon.command = command
	daemon.done = make(chan error, 1)
	go caTrustReap(command, daemon.done)
	return nil
}

// caTrustReap owns the one Wait each launched daemon needs.
func caTrustReap(command *exec.Cmd, done chan<- error) { done <- command.Wait() }

// caTrustImportConfig writes one config file and imports it into the daemon's
// blob store, where `file/active/<name>` is what both daemons read.
func caTrustImportConfig(ctx context.Context, daemon *caTrustDaemon, name, body string) error {
	path := filepath.Join(daemon.dir, name)
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		return err
	}
	_, err := managedRunCommand(ctx, daemon.env, daemon.dir, "", "data", "--path", daemon.dbPath, "import", path)
	return err
}

// caTrustRequireFetch waits for the client's active config to become the one the
// hub serves. The active blob is the verdict: it changes only after the client
// completed the TLS handshake, authenticated, fetched, and committed.
func caTrustRequireFetch(ctx context.Context, client *caTrustDaemon) error {
	if caTrustPollActive(ctx, client, caTrustServedRouterID, 300) {
		return nil
	}
	active, _ := caTrustActiveConfig(ctx, client)
	return fmt.Errorf("the client never fetched the hub's config; its active config is still:\n%s", active)
}

// caTrustRequireRefusal holds the opposite: with a root the hub did not use, the
// client MUST NOT fetch, and it must say why.
//
// The wait is bounded rather than instant because "did not happen" needs time to
// be worth anything. 12 seconds covers several reconnect attempts of a client
// that retries.
func caTrustRequireRefusal(ctx context.Context, client *caTrustDaemon) error {
	if caTrustPollActive(ctx, client, caTrustServedRouterID, 120) {
		return errors.New("the client fetched config from a hub whose chain its configured root did not sign")
	}
	output := client.output.String()
	for _, phrase := range []string{"certificate signed by unknown authority", "tls"} {
		if strings.Contains(output, phrase) {
			return nil
		}
	}
	return errors.New("the client did not fetch, and reported no verification failure either, so the refusal is unexplained")
}

// caTrustPollActive reports whether the client's active config carries routerID
// within the attempt budget.
func caTrustPollActive(ctx context.Context, client *caTrustDaemon, routerID string, attempts int) bool {
	want := "router-id " + routerID
	return Poll(ctx, attempts, 100*time.Millisecond, func() bool {
		active, err := caTrustActiveConfig(ctx, client)
		return err == nil && strings.Contains(active, want)
	})
}

// caTrustActiveConfig reads the client's committed config out of its own store.
func caTrustActiveConfig(ctx context.Context, client *caTrustDaemon) (string, error) {
	data, err := managedRunCommand(ctx, client.env, client.dir, "",
		"data", "--path", client.dbPath, "cat", "file/active/"+caTrustClientName+".conf")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// caTrustHubConfig is the hub's own config.
//
// TWO server blocks, in this order. The FIRST is the plugin transport, which
// NewHubAcceptor takes from Servers[0]; the second is the managed listener,
// which startManagedServer takes from every block that names clients. One block
// serving both would ask two listeners to bind one address. The names sort in
// the same order they are written, so neither reading of the list changes which
// block is which.
func caTrustHubConfig(pluginPort, managedPort int, exportPath string) string {
	var b textbuf.Buffer
	b.Str("plugin {\n    hub {\n")
	b.Str("        server local { ip 127.0.0.1; port ").Str(strconv.Itoa(pluginPort)).
		Str("; secret \"").Str(caTrustPluginSecret).Str("\"; }\n")
	b.Str("        server remote-fleet { ip 127.0.0.1; port ").Str(strconv.Itoa(managedPort)).
		Str("; secret \"").Str(caTrustHubSecret).Str("\";\n")
	b.Str("            client ").Str(caTrustClientName).Str(" { secret \"").Str(caTrustClientSecret).Str("\"; }\n")
	b.Str("        }\n    }\n")
	b.Str("    external ca-export { run \"ze-test fixture managed/hub-ca-export -out ").Str(exportPath).
		Str("\"; encoder json; }\n")
	b.Str("}\n")
	b.Str("bgp {\n    router-id 10.0.0.1\n}\n")
	return b.String()
}

// caTrustClientConfig is the managed client's config, in the shape the operator
// writes it: the exported root pasted into a pki ca block, and the hub client
// naming that block.
func caTrustClientConfig(managedPort int, routerID, anchorPEM string) string {
	var b textbuf.Buffer
	b.Str("pki {\n    ca ").Str(caTrustAnchorName).Str(" {\n        certificate \"").
		Str(anchorPEM).Str("\";\n    }\n}\n")
	b.Str("plugin {\n    hub {\n")
	b.Str("        client ").Str(caTrustClientName).Str(" { host 127.0.0.1; port ").
		Str(strconv.Itoa(managedPort)).Str("; secret \"").Str(caTrustClientSecret).
		Str("\"; ca ").Str(caTrustAnchorName).Str("; }\n")
	b.Str("    }\n}\n")
	b.Str("bgp {\n    router-id ").Str(routerID).Str("\n}\n")
	return b.String()
}

// caTrustForeignRootPEM mints a certificate authority the hub never used. It is
// a well-formed root, so what the client refuses is the ISSUER and not the
// encoding.
func caTrustForeignRootPEM() ([]byte, error) {
	_, _, der, err := lgPKIIssueCA("another fleet root", 1, nil, nil)
	if err != nil {
		return nil, err
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), nil
}

// managedHubCAExportDriver runs inside the hub as an ordinary external plugin
// and writes the root the export command answers to the path it is given.
//
// It goes through `show pki local-ca pem`, the command an operator runs, so the
// bytes the client trusts below are the bytes the operator would have copied.
// Reading meta/ca/cert out of the store would prove the same handshake and a
// different chain of custody.
func managedHubCAExportDriver(ctx context.Context, args []string) error {
	flags := flag.NewFlagSet("managed/hub-ca-export", flag.ContinueOnError)
	out := flags.String("out", "", "where to write the exported root")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *out == "" {
		return errors.New("out is required")
	}

	plugin, err := newObserver("ca-export")
	if err != nil {
		return fmt.Errorf("connect ca-export: %w", err)
	}
	defer plugin.Close() //nolint:errcheck // the run result carries the useful transport error

	plugin.OnAllPluginsReady(func() error {
		go caTrustExportRoot(ctx, plugin, *out)
		return nil
	})
	// The hub stays up for the client to reach, so this returns only when the
	// daemon stops. It never asks for a shutdown.
	return plugin.Run(ctx, sdk.Registration{})
}

// caTrustExportRoot dispatches the export command and writes its PEM field.
// The write is a rename so the waiting fixture never reads a partial file.
func caTrustExportRoot(ctx context.Context, plugin *sdk.Plugin, out string) {
	root, err := fixture10PKIData(ctx, plugin, "show pki local-ca pem", "local-ca")
	if err != nil {
		ReportFailure(fmt.Errorf("export the local certificate authority root: %w", err))
		return
	}
	text, ok := root["pem"].(string)
	if !ok || text == "" {
		ReportFailure(errors.New("the local certificate authority export carried no pem field"))
		return
	}
	staging := out + ".partial"
	if writeErr := os.WriteFile(staging, []byte(text), 0o600); writeErr != nil {
		ReportFailure(fmt.Errorf("write the exported root: %w", writeErr))
		return
	}
	if renameErr := os.Rename(staging, out); renameErr != nil {
		ReportFailure(fmt.Errorf("publish the exported root: %w", renameErr))
	}
}
