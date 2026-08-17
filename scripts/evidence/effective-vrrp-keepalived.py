#!/usr/bin/env python3
"""Run Ze's VRRP against a real keepalived peer on one L2 segment in Linux netns.

Blueprint: scripts/evidence/effective-l2tp-ppp.py (PID-suffixed names, netns
setup/cleanup, kernel probe, ensure_ze, LineCollector predicate waits,
diagnostics-on-failure, artifacts kept on failure).

Design: plan/spec-vrrp-6-interop.md -- "Scenario reference (QEMU netns lab)".
Driven by `make ze-qemu-vrrp-keepalived-test` (mk/test-integration.mk), which
boots the STOCK Alpine kernel (no --kernel) because VRRP needs only macvlan,
bridge and veth. ensure_kernel_support() below is the probe that proves it.

Topology (spec-vrrp-6): three leaf namespaces, each with a veth into a bridge
in a fourth namespace, so the observer is an independent third party that sees
every flooded multicast/broadcast frame.

    [ze netns]            [ka netns]           [observer netns]
      ZE_VETH ---.          KA_VETH ---.         OB_VETH ---.
                 |                     |                    |
              (veth)                (veth)               (veth)
                 |                     |                    |
    [lan netns:  BRIDGE  <- all three peer ends enslaved, multicast snooping off]

Everything asserted here is observable from OUTSIDE ze: tcpdump wire fields and
timestamps, keepalived notify markers, `ip -j neigh`, and ping exit codes. The
one ze-internal signal used is its state-change log line, and only as a
readiness/settled marker, never as the proof of an interop claim (spec-vrrp-6
"Core Insight"). Every timing band is measured from tcpdump wire timestamps.

Scenarios implemented: QS-1, QS-2, QS-3 (spec AC-1, AC-2, AC-3).
QS-4..QS-8 are declared but not implemented -- see PENDING_SCENARIOS.

Privileges: needs CAP_NET_ADMIN + CAP_NET_RAW (netns, veth, bridge, macvlan,
AF_PACKET). The QEMU Alpine VM runs this as root, which covers both.
"""

from __future__ import annotations

import dataclasses
import json
import os
import platform
import re
import shutil
import signal
import subprocess
import sys
import tempfile
import threading
import time
from pathlib import Path
from typing import Callable


# --- Topology names (PID-suffixed, per the blueprint :29-34) -----------------
# Namespace names are unbounded, device names must fit IFNAMSIZ-1 (15 chars):
# a 4-char prefix plus 6 suffix digits leaves headroom.
NS_SUFFIX = str(os.getpid())
VETH_SUFFIX = NS_SUFFIX[-6:]
ZE_NS = f"ze-vrrp-ze-{NS_SUFFIX}"
KA_NS = f"ze-vrrp-ka-{NS_SUFFIX}"
OB_NS = f"ze-vrrp-ob-{NS_SUFFIX}"
LAN_NS = f"ze-vrrp-lan-{NS_SUFFIX}"
PROBE_NS = f"ze-vrrp-probe-{NS_SUFFIX}"

# Leaf ends live in the three leaf namespaces; bridge ends are all enslaved to
# BRIDGE in LAN_NS. Both ends of a pair exist in the root namespace at creation
# time, which is why every name carries the PID suffix.
ZE_VETH = f"zvz{VETH_SUFFIX}"
ZE_BR_VETH = f"zvzb{VETH_SUFFIX}"
KA_VETH = f"zvk{VETH_SUFFIX}"
KA_BR_VETH = f"zvkb{VETH_SUFFIX}"
OB_VETH = f"zvo{VETH_SUFFIX}"
OB_BR_VETH = f"zvob{VETH_SUFFIX}"
BRIDGE = f"zvbr{VETH_SUFFIX}"

ALL_NAMESPACES = [ZE_NS, KA_NS, OB_NS, LAN_NS, PROBE_NS]

# keepalived's notify marker file: the machine-readable state source (spec
# assumption A-4). Written by NOTIFY_SCRIPT, polled by Lab.ka_state().
MARKER_NAME = "ka-state.log"
ROOT_VETH_NAMES = [
    ZE_VETH,
    ZE_BR_VETH,
    KA_VETH,
    KA_BR_VETH,
    OB_VETH,
    OB_BR_VETH,
]

# --- Addressing (RFC 5737 TEST-NET-1 documentation prefix) -------------------
PREFIX_LEN = "24"
VIP = "192.0.2.1"  # spec-vrrp-6 QS-1 row: the virtual address both routers claim
ZE_ADDR = "192.0.2.251"  # spec-vrrp-6 "Example ze config shape, QS-1"
KA_ADDR = "192.0.2.252"
OB_ADDR = "192.0.2.253"

# --- Protocol constants -----------------------------------------------------
VRID = 10  # spec-vrrp-6 QS-1 row
ZE_PRIORITY = 200  # spec-vrrp-6 QS-1 row: ze is the higher-priority router
KA_PRIORITY = 100  # spec-vrrp-6 QS-1 row: keepalived is the Backup
ADVERT_MS = 1000  # ze config leaf advertise-interval-milliseconds
ADVERT_S = ADVERT_MS / 1000.0
# RFC 9568 Section 5.2.7: the v3 Max Advertise Interval field is a 12-bit value
# in CENTISECONDS, so 1000 ms maps to exactly 100 cs (spec Boundary Tests row
# "QS-1 advert interval field": 100 is the only valid value, 99 and 101 fail).
ADVERT_CS = 100
# RFC 9568 Section 7.3: the IPv4 Virtual Router MAC is 00-00-5E-00-01-{VRID}.
# VRID 10 = 0x0a. ze egresses adverts and GARPs from a macvlan carrying this
# address (transport/backend_linux.go:138-139, garp.go BuildGARP).
VIRTUAL_MAC = "00:00:5e:00:01:0a"
# RFC 9568 Section 5.1.1.3: IPv4 adverts MUST carry TTL 255; a receiver MUST
# discard anything else, which is what makes this assertable end to end.
ADVERT_TTL = 255
# RFC 9568 Section 5.1.1.2: the IPv4 VRRP destination multicast group.
VRRP_MCAST_V4 = "224.0.0.18"
VRRP_TYPE_ADVERTISEMENT = "Advertisement"  # tcpdump's token for VRRP type 1
VRRP_VERSION_V3 = 3  # RFC 9568


def skew_time_s(priority: int, advert_s: float) -> float:
    """RFC 9568 Section 6.1: Skew_Time = ((256 - priority) * Adver_Interval) / 256."""
    return ((256 - priority) * advert_s) / 256.0


def master_down_s(priority: int, advert_s: float) -> float:
    """RFC 9568 Section 6.1: Active_Down_Interval = 3 * Adver_Interval + Skew_Time."""
    return (3.0 * advert_s) + skew_time_s(priority, advert_s)


# Derived, for assertion messages and observation windows. At advert 1 s:
# ze prio 200 -> skew 0.21875 s, master-down 3.21875 s.
# keepalived prio 100 -> skew 0.609375 s, master-down 3.609375 s.
ZE_MASTER_DOWN_S = master_down_s(ZE_PRIORITY, ADVERT_S)
KA_MASTER_DOWN_S = master_down_s(KA_PRIORITY, ADVERT_S)
KA_SKEW_S = skew_time_s(KA_PRIORITY, ADVERT_S)

# --- Timing acceptance bands (spec-vrrp-6 "Boundary Tests") ------------------
# All measured from tcpdump wire timestamps (-tt, CLOCK_REALTIME epoch), never
# from wall-clock durations across process scheduling (spec risk R-2).
#
# QS-2: silence -> keepalived promotion, from the last ze advert.
# Below 3.0 s would mean keepalived never timed against ze's adverts; above
# 6.0 s means its 3.609 s master-down plus TCG margin was exceeded.
QS2_PROMOTE_MIN_S = 3.0
QS2_PROMOTE_MAX_S = 6.0
# QS-2: ze preempt-return promotion. Below 2.8 s would mean ze preempted before
# waiting out its own 3.21875 s master-down.
QS2_PREEMPT_MIN_S = 2.8
QS2_PREEMPT_MAX_S = 6.0
# QS-3: prio-0 -> keepalived promotion. The skew path is <= 3.0 s; the
# distinguishing bound is 3.609 s (a full master-down would mean the prio-0
# advert was ignored). RFC 9568 Section 6.4.2: on a Priority-0 advert a Backup
# sets its master-down timer to Skew_Time (0.609 s here), not Active_Down.
QS3_PRIO0_PROMOTE_MAX_S = 3.0
QS3_NO_SKEW_PATH_S = 3.61

# --- Wait timeouts (generous: QEMU may fall back to TCG software emulation) --
CAPTURE_READY_TIMEOUT_S = 20.0
ZE_START_TIMEOUT_S = 45.0
ZE_MASTER_TIMEOUT_S = 45.0
KA_STATE_TIMEOUT_S = 30.0
WIRE_EVENT_TIMEOUT_S = 30.0
# keepalived's garp_master_delay defaults to 5 s, so its GARP burst trails its
# promotion by that much. Left at the default for fidelity; the window absorbs it.
KA_GARP_TIMEOUT_S = 25.0
PING_TIMEOUT_S = 20.0


def repo_root() -> Path:
    here = Path(__file__).resolve()
    for parent in here.parents:
        if (parent / "go.mod").is_file():
            return parent
    raise SystemExit("cannot locate repository root")


def run(cmd: list[str], **kwargs) -> subprocess.CompletedProcess[str]:
    return subprocess.run(cmd, text=True, check=False, **kwargs)


def run_required(
    cmd: list[str], context: str, **kwargs
) -> subprocess.CompletedProcess[str]:
    result = run(cmd, stdout=subprocess.PIPE, stderr=subprocess.PIPE, **kwargs)
    if result.returncode != 0:
        sys.stderr.write((result.stdout or "") + (result.stderr or ""))
        raise RuntimeError(f"{context} failed")
    return result


def ns_run(ns: str, cmd: list[str], **kwargs) -> subprocess.CompletedProcess[str]:
    return run(["ip", "netns", "exec", ns, *cmd], **kwargs)


def ns_run_required(
    ns: str, cmd: list[str], context: str, **kwargs
) -> subprocess.CompletedProcess[str]:
    return run_required(["ip", "netns", "exec", ns, *cmd], context, **kwargs)


def ns_popen(ns: str, cmd: list[str], **kwargs) -> subprocess.Popen[str]:
    return subprocess.Popen(["ip", "netns", "exec", ns, *cmd], **kwargs)


def ns_output(ns: str, cmd: list[str]) -> str:
    result = ns_run(ns, cmd, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
    return (result.stdout or "") + (result.stderr or "")


def require_cmd(name: str) -> None:
    if shutil.which(name) is None:
        raise SystemExit(f"missing required command: {name}")


def terminate(proc: subprocess.Popen[str] | None, grace: float = 3.0) -> None:
    if proc is None or proc.poll() is not None:
        return
    proc.send_signal(signal.SIGTERM)
    try:
        proc.wait(timeout=grace)
    except subprocess.TimeoutExpired:
        proc.kill()
        proc.wait(timeout=2.0)


def has_cap_net_admin() -> bool:
    if os.geteuid() == 0:
        return True
    status = Path("/proc/self/status")
    if not status.is_file():
        return False
    for line in status.read_text(encoding="utf-8", errors="replace").splitlines():
        if not line.startswith("CapEff:"):
            continue
        cap_eff = int(line.split()[1], 16)
        # CAP_NET_ADMIN is bit 12 (linux/capability.h).
        return bool(cap_eff & (1 << 12))
    return False


def wait_until(
    predicate: Callable[[], bool], timeout_s: float, interval_s: float = 0.1
) -> bool:
    """Deadline-bounded predicate poll.

    The only sleep in this file is this poll interval and the teardown settle
    below: no fixed duration is ever used as a synchronization primitive
    (ai/rules/testing.md, spec-vrrp-6 Required Reading).
    """
    deadline = time.time() + timeout_s
    while True:
        if predicate():
            return True
        now = time.time()
        if now >= deadline:
            return False
        time.sleep(min(interval_s, max(0.0, deadline - now)))


def try_load_modules() -> None:
    """Best-effort module load. Built-in (=y) kernels have nothing to load, so a
    failure here is not itself an error: ensure_kernel_support probes function,
    not module presence."""
    modprobe = shutil.which("modprobe")
    if modprobe is None or os.geteuid() != 0:
        return
    for mod in ["veth", "bridge", "macvlan", "dummy"]:
        run([modprobe, mod], stdout=subprocess.PIPE, stderr=subprocess.PIPE)


def kill_netns_processes(ns: str, sig: signal.Signals) -> None:
    pids = run(
        ["ip", "netns", "pids", ns], stdout=subprocess.PIPE, stderr=subprocess.PIPE
    )
    if pids.returncode != 0:
        return
    for raw in (pids.stdout or "").split():
        try:
            os.kill(int(raw), sig)
        except (ValueError, ProcessLookupError, PermissionError):
            pass


def cleanup_netns() -> None:
    """Tear the lab down. Safe to call when nothing exists (cleanup-first setup)."""
    for ns in ALL_NAMESPACES:
        kill_netns_processes(ns, signal.SIGTERM)
    time.sleep(0.2)
    for ns in ALL_NAMESPACES:
        kill_netns_processes(ns, signal.SIGKILL)
    # Deleting a namespace destroys the devices inside it; these deletes only
    # matter for a run that died between veth creation and the netns move.
    for link in ROOT_VETH_NAMES:
        run(
            ["ip", "link", "delete", link],
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
        )
    for ns in ALL_NAMESPACES:
        run(
            ["ip", "netns", "delete", ns],
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
        )


def add_leaf(ns: str, leaf: str, bridge_end: str, addr: str | None) -> None:
    """Create one leaf: veth pair, leaf end into ns, bridge end enslaved to BRIDGE."""
    run_required(
        ["ip", "link", "add", leaf, "type", "veth", "peer", "name", bridge_end],
        f"create veth pair {leaf}/{bridge_end}",
    )
    run_required(["ip", "link", "set", leaf, "netns", ns], f"move {leaf} into {ns}")
    run_required(
        ["ip", "link", "set", bridge_end, "netns", LAN_NS],
        f"move {bridge_end} into {LAN_NS}",
    )
    ns_run_required(
        LAN_NS,
        ["ip", "link", "set", bridge_end, "master", BRIDGE],
        f"enslave {bridge_end} to {BRIDGE}",
    )
    ns_run_required(LAN_NS, ["ip", "link", "set", bridge_end, "up"], f"up {bridge_end}")
    ns_run_required(ns, ["ip", "link", "set", "lo", "up"], f"up loopback in {ns}")
    ns_run_required(ns, ["ip", "link", "set", leaf, "up"], f"up {leaf}")
    if addr is not None:
        ns_run_required(
            ns,
            ["ip", "addr", "add", f"{addr}/{PREFIX_LEN}", "dev", leaf],
            f"assign {addr} to {leaf}",
        )


def setup_netns() -> None:
    """Build the lab: bridge namespace, three leaves, underlay reachability proof."""
    cleanup_netns()
    Path("/run/netns").mkdir(parents=True, exist_ok=True)
    for ns in [LAN_NS, ZE_NS, KA_NS, OB_NS]:
        run_required(["ip", "netns", "add", ns], f"create netns {ns}")

    ns_run_required(LAN_NS, ["ip", "link", "set", "lo", "up"], "up lan loopback")
    ns_run_required(
        LAN_NS, ["ip", "link", "add", BRIDGE, "type", "bridge"], f"create {BRIDGE}"
    )
    # spec-vrrp-6 topology: "no snooping filter". VRRP's 224.0.0.18 is
    # link-local multicast, which snooping already floods, but an explicit off
    # removes the whole question (and covers the ff02::12 IPv6 case QS-6 adds).
    ns_run_required(
        LAN_NS,
        ["ip", "link", "set", BRIDGE, "type", "bridge", "mcast_snooping", "0"],
        f"disable multicast snooping on {BRIDGE}",
    )
    ns_run_required(LAN_NS, ["ip", "link", "set", BRIDGE, "up"], f"up {BRIDGE}")

    # ze gets NO pre-assigned address: its config owns 192.0.2.251/24, so the
    # lab exercises the production iface path rather than faking its state.
    add_leaf(ZE_NS, ZE_VETH, ZE_BR_VETH, None)
    add_leaf(KA_NS, KA_VETH, KA_BR_VETH, KA_ADDR)
    add_leaf(OB_NS, OB_VETH, OB_BR_VETH, OB_ADDR)

    # Underlay proof (blueprint :171-179): the bridge forwards between two
    # statically addressed leaves. Uses ka and ob, not ze, because ze has no
    # address until its daemon applies its config.
    ping = ns_run(
        OB_NS,
        ["ping", "-c", "1", "-W", "3", KA_ADDR],
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
    )
    if ping.returncode != 0:
        sys.stderr.write((ping.stdout or "") + (ping.stderr or ""))
        raise RuntimeError(
            f"observer namespace cannot reach the keepalived namespace over {BRIDGE}"
        )


def probe_kernel_feature(cmd: list[str], config_symbol: str, what: str) -> None:
    """Create a device inside PROBE_NS; a failure names the missing CONFIG_*.

    Probing by CREATION, not by /proc/config.gz or module presence: a built-in
    (=y) feature has no module, and a module present but broken still fails. The
    lab needs the feature to work, so working is what gets probed.
    """
    result = ns_run(PROBE_NS, cmd, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
    if result.returncode != 0:
        sys.stderr.write((result.stdout or "") + (result.stderr or ""))
        raise SystemExit(
            f"kernel lacks {what}: `{' '.join(cmd)}` failed in a private namespace. "
            f"This lab needs {config_symbol} in the running kernel. "
            f"Either boot a kernel with {config_symbol} built in (add it to "
            f"gokrazy/kernel/runtime.config and run `make ze-kernel-build`, then pass "
            f"--kernel tmp/kernel/vmlinuz to qemu-run.py like the l2tp/pppoe labs "
            f"in mk/test-integration.mk), or install the matching module package."
        )


def ensure_kernel_support() -> None:
    """Prove the running kernel can build this lab, or fail naming the CONFIG_*.

    The probe result is the evidence for which kernel the make target needs.
    `ze-qemu-vrrp-keepalived-test` deliberately runs the STOCK Alpine kernel (no
    --kernel), unlike the l2tp/pppoe labs: those need CONFIG_PPPOL2TP/CONFIG_PPPOE
    which Alpine lacks, whereas macvlan, bridge and veth are all present. If that
    ever stops being true, this function is what says so, in one line, instead of
    a confusing failure five scenarios later.

    Everything is probed inside PROBE_NS, so the host default namespace is never
    touched beyond creating and deleting that namespace.
    """
    if platform.system() != "Linux":
        raise SystemExit("VRRP keepalived interop evidence requires Linux")
    if not has_cap_net_admin():
        raise SystemExit(
            "VRRP keepalived interop evidence requires root or CAP_NET_ADMIN "
            "(netns/veth/bridge/macvlan creation) plus CAP_NET_RAW (ze's raw "
            "proto-112 and AF_PACKET sockets)"
        )

    try_load_modules()

    run(
        ["ip", "netns", "delete", PROBE_NS],
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
    )
    Path("/run/netns").mkdir(parents=True, exist_ok=True)
    run_required(["ip", "netns", "add", PROBE_NS], f"create probe netns {PROBE_NS}")
    try:
        # veth: the lab's only transport between namespaces.
        probe_kernel_feature(
            ["ip", "link", "add", "zprobe0", "type", "veth", "peer", "name", "zprobe1"],
            "CONFIG_VETH",
            "veth pair support",
        )
        # bridge: the shared L2 segment the three leaves meet on.
        probe_kernel_feature(
            ["ip", "link", "add", "zprobebr", "type", "bridge"],
            "CONFIG_BRIDGE",
            "bridge support",
        )
        # macvlan in BRIDGE mode: ze gives every virtual router its own macvlan
        # carrying the RFC 9568 virtual MAC (internal/plugins/vrrp/engine.go:103-118,
        # iface/device_owner.go:49 "bridge-mode macvlan"). No macvlan, no VRRP.
        probe_kernel_feature(
            [
                "ip",
                "link",
                "add",
                "zprobemv",
                "link",
                "zprobe0",
                "type",
                "macvlan",
                "mode",
                "bridge",
            ],
            "CONFIG_MACVLAN",
            "bridge-mode macvlan support",
        )
        release = platform.release()
        print(
            f"kernel probe: {release}: veth OK, bridge OK, macvlan (bridge mode) OK "
            f"-- stock kernel is sufficient, no custom kernel needed"
        )
    finally:
        run(
            ["ip", "netns", "delete", PROBE_NS],
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
        )


def ensure_ze(root: Path) -> Path:
    """Build ze from the workspace, or honor an override (blueprint :232-258)."""
    override = os.environ.get("ZE_EVIDENCE_ZE_BINARY") or os.environ.get(
        "ZE_VRRP_KEEPALIVED_ZE_BINARY"
    )
    if override:
        ze = Path(override)
        if not ze.is_file():
            raise SystemExit(f"ze binary override does not exist: {ze}")
        if not os.access(ze, os.X_OK):
            raise SystemExit(f"ze binary override is not executable: {ze}")
        return ze

    require_cmd("go")
    bindir = root / "tmp" / "evidence" / "bin"
    bindir.mkdir(parents=True, exist_ok=True)
    ze = bindir / "ze-vrrp-keepalived"

    env = os.environ.copy()
    env["CGO_ENABLED"] = "0"
    env.setdefault("GOCACHE", str(root / "tmp" / "go-cache"))
    build = run(
        ["go", "build", "-tags", "ze_core,ze_distro", "-o", str(ze), "./cmd/ze"],
        cwd=root,
        env=env,
    )
    if build.returncode != 0:
        raise SystemExit("go build ./cmd/ze failed")
    return ze


class LineCollector:
    """Blueprint :261-304. Collects a stream's lines and offers predicate waits."""

    def __init__(self, prefix: str, stream) -> None:
        self.prefix = prefix
        self.lines: list[str] = []
        self.cond = threading.Condition()
        self.thread = threading.Thread(target=self._worker, args=(stream,), daemon=True)
        self.thread.start()

    def _worker(self, stream) -> None:
        try:
            for line in stream:
                with self.cond:
                    self.lines.append(line)
                    self.cond.notify_all()
                sys.stderr.write(self.prefix + line)
        except (ValueError, OSError):
            pass

    def snapshot(self) -> list[str]:
        with self.cond:
            return list(self.lines)

    def wait_for(
        self,
        predicate: Callable[[list[str]], bool],
        timeout_s: float,
        proc: subprocess.Popen[str] | None = None,
        fatal: Callable[[list[str]], str | None] | None = None,
    ) -> bool:
        deadline = time.time() + timeout_s
        while time.time() < deadline:
            with self.cond:
                lines = list(self.lines)
                if fatal is not None:
                    msg = fatal(lines)
                    if msg is not None:
                        raise RuntimeError(msg)
                if predicate(lines):
                    return True
                if proc is not None and proc.poll() is not None:
                    return False
                remaining = max(0.0, deadline - time.time())
                self.cond.wait(timeout=min(0.2, remaining))
        return False


# Needles that mean the run can only fail from here, so waits abort early
# instead of burning their full timeout. Sources (all read this session):
#   engine.go:95        "vrrp: instance create failed"  (macvlan or transport)
#   register.go:127     "vrrp: this plugin must run in-process"
#   instance.go:371     "vrrp: install virtual addresses failed"
#   instance.go:341     "vrrp: unhandled FSM action, feature will not work"
#   plugin/server/startup.go:101 "auto-load config path plugin startup failed"
# Deliberately NOT fatal: "vrrp: parent unusable, stopping virtual router"
# (instance.go:234) is the EXPECTED reaction to QS-2's carrier loss, and
# "send advertisement failed" (instance.go:362) can fire transiently while the
# parent address is still being resolved.
FATAL_NEEDLES = [
    "vrrp: instance create failed",
    "vrrp: this plugin must run in-process",
    "vrrp: install virtual addresses failed",
    "vrrp: unhandled FSM action",
    "auto-load config path plugin startup failed",
]


def fatal_ze(lines: list[str]) -> str | None:
    for line in lines:
        for needle in FATAL_NEEDLES:
            if needle in line:
                return f"ze reported fatal failure: {needle}"
    return None


def lines_contain(needle: str) -> Callable[[list[str]], bool]:
    return lambda lines: any(needle in line for line in lines)


def ze_state_is(state: str) -> Callable[[list[str]], bool]:
    """Match ze's production state-change log line.

    Producer: emitStateChange, internal/plugins/vrrp/engine.go:320-324, which
    logs at Info: msg "vrrp: state change" with from=/to= attributes. The values
    come from viewState (internal/plugins/vrrp/vrrp.go:61-72) and are lowercase:
    initialize / backup / master. slogutil renders attributes as key=value via
    slog.NewTextHandler whenever the sink is not a TTY, which a pipe never is
    (internal/core/slogutil/slogutil.go:294-299).

    Info is below the default warn level, so ZE_LOG_VRRP=info is mandatory for
    this line to exist at all (slogutil.go:183-195). ze_env() sets it.
    """
    needle = f"to={state}"
    return lambda lines: any(
        "vrrp: state change" in line and needle in line for line in lines
    )


# --- tcpdump capture parsing -------------------------------------------------
#
# Assumption A-2 of the spec (tcpdump's printers expose every asserted field) is
# settled here. tcpdump -e -vv -tt -n renders one IPv4 VRRP advert as a record of
# two lines, the second indented:
#
#   1752580000.123456 00:00:5e:00:01:0a > 01:00:5e:00:00:12, ethertype IPv4 \
#       (0x0800), length 54: (tos 0xc0, ttl 255, id 0, offset 0, flags [none], \
#       proto VRRP (112), length 40)
#       192.0.2.251 > 224.0.0.18: VRRPv3, Advertisement, vrid 10, prio 200, \
#       intvl 100cs, length 12
#
# and a gratuitous ARP as a single line:
#
#   1752580000.500000 00:00:5e:00:01:0a > ff:ff:ff:ff:ff:ff, ethertype ARP \
#       (0x0806), length 42: Request who-has 192.0.2.1 (00:00:5e:00:01:0a) \
#       tell 192.0.2.1, length 28
#
# The parsers below are tolerant (regex over the joined record, optional groups)
# rather than exact-match, because tcpdump's exact wording drifts between
# releases (spec risk R-3). A record that does not parse is skipped, never
# guessed at, and assert_* failures print the raw records so a printer change is
# diagnosable in one run instead of silently passing.

RECORD_START = re.compile(r"^(\d+\.\d+)\s")
ETHER_RE = re.compile(r"^\d+\.\d+\s+([0-9a-f:]{17})\s+>\s+([0-9a-f:]{17}),")
TTL_RE = re.compile(r"\bttl (\d+)")
VRRP_RE = re.compile(r"([\d.]+) > ([\d.]+): VRRPv(\d), (\w+), vrid (\d+), prio (\d+)")
INTVL_RE = re.compile(r"intvl (\d+)(cs|s)\b")
# tcpdump prints the target hardware address in parentheses only when it is
# non-zero, which for an RFC 9568 errata-7947/7949 GARP it always is.
GARP_RE = re.compile(r"Request who-has ([\d.]+)(?: \(([0-9a-f:]+)\))? tell ([\d.]+)")


@dataclasses.dataclass(frozen=True)
class Advert:
    ts: float
    ether_src: str
    ether_dst: str
    ip_src: str
    ip_dst: str
    version: int
    kind: str
    vrid: int
    priority: int
    ttl: int | None
    interval: int | None
    interval_unit: str | None
    raw: str


@dataclasses.dataclass(frozen=True)
class Garp:
    ts: float
    ether_src: str
    ether_dst: str
    sender_ip: str
    target_ip: str
    target_mac: str | None
    raw: str


def capture_records(lines: list[str]) -> list[list[str]]:
    """Group tcpdump output into one record per packet (a record starts with -tt's
    epoch timestamp; every following indented line belongs to it)."""
    records: list[list[str]] = []
    current: list[str] | None = None
    for line in lines:
        if RECORD_START.match(line):
            if current is not None:
                records.append(current)
            current = [line.rstrip("\n")]
        elif current is not None:
            current.append(line.rstrip("\n"))
    if current is not None:
        records.append(current)
    return records


def parse_adverts(lines: list[str]) -> list[Advert]:
    out: list[Advert] = []
    for record in capture_records(lines):
        raw = " ".join(part.strip() for part in record)
        vrrp = VRRP_RE.search(raw)
        ether = ETHER_RE.match(record[0])
        start = RECORD_START.match(record[0])
        if not (vrrp and ether and start):
            continue
        ttl = TTL_RE.search(raw)
        intvl = INTVL_RE.search(raw)
        out.append(
            Advert(
                ts=float(start.group(1)),
                ether_src=ether.group(1),
                ether_dst=ether.group(2),
                ip_src=vrrp.group(1),
                ip_dst=vrrp.group(2),
                version=int(vrrp.group(3)),
                kind=vrrp.group(4),
                vrid=int(vrrp.group(5)),
                priority=int(vrrp.group(6)),
                ttl=int(ttl.group(1)) if ttl else None,
                interval=int(intvl.group(1)) if intvl else None,
                interval_unit=intvl.group(2) if intvl else None,
                raw=raw,
            )
        )
    return out


def parse_garps(lines: list[str]) -> list[Garp]:
    out: list[Garp] = []
    for record in capture_records(lines):
        raw = " ".join(part.strip() for part in record)
        if "ethertype ARP" not in raw:
            continue
        garp = GARP_RE.search(raw)
        ether = ETHER_RE.match(record[0])
        start = RECORD_START.match(record[0])
        if not (garp and ether and start):
            continue
        target_ip, target_mac, sender_ip = garp.group(1), garp.group(2), garp.group(3)
        # RFC 9568 Section 8.1.2 / errata 7947-7949: a gratuitous ARP has sender
        # IP == target IP. A plain "who-has X tell Y" lookup is not one.
        if sender_ip != target_ip:
            continue
        out.append(
            Garp(
                ts=float(start.group(1)),
                ether_src=ether.group(1),
                ether_dst=ether.group(2),
                sender_ip=sender_ip,
                target_ip=target_ip,
                target_mac=target_mac,
                raw=raw,
            )
        )
    return out


# --- Config generation -------------------------------------------------------


def ze_conf_v3_ipv4(priority: int, preempt: bool = True) -> str:
    """The QS-1 ze config shape.

    Deviation from the spec's "Example ze config shape": the spec writes
    `group 10 { ... }` (keyed by VRID). The landed YANG keys the list by an
    operator NAME with a mandatory vrid leaf
    (internal/plugins/vrrp/yang/ze-vrrp-conf.yang:32-52), and leaf-lists use the
    bracket syntax (ze:syntax "bracket", :58). This is the shape
    test/vrrp/vrrp-config.ci:13-47 validates, so it is the one used here.

    accept-mode true per the spec QS-1 row and assumption A-7. Note the YANG
    documents accept-mode as FSM/owner semantics only, "not dataplane-enforced
    this pass" (ze-vrrp-conf.yang:121-129), so the VIP answers ping either way;
    the leaf is set because the scenario calls for a non-owner Active that
    accepts VIP traffic, not because the ping assertion depends on it.
    """
    return f"""interface {{
    backend netlink;
    ethernet {ZE_VETH} {{
        unit 0 {{
            ipv4 {{
                address [ {ZE_ADDR}/{PREFIX_LEN} ];
                vrrp {{
                    group lab {{
                        vrid {VRID};
                        virtual-address [ {VIP} ];
                        priority {priority};
                        preempt {"true" if preempt else "false"};
                        accept-mode true;
                        advertise-interval-milliseconds {ADVERT_MS};
                    }}
                }}
            }}
        }}
    }}
}}
"""


def keepalived_conf_v3_ipv4(notify: Path, marker: Path, priority: int) -> str:
    """The QS-1 keepalived peer config (spec-vrrp-6 "Example keepalived peer config").

    `vrrp_version 3` is load-bearing: keepalived speaks VRRPv2 by default for
    IPv4, and a v3-only ze silently ignores v2 adverts, so both sides must be
    pinned to the same version (spec "Key insights"). QS-5 is the v2 pairing.

    The notify markers are the machine-readable state source (spec Key Design
    Decisions: markers are a documented, version-stable contract, unlike log
    text which drifts, risk R-3). Written as one executable script taking the
    state and the marker path as its arguments, rather than the spec's inline
    `/bin/sh -c 'echo ... >> ...'`: same contract, no nested quoting through
    keepalived's config parser. keepalived appends its own
    (type, name, state, priority) arguments after ours, which the script ignores.

    notify is the script path; it MUST be on a path with no group/other-writable
    component (see Lab.setup and the security note below). marker is where the
    script appends state; it can be anywhere the root-run script can write.
    """
    # enable_script_security is mandatory in keepalived 2.x whenever notify_*
    # scripts are configured: without it keepalived refuses the whole config
    # with "SECURITY VIOLATION - scripts are being executed but script_security
    # not enabled" and never runs, so the notify markers (our state source)
    # never appear. But enable_script_security alone makes keepalived try to run
    # the scripts as user "keepalived_script", which the minimal Alpine image
    # lacks ("Unable to set default user keepalived_script for script"), so
    # script_user root pins them to root, which exists. All three lines are
    # required together; probed against keepalived 2.3.1 on 2026-07-15. keepalived
    # ALSO disables a root-run script whose path is writable by group/other,
    # which the 9p-mounted repo tree is (security_model=none), so Lab.setup keeps
    # the script under /root -- see Lab.setup's secure-dir note.
    return f"""global_defs {{
    vrrp_version 3
    enable_script_security
    script_user root
}}
vrrp_instance lab {{
    state BACKUP
    interface {KA_VETH}
    virtual_router_id {VRID}
    priority {priority}
    advert_int {int(ADVERT_S)}
    virtual_ipaddress {{
        {VIP}/{PREFIX_LEN}
    }}
    notify_master "{notify} MASTER {marker}"
    notify_backup "{notify} BACKUP {marker}"
    notify_fault  "{notify} FAULT {marker}"
}}
"""


NOTIFY_SCRIPT = """#!/bin/sh
# keepalived notify hook: append the new state to the marker file the evidence
# script polls. $1 and $2 are our own arguments (state, marker path); keepalived
# appends its (type, name, state, priority) arguments after them, unused here.
printf '%s\\n' "$1" >> "$2"
"""


# --- The lab -----------------------------------------------------------------


class Lab:
    """One scenario's processes and artifacts. Namespaces are global (PID-suffixed)
    and rebuilt per scenario, so a scenario never inherits another's state (R-6)."""

    def __init__(self, root: Path, ze_bin: Path, scenario: str) -> None:
        self.root = root
        self.ze_bin = ze_bin
        self.scenario = scenario
        parent = root / "tmp" / "evidence"
        parent.mkdir(parents=True, exist_ok=True)
        self.work = Path(
            tempfile.mkdtemp(prefix=f"effective-vrrp-{scenario.lower()}-", dir=parent)
        )
        # keepalived disables a root-run notify script whose path is writable by
        # group/other, and the 9p repo mount (security_model=none) presents the
        # whole /workspace tree that way, so a script under self.work is always
        # rejected ("Unsafe permissions found for script ... disabling"). Keep the
        # script and its marker under /root instead -- root's home is 0700 and
        # off the 9p mount, so the whole path is secure. The lab runs as root in
        # the VM, so /root is available and writable.
        self.secure = Path(
            tempfile.mkdtemp(prefix=f"vrrp-ka-{scenario.lower()}-", dir="/root")
        )
        self.notify = self.secure / "notify.sh"
        self.marker = self.secure / MARKER_NAME
        self.pcap = self.work / "observer.pcap"
        self.ze_proc: subprocess.Popen[str] | None = None
        self.ka_proc: subprocess.Popen[str] | None = None
        self.cap_proc: subprocess.Popen[str] | None = None
        self.pcap_proc: subprocess.Popen[str] | None = None
        self.cap: LineCollector | None = None
        self.ze_log: LineCollector | None = None
        self.ka_mac = ""

    # --- setup / teardown ---
    def setup(self) -> None:
        setup_netns()
        self.ka_mac = link_mac(KA_NS, KA_VETH)
        # Notify script under /root (see __init__): 0700 so no component of the
        # path is group/other-writable, which is what keepalived's
        # enable_script_security check requires before it will run the script.
        self.notify.write_text(NOTIFY_SCRIPT, encoding="utf-8")
        self.notify.chmod(0o700)

    def teardown(self) -> None:
        terminate(self.ka_proc)
        terminate(self.ze_proc)
        terminate(self.cap_proc)
        terminate(self.pcap_proc)
        cleanup_netns()
        # The secure dir is VM-local (/root); remove it so repeated runs in one
        # VM boot do not accumulate. self.work stays for host-side postmortem.
        shutil.rmtree(self.secure, ignore_errors=True)

    # --- capture ---
    def start_capture(self) -> None:
        """Start both captures on the observer's veth and wait until each is live.

        Two tcpdump processes rather than one with `--print -w`: the combined
        form needs tcpdump >= 4.9, and this way the pcap artifact and the line
        parser cannot interfere. The pcap is for post-mortem; every assertion
        reads the parsed lines.

        Waiting for "listening on" before any scenario acts is risk R-5's
        mitigation: a capture that starts late misses the first frames of a
        short GARP burst.
        """
        # ip proto 112 (VRRP), arp (GARP), icmp (VIP reachability). Narrow enough
        # to keep the transcript readable, wide enough for every QS-1..QS-3 claim.
        pcap_filter = "ip proto 112 or arp or icmp"
        self.pcap_proc = ns_popen(
            OB_NS,
            [
                "tcpdump",
                "-i",
                OB_VETH,
                "-n",
                "-s",
                "0",
                "-U",
                "-w",
                str(self.pcap),
                pcap_filter,
            ],
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            text=True,
            bufsize=1,
        )
        assert self.pcap_proc.stderr is not None
        pcap_err = LineCollector("pcap> ", self.pcap_proc.stderr)

        self.cap_proc = ns_popen(
            OB_NS,
            [
                "tcpdump",
                "-i",
                OB_VETH,
                "-n",  # no name resolution: addresses stay assertable
                "-e",  # ether header: the virtual-MAC assertion needs it
                "-vv",  # ttl, vrid, prio, intvl
                "-l",  # line buffered: the LineCollector sees frames live
                "-tt",  # epoch timestamps: every timing band is measured from these
                "-s",
                "0",
                pcap_filter,
            ],
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            text=True,
            bufsize=1,
        )
        assert self.cap_proc.stdout is not None
        assert self.cap_proc.stderr is not None
        self.cap = LineCollector("cap> ", self.cap_proc.stdout)
        cap_err = LineCollector("cap-err> ", self.cap_proc.stderr)

        for collector, proc, what in [
            (cap_err, self.cap_proc, "line capture"),
            (pcap_err, self.pcap_proc, "pcap capture"),
        ]:
            if not collector.wait_for(
                lines_contain("listening on"), CAPTURE_READY_TIMEOUT_S, proc
            ):
                raise RuntimeError(f"tcpdump {what} did not start on {OB_VETH}")

    def capture_lines(self) -> list[str]:
        return self.cap.snapshot() if self.cap is not None else []

    def adverts(self) -> list[Advert]:
        return parse_adverts(self.capture_lines())

    def garps(self) -> list[Garp]:
        return parse_garps(self.capture_lines())

    def ze_adverts(self) -> list[Advert]:
        return [a for a in self.adverts() if a.ip_src == ZE_ADDR]

    def ka_adverts(self) -> list[Advert]:
        return [a for a in self.adverts() if a.ip_src == KA_ADDR]

    def last_ze_advert_ts(self) -> float | None:
        adverts = self.ze_adverts()
        return adverts[-1].ts if adverts else None

    def wait_for_wire_window(self, seconds: float, timeout_s: float) -> None:
        """Observe `seconds` of WIRE time, anchored on advert timestamps.

        A protocol-defined observation window (spec: "the only permitted fixed
        durations are protocol-defined observation windows ... asserted from
        tcpdump wire timestamps, not wall-clock sleeps"). Implemented as a
        predicate poll over the capture's own clock: it returns when the capture
        has advanced `seconds` past the anchor, so a stalled wire fails the
        window rather than passing it by sleeping through it.
        """
        # Wait for the capture to hold at least one advert before anchoring: the
        # capture starts a beat before ze's first advert lands, so reading once
        # here races the very first frame. A wire that never advertises still
        # fails, after the timeout.
        if not wait_until(lambda: bool(self.adverts()), timeout_s):
            raise RuntimeError("no adverts captured to anchor the observation window")
        anchor = self.adverts()[-1].ts

        def advanced() -> bool:
            current = self.adverts()
            return bool(current) and (current[-1].ts - anchor) >= seconds

        if not wait_until(advanced, timeout_s):
            raise RuntimeError(
                f"wire did not advance {seconds:.2f}s within {timeout_s:.0f}s "
                f"(the segment went quiet: adverts stopped)"
            )

    # --- ze ---
    def ze_env(self) -> dict[str, str]:
        env = os.environ.copy()
        # emitStateChange logs at Info (engine.go:321) and slogutil defaults every
        # subsystem to warn (slogutil.go:186-190), so without this the state-change
        # line does not exist. The subsystem name is the plugin's registry name
        # ("vrrp", register.go:40) mapped by CanonicalSubsystemName
        # (component/plugin/inprocess.go:27-29, :105).
        env["ZE_LOG_VRRP"] = "info"
        # Defensive: the vrrp plugin auto-loads in-process (its ConfigRoots is
        # "interface", groups.go:32, and config-path auto-load sets Internal: true,
        # plugin/server/startup_autoload.go:127-136), so its logger writes straight
        # to this process's stderr. Should it ever be forked, its stderr arrives via
        # the relay, which defaults to warn and would swallow the Info line.
        env["ZE_LOG_PLUGIN_RELAY"] = "info"
        env["ZE_STORAGE_BLOB"] = "false"
        env["ZE_CONFIG_DIR"] = str(self.work / "ze")
        return env

    def start_ze(self, conf: str) -> float:
        """Write the config, start ze in its namespace, wait until its virtual
        router is running. Returns the exec instant (epoch seconds).

        The exec instant shares tcpdump's clock: -tt prints CLOCK_REALTIME epoch
        seconds, which is what time.time() reads, so the two are directly
        comparable (used by QS-2's preempt-return band).
        """
        (self.work / "ze").mkdir(parents=True, exist_ok=True)
        conf_path = self.work / "ze.conf"
        conf_path.write_text(conf, encoding="utf-8")

        started = time.time()
        # `start <config>`: the bare `ze <config>` launch form was removed from the
        # CLI (learned 1248), so a positional path now dies with "unknown command".
        self.ze_proc = ns_popen(
            ZE_NS,
            [str(self.ze_bin), "start", str(conf_path)],
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            text=True,
            bufsize=1,
            env=self.ze_env(),
        )
        assert self.ze_proc.stdout is not None
        assert self.ze_proc.stderr is not None
        LineCollector("ze-out> ", self.ze_proc.stdout)
        self.ze_log = LineCollector("ze> ", self.ze_proc.stderr)

        # instance.go:224 logs this at Info the moment the parent is usable and
        # the FSM starts, which is exactly when the master-down timer is armed.
        if not self.ze_log.wait_for(
            lines_contain("vrrp: parent usable, starting virtual router"),
            ZE_START_TIMEOUT_S,
            self.ze_proc,
            fatal_ze,
        ):
            raise RuntimeError("ze did not start its virtual router")
        return started

    def wait_ze_state(self, state: str, timeout_s: float = ZE_MASTER_TIMEOUT_S) -> None:
        if self.ze_log is None:
            raise RuntimeError("ze is not running")
        if not self.ze_log.wait_for(
            ze_state_is(state), timeout_s, self.ze_proc, fatal_ze
        ):
            raise RuntimeError(f"ze did not reach {state} state")

    def kill_ze_node_death(self) -> None:
        """Node death: SIGKILL plus carrier loss (spec Design Insights, risk R-4).

        SIGKILL alone is not a faithful failover trigger: the dead process leaves
        its macvlan and the installed VIP behind, so the namespace keeps answering
        ARP for the VIP. Dropping the parent's carrier is the cable-pull half.
        """
        if self.ze_proc is not None and self.ze_proc.poll() is None:
            self.ze_proc.send_signal(signal.SIGKILL)
            self.ze_proc.wait(timeout=5.0)
        ns_run_required(
            ZE_NS, ["ip", "link", "set", ZE_VETH, "down"], f"down {ZE_VETH}"
        )

    def revive_ze_link(self) -> None:
        ns_run_required(ZE_NS, ["ip", "link", "set", ZE_VETH, "up"], f"up {ZE_VETH}")

    def stop_ze_graceful(self) -> None:
        """Graceful stop: SIGTERM, which must produce the Priority-0 advert.

        RFC 9568 Section 6.4.3: an Active router sends an advert with Priority 0
        on shutdown. In ze that is runVRRPEngine's `defer eng.stopAll()`
        (register.go:132-133) -> instance.shutdown() -> FSM Shutdown ->
        SendAdvertZeroPriority (instance.go:306-311), executed while the transport
        sockets are still open (engine.go:138-144).
        """
        if self.ze_proc is None:
            return
        self.ze_proc.send_signal(signal.SIGTERM)

    # --- keepalived ---
    def keepalived_version(self) -> str:
        result = ns_run(
            KA_NS, ["keepalived", "-v"], stdout=subprocess.PIPE, stderr=subprocess.PIPE
        )
        text = ((result.stdout or "") + (result.stderr or "")).strip()
        return text.splitlines()[0] if text else "unknown"

    def start_keepalived(self, conf: str) -> None:
        """Validate the config, then run keepalived in the foreground in its netns.

        `keepalived -t` is the config gate demanded by risk R-3, and it runs
        INSIDE the namespace because the config names KA_VETH, which only exists
        there. A parse failure reports the keepalived version, so a keyword drift
        across Alpine releases is identifiable from one run's output.
        """
        conf_path = self.work / "keepalived.conf"
        conf_path.write_text(conf, encoding="utf-8")

        check = ns_run(
            KA_NS,
            ["keepalived", "-t", "-f", str(conf_path)],
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
        )
        if check.returncode != 0:
            sys.stderr.write((check.stdout or "") + (check.stderr or ""))
            raise RuntimeError(
                f"keepalived rejected the generated config "
                f"({self.keepalived_version()}); see {conf_path}"
            )

        self.ka_proc = ns_popen(
            KA_NS,
            [
                "keepalived",
                "-n",  # foreground: the process is ours to supervise and reap
                "-l",  # log to console, so its stderr lands in the transcript
                "-D",  # detailed log (diagnostics only; nothing is asserted on it)
                "-P",  # VRRP subsystem only: no healthchecker in this lab
                "-f",
                str(conf_path),
                "-p",
                str(self.work / "keepalived.pid"),
            ],
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            text=True,
            bufsize=1,
        )
        assert self.ka_proc.stdout is not None
        assert self.ka_proc.stderr is not None
        LineCollector("ka-out> ", self.ka_proc.stdout)
        LineCollector("ka> ", self.ka_proc.stderr)

    def ka_states(self) -> list[str]:
        """Every state keepalived has notified, oldest first."""
        if not self.marker.is_file():
            return []
        text = self.marker.read_text(encoding="utf-8", errors="replace")
        return [line.strip() for line in text.splitlines() if line.strip()]

    def ka_state(self) -> str:
        states = self.ka_states()
        return states[-1] if states else ""

    def wait_ka_state(self, state: str, timeout_s: float = KA_STATE_TIMEOUT_S) -> None:
        """Wait for keepalived's notify marker to report `state` (assumption A-4)."""
        if not wait_until(lambda: self.ka_state() == state, timeout_s):
            raise RuntimeError(
                f"keepalived did not reach {state}: markers so far "
                f"{self.ka_states() or '(none: no notify script ran)'}"
            )

    # --- diagnostics ---
    def dump_diagnostics(self) -> None:
        sys.stderr.write("\n--- diagnostics ---\n")
        if self.ze_log is not None:
            sys.stderr.write("ze log tail:\n")
            sys.stderr.write("".join(self.ze_log.snapshot()[-80:]))
        sys.stderr.write(f"\nkeepalived version: {self.keepalived_version()}\n")
        sys.stderr.write(f"keepalived state markers: {self.ka_states()}\n")
        # Lab namespaces only: a failure dump never reports host-wide state
        # (spec Security Review Checklist, "Error leakage").
        for ns_name in [ZE_NS, KA_NS, OB_NS]:
            sys.stderr.write(
                f"\n{ns_name} links:\n{ns_output(ns_name, ['ip', 'addr'])}"
            )
            sys.stderr.write(
                f"{ns_name} neigh:\n{ns_output(ns_name, ['ip', 'neigh', 'show'])}"
            )
        # ze's virtual-MAC dataplane: the macvlan mode (must be private) and the
        # ARP recipe sysctls that make the virtual MAC the sole responder for the
        # VIP (spec-vrrp-6). Surfaced on failure so a broken recipe is diagnosable
        # without another run.
        sys.stderr.write(
            f"\n{ZE_NS} macvlan detail:\n{ns_output(ZE_NS, ['ip', '-d', 'link', 'show'])}"
        )
        sys.stderr.write(
            f"{ZE_NS} dataplane sysctls:\n"
            + ns_output(
                ZE_NS,
                [
                    "sh",
                    "-c",
                    "grep -H . /proc/sys/net/ipv4/conf/*/arp_ignore "
                    "/proc/sys/net/ipv4/conf/*/arp_filter "
                    "/proc/sys/net/ipv4/conf/*/rp_filter "
                    "/proc/sys/net/ipv6/conf/*/disable_ipv6 2>/dev/null "
                    "| grep -vE '/(default|lo)/'",
                ],
            )
        )
        sys.stderr.write(
            f"\n{LAN_NS} bridge:\n{ns_output(LAN_NS, ['ip', 'link', 'show', 'master', BRIDGE])}"
        )
        sys.stderr.write("\ncapture tail:\n")
        sys.stderr.write("".join(self.capture_lines()[-60:]))
        sys.stderr.write(f"\nartifacts kept in: {self.work}\n")


def link_mac(ns: str, dev: str) -> str:
    """The device's real MAC, used to tell keepalived's frames from ze's.

    keepalived runs in its default mode (no use_vmac), so it sources adverts and
    GARPs from this address, while ze sources from the virtual MAC. That contrast
    is what makes "only ze-sourced adverts" assertable (spec Known Limitations).
    """
    out = ns_run(
        ns,
        ["ip", "-j", "link", "show", dev],
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
    )
    if out.returncode != 0:
        raise RuntimeError(f"ip -j link show {dev} failed in {ns}")
    try:
        data = json.loads(out.stdout or "[]")
        return str(data[0]["address"])
    except (json.JSONDecodeError, IndexError, KeyError) as err:
        raise RuntimeError(f"cannot read {dev} MAC in {ns}: {err}") from err


def neigh_lladdr(ns: str, dev: str, dst: str) -> str:
    """The link-layer address `dst` currently resolves to in ns's neighbour cache."""
    out = ns_run(
        ns,
        ["ip", "-j", "neigh", "show", "dev", dev],
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
    )
    if out.returncode != 0:
        return ""
    try:
        for entry in json.loads(out.stdout or "[]"):
            if entry.get("dst") == dst:
                return str(entry.get("lladdr", ""))
    except json.JSONDecodeError:
        return ""
    return ""


def ping_vip(ns: str, target: str = VIP) -> bool:
    result = ns_run(
        ns,
        ["ping", "-c", "2", "-W", "3", target],
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
    )
    return result.returncode == 0


# --- Assertions --------------------------------------------------------------


def assert_ze_advert_fields(lab: Lab) -> None:
    """AC-1's wire claims about ze's adverts."""
    # Poll for the first advert: the capture can lag ze's first frame by a beat,
    # so a single read races it. A genuinely silent ze still fails after timeout.
    if not wait_until(lambda: bool(lab.ze_adverts()), WIRE_EVENT_TIMEOUT_S):
        raise RuntimeError(
            "no ze-sourced VRRP advert captured on the observer's segment:\n"
            + "".join(lab.capture_lines()[-40:])
        )
    adverts = lab.ze_adverts()
    for advert in adverts:
        problems: list[str] = []
        if advert.version != VRRP_VERSION_V3:
            # RFC 9568 Section 5.2.1: version nibble 3.
            problems.append(f"version {advert.version} != {VRRP_VERSION_V3}")
        if advert.kind != VRRP_TYPE_ADVERTISEMENT:
            # RFC 9568 Section 5.2.2: type 1 is the only defined type.
            problems.append(f"type {advert.kind} != {VRRP_TYPE_ADVERTISEMENT}")
        if advert.vrid != VRID:
            problems.append(f"vrid {advert.vrid} != {VRID}")
        if advert.priority != ZE_PRIORITY:
            problems.append(f"prio {advert.priority} != {ZE_PRIORITY}")
        if advert.ttl != ADVERT_TTL:
            # RFC 9568 Section 5.1.1.3.
            problems.append(f"ttl {advert.ttl} != {ADVERT_TTL}")
        if advert.ether_src != VIRTUAL_MAC:
            # RFC 9568 Section 7.3.
            problems.append(f"ether src {advert.ether_src} != {VIRTUAL_MAC}")
        if advert.ip_dst != VRRP_MCAST_V4:
            # RFC 9568 Section 5.1.1.2.
            problems.append(f"ip dst {advert.ip_dst} != {VRRP_MCAST_V4}")
        if advert.interval != ADVERT_CS or advert.interval_unit != "cs":
            # RFC 9568 Section 5.2.7 + spec Boundary Tests: 1000 ms -> 100 cs.
            problems.append(
                f"intvl {advert.interval}{advert.interval_unit} != {ADVERT_CS}cs"
            )
        if problems:
            raise RuntimeError(
                "ze advert violates RFC 9568: "
                + "; ".join(problems)
                + f"\nrecord: {advert.raw}"
            )
    print(
        f"  wire: {len(adverts)} ze adverts, all VRRPv3 type 1 vrid {VRID} "
        f"prio {ZE_PRIORITY} intvl {ADVERT_CS}cs ttl {ADVERT_TTL} "
        f"ether-src {VIRTUAL_MAC} dst {VRRP_MCAST_V4}"
    )


def assert_no_ka_adverts(lab: Lab, since: float = 0.0) -> None:
    """AC-1: while ze is Active, keepalived must be silent on the wire."""
    intruders = [a for a in lab.ka_adverts() if a.ts >= since]
    if intruders:
        raise RuntimeError(
            f"keepalived sent {len(intruders)} advert(s) while ze was Active "
            f"(it should stay Backup):\n" + "\n".join(a.raw for a in intruders[:5])
        )


def assert_vip_resolves_to_virtual_mac(lab: Lab) -> None:
    """AC-1/AC-2: an independent third party pings the VIP and resolves it to the
    virtual MAC.

    Each poll flushes the observer's neighbour cache and re-resolves, rather than
    resolving once and re-reading: the VERY FIRST cold ARP for the VIP after a
    flush can lose an ARP-flux race and cache the parent's real MAC, but every
    resolution after that returns the virtual MAC. This is inherent Linux
    behaviour with a macvlan whose parent shares the subnet -- keepalived's own
    use_vmac shows the identical first-resolution race and converges the same way
    (proven in QEMU, plan/spec-vrrp-6). Re-resolving is also what a real gateway
    client does: it caches the answer, and its next resolution (on entry expiry,
    or a gratuitous ARP updating the entry) lands on the virtual MAC. The poll
    asserts the convergent, steady-state property the RFC promises.
    """
    del lab  # the claim is about the observer's kernel, not the capture

    def observer_resolves_vmac() -> bool:
        ns_run(
            OB_NS,
            ["ip", "neigh", "flush", "all"],
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
        )
        if not ping_vip(OB_NS):
            return False
        return neigh_lladdr(OB_NS, OB_VETH, VIP) == VIRTUAL_MAC

    if not wait_until(observer_resolves_vmac, PING_TIMEOUT_S):
        lladdr = neigh_lladdr(OB_NS, OB_VETH, VIP)
        raise RuntimeError(
            f"observer resolves {VIP} to {lladdr or '(nothing)'}, expected the "
            f"virtual MAC {VIRTUAL_MAC} (RFC 9568 Section 7.3)"
        )
    print(f"  dataplane: observer pings {VIP}, ARP resolves to {VIRTUAL_MAC}")


def establish_ze_master(lab: Lab) -> None:
    """The shared precondition of QS-1/QS-2/QS-3: ze Active, keepalived Backup.

    Order matters: ze must own the VIP before keepalived starts, otherwise
    keepalived (state BACKUP, prio 100) would promote after its own master-down
    and the scenario would be testing a different election.
    """
    lab.start_capture()
    lab.start_ze(ze_conf_v3_ipv4(ZE_PRIORITY))
    # ze starts in Backup and promotes after its master-down (RFC 9568 Section
    # 6.4.1: a non-owner starts Backup), since the segment is silent.
    lab.wait_ze_state("master")
    lab.start_keepalived(keepalived_conf_v3_ipv4(lab.notify, lab.marker, KA_PRIORITY))
    lab.wait_ka_state("BACKUP")


# --- Scenarios ---------------------------------------------------------------


def scenario_qs1(lab: Lab) -> None:
    """QS-1 / AC-1: v3 IPv4 election. ze prio 200 vs keepalived prio 100."""
    establish_ze_master(lab)

    # Settled, not merely "not yet promoted": observe more than keepalived's own
    # master-down interval (3.609 s) of wire time and require it to still be
    # Backup. Without this, a keepalived that was simply slow to promote would
    # pass. This is the interop contract's "re-verify stability afterward" step.
    lab.wait_for_wire_window(KA_MASTER_DOWN_S + 1.0, WIRE_EVENT_TIMEOUT_S)
    if lab.ka_state() != "BACKUP":
        raise RuntimeError(
            f"keepalived did not settle BACKUP: markers {lab.ka_states()}"
        )

    assert_ze_advert_fields(lab)
    assert_no_ka_adverts(lab)
    assert_vip_resolves_to_virtual_mac(lab)
    print(
        f"  state: keepalived settled BACKUP across {KA_MASTER_DOWN_S + 1.0:.2f}s "
        f"of wire time (> its {KA_MASTER_DOWN_S:.3f}s master-down)"
    )


def scenario_qs2(lab: Lab) -> None:
    """QS-2 / AC-2: node death, keepalived promotion, then ze preempt return."""
    establish_ze_master(lab)
    assert_ze_advert_fields(lab)

    # --- Phase A: node death ---
    last_ze = lab.last_ze_advert_ts()
    if last_ze is None:
        raise RuntimeError("no ze advert to anchor the failover measurement")
    lab.kill_ze_node_death()

    lab.wait_ka_state("MASTER")

    # The MASTER notify marker can be written a beat before keepalived's first
    # promotion advert reaches the observer's capture; poll instead of reading
    # once. A genuine no-advert still fails after the timeout.
    if not wait_until(
        lambda: any(a.ts > last_ze for a in lab.ka_adverts()), WIRE_EVENT_TIMEOUT_S
    ):
        raise RuntimeError(
            "keepalived reported MASTER but sent no advert of its own "
            "(promotion not visible on the wire)"
        )
    promotions = [a for a in lab.ka_adverts() if a.ts > last_ze]
    first = promotions[0]
    delta = first.ts - last_ze
    # Spec Boundary Tests: below 3.0 s means keepalived never timed against ze's
    # adverts; above 6.0 s means its 3.609 s master-down plus TCG margin blew.
    if not QS2_PROMOTE_MIN_S <= delta <= QS2_PROMOTE_MAX_S:
        raise RuntimeError(
            f"keepalived promoted {delta:.3f}s after ze's last advert, outside the "
            f"[{QS2_PROMOTE_MIN_S}, {QS2_PROMOTE_MAX_S}]s band "
            f"(its master-down is {KA_MASTER_DOWN_S:.3f}s)"
        )
    if first.priority != KA_PRIORITY:
        raise RuntimeError(
            f"keepalived advertised prio {first.priority}, expected {KA_PRIORITY}"
        )
    print(
        f"  failover: keepalived promoted {delta:.3f}s after ze's last advert "
        f"(band [{QS2_PROMOTE_MIN_S}, {QS2_PROMOTE_MAX_S}]s), advertising prio "
        f"{first.priority} from {first.ether_src}"
    )

    # keepalived announces the VIP move with its own GARPs, from its REAL MAC
    # (no use_vmac in this lab). garp_master_delay defaults to 5 s.
    if not wait_until(
        lambda: any(
            g.ts > last_ze and g.sender_ip == VIP and g.ether_src == lab.ka_mac
            for g in lab.garps()
        ),
        KA_GARP_TIMEOUT_S,
    ):
        raise RuntimeError(
            f"keepalived sent no gratuitous ARP for {VIP} from its own MAC "
            f"{lab.ka_mac} after promoting"
        )
    print(f"  failover: keepalived sent gratuitous ARP for {VIP} from {lab.ka_mac}")

    # --- Phase B: fresh ze restart, preempt return ---
    lab.revive_ze_link()
    restart = lab.start_ze(ze_conf_v3_ipv4(ZE_PRIORITY))
    lab.wait_ze_state("master")

    # Poll for the advert: wait_ze_state returns on the LOG marker, and the first
    # advert reaches the capture's LineCollector a moment later. A one-shot read
    # here raced that gap and reported "no advert" even though ze sent one (seen
    # in the capture at the master-transition timestamp). WIRE_EVENT_TIMEOUT_S
    # bounds the wait; a genuine no-advert still fails, just not spuriously.
    if not wait_until(
        lambda: any(a.ts > restart for a in lab.ze_adverts()),
        WIRE_EVENT_TIMEOUT_S,
    ):
        raise RuntimeError("restarted ze reached master but sent no advert")
    returns = [a for a in lab.ze_adverts() if a.ts > restart]
    first_return = returns[0]
    preempt_delta = first_return.ts - restart
    # Spec Boundary Tests row "QS-2 ze preempt-return promotion delta".
    #
    # Anchor: the ze exec instant, not "first advert received", which the spec
    # phrases as "from ze restart advert-rx start". The two differ by ze's
    # startup latency, and only the exec anchor is safe in both directions: it
    # PRECEDES the FSM's master-down arm, so the measured delta is always >= the
    # true one, meaning the load-bearing lower bound (ze must not preempt before
    # waiting out its own 3.219 s master-down) can only fail on a real violation.
    # tcpdump -tt and time.time() read the same CLOCK_REALTIME, so the subtraction
    # is sound.
    if not QS2_PREEMPT_MIN_S <= preempt_delta <= QS2_PREEMPT_MAX_S:
        raise RuntimeError(
            f"ze preempted {preempt_delta:.3f}s after restart, outside the "
            f"[{QS2_PREEMPT_MIN_S}, {QS2_PREEMPT_MAX_S}]s band (its own master-down "
            f"is {ZE_MASTER_DOWN_S:.3f}s; the delta includes ze's startup latency)"
        )

    # RFC 9568 Section 8.1.2 + errata 7947/7949: on promotion the new Active
    # broadcasts a gratuitous ARP per VIP with sender IP == target IP == VIP and
    # the Virtual Router MAC as both the Ethernet source and the sha/tha.
    if not wait_until(
        lambda: any(
            g.ts > restart and g.sender_ip == VIP and g.ether_src == VIRTUAL_MAC
            for g in lab.garps()
        ),
        WIRE_EVENT_TIMEOUT_S,
    ):
        raise RuntimeError(
            f"restarted ze sent no gratuitous ARP for {VIP} from {VIRTUAL_MAC}"
        )
    burst = [
        g
        for g in lab.garps()
        if g.ts > restart and g.sender_ip == VIP and g.ether_src == VIRTUAL_MAC
    ]
    for garp in burst:
        if garp.target_ip != VIP or garp.sender_ip != VIP:
            raise RuntimeError(
                f"ze GARP is not gratuitous: sender {garp.sender_ip} target "
                f"{garp.target_ip}, both must be {VIP}\nrecord: {garp.raw}"
            )
        if garp.target_mac is not None and garp.target_mac != VIRTUAL_MAC:
            raise RuntimeError(
                f"ze GARP target link-layer {garp.target_mac} != {VIRTUAL_MAC} "
                f"(RFC 9568 errata 7947/7949)\nrecord: {garp.raw}"
            )
    print(
        f"  preempt: ze promoted {preempt_delta:.3f}s after restart "
        f"(band [{QS2_PREEMPT_MIN_S}, {QS2_PREEMPT_MAX_S}]s), GARP burst of "
        f"{len(burst)} frame(s), sender IP == target IP == {VIP}, MAC {VIRTUAL_MAC}"
    )

    lab.wait_ka_state("BACKUP")
    # Only now is the neigh cache checked: asserting the repoint before the GARP
    # is observed would be racing the announcement (risk R-4).
    assert_vip_resolves_to_virtual_mac(lab)
    print("  preempt: keepalived returned to BACKUP, observer repointed to the VIP")


def scenario_qs3(lab: Lab) -> None:
    """QS-3 / AC-3: graceful stop of the Active ze takes keepalived's skew path."""
    establish_ze_master(lab)
    assert_ze_advert_fields(lab)

    lab.stop_ze_graceful()

    # RFC 9568 Section 6.4.3: the resigning Active sends Priority 0.
    if not wait_until(
        lambda: any(a.priority == 0 for a in lab.ze_adverts()), WIRE_EVENT_TIMEOUT_S
    ):
        raise RuntimeError(
            "ze sent no Priority-0 advert on SIGTERM (RFC 9568 Section 6.4.3): "
            "the graceful-resignation path did not run"
        )
    prio0 = [a for a in lab.ze_adverts() if a.priority == 0][0]
    print(f"  wire: ze sent the Priority-0 resignation advert at {prio0.ts:.6f}")

    lab.wait_ka_state("MASTER")

    # The MASTER notify marker is written by keepalived's notify script, which can
    # fire a beat before its first advert reaches the observer's capture. Poll for
    # the advert rather than reading once, matching the Priority-0 poll above: a
    # genuine no-advert still fails after the timeout.
    if not wait_until(
        lambda: any(a.ts >= prio0.ts for a in lab.ka_adverts()), WIRE_EVENT_TIMEOUT_S
    ):
        raise RuntimeError(
            "keepalived reported MASTER but sent no advert after ze's Priority-0"
        )
    promotions = [a for a in lab.ka_adverts() if a.ts >= prio0.ts]
    delta = promotions[0].ts - prio0.ts
    # Spec Boundary Tests: the skew path is <= 3.0 s. RFC 9568 Section 6.4.2: on
    # a Priority-0 advert a Backup arms master-down with Skew_Time (0.609 s at
    # prio 100), not Active_Down_Interval (3.609 s). A delta at or above 3.61 s
    # would prove the prio-0 advert was ignored and the full master-down ran,
    # which is exactly the distinction this scenario exists to make.
    if delta > QS3_PRIO0_PROMOTE_MAX_S:
        detail = (
            f" (>= {QS3_NO_SKEW_PATH_S}s means the skew path was NOT taken: "
            f"keepalived timed out its full {KA_MASTER_DOWN_S:.3f}s master-down "
            f"instead of reacting to the Priority-0 advert)"
            if delta >= QS3_NO_SKEW_PATH_S
            else ""
        )
        raise RuntimeError(
            f"keepalived promoted {delta:.3f}s after ze's Priority-0 advert, "
            f"above the {QS3_PRIO0_PROMOTE_MAX_S}s skew-path bound{detail}"
        )
    print(
        f"  prio-0: keepalived promoted {delta:.3f}s after the resignation "
        f"(bound {QS3_PRIO0_PROMOTE_MAX_S}s; its skew is {KA_SKEW_S:.3f}s, its "
        f"full master-down {KA_MASTER_DOWN_S:.3f}s), proving the Skew_Time path"
    )


# Implemented scenarios. QS-4..QS-8 slot in here as they land: each is a
# `def scenario_qsN(lab: Lab) -> None` that raises RuntimeError on failure and
# lets main() own setup, teardown, and the PASS marker.
SCENARIOS: dict[str, tuple[Callable[[Lab], None], str]] = {
    "QS-1": (
        scenario_qs1,
        "v3 IPv4 election: ze prio 200 vs keepalived prio 100 (AC-1)",
    ),
    "QS-2": (scenario_qs2, "node-death failover and ze preempt return (AC-2)"),
    "QS-3": (scenario_qs3, "graceful stop: Priority-0 skew path (AC-3)"),
}

# Declared by spec-vrrp-6 but NOT implemented here. This script's committed scope
# is the core election/failover/prio-0 proofs; the rest stay listed so the gap is
# visible rather than forgotten (they are still open work against AC-4..AC-8).
PENDING_SCENARIOS: dict[str, str] = {
    "QS-4": "reverse mastership and preempt false (AC-4): NOT IMPLEMENTED",
    "QS-5": "v2 opt-in wire format vs default keepalived (AC-5): NOT IMPLEMENTED",
    "QS-6": "IPv6 v3: link-local plus global VIP, unsolicited NA (AC-6): NOT IMPLEMENTED",
    "QS-7": "duplicate-VRID tie-break by greater primary IP (AC-7): NOT IMPLEMENTED",
    "QS-8": "ze vs ze election (AC-8): NOT IMPLEMENTED",
}


def print_scenarios() -> None:
    for name, (_, desc) in SCENARIOS.items():
        print(f"{name}\t{desc}")
    for name, desc in PENDING_SCENARIOS.items():
        print(f"{name}\t{desc}")


def usage() -> str:
    return (
        "usage: effective-vrrp-keepalived.py [--list] [QS-1 QS-2 ...]\n"
        "  --list   print every scenario (implemented and pending)\n"
        "  QS-N     run only the named scenarios (default: all implemented)"
    )


def main() -> int:
    args = sys.argv[1:]
    if "--help" in args or "-h" in args:
        print(usage())
        return 0
    if "--list" in args:
        print_scenarios()
        return 0

    selected = [a for a in args if not a.startswith("-")] or list(SCENARIOS)
    unknown = [name for name in selected if name not in SCENARIOS]
    if unknown:
        pending = [name for name in unknown if name in PENDING_SCENARIOS]
        if pending:
            raise SystemExit(
                f"scenario(s) not implemented in this script: {', '.join(pending)}\n"
                + usage()
            )
        raise SystemExit(f"unknown scenario(s): {', '.join(unknown)}\n" + usage())

    if platform.system() != "Linux":
        raise SystemExit("VRRP keepalived interop evidence requires Linux")
    require_cmd("ip")
    require_cmd("ping")
    require_cmd("tcpdump")
    require_cmd("keepalived")
    ensure_kernel_support()

    root = repo_root()
    ze = ensure_ze(root)

    failures: list[str] = []
    for name in selected:
        scenario, description = SCENARIOS[name]
        print(f"\n=== {name}: {description} ===", flush=True)
        lab = Lab(root, ze, name)
        success = False
        try:
            lab.setup()
            scenario(lab)
            print(f"PASS: {name} {description}", flush=True)
            success = True
        except RuntimeError as err:
            sys.stderr.write(f"FAIL: {name}: {err}\n")
            lab.dump_diagnostics()
            failures.append(name)
        finally:
            lab.teardown()
            if success:
                shutil.rmtree(lab.work, ignore_errors=True)

    if failures:
        sys.stderr.write(
            f"\nFAIL: {len(failures)} scenario(s): {', '.join(failures)}\n"
        )
        return 1
    print(
        f"\nOK: ze VRRP interoperates with keepalived across {len(selected)} "
        f"scenario(s): {', '.join(selected)}"
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
