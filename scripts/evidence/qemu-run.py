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

from alpine_iso import ALPINE_MINOR, ALPINE_VERSION, ensure_iso

ALPINE_ARCH = "aarch64" if platform.machine() == "arm64" else "x86_64"
QEMU_BIN = f"qemu-system-{ALPINE_ARCH}"
GO_VERSION = "1.25.9"
VM_MEMORY = os.environ.get("ZE_QEMU_MEMORY", "16384")
VM_CPUS = os.environ.get("ZE_QEMU_CPUS", "8")
BOOT_TIMEOUT = int(os.environ.get("ZE_QEMU_BOOT_TIMEOUT", "60"))
DEFAULT_CMD_TIMEOUT = 1200
SSH_PORT = "2222"


def repo_root() -> Path:
    here = Path(__file__).resolve()
    for parent in here.parents:
        if (parent / "go.mod").is_file():
            return parent
    raise SystemExit("cannot locate repository root")


def cache_dir(root: Path) -> Path:
    # tmp/ is a symlink to an out-of-tree scratch dir (scripts/dev/ensure-links.py); a
    # symlinked tmp/ is skipped by `go list ./...`, so no tmp/go.mod sentinel is needed.
    d = root / "tmp" / "qemu"
    d.mkdir(parents=True, exist_ok=True)
    (d / "go-dl").mkdir(exist_ok=True)
    (d / "go-cache").mkdir(exist_ok=True)
    (d / "gomodcache").mkdir(exist_ok=True)
    return d


def run(cmd: list[str], **kwargs) -> subprocess.CompletedProcess[str]:
    return subprocess.run(cmd, text=True, check=False, **kwargs)


def _extract_dir_for(iso: Path) -> Path:
    """Directory holding the contents extracted from `iso`.

    Keyed by the ISO's filename, which already carries version and arch (see
    ensure_iso). A shared, unkeyed directory made the extract outlive the ISO it
    came from: bumping ALPINE_VERSION downloaded a new ISO, then booted it with
    the previous version's initramfs, because the early return in
    _extract_alpine_initramfs only tests that initramfs-virt exists.
    """
    return iso.parent / f"{iso.stem}-extract"


def _extract_alpine_initramfs(iso: Path) -> Path:
    """Extract initramfs-virt from Alpine ISO (needed for custom kernel boot)."""
    extract_dir = _extract_dir_for(iso)
    initrd = extract_dir / "boot" / "initramfs-virt"
    if initrd.is_file():
        return initrd
    extract_dir.mkdir(parents=True, exist_ok=True)
    # p7zip installs `7z`; the official 7-Zip build installs `7zz`. Select by
    # what is actually on PATH rather than exec-ing a name that may not exist:
    # subprocess raises FileNotFoundError (not our SystemExit) for a missing
    # binary, which would otherwise crash callers -- and the --selftest, which
    # is documented to run without 7z -- instead of failing loudly here.
    extractors = [e for e in ("7z", "7zz") if shutil.which(e)]
    if not extractors:
        raise SystemExit(
            "cannot extract initramfs: install 7z (p7zip) to boot a custom --kernel"
        )
    for extractor in extractors:
        result = run(
            [extractor, "x", str(iso), "-y", f"-o{extract_dir}"],
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
        )
        if result.returncode == 0 and initrd.is_file():
            return initrd
    raise SystemExit(f"failed to extract initramfs from {iso}")


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
                "virt,highmem=on,accel=hvf:tcg",
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
    keep_alive: bool = False,
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
        # Bootstrap can flake: the `;`-chained commands echo SSHD_READY even if
        # ifup/apk/sshd raced or failed, and HVF boot timing varies. Re-run the
        # bootstrap (re-attempting network + sshd) until SSH actually answers.
        ready = False
        for attempt in range(1, 4):
            send(proc, bootstrap)
            print(
                f"  bootstrapping VM (network + sshd), attempt {attempt}...",
                file=sys.stderr,
            )
            if not expect(proc, "SSHD_READY", 90):
                continue
            print("  waiting for SSH...", file=sys.stderr)
            try:
                wait_for_ssh(timeout=30)
                ready = True
                break
            except RuntimeError:
                print("  SSH not up yet; re-running bootstrap...", file=sys.stderr)
                time.sleep(2)
        if not ready:
            raise RuntimeError("VM bootstrap failed: SSH not reachable after retries")
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
                # Register nf_conntrack tracking so flow-export's conntrack export
                # (and thus DDoS characterization) sees per-flow records. Three
                # prerequisites, none of which a plain module load provides:
                #   1. nf_conntrack + nf_conntrack_netlink: the reader dumps the
                #      table over ctnetlink (conntrack/reader_linux.go), which needs
                #      the netlink module, not just the tracker.
                #   2. a rule referencing `ct state`: loading the module does NOT
                #      register the tracking hooks; nftables registers them netns-
                #      wide the moment any rule uses a ct expression. An accept-only
                #      table affects no test's forwarding or drop behavior.
                #   3. nf_conntrack_acct=1: without accounting the kernel reports
                #      0 bytes/packets per flow, and the export worker drops every
                #      zero-delta flow (conntrack_worker.go) so the ring stays empty.
                "modprobe nf_conntrack 2>/dev/null || true",
                "modprobe nf_conntrack_netlink 2>/dev/null || true",
                "[ -w /proc/sys/net/netfilter/nf_conntrack_acct ] && echo 1 > /proc/sys/net/netfilter/nf_conntrack_acct || true",
                "nft add table inet ztrack 2>/dev/null || true",
                "nft 'add chain inet ztrack out { type filter hook output priority -150 ; policy accept ; }' 2>/dev/null || true",
                "nft add rule inet ztrack out ct state new,established,related counter 2>/dev/null || true",
                "echo \"CONNTRACK-SETUP: acct=$(cat /proc/sys/net/netfilter/nf_conntrack_acct 2>/dev/null || echo MISSING) rules=$(nft list ruleset 2>/dev/null | grep -c 'ct state' || echo 0)\"",
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

        if keep_alive:
            # Boot + run setup once, persist the Go/workspace env for future
            # login shells, then idle so the caller can SSH in repeatedly. Used
            # by `make ze-qemu-shell` for interactive failure investigation: run
            # one .ci test at a time and inspect dmesg / nft / netlink state
            # between runs, without rebooting the VM each iteration.
            profile_lines = [
                "export PATH=/usr/local/go/bin:$PATH",
                "export GOROOT=/usr/local/go",
                "export GOCACHE=/workspace/tmp/qemu/go-cache",
                "export GOMODCACHE=/workspace/tmp/qemu/gomodcache",
                "export GOFLAGS=-buildvcs=false",
                "export HOME=/root",
                "export TMPDIR=/tmp",
                "cd /workspace",
            ]
            write_profile = (
                "printf '%s\\n' "
                + " ".join(shell_quote(s) for s in profile_lines)
                + " > /etc/profile.d/ze.sh"
            )
            rc = ssh_run(
                "sh -c " + shell_quote(setup + " && " + write_profile),
                timeout=timeout,
            )
            if rc != 0:
                sys.stderr.write("keep-alive: VM setup failed\n")
                return rc
            ssh_cmd = "ssh " + " ".join(SSH_OPTS) + " root@localhost"
            print("\nZE_QEMU_READY", flush=True)
            print("VM is up; it stays up until this process is stopped.", flush=True)
            print(
                "Run a .ci test in the VM (no Go needed, reuses cross-compiled ze):",
                flush=True,
            )
            print(
                f"  {ssh_cmd} 'cd /workspace && ZE_TEST_NO_BUILD=1 ZE_BIN=bin/ze-linux-arm64 bin/ze-test-linux-arm64 bgp parse 264 -v'",
                flush=True,
            )
            print(
                "Run with the Go toolchain (login shell sources the env):", flush=True
            )
            print(
                f"  {ssh_cmd} 'bash -lc \"go test -tags integration ./cmd/ze/doctor/...\"'",
                flush=True,
            )
            print("Inspect kernel state between runs, e.g.:", flush=True)
            print(f"  {ssh_cmd} 'dmesg | tail -50'", flush=True)
            print(
                "Stop this process (Ctrl-C / TaskStop) to power off the VM.\n",
                flush=True,
            )
            try:
                while proc.poll() is None:
                    time.sleep(5)
            except KeyboardInterrupt:
                pass
            return 0

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


def _selftest() -> int:
    """Fixture tests for the ISO extract cache. Run by qemu_run_test.go so
    `make ze-unit-test` covers this without QEMU, a download, or 7z.

    Guards the invariant that a cached extract belongs to exactly one ISO: the
    ISO filename is version-keyed (see ensure_iso), so its extract must be too,
    or bumping ALPINE_VERSION boots a new ISO with the previous initramfs.
    """
    import tempfile

    failures: list[str] = []
    with tempfile.TemporaryDirectory() as td:
        d = Path(td)
        iso_a = d / f"alpine-virt-3.21.3-{ALPINE_ARCH}.iso"
        iso_b = d / f"alpine-virt-3.22.0-{ALPINE_ARCH}.iso"
        iso_a.write_bytes(b"not-a-real-iso")
        iso_b.write_bytes(b"not-a-real-iso-either")

        # Simulate a completed extract of iso_a.
        initrd_a = _extract_dir_for(iso_a) / "boot" / "initramfs-virt"
        initrd_a.parent.mkdir(parents=True, exist_ok=True)
        initrd_a.write_bytes(b"initramfs-from-3.21.3")

        # Same ISO: the extract is reused, no re-extraction.
        if _extract_alpine_initramfs(iso_a) != initrd_a:
            failures.append("cached extract was not reused for the same ISO")

        # Different ISO: iso_a's extract must NOT be reused. iso_b is a stub that
        # cannot really be extracted, so a correct implementation fails loudly
        # here; a buggy one silently returns iso_a's initramfs.
        if _extract_dir_for(iso_b) == _extract_dir_for(iso_a):
            failures.append(
                "extract dir is not keyed to the ISO: "
                f"{iso_a.name} and {iso_b.name} share {_extract_dir_for(iso_a).name}"
            )
        got: Path | None
        try:
            got = _extract_alpine_initramfs(iso_b)
        except SystemExit:
            got = None  # correct: tried to extract the stub iso_b and failed
        if got is not None and got == initrd_a:
            failures.append(
                "stale hit: a version bump reused the previous ISO's initramfs"
            )

    for f in failures:
        print(f"selftest FAILED: {f}", file=sys.stderr)
    if failures:
        return 1
    print("qemu-run selftest OK")
    return 0


def main() -> int:
    import argparse

    parser = argparse.ArgumentParser(
        description="Run commands in a QEMU Linux VM with full kernel capabilities.",
    )
    parser.add_argument(
        "--selftest",
        action="store_true",
        help="Run the ISO-cache fixture tests and exit (no QEMU or download needed)",
    )
    parser.add_argument(
        "--run",
        default="",
        help="Command(s) to run inside the VM (omit with --keep-alive)",
    )
    parser.add_argument(
        "--keep-alive",
        action="store_true",
        help="Boot the VM, run setup, then idle for interactive SSH debugging "
        "instead of running a single --run command and exiting",
    )
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

    # Before the QEMU probe: the selftest is pure fixture work, so it must run
    # on a host without qemu installed.
    if args.selftest:
        return _selftest()

    if shutil.which(QEMU_BIN) is None:
        raise SystemExit(f"missing: {QEMU_BIN} (brew install qemu)")

    root = repo_root()
    cache_dir(root)  # ensure the tmp/qemu scratch dirs (go caches, gomodcache) exist
    iso = ensure_iso(ALPINE_ARCH)  # verified ISO from durable cache (~/.cache/ze/alpine-iso)

    kernel = None
    if args.kernel:
        kernel = Path(args.kernel)
        if not kernel.is_absolute():
            kernel = root / kernel
        if not kernel.is_file():
            raise SystemExit(f"kernel not found: {kernel}")

    packages = args.packages.split() if args.packages.strip() else []
    if not args.run and not args.keep_alive:
        raise SystemExit("error: provide --run or --keep-alive")
    rc = run_in_vm(
        iso,
        root,
        args.run,
        packages,
        args.timeout,
        kernel=kernel,
        keep_alive=args.keep_alive,
    )

    if rc == 0:
        print("\nQEMU VM: PASS", file=sys.stderr)
    else:
        print(f"\nQEMU VM: FAIL (exit code {rc})", file=sys.stderr)

    return rc


if __name__ == "__main__":
    raise SystemExit(main())
