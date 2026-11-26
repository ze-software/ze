package fleet

import (
	"math"
	"strings"
	"testing"
)

const testSampleTimestamp = "2026-11-26T12:34:56.789Z"

func TestSampleFigureVocabulary(t *testing.T) {
	// VALIDATES: The fixed fleet sample vocabulary exports all 15 registered
	// figures with exact units, owners, aggregation rules, bounds, denominators,
	// and defensive slice cloning on LookupSampleFigure / AllSampleFigures.
	// PREVENTS: Drift in figure names, units, or ownership rules, or callers
	// mutating the package vocabulary through a returned AdditiveOwners slice.
	all := AllSampleFigures()
	if len(all) != 15 {
		t.Fatalf("AllSampleFigures len = %d, want 15", len(all))
	}

	wantSpecs := map[string]struct {
		unit        string
		owner       string
		agg         SampleAggKind
		hasMax      bool
		maxVal      float64
		denominator string
		additive    bool
	}{
		FigureCPUUsed:              {SampleUnitPercent, SampleOwnerHost, SampleAggMean, true, 100, "", false},
		FigureMemoryUsed:           {SampleUnitBytes, SampleOwnerHost, SampleAggSum, false, 0, FigureMemoryTotal, false},
		FigureMemoryTotal:          {SampleUnitBytes, SampleOwnerHost, SampleAggSum, false, 0, "", false},
		FigureStorageUsed:          {SampleUnitBytes, SampleOwnerHost, SampleAggSum, false, 0, FigureStorageTotal, false},
		FigureStorageTotal:         {SampleUnitBytes, SampleOwnerHost, SampleAggSum, false, 0, "", false},
		FigureLoad1m:               {SampleUnitNumber, SampleOwnerHost, SampleAggMean, false, 0, "", false},
		FigureUptime:               {SampleUnitSeconds, SampleOwnerHost, SampleAggMean, false, 0, "", false},
		FigureInterfacesUp:         {SampleUnitCount, SampleOwnerIface, SampleAggSum, false, 0, "", false},
		FigureInterfacesConfigured: {SampleUnitCount, SampleOwnerIface, SampleAggSum, false, 0, "", false},
		FigureTrafficIn:            {SampleUnitBPS, SampleOwnerIface, SampleAggSum, false, 0, "", false},
		FigureTrafficOut:           {SampleUnitBPS, SampleOwnerIface, SampleAggSum, false, 0, "", false},
		FigureBGPPeersEstablished:  {SampleUnitCount, SampleOwnerBGPPeer, SampleAggSum, false, 0, "", false},
		FigureBGPPeersConfigured:   {SampleUnitCount, SampleOwnerBGPPeer, SampleAggSum, false, 0, "", false},
		FigureRoutesHeld:           {SampleUnitCount, SampleOwnerBGPRIB, SampleAggSum, false, 0, "", false},
		FigureSubscriberSessions:   {SampleUnitCount, SampleOwnerL2TP, SampleAggSum, false, 0, "", true},
	}

	for name, want := range wantSpecs {
		spec, ok := LookupSampleFigure(name)
		if !ok {
			t.Fatalf("LookupSampleFigure(%q) returned not found", name)
		}
		if spec.Unit != want.unit {
			t.Errorf("%s unit = %q, want %q", name, spec.Unit, want.unit)
		}
		if spec.Owner != want.owner {
			t.Errorf("%s owner = %q, want %q", name, spec.Owner, want.owner)
		}
		if spec.Agg != want.agg {
			t.Errorf("%s agg = %v, want %v", name, spec.Agg, want.agg)
		}
		if !spec.Agg.Valid() {
			t.Errorf("%s agg %v is not valid", name, spec.Agg)
		}
		if spec.Min != 0 {
			t.Errorf("%s min = %g, want 0", name, spec.Min)
		}
		if spec.HasMax != want.hasMax || spec.Max != want.maxVal {
			t.Errorf("%s max = (%v, %g), want (%v, %g)", name, spec.HasMax, spec.Max, want.hasMax, want.maxVal)
		}
		if spec.Denominator != want.denominator {
			t.Errorf("%s denominator = %q, want %q", name, spec.Denominator, want.denominator)
		}
		if IsAdditiveSampleFigure(name) != want.additive {
			t.Errorf("IsAdditiveSampleFigure(%q) = %v, want %v", name, IsAdditiveSampleFigure(name), want.additive)
		}
	}

	if _, ok := LookupSampleFigure("nonexistent-figure"); ok {
		t.Fatal("expected unknown figure lookup to return false")
	}

	// Ownership checks including additive subscriber-sessions.
	if !IsAllowedSampleFigureOwner(FigureSubscriberSessions, SampleOwnerL2TP) {
		t.Error("l2tp must be allowed for subscriber-sessions")
	}
	if !IsAllowedSampleFigureOwner(FigureSubscriberSessions, SampleOwnerPPPoE) {
		t.Error("pppoe must be allowed for subscriber-sessions")
	}
	if IsAllowedSampleFigureOwner(FigureSubscriberSessions, SampleOwnerHost) {
		t.Error("host must not be allowed for subscriber-sessions")
	}
	if IsAllowedSampleFigureOwner(FigureCPUUsed, SampleOwnerIface) {
		t.Error("iface must not be allowed for cpu-used")
	}
	if IsAllowedSampleFigureOwner("unknown", SampleOwnerHost) {
		t.Error("unknown figure must reject any owner")
	}

	// Mutating returned AdditiveOwners must not corrupt package state.
	sub, _ := LookupSampleFigure(FigureSubscriberSessions)
	sub.AdditiveOwners[0] = "tampered"
	subAgain, _ := LookupSampleFigure(FigureSubscriberSessions)
	if subAgain.AdditiveOwners[0] != SampleOwnerL2TP {
		t.Fatalf("AdditiveOwners aliased package state: got %v", subAgain.AdditiveOwners)
	}
}

func TestSampleAggKind(t *testing.T) {
	// VALIDATES: SampleAggKind distinguishes valid aggregation modes from the
	// invalid zero value and unknown numeric values.
	// PREVENTS: Zero values or out-of-range uint8 casts passing as valid rules.
	if SampleAggUnspecified.Valid() {
		t.Error("SampleAggUnspecified must be invalid")
	}
	if SampleAggKind(99).Valid() {
		t.Error("unknown SampleAggKind must be invalid")
	}
	if SampleAggSum.String() != "sum" || SampleAggMean.String() != "mean" || SampleAggRatio.String() != "ratio" {
		t.Errorf("unexpected String values: %q %q %q", SampleAggSum.String(), SampleAggMean.String(), SampleAggRatio.String())
	}
	if SampleAggUnspecified.String() != "unspecified" || SampleAggKind(99).String() != "unspecified" {
		t.Errorf("unexpected unspecified String values")
	}
}

func TestSampleRatioValue(t *testing.T) {
	// VALIDATES: SampleRatioValue returns the quotient for valid positive
	// denominators and (0, false) for zero, negative, or non-finite inputs.
	// PREVENTS: NaN, Inf, or bogus zero ratios when installed capacity is absent.
	ratio, ok := SampleRatioValue(50, 200)
	if !ok || ratio != 0.25 {
		t.Fatalf("SampleRatioValue(50, 200) = (%g, %v), want (0.25, true)", ratio, ok)
	}
	ratioZeroNum, ok := SampleRatioValue(0, 100)
	if !ok || ratioZeroNum != 0 {
		t.Fatalf("SampleRatioValue(0, 100) = (%g, %v), want (0, true)", ratioZeroNum, ok)
	}

	invalid := []struct {
		name string
		num  float64
		den  float64
	}{
		{"zero denominator", 10, 0},
		{"negative denominator", 10, -5},
		{"nan numerator", math.NaN(), 10},
		{"inf numerator", math.Inf(1), 10},
		{"nan denominator", 10, math.NaN()},
		{"inf denominator", 10, math.Inf(1)},
		{"overflow quotient", math.MaxFloat64, 1e-308},
	}
	for _, tc := range invalid {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := SampleRatioValue(tc.num, tc.den)
			if ok || got != 0 {
				t.Fatalf("SampleRatioValue(%g, %g) = (%g, %v), want (0, false)", tc.num, tc.den, got, ok)
			}
		})
	}
}

func TestSampleReportEncodeDecodeAndValidate(t *testing.T) {
	// VALIDATES: SampleReport round-trips valid reports (including measured
	// zero values and optional RFC3339/RFC3339Nano device-time) and rejects
	// oversized, malformed, unknown-field, trailing-token, wrong-version,
	// duplicate, unknown-figure, non-finite, and out-of-domain payloads.
	// PREVENTS: Hostile or buggy peers injecting invalid samples into history.
	t.Run("valid round-trip", func(t *testing.T) {
		in := SampleReport{
			SchemaVersion: SampleSchemaVersion,
			DeviceTime:    testSampleTimestamp,
			Samples: []SampleReading{
				{Figure: FigureCPUUsed, Value: 0},
				{Figure: FigureMemoryUsed, Value: 512 << 20},
				{Figure: FigureMemoryTotal, Value: 1 << 30},
			},
		}
		body, err := EncodeSampleReport(in)
		if err != nil {
			t.Fatalf("EncodeSampleReport: %v", err)
		}
		raw := string(body)
		for _, want := range []string{`"schema-version":1`, `"device-time":"` + testSampleTimestamp + `"`, `"samples":[`} {
			if !strings.Contains(raw, want) {
				t.Errorf("encoded JSON missing %s: %s", want, raw)
			}
		}
		got, err := DecodeAndValidateSampleReport(body)
		if err != nil {
			t.Fatalf("DecodeAndValidateSampleReport: %v", err)
		}
		if got.SchemaVersion != in.SchemaVersion || got.DeviceTime != in.DeviceTime || len(got.Samples) != len(in.Samples) {
			t.Fatalf("decoded mismatch: got %+v, want %+v", got, in)
		}
		for i := range in.Samples {
			if got.Samples[i] != in.Samples[i] {
				t.Errorf("sample[%d] = %+v, want %+v", i, got.Samples[i], in.Samples[i])
			}
		}

		// Clone isolates backing slice.
		cloned := got.Clone()
		cloned.Samples[0].Value = 99
		if got.Samples[0].Value != 0 {
			t.Fatal("Clone shared Samples backing slice")
		}
	})

	t.Run("empty samples and omitted device-time", func(t *testing.T) {
		in := SampleReport{SchemaVersion: SampleSchemaVersion}
		body, err := EncodeSampleReport(in)
		if err != nil {
			t.Fatalf("EncodeSampleReport empty: %v", err)
		}
		if want := `{"schema-version":1,"samples":[]}`; string(body) != want {
			t.Fatalf("empty report json = %s, want %s", body, want)
		}
		got, err := DecodeAndValidateSampleReport(body)
		if err != nil {
			t.Fatalf("DecodeAndValidateSampleReport empty: %v", err)
		}
		if len(got.Samples) != 0 {
			t.Fatalf("expected 0 samples, got %d", len(got.Samples))
		}
	})

	t.Run("decode rejections", func(t *testing.T) {
		oversized := make([]byte, SampleReportBodyMax+1)
		for i := range oversized {
			oversized[i] = ' '
		}
		tooMany := `{"schema-version":1,"samples":[` +
			strings.Repeat(`{"figure":"cpu-used","value":1},`, SampleFiguresMax) +
			`{"figure":"load-1m","value":1}]}`

		badCases := []struct {
			name string
			body []byte
		}{
			{"empty body", nil},
			{"oversized body", oversized},
			{"invalid UTF-8", []byte("{\"schema-version\":1,\"device-time\":\"\xff\",\"samples\":[]}")},
			{"unknown top-level field", []byte(`{"schema-version":1,"device":"edge-01","samples":[]}`)},
			{"trailing json token", []byte(`{"schema-version":1,"samples":[]}{}`)},
			{"json null", []byte(`null`)},
			{"wrong schema version", []byte(`{"schema-version":2,"samples":[]}`)},
			{"invalid device-time", []byte(`{"schema-version":1,"device-time":"not-a-time","samples":[]}`)},
			{"too many figures", []byte(tooMany)},
			{"unknown figure", []byte(`{"schema-version":1,"samples":[{"figure":"gpu-temp","value":40}]}`)},
			{"duplicate figure", []byte(`{"schema-version":1,"samples":[{"figure":"cpu-used","value":10},{"figure":"cpu-used","value":20}]}`)},
			{"negative cpu-used", []byte(`{"schema-version":1,"samples":[{"figure":"cpu-used","value":-0.1}]}`)},
			{"cpu-used over 100", []byte(`{"schema-version":1,"samples":[{"figure":"cpu-used","value":100.1}]}`)},
			{"negative memory-used", []byte(`{"schema-version":1,"samples":[{"figure":"memory-used","value":-1}]}`)},
		}
		for _, tc := range badCases {
			t.Run(tc.name, func(t *testing.T) {
				if _, err := DecodeAndValidateSampleReport(tc.body); err == nil {
					t.Fatalf("expected error for %s, got nil", tc.name)
				}
			})
		}
	})

	t.Run("non-finite values rejected by Validate", func(t *testing.T) {
		for _, badVal := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
			r := SampleReport{
				SchemaVersion: SampleSchemaVersion,
				Samples:       []SampleReading{{Figure: FigureLoad1m, Value: badVal}},
			}
			if err := r.Validate(); err == nil {
				t.Fatalf("expected error for non-finite value %v", badVal)
			}
		}
	})
}

func TestValidateSourceSnapshot(t *testing.T) {
	// VALIDATES: ValidateSourceSnapshot accepts valid owner-scoped snapshots
	// (including both additive owners for subscriber-sessions) and rejects nil,
	// owner mismatch, wrong unit, disallowed owner, unknown/duplicate figures,
	// non-finite, out-of-domain, and over-bound reading lists.
	// PREVENTS: A buggy or forked source plugin corrupting wire sample reports.
	validHost := &SourceSnapshot{
		Owner: SampleOwnerHost,
		Readings: []SourceReading{
			{Figure: FigureCPUUsed, Unit: SampleUnitPercent, Value: 42.5},
			{Figure: FigureMemoryUsed, Unit: SampleUnitBytes, Value: 1024},
			{Figure: FigureMemoryTotal, Unit: SampleUnitBytes, Value: 4096},
		},
	}
	if err := ValidateSourceSnapshot(SampleOwnerHost, validHost); err != nil {
		t.Fatalf("valid host snapshot rejected: %v", err)
	}

	// Both additive owners may emit subscriber-sessions.
	for _, owner := range []string{SampleOwnerL2TP, SampleOwnerPPPoE} {
		snap := &SourceSnapshot{
			Owner:    owner,
			Readings: []SourceReading{{Figure: FigureSubscriberSessions, Unit: SampleUnitCount, Value: 0}},
		}
		if err := ValidateSourceSnapshot(owner, snap); err != nil {
			t.Fatalf("additive owner %q rejected: %v", owner, err)
		}
	}

	overLimit := make([]SourceReading, SampleFiguresMax+1)
	for i := range overLimit {
		overLimit[i] = SourceReading{Figure: FigureCPUUsed, Unit: SampleUnitPercent, Value: 1}
	}

	badCases := []struct {
		name          string
		expectedOwner string
		snap          *SourceSnapshot
	}{
		{"empty expected owner", "", validHost},
		{"nil snapshot", SampleOwnerHost, nil},
		{"owner mismatch", SampleOwnerIface, validHost},
		{"unknown figure", SampleOwnerHost, &SourceSnapshot{
			Owner:    SampleOwnerHost,
			Readings: []SourceReading{{Figure: "disk-temp", Unit: SampleUnitNumber, Value: 10}},
		}},
		{"duplicate figure", SampleOwnerHost, &SourceSnapshot{
			Owner: SampleOwnerHost,
			Readings: []SourceReading{
				{Figure: FigureLoad1m, Unit: SampleUnitNumber, Value: 1},
				{Figure: FigureLoad1m, Unit: SampleUnitNumber, Value: 2},
			},
		}},
		{"disallowed owner for figure", SampleOwnerIface, &SourceSnapshot{
			Owner:    SampleOwnerIface,
			Readings: []SourceReading{{Figure: FigureCPUUsed, Unit: SampleUnitPercent, Value: 10}},
		}},
		{"unit mismatch", SampleOwnerHost, &SourceSnapshot{
			Owner:    SampleOwnerHost,
			Readings: []SourceReading{{Figure: FigureMemoryUsed, Unit: "kib", Value: 1024}},
		}},
		{"non-finite value", SampleOwnerHost, &SourceSnapshot{
			Owner:    SampleOwnerHost,
			Readings: []SourceReading{{Figure: FigureLoad1m, Unit: SampleUnitNumber, Value: math.NaN()}},
		}},
		{"out of domain cpu", SampleOwnerHost, &SourceSnapshot{
			Owner:    SampleOwnerHost,
			Readings: []SourceReading{{Figure: FigureCPUUsed, Unit: SampleUnitPercent, Value: 101}},
		}},
		{"over max readings", SampleOwnerHost, &SourceSnapshot{
			Owner:    SampleOwnerHost,
			Readings: overLimit,
		}},
	}
	for _, tc := range badCases {
		t.Run(tc.name, func(t *testing.T) {
			if err := ValidateSourceSnapshot(tc.expectedOwner, tc.snap); err == nil {
				t.Fatalf("expected error for %s, got nil", tc.name)
			}
		})
	}
}
