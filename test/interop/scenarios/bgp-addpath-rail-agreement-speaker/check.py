#!/usr/bin/env python3
"""The live forward and the peer-up replay put the SAME bytes on the wire.

GoBGP announces 10.99.0.0/24 once. speaker1 is established before that announce, so
Ze forwards it live. speaker2 dials 45 seconds later, so the only route Ze can give
it is the one it replays from store. Both speakers negotiated ADD-PATH and hold
identical session settings, so the UPDATE bodies they log must be equal, Path
Identifier included.

This scenario says nothing about WHICH identifier Ze picks: with one source there is
nothing for it to collide with. That is bgp-addpath-readvertise-collision-frr's job.
This one asks whether the two rails agree, which is the invariant
spec-fixit-bgp-egress-rail-divergence closed on and the new generator could break.
"""

import os
import sys
import time

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
import interop  # noqa: E402  (the sys.path insert above must run first)

# The speaker's decoder, reached through the interop module's own location. A path
# relative to THIS file would not find it: the runner copies each scenario to a
# rendered directory outside test/interop before it loads check.py.
sys.path.insert(0, os.path.join(os.path.dirname(interop.__file__), "speaker"))
import engine  # noqa: E402

from interop import (  # noqa: E402
    SPEAKER2_CONTAINER,
    SPEAKER2_IP,
    SPEAKER_CONTAINER,
    SPEAKER_IP,
    ZE_CONTAINER,
    GoBGP,
    Ze,
    ZE_IP,
    docker_logs,
    log_fail,
    log_info,
    log_pass,
)

PREFIX = "10.99.0.0/24"
PREFIX_BYTES = bytes([24, 10, 99, 0])  # ADD-PATH framing: length octet, then 3 octets


def speaker_report(container, timeout=150):
    """Poll a speaker container's logs until it prints its final verdict."""
    deadline = time.time() + timeout
    while time.time() < deadline:
        logs = docker_logs(container, 200)
        if "result:" in logs:
            return logs
        time.sleep(3)
    return docker_logs(container, 200)


def notes(logs, key):
    """Return every `note: <key>: <value>` value the engine printed, in order."""
    token = "note: %s:" % key
    out = []
    for line in logs.splitlines():
        idx = line.find(token)
        if idx != -1:
            out.append(line[idx + len(token) :].strip())
    return out


def field(logs, key, default=None):
    token = key + ":"
    for line in logs.splitlines():
        idx = line.find(token)
        if idx != -1:
            return line[idx + len(token) :].strip()
    return default


def route_update(logs, who):
    """Return the one route-bearing UPDATE body a speaker logged, as bytes."""
    established = field(logs, "established")
    if established != "yes":
        log_fail("%s never reached Established with Ze" % who)
        raise AssertionError("%s: no session" % who)
    hexes = notes(logs, "update-hex")
    if len(hexes) != 1:
        log_fail(
            "%s logged %d route-bearing UPDATE(s), expected exactly 1: %s"
            % (who, len(hexes), hexes)
        )
        raise AssertionError("%s: %d route-bearing UPDATEs" % (who, len(hexes)))
    return bytes.fromhex(hexes[0])


def path_identifier(body, who):
    """Return the Path Identifier from an ADD-PATH framed IPv4 NLRI section.

    Fails when the section is not ADD-PATH framed. Without that guard the whole
    comparison is vacuous: two UPDATEs carrying bare prefixes are equal for reasons
    that have nothing to do with the identifier, and a capability Ze silently stopped
    negotiating would leave this scenario green.
    """
    nlri = engine.decode_update(body).nlri
    if len(nlri) != 8 or nlri[4:] != PREFIX_BYTES:
        log_fail(
            "%s received NLRI %s, expected 4 octets of Path Identifier followed by "
            "%s (RFC 7911 Section 3)" % (who, nlri.hex(), PREFIX_BYTES.hex())
        )
        raise AssertionError("%s: NLRI is not ADD-PATH framed" % who)
    return int.from_bytes(nlri[:4], "big")


def peer_state(ze, ip):
    """Return Ze's own state for one peer: `established`, `active`, and so on."""
    peers = ze.cli_json("show bgp peer list", want=dict).get("peers", {})
    return peers.get(ip, {}).get("state", "absent")


def wait_peer_established(ze, ip, who, timeout=60):
    deadline = time.time() + timeout
    while time.time() < deadline:
        if peer_state(ze, ip) == "established":
            log_pass("%s is established with Ze" % who)
            return
        time.sleep(2)
    log_fail("%s never established with Ze within %ds" % (who, timeout))
    raise AssertionError("%s: no session" % who)


def _check():
    gobgp = GoBGP()
    ze = Ze()

    # The announce must land while speaker1 holds a session and speaker2 does not.
    # Ze's own peer list is what decides that, rather than a sleep: the whole point
    # of the comparison is that one copy was forwarded live and the other replayed
    # from store, and a run where both clients were connected at announce time
    # compares two live forwards and passes for the wrong reason.
    wait_peer_established(ze, SPEAKER_IP, "speaker1")
    gobgp.wait_session(ZE_IP)

    log_info("announcing %s from GoBGP while only speaker1 is connected..." % PREFIX)
    gobgp.inject_route(PREFIX)

    deadline = time.time() + 30
    while time.time() < deadline and ze.rib_count() < 1:
        time.sleep(2)
    ze.rib_received(1)

    state = peer_state(ze, SPEAKER2_IP)
    if state == "established":
        log_fail(
            "speaker2 was already established when %s was stored, so its copy could "
            "have been a live forward: this run proves nothing about the replay rail"
            % PREFIX
        )
        raise AssertionError("scenario timing collapsed: speaker2 joined too early")
    log_pass(
        "%s is stored and speaker2 has not joined yet (state %s), so its copy can "
        "only come from the peer-up replay" % (PREFIX, state)
    )

    log_info("waiting for both speakers to report...")
    live = route_update(speaker_report(SPEAKER_CONTAINER), "speaker1 (live forward)")
    replayed = route_update(
        speaker_report(SPEAKER2_CONTAINER), "speaker2 (peer-up replay)"
    )

    live_id = path_identifier(live, "speaker1 (live forward)")
    replayed_id = path_identifier(replayed, "speaker2 (peer-up replay)")
    log_pass(
        "both clients received %s with an ADD-PATH framed NLRI (identifiers %d and %d)"
        % (PREFIX, live_id, replayed_id)
    )

    if live != replayed:
        log_fail("the live forward and the peer-up replay differ on the wire")
        print("  live     : %s" % live.hex())
        print("  replayed : %s" % replayed.hex())
        raise AssertionError("the two forward rails produced different bytes")
    log_pass("the two rails produced identical UPDATE bodies: %s" % live.hex())


def check():
    try:
        _check()
    except Exception:
        print("--- speaker1 log ---")
        print(docker_logs(SPEAKER_CONTAINER, 60))
        print("--- speaker2 log ---")
        print(docker_logs(SPEAKER2_CONTAINER, 60))
        print("--- ze log ---")
        print(docker_logs(ZE_CONTAINER, 60))
        raise
