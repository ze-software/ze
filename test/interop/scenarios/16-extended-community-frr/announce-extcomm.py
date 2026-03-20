#!/usr/bin/env python3
"""Process plugin that announces routes with extended communities.

Announces a route with extended community target:65001:100 to verify
Ze can encode extended community attributes that FRR accepts.
"""

import time

from ze_api import flush, ready, wait_for_shutdown

ready()

# Let BGP session fully establish.
time.sleep(1)

# Announce route with extended community (route target).
flush('peer * update text origin igp path 65001 nhop 172.30.0.2 extended-community [target:65001:100] nlri ipv4/unicast add 10.10.0.0/24\n')

# Keep plugin alive for the check script.
wait_for_shutdown(timeout=120)
