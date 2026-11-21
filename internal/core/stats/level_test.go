// VALIDATES: AC-1 to AC-3 of spec-host-resource-trend-alerts -- the level
// history buckets a sampled level (the minimum and the last sample of each
// period, the newest N buckets kept, a period with no sample recorded as
// missing), and the growth pattern is judged on it (a run of rising bucket
// minimums, a rise of at least the stated amount, a fitted line reaching the
// limit within the horizon).
// PREVENTS: a burst read as a leak, growth that settles read as growth, a run
// bridged across a missing period, a projection from too few buckets, "not
// enough history" read as "no growth", a clock step that closes buckets early
// or without bound, and an unbounded history.

package stats

import (
	"errors"
	"math"
	"testing"
	"time"
)

var levelEpoch = time.Date(2026, 11, 20, 0, 0, 0, 0, time.UTC)

// kernelMemoryParams are the figures of the kernel-memory rule: one-hour
// buckets, a week kept, six rises, a seven-day horizon.
func kernelMemoryParams() LevelParams {
	return LevelParams{
		BucketPeriod:  time.Hour,
		BucketCount:   168,
		GrowthRises:   6,
		GrowthHorizon: 7 * 24 * time.Hour,
	}
}

func newLevelHistory(t *testing.T, p LevelParams) *LevelHistory {
	t.Helper()
	h, err := NewLevelHistory(p)
	if err != nil {
		t.Fatalf("NewLevelHistory(%+v): %v", p, err)
	}
	return h
}

func addLevel(t *testing.T, h *LevelHistory, at time.Time, level float64) bool {
	t.Helper()
	closed, err := h.Add(at, level)
	if err != nil {
		t.Fatalf("Add(%s, %v): %v", at, level, err)
	}
	return closed
}

func judgeGrowth(t *testing.T, h *LevelHistory, riseMin, limit float64) Growth {
	t.Helper()
	g, err := h.Growth(riseMin, limit)
	if err != nil {
		t.Fatalf("Growth(%v, %v): %v", riseMin, limit, err)
	}
	return g
}

// gap marks an entry of feedBuckets that receives no sample.
var gap = math.NaN()

// burstAbove is how far above its bucket's minimum feedBuckets puts a burst. The
// closing sample sits far above every floor, so it never becomes a minimum.
const (
	burstAbove   = 1000
	closingLevel = 1e12
)

// feedBuckets feeds one bucket per entry of minimums, the first at bucket index
// first counted from levelEpoch. An observed bucket gets its minimum at its start
// and a burst above it at mid-period; a gap entry gets no sample. A sample at the
// start of the next bucket closes the last one. The first entry of a fresh
// history MUST NOT be a gap: the history starts at its first sample.
func feedBuckets(t *testing.T, h *LevelHistory, first int, minimums []float64) {
	t.Helper()
	period := h.Params().BucketPeriod
	for i, minimum := range minimums {
		if math.IsNaN(minimum) {
			continue
		}
		start := levelEpoch.Add(time.Duration(first+i) * period)
		addLevel(t, h, start, minimum)
		addLevel(t, h, start.Add(period/2), minimum+burstAbove)
	}
	addLevel(t, h, levelEpoch.Add(time.Duration(first+len(minimums))*period), closingLevel)
}

func bucketMinimum(t *testing.T, b LevelBucket) float64 {
	t.Helper()
	minimum, ok := b.Minimum()
	if !ok {
		t.Fatalf("bucket at %s is missing, want an observed minimum", b.Start())
	}
	return minimum
}

func bucketLast(t *testing.T, b LevelBucket) float64 {
	t.Helper()
	last, ok := b.Last()
	if !ok {
		t.Fatalf("bucket at %s is missing, want an observed last sample", b.Start())
	}
	return last
}

func TestLevelHistoryBucketsMinimumAndLast(t *testing.T) {
	// Goal: a closed bucket holds the lowest and the most recent sample of its
	// period, and the sample that closes it is the first one a period on.
	// Method: four samples inside the first hour, then one at the hour.
	h := newLevelHistory(t, kernelMemoryParams())
	inside := []struct {
		after time.Duration
		level float64
	}{{0, 50}, {10 * time.Minute, 40}, {20 * time.Minute, 70}, {50 * time.Minute, 60}}
	for _, s := range inside {
		if addLevel(t, h, levelEpoch.Add(s.after), s.level) {
			t.Fatalf("sample at +%s closed a bucket, want none inside the period", s.after)
		}
	}
	if !addLevel(t, h, levelEpoch.Add(time.Hour), 55) {
		t.Fatal("sample at +1h closed no bucket, want the first bucket closed")
	}
	buckets := h.Buckets()
	if len(buckets) != 1 {
		t.Fatalf("closed buckets = %d, want 1", len(buckets))
	}
	if !buckets[0].Start().Equal(levelEpoch) {
		t.Errorf("bucket start = %s, want %s", buckets[0].Start(), levelEpoch)
	}
	if got := bucketMinimum(t, buckets[0]); got != 40 {
		t.Errorf("minimum = %v, want 40", got)
	}
	if got := bucketLast(t, buckets[0]); got != 60 {
		t.Errorf("last = %v, want 60", got)
	}
}

func TestLevelHistoryKeepsFallingLevel(t *testing.T) {
	// Goal: a falling level is kept as it is, unlike a byte counter's window,
	// which clamps a decrease.
	// Method: a level of 100, then a second hour falling from 90 to 30.
	h := newLevelHistory(t, kernelMemoryParams())
	addLevel(t, h, levelEpoch, 100)
	addLevel(t, h, levelEpoch.Add(time.Hour), 90)
	addLevel(t, h, levelEpoch.Add(90*time.Minute), 30)
	addLevel(t, h, levelEpoch.Add(2*time.Hour), 20)
	buckets := h.Buckets()
	if len(buckets) != 2 {
		t.Fatalf("closed buckets = %d, want 2", len(buckets))
	}
	if got := bucketMinimum(t, buckets[1]); got != 30 {
		t.Errorf("minimum of the falling hour = %v, want 30", got)
	}
	if got := bucketLast(t, buckets[1]); got != 30 {
		t.Errorf("last of the falling hour = %v, want 30", got)
	}
}

func TestLevelHistoryKeepsNewestBuckets(t *testing.T) {
	// Goal: the history holds at most BucketCount closed buckets, the newest,
	// oldest first.
	// Method: a three-bucket history fed five buckets.
	h := newLevelHistory(t, LevelParams{
		BucketPeriod: time.Hour, BucketCount: 3, GrowthRises: 1, GrowthHorizon: time.Hour,
	})
	feedBuckets(t, h, 0, []float64{1, 2, 3, 4, 5})
	buckets := h.Buckets()
	if len(buckets) != 3 {
		t.Fatalf("closed buckets = %d, want 3", len(buckets))
	}
	for i, want := range []float64{3, 4, 5} {
		if got := bucketMinimum(t, buckets[i]); got != want {
			t.Errorf("bucket %d minimum = %v, want %v", i, got, want)
		}
		wantStart := levelEpoch.Add(time.Duration(i+2) * time.Hour)
		if !buckets[i].Start().Equal(wantStart) {
			t.Errorf("bucket %d start = %s, want %s", i, buckets[i].Start(), wantStart)
		}
	}
}

func TestLevelHistoryRecordsMissingBucket(t *testing.T) {
	// Goal: a period with no sample is recorded as a missing bucket that
	// answers no level, and the buckets after it keep their alignment.
	// Method: a sample in hour 0, none in hour 1, one in hour 2, then the
	// samples around the end of hour 2.
	h := newLevelHistory(t, kernelMemoryParams())
	addLevel(t, h, levelEpoch, 10)
	if !addLevel(t, h, levelEpoch.Add(150*time.Minute), 20) {
		t.Fatal("sample in hour 2 closed no bucket")
	}
	buckets := h.Buckets()
	if len(buckets) != 2 {
		t.Fatalf("closed buckets = %d, want 2 (hour 0 observed, hour 1 missing)", len(buckets))
	}
	if !buckets[0].Observed() {
		t.Error("hour 0 reads missing, want observed")
	}
	missing := buckets[1]
	if missing.Observed() {
		t.Error("hour 1 reads observed, want missing")
	}
	if !missing.Start().Equal(levelEpoch.Add(time.Hour)) {
		t.Errorf("missing bucket start = %s, want %s", missing.Start(), levelEpoch.Add(time.Hour))
	}
	if level, ok := missing.Minimum(); ok {
		t.Errorf("missing bucket answers minimum %v, want none", level)
	}
	if level, ok := missing.Last(); ok {
		t.Errorf("missing bucket answers last %v, want none", level)
	}
	if addLevel(t, h, levelEpoch.Add(179*time.Minute), 25) {
		t.Error("sample at +2h59m closed a bucket, want hour 2 still open")
	}
	if !addLevel(t, h, levelEpoch.Add(3*time.Hour), 30) {
		t.Fatal("sample at +3h closed no bucket, want hour 2 closed")
	}
	buckets = h.Buckets()
	if !buckets[2].Start().Equal(levelEpoch.Add(2 * time.Hour)) {
		t.Errorf("hour 2 start = %s, want %s", buckets[2].Start(), levelEpoch.Add(2*time.Hour))
	}
	if got := bucketMinimum(t, buckets[2]); got != 20 {
		t.Errorf("hour 2 minimum = %v, want 20", got)
	}
}

func TestLevelHistoryFoldsBackwardSampleIntoOpenBucket(t *testing.T) {
	// Goal: a sample dated before the open bucket's start (the clock stepped
	// back) joins the open bucket and closes nothing.
	// Method: two samples in hour 0, one dated three hours earlier, then one
	// at the hour.
	h := newLevelHistory(t, kernelMemoryParams())
	addLevel(t, h, levelEpoch, 50)
	addLevel(t, h, levelEpoch.Add(30*time.Minute), 40)
	if addLevel(t, h, levelEpoch.Add(-3*time.Hour), 10) {
		t.Fatal("a sample dated before the open bucket closed a bucket")
	}
	if !addLevel(t, h, levelEpoch.Add(time.Hour), 99) {
		t.Fatal("sample at +1h closed no bucket")
	}
	buckets := h.Buckets()
	if len(buckets) != 1 {
		t.Fatalf("closed buckets = %d, want 1", len(buckets))
	}
	if !buckets[0].Start().Equal(levelEpoch) {
		t.Errorf("bucket start = %s, want %s", buckets[0].Start(), levelEpoch)
	}
	if got := bucketMinimum(t, buckets[0]); got != 10 {
		t.Errorf("minimum = %v, want 10 (the backward sample)", got)
	}
	if got := bucketLast(t, buckets[0]); got != 10 {
		t.Errorf("last = %v, want 10 (the most recently added sample)", got)
	}
}

func TestLevelHistoryForwardStepRecordsBoundedMissing(t *testing.T) {
	// Goal: a forward clock step closes the skipped periods as missing, records
	// no more of them than the history keeps, and keeps the bucket alignment.
	// Method: a four-bucket history, a sample, then one 1000 hours later.
	h := newLevelHistory(t, LevelParams{
		BucketPeriod: time.Hour, BucketCount: 4, GrowthRises: 1, GrowthHorizon: time.Hour,
	})
	addLevel(t, h, levelEpoch, 5)
	if !addLevel(t, h, levelEpoch.Add(1000*time.Hour), 6) {
		t.Fatal("a sample 1000 hours on closed no bucket")
	}
	buckets := h.Buckets()
	if len(buckets) != 4 {
		t.Fatalf("closed buckets = %d, want 4", len(buckets))
	}
	for i, b := range buckets {
		if b.Observed() {
			t.Errorf("bucket %d reads observed, want missing", i)
		}
		wantStart := levelEpoch.Add(time.Duration(996+i) * time.Hour)
		if !b.Start().Equal(wantStart) {
			t.Errorf("bucket %d start = %s, want %s", i, b.Start(), wantStart)
		}
	}
	if run := h.ObservedRun(); run != 0 {
		t.Errorf("observed run = %d, want 0 after the step", run)
	}
	if addLevel(t, h, levelEpoch.Add(1000*time.Hour+30*time.Minute), 7) {
		t.Error("sample at +1000h30m closed a bucket, want the stepped-to hour open")
	}
	if !addLevel(t, h, levelEpoch.Add(1001*time.Hour), 8) {
		t.Fatal("sample at +1001h closed no bucket")
	}
	newest := h.Buckets()[3]
	if !newest.Start().Equal(levelEpoch.Add(1000 * time.Hour)) {
		t.Errorf("newest start = %s, want %s", newest.Start(), levelEpoch.Add(1000*time.Hour))
	}
	if got := bucketMinimum(t, newest); got != 6 {
		t.Errorf("newest minimum = %v, want 6", got)
	}
	if got := bucketLast(t, newest); got != 7 {
		t.Errorf("newest last = %v, want 7", got)
	}
	if run := h.ObservedRun(); run != 1 {
		t.Errorf("observed run = %d, want 1", run)
	}
}

func TestLevelHistoryRefusesNonFiniteLevel(t *testing.T) {
	// Goal: a level that is not a finite number is refused and leaves the
	// history as it was.
	// Method: NaN and both infinities first, then a valid sample, whose time
	// must be where the history starts.
	h := newLevelHistory(t, kernelMemoryParams())
	for _, level := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
		if _, err := h.Add(levelEpoch, level); !errors.Is(err, ErrLevelSample) {
			t.Errorf("Add(%v) error = %v, want ErrLevelSample", level, err)
		}
	}
	addLevel(t, h, levelEpoch.Add(30*time.Minute), 1)
	if addLevel(t, h, levelEpoch.Add(80*time.Minute), 2) {
		t.Fatal("a refused sample opened the history: +1h20m closed a bucket")
	}
	addLevel(t, h, levelEpoch.Add(90*time.Minute), 3)
	if start := h.Buckets()[0].Start(); !start.Equal(levelEpoch.Add(30 * time.Minute)) {
		t.Errorf("history starts at %s, want %s", start, levelEpoch.Add(30*time.Minute))
	}
}

func TestLevelHistoryZeroValueRefuses(t *testing.T) {
	// Goal: a LevelHistory that NewLevelHistory did not build refuses to work
	// rather than divide by a zero period.
	// Method: Add and Growth on the zero value.
	var h LevelHistory
	if _, err := h.Add(levelEpoch, 1); !errors.Is(err, ErrLevelParams) {
		t.Errorf("Add error = %v, want ErrLevelParams", err)
	}
	if _, err := h.Growth(0, 1); !errors.Is(err, ErrLevelParams) {
		t.Errorf("Growth error = %v, want ErrLevelParams", err)
	}
}

func TestLevelParamsRefuseInvalid(t *testing.T) {
	// Goal: every figure a history cannot work with is refused at construction.
	// Method: one invalid figure per case on otherwise valid parameters.
	base := kernelMemoryParams()
	cases := []struct {
		name   string
		mutate func(*LevelParams)
	}{
		{"zero period", func(p *LevelParams) { p.BucketPeriod = 0 }},
		{"negative period", func(p *LevelParams) { p.BucketPeriod = -time.Hour }},
		{"zero bucket count", func(p *LevelParams) { p.BucketCount = 0 }},
		{"negative bucket count", func(p *LevelParams) { p.BucketCount = -1 }},
		{"bucket count above the bound", func(p *LevelParams) { p.BucketCount = levelBucketCountMax + 1 }},
		{"zero horizon", func(p *LevelParams) { p.GrowthHorizon = 0 }},
		{"negative horizon", func(p *LevelParams) { p.GrowthHorizon = -time.Hour }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := base
			c.mutate(&p)
			if _, err := NewLevelHistory(p); !errors.Is(err, ErrLevelParams) {
				t.Errorf("NewLevelHistory(%+v) error = %v, want ErrLevelParams", p, err)
			}
		})
	}
}

func TestLevelParamsBucketCountBound(t *testing.T) {
	// Goal: the largest bucket count the bound admits is accepted.
	// Method: the bound itself, with the smallest run.
	p := LevelParams{
		BucketPeriod: time.Minute, BucketCount: levelBucketCountMax, GrowthRises: 1, GrowthHorizon: time.Hour,
	}
	if _, err := NewLevelHistory(p); err != nil {
		t.Errorf("NewLevelHistory at the bucket count bound: %v", err)
	}
}

func TestLevelParamsGrowthRisesBounds(t *testing.T) {
	// Goal: the growth run is 1 rise at least and at most one fewer than the
	// bucket count, because N rises compare N+1 buckets the history must hold.
	// Method: a ten-bucket history with runs of 0, 1, 9, 10 and 11 rises.
	cases := []struct {
		rises int
		valid bool
	}{{0, false}, {1, true}, {9, true}, {10, false}, {11, false}}
	for _, c := range cases {
		p := LevelParams{BucketPeriod: time.Hour, BucketCount: 10, GrowthRises: c.rises, GrowthHorizon: time.Hour}
		_, err := NewLevelHistory(p)
		if c.valid && err != nil {
			t.Errorf("rises %d refused: %v", c.rises, err)
		}
		if !c.valid && !errors.Is(err, ErrLevelParams) {
			t.Errorf("rises %d error = %v, want ErrLevelParams", c.rises, err)
		}
	}
}

func TestGrowthHoldsOnSustainedRise(t *testing.T) {
	// Goal: six hourly minimums each above the one before, a rise above the
	// minimum and a fitted line reaching the limit within the horizon hold as
	// growth, with the rise, the span and the projected time to exhaustion.
	// Method: floors 100 to 160 by 10 an hour under bursts of +1000, a limit of
	// 400: the line reaches it (400-160)/10 = 24 hours after the newest bucket.
	h := newLevelHistory(t, kernelMemoryParams())
	feedBuckets(t, h, 0, []float64{100, 110, 120, 130, 140, 150, 160})
	g := judgeGrowth(t, h, 50, 400)
	if g.Outcome() != GrowthHolds {
		t.Fatalf("outcome = %s, want %s", g.Outcome(), GrowthHolds)
	}
	if rise, ok := g.Rise(); !ok || rise != 60 {
		t.Errorf("rise = %v (measured %v), want 60", rise, ok)
	}
	if span, ok := g.Span(); !ok || span != 6*time.Hour {
		t.Errorf("span = %s (measured %v), want 6h", span, ok)
	}
	if exhaustion, ok := g.Exhaustion(); !ok || exhaustion != 24*time.Hour {
		t.Errorf("exhaustion = %s (projected %v), want 24h", exhaustion, ok)
	}
}

func TestGrowthFalseWhenRiseSettles(t *testing.T) {
	// Goal: a level that rises and then holds is not growth: settling is
	// healthy, and no projection is offered.
	// Method: floors rising for three hours, then flat for four.
	h := newLevelHistory(t, kernelMemoryParams())
	feedBuckets(t, h, 0, []float64{100, 110, 120, 130, 130, 130, 130, 130})
	g := judgeGrowth(t, h, 0, 400)
	if g.Outcome() != GrowthNotRising {
		t.Fatalf("outcome = %s, want %s", g.Outcome(), GrowthNotRising)
	}
	if exhaustion, ok := g.Exhaustion(); ok {
		t.Errorf("settled level projects exhaustion in %s, want no projection", exhaustion)
	}
}

func TestGrowthRunEndsAtMissingBucket(t *testing.T) {
	// Goal: a missing bucket ends a growth run, so the pattern answers "not
	// enough observed history" until a new run of observed buckets is long
	// enough, and growth after the gap then counts.
	// Method: floors rising every hour with hour 3 missing, then four more
	// rising hours, judged after each part.
	h := newLevelHistory(t, kernelMemoryParams())
	feedBuckets(t, h, 0, []float64{100, 110, 120, gap, 140, 150, 160})
	if run := h.ObservedRun(); run != 3 {
		t.Errorf("observed run after the gap = %d, want 3", run)
	}
	if g := judgeGrowth(t, h, 0, 1000); g.Outcome() != GrowthWarmingUp {
		t.Fatalf("outcome across the gap = %s, want %s", g.Outcome(), GrowthWarmingUp)
	}
	feedBuckets(t, h, 7, []float64{170, 180, 190, 200})
	if g := judgeGrowth(t, h, 0, 1000); g.Outcome() != GrowthHolds {
		t.Fatalf("outcome after seven observed hours = %s, want %s", g.Outcome(), GrowthHolds)
	}
}

func TestGrowthRunRestartsAfterFlatBucket(t *testing.T) {
	// Goal: one bucket whose minimum did not rise ends the run, and the run
	// restarts from it: six rises after it hold as growth.
	// Method: hour 3 flat with hour 2, judged, then three more rising hours.
	h := newLevelHistory(t, kernelMemoryParams())
	feedBuckets(t, h, 0, []float64{100, 110, 120, 120, 130, 140, 150})
	if g := judgeGrowth(t, h, 0, 1000); g.Outcome() != GrowthNotRising {
		t.Fatalf("outcome with a flat hour = %s, want %s", g.Outcome(), GrowthNotRising)
	}
	feedBuckets(t, h, 7, []float64{160, 170, 180})
	if g := judgeGrowth(t, h, 0, 1000); g.Outcome() != GrowthHolds {
		t.Fatalf("outcome after six rises from the flat hour = %s, want %s", g.Outcome(), GrowthHolds)
	}
}

func TestGrowthWarmingUpBeforeEnoughBuckets(t *testing.T) {
	// Goal: with fewer observed buckets than the run compares, the pattern says
	// so, distinct from "no growth", and computes no projection.
	// Method: an empty history, then six rising hours where seven are needed.
	h := newLevelHistory(t, kernelMemoryParams())
	if g := judgeGrowth(t, h, 0, 1000); g.Outcome() != GrowthWarmingUp {
		t.Fatalf("outcome on an empty history = %s, want %s", g.Outcome(), GrowthWarmingUp)
	}
	feedBuckets(t, h, 0, []float64{100, 110, 120, 130, 140, 150})
	g := judgeGrowth(t, h, 0, 1000)
	if g.Outcome() != GrowthWarmingUp {
		t.Fatalf("outcome on six hours = %s, want %s", g.Outcome(), GrowthWarmingUp)
	}
	if exhaustion, ok := g.Exhaustion(); ok {
		t.Errorf("warming up projects exhaustion in %s, want no projection", exhaustion)
	}
	if run := h.ObservedRun(); run != 6 {
		t.Errorf("observed run = %d, want 6", run)
	}
}

func TestGrowthRiseBelowMinimum(t *testing.T) {
	// Goal: a run that rises by less than the stated minimum is not growth, and
	// still reports the rise it measured.
	// Method: floors rising by 1 an hour against a minimum rise of 50.
	h := newLevelHistory(t, kernelMemoryParams())
	feedBuckets(t, h, 0, []float64{100, 101, 102, 103, 104, 105, 106})
	g := judgeGrowth(t, h, 50, 1000)
	if g.Outcome() != GrowthRiseTooSmall {
		t.Fatalf("outcome = %s, want %s", g.Outcome(), GrowthRiseTooSmall)
	}
	if rise, ok := g.Rise(); !ok || rise != 6 {
		t.Errorf("rise = %v (measured %v), want 6", rise, ok)
	}
	if exhaustion, ok := g.Exhaustion(); ok {
		t.Errorf("a rise below the minimum projects exhaustion in %s, want none", exhaustion)
	}
}

func TestGrowthBeyondHorizon(t *testing.T) {
	// Goal: a rising run whose line reaches the limit after the horizon is not
	// growth, and reports the projection it computed.
	// Method: floors rising by 1 an hour against a limit of 1e6: about 999894
	// hours away, past the seven-day horizon.
	h := newLevelHistory(t, kernelMemoryParams())
	feedBuckets(t, h, 0, []float64{100, 101, 102, 103, 104, 105, 106})
	g := judgeGrowth(t, h, 1, 1e6)
	if g.Outcome() != GrowthBeyondHorizon {
		t.Fatalf("outcome = %s, want %s", g.Outcome(), GrowthBeyondHorizon)
	}
	exhaustion, ok := g.Exhaustion()
	if !ok {
		t.Fatal("beyond the horizon answers no projection, want the computed one")
	}
	want := 999894 * time.Hour
	if diff := (exhaustion - want).Abs(); diff > time.Second {
		t.Errorf("exhaustion = %s, want %s", exhaustion, want)
	}
}

func TestGrowthBeyondDurationRange(t *testing.T) {
	// Goal: a projection longer than a Duration can hold is beyond the horizon
	// with no projection, never a wrapped or saturated figure.
	// Method: floors rising by 1 an hour against a limit of 1e18.
	h := newLevelHistory(t, kernelMemoryParams())
	feedBuckets(t, h, 0, []float64{100, 101, 102, 103, 104, 105, 106})
	g := judgeGrowth(t, h, 1, 1e18)
	if g.Outcome() != GrowthBeyondHorizon {
		t.Fatalf("outcome = %s, want %s", g.Outcome(), GrowthBeyondHorizon)
	}
	if exhaustion, ok := g.Exhaustion(); ok {
		t.Errorf("exhaustion = %s, want no projection past the Duration range", exhaustion)
	}
}

func TestGrowthLimitReached(t *testing.T) {
	// Goal: a limit at or below the newest floor projects nothing and says the
	// limit is reached, never a zero time to exhaustion.
	// Method: floors rising to 160, judged against limits of 160 and 150.
	h := newLevelHistory(t, kernelMemoryParams())
	feedBuckets(t, h, 0, []float64{100, 110, 120, 130, 140, 150, 160})
	for _, limit := range []float64{160, 150} {
		g := judgeGrowth(t, h, 0, limit)
		if g.Outcome() != GrowthLimitReached {
			t.Errorf("limit %v: outcome = %s, want %s", limit, g.Outcome(), GrowthLimitReached)
		}
		if exhaustion, ok := g.Exhaustion(); ok {
			t.Errorf("limit %v: exhaustion = %s, want no projection", limit, exhaustion)
		}
	}
}

func TestGrowthRefusesInvalidInputs(t *testing.T) {
	// Goal: a limit or a minimum rise the pattern cannot judge with is an
	// error, never a judgment; a limit of zero is "cannot judge".
	// Method: each invalid figure against a history that holds growth.
	h := newLevelHistory(t, kernelMemoryParams())
	feedBuckets(t, h, 0, []float64{100, 110, 120, 130, 140, 150, 160})
	cases := []struct {
		name           string
		riseMin, limit float64
	}{
		{"zero limit", 0, 0},
		{"negative limit", 0, -1},
		{"NaN limit", 0, math.NaN()},
		{"infinite limit", 0, math.Inf(1)},
		{"negative minimum rise", -1, 400},
		{"NaN minimum rise", math.NaN(), 400},
		{"infinite minimum rise", math.Inf(1), 400},
	}
	for _, c := range cases {
		if _, err := h.Growth(c.riseMin, c.limit); !errors.Is(err, ErrGrowthInput) {
			t.Errorf("%s: error = %v, want ErrGrowthInput", c.name, err)
		}
	}
}

func TestGrowthZeroValueAnswersNothing(t *testing.T) {
	// Goal: the zero Growth is Unspecified and answers no figure, so a result
	// nobody computed cannot pass for a judgment.
	// Method: read every accessor of the zero value.
	var g Growth
	if g.Outcome() != GrowthUnspecified {
		t.Errorf("outcome = %s, want %s", g.Outcome(), GrowthUnspecified)
	}
	if _, ok := g.Rise(); ok {
		t.Error("zero Growth answers a rise")
	}
	if _, ok := g.Span(); ok {
		t.Error("zero Growth answers a span")
	}
	if _, ok := g.Exhaustion(); ok {
		t.Error("zero Growth answers an exhaustion")
	}
}
