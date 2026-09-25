// Design: docs/architecture/testing/ci-format.md -- tree storage functional scenarios

package fixture

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/pkg/plugin/sdk"
	"github.com/ze-software/ze/pkg/zefs"
)

// storageTreeName is the published live tree the storage scenarios inspect and
// damage, relative to the scenario's working directory.
const storageTreeName = "database"

// storageSeedUser is the operator every storage scenario seeds at init and logs
// in as over SSH.
const storageSeedUser = "admin"

// The two passes of the state observer: the first writes durable plugin state,
// the second, in a fresh daemon, reads it back and removes it.
const (
	observerModeWrite = "write"
	observerModeRead  = "read"
)

func init() {
	Register("storage/empty-tree", storageEmptyTree)
	Register("storage/tree", storageTreeScenario)
	Register("storage/probe", storageProbe)
	Register("storage/state-observer", storageStateObserver)
}

func storageEmptyTree(_ context.Context, _ []string) error {
	store, err := storage.Create(".")
	if err != nil {
		return err
	}
	return store.Close()
}

// storageCommand runs the actual daemon binary selected by the functional runner.
//
// The deadline is a share of the test's own budget, never a constant. At 25s it
// decided every case here while the .ci files declared `option=timeout:value=90s`,
// so the authored budget was inert and a command slowed by contention failed on a
// number nobody wrote.
func storageCommand(ctx context.Context, input string, args ...string) ([]byte, error) {
	deadline, cancel := context.WithTimeout(ctx, WaitBudget(75, 25*time.Second))
	defer cancel()
	command := exec.CommandContext(deadline, "ze", args...) //nolint:gosec // the fixture chooses the program and its arguments
	command.Stdin = strings.NewReader(input)
	return command.CombinedOutput()
}

func storageRequireCommand(ctx context.Context, input string, args ...string) ([]byte, error) {
	output, err := storageCommand(ctx, input, args...)
	if err != nil {
		return output, fmt.Errorf("ze %v: %w\n%s", args, err, output)
	}
	if bytes.Contains(output, []byte(observerFailure)) {
		return output, fmt.Errorf("observer refused scenario:\n%s", output)
	}
	return output, nil
}

func storageProbe(ctx context.Context, _ []string) error {
	anchor := env.Get("ze.plugin.ca.pem")
	if anchor == "" {
		return errors.New("plugin received no CA trust anchor")
	}
	fingerprint := fmt.Sprintf("%x", sha256.Sum256([]byte(anchor)))
	if err := os.WriteFile("ca-fingerprint", []byte(fingerprint), 0o600); err != nil {
		return err
	}
	return Observe(ctx, "storage-probe", sdk.Registration{}, func(context.Context, *sdk.Plugin) error {
		fmt.Fprintln(os.Stderr, "OK: storage probe authenticated")
		return nil
	})
}

const storageProbeConfig = `plugin {
 external storage-probe {
  run "le test fixture storage/probe"
  encoder json
 }
}
bgp { router-id 1.2.3.4; session { asn { local 65000; } } }
`

func storageTreeScenario(ctx context.Context, args []string) error {
	if len(args) != 1 {
		return errors.New("storage/tree requires one scenario name")
	}
	var err error
	switch args[0] {
	case "init-creates-tree":
		err = storageInitScenario(ctx)
	case "init-from-blob":
		err = storageInitFromBlob(ctx)
	case "start-auto-creates", "start-on-tree":
		err = storageStartScenario(ctx, args[0] == "start-on-tree")
	case "start-refuses-blob":
		err = storageRefusalScenario(ctx, storageRefusalBlob)
	case "start-refuses-loose-mode":
		err = storageRefusalScenario(ctx, storageRefusalLooseMode)
	case "start-refuses-unfinished-import":
		err = storageRefusalScenario(ctx, storageRefusalUnfinishedImport)
	case "doctor-without-store":
		err = storageDoctorScenario(ctx, false)
	case "doctor-refuses-loose-mode":
		err = storageDoctorLooseModeScenario(ctx)
	case "doctor-tree-corrupt-key":
		err = storageDoctorScenario(ctx, true)
	case "data-check-tree":
		err = storageDataScenario(ctx)
	case "stdin-ephemeral-authority":
		err = storageEphemeralScenario(ctx)
	case "explicit-file-restart":
		err = storageExplicitRestart(ctx)
	case "offline-file-without-store":
		err = storageOfflineScenario(ctx)
	case "statestore-on-tree":
		err = storageStateRestart(ctx)
	case "ospf-state-through-daemon":
		err = storageOSPFRestart(ctx)
	default:
		return fmt.Errorf("unknown storage scenario %q", args[0])
	}
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stdout, "OK: %s\n", args[0]) //nolint:errcheck // status output
	return nil
}

func storageInitScenario(ctx context.Context) error {
	if _, err := storageRequireCommand(ctx, "admin\nsecret123\n127.0.0.1\n2222\n\n", "init", "--web-cert", "127.0.0.1"); err != nil {
		return err
	}
	store, err := storage.OpenReadOnly(".")
	if err != nil {
		return err
	}
	for _, key := range []string{zefs.KeyLocalAdminUsername.Pattern, zefs.KeyLocalAdminPassword.Pattern, zefs.KeySSHDefault.Pattern, zefs.KeyWebCert.Pattern, zefs.KeyWebKey.Pattern} {
		if _, err := store.ReadKey(key); err != nil {
			_ = store.Close()
			return fmt.Errorf("init omitted %s: %w", key, err)
		}
	}
	if err := store.Close(); err != nil {
		return err
	}
	if err := filepath.WalkDir(storageTreeName, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		want := fs.FileMode(0o600)
		if entry.IsDir() {
			want = 0o700
		}
		if info.Mode().Perm() != want {
			return fmt.Errorf("init mode %s = %o, want %o", path, info.Mode().Perm(), want)
		}
		return nil
	}); err != nil {
		return err
	}
	output, err := storageCommand(ctx, "admin\nsecret123\n127.0.0.1\n2222\n\n", "init")
	if err == nil || !bytes.Contains(output, []byte("database already exists")) {
		return fmt.Errorf("second init did not refuse the tree: %w\n%s", err, output)
	}
	return nil
}

func storageStartScenario(ctx context.Context, existing bool) error {
	if existing {
		if err := storageEmptyTree(ctx, nil); err != nil {
			return err
		}
	}
	if err := os.WriteFile("router.conf", []byte(storageProbeConfig), 0o600); err != nil {
		return err
	}
	output, err := storageRequireCommand(ctx, "", "start", "router.conf")
	if err != nil {
		return err
	}
	if !bytes.Contains(output, []byte("OK: storage probe authenticated")) {
		return fmt.Errorf("daemon never authenticated probe:\n%s", output)
	}
	store, err := storage.OpenReadOnly(".")
	if err != nil {
		return err
	}
	defer store.Close() //nolint:errcheck // fixture cleanup
	active, err := storage.ReadActiveConfig(store, "router.conf")
	if err != nil {
		return err
	}
	if !bytes.Contains(active, []byte("1.2.3.4")) {
		return fmt.Errorf("active config lost source: %s", active)
	}
	_, err = store.ReadKey(zefs.KeyCACert.Pattern)
	return err
}

// storageInitFromBlob proves `ze init --from <blob>` publishes every blob key
// byte-equal into database/, retires the blob, refuses a live store without
// --force, refuses a running owner, and replaces under --force --yes.
func storageInitFromBlob(ctx context.Context) error {
	values := map[string][]byte{
		zefs.KeyLocalAdminUsername.Pattern: []byte(storageSeedUser),
		zefs.KeyLocalAdminPassword.Pattern: []byte("$2a$10$seedhash"),
		zefs.KeyInstanceName.Pattern:       []byte("seeded-router"),
		zefs.KeyFileActive.Key("ze.conf"):  []byte("bgp { router-id 10.0.0.1; }\n"),
	}
	blob, err := storage.CreateBlobPopulated("database.zefs", func(seed storage.Storage) error {
		for key, value := range values {
			if err := seed.WriteKey(key, value); err != nil {
				return err
			}
		}
		return nil
	}, false)
	if err != nil {
		return err
	}
	if err := blob.Close(); err != nil {
		return err
	}
	if _, err := storageRequireCommand(ctx, "", "init", "--from", "database.zefs"); err != nil {
		return err
	}
	_, err = os.Lstat("database.zefs")
	if err == nil {
		return errors.New("import left the blob in place")
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("stat database.zefs: %w", err)
	}
	archives, err := storageArchives("database.zefs")
	if err != nil {
		return err
	}
	if len(archives) != 1 {
		return fmt.Errorf("import retired %d archives, want 1: %v", len(archives), archives)
	}
	_, err = os.Lstat("database.zefs.lock")
	if err == nil {
		return errors.New("import left the retired source's lock behind")
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("stat database.zefs.lock: %w", err)
	}
	_, err = os.Lstat("database.import-intent")
	if err == nil {
		return errors.New("import left its intent behind")
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("stat database.import-intent: %w", err)
	}
	if err := storageTreeEqualsBlob(archives[0], values); err != nil {
		return err
	}
	output, err := storageCommand(ctx, "", "init", "--from", archives[0])
	if err == nil {
		return fmt.Errorf("import did not refuse the live store:\n%s", output)
	}
	if !bytes.Contains(output, []byte("database already exists")) {
		return fmt.Errorf("import refused the live store without naming it: %w\n%s", err, output)
	}
	output, err = storageCommand(ctx, "", "init", "--from", archives[0], "--seed")
	if err == nil {
		return fmt.Errorf("--from with --seed was not refused:\n%s", output)
	}
	if !bytes.Contains(output, []byte("--seed")) {
		return fmt.Errorf("--from with --seed was not refused by name: %w\n%s", err, output)
	}
	owner, err := storage.Open(".")
	if err != nil {
		return err
	}
	output, err = storageCommand(ctx, "", "init", "--from", archives[0], "--force", "--yes")
	if err == nil {
		return errors.Join(fmt.Errorf("import ran beside a live owner:\n%s", output), owner.Close())
	}
	if !bytes.Contains(output, []byte("owned by another process")) {
		return errors.Join(fmt.Errorf("import beside a live owner was refused without naming it: %w\n%s", err, output), owner.Close())
	}
	if err := owner.Close(); err != nil {
		return err
	}
	if _, err := storageRequireCommand(ctx, "", "init", "--from", archives[0], "--force", "--yes"); err != nil {
		return err
	}
	replaced, err := filepath.Glob("database.replaced-*")
	if err != nil {
		return err
	}
	if len(replaced) != 1 {
		return fmt.Errorf("--force retired %d trees, want 1: %v", len(replaced), replaced)
	}
	archives, err = storageArchives("database.zefs")
	if err != nil {
		return err
	}
	if len(archives) != 1 {
		return fmt.Errorf("second import retired %d archives, want 1: %v", len(archives), archives)
	}
	return storageTreeEqualsBlob(archives[0], values)
}

// storageArchives lists the retired copies of a blob. Every blob artifact keeps
// its lock file beside it, so the archives' locks are not archives.
func storageArchives(blob string) ([]string, error) {
	names, err := filepath.Glob(blob + ".replaced-*")
	if err != nil {
		return nil, err
	}
	archives := names[:0]
	for _, name := range names {
		if strings.HasSuffix(name, ".lock") {
			continue
		}
		archives = append(archives, name)
	}
	return archives, nil
}

// storageTreeEqualsBlob compares the published tree with the retired archive
// key by key, and with the values the scenario seeded.
func storageTreeEqualsBlob(archive string, values map[string][]byte) error {
	tree, err := storage.OpenReadOnly(".")
	if err != nil {
		return err
	}
	defer tree.Close() //nolint:errcheck // read-only comparison handle.
	source, err := storage.OpenBlob(archive, false)
	if err != nil {
		return err
	}
	defer source.Close() //nolint:errcheck // read-only comparison handle.
	keys, err := tree.ListKeys("")
	if err != nil {
		return err
	}
	if len(keys) != len(values) {
		return fmt.Errorf("tree holds %d keys, seeded %d: %v", len(keys), len(values), keys)
	}
	for key, want := range values {
		got, err := tree.ReadKey(key)
		if err != nil {
			return fmt.Errorf("tree lost %s: %w", key, err)
		}
		if !bytes.Equal(got, want) {
			return fmt.Errorf("tree %s = %q, want %q", key, got, want)
		}
		archived, err := source.ReadKey(key)
		if err != nil {
			return fmt.Errorf("archive lost %s: %w", key, err)
		}
		if !bytes.Equal(archived, want) {
			return fmt.Errorf("archive %s = %q, want %q", key, archived, want)
		}
	}
	return nil
}

func storageRefusalScenario(ctx context.Context, prepare func(context.Context) ([]string, error)) error {
	if err := os.WriteFile("router.conf", []byte(storageProbeConfig), 0o600); err != nil {
		return err
	}
	// AC-4: the refusal names the path, the mode found and the repair command.
	needles, err := prepare(ctx)
	if err != nil {
		return err
	}
	_, treeErr := os.Stat(storageTreeName)
	treeAbsent := errors.Is(treeErr, fs.ErrNotExist)
	output, err := storageCommand(ctx, "", "start", "router.conf")
	if err == nil {
		return fmt.Errorf("start did not refuse invalid storage:\n%s", output)
	}
	for _, needle := range needles {
		if !bytes.Contains(output, []byte(needle)) {
			return fmt.Errorf("start refusal omitted %q:\n%s", needle, output)
		}
	}
	if _, err := os.Stat("ca-fingerprint"); !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("refused daemon ran probe: %w", err)
	}
	if treeAbsent {
		if _, err := os.Stat(storageTreeName); !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("refusal created a tree: %w", err)
		}
	}
	return nil
}

// storageRefusalBlob leaves a legacy blob and no tree: start names the blob
// and the import command.
func storageRefusalBlob(context.Context) ([]string, error) {
	blob, err := storage.CreateBlob("database.zefs")
	if err != nil {
		return nil, err
	}
	return []string{"ze init --from"}, blob.Close()
}

// storageRefusalLooseMode weakens the tree root to 0755: start names the path,
// the mode found and the chmod repair.
func storageRefusalLooseMode(ctx context.Context) ([]string, error) {
	if err := storageEmptyTree(ctx, nil); err != nil {
		return nil, err
	}
	if err := os.Chmod(storageTreeName, 0o755); err != nil { //nolint:gosec // the fixture weakens the mode so doctor reports it
		return nil, err
	}
	return []string{storageTreeName, "mode 0755", "chmod 0700"}, nil
}

// storageRefusalUnfinishedImport installs the state of an import that crashed
// after moving the old tree to database.replaced-* and before publishing its
// stage: an intent beside no tree. start MUST refuse and name the intent file
// and the import to finish, and MUST NOT auto-create an empty tree over the
// operator's store.
func storageRefusalUnfinishedImport(context.Context) ([]string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	source := filepath.Join(cwd, "database.zefs")
	// The layout mirrors storage's importIntent; the identity fields are never
	// consulted here because no tree exists to compare them with.
	intent := struct {
		Source  string `json:"source"`
		Digest  string `json:"digest"`
		Stage   string `json:"stage"`
		Archive string `json:"archive"`
		Device  uint64 `json:"device"`
		Inode   uint64 `json:"inode"`
	}{
		Source:  source,
		Digest:  strings.Repeat("0", 64),
		Stage:   "database.import-tmp-crashed",
		Archive: source + ".replaced-20260101T000000.000000000",
		Device:  1,
		Inode:   1,
	}
	encoded, err := json.Marshal(intent)
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile("database.import-intent", encoded, 0o600); err != nil {
		return nil, err
	}
	if err := os.Mkdir(storageTreeName+".replaced-20260101T000000.000000000", 0o700); err != nil {
		return nil, err
	}
	return []string{filepath.Join(cwd, "database.import-intent"), "ze init --from " + source}, nil
}

// storageDoctorLooseModeScenario proves AC-23 through `ze doctor`: a tree root
// at 0755 is an ERROR (the run exits non-zero) that names the path, the mode
// found and the repair command, and nothing is auto-created or repaired.
func storageDoctorLooseModeScenario(ctx context.Context) error {
	if err := os.WriteFile("router.conf", []byte(storageProbeConfig), 0o600); err != nil {
		return err
	}
	if err := storageEmptyTree(ctx, nil); err != nil {
		return err
	}
	if err := os.Chmod(storageTreeName, 0o755); err != nil { //nolint:gosec // the fixture weakens the mode so doctor reports it
		return err
	}
	output, err := storageCommand(ctx, "", "doctor", "--json", "router.conf")
	if err == nil {
		return fmt.Errorf("doctor accepted a loose-mode tree:\n%s", output)
	}
	for _, needle := range []string{"doctor-store-permissions", storageTreeName, "mode 0755", "chmod 0700"} {
		if !bytes.Contains(output, []byte(needle)) {
			return fmt.Errorf("doctor omitted %q:\n%s", needle, output)
		}
	}
	info, err := os.Stat(storageTreeName)
	if err != nil {
		return err
	}
	if info.Mode().Perm() != 0o755 {
		return fmt.Errorf("doctor changed the tree mode to %o; a diagnosis repairs nothing", info.Mode().Perm())
	}
	return nil
}

func storageDoctorScenario(ctx context.Context, corrupt bool) error {
	needle := "doctor-storage-unavailable"
	if corrupt {
		if err := storageSeedCorrupt(); err != nil {
			return err
		}
		needle = "doctor-store-integrity"
	}
	output, _ := storageCommand(ctx, "", "doctor")
	if !bytes.Contains(output, []byte(needle)) {
		return fmt.Errorf("doctor omitted %s:\n%s", needle, output)
	}
	if corrupt {
		if !bytes.Contains(output, []byte("meta/fixture/bad")) {
			return fmt.Errorf("doctor did not identify corrupt key:\n%s", output)
		}
		return nil
	}
	if !bytes.Contains(output, []byte("ze init")) {
		return fmt.Errorf("doctor did not name initialization remedy:\n%s", output)
	}
	if _, err := os.Stat(storageTreeName); !errors.Is(err, fs.ErrNotExist) {
		return errors.Join(errors.New("doctor created a store"), err)
	}
	return nil
}

func storageSeedCorrupt() error {
	store, err := storage.Create(".")
	if err != nil {
		return err
	}
	for key, value := range map[string]string{"meta/fixture/good": "survives", "meta/fixture/bad": "damaged"} {
		if err := store.WriteKey(key, []byte(value)); err != nil {
			_ = store.Close()
			return err
		}
	}
	if err := store.Close(); err != nil {
		return err
	}
	path := filepath.Join(storageTreeName, "meta", "fixture", "bad")
	frame, err := os.ReadFile(path) //nolint:gosec // the path is the fixture's own scratch file
	if err != nil {
		return err
	}
	index := bytes.Index(frame, []byte("damaged"))
	if index < 0 {
		return errors.New("frame does not contain seed data")
	}
	frame[index] ^= 1
	return os.WriteFile(path, frame, 0o600)
}

func storageDataScenario(ctx context.Context) error {
	if err := storageSeedCorrupt(); err != nil {
		return err
	}
	output, err := storageCommand(ctx, "", "data", "check", "--path", storageTreeName)
	if err == nil || !bytes.Contains(output, []byte("meta/fixture/bad")) {
		return errors.Join(fmt.Errorf("data check accepted corruption\n%s", output), err)
	}
	output, err = storageCommand(ctx, "", "data", "repair", "--path", storageTreeName, "--output", "repaired")
	if exitErr, ok := errors.AsType[*exec.ExitError](err); !ok || exitErr.ExitCode() != 1 {
		return errors.Join(fmt.Errorf("repair did not report partial recovery\n%s", output), err)
	}
	output, err = storageRequireCommand(ctx, "", "data", "cat", "meta/fixture/good", "--path", "repaired")
	if err != nil {
		return err
	}
	if !bytes.Contains(output, []byte("survives")) {
		return fmt.Errorf("repair lost good value: %s", output)
	}
	if output, err = storageCommand(ctx, "", "data", "cat", "meta/fixture/bad", "--path", "repaired"); err == nil {
		return fmt.Errorf("repair retained corrupt key: %s", output)
	}
	return nil
}

func storageEphemeralScenario(ctx context.Context) error {
	var previous []byte
	for range 2 {
		if _, err := storageRequireCommand(ctx, storageProbeConfig, "-"); err != nil {
			return err
		}
		fingerprint, err := os.ReadFile("ca-fingerprint")
		if err != nil {
			return err
		}
		if bytes.Equal(previous, fingerprint) {
			return errors.New("storeless restart reused its CA")
		}
		previous = fingerprint
		for _, path := range []string{storageTreeName, "database.zefs", "meta/ca/cert", "meta/ca/key"} {
			if _, err := os.Stat(path); !errors.Is(err, fs.ErrNotExist) {
				return errors.Join(fmt.Errorf("storeless startup persisted %s", path), err)
			}
		}
	}
	store, err := storage.Create(".")
	if err != nil {
		return err
	}
	if err := store.WriteKey(zefs.KeyCACert.Pattern, []byte("not a certificate")); err != nil {
		_ = store.Close()
		return err
	}
	if err := store.Close(); err != nil {
		return err
	}
	if err := os.Remove("ca-fingerprint"); err != nil {
		return err
	}
	output, err := storageCommand(ctx, storageProbeConfig, "-")
	if err == nil {
		return fmt.Errorf("stdin startup silently replaced corrupt persistent CA:\n%s", output)
	}
	if _, err := os.Stat("ca-fingerprint"); !errors.Is(err, fs.ErrNotExist) {
		return errors.Join(errors.New("plugin started despite corrupt persistent CA"), err)
	}
	store, err = storage.OpenReadOnly(".")
	if err != nil {
		return err
	}
	defer store.Close() //nolint:errcheck // fixture cleanup
	certificate, err := store.ReadKey(zefs.KeyCACert.Pattern)
	if err != nil {
		return err
	}
	if !bytes.Equal(certificate, []byte("not a certificate")) {
		return errors.New("failed startup changed the persistent CA")
	}
	return nil
}

func storageExplicitRestart(ctx context.Context) error {
	if err := storageStartScenario(ctx, false); err != nil {
		return err
	}
	changed := strings.ReplaceAll(storageProbeConfig, "1.2.3.4", "5.6.7.8")
	if err := os.WriteFile("router.conf", []byte(changed), 0o600); err != nil {
		return err
	}
	if _, err := storageRequireCommand(ctx, "", "start", "router.conf"); err != nil {
		return err
	}
	store, err := storage.OpenReadOnly(".")
	if err != nil {
		return err
	}
	defer store.Close() //nolint:errcheck // fixture cleanup
	active, err := storage.ReadActiveConfig(store, "router.conf")
	if err != nil {
		return err
	}
	if !bytes.Contains(active, []byte("5.6.7.8")) || bytes.Contains(active, []byte("1.2.3.4")) {
		return fmt.Errorf("restart hid offline source edit: %s", active)
	}
	return nil
}

func storageOfflineScenario(ctx context.Context) error {
	if err := os.WriteFile("router.conf", []byte("bgp { router-id 1.2.3.4; }\n"), 0o600); err != nil {
		return err
	}
	if _, err := storageRequireCommand(ctx, "", "config", "set", "router.conf", "bgp", "router-id", "5.6.7.8"); err != nil {
		return err
	}
	content, err := os.ReadFile("router.conf")
	if err != nil {
		return err
	}
	if !bytes.Contains(content, []byte("5.6.7.8")) {
		return fmt.Errorf("offline set did not write source: %s", content)
	}
	for _, args := range [][]string{{"config", "history", "router.conf"}, {"config", "rollback", "1", "router.conf"}} {
		output, err := storageCommand(ctx, "", args...)
		if err == nil || !bytes.Contains(output, []byte("ze init")) {
			return errors.Join(fmt.Errorf("%v did not refuse unavailable history\n%s", args, output), err)
		}
	}
	if _, err := os.Stat(storageTreeName); !errors.Is(err, fs.ErrNotExist) {
		return errors.Join(errors.New("offline edit created a store"), err)
	}
	return nil
}

func storageStateObserver(ctx context.Context, args []string) error {
	if len(args) != 1 || (args[0] != observerModeWrite && args[0] != observerModeRead) {
		return errors.New("state-observer requires write or read")
	}
	return Observe(ctx, "ospf", sdk.Registration{}, func(ctx context.Context, plugin *sdk.Plugin) error {
		const key = "meta/ospf/gr-fact-storage-fixture"
		if args[0] == observerModeWrite {
			if _, found, err := plugin.StateGet(ctx, key); err != nil || found {
				return errors.Join(fmt.Errorf("new state key: found=%v", found), err)
			}
			if err := plugin.StatePut(ctx, key, []byte("durable fixture state")); err != nil {
				return err
			}
		}
		value, found, err := plugin.StateGet(ctx, key)
		if err != nil || !found || string(value) != "durable fixture state" {
			return errors.Join(fmt.Errorf("persisted state: found=%v value=%q", found, value), err)
		}
		if args[0] == observerModeRead {
			if err := plugin.StateRemove(ctx, key); err != nil {
				return err
			}
			if _, found, err := plugin.StateGet(ctx, key); err != nil || found {
				return errors.Join(fmt.Errorf("removed state still present: found=%v", found), err)
			}
		}
		return nil
	})
}

func storageStateRestart(ctx context.Context) error {
	for _, mode := range []string{observerModeWrite, observerModeRead} {
		config := fmt.Sprintf("plugin { external ospf { run \"le test fixture storage/state-observer %s\"; encoder json; } }\n", mode)
		if err := os.WriteFile("state.conf", []byte(config), 0o600); err != nil {
			return err
		}
		if _, err := storageRequireCommand(ctx, "", "start", "state.conf"); err != nil {
			return err
		}
	}
	return nil
}

func storageOSPFRestart(ctx context.Context) error {
	config := storageProbeConfig + "\nospf { router-id 10.0.0.1; areas { area 0.0.0.0 { } } }\n"
	var previous uint32
	for range 2 {
		if err := os.WriteFile("ospf.conf", []byte(config), 0o600); err != nil {
			return err
		}
		if _, err := storageRequireCommand(ctx, "", "start", "ospf.conf"); err != nil {
			return err
		}
		store, err := storage.OpenReadOnly(".")
		if err != nil {
			return err
		}
		value, readErr := store.ReadKey("meta/ospf/auth/boot-count")
		closeErr := store.Close()
		if readErr != nil {
			return readErr
		}
		if closeErr != nil {
			return closeErr
		}
		if len(value) != 4 {
			return fmt.Errorf("OSPF boot counter has %d bytes, want 4", len(value))
		}
		current := binary.BigEndian.Uint32(value)
		if current <= previous {
			return fmt.Errorf("OSPF restart did not increment persisted counter: %d <= %d", current, previous)
		}
		previous = current
	}
	return nil
}
