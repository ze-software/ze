package appliance

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsureModcacheRW(t *testing.T) {
	t.Setenv("GOFLAGS", "")
	if err := ensureModcacheRW(); err != nil {
		t.Fatalf("empty GOFLAGS: %v", err)
	}
	if got := os.Getenv("GOFLAGS"); got != "-modcacherw" {
		t.Fatalf("empty GOFLAGS: got %q, want %q", got, "-modcacherw")
	}

	t.Setenv("GOFLAGS", "-mod=mod")
	if err := ensureModcacheRW(); err != nil {
		t.Fatalf("existing GOFLAGS: %v", err)
	}
	if got := os.Getenv("GOFLAGS"); got != "-mod=mod -modcacherw" {
		t.Fatalf("existing GOFLAGS: got %q, want %q", got, "-mod=mod -modcacherw")
	}

	t.Setenv("GOFLAGS", "-modcacherw -mod=mod")
	if err := ensureModcacheRW(); err != nil {
		t.Fatalf("already set: %v", err)
	}
	if got := os.Getenv("GOFLAGS"); got != "-modcacherw -mod=mod" {
		t.Fatalf("already set: got %q, want %q", got, "-modcacherw -mod=mod")
	}
}

func TestBuildUsesGokBuildFn(t *testing.T) {
	var called bool
	var gotArgs []string
	var gotArch string
	old := gokBuildFn
	gokBuildFn = func(args []string) error {
		called = true
		gotArgs = args
		gotArch = os.Getenv("GOARCH")
		return nil
	}
	defer func() { gokBuildFn = old }()

	oldExt := runExternalFn
	runExternalFn = func(name string, args ...string) ([]byte, error) {
		return nil, nil
	}
	defer func() { runExternalFn = oldExt }()

	t.Setenv("GOARCH", archAMD64)

	cfg := &applianceConfig{}
	cfg.Image.Arch = archARM64
	cfg.Image.SizeBytes = 1073741824

	runGokBuild(cfg, "/tmp/test.img")

	if !called {
		t.Fatal("gokBuildFn was not called")
	}

	// runGokBuild passes absolute paths because gok resolves modules from
	// gokrazy/modcache; relative paths would resolve incorrectly.
	wantParent, _ := filepath.Abs("gokrazy")
	wantImg, _ := filepath.Abs("/tmp/test.img")
	wantArgs := []string{
		"--parent_dir", wantParent,
		"-i", "ze",
		"overwrite",
		"--full", wantImg,
		"--target_storage_bytes", "1073741824",
	}
	if len(gotArgs) != len(wantArgs) {
		t.Fatalf("got %d args, want %d", len(gotArgs), len(wantArgs))
	}
	for i := range wantArgs {
		if gotArgs[i] != wantArgs[i] {
			t.Errorf("arg[%d] = %q, want %q", i, gotArgs[i], wantArgs[i])
		}
	}
	if gotArch != archARM64 {
		t.Fatalf("GOARCH = %q, want %q", gotArch, archARM64)
	}
}

// TestGokrazyConfigMatchesApplianceBuildTags asserts the gokrazy image builds
// cmd/ze with the same tag set as the Makefile ze-appliance target
// ("ze_core ze_appliance $(ZE_FEATURES)"): the ze_core base, the ze_appliance
// on-device mode tag, and every default-on feature gate declared in
// feature-gates.txt -- the single source of truth from which ZE_FEATURES is
// derived. The comparison is a set, not a fixed list, so a gate added to
// feature-gates.txt but not to the gokrazy config fails here instead of
// silently shipping a feature-less appliance image (the desync that left this
// test asserting a stale "ze_core only" after SSH was gated on).
func TestGokrazyConfigMatchesApplianceBuildTags(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "gokrazy", "ze", "config.json")) //nolint:gosec // repo fixture
	if err != nil {
		t.Fatalf("read gokrazy config: %v", err)
	}
	var cfg map[string]any
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("parse gokrazy config: %v", err)
	}
	pkgConfig, ok := cfg["PackageConfig"].(map[string]any)
	if !ok {
		t.Fatal("PackageConfig missing")
	}
	zePkg, ok := pkgConfig["codeberg.org/thomas-mangin/ze/cmd/ze"].(map[string]any)
	if !ok {
		t.Fatal("ze package config missing")
	}
	rawTags, ok := zePkg["GoBuildTags"].([]any)
	if !ok {
		t.Fatal("GoBuildTags missing for ze package")
	}
	got := make(map[string]bool, len(rawTags))
	for _, v := range rawTags {
		s, ok := v.(string)
		if !ok {
			t.Fatalf("GoBuildTags entry %v is not a string", v)
		}
		got[s] = true
	}

	want := map[string]bool{"ze_core": true, "ze_appliance": true}
	for _, tag := range readFeatureGateTags(t) {
		want[tag] = true
	}

	for tag := range want {
		if !got[tag] {
			t.Errorf("gokrazy config GoBuildTags missing %q (ze_core/ze_appliance base or a feature-gates.txt gate); add it to gokrazy/ze/config.json", tag)
		}
	}
	for tag := range got {
		if !want[tag] {
			t.Errorf("gokrazy config GoBuildTags has unexpected %q (not the appliance base and not in feature-gates.txt); remove it or declare the gate in feature-gates.txt", tag)
		}
	}
}

// readFeatureGateTags returns the unique build-tag column from feature-gates.txt
// (the single source of truth; Makefile ZE_FEATURES derives from the same file).
// Blank lines and '#' comments are ignored. Mirrors the reader in
// internal/test/runner so both stay in sync with the manifest format.
func readFeatureGateTags(t *testing.T) []string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "feature-gates.txt")) //nolint:gosec // repo fixture
	if err != nil {
		t.Fatalf("read feature-gates.txt: %v", err)
	}
	var tags []string
	seen := make(map[string]bool)
	for line := range strings.SplitSeq(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) == 0 || seen[fields[0]] {
			continue
		}
		seen[fields[0]] = true
		tags = append(tags, fields[0])
	}
	if len(tags) == 0 {
		t.Fatal("feature-gates.txt yielded no tags")
	}
	return tags
}

func TestBuildNoGokBinaryCheck(t *testing.T) {
	dir := t.TempDir()
	appDir := filepath.Join(dir, "test-app")
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		t.Fatal(err)
	}

	cfg := &applianceConfig{}
	cfg.Identity.Name = "test-app"
	cfg.Image.SizeBytes = 1073741824
	if err := saveConfig(ConfigPath(dir, "test-app"), cfg); err != nil {
		t.Fatal(err)
	}

	old := gokBuildFn
	gokBuildFn = func(args []string) error { return nil }
	defer func() { gokBuildFn = old }()

	oldExt := runExternalFn
	runExternalFn = func(name string, args ...string) ([]byte, error) {
		return nil, nil
	}
	defer func() { runExternalFn = oldExt }()

	oldE2fs := e2fsDir
	e2fsDir = "/usr/sbin"
	defer func() { e2fsDir = oldE2fs }()

	code := runGokBuild(cfg, filepath.Join(appDir, "test.img"))
	if code != exitOK {
		t.Errorf("runGokBuild returned %d, want %d", code, exitOK)
	}
}

func TestBuildGokFailure(t *testing.T) {
	old := gokBuildFn
	gokBuildFn = func(args []string) error {
		return os.ErrNotExist
	}
	defer func() { gokBuildFn = old }()

	cfg := &applianceConfig{}
	cfg.Image.SizeBytes = 1073741824

	code := runGokBuild(cfg, "/tmp/test.img")
	if code != exitError {
		t.Errorf("runGokBuild returned %d on failure, want %d", code, exitError)
	}
}

// TestBuildValidatesConfig verifies buildOne rejects an out-of-range hugepage
// reservation before invoking the gok builder (AC-4, AC-7). The validation runs
// right after LoadConfig, so the builder must never be reached.
func TestBuildValidatesConfig(t *testing.T) {
	oldBase := baseDir
	baseDir = t.TempDir()
	defer func() { baseDir = oldBase }()

	appDir := filepath.Join(baseDir, "hp-app")
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := DefaultConfig("hp-app")
	cfg.Image.Hugepages = &Hugepages{PageSize: "2M", Count: 0} // invalid: minimum is 1
	if err := saveConfig(ConfigPath(baseDir, "hp-app"), &cfg); err != nil {
		t.Fatal(err)
	}

	called := false
	old := gokBuildFn
	gokBuildFn = func(_ []string) error { called = true; return nil }
	defer func() { gokBuildFn = old }()

	if code := buildOne("hp-app"); code != exitError {
		t.Errorf("buildOne with invalid hugepages = %d, want exitError (%d)", code, exitError)
	}
	if called {
		t.Error("gok builder was invoked despite an invalid config")
	}
}

func TestGokSizeArg(t *testing.T) {
	tests := []struct {
		input int64
		want  string
	}{
		{0, "0"},
		{1073741824, "1073741824"},
		{-1, "-1"},
		{4294967296, "4294967296"},
	}
	for _, tt := range tests {
		got := gokSizeArg(tt.input)
		if got != tt.want {
			t.Errorf("gokSizeArg(%d) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestInjectZeFSUsesRunExternalFn(t *testing.T) {
	var calledNames []string
	var mkfsArgs []string
	old := runExternalFn
	defer func() { runExternalFn = old }()

	imgDir := t.TempDir()
	imgPath := filepath.Join(imgDir, "test.img")

	// Image must be large enough for extractPartition (Go ReadAt) to succeed.
	// Partition at LBA 8..15 → offset 4096, size 4096 → image >= 8192 bytes.
	img := make([]byte, 8192)
	for i := range 16 {
		img[1024+i] = 0xAA
	}
	img[1024+32] = 0x08 // Start LBA = 8
	img[1024+40] = 0x0F // End LBA = 15

	if err := os.WriteFile(imgPath, img, 0o644); err != nil {
		t.Fatal(err)
	}

	dbContent := []byte("test-database-zefs-content")
	dbPath := filepath.Join(imgDir, "database.zefs")
	if err := os.WriteFile(dbPath, dbContent, 0o644); err != nil {
		t.Fatal(err)
	}

	e2fs := t.TempDir()
	if err := os.WriteFile(filepath.Join(e2fs, "mkfs.ext4"), nil, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(e2fs, "debugfs"), nil, 0o755); err != nil {
		t.Fatal(err)
	}
	oldE2fs := e2fsDir
	e2fsDir = e2fs
	defer func() { e2fsDir = oldE2fs }()

	// debugfs write is mocked: simulate success by injecting db bytes into
	// the perm temp file so verifyInjectedDB (bytes.Contains) passes.
	runExternalFn = func(name string, args ...string) ([]byte, error) {
		base := filepath.Base(name)
		calledNames = append(calledNames, base)
		if base == "mkfs.ext4" {
			mkfsArgs = append([]string{}, args...)
		}
		if base == "debugfs" {
			for _, a := range args {
				if len(a) > 6 && a[:6] == "write " {
					// Append db content to the perm temp file so verify passes.
					permTmp := args[len(args)-1]
					f, _ := os.OpenFile(permTmp, os.O_APPEND|os.O_WRONLY, 0) //nolint:gosec,errcheck // test
					if f != nil {
						f.Write(dbContent) //nolint:errcheck // test
						f.Close()          //nolint:errcheck // test
					}
				}
			}
		}
		return nil, nil
	}

	code := injectZeFS(imgPath, dbPath, "")
	if code != exitOK {
		t.Fatalf("injectZeFS returned %d, want %d", code, exitOK)
	}

	hasMkfs := false
	hasDebugfs := false
	for _, n := range calledNames {
		if n == "mkfs.ext4" {
			hasMkfs = true
		}
		if n == "debugfs" {
			hasDebugfs = true
		}
	}
	if !hasMkfs {
		t.Error("injectZeFS did not call mkfs.ext4 via runExternalFn")
	}
	if !hasDebugfs {
		t.Error("injectZeFS did not call debugfs via runExternalFn")
	}

	// mkfs.ext4 must force 4096-byte blocks. For partition LBA 8..15:
	// permSize = 4096, perm4K = 1, permBlocks = 1.
	foundB := false
	for i := 0; i+1 < len(mkfsArgs); i++ {
		if mkfsArgs[i] == "-b" && mkfsArgs[i+1] == "4096" {
			foundB = true
		}
	}
	if !foundB {
		t.Errorf("mkfs.ext4 missing -b 4096: %v", mkfsArgs)
	}
	if len(mkfsArgs) == 0 || mkfsArgs[len(mkfsArgs)-1] != "1" {
		t.Errorf("mkfs.ext4 block count = last arg of %v, want 1 (perm4K)", mkfsArgs)
	}
}

// VALIDATES: AC-1 wiring test — injectZeFS fails when debugfs write silently drops the file.
func TestInjectZeFSFailsWhenWriteSilentlyDropped(t *testing.T) {
	imgDir := t.TempDir()
	imgPath := filepath.Join(imgDir, "test.img")

	img := make([]byte, 8192)
	for i := range 16 {
		img[1024+i] = 0xAA
	}
	img[1024+32] = 0x08
	img[1024+40] = 0x0F

	if err := os.WriteFile(imgPath, img, 0o644); err != nil {
		t.Fatal(err)
	}
	dbPath := filepath.Join(imgDir, "database.zefs")
	if err := os.WriteFile(dbPath, []byte("unique-zefs-content"), 0o644); err != nil {
		t.Fatal(err)
	}

	e2fs := t.TempDir()
	if err := os.WriteFile(filepath.Join(e2fs, "mkfs.ext4"), nil, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(e2fs, "debugfs"), nil, 0o755); err != nil {
		t.Fatal(err)
	}
	oldE2fs := e2fsDir
	e2fsDir = e2fs
	defer func() { e2fsDir = oldE2fs }()

	old := runExternalFn
	runExternalFn = func(_ string, _ ...string) ([]byte, error) {
		return nil, nil
	}
	defer func() { runExternalFn = old }()

	code := injectZeFS(imgPath, dbPath, "")
	if code != exitError {
		t.Fatalf("injectZeFS should fail when debugfs write is silent-dropped, got %d want %d", code, exitError)
	}

	if _, err := os.Stat(imgPath); err != nil {
		t.Errorf("image file should still exist (caller removes it), got: %v", err)
	}
}

// VALIDATES: AC-5 — exact mkfs.ext4 argument vector is pinned.
func TestInjectZeFSMkfsArgsPinned(t *testing.T) {
	imgDir := t.TempDir()
	imgPath := filepath.Join(imgDir, "test.img")

	img := make([]byte, 8192)
	for i := range 16 {
		img[1024+i] = 0xAA
	}
	img[1024+32] = 0x08
	img[1024+40] = 0x0F

	if err := os.WriteFile(imgPath, img, 0o644); err != nil {
		t.Fatal(err)
	}
	dbContent := []byte("mkfs-test-db")
	dbPath := filepath.Join(imgDir, "database.zefs")
	if err := os.WriteFile(dbPath, dbContent, 0o644); err != nil {
		t.Fatal(err)
	}

	e2fs := t.TempDir()
	if err := os.WriteFile(filepath.Join(e2fs, "mkfs.ext4"), nil, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(e2fs, "debugfs"), nil, 0o755); err != nil {
		t.Fatal(err)
	}
	oldE2fs := e2fsDir
	e2fsDir = e2fs
	defer func() { e2fsDir = oldE2fs }()

	var mkfsArgs []string
	old := runExternalFn
	runExternalFn = func(name string, args ...string) ([]byte, error) {
		if filepath.Base(name) == "mkfs.ext4" {
			mkfsArgs = append([]string{}, args...)
		}
		if filepath.Base(name) == "debugfs" {
			for _, a := range args {
				if len(a) > 6 && a[:6] == "write " {
					permTmp := args[len(args)-1]
					f, _ := os.OpenFile(permTmp, os.O_APPEND|os.O_WRONLY, 0) //nolint:gosec,errcheck // test
					if f != nil {
						f.Write(dbContent) //nolint:errcheck // test
						f.Close()          //nolint:errcheck // test
					}
				}
			}
		}
		return nil, nil
	}
	defer func() { runExternalFn = old }()

	code := injectZeFS(imgPath, dbPath, "")
	if code != exitOK {
		t.Fatalf("injectZeFS returned %d, want %d", code, exitOK)
	}

	// Partition LBA 8..15: permOff=4096, permBlocks=1
	wantArgs := []string{
		"-q", "-F", "-O", "^metadata_csum",
		"-b", "4096",
		"-E", "offset=4096",
		imgPath, "1",
	}
	if len(mkfsArgs) != len(wantArgs) {
		t.Fatalf("mkfs.ext4 args = %v, want %v", mkfsArgs, wantArgs)
	}
	for i := range wantArgs {
		if mkfsArgs[i] != wantArgs[i] {
			t.Errorf("mkfs.ext4 arg[%d] = %q, want %q", i, mkfsArgs[i], wantArgs[i])
		}
	}
}

func TestFindLastPartitionRejectsCorruptGPT(t *testing.T) {
	imgDir := t.TempDir()
	imgPath := filepath.Join(imgDir, "bad.img")

	img := make([]byte, 4096)
	for i := range 16 {
		img[1024+i] = 0xAA // non-zero partition type GUID
	}
	// startLBA = 4096 (0x1000)
	img[1024+32] = 0x00
	img[1024+33] = 0x10
	// endLBA = 2048 (0x0800), deliberately < startLBA
	img[1024+40] = 0x00
	img[1024+41] = 0x08

	if err := os.WriteFile(imgPath, img, 0o644); err != nil {
		t.Fatal(err)
	}

	if _, _, err := findLastPartition(imgPath); err == nil {
		t.Fatal("findLastPartition accepted a GPT with endLBA < startLBA; expected error")
	}
}

func TestBuildOneRejectsInvalidName(t *testing.T) {
	oldBase := baseDir
	baseDir = t.TempDir()
	defer func() { baseDir = oldBase }()

	// A name with a space would inject an extra debugfs command via dbPath.
	if code := buildOne("bad name"); code != exitError {
		t.Errorf("buildOne(%q) = %d, want exitError (%d)", "bad name", code, exitError)
	}
}

// VALIDATES: a non-empty manifest path makes injectZeFS bake ze/build.json into
// the image alongside ze/database.zefs, so `ze version` can report build identity.
func TestInjectZeFSBakesManifest(t *testing.T) {
	imgDir := t.TempDir()
	imgPath := filepath.Join(imgDir, "test.img")

	img := make([]byte, 8192)
	for i := range 16 {
		img[1024+i] = 0xAA
	}
	img[1024+32] = 0x08
	img[1024+40] = 0x0F
	if err := os.WriteFile(imgPath, img, 0o644); err != nil {
		t.Fatal(err)
	}

	dbContent := []byte("bake-test-db")
	dbPath := filepath.Join(imgDir, "database.zefs")
	if err := os.WriteFile(dbPath, dbContent, 0o644); err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(imgDir, ".build.json.baked")
	if err := os.WriteFile(manifestPath, []byte(`{"image":"ze-x.img"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	e2fs := t.TempDir()
	for _, tool := range []string{"mkfs.ext4", "debugfs"} {
		if err := os.WriteFile(filepath.Join(e2fs, tool), nil, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	oldE2fs := e2fsDir
	e2fsDir = e2fs
	defer func() { e2fsDir = oldE2fs }()

	hasSuffix := func(s, suf string) bool {
		return len(s) >= len(suf) && s[len(s)-len(suf):] == suf
	}

	var writes []string
	old := runExternalFn
	runExternalFn = func(name string, args ...string) ([]byte, error) {
		if filepath.Base(name) == "debugfs" {
			for _, a := range args {
				if len(a) > 6 && a[:6] == "write " {
					writes = append(writes, a)
					permTmp := args[len(args)-1]
					f, _ := os.OpenFile(permTmp, os.O_APPEND|os.O_WRONLY, 0) //nolint:gosec,errcheck // test
					if f != nil {
						f.Write(dbContent) //nolint:errcheck // test
						f.Close()          //nolint:errcheck // test
					}
				}
			}
		}
		return nil, nil
	}
	defer func() { runExternalFn = old }()

	if code := injectZeFS(imgPath, dbPath, manifestPath); code != exitOK {
		t.Fatalf("injectZeFS returned %d, want %d", code, exitOK)
	}

	foundDB, foundManifest := false, false
	for _, w := range writes {
		if hasSuffix(w, "ze/database.zefs") {
			foundDB = true
		}
		if hasSuffix(w, "ze/build.json") {
			foundManifest = true
		}
	}
	if !foundDB {
		t.Errorf("expected a debugfs write of ze/database.zefs, got %v", writes)
	}
	if !foundManifest {
		t.Errorf("expected a debugfs write of ze/build.json, got %v", writes)
	}
}

// VALIDATES: an empty manifest path bakes only ze/database.zefs (no build.json).
func TestInjectZeFSNoManifest(t *testing.T) {
	imgDir := t.TempDir()
	imgPath := filepath.Join(imgDir, "test.img")

	img := make([]byte, 8192)
	for i := range 16 {
		img[1024+i] = 0xAA
	}
	img[1024+32] = 0x08
	img[1024+40] = 0x0F
	if err := os.WriteFile(imgPath, img, 0o644); err != nil {
		t.Fatal(err)
	}

	dbContent := []byte("no-manifest-db")
	dbPath := filepath.Join(imgDir, "database.zefs")
	if err := os.WriteFile(dbPath, dbContent, 0o644); err != nil {
		t.Fatal(err)
	}

	e2fs := t.TempDir()
	for _, tool := range []string{"mkfs.ext4", "debugfs"} {
		if err := os.WriteFile(filepath.Join(e2fs, tool), nil, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	oldE2fs := e2fsDir
	e2fsDir = e2fs
	defer func() { e2fsDir = oldE2fs }()

	hasSuffix := func(s, suf string) bool {
		return len(s) >= len(suf) && s[len(s)-len(suf):] == suf
	}

	sawManifest := false
	old := runExternalFn
	runExternalFn = func(name string, args ...string) ([]byte, error) {
		if filepath.Base(name) == "debugfs" {
			for _, a := range args {
				if len(a) > 6 && a[:6] == "write " {
					if hasSuffix(a, "ze/build.json") {
						sawManifest = true
					}
					permTmp := args[len(args)-1]
					f, _ := os.OpenFile(permTmp, os.O_APPEND|os.O_WRONLY, 0) //nolint:gosec,errcheck // test
					if f != nil {
						f.Write(dbContent) //nolint:errcheck // test
						f.Close()          //nolint:errcheck // test
					}
				}
			}
		}
		return nil, nil
	}
	defer func() { runExternalFn = old }()

	if code := injectZeFS(imgPath, dbPath, ""); code != exitOK {
		t.Fatalf("injectZeFS returned %d, want %d", code, exitOK)
	}
	if sawManifest {
		t.Error("injectZeFS wrote ze/build.json despite an empty manifest path")
	}
}
