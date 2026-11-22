// Design: docs/contributing/gh-pages.md -- a compact animated README preview derives from the site's retained cast.
package siteterminaldemo

import (
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"strconv"
	"strings"
	"unicode/utf8"
)

const (
	readmeSVGBytesMax  = 2 * 1024 * 1024
	readmeSVGEventsMax = 4096
	readmeSVGWidthMax  = 200
	readmeSVGHeightMax = 80
	readmeSVGTimeMax   = 120
)

type readmeSVGFrame struct {
	at   float64
	text string
}

// ReadmeSVG converts retained asciicast-v2 bytes into a self-contained animated
// terminal preview. It is safe for concurrent use. Its returned immutable Asset
// uses the existing atomic Publish operation; conversion never reopens a path.
// The input, grid, event count, duration and expanded SVG all have fixed bounds.
func (asset Asset) ReadmeSVG() (Asset, error) {
	content, err := asset.Text()
	if err != nil {
		return Asset{}, err
	}
	if len(content) > readmeSVGBytesMax {
		return Asset{}, errors.New("README recording exceeds 2 MiB; select a compact terminal recording")
	}
	if !utf8.ValidString(content) {
		return Asset{}, errors.New("README recording is not UTF-8")
	}
	decoder := json.NewDecoder(strings.NewReader(content))
	var header struct {
		Version int `json:"version"`
		Width   int `json:"width"`
		Height  int `json:"height"`
	}
	if err := decoder.Decode(&header); err != nil {
		return Asset{}, fmt.Errorf("README cast header: %w", err)
	}
	if header.Version != 2 {
		return Asset{}, errors.New("README preview requires an asciicast-v2 recording")
	}
	if header.Width < 1 {
		return Asset{}, errors.New("README preview terminal width must be between 1 and 200 columns")
	}
	if header.Width > readmeSVGWidthMax {
		return Asset{}, errors.New("README preview terminal width must be between 1 and 200 columns")
	}
	if header.Height < 1 {
		return Asset{}, errors.New("README preview terminal height must be between 1 and 80 rows")
	}
	if header.Height > readmeSVGHeightMax {
		return Asset{}, errors.New("README preview terminal height must be between 1 and 80 rows")
	}
	screen := newTerminalScreen(header.Height, header.Width)
	screen.preview = true
	frames, duration, err := readmeSVGFrames(decoder, screen)
	if err != nil {
		return Asset{}, err
	}
	svg, err := readmeSVGDocument(frames, duration, header.Width, header.Height, asset.metadata.SHA256)
	if err != nil {
		return Asset{}, err
	}
	return assetFromText("preview.svg", svg), nil
}

func readmeSVGFrames(decoder *json.Decoder, screen *terminalScreen) ([]readmeSVGFrame, float64, error) {
	var frames []readmeSVGFrame
	previous, expanded := 0.0, 0
	for count := 0; ; count++ {
		var raw []json.RawMessage
		if err := decoder.Decode(&raw); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, 0, fmt.Errorf("README cast event: %w", err)
		}
		if count >= readmeSVGEventsMax {
			return nil, 0, errors.New("README recording exceeds 4096 events; select a compact terminal recording")
		}
		if len(raw) != 3 {
			return nil, 0, errors.New("README cast event must contain time, kind and text")
		}
		var at float64
		var kind, text string
		if string(raw[0]) == jsonNull {
			return nil, 0, errors.New("README cast event time must be a number")
		}
		if err := json.Unmarshal(raw[0], &at); err != nil {
			return nil, 0, fmt.Errorf("README cast event time: %w", err)
		}
		if at < previous {
			return nil, 0, errors.New("README cast event times must not decrease and must stay within 120 seconds")
		}
		if at > readmeSVGTimeMax {
			return nil, 0, errors.New("README cast event times must not decrease and must stay within 120 seconds")
		}
		if err := json.Unmarshal(raw[1], &kind); err != nil {
			return nil, 0, fmt.Errorf("README cast event kind: %w", err)
		}
		if string(raw[2]) == jsonNull {
			return nil, 0, errors.New("README cast event text must be a string")
		}
		if err := json.Unmarshal(raw[2], &text); err != nil {
			return nil, 0, fmt.Errorf("README cast event text: %w", err)
		}
		previous = at
		switch kind {
		case "i", "m":
			continue // Input and markers contribute time but do not paint the terminal.
		case "o":
			if !utf8.ValidString(text) {
				return nil, 0, errors.New("README cast output is not UTF-8")
			}
			for _, character := range text {
				switch character {
				case '\ufffe', '\uffff':
					return nil, 0, errors.New("README cast output contains a character XML cannot represent")
				}
			}
		default:
			return nil, 0, fmt.Errorf("README preview cannot replay cast event kind %q", kind)
		}
		if len(frames) == 0 {
			if at > 0 {
				frames = append(frames, readmeSVGFrame{at: 0, text: ""})
			}
		}
		screen.feed(text)
		if screen.styleError != nil {
			return nil, 0, screen.styleError
		}
		painted := readmeSVGScreen(screen)
		// Transcript history is not part of a screen preview. Do not retain it
		// while replaying cursor edits or scrolling in the bounded grid.
		screen.history = nil
		if len(frames) > 0 {
			last := &frames[len(frames)-1]
			if painted == last.text {
				continue
			}
			if at == last.at {
				expanded -= len(last.text)
				frames = frames[:len(frames)-1]
			}
		}
		expanded += len(painted)
		if expanded > readmeSVGBytesMax {
			return nil, 0, errors.New("README preview expands beyond 2 MiB; select a shorter terminal recording")
		}
		frames = append(frames, readmeSVGFrame{at: at, text: painted})
	}
	if len(frames) == 0 {
		return nil, 0, errors.New("README cast has no recorded terminal output")
	}
	if previous <= 0 {
		return nil, 0, errors.New("README cast has no positive playback duration")
	}
	if frames[len(frames)-1].at == previous {
		// A cast player stops on its final screen. A looping image needs a
		// readable final hold before it returns to the opening command.
		previous += 2
	}
	return frames, previous, nil
}

func readmeSVGScreen(screen *terminalScreen) string {
	var painted strings.Builder
	for _, row := range sortedRowKeys(screen.rows) {
		cells := screen.rows[row]
		for start := 0; start < len(cells); {
			end := start + 1
			for end < len(cells) && cells[end].style == cells[start].style {
				end++
			}
			style := cells[start].style
			foreground, background := style.foreground, style.background
			if style.reverse {
				foreground, background = background, foreground
			}
			left, top := 16+start*10, 16+(row-1)*22
			width := (end - start) * 10
			if background != terminalDefaultStyle().background {
				fmt.Fprintf(&painted, `<rect x="%d" y="%d" width="%d" height="22" fill="#%06x"/>`, left, top, width, background)
			}
			text := terminalLineText(cells[start:end])
			if strings.TrimSpace(text) == "" {
				start = end
				continue // Blank cells have no glyph; only their background needs paint.
			}
			fmt.Fprintf(&painted, `<text x="%d" y="%d" fill="#%06x" textLength="%d" lengthAdjust="spacingAndGlyphs"`, left, top+17, foreground, width)
			if style.bold {
				painted.WriteString(` font-weight="bold"`)
			}
			painted.WriteByte('>')
			painted.WriteString(html.EscapeString(text))
			painted.WriteString("</text>")
			start = end
		}
	}
	return painted.String()
}

func readmeSVGDocument(frames []readmeSVGFrame, duration float64, width, height int, digest string) (string, error) {
	var svg strings.Builder
	fmt.Fprintf(&svg, `<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 %d %d" role="img" aria-labelledby="title description"><title id="title">Ze terminal demonstration</title><desc id="description">Recorded terminal playback. Open the linked website player for controls and the transcript.</desc><metadata>generated-by:./le site build; source-cast-sha256:%s</metadata>`, width*10+32, height*22+32, width*10+32, height*22+32, html.EscapeString(digest))
	svg.WriteString(`<style>.frame{visibility:hidden}text{font-family:monospace;font-size:16px;white-space:pre}`)
	readable := 0
	for index, frame := range frames {
		if len(frame.text) > len(frames[readable].text) {
			readable = index
		}
		start := frame.at / duration * 100
		finish := 100.0
		if index+1 < len(frames) {
			finish = frames[index+1].at / duration * 100
		}
		fmt.Fprintf(&svg, ".f%d{animation:f%d %ss steps(1,end) infinite}@keyframes f%d{", index, index, readmeSVGNumber(duration), index)
		if start > 0 {
			svg.WriteString("0%{visibility:hidden}")
		}
		fmt.Fprintf(&svg, "%s%%{visibility:visible}", readmeSVGNumber(start))
		if finish < 100 {
			fmt.Fprintf(&svg, "%s%%{visibility:hidden}", readmeSVGNumber(finish))
		} else {
			svg.WriteString("100%{visibility:visible}")
		}
		svg.WriteString("}")
	}
	fmt.Fprintf(&svg, "@media(prefers-reduced-motion:reduce){.frame{animation:none;visibility:hidden}.f%d{visibility:visible}}</style>", readable)
	fmt.Fprintf(&svg, `<rect width="100%%" height="100%%" rx="8" fill="#%06x"/>`, terminalDefaultStyle().background)
	for index, frame := range frames {
		visibility := "hidden"
		if index == readable {
			visibility = "visible"
		}
		fmt.Fprintf(&svg, `<g class="frame f%d" data-frame-at="%s" visibility="%s" xml:space="preserve">%s</g>`, index, readmeSVGNumber(frame.at), visibility, frame.text)
	}
	svg.WriteString("</svg>\n")
	if svg.Len() > readmeSVGBytesMax {
		return "", errors.New("README preview exceeds 2 MiB; select a shorter terminal recording")
	}
	return svg.String(), nil
}

func readmeSVGNumber(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}
