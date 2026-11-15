// Design: docs/contributing/gh-pages.md -- styled terminal cells preserve the recorded presentation.
package siteterminaldemo

import (
	"fmt"
	"strconv"
	"strings"
)

// A cell keeps its own style because cursor movement and scrolls repaint old
// text after the active SGR state has changed. Zero cells are blank and unused.
type terminalCell struct {
	character rune
	style     terminalStyle
}

type terminalStyle struct {
	foreground uint32
	background uint32
	bold       bool
	reverse    bool
}

func terminalDefaultStyle() terminalStyle {
	return terminalStyle{foreground: 0xd8dee9, background: 0x0b1425}
}

// erasesBackground identifies erasure whose blank cells must remain visible.
func (style terminalStyle) erasesBackground() bool {
	if style.reverse {
		return true
	}
	return style.background != terminalDefaultStyle().background
}

func terminalLineText(cells []terminalCell) string {
	var line strings.Builder
	for _, cell := range cells {
		line.WriteRune(cell.character)
	}
	return line.String()
}

func terminalBlankCells(count int, style terminalStyle) []terminalCell {
	cells := make([]terminalCell, count)
	for index := range cells {
		cells[index] = terminalCell{character: ' ', style: style}
	}
	return cells
}

// cursor is explicit cursor placement, which cannot scroll a preview's grid.
// Ordinary terminal output still uses move so newline and delayed wrap scroll.
func (s *terminalScreen) cursor(row, column int) {
	if !s.preview {
		s.move(row, column)
		return
	}
	row = min(max(1, row), s.height)
	if row != s.row {
		s.show(s.row)
	}
	s.row = row
	s.column = min(max(0, column), s.width-1)
}

// sgr interprets presentation only. A transcript still reads the same painted
// text, while SVG conversion refuses a presentation it cannot faithfully draw.
func (s *terminalScreen) sgr(numbers []int) {
	for index := 0; index < len(numbers); index++ {
		number := numbers[index]
		switch {
		case number == 0:
			s.style = terminalDefaultStyle()
		case number == 1:
			s.style.bold = true
		case number == 22:
			s.style.bold = false
		case number == 7:
			s.style.reverse = true
		case number == 27:
			s.style.reverse = false
		case number == 39:
			s.style.foreground = terminalDefaultStyle().foreground
		case number == 49:
			s.style.background = terminalDefaultStyle().background
		case number >= 30 && number <= 37:
			s.style.foreground = terminalPalette(number - 30)
		case number >= 90 && number <= 97:
			s.style.foreground = terminalPalette(number - 90 + 8)
		case number >= 40 && number <= 47:
			s.style.background = terminalPalette(number - 40)
		case number >= 100 && number <= 107:
			s.style.background = terminalPalette(number - 100 + 8)
		case number == 38 || number == 48:
			color, consumed, err := terminalExtendedColor(numbers[index+1:])
			if err != nil {
				s.styleError = err
				return
			}
			if number == 38 {
				s.style.foreground = color
			} else {
				s.style.background = color
			}
			index += consumed
		default:
			s.styleError = fmt.Errorf("README terminal preview cannot render SGR %d; select a recording with supported terminal colors", number)
			return
		}
	}
}

func terminalExtendedColor(numbers []int) (uint32, int, error) {
	if len(numbers) < 2 {
		return 0, 0, fmt.Errorf("README terminal preview has an incomplete extended color")
	}
	if numbers[0] == 5 {
		if numbers[1] < 0 {
			return 0, 0, fmt.Errorf("README terminal preview has a negative palette index")
		}
		if numbers[1] > 255 {
			return 0, 0, fmt.Errorf("README terminal preview palette index exceeds 255")
		}
		return terminalPalette(numbers[1]), 2, nil
	}
	if numbers[0] != 2 {
		return 0, 0, fmt.Errorf("README terminal preview has unknown extended color mode %d", numbers[0])
	}
	if len(numbers) < 4 {
		return 0, 0, fmt.Errorf("README terminal preview has an incomplete RGB color")
	}
	for _, channel := range numbers[1:4] {
		if channel < 0 {
			return 0, 0, fmt.Errorf("README terminal preview has a negative RGB channel")
		}
		if channel > 255 {
			return 0, 0, fmt.Errorf("README terminal preview RGB channel exceeds 255")
		}
	}
	return uint32(numbers[1])<<16 | uint32(numbers[2])<<8 | uint32(numbers[3]), 4, nil
}

// terminalPalette is the xterm 256-color palette carried by indexed SGR.
// The caller MUST validate that index lies in [0,255].
func terminalPalette(index int) uint32 {
	if index < 16 {
		return [...]uint32{0x000000, 0xcd0000, 0x00cd00, 0xcdcd00, 0x0000ee, 0xcd00cd, 0x00cdcd, 0xe5e5e5,
			0x7f7f7f, 0xff0000, 0x00ff00, 0xffff00, 0x5c5cff, 0xff00ff, 0x00ffff, 0xffffff}[index]
	}
	if index >= 232 {
		level := uint32(8 + (index-232)*10)
		return level<<16 | level<<8 | level
	}
	levels := [...]uint32{0, 95, 135, 175, 215, 255}
	index -= 16
	return levels[index/36]<<16 | levels[(index/6)%6]<<8 | levels[index%6]
}

// readmeSVGControl bounds cursor expansion before the shared screen performs
// it. The same check applies when an escape sequence crosses event boundaries.
func readmeSVGControl(parameters string) error {
	parameters = strings.TrimLeft(parameters, "?<>=")
	for _, parameter := range strings.FieldsFunc(parameters, func(character rune) bool {
		return character == ';' || character == ':'
	}) {
		number, err := strconv.Atoi(parameter)
		if err != nil {
			return fmt.Errorf("README cast has invalid terminal control %q", parameter)
		}
		if number < 0 {
			return fmt.Errorf("README cast has negative terminal control %d", number)
		}
		if number > readmeSVGEventsMax {
			return fmt.Errorf("README cast terminal control %d exceeds the preview limit", number)
		}
	}
	return nil
}
