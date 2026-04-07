// Design: (none -- build tool)
//
// Generate the colored HTML comparison page from docs/comparison.md.
//
// Usage: go run scripts/codegen/comparison_html.go [input.md] [output.html]
// Defaults: docs/comparison.md -> docs/comparison.html
//
// Replaces the previous Python implementation (scripts/comparison-html.py).
// Output is byte-equivalent.

//go:build ignore

package main

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

const css = `:root {
  --bg: #1a1b26; --surface: #24283b; --text: #c0caf5; --muted: #565f89;
  --border: #3b4261; --green: #9ece6a; --green-bg: #1a2e1a;
  --red: #f7768e; --red-bg: #2d1a1e; --yellow: #e0af68; --yellow-bg: #2d2a1a;
  --blue: #7aa2f7; --blue-bg: #1a1e2d; --purple: #bb9af7; --accent: #7dcfff;
}
* { margin: 0; padding: 0; box-sizing: border-box; }
body { font-family: "Inter",-apple-system,BlinkMacSystemFont,"Segoe UI",Roboto,sans-serif;
  background: var(--bg); color: var(--text); line-height: 1.6; padding: 2rem;
  max-width: 1400px; margin: 0 auto; }
h1 { font-size: 2rem; font-weight: 700; margin-bottom: .25rem; color: var(--accent); }
.subtitle { color: var(--muted); font-size: .9rem; margin-bottom: 2rem; }
h2 { font-size: 1.3rem; font-weight: 600; margin: 2.5rem 0 .75rem 0;
  color: var(--purple); border-bottom: 1px solid var(--border); padding-bottom: .4rem; }
p.note { color: var(--muted); font-size: .85rem; margin-bottom: 1rem; font-style: italic; }
.table-wrap { overflow-x: auto; margin-bottom: 1.5rem; border-radius: 8px;
  border: 1px solid var(--border); }
table { width: 100%; border-collapse: collapse; font-size: .85rem; }
th { background: var(--surface); color: var(--accent); font-weight: 600;
  text-align: center; padding: .6rem .8rem; border-bottom: 2px solid var(--border);
  position: sticky; top: 0; white-space: nowrap; }
th:first-child { text-align: left; color: var(--text); min-width: 220px; }
th.ze { color: var(--green); background: #1e2235;
  border-left: 2px solid var(--accent); border-right: 2px solid var(--accent); }
td { padding: .45rem .8rem; border-bottom: 1px solid var(--border);
  text-align: center; white-space: nowrap; }
td:first-child { text-align: left; color: var(--text); font-weight: 500; }
tr:hover { background: rgba(125,207,255,.04); }
td.yes { color: var(--green); background: var(--green-bg); font-weight: 600; }
td.no { color: var(--red); background: var(--red-bg); }
td.partial { color: var(--yellow); background: var(--yellow-bg); }
td.api { color: var(--blue); background: var(--blue-bg); font-weight: 600; }
td.ze-col { border-left: 2px solid var(--accent); border-right: 2px solid var(--accent); }
.positioning { margin-top: 2.5rem; }
.impl-card { background: var(--surface); border: 1px solid var(--border);
  border-radius: 8px; padding: 1rem 1.2rem; margin-bottom: .75rem; }
.impl-card.ze-card { border-color: var(--accent); border-width: 2px; }
.impl-card strong { color: var(--accent); font-size: 1rem; }
.impl-card.ze-card strong { color: var(--green); }
.impl-card p { color: var(--text); font-size: .85rem; margin-top: .3rem; }
.legend { display: flex; gap: 1.5rem; flex-wrap: wrap; margin-bottom: 1.5rem; font-size: .8rem; }
.legend-item { display: flex; align-items: center; gap: .4rem; }
.legend-swatch { width: 14px; height: 14px; border-radius: 3px; display: inline-block; }
@media (max-width: 768px) { body { padding: 1rem; } h1 { font-size: 1.5rem; }
  table { font-size: .75rem; } td, th { padding: .35rem .5rem; } }
`

const legend = `<div class="legend">
  <div class="legend-item"><span class="legend-swatch" style="background:var(--green-bg);border:1px solid var(--green)"></span> <span style="color:var(--green)">Yes</span></div>
  <div class="legend-item"><span class="legend-swatch" style="background:var(--red-bg);border:1px solid var(--red)"></span> <span style="color:var(--red)">No</span></div>
  <div class="legend-item"><span class="legend-swatch" style="background:var(--yellow-bg);border:1px solid var(--yellow)"></span> <span style="color:var(--yellow)">Partial</span></div>
  <div class="legend-item"><span class="legend-swatch" style="background:var(--blue-bg);border:1px solid var(--blue)"></span> <span style="color:var(--blue)">API</span> <span style="color:var(--muted)">(programmatic)</span></div>
</div>`

var (
	reBold = regexp.MustCompile(`\*\*(.+?)\*\*`)
	reLink = regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)
)

// classify returns the CSS class for a table cell based on its text.
func classify(text string) string {
	v := strings.ToLower(strings.TrimSpace(text))
	switch v {
	case "yes":
		return "yes"
	case "no":
		return "no"
	case "api":
		return "api"
	case "partial":
		return "partial"
	}
	if strings.HasPrefix(v, "rx ") {
		return "partial"
	}
	for _, k := range []string{"decode", "partial", "only"} {
		if strings.Contains(v, k) {
			return "partial"
		}
	}
	return "plain"
}

// inlineMD converts markdown bold and links to HTML.
func inlineMD(s string) string {
	s = reBold.ReplaceAllString(s, "<strong>$1</strong>")
	s = reLink.ReplaceAllString(s, `<a href="$2">$1</a>`)
	return s
}

// splitRow splits a markdown table row into trimmed cells.
func splitRow(line string) []string {
	line = strings.TrimSpace(line)
	line = strings.Trim(line, "|")
	parts := strings.Split(line, "|")
	out := make([]string, len(parts))
	for i, p := range parts {
		out[i] = strings.TrimSpace(p)
	}
	return out
}

func generate(inputPath, outputPath string) error {
	data, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("read %s: %w", inputPath, err)
	}
	lines := strings.Split(string(data), "\n")

	var out []string
	w := func(s string) { out = append(out, s) }

	w("<!DOCTYPE html>")
	w(`<html lang="en">`)
	w("<head>")
	w(`<meta charset="utf-8">`)
	w(`<meta name="viewport" content="width=device-width, initial-scale=1">`)
	w("<title>Ze \u2014 BGP Implementation Comparison</title>")
	w(fmt.Sprintf("<style>\n%s</style>", css))
	w("</head>")
	w("<body>")
	w("<h1>Ze \u2014 BGP Implementation Comparison</h1>")
	w(`<div class="subtitle">Open-source BGP daemon feature matrix</div>`)
	w(legend)

	var (
		inTable       bool
		inPositioning bool
		zeCol         = -1
		noteLines     []string
		para          string
	)

	flushNotes := func() {
		if len(noteLines) > 0 {
			w(fmt.Sprintf(`<p class="note">%s</p>`, strings.Join(noteLines, " ")))
			noteLines = nil
		}
	}

	closeTable := func() {
		if inTable {
			w("</table></div>")
			inTable = false
		}
	}

	flushPara := func() {
		if para != "" {
			isZe := strings.HasPrefix(para, "**Ze**")
			cls := "impl-card"
			if isZe {
				cls = "impl-card ze-card"
			}
			w(fmt.Sprintf(`<div class="%s"><p>%s</p></div>`, cls, inlineMD(para)))
			para = ""
		}
	}

	for i := 0; i < len(lines); i++ {
		line := strings.TrimRight(lines[i], "\n")
		trimmed := strings.TrimSpace(line)

		// Skip title lines.
		if trimmed == "# BGP Implementation Comparison" {
			continue
		}
		if strings.HasPrefix(trimmed, "A feature comparison") || strings.HasPrefix(trimmed, "Last updated") {
			continue
		}

		// Blockquotes: collect > lines into a styled box.
		if strings.HasPrefix(trimmed, ">") {
			bqLines := []string{strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(trimmed, ">"), " "))}
			for i+1 < len(lines) {
				nxt := strings.TrimSpace(strings.TrimRight(lines[i+1], "\n"))
				if !strings.HasPrefix(nxt, ">") {
					break
				}
				bqLines = append(bqLines, strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(nxt, ">"), " ")))
				i++
			}
			text := inlineMD(strings.Join(bqLines, " "))
			w(fmt.Sprintf(`<div class="impl-card"><p>%s</p></div>`, text))
			continue
		}

		// Section headers.
		if strings.HasPrefix(trimmed, "## ") {
			closeTable()
			flushNotes()
			flushPara()
			title := trimmed[3:]
			if title == "Positioning" {
				inPositioning = true
				w(fmt.Sprintf(`<div class="positioning"><h2>%s</h2>`, title))
			} else {
				inPositioning = false
				w(fmt.Sprintf("<h2>%s</h2>", title))
			}
			continue
		}

		// Table separator.
		if strings.HasPrefix(trimmed, "|") && strings.Contains(trimmed, "---") {
			continue
		}

		// Table rows.
		if strings.HasPrefix(trimmed, "|") {
			cells := splitRow(trimmed)

			if !inTable {
				flushNotes()
				inTable = true
				zeCol = -1
				for j, h := range cells {
					if strings.ToLower(h) == "ze" {
						zeCol = j
					}
				}

				row := "<tr>"
				for j, h := range cells {
					if j == zeCol {
						row += fmt.Sprintf(`<th class="ze">%s</th>`, h)
					} else {
						row += fmt.Sprintf("<th>%s</th>", h)
					}
				}
				row += "</tr>"
				w(`<div class="table-wrap"><table>`)
				w(row)
				continue
			}

			// Data row.
			row := "<tr>"
			for j, cell := range cells {
				if j == 0 {
					row += fmt.Sprintf("<td>%s</td>", cell)
					continue
				}
				cls := classify(cell)
				isZe := j == zeCol
				var classes []string
				if cls != "plain" {
					classes = append(classes, cls)
				}
				if isZe {
					classes = append(classes, "ze-col")
				}
				if len(classes) > 0 {
					row += fmt.Sprintf(`<td class="%s">%s</td>`, strings.Join(classes, " "), cell)
				} else {
					row += fmt.Sprintf("<td>%s</td>", cell)
				}
			}
			row += "</tr>"
			w(row)
			continue
		}

		// Empty line.
		if trimmed == "" {
			closeTable()
			flushPara()
			continue
		}

		// Positioning text -- accumulate paragraph.
		if inPositioning && !strings.HasPrefix(trimmed, "|") {
			if para != "" {
				para += " " + trimmed
			} else {
				para = trimmed
			}
			continue
		}

		// Non-table prose (notes before tables).
		if !inTable && !inPositioning {
			noteLines = append(noteLines, trimmed)
			continue
		}
	}

	closeTable()
	flushPara()
	flushNotes()
	if inPositioning {
		w("</div>")
	}

	w("</body>")
	w("</html>")

	output := strings.Join(out, "\n") + "\n"
	if err := os.WriteFile(outputPath, []byte(output), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", outputPath, err)
	}
	fmt.Printf("wrote %s\n", outputPath)
	return nil
}

func main() {
	input := "docs/comparison.md"
	output := "docs/comparison.html"
	if len(os.Args) > 1 {
		input = os.Args[1]
	}
	if len(os.Args) > 2 {
		output = os.Args[2]
	}
	if err := generate(input, output); err != nil {
		fmt.Fprintf(os.Stderr, "comparison_html: %v\n", err)
		os.Exit(1)
	}
}
