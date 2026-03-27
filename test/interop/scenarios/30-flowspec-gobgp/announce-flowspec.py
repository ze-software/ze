#!/usr/bin/env python3
"""Process plugin that announces FlowSpec rules to GoBGP."""

import time

from ze_api import flush, ready, wait_for_ack, wait_for_shutdown

ready()
time.sleep(1)

# nhop required for MP_REACH_NLRI; add required before FlowSpec components.
flush('peer * update text extended-community [rate-limit:0] nhop 172.30.0.2 nlri ipv4/flow add destination 10.99.0.0/24\n')
time.sleep(0.1)
flush('peer * update text extended-community [rate-limit:9600] nhop 172.30.0.2 nlri ipv4/flow add destination 10.99.1.0/24 source 10.0.0.0/8 protocol tcp\n')
wait_for_ack(2)

wait_for_shutdown(timeout=120)
