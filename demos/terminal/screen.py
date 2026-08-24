#!/usr/bin/env python3
"""A terminal screen, rebuilt from the bytes a program painted onto it.

A recording is a byte stream, and the stream is NOT what a reader sees: a
program that repaints writes over what it wrote before, and the cursor is what
says where. Two readers of this file both need the resolved answer rather than
the stream.

`pty-session.py` needs it because a tape's `Wait+Screen` names something the
SCREEN shows. Typing "show" into the Ze launcher paints "filter: sh", then "o"
at column 11, then "w" at column 12, each with the menu footer after it, so the
stream holds "filter: sh ... o ... w" and the screen holds "filter: show". A
recorder that searched the stream would wait for a string no program will ever
emit.

`render.py` needs it because the transcript gate asks what the reader was shown.
Typing "m" into the Ze CLI editor emits "m", then "onitor" as a dim inline
completion, then an erase-to-end-of-line, then a cursor move back over the
completion; seven keystrokes leave "monitoronniittoorrbgpgpp" in the stream
where the screen holds "monitor bgp".

This is deliberately less than a terminal emulator. It models the cursor, the
grid, the scrolling region and the erases, which is what those two questions
need, and it models nothing else: no colours, no character sets, no alternate
buffer, no scrollback of its own. Every mechanism in it was added because a real
demo could not be recorded without it, and `ScreenTest` names which.
"""

import re

# What a terminal is sent, split into the three things it can be: a control
# sequence, an operating-system command, and a two-character escape.
CSI_RE = re.compile(r"\x1b\[([\x30-\x3f]*)[\x20-\x2f]*([\x40-\x7e])")
OSC_RE = re.compile(r"\x1b\][^\x07\x1b]*(?:\x07|\x1b\\)")
# A character set or a DEC private selection: two bytes, and neither of them
# reaches the screen. Matched before the single-byte form below, because the
# byte after the intermediate is a letter that would otherwise be printed.
DESIGNATOR_RE = re.compile(r"\x1b[()*+#%][\x20-\x7e]")
ESCAPE_RE = re.compile(r"\x1b[\x20-\x7e]")
TAB_STOP = 8


class Screen:
    """What the terminal holds now, and every line it has held.

    A line joins the history each time the cursor leaves it, each time the
    screen is erased under it, each time it scrolls off the top, and at every
    `settle` a caller declares, unless it is unchanged since it last did.

    `settle` is what catches a line the cursor never leaves: the Ze editor
    repaints its prompt row where it stands, so "ze# commit" is replaced by the
    answer to it without the cursor going anywhere. A caller settles between two
    BURSTS of output, where a burst is one read from the terminal, because that
    is when the screen stops moving and is therefore what a reader saw. Nothing
    about where a repaint starts is guessed at, and there is nothing to tune.

    So the history carries what scrolled away and what was drawn over, in the
    order it was shown, which is what a question about the whole session needs.
    `text` carries only what is on the screen, which is what a question about
    this moment needs.

    A height makes the screen a screen: without one the rows run on for the
    whole session, and "what is on the screen" becomes "everything that ever
    was". A width makes a line a line: without one a line runs as far as a
    program paints it, where a terminal would have wrapped it and a reader
    would have read two. Give it the grid the terminal was opened with.
    """

    def __init__(self, height: int | None = None, width: int | None = None) -> None:
        self._height = height
        self._width = width
        # The rows a scroll moves. The Ze editor sets one over the answer area
        # and scrolls inside it, which leaves its own status line and prompt
        # where they are; a scroll of the whole screen would carry them off.
        self._top = 1
        self._bottom = height
        self._rows: dict[int, list[str]] = {}
        self._shown: dict[int, str] = {}
        self._row = 1
        self._column = 0
        self._saved = (1, 0)
        # Rows written since they were last put in the history. A row that is
        # not dirty needs no join to know it would add nothing, and the check
        # below runs once per overwritten character.
        self._dirty: set[int] = set()
        self.history: list[str] = []

    # -- reading ---------------------------------------------------------

    def text(self) -> str:
        """The screen as it stands, one line per row it holds.

        Trailing blanks are KEPT, because a screen row is blank to its right
        edge and a search over the screen can name that: `Wait+Screen /\\$ /`
        is how six tapes wait for the container shell, and its prompt is a
        dollar and a SPACE.
        """
        return "\n".join("".join(self._rows[row]) for row in sorted(self._rows))

    def painted(self) -> list[str]:
        """Every line the screen has shown, oldest first, blanks dropped."""
        lines = list(self.history)
        for row in sorted(self._rows):
            line = self._line(row)
            if line and line != self._shown.get(row):
                lines.append(line)
        return lines

    def _line(self, row: int) -> str:
        """One row of the history, where a trailing blank says nothing."""
        return "".join(self._rows.get(row, [])).rstrip()

    # -- writing ---------------------------------------------------------

    def settle(self, text: str = "") -> None:
        """Paint a burst, then remember every row it left changed.

        A burst is one read from the terminal. Between two of them the screen
        stops moving, so what it holds is what a reader was shown, whether or
        not the cursor ever left the rows that moved.
        """
        if text:
            self.feed(text)
        for row in sorted(self._dirty):
            self._show(row)

    def feed(self, text: str) -> None:
        """Paint everything a terminal was sent, in the order it was sent."""
        index = 0
        while index < len(text):
            character = text[index]
            if character == "\x1b":
                control = CSI_RE.match(text, index)
                if control is not None:
                    self._control(control.group(1), control.group(2))
                    index = control.end()
                    continue
                for pattern in (OSC_RE, DESIGNATOR_RE):
                    skipped = pattern.match(text, index)
                    if skipped is not None:
                        index = skipped.end()
                        break
                else:
                    simple = ESCAPE_RE.match(text, index)
                    if simple is None:
                        index += 1
                        continue
                    self._escape(simple.group()[1])
                    index = simple.end()
                continue
            index += 1
            if character == "\n":
                self._move(self._row + 1, self._column)
            elif character == "\r":
                self._column = 0
            elif character == "\b":
                self._column = max(0, self._column - 1)
            elif character == "\t":
                self._column = (self._column // TAB_STOP + 1) * TAB_STOP
            elif character >= " " and character != "\x7f":
                self._put(character)

    def _escape(self, final: str) -> None:
        """The single-byte escapes, of which a TUI uses three.

        "ESC M" is what the Ze launcher paints its filtered list with: it moves
        the cursor UP a row, which is how a program rewrites the line above the
        one it is on without addressing it.
        """
        if final == "M":
            if self._row == 1:
                self._insert_lines(1)
            else:
                self._move(self._row - 1, self._column)
        elif final == "D":
            self._move(self._row + 1, self._column)
        elif final == "E":
            self._move(self._row + 1, 0)
        elif final == "7":
            self._saved = (self._row, self._column)
        elif final == "8":
            self._move(*self._saved)

    def _insert_lines(self, count: int) -> None:
        """Open `count` blank rows at the cursor, pushing the rest down.

        Rows pushed past the BOTTOM OF THE REGION leave the screen, not rows
        pushed past the bottom of the screen: a program that set a region
        expects its own status line below it to stay where it put it.
        """
        top = self._row
        bottom = self._bottom
        moved = {row: line for row, line in self._rows.items() if row < top}
        for row, line in self._rows.items():
            if row < top:
                continue
            if bottom is not None and row > bottom:
                moved[row] = line
                continue
            if bottom is not None and row + count > bottom:
                self._show(row)
                continue
            moved[row + count] = line
        self._rows = moved
        self._shown = {row: line for row, line in self._shown.items() if row < top}
        self._dirty = {row for row in self._dirty if row < top}

    def _delete_lines(self, count: int) -> None:
        """Take `count` rows out at the cursor, pulling the rest up.

        Only rows inside the region move up, for the same reason `_insert_lines`
        leaves the ones below it alone.
        """
        top = self._row
        bottom = self._bottom
        for row in range(top, top + count):
            self._show(row)
        moved = {row: line for row, line in self._rows.items() if row < top}
        for row, line in self._rows.items():
            if bottom is not None and row > bottom:
                moved[row] = line
            elif row >= top + count:
                moved[row - count] = line
        self._rows = moved
        self._shown = {row: line for row, line in self._shown.items() if row < top}
        self._dirty = {row for row in self._dirty if row < top}

    def _put(self, character: str) -> None:
        if self._width is not None and self._column >= self._width:
            self._move(self._row + 1, 0)
        line = self._rows.setdefault(self._row, [])
        while len(line) < self._column:
            line.append(" ")
        if self._column < len(line):
            line[self._column] = character
        else:
            line.append(character)
        self._column += 1
        self._dirty.add(self._row)

    def _move(self, row: int, column: int) -> None:
        row = max(1, row)
        if row != self._row:
            self._show(self._row)
        bottom = self._bottom
        if bottom is not None and row > bottom:
            self._scroll(row - bottom)
            row = bottom
        self._row = row
        self._column = max(0, column)

    def _scroll(self, count: int) -> None:
        """Move the scrolling region up, showing what leaves the top of it."""
        top = self._top
        bottom = self._bottom
        for row in range(top, top + count):
            self._show(row)

        def moved(row: int) -> int | None:
            if bottom is not None and (row < top or row > bottom):
                return row
            if row < top + count:
                return None
            return row - count

        self._rows = {
            new: line
            for new, line in ((moved(row), line) for row, line in self._rows.items())
            if new is not None
        }
        self._shown = {
            new: line
            for new, line in ((moved(row), line) for row, line in self._shown.items())
            if new is not None
        }
        self._dirty = {new for new in map(moved, self._dirty) if new is not None}

    def _show(self, row: int) -> None:
        if row not in self._dirty:
            return
        self._dirty.discard(row)
        line = self._line(row)
        if line and line != self._shown.get(row):
            self.history.append(line)
            self._shown[row] = line

    def _erase_rows(self, rows: list[int]) -> None:
        for row in sorted(rows):
            self._show(row)
            self._rows.pop(row, None)
            self._shown.pop(row, None)
            self._dirty.discard(row)

    def _erase_in_line(self, mode: int) -> None:
        line = self._rows.setdefault(self._row, [])
        self._dirty.add(self._row)
        if mode == 0:
            del line[self._column :]
        elif mode == 1:
            for index in range(min(self._column + 1, len(line))):
                line[index] = " "
        else:
            line.clear()

    def _erase_in_display(self, mode: int) -> None:
        if mode == 0:
            self._erase_in_line(0)
            self._erase_rows([row for row in self._rows if row > self._row])
        elif mode == 1:
            self._erase_in_line(1)
            self._erase_rows([row for row in self._rows if row < self._row])
        else:
            self._erase_rows(list(self._rows))

    def _control(self, parameters: str, final: str) -> None:
        # A private sequence (DEC mode, cursor visibility, alternate screen)
        # changes how the terminal behaves rather than what it holds, and
        # nothing here models behaviour.
        if parameters.startswith(("?", ">", "<", "=")):
            return
        numbers = [int(part) if part.isdigit() else 0 for part in parameters.split(";")]
        first = numbers[0] if numbers else 0
        if final in "Hf":
            column = (numbers[1] if len(numbers) > 1 else 0) or 1
            self._move(first or 1, column - 1)
        elif final == "A":
            self._move(self._row - (first or 1), self._column)
        elif final in "Be":
            self._move(self._row + (first or 1), self._column)
        elif final in "Ca":
            self._column += first or 1
        elif final == "D":
            self._column = max(0, self._column - (first or 1))
        elif final == "E":
            self._move(self._row + (first or 1), 0)
        elif final == "F":
            self._move(self._row - (first or 1), 0)
        elif final in "G`":
            self._column = max(0, (first or 1) - 1)
        elif final == "d":
            self._move(first or 1, self._column)
        elif final == "K":
            self._erase_in_line(first)
        elif final == "J":
            self._erase_in_display(first)
        elif final == "X":
            line = self._rows.setdefault(self._row, [])
            for index in range(
                self._column, min(self._column + (first or 1), len(line))
            ):
                line[index] = " "
        elif final == "L":
            self._insert_lines(first or 1)
        elif final == "M":
            self._delete_lines(first or 1)
        elif final == "P":
            line = self._rows.setdefault(self._row, [])
            del line[self._column : self._column + (first or 1)]
        elif final == "@":
            line = self._rows.setdefault(self._row, [])
            while len(line) < self._column:
                line.append(" ")
            line[self._column : self._column] = " " * (first or 1)
        elif final == "S":
            self._scroll(first or 1)
        elif final == "r":
            self._top = first or 1
            self._bottom = (numbers[1] if len(numbers) > 1 else 0) or self._height
        elif final == "s":
            self._saved = (self._row, self._column)
        elif final == "u":
            self._move(*self._saved)
