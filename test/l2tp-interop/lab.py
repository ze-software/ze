#!/usr/bin/env python3
"""Docker lifecycle and helpers for Ze L2TP PPP/NCP interop lab.

Manages container creation, network setup, log collection, and daemon
helpers for Ze (LNS), xl2tpd/pppd (LAC), and FRR (BGP peer).
"""

import atexit
import json
import os
import re
import subprocess
import sys
import time

_SUFFIX = os.environ.get("ZE_L2TP_INTEROP_SUFFIX", str(os.getpid()))
NETWORK = "ze-l2tp-%s" % _SUFFIX
ZE_CONTAINER = "ze-l2tp-ze-%s" % _SUFFIX
LAC_CONTAINER = "ze-l2tp-lac-%s" % _SUFFIX
FRR_CONTAINER = "ze-l2tp-frr-%s" % _SUFFIX

SUBNET = "172.29.0.0/24"
ZE_IP = "172.29.0.2"
LAC_IP = "172.29.0.3"
FRR_IP = "172.29.0.4"

try:
    SESSION_TIMEOUT = int(os.environ.get("SESSION_TIMEOUT", "90"))
except ValueError:
    SESSION_TIMEOUT = 90

VERBOSE = os.environ.get("VERBOSE", "0") == "1"


# --- Logging ----------------------------------------------------------------


def log_info(msg):
    print("  %s" % msg)


def log_pass(msg):
    print("  \033[32m✓ %s\033[0m" % msg)


def log_fail(msg):
    print("  \033[31m✗ %s\033[0m" % msg)


def log_debug(msg):
    if VERBOSE:
        print("  [debug] %s" % msg)


# --- Docker helpers ----------------------------------------------------------


def docker_exec(container, cmd, timeout=30):
    try:
        result = subprocess.run(
            ["docker", "exec", container] + cmd,
            capture_output=True,
            text=True,
            timeout=timeout,
        )
    except subprocess.TimeoutExpired:
        raise RuntimeError(
            "docker exec %s %s timed out after %ds"
            % (container, " ".join(cmd), timeout)
        )
    if result.returncode != 0:
        raise RuntimeError(
            "docker exec %s %s failed (rc=%d): %s"
            % (container, " ".join(cmd), result.returncode, result.stderr.strip())
        )
    return result.stdout


def docker_exec_quiet(container, cmd, timeout=30):
    try:
        result = subprocess.run(
            ["docker", "exec", container] + cmd,
            capture_output=True,
            text=True,
            timeout=timeout,
        )
        if result.returncode != 0:
            return ""
        return result.stdout
    except Exception:
        return ""


def docker_run(name, image, ip, volumes=None, caps=None, extra_args=None, cmd=None):
    args = ["docker", "run", "-d", "--name", name, "--network", NETWORK, "--ip", ip]
    for cap in caps or []:
        args.extend(["--cap-add", cap])
    for vol in volumes or []:
        args.extend(["-v", vol])
    for arg in extra_args or []:
        args.append(arg)
    args.append(image)
    for c in cmd or []:
        args.append(c)
    try:
        result = subprocess.run(args, capture_output=True, text=True, timeout=60)
    except subprocess.TimeoutExpired:
        raise RuntimeError("docker run %s timed out after 60s" % name)
    if result.returncode != 0:
        raise RuntimeError("docker run %s failed: %s" % (name, result.stderr.strip()))


def docker_rm(name):
    subprocess.run(
        ["docker", "rm", "-f", name],
        capture_output=True,
        text=True,
        timeout=30,
    )


def docker_logs(container, lines=50):
    try:
        result = subprocess.run(
            ["docker", "logs", container, "--tail", str(lines)],
            capture_output=True,
            text=True,
            timeout=15,
        )
        return result.stdout + result.stderr
    except subprocess.TimeoutExpired:
        return "(docker logs timed out)"


def docker_logs_all(container):
    try:
        result = subprocess.run(
            ["docker", "logs", container],
            capture_output=True,
            text=True,
            timeout=30,
        )
        return result.stdout + result.stderr
    except subprocess.TimeoutExpired:
        return "(docker logs timed out)"


# --- Preflight ---------------------------------------------------------------


def preflight_strict():
    """Verify PPPoL2TP host-kernel support from inside a privileged container.

    Exits non-zero with a clear message when requirements are missing.
    This runs a temporary Alpine container with --privileged to probe the
    host kernel, which is what the lab containers will share.
    """
    for key in ["ZE_L2TP_SKIP_KERNEL_PROBE", "ze.l2tp.skip-kernel-probe"]:
        if key in os.environ:
            raise SystemExit(
                "refusing to run with %s set; full proof must not skip the kernel probe"
                % key
            )

    probe_name = "ze-l2tp-preflight-%s" % _SUFFIX
    modules_mount = (
        ["-v", "/lib/modules:/lib/modules:ro"] if os.path.isdir("/lib/modules") else []
    )
    try:
        result = subprocess.run(
            [
                "docker",
                "run",
                "--rm",
                "--privileged",
                "--name",
                probe_name,
            ]
            + modules_mount
            + [
                "alpine:3.21",
                "sh",
                "-c",
                "apk add --no-cache -q iproute2 kmod > /dev/null 2>&1 && "
                "modprobe ppp_generic 2>/dev/null; "
                "modprobe l2tp_ppp 2>/dev/null; "
                "modprobe pppol2tp 2>/dev/null; "
                "echo DEV_PPP=$(test -c /dev/ppp && echo ok || echo missing); "
                "echo L2TP_PPP=$(test -d /sys/module/l2tp_ppp -o -d /sys/module/pppol2tp -o -f /proc/net/pppol2tp && echo ok || echo missing); "
                "echo IP_L2TP=$(ip l2tp show tunnel > /dev/null 2>&1 && echo ok || echo missing)",
            ],
            capture_output=True,
            text=True,
            timeout=120,
        )
    except subprocess.TimeoutExpired:
        raise SystemExit("preflight probe container timed out")

    if result.returncode != 0:
        raise SystemExit(
            "preflight probe failed (rc=%d): %s"
            % (result.returncode, result.stderr.strip())
        )

    output = result.stdout
    checks = {}
    for line in output.splitlines():
        if "=" in line:
            k, v = line.strip().split("=", 1)
            checks[k] = v

    missing = []
    if checks.get("DEV_PPP") != "ok":
        missing.append("/dev/ppp (PPP character device)")
    if checks.get("L2TP_PPP") != "ok":
        missing.append("l2tp_ppp/pppol2tp kernel module")
    if checks.get("IP_L2TP") != "ok":
        missing.append("ip l2tp (L2TP Generic Netlink)")

    if missing:
        raise SystemExit(
            "host kernel missing PPPoL2TP requirements: %s" % ", ".join(missing)
        )

    log_pass("preflight: host kernel has PPPoL2TP support")


# --- Ze helpers --------------------------------------------------------------


def wait_ze_log(needle, timeout=None):
    if timeout is None:
        timeout = SESSION_TIMEOUT
    log_info("waiting for Ze log: '%s' (timeout %ds)..." % (needle, timeout))
    deadline = time.time() + timeout
    while time.time() < deadline:
        logs = docker_logs_all(ZE_CONTAINER)
        if needle in logs:
            log_pass("Ze log contains: %s" % needle)
            return
        time.sleep(2)
    log_fail("Ze log missing: '%s' within %ds" % (needle, timeout))
    raise AssertionError("Ze log missing: %s" % needle)


def ze_log_contains(needle):
    logs = docker_logs_all(ZE_CONTAINER)
    return needle in logs


# --- LAC helpers -------------------------------------------------------------


def wait_lac_ready(timeout=30):
    log_info("waiting for LAC container to start xl2tpd...")
    deadline = time.time() + timeout
    while time.time() < deadline:
        logs = docker_logs(LAC_CONTAINER, 100)
        if "xl2tpd" in logs.lower() or "listening" in logs.lower():
            log_debug("LAC container producing logs")
            return
        time.sleep(1)
    log_debug(
        "LAC container logs (may still be starting): %s"
        % docker_logs(LAC_CONTAINER, 10)
    )


# --- FRR helpers -------------------------------------------------------------


class FRR:
    def __init__(self, container=FRR_CONTAINER, ip=FRR_IP):
        self.container = container
        self.ip = ip

    def _vtysh_quiet(self, command):
        return docker_exec_quiet(self.container, ["vtysh", "-c", command])

    def wait_session(self, neighbor, timeout=None):
        if timeout is None:
            timeout = SESSION_TIMEOUT
        log_info(
            "waiting for FRR session with %s (timeout %ds)..." % (neighbor, timeout)
        )
        deadline = time.time() + timeout
        while time.time() < deadline:
            output = self._vtysh_quiet("show bgp neighbor %s" % neighbor)
            if "BGP state = Established" in output:
                log_pass("FRR session with %s is Established" % neighbor)
                return
            time.sleep(2)
        log_fail(
            "FRR session with %s did not reach Established within %ds"
            % (neighbor, timeout)
        )
        output = self._vtysh_quiet("show bgp neighbor %s" % neighbor)
        for line in output.splitlines()[:10]:
            print("  %s" % line)
        print(docker_logs(ZE_CONTAINER, 20))
        raise AssertionError("FRR session with %s not Established" % neighbor)

    def has_route(self, prefix, family="ipv4 unicast"):
        output = self._vtysh_quiet("show bgp %s %s json" % (family, prefix))
        if not output.strip():
            return False
        try:
            data = json.loads(output)
        except json.JSONDecodeError:
            return False
        if "paths" in data or "prefix" in data:
            return True
        for v in data.values():
            if isinstance(v, dict) and ("paths" in v or "prefix" in v):
                return True
        return False

    def wait_route(self, prefix, timeout=30, family="ipv4 unicast"):
        deadline = time.time() + timeout
        while time.time() < deadline:
            if self.has_route(prefix, family):
                log_pass("FRR has route %s" % prefix)
                return
            time.sleep(2)
        log_fail("FRR route %s did not appear within %ds" % (prefix, timeout))
        raise AssertionError("FRR route %s not found" % prefix)

    def wait_route_absent(self, prefix, timeout=30, family="ipv4 unicast"):
        deadline = time.time() + timeout
        while time.time() < deadline:
            if not self.has_route(prefix, family):
                log_pass("FRR route %s withdrawn" % prefix)
                return
            time.sleep(2)
        log_fail("FRR route %s still present after %ds" % (prefix, timeout))
        raise AssertionError("FRR route %s still present" % prefix)

    def check_route(self, prefix, family="ipv4 unicast"):
        if self.has_route(prefix, family):
            log_pass("FRR has route %s" % prefix)
            return
        log_fail("FRR does not have route %s" % prefix)
        raise AssertionError("FRR missing route %s" % prefix)

    def session_established(self, neighbor):
        output = self._vtysh_quiet("show bgp neighbor %s" % neighbor)
        return "BGP state = Established" in output


# --- PPP/L2TP verification helpers -------------------------------------------


def ze_ppp_links():
    output = docker_exec_quiet(
        ZE_CONTAINER, ["ip", "-o", "link", "show", "type", "ppp"]
    )
    found = set()
    for line in output.splitlines():
        match = re.match(r"\d+:\s+([^:@]+)", line)
        if match:
            found.add(match.group(1))
    return found


def ze_l2tp_tunnels():
    return docker_exec_quiet(ZE_CONTAINER, ["ip", "l2tp", "show", "tunnel"]).strip()


def lac_ppp_links():
    output = docker_exec_quiet(
        LAC_CONTAINER, ["ip", "-o", "link", "show", "type", "ppp"]
    )
    found = set()
    for line in output.splitlines():
        match = re.match(r"\d+:\s+([^:@]+)", line)
        if match:
            found.add(match.group(1))
    return found


def ze_ppp_addr(iface):
    return docker_exec_quiet(ZE_CONTAINER, ["ip", "-o", "addr", "show", "dev", iface])


def lac_ping(target, count=2):
    try:
        result = subprocess.run(
            [
                "docker",
                "exec",
                LAC_CONTAINER,
                "ping",
                "-c",
                str(count),
                "-W",
                "3",
                target,
            ],
            capture_output=True,
            text=True,
            timeout=30,
        )
        return result.returncode == 0
    except subprocess.TimeoutExpired:
        return False


def wait_ppp_up(timeout=None):
    if timeout is None:
        timeout = SESSION_TIMEOUT
    log_info("waiting for PPP interface in Ze container (timeout %ds)..." % timeout)
    deadline = time.time() + timeout
    while time.time() < deadline:
        links = ze_ppp_links()
        if links:
            iface = sorted(links)[0]
            log_pass("Ze has PPP interface: %s" % iface)
            return iface
        time.sleep(2)
    log_fail("no PPP interface appeared in Ze within %ds" % timeout)
    raise AssertionError("no PPP interface in Ze")


def wait_l2tp_clean(timeout=30):
    log_info("waiting for L2TP/PPP cleanup (timeout %ds)..." % timeout)
    deadline = time.time() + timeout
    while time.time() < deadline:
        ze_links = ze_ppp_links()
        ze_tunnels = ze_l2tp_tunnels()
        lac_links = lac_ppp_links()
        lac_tunnels = docker_exec_quiet(
            LAC_CONTAINER, ["ip", "l2tp", "show", "tunnel"]
        ).strip()
        if not ze_links and not ze_tunnels and not lac_links and not lac_tunnels:
            log_pass("L2TP/PPP state clean in both containers")
            return
        time.sleep(1)
    log_fail("L2TP/PPP cleanup did not complete within %ds" % timeout)
    log_info("Ze PPP links: %s, tunnels: %s" % (ze_ppp_links(), ze_l2tp_tunnels()))
    log_info("LAC PPP links: %s" % lac_ppp_links())
    raise AssertionError("L2TP/PPP cleanup timeout")


# --- Scenario lifecycle ------------------------------------------------------


class Scenario:
    def __init__(self, scenario_dir, frr_image):
        self.scenario_dir = scenario_dir
        self.frr_image = frr_image
        self.name = os.path.basename(scenario_dir.rstrip("/"))

    def setup(self):
        self.teardown()

        result = subprocess.run(
            ["docker", "network", "create", "--subnet=%s" % SUBNET, NETWORK],
            capture_output=True,
            text=True,
            timeout=30,
        )
        if result.returncode != 0 and "already exists" not in result.stderr:
            raise RuntimeError(
                "docker network create failed: %s" % result.stderr.strip()
            )

        script_dir = os.path.dirname(os.path.abspath(__file__))
        ze_conf = os.path.join(self.scenario_dir, "ze.conf")
        if not os.path.isfile(ze_conf):
            raise RuntimeError("missing ze.conf in %s" % self.name)

        ze_volumes = [
            "%s:/etc/ze/ze.conf:ro" % os.path.abspath(ze_conf),
        ]
        if os.path.isdir("/lib/modules"):
            ze_volumes.append("/lib/modules:/lib/modules:ro")

        docker_run(
            ZE_CONTAINER,
            "ze-l2tp-interop",
            ZE_IP,
            volumes=ze_volumes,
            extra_args=[
                "--privileged",
                "-e",
                "ZE_LOG_L2TP=debug",
                "-e",
                "ZE_STORAGE_BLOB=false",
                "-e",
                "ze.l2tp.ncp.enable-ipv6cp=false",
                "-e",
                "ze.l2tp.ncp.ip-timeout=15s",
                "-e",
                "ze.l2tp.auth.timeout=15s",
            ],
            cmd=["/etc/ze/ze.conf"],
        )

        frr_conf = os.path.join(self.scenario_dir, "frr.conf")
        if os.path.isfile(frr_conf):
            docker_run(
                FRR_CONTAINER,
                self.frr_image,
                FRR_IP,
                volumes=[
                    "%s:/etc/frr/frr.conf:ro" % os.path.abspath(frr_conf),
                    "%s/daemons:/etc/frr/daemons:ro" % script_dir,
                    "%s/vtysh.conf:/etc/frr/vtysh.conf:ro" % script_dir,
                ],
                caps=["NET_ADMIN", "SYS_ADMIN"],
            )

        wait_ze_log("L2TP listener bound", timeout=30)

        xl2tpd_conf = os.path.join(self.scenario_dir, "xl2tpd.conf")
        ppp_options = os.path.join(self.scenario_dir, "ppp-options")
        l2tp_secrets = os.path.join(self.scenario_dir, "l2tp-secrets")

        lac_volumes = []
        if os.path.isdir("/lib/modules"):
            lac_volumes.append("/lib/modules:/lib/modules:ro")
        if os.path.isfile(xl2tpd_conf):
            lac_volumes.append(
                "%s:/etc/xl2tpd/xl2tpd.conf:ro" % os.path.abspath(xl2tpd_conf)
            )
        if os.path.isfile(ppp_options):
            lac_volumes.append(
                "%s:/etc/ppp/options.l2tpd.client:ro" % os.path.abspath(ppp_options)
            )
        if os.path.isfile(l2tp_secrets):
            lac_volumes.append(
                "%s:/etc/xl2tpd/l2tp-secrets:ro" % os.path.abspath(l2tp_secrets)
            )

        docker_run(
            LAC_CONTAINER,
            "ze-l2tp-lac",
            LAC_IP,
            volumes=lac_volumes,
            extra_args=["--privileged"],
        )

        wait_lac_ready(timeout=15)

    def teardown(self):
        docker_rm(ZE_CONTAINER)
        docker_rm(LAC_CONTAINER)
        docker_rm(FRR_CONTAINER)
        subprocess.run(
            ["docker", "network", "rm", NETWORK],
            capture_output=True,
            text=True,
            timeout=30,
        )

    def dump_logs(self, lines=80):
        print("\n--- Ze logs ---")
        print(docker_logs(ZE_CONTAINER, lines))
        print("\n--- LAC logs ---")
        print(docker_logs(LAC_CONTAINER, lines))
        frr_conf = os.path.join(self.scenario_dir, "frr.conf")
        if os.path.isfile(frr_conf):
            print("\n--- FRR logs ---")
            print(docker_logs(FRR_CONTAINER, lines))

    def run_check(self):
        check_path = os.path.join(self.scenario_dir, "check.py")
        if not os.path.isfile(check_path):
            raise RuntimeError("no check.py in %s" % self.name)

        import importlib.util

        spec = importlib.util.spec_from_file_location("check", check_path)
        if spec is None:
            raise RuntimeError("cannot load check.py from %s" % self.name)
        mod = importlib.util.module_from_spec(spec)
        try:
            spec.loader.exec_module(mod)
        except Exception as e:
            raise RuntimeError(
                "check.py in %s failed to load: %s" % (self.name, e)
            ) from e
        if not hasattr(mod, "check"):
            raise RuntimeError("check.py in %s has no check() function" % self.name)
        mod.check()


def global_cleanup():
    for name in [ZE_CONTAINER, LAC_CONTAINER, FRR_CONTAINER]:
        subprocess.run(
            ["docker", "rm", "-f", name], capture_output=True, text=True, timeout=30
        )
    subprocess.run(
        ["docker", "network", "rm", NETWORK], capture_output=True, text=True, timeout=30
    )


atexit.register(global_cleanup)
