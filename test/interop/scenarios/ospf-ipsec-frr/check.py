#!/usr/bin/env python3
"""Scenario ospf-ipsec-frr: OSPFv3 (IPv6 family) with RFC 4552 ESP transport-mode IPsec.

VALIDATES: spec-ospf-ext-16 AC-13 / User story 1 -- Ze and FRR ospf6d form a Full OSPFv3
adjacency over proto-89 multicast with matching manual ESP transport-mode SAs; the wire
carries ESP and an unprotected packet is dropped (RFC 4552 §3/§4).

STATUS: authored-pending-harness. The interop runner (test/interop/interop.py) is BGP-only
today; an OSPF/ospf6d helper (Full-adjacency wait + `ip xfrm` assertion) does not yet
exist, and the kernel XFRM + raw proto-89 paths require QEMU/CAP_NET_ADMIN. This scenario
therefore documents the expected topology and skips until the OSPF interop harness lands.

Expected wiring once the harness exists:
  1. Bring up a veth pair (ze0 <-> frr0), assign fe80:: link-locals.
  2. On BOTH sides install the shared manual ESP SAs + a proto-89 transport-mode policy:
       ip xfrm state add src <ll> dst ff02::5 proto esp spi 0x100 mode transport \
           auth 'hmac(sha256)' 0x0123...def enc 'ecb(cipher_null)' ''
       ip -6 xfrm policy add dir out src <ll>/128 dst ::/0 proto ospf \
           tmpl proto esp mode transport
     (Ze installs its side automatically from ze.conf on interface up.)
  3. Start FRR ospf6d (frr.conf) and Ze (ze.conf).
  4. Assert the OSPFv3 adjacency reaches Full and `ip xfrm state` shows the transport SA.
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
        "ospf-ipsec-frr: OSPFv3 IPsec interop needs the OSPF interop harness "
        "(ospf6d Full-adjacency + ip-xfrm assertion) and QEMU/CAP_NET_ADMIN; "
        "authored pending that harness (spec-ospf-ext-16)."
    )


if __name__ == "__main__":
    check()
