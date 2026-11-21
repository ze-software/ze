package trend

import (
	"errors"
	"math"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/host"
)

// VALIDATES: a rule the monitor could not run is refused at registration,
// one guard per field, and a valid one passes.
// Method: validateRule over the test rule with one field broken at a time.
func TestValidateRuleRefusesEachMissingField(t *testing.T) {
	valid := testRule("test-valid", "doctor-test-valid", func(*host.Detector) reading { return absentReading{} })
	if err := validateRule(valid); err != nil {
		t.Fatalf("valid rule refused: %v", err)
	}
	cases := map[string]func(*rule){
		"no name":                    func(r *rule) { r.name = "" },
		"no code":                    func(r *rule) { r.code = "" },
		"no unit":                    func(r *rule) { r.unit = UnitUnspecified },
		"sample under a second":      func(r *rule) { r.samplePeriod = time.Second - 1 },
		"sample longer than bucket":  func(r *rule) { r.samplePeriod = r.history.BucketPeriod + time.Second },
		"invalid history":            func(r *rule) { r.history.GrowthRises = r.history.BucketCount },
		"confirm count 0":            func(r *rule) { r.confirmAfter = 0 },
		"clear count 0":              func(r *rule) { r.clearAfter = 0 },
		"no reader":                  func(r *rule) { r.read = nil },
		"no message writer":          func(r *rule) { r.describe = nil },
		"no fix":                     func(r *rule) { r.fix = "" },
		"confirm count 1 (boundary)": nil,
	}
	for name, breakRule := range cases {
		r := valid
		if breakRule == nil {
			r.confirmAfter = 1
			if err := validateRule(r); err != nil {
				t.Errorf("%s: refused: %v", name, err)
			}
			continue
		}
		breakRule(&r)
		if err := validateRule(r); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

// VALIDATES: a level reading refuses every figure the growth pattern cannot
// judge, so no reader can hand the monitor a NaN, a negative level or a zero
// limit.
// Method: newLevelReading over each invalid figure and one valid sample.
func TestLevelReadingRefusesUnjudgeableFigures(t *testing.T) {
	if _, err := newLevelReading(10, 100, 1); err != nil {
		t.Fatalf("valid sample refused: %v", err)
	}
	invalid := map[string][3]float64{
		"NaN level":      {math.NaN(), 100, 1},
		"negative level": {-1, 100, 1},
		"zero limit":     {10, 0, 1},
		"infinite limit": {10, math.Inf(1), 1},
		"negative rise":  {10, 100, -1},
	}
	for name, figures := range invalid {
		if _, err := newLevelReading(figures[0], figures[1], figures[2]); !errors.Is(err, errReadingInvalid) {
			t.Errorf("%s: error = %v, want errReadingInvalid", name, err)
		}
	}
}

// VALIDATES: the warning's figures are worded in human units, as the spec's
// example message ("412 MiB", "about 2 days").
// Method: humanBytes and humanDuration over the unit boundaries.
func TestHumanUnitsWordFigures(t *testing.T) {
	bytes := map[float64]string{
		0:                        "0 B",
		1023:                     "1023 B",
		1024:                     "1.0 KiB",
		412 * 1024 * 1024:        "412 MiB",
		1.5 * 1024 * 1024 * 1024: "1.5 GiB",
	}
	for value, want := range bytes {
		if got := humanBytes(value); got != want {
			t.Errorf("humanBytes(%v) = %q, want %q", value, got, want)
		}
	}
	durations := map[time.Duration]string{
		30 * time.Second: "1 minute",
		5 * time.Minute:  "5 minutes",
		time.Hour:        "1 hour",
		6 * time.Hour:    "6 hours",
		30 * time.Hour:   "1 day",
		50 * time.Hour:   "2 days",
	}
	for value, want := range durations {
		if got := humanDuration(value); got != want {
			t.Errorf("humanDuration(%s) = %q, want %q", value, got, want)
		}
	}
}
