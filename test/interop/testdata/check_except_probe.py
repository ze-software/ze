#!/usr/bin/env python3
"""Drive scenario 55's `_check` failure path with no Docker and no containers.

`_check` catches its own assertion, asks whether a process plugin was the real
cause, and re-raises. Two answers must reach the runner differently:

  * the plugin signalled failure. Its message REPLACES the assertion, because
    "BIRD route not found" is the symptom and the plugin's message is the cause.
  * Ze's log could not be READ. The assertion must survive, because an unreadable
    log establishes nothing and would otherwise replace a real failure with a
    docker error.

The second case only became reachable when the read was made strict, and it is
what this probe pins.

    check_except_probe.py sentinel
    check_except_probe.py unreadable
    check_except_probe.py unreadable-oserror

Neither mode starts a container. `_check` is driven directly rather than
`check`, because `check`'s diagnostic dump talks to Docker on the way out.
"""

import importlib.util
import os
import sys

PROBE_DIR = os.path.dirname(os.path.abspath(__file__))
INTEROP_DIR = os.path.dirname(PROBE_DIR)
CHECK_PATH = os.path.join(
    INTEROP_DIR, "scenarios", "55-wire-edit-api-origin-bird", "check.py"
)

sys.path.insert(0, INTEROP_DIR)

spec = importlib.util.spec_from_file_location("scenario55_check", CHECK_PATH)
check = importlib.util.module_from_spec(spec)
spec.loader.exec_module(check)

ORIGINAL = "BIRD route 10.55.0.0/24 not found after 90s"


class FakeBIRD:
    """Establishes, then loses the route the scenario is waiting for."""

    def wait_session(self, name, timeout=None):
        return True

    def wait_route(self, prefix, timeout=None):
        raise AssertionError(ORIGINAL)


def main():
    if len(sys.argv) != 2:
        print(
            "usage: check_except_probe.py <sentinel|unreadable|unreadable-oserror>",
            file=sys.stderr,
        )
        return 2
    mode = sys.argv[1]
    if mode not in ("sentinel", "unreadable", "unreadable-oserror"):
        print("unknown mode %s" % mode, file=sys.stderr)
        return 2

    calls = {"n": 0}

    def fake_observer_check(when, container=None):
        # The first call sits ahead of the waits and must stay quiet; the second
        # is the one inside the failure handler.
        calls["n"] += 1
        if calls["n"] < 2:
            return
        if mode == "sentinel":
            raise AssertionError(
                "process plugin failed: ZE-OBSERVER-FAIL: queue-rail guard denied"
            )
        if mode == "unreadable-oserror":
            # `docker_logs` converts only `TimeoutExpired`, so a missing or
            # unusable docker binary reaches the wrap as an OSError. This mode
            # is what pins the `OSError` half of the wrap's except clause.
            raise FileNotFoundError(2, "No such file or directory: 'docker'")
        raise RuntimeError("docker logs ze-iop-ze-1 failed (exit 1): No such container")

    check.BIRD = FakeBIRD
    check.raise_if_observer_failed = fake_observer_check

    try:
        check._check()  # noqa: SLF001  # the function under test
    except BaseException as exc:  # noqa: BLE001
        print("RAISED=%s: %s" % (type(exc).__name__, exc))
        return 0
    print("RAISED=none")
    return 0


if __name__ == "__main__":
    sys.exit(main())
