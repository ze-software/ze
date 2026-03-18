package config

import (
	"bytes"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/config/storage"
)

// TestPromptCreateConfigYes verifies that answering "y" creates the file.
//
// VALIDATES: "y" answer creates an empty config file with correct permissions.
//
// PREVENTS: Create prompt accepting "y" but failing to create the file.
func TestPromptCreateConfigYes(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "new.conf")

	in := strings.NewReader("y\n")
	var errBuf bytes.Buffer

	ok := doPromptCreateConfig(path, in, &errBuf, time.Second)

	if !ok {
		t.Fatal("expected true for 'y' answer")
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("file not created: %v", err)
	}

	if info.Size() != 0 {
		t.Errorf("expected empty file, got %d bytes", info.Size())
	}

	if info.Mode().Perm() != 0o600 {
		t.Errorf("expected permissions 0600, got %04o", info.Mode().Perm())
	}
}

// TestPromptCreateConfigYesFull verifies that "yes" (full word) also works.
//
// VALIDATES: "yes" is accepted as affirmative.
//
// PREVENTS: Only single-letter "y" being accepted.
func TestPromptCreateConfigYesFull(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "new.conf")

	in := strings.NewReader("yes\n")
	var errBuf bytes.Buffer

	ok := doPromptCreateConfig(path, in, &errBuf, time.Second)

	if !ok {
		t.Fatal("expected true for 'yes' answer")
	}

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("file not created: %v", err)
	}
}

// TestPromptCreateConfigNo verifies that "n" does not create the file.
//
// VALIDATES: "n" answer returns false without creating file.
//
// PREVENTS: Accidental file creation on negative answer.
func TestPromptCreateConfigNo(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "new.conf")

	in := strings.NewReader("n\n")
	var errBuf bytes.Buffer

	ok := doPromptCreateConfig(path, in, &errBuf, time.Second)

	if ok {
		t.Fatal("expected false for 'n' answer")
	}

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("file should not exist after 'n' answer")
	}
}

// TestPromptCreateConfigEmpty verifies that empty input (just Enter) is treated as no.
//
// VALIDATES: Default [N] behavior — empty input declines creation.
//
// PREVENTS: Empty input being treated as affirmative.
func TestPromptCreateConfigEmpty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "new.conf")

	in := strings.NewReader("\n")
	var errBuf bytes.Buffer

	ok := doPromptCreateConfig(path, in, &errBuf, time.Second)

	if ok {
		t.Fatal("expected false for empty answer")
	}
}

// TestPromptCreateConfigTimeout verifies that no input triggers timeout.
//
// VALIDATES: Timeout returns false and prints error message.
//
// PREVENTS: Hanging forever when stdin has no input.
func TestPromptCreateConfigTimeout(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "new.conf")

	// Reader that blocks forever (never returns data)
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close() //nolint:errcheck // test cleanup
	defer w.Close() //nolint:errcheck // test cleanup

	var errBuf bytes.Buffer

	ok := doPromptCreateConfig(path, r, &errBuf, 50*time.Millisecond)

	if ok {
		t.Fatal("expected false on timeout")
	}

	if !strings.Contains(errBuf.String(), "no response") {
		t.Errorf("expected timeout message, got: %s", errBuf.String())
	}
}

// TestPromptCreateConfigCreatesParentDirs verifies that parent directories are created.
//
// VALIDATES: Missing parent directories are created with 0750 permissions.
//
// PREVENTS: Failure when parent directory doesn't exist.
func TestPromptCreateConfigCreatesParentDirs(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "dir", "new.conf")

	in := strings.NewReader("y\n")
	var errBuf bytes.Buffer

	ok := doPromptCreateConfig(path, in, &errBuf, time.Second)

	if !ok {
		t.Fatalf("expected true, stderr: %s", errBuf.String())
	}

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("file not created: %v", err)
	}

	// Check parent dir permissions
	info, err := os.Stat(filepath.Join(dir, "sub"))
	if err != nil {
		t.Fatalf("parent dir not created: %v", err)
	}

	if info.Mode().Perm() != 0o750 {
		t.Errorf("expected dir permissions 0750, got %04o", info.Mode().Perm())
	}
}

// TestPromptCreateConfigCaseInsensitive verifies that "Y" and "YES" are accepted.
//
// VALIDATES: Answer comparison is case-insensitive.
//
// PREVENTS: Uppercase input being rejected.
func TestPromptCreateConfigCaseInsensitive(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"uppercase Y", "Y\n", true},
		{"uppercase YES", "YES\n", true},
		{"mixed Yes", "Yes\n", true},
		{"no", "no\n", false},
		{"random", "maybe\n", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "new.conf")

			in := strings.NewReader(tt.input)
			var errBuf bytes.Buffer

			got := doPromptCreateConfig(path, in, &errBuf, time.Second)

			if got != tt.want {
				t.Errorf("input %q: got %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

// TestPromptCreateConfigOutputFormat verifies the prompt message format.
//
// VALIDATES: Prompt includes file path and [y/N] indicator.
//
// PREVENTS: Missing context in the prompt shown to users.
func TestPromptCreateConfigOutputFormat(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "myconfig.conf")

	in := strings.NewReader("n\n")
	var errBuf bytes.Buffer

	doPromptCreateConfig(path, in, &errBuf, time.Second)

	output := errBuf.String()

	if !strings.Contains(output, path) {
		t.Errorf("expected path in output, got: %s", output)
	}

	if !strings.Contains(output, "[y/N]") {
		t.Errorf("expected [y/N] prompt, got: %s", output)
	}
}

// TestDefaultConfigName verifies the default config name fallback and identity-based naming.
//
// VALIDATES: Default config name is ze.conf without identity, <name>.conf with identity.
// PREVENTS: Accidental change of default config name logic.
func TestDefaultConfigName(t *testing.T) {
	if fallbackConfigName != "ze.conf" {
		t.Errorf("fallbackConfigName = %q, want %q", fallbackConfigName, "ze.conf")
	}

	// Filesystem storage: always falls back to ze.conf
	fsStore := storage.NewFilesystem()
	if got := defaultConfigName(fsStore); got != "ze.conf" {
		t.Errorf("defaultConfigName(filesystem) = %q, want %q", got, "ze.conf")
	}

	// Blob storage with identity name
	dir := t.TempDir()
	blobPath := filepath.Join(dir, "database.zefs")
	blobStore, err := storage.NewBlob(blobPath, dir)
	if err != nil {
		t.Fatal(err)
	}
	defer blobStore.Close() //nolint:errcheck // test cleanup

	// No identity set: falls back to ze.conf
	if got := defaultConfigName(blobStore); got != "ze.conf" {
		t.Errorf("defaultConfigName(blob, no identity) = %q, want %q", got, "ze.conf")
	}

	// Set identity name
	if err := blobStore.WriteFile("meta/instance/name", []byte("ze-first"), 0); err != nil {
		t.Fatal(err)
	}
	if got := defaultConfigName(blobStore); got != "ze-first.conf" {
		t.Errorf("defaultConfigName(blob, identity=ze-first) = %q, want %q", got, "ze-first.conf")
	}
}

// TestSelectConfigAC6 verifies config selection when configs exist but ze.conf is missing (AC-6).
//
// VALIDATES: User can select from available configs when ze.conf is missing.
// PREVENTS: Silent failure when default config doesn't exist but others do.
func TestSelectConfigAC6(t *testing.T) {
	dir := t.TempDir()
	blobPath := filepath.Join(dir, "test.zefs")
	store, err := storage.NewBlob(blobPath, dir)
	if err != nil {
		t.Fatalf("NewBlob: %v", err)
	}
	defer store.Close() //nolint:errcheck // test cleanup

	// Write two configs into blob (not ze.conf)
	configDir := filepath.Join(dir, "etc", "ze")
	pathA := filepath.Join(configDir, "site-a.conf")
	pathB := filepath.Join(configDir, "site-b.conf")
	if err := store.WriteFile(pathA, []byte("config-a"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := store.WriteFile(pathB, []byte("config-b"), 0o600); err != nil {
		t.Fatal(err)
	}

	// Simulate user selecting option 1
	in := strings.NewReader("1\n")
	var errBuf bytes.Buffer
	defaultPath := filepath.Join(configDir, "ze.conf")

	selected := doSelectConfig(store, configDir, defaultPath, in, &errBuf, time.Second)

	if selected == "" {
		t.Fatalf("expected a selection, got empty string. stderr: %s", errBuf.String())
	}

	// Should have selected the first config alphabetically
	if !strings.HasSuffix(selected, "site-a.conf") {
		t.Errorf("expected site-a.conf (first alphabetically), got %q", selected)
	}

	// Verify prompt was shown
	output := errBuf.String()
	if !strings.Contains(output, "not found in store") {
		t.Errorf("expected 'not found in store' in output, got: %s", output)
	}
	if !strings.Contains(output, "site-a.conf") {
		t.Errorf("expected 'site-a.conf' listed, got: %s", output)
	}
}

// TestSelectConfigAC6SecondChoice verifies selecting a different config.
//
// VALIDATES: User can select any numbered option.
// PREVENTS: Always selecting the first config regardless of input.
func TestSelectConfigAC6SecondChoice(t *testing.T) {
	dir := t.TempDir()
	blobPath := filepath.Join(dir, "test.zefs")
	store, err := storage.NewBlob(blobPath, dir)
	if err != nil {
		t.Fatalf("NewBlob: %v", err)
	}
	defer store.Close() //nolint:errcheck // test cleanup

	configDir := filepath.Join(dir, "etc", "ze")
	pathA := filepath.Join(configDir, "site-a.conf")
	pathB := filepath.Join(configDir, "site-b.conf")
	if err := store.WriteFile(pathA, []byte("config-a"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := store.WriteFile(pathB, []byte("config-b"), 0o600); err != nil {
		t.Fatal(err)
	}

	in := strings.NewReader("2\n")
	var errBuf bytes.Buffer
	defaultPath := filepath.Join(configDir, "ze.conf")

	selected := doSelectConfig(store, configDir, defaultPath, in, &errBuf, time.Second)

	if !strings.HasSuffix(selected, "site-b.conf") {
		t.Errorf("expected site-b.conf (second option), got %q", selected)
	}
}

// TestSelectConfigAC7 verifies default config creation when blob is empty (AC-7).
//
// VALIDATES: Empty blob triggers creation of default config.
// PREVENTS: Error or hang when blob has no configs at all.
func TestSelectConfigAC7(t *testing.T) {
	dir := t.TempDir()
	blobPath := filepath.Join(dir, "test.zefs")
	store, err := storage.NewBlob(blobPath, dir)
	if err != nil {
		t.Fatalf("NewBlob: %v", err)
	}
	defer store.Close() //nolint:errcheck // test cleanup

	// Don't write any configs - blob is empty
	configDir := filepath.Join(dir, "etc", "ze")
	defaultPath := filepath.Join(configDir, "ze.conf")

	in := strings.NewReader("") // no input needed for AC-7
	var errBuf bytes.Buffer

	selected := doSelectConfig(store, configDir, defaultPath, in, &errBuf, time.Second)

	if selected != defaultPath {
		t.Errorf("expected %q (created default config), got %q", defaultPath, selected)
	}

	// Verify config was created in blob
	if !store.Exists(defaultPath) {
		t.Error("default config should exist in blob after AC-7 creation")
	}

	// Verify creation message
	if !strings.Contains(errBuf.String(), "creating ze.conf") {
		t.Errorf("expected creation message, got: %s", errBuf.String())
	}
}

// TestSelectConfigInvalidInput verifies error on invalid selection.
//
// VALIDATES: Non-numeric and out-of-range input returns empty string.
// PREVENTS: Panic on bad user input.
func TestSelectConfigInvalidInput(t *testing.T) {
	dir := t.TempDir()
	blobPath := filepath.Join(dir, "test.zefs")
	store, err := storage.NewBlob(blobPath, dir)
	if err != nil {
		t.Fatalf("NewBlob: %v", err)
	}
	defer store.Close() //nolint:errcheck // test cleanup

	configDir := filepath.Join(dir, "etc", "ze")
	if err := store.WriteFile(filepath.Join(configDir, "test.conf"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	defaultPath := filepath.Join(configDir, "ze.conf")

	tests := []struct {
		name  string
		input string
	}{
		{"non-numeric", "abc\n"},
		{"zero", "0\n"},
		{"out of range", "99\n"},
		{"negative", "-1\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := strings.NewReader(tt.input)
			var errBuf bytes.Buffer
			selected := doSelectConfig(store, configDir, defaultPath, in, &errBuf, time.Second)
			if selected != "" {
				t.Errorf("expected empty for %q, got %q", tt.input, selected)
			}
		})
	}
}

// TestSelectConfigTimeout verifies timeout behavior.
//
// VALIDATES: No response within timeout returns empty string.
// PREVENTS: Hanging forever when stdin has no input.
func TestSelectConfigTimeout(t *testing.T) {
	dir := t.TempDir()
	blobPath := filepath.Join(dir, "test.zefs")
	store, err := storage.NewBlob(blobPath, dir)
	if err != nil {
		t.Fatalf("NewBlob: %v", err)
	}
	defer store.Close() //nolint:errcheck // test cleanup

	configDir := filepath.Join(dir, "etc", "ze")
	if err := store.WriteFile(filepath.Join(configDir, "test.conf"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	defaultPath := filepath.Join(configDir, "ze.conf")

	// Reader that blocks forever
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close() //nolint:errcheck // test cleanup
	defer w.Close() //nolint:errcheck // test cleanup

	var errBuf bytes.Buffer
	selected := doSelectConfig(store, configDir, defaultPath, r, &errBuf, 50*time.Millisecond)

	if selected != "" {
		t.Errorf("expected empty on timeout, got %q", selected)
	}
	if !strings.Contains(errBuf.String(), "no response") {
		t.Errorf("expected timeout message, got: %s", errBuf.String())
	}
}

// TestSelectConfigFiltersNonConf verifies only .conf files are listed.
//
// VALIDATES: Draft files and non-config files are excluded from selection.
// PREVENTS: .draft, .lock, ssh_host_* files appearing in the selection list.
func TestSelectConfigFiltersNonConf(t *testing.T) {
	dir := t.TempDir()
	blobPath := filepath.Join(dir, "test.zefs")
	store, err := storage.NewBlob(blobPath, dir)
	if err != nil {
		t.Fatalf("NewBlob: %v", err)
	}
	defer store.Close() //nolint:errcheck // test cleanup

	configDir := filepath.Join(dir, "etc", "ze")
	// Write a mix of file types
	if err := store.WriteFile(filepath.Join(configDir, "router.conf"), []byte("config"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := store.WriteFile(filepath.Join(configDir, "router.conf.draft"), []byte("draft"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := store.WriteFile(filepath.Join(configDir, "router.conf.lock"), []byte("lock"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := store.WriteFile(filepath.Join(configDir, "ssh_host_ed25519_key"), []byte("key"), 0o600); err != nil {
		t.Fatal(err)
	}

	in := strings.NewReader("1\n")
	var errBuf bytes.Buffer
	defaultPath := filepath.Join(configDir, "ze.conf")

	selected := doSelectConfig(store, configDir, defaultPath, in, &errBuf, time.Second)

	// Should only show router.conf (the only .conf file)
	if !strings.HasSuffix(selected, "router.conf") {
		t.Errorf("expected router.conf, got %q", selected)
	}

	// Verify non-.conf files not in output
	output := errBuf.String()
	if strings.Contains(output, ".draft") {
		t.Error("draft files should not appear in selection list")
	}
	if strings.Contains(output, "ssh_host") {
		t.Error("ssh host key should not appear in selection list")
	}
}

// VALIDATES: Live SSH port detected, returns true (daemon running)
// PREVENTS: False negative causing unnecessary ephemeral daemon start

func TestLivePortDetection(t *testing.T) {
	var lc net.ListenConfig
	ln, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close() //nolint:errcheck // test cleanup

	_, port, _ := net.SplitHostPort(ln.Addr().String())

	if !probeDaemonSSH("127.0.0.1", port) {
		t.Error("expected true for live port")
	}
}

// VALIDATES: Dead SSH port detected, returns false (no daemon)
// PREVENTS: False positive causing editor to skip ephemeral daemon

func TestStalePortDetection(t *testing.T) {
	if probeDaemonSSH("127.0.0.1", "1") {
		t.Error("expected false for unreachable port")
	}
}
