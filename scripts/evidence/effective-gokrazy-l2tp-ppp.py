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
QEMU_ACCEL = os.environ.get("ZE_GOKRAZY_QEMU_ACCEL") or (
    "kvm" if Path("/dev/kvm").exists() else "tcg"
)
NS_SUFFIX = str(os.getpid())
VETH_SUFFIX = NS_SUFFIX[-6:]
LAC_NS = f"ze-gokrazy-lac-{NS_SUFFIX}"
VETH_HOST = f"zgokh{VETH_SUFFIX}"
VETH_LAC = f"zgokl{VETH_SUFFIX}"


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


def cleanup_netns() -> None:
    kill_netns_processes(signal.SIGTERM)
    time.sleep(0.2)
    kill_netns_processes(signal.SIGKILL)
    for link in [VETH_HOST, VETH_LAC]:
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
    run_required(
        ["ip", "link", "add", VETH_HOST, "type", "veth", "peer", "name", VETH_LAC],
        "create LAC veth pair",
    )
    run_required(["ip", "link", "set", VETH_LAC, "netns", LAC_NS], "move LAC veth")
    run_required(
        [
            "ip",
            "addr",
            "add",
            f"{HOST_UNDERLAY_IP}/{UNDERLAY_PREFIX}",
            "dev",
            VETH_HOST,
        ],
        "assign host underlay address",
    )
    run_required(["ip", "link", "set", VETH_HOST, "up"], "bring up host veth")
    ns_run_required(["ip", "link", "set", "lo", "up"], "bring up LAC loopback")
    ns_run_required(
        ["ip", "addr", "add", f"{LAC_UNDERLAY_IP}/{UNDERLAY_PREFIX}", "dev", VETH_LAC],
        "assign LAC underlay address",
    )
    ns_run_required(["ip", "link", "set", VETH_LAC, "up"], "bring up LAC veth")

    ping = ns_run(
        ["ping", "-c", "1", "-W", "2", HOST_UNDERLAY_IP],
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
    )
    if ping.returncode != 0:
        sys.stderr.write((ping.stdout or "") + (ping.stderr or ""))
        raise RuntimeError("LAC namespace cannot reach host QEMU-forwarding address")


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
    "skipping kernel module probe",
    "genl family resolve failed",
    "kernel integration disabled",
    "kernel session ready but no PPP driver wired",
    "ipcp: handler rejected",
    "ipv6cp: handler rejected",
    "IPv6 not supported by static pool",
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
        f"lns = {HOST_UNDERLAY_IP}\n"
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
    parent = work / "gokrazy-parent"
    instance = parent / "ze"
    instance.mkdir(parents=True, exist_ok=True)

    source = root / "gokrazy" / "ze"
    shutil.copytree(source / "builddir", instance / "builddir", symlinks=True)

    config = json.loads((source / "config.json").read_text(encoding="utf-8"))
    pkg_cfg = config.setdefault("PackageConfig", {}).setdefault(ZE_PACKAGE, {})
    envs = pkg_cfg.setdefault("Environment", [])
    for item in PROOF_ZE_ENV:
        add_env_once(envs, item)
    (instance / "config.json").write_text(
        json.dumps(config, indent=4) + "\n", encoding="utf-8"
    )

    ze_mod = instance / "builddir" / "codeberg.org" / "thomas-mangin" / "ze" / "go.mod"
    text = ze_mod.read_text(encoding="utf-8")
    text = re.sub(
        r"replace codeberg\.org/thomas-mangin/ze => .+",
        f"replace codeberg.org/thomas-mangin/ze => {root}",
        text,
    )
    ze_mod.write_text(text, encoding="utf-8")

    return parent


def build_image(root: Path, work: Path, template: Path) -> Path:
    image = proof_image_path(root, work)
    if os.environ.get("ZE_GOKRAZY_SKIP_BUILD") == "1":
        if not image.is_file():
            raise SystemExit(f"gokrazy image not found: {image}")
        sys.stderr.write(
            "using existing gokrazy image; it must already contain the L2TP proof template and proof runtime environment\n"
        )
        return image

    env = os.environ.copy()
    env.setdefault("USER", "admin")
    run_required(["make", "bin/gok"], "prepare gokrazy build tool", cwd=root, env=env)
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
    result = run(cmd, cwd=root, env=env)
    if result.returncode != 0:
        raise SystemExit("make ze-gokrazy for L2TP appliance evidence failed")
    if not image.is_file():
        raise SystemExit(f"gokrazy image not found after build: {image}")
    return image


def qemu_command(image: Path) -> list[str]:
    netdev = (
        f"user,id=net0,hostfwd=tcp::{WEB_HOST_PORT}-:8080,"
        f"hostfwd=tcp::{SSH_HOST_PORT}-:22,hostfwd=udp::{L2TP_HOST_PORT}-:1701"
    )
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
            "e1000,netdev=net0",
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
            "e1000,netdev=net0",
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
    ensure_host_kernel_support()

    root = repo_root()
    tmp_parent = root / "tmp" / "evidence"
    tmp_parent.mkdir(parents=True, exist_ok=True)
    work = Path(tempfile.mkdtemp(prefix="gokrazy-l2tp-ppp-", dir=tmp_parent))
    template = write_template(work)
    write_lac_inputs(work)

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

        xl2tpd = ns_popen(
            [
                "xl2tpd",
                "-D",
                "-c",
                str(work / "xl2tpd.conf"),
                "-s",
                str(work / "l2tp-secrets"),
                "-p",
                str(work / "xl2tpd.pid"),
                "-C",
                str(work / "l2tp-control"),
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
        if success:
            shutil.rmtree(work, ignore_errors=True)


if __name__ == "__main__":
    raise SystemExit(main())
