package init

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/component/pki"
	"github.com/ze-software/ze/pkg/zefs"
)

// VALIDATES: ze init --managed stores what the first start of a managed
// client connects with -- the hub address, the client's secret, the hub root
// given with --hub-ca and the first-use choice -- each only when it was given,
// and refuses a value that could never work.
// PREVENTS: a managed client that has to be handed its hub on the command
// line of its first start, and a first start with nothing to authenticate the
// hub against.

const (
	testHubAddress = "10.0.0.1:1791"
	testHubToken   = "edge-01-secret-of-at-least-32-chars"
	// managedInput is the seven lines ze init --managed reads.
	managedInput = "admin\nsecret123\n127.0.0.1\n2222\nedge-01\n" + testHubAddress + "\n" + testHubToken + "\n"
)

// hubRootPEM returns a real certificate authority root in PEM, the document
// `ze show pki local-ca pem` prints on a hub.
func hubRootPEM(t *testing.T) []byte {
	t.Helper()
	store, err := storage.Create(t.TempDir())
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	t.Cleanup(func() { store.Close() }) //nolint:errcheck // test cleanup
	root, err := pki.LoadOrGenerateRoot(store)
	if err != nil {
		t.Fatalf("LoadOrGenerateRoot: %v", err)
	}
	return root.CertificatePEM()
}

func openInitStore(t *testing.T, dir string) storage.Storage {
	t.Helper()
	store, err := storage.OpenReadOnly(dir, dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { store.Close() }) //nolint:errcheck // test cleanup
	return store
}

func assertAbsent(t *testing.T, store storage.Storage, key string) {
	t.Helper()
	if store.Exists(key) {
		t.Errorf("%s is in the store and was not given", key)
	}
}

// TestInitManagedStoresHubAddressAndToken: the two extra lines of a managed
// init are stored, and no trust key is written when no trust was chosen.
func TestInitManagedStoresHubAddressAndToken(t *testing.T) {
	dir := t.TempDir()
	if code := runInitFleet(strings.NewReader(managedInput), nil, dir, managedInit{enabled: true}, "", "", false, false); code != 0 {
		t.Fatalf("exit = %d", code)
	}
	store := openInitStore(t, dir)
	assertStoreFile(t, store, zefs.KeyInstanceManaged.Pattern, "true")
	assertStoreFile(t, store, zefs.KeyInstanceName.Pattern, "edge-01")
	assertStoreFile(t, store, zefs.KeyManagedHubAddress.Pattern, testHubAddress)
	assertStoreFile(t, store, zefs.KeyManagedHubToken.Pattern, testHubToken)
	assertAbsent(t, store, zefs.KeyManagedHubCA.Pattern)
	assertAbsent(t, store, zefs.KeyManagedHubTrust.Pattern)
}

// TestInitManagedWithoutHubLines: the five lines every existing script feeds
// still initialise a managed client, which then names its hub at the first
// start. Nothing about a hub is stored.
func TestInitManagedWithoutHubLines(t *testing.T) {
	dir := t.TempDir()
	input := "admin\nsecret123\n127.0.0.1\n2222\nedge-01\n"
	if code := runInitFleet(strings.NewReader(input), nil, dir, managedInit{enabled: true}, "", "", false, false); code != 0 {
		t.Fatalf("exit = %d", code)
	}
	store := openInitStore(t, dir)
	assertStoreFile(t, store, zefs.KeyInstanceManaged.Pattern, "true")
	assertAbsent(t, store, zefs.KeyManagedHubAddress.Pattern)
	assertAbsent(t, store, zefs.KeyManagedHubToken.Pattern)
}

// TestInitUnmanagedAsksNothingAboutAHub: without --managed the sixth and
// seventh lines are not read as a hub, so nothing about one is stored.
func TestInitUnmanagedAsksNothingAboutAHub(t *testing.T) {
	dir := t.TempDir()
	if code := runInitFleet(strings.NewReader(managedInput), nil, dir, managedInit{}, "", "", false, false); code != 0 {
		t.Fatalf("exit = %d", code)
	}
	store := openInitStore(t, dir)
	assertStoreFile(t, store, zefs.KeyInstanceManaged.Pattern, "false")
	assertAbsent(t, store, zefs.KeyManagedHubAddress.Pattern)
	assertAbsent(t, store, zefs.KeyManagedHubToken.Pattern)
}

// TestInitManagedStoresHubRoot: the root given with --hub-ca is stored as it
// was given, and the first-use choice is stored as its one value.
func TestInitManagedStoresHubRoot(t *testing.T) {
	rootPEM := hubRootPEM(t)

	t.Run("hub root", func(t *testing.T) {
		dir := t.TempDir()
		fleet := managedInit{enabled: true, hubCA: rootPEM}
		if code := runInitFleet(strings.NewReader(managedInput), nil, dir, fleet, "", "", false, false); code != 0 {
			t.Fatalf("exit = %d", code)
		}
		store := openInitStore(t, dir)
		assertStoreFile(t, store, zefs.KeyManagedHubCA.Pattern, string(rootPEM))
		assertAbsent(t, store, zefs.KeyManagedHubTrust.Pattern)
	})
	t.Run("first use", func(t *testing.T) {
		dir := t.TempDir()
		fleet := managedInit{enabled: true, trustFirstUse: true}
		if code := runInitFleet(strings.NewReader(managedInput), nil, dir, fleet, "", "", false, false); code != 0 {
			t.Fatalf("exit = %d", code)
		}
		store := openInitStore(t, dir)
		assertStoreFile(t, store, zefs.KeyManagedHubTrust.Pattern, zefs.ManagedHubTrustFirstUse)
		assertAbsent(t, store, zefs.KeyManagedHubCA.Pattern)
	})
}

// TestInitManagedRefusesWhatCouldNotWork: a hub address that is not
// host:port and a secret shorter than a hub accepts are refused, and no store
// is created for either.
func TestInitManagedRefusesWhatCouldNotWork(t *testing.T) {
	for _, tc := range []struct {
		name  string
		input string
	}{
		{"address with no port", "admin\nsecret123\n127.0.0.1\n2222\nedge-01\n10.0.0.1\n" + testHubToken + "\n"},
		{"short secret", "admin\nsecret123\n127.0.0.1\n2222\nedge-01\n" + testHubAddress + "\nshort\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if code := runInitFleet(strings.NewReader(tc.input), nil, dir, managedInit{enabled: true}, "", "", false, false); code == 0 {
				t.Fatal("accepted")
			}
			if _, err := os.Stat(filepath.Join(dir, "database")); !os.IsNotExist(err) {
				t.Errorf("a store was created for a refused init: %v", err)
			}
		})
	}
}

// TestManagedInitFromFlags: the managed flags are judged together. The hub
// flags need --managed, exclude each other, and --hub-ca takes one PEM
// certificate of a certificate authority and nothing else.
func TestManagedInitFromFlags(t *testing.T) {
	dir := t.TempDir()
	rootPEM := hubRootPEM(t)
	rootPath := filepath.Join(dir, "root.pem")
	if err := os.WriteFile(rootPath, rootPEM, 0o600); err != nil {
		t.Fatalf("write root: %v", err)
	}
	notPEM := filepath.Join(dir, "not.pem")
	if err := os.WriteFile(notPEM, []byte("not a certificate\n"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	twoRoots := filepath.Join(dir, "two.pem")
	if err := os.WriteFile(twoRoots, append(append([]byte{}, rootPEM...), rootPEM...), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	t.Run("accepted", func(t *testing.T) {
		fleet, err := managedInitFromFlags(true, rootPath, false)
		if err != nil {
			t.Fatalf("managedInitFromFlags: %v", err)
		}
		if !fleet.enabled || string(fleet.hubCA) != string(rootPEM) || fleet.trustFirstUse {
			t.Errorf("fleet = %+v", fleet)
		}
		firstUse, err := managedInitFromFlags(true, "", true)
		if err != nil {
			t.Fatalf("managedInitFromFlags: %v", err)
		}
		if !firstUse.trustFirstUse || len(firstUse.hubCA) != 0 {
			t.Errorf("fleet = %+v", firstUse)
		}
		plain, err := managedInitFromFlags(false, "", false)
		if err != nil {
			t.Fatalf("managedInitFromFlags: %v", err)
		}
		if plain.enabled {
			t.Errorf("fleet = %+v", plain)
		}
	})

	for _, tc := range []struct {
		name          string
		managed       bool
		hubCA         string
		trustFirstUse bool
	}{
		{"hub root without managed", false, rootPath, false},
		{"first use without managed", false, "", true},
		{"hub root and first use", true, rootPath, true},
		{"missing file", true, filepath.Join(dir, "absent.pem"), false},
		{"not a certificate", true, notPEM, false},
		{"two certificates", true, twoRoots, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if fleet, err := managedInitFromFlags(tc.managed, tc.hubCA, tc.trustFirstUse); err == nil {
				t.Errorf("accepted as %+v", fleet)
			}
		})
	}
}
