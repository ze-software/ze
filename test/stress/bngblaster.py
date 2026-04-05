#!/usr/bin/env python3
"""Shared helpers for Ze BGP stress tests using BNG Blaster.

Uses network namespaces and veth pairs instead of Docker.
BNG Blaster uses its own TCP/IP stack (LwIP) and needs raw interface access,
which is incompatible with Docker's managed networking.

Architecture:
    [ze-ns]                    [bb-ns]
    ze binary                  bngblaster
    ze-veth (172.31.0.2/24)    bb-veth (managed by LwIP)
         \________________________/
              veth pair
"""

import atexit
import json
import os
import re
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
    for path in [BB_SOCKET, "/tmp/ze-stress-bb-%s.json" % _SUFFIX]:
        try:
            os.unlink(path)
        except OSError:
            pass


# --- Process management ------------------------------------------------------

def start_ze(ze_binary, config_path):
    """Start Ze in the ze-ns namespace. Returns Popen."""
    log_info("starting Ze in %s..." % ZE_NS)
    proc = subprocess.Popen(
        ["ip", "netns", "exec", ZE_NS, ze_binary, config_path],
        stdout=subprocess.PIPE, stderr=subprocess.STDOUT,
    )
    _processes.append(proc)
    # Give Ze a moment to start and bind port 179.
    time.sleep(2)
    if proc.poll() is not None:
        output = proc.stdout.read().decode() if proc.stdout else ""
        raise RuntimeError("Ze exited immediately (rc=%d): %s" % (proc.returncode, output[:500]))
    log_pass("Ze started (pid %d)" % proc.pid)
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
    proc = subprocess.Popen(
        ["ip", "netns", "exec", BB_NS,
         "bngblaster", "-C", rendered, "-S", BB_SOCKET],
        stdout=subprocess.PIPE, stderr=subprocess.STDOUT,
    )
    _processes.append(proc)
    time.sleep(2)
    if proc.poll() is not None:
        output = proc.stdout.read().decode() if proc.stdout else ""
        raise RuntimeError("BNG Blaster exited immediately (rc=%d): %s" % (proc.returncode, output[:500]))
    log_pass("BNG Blaster started (pid %d)" % proc.pid)
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
    _run(cmd, timeout=120)
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
        while time.time() < deadline:
            data = self.bgp_sessions()
            sessions = data.get("bgp-sessions", [])
            for s in sessions:
                if s.get("state") == "established":
                    log_pass("BGP session established (peer %s)" % s.get("peer-address", "?"))
                    return s
            time.sleep(2)
        log_fail("BGP session did not reach Established within %ds" % timeout)
        raise AssertionError("BGP session not Established")

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
    """Helpers for querying Ze in stress tests."""

    def rib_count(self):
        """Return the number of received routes in Ze's RIB, or 0 on failure."""
        output = _nsexec_ok(ZE_NS, ["ze", "rib", "status"])
        m = re.search(r'"routes-in"\s*:\s*(\d+)', output)
        if m:
            return int(m.group(1))
        return 0

    def wait_rib_count(self, minimum, timeout=120):
        """Poll until Ze's RIB has at least `minimum` routes."""
        log_info("waiting for Ze RIB >= %d routes (timeout %ds)..." % (minimum, timeout))
        deadline = time.time() + timeout
        last_count = 0
        while time.time() < deadline:
            count = self.rib_count()
            if count >= minimum:
                log_pass("Ze RIB has %d routes (target: %d)" % (count, minimum))
                return count
            if count != last_count:
                log_debug("Ze RIB: %d / %d" % (count, minimum))
                last_count = count
            time.sleep(2)
        count = self.rib_count()
        log_fail("Ze RIB has %d routes after %ds (expected >= %d)" % (count, timeout, minimum))
        raise AssertionError("Ze RIB has %d routes, expected >= %d" % (count, minimum))


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
        """Create namespaces, start Ze and BNG Blaster."""
        self.teardown()
        create_netns_pair()

        ze_conf = os.path.join(self.scenario_dir, "ze.conf")
        if not os.path.isfile(ze_conf):
            raise RuntimeError("missing ze.conf in %s" % self.name)

        bb_conf = os.path.join(self.scenario_dir, "bb.json")
        if not os.path.isfile(bb_conf):
            raise RuntimeError("missing bb.json in %s" % self.name)

        start_ze(self.ze_binary, ze_conf)
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
