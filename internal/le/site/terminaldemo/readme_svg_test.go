// Design: docs/contributing/gh-pages.md -- the README replays verified terminal events as SVG.
package siteterminaldemo

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
	"testing"
)

// TestReadmeSVGReplaysRecordedFrames checks event timing, real terminal text,
// deterministic conversion and provenance through the retained-asset API.
func TestReadmeSVGReplaysRecordedFrames(t *testing.T) {
	asset := readmeSVGTestAsset(t, 20, 4,
		[]any{0, "o", "ze> show bgp"},
		[]any{2, "o", "\r\npeer Established"},
		[]any{4, "i", "\r"})
	content := readmeSVGTestContent(t, asset)
	frames := readmeSVGTestFrames(t, content)
	if len(frames) != 2 {
		t.Fatalf("got %d frames, want two distinct recorded screens", len(frames))
	}
	if frames[0].at != "0" || frames[1].at != "2" {
		t.Fatalf("recorded timing changed: %+v", frames)
	}
	if !strings.Contains(frames[0].text, "ze> show bgp") {
		t.Fatalf("opening command absent: %+v", frames[0])
	}
	if !strings.Contains(frames[1].text, "peer Established") {
		t.Fatalf("recorded answer absent: %+v", frames[1])
	}
	for _, required := range []string{"@keyframes", "4s", "prefers-reduced-motion", "viewBox=", "<title", asset.Metadata().SHA256} {
		if !bytes.Contains(content, []byte(required)) {
			t.Errorf("SVG does not carry %q", required)
		}
	}
	for _, forbidden := range []string{"<script", "<foreignObject", "http://", "https://", "url("} {
		if forbidden == "http://" {
			continue // The SVG namespace is required and is not an external asset.
		}
		if bytes.Contains(content, []byte(forbidden)) {
			t.Errorf("SVG loads executable or external content: %q", forbidden)
		}
	}
	again := readmeSVGTestContent(t, asset)
	if !bytes.Equal(content, again) {
		t.Fatal("identical retained bytes produced a different SVG")
	}
	changed := readmeSVGTestAsset(t, 20, 4,
		[]any{0, "o", "ze> show bgp"},
		[]any{2, "o", "\r\npeer Idle"},
		[]any{4, "i", "\r"})
	updated := readmeSVGTestContent(t, changed)
	if bytes.Equal(content, updated) {
		t.Fatal("a changed recording produced an unchanged SVG")
	}
}

// TestReadmeSVGKeepsCursorEraseAndColor checks terminal state rather than
// concatenated event text, including markup escaping and foreground/background.
func TestReadmeSVGKeepsCursorEraseAndColor(t *testing.T) {
	asset := readmeSVGTestAsset(t, 20, 4,
		[]any{0, "o", "\x1b[38;5;196;48;5;21;1m<old>&\x1b[0m"},
		[]any{1, "o", "\r\x1b[2K\x1b[38;5;82mnew\x1b[0m"},
		[]any{2, "i", "\r"})
	content := readmeSVGTestContent(t, asset)
	frames := readmeSVGTestFrames(t, content)
	if len(frames) != 2 {
		t.Fatalf("got %d frames, want two", len(frames))
	}
	if !strings.Contains(frames[0].text, "<old>&") {
		t.Fatalf("markup characters were not retained: %+v", frames[0])
	}
	if strings.Contains(frames[1].text, "old") {
		t.Fatalf("erased text survived: %+v", frames[1])
	}
	if !strings.Contains(frames[1].text, "new") {
		t.Fatalf("replacement text absent: %+v", frames[1])
	}
	for _, required := range []string{"#ff0000", "#0000ff", "#5fff00", "font-weight=\"bold\""} {
		if !bytes.Contains(content, []byte(required)) {
			t.Errorf("recorded style absent: %q", required)
		}
	}
}

// TestReadmeSVGDeduplicatesHeldScreens verifies repeated paint and non-output
// events retain their duration without duplicating a screen.
func TestReadmeSVGDeduplicatesHeldScreens(t *testing.T) {
	asset := readmeSVGTestAsset(t, 20, 4,
		[]any{0, "o", "ready"},
		[]any{1, "o", "\rready"},
		[]any{2, "o", "\rdone "},
		[]any{3, "i", "x"})
	content := readmeSVGTestContent(t, asset)
	if frames := readmeSVGTestFrames(t, content); len(frames) != 2 {
		t.Fatalf("got %d frames for two distinct screens", len(frames))
	}
}

// TestReadmeSVGRefusesInvalidOrUnboundedRecordings checks the parser's public
// boundary with malformed input, invalid time order and all expansion limits.
func TestReadmeSVGRefusesInvalidOrUnboundedRecordings(t *testing.T) {
	for name, content := range map[string]string{
		"not JSON":        "not a cast",
		"wrong version":   "{\"version\":1,\"width\":20,\"height\":4}\n[1,\"o\",\"x\"]\n",
		"oversized grid":  "{\"version\":2,\"width\":201,\"height\":4}\n[1,\"o\",\"x\"]\n",
		"empty grid":      "{\"version\":2,\"width\":0,\"height\":4}\n[1,\"o\",\"x\"]\n",
		"no events":       "{\"version\":2,\"width\":20,\"height\":4}\n",
		"malformed event": "{\"version\":2,\"width\":20,\"height\":4}\n[1,\"o\",42]\n",
		"reversed time":   "{\"version\":2,\"width\":20,\"height\":4}\n[2,\"o\",\"x\"]\n[1,\"o\",\"y\"]\n",
		"negative time":   "{\"version\":2,\"width\":20,\"height\":4}\n[-1,\"o\",\"x\"]\n",
		"excess duration": "{\"version\":2,\"width\":20,\"height\":4}\n[121,\"o\",\"x\"]\n",
		"null time":       "{\"version\":2,\"width\":20,\"height\":4}\n[null,\"o\",\"x\"]\n[2,\"i\",\"\"]\n",
		"large cursor":    "{\"version\":2,\"width\":20,\"height\":4}\n[0,\"o\",\"\\u001b[999999999@\"]\n[2,\"i\",\"\"]\n",
		"input bytes":     strings.Repeat("x", 2*1024*1024+1),
		"events":          "{\"version\":2,\"width\":20,\"height\":4}\n" + strings.Repeat("[1,\"o\",\"x\"]\n", 4097),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := assetFromText("recording.cast", content).ReadmeSVG(); err == nil {
				t.Fatal("accepted invalid or unbounded recording")
			}
		})
	}
	if _, err := (Asset{}).ReadmeSVG(); err == nil {
		t.Fatal("accepted an uninitialized asset")
	}
}

// TestReadmeSVGRefusesExpandedOutput proves the output limit is independent of
// the much smaller input, so repeated full-screen redraws cannot grow unbounded.
func TestReadmeSVGRefusesExpandedOutput(t *testing.T) {
	events := make([][]any, 400)
	for index := range events {
		text := strings.Repeat(string(rune('a'+index%26)), 4000)
		events[index] = []any{float64(index) / 10, "o", "\x1b[H" + text}
	}
	asset := readmeSVGTestAsset(t, 200, 20, events...)
	if len(asset.content) >= 2*1024*1024 {
		t.Fatal("fixture exceeds the input limit and cannot prove output expansion")
	}
	if _, err := asset.ReadmeSVG(); err == nil {
		t.Fatal("accepted an expanded preview beyond 2 MiB")
	}
}

// TestReadmeSVGClampsCursor checks that repeated cursor movement, tabs and
// saved positions cannot grow the grid or move a last-column paint to a new row.
func TestReadmeSVGClampsCursor(t *testing.T) {
	for name, output := range map[string]string{
		"relative and insert": "a" + strings.Repeat("\x1b[4096C", 8) + "\x1b[@Z",
		"absolute":            "a\x1b[1;4096HZ",
		"saved":               "a" + strings.Repeat("\x1b[4096C", 8) + "\x1b7\r\x1b8Z",
		"tab":                 "a\t\t\tZ",
	} {
		t.Run(name, func(t *testing.T) {
			asset := readmeSVGTestAsset(t, 10, 2,
				[]any{0, "o", output}, []any{1, "i", ""})
			frames := readmeSVGTestFrames(t, readmeSVGTestContent(t, asset))
			if got := frames[len(frames)-1].text; got != "a        Z" {
				t.Fatalf("cursor escaped the last column: %q", got)
			}
		})
	}
}

// TestReadmeSVGRefusesInvalidTerminalState rejects scroll margins outside the
// recorded grid and characters XML cannot represent, before publishing an image.
func TestReadmeSVGRefusesInvalidTerminalState(t *testing.T) {
	for name, output := range map[string]string{
		"scroll bottom":    "\x1b[1;4096rX",
		"scroll top":       "\x1b[3;2rX",
		"XML noncharacter": "\uffff",
	} {
		t.Run(name, func(t *testing.T) {
			asset := readmeSVGTestAsset(t, 10, 2,
				[]any{0, "o", output}, []any{1, "i", ""})
			if _, err := asset.ReadmeSVG(); err == nil {
				t.Fatal("accepted terminal state outside the preview contract")
			}
		})
	}
}

// TestReadmeSVGPaintsBlankCellBackgrounds proves erasure creates visible blank
// cells even when those cells did not hold text before the command.
func TestReadmeSVGPaintsBlankCellBackgrounds(t *testing.T) {
	for name, test := range map[string]struct {
		command string
		rects   []string
	}{
		"display": {"\x1b[2J", []string{
			`<rect x="16" y="16" width="100" height="22" fill="#0000ff"/>`,
			`<rect x="16" y="38" width="100" height="22" fill="#0000ff"/>`,
		}},
		"line start": {"\x1b[6G\x1b[1K", []string{
			`<rect x="16" y="16" width="60" height="22" fill="#0000ff"/>`,
		}},
		"characters": {"\x1b[6G\x1b[3X", []string{
			`<rect x="66" y="16" width="30" height="22" fill="#0000ff"/>`,
		}},
	} {
		t.Run(name, func(t *testing.T) {
			asset := readmeSVGTestAsset(t, 10, 2,
				[]any{0, "o", "\x1b[48;5;21m" + test.command}, []any{1, "i", ""})
			content := readmeSVGTestContent(t, asset)
			for _, rectangle := range test.rects {
				if !bytes.Contains(content, []byte(rectangle)) {
					t.Errorf("blank-cell background absent: %s", rectangle)
				}
			}
		})
	}
}

// readmeSVGTestAsset builds an asciicast fixture without embedding a second
// rendering path in the tests.
func readmeSVGTestAsset(t *testing.T, width, height int, events ...[]any) Asset {
	t.Helper()
	var content bytes.Buffer
	encoder := json.NewEncoder(&content)
	if err := encoder.Encode(map[string]int{"version": 2, "width": width, "height": height}); err != nil {
		t.Fatal(err)
	}
	for _, event := range events {
		if err := encoder.Encode(event); err != nil {
			t.Fatal(err)
		}
	}
	return assetFromText("recording.cast", content.String())
}

type readmeSVGTestFrame struct {
	at   string
	text string
}

// readmeSVGTestContent verifies that conversion returns a retained publishable
// asset rather than making the caller reopen a generated path.
func readmeSVGTestContent(t *testing.T, asset Asset) []byte {
	t.Helper()
	generated, err := asset.ReadmeSVG()
	if err != nil {
		t.Fatal(err)
	}
	content, err := generated.Text()
	if err != nil {
		t.Fatal(err)
	}
	return []byte(content)
}

// readmeSVGTestFrames parses the actual SVG and retains each timed screen's
// rendered text. Broken XML is a failure, rather than a passing substring check.
func readmeSVGTestFrames(t *testing.T, content []byte) []readmeSVGTestFrame {
	t.Helper()
	decoder := xml.NewDecoder(bytes.NewReader(content))
	var frames []readmeSVGTestFrame
	frameDepth, depth := 0, 0
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			return frames
		}
		if err != nil {
			t.Fatal(fmt.Errorf("invalid SVG: %w", err))
		}
		switch value := token.(type) {
		case xml.StartElement:
			depth++
			for _, attribute := range value.Attr {
				if attribute.Name.Local == "data-frame-at" {
					frames = append(frames, readmeSVGTestFrame{at: attribute.Value})
					frameDepth = depth
				}
			}
		case xml.EndElement:
			if frameDepth == depth {
				frameDepth = 0
			}
			depth--
		case xml.CharData:
			if frameDepth > 0 {
				frames[len(frames)-1].text += string(value)
			}
		}
	}
}
