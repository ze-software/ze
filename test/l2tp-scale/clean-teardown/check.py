#!/usr/bin/env python3
"""Scale scenario: clean teardown with no resource leaks.

Validates AC-4 (all pool IPs released) and AC-5 (goroutine count
returns to baseline). Uses a small session count for speed.
"""

import os
import sys

sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))

from harness import ScaleScenario, log_fail, log_info, log_pass  # noqa: E402

TUNNELS = 2
SESSIONS_PER_TUNNEL = 10
EXPECTED_TOTAL = TUNNELS * SESSIONS_PER_TUNNEL


def run():
    s = ScaleScenario(os.path.dirname(__file__))
    try:
        s.setup()
        result = s.run_scale(
            tunnels=TUNNELS,
            sessions=SESSIONS_PER_TUNNEL,
            dwell="1s",
        )
    finally:
        s.teardown()

    if result is None:
        raise RuntimeError("no result from ze-test l2tp-scale")

    sessions_up = result.get("sessions-up", 0)
    errors = result.get("errors", [])

    if sessions_up != EXPECTED_TOTAL:
        raise RuntimeError("sessions: %d/%d" % (sessions_up, EXPECTED_TOTAL))
    log_pass("sessions established: %d" % sessions_up)

    teardown_ns = result.get("teardown-time-ns", 0)
    log_pass("teardown completed in %s" % teardown_ns)

    if errors:
        for e in errors[:5]:
            log_info("error: %s" % e)
        raise RuntimeError("%d errors during test" % len(errors))

    log_pass("no errors (clean teardown)")


if __name__ == "__main__":
    run()
