#!/usr/bin/env python3
"""Shared helpers for Ze BGP stress tests.

Orchestrates the ze-vs-injector network namespace pair, starts the DUT
(ze or bird), and spawns `bin/ze-test peer --mode inject --dial` to
generate and stream the BGP UPDATE traffic.

Architecture:
    [ze-ns]                        [bb-ns]
    ze listens 172.31.0.2:179      ze-test peer --dial 172.31.0.2:179
    ze-veth                        bb-veth (kernel TCP stack)
         \\______________________/
                  veth pair
"""

import atexit
import os
import re
import shutil
import subprocess
import time

# Namespace and interface naming (PID suffix avoids conflicts).
_SUFFIX = os.environ.get("ZE_STRESS_SUFFIX", str(os.getpid()))
ZE_NS = "ze-stress-ze-%s" % _SUFFIX
BB_NS = "ze-stress-bb-%s" % _SUFFIX
ZE_VETH = "ze-v-%s" % _SUFFIX[:6]
BB_VETH = "bb-v-%s" % _SUFFIX[:6]

# IP addresses.
ZE_IP = "172.31.0.2"
BB_IP = "172.31.0.3"
SUBNET = "172.31.0.0/24"

# Default timeout for session establishment (seconds).
try:
    SESSION_TIMEOUT = int(os.environ.get("SESSION_TIMEOUT", "120"))
except ValueError:
    SESSION_TIMEOUT = 120

VERBOSE = os.environ.get("VERBOSE", "0") == "1"

# Process references for cleanup.
_processes = []


# --- Logging ----------------------------------------------------------------


def log_info(msg):
    print("  %s" % msg)


def log_pass(msg):
    print("  \033[32m\u2713 %s\033[0m" % msg)


def log_fail(msg):
    print("  \033[31m\u2717 %s\033[0m" % msg)


def log_debug(msg):
    if VERBOSE:
        print("  [debug] %s" % msg)


# --- Shell helpers -----------------------------------------------------------


def _run(cmd, check=True, timeout=30, capture=True):
    """Run a command, return CompletedProcess."""
    log_debug("$ %s" % " ".join(cmd))
    try:
        return subprocess.run(
            cmd,
            capture_output=capture,
            text=True,
            timeout=timeout,
        )
    except subprocess.TimeoutExpired:
        raise RuntimeError(
            "command timed out after %ds: %s" % (timeout, " ".join(cmd[:4]))
        )


def _run_ok(cmd, timeout=30):
    """Run a command, return stdout or empty string on failure."""
    try:
        result = _run(cmd, check=False, timeout=timeout)
        if result.returncode != 0:
            return ""
        return result.stdout
    except Exception:
        return ""


def _nsexec(ns, cmd, **kwargs):
    """Run a command in a network namespace."""
    return _run(["ip", "netns", "exec", ns] + cmd, **kwargs)


def _nsexec_ok(ns, cmd, **kwargs):
    """Run a command in a network namespace, return stdout or empty."""
    return _run_ok(["ip", "netns", "exec", ns] + cmd, **kwargs)


# --- Network namespace management --------------------------------------------


def create_netns_pair():
    """Create two network namespaces connected by a veth pair.

    Both sides use the kernel TCP stack (ze and ze-test peer).
    """
    log_info("creating network namespaces and veth pair...")

    _run(["ip", "netns", "add", ZE_NS])
    _run(["ip", "netns", "add", BB_NS])
    _run(["ip", "link", "add", ZE_VETH, "type", "veth", "peer", "name", BB_VETH])
    _run(["ip", "link", "set", ZE_VETH, "netns", ZE_NS])
    _run(["ip", "link", "set", BB_VETH, "netns", BB_NS])

    _nsexec(ZE_NS, ["ip", "addr", "add", "%s/24" % ZE_IP, "dev", ZE_VETH])
    _nsexec(ZE_NS, ["ip", "link", "set", ZE_VETH, "up"])
    _nsexec(ZE_NS, ["ip", "link", "set", "lo", "up"])
    _nsexec(BB_NS, ["ip", "addr", "add", "%s/24" % BB_IP, "dev", BB_VETH])
    _nsexec(BB_NS, ["ip", "link", "set", BB_VETH, "up"])
    _nsexec(BB_NS, ["ip", "link", "set", "lo", "up"])

    # Checksum offload off on both ends -- harmless with kernel TCP,
    # historically required by BNG Blaster's LwIP stack.
    _nsexec(ZE_NS, ["ethtool", "-K", ZE_VETH, "tx", "off", "rx", "off"], check=False)
    _nsexec(BB_NS, ["ethtool", "-K", BB_VETH, "tx", "off", "rx", "off"], check=False)

    log_pass("namespaces ready: %s (%s) <-> %s (%s)" % (ZE_NS, ZE_IP, BB_NS, BB_IP))


def destroy_netns_pair():
    """Remove namespaces, stop processes, clean up temp files."""
    for proc in _processes:
        try:
            proc.terminate()
            proc.wait(timeout=5)
        except Exception:
            try:
                proc.kill()
            except Exception:
                pass
    _processes.clear()

    _run(["ip", "netns", "del", ZE_NS], check=False)
    _run(["ip", "netns", "del", BB_NS], check=False)

    for path in [
        "/tmp/ze-stress-ze-%s.log" % _SUFFIX,
        "/tmp/ze-stress-peer-%s.log" % _SUFFIX,
        "/tmp/ze-stress-pcap-%s.txt" % _SUFFIX,
        "/tmp/ze-stress-bird-%s.log" % _SUFFIX,
        "/tmp/ze-stress-bird-%s.pid" % _SUFFIX,
        "/tmp/ze-stress-bird-%s.ctl" % _SUFFIX,
    ]:
        try:
            os.unlink(path)
        except OSError:
            pass


# --- Process management ------------------------------------------------------


def start_ze(ze_binary, config_path):
    """Start Ze in the ze-ns namespace. Returns Popen."""
    log_info("starting Ze in %s..." % ZE_NS)
    ze_log = "/tmp/ze-stress-ze-%s.log" % _SUFFIX
    ze_logfile = open(ze_log, "w")
    # Enable reactor debug logging so failures are visible.
    ze_env = os.environ.copy()
    ze_env["ze.log.bgp.reactor"] = "info"
    ze_env["ze.log.plugin"] = "info"
    cmd = ["ip", "netns", "exec", ZE_NS, ze_binary]
    if os.environ.get("ZE_PPROF"):
        cmd.extend(["--pprof", "127.0.0.1:6060"])
    cmd.append(config_path)
    proc = subprocess.Popen(
        cmd,
        stdout=ze_logfile,
        stderr=subprocess.STDOUT,
        env=ze_env,
    )
    proc._ze_logfile = ze_logfile  # prevent GC close
    proc._ze_log = ze_log
    _processes.append(proc)
    # Give Ze a moment to start and bind port 179.
    time.sleep(2)
    if proc.poll() is not None:
        ze_logfile.flush()
        with open(ze_log) as f:
            output = f.read()
        raise RuntimeError(
            "Ze exited immediately (rc=%d): %s" % (proc.returncode, output[:1000])
        )
    # Verify port 179 is listening.
    ss = _nsexec_ok(ZE_NS, ["ss", "-tlnp", "sport", "=", "179"])
    if "LISTEN" in ss:
        log_pass("Ze started (pid %d), port 179 listening" % proc.pid)
        log_debug("listeners: %s" % ss.strip())
    else:
        log_info("Ze started (pid %d) but port 179 NOT YET listening" % proc.pid)
        log_debug("ss output: %s" % ss.strip())
    # Start background packet capture on ze-veth for post-mortem analysis.
    pcap_file = "/tmp/ze-stress-pcap-%s.txt" % _SUFFIX
    tcpdump_proc = subprocess.Popen(
        [
            "ip",
            "netns",
            "exec",
            ZE_NS,
            "tcpdump",
            "-i",
            ZE_VETH,
            "-nn",
            "-l",
            "-c",
            "100",
            "tcp",
            "port",
            "179",
        ],
        stdout=open(pcap_file, "w"),
        stderr=subprocess.STDOUT,
    )
    tcpdump_proc._pcap_file = pcap_file
    _processes.append(tcpdump_proc)
    log_debug("Ze log: %s" % ze_log)
    return proc


# --- Peer inject orchestration ----------------------------------------------


def _project_root():
    return os.path.abspath(os.path.join(os.path.dirname(__file__), "..", ".."))


def _ze_test_binary():
    """Return path to bin/ze-test, raising if missing."""
    candidate = os.path.join(_project_root(), "bin", "ze-test")
    if not os.path.isfile(candidate):
        raise RuntimeError(
            "bin/ze-test not found at %s -- run 'make ze-test' first" % candidate
        )
    return candidate


def start_peer_inject(
    prefix_base,
    prefix_count,
    nexthop,
    asn,
    dwell="30s",
    dial_host=None,
    dial_port=179,
):
    """Spawn ze-test peer --mode inject --dial in bb-ns.

    Returns Popen. The peer dials `dial_host:dial_port` (ze by default),
    completes the BGP handshake, streams the built UPDATE image, then
    holds the session open for `dwell` before exiting cleanly.
    """
    ze_test = _ze_test_binary()
    dial_host = dial_host or ZE_IP
    log_info(
        "starting ze-test peer (inject %d prefixes from %s -> %s:%d)..."
        % (prefix_count, prefix_base, dial_host, dial_port)
    )
    peer_log = "/tmp/ze-stress-peer-%s.log" % _SUFFIX
    peer_logfile = open(peer_log, "w")
    cmd = [
        "ip",
        "netns",
        "exec",
        BB_NS,
        ze_test,
        "peer",
        "--mode",
        "inject",
        "--dial",
        "%s:%d" % (dial_host, dial_port),
        "--inject-prefix",
        prefix_base,
        "--inject-count",
        str(prefix_count),
        "--inject-nexthop",
        nexthop,
        "--inject-asn",
        str(asn),
        "--inject-dwell",
        dwell,
    ]
    proc = subprocess.Popen(
        cmd,
        stdout=peer_logfile,
        stderr=subprocess.STDOUT,
    )
    proc._peer_logfile = peer_logfile
    proc._peer_log = peer_log
    _processes.append(proc)
    return proc


def wait_peer_done(proc, timeout=1800):
    """Wait for the inject peer to exit. Tail its log on failure."""
    try:
        rc = proc.wait(timeout=timeout)
    except subprocess.TimeoutExpired:
        log_fail("peer did not exit within %ds, killing" % timeout)
        proc.kill()
        proc.wait(timeout=5)
        _dump_peer_log(proc)
        raise AssertionError("peer inject timeout")
    if rc != 0:
        log_fail("peer exited rc=%d" % rc)
        _dump_peer_log(proc)
        raise AssertionError("peer inject failed (rc=%d)" % rc)
    log_pass("peer inject complete")
    return _parse_peer_metrics(proc)


def _dump_peer_log(proc):
    log_path = getattr(proc, "_peer_log", None)
    if log_path and os.path.isfile(log_path):
        with open(log_path) as f:
            tail = f.readlines()[-40:]
        log_info("peer log tail:")
        for line in tail:
            print("    %s" % line.rstrip())


def _parse_peer_metrics(proc):
    """Parse 'inject built:' / 'inject sent:' lines from peer stdout."""
    log_path = getattr(proc, "_peer_log", None)
    metrics = {}
    if not log_path or not os.path.isfile(log_path):
        return metrics
    with open(log_path) as f:
        for line in f:
            m = re.match(r"\s*inject built: (\d+) messages, (\d+) bytes in (\S+)", line)
            if m:
                metrics["messages"] = int(m.group(1))
                metrics["bytes"] = int(m.group(2))
                metrics["build_time"] = m.group(3)
            m = re.match(r"\s*inject sent: \d+ bytes in (\S+) \(([\d.]+) MB/s\)", line)
            if m:
                metrics["send_time"] = m.group(1)
                metrics["mbps"] = float(m.group(2))
    return metrics


# --- BIRD helpers ------------------------------------------------------------


def start_bird(config_path):
    """Start BIRD in the ze-ns namespace. Returns Popen."""
    log_info("starting BIRD in %s..." % ZE_NS)
    bird_log = "/tmp/ze-stress-bird-%s.log" % _SUFFIX
    bird_logfile = open(bird_log, "w")
    bird_pid = "/tmp/ze-stress-bird-%s.pid" % _SUFFIX
    bird_sock = "/tmp/ze-stress-bird-%s.ctl" % _SUFFIX
    proc = subprocess.Popen(
        [
            "ip",
            "netns",
            "exec",
            ZE_NS,
            "bird",
            "-f",
            "-c",
            config_path,
            "-P",
            bird_pid,
            "-s",
            bird_sock,
        ],
        stdout=bird_logfile,
        stderr=subprocess.STDOUT,
    )
    proc._ze_logfile = bird_logfile
    proc._ze_log = bird_log
    _processes.append(proc)
    # Give BIRD a moment to start and bind port 179.
    time.sleep(2)
    if proc.poll() is not None:
        bird_logfile.flush()
        with open(bird_log) as f:
            output = f.read()
        raise RuntimeError(
            "BIRD exited immediately (rc=%d): %s" % (proc.returncode, output[:1000])
        )
    # Verify port 179 is listening.
    ss = _nsexec_ok(ZE_NS, ["ss", "-tln", "sport", "=", "179"])
    if "LISTEN" in ss:
        log_pass("BIRD started (pid %d), port 179 listening" % proc.pid)
    else:
        log_info("BIRD started (pid %d) but port 179 NOT YET listening" % proc.pid)
    log_debug("BIRD log: %s" % bird_log)
    return proc


class BIRD:
    """Helpers for querying BIRD in stress tests."""

    def route_count(self):
        """Return the number of imported routes in BIRD's RIB, or 0 on failure."""
        bird_sock = "/tmp/ze-stress-bird-%s.ctl" % _SUFFIX
        output = _nsexec_ok(ZE_NS, ["birdc", "-s", bird_sock, "show", "route", "count"])
        # BIRD output: "1234 of 1234 routes (1 network)"
        m = re.search(r"(\d+)\s+of\s+\d+\s+routes", output)
        if m:
            return int(m.group(1))
        return 0

    def wait_route_count(self, minimum, timeout=120):
        """Poll until BIRD's RIB has at least `minimum` routes."""
        log_info(
            "waiting for BIRD RIB >= %d routes (timeout %ds)..." % (minimum, timeout)
        )
        deadline = time.time() + timeout
        last_count = 0
        while time.time() < deadline:
            count = self.route_count()
            if count >= minimum:
                log_pass("BIRD RIB has %d routes (target: %d)" % (count, minimum))
                return count
            if count != last_count:
                log_debug("BIRD RIB: %d / %d" % (count, minimum))
                last_count = count
            time.sleep(2)
        count = self.route_count()
        log_fail(
            "BIRD RIB has %d routes after %ds (expected >= %d)"
            % (count, timeout, minimum)
        )
        raise AssertionError(
            "BIRD RIB has %d routes, expected >= %d" % (count, minimum)
        )


# --- Timing helpers ----------------------------------------------------------


class Timer:
    """Simple wall-clock timer for measuring phases."""

    def __init__(self, label):
        self.label = label
        self.start = None
        self.elapsed = None

    def __enter__(self):
        self.start = time.time()
        return self

    def __exit__(self, *args):
        self.elapsed = time.time() - self.start
        log_info("%s: %.2fs" % (self.label, self.elapsed))


# --- Scenario lifecycle ------------------------------------------------------


class Scenario:
    """Manages namespace lifecycle for a stress test scenario."""

    def __init__(self, scenario_dir, ze_binary):
        self.scenario_dir = scenario_dir
        self.ze_binary = ze_binary
        self.name = os.path.basename(scenario_dir.rstrip("/"))

    def setup(self):
        """Create namespaces and start the DUT (ze or bird).

        Each check.py spawns its own ze-test peer(s) via start_peer_inject().
        """
        self.teardown()
        create_netns_pair()

        ze_conf = os.path.join(self.scenario_dir, "ze.conf")
        bird_conf = os.path.join(self.scenario_dir, "bird.conf")

        if os.path.isfile(ze_conf):
            start_ze(self.ze_binary, ze_conf)
        elif os.path.isfile(bird_conf):
            if not shutil.which("bird"):
                print("Installing BIRD 2.x (first run)...")
                subprocess.run(
                    ["apt-get", "install", "-y", "--no-install-recommends", "bird2"],
                    check=True,
                    timeout=120,
                )
                if not shutil.which("bird"):
                    raise RuntimeError("BIRD install failed")
            start_bird(bird_conf)
        else:
            raise RuntimeError("no DUT config (ze.conf or bird.conf) in %s" % self.name)

    def teardown(self):
        """Remove namespaces and stop processes."""
        destroy_netns_pair()

    def run_check(self):
        """Import and run check.py."""
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
            raise RuntimeError("check.py in %s failed: %s" % (self.name, e)) from e
        if not hasattr(mod, "check"):
            raise RuntimeError("check.py in %s has no check() function" % self.name)
        mod.check()


def global_cleanup():
    """Remove namespaces and kill processes on exit."""
    destroy_netns_pair()


atexit.register(global_cleanup)
