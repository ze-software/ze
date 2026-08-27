// Design: docs/architecture/testing/qemu-integration.md -- appliance ISO evidence
// Overview: install.go -- the shared installer runner
// Related: install_build.go -- artifacts assembled into the ISO
// Related: install_ventoy.go -- Ventoy imports these ISO primitives
package qemu

import (
	"compress/gzip"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// The default preserves the appliance config's full valid image range when no exact size is set.
const installImageExtractBytesMaxDefault = int64(64 << 30)

type installISOImage struct {
	Image  string
	AppDir string
	Env    []string
	Ze     string
}

func (installer *Installer) prepareISOImage(ctx context.Context, work string) (installISOImage, error) {
	ze, err := installer.buildHostZe(ctx, work)
	if err != nil {
		return installISOImage{}, err
	}
	applianceDir := filepath.Join(work, "appliances")
	environ := installer.applianceEnv(applianceDir)
	result, err := installer.run(ctx, commandSpec{Name: ze, Args: []string{"appliance", "init", InstallApplianceName}, Dir: installer.Tree, Env: environ, Stdin: strings.NewReader("")})
	if err != nil {
		return installISOImage{}, err
	}
	var tb textbuf.Buffer
	if result.Code != 0 {
		return installISOImage{}, errors.New(tb.Str("ze appliance init failed:\n").
			Str(result.Stdout).Str(result.Stderr).String())
	}
	appDir := filepath.Join(applianceDir, InstallApplianceName)
	if installer.Options.Image != "" {
		source := installer.Options.Image
		if !filepath.IsAbs(source) {
			source = filepath.Join(installer.Tree, source)
		}
		if info, statErr := installer.ops.FS.Stat(source); statErr != nil || !info.Mode().IsRegular() {
			tb.Reset()
			return installISOImage{}, errors.New(tb.Str("ZE_INSTALL_IMAGE does not exist: ").Str(source).String())
		}
		target := filepath.Join(appDir, "ze-override.img")
		if err := copyInstallFile(source, target); err != nil {
			return installISOImage{}, err
		}
		if err := writeInstallSidecar(target); err != nil {
			return installISOImage{}, err
		}
		return installISOImage{Image: target, AppDir: appDir, Env: environ, Ze: ze}, nil
	}
	if err := installer.setApplianceArch(filepath.Join(appDir, "appliance.json")); err != nil {
		return installISOImage{}, err
	}
	result, err = installer.run(ctx, commandSpec{Name: ze, Args: []string{"appliance", "build", InstallApplianceName}, Dir: installer.Tree, Env: environ})
	if err != nil {
		return installISOImage{}, err
	}
	if result.Code != 0 {
		tb.Reset()
		return installISOImage{}, errors.New(tb.Str("ze appliance build failed:\n").
			Str(result.Stdout).Str(result.Stderr).String())
	}
	images, err := filepath.Glob(filepath.Join(appDir, "ze-*.img"))
	if err != nil {
		return installISOImage{}, err
	}
	if len(images) == 0 {
		return installISOImage{}, errors.New("appliance build produced no image")
	}
	return installISOImage{Image: images[len(images)-1], AppDir: appDir, Env: environ, Ze: ze}, nil
}

func writeInstallSidecar(path string) error {
	digest, err := installSHA256(path)
	if err != nil {
		return err
	}
	return os.WriteFile(path+".sha256", []byte(digest+"  "+filepath.Base(path)+"\n"), 0o600)
}

func (installer *Installer) createISO(ctx context.Context, image installISOImage, kernel, initrd, target string) (string, error) {
	iso := filepath.Join(image.AppDir, "ze-install.iso")
	argv := []string{"appliance", "iso", "--kernel", kernel, "--initrd", initrd, "--image", filepath.Base(image.Image), "--output", iso}
	if target != "" {
		argv = append(argv, "--target", target)
	}
	argv = append(argv, InstallApplianceName)
	result, err := installer.run(ctx, commandSpec{Name: image.Ze, Args: argv, Dir: installer.Tree, Env: image.Env})
	if err != nil {
		return "", err
	}
	if result.Code != 0 {
		var tb textbuf.Buffer
		return "", errors.New(tb.Str("ze appliance iso failed:\n").Str(result.Stdout).Str(result.Stderr).String())
	}
	if info, statErr := installer.ops.FS.Stat(iso); statErr != nil || !info.Mode().IsRegular() {
		return "", errors.New("ISO command returned success but produced no ISO")
	}
	return iso, nil
}

func (installer *Installer) extractISOFile(ctx context.Context, iso, source, target string) error {
	xorriso, err := installer.ops.Look("xorriso")
	if err != nil {
		return errors.New("xorriso missing after prerequisite check")
	}
	result, err := installer.run(ctx, commandSpec{Name: xorriso, Args: []string{"-osirrox", "on", "-indev", iso, "-extract", source, target}, Env: installer.ops.Environ()})
	if err != nil {
		return err
	}
	var tb textbuf.Buffer
	if result.Code != 0 {
		return errors.New(tb.Str("extract ISO file failed:\n").Str(result.Stdout).Str(result.Stderr).String())
	}
	if info, statErr := installer.ops.FS.Stat(target); statErr != nil || !info.Mode().IsRegular() {
		tb.Reset()
		return errors.New(tb.Str("extract ISO file produced no output: ").Str(target).String())
	}
	return nil
}

func (installer *Installer) extractISOImage(
	ctx context.Context,
	iso, imageName, work string,
) (result string, resultErr error) {
	compressed := strings.HasSuffix(imageName, ".gz")
	target := filepath.Join(work, "iso-contained.img")
	extracted := target
	if compressed {
		extracted += ".gz"
	}
	var tb textbuf.Buffer
	source := tb.Str("/ze-install/images/").Str(imageName).String()
	if err := installer.extractISOFile(ctx, iso, source, extracted); err != nil {
		return "", err
	}
	if !compressed {
		return extracted, nil
	}
	if err := installer.extractISOImageGzip(extracted, target); err != nil {
		return "", err
	}
	return target, nil
}

func (installer *Installer) extractISOImageGzip(source, target string) (resultErr error) {
	// #nosec G304 -- source is produced beneath the installer-owned work directory.
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer func() {
		resultErr = joinInstallCleanup(resultErr, input.Close, "close extracted installer image")
	}()
	archive, err := gzip.NewReader(input)
	if err != nil {
		return err
	}
	defer func() {
		resultErr = joinInstallCleanup(resultErr, archive.Close, "close installer image gzip reader")
	}()
	imageBytesMax := installer.Options.ImageSize
	if imageBytesMax == 0 {
		imageBytesMax = installImageExtractBytesMaxDefault
	}
	// #nosec G304 -- target is constructed beneath the installer-owned work directory.
	output, err := os.Create(target)
	if err != nil {
		return err
	}
	removeIncomplete := func(cause error) error {
		closeErr := output.Close()
		removeErr := os.Remove(target)
		return errors.Join(cause, closeErr, removeErr)
	}
	written, err := io.Copy(output, io.LimitReader(archive, imageBytesMax))
	if err != nil {
		return removeIncomplete(err)
	}
	if written == imageBytesMax {
		var extra [1]byte
		_, readErr := io.ReadFull(archive, extra[:])
		if readErr == nil {
			return removeIncomplete(fmt.Errorf(
				"decompress installer image: output exceeds %d-byte ceiling",
				imageBytesMax,
			))
		}
		if !errors.Is(readErr, io.EOF) {
			return removeIncomplete(readErr)
		}
	}
	if err := output.Close(); err != nil {
		removeErr := os.Remove(target)
		return errors.Join(err, removeErr)
	}
	return nil
}

func (installer *Installer) findUEFIFirmware() string {
	if installer.Options.Arch == ArchARM64 {
		if installer.Options.AArch64BIOS != "" {
			if installer.regularFile(installer.Options.AArch64BIOS) {
				return installer.Options.AArch64BIOS
			}
		}
		candidates := append((&Run{ops: installer.ops.runOps}).brewFiles(
			"share/qemu/edk2-aarch64-code.fd"),
			"/usr/share/AAVMF/AAVMF_CODE.fd",
			"/usr/share/edk2/aarch64/QEMU_EFI.fd",
			"/usr/share/qemu-efi-aarch64/QEMU_EFI.fd",
			"/usr/share/qemu/edk2-aarch64-code.fd")
		for _, candidate := range candidates {
			if installer.regularFile(candidate) {
				return candidate
			}
		}
		return ""
	}
	if installer.Options.X86UEFIBIOS != "" {
		if installer.regularFile(installer.Options.X86UEFIBIOS) {
			return installer.Options.X86UEFIBIOS
		}
	}
	candidates := append((&Run{ops: installer.ops.runOps}).brewFiles(
		"share/qemu/edk2-x86_64-code.fd"),
		"/usr/share/OVMF/OVMF_CODE.fd",
		"/usr/share/ovmf/OVMF.fd",
		"/usr/share/edk2/ovmf/OVMF_CODE.fd",
		"/usr/share/qemu/OVMF.fd")
	for _, candidate := range candidates {
		if installer.regularFile(candidate) {
			return candidate
		}
	}
	return ""
}

func (installer *Installer) regularFile(path string) bool {
	info, err := installer.ops.FS.Stat(path)
	return err == nil && info.Mode().IsRegular()
}

func (installer *Installer) isoCDROMArgs(iso string) []string {
	var tb textbuf.Buffer
	if installer.Options.Arch == ArchARM64 {
		drive := tb.Str("file=").Str(iso).Str(",if=none,id=cdrom,media=cdrom,readonly=on").String()
		return []string{"-drive", drive, "-device", "virtio-scsi-pci,id=scsi0", "-device", "scsi-cd,drive=cdrom,bus=scsi0.0"}
	}
	drive := tb.Str("file=").Str(iso).Str(",media=cdrom,readonly=on,if=ide").String()
	return []string{"-drive", drive, "-boot", "d"}
}

// ISOArgv returns the complete UEFI ISO installer invocation.
func (installer *Installer) ISOArgv(iso, disk, extraDisk, firmware string) ([]string, error) {
	base := installer.qemuBase(false)
	argv := slices.Clone(base)
	argv = append(argv, "-bios", firmware)
	argv = append(argv, installer.isoCDROMArgs(iso)...)
	var tb textbuf.Buffer
	targetDrive := tb.Str("file=").Str(disk).Str(",format=raw,if=virtio").String()
	tb.Reset()
	extraDrive := tb.Str("file=").Str(extraDisk).Str(",format=raw,if=virtio").String()
	return append(argv, "-drive", targetDrive, "-drive", extraDrive), nil
}

func (installer *Installer) bootISO(ctx context.Context, iso, disk, extraDisk, firmware string) (string, error) {
	argv, err := installer.ISOArgv(iso, disk, extraDisk, firmware)
	if err != nil {
		return "", err
	}
	return installer.runCapture(ctx, argv, installer.Options.BootTimeout)
}

type installGPTEntry struct {
	First uint64
	Last  uint64
	Type  string
}

func installGPTEntries(path string) (entries []installGPTEntry, resultErr error) {
	// #nosec G304 -- path is a resolved source image or installer-owned target disk.
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() {
		resultErr = joinInstallCleanup(resultErr, file.Close, "close installer GPT image")
	}()
	header := make([]byte, 92)
	if _, err := file.ReadAt(header, 512); err != nil {
		return nil, err
	}
	var tb textbuf.Buffer
	if string(header[:8]) != "EFI PART" {
		return nil, errors.New(tb.Str(path).Str(" has no primary GPT header").String())
	}
	entryLBA := binary.LittleEndian.Uint64(header[72:80])
	count := binary.LittleEndian.Uint32(header[80:84])
	size := binary.LittleEndian.Uint32(header[84:88])
	if count > 4096 || size < 56 || size > 4096 {
		return nil, fmt.Errorf("%s has invalid GPT entry geometry count=%d size=%d", path, count, size)
	}
	raw := make([]byte, int(count)*int(size))
	if _, err := file.ReadAt(raw, int64(entryLBA)*512); err != nil {
		return nil, err
	}
	entries = make([]installGPTEntry, 0, count)
	for index := range int(count) {
		entry := raw[index*int(size) : (index+1)*int(size)]
		zero := true
		for _, value := range entry[:16] {
			if value != 0 {
				zero = false
				break
			}
		}
		if zero {
			continue
		}
		entries = append(entries, installGPTEntry{
			First: binary.LittleEndian.Uint64(entry[32:40]),
			Last:  binary.LittleEndian.Uint64(entry[40:48]),
			Type:  tb.Hex(entry[:16]).String(),
		})
		tb.Reset()
	}
	return entries, nil
}

func assertInstallGPT(source, installed string) error {
	left, err := installGPTEntries(source)
	if err != nil {
		return err
	}
	right, err := installGPTEntries(installed)
	if err != nil {
		return err
	}
	if len(left) < 4 {
		return fmt.Errorf("source image has %d GPT entries, want at least 4", len(left))
	}
	if len(right) < 4 {
		return fmt.Errorf("installed disk has %d GPT entries, want at least 4", len(right))
	}
	for index := range 4 {
		if left[index] != right[index] {
			return fmt.Errorf("installed disk GPT entry %d does not match source image", index)
		}
	}
	return nil
}

func (installer *Installer) executeISO(ctx context.Context, work string, report InstallReport) (InstallReport, error) {
	var tb textbuf.Buffer
	report.line(installer.prefix(), tb.Str("arch=").Str(installer.Options.Arch).
		Str(" accel=").Str(installer.accelerator()).Str(" kernel=").Str(installer.Options.Kernel).String())
	tb.Reset()
	initrd, err := installer.buildInitrd(ctx, work)
	if err != nil {
		return report, err
	}
	initrdInfo, err := installer.ops.FS.Stat(initrd)
	if err != nil {
		return report, err
	}
	report.artifact("initrd", initrd, initrdInfo.Size())
	report.line(installer.prefix(), tb.Str("initrd built (").Int(initrdInfo.Size()).Str(" bytes)").String())
	image, err := installer.prepareISOImage(ctx, work)
	if err != nil {
		return report, err
	}
	imageInfo, err := installer.ops.FS.Stat(image.Image)
	if err != nil {
		return report, err
	}
	report.artifact("image", image.Image, imageInfo.Size())
	tb.Reset()
	report.line(installer.prefix(), tb.Str("image ready ").Str(image.Image).String())
	iso, err := installer.createISO(ctx, image, installer.Options.Kernel, initrd, installTarget)
	if err != nil {
		return report, err
	}
	isoInfo, err := installer.ops.FS.Stat(iso)
	if err != nil {
		return report, err
	}
	report.artifact("iso", iso, isoInfo.Size())
	tb.Reset()
	report.line(installer.prefix(), tb.Str("iso ready ").Str(iso).String())
	extracted, err := installer.extractISOImage(ctx, iso, filepath.Base(image.Image)+".gz", work)
	if err != nil {
		return report, err
	}
	sourceSHA, err := installSHA256(image.Image)
	if err != nil {
		return report, err
	}
	extractedSHA, err := installSHA256(extracted)
	if err != nil {
		return report, err
	}
	if sourceSHA != extractedSHA {
		report.line(installer.prefix(), "FAIL ISO-contained image hash differs from source image")
		tb.Reset()
		report.lines = append(report.lines, tb.Str("source=").Str(sourceSHA).Str(" iso=").Str(extractedSHA).String())
		report.Verdict = InstallVerdictFail
		return report, nil
	}
	report.check("iso-image-hash", InstallVerdictPass, sourceSHA)
	report.line(installer.prefix(), "ISO-contained image hash matches source")
	disk, extra := filepath.Join(work, "target.img"), filepath.Join(work, "untargeted.img")
	if err := truncateInstallFile(disk, imageInfo.Size()); err != nil {
		return report, err
	}
	if err := truncateInstallFile(extra, imageInfo.Size()); err != nil {
		return report, err
	}
	firmware := installer.findUEFIFirmware()
	serial, err := installer.bootISO(ctx, iso, disk, extra, firmware)
	if err != nil {
		return report, err
	}
	tb.Reset()
	targetMark := tb.Str("disk=").Str(installTarget).String()
	checks := []struct {
		ok     bool
		reason string
	}{
		{strings.Contains(serial, targetMark), "ISO installer did not consume explicit ze.target"},
		{strings.Contains(serial, InstallMarkWritten) && strings.Contains(serial, InstallMarkISODone), "ISO installer did not report safe poweroff completion on serial"},
		{!strings.Contains(serial, InstallMarkDone), "ISO path still reported reboot instead of safe poweroff"},
		{!strings.Contains(serial, "zefs database written"), "ISO path ran PXE zefs injection branch"},
	}
	for _, check := range checks {
		if !check.ok {
			report.lines = append(report.lines, serial)
			return installer.fail(report, check.reason)
		}
	}
	tb.Reset()
	report.line(installer.prefix(), tb.Str("installer consumed explicit ze.target=").Str(installTarget).String())
	report.line(installer.prefix(), "installer wrote embedded image + powered off safely")
	if err := assertInstallGPT(image.Image, disk); err != nil {
		return report, err
	}
	report.check("gpt-layout", InstallVerdictPass, "first four entries match")
	report.line(installer.prefix(), "installed GPT partition layout matches source image")
	ok, serialPath, err := installer.bootTargetSSH(ctx, work, disk, 120*time.Second)
	if err != nil {
		return report, err
	}
	if !ok {
		report.line(installer.prefix(), "FAIL second boot SSH login as power user failed")
		installer.appendSerialTail(&report, serialPath)
		report.Verdict = InstallVerdictFail
		return report, nil
	}
	report.check("ssh-login", InstallVerdictPass, "embedded ZeFS power user authenticated")
	report.line(installer.prefix(), "SSH login as embedded-ZeFS power user succeeded")
	report.line(installer.prefix(), "PASS")
	report.Verdict = InstallVerdictPass
	return report, nil
}
