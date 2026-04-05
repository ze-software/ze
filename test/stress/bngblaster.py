#!/usr/bin/env python3
"""Shared helpers for Ze BGP stress tests using BNG Blaster.

Provides container management, route generation, BNG Blaster control socket
interaction, and scenario lifecycle management.
"""

import atexit
import json
import os
import re
import subprocess
import sys
import time

# Network and container naming (PID suffix avoids conflicts between concurrent runs).
_SUFFIX = os.environ.get("ZE_STRESS_SUFFIX", str(os.getpid()))
NETWORK = "ze-stress-%s" % _SUFFIX
ZE_CONTAINER = "ze-stress-ze-%s" % _SUFFIX
BB_CONTAINER = "ze-stress-bb-%s" % _SUFFIX

# IP addresses on the test network.
ZE_IP = "172.31.0.2"
BB_IP = "172.31.0.3"

# Default timeout for session establishment (seconds).
try:
    SESSION_TIMEOUT = int(os.environ.get("SESSION_TIMEOUT", "120"))
except ValueError:
    SESSION_TIMEOUT = 120

VERBOSE = os.environ.get("VERBOSE", "0") == "1"


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


# --- Docker helpers ----------------------------------------------------------

def _docker(args, timeout=30):
    """Run a docker command and return the CompletedProcess."""
    try:
        return subprocess.run(
            ["docker"] + args,
            capture_output=True, text=True, timeout=timeout,
        )
    except subprocess.TimeoutExpired:
        raise RuntimeError("docker %s timed out after %ds" % (" ".join(args[:3]), timeout))


def docker_exec(container, cmd, timeout=30):
    """Run command in container, return stdout. Raises on failure."""
    result = _docker(["exec", container] + cmd, timeout=timeout)
    if result.returncode != 0:
        raise RuntimeError(
            "docker exec %s %s failed (rc=%d): %s"
            % (container, " ".join(cmd), result.returncode, result.stderr.strip())
        )
    return result.stdout


def docker_exec_quiet(container, cmd, timeout=30):
    """Run command in container, return stdout or empty string on failure."""
    try:
        result = _docker(["exec", container] + cmd, timeout=timeout)
        if result.returncode != 0:
            return ""
        return result.stdout
    except Exception:
        return ""


def docker_run(name, image, ip, volumes=None, caps=None, extra_args=None, cmd=None):
    """Start a container."""
    args = ["run", "-d", "--name", name, "--network", NETWORK, "--ip", ip]
    for cap in (caps or []):
        args.extend(["--cap-add", cap])
    for vol in (volumes or []):
        args.extend(["-v", vol])
    for arg in (extra_args or []):
        args.append(arg)
    args.append(image)
    for c in (cmd or []):
        args.append(c)
    result = _docker(args, timeout=60)
    if result.returncode != 0:
        raise RuntimeError("docker run %s failed: %s" % (name, result.stderr.strip()))


def docker_rm(name):
    """Remove container, ignore if not exists."""
    _docker(["rm", "-f", name], timeout=30)


def docker_logs(container, lines=30):
    """Get last N lines of container logs."""
    try:
        result = _docker(["logs", container, "--tail", str(lines)], timeout=15)
        return result.stdout + result.stderr
    except Exception:
        return "(docker logs timed out)"


# --- Route generation -------------------------------------------------------

def generate_updates(scenario_dir, prefix_base, prefix_count, nexthop, asn,
                     filename="updates.bgp", extra_args=None):
    """Generate BGP raw update file using bgpupdate inside the BB container.

    Must be called after the BB container is running.
    """
    output_path = "/test/%s" % filename
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
    docker_exec(BB_CONTAINER, cmd, timeout=120)
    log_pass("generated %s (%d prefixes)" % (filename, prefix_count))
    return output_path


# --- BNG Blaster control socket ---------------------------------------------

class BNGBlaster:
    """Control socket client for BNG Blaster."""

    SOCKET = "/var/bngblaster/stress/run.sock"

    def __init__(self, container=BB_CONTAINER):
        self.container = container

    def _cli(self, method, params=None):
        """Send command via bngblaster-cli."""
        cmd = ["bngblaster-cli", "-s", self.SOCKET, method]
        if params:
            for k, v in params.items():
                cmd.extend(["--%s" % k, str(v)])
        output = docker_exec_quiet(self.container, cmd, timeout=30)
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
        log_info("waiting for BNG Blaster BGP session (timeout %ds)..." % timeout)
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
        log_debug("ze logs:\n%s" % docker_logs(ZE_CONTAINER, 20))
        log_debug("bb logs:\n%s" % docker_logs(BB_CONTAINER, 20))
        raise AssertionError("BGP session not Established")

    def wait_raw_update_done(self, timeout=300):
        """Poll until raw update injection is complete."""
        log_info("waiting for raw update injection to complete (timeout %ds)..." % timeout)
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

    def __init__(self, container=ZE_CONTAINER):
        self.container = container

    def rib_count(self):
        """Return the number of received routes in Ze's RIB, or 0 on failure."""
        output = docker_exec_quiet(self.container, ["ze", "show", "rib", "status"])
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

    def logs(self, lines=30):
        """Get last N lines of container logs."""
        return docker_logs(self.container, lines)


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
    """Manages container lifecycle for a stress test scenario."""

    def __init__(self, scenario_dir):
        self.scenario_dir = scenario_dir
        self.name = os.path.basename(scenario_dir.rstrip("/"))

    def setup(self):
        """Create network, start containers."""
        self.teardown()

        # Create network.
        result = _docker(
            ["network", "create", "--subnet=172.31.0.0/24", NETWORK],
            timeout=30,
        )
        if result.returncode != 0 and "already exists" not in result.stderr:
            raise RuntimeError("docker network create failed: %s" % result.stderr.strip())

        ze_conf = os.path.join(self.scenario_dir, "ze.conf")
        if not os.path.isfile(ze_conf):
            raise RuntimeError("missing ze.conf in %s" % self.name)

        # Collect extra volume mounts for Ze (plugin scripts, etc.).
        volumes = ["%s:/etc/ze/bgp.conf:ro" % os.path.abspath(ze_conf)]
        for fname in sorted(os.listdir(self.scenario_dir)):
            if fname in ("check.py", "ze.conf", "bb.json"):
                continue
            fpath = os.path.join(self.scenario_dir, fname)
            if not os.path.isfile(fpath):
                continue
            if fname.endswith(".sh") or fname.endswith(".py") or fname.endswith(".bgp"):
                volumes.append("%s:/etc/ze/%s:ro" % (os.path.abspath(fpath), fname))

        # Start Ze.
        docker_run(
            ZE_CONTAINER, "ze-interop", ZE_IP,
            volumes=volumes,
            caps=["NET_ADMIN"],
            cmd=["/etc/ze/bgp.conf"],
        )

        # Start BNG Blaster.
        bb_conf = os.path.join(self.scenario_dir, "bb.json")
        if not os.path.isfile(bb_conf):
            raise RuntimeError("missing bb.json in %s" % self.name)

        bb_volumes = [
            "%s:/test/bb.json:ro" % os.path.abspath(bb_conf),
        ]
        # Mount any pre-generated .bgp update files.
        for fname in sorted(os.listdir(self.scenario_dir)):
            if fname.endswith(".bgp"):
                fpath = os.path.join(self.scenario_dir, fname)
                bb_volumes.append("%s:/test/%s:ro" % (os.path.abspath(fpath), fname))

        docker_run(
            BB_CONTAINER, "ze-stress-bb", BB_IP,
            volumes=bb_volumes,
            caps=["NET_ADMIN", "NET_RAW"],
            cmd=["-C", "/test/bb.json", "-S", "/var/bngblaster/stress/run.sock"],
        )

        # Give containers a moment to start.
        time.sleep(2)

    def teardown(self):
        """Remove containers and network."""
        docker_rm(ZE_CONTAINER)
        docker_rm(BB_CONTAINER)
        _docker(["network", "rm", NETWORK], timeout=30)

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
            raise RuntimeError("check.py in %s failed to load: %s" % (self.name, e)) from e
        if not hasattr(mod, "check"):
            raise RuntimeError("check.py in %s has no check() function" % self.name)
        mod.check()


def global_cleanup():
    """Remove all containers and network on exit."""
    for name in [ZE_CONTAINER, BB_CONTAINER]:
        _docker(["rm", "-f", name], timeout=30)
    _docker(["network", "rm", NETWORK], timeout=30)


atexit.register(global_cleanup)
