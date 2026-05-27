#!/usr/bin/env python3
"""BMP collector sidecar used by scenario 44."""

import json
import socket
import time

STATUS = "/tmp/bmp-collector.json"


def recv_exact(conn, size):
    data = b""
    while len(data) < size:
        chunk = conn.recv(size - len(data))
        if not chunk:
            return None
        data += chunk
    return data


def write_status(types):
    with open(STATUS, "w", encoding="utf-8") as fh:
        json.dump({"types": types}, fh)


sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
sock.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
sock.bind(("0.0.0.0", 11019))
sock.listen(1)
sock.settimeout(60)

types = []
deadline = time.time() + 180
try:
    conn, _addr = sock.accept()
    conn.settimeout(120)
    with conn:
        while time.time() < deadline:
            header = recv_exact(conn, 6)
            if header is None:
                break
            length = int.from_bytes(header[1:5], "big")
            msg_type = header[5]
            payload_len = length - 6
            if payload_len < 0:
                break
            payload = recv_exact(conn, payload_len)
            if payload is None:
                break
            types.append(msg_type)
            write_status(types)
            if {0, 3, 4}.issubset(set(types)):
                break
except Exception as exc:
    with open(STATUS, "w", encoding="utf-8") as fh:
        json.dump({"types": types, "error": str(exc)}, fh)
finally:
    sock.close()
    while time.time() < deadline:
        time.sleep(1)
