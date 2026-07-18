#!/usr/bin/env python3
"""Build the ze installer kernel inside a QEMU Alpine VM.

Replaces the Docker-based build. Downloads Alpine virt ISO, boots QEMU
with virtio-9p file sharing, installs build dependencies, and runs
build.py inside the VM. The built kernel lands in build/ on the host
via the 9p mount.

Usage:
    python3 qemu-build.py                                # arm64, qemu profile
    python3 qemu-build.py --arch amd64                   # x86_64, qemu profile
    python3 qemu-build.py --profile hardware             # arm64, hardware profile
    KERNEL_VERSION=6.12.9 python3 qemu-build.py          # pin kernel version

Prerequisites: qemu (brew install qemu), python3, curl.
"""

from __future__ import annotations

import os
import select
import shutil
import signal
import socket
import subprocess
import sys
import time
from pathlib import Path

# Avoid __pycache__ write contention when many qemu-build.py processes run
# concurrently under the parallel functional-test suite.
sys.dont_write_bytecode = True

import ksource  # noqa: E402

# The Alpine ISO (version pin + verified, durable download) has ONE implementation, shared
# with scripts/evidence/qemu-run.py, so the two producers cannot drift.
sys.path.insert(0, str(Path(__file__).resolve().parents[2] / "scripts" / "evidence"))
from alpine_iso import ALPINE_MINOR, ALPINE_VERSION, ensure_iso  # noqa: E402,F401

VM_MEMORY_MIN = 9216
VM_MEMORY_MAX = 12288
VM_MEMORY_FRACTION = 4
BOOT_TIMEOUT = 120
BUILD_TIMEOUT = 14400
# The kernel version has a single source of truth: internal/appliance/kernel.version,
# read once by tools/kernel-builder/run.py. run.py always passes --version, so this
# tool carries no default of its own.

BUILD_PACKAGES = (
    "build-base bc bison flex elfutils-dev openssl-dev "
    "linux-headers perl wget xz diffutils findutils cpio patch kmod zstd python3"
)


def script_dir() -> Path:
    return Path(__file__).resolve().parent


def repo_root() -> Path:
    here = script_dir()
    for parent in [here, *here.parents]:
        if (parent / "go.mod").is_file():
            return parent
    raise SystemExit("cannot locate repository root (no go.mod found)")


def ccache_dir() -> Path:
    root = repo_root()
    d = root / "tmp" / "qemu" / "ccache"
    d.mkdir(parents=True, exist_ok=True)
    return d


def validate_version(value: str) -> str:
    if (
        value == ""
        or value.startswith(".")
        or value.endswith(".")
        or any(ch not in "0123456789." for ch in value)
    ):
        raise SystemExit(f"--version must contain digits and dots only: {value}")
    return value


def validate_arch(value: str) -> str:
    if value in ("arm64", "aarch64"):
        return "arm64"
    if value in ("amd64", "x86_64"):
        return "amd64"
    raise SystemExit(f"--arch must be arm64 or amd64: {value}")


def validate_profile(value: str) -> str:
    if (
        not value
        or value[0] not in "abcdefghijklmnopqrstuvwxyz0123456789"
        or any(ch not in "abcdefghijklmnopqrstuvwxyz0123456789-" for ch in value)
    ):
        raise SystemExit(f"--profile must match ^[a-z0-9][a-z0-9-]*$: {value}")
    return value


def validate_jobs(value: str) -> str:
    if value and any(ch not in "0123456789" for ch in value):
        raise SystemExit(f"--jobs must be numeric: {value}")
    return value


def build_dir(target_arch: str) -> Path:
    root = repo_root()
    d = root / "tmp" / "qemu" / "build" / _alpine_arch(target_arch)
    d.mkdir(parents=True, exist_ok=True)
    return d


def repo_relative(value: str, flag: str) -> str:
    path = Path(value)
    allowed = set("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789/._-")
    if (
        value == ""
        or path.is_absolute()
        or ".." in path.parts
        or any(ch not in allowed for ch in value)
    ):
        raise SystemExit(
            f"{flag} must be a repo-relative path with only [A-Za-z0-9/._-]: {value}"
        )
    return path.as_posix()


def find_free_port() -> int:
    with socket.socket() as s:
        s.bind(("", 0))
        return s.getsockname()[1]


def qemu_binary(target_arch: str) -> str:
    if target_arch in ("arm64", "aarch64"):
        return "qemu-system-aarch64"
    return "qemu-system-x86_64"


def _alpine_arch(target_arch: str) -> str:
    if target_arch in ("arm64", "aarch64"):
        return "aarch64"
    return "x86_64"


def _find_aarch64_firmware() -> Path | None:
    candidates = [
        Path("/opt/homebrew/share/qemu/edk2-aarch64-code.fd"),
        Path("/usr/share/qemu/edk2-aarch64-code.fd"),
        Path("/usr/share/AAVMF/AAVMF_CODE.fd"),
        Path("/usr/share/edk2/aarch64/QEMU_EFI-pflash.raw"),
    ]
    return next((p for p in candidates if p.is_file()), None)


def _vm_cpus() -> str:
    n = os.cpu_count() or 4
    return str(max(2, n))


def _vm_memory() -> str:
    try:
        total = os.sysconf("SC_PAGE_SIZE") * os.sysconf("SC_PHYS_PAGES")
        quarter = total // VM_MEMORY_FRACTION // (1024 * 1024)
        return str(min(VM_MEMORY_MAX, max(VM_MEMORY_MIN, quarter)))
    except (ValueError, OSError):
        return str(VM_MEMORY_MIN)


def _available_accels(qemu: str) -> list[str]:
    result = subprocess.run(
        [qemu, "-accel", "help"],
        capture_output=True,
        text=True,
        check=False,
    )
    known = {"hvf", "kvm", "tcg", "whpx", "xen"}
    accels = []
    for line in result.stdout.splitlines():
        stripped = line.strip()
        if stripped in known:
            accels.append(stripped)
    return accels


def _build_qemu_args(
    iso: Path,
    workspace: Path,
    target_arch: str,
    ssh_port: int,
    memory: str,
    ccache_path: Path | None = None,
    build_path: Path | None = None,
    firmware_path: Path | None = None,
) -> list[str]:
    qemu = qemu_binary(target_arch)
    args = [qemu]

    if _alpine_arch(target_arch) == "aarch64":
        fw = _find_aarch64_firmware()
        if fw is None:
            raise SystemExit(
                "aarch64 UEFI firmware not found; "
                "install QEMU with firmware (brew install qemu) "
                "or qemu-efi-aarch64 on Debian/Ubuntu"
            )
        args.extend(
            [
                "-machine",
                "virt,highmem=on",
                "-cpu",
                "max",
                "-bios",
                str(fw),
            ]
        )

    available = _available_accels(qemu)
    for accel in ("hvf", "kvm"):
        if accel in available:
            args.extend(["-accel", accel])
    if "tcg" in available:
        args.extend(["-accel", "tcg,thread=multi,tb-size=512"])

    args.extend(
        [
            "-smp",
            _vm_cpus(),
            "-m",
            memory,
            "-cdrom",
            str(iso),
            "-boot",
            "d",
            "-nographic",
            "-serial",
            "mon:stdio",
            "-netdev",
            f"user,id=net0,hostfwd=tcp::{ssh_port}-:22",
            "-device",
            "virtio-net-pci,netdev=net0",
            "-virtfs",
            f"local,path={workspace},mount_tag=workspace,"
            f"security_model=none,id=ws0,readonly=off",
        ]
    )
    if ccache_path is not None:
        args.extend(
            [
                "-virtfs",
                f"local,path={ccache_path},mount_tag=ccache,"
                f"security_model=none,id=cc0,readonly=off",
            ]
        )
    if build_path is not None:
        args.extend(
            [
                "-virtfs",
                f"local,path={build_path},mount_tag=builddir,"
                f"security_model=none,id=bd0,readonly=off",
            ]
        )
    if firmware_path is not None:
        args.extend(
            [
                "-virtfs",
                f"local,path={firmware_path},mount_tag=firmware,"
                f"security_model=none,id=fw0,readonly=on",
            ]
        )
    return args


def _expect(proc: subprocess.Popen[str], pattern: str, timeout: float) -> bool:
    deadline = time.time() + timeout
    buf = ""
    fd = proc.stdout.fileno()
    while time.time() < deadline:
        if proc.poll() is not None:
            return False
        ready, _, _ = select.select([fd], [], [], 1.0)
        if ready:
            chunk = os.read(fd, 4096)
            if not chunk:
                return False
            buf += chunk.decode("utf-8", errors="replace")
            if pattern in buf:
                return True
            if len(buf) > 20000:
                buf = buf[-10000:]
    return False


def _send(proc: subprocess.Popen[str], cmd: str) -> None:
    proc.stdin.write(cmd + "\n")
    proc.stdin.flush()


def _ssh_opts(port: int) -> list[str]:
    return [
        "-o",
        "StrictHostKeyChecking=no",
        "-o",
        "UserKnownHostsFile=/dev/null",
        "-o",
        "PreferredAuthentications=none",
        "-o",
        "LogLevel=ERROR",
        "-p",
        str(port),
    ]


def _wait_for_ssh(port: int, timeout: float) -> None:
    deadline = time.time() + timeout
    opts = _ssh_opts(port)
    while time.time() < deadline:
        result = subprocess.run(
            ["ssh", *opts, "-o", "ConnectTimeout=2", "root@localhost", "true"],
            stdin=subprocess.DEVNULL,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            check=False,
        )
        if result.returncode == 0:
            return
        time.sleep(2)
    raise RuntimeError(f"SSH not reachable on port {port} after {timeout}s")


def _ssh_run(cmd: str, port: int, timeout: int) -> int:
    result = subprocess.run(
        [
            "ssh",
            *_ssh_opts(port),
            "-o",
            "ServerAliveInterval=30",
            "root@localhost",
            cmd,
        ],
        check=False,
        timeout=timeout,
    )
    return result.returncode


def _shell_quote(s: str) -> str:
    return "'" + s.replace("'", "'\\''") + "'"


def _run_build(
    iso: Path,
    workspace: Path,
    target_arch: str,
    profile: str,
    version: str,
    jobs: str,
    src_dir: str,
    out_dir: str,
    builder_dir: str,
    modules: str,
    patches_dir: str,
    firmware_dir: str,
    fragments: list[str],
) -> int:
    ssh_port = find_free_port()
    cc_dir = ccache_dir()
    bd_dir = build_dir(target_arch)
    memory = _vm_memory()
    fw_path = Path(firmware_dir) if firmware_dir else None
    args = _build_qemu_args(
        iso, workspace, target_arch, ssh_port, memory, cc_dir, bd_dir, fw_path
    )

    print(
        f">>> booting Alpine VM ({_alpine_arch(target_arch)}, "
        f"{memory}MB RAM, ssh port {ssh_port})...",
        file=sys.stderr,
    )
    proc = subprocess.Popen(
        args,
        stdin=subprocess.PIPE,
        stdout=subprocess.PIPE,
        stderr=subprocess.DEVNULL,
        text=True,
    )

    def cleanup(signum=None, _frame=None):
        proc.kill()
        proc.wait()
        if signum:
            raise SystemExit(128 + signum)

    signal.signal(signal.SIGTERM, cleanup)
    signal.signal(signal.SIGINT, cleanup)

    try:
        if not _expect(proc, "login:", BOOT_TIMEOUT):
            raise RuntimeError("timeout waiting for VM login prompt")

        time.sleep(1)
        _send(proc, "root")
        time.sleep(3)

        bootstrap = (
            "setup-interfaces -a 2>/dev/null; "
            "ifup eth0 2>/dev/null; ifup lo 2>/dev/null; "
            "echo nameserver 8.8.8.8 > /etc/resolv.conf; "
            "apk add --no-cache openssh; "
            "echo PermitRootLogin yes >> /etc/ssh/sshd_config; "
            "echo PermitEmptyPasswords yes >> /etc/ssh/sshd_config; "
            "passwd -d root; "
            "ssh-keygen -t ed25519 -f /etc/ssh/ssh_host_ed25519_key "
            "-N '' 2>/dev/null; "
            "ssh-keygen -t rsa -f /etc/ssh/ssh_host_rsa_key "
            "-N '' 2>/dev/null; "
            "/usr/sbin/sshd; "
            "echo SSHD_READY"
        )

        ready = False
        for attempt in range(1, 6):
            _send(proc, bootstrap)
            print(
                f"  bootstrapping VM, attempt {attempt}...",
                file=sys.stderr,
            )
            if not _expect(proc, "SSHD_READY", 180):
                continue
            print("  waiting for SSH...", file=sys.stderr)
            try:
                _wait_for_ssh(ssh_port, timeout=60)
                ready = True
                break
            except RuntimeError:
                print("  SSH not up; retrying...", file=sys.stderr)
                time.sleep(2)
        if not ready:
            raise RuntimeError("VM bootstrap failed: SSH not reachable")

        print(
            "  VM ready, installing build dependencies...",
            file=sys.stderr,
        )

        setup = " && ".join(
            [
                "set -e",
                f"printf 'https://dl-cdn.alpinelinux.org/alpine/"
                f"v{ALPINE_VERSION}/main\\n"
                f"https://dl-cdn.alpinelinux.org/alpine/"
                f"v{ALPINE_VERSION}/community\\n' > /etc/apk/repositories",
                "apk update",
                f"apk add --no-cache {BUILD_PACKAGES} ccache",
                "mkdir -p /workspace",
                "mount -t 9p -o trans=virtio,version=9p2000.L,msize=1048576 "
                "workspace /workspace",
                "mkdir -p /ccache",
                "mount -t 9p -o trans=virtio,version=9p2000.L,msize=1048576 "
                "ccache /ccache",
                "mkdir -p /build",
                "mount -t 9p -o trans=virtio,version=9p2000.L,msize=1048576 "
                "builddir /build",
            ]
        )
        if firmware_dir:
            setup += (
                " && mkdir -p /firmware"
                " && mount -t 9p -o trans=virtio,version=9p2000.L,msize=1048576,ro "
                "firmware /firmware"
            )

        src_vm = f"/workspace/{src_dir}"
        out_vm = f"/workspace/{out_dir}"
        builder_vm = f"/workspace/{builder_dir}"
        build_env = (
            "CCACHE_DIR=/ccache CCACHE_MAXSIZE=5G PATH=/usr/lib/ccache/bin:$PATH"
        )
        build_args = [
            "python3",
            f"{builder_vm}/build.py",
            "--version",
            version,
            "--arch",
            target_arch,
            "--profile",
            profile,
            "--src-dir",
            src_vm,
            "--out-dir",
            out_vm,
            "--modules",
            modules,
        ]
        for fragment in fragments:
            build_args.extend(["--fragment", f"/workspace/{fragment}"])
        if patches_dir:
            build_args.extend(["--patches-dir", f"/workspace/{patches_dir}"])
        if firmware_dir:
            build_args.extend(["--firmware-dir", "/firmware"])
        if jobs:
            build_args.extend(["--jobs", jobs])
        build_cmd = build_env + " " + " ".join(_shell_quote(arg) for arg in build_args)

        full_cmd = f"sh -c {_shell_quote(setup + ' && ' + build_cmd)}"

        tarball = ksource.tarball_name(version)
        tarball_path = bd_dir / tarball
        if not tarball_path.is_file():
            url = ksource.tarball_url(version)
            print(
                f"  downloading {tarball} on host...",
                file=sys.stderr,
            )
            dl = subprocess.run(
                ["curl", "-fSL", "--progress-bar", "-o", str(tarball_path), url],
                check=False,
            )
            if dl.returncode != 0:
                tarball_path.unlink(missing_ok=True)
                raise RuntimeError(f"kernel tarball download failed: {url}")
        else:
            print(
                f"  {tarball} cached on host",
                file=sys.stderr,
            )

        print(
            f"  building kernel "
            f"(version={version}, arch={target_arch}, profile={profile})...",
            file=sys.stderr,
        )
        rc = _ssh_run(full_cmd, ssh_port, BUILD_TIMEOUT)

        if rc == 0:
            print(">>> kernel build complete", file=sys.stderr)
        else:
            print(
                f">>> kernel build FAILED (exit code {rc})",
                file=sys.stderr,
            )
        return rc

    except RuntimeError as e:
        print(f"error: {e}", file=sys.stderr)
        return 1
    finally:
        proc.kill()
        proc.wait()


def main() -> int:
    import argparse

    parser = argparse.ArgumentParser(
        description="Build a Ze kernel in a QEMU Alpine VM.",
    )
    parser.add_argument(
        "--arch",
        default=os.environ.get("ARCH", "arm64"),
        help="Target architecture: arm64 or amd64 (default: arm64)",
    )
    parser.add_argument(
        "--profile",
        default=os.environ.get("PROFILE", "qemu"),
        help="Kernel profile token (default: qemu)",
    )
    parser.add_argument(
        "--version",
        default=os.environ.get("KERNEL_VERSION"),
        required="KERNEL_VERSION" not in os.environ,
        help="Linux kernel version (required; or set KERNEL_VERSION). "
        "Canonical: internal/appliance/kernel.version",
    )
    parser.add_argument(
        "--jobs",
        default=os.environ.get("JOBS", ""),
        help="Parallel make jobs (default: nproc inside VM)",
    )
    parser.add_argument(
        "--src-dir",
        default=os.environ.get("SRC_DIR", "tools/installer-kernel"),
        help="Repo-relative directory containing kernel.config and profile config",
    )
    parser.add_argument(
        "--out-dir",
        default=os.environ.get("OUT_DIR", "build/kernel"),
        help="Repo-relative output directory",
    )
    parser.add_argument(
        "--builder-dir",
        default=os.environ.get("BUILDER_DIR", "tools/kernel-builder"),
        help="Repo-relative directory containing build.py",
    )
    parser.add_argument(
        "--modules",
        choices=("yes", "no"),
        default=os.environ.get("MODULES", "no"),
        help="Install runtime modules and vmlinuz artifacts",
    )
    parser.add_argument(
        "--patches-dir",
        default=os.environ.get("PATCHES_DIR", ""),
        help="Repo-relative patch series directory",
    )
    parser.add_argument(
        "--firmware-dir",
        default="",
        help="Host-absolute path to firmware directory for kernel embedding",
    )
    parser.add_argument(
        "--fragment",
        action="append",
        default=[],
        help="Repo-relative config fragment path; repeat to pass Go-resolved order",
    )
    args = parser.parse_args()

    root = repo_root()
    arch = validate_arch(args.arch)
    profile = validate_profile(args.profile)
    version = validate_version(args.version)
    jobs = validate_jobs(args.jobs)
    src_dir = repo_relative(args.src_dir, "--src-dir")
    out_dir = repo_relative(args.out_dir, "--out-dir")
    builder_dir = repo_relative(args.builder_dir, "--builder-dir")
    patches_dir = ""
    if args.patches_dir:
        patches_dir = repo_relative(args.patches_dir, "--patches-dir")
    fragments = [repo_relative(fragment, "--fragment") for fragment in args.fragment]
    qemu = qemu_binary(arch)
    if shutil.which(qemu) is None:
        raise SystemExit(f"missing: {qemu} (install: brew install qemu)")
    iso = ensure_iso(_alpine_arch(arch))  # shared, verified, durable-cached ISO
    return _run_build(
        iso,
        root,
        arch,
        profile,
        version,
        jobs,
        src_dir,
        out_dir,
        builder_dir,
        args.modules,
        patches_dir,
        args.firmware_dir,
        fragments,
    )


if __name__ == "__main__":
    raise SystemExit(main())
