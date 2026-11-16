// Design: docs/architecture/appliance/build-artifacts.md -- immutable runtime kernel entries.
// Related: cache.go -- the namespace and variant grammar this reader accepts.
// Related: kernelbuilder/recipe.go -- provenance written after a successful build.

package appliance

import (
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/ze-software/ze/internal/appliance/kernelbuilder"
)

// ExistingRuntimeKernelTree validates the identity of an explicitly selected
// cached runtime. It never consults today's recipe or invokes a builder: an
// image/kernel pair can still be used after the source recipe changes.
// The caller MUST additionally verify the requested bytes, modules and image.
func ExistingRuntimeKernelTree(requested, arch string) (tree string, resultErr error) {
	defer func() {
		if resultErr != nil {
			resultErr = fmt.Errorf("validate existing runtime kernel %s: %w. Fix: name its cached vmlinuz and matching image with ./le test qemu docker-lab kernel <runtime-tree>/vmlinuz image <image> command <command>, or omit image to resolve the current runtime", requested, resultErr)
		}
	}()
	if !kernelbuilder.ArchSupported(arch) {
		return "", fmt.Errorf("unsupported architecture %q", arch)
	}
	selected, err := filepath.Abs(requested)
	if err != nil {
		return "", err
	}
	selected, err = filepath.EvalSymlinks(selected)
	if err != nil {
		return "", err
	}
	if filepath.Base(selected) != runtimeKernelArtifact {
		return "", fmt.Errorf("the selected cache artifact is not %s", runtimeKernelArtifact)
	}
	namespace, err := filepath.Abs(filepath.Join(resolveCacheDir(), runtimeKernelCacheDir))
	if err != nil {
		return "", err
	}
	namespace, err = filepath.EvalSymlinks(namespace)
	if err != nil {
		return "", err
	}
	tree = filepath.Dir(selected)
	if filepath.Dir(tree) != namespace {
		return "", fmt.Errorf("the selected tree is outside the runtime cache %s", namespace)
	}
	// kernelCacheVariantFor writes target, arch, profile, backend and two
	// eight-hex input hashes. Keep those old hashes, not today's source key.
	prefix := defaultKernelVersion + "-" + kernelTargetRuntime + "-" + arch + "-" + runtimeKernelProfile + "-"
	variant, found := strings.CutPrefix(filepath.Base(tree), prefix)
	if !found {
		return "", fmt.Errorf("cache entry %s does not name Linux %s runtime for %s", tree, defaultKernelVersion, arch)
	}
	parts := strings.Split(variant, "-")
	if len(parts) != 3 {
		return "", fmt.Errorf("cache entry %s has no backend and two input hashes", tree)
	}
	switch parts[0] {
	case builderDocker, builderQEMU:
	default:
		return "", fmt.Errorf("cache entry %s names unknown builder %q", tree, parts[0])
	}
	for _, hash := range parts[1:] {
		if len(hash) != 8 {
			return "", fmt.Errorf("cache entry %s has an invalid input hash %q", tree, hash)
		}
		if _, err := hex.DecodeString(hash); err != nil {
			return "", fmt.Errorf("cache entry %s has an invalid input hash: %w", tree, err)
		}
	}
	fields, err := existingRuntimeProvenance(filepath.Join(tree, kernelbuilder.ProvenanceName))
	if err != nil {
		return "", err
	}
	digest, tracked := kernelbuilder.SourceDigest(defaultKernelVersion)
	if !tracked {
		return "", fmt.Errorf("the Linux version %s has no tracked source digest", defaultKernelVersion)
	}
	for _, expected := range []struct{ key, value string }{
		{kernelbuilder.ProvenanceKeyVersion, defaultKernelVersion},
		{"target", kernelTargetRuntime},
		{"profile", runtimeKernelProfile},
		{"arch", arch},
		{"modules", "yes"},
		{"builder", parts[0]},
		{kernelbuilder.ProvenanceKeySourceSHA256, digest},
	} {
		if fields[expected.key] != expected.value {
			return "", fmt.Errorf("runtime provenance %s=%q, want %q", expected.key, fields[expected.key], expected.value)
		}
	}
	if fields[kernelbuilder.ProvenanceKeySourceURL] == "" {
		return "", errors.New("runtime provenance names no source URL")
	}
	return tree, nil
}

// existingRuntimeProvenance reads only the bounded completion record written
// beside a built kernel; duplicate keys cannot provide two identities.
func existingRuntimeProvenance(path string) (map[string]string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("runtime provenance %s is not a regular file", path)
	}
	file, err := os.Open(path) //nolint:gosec // the canonical runtime cache entry's completion record
	if err != nil {
		return nil, err
	}
	defer file.Close() //nolint:errcheck // read-only completion metadata
	const limit = 4096
	body, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return nil, err
	}
	if len(body) > limit {
		return nil, fmt.Errorf("runtime provenance %s exceeds %d bytes", path, limit)
	}
	fields := make(map[string]string)
	for line := range strings.SplitSeq(string(body), "\n") {
		if line == "" {
			continue
		}
		key, value, found := strings.Cut(line, "=")
		if !found {
			return nil, fmt.Errorf("runtime provenance %s contains a line without '='", path)
		}
		if _, duplicate := fields[key]; duplicate {
			return nil, fmt.Errorf("runtime provenance %s repeats %q", path, key)
		}
		fields[key] = value
	}
	return fields, nil
}
