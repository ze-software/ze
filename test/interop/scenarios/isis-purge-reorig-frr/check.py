#!/usr/bin/env python3
"""Scenario isis-purge-reorig-frr: Ze re-originates above a purge of its own LSP.

Validates ISO/IEC 10589 clause 7.3.16.4 c) against a real IS-IS peer. A purge of
Ze's own LSP is flooded onto the segment at sequence 4096 -- a value Ze has never
issued and cannot reach in this run. Ze must refuse to overwrite its own LSP with
it, move its sequence ABOVE the received one, and re-originate. FRR is the
witness: it must end up holding Ze's LSP at a sequence greater than 4096 with a
live holdtime, and must still see Ze in its Level-1 topology.

Prevents: the defect where the originator computed the next sequence from its own
private counter alone and never consulted what the network claimed. A purge above
that counter then won permanently and Ze stayed withdrawn from every peer's
database. Under that defect FRR ends this scenario holding either a zero-holdtime
purge of Ze's LSP or nothing at all, and Ze's LSP never passes sequence 4096.

Runs under the Linux Docker/QEMU interop harness ONLY (raw L2 + FRR isisd).
"""

import os
import re
import sys
import time

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))

from interop import (  # noqa: E402
    ZE_CONTAINER,
    FRRISIS,
    docker_exec,
    log_info,
    log_pass,
)

# Ze's System ID, from ze.conf's NET (49.0001.0000.0000.0000.0002.00).
ZE_SYSTEM_ID = "0000.0000.0002"
# Ze's own non-pseudonode fragment 0, as FRR prints it when dynamic-hostname
# resolution is unavailable.
ZE_LSP_ID = ZE_SYSTEM_ID + ".00-00"
# The hostname Ze advertises in TLV 137; FRR prints this instead of the raw LSP
# ID once it has resolved it.
ZE_HOSTNAME = "ze-purge"
ZE_LSP_NAMES = (ZE_LSP_ID, ZE_HOSTNAME + ".00-00")

# The sequence the injected purge claims. Far above anything Ze reaches in a run
# of this length (Ze starts at 1 and increments once per re-origination), so a
# sequence beyond it can ONLY come from Ze answering the claim.
CLAIMED_SEQUENCE = 4096

# An 8-hex-digit 0x token is the SeqNumber column; the Chksum column is 4 digits,
# so the width disambiguates them without depending on column order.
SEQ_TOKEN = re.compile(r"^0x[0-9a-fA-F]{8}$")

# The PDU length of a header-only LSP: the 8-octet common header plus the 19
# fixed LSP fields. A purge carries exactly this and no TLVs (RFC 5304 sec 2
# strips the body), so anything longer is a real advertisement.
PURGE_PDU_LEN = 27


def ze_lsp_row(frr):
    """Return (sequence, holdtime, pdulen) for Ze's own LSP in FRR's LSDB.

    None when FRR holds no row for it. FRR prints one line per LSP:
        LSP ID          PduLen  SeqNumber   Chksum  Holdtime  ATT/P/OL
        ze-purge.00-00      83  0x00000003  0xc530      1176     0/0/0
    """
    out = frr._vtysh_quiet("show isis database")
    for line in out.splitlines():
        tokens = line.split()
        if not tokens or tokens[0] not in ZE_LSP_NAMES:
            continue
        seq = None
        seq_index = None
        for i, token in enumerate(tokens):
            if SEQ_TOKEN.match(token):
                seq = int(token, 16)
                seq_index = i
                break
        if seq is None or seq_index < 1 or seq_index + 2 >= len(tokens):
            continue
        # PduLen is the column before SeqNumber; a purge is header-only.
        pdulen = tokens[seq_index - 1]
        if not pdulen.isdigit():
            continue
        # Holdtime is the column after Chksum, which is the column after
        # SeqNumber. FRR 10.3.1 prints a live holdtime as a bare number and a
        # zero-lifetime LSP it is ageing out as "(N)" (isisd formats the two
        # branches with "%5hu" and "%7s"). Both must be readable here: reporting
        # the purged row as holdtime 0 is what makes a failure say "seq 4096,
        # holdtime 0" instead of "no row found".
        holdtime = tokens[seq_index + 2]
        if holdtime.isdigit():
            return seq, int(holdtime), int(pdulen)
        if holdtime.startswith("("):
            return seq, 0, int(pdulen)
        continue
    return None


def wait_for(timeout, sample, predicate):
    """Poll sample() until predicate() holds; return the sample, or None."""
    deadline = time.time() + timeout
    while time.time() < deadline:
        out = sample()
        if predicate(out):
            return out
        time.sleep(1)
    return None


def wait_ze_lsp(frr, timeout, predicate, what):
    """Poll FRR's LSDB until Ze's LSP row satisfies predicate; return the row."""
    deadline = time.time() + timeout
    row = None
    while time.time() < deadline:
        row = ze_lsp_row(frr)
        if row is not None and predicate(row):
            return row
        time.sleep(1)
    print(frr._vtysh_quiet("show isis database")[:1200])
    raise AssertionError(
        "FRR never saw Ze's LSP %s (last row seq/holdtime = %s)" % (what, row)
    )


def check():
    frr = FRRISIS()

    # 1. The LAN adjacency comes up and FRR learns Ze's own LSP.
    log_info("waiting for the Level-1 adjacency and Ze's LSP in FRR's LSDB...")
    frr.wait_adjacency(timeout=90)
    before = wait_ze_lsp(
        frr,
        60,
        lambda row: row[1] > 0 and row[2] > PURGE_PDU_LEN,
        "with a live holdtime and a body before the purge",
    )
    log_info("FRR holds Ze's LSP at sequence %d, holdtime %d, PduLen %d" % before)
    if before[0] >= CLAIMED_SEQUENCE:
        raise AssertionError(
            "scenario premise broken: Ze already reached sequence %d, "
            "so a claim at %d proves nothing" % (before[0], CLAIMED_SEQUENCE)
        )

    # 2. Flood a purge of Ze's own LSP at a sequence Ze never issued. purge.py is
    #    mounted into the Ze container by the harness (every scenario .py except
    #    check.py is). docker_exec raises if the injector fails, so a broken
    #    injection is never mistaken for a protocol result.
    log_info("flooding a purge of Ze's own LSP at sequence %d..." % CLAIMED_SEQUENCE)
    out = docker_exec(
        ZE_CONTAINER,
        ["python3", "/etc/ze/purge.py", "eth0", ZE_SYSTEM_ID, str(CLAIMED_SEQUENCE)],
    )
    log_info(out.strip())

    # 3. THE ASSERTION. Ze must answer the claim by re-originating above it, and
    #    FRR must accept the regenerated LSP: a sequence strictly greater than the
    #    purge, and a live holdtime (not a retained zero-age purge).
    #    ISO/IEC 10589 clause 7.3.16.1: "it shall change its sequence number to be
    #    the next number greater than the new one received, and shall generate a
    #    link state PDU".
    #    PduLen is asserted too: a purge is header-only (PURGE_PDU_LEN), so a body
    #    proves FRR installed Ze's real advertisement and not another purge.
    after = wait_ze_lsp(
        frr,
        60,
        lambda row: (row[0] > CLAIMED_SEQUENCE and row[1] > 0
                     and row[2] > PURGE_PDU_LEN),
        "re-originated above the purged sequence %d" % CLAIMED_SEQUENCE,
    )
    log_info("FRR now holds Ze's LSP at sequence %d, holdtime %d, PduLen %d" % after)

    # 4. FRR must USE it, not merely store it: Ze stays in FRR's Level-1 topology,
    #    which is FRR's SPF reading the regenerated LSP.
    # POLLED, because FRR runs SPF on its own timer: the LSDB row appears first
    # and the topology follows a second or two later. Sampling once here read the
    # tree from before FRR had re-run SPF.
    #
    # Match the WHOLE identifier. A prefix would not identify Ze: the first
    # dotted group of the System ID is "0000", which every LSP ID in this area
    # starts with, so the assertion would hold with Ze absent.
    topology = wait_for(
        60,
        lambda: frr._vtysh_quiet("show isis topology level-1"),
        lambda out: ZE_HOSTNAME in out or ZE_SYSTEM_ID in out,
    )
    if topology is None:
        print(frr._vtysh_quiet("show isis topology level-1")[:1200])
        raise AssertionError("Ze is absent from FRR's Level-1 topology after the purge")

    # 5. The adjacency must not have flapped: answering a purge is an origination,
    #    not a session event.
    time.sleep(5)
    if not frr.adjacency_up():
        raise AssertionError("the IS-IS adjacency dropped while answering the purge")

    log_pass(
        "isis-purge-reorig-frr: purge at %d answered, FRR installed Ze's LSP at %d "
        "(%d octets, holdtime %d) and kept Ze in its topology"
        % (CLAIMED_SEQUENCE, after[0], after[2], after[1])
    )


if __name__ == "__main__":
    check()
