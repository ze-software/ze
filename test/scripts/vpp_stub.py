#!/usr/bin/env python3
"""VPP GoVPP-API stub for ze functional tests.

Speaks enough of VPP's binary API socket-client protocol to satisfy ze's
GoVPP client (vendored under vendor/go.fd.io/govpp). Accepts one client
connection on a caller-provided Unix socket, negotiates the message table
by scraping (name, crc) pairs from the vendored binapi packages, and
logs every request as JSON Lines to a caller-provided file.

Handlers:
    sockclnt_create     -> sockclnt_create_reply (with full message table)
    sockclnt_delete     -> sockclnt_delete_reply, close connection
    control_ping        -> control_ping_reply retval=0
    ip_route_add_del    -> ip_route_add_del_reply retval=0, log decoded fields

Reference:
    vendor/go.fd.io/govpp/adapter/socketclient/socketclient.go
    vendor/go.fd.io/govpp/codec/codec.go
    vendor/go.fd.io/govpp/binapi/*/*.ba.go

Frame format:
    16-byte pool-reuse header where only bytes 8..11 carry the big-endian
    uint32 payload length; bytes 0..7 and 12..15 are whatever happened to
    be in the sync.Pool buffer at write time -- ignore them.

Request body layout (10-byte msg header + body):
    [0..1]  MsgID (u16 BE)
    [2..5]  ClientIndex (u32 BE)
    [6..9]  Context (u32 BE)
    [10..]  payload

Reply body layout varies with message type (see getOffset in codec.go):
    RequestMessage (e.g. sockclnt_create_reply):   10-byte header
    ReplyMessage   (e.g. ip_route_add_del_reply):   6-byte header
                      [0..1]  MsgID (u16 BE)
                      [2..5]  Context (u32 BE echoed from request)
                      [6..]   payload
"""

import argparse
import datetime
import errno
import io
import json
import os
import re
import select
import signal
import socket
import socketserver
import struct
import sys
import threading


# Hard-coded message IDs from vendor/go.fd.io/govpp/adapter/socketclient/socketclient.go
SOCKCLNT_CREATE_HARDCODED_ID = 15

# First MsgID assigned to negotiated messages. Anything >= 100 stays out of
# the way of VPP's internal reserved IDs (0..16 in practice).
NEGOTIATED_ID_BASE = 100


# Regex scrapers for vendored binapi Go files.
MSG_NAME_RE = re.compile(
    r'func \(\*(\w+)\) GetMessageName\(\) string \{ return "(\w+)" \}'
)
CRC_RE = re.compile(
    r'func \(\*(\w+)\) GetCrcString\(\) string\s+\{ return "([0-9a-f]+)" \}'
)
MSG_TYPE_RE = re.compile(
    r"func \(\*(\w+)\) GetMessageType\(\) api\.MessageType \{\s*return api\.(\w+)"
)


def scrape_binapi(binapi_root):
    """Walk vendored binapi, return dict: message_name -> (crc, msg_type).

    msg_type is one of: "RequestMessage", "ReplyMessage", "EventMessage", "OtherMessage".
    """
    names = {}  # TypeName -> message_name
    crcs = {}  # TypeName -> crc
    types = {}  # TypeName -> msg_type

    if not os.path.isdir(binapi_root):
        raise SystemExit(f"vpp-stub: binapi not found at {binapi_root}")

    for root, _dirs, files in os.walk(binapi_root):
        for fname in files:
            if not fname.endswith(".ba.go"):
                continue
            with open(os.path.join(root, fname), "r", encoding="utf-8") as fh:
                content = fh.read()
            for m in MSG_NAME_RE.finditer(content):
                names[m.group(1)] = m.group(2)
            for m in CRC_RE.finditer(content):
                crcs[m.group(1)] = m.group(2)
            # GetMessageType spans multiple lines; just scan them individually.
            # The simpler regex above only catches single-line returns, so walk
            # the file instead and pair the type declaration with the next
            # "return api.XxxMessage" line.
            lines = content.splitlines()
            for i, line in enumerate(lines):
                m = re.match(r"func \(\*(\w+)\) GetMessageType\(\)", line)
                if not m:
                    continue
                type_name = m.group(1)
                # Search next few lines for "return api.<Kind>"
                for j in range(i + 1, min(i + 4, len(lines))):
                    rm = re.search(r"return api\.(\w+)", lines[j])
                    if rm:
                        types[type_name] = rm.group(1)
                        break

    out = {}
    for tn, name in names.items():
        if tn not in crcs:
            continue
        out[name] = (crcs[tn], types.get(tn, "OtherMessage"))
    return out


class StubState:
    """Shared state for a running stub instance."""

    def __init__(self, binapi_root, log_path, deadline=None, verbose=False):
        self.log_path = log_path
        self.deadline = deadline
        self.verbose = verbose

        self.messages = scrape_binapi(binapi_root)  # name -> (crc, kind)
        if "sockclnt_create_reply" not in self.messages:
            raise SystemExit("vpp-stub: vendored binapi missing sockclnt_create_reply")

        # Assign MsgIDs. sockclnt_create is hardcoded to 15; everything else
        # gets sequential IDs from NEGOTIATED_ID_BASE in sorted-name order so
        # the assignment is deterministic across runs.
        self.name_to_id = {}
        self.id_to_name = {}
        self.name_to_id["sockclnt_create"] = SOCKCLNT_CREATE_HARDCODED_ID
        self.id_to_name[SOCKCLNT_CREATE_HARDCODED_ID] = "sockclnt_create"

        next_id = NEGOTIATED_ID_BASE
        for name in sorted(self.messages.keys()):
            if name == "sockclnt_create":
                continue
            self.name_to_id[name] = next_id
            self.id_to_name[next_id] = name
            next_id += 1

        self._log_lock = threading.Lock()
        # Truncate log at startup so each stub run begins clean.
        with open(self.log_path, "w", encoding="utf-8") as fh:
            fh.write("")

    def log(self, msg_name, context, fields):
        entry = {
            "ts": datetime.datetime.now(datetime.timezone.utc).isoformat(),
            "msg": msg_name,
            "context": context,
            "fields": fields,
        }
        with self._log_lock:
            with open(self.log_path, "a", encoding="utf-8") as fh:
                fh.write(json.dumps(entry) + "\n")
        if self.verbose:
            sys.stderr.write(f"vpp-stub: {msg_name} {fields}\n")
            sys.stderr.flush()

    def msg_name_crc(self, name):
        """Return the `<name>_<crc>` string the client expects for name."""
        crc, _kind = self.messages[name]
        return f"{name}_{crc}"

    def msg_kind(self, name):
        """Return the GetMessageType kind for name."""
        _crc, kind = self.messages[name]
        return kind


def read_frame(sock):
    """Read one complete message frame off the wire.

    Returns (body_bytes, ) where body is everything after the 16-byte header.
    Returns None on clean EOF.
    """
    header = _readn(sock, 16)
    if header is None:
        return None
    if len(header) < 16:
        raise IOError(f"short frame header: {len(header)} bytes")
    # Only bytes 8..11 are meaningful; the rest is pool-reuse padding.
    (payload_len,) = struct.unpack_from(">I", header, 8)
    if payload_len == 0:
        return b""
    body = _readn(sock, payload_len)
    if body is None or len(body) < payload_len:
        raise IOError(
            f"short frame body: want {payload_len}, got {len(body) if body else 0}"
        )
    return body


def _readn(sock, n):
    """Read exactly n bytes from sock. Return None on clean EOF at start."""
    buf = bytearray()
    while len(buf) < n:
        try:
            chunk = sock.recv(n - len(buf))
        except OSError as e:
            if e.errno in (errno.EINTR,):
                continue
            raise
        if not chunk:
            if not buf:
                return None
            return bytes(buf)
        buf.extend(chunk)
    return bytes(buf)


def write_frame(sock, payload):
    """Write one GoVPP frame: 16-byte header with payload-length at 8..11."""
    header = bytearray(16)
    struct.pack_into(">I", header, 8, len(payload))
    sock.sendall(bytes(header) + payload)


def build_reply(state, reply_name, context, body, client_index=1):
    """Build a reply payload with the right header offset for reply_name.

    GoVPP's getOffset() decides the offset by GetMessageType. A "RequestMessage"
    reply (e.g. sockclnt_create_reply) uses a 10-byte header; a "ReplyMessage"
    reply (e.g. ip_route_add_del_reply) uses a 6-byte header; "EventMessage"
    also uses 6. The stub must match.
    """
    msg_id = state.name_to_id[reply_name]
    kind = state.msg_kind(reply_name)

    if kind == "RequestMessage":
        header = bytearray(10)
        struct.pack_into(">H", header, 0, msg_id)
        struct.pack_into(">I", header, 2, client_index)
        struct.pack_into(">I", header, 6, context)
    elif kind in ("ReplyMessage", "EventMessage"):
        header = bytearray(6)
        struct.pack_into(">H", header, 0, msg_id)
        struct.pack_into(">I", header, 2, context)
    else:
        header = bytearray(2)
        struct.pack_into(">H", header, 0, msg_id)

    return bytes(header) + body


def encode_sockclnt_create_reply_body(state):
    """Encode the body of sockclnt_create_reply: Response, Index, Count, MessageTable.

    Order matters: GoVPP's client iterates the table with a HasPrefix check
    for 'sockclnt_delete_' to find sockDelMsgId (see socketclient.go open()),
    and lets later entries overwrite earlier ones. Sorted alphabetical order
    would put sockclnt_delete BEFORE sockclnt_delete_reply, so the client
    would end up with the reply's MsgID as sockDelMsgId. Force
    sockclnt_delete to appear last so the HasPrefix scan settles on the
    real request message.
    """
    out = io.BytesIO()
    names = sorted(state.messages.keys())
    if "sockclnt_delete" in state.messages:
        names = [n for n in names if n != "sockclnt_delete"]
        names.append("sockclnt_delete")
    out.write(struct.pack(">iIH", 0, 1, len(names)))
    for name in names:
        name_crc = state.msg_name_crc(name)
        entry_name = name_crc.encode("ascii")
        if len(entry_name) > 64:
            entry_name = entry_name[:64]
        entry_name = entry_name.ljust(64, b"\x00")
        msg_id = state.name_to_id[name]
        out.write(struct.pack(">H", msg_id))
        out.write(entry_name)
    return out.getvalue()


def handle_sockclnt_create(state, sock, context, _body):
    body = encode_sockclnt_create_reply_body(state)
    state.log("sockclnt_create", context, {})
    reply = build_reply(state, "sockclnt_create_reply", context, body)
    write_frame(sock, reply)


def handle_sockclnt_delete(state, sock, context, _body):
    state.log("sockclnt_delete", context, {})
    body = struct.pack(">i", 0)  # Response=0
    reply = build_reply(state, "sockclnt_delete_reply", context, body)
    write_frame(sock, reply)
    raise _CloseConnection()


def handle_control_ping(state, sock, context, _body):
    state.log("control_ping", context, {})
    # ControlPingReply fields (from vendored binapi):
    #   Retval i32, ClientIndex u32, VpePID u32
    body = struct.pack(">iII", 0, 1, os.getpid())
    reply = build_reply(state, "control_ping_reply", context, body)
    write_frame(sock, reply)


def _parse_ip_address(af, un_16):
    """af=0 is IPv4 (first 4 bytes), af=1 is IPv6 (all 16 bytes)."""
    if af == 0:
        return socket.inet_ntop(socket.AF_INET, un_16[:4])
    return socket.inet_ntop(socket.AF_INET6, un_16[:16])


def handle_ip_route_add_del(state, sock, context, body):
    """Parse just enough of the IPRouteAddDel body to log the decision.

    Body layout after the 10-byte msg header (confirmed by Unmarshal at
    vendor/go.fd.io/govpp/binapi/ip/ip.ba.go:2540):
      0       IsAdd (u8)
      1       IsMultipath (u8)
      2..5    Route.TableID (u32 BE)
      6..9    Route.StatsIndex (u32 BE)
      10      Route.Prefix.Address.Af (u8: 0=IP4, 1=IP6)
      11..26  Route.Prefix.Address.Un (16 bytes, IPv4 uses first 4)
      27      Route.Prefix.Len (u8)
      28      Route.NPaths (u8)
      29+     Paths[NPaths]
    Each FibPath is 167 bytes:
      0..3    SwIfIndex u32
      4..7    TableID u32
      8..11   RpfID u32
      12      Weight u8
      13      Preference u8
      14..17  Type u32
      18..21  Flags u32
      22..25  Proto u32 (0=IP4, 1=IP6)
      26..41  Nh.Address (16 bytes)
      42..45  Nh.ViaLabel u32
      ...
    """
    fields = {}
    if len(body) >= 29:
        is_add = body[0] != 0
        is_multipath = body[1] != 0
        (table_id,) = struct.unpack_from(">I", body, 2)
        af = body[10]
        un = body[11:27]
        prefix_len = body[27]
        n_paths = body[28]
        prefix_addr = _parse_ip_address(af, un)
        fields["is_add"] = is_add
        fields["is_multipath"] = is_multipath
        fields["table_id"] = table_id
        fields["prefix"] = f"{prefix_addr}/{prefix_len}"
        fields["n_paths"] = n_paths
        if n_paths >= 1 and len(body) >= 29 + 42 + 16:
            path_off = 29
            (proto,) = struct.unpack_from(">I", body, path_off + 22)
            nh_un = body[path_off + 26 : path_off + 42]
            fields["next_hop"] = _parse_ip_address(proto, nh_un)
    state.log("ip_route_add_del", context, fields)
    # Reply body: Retval i32, StatsIndex u32
    body_out = struct.pack(">iI", 0, 0)
    reply = build_reply(state, "ip_route_add_del_reply", context, body_out)
    write_frame(sock, reply)


HANDLERS = {
    "sockclnt_create": handle_sockclnt_create,
    "sockclnt_delete": handle_sockclnt_delete,
    "control_ping": handle_control_ping,
    "ip_route_add_del": handle_ip_route_add_del,
}


class _CloseConnection(Exception):
    """Signal from a handler that the connection should be closed cleanly."""


def _request_header_len(state, msg_id):
    """Return the header length the client used for this outgoing message.

    GoVPP's EncodeMsg picks offset via GetMessageType. For most requests the
    type is RequestMessage (10-byte header). The two sockclnt handshake
    messages are swapped in the vendored binapi: sockclnt_create is typed
    as ReplyMessage (6-byte header) even though the client sends it, and
    sockclnt_create_reply is typed as RequestMessage (10-byte header) even
    though the server sends it. Honor the type declarations.
    """
    name = state.id_to_name.get(msg_id)
    if name is None:
        return 10
    kind = state.msg_kind(name) if name in state.messages else "OtherMessage"
    if kind in ("ReplyMessage", "EventMessage"):
        return 6
    if kind == "RequestMessage":
        return 10
    return 2


def serve_client(state, sock):
    """Read frames until the client closes or sends sockclnt_delete."""
    while True:
        body = read_frame(sock)
        if body is None:
            return
        if len(body) < 2:
            raise IOError(f"short request body: {len(body)} bytes")
        (msg_id,) = struct.unpack_from(">H", body, 0)
        header_len = _request_header_len(state, msg_id)
        if len(body) < header_len:
            raise IOError(f"short request body for header: {len(body)} < {header_len}")
        client_index = 0
        if header_len == 10:
            (client_index,) = struct.unpack_from(">I", body, 2)
            (context,) = struct.unpack_from(">I", body, 6)
        elif header_len == 6:
            (context,) = struct.unpack_from(">I", body, 2)
        else:
            context = 0
        payload = body[header_len:]
        name = state.id_to_name.get(msg_id, f"unknown_{msg_id}")
        handler = HANDLERS.get(name)
        if handler is None:
            state.log(name, context, {"client_index": client_index, "unhandled": True})
            # Best-effort: reply with a generic retval=0 if the reply type exists.
            reply_name = f"{name}_reply"
            if reply_name in state.messages:
                body_out = struct.pack(">i", 0)
                reply = build_reply(state, reply_name, context, body_out)
                write_frame(sock, reply)
            continue
        try:
            handler(state, sock, context, payload)
        except _CloseConnection:
            return


class _ReuseAddrUnixServer(socketserver.UnixStreamServer):
    allow_reuse_address = True


def main():
    parser = argparse.ArgumentParser(description="VPP API stub for ze tests")
    parser.add_argument("--socket", required=True, help="Unix socket path to listen on")
    parser.add_argument("--log", required=True, help="JSONL log file path")
    parser.add_argument(
        "--deadline",
        type=float,
        default=30.0,
        help="max lifetime in seconds before self-SIGTERM",
    )
    parser.add_argument(
        "--binapi",
        default=None,
        help="path to vendor/go.fd.io/govpp/binapi (default: auto)",
    )
    parser.add_argument("-v", "--verbose", action="store_true")
    args = parser.parse_args()

    if args.binapi is None:
        here = os.path.dirname(os.path.abspath(__file__))
        repo_root = os.path.abspath(os.path.join(here, "..", ".."))
        args.binapi = os.path.join(repo_root, "vendor", "go.fd.io", "govpp", "binapi")

    # Remove any stale socket file before bind.
    try:
        os.unlink(args.socket)
    except FileNotFoundError:
        pass

    state = StubState(
        args.binapi, args.log, deadline=args.deadline, verbose=args.verbose
    )

    listener = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
    listener.bind(args.socket)
    listener.listen(1)
    os.chmod(args.socket, 0o600)

    stop = threading.Event()

    def _handle_signal(_signum, _frame):
        stop.set()

    signal.signal(signal.SIGTERM, _handle_signal)
    signal.signal(signal.SIGINT, _handle_signal)

    deadline_timer = None
    if args.deadline > 0:

        def _deadline_fire():
            stop.set()

        deadline_timer = threading.Timer(args.deadline, _deadline_fire)
        deadline_timer.daemon = True
        deadline_timer.start()

    if args.verbose:
        sys.stderr.write(
            f"vpp-stub: listening on {args.socket} "
            f"(log={args.log}, deadline={args.deadline}s)\n"
        )
        sys.stderr.flush()

    listener.settimeout(0.2)
    try:
        while not stop.is_set():
            try:
                conn, _addr = listener.accept()
            except socket.timeout:
                continue
            try:
                serve_client(state, conn)
            except (IOError, OSError) as e:
                if args.verbose:
                    sys.stderr.write(f"vpp-stub: client error: {e}\n")
            finally:
                try:
                    conn.close()
                except OSError:
                    pass
    finally:
        if deadline_timer is not None:
            deadline_timer.cancel()
        try:
            listener.close()
        except OSError:
            pass
        try:
            os.unlink(args.socket)
        except FileNotFoundError:
            pass

    return 0


if __name__ == "__main__":
    sys.exit(main())
