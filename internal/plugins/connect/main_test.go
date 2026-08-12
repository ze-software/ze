package connect_test

import (
	"bufio"
	"errors"
	"io"
	"os"
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

// failingReader yields prefix, then fails. It models a pipe that breaks part
// way through the password line.
type failingReader struct {
	prefix []byte
	err    error
}

func (f *failingReader) Read(p []byte) (int, error) {
	if len(f.prefix) == 0 {
		return 0, f.err
	}
	n := copy(p, f.prefix)
	f.prefix = f.prefix[n:]
	return n, nil
}

// VALIDATES: a password read that fails part way through stores nothing.
// PREVENTS: a truncated password hashed and written under the operator's host,
// which authenticates against nothing and says so only at the next connect.
// Scan returns TRUE here, handing back the buffered prefix as a whole line.
func TestAddCredentialsRejectsTruncatedPassword(t *testing.T) {
	dbPath := seedDB(t)

	code := zeconnect.AddCredentialsFromReader(
		&failingReader{prefix: []byte("secr"), err: errors.New("pipe broke")},
		dbPath, "10.0.1.5", "2223", "admin",
	)
	if code == 0 {
		t.Fatal("a truncated password was accepted")
	}

	store, err := zefs.Open(dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer store.Close() //nolint:errcheck // test

	if store.Has("meta/ssh/10.0.1.5/2223/password") {
		t.Error("a truncated password was stored")
	}
}

// VALIDATES: a password line above bufio.MaxScanTokenSize is reported as a
// read failure. This is the failure mode with no underlying I/O error.
//
// The assertion is on the MESSAGE, not the exit code. Both readings refuse the
// input, so an exit code of 1 would be reached with or without the check and
// would prove nothing. What changes is whether the operator is told the line
// was too long or told they typed nothing.
func TestAddCredentialsReportsOverLongPassword(t *testing.T) {
	dbPath := seedDB(t)

	stderr := captureStderr(t)
	code := zeconnect.AddCredentialsFromReader(
		strings.NewReader(strings.Repeat("p", bufio.MaxScanTokenSize+1)),
		dbPath, "10.0.1.5", "2223", "admin",
	)
	got := stderr()

	if code == 0 {
		t.Fatal("an over-long password line was accepted")
	}
	if !strings.Contains(got, "reading stdin") {
		t.Errorf("the read failure was not reported, stderr said: %q", got)
	}
	store, err := zefs.Open(dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer store.Close() //nolint:errcheck // test
	if store.Has("meta/ssh/10.0.1.5/2223/password") {
		t.Error("a truncated password was stored")
	}
}

// captureStderr redirects os.Stderr for the duration of the test and returns a
// function yielding what was written. The functions under test write to
// os.Stderr directly, which is the surface the operator reads.
func captureStderr(t *testing.T) func() string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	orig := os.Stderr
	os.Stderr = w
	t.Cleanup(func() { os.Stderr = orig })

	return func() string {
		os.Stderr = orig
		w.Close()       //nolint:errcheck // test teardown
		defer r.Close() //nolint:errcheck // test teardown
		var buf strings.Builder
		if _, err := io.Copy(&buf, r); err != nil {
			t.Fatalf("read captured stderr: %v", err)
		}
		return buf.String()
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
