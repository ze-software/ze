#!/usr/bin/env python3
"""Ze's OWN MULTI_EXIT_DISC, announced to both external destinations.

This is the half that stops the scenario from passing on a blanket strip. RFC
4271 Section 5.1.4 forbids relaying a metric RECEIVED from a neighboring AS, and
forbids nothing about the metric a speaker sets itself: that metric is what the
attribute exists for. A daemon that dropped attribute 4 on every external
session would satisfy the "no leaked MED" assertion and fail this one.

The route is ORIGINATED here, so it takes the announce rail
(writeAnnounceUpdate, reactor/reactor_wire.go) rather than the forward
transform. The two rails must disagree about a received metric and agree about
an originated one.

The wait matches the injector's: both halves of the scenario travel a live
session rather than a peer-up replay. Ze stores the route either way (the rib
plugin is loaded), so a slow lab still delivers it, and check.py polls.
"""

import time

from ze_api import flush, ready, wait_for_shutdown

# The metric Ze sets. 42, never 100: the injector's relayed metric is 100, so no
# assertion in this scenario can confuse one for the other.
LOCAL_MED = 42

ready()

# Four of Ze's KEEPALIVEs at the 15 s hold ze.conf sets, the same clock
# inject.msg counts, plus a margin for FRR and GoBGP to finish establishing.
time.sleep(25)

# The selector EXCLUDES the injector (peer !<ip>, docs/guide/route-injection.md).
# `ze-test peer` asserts the exact frames it is told to expect, so an announce it
# was not told about is a mismatch that kills its session, and Ze then withdraws
# the relayed routes this scenario exists to inspect. The injector is a source
# here, never a destination.
flush(
    "peer !172.30.0.9 update text origin igp med %d nhop 172.30.0.2 "
    "nlri ipv4/unicast add 10.60.9.0/24\n" % LOCAL_MED
)

# Keep the plugin alive while check.py polls both daemons. Ze withdraws a
# process plugin's routes when it exits.
wait_for_shutdown(timeout=180)
