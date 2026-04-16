#!/usr/bin/env python3
"""Process plugin that announces routes for the graceful restart test.

Announces 2 routes so we can verify GR capability is negotiated
and routes are exchanged normally.
"""

import time

from ze_api import flush, ready, wait_for_shutdown

ready()

# Let BGP session fully establish.
time.sleep(1)

# Announce routes.
flush(
    "peer * update text origin igp path 65001 nhop 172.30.0.2 nlri ipv4/unicast add 10.10.0.0/24\n"
)
time.sleep(0.1)
flush(
    "peer * update text origin igp path 65001 nhop 172.30.0.2 nlri ipv4/unicast add 10.10.1.0/24\n"
)

# Keep plugin alive.
wait_for_shutdown(timeout=120)
