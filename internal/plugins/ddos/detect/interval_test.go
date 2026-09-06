package detect

import (
	"testing"

	"github.com/ze-software/ze/internal/component/trafficstat"
	"github.com/ze-software/ze/internal/core/ddosevent"
)

// intervalTestConfig parses the operator's own config text, so these tests enter
// where an operator enters: a `ddos detect` block with check-interval set. The
// framework delivers every leaf as a JSON string, which ParseConfig is what
// handles, so the value under test crosses the same boundary it crosses in
// production.
func intervalTestConfig(t *testing.T, data string) *Config {
	t.Helper()
	cfg, err := ParseConfig(data)
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	return cfg
}

// VALIDATES: check-interval decides how often the detector evaluates. With
// check-interval 3 the rate feed's first two ticks accumulate and only the third
// runs an evaluation, so an above-threshold rate cannot trigger before the
// interval closes.
// PREVENTS: the leaf being parsed, range-checked and read by nothing. The
// detector evaluated once per feed tick, and both feeds tick once a second, so
// the cadence was one second whatever the operator configured.
func TestCheckIntervalDecimatesEvaluations(t *testing.T) {
	cfg := intervalTestConfig(t, `{"ddos":{"detect":{"enabled":"true","check-interval":"3",`+
		`"confirm-duration":"1","absolute-floor":"100","baseline-window":"10","startup-grace":"0"}}}`)
	if cfg.CheckInterval != 3 {
		t.Fatalf("check-interval = %d, want 3", cfg.CheckInterval)
	}

	bus := newDTestBus()
	d := newDetector(cfg, bus, nil)
	var detected *ddosevent.AttackDetected
	ddosevent.Detected.Subscribe(bus, func(e *ddosevent.AttackDetected) { detected = e })

	flood := []trafficstat.InterfaceEntry{{Name: "xe0", RxPps: 100000, RxBps: 100000}}

	// The threshold is the absolute floor (100 pps) while the baseline is cold, so
	// each of these ticks is above it. confirm-duration 1 means the first
	// EVALUATION activates -- and the first two ticks must not be evaluations.
	for range 2 {
		d.onRates(flood)
	}
	d.wg.Wait()
	if detected != nil {
		t.Fatalf("the detector evaluated inside the check-interval: AttackDetected after %d of 3 ticks", 2)
	}

	d.onRates(flood)
	d.wg.Wait()
	if detected == nil {
		t.Fatal("the detector did not evaluate when the check-interval closed on the third tick")
	}
}

// VALIDATES: an evaluation reads the PEAK rate of its interval, on the interface
// that carried it, so a flood shorter than check-interval still triggers.
// PREVENTS: decimation that samples the feed instead of folding it. Reading only
// the tick the interval closes on would make every burst shorter than the
// interval invisible, which is the attack shape the detector exists to catch.
func TestCheckIntervalEvaluatesTheIntervalPeak(t *testing.T) {
	cfg := intervalTestConfig(t, `{"ddos":{"detect":{"enabled":"true","check-interval":"5",`+
		`"confirm-duration":"1","absolute-floor":"100","baseline-window":"10","startup-grace":"0"}}}`)

	bus := newDTestBus()
	d := newDetector(cfg, bus, nil)
	var detected *ddosevent.AttackDetected
	ddosevent.Detected.Subscribe(bus, func(e *ddosevent.AttackDetected) { detected = e })

	// One tick of flood on "uplink", then four quiet ticks. The interval closes on
	// a quiet tick, so only the held peak can carry the flood into the evaluation.
	d.onRates([]trafficstat.InterfaceEntry{
		{Name: "lan", RxPps: 10, RxBps: 10},
		{Name: "uplink", RxPps: 100000, RxBps: 100000},
	})
	for range 4 {
		d.onRates([]trafficstat.InterfaceEntry{
			{Name: "lan", RxPps: 10, RxBps: 10},
			{Name: "uplink", RxPps: 10, RxBps: 10},
		})
	}
	d.wg.Wait()

	if detected == nil {
		t.Fatal("a flood shorter than the check-interval was lost: the evaluation must read the interval peak")
	}
	if detected.Interface != "uplink" {
		t.Errorf("attack attributed to %q, want uplink (the interface the peak was seen on)", detected.Interface)
	}
}

// VALIDATES: the periodic baseline save stays on the feed tick, so it keeps its
// ~5 minute cadence at a check-interval that does not divide baselineSaveInterval.
// PREVENTS: moving the save under the evaluation. baselineSaveInterval is 300
// ticks and the trigger is a modulo, so at check-interval 7 no evaluation tick is
// ever a multiple of 300 and the crash-safety save would never run again.
func TestBaselineSaveFollowsTheFeedTickNotTheEvaluation(t *testing.T) {
	useBaselineStore(t)
	cfg := intervalTestConfig(t, `{"ddos":{"detect":{"enabled":"true","check-interval":"7",`+
		`"absolute-floor":"1000000","baseline-window":"10","startup-grace":"0"}}}`)

	d := newDetector(cfg, newDTestBus(), nil)
	for range baselineSaveInterval {
		d.onRates([]trafficstat.InterfaceEntry{{Name: "xe0", RxPps: 10, RxBps: 10}})
	}
	d.wg.Wait()

	if _, ok := loadBaselines(); !ok {
		t.Fatalf("no baseline was persisted after %d feed ticks at check-interval 7", baselineSaveInterval)
	}
}
