#!/usr/bin/env python3
"""Process plugin that announces VPN (L3VPN) routes to GoBGP."""

import time

from ze_api import flush, ready, wait_for_ack, wait_for_shutdown

ready()
time.sleep(1)

flush(
    "peer * update text origin igp nhop 172.30.0.2 nlri ipv4/mpls-vpn rd 65001:100 label 1000 add 10.99.0.0/24\n"
)
wait_for_ack(1)
flush(
    "peer * update text origin igp nhop 172.30.0.2 nlri ipv4/mpls-vpn rd 65001:100 label 1001 add 10.99.1.0/24\n"
)
wait_for_ack(1)

wait_for_shutdown(timeout=120)
