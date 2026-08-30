// Design: docs/architecture/testing/qemu-integration.md -- Ventoy installer evidence
// Overview: install.go -- base install primitives
// Related: install_iso.go -- ISO primitives imported by this proof
package qemu

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

func (installer *Installer) extractMediaID(ctx context.Context, iso, work string) (string, error) {
	target := filepath.Join(work, "media-id")
	if err := installer.extractISOFile(ctx, iso, "/ze-install/media-id", target); err != nil {
		return "", fmt.Errorf("extract media-id from ISO failed: %w", err)
	}
	data, err := installer.ops.FS.ReadFile(target)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

func ventoyDiskBytes(isoBytes int64) int64 {
	size := max(isoBytes+(64<<20), 96<<20)
	const mib = int64(1 << 20)
	return ((size + mib - 1) / mib) * mib
}

func (installer *Installer) buildVentoyDisk(ctx context.Context, iso, work string) (string, error) {
	info, err := installer.ops.FS.Stat(iso)
	if err != nil {
		return "", err
	}
	disk := filepath.Join(work, "ventoy-data.img")
	if err := truncateInstallFile(disk, ventoyDiskBytes(info.Size())); err != nil {
		return "", err
	}
	result, err := installer.run(ctx, commandSpec{Name: "mformat", Args: []string{"-i", disk, "-F", "::"}, Env: installer.ops.Environ()})
	if err != nil {
		return "", err
	}
	var tb textbuf.Buffer
	if result.Code != 0 {
		return "", errors.New(tb.Str("mformat FAT data disk failed:\n").Str(result.Stdout).Str(result.Stderr).String())
	}
	tb.Str("::/").Str(filepath.Base(iso))
	result, err = installer.run(ctx, commandSpec{Name: "mcopy", Args: []string{"-i", disk, iso, tb.String()}, Env: installer.ops.Environ()})
	if err != nil {
		return "", err
	}
	if result.Code != 0 {
		tb.Reset()
		return "", errors.New(tb.Str("mcopy ISO into FAT data disk failed:\n").Str(result.Stdout).Str(result.Stderr).String())
	}
	return disk, nil
}

// VentoyArgv returns the direct-kernel invocation with target disk before the FAT disk.
func (installer *Installer) VentoyArgv(initrd, target, ventoyDisk, mediaID, image string) ([]string, error) {
	base := installer.qemuBase(false)
	console := installConsoleAMD64
	if installer.Options.Arch == ArchARM64 {
		console = installConsoleARM64
	}
	var tb textbuf.Buffer
	line := tb.Str("console=").Str(console).Str(" ze.source=iso ze.media-id=").Str(mediaID).
		Str(" ze.image=").Str(image).Str(" ze.target=").Str(installTarget).Str(" panic=-1").String()
	tb.Reset()
	targetDrive := tb.Str("file=").Str(target).Str(",format=raw,if=virtio").String()
	tb.Reset()
	ventoyDrive := tb.Str("file=").Str(ventoyDisk).Str(",format=raw,if=virtio").String()
	return append(base, "-no-reboot", "-kernel", installer.Options.Kernel,
		"-initrd", initrd, "-append", line,
		"-drive", targetDrive, "-drive", ventoyDrive), nil
}

func (installer *Installer) bootVentoy(ctx context.Context, initrd, target, ventoyDisk, mediaID, image string) (string, error) {
	argv, err := installer.VentoyArgv(initrd, target, ventoyDisk, mediaID, image)
	if err != nil {
		return "", err
	}
	return installer.runCapture(ctx, argv, installer.Options.BootTimeout)
}

func (installer *Installer) executeVentoy(ctx context.Context, work string, report InstallReport) (InstallReport, error) {
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
	mediaID, err := installer.extractMediaID(ctx, iso, work)
	if err != nil {
		return report, err
	}
	tb.Reset()
	report.line(installer.prefix(), tb.Str("media-id=").Str(mediaID).String())
	ventoy, err := installer.buildVentoyDisk(ctx, iso, work)
	if err != nil {
		return report, err
	}
	ventoyInfo, err := installer.ops.FS.Stat(ventoy)
	if err != nil {
		return report, err
	}
	report.artifact("ventoy-fat-disk", ventoy, ventoyInfo.Size())
	tb.Reset()
	report.line(installer.prefix(), tb.Str("ventoy FAT data disk built (").
		Int(ventoyInfo.Size()).Str(" bytes)").String())
	target := filepath.Join(work, "ventoy-target.img")
	if err := truncateInstallFile(target, imageInfo.Size()); err != nil {
		return report, err
	}
	serial, err := installer.bootVentoy(ctx, initrd, target, ventoy, mediaID, filepath.Base(image.Image)+".gz")
	if err != nil {
		return report, err
	}
	if !strings.Contains(serial, InstallMarkVentoy) {
		report.lines = append(report.lines, serial)
		return installer.fail(report, "installer did not locate the ISO via the Ventoy scan")
	}
	if !strings.Contains(serial, InstallMarkWritten) || !strings.Contains(serial, InstallMarkISODone) {
		report.lines = append(report.lines, serial)
		return installer.fail(report, "Ventoy install did not write the image and power off")
	}
	if strings.Contains(serial, InstallMarkDone) {
		report.lines = append(report.lines, serial)
		return installer.fail(report, "Ventoy (ISO) path rebooted instead of powering off")
	}
	report.check("ventoy-scan", InstallVerdictPass, mediaID)
	report.line(installer.prefix(), "PASS Ventoy ISO located on FAT data disk, image written, powered off")
	report.Verdict = InstallVerdictPass
	return report, nil
}
