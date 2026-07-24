#!/usr/bin/env python3
"""Run a real xl2tpd/pppd LAC against a gokrazy Ze appliance VM."""

from __future__ import annotations

import json
import os
import platform
import re
import shutil
import signal
import stat
import subprocess
import sys
import tempfile
import threading
import time
from pathlib import Path
from typing import Callable


LOCAL_ADDR = "10.100.0.1"
PEER_ADDR = "10.100.0.2"
ZE_PACKAGE = "codeberg.org/thomas-mangin/ze/cmd/ze"
PROOF_ZE_ENV = [
    "ze.l2tp.ncp.enable-ipv6cp=false",
    "ze.l2tp.ncp.ip-timeout=15s",
    "ze.l2tp.auth.timeout=15s",
]
HOST_UNDERLAY_IP = os.environ.get("ZE_GOKRAZY_L2TP_HOST_IP", "172.31.0.1")
LAC_UNDERLAY_IP = os.environ.get("ZE_GOKRAZY_L2TP_LAC_IP", "172.31.0.2")
UNDERLAY_PREFIX = os.environ.get("ZE_GOKRAZY_L2TP_PREFIX", "24")
L2TP_HOST_PORT = os.environ.get("ZE_GOKRAZY_L2TP_HOST_PORT", "1701")
XL2TPD_SOURCE_PORT = os.environ.get("ZE_GOKRAZY_L2TP_XL2TPD_PORT", "1702")
WEB_HOST_PORT = os.environ.get("ZE_GOKRAZY_WEB_HOST_PORT", "28080")
SSH_HOST_PORT = os.environ.get("ZE_GOKRAZY_SSH_HOST_PORT", "2222")
ARCH = os.environ.get("ZE_GOKRAZY_ARCH") or os.environ.get("GOKRAZY_ARCH") or "amd64"
# No darwin/hvf branch here, unlike the sibling QEMU evidence scripts: this
# harness drives the LAC through Linux network namespaces (LAC_NS below), so it
# is Linux-only by construction. os.access, not Path.exists: /dev/kvm is
# root:kvm 0660, so a user outside the kvm group sees the node while qemu cannot
# open it ("Could not access KVM kernel module: Permission denied") and never
# falls back to tcg.
QEMU_ACCEL = os.environ.get("ZE_GOKRAZY_QEMU_ACCEL") or (
    "kvm" if os.access("/dev/kvm", os.R_OK | os.W_OK) else "tcg"
)
NS_SUFFIX = str(os.getpid())
VETH_SUFFIX = NS_SUFFIX[-6:]
LAC_NS = f"ze-gokrazy-lac-{NS_SUFFIX}"
VETH_HOST = f"zgokh{VETH_SUFFIX}"
VETH_LAC = f"zgokl{VETH_SUFFIX}"
# TAP + bridge underlay. qemu user-mode (slirp) networking does NOT deliver
# inbound UDP hostfwd to the guest, which L2TP's SCCRQ requires, so the appliance
# attaches to a bridge via a TAP and is L2-reachable from the LAC with no NAT.
BRIDGE = f"zebr{VETH_SUFFIX}"
TAP = f"zetap{VETH_SUFFIX}"
APPLIANCE_IP = os.environ.get("ZE_GOKRAZY_L2TP_APPLIANCE_IP", "172.31.0.10")
APPLIANCE_MAC = os.environ.get("ZE_GOKRAZY_L2TP_APPLIANCE_MAC", "52:54:00:12:34:56")
DNSMASQ_PID = f"/run/ze-l2tp-dnsmasq-{NS_SUFFIX}.pid"
DNSMASQ_LEASES = f"/run/ze-l2tp-dnsmasq-{NS_SUFFIX}.leases"
DNSMASQ_LOG = f"/run/ze-l2tp-dnsmasq-{NS_SUFFIX}.log"


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


def ns_run(cmd: list[str], **kwargs) -> subprocess.CompletedProcess[str]:
    return run(["ip", "netns", "exec", LAC_NS, *cmd], **kwargs)


def ns_run_required(
    cmd: list[str], context: str, **kwargs
) -> subprocess.CompletedProcess[str]:
    return run_required(["ip", "netns", "exec", LAC_NS, *cmd], context, **kwargs)


def ns_popen(cmd: list[str], **kwargs) -> subprocess.Popen[str]:
    return subprocess.Popen(["ip", "netns", "exec", LAC_NS, *cmd], **kwargs)


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
        return bool(cap_eff & (1 << 12))
    return False


def try_load_modules() -> None:
    modprobe = shutil.which("modprobe")
    if modprobe is None or os.geteuid() != 0:
        return
    for mod in ["ppp_generic", "l2tp_core", "l2tp_netlink", "pppox", "l2tp_ppp"]:
        run([modprobe, mod], stdout=subprocess.PIPE, stderr=subprocess.PIPE)


def ensure_host_kernel_support() -> None:
    if platform.system() != "Linux":
        raise SystemExit("gokrazy L2TP PPP appliance evidence requires Linux")
    if not has_cap_net_admin():
        raise SystemExit(
            "gokrazy L2TP PPP appliance evidence requires root or CAP_NET_ADMIN"
        )

    dev_ppp = Path("/dev/ppp")
    if not dev_ppp.exists():
        raise SystemExit("missing /dev/ppp")
    if not stat.S_ISCHR(dev_ppp.stat().st_mode):
        raise SystemExit("/dev/ppp exists but is not a character device")

    try_load_modules()
    if not (
        Path("/proc/net/pppol2tp").exists()
        or Path("/sys/module/l2tp_ppp").exists()
        or Path("/sys/module/pppol2tp").exists()
    ):
        raise SystemExit("missing host PPPoL2TP support for the real LAC peer")

    genl = run(
        ["ip", "l2tp", "show", "tunnel"], stdout=subprocess.PIPE, stderr=subprocess.PIPE
    )
    if genl.returncode != 0:
        sys.stderr.write((genl.stdout or "") + (genl.stderr or ""))
        raise SystemExit("ip l2tp cannot access the host L2TP Generic Netlink family")


def kill_netns_processes(sig: signal.Signals) -> None:
    pids = run(
        ["ip", "netns", "pids", LAC_NS], stdout=subprocess.PIPE, stderr=subprocess.PIPE
    )
    if pids.returncode != 0:
        return
    for raw in (pids.stdout or "").split():
        try:
            os.kill(int(raw), sig)
        except (ValueError, ProcessLookupError, PermissionError):
            pass


def ufw_active() -> bool:
    if shutil.which("ufw") is None:
        return False
    r = run(["ufw", "status"], stdout=subprocess.PIPE, stderr=subprocess.PIPE)
    return r.returncode == 0 and "Status: active" in (r.stdout or "")


def allow_bridge_firewall() -> None:
    # ufw's default-deny INPUT drops the appliance's DHCP DISCOVER (host-terminated
    # by dnsmasq) before dnsmasq can see it. Punch a scoped hole for the bridge.
    # Bridged L2TP traffic (LAC<->appliance) bypasses netfilter (br_netfilter is
    # not loaded) and needs no rule.
    if not ufw_active():
        return
    run(
        ["ufw", "allow", "in", "on", BRIDGE],
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
    )


def clear_bridge_firewall() -> None:
    if shutil.which("ufw") is None:
        return
    run(
        ["ufw", "--force", "delete", "allow", "in", "on", BRIDGE],
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
    )


def stop_dnsmasq() -> None:
    try:
        pid = int(Path(DNSMASQ_PID).read_text(encoding="utf-8").strip())
        os.kill(pid, signal.SIGTERM)
    except (FileNotFoundError, ValueError, ProcessLookupError, PermissionError):
        pass
    Path(DNSMASQ_PID).unlink(missing_ok=True)
    Path(DNSMASQ_LEASES).unlink(missing_ok=True)


def start_dnsmasq() -> None:
    Path(DNSMASQ_PID).unlink(missing_ok=True)
    Path(DNSMASQ_LEASES).unlink(missing_ok=True)
    run_required(
        [
            "dnsmasq",
            f"--interface={BRIDGE}",
            # NOT --bind-interfaces: that binds the DHCP socket to the interface's
            # unicast address and cannot receive the broadcast DISCOVER. --port=0
            # disables DNS so there is no wildcard :53 clash with host resolvers.
            "--port=0",  # DHCP only, no DNS (avoid clashing with host resolvers)
            "--no-resolv",
            "--no-hosts",
            f"--dhcp-range={APPLIANCE_IP},{APPLIANCE_IP},255.255.255.0,2m",
            f"--dhcp-host={APPLIANCE_MAC},{APPLIANCE_IP}",
            "--log-dhcp",
            f"--log-facility={DNSMASQ_LOG}",
            f"--pid-file={DNSMASQ_PID}",
            f"--dhcp-leasefile={DNSMASQ_LEASES}",
        ],
        "start dnsmasq for appliance DHCP",
    )


def cleanup_netns() -> None:
    kill_netns_processes(signal.SIGTERM)
    time.sleep(0.2)
    kill_netns_processes(signal.SIGKILL)
    stop_dnsmasq()
    clear_bridge_firewall()
    for link in [TAP, VETH_HOST, VETH_LAC, BRIDGE]:
        run(
            ["ip", "link", "delete", link],
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
        )
    run(
        ["ip", "netns", "delete", LAC_NS],
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
    )


def setup_lac_netns() -> None:
    cleanup_netns()
    Path("/run/netns").mkdir(parents=True, exist_ok=True)
    run_required(["ip", "netns", "add", LAC_NS], f"create netns {LAC_NS}")

    # Underlay bridge (root netns): the appliance TAP and the LAC veth both attach
    # here, so the appliance is on the same L2 segment as the LAC with no NAT.
    # STP off + zero forward delay so newly-enslaved ports forward immediately;
    # otherwise the bridge drops the appliance's early DHCP DISCOVER.
    run_required(
        ["ip", "link", "add", "name", BRIDGE, "type", "bridge", "forward_delay", "0"],
        "create bridge",
    )
    run(
        ["ip", "link", "set", BRIDGE, "type", "bridge", "stp_state", "0"],
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
    )
    run_required(
        ["ip", "addr", "add", f"{HOST_UNDERLAY_IP}/{UNDERLAY_PREFIX}", "dev", BRIDGE],
        "assign bridge underlay address",
    )
    run_required(["ip", "link", "set", BRIDGE, "up"], "bring up bridge")

    # TAP for the appliance NIC, enslaved to the bridge. qemu (root) attaches with
    # script=no, so the TAP must already exist and be up.
    run_required(["ip", "tuntap", "add", "dev", TAP, "mode", "tap"], "create tap")
    run_required(["ip", "link", "set", TAP, "master", BRIDGE], "enslave tap to bridge")
    run_required(["ip", "link", "set", TAP, "up"], "bring up tap")

    # veth pair: host side enslaved to the bridge, LAC side into the LAC netns.
    run_required(
        ["ip", "link", "add", VETH_HOST, "type", "veth", "peer", "name", VETH_LAC],
        "create LAC veth pair",
    )
    run_required(
        ["ip", "link", "set", VETH_HOST, "master", BRIDGE], "enslave host veth"
    )
    run_required(["ip", "link", "set", VETH_HOST, "up"], "bring up host veth")
    run_required(["ip", "link", "set", VETH_LAC, "netns", LAC_NS], "move LAC veth")
    ns_run_required(["ip", "link", "set", "lo", "up"], "bring up LAC loopback")
    ns_run_required(
        ["ip", "addr", "add", f"{LAC_UNDERLAY_IP}/{UNDERLAY_PREFIX}", "dev", VETH_LAC],
        "assign LAC underlay address",
    )
    ns_run_required(["ip", "link", "set", VETH_LAC, "up"], "bring up LAC veth")

    # Allow the appliance's DHCP DISCOVER through the host firewall to dnsmasq.
    allow_bridge_firewall()

    # DHCP for the appliance (boots with dhcp-auto); fixed reservation so xl2tpd
    # knows the appliance underlay address in advance.
    start_dnsmasq()

    ping = ns_run(
        ["ping", "-c", "1", "-W", "2", HOST_UNDERLAY_IP],
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
    )
    if ping.returncode != 0:
        sys.stderr.write((ping.stdout or "") + (ping.stderr or ""))
        raise RuntimeError("LAC namespace cannot reach the underlay bridge")


def wait_for_appliance_underlay(timeout_s: float) -> None:
    """Wait until the appliance has DHCP'd its underlay IP and answers on it."""
    deadline = time.time() + timeout_s
    while time.time() < deadline:
        ping = ns_run(
            ["ping", "-c", "1", "-W", "1", APPLIANCE_IP],
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
        )
        if ping.returncode == 0:
            return
        time.sleep(0.5)
    raise RuntimeError(
        f"appliance did not obtain underlay address {APPLIANCE_IP} (DHCP) in time"
    )


class LineCollector:
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


FATAL_NEEDLES = [
    # ze's fail-closed L2TP module probe refusing startup: the appliance will
    # crash-loop forever, so fail the proof now instead of burning the 90s
    # boot timeout (this line reaches serial via the kmsg startup mirror).
    "failed to load kernel modules",
    "skipping kernel module probe",
    "genl family resolve failed",
    "kernel integration disabled",
    "kernel session ready but no PPP driver wired",
    "ipcp: handler rejected",
    "ipv6cp: handler rejected",
    # NOTE: "IPv6 not supported by static pool" is NOT fatal -- it is the reason
    # the appliance logs when gracefully declining IPv6CP ("continuing IPv4-only")
    # against the IPv4-only proof pool, which is the expected behaviour here.
    "ncp: timeout",
    "ip-response timeout",
]


def fatal_any_phase(lines: list[str]) -> str | None:
    for line in lines:
        for needle in FATAL_NEEDLES:
            if needle in line:
                return f"ze appliance reported fatal failure: {needle}"
    return None


def fatal_pre_session(lines: list[str]) -> str | None:
    msg = fatal_any_phase(lines)
    if msg:
        return msg
    for line in lines:
        if "PPP requested session teardown" in line:
            return "ze appliance reported fatal failure: PPP requested session teardown"
    return None


def lines_contain(needle: str) -> Callable[[list[str]], bool]:
    return lambda lines: any(needle in line for line in lines)


def lines_contain_all(needles: list[str]) -> Callable[[list[str]], bool]:
    return lambda lines: all(
        any(needle in line for line in lines) for needle in needles
    )


def ppp_links() -> set[str]:
    links = ns_run(
        ["ip", "-o", "link", "show", "type", "ppp"],
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
    )
    if links.returncode != 0:
        sys.stderr.write((links.stdout or "") + (links.stderr or ""))
        raise RuntimeError("ip link show type ppp failed in LAC namespace")
    found: set[str] = set()
    for line in (links.stdout or "").splitlines():
        match = re.match(r"\d+:\s+([^:@]+)", line)
        if match:
            found.add(match.group(1))
    return found


def l2tp_state() -> tuple[str, str]:
    tunnel = ns_run(
        ["ip", "l2tp", "show", "tunnel"], stdout=subprocess.PIPE, stderr=subprocess.PIPE
    )
    session = ns_run(
        ["ip", "l2tp", "show", "session"],
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
    )
    if tunnel.returncode != 0 or session.returncode != 0:
        sys.stderr.write((tunnel.stdout or "") + (tunnel.stderr or ""))
        sys.stderr.write((session.stdout or "") + (session.stderr or ""))
        raise RuntimeError("ip l2tp state inspection failed in LAC namespace")
    return tunnel.stdout or "", session.stdout or ""


def discover_lac_ppp_iface(initial: set[str]) -> str:
    current = ppp_links()
    new_links = sorted(current - initial)
    if not new_links:
        raise RuntimeError("no new pppN interface appeared in LAC namespace")
    if len(new_links) > 1:
        raise RuntimeError(
            f"more than one new PPP interface appeared in LAC namespace: {', '.join(new_links)}"
        )
    return new_links[0]


def verify_ppp_address(iface: str) -> None:
    addr = ns_run(
        ["ip", "-o", "addr", "show", "dev", iface],
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
    )
    if addr.returncode != 0:
        sys.stderr.write((addr.stdout or "") + (addr.stderr or ""))
        raise RuntimeError(f"ip addr show dev {iface} failed")
    out = addr.stdout or ""
    if PEER_ADDR not in out or LOCAL_ADDR not in out:
        raise RuntimeError(
            f"{iface} lacks expected {PEER_ADDR} peer {LOCAL_ADDR} address state:\n{out}"
        )


def verify_dataplane() -> None:
    ping = ns_run(
        ["ping", "-c", "2", "-W", "3", LOCAL_ADDR],
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
    )
    if ping.returncode != 0:
        raise RuntimeError(
            f"dataplane ping to appliance LNS {LOCAL_ADDR} through PPP tunnel failed"
        )


def wait_for_lac_cleanup(
    initial_links: set[str], initial_l2tp: tuple[str, str], iface: str, timeout_s: float
) -> None:
    deadline = time.time() + timeout_s
    last_error = ""
    while time.time() < deadline:
        try:
            links = ppp_links()
            state = l2tp_state()
        except RuntimeError as err:
            last_error = str(err)
            time.sleep(0.2)
            continue
        if iface not in links and links == initial_links and state == initial_l2tp:
            return
        last_error = f"lac_ppp={sorted(links)} lac_l2tp_changed={state != initial_l2tp}"
        time.sleep(0.2)
    raise RuntimeError(
        f"LAC kernel L2TP/PPP cleanup did not return to initial state: {last_error}"
    )


def write_template(work: Path) -> Path:
    template = work / "ze-gokrazy-l2tp.conf"
    template.write_text(
        "set environment log level info\n"
        "set environment web enabled true\n"
        "set environment web server default ip 0.0.0.0\n"
        "set environment web server default port 8080\n"
        "set environment ssh enabled true\n"
        "set environment ssh server default ip 0.0.0.0\n"
        "set environment ssh server default port 22\n"
        "set environment ntp enabled false\n"
        "set interface dhcp-auto true\n"
        "set l2tp enabled true\n"
        "set l2tp auth-method none\n"
        "set l2tp allow-no-auth true\n"
        "set l2tp hello-interval 5\n"
        "set l2tp max-tunnels 4\n"
        "set l2tp max-sessions 4\n"
        f"set l2tp pool ipv4 gateway {LOCAL_ADDR}\n"
        f"set l2tp pool ipv4 start {PEER_ADDR}\n"
        "set l2tp pool ipv4 end 10.100.0.10\n"
        "set l2tp pool ipv4 dns-primary 8.8.8.8\n"
        "set l2tp pool ipv4 dns-secondary 8.8.4.4\n"
        "set environment l2tp server main ip 0.0.0.0\n"
        "set environment l2tp server main port 1701\n",
        encoding="utf-8",
    )
    return template


def write_lac_inputs(work: Path) -> None:
    (work / "xl2tpd.conf").write_text(
        "[global]\n"
        f"port = {XL2TPD_SOURCE_PORT}\n"
        f"auth file = {work / 'l2tp-secrets'}\n"
        "debug tunnel = yes\n"
        "debug state = yes\n"
        "debug packet = yes\n"
        "debug avp = yes\n"
        "\n"
        "[lac ze]\n"
        f"lns = {APPLIANCE_IP}\n"
        "autodial = yes\n"
        "redial = yes\n"
        "redial timeout = 1\n"
        "max redials = 5\n"
        "require authentication = no\n"
        "ppp debug = yes\n"
        f"pppoptfile = {work / 'ppp-options'}\n"
        "length bit = yes\n",
        encoding="utf-8",
    )
    (work / "l2tp-secrets").write_text("* * s3cr3t\n", encoding="utf-8")
    (work / "l2tp-secrets").chmod(0o600)
    (work / "ppp-options").write_text(
        "noauth\n"
        "name alice\n"
        "password s3cr3t\n"
        "refuse-eap\n"
        "nodefaultroute\n"
        "ipcp-accept-local\n"
        "ipcp-accept-remote\n"
        "noipv6\n"
        "debug\n"
        "nodetach\n",
        encoding="utf-8",
    )


# The appliance kernel must provide PPPoL2TP: the baked template sets
# `l2tp enabled true`, and ze's RFC 2661 fail-closed probe (probeKernelModules,
# internal/component/l2tp/kernel_linux.go) exits the daemon when neither
# l2tp_ppp nor pppol2tp is available, which gokrazy turns into a first-boot
# crash-loop with no "web server listening" ever appearing. The pinned
# github.com/rtr7/kernel has NO l2tp/ppp support (nothing in modules.builtin,
# zero loadable modules), so booting it here can never pass. The runtime kernel
# (gokrazy/kernel/runtime.config) builds CONFIG_L2TP/PPPOL2TP in; this proof
# therefore REQUIRES a kernel that carries l2tp and resolves one itself.
KERNEL_MAGIC = {
    # (offset, magic): amd64 bzImage "HdrS", arm64 Image "ARMd".
    "amd64": (0x202, b"HdrS"),
    "arm64": (0x38, b"ARMd"),
}


def pinned_kernel_version(root: Path) -> str:
    """The repo's single kernel source of truth (internal/appliance/kernel.version)."""
    return (
        (root / "internal" / "appliance" / "kernel.version")
        .read_text(encoding="utf-8")
        .strip()
    )


def kernel_pkg_problems(
    pkg: Path, arch: str, expected_version: str | None = None
) -> list[str]:
    """Pure validation of an out-of-tree kernel package tree.

    Returns the list of reasons this package cannot boot this proof: missing
    or wrong-architecture vmlinuz (a polluted module cache once held an arm64
    kernel under the amd64 pin), absent PPPoL2TP support (neither built into
    the kernel per modules.builtin nor present as a loadable module), and,
    when expected_version is given, a module tree for a different kernel than
    the pinned one (a staged tmp/kernel/pkg left by an older build would
    otherwise be reused across a kernel.version bump).
    """
    if arch not in KERNEL_MAGIC:
        # Same closed set qemu_command enforces, but that check runs AFTER
        # kernel resolution; failing here keeps the message ahead of a KeyError.
        raise SystemExit(
            f"unsupported ZE_GOKRAZY_ARCH={arch} (expected amd64 or arm64)"
        )
    problems: list[str] = []
    vmlinuz = pkg / "vmlinuz"
    if not vmlinuz.is_file():
        problems.append(f"no vmlinuz at {vmlinuz}")
    else:
        offset, magic = KERNEL_MAGIC[arch]
        with vmlinuz.open("rb") as fh:
            header = fh.read(offset + len(magic))
        if (
            len(header) < offset + len(magic)
            or header[offset : offset + len(magic)] != magic
        ):
            problems.append(
                f"vmlinuz at {vmlinuz} is not a {arch} kernel"
                f" (magic {magic!r} not found at {hex(offset)})"
            )
    builtin_files = sorted(pkg.glob("lib/modules/*/modules.builtin"))
    # l2tp_ppp.ko* also accepts compressed loadable modules (.ko.xz/.ko.zst);
    # the runtime kernel builds l2tp in, so this arm is for foreign packages.
    loadable = sorted(pkg.glob("lib/modules/*/**/l2tp_ppp.ko*"))
    if not builtin_files and not loadable:
        problems.append(f"no lib/modules/*/modules.builtin under {pkg}")
    else:
        builtin_l2tp = any(
            "l2tp_ppp" in path.read_text(encoding="utf-8", errors="replace")
            for path in builtin_files
        )
        if not builtin_l2tp and not loadable:
            problems.append(
                f"kernel package {pkg} has no PPPoL2TP support:"
                " l2tp_ppp is neither in modules.builtin nor a loadable .ko"
            )
    if expected_version is not None:
        releases = sorted(p.name for p in pkg.glob("lib/modules/*") if p.is_dir())
        if not any(
            r == expected_version or r.startswith(expected_version + "-")
            for r in releases
        ):
            problems.append(
                f"kernel package {pkg} carries release(s) {releases or ['none']},"
                f" not the pinned kernel.version {expected_version}"
            )
    return problems


def assert_kernel_pkg(
    pkg: Path, arch: str, context: str, expected_version: str | None = None
) -> None:
    problems = kernel_pkg_problems(pkg, arch, expected_version)
    if problems:
        detail = "\n  ".join(problems)
        raise SystemExit(
            f"unusable kernel package ({context}):\n  {detail}\n"
            f"rebuild it with: make ze-kernel KERNEL_ARCH={arch}"
            " (~30 min on a cache miss, needs docker)"
        )


def copy_kernel_pkg(src: Path, work: Path, arch: str) -> Path:
    """Copy a kernel package into this run's work dir and validate the copy.

    `tmp/kernel/pkg` is a shared fixed path that any concurrent `make
    ze-kernel` (another proof run, another arch, another session) rewrites
    starting with an rm -rf. gok reads the package minutes after validation,
    so consuming the shared path directly is validate-then-clobber TOCTOU;
    the per-run copy is what gok actually reads.
    """
    dst = work / "kernel-pkg"
    if dst.exists():
        shutil.rmtree(dst)
    shutil.copytree(src, dst, symlinks=True)
    assert_kernel_pkg(dst, arch, f"per-run copy of {src}")
    return dst


def probe_kernel_cache_dir(root: Path) -> Path:
    """Ask ze-host for the arch-keyed runtime-kernel cache directory."""
    run_required(["make", "ze-host"], "build host ze binary", cwd=root)
    cache_probe = run_required(
        [
            str(root / "ze-host"),
            "appliance",
            "kernel",
            "--target",
            "runtime",
            "--arch",
            ARCH,
            "--print-cache-dir",
        ],
        "resolve runtime kernel cache dir",
        cwd=root,
    )
    lines = [line for line in cache_probe.stdout.splitlines() if line.strip()]
    if not lines:
        # The make equivalent guards `[ -z "$cache_dir" ]`; without this the
        # operator would get an IndexError traceback instead of a next step.
        raise SystemExit(
            "could not resolve the runtime kernel cache dir: ze-host"
            " appliance kernel --print-cache-dir produced no output;"
            f" try: make ze-kernel KERNEL_ARCH={ARCH}"
        )
    return Path(lines[-1].strip())


def resolve_kernel_pkg(root: Path, work: Path) -> Path:
    """Resolve the L2TP-capable kernel package this proof boots.

    An explicit KERNEL_PKG is validated and used as-is. Otherwise: a valid
    already-assembled `tmp/kernel/pkg` is reused (no shared-state rewrite, no
    `tmp/kernel/vmlinuz` restaging for the sibling QEMU labs); failing that,
    the runtime kernel is materialized from the durable cache via `make
    ze-kernel`, and a cold or unusable cache fails fast with the exact command
    to run, instead of booting an image whose ze can only crash-loop. Either
    way the package handed to the build is a per-run copy under this proof's
    work dir (see copy_kernel_pkg).
    """
    explicit = os.environ.get("KERNEL_PKG")
    if explicit:
        # No version pin here: an explicit KERNEL_PKG is the operator's own
        # kernel choice (that is the flag's purpose); arch + l2tp still gate.
        pkg = Path(explicit)
        if not pkg.is_absolute():
            pkg = root / pkg
        assert_kernel_pkg(pkg, ARCH, "from KERNEL_PKG")
        return copy_kernel_pkg(pkg, work, ARCH)

    pinned = pinned_kernel_version(root)
    staged = root / "tmp" / "kernel" / "pkg"
    if not kernel_pkg_problems(staged, ARCH, pinned):
        return copy_kernel_pkg(staged, work, ARCH)

    cache_dir = probe_kernel_cache_dir(root)
    problems = kernel_pkg_problems(cache_dir, ARCH, pinned)
    if problems:
        detail = "\n  ".join(problems)
        # make's HIT branch is existence-only, so an EXISTING-but-invalid
        # cache entry (stub, wrong arch from a pre-guard populate) would be
        # re-materialized as-is by `make ze-kernel`; the remediation is only
        # true if the bad entry is removed first.
        if cache_dir.is_dir():
            remediation = (
                f"the cache entry exists but is unusable; remove it first:\n"
                f"  rm -rf {cache_dir}\n"
                f"then rebuild it: make ze-kernel KERNEL_ARCH={ARCH}"
            )
        else:
            remediation = f"build it once with: make ze-kernel KERNEL_ARCH={ARCH}"
        raise SystemExit(
            "this proof needs the runtime kernel (PPPoL2TP built in;"
            " the pinned rtr7 kernel has no l2tp support and the appliance"
            " would crash-loop on ze's fail-closed module probe), but the"
            f" durable cache at {cache_dir} cannot provide it:\n  {detail}\n"
            f"{remediation}"
            " (~30 min, needs docker), then re-run this proof.\n"
            "note: this proof usually runs under sudo, and sudo commonly"
            " resets HOME, so the cache probed here is root's; a kernel built"
            " as your own user lives in YOUR cache. Either run the ze-kernel"
            " build under sudo too, or re-run the proof with XDG_CACHE_HOME"
            " pointed at the cache that holds the kernel"
        )
    sys.stderr.write(
        "materializing the runtime kernel package from the durable cache"
        " (make ze-kernel; instant on this validated cache)...\n"
    )
    # Streamed (no capture): if the cache is evicted between the probe above
    # and this invocation, make's MISS branch starts a ~30-minute docker
    # build, and that must be visible live, not silent inside a pipe.
    result = run(["make", "ze-kernel", f"KERNEL_ARCH={ARCH}"], cwd=root)
    if result.returncode != 0:
        raise SystemExit("make ze-kernel failed; see output above")
    assert_kernel_pkg(staged, ARCH, "assembled by make ze-kernel", pinned)
    return copy_kernel_pkg(staged, work, ARCH)


def proof_image_path(root: Path, work: Path) -> Path:
    override = os.environ.get("ZE_GOKRAZY_IMAGE")
    if override:
        path = Path(override)
        return path if path.is_absolute() else root / path
    if os.environ.get("ZE_GOKRAZY_SKIP_BUILD") == "1":
        return root / "tmp" / "gokrazy" / "ze.img"
    return work / "ze.img"


def add_env_once(envs: list[str], item: str) -> None:
    key = item.split("=", 1)[0] + "="
    if not any(existing.startswith(key) for existing in envs):
        envs.append(item)


def prepare_instance(root: Path, work: Path) -> Path:
    """Build a gokrazy parent dir carrying ONLY this proof's config.json patch.

    Copying the builddir and rewriting its `replace` directives used to happen
    here too, in parallel with the same logic in Go. That duplication is gone:
    ze-gok prepares the instance it is handed (internal/appliance/instance),
    copying the builddir and absolutizing every filesystem-path replace. So this
    symlinks the builddir rather than duplicating it, and the go.mod regexes are
    deleted along with the custom-kernel one, which is now selected per build
    with `make ze-gokrazy KERNEL_PKG=...` instead of an in-tree replace.
    """
    parent = work / "gokrazy-parent"
    instance = parent / "ze"
    instance.mkdir(parents=True, exist_ok=True)

    source = root / "gokrazy" / "ze"

    # A symlink, not a copy: ze-gok copies it into the prepared instance. The
    # preparer resolves a symlinked builddir precisely so callers need not.
    builddir = instance / "builddir"
    if builddir.is_symlink() or builddir.exists():
        if builddir.is_symlink():
            builddir.unlink()
        else:
            shutil.rmtree(builddir)
    builddir.symlink_to(source / "builddir")

    config = json.loads((source / "config.json").read_text(encoding="utf-8"))
    pkg_cfg = config.setdefault("PackageConfig", {}).setdefault(ZE_PACKAGE, {})
    envs = pkg_cfg.setdefault("Environment", [])
    for item in PROOF_ZE_ENV:
        add_env_once(envs, item)
    (instance / "config.json").write_text(
        json.dumps(config, indent=4) + "\n", encoding="utf-8"
    )

    return parent


def build_image(root: Path, work: Path, template: Path) -> Path:
    image = proof_image_path(root, work)
    if os.environ.get("ZE_GOKRAZY_SKIP_BUILD") == "1":
        if not image.is_file():
            raise SystemExit(f"gokrazy image not found: {image}")
        sys.stderr.write(
            "using existing gokrazy image; it must already contain the L2TP"
            " proof template, the proof runtime environment, AND an"
            " L2TP-capable kernel (the pinned rtr7 kernel has none; an image"
            " built without KERNEL_PKG crash-loops at first boot)\n"
        )
        return image

    env = os.environ.copy()
    env.setdefault("USER", "admin")
    # No `make bin/gok` here: bin/gok is .PHONY and the ze-gokrazy target
    # below rebuilds it (a stale gok once silently ran pre-preparer logic).
    parent = prepare_instance(root, work)
    cmd = [
        "make",
        "ze-gokrazy",
        f"GOKRAZY_DIR={parent}",
        "GOKRAZY_INSTANCE=ze",
        f"GOKRAZY_ARCH={ARCH}",
        f"GOKRAZY_IMG={image}",
        "USER=admin",
        "PASS=secret",
        f"GOKRAZY_TEMPLATE={template}",
    ]
    # This proof ALWAYS builds with an L2TP-capable kernel: the pinned rtr7
    # kernel has no l2tp support, so without this the appliance ze crash-loops
    # at first boot on its fail-closed module probe and the proof can never
    # pass. resolve_kernel_pkg validates an explicit KERNEL_PKG or reuses /
    # materializes the runtime kernel, always handing back a per-run copy.
    kernel_pkg = resolve_kernel_pkg(root, work)
    cmd.append(f"KERNEL_PKG={kernel_pkg}")
    result = run(cmd, cwd=root, env=env)
    if result.returncode != 0:
        raise SystemExit("make ze-gokrazy for L2TP appliance evidence failed")
    if not image.is_file():
        raise SystemExit(f"gokrazy image not found after build: {image}")
    return image


def qemu_command(image: Path) -> list[str]:
    # TAP netdev on the underlay bridge (not slirp/user): slirp does not deliver
    # inbound UDP hostfwd to the guest, which L2TP's SCCRQ requires. The appliance
    # is reachable directly at APPLIANCE_IP over the bridge; the harness verifies
    # web/L2TP via the serial console and LAC PPP state, so no host port-forwards
    # are needed.
    netdev = f"tap,id=net0,ifname={TAP},script=no,downscript=no"
    if ARCH == "amd64":
        require_cmd("qemu-system-x86_64")
        return [
            "qemu-system-x86_64",
            "-machine",
            f"accel={QEMU_ACCEL}",
            "-smp",
            "2",
            "-m",
            "512",
            "-drive",
            f"file={image},format=raw",
            "-nographic",
            "-serial",
            "mon:stdio",
            "-netdev",
            netdev,
            "-device",
            f"e1000,netdev=net0,mac={APPLIANCE_MAC}",
        ]
    if ARCH == "arm64":
        require_cmd("qemu-system-aarch64")
        bios = Path(
            os.environ.get(
                "ZE_GOKRAZY_AARCH64_BIOS", "/usr/share/qemu-efi-aarch64/QEMU_EFI.fd"
            )
        )
        if not bios.is_file():
            raise SystemExit(f"aarch64 QEMU firmware not found: {bios}")
        cpu = os.environ.get("ZE_GOKRAZY_AARCH64_CPU", "max")
        return [
            "qemu-system-aarch64",
            "-machine",
            f"virt,highmem=off,accel={QEMU_ACCEL}",
            "-cpu",
            cpu,
            "-smp",
            "2",
            "-m",
            "512",
            "-bios",
            str(bios),
            "-drive",
            f"file={image},format=raw",
            "-nographic",
            "-serial",
            "mon:stdio",
            "-netdev",
            netdev,
            "-device",
            f"e1000,netdev=net0,mac={APPLIANCE_MAC}",
        ]
    raise SystemExit(f"unsupported ZE_GOKRAZY_ARCH={ARCH} (expected amd64 or arm64)")


def main() -> int:
    for key in ["ZE_L2TP_SKIP_KERNEL_PROBE", "ze.l2tp.skip-kernel-probe"]:
        if key in os.environ:
            raise SystemExit(
                f"refusing to run with {key} set; full proof must not skip the kernel probe"
            )
    require_cmd("ip")
    require_cmd("ping")
    require_cmd("xl2tpd")
    require_cmd("pppd")
    require_cmd("dnsmasq")
    ensure_host_kernel_support()

    root = repo_root()
    tmp_parent = root / "tmp" / "evidence"
    tmp_parent.mkdir(parents=True, exist_ok=True)
    work = Path(tempfile.mkdtemp(prefix="gokrazy-l2tp-ppp-", dir=tmp_parent))
    template = write_template(work)
    # xl2tpd 1.3.18 truncates -c config paths longer than ~90 chars, so the LAC's
    # runtime files (config, secrets, pid, control, ppp-options) must live at a
    # short path -- the deep repo tmp/evidence/<mkdtemp>/ path (~100+ chars here)
    # gets silently cut and xl2tpd fails with "Unable to open config file". Build
    # artifacts (image, template) stay under `work`; only the host LAC files move.
    lac_dir = Path(tempfile.mkdtemp(prefix="zel2tp-"))
    write_lac_inputs(lac_dir)

    qemu: subprocess.Popen[str] | None = None
    xl2tpd: subprocess.Popen[str] | None = None
    success = False
    try:
        image = build_image(root, work, template)
        setup_lac_netns()
        initial_lac_links = ppp_links()
        initial_lac_l2tp = l2tp_state()

        qemu = subprocess.Popen(
            qemu_command(image),
            cwd=root,
            stdout=subprocess.PIPE,
            stderr=subprocess.STDOUT,
            text=True,
            bufsize=1,
        )
        assert qemu.stdout is not None
        qemu_log = LineCollector("qemu> ", qemu.stdout)

        if not qemu_log.wait_for(
            lines_contain("web server listening"), 90, qemu, fatal_pre_session
        ):
            raise RuntimeError("gokrazy appliance web server did not start")
        if not qemu_log.wait_for(
            lines_contain("L2TP listener bound"), 30, qemu, fatal_pre_session
        ):
            raise RuntimeError("gokrazy appliance L2TP listener did not start")

        # The appliance DHCPs its underlay address; wait until it answers there
        # before xl2tpd dials it as the LNS.
        wait_for_appliance_underlay(30)

        xl2tpd = ns_popen(
            [
                "xl2tpd",
                "-D",
                "-c",
                str(lac_dir / "xl2tpd.conf"),
                "-s",
                str(lac_dir / "l2tp-secrets"),
                "-p",
                str(lac_dir / "xl2tpd.pid"),
                "-C",
                str(lac_dir / "l2tp-control"),
            ],
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            text=True,
            bufsize=1,
        )
        assert xl2tpd.stdout is not None
        assert xl2tpd.stderr is not None
        LineCollector("xl2tpd> ", xl2tpd.stdout)
        LineCollector("xl2tpd-err> ", xl2tpd.stderr)

        if not qemu_log.wait_for(
            lines_contain("l2tp: session established (incoming LNS)"),
            30,
            qemu,
            fatal_pre_session,
        ):
            raise RuntimeError(
                "xl2tpd did not establish an incoming L2TP session with the appliance"
            )

        success_needles = [
            "l2tp: session IP assigned",
            "l2tp: subscriber route inject",
            "l2tp: PPP session up",
        ]
        if not qemu_log.wait_for(
            lines_contain_all(success_needles), 60, qemu, fatal_any_phase
        ):
            raise RuntimeError(
                "appliance PPP LCP/IPCP completion and route injection were not observed"
            )

        snapshot = qemu_log.snapshot()
        ip_assigned_lines = [
            line for line in snapshot if "l2tp: session IP assigned" in line
        ]
        if not any(f"address={PEER_ADDR}" in line for line in ip_assigned_lines):
            raise RuntimeError(
                f"session IP assigned log missing expected address={PEER_ADDR}"
            )

        session_up_lines = [line for line in snapshot if "l2tp: PPP session up" in line]
        ze_iface = None
        for line in session_up_lines:
            match = re.search(r"interface=([^\s]+)", line)
            if match:
                candidate = match.group(1).strip('"')
                if candidate.startswith("ppp"):
                    ze_iface = candidate
                    break
        if ze_iface is None:
            raise RuntimeError(
                "appliance PPP session up log missing interface=pppN field"
            )

        lac_iface = discover_lac_ppp_iface(initial_lac_links)
        verify_ppp_address(lac_iface)
        verify_dataplane()

        terminate(xl2tpd)
        xl2tpd = None
        if not qemu_log.wait_for(
            lines_contain("l2tp: subscriber routes withdrawn"), 20, qemu
        ):
            raise RuntimeError(
                "appliance subscriber route withdraw was not observed during teardown"
            )
        wait_for_lac_cleanup(initial_lac_links, initial_lac_l2tp, lac_iface, 30)

        print(
            f"OK: gokrazy Ze appliance completed real L2TP PPP/IPCP with Ze {ze_iface} and LAC {lac_iface}, dataplane ping, route inject, and clean teardown"
        )
        success = True
        return 0
    except RuntimeError as err:
        sys.stderr.write(f"FAIL: {err}\n")
        if "qemu_log" in locals():
            lines = qemu_log.snapshot()
            sys.stderr.write("qemu log tail:\n" + "".join(lines[-140:]))
        return 1
    finally:
        terminate(xl2tpd)
        terminate(qemu)
        cleanup_netns()
        shutil.rmtree(lac_dir, ignore_errors=True)
        if success:
            shutil.rmtree(work, ignore_errors=True)


if __name__ == "__main__":
    raise SystemExit(main())
