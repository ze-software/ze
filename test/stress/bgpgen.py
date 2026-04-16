#!/usr/bin/env python3
"""Fast BGP RAW update file generator (stdlib only).

Produces the same on-wire BGP UPDATE byte stream as RtBrick's scapy-based
`bgpupdate` helper from BNG Blaster but without scapy. The fixed parts of
every UPDATE (header + ORIGIN + AS_PATH + NEXT_HOP, or + MP_REACH shell)
are templated once; each message appends pre-packed NLRI bytes. For the
1M /24 scenario this reduces generation time from minutes to seconds.

Reference: RFC 4271 (BGP-4), RFC 4760 (MP-BGP), RFC 6793 (4-byte ASN).
Upstream scapy reference:
    https://github.com/rtbrick/bngblaster/blob/main/code/bgpupdate/bgpupdate

Output format matches the BNG Blaster `bgp-raw-update` reader, which
replays the file byte-for-byte over an established BGP session.
"""

import ipaddress
import socket

BGP_MAX_MSG_LEN = 4096
BGP_HEADER_LEN = 19
BGP_MARKER = b"\xff" * 16
_UPDATE_TYPE = b"\x02"
_WDR_LEN_ZERO = b"\x00\x00"

# End-of-RIB for IPv4 unicast: an empty UPDATE.
# marker(16) + len=23 + type=2 + wdr_len=0 + attr_len=0
_EOR_MSG = (
    BGP_MARKER + (23).to_bytes(2, "big") + _UPDATE_TYPE + _WDR_LEN_ZERO + b"\x00\x00"
)

# Path attribute flag constants (RFC 4271 §4.3).
_WK_TRANS = 0x40  # well-known, transitive
_OPT_NON_TRANS_EXT = 0x90  # optional, non-transitive, extended length


def _attrs_v4_unicast(asn, nexthop):
    """ORIGIN + AS_PATH + NEXT_HOP (RFC 4271 §5.1)."""
    origin = bytes((_WK_TRANS, 1, 1, 0))  # IGP
    as_path = bytes((_WK_TRANS, 2, 6, 2, 1)) + asn.to_bytes(
        4, "big"
    )  # AS_SEQUENCE, one 4-byte ASN
    next_hop = bytes((_WK_TRANS, 3, 4)) + socket.inet_aton(nexthop)
    return origin + as_path + next_hop


def _attrs_v6_prefix(asn):
    """ORIGIN + AS_PATH (NEXT_HOP lives inside MP_REACH for IPv6)."""
    origin = bytes((_WK_TRANS, 1, 1, 0))
    as_path = bytes((_WK_TRANS, 2, 6, 2, 1)) + asn.to_bytes(4, "big")
    return origin + as_path


def _mp_reach_ipv6(nh_bytes, nlri_bytes):
    """MP_REACH_NLRI attribute for AFI=2, SAFI=1 (RFC 4760 §3).

    Always uses the extended-length (2-byte) form because the NLRI block
    routinely exceeds 255 bytes.
    """
    value = (
        b"\x00\x02"  # AFI: IPv6
        b"\x01"  # SAFI: unicast
        b"\x10"  # Next-Hop length: 16
        + nh_bytes  # 16-byte IPv6 next hop
        + b"\x00"  # reserved / SNPA count
        + nlri_bytes
    )
    return bytes((_OPT_NON_TRANS_EXT, 14)) + len(value).to_bytes(2, "big") + value


def _pack_nlri(chunk, off, plen, plen_bytes, addr_int, addr_bytes):
    """Pack one (prefix-length, prefix) entry into chunk at offset off.

    `addr_bytes` is the total big-endian width (4 for v4, 16 for v6).
    Only the top `plen_bytes` of the address are written.
    """
    chunk[off] = plen
    chunk[off + 1 : off + 1 + plen_bytes] = addr_int.to_bytes(addr_bytes, "big")[
        :plen_bytes
    ]


def _write_v4_unicast(f, base_prefix, prefix_count, asn, nexthop):
    attrs = _attrs_v4_unicast(asn, nexthop)
    attrs_len = len(attrs)
    plen = base_prefix.prefixlen
    plen_bytes = (plen + 7) // 8
    stride = 1 + plen_bytes
    budget = BGP_MAX_MSG_LEN - BGP_HEADER_LEN - 2 - 2 - attrs_len
    nlri_per_msg = budget // stride
    if nlri_per_msg <= 0:
        raise ValueError("attrs exceed BGP max message size")

    step = 1 << (32 - plen) if plen < 32 else 1
    base_int = int(base_prefix.network_address)
    mask = 0xFFFFFFFF

    full_nlri = nlri_per_msg * stride
    full_msg_len = BGP_HEADER_LEN + 2 + 2 + attrs_len + full_nlri
    full_prefix_bytes = (
        BGP_MARKER
        + full_msg_len.to_bytes(2, "big")
        + _UPDATE_TYPE
        + _WDR_LEN_ZERO
        + attrs_len.to_bytes(2, "big")
        + attrs
    )

    chunk = bytearray(full_nlri)
    written = 0
    while written + nlri_per_msg <= prefix_count:
        off = 0
        for j in range(nlri_per_msg):
            addr = (base_int + (written + j) * step) & mask
            _pack_nlri(chunk, off, plen, plen_bytes, addr, 4)
            off += stride
        f.write(full_prefix_bytes)
        f.write(chunk)
        written += nlri_per_msg

    remaining = prefix_count - written
    if remaining > 0:
        partial_nlri = remaining * stride
        partial = bytearray(partial_nlri)
        off = 0
        for j in range(remaining):
            addr = (base_int + (written + j) * step) & mask
            _pack_nlri(partial, off, plen, plen_bytes, addr, 4)
            off += stride
        msg_len = BGP_HEADER_LEN + 2 + 2 + attrs_len + partial_nlri
        f.write(
            BGP_MARKER
            + msg_len.to_bytes(2, "big")
            + _UPDATE_TYPE
            + _WDR_LEN_ZERO
            + attrs_len.to_bytes(2, "big")
            + attrs
        )
        f.write(partial)


def _write_v6_unicast(f, base_prefix, prefix_count, asn, nexthop):
    base_attrs = _attrs_v6_prefix(asn)
    base_attrs_len = len(base_attrs)
    nh_bytes = socket.inet_pton(socket.AF_INET6, nexthop)

    # MP_REACH fixed overhead up to NLRI start:
    # attr_flag(1) + attr_type(1) + ext_len(2) + AFI(2) + SAFI(1) + nh_len(1) + nh(16) + reserved(1) = 25
    mp_fixed = 25
    plen = base_prefix.prefixlen
    plen_bytes = (plen + 7) // 8
    stride = 1 + plen_bytes
    budget = BGP_MAX_MSG_LEN - BGP_HEADER_LEN - 2 - 2 - base_attrs_len - mp_fixed
    nlri_per_msg = budget // stride
    if nlri_per_msg <= 0:
        raise ValueError("attrs exceed BGP max message size")

    step = 1 << (128 - plen) if plen < 128 else 1
    base_int = int(base_prefix.network_address)
    mask = (1 << 128) - 1

    written = 0
    while written + nlri_per_msg <= prefix_count:
        chunk = bytearray(nlri_per_msg * stride)
        off = 0
        for j in range(nlri_per_msg):
            addr = (base_int + (written + j) * step) & mask
            _pack_nlri(chunk, off, plen, plen_bytes, addr, 16)
            off += stride
        _emit_v6_msg(f, base_attrs, nh_bytes, bytes(chunk))
        written += nlri_per_msg

    remaining = prefix_count - written
    if remaining > 0:
        chunk = bytearray(remaining * stride)
        off = 0
        for j in range(remaining):
            addr = (base_int + (written + j) * step) & mask
            _pack_nlri(chunk, off, plen, plen_bytes, addr, 16)
            off += stride
        _emit_v6_msg(f, base_attrs, nh_bytes, bytes(chunk))


def _emit_v6_msg(f, base_attrs, nh_bytes, nlri_bytes):
    mp_reach = _mp_reach_ipv6(nh_bytes, nlri_bytes)
    attrs = base_attrs + mp_reach
    attrs_len = len(attrs)
    msg_len = BGP_HEADER_LEN + 2 + 2 + attrs_len
    f.write(
        BGP_MARKER
        + msg_len.to_bytes(2, "big")
        + _UPDATE_TYPE
        + _WDR_LEN_ZERO
        + attrs_len.to_bytes(2, "big")
        + attrs
    )


def generate_file(path, prefix_base, prefix_count, nexthop, asn, end_of_rib=True):
    """Write `prefix_count` BGP UPDATE messages to `path`.

    `prefix_base` may be a string ("10.0.0.0/24") or an ipaddress network.
    Family (v4 / v6) is taken from prefix_base; `nexthop` must match.
    """
    if isinstance(prefix_base, str):
        prefix_base = ipaddress.ip_network(prefix_base, strict=False)
    with open(path, "wb") as f:
        if prefix_base.version == 4:
            _write_v4_unicast(f, prefix_base, prefix_count, asn, nexthop)
        elif prefix_base.version == 6:
            _write_v6_unicast(f, prefix_base, prefix_count, asn, nexthop)
        else:
            raise ValueError("unknown address family: %r" % prefix_base)
        if end_of_rib:
            f.write(_EOR_MSG)


# --- Verification helpers (used by self-test + CI) --------------------------


def count_nlri(path):
    """Walk `path` and return (message_count, total_nlri_count).

    NLRI counted across IPv4 NLRI field and MP_REACH_NLRI payload. End-of-RIB
    messages (empty UPDATEs) count as messages but contribute zero prefixes.
    """
    with open(path, "rb") as f:
        data = f.read()
    n_msgs = 0
    n_nlri = 0
    i = 0
    while i < len(data):
        if data[i : i + 16] != BGP_MARKER:
            raise ValueError("bad marker at offset %d" % i)
        msg_len = int.from_bytes(data[i + 16 : i + 18], "big")
        msg_type = data[i + 18]
        if msg_type != 2:
            raise ValueError("non-UPDATE type %d at offset %d" % (msg_type, i))
        body = data[i + 19 : i + msg_len]
        wdr_len = int.from_bytes(body[0:2], "big")
        pos = 2 + wdr_len
        attr_len = int.from_bytes(body[pos : pos + 2], "big")
        attrs = body[pos + 2 : pos + 2 + attr_len]
        nlri = body[pos + 2 + attr_len :]
        n_nlri += _count_nlri_field(nlri, v6=False)
        n_nlri += _count_mp_reach_nlri(attrs)
        n_msgs += 1
        i += msg_len
    return n_msgs, n_nlri


def _count_nlri_field(buf, v6):
    """Count NLRI entries in an IPv4 NLRI field (trailing part of UPDATE)."""
    n = 0
    i = 0
    while i < len(buf):
        plen = buf[i]
        plen_bytes = (plen + 7) // 8
        i += 1 + plen_bytes
        n += 1
    return n


def _count_mp_reach_nlri(attrs):
    """Count NLRI inside any MP_REACH_NLRI (type 14) attribute."""
    n = 0
    i = 0
    while i < len(attrs):
        flags = attrs[i]
        typ = attrs[i + 1]
        if flags & 0x10:
            attr_len = int.from_bytes(attrs[i + 2 : i + 4], "big")
            hdr = 4
        else:
            attr_len = attrs[i + 2]
            hdr = 3
        value = attrs[i + hdr : i + hdr + attr_len]
        if typ == 14:
            # AFI(2) + SAFI(1) + nh_len(1) + nh + reserved(1) + NLRI
            nh_len = value[3]
            pos = 4 + nh_len + 1
            nlri = value[pos:]
            j = 0
            while j < len(nlri):
                plen = nlri[j]
                plen_bytes = (plen + 7) // 8
                j += 1 + plen_bytes
                n += 1
        i += hdr + attr_len
    return n


def _selftest():
    import os
    import tempfile

    with tempfile.TemporaryDirectory() as d:
        v4_path = os.path.join(d, "v4.bgp")
        generate_file(v4_path, "10.0.0.0/24", 5000, "172.31.0.3", 65100)
        msgs, nlri = count_nlri(v4_path)
        assert nlri == 5000, (msgs, nlri)
        size = os.path.getsize(v4_path)
        print("v4 ok: %d msgs, %d NLRI, %d bytes" % (msgs, nlri, size))

        v6_path = os.path.join(d, "v6.bgp")
        generate_file(v6_path, "2001:db8::/48", 2000, "2001:db8::3", 65100)
        msgs6, nlri6 = count_nlri(v6_path)
        assert nlri6 == 2000, (msgs6, nlri6)
        size6 = os.path.getsize(v6_path)
        print("v6 ok: %d msgs, %d NLRI, %d bytes" % (msgs6, nlri6, size6))

        big_path = os.path.join(d, "big.bgp")
        import time

        t0 = time.time()
        generate_file(big_path, "10.0.0.0/24", 1_000_000, "172.31.0.3", 65100)
        t = time.time() - t0
        msgs_b, nlri_b = count_nlri(big_path)
        assert nlri_b == 1_000_000, (msgs_b, nlri_b)
        print("1M ok: %d msgs, %d NLRI, %.2fs" % (msgs_b, nlri_b, t))


if __name__ == "__main__":
    _selftest()
