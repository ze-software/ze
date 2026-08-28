// Design: docs/architecture/testing/qemu-integration.md -- the kernel an appliance proof boots
// Related: deployment.go -- the package these appliance proofs live in
// Related: gokrazyimage.go -- the resolution that calls this on every candidate
// Related: gokrazyl2tp.go -- the proof that will not boot without this
// Related: gokrazylab.go -- the network the validated kernel is booted onto
//
// gokrazykernel.go answers whether a kernel package can carry an appliance L2TP
// PPP proof. This check is a gate rather than a preference. The measured reason
// is ze's FAIL-CLOSED L2TP module probe (probeKernelModules,
// internal/component/l2tp/kernel_linux.go). An appliance booted on a kernel with
// no PPPoL2TP exits at startup, and gokrazy turns that into a crash loop. The
// proof then burns its whole boot bound and reports "the web server did not
// start". This report directs the reader to the appliance rather than the kernel.
//
// The check verifies three conditions that have each caused an obscure failure.
// The vmlinuz must be for the architecture the image is built for. The kernel
// must carry PPPoL2TP either built in or as a loadable module. The module tree
// must belong to the pinned kernel rather than to an older one a previous build
// staged.

package deployment

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// The two architectures an appliance is built for, as GOARCH spells them.
const (
	ArchAMD64 = "amd64"
	ArchARM64 = "arm64"
)

// kernelMagic identifies where each architecture's kernel image carries its own
// identifying bytes. The amd64 bzImage header is at 0x202, and the arm64 Image
// header is at 0x38.
//
// The code reads these bytes rather than trust the package path because a shared
// module cache has already held the wrong kernel. An arm64 kernel staged under
// an amd64 pin boots to nothing. QEMU's own silence is the only symptom.
var kernelMagic = map[string]struct {
	offset int64
	magic  string
}{
	ArchAMD64: {offset: 0x202, magic: "HdrS"},
	ArchARM64: {offset: 0x38, magic: "ARMd"},
}

// l2tpModuleName is the module the appliance's L2TP path needs, and
// l2tpModuleFile is how a loadable copy of it is named before any compression
// suffix.
const (
	l2tpModuleName = "l2tp_ppp"
	l2tpModuleFile = "l2tp_ppp.ko"
)

// modulesBuiltin is the file a kernel package lists its built-in modules in.
const modulesBuiltin = "modules.builtin"

// KernelVersionFile is where this repository states which kernel it pins. It is
// the single source of truth, so the proof reads it rather than carrying a
// version of its own.
const KernelVersionFile = "internal/appliance/kernel.version"

// errUnsupportedArch is what a caller is told when the architecture is neither
// of the two an appliance is built for.
var errUnsupportedArch = errors.New("unsupported appliance architecture (expected amd64 or arm64)")

// pinnedKernelVersion answers the kernel release this repository pins.
func pinnedKernelVersion(tree string) (string, error) {
	body, err := os.ReadFile(filepath.Join(tree, KernelVersionFile)) //nolint:gosec // a path inside the checkout this command was run in
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(body)), nil
}

// kernelPackageProblems answers every reason pkg cannot carry this proof. It
// returns each reason as one sentence.
//
// The function collects every reason rather than return only the first. If it
// returned only the first, an operator would fix one problem, rerun a
// thirty-minute kernel build, and then receive the next problem. The operator
// would pay twice for one answer.
//
// A wantVersion of the empty string skips the version check. An operator's own
// explicitly named package gets this value because naming it IS the choice.
func kernelPackageProblems(pkg, arch, wantVersion string) ([]string, error) {
	header, known := kernelMagic[arch]
	if !known {
		return nil, errUnsupportedArch
	}

	var problems []string
	var tb textbuf.Buffer

	vmlinuz := filepath.Join(pkg, "vmlinuz")
	switch info, err := os.Stat(vmlinuz); {
	case err != nil || info.IsDir():
		problems = append(problems, tb.Reset().Str("no vmlinuz at ").Str(vmlinuz).String())
	case !hasMagicAt(vmlinuz, header.offset, header.magic):
		problems = append(problems, tb.Reset().Str("vmlinuz at ").Str(vmlinuz).
			Str(" is not an ").Str(arch).Str(" kernel (magic ").Str(header.magic).
			Str(" not found at offset ").Int(header.offset).Byte(')').String())
	}

	problems = append(problems, l2tpProblems(pkg)...)
	if wantVersion != "" {
		problems = append(problems, versionProblems(pkg, wantVersion)...)
	}
	return problems, nil
}

// l2tpProblems answers whether the package carries PPPoL2TP either built into
// the kernel or as a loadable module.
//
// Both forms are accepted because both work. The runtime kernel this repository
// builds carries L2TP built in and ships no loadable module for it. A
// distribution kernel an operator points at with their own package usually
// ships the module.
func l2tpProblems(pkg string) []string {
	builtins, loadable := scanModuleTree(filepath.Join(pkg, "lib", "modules"))

	var tb textbuf.Buffer
	if len(builtins) == 0 && !loadable {
		return []string{tb.Str("no lib/modules/*/").Str(modulesBuiltin).Str(" under ").Str(pkg).String()}
	}

	for _, path := range builtins {
		body, err := os.ReadFile(path) //nolint:gosec // a path the walk below found inside the package
		if err == nil && strings.Contains(string(body), l2tpModuleName) {
			return nil
		}
	}
	if loadable {
		return nil
	}
	return []string{tb.Str("kernel package ").Str(pkg).Str(" has no PPPoL2TP support: ").
		Str(l2tpModuleName).Str(" is neither in ").Str(modulesBuiltin).
		Str(" nor a loadable module").String()}
}

// scanModuleTree answers every modules.builtin under the tree. It also answers
// whether any loadable l2tp_ppp module is present.
//
// The check matches the module file by PREFIX because a distribution compresses
// it. l2tp_ppp.ko, .ko.xz and .ko.zst are all the same module. A check that
// matched only the bare name would refuse a kernel that carries it.
func scanModuleTree(root string) ([]string, bool) {
	var builtins []string
	loadable := false

	filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error { //nolint:errcheck // a tree that cannot be walked answers "nothing found", which is the refusal the caller wants
		if err != nil || entry.IsDir() {
			return nil //nolint:nilerr // an unreadable subtree is not a reason to stop reading the rest
		}
		switch name := entry.Name(); {
		case name == modulesBuiltin:
			builtins = append(builtins, path)
		case strings.HasPrefix(name, l2tpModuleFile):
			loadable = true
		}
		return nil
	})
	return builtins, loadable
}

// versionProblems answers whether the package's module tree belongs to the
// pinned kernel.
//
// A release matches if it IS the pinned version or begins with that version and
// a dash. A local build uses this form for its own release suffix. Without this
// check, the code silently reuses a staged package left by a build before a
// kernel.version bump. The appliance then boots a kernel whose modules do not
// match it.
func versionProblems(pkg, want string) []string {
	entries, err := os.ReadDir(filepath.Join(pkg, "lib", "modules"))
	var releases []string
	if err == nil {
		for _, entry := range entries {
			if entry.IsDir() {
				releases = append(releases, entry.Name())
			}
		}
	}

	var tb textbuf.Buffer
	local := tb.Str(want).Byte('-').String()
	for _, release := range releases {
		if release == want || strings.HasPrefix(release, local) {
			return nil
		}
	}

	found := strings.Join(releases, ", ")
	if found == "" {
		found = "none"
	}
	return []string{tb.Reset().Str("kernel package ").Str(pkg).Str(" carries release(s) ").Str(found).
		Str(", not the pinned kernel version ").Str(want).String()}
}

// hasMagicAt reports whether the file carries magic at offset.
func hasMagicAt(path string, offset int64, magic string) bool {
	file, err := os.Open(path) //nolint:gosec // a path inside the package this command was pointed at
	if err != nil {
		return false
	}
	defer file.Close() //nolint:errcheck // a read-only file

	buf := make([]byte, len(magic))
	if _, err := file.ReadAt(buf, offset); err != nil {
		return false
	}
	return string(buf) == magic
}

// kernelPackageError answers the refusal for a package that cannot carry the
// proof. It names every reason and the command that rebuilds the package.
//
// The message contains the rebuild command because it is the operator's next
// action. The command is not obvious because the kernel is built in a container
// and takes about thirty minutes on a cache miss.
func kernelPackageError(context, arch string, problems []string) error {
	var tb textbuf.Buffer
	tb.Str("unusable kernel package (").Str(context).Str("):\n  ").
		Str(strings.Join(problems, "\n  ")).
		Str("\nrebuild it with: ./ze appliance kernel --target runtime --arch ").Str(arch).
		Str(" (about 30 minutes on a cache miss, needs Docker)")
	return errors.New(tb.String())
}
