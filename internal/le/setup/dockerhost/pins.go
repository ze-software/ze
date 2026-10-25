// Design: docs/architecture/appliance/docker-host.md -- the pin file and the image key
// Related: fetch.go -- Fetcher, which checks every download against these pins

// Package dockerhost is le's `setup docker-host` area: it builds Ze's
// docker-host image from pinned inputs and runs it as the `ze` Colima
// profile.
package dockerhost

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"slices"
	"strings"
)

// PinFile is the pin file's path from the repository root. It is the only
// home of the third-party versions and sha256 sums the image carries.
const PinFile = "gokrazy/docker-host/binaries.json"

// pinArches are the image architectures a download can be pinned for.
var pinArches = []string{archAMD64, archARM64}

// Pin is one pinned third-party input: a release download (per arch) or a
// source tarball (arch empty) built in the pinned builder container.
type Pin struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Arch    string `json:"arch,omitempty"`
	URL     string `json:"url"`
	SHA256  string `json:"sha256"`
}

// Pins is a pin file that passed LoadPins: every entry names an http(s) URL
// (the sha256, not the transport, vouches for the bytes) and
// a 64-digit lowercase sha256, every download an arch Ze builds, and the
// builder image a digest.
type Pins struct {
	builder   string
	downloads []Pin
	sources   []Pin
	path      string
}

// pinFileBody is the JSON shape of the pin file.
type pinFileBody struct {
	Builder   string `json:"builder"`
	Downloads []Pin  `json:"downloads"`
	Sources   []Pin  `json:"sources"`
}

// LoadPins reads and validates a pin file. Unknown keys are refused, so a
// misspelled field is never read as an absent one.
func LoadPins(path string) (*Pins, error) {
	data, err := os.ReadFile(path) //nolint:gosec // the pin file the caller names
	if err != nil {
		return nil, fmt.Errorf("read pin file %s: %w", path, err)
	}
	var body pinFileBody
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&body); err != nil {
		return nil, fmt.Errorf("pin file %s: %w", path, err)
	}
	if _, digest, found := strings.Cut(body.Builder, "@sha256:"); !found || !isSHA256(digest) {
		return nil, fmt.Errorf("pin file %s: builder %q is not pinned by digest (want <image>@sha256:<64 hex digits>)", path, body.Builder)
	}
	for _, pin := range body.Downloads {
		if !slices.Contains(pinArches, pin.Arch) {
			return nil, fmt.Errorf("pin file %s: download %s has arch %q, want one of %s", path, pin.Name, pin.Arch, strings.Join(pinArches, ", "))
		}
		if err := checkPin(pin); err != nil {
			return nil, fmt.Errorf("pin file %s: %w", path, err)
		}
	}
	for _, pin := range body.Sources {
		if pin.Arch != "" {
			return nil, fmt.Errorf("pin file %s: source %s names arch %q; a source tarball serves every arch", path, pin.Name, pin.Arch)
		}
		if err := checkPin(pin); err != nil {
			return nil, fmt.Errorf("pin file %s: %w", path, err)
		}
	}
	return &Pins{builder: body.Builder, downloads: body.Downloads, sources: body.Sources, path: path}, nil
}

// checkPin refuses a pin with no name or version, a URL that is not https
// with a file name, or a sha256 that is not 64 lowercase hex digits.
func checkPin(pin Pin) error {
	if pin.Name == "" {
		return errors.New("an entry has no name")
	}
	if pin.Version == "" {
		return fmt.Errorf("%s has no version", pin.Name)
	}
	parsed, err := url.Parse(pin.URL)
	if err != nil {
		return fmt.Errorf("%s url %q: %w", pin.Name, pin.URL, err)
	}
	if parsed.Scheme != "https" && parsed.Scheme != "http" {
		return fmt.Errorf("%s url %q is not http or https", pin.Name, pin.URL)
	}
	if fileName(pin) == "" {
		return fmt.Errorf("%s url %q names no file", pin.Name, pin.URL)
	}
	if !isSHA256(pin.SHA256) {
		return fmt.Errorf("%s sha256 %q is not 64 lowercase hex digits", pin.Name, pin.SHA256)
	}
	return nil
}

// isSHA256 answers whether s is a sha256 written as 64 lowercase hex digits.
func isSHA256(s string) bool {
	if len(s) != sha256.Size*2 {
		return false
	}
	if strings.ToLower(s) != s {
		return false
	}
	_, err := hex.DecodeString(s)
	return err == nil
}

// fileName is the last path element of a pin's URL, the name it is cached
// under.
func fileName(pin Pin) string {
	parsed, err := url.Parse(pin.URL)
	if err != nil {
		return ""
	}
	name := parsed.Path[strings.LastIndex(parsed.Path, "/")+1:]
	if name == "." || name == ".." {
		return ""
	}
	return name
}

// Path is the pin file these pins were read from, named in every refusal.
func (p *Pins) Path() string { return p.path }

// Builder is the builder container image, pinned by digest.
func (p *Pins) Builder() string { return p.builder }

// Sources are the source tarballs built in the builder container.
func (p *Pins) Sources() []Pin { return slices.Clone(p.sources) }

// Download answers the pinned release download of a name for an arch.
func (p *Pins) Download(name, arch string) (Pin, error) {
	for _, pin := range p.downloads {
		if pin.Name == name && pin.Arch == arch {
			return pin, nil
		}
	}
	return Pin{}, fmt.Errorf("pin file %s has no %s download for %s: add one with its url and sha256", p.path, name, arch)
}

// KeyInputs are the image inputs outside the pin file.
type KeyInputs struct {
	// Arch is the image architecture, amd64 or arm64.
	Arch string
	// KernelKey names the runtime kernel cache entry the image boots.
	KernelKey string
	// SourceDigest digests the guest program's source and the instance
	// directory, so a change to either builds a new image.
	SourceDigest string
}

// ImageKey answers the cache key of the image built from these pins for one
// arch and the other inputs: the first 16 hex digits of a sha256 over the
// builder digest, the arch's docker download, every source pin, the kernel
// key and the source digest. The same inputs always give the same key.
func ImageKey(p *Pins, in KeyInputs) (string, error) {
	docker, err := p.Download("docker", in.Arch)
	if err != nil {
		return "", err
	}
	if in.KernelKey == "" {
		return "", errors.New("image key: no runtime kernel key")
	}
	if in.SourceDigest == "" {
		return "", errors.New("image key: no source digest")
	}
	hash := sha256.New()
	write := func(fields ...string) {
		for _, field := range fields {
			_, _ = hash.Write([]byte(field)) //nolint:errcheck // a hash write never fails
			_, _ = hash.Write([]byte{0})     //nolint:errcheck // a hash write never fails
		}
	}
	write("builder", p.builder, "arch", in.Arch, "kernel", in.KernelKey, "source", in.SourceDigest)
	for _, pin := range append([]Pin{docker}, p.sources...) {
		write(pin.Name, pin.Version, pin.URL, pin.SHA256)
	}
	return hex.EncodeToString(hash.Sum(nil))[:16], nil
}
