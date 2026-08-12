#!/usr/bin/env python3
"""VRRP mastership: Ze against a real keepalived, contesting one virtual IP.

Validates: RFC 9568 election between Ze and keepalived on one L2 segment.
           Ze at priority 200 takes and holds the VIP; keepalived at 100 stays
           BACKUP and holds nothing; when Ze goes away keepalived takes the VIP
           over; when Ze returns it preempts and takes it back.
Prevents:  the state VRRP was in before this scenario existed -- 150 unit tags
           and no executed interop of any kind. A codec that round-trips against
           itself proves nothing about whether another implementation accepts
           the advertisement, and mastership is the one outcome the protocol
           exists to decide.

WHY MASTERSHIP AND NOT THE WIRE BYTES. Every assertion here reads which
container owns 172.30.0.100, from `ip -o -f inet addr` inside that container, not from
Ze's own output. keepalived leaves BACKUP for MASTER on its own unless a
higher-priority advertisement keeps arriving (RFC 9568 Section 6.4.2), so
"keepalived is still empty-handed" is a reading of Ze's wire output taken by an
independent implementation. It is not a claim Ze makes about itself.

The three assertions are ordered so each one can only pass for the right reason:

  1. Ze holds the VIP and keepalived does not, CONTINUOUSLY for longer than
     keepalived's own Master_Down_Interval. Checked at one instant this is
     weak: keepalived has not timed out yet, so a Ze that sends nothing at
     all passes it. Held over the window, a silent Ze produces two owners.
  2. Ze is stopped, and keepalived takes the VIP. This is what makes 1 strong:
     keepalived was capable of mastership the whole time and was held off by
     something. Only Ze's advertisements can have been that something.
  3. Ze returns and takes the VIP back (preempt true, priority 200 > 100), and
     keepalived releases it. This proves the election runs again rather than
     latching, and that keepalived yields to Ze as readily as Ze yields to it.

Ze is stopped with SIGTERM rather than SIGKILL because that is the
administrative stop an init system gives a daemon, and because an Active router
sends a Priority 0 advertisement on Shutdown (RFC 9568 Section 6.4.3). Step 2
asserts the takeover lands INSIDE keepalived's Active_Down_Interval, so the
scenario fails if that advertisement stops being sent or stops being accepted:
the slow path would still hand over the VIP, and a budget that allowed it would
call a lost obligation a pass.

There is deliberately no RFC-requirement tag here. RFC9568-6.4.3-8 already
carries both polarities in the unit suite, and an interop-kind tag is a
permanent commitment under check_evidence_ratchet -- a decision for whoever owns
that ledger, not a side effect of writing this scenario.
"""

import os
import sys
import time

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from interop import (  # noqa: E402
    KEEPALIVED_CONTAINER,
    VRRP_VIP,
    ZE_CONTAINER,
    docker_exec,
    docker_logs,
    docker_signal,
    docker_start,
    log_pass,
)

# RFC 9568 Section 6.1: Active_Down_Interval is 3 x Active_Adver_Interval plus
# Skew_Time, and Skew_Time is (256 - Priority) / 256 seconds. Both routers
# advertise at 1s and keepalived runs at priority 100, so it waits 3 + 0.61 =
# 3.61s of silence before it claims the role. A takeover measured BELOW that
# cannot have come from silence: it is ze's shutdown Priority 0 advertisement
# collapsing the wait to Skew_Time, which is what step 2 asserts.
ACTIVE_DOWN_INTERVAL_S = 3.6
# The WAIT budget is deliberately past that, so the timeout path completes and
# the assertion can report which path ran. A budget at the boundary would time
# out with no measurement, and "keepalived never took over" is a different
# failure from "keepalived took over the slow way".
TAKEOVER_BUDGET_S = 12.0
# Ze must reach mastership from a cold start too: it holds no VIP until its
# interface reconcile, its VRRP engine start and one election have all run.
STARTUP_BUDGET_S = 40.0
# Longer than keepalived's Master_Down_Interval (about 3.6s here), so a silent
# Ze gives keepalived time to time out and claim the VIP INSIDE this window.
SOLE_OWNER_WINDOW_S = 8.0
POLL_INTERVAL_S = 0.5


def _addresses(container):
    """Every IPv4 address the container currently holds, on any interface.

    Read over EVERY interface, not over eth0: the VIP sits on eth0 for
    keepalived and on a macvlan carrying the RFC 9568 virtual MAC for Ze, and
    which device it hangs off is an implementation choice. The address is the
    fact this scenario is about.

    `ip -o -f inet addr` and not `ip -j addr`, because the two images do not
    ship the same tool. Alpine's busybox `ip` in Dockerfile.ze has no `-j`, and
    the JSON form fails there with a usage message rather than an empty answer.
    `-o` is one line per address in both, so one parser reads both sides.
    """
    out = docker_exec(container, ["ip", "-o", "-f", "inet", "addr"])
    found = set()
    for line in out.splitlines():
        fields = line.split()
        for i, field in enumerate(fields):
            if field == "inet" and i + 1 < len(fields):
                found.add(fields[i + 1].split("/")[0])
    return found


def _holds_vip(container):
    return VRRP_VIP in _addresses(container)


def _wait_vip(container, want, budget, what):
    """Wait for `container` to hold (or not hold) the VIP, then return.

    Raises with both containers' address sets, because "who has it" is the
    whole verdict and the failure is unreadable without the other side.
    """
    deadline = time.monotonic() + budget
    while True:
        if _holds_vip(container) == want:
            return time.monotonic()
        if time.monotonic() >= deadline:
            raise AssertionError(
                "%s: %s after %.0fs. ze holds %s, keepalived holds %s.\n"
                "--- keepalived log ---\n%s"
                % (
                    what,
                    "the VIP never appeared" if want else "the VIP was never released",
                    budget,
                    sorted(_addresses(ZE_CONTAINER)),
                    sorted(_addresses(KEEPALIVED_CONTAINER)),
                    docker_logs(KEEPALIVED_CONTAINER, 40),
                )
            )
        time.sleep(POLL_INTERVAL_S)


def _assert_sole_owner(owner, other, what):
    """Exactly one router holds the VIP. Two owners is a split brain.

    This is the assertion a scenario that only waited for its own side would
    miss: if the two routers never see each other -- wrong VRID, wrong version,
    an advertisement that does not leave the container -- then BOTH become
    master, both add the address, and the side under test looks correct.
    """
    if _holds_vip(other):
        raise AssertionError(
            "%s: both routers hold %s. They are not seeing each other's "
            "advertisements, so each elected itself.\n--- keepalived log ---\n%s"
            % (what, VRRP_VIP, docker_logs(KEEPALIVED_CONTAINER, 40))
        )
    if not _holds_vip(owner):
        raise AssertionError("%s: %s does not hold %s" % (what, owner, VRRP_VIP))


def _hold_sole_owner(owner, other, window, what):
    """Assert sole ownership CONTINUOUSLY over a window, not at one instant.

    The one-shot form races. Ze takes the VIP within a second or two of start,
    and keepalived does not leave BACKUP until its Master_Down_Interval expires,
    so a check taken the moment Ze is up sees a single owner even when Ze is
    sending nothing at all. Measured: a build whose SendAdvert returns without
    sending passed the instantaneous check and was caught four steps later.

    The window must therefore outlast Master_Down_Interval. Then a Ze that is
    silent leaves keepalived to time out INSIDE the window, both routers hold
    the address, and this reports it here, where the cause is one step away.
    """
    deadline = time.monotonic() + window
    while time.monotonic() < deadline:
        _assert_sole_owner(owner, other, what)
        time.sleep(POLL_INTERVAL_S)


def check():
    # 1. Ze wins the election, and keepalived stays empty-handed for longer than
    #    its own Master_Down_Interval.
    _wait_vip(ZE_CONTAINER, True, STARTUP_BUDGET_S, "ze never became master")
    _hold_sole_owner(
        ZE_CONTAINER, KEEPALIVED_CONTAINER, SOLE_OWNER_WINDOW_S, "initial election"
    )
    log_pass(
        "ze (priority 200) held %s alone for %.0fs; keepalived (100) stayed backup"
        % (VRRP_VIP, SOLE_OWNER_WINDOW_S)
    )

    # 2. Ze leaves, keepalived takes over. This is what proves step 1 was the
    #    protocol and not keepalived failing to start.
    #
    # The takeover TIME is the second reading, and it is about RFC 9568 Section
    # 6.4.3's shutdown obligation: an Active router sends an ADVERTISEMENT with
    # Priority 0 on Shutdown. keepalived takes the VIP over well inside its own
    # Active_Down_Interval, and nothing else explains that: Section 6.4.2 leaves
    # a Backup waiting the full interval on silence, and only the Priority 0
    # advertisement collapses that wait to Skew_Time.
    t0 = time.monotonic()
    docker_signal(ZE_CONTAINER, "TERM")
    t_up = _wait_vip(
        KEEPALIVED_CONTAINER, True, TAKEOVER_BUDGET_S, "keepalived never took over"
    )
    elapsed = t_up - t0
    if elapsed >= ACTIVE_DOWN_INTERVAL_S:
        raise AssertionError(
            "keepalived took %.1fs to claim %s, which is the Active_Down_Interval "
            "timeout path (>= %.1fs). ze's shutdown Priority 0 advertisement "
            "(RFC 9568 Section 6.4.3) did not reach it, so the peer waited out the "
            "silence instead of being handed the role.\n--- keepalived log ---\n%s"
            % (
                elapsed,
                VRRP_VIP,
                ACTIVE_DOWN_INTERVAL_S,
                docker_logs(KEEPALIVED_CONTAINER, 40),
            )
        )
    log_pass(
        "keepalived took %s over %.1fs after ze stopped, inside Skew_Time: ze's "
        "shutdown Priority 0 advertisement was accepted" % (VRRP_VIP, elapsed)
    )

    # 3. Ze returns and preempts. `docker start` restarts the same container
    #    with the same config, so this is the router coming back, not a new one.
    docker_start(ZE_CONTAINER)
    _wait_vip(ZE_CONTAINER, True, STARTUP_BUDGET_S, "ze never preempted")
    _wait_vip(
        KEEPALIVED_CONTAINER,
        False,
        TAKEOVER_BUDGET_S,
        "keepalived kept the VIP after ze preempted",
    )
    _assert_sole_owner(ZE_CONTAINER, KEEPALIVED_CONTAINER, "after preemption")
    log_pass("ze preempted and holds %s again; keepalived released it" % VRRP_VIP)
