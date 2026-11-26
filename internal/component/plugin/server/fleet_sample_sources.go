// Design: docs/architecture/fleet-config.md — device-side fleet metric sample source collector
// Related: command.go — Dispatcher and CommandContext internal dispatch
// Related: command_registry.go — CommandRegistry full plugin command lookup

package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"math"
	"slices"
	"strings"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"github.com/ze-software/ze/internal/component/aaa"
	plugin "github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/pkg/fleet"
)

// FleetSampleSourcePrefix is the exact command path prefix shared by every
// device-local fleet metric sample source command (`show fleet source <owner>`).
const FleetSampleSourcePrefix = "show fleet source "

// defaultFleetSampleSourceTimeout bounds one device-local sample source command
// invocation when no shorter caller deadline applies.
const defaultFleetSampleSourceTimeout = 2 * time.Second

var (
	errDuplicateSampleFigureOwner      = errors.New("fleet sample figure emitted by multiple non-additive owners")
	errFleetSampleSnapshotEmpty        = errors.New("fleet sample source returned empty output")
	errFleetSampleSnapshotTooLarge     = errors.New("fleet sample source output exceeds maximum body size")
	errFleetSampleSnapshotInvalidUTF8  = errors.New("fleet sample source output is not valid UTF-8")
	errFleetSampleSnapshotTrailingData = errors.New("fleet sample source output has trailing JSON data")
	errFleetSampleSumInvalid           = errors.New("fleet sample additive sum is out of domain")

	fleetSampleSourcesSkippedCount atomic.Uint64
)

type mergedSampleReading struct {
	value float64
	owner string
}

// FleetSampleCollector enumerates live device-local `show fleet source <owner>`
// commands from the full dispatcher and plugin registries (including hidden
// commands) and merges their validated snapshots into a fleet.SampleReport.
//
// Safe for concurrent use once configured before first Collect call.
type FleetSampleCollector struct {
	server           *Server
	dispatcher       *Dispatcher
	perSourceTimeout time.Duration
}

// NewFleetSampleCollector constructs a FleetSampleCollector backed by s so
// dispatched builtin source handlers and plugin proxy handlers receive s on
// CommandContext.Server.
func NewFleetSampleCollector(s *Server) *FleetSampleCollector {
	var d *Dispatcher
	if s != nil {
		d = s.Dispatcher()
	}
	return &FleetSampleCollector{
		server:           s,
		dispatcher:       d,
		perSourceTimeout: defaultFleetSampleSourceTimeout,
	}
}

// NewFleetSampleCollectorWithDispatcher constructs a FleetSampleCollector over
// a standalone Dispatcher with no Server attached. Production daemon wiring
// MUST prefer NewFleetSampleCollector so builtin handlers can reach Server.
func NewFleetSampleCollectorWithDispatcher(d *Dispatcher) *FleetSampleCollector {
	return &FleetSampleCollector{
		dispatcher:       d,
		perSourceTimeout: defaultFleetSampleSourceTimeout,
	}
}

// SetPerSourceTimeout configures the maximum duration allowed for each
// individual source command dispatch. Non-positive values restore the default.
func (c *FleetSampleCollector) SetPerSourceTimeout(timeout time.Duration) {
	if c == nil {
		return
	}
	if timeout <= 0 {
		c.perSourceTimeout = defaultFleetSampleSourceTimeout
		return
	}
	c.perSourceTimeout = timeout
}

func (c *FleetSampleCollector) sourceTimeout() time.Duration {
	if c != nil && c.perSourceTimeout > 0 {
		return c.perSourceTimeout
	}
	return defaultFleetSampleSourceTimeout
}

func (c *FleetSampleCollector) activeDispatcher() *Dispatcher {
	if c == nil {
		return nil
	}
	if c.server != nil {
		if d := c.server.Dispatcher(); d != nil {
			return d
		}
	}
	return c.dispatcher
}

// CollectFleetSampleReport collects one fleet.SampleReport from all live
// `show fleet source <owner>` commands registered on d.
func CollectFleetSampleReport(ctx context.Context, d *Dispatcher, now time.Time) (fleet.SampleReport, error) {
	return NewFleetSampleCollectorWithDispatcher(d).Collect(ctx, now)
}

// CollectFleetSampleReportFromServer collects one fleet.SampleReport from all
// live `show fleet source <owner>` commands registered on s's dispatcher,
// passing s on CommandContext.Server for builtin and proxy handlers.
func CollectFleetSampleReportFromServer(ctx context.Context, s *Server, now time.Time) (fleet.SampleReport, error) {
	return NewFleetSampleCollector(s).Collect(ctx, now)
}

// Collect enumerates all live source owners in deterministic sorted order,
// invokes each once under its own bounded deadline derived from ctx, validates
// every snapshot against the fixed fleet vocabulary, and merges the readings
// into a validated fleet.SampleReport. Missing, failed, timed-out, or malformed
// sources are skipped without synthesizing zero values.
func (c *FleetSampleCollector) Collect(ctx context.Context, now time.Time) (fleet.SampleReport, error) {
	if err := ctx.Err(); err != nil {
		return fleet.SampleReport{}, err
	}
	if now.IsZero() {
		now = time.Now()
	}

	d := c.activeDispatcher()
	if d == nil {
		return emptySampleReport(now), nil
	}

	owners := discoverFleetSampleSources(d)
	if len(owners) == 0 {
		return emptySampleReport(now), nil
	}

	merged := make(map[string]mergedSampleReading, fleet.SampleFiguresMax)
	for _, owner := range owners {
		if err := ctx.Err(); err != nil {
			return fleet.SampleReport{}, err
		}
		snap, ok, err := c.collectOneSource(ctx, d, owner)
		if err != nil {
			return fleet.SampleReport{}, err
		}
		if !ok {
			continue
		}
		if mergeErr := mergeSourceSnapshotInto(merged, snap); mergeErr != nil {
			recordFleetSampleSourceSkip(owner, "conflicting source snapshot", mergeErr)
		}
	}

	return buildSampleReportFromMerged(merged, now)
}

func recordFleetSampleSourceSkip(owner, reason string, err error) {
	fleetSampleSourcesSkippedCount.Add(1)
	logger().Debug("skipping fleet sample source; omitting readings",
		"owner", owner, "reason", reason, "error", err)
}

func (c *FleetSampleCollector) collectOneSource(
	ctx context.Context,
	d *Dispatcher,
	owner string,
) (*fleet.SourceSnapshot, bool, error) {
	sourceCtx, cancel := context.WithTimeout(ctx, c.sourceTimeout())
	defer cancel()

	cmdCtx := &CommandContext{
		Server:         c.server,
		RequestContext: sourceCtx,
		Peer:           "*",
		Username:       aaa.ReservedInternalRPCIdentity,
		Sender:         plugin.OperatorSender(),
	}

	cmdLine := FleetSampleSourcePrefix + owner
	resp, dispatchErr := d.Dispatch(cmdCtx, cmdLine)
	if ctx.Err() != nil {
		return nil, false, ctx.Err()
	}
	if dispatchErr != nil {
		recordFleetSampleSourceSkip(owner, "command dispatch failed", dispatchErr)
		return nil, false, nil
	}
	if _, isStream := plugin.RecordRows(resp); isStream {
		recordFleetSampleSourceSkip(owner, "streamed rows instead of snapshot document", nil)
		return nil, false, nil
	}

	rawJSON, jsonErr := plugin.ResponseJSON(resp, nil)
	if jsonErr != nil {
		recordFleetSampleSourceSkip(owner, "response failed", jsonErr)
		return nil, false, nil
	}
	if rawJSON == "" {
		return nil, false, nil
	}

	snap, decodeErr := decodeAndValidateSourceSnapshot(owner, []byte(rawJSON))
	if decodeErr != nil {
		recordFleetSampleSourceSkip(owner, "malformed snapshot", decodeErr)
		return nil, false, nil
	}
	return snap, true, nil
}

// discoverFleetSampleSources enumerates every live `show fleet source <owner>`
// owner from both the full builtin command map and the full plugin command
// registry (including Hidden entries), deduplicating builtin proxies with their
// matching plugin command. Never reads operator-discovery visible-only lists.
func discoverFleetSampleSources(d *Dispatcher) []string {
	if d == nil {
		return nil
	}

	seen := make(map[string]struct{})
	for _, cmd := range d.Commands() {
		if cmd == nil {
			continue
		}
		if owner, ok := parseFleetSampleSourceOwner(cmd.Name); ok {
			seen[owner] = struct{}{}
		}
	}

	if reg := d.Registry(); reg != nil {
		for _, cmd := range reg.All() {
			if cmd == nil {
				continue
			}
			if cmd.Process == nil {
				continue
			}
			if !cmd.Process.Running() {
				continue
			}
			if owner, ok := parseFleetSampleSourceOwner(cmd.Name); ok {
				seen[owner] = struct{}{}
			}
		}
	}

	if len(seen) == 0 {
		return nil
	}
	owners := make([]string, 0, len(seen))
	for owner := range seen {
		owners = append(owners, owner)
	}
	slices.Sort(owners)
	return owners
}

// parseFleetSampleSourceOwner extracts the single owner token from a command
// name matching `show fleet source <owner>`. Empty or multi-token suffixes are
// rejected.
func parseFleetSampleSourceOwner(cmdName string) (string, bool) {
	lower := strings.ToLower(strings.TrimSpace(cmdName))
	if !strings.HasPrefix(lower, FleetSampleSourcePrefix) {
		return "", false
	}
	owner := lower[len(FleetSampleSourcePrefix):]
	if owner == "" {
		return "", false
	}
	if strings.ContainsAny(owner, " \t\r\n") {
		return "", false
	}
	if err := validateCommandToken(lower, owner); err != nil {
		return "", false
	}
	return owner, true
}

func decodeAndValidateSourceSnapshot(expectedOwner string, raw []byte) (*fleet.SourceSnapshot, error) {
	if len(raw) == 0 {
		return nil, errFleetSampleSnapshotEmpty
	}
	if len(raw) > fleet.SampleReportBodyMax {
		return nil, fmt.Errorf("%w: %d > %d", errFleetSampleSnapshotTooLarge, len(raw), fleet.SampleReportBodyMax)
	}
	if !utf8.Valid(raw) {
		return nil, errFleetSampleSnapshotInvalidUTF8
	}

	var snap fleet.SourceSnapshot
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&snap); err != nil {
		return nil, fmt.Errorf("decode source snapshot: %w", err)
	}
	var trailing any
	if err := dec.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return nil, errFleetSampleSnapshotTrailingData
		}
		return nil, fmt.Errorf("%w: %w", errFleetSampleSnapshotTrailingData, err)
	}
	if err := fleet.ValidateSourceSnapshot(expectedOwner, &snap); err != nil {
		return nil, err
	}
	return &snap, nil
}

// mergeSourceSnapshots merges a slice of validated source snapshots into one
// canonical fleet.SampleReport, summing only registered additive figures across
// distinct owners and rejecting duplicate non-additive ownership.
func mergeSourceSnapshots(snapshots []*fleet.SourceSnapshot, now time.Time) (fleet.SampleReport, error) {
	if now.IsZero() {
		now = time.Now()
	}
	merged := make(map[string]mergedSampleReading, fleet.SampleFiguresMax)
	for _, snap := range snapshots {
		if snap == nil {
			continue
		}
		if err := mergeSourceSnapshotInto(merged, snap); err != nil {
			return fleet.SampleReport{}, err
		}
	}
	return buildSampleReportFromMerged(merged, now)
}

func mergeSourceSnapshotInto(merged map[string]mergedSampleReading, snap *fleet.SourceSnapshot) error {
	if snap == nil {
		return nil
	}
	// Stage into a scratch copy of updates so a conflict inside snap rejects
	// snap atomically without leaving partial readings in merged.
	staged := make(map[string]mergedSampleReading, len(snap.Readings))
	for _, reading := range snap.Readings {
		if existing, exists := merged[reading.Figure]; exists {
			if !fleet.IsAdditiveSampleFigure(reading.Figure) {
				return fmt.Errorf("%w: figure %q (owners %q and %q)",
					errDuplicateSampleFigureOwner, reading.Figure, existing.owner, snap.Owner)
			}
			if existing.owner == snap.Owner {
				return fmt.Errorf("%w: figure %q emitted twice by owner %q",
					errDuplicateSampleFigureOwner, reading.Figure, snap.Owner)
			}
			sum := existing.value + reading.Value
			if err := validateMergedFigureSum(reading.Figure, sum); err != nil {
				return err
			}
			staged[reading.Figure] = mergedSampleReading{value: sum, owner: snap.Owner}
			continue
		}
		if _, dup := staged[reading.Figure]; dup {
			return fmt.Errorf("%w: duplicate figure %q in owner %q",
				errDuplicateSampleFigureOwner, reading.Figure, snap.Owner)
		}
		staged[reading.Figure] = mergedSampleReading{value: reading.Value, owner: snap.Owner}
	}
	maps.Copy(merged, staged)
	return nil
}

func validateMergedFigureSum(figure string, sum float64) error {
	if math.IsNaN(sum) {
		return fmt.Errorf("%w: figure %q sum is not finite", errFleetSampleSumInvalid, figure)
	}
	if math.IsInf(sum, 0) {
		return fmt.Errorf("%w: figure %q sum is not finite", errFleetSampleSumInvalid, figure)
	}
	spec, ok := fleet.LookupSampleFigure(figure)
	if !ok {
		return fmt.Errorf("%w: unknown figure %q", errFleetSampleSumInvalid, figure)
	}
	if sum < spec.Min {
		return fmt.Errorf("%w: figure %q sum %g below minimum %g", errFleetSampleSumInvalid, figure, sum, spec.Min)
	}
	if spec.HasMax && sum > spec.Max {
		return fmt.Errorf("%w: figure %q sum %g exceeds maximum %g", errFleetSampleSumInvalid, figure, sum, spec.Max)
	}
	return nil
}

func emptySampleReport(now time.Time) fleet.SampleReport {
	return fleet.SampleReport{
		SchemaVersion: fleet.SampleSchemaVersion,
		DeviceTime:    now.UTC().Format(time.RFC3339),
		Samples:       []fleet.SampleReading{},
	}
}

func buildSampleReportFromMerged(merged map[string]mergedSampleReading, now time.Time) (fleet.SampleReport, error) {
	vocab := fleet.AllSampleFigures()
	samples := make([]fleet.SampleReading, 0, len(merged))
	for i := range vocab {
		name := vocab[i].Name
		entry, ok := merged[name]
		if !ok {
			continue
		}
		samples = append(samples, fleet.SampleReading{
			Figure: name,
			Value:  entry.value,
		})
	}

	report := fleet.SampleReport{
		SchemaVersion: fleet.SampleSchemaVersion,
		DeviceTime:    now.UTC().Format(time.RFC3339),
		Samples:       samples,
	}
	if err := report.Validate(); err != nil {
		return fleet.SampleReport{}, err
	}
	return report, nil
}
