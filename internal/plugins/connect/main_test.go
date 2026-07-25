package connect_test

import (
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"

	zeconnect "github.com/ze-software/ze/internal/plugins/connect"
	"github.com/ze-software/ze/pkg/zefs"
)

func seedDB(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "database.zefs")
	store, err := zefs.Create(dbPath)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	store.Close() //nolint:errcheck // test setup
	return dbPath
}

func TestAddCredentials(t *testing.T) {
	dbPath := seedDB(t)

	code := zeconnect.AddCredentialsFromReader(
		strings.NewReader("secret123\n"),
		dbPath, "10.0.1.5", "2223", "admin",
	)
	if code != 0 {
		t.Fatalf("AddCredentialsFromReader: exit %d", code)
	}

	store, err := zefs.Open(dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer store.Close() //nolint:errcheck // test

	data, err := store.ReadFile("meta/ssh/10.0.1.5/2223/username")
	if err != nil {
		t.Fatalf("read username: %v", err)
	}
	if string(data) != "admin" {
		t.Errorf("username: got %q, want %q", string(data), "admin")
	}

	hash, err := store.ReadFile("meta/ssh/10.0.1.5/2223/password")
	if err != nil {
		t.Fatalf("read password: %v", err)
	}
	if err := bcrypt.CompareHashAndPassword(hash, []byte("secret123")); err != nil {
		t.Errorf("password hash mismatch: %v", err)
	}
}

func TestAddDefaultPort(t *testing.T) {
	dbPath := seedDB(t)

	code := zeconnect.AddCredentialsFromReader(
		strings.NewReader("pass\n"),
		dbPath, "10.0.1.5", "2222", "admin",
	)
	if code != 0 {
		t.Fatalf("exit %d", code)
	}

	store, err := zefs.Open(dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer store.Close() //nolint:errcheck // test

	if !store.Has("meta/ssh/10.0.1.5/2222/username") {
		t.Error("credentials not stored at default port path")
	}
}

func TestListRemotes(t *testing.T) {
	dbPath := seedDB(t)

	zeconnect.AddCredentialsFromReader(strings.NewReader("pw1\n"), dbPath, "10.0.1.5", "2222", "alice")
	zeconnect.AddCredentialsFromReader(strings.NewReader("pw2\n"), dbPath, "10.0.1.6", "2223", "bob")

	code := zeconnect.ListRemotes(dbPath)
	if code != 0 {
		t.Fatalf("ListRemotes: exit %d", code)
	}
}

func TestListRemotesMarksDefault(t *testing.T) {
	dbPath := seedDB(t)

	zeconnect.AddCredentialsFromReader(strings.NewReader("pw1\n"), dbPath, "10.0.1.5", "2222", "alice")
	zeconnect.SetDefault(dbPath, "10.0.1.5", "2222")

	code := zeconnect.ListRemotes(dbPath)
	if code != 0 {
		t.Fatalf("ListRemotes: exit %d", code)
	}
}

func TestRemoveCredentials(t *testing.T) {
	dbPath := seedDB(t)

	zeconnect.AddCredentialsFromReader(strings.NewReader("pw\n"), dbPath, "10.0.1.5", "2222", "admin")

	code := zeconnect.RemoveCredentials(dbPath, "10.0.1.5", "2222")
	if code != 0 {
		t.Fatalf("RemoveCredentials: exit %d", code)
	}

	store, err := zefs.Open(dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer store.Close() //nolint:errcheck // test

	if store.Has("meta/ssh/10.0.1.5/2222/username") {
		t.Error("username still exists after remove")
	}
	if store.Has("meta/ssh/10.0.1.5/2222/password") {
		t.Error("password still exists after remove")
	}
}

func TestRemoveNonExistent(t *testing.T) {
	dbPath := seedDB(t)

	code := zeconnect.RemoveCredentials(dbPath, "unknown", "2222")
	if code == 0 {
		t.Fatal("expected non-zero exit for removing non-existent remote")
	}
}

func TestSetDefault(t *testing.T) {
	dbPath := seedDB(t)

	zeconnect.AddCredentialsFromReader(strings.NewReader("pw\n"), dbPath, "10.0.1.5", "2223", "admin")

	code := zeconnect.SetDefault(dbPath, "10.0.1.5", "2223")
	if code != 0 {
		t.Fatalf("SetDefault: exit %d", code)
	}

	store, err := zefs.Open(dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer store.Close() //nolint:errcheck // test

	data, err := store.ReadFile("meta/ssh/default")
	if err != nil {
		t.Fatalf("read default: %v", err)
	}
	if string(data) != "10.0.1.5/2223" {
		t.Errorf("default: got %q, want %q", string(data), "10.0.1.5/2223")
	}
}

func TestSetDefaultNonExistent(t *testing.T) {
	dbPath := seedDB(t)

	code := zeconnect.SetDefault(dbPath, "unknown", "2222")
	if code == 0 {
		t.Fatal("expected non-zero exit for setting default to non-existent remote")
	}
}
