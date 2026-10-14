// Design: docs/contributing/patching-the-standard-library.md -- the review stamp, vendored-module family
//
// Package vendorpatch holds the patches Ze applies to vendored modules after
// `go mod vendor` (applied by internal/le/setup applyVendorPatches), and the
// reading of the review stamp each one carries.

package vendorpatch

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/le/go/stdpatch"
)

// Dir is where the vendor patches live, relative to the checkout. The
// directory is the set: every *.patch file in it is applied.
const Dir = "internal/le/vendorpatch/patches"

// repairCommand is the command line a vendor stamp refusal ends with.
const repairCommand = "./le setup install"

// Patches answers every *.patch file in Dir, relative to the checkout and
// sorted, so the set is declared once: by the files present.
func Patches(root string) ([]string, error) {
	entries, err := os.ReadDir(filepath.Join(root, Dir))
	if err != nil {
		return nil, fmt.Errorf("vendorpatch: reading the patch set: %w", err)
	}
	paths := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".patch") {
			continue
		}
		paths = append(paths, Dir+"/"+entry.Name())
	}
	slices.Sort(paths)
	return paths, nil
}

// Stamp answers the module and version a vendor patch was reviewed against,
// from its `Ze-Reviewed-Against: <module path> <version>` line.
func Stamp(body []byte) (module, version string, err error) {
	value, err := stdpatch.ReviewStamp(body)
	if err != nil {
		return "", "", err
	}
	fields := strings.Fields(value)
	if len(fields) != 2 {
		return "", "", fmt.Errorf("the Ze-Reviewed-Against line names %q, want a module path and a version", value)
	}
	return fields[0], fields[1], nil
}

// Vendored answers the version of every module vendor/modules.txt lists,
// from its `# <module path> <version>` lines. The go command keeps that file
// consistent with go.mod: -mod=vendor refuses an inconsistent vendor tree.
func Vendored(root string) (map[string]string, error) {
	raw, err := os.ReadFile(filepath.Join(root, "vendor", "modules.txt")) //nolint:gosec // a build tool reads the checkout it was pointed at
	if err != nil {
		return nil, fmt.Errorf("vendorpatch: %w", err)
	}
	versions := make(map[string]string)
	for line := range strings.SplitSeq(string(raw), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 || fields[0] != "#" {
			continue
		}
		versions[fields[1]] = fields[2]
	}
	return versions, nil
}

// CheckStamp refuses a vendor patch whose stamp is missing, names a module
// the vendor directory lacks, or names a version other than the vendored
// one. A patch that still applies may change code upstream has since
// changed, so the stamp is checked before any `git apply`, the
// already-applied probe included. A patch the set names that cannot be read
// refuses too: the set and the disk disagree.
func CheckStamp(root, relative string, vendored map[string]string) error {
	body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(relative))) //nolint:gosec // a patch of the checkout's own set
	if err != nil {
		return fmt.Errorf("vendor patch %s: %w. %s", relative, err, reviewStep("its module", "the vendored version"))
	}
	module, version, err := Stamp(body)
	if err != nil {
		return fmt.Errorf("vendor patch %s: %w. %s", relative, err, reviewStep("the module it patches", "the version vendor/modules.txt lists"))
	}
	current, found := vendored[module]
	if !found {
		return fmt.Errorf("vendor patch %s was reviewed against %s %s, and vendor/modules.txt holds no %s. %s",
			relative, module, version, module, reviewStep(module, "the vendored version"))
	}
	if current != version {
		return fmt.Errorf("vendor patch %s was reviewed against %s %s, and the vendored %s is %s. A patch that still applies may change code upstream has since changed. %s",
			relative, module, version, module, current, reviewStep(module, current))
	}
	return nil
}

// reviewStep is the re-review procedure a refusal carries.
func reviewStep(module, version string) string {
	return "Re-read the functions the patch changes in " + module + " at " + version +
		". When upstream now does what the patch does, delete the patch and what depends on it. " +
		"Otherwise regenerate the patch if git no longer applies it, run its tests, and change its Ze-Reviewed-Against line to " +
		version + " (docs/contributing/patching-the-standard-library.md, section \"The review stamp\"). Then run: " + repairCommand
}
