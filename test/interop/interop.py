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


def _set_subnet_prefix(prefix):
    """Update global peer IPs for the allocated /24."""
    global SUBNET_PREFIX, SUBNET_CIDR, ZE_IP, FRR_IP, BIRD_IP, GOBGP_IP, BMP_IP, RPKI_IP
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


def _create_network():
    """Create the Docker network, retrying on overlapping subnets."""
    last_error = ""
    forced = os.environ.get("ZE_INTEROP_SUBNET_PREFIX") or os.environ.get(
        "ZE_INTEROP_SUBNET_INDEX"
    )
    for prefix in _candidate_subnet_prefixes():
        _set_subnet_prefix(prefix)
        result = subprocess.run(
            ["docker", "network", "create", "--subnet=%s" % SUBNET_CIDR, NETWORK],
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
    """Copy a scenario and replace the default Docker subnet prefix."""
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
                with open(dst, "w", encoding="utf-8") as fh:
                    fh.write(text)
            shutil.copymode(src, dst)
    return target


# --- Docker helpers ----------------------------------------------------------


def docker_exec(container, cmd):
    """Run command in container, return stdout. Raises on failure."""
    try:
        result = subprocess.run(
            ["docker", "exec", container] + cmd,
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


def docker_run(name, image, ip, volumes=None, caps=None, extra_args=None, cmd=None):
    """Start a container."""
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
    """Remove container, ignore if not exists."""
    subprocess.run(
        ["docker", "rm", "-f", name],
        capture_output=True,
        text=True,
        timeout=30,
    )


def docker_logs(container, lines=30):
    """Get last N lines of container logs."""
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


# --- FRR helpers -------------------------------------------------------------


class FRR:
    """Helpers for querying FRR via vtysh."""

    def __init__(self, container=None, ip=None):
        self.container = container or FRR_CONTAINER
        self.ip = ip or FRR_IP

    def _vtysh_quiet(self, command):
        """Run a vtysh command, return stdout or empty string on failure."""
        return docker_exec_quiet(self.container, ["vtysh", "-c", command])

    def wait_session(self, neighbor, timeout=None):
        """Poll until BGP session with neighbor reaches Established."""
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

    def route(self, prefix, family="ipv4 unicast"):
        """Get route info as parsed JSON from vtysh."""
        output = self._vtysh_quiet("show bgp %s %s json" % (family, prefix))
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
        """Get prefix count from JSON summary for a specific neighbor."""
        output = self._vtysh_quiet("show bgp ipv4 unicast summary json")
        if not output.strip():
            return 0
        try:
            data = json.loads(output)
            peers = data.get("peers", data.get("ipv4Unicast", {}).get("peers", {}))
            peer = peers.get(neighbor, {})
            return peer.get("pfxSnt", peer.get("pfxRcd", 0))
        except (json.JSONDecodeError, AttributeError):
            log_debug("JSON summary parse failed, falling back to text")
            output = self._vtysh_quiet("show bgp ipv4 unicast summary")
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
        """Check if session is currently Established."""
        output = self._vtysh_quiet("show bgp neighbor %s" % neighbor)
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
            output = self._vtysh_quiet("show bfd peers")
            if peer in output and "Status: up" in output:
                log_pass("FRR BFD peer %s is Up" % peer)
                return
            time.sleep(2)
        log_fail("FRR BFD peer %s did not reach Up within %ds" % (peer, timeout))
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

    def adjacency_up(self):
        """Report whether FRR has at least one IS-IS adjacency in the Up state."""
        out = self._vtysh_quiet("show isis neighbor")
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
        print(self._vtysh_quiet("show isis neighbor")[:500])
        print(docker_logs(ZE_CONTAINER, 30))
        raise AssertionError("FRR IS-IS adjacency not Up")

    def is_dis(self):
        """Report whether FRR shows a Designated IS elected on any circuit (LAN)."""
        out = self._vtysh_quiet("show isis interface detail")
        return "DIS" in out or "Designated" in out

    def has_isis_route(self, prefix):
        """Check FRR's kernel/zebra RIB for an IS-IS-learned IPv4 route."""
        out = self._vtysh_quiet("show ip route isis")
        return prefix in out

    def has_isis_route_v6(self, prefix):
        """Check FRR's kernel/zebra RIB for an IS-IS-learned IPv6 route."""
        out = self._vtysh_quiet("show ipv6 route isis")
        return prefix in out

    def has_database_lsp(self, fragment):
        """Check FRR's IS-IS LSDB for an LSP whose ID contains fragment."""
        out = self._vtysh_quiet("show isis database")
        return fragment in out

    def has_pseudonode_lsp(self):
        """Report whether FRR's LSDB carries a pseudo-node LSP (LAN DIS, AC-14).

        A pseudo-node LSP ID has a non-zero pseudo-node octet, e.g. the
        ".XX-00" segment is non-zero (FRR renders e.g. hostname.01-00).
        """
        out = self._vtysh_quiet("show isis database")
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
        print(self._vtysh_quiet("show ip route isis")[:500])
        print(docker_logs(ZE_CONTAINER, 30))
        raise AssertionError("FRR missing IS-IS %s route %s" % (family, prefix))

    def has_bgp_route_from_isis(self):
        """Report whether FRR's BGP table carries a route learned from a peer.

        Used by the redistribution scenario: an IS-IS route Ze redistributed to
        BGP must land in FRR's BGP RIB. We look for any non-local BGP path.
        """
        out = self._vtysh_quiet("show bgp ipv4 unicast")
        # A received BGP route shows a next-hop that is the Ze peer address.
        return ZE_IP in out

    def isis_summary(self):
        """Return the `show isis summary` text (diagnostics on failure)."""
        return self._vtysh_quiet("show isis summary")


# --- BIRD helpers ------------------------------------------------------------


class BIRD:
    """Helpers for querying BIRD via birdc."""

    def __init__(self, container=None, ip=None):
        self.container = container or BIRD_CONTAINER
        self.ip = ip or BIRD_IP

    def _birdc_quiet(self, command):
        """Run a birdc command, return stdout or empty string on failure."""
        return docker_exec_quiet(self.container, ["birdc", command])

    def wait_session(self, protocol, timeout=None):
        """Poll until protocol reaches Established."""
        if timeout is None:
            timeout = SESSION_TIMEOUT
        log_info("waiting for BIRD protocol %s (timeout %ds)..." % (protocol, timeout))
        deadline = time.time() + timeout
        while time.time() < deadline:
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
        output = self._birdc_quiet("show protocols all")
        print(output)
        print(docker_logs(ZE_CONTAINER, 20))
        raise AssertionError("BIRD protocol %s not Established" % protocol)

    def has_route(self, prefix):
        """Check if prefix in routing table."""
        output = self._birdc_quiet("show route for %s" % prefix)
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
        output = self._birdc_quiet("show route for %s all" % prefix)
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
        """Get exported route count from protocol details."""
        output = self._birdc_quiet("show protocols all %s" % protocol)
        for line in output.splitlines():
            if "Routes:" in line:
                m = re.search(r"(\d+)\s+exported", line)
                if m:
                    return int(m.group(1))
        return 0

    def session_established(self, protocol):
        """Check if protocol is Established."""
        output = self._birdc_quiet("show protocols")
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
            output = self._gobgp_quiet(["neighbor", neighbor])
            if "established" in output.lower():
                log_pass("GoBGP session with %s is Established" % neighbor)
                return
            time.sleep(2)
        log_fail(
            "GoBGP session with %s did not reach Established within %ds"
            % (neighbor, timeout)
        )
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

    def rib_count(self):
        """Return the number of received routes in Ze's RIB, or 0 on failure."""
        output = docker_exec_quiet(self.container, ["ze", "show", "rib", "status"])
        m = re.search(r'"routes-in"\s*:\s*(\d+)', output)
        if m:
            return int(m.group(1))
        return 0

    def rib_received(self, minimum):
        """Assert RIB has >= minimum received routes."""
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

        if _check_container_running(GOBGP_CONTAINER):
            if not _check_container_responsive(GOBGP_CONTAINER, ["gobgp", "neighbor"]):
                all_ready = False

        if all_ready:
            log_debug("all containers healthy")
            return
        time.sleep(1)

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

    def setup(self):
        """Create network, start containers based on which config files exist."""
        self.teardown()

        _create_network()
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
            cmd=["/etc/ze/bgp.conf"],
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

    def teardown(self):
        """Remove containers and network."""
        docker_rm(ZE_CONTAINER)
        docker_rm(FRR_CONTAINER)
        docker_rm(BIRD_CONTAINER)
        docker_rm(GOBGP_CONTAINER)
        docker_rm(BMP_CONTAINER)
        docker_rm(RPKI_CONTAINER)
        subprocess.run(
            ["docker", "network", "rm", NETWORK],
            capture_output=True,
            text=True,
            timeout=30,
        )
        if self.rendered_dir:
            shutil.rmtree(self.rendered_dir, ignore_errors=True)
            self.rendered_dir = None
            self.scenario_dir = self.source_dir

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
    for name in [
        ZE_CONTAINER,
        FRR_CONTAINER,
        BIRD_CONTAINER,
        GOBGP_CONTAINER,
        BMP_CONTAINER,
        RPKI_CONTAINER,
    ]:
        subprocess.run(
            ["docker", "rm", "-f", name], capture_output=True, text=True, timeout=30
        )
    subprocess.run(
        ["docker", "network", "rm", NETWORK], capture_output=True, text=True, timeout=30
    )


atexit.register(global_cleanup)
