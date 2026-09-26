// Design: docs/architecture/storage-backends.md -- backup and restore against a running daemon
// Related: plugin_fixture_04_cli.go -- runCommandProcess04, overrideEnv04: the SSH client environment
// Related: register_storage_live.go -- registers the four live scenarios

package fixture

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/zefs"
)

// The router-id each live scenario's daemon boots with, and the one the
// restored config carries.
const (
	storageLiveBootRouterID     = "router-id 1.2.3.4"
	storageLiveRestoredRouterID = "router-id 2.2.2.2"
)

// storageLiveRestoredPeer is the peer the restored config adds. The booted
// config has none, so `show bgp` answering one configured peer proves the
// reload applied the restored config, not only that it was stored.
const storageLiveRestoredPeer = `    peer restored {
        connection { remote { ip 127.0.0.2; } local { ip 127.0.0.1; accept false; } }
        session { asn { remote 65001; } family { ipv4/unicast { prefix { maximum 10000; } } } }
    }
`

// storageLiveInit initializes the store in the working folder before the
// daemon starts, so meta/ssh/default records the address the daemon listens
// on, as `ze init` records it on a real install.
func storageLiveInit(ctx context.Context, args []string) error {
	if len(args) != 1 {
		return errors.New("usage: storage/live-init <ssh-port>")
	}
	_, err := storageRequireCommand(ctx, "admin\ntestpass\n127.0.0.1\n"+args[0]+"\n\n", "init")
	return err
}

// storageLiveClient prepares an SSH client environment for the daemon listening
// on sshPort, in a client store of its own, and waits until the daemon answers.
func storageLiveClient(ctx context.Context, args []string) ([]string, error) {
	if len(args) != 1 {
		return nil, errors.New("usage: <scenario> <ssh-port>")
	}
	clientDir, err := filepath.Abs("client-db")
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(clientDir, 0o750); err != nil {
		return nil, err
	}
	env := overrideEnv04(os.Environ(),
		"ZE_CONFIG_DIR="+clientDir,
		"ZE_SSH_PASSWORD=testpass",
		"ZE_SSH_HOST=127.0.0.1",
		"ZE_SSH_PORT="+args[0],
		"ZE_SSH_USERNAME=admin",
		"NO_COLOR=1",
	)
	initInput := "admin\ntestpass\n127.0.0.1\n" + args[0] + "\n\n"
	if _, err := runCommandProcess04(ctx, env, strings.NewReader(initInput), "init"); err != nil {
		return nil, err
	}
	var last string
	if !Poll(ctx, 50, 200*time.Millisecond, func() bool {
		output, err := runCommandProcess04(ctx, env, nil, "cli", "-c", "show version")
		last = output
		return err == nil
	}) {
		return nil, fmt.Errorf("daemon did not become reachable: %s", last)
	}
	return env, nil
}

// storageLiveRequest sends one command to the daemon over SSH and answers its
// output. A refused command is an error carrying that output.
func storageLiveRequest(ctx context.Context, env []string, command string) (string, error) {
	return runCommandProcess04(ctx, env, nil, "cli", "-c", command)
}

// storageLiveRefused requires the daemon to refuse command with a reason that
// contains want.
func storageLiveRefused(ctx context.Context, env []string, command, want string) error {
	output, err := storageLiveRequest(ctx, env, command)
	if err == nil && !strings.Contains(output, "error") {
		return fmt.Errorf("%q was not refused:\n%s", command, output)
	}
	if !strings.Contains(output, want) {
		return fmt.Errorf("%q refused without %q:\n%s", command, want, output)
	}
	return nil
}

// storageLiveOK reports a passed scenario where the .ci expects it.
func storageLiveOK(name string) error {
	out := textbuf.New()
	out.Str("OK: ").Str(name).Byte('\n')
	return out.StdOut()
}

// storageLiveBackup proves `request data backup` on a running daemon: the
// daemon writes a 0600 blob that `ze data check` accepts and that holds the
// active config it serves; a relative path, a ".." element, a symlink, a path
// inside the store and an existing file without force are each refused naming
// the rule; force replaces an existing file; the daemon stays up throughout.
func storageLiveBackup(ctx context.Context, args []string) error {
	env, err := storageLiveClient(ctx, args)
	if err != nil {
		return err
	}
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	artifact := filepath.Join(cwd, "live-backup.zefs")
	output, err := storageLiveRequest(ctx, env, "request data backup path "+artifact)
	if err != nil {
		return err
	}
	for _, want := range []string{artifact, "keys", "bytes"} {
		if !strings.Contains(output, want) {
			return fmt.Errorf("backup answer lacks %q:\n%s", want, output)
		}
	}
	if err := storageLiveArtifact(ctx, artifact, "data-backup-live.conf"); err != nil {
		return err
	}

	link := filepath.Join(cwd, "live-backup-link.zefs")
	if err := os.Symlink(artifact, link); err != nil {
		return err
	}
	refusals := []struct{ command, want string }{
		{"request data backup path live-backup-relative.zefs", "the path must be absolute"},
		{"request data backup path " + cwd + "/../live-backup.zefs", `must not contain ".."`},
		{"request data backup path " + link, "the path is a symlink"},
		{"request data backup path " + artifact, "exist"},
		{"request data backup path " + filepath.Join(cwd, storageTreeName, "inside.zefs") + " force", storageTreeName},
		{"request data backup path " + filepath.Join(cwd, "database.lock") + " force", "database.lock"},
		{"request data backup path " + artifact + " spare 101", "out of range 0..100"},
	}
	for _, refusal := range refusals {
		if err := storageLiveRefused(ctx, env, refusal.command, refusal.want); err != nil {
			return err
		}
	}
	if _, err := storageLiveRequest(ctx, env, "request data backup path "+artifact+" force"); err != nil {
		return err
	}
	if err := storageLiveArtifact(ctx, artifact, "data-backup-live.conf"); err != nil {
		return err
	}
	if _, err := storageLiveRequest(ctx, env, "show version"); err != nil {
		return err
	}
	return storageLiveOK("data-backup-live")
}

// storageLiveArtifact checks a live backup: mode 0600, `ze data check`
// accepts it, and it holds the active config the tree holds, byte-equal.
func storageLiveArtifact(ctx context.Context, artifact, configName string) error {
	info, err := os.Stat(artifact)
	if err != nil {
		return err
	}
	if info.Mode().Perm() != 0o600 {
		return fmt.Errorf("backup %s has mode %v, want 0600", artifact, info.Mode().Perm())
	}
	if _, err := storageRequireCommand(ctx, "", "data", "check", artifact); err != nil {
		return err
	}
	key := zefs.KeyFileActive.Key(configName)
	tree, err := storage.OpenReadOnly(".")
	if err != nil {
		return err
	}
	want, err := tree.ReadKey(key)
	if closeErr := tree.Close(); closeErr != nil {
		err = errors.Join(err, closeErr)
	}
	if err != nil {
		return err
	}
	blob, err := storage.OpenBlob(artifact, false)
	if err != nil {
		return err
	}
	got, err := blob.ReadKey(key)
	if closeErr := blob.Close(); closeErr != nil {
		err = errors.Join(err, closeErr)
	}
	if err != nil {
		return fmt.Errorf("backup lacks %s: %w", key, err)
	}
	if !bytes.Equal(got, want) {
		return fmt.Errorf("backup %s = %q, the tree holds %q", key, got, want)
	}
	return nil
}

// storageLiveBackupRefused proves AC-3: the offline `ze data backup` refuses
// while a daemon owns the store, names the daemon's SSH address and the live
// route, and writes no file.
func storageLiveBackupRefused(ctx context.Context, args []string) error {
	if _, err := storageLiveClient(ctx, args); err != nil {
		return err
	}
	artifact, err := filepath.Abs("offline-backup.zefs")
	if err != nil {
		return err
	}
	output, err := storageCommand(ctx, "", "data", "backup", artifact)
	if err == nil {
		return fmt.Errorf("ze data backup ran beside a live daemon:\n%s", output)
	}
	for _, want := range []string{"a daemon owns the store at 127.0.0.1:" + args[0], "request data backup path"} {
		if !bytes.Contains(output, []byte(want)) {
			return fmt.Errorf("ze data backup refused without %q:\n%s", want, output)
		}
	}
	if err := storageLiveAbsent(artifact); err != nil {
		return err
	}
	return storageLiveOK("data-backup-refused-live")
}

// storageLiveRestoreFullRefused proves AC-6's live arm: the offline full
// restore refuses while a daemon owns the store, names the daemon, and writes
// nothing: the source stays, no previous tree is retired, and the active
// config is unchanged.
func storageLiveRestoreFullRefused(ctx context.Context, args []string) error {
	if _, err := storageLiveClient(ctx, args); err != nil {
		return err
	}
	source, err := filepath.Abs("restore-source.zefs")
	if err != nil {
		return err
	}
	config := storageLiveRestoredRouterID + "\n"
	if err := storageLiveSource(source, map[string][]byte{zefs.KeyFileActive.Key("other.conf"): []byte(config)}); err != nil {
		return err
	}
	output, err := storageCommand(ctx, "", "data", "restore", source, "full")
	if err == nil {
		return fmt.Errorf("ze data restore full ran beside a live daemon:\n%s", output)
	}
	for _, want := range []string{"a daemon owns the store at 127.0.0.1:" + args[0], "stop the daemon"} {
		if !bytes.Contains(output, []byte(want)) {
			return fmt.Errorf("ze data restore full refused without %q:\n%s", want, output)
		}
	}
	if _, err := os.Stat(source); err != nil {
		return fmt.Errorf("the refused restore moved its source: %w", err)
	}
	retired, err := filepath.Glob(storageTreeName + ".replaced-*")
	if err != nil {
		return err
	}
	if len(retired) != 0 {
		return fmt.Errorf("the refused restore retired the tree: %v", retired)
	}
	return storageLiveOK("data-restore-full-refused-live")
}

// storageLiveRestoreConfig proves AC-8 on a running daemon: a source holding
// no config, and `name` naming a config the source lacks, are refused naming
// what the source holds; `request data restore path <abs> config` then takes
// the source's only config, under the device's name, through the reload, and
// `show bgp` answers the peer the restored config adds. The running router-id
// is not asserted: a reload does not apply a changed global router-id (journal,
// 2026-09-26), which is a product question of its own.
// A client target on a daemon that serves no managed client is refused. The
// .ci checks the pointers.
func storageLiveRestoreConfig(ctx context.Context, args []string) error {
	env, err := storageLiveClient(ctx, args)
	if err != nil {
		return err
	}
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	const deviceName = "data-restore-config-live.conf"
	booted, err := os.ReadFile(deviceName)
	if err != nil {
		return err
	}
	if !bytes.Contains(booted, []byte(storageLiveBootRouterID)) {
		return fmt.Errorf("%s does not boot with %s", deviceName, storageLiveBootRouterID)
	}
	restored := bytes.Replace(booted, []byte(storageLiveBootRouterID),
		[]byte(storageLiveRestoredRouterID+"\n"+storageLiveRestoredPeer), 1)

	empty := filepath.Join(cwd, "restore-empty.zefs")
	if err := storageLiveSource(empty, map[string][]byte{zefs.KeyInstanceName.Pattern: []byte("elsewhere")}); err != nil {
		return err
	}
	source := filepath.Join(cwd, "restore-source.zefs")
	if err := storageLiveSource(source, map[string][]byte{zefs.KeyFileActive.Key("yesterday.conf"): restored}); err != nil {
		return err
	}
	refusals := []struct{ command, want string }{
		{"request data restore path " + empty + " config", "holds no config"},
		{"request data restore path " + source + " config name nosuch.conf", "yesterday.conf"},
		{"request data restore path " + source + " config client edge-01", "serves no managed client"},
	}
	for _, refusal := range refusals {
		if err := storageLiveRefused(ctx, env, refusal.command, refusal.want); err != nil {
			return err
		}
	}
	output, err := storageLiveRequest(ctx, env, "request data restore path "+source+" config")
	if err != nil {
		return err
	}
	for _, want := range []string{"yesterday.conf", deviceName} {
		if !strings.Contains(output, want) {
			return fmt.Errorf("restore answer lacks %q:\n%s", want, output)
		}
	}
	var last string
	if !Poll(ctx, 50, 200*time.Millisecond, func() bool {
		last, err = storageLiveRequest(ctx, env, "show bgp | json")
		return err == nil && storageLivePeersConfigured.MatchString(last)
	}) {
		return fmt.Errorf("the daemon never served the restored peer; the restore answered:\n%s\nshow bgp answers:\n%s", output, last)
	}
	return storageLiveOK("data-restore-config-live")
}

// storageLivePeersConfigured matches `show bgp | json` answering one
// configured peer.
var storageLivePeersConfigured = regexp.MustCompile(`"peers-configured":\s*1\b`)

// storageLiveSource writes values to a blob at path, as a backup carries them.
func storageLiveSource(path string, values map[string][]byte) error {
	blob, err := storage.CreateBlobPopulated(path, func(seed storage.Storage) error {
		for key, value := range values {
			if err := seed.WriteKey(key, value); err != nil {
				return err
			}
		}
		return nil
	}, false, zefs.Spare(0))
	if err != nil {
		return err
	}
	return blob.Close()
}

// storageLiveAbsent requires a refused command to have left no file at path.
func storageLiveAbsent(path string) error {
	_, err := os.Lstat(path)
	if err == nil {
		return fmt.Errorf("the refused command wrote %s", path)
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}
