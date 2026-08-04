#!/usr/bin/env python3
"""Minimal, independent BGP speaker engine for interop-style testing.

Design (see plan/spec-bgp-plugin-speaker.md): the ENGINE is fixed plumbing -- TCP, an iBGP
OPEN/KEEPALIVE handshake with a per-instance router-id, a receive loop, and minimal UPDATE
decode. Each TEST is a small plugin the engine loads dynamically (importlib); the plugin
implements ONLY the check that test needs and names itself in any failure. The engine
validates nothing on its own -- that is the point: independence from ze's Go code, so a bug
ze's own (lenient) validator would wave through is caught here the way a real peer catches it.

Inspired by ExaBGP's process model (the plumbing runs; a loaded module reacts to events).

Plugin interface (duck-typed):
    NAME = "some-check"                 # the plugin names itself
    def on_update(update, ctx): ...     # called per received UPDATE (required)
    def on_end(ctx): ...                # called once at session end (optional)

RFC 4271 Section 4: message framing. RFC 4271 Section 4.3: UPDATE body.
"""

import argparse
import importlib.util
import socket
import struct
import sys
import time

MARKER = b"\xff" * 16
HEADER_LEN = 19

MSG_OPEN = 1
MSG_UPDATE = 2
MSG_NOTIFICATION = 3
MSG_KEEPALIVE = 4

BGP_VERSION = 4


# --------------------------------------------------------------------------- #
# Minimal decode
# --------------------------------------------------------------------------- #
class Attr:
    """A path attribute: flags, type code, and raw value bytes (RFC 4271 Section 4.3)."""

    __slots__ = ("flags", "code", "value")

    def __init__(self, flags, code, value):
        self.flags = flags
        self.code = code
        self.value = value

    def __repr__(self):
        return "Attr(code=%d, len=%d)" % (self.code, len(self.value))


class Update:
    """A minimally-decoded UPDATE: the two legacy sections plus the attribute list."""

    __slots__ = ("withdrawn", "attributes", "nlri", "raw")

    def __init__(self, withdrawn, attributes, nlri, raw):
        self.withdrawn = withdrawn
        self.attributes = attributes
        self.nlri = nlri
        self.raw = raw


def decode_attributes(data):
    """Decode a path-attribute block into a list of Attr. Extended-length aware."""
    attrs = []
    pos = 0
    n = len(data)
    while pos + 2 <= n:
        flags = data[pos]
        code = data[pos + 1]
        if flags & 0x10:  # Extended Length
            if pos + 4 > n:
                break
            length = struct.unpack_from(">H", data, pos + 2)[0]
            hdr = 4
        else:
            if pos + 3 > n:
                break
            length = data[pos + 2]
            hdr = 3
        if pos + hdr + length > n:
            break
        value = data[pos + hdr : pos + hdr + length]
        attrs.append(Attr(flags, code, value))
        pos += hdr + length
    return attrs


def decode_update(body):
    """Decode an UPDATE body (after the 19-byte header) into an Update. Minimal, no validation.

    RFC 4271 Section 4.3:
        Withdrawn Routes Length (2) | Withdrawn Routes | Total Path Attr Length (2) |
        Path Attributes | NLRI
    """
    # Bounds-guarded so a truncated or malformed body returns what parsed rather than raising
    # (an uncaught struct.error would abort the engine with no verdict line).
    if len(body) < 4:
        return Update(b"", [], b"", body)
    wlen = min(struct.unpack_from(">H", body, 0)[0], len(body) - 2)
    withdrawn = body[2 : 2 + wlen]
    alen_off = 2 + wlen
    if alen_off + 2 > len(body):
        return Update(withdrawn, [], b"", body)
    astart = alen_off + 2
    alen = min(struct.unpack_from(">H", body, alen_off)[0], len(body) - astart)
    attrs_bytes = body[astart : astart + alen]
    nlri = body[astart + alen :]
    return Update(withdrawn, decode_attributes(attrs_bytes), nlri, body)


# --------------------------------------------------------------------------- #
# Plugin dispatch
# --------------------------------------------------------------------------- #
class Context:
    """Passed to plugin hooks. The plugin records outcomes here; the engine owns the rest."""

    def __init__(self):
        self.failures = []
        self.notes = []
        self.store = {}
        self._name = "?"

    def fail(self, msg):
        self.failures.append("[%s] %s" % (self._name, msg))

    def note(self, msg):
        self.notes.append(msg)

    def failed(self):
        return bool(self.failures)


def load_plugin(path):
    """Dynamically load a test plugin module by file path (importlib)."""
    spec = importlib.util.spec_from_file_location("speaker_plugin", path)
    if spec is None or spec.loader is None:
        raise RuntimeError("cannot load plugin: %s" % path)
    mod = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(mod)
    if not hasattr(mod, "NAME") or not hasattr(mod, "on_update"):
        raise RuntimeError("plugin %s must define NAME and on_update" % path)
    return mod


def dispatch(update, plugin, ctx):
    """Call the plugin's on_update for one UPDATE, tagging the context with the plugin name."""
    ctx._name = plugin.NAME
    plugin.on_update(update, ctx)


def finish(plugin, ctx):
    """Call the plugin's optional on_end once at session end."""
    ctx._name = plugin.NAME
    if hasattr(plugin, "on_end"):
        plugin.on_end(ctx)


# --------------------------------------------------------------------------- #
# Wire encode (OPEN / KEEPALIVE)
# --------------------------------------------------------------------------- #
def _message(msg_type, body=b""):
    total = HEADER_LEN + len(body)
    return MARKER + struct.pack(">HB", total, msg_type) + body


def keepalive():
    return _message(MSG_KEEPALIVE)


def eor():
    """An End-of-RIB marker: an empty IPv4-unicast UPDATE (RFC 4724 Section 2)."""
    return _message(MSG_UPDATE, b"\x00\x00\x00\x00")


DEFAULT_FAMILIES = ((1, 1),)  # IPv4 unicast


MP_REACH = 14
MP_UNREACH = 15


def carries_routes(update):
    """True when an UPDATE conveys reachability, in ANY family.

    The legacy fields answer for IPv4 unicast. Every other family carries its routes inside
    MP_REACH_NLRI or MP_UNREACH_NLRI, where both legacy fields are empty, so reading them
    alone reported 0 route-bearing UPDATEs for a perfectly good EVPN or MCAST-VPN relay --
    and a check asserting "at least one route arrived" then failed on a working daemon.

    An End-of-RIB is deliberately NOT route-bearing, in either encoding: the empty UPDATE
    has nothing at all, and the multiprotocol marker is an MP_UNREACH whose value holds only
    AFI(2) and SAFI(1) with no NLRI after it (RFC 4724 Section 2, RFC 7606 Section 5.2). That
    keeps --stop-after-updates from tripping before the route it exists to inspect arrives.
    """
    if update.nlri or update.withdrawn:
        return True
    for attr in update.attributes:
        value = attr.value
        if attr.code == MP_REACH:
            # RFC 4760 Section 3: AFI(2) SAFI(1) NextHopLen(1) NextHop(n) Reserved(1) NLRI.
            if len(value) >= 4 and len(value) > 4 + value[3] + 1:
                return True
        elif attr.code == MP_UNREACH:
            # RFC 4760 Section 4: AFI(2) SAFI(1) then the withdrawn NLRI.
            if len(value) > 3:
                return True
    return False


def parse_families(values):
    """Turn ["25:70", ...] into ((25, 70), ...); None or [] gives DEFAULT_FAMILIES.

    Raises on anything else. A silently ignored family spelling would leave the speaker
    negotiating IPv4 unicast alone while its check waits for a family that can never
    arrive, which reads as "the relay dropped it" -- a false failure that costs more to
    diagnose than the error costs to raise.
    """
    if not values:
        return DEFAULT_FAMILIES
    out = []
    for value in values:
        afi, sep, safi = value.partition(":")
        if not sep:
            raise ValueError("--family wants AFI:SAFI, got %r" % value)
        out.append((int(afi), int(safi)))
    return tuple(out)


def open_message(asn, hold_time, router_id, families=DEFAULT_FAMILIES):
    """Build an OPEN with one MP capability per family + 4-octet-ASN (RFC 4271 Section 4.2).

    `families` is a sequence of (afi, safi) pairs and defaults to IPv4 unicast alone, so a
    scenario that says nothing gets the session it always got. A scenario that must receive
    a typed family (l2vpn/evpn is 25/70) names it, because ze gates every announce on the
    NEGOTIATED family set: a family this OPEN does not carry is a family the speaker can
    never be sent, and a check written against it would pass vacuously.
    """
    my_as = asn if asn <= 0xFFFF else 23456  # AS_TRANS for 4-octet ASNs
    rid = socket.inet_aton(router_id)

    # Capabilities (RFC 5492), each: code(1) len(1) value.
    # MP-BGP (RFC 4760 Section 8): AFI(2) Reserved(1) SAFI(1), one capability per family.
    cap_mp = b"".join(
        bytes([1, 4]) + struct.pack(">HBB", afi, 0, safi) for afi, safi in families
    )
    cap_as4 = bytes([65, 4]) + struct.pack(">I", asn)  # 4-octet ASN
    caps = cap_mp + cap_as4
    # Optional parameter: type 2 (Capabilities), length, value.
    opt = bytes([2, len(caps)]) + caps

    body = (
        struct.pack(">BHH", BGP_VERSION, my_as, hold_time)
        + rid
        + bytes([len(opt)])
        + opt
    )
    return _message(MSG_OPEN, body)


# --------------------------------------------------------------------------- #
# Session loop
# --------------------------------------------------------------------------- #
# Read-outcome sentinels. Distinguishing an idle socket from a real peer close is load-bearing:
# the session loop must KEEP looping on idle (to stay connected for the full duration and receive
# a replay that arrives after a quiet gap) and stop ONLY on a genuine close. Collapsing the two
# would break the loop at the first sub-second gap and could hide a delayed duplicate (false PASS).
_TIMEOUT = object()
_CLOSED = object()


def _read_message(sock):
    """Read one full BGP message.

    Returns (msg_type, body) on success, (_TIMEOUT, None) when the socket was idle with no
    message pending, or (_CLOSED, None) when the peer closed (or a partial message could not be
    completed).
    """
    hdr = _recv_exact(sock, HEADER_LEN)
    if hdr is _TIMEOUT:
        return _TIMEOUT, None
    if hdr is _CLOSED:
        return _CLOSED, None
    total, msg_type = struct.unpack_from(">HB", hdr, 16)
    if total <= HEADER_LEN:
        return msg_type, b""
    body = _recv_exact(sock, total - HEADER_LEN)
    # A timeout or close MID-message means the message cannot be framed; treat as closed.
    if body is _TIMEOUT or body is _CLOSED:
        return _CLOSED, None
    return msg_type, body


def _recv_exact(sock, n):
    """Read exactly n bytes. Returns the bytes, _CLOSED on EOF, or _TIMEOUT if the socket was
    idle for the whole timeout before ANY byte of this read arrived. Once bytes have arrived it
    keeps waiting through timeouts for the rest, so a message split across a timeout boundary is
    framed correctly rather than truncated or misread as a close."""
    buf = b""
    while len(buf) < n:
        try:
            chunk = sock.recv(n - len(buf))
        except socket.timeout:
            if not buf:
                return _TIMEOUT
            continue
        if not chunk:
            return _CLOSED
        buf += chunk
    return buf


def run(
    host,
    port,
    asn,
    router_id,
    plugin,
    ctx,
    hold_time=90,
    duration=45.0,
    stop_after=0,
    connect_delay=0.0,
    families=DEFAULT_FAMILIES,
):
    """Dial ze, establish an iBGP session, and dispatch every UPDATE to the plugin.

    Returns when the peer closes, `duration` seconds elapse, or (when `stop_after` > 0) that
    many route-bearing UPDATEs have been dispatched. KEEPALIVEs keep the session up; the engine
    replies to every KEEPALIVE and sends its own each hold_time/3. On reaching Established it
    sends one End-of-RIB, since a route server may gate release of a client's routes on the
    receiving client's EoR.

    `stop_after` counts only UPDATEs that carry NLRI or withdrawals; an EoR (empty UPDATE) never
    trips it, so the engine cannot exit before the route it exists to inspect arrives.

    `connect_delay` holds off the dial so a route a peer announced earlier is already stored in
    the server and is delivered by its replay-on-peer-up (re-encode) path, not forwarded live --
    the two paths differ, and a check aimed at the re-encode path must land on it.
    """
    if connect_delay > 0:
        time.sleep(connect_delay)
    sock = socket.create_connection((host, port), timeout=10)
    sock.settimeout(1.0)
    sock.sendall(open_message(asn, hold_time, router_id, families))

    deadline = time.monotonic() + duration
    next_ka = time.monotonic() + hold_time / 3.0
    established = False
    routes_seen = 0

    while time.monotonic() < deadline:
        if time.monotonic() >= next_ka:
            try:
                sock.sendall(keepalive())
            except OSError:
                break
            next_ka = time.monotonic() + hold_time / 3.0

        msg_type, body = _read_message(sock)
        if msg_type is _TIMEOUT:
            continue  # idle: keep the session up, re-check deadline, send periodic keepalives
        if msg_type is _CLOSED:
            if not established:
                ctx.note("peer closed before establishment")
            break

        try:
            if msg_type == MSG_OPEN:
                sock.sendall(keepalive())  # accept the OPEN, move toward Established
            elif msg_type == MSG_KEEPALIVE:
                if not established:
                    established = True
                    sock.sendall(eor())  # our EoR; a route server may gate replay on it
                sock.sendall(keepalive())
            elif msg_type == MSG_UPDATE:
                if not established:
                    established = True
                    sock.sendall(eor())
                update = decode_update(body)
                dispatch(update, plugin, ctx)
                if carries_routes(update):
                    routes_seen += 1
                    # Raw bytes of the route so a consumer (and a human debugging the scenario)
                    # can see exactly what Ze put on the wire, e.g. a NEXT_HOP that appears twice.
                    ctx.note("update-hex: %s" % body.hex())
                    if stop_after and routes_seen >= stop_after:
                        break
            elif msg_type == MSG_NOTIFICATION:
                ctx.note("received NOTIFICATION: %s" % body.hex())
                break
        except OSError:
            # Peer closed between our recv and a reply. Stop; the verdict below still reports
            # what was inspected rather than crashing with no result line.
            ctx.note("send failed, peer closed")
            break

    # Record how many route-bearing UPDATEs were inspected. A check that only asserts "no
    # failure" is vacuous when zero routes arrived (a broken session looks clean); a consumer
    # asserts this count is >= 1 so a silent session is a failure, not a false pass.
    ctx.note("route-bearing-updates: %d" % routes_seen)
    ctx.note("established: %s" % ("yes" if established else "no"))

    finish(plugin, ctx)
    try:
        sock.close()
    except OSError:
        pass
    return ctx


def main(argv=None):
    ap = argparse.ArgumentParser(
        description="Minimal BGP speaker engine (per-test plugin)."
    )
    ap.add_argument("--connect", required=True, help="ze host:port to dial")
    ap.add_argument(
        "--asn", type=int, required=True, help="local AS (iBGP: same as ze's)"
    )
    ap.add_argument("--router-id", required=True, help="this engine's BGP router-id")
    ap.add_argument("--test", required=True, help="path to the test plugin module")
    ap.add_argument("--result", help="write PASS/FAIL result here")
    ap.add_argument("--hold-time", type=int, default=90)
    ap.add_argument("--duration", type=float, default=45.0)
    ap.add_argument(
        "--stop-after-updates",
        type=int,
        default=0,
        help="exit after this many route-bearing UPDATEs (0 = run full duration)",
    )
    ap.add_argument(
        "--family",
        action="append",
        default=None,
        metavar="AFI:SAFI",
        help="advertise this multiprotocol family in the OPEN, repeatable "
        "(default: 1:1, IPv4 unicast). l2vpn/evpn is 25:70",
    )
    ap.add_argument(
        "--connect-delay",
        type=float,
        default=0.0,
        help="wait this many seconds before dialing (lets a route be stored first, so it is "
        "delivered by the server's replay-on-peer-up re-encode path)",
    )
    args = ap.parse_args(argv)

    host, _, port = args.connect.partition(":")
    families = parse_families(args.family)
    plugin = load_plugin(args.test)
    ctx = Context()
    ctx._name = plugin.NAME
    try:
        run(
            host,
            int(port or "179"),
            args.asn,
            args.router_id,
            plugin,
            ctx,
            hold_time=args.hold_time,
            duration=args.duration,
            stop_after=args.stop_after_updates,
            connect_delay=args.connect_delay,
            families=families,
        )
    except Exception as exc:  # noqa: BLE001 -- a crash must still emit a FAIL verdict, never a bare traceback
        # A test that dies with no "result:" line would be read as a relay bug, not a speaker
        # crash. Fail closed with the reason so check.py reports the truth.
        ctx.fail("engine crashed: %s: %s" % (type(exc).__name__, exc))

    ok = not ctx.failed()
    lines = ["result: %s" % ("PASS" if ok else "FAIL"), "plugin: %s" % plugin.NAME]
    lines += ["fail: %s" % f for f in ctx.failures]
    lines += ["note: %s" % n for n in ctx.notes]
    report = "\n".join(lines) + "\n"
    sys.stdout.write(report)
    sys.stdout.flush()
    if args.result:
        with open(args.result, "w", encoding="utf-8") as fh:
            fh.write(report)
    return 0 if ok else 1


if __name__ == "__main__":
    raise SystemExit(main())
