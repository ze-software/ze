#!/usr/bin/env python3
"""Process plugin that announces one route for the shutdown-cease scenario.

The route is what the check watches: FRR must drop it when Ze is stopped,
rather than keeping it as a stale Graceful Restart route.
"""

import time

from ze_api import flush, ready, wait_for_shutdown

ready()

# Let the BGP session fully establish.
time.sleep(1)

flush(
    "peer * update text origin igp path 65001 nhop 172.30.0.2 nlri ipv4/unicast add 10.10.0.0/24\n"
)

# Keep the plugin alive until ze stops it.
wait_for_shutdown(timeout=180)
