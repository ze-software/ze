#!/usr/bin/env python3
"""Shared helpers for Ze BGP stress tests using BNG Blaster.

Uses network namespaces and veth pairs instead of Docker.
BNG Blaster uses its own TCP/IP stack (LwIP) and needs raw interface access,
which is incompatible with Docker's managed networking.

Architecture:
    [ze-ns]                    [bb-ns]
    ze binary                  bngblaster
    ze-veth (172.31.0.2/24)    bb-veth (managed by LwIP)
         \\________________________/
              veth pair
"""

import atexit
import json
import os
import re
import shutil
import signal
import subprocess
import sys
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

# Control socket path (inside bb-ns, visible from host).
BB_SOCKET = "/tmp/ze-stress-bb-%s.sock" % _SUFFIX

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
            cmd, capture_output=capture, text=True, timeout=timeout,
        )
    except subprocess.TimeoutExpired:
        raise RuntimeError("command timed out after %ds: %s" % (timeout, " ".join(cmd[:4])))


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

    ze-ns gets a kernel IP (172.31.0.2/24) for Ze's normal TCP stack.
    bb-ns gets a raw interface (no kernel IP) for BNG Blaster's LwIP stack.
    """
    log_info("creating network namespaces and veth pair...")

    # Create namespaces.
    _run(["ip", "netns", "add", ZE_NS])
    _run(["ip", "netns", "add", BB_NS])

    # Create veth pair.
    _run(["ip", "link", "add", ZE_VETH, "type", "veth", "peer", "name", BB_VETH])

    # Move interfaces into namespaces.
    _run(["ip", "link", "set", ZE_VETH, "netns", ZE_NS])
    _run(["ip", "link", "set", BB_VETH, "netns", BB_NS])

    # Configure Ze side (kernel IP).
    _nsexec(ZE_NS, ["ip", "addr", "add", "%s/24" % ZE_IP, "dev", ZE_VETH])
    _nsexec(ZE_NS, ["ip", "link", "set", ZE_VETH, "up"])
    _nsexec(ZE_NS, ["ip", "link", "set", "lo", "up"])

    # Configure BB side (up but no kernel IP -- LwIP manages it).
    _nsexec(BB_NS, ["ip", "link", "set", BB_VETH, "up"])
    _nsexec(BB_NS, ["ip", "link", "set", "lo", "up"])

    # Disable checksum offloading on both veth ends. Without this, the kernel
    # marks outgoing packets as CHECKSUM_PARTIAL (expecting hardware computation)
    # but veth has no hardware. BNG Blaster's LwIP receives packets with invalid
    # TCP checksums and silently drops them, preventing the 3-way handshake.
    _nsexec(ZE_NS, ["ethtool", "-K", ZE_VETH, "tx", "off", "rx", "off"], check=False)
    _nsexec(BB_NS, ["ethtool", "-K", BB_VETH, "tx", "off", "rx", "off"], check=False)

    log_pass("namespaces ready: %s (%s) <-> %s" % (ZE_NS, ZE_IP, BB_NS))


def destroy_netns_pair():
    """Remove namespaces and veth pair."""
    # Kill any processes still in the namespaces.
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

    # Delete namespaces (also removes veth pair).
    _run(["ip", "netns", "del", ZE_NS], check=False)
    _run(["ip", "netns", "del", BB_NS], check=False)

    # Clean up temp files.
    for path in [BB_SOCKET, "/tmp/ze-stress-bb-%s.json" % _SUFFIX,
                  "/tmp/ze-stress-ze-%s.log" % _SUFFIX, "/tmp/ze-stress-bb-%s.log" % _SUFFIX,
                  "/tmp/ze-stress-bb-%s.stdout" % _SUFFIX, "/tmp/ze-stress-pcap-%s.txt" % _SUFFIX,
                  "/tmp/ze-stress-bird-%s.log" % _SUFFIX, "/tmp/ze-stress-bird-%s.pid" % _SUFFIX,
                  "/tmp/ze-stress-bird-%s.ctl" % _SUFFIX]:
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
        stdout=ze_logfile, stderr=subprocess.STDOUT,
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
        raise RuntimeError("Ze exited immediately (rc=%d): %s" % (proc.returncode, output[:1000]))
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
        ["ip", "netns", "exec", ZE_NS, "tcpdump", "-i", ZE_VETH,
         "-nn", "-l", "-c", "100", "tcp", "port", "179"],
        stdout=open(pcap_file, "w"), stderr=subprocess.STDOUT,
    )
    tcpdump_proc._pcap_file = pcap_file
    _processes.append(tcpdump_proc)
    log_debug("Ze log: %s" % ze_log)
    return proc


def _render_bb_config(template_path):
    """Render bb.json template, replacing __BB_VETH__ with actual veth name."""
    with open(template_path) as f:
        content = f.read()
    content = content.replace("__BB_VETH__", BB_VETH)
    rendered = "/tmp/ze-stress-bb-%s.json" % _SUFFIX
    with open(rendered, "w") as f:
        f.write(content)
    return rendered


def start_bngblaster(config_path):
    """Start BNG Blaster in the bb-ns namespace. Returns Popen."""
    rendered = _render_bb_config(config_path)
    log_info("starting BNG Blaster in %s..." % BB_NS)
    bb_log = "/tmp/ze-stress-bb-%s.log" % _SUFFIX
    bb_stdout = "/tmp/ze-stress-bb-%s.stdout" % _SUFFIX
    bb_stdoutfile = open(bb_stdout, "w")
    proc = subprocess.Popen(
        ["ip", "netns", "exec", BB_NS,
         "bngblaster", "-C", rendered, "-S", BB_SOCKET,
         "-L", bb_log, "-l", "bgp"],
        stdout=bb_stdoutfile, stderr=subprocess.STDOUT,
    )
    proc._bb_logfile = bb_stdoutfile
    proc._bb_log = bb_log
    proc._bb_stdout = bb_stdout
    _processes.append(proc)
    time.sleep(2)
    if proc.poll() is not None:
        bb_stdoutfile.flush()
        with open(bb_stdout) as f:
            output = f.read()
        raise RuntimeError("BNG Blaster exited immediately (rc=%d): %s" % (proc.returncode, output[:1000]))
    log_pass("BNG Blaster started (pid %d)" % proc.pid)
    log_debug("BNG Blaster log: %s" % bb_log)
    return proc


# --- Route generation -------------------------------------------------------

def generate_updates(prefix_base, prefix_count, nexthop, asn,
                     filename="updates.bgp", output_dir="/tmp", extra_args=None):
    """Generate BGP raw update file using bgpupdate."""
    output_path = os.path.join(output_dir, filename)
    cmd = [
        "bgpupdate",
        "-p", prefix_base,
        "-P", str(prefix_count),
        "-n", nexthop,
        "-a", str(asn),
        "-f", output_path,
        "--end-of-rib",
    ]
    for arg in (extra_args or []):
        cmd.append(arg)
    log_info("generating %d prefixes from %s..." % (prefix_count, prefix_base))
    gen_timeout = max(120, prefix_count // 2000)  # ~2s per 1k prefixes, min 120s
    _run(cmd, timeout=gen_timeout)
    log_pass("generated %s (%d prefixes)" % (filename, prefix_count))
    return output_path


# --- BNG Blaster control socket ---------------------------------------------

class BNGBlaster:
    """Control socket client for BNG Blaster."""

    def __init__(self, socket_path=None):
        self.socket = socket_path or BB_SOCKET

    def _cli(self, method, params=None):
        """Send command via bngblaster-cli (runs in bb-ns)."""
        cmd = ["ip", "netns", "exec", BB_NS,
               "bngblaster-cli", self.socket, method]
        if params:
            for k, v in params.items():
                cmd.extend([k, str(v)])
        output = _run_ok(cmd, timeout=30)
        if not output.strip():
            return {}
        try:
            return json.loads(output)
        except json.JSONDecodeError:
            log_debug("JSON parse failed for %s: %s" % (method, output[:200]))
            return {}

    def bgp_sessions(self):
        """Get BGP session status."""
        return self._cli("bgp-sessions")

    def bgp_disconnect(self):
        """Disconnect all BGP sessions."""
        return self._cli("bgp-disconnect")

    def bgp_raw_update(self, filepath):
        """Inject BGP raw update file."""
        return self._cli("bgp-raw-update", {"file": filepath})

    def bgp_raw_update_list(self):
        """List loaded raw update files."""
        return self._cli("bgp-raw-update-list")

    def wait_session_established(self, timeout=None):
        """Poll until at least one BGP session reaches Established."""
        if timeout is None:
            timeout = SESSION_TIMEOUT
        log_info("waiting for BGP session (timeout %ds)..." % timeout)
        deadline = time.time() + timeout
        last_state = ""
        while time.time() < deadline:
            data = self.bgp_sessions()
            sessions = data.get("bgp-sessions", [])
            for s in sessions:
                state = s.get("state", "unknown")
                if state == "established":
                    log_pass("BGP session established (peer %s)" % s.get("peer-address", "?"))
                    return s
                if state != last_state:
                    log_debug("BGP session state: %s" % state)
                    last_state = state
            time.sleep(2)
        # Dump diagnostics on failure.
        log_fail("BGP session did not reach Established within %ds" % timeout)
        self._dump_failure_diagnostics()
        raise AssertionError("BGP session not Established")

    def _dump_failure_diagnostics(self):
        """Print diagnostics when session fails to establish."""
        # BNG Blaster session state.
        data = self.bgp_sessions()
        sessions = data.get("bgp-sessions", [])
        if sessions:
            log_info("BNG Blaster BGP sessions: %s" % json.dumps(sessions, indent=2))
        else:
            log_info("BNG Blaster reports no BGP sessions")
        # Ze port check.
        ss = _nsexec_ok(ZE_NS, ["ss", "-tln", "sport", "=", "179"])
        if "LISTEN" in ss:
            log_info("Ze port 179: LISTENING")
        else:
            log_info("Ze port 179: NOT listening")
        # Network connectivity check.
        log_info("--- network diagnostics ---")
        # Full listener details (check bind address).
        ss_listen = _nsexec_ok(ZE_NS, ["ss", "-tlnp", "sport", "=", "179"])
        log_info("Ze listeners:\n%s" % ss_listen.strip())
        # All TCP states on port 179.
        ss_all = _nsexec_ok(ZE_NS, ["ss", "-tanp", "sport", "=", "179", "or", "dport", "=", "179"])
        log_info("Ze TCP all states:\n%s" % ss_all.strip())
        arp = _nsexec_ok(ZE_NS, ["ip", "neigh", "show"])
        log_info("Ze ARP table: %s" % arp.strip() if arp.strip() else "Ze ARP table: (empty)")
        # Background pcap capture (started with Ze).
        pcap_file = "/tmp/ze-stress-pcap-%s.txt" % _SUFFIX
        if os.path.isfile(pcap_file):
            with open(pcap_file) as f:
                pcap_lines = f.readlines()
            if pcap_lines:
                log_info("tcpdump tcp/179 (%d packets):" % len(pcap_lines))
                for line in pcap_lines[:20]:
                    print("    %s" % line.rstrip())
            else:
                log_info("tcpdump tcp/179: NO TCP PACKETS on port 179 (BNG Blaster never sent SYN)")
        # BNG Blaster interface state via control socket.
        ifaces = self._cli("interfaces")
        if ifaces:
            log_info("BB interfaces: %s" % json.dumps(ifaces, indent=2))
        # Ze log tail.
        for proc in _processes:
            ze_log = getattr(proc, "_ze_log", None)
            if ze_log and os.path.isfile(ze_log):
                proc._ze_logfile.flush()
                with open(ze_log) as f:
                    lines = f.readlines()
                tail = lines[-30:] if len(lines) > 30 else lines
                log_info("Ze log (last %d lines):" % len(tail))
                for line in tail:
                    print("    %s" % line.rstrip())
            bb_log = getattr(proc, "_bb_log", None)
            if bb_log and os.path.isfile(bb_log):
                with open(bb_log) as f:
                    lines = f.readlines()
                tail = lines[-50:] if len(lines) > 50 else lines
                log_info("BNG Blaster log (last %d lines):" % len(tail))
                for line in tail:
                    print("    %s" % line.rstrip())
            bb_stdout = getattr(proc, "_bb_stdout", None)
            if bb_stdout and os.path.isfile(bb_stdout):
                proc._bb_logfile.flush()
                with open(bb_stdout) as f:
                    lines = f.readlines()
                tail = lines[-30:] if len(lines) > 30 else lines
                if tail:
                    log_info("BNG Blaster stdout (last %d lines):" % len(tail))
                    for line in tail:
                        print("    %s" % line.rstrip())

    def wait_raw_update_done(self, timeout=300):
        """Poll until raw update injection is complete."""
        log_info("waiting for raw update injection (timeout %ds)..." % timeout)
        deadline = time.time() + timeout
        while time.time() < deadline:
            data = self.bgp_sessions()
            sessions = data.get("bgp-sessions", [])
            all_done = True
            for s in sessions:
                raw_state = s.get("raw-update-state", "")
                if raw_state == "done":
                    continue
                all_done = False
                break
            if all_done and sessions:
                duration = 0
                for s in sessions:
                    d = s.get("raw-update-duration-ms", 0)
                    if d > duration:
                        duration = d
                log_pass("raw update injection complete (%.1fs)" % (duration / 1000.0))
                return sessions
            time.sleep(1)
        log_fail("raw update injection did not complete within %ds" % timeout)
        raise AssertionError("raw update injection timeout")


# --- Ze helpers --------------------------------------------------------------

class Ze:
    """Helpers for querying Ze in stress tests.

    Uses BNG Blaster session stats as a proxy for Ze's route processing,
    since querying Ze's RIB directly requires SSH credentials (ze init).
    """

    def __init__(self, bb):
        self.bb = bb

    def wait_settled(self, timeout=30):
        """Wait for Ze to finish processing by checking session stability.

        After route injection, polls BNG Blaster until the session is stable
        (update-tx count stops changing) or timeout. Returns the final update-tx.
        """
        log_info("waiting for Ze to settle (timeout %ds)..." % timeout)
        deadline = time.time() + timeout
        last_tx = -1
        stable_count = 0
        while time.time() < deadline:
            data = self.bb.bgp_sessions()
            sessions = data.get("bgp-sessions", [])
            for s in sessions:
                if s.get("state") != "established":
                    log_fail("session dropped during settling")
                    raise AssertionError("session dropped")
                tx = s.get("stats", {}).get("update-tx", 0)
                if tx == last_tx:
                    stable_count += 1
                    if stable_count >= 3:
                        log_pass("Ze settled (update-tx: %d)" % tx)
                        return tx
                else:
                    stable_count = 0
                    last_tx = tx
            time.sleep(1)
        log_pass("Ze settle timeout reached (update-tx: %d)" % last_tx)
        return last_tx


# --- BIRD helpers ------------------------------------------------------------

def start_bird(config_path):
    """Start BIRD in the ze-ns namespace. Returns Popen."""
    log_info("starting BIRD in %s..." % ZE_NS)
    bird_log = "/tmp/ze-stress-bird-%s.log" % _SUFFIX
    bird_logfile = open(bird_log, "w")
    bird_pid = "/tmp/ze-stress-bird-%s.pid" % _SUFFIX
    bird_sock = "/tmp/ze-stress-bird-%s.ctl" % _SUFFIX
    proc = subprocess.Popen(
        ["ip", "netns", "exec", ZE_NS,
         "bird", "-f", "-c", config_path,
         "-P", bird_pid, "-s", bird_sock],
        stdout=bird_logfile, stderr=subprocess.STDOUT,
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
        raise RuntimeError("BIRD exited immediately (rc=%d): %s" % (proc.returncode, output[:1000]))
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
        m = re.search(r'(\d+)\s+of\s+\d+\s+routes', output)
        if m:
            return int(m.group(1))
        return 0

    def wait_route_count(self, minimum, timeout=120):
        """Poll until BIRD's RIB has at least `minimum` routes."""
        log_info("waiting for BIRD RIB >= %d routes (timeout %ds)..." % (minimum, timeout))
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
        log_fail("BIRD RIB has %d routes after %ds (expected >= %d)" % (count, timeout, minimum))
        raise AssertionError("BIRD RIB has %d routes, expected >= %d" % (count, minimum))


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
        """Create namespaces, start DUT and BNG Blaster.

        DUT is detected from config files: ze.conf -> Ze, bird.conf -> BIRD.
        """
        self.teardown()
        create_netns_pair()

        bb_conf = os.path.join(self.scenario_dir, "bb.json")
        if not os.path.isfile(bb_conf):
            raise RuntimeError("missing bb.json in %s" % self.name)

        # Detect DUT from config files present.
        ze_conf = os.path.join(self.scenario_dir, "ze.conf")
        bird_conf = os.path.join(self.scenario_dir, "bird.conf")

        if os.path.isfile(ze_conf):
            start_ze(self.ze_binary, ze_conf)
        elif os.path.isfile(bird_conf):
            if not shutil.which("bird"):
                print("Installing BIRD 2.x (first run)...")
                subprocess.run(["apt-get", "install", "-y", "--no-install-recommends", "bird2"],
                               check=True, timeout=120)
                if not shutil.which("bird"):
                    raise RuntimeError("BIRD install failed")
            start_bird(bird_conf)
        else:
            raise RuntimeError("no DUT config (ze.conf or bird.conf) in %s" % self.name)

        start_bngblaster(bb_conf)

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
