#!/usr/bin/env python3
"""Process plugin that announces routes with standard and large communities.

Announces routes with:
- Standard community: 65001:100, 65001:200
- Large community: 65001:0:1
This verifies Ze can encode community attributes that FRR parses correctly.
"""

import time

from ze_api import flush, ready, wait_for_shutdown

ready()

# Let BGP session fully establish.
time.sleep(1)

# Announce route with standard communities.
flush(
    "peer * update text origin igp path 65001 nhop 172.30.0.2 community [65001:100 65001:200] nlri ipv4/unicast add 10.10.0.0/24\n"
)
time.sleep(0.1)

# Announce route with large community.
flush(
    "peer * update text origin igp path 65001 nhop 172.30.0.2 large-community [65001:0:1] nlri ipv4/unicast add 10.10.1.0/24\n"
)

# Keep plugin alive for the check script.
wait_for_shutdown(timeout=120)
