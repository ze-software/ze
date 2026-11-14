// Design: docs/guide/config-editor.md -- ze config import beside a running daemon
// Related: storage_live_import_fixture.go -- the restored config, the revision count and the active-config reads
// Related: register_storage_live.go -- registers storage/config-import-live

package fixture

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"

	"github.com/ze-software/ze/internal/component/cli/sshclient"
)

// storageConfigImportName is the config config-import-live.ci's daemon runs
// and the local `ze config import` replaces.
const storageConfigImportName = "config-import-live.conf"

// storageLiveConfigImportLocally proves AC-16 on a running daemon over the
// real SSH route. The local `ze config import --yes` of the running config
// beside the daemon prints the offline route's lines, `replaced <name> (<n>
// bytes)` then `1 file(s) imported`, and exits 0; the daemon reloads and
// serves the imported peer and router-id; `show config history` lists one
// revision more; and the daemon's store reads the imported bytes as the
// active config.
func storageLiveConfigImportLocally(ctx context.Context, args []string) error {
	env, err := storageLiveClient(ctx, args)
	if err != nil {
		return err
	}
	importEnv := overrideEnv04(os.Environ(),
		"ZE_SSH_PASSWORD="+valueTestPassword,
		"ZE_SSH_HOST="+addrLoopback,
		"ZE_SSH_PORT="+args[0],
		"ZE_SSH_USERNAME="+storageSeedUser,
		"NO_COLOR=1",
	)
	creds := sshclient.Credentials{Host: addrLoopback, Port: args[0], Username: storageSeedUser, Auth: valueTestPassword}
	booted, err := os.ReadFile(storageConfigImportName)
	if err != nil {
		return err
	}
	restored, err := storageImportRestored(booted)
	if err != nil {
		return err
	}
	before, err := storageImportRevisions(creds, storageConfigImportName)
	if err != nil {
		return err
	}

	output, code, err := storageLiveConfigImport(ctx, importEnv, storageConfigImportName, restored)
	if err != nil {
		return err
	}
	if code != 0 {
		return fmt.Errorf("ze config import --yes of the running config exited %d, want 0:\n%s", code, output)
	}
	want := "replaced " + storageConfigImportName + " (" + strconv.Itoa(len(restored)) + " bytes)\n1 file(s) imported\n"
	if output != want {
		return fmt.Errorf("ze config import printed %q, want the offline route's lines %q", output, want)
	}
	if err := storageImportServesRestored(ctx, env); err != nil {
		return err
	}
	after, err := storageImportRevisions(creds, storageConfigImportName)
	if err != nil {
		return err
	}
	if after != before+1 {
		return fmt.Errorf("show config history lists %d revisions after the import, want %d", after, before+1)
	}
	if err := storageImportActiveHolds(storageConfigImportName, restored); err != nil {
		return err
	}
	return storageLiveOK("config-import-live")
}

// storageLiveConfigImport writes data to a file named name in a folder of its
// own and runs the local `ze config import --yes <file>`, answering its output
// (stdout and stderr together) and its exit code. An error is a run that did
// not reach an exit code, never a refusal.
func storageLiveConfigImport(ctx context.Context, env []string, name string, data []byte) (string, int, error) {
	folder, err := filepath.Abs("config-import")
	if err != nil {
		return "", 0, err
	}
	if err := os.MkdirAll(folder, 0o750); err != nil {
		return "", 0, err
	}
	path := filepath.Join(folder, name)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return "", 0, err
	}
	output, err := runCommandProcess04(ctx, env, nil, "config", "import", "--yes", path)
	if err == nil {
		return output, 0, nil
	}
	if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
		return output, exitErr.ExitCode(), nil
	}
	return output, 0, err
}
