#!/usr/bin/env python3
"""Boot-time hugepage reservation QEMU evidence for spec-vpp-host-tuning (AC-3).

This is the real coverage behind test/appliance/vpp-hugepages-qemu.ci. It builds
a HOST ze (ze_core,ze_setup), inits an appliance whose config carries
image.hugepages + image.memory, builds the gokrazy image (exercising the
derived-instance-config kernel-argument path in internal/appliance/kernelargs.go),
boots that image directly in QEMU, logs in over SSH, and asserts, through the Ze
CLI (the appliance's SSH server is that CLI, not a Unix shell -- `cat /proc/...`
is answered with "error: unknown command"):

  * `show host kernel | json` .cmdline contains the reserved-pages arguments
    (default_hugepagesz=<sz> hugepagesz=<sz> hugepages=<n>) -- proving the derived
    KernelExtraArgs reached the baked /cmdline.txt (A-4); and
  * `show host memory | json` .hugepages-total >= 1 -- proving the kernel honored
    the reservation, and that an operator can see it (A-5/A-6).

Self-skip contract: when any prerequisite is missing (go, qemu, e2fsprogs,
sshpass, or a target kernel) the script prints a single
`VPP-HUGEPAGES-QEMU: SKIP <reason>` line and exits 0, so it is safe to wire into
the functional suite and into CI that lacks the artifacts. A genuine mismatch
exits non-zero -- and so does an appliance that never answers when a HARDWARE
accelerator was in use, because that is a failure, not a slow machine. Only the
software-emulation (tcg) case may still skip on a no-answer.

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
# Per-OS accelerator, same shape as effective-install-qemu.py: macOS has no
# /dev/kvm and uses the Apple hypervisor. On Linux, existence is not usability --
# /dev/kvm is root:kvm 0660, so a user outside the kvm group sees the node and
# qemu then dies with "Could not access KVM kernel module: Permission denied"
# instead of falling back. Probe access, not presence.
QEMU_ACCEL = (
    "hvf"
    if sys.platform == "darwin"
    else ("kvm" if os.access("/dev/kvm", os.R_OK | os.W_OK) else "tcg")
)


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


def build_hint(output: str) -> str:
    """Name the one-time setup step when the build output shows it is missing.

    `ze appliance build` resolves the gokrazy system packages strictly from the
    repo-local gokrazy/modcache with GOPROXY=off (internal/appliance/cmd_build.go
    ensureModcache). On a checkout where `make ze-gokrazy-deps` has never run,
    that cache holds neither github.com/rtr7/kernel nor the Go toolchain the
    builddir modules pin, and gok reports "toolchain not available" -- which
    reads as a broken Go installation and is not one. This does NOT skip: the
    prerequisite is one documented command away, and skipping would delete the
    only coverage of the boot-time hugepage reservation
    (ai/rules/fail-closed-guards.md). It makes the failure actionable.
    """
    for marker in ("toolchain not available", "incomplete packages", "GOPROXY=off"):
        if marker in output:
            return (
                " (gokrazy/modcache looks unpopulated -- run `make ze-gokrazy-deps`"
                " once, then retry)"
            )
    return ""


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
        raise SystemExit(
            f"ze appliance build failed:{build_hint(build.stdout)}\n{build.stdout}"
        )
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

    # Keep the serial console: when the appliance does not answer, its boot log
    # is the only evidence of why, and discarding it once cost a full debugging
    # session (the daemon was up in 10s; the QUERY was wrong).
    # Beside the image, which lives under the project tmp/ (ai/rules/testing.md),
    # so the boot log is where the operator is already looking after a failure.
    console = img.parent / f"ze-vpp-hp-console-{ssh_port}.log"
    with console.open("wb") as clog:
        vm = subprocess.Popen(qemu_cmd, stdout=clog, stderr=subprocess.STDOUT)
        try:
            # The appliance SSH server is the Ze CLI, NOT a Unix shell: `cat
            # /proc/cmdline` is answered with "error: unknown command". Ask the
            # operator surface instead, which also proves the reservation is
            # visible to whoever has to verify it on a real box.
            kernel_json, kernel_err = ssh_capture(ssh_port, "show host kernel | json")
            memory_json, memory_err = ssh_capture(ssh_port, "show host memory | json")
        finally:
            vm.terminate()
            try:
                vm.wait(timeout=10)
            except subprocess.TimeoutExpired:
                vm.kill()

    if kernel_json is None:
        tail = console_tail(console)
        detail = (
            f"last ssh error: {kernel_err}" if kernel_err else "no ssh error captured"
        )
        # Under a hardware accelerator a boot that never answers is a FAILURE, not
        # a slow machine. Reporting it as SKIP is how this stayed broken.
        if QEMU_ACCEL in ("kvm", "hvf"):
            print(
                f"VPP-HUGEPAGES-QEMU: FAIL appliance did not answer over SSH"
                f" (accel={QEMU_ACCEL}); {detail}\n{tail}"
            )
            return 1
        return skip(
            f"appliance did not answer over SSH within the timeout"
            f" (accel={QEMU_ACCEL}, software emulation); {detail}"
        )

    try:
        cmdline = json.loads(kernel_json).get("cmdline", "")
    except ValueError as exc:
        print(
            f"VPP-HUGEPAGES-QEMU: FAIL `show host kernel | json` is not JSON: {exc}\n{kernel_json}"
        )
        return 1

    want_args = [
        f"default_hugepagesz={PAGE_TOKEN}",
        f"hugepagesz={PAGE_TOKEN}",
        f"hugepages={HP_COUNT}",
    ]
    for arg in want_args:
        if arg not in cmdline:
            print(
                f"VPP-HUGEPAGES-QEMU: FAIL kernel cmdline missing {arg!r}\ncmdline: {cmdline}"
            )
            return 1

    if memory_json is None:
        print(
            "VPP-HUGEPAGES-QEMU: FAIL kernel cmdline is right but"
            f" `show host memory | json` never answered; last ssh error: {memory_err}"
        )
        return 1
    try:
        total = int(json.loads(memory_json).get("hugepages-total", 0))
    except ValueError as exc:
        print(
            f"VPP-HUGEPAGES-QEMU: FAIL `show host memory | json` is not JSON: {exc}\n{memory_json}"
        )
        return 1

    # The cmdline only proves the REQUEST reached the kernel. hugepages-total is
    # the kernel's answer, and it is the assertion that can fail on a box with
    # too little contiguous memory.
    if total < 1:
        print(
            f"VPP-HUGEPAGES-QEMU: FAIL hugepages-total={total}"
            f" (cmdline asked for {HP_COUNT}; kernel reserved none)"
        )
        return 1

    print(
        f"VPP-HUGEPAGES-QEMU: PASS cmdline has {want_args[2]}, hugepages-total={total}"
    )
    return 0


def console_tail(path: Path, lines: int = 25) -> str:
    """Return the last lines of the captured serial console, for failure output."""
    try:
        text = path.read_text(errors="replace")
    except OSError as exc:
        return f"  (no serial console captured: {exc})"
    tail = text.splitlines()[-lines:]
    return "  serial console tail:\n" + "\n".join(f"    {line}" for line in tail)


def ssh_capture(
    port: int, remote_cmd: str, deadline_s: int = 180
) -> tuple[str | None, str]:
    """Run one Ze CLI command over SSH, retrying until the deadline.

    Returns (stdout, last_error). Reporting the last error matters: a refused
    connection (still booting) and a connected session whose COMMAND was
    rejected are the same returncode to the caller, and conflating them is what
    made "error: unknown command" look like a boot timeout for months.
    """
    end = time.time() + deadline_s
    last_error = ""
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
        r = run(ssh, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
        if r.returncode == 0:
            return r.stdout, ""
        last_error = (r.stderr or r.stdout or "").strip().splitlines()[-1:]
        last_error = last_error[0] if last_error else f"exit {r.returncode}"
        time.sleep(3)
    return None, last_error


def main() -> int:
    root = repo_root()
    reason = missing_prereqs(root)
    if reason:
        return skip(reason)

    keep = os.environ.get("ZE_VPP_HP_KEEP") == "1"
    # Project tmp/, never the system temp dir (ai/rules/testing.md). This build
    # writes a ~2GB image; on a host where /tmp is a small tmpfs, or on a
    # different filesystem from the checkout, mkdtemp() with no dir= put it
    # somewhere the operator could neither predict nor clean up. A failed run
    # leaves it behind for inspection under tmp/, where it is gitignored and
    # visible.
    work_root = root / "tmp" / "vpp-hugepages-qemu"
    work_root.mkdir(parents=True, exist_ok=True)
    work = Path(tempfile.mkdtemp(prefix="run-", dir=work_root))
    try:
        ze = build_host_ze(root, work)
        img = build_image(ze, root, work)
        return boot_and_assert(img)
    finally:
        if not keep:
            shutil.rmtree(work, ignore_errors=True)


if __name__ == "__main__":
    sys.exit(main())
