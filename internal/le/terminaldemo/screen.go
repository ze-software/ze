// Design: docs/architecture/core-design.md -- transcript verification reads the terminal grid, not its byte stream
// Overview: render.go -- the transcript gate that consumes painted lines

package terminaldemo

import (
	"sort"
	"strconv"
	"strings"
)

const tabStop = 8

type terminalScreen struct {
	height      int
	width       int
	top         int
	bottom      int
	rows        map[int][]rune
	shown       map[int]string
	row         int
	column      int
	savedRow    int
	savedColumn int
	dirty       map[int]struct{}
	history     []string
}

func newTerminalScreen(height, width int) *terminalScreen {
	return &terminalScreen{
		height:   height,
		width:    width,
		top:      1,
		bottom:   height,
		rows:     map[int][]rune{},
		shown:    map[int]string{},
		row:      1,
		savedRow: 1,
		dirty:    map[int]struct{}{},
	}
}

func (s *terminalScreen) painted() []string {
	lines := append([]string(nil), s.history...)
	for _, row := range sortedRowKeys(s.rows) {
		line := s.line(row)
		if line == "" {
			continue
		}
		if line == s.shown[row] {
			continue
		}
		lines = append(lines, line)
	}
	return lines
}

func (s *terminalScreen) text() string {
	rows := make([]string, 0, len(s.rows))
	for _, row := range sortedRowKeys(s.rows) {
		rows = append(rows, string(s.rows[row]))
	}
	return strings.Join(rows, "\n")
}

func (s *terminalScreen) line(row int) string {
	return strings.TrimRight(string(s.rows[row]), " ")
}

func (s *terminalScreen) settle(text string) {
	if text != "" {
		s.feed(text)
	}
	rows := make([]int, 0, len(s.dirty))
	for row := range s.dirty {
		rows = append(rows, row)
	}
	sort.Ints(rows)
	for _, row := range rows {
		s.show(row)
	}
}

func (s *terminalScreen) feed(text string) {
	runes := []rune(text)
	for index := 0; index < len(runes); {
		character := runes[index]
		if character == '\x1b' {
			index = s.escapeSequence(runes, index)
			continue
		}
		index++
		switch character {
		case '\n':
			s.move(s.row+1, s.column)
		case '\r':
			s.column = 0
		case '\b':
			s.column = max(0, s.column-1)
		case '\t':
			s.column = (s.column/tabStop + 1) * tabStop
		default:
			if character >= ' ' && character != '\x7f' {
				s.put(character)
			}
		}
	}
}

func (s *terminalScreen) escapeSequence(runes []rune, index int) int {
	if index+1 >= len(runes) {
		return index + 1
	}
	next := runes[index+1]
	if next == '[' {
		return s.controlSequence(runes, index)
	}
	if next == ']' {
		at := index + 2
		for at < len(runes) {
			if runes[at] == '\a' {
				return at + 1
			}
			if runes[at] == '\x1b' && at+1 < len(runes) && runes[at+1] == '\\' {
				return at + 2
			}
			at++
		}
		return len(runes)
	}
	if strings.ContainsRune("()*+#%", next) {
		return min(index+3, len(runes))
	}
	s.escape(next)
	return index + 2
}

func (s *terminalScreen) controlSequence(runes []rune, index int) int {
	at := index + 2
	parameterStart := at
	for at < len(runes) {
		character := runes[at]
		if character >= 0x40 && character <= 0x7e {
			parameters := string(runes[parameterStart:at])
			parameterEnd := len(parameters)
			for offset, value := range parameters {
				if value >= 0x20 && value <= 0x2f {
					parameterEnd = offset
					break
				}
			}
			s.control(parameters[:parameterEnd], character)
			return at + 1
		}
		at++
	}
	return len(runes)
}

func (s *terminalScreen) escape(final rune) {
	switch final {
	case 'M':
		if s.row == 1 {
			s.insertLines(1)
			return
		}
		s.move(s.row-1, s.column)
	case 'D':
		s.move(s.row+1, s.column)
	case 'E':
		s.move(s.row+1, 0)
	case '7':
		s.savedRow, s.savedColumn = s.row, s.column
	case '8':
		s.move(s.savedRow, s.savedColumn)
	}
}

func (s *terminalScreen) put(character rune) {
	if s.width > 0 && s.column >= s.width {
		s.move(s.row+1, 0)
	}
	line := s.rows[s.row]
	for len(line) < s.column {
		line = append(line, ' ')
	}
	if s.column < len(line) {
		line[s.column] = character
	} else {
		line = append(line, character)
	}
	s.rows[s.row] = line
	s.column++
	s.dirty[s.row] = struct{}{}
}

func (s *terminalScreen) move(row, column int) {
	row = max(1, row)
	if row != s.row {
		s.show(s.row)
	}
	if s.bottom > 0 && row > s.bottom {
		s.scroll(row - s.bottom)
		row = s.bottom
	}
	s.row = row
	s.column = max(0, column)
}

func (s *terminalScreen) insertLines(count int) {
	top := s.row
	moved := make(map[int][]rune, len(s.rows))
	for row, line := range s.rows {
		if row < top {
			moved[row] = line
			continue
		}
		if s.bottom > 0 && row > s.bottom {
			moved[row] = line
			continue
		}
		if s.bottom > 0 && row+count > s.bottom {
			s.show(row)
			continue
		}
		moved[row+count] = line
	}
	s.rows = moved
	s.discardRowsAtOrBelow(top)
}

func (s *terminalScreen) deleteLines(count int) {
	top := s.row
	for row := top; row < top+count; row++ {
		s.show(row)
	}
	moved := make(map[int][]rune, len(s.rows))
	for row, line := range s.rows {
		if row < top {
			moved[row] = line
			continue
		}
		if s.bottom > 0 && row > s.bottom {
			moved[row] = line
			continue
		}
		if row >= top+count {
			moved[row-count] = line
		}
	}
	s.rows = moved
	s.discardRowsAtOrBelow(top)
}

func (s *terminalScreen) discardRowsAtOrBelow(top int) {
	for row := range s.shown {
		if row >= top {
			delete(s.shown, row)
		}
	}
	for row := range s.dirty {
		if row >= top {
			delete(s.dirty, row)
		}
	}
}

func (s *terminalScreen) scroll(count int) {
	for row := s.top; row < s.top+count; row++ {
		s.show(row)
	}
	moveRow := func(row int) (int, bool) {
		if s.bottom > 0 && (row < s.top || row > s.bottom) {
			return row, true
		}
		if row < s.top+count {
			return 0, false
		}
		return row - count, true
	}
	rows := make(map[int][]rune, len(s.rows))
	for row, line := range s.rows {
		if moved, keep := moveRow(row); keep {
			rows[moved] = line
		}
	}
	s.rows = rows
	shown := make(map[int]string, len(s.shown))
	for row, line := range s.shown {
		if moved, keep := moveRow(row); keep {
			shown[moved] = line
		}
	}
	s.shown = shown
	dirty := make(map[int]struct{}, len(s.dirty))
	for row := range s.dirty {
		if moved, keep := moveRow(row); keep {
			dirty[moved] = struct{}{}
		}
	}
	s.dirty = dirty
}

func (s *terminalScreen) show(row int) {
	if _, changed := s.dirty[row]; !changed {
		return
	}
	delete(s.dirty, row)
	line := s.line(row)
	if line == "" {
		return
	}
	if line == s.shown[row] {
		return
	}
	s.history = append(s.history, line)
	s.shown[row] = line
}

func (s *terminalScreen) eraseRows(rows []int) {
	sort.Ints(rows)
	for _, row := range rows {
		s.show(row)
		delete(s.rows, row)
		delete(s.shown, row)
		delete(s.dirty, row)
	}
}

func (s *terminalScreen) eraseInLine(mode int) {
	line := s.rows[s.row]
	s.dirty[s.row] = struct{}{}
	switch mode {
	case 0:
		line = line[:min(s.column, len(line))]
	case 1:
		for index := range min(s.column+1, len(line)) {
			line[index] = ' '
		}
	default:
		line = nil
	}
	s.rows[s.row] = line
}

func (s *terminalScreen) eraseInDisplay(mode int) {
	rows := make([]int, 0)
	switch mode {
	case 0:
		s.eraseInLine(0)
		for row := range s.rows {
			if row > s.row {
				rows = append(rows, row)
			}
		}
	case 1:
		s.eraseInLine(1)
		for row := range s.rows {
			if row < s.row {
				rows = append(rows, row)
			}
		}
	default:
		for row := range s.rows {
			rows = append(rows, row)
		}
	}
	s.eraseRows(rows)
}

func (s *terminalScreen) control(parameters string, final rune) {
	if strings.HasPrefix(parameters, "?") {
		return
	}
	if strings.HasPrefix(parameters, ">") {
		return
	}
	if strings.HasPrefix(parameters, "<") {
		return
	}
	if strings.HasPrefix(parameters, "=") {
		return
	}
	parts := strings.Split(parameters, ";")
	numbers := make([]int, len(parts))
	for index, part := range parts {
		if part == "" {
			continue
		}
		value, err := strconv.Atoi(part)
		if err == nil {
			numbers[index] = value
		}
	}
	first := 0
	if len(numbers) > 0 {
		first = numbers[0]
	}
	one := first
	if one == 0 {
		one = 1
	}
	switch final {
	case 'H', 'f':
		column := 1
		if len(numbers) > 1 && numbers[1] != 0 {
			column = numbers[1]
		}
		row := first
		if row == 0 {
			row = 1
		}
		s.move(row, column-1)
	case 'A':
		s.move(s.row-one, s.column)
	case 'B', 'e':
		s.move(s.row+one, s.column)
	case 'C', 'a':
		s.column += one
	case 'D':
		s.column = max(0, s.column-one)
	case 'E':
		s.move(s.row+one, 0)
	case 'F':
		s.move(s.row-one, 0)
	case 'G', '`':
		s.column = max(0, one-1)
	case 'd':
		s.move(one, s.column)
	case 'K':
		s.eraseInLine(first)
	case 'J':
		s.eraseInDisplay(first)
	case 'X':
		line := s.rows[s.row]
		for index := s.column; index < min(s.column+one, len(line)); index++ {
			line[index] = ' '
		}
		s.rows[s.row] = line
	case 'L':
		s.insertLines(one)
	case 'M':
		s.deleteLines(one)
	case 'P':
		line := s.rows[s.row]
		start := min(s.column, len(line))
		end := min(start+one, len(line))
		s.rows[s.row] = append(line[:start], line[end:]...)
	case '@':
		line := s.rows[s.row]
		for len(line) < s.column {
			line = append(line, ' ')
		}
		spaces := make([]rune, one)
		for index := range spaces {
			spaces[index] = ' '
		}
		line = append(line, spaces...)
		copy(line[s.column+one:], line[s.column:len(line)-one])
		copy(line[s.column:], spaces)
		s.rows[s.row] = line
	case 'S':
		s.scroll(one)
	case 'r':
		s.top = one
		s.bottom = s.height
		if len(numbers) > 1 && numbers[1] != 0 {
			s.bottom = numbers[1]
		}
	case 's':
		s.savedRow, s.savedColumn = s.row, s.column
	case 'u':
		s.move(s.savedRow, s.savedColumn)
	}
}

func sortedRowKeys[T any](values map[int]T) []int {
	keys := make([]int, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Ints(keys)
	return keys
}
