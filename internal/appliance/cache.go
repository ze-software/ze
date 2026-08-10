// Design: docs/architecture/appliance/build-artifacts.md -- XDG cache resolution and artifact download

package appliance

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
)

const (
	cacheSubdir           = "ze"
	kernelCacheDir        = "installer-kernel"
	runtimeKernelCacheDir = "runtime-kernel"
	initrdCacheDir        = "installer-initrd"
	kernelFileName        = "Image"
	initrdFileName        = "initrd.img.gz"
	checksumSuffix        = ".sha256"
	downloadTimeout       = 10 * time.Minute
	cacheDirPerm          = 0o755
	checksumHexLen        = 64
	minArtifactBytes      = 1
	evictKeepDefault      = 2
)

var httpGetFn = defaultHTTPGet

// Eviction tunables (overridable in tests). evictGrace protects entries touched recently,
// which a concurrent run may be materializing or booting (R-1/AC-8): leaving garbage is
// preferred over deleting an in-use artifact.
var (
	evictGrace = 10 * time.Minute
	evictNow   = time.Now
)

func defaultHTTPGet(url string) (*http.Response, error) {
	ctx, cancel := context.WithTimeout(context.Background(), downloadTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody) //nolint:gosec // URL validated by caller
	if err != nil {
		return nil, err
	}
	return http.DefaultClient.Do(req) //nolint:bodyclose // caller closes
}

func resolveCacheDir() string {
	cacheHome := os.Getenv("XDG_CACHE_HOME")
	if cacheHome != "" {
		return filepath.Join(cacheHome, cacheSubdir)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".", ".cache", cacheSubdir)
	}
	return filepath.Join(home, ".cache", cacheSubdir)
}

func kernelCachePath(version, variant string) string {
	var tb textbuf.Buffer
	return filepath.Join(resolveCacheDir(), kernelCacheDir, tb.Str(version).Byte('-').Str(variant).String(), kernelFileName)
}

// kernelTreeCachePath returns the runtime kernel's cache DIRECTORY. The runtime
// artifact is a tree (vmlinuz + lib/modules + DTBs + overlays), so unlike the
// single-file installer cache the path is the directory itself.
func kernelTreeCachePath(version, variant string) string {
	var tb textbuf.Buffer
	return filepath.Join(resolveCacheDir(), runtimeKernelCacheDir, tb.Str(version).Byte('-').Str(variant).String())
}

// kernelBuilderScripts lists every Python module under kernelBuilderDir, sorted
// so the hash is stable regardless of directory walk order. ok is false if the
// directory cannot be read, so the caller falls back to a builder-independent
// variant.
//
// Found rather than named, for the reason tools/installer-kernel/Makefile globs
// the same directory: a hand-written list stops covering the builder the day
// somebody adds a module to it. This one had already stopped. It named build.py,
// run.py and ksource.py, so an edit to qemu-build.py -- the whole QEMU backend --
// left the key unchanged and served a kernel the old backend had built.
func kernelBuilderScripts() (paths []string, ok bool) {
	entries, err := os.ReadDir(kernelBuilderDir)
	if err != nil {
		return nil, false
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".py") {
			continue
		}
		paths = append(paths, filepath.Join(kernelBuilderDir, e.Name()))
	}
	sort.Strings(paths)
	return paths, true
}

func cacheFileHashPaths(paths []string) (string, bool) {
	h := sha256.New()
	for _, path := range paths {
		data, err := os.ReadFile(path) //nolint:gosec // paths come from validated kernel profile registry
		if err != nil {
			return "", false
		}
		h.Write(data)
	}
	return hex.EncodeToString(h.Sum(nil))[:8], true
}

func kernelCacheVariant(arch, profile string) string {
	resolved, err := resolveKernelProfile(kernelInstallerConfigDir, profile)
	if err != nil {
		var tb textbuf.Buffer
		return tb.Str(kernelTargetInstaller).Byte('-').Str(arch).Byte('-').Str(profile).String()
	}
	return kernelCacheVariantFor(kernelTargetInstaller, arch, resolved)
}

// kernelCacheVariantFor keys the cache by target so installer (single-file
// Image) and runtime (vmlinuz tree) artifacts never collide, and by the config
// + builder-script hashes so profile/fragment/builder changes invalidate stale
// artifacts. The resolved Fragments already include any # ze-include shared
// fragment, so editing the shared fragment invalidates the cache too. The
// builder hash covers every module under kernelBuilderDir, the QEMU backend
// included (kernelBuilderScripts).
func kernelCacheVariantFor(target, arch string, profile kernelProfileResolution) string {
	var tb textbuf.Buffer
	configInputs := append([]string{}, profile.Fragments...)
	configInputs = append(configInputs, profile.Manifests...)
	configHash, configOK := cacheFileHashPaths(configInputs)
	builderHash, builderOK := "", false
	if scripts, found := kernelBuilderScripts(); found {
		builderHash, builderOK = cacheFileHashPaths(scripts)
	}
	if !configOK || !builderOK {
		return tb.Str(target).Byte('-').Str(arch).Byte('-').Str(profile.Name).String()
	}
	return tb.Str(target).Byte('-').Str(arch).Byte('-').Str(profile.Name).Byte('-').Str(configHash).Byte('-').Str(builderHash).String()
}

// initrdSourceDirs are the first-party source directories the installer initrd
// binary is compiled from: `go build ./cmd/ze-installer` (tag ze_installer)
// pulls in all of internal/install/disk. The cache variant hashes EVERY non-test
// .go file under these dirs, not a hand-picked subset, so any installer source
// edit (DHCP, netlink, download, ...) invalidates the cached initrd instead of
// silently serving a stale binary. Regression that motivated this: a
// dhcp_linux.go fix was masked because the old 4-file hash omitted it.
var initrdSourceDirs = []string{
	"cmd/ze-installer",
	"internal/install/disk",
}

// initrdSourceFiles lists every non-test .go file under initrdSourceDirs, sorted
// so the hash is stable regardless of directory walk order. ok is false if any
// dir cannot be read, so the caller falls back to a source-independent variant.
func initrdSourceFiles() (paths []string, ok bool) {
	for _, dir := range initrdSourceDirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return nil, false
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			name := e.Name()
			if strings.HasSuffix(name, ".go") && !strings.HasSuffix(name, "_test.go") {
				paths = append(paths, filepath.Join(dir, name))
			}
		}
	}
	sort.Strings(paths)
	return paths, true
}

func initrdCacheVariant(version, arch string) string {
	if paths, ok := initrdSourceFiles(); ok {
		if hash, hok := cacheFileHashPaths(paths); hok {
			var tb textbuf.Buffer
			return tb.Str(version).Byte('-').Str(arch).Byte('-').Str(hash).String()
		}
	}
	var tb textbuf.Buffer
	return tb.Str(version).Byte('-').Str(arch).String()
}

//nolint:unparam // version names the cache namespace and mirrors kernelCachePath.
func initrdCachePath(version, arch string) string {
	return filepath.Join(resolveCacheDir(), initrdCacheDir, initrdCacheVariant(version, arch), initrdFileName)
}

func downloadAndVerify(artifactURL, checksumURL, destPath string) error {
	if err := validateDownloadURL(artifactURL); err != nil {
		return err
	}

	checksumHex, err := downloadChecksum(checksumURL)
	if err != nil {
		return fmt.Errorf("download checksum from %s: %w", checksumURL, err)
	}

	if err := os.MkdirAll(filepath.Dir(destPath), cacheDirPerm); err != nil {
		return fmt.Errorf("create cache directory %s: %w", filepath.Dir(destPath), err)
	}

	tmp, err := os.CreateTemp(filepath.Dir(destPath), ".download-*")
	if err != nil {
		return fmt.Errorf("create temp file for download: %w", err)
	}
	tmpPath := tmp.Name()
	defer func() {
		tmp.Close()        //nolint:errcheck // cleanup
		os.Remove(tmpPath) //nolint:errcheck // cleanup
	}()

	resp, err := httpGetFn(artifactURL)
	if err != nil {
		return fmt.Errorf("download %s: %w", artifactURL, err)
	}
	defer resp.Body.Close() //nolint:errcheck // HTTP response

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: HTTP %d", artifactURL, resp.StatusCode)
	}

	h := sha256.New()
	written, err := io.Copy(io.MultiWriter(tmp, h), resp.Body)
	if err != nil {
		return fmt.Errorf("download %s: %w", artifactURL, err)
	}
	if written < minArtifactBytes {
		return fmt.Errorf("download %s: empty response", artifactURL)
	}

	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close downloaded artifact: %w", err)
	}

	actualHex := hex.EncodeToString(h.Sum(nil))
	if actualHex != checksumHex {
		return fmt.Errorf("checksum mismatch for %s: expected %s, got %s", artifactURL, checksumHex, actualHex)
	}

	if err := os.Rename(tmpPath, destPath); err != nil {
		return fmt.Errorf("move downloaded artifact to %s: %w", destPath, err)
	}

	return nil
}

func downloadChecksum(url string) (string, error) {
	resp, err := httpGetFn(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close() //nolint:errcheck // HTTP response

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, 1024))
	if err != nil {
		return "", err
	}

	fields := strings.Fields(string(data))
	if len(fields) == 0 {
		return "", fmt.Errorf("empty checksum file")
	}
	hex := strings.ToLower(fields[0])
	if len(hex) != checksumHexLen {
		return "", fmt.Errorf("invalid checksum length %d, expected %d hex characters", len(hex), checksumHexLen)
	}
	return hex, nil
}

func validateDownloadURL(rawURL string) error {
	if rawURL == "" {
		return fmt.Errorf("download URL is empty")
	}
	if !strings.HasPrefix(rawURL, "https://") && !strings.HasPrefix(rawURL, "file://") {
		return fmt.Errorf("download URL %q must use HTTPS or file:// scheme", rawURL)
	}
	return nil
}

func copyToToolsPath(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), cacheDirPerm); err != nil {
		return fmt.Errorf("create directory %s: %w", filepath.Dir(dst), err)
	}

	in, err := os.Open(src) //nolint:gosec // controlled source artifact
	if err != nil {
		return fmt.Errorf("open %s: %w", src, err)
	}
	defer in.Close() //nolint:errcheck // read-only

	tmp, err := os.CreateTemp(filepath.Dir(dst), ".copy-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmp.Name()
	defer func() {
		os.Remove(tmpPath) //nolint:errcheck // cleanup on failure
	}()

	if _, err := io.Copy(tmp, in); err != nil {
		tmp.Close() //nolint:errcheck // cleanup
		return fmt.Errorf("copy %s to %s: %w", src, dst, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}
	if err := os.Rename(tmpPath, dst); err != nil {
		return fmt.Errorf("move %s to %s: %w", tmpPath, dst, err)
	}
	return nil
}

// copyTree copies a directory tree (the runtime kernel: vmlinuz + lib/modules +
// DTBs + overlays) from src to dst, replacing dst. It stages the whole tree in a
// sibling temp dir and swaps it in with a rename, so a concurrent reader on the
// shared cache key sees the old complete tree, nothing (a loud miss that
// rebuilds), or the new complete tree -- never a half-written one -- and a failed
// copy leaves the existing dst intact (R-1/AC-8). An in-place copy instead
// exposed the partial tree for the whole copy duration. Directories, regular
// files, and symlinks are preserved (matching the `cp -R` Make path); other
// special files (devices, sockets) are skipped.
func copyTree(src, dst string) error {
	parent := filepath.Dir(dst)
	if err := os.MkdirAll(parent, cacheDirPerm); err != nil {
		return fmt.Errorf("create directory %s: %w", parent, err)
	}
	// Stage in a dot-prefixed sibling: on the same filesystem as dst (so the
	// rename is atomic) and skipped by evictKeepN if a concurrent evict runs.
	staging, err := os.MkdirTemp(parent, ".copytree-*")
	if err != nil {
		return fmt.Errorf("create staging dir in %s: %w", parent, err)
	}
	committed := false
	defer func() {
		if !committed {
			os.RemoveAll(staging) //nolint:errcheck // best-effort cleanup on failure
		}
	}()

	if err := walkCopyInto(src, staging); err != nil {
		return err
	}
	if err := os.RemoveAll(dst); err != nil {
		return fmt.Errorf("clear %s: %w", dst, err)
	}
	if err := os.Rename(staging, dst); err != nil {
		return fmt.Errorf("move %s to %s: %w", staging, dst, err)
	}
	committed = true
	return nil
}

// walkCopyInto copies the tree rooted at src into dst (which must already exist),
// preserving directories, regular files, and symlinks and skipping other special
// files (devices, sockets).
func walkCopyInto(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			info, err := d.Info()
			if err != nil {
				return err
			}
			if err := os.MkdirAll(target, info.Mode().Perm()); err != nil {
				return err
			}
			// MkdirAll leaves an existing dir's perm (the staging root) unchanged,
			// so set it explicitly to mirror the source.
			return os.Chmod(target, info.Mode().Perm())
		}
		if d.Type()&os.ModeSymlink != 0 {
			link, err := os.Readlink(path)
			if err != nil {
				return err
			}
			if err := os.MkdirAll(filepath.Dir(target), cacheDirPerm); err != nil {
				return err
			}
			os.Remove(target) //nolint:errcheck // best-effort replace
			return os.Symlink(link, target)
		}
		if !d.Type().IsRegular() {
			return nil
		}
		return copyRegularFile(path, target)
	})
}

// evictKeepN bounds a cache namespace directory to the evictKeepDefault most-recently-
// modified entries, removing older ones so a version/config bump reclaims what it supersedes
// (AC-4). It is called ONLY after a new entry is populated (key-change-only, never on a
// wall-clock timer), and it never removes an entry modified within evictGrace, since a
// concurrent run may be materializing or booting it (R-1/AC-8). Errors are swallowed:
// eviction is a best-effort space bound, never a correctness gate.
func evictKeepN(nsDir string) {
	entries, err := os.ReadDir(nsDir)
	if err != nil {
		return
	}
	type ent struct {
		name  string
		mtime time.Time
	}
	cutoff := evictNow().Add(-evictGrace)
	dirs := make([]ent, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if strings.HasPrefix(e.Name(), ".") {
			// Never a cache entry. Reap a staging dir (.copytree-*) orphaned by a
			// populate that was killed or lost a rename race, but only past the
			// grace window so a live populate's staging is never removed.
			if strings.HasPrefix(e.Name(), ".copytree-") && info.ModTime().Before(cutoff) {
				_ = os.RemoveAll(filepath.Join(nsDir, e.Name())) //nolint:errcheck // best-effort
			}
			continue
		}
		dirs = append(dirs, ent{e.Name(), info.ModTime()})
	}
	if len(dirs) <= evictKeepDefault {
		return
	}
	sort.Slice(dirs, func(i, j int) bool { return dirs[i].mtime.After(dirs[j].mtime) })
	for _, d := range dirs[evictKeepDefault:] {
		if d.mtime.After(cutoff) {
			continue // too fresh to be safe: leave garbage over racing a live run
		}
		_ = os.RemoveAll(filepath.Join(nsDir, d.name)) //nolint:errcheck // best-effort
	}
}

func copyRegularFile(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	in, err := os.Open(src) //nolint:gosec // controlled source tree
	if err != nil {
		return err
	}
	defer in.Close() //nolint:errcheck // read-only
	if err := os.MkdirAll(filepath.Dir(dst), cacheDirPerm); err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, info.Mode().Perm()) //nolint:gosec // controlled dest tree
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close() //nolint:errcheck // cleanup
		return err
	}
	return out.Close()
}
