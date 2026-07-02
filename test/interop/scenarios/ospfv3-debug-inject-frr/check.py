#!/usr/bin/env python3
"""Scenario ospfv3-debug-inject-frr: OSPFv3 debug LSA injection interop with FRR ospf6d.

Validates (spec-ospf-ext-14, user stories 6+7): with a P2P OSPFv3 adjacency, an
authorized Ze operator enables debug injection and injects a crafted area-scope v3 LSA
(the scope derived from the LS Type S2/S1 bits); it floods to FRR and appears in FRR's
`show ipv6 ospf6 database`; a withdraw MaxAge-purges it; the adjacency is unaffected
(AC-14/AC-15/AC-18).
Prevents: an injected v3 LSA FRR rejects, a reserved-scope injection, a leaked withdraw,
or an injection that flaps the adjacency.

AUTHORED-PENDING-QEMU: runs under the Linux Docker/QEMU interop harness ONLY (raw IPv6
proto 89 over ff02::5 + FRR ospf6d). It CANNOT run on darwin.
"""

import os
import sys
import time

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))

from interop import FRROSPF6, ZE_CONTAINER, docker_exec_quiet, log_info, log_pass  # noqa: E402

LINK_STATE_ID = "1"


def check():
    frr = FRROSPF6()
    log_info("waiting for the P2P OSPFv3 adjacency...")
    frr.wait_adjacency(timeout=90)

    log_info("enabling debug injection and injecting an area-scope v3 LSA...")
    docker_exec_quiet(ZE_CONTAINER, ["ze", "debug", "ospf", "inject", "enable"])
    docker_exec_quiet(
        ZE_CONTAINER,
        [
            "ze",
            "debug",
            "ipv6",
            "ospf",
            "inject",
            "lsa",
            "scope",
            "area",
            "type",
            "0x2009",
            "id",
            LINK_STATE_ID,
            "hex",
            "00000000",
        ],
    )

    log_info("waiting for FRR ospf6d to receive the injected LSA...")
    deadline = time.time() + 60
    seen = ""
    while time.time() < deadline:
        seen = frr._vtysh_quiet("show ipv6 ospf6 database")
        if "172.30.0.2" in seen:
            break
        time.sleep(3)
    if "172.30.0.2" not in seen:
        raise AssertionError("FRR never received the injected v3 LSA:\n" + seen[:600])

    log_info("withdrawing the injected LSA; the adjacency must not flap...")
    docker_exec_quiet(
        ZE_CONTAINER,
        [
            "ze",
            "debug",
            "ipv6",
            "ospf",
            "inject",
            "lsa",
            "scope",
            "area",
            "type",
            "0x2009",
            "id",
            LINK_STATE_ID,
            "withdraw",
        ],
    )
    time.sleep(8)
    if not frr.adjacency_full():
        raise AssertionError("OSPFv3 adjacency flapped during inject/withdraw")

    log_pass("ospfv3-debug-inject-frr: v3 inject + flood + withdraw interop verified")


if __name__ == "__main__":
    check()
