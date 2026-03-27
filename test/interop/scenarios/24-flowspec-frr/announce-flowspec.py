#!/usr/bin/env python3
"""Process plugin that announces FlowSpec rules to FRR.

Announces FlowSpec rules via MP_REACH_NLRI to verify Ze can encode
ipv4/flow UPDATE messages that FRR accepts.
"""

import time

from ze_api import flush, ready, wait_for_ack, wait_for_shutdown

ready()

# Let BGP session fully establish.
time.sleep(1)

# FlowSpec rule: discard traffic to 10.99.0.0/24.
# nhop required for MP_REACH_NLRI; add required before FlowSpec components.
flush('peer * update text extended-community [rate-limit:0] nhop 172.30.0.2 nlri ipv4/flow add destination 10.99.0.0/24\n')
wait_for_ack(1)

# FlowSpec rule: rate-limit traffic to 10.99.1.0/24 from 10.0.0.0/8 using TCP.
# Text API uses "destination"/"source" (not "destination-ipv4"/"source-ipv4")
# and plain protocol names (not operator-prefixed).
flush('peer * update text extended-community [rate-limit:9600] nhop 172.30.0.2 nlri ipv4/flow add destination 10.99.1.0/24 source 10.0.0.0/8 protocol tcp\n')
wait_for_ack(2)

wait_for_shutdown(timeout=120)
