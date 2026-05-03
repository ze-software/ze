#!/usr/bin/env python3
"""Run ze against a real xl2tpd LAC peer with PPPoL2TP."""

from __future__ import annotations

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


ZE_LISTEN_IP = os.environ.get("ZE_L2TP_PPP_LISTEN_IP", "127.0.0.1")
ZE_LISTEN_PORT = os.environ.get("ZE_L2TP_PPP_LISTEN_PORT", "1701")
XL2TPD_SOURCE_PORT = os.environ.get("ZE_L2TP_PPP_XL2TPD_PORT", "1702")
LOCAL_ADDR = "10.100.0.1"
PEER_ADDR = "10.100.0.2"


def repo_root() -> Path:
    here = Path(__file__).resolve()
    for parent in here.parents:
        if (parent / "go.mod").is_file():
            return parent
    raise SystemExit("cannot locate repository root")


def run(cmd: list[str], **kwargs) -> subprocess.CompletedProcess[str]:
    return subprocess.run(cmd, text=True, check=False, **kwargs)


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


def ensure_kernel_support() -> None:
    if platform.system() != "Linux":
        raise SystemExit("full L2TP PPP/NCP evidence requires Linux")
    if not has_cap_net_admin():
        raise SystemExit("full L2TP PPP/NCP evidence requires root or CAP_NET_ADMIN")

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
        raise SystemExit(
            "missing PPPoL2TP kernel support: expected /proc/net/pppol2tp or l2tp_ppp module"
        )

    genl = run(
        ["ip", "l2tp", "show", "tunnel"], stdout=subprocess.PIPE, stderr=subprocess.PIPE
    )
    if genl.returncode != 0:
        sys.stderr.write((genl.stdout or "") + (genl.stderr or ""))
        raise SystemExit("ip l2tp cannot access the kernel L2TP Generic Netlink family")


def reject_skip_kernel_probe_env() -> None:
    for key in ["ZE_L2TP_SKIP_KERNEL_PROBE", "ze.l2tp.skip-kernel-probe"]:
        if key in os.environ:
            raise SystemExit(
                f"refusing to run with {key} set; full proof must not skip the kernel probe"
            )


def ensure_ze(root: Path) -> Path:
    override = os.environ.get("ZE_EVIDENCE_ZE_BINARY") or os.environ.get(
        "ZE_L2TP_PPP_ZE_BINARY"
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
    ze = bindir / "ze-l2tp-ppp"

    env = os.environ.copy()
    env.setdefault("GOCACHE", str(root / "tmp" / "go-cache"))
    build = run(["go", "build", "-o", str(ze), "./cmd/ze"], cwd=root, env=env)
    if build.returncode != 0:
        raise SystemExit("go build ./cmd/ze failed")
    return ze


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
    "ncp: timeout",
    "ip-response timeout",
]


def fatal_any_phase(lines: list[str]) -> str | None:
    for line in lines:
        for needle in FATAL_NEEDLES:
            if needle in line:
                return f"ze reported fatal failure: {needle}"
    return None


def fatal_pre_session(lines: list[str]) -> str | None:
    msg = fatal_any_phase(lines)
    if msg:
        return msg
    for line in lines:
        if "PPP requested session teardown" in line:
            return "ze reported fatal failure: PPP requested session teardown"
    return None


def lines_contain(needle: str) -> Callable[[list[str]], bool]:
    return lambda lines: any(needle in line for line in lines)


def lines_contain_all(needles: list[str]) -> Callable[[list[str]], bool]:
    return lambda lines: all(
        any(needle in line for line in lines) for needle in needles
    )


def ppp_links() -> set[str]:
    links = run(
        ["ip", "-o", "link", "show", "type", "ppp"],
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
    )
    if links.returncode != 0:
        sys.stderr.write((links.stdout or "") + (links.stderr or ""))
        raise RuntimeError("ip link show type ppp failed")
    found: set[str] = set()
    for line in (links.stdout or "").splitlines():
        match = re.match(r"\d+:\s+([^:@]+)", line)
        if match:
            found.add(match.group(1))
    return found


def l2tp_state() -> tuple[str, str]:
    tunnel = run(
        ["ip", "l2tp", "show", "tunnel"], stdout=subprocess.PIPE, stderr=subprocess.PIPE
    )
    session = run(
        ["ip", "l2tp", "show", "session"],
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
    )
    if tunnel.returncode != 0 or session.returncode != 0:
        sys.stderr.write((tunnel.stdout or "") + (tunnel.stderr or ""))
        sys.stderr.write((session.stdout or "") + (session.stderr or ""))
        raise RuntimeError("ip l2tp state inspection failed")
    return tunnel.stdout or "", session.stdout or ""


def discover_new_ppp_iface(initial: set[str], ze_lines: list[str]) -> str:
    for line in ze_lines:
        match = re.search(r"interface=([^\s]+)", line)
        if match and match.group(1).startswith("ppp"):
            return match.group(1).strip('"')
    current = ppp_links()
    new_links = sorted(current - initial)
    if not new_links:
        raise RuntimeError("no new pppN interface appeared")
    if len(new_links) > 1:
        raise RuntimeError(
            f"more than one new PPP interface appeared: {', '.join(new_links)}"
        )
    return new_links[0]


def verify_ppp_address(iface: str) -> None:
    addr = run(
        ["ip", "-o", "addr", "show", "dev", iface],
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
    )
    if addr.returncode != 0:
        sys.stderr.write((addr.stdout or "") + (addr.stderr or ""))
        raise RuntimeError(f"ip addr show dev {iface} failed")
    out = addr.stdout or ""
    if LOCAL_ADDR not in out or PEER_ADDR not in out:
        raise RuntimeError(
            f"{iface} lacks expected {LOCAL_ADDR} peer {PEER_ADDR} address state:\n{out}"
        )


def verify_dataplane() -> None:
    ping = run(
        ["ping", "-c", "2", "-W", "3", LOCAL_ADDR],
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
    )
    if ping.returncode != 0:
        raise RuntimeError(
            f"dataplane ping to {LOCAL_ADDR} (LNS) through PPP tunnel failed"
        )


def wait_for_cleanup(
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
        last_error = (
            f"ppp links={sorted(links)} l2tp_state_changed={state != initial_l2tp}"
        )
        time.sleep(0.2)
    raise RuntimeError(
        f"kernel L2TP/PPP cleanup did not return to initial state: {last_error}"
    )


def write_inputs(work: Path) -> None:
    (work / "ze").mkdir(parents=True, exist_ok=True)
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
        f"lns = {ZE_LISTEN_IP}\n"
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
        "debug\n"
        "nodetach\n",
        encoding="utf-8",
    )
    (work / "ze.conf").write_text(
        "l2tp {\n"
        "    enabled true;\n"
        "    auth-method none;\n"
        "    allow-no-auth true;\n"
        "    hello-interval 5;\n"
        "    max-tunnels 4;\n"
        "    max-sessions 4;\n"
        "    pool {\n"
        "        ipv4 {\n"
        f"            gateway {LOCAL_ADDR};\n"
        f"            start {PEER_ADDR};\n"
        "            end 10.100.0.10;\n"
        "            dns-primary 8.8.8.8;\n"
        "            dns-secondary 8.8.4.4;\n"
        "        }\n"
        "    }\n"
        "}\n"
        "environment {\n"
        "    l2tp {\n"
        "        server main {\n"
        f"            ip {ZE_LISTEN_IP};\n"
        f"            port {ZE_LISTEN_PORT};\n"
        "        }\n"
        "    }\n"
        "}\n",
        encoding="utf-8",
    )


def main() -> int:
    reject_skip_kernel_probe_env()
    if platform.system() != "Linux":
        raise SystemExit("full L2TP PPP/NCP evidence requires Linux")
    require_cmd("ip")
    require_cmd("xl2tpd")
    require_cmd("pppd")
    ensure_kernel_support()

    root = repo_root()
    ze = ensure_ze(root)
    tmp_parent = root / "tmp" / "evidence"
    tmp_parent.mkdir(parents=True, exist_ok=True)
    work = Path(tempfile.mkdtemp(prefix="effective-l2tp-ppp-", dir=tmp_parent))
    write_inputs(work)

    initial_links = ppp_links()
    initial_l2tp = l2tp_state()

    ze_proc: subprocess.Popen[str] | None = None
    xl2tpd: subprocess.Popen[str] | None = None
    success = False
    try:
        env = os.environ.copy()
        env["ZE_LOG_L2TP"] = "debug"
        env["ZE_STORAGE_BLOB"] = "false"
        env["ZE_CONFIG_DIR"] = str(work / "ze")
        env["ze.l2tp.ncp.enable-ipv6cp"] = "false"
        env["ze.l2tp.ncp.ip-timeout"] = "15s"
        env["ze.l2tp.auth.timeout"] = "15s"
        for key in ["ZE_L2TP_SKIP_KERNEL_PROBE", "ze.l2tp.skip-kernel-probe"]:
            env.pop(key, None)

        ze_proc = subprocess.Popen(
            [str(ze), str(work / "ze.conf")],
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            text=True,
            bufsize=1,
            env=env,
        )
        assert ze_proc.stdout is not None
        assert ze_proc.stderr is not None
        LineCollector("ze-out> ", ze_proc.stdout)
        ze_log = LineCollector("ze> ", ze_proc.stderr)

        if not ze_log.wait_for(
            lines_contain("L2TP listener bound"), 20, ze_proc, fatal_pre_session
        ):
            raise RuntimeError("ze L2TP listener did not start")

        xl2tpd = subprocess.Popen(
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

        if not ze_log.wait_for(
            lines_contain("l2tp: session established (incoming LNS)"),
            30,
            ze_proc,
            fatal_pre_session,
        ):
            raise RuntimeError("xl2tpd did not establish an incoming L2TP session")

        success_needles = [
            "l2tp: session IP assigned",
            "l2tp: subscriber route inject",
            "l2tp: PPP session up",
        ]
        if not ze_log.wait_for(
            lines_contain_all(success_needles), 45, ze_proc, fatal_any_phase
        ):
            raise RuntimeError(
                "PPP LCP/IPCP completion and subscriber route injection were not observed"
            )

        snapshot = ze_log.snapshot()
        ip_assigned_lines = [l for l in snapshot if "l2tp: session IP assigned" in l]
        if not any(f"address={PEER_ADDR}" in l for l in ip_assigned_lines):
            raise RuntimeError(
                f"session IP assigned log missing expected address={PEER_ADDR}"
            )

        iface = discover_new_ppp_iface(initial_links, snapshot)

        session_up_lines = [l for l in ze_log.snapshot() if "l2tp: PPP session up" in l]
        if not any(f"interface={iface}" in l for l in session_up_lines):
            raise RuntimeError(f"PPP session up log missing expected interface={iface}")

        verify_ppp_address(iface)
        verify_dataplane()

        terminate(xl2tpd)
        xl2tpd = None
        if not ze_log.wait_for(
            lines_contain("l2tp: subscriber routes withdrawn"), 15, ze_proc
        ):
            raise RuntimeError(
                "subscriber route withdraw was not observed during teardown"
            )
        wait_for_cleanup(initial_links, initial_l2tp, iface, 15)

        print(
            f"OK: real xl2tpd/pppd peer completed PPP LCP, IPCP, {iface} address assignment, dataplane ping, route inject, and clean teardown"
        )
        success = True
        return 0
    except RuntimeError as err:
        sys.stderr.write(f"FAIL: {err}\n")
        if ze_proc is not None:
            lines = ze_log.snapshot() if "ze_log" in locals() else []
            sys.stderr.write("ze log tail:\n" + "".join(lines[-100:]))
        return 1
    finally:
        terminate(xl2tpd)
        terminate(ze_proc)
        if success:
            shutil.rmtree(work, ignore_errors=True)


if __name__ == "__main__":
    raise SystemExit(main())
