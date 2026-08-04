#!/usr/bin/env python3
"""Unit tests for the minimal BGP speaker engine and its plugin dispatch.

These run without a socket or a live ze: they exercise the wire decode and the dynamic
plugin dispatch directly, so each check ships with a red/green fixture (spec risk R-1/R-2).
"""

import os
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, HERE)

import engine  # noqa: E402

# Wire fragments (RFC 4271 Section 4.3 path attributes).
ORIGIN = bytes([0x40, 0x01, 0x01, 0x00])  # ORIGIN IGP
ASPATH = bytes([0x40, 0x02, 0x00])  # empty AS_PATH


def next_hop(ip=b"\x0a\x00\x00\x09"):
    return bytes([0x40, 0x03, 0x04]) + ip


def build_update(withdrawn=b"", attrs=b"", nlri=b""):
    return (
        len(withdrawn).to_bytes(2, "big")
        + withdrawn
        + len(attrs).to_bytes(2, "big")
        + attrs
        + nlri
    )


def test_decode_update_sections():
    attrs = ORIGIN + ASPATH + next_hop()
    nlri = bytes([0x18, 0x0A, 0x00, 0x00])  # 10.0.0.0/24
    body = build_update(withdrawn=b"", attrs=attrs, nlri=nlri)

    u = engine.decode_update(body)
    codes = [a.code for a in u.attributes]
    assert codes == [1, 2, 3], codes
    assert u.attributes[2].value == b"\x0a\x00\x00\x09"
    assert u.nlri == nlri
    assert u.withdrawn == b""


def test_decode_update_extended_length_attr():
    # An extended-length attribute (flag 0x10): 2-byte length field.
    ext = bytes([0x90, 0x0E, 0x00, 0x02, 0xAA, 0xBB])  # MP_REACH-ish, ext len 2
    body = build_update(attrs=ORIGIN + ext)
    u = engine.decode_update(body)
    assert [a.code for a in u.attributes] == [1, 14]
    assert u.attributes[1].value == b"\xaa\xbb"


def _run_plugin(plugin_name, body):
    plugin = engine.load_plugin(os.path.join(HERE, "plugins", plugin_name))
    ctx = engine.Context()
    u = engine.decode_update(body)
    engine.dispatch(u, plugin, ctx)
    return ctx


def test_plugin_flags_duplicate_next_hop():
    # AC-2: two NEXT_HOP attributes -> the plugin fails, naming itself.
    attrs = (
        ORIGIN + ASPATH + next_hop(b"\x0a\x00\x00\x09") + next_hop(b"\xc0\x00\x02\x01")
    )
    ctx = _run_plugin("no_duplicate_attribute.py", build_update(attrs=attrs))
    assert ctx.failed(), "duplicate NEXT_HOP must be flagged"
    assert any("no-duplicate-attribute" in f for f in ctx.failures), ctx.failures
    assert any("3" in f for f in ctx.failures), ctx.failures


def test_plugin_passes_clean_update():
    # AC-3: a well-formed UPDATE -> no failure.
    attrs = ORIGIN + ASPATH + next_hop()
    ctx = _run_plugin("no_duplicate_attribute.py", build_update(attrs=attrs))
    assert not ctx.failed(), ctx.failures


def evpn_mp_reach(*nlris):
    """One MP_REACH_NLRI attribute for l2vpn/evpn (AFI 25 / SAFI 70) carrying nlris.

    RFC 4760 Section 3: AFI(2) SAFI(1) NextHopLen(1) NextHop(n) Reserved(1) then NLRI.
    """
    value = bytes([0x00, 0x19, 0x46, 0x04, 0xAC, 0x1E, 0x00, 0x09, 0x00])
    for one in nlris:
        value += one
    return bytes([0x80, 0x0E, len(value)]) + value


def evpn_nlri(route_type, body=b"\xaa\xbb"):
    """One EVPN NLRI: [route-type:1][length:1][body] (RFC 7432 Section 7.1)."""
    return bytes([route_type, len(body)]) + body


def test_carries_routes_counts_mp_families():
    # VALIDATES: an MP_REACH-only UPDATE is route-bearing even though both legacy fields are
    # empty, and so is an MP_UNREACH carrying withdrawn NLRI.
    # PREVENTS: a check asserting "at least one route arrived" failing on a working EVPN
    # relay, which reads as a relay bug and hides the real verdict.
    mp_only = engine.decode_update(
        build_update(attrs=ORIGIN + ASPATH + evpn_mp_reach(evpn_nlri(2)))
    )
    assert engine.carries_routes(mp_only)

    unreach = bytes([0x80, 0x0F, 0x05, 0x00, 0x19, 0x46, 0x02, 0x00])
    assert engine.carries_routes(engine.decode_update(build_update(attrs=unreach)))


def test_carries_routes_excludes_end_of_rib():
    # VALIDATES: neither RFC 4724 Section 2 End-of-RIB encoding counts as route-bearing.
    # PREVENTS: --stop-after-updates tripping on an EoR before the route arrives.
    assert not engine.carries_routes(engine.decode_update(build_update()))
    mp_eor = bytes([0x80, 0x0F, 0x03, 0x00, 0x19, 0x46])
    assert not engine.carries_routes(engine.decode_update(build_update(attrs=mp_eor)))


def test_evpn_plugin_flags_unassigned_route_type():
    # VALIDATES: RFC 7606 Section 5.4 -- an EVPN route type that is not assigned must never
    # reach a peer, so receiving one is a failure.
    attrs = ORIGIN + ASPATH + evpn_mp_reach(evpn_nlri(2), evpn_nlri(99))
    ctx = _run_plugin("no_unrecognized_evpn_type.py", build_update(attrs=attrs))
    assert ctx.failed(), "route type 99 must be flagged"
    assert any("99" in f for f in ctx.failures), ctx.failures
    assert ctx.store["evpn_nlri"] == 2, ctx.store


def test_evpn_plugin_passes_assigned_route_types():
    # VALIDATES: the five assigned route types are accepted, so the check cannot pass by
    # failing everything.
    attrs = ORIGIN + ASPATH + evpn_mp_reach(*(evpn_nlri(rt) for rt in (1, 2, 3, 4, 5)))
    ctx = _run_plugin("no_unrecognized_evpn_type.py", build_update(attrs=attrs))
    assert not ctx.failed(), ctx.failures
    assert ctx.store["evpn_nlri"] == 5, ctx.store


def test_evpn_plugin_ignores_other_families():
    # VALIDATES: an IPv4-unicast MP_REACH is not walked as EVPN.
    # PREVENTS: reading a prefix length as a route type and failing an unrelated UPDATE.
    value = bytes(
        [0x00, 0x01, 0x01, 0x04, 0x0A, 0x00, 0x00, 0x09, 0x00, 0x18, 0x0A, 0x00, 0x00]
    )
    attrs = ORIGIN + ASPATH + bytes([0x80, 0x0E, len(value)]) + value
    ctx = _run_plugin("no_unrecognized_evpn_type.py", build_update(attrs=attrs))
    assert not ctx.failed(), ctx.failures
    assert ctx.store["evpn_nlri"] == 0, ctx.store


def test_eor_is_empty_update():
    # The engine's EoR is a valid empty IPv4-unicast UPDATE: 19-byte header + 4 zero bytes.
    msg = engine.eor()
    assert msg[:16] == engine.MARKER
    total, mtype = engine.struct.unpack_from(">HB", msg, 16)
    assert mtype == engine.MSG_UPDATE
    assert total == engine.HEADER_LEN + 4
    u = engine.decode_update(msg[engine.HEADER_LEN :])
    # An EoR carries no route: this is exactly the predicate --stop-after-updates counts on,
    # so an EoR must never trip the early exit before the real route arrives.
    assert not u.nlri and not u.withdrawn


def test_open_message_carries_per_instance_router_id():
    # AC-5: two engine instances with different router-ids produce different OPEN messages, so
    # two speakers in one run never collide (ExaBGP-style per-instance router-id).
    a = engine.open_message(65001, 90, "1.2.3.4")
    b = engine.open_message(65001, 90, "5.6.7.8")
    assert a != b
    # Router-id sits at message offset 24: 19-byte header + body[version(1), AS(2), hold(2)] = 5.
    assert a[24:28] == engine.socket.inet_aton("1.2.3.4")
    assert b[24:28] == engine.socket.inet_aton("5.6.7.8")


def test_parse_families_defaults_and_parses():
    # VALIDATES: no --family keeps the IPv4-unicast-only OPEN every earlier scenario relies on,
    # and AFI:SAFI pairs parse in order.
    assert engine.parse_families(None) == ((1, 1),)
    assert engine.parse_families([]) == ((1, 1),)
    assert engine.parse_families(["1:1", "25:70"]) == ((1, 1), (25, 70))


def test_parse_families_rejects_malformed():
    # VALIDATES: a spelling the engine cannot read raises instead of being ignored.
    # PREVENTS: a speaker silently negotiating IPv4 unicast alone while its check waits for a
    # family that can never arrive -- which reads as a relay bug, not a typo.
    for bad in ["25", "", "l2vpn/evpn"]:
        try:
            engine.parse_families([bad])
        except ValueError:
            continue
        raise AssertionError("--family %r must be rejected" % bad)


def test_open_message_carries_each_requested_family():
    # VALIDATES: one RFC 4760 Section 8 multiprotocol capability per requested family, encoded
    # AFI(2) Reserved(1) SAFI(1). l2vpn/evpn is AFI 25 / SAFI 70.
    # PREVENTS: an EVPN scenario passing vacuously because ze never had a negotiated family to
    # send the route on (announces gate on the negotiated set).
    default = engine.open_message(65001, 90, "1.2.3.4")
    evpn = engine.open_message(65001, 90, "1.2.3.4", ((1, 1), (25, 70)))
    assert len(evpn) > len(default), "a second family must add a capability"
    assert bytes([1, 4, 0, 25, 0, 70]) in evpn
    assert bytes([1, 4, 0, 25, 0, 70]) not in default
    assert bytes([1, 4, 0, 1, 0, 1]) in default


class _FakeSock:
    """A socket stub whose recv() replays a scripted sequence. A `socket.timeout` item raises
    that exception (an idle read); a bytes item is returned; b"" is EOF."""

    def __init__(self, script):
        self._script = list(script)

    def recv(self, _n):
        if not self._script:
            return b""
        item = self._script.pop(0)
        if item is engine.socket.timeout:
            raise engine.socket.timeout()
        return item


def test_recv_exact_idle_timeout_is_distinct_from_close():
    # VALIDATES: an idle read (no bytes) returns _TIMEOUT, NOT _CLOSED, so the session loop keeps
    # running instead of quitting at the first sub-second gap (the false-GREEN risk).
    sock = _FakeSock([engine.socket.timeout])
    assert engine._recv_exact(sock, 4) is engine._TIMEOUT

    # A real EOF (recv returns b"") is _CLOSED.
    sock = _FakeSock([b""])
    assert engine._recv_exact(sock, 4) is engine._CLOSED


def test_recv_exact_waits_through_midmessage_timeout():
    # VALIDATES: once bytes have arrived, a timeout does NOT truncate the message; _recv_exact
    # keeps waiting and frames the full message.
    sock = _FakeSock([b"\xab", engine.socket.timeout, b"\xcd\xef\x01"])
    assert engine._recv_exact(sock, 4) == b"\xab\xcd\xef\x01"


def test_read_message_timeout_then_message():
    # VALIDATES: _read_message surfaces (_TIMEOUT, None) on idle, and frames a real message after.
    hdr = engine.MARKER + engine.struct.pack(
        ">HB", engine.HEADER_LEN, engine.MSG_KEEPALIVE
    )
    sock = _FakeSock([engine.socket.timeout])
    assert engine._read_message(sock) == (engine._TIMEOUT, None)
    sock = _FakeSock([hdr])
    mtype, body = engine._read_message(sock)
    assert mtype == engine.MSG_KEEPALIVE and body == b""


def test_decode_update_truncated_does_not_crash():
    # VALIDATES: a malformed/truncated body returns what parsed rather than raising struct.error
    # (which would abort the engine with no verdict line).
    for bad in [b"", b"\x00", b"\xff\xff", b"\x00\x0a\x00", b"\x00\x00\xff\xff"]:
        u = engine.decode_update(bad)  # must not raise
        assert isinstance(u.attributes, list)


def test_dynamic_load_optional_on_end():
    # AC-1 / AC-6: a plugin with only NAME + on_update loads and runs; on_end is optional.
    plugin = engine.load_plugin(
        os.path.join(HERE, "plugins", "no_duplicate_attribute.py")
    )
    assert plugin.NAME == "no-duplicate-attribute"
    assert callable(plugin.on_update)
    ctx = engine.Context()
    # Calling the engine's end hook when the plugin has no on_end must not raise.
    engine.finish(plugin, ctx)


def _main():
    """Run every test_* function in this module and exit non-zero on any failure.

    Deliberately dependency-free. This file used to hand itself to pytest, which is not
    installed here, so `python3 test_engine.py` exited with ImportError and no gate could
    run it: every test below was dead for as long as that was true
    (scripts/dev/python_tests_test.go, which now globs this root).
    """
    import traceback

    failures = 0
    for name in sorted(globals()):
        if not name.startswith("test_"):
            continue
        try:
            globals()[name]()
        except KeyboardInterrupt:
            # Ctrl-C is the operator, not a failure. Re-raise so the run stops now instead
            # of marking one test FAIL and grinding through the rest.
            raise
        # BaseException rather than Exception: a test that raises SystemExit would otherwise
        # escape this loop and end the run with that test's own exit code, leaving the rest
        # unrun and a SystemExit(0) reading as a clean pass.
        except BaseException:  # noqa: BLE001 -- report every failure, do not stop at the first
            failures += 1
            print("FAIL %s" % name)
            traceback.print_exc()
        else:
            print("PASS %s" % name)
    print("failures: %d" % failures)
    return 1 if failures else 0


if __name__ == "__main__":
    raise SystemExit(_main())
