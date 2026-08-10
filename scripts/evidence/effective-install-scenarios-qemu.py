#!/usr/bin/env python3
"""QEMU evidence for the installer's failure-path, pin, and rescue scenarios.

The happy-path HTTP and ISO installs are covered by effective-install-qemu.py
and effective-install-iso-qemu.py. This script adds the scenarios those two do
not exercise, each an acceptance criterion of the pure-Go installer initrd
work (docs/architecture/appliance/installer-initrd.md):

  fault    R-6      : a forced goroutine panic mid-init ends in the recovery/
                      reboot path (recover -> fatalInitrd), never an uncaught
                      crash that kills PID 1. Built with ZE_INITRD_FAULT=1
                      (ze_installer_fault build tag); the shipping initrd never
                      contains the hook.
  pin-ac4  AC-4     : on a multi-homed box, ze.mac pins the install to the
                      matching NIC; the foreign NIC is never brought up; the
                      server is reachable over the pinned NIC.
  pin-ac5  AC-5     : when the pinned NIC gets a lease but cannot reach the
                      server, its address/default route is flushed and the
                      remaining NIC recovers the install.

It reuses effective-install-qemu.py for initrd/image building, the slirp HTTP
server, and the QEMU command base, so the artifact under test is the exact one
the production path builds. Like the sibling scripts, a missing kernel / QEMU /
build tool prints one `INSTALL-SCENARIOS-QEMU: SKIP <reason>` line and exits 0;
a real failure exits non-zero with the captured serial.

Determinism note (AC-4/AC-5): ze.server is placed on the *reachable* NIC's
directly-connected slirp subnet (10.0.2.0/24, gateway 10.0.2.2), so the
connected route to it always wins over any competing default route a second NIC
might hold. The unreachable NIC sits on a different subnet with restrict=on, so
it still gets a DHCP lease + default route (exercising the flush path) but can
never carry traffic to the host.

The installer kernel is operator-supplied by design: set ZE_INSTALL_KERNEL to a
vmlinuz with IP_PNP_DHCP/VIRTIO_NET/VIRTIO_BLK/EXT4 built in (=y).
"""

from __future__ import annotations

import importlib.util
import os
import select
import shutil
import subprocess
import sys
import tempfile
import time
from pathlib import Path


def repo_root() -> Path:
    here = Path(__file__).resolve()
    for parent in [here.parent, *here.parents]:
        if (parent / "go.mod").is_file() and (parent / "cmd" / "ze").is_dir():
            return parent
    raise SystemExit("cannot locate repository root")


ROOT = repo_root()


def load_base():
    path = ROOT / "scripts" / "evidence" / "effective-install-qemu.py"
    spec = importlib.util.spec_from_file_location("effective_install_qemu", path)
    if spec is None or spec.loader is None:
        raise SystemExit(f"cannot load {path}")
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


base = load_base()

# Deterministic MACs for the multi-homed scenarios. ze.mac points at PINNED_MAC.
PINNED_MAC = "52:54:00:ab:cd:01"
FOREIGN_MAC = "52:54:00:ab:cd:02"


def skip(reason: str) -> int:
    print(f"INSTALL-SCENARIOS-QEMU: SKIP {reason}")
    return 0


def log(msg: str) -> None:
    print(f"INSTALL-SCENARIOS-QEMU: {msg}")


class ScenarioSkip(Exception):
    """Raised by a scenario when its prerequisites are unmet (counts as skip)."""


# ── shared initrd builders ────────────────────────────────────────────────


def _build_initrd_with_env(root: Path, work: Path, overrides: dict[str, str]) -> Path:
    """Build the Go initrd via the production path with temporary env overrides.

    base.build_initrd reads os.environ (GOARCH, XDG_CACHE_HOME, ZE_INITRD_FAULT)
    through os.environ.copy(), so set the overrides around the call. A distinct
    XDG_CACHE_HOME is essential: initrdCacheVariant keys only on version+arch+a
    source hash (internal/appliance/cache.go:121), NOT on build tags, so a fault
    build would otherwise be served a cached non-fault initrd (or vice versa).
    """
    prev = {k: os.environ.get(k) for k in overrides}
    os.environ.update(overrides)
    try:
        return base.build_initrd(root, work)
    finally:
        for k, v in prev.items():
            if v is None:
                os.environ.pop(k, None)
            else:
                os.environ[k] = v


def build_fault_initrd(root: Path, work: Path) -> Path:
    return _build_initrd_with_env(
        root,
        work,
        {"XDG_CACHE_HOME": str(work / "cache-fault"), "ZE_INITRD_FAULT": "1"},
    )


def build_normal_initrd(root: Path, work: Path) -> Path:
    return _build_initrd_with_env(
        root,
        work,
        {"XDG_CACHE_HOME": str(work / "cache-normal"), "ZE_INITRD_FAULT": ""},
    )


# ── shared install fixtures (image + slirp HTTP) ──────────────────────────


class InstallFixtures:
    def __init__(self, initrd: Path, image: Path, http_port: int):
        self.initrd = initrd
        self.image = image
        self.http_port = http_port


def setup_install_fixtures(root: Path, work: Path) -> InstallFixtures:
    """Build the normal initrd, an appliance image, and serve it over slirp.

    Raises ScenarioSkip when the image-build tooling (debugfs) is unavailable.
    """
    tool_skip = base.have_image_build_tools(root)
    if tool_skip:
        raise ScenarioSkip(tool_skip)

    log("building normal initrd + appliance image (shared by pin/rescue)")
    initrd = build_normal_initrd(root, work)
    image, zefs = base.build_image(root, work)

    served = work / "served"
    served.mkdir()
    base.write_checksum(image, served)
    if not (zefs and zefs.is_file()):
        raise ScenarioSkip("no database.zefs produced by appliance assemble")
    shutil.copy(zefs, served / "database.zefs")

    _httpd, port = base.start_http(served)
    log(f"serving install artifacts on :{port}")
    return InstallFixtures(initrd, image, port)


def _blank_target(work: Path, name: str, image: Path) -> Path:
    """A blank target disk sized to the image (gokrazy --full writes exactly that)."""
    disk = work / name
    with open(disk, "wb") as f:
        f.truncate(image.stat().st_size)
    return disk


# ── scenario: R-6 forced goroutine panic ──────────────────────────────────

# Emitted by internal/install/disk/fault_linux.go's recover handler. Its
# presence proves the per-goroutine recover ran instead of the runtime killing
# PID 1.
FAULT_RECOVER_MARK = "recovered goroutine panic"
# Printed by the kernel when PID 1 dies from an uncaught crash. Its ABSENCE is
# the R-6 guarantee: the goroutine panic was contained, not fatal to init.
KERNEL_INIT_PANIC = "Attempted to kill init"
# fatalInitrd's policy line (rescue_linux.go:71) before it reboots.
FATAL_POLICY_MARK = "fatal policy"


def boot_fault(kernel: Path, initrd: Path, disk: Path, timeout: float) -> str:
    """Boot the fault initrd; ze.fault=panic-goroutine fires before any install.

    -no-reboot makes QEMU exit when the guest requests a reset, so reaching the
    reboot path ends the VM (a hang would instead run to the timeout). The fault
    fires in RunInitrd before runHTTP, so no image or HTTP server is needed;
    ze.server only has to pass validation (valid IPv4 literal).
    """
    console = "ttyAMA0" if base.ARCH == "arm64" else "ttyS0"
    append = (
        f"console={console} ze.server={base.GUEST_SERVER_IP} "
        f"ze.image={base.IMAGE_NAME} ip=dhcp panic=-1 ze.fault=panic-goroutine"
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
        f"file={disk},format=raw,if=virtio",
        "-netdev",
        "user,id=net0",
        "-device",
        f"{os.environ.get('ZE_INSTALL_NIC', 'virtio-net-pci')},netdev=net0",
    ]
    return base._run_capture(cmd, timeout)


def scenario_fault(
    root: Path, work: Path, kernel: Path, _fx: InstallFixtures | None
) -> bool:
    """R-6: a goroutine panic ends in recovery/reboot, not a kernel-killed init."""
    log("scenario fault: building fault initrd (ze_installer_fault)")
    initrd = build_fault_initrd(root, work)
    log(f"scenario fault: fault initrd built ({initrd.stat().st_size} bytes)")

    disk = work / "fault-target.img"
    with open(disk, "wb") as f:
        f.truncate(64 << 20)  # small blank disk; the install is never reached

    # branchReboot sleeps 30s before unix.Reboot, so allow headroom over that.
    serial = boot_fault(
        kernel,
        initrd,
        disk,
        timeout=float(os.environ.get("ZE_INSTALL_FAULT_TIMEOUT", "90")),
    )

    if KERNEL_INIT_PANIC in serial:
        sys.stdout.write(serial)
        log(
            "FAIL fault: kernel reported PID-1 death (panic escaped recover, R-6 violated)"
        )
        return False
    if FAULT_RECOVER_MARK not in serial:
        sys.stdout.write(serial)
        log("FAIL fault: recover marker absent (goroutine panic was not caught)")
        return False
    if FATAL_POLICY_MARK not in serial:
        sys.stdout.write(serial)
        log("FAIL fault: recovered but never reached the FATAL/reboot path")
        return False
    log("PASS fault: goroutine panic recovered -> FATAL -> reboot; init never killed")
    return True


# ── scenarios: ze.mac pin (AC-4 / AC-5) ───────────────────────────────────

PIN_MARK = "pinning to boot NIC"
PIN_REACHABLE_MARK = "server reachable on pinned NIC"
PIN_FLUSH_MARK = "pinned NIC cannot reach server, flushing"
FALLBACK_MARK = "bringing up all NICs"


def boot_pin(
    kernel: Path,
    fx: InstallFixtures,
    disk: Path,
    pinned_netdev: str,
    foreign_netdev: str,
    timeout: float,
) -> str:
    """Boot a two-NIC topology with ze.mac pinned to PINNED_MAC.

    No ip=dhcp: the kernel configures no NIC, so the server probe fails and the
    installer must run its pin path (network.go:50). ze.server is an IPv4 on the
    reachable NIC's connected subnet, so reachability is route-deterministic.
    """
    console = "ttyAMA0" if base.ARCH == "arm64" else "ttyS0"
    nic = os.environ.get("ZE_INSTALL_NIC", "virtio-net-pci")
    append = (
        f"console={console} ze.server={base.GUEST_SERVER_IP} ze.port={fx.http_port} "
        f"ze.image={base.IMAGE_NAME} ze.mac={PINNED_MAC} panic=-1"
    )
    cmd = base.qemu_base(needs_bios=False) + [
        "-kernel",
        str(kernel),
        "-initrd",
        str(fx.initrd),
        "-append",
        append,
        "-drive",
        f"file={disk},format=raw,if=virtio",
        "-netdev",
        pinned_netdev,
        "-device",
        f"{nic},netdev=net0,mac={PINNED_MAC}",
        "-netdev",
        foreign_netdev,
        "-device",
        f"{nic},netdev=net1,mac={FOREIGN_MAC}",
    ]
    return base._run_capture(cmd, timeout)


def scenario_pin_ac4(
    root: Path, work: Path, kernel: Path, fx: InstallFixtures | None
) -> bool:
    """AC-4: pin to ze.mac NIC (reachable), foreign NIC never brought up."""
    if fx is None:
        raise ScenarioSkip("install fixtures unavailable")
    disk = _blank_target(work, "pin-ac4-target.img", fx.image)
    # net0 pinned + reachable (server 10.0.2.2 is on its connected subnet).
    # net1 foreign + isolated (different subnet, restrict=on): never touched.
    serial = boot_pin(
        kernel,
        fx,
        disk,
        pinned_netdev="user,id=net0",
        foreign_netdev="user,id=net1,net=10.0.99.0/24,restrict=on",
        timeout=float(os.environ.get("ZE_INSTALL_BOOT_TIMEOUT", "300")),
    )
    if PIN_MARK not in serial or PINNED_MAC not in serial:
        sys.stdout.write(serial)
        log("FAIL pin-ac4: installer did not pin to the ze.mac NIC")
        return False
    if PIN_REACHABLE_MARK not in serial:
        sys.stdout.write(serial)
        log("FAIL pin-ac4: server not reported reachable over the pinned NIC")
        return False
    if FALLBACK_MARK in serial:
        sys.stdout.write(serial)
        log("FAIL pin-ac4: fell back to bringing up all NICs (foreign NIC was touched)")
        return False
    if base.MARK_WRITTEN not in serial or base.MARK_DONE not in serial:
        sys.stdout.write(serial)
        log("FAIL pin-ac4: install did not complete over the pinned NIC")
        return False
    log("PASS pin-ac4: pinned to ze.mac NIC, foreign NIC never up, install completed")
    return True


def scenario_pin_ac5(
    root: Path, work: Path, kernel: Path, fx: InstallFixtures | None
) -> bool:
    """AC-5: pinned NIC leases but can't reach server -> flush -> recover."""
    if fx is None:
        raise ScenarioSkip("install fixtures unavailable")
    disk = _blank_target(work, "pin-ac5-target.img", fx.image)
    # net0 pinned but unreachable: leases on 10.0.88.0/24 (gateway 10.0.88.2)
    # with restrict=on, so it gets an address + default route it cannot use to
    # reach 10.0.2.2 -> the flush path fires.
    # net1 recovery: 10.0.2.0/24, server 10.0.2.2 directly connected -> wins.
    serial = boot_pin(
        kernel,
        fx,
        disk,
        pinned_netdev="user,id=net0,net=10.0.88.0/24,restrict=on",
        foreign_netdev="user,id=net1",
        timeout=float(os.environ.get("ZE_INSTALL_BOOT_TIMEOUT", "300")),
    )
    if PIN_MARK not in serial or PINNED_MAC not in serial:
        sys.stdout.write(serial)
        log("FAIL pin-ac5: installer did not pin to the ze.mac NIC")
        return False
    if PIN_FLUSH_MARK not in serial:
        sys.stdout.write(serial)
        log(
            "FAIL pin-ac5: pinned NIC was not flushed when it could not reach the server"
        )
        return False
    if FALLBACK_MARK not in serial:
        sys.stdout.write(serial)
        log("FAIL pin-ac5: did not scan remaining NICs after the flush")
        return False
    if base.MARK_WRITTEN not in serial or base.MARK_DONE not in serial:
        sys.stdout.write(serial)
        log("FAIL pin-ac5: install did not recover on the remaining NIC")
        return False
    log("PASS pin-ac5: pinned NIC flushed, install recovered on the remaining NIC")
    return True


# ── scenarios: rescue console (AC-7 / AC-7b / AC-7c) ──────────────────────

# A fixed rescue token and its encoded credential. ze.rescue-auth is
# "<saltHex>:<argon2idHex>" (internal/core/rescueauth), so the harness passes
# RESCUE_AUTH on the cmdline and types RESCUE_TOKEN. The value is pinned rather
# than computed here: reproducing argon2id in Python would need a dependency,
# and TestValuePinnedVector (internal/core/rescueauth) fails if the parameters
# drift away from this exact pair.
RESCUE_TOKEN = "ze-rescue-evidence"
RESCUE_AUTH = "5a65726573637565536f6c74303031ff:fed7b65bb317bc34097440c9bbd0a2ab3749edb8d88d3d37c94abe6cf62e399b"

# Markers emitted by internal/install/disk/rescue_linux.go.
TOKEN_PROMPT = "rescue token:"
AUTH_OK = "authenticated"
AUTH_BAD = "incorrect"
MENU_MARK = "Recovery Console"
REBOOT_30S = "rebooting in 30s"
# A dummy but format-valid (32 hex) ISO media id, so validateConfig passes and
# runISO proceeds to fail at "media not found" (forcing the FATAL we want).
DUMMY_MEDIA_ID = "00112233445566778899aabbccddeeff"


class SerialConsole:
    """Minimal pexpect-style driver over a QEMU serial on stdio.

    The rescue password prompt is printed with no trailing newline, so a
    line-based reader would block on it; this reads raw bytes via select+os.read
    and matches substrings in the accumulated buffer.
    """

    def __init__(self, cmd: list[str]):
        self.proc = subprocess.Popen(
            cmd,
            stdin=subprocess.PIPE,
            stdout=subprocess.PIPE,
            stderr=subprocess.STDOUT,
        )
        self.buf = ""

    def expect(self, needle: str, timeout: float) -> bool:
        deadline = time.time() + timeout
        while needle not in self.buf:
            remaining = deadline - time.time()
            if remaining <= 0:
                return False
            fd = self.proc.stdout.fileno()
            ready, _, _ = select.select([fd], [], [], min(1.0, remaining))
            if not ready:
                if self.proc.poll() is not None:
                    return needle in self.buf
                continue
            chunk = os.read(fd, 4096)
            if not chunk:  # EOF
                return needle in self.buf
            self.buf += chunk.decode("utf-8", "replace")
        return True

    def send_line(self, data: str) -> None:
        assert self.proc.stdin is not None
        self.proc.stdin.write((data + "\n").encode())
        self.proc.stdin.flush()

    def close(self) -> None:
        try:
            self.proc.terminate()
            self.proc.wait(timeout=5)
        except Exception:
            self.proc.kill()


def rescue_cmd(
    kernel: Path,
    initrd: Path,
    *,
    source: str,
    rescue_auth: str | None,
) -> list[str]:
    """Build a QEMU cmdline that forces a FATAL, then enters the rescue policy.

    HTTP source: ze.server is an address with no slirp service, so ensureNetwork
    fails fast (ze.wait=3) and runHTTP returns 1. ISO source: no media is
    attached, so runISO fails at "media not found". Either way RunInitrd reaches
    fatalInitrd, whose branch is chosen by (rescue_auth, source).
    """
    console = "ttyAMA0" if base.ARCH == "arm64" else "ttyS0"
    nic = os.environ.get("ZE_INSTALL_NIC", "virtio-net-pci")
    parts = [
        f"console={console}",
        "panic=-1",
        f"ze.source={source}",
        f"ze.image={base.IMAGE_NAME}",
    ]
    if source == "http":
        parts += ["ze.server=10.0.2.99", "ze.wait=3", "ip=dhcp"]
    else:
        parts.append(f"ze.media-id={DUMMY_MEDIA_ID}")
    if rescue_auth:
        parts.append(f"ze.rescue-auth={rescue_auth}")
    return base.qemu_base(needs_bios=False) + [
        "-no-reboot",
        "-kernel",
        str(kernel),
        "-initrd",
        str(initrd),
        "-append",
        " ".join(parts),
        "-netdev",
        "user,id=net0",
        "-device",
        f"{nic},netdev=net0",
    ]


def scenario_rescue_ac7(
    root: Path, work: Path, kernel: Path, _fx: InstallFixtures | None
) -> bool:
    """AC-7: gated console. Wrong password rejected, correct opens the menu."""
    initrd = build_normal_initrd(root, work)
    cmd = rescue_cmd(kernel, initrd, source="http", rescue_auth=RESCUE_AUTH)
    con = SerialConsole(cmd)
    step = float(os.environ.get("ZE_INSTALL_RESCUE_STEP_TIMEOUT", "120"))
    try:
        if not con.expect(TOKEN_PROMPT, step):
            log("FAIL rescue-ac7: gated password prompt never appeared")
            sys.stdout.write(con.buf)
            return False
        con.send_line("definitely-wrong")
        if not con.expect(AUTH_BAD, 30):
            log("FAIL rescue-ac7: wrong password was not rejected")
            sys.stdout.write(con.buf)
            return False
        con.send_line(RESCUE_TOKEN)
        if not con.expect(AUTH_OK, 30) or not con.expect(MENU_MARK, 30):
            log("FAIL rescue-ac7: correct password did not open the recovery menu")
            sys.stdout.write(con.buf)
            return False
        con.send_line("4")  # power off to end the session
        log("PASS rescue-ac7: wrong password rejected, correct opens gated menu")
        return True
    finally:
        con.close()


def scenario_rescue_ac7b(
    root: Path, work: Path, kernel: Path, _fx: InstallFixtures | None
) -> bool:
    """AC-7b: no credential + ISO source -> menu opens UNGATED (no password)."""
    initrd = build_normal_initrd(root, work)
    cmd = rescue_cmd(kernel, initrd, source="iso", rescue_auth=None)
    con = SerialConsole(cmd)
    step = float(os.environ.get("ZE_INSTALL_RESCUE_STEP_TIMEOUT", "120"))
    try:
        if not con.expect(MENU_MARK, step):
            log("FAIL rescue-ac7b: ungated recovery menu never appeared")
            sys.stdout.write(con.buf)
            return False
        if TOKEN_PROMPT in con.buf:
            log("FAIL rescue-ac7b: a password was demanded on an ungated ISO console")
            sys.stdout.write(con.buf)
            return False
        con.send_line("4")  # power off to end the session
        log("PASS rescue-ac7b: ISO + no credential opens the menu without a password")
        return True
    finally:
        con.close()


def scenario_rescue_ac7c(
    root: Path, work: Path, kernel: Path, _fx: InstallFixtures | None
) -> bool:
    """AC-7c: no credential + network -> no console, ~30s wait, reboot (no hang)."""
    initrd = build_normal_initrd(root, work)
    cmd = rescue_cmd(kernel, initrd, source="http", rescue_auth=None)
    # Non-interactive: -no-reboot makes QEMU exit when the 30s timer reboots, so
    # reaching the reboot proves it did not hang waiting for a password.
    serial = base._run_capture(
        cmd, float(os.environ.get("ZE_INSTALL_RESCUE_TIMEOUT", "120"))
    )
    if MENU_MARK in serial or TOKEN_PROMPT in serial:
        sys.stdout.write(serial)
        log(
            "FAIL rescue-ac7c: a rescue console was offered on an unattended network install"
        )
        return False
    if REBOOT_30S not in serial:
        sys.stdout.write(serial)
        log("FAIL rescue-ac7c: 30s-reboot safety valve marker absent")
        return False
    log(
        "PASS rescue-ac7c: network + no credential printed message and rebooted, never hung"
    )
    return True


# ── main ──────────────────────────────────────────────────────────────────

# (name, needs_install_fixtures, fn)
SCENARIOS = [
    ("fault", False, scenario_fault),
    ("pin-ac4", True, scenario_pin_ac4),
    ("pin-ac5", True, scenario_pin_ac5),
    ("rescue-ac7", False, scenario_rescue_ac7),
    ("rescue-ac7b", False, scenario_rescue_ac7b),
    ("rescue-ac7c", False, scenario_rescue_ac7c),
]


def main() -> int:
    if shutil.which(base.QEMU_BIN) is None:
        return skip(f"{base.QEMU_BIN} not found")
    kernel = base.find_installer_kernel()
    if kernel is None:
        return skip(
            "no installer kernel — set ZE_INSTALL_KERNEL to a vmlinuz with "
            "IP_PNP_DHCP/VIRTIO_NET/VIRTIO_BLK/EXT4 built in (=y)"
        )
    initrd_skip = base.have_initrd_build_tools()
    if initrd_skip:
        return skip(initrd_skip)

    work = Path(tempfile.mkdtemp(prefix="ze-install-scenarios-qemu-"))
    keep = os.environ.get("ZE_INSTALL_KEEP") == "1"
    try:
        log(f"arch={base.ARCH} accel={base.QEMU_ACCEL} kernel={kernel}")

        # Build the shared install fixtures once, lazily: only if a fixture-using
        # scenario is selected and the image tooling is present.
        fx: InstallFixtures | None = None
        if any(needs for _, needs, _ in SCENARIOS):
            try:
                fx = setup_install_fixtures(ROOT, work)
            except ScenarioSkip as exc:
                log(f"install fixtures unavailable, fixture scenarios will skip: {exc}")

        failures: list[str] = []
        skipped: list[str] = []
        passed: list[str] = []
        for name, needs, fn in SCENARIOS:
            if needs and fx is None:
                skipped.append(name)
                log(f"SKIP scenario {name} (no install fixtures)")
                continue
            try:
                ok = fn(ROOT, work, kernel, fx)
            except ScenarioSkip as exc:
                skipped.append(name)
                log(f"SKIP scenario {name}: {exc}")
                continue
            (passed if ok else failures).append(name)

        log(f"passed={passed} skipped={skipped} failed={failures}")
        if failures:
            log("FAIL one or more scenarios failed")
            return 1
        if not passed:
            return skip("no scenarios ran (all skipped)")
        log("PASS")
        return 0
    finally:
        if not keep:
            shutil.rmtree(work, ignore_errors=True)


if __name__ == "__main__":
    sys.exit(main())
