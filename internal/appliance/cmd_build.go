// Design: docs/architecture/appliance/builder.md -- full image build (assemble + gok + ext4)

package appliance

import (
	"context"
	"encoding/binary"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gokrazy/tools/gok"

	"github.com/ze-software/ze/internal/appliance/instance"
	"github.com/ze-software/ze/internal/core/textbuf"
)

var errNoPartitionsFoundInGpt = errors.New("no partitions found in GPT")

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
	// Each e2fsprogs tool is resolved on its own; see resolveE2FSTool for why a
	// single shared directory was the wrong assumption.
	e2fsMkfs      = resolveE2FSTool("mkfs.ext4")
	e2fsDebugfs   = resolveE2FSTool("debugfs")
	e2fsE2fsck    = resolveE2FSTool("e2fsck")
	runExternalFn = runExternal
	gokBuildFn    = runGokInProcess
)

// e2fsSearchDirs are the directories searched for each e2fsprogs tool, in order.
//
// The Homebrew directories come first and are resolved from the running host
// (brewPrefixes), never written as a literal: e2fsprogs is keg-only, so it is
// absent from <prefix>/bin, and the prefix itself differs between an Apple
// Silicon and an Intel Mac.
func e2fsSearchDirs() []string {
	dirs := brewKegDirs("e2fsprogs", "sbin")
	for _, prefix := range brewPrefixes() {
		dirs = append(dirs, filepath.Join(prefix, "sbin"))
	}
	// /usr/local/sbin is in the tail because mk/gokrazy.mk searches it for the
	// same two tools, and leaving it out here let `make ze-gokrazy` and `ze
	// appliance build` pick different mkfs.ext4 binaries on one host.
	//
	// On macOS it has usually arrived already: /usr/local is one of the two
	// Homebrew defaults, so the prefix loop above contributes <prefix>/sbin for
	// it. Hence the dedup, which keeps the FIRST occurrence and leaves the
	// order the loops produced.
	dirs = append(dirs, "/usr/sbin", "/sbin", "/usr/local/sbin")

	seen := make(map[string]bool, len(dirs))
	unique := dirs[:0]
	for _, d := range dirs {
		if seen[d] {
			continue
		}
		seen[d] = true
		unique = append(unique, d)
	}
	return unique
}

// resolveE2FSTool returns the absolute path of one e2fsprogs tool, or "" when it
// is not installed.
//
// Each tool is resolved INDEPENDENTLY. Requiring them all in one directory was
// wrong on any distribution that splits the package: Alpine ships debugfs in
// e2fsprogs-extra, so with both packages installed no single directory held both
// mkfs.ext4 and debugfs, the whole lookup returned "", and every tool read as
// absent -- injectZeFS logged "e2fsck not found" and "debugfs write silently
// failed" while the binaries sat on disk. PATH is consulted last so a tool
// installed anywhere else is still found.
func resolveE2FSTool(name string) string {
	for _, dir := range e2fsSearchDirs() {
		p := filepath.Join(dir, name)
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	if p, err := exec.LookPath(name); err == nil {
		return p
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

	// Validate the appliance name before it flows into filesystem paths and,
	// via dbPath, into debugfs -R command arguments (a space or metacharacter
	// would inject an extra debugfs command). init enforces this, but build is
	// reachable on a directory created by other means, so re-check here.
	if !validNameRe.MatchString(name) {
		fmt.Fprintf(os.Stderr, "error: invalid appliance name %q: must match %s\n", name, validNameRe.String())
		return exitError
	}

	cfg, err := LoadConfig(ConfigPath(dir, name))
	if err == nil {
		// Validate before building so an out-of-range hugepage reservation (or
		// any other invalid field) is rejected here rather than baked into an
		// image or silently clamped at boot (AC-4, AC-7). init validates too,
		// but build is reachable on directories created by other means.
		err = cfg.Validate()
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return exitError
	}

	var passphrase []byte
	if isEncrypted(dir, name) {
		var resolveErr error
		passphrase, _, resolveErr = ResolvePassphrase(nil)
		if resolveErr != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", resolveErr)
			return exitError
		}
		defer ZeroBytes(passphrase)
	}

	// Name the tool that is missing: "e2fsprogs not found" sent readers looking
	// for the package when, on a split-package distribution, only debugfs was
	// absent (Alpine keeps it in e2fsprogs-extra).
	if e2fsMkfs == "" || e2fsDebugfs == "" {
		fmt.Fprintf(os.Stderr, "error: e2fsprogs tools not found (mkfs.ext4=%q debugfs=%q); install e2fsprogs (and e2fsprogs-extra on Alpine, brew install e2fsprogs on macOS)\n",
			e2fsMkfs, e2fsDebugfs)
		return exitError
	}

	dbPath := databasePath(dir, name)
	if code := assembleZeFS(dir, name, cfg, passphrase, dbPath); code != exitOK {
		return code
	}
	defer os.Remove(dbPath) //nolint:errcheck // cleanup after build

	ts := imageTimestamp()
	imgName := imageFileName(ts)
	imgPath := filepath.Join(AppliancePath(dir, name), imgName)

	if code := runGokBuild(cfg, imgPath); code != exitOK {
		return code
	}

	// Bake an identity manifest into the image so `ze version` on the installed
	// box can report which build it is running. It omits image-sha256, which
	// would be self-referential (baking changes the image); the external
	// build.json below carries the full checksum for integrity.
	seedConfig, _ := resolveSeedConfig(dir, name, cfg)
	bakedPath := filepath.Join(AppliancePath(dir, name), ".build.json.baked")
	if writeErr := writeManifest(bakedPath, &BuildManifest{
		Appliance:  name,
		Timestamp:  ts,
		ZeVersion:  "dev",
		Arch:       cfg.Image.Arch,
		ConfigHash: configHash(seedConfig),
		Image:      imgName,
	}); writeErr != nil {
		slog.Warn("prepare baked build manifest failed (non-fatal)", "error", writeErr)
		bakedPath = ""
	} else {
		defer os.Remove(bakedPath) //nolint:errcheck // temp cleanup
	}

	if code := injectZeFS(imgPath, dbPath, bakedPath); code != exitOK {
		os.Remove(imgPath) //nolint:errcheck // cleanup failed image
		return code
	}

	var tbChk textbuf.Buffer
	checksumPath := tbChk.Str(imgPath).Str(".sha256").String()
	imgHash, hashErr := writeImageChecksum(imgPath, checksumPath)
	if hashErr != nil {
		fmt.Fprintf(os.Stderr, "warning: checksum: %v\n", hashErr)
	}

	manifest := &BuildManifest{
		Appliance:   name,
		Timestamp:   ts,
		ZeVersion:   "dev",
		Arch:        cfg.Image.Arch,
		ConfigHash:  configHash(seedConfig),
		Image:       imgName,
		ImageSHA256: imgHash,
	}

	manifestPath := filepath.Join(AppliancePath(dir, name), "build.json")
	if writeErr := writeManifest(manifestPath, manifest); writeErr != nil {
		fmt.Fprintf(os.Stderr, "error: write manifest: %v\n", writeErr)
	}

	fmt.Printf("image ready: %s\n", imgPath)
	return exitOK
}

func gokSizeArg(n int64) string {
	var buf []byte
	buf = strconv.AppendInt(buf, n, 10)
	return string(buf)
}

// ensureModcacheRW appends -modcacherw to GOFLAGS (preserving existing
// flags) so go subprocesses that download into the checked-in
// gokrazy/modcache leave it user-writable. Go's default read-only cache
// permissions break later git checkouts/rebases that must delete or
// overwrite modcache files (git cannot unlink inside r-x directories).
func ensureModcacheRW() error {
	goflags := os.Getenv("GOFLAGS")
	if strings.Contains(goflags, "-modcacherw") {
		return nil
	}
	if goflags == "" {
		return os.Setenv("GOFLAGS", "-modcacherw")
	}
	var tb textbuf.Buffer
	tb.Str(goflags).Byte(' ').Str("-modcacherw")
	return os.Setenv("GOFLAGS", tb.String())
}

// runGokInProcess runs the gokrazy builder (gok) embedded in-process rather
// than shelling out to the ze-gok binary. gok still spawns its own
// `go build`/`go list` subprocesses for the target packages, and those resolve
// modules from the repo-local gokrazy/modcache, so GOMODCACHE is pointed there
// (gok hardcodes -mod=mod, so it always reads the module cache). Must run from
// the ze source tree root, where gokrazy/{ze,modcache} live.
func runGokInProcess(args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), buildTimeout)
	defer cancel()

	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getwd: %w", err)
	}
	modcache := filepath.Join(wd, "gokrazy", "modcache")
	if _, statErr := os.Stat(modcache); statErr != nil {
		return fmt.Errorf("gokrazy module cache not found at %s; run from the ze source tree root: %w", modcache, statErr)
	}
	if setErr := os.Setenv("GOMODCACHE", modcache); setErr != nil {
		return fmt.Errorf("set GOMODCACHE: %w", setErr)
	}
	if setErr := ensureModcacheRW(); setErr != nil {
		return fmt.Errorf("set GOFLAGS: %w", setErr)
	}
	// Resolve strictly from the checked-in modcache. gok reads the ambient GOPROXY
	// and does not force offline, so a module missing from the builddir/modcache
	// would silently resolve to a NEWER version than the pins choose. off makes
	// that a loud failure instead. Explicit GOPROXY wins (ze-gokrazy-deps is a
	// separate, network-using target). Mirrors cmd/ze-gok/main.go.
	if os.Getenv("GOPROXY") == "" {
		if setErr := os.Setenv("GOPROXY", "off"); setErr != nil {
			return fmt.Errorf("set GOPROXY: %w", setErr)
		}
	}

	return gok.Context{
		Stdout: os.Stdout,
		Stderr: os.Stderr,
		Args:   args,
	}.Execute(ctx)
}

func runGokBuild(cfg *applianceConfig, imgPath string) int {
	fmt.Fprintf(os.Stderr, "building gokrazy image...\n")

	// gok resolves modules from the repo-local gokrazy/modcache, so relative
	// paths would resolve against the wrong root. Pass absolute paths.
	parentDir, cleanup, err := resolveBuildParentDir(cfg)
	defer cleanup()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: resolve gokrazy parent dir: %v\n", err)
		return exitError
	}
	absImg, err := filepath.Abs(imgPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: resolve image path: %v\n", err)
		return exitError
	}

	oldArch, hadArch := os.LookupEnv("GOARCH")
	if err := os.Setenv("GOARCH", cfg.Image.Arch); err != nil {
		fmt.Fprintf(os.Stderr, "error: set GOARCH: %v\n", err)
		return exitError
	}
	defer func() {
		if hadArch {
			_ = os.Setenv("GOARCH", oldArch)
			return
		}
		_ = os.Unsetenv("GOARCH")
	}()

	if err := gokBuildFn([]string{
		"--parent_dir", parentDir,
		"-i", instance.Name,
		"overwrite",
		"--full", absImg,
		"--target_storage_bytes", gokSizeArg(cfg.Image.SizeBytes),
	}); err != nil {
		fmt.Fprintf(os.Stderr, "error: gok build failed: %v\n", err)
		return exitError
	}

	return exitOK
}

// injectZeFS writes the credential database into the image /perm partition and,
// when manifestPath is non-empty, also bakes the build manifest at
// ze/build.json so an installed box can report its build identity via
// `ze version`. The manifest write is non-fatal: it is a diagnostic aid, not
// required for the image to boot or for credentials to load.
func injectZeFS(imgPath, dbPath, manifestPath string) int {
	permOff, permSize, err := findLastPartition(imgPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: find /perm partition: %v\n", err)
		return exitError
	}

	perm4K := permSize / 4096
	// Filesystem size in 4096-byte blocks. mkfs is forced to -b 4096 below, so
	// the block count must be in 4096-byte units. Using permSize/1024 here with
	// 4096-byte blocks would format a filesystem 4x the partition, overrunning
	// /perm so it fails to mount on the target.
	permBlocks := perm4K

	mkfs := e2fsMkfs
	debugfs := e2fsDebugfs

	fmt.Fprintf(os.Stderr, "formatting /perm partition...\n")
	if _, err := runExternalFn(mkfs, "-q", "-F", "-O", "^metadata_csum",
		"-b", "4096",
		"-E", "offset="+strconv.FormatInt(permOff, 10),
		imgPath, strconv.FormatInt(permBlocks, 10)); err != nil {
		fmt.Fprintf(os.Stderr, "error: mkfs.ext4: %v\n", err)
		return exitError
	}

	var tbPerm textbuf.Buffer
	permImg := tbPerm.Str(imgPath).Str(".perm.tmp").String()
	defer os.Remove(permImg) //nolint:errcheck // temp file cleanup

	fmt.Fprintf(os.Stderr, "injecting credentials into /perm...\n")
	permData, extractErr := extractPartition(imgPath, permOff, permSize)
	if extractErr != nil {
		slog.Error("extract /perm partition", "error", extractErr)
		return exitError
	}
	if writeErr := os.WriteFile(permImg, permData, 0o600); writeErr != nil {
		slog.Error("write perm temp file", "error", writeErr)
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

	if manifestPath != "" {
		var tb textbuf.Buffer
		manifestCmd := tb.Str("write ").Str(manifestPath).Str(" ze/build.json").String()
		if _, err := runExternalFn(debugfs, "-w", "-R", manifestCmd, permImg); err != nil {
			slog.Warn("bake build.json into image failed (non-fatal)", "error", err)
		}
	}

	if err := verifyInject(permImg, dbPath); err != nil {
		return exitError
	}

	updatedPerm, readErr := os.ReadFile(permImg) //nolint:gosec // build-controlled temp file
	if readErr != nil {
		slog.Error("read perm temp file", "error", readErr)
		return exitError
	}
	if wbErr := writePartition(imgPath, updatedPerm, permOff); wbErr != nil {
		slog.Error("write /perm back", "error", wbErr)
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
		return 0, 0, errNoPartitionsFoundInGpt
	}

	// Guard against a corrupt/crafted GPT where endLBA < startLBA: the size
	// computation below subtracts in uint64, so it would underflow to a huge
	// value and then cast to a negative int64.
	if lastEnd < lastStart {
		return 0, 0, fmt.Errorf("GPT corrupt: last partition end LBA %d < start LBA %d", lastEnd, lastStart)
	}

	return int64(lastStart) * gptSectorSize, int64(lastEnd-lastStart+1) * gptSectorSize, nil
}

// uniformArch returns the single image architecture shared by every named
// appliance, or an error naming the first appliance that differs.
//
// This is a fail-closed guard on a process-wide memoization in the vendored
// builder. packer.Env() computes the target build environment once behind a
// sync.Once (vendor/github.com/gokrazy/tools/packer/gotool.go), and that
// memoized slice is handed to every target compile. buildAll runs in a single
// process and sets GOARCH per appliance, so appliance two onwards would compile
// for appliance one's architecture, while packer.TargetArch() reads GOARCH fresh
// and lays their images out for their own. Nothing fails: the result is an image
// whose partition layout and binaries disagree.
//
// Refusing is the honest option. Building each appliance in its own process
// would also work, but that is a larger change than this guard, and a wrong
// image shipped silently is worse than a run that stops and says why.
func uniformArch(dir string, names []string) (string, error) {
	var arch, archOwner string
	for _, name := range names {
		cfg, err := LoadConfig(ConfigPath(dir, name))
		if err != nil {
			return "", fmt.Errorf("read config for %s: %w", name, err)
		}
		if arch == "" {
			arch, archOwner = cfg.Image.Arch, name
			continue
		}
		if cfg.Image.Arch != arch {
			return "", fmt.Errorf(
				"build --all cannot mix architectures in one run: %s is %s but %s is %s; "+
					"build each architecture separately (ze appliance build <name>)",
				archOwner, arch, name, cfg.Image.Arch)
		}
	}
	return arch, nil
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

	// Refuse a mixed-architecture set BEFORE writing anything: every appliance in
	// this run would compile for the first one's arch. See uniformArch.
	if _, err := uniformArch(dir, names); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
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
