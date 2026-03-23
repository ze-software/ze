// Design: (none -- new tool, predates documentation)
package report

import (
	"fmt"
	"io"

	"codeberg.org/thomas-mangin/ze/internal/perf"
)

const htmlHeader = `<html>
<head>
<title>Ze Performance Report</title>
<style>
body { font-family: sans-serif; margin: 2em; }
table { border-collapse: collapse; margin: 1em 0; }
th, td { border: 1px solid #ddd; padding: 8px 12px; text-align: right; }
th { background-color: #f5f5f5; text-align: left; }
tr:nth-child(even) { background-color: #fafafa; }
tr:hover { background-color: #f0f0f0; }
td:first-child, th:first-child { text-align: left; }
td:nth-child(2), th:nth-child(2) { text-align: left; }
h2 { margin-top: 2em; color: #333; }
svg { margin: 1em 0; }
.best { color: #4CAF50; font-weight: bold; }
.worst { color: #f44336; font-weight: bold; }
</style>
</head>
<body>
<h1>Ze Performance Comparison</h1>
`

const htmlFooter = `</body>
</html>
`

// HTML writes a self-contained HTML comparison report with inline SVG bar charts.
// No external CSS or JS dependencies.
func HTML(results []perf.Result, w io.Writer) error {
	if _, err := fmt.Fprint(w, htmlHeader); err != nil {
		return fmt.Errorf("writing HTML header: %w", err)
	}

	groups := groupResults(results)
	keys := sortedGroupKeys(groups)

	for _, key := range keys {
		header := key.Family
		if key.ForceMP {
			header += " [force-mp]"
		}

		if _, err := fmt.Fprintf(w, "<h2>%s</h2>\n", header); err != nil {
			return fmt.Errorf("writing group header: %w", err)
		}

		grouped := groups[key]
		sortByConvergence(grouped)

		if err := writeHTMLTable(w, grouped); err != nil {
			return err
		}

		if err := writeSVGChart(w, grouped); err != nil {
			return err
		}
	}

	if _, err := fmt.Fprint(w, htmlFooter); err != nil {
		return fmt.Errorf("writing HTML footer: %w", err)
	}

	return nil
}

func writeHTMLTable(w io.Writer, results []perf.Result) error {
	if _, err := fmt.Fprintln(w, "<table>"); err != nil {
		return fmt.Errorf("writing table open: %w", err)
	}

	if _, err := fmt.Fprintln(w, "<tr><th>DUT</th><th>Version</th><th>Convergence</th><th>+/-</th><th>Throughput</th><th>+/-</th><th>p99</th><th>+/-</th></tr>"); err != nil {
		return fmt.Errorf("writing table header: %w", err)
	}

	bestConv, worstConv := minMaxConvergence(results)

	for i := range results {
		convClass := cellClass(results[i].ConvergenceMs, bestConv, worstConv)

		if _, err := fmt.Fprintf(w, "<tr><td>%s</td><td>%s</td><td class=\"%s\">%s</td><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td>%s</td></tr>\n",
			results[i].DUTName,
			results[i].DUTVersion,
			convClass,
			fmtMs(results[i].ConvergenceMs),
			fmtMs(results[i].ConvergenceStddevMs),
			fmtNum(results[i].ThroughputAvg),
			fmtNum(results[i].ThroughputAvgStddev),
			fmtMs(results[i].LatencyP99Ms),
			fmtMs(results[i].LatencyP99StddevMs),
		); err != nil {
			return fmt.Errorf("writing table row: %w", err)
		}
	}

	if _, err := fmt.Fprintln(w, "</table>"); err != nil {
		return fmt.Errorf("writing table close: %w", err)
	}

	return nil
}

func writeSVGChart(w io.Writer, results []perf.Result) error {
	if len(results) == 0 {
		return nil
	}

	const (
		barHeight  = 30
		barGap     = 10
		labelWidth = 120
		maxWidth   = 600
		padding    = 10
	)

	svgHeight := len(results)*(barHeight+barGap) + padding*2
	svgWidth := labelWidth + maxWidth + padding*2

	if _, err := fmt.Fprintf(w, "<svg width=\"%d\" height=\"%d\" xmlns=\"http://www.w3.org/2000/svg\">\n",
		svgWidth, svgHeight); err != nil {
		return fmt.Errorf("writing SVG open: %w", err)
	}

	maxConv := 0
	for i := range results {
		if results[i].ConvergenceMs > maxConv {
			maxConv = results[i].ConvergenceMs
		}
	}

	if maxConv == 0 {
		maxConv = 1
	}

	bestConv, worstConv := minMaxConvergence(results)

	for i := range results {
		y := padding + i*(barHeight+barGap)
		barW := float64(results[i].ConvergenceMs) / float64(maxConv) * float64(maxWidth)

		if barW < 1 {
			barW = 1
		}

		color := "#888"

		switch results[i].ConvergenceMs {
		case bestConv:
			color = "#4CAF50"
		case worstConv:
			color = "#f44336"
		}

		if _, err := fmt.Fprintf(w,
			"  <text x=\"%d\" y=\"%d\" font-size=\"14\" font-family=\"sans-serif\" dominant-baseline=\"middle\">%s</text>\n",
			padding, y+barHeight/2, results[i].DUTName); err != nil {
			return fmt.Errorf("writing SVG label: %w", err)
		}

		if _, err := fmt.Fprintf(w,
			"  <rect x=\"%d\" y=\"%d\" width=\"%.0f\" height=\"%d\" fill=\"%s\" rx=\"3\"/>\n",
			labelWidth+padding, y, barW, barHeight, color); err != nil {
			return fmt.Errorf("writing SVG bar: %w", err)
		}

		if _, err := fmt.Fprintf(w,
			"  <text x=\"%.0f\" y=\"%d\" font-size=\"12\" font-family=\"sans-serif\" dominant-baseline=\"middle\" fill=\"#333\"> %s</text>\n",
			float64(labelWidth+padding)+barW+5, y+barHeight/2, fmtMs(results[i].ConvergenceMs)); err != nil {
			return fmt.Errorf("writing SVG value: %w", err)
		}
	}

	if _, err := fmt.Fprintln(w, "</svg>"); err != nil {
		return fmt.Errorf("writing SVG close: %w", err)
	}

	return nil
}

func minMaxConvergence(results []perf.Result) (min, max int) {
	if len(results) == 0 {
		return 0, 0
	}

	min = results[0].ConvergenceMs
	max = results[0].ConvergenceMs

	for i := 1; i < len(results); i++ {
		if results[i].ConvergenceMs < min {
			min = results[i].ConvergenceMs
		}

		if results[i].ConvergenceMs > max {
			max = results[i].ConvergenceMs
		}
	}

	return min, max
}

func cellClass(value, best, worst int) string {
	if value == best {
		return "best"
	}

	if value == worst && best != worst {
		return "worst"
	}

	return ""
}
