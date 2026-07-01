#!/usr/bin/env python3
"""QEMU evidence for the installer's Ventoy ISO path (AC-3, Story 2).

Ventoy multi-boot USBs keep the appliance ISO as a *file* on an exFAT/FAT data
partition rather than burning it as the boot medium. The installer handles this
in tryVentoyISO (internal/install/disk/iso.go:69): it scans /sys/block, mounts
each node as vfat/exfat, globs `*.iso` / `*/*.iso`, loop-mounts each, and
accepts the one whose /ze-install/media-id matches ze.media-id.

This harness proves that path end to end:
  1. build the appliance ISO exactly as effective-install-iso-qemu.py does
     (`ze appliance iso`), reusing that module;
  2. drop that ISO onto a raw disk formatted as a single FAT volume via mtools
     (mformat/mcopy, no root, no loop). blockDevNodes (iso.go:275) enumerates
     the whole-disk node first, so a bare FAT filesystem on /dev/vdb (no
     partition table) is what tryVentoyISO mounts;
  3. boot the installer kernel+initrd DIRECTLY (Ventoy is not the boot medium
     here) with ze.source=iso, attaching the blank target (vda) and the FAT data
     disk (vdb);
  4. assert the installer found the ISO via Ventoy, wrote the image, and powered
     off (the ISO path never runs the PXE zefs-injection branch).

Missing tooling (kernel / QEMU / grub / xorriso / mtools / debugfs) prints one
`INSTALL-VENTOY-QEMU: SKIP <reason>` and exits 0. The installer kernel is
operator-supplied and must additionally have iso9660 + loop (CONFIG_BLK_DEV_LOOP)
+ vfat built in (=y).

NOTE: this script cannot be exercised on a machine without mtools + the ISO
build chain; the FAT-construction flags (mformat -F, mcopy) and the vdb device
ordering are its two first-run-validation points.
"""

from __future__ import annotations

import importlib.util
import math
import os
import shutil
import subprocess
import sys
import tempfile
from pathlib import Path


def repo_root() -> Path:
    here = Path(__file__).resolve()
    for parent in [here.parent, *here.parents]:
        if (parent / "go.mod").is_file() and (parent / "cmd" / "ze").is_dir():
            return parent
    raise SystemExit("cannot locate repository root")


ROOT = repo_root()


def load_iso_module():
    path = ROOT / "scripts" / "evidence" / "effective-install-iso-qemu.py"
    spec = importlib.util.spec_from_file_location("effective_install_iso_qemu", path)
    if spec is None or spec.loader is None:
        raise SystemExit(f"cannot load {path}")
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


iso = load_iso_module()
base = iso.base  # the PXE base module (initrd/image/qemu helpers)

# Emitted by internal/install/disk/iso.go:131 when the Ventoy scan succeeds.
VENTOY_MARK = "found installer ISO via Ventoy"


def skip(reason: str) -> int:
    print(f"INSTALL-VENTOY-QEMU: SKIP {reason}")
    return 0


def log(msg: str) -> None:
    print(f"INSTALL-VENTOY-QEMU: {msg}")


def run(cmd: list[str]) -> subprocess.CompletedProcess[str]:
    return subprocess.run(
        cmd, text=True, check=False, stdout=subprocess.PIPE, stderr=subprocess.STDOUT
    )


def have_mtools() -> str | None:
    for tool in ("mformat", "mcopy"):
        if shutil.which(tool) is None:
            return (
                f"{tool} not found (install mtools) — needed to build the FAT data disk"
            )
    return None


def extract_media_id(iso_path: Path, work: Path) -> str:
    """Read /ze-install/media-id out of the built ISO (xorriso), for ze.media-id."""
    dest = work / "media-id"
    xorriso = shutil.which("xorriso")
    if xorriso is None:
        raise SystemExit("xorriso missing after prerequisite check")
    r = run(
        [
            xorriso,
            "-osirrox",
            "on",
            "-indev",
            str(iso_path),
            "-extract",
            "/ze-install/media-id",
            str(dest),
        ]
    )
    if r.returncode != 0 or not dest.is_file():
        raise SystemExit(f"extract media-id from ISO failed:\n{r.stdout}")
    return dest.read_text().strip()


def build_ventoy_data_disk(iso_path: Path, work: Path) -> Path:
    """A raw disk holding the ISO on a single whole-disk FAT volume (mtools).

    No partition table: blockDevNodes(iso.go:275) enumerates /dev/vdb itself
    before any /dev/vdbN, so the installer mounts the bare FAT filesystem on the
    whole disk. FAT32 needs a floor of ~33 MiB of clusters, so size the volume to
    the ISO plus generous slack.
    """
    disk = work / "ventoy-data.img"
    iso_bytes = iso_path.stat().st_size
    size_bytes = max(iso_bytes + (64 << 20), 96 << 20)
    size_bytes = int(math.ceil(size_bytes / (1 << 20))) * (1 << 20)  # round to MiB
    with open(disk, "wb") as f:
        f.truncate(size_bytes)

    # mformat treats the -i image as one FAT volume (drive "::"); -F forces FAT32.
    fmt = run(["mformat", "-i", str(disk), "-F", "::"])
    if fmt.returncode != 0:
        raise SystemExit(f"mformat FAT data disk failed:\n{fmt.stdout}")
    # mcopy writes into the FAT image without mounting (no root).
    cp = run(["mcopy", "-i", str(disk), str(iso_path), f"::/{iso_path.name}"])
    if cp.returncode != 0:
        raise SystemExit(f"mcopy ISO into FAT data disk failed:\n{cp.stdout}")
    return disk


def boot_ventoy(
    kernel: Path,
    initrd: Path,
    target: Path,
    ventoy_disk: Path,
    media_id: str,
    ze_image: str,
    timeout: float,
) -> str:
    """Boot kernel+initrd directly against a blank target + the FAT data disk.

    vda is the blank install target, vdb carries the ISO. ISO installs power off,
    so -no-reboot makes QEMU exit at completion (a hang would run to the timeout).
    """
    console = "ttyAMA0" if base.ARCH == "arm64" else "ttyS0"
    append = (
        f"console={console} ze.source=iso ze.media-id={media_id} "
        f"ze.image={ze_image} ze.target=/dev/vda panic=-1"
    )
    cmd = base.qemu_base(needs_bios=False) + [
        "-no-reboot",
        "-kernel",
        str(kernel),
        "-initrd",
        str(initrd),
        "-append",
        append,
        "-drive",
        f"file={target},format=raw,if=virtio",
        "-drive",
        f"file={ventoy_disk},format=raw,if=virtio",
    ]
    return base._run_capture(cmd, timeout)


def main() -> int:
    if shutil.which(base.QEMU_BIN) is None:
        return skip(f"{base.QEMU_BIN} not found")
    kernel = base.find_installer_kernel()
    if kernel is None:
        return skip(
            "no installer kernel — set ZE_INSTALL_KERNEL to a vmlinuz with "
            "IP_PNP_DHCP/VIRTIO_BLK/EXT4/ISO9660/VFAT/BLK_DEV_LOOP built in (=y)"
        )
    if (
        shutil.which("grub-mkstandalone") is None
        and shutil.which("grub2-mkstandalone") is None
    ):
        return skip("grub-mkstandalone not found (needed to build the appliance ISO)")
    if shutil.which("xorriso") is None:
        return skip("xorriso not found (needed to build/inspect the appliance ISO)")
    mtools_skip = have_mtools()
    if mtools_skip:
        return skip(mtools_skip)
    tool_skip = base.have_image_build_tools(ROOT)
    if tool_skip:
        return skip(tool_skip)
    initrd_skip = base.have_initrd_build_tools()
    if initrd_skip:
        return skip(initrd_skip)

    work = Path(tempfile.mkdtemp(prefix="ze-install-ventoy-qemu-"))
    keep = os.environ.get("ZE_INSTALL_KEEP") == "1"
    try:
        log(f"arch={base.ARCH} accel={base.QEMU_ACCEL} kernel={kernel}")
        initrd = base.build_initrd(ROOT, work)
        log(f"initrd built ({initrd.stat().st_size} bytes)")

        ze = iso.build_host_ze(ROOT, work)
        image, app_dir, env = iso.prepare_image(ROOT, work, ze)
        log(f"image ready {image}")

        # The ISO stores the image compressed at /ze-install/images/<name>.gz and
        # its manifest/cmdline reference that name (cmd_iso.go:650,663,755), so the
        # installer expects ze.image=<name>.gz.
        ze_image = image.name + ".gz"
        iso_path = iso.create_iso(ze, kernel, initrd, image, app_dir, env, "/dev/vda")
        log(f"iso ready {iso_path}")

        media_id = extract_media_id(iso_path, work)
        log(f"media-id={media_id}")

        ventoy_disk = build_ventoy_data_disk(iso_path, work)
        log(f"ventoy FAT data disk built ({ventoy_disk.stat().st_size} bytes)")

        target = work / "ventoy-target.img"
        with open(target, "wb") as f:
            f.truncate(image.stat().st_size)

        serial = boot_ventoy(
            kernel,
            initrd,
            target,
            ventoy_disk,
            media_id,
            ze_image,
            timeout=float(os.environ.get("ZE_INSTALL_BOOT_TIMEOUT", "300")),
        )

        if VENTOY_MARK not in serial:
            sys.stdout.write(serial)
            log("FAIL installer did not locate the ISO via the Ventoy scan")
            return 1
        if base.MARK_WRITTEN not in serial or iso.MARK_ISO_DONE not in serial:
            sys.stdout.write(serial)
            log("FAIL Ventoy install did not write the image and power off")
            return 1
        if base.MARK_DONE in serial:
            sys.stdout.write(serial)
            log("FAIL Ventoy (ISO) path rebooted instead of powering off")
            return 1
        log("PASS Ventoy ISO located on FAT data disk, image written, powered off")
        return 0
    finally:
        if not keep:
            shutil.rmtree(work, ignore_errors=True)


if __name__ == "__main__":
    sys.exit(main())
