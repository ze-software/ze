#!/usr/bin/env python3
"""Scenario ospf-ipsec-ah-frr: OSPFv3 (IPv6 family) with RFC 4552 AH transport-mode IPsec.

VALIDATES: spec-ospf-ext-16 AC-2/AC-13 / User story 2 -- Ze and FRR ospf6d form a Full
OSPFv3 adjacency with matching manual AH transport-mode SAs (RFC 4552 §3, AH MAY); the AH
install path carries integrity only (no encryption).

STATUS: authored-pending-harness. The interop runner (test/interop/interop.py) is BGP-only
today; an OSPF/ospf6d helper does not yet exist, and the kernel XFRM + raw proto-89 paths
require QEMU/CAP_NET_ADMIN. This scenario documents the expected topology and skips until
the OSPF interop harness lands.

Expected wiring once the harness exists:
  1. veth pair ze0 <-> frr0 with fe80:: link-locals.
  2. On BOTH sides install the shared manual AH SA + a proto-89 transport-mode policy:
       ip xfrm state add src <ll> dst ff02::5 proto ah spi 0x101 mode transport \
           auth 'hmac(sha256)' 0x0123...def
       ip -6 xfrm policy add dir out src <ll>/128 dst ::/0 proto ospf \
           tmpl proto ah mode transport
     (Ze installs its side automatically from ze.conf on interface up.)
  3. Start FRR ospf6d and Ze; assert the OSPFv3 adjacency reaches Full and
     `ip xfrm state` shows the AH transport SA (Auth only, no encryption).
"""

import os
import sys

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))

try:
    from interop import log_skip  # type: ignore
except ImportError:  # pragma: no cover - harness helper not present yet

    def log_skip(msg):
        print("SKIP:", msg)
        sys.exit(0)


def check():
    log_skip(
        "ospf-ipsec-ah-frr: OSPFv3 AH IPsec interop needs the OSPF interop harness "
        "and QEMU/CAP_NET_ADMIN; authored pending that harness (spec-ospf-ext-16)."
    )


if __name__ == "__main__":
    check()
