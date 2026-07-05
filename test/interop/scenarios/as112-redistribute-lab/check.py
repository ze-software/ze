#!/usr/bin/env python3
"""Scenario: AS112 anycast node lab -- spec-as112-3 AC-13.

Validates the full node end to end, asserting BOTH facets the AS112 lab is meant
to prove together:

  (a) DNS: the REAL as112 server answers an authoritative SOA query for the
      Direct-Delegation reverse zone 10.in-addr.arpa (RFC 7534 Section 2.2) with
      NOERROR and the zone SOA whose MNAME is prisoner.iana.org. The query is a
      stdlib-only DNS packet sent to 127.0.0.1:53 from INSIDE the ze container
      (the anycast/loopback listeners bind on lo, not on the eth0 wire, so the
      probe must run on-box). This mirrors the plugin's own healthcheck query
      (internal/plugins/as112/health.go: SOA 10.in-addr.arpa on 127.0.0.1:53).

  (b) BGP: with watchdog true (RFC 7534 Section 3.3 default) the redistribute
      producer announces the AS112 COVERING prefixes only while the node serves;
      the eBGP FRR peer observes them with AS_PATH [65001, 112] (ze prepends its
      local AS ahead of the producer's origin AS 112). Only the /24 covering
      prefixes appear, never the /32 host addresses (finding H3).
"""

import os
import sys
import time

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from interop import FRR, ZE_CONTAINER, ZE_IP, docker_exec_quiet, log_fail, log_info, log_pass

PREFIX1 = "192.175.48.0/24"
PREFIX2 = "192.31.196.0/24"
HOST1 = "192.175.48.1/32"
HOST2 = "192.31.196.1/32"
ZE_REAL_AS = "65001"
AS112 = "112"
DNS_ZONE = "10.in-addr.arpa"

# Stdlib-only DNS SOA probe, run inside the ze container (Alpine python3, no
# dnspython/dig available). Sends a SOA query for 10.in-addr.arpa to 127.0.0.1:53
# and prints DNS_OK iff the reply is a NOERROR authoritative answer carrying the
# zone SOA (MNAME prisoner.iana.org, whose label bytes appear literally on the
# wire). Exit code mirrors the print so a non-DNS_OK reply yields empty stdout
# through docker_exec_quiet.
DNS_PROBE = r'''
import socket, struct, sys

def build_query(qname, qtype):
    header = struct.pack(">HHHHHH", 0x1234, 0x0000, 1, 0, 0, 0)
    body = b""
    for label in qname.split("."):
        if label:
            body += bytes([len(label)]) + label.encode("ascii")
    body += b"\x00"
    body += struct.pack(">HH", qtype, 1)  # QTYPE, QCLASS=IN
    return header + body

def main():
    query = build_query("10.in-addr.arpa", 6)  # QTYPE 6 = SOA
    s = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
    s.settimeout(3.0)
    try:
        s.sendto(query, ("127.0.0.1", 53))
        resp, _ = s.recvfrom(4096)
    except Exception as e:
        print("DNS_FAIL exc=%s" % e)
        sys.exit(1)
    if len(resp) < 12:
        print("DNS_FAIL short")
        sys.exit(1)
    _tid, flags, _qd, an, ns, _ar = struct.unpack(">HHHHHH", resp[:12])
    qr = (flags >> 15) & 1
    aa = (flags >> 10) & 1
    rcode = flags & 0x000F
    if qr == 1 and rcode == 0 and an >= 1 and b"prisoner" in resp:
        print("DNS_OK aa=%d an=%d ns=%d" % (aa, an, ns))
        sys.exit(0)
    print("DNS_FAIL qr=%d rcode=%d an=%d ns=%d" % (qr, rcode, an, ns))
    sys.exit(1)

main()
'''


def _frr_aspath(frr, prefix):
    """Return FRR's AS_PATH string for prefix (JSON), or "" if unavailable."""
    data = frr.route(prefix)
    paths = data.get("paths", [])
    if not paths:
        return ""
    aspath = paths[0].get("aspath", {})
    if isinstance(aspath, dict):
        return aspath.get("string", "")
    if isinstance(aspath, str):
        return aspath
    return ""


def _wait_dns_soa(timeout=30):
    """Poll the on-box as112 DNS server until it answers the SOA probe."""
    log_info("waiting for as112 authoritative SOA answer on 127.0.0.1:53...")
    deadline = time.time() + timeout
    while time.time() < deadline:
        out = docker_exec_quiet(ZE_CONTAINER, ["python3", "-c", DNS_PROBE])
        if "DNS_OK" in out:
            return out.strip()
        time.sleep(2)
    return None


def check():
    # (a) DNS facet: the node serves authoritatively. With watchdog true this is
    # also the precondition that opens the BGP announcement, so assert it first.
    result = _wait_dns_soa()
    if result is None:
        log_fail(
            "as112 did not answer an authoritative SOA for %s on 127.0.0.1:53"
            % DNS_ZONE
        )
        raise AssertionError("as112 authoritative DNS answer missing")
    log_pass(
        "as112 answers authoritative SOA for %s (prisoner.iana.org) [%s]"
        % (DNS_ZONE, result)
    )

    # (b) BGP facet: while serving, the covering prefixes are announced to the
    # eBGP peer with AS_PATH [65001, 112].
    frr = FRR()
    frr.wait_session(ZE_IP)
    frr.wait_route(PREFIX1)

    frr_aspath = _frr_aspath(frr, PREFIX1)
    if frr_aspath.split() != [ZE_REAL_AS, AS112]:
        log_fail(
            "FRR (eBGP) AS_PATH for %s = %r, expected exactly [%s %s]"
            % (PREFIX1, frr_aspath, ZE_REAL_AS, AS112)
        )
        raise AssertionError("AS112 covering prefix not announced with expected AS_PATH")
    log_pass("FRR (eBGP) observes covering prefix with AS_PATH [65001, 112]")

    frr.check_route(PREFIX2)

    # Finding H3: only the /24 covering prefixes, never the /32 host addresses.
    frr.route_absent(HOST1)
    frr.route_absent(HOST2)

    assert frr.session_established(ZE_IP), "FRR session dropped"
    log_pass("AS112 node lab: DNS serving and BGP origination interoperate (AC-13)")
