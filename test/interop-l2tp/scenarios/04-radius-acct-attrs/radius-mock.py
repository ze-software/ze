#!/usr/bin/env python3
"""Mock RADIUS server for the subscriber-attribute scenario.

Answers Access-Request with Access-Accept and Accounting-Request with
Accounting-Response, and prints one decoded line per received packet so the
scenario can assert on what ze actually put on the wire. No attributes are
returned in the Access-Accept: the address the accounting records must report
comes from ze's own pool, so a Framed-IP-Address in the reply would let the
test pass on a value the server handed over rather than on the negotiated one.
"""

import hashlib
import os
import socket
import struct
import sys

PORT = int(os.environ.get("RADIUS_PORT", "1812"))
SECRET = os.environ.get("RADIUS_KEY", "testing123").encode()

CODE_ACCESS_REQUEST = 1
CODE_ACCESS_ACCEPT = 2
CODE_ACCOUNTING_REQUEST = 4
CODE_ACCOUNTING_RESPONSE = 5

ATTR_USER_NAME = 1
ATTR_FRAMED_IP_ADDRESS = 8
ATTR_NAS_IP_ADDRESS = 4
ATTR_ACCT_STATUS_TYPE = 40
ATTR_ACCT_SESSION_ID = 44
ATTR_NAS_PORT_ID = 87

ACCT_STATUS = {1: "Start", 2: "Stop", 3: "Interim-Update"}


def parse_attrs(data):
    """RFC 2865 Section 5: Type(1) Length(1) Value(Length-2), repeated."""
    attrs = []
    i = 0
    while i + 2 <= len(data):
        atype = data[i]
        alen = data[i + 1]
        if alen < 2 or i + alen > len(data):
            break
        attrs.append((atype, data[i + 2 : i + alen]))
        i += alen
    return attrs


def response_auth(code, ident, length, req_auth, attrs, secret):
    h = hashlib.md5()
    h.update(bytes([code, ident]))
    h.update(struct.pack("!H", length))
    h.update(req_auth)
    h.update(attrs)
    h.update(secret)
    return h.digest()


def describe(code, attrs):
    """One line per packet, in a shape the scenario greps."""
    fields = []
    for atype, value in attrs:
        if atype == ATTR_NAS_PORT_ID:
            fields.append("NAS-Port-Id=%s" % value.decode("utf-8", "replace"))
        elif atype == ATTR_FRAMED_IP_ADDRESS and len(value) == 4:
            fields.append("Framed-IP-Address=%s" % socket.inet_ntoa(value))
        elif atype == ATTR_NAS_IP_ADDRESS and len(value) == 4:
            fields.append("NAS-IP-Address=%s" % socket.inet_ntoa(value))
        elif atype == ATTR_ACCT_STATUS_TYPE and len(value) == 4:
            n = struct.unpack("!I", value)[0]
            fields.append("Acct-Status-Type=%s" % ACCT_STATUS.get(n, str(n)))
        elif atype == ATTR_ACCT_SESSION_ID:
            fields.append("Acct-Session-Id=%s" % value.decode("utf-8", "replace"))
        elif atype == ATTR_USER_NAME:
            fields.append("User-Name=%s" % value.decode("utf-8", "replace"))
    name = {
        CODE_ACCESS_REQUEST: "Access-Request",
        CODE_ACCOUNTING_REQUEST: "Accounting-Request",
    }.get(code, "code-%d" % code)
    return "RADIUS-RX %s %s" % (name, " ".join(fields))


def main():
    sock = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
    sock.bind(("0.0.0.0", PORT))
    print("radius-mock listening on 0.0.0.0:%d" % PORT, flush=True)

    while True:
        data, addr = sock.recvfrom(4096)
        if len(data) < 20:
            continue
        code = data[0]
        ident = data[1]
        req_auth = data[4:20]
        attrs = parse_attrs(data[20:])
        print(describe(code, attrs), flush=True)

        if code == CODE_ACCESS_REQUEST:
            reply_code = CODE_ACCESS_ACCEPT
        elif code == CODE_ACCOUNTING_REQUEST:
            reply_code = CODE_ACCOUNTING_RESPONSE
        else:
            continue

        resp = bytearray(20)
        resp[0] = reply_code
        resp[1] = ident
        struct.pack_into("!H", resp, 2, 20)
        resp[4:20] = response_auth(reply_code, ident, 20, req_auth, b"", SECRET)
        sock.sendto(bytes(resp), addr)


if __name__ == "__main__":
    sys.exit(main())
