#!/usr/bin/env python3
"""Scenario 48: an INDEPENDENT Python speaker catches Ze emitting a duplicate NEXT_HOP.

The injector announces 10.0.0.0/24; adj-rib-in stores it. When the speaker establishes, Ze's
delta-replay re-announces the stored route to it through the wire-mode announce builder
(reactor_api_batch.go buildWireModeUpdate): the stored attribute block carries the received
NEXT_HOP, and the builder is handed a fresh authoritative next-hop, so it must de-duplicate or
emit NEXT_HOP twice. RFC 7606 Section 3(g) makes a duplicated attribute a treat-as-withdraw.
The speaker's no-duplicate-attribute plugin flags exactly that.

Discrimination (both directions proven against a real ze):
  - fix in place  -> every replayed UPDATE carries ONE NEXT_HOP; speaker PASS (GREEN).
  - fix reverted  -> the delta-replay emits "...400304ac1e0009 400304ac1e0009..." (NEXT_HOP
                     twice); speaker FAIL "path attribute type 3 appears more than once" (RED).
Non-vacuous the other way too: check.py asserts the speaker actually RECEIVED the replayed
route (route-bearing-updates >= 1), so a silent session also fails rather than passing quietly.

Why this is the RIGHT harness: the dup lives in buildWireModeUpdate, reached by the wire-mode
announce (the stored block + a separate next-hop). Route-server forwarding (buildFwdBody) is
verbatim and does NOT reach it; the .ci suite cannot reach it either (delivery needs a real
SECOND peer at a distinct IP -- adj-rib-in-replay-on-peerup.ci:4-5). The speaker must STAY
CONNECTED to receive the delta-replay (the first initial-sync UPDATE is verbatim; the dup
arrives on the re-announce), which is why speaker-args uses --stop-after-updates 0. `ze-test
peer` asserts only the bytes it was told to expect; a strict independent peer applies its own
check. Engine + plugin discrimination are also unit-proven (test/interop/speaker/test_engine.py).
"""

import os
import sys
import time

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from interop import (  # noqa: E402
    INJECT_CONTAINER,
    SPEAKER_CONTAINER,
    ZE_CONTAINER,
    docker_logs,
    log_info,
    log_pass,
)


def _speaker_report(timeout=120):
    """Poll the speaker container's logs until it prints its final verdict, or time out."""
    deadline = time.time() + timeout
    while time.time() < deadline:
        logs = docker_logs(SPEAKER_CONTAINER, 60)
        if "result:" in logs:
            return logs
        time.sleep(2)
    return docker_logs(SPEAKER_CONTAINER, 60)


def _field(logs, key, default=None):
    # The engine prints its verdict as "result: PASS" and its notes as "note: <key>: <value>",
    # so match the "<key>:" token anywhere on the line, not only at the start.
    token = key + ":"
    for line in logs.splitlines():
        idx = line.find(token)
        if idx != -1:
            return line[idx + len(token) :].strip()
    return default


def _check():
    log_info("waiting for the speaker's verdict on the replayed 10.0.0.0/24...")
    logs = _speaker_report()

    # Non-vacuous: the speaker must have actually received the replayed route. A broken replay
    # path (route never delivered) must fail here, not pass silently.
    established = _field(logs, "established")
    routes = int(_field(logs, "route-bearing-updates", "0") or "0")
    assert established == "yes", "speaker never reached Established (session failure)"
    assert routes >= 1, (
        "speaker received %d route-bearing UPDATEs; the replay never reached it"
        % routes
    )
    log_pass("speaker established and received the replayed route")

    # Discriminating signal: reverting the buildWireModeUpdate NEXT_HOP de-duplication makes Ze
    # emit NEXT_HOP twice, which this plugin flags -> result FAIL.
    result = _field(logs, "result")
    if result != "PASS":
        raise AssertionError(
            "speaker rejected Ze's replayed UPDATE: %s"
            % _field(logs, "fail", "(no detail)")
        )
    log_pass("speaker accepted Ze's replayed UPDATE (no duplicate attribute)")


def check():
    """Entry point the runner calls. Dumps the speaker, injector, and Ze logs on failure,
    because a silent injector (bad expect file, refused dial) looks like a relay bug."""
    try:
        _check()
    except Exception:
        print("--- speaker log ---")
        print(docker_logs(SPEAKER_CONTAINER, 60))
        print("--- injector log ---")
        print(docker_logs(INJECT_CONTAINER, 40))
        print("--- ze log ---")
        print(docker_logs(ZE_CONTAINER, 60))
        raise
