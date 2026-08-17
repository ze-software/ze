#!/usr/bin/env python3
"""Docker lifecycle and helpers for the Ze PPPoE Docker interop lab.

The lab runs Ze in either PPPoE role against a third-party implementation, and
each scenario directory declares which one in its `role` file:

  ze-client  Ze is the PPPoE client (`pppoe-client` interface kind) and
             accel-ppp is the access concentrator.
  ze-ac      Ze is the access concentrator (`pppoe` subsystem) and pppd with
             the rp-pppoe plugin is the client.

Both shapes put the two containers on one L2 segment on a user-defined Docker
bridge, so PPPoE discovery (broadcast EtherType 0x8863) reaches the peer.
"""

import atexit
import json
import os
import re
import subprocess
import time

_SUFFIX = os.environ.get("ZE_PPPOE_INTEROP_SUFFIX", str(os.getpid()))
NETWORK = "ze-pppoe-%s" % _SUFFIX
ZE_CONTAINER = "ze-pppoe-ze-%s" % _SUFFIX
ACCEL_CONTAINER = "ze-pppoe-accel-%s" % _SUFFIX
CLIENT_CONTAINER = "ze-pppoe-client-%s" % _SUFFIX

SUBNET = "172.30.0.0/24"
ZE_IP = "172.30.0.2"
ACCEL_IP = "172.30.0.3"
CLIENT_IP = "172.30.0.4"

# REST port the ze-ac scenarios expose so a check can read Ze's own session
# table (`show pppoe sessions`) rather than infer it from logs.
ZE_REST_PORT = 9099
ZE_REST_TOKEN = "ze-pppoe-interop"

ROLE_ZE_CLIENT = "ze-client"
ROLE_ZE_AC = "ze-ac"

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


# --- Ze (access concentrator) helpers ---------------------------------------


def docker_exec(container, cmd, timeout=30):
    """Run a command in a container. Returns (returncode, stdout+stderr)."""
    try:
        result = subprocess.run(
            ["docker", "exec", container] + cmd,
            capture_output=True,
            text=True,
            timeout=timeout,
        )
        return result.returncode, result.stdout + result.stderr
    except subprocess.TimeoutExpired:
        return 124, "(timed out after %ds)" % timeout


def wait_ze_ac_ready(timeout=60):
    """Ze's AC is ready once it reports the access interface it bound.

    subsystem.go logs "PPPoE interface configured" per resolved interface, which
    is emitted after the AF_PACKET discovery socket is open. A PADI sent before
    that line is not seen by anybody.
    """
    log_info("waiting for Ze PPPoE access concentrator (timeout %ds)..." % timeout)
    deadline = time.time() + timeout
    while time.time() < deadline:
        if "PPPoE interface configured" in docker_logs_all(ZE_CONTAINER):
            log_pass("Ze AC has bound its access interface")
            return
        time.sleep(1)
    log_fail("Ze AC did not bind an access interface within %ds" % timeout)
    print(docker_logs(ZE_CONTAINER, 60))
    raise AssertionError("Ze PPPoE AC not ready")


def ze_rest_execute(command, timeout=15):
    """Run a Ze CLI command through the REST API and return the parsed response.

    The ze-ac scenarios enable `environment.api-server.rest` so a check can read
    Ze's own state rather than infer it from log lines. The request runs INSIDE
    the container: the REST server refuses a non-loopback listen address because
    it has no TLS transport, so the bridge address is not an option. The body is
    plugin.Response: `status`, `data`, `error` (internal/component/plugin/types.go).
    """
    payload = json.dumps({"command": command})
    rc, body = docker_exec(
        ZE_CONTAINER,
        [
            "curl",
            "-sS",
            "--fail-with-body",
            "-X",
            "POST",
            "http://127.0.0.1:%d/api/v1/execute" % ZE_REST_PORT,
            "-H",
            "Authorization: Bearer " + ZE_REST_TOKEN,
            "-H",
            "Content-Type: application/json",
            "-d",
            payload,
        ],
        timeout=timeout,
    )
    if rc != 0:
        raise AssertionError(
            "REST %s unreachable (rc=%d): %s" % (command, rc, body.strip())
        )
    try:
        parsed = json.loads(body)
    except ValueError:
        raise AssertionError("REST %s returned non-JSON: %s" % (command, body[:200]))
    if not isinstance(parsed, dict):
        raise AssertionError(
            "REST %s returned %r, expected an object" % (command, parsed)
        )
    if parsed.get("status") == "error":
        raise AssertionError("REST %s: %s" % (command, parsed.get("error", "")))
    return parsed


def ze_pppoe_sessions():
    """Ze's own PPPoE session table (`show pppoe sessions`), as a list."""
    resp = ze_rest_execute("show pppoe sessions")
    data = resp.get("data")
    if data is None:
        return []
    if not isinstance(data, list):
        raise AssertionError(
            "show pppoe sessions returned %r, expected a list" % (data,)
        )
    return data


def wait_ze_rest_ready(timeout=45):
    log_info("waiting for Ze REST API (timeout %ds)..." % timeout)
    deadline = time.time() + timeout
    last = ""
    while time.time() < deadline:
        try:
            ze_pppoe_sessions()
            log_pass("Ze REST API answers")
            return
        except AssertionError as e:
            last = str(e)
            time.sleep(1)
    log_fail("Ze REST API never answered within %ds: %s" % (timeout, last))
    print(docker_logs(ZE_CONTAINER, 60))
    raise AssertionError("Ze REST API not ready")


# --- pppd / rp-pppoe client helpers -----------------------------------------


def pppd_dial(username, password, service_name="", logfile="/var/log/ppp/dial.log"):
    """Start pppd with the rp-pppoe plugin in the client container.

    Runs detached so the caller can poll for the outcome. `refuse-pap`,
    `refuse-eap` and the mschap refusals force CHAP-MD5, so what the AC offers
    is what is proven rather than whatever both ends happen to agree on.
    `noauth` means the client does not demand that Ze authenticate ITSELF --
    it does not weaken the AC's demand on the client.
    """
    docker_exec(CLIENT_CONTAINER, ["sh", "-c", "rm -f %s" % logfile], timeout=15)
    args = [
        "pppd",
        "plugin",
        "pppoe.so",
        "nic-eth0",
        "user",
        username,
        "password",
        password,
        "noauth",
        "refuse-pap",
        "refuse-eap",
        "refuse-mschap",
        "refuse-mschap-v2",
        "noipdefault",
        "nodefaultroute",
        "noaccomp",
        "nopcomp",
        "mtu",
        "1492",
        "mru",
        "1492",
        "lcp-echo-interval",
        "10",
        "lcp-echo-failure",
        "5",
        "maxfail",
        "1",
        "nodetach",
        "debug",
    ]
    if service_name:
        args.extend(["rp_pppoe_service", service_name])
    # `exec` under `docker exec -d`: pppd becomes the exec'd process rather than
    # a child of a shell that exits, so it is not orphaned mid-negotiation. Its
    # stdout/stderr (nodetach + debug) is the negotiation trace the check reads.
    subprocess.run(
        [
            "docker",
            "exec",
            "-d",
            CLIENT_CONTAINER,
            "sh",
            "-c",
            "exec %s >%s 2>&1" % (" ".join(args), logfile),
        ],
        capture_output=True,
        text=True,
        timeout=30,
    )


def pppd_log(logfile="/var/log/ppp/dial.log"):
    _, out = docker_exec(
        CLIENT_CONTAINER, ["sh", "-c", "cat %s 2>/dev/null" % logfile], timeout=15
    )
    return out


def pppd_failure_stage(log=None):
    """Name the stage the client stopped at, read from its own trace.

    A caller that polls for the END of the session (an address, an interface)
    learns only that the end never came, so its own message names the stage it
    was waiting for and not the stage that broke. The client's trace holds the
    answer, and a scenario whose failure names the wrong stage sends the reader
    to the wrong code.
    """
    if log is None:
        log = pppd_log()
    if "PADO" not in log:
        return "discovery: the AC sent no PADO"
    if "PADS" not in log:
        return "discovery: the AC sent no PADS"
    if "rcvd [CHAP Failure" in log or "authentication failed" in log:
        return "auth: the AC refused the credential"
    if "rcvd [LCP ConfReq" not in log:
        return "lcp: the AC never sent its own Configure-Request"
    if "sent [LCP ConfAck" not in log or "rcvd [LCP ConfAck" not in log:
        return "lcp: a Configure-Ack is missing in one direction"
    if "rcvd [CHAP Challenge" not in log and "sent [PAP AuthReq" not in log:
        return "auth: the AC asked for a method and never started it"
    if "rcvd [IPCP ConfAck" not in log:
        return "ipcp: no address was agreed"
    return "after IPCP"


def pppd_running():
    rc, _ = docker_exec(CLIENT_CONTAINER, ["pgrep", "-x", "pppd"], timeout=15)
    return rc == 0


def pppd_stop(timeout=20):
    """Ask pppd to terminate cleanly: SIGTERM makes it send LCP Terminate-Request
    and a PADT before it exits (pppd(8) EXIT STATUS / rp-pppoe plugin)."""
    docker_exec(CLIENT_CONTAINER, ["pkill", "-TERM", "-x", "pppd"], timeout=15)
    deadline = time.time() + timeout
    while time.time() < deadline:
        if not pppd_running():
            return True
        time.sleep(1)
    return False


def client_ppp_links():
    _, output = docker_exec(
        CLIENT_CONTAINER, ["ip", "-o", "link", "show", "type", "ppp"], timeout=15
    )
    found = set()
    for line in output.splitlines():
        match = re.match(r"\d+:\s+([^:@]+)", line)
        if match:
            found.add(match.group(1).strip())
    return found


def client_ppp_addr(iface):
    _, out = docker_exec(
        CLIENT_CONTAINER, ["ip", "-o", "addr", "show", "dev", iface], timeout=15
    )
    return out


def wait_client_ppp_up(timeout=None):
    if timeout is None:
        timeout = SESSION_TIMEOUT
    log_info(
        "waiting for a PPP interface in the pppd client (timeout %ds)..." % timeout
    )
    deadline = time.time() + timeout
    while time.time() < deadline:
        links = client_ppp_links()
        if links:
            iface = sorted(links)[0]
            log_pass("pppd client has PPP interface: %s" % iface)
            return iface
        if not pppd_running():
            stage = pppd_failure_stage()
            log_fail("pppd exited before a PPP interface appeared -- %s" % stage)
            print(pppd_log())
            raise AssertionError("pppd exited: %s" % stage)
        time.sleep(1)
    log_fail("no PPP interface appeared in the pppd client within %ds" % timeout)
    print(pppd_log())
    raise AssertionError("no PPP interface in the pppd client")


def client_ping(target, count=3, source_iface=None):
    cmd = ["ping", "-c", str(count), "-W", "3"]
    if source_iface:
        cmd.extend(["-I", source_iface])
    cmd.append(target)
    rc, out = docker_exec(CLIENT_CONTAINER, cmd, timeout=30)
    log_debug("ping output: %s" % out.strip())
    return rc == 0


# --- Scenario lifecycle ------------------------------------------------------


class Scenario:
    def __init__(self, scenario_dir):
        self.scenario_dir = scenario_dir
        self.name = os.path.basename(scenario_dir.rstrip("/"))
        self.role = read_role(scenario_dir)

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

        if self.role == ROLE_ZE_AC:
            self.setup_ze_ac()
        else:
            self.setup_ze_client()

    def setup_ze_ac(self):
        """Ze is the access concentrator; pppd with rp-pppoe is the client."""
        ze_conf = os.path.join(self.scenario_dir, "ze.conf")
        if not os.path.isfile(ze_conf):
            raise RuntimeError("missing ze.conf in %s" % self.name)

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
                "ze.log.pppoe=debug",
                "-e",
                "ze.log.l2tp=debug",
                "-e",
                "ZE_STORAGE_BLOB=false",
            ],
            cmd=["start", "/etc/ze/ze.conf"],
        )
        wait_ze_ac_ready(timeout=60)

        # The client only starts once the AC is listening: a PADI that arrives
        # before the AF_PACKET bind is lost, and pppd runs with maxfail 1.
        client_volumes = []
        if os.path.isdir("/lib/modules"):
            client_volumes.append("/lib/modules:/lib/modules:ro")
        docker_run(
            CLIENT_CONTAINER,
            "ze-pppoe-client",
            CLIENT_IP,
            volumes=client_volumes,
            extra_args=["--privileged"],
        )

    def setup_ze_client(self):
        """Ze is the PPPoE client; accel-ppp is the access concentrator."""
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
            # token, not the ones sharing a directory (ai/rules/architecture.md,
            # Sibling Call-Site Audit).
            cmd=["start", "/etc/ze/ze.conf"],
        )

    def teardown(self):
        docker_rm(ZE_CONTAINER)
        docker_rm(ACCEL_CONTAINER)
        docker_rm(CLIENT_CONTAINER)
        subprocess.run(
            ["docker", "network", "rm", NETWORK],
            capture_output=True,
            text=True,
            timeout=30,
        )

    def dump_logs(self, lines=80):
        if self.role == ROLE_ZE_AC:
            print("\n--- Ze (access concentrator) logs ---")
            print(docker_logs(ZE_CONTAINER, lines))
            print("\n--- pppd (client) log ---")
            print(pppd_log())
            return
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


def read_role(scenario_dir):
    """Read a scenario's `role` file. Required, and never defaulted.

    Which side Ze plays decides the whole container layout, so a scenario that
    does not say gets an error rather than a guess: a silent default would run
    the wrong lab and report the wrong thing as proven.
    """
    path = os.path.join(scenario_dir, "role")
    if not os.path.isfile(path):
        raise RuntimeError(
            "missing role file in %s: write %s or %s into it"
            % (os.path.basename(scenario_dir.rstrip("/")), ROLE_ZE_CLIENT, ROLE_ZE_AC)
        )
    with open(path, "r", encoding="utf-8") as fh:
        role = fh.read().strip()
    if role not in (ROLE_ZE_CLIENT, ROLE_ZE_AC):
        raise RuntimeError(
            "unknown role %r in %s: expected %s or %s"
            % (
                role,
                os.path.basename(scenario_dir.rstrip("/")),
                ROLE_ZE_CLIENT,
                ROLE_ZE_AC,
            )
        )
    return role


def global_cleanup():
    for name in [ZE_CONTAINER, ACCEL_CONTAINER, CLIENT_CONTAINER]:
        subprocess.run(
            ["docker", "rm", "-f", name], capture_output=True, text=True, timeout=30
        )
    subprocess.run(
        ["docker", "network", "rm", NETWORK], capture_output=True, text=True, timeout=30
    )


atexit.register(global_cleanup)
