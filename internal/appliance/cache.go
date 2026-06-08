// Design: plan/spec-install-10-iso-prerequisites.md — XDG cache resolution and artifact download

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
	"strings"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/core/textbuf"
)

const (
	cacheSubdir      = "ze"
	kernelCacheDir   = "installer-kernel"
	initrdCacheDir   = "installer-initrd"
	kernelFileName   = "Image"
	initrdFileName   = "initrd.img.gz"
	checksumSuffix   = ".sha256"
	downloadTimeout  = 10 * time.Minute
	cacheDirPerm     = 0o755
	checksumHexLen   = 64
	minArtifactBytes = 1
)

var httpGetFn = defaultHTTPGet

func defaultHTTPGet(url string) (*http.Response, error) {
	ctx, cancel := context.WithTimeout(context.Background(), downloadTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody) //nolint:gosec // URL validated by caller
	if err != nil {
		return nil, err
	}
	return http.DefaultClient.Do(req) //nolint:bodyclose // caller closes
}

func ResolveCacheDir() string {
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
	return filepath.Join(ResolveCacheDir(), kernelCacheDir, tb.Str(version).Byte('-').Str(variant).String(), kernelFileName)
}

func cacheFileHash(dir string, names []string) (string, bool) {
	h := sha256.New()
	for _, name := range names {
		data, err := os.ReadFile(filepath.Join(dir, name)) //nolint:gosec // constant base dir + validated names
		if err != nil {
			return "", false
		}
		h.Write(data)
	}
	return hex.EncodeToString(h.Sum(nil))[:8], true
}

func kernelCacheVariant(arch, profile string) string {
	var tb textbuf.Buffer
	var inputs []string
	if profile == ProfileHardwareKMS {
		inputs = []string{"kernel.config", "hardware.config", "hardware-kms.config", "build.sh"}
	} else {
		inputs = []string{"kernel.config", tb.Str(profile).Str(".config").String(), "build.sh"}
	}
	hash, ok := cacheFileHash(kernelToolsDir, inputs)
	if !ok {
		return tb.Reset().Str(arch).Byte('-').Str(profile).String()
	}
	return tb.Reset().Str(arch).Byte('-').Str(profile).Byte('-').Str(hash).String()
}

func initrdCacheVariant(version string) string {
	hash, ok := cacheFileHash(initrdToolsDir, []string{"init", "Makefile"})
	if !ok {
		return version
	}
	var tb textbuf.Buffer
	return tb.Str(version).Byte('-').Str(hash).String()
}

func initrdCachePath(version string) string {
	return filepath.Join(ResolveCacheDir(), initrdCacheDir, initrdCacheVariant(version), initrdFileName)
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
