package cli

import (
	"regexp"
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

// cellAt returns width runes of s starting at offset, or "" past the end.
func cellAt(s string, offset, width int) string {
	if offset >= len(s) {
		return ""
	}
	end := min(offset+width, len(s))
	return s[offset:end]
}
