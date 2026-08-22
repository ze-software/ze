#!/usr/bin/env python3
"""Scenario show-rib-under-frr-load: RIB dumps while a real peer feeds routes.

Validates: AC-5 and AC-12 of spec-record-answers-3-zero-alloc against a real
           peer. FRR announces and withdraws a 256-route block over and over
           while ze answers `show bgp rib`, `show bgp rib count` and
           `show bgp rib best`. Every dump answers, the table follows FRR, and
           the session stays Established.
Prevents:  a RIB read path that stops answering while UPDATEs are being
           processed. Both show walks iterate a PeerRIB under its read lock
           while handleReceivedStructured takes the write side for every UPDATE
           it applies, and the best-path walk dereferences pool handles the
           UPDATE path may release.

WHAT MAKES THIS DISCRIMINATE, measured on 2026-08-22. Reverting the deadlock fix
in inboundSource -- reading the ADD-PATH flag with peerRIB.IsAddPath from inside
the IterateSorted callback instead of AddPathFamilies before the walk -- and
rebuilding the ze image reddens this scenario with

  docker exec <ze> ze cli -c show bgp rib count ... timed out after 30s

which is the wedge: the walk never returns, so ze answers nothing more.

The first shape of this check did NOT discriminate, and the reason is worth
keeping. It toggled FRR and then ran its walks, in turns. vtysh returns before
FRR has sent anything and the next `ze cli` costs an SSH round trip, so every
burst was over before the walk meant to race it started, and the reverted daemon
passed. The flapper and the walkers run at the same time now, and several
walkers run at once, so the phase between a burst and a walk is left to the
schedulers instead of being fixed at "after".

Every command this file runs against a container goes through docker_exec or a
helper built on it, so a non-zero return code raises rather than reading as an
empty answer.
"""

import os
import re
import sys
import threading
import time

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from interop import (
    FRR,
    SESSION_TIMEOUT,
    ZE_IP,
    Ze,
    docker_exec,
    log_info,
    log_pass,
    poll,
)

# The routes frr.conf originates. The check reads this back rather than
# trusting it: a block that shrank would otherwise make every assertion below
# weaker without saying so.
ROUTES = 256

# How long the flapper and the walkers run together. Long enough that many walks
# land inside a burst of UPDATEs, short enough to stay a test.
LOAD_SECONDS = 45

# How many walkers dump the RIB at once. Each `ze cli` costs an SSH round trip,
# so one walker alone leaves the daemon idle between its dumps and the bursts
# pass unwitnessed.
WALKERS = 8


def rib_count(ze):
    """Return what `show bgp rib count` walked. Raises on a failed query.

    This drives the same iteration the document form drives and answers one
    number, so it is the cheapest way to run the walk many times. It raises
    rather than answering 0, because 0 is a legitimate table size after a
    withdrawal and a failed query is not (ai/rules/evidence.md).
    """
    output = ze.cli("show bgp rib count")
    match = re.search(r'"count"\s*:\s*(\d+)', output)
    if match is None:
        raise RuntimeError(
            "`show bgp rib count` in container %s answered without a count "
            "field (harness failure, not an empty RIB): %r"
            % (ze.container, output[:400])
        )
    return int(match.group(1))


def redistribute(frr, enable):
    """Turn FRR's static redistribution on or off, and raise if vtysh refused."""
    verb = "redistribute static" if enable else "no redistribute static"
    docker_exec(
        frr.container,
        [
            "vtysh",
            "-c", "configure terminal",
            "-c", "router bgp 65002",
            "-c", "address-family ipv4 unicast",
            "-c", verb,
        ],
    )


def document_routes(ze):
    """Count the received routes in one `show bgp rib` document.

    An empty table answers {} rather than an empty adj-rib-in map, and that is a
    real answer during the load below, because FRR withdraws its whole block on
    every other toggle. What this refuses is an answer that is not the document
    at all, which is a failed query wearing an empty result.
    """
    document = ze.cli_json("show bgp rib", want=dict)
    rib_in = document.get("adj-rib-in", {})
    if not isinstance(rib_in, dict):
        raise RuntimeError(
            "`show bgp rib` in container %s answered a non-object adj-rib-in "
            "(harness failure, not an empty RIB): %r" % (ze.container, rib_in)
        )
    return sum(len(routes) for routes in rib_in.values())


def best_path_rows(ze):
    """Read the best-path walk, bounded, and return how many rows it carried.

    `first 50` bounds the walk itself rather than its rendering, so this reads
    the row path without asking the daemon for a document the size of the table.
    The rows arrive as the array the collapse rebuilt, one object per route.
    """
    rows = ze.cli_json("show bgp rib best first 50", want=list)
    for row in rows:
        if "prefix" not in row:
            raise RuntimeError(
                "`show bgp rib best first 50` in container %s answered a row "
                "with no prefix (harness failure, not an empty RIB): %r"
                % (ze.container, row)
            )
    return len(rows)


def run_load(frr, ze):
    """Run the flapper and the walkers together, and return (counts, walks).

    The flapper toggles FRR's redistribution as fast as vtysh answers. The
    walkers dump ze's RIB as fast as `ze cli` answers. Neither waits for the
    other, so the phase between a burst of UPDATEs and a walk is whatever the
    schedulers make it, and over LOAD_SECONDS some walks iterate the peer RIB
    while the UPDATE path is writing it.

    A wedged walk shows up here as the 30s docker exec timeout docker_exec
    raises, which this re-raises on the calling thread rather than swallowing.
    """
    stop = threading.Event()
    counts = set()
    walks = [0]
    errors = []
    guard = threading.Lock()

    def flapper():
        enable = False
        try:
            while not stop.is_set():
                redistribute(frr, enable)
                enable = not enable
        except Exception as exc:  # noqa: BLE001  # re-raised on the main thread
            with guard:
                errors.append(exc)

    def walker(documents):
        """Dump the RIB until told to stop.

        One walker reads the whole document rather than its count. The document
        form is what an operator reads and it holds the read lock for far longer
        than the counting form does, so it is the walk most likely to have a
        write land inside it.
        """
        try:
            while not stop.is_set():
                if documents:
                    document_routes(ze)
                    with guard:
                        walks[0] += 1
                    continue
                count = rib_count(ze)
                with guard:
                    counts.add(count)
                    walks[0] += 1
        except Exception as exc:  # noqa: BLE001  # re-raised on the main thread
            with guard:
                errors.append(exc)

    threads = [threading.Thread(target=flapper, daemon=True)]
    threads += [
        threading.Thread(target=walker, args=(index == 0,), daemon=True)
        for index in range(WALKERS)
    ]
    for thread in threads:
        thread.start()

    deadline = time.monotonic() + LOAD_SECONDS
    while time.monotonic() < deadline and not errors:
        time.sleep(0.5)
    stop.set()
    for thread in threads:
        thread.join(timeout=60)

    if errors:
        raise errors[0]

    # Leave FRR announcing, so the table comes back for the assertions below.
    redistribute(frr, True)
    log_info("load: %d walks, totals seen %s" % (walks[0], sorted(counts)))
    return counts, walks[0]


def check():
    frr = FRR()
    ze = Ze()

    frr.wait_session(ZE_IP)

    # FRR's whole block has to reach ze before the load starts, or the cycles
    # below would be measuring an empty table.
    # SESSION_TIMEOUT is this suite's budget for a peer becoming useful, and
    # that is what this waits for: FRR installs 256 statics into zebra and
    # redistributes them as zebra reports each one, so the table arrives over a
    # period rather than in one message. A one-second interval returns as soon
    # as it has, which is milliseconds on an idle host.
    settled_count = poll(
        lambda: rib_count(ze),
        lambda count: count >= ROUTES,
        timeout=SESSION_TIMEOUT,
        interval=1,
        what="ze RIB reaching %d routes" % ROUTES,
    )
    assert settled_count >= ROUTES, (
        "ze received %d of FRR's %d routes" % (settled_count, ROUTES)
    )
    log_pass("ze received FRR's %d routes" % ROUTES)

    # One document dump and one best-path walk on the settled table, so a broken
    # answer is reported as a broken answer rather than as a timing result.
    settled = document_routes(ze)
    assert settled >= ROUTES, "show bgp rib dumped %d routes, expected >= %d" % (
        settled,
        ROUTES,
    )
    log_pass("show bgp rib dumped %d routes" % settled)

    rows = best_path_rows(ze)
    assert rows == 50, "show bgp rib best first 50 carried %d rows, expected 50" % rows
    log_pass("show bgp rib best first 50 carried %d rows" % rows)

    # The load. The flapper and the walkers run at the same time rather than in
    # turns, because a turn-taking loop measures nothing: vtysh returns before
    # FRR has sent anything, and the next `ze cli` needs an SSH round trip, so
    # every burst is over before the walk that was meant to race it starts.
    counts, walks = run_load(frr, ze)
    log_pass("%d walks answered while FRR announced and withdrew its block" % walks)

    # The load has to have reached ze for any of the above to mean anything. A
    # run where every walk saw the same table never exercised the UPDATE path
    # beside the walk, and would pass here for a concurrency that was not there.
    assert len(counts) >= 2, (
        "every walk saw the same route total %s, so FRR's announce and withdraw "
        "cycles never reached ze while the walks ran" % sorted(counts)
    )

    # FRR ends with redistribution ON, so the table comes back.
    final_count = poll(
        lambda: rib_count(ze),
        lambda count: count >= ROUTES,
        timeout=SESSION_TIMEOUT,
        interval=1,
        what="ze RIB returning to %d routes" % ROUTES,
    )
    assert final_count >= ROUTES, (
        "ze RIB holds %d routes after the load, expected %d back"
        % (final_count, ROUTES)
    )
    log_pass("ze RIB returned to %d routes after the load" % ROUTES)

    assert frr.session_established(ZE_IP), "session dropped under the RIB dumps"
    log_pass("session still Established after %d walks" % walks)
