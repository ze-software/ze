#!/usr/bin/env python3
"""Process plugin that announces EVPN Type-2 (MAC/IP) routes to FRR.

Announces EVPN routes via MP_REACH_NLRI to verify Ze can encode
l2vpn/evpn UPDATE messages that FRR accepts.
"""

import time

from ze_api import flush, ready, wait_for_shutdown

ready()

# Let BGP session fully establish.
time.sleep(1)

# EVPN Type-2 MAC/IP advertisement route.
# RD 1:1, MAC 00:11:22:33:44:55, etag 0, label 100.
flush('peer * update text origin igp nhop 172.30.0.2 nlri l2vpn/evpn add mac-ip rd 1:1 mac 00:11:22:33:44:55 etag 0 label 100\n')
time.sleep(0.1)

# Second MAC/IP route with IP address.
flush('peer * update text origin igp nhop 172.30.0.2 nlri l2vpn/evpn add mac-ip rd 1:1 mac 00:11:22:33:44:66 ip 192.168.1.1 etag 0 label 100\n')

wait_for_shutdown(timeout=120)
