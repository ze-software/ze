package appliance

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

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

	cfg := &ApplianceConfig{}
	cfg.Image.Arch = archARM64
	cfg.Image.SizeBytes = 1073741824

	runGokBuild(cfg, "/tmp/test.img")

	if !called {
		t.Fatal("gokBuildFn was not called")
	}

	// runGokBuild passes absolute paths because ze-gok/gok is invoked with a
	// different working directory; relative paths would resolve incorrectly.
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

func TestGokrazyConfigBuildsStrippedZeBinary(t *testing.T) {
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
	tags, ok := zePkg["GoBuildTags"].([]any)
	if !ok {
		t.Fatal("GoBuildTags missing for ze package")
	}
	if len(tags) != 1 {
		t.Fatalf("GoBuildTags = %v, want [ze_core]", tags)
	}
	if tag, ok := tags[0].(string); !ok || tag != "ze_core" {
		t.Fatalf("GoBuildTags[0] = %v, want ze_core", tags[0])
	}
}

func TestBuildNoGokBinaryCheck(t *testing.T) {
	dir := t.TempDir()
	appDir := filepath.Join(dir, "test-app")
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		t.Fatal(err)
	}

	cfg := &ApplianceConfig{}
	cfg.Identity.Name = "test-app"
	cfg.Image.SizeBytes = 1073741824
	if err := SaveConfig(ConfigPath(dir, "test-app"), cfg); err != nil {
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

	cfg := &ApplianceConfig{}
	cfg.Image.SizeBytes = 1073741824

	code := runGokBuild(cfg, "/tmp/test.img")
	if code != exitError {
		t.Errorf("runGokBuild returned %d on failure, want %d", code, exitError)
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
	runExternalFn = func(name string, args ...string) ([]byte, error) {
		base := filepath.Base(name)
		calledNames = append(calledNames, base)
		if base == "mkfs.ext4" {
			mkfsArgs = append([]string{}, args...)
		}
		return nil, nil
	}
	defer func() { runExternalFn = old }()

	imgDir := t.TempDir()
	imgPath := filepath.Join(imgDir, "test.img")

	// Create a minimal GPT image with one partition entry.
	img := make([]byte, 4096)
	// GPT entry at LBA 2 (offset 1024), partition type GUID (non-zero).
	for i := range 16 {
		img[1024+i] = 0xAA
	}
	// Start LBA = 2048 sectors.
	img[1024+32] = 0x00
	img[1024+33] = 0x08
	// End LBA = 4096 sectors.
	img[1024+40] = 0x00
	img[1024+41] = 0x10

	if err := os.WriteFile(imgPath, img, 0o644); err != nil {
		t.Fatal(err)
	}

	dbPath := filepath.Join(imgDir, "database.zefs")
	if err := os.WriteFile(dbPath, []byte("test"), 0o644); err != nil {
		t.Fatal(err)
	}

	oldE2fs := e2fsDir
	e2fsDir = "/usr/sbin"
	defer func() { e2fsDir = oldE2fs }()

	injectZeFS(imgPath, dbPath)

	hasDD := false
	for _, n := range calledNames {
		if n == "dd" {
			hasDD = true
		}
	}
	if !hasDD {
		t.Error("injectZeFS did not call dd via runExternalFn")
	}

	// Regression: mkfs.ext4 must force 4096-byte blocks and size the filesystem
	// in those blocks. For this partition (start 2048, end 4096 → permSize
	// 1049088) perm4K = 256. The old code passed permSize/1024 = 1024 with no
	// -b, which formats a filesystem 4x the partition so /perm won't mount.
	foundB := false
	for i := 0; i+1 < len(mkfsArgs); i++ {
		if mkfsArgs[i] == "-b" && mkfsArgs[i+1] == "4096" {
			foundB = true
		}
	}
	if !foundB {
		t.Errorf("mkfs.ext4 missing -b 4096: %v", mkfsArgs)
	}
	if len(mkfsArgs) == 0 || mkfsArgs[len(mkfsArgs)-1] != "256" {
		t.Errorf("mkfs.ext4 block count = last arg of %v, want 256 (perm4K)", mkfsArgs)
	}
	for _, a := range mkfsArgs {
		if a == "1024" {
			t.Errorf("mkfs.ext4 uses permSize/1024 block count (the 4x bug): %v", mkfsArgs)
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
