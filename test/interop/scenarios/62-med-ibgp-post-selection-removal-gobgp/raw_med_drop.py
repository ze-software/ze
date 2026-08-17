#!/usr/bin/env python3
"""Raw export filter that removes MULTI_EXIT_DISC from the destination base.

The scenario expects Ze to restore the selected MED after this post-selection raw
replacement. If the filter does not run, the check fails on the sentinel in Ze's
log. If the guard does not run, GoBGP receives the route without Med: 100.
"""

import base64
import sys

from ze_api import API, runtime_fail

removed = 0


def drop_med(body):
    if len(body) < 4:
        runtime_fail("RAW-MED-DROP: raw UPDATE body too short")
        raise RuntimeError("short raw body")
    withdrawn_len = int.from_bytes(body[0:2], "big")
    attrs_len_off = 2 + withdrawn_len
    if attrs_len_off + 2 > len(body):
        runtime_fail("RAW-MED-DROP: withdrawn section overruns body")
        raise RuntimeError("bad withdrawn length")
    attr_len = int.from_bytes(body[attrs_len_off:attrs_len_off + 2], "big")
    attrs_start = attrs_len_off + 2
    attrs_end = attrs_start + attr_len
    if attrs_end > len(body):
        runtime_fail("RAW-MED-DROP: attribute section overruns body")
        raise RuntimeError("bad attribute length")
    attrs = body[attrs_start:attrs_end]
    out = bytearray()
    pos = 0
    saw_med = False
    while pos < len(attrs):
        if pos + 3 > len(attrs):
            runtime_fail("RAW-MED-DROP: truncated attribute header")
            raise RuntimeError("truncated attribute")
        flags = attrs[pos]
        code = attrs[pos + 1]
        if flags & 0x10:
            if pos + 4 > len(attrs):
                runtime_fail("RAW-MED-DROP: truncated extended attribute header")
                raise RuntimeError("truncated extended attribute")
            length = int.from_bytes(attrs[pos + 2:pos + 4], "big")
            header_len = 4
        else:
            length = attrs[pos + 2]
            header_len = 3
        end = pos + header_len + length
        if end > len(attrs):
            runtime_fail("RAW-MED-DROP: attribute length overruns section")
            raise RuntimeError("bad attribute span")
        if code == 4:
            saw_med = True
        else:
            out.extend(attrs[pos:end])
        pos = end
    if not saw_med:
        return None
    return body[:attrs_len_off] + len(out).to_bytes(2, "big") + bytes(out) + body[attrs_end:]


def filter_handler(params):
    global removed
    if params.get("direction") != "export":
        return {"action": "accept"}
    raw_b64 = params.get("raw") or ""
    if not raw_b64:
        runtime_fail("RAW-MED-DROP: export callback had no raw body")
        return {"action": "reject"}
    stripped = drop_med(base64.b64decode(raw_b64))
    if stripped is None:
        return {"action": "accept"}
    removed += 1
    print(
        "RAW-MED-DROP: removed MULTI_EXIT_DISC for %s" % params.get("peer"),
        file=sys.stderr,
        flush=True,
    )
    return {"action": "modify", "raw": base64.b64encode(stripped).decode("ascii")}


def main():
    api = API()
    api.declare_filter(name="drop-med-raw", direction="export", on_error="reject")
    api._filters[-1]["raw"] = True
    api.on_filter_update(filter_handler)
    api.declare_done()
    api.wait_for_config()
    api.capability_done()
    api.wait_for_registry()
    api.ready()

    while True:
        api.read_line(timeout=1.0)


if __name__ == "__main__":
    main()
