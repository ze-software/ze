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

Before writing a probe that asserts on a dataplane counter, read
`ai/rules/platform-linux.md`, section "4. Dataplane counters need a real remote
peer". A VM addressing its own address moves no xfrm counter, so such a probe
reads zero whether the dataplane works or not.
`test/ipsec-interop/scenarios/01-psk-site-to-site/check.py` is the in-tree
pattern, over `assert_esp_accepted` in `test/ipsec-interop/lab.py`.
"""

from __future__ import annotations

import os
import platform
import select
import shutil
import signal
import socket
import subprocess
import sys
import tempfile
import time
from pathlib import Path

from alpine_iso import ALPINE_MINOR, ALPINE_VERSION, ensure_iso
from homebrew import brew_files

ALPINE_ARCH = "aarch64" if platform.machine() == "arm64" else "x86_64"
QEMU_BIN = f"qemu-system-{ALPINE_ARCH}"
GO_VERSION = "1.25.9"
VM_MEMORY = os.environ.get("ZE_QEMU_MEMORY", "16384")
VM_CPUS = os.environ.get("ZE_QEMU_CPUS", "8")
BOOT_TIMEOUT = int(os.environ.get("ZE_QEMU_BOOT_TIMEOUT", "60"))
DEFAULT_CMD_TIMEOUT = 1200


def _free_ssh_port() -> str:
    """Host port forwarded to the VM's sshd.

    2222 was a constant, and a constant makes two concurrent QEMU evidence runs
    mutually exclusive: qemu refuses to start at all with "Could not set up host
    forwarding rule", which this script then reported as "timeout waiting for VM
    login prompt" because the process is already gone. Several agents share this
    checkout and run these targets at once, so the first free port from 2222 up
    is taken instead. ZE_QEMU_SSH_PORT pins it when a caller needs a known port.

    This narrows the window; it does not close it. The probe binds and closes
    before qemu binds, so two runs starting in the same instant still choose the
    same port and the loser still fails to boot. What makes that survivable is
    the sibling change in run_in_vm: qemu's stderr is kept and printed, so the
    loser now reads "Could not set up host forwarding rule" and re-runs, instead
    of reading a boot timeout and hunting the VM. Closing the window properly
    means retrying the boot on that specific error, which is a change to the
    launch path every QEMU target shares.
    """
    pinned = os.environ.get("ZE_QEMU_SSH_PORT")
    if pinned:
        return pinned
    for candidate in range(2222, 2322):
        # Bind exactly as qemu will: the wildcard address, and no SO_REUSEADDR. A
        # probe on 127.0.0.1 with SO_REUSEADDR set succeeds against a live
        # wildcard listener on macOS, so it reports every busy port as free and
        # the collision it exists to avoid happens anyway.
        with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as probe:
            try:
                probe.bind(("", candidate))
            except OSError:
                continue
        return str(candidate)
    raise SystemExit("no free host port in 2222..2321 to forward to the VM's sshd")


SSH_PORT = _free_ssh_port()


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


SCRATCH_MOUNT_TAG = "zescratch"


def scratch_share(root: Path) -> tuple[Path, str] | None:
    """The second 9p share a symlinked tmp/ needs, or None when tmp/ is real.

    `-virtfs local,path=<root>` exports the checkout, and 9p with
    security_model=none gives the guest a symlink as a symlink. So once
    scripts/dev/ensure-links.py has pointed tmp/ at an out-of-tree scratch
    directory, /workspace/tmp dangles in the guest: nothing under it resolves,
    the DUT binary at tmp/session/<YYYY-MM-DD>-<id>/bin/ze (mk/session.mk
    ZE_BIN_DIR) included, and the run fails before it can exec anything.

    Returns (host path to export, guest mount point). The two differ on
    purpose. The exported path is fully resolved, because that is what QEMU
    opens on the host. The mount point is the LINK'S OWN TEXT, because that is
    what the guest kernel resolves /workspace/tmp to, and mounting anywhere
    else leaves the link dangling.
    """
    tmp = root / "tmp"
    if not tmp.is_symlink():
        return None
    link = os.readlink(tmp)
    # ensure-links.py always writes an absolute target ($TMPDIR/ze/<checkout-id>,
    # else /tmp/ze/<checkout-id>). A relative one is resolved the way the guest
    # resolves it: against the directory holding the link, which is /workspace.
    guest = link if os.path.isabs(link) else os.path.normpath("/workspace/" + link)
    return Path(os.path.realpath(tmp)), guest


def virtfs_args(root: Path) -> list[str]:
    """Every 9p export this run needs: the checkout, plus tmp/ when it is a link.

    Pure, and separate from qemu_args so the selftest can read it on a host with
    no QEMU and no UEFI firmware installed.
    """
    args = [
        "-virtfs",
        f"local,path={root},mount_tag=workspace,security_model=none,id=ws0,readonly=off",
    ]
    share = scratch_share(root)
    if share is not None:
        host, _guest = share
        args.extend(
            [
                "-virtfs",
                f"local,path={host},mount_tag={SCRATCH_MOUNT_TAG},"
                "security_model=none,id=ws1,readonly=off",
            ]
        )
    return args


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
    """Extract initramfs-virt from Alpine ISO (needed for custom kernel boot).

    Extraction lands in a sibling staging dir and is renamed into place, so a
    concurrent boot on the shared per-ISO extract dir sees a complete extract or
    nothing, never a half-written tree.
    """
    extract_dir = _extract_dir_for(iso)
    initrd = extract_dir / "boot" / "initramfs-virt"
    if initrd.is_file():
        return initrd
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
    extract_dir.parent.mkdir(parents=True, exist_ok=True)
    for extractor in extractors:
        staging = Path(
            tempfile.mkdtemp(prefix=f".{extract_dir.name}.", dir=extract_dir.parent)
        )
        committed = False
        try:
            result = run(
                [extractor, "x", str(iso), "-y", f"-o{staging}"],
                stdout=subprocess.PIPE,
                stderr=subprocess.PIPE,
            )
            if (
                result.returncode == 0
                and (staging / "boot" / "initramfs-virt").is_file()
            ):
                shutil.rmtree(extract_dir, ignore_errors=True)
                os.replace(staging, extract_dir)
                committed = True
                return initrd
        finally:
            if not committed:
                shutil.rmtree(staging, ignore_errors=True)
    raise SystemExit(f"failed to extract initramfs from {iso}")


def qemu_args(iso: Path, root: Path, kernel: Path | None = None) -> list[str]:
    args = [QEMU_BIN]

    if platform.machine() == "arm64":
        bios_paths = [
            *brew_files("share/qemu/edk2-aarch64-code.fd"),
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
            *virtfs_args(root),
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

    print(
        f"Booting Alpine VM (ssh forwarded on host port {SSH_PORT})...", file=sys.stderr
    )
    # qemu's own diagnostics went to DEVNULL, so every startup refusal -- a
    # host-forward port already taken, missing firmware, an accelerator it cannot
    # open -- surfaced as the boot timeout below, which names none of them. Keep
    # the stream and print it when the boot does not complete.
    qemu_errors = tempfile.TemporaryFile(mode="w+")
    proc = subprocess.Popen(
        args,
        stdin=subprocess.PIPE,
        stdout=subprocess.PIPE,
        stderr=qemu_errors,
        text=True,
    )

    def qemu_stderr() -> str:
        qemu_errors.seek(0)
        return qemu_errors.read().strip()

    def cleanup(signum=None, _frame=None):
        proc.kill()
        proc.wait()
        if signum:
            raise SystemExit(128 + signum)

    signal.signal(signal.SIGTERM, cleanup)
    signal.signal(signal.SIGINT, cleanup)

    try:
        if not expect(proc, "login:", BOOT_TIMEOUT):
            detail = qemu_stderr()
            if detail:
                raise RuntimeError(
                    f"VM never reached a login prompt; qemu said:\n{detail}"
                )
            raise RuntimeError(
                f"timeout waiting for VM login prompt after {BOOT_TIMEOUT}s"
            )

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
            ]
        )
        # A symlinked tmp/ leaves /workspace/tmp dangling in the guest, so the
        # scratch share is mounted at the path the link names -- before anything
        # below touches /workspace/tmp (scratch_share). Nothing to do when tmp/
        # is a real directory: the workspace share already carries it.
        share = scratch_share(root)
        if share is not None:
            _host, guest = share
            setup_parts.extend(
                [
                    f"mkdir -p {shell_quote(guest)}",
                    "mount -t 9p -o trans=virtio,version=9p2000.L,msize=1048576 "
                    f"{SCRATCH_MOUNT_TAG} {shell_quote(guest)}",
                ]
            )
        setup_parts.extend(
            [
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
                # The 9p mount carries the HOST user's uid, so every repository
                # under /workspace looks foreign to root and git refuses it with
                # "detected dubious ownership". That is a sane default for a
                # multi-user box and pure noise in a single-purpose throwaway VM:
                # it broke every commit_helper and spec-closure test, which create
                # their own scratch repositories under /workspace/tmp.
                "git config --global --add safe.directory '*' 2>/dev/null || true",
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
            # Print the paths the caller actually cross-compiled. Under an AI
            # session those sit in the session's own directory ($(ZE_BIN_DIR),
            # mk/session.mk), so a literal bin/ze-linux-<arch> here would be a
            # copy-paste hint pointing at a file that does not exist.
            hint_ze = os.environ.get("ZE_QEMU_BIN", "bin/ze-linux-arm64")
            hint_test = os.environ.get("ZE_QEMU_TEST_BIN", "bin/ze-test-linux-arm64")
            print(
                f"  {ssh_cmd} 'cd /workspace && ZE_TEST_NO_BUILD=1 ZE_BIN={hint_ze} {hint_test} bgp parse 264 -v'",
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


def _selftest_scratch_share() -> list[str]:
    """Fixture tests for the second 9p share (scratch_share, virtfs_args).

    A symlinked tmp/ is the migrated layout (`make ze-migrate-scratch`), and on
    a checkout that has migrated, every session path -- the DUT binary at
    tmp/session/<YYYY-MM-DD>-<id>/bin/ze most of all -- lives behind that link.
    A share of the repo root alone leaves it dangling in the guest, and the
    failure is silent until somebody migrates, so it is pinned here.
    """
    failures: list[str] = []
    with tempfile.TemporaryDirectory() as td:
        d = Path(td)

        real_root = d / "real"
        (real_root / "tmp").mkdir(parents=True)
        if scratch_share(real_root) is not None:
            failures.append("a real tmp/ asked for a second share it does not need")
        if virtfs_args(real_root).count("-virtfs") != 1:
            failures.append("a real tmp/ produced more than one -virtfs export")

        link_root = d / "linked"
        link_root.mkdir()
        target = d / "out-of-tree"
        (target / "session" / "2026-08-10-abc123" / "bin").mkdir(parents=True)
        (link_root / "tmp").symlink_to(target)

        share = scratch_share(link_root)
        if share is None:
            failures.append(
                "a symlinked tmp/ got no second share: the guest cannot "
                "resolve /workspace/tmp/session/<dated>/bin/ze"
            )
        else:
            host, guest = share
            if host != Path(os.path.realpath(target)):
                failures.append(f"second share exports {host}, not the tmp/ target")
            # The mount point must be the link's own text: that is the path the
            # guest resolves /workspace/tmp to, so mounting anywhere else leaves
            # the link dangling however correct the exported directory is.
            if guest != str(target):
                failures.append(
                    f"second share mounts at {guest}, not at the target the link "
                    f"names ({target}); /workspace/tmp would still dangle"
                )
            args = virtfs_args(link_root)
            exports = [a for a in args if a.startswith("local,path=")]
            if len(exports) != 2:
                failures.append("a symlinked tmp/ did not add a second -virtfs export")
            tags = {e.split("mount_tag=")[1].split(",")[0] for e in exports}
            ids = {e.split("id=")[1].split(",")[0] for e in exports}
            if len(tags) != len(exports) or len(ids) != len(exports):
                failures.append(
                    f"the two 9p exports share a mount_tag or an id: {exports}"
                )

        # A relative link is not what ensure-links.py writes, but the guest
        # resolves one against the directory holding it, which is /workspace.
        rel_root = d / "relative"
        rel_root.mkdir()
        (d / "sibling").mkdir()
        (rel_root / "tmp").symlink_to("../sibling")
        rel = scratch_share(rel_root)
        if rel is None or rel[1] != "/sibling":
            got = None if rel is None else rel[1]
            failures.append(
                f"a relative tmp/ link resolves to {got}, not to /sibling, the way "
                "the guest resolves it against /workspace"
            )

    return failures


def _selftest() -> int:
    """Fixture tests for the ISO extract cache. Run by qemu_run_test.go so
    `make ze-unit-test` covers this without QEMU, a download, or 7z.

    Guards the invariant that a cached extract belongs to exactly one ISO: the
    ISO filename is version-keyed (see ensure_iso), so its extract must be too,
    or bumping ALPINE_VERSION boots a new ISO with the previous initramfs.
    """
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

    failures.extend(_selftest_scratch_share())

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
    iso = ensure_iso(
        ALPINE_ARCH
    )  # verified ISO from durable cache (~/.cache/ze/alpine-iso)

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
