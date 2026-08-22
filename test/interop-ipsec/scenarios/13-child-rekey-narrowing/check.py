#!/usr/bin/env python3
"""Scenario 13: the peer's child policy narrows mid-tunnel, Ze <- strongSwan.

Validates: the two ends hold the SAME SPD after the peer's traffic-selector policy
           narrows. strongSwan dials the tunnel on 10.1.0.0/24 <-> 10.2.0.0/24, its
           child policy is narrowed to 10.1.0.0/25 and reloaded, and charon then
           deletes the SA the old policy built and dials a new one. Ze must follow it
           onto the narrowed scope and keep NOTHING of the wide one. The assertion is
           read from both kernels: the ESP policy selectors in the Ze container and in
           the strongSwan container must name the same pair, and Ze's own
           `show vpn ipsec sa` must name that pair too.
Prevents:  a retired selector left installed after the peer's Delete. Ze programs the
           SPD per Child SA and a replacement claims the selector under the same Owner
           (dataplane.SPParams). A pair that is never withdrawn leaves Ze protecting
           10.1.0.128/25 that the peer now drops, which is the silent one-way tunnel
           this area exists to prevent.

WHAT THIS SCENARIO DOES NOT PROVE. It does not exercise the RFC 7296 Section 2.9.2
refusal in `narrowChildSelectors` (engine/ts_narrow.go). That refusal answers a
CREATE_CHILD_SA rekey whose selectors no longer cover the scope in use, and strongSwan
cannot send one. charon builds a rekey from the CHILD_SA's stored child_cfg on both
roles (child_rekey.c, build_i and process_r), so `swanctl --load-conns` never changes
what an established CHILD_SA proposes. With start_action = start charon deletes the SA
and dials again, which is what Section 2.9.2 says a policy change should do: "the SA
should have been already deleted after the policy change took effect". The refusal is
proven between two Ze daemons in test/ipsec/ipsec-child-rekey-xfrm-narrowing.ci and in
the engine unit tests.

Run one scenario: `make ze-interop-ipsec-test IPSEC_INTEROP_SCENARIO=13-child-rekey-narrowing`.
"""

import os
import re
import sys
import time

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from lab import (
    StrongSwan,
    SWAN_CONTAINER,
    ZE_CONTAINER,
    docker_exec,
    log_info,
    log_pass,
    wait_swan_log,
    ze_cli,
)

# The selector pair each end holds, before and after the peer's policy narrows. The
# halves are unordered because one kernel prints them as the src and dst of an outbound
# policy and the other prints the same pair reversed.
WIDE = frozenset(["10.1.0.0/24", "10.2.0.0/24"])
NARROW = frozenset(["10.1.0.0/25", "10.2.0.0/24"])

# conf.d files are read in name order and a later value replaces an earlier one, so this
# file narrows local_ts without writing to the read-only interop.conf mount.
NARROW_CONF = """connections {
    ze {
        children {
            ze-child {
                local_ts = 10.1.0.0/25
            }
        }
    }
}
"""

_SRC_DST = re.compile(r"^src (\S+) dst (\S+)")

# Poll interval for every wait below, in seconds.
_TICK = 2


def esp_policy_pairs(container):
    """ESP policy selector pairs from one container's SPD.

    `ip xfrm policy` is read through docker_exec, which raises when the command fails,
    and the dump is then tested for emptiness. Both ends always hold at least the IKE
    bypass policies, so an empty dump is a failed read rather than an answer, and
    reading on would assert over nothing.
    """
    text = docker_exec(container, ["ip", "xfrm", "policy"])
    if not text.strip():
        raise RuntimeError(
            "`ip xfrm policy` printed nothing in %s; this scenario asserts over those "
            "policies, so an empty dump is a failed read" % container
        )

    pairs = set()
    state = {"src": None, "dst": None, "body": []}

    def keep():
        if state["src"] and any("proto esp" in line for line in state["body"]):
            pairs.add(frozenset([state["src"], state["dst"]]))

    for line in text.splitlines():
        match = _SRC_DST.match(line)
        if match:
            keep()
            state = {"src": match.group(1), "dst": match.group(2), "body": []}
        elif state["src"]:
            state["body"].append(line)
    keep()
    return pairs


def wait_esp_policy(container, expected, timeout=90):
    """Poll one container's SPD until its ESP policies name exactly `expected`."""
    log_info("waiting for %s to hold ESP policy %s..." % (container, sorted(expected)))
    deadline = time.time() + timeout
    seen = set()
    while time.time() < deadline:
        seen = esp_policy_pairs(container)
        if seen == {expected}:
            log_pass("%s holds ESP policy %s" % (container, sorted(expected)))
            return
        time.sleep(_TICK)
    raise AssertionError(
        "%s holds ESP policy %s, expected exactly %s"
        % (container, [sorted(pair) for pair in seen], sorted(expected))
    )


def wait_swan_child_ts(swan, local, remote, timeout=90):
    """Poll swanctl until the installed Child SA names this selector pair."""
    log_info("waiting for strongSwan Child SA on %s <-> %s..." % (local, remote))
    deadline = time.time() + timeout
    output = ""
    while time.time() < deadline:
        output = swan.list_sas()
        if "local  %s" % local in output and "remote %s" % remote in output:
            log_pass("strongSwan Child SA carries %s <-> %s" % (local, remote))
            return
        time.sleep(_TICK)
    raise AssertionError(
        "strongSwan Child SA never carried %s <-> %s; swanctl reported:\n%s"
        % (local, remote, output)
    )


def assert_ze_reports(local, remote):
    """Ze's own view of the Child SA must name the pair its kernel holds."""
    output = ze_cli("show vpn ipsec sa")
    for field, value in (("ts-local", local), ("ts-remote", remote)):
        if not re.search(r"^%s\s+%s\s*$" % (field, re.escape(value)), output, re.M):
            raise AssertionError(
                "`show vpn ipsec sa` does not report %s %s; it printed:\n%s"
                % (field, value, output)
            )
    log_pass("Ze reports ts-local %s and ts-remote %s" % (local, remote))


def check():
    swan = StrongSwan()

    # 1. The tunnel comes up on the policy both ends were configured with, and the two
    #    kernels agree on it.
    swan.wait_sa_established("ze")
    swan.wait_child_sa("ze-child")
    wait_swan_child_ts(swan, "10.1.0.0/24", "10.2.0.0/24")
    wait_esp_policy(ZE_CONTAINER, WIDE)
    wait_esp_policy(SWAN_CONTAINER, WIDE)
    assert_ze_reports("10.2.0.0/24", "10.1.0.0/24")

    # 2. The operator narrows the peer's half of the policy and loads it.
    docker_exec(
        SWAN_CONTAINER,
        [
            "sh",
            "-c",
            "cat > /etc/swanctl/conf.d/zz-narrow.conf <<'EOF'\n%s\nEOF" % NARROW_CONF,
        ],
    )
    loaded = docker_exec(SWAN_CONTAINER, ["swanctl", "--load-conns"])
    if "loaded connection 'ze'" not in loaded:
        raise AssertionError(
            "strongSwan did not load the narrowed connection; swanctl printed:\n%s"
            % loaded
        )
    log_pass("strongSwan loaded the narrowed child policy")

    # 3. charon acts on the change the way RFC 7296 Section 2.9.2 describes. It deletes
    #    the SA the old policy built, then dials one the new policy allows.
    wait_swan_log("deleting IKE_SA ze[")
    wait_swan_child_ts(swan, "10.1.0.0/25", "10.2.0.0/24")

    # 4. Ze follows the peer onto the narrowed scope. wait_esp_policy asserts the SET
    #    of ESP policies, so a retired 10.1.0.0/24 pair left behind fails here.
    wait_esp_policy(ZE_CONTAINER, NARROW)
    wait_esp_policy(SWAN_CONTAINER, NARROW)

    # 5. The two kernels name the same pair, read once more side by side.
    ze_pairs = esp_policy_pairs(ZE_CONTAINER)
    swan_pairs = esp_policy_pairs(SWAN_CONTAINER)
    if ze_pairs != swan_pairs:
        raise AssertionError(
            "the two ends hold different SPDs: Ze %s, strongSwan %s"
            % ([sorted(p) for p in ze_pairs], [sorted(p) for p in swan_pairs])
        )
    assert_ze_reports("10.2.0.0/24", "10.1.0.0/25")
    log_pass("both ends hold %s after the peer's policy narrowed" % sorted(NARROW))
