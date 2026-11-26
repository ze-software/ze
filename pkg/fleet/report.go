// Design: docs/architecture/fleet-config.md — managed inventory and health report payloads
// Related: envelope.go — RPC verb constants and config payloads

package fleet

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"
)

// Wire and collection bounds for fleet inventory and health reports.
const (
	// ReportBodyMax is the maximum encoded JSON size in bytes for a fleet
	// inventory or health report, enforced before sending and before decoding.
	ReportBodyMax = 64 << 10
	// InventoryDriversMax is the maximum number of NIC driver entries permitted
	// in an InventoryReport.
	InventoryDriversMax = 128
	// HealthComponentsMax is the maximum number of component rows permitted in
	// a HealthReport.
	HealthComponentsMax = 128
	// ReportNameMax is the maximum UTF-8 byte length for identifier fields such
	// as component names, driver names, producer names, and timestamps.
	ReportNameMax = 128
	// ReportTextMax is the maximum UTF-8 byte length for descriptive text fields
	// such as CPU/DMI models, DMI serials, and health reasons.
	ReportTextMax = 512
	// ReportVersionMax is the maximum UTF-8 byte length for software version
	// strings.
	ReportVersionMax = 128
)

const (
	healthStatusStrUnspecified = "unspecified"
	healthStatusStrHealthy     = "healthy"
	healthStatusStrDegraded    = "degraded"
	healthStatusStrDown        = "down"
)

// InventoryReport is the bounded hardware and software summary pushed by a
// managed client over VerbInventoryReport. It carries no client identity (the
// hub scopes the report to the authenticated session) and excludes raw SMART
// data and block-device serials. Each hardware section carries an explicit
// Available flag so an unreadable section is never mistaken for a zero reading.
type InventoryReport struct {
	CollectedAt     string           `json:"collected-at"`
	SoftwareVersion string           `json:"software-version"`
	CPU             InventoryCPU     `json:"cpu"`
	NIC             InventoryNIC     `json:"nic"`
	Memory          InventoryMemory  `json:"memory"`
	Storage         InventoryStorage `json:"storage"`
	DMI             InventoryDMI     `json:"dmi"`
}

// InventoryCPU summarizes detected CPU resources.
type InventoryCPU struct {
	Available   bool   `json:"available"`
	LogicalCPUs int    `json:"logical-cpus,omitempty"`
	Model       string `json:"model,omitempty"`
}

// InventoryNIC summarizes detected physical network interfaces and their kernel
// drivers.
type InventoryNIC struct {
	Available bool     `json:"available"`
	Count     int      `json:"count,omitempty"`
	Drivers   []string `json:"drivers,omitempty"`
}

// InventoryMemory summarizes system memory capacity and optional ECC error
// counters. ECC is nil when ECC reporting is absent or unavailable.
type InventoryMemory struct {
	Available  bool          `json:"available"`
	TotalBytes uint64        `json:"total-bytes,omitempty"`
	ECC        *InventoryECC `json:"ecc,omitempty"`
}

// InventoryECC reports EDAC ECC error counters when ECC hardware is present.
type InventoryECC struct {
	CorrectableErrors   uint64 `json:"correctable-errors"`
	UncorrectableErrors uint64 `json:"uncorrectable-errors"`
}

// InventoryStorage summarizes block storage capacity without exposing raw SMART
// attributes or drive serial numbers.
type InventoryStorage struct {
	Available   bool   `json:"available"`
	DeviceCount int    `json:"device-count,omitempty"`
	TotalBytes  uint64 `json:"total-bytes,omitempty"`
}

// InventoryDMI summarizes platform identity read from system DMI tables.
type InventoryDMI struct {
	Available bool   `json:"available"`
	Model     string `json:"model,omitempty"`
	Serial    string `json:"serial,omitempty"`
}

// Validate checks that r satisfies all structural, availability, length, UTF-8,
// and timestamp invariants for a wire inventory report.
func (r InventoryReport) Validate() error {
	if err := validateTimestamp("collected-at", r.CollectedAt); err != nil {
		return err
	}
	if r.SoftwareVersion == "" {
		return errors.New("fleet: software-version is required")
	}
	if err := validateOptionalText("software-version", r.SoftwareVersion, ReportVersionMax); err != nil {
		return err
	}
	if err := validateInventoryCPU(r.CPU); err != nil {
		return err
	}
	if err := validateInventoryNIC(r.NIC); err != nil {
		return err
	}
	if err := validateInventoryMemory(r.Memory); err != nil {
		return err
	}
	if err := validateInventoryStorage(r.Storage); err != nil {
		return err
	}
	return validateInventoryDMI(r.DMI)
}

// Clone returns a deep copy of r whose Drivers slice and ECC pointer share no
// memory with the receiver.
func (r InventoryReport) Clone() InventoryReport {
	out := r
	if r.NIC.Drivers != nil {
		out.NIC.Drivers = make([]string, len(r.NIC.Drivers))
		copy(out.NIC.Drivers, r.NIC.Drivers)
	}
	if r.Memory.ECC != nil {
		ecc := *r.Memory.ECC
		out.Memory.ECC = &ecc
	}
	return out
}

// EncodeInventoryReport validates r and encodes it as compact JSON bounded by
// ReportBodyMax.
func EncodeInventoryReport(r InventoryReport) ([]byte, error) {
	if err := r.Validate(); err != nil {
		return nil, err
	}
	data, err := json.Marshal(r)
	if err != nil {
		return nil, fmt.Errorf("fleet: encode inventory report: %w", err)
	}
	if len(data) > ReportBodyMax {
		return nil, fmt.Errorf("fleet: encoded inventory report size %d exceeds max %d", len(data), ReportBodyMax)
	}
	return data, nil
}

// DecodeInventoryReport strictly decodes a compact JSON inventory report,
// enforcing ReportBodyMax, valid UTF-8, no unknown fields, a single JSON value,
// and full field validation.
func DecodeInventoryReport(data []byte) (InventoryReport, error) {
	var out InventoryReport
	if err := decodeStrictReport(data, &out); err != nil {
		return InventoryReport{}, fmt.Errorf("fleet: decode inventory report: %w", err)
	}
	if err := out.Validate(); err != nil {
		return InventoryReport{}, err
	}
	return out, nil
}

func validateInventoryCPU(cpu InventoryCPU) error {
	if cpu.Available {
		if cpu.LogicalCPUs < 0 {
			return fmt.Errorf("fleet: cpu.logical-cpus must be non-negative, got %d", cpu.LogicalCPUs)
		}
		return validateOptionalText("cpu.model", cpu.Model, ReportTextMax)
	}
	if cpu.LogicalCPUs != 0 {
		return errors.New("fleet: unavailable cpu section must not carry logical-cpus")
	}
	if cpu.Model != "" {
		return errors.New("fleet: unavailable cpu section must not carry model")
	}
	return nil
}

func validateInventoryNIC(nic InventoryNIC) error {
	if nic.Available {
		if nic.Count < 0 {
			return fmt.Errorf("fleet: nic.count must be non-negative, got %d", nic.Count)
		}
		if len(nic.Drivers) > InventoryDriversMax {
			return fmt.Errorf("fleet: nic.drivers count %d exceeds max %d", len(nic.Drivers), InventoryDriversMax)
		}
		for _, driver := range nic.Drivers {
			if err := validateRequiredName("nic.drivers entry", driver); err != nil {
				return err
			}
		}
		return nil
	}
	if nic.Count != 0 {
		return errors.New("fleet: unavailable nic section must not carry count")
	}
	if len(nic.Drivers) != 0 {
		return errors.New("fleet: unavailable nic section must not carry drivers")
	}
	return nil
}

func validateInventoryMemory(mem InventoryMemory) error {
	if mem.Available {
		return nil
	}
	if mem.TotalBytes != 0 {
		return errors.New("fleet: unavailable memory section must not carry total-bytes")
	}
	if mem.ECC != nil {
		return errors.New("fleet: unavailable memory section must not carry ecc")
	}
	return nil
}

func validateInventoryStorage(storage InventoryStorage) error {
	if storage.Available {
		if storage.DeviceCount < 0 {
			return fmt.Errorf("fleet: storage.device-count must be non-negative, got %d", storage.DeviceCount)
		}
		return nil
	}
	if storage.DeviceCount != 0 {
		return errors.New("fleet: unavailable storage section must not carry device-count")
	}
	if storage.TotalBytes != 0 {
		return errors.New("fleet: unavailable storage section must not carry total-bytes")
	}
	return nil
}

func validateInventoryDMI(dmi InventoryDMI) error {
	if dmi.Available {
		if err := validateOptionalText("dmi.model", dmi.Model, ReportTextMax); err != nil {
			return err
		}
		return validateOptionalText("dmi.serial", dmi.Serial, ReportTextMax)
	}
	if dmi.Model != "" {
		return errors.New("fleet: unavailable dmi section must not carry model")
	}
	if dmi.Serial != "" {
		return errors.New("fleet: unavailable dmi section must not carry serial")
	}
	return nil
}

// HealthStatus is a typed enum representing a component or overall health
// state. The zero value HealthStatusUnspecified is invalid so an unset field
// cannot pass for a real status.
type HealthStatus uint8

const (
	// HealthStatusUnspecified is the invalid zero value.
	HealthStatusUnspecified HealthStatus = 0
	// HealthStatusHealthy indicates normal operation.
	HealthStatusHealthy HealthStatus = 1
	// HealthStatusDegraded indicates reduced function or warning state.
	HealthStatusDegraded HealthStatus = 2
	// HealthStatusDown indicates a failed component or service.
	HealthStatusDown HealthStatus = 3
)

// String returns the canonical lowercase wire string for s. Use typed
// HealthStatus constants for comparisons rather than String.
func (s HealthStatus) String() string {
	switch s {
	case HealthStatusUnspecified:
		return healthStatusStrUnspecified
	case HealthStatusHealthy:
		return healthStatusStrHealthy
	case HealthStatusDegraded:
		return healthStatusStrDegraded
	case HealthStatusDown:
		return healthStatusStrDown
	default:
		// Open enum: callers outside pkg/fleet can construct an arbitrary uint8.
		return healthStatusStrUnspecified
	}
}

// Valid reports whether s is healthy, degraded, or down.
func (s HealthStatus) Valid() bool {
	switch s {
	case HealthStatusHealthy, HealthStatusDegraded, HealthStatusDown:
		return true
	case HealthStatusUnspecified:
		return false
	default:
		// Open enum: callers outside pkg/fleet can construct an arbitrary uint8.
		return false
	}
}

// ParseHealthStatus parses a JSON or wire health status string into its typed
// HealthStatus value.
func ParseHealthStatus(raw string) (HealthStatus, error) {
	switch raw {
	case healthStatusStrHealthy:
		return HealthStatusHealthy, nil
	case healthStatusStrDegraded:
		return HealthStatusDegraded, nil
	case healthStatusStrDown:
		return HealthStatusDown, nil
	default:
		if len(raw) > ReportNameMax {
			return HealthStatusUnspecified, errors.New("fleet: invalid health status")
		}
		return HealthStatusUnspecified, fmt.Errorf("fleet: invalid health status %q", raw)
	}
}

// MarshalJSON encodes a valid HealthStatus as its JSON string form and refuses
// unspecified or unknown values.
func (s HealthStatus) MarshalJSON() ([]byte, error) {
	if !s.Valid() {
		return nil, fmt.Errorf("fleet: invalid health status %d", uint8(s))
	}
	return json.Marshal(s.String())
}

// UnmarshalJSON decodes a JSON string into a valid HealthStatus. On error, the
// receiver is left unchanged.
func (s *HealthStatus) UnmarshalJSON(data []byte) error {
	var raw string
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("fleet: health status must be a string: %w", err)
	}
	parsed, err := ParseHealthStatus(raw)
	if err != nil {
		return err
	}
	*s = parsed
	return nil
}

func healthStatusRank(s HealthStatus) int {
	switch s {
	case HealthStatusUnspecified:
		return 0
	case HealthStatusHealthy:
		return 1
	case HealthStatusDegraded:
		return 2
	case HealthStatusDown:
		return 3
	default:
		// Open enum: callers outside pkg/fleet can construct an arbitrary uint8.
		return 0
	}
}

// HealthComponent is one component's health observation in a HealthReport. Its
// JSON shape matches the daemon /health endpoint.
type HealthComponent struct {
	Name      string       `json:"name"`
	Status    HealthStatus `json:"status"`
	Reason    string       `json:"reason,omitempty"`
	CheckedAt string       `json:"checked-at"`
}

// HealthReport is the aggregated component health pushed by a managed client
// over VerbHealthReport. Its JSON shape matches the daemon /health endpoint.
type HealthReport struct {
	Status     HealthStatus      `json:"status"`
	Components []HealthComponent `json:"components"`
	CheckedAt  string            `json:"checked-at"`
}

// MarshalJSON encodes r as JSON and always emits "components":[] rather than
// null when no components are present, matching the /health endpoint.
func (r HealthReport) MarshalJSON() ([]byte, error) {
	type wireHealthReport struct {
		Status     HealthStatus      `json:"status"`
		Components []HealthComponent `json:"components"`
		CheckedAt  string            `json:"checked-at"`
	}
	comps := r.Components
	if comps == nil {
		comps = []HealthComponent{}
	}
	return json.Marshal(wireHealthReport{
		Status:     r.Status,
		Components: comps,
		CheckedAt:  r.CheckedAt,
	})
}

// Validate checks that r satisfies all status, timestamp, uniqueness, length,
// UTF-8, and overall-status consistency invariants.
func (r HealthReport) Validate() error {
	if !r.Status.Valid() {
		return fmt.Errorf("fleet: invalid overall health status %d", uint8(r.Status))
	}
	if err := validateTimestamp("checked-at", r.CheckedAt); err != nil {
		return err
	}
	if len(r.Components) > HealthComponentsMax {
		return fmt.Errorf("fleet: health component count %d exceeds max %d", len(r.Components), HealthComponentsMax)
	}
	seen := make(map[string]struct{}, len(r.Components))
	expectedOverall := HealthStatusHealthy
	for _, c := range r.Components {
		if err := validateRequiredName("component name", c.Name); err != nil {
			return err
		}
		if _, dup := seen[c.Name]; dup {
			return fmt.Errorf("fleet: duplicate health component %q", c.Name)
		}
		seen[c.Name] = struct{}{}
		if !c.Status.Valid() {
			return fmt.Errorf("fleet: component %q has invalid status %d", c.Name, uint8(c.Status))
		}
		if err := validateOptionalText("component reason", c.Reason, ReportTextMax); err != nil {
			return err
		}
		if err := validateTimestamp("component checked-at", c.CheckedAt); err != nil {
			return err
		}
		if healthStatusRank(c.Status) > healthStatusRank(expectedOverall) {
			expectedOverall = c.Status
		}
	}
	if r.Status != expectedOverall {
		return fmt.Errorf("fleet: overall health status %s does not match worst component status %s", r.Status.String(), expectedOverall.String())
	}
	return nil
}

// Clone returns a deep copy of r whose Components slice does not share backing
// storage with the receiver.
func (r HealthReport) Clone() HealthReport {
	out := r
	if r.Components != nil {
		out.Components = make([]HealthComponent, len(r.Components))
		copy(out.Components, r.Components)
	}
	return out
}

// EncodeHealthReport validates r and encodes it as compact JSON bounded by
// ReportBodyMax.
func EncodeHealthReport(r HealthReport) ([]byte, error) {
	if err := r.Validate(); err != nil {
		return nil, err
	}
	data, err := json.Marshal(r)
	if err != nil {
		return nil, fmt.Errorf("fleet: encode health report: %w", err)
	}
	if len(data) > ReportBodyMax {
		return nil, fmt.Errorf("fleet: encoded health report size %d exceeds max %d", len(data), ReportBodyMax)
	}
	return data, nil
}

// DecodeHealthReport strictly decodes a compact JSON health report, enforcing
// ReportBodyMax, valid UTF-8, no unknown fields, a single JSON value, and full
// field validation.
func DecodeHealthReport(data []byte) (HealthReport, error) {
	var out HealthReport
	if err := decodeStrictReport(data, &out); err != nil {
		return HealthReport{}, fmt.Errorf("fleet: decode health report: %w", err)
	}
	if err := out.Validate(); err != nil {
		return HealthReport{}, err
	}
	return out, nil
}

// ReportProducer declares one periodic report the managed client collects and
// pushes to its hub.
type ReportProducer struct {
	Name     string
	Method   string
	Interval time.Duration
	Collect  func(ctx context.Context) ([]byte, error)
}

var (
	reportProducersMu sync.RWMutex
	reportProducers   = map[string]ReportProducer{}
)

// RegisterReportProducer registers p in the process-wide managed report
// producer registry. It rejects missing or invalid fields, non-positive
// intervals, nil collectors, duplicate names, and duplicate RPC methods. Safe
// for concurrent use.
func RegisterReportProducer(p ReportProducer) error {
	if err := validateRequiredName("producer name", p.Name); err != nil {
		return err
	}
	if err := validateRequiredName("producer method", p.Method); err != nil {
		return err
	}
	if p.Interval <= 0 {
		return fmt.Errorf("fleet: producer %q interval must be positive, got %v", p.Name, p.Interval)
	}
	if p.Collect == nil {
		return fmt.Errorf("fleet: producer %q collect callback is required", p.Name)
	}
	reportProducersMu.Lock()
	defer reportProducersMu.Unlock()
	if _, exists := reportProducers[p.Method]; exists {
		return fmt.Errorf("fleet: duplicate report producer method %q", p.Method)
	}
	for _, existing := range reportProducers {
		if existing.Name == p.Name {
			return fmt.Errorf("fleet: duplicate report producer name %q", p.Name)
		}
	}
	reportProducers[p.Method] = p
	return nil
}

// MustRegisterReportProducer registers p and panics on error. Intended for
// compile-time init() registration only.
func MustRegisterReportProducer(p ReportProducer) {
	if err := RegisterReportProducer(p); err != nil {
		panic("BUG: fleet: " + strings.TrimPrefix(err.Error(), "fleet: "))
	}
}

// ReportProducers returns a copy of all registered report producers sorted by
// Method. Safe for concurrent use.
func ReportProducers() []ReportProducer {
	reportProducersMu.RLock()
	out := make([]ReportProducer, 0, len(reportProducers))
	for _, p := range reportProducers {
		out = append(out, p)
	}
	reportProducersMu.RUnlock()
	slices.SortFunc(out, func(a, b ReportProducer) int {
		return cmp.Compare(a.Method, b.Method)
	})
	return out
}

// SnapshotReportProducers captures the current producer registry and returns a
// restore function for test cleanup. Callers MUST invoke the returned function
// (for example via t.Cleanup) to restore the captured state. Safe for
// concurrent use.
func SnapshotReportProducers() func() {
	reportProducersMu.RLock()
	saved := maps.Clone(reportProducers)
	reportProducersMu.RUnlock()
	return func() {
		reportProducersMu.Lock()
		reportProducers = maps.Clone(saved)
		reportProducersMu.Unlock()
	}
}

func decodeStrictReport(data []byte, dst any) error {
	if len(data) == 0 {
		return errors.New("empty body")
	}
	if len(data) > ReportBodyMax {
		return fmt.Errorf("body size %d exceeds max %d", len(data), ReportBodyMax)
	}
	if !utf8.Valid(data) {
		return errors.New("body is not valid UTF-8")
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return err
	}
	var trailing any
	if err := dec.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("unexpected trailing JSON value")
		}
		return fmt.Errorf("unexpected trailing data: %w", err)
	}
	return nil
}

func validateTimestamp(field, value string) error {
	if err := validateRequiredName(field, value); err != nil {
		return err
	}
	if _, err := time.Parse(time.RFC3339, value); err != nil {
		return fmt.Errorf("fleet: %s must be RFC3339: %w", field, err)
	}
	return nil
}

func validateRequiredName(field, value string) error {
	if value == "" {
		return fmt.Errorf("fleet: %s is required", field)
	}
	return validateOptionalText(field, value, ReportNameMax)
}

func validateOptionalText(field, value string, maxLen int) error {
	if len(value) > maxLen {
		return fmt.Errorf("fleet: %s length %d exceeds max %d", field, len(value), maxLen)
	}
	if !utf8.ValidString(value) {
		return fmt.Errorf("fleet: %s is not valid UTF-8", field)
	}
	for _, r := range value {
		if unicode.IsControl(r) {
			return fmt.Errorf("fleet: %s contains control character", field)
		}
	}
	return nil
}
