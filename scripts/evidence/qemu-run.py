#!/usr/bin/env python3
"""Run commands inside a QEMU Linux VM with full kernel capabilities.

Boots Alpine Linux from its virt ISO as a live system, shares the repo
via virtio-9p, and runs tests. The VM provides CAP_NET_ADMIN, network
namespaces, nftables, PPPoL2TP kernel support -- all features unavailable
in Docker Desktop or macOS.

Usage:
    python3 scripts/evidence/qemu-run.py --run "make ze-integration-test"
    python3 scripts/evidence/qemu-run.py --run "python3 scripts/evidence/effective-l2tp-ppp.py" \\
        --packages "xl2tpd ppp iproute2 iputils-ping nftables kmod"

The ISO is cached at tmp/qemu/ and reused. Each run boots fresh from ISO.
"""

from __future__ import annotations

import os
import platform
import select
import shutil
import signal
import subprocess
import sys
import time
from pathlib import Path

ALPINE_VERSION = "3.21"
ALPINE_MINOR = "3"
ALPINE_ARCH = "aarch64" if platform.machine() == "arm64" else "x86_64"
QEMU_BIN = f"qemu-system-{ALPINE_ARCH}"
GO_VERSION = "1.25.9"
VM_MEMORY = "2048"
VM_CPUS = "4"
BOOT_TIMEOUT = 60
DEFAULT_CMD_TIMEOUT = 1200
SSH_PORT = "2222"


def repo_root() -> Path:
    here = Path(__file__).resolve()
    for parent in here.parents:
        if (parent / "go.mod").is_file():
            return parent
    raise SystemExit("cannot locate repository root")


def cache_dir(root: Path) -> Path:
    d = root / "tmp" / "qemu"
    d.mkdir(parents=True, exist_ok=True)
    (d / "iso").mkdir(exist_ok=True)
    (d / "go-dl").mkdir(exist_ok=True)
    (d / "go-cache").mkdir(exist_ok=True)
    (d / "gomodcache").mkdir(exist_ok=True)
    return d


def run(cmd: list[str], **kwargs) -> subprocess.CompletedProcess[str]:
    return subprocess.run(cmd, text=True, check=False, **kwargs)


def ensure_iso(cdir: Path) -> Path:
    """Download Alpine virt ISO if not cached."""
    name = f"alpine-virt-{ALPINE_VERSION}.{ALPINE_MINOR}-{ALPINE_ARCH}.iso"
    iso = cdir / "iso" / name
    if iso.is_file():
        return iso

    print("Downloading Alpine virt ISO...", file=sys.stderr)
    url = (
        f"https://dl-cdn.alpinelinux.org/alpine/v{ALPINE_VERSION}/releases"
        f"/{ALPINE_ARCH}/{name}"
    )
    result = run(["curl", "-fSL", "--progress-bar", "-o", str(iso), url])
    if result.returncode != 0:
        iso.unlink(missing_ok=True)
        raise SystemExit(f"download failed: {url}")
    return iso


def _extract_alpine_initramfs(iso: Path) -> Path:
    """Extract initramfs-virt from Alpine ISO (needed for custom kernel boot)."""
    extract_dir = iso.parent / "alpine-extract"
    initrd = extract_dir / "boot" / "initramfs-virt"
    if initrd.is_file():
        return initrd
    extract_dir.mkdir(parents=True, exist_ok=True)
    result = run(
        ["7z", "x", str(iso), "-y", f"-o{extract_dir}"],
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
    )
    if result.returncode != 0:
        result = run(
            ["7zz", "x", str(iso), "-y", f"-o{extract_dir}"],
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
        )
    if result.returncode != 0 or not initrd.is_file():
        raise SystemExit(f"failed to extract initramfs from {iso}")
    return initrd


def qemu_args(iso: Path, root: Path, kernel: Path | None = None) -> list[str]:
    args = [QEMU_BIN]

    if platform.machine() == "arm64":
        bios_paths = [
            Path("/opt/homebrew/share/qemu/edk2-aarch64-code.fd"),
            Path("/usr/share/qemu/edk2-aarch64-code.fd"),
        ]
        bios = next((p for p in bios_paths if p.is_file()), None)
        if bios is None:
            raise SystemExit("cannot find aarch64 UEFI firmware (edk2-aarch64-code.fd)")
        args.extend(
            [
                "-machine",
                "virt,highmem=off,accel=hvf:tcg",
                "-cpu",
                "max",
            ]
        )
        if not kernel:
            args.extend(["-bios", str(bios)])
    else:
        args.extend(["-machine", "accel=hvf:kvm:tcg"])

    args.extend(
        [
            "-smp",
            VM_CPUS,
            "-m",
            VM_MEMORY,
            "-cdrom",
            str(iso),
            "-boot",
            "d",
            "-nographic",
            "-serial",
            "mon:stdio",
            "-netdev",
            f"user,id=net0,hostfwd=tcp::{SSH_PORT}-:22",
            "-device",
            "virtio-net-pci,netdev=net0",
            "-virtfs",
            f"local,path={root},mount_tag=workspace,security_model=none,id=ws0,readonly=off",
        ]
    )

    if kernel:
        initrd = _extract_alpine_initramfs(iso)
        args.extend(
            [
                "-kernel",
                str(kernel),
                "-initrd",
                str(initrd),
                "-append",
                "console=ttyAMA0 alpine_dev=cdrom modules=loop,squashfs quiet",
            ]
        )

    return args


def expect(proc: subprocess.Popen[str], pattern: str, timeout: float) -> bool:
    """Wait for pattern in process stdout."""
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


def send(proc: subprocess.Popen[str], cmd: str) -> None:
    proc.stdin.write(cmd + "\n")
    proc.stdin.flush()


SSH_OPTS = [
    "-o",
    "StrictHostKeyChecking=no",
    "-o",
    "UserKnownHostsFile=/dev/null",
    "-o",
    "PreferredAuthentications=none",
    "-o",
    "LogLevel=ERROR",
    "-p",
    SSH_PORT,
]


def wait_for_ssh(timeout: float) -> None:
    deadline = time.time() + timeout
    while time.time() < deadline:
        result = run(
            ["ssh", *SSH_OPTS, "-o", "ConnectTimeout=2", "root@localhost", "true"],
            stdin=subprocess.DEVNULL,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
        )
        if result.returncode == 0:
            return
        time.sleep(2)
    raise RuntimeError(f"SSH not reachable after {timeout}s")


def ssh_run(cmd: str, timeout: int) -> int:
    ssh_cmd = [
        "ssh",
        *SSH_OPTS,
        "-o",
        "ServerAliveInterval=30",
        "root@localhost",
        cmd,
    ]
    result = subprocess.run(ssh_cmd, text=True, check=False, timeout=timeout)
    return result.returncode


def shell_quote(s: str) -> str:
    return "'" + s.replace("'", "'\\''") + "'"


def run_in_vm(
    iso: Path,
    root: Path,
    commands: str,
    packages: list[str],
    timeout: int,
    kernel: Path | None = None,
) -> int:
    """Boot ISO, configure live system, run commands via SSH."""
    args = qemu_args(iso, root, kernel=kernel)

    print("Booting Alpine VM...", file=sys.stderr)
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
        if not expect(proc, "login:", BOOT_TIMEOUT):
            raise RuntimeError("timeout waiting for VM login prompt")

        time.sleep(1)
        send(proc, "root")
        time.sleep(3)

        bootstrap = (
            "setup-interfaces -a 2>/dev/null; ifup eth0 2>/dev/null; ifup lo 2>/dev/null; "
            "echo nameserver 8.8.8.8 > /etc/resolv.conf; "
            "apk add --no-cache openssh; "
            "echo PermitRootLogin yes >> /etc/ssh/sshd_config; "
            "echo PermitEmptyPasswords yes >> /etc/ssh/sshd_config; "
            "passwd -d root; "
            "ssh-keygen -t ed25519 -f /etc/ssh/ssh_host_ed25519_key -N '' 2>/dev/null; "
            "ssh-keygen -t rsa -f /etc/ssh/ssh_host_rsa_key -N '' 2>/dev/null; "
            "/usr/sbin/sshd; "
            "echo SSHD_READY"
        )
        send(proc, bootstrap)

        print("  bootstrapping VM (network + sshd)...", file=sys.stderr)
        if not expect(proc, "SSHD_READY", 90):
            raise RuntimeError("timeout waiting for VM bootstrap (sshd)")

        print("  waiting for SSH...", file=sys.stderr)
        wait_for_ssh(timeout=30)
        print("  VM ready.", file=sys.stderr)

        go_arch = "arm64" if ALPINE_ARCH == "aarch64" else "amd64"
        go_url = f"https://go.dev/dl/go{GO_VERSION}.linux-{go_arch}.tar.gz"

        setup_parts = [
            "set -e",
            f"printf 'https://dl-cdn.alpinelinux.org/alpine/v{ALPINE_VERSION}/main\\nhttps://dl-cdn.alpinelinux.org/alpine/v{ALPINE_VERSION}/community\\n' > /etc/apk/repositories",
            "apk update",
            "apk add --no-cache git python3 bash curl musl-dev",
        ]
        if packages:
            setup_parts.append(f"apk add --no-cache {' '.join(packages)}")
        setup_parts.extend(
            [
                "modprobe ppp_generic 2>/dev/null || true",
                "modprobe l2tp_ppp 2>/dev/null || true",
                "modprobe l2tp_netlink 2>/dev/null || true",
                "modprobe nft_chain_nat 2>/dev/null || true",
                "mkdir -p /workspace",
                "mount -t 9p -o trans=virtio,version=9p2000.L,msize=1048576 workspace /workspace",
                "cd /workspace",
                "mkdir -p /workspace/tmp/qemu/go-dl",
                f'GO_TAR="/workspace/tmp/qemu/go-dl/go{GO_VERSION}.linux-{go_arch}.tar.gz"',
                f'[ -f "$GO_TAR" ] || curl -fsSL -o "$GO_TAR" "{go_url}"',
                'tar -C /usr/local -xzf "$GO_TAR"',
                'export PATH="/usr/local/go/bin:$PATH"',
                'export GOROOT="/usr/local/go"',
                'export GOCACHE="/workspace/tmp/qemu/go-cache"',
                'export GOMODCACHE="/workspace/tmp/qemu/gomodcache"',
                'export GOFLAGS="-buildvcs=false"',
                'export HOME="/root"',
                'export TMPDIR="/tmp"',
                "mkdir -p /workspace/tmp/evidence",
                "mount -t tmpfs tmpfs /workspace/tmp/evidence",
            ]
        )

        setup = " && ".join(setup_parts)
        full_cmd = f"sh -c {shell_quote(setup + ' && ' + commands)}"

        print(f"  running: {commands}", file=sys.stderr)
        rc = ssh_run(full_cmd, timeout=timeout)
        return rc

    except RuntimeError as e:
        sys.stderr.write(f"error: {e}\n")
        return 1
    finally:
        proc.kill()
        proc.wait()


def main() -> int:
    import argparse

    parser = argparse.ArgumentParser(
        description="Run commands in a QEMU Linux VM with full kernel capabilities.",
    )
    parser.add_argument("--run", required=True, help="Command(s) to run inside the VM")
    parser.add_argument(
        "--packages", default="", help="Space-separated Alpine packages to install"
    )
    parser.add_argument(
        "--timeout",
        type=int,
        default=DEFAULT_CMD_TIMEOUT,
        help=f"Command timeout (default {DEFAULT_CMD_TIMEOUT}s)",
    )
    parser.add_argument(
        "--kernel",
        default="",
        help="Path to custom kernel (e.g. tmp/kernel/vmlinuz for gokrazy kernel with PPPoL2TP)",
    )
    args = parser.parse_args()

    if shutil.which(QEMU_BIN) is None:
        raise SystemExit(f"missing: {QEMU_BIN} (brew install qemu)")

    root = repo_root()
    cdir = cache_dir(root)
    iso = ensure_iso(cdir)

    kernel = None
    if args.kernel:
        kernel = Path(args.kernel)
        if not kernel.is_absolute():
            kernel = root / kernel
        if not kernel.is_file():
            raise SystemExit(f"kernel not found: {kernel}")

    packages = args.packages.split() if args.packages.strip() else []
    rc = run_in_vm(iso, root, args.run, packages, args.timeout, kernel=kernel)

    if rc == 0:
        print("\nQEMU VM: PASS", file=sys.stderr)
    else:
        print(f"\nQEMU VM: FAIL (exit code {rc})", file=sys.stderr)

    return rc


if __name__ == "__main__":
    raise SystemExit(main())
