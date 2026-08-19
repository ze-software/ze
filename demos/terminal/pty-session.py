#!/usr/bin/env python3
"""Drive an interactive Ze screen through a real pseudo-terminal.

That is the SSH CLI, and the full-screen programs it and `ze` paint: the
command launcher, the monitor dashboards. A screen that reads single
keystrokes needs "@type" and "@key", because a typed LINE ends in a carriage
return that such a screen reads as an answer.
"""

from __future__ import annotations

import argparse
import fcntl
import math
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
# Default --ready: the SSH CLI prints its prompt when it can take a command.
READY_DEFAULT = rb"ze[>#]"
CLOSE_RE = re.compile(rb"Connection .* closed|logout")
# Directives the command list understands, and whether each takes an argument.
DIRECTIVES = {
    "@escape": False,
    "@sleep": True,
    "@wait": True,
    "@type": True,
    "@key": True,
}
# Keys a full-screen program reads that a line of text cannot carry. A menu
# that filters as you type reads every character as it arrives, so the carriage
# return a typed line ends with is a SELECTION rather than punctuation: "@type"
# is how a filter is entered, and "@key enter" is how it is then answered.
KEYS = {
    "enter": b"\r",
    "up": b"\x1b[A",
    "down": b"\x1b[B",
    "left": b"\x1b[D",
    "right": b"\x1b[C",
    "tab": b"\t",
    "space": b" ",
}
# Directives that read on their own terms and return early. As the LAST command
# they would skip the read-until-the-connection-closes and drop the tail.
NON_TERMINAL_DIRECTIVES = ("@sleep", "@wait")


def split_directive(command: str) -> tuple[str, str] | None:
    """Split a --command into (directive, argument), or None when it is a line.

    Both the argument checks and the dispatch loop call this, and that is the
    point: a guard that tokenises a command differently from the code that runs
    it passes exactly the inputs the runner then mishandles.
    """
    word, _, argument = command.partition(" ")
    if not word.startswith("@"):
        return None
    return word, argument


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "--command",
        action="append",
        required=True,
        help=(
            "a line to type, or one of the directives '@escape', '@sleep <s>', "
            "'@wait <regex>', '@type <text>' and '@key <name>'. A line is typed "
            "with a carriage return after it; '@type' is the same characters "
            "without one, which is what a filter-as-you-type screen reads. Use "
            "'@wait' after any command the NEXT command depends on: it holds "
            "until the regex appears, where --delay only hopes the answer "
            "arrived in time"
        ),
    )
    parser.add_argument("--timeout", type=float, default=15.0)
    parser.add_argument(
        "--ready",
        default=READY_DEFAULT.decode(),
        help=(
            "regex the program prints once it is ready for the first command. "
            "The default is the CLI prompt; a full-screen program that paints "
            "no prompt needs its own first frame named here"
        ),
    )
    parser.add_argument("--delay", type=float, default=1.0)
    parser.add_argument("program", nargs=argparse.REMAINDER)
    args = parser.parse_args()
    if args.program and args.program[0] == "--":
        args.program = args.program[1:]
    if not args.program:
        parser.error("program is required after --")
    try:
        re.compile(args.ready.encode())
    except (re.error, UnicodeEncodeError) as exc:
        parser.error(f"--ready needs a regex, got {args.ready!r}: {exc}")
    for name, seconds in (("--timeout", args.timeout), ("--delay", args.delay)):
        # Same reason as '@sleep' below: a negative or NaN duration reads
        # nothing and an infinite one never returns.
        if not 0 <= seconds < math.inf:
            parser.error(f"{name} needs finite seconds >= 0, got {seconds!r}")
    for command in args.command:
        # A misspelt or malformed directive is typed into the CLI verbatim and
        # the sequence then fails somewhere else, so refuse anything that looks
        # like a directive and is not a usable one.
        parsed = split_directive(command)
        if parsed is None:
            continue
        word, argument = parsed
        if word not in DIRECTIVES:
            parser.error(
                f"unknown directive {word!r}; use " + ", ".join(sorted(DIRECTIVES))
            )
        if DIRECTIVES[word] and not argument.strip():
            parser.error(f"directive {word!r} needs an argument")
        if not DIRECTIVES[word] and argument:
            parser.error(f"directive {word!r} takes no argument")
        if word == "@key" and argument.strip() not in KEYS:
            names = ", ".join(sorted(KEYS))
            parser.error(f"directive '@key' needs one of {names}; got {argument!r}")
        if word == "@sleep":
            try:
                seconds = float(argument)
            except ValueError:
                parser.error(f"directive '@sleep' needs seconds, got {argument!r}")
            # A negative or NaN duration reads nothing and an infinite one never
            # returns, and both look like a directive that ran.
            if not 0 <= seconds < math.inf:
                parser.error(
                    f"directive '@sleep' needs finite seconds >= 0, got {argument!r}"
                )
        if word == "@wait":
            # Compile here so a bad pattern is an argument error, not a
            # traceback out of the middle of a live session. Compile the BYTES
            # form the dispatch loop uses: a str pattern accepts "(?u)", "\N{}"
            # and non-ASCII group names that the bytes compile then refuses.
            try:
                re.compile(argument.encode())
            except (re.error, UnicodeEncodeError) as exc:
                parser.error(
                    f"directive '@wait' needs a regex, got {argument!r}: {exc}"
                )
    tail = split_directive(args.command[-1])
    if tail is not None and tail[0] in NON_TERMINAL_DIRECTIVES:
        # The final command's output is read until the connection closes. These
        # directives return on their own terms, so everything after them,
        # including the close, would be dropped from the capture with no error.
        # Refuse them last rather than lose the tail silently.
        parser.error(
            "the last --command must not be "
            + " or ".join(repr(name) for name in NON_TERMINAL_DIRECTIVES)
        )
    return args


def poll(fd: int, deadline: float) -> bool:
    """Return True when fd is readable, never raising on a passed deadline.

    select.select refuses a negative timeout, and the deadline can pass between
    the caller's loop test and this call.
    """
    readable, _, _ = select.select(
        [fd], [], [], max(0.0, min(0.1, deadline - time.monotonic()))
    )
    return bool(readable)


def read_until(
    fd: int,
    pattern: re.Pattern[bytes],
    timeout: float,
    *,
    eof_ok: bool = False,
    seen: bytes = b"",
) -> bytes:
    """Read until pattern appears, and return only the bytes THIS call read.

    "seen" is output already read by the caller that belongs to the same search
    window. It is searched but never returned, so the caller does not double it
    into its own buffer. Without it a wait placed after any read at all starts
    blind, and an answer that arrived during that read is missed by the search
    that exists to find it.
    """
    deadline = time.monotonic() + timeout
    output = bytearray()
    if pattern.search(ANSI_RE.sub(b"", seen)):
        return b""
    while time.monotonic() < deadline:
        if not poll(fd, deadline):
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
        if pattern.search(ANSI_RE.sub(b"", seen + bytes(output))):
            return bytes(output)
    clean = ANSI_RE.sub(b"", seen + bytes(output)).decode(errors="replace")
    raise TimeoutError(f"timed out waiting for {pattern.pattern!r}; output: {clean}")


def read_for(fd: int, duration: float) -> bytes:
    deadline = time.monotonic() + duration
    output = bytearray()
    while time.monotonic() < deadline:
        if not poll(fd, deadline):
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
    # Offset in "captured" where the window a "@wait" searches begins. It is the
    # moment the directive BEFORE the wait started, never the moment the wait
    # itself starts: output that arrived in between belongs to the answer the
    # wait is looking for, and a search that began later would miss it.
    window = 0
    try:
        captured.extend(read_until(fd, re.compile(args.ready.encode()), args.timeout))
        # The login banner belongs to no directive, so a leading "@wait" must
        # not search it.
        window = len(captured)
        for index, command in enumerate(args.command):
            last = index + 1 == len(args.command)
            directive, argument = split_directive(command) or ("", "")
            following = None if last else split_directive(args.command[index + 1])
            # A following "@wait" already blocks on the answer, so the fixed
            # delay before it buys nothing. Skipping it is speed only: the
            # "window" offset, not this branch, is what keeps the wait correct.
            handing_off = following is not None and following[0] == "@wait"
            if directive == "@wait":
                captured.extend(
                    read_until(
                        fd,
                        re.compile(argument.encode()),
                        args.timeout,
                        seen=bytes(captured[window:]),
                    )
                )
                # A second wait observes what happens after the FRAME this one
                # matched in, never the same window over again. The boundary is
                # the frame rather than the match, so two patterns that must be
                # distinguished inside one frame need one regex, not two waits.
                window = len(captured)
                continue
            window = len(captured)
            if directive == "@escape":
                os.write(fd, b"\x1b")
            elif directive == "@type":
                # No carriage return: the characters are the whole input.
                os.write(fd, argument.encode())
            elif directive == "@key":
                os.write(fd, KEYS[argument.strip()])
            elif directive == "@sleep":
                captured.extend(read_for(fd, float(argument)))
                continue
            else:
                os.write(fd, command.encode() + b"\r")
            if handing_off:
                continue
            if not last:
                captured.extend(read_for(fd, args.delay))
            else:
                captured.extend(
                    read_until(
                        fd,
                        CLOSE_RE,
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
