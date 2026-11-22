package init

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/core/resolve"
	"github.com/ze-software/ze/pkg/zefs"
)

// sshListenerBlock is the listener an operator writes to make the daemon
// accept the CLI on host and port. It is spelled here, apart from the
// producer, so the tests compare the producer with the config grammar rather
// than with itself.
func sshListenerBlock(host, port string) string {
	return "environment {\n" +
		"    ssh {\n" +
		"        enabled true;\n" +
		"        server local {\n" +
		"            ip " + host + ";\n" +
		"            port " + port + ";\n" +
		"        }\n" +
		"    }\n" +
		"}\n"
}

// openInitBlob opens the blob `ze init --seed` wrote at dir, and closes it
// when the test ends. openInitStore opens the live tree.
func openInitBlob(t *testing.T, dir string) storage.Storage {
	t.Helper()
	store, err := storage.OpenBlob(filepath.Join(dir, "database.zefs"), false)
	if err != nil {
		t.Fatalf("open the blob init wrote: %v", err)
	}
	t.Cleanup(func() { store.Close() }) //nolint:errcheck // test cleanup
	return store
}

// initialConfigKey is the key of the config a bare `ze start` on store reads.
func initialConfigKey(t *testing.T, store storage.Storage) string {
	t.Helper()
	name, err := resolve.DefaultConfig(store)
	if err != nil {
		t.Fatalf("name the initial config: %v", err)
	}
	return zefs.KeyFileActive.Key(name)
}

// TestZeInitDefaultWritesSSHListener: the config the default profile writes
// makes the daemon listen for the CLI on the endpoint ze init saved for it.
//
// Method: run init with each address family and a port other than the
// default, then compare the opening of the written config, byte for byte,
// with the listener block, and the CLI's saved target with the same endpoint.
// PREVENTS: only `ze install` writing the listener, so after `ze init` and
// `ze start` every `ze cli` command was refused with "connection refused".
func TestZeInitDefaultWritesSSHListener(t *testing.T) {
	tests := []struct {
		name string
		host string
		port string
	}{
		{"ipv4", "127.0.0.1", "2299"},
		{"ipv6", "::1", "2300"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			input := "admin\nsecret123\n" + tt.host + "\n" + tt.port + "\nedge\n"
			if code := runWithReader(strings.NewReader(input), dir, false); code != 0 {
				t.Fatalf("ze init exit = %d, want 0", code)
			}
			store := openInitStore(t, dir)
			assertStoreFile(t, store, zefs.KeySSHDefault.Pattern, tt.host+"/"+tt.port)
			data, err := store.ReadKey(initialConfigKey(t, store))
			if err != nil {
				t.Fatalf("ze init wrote no initial config: %v", err)
			}
			want := sshListenerBlock(tt.host, tt.port)
			if !strings.HasPrefix(string(data), want) {
				t.Fatalf("the initial config does not open with the SSH listener on %s port %s:\n%s", tt.host, tt.port, data)
			}
		})
	}
}

// TestZeInitDefaultKeepsDiscoveredInterfaces: the listener is written beside
// the discovered interfaces, not in place of them.
//
// Method: on Linux netlink discovery always finds the loopback, so the config
// after the listener block MUST hold the interface block with its loopback.
func TestZeInitDefaultKeepsDiscoveredInterfaces(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("netlink discovery, which always finds the loopback, runs on Linux only")
	}
	dir := t.TempDir()
	if code := runWithReader(strings.NewReader("admin\nsecret123\n\n\nedge\n"), dir, false); code != 0 {
		t.Fatalf("ze init exit = %d, want 0", code)
	}
	store := openInitStore(t, dir)
	data, err := store.ReadKey(initialConfigKey(t, store))
	if err != nil {
		t.Fatalf("ze init wrote no initial config: %v", err)
	}
	interfaces, found := strings.CutPrefix(string(data), sshListenerBlock(defaultHost, defaultPort))
	if !found {
		t.Fatalf("the initial config does not open with the default SSH listener:\n%s", data)
	}
	if !strings.HasPrefix(interfaces, "interface {\n") {
		t.Fatalf("no interface block follows the listener:\n%s", data)
	}
	if !strings.Contains(interfaces, "\n    loopback {\n") {
		t.Fatalf("the discovered loopback is missing:\n%s", data)
	}
}

// TestZeInitRefusesListenerItCannotBind: an address or port the listener
// could not bind is refused before a store exists, so ze init never writes a
// config `ze start` refuses or a listener the CLI cannot find.
func TestZeInitRefusesListenerItCannotBind(t *testing.T) {
	tests := []struct {
		name string
		host string
		port string
	}{
		{"host name", "localhost", "2222"},
		{"dotted host name", "router.example.net", "2222"},
		{"port zero", "127.0.0.1", "0"},
		{"port above 65535", "127.0.0.1", "65536"},
		{"port not a number", "127.0.0.1", "ssh"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			input := "admin\nsecret123\n" + tt.host + "\n" + tt.port + "\nedge\n"
			if code := runWithReader(strings.NewReader(input), dir, false); code == 0 {
				t.Fatalf("ze init accepted the SSH endpoint %s port %s", tt.host, tt.port)
			}
			if _, err := os.Stat(filepath.Join(dir, "database")); !os.IsNotExist(err) {
				t.Fatalf("a refused ze init left a store behind: %v", err)
			}
		})
	}
}

// TestZeInitSeedAndManagedWriteNoListener: --seed and --managed keep writing
// no initial config, so neither the appliance template nor the hub's config
// is shadowed, and they keep accepting a host name as the CLI's target, which
// no listener has to bind.
func TestZeInitSeedAndManagedWriteNoListener(t *testing.T) {
	tests := []struct {
		name    string
		seed    bool
		managed bool
	}{
		{"seed", true, false},
		{"managed", false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			input := "admin\nsecret123\nrouter.example.net\n2222\nedge\n"
			if code := runInit(strings.NewReader(input), nil, dir, tt.managed, "", "", tt.seed, false); code != 0 {
				t.Fatalf("ze init exit = %d, want 0", code)
			}
			var store storage.Storage
			if tt.seed {
				store = openInitBlob(t, dir)
			} else {
				store = openInitStore(t, dir)
			}
			assertStoreFile(t, store, zefs.KeySSHDefault.Pattern, "router.example.net/2222")
			if key := initialConfigKey(t, store); store.Exists(key) {
				t.Fatalf("ze init wrote %s; this profile must write no initial config", key)
			}
		})
	}
}
