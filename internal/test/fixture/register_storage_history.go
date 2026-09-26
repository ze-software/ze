// Design: docs/architecture/testing/ci-format.md -- content-addressed history functional scenarios
// Related: storage_tree_fixture.go -- storageCommand, storageRequireCommand and the tree scenarios

package fixture

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/sdk"
	"github.com/ze-software/ze/pkg/zefs"
)

// historyObserverPrefix opens the one stderr line the history observer
// prints for the bgp section the daemon configured it with.
const historyObserverPrefix = "history-observer bgp: "

// historyConfig is a config whose router ID identifies it at runtime: the
// observer plugin prints the bgp section the daemon hands it.
const historyConfig = `plugin {
 external history-observer {
  run "le test fixture storage/history-observer"
  encoder json
 }
}
bgp { router-id ROUTER-ID; session { asn { local 65000; } } }
`

func init() {
	Register("storage/history-rollback-object", historyScenario("history-rollback-object", storageHistoryRollback))
	Register("storage/history-restore-config-object", historyScenario("history-restore-config-object", storageHistoryRestoreConfig))
	Register("storage/history-repaired-store-start", historyScenario("history-repaired-store-start", storageHistoryRepairedStart))
	Register("storage/history-observer", storageHistoryObserver)
}

// historyScenario wraps a scenario taking no arguments and prints
// "OK: <name>" when it passes, the line each .ci file expects.
func historyScenario(name string, scenario func(context.Context) error) Driver {
	return func(ctx context.Context, args []string) error {
		if len(args) != 0 {
			return errors.New(name + " takes no arguments")
		}
		if err := scenario(ctx); err != nil {
			return err
		}
		var status textbuf.Buffer
		return status.Str("OK: ").Str(name).Byte('\n').StdOut()
	}
}

// historyConfigFor is historyConfig carrying routerID.
func historyConfigFor(routerID string) []byte {
	return []byte(strings.Replace(historyConfig, "ROUTER-ID", routerID, 1))
}

// storageHistoryObserver prints the bgp section it is configured with, so
// the scenario can tell which config the daemon really runs, then lets the
// daemon shut down.
func storageHistoryObserver(ctx context.Context, _ []string) error {
	setup := func(plugin *sdk.Plugin) error {
		plugin.OnConfigure(func(sections []sdk.ConfigSection) error {
			for _, section := range sections {
				if section.Root != namespaceBGP {
					continue
				}
				var line textbuf.Buffer
				if err := line.Str(historyObserverPrefix).Join(strings.Fields(section.Data), " ").Byte('\n').StdErr(); err != nil {
					return err
				}
			}
			return nil
		})
		return nil
	}
	registration := sdk.Registration{WantsConfig: []string{namespaceBGP}}
	return observeConfigured(ctx, "history-observer", registration, setup, func(context.Context, *sdk.Plugin) error { return nil })
}

func historyDigest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// storageHistoryRollback drives `ze config rollback` over content-addressed
// history: the rollback reads the version's bytes through entry -> object,
// and a version whose object holds bytes of another hash is refused naming
// that hash rather than restored.
func storageHistoryRollback(ctx context.Context) error {
	older := historyConfigFor("10.0.0.1")
	newer := historyConfigFor("10.0.0.2")
	store, err := storage.Create(".")
	if err != nil {
		return err
	}
	at := time.Date(2026, 9, 26, 10, 0, 0, 0, time.Local)
	err = errors.Join(
		store.WriteVersion("router.conf", older, at),
		store.WriteVersion("router.conf", newer, at.Add(time.Hour)),
	)
	if err := errors.Join(err, store.Close()); err != nil {
		return err
	}
	if err := os.WriteFile("router.conf", historyConfigFor("10.0.0.3"), 0o600); err != nil {
		return err
	}
	output, err := storageRequireCommand(ctx, "", "config", "rollback", "1", "router.conf")
	if err != nil {
		return err
	}
	if !bytes.Contains(output, []byte("Rolled back to revision 1")) {
		return fmt.Errorf("rollback did not report revision 1\n%s", output)
	}
	restored, err := os.ReadFile("router.conf")
	if err != nil {
		return err
	}
	if !bytes.Contains(restored, []byte("10.0.0.2")) {
		return fmt.Errorf("rollback 1 did not restore the newer version's bytes: %s", restored)
	}

	// Replace the older version's object with bytes of another hash.
	store, err = storage.Open(".")
	if err != nil {
		return err
	}
	olderDigest := historyDigest(older)
	err = store.WriteKey(zefs.KeyObject.Key(olderDigest), []byte("not the older config\n"))
	if err := errors.Join(err, store.Close()); err != nil {
		return err
	}
	before, err := os.ReadFile("router.conf")
	if err != nil {
		return err
	}
	output, err = storageCommand(ctx, "", "config", "rollback", "2", "router.conf")
	if err == nil {
		return fmt.Errorf("rollback restored a version whose object does not hash to its name\n%s", output)
	}
	if !bytes.Contains(output, []byte(olderDigest)) {
		return fmt.Errorf("rollback refusal does not name the hash %s\n%s", olderDigest, output)
	}
	after, err := os.ReadFile("router.conf")
	if err != nil {
		return err
	}
	if !bytes.Equal(before, after) {
		return fmt.Errorf("refused rollback changed the config: %s", after)
	}
	return nil
}

// storageHistoryRestoreConfig drives `ze data restore <backup> config` over a
// backup whose active version is an entry and an object: the restore reads
// the bytes through the entry and commits them, and a backup whose object is
// absent is refused naming the hash, before anything is written.
func storageHistoryRestoreConfig(ctx context.Context) error {
	device := "ze.conf"
	if err := storageWriteTree(".", map[string][]byte{"file/active/" + device: []byte("old config\n")}); err != nil {
		return err
	}
	yesterday := []byte("bgp { router-id 10.0.0.7; session { asn { local 65000; } } }\n")
	stamp := storage.FormatVersionStamp(time.Date(2026, 9, 25, 10, 0, 0, 0, time.Local))
	digest := historyDigest(yesterday)
	backup := map[string][]byte{
		"file/active/" + device:          yesterday,
		"file/" + stamp + "/" + device:   []byte("sha256:" + digest),
		zefs.KeyObject.Key(digest):       yesterday,
		zefs.KeyConfigActive.Key(device): []byte(stamp),
	}
	if err := storageWriteBlob("backup.zefs", backup); err != nil {
		return err
	}
	delete(backup, zefs.KeyObject.Key(digest))
	if err := storageWriteBlob("broken.zefs", backup); err != nil {
		return err
	}

	output, err := storageCommand(ctx, "", "data", "--path", storageTreeName, "restore", "broken.zefs", "config")
	if err == nil {
		return fmt.Errorf("restore accepted a backup whose object is absent\n%s", output)
	}
	if !bytes.Contains(output, []byte(digest)) {
		return fmt.Errorf("restore refusal does not name the hash %s\n%s", digest, output)
	}
	if err := storageActiveEquals(device, []byte("old config\n"), false); err != nil {
		return fmt.Errorf("refused restore wrote the device: %w", err)
	}

	output, err = storageRequireCommand(ctx, "", "data", "--path", storageTreeName, "restore", "backup.zefs", "config")
	if err != nil {
		return err
	}
	if err := storageRequireOutput(output, "committed as "+device); err != nil {
		return err
	}
	return storageActiveEquals(device, yesterday, true)
}

// storageActiveEquals reads the device's active config: through the active
// pointer when versioned is set, else the direct mirror a pointer-free store
// holds.
func storageActiveEquals(device string, want []byte, versioned bool) error {
	store, err := storage.OpenTree(storageTreeName, false)
	if err != nil {
		return err
	}
	defer store.Close() //nolint:errcheck // read-only comparison
	var got []byte
	if versioned {
		got, err = storage.ReadActiveConfig(store, device)
	} else {
		got, err = store.ReadKey("file/active/" + device)
	}
	if err != nil {
		return err
	}
	if !bytes.Equal(got, want) {
		return fmt.Errorf("active %s = %q, want %q", device, got, want)
	}
	return nil
}

// storageHistoryRepairedStart is the spec's repaired-output scenario through
// the binary: history R, S, C with rollback=R, active=S, candidate=C; the
// objects of S and C removed; `ze data repair` into a fresh config
// directory; bare `ze start` refused naming the active pointer and S; then
// `ze start <file>` serving F, logging the rebuild, clearing C, and keeping R.
func storageHistoryRepairedStart(ctx context.Context) error {
	const name = "ze.conf"
	configR := historyConfigFor("10.0.0.1")
	configS := historyConfigFor("10.0.0.2")
	configC := historyConfigFor("10.0.0.3")
	configF := historyConfigFor("10.0.0.9")

	store, err := storage.Create("source")
	if err != nil {
		return err
	}
	at := time.Date(2026, 9, 26, 10, 0, 0, 0, time.Local)
	var stamps [3]string
	for index, config := range [][]byte{configR, configS, configC} {
		stamps[index], err = storage.WriteCandidateVersion(store, name, config, at.Add(time.Duration(index)*time.Hour))
		if err == nil && index < 2 {
			err = storage.PromoteCandidate(store, name)
		}
		if err != nil {
			return errors.Join(err, store.Close())
		}
	}
	stampR, stampS, stampC := stamps[0], stamps[1], stamps[2]
	err = errors.Join(store.RemoveKey(zefs.KeyObject.Key(historyDigest(configS))), store.RemoveKey(zefs.KeyObject.Key(historyDigest(configC))))
	if err := errors.Join(err, store.Close()); err != nil {
		return err
	}

	// The repair output is the storage tree of a fresh config directory.
	if err := os.Mkdir("config", 0o700); err != nil {
		return err
	}
	repaired := filepath.Join("config", storageTreeName)
	output, err := storageCommand(ctx, "", "data", "repair", "--path", filepath.Join("source", storageTreeName), "--output", repaired)
	if exitErr, ok := errors.AsType[*exec.ExitError](err); !ok || exitErr.ExitCode() != 1 {
		return errors.Join(fmt.Errorf("repair did not report the dropped entries\n%s", output), err)
	}
	for _, want := range []string{
		"dropped: dangling-entry: file/" + stampS + "/" + name,
		"dropped: dangling-entry: file/" + stampC + "/" + name,
		"error: dangling-pointer: " + zefs.KeyConfigActive.Key(name),
		"error: dangling-pointer: " + zefs.KeyConfigCandidate.Key(name),
	} {
		if !bytes.Contains(output, []byte(want)) {
			return fmt.Errorf("repair report does not name %q\n%s", want, output)
		}
	}
	if err := storageHistoryPointers(repaired, name, stampS, stampC, stampR); err != nil {
		return err
	}

	configDir, err := filepath.Abs("config")
	if err != nil {
		return err
	}
	output, err = storageCommandEnv(ctx, []string{"ZE_CONFIG_DIR=" + configDir}, "start")
	if exitErr, ok := errors.AsType[*exec.ExitError](err); !ok || exitErr.ExitCode() != 1 {
		return errors.Join(fmt.Errorf("stored-source start did not exit 1\n%s", output), err)
	}
	for _, want := range []string{zefs.KeyConfigActive.Key(name), stampS} {
		if !bytes.Contains(output, []byte(want)) {
			return fmt.Errorf("stored-source refusal does not name %q\n%s", want, output)
		}
	}
	if bytes.Contains(output, []byte(historyObserverPrefix)) {
		return fmt.Errorf("stored-source start served a config\n%s", output)
	}
	if err := storageHistoryPointers(repaired, name, stampS, stampC, stampR); err != nil {
		return err
	}

	explicit := filepath.Join(configDir, name)
	if err := os.WriteFile(explicit, configF, 0o600); err != nil {
		return err
	}
	output, err = storageRequireCommand(ctx, "", "start", explicit)
	if err != nil {
		return err
	}
	served := ""
	for line := range strings.Lines(string(output)) {
		if strings.Contains(line, historyObserverPrefix) {
			served = line
		}
	}
	if !strings.Contains(served, "10.0.0.9") {
		return fmt.Errorf("explicit start did not serve F\n%s", output)
	}
	for _, want := range []string{"rebuilt active config from explicit file", zefs.KeyConfigActive.Key(name), stampS} {
		if !bytes.Contains(output, []byte(want)) {
			return fmt.Errorf("explicit start did not log the rebuild naming %q\n%s", want, output)
		}
	}
	return storageHistoryRecovered(repaired, explicit, name, stampR, stampS, stampC, configR, configF)
}

// storageCommandEnv runs the daemon binary with extra environment entries.
func storageCommandEnv(ctx context.Context, extra []string, args ...string) ([]byte, error) {
	deadline, cancel := context.WithTimeout(ctx, WaitBudget(75, 25*time.Second))
	defer cancel()
	command := exec.CommandContext(deadline, "ze", args...) //nolint:gosec // the fixture chooses the program and its arguments
	command.Env = append(os.Environ(), extra...)
	return command.CombinedOutput()
}

// storageHistoryPointers asserts the active, candidate and rollback pointers
// of the tree at path.
func storageHistoryPointers(path, name, active, candidate, rollback string) error {
	store, err := storage.OpenTree(path, false)
	if err != nil {
		return err
	}
	defer store.Close() //nolint:errcheck // read-only comparison
	pointers := []struct{ key, want string }{
		{zefs.KeyConfigActive.Key(name), active},
		{zefs.KeyConfigCandidate.Key(name), candidate},
		{zefs.KeyConfigRollback.Key(name), rollback},
	}
	for _, pointer := range pointers {
		value, err := store.ReadKey(pointer.key)
		if err != nil {
			return fmt.Errorf("%s: %w", pointer.key, err)
		}
		if strings.TrimSpace(string(value)) != pointer.want {
			return fmt.Errorf("%s = %q, want %s", pointer.key, value, pointer.want)
		}
	}
	return nil
}

// storageHistoryRecovered checks the tree after the explicit-file start:
// active resolves to exactly F, candidate is gone, rollback still names R and
// R's raw entry and object still hold R's digest and bytes, S and C stay
// dropped, and the loose file is still F.
func storageHistoryRecovered(path, explicit, name, stampR, stampS, stampC string, configR, configF []byte) error {
	store, err := storage.OpenTree(path, false)
	if err != nil {
		return err
	}
	var problems []error
	active, err := storage.ReadActiveConfig(store, name)
	if err != nil {
		problems = append(problems, fmt.Errorf("read active: %w", err))
	} else if !bytes.Equal(active, configF) {
		problems = append(problems, fmt.Errorf("active = %q; want F", active))
	}
	if _, err := store.ReadKey(zefs.KeyConfigCandidate.Key(name)); err == nil {
		problems = append(problems, errors.New("the stale candidate pointer survived startup"))
	}
	rollback, err := store.ReadKey(zefs.KeyConfigRollback.Key(name))
	if err != nil {
		problems = append(problems, fmt.Errorf("read rollback pointer: %w", err))
	} else if strings.TrimSpace(string(rollback)) != stampR {
		problems = append(problems, fmt.Errorf("rollback = %q; want %s", rollback, stampR))
	}
	digest := historyDigest(configR)
	entry, err := store.ReadKey("file/" + stampR + "/" + name)
	if err != nil {
		problems = append(problems, fmt.Errorf("read rollback entry: %w", err))
	} else if string(entry) != "sha256:"+digest {
		problems = append(problems, fmt.Errorf("rollback entry = %q; want sha256:%s", entry, digest))
	}
	object, err := store.ReadKey(zefs.KeyObject.Key(digest))
	if err != nil {
		problems = append(problems, fmt.Errorf("read rollback object: %w", err))
	} else if !bytes.Equal(object, configR) {
		problems = append(problems, fmt.Errorf("rollback object = %q; want R's bytes", object))
	}
	for _, dropped := range []string{stampS, stampC} {
		if _, err := store.ReadKey("file/" + dropped + "/" + name); err == nil {
			problems = append(problems, fmt.Errorf("dropped entry %s is back", dropped))
		}
	}
	if err := store.Close(); err != nil {
		problems = append(problems, err)
	}
	disk, err := os.ReadFile(explicit) //nolint:gosec // the fixture wrote this path
	if err != nil {
		problems = append(problems, fmt.Errorf("read explicit file: %w", err))
	} else if !bytes.Equal(disk, configF) {
		problems = append(problems, fmt.Errorf("explicit file = %q; want F", disk))
	}
	return errors.Join(problems...)
}
