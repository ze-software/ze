#!/usr/bin/env python3
"""RTR origin validation: Ze as the client of a real StayRTR cache.

Validates: RFC 8210 Prefix PDU decoding and RFC 6811 Section 2 validation
           outcomes, against VRPs a third-party cache encoded. Valid, Invalid
           by wrong origin, Invalid by max-length, and NotFound, over IPv4 and
           IPv6, with a 4-byte and a 2-byte origin AS.
Prevents:  the state RTR was in before this scenario existed -- Ze was the
           client in production and Ze was also the SERVER in every test that
           exercised the protocol (`ze-test rpki` in 43-rpki-frr, `ze-test
           rtr-mock` in 25 functional tests). A codec that round-trips against
           itself agrees with itself by construction.

WHY THE ANSWERS AND NOT THE SESSION. A cache-side encoding disagreement FAILS
OPEN: a Prefix PDU Ze mis-decodes produces a VRP that covers nothing an operator
announces, every prefix then reads NotFound, `not-found accept` accepts it, and
the router keeps forwarding while validating nothing. The session is up
throughout, its serial advances, and its VRP count is non-zero, so every
session-level assertion passes while origin validation protects nobody. Only the
per-prefix ANSWER can tell that apart, which is why nothing here stops at
"connected and received something".

The inventory assertion reads StayRTR's OWN /rpki.json export, not this
scenario's vrps.json. The question is whether the VRPs Ze built from the RTR
wire agree with what the CACHE meant, and the input file is Ze's co-witness only
if the cache serves it unchanged -- which is a claim about StayRTR, not about
Ze. Reading it back from StayRTR removes that assumption.

Every expectation below fails when the cache is empty, so the headline failure
mode is discriminated by construction: an empty Ze cache answers NotFound to all
ten queries, and eight of them want something else.

There is deliberately no `RFC requirement:` tag here. An interop-tier tag is a
permanent commitment under check_evidence_ratchet, which is a decision for
whoever owns that ledger and not a side effect of writing a scenario.
"""

import ipaddress
import json
import os
import sys

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from interop import (  # noqa: E402
    STAYRTR_CONTAINER,
    STAYRTR_HTTP_PORT,
    Ze,
    docker_exec,
    log_info,
    log_pass,
    poll,
)

# StayRTR loads its cache file at startup and Ze dials on its own retry
# schedule, so the first RESET QUERY can land before either is ready.
SYNC_TIMEOUT_S = 90

# The VRPs in vrps.json, restated as the count Ze must hold. Stated here rather
# than derived from the file so a truncated sync cannot satisfy the wait by
# comparing an empty set against an empty set.
WANT_VRP_IPV4 = 2
WANT_VRP_IPV6 = 2

# prefix, origin AS, expected RFC 6811 state, and what a wrong answer would mean.
#
# The 4-byte origin (4200000001, 0xFA56EA01) is the one that moves under a
# byte-order or field-width fault in the ASN read; the 2-byte one (65001) still
# decodes correctly under a low-half-only read, so keeping both separates "the
# ASN field is misread" from "the ASN field is ignored".
EXPECTATIONS = [
    ("9.58.0.0/16", 4200000001, "valid", "exact VRP, 4-byte origin AS"),
    ("9.58.0.0/24", 4200000001, "valid", "more specific at max-length 24"),
    ("9.58.0.0/25", 4200000001, "invalid", "one bit past max-length 24"),
    ("9.58.0.0/16", 65001, "invalid", "covered, origin belongs to another VRP"),
    ("10.58.0.0/16", 65001, "valid", "exact VRP, 2-byte origin AS"),
    ("10.58.0.0/24", 65001, "invalid", "max-length equals prefix length"),
    ("2001:db8:58::/48", 4200000001, "valid", "IPv6 Prefix PDU, exact VRP"),
    ("2001:db8:58::/64", 4200000001, "invalid", "IPv6, past max-length 56"),
    ("11.58.0.0/16", 65001, "not-found", "IPv4 covered by no VRP"),
    ("2001:db8:5a::/48", 65001, "not-found", "IPv6 covered by no VRP"),
]


def _canonical(prefix, max_length, asn):
    """One spelling for a VRP, so two implementations' JSON can be compared.

    StayRTR writes `2001:db8:58::/48` and `AS4200000001`; Ze writes Go's
    `net.IPNet.String()` and a bare integer. `ip_network` normalises the first
    half and int() the second, so a mismatch reported below is a mismatch in the
    DATA rather than in either side's formatting.
    """
    return (str(ipaddress.ip_network(prefix)), int(max_length), int(asn))


def _stayrtr_inventory():
    """The VRP set StayRTR holds, read from StayRTR itself over its own export."""
    raw = docker_exec(
        STAYRTR_CONTAINER,
        ["wget", "-q", "-O", "-", "http://127.0.0.1:%d/rpki.json" % STAYRTR_HTTP_PORT],
    )
    doc = json.loads(raw)
    found = set()
    for roa in doc.get("roas", []):
        asn = str(roa["asn"])
        if asn.upper().startswith("AS"):
            asn = asn[2:]
        found.add(_canonical(roa["prefix"], roa["maxLength"], asn))
    return found


def _ze_inventory(ze):
    """The VRP set Ze built from the RTR wire."""
    doc = ze.cli_json("show bgp rpki roa", want=dict)
    return {
        _canonical(e["prefix"], e["max-length"], e["asn"])
        for e in doc.get("entries", [])
    }


def _wait_synced(ze):
    """Wait until Ze holds the whole set, and say which session delivered it.

    THE COUNT IS THE SESSION EVIDENCE, not the reported state. Ze commits a
    sync's VRPs only when the End of Data PDU arrives (handlePDU,
    internal/component/bgp/plugins/rpki/rtr_session.go), and it closes the
    connection there and re-polls after the retry interval, so `state` reads
    `idle` for all but the few milliseconds of a poll. Asserting `establish`
    would be asserting that the query landed inside that window. A non-zero VRP
    count cannot be reached any other way: it means a Cache Response, the Prefix
    PDUs, and an End of Data all arrived from StayRTR and parsed.

    Waiting for `sessions >= 1` alone would proceed against a half-loaded cache
    and blame Ze for VRPs that had not arrived yet, so the barrier is the whole
    set.
    """

    def ready(doc):
        return (
            doc.get("vrp-count-ipv4", 0) >= WANT_VRP_IPV4
            and doc.get("vrp-count-ipv6", 0) >= WANT_VRP_IPV6
        )

    status = poll(
        lambda: ze.cli_json("show bgp rpki status", want=dict),
        ready,
        SYNC_TIMEOUT_S,
        interval=2,
        what="show bgp rpki status",
    )
    if not ready(status):
        raise AssertionError(
            "ze did not load the StayRTR VRP set within %ds: want >= %d IPv4 and "
            ">= %d IPv6, got %r"
            % (
                SYNC_TIMEOUT_S,
                WANT_VRP_IPV4,
                WANT_VRP_IPV6,
                status,
            )
        )
    servers = status.get("cache-servers", [])
    if not servers:
        raise AssertionError("ze reports no RTR cache server at all: %r" % status)
    log_pass(
        "synced from StayRTR at %s:%s, RTR version %s (StayRTR offers v1 by "
        "default and ze asks for v2, so this reads the negotiation), %d IPv4 "
        "and %d IPv6 VRPs"
        % (
            servers[0].get("address"),
            servers[0].get("port"),
            servers[0].get("version"),
            status.get("vrp-count-ipv4", 0),
            status.get("vrp-count-ipv6", 0),
        )
    )


def _inventory_diff(ze):
    """What StayRTR holds and Ze does not, and the other way round."""
    cache = _stayrtr_inventory()
    mine = _ze_inventory(ze)
    if not cache:
        raise AssertionError(
            "StayRTR served an empty VRP set, so nothing about ze was measured"
        )
    return sorted(cache - mine), sorted(mine - cache), len(cache)


def _validation_failures(ze):
    """Every expectation, then one verdict, so a failure names the whole pattern.

    Reported together on purpose: "all ten read not-found" is the fail-open
    signature and "one read invalid" is a max-length or origin fault, and a
    check that raised on the first mismatch would show the same one line for
    both.
    """
    wrong = []
    for prefix, origin, want, why in EXPECTATIONS:
        answer = ze.cli_json(
            "request bgp rpki validate %s %d" % (prefix, origin), want=dict
        )
        got = answer.get("state")
        if got != want:
            wrong.append((prefix, origin, want, got, why, answer.get("covering-vrps")))
        else:
            log_info("%s origin %d: %s (%s)" % (prefix, origin, got, why))
    return wrong


def check():
    """The validation answers are the verdict; the VRP diff explains them.

    The order is deliberate. A decode fault shows up in BOTH readings, and the
    one an operator lives with is the answer: a router that reads NotFound for
    an address space a ROA covers accepts a hijack. So the answers are judged
    first and the inventory difference is attached to that failure as its cause,
    rather than pre-empting it with a message about wire encoding.
    """
    ze = Ze()
    _wait_synced(ze)

    wrong = _validation_failures(ze)
    only_cache, only_ze, cache_size = _inventory_diff(ze)

    if wrong:
        lines = [
            "  %s origin %d: want %s, got %s (%s); covering VRPs: %s"
            % (prefix, origin, want, got, why, covering)
            for prefix, origin, want, got, why, covering in wrong
        ]
        raise AssertionError(
            "ze answered %d of %d origin validations wrongly against VRPs it "
            "decoded from StayRTR:\n%s\nVRPs only StayRTR holds: %s\nVRPs only "
            "ze holds: %s"
            % (
                len(wrong),
                len(EXPECTATIONS),
                "\n".join(lines),
                only_cache,
                only_ze,
            )
        )
    log_pass("all %d origin validations agree with StayRTR's VRPs" % len(EXPECTATIONS))

    if only_cache or only_ze:
        raise AssertionError(
            "the VRPs ze built from the RTR wire disagree with the ones StayRTR "
            "holds.\n  only StayRTR: %s\n  only ze:      %s" % (only_cache, only_ze)
        )
    log_pass("%d VRPs decoded from StayRTR's wire, all matching" % cache_size)
