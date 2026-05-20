#!/usr/bin/env python3
"""Docker lifecycle and helpers for Ze IPsec interop lab.

Manages container creation, network setup, log collection, and daemon
helpers for Ze (IKE initiator) and strongSwan/charon (IKE responder).
"""

import atexit
import os
import subprocess
import time

_SUFFIX = os.environ.get("ZE_IPSEC_INTEROP_SUFFIX", str(os.getpid()))
NETWORK = "ze-ipsec-%s" % _SUFFIX
ZE_CONTAINER = "ze-ipsec-ze-%s" % _SUFFIX
SWAN_CONTAINER = "ze-ipsec-swan-%s" % _SUFFIX

SUBNET = "172.28.0.0/24"
ZE_IP = "172.28.0.2"
SWAN_IP = "172.28.0.3"

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


# --- Charon startup ----------------------------------------------------------


def _wait_charon_ready(timeout=30):
    """Poll until charon's VICI socket is responding to swanctl."""
    log_info("waiting for charon to start (timeout %ds)..." % timeout)
    deadline = time.time() + timeout
    while time.time() < deadline:
        output = docker_exec_quiet(SWAN_CONTAINER, ["swanctl", "--stats"])
        if "uptime" in output:
            log_debug("charon is ready")
            return
        time.sleep(1)
    log_fail("charon did not start within %ds" % timeout)
    raise RuntimeError("charon not ready")


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


def ze_xfrm_state():
    """Return XFRM SA state from Ze container."""
    return docker_exec_quiet(ZE_CONTAINER, ["ip", "xfrm", "state"])


def ze_xfrm_policy():
    """Return XFRM policy state from Ze container."""
    return docker_exec_quiet(ZE_CONTAINER, ["ip", "xfrm", "policy"])


# --- strongSwan helpers ------------------------------------------------------


class StrongSwan:
    """Helpers for querying strongSwan/charon via swanctl."""

    def __init__(self, container=SWAN_CONTAINER, ip=SWAN_IP):
        self.container = container
        self.ip = ip

    def _swanctl(self, args):
        """Run a swanctl command, return stdout or empty string on failure."""
        return docker_exec_quiet(self.container, ["swanctl"] + args)

    def list_sas(self):
        """Return raw swanctl --list-sas output."""
        return self._swanctl(["--list-sas"])

    def wait_sa_established(self, conn_name=None, timeout=None):
        """Poll until an IKE SA is established in strongSwan."""
        if timeout is None:
            timeout = SESSION_TIMEOUT
        label = conn_name or "any"
        log_info("waiting for strongSwan SA '%s' (timeout %ds)..." % (label, timeout))
        deadline = time.time() + timeout
        while time.time() < deadline:
            output = self.list_sas()
            if "ESTABLISHED" in output:
                if conn_name is None or conn_name in output:
                    log_pass("strongSwan SA '%s' is ESTABLISHED" % label)
                    return
            time.sleep(2)
        log_fail(
            "strongSwan SA '%s' did not reach ESTABLISHED within %ds" % (label, timeout)
        )
        output = self.list_sas()
        for line in output.splitlines()[:20]:
            print("  %s" % line)
        print(docker_logs(ZE_CONTAINER, 20))
        raise AssertionError("strongSwan SA '%s' not ESTABLISHED" % label)

    def has_child_sa(self, child_name=None):
        """Check if a Child SA is installed."""
        output = self.list_sas()
        if not output.strip():
            return False
        if child_name:
            return child_name in output and "INSTALLED" in output
        return "INSTALLED" in output

    def wait_child_sa(self, child_name=None, timeout=30):
        """Poll until a Child SA appears as INSTALLED."""
        label = child_name or "any"
        log_info("waiting for strongSwan Child SA '%s'..." % label)
        deadline = time.time() + timeout
        while time.time() < deadline:
            if self.has_child_sa(child_name):
                log_pass("strongSwan Child SA '%s' INSTALLED" % label)
                return
            time.sleep(2)
        log_fail("strongSwan Child SA '%s' not INSTALLED within %ds" % (label, timeout))
        raise AssertionError("strongSwan Child SA '%s' not INSTALLED" % label)

    def sa_count(self):
        """Return the number of established IKE SAs."""
        output = self.list_sas()
        return output.count("ESTABLISHED")

    def xfrm_state(self):
        """Return XFRM SA state from strongSwan container."""
        return docker_exec_quiet(self.container, ["ip", "xfrm", "state"])

    def xfrm_policy(self):
        """Return XFRM policy state from strongSwan container."""
        return docker_exec_quiet(self.container, ["ip", "xfrm", "policy"])

    def break_link(self):
        """Drop outbound IKE/ESP packets to induce a DPD timeout."""
        docker_exec(
            self.container,
            ["iptables", "-I", "OUTPUT", "1", "-d", ZE_IP, "-j", "DROP"],
        )

    def restore_link(self):
        """Remove the iptables drop rule installed by break_link."""
        docker_exec_quiet(
            self.container,
            ["iptables", "-D", "OUTPUT", "-d", ZE_IP, "-j", "DROP"],
        )


# --- XFRM verification helpers ----------------------------------------------


def wait_xfrm_sa(container, proto="esp", timeout=30):
    """Poll until at least one XFRM SA of the given protocol exists."""
    log_info("waiting for XFRM SA (proto %s) in %s..." % (proto, container))
    deadline = time.time() + timeout
    while time.time() < deadline:
        output = docker_exec_quiet(container, ["ip", "xfrm", "state"])
        if "proto %s" % proto in output:
            log_pass("XFRM SA (proto %s) present in %s" % (proto, container))
            return output
        time.sleep(2)
    log_fail("no XFRM SA (proto %s) in %s within %ds" % (proto, container, timeout))
    raise AssertionError("no XFRM SA in %s" % container)


def check_xfrm_sa_count(container, expected, proto="esp"):
    """Assert the number of XFRM SAs matches expected (each direction = 1 SA)."""
    output = docker_exec_quiet(container, ["ip", "xfrm", "state"])
    count = output.count("proto %s" % proto)
    if count == expected:
        log_pass("%s has %d XFRM SA(s) (expected %d)" % (container, count, expected))
        return
    log_fail("%s has %d XFRM SA(s) (expected %d)" % (container, count, expected))
    raise AssertionError("%s XFRM SA count %d != %d" % (container, count, expected))


# --- Scenario lifecycle ------------------------------------------------------


class Scenario:
    def __init__(self, scenario_dir):
        self.scenario_dir = scenario_dir
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

        ze_conf = os.path.join(self.scenario_dir, "ze.conf")
        if not os.path.isfile(ze_conf):
            raise RuntimeError("missing ze.conf in %s" % self.name)

        ze_volumes = [
            "%s:/etc/ze/ze.conf:ro" % os.path.abspath(ze_conf),
        ]

        docker_run(
            ZE_CONTAINER,
            "ze-ipsec-interop",
            ZE_IP,
            volumes=ze_volumes,
            extra_args=["--privileged"],
            cmd=["/etc/ze/ze.conf"],
        )

        swanctl_conf = os.path.join(self.scenario_dir, "swanctl.conf")
        if os.path.isfile(swanctl_conf):
            swan_volumes = [
                "%s:/etc/swanctl/conf.d/interop.conf:ro"
                % os.path.abspath(swanctl_conf),
            ]

            docker_run(
                SWAN_CONTAINER,
                "ze-ipsec-strongswan",
                SWAN_IP,
                volumes=swan_volumes,
                extra_args=["--privileged"],
            )

            _wait_charon_ready()
            docker_exec(SWAN_CONTAINER, ["swanctl", "--load-all"])

    def teardown(self):
        docker_rm(ZE_CONTAINER)
        docker_rm(SWAN_CONTAINER)
        subprocess.run(
            ["docker", "network", "rm", NETWORK],
            capture_output=True,
            text=True,
            timeout=30,
        )

    def dump_logs(self, lines=80):
        print("\n--- Ze logs ---")
        print(docker_logs(ZE_CONTAINER, lines))
        print("\n--- strongSwan logs ---")
        print(docker_logs(SWAN_CONTAINER, lines))

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
    for name in [ZE_CONTAINER, SWAN_CONTAINER]:
        subprocess.run(
            ["docker", "rm", "-f", name],
            capture_output=True,
            text=True,
            timeout=30,
        )
    subprocess.run(
        ["docker", "network", "rm", NETWORK],
        capture_output=True,
        text=True,
        timeout=30,
    )


atexit.register(global_cleanup)
