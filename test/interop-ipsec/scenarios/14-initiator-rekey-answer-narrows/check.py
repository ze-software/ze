#!/usr/bin/env python3
"""Scenario 14: strongSwan answers Ze's Child SA rekey below the scope in use.

Validates: Ze, as the CREATE_CHILD_SA INITIATOR, reads the traffic selectors the
           responder answered instead of inheriting the retired pair's, and refuses an
           answer narrower than the scope the SA in use carries. RFC 7296 Section 2.9:
           "TS payloads specify the selection criteria for packets that will be
           forwarded over the newly set up SA." RFC 7296 Section 2.9.2: "Thus, the new
           SA MUST NOT have narrower selectors than the original."
Prevents:  the divergence measured against strongSwan 5.9.14 on 2026-08-22. charon
           logged "inbound CHILD_SA ze-child{2} established with SPIs ... and TS
           10.1.0.0/24 === 10.2.0.0/25" and programmed that pair, while Ze installed a
           replacement carrying 10.2.0.0/24 <-> 10.1.0.0/24. Traffic inside the
           difference is protected at one end and dropped at the other, and neither end
           reports anything.

THE STIMULUS, AND WHY IT IS AN ENV VAR. charon builds a rekey proposal from the
CHILD_SA's stored child_cfg on both roles (child_rekey.c, build_i and process_r), so
`swanctl --load-conns` cannot change what an established CHILD_SA proposes or answers.
A conforming charon therefore never answers below the scope in use on its own: it
narrows what it is OFFERED. ZE_TEST_IKE_REKEY_TS_LOCAL (the ze-env file beside this
one) is what offers it a proposal below that scope, and charon's honest narrowing of
that proposal is the answer under test. RFC 7296 Section 2.9.2 names exactly this
state: "the policy was changed in a way such that the currently used SA is against the
policy."

WHAT DISCRIMINATES. Two facts, and both flip when the producer is reverted. Ze logs the
refusal, and Ze's ESP SPIs do not move. Before the fix Ze accepted the answer, installed
a replacement pair under the RETIRED selectors and deleted the old pair, so its SPIs
changed and no refusal was logged. The SPD prefixes read the same either way, which is
the point: the divergence is invisible in Ze's own policy table.

Run one scenario: `make ze-interop-ipsec-test IPSEC_INTEROP_SCENARIO=14-initiator-rekey-answer-narrows`.
"""

import os
import re
import sys
import time

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from lab import (
    StrongSwan,
    ZE_CONTAINER,
    docker_exec,
    log_info,
    log_pass,
    wait_swan_log,
    wait_ze_log,
    ze_cli,
)

# The scope the SA in use carries: this node's half and the peer's half.
ZE_HALF = "10.2.0.0/24"
PEER_HALF = "10.1.0.0/24"

# The half ZE_TEST_IKE_REKEY_TS_LOCAL offers at the rekey, and the answer charon
# narrows to. It sits INSIDE the proposal and covers no pair of the scope in use.
NARROWED_HALF = "10.2.0.0/25"

# Poll interval for every wait below, in seconds.
_TICK = 2

_SPI = re.compile(r"proto esp spi (0x[0-9a-fA-F]+)")
_SRC_DST = re.compile(r"^src (\S+) dst (\S+)")


def ze_esp_spis():
    """Every ESP SPI Ze's kernel holds a state for.

    Read through docker_exec, which raises when the command fails. The tunnel is up
    before any caller runs, so an empty set is a failed read rather than an answer, and
    an SPI set compared against an empty one would call every outcome unchanged.
    """
    text = docker_exec(ZE_CONTAINER, ["ip", "xfrm", "state"])
    spis = frozenset(_SPI.findall(text))
    if not spis:
        raise RuntimeError(
            "`ip xfrm state` in %s holds no ESP state; this scenario asserts over those "
            "SPIs, so an empty set is a failed read" % ZE_CONTAINER
        )
    return spis


def ze_esp_policy_prefixes():
    """Every prefix an ESP policy selector names in Ze's SPD.

    A flat set rather than pairs: the question here is which SCOPES Ze programs, and the
    answer must hold the scope in use and never the narrowed half.
    """
    text = docker_exec(ZE_CONTAINER, ["ip", "xfrm", "policy"])
    if not text.strip():
        raise RuntimeError(
            "`ip xfrm policy` printed nothing in %s; both ends always hold at least the "
            "IKE bypass policies, so an empty dump is a failed read" % ZE_CONTAINER
        )

    prefixes = set()
    state = {"selector": None, "body": []}

    def keep():
        if state["selector"] and any("proto esp" in line for line in state["body"]):
            prefixes.update(state["selector"])

    for line in text.splitlines():
        match = _SRC_DST.match(line)
        if match:
            keep()
            state = {"selector": [match.group(1), match.group(2)], "body": []}
        elif state["selector"]:
            state["body"].append(line)
    keep()
    return prefixes


def wait_ze_esp_policy(expected, timeout=60):
    """Poll Ze's SPD until its ESP policies name exactly `expected`."""
    log_info("waiting for Ze to hold ESP policy prefixes %s..." % sorted(expected))
    deadline = time.time() + timeout
    seen = set()
    while time.time() < deadline:
        seen = ze_esp_policy_prefixes()
        if seen == expected:
            log_pass("Ze holds ESP policy prefixes %s" % sorted(expected))
            return
        time.sleep(_TICK)
    raise AssertionError(
        "Ze holds ESP policy prefixes %s, expected exactly %s"
        % (sorted(seen), sorted(expected))
    )


def assert_ze_reports(local, remote):
    """Ze's own view of the Child SA, which is what an operator reads."""
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

    # 1. The tunnel comes up on the configured scope. IKE_AUTH proposes the whole
    #    policy: the override applies to a REKEY proposal alone (initiateChildRekey,
    #    engine/rekey.go), so the SA in use carries the wide pair.
    swan.wait_sa_established("ze")
    swan.wait_child_sa("ze-child")
    wait_swan_log("%s === %s" % (PEER_HALF, ZE_HALF))
    wait_ze_esp_policy({ZE_HALF, PEER_HALF})
    assert_ze_reports(ZE_HALF, PEER_HALF)

    before = ze_esp_spis()
    log_info("Ze holds ESP SPIs %s before the rekey" % sorted(before))

    # 2. Ze's ESP soft lifetime expires at about 15s of 30, and it sends the rekey with
    #    the narrowed TSi. charon narrows its own child policy against that proposal and
    #    ANSWERS the narrowed pair, which is the answer this scenario is about. charon's
    #    log is the proof that the stimulus crossed the wire and the peer acted on it.
    wait_swan_log("%s === %s" % (PEER_HALF, NARROWED_HALF))
    log_pass(
        "strongSwan answered the rekey with %s === %s" % (PEER_HALF, NARROWED_HALF)
    )

    # 3. Ze refuses that answer. RFC 7296 Section 2.9.2 forbids the new SA carrying
    #    narrower selectors than the original, and Section 2.9 refuses an answer wider
    #    than the proposal, so no legal set is left. The refusal names both scopes, so an
    #    operator can see which prefix the peer dropped.
    wait_ze_log(
        "narrows the scope in use %s <-> %s down to %s <-> %s"
        % (ZE_HALF, PEER_HALF, NARROWED_HALF, PEER_HALF)
    )

    # 4. The refusal installed nothing. Before the fix Ze accepted the answer, installed
    #    a replacement pair carrying the RETIRED selectors and deleted the old pair, so
    #    every ESP SPI moved. This is the assertion that reddens on a revert.
    after = ze_esp_spis()
    if after != before:
        raise AssertionError(
            "Ze's ESP SPIs moved from %s to %s: it installed a replacement Child SA for "
            "an answer that narrows the scope in use, so the peer now protects %s while "
            "Ze protects %s" % (sorted(before), sorted(after), NARROWED_HALF, ZE_HALF)
        )
    log_pass("Ze kept its Child SA: ESP SPIs are still %s" % sorted(before))

    # 5. The SA in use is untouched, in the kernel and in Ze's own view, and the narrowed
    #    half reaches the SPD by neither route.
    prefixes = ze_esp_policy_prefixes()
    if prefixes != {ZE_HALF, PEER_HALF}:
        raise AssertionError(
            "Ze's ESP policies name %s, expected exactly %s"
            % (sorted(prefixes), sorted([ZE_HALF, PEER_HALF]))
        )
    assert_ze_reports(ZE_HALF, PEER_HALF)
    log_pass(
        "Ze kept the scope in use %s <-> %s after refusing the narrowed answer"
        % (ZE_HALF, PEER_HALF)
    )
