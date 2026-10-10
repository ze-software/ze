// Design: docs/architecture/appliance/gokrazy-build-pins.md -- the appliance
// image boots ze's own runtime kernel and no other; this file turns a resolved
// runtime kernel tree into the Go package gokrazy reads a kernel from.
// Related: prepare.go -- Prepare assembles the package inside the prepared instance.

package instance

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/ze-software/ze/internal/appliance/kernelbuilder"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// The image carries the kernel's GPLv2 notice as a file in its root file system,
// through the ExtraFileContents of the ze package, which every appliance image
// builds. gokrazy copies only lib/modules from a kernel package into the root
// (packerbuild.go in the vendored packer), so the kernel package cannot carry it.
const (
	noticePackage   = "github.com/ze-software/ze/cmd/ze"
	linuxNoticePath = "/etc/linux-gpl-notice"
)

// addLinuxNotice returns config with the GPLv2 notice for the kernel in tree
// added to noticePackage's ExtraFileContents, preserving every other field. The
// notice derives from the tree's own provenance (kernelbuilder.ReadProvenance),
// so it names the version built and the source the build verified. A tree with
// no provenance, or one recorded before the source was, is refused: the image
// would ship a kernel whose source it cannot name.
func addLinuxNotice(config []byte, tree string) ([]byte, error) {
	record, err := kernelbuilder.ReadProvenance(filepath.Join(tree, kernelbuilder.ProvenanceName))
	if err != nil {
		return nil, fmt.Errorf("kernel package: %w", err)
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(config, &obj); err != nil {
		return nil, fmt.Errorf("parse instance config: %w", err)
	}
	packages := make(map[string]map[string]json.RawMessage)
	if raw, ok := obj["PackageConfig"]; ok {
		if err := json.Unmarshal(raw, &packages); err != nil {
			return nil, fmt.Errorf("parse PackageConfig: %w", err)
		}
	}
	pkg := packages[noticePackage]
	if pkg == nil {
		pkg = make(map[string]json.RawMessage)
	}
	files := make(map[string]string)
	if raw, ok := pkg["ExtraFileContents"]; ok {
		if err := json.Unmarshal(raw, &files); err != nil {
			return nil, fmt.Errorf("parse ExtraFileContents of %s: %w", noticePackage, err)
		}
	}
	if _, taken := files[linuxNoticePath]; taken {
		return nil, fmt.Errorf("instance config already places a file at %s, where the image's Linux GPLv2 notice goes", linuxNoticePath)
	}
	files[linuxNoticePath] = record.LinuxNotice()

	if pkg["ExtraFileContents"], err = json.Marshal(files); err != nil {
		return nil, fmt.Errorf("encode ExtraFileContents: %w", err)
	}
	packages[noticePackage] = pkg
	if obj["PackageConfig"], err = json.Marshal(packages); err != nil {
		return nil, fmt.Errorf("encode PackageConfig: %w", err)
	}
	out, err := json.MarshalIndent(obj, "", "    ")
	if err != nil {
		return nil, fmt.Errorf("encode instance config: %w", err)
	}
	return out, nil
}

// KernelModule is the module path of the appliance kernel package. gokrazy
// resolves the instance's KernelPackage through the builddir module that
// requires it, and Prepare points that require at the package it assembles.
//
// The path sits under .invalid, which RFC 2606 reserves so it never resolves.
// A prepared instance whose replace went missing therefore fails at `go list`
// rather than fetching some other module that happens to carry the name.
const KernelModule = "ze.invalid/kernel"

// kernelPackageDir names the assembled package inside a prepared parent dir.
const kernelPackageDir = "kernel"

// kernelHeader is where a vmlinuz of one architecture carries its magic. gokrazy
// refuses a kernel whose architecture differs from the image's
// (validateTargetArchMatchesKernel in the vendored packer), and checking here
// says which tree was wrong before gok is started.
type kernelHeader struct {
	offset int64
	magic  string
}

// kernelHeaders is keyed by GOARCH. x86 bzImage carries "HdrS" at 0x202
// (Documentation/arch/x86/boot.rst); an arm64 Image carries "ARM\x64" at 0x38
// (Documentation/arch/arm64/booting.rst).
var kernelHeaders = map[string]kernelHeader{
	"amd64": {offset: 0x202, magic: "HdrS"},
	"arm64": {offset: 0x38, magic: "ARMd"},
}

// The files gokrazy copies from a kernel package (kernelGlobs in the vendored
// packer's write.go), beside the lib/modules tree it copies into the root.
const (
	vmlinuzName  = "vmlinuz"
	overlaysName = "overlays"
	dtbPattern   = "*.dtb"
	cmdlineName  = "cmdline.txt"
	configName   = "config.txt"
)

// kernelCmdline is the kernel command line the package carries. gokrazy's
// packer reads <kernel package>/cmdline.txt and fails the build without it
// (writeCmdline in the vendored packer's write.go). It rewrites root=/dev/sda2
// to the image's root PARTUUID, so this spelling is what makes the root device
// right on every disk; KernelExtraArgs from the instance config is appended
// after it. The line is the one the rtr7 kernel package carried.
const kernelCmdline = "root=/dev/sda2 ro init=/gokrazy/init panic=10 oops=panic\n"

// assembleKernelPackage writes the gokrazy kernel package for a resolved runtime
// kernel tree into destination: a go.mod naming KernelModule, one Go file, the
// boot files the packer reads (cmdline.txt, config.txt), and the tree's vmlinuz, lib/modules, device trees and overlays.
//
// tree MUST be the arch-keyed cache entry the resolver answered, never
// tmp/kernel/build, which every resolver call rewrites. destination MUST NOT
// exist yet.
func assembleKernelPackage(tree, destination, arch string) error {
	if err := checkKernelArch(filepath.Join(tree, vmlinuzName), arch); err != nil {
		return err
	}
	modules := filepath.Join(tree, "lib", "modules")
	if info, err := os.Stat(modules); err != nil || !info.IsDir() {
		return fmt.Errorf("runtime kernel tree %s has no lib/modules", tree)
	}
	if err := os.Mkdir(destination, 0o750); err != nil {
		return fmt.Errorf("create kernel package %s: %w", destination, err)
	}

	var tb textbuf.Buffer
	goMod := tb.Str("module ").Str(KernelModule).Str("\n\ngo 1.26\n").String()
	if err := os.WriteFile(filepath.Join(destination, GoModName), []byte(goMod), 0o600); err != nil {
		return fmt.Errorf("write kernel package go.mod: %w", err)
	}
	// gokrazy finds the package with `go list`, which needs one Go file.
	goFile := "// Package kernel carries ze's runtime kernel for the gokrazy packer.\npackage kernel\n"
	if err := os.WriteFile(filepath.Join(destination, "kernel.go"), []byte(goFile), 0o600); err != nil {
		return fmt.Errorf("write kernel package source: %w", err)
	}

	if err := os.WriteFile(filepath.Join(destination, cmdlineName), []byte(kernelCmdline), 0o600); err != nil {
		return fmt.Errorf("write kernel package %s: %w", cmdlineName, err)
	}
	// writeConfig in the vendored packer reads config.txt as well. It is the
	// Raspberry Pi bootloader's file, which no ze target reads, so it is empty,
	// as the rtr7 package's was.
	if err := os.WriteFile(filepath.Join(destination, configName), nil, 0o600); err != nil {
		return fmt.Errorf("write kernel package %s: %w", configName, err)
	}
	if err := copyKernelFile(filepath.Join(tree, vmlinuzName), filepath.Join(destination, vmlinuzName)); err != nil {
		return err
	}
	if err := copyKernelTree(modules, filepath.Join(destination, "lib", "modules")); err != nil {
		return err
	}
	dtbs, err := filepath.Glob(filepath.Join(tree, dtbPattern))
	if err != nil {
		return fmt.Errorf("list device trees in %s: %w", tree, err)
	}
	for _, dtb := range dtbs {
		if err := copyKernelFile(dtb, filepath.Join(destination, filepath.Base(dtb))); err != nil {
			return err
		}
	}
	overlays := filepath.Join(tree, overlaysName)
	if info, err := os.Stat(overlays); err == nil && info.IsDir() {
		return copyKernelTree(overlays, filepath.Join(destination, overlaysName))
	}
	return nil
}

// checkKernelArch refuses a vmlinuz that is absent or built for another arch.
func checkKernelArch(vmlinuz, arch string) error {
	header, known := kernelHeaders[arch]
	if !known {
		return fmt.Errorf("kernel architecture %q is not amd64 or arm64", arch)
	}
	file, err := os.Open(vmlinuz) //nolint:gosec // a file inside the resolved kernel cache entry
	if err != nil {
		return fmt.Errorf("runtime kernel: %w", err)
	}
	defer file.Close() //nolint:errcheck // a read-only file

	found := make([]byte, len(header.magic))
	if _, err := file.ReadAt(found, header.offset); err != nil {
		return fmt.Errorf("read the kernel header of %s: %w", vmlinuz, err)
	}
	if string(found) != header.magic {
		return fmt.Errorf("%s is not an %s kernel: magic %q not found at offset %#x", vmlinuz, arch, header.magic, header.offset)
	}
	return nil
}

// copyKernelTree copies a directory and keeps symlinks as symlinks. A modules
// tree carries `build` and `source` links into a kernel source tree that is not
// there, and following them would fail.
func copyKernelTree(source, target string) error {
	return filepath.WalkDir(source, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		to := filepath.Join(target, rel)
		if entry.IsDir() {
			return os.MkdirAll(to, 0o750)
		}
		if entry.Type()&fs.ModeSymlink != 0 {
			link, err := os.Readlink(path)
			if err != nil {
				return err
			}
			return os.Symlink(link, to) //nolint:gosec // G122: the source is the resolved kernel cache entry, the target this build's own prepared dir
		}
		return copyKernelFile(path, to)
	})
}

// copyKernelFile copies one regular file, keeping its permission bits.
func copyKernelFile(source, target string) error {
	info, err := os.Stat(source)
	if err != nil {
		return fmt.Errorf("kernel package: %w", err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("kernel package: %s is not a regular file", source)
	}

	from, err := os.Open(source) //nolint:gosec // a file inside the resolved kernel cache entry
	if err != nil {
		return fmt.Errorf("kernel package: %w", err)
	}
	defer from.Close() //nolint:errcheck // a read-only file

	if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
		return fmt.Errorf("kernel package: %w", err)
	}
	to, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, info.Mode().Perm()) //nolint:gosec // a path under this build's prepared dir
	if err != nil {
		return fmt.Errorf("kernel package: %w", err)
	}
	if _, err := io.Copy(to, from); err != nil {
		return errors.Join(fmt.Errorf("kernel package: copy %s: %w", source, err), to.Close())
	}
	return to.Close()
}
