#!/usr/bin/env python3
"""Process plugin that announces EVPN Type-2 (MAC/IP) routes to GoBGP."""

import time

from ze_api import flush, ready, wait_for_ack, wait_for_shutdown

ready()
time.sleep(1)

flush(
    "peer * update text origin igp nhop 172.30.0.2 nlri l2vpn/evpn add mac-ip rd 1:1 mac 00:11:22:33:44:55 etag 0 label 100\n"
)
wait_for_ack(1)
flush(
    "peer * update text origin igp nhop 172.30.0.2 nlri l2vpn/evpn add mac-ip rd 1:1 mac 00:11:22:33:44:66 ip 192.168.1.1 etag 0 label 100\n"
)
wait_for_ack(2)

wait_for_shutdown(timeout=120)
