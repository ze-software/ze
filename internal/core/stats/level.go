// Design: docs/architecture/traffic/traffic-analysis-layers.md -- the level history and the growth pattern
// Related: window.go -- the rate primitive, which clamps a decrease where a level keeps it

package stats

import (
	"errors"
	"fmt"
	"math"
	"time"
)

// levelBucketCountMax bounds the history a caller can ask for, so a mistyped
// figure cannot allocate without limit. It is far above any rule's need: a week
// of one-minute buckets is 10080.
const levelBucketCountMax = 1 << 16

var (
	// ErrLevelParams reports figures NewLevelHistory refuses, and a LevelHistory
	// that NewLevelHistory did not build.
	ErrLevelParams = errors.New("invalid level history parameters")
	// ErrLevelSample reports a level that is not a finite number.
	ErrLevelSample = errors.New("invalid level sample")
	// ErrGrowthInput reports a limit or a minimum rise the growth pattern cannot
	// judge with.
	ErrGrowthInput = errors.New("invalid growth input")
)

// LevelParams are the figures of a level history and of the growth pattern it
// judges. It is raw input: NewLevelHistory validates it, and the history keeps
// its own copy.
type LevelParams struct {
	// BucketPeriod is the length of one bucket.
	BucketPeriod time.Duration
	// BucketCount is how many closed buckets the history keeps, the newest.
	BucketCount int
	// GrowthRises is how many consecutive bucket minimums must each exceed the
	// one before for growth to hold. N rises compare N+1 buckets, so it is at
	// least 1 and at most BucketCount-1.
	GrowthRises int
	// GrowthHorizon is how soon the fitted line must reach the limit for growth
	// to hold.
	GrowthHorizon time.Duration
}

// Validate answers nil when a history can work with p, and an error wrapping
// ErrLevelParams that names the first figure it cannot.
func (p LevelParams) Validate() error {
	if p.BucketPeriod <= 0 {
		return fmt.Errorf("%w: bucket period %s is not positive", ErrLevelParams, p.BucketPeriod)
	}
	if p.BucketCount < 1 {
		return fmt.Errorf("%w: bucket count %d is below 1", ErrLevelParams, p.BucketCount)
	}
	if p.BucketCount > levelBucketCountMax {
		return fmt.Errorf("%w: bucket count %d is above %d", ErrLevelParams, p.BucketCount, levelBucketCountMax)
	}
	if p.GrowthRises < 1 {
		return fmt.Errorf("%w: growth rises %d is below 1", ErrLevelParams, p.GrowthRises)
	}
	if p.GrowthRises >= p.BucketCount {
		return fmt.Errorf("%w: %d growth rises compare %d buckets, and the history keeps %d",
			ErrLevelParams, p.GrowthRises, p.GrowthRises+1, p.BucketCount)
	}
	if p.GrowthHorizon <= 0 {
		return fmt.Errorf("%w: growth horizon %s is not positive", ErrLevelParams, p.GrowthHorizon)
	}
	return nil
}

// LevelBucket is one period of a level history: the minimum and the last
// sample of the period, or no level when the period had no sample (missing).
// The zero LevelBucket is a missing bucket starting at the zero time.
type LevelBucket struct {
	start    time.Time
	minimum  float64
	last     float64
	observed bool
}

// Start is when the bucket's period begins.
func (b LevelBucket) Start() time.Time { return b.start }

// Observed reports whether a sample fell in the bucket's period.
func (b LevelBucket) Observed() bool { return b.observed }

// Minimum is the lowest sample of the period; ok is false for a missing bucket.
func (b LevelBucket) Minimum() (level float64, ok bool) { return b.minimum, b.observed }

// Last is the sample most recently added to the period; ok is false for a
// missing bucket.
func (b LevelBucket) Last() (level float64, ok bool) { return b.last, b.observed }

func (b *LevelBucket) fold(level float64) {
	b.minimum = min(b.minimum, level)
	b.last = level
}

// LevelHistory buckets the samples of one level (how much of a resource is
// used) by period, keeps the newest BucketCount closed buckets, and judges the
// growth pattern on them. A falling level is kept as it is: unlike a byte
// counter, a level legitimately goes down.
//
// The caller passes each sample's time: the history never reads the clock. The
// history starts at its first sample, and the buckets follow from it one period
// apart. The zero LevelHistory holds no parameters and refuses every call with
// ErrLevelParams; build one with NewLevelHistory.
//
// Not safe for concurrent use; the caller serializes access.
type LevelHistory struct {
	params LevelParams
	ring   []LevelBucket // closed buckets; ring[head] is the oldest, BucketCount slots
	head   int           // index of the oldest closed bucket
	size   int           // closed buckets held, at most BucketCount
	open   LevelBucket   // the bucket the newest sample fell in, once opened
	opened bool          // the first sample has opened a bucket
}

// NewLevelHistory returns an empty history for p, or an error wrapping
// ErrLevelParams when p does not validate.
func NewLevelHistory(p LevelParams) (*LevelHistory, error) {
	if err := p.Validate(); err != nil {
		return nil, err
	}
	return &LevelHistory{params: p, ring: make([]LevelBucket, p.BucketCount)}, nil
}

// Params returns the figures the history was built with.
func (h *LevelHistory) Params() LevelParams { return h.params }

// Add folds one sample of the level, taken at at, into the history, and reports
// whether it closed a bucket: that is when the caller judges growth.
//
// A sample dated before the open bucket's start (the clock stepped back) joins
// the open bucket. A sample dated a period or more past the open bucket's start
// closes it, records each skipped period as a missing bucket, and opens the
// bucket the sample falls in, so a forward clock step ends a growth run rather
// than inventing one. At most BucketCount missing buckets are recorded however
// far the clock stepped: older ones would be dropped at once.
//
// A level that is not finite is refused with ErrLevelSample and changes nothing.
func (h *LevelHistory) Add(at time.Time, level float64) (closed bool, err error) {
	if err := h.usable(); err != nil {
		return false, err
	}
	if math.IsNaN(level) {
		return false, fmt.Errorf("%w: level is NaN", ErrLevelSample)
	}
	if math.IsInf(level, 0) {
		return false, fmt.Errorf("%w: level %v is infinite", ErrLevelSample, level)
	}
	if !h.opened {
		h.open = LevelBucket{start: at, minimum: level, last: level, observed: true}
		h.opened = true
		return false, nil
	}
	period := h.params.BucketPeriod
	elapsed := at.Sub(h.open.start)
	if elapsed < period {
		h.open.fold(level)
		return false, nil
	}
	periods := int64(elapsed / period)
	h.push(h.open)
	h.pushMissing(h.open.start, periods-1)
	start := h.open.start.Add(time.Duration(periods) * period)
	h.open = LevelBucket{start: start, minimum: level, last: level, observed: true}
	return true, nil
}

// pushMissing records the last of skipped missing periods that follow the
// bucket starting at from, no more than the ring holds.
func (h *LevelHistory) pushMissing(from time.Time, skipped int64) {
	recorded := min(skipped, int64(h.params.BucketCount))
	period := h.params.BucketPeriod
	for k := skipped - recorded + 1; k <= skipped; k++ {
		h.push(LevelBucket{start: from.Add(time.Duration(k) * period)})
	}
}

// push appends a closed bucket, dropping the oldest when the ring is full.
func (h *LevelHistory) push(b LevelBucket) {
	if h.size < len(h.ring) {
		h.ring[(h.head+h.size)%len(h.ring)] = b
		h.size++
		return
	}
	h.ring[h.head] = b
	h.head = (h.head + 1) % len(h.ring)
}

// closedAt returns the i-th closed bucket, oldest first. i MUST be below h.size.
func (h *LevelHistory) closedAt(i int) LevelBucket {
	return h.ring[(h.head+i)%len(h.ring)]
}

// Buckets returns a copy of the closed buckets, oldest first, or nil when none
// has closed.
func (h *LevelHistory) Buckets() []LevelBucket {
	if h.size == 0 {
		return nil
	}
	out := make([]LevelBucket, h.size)
	for i := range out {
		out[i] = h.closedAt(i)
	}
	return out
}

// ObservedRun is how many consecutive closed buckets, ending at the newest, were
// observed: the history growth can be judged over. The span it covers is
// ObservedRun times BucketPeriod. Growth needs GrowthRises+1 of them.
func (h *LevelHistory) ObservedRun() int {
	run := 0
	for i := h.size - 1; i >= 0; i-- {
		if !h.closedAt(i).observed {
			break
		}
		run++
	}
	return run
}

// usable refuses a history NewLevelHistory did not build.
func (h *LevelHistory) usable() error {
	if h.params.BucketCount < 1 {
		return fmt.Errorf("%w: the history was not built by NewLevelHistory", ErrLevelParams)
	}
	return nil
}

// GrowthOutcome is the answer of the growth pattern. The zero value is
// GrowthUnspecified, which no judgment answers.
type GrowthOutcome uint8

const (
	// GrowthUnspecified is the zero value: no judgment was made.
	GrowthUnspecified GrowthOutcome = iota
	// GrowthWarmingUp: fewer consecutive observed buckets end at the newest than
	// the run compares, so nothing is judged yet.
	GrowthWarmingUp
	// GrowthNotRising: a bucket minimum of the run did not exceed the one before.
	GrowthNotRising
	// GrowthRiseTooSmall: every minimum rose, by less than the minimum rise in
	// total.
	GrowthRiseTooSmall
	// GrowthLimitReached: the run rose enough, and the newest minimum or the
	// fitted line already stands at or above the limit, so no time is projected.
	GrowthLimitReached
	// GrowthBeyondHorizon: the run rose enough, and the fitted line reaches the
	// limit later than the horizon.
	GrowthBeyondHorizon
	// GrowthHolds: the run rose enough, and the fitted line reaches the limit
	// within the horizon.
	GrowthHolds
)

// String names the outcome for a log line or a test failure. Only this package
// produces a GrowthOutcome, so a value outside the named set is a Ze defect.
func (o GrowthOutcome) String() string {
	switch o {
	case GrowthUnspecified:
		return "unspecified"
	case GrowthWarmingUp:
		return "warming-up"
	case GrowthNotRising:
		return "not-rising"
	case GrowthRiseTooSmall:
		return "rise-too-small"
	case GrowthLimitReached:
		return "limit-reached"
	case GrowthBeyondHorizon:
		return "beyond-horizon"
	case GrowthHolds:
		return "holds"
	default:
		panic("BUG: stats: invalid growth outcome")
	}
}

// Growth is one judgment of the growth pattern: its outcome and the figures it
// measured. The zero Growth is GrowthUnspecified and answers no figure.
type Growth struct {
	outcome    GrowthOutcome
	rise       float64
	span       time.Duration
	exhaustion time.Duration
	measured   bool // every minimum of the run rose: rise and span are known
	projected  bool // the fitted line reaches the limit within a Duration
}

// Outcome is the answer of the judgment.
func (g Growth) Outcome() GrowthOutcome { return g.outcome }

// Rise is how much the minimum rose from the oldest to the newest bucket of the
// run; ok is false unless every minimum of the run rose.
func (g Growth) Rise() (rise float64, ok bool) { return g.rise, g.measured }

// Span is the time from the oldest to the newest bucket of the run; ok is false
// unless every minimum of the run rose.
func (g Growth) Span() (span time.Duration, ok bool) { return g.span, g.measured }

// Exhaustion is how long after the newest bucket of the run the fitted line
// reaches the limit; ok is true for GrowthHolds, and for GrowthBeyondHorizon
// when the time fits in a Duration (about 292 years).
func (g Growth) Exhaustion() (exhaustion time.Duration, ok bool) {
	return g.exhaustion, g.projected
}

// Growth judges the growth pattern over the newest GrowthRises+1 closed
// buckets: each bucket's minimum must exceed the one before, the minimum must
// rise by at least riseMin in total, and a least-squares line fitted to those
// minimums must reach limit within GrowthHorizon. The run must be observed
// buckets ending at the newest closed bucket: a missing bucket ends it.
//
// The caller passes riseMin and limit as levels, because both can move between
// judgments. riseMin is at least 0 and limit above 0, both finite; anything
// else is refused with ErrGrowthInput.
func (h *LevelHistory) Growth(riseMin, limit float64) (Growth, error) {
	if err := h.usable(); err != nil {
		return Growth{}, err
	}
	if err := checkGrowthInput(riseMin, limit); err != nil {
		return Growth{}, err
	}
	compared := h.params.GrowthRises + 1
	if h.ObservedRun() < compared {
		return Growth{outcome: GrowthWarmingUp}, nil
	}
	minimums := make([]float64, compared)
	for i := range minimums {
		minimums[i] = h.closedAt(h.size - compared + i).minimum
	}
	for i := 1; i < compared; i++ {
		if minimums[i] <= minimums[i-1] {
			return Growth{outcome: GrowthNotRising}, nil
		}
	}
	newest := minimums[compared-1]
	g := Growth{
		rise:     newest - minimums[0],
		span:     time.Duration(h.params.GrowthRises) * h.params.BucketPeriod,
		measured: true,
	}
	if g.rise < riseMin {
		g.outcome = GrowthRiseTooSmall
		return g, nil
	}
	slope, fitted := fitLevelLine(minimums)
	// A strictly rising run fits a positive slope; this guards the division
	// below against a rounding that answers otherwise.
	if slope <= 0 {
		return Growth{outcome: GrowthNotRising}, nil
	}
	if limit <= newest {
		g.outcome = GrowthLimitReached
		return g, nil
	}
	if limit <= fitted {
		g.outcome = GrowthLimitReached
		return g, nil
	}
	nanoseconds := (limit - fitted) / slope * float64(h.params.BucketPeriod)
	if nanoseconds > float64(h.params.GrowthHorizon) {
		g.outcome = GrowthBeyondHorizon
		if nanoseconds < math.MaxInt64 {
			g.exhaustion = time.Duration(nanoseconds)
			g.projected = true
		}
		return g, nil
	}
	g.outcome = GrowthHolds
	g.exhaustion = time.Duration(nanoseconds)
	g.projected = true
	return g, nil
}

func checkGrowthInput(riseMin, limit float64) error {
	if math.IsNaN(limit) {
		return fmt.Errorf("%w: limit is NaN", ErrGrowthInput)
	}
	if math.IsInf(limit, 0) {
		return fmt.Errorf("%w: limit %v is infinite", ErrGrowthInput, limit)
	}
	if limit <= 0 {
		return fmt.Errorf("%w: limit %v is not positive, so nothing can be judged against it", ErrGrowthInput, limit)
	}
	if math.IsNaN(riseMin) {
		return fmt.Errorf("%w: minimum rise is NaN", ErrGrowthInput)
	}
	if math.IsInf(riseMin, 0) {
		return fmt.Errorf("%w: minimum rise %v is infinite", ErrGrowthInput, riseMin)
	}
	if riseMin < 0 {
		return fmt.Errorf("%w: minimum rise %v is negative", ErrGrowthInput, riseMin)
	}
	return nil
}

// fitLevelLine fits a least-squares line to levels taken one bucket apart, and
// returns its slope (level per bucket) and its value at the newest bucket.
// levels MUST hold at least two values, so the spread of positions is not zero.
func fitLevelLine(levels []float64) (slope, fitted float64) {
	newestAt := float64(len(levels) - 1)
	positionMean := newestAt / 2
	levelMean := Mean(levels)
	var covariance, spread float64
	for i, level := range levels {
		offset := float64(i) - positionMean
		covariance += offset * (level - levelMean)
		spread += offset * offset
	}
	slope = covariance / spread
	return slope, levelMean + slope*(newestAt-positionMean)
}
