#!/usr/bin/env python3
"""Docker lifecycle and helpers for the Ze PPPoE-client / accel-ppp interop lab.

Roles are inverted relative to the L2TP lab: here Ze is the PPPoE *client*
(`pppoe-client` interface kind) and accel-ppp is the access concentrator
(server). The two containers share an L2 segment on a user-defined Docker
bridge, so PPPoE discovery (broadcast EtherType 0x8863) reaches the AC.
"""

import atexit
import os
import re
import subprocess
import time

_SUFFIX = os.environ.get("ZE_PPPOE_INTEROP_SUFFIX", str(os.getpid()))
NETWORK = "ze-pppoe-%s" % _SUFFIX
ZE_CONTAINER = "ze-pppoe-ze-%s" % _SUFFIX
ACCEL_CONTAINER = "ze-pppoe-accel-%s" % _SUFFIX

SUBNET = "172.30.0.0/24"
ZE_IP = "172.30.0.2"
ACCEL_IP = "172.30.0.3"

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
    """Verify PPPoE host-kernel support from inside a privileged container.

    Probes for /dev/ppp (ppp_generic) and the `pppoe` (pppox PX_PROTO_OE)
    kernel module, which both lab containers share via the host kernel. Exits
    non-zero with a clear message when requirements are missing. Refuses to run
    if a skip-probe override is set: the proof must not be downgraded.
    """
    for key in ["ZE_PPPOE_SKIP_KERNEL_PROBE", "ze.pppoe.skip-kernel-probe"]:
        if key in os.environ:
            raise SystemExit(
                "refusing to run with %s set; full proof must not skip the kernel probe"
                % key
            )

    probe_name = "ze-pppoe-preflight-%s" % _SUFFIX
    modules_mount = (
        ["-v", "/lib/modules:/lib/modules:ro"] if os.path.isdir("/lib/modules") else []
    )
    try:
        result = subprocess.run(
            ["docker", "run", "--rm", "--privileged", "--name", probe_name]
            + modules_mount
            + [
                "alpine:3.21",
                "sh",
                "-c",
                "apk add --no-cache -q kmod > /dev/null 2>&1 && "
                "modprobe ppp_generic 2>/dev/null; "
                "modprobe pppoe 2>/dev/null; "
                "echo DEV_PPP=$(test -c /dev/ppp && echo ok || echo missing); "
                "echo PPPOE=$(test -d /sys/module/pppoe -o -f /proc/net/pppoe && echo ok || echo missing)",
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

    checks = {}
    for line in result.stdout.splitlines():
        if "=" in line:
            k, v = line.strip().split("=", 1)
            checks[k] = v

    missing = []
    if checks.get("DEV_PPP") != "ok":
        missing.append("/dev/ppp (PPP character device)")
    if checks.get("PPPOE") != "ok":
        missing.append("pppoe (PPPoE pppox kernel module)")

    if missing:
        raise SystemExit(
            "host kernel missing PPPoE requirements: %s" % ", ".join(missing)
        )

    log_pass("preflight: host kernel has PPPoE support")


# --- Ze (client) helpers -----------------------------------------------------


def wait_ze_log(needle, timeout=None):
    if timeout is None:
        timeout = SESSION_TIMEOUT
    log_info("waiting for Ze log: '%s' (timeout %ds)..." % (needle, timeout))
    deadline = time.time() + timeout
    while time.time() < deadline:
        if needle in docker_logs_all(ZE_CONTAINER):
            log_pass("Ze log contains: %s" % needle)
            return
        time.sleep(2)
    log_fail("Ze log missing: '%s' within %ds" % (needle, timeout))
    print(docker_logs(ZE_CONTAINER, 40))
    raise AssertionError("Ze log missing: %s" % needle)


def ze_ppp_links():
    output = docker_exec_quiet(
        ZE_CONTAINER, ["ip", "-o", "link", "show", "type", "ppp"]
    )
    found = set()
    for line in output.splitlines():
        match = re.match(r"\d+:\s+([^:@]+)", line)
        if match:
            found.add(match.group(1).strip())
    return found


def ze_ppp_addr(iface):
    return docker_exec_quiet(ZE_CONTAINER, ["ip", "-o", "addr", "show", "dev", iface])


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


def ze_ping(target, count=3):
    try:
        result = subprocess.run(
            [
                "docker",
                "exec",
                ZE_CONTAINER,
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


# --- accel-ppp (server) helpers ---------------------------------------------


def wait_accel_ready(timeout=60):
    """accel-pppd is ready once its control socket answers accel-cmd."""
    log_info("waiting for accel-ppp control socket (timeout %ds)..." % timeout)
    deadline = time.time() + timeout
    while time.time() < deadline:
        try:
            result = subprocess.run(
                ["docker", "exec", ACCEL_CONTAINER, "accel-cmd", "show", "stat"],
                capture_output=True,
                text=True,
                timeout=10,
            )
            if result.returncode == 0:
                log_pass("accel-ppp control socket is up")
                return
        except subprocess.TimeoutExpired:
            pass
        time.sleep(1)
    log_fail("accel-ppp did not become ready within %ds" % timeout)
    print(docker_logs(ACCEL_CONTAINER, 40))
    raise AssertionError("accel-ppp not ready")


def accel_show_sessions():
    return docker_exec_quiet(ACCEL_CONTAINER, ["accel-cmd", "show", "sessions"])


def accel_session_count():
    """Count active subscriber sessions in accel-ppp's session table.

    `accel-cmd show sessions` prints a header row plus one row per session.
    Data rows carry a kernel ppp interface name (pppN); the header does not, so
    counting `pppN` matches is enough without parsing columns.
    """
    output = accel_show_sessions()
    return sum(1 for line in output.splitlines() if re.search(r"\bppp\d+\b", line))


def wait_accel_session(timeout=30):
    deadline = time.time() + timeout
    while time.time() < deadline:
        if accel_session_count() >= 1:
            log_pass("accel-ppp reports an active subscriber session")
            return
        time.sleep(2)
    log_fail("accel-ppp never reported a session within %ds" % timeout)
    print(accel_show_sessions())
    raise AssertionError("accel-ppp session not established")


def wait_accel_session_gone(timeout=30):
    deadline = time.time() + timeout
    while time.time() < deadline:
        if accel_session_count() == 0:
            log_pass("accel-ppp session table is empty after teardown")
            return
        time.sleep(2)
    log_fail("accel-ppp still reports a session after %ds" % timeout)
    print(accel_show_sessions())
    raise AssertionError("accel-ppp session not torn down")


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

        accel_conf = os.path.join(self.scenario_dir, "accel-ppp.conf")
        chap_secrets = os.path.join(self.scenario_dir, "chap-secrets")
        ze_conf = os.path.join(self.scenario_dir, "ze.conf")
        for path in (accel_conf, chap_secrets, ze_conf):
            if not os.path.isfile(path):
                raise RuntimeError(
                    "missing %s in %s" % (os.path.basename(path), self.name)
                )

        # Start the access concentrator first so it answers Ze's PADI.
        accel_volumes = [
            "%s:/etc/accel-ppp.conf:ro" % os.path.abspath(accel_conf),
            "%s:/etc/accel-ppp/chap-secrets:ro" % os.path.abspath(chap_secrets),
        ]
        if os.path.isdir("/lib/modules"):
            accel_volumes.append("/lib/modules:/lib/modules:ro")
        docker_run(
            ACCEL_CONTAINER,
            "ze-pppoe-accel",
            ACCEL_IP,
            volumes=accel_volumes,
            extra_args=["--privileged"],
        )
        wait_accel_ready(timeout=60)

        # Start Ze as the PPPoE client.
        ze_volumes = ["%s:/etc/ze/ze.conf:ro" % os.path.abspath(ze_conf)]
        if os.path.isdir("/lib/modules"):
            ze_volumes.append("/lib/modules:/lib/modules:ro")
        docker_run(
            ZE_CONTAINER,
            "ze-pppoe-interop",
            ZE_IP,
            volumes=ze_volumes,
            extra_args=[
                "--privileged",
                "-e",
                # pppoe-client logs under the "interface" component domain.
                "ze.log.interface=debug",
                "-e",
                "ZE_STORAGE_BLOB=false",
            ],
            # `start <config>`: the bare `ze <config>` launch form was removed from the CLI,
            # and the image ENTRYPOINT is `tini -- ze`, so the old cmd died with "unknown
            # command". Same defect as test/interop/interop.py. The first pass fixed only
            # the four Docker labs; a later audit found the dead form still live at eight
            # more executable sites (scripts/evidence/effective-{l2tp-ppp,pppoe-accel,
            # vrrp-keepalived,vpp,vpp-iface}.py, test/stress/harness.py,
            # test/plugin/lg-graph-lab/run.sh, docker/compose.yaml) and in five docs. The
            # audit is only complete when it covers EVERY invocation site of the bare
            # token, not the ones sharing a directory (ai/rules/before-writing-code.md,
            # Sibling Call-Site Audit).
            cmd=["start", "/etc/ze/ze.conf"],
        )

    def teardown(self):
        docker_rm(ZE_CONTAINER)
        docker_rm(ACCEL_CONTAINER)
        subprocess.run(
            ["docker", "network", "rm", NETWORK],
            capture_output=True,
            text=True,
            timeout=30,
        )

    def dump_logs(self, lines=80):
        print("\n--- Ze (client) logs ---")
        print(docker_logs(ZE_CONTAINER, lines))
        print("\n--- accel-ppp (server) logs ---")
        print(docker_logs(ACCEL_CONTAINER, lines))

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
    for name in [ZE_CONTAINER, ACCEL_CONTAINER]:
        subprocess.run(
            ["docker", "rm", "-f", name], capture_output=True, text=True, timeout=30
        )
    subprocess.run(
        ["docker", "network", "rm", NETWORK], capture_output=True, text=True, timeout=30
    )


atexit.register(global_cleanup)
