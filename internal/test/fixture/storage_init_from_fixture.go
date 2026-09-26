// Design: docs/architecture/testing/ci-format.md -- tree storage functional scenarios
// Related: storage_tree_fixture.go -- storageRequireCommand, storageArchives, storageTreeEqualsBlob
// Related: register_storage_init_from.go -- registers the three scenarios

package fixture

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"

	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/zefs"
)

// flagFrom is the ze init flag every scenario here drives.
const flagFrom = "--from"

// storageInitFromOK reports a passed scenario on stdout, where the .ci
// expects it. Every scenario runs in the working directory, which the runner
// makes the store folder.
func storageInitFromOK(name string) error {
	out := textbuf.New()
	out.Str("OK: ").Str(name).Byte('\n')
	return out.StdOut()
}

// storageInitFromValues is the whole store a backup or a seed carries. Its
// active router.conf is the storage probe, so a daemon started on the
// imported tree proves it runs by authenticating the probe.
func storageInitFromValues() map[string][]byte {
	return map[string][]byte{
		zefs.KeyLocalAdminUsername.Pattern:    []byte(storageSeedUser),
		zefs.KeyLocalAdminPassword.Pattern:    []byte("$2a$10$seedhash"),
		zefs.KeyInstanceName.Pattern:          []byte("seeded-router"),
		zefs.KeyFileActive.Key("router.conf"): []byte(storageProbeConfig),
	}
}

// storageInitFromSeed writes values to an exact-fit blob, as the appliance
// seed builders do, and returns its bytes. The blob is built in a private
// temporary folder that is removed on return: a fixed name under the shared
// temporary folder survives the run, and the next run's create refuses it.
func storageInitFromSeed(values map[string][]byte) ([]byte, error) {
	dir, err := os.MkdirTemp("", "ze-init-from-seed-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir) //nolint:errcheck // a leftover temporary folder changes no result
	path := filepath.Join(dir, "seed.zefs")
	blob, err := storage.CreateBlobPopulated(path, func(seed storage.Storage) error {
		for key, value := range values {
			if err := seed.WriteKey(key, value); err != nil {
				return err
			}
		}
		return nil
	}, false, zefs.Spare(0))
	if err != nil {
		return nil, err
	}
	if err := blob.Close(); err != nil {
		return nil, err
	}
	return os.ReadFile(path) //nolint:gosec // the fixture chose the path
}

// storageInitFromPath rebuilds a box from a backup: a store in old/ is backed
// up, `ze init --from ./backup.zefs` publishes it byte-equal, retires the
// backup, and `ze start` runs on the imported tree.
func storageInitFromPath(ctx context.Context, _ []string) error {
	values := storageInitFromValues()
	old, err := storage.Create("old")
	if err != nil {
		return err
	}
	for key, value := range values {
		if err := old.WriteKey(key, value); err != nil {
			return errors.Join(err, old.Close())
		}
	}
	if _, err := storage.Backup(old, "backup.zefs", false); err != nil {
		return errors.Join(err, old.Close())
	}
	if err := old.Close(); err != nil {
		return err
	}
	if _, err := storageRequireCommand(ctx, "", argInit, flagFrom, "./backup.zefs"); err != nil {
		return err
	}
	archives, err := storageArchives("backup.zefs")
	if err != nil {
		return err
	}
	if len(archives) != 1 {
		return fmt.Errorf("import retired %d archives, want 1: %v", len(archives), archives)
	}
	if err := storageTreeEqualsBlob(archives[0], values); err != nil {
		return err
	}
	if err := storageInitFromStart(ctx); err != nil {
		return err
	}
	return storageInitFromOK("init-from-path")
}

// storageInitFromURL provisions a box from a blob a web server publishes:
// `ze init --from http://.../install/database.zefs --sha256 <hex>` fetches,
// checks and imports it, leaves no fetched copy and no retired name behind,
// and `ze start` runs on the imported tree.
func storageInitFromURL(ctx context.Context, _ []string) error {
	values := storageInitFromValues()
	blob, err := storageInitFromSeed(values)
	if err != nil {
		return err
	}
	server := storageInitFromServer(map[string][]byte{"/install/database.zefs": blob})
	defer server.Close()

	sum := sha256.Sum256(blob)
	source := server.URL + "/install/database.zefs"
	if _, err := storageRequireCommand(ctx, "", argInit, flagFrom, source, "--sha256", textbuf.StringHex(sum[:])); err != nil {
		return err
	}
	if err := storageInitFromLeftovers(); err != nil {
		return err
	}
	tree, err := storage.OpenReadOnly(".")
	if err != nil {
		return err
	}
	for key, want := range values {
		got, readErr := tree.ReadKey(key)
		if readErr != nil || !bytes.Equal(got, want) {
			return errors.Join(fmt.Errorf("tree %s = %q, want %q", key, got, want), readErr, tree.Close())
		}
	}
	if err := tree.Close(); err != nil {
		return err
	}
	if err := storageInitFromStart(ctx); err != nil {
		return err
	}
	return storageInitFromOK("init-from-url")
}

// storageInitFromRefused proves every refusal imports nothing and leaves no
// fetched copy: an unsupported scheme names itself and the supported list, a
// digest mismatch and a body that is not a blob name the reason, and a new
// import beside an existing store names the store.
func storageInitFromRefused(ctx context.Context, _ []string) error {
	blob, err := storageInitFromSeed(storageInitFromValues())
	if err != nil {
		return err
	}
	server := storageInitFromServer(map[string][]byte{"/blob": blob, "/junk": []byte("not a blob")})
	defer server.Close()

	refusals := []struct {
		args []string
		want string
	}{
		{[]string{argInit, flagFrom, "ftp://127.0.0.1/database.zefs"}, `unsupported scheme "ftp": supported http, https`},
		{[]string{argInit, flagFrom, server.URL + "/blob", "--sha256", strings.Repeat("0", 64)}, "sha256 mismatch"},
		{[]string{argInit, flagFrom, server.URL + "/junk"}, "invalid magic"},
	}
	for _, refusal := range refusals {
		if err := storageInitFromRefusal(ctx, refusal.args, refusal.want); err != nil {
			return err
		}
		_, err := os.Lstat(storageTreeName)
		if err == nil {
			return fmt.Errorf("refused %v published %s", refusal.args, storageTreeName)
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("stat %s: %w", storageTreeName, err)
		}
	}
	existing, err := storage.Create(".")
	if err != nil {
		return err
	}
	if err := existing.Close(); err != nil {
		return err
	}
	if err := storageInitFromRefusal(ctx, []string{argInit, flagFrom, server.URL + "/blob"}, "database already exists"); err != nil {
		return err
	}
	return storageInitFromOK("init-from-refused")
}

func storageInitFromRefusal(ctx context.Context, args []string, want string) error {
	output, err := storageCommand(ctx, "", args...)
	if err == nil {
		return fmt.Errorf("ze %v was not refused:\n%s", args, output)
	}
	if !bytes.Contains(output, []byte(want)) {
		return fmt.Errorf("ze %v refused without %q:\n%s", args, want, output)
	}
	return storageInitFromLeftovers()
}

// storageInitFromServer serves bodies by path from a loopback listener, so no
// scenario reaches an external network.
func storageInitFromServer(bodies map[string][]byte) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, found := bodies[r.URL.Path]
		if !found {
			http.NotFound(w, r)
			return
		}
		w.Write(body) //nolint:errcheck // a short write fails the import, which the scenario reports
	}))
}

// storageInitFromLeftovers refuses a fetch staging folder or a retired fetched
// copy left in the store folder.
func storageInitFromLeftovers() error {
	entries, err := os.ReadDir(".")
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "database.fetch-") {
			return fmt.Errorf("fetched copy left behind: %s", entry.Name())
		}
		if strings.Contains(entry.Name(), "fetched.zefs") {
			return fmt.Errorf("retired fetched copy left behind: %s", entry.Name())
		}
	}
	return nil
}

// storageInitFromStart starts the daemon on the imported tree. The probe
// authenticates only when the daemon runs, and the seeded credentials and
// identity must survive the start.
func storageInitFromStart(ctx context.Context) error {
	if err := os.WriteFile("router.conf", []byte(storageProbeConfig), 0o600); err != nil {
		return err
	}
	output, err := storageRequireCommand(ctx, "", "start", "router.conf")
	if err != nil {
		return err
	}
	if !bytes.Contains(output, []byte("OK: storage probe authenticated")) {
		return fmt.Errorf("daemon on the imported tree never authenticated the probe:\n%s", output)
	}
	tree, err := storage.OpenReadOnly(".")
	if err != nil {
		return err
	}
	defer tree.Close() //nolint:errcheck // read-only check
	values := storageInitFromValues()
	for _, key := range []string{zefs.KeyLocalAdminUsername.Pattern, zefs.KeyLocalAdminPassword.Pattern, zefs.KeyInstanceName.Pattern} {
		got, err := tree.ReadKey(key)
		if err != nil {
			return fmt.Errorf("start lost seeded %s: %w", key, err)
		}
		if !bytes.Equal(got, values[key]) {
			return fmt.Errorf("start changed seeded %s to %q", key, got)
		}
	}
	return nil
}
