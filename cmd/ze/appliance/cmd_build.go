// Design: plan/learned/675-appliance-1-builder.md — full image build (assemble + gok + ext4)

package appliance

import (
	"context"
	"encoding/binary"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func init() {
	cmdBuild = runBuild
}

const (
	gptEntryLBA   = 2
	gptSectorSize = 512
	gptEntrySize  = 128
	gptMaxEntries = 128
	buildTimeout  = 10 * time.Minute
)

var (
	gokBinary     = "bin/gok"
	e2fsDir       = resolveE2FSDir()
	runExternalFn = runExternal
)

func resolveE2FSDir() string {
	dirs := []string{
		"/opt/homebrew/sbin",
		"/usr/sbin",
		"/sbin",
	}
	// Homebrew doesn't symlink e2fsprogs to /opt/homebrew/sbin on macOS;
	// pick the latest versioned Cellar path if present.
	if matches, _ := filepath.Glob("/opt/homebrew/Cellar/e2fsprogs/*/sbin"); len(matches) > 0 {
		dirs = append([]string{matches[len(matches)-1]}, dirs...)
	}
	for _, dir := range dirs {
		if _, err := os.Stat(filepath.Join(dir, "mkfs.ext4")); err != nil {
			continue
		}
		if _, err := os.Stat(filepath.Join(dir, "debugfs")); err != nil {
			continue
		}
		return dir
	}
	return ""
}

func runExternal(name string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), buildTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...) //nolint:gosec // controlled invocation
	cmd.Stderr = os.Stderr
	return cmd.Output()
}

func runBuild(args []string) int {
	fs := flag.NewFlagSet("appliance build", flag.ContinueOnError)
	allFlag := fs.Bool("all", false, "Build all appliances in the directory")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: ze appliance build [options] <name>\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		return exitError
	}

	if *allFlag {
		return buildAll()
	}

	if fs.NArg() < 1 {
		fmt.Fprintf(os.Stderr, "error: requires <name> or --all\n")
		fs.Usage()
		return exitError
	}

	return buildOne(fs.Arg(0))
}

func buildOne(name string) int {
	dir := getBaseDir()

	cfg, err := LoadConfig(ConfigPath(dir, name))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return exitError
	}

	var passphrase []byte
	if IsEncrypted(dir, name) {
		var resolveErr error
		passphrase, _, resolveErr = ResolvePassphrase(nil)
		if resolveErr != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", resolveErr)
			return exitError
		}
		defer ZeroBytes(passphrase)
	}

	if _, statErr := os.Stat(gokBinary); statErr != nil {
		fmt.Fprintf(os.Stderr, "error: %s not found (run: make bin/gok)\n", gokBinary)
		return exitError
	}

	if e2fsDir == "" {
		fmt.Fprintf(os.Stderr, "error: e2fsprogs not found (brew install e2fsprogs)\n")
		return exitError
	}

	dbPath := DatabasePath(dir, name)
	if code := assembleZeFS(dir, name, cfg, passphrase, dbPath); code != exitOK {
		return code
	}
	defer os.Remove(dbPath) //nolint:errcheck // cleanup after build

	ts := ImageTimestamp()
	imgName := ImageFileName(ts)
	imgPath := filepath.Join(AppliancePath(dir, name), imgName)

	if code := runGokBuild(cfg, imgPath); code != exitOK {
		return code
	}

	if code := injectZeFS(imgPath, dbPath); code != exitOK {
		os.Remove(imgPath) //nolint:errcheck // cleanup failed image
		return code
	}

	checksumPath := imgPath + ".sha256"
	imgHash, hashErr := WriteImageChecksum(imgPath, checksumPath)
	if hashErr != nil {
		fmt.Fprintf(os.Stderr, "warning: checksum: %v\n", hashErr)
	}

	seedConfig, _ := resolveSeedConfig(dir, name, cfg)
	manifest := &BuildManifest{
		Appliance:   name,
		Timestamp:   ts,
		ZeVersion:   "dev",
		Arch:        cfg.Image.Arch,
		ConfigHash:  ConfigHash(seedConfig),
		Image:       imgName,
		ImageSHA256: imgHash,
	}

	manifestPath := filepath.Join(AppliancePath(dir, name), "build.json")
	if writeErr := WriteManifest(manifestPath, manifest); writeErr != nil {
		fmt.Fprintf(os.Stderr, "error: write manifest: %v\n", writeErr)
	}

	fmt.Printf("image ready: %s\n", imgPath)
	return exitOK
}

func runGokBuild(cfg *ApplianceConfig, imgPath string) int {
	fmt.Fprintf(os.Stderr, "building gokrazy image...\n")

	_, err := runExternalFn(gokBinary,
		"--parent_dir", "gokrazy",
		"-i", "ze",
		"overwrite",
		"--full", imgPath,
		"--target_storage_bytes", strconv.FormatInt(cfg.Image.SizeBytes, 10),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: gok build failed: %v\n", err)
		return exitError
	}

	return exitOK
}

func injectZeFS(imgPath, dbPath string) int {
	permOff, permSize, err := findLastPartition(imgPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: find /perm partition: %v\n", err)
		return exitError
	}

	permBlocks := permSize / 1024
	perm4K := permSize / 4096
	permSkip := permOff / 4096

	mkfs := filepath.Join(e2fsDir, "mkfs.ext4")
	debugfs := filepath.Join(e2fsDir, "debugfs")

	fmt.Fprintf(os.Stderr, "formatting /perm partition...\n")
	if _, err := runExternalFn(mkfs, "-q", "-F", "-O", "^metadata_csum",
		"-E", "offset="+strconv.FormatInt(permOff, 10),
		imgPath, strconv.FormatInt(permBlocks, 10)); err != nil {
		fmt.Fprintf(os.Stderr, "error: mkfs.ext4: %v\n", err)
		return exitError
	}

	permImg := imgPath + ".perm.tmp"
	defer os.Remove(permImg) //nolint:errcheck // temp file cleanup

	fmt.Fprintf(os.Stderr, "injecting credentials into /perm...\n")
	if _, err := runExternalFn("dd",
		"if="+imgPath, "of="+permImg,
		"bs=4096",
		"skip="+strconv.FormatInt(permSkip, 10),
		"count="+strconv.FormatInt(perm4K, 10),
	); err != nil {
		fmt.Fprintf(os.Stderr, "error: extract /perm: %v\n", err)
		return exitError
	}

	if _, err := runExternalFn(debugfs, "-w", "-R", "mkdir ze", permImg); err != nil {
		fmt.Fprintf(os.Stderr, "error: debugfs mkdir: %v\n", err)
		return exitError
	}

	writeCmd := fmt.Sprintf("write %s ze/database.zefs", dbPath)
	if _, err := runExternalFn(debugfs, "-w", "-R", writeCmd, permImg); err != nil {
		fmt.Fprintf(os.Stderr, "error: debugfs write: %v\n", err)
		return exitError
	}

	if _, err := runExternalFn("dd",
		"if="+permImg, "of="+imgPath,
		"bs=4096",
		"seek="+strconv.FormatInt(permSkip, 10),
		"conv=notrunc",
	); err != nil {
		fmt.Fprintf(os.Stderr, "error: write /perm back: %v\n", err)
		return exitError
	}

	return exitOK
}

func findLastPartition(imgPath string) (offsetBytes, sizeBytes int64, err error) {
	f, err := os.Open(imgPath) //nolint:gosec // user-controlled image path
	if err != nil {
		return 0, 0, fmt.Errorf("open image: %w", err)
	}
	defer f.Close() //nolint:errcheck // read-only

	buf := make([]byte, gptEntrySize)
	var lastStart, lastEnd uint64

	for i := range gptMaxEntries {
		off := int64(gptEntryLBA*gptSectorSize) + int64(i)*gptEntrySize
		n, readErr := f.ReadAt(buf, off)
		if readErr != nil || n < gptEntrySize {
			break
		}

		allZero := true
		for _, b := range buf[0:16] {
			if b != 0 {
				allZero = false
				break
			}
		}
		if allZero {
			break
		}

		startLBA := binary.LittleEndian.Uint64(buf[32:40])
		endLBA := binary.LittleEndian.Uint64(buf[40:48])
		if startLBA > lastStart {
			lastStart = startLBA
			lastEnd = endLBA
		}
	}

	if lastStart == 0 {
		return 0, 0, fmt.Errorf("no partitions found in GPT")
	}

	return int64(lastStart) * gptSectorSize, int64(lastEnd-lastStart+1) * gptSectorSize, nil
}

func buildAll() int {
	dir := getBaseDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: read %s: %v\n", dir, err)
		return exitError
	}

	var names []string
	for _, e := range entries {
		if !e.IsDir() || e.Name() == sharedDirName || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		if _, loadErr := LoadConfig(ConfigPath(dir, e.Name())); loadErr == nil {
			names = append(names, e.Name())
		}
	}

	if len(names) == 0 {
		fmt.Fprintf(os.Stderr, "no appliances found in %s\n", dir)
		return exitError
	}

	succeeded, failed := 0, 0
	for _, name := range names {
		fmt.Fprintf(os.Stderr, "building %s...\n", name)
		if code := buildOne(name); code != exitOK {
			fmt.Fprintf(os.Stderr, "FAILED: %s\n", name)
			failed++
		} else {
			succeeded++
		}
	}

	fmt.Printf("%d succeeded, %d failed\n", succeeded, failed)
	if failed > 0 {
		return exitError
	}
	return exitOK
}
