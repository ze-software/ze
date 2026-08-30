// Design: docs/architecture/testing/qemu-integration.md -- the Alpine boot medium
// Overview: run.go -- the host harness that consumes this cache
package qemu

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

func alpineISOName(arch string) string {
	var b textbuf.Buffer
	return b.Str("alpine-virt-").Str(AlpineVersion).Byte('.').Str(AlpineMinor).
		Byte('-').Str(arch).Str(".iso").String()
}

func (r *Run) durableCacheDir() (string, error) {
	base := r.ops.Getenv("XDG_CACHE_HOME")
	if base == "" {
		home, err := r.ops.Home()
		if err != nil {
			return "", fmt.Errorf("locate the home directory for the Alpine cache: %w", err)
		}
		base = filepath.Join(home, ".cache")
	}
	return filepath.Join(base, "ze"), nil
}

func (r *Run) ensureISO(ctx context.Context, arch string) (string, error) {
	cache, err := r.durableCacheDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(cache, "alpine-iso")
	if err := r.ops.FS.MkdirAll(dir, 0o750); err != nil {
		return "", fmt.Errorf("create Alpine ISO cache %s: %w", dir, err)
	}
	name := alpineISOName(arch)
	iso := filepath.Join(dir, name)
	var b textbuf.Buffer
	sidecar := b.Str(iso).Str(".sha256").String()
	cachedPair := r.regularFile(iso) && r.regularFile(sidecar)
	if cachedPair {
		stored, readErr := r.ops.FS.ReadFile(sidecar)
		if readErr != nil {
			return "", fmt.Errorf("read Alpine ISO checksum %s: %w", sidecar, readErr)
		}
		fields := strings.Fields(string(stored))
		storedChecksumValid := len(fields) != 0 && validSHA256(fields[0])
		if storedChecksumValid {
			got, hashErr := r.sha256File(iso)
			if hashErr != nil {
				return "", hashErr
			}
			if strings.EqualFold(got, fields[0]) {
				return iso, nil
			}
		}
		fmt.Fprintf(os.Stderr, "cached ISO %s failed integrity check; re-downloading\n", iso) //nolint:errcheck // progress output
		if removeErr := r.removeCachePair(iso, sidecar); removeErr != nil {
			return "", removeErr
		}
	}

	b.Reset()
	url := b.Str("https://dl-cdn.alpinelinux.org/alpine/v").Str(AlpineVersion).
		Str("/releases/").Str(arch).Byte('/').Str(name).String()
	checksum, err := r.publishedSHA256(ctx, url)
	if err != nil {
		return "", err
	}
	b.Reset()
	fmt.Fprintln(os.Stderr, "Downloading Alpine virt ISO...") //nolint:errcheck // progress output
	part := b.Str(iso).Str(".part").String()
	result, err := r.runCommand(ctx, commandSpec{
		Name: "curl", Args: []string{"-fSL", "--progress-bar", "-o", part, url},
		Env: r.ops.Environ(), Stdout: os.Stderr, Stderr: os.Stderr,
	})
	if err != nil {
		r.ops.FS.Remove(part) //nolint:errcheck // the download error is the primary error
		return "", err
	}
	if result.Code != 0 {
		r.ops.FS.Remove(part) //nolint:errcheck // the curl status is the primary error
		return "", fmt.Errorf("download Alpine ISO %s: curl exited %d", url, result.Code)
	}
	got, err := r.sha256File(part)
	if err != nil {
		return "", err
	}
	if got != checksum {
		r.ops.FS.Remove(part) //nolint:errcheck // the mismatch is the primary error
		return "", fmt.Errorf("alpine ISO checksum mismatch for %s: got %s, want %s", name, got, checksum)
	}
	if err := r.ops.FS.Rename(part, iso); err != nil {
		return "", fmt.Errorf("publish Alpine ISO %s: %w", iso, err)
	}
	b.Reset()
	data := b.Str(checksum).Str("  ").Str(name).Byte('\n').Bytes()
	if err := r.ops.FS.WriteFile(sidecar, data, 0o600); err != nil {
		return "", fmt.Errorf("write Alpine ISO checksum %s: %w", sidecar, err)
	}
	return iso, nil
}

func (r *Run) publishedSHA256(ctx context.Context, url string) (string, error) {
	var b textbuf.Buffer
	checksumURL := b.Str(url).Str(".sha256").String()
	result, err := r.runCommand(ctx, commandSpec{
		Name: "curl", Args: []string{"-fSL", checksumURL}, Env: r.ops.Environ(),
	})
	if err != nil {
		return "", err
	}
	checksumUnavailable := result.Code != 0 || strings.TrimSpace(result.Stdout) == ""
	if checksumUnavailable {
		return "", fmt.Errorf("fetch Alpine checksum %s.sha256: curl exited %d", url, result.Code)
	}
	fields := strings.Fields(result.Stdout)
	checksumMalformed := len(fields) == 0 || !validSHA256(fields[0])
	if checksumMalformed {
		return "", fmt.Errorf("malformed Alpine checksum from %s.sha256: %q", url, result.Stdout)
	}
	return strings.ToLower(fields[0]), nil
}

func validSHA256(value string) bool {
	if len(value) != sha256.Size*2 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func (r *Run) sha256File(name string) (string, error) {
	file, err := r.ops.FS.Open(name)
	if err != nil {
		return "", fmt.Errorf("open %s for checksum: %w", name, err)
	}
	defer file.Close() //nolint:errcheck // a read-only file has no state to flush

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", fmt.Errorf("checksum %s: %w", name, err)
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func (r *Run) removeCachePair(iso, sidecar string) error {
	for _, name := range []string{iso, sidecar} {
		if err := r.ops.FS.Remove(name); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return fmt.Errorf("remove invalid Alpine cache file %s: %w", name, err)
		}
	}
	return nil
}

func (r *Run) regularFile(name string) bool {
	info, err := r.ops.FS.Stat(name)
	if err != nil {
		return false
	}
	return info.Mode().IsRegular()
}

func alpineExtractDir(iso string) string {
	extension := filepath.Ext(iso)
	stem := strings.TrimSuffix(filepath.Base(iso), extension)
	var b textbuf.Buffer
	name := b.Str(stem).Str("-extract").String()
	return filepath.Join(filepath.Dir(iso), name)
}

func (r *Run) extractAlpineInitramfs(ctx context.Context, iso string) (string, error) {
	dir := alpineExtractDir(iso)
	initrd := filepath.Join(dir, "boot", "initramfs-virt")
	if r.regularFile(initrd) {
		return initrd, nil
	}
	extractors := make([]string, 0, 2)
	for _, name := range []string{"7z", "7zz"} {
		if _, err := r.ops.Look(name); err == nil {
			extractors = append(extractors, name)
		}
	}
	if len(extractors) == 0 {
		return "", errors.New("cannot extract Alpine initramfs: install 7z or p7zip")
	}
	if err := r.ops.FS.MkdirAll(filepath.Dir(dir), 0o750); err != nil {
		return "", err
	}
	for _, extractor := range extractors {
		var b textbuf.Buffer
		prefix := b.Byte('.').Str(filepath.Base(dir)).Byte('.').String()
		staging, err := r.ops.FS.MkdirTemp(filepath.Dir(dir), prefix)
		if err != nil {
			return "", err
		}
		committed := false
		b.Reset()
		output := b.Str("-o").Str(staging).String()
		result, runErr := r.runCommand(ctx, commandSpec{
			Name: extractor, Args: []string{"x", iso, "-y", output}, Env: r.ops.Environ(),
		})
		extracted := r.regularFile(filepath.Join(staging, "boot", "initramfs-virt"))
		extractSucceeded := runErr == nil && result.Code == 0 && extracted
		if extractSucceeded {
			if err := r.ops.FS.RemoveAll(dir); err != nil {
				r.ops.FS.RemoveAll(staging) //nolint:errcheck // the remove error is primary
				return "", err
			}
			if err := r.ops.FS.Rename(staging, dir); err != nil {
				r.ops.FS.RemoveAll(staging) //nolint:errcheck // the rename error is primary
				return "", err
			}
			committed = true
		}
		if !committed {
			r.ops.FS.RemoveAll(staging) //nolint:errcheck // the next extractor is the recovery
			continue
		}
		return initrd, nil
	}
	return "", fmt.Errorf("extract Alpine initramfs from %s: every extractor failed", iso)
}

func (r *Run) brewFiles(relative string) []string {
	prefixes := make([]string, 0, 4)
	if exported := r.ops.Getenv("HOMEBREW_PREFIX"); exported != "" {
		prefixes = append(prefixes, exported)
	}
	if brew, err := r.ops.Look("brew"); err == nil {
		prefixes = append(prefixes, filepath.Dir(filepath.Dir(brew)))
	}
	if r.ops.GOOS == goosDarwin {
		prefixes = append(prefixes, "/opt/homebrew", "/usr/local")
	}
	seen := make(map[string]bool, len(prefixes))
	files := make([]string, 0, len(prefixes))
	for _, prefix := range prefixes {
		if seen[prefix] {
			continue
		}
		seen[prefix] = true
		info, err := r.ops.FS.Stat(prefix)
		if err != nil {
			continue
		}
		if !info.IsDir() {
			continue
		}
		candidate := filepath.Join(prefix, filepath.FromSlash(relative))
		if r.regularFile(candidate) {
			files = append(files, candidate)
		}
	}
	return files
}

func (r *Run) runCommand(ctx context.Context, spec commandSpec) (commandResult, error) {
	if _, err := r.ops.Look(spec.Name); err != nil {
		return commandResult{}, fmt.Errorf("missing required command %s: %w", spec.Name, err)
	}
	return r.ops.Run(ctx, spec)
}
