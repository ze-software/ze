// Design: docs/contributing/preview-artifacts.md -- portable preview verification.
package buildpreview

import (
	"archive/tar"
	"compress/gzip"
	"debug/buildinfo"
	"debug/elf"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

func inspectBinary(path, arch string, source Source, version string) error {
	info, err := buildinfo.ReadFile(path)
	if err != nil {
		return err
	}
	if info.GoVersion != source.Toolchain {
		return fmt.Errorf("binary toolchain %s differs from %s", info.GoVersion, source.Toolchain)
	}
	settings := map[string]string{}
	for _, setting := range info.Settings {
		settings[setting.Key] = setting.Value
	}
	for key, want := range map[string]string{"GOOS": targetOS, "GOARCH": arch, "CGO_ENABLED": "0", "-trimpath": "true"} {
		if settings[key] != want {
			return fmt.Errorf("binary %s = %q, expected %q", key, settings[key], want)
		}
	}
	tags := strings.FieldsFunc(settings["-tags"], func(r rune) bool { return r == ',' || r == ' ' })
	wantTags := slices.Clone(source.Tags)
	slices.Sort(tags)
	slices.Sort(wantTags)
	if !slices.Equal(tags, wantTags) {
		return fmt.Errorf("binary feature inventory differs: %v versus %v", tags, wantTags)
	}
	file, err := elf.Open(path)
	if err != nil {
		return err
	}
	defer file.Close() //nolint:errcheck // read-only file
	// Trimpath intentionally omits linker flags from Go's build settings.
	// Read the stamped Go string variables from ELF instead of executing a
	// foreign target or trusting a separately written declaration.
	for symbol, want := range map[string]string{"main.version": version, "main.buildDate": source.BuildDate} {
		got, err := elfString(file, symbol)
		if err != nil {
			return err
		}
		if got != want {
			return fmt.Errorf("binary %s = %q, expected %q", symbol, got, want)
		}
	}
	for _, program := range file.Progs {
		if program.Type == elf.PT_INTERP {
			return errors.New("preview binary depends on a dynamic interpreter")
		}
	}
	return nil
}

func elfString(file *elf.File, name string) (string, error) {
	symbols, err := file.Symbols()
	if err != nil {
		return "", err
	}
	read := func(address uint64, size uint64) ([]byte, error) {
		if size > 1024 {
			return nil, errors.New("oversized preview identity string")
		}
		for _, section := range file.Sections {
			if address >= section.Addr && address-section.Addr <= section.Size && size <= section.Size-(address-section.Addr) {
				body := make([]byte, int(size))
				_, err := section.ReadAt(body, int64(address-section.Addr))
				return body, err
			}
		}
		return nil, fmt.Errorf("preview identity address %x is absent from ELF", address)
	}
	for _, symbol := range symbols {
		if symbol.Name != name {
			continue
		}
		if file.Class != elf.ELFCLASS64 || symbol.Size != 16 {
			return "", errors.New("unsupported preview Go string layout")
		}
		header, err := read(symbol.Value, 16)
		if err != nil {
			return "", err
		}
		body, err := read(file.ByteOrder.Uint64(header[:8]), file.ByteOrder.Uint64(header[8:]))
		return string(body), err
	}
	return "", fmt.Errorf("preview identity symbol %s is absent", name)
}

func check(output string, inspect func(string, string, Source, string) error) (Manifest, error) {
	var manifest Manifest
	body, err := regularFile(output, "manifest.json")
	if err != nil {
		return manifest, err
	}
	decoder := json.NewDecoder(strings.NewReader(string(body)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&manifest); err != nil {
		return manifest, err
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return manifest, errors.New("preview manifest has trailing content")
	}
	if manifest.Schema != 1 || manifest.Channel != "preview" || manifest.Published || len(manifest.Source.Commit) != 40 || len(manifest.Source.Digest) != 64 {
		return manifest, errors.New("invalid preview manifest identity")
	}
	if manifest.Version != previewVersion(manifest.Source) {
		return manifest, errors.New("preview version differs from source identity")
	}
	if sourceDigest(inputDigest(manifest.Source.Inputs), manifest.Source.Toolchain, manifest.Source.Tags) != manifest.Source.Digest {
		return manifest, errors.New("preview selected-input inventory differs from source digest")
	}
	checksums, err := regularFile(output, "SHA256SUMS")
	if err != nil {
		return manifest, err
	}
	expected := map[string]string{}
	for line := range strings.SplitSeq(strings.TrimSuffix(string(checksums), "\n"), "\n") {
		sum, file, found := strings.Cut(line, "  ")
		if !found || !safeName(file) || len(sum) != 64 || expected[file] != "" {
			return manifest, fmt.Errorf("invalid preview checksum line %q", line)
		}
		expected[file] = sum
	}
	allowed := map[string]bool{"manifest.json": true, "SHA256SUMS": true}
	binaries := map[string]string{}
	archives := map[string]string{}
	dockerArtifacts := map[string]Artifact{}
	for _, artifact := range manifest.Artifacts {
		if !safeName(artifact.File) || allowed[artifact.File] || artifact.OS != targetOS || (artifact.Arch != archAMD64 && artifact.Arch != archARM64) {
			return manifest, errors.New("invalid or repeated preview artifact")
		}
		allowed[artifact.File] = true
		if expected[artifact.File] != artifact.SHA256 {
			return manifest, fmt.Errorf("checksum declaration differs for %s", artifact.File)
		}
		if err := verifySum(output, artifact.File, artifact.SHA256); err != nil {
			return manifest, err
		}
		switch artifact.Kind {
		case "binary":
			if binaries[artifact.Arch] != "" {
				return manifest, errors.New("repeated binary target")
			}
			binaries[artifact.Arch] = artifact.File
			if err := inspect(filepath.Join(output, artifact.File), artifact.Arch, manifest.Source, manifest.Version); err != nil {
				return manifest, fmt.Errorf("%s: %w", artifact.File, err)
			}
		case "archive":
			if archives[artifact.Arch] != "" {
				return manifest, errors.New("repeated archive target")
			}
			archives[artifact.Arch] = artifact.File
		case "docker":
			dockerArtifacts[artifact.File] = artifact
		default:
			return manifest, fmt.Errorf("unknown preview artifact kind %q", artifact.Kind)
		}
	}
	if len(expected) != len(manifest.Artifacts)+1 {
		return manifest, errors.New("checksum population differs from manifest")
	}
	if err := verifySum(output, "manifest.json", expected["manifest.json"]); err != nil {
		return manifest, err
	}
	for _, arch := range []string{archAMD64, archARM64} {
		if binaries[arch] == "" || archives[arch] == "" {
			return manifest, fmt.Errorf("incomplete Linux %s preview", arch)
		}
		if err := checkArchive(filepath.Join(output, archives[arch]), filepath.Join(output, binaries[arch])); err != nil {
			return manifest, fmt.Errorf("%s: %w", archives[arch], err)
		}
	}
	if len(manifest.Images) != len(dockerArtifacts) || (len(manifest.Images) != 0 && len(manifest.Images) != 2) {
		return manifest, errors.New("incomplete preview image population")
	}
	roles := map[string]bool{}
	imageFiles := map[string]bool{}
	for _, image := range manifest.Images {
		artifact, found := dockerArtifacts[image.File]
		if !found || imageFiles[image.File] || image.OS != artifact.OS || image.Arch != artifact.Arch || !validImageDigest(image.ID) || roles[image.Role] || (image.Role != "deployment" && image.Role != "lab") || !strings.Contains(image.VersionOutput, manifest.Version) {
			return manifest, errors.New("invalid preview image record")
		}
		roles[image.Role] = true
		imageFiles[image.File] = true
		if err := checkDockerArchive(filepath.Join(output, image.File), image); err != nil {
			return manifest, fmt.Errorf("%s: %w", image.File, err)
		}
	}
	entries, err := os.ReadDir(output)
	if err != nil {
		return manifest, err
	}
	for _, entry := range entries {
		if !allowed[entry.Name()] {
			return manifest, fmt.Errorf("unexpected file in preview bundle: %s", entry.Name())
		}
	}
	return manifest, nil
}

func safeName(name string) bool {
	return name != "" && name != "." && name != ".." && filepath.Base(name) == name && !strings.ContainsAny(name, "\\\r\n")
}

func regularFile(root, name string) ([]byte, error) {
	if !safeName(name) {
		return nil, errors.New("unsafe preview artifact path")
	}
	path := filepath.Join(root, name)
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("preview artifact %s is not a regular file", name)
	}
	return os.ReadFile(path) //nolint:gosec // G304: safeName confines name to the bundle directory the caller named, and Lstat proved a regular file
}

func verifySum(root, name, want string) error {
	if !safeName(name) || len(want) != 64 {
		return errors.New("invalid preview checksum")
	}
	info, err := os.Lstat(filepath.Join(root, name))
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("preview artifact %s is not a regular file", name)
	}
	sum, err := fileSum(filepath.Join(root, name))
	if err != nil {
		return err
	}
	if sum != want {
		return fmt.Errorf("preview checksum mismatch: %s", name)
	}
	return nil
}

// archiveSizeLimit is layerSizeLimit, the ceiling a Docker layer gets,
// applied to a preview archive: it bounds the decompressed bytes checkArchive
// reads, the whole gzip stream past the tar end marker included. The check
// runs on bundles nobody here built, so a small crafted archive must not cost
// unbounded CPU and time.
const archiveSizeLimit = layerSizeLimit

func checkArchive(path, binary string) error {
	return checkArchiveWithin(path, binary, archiveSizeLimit)
}

// checkArchiveWithin verifies the preview archive at path against binary,
// reading at most limit decompressed bytes. Every read of the stream (the
// tar headers, the hashed ze entry, the skipped bodies and the final drain)
// goes through one limited reader, so a stream longer than limit is refused
// with the size-limit error wherever it ran out. limit MUST be positive.
func checkArchiveWithin(path, binary string, limit int64) error {
	file, err := os.Open(path) //nolint:gosec // G304: check joins a manifest name safeName confines to the bundle directory the caller named
	if err != nil {
		return err
	}
	defer file.Close() //nolint:errcheck // read-only file
	zip, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer zip.Close() //nolint:errcheck // read side; the trailing io.Copy reports any stream error
	// One byte past the limit tells a stream of exactly limit bytes from a
	// longer one: N reaches zero only once more than limit bytes were read.
	// An exhausted reader answers io.EOF, which the tar reader turns into a
	// short read or a clean end, so N is checked before any of those answers.
	limited := &io.LimitedReader{R: zip, N: limit + 1}
	oversized := func() error {
		return fmt.Errorf("preview archive exceeds its size limit of %d decompressed bytes", limit)
	}
	archive := tar.NewReader(limited)
	seen := map[string]bool{}
	want, err := fileSum(binary)
	if err != nil {
		return err
	}
	for {
		header, err := archive.Next()
		if limited.N <= 0 {
			return oversized()
		}
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}
		if seen[header.Name] || (header.Name != "ze" && header.Name != "README" && header.Name != "LICENSE") || header.Typeflag != tar.TypeReg || header.Uid != 0 || header.Gid != 0 {
			return errors.New("preview archive contains unexpected files or ownership")
		}
		seen[header.Name] = true
		if header.Name == "ze" {
			// A sparse entry's holes are filled with zeros that never pass
			// through the limited reader, so the declared size is what bounds
			// the hash. Skipped bodies are never expanded: Next discards them
			// by their physical size, which the limited reader counts. A
			// negative size never reaches here: archive/tar answers ErrHeader
			// for one, the sparse real size included.
			if header.Size > limit {
				return oversized()
			}
			sum, err := readerSum(archive)
			if limited.N <= 0 {
				return oversized()
			}
			if err != nil {
				return err
			}
			if sum != want || header.Mode != 0o755 {
				return errors.New("preview archive binary differs")
			}
		}
	}
	if len(seen) != 3 {
		return errors.New("preview archive is incomplete")
	}
	// Consume the rest of the stream and the gzip trailer, within the same
	// limit, so a truncated or corrupt archive cannot pass and trailing data
	// cannot run the check unbounded.
	_, err = io.Copy(io.Discard, limited)
	if limited.N <= 0 {
		return oversized()
	}
	return err
}
