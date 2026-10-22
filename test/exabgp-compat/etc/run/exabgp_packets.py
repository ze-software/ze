#!/usr/bin/env python3
"""Check the lines a script reads for each BGP message it was fed.

A script whose api block names `packets`, `consolidate` or `parsed` reads one
or two lines per message: the packet (its header and body in hex), the decoded
message, or both on one line. The mock BGP server only sees the wire, so a
script's verdict on what it read reaches the case as a route: the route the
case expects when every line matched, another one when any did not.

The expected lines were read from ExaBGP itself, running the same case.
A text script and a json script share one .run file, so each one names its
encoder from the first line it reads, and the two answer with two routes.
"""

from __future__ import annotations

import json
import os
import sys
import time

from exabgp_api import API

# the members of a json message which change from run to run
VOLATILE = ('time', 'host', 'pid', 'ppid', 'counter')

# the route each encoder announces when it read what it should have, and the
# one it announces when it did not, which no case expects
PASS = {'text': '10.0.1.0/24', 'json': '10.0.2.0/24'}
FAIL = '10.0.9.0/24'


def normalise(line: str) -> str:
    """The line with what changes from run to run removed."""
    if not line.startswith('{'):
        return line
    document = json.loads(line)
    for key in VOLATILE:
        document.pop(key, None)
    return json.dumps(document, sort_keys=True, separators=(',', ':'))


def is_answer(line: str) -> bool:
    """Whether the line answers a command rather than reports a message."""
    if line in ('done', 'error'):
        return True
    return line.startswith('{') and '"answer"' in line


def read_until(api: API, last: tuple[str, ...], timeout: float, on_line=None) -> list[str]:
    """Every message line read up to and including the first holding a `last`.

    A `last` is a word only the line that ends the session's final message
    carries: the decoded half follows the packet line when the two are not
    consolidated, so it is that half which names the message, and a packet
    line alone names it by its type code. Reading stops there, before the next
    session's messages. The empty line ExaBGP writes after a message is no
    message line, so it is skipped. `on_line(api, line)` sees each message line
    as it arrives, for a script which has to make ze send something before it
    can judge.
    """
    lines: list[str] = []
    deadline = time.time() + timeout
    while time.time() < deadline and os.getppid() != 1:
        line = api.read_line(0.1)
        if line is None or not line.strip() or is_answer(line):
            continue
        lines.append(line)
        if on_line is not None:
            on_line(api, line)
        if any(word in line for word in last):
            break
    return lines


def mismatch(api: API, note: str) -> None:
    """Say which line differed, where a reader of a red case looks.

    The case only sees the route the script announces, so the line itself goes
    to stderr, which the daemon logs, and to the trace file when one is named.
    """
    api.trace(note)
    sys.stderr.write(f'api-packets: {note}\n')
    sys.stderr.flush()


def verdict(api: API, encoder: str, expected: list[str], got: list[str]) -> None:
    """Announce the route that tells the case whether `got` was `expected`."""
    wanted = [normalise(line) for line in expected]
    seen = [normalise(line) for line in got]
    api.trace(f'{encoder} read {len(seen)} line(s)')
    for line in seen:
        api.trace(f'{encoder} got {line}')
    route = PASS[encoder] if seen == wanted else FAIL
    if seen != wanted:
        for index in range(max(len(seen), len(wanted))):
            want = wanted[index] if index < len(wanted) else '<nothing>'
            have = seen[index] if index < len(seen) else '<nothing>'
            if want != have:
                mismatch(api, f'{encoder} line {index}: wanted {want}')
                mismatch(api, f'{encoder} line {index}: got    {have}')
    api.send(f'announce route {route} next-hop 1.1.1.1')


def encoder_of(line: str) -> str:
    return 'json' if line.startswith('{') else 'text'


def run(expected: dict[str, list[str]], last: tuple[str, ...], on_line=None) -> None:
    """Read the messages of the first session, then announce the verdict.

    Every message line up to the first holding a `last` is compared with the
    lines `expected` names for the script's encoder.
    """
    api = API()
    try:
        got = read_until(api, last, 60.0, on_line)
        if not got:
            return
        encoder = encoder_of(got[0])
        verdict(api, encoder, expected[encoder], got)
        api.wait_for_shutdown(timeout=30.0)
    except IOError:
        pass


if __name__ == '__main__':
    sys.exit('exabgp_packets is a module for the api-packets-*.run scripts')
