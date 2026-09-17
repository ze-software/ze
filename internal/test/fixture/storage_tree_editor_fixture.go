// Design: docs/architecture/testing/ci-format.md -- explicit source authority across editor commits

package fixture

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/creack/pty"

	"github.com/ze-software/ze/internal/component/config/storage"
)

func init() {
	Register("storage/explicit-editor-authority", storageExplicitEditorAuthority)
}

// storageExplicitEditorAuthority drives SSH login and an editor commit, restarts
// on the updated source, then edits that source behind an open editor session.
func storageExplicitEditorAuthority(ctx context.Context, _ []string) error {
	port, err := uiFreeTCPPort()
	if err != nil {
		return err
	}
	config := fmt.Sprintf(`system {
 authentication { user admin { password "$2a$04$UlwuiuH82Unfsq.XEMPGJeDkXwbm3KW.nvVaVXOd/JeFK8VjMjrQO"; profile [ admin ]; } }
 authorization { profile admin { run { default-action allow; } edit { default-action allow; } } }
}
environment { ssh { enabled true; server main { ip 127.0.0.1; port %d; } } }
bgp { router-id 1.2.3.4; }
`, port)
	path, err := filepath.Abs("cli-commit.conf")
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, []byte(config), 0o600); err != nil {
		return err
	}
	daemon, err := extra1StartDaemon(ctx, path, "storage-editor.log", nil)
	if err != nil {
		return err
	}
	defer daemon.stop()
	sshPort, err := daemon.waitSSH(ctx)
	if err != nil {
		return err
	}
	clientDir, err := extra1InitCLI(ctx, sshPort)
	if err != nil {
		return err
	}
	clientEnv := extra1Environment(map[string]string{"ZE_CONFIG_DIR": clientDir, "ZE_SSH_HOST": addrLoopback, "ZE_SSH_PORT": sshPort, "ZE_SSH_USERNAME": storageSeedUser, "ZE_SSH_PASSWORD": "testpass", "TERM": "xterm", "NO_COLOR": "1"})
	defer os.RemoveAll(clientDir) //nolint:errcheck // fixture cleanup
	if _, err := runCommandProcess04(ctx, clientEnv, nil, "ze", "cli", "-c", "show version"); err != nil {
		return fmt.Errorf("tree credentials did not authenticate: %w", err)
	}
	if _, err := driveEditor04(ctx, clientEnv, "cli-commit.conf", false); err != nil {
		return err
	}
	daemon.stop()
	committed, err := os.ReadFile(path) //nolint:gosec // the path is the fixture's own scratch file
	if err != nil {
		return err
	}
	if !bytes.Contains(committed, []byte(editorRouterID)) {
		return fmt.Errorf("successful editor commit did not update explicit source: %s", committed)
	}
	if err := storageAssertActive(path); err != nil {
		return err
	}
	restarted, err := extra1StartDaemon(ctx, path, "storage-editor-restart.log", nil)
	if err != nil {
		return err
	}
	defer restarted.stop()
	if _, err := restarted.waitSSH(ctx); err != nil {
		return err
	}
	if err := storageAssertActive(path); err != nil {
		return err
	}
	external := bytes.ReplaceAll(committed, []byte(editorRouterID), []byte("9.9.9.9"))
	if err := storageEditorConflict(ctx, clientEnv, path, external); err != nil {
		return err
	}
	after, err := os.ReadFile(path) //nolint:gosec // the path is the fixture's own scratch file
	if err != nil {
		return err
	}
	if !bytes.Equal(after, external) {
		return errors.New("refused editor commit overwrote external source edit")
	}
	if err := storageAssertActive(path); err != nil {
		return err
	}
	fmt.Fprintln(os.Stdout, "OK: tree login, editor commit, restart and external conflict") //nolint:errcheck // status output
	return nil
}

// editorRouterID is the router-id the editor scenarios commit, and the value
// every later read of the active config MUST still hold.
const editorRouterID = "2.2.2.2"

// storageAssertActive reads the active config named by path from the tree in
// its folder and checks it holds editorRouterID.
func storageAssertActive(path string) error {
	store, err := storage.OpenReadOnly(filepath.Dir(path))
	if err != nil {
		return err
	}
	defer store.Close() //nolint:errcheck // fixture cleanup
	active, err := storage.ReadActiveConfig(store, filepath.Base(path))
	if err != nil {
		return err
	}
	if !bytes.Contains(active, []byte(editorRouterID)) {
		return fmt.Errorf("active config does not hold router-id %s: %s", editorRouterID, active)
	}
	return nil
}

func storageEditorConflict(ctx context.Context, environment []string, path string, external []byte) error {
	command := exec.CommandContext(ctx, "ze", "config", "edit", filepath.Base(path)) //nolint:gosec // the fixture chooses the program and its arguments
	command.Env = environment
	terminal, err := pty.StartWithSize(command, &pty.Winsize{Rows: 30, Cols: 120})
	if err != nil {
		return err
	}
	defer terminal.Close() //nolint:errcheck // fixture cleanup
	defer func() {
		_ = command.Process.Kill()
		_ = command.Wait()
	}()
	transcript, err := readPTYUntil04(terminal, nil, 20*time.Second, false, "\x1b[?1049h", "╭")
	if err != nil {
		return err
	}
	if _, err := terminal.WriteString("set bgp router-id 3.3.3.3\r"); err != nil {
		return err
	}
	transcript, err = readPTYUntil04(terminal, []byte(transcript), 20*time.Second, false, "3.3.3.3")
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, external, 0o600); err != nil {
		return err
	}
	if _, err := terminal.WriteString("commit\r"); err != nil {
		return err
	}
	transcript, err = readPTYUntil04(terminal, []byte(transcript), 20*time.Second, false, "commit failed:", "commit blocked:", "Configuration committed")
	if err != nil {
		return err
	}
	if !strings.Contains(transcript, "explicit config changed externally") {
		return fmt.Errorf("editor did not report explicit-source conflict:\n%s", transcript)
	}
	return nil
}
