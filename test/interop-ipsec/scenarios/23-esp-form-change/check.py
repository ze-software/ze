#!/usr/bin/env python3
"""Scenario 23: one live Child SA carries an ESP form its kernel state refuses.

Validates: AC-4 of plan/spec-ipsec-esp-dual-form-receive.md, against a real peer.
           strongSwan floats IKE to port 4500 without finding a NAT, so Ze gives its
           INBOUND XFRM state an ESP-in-UDP template; strongSwan keeps sending BARE
           ESP, because its own NAT condition stays false. The two forms therefore
           disagree for the whole life of this Child SA. Linux XFRM refuses every one
           of those datagrams (XfrmInStateMismatch), and Ze's userspace receiver
           (internal/component/ike/dataplane/espform_linux.go) must recover each one
           and re-present it, so traffic flows in BOTH directions on a Child SA that
           is neither rekeyed nor deleted. RFC 7296 Section 2.23 requires both forms
           to be received "at any time".
Prevents:  the regression this scenario was written to catch, which was live. Ze's
           own inbound Child SA policy made the kernel drop the refused datagram
           before the receiver could read it: net/ipv4/raw.c raw_rcv runs
           xfrm4_policy_check, and a policy demanding ESP rejects a datagram that
           arrived outside an SA. /proc/net/raw showed the protocol-50 socket's drop
           counter rising once per datagram, Ze's inbound SA stayed at 0 packets, and
           every ping failed. No unit or integration test could see it: a loopback
           dst entry carries DST_NOPOLICY, so the policy check the daemon trips is
           skipped there. Only a peer on a real interface reaches it.

Why no RFC requirement tag: test/interop-ipsec is declared TIER_UNRUN
(scripts/dev/rfc_requirements.py), so a tag here would be refused. The compliance
evidence for RFC7296-2.23-10 and -2.23-11 stays at unit tier, in
internal/component/ike/engine/rfc7296_natt_bothforms_test.go.

What this scenario does NOT prove: a peer CHANGING form part way through the SA.
strongSwan changes the form of a live SA only through MOBIKE
(kernel_netlink_ipsec.c update_sa, driven by ike_mobike.c), and Ze never advertises
MOBIKE_SUPPORTED -- internal/component/ike/wire/payload_notify.go defines no notify
type 16396, and plan/spec-ipsec-11-mobike.md is still at design. So no trigger for a
mid-SA change exists in this lab. What is proven is the property the change would
exercise: a live SA serving the form its kernel state does not accept.
"""

import os
import re
import sys
import time

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from lab import (
    StrongSwan,
    SWAN_CONTAINER,
    SWAN_IP,
    ZE_CONTAINER,
    ZE_IP,
    assert_esp_accepted,
    docker_exec,
    docker_exec_quiet,
    docker_logs_all,
    log_pass,
    xfrm_sa_bytes_by_spi,
    ze_xfrm_state,
)

# The protocol-50 raw socket the receiver reads. /proc/net/raw prints the local port
# column as the IP protocol number in hex, so ESP is 0032.
RAW_ESP_LINE = re.compile(r"^\s*\d+:\s+\S+:0032\s")


def esp_spis():
    """The ESP SPIs currently installed in Ze's XFRM state."""
    return set(re.findall(r"proto esp spi (0x[0-9a-fA-F]+)", ze_xfrm_state()))


def xfrm_stat(container, name):
    for line in docker_exec(container, ["cat", "/proc/net/xfrm_stat"]).splitlines():
        f = line.split()
        if len(f) == 2 and f[0] == name:
            return int(f[1])
    raise AssertionError("%s absent from /proc/net/xfrm_stat in %s" % (name, container))


def raw_esp_drops():
    """Packets the kernel refused to queue to Ze's raw ESP socket.

    This is the counter the regression moved, so it is asserted by name rather than
    left to be inferred from a failed ping.
    """
    out = docker_exec(ZE_CONTAINER, ["cat", "/proc/net/raw"])
    for line in out.splitlines():
        if RAW_ESP_LINE.match(line):
            return int(line.split()[-1])
    raise AssertionError(
        "Ze holds no raw ESP socket; espFormReceiver.Watch never ran, so the SA "
        "receives one ESP wire form only:\n%s" % out
    )


def ping(container, target):
    """Ping and report the loss percentage, without raising on loss."""
    out = docker_exec_quiet(container, ["ping", "-c", "3", "-W", "2", target])
    m = re.search(r"(\d+)% packet loss", out)
    return (int(m.group(1)) if m else 100), out


def check():
    swan = StrongSwan()

    # 1. Control plane: strongSwan (initiator) establishes against Ze (responder).
    swan.wait_sa_established("ze")
    swan.wait_child_sa()
    log_pass("strongSwan established IKE + Child SA against Ze the responder")

    # 2. The two ESP forms must actually DISAGREE. Without this the scenario would
    #    silently degrade into the ordinary matching case and prove nothing, which is
    #    the vacuity trap in ai/rules/interop-and-goal-validation.md.
    ze_state = ze_xfrm_state()
    if not ze_state.strip():
        log_pass("XFRM/ESP unsupported on this host; nothing to measure")
        return
    if "encap type espinudp" not in ze_state:
        raise AssertionError(
            "Ze's inbound state carries no ESP-in-UDP template, so the peer's bare "
            "ESP is the form the kernel already accepts and this scenario is "
            "measuring nothing:\n%s" % ze_state
        )
    sas = swan.list_sas()
    if "TUNNEL-in-UDP" in sas:
        raise AssertionError(
            "strongSwan is sending UDP-encapsulated ESP, which is the form Ze's "
            "template already accepts; the forms agree and this scenario is "
            "measuring nothing:\n%s" % sas
        )
    log_pass(
        "forms disagree: Ze's inbound state is ESP-in-UDP, strongSwan sends bare ESP"
    )

    # 3. Ze must be holding the raw ESP socket, with nothing dropped on it yet.
    drops_before = raw_esp_drops()

    spis_before = esp_spis()
    mismatch_before = xfrm_stat(ZE_CONTAINER, "XfrmInStateMismatch")
    ze_before = xfrm_sa_bytes_by_spi(ZE_CONTAINER)
    peer_before = xfrm_sa_bytes_by_spi(SWAN_CONTAINER)

    # 4. Traffic in BOTH directions. The peer-to-Ze direction is the one that only
    #    works through the receiver, so it is asserted on its own rather than folded
    #    into a single "some counter moved" reading.
    loss_out, out_text = ping(ZE_CONTAINER, SWAN_IP)
    if loss_out != 0:
        raise AssertionError(
            "Ze -> strongSwan lost %d%% of its ESP traffic:\n%s" % (loss_out, out_text)
        )

    loss_in, in_text = ping(SWAN_CONTAINER, ZE_IP)
    if loss_in != 0:
        raise AssertionError(
            "strongSwan -> Ze lost %d%% of its ESP traffic. The peer sends the wire "
            "form Ze's kernel state refuses, so this is the direction the userspace "
            "receiver serves:\n%s" % (loss_in, in_text)
        )
    log_pass("traffic flows in both directions across the form disagreement")

    # 5. The recovery genuinely happened. XfrmInStateMismatch rising proves the kernel
    #    REFUSED the peer's form, and the inbound SA's byte counter rising proves the
    #    same datagrams reached the crypto check anyway. Either reading alone is
    #    satisfied by a tunnel where the forms agree.
    mismatch_after = xfrm_stat(ZE_CONTAINER, "XfrmInStateMismatch")
    if mismatch_after <= mismatch_before:
        raise AssertionError(
            "XfrmInStateMismatch did not rise (%d -> %d). The kernel accepted the "
            "peer's ESP directly, so the userspace receive path was never exercised "
            "and this scenario proves nothing" % (mismatch_before, mismatch_after)
        )
    assert_esp_accepted(
        ZE_CONTAINER,
        ze_before,
        xfrm_sa_bytes_by_spi(ZE_CONTAINER),
        "Ze accepted no ESP from strongSwan across the form disagreement",
    )
    assert_esp_accepted(
        SWAN_CONTAINER,
        peer_before,
        xfrm_sa_bytes_by_spi(SWAN_CONTAINER),
        "strongSwan accepted no ESP from Ze",
    )

    drops_after = raw_esp_drops()
    if drops_after > drops_before:
        raise AssertionError(
            "the kernel dropped %d packet(s) on Ze's raw ESP socket. Ze's own inbound "
            "IPsec policy is rejecting the datagrams its receiver exists to recover "
            "(net/ipv4/raw.c raw_rcv runs xfrm4_policy_check); the per-socket bypass "
            "in espFormReceiver.startLocked is missing or ineffective"
            % (drops_after - drops_before)
        )
    log_pass("the kernel refused the peer's form and Ze recovered every datagram")

    # 6. AC-4's second half: the SA is neither rekeyed nor deleted. The SPI set is the
    #    witness -- scenario 05 asserts the same set CHANGES on a rekey, so an
    #    unchanged set is a real reading and not an unfalsifiable absence.
    time.sleep(2)
    spis_after = esp_spis()
    if spis_after != spis_before:
        raise AssertionError(
            "the Child SA was rekeyed or replaced during the form disagreement "
            "(%s -> %s); AC-4 requires it to survive untouched"
            % (sorted(spis_before), sorted(spis_after))
        )

    swan_logs = docker_logs_all(SWAN_CONTAINER)
    if "received DELETE for ESP CHILD_SA" in swan_logs:
        raise AssertionError(
            "Ze deleted the Child SA rather than serving the peer's ESP form"
        )
    if "REKEY_SA" in swan_logs:
        raise AssertionError(
            "Ze rekeyed the Child SA rather than serving the peer's ESP form"
        )
    log_pass(
        "Child SA neither rekeyed nor deleted; SPIs %s unchanged throughout"
        % sorted(spis_after)
    )
