// Design: docs/architecture/fleet-interface.md -- geometry of the status donut and shares of a stacked bar
// Related: component_donut.templ -- draws the arcs
// Related: component_usage.templ -- draws the stacked bar and the usage bar

package fleetui

import (
	"math"
	"strconv"
)

// The donut is drawn on a square of donutSize units. Each segment is one
// circle of radius donutRadius whose stroke is dashed so that only its arc
// shows; the ring starts at twelve o'clock and runs clockwise.
const (
	donutSize   = 120
	donutCenter = donutSize / 2
	donutRadius = 46
	// donutGap is the length left empty after each arc when the ring has
	// more than one, so neighbors read as separate segments.
	donutGap = 3.0
	// donutArcMin is the shortest arc drawn for a segment that counts
	// something, so a count of one among thousands is still visible.
	donutArcMin = 2.0
)

// donutCircumference is the length of the whole ring.
const donutCircumference = 2 * math.Pi * donutRadius

// DonutSegmentView is one segment of a donut and one line of its legend. A
// state's segment is drawn in its Tone; any other, a software version for
// example, in the chart colour Color, 1 to 6. Href opens the rows the segment
// counts. Info, when set, explains the word in a tooltip.
type DonutSegmentView struct {
	Word  string
	Href  string
	Info  string
	Count int
	Color int
	Tone  Tone
}

// DonutArc is the drawn part of one segment: its colour class, its length
// along the ring and where it starts, both in the ring's own units.
type DonutArc struct {
	Class  string
	Length float64
	Offset float64
}

// Dash answers the arc as a stroke-dasharray: the arc, then the rest of the
// ring left empty.
func (arc DonutArc) Dash() string {
	return formatUnits(arc.Length) + " " + formatUnits(donutCircumference-arc.Length)
}

// Shift answers the arc's start as a stroke-dashoffset.
func (arc DonutArc) Shift() string {
	return formatUnits(-arc.Offset)
}

// formatUnits writes a length to two decimals, which is finer than a pixel
// at the size a donut is drawn.
func formatUnits(length float64) string {
	return strconv.FormatFloat(length, 'f', 2, 64)
}

// donutColorClass answers the class that colours a segment and its legend
// dot.
func donutColorClass(segment DonutSegmentView) string {
	if segment.Tone != ToneUnspecified {
		return segment.Tone.Class()
	}

	return chartColorClass(segment.Color)
}

// DonutTotal answers the sum of the segments' counts. A count below zero
// counts nothing.
func DonutTotal(segments []DonutSegmentView) int {
	total := 0
	for index := range segments {
		total += max(segments[index].Count, 0)
	}

	return total
}

// DonutArcs answers one arc per segment that counts something, in order. The
// arcs and the gaps after them cover the ring exactly: a single arc is the
// whole ring with no gap, and no arc at all means the total is zero, which
// the donut draws as its empty track.
//
// Every arc is at least donutArcMin long and the rest of the ring is shared
// in proportion to the counts. When the ring cannot hold that many gaps and
// minimum arcs, the arcs are purely proportional and touch.
func DonutArcs(segments []DonutSegmentView) []DonutArc {
	total := DonutTotal(segments)
	if total == 0 {
		return nil
	}

	drawn := 0
	for index := range segments {
		if segments[index].Count > 0 {
			drawn++
		}
	}

	gap, arcMin := donutGap, donutArcMin
	if drawn == 1 {
		gap, arcMin = 0, 0
	}

	shared := donutCircumference - float64(drawn)*(gap+arcMin)
	if shared < 0 {
		gap, arcMin, shared = 0, 0, donutCircumference
	}

	arcs := make([]DonutArc, 0, drawn)
	offset := 0.0
	for index := range segments {
		segment := segments[index]
		if segment.Count <= 0 {
			continue
		}

		length := arcMin + shared*float64(segment.Count)/float64(total)
		arcs = append(arcs, DonutArc{Class: donutColorClass(segment), Length: length, Offset: offset})
		offset += length + gap
	}

	return arcs
}

// StackedShares answers the width of each count as a whole percentage of
// their sum. The shares add up to exactly 100 when the sum is positive: each
// is rounded down and the percent points left over go to the largest counts
// first. With a sum of zero every share is zero, and the bar draws its empty
// track. A count below zero counts nothing.
func StackedShares(counts []int) []int {
	shares := make([]int, len(counts))
	total := 0
	for _, count := range counts {
		total += max(count, 0)
	}

	if total == 0 {
		return shares
	}

	left := widthPercentMax
	for index, count := range counts {
		shares[index] = max(count, 0) * widthPercentMax / total
		left -= shares[index]
	}

	// Rounding down leaves fewer points than there are counts with a
	// remainder, so each pass finds a count that has not taken one yet, and
	// the points go one each to the counts whose share lost most.
	taken := make([]bool, len(counts))
	for ; left > 0; left-- {
		best := 0
		found := false
		for index, count := range counts {
			if taken[index] {
				continue
			}

			if found && stackedRemainder(count, total) <= stackedRemainder(counts[best], total) {
				continue
			}

			best, found = index, true
		}

		taken[best] = true
		shares[best]++
	}

	return shares
}

// stackedRemainder answers what rounding a count's share down left out.
func stackedRemainder(count, total int) int {
	return max(count, 0) * widthPercentMax % total
}

// The usage thresholds: a bar turns to the warning colour at
// usageWarningPercent and to the error colour at usageErrorPercent.
const (
	usageWarningPercent = 80
	usageErrorPercent   = 90
	// usageInsidePercent is the narrowest fill that holds its own figure.
	usageInsidePercent = 15
)

// UsageTone answers the colour of a usage bar: the accent below 80%, the
// warning colour from 80% and the error colour from 90%.
func UsageTone(percent int) Tone {
	if percent >= usageErrorPercent {
		return ToneError
	}

	if percent >= usageWarningPercent {
		return ToneWarning
	}

	return ToneAccent
}
