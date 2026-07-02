#!/usr/bin/env python3
"""Process plugin that announces AS112 covering prefixes carrying well-known
BGP communities (RFC 7534 Section 3.4 recommendation; spec-as112-3 AC-4/AC-5).

Announces:
- 192.175.48.0/24 with NO_EXPORT (RFC 1997, 0xFFFFFF01) -- AC-4
- 192.31.196.0/24 with NOPEER (RFC 3765, 0xFFFFFF04) -- AC-5, via the
  "update text" runtime command grammar's well-known-name table
  (parseCommunityText in
  internal/component/bgp/plugins/cmd/update/update_text.go), which names
  the full ~15-entry IANA well-known set including nopeer directly.
"""

import time

from ze_api import flush, ready, wait_for_shutdown

ready()

# Let the BGP session fully establish.
time.sleep(1)

# AC-4: NO_EXPORT on an AS112 covering prefix.
flush(
    "peer * update text origin igp nhop 172.30.0.2 community [no-export] "
    "nlri ipv4/unicast add 192.175.48.0/24\n"
)
time.sleep(0.1)

# AC-5: NOPEER on the other AS112 covering prefix.
flush(
    "peer * update text origin igp nhop 172.30.0.2 community [nopeer] "
    "nlri ipv4/unicast add 192.31.196.0/24\n"
)

# Keep plugin alive for the check script.
wait_for_shutdown(timeout=120)
