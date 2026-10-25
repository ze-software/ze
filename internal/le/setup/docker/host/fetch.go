// Design: docs/architecture/appliance/docker-host.md -- the pin file and the image key
// Related: pins.go -- Pins, the sums every fetched file is checked against

package dockerhost

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
)

// Fetcher downloads pinned files into CacheDir/downloads and checks each one
// against its pin, on the first download and again on every reuse of the
// cached copy. A file whose sha256 differs from its pin never stays in the
// cache, so it cannot reach an image (spec AC-5, R-8).
type Fetcher struct {
	// CacheDir is the docker-host cache root, ~/.cache/ze/docker-host.
	CacheDir string
	// Client performs the downloads.
	Client *http.Client
}

// Fetch answers the path of the cached file for a pin, downloading it when
// the cache has none. It refuses, naming the file, both sums and the pin
// file, when the bytes differ from the pin.
func (f Fetcher) Fetch(ctx context.Context, pins *Pins, pin Pin) (string, error) {
	dir := filepath.Join(f.CacheDir, "downloads")
	target := filepath.Join(dir, fileName(pin))
	sum, err := fileSHA256(target)
	if err == nil {
		if sum != pin.SHA256 {
			return "", mismatch(pins, pin, target, sum, "delete "+target+" and run the build again to download it afresh")
		}
		return target, nil
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil { //nolint:gosec // a cache directory in the user's home
		return "", fmt.Errorf("create %s: %w", dir, err)
	}
	partial, err := os.CreateTemp(dir, fileName(pin)+".partial-*")
	if err != nil {
		return "", fmt.Errorf("create a download file in %s: %w", dir, err)
	}
	defer os.Remove(partial.Name()) //nolint:errcheck // gone already once renamed into place

	sum, err = f.download(ctx, pin.URL, partial)
	closeErr := partial.Close()
	if err != nil {
		return "", err
	}
	if closeErr != nil {
		return "", fmt.Errorf("write %s: %w", partial.Name(), closeErr)
	}
	if sum != pin.SHA256 {
		return "", mismatch(pins, pin, pin.URL, sum, "nothing was kept in the cache")
	}
	if err := os.Rename(partial.Name(), target); err != nil {
		return "", fmt.Errorf("move the download into %s: %w", target, err)
	}
	return target, nil
}

// download writes the body of url to w and answers its sha256.
func (f Fetcher) download(ctx context.Context, url string, w io.Writer) (string, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return "", fmt.Errorf("download %s: %w", url, err)
	}
	response, err := f.Client.Do(request)
	if err != nil {
		return "", fmt.Errorf("download %s: %w", url, err)
	}
	defer response.Body.Close() //nolint:errcheck // a read-only body
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download %s: server answered %s", url, response.Status)
	}
	hash := sha256.New()
	if _, err := io.Copy(io.MultiWriter(w, hash), response.Body); err != nil {
		return "", fmt.Errorf("download %s: %w", url, err)
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

// fileSHA256 answers the sha256 of a file.
func fileSHA256(path string) (string, error) {
	file, err := os.Open(path) //nolint:gosec // a file in the docker-host cache
	if err != nil {
		return "", err
	}
	defer file.Close() //nolint:errcheck // read-only
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", fmt.Errorf("read %s: %w", path, err)
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

// mismatch is the refusal for bytes whose sha256 differs from their pin.
func mismatch(pins *Pins, pin Pin, source, got, kept string) error {
	return fmt.Errorf("%s: sha256 is %s, but %s pins %s %s (%s) at sha256 %s; "+
		"these are not the bytes that were pinned, so the image build stopped (%s). "+
		"Fix: if upstream republished %s on purpose, check the new file and set its sha256 in %s; otherwise run the build again",
		source, got, pins.Path(), pin.Name, pin.Version, archLabel(pin), pin.SHA256, kept, fileName(pin), pins.Path())
}

// archLabel names a pin's arch, or that a source tarball serves every arch.
func archLabel(pin Pin) string {
	if pin.Arch == "" {
		return "source"
	}
	return pin.Arch
}
