#!/usr/bin/env python3
"""End-to-end QEMU evidence for the ze PXE/installer chain.

This is the real coverage behind test/install/qemu-full.ci. It builds the
installer initrd from scratch, builds a credential-matched gokrazy image with
`ze install appliance`, serves the image + checksum + zefs database over HTTP,
then boots the installer kernel + initrd in QEMU against a blank disk. It
asserts the initrd's serial success markers (image written, install complete),
and finally re-boots the written disk and logs in over SSH as the power user to
prove the loadZefsUsers credential path (install spec AC-10).

The installer kernel is operator-supplied by design (docs/guide/ze-install.md):
ze neither ships nor builds it. A *suitable* kernel for this test must have, as
BUILT-IN (=y, not modules, since the busybox initrd carries no modules):

  - CONFIG_IP_PNP / CONFIG_IP_PNP_DHCP   (init relies on kernel `ip=dhcp`)
  - CONFIG_VIRTIO_NET, CONFIG_VIRTIO_BLK (virtio NIC + target disk)
  - CONFIG_EXT4_FS, CONFIG_BLK_DEV_INITRD, CONFIG_DEVTMPFS

Point the test at one with ZE_INSTALL_KERNEL=/path/to/vmlinuz. When no usable
kernel (or QEMU, or a static busybox, or the image-build tooling) is present the
script prints a single `INSTALL-QEMU: SKIP <reason>` line and exits 0, so it is
safe to wire into the functional suite and into CI that lacks the artifacts. A
genuine end-to-end pass prints `INSTALL-QEMU: PASS`; a genuine failure exits
non-zero with the captured serial log.

Environment overrides:
  ZE_INSTALL_KERNEL   path to a suitable installer kernel (vmlinuz/bzImage)
  ZE_INSTALL_BUSYBOX  path to a static busybox for the initrd (else auto-sourced)
  ZE_INSTALL_ARCH     amd64 | arm64           (default: host arch)
  ZE_INSTALL_IMAGE    pre-built gokrazy image (skip the appliance build)
  ZE_INSTALL_KEEP     1 to keep the work dir for inspection
"""

from __future__ import annotations

import http.server
import json
import os
import platform
import shutil
import socket
import socketserver
import subprocess
import sys
import tempfile
import threading
import time
from pathlib import Path


# Serial markers emitted by tools/installer-initrd/init on success.
MARK_WRITTEN = "[ze-install] Disk image written successfully"
MARK_DONE = "[ze-install] Installation complete. Rebooting"

IMAGE_NAME = "ze-test.img"
SSH_USER = os.environ.get("ZE_INSTALL_SSH_USER", "admin")
SSH_PASS = os.environ.get("ZE_INSTALL_SSH_PASS", "secret")
# None => pick a free port at boot time. A fixed 2222 collides with any other
# QEMU/SSH forward already running (e.g. the ze-qemu integration VM), which
# fails the hostfwd rule and looks like an AC-10 failure.
_SSH_PORT_ENV = os.environ.get("ZE_INSTALL_SSH_PORT")


def _free_tcp_port() -> int:
    with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as s:
        s.bind(("127.0.0.1", 0))
        return s.getsockname()[1]


def host_arch() -> str:
    m = platform.machine().lower()
    if m in ("arm64", "aarch64"):
        return "arm64"
    return "amd64"


ARCH = os.environ.get("ZE_INSTALL_ARCH") or host_arch()
QEMU_BIN = "qemu-system-aarch64" if ARCH == "arm64" else "qemu-system-x86_64"
QEMU_ACCEL = os.environ.get("ZE_INSTALL_QEMU_ACCEL") or (
    "hvf"
    if sys.platform == "darwin"
    else ("kvm" if Path("/dev/kvm").exists() else "tcg")
)


def repo_root() -> Path:
    here = Path(__file__).resolve()
    for parent in here.parents:
        if (parent / "go.mod").is_file():
            return parent
    raise SystemExit("cannot locate repository root")


def skip(reason: str) -> int:
    print(f"INSTALL-QEMU: SKIP {reason}")
    return 0


def run(cmd: list[str], **kwargs) -> subprocess.CompletedProcess[str]:
    return subprocess.run(cmd, text=True, check=False, **kwargs)


# ── prerequisites ─────────────────────────────────────────────────────────


def _ensure_docker_host() -> None:
    """On macOS, point docker at colima's socket when DOCKER_HOST is unset.

    The default docker context is usually down on a colima box, so without this
    `docker info` fails and the test self-skips. Covers every entry point (make
    target, the .ci runner, direct invocation) — a Makefile-only export would
    miss the .ci path. No-op on Linux or when DOCKER_HOST is already set.
    """
    if platform.system() != "Darwin" or os.environ.get("DOCKER_HOST"):
        return
    sock = Path.home() / ".colima" / "default" / "docker.sock"
    if sock.is_socket():
        os.environ["DOCKER_HOST"] = f"unix://{sock}"


def find_static_busybox(work: Path) -> Path | None:
    """Locate or extract a statically-linked busybox for the initrd."""
    override = os.environ.get("ZE_INSTALL_BUSYBOX")
    if override and Path(override).is_file():
        return Path(override)

    host = shutil.which("busybox")
    if (
        host
        and "statically linked" in run(["file", host], stdout=subprocess.PIPE).stdout
    ):
        return Path(host)

    # Try a container runtime: the dockerhub busybox image is static musl.
    _ensure_docker_host()
    docker = shutil.which("docker")
    if (
        docker
        and run(
            [docker, "info"], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL
        ).returncode
        == 0
    ):
        dest = work / "busybox-static"
        cid = run(
            [docker, "create", "busybox:musl"], stdout=subprocess.PIPE
        ).stdout.strip()
        if cid:
            run(
                [docker, "cp", f"{cid}:/bin/busybox", str(dest)],
                stdout=subprocess.DEVNULL,
            )
            run(
                [docker, "rm", cid],
                stdout=subprocess.DEVNULL,
                stderr=subprocess.DEVNULL,
            )
            if dest.is_file():
                return dest
    return None


def find_installer_kernel() -> Path | None:
    override = os.environ.get("ZE_INSTALL_KERNEL")
    if override and Path(override).is_file():
        return Path(override)
    return None  # operator-supplied by design; no safe default exists


def have_image_build_tools(root: Path) -> str | None:
    """Return a skip-reason if the gokrazy image build cannot run, else None."""
    if os.environ.get("ZE_INSTALL_IMAGE"):
        return None
    debugfs = shutil.which("debugfs") or _brew_debugfs()
    if debugfs is None:
        return "debugfs missing (install e2fsprogs) — needed to inject zefs into /perm"
    return None


def _brew_debugfs() -> str | None:
    cand = Path("/opt/homebrew/opt/e2fsprogs/sbin/debugfs")
    return str(cand) if cand.is_file() else None


# ── build steps ───────────────────────────────────────────────────────────


def build_initrd(root: Path, busybox: Path) -> Path:
    out = root / "tools" / "installer-initrd" / "build" / "initrd.img.gz"
    r = run(
        ["make", "-C", str(root / "tools" / "installer-initrd"), f"BUSYBOX={busybox}"],
        stdout=subprocess.PIPE,
        stderr=subprocess.STDOUT,
    )
    if r.returncode != 0 or not out.is_file():
        raise SystemExit(f"initrd build failed:\n{r.stdout}")
    return out


def build_image(root: Path, work: Path) -> Path:
    override = os.environ.get("ZE_INSTALL_IMAGE")
    if override:
        zefs = os.environ.get("ZE_INSTALL_ZEFS")
        return Path(override), (Path(zefs) if zefs else None)
    # Build a HOST ze for the appliance commands (they run on this machine:
    # gok in-process, debugfs/mkfs). A checked-in bin/ze is often a cross-build
    # (GOOS=linux) and would fail with "exec format error", so never rely on it.
    ze = str(work / "ze-host")
    b = run(
        ["go", "build", "-o", ze, "./cmd/ze"],
        cwd=str(root),
        stdout=subprocess.PIPE,
        stderr=subprocess.STDOUT,
    )
    if b.returncode != 0:
        raise SystemExit(f"host ze build failed:\n{b.stdout}")
    name = "ze-install-qemu"
    appliance_dir = work / "appliances"
    env = os.environ.copy()
    env["ZE_APPLIANCE_DIR"] = str(appliance_dir)
    # Non-interactive init: password via the CI env var, EOF stdin so the wizard
    # falls back to DefaultConfig (username defaults to "admin" == SSH_USER).
    env["ze.appliance.ssh.password"] = SSH_PASS
    # `ze install appliance build` shells out to debugfs/mkfs.ext4 by name; on
    # macOS those live in the (keg-only) e2fsprogs prefix, off PATH by default.
    e2fs_sbin = Path("/opt/homebrew/opt/e2fsprogs/sbin")
    if e2fs_sbin.is_dir():
        env["PATH"] = f"{e2fs_sbin}:{env.get('PATH', '')}"
    init = run(
        [ze, "install", "appliance", "init", name],
        stdin=subprocess.DEVNULL,
        stdout=subprocess.PIPE,
        stderr=subprocess.STDOUT,
        env=env,
    )
    if init.returncode != 0:
        raise SystemExit(f"ze install appliance init failed:\n{init.stdout}")
    # Match the appliance config to ZE_INSTALL_ARCH. `ze install appliance build`
    # reads image.arch from appliance.json, not GOKRAZY_ARCH.
    cfg_path = appliance_dir / name / "appliance.json"
    cfg_json = json.loads(cfg_path.read_text())
    cfg_json.setdefault("image", {})["arch"] = ARCH
    # Optional image-size override for the test (e.g. to keep the streamed image
    # well under the 2 GiB / 2^31 boundary while debugging the transfer path).
    size_override = os.environ.get("ZE_INSTALL_IMAGE_SIZE")
    if size_override:
        try:
            size_bytes = int(size_override)
        except ValueError:
            raise SystemExit(
                f"ZE_INSTALL_IMAGE_SIZE must be an integer byte count, got {size_override!r}"
            ) from None
        cfg_json.setdefault("image", {})["size-bytes"] = size_bytes
    cfg_path.write_text(json.dumps(cfg_json))
    build = run(
        [ze, "install", "appliance", "build", name],
        stdout=subprocess.PIPE,
        stderr=subprocess.STDOUT,
        env=env,
    )
    if build.returncode != 0:
        raise SystemExit(f"ze install appliance build failed:\n{build.stdout}")
    imgs = sorted(appliance_dir.rglob("ze-*.img"))
    if not imgs:
        raise SystemExit("appliance build produced no image")
    # build assembles its zefs to database.zefs and deletes it after injecting
    # it into /perm, so re-run assemble --keep AFTERWARD to retain a copy of the
    # same credential-bearing database for the initrd to download.
    assemble = run(
        [ze, "install", "appliance", "assemble", "--keep", name],
        stdout=subprocess.PIPE,
        stderr=subprocess.STDOUT,
        env=env,
    )
    if assemble.returncode != 0:
        raise SystemExit(f"ze install appliance assemble failed:\n{assemble.stdout}")
    zefs = next(iter(sorted(appliance_dir.rglob("*.zefs"))), None)
    return imgs[-1], zefs


def write_checksum(image: Path, served_dir: Path) -> None:
    """Serve image + .sha256 sidecar so the initrd verifies the checksum."""
    target = served_dir / IMAGE_NAME
    shutil.copy(image, target)
    import hashlib

    h = hashlib.sha256()
    with open(target, "rb") as f:
        for chunk in iter(lambda: f.read(1 << 20), b""):
            h.update(chunk)
    (served_dir / f"{IMAGE_NAME}.sha256").write_text(f"{h.hexdigest()}  {IMAGE_NAME}\n")


# ── HTTP install server ──────────────────────────────────────────────────


class _ThreadingHTTPServer(socketserver.ThreadingMixIn, socketserver.TCPServer):
    daemon_threads = True
    allow_reuse_address = True


def start_http(served_dir: Path) -> tuple[socketserver.TCPServer, int]:
    handler = _make_handler(served_dir)
    httpd = _ThreadingHTTPServer(("0.0.0.0", 0), handler)
    port = httpd.server_address[1]
    threading.Thread(target=httpd.serve_forever, daemon=True).start()
    return httpd, port


def _make_handler(served_dir: Path):
    class Handler(http.server.BaseHTTPRequestHandler):
        # HTTP/1.1 + explicit Content-Length: the guest's busybox wget reads
        # exactly that many bytes, so a healthy transfer ends cleanly instead of
        # relying on connection close (which can race a slow guest mid-stream).
        protocol_version = "HTTP/1.1"

        def log_message(self, *_a):  # quiet
            pass

        def do_GET(self):  # noqa: N802
            mapping = {
                f"/install/image/{IMAGE_NAME}": served_dir / IMAGE_NAME,
                f"/install/image/{IMAGE_NAME}.sha256": served_dir
                / f"{IMAGE_NAME}.sha256",
                "/install/database.zefs": served_dir / "database.zefs",
            }
            path = mapping.get(self.path)
            if path is None or not path.is_file():
                self.send_error(404)
                return
            self.send_response(200)
            self.send_header("Content-Type", "application/octet-stream")
            self.send_header("Content-Length", str(path.stat().st_size))
            self.end_headers()
            try:
                with open(path, "rb") as f:
                    shutil.copyfileobj(f, self.wfile)
            except (BrokenPipeError, ConnectionResetError):
                # Guest closed early (e.g. dd hit end-of-disk); not a server bug.
                pass

    return Handler


# ── QEMU ────────────────────────────────────────────────────────────────


# slirp's gateway address. The guest reaches host services through it, and
# unlike guestfwd it services unlimited sequential connections (guestfwd only
# pumps the first connection, stalling the installer's 2nd/3rd fetch). The
# installer is pointed here via ze.server + ze.port.
GUEST_SERVER_IP = "10.0.2.2"


def qemu_base(needs_bios: bool) -> list[str]:
    cmd = [QEMU_BIN, "-smp", "2", "-m", "1024", "-nographic", "-serial", "mon:stdio"]
    if ARCH == "arm64":
        cmd += ["-machine", f"virt,highmem=off,accel={QEMU_ACCEL}", "-cpu", "max"]
        if needs_bios:
            bios = os.environ.get(
                "ZE_INSTALL_AARCH64_BIOS",
                "/opt/homebrew/share/qemu/edk2-aarch64-code.fd",
            )
            cmd += ["-bios", bios]
    else:
        cmd += ["-machine", f"accel={QEMU_ACCEL}"]
    return cmd


def boot_installer(
    kernel: Path, initrd: Path, disk: Path, http_port: int, timeout: float
) -> str:
    """Boot kernel+initrd against the blank disk; return captured serial.

    Direct kernel boot, so no firmware (-bios) on arm64. The installer reaches
    the host HTTP server at GUEST_SERVER_IP:80, forwarded by QEMU to the host.
    """
    append = (
        f"console={'ttyAMA0' if ARCH == 'arm64' else 'ttyS0'} "
        f"ze.server={GUEST_SERVER_IP} ze.port={http_port} "
        f"ze.image={IMAGE_NAME} ip=dhcp panic=-1"
    )
    cmd = qemu_base(needs_bios=False) + [
        "-kernel",
        str(kernel),
        "-initrd",
        str(initrd),
        "-append",
        append,
        "-drive",
        f"file={disk},format=raw,if=virtio",
        "-netdev",
        # No guestfwd: the guest reaches the host HTTP server on the slirp
        # gateway (10.0.2.2) at the server's real port, which handles the
        # installer's multiple sequential downloads.
        "user,id=net0",
        "-device",
        f"{os.environ.get('ZE_INSTALL_NIC', 'virtio-net-pci')},netdev=net0",
    ]
    return _run_capture(cmd, timeout)


def boot_target_ssh(work: Path, disk: Path, timeout: float) -> bool:
    """Boot the written disk and attempt an SSH login as the power user (AC-10)."""
    # Pick a free host port (unless pinned): a fixed port collides with any other
    # QEMU forward already running on this machine.
    ssh_port = int(_SSH_PORT_ENV) if _SSH_PORT_ENV else _free_tcp_port()
    cmd = qemu_base(needs_bios=True) + [
        "-drive",
        f"file={disk},format=raw,if=virtio",
        "-netdev",
        f"user,id=net0,hostfwd=tcp::{ssh_port}-:22",
        "-device",
        "virtio-net-pci,netdev=net0",
    ]
    # Serial goes to a file, never an unread PIPE: gokrazy prints far more than a
    # 64 KiB pipe buffer holds, and an undrained pipe would block QEMU and hang
    # the boot before sshd starts. The file is dumped by the caller on failure.
    serial_path = work / "target-serial.log"
    with open(serial_path, "wb") as serial:
        proc = subprocess.Popen(cmd, stdout=serial, stderr=subprocess.STDOUT)
        try:
            deadline = time.time() + timeout
            while time.time() < deadline:
                if _ssh_login_ok(ssh_port):
                    return True
                time.sleep(3)
            return False
        finally:
            proc.terminate()
            try:
                proc.wait(timeout=5)
            except subprocess.TimeoutExpired:
                proc.kill()


# Password SSH against the installed appliance, proving loadZefsUsers resolved
# the config power user from zefs. paramiko (via uv) avoids a hard sshpass dep.
_PARAMIKO_LOGIN = """
import os, sys, paramiko
c = paramiko.SSHClient()
c.set_missing_host_key_policy(paramiko.AutoAddPolicy())
try:
    c.connect(os.environ["H"], port=int(os.environ["P"]), username=os.environ["U"],
              password=os.environ["PW"], timeout=5, allow_agent=False, look_for_keys=False)
    # Successful password authentication is the AC-10 proof: ze accepted the
    # power-user credentials it loaded from the installed zefs (loadZefsUsers).
    # ze's SSH endpoint is an interactive network-OS CLI, not a shell, so we
    # assert on the authenticated transport, not on remote command output.
    ok = c.get_transport() is not None and c.get_transport().is_authenticated()
    c.close()
    sys.stdout.write("ZE_SSH_OK" if ok else "ZE_SSH_NOAUTH")
except Exception as exc:
    sys.stderr.write(str(exc))
    sys.exit(1)
"""


def _ssh_login_ok(ssh_port: int) -> bool:
    env = os.environ.copy()
    env.update(H="127.0.0.1", P=str(ssh_port), U=SSH_USER, PW=SSH_PASS)
    # paramiko first: it checks SSH authentication directly, which is the AC-10
    # signal. The ssh/sshpass fallback runs a command and would misread ze's
    # interactive CLI (no shell `echo`), so it is only a last resort.
    uv = shutil.which("uv")
    if uv is not None:
        r = run(
            [
                uv,
                "run",
                "--quiet",
                "--with",
                "paramiko",
                "python3",
                "-c",
                _PARAMIKO_LOGIN,
            ],
            stdout=subprocess.PIPE,
            stderr=subprocess.DEVNULL,
            env=env,
        )
        return "ZE_SSH_OK" in r.stdout
    sshpass = shutil.which("sshpass")
    if sshpass is not None:
        # No usable uv/paramiko: fall back to ssh exit status. ze's CLI is not a
        # shell, so we cannot assert on command output; a clean exit (not 255)
        # means authentication succeeded.
        r = run(
            [
                sshpass,
                "-p",
                SSH_PASS,
                "ssh",
                "-o",
                "StrictHostKeyChecking=no",
                "-o",
                "UserKnownHostsFile=/dev/null",
                "-o",
                "ConnectTimeout=5",
                "-p",
                str(ssh_port),
                f"{SSH_USER}@127.0.0.1",
                "show version",
            ],
            stdout=subprocess.DEVNULL,
            stderr=subprocess.DEVNULL,
        )
        return r.returncode != 255
    return False


def _run_capture(cmd: list[str], timeout: float) -> str:
    proc = subprocess.Popen(
        cmd, stdout=subprocess.PIPE, stderr=subprocess.STDOUT, text=True
    )
    captured: list[str] = []
    deadline = time.time() + timeout
    try:
        while True:
            if proc.poll() is not None:
                captured.append(proc.stdout.read() or "")
                break
            line = proc.stdout.readline()
            if line:
                captured.append(line)
                if MARK_DONE in line:
                    break
            if time.time() > deadline:
                break
    finally:
        proc.terminate()
        try:
            proc.wait(timeout=5)
        except subprocess.TimeoutExpired:
            proc.kill()
    return "".join(captured)


# ── main ──────────────────────────────────────────────────────────────────


def main() -> int:
    root = repo_root()

    if shutil.which(QEMU_BIN) is None:
        return skip(f"{QEMU_BIN} not found")
    kernel = find_installer_kernel()
    if kernel is None:
        return skip(
            "no installer kernel — set ZE_INSTALL_KERNEL to a vmlinuz with "
            "IP_PNP_DHCP/VIRTIO_NET/VIRTIO_BLK/EXT4 built in (=y)"
        )
    tool_skip = have_image_build_tools(root)
    if tool_skip:
        return skip(tool_skip)

    work = Path(tempfile.mkdtemp(prefix="ze-install-qemu-"))
    keep = os.environ.get("ZE_INSTALL_KEEP") == "1"
    try:
        busybox = find_static_busybox(work)
        if busybox is None:
            return skip(
                "no static busybox (set ZE_INSTALL_BUSYBOX or run a container runtime)"
            )

        print(f"INSTALL-QEMU: arch={ARCH} accel={QEMU_ACCEL} kernel={kernel}")
        initrd = build_initrd(root, busybox)
        print(f"INSTALL-QEMU: initrd built ({initrd.stat().st_size} bytes)")

        image, zefs = build_image(root, work)
        print(f"INSTALL-QEMU: image built {image}")

        served = work / "served"
        served.mkdir()
        write_checksum(image, served)
        # The initrd downloads database.zefs and writes it to /perm, overwriting
        # the copy the build injected; serve the real credential-bearing zefs so
        # the AC-10 SSH login works on the second boot.
        if zefs and zefs.is_file():
            shutil.copy(zefs, served / "database.zefs")
            print(
                f"INSTALL-QEMU: serving zefs {zefs.name} ({zefs.stat().st_size} bytes)"
            )
        else:
            print("INSTALL-QEMU: FAIL no database.zefs produced by appliance assemble")
            return 1

        _httpd, port = start_http(served)
        print(f"INSTALL-QEMU: serving install artifacts on :{port}")

        # Target disk matches the image size exactly (gokrazy --full produces an
        # image of target_storage_bytes); a fixed 2 GiB would mismatch a resized
        # test image and break dd-to-device.
        disk = work / "target.img"
        with open(disk, "wb") as f:
            f.truncate(image.stat().st_size)

        # 300s headroom: the installer streams the full ~2 GiB image over
        # guestfwd and dd-writes it to the virtio disk before the success
        # markers print; the heaviest step in the chain.
        serial = boot_installer(
            kernel,
            initrd,
            disk,
            port,
            timeout=float(os.environ.get("ZE_INSTALL_BOOT_TIMEOUT", "300")),
        )
        if MARK_WRITTEN not in serial or MARK_DONE not in serial:
            sys.stdout.write(serial)
            print("INSTALL-QEMU: FAIL installer did not report success on serial")
            return 1
        print("INSTALL-QEMU: installer wrote disk + completed")

        if not boot_target_ssh(work, disk, timeout=120):
            print(
                "INSTALL-QEMU: FAIL second boot SSH login as power user failed (AC-10)"
            )
            target_serial = work / "target-serial.log"
            if target_serial.is_file():
                print("INSTALL-QEMU: --- target boot serial (tail) ---")
                data = target_serial.read_bytes()
                sys.stdout.buffer.write(data[-8000:])
                sys.stdout.flush()
            return 1
        print("INSTALL-QEMU: AC-10 SSH login as power user succeeded")

        print("INSTALL-QEMU: PASS")
        return 0
    finally:
        if not keep:
            shutil.rmtree(work, ignore_errors=True)


if __name__ == "__main__":
    sys.exit(main())
