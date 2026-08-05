#!/usr/bin/env python3
"""Flood one IS-IS Level-1 LSP purge onto a link, for an LSP ID we choose.

This stands in for a third router that has (or believes it has) a NEWER view of
another system's LSP than that system does. It exists because no vtysh command
makes FRR purge a peer's LSP on demand, and because ze itself must never be the
tool that proves its own conformance.

The purge is a real IS-IS PDU: an ISO/IEC 10589 clause 9.8 Link State PDU with
Remaining Lifetime 0 and no TLVs, framed per IEEE 802.3 + LLC (DSAP/SSAP 0xFE,
control 0x03) to AllL1ISs (01:80:c2:00:00:14). Both the ISO 8473 Fletcher
checksum and the 802.3 length field are computed here, so a receiver that
validates either one (ze validates both) accepts the frame.

Usage:
    purge.py <interface> <system-id> <sequence> [pseudonode] [lsp-number]

    system-id  six octets as xxxx.xxxx.xxxx
    sequence   the sequence number to claim, decimal

Everything is stdlib: the container images carry no scapy.
"""

import socket
import sys

# ISO/IEC 10589: IS-IS runs directly over IEEE 802.3 with an LLC header, never
# over an Ethernet II ethertype and never over IP.
LLC = bytes([0xFE, 0xFE, 0x03])
ALL_L1_ISS = bytes([0x01, 0x80, 0xC2, 0x00, 0x00, 0x14])

# Common header (clause 9.5) and Level-1 LSP (clause 9.8).
PROTOCOL_DISCRIMINATOR = 0x83
VERSION_PROTOCOL_ID_EXTENSION = 0x01
VERSION = 0x01
ID_LENGTH = 6
PDU_TYPE_L1_LSP = 0x12
COMMON_HEADER_LEN = 8
# PDU length (2) + Remaining Lifetime (2) + LSP ID (8) + Sequence (4)
# + Checksum (2) + type block (1).
LSP_FIXED_LEN = 19
# IS-type bits for Level 1 in the type block (clause 9.8).
TYPE_BLOCK_L1 = 0x01

FLETCHER_MODULUS = 255
# Offset of the checksum field inside the checksummed region: the region starts
# at the LSP ID (the octet after Remaining Lifetime, clause 7.3.11) and the
# checksum follows LSP ID (8) + Sequence Number (4).
CHECKSUM_OFFSET_IN_REGION = 12


def fletcher(region, check_off):
    """Return the two ISO 8473 checksum octets for region.

    The checksum field participates in its own computation, so the two octets
    cannot simply be the running sums: ISO 8473 annex C.3.4.2 gives the closed
    form used here. A computed 0 is stored as 255, which is congruent under the
    mod-255 arithmetic and keeps the field from colliding with the reserved
    all-zero "checksum not computed" value.
    """
    c0 = c1 = 0
    for i, octet in enumerate(region):
        b = 0 if i in (check_off, check_off + 1) else octet
        c0 = (c0 + b) % FLETCHER_MODULUS
        c1 = (c1 + c0) % FLETCHER_MODULUS
    m = len(region) - check_off
    x = ((m - 1) * c0 - c1) % FLETCHER_MODULUS
    y = (c1 - m * c0) % FLETCHER_MODULUS
    return (x or FLETCHER_MODULUS), (y or FLETCHER_MODULUS)


def parse_system_id(text):
    """Parse xxxx.xxxx.xxxx into six octets."""
    groups = text.split(".")
    if len(groups) != 3 or any(len(g) != 4 for g in groups):
        raise SystemExit("system-id must be xxxx.xxxx.xxxx, got %r" % text)
    return bytes.fromhex("".join(groups))


def build_purge(system_id, sequence, pseudonode, lsp_number):
    """Build the Level-1 LSP purge PDU (clause 9.8, Remaining Lifetime 0)."""
    total = COMMON_HEADER_LEN + LSP_FIXED_LEN
    header = bytes(
        [
            PROTOCOL_DISCRIMINATOR,
            total,  # length indicator: the fixed header for this PDU type
            VERSION_PROTOCOL_ID_EXTENSION,
            ID_LENGTH,
            PDU_TYPE_L1_LSP,
            VERSION,
            0x00,  # reserved
            0x00,  # maximum area addresses (0 means the default of 3)
        ]
    )
    lsp_id = system_id + bytes([pseudonode, lsp_number])
    # The checksummed region: LSP ID .. end of PDU, with the checksum zeroed.
    region = bytearray(lsp_id)
    region += sequence.to_bytes(4, "big")
    region += b"\x00\x00"  # checksum, filled in below
    region += bytes([TYPE_BLOCK_L1])
    high, low = fletcher(region, CHECKSUM_OFFSET_IN_REGION)
    region[CHECKSUM_OFFSET_IN_REGION] = high
    region[CHECKSUM_OFFSET_IN_REGION + 1] = low

    body = total.to_bytes(2, "big") + b"\x00\x00" + bytes(region)  # lifetime 0 = purge
    return header + body


def main():
    if len(sys.argv) < 4:
        raise SystemExit(__doc__)
    interface = sys.argv[1]
    system_id = parse_system_id(sys.argv[2])
    sequence = int(sys.argv[3])
    pseudonode = int(sys.argv[4]) if len(sys.argv) > 4 else 0
    lsp_number = int(sys.argv[5]) if len(sys.argv) > 5 else 0

    pdu = build_purge(system_id, sequence, pseudonode, lsp_number)

    sock = socket.socket(socket.AF_PACKET, socket.SOCK_RAW)
    sock.bind((interface, 0))
    src_mac = sock.getsockname()[4]
    # The 802.3 length field carries LLC + PDU, and MUST stay below 0x0600 or a
    # receiver reads it as an Ethernet II ethertype.
    llc_and_pdu = len(LLC) + len(pdu)
    frame = ALL_L1_ISS + src_mac + llc_and_pdu.to_bytes(2, "big") + LLC + pdu
    # A short frame is padded by the driver; IS-IS carries its own PDU length so
    # trailing padding is ignored by the receiver.
    sock.send(frame)
    sock.close()
    print(
        "sent L1 purge for %s.%02x-%02x at sequence %d on %s (%d octets)"
        % (sys.argv[2], pseudonode, lsp_number, sequence, interface, len(frame))
    )
    # The PDU alone, so a caller can feed it to `ze isis decode` to check it.
    print("pdu %s" % pdu.hex())


if __name__ == "__main__":
    main()
