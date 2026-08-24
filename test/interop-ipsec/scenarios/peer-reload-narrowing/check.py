#!/usr/bin/env python3
"""Scenario peer-reload-narrowing: Ze's own child policy narrows mid-tunnel, Ze -> strongSwan.

Validates: an operator edits a live Ze peer's traffic-selector list and commits, and the
           edit reaches the wire. Ze dials the tunnel on 10.2.0.0/24 <-> 10.1.0.0/24, its
           config file is rewritten with 10.2.0.0/25 and SIGHUPed, and Ze must restart the
           peer, re-initiate, and PROPOSE the narrowed pair. strongSwan then answers with
           Ze's own choice (RFC 7296 Section 2.9, a responder narrows to a subset of its
           policy and never widens), and both SPDs must name the narrowed pair with
           nothing of the wide one left.
Prevents:  the defect spec-fixit-ipsec-peer-reload-ignored removes. peerConfigChanged
           compared eight named members of ipsec.SiteToSitePeer and TrafficSelectors was
           not among them, so the commit succeeded, `show configuration` agreed, and the
           tunnel kept carrying the prefix the operator had removed. With the old guard
           this scenario stops at step 3: both kernels still hold the wide pair, because
           Ze never restarted and never proposed anything.

WHY THIS IS THE MIRROR OF child-rekey-narrowing. There, the PEER's policy narrows and Ze follows it.
Here, ZE's policy narrows and the peer follows. The two ends of one obligation: scenario
child-rekey-narrowing proves Ze withdraws a selector it was told to stop protecting, this one proves Ze can
be the end that says so.

WHY THE RESTART AND NOT A REKEY. RFC 7296 Section 2.9.2: "If the rekeyed SA would ever
need to have a narrower scope than the currently used SA, that would mean that the policy
was changed in a way such that the currently used SA is against the policy. In that case,
the SA should have been already deleted after the policy change took effect." A
CREATE_CHILD_SA cannot carry the narrowing, so the reload deletes the SA and dials again.
This is the same action charon takes in scenario child-rekey-narrowing.

Run one scenario: `make ze-interop-ipsec-test IPSEC_INTEROP_SCENARIO=peer-reload-narrowing`.
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
    reload_ze_config,
    ze_cli,
)

SCENARIO_DIR = os.path.dirname(os.path.abspath(__file__))

# The selector pair each end holds, before and after Ze's own policy narrows. The halves
# are unordered because one kernel prints them as the src and dst of an outbound policy
# and the other prints the same pair reversed.
WIDE = frozenset(["10.1.0.0/24", "10.2.0.0/24"])
NARROW = frozenset(["10.1.0.0/24", "10.2.0.0/25"])

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

    # 1. Ze dials the tunnel on the policy both ends were configured with, and the two
    #    kernels agree on it.
    swan.wait_sa_established("ze")
    swan.wait_child_sa("ze-child")
    wait_swan_child_ts(swan, "10.1.0.0/24", "10.2.0.0/24")
    wait_esp_policy(ZE_CONTAINER, WIDE)
    wait_esp_policy(SWAN_CONTAINER, WIDE)
    assert_ze_reports("10.2.0.0/24", "10.1.0.0/24")

    # 2. The operator narrows Ze's half of the policy and commits. On a real box that is
    #    an edit plus `commit`; here it is the mounted config file plus SIGHUP.
    reload_ze_config(SCENARIO_DIR, "ze-narrowed.conf")

    # 3. Ze acts on the change. peerConfigChanged sees the peer differ, reconcilePeers
    #    restarts it, startPeerSession writes the new config, and the fresh session
    #    proposes the narrowed pair. strongSwan answers with Ze's own choice.
    wait_swan_child_ts(swan, "10.1.0.0/24", "10.2.0.0/25")

    # 4. The peer follows onto the narrowed scope. wait_esp_policy asserts the SET of
    #    ESP policies, so a retired 10.2.0.0/24 pair left in either kernel fails here.
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
    assert_ze_reports("10.2.0.0/25", "10.1.0.0/24")
    log_pass("both ends hold %s after Ze's own policy narrowed" % sorted(NARROW))
