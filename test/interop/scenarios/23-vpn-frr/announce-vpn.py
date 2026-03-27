#!/usr/bin/env python3
"""Process plugin that announces VPN (L3VPN) routes to FRR.

Announces VPNv4 routes with Route Distinguisher and label
via MP_REACH_NLRI to verify Ze can encode ipv4/mpls-vpn
UPDATE messages that FRR accepts.
"""

import time

from ze_api import flush, ready, wait_for_ack, wait_for_shutdown

ready()

# Let BGP session fully establish.
time.sleep(1)

# VPN route with RD 65001:100, label 1000.
flush('peer * update text origin igp nhop 172.30.0.2 nlri ipv4/mpls-vpn rd 65001:100 label 1000 add 10.99.0.0/24\n')
wait_for_ack(1)
flush('peer * update text origin igp nhop 172.30.0.2 nlri ipv4/mpls-vpn rd 65001:100 label 1001 add 10.99.1.0/24\n')
wait_for_ack(1)

wait_for_shutdown(timeout=120)
