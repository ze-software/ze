// Design: docs/architecture/zefs-format.md -- the ze data CLI and its daemon forms
// Related: storage_live_fixture.go -- storageLiveClient, storageLiveRequest, the restored peer and router-id
// Related: storage_live_write_fixture.go -- the dataEnv shape and the content bound
// Related: register_storage_live.go -- registers the three import scenarios

package fixture

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/component/cli/sshclient"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/core/resolve"
	"github.com/ze-software/ze/pkg/zefs"
)

// The configs the import scenarios replace, each the config its daemon runs,
// and the other config data-import-live.ci imports beside the running one.
// data-import-existing.ci starts its second daemon with a bare `ze start`,
// which takes its config from the store under the name resolve.DefaultConfig
// answers for a store with no instance name: so its config is that name.
const (
	storageImportExistingName = resolve.DefaultConfigName
	storageImportLiveName     = "data-import-live.conf"
	storageImportOtherName    = "data-import-other.conf"
)

// storageImportRefusedRouterID makes a config the reload refuses: it is not an
// IPv4 address.
const storageImportRefusedRouterID = "router-id 300.0.0.1"

// storageLiveImportSeed is the offline half of AC-9, run while ze is stopped:
// the store holds an active version of the config the first start booted from
// its file; `ze data import` of a file of that name prints the local route's
// lines and commits the restored config as the new active version, the
// previous one kept as rollback. The daemon started next from the store must
// run it (storageLiveImportExisting).
func storageLiveImportSeed(ctx context.Context, _ []string) error {
	booted, err := os.ReadFile(storageImportExistingName)
	if err != nil {
		return err
	}
	restored, err := storageImportRestored(booted)
	if err != nil {
		return err
	}
	if err := storageImportHoldsKey(zefs.KeyConfigActive.Key(storageImportExistingName)); err != nil {
		return fmt.Errorf("the first start left no active version to replace: %w", err)
	}
	if err := storageImportStartsName(storageImportExistingName); err != nil {
		return err
	}
	output, code, err := storageLiveDataImport(ctx, os.Environ(), storageImportExistingName, restored)
	if err != nil {
		return err
	}
	if code != 0 {
		return fmt.Errorf("ze data import exited %d, want 0:\n%s", code, output)
	}
	if err := storageImportPrinted(output, storageImportExistingName, len(restored)); err != nil {
		return err
	}
	if err := storageImportActiveHolds(storageImportExistingName, restored); err != nil {
		return err
	}
	if err := storageImportHoldsKey(zefs.KeyConfigRollback.Key(storageImportExistingName)); err != nil {
		return fmt.Errorf("the replaced version is not kept as rollback: %w", err)
	}
	return storageLiveOK("data-import-existing-seed")
}

// storageLiveImportExisting is the daemon half of AC-9: the daemon started
// from the store after the offline import (a bare `ze start`, which reads the
// active version, while a start naming the file would rebuild the store from
// that file) serves the restored peer and router-id, so it read the imported
// config and not the version the import replaced. The config file beside it
// still holds the boot router-id, so the answer can only come from the store.
func storageLiveImportExisting(ctx context.Context, args []string) error {
	env, err := storageLiveClient(ctx, args)
	if err != nil {
		return err
	}
	if err := storageImportServesRestored(ctx, env); err != nil {
		return err
	}
	return storageLiveOK("data-import-existing")
}

// storageLiveImportLocally proves AC-10 on a running daemon over the real SSH
// route. The local `ze data import` of the running config beside the daemon
// prints the local route's lines, the daemon reloads and serves the restored
// peer and router-id, and `show config history` lists one version more. A
// config the reload refuses exits 2 with the reload's reason and leaves the
// active config, the history and the running router-id as they were.
// `update data config` sent over SSH exec for another name commits it with no
// reload, and a name that is a directory step is refused.
func storageLiveImportLocally(ctx context.Context, args []string) error {
	env, err := storageLiveClient(ctx, args)
	if err != nil {
		return err
	}
	dataEnv := overrideEnv04(os.Environ(),
		"ZE_SSH_PASSWORD=testpass",
		"ZE_SSH_HOST="+addrLoopback,
		"ZE_SSH_PORT="+args[0],
		"ZE_SSH_USERNAME="+storageSeedUser,
		"NO_COLOR=1",
	)
	creds := sshclient.Credentials{Host: addrLoopback, Port: args[0], Username: storageSeedUser, Auth: valueTestPassword}
	booted, err := os.ReadFile(storageImportLiveName)
	if err != nil {
		return err
	}
	restored, err := storageImportRestored(booted)
	if err != nil {
		return err
	}
	before, err := storageImportRevisions(creds, storageImportLiveName)
	if err != nil {
		return err
	}

	output, code, err := storageLiveDataImport(ctx, dataEnv, storageImportLiveName, restored)
	if err != nil {
		return err
	}
	if code != 0 {
		return fmt.Errorf("ze data import of the running config exited %d, want 0:\n%s", code, output)
	}
	if err := storageImportPrinted(output, storageImportLiveName, len(restored)); err != nil {
		return err
	}
	if err := storageImportServesRestored(ctx, env); err != nil {
		return err
	}
	imported, err := storageImportRevisions(creds, storageImportLiveName)
	if err != nil {
		return err
	}
	if imported != before+1 {
		return fmt.Errorf("show config history lists %d revisions after the import, want %d", imported, before+1)
	}

	if err := storageImportRefused(ctx, dataEnv, restored); err != nil {
		return err
	}
	refused, err := storageImportRevisions(creds, storageImportLiveName)
	if err != nil {
		return err
	}
	if refused != imported {
		return fmt.Errorf("show config history lists %d revisions after a refused import, want %d", refused, imported)
	}
	if err := storageImportServesRestored(ctx, env); err != nil {
		return fmt.Errorf("after a refused import: %w", err)
	}

	if err := storageImportOtherOverSSH(creds); err != nil {
		return err
	}
	return storageLiveOK("data-import-live")
}

// storageImportRefused imports a config the reload refuses beside the daemon
// and requires exit 2, the reload's reason, and the active config still the
// restored one.
func storageImportRefused(ctx context.Context, env []string, restored []byte) error {
	broken := bytes.Replace(restored, []byte(storageLiveRestoredRouterID), []byte(storageImportRefusedRouterID), 1)
	output, code, err := storageLiveDataImport(ctx, env, storageImportLiveName, broken)
	if err != nil {
		return err
	}
	if code != 2 {
		return fmt.Errorf("ze data import of a config the reload refuses exited %d, want 2:\n%s", code, output)
	}
	for _, want := range []string{storageImportLiveName + " was not imported", "the reload refused the config"} {
		if !strings.Contains(output, want) {
			return fmt.Errorf("the refused import lacks %q:\n%s", want, output)
		}
	}
	return storageImportActiveHolds(storageImportLiveName, restored)
}

// storageImportOtherOverSSH sends `update data config` over SSH exec, the
// route a remote operator takes, for a config the daemon does not run, and
// requires its new active version; a name that is a directory step is refused
// naming the rule.
func storageImportOtherOverSSH(creds sshclient.Credentials) error {
	other := []byte("# a config imported beside the running one\n")
	output, err := sshclient.ExecCommand(creds, "update data config name "+storageImportOtherName+" content "+
		base64.StdEncoding.EncodeToString(other))
	if err != nil {
		return fmt.Errorf("update data config of another name over SSH exec: %w", err)
	}
	if !strings.Contains(output, storageImportOtherName) {
		return fmt.Errorf("update data config answer lacks %s:\n%s", storageImportOtherName, output)
	}
	if err := storageImportActiveHolds(storageImportOtherName, other); err != nil {
		return err
	}
	_, err = sshclient.ExecCommand(creds, "update data config name .. content YWJj")
	if err == nil {
		return errors.New("update data config name .. was not refused")
	}
	if !strings.Contains(err.Error(), "is not a plain config name") {
		return fmt.Errorf("update data config name .. refused without the rule: %w", err)
	}
	return nil
}

// storageImportRestored answers booted with the restored router-id and peer
// in place of the boot router-id.
func storageImportRestored(booted []byte) ([]byte, error) {
	if !bytes.Contains(booted, []byte(storageLiveBootRouterID)) {
		return nil, fmt.Errorf("the config does not boot with %s", storageLiveBootRouterID)
	}
	return bytes.Replace(booted, []byte(storageLiveBootRouterID),
		[]byte(storageLiveRestoredRouterID+"\n"+storageLiveRestoredPeer), 1), nil
}

// storageImportServesRestored requires the daemon to serve the restored peer
// and router-id, which only a reload of the imported config applies.
func storageImportServesRestored(ctx context.Context, env []string) error {
	var last string
	var err error
	if !Poll(ctx, 50, 200*time.Millisecond, func() bool {
		last, err = storageLiveRequest(ctx, env, "show bgp | json")
		if err != nil {
			return false
		}
		if !storageLivePeersConfigured.MatchString(last) {
			return false
		}
		return storageLiveRestoredRouterIDJSON.MatchString(last)
	}) {
		return fmt.Errorf("the daemon never served the imported peer and router-id; show bgp answers:\n%s", last)
	}
	return nil
}

// storageImportRevisions answers how many revisions `show config history`
// lists for name, asked of the daemon over SSH exec so the daemon's own store
// answers.
func storageImportRevisions(creds sshclient.Credentials, name string) (int, error) {
	output, err := sshclient.ExecCommand(creds, "show config history "+name+" | json")
	if err != nil {
		return 0, fmt.Errorf("show config history %s: %w", name, err)
	}
	return strings.Count(output, `"revision":`), nil
}

// storageImportPrinted requires the local route's two lines for one file of
// octets bytes imported as name.
func storageImportPrinted(output, name string, octets int) error {
	lines := strings.Split(strings.TrimSuffix(output, "\n"), "\n")
	if len(lines) != 2 {
		return fmt.Errorf("ze data import printed %d lines, want the local route's two:\n%s", len(lines), output)
	}
	want := "imported " + zefs.KeyFileActive.Key(name) + " (" + strconv.Itoa(octets) + " bytes)"
	if lines[0] != want {
		return fmt.Errorf("ze data import printed %q, want %q", lines[0], want)
	}
	if !strings.HasPrefix(lines[1], "1 files imported into ") {
		return fmt.Errorf("ze data import ended with %q, want the count of one file", lines[1])
	}
	return nil
}

// storageImportActiveHolds requires a fresh reader handle on the daemon's
// store to read want as the active config of name.
func storageImportActiveHolds(name string, want []byte) error {
	tree, err := openDaemonStoreReadOnly(os.Environ(), ".")
	if err != nil {
		return err
	}
	active, err := storage.ReadActiveConfig(tree, name)
	if closeErr := tree.Close(); closeErr != nil {
		return errors.Join(err, closeErr)
	}
	if err != nil {
		return fmt.Errorf("read the active config of %s: %w", name, err)
	}
	if !bytes.Equal(active, want) {
		return fmt.Errorf("the active config of %s holds %d bytes, not the %d bytes imported", name, len(active), len(want))
	}
	return nil
}

// storageImportStartsName requires the store to name name as the config a
// bare `ze start` boots (resolve.DefaultConfig, the producer the start calls),
// so the next start reads the version the import commits and not another
// config.
func storageImportStartsName(name string) error {
	tree, err := openDaemonStoreReadOnly(os.Environ(), ".")
	if err != nil {
		return err
	}
	started, err := resolve.DefaultConfig(tree)
	if closeErr := tree.Close(); closeErr != nil {
		return errors.Join(err, closeErr)
	}
	if err != nil {
		return err
	}
	if started != name {
		return fmt.Errorf("a bare ze start boots %s from this store, not %s", started, name)
	}
	return nil
}

// storageImportHoldsKey requires a fresh reader handle on the daemon's store
// to read key.
func storageImportHoldsKey(key string) error {
	tree, err := openDaemonStoreReadOnly(os.Environ(), ".")
	if err != nil {
		return err
	}
	_, err = tree.ReadKey(key)
	if closeErr := tree.Close(); closeErr != nil {
		return errors.Join(err, closeErr)
	}
	return err
}

// storageLiveDataImport writes data to a file named name in a folder of its
// own and runs the local `ze data import <file>`, answering its output (stdout
// and stderr together) and its exit code. An error is a run that did not reach
// an exit code, never a refusal.
func storageLiveDataImport(ctx context.Context, env []string, name string, data []byte) (string, int, error) {
	folder, err := filepath.Abs("import")
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
	output, err := runCommandProcess04(ctx, env, nil, "data", "import", path)
	if err == nil {
		return output, 0, nil
	}
	if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
		return output, exitErr.ExitCode(), nil
	}
	return output, 0, err
}
