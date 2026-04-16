#!/usr/bin/env python3
"""Process plugin that announces multiple paths for the same prefix via Add-Path.

Sends two UPDATE messages for 10.10.0.0/24 with different path-id values
and different AS_PATH, to verify Add-Path capability works end-to-end.
"""

import time

from ze_api import flush, ready, wait_for_shutdown

ready()

# Let BGP session fully establish and Add-Path negotiate.
time.sleep(1)

# Announce two paths for the same prefix with different path-ids and AS paths.
# path-information is a top-level keyword before origin, using dotted-quad notation.
flush(
    "peer * update text path-information 0.0.0.1 origin igp path 65001 65010 nhop 172.30.0.2 nlri ipv4/unicast add 10.10.0.0/24\n"
)
time.sleep(0.1)
flush(
    "peer * update text path-information 0.0.0.2 origin igp path 65001 65020 nhop 172.30.0.2 nlri ipv4/unicast add 10.10.0.0/24\n"
)

# Keep plugin alive for the check script to verify routes.
wait_for_shutdown(timeout=120)
