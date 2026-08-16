#!/usr/bin/env python3
"""Shared helpers for Ze interoperability tests.

Provides container management, daemon query classes (FRR, BIRD, Ze),
assertion helpers, and scenario lifecycle management.
"""

import atexit
import json
import os
import re
import shutil
import subprocess
import time

# Network and container naming (PID suffix avoids conflicts between concurrent runs).
_SUFFIX = os.environ.get("ZE_INTEROP_SUFFIX", str(os.getpid()))
NETWORK = "ze-iop-%s" % _SUFFIX
ZE_CONTAINER = "ze-iop-ze-%s" % _SUFFIX
FRR_CONTAINER = "ze-iop-frr-%s" % _SUFFIX
BIRD_CONTAINER = "ze-iop-bird-%s" % _SUFFIX
GOBGP_CONTAINER = "ze-iop-gobgp-%s" % _SUFFIX
BMP_CONTAINER = "ze-iop-bmp-%s" % _SUFFIX
RPKI_CONTAINER = "ze-iop-rpki-%s" % _SUFFIX
KEEPALIVED_CONTAINER = "ze-iop-keepalived-%s" % _SUFFIX
STAYRTR_CONTAINER = "ze-iop-stayrtr-%s" % _SUFFIX
INJECT_CONTAINER = "ze-iop-inject-%s" % _SUFFIX
SPEAKER_CONTAINER = "ze-iop-speaker-%s" % _SUFFIX
SPEAKER2_CONTAINER = "ze-iop-speaker2-%s" % _SUFFIX

# IP addresses on the test network.
_BASE_SUBNET_PREFIX = "172.30.0."
_SUBNET_POOLS = ((172, 30), (172, 31), (10, 254))
SUBNET_PREFIX = _BASE_SUBNET_PREFIX
SUBNET_CIDR = "172.30.0.0/24"
ZE_IP = "172.30.0.2"
FRR_IP = "172.30.0.3"
BIRD_IP = "172.30.0.4"
GOBGP_IP = "172.30.0.5"
BMP_IP = "172.30.0.6"
RPKI_IP = "172.30.0.7"
KEEPALIVED_IP = "172.30.0.8"
INJECT_IP = "172.30.0.9"
SPEAKER_IP = "172.30.0.10"
SPEAKER2_IP = "172.30.0.11"
STAYRTR_IP = "172.30.0.12"

# StayRTR's metrics port, which also serves the cache's own VRP set at
# /rpki.json -- the reading a scenario takes to learn what the cache MEANT,
# independently of what Ze decoded from the RTR wire. Its RTR port (8282) is
# named where it is used, in Dockerfile.stayrtr and in each scenario's ze.conf,
# and nothing in this module dials it.
STAYRTR_HTTP_PORT = 9847

# The VRRP virtual IP the ze and keepalived scenarios contend for. It is
# deliberately NOT any container's own address: it is owned by whichever router
# currently holds mastership, which is the whole point of the protocol.
VRRP_VIP = "172.30.0.100"

# CLI access to the Ze daemon inside the container.
#
# `ze cli` reaches the daemon over SSH, and the daemon starts NO SSH listener
# unless the config asks for one (infraSetup, cmd/ze/hub/infra_setup.go). No
# scenario ze.conf asked, so every CLI query answered "no credentials for
# 127.0.0.1:2222" and the RIB helpers below could not run at all. The harness
# appends the two blocks to the RENDERED copy of every ze.conf
# (_render_scenario_dir), so no scenario carries boilerplate and none can
# forget it. The same arrangement drives test/ipsec-interop/lab.py.
#
# The bcrypt hash is the published cost-4 hash the functional suite already uses
# (test/plugin/authz-default.ci), so this adds no new secret.
ZE_CLI_USER = "interop"
ZE_CLI_PASSWORD = "testpass"
ZE_CLI_CONFIG = """
system {
\tauthentication {
\t\tuser interop {
\t\t\tpassword "$2a$04$UlwuiuH82Unfsq.XEMPGJeDkXwbm3KW.nvVaVXOd/JeFK8VjMjrQO"
\t\t}
\t}
}

environment {
\tssh {
\t\tenabled true
\t\tserver main {
\t\t\tip 127.0.0.1;
\t\t\tport 2222;
\t\t}
\t}
}
"""

# IPv6 link-local must be enabled in the container netns for the OSPFv3 (ospf-v6-frr)
# scenario: OSPFv3 runs over ff02::5 on eth0. These sysctls are no-ops on hosts that
# already default disable_ipv6=0 (the common case) and harmless to IPv4-only scenarios.
_IPV6_SYSCTLS = [
    "--sysctl",
    "net.ipv6.conf.all.disable_ipv6=0",
    "--sysctl",
    "net.ipv6.conf.default.disable_ipv6=0",
]

_SCRIPT_DIR = os.path.dirname(os.path.abspath(__file__))
_PROJECT_ROOT = os.path.abspath(os.path.join(_SCRIPT_DIR, "..", ".."))
_RENDER_ROOT = os.path.join(_PROJECT_ROOT, "tmp", "interop-rendered")

# Default timeout for session establishment (seconds).
try:
    SESSION_TIMEOUT = int(os.environ.get("SESSION_TIMEOUT", "90"))
except ValueError:
    SESSION_TIMEOUT = 90

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


# --- Polling ------------------------------------------------------------------


def poll(probe, ready, timeout, interval=3, what="query"):
    """Call probe() until ready(result) is true, and return the last result.

    A retry loop must tell "not ready yet" from "cannot run at all", and only
    the first of those is worth waiting out. probe() raising is the second kind:
    Ze.cli raises for an auth refusal, an unreachable daemon and a rejected
    command alike, and a loop written as `while time.time() < deadline: db =
    Ze().cli(...)` dies on the first one instead of retrying to its deadline.

    So one raise is TOLERATED and every tolerated raise is REPORTED. It is never
    swallowed: the empty string that used to come back from docker_exec_quiet is
    what let a harness fault read as an empty result, and the assertion that
    followed blamed the protocol implementation.

    | Outcome | What this does |
    |---------|----------------|
    | ready(result) is true | return that result |
    | the deadline passes, some call succeeded | return the last successful result, so the caller's own assertion names the protocol fault |
    | the deadline passes, EVERY call raised | raise, carrying the last error: nothing was ever measured, and no verdict about the protocol is available |

    `what` names the probe in that last message. `ready` decides success; pass
    `bool` when any truthy result will do.
    """
    deadline = time.time() + timeout
    result = None
    succeeded = False
    tolerated = 0
    last_error = None
    while True:
        try:
            result = probe()
            succeeded = True
            if ready(result):
                return result
        except RuntimeError as exc:
            tolerated += 1
            last_error = exc
            log_info("%s failed (attempt %d), retrying: %s" % (what, tolerated, exc))
        if time.time() >= deadline:
            break
        time.sleep(interval)
    if not succeeded:
        log_fail(
            "%s never ran: %d attempts in %ds, all failed" % (what, tolerated, timeout)
        )
        raise RuntimeError(
            "%s never succeeded in %ds (%d attempts, all failed): %s "
            "(harness failure, not a protocol result)"
            % (what, timeout, tolerated, last_error)
        )
    if tolerated:
        log_info(
            "%s tolerated %d transient failure(s) before the deadline"
            % (what, tolerated)
        )
    return result


def _set_subnet_prefix(prefix):
    """Update global peer IPs for the allocated /24, both families.

    EVERY address the harness pins with `--ip` is rebased here. One that is not
    keeps its 172.30.0.x spelling while the network is created on the fallback
    /24, and `docker run --ip` then names an address outside the network's own
    subnet and fails the container outright. Five did until 2026-08-12
    (keepalived, inject, both speakers, and the VRRP virtual address), so the
    fallback path -- taken whenever 172.30.0.0/24 is already in use, which is
    exactly the concurrent-run case the fallback exists for -- could not start
    those scenarios at all.
    """
    global SUBNET_PREFIX, SUBNET_CIDR, ZE_IP, FRR_IP, BIRD_IP, GOBGP_IP, BMP_IP, RPKI_IP
    global KEEPALIVED_IP, STAYRTR_IP, INJECT_IP, SPEAKER_IP, SPEAKER2_IP, VRRP_VIP
    global V6_SUBNET_CIDR, V6_SUBNET_PREFIX, ZE_IP6, FRR_IP6
    if not prefix.endswith("."):
        prefix += "."
    SUBNET_PREFIX = prefix
    SUBNET_CIDR = "%s0/24" % prefix
    ZE_IP = "%s2" % prefix
    FRR_IP = "%s3" % prefix
    BIRD_IP = "%s4" % prefix
    GOBGP_IP = "%s5" % prefix
    BMP_IP = "%s6" % prefix
    RPKI_IP = "%s7" % prefix
    KEEPALIVED_IP = "%s8" % prefix
    INJECT_IP = "%s9" % prefix
    SPEAKER_IP = "%s10" % prefix
    SPEAKER2_IP = "%s11" % prefix
    STAYRTR_IP = "%s12" % prefix
    VRRP_VIP = "%s100" % prefix
    V6_SUBNET_CIDR = _v6_subnet_for(prefix)
    V6_SUBNET_PREFIX = V6_SUBNET_CIDR.split("/")[0]
    ZE_IP6 = _v6_for(ZE_IP)
    FRR_IP6 = _v6_for(FRR_IP)


def _candidate_subnet_prefixes():
    """Yield /24 prefixes to try for this scenario run."""
    explicit_prefix = os.environ.get("ZE_INTEROP_SUBNET_PREFIX")
    if explicit_prefix:
        yield explicit_prefix
        return

    explicit_index = os.environ.get("ZE_INTEROP_SUBNET_INDEX")
    if explicit_index:
        try:
            index = int(explicit_index)
        except ValueError as exc:
            raise RuntimeError(
                "invalid ZE_INTEROP_SUBNET_INDEX %r" % explicit_index
            ) from exc
        max_index = len(_SUBNET_POOLS) * 256 - 1
        if index < 0 or index > max_index:
            raise RuntimeError(
                "ZE_INTEROP_SUBNET_INDEX must be between 0 and %d" % max_index
            )
        first, second = _SUBNET_POOLS[index // 256]
        yield "%d.%d.%d." % (first, second, index % 256)
        return

    for first, second in _SUBNET_POOLS:
        for index in range(256):
            yield "%d.%d.%d." % (first, second, index)


def _v6_subnet_for(prefix):
    """Derive a unique ULA /64 from the IPv4 /24 prefix.

    OSPFv3 (and any native-IPv6 wire protocol) needs a real IPv6 link-local on the
    container interface, which Docker only enables when the network is IPv6-capable.
    The ULA is derived from the IPv4 octets so it is unique per allocated /24 and the
    overlap-retry in _create_network covers any collision.
    """
    octets = prefix.rstrip(".").split(".")
    b = int(octets[1]) if len(octets) > 1 else 0
    c = int(octets[2]) if len(octets) > 2 else 0
    return "fd00:%x:%x::/64" % (b, c)


def _v6_for(ip):
    """Derive a container's IPv6 address on the dual-stack network from its IPv4 one.

    ONE derivation, so a scenario file, `_set_subnet_prefix` and `docker_run` cannot
    disagree about where a container sits. The IPv4 host octet is carried across as
    text (`.3` -> `::3`), which is injective, so distinct IPv4 addresses stay distinct
    -- it is a naming scheme, not an arithmetic conversion.
    """
    return V6_SUBNET_PREFIX + ip.rsplit(".", 1)[1]


# IPv6 addresses on the test network, the mirror of the IPv4 block near the top of
# this file. `_BASE_V6_PREFIX` is the token a scenario file writes; `_render_scenario_dir`
# replaces it with `V6_SUBNET_PREFIX` exactly as it replaces `_BASE_SUBNET_PREFIX`, and
# `_needs_ipv6_wire` reads a scenario naming the token as the request for a dual-stack
# network. Both are DERIVED from the IPv4 base, so there is no second copy to drift.
_BASE_V6_PREFIX = _v6_subnet_for(_BASE_SUBNET_PREFIX).split("/")[0]
V6_SUBNET_CIDR = _v6_subnet_for(_BASE_SUBNET_PREFIX)
V6_SUBNET_PREFIX = _BASE_V6_PREFIX
ZE_IP6 = _v6_for(ZE_IP)
FRR_IP6 = _v6_for(FRR_IP)

# Whether the current lab network carries IPv6. `docker_run` reads it to decide
# whether it can ask for a deterministic `--ip6`: docker rejects the flag on a
# network with no IPv6 subnet, so it cannot simply be passed unconditionally.
_DUAL_STACK = False


def _create_network(dual_stack=False):
    """Create the Docker network, retrying on overlapping subnets.

    dual_stack adds an IPv6 ULA subnet (and --ipv6) so containers get an IPv6
    link-local AND a global address on eth0; required for OSPFv3 and for any
    scenario that names an IPv6 peer or next hop, opt-in so IPv4-only scenarios
    are unaffected.
    """
    global _DUAL_STACK
    _DUAL_STACK = dual_stack
    last_error = ""
    forced = os.environ.get("ZE_INTEROP_SUBNET_PREFIX") or os.environ.get(
        "ZE_INTEROP_SUBNET_INDEX"
    )
    for prefix in _candidate_subnet_prefixes():
        _set_subnet_prefix(prefix)
        create_args = ["docker", "network", "create", "--subnet=%s" % SUBNET_CIDR]
        if dual_stack:
            create_args += ["--ipv6", "--subnet=%s" % V6_SUBNET_CIDR]
        create_args.append(NETWORK)
        result = subprocess.run(
            create_args,
            capture_output=True,
            text=True,
            timeout=30,
        )
        if result.returncode == 0:
            log_debug(
                "created Docker network %s with subnet %s" % (NETWORK, SUBNET_CIDR)
            )
            return
        last_error = result.stderr.strip()
        if "already exists" in last_error:
            log_debug("reusing existing Docker network %s" % NETWORK)
            return
        if forced or "overlap" not in last_error.lower():
            raise RuntimeError("docker network create failed: %s" % last_error)

    raise RuntimeError(
        "docker network create failed: no available /24: %s" % last_error
    )


def _render_scenario_dir(source_dir, name):
    """Copy a scenario, replace the Docker subnet prefix, enable CLI access.

    ze.conf gets ZE_CLI_CONFIG appended so `ze cli` can reach the daemon. It is
    done here, on the rendered copy, rather than in 115 scenario files: a
    scenario that forgot the block would fail its RIB assertions for a reason
    that has nothing to do with what it tests.
    """
    target = os.path.join(_RENDER_ROOT, "%s-%s" % (name, _SUFFIX))
    shutil.rmtree(target, ignore_errors=True)
    for root, dirs, files in os.walk(source_dir):
        dirs[:] = [d for d in dirs if d != "__pycache__"]
        rel = os.path.relpath(root, source_dir)
        dst_root = target if rel == "." else os.path.join(target, rel)
        os.makedirs(dst_root, exist_ok=True)
        for fname in files:
            if fname.endswith(".pyc"):
                continue
            src = os.path.join(root, fname)
            dst = os.path.join(dst_root, fname)
            with open(src, "rb") as fh:
                data = fh.read()
            try:
                text = data.decode("utf-8")
            except UnicodeDecodeError:
                with open(dst, "wb") as fh:
                    fh.write(data)
            else:
                text = text.replace(_BASE_SUBNET_PREFIX, SUBNET_PREFIX)
                text = text.replace(_BASE_V6_PREFIX, V6_SUBNET_PREFIX)
                if rel == "." and fname == "ze.conf":
                    text = text.rstrip("\n") + "\n" + ZE_CLI_CONFIG
                with open(dst, "w", encoding="utf-8") as fh:
                    fh.write(text)
            shutil.copymode(src, dst)
    return target


# --- Docker helpers ----------------------------------------------------------


def docker_exec(container, cmd, env=None):
    """Run command in container, return stdout. Raises on failure.

    `env` is a dict passed with `-e`, for a secret that must not appear in the
    scenario's config or in the container's own environment.
    """
    args = ["docker", "exec"]
    for key, value in (env or {}).items():
        args.extend(["-e", "%s=%s" % (key, value)])
    args.append(container)
    try:
        result = subprocess.run(
            args + cmd,
            capture_output=True,
            text=True,
            timeout=30,
        )
    except subprocess.TimeoutExpired:
        raise RuntimeError(
            "docker exec %s %s timed out after 30s" % (container, " ".join(cmd))
        )
    if result.returncode != 0:
        raise RuntimeError(
            "docker exec %s %s failed (rc=%d): %s"
            % (container, " ".join(cmd), result.returncode, result.stderr.strip())
        )
    return result.stdout


def docker_exec_quiet(container, cmd):
    """Run command in container, return stdout or empty string on failure."""
    try:
        result = subprocess.run(
            ["docker", "exec", container] + cmd,
            capture_output=True,
            text=True,
            timeout=30,
        )
        if result.returncode != 0:
            return ""
        return result.stdout
    except Exception:
        return ""


def docker_run(
    name, image, ip, volumes=None, caps=None, extra_args=None, cmd=None, env=None
):
    """Start a container.

    `env` is a dict of environment variables passed with `-e`. It exists so a
    process plugin inside the container can DERIVE a barrier from the harness
    budget rather than repeating a constant: a plugin whose own timeout is
    shorter than `SESSION_TIMEOUT` shuts ze down while the check is still
    waiting, and the red then surfaces two hops from its cause.
    """
    args = ["docker", "run", "-d", "--name", name, "--network", NETWORK, "--ip", ip]
    # On a dual-stack network the container must sit at a KNOWN IPv6 address, the
    # same way `--ip` pins the IPv4 one. Without this docker assigns from the ULA
    # pool and a scenario that names a peer or a next hop by its IPv6 address is
    # naming an address nothing guarantees. `--ip6` is rejected on a network with
    # no IPv6 subnet, so it is passed only when `_create_network` made one.
    if _DUAL_STACK:
        args.extend(["--ip6", _v6_for(ip)])
    for cap in caps or []:
        args.extend(["--cap-add", cap])
    for vol in volumes or []:
        args.extend(["-v", vol])
    for key, value in (env or {}).items():
        args.extend(["-e", "%s=%s" % (key, value)])
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


def docker_rm(name, strict=False):
    """Remove container, ignore if not exists.

    Two contracts, selected by `strict`, because the same removal runs at two
    moments that owe opposite answers. This mirrors `docker_logs`.

    The default is the CLEANUP contract: nothing raises and the run goes on.
    `Scenario.teardown` runs from `run.py`'s `finally`, so an exception here
    escapes that `finally`, escapes `main`, and the run ends with NO summary,
    discarding every tally the suite had accumulated. A timeout and an unusable
    binary are REPORTED on that path; a non-zero exit is not, because the exit
    code is read only to decide, and the cleanup contract decides nothing.

    `strict=True` is the PRE-CLEAN contract, and it must raise. `Scenario.setup`
    removes leftovers BEFORE starting its own containers, and a removal that
    failed silently there leaves this scenario running beside another one's
    daemon on the same address. Nothing downstream catches that: a container
    THIS scenario starts collides by name and `docker_run` raises, but a stale
    peer it never starts does not, and `_create_network` accepts a network that
    already exists. Swallowing on that path is a removed guard, not tidiness.

    THREE failure shapes, and all of them leave the container standing, so all
    of them deny under `strict` and none of them raises under cleanup.

      * the call never answers. `TimeoutExpired`.
      * docker answers with an error. A non-zero exit: "removal already in
        progress", a device-busy driver error. `docker rm -f` on a container
        that does not exist exits 0 with no output (measured on docker 29.7.1),
        so a non-zero exit here is always a real failure and never the ordinary
        nothing-to-remove case.
      * the call cannot run at all. `subprocess.run` reports a missing or
        unusable docker binary as an `OSError`, which is why
        `observer_failure_note` catches that type beside `RuntimeError`.
        `run.py` probes `docker info` once before the suite, so this needs
        docker to become unusable mid-run, and an uncaught one on the cleanup
        path escapes the `finally` and takes the summary.
    """
    try:
        result = subprocess.run(
            ["docker", "rm", "-f", name],
            capture_output=True,
            text=True,
            timeout=30,
        )
    except subprocess.TimeoutExpired:
        if strict:
            raise RuntimeError(
                "docker rm -f %s timed out after 30s: a leftover container "
                "would race this scenario" % name
            )
        print(
            "--- docker rm %s timed out after 30s, container can be left behind ---"
            % name
        )
        return
    except OSError as exc:
        if strict:
            raise RuntimeError(
                "docker rm -f %s could not run (%s): a leftover container "
                "would race this scenario" % (name, exc)
            )
        print(
            "--- docker rm %s could not run (%s), container can be left behind ---"
            % (name, exc)
        )
        return
    if strict and result.returncode != 0:
        raise RuntimeError(
            "docker rm -f %s failed (exit %d): %s -- a leftover container would "
            "race this scenario"
            % (name, result.returncode, (result.stderr or "").strip() or "no stderr")
        )


def docker_signal(container, signal="TERM"):
    """Send a signal to the container's PID 1.

    The ze image entrypoint is `tini -- ze`, and tini forwards the signal to the
    ze process, so this is the same administrative stop an init system gives a
    daemon. `docker exec kill` is not equivalent: it would need the ze PID, and
    a scenario that guesses one asserts against the wrong process when it is
    wrong.
    """
    try:
        result = subprocess.run(
            ["docker", "kill", "--signal", signal, container],
            capture_output=True,
            text=True,
            timeout=30,
        )
    except subprocess.TimeoutExpired:
        raise RuntimeError("docker kill --signal %s %s timed out" % (signal, container))
    if result.returncode != 0:
        raise RuntimeError(
            "docker kill --signal %s %s failed: %s"
            % (signal, container, result.stderr.strip())
        )


def docker_start(container):
    """Start a stopped container again, with the config it already has.

    The pair of `docker_signal`. A scenario that proves a router comes BACK --
    a VRRP master returning and preempting, a peer re-establishing -- must
    restart the same container: `docker run` a second time would build a new
    one, and any state the first accumulated (its interfaces, its addresses,
    its logs) would be gone from the evidence the check reads.
    """
    try:
        result = subprocess.run(
            ["docker", "start", container],
            capture_output=True,
            text=True,
            timeout=60,
        )
    except subprocess.TimeoutExpired:
        raise RuntimeError("docker start %s timed out" % container)
    if result.returncode != 0:
        raise RuntimeError(
            "docker start %s failed: %s" % (container, result.stderr.strip())
        )


def docker_logs(container, lines=30, strict=False):
    """Get last N lines of container logs.

    Two contracts, selected by `strict`, because two kinds of caller read this.

    The default is the DISPLAY contract: a docker failure comes back as a
    string, so a diagnostic dump on an already-failing path never raises a
    second time over the first fault.

    `strict=True` is the DECISION contract: a caller that reads the log to
    decide something gets a RuntimeError instead. "" and "(docker logs timed
    out)" are facts about docker, and a decision that consumes them as facts
    about the CONTAINER answers on no evidence (ai/rules/evidence.md).
    """
    try:
        result = subprocess.run(
            ["docker", "logs", container, "--tail", str(lines)],
            capture_output=True,
            text=True,
            timeout=15,
        )
    except subprocess.TimeoutExpired:
        if strict:
            raise RuntimeError(
                "docker logs %s timed out after 15s: the log was never read" % container
            )
        return "(docker logs timed out)"
    if strict and result.returncode != 0:
        raise RuntimeError(
            "docker logs %s failed (exit %d): %s"
            % (container, result.returncode, result.stderr.strip() or "no stderr")
        )
    return result.stdout + result.stderr


# --- Observer failures -------------------------------------------------------

# The sentinel `runtime_fail` (test/scripts/ze_api.py) writes to a process
# plugin's stderr, which ze relays to its own. The `.ci` runner rejects it
# implicitly in `validateLogging` (internal/test/runner/runner_validate.go). This
# lab has no such reject, so a plugin that signals failure here only shows up as
# whatever the check happens to miss next -- a route that never arrived, or a
# container that stopped being healthy. Both name the symptom, never the cause.
OBSERVER_FAIL = "ZE-OBSERVER-FAIL"


def observer_fail_line(container=None, lines=2000):
    """Return the first ZE-OBSERVER-FAIL line ze relayed, or None.

    None means ONE thing: the log was read and holds no sentinel. A log that
    could not be read raises instead, and keeping those two apart is the whole
    value of this helper. `docker_logs` answers a docker failure with "" and a
    timeout with "(docker logs timed out)"; neither holds the sentinel, so a
    scan over either one reports a healthy plugin on no evidence, which is the
    fail-open shape `ai/rules/evidence.md` bans.

    `lines` is a TRUNCATION BOUND: this reads `docker logs --tail 2000`, never
    the whole log. The tail is the right end to read, because `runtime_fail`
    (test/scripts/ze_api.py) requests shutdown immediately after writing the
    sentinel, so ze stops within a few lines of it. A scenario whose ze writes
    more than 2000 lines between the sentinel and the stop needs a larger bound.
    """
    for line in docker_logs(container or ZE_CONTAINER, lines, strict=True).splitlines():
        if OBSERVER_FAIL in line:
            return line.strip()
    return None


def raise_if_observer_failed(when, container=None):
    """Raise with the PLUGIN's own message when it signalled failure.

    Raises on an unreadable log too, with the docker error rather than the
    plugin's message: "I could not look" is not "the plugin is fine".
    """
    line = observer_fail_line(container)
    if line is None:
        return
    log_fail("a process plugin signalled failure (%s)" % when)
    raise AssertionError("process plugin failed: %s" % line)


def observer_failure_note(container=None):
    """Give a caller that must not raise a COMPLETE line to print, or None.

    Three states reach a caller, and two of them make DIFFERENT claims, so the
    wording belongs here rather than at the call site:

      * the sentinel is present. The plugin did signal failure, so the line
        names it as the cause.
      * the log cannot be read. Nothing is known, so the line says that.
      * the log was read and holds no sentinel. None.

    One writer, one claim. A caller that received the bare fact and added its
    own prefix asserted a cause it had not established: `run.py` printed "the
    cause is a process plugin" in front of a sentence saying the opposite, for
    every scenario whose failure preceded ze's container (round 5 review).

    Nothing `docker_logs` produces raises out of here. `run.py`'s handler
    already holds the scenario's failure, and a second exception there would
    replace the failure being reported. `OSError` is caught beside
    `RuntimeError` because `subprocess.run` reports a missing or unusable
    `docker` binary that way, and the strict contract converts only the
    failures docker itself reports.
    """
    try:
        line = observer_fail_line(container)
    except (RuntimeError, OSError) as exc:
        return (
            "a process-plugin failure cannot be ruled out: ze's log could not "
            "be read: %s" % exc
        )
    if line is None:
        return None
    return "the cause is a process plugin: %s" % line


# --- FRR helpers -------------------------------------------------------------


class FRR:
    """Helpers for querying FRR via vtysh."""

    def __init__(self, container=None, ip=None):
        self.container = container or FRR_CONTAINER
        self.ip = ip or FRR_IP

    def _vtysh_quiet(self, command):
        """Run a vtysh command, return stdout or empty string on failure."""
        return docker_exec_quiet(self.container, ["vtysh", "-c", command])

    def _vtysh(self, command):
        """Run a vtysh command, return stdout, raise when it could not be run.

        The counterpart of `_vtysh_quiet`, for every read whose value leaves the
        helper as a FACT about the peer. `_vtysh_quiet` answers "" for a vtysh
        that could not run and for a vtysh that ran and printed nothing, so
        `"Full" in out` is False for a dead container exactly as it is for a
        peer that is really down. A caller that asserts ABSENCE then passes on
        the harness fault: `ospf-convergence-frr` reads `adjacency_down()` to
        prove the dead interval expired, and a failed read proves it in 0s.

        Empty output stays a real answer here, because `show ip route ospf`
        prints nothing when no OSPF route exists. The fault is therefore read
        from the EXIT STATUS, which is what `docker_exec` raises on.
        `wait_containers_healthy` has already proven `vtysh -c "show version"`
        exits 0 in this container before any scenario runs, so a non-zero exit
        later is a fault rather than a start-up race.

        A POLL LOOP keeps `_vtysh_quiet`: it waits for a sentinel that "" can
        never match, and its own deadline raises.
        """
        return docker_exec(self.container, ["vtysh", "-c", command])

    def wait_session(self, neighbor, timeout=None):
        """Poll until BGP session with neighbor reaches Established."""
        if timeout is None:
            timeout = SESSION_TIMEOUT
        log_info(
            "waiting for FRR session with %s (timeout %ds)..." % (neighbor, timeout)
        )
        deadline = time.time() + timeout
        while time.time() < deadline:
            # fail-open-ok: poll loop, "" never matches the sentinel and the deadline below raises
            output = self._vtysh_quiet("show bgp neighbor %s" % neighbor)
            if "BGP state = Established" in output:
                log_pass("FRR session with %s is Established" % neighbor)
                return
            time.sleep(2)
        log_fail(
            "FRR session with %s did not reach Established within %ds"
            % (neighbor, timeout)
        )
        # fail-open-ok: diagnostic dump, the AssertionError below is unconditional
        output = self._vtysh_quiet("show bgp neighbor %s" % neighbor)
        for line in output.splitlines()[:10]:
            print("  %s" % line)
        print(docker_logs(ZE_CONTAINER, 20))
        raise AssertionError("FRR session with %s not Established" % neighbor)

    def route(self, prefix, family="ipv4 unicast"):
        """Get route info as parsed JSON from vtysh. Raises when vtysh failed.

        `route_absent` and `wait_route_absent` assert the ABSENCE of a route
        through `has_route`, so a failed read answering {} would prove the
        withdrawal that the scenario is testing for. `_vtysh` raises instead.
        """
        output = self._vtysh("show bgp %s %s json" % (family, prefix))
        if not output.strip():
            return {}
        try:
            return json.loads(output)
        except json.JSONDecodeError:
            log_debug("JSON parse failed for route %s: %s" % (prefix, output[:200]))
            return {}

    def has_route(self, prefix, family="ipv4 unicast"):
        """Check if prefix exists in BGP table via JSON query.

        Handles FRR's nested JSON for VPN/EVPN families where routes
        are wrapped under the Route Distinguisher key (e.g., "65001:100").
        """
        data = self.route(prefix, family)
        if not data:
            return False
        if "paths" in data or "prefix" in data:
            return True
        # VPN/EVPN: routes nested under RD key (e.g., {"65001:100": {"prefix":...}})
        for v in data.values():
            if isinstance(v, dict) and ("paths" in v or "prefix" in v):
                return True
        return False

    def route_absent(self, prefix, family="ipv4 unicast"):
        """Assert prefix is NOT in BGP table."""
        if self.has_route(prefix, family):
            log_fail("FRR still has route %s (expected absent)" % prefix)
            raise AssertionError("FRR still has route %s" % prefix)
        log_pass("FRR does not have route %s (as expected)" % prefix)

    def wait_route(self, prefix, timeout=30, family="ipv4 unicast"):
        """Poll until route appears."""
        deadline = time.time() + timeout
        while time.time() < deadline:
            if self.has_route(prefix, family):
                return
            time.sleep(2)
        log_fail("FRR route %s did not appear within %ds" % (prefix, timeout))
        raise AssertionError("FRR route %s not found" % prefix)

    def wait_route_absent(self, prefix, timeout=30, family="ipv4 unicast"):
        """Poll until route disappears."""
        deadline = time.time() + timeout
        while time.time() < deadline:
            if not self.has_route(prefix, family):
                return
            time.sleep(2)
        log_fail("FRR route %s still present after %ds" % (prefix, timeout))
        raise AssertionError("FRR route %s still present" % prefix)

    def route_count(self, neighbor):
        """Get prefix count from JSON summary for a specific neighbor.

        Raises when vtysh could not be run: 0 is a legitimate prefix count and a
        failed query is not, which is `Ze.rib_count`'s rule applied to the peer
        side. The text fallback below is the site the docker-exec gate was
        written for -- `"".splitlines()` yields no lines, so the loop found no
        neighbor and the helper answered 0 for a vtysh that never ran.
        """
        output = self._vtysh("show bgp ipv4 unicast summary json")
        if not output.strip():
            return 0
        try:
            data = json.loads(output)
            peers = data.get("peers", data.get("ipv4Unicast", {}).get("peers", {}))
            peer = peers.get(neighbor, {})
            return peer.get("pfxSnt", peer.get("pfxRcd", 0))
        except (json.JSONDecodeError, AttributeError):
            log_debug("JSON summary parse failed, falling back to text")
            output = self._vtysh("show bgp ipv4 unicast summary")
            for line in output.splitlines():
                if neighbor in line:
                    parts = line.split()
                    if len(parts) >= 2:
                        try:
                            return int(parts[-2])
                        except ValueError:
                            pass
            return 0

    def check_route(self, prefix, family="ipv4 unicast"):
        """Assert route exists."""
        if self.has_route(prefix, family):
            log_pass("FRR has route %s" % prefix)
            return
        log_fail("FRR does not have route %s" % prefix)
        raise AssertionError("FRR missing route %s" % prefix)

    def check_route_community(self, prefix, community):
        """Assert route has community string (standard, extended, or large)."""
        # fail-open-ok: "" matches no community, so a failed read raises below, never passes
        output = self._vtysh_quiet("show bgp ipv4 unicast %s" % prefix)
        # FRR displays large communities with commas in parens: (65001,0,1)
        comma_form = community.replace(":", ",")
        if community in output or ("(%s)" % comma_form) in output:
            log_pass("FRR route %s has community %s" % (prefix, community))
            return
        log_fail("FRR route %s missing community %s" % (prefix, community))
        raise AssertionError("FRR route %s missing community %s" % (prefix, community))

    def check_route_no_as(self, prefix, asn):
        """Assert AS is NOT in the route's AS_PATH. Uses JSON for reliable parsing."""
        data = self.route(prefix)
        if not data:
            log_fail("FRR has no data for route %s (cannot verify AS_PATH)" % prefix)
            raise AssertionError("no route data for %s" % prefix)
        paths = data.get("paths", [])
        if not paths:
            log_fail("FRR route %s has no paths (cannot verify AS_PATH)" % prefix)
            raise AssertionError("no paths for %s" % prefix)
        asn_str = str(asn)
        for path in paths:
            aspath = path.get("aspath", {})
            if isinstance(aspath, dict):
                aspath_str = aspath.get("string", "")
                if asn_str in aspath_str.split():
                    log_fail("FRR route %s AS_PATH contains AS %s" % (prefix, asn))
                    raise AssertionError(
                        "AS %s found in AS_PATH for %s" % (asn, prefix)
                    )
            elif isinstance(aspath, str):
                if asn_str in aspath.split():
                    log_fail("FRR route %s AS_PATH contains AS %s" % (prefix, asn))
                    raise AssertionError(
                        "AS %s found in AS_PATH for %s" % (asn, prefix)
                    )
        log_pass("FRR route %s AS_PATH does not contain AS %s" % (prefix, asn))

    def session_established(self, neighbor):
        """Check if session is currently Established. Raises when vtysh failed.

        Three scenarios read this NEGATIVELY to prove a session went down:
        `45-max-prefix-cease-frr` waits for the cease teardown, `33-bfd-frr`
        times the BFD-driven failover, and `46-max-prefix-per-family-frr` holds
        the session up. A "" from a failed read is not Established, so the first
        two would call the teardown proven the moment the read broke, and the
        second would report a drop that never happened.
        """
        output = self._vtysh("show bgp neighbor %s" % neighbor)
        return "BGP state = Established" in output

    def wait_bfd_up(self, peer, timeout=None):
        """Poll until FRR's bfdd reports the session to peer is Up.

        Used by the spec-bfd-3b-frr-interop scenario to verify that
        ze's engine + FRR's bfdd completed the three-way BFD
        handshake. `peer` is the neighbor address ze is running on.
        """
        if timeout is None:
            timeout = SESSION_TIMEOUT
        log_info(
            "waiting for FRR BFD session with %s (timeout %ds)..." % (peer, timeout)
        )
        deadline = time.time() + timeout
        while time.time() < deadline:
            # fail-open-ok: poll loop, "" never matches the sentinel and the deadline below raises
            output = self._vtysh_quiet("show bfd peers")
            if peer in output and "Status: up" in output:
                log_pass("FRR BFD peer %s is Up" % peer)
                return
            time.sleep(2)
        log_fail("FRR BFD peer %s did not reach Up within %ds" % (peer, timeout))
        # fail-open-ok: diagnostic dump, the AssertionError below is unconditional
        output = self._vtysh_quiet("show bfd peers")
        for line in output.splitlines()[:20]:
            print("  %s" % line)
        print(docker_logs(ZE_CONTAINER, 20))
        raise AssertionError("FRR BFD peer %s not Up" % peer)

    def break_link(self, port=3784):
        """Drop outbound BFD control packets to induce a link failure.

        Used by spec-bfd-3b-frr-interop to measure sub-2s failover.
        Requires NET_ADMIN cap on the container (scenario runner sets it).
        """
        docker_exec(
            self.container,
            [
                "iptables",
                "-I",
                "OUTPUT",
                "1",
                "-p",
                "udp",
                "--dport",
                str(port),
                "-j",
                "DROP",
            ],
        )

    def restore_link(self, port=3784):
        """Remove the iptables drop rule installed by break_link."""
        docker_exec_quiet(
            self.container,
            [
                "iptables",
                "-D",
                "OUTPUT",
                "-p",
                "udp",
                "--dport",
                str(port),
                "-j",
                "DROP",
            ],
        )


# --- FRR IS-IS helpers -------------------------------------------------------


class FRRISIS:
    """Helpers for querying FRR isisd via vtysh (spec-isis-13 interop).

    The IS-IS scenarios run Ze and FRR on the shared Docker bridge (eth0), which
    is a single L2 broadcast domain, so IS-IS PDUs (IIH/LSP/CSNP/PSNP) reach FRR
    over that link without a separate veth. This class drives the FRR isisd CLI
    to wait for the adjacency to Up and to inspect the IS-IS-learned routes and
    the BGP routes redistributed from IS-IS.
    """

    def __init__(self, container=None, ip=None):
        self.container = container or FRR_CONTAINER
        self.ip = ip or FRR_IP

    def _vtysh_quiet(self, command):
        """Run a vtysh command, return stdout or empty string on failure."""
        return docker_exec_quiet(self.container, ["vtysh", "-c", command])

    def _vtysh(self, command):
        """Run a vtysh command, return stdout, raise when it could not be run.

        `FRR._vtysh` carries the reasoning; every predicate below uses this one
        so that a vtysh fault cannot be reported as an answer about IS-IS.
        """
        return docker_exec(self.container, ["vtysh", "-c", command])

    def adjacency_up(self):
        """Report whether FRR has at least one IS-IS adjacency in the Up state.

        Raises when vtysh failed: `isis-convergence-frr` reads this negatively
        to prove the hold timer expired after a link cut, and "" holds no Up
        line, so a failed read would prove the reconvergence instantly.
        """
        out = self._vtysh("show isis neighbor")
        # FRR prints a per-neighbor table; an Up adjacency shows state "Up".
        for line in out.splitlines():
            if "Up" in line.split():
                return True
        return False

    def wait_adjacency(self, timeout=None):
        """Poll until FRR reports an IS-IS adjacency Up; raise on timeout."""
        if timeout is None:
            timeout = SESSION_TIMEOUT
        log_info("waiting for FRR IS-IS adjacency Up (timeout %ds)..." % timeout)
        deadline = time.time() + timeout
        while time.time() < deadline:
            if self.adjacency_up():
                log_pass("FRR IS-IS adjacency is Up")
                return
            time.sleep(2)
        log_fail("FRR IS-IS adjacency did not reach Up within %ds" % timeout)
        # fail-open-ok: diagnostic print, the AssertionError below is unconditional
        print(self._vtysh_quiet("show isis neighbor")[:500])
        print(docker_logs(ZE_CONTAINER, 30))
        raise AssertionError("FRR IS-IS adjacency not Up")

    def is_dis(self):
        """Report whether FRR shows a Designated IS elected on any circuit (LAN)."""
        out = self._vtysh("show isis interface detail")
        return "DIS" in out or "Designated" in out

    def has_isis_route(self, prefix):
        """Check FRR's kernel/zebra RIB for an IS-IS-learned IPv4 route."""
        out = self._vtysh("show ip route isis")
        return prefix in out

    def has_isis_route_v6(self, prefix):
        """Check FRR's kernel/zebra RIB for an IS-IS-learned IPv6 route."""
        out = self._vtysh("show ipv6 route isis")
        return prefix in out

    def has_database_lsp(self, fragment):
        """Check FRR's IS-IS LSDB for an LSP whose ID contains fragment."""
        out = self._vtysh("show isis database")
        return fragment in out

    def has_pseudonode_lsp(self):
        """Report whether FRR's LSDB carries a pseudo-node LSP (LAN DIS, AC-14).

        A pseudo-node LSP ID has a non-zero pseudo-node octet, e.g. the
        ".XX-00" segment is non-zero (FRR renders e.g. hostname.01-00).
        """
        out = self._vtysh("show isis database")
        for line in out.splitlines():
            # LSP IDs look like "<sysid-or-host>.<pn>-<frag>"; a pseudo-node has
            # a non-zero <pn> field.
            for tok in line.split():
                if "." in tok and "-" in tok:
                    tail = tok.rsplit(".", 1)[-1]
                    parts = tail.split("-")
                    if (
                        len(parts) == 2
                        and parts[0].isalnum()
                        and parts[0] not in ("00", "0")
                    ):
                        return True
        return False

    def wait_isis_route(self, prefix, timeout=60, family="ipv4"):
        """Poll until an IS-IS route for prefix appears in FRR's RIB."""
        deadline = time.time() + timeout
        check = self.has_isis_route_v6 if family == "ipv6" else self.has_isis_route
        while time.time() < deadline:
            if check(prefix):
                return
            time.sleep(2)
        log_fail(
            "FRR IS-IS %s route %s not present after %ds" % (family, prefix, timeout)
        )
        # fail-open-ok: diagnostic print, the AssertionError below is unconditional
        print(self._vtysh_quiet("show ip route isis")[:500])
        print(docker_logs(ZE_CONTAINER, 30))
        raise AssertionError("FRR missing IS-IS %s route %s" % (family, prefix))

    def has_bgp_route_from_isis(self):
        """Report whether FRR's BGP table carries a route learned from a peer.

        Used by the redistribution scenario: an IS-IS route Ze redistributed to
        BGP must land in FRR's BGP RIB. We look for any non-local BGP path.
        """
        out = self._vtysh("show bgp ipv4 unicast")
        # A received BGP route shows a next-hop that is the Ze peer address.
        return ZE_IP in out

    def isis_summary(self):
        """Return the `show isis summary` text (diagnostics on failure)."""
        return self._vtysh_quiet("show isis summary")


# --- FRR OSPF helpers --------------------------------------------------------


class FRROSPF:
    """Helpers for querying FRR ospfd via vtysh (spec-ospf-13 interop).

    The OSPF scenarios run Ze and FRR on the shared Docker bridge (eth0), a single
    L2 broadcast domain, so OSPFv2 packets (Hello/DD/LSR/LSU/LSAck over IP proto
    89) reach FRR over that link. This class drives the FRR ospfd CLI to wait for
    the adjacency to Full and to inspect the OSPF-learned routes and the LSDB.
    """

    def __init__(self, container=None, ip=None):
        self.container = container or FRR_CONTAINER
        self.ip = ip or FRR_IP

    def _vtysh_quiet(self, command):
        """Run a vtysh command, return stdout or empty string on failure."""
        return docker_exec_quiet(self.container, ["vtysh", "-c", command])

    def _vtysh(self, command):
        """Run a vtysh command, return stdout, raise when it could not be run.

        `FRR._vtysh` carries the reasoning; every predicate below uses this one
        so that a vtysh fault cannot be reported as an answer about OSPF.
        """
        return docker_exec(self.container, ["vtysh", "-c", command])

    def adjacency_full(self):
        """Report whether FRR has at least one OSPF neighbor in the Full state.

        Raises when vtysh failed. `adjacency_down` below is its negation, and
        `ospf-bfd-frr` and `ospf-convergence-frr` both read that to prove the
        adjacency dropped after a link cut. "" holds no "Full", so a failed read
        would report a drop in 0s and pass the BFD detection budget with it.
        """
        out = self._vtysh("show ip ospf neighbor")
        # FRR prints a per-neighbor table; a full adjacency shows "Full/DR",
        # "Full/BDR", "Full/DROther", or "Full/-" (point-to-point).
        return "Full" in out

    def wait_adjacency(self, timeout=None):
        """Poll until FRR reports an OSPF adjacency Full; raise on timeout."""
        if timeout is None:
            timeout = SESSION_TIMEOUT
        log_info("waiting for FRR OSPF adjacency Full (timeout %ds)..." % timeout)
        deadline = time.time() + timeout
        while time.time() < deadline:
            if self.adjacency_full():
                log_pass("FRR OSPF adjacency is Full")
                return
            time.sleep(2)
        log_fail("FRR OSPF adjacency did not reach Full within %ds" % timeout)
        # fail-open-ok: diagnostic print, the AssertionError below is unconditional
        print(self._vtysh_quiet("show ip ospf neighbor")[:500])
        print(docker_logs(ZE_CONTAINER, 30))
        raise AssertionError("FRR OSPF adjacency not Full")

    def adjacency_down(self):
        """Report whether FRR has NO OSPF neighbor in the Full state (convergence)."""
        return not self.adjacency_full()

    def has_dr_bdr(self):
        """Report whether FRR elected a DR/BDR on a broadcast segment (AC-16)."""
        out = self._vtysh("show ip ospf neighbor")
        return "Full/DR" in out or "Full/BDR" in out or "DROther" in out

    def has_network_lsa(self):
        """Report whether FRR's LSDB carries a Network-LSA (Type 2, broadcast DR)."""
        out = self._vtysh("show ip ospf database network")
        return "Net Link States" in out or "Network Link States" in out

    def has_summary_lsa(self, prefix=None):
        """Report whether FRR's LSDB carries a Summary-LSA (Type 3, inter-area)."""
        out = self._vtysh("show ip ospf database summary")
        if prefix:
            return prefix in out
        return "Summary Link States" in out

    def has_external_lsa(self, prefix=None):
        """Report whether FRR's LSDB carries an actual AS-external-LSA (Type 5).

        `show ip ospf database external` always prints the "AS External Link States"
        section header even when empty, so match a real LSA entry ("LS age:") -- or the
        given prefix -- not the header.

        Raises when vtysh failed: `ospf-stub-nssa-frr` proves a stub area carries
        no Type-5 LSA by asserting this is False, and "" would prove it.
        """
        out = self._vtysh("show ip ospf database external")
        if prefix:
            return prefix in out
        return "LS age" in out

    def has_ospf_route(self, prefix):
        """Check FRR's kernel/zebra RIB for an OSPF-learned IPv4 route.

        Raises when vtysh failed: `wait_route_withdrawn` below polls for this to
        become False, and `show ip route ospf` prints nothing when the RIB holds
        no OSPF route, so only the exit status separates the withdrawal from a
        vtysh that never ran.
        """
        out = self._vtysh("show ip route ospf")
        return prefix in out

    def wait_ospf_route(self, prefix, timeout=60):
        """Poll until an OSPF route for prefix appears in FRR's RIB; raise on timeout."""
        deadline = time.time() + timeout
        while time.time() < deadline:
            if self.has_ospf_route(prefix):
                return
            time.sleep(2)
        log_fail("FRR OSPF route %s not present after %ds" % (prefix, timeout))
        # fail-open-ok: diagnostic print, the AssertionError below is unconditional
        print(self._vtysh_quiet("show ip route ospf")[:500])
        print(docker_logs(ZE_CONTAINER, 30))
        raise AssertionError("FRR missing OSPF route %s" % prefix)

    def wait_route_withdrawn(self, prefix, timeout=60):
        """Poll until an OSPF route for prefix DISAPPEARS from FRR's RIB (AC-20)."""
        deadline = time.time() + timeout
        while time.time() < deadline:
            if not self.has_ospf_route(prefix):
                return
            time.sleep(2)
        log_fail("FRR OSPF route %s still present after %ds" % (prefix, timeout))
        # fail-open-ok: diagnostic print, the AssertionError below is unconditional
        print(self._vtysh_quiet("show ip route ospf")[:500])
        raise AssertionError("FRR did not withdraw OSPF route %s" % prefix)

    def ospf_summary(self):
        """Return the `show ip ospf` text (diagnostics on failure)."""
        return self._vtysh_quiet("show ip ospf")


class FRROSPF6:
    """Helpers for querying FRR ospf6d via vtysh (spec-ospf-af-unify v6 interop).

    The OSPFv3 scenario runs Ze and FRR on the shared Docker bridge (eth0).
    OSPFv3 (RFC 5340) runs directly over IPv6 link-local (ff02::5/6, IP proto
    89), so the adjacency needs IPv6 enabled on eth0 -- the scenario setup sets
    the disable_ipv6=0 sysctl on the containers. This class drives FRR ospf6d to
    wait for the v6 adjacency to Full and to inspect the OSPFv3 LSDB and routes.
    """

    def __init__(self, container=None, ip=None):
        self.container = container or FRR_CONTAINER
        self.ip = ip or FRR_IP

    def _vtysh_quiet(self, command):
        """Run a vtysh command, return stdout or empty string on failure."""
        return docker_exec_quiet(self.container, ["vtysh", "-c", command])

    def _vtysh(self, command):
        """Run a vtysh command, return stdout, raise when it could not be run.

        `FRR._vtysh` carries the reasoning; every predicate below uses this one
        so that a vtysh fault cannot be reported as an answer about OSPFv3.
        """
        return docker_exec(self.container, ["vtysh", "-c", command])

    def adjacency_full(self):
        """Report whether FRR has at least one OSPFv3 neighbor in the Full state.

        Raises when vtysh failed: `ospfv3-bfd-frr` reads this negatively to time
        the BFD-driven drop after a link cut, and "" holds no "Full", so a failed
        read would report the drop instantly and pass the detection budget.
        """
        out = self._vtysh("show ipv6 ospf6 neighbor")
        # ospf6d prints a per-neighbor table; a full adjacency shows the neighbor
        # state column as "Full" (e.g. "Full/PointToPoint" or "Full/DR").
        return "Full" in out

    def wait_adjacency(self, timeout=None):
        """Poll until FRR reports an OSPFv3 adjacency Full; raise on timeout."""
        if timeout is None:
            timeout = SESSION_TIMEOUT
        log_info("waiting for FRR OSPFv3 adjacency Full (timeout %ds)..." % timeout)
        deadline = time.time() + timeout
        while time.time() < deadline:
            if self.adjacency_full():
                log_pass("FRR OSPFv3 adjacency is Full")
                return
            time.sleep(2)
        log_fail("FRR OSPFv3 adjacency did not reach Full within %ds" % timeout)
        # fail-open-ok: diagnostic print, the AssertionError below is unconditional
        print(self._vtysh_quiet("show ipv6 ospf6 neighbor")[:500])
        print(docker_logs(ZE_CONTAINER, 30))
        raise AssertionError("FRR OSPFv3 adjacency not Full")

    def _database_has_adv_router(self, command, lsa_types, adv_router):
        out = self._vtysh(command)
        for line in out.splitlines():
            fields = line.split()
            if len(fields) >= 3 and fields[0] in lsa_types and fields[2] == adv_router:
                return True
        return False

    def has_router_lsa(self, router_id):
        """Report whether FRR's OSPFv3 LSDB carries a Router-LSA from router_id.

        Proves the DD exchange + LSDB synchronisation completed (the Full
        milestone), independent of any IPv6 prefix being installed as a route.
        """
        return self._database_has_adv_router(
            "show ipv6 ospf6 database router", ("Rtr", "Router"), router_id
        )

    def has_dr_bdr(self):
        """Report whether FRR sees a DR or BDR role on a v6 broadcast segment.

        OSPFv3 ospf6d prints the neighbor's role in the State/IfState column as
        "Full/DR" or "Full/BDR" (vs "Full/PointToPoint" on a p2p link).
        """
        out = self._vtysh("show ipv6 ospf6 neighbor")
        return "/DR" in out or "/BDR" in out or "/Backup" in out

    def has_network_lsa(self, adv_router=None):
        """Report whether FRR's OSPFv3 LSDB carries a Network-LSA (Type 0x2002).

        With adv_router set, requires the Network-LSA to be advertised by that router
        (the elected DR), proving the DR originated it for the segment.
        """
        if adv_router:
            return self._database_has_adv_router(
                "show ipv6 ospf6 database network", ("Net", "Network"), adv_router
            )
        out = self._vtysh("show ipv6 ospf6 database network")
        return any(
            line.split() and line.split()[0] in ("Net", "Network")
            for line in out.splitlines()
        )

    def has_link_lsa(self, adv_router=None):
        """Report whether FRR's OSPFv3 LSDB carries a Link-LSA (Type 0x0008, link-local).

        With adv_router set, requires a Link-LSA advertised by that router -- proving the
        peer originated it and flooded it on the shared link (link-local scope). The
        advertising-router id only appears in the AdvRouter column of `database link`, so a
        whole-token match is robust across FRR's per-version type-abbreviation differences.
        """
        out = self._vtysh("show ipv6 ospf6 database link")
        if adv_router:
            return any(adv_router in line.split() for line in out.splitlines())
        return any(
            len(fields) >= 3 and fields[0] in ("Lnk", "Link")
            for fields in (line.split() for line in out.splitlines())
        )

    def link_lsa_dump(self):
        """Return `show ipv6 ospf6 database link` text (diagnostics on assertion failure)."""
        return self._vtysh_quiet("show ipv6 ospf6 database link")

    def has_inter_area_prefix_lsa(self, adv_router=None):
        """Report whether FRR's OSPFv3 LSDB carries an Inter-Area-Prefix-LSA (Type 0x2003).

        With adv_router set, requires one advertised by that router (the ABR), proving the ABR
        summarised another area's prefixes into this one. The advertising-router id appears as a
        whole token, robust across FRR's per-version type abbreviations.
        """
        out = self._vtysh("show ipv6 ospf6 database inter-prefix")
        if adv_router:
            return any(adv_router in line.split() for line in out.splitlines())
        return any(
            fields and fields[0] in ("INP", "IAP", "Inter-Prefix")
            for fields in (line.split() for line in out.splitlines())
        )

    def inter_area_prefix_dump(self):
        """Return `show ipv6 ospf6 database inter-prefix` text (diagnostics on failure)."""
        return self._vtysh_quiet("show ipv6 ospf6 database inter-prefix")

    def has_as_external_lsa(self):
        """Report whether FRR's OSPFv3 LSDB carries any AS-External-LSA (Type 0x4005).

        Used to prove a v6 stub area is clean: a stub must never receive Type-5 LSAs. The
        whole-token type match (against FRR's per-version abbreviations) never matches the
        database header lines, so an empty as-external LSDB reports False.

        That is why it raises when vtysh failed: `ospf-v6-stub-frr` asserts this
        is False, and a failed read reports False for a reason the stub area had
        nothing to do with.
        """
        out = self._vtysh("show ipv6 ospf6 database as-external")
        return any(
            fields
            and fields[0] in ("ASE", "Type-5", "AS-External", "External", "Extern")
            for fields in (line.split() for line in out.splitlines())
        )

    def has_ospf6_route(self, prefix):
        """Check FRR's kernel/zebra RIB for an OSPFv3-learned IPv6 route."""
        out = self._vtysh("show ipv6 route ospf6")
        return prefix in out

    def wait_ospf6_route(self, prefix, timeout=60):
        """Poll until an OSPFv3 route for prefix appears in FRR's RIB; raise on timeout."""
        deadline = time.time() + timeout
        while time.time() < deadline:
            if self.has_ospf6_route(prefix):
                return
            time.sleep(2)
        log_fail("FRR OSPFv3 route %s not present after %ds" % (prefix, timeout))
        # fail-open-ok: diagnostic print, the AssertionError below is unconditional
        print(self._vtysh_quiet("show ipv6 route ospf6")[:500])
        print(docker_logs(ZE_CONTAINER, 30))
        raise AssertionError("FRR missing OSPFv3 route %s" % prefix)

    def ospf6_summary(self):
        """Return the `show ipv6 ospf6` text (diagnostics on failure)."""
        return self._vtysh_quiet("show ipv6 ospf6")


# --- BIRD helpers ------------------------------------------------------------


class BIRD:
    """Helpers for querying BIRD via birdc."""

    def __init__(self, container=None, ip=None):
        self.container = container or BIRD_CONTAINER
        self.ip = ip or BIRD_IP

    def _birdc_quiet(self, command):
        """Run a birdc command, return stdout or empty string on failure."""
        return docker_exec_quiet(self.container, ["birdc", command])

    def _birdc(self, command):
        """Run a birdc command, return stdout, raise when it could not be run.

        `FRR._vtysh` carries the reasoning. birdc answers a valid command with
        exit 0 and its own banner even when the table is empty, so a non-zero
        exit means birdc could not be run at all, and an empty answer means the
        route is absent. Only the exit status separates the two.
        """
        return docker_exec(self.container, ["birdc", command])

    def wait_session(self, protocol, timeout=None):
        """Poll until protocol reaches Established."""
        if timeout is None:
            timeout = SESSION_TIMEOUT
        log_info("waiting for BIRD protocol %s (timeout %ds)..." % (protocol, timeout))
        deadline = time.time() + timeout
        while time.time() < deadline:
            # fail-open-ok: poll loop, "" never matches the sentinel and the deadline below raises
            output = self._birdc_quiet("show protocols")
            for line in output.splitlines():
                if protocol in line and "Established" in line:
                    log_pass("BIRD protocol %s is Established" % protocol)
                    return
            time.sleep(2)
        log_fail(
            "BIRD protocol %s did not reach Established within %ds"
            % (protocol, timeout)
        )
        # fail-open-ok: diagnostic dump, the AssertionError below is unconditional
        output = self._birdc_quiet("show protocols all")
        print(output)
        print(docker_logs(ZE_CONTAINER, 20))
        raise AssertionError("BIRD protocol %s not Established" % protocol)

    def has_route(self, prefix):
        """Check if prefix in routing table. Raises when birdc failed.

        `39-policy-import-export-frr` asserts this is False to prove the export
        prefix-list denied a route, so a failed read would prove the deny.
        """
        output = self._birdc("show route for %s" % prefix)
        return prefix in output

    def wait_route(self, prefix, timeout=30):
        """Poll until route appears."""
        deadline = time.time() + timeout
        while time.time() < deadline:
            if self.has_route(prefix):
                return
            time.sleep(2)
        log_fail("BIRD route %s did not appear within %ds" % (prefix, timeout))
        raise AssertionError("BIRD route %s not found" % prefix)

    def check_route(self, prefix):
        """Assert route exists."""
        if self.has_route(prefix):
            log_pass("BIRD has route %s" % prefix)
            return
        log_fail("BIRD does not have route %s" % prefix)
        raise AssertionError("BIRD missing route %s" % prefix)

    def check_route_no_as(self, prefix, asn):
        """Assert AS not in route's AS_PATH."""
        output = self._birdc("show route for %s all" % prefix)
        found_aspath = False
        for line in output.splitlines():
            if "BGP.as_path" in line:
                found_aspath = True
                if str(asn) in line.split():
                    log_fail("BIRD route %s AS_PATH contains AS %s" % (prefix, asn))
                    raise AssertionError(
                        "AS %s found in AS_PATH for %s" % (asn, prefix)
                    )
        if not found_aspath:
            log_fail("BIRD route %s has no AS_PATH line (cannot verify)" % prefix)
            raise AssertionError("no AS_PATH found for %s" % prefix)
        log_pass("BIRD route %s AS_PATH does not contain AS %s" % (prefix, asn))

    def exported_count(self, protocol):
        """Get exported route count from protocol details. Raises when birdc failed.

        0 is a legitimate export count and a failed query is not, which is
        `Ze.rib_count`'s rule applied to BIRD.
        """
        output = self._birdc("show protocols all %s" % protocol)
        for line in output.splitlines():
            if "Routes:" in line:
                m = re.search(r"(\d+)\s+exported", line)
                if m:
                    return int(m.group(1))
        return 0

    def session_established(self, protocol):
        """Check if protocol is Established. Raises when birdc failed."""
        output = self._birdc("show protocols")
        for line in output.splitlines():
            if protocol in line and "Established" in line:
                return True
        return False


# --- GoBGP helpers -----------------------------------------------------------


class GoBGP:
    """Helpers for querying GoBGP via gobgp CLI."""

    def __init__(self, container=None, ip=None):
        self.container = container or GOBGP_CONTAINER
        self.ip = ip or GOBGP_IP

    def _gobgp_quiet(self, args):
        """Run a gobgp command, return stdout or empty string on failure."""
        return docker_exec_quiet(self.container, ["gobgp"] + args)

    def _gobgp_json(self, args):
        """Run a gobgp command with -j (JSON), return parsed data or None."""
        output = self._gobgp_quiet(args + ["-j"])
        if not output.strip():
            return None
        try:
            return json.loads(output)
        except json.JSONDecodeError:
            return None

    def wait_session(self, neighbor, timeout=None):
        """Poll until BGP session with neighbor reaches Established."""
        if timeout is None:
            timeout = SESSION_TIMEOUT
        log_info(
            "waiting for GoBGP session with %s (timeout %ds)..." % (neighbor, timeout)
        )
        deadline = time.time() + timeout
        while time.time() < deadline:
            # fail-open-ok: poll loop, "" never matches the sentinel and the deadline below raises
            output = self._gobgp_quiet(["neighbor", neighbor])
            if "established" in output.lower():
                log_pass("GoBGP session with %s is Established" % neighbor)
                return
            time.sleep(2)
        log_fail(
            "GoBGP session with %s did not reach Established within %ds"
            % (neighbor, timeout)
        )
        # fail-open-ok: diagnostic dump, the AssertionError below is unconditional
        output = self._gobgp_quiet(["neighbor"])
        print("  %s" % output[:500])
        print(docker_logs(ZE_CONTAINER, 20))
        raise AssertionError("GoBGP session with %s not Established" % neighbor)

    def route_json(self, prefix, family="ipv4 unicast"):
        """Get route info as parsed JSON from gobgp. Returns list or None."""
        afi = family.split("/")[0] if "/" in family else family.split()[0]
        return self._gobgp_json(["global", "rib", "-a", afi, prefix])

    def has_route(self, prefix, family="ipv4 unicast"):
        """Check if prefix exists in GoBGP's RIB via JSON."""
        data = self.route_json(prefix, family)
        if data is None:
            return False
        if isinstance(data, list):
            for dest in data:
                paths = dest.get("paths", [])
                if paths:
                    return True
            return False
        return bool(data)

    def check_route(self, prefix, family="ipv4 unicast"):
        """Assert route exists in GoBGP's RIB."""
        if self.has_route(prefix, family):
            log_pass("GoBGP has route %s" % prefix)
            return
        log_fail("GoBGP does not have route %s" % prefix)
        raise AssertionError("GoBGP missing route %s" % prefix)

    def wait_route(self, prefix, timeout=30, family="ipv4 unicast"):
        """Poll until route appears."""
        deadline = time.time() + timeout
        while time.time() < deadline:
            if self.has_route(prefix, family):
                return
            time.sleep(2)
        log_fail("GoBGP route %s did not appear within %ds" % (prefix, timeout))
        raise AssertionError("GoBGP route %s not found" % prefix)

    def session_established(self, neighbor):
        """Check if session is currently Established."""
        output = self._gobgp_quiet(["neighbor", neighbor])
        return "established" in output.lower()

    def inject_route(self, prefix, nexthop=None):
        """Inject a route into GoBGP's global RIB. Raises on failure."""
        if nexthop is None:
            nexthop = GOBGP_IP
        docker_exec(
            self.container,
            [
                "gobgp",
                "global",
                "rib",
                "add",
                prefix,
                "-a",
                "ipv4",
                "nexthop",
                nexthop,
            ],
        )


# --- Ze helpers --------------------------------------------------------------


class Ze:
    """Helpers for querying Ze."""

    def __init__(self, container=ZE_CONTAINER):
        self.container = container

    def cli(self, command):
        """Run one CLI command against the daemon and return its JSON output.

        THE ONLY WAY A SCENARIO QUERIES ZE. `ze cli -c` is the form that carries
        credentials: --user and --format are its own flags, while the verb form
        (`ze show ospf te-database`) has no slot for either. The SSH listener and
        the account this authenticates against are appended to every rendered
        ze.conf (ZE_CLI_CONFIG), and nothing writes a zefs identity into the
        container, so the verb form resolves no username at all and dies with
        "no credentials for 127.0.0.1:2222" (readCredentials,
        internal/core/ssh/client/client.go).

        This RAISES on every failure, through docker_exec. `ze cli -c` exits 1
        for an auth refusal, an unreachable daemon and a command the daemon
        rejects alike, so one raise covers all three. Nineteen scenario sites ran
        the verb form through docker_exec_quiet until 2026-08-08, which answers
        "" on failure: the query never ran, the check read an empty result, and
        the assertion that followed blamed the protocol implementation for a
        harness fault. One (ospf-opaque-frr) treated the empty answer as
        inconclusive and PASSED.
        """
        return docker_exec(
            self.container,
            ["ze", "cli", "-c", command, "--user", ZE_CLI_USER, "--format", "json"],
            env={"ZE_SSH_PASSWORD": ZE_CLI_PASSWORD},
        )

    def cli_json(self, command, want=None):
        """Run one CLI command and return its parsed JSON. Raises on failure.

        A parse failure is a harness fault, never an empty result: every daemon
        command answers valid JSON under --format json, and an empty collection
        is spelled [] or {}.

        `want` is the container type the caller is about to INDEX, and passing it
        is how a caller that iterates rows says so. The docstring above names
        both shapes, so a caller writing `for row in ...: row.get(...)` over a
        document that decoded to an object gets a str back and raises
        AttributeError from inside its own probe. poll() bounds itself to
        RuntimeError on purpose -- a transport fault is worth retrying and a
        programming error is not, and widening that bound would retry a typo for
        the whole timeout and then return a stale result as if it had been
        measured. So the shape is asserted HERE, where the command name is still
        in hand, and it is raised as the same RuntimeError a transport fault
        raises: an object where a list belongs is a daemon that answered
        something other than the collection, which poll retries and then reports
        as a harness failure rather than a protocol verdict.
        """
        output = self.cli(command)
        try:
            decoded = json.loads(output)
        except ValueError as exc:
            raise RuntimeError(
                "`%s` in container %s did not answer JSON (harness failure, not "
                "an empty result): %s: %r"
                % (command, self.container, exc, output[:400])
            )
        if want is not None and not isinstance(decoded, want):
            raise RuntimeError(
                "`%s` in container %s answered %s, want %s (harness failure, not "
                "an empty result): %r"
                % (
                    command,
                    self.container,
                    type(decoded).__name__,
                    want.__name__,
                    output[:400],
                )
            )
        return decoded

    def rib_count(self):
        """Return the number of received routes in Ze's RIB. Raises on failure.

        The command is `show bgp rib status` (docs/architecture/api/commands.md).

        This RAISES rather than answering 0 when the query fails, because 0 is a
        legitimate RIB size and a failed query is not (ai/rules/evidence.md: a
        zero value must never be a valid-looking answer). It returned 0 on
        failure until 2026-08-07, and three separate defects hid behind that 0:
        the helper asked for `show rib status` when the command is `show bgp rib
        status`; `ze` then resolved no verb at all from a shell argv; and the
        daemon started no SSH listener for any client to reach. Every caller
        read `Ze RIB has 0 received routes`, which named none of them.
        """
        try:
            output = self.cli("show bgp rib status")
        except RuntimeError as exc:
            log_fail("Ze RIB query failed: %s" % exc)
            raise RuntimeError(
                "`show bgp rib status` failed in container %s: %s "
                "(harness failure, not an empty RIB)" % (self.container, exc)
            )
        m = re.search(r'"routes-in"\s*:\s*(\d+)', output)
        if m is None:
            log_fail("Ze RIB query returned no routes-in field")
            raise RuntimeError(
                "`show bgp rib status` in container %s answered without a "
                '"routes-in" field (harness failure, not an empty RIB): %r'
                % (self.container, output[:400])
            )
        return int(m.group(1))

    def rib_received(self, minimum):
        """Assert RIB has >= minimum received routes. Raises on query failure."""
        count = self.rib_count()
        if count >= minimum:
            log_pass(
                "Ze RIB has %d received routes (expected >= %d)" % (count, minimum)
            )
            return
        log_fail("Ze RIB has %d received routes (expected >= %d)" % (count, minimum))
        raise AssertionError("Ze RIB has %d routes, expected >= %d" % (count, minimum))

    def logs(self, lines=30):
        """Get last N lines of container logs."""
        return docker_logs(self.container, lines)


# --- Container health --------------------------------------------------------


def _check_container_running(name):
    """Check if a container is in running state."""
    try:
        result = subprocess.run(
            ["docker", "inspect", name, "--format", "{{.State.Running}}"],
            capture_output=True,
            text=True,
            timeout=10,
        )
        return "true" in result.stdout
    except subprocess.TimeoutExpired:
        return False


def _check_container_responsive(name, cmd):
    """Check if a container responds to a command."""
    try:
        result = subprocess.run(
            ["docker", "exec", name] + cmd,
            capture_output=True,
            text=True,
            timeout=10,
        )
        return result.returncode == 0
    except subprocess.TimeoutExpired:
        return False


def wait_containers_healthy(timeout=30):
    """Wait for all running containers to be responsive."""
    deadline = time.time() + timeout
    while time.time() < deadline:
        all_ready = True

        if not _check_container_running(ZE_CONTAINER):
            all_ready = False

        if _check_container_running(FRR_CONTAINER):
            if not _check_container_responsive(
                FRR_CONTAINER, ["vtysh", "-c", "show version"]
            ):
                all_ready = False

        if _check_container_running(BIRD_CONTAINER):
            if not _check_container_responsive(
                BIRD_CONTAINER, ["birdc", "show status"]
            ):
                all_ready = False

        # keepalived has no client CLI to interrogate (no vtysh/birdc/gobgp
        # equivalent), so "responsive" is read from the kernel state it is
        # responsible for: if the VRRP process is alive it can list links.
        # Readiness here means "the container is up and usable"; whether it
        # reached MASTER or BACKUP is a per-scenario assertion, not a health
        # check, because for most VRRP scenarios BACKUP is the correct
        # steady state and treating it as unhealthy would be exactly wrong.
        if _check_container_running(KEEPALIVED_CONTAINER):
            if not _check_container_responsive(KEEPALIVED_CONTAINER, ["ip", "link"]):
                all_ready = False

        # StayRTR is ready when it SERVES the VRP set, not when its process is
        # up: it binds the RTR port before the cache file is loaded, so a TCP
        # probe would pass against an empty set and every prefix would then read
        # NotFound for a harness reason. Its own /rpki.json export is the set it
        # will hand out over RTR, so a 200 there is the readiness that matters.
        if _check_container_running(STAYRTR_CONTAINER):
            if not _check_container_responsive(
                STAYRTR_CONTAINER,
                [
                    "wget",
                    "-q",
                    "-O",
                    "-",
                    "http://127.0.0.1:%d/rpki.json" % STAYRTR_HTTP_PORT,
                ],
            ):
                all_ready = False

        if all_ready:
            log_debug("all containers healthy")
            return
        time.sleep(1)

    # This site catches ONE case: the plugin already stopped ze by the time the
    # loop gave up, so "containers not healthy" would name the symptom 30
    # seconds after the cause. It is not the general net, and the round 4 review
    # measured why: on `11-addpath-frr` the plugin failed 6s in, this loop had
    # already returned green, and the run still spent 90s reporting the wrong
    # cause. Whether this site fires is a race between the peer daemon becoming
    # responsive and the plugin failing. The site that fires on EVERY scenario
    # failure is `run.py`'s handler, which calls `observer_failure_note` after
    # the scenario raises. The log dump below covers every other way ze can die
    # during setup.
    try:
        raise_if_observer_failed("while waiting for the containers")
    except (RuntimeError, OSError) as exc:
        # An unreadable log must not REPLACE the failure this function exists
        # to report. Say what could not be read, then let the health timeout
        # below raise with its own message (round 5 review).
        print("--- ze log could not be read: %s ---" % exc)
    if not _check_container_running(ZE_CONTAINER):
        print("--- ze log ---")
        print(docker_logs(ZE_CONTAINER, 60))
    log_fail("containers did not become healthy within %ds" % timeout)
    raise RuntimeError("containers not healthy")


# --- Scenario lifecycle ------------------------------------------------------


class Scenario:
    """Manages container lifecycle for a scenario."""

    def __init__(self, scenario_dir, frr_image):
        self.source_dir = scenario_dir
        self.scenario_dir = scenario_dir
        self.rendered_dir = None
        self.frr_image = frr_image
        self.name = os.path.basename(scenario_dir.rstrip("/"))

    def _needs_ipv6_wire(self):
        """Report whether this scenario needs the Docker network to be dual-stack.

        Two scenarios need it, for different reasons.

        A protocol that runs natively over IPv6 (OSPFv3) needs the IPv6 link-local
        on eth0 that docker only enables on an IPv6-capable network. ISIS rides L2
        and a plain BGP-v6 session rides IPv4 TCP, so neither of those needs it.

        A scenario that names an address in the lab's IPv6 range needs the range to
        exist and its containers to sit at known addresses in it. That covers BGP
        carrying an IPv6 next hop that must land on a locally connected subnet (RFC
        2545 Section 3), where the whole point is that the address is real. The
        trigger is the `_BASE_V6_PREFIX` token itself, in any file of the scenario,
        so it is the scenario's own text that asks -- there is no list of names here
        to keep in step with the directory.
        """
        for root, dirs, files in os.walk(self.source_dir):
            dirs[:] = [d for d in dirs if d != "__pycache__"]
            for fname in files:
                try:
                    with open(os.path.join(root, fname), "r", encoding="utf-8") as fh:
                        text = fh.read()
                except (OSError, UnicodeDecodeError):
                    continue
                if _BASE_V6_PREFIX in text:
                    return True
                if (
                    root == self.source_dir
                    and fname == "ze.conf"
                    and "ospf" in text
                    and "address-family ipv6" in text
                ):
                    return True
        return False

    def setup(self):
        """Create network, start containers based on which config files exist."""
        # PRE-CLEAN, not cleanup: a removal that failed here would leave this
        # scenario running beside a leftover daemon, so it must deny.
        self.teardown(strict=True)

        _create_network(dual_stack=self._needs_ipv6_wire())
        self.rendered_dir = _render_scenario_dir(self.source_dir, self.name)
        self.scenario_dir = self.rendered_dir

        ze_conf = os.path.join(self.scenario_dir, "ze.conf")
        if not os.path.isfile(ze_conf):
            raise RuntimeError("missing ze.conf in %s" % self.name)

        # Start a BMP collector sidecar before Ze if the scenario provides one.
        # This avoids racing Ze's BMP sender reconnect backoff against a process
        # plugin that would only start after Ze itself is already running.
        bmp_collector = os.path.join(self.scenario_dir, "bmp-collector.py")
        if os.path.isfile(bmp_collector):
            docker_run(
                BMP_CONTAINER,
                "ze-interop",
                BMP_IP,
                volumes=["%s:/bmp-collector.py:ro" % os.path.abspath(bmp_collector)],
                extra_args=["--entrypoint", "python3"],
                cmd=["/bmp-collector.py"],
            )

        # Start a raw BGP injector sidecar before Ze if the scenario provides one.
        # It runs `ze-test peer` in check mode against a scenario-supplied expect file,
        # which is how a scenario sends wire bytes no conforming daemon would produce --
        # for instance an UPDATE mixing Withdrawn Routes with NLRI, which RFC 7606
        # Section 5.1 forbids a sender to emit but obliges every receiver to accept. Ze
        # dials it (accept false in ze.conf), the same way the plugin .ci suite does.
        inject_expect = os.path.join(self.scenario_dir, "inject.msg")
        if os.path.isfile(inject_expect):
            # An optional inject-args file supplies extra `ze-test peer` flags, the same
            # way rpki-server does below. --asn matters: without it the peer adopts the
            # ASN from Ze's OPEN, silently turning an eBGP scenario into an iBGP one.
            inject_args = []
            inject_args_file = os.path.join(self.scenario_dir, "inject-args")
            if os.path.isfile(inject_args_file):
                with open(inject_args_file, "r", encoding="utf-8") as fh:
                    inject_args = fh.read().split()
            docker_run(
                INJECT_CONTAINER,
                "ze-interop",
                INJECT_IP,
                volumes=["%s:/inject.msg:ro" % os.path.abspath(inject_expect)],
                extra_args=["--entrypoint", "ze-test"],
                cmd=["peer", "--port", "179", "--decode"]
                + inject_args
                + ["/inject.msg"],
            )

        # Start StayRTR if the scenario supplies a VRP file. StayRTR is a real
        # third-party RTR server (Cloudflare, RFC 8210), so Ze is the client on
        # both sides of nothing: `rpki-server` below runs `ze-test rpki`, which
        # is Ze's own encoder answering Ze's own decoder.
        #
        # It starts BEFORE Ze so the first RESET QUERY finds a cache that
        # already holds the set, rather than an empty one it would have to
        # re-poll for after `-refresh`.
        stayrtr_vrps = os.path.join(self.scenario_dir, "vrps.json")
        if os.path.isfile(stayrtr_vrps):
            docker_run(
                STAYRTR_CONTAINER,
                "stayrtr-interop",
                STAYRTR_IP,
                volumes=["%s:/vrps.json:ro" % os.path.abspath(stayrtr_vrps)],
            )

        rpki_server = os.path.join(self.scenario_dir, "rpki-server")
        if os.path.isfile(rpki_server):
            with open(rpki_server, "r", encoding="utf-8") as fh:
                rpki_args = fh.read().split()
            docker_run(
                RPKI_CONTAINER,
                "ze-interop",
                RPKI_IP,
                extra_args=["--entrypoint", "ze-test"],
                cmd=["rpki", "--bind", "0.0.0.0"] + rpki_args,
            )

        # Collect extra volume mounts for Ze (plugin scripts, etc.).
        volumes = ["%s:/etc/ze/bgp.conf:ro" % os.path.abspath(ze_conf)]
        for fname in sorted(os.listdir(self.scenario_dir)):
            if fname in ("check.sh", "check.py"):
                continue
            fpath = os.path.join(self.scenario_dir, fname)
            if not os.path.isfile(fpath):
                continue
            if fname.endswith(".sh") or fname.endswith(".py"):
                volumes.append("%s:/etc/ze/%s:ro" % (os.path.abspath(fpath), fname))

        # Start Ze (always present).
        docker_run(
            ZE_CONTAINER,
            "ze-interop",
            ZE_IP,
            volumes=volumes,
            caps=["NET_ADMIN"],
            extra_args=_IPV6_SYSCTLS,
            # A process plugin reads this to size its own barriers against the
            # harness budget instead of hardcoding 90 (docker_run, `env`).
            env={"SESSION_TIMEOUT": str(SESSION_TIMEOUT)},
            # `start <config>`, not the bare `<config>`. The bare launch form was removed
            # from the CLI (`ze - ` for stdin, `ze start <config>` for a file); the image's
            # ENTRYPOINT is `tini -- ze`, so cmd=["/etc/ze/bgp.conf"] became
            # `ze /etc/ze/bgp.conf` and died with "unknown command: /etc/ze/bgp.conf".
            # EVERY scenario failed at wait_containers_healthy, and nothing noticed because
            # ze-interop-test had no automated caller (learned 1248: a removed launch form
            # hides in call sites a directive-level grep never sees).
            cmd=["start", "/etc/ze/bgp.conf"],
        )

        # Start the minimal Python speaker sidecar if the scenario provides one. It dials Ze
        # AFTER Ze is up (so replay-on-peer-up fires), loads one per-test plugin, and reports a
        # verdict on stdout that check.py reads via docker logs. Unlike `ze-test peer` (which
        # asserts only the bytes it was told to expect), the speaker's plugin applies an
        # INDEPENDENT check -- e.g. RFC 7606 Section 3(g) duplicate attributes -- so it catches
        # wire output Ze's own lenient validator waves through. See test/interop/speaker/ and
        # plan/spec-bgp-plugin-speaker.md.
        # A scenario may run one or two speaker instances. `speaker-args` starts the first,
        # `speaker2-args` a second at a distinct IP -- used to prove two engines with different
        # router-ids establish without colliding (spec-bgp-plugin-speaker AC-5).
        speaker_dir = os.path.join(
            os.path.dirname(os.path.abspath(__file__)), "speaker"
        )
        for args_name, container, ip in (
            ("speaker-args", SPEAKER_CONTAINER, SPEAKER_IP),
            ("speaker2-args", SPEAKER2_CONTAINER, SPEAKER2_IP),
        ):
            args_file = os.path.join(self.scenario_dir, args_name)
            if not os.path.isfile(args_file):
                continue
            with open(args_file, "r", encoding="utf-8") as fh:
                speaker_args = fh.read().split()
            docker_run(
                container,
                "ze-interop",
                ip,
                volumes=["%s:/speaker:ro" % speaker_dir],
                extra_args=["--entrypoint", "python3"],
                cmd=["/speaker/engine.py", "--connect", "%s:179" % ZE_IP]
                + speaker_args,
            )

        # Start FRR if config exists.
        frr_conf = os.path.join(self.scenario_dir, "frr.conf")
        if os.path.isfile(frr_conf):
            script_dir = os.path.dirname(os.path.abspath(__file__))
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
                extra_args=_IPV6_SYSCTLS,
            )

        # Start BIRD if config exists.
        bird_conf = os.path.join(self.scenario_dir, "bird.conf")
        if os.path.isfile(bird_conf):
            docker_run(
                BIRD_CONTAINER,
                "bird-interop",
                BIRD_IP,
                volumes=[
                    "%s:/etc/bird/bird.conf:ro" % os.path.abspath(bird_conf),
                ],
                caps=["NET_ADMIN"],
            )

        # Start keepalived if config exists (the VRRP peer, spec-vrrp-6).
        #
        # NET_ADMIN alone is not enough here, unlike BIRD: keepalived must add
        # the virtual IP to an interface and (with use_vmac) create a macvlan
        # carrying the RFC 9568 virtual MAC, and it sends VRRP adverts over a
        # raw IP protocol 112 socket, which needs NET_RAW. NET_BROADCAST lets it
        # reach 224.0.0.18. Without these it starts, logs, and silently never
        # becomes Master, which reads as a ze bug.
        keepalived_conf = os.path.join(self.scenario_dir, "keepalived.conf")
        if os.path.isfile(keepalived_conf):
            docker_run(
                KEEPALIVED_CONTAINER,
                "keepalived-interop",
                KEEPALIVED_IP,
                volumes=[
                    "%s:/etc/keepalived/keepalived.conf:ro"
                    % os.path.abspath(keepalived_conf),
                ],
                caps=["NET_ADMIN", "NET_RAW", "NET_BROADCAST"],
                extra_args=_IPV6_SYSCTLS,
            )

        # Start GoBGP if config exists.
        gobgp_conf = os.path.join(self.scenario_dir, "gobgp.toml")
        if os.path.isfile(gobgp_conf):
            docker_run(
                GOBGP_CONTAINER,
                "gobgp-interop",
                GOBGP_IP,
                volumes=[
                    "%s:/etc/gobgp/gobgp.toml:ro" % os.path.abspath(gobgp_conf),
                ],
                caps=["NET_ADMIN"],
            )

        # Wait for containers to be healthy.
        wait_containers_healthy(30)

    def teardown(self, strict=False):
        """Remove containers and network.

        `strict` picks the same two contracts as `docker_rm`, and for the same
        reason: `setup` calls this to PRE-CLEAN and must not proceed on a
        removal that failed, while `run.py`'s `finally` must not lose the run's
        summary to one.
        """
        docker_rm(ZE_CONTAINER, strict)
        docker_rm(FRR_CONTAINER, strict)
        docker_rm(BIRD_CONTAINER, strict)
        docker_rm(GOBGP_CONTAINER, strict)
        docker_rm(BMP_CONTAINER, strict)
        docker_rm(RPKI_CONTAINER, strict)
        docker_rm(INJECT_CONTAINER, strict)
        docker_rm(SPEAKER_CONTAINER, strict)
        docker_rm(SPEAKER2_CONTAINER, strict)
        docker_rm(KEEPALIVED_CONTAINER, strict)
        docker_rm(STAYRTR_CONTAINER, strict)
        # The rendered copy is host-side and unrelated to docker, so it is
        # cleared whatever the network removal does.
        #
        # The EXTRACTION is what removes the leak: while the removal sat inline
        # here, its early `return` exited `teardown` itself and skipped this
        # block, so a timed-out removal left the copy behind for the whole run.
        # A `return` inside `_remove_network` now exits only that method. The
        # `finally` keeps the property if the removal ever raises with a copy
        # rendered, which `setup` cannot reach today because it pre-cleans
        # before it renders.
        try:
            self._remove_network(strict)
        finally:
            if self.rendered_dir:
                shutil.rmtree(self.rendered_dir, ignore_errors=True)
                self.rendered_dir = None
                self.scenario_dir = self.source_dir

    def _remove_network(self, strict):
        """Remove the lab network, under the same two contracts as `docker_rm`."""
        try:
            result = subprocess.run(
                ["docker", "network", "rm", NETWORK],
                capture_output=True,
                text=True,
                timeout=30,
            )
        except subprocess.TimeoutExpired:
            if strict:
                raise RuntimeError(
                    "docker network rm %s timed out after 30s: a leftover "
                    "network would race this scenario" % NETWORK
                )
            print(
                "--- docker network rm %s timed out after 30s, network can be "
                "left behind ---" % NETWORK
            )
            return
        except OSError as exc:
            if strict:
                raise RuntimeError(
                    "docker network rm %s could not run (%s): a leftover "
                    "network would race this scenario" % (NETWORK, exc)
                )
            print(
                "--- docker network rm %s could not run (%s), network can be "
                "left behind ---" % (NETWORK, exc)
            )
            return
        # A network that is not there is the ORDINARY pre-clean case, and docker
        # reports it as exit 1 `network <name> not found` (measured on docker
        # 29.7.1), so this one needs the message and not the code alone.
        #
        # The exemption is matched on the WHOLE phrase including this network's
        # name. A bare "not found" is not specific to a missing network: a
        # misconfigured `DOCKER_CONTEXT` answers `context ... not found` having
        # removed nothing, and that would exempt itself. Anything the exemption
        # does not match denies, and "has active endpoints" is the shape that
        # says a container is still holding the network.
        absent = "network %s not found" % NETWORK
        stderr = (result.stderr or "").strip()
        if strict and result.returncode != 0 and absent not in stderr:
            raise RuntimeError(
                "docker network rm %s failed (exit %d): %s -- a leftover "
                "network would race this scenario"
                % (NETWORK, result.returncode, stderr or "no stderr")
            )

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
            raise RuntimeError(
                "check.py in %s failed to load: %s" % (self.name, e)
            ) from e
        if not hasattr(mod, "check"):
            raise RuntimeError("check.py in %s has no check() function" % self.name)
        mod.check()


def global_cleanup():
    """Remove all containers and network on exit."""
    # No docker binary means no container of ours can exist, so there is nothing to
    # remove -- a complete answer, not a swallowed error. Without this the atexit hook
    # raises FileNotFoundError on every Docker-less run and Python prints "Exception
    # ignored in atexit callback" plus a traceback AFTER the runner's own message. That
    # path is routine now that run.py exits 1 there and a Go test exercises it on every
    # `go test ./...` (test/interop/run_test.go).
    if shutil.which("docker") is None:
        return
    for name in [
        ZE_CONTAINER,
        FRR_CONTAINER,
        BIRD_CONTAINER,
        GOBGP_CONTAINER,
        BMP_CONTAINER,
        RPKI_CONTAINER,
        KEEPALIVED_CONTAINER,
        STAYRTR_CONTAINER,
        INJECT_CONTAINER,
        SPEAKER_CONTAINER,
        SPEAKER2_CONTAINER,
    ]:
        subprocess.run(
            ["docker", "rm", "-f", name], capture_output=True, text=True, timeout=30
        )
    subprocess.run(
        ["docker", "network", "rm", NETWORK], capture_output=True, text=True, timeout=30
    )


atexit.register(global_cleanup)
