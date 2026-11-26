// Design: docs/architecture/fleet-config.md — managed metric sample wire types and figure vocabulary
// Related: envelope.go — RPC verb constants and config-fetch interval advertisement
// Related: report.go — inventory and health report wire codecs

package fleet

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"slices"
	"time"
	"unicode/utf8"
)

// Wire, sampling, retention, and storage bounds for fleet metric history.
const (
	// SampleSchemaVersion is the current wire schema version carried by
	// SampleReport payloads.
	SampleSchemaVersion = 1
	// SampleReportBodyMax is the maximum encoded JSON size in bytes for a
	// SampleReport payload, enforced before sending and before decoding.
	SampleReportBodyMax = 8 << 10
	// SampleFiguresMax is the maximum number of named figures permitted in one
	// SampleReport or SourceSnapshot.
	SampleFiguresMax = 16

	// DefaultSampleIntervalSeconds is the default interval advertised to
	// managed clients for periodic metric sample reports.
	DefaultSampleIntervalSeconds = 60
	// MinSampleIntervalSeconds is the shortest non-zero sample interval in
	// seconds accepted by hub configuration.
	MinSampleIntervalSeconds = 1
	// MaxSampleIntervalSeconds is the longest sample interval in seconds
	// accepted by hub configuration.
	MaxSampleIntervalSeconds = 3600

	// MinHistoryRetentionDays is the shortest retention horizon in days.
	MinHistoryRetentionDays = 1
	// MaxHistoryRetentionDays is the longest retention horizon in days.
	MaxHistoryRetentionDays = 7
	// DefaultHistoryRetentionDays is the default retention horizon in days.
	DefaultHistoryRetentionDays = 7

	// DefaultHistoryPerDeviceBytes is the default persisted byte budget per
	// managed device (1 MiB).
	DefaultHistoryPerDeviceBytes = 1 << 20
	// DefaultHistoryHubBytes is the default total persisted byte budget across
	// all managed devices on the hub (2 GiB).
	DefaultHistoryHubBytes = 2 << 30
	// MinHistoryLatestReservePerDeviceBytes is the minimum persisted space
	// reserved per declared managed device for its retained latest observation
	// and metadata (4 KiB).
	MinHistoryLatestReservePerDeviceBytes = 4 << 10
)

// Registered wire figure names in the fixed fleet sample vocabulary.
const (
	FigureCPUUsed              = "cpu-used"
	FigureMemoryUsed           = "memory-used"
	FigureMemoryTotal          = "memory-total"
	FigureStorageUsed          = "storage-used"
	FigureStorageTotal         = "storage-total"
	FigureLoad1m               = "load-1m"
	FigureUptime               = "uptime"
	FigureInterfacesUp         = "interfaces-up"
	FigureInterfacesConfigured = "interfaces-configured"
	FigureTrafficIn            = "traffic-in"
	FigureTrafficOut           = "traffic-out"
	FigureBGPPeersEstablished  = "bgp-peers-established"
	FigureBGPPeersConfigured   = "bgp-peers-configured"
	FigureRoutesHeld           = "routes-held"
	FigureSubscriberSessions   = "subscriber-sessions"
)

// Canonical measurement units for registered fleet sample figures.
const (
	SampleUnitPercent = "percent"
	SampleUnitBytes   = "bytes"
	SampleUnitNumber  = "number"
	SampleUnitSeconds = "seconds"
	SampleUnitCount   = "count"
	SampleUnitBPS     = "bps"
)

// Canonical device-local producer owner names for fleet sample sources.
const (
	SampleOwnerHost    = "host"
	SampleOwnerIface   = "iface"
	SampleOwnerBGPPeer = "bgp-peer"
	SampleOwnerBGPRIB  = "bgp-rib"
	SampleOwnerL2TP    = "l2tp"
	SampleOwnerPPPoE   = "pppoe"
)

const (
	sampleAggStrUnspecified = "unspecified"
	sampleAggStrSum         = "sum"
	sampleAggStrMean        = "mean"
	sampleAggStrRatio       = "ratio"
)

// SampleAggKind defines how a registered sample figure aggregates across
// devices in a set query. The zero value SampleAggUnspecified is invalid so an
// uninitialized figure specification cannot pass for a real aggregation rule.
type SampleAggKind uint8

const (
	// SampleAggUnspecified is the invalid zero value.
	SampleAggUnspecified SampleAggKind = 0
	// SampleAggSum sums measurements across contributing devices.
	SampleAggSum SampleAggKind = 1
	// SampleAggMean averages measurements across contributing devices.
	SampleAggMean SampleAggKind = 2
	// SampleAggRatio derives a ratio from summed numerators and denominators
	// across the same reporting devices.
	SampleAggRatio SampleAggKind = 3
)

// String returns the canonical lowercase name for k. Compare typed
// SampleAggKind constants rather than String.
func (k SampleAggKind) String() string {
	switch k {
	case SampleAggUnspecified:
		return sampleAggStrUnspecified
	case SampleAggSum:
		return sampleAggStrSum
	case SampleAggMean:
		return sampleAggStrMean
	case SampleAggRatio:
		return sampleAggStrRatio
	default:
		// Open enum: callers outside pkg/fleet can construct an arbitrary uint8.
		return sampleAggStrUnspecified
	}
}

// Valid reports whether k is a known aggregation kind.
func (k SampleAggKind) Valid() bool {
	switch k {
	case SampleAggSum, SampleAggMean, SampleAggRatio:
		return true
	case SampleAggUnspecified:
		return false
	default:
		// Open enum: callers outside pkg/fleet can construct an arbitrary uint8.
		return false
	}
}

// SampleFigureSpec describes one figure in the fixed fleet sample vocabulary.
type SampleFigureSpec struct {
	Name           string
	Unit           string
	Owner          string
	AdditiveOwners []string
	Agg            SampleAggKind
	Min            float64
	HasMax         bool
	Max            float64
	Denominator    string
}

var sampleFigureVocabulary = []SampleFigureSpec{
	{
		Name:   FigureCPUUsed,
		Unit:   SampleUnitPercent,
		Owner:  SampleOwnerHost,
		Agg:    SampleAggMean,
		Min:    0,
		HasMax: true,
		Max:    100,
	},
	{
		Name:        FigureMemoryUsed,
		Unit:        SampleUnitBytes,
		Owner:       SampleOwnerHost,
		Agg:         SampleAggSum,
		Min:         0,
		Denominator: FigureMemoryTotal,
	},
	{
		Name:  FigureMemoryTotal,
		Unit:  SampleUnitBytes,
		Owner: SampleOwnerHost,
		Agg:   SampleAggSum,
		Min:   0,
	},
	{
		Name:        FigureStorageUsed,
		Unit:        SampleUnitBytes,
		Owner:       SampleOwnerHost,
		Agg:         SampleAggSum,
		Min:         0,
		Denominator: FigureStorageTotal,
	},
	{
		Name:  FigureStorageTotal,
		Unit:  SampleUnitBytes,
		Owner: SampleOwnerHost,
		Agg:   SampleAggSum,
		Min:   0,
	},
	{
		Name:  FigureLoad1m,
		Unit:  SampleUnitNumber,
		Owner: SampleOwnerHost,
		Agg:   SampleAggMean,
		Min:   0,
	},
	{
		Name:  FigureUptime,
		Unit:  SampleUnitSeconds,
		Owner: SampleOwnerHost,
		Agg:   SampleAggMean,
		Min:   0,
	},
	{
		Name:  FigureInterfacesUp,
		Unit:  SampleUnitCount,
		Owner: SampleOwnerIface,
		Agg:   SampleAggSum,
		Min:   0,
	},
	{
		Name:  FigureInterfacesConfigured,
		Unit:  SampleUnitCount,
		Owner: SampleOwnerIface,
		Agg:   SampleAggSum,
		Min:   0,
	},
	{
		Name:  FigureTrafficIn,
		Unit:  SampleUnitBPS,
		Owner: SampleOwnerIface,
		Agg:   SampleAggSum,
		Min:   0,
	},
	{
		Name:  FigureTrafficOut,
		Unit:  SampleUnitBPS,
		Owner: SampleOwnerIface,
		Agg:   SampleAggSum,
		Min:   0,
	},
	{
		Name:  FigureBGPPeersEstablished,
		Unit:  SampleUnitCount,
		Owner: SampleOwnerBGPPeer,
		Agg:   SampleAggSum,
		Min:   0,
	},
	{
		Name:  FigureBGPPeersConfigured,
		Unit:  SampleUnitCount,
		Owner: SampleOwnerBGPPeer,
		Agg:   SampleAggSum,
		Min:   0,
	},
	{
		Name:  FigureRoutesHeld,
		Unit:  SampleUnitCount,
		Owner: SampleOwnerBGPRIB,
		Agg:   SampleAggSum,
		Min:   0,
	},
	{
		Name:           FigureSubscriberSessions,
		Unit:           SampleUnitCount,
		Owner:          SampleOwnerL2TP,
		AdditiveOwners: []string{SampleOwnerL2TP, SampleOwnerPPPoE},
		Agg:            SampleAggSum,
		Min:            0,
	},
}

var sampleFigureIndex = buildSampleFigureIndex()

func buildSampleFigureIndex() map[string]int {
	idx := make(map[string]int, len(sampleFigureVocabulary))
	for i := range sampleFigureVocabulary {
		idx[sampleFigureVocabulary[i].Name] = i
	}
	return idx
}

func cloneSampleFigureSpec(spec SampleFigureSpec) SampleFigureSpec {
	spec.AdditiveOwners = slices.Clone(spec.AdditiveOwners)
	return spec
}

// LookupSampleFigure returns the specification for a registered figure name.
// The second result is false when name is not in the fixed vocabulary.
func LookupSampleFigure(name string) (SampleFigureSpec, bool) {
	pos, ok := sampleFigureIndex[name]
	if !ok {
		return SampleFigureSpec{}, false
	}
	return cloneSampleFigureSpec(sampleFigureVocabulary[pos]), true
}

// AllSampleFigures returns a fresh slice of every figure in the fixed fleet
// sample vocabulary in canonical order.
func AllSampleFigures() []SampleFigureSpec {
	out := make([]SampleFigureSpec, len(sampleFigureVocabulary))
	for i := range sampleFigureVocabulary {
		out[i] = cloneSampleFigureSpec(sampleFigureVocabulary[i])
	}
	return out
}

// IsAllowedSampleFigureOwner reports whether owner is permitted to emit figure.
func IsAllowedSampleFigureOwner(figure, owner string) bool {
	pos, ok := sampleFigureIndex[figure]
	if !ok {
		return false
	}
	spec := &sampleFigureVocabulary[pos]
	if len(spec.AdditiveOwners) > 0 {
		return slices.Contains(spec.AdditiveOwners, owner)
	}
	return spec.Owner == owner
}

// IsAdditiveSampleFigure reports whether figure combines contributions from
// multiple owners on one device (such as subscriber-sessions across l2tp and
// pppoe).
func IsAdditiveSampleFigure(figure string) bool {
	pos, ok := sampleFigureIndex[figure]
	if !ok {
		return false
	}
	return len(sampleFigureVocabulary[pos].AdditiveOwners) > 1
}

// SampleRatioValue computes a capacity ratio from summed numerators and
// denominators across the same reporting devices. It returns (0, false) when
// either input is non-finite, denSum is not positive, or the quotient is not
// finite.
func SampleRatioValue(numSum, denSum float64) (float64, bool) {
	if !isFiniteFloat(numSum) {
		return 0, false
	}
	if !isFiniteFloat(denSum) {
		return 0, false
	}
	if denSum <= 0 {
		return 0, false
	}
	ratio := numSum / denSum
	if !isFiniteFloat(ratio) {
		return 0, false
	}
	return ratio, true
}

// SampleReading carries one registered scalar figure and its finite numeric
// measurement in a SampleReport.
type SampleReading struct {
	Figure string  `json:"figure"`
	Value  float64 `json:"value"`
}

// SampleReport is the versioned payload pushed by a managed client over
// VerbSampleReport. It carries no client identity (the hub scopes each report
// to the authenticated session). DeviceTime is optional and diagnostic only;
// the hub stamps every accepted report using its own receipt time.
type SampleReport struct {
	SchemaVersion int             `json:"schema-version"`
	DeviceTime    string          `json:"device-time,omitempty"`
	Samples       []SampleReading `json:"samples"`
}

// MarshalJSON encodes r as JSON and always emits "samples":[] rather than null
// when Samples is nil.
func (r SampleReport) MarshalJSON() ([]byte, error) {
	type wireSampleReport struct {
		SchemaVersion int             `json:"schema-version"`
		DeviceTime    string          `json:"device-time,omitempty"`
		Samples       []SampleReading `json:"samples"`
	}
	samples := r.Samples
	if samples == nil {
		samples = []SampleReading{}
	}
	return json.Marshal(wireSampleReport{
		SchemaVersion: r.SchemaVersion,
		DeviceTime:    r.DeviceTime,
		Samples:       samples,
	})
}

// Clone returns a deep copy of r whose Samples slice does not share backing
// storage with the receiver.
func (r SampleReport) Clone() SampleReport {
	out := r
	if r.Samples != nil {
		out.Samples = slices.Clone(r.Samples)
	}
	return out
}

// Validate checks that r satisfies schema version, optional diagnostic
// timestamp, figure bound, vocabulary membership, uniqueness, finiteness, and
// numeric domain invariants.
func (r SampleReport) Validate() error {
	if r.SchemaVersion != SampleSchemaVersion {
		return fmt.Errorf("fleet: unsupported sample schema-version %d (want %d)", r.SchemaVersion, SampleSchemaVersion)
	}
	if r.DeviceTime != "" {
		if err := validateOptionalText("device-time", r.DeviceTime, ReportNameMax); err != nil {
			return err
		}
		if _, err := time.Parse(time.RFC3339Nano, r.DeviceTime); err != nil {
			return fmt.Errorf("fleet: device-time must be RFC3339: %w", err)
		}
	}
	if len(r.Samples) > SampleFiguresMax {
		return fmt.Errorf("fleet: sample count %d exceeds max %d", len(r.Samples), SampleFiguresMax)
	}
	seen := make(map[string]struct{}, len(r.Samples))
	for _, reading := range r.Samples {
		pos, ok := sampleFigureIndex[reading.Figure]
		if !ok {
			if len(reading.Figure) > ReportNameMax {
				return errors.New("fleet: unknown sample figure")
			}
			return fmt.Errorf("fleet: unknown sample figure %q", reading.Figure)
		}
		if _, dup := seen[reading.Figure]; dup {
			return fmt.Errorf("fleet: duplicate sample figure %q", reading.Figure)
		}
		seen[reading.Figure] = struct{}{}
		if err := validateFigureValue(&sampleFigureVocabulary[pos], reading.Value); err != nil {
			return err
		}
	}
	return nil
}

// EncodeSampleReport validates r and encodes it as compact JSON bounded by
// SampleReportBodyMax.
func EncodeSampleReport(r SampleReport) ([]byte, error) {
	if err := r.Validate(); err != nil {
		return nil, err
	}
	data, err := json.Marshal(r)
	if err != nil {
		return nil, fmt.Errorf("fleet: encode sample report: %w", err)
	}
	if len(data) > SampleReportBodyMax {
		return nil, fmt.Errorf("fleet: encoded sample report size %d exceeds max %d", len(data), SampleReportBodyMax)
	}
	return data, nil
}

// DecodeAndValidateSampleReport strictly decodes a compact JSON sample report,
// enforcing SampleReportBodyMax before decoding, valid UTF-8, no unknown
// fields, no trailing JSON tokens, and full SampleReport validation.
func DecodeAndValidateSampleReport(body []byte) (*SampleReport, error) {
	if len(body) == 0 {
		return nil, errors.New("fleet: decode sample report: empty body")
	}
	if len(body) > SampleReportBodyMax {
		return nil, fmt.Errorf("fleet: decode sample report: body size %d exceeds max %d", len(body), SampleReportBodyMax)
	}
	if !utf8.Valid(body) {
		return nil, errors.New("fleet: decode sample report: body is not valid UTF-8")
	}
	var out SampleReport
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&out); err != nil {
		return nil, fmt.Errorf("fleet: decode sample report: %w", err)
	}
	var trailing any
	if err := dec.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return nil, errors.New("fleet: decode sample report: unexpected trailing JSON value")
		}
		return nil, fmt.Errorf("fleet: decode sample report: unexpected trailing data: %w", err)
	}
	if err := out.Validate(); err != nil {
		return nil, err
	}
	return &out, nil
}

// SourceReading carries one scalar figure emitted by a device-local sample
// source command together with its declared unit.
type SourceReading struct {
	Figure string  `json:"figure"`
	Unit   string  `json:"unit"`
	Value  float64 `json:"value"`
}

// SourceSnapshot is the bounded structured answer returned by a device-local
// `show fleet source <owner>` command across the process boundary.
type SourceSnapshot struct {
	Owner    string          `json:"owner"`
	Readings []SourceReading `json:"readings"`
}

// ValidateSourceSnapshot verifies that snap belongs to expectedOwner, stays
// within SampleFiguresMax, contains only registered figures owned by
// expectedOwner with matching canonical units, has no duplicate figures, and
// carries finite, in-domain values.
func ValidateSourceSnapshot(expectedOwner string, snap *SourceSnapshot) error {
	if err := validateRequiredName("expected source owner", expectedOwner); err != nil {
		return err
	}
	if snap == nil {
		return errors.New("fleet: source snapshot is nil")
	}
	if err := validateRequiredName("source owner", snap.Owner); err != nil {
		return err
	}
	if snap.Owner != expectedOwner {
		return fmt.Errorf("fleet: source snapshot owner %q does not match expected owner %q", snap.Owner, expectedOwner)
	}
	if len(snap.Readings) > SampleFiguresMax {
		return fmt.Errorf("fleet: source %q reading count %d exceeds max %d", snap.Owner, len(snap.Readings), SampleFiguresMax)
	}
	seen := make(map[string]struct{}, len(snap.Readings))
	for _, reading := range snap.Readings {
		pos, ok := sampleFigureIndex[reading.Figure]
		if !ok {
			if len(reading.Figure) > ReportNameMax {
				return fmt.Errorf("fleet: source %q emitted unknown figure", snap.Owner)
			}
			return fmt.Errorf("fleet: source %q emitted unknown figure %q", snap.Owner, reading.Figure)
		}
		if _, dup := seen[reading.Figure]; dup {
			return fmt.Errorf("fleet: source %q emitted duplicate figure %q", snap.Owner, reading.Figure)
		}
		seen[reading.Figure] = struct{}{}
		if !IsAllowedSampleFigureOwner(reading.Figure, snap.Owner) {
			return fmt.Errorf("fleet: source %q is not allowed to emit figure %q", snap.Owner, reading.Figure)
		}
		spec := &sampleFigureVocabulary[pos]
		if reading.Unit != spec.Unit {
			return fmt.Errorf("fleet: source %q figure %q unit %q does not match registered unit %q", snap.Owner, reading.Figure, reading.Unit, spec.Unit)
		}
		if err := validateFigureValue(spec, reading.Value); err != nil {
			return err
		}
	}
	return nil
}

func isFiniteFloat(v float64) bool {
	if math.IsNaN(v) {
		return false
	}
	return !math.IsInf(v, 0)
}

func validateFigureValue(spec *SampleFigureSpec, v float64) error {
	if !isFiniteFloat(v) {
		return fmt.Errorf("fleet: figure %q value must be finite", spec.Name)
	}
	if v < spec.Min {
		return fmt.Errorf("fleet: figure %q value %g is below minimum %g", spec.Name, v, spec.Min)
	}
	if spec.HasMax && v > spec.Max {
		return fmt.Errorf("fleet: figure %q value %g exceeds maximum %g", spec.Name, v, spec.Max)
	}
	return nil
}
