// Design: docs/guide/authorization.md -- the read-only profile and the store maintenance commands
// Related: storage_live_fixture.go -- storageLiveClient, storageLiveStaleKey, storageLiveHolds
// Related: storage_live_repair_fixture.go -- storageLiveCleanVerdict, the show data check document
// Related: register_storage_live.go -- registers storage/data-authz-read-only

package fixture

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/ze-software/ze/internal/component/cli/sshclient"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/pkg/zefs"
)

// The read-only user data-authz-read-only.ci configures, with the password
// its bcrypt hash is of, as test/plugin/rbac-ssh-only-enforced.ci holds it.
const (
	storageAuthzReader       = "noc"
	storageAuthzReaderSecret = "noc-secret"
)

// storageAuthzOtherName is the config the two `update data config` attempts
// name: one the daemon does not run, so an allowed import commits a version
// and runs no reload.
const storageAuthzOtherName = "data-authz-other.conf"

// storageAuthzSalvage is the folder, under the test's working folder, the two
// `request data repair` attempts name.
const storageAuthzSalvage = "authz-salvage"

// storageLiveAuthzReadOnly proves AC-14 on a running daemon over SSH exec,
// the route every remote operator takes. A user whose profile is the
// read-only one is refused `delete data key`, `update data key`,
// `update data config` and `request data repair` by authorization, with the
// access-control refusal, and nothing they name changes: the seeded key still
// holds its bytes, the other config has no active version, no salvage exists.
// The same user is allowed `show data check`, answered by the daemon on its
// own tree. An admin is then allowed all five, each proven by its effect.
func storageLiveAuthzReadOnly(ctx context.Context, args []string) error {
	if _, err := storageLiveClient(ctx, args); err != nil {
		return err
	}
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	salvage := filepath.Join(cwd, storageAuthzSalvage)
	tree := filepath.Join(cwd, storageTreeName)
	reader := sshclient.Credentials{Host: addrLoopback, Port: args[0], Username: storageAuthzReader, Auth: storageAuthzReaderSecret}
	admin := sshclient.Credentials{Host: addrLoopback, Port: args[0], Username: storageSeedUser, Auth: valueTestPassword}
	seeded, err := storageAuthzKeyBytes()
	if err != nil {
		return err
	}
	written := []byte("written by the admin\n")
	other := []byte("# a config the read-only profile may not import\n")

	if err := storageAuthzReaderDenied(reader, salvage, other); err != nil {
		return err
	}
	if err := storageAuthzNothingChanged(seeded, salvage); err != nil {
		return err
	}
	checked, err := sshclient.ExecCommand(reader, "show data check | json")
	if err != nil {
		return fmt.Errorf("the read-only profile was refused show data check: %w:\n%s", err, checked)
	}
	if err := storageLiveCleanVerdict("read-only profile", checked, tree); err != nil {
		return err
	}

	checked, err = sshclient.ExecCommand(admin, "show data check | json")
	if err != nil {
		return fmt.Errorf("the admin was refused show data check: %w:\n%s", err, checked)
	}
	if err := storageLiveCleanVerdict("admin", checked, tree); err != nil {
		return err
	}
	if err := storageAuthzAdminAllowed(admin, salvage, written, other); err != nil {
		return err
	}
	return storageLiveOK("data-authz-read-only")
}

// storageAuthzReaderDenied sends the four store writes as the read-only user
// and requires each refused by authorization. Each command line starts with
// its literal path, so the dispatch check can route it.
func storageAuthzReaderDenied(reader sshclient.Credentials, salvage string, other []byte) error {
	content := base64.StdEncoding.EncodeToString([]byte("written by the read-only profile\n"))
	output, err := sshclient.ExecCommand(reader, "delete data key "+storageLiveStaleKey)
	if err := storageAuthzRefused("delete data key", output, err); err != nil {
		return err
	}
	output, err = sshclient.ExecCommand(reader, "update data key "+storageLiveStaleKey+" content "+content)
	if err := storageAuthzRefused("update data key", output, err); err != nil {
		return err
	}
	output, err = sshclient.ExecCommand(reader, "update data config name "+storageAuthzOtherName+" content "+
		base64.StdEncoding.EncodeToString(other))
	if err := storageAuthzRefused("update data config", output, err); err != nil {
		return err
	}
	output, err = sshclient.ExecCommand(reader, "request data repair path "+salvage)
	return storageAuthzRefused("request data repair", output, err)
}

// storageAuthzRefused requires the answer to command to be the access-control
// refusal, the text every surface prints for a command a profile denies, and
// not a handler's own refusal.
func storageAuthzRefused(command, output string, err error) error {
	if err == nil {
		return fmt.Errorf("the read-only profile was allowed %s:\n%s", command, output)
	}
	if !strings.Contains(err.Error(), plugin.UnauthorizedMessage) {
		return fmt.Errorf("%s was refused, but not by authorization (want %q): %w", command, plugin.UnauthorizedMessage, err)
	}
	return nil
}

// storageAuthzNothingChanged requires the state the four refused writes named
// to be as it was: the seeded key holds its bytes, the other config has no
// active version, and no salvage exists.
func storageAuthzNothingChanged(seeded []byte, salvage string) error {
	held, err := storageAuthzKeyBytes()
	if err != nil {
		return fmt.Errorf("after the refused writes: %w", err)
	}
	if !bytes.Equal(held, seeded) {
		return fmt.Errorf("%s changed after the refused writes: %q, was %q", storageLiveStaleKey, held, seeded)
	}
	err = storageImportHoldsKey(zefs.KeyConfigActive.Key(storageAuthzOtherName))
	if err == nil {
		return fmt.Errorf("the refused update data config committed %s", storageAuthzOtherName)
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("read the active pointer of %s: %w", storageAuthzOtherName, err)
	}
	_, statErr := os.Lstat(salvage)
	if statErr == nil {
		return fmt.Errorf("the refused request data repair left %s", salvage)
	}
	if !errors.Is(statErr, fs.ErrNotExist) {
		return fmt.Errorf("stat %s after the refused request data repair: %w", salvage, statErr)
	}
	return nil
}

// storageAuthzAdminAllowed sends the four store writes as the admin and
// requires each one's effect: the key holds the bytes written, a salvage
// exists, the other config holds the bytes imported, and the key is then gone.
func storageAuthzAdminAllowed(admin sshclient.Credentials, salvage string, written, other []byte) error {
	output, err := sshclient.ExecCommand(admin, "update data key "+storageLiveStaleKey+" content "+
		base64.StdEncoding.EncodeToString(written))
	if err != nil {
		return fmt.Errorf("the admin was refused update data key: %w:\n%s", err, output)
	}
	held, err := storageAuthzKeyBytes()
	if err != nil {
		return err
	}
	if !bytes.Equal(held, written) {
		return fmt.Errorf("%s holds %q after the admin's write, want %q", storageLiveStaleKey, held, written)
	}
	output, err = sshclient.ExecCommand(admin, "request data repair path "+salvage)
	if err != nil {
		return fmt.Errorf("the admin was refused request data repair: %w:\n%s", err, output)
	}
	if _, err := os.Stat(salvage); err != nil {
		return fmt.Errorf("the admin's request data repair wrote no salvage: %w", err)
	}
	output, err = sshclient.ExecCommand(admin, "update data config name "+storageAuthzOtherName+" content "+
		base64.StdEncoding.EncodeToString(other))
	if err != nil {
		return fmt.Errorf("the admin was refused update data config: %w:\n%s", err, output)
	}
	if err := storageImportActiveHolds(storageAuthzOtherName, other); err != nil {
		return err
	}
	output, err = sshclient.ExecCommand(admin, "delete data key "+storageLiveStaleKey)
	if err != nil {
		return fmt.Errorf("the admin was refused delete data key: %w:\n%s", err, output)
	}
	present, err := storageLiveHolds(storageLiveStaleKey)
	if err != nil {
		return err
	}
	if present {
		return fmt.Errorf("%s is still held after the admin's delete data key", storageLiveStaleKey)
	}
	return nil
}

// storageAuthzKeyBytes reads the seeded key through a fresh reader handle on
// the daemon's store.
func storageAuthzKeyBytes() ([]byte, error) {
	tree, err := openDaemonStoreReadOnly(os.Environ(), ".")
	if err != nil {
		return nil, err
	}
	held, err := tree.ReadKey(storageLiveStaleKey)
	if closeErr := tree.Close(); closeErr != nil {
		return nil, errors.Join(err, closeErr)
	}
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", storageLiveStaleKey, err)
	}
	return held, nil
}
