// Design: docs/architecture/fleet-config.md -- centralized hub-side fleet audit log functional test driver
// Related: register_managed_fleet_audit.go -- registers managed/fleet-audit-log
// Related: misc_fixture_managed_fleet_hold.go -- fleetHoldRun provisions hub and device stores
// Related: misc_fixture_managed_fleet_alarms.go -- dialFleetAlarmClient and decodeAlarmJSON helpers
// Related: misc_fixture_managed_fleet_divergence_adopt.go -- fleetHoldScenario.restartHub

package fixture

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/pkg/fleet"
)

const (
	fleetAuditShowCmd       = "show fleet audit"
	fleetAuditWaitTimeout   = 45 * time.Second
	fleetAuditReceiptEpoch  = "fixture-audit-epoch-1"
	fleetAuditReceiptActor  = "alice"
	fleetAuditReceiptSurf   = "cli"
	fleetAuditExpectedActor = "device:" + caTrustClientName
)

type fleetAuditFixtureRow struct {
	Seq             uint64            `json:"seq"`
	EventTime       string            `json:"event-time"`
	ReceiptTime     string            `json:"receipt-time"`
	Type            string            `json:"type"`
	Device          string            `json:"device,omitempty"`
	Actor           string            `json:"actor"`
	Surface         string            `json:"surface"`
	Outcome         string            `json:"outcome"`
	Version         string            `json:"version,omitempty"`
	PreviousVersion string            `json:"previous-version,omitempty"`
	DiagnosticCode  string            `json:"diagnostic-code,omitempty"`
	Summary         string            `json:"summary,omitempty"`
	Metadata        map[string]string `json:"metadata,omitempty"`
}

// managedFleetAuditLogDriver validates spec-fleet-3 end-to-end against a real
// ze hub daemon backed by disk ZeFS: live device.connect, config.fetch,
// config.ack, config.push, fleet-audit-receipts ingestion with replay
// deduplication, device.disconnect, command-specific device/type/since pipes
// over SSH CLI, and durable sequence survival across a hub restart.
//
// Args: <hub-ssh-port> <hub-plugin-port> <hub-managed-port>.
func managedFleetAuditLogDriver(ctx context.Context, args []string) error {
	return fleetHoldRun(ctx, "managed/fleet-audit-log", args, false, func(ctx context.Context, s *fleetHoldScenario) error {
		// Step 1: Connect edge-01 (connect + initial config-fetch + config-ack ok).
		client, err := dialFleetAlarmClient(ctx, s.hubAddress, caTrustClientSecret)
		if err != nil {
			return fmt.Errorf("connect managed client %s: %w", caTrustClientName, err)
		}
		defer func() {
			if client != nil {
				_ = client.close() //nolint:errcheck // best-effort cleanup on fixture exit
			}
		}()

		initialRows, rawInitial, err := s.waitAuditRows(ctx, "", func(rows []fleetAuditFixtureRow) bool {
			return hasAuditType(rows, pluginserver.FleetAuditDeviceConnect) &&
				hasAuditType(rows, pluginserver.FleetAuditConfigFetch) &&
				hasAuditType(rows, pluginserver.FleetAuditConfigAck)
		}, "initial connect, fetch, and ack audit entries")
		if err != nil {
			return err
		}
		if err := requireDescendingSeq(initialRows, "initial audit log", rawInitial); err != nil {
			return err
		}

		fetchRow, _ := findAuditRow(initialRows, pluginserver.FleetAuditConfigFetch, caTrustClientName)
		ackRow, _ := findAuditRow(initialRows, pluginserver.FleetAuditConfigAck, caTrustClientName)
		if fetchRow.Version == "" {
			return fmt.Errorf("config.fetch audit entry missing version: %s", rawInitial)
		}
		if ackRow.Version != fetchRow.Version {
			return fmt.Errorf("config.ack version=%q, want %q: %s", ackRow.Version, fetchRow.Version, rawInitial)
		}
		if ackRow.Outcome != outcomeSuccess {
			return fmt.Errorf("config.ack outcome=%q, want %s: %s", ackRow.Outcome, outcomeSuccess, rawInitial)
		}

		// Step 2: Verify device, type, and inclusive since pipes over SSH CLI.
		if err := verifyAuditPipes(ctx, s, &ackRow); err != nil {
			return err
		}

		// Step 3: Trigger a config push (config-changed) and send a
		// fleet-audit-receipts batch twice to verify deduplication.
		if err := s.hubImport(ctx, fleetHoldChangedRouterID); err != nil {
			return err
		}
		if err := sendAndReplayAuditReceipts(ctx, client); err != nil {
			return err
		}

		afterPushRows, rawAfterPush, err := s.waitAuditRows(ctx, "", func(rows []fleetAuditFixtureRow) bool {
			return hasAuditType(rows, pluginserver.FleetAuditConfigPush) &&
				hasAuditType(rows, pluginserver.FleetAuditFleetDisable) &&
				hasAuditType(rows, pluginserver.FleetAuditFleetEnable)
		}, "config.push and deduplicated fleet.disable/enable audit entries")
		if err != nil {
			return err
		}
		if err := requireDescendingSeq(afterPushRows, "post-push audit log", rawAfterPush); err != nil {
			return err
		}
		if err := verifyPushAndReceipts(afterPushRows, s.version(fleetHoldChangedRouterID), rawAfterPush); err != nil {
			return err
		}

		// Step 4: Disconnect client and verify device.disconnect is recorded.
		if err := client.close(); err != nil {
			return fmt.Errorf("disconnect managed client %s: %w", caTrustClientName, err)
		}
		client = nil

		if _, _, err := s.waitAuditRows(ctx, " | type "+pluginserver.FleetAuditDeviceDisconnect, func(rows []fleetAuditFixtureRow) bool {
			return hasAuditType(rows, pluginserver.FleetAuditDeviceDisconnect)
		}, "device.disconnect audit entry"); err != nil {
			return err
		}

		beforeRestartRows, rawBeforeRestart, err := s.queryAuditRows(ctx, "")
		if err != nil {
			return err
		}
		if err := requireDescendingSeq(beforeRestartRows, "pre-restart audit log", rawBeforeRestart); err != nil {
			return err
		}
		beforeMaxSeq := beforeRestartRows[0].Seq

		// Step 5: Restart the real ze hub daemon on the same disk ZeFS store
		// and verify exact sequence preservation and monotonic continuation.
		if err := s.restartHub(ctx); err != nil {
			return fmt.Errorf("restart hub to verify audit log persistence: %w", err)
		}

		afterRestartRows, rawAfterRestart, err := s.queryAuditRows(ctx, "")
		if err != nil {
			return err
		}
		if err := requireSameAuditRows(beforeRestartRows, afterRestartRows, rawBeforeRestart, rawAfterRestart); err != nil {
			return err
		}

		client, err = dialFleetAlarmClient(ctx, s.hubAddress, caTrustClientSecret)
		if err != nil {
			return fmt.Errorf("reconnect managed client %s after hub restart: %w", caTrustClientName, err)
		}
		finalRows, rawFinal, err := s.waitAuditRows(ctx, "", func(rows []fleetAuditFixtureRow) bool {
			for i := range rows {
				if rows[i].Type == pluginserver.FleetAuditDeviceConnect {
					if rows[i].Device == caTrustClientName {
						if rows[i].Seq > beforeMaxSeq {
							return true
						}
					}
				}
			}
			return false
		}, "post-restart device.connect audit entry with monotonic sequence")
		if err != nil {
			return err
		}
		if err := requireDescendingSeq(finalRows, "post-restart reconnect audit log", rawFinal); err != nil {
			return err
		}

		return storageLiveOK("fleet audit log recorded live events, filtered by device/type/since, deduplicated receipts, and survived hub restart")
	})
}

func (s *fleetHoldScenario) queryAuditRows(ctx context.Context, pipes string) ([]fleetAuditFixtureRow, string, error) {
	cmd := fleetAuditShowCmd + pipes + fleetJSONCompactSuffix
	raw, err := storageLiveRequest(ctx, s.hubOperator, cmd)
	if err != nil {
		return nil, raw, fmt.Errorf("%s: %w: %s", cmd, err, raw)
	}
	var rows []fleetAuditFixtureRow
	if err := decodeAuditJSON(raw, &rows); err != nil {
		return nil, raw, fmt.Errorf("%s: %w: %s", cmd, err, raw)
	}
	return rows, raw, nil
}

func decodeAuditJSON(raw string, dst *[]fleetAuditFixtureRow) error {
	start := strings.IndexAny(raw, "[{")
	if start < 0 {
		return errors.New("audit CLI answer carries no JSON document")
	}
	trimmed := bytes.TrimSpace([]byte(raw[start:]))
	if len(trimmed) > 0 {
		if trimmed[0] == '{' {
			var env struct {
				Data json.RawMessage `json:"data"`
			}
			if err := json.Unmarshal(trimmed, &env); err == nil {
				if len(env.Data) > 0 {
					return json.Unmarshal(env.Data, dst)
				}
			}
		}
	}
	return json.NewDecoder(bytes.NewReader(trimmed)).Decode(dst)
}

func (s *fleetHoldScenario) waitAuditRows(
	ctx context.Context,
	pipes string,
	predicate func([]fleetAuditFixtureRow) bool,
	desc string,
) ([]fleetAuditFixtureRow, string, error) {
	waitCtx, cancel := context.WithTimeout(ctx, fleetAuditWaitTimeout)
	defer cancel()

	var matched []fleetAuditFixtureRow
	var lastRaw string
	var lastErr error
	ok := Poll(waitCtx, fleetHoldDoctorAttempts, fleetStatusPollDelay, func() bool {
		rows, raw, err := s.queryAuditRows(waitCtx, pipes)
		if raw != "" {
			lastRaw = raw
		}
		lastErr = err
		if err != nil {
			return false
		}
		if !predicate(rows) {
			return false
		}
		matched = rows
		return true
	})
	if !ok {
		return nil, lastRaw, fmt.Errorf("%s did not appear in show fleet audit (err=%w): %s", desc, lastErr, lastRaw)
	}
	return matched, lastRaw, nil
}

func verifyAuditPipes(ctx context.Context, s *fleetHoldScenario, ackRow *fleetAuditFixtureRow) error {
	devRows, rawDev, err := s.queryAuditRows(ctx, " | device "+caTrustClientName)
	if err != nil {
		return err
	}
	if len(devRows) < 3 {
		return fmt.Errorf("device filter returned %d rows, want >= 3: %s", len(devRows), rawDev)
	}
	for i := range devRows {
		if devRows[i].Device != caTrustClientName {
			return fmt.Errorf("device filter returned unexpected device %q: %s", devRows[i].Device, rawDev)
		}
	}

	typeRows, rawType, err := s.queryAuditRows(ctx, " | type "+pluginserver.FleetAuditConfigFetch)
	if err != nil {
		return err
	}
	if len(typeRows) == 0 {
		return fmt.Errorf("type filter returned zero %s rows: %s", pluginserver.FleetAuditConfigFetch, rawType)
	}
	for i := range typeRows {
		if typeRows[i].Type != pluginserver.FleetAuditConfigFetch {
			return fmt.Errorf("type filter returned unexpected type %q: %s", typeRows[i].Type, rawType)
		}
	}

	sinceInstant, parseErr := time.Parse(time.RFC3339Nano, ackRow.EventTime)
	if parseErr != nil {
		return fmt.Errorf("parse ack event-time %q: %w", ackRow.EventTime, parseErr)
	}
	sinceRows, rawSince, err := s.queryAuditRows(ctx, " | since "+ackRow.EventTime)
	if err != nil {
		return err
	}
	foundAck := false
	for i := range sinceRows {
		rowTime, err := time.Parse(time.RFC3339Nano, sinceRows[i].EventTime)
		if err != nil {
			return fmt.Errorf("parse filtered event-time %q: %w", sinceRows[i].EventTime, err)
		}
		if rowTime.Before(sinceInstant) {
			return fmt.Errorf("since filter returned row at %s before %s: %s", rowTime, sinceInstant, rawSince)
		}
		if sinceRows[i].Seq == ackRow.Seq {
			foundAck = true
		}
	}
	if !foundAck {
		return fmt.Errorf("inclusive since filter omitted boundary entry seq=%d at %s: %s", ackRow.Seq, ackRow.EventTime, rawSince)
	}
	return nil
}

func sendAndReplayAuditReceipts(ctx context.Context, client *fleetAlarmClient) error {
	now := time.Now().UTC().Truncate(time.Second)
	batch := fleet.AuditReceiptBatch{
		Epoch: fleetAuditReceiptEpoch,
		Entries: []fleet.AuditReceiptEntry{
			{
				Seq:     1,
				Action:  fleet.FleetAuditReceiptActionDisable,
				Actor:   fleetAuditReceiptActor,
				Surface: fleetAuditReceiptSurf,
				EventAt: now.Add(-2 * time.Minute),
				Outcome: outcomeSuccess,
			},
			{
				Seq:     2,
				Action:  fleet.FleetAuditReceiptActionEnable,
				Actor:   fleetAuditReceiptActor,
				Surface: fleetAuditReceiptSurf,
				EventAt: now.Add(-1 * time.Minute),
				Outcome: outcomeSuccess,
			},
		},
	}

	for attempt := 1; attempt <= 2; attempt++ {
		callCtx, cancel := context.WithTimeout(ctx, fleetAlarmRPCTimeout)
		raw, err := client.mc.CallRPC(callCtx, fleet.VerbFleetAuditReceipts, batch)
		cancel()
		if err != nil {
			return fmt.Errorf("send %s batch attempt %d: %w", fleet.VerbFleetAuditReceipts, attempt, err)
		}
		var ack fleet.AuditReceiptAck
		if err := json.Unmarshal(raw, &ack); err != nil {
			return fmt.Errorf("decode %s ack attempt %d: %w", fleet.VerbFleetAuditReceipts, attempt, err)
		}
		if ack.Epoch != fleetAuditReceiptEpoch {
			return fmt.Errorf("%s ack attempt %d epoch=%q, want %q", fleet.VerbFleetAuditReceipts, attempt, ack.Epoch, fleetAuditReceiptEpoch)
		}
		if ack.Committed != 2 {
			return fmt.Errorf("%s ack attempt %d committed=%d, want 2", fleet.VerbFleetAuditReceipts, attempt, ack.Committed)
		}
	}
	return nil
}

func verifyPushAndReceipts(rows []fleetAuditFixtureRow, pushedVersion, raw string) error {
	pushRow, ok := findAuditRow(rows, pluginserver.FleetAuditConfigPush, caTrustClientName)
	if !ok {
		return fmt.Errorf("missing config.push entry: %s", raw)
	}
	if pushRow.Version != pushedVersion {
		return fmt.Errorf("config.push version=%q, want %q: %s", pushRow.Version, pushedVersion, raw)
	}

	for _, action := range []string{pluginserver.FleetAuditFleetDisable, pluginserver.FleetAuditFleetEnable} {
		count := 0
		for i := range rows {
			if rows[i].Type != action {
				continue
			}
			if rows[i].Device != caTrustClientName {
				continue
			}
			count++
			if rows[i].Actor != fleetAuditExpectedActor {
				return fmt.Errorf("%s actor=%q, want %q: %s", action, rows[i].Actor, fleetAuditExpectedActor, raw)
			}
			if rows[i].Surface != "device" {
				return fmt.Errorf("%s surface=%q, want device: %s", action, rows[i].Surface, raw)
			}
			if rows[i].Metadata["device-actor"] != fleetAuditReceiptActor {
				return fmt.Errorf("%s metadata device-actor=%q, want %q: %s", action, rows[i].Metadata["device-actor"], fleetAuditReceiptActor, raw)
			}
		}
		if count != 1 {
			return fmt.Errorf("%s count=%d, want 1 (replay must deduplicate): %s", action, count, raw)
		}
	}
	return nil
}

func hasAuditType(rows []fleetAuditFixtureRow, eventType string) bool {
	_, ok := findAuditRow(rows, eventType, caTrustClientName)
	return ok
}

func findAuditRow(rows []fleetAuditFixtureRow, eventType, device string) (fleetAuditFixtureRow, bool) {
	for i := range rows {
		if rows[i].Type != eventType {
			continue
		}
		if rows[i].Device != device {
			continue
		}
		return rows[i], true
	}
	return fleetAuditFixtureRow{}, false
}

func requireDescendingSeq(rows []fleetAuditFixtureRow, label, raw string) error {
	if len(rows) == 0 {
		return fmt.Errorf("%s is empty: %s", label, raw)
	}
	for i := 1; i < len(rows); i++ {
		if rows[i-1].Seq <= rows[i].Seq {
			return fmt.Errorf("%s is not strictly newest-first (seq %d followed by %d): %s",
				label, rows[i-1].Seq, rows[i].Seq, raw)
		}
	}
	return nil
}

func requireSameAuditRows(before, after []fleetAuditFixtureRow, rawBefore, rawAfter string) error {
	if len(after) != len(before) {
		return fmt.Errorf("post-restart audit row count=%d, want %d\nbefore: %s\nafter: %s",
			len(after), len(before), rawBefore, rawAfter)
	}
	for i := range before {
		if after[i].Seq != before[i].Seq {
			return fmt.Errorf("post-restart row[%d] seq=%d, want %d: %s", i, after[i].Seq, before[i].Seq, rawAfter)
		}
		if after[i].Type != before[i].Type {
			return fmt.Errorf("post-restart row[%d] type=%q, want %q: %s", i, after[i].Type, before[i].Type, rawAfter)
		}
		if after[i].Device != before[i].Device {
			return fmt.Errorf("post-restart row[%d] device=%q, want %q: %s", i, after[i].Device, before[i].Device, rawAfter)
		}
		if after[i].Actor != before[i].Actor {
			return fmt.Errorf("post-restart row[%d] actor=%q, want %q: %s", i, after[i].Actor, before[i].Actor, rawAfter)
		}
		if after[i].Outcome != before[i].Outcome {
			return fmt.Errorf("post-restart row[%d] outcome=%q, want %q: %s", i, after[i].Outcome, before[i].Outcome, rawAfter)
		}
		if after[i].Version != before[i].Version {
			return fmt.Errorf("post-restart row[%d] version=%q, want %q: %s", i, after[i].Version, before[i].Version, rawAfter)
		}
	}
	return nil
}
