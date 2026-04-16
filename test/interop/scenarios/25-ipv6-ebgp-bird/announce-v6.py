#!/usr/bin/env python3
"""Process plugin that announces IPv6 unicast routes to BIRD.

Announces 3 IPv6 prefixes via MP_REACH_NLRI to verify Ze can encode
IPv6 UPDATE messages that BIRD accepts.
"""

import time

from ze_api import flush, ready, wait_for_ack, wait_for_shutdown

ready()

# Let BGP session fully establish.
time.sleep(1)

flush(
    "peer * update text origin igp path 65001 nhop 2001:db8::2 nlri ipv6/unicast add 2001:db8:1::/48\n"
)
wait_for_ack(1)
flush(
    "peer * update text origin igp path 65001 nhop 2001:db8::2 nlri ipv6/unicast add 2001:db8:2::/48\n"
)
wait_for_ack(1)
flush(
    "peer * update text origin igp path 65001 nhop 2001:db8::2 nlri ipv6/unicast add 2001:db8:3::/48\n"
)
wait_for_ack(1)

wait_for_shutdown(timeout=120)
