package fleetui

import (
	"math"
	"strings"
	"testing"
)

// unitsTolerance is the rounding error allowed when lengths along the ring
// are added up.
const unitsTolerance = 1e-9

// segmentsOf answers one donut segment per count.
func segmentsOf(counts ...int) []DonutSegmentView {
	segments := make([]DonutSegmentView, len(counts))
	for index, count := range counts {
		segments[index] = DonutSegmentView{Word: "segment", Count: count, Tone: ToneAccent}
	}

	return segments
}

// TestDonutArcsCoverTheRing walks the arcs of rings with several segments.
//
// VALIDATES: the arcs and the gaps after them add up to the whole ring, each
// arc starts where the one before it and its gap end, every arc is at least
// the minimum length, and a larger count never gets a shorter arc.
// PREVENTS: a ring with a hole or an overlap, and a count of one among
// thousands drawn as nothing.
func TestDonutArcsCoverTheRing(t *testing.T) {
	cases := [][]int{
		{36, 3, 1, 3, 1, 0},
		{1, 1},
		{5000, 1},
		{1, 2, 3, 4, 5, 6, 7, 8},
		{0, 7, 0, 7, 0},
	}

	for _, counts := range cases {
		arcs := DonutArcs(segmentsOf(counts...))

		drawn := 0
		for _, count := range counts {
			if count > 0 {
				drawn++
			}
		}

		if len(arcs) != drawn {
			t.Fatalf("%v: %d arcs, want one per counted segment, %d", counts, len(arcs), drawn)
		}

		offset := 0.0
		for index, arc := range arcs {
			if math.Abs(arc.Offset-offset) > unitsTolerance {
				t.Errorf("%v: arc %d starts at %f, want %f", counts, index, arc.Offset, offset)
			}

			if arc.Length < donutArcMin {
				t.Errorf("%v: arc %d is %f long, below the minimum %f", counts, index, arc.Length, donutArcMin)
			}

			offset += arc.Length + donutGap
		}

		if math.Abs(offset-donutCircumference) > unitsTolerance {
			t.Errorf("%v: the arcs and gaps cover %f of a ring of %f", counts, offset, donutCircumference)
		}
	}

	arcs := DonutArcs(segmentsOf(5000, 1))
	if arcs[0].Length <= arcs[1].Length {
		t.Errorf("a count of 5000 got an arc of %f, not longer than the %f of a count of 1", arcs[0].Length, arcs[1].Length)
	}
}

// TestDonutArcsZeroAndSingle covers the two rings with no gap.
//
// VALIDATES: a total of zero draws no arc, so the empty track shows; a
// single counted segment is the whole ring with no gap; counts below zero
// count nothing.
// PREVENTS: a division by a zero total, and a full ring drawn with a notch.
func TestDonutArcsZeroAndSingle(t *testing.T) {
	if arcs := DonutArcs(segmentsOf(0, 0, 0)); len(arcs) != 0 {
		t.Errorf("a total of zero draws %d arcs, want none", len(arcs))
	}

	if arcs := DonutArcs(nil); len(arcs) != 0 {
		t.Errorf("no segment draws %d arcs, want none", len(arcs))
	}

	if arcs := DonutArcs(segmentsOf(-4, 0)); len(arcs) != 0 {
		t.Errorf("a negative count draws %d arcs, want none", len(arcs))
	}

	arcs := DonutArcs(segmentsOf(0, 12, 0))
	if len(arcs) != 1 {
		t.Fatalf("a single counted segment draws %d arcs, want 1", len(arcs))
	}

	if math.Abs(arcs[0].Length-donutCircumference) > unitsTolerance || arcs[0].Offset != 0 {
		t.Errorf("the single arc is %f long from %f, want the whole ring %f from 0", arcs[0].Length, arcs[0].Offset, donutCircumference)
	}

	if dash := arcs[0].Dash(); !strings.HasSuffix(dash, " 0.00") {
		t.Errorf("the single arc's dash is %q, want no empty part", dash)
	}

	if total := DonutTotal(segmentsOf(-4, 3, 2)); total != 5 {
		t.Errorf("the total of -4, 3 and 2 is %d, want 5", total)
	}
}

// TestDonutArcsBeyondTheRingsRoom gives the ring more segments than it can
// separate.
//
// VALIDATES: when gaps and minimum arcs no longer fit, the arcs are purely
// proportional and still cover the ring exactly.
// PREVENTS: arcs running past the end of the ring.
func TestDonutArcsBeyondTheRingsRoom(t *testing.T) {
	counts := make([]int, 100)
	for index := range counts {
		counts[index] = 1
	}

	covered := 0.0
	for _, arc := range DonutArcs(segmentsOf(counts...)) {
		covered += arc.Length
	}

	if math.Abs(covered-donutCircumference) > 1e-6 {
		t.Errorf("100 arcs cover %f of a ring of %f", covered, donutCircumference)
	}
}

// TestDonutLegendListsZeroCounts renders a donut with an empty state.
//
// VALIDATES: a state with a count of zero is still a legend line, and the
// total is written.
// PREVENTS: a legend whose shape changes with the data.
func TestDonutLegendListsZeroCounts(t *testing.T) {
	markup := componentMarkup(t, Donut(DonutView{Caption: "Devices", Icon: "device", Segments: []DonutSegmentView{
		{Word: "Healthy", Count: 3, Tone: ToneSuccess},
		{Word: "Down", Count: 0, Tone: ToneError},
	}}))

	if got := strings.Count(markup, "fleet-legend-line"); got != 2 {
		t.Errorf("the legend has %d lines, want 2", got)
	}

	if got := strings.Count(markup, "fleet-donut-arc"); got != 1 {
		t.Errorf("the ring has %d arcs, want 1", got)
	}

	if !strings.Contains(markup, ">Down<") {
		t.Error("the legend does not list the state with a count of zero")
	}
}

// TestStackedShares checks the widths of a stacked bar.
//
// VALIDATES: the shares add up to 100 whenever something is counted, the
// points rounding leaves go to the counts that lost most, and a sum of zero
// is all zeros.
// PREVENTS: a stacked bar that overflows its track or stops short of it.
func TestStackedShares(t *testing.T) {
	cases := []struct {
		counts []int
		want   []int
	}{
		{[]int{1, 1, 1}, []int{34, 33, 33}},
		{[]int{36, 3, 1, 3, 1, 0}, []int{82, 7, 2, 7, 2, 0}},
		{[]int{0, 0}, []int{0, 0}},
		{[]int{5}, []int{100}},
		{[]int{-3, 1}, []int{0, 100}},
		{nil, []int{}},
	}

	for _, test := range cases {
		got := StackedShares(test.counts)
		if len(got) != len(test.want) {
			t.Fatalf("%v: %d shares, want %d", test.counts, len(got), len(test.want))
		}

		for index := range got {
			if got[index] != test.want[index] {
				t.Errorf("%v: shares %v, want %v", test.counts, got, test.want)

				break
			}
		}
	}
}

// TestUsageToneThresholds checks the color of a usage bar at its two
// thresholds.
//
// VALIDATES: a bar is the accent color up to 79%, the warning color from
// 80% to 89% and the error color from 90%, in the function and in the
// markup.
// PREVENTS: an off-by-one at 80 or 90.
func TestUsageToneThresholds(t *testing.T) {
	cases := []struct {
		percent int
		tone    Tone
	}{
		{0, ToneAccent}, {79, ToneAccent}, {80, ToneWarning}, {89, ToneWarning},
		{90, ToneError}, {100, ToneError},
	}

	for _, test := range cases {
		if got := UsageTone(test.percent); got != test.tone {
			t.Errorf("%d%%: tone %q, want %q", test.percent, got.Key(), test.tone.Key())
		}

		markup := componentMarkup(t, UsageBar(UsageBarView{Name: "CPU", Percent: test.percent, Known: true}))
		if !markupHasClass(markup, test.tone.Class()) {
			t.Errorf("%d%%: the bar is not drawn with %q", test.percent, test.tone.Class())
		}

		if !markupHasClass(markup, widthClass(test.percent)) {
			t.Errorf("%d%%: the fill does not carry %q", test.percent, widthClass(test.percent))
		}
	}
}

// TestUsageBarNotReported renders a bar with no figure.
//
// VALIDATES: a figure the device has not reported is a dash and no fill.
// PREVENTS: an unknown usage drawn as 0%, which reads as an idle device.
func TestUsageBarNotReported(t *testing.T) {
	markup := componentMarkup(t, UsageBar(UsageBarView{Name: "Storage"}))

	if markupHasClass(markup, "fleet-usage-fill") {
		t.Error("a bar with no figure draws a fill")
	}

	if strings.Contains(markup, "0%<") {
		t.Error("a bar with no figure writes 0%")
	}

	if !strings.Contains(markup, "not reported") {
		t.Error("a bar with no figure does not say it is not reported")
	}
}

// TestUsageInsideFigure checks where a tall bar writes its percentage.
//
// VALIDATES: a tall bar writes the figure inside its fill from 15%, and a
// thin bar never does.
// PREVENTS: a figure clipped by a fill too narrow to hold it.
func TestUsageInsideFigure(t *testing.T) {
	cases := []struct {
		view   UsageBarView
		inside bool
	}{
		{UsageBarView{Percent: 14, Known: true, Tall: true}, false},
		{UsageBarView{Percent: 15, Known: true, Tall: true}, true},
		{UsageBarView{Percent: 60, Known: true}, false},
	}

	for _, test := range cases {
		if got := usageInside(test.view); got != test.inside {
			t.Errorf("%d%%, tall %v: inside %v, want %v", test.view.Percent, test.view.Tall, got, test.inside)
		}
	}
}
