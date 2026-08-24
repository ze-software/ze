#!/usr/bin/env python3
"""Drive an interactive Ze screen through a real pseudo-terminal.

That is the SSH CLI, and the full-screen programs it and `ze` paint: the
command launcher, the monitor dashboards. A screen that reads single
keystrokes needs "@type" and "@key", because a typed LINE ends in a carriage
return that such a screen reads as an answer.

Two front ends drive that terminal, and they never mix on one run. "--command"
takes the directives a validator writes by hand and prints what the session
produced. "--tape" reads a checked-in demo definition and records the session
as an asciicast v2 file, which is what a demo publishes.
"""

from __future__ import annotations

import argparse
import codecs
import fcntl
import json
import math
import os
import pathlib
import pty
import re
import select
import signal
import sys
import struct
import termios
import time
from typing import Any, NamedTuple

# The sibling screen model. Both of this directory's programs are run by path,
# and both are also LOADED by path from the tests, where the directory is not on
# the import path. Putting it there is what makes the sibling reachable either
# way.
sys.path.insert(0, str(pathlib.Path(__file__).resolve().parent))

import screen  # noqa: E402  reached by the line above

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
ESCAPE = b"\x1b"
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
        "--tape",
        help=(
            "a demo definition to drive and record as an asciicast v2 file. "
            "The tape is read as it is checked in, so the definition a demo "
            "publishes is the same text under either recorder. Takes no "
            "'--command' and no program: the tape names the shell it runs"
        ),
    )
    parser.add_argument(
        "--command",
        action="append",
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
    if args.tape is not None:
        # The tape carries the whole session, so anything the other front end
        # takes is a contradiction rather than an addition. Refuse it here: a
        # "--command" silently dropped would run a session the caller did not
        # ask for and record it as the demo.
        if args.command:
            parser.error("--tape drives a tape; it takes no --command")
        if args.program:
            parser.error("--tape drives a tape; it takes no program after --")
        return args
    if not args.command:
        parser.error("--command is required, unless --tape names a tape to drive")
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


class TapeError(ValueError):
    """A tape line the recorder refuses to guess at.

    Every unknown directive, every unknown "Set" key and every unusable value
    raises this. A line that was skipped instead would record a session the
    demo's definition never described, and no later check could see it: the
    cast would be well formed, and only a person who knew the tape would know
    a step was missing.
    """


# The tape vocabulary, and whether each directive takes an argument. It is
# CLOSED: these thirteen are every directive the checked-in demos use, and
# anything else is refused by name.
TAPE_DIRECTIVES = {
    "Down": False,
    "Enter": False,
    "Escape": False,
    "Hide": False,
    "Left": False,
    "Output": True,
    "Screenshot": True,
    "Set": True,
    "Show": False,
    "Sleep": True,
    "Source": True,
    "Type": True,
    "Wait": True,
}
# What each keystroke directive writes. The named keys come from the map the
# other front end already uses, so the two front ends cannot disagree about
# what "Down" sends.
TAPE_KEYS = {
    "Enter": KEYS["enter"],
    "Escape": ESCAPE,
    "Left": KEYS["left"],
    "Down": KEYS["down"],
}
DURATION_RE = re.compile(r"(\d+(?:\.\d+)?)(ms|s|m)")
DURATION_UNITS = {"ms": 0.001, "s": 1.0, "m": 60.0}


def _duration(value: str) -> float:
    match = DURATION_RE.fullmatch(value.strip())
    if not match:
        raise TapeError(f"needs a duration like '500ms' or '3s', got {value!r}")
    return float(match.group(1)) * DURATION_UNITS[match.group(2)]


def _pixels(value: str) -> int:
    try:
        pixels = int(value.strip())
    except ValueError:
        raise TapeError(f"needs a whole number of pixels, got {value!r}") from None
    if pixels <= 0:
        raise TapeError(f"needs more than 0 pixels, got {value!r}")
    return pixels


def _quoted(value: str) -> str:
    text = value.strip()
    if len(text) < 2 or not text.startswith('"') or not text.endswith('"'):
        raise TapeError(f"needs a double-quoted string, got {value!r}")
    return re.sub(r"\\(.)", r"\1", text[1:-1])


# The "Set" keys, and what each one means to a cast. Seven carry meaning. Two
# of those seven are the terminal GEOMETRY, and they need the other two with
# them: "Set Width" and "Set Height" are PIXELS, so the character grid a cast
# records is derived from that box with the tape's own font size and padding.
# The three that map to None are read and deliberately ignored: a colour theme,
# a font face and a video frame rate say how a recording LOOKS, which the
# player's CSS decides for a cast, and none of them writes a byte to the
# terminal.
TAPE_SET_KEYS = {
    "FontFamily": None,
    "FontSize": _pixels,
    "Framerate": None,
    "Height": _pixels,
    "Padding": _pixels,
    "Shell": _quoted,
    "Theme": None,
    "TypingSpeed": _duration,
    "WaitTimeout": _duration,
    "Width": _pixels,
}
# Used only for a key a tape leaves unset. `common.tape` sets every one of
# them, and every demo sources it, so these values reach no published demo.
TAPE_DEFAULTS: dict[str, Any] = {
    "FontSize": 22,
    "Height": 600,
    "Padding": 60,
    "Shell": "bash",
    "TypingSpeed": 0.05,
    "WaitTimeout": 30.0,
    "Width": 1200,
}
# A cell of the font the tapes name, as a fraction of the font size.
# JetBrains Mono advances 600/1000 of an em per character and stacks
# 1320/1000 of one per line. At `common.tape`'s 20px that is a 12x27 cell, so
# its 1680x1008 box, padded by 17 on each side, holds 137x36 characters and
# fills 1678x1006 of it: the box was chosen for a whole grid, and this
# recovers the grid from the box.
CELL_ADVANCE_RATIO = 0.6
CELL_LINE_RATIO = 1.32
# How long the terminal must stay silent before a "Wait" is satisfied, and the
# longest a settle may take. A "Wait" is satisfied by the RENDERED SCREEN, so
# a program that repaints has painted the text the tape names while the rest
# of the frame is still arriving. Reading on until the output stops is what
# puts the whole frame in the cast rather than the first half of it.
SETTLE_SECONDS = 0.25
SETTLE_LIMIT = 2.0
# Erase the screen and the scrollback, and put the cursor home. Written into
# the cast at a "Show" that follows a hidden `clear`, and matched against the
# hidden output to know that one happened (see CastWriter.show).
ERASE_SCREEN = "\x1b[H\x1b[2J\x1b[3J"
ERASE_SCREEN_RE = re.compile(r"\x1b\[[23]J")


class Tape(NamedTuple):
    """A parsed demo definition: where it writes, how it looks, what it does."""

    output: pathlib.Path | None
    settings: dict[str, Any]
    actions: list[tuple[str, Any, str]]


def _tape_lines(path: pathlib.Path, root: pathlib.Path, seen: set[pathlib.Path]):
    """Yield (where, line) for a tape and, in place, every tape it sources.

    "Source" is resolved against the directory the recorder RUNS in before the
    tape's own directory, because that is the order the tapes need: a render
    drives an accelerated copy written to a scratch directory, and the
    `common.tape` it sources sits beside the original.
    """
    resolved = path.resolve()
    if resolved in seen:
        raise TapeError(f"{path.name}: sources itself")
    seen.add(resolved)
    for number, raw in enumerate(path.read_text(encoding="utf-8").splitlines(), 1):
        line = raw.strip()
        if not line or line.startswith("#"):
            continue
        where = f"{path.name}:{number}"
        word, _, argument = line.partition(" ")
        if word != "Source":
            yield where, line
            continue
        name = argument.strip()
        if not name:
            raise TapeError(f"{where}: directive 'Source' needs a tape to source")
        for candidate in (root / name, path.parent / name):
            if candidate.is_file():
                yield from _tape_lines(candidate, root, seen)
                break
        else:
            raise TapeError(f"{where}: sources {name!r}, which is not a file")


def parse_tape(path: pathlib.Path, root: pathlib.Path | None = None) -> Tape:
    """Read a demo definition, refusing anything it cannot drive faithfully."""
    settings = dict(TAPE_DEFAULTS)
    actions: list[tuple[str, Any, str]] = []
    output: pathlib.Path | None = None
    root = pathlib.Path(root) if root is not None else pathlib.Path.cwd()
    for where, line in _tape_lines(pathlib.Path(path), root, set()):
        word, _, argument = line.partition(" ")
        name, _, modifier = word.partition("+")
        if name not in TAPE_DIRECTIVES:
            known = ", ".join(sorted(TAPE_DIRECTIVES))
            raise TapeError(f"{where}: unknown directive {word!r}; use {known}")
        try:
            if TAPE_DIRECTIVES[name] and not argument.strip():
                raise TapeError(f"directive {name!r} needs an argument")
            if not TAPE_DIRECTIVES[name] and argument.strip():
                raise TapeError(f"directive {name!r} takes no argument")
            if modifier and name != "Wait":
                raise TapeError(f"directive {name!r} takes no {'+' + modifier!r}")
            if name == "Output":
                output = pathlib.Path(argument.strip())
            elif name == "Screenshot":
                # Deliberately ignored. It writes the poster image the video
                # embed needed, and a cast is played without one.
                pass
            elif name == "Set":
                if actions:
                    raise TapeError(
                        "a 'Set' after the session starts is refused: the "
                        "settings are read once, before the terminal exists"
                    )
                _apply_setting(argument, settings)
            else:
                actions.append(
                    (name, _action_argument(name, modifier, argument), where)
                )
        except TapeError as exc:
            raise TapeError(f"{where}: {exc}") from None
    return Tape(output, settings, actions)


def _apply_setting(argument: str, settings: dict[str, Any]) -> None:
    key, _, value = argument.strip().partition(" ")
    if key not in TAPE_SET_KEYS:
        known = ", ".join(sorted(TAPE_SET_KEYS))
        raise TapeError(f"unknown 'Set' key {key!r}; use {known}")
    reader = TAPE_SET_KEYS[key]
    if reader is None:
        return
    if not value.strip():
        raise TapeError(f"'Set {key}' needs a value")
    try:
        settings[key] = reader(value)
    except TapeError as exc:
        raise TapeError(f"'Set {key}' {exc}") from None


def _action_argument(name: str, modifier: str, argument: str) -> Any:
    if name == "Type":
        return _quoted(argument)
    if name == "Sleep":
        return _duration(argument)
    if name != "Wait":
        return None
    if modifier != "Screen":
        seen = f"'Wait+{modifier}'" if modifier else "a bare 'Wait'"
        raise TapeError(
            f"{seen} is not a wait this recorder makes; use 'Wait+Screen /regex/'"
        )
    text = argument.strip()
    if len(text) < 2 or not text.startswith("/") or not text.endswith("/"):
        raise TapeError(f"'Wait+Screen' needs a /regex/, got {argument!r}")
    try:
        return re.compile(re.sub(r"\\/", "/", text[1:-1]).encode())
    except (re.error, UnicodeEncodeError) as exc:
        raise TapeError(
            f"'Wait+Screen' needs a regex, got {argument!r}: {exc}"
        ) from None


def terminal_size(settings: dict[str, Any]) -> tuple[int, int]:
    """Return the (columns, rows) grid the tape's pixel box holds."""
    cell_width = max(1, round(settings["FontSize"] * CELL_ADVANCE_RATIO))
    cell_height = max(1, math.ceil(settings["FontSize"] * CELL_LINE_RATIO))
    columns = (settings["Width"] - 2 * settings["Padding"]) // cell_width
    rows = (settings["Height"] - 2 * settings["Padding"]) // cell_height
    if columns < 1 or rows < 1:
        raise TapeError(
            f"a {settings['Width']}x{settings['Height']} box padded by "
            f"{settings['Padding']} holds no {cell_width}x{cell_height} cell"
        )
    return columns, rows


class CastWriter:
    """An asciicast v2 file whose clock stops between "Hide" and "Show".

    The header carries the grid and the format version, and nothing else. A
    "timestamp" or a "title" would differ on every render, and the artifact is
    committed, so a header field that moves for no reason costs a diff every
    time the demo is recorded again.
    """

    def __init__(self, path: pathlib.Path, columns: int, rows: int) -> None:
        self._handle = path.open("w", encoding="utf-8")
        header = {"version": 2, "width": columns, "height": rows}
        self._handle.write(json.dumps(header) + "\n")
        # A read can split a multi-byte character, so the decoder holds the
        # tail of one chunk until the next arrives. A per-chunk decode would
        # write a replacement character into the middle of a box drawing.
        self._decoder = codecs.getincrementaldecoder("utf-8")(errors="replace")
        self._origin = time.monotonic()
        self._hidden_at: float | None = None
        self._cleared = False
        self._residue: list[str] = []
        self._last = 0.0
        self.events = 0

    @property
    def duration(self) -> float:
        return self._last

    def write(self, data: bytes) -> None:
        """Record output, unless the tape is hiding it.

        Hidden bytes still reach the decoder, so a character split across the
        boundary leaves the recording whole on the other side of it. They are
        also tracked for what the screen holds when the tape shows it again,
        which is the only thing a hidden region leaves behind. See `show`.
        """
        text = self._decoder.decode(data)
        if self._hidden_at is not None:
            self._track_hidden_screen(text)
            return
        if not text:
            return
        self._emit(text)

    def _track_hidden_screen(self, text: str) -> None:
        """Keep what a hidden region left ON the screen, and nothing before it.

        A screen erase throws away everything written before it, so the text
        that follows the LAST erase is the whole of what a reader would be
        looking at when the tape shows again. Everything earlier is the setup
        the tape hid, and it is dropped here rather than remembered.
        """
        erased = None
        for match in ERASE_SCREEN_RE.finditer(text):
            erased = match
        if erased is None:
            self._residue.append(text)
            return
        self._cleared = True
        self._residue = [text[erased.end() :]]

    def _emit(self, text: str) -> None:
        self._last = max(self._last, round(time.monotonic() - self._origin, 6))
        self._handle.write(json.dumps([self._last, "o", text]) + "\n")
        self.events += 1

    def hide(self) -> None:
        if self._hidden_at is None:
            self._hidden_at = time.monotonic()
            self._cleared = False
            self._residue = []

    def show(self) -> None:
        """Resume, give back the hidden time, and hand back the screen.

        The origin moves forward by exactly the hidden duration, so the next
        event lands where the last visible one left off: no gap the length of
        the region, and no timestamp before the one written before it.

        A hidden region that CLEARED the screen leaves a player holding a
        screen the terminal no longer has, and every tape but one clears
        before it shows again: the setup it hides ends in `clear`, or in a
        card whose script starts with one. Written here are the reset and what
        the terminal painted AFTER that last erase, which together are the
        screen the reader resumes on -- the shell prompt, in every tape that
        ends its hidden region with `clear`.

        Nothing erased before the reset is written, which is the setup the tape
        hid. Phase 3 of the spec measured what withholding the residue as well
        costs: `cli-dashboard` showed `sshpass -e ssh ze-demo` typed onto a bare
        line, where the transcript, and the video this replaces, both show it
        behind the `$ ` prompt the clear painted.
        """
        if self._hidden_at is None:
            return
        self._origin += time.monotonic() - self._hidden_at
        self._hidden_at = None
        if self._cleared:
            self._emit(ERASE_SCREEN + "".join(self._residue))
            self._cleared = False
        self._residue = []

    def close(self) -> None:
        self._handle.close()


class TapeSession:
    """The terminal a tape drives, and the screen a "Wait" searches.

    `Wait+Screen` names what the RENDERED SCREEN shows, and the tapes are
    written for that: they name strings a program assembles over several paints,
    and they name them on the screen the whole session has built. So the screen
    here is session-wide and scrolls at the terminal's own height, rather than
    being a window over the last action -- the Ze launcher's breadcrumb reads
    "ze > show" only because "ze" was painted when it started and " > show" when
    a command was picked out of it, whole minutes apart.
    """

    def __init__(
        self,
        fd: int,
        writer: CastWriter,
        settings: dict[str, Any],
        rows: int,
        columns: int,
    ) -> None:
        self._fd = fd
        self._writer = writer
        self._screen = screen.Screen(rows, columns)
        self._typing = settings["TypingSpeed"]
        self._timeout = settings["WaitTimeout"]
        self._decoder = codecs.getincrementaldecoder("utf-8")(errors="replace")
        self._closed = False

    def _read(self) -> None:
        try:
            chunk = os.read(self._fd, 65536)
        except OSError:
            chunk = b""
        if not chunk:
            self._closed = True
            return
        self._writer.write(chunk)
        self._screen.feed(self._decoder.decode(chunk))

    def pump(self, seconds: float) -> None:
        """Record for a while, doing nothing else."""
        deadline = time.monotonic() + seconds
        while not self._closed and time.monotonic() < deadline:
            if poll(self._fd, deadline):
                self._read()

    def settle(self) -> None:
        """Read on until the program stops painting (SETTLE_SECONDS).

        A "Wait" is satisfied by the screen holding a string, and a screen that
        holds it may still be painting the rest of the frame. It is also the
        answer to a match on something that was ALREADY on the screen when the
        action started: reading on to quiet costs one settle and leaves the cast
        holding the frame the tape was waiting for either way.
        """
        limit = time.monotonic() + SETTLE_LIMIT
        quiet = time.monotonic() + SETTLE_SECONDS
        while not self._closed and time.monotonic() < min(quiet, limit):
            before = self._writer.events
            if poll(self._fd, quiet):
                self._read()
            if self._writer.events != before:
                quiet = time.monotonic() + SETTLE_SECONDS

    def wait(self, pattern: re.Pattern[bytes]) -> None:
        """Wait for the SCREEN to show something, which is what a tape means.

        `Wait+Screen` is a match against the rendered screen, so the tapes name
        strings a program assembles across several paints. Rebuilding the screen
        from the byte stream is what makes those strings findable; searching the
        stream finds only the ones a program happened to emit in one piece.
        """
        deadline = time.monotonic() + self._timeout
        while not pattern.search(self._screen.text().encode()):
            if self._closed:
                raise TapeError(
                    f"the terminal closed while waiting for {pattern.pattern!r}"
                )
            if time.monotonic() >= deadline:
                raise TimeoutError(
                    f"timed out waiting for {pattern.pattern!r}; "
                    f"screen: {self._screen.text()}"
                )
            if poll(self._fd, deadline):
                self._read()
        self.settle()

    def send(self, data: bytes) -> None:
        self._write(data)
        self.pump(self._typing)

    def type_text(self, text: str) -> None:
        """Type one character at a time, at the tape's typing speed."""
        for character in text:
            self._write(character.encode())
            self.pump(self._typing)

    def _write(self, data: bytes) -> None:
        if self._closed:
            raise TapeError("the terminal closed before the tape ended")
        os.write(self._fd, data)


def record_tape(tape_path: str) -> int:
    """Drive a demo definition and write the session as an asciicast v2 file."""
    try:
        tape = parse_tape(pathlib.Path(tape_path))
        if tape.output is None:
            raise TapeError(f"{tape_path}: no 'Output': the tape names no artifact")
        columns, rows = terminal_size(tape.settings)
        # The tape's "Output" names a video, because the definitions are not
        # edited by the change of format. What it names is the artifact's
        # directory and stem; the suffix belongs to whatever is recording.
        output = tape.output.with_suffix(".cast")
        output.parent.mkdir(parents=True, exist_ok=True)
        writer = drive_tape(tape, output, columns, rows)
    except (TapeError, TimeoutError, OSError) as exc:
        print(f"error: {exc}", file=sys.stderr)
        return 1
    print(
        f"recorded {output} ({writer.events} events, {writer.duration:.1f}s, "
        f"{columns}x{rows})"
    )
    return 0


def drive_tape(tape: Tape, output: pathlib.Path, columns: int, rows: int) -> CastWriter:
    """Run a parsed tape against a real terminal, recording as it goes."""
    shell = tape.settings["Shell"]
    pid, fd = pty.fork()
    if pid == 0:
        try:
            os.execvp(shell, [shell])
        finally:
            # execvp only returns when it failed, and the child must not fall
            # through into the recorder: two processes would drive one tape.
            os._exit(127)
    fcntl.ioctl(fd, termios.TIOCSWINSZ, struct.pack("HHHH", rows, columns, 0, 0))
    writer = CastWriter(output, columns, rows)
    session = TapeSession(fd, writer, tape.settings, rows, columns)
    try:
        # The shell paints its first prompt before the tape types into it. A
        # tape that typed at once would lose the characters the shell was not
        # yet reading.
        session.settle()
        for name, argument, where in tape.actions:
            try:
                if name == "Hide":
                    writer.hide()
                elif name == "Show":
                    writer.show()
                elif name == "Sleep":
                    session.pump(argument)
                elif name == "Wait":
                    session.wait(argument)
                elif name == "Type":
                    session.type_text(argument)
                else:
                    session.send(TAPE_KEYS[name])
            except TapeError as exc:
                raise TapeError(f"{where}: {exc}") from None
            except TimeoutError as exc:
                raise TimeoutError(f"{where}: {exc}") from None
    finally:
        try:
            os.close(fd)
        except OSError:
            pass
        terminate_process_group(pid)
        writer.close()
    return writer


def main() -> int:
    args = parse_args()
    if args.tape is not None:
        return record_tape(args.tape)
    pid, fd = pty.fork()
    if pid == 0:
        try:
            os.execvp(args.program[0], args.program)
        finally:
            # execvp only returns when it failed, and the child must not fall
            # through into the driver below: two processes would then read one
            # terminal and each would see half the output.
            os._exit(127)
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
