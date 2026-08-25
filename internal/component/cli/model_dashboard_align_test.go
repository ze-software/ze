package cli

import (
	"regexp"
	"slices"
	"strings"
	"testing"
)

var zzAnsiRe = regexp.MustCompile("\x1b\\[[0-9;]*m")

// VALIDATES: every column of a `monitor bgp` peer row starts at the same screen
// column as its header, for any peer state.
//
// PREVENTS: the State cell losing its padding. peerColumnValue returns a STYLED
// string for that column -- stateStyled wraps it in an ANSI escape -- and
// textbuf.PadRight measures with utf8.RuneCountInString, which counts those
// escape bytes as content. The count already exceeded the column width, so
// PadRight added no padding at all and every column after State sat left of its
// header: one place for `established`, 11 runes in a 12-wide column, and eight
// for `idle`. The one-place case is what an operator reported, and it reads as a
// header spacing bug rather than as a row that was never padded, which is why
// this test drives BOTH widths. Padding by visible width is the fix; measuring
// the raw string is the defect.
func TestDashboardRowColumnsAlignWithHeader(t *testing.T) {
	t.Parallel()
	for _, state := range []string{"established", "idle", "connecting", "opensent", "stopped"} {
		peers := []dashboardPeer{{Address: "10.0.0.1", RemoteAS: 65001, State: state, Uptime: "1h2m"}}
		table := renderDashboardPeerTable(peers, &dashboardState{}, sortColumnAddress, true, 200, 0)
		lines := strings.Split(table, "\n")
		if len(lines) < 2 {
			t.Fatalf("state %q: table has %d lines, want a header and a row", state, len(lines))
		}
		header := zzAnsiRe.ReplaceAllString(lines[0], "")
		row := zzAnsiRe.ReplaceAllString(lines[1], "")

		// Walk the column starts. The header's first cell carries a sort
		// indicator, so its own text differs; what must match is where every
		// LATER column begins.
		offset := 0
		for i, c := range dashboardColumns {
			if i > 0 {
				hCell := strings.TrimRight(cellAt(header, offset, c.width), " ")
				rCell := strings.TrimRight(cellAt(row, offset, c.width), " ")
				if hCell == "" && rCell == "" {
					continue
				}
				if !strings.HasPrefix(header[min(offset, len(header)):], hCell) ||
					!strings.HasPrefix(row[min(offset, len(row)):], rCell) {
					t.Errorf("state %q column %d: cell does not start at offset %d", state, i, offset)
				}
				// The real assertion: the row's cell must not have run into
				// the column, which is what an unpadded predecessor causes.
				if got := cellAt(row, offset, 1); got == "" {
					t.Errorf("state %q column %d: row is short at offset %d\n  header: %q\n  row:    %q",
						state, i, offset, header, row)
				}
			}
			offset += c.width + 2
		}

		// Uptime is the column the report named. Assert it directly: the row's
		// uptime must begin exactly where the header's does.
		up := 0
		for _, c := range dashboardColumns {
			if c.col == sortColumnUptime {
				break
			}
			up += c.width + 2
		}
		if !strings.HasPrefix(header[min(up, len(header)):], "Uptime") {
			t.Fatalf("state %q: header does not carry Uptime at offset %d: %q", state, up, header)
		}
		if !strings.HasPrefix(row[min(up, len(row)):], "1h2m") {
			t.Errorf("state %q: Uptime value does not start under its header\n  header: %q\n  row:    %q", state, header, row)
		}
	}
}

// VALIDATES: the selected row is covered by the selection background end to
// end, the state keeps its own color on top of it, and an unselected row still
// colors its state.
//
// PREVENTS: two opposite regressions, which is why it asserts both halves.
//
// The first is the selection background dying halfway along the row. It used to
// be applied by wrapping the whole joined row, and `stateStyled` emitted a FULL
// reset after the state text; a reset inside the wrap terminates the
// background, so every column after State fell back to the terminal default and
// rendered black. No alignment test could see it, because escapes move no
// column.
//
// The second is the fix that dropped the color to win the background back.
// Rendering the selected row unstyled removes the black, and it also removes
// the green an operator reads the table by. Both are wrong, and only asserting
// both catches it: the row is now styled PER CELL, so the state cell carries a
// foreground and the selection background together and no cell is left bare.
// Many resets are expected; text between them with no background re-opened is
// the defect.
func TestDashboardSelectedRowKeepsOneStyle(t *testing.T) {
	t.Parallel()
	peers := []dashboardPeer{
		{Address: "10.0.0.1", RemoteAS: 65001, State: "established", Uptime: "1h2m"},
		{Address: "10.0.0.2", RemoteAS: 65002, State: "idle", Uptime: "5m"},
	}
	lines := strings.Split(renderDashboardPeerTable(peers, &dashboardState{}, sortColumnAddress, true, 200, 0), "\n")
	if len(lines) < 3 {
		t.Fatalf("want a header and two rows, got %d lines", len(lines))
	}
	selected, other := lines[1], lines[2]

	// Every printable run in the selected row must sit inside a span that
	// carries the selection background. Cells are styled individually, so the
	// row holds many resets; what must never appear is text between them with
	// no background re-opened, which is the gap an operator sees as black.
	if bare := unstyledRuns(selected); bare != "" {
		t.Errorf("selected row has text outside the selection background (%q), so the highlight breaks there: %q", bare, selected)
	}
	// The selection pins an explicit foreground. Setting a background and
	// leaving the foreground at the terminal default put light text on the
	// light cyan selection: 1.53:1 against the palette the website replays
	// these recordings with, and 4.5:1 is the readable floor.
	if !strings.Contains(selected, ";30;46m") && !strings.Contains(selected, ";46;30m") {
		t.Errorf("selected row does not pin a foreground, so its text keeps the terminal default over the selection background: %q", selected)
	}
	// The state color is dropped on the selected row. It is picked to read
	// against the terminal background, and over the selection it measures
	// 1.29:1, which is the pair an operator reports as unreadable.
	if strings.Contains(selected, ";32;46m") || strings.Contains(selected, ";46;32m") {
		t.Errorf("selected row still paints the state color over the selection background: %q", selected)
	}

	// The unselected row has no wrapping style, so its state color must survive.
	if !strings.Contains(other, "\x1b[") {
		t.Errorf("unselected row lost its state color entirely: %q", other)
	}
}

// cellAt returns width runes of s starting at offset, or "" past the end.
func cellAt(s string, offset, width int) string {
	if offset >= len(s) {
		return ""
	}
	end := min(offset+width, len(s))
	return s[offset:end]
}

// unstyledRuns returns the first run of s that is not inside a span carrying
// the selection background, or "" when every run is covered.
//
// WHITESPACE COUNTS. A gap of two spaces between cells with no background is
// exactly where an operator sees black on a highlighted row, so trimming it
// would make this check blind to the defect it exists for: joining the cells
// with a plain "  " instead of a styled one leaves the row looking striped, and
// an earlier version of this helper passed that mutation.
func unstyledRuns(s string) string {
	const bg = "46"
	inBG := false
	var run strings.Builder
	for i := 0; i < len(s); {
		if s[i] == 0x1b && i+1 < len(s) && s[i+1] == '[' {
			j := i + 2
			for j < len(s) && s[j] != 'm' {
				j++
			}
			if j >= len(s) {
				break
			}
			inBG = slices.Contains(strings.Split(s[i+2:j], ";"), bg)
			if run.Len() > 0 {
				return run.String()
			}
			i = j + 1
			continue
		}
		if !inBG {
			run.WriteByte(s[i])
		}
		i++
	}
	return run.String()
}
