// Design: docs/architecture/testing/ci-format.md -- tree storage functional scenarios

package fixture

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
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
func storageCommand(ctx context.Context, input string, args ...string) ([]byte, error) {
	deadline, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()
	command := exec.CommandContext(deadline, "ze", args...)
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
  run "ze-test fixture storage/probe"
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
	case "start-auto-creates", "start-on-tree":
		err = storageStartScenario(ctx, args[0] == "start-on-tree")
	case "start-refuses-blob":
		err = storageRefusalScenario(ctx, false)
	case "start-refuses-loose-mode":
		err = storageRefusalScenario(ctx, true)
	case "doctor-without-store":
		err = storageDoctorScenario(ctx, false)
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
	fmt.Fprintf(os.Stdout, "OK: %s\n", args[0])
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
	if err := filepath.WalkDir("database", func(path string, entry fs.DirEntry, walkErr error) error {
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
		return fmt.Errorf("second init did not refuse the tree: %v\n%s", err, output)
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

func storageRefusalScenario(ctx context.Context, looseMode bool) error {
	if err := os.WriteFile("router.conf", []byte(storageProbeConfig), 0o600); err != nil {
		return err
	}
	needle := "ze init from"
	if looseMode {
		if err := storageEmptyTree(ctx, nil); err != nil {
			return err
		}
		if err := os.Chmod("database", 0o755); err != nil {
			return err
		}
		needle = "database"
	} else {
		blob, err := storage.CreateBlob("database.zefs")
		if err != nil {
			return err
		}
		if err := blob.Close(); err != nil {
			return err
		}
	}
	output, err := storageCommand(ctx, "", "start", "router.conf")
	if err == nil || !bytes.Contains(output, []byte(needle)) {
		return fmt.Errorf("start did not refuse invalid storage: %v\n%s", err, output)
	}
	if _, err := os.Stat("ca-fingerprint"); !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("refused daemon ran probe: %v", err)
	}
	if !looseMode {
		if _, err := os.Stat("database"); !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("blob refusal created a tree: %v", err)
		}
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
	if _, err := os.Stat("database"); !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("doctor created a store: %v", err)
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
	path := filepath.Join("database", "meta", "fixture", "bad")
	frame, err := os.ReadFile(path)
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
	output, err := storageCommand(ctx, "", "data", "check", "--path", "database")
	if err == nil || !bytes.Contains(output, []byte("meta/fixture/bad")) {
		return fmt.Errorf("data check accepted corruption: %v\n%s", err, output)
	}
	output, err = storageCommand(ctx, "", "data", "repair", "--path", "database", "--output", "repaired")
	if exitErr, ok := errors.AsType[*exec.ExitError](err); !ok || exitErr.ExitCode() != 1 {
		return fmt.Errorf("repair did not report partial recovery: %v\n%s", err, output)
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
		for _, path := range []string{"database", "database.zefs", "meta/ca/cert", "meta/ca/key"} {
			if _, err := os.Stat(path); !errors.Is(err, fs.ErrNotExist) {
				return fmt.Errorf("storeless startup persisted %s: %v", path, err)
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
		return fmt.Errorf("plugin started despite corrupt persistent CA: %v", err)
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
			return fmt.Errorf("%v did not refuse unavailable history: %v\n%s", args, err, output)
		}
	}
	if _, err := os.Stat("database"); !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("offline edit created a store: %v", err)
	}
	return nil
}

func storageStateObserver(ctx context.Context, args []string) error {
	if len(args) != 1 || (args[0] != "write" && args[0] != "read") {
		return errors.New("state-observer requires write or read")
	}
	return Observe(ctx, "ospf", sdk.Registration{}, func(ctx context.Context, plugin *sdk.Plugin) error {
		const key = "meta/ospf/gr-fact-storage-fixture"
		if args[0] == "write" {
			if _, found, err := plugin.StateGet(ctx, key); err != nil || found {
				return fmt.Errorf("new state key: found=%v err=%v", found, err)
			}
			if err := plugin.StatePut(ctx, key, []byte("durable fixture state")); err != nil {
				return err
			}
		}
		value, found, err := plugin.StateGet(ctx, key)
		if err != nil || !found || string(value) != "durable fixture state" {
			return fmt.Errorf("persisted state: found=%v value=%q err=%v", found, value, err)
		}
		if args[0] == "read" {
			if err := plugin.StateRemove(ctx, key); err != nil {
				return err
			}
			if _, found, err := plugin.StateGet(ctx, key); err != nil || found {
				return fmt.Errorf("removed state still present: found=%v err=%v", found, err)
			}
		}
		return nil
	})
}

func storageStateRestart(ctx context.Context) error {
	for _, mode := range []string{"write", "read"} {
		config := fmt.Sprintf("plugin { external ospf { run \"ze-test fixture storage/state-observer %s\"; encoder json; } }\n", mode)
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
