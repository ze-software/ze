#!/usr/bin/env python3
"""Permute the path-attribute block of committed BGP UPDATE hex expectations.

Why a transform and not a paste
-------------------------------
Ze chose ONE attribute order for every UPDATE builder (MP_UNREACH first, then
everything else by ascending type code) and rewrote 52 expectations to match.
Pasting the daemon's
fresh output would have produced the same green suite while silently baking in
whatever ELSE changed at the same time. This tool cannot do that: it parses the
COMMITTED hex, permutes the attribute block, and re-emits, asserting per
rewrite that

  * the attribute SET is unchanged (same type codes, same multiset),
  * every attribute's BYTES are unchanged (flags, length field, value),
  * the total message length is unchanged.

Anything that fails those assertions is left alone and reported, so a green
suite afterwards means the bytes MOVED and nothing else did.

The 1285 sweep counted test/encode and test/plugin and missed
test/exabgp-compat/encoding, which is why `make ze-exabgp-test` went red on the
13 conf-* tests whose families carry MP_REACH alongside a higher-coded
attribute. Those expectations were captured from ExaBGP, and permuting them
keeps the suite's byte-exact comparison while accepting the convention the
owner already chose -- RFC 4271 Section 5 leaves attribute order free, so the
permuted bytes remain a faithful encoding of the same route.

Usage:
    reorder_attr_expectations.py --check FILE...   # report, write nothing
    reorder_attr_expectations.py --write FILE...   # rewrite in place
"""

from __future__ import annotations

import argparse
import re
import sys

# BGP message type 2 = UPDATE. Only UPDATE carries a path-attribute block.
MSG_TYPE_UPDATE = 0x02

# RFC 4760 Section 4: MP_UNREACH_NLRI. It leads, out of type-code order, so a
# withdrawal is never parsed behind the announcement it supersedes
# (internal/core/bgp/attribute/origin.go OrderAttributes).
ATTR_MP_UNREACH = 15

# Attribute Flags bit 4 (0x10): Extended Length -- the length field is two
# octets rather than one (RFC 4271 Section 4.3).
FLAG_EXTENDED_LENGTH = 0x10

# A `<conn>:raw:` expectation. Everything after the prefix is ONE hex string;
# the colons in it are decoration and do not mark byte boundaries. The suite
# writes at least four shapes -- `marker:len:type:payload`, `marker:lentype:
# payload`, one unbroken run, and (conf-path-information.ci) colons that fall
# mid-byte, e.g. `...4003040:A0A0103:4005...`. Splitting on them is what made
# the first version of this tool skip conf-sr-policy.ci silently: it matched
# only the first shape, and a non-matching line looked exactly like a line that
# needed no change.
RAW_LINE = re.compile(r"^(?P<head>\d+:raw:)(?P<body>[0-9A-Fa-f:]+?)(?P<tail>\s*)$")

# BGP message header: 16-octet marker, 2-octet length, 1-octet type
# (RFC 4271 Section 4.1). The marker is all-ones on every message ze or ExaBGP
# emits.
MARKER_LEN = 16
HEADER_LEN = 19


class Unparsable(Exception):
    """The hex does not decode as a well-formed UPDATE; leave it alone."""


def split_attributes(block: bytes) -> list[tuple[int, bytes]]:
    """Return [(type_code, raw_attribute_bytes)] for one path-attribute block.

    Each element's bytes are the attribute EXACTLY as committed -- flags,
    type, length field and value -- so re-emitting them in a different order
    cannot alter any attribute's encoding.
    """
    out: list[tuple[int, bytes]] = []
    off = 0
    while off < len(block):
        if off + 3 > len(block):
            raise Unparsable(f"attribute header truncated at offset {off}")
        flags = block[off]
        code = block[off + 1]
        if flags & FLAG_EXTENDED_LENGTH:
            if off + 4 > len(block):
                raise Unparsable(f"extended length truncated at offset {off}")
            length = int.from_bytes(block[off + 2 : off + 4], "big")
            header = 4
        else:
            length = block[off + 2]
            header = 3
        end = off + header + length
        if end > len(block):
            raise Unparsable(f"attribute {code} value truncated at offset {off}")
        out.append((code, block[off:end]))
        off = end
    return out


def canonical_order(attrs: list[tuple[int, bytes]]) -> list[tuple[int, bytes]]:
    """MP_UNREACH first, then ascending type code. Stable within a type code."""
    unreach = [a for a in attrs if a[0] == ATTR_MP_UNREACH]
    regular = [a for a in attrs if a[0] != ATTR_MP_UNREACH]
    regular.sort(key=lambda a: a[0])
    return unreach + regular


def reorder_update_payload(payload: bytes) -> bytes:
    """Return the payload with its attribute block in canonical order."""
    if len(payload) < 2:
        raise Unparsable("payload shorter than the withdrawn-routes length field")
    withdrawn_len = int.from_bytes(payload[0:2], "big")
    attrs_len_off = 2 + withdrawn_len
    if attrs_len_off + 2 > len(payload):
        raise Unparsable("withdrawn-routes length runs past the payload")
    attrs_len = int.from_bytes(payload[attrs_len_off : attrs_len_off + 2], "big")
    attrs_off = attrs_len_off + 2
    attrs_end = attrs_off + attrs_len
    if attrs_end > len(payload):
        raise Unparsable("path-attribute length runs past the payload")

    attrs = split_attributes(payload[attrs_off:attrs_end])
    ordered = canonical_order(attrs)

    # The three invariants. A permutation may not change what is there.
    if sorted(a[0] for a in attrs) != sorted(a[0] for a in ordered):
        raise Unparsable("attribute set changed")
    if sorted(a[1] for a in attrs) != sorted(a[1] for a in ordered):
        raise Unparsable("attribute bytes changed")
    rebuilt = b"".join(a[1] for a in ordered)
    if len(rebuilt) != attrs_len:
        raise Unparsable("attribute block length changed")

    return payload[:attrs_off] + rebuilt + payload[attrs_end:]


def reorder_line(line: str) -> tuple[str, bool]:
    """Return (line, changed) for one `:raw:` expectation.

    Lines that are not `:raw:` pass through untouched. A `:raw:` line that
    cannot be decoded as a BGP message raises, so the caller reports it -- a
    silent skip here is indistinguishable from "needed no change", which is
    exactly how conf-sr-policy.ci was missed.
    """
    m = RAW_LINE.match(line)
    if not m:
        # A commented-out expectation is not a live one; the suite never reads
        # it, so it is neither reordered nor reported.
        if ":raw:" in line and not line.lstrip().startswith("#"):
            raise Unparsable("raw expectation is not hex")
        return line, False

    body = m.group("body")
    colon_at = [i for i, ch in enumerate(body) if ch == ":"]
    hexstr = body.replace(":", "")
    if len(hexstr) % 2:
        raise Unparsable(f"odd hex length {len(hexstr)}")

    msg = bytes.fromhex(hexstr)
    if len(msg) < HEADER_LEN:
        raise Unparsable(f"message shorter than a BGP header ({len(msg)} bytes)")
    if msg[:MARKER_LEN] != b"\xff" * MARKER_LEN:
        raise Unparsable("no all-ones BGP marker")
    declared = int.from_bytes(msg[MARKER_LEN : MARKER_LEN + 2], "big")
    if declared != len(msg):
        raise Unparsable(f"header length {declared} != {len(msg)} bytes present")
    if msg[MARKER_LEN + 2] != MSG_TYPE_UPDATE:
        return line, False

    payload = msg[HEADER_LEN:]
    new_payload = reorder_update_payload(payload)
    if new_payload == payload:
        return line, False
    if len(new_payload) != len(payload):
        raise Unparsable("payload length changed")

    # Re-insert the decorative colons at the same character offsets. The hex is
    # a permutation, so its length is unchanged and every offset stays valid;
    # the file then diffs by exactly the bytes that moved.
    new_hex = (msg[:HEADER_LEN] + new_payload).hex().upper()
    chars = list(new_hex)
    for i in colon_at:
        chars.insert(i, ":")
    return f"{m.group('head')}{''.join(chars)}{m.group('tail')}", True


def process(path: str, write: bool) -> tuple[int, int]:
    """Return (rewritten_lines, skipped_lines) for one expectation file."""
    with open(path, encoding="utf-8") as fh:
        lines = fh.readlines()

    changed = 0
    skipped = 0
    out: list[str] = []
    for lineno, line in enumerate(lines, 1):
        try:
            new_line, did = reorder_line(line)
        except (Unparsable, ValueError) as exc:
            print(f"{path}:{lineno}: left alone: {exc}", file=sys.stderr)
            out.append(line)
            skipped += 1
            continue
        out.append(new_line)
        if did:
            changed += 1

    if changed and write:
        with open(path, "w", encoding="utf-8") as fh:
            fh.writelines(out)
    return changed, skipped


def main() -> int:
    ap = argparse.ArgumentParser(
        description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter
    )
    mode = ap.add_mutually_exclusive_group(required=True)
    mode.add_argument(
        "--check", action="store_true", help="report what would change, write nothing"
    )
    mode.add_argument("--write", action="store_true", help="rewrite the files in place")
    ap.add_argument("files", nargs="+")
    args = ap.parse_args()

    total_changed = 0
    total_skipped = 0
    for path in args.files:
        changed, skipped = process(path, args.write)
        total_changed += changed
        total_skipped += skipped
        if changed:
            verb = "would reorder" if args.check else "reordered"
            print(f"{path}: {verb} {changed} expectation(s)")

    print(f"total: {total_changed} expectation(s), {total_skipped} left alone")
    return 1 if total_skipped else 0


if __name__ == "__main__":
    sys.exit(main())
