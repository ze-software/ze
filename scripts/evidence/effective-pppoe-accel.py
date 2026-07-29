#!/usr/bin/env python3
"""Run Ze's PPPoE client and a real accel-ppp access concentrator in isolated
Linux namespaces (no Docker).

This is the QEMU/netns sibling of the Docker lab in test/pppoe-interop/: same
roles (Ze is the PPPoE client, accel-ppp is the AC), but joined by a veth pair
across two network namespaces so it runs inside the QEMU runtime kernel where
CONFIG_PPPOE is built in. Driven by `make ze-qemu-pppoe-accel-test`.

Proves: PADI/PADO/PADR/PADS discovery, LCP, CHAP-MD5 auth, IPCP address
assignment, the kernel pppN interface on both ends with the server-assigned
P2P address, dataplane ping to the AC gateway through the session, and clean
teardown when the client stops.
"""

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

PEER_ADDR = "10.11.0.1"  # accel-ppp [ip-pool] gw-ip-address (the AC side)
LOCAL_ADDR = "10.11.0.2"  # first address handed out from the pool (the client)
USERNAME = "alice"
PASSWORD = "s3cr3t"
SERVICE_NAME = "internet"

NS_SUFFIX = str(os.getpid())
VETH_SUFFIX = NS_SUFFIX[-6:]
ZE_NS = f"ze-pppoe-ze-{NS_SUFFIX}"
AC_NS = f"ze-pppoe-ac-{NS_SUFFIX}"
VETH_ZE = f"zpoez{VETH_SUFFIX}"
VETH_AC = f"zpoea{VETH_SUFFIX}"


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


def ns_run_required(ns: str, cmd: list[str], context: str, **kwargs):
    return run_required(["ip", "netns", "exec", ns, *cmd], context, **kwargs)


def ns_popen(ns: str, cmd: list[str], **kwargs) -> subprocess.Popen[str]:
    return subprocess.Popen(["ip", "netns", "exec", ns, *cmd], **kwargs)


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
        if line.startswith("CapEff:"):
            return bool(int(line.split()[1], 16) & (1 << 12))
    return False


def try_load_modules() -> None:
    modprobe = shutil.which("modprobe")
    if modprobe is None or os.geteuid() != 0:
        return
    for mod in ["ppp_generic", "pppox", "pppoe"]:
        run([modprobe, mod], stdout=subprocess.PIPE, stderr=subprocess.PIPE)


def reject_skip_kernel_probe_env() -> None:
    for key in ["ZE_PPPOE_SKIP_KERNEL_PROBE", "ze.pppoe.skip-kernel-probe"]:
        if key in os.environ:
            raise SystemExit(
                f"refusing to run with {key} set; full proof must not skip the kernel probe"
            )


def ensure_kernel_support() -> None:
    if platform.system() != "Linux":
        raise SystemExit("full PPPoE evidence requires Linux")
    if not has_cap_net_admin():
        raise SystemExit("full PPPoE evidence requires root or CAP_NET_ADMIN")

    dev_ppp = Path("/dev/ppp")
    if not dev_ppp.exists() or not stat.S_ISCHR(dev_ppp.stat().st_mode):
        raise SystemExit("missing /dev/ppp character device (CONFIG_PPP)")

    try_load_modules()

    if not (Path("/sys/module/pppoe").exists() or Path("/proc/net/pppoe").exists()):
        raise SystemExit(
            "missing PPPoE kernel support: expected /sys/module/pppoe or /proc/net/pppoe (CONFIG_PPPOE)"
        )


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
    for ns in [ZE_NS, AC_NS]:
        kill_netns_processes(ns, signal.SIGTERM)
    time.sleep(0.2)
    for ns in [ZE_NS, AC_NS]:
        kill_netns_processes(ns, signal.SIGKILL)
    for link in [VETH_ZE, VETH_AC]:
        run(
            ["ip", "link", "delete", link],
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
        )
    for ns in [AC_NS, ZE_NS]:
        run(
            ["ip", "netns", "delete", ns],
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
        )


def setup_netns() -> None:
    cleanup_netns()
    Path("/run/netns").mkdir(parents=True, exist_ok=True)
    run_required(["ip", "netns", "add", ZE_NS], f"create netns {ZE_NS}")
    run_required(["ip", "netns", "add", AC_NS], f"create netns {AC_NS}")
    # A veth pair is a single L2 segment, exactly what PPPoE discovery needs.
    run_required(
        ["ip", "link", "add", VETH_ZE, "type", "veth", "peer", "name", VETH_AC],
        "create PPPoE access-link veth pair",
    )
    run_required(["ip", "link", "set", VETH_ZE, "netns", ZE_NS], "move Ze veth")
    run_required(["ip", "link", "set", VETH_AC, "netns", AC_NS], "move AC veth")
    for ns, veth in [(ZE_NS, VETH_ZE), (AC_NS, VETH_AC)]:
        ns_run_required(
            ns, ["ip", "link", "set", "lo", "up"], f"bring up {ns} loopback"
        )
        ns_run_required(ns, ["ip", "link", "set", veth, "up"], f"bring up {veth}")


class LineCollector:
    def __init__(self, prefix: str, stream) -> None:
        self.prefix = prefix
        self.lines: list[str] = []
        self.cond = threading.Condition()
        threading.Thread(target=self._worker, args=(stream,), daemon=True).start()

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
    ) -> bool:
        deadline = time.time() + timeout_s
        while time.time() < deadline:
            with self.cond:
                if predicate(list(self.lines)):
                    return True
                if proc is not None and proc.poll() is not None:
                    return False
                self.cond.wait(timeout=min(0.2, max(0.0, deadline - time.time())))
        return False


def lines_contain(needle: str) -> Callable[[list[str]], bool]:
    return lambda lines: any(needle in line for line in lines)


def ppp_links(ns: str) -> set[str]:
    links = ns_run(
        ns,
        ["ip", "-o", "link", "show", "type", "ppp"],
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
    )
    found: set[str] = set()
    for line in (links.stdout or "").splitlines():
        match = re.match(r"\d+:\s+([^:@]+)", line)
        if match:
            found.add(match.group(1).strip())
    return found


def wait_ppp_address(
    ns: str, iface: str, local: str, peer: str, timeout_s: float
) -> None:
    # The pppN link appears at SetAdminUp, a beat before AddAddressP2P assigns
    # the address, so poll rather than read once.
    deadline = time.time() + timeout_s
    last = ""
    while time.time() < deadline:
        addr = ns_run(
            ns,
            ["ip", "-o", "addr", "show", "dev", iface],
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
        )
        last = addr.stdout or ""
        if local in last and peer in last:
            return
        time.sleep(0.5)
    raise RuntimeError(
        f"{iface} never got local {local} peer {peer} within {timeout_s}s:\n{last}"
    )


def wait_new_ppp(
    ns: str, initial: set[str], role: str, timeout_s: float, proc: subprocess.Popen[str]
) -> str:
    # Kernel ground truth: the pppN interface exists only after discovery, LCP,
    # auth, and IPCP all complete, so its appearance is the session-up proof.
    deadline = time.time() + timeout_s
    while time.time() < deadline:
        new_links = sorted(ppp_links(ns) - initial)
        if len(new_links) == 1:
            return new_links[0]
        if len(new_links) > 1:
            raise RuntimeError(
                f"more than one new PPP interface in {role} namespace: {', '.join(new_links)}"
            )
        if proc.poll() is not None:
            raise RuntimeError(
                f"{role} process exited before a pppN interface appeared"
            )
        time.sleep(0.5)
    raise RuntimeError(
        f"no pppN interface appeared in {role} namespace within {timeout_s}s"
    )


def wait_links_cleared(ns: str, initial: set[str], timeout_s: float) -> None:
    deadline = time.time() + timeout_s
    while time.time() < deadline:
        if ppp_links(ns) == initial:
            return
        time.sleep(0.2)
    raise RuntimeError(f"PPP links in {ns} did not return to {sorted(initial)}")


def write_accel_conf(work: Path) -> Path:
    (work / "chap-secrets").write_text(
        f"{USERNAME}\t*\t{PASSWORD}\t*\n", encoding="utf-8"
    )
    (work / "chap-secrets").chmod(0o600)
    conf = work / "accel-ppp.conf"
    conf.write_text(
        "[modules]\n"
        "log_file\n"
        "pppoe\n"
        "auth_chap_md5\n"
        "chap-secrets\n"
        "ippool\n"
        "\n"
        "[core]\n"
        "thread-count=1\n"
        "\n"
        "[log]\n"
        f"log-file={work / 'accel.log'}\n"
        "level=4\n"
        "\n"
        "[pppoe]\n"
        f"interface={VETH_AC}\n"
        "ac-name=ze-accel-ac\n"
        f"service-name={SERVICE_NAME}\n"
        "verbose=1\n"
        "\n"
        "[ppp]\n"
        "verbose=1\n"
        "mtu=1492\n"
        "mru=1492\n"
        "ipv4=require\n"
        "ipv6=deny\n"
        "lcp-echo-interval=30\n"
        "lcp-echo-failure=3\n"
        "mppe=deny\n"
        "\n"
        "[ip-pool]\n"
        f"gw-ip-address={PEER_ADDR}\n"
        f"{LOCAL_ADDR}-10\n"
        "\n"
        "[chap-secrets]\n"
        f"chap-secrets={work / 'chap-secrets'}\n",
        encoding="utf-8",
    )
    return conf


def write_ze_conf(work: Path) -> Path:
    conf = work / "ze.conf"
    conf.write_text(
        "interface {\n"
        f"    pppoe-client pppoe0 {{\n"
        f"        source-interface {VETH_ZE};\n"
        f"        service-name {SERVICE_NAME};\n"
        "        authentication {\n"
        f"            username {USERNAME};\n"
        f"            password {PASSWORD};\n"
        "        }\n"
        "    }\n"
        "}\n",
        encoding="utf-8",
    )
    return conf


def ensure_ze(root: Path) -> Path:
    override = os.environ.get("ZE_EVIDENCE_ZE_BINARY") or os.environ.get("ZE_BIN")
    if override:
        ze = Path(override)
        if not ze.is_file() or not os.access(ze, os.X_OK):
            raise SystemExit(f"ze binary override is not an executable file: {ze}")
        return ze
    require_cmd("go")
    bindir = root / "tmp" / "evidence" / "bin"
    bindir.mkdir(parents=True, exist_ok=True)
    ze = bindir / "ze-pppoe-accel"
    env = os.environ.copy()
    env.setdefault("GOCACHE", str(root / "tmp" / "go-cache"))
    build = run(
        ["go", "build", "-tags", "ze_core,ze_distro", "-o", str(ze), "./cmd/ze"],
        cwd=root,
        env=env,
    )
    if build.returncode != 0:
        raise SystemExit("go build ./cmd/ze failed")
    return ze


def main() -> int:
    reject_skip_kernel_probe_env()
    if platform.system() != "Linux":
        raise SystemExit("full PPPoE evidence requires Linux")
    require_cmd("ip")
    require_cmd("ping")
    require_cmd("accel-pppd")
    ensure_kernel_support()

    root = repo_root()
    ze = ensure_ze(root)
    tmp_parent = root / "tmp" / "evidence"
    tmp_parent.mkdir(parents=True, exist_ok=True)
    work = Path(tempfile.mkdtemp(prefix="effective-pppoe-accel-", dir=tmp_parent))
    accel_conf = write_accel_conf(work)
    ze_conf = write_ze_conf(work)

    ze_proc: subprocess.Popen[str] | None = None
    accel: subprocess.Popen[str] | None = None
    success = False
    try:
        setup_netns()
        initial_ze = ppp_links(ZE_NS)
        initial_ac = ppp_links(AC_NS)

        # Start the access concentrator first; Ze's dialer retries discovery, so
        # a brief startup race is absorbed by its reconnect loop.
        accel = ns_popen(
            AC_NS,
            ["accel-pppd", "-c", str(accel_conf), "-p", str(work / "accel.pid")],
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            text=True,
            bufsize=1,
        )
        LineCollector("accel> ", accel.stdout)
        LineCollector("accel-err> ", accel.stderr)
        time.sleep(2.0)
        if accel.poll() is not None:
            raise RuntimeError("accel-pppd exited during startup")

        env = os.environ.copy()
        env["ze.log.interface"] = (
            "debug"  # pppoe-client logs under the "interface" domain
        )
        env["ZE_STORAGE_BLOB"] = "false"
        env["ZE_CONFIG_DIR"] = str(work / "ze")
        for key in ["ZE_PPPOE_SKIP_KERNEL_PROBE", "ze.pppoe.skip-kernel-probe"]:
            env.pop(key, None)

        # `start <config>`: the bare `ze <config>` launch form was removed from the
        # CLI (learned 1248), so a positional path now dies with "unknown command".
        ze_proc = ns_popen(
            ZE_NS,
            [str(ze), "start", str(ze_conf)],
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            text=True,
            bufsize=1,
            env=env,
        )
        # ze stderr is captured for diagnostics; ze.log.interface=debug surfaces
        # the discovery/LCP/CHAP/IPCP phase logs there. Success is gated on kernel
        # ground truth (the pppN interface + assigned address), not log scraping.
        LineCollector("ze> ", ze_proc.stderr)
        LineCollector("ze-out> ", ze_proc.stdout)

        # The pppN interface appears only after the full PPPoE + PPP handshake
        # (discovery, LCP, CHAP-MD5 auth, IPCP) completes against accel-ppp.
        ze_iface = wait_new_ppp(ZE_NS, initial_ze, "Ze", 75, ze_proc)
        ac_iface = wait_new_ppp(AC_NS, initial_ac, "accel-ppp", 30, accel)
        wait_ppp_address(ZE_NS, ze_iface, LOCAL_ADDR, PEER_ADDR, 20)
        wait_ppp_address(AC_NS, ac_iface, PEER_ADDR, LOCAL_ADDR, 20)

        ping = ns_run(
            ZE_NS,
            ["ping", "-c", "2", "-W", "3", PEER_ADDR],
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
        )
        if ping.returncode != 0:
            raise RuntimeError(
                f"dataplane ping to AC gateway {PEER_ADDR} through {ze_iface} failed"
            )

        # Stop the client; both ends must release the kernel PPP interface.
        terminate(ze_proc)
        ze_proc = None
        wait_links_cleared(ZE_NS, initial_ze, 20)
        wait_links_cleared(AC_NS, initial_ac, 20)

        print(
            f"OK: ze pppoe-client completed discovery, CHAP, IPCP against accel-ppp; "
            f"Ze {ze_iface} ({LOCAL_ADDR}) and accel {ac_iface} ({PEER_ADDR}) up, "
            f"dataplane ping ok, clean teardown"
        )
        success = True
        return 0
    except RuntimeError as err:
        sys.stderr.write(f"FAIL: {err}\n")
        sys.stderr.write("\n--- diagnostics ---\n")
        for ns_name in [ZE_NS, AC_NS]:
            r = ns_run(
                ns_name,
                ["ip", "-o", "link", "show", "type", "ppp"],
                stdout=subprocess.PIPE,
                stderr=subprocess.PIPE,
            )
            sys.stderr.write(
                f"{ns_name} ppp links: {(r.stdout or '').strip() or '(none)'}\n"
            )
        accel_log = work / "accel.log"
        if accel_log.is_file():
            sys.stderr.write(
                "\naccel.log tail:\n"
                + "".join(accel_log.read_text().splitlines(keepends=True)[-40:])
            )
        return 1
    finally:
        terminate(ze_proc)
        terminate(accel)
        cleanup_netns()
        if success:
            shutil.rmtree(work, ignore_errors=True)


if __name__ == "__main__":
    raise SystemExit(main())
