#!/usr/bin/env python3
"""End-to-end QEMU evidence for the ze appliance ISO installer chain.

This is the real coverage behind test/install/qemu-iso.ci. It builds the
installer initrd, builds a credential-bearing appliance image, wraps that exact
image in a bootable ISO via `ze appliance iso`, boots the ISO in QEMU
against a blank disk, checks the installer serial markers, verifies that the ISO
path did not run the PXE-only zefs injection branch, compares the source and
ISO-contained image hashes, inspects the written GPT partition layout, verifies
that the ISO installer powers off instead of rebooting back into removable
media, and finally boots the written disk and logs in over SSH with the
appliance's embedded ZeFS credentials.

The installer kernel is operator-supplied by design. For ISO evidence it must
also have ISO9660 and CD-ROM block support built in (=y), because the initrd has
no modules. Missing prerequisites print one `INSTALL-ISO-QEMU: SKIP <reason>`
line and exit 0; genuine install or boot failures exit non-zero.
"""

from __future__ import annotations

import hashlib
import importlib.util
import json
import os
import shutil
import struct
import subprocess
import sys
import tempfile
from pathlib import Path

from homebrew import brew_files, brew_keg_dirs

# The ISO builder and this QEMU proof support both amd64 (x86_64 UEFI) and arm64
# (aarch64 UEFI). amd64 UEFI is the proven default; arm64 is OPT-IN via
# ZE_INSTALL_ARCH=arm64 until it has a green QEMU proof, so an arm64 host still
# defaults to the validated amd64 path rather than the unproven arm64 one. Set
# this before importing effective-install-qemu.py, which computes ARCH at import.
os.environ.setdefault("ZE_INSTALL_ARCH", "amd64")


def repo_root() -> Path:
    here = Path(__file__).resolve()
    for parent in [here.parent, *here.parents]:
        if (parent / "go.mod").is_file() and (parent / "cmd" / "ze").is_dir():
            return parent
    raise SystemExit("cannot locate repository root")


ROOT = repo_root()


def load_pxe_module():
    path = ROOT / "scripts" / "evidence" / "effective-install-qemu.py"
    spec = importlib.util.spec_from_file_location("effective_install_qemu", path)
    if spec is None or spec.loader is None:
        raise SystemExit(f"cannot load {path}")
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


base = load_pxe_module()
IMAGE_NAME = "ze-install-qemu"
SSH_PASS = os.environ.get("ZE_INSTALL_SSH_PASS", "secret")

MARK_ISO_DONE = "ISO installation complete, powering off"


def skip(reason: str) -> int:
    print(f"INSTALL-ISO-QEMU: SKIP {reason}")
    return 0


def run(cmd: list[str], **kwargs) -> subprocess.CompletedProcess[str]:
    return subprocess.run(cmd, text=True, check=False, **kwargs)


def sha256_file(path: Path) -> str:
    h = hashlib.sha256()
    with open(path, "rb") as f:
        for chunk in iter(lambda: f.read(1 << 20), b""):
            h.update(chunk)
    return h.hexdigest()


def build_host_ze(root: Path, work: Path) -> Path:
    ze = work / "ze-host"
    built = run(
        ["go", "build", "-tags", "ze_core,ze_distro", "-o", str(ze), "./cmd/ze"],
        cwd=str(root),
        env={**os.environ, "CGO_ENABLED": "0"},
        stdout=subprocess.PIPE,
        stderr=subprocess.STDOUT,
    )
    if built.returncode != 0:
        raise SystemExit(f"host ze build failed:\n{built.stdout}")
    return ze


def write_checksum(path: Path) -> None:
    path.with_name(path.name + ".sha256").write_text(
        f"{sha256_file(path)}  {path.name}\n"
    )


def init_appliance(ze: Path, appliance_dir: Path, env: dict[str, str]) -> Path:
    init = run(
        [str(ze), "appliance", "init", IMAGE_NAME],
        stdin=subprocess.DEVNULL,
        stdout=subprocess.PIPE,
        stderr=subprocess.STDOUT,
        env=env,
    )
    if init.returncode != 0:
        raise SystemExit(f"ze appliance init failed:\n{init.stdout}")
    return appliance_dir / IMAGE_NAME


def prepare_image(
    root: Path, work: Path, ze: Path
) -> tuple[Path, Path, dict[str, str]]:
    appliance_dir = work / "appliances"
    env = os.environ.copy()
    env["ZE_APPLIANCE_DIR"] = str(appliance_dir)
    env["ze.appliance.ssh.password"] = SSH_PASS
    # One assignment, in order: prepending inside the loop would reverse the
    # list and put the oldest Cellar version first.
    e2fs_sbins = [str(d) for d in brew_keg_dirs("e2fsprogs")]
    if e2fs_sbins:
        env["PATH"] = ":".join([*e2fs_sbins, *filter(None, [env.get("PATH", "")])])

    app_dir = init_appliance(ze, appliance_dir, env)

    override = os.environ.get("ZE_INSTALL_IMAGE")
    if override:
        src = Path(override)
        if not src.is_file():
            raise SystemExit(f"ZE_INSTALL_IMAGE does not exist: {src}")
        dst = app_dir / "ze-override.img"
        shutil.copy(src, dst)
        write_checksum(dst)
        return dst, app_dir, env

    # Match the appliance config to ZE_INSTALL_ARCH/base.ARCH. `ze install
    # appliance build` reads image.arch from appliance.json, not GOKRAZY_ARCH.
    cfg_path = app_dir / "appliance.json"
    cfg = json.loads(cfg_path.read_text())
    cfg.setdefault("image", {})["arch"] = base.ARCH
    size_override = os.environ.get("ZE_INSTALL_IMAGE_SIZE")
    if size_override:
        try:
            size_bytes = int(size_override)
        except ValueError:
            raise SystemExit(
                f"ZE_INSTALL_IMAGE_SIZE must be an integer byte count, got {size_override!r}"
            ) from None
        cfg.setdefault("image", {})["size-bytes"] = size_bytes
    cfg_path.write_text(json.dumps(cfg))

    build = run(
        [str(ze), "appliance", "build", IMAGE_NAME],
        stdout=subprocess.PIPE,
        stderr=subprocess.STDOUT,
        env=env,
    )
    if build.returncode != 0:
        raise SystemExit(f"ze appliance build failed:\n{build.stdout}")
    imgs = sorted(app_dir.glob("ze-*.img"))
    if not imgs:
        raise SystemExit("appliance build produced no image")
    return imgs[-1], app_dir, env


def create_iso(
    ze: Path,
    kernel: Path,
    initrd: Path,
    image: Path,
    app_dir: Path,
    env: dict[str, str],
    target: str,
) -> Path:
    iso = app_dir / "ze-install.iso"
    cmd = [
        str(ze),
        "appliance",
        "iso",
        "--kernel",
        str(kernel),
        "--initrd",
        str(initrd),
        "--image",
        image.name,
        "--output",
        str(iso),
    ]
    if target:
        cmd.extend(["--target", target])
    cmd.append(IMAGE_NAME)
    built = run(cmd, stdout=subprocess.PIPE, stderr=subprocess.STDOUT, env=env)
    if built.returncode != 0:
        raise SystemExit(f"ze appliance iso failed:\n{built.stdout}")
    if not iso.is_file():
        raise SystemExit("ISO command returned success but produced no ISO")
    return iso


def extract_iso_image(iso: Path, image_name: str, work: Path) -> Path:
    compressed = image_name.endswith(".gz")
    raw_dest = work / "iso-contained.img"
    extract_dest = work / (
        "iso-contained.img.gz" if compressed else "iso-contained.img"
    )
    xorriso = shutil.which("xorriso")
    if xorriso is None:
        raise SystemExit("xorriso missing after prerequisite check")
    extracted = run(
        [
            xorriso,
            "-osirrox",
            "on",
            "-indev",
            str(iso),
            "-extract",
            f"/ze-install/images/{image_name}",
            str(extract_dest),
        ],
        stdout=subprocess.PIPE,
        stderr=subprocess.STDOUT,
    )
    if extracted.returncode != 0 or not extract_dest.is_file():
        raise SystemExit(f"extract ISO-contained image failed:\n{extracted.stdout}")
    if compressed:
        import gzip as gzmod

        with gzmod.open(extract_dest, "rb") as gz_in:
            with open(raw_dest, "wb") as out:
                shutil.copyfileobj(gz_in, out)
        return raw_dest
    return extract_dest


def find_x86_uefi_firmware() -> Path | None:
    override = os.environ.get("ZE_INSTALL_X86_UEFI_BIOS")
    if override and Path(override).is_file():
        return Path(override)
    for path in brew_files("share/qemu/edk2-x86_64-code.fd"):
        return path
    for candidate in (
        "/usr/share/OVMF/OVMF_CODE.fd",
        "/usr/share/ovmf/OVMF.fd",
        "/usr/share/edk2/ovmf/OVMF_CODE.fd",
        "/usr/share/qemu/OVMF.fd",
    ):
        path = Path(candidate)
        if path.is_file():
            return path
    return None


def find_aarch64_uefi_firmware() -> Path | None:
    override = os.environ.get("ZE_INSTALL_AARCH64_BIOS")
    if override and Path(override).is_file():
        return Path(override)
    for path in brew_files("share/qemu/edk2-aarch64-code.fd"):
        return path
    for candidate in (
        "/usr/share/AAVMF/AAVMF_CODE.fd",
        "/usr/share/edk2/aarch64/QEMU_EFI.fd",
        "/usr/share/qemu-efi-aarch64/QEMU_EFI.fd",
        "/usr/share/qemu/edk2-aarch64-code.fd",
    ):
        path = Path(candidate)
        if path.is_file():
            return path
    return None


def find_uefi_firmware() -> tuple[Path | None, str]:
    """Return (firmware, skip_reason) for the active arch's UEFI image."""
    if base.ARCH == "arm64":
        return (
            find_aarch64_uefi_firmware(),
            "aarch64 UEFI firmware not found (set ZE_INSTALL_AARCH64_BIOS)",
        )
    return (
        find_x86_uefi_firmware(),
        "x86_64 UEFI firmware not found (set ZE_INSTALL_X86_UEFI_BIOS)",
    )


def iso_cdrom_args(iso: Path) -> list[str]:
    """Attach the install ISO as a CD-ROM appropriate to the machine type.

    amd64 (q35/pc) has an IDE bus and honours -boot d. arm64 `virt` has no IDE,
    so the ISO is attached as a virtio-scsi CD-ROM; UEFI enumerates the
    removable media's EFI application itself, so no -boot d is needed.
    """
    if base.ARCH == "arm64":
        return [
            "-drive",
            f"file={iso},if=none,id=cdrom,media=cdrom,readonly=on",
            "-device",
            "virtio-scsi-pci,id=scsi0",
            "-device",
            "scsi-cd,drive=cdrom,bus=scsi0.0",
        ]
    return [
        "-drive",
        f"file={iso},media=cdrom,readonly=on,if=ide",
        "-boot",
        "d",
    ]


def boot_iso_installer(
    iso: Path, disk: Path, extra_disk: Path, firmware: Path, timeout: float
) -> str:
    # base.qemu_base supplies the arch-correct -machine (q35/accel on amd64,
    # virt,highmem=off,-cpu max on arm64); we add the UEFI firmware for both.
    cmd = (
        base.qemu_base(needs_bios=False)
        + ["-bios", str(firmware)]
        + iso_cdrom_args(iso)
        + [
            "-drive",
            f"file={disk},format=raw,if=virtio",
            "-drive",
            f"file={extra_disk},format=raw,if=virtio",
        ]
    )
    return base._run_capture(cmd, timeout)


def gpt_entries(path: Path) -> list[tuple[int, int, str]]:
    with open(path, "rb") as f:
        f.seek(512)
        header = f.read(92)
        if header[:8] != b"EFI PART":
            raise SystemExit(f"{path} has no primary GPT header")
        entry_lba = struct.unpack_from("<Q", header, 72)[0]
        entry_count = struct.unpack_from("<I", header, 80)[0]
        entry_size = struct.unpack_from("<I", header, 84)[0]
        f.seek(entry_lba * 512)
        raw = f.read(entry_count * entry_size)
    entries: list[tuple[int, int, str]] = []
    for i in range(entry_count):
        entry = raw[i * entry_size : (i + 1) * entry_size]
        type_guid = entry[:16]
        if type_guid == b"\x00" * 16:
            continue
        first_lba = struct.unpack_from("<Q", entry, 32)[0]
        last_lba = struct.unpack_from("<Q", entry, 40)[0]
        entries.append((first_lba, last_lba, type_guid.hex()))
    return entries


def assert_partition_layout(source: Path, installed: Path) -> None:
    src_entries = gpt_entries(source)
    dst_entries = gpt_entries(installed)
    if len(src_entries) < 4:
        raise SystemExit(
            f"source image has {len(src_entries)} GPT entries, want at least 4"
        )
    if dst_entries[:4] != src_entries[:4]:
        raise SystemExit(
            "installed disk GPT entries do not match source image\n"
            f"source={src_entries[:4]}\ninstalled={dst_entries[:4]}"
        )


def main() -> int:
    if shutil.which(base.QEMU_BIN) is None:
        return skip(f"{base.QEMU_BIN} not found")
    if (
        shutil.which("grub-mkstandalone") is None
        and shutil.which("grub2-mkstandalone") is None
    ):
        # arm64 additionally needs the grub arm64-efi module set installed.
        return skip("grub-mkstandalone not found")
    if shutil.which("xorriso") is None:
        return skip("xorriso not found")
    firmware, fw_skip = find_uefi_firmware()
    if firmware is None:
        return skip(fw_skip)
    kernel = base.find_installer_kernel()
    if kernel is None:
        return skip(
            "no installer kernel — set ZE_INSTALL_KERNEL to a vmlinuz with "
            "IP_PNP_DHCP/VIRTIO_NET/VIRTIO_BLK/EXT4/ISO9660/SR built in (=y)"
        )
    tool_skip = base.have_image_build_tools(ROOT)
    if tool_skip:
        return skip(tool_skip)
    initrd_skip = base.have_initrd_build_tools()
    if initrd_skip:
        return skip(initrd_skip)

    work = Path(tempfile.mkdtemp(prefix="ze-install-iso-qemu-"))
    keep = os.environ.get("ZE_INSTALL_KEEP") == "1"
    try:
        print(
            f"INSTALL-ISO-QEMU: arch={base.ARCH} accel={base.QEMU_ACCEL} kernel={kernel}"
        )
        initrd = base.build_initrd(ROOT, work)
        print(f"INSTALL-ISO-QEMU: initrd built ({initrd.stat().st_size} bytes)")

        ze = build_host_ze(ROOT, work)
        image, app_dir, env = prepare_image(ROOT, work, ze)
        print(f"INSTALL-ISO-QEMU: image ready {image}")

        install_target = "/dev/vda"
        iso = create_iso(ze, kernel, initrd, image, app_dir, env, install_target)
        print(f"INSTALL-ISO-QEMU: iso ready {iso}")

        extracted = extract_iso_image(iso, image.name + ".gz", work)
        source_sha = sha256_file(image)
        extracted_sha = sha256_file(extracted)
        if source_sha != extracted_sha:
            print(
                "INSTALL-ISO-QEMU: FAIL ISO-contained image hash differs from source image"
            )
            print(f"source={source_sha} iso={extracted_sha}")
            return 1
        print("INSTALL-ISO-QEMU: ISO-contained image hash matches source")

        disk = work / "target.img"
        extra_disk = work / "untargeted.img"
        for qemu_disk in (disk, extra_disk):
            with open(qemu_disk, "wb") as f:
                f.truncate(image.stat().st_size)
        serial = boot_iso_installer(
            iso,
            disk,
            extra_disk,
            firmware,
            timeout=float(os.environ.get("ZE_INSTALL_BOOT_TIMEOUT", "300")),
        )
        if f"disk={install_target}" not in serial:
            sys.stdout.write(serial)
            print(
                "INSTALL-ISO-QEMU: FAIL ISO installer did not consume explicit ze.target"
            )
            return 1
        print(
            f"INSTALL-ISO-QEMU: installer consumed explicit ze.target={install_target}"
        )
        if base.MARK_WRITTEN not in serial or MARK_ISO_DONE not in serial:
            sys.stdout.write(serial)
            print(
                "INSTALL-ISO-QEMU: FAIL ISO installer did not report safe poweroff completion on serial"
            )
            return 1
        if base.MARK_DONE in serial:
            sys.stdout.write(serial)
            print(
                "INSTALL-ISO-QEMU: FAIL ISO path still reported reboot instead of safe poweroff"
            )
            return 1
        if "zefs database written" in serial:
            sys.stdout.write(serial)
            print("INSTALL-ISO-QEMU: FAIL ISO path ran PXE zefs injection branch")
            return 1
        print("INSTALL-ISO-QEMU: installer wrote embedded image + powered off safely")

        assert_partition_layout(image, disk)
        print("INSTALL-ISO-QEMU: installed GPT partition layout matches source image")

        if not base.boot_target_ssh(work, disk, timeout=120):
            print("INSTALL-ISO-QEMU: FAIL second boot SSH login as power user failed")
            target_serial = work / "target-serial.log"
            if target_serial.is_file():
                print("INSTALL-ISO-QEMU: --- target boot serial (tail) ---")
                data = target_serial.read_bytes()
                sys.stdout.buffer.write(data[-8000:])
                sys.stdout.flush()
            return 1
        print("INSTALL-ISO-QEMU: SSH login as embedded-ZeFS power user succeeded")

        print("INSTALL-ISO-QEMU: PASS")
        return 0
    finally:
        if not keep:
            shutil.rmtree(work, ignore_errors=True)


if __name__ == "__main__":
    sys.exit(main())
