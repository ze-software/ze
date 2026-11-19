// Design: docs/architecture/fleet-config.md -- the hub's session editor refuses a client no serving block declares
// Related: register_managed_hub_editor.go -- registers the driver
// Related: storage_live_client_fixture.go -- restoreClientDaemon, restoreClientHubConfig, restoreClientConfig, the hub this driver starts
// Related: plugin_fixture_session_editor.go -- sessionEditorClientEnv and sessionEditorRun, the SSH editor over a PTY

package fixture

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/creack/pty"
)

// The client the hub's serving block does not declare, the router-id the
// editor commits into the declared client's config, and the script that
// commits it. The served router-id is the one both stored configs start with.
const (
	hubEditorUndeclaredClient = "edge-99"
	hubEditorServedRouterID   = "2.2.2.2"
	hubEditorEditedRouterID   = "9.9.9.9"
	hubEditorCommitScript     = "send set bgp router-id " + hubEditorEditedRouterID + "\n" +
		"send commit now\n" +
		"wait Session committed:\n"
)

// The read of the hub's store after the editor reported its commit: ten
// seconds at most, far inside the .ci budget.
const (
	hubEditorActiveAttempts = 100
	hubEditorPollDelay      = 100 * time.Millisecond
)

// hubEditorAltScreen is what a terminal program writes when it takes the
// screen: the first byte sequence of an editor that opened.
const hubEditorAltScreen = "\x1b[?1049h"

// managedHubEditorDeclaredDriver proves spec-fleet-6 AC-20 with one real hub,
// `ze start`, whose serving block declares the managed client edge-01 and
// whose store holds a config for edge-01 and one for edge-99, which no block
// declares. No client daemon is started, so edge-01 is declared and not
// connected.
//
// `ze config edit client-edge-99.conf` beside the hub MUST be refused, with
// the reason and the fix the import gives, before any editor is drawn.
// `ze config edit client-edge-01.conf` MUST open, and a commit typed there
// MUST become the active version of that client's config in the hub's store.
//
// Args: <hub-ssh-port> <hub-plugin-port> <hub-managed-port>.
func managedHubEditorDeclaredDriver(ctx context.Context, args []string) (retErr error) {
	if len(args) != 3 {
		return errors.New("usage: managed/fleet-hub-editor-declared <hub-ssh-port> <hub-plugin-port> <hub-managed-port>")
	}
	sshPort, pluginPort, managedPort := args[0], args[1], args[2]
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	base, err := os.MkdirTemp(cwd, "hub-editor-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(base) //nolint:errcheck // fixture cleanup

	hub, err := restoreClientDaemon(ctx, base, "hub", false)
	if err != nil {
		return err
	}
	declared := "client-" + caTrustClientName + ".conf"
	undeclared := "client-" + hubEditorUndeclaredClient + ".conf"
	imports := []struct{ name, body string }{
		{caTrustHubName + ".conf", restoreClientHubConfig(sshPort, pluginPort, managedPort)},
		{declared, restoreClientConfig(managedPort, hubEditorServedRouterID)},
		{undeclared, restoreClientConfig(managedPort, hubEditorServedRouterID)},
	}
	for _, config := range imports {
		if err := caTrustImportConfig(ctx, hub, config.name, config.body); err != nil {
			return err
		}
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

	env, err := sessionEditorClientEnv(ctx, sshPort, valueTestUser)
	if err != nil {
		return err
	}
	// Both editors are typed in the hub's store folder, as an operator on the
	// hub's host types them: `ze config edit <name>` opens the store of the
	// folder its path names (resolve.StoreDir), the hub's here, and dials the
	// SSH address env names. Typed elsewhere, the bare name finds no store.
	if err := hubEditorRequireRefusal(ctx, env, hub.dir, undeclared, hubEditorUndeclaredClient); err != nil {
		return err
	}
	steps, err := parseSessionEditorScript(strings.NewReader(hubEditorCommitScript))
	if err != nil {
		return err
	}
	if err := sessionEditorRunIn(ctx, env, hub.dir, declared, steps); err != nil {
		return fmt.Errorf("the editor on %s, a client the hub declares: %w", declared, err)
	}
	if err := hubEditorRequireActive(ctx, hub, declared, hubEditorEditedRouterID); err != nil {
		return err
	}
	fmt.Fprintln(os.Stderr, "OK: the hub's session editor refused an undeclared client and committed a declared one that is not connected") //nolint:errcheck // fixture progress line
	return nil
}

// hubEditorRequireRefusal runs `ze config edit config` in dir over a PTY and
// requires the hub to refuse the session: the client exits non-zero, no editor
// was drawn, and the output names the config, the missing client entry and the
// fix. An editor that opens is killed and reported.
func hubEditorRequireRefusal(ctx context.Context, env []string, dir, config, client string) error {
	cmd := exec.CommandContext(ctx, "ze", "config", "edit", config) //nolint:gosec // the fixture chooses the program and its arguments
	cmd.Env = env
	cmd.Dir = dir
	terminal, err := pty.StartWithSize(cmd, &pty.Winsize{Rows: 30, Cols: 160})
	if err != nil {
		return err
	}
	defer terminal.Close() //nolint:errcheck // fixture teardown

	entry := "no client entry named " + client + " under plugin hub server"
	output, err := readPTYUntilContext04(ctx, terminal, nil, true, hubEditorAltScreen, entry)
	if strings.Contains(output, hubEditorAltScreen) {
		tail, _ := killSessionEditor(cmd, terminal) //nolint:errcheck // the editor opened, which is the failure reported below
		return fmt.Errorf("the editor opened on %s, a client no serving block declares:\n%s%s", config, output, tail)
	}
	if err != nil {
		_, _ = killSessionEditor(cmd, terminal) //nolint:errcheck // fixture teardown after a failed read
		return fmt.Errorf("reading the answer to ze config edit %s: %w\n%s", config, err, output)
	}
	// Drain to the end, so the client can exit and the fix line is read.
	output, err = readPTYUntil04(terminal, []byte(output), true)
	if err != nil {
		_, _ = killSessionEditor(cmd, terminal) //nolint:errcheck // fixture teardown after a failed read
		return fmt.Errorf("ze config edit %s did not exit after its refusal: %w\n%s", config, err, output)
	}
	if waitErr := cmd.Wait(); waitErr == nil {
		return fmt.Errorf("ze config edit %s exited zero after a refusal:\n%s", config, output)
	}
	for _, want := range []string{"config edit: " + config + " was not opened", entry, "so no client would ever fetch it", "ze config edit"} {
		if !strings.Contains(output, want) {
			return fmt.Errorf("the refusal of %s lacks %q:\n%s", config, want, output)
		}
	}
	return nil
}

// hubEditorRequireActive waits until the hub's active version of config
// carries routerID. The store is read through ze data's read-only access
// while the hub holds the writer handle.
func hubEditorRequireActive(ctx context.Context, hub *caTrustDaemon, config, routerID string) error {
	want := "router-id " + routerID
	var last string
	if Poll(ctx, hubEditorActiveAttempts, hubEditorPollDelay, func() bool {
		active, err := managedRunCommand(ctx, hub.env, hub.dir, "",
			"data", "--path", hub.dbPath, "cat", "file/active/"+config)
		last = string(active)
		return err == nil && strings.Contains(last, want)
	}) {
		return nil
	}
	return fmt.Errorf("the hub's active %s never carried %q after the editor's commit:\n%s", config, want, last)
}
