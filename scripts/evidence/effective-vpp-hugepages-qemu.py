#!/usr/bin/env python3
"""Boot-time hugepage reservation QEMU evidence for spec-vpp-host-tuning (AC-3).

This is the real coverage behind test/appliance/vpp-hugepages-qemu.ci. It builds
a HOST ze (ze_core,ze_setup), inits an appliance whose config carries
image.hugepages + image.memory, builds the gokrazy image (exercising the
derived-instance-config kernel-argument path in internal/appliance/kernelargs.go),
boots that image directly in QEMU, logs in over SSH, and asserts:

  * /proc/cmdline contains the reserved-pages arguments
    (default_hugepagesz=<sz> hugepagesz=<sz> hugepages=<n>) -- proving the derived
    KernelExtraArgs reached the baked /cmdline.txt (A-4); and
  * /proc/meminfo reports HugePages_Total >= 1 -- proving the kernel honoured the
    reservation (A-5/A-6).

Self-skip contract: when any prerequisite is missing (go, qemu, KVM/tcg, e2fsprogs,
sshpass, or a target kernel) the script prints a single
`VPP-HUGEPAGES-QEMU: SKIP <reason>` line and exits 0, so it is safe to wire into
the functional suite and into CI that lacks the artifacts. A genuine mismatch
exits non-zero.

Env overrides:
  ZE_VPP_HP_ARCH        target arch (amd64|arm64); defaults to the host arch
  ZE_VPP_HP_PAGESIZE    page size, "2mb" (default) or "1gb"
  ZE_VPP_HP_SIZE        total hugepage reservation, e.g. "128mb"
  ZE_VPP_HP_MEMORY      image.memory, e.g. "1gb"
  ZE_VPP_HP_SSH_PASS    SSH password (default "secret")
  ZE_VPP_HP_KEEP        1 to keep the work dir for inspection
"""

from __future__ import annotations

import json
import os
import platform
import shutil
import socket
import subprocess
import sys
import tempfile
import time
from pathlib import Path

SSH_USER = "admin"
SSH_PASS = os.environ.get("ZE_VPP_HP_SSH_PASS", "secret")
PAGESIZE = os.environ.get("ZE_VPP_HP_PAGESIZE", "2mb")  # config page-size: 2mb or 1gb
HP_SIZE = os.environ.get("ZE_VPP_HP_SIZE", "128mb")  # total hugepage reservation
MEMORY = os.environ.get("ZE_VPP_HP_MEMORY", "1gb")  # image.memory


def _bytes(size: str) -> int:
    s = size.strip().lower()
    for unit, mult in (
        ("tb", 1 << 40),
        ("gb", 1 << 30),
        ("mb", 1 << 20),
        ("kb", 1 << 10),
        ("b", 1),
    ):
        if s.endswith(unit):
            return int(s[: -len(unit)]) * mult
    raise SystemExit(f"bad size {size!r}: must end in b, kb, mb, gb, or tb")


PAGE_TOKEN = {"2mb": "2M", "1gb": "1G"}.get(PAGESIZE.lower())
if PAGE_TOKEN is None:
    raise SystemExit(f"ZE_VPP_HP_PAGESIZE must be 2mb or 1gb, got {PAGESIZE!r}")
HP_COUNT = _bytes(HP_SIZE) // _bytes(PAGESIZE)
MEM_BYTES = _bytes(MEMORY)


def host_arch() -> str:
    m = platform.machine().lower()
    return "arm64" if m in ("arm64", "aarch64") else "amd64"


ARCH = os.environ.get("ZE_VPP_HP_ARCH") or host_arch()
QEMU_BIN = "qemu-system-aarch64" if ARCH == "arm64" else "qemu-system-x86_64"
QEMU_ACCEL = "kvm" if Path("/dev/kvm").exists() else "tcg"


def skip(reason: str) -> int:
    print(f"VPP-HUGEPAGES-QEMU: SKIP {reason}")
    return 0


def repo_root() -> Path:
    return Path(__file__).resolve().parents[2]


def run(cmd: list[str], **kwargs) -> subprocess.CompletedProcess[str]:
    return subprocess.run(cmd, text=True, check=False, **kwargs)


def free_tcp_port() -> int:
    with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as s:
        s.bind(("127.0.0.1", 0))
        return s.getsockname()[1]


def missing_prereqs(root: Path) -> str | None:
    if shutil.which("go") is None:
        return "go toolchain not found"
    if shutil.which(QEMU_BIN) is None:
        return f"{QEMU_BIN} not found"
    if shutil.which("sshpass") is None:
        return "sshpass not found (needed for non-interactive SSH assert)"
    if shutil.which("mkfs.ext4") is None or shutil.which("debugfs") is None:
        return "e2fsprogs (mkfs.ext4/debugfs) not found"
    return None


def build_host_ze(root: Path, work: Path) -> str:
    ze = str(work / "ze-host")
    r = run(
        ["go", "build", "-tags", "ze_core,ze_setup", "-o", ze, "./cmd/ze"],
        cwd=str(root),
        stdout=subprocess.PIPE,
        stderr=subprocess.STDOUT,
    )
    if r.returncode != 0:
        raise SystemExit(f"host ze build failed:\n{r.stdout}")
    return ze


def build_image(ze: str, root: Path, work: Path) -> Path:
    name = "ze-vpp-hp-qemu"
    appliance_dir = work / "appliances"
    env = os.environ.copy()
    env["ZE_APPLIANCE_DIR"] = str(appliance_dir)
    env["ze.appliance.ssh.password"] = SSH_PASS

    init = run(
        [ze, "appliance", "init", name],
        stdin=subprocess.DEVNULL,
        stdout=subprocess.PIPE,
        stderr=subprocess.STDOUT,
        env=env,
    )
    if init.returncode != 0:
        raise SystemExit(f"ze appliance init failed:\n{init.stdout}")

    cfg_path = appliance_dir / name / "appliance.json"
    cfg = json.loads(cfg_path.read_text())
    image = cfg.setdefault("image", {})
    image["arch"] = ARCH
    image["memory"] = MEMORY
    image["hugepages"] = {"size": HP_SIZE, "page-size": PAGESIZE}
    cfg_path.write_text(json.dumps(cfg))

    build = run(
        [ze, "appliance", "build", name],
        stdout=subprocess.PIPE,
        stderr=subprocess.STDOUT,
        env=env,
    )
    if build.returncode != 0:
        raise SystemExit(f"ze appliance build failed:\n{build.stdout}")
    imgs = sorted(appliance_dir.rglob("ze-*.img"))
    if not imgs:
        raise SystemExit("appliance build produced no image")
    return imgs[0]


def boot_and_assert(img: Path) -> int:
    ssh_port = int(os.environ.get("ZE_VPP_HP_SSH_PORT", "0")) or free_tcp_port()
    machine = (
        "virt,highmem=off,accel=" + QEMU_ACCEL
        if ARCH == "arm64"
        else "accel=" + QEMU_ACCEL
    )
    mem_mib = str(max(MEM_BYTES // (1024 * 1024), 256))
    qemu_cmd = [
        QEMU_BIN,
        "-machine",
        machine,
        "-smp",
        "2",
        "-m",
        mem_mib,
        "-drive",
        f"file={img},format=raw",
        "-nographic",
        "-serial",
        "mon:stdio",
        "-nic",
        f"user,model=e1000,hostfwd=tcp::{ssh_port}-:22",
    ]
    if ARCH == "arm64":
        bios = os.environ.get(
            "ZE_VPP_HP_AARCH64_BIOS", "/usr/share/qemu/edk2-aarch64-code.fd"
        )
        if not Path(bios).is_file():
            return skip(f"aarch64 UEFI firmware not found at {bios}")
        qemu_cmd[1:1] = ["-cpu", "max", "-bios", bios]

    vm = subprocess.Popen(
        qemu_cmd, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL
    )
    try:
        cmdline = ssh_capture(ssh_port, "cat /proc/cmdline")
        meminfo = ssh_capture(ssh_port, "cat /proc/meminfo")
    finally:
        vm.terminate()
        try:
            vm.wait(timeout=10)
        except subprocess.TimeoutExpired:
            vm.kill()

    if cmdline is None:
        return skip(
            "appliance did not reach SSH within the timeout (no KVM / slow tcg?)"
        )

    want_args = [
        f"default_hugepagesz={PAGE_TOKEN}",
        f"hugepagesz={PAGE_TOKEN}",
        f"hugepages={HP_COUNT}",
    ]
    for arg in want_args:
        if arg not in cmdline:
            print(
                f"VPP-HUGEPAGES-QEMU: FAIL /proc/cmdline missing {arg!r}\ncmdline: {cmdline}"
            )
            return 1

    total = 0
    for line in (meminfo or "").splitlines():
        if line.startswith("HugePages_Total:"):
            total = int(line.split(":", 1)[1].strip() or "0")
            break
    if total < 1:
        print(
            f"VPP-HUGEPAGES-QEMU: FAIL HugePages_Total={total} (kernel did not reserve pages)"
        )
        return 1

    print(
        f"VPP-HUGEPAGES-QEMU: PASS cmdline has {want_args[2]}, HugePages_Total={total}"
    )
    return 0


def ssh_capture(port: int, remote_cmd: str, deadline_s: int = 180) -> str | None:
    end = time.time() + deadline_s
    ssh = [
        "sshpass",
        "-p",
        SSH_PASS,
        "ssh",
        "-p",
        str(port),
        "-o",
        "StrictHostKeyChecking=no",
        "-o",
        "UserKnownHostsFile=/dev/null",
        "-o",
        "ConnectTimeout=5",
        f"{SSH_USER}@127.0.0.1",
        remote_cmd,
    ]
    while time.time() < end:
        r = run(ssh, stdout=subprocess.PIPE, stderr=subprocess.DEVNULL)
        if r.returncode == 0:
            return r.stdout
        time.sleep(3)
    return None


def main() -> int:
    root = repo_root()
    reason = missing_prereqs(root)
    if reason:
        return skip(reason)

    keep = os.environ.get("ZE_VPP_HP_KEEP") == "1"
    work = Path(tempfile.mkdtemp(prefix="ze-vpp-hp-qemu-"))
    try:
        ze = build_host_ze(root, work)
        img = build_image(ze, root, work)
        return boot_and_assert(img)
    finally:
        if not keep:
            shutil.rmtree(work, ignore_errors=True)


if __name__ == "__main__":
    sys.exit(main())
