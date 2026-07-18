#!/usr/bin/env python3
"""Drive Ze's interactive SSH CLI through a real pseudo-terminal."""

from __future__ import annotations

import argparse
import fcntl
import os
import pty
import re
import select
import signal
import sys
import struct
import termios
import time

ANSI_RE = re.compile(rb"\x1b(?:\[[0-?]*[ -/]*[@-~]|\][^\x07]*(?:\x07|\x1b\\))")
PROMPT_RE = re.compile(rb"ze[>#]")


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--command", action="append", required=True)
    parser.add_argument("--timeout", type=float, default=15.0)
    parser.add_argument("--delay", type=float, default=1.0)
    parser.add_argument("program", nargs=argparse.REMAINDER)
    args = parser.parse_args()
    if args.program and args.program[0] == "--":
        args.program = args.program[1:]
    if not args.program:
        parser.error("program is required after --")
    return args


def read_until(
    fd: int, pattern: re.Pattern[bytes], timeout: float, *, eof_ok: bool = False
) -> bytes:
    deadline = time.monotonic() + timeout
    output = bytearray()
    while time.monotonic() < deadline:
        readable, _, _ = select.select([fd], [], [], min(0.1, deadline - time.monotonic()))
        if not readable:
            continue
        try:
            chunk = os.read(fd, 65536)
        except OSError:
            if eof_ok:
                return bytes(output)
            break
        if not chunk:
            if eof_ok:
                return bytes(output)
            break
        output.extend(chunk)
        if pattern.search(ANSI_RE.sub(b"", output)):
            return bytes(output)
    clean = ANSI_RE.sub(b"", output).decode(errors="replace")
    raise TimeoutError(f"timed out waiting for {pattern.pattern!r}; output: {clean}")


def read_for(fd: int, duration: float) -> bytes:
    deadline = time.monotonic() + duration
    output = bytearray()
    while time.monotonic() < deadline:
        readable, _, _ = select.select([fd], [], [], min(0.1, deadline - time.monotonic()))
        if not readable:
            continue
        try:
            output.extend(os.read(fd, 65536))
        except OSError:
            break
    return bytes(output)


def terminate_process_group(pid: int) -> None:
    try:
        os.killpg(pid, signal.SIGTERM)
    except ProcessLookupError:
        return
    deadline = time.monotonic() + 1.0
    while time.monotonic() < deadline:
        if os.waitpid(pid, os.WNOHANG)[0] == pid:
            return
        time.sleep(0.05)
    try:
        os.killpg(pid, signal.SIGKILL)
    except ProcessLookupError:
        pass
    os.waitpid(pid, 0)


def main() -> int:
    args = parse_args()
    pid, fd = pty.fork()
    if pid == 0:
        os.execvp(args.program[0], args.program)
    fcntl.ioctl(fd, termios.TIOCSWINSZ, struct.pack("HHHH", 40, 120, 0, 0))

    captured = bytearray()
    try:
        captured.extend(read_until(fd, PROMPT_RE, args.timeout))
        for index, command in enumerate(args.command):
            if command == "@escape":
                os.write(fd, b"\x1b")
                if index + 1 < len(args.command):
                    captured.extend(read_for(fd, args.delay))
                else:
                    captured.extend(
                        read_until(
                            fd,
                            re.compile(rb"Connection .* closed|logout"),
                            args.timeout,
                            eof_ok=True,
                        )
                    )
                continue
            if command.startswith("@sleep "):
                captured.extend(read_for(fd, float(command.removeprefix("@sleep "))))
                continue
            os.write(fd, command.encode() + b"\r")
            if index + 1 < len(args.command):
                captured.extend(read_for(fd, args.delay))
            else:
                captured.extend(
                    read_until(
                        fd,
                        re.compile(rb"Connection .* closed|logout"),
                        args.timeout,
                        eof_ok=True,
                    )
                )
    finally:
        try:
            os.close(fd)
        except OSError:
            pass
        terminate_process_group(pid)

    sys.stdout.buffer.write(ANSI_RE.sub(b"", captured))
    return 0


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except (OSError, TimeoutError) as exc:
        print(f"error: {exc}", file=sys.stderr)
        raise SystemExit(1)
