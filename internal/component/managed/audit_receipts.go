// Design: docs/architecture/fleet-config.md -- device-side offline fleet mode-change audit receipt outbox and drain
// Related: cmd_fleet.go -- wraps managed mode transitions with executeManagedModeChange
// Related: client.go -- drains retained receipts after authentication and config negotiation
// Related: doctor.go -- reports outbox overflow gaps and unreadable receipt state

package managed

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os/user"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/pkg/fleet"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/zefs"
)

const (
	auditReceiptOutboxVersion  = 1
	auditReceiptLockKey        = "meta/managed/audit-receipts"
	defaultReceiptTimeout      = 10 * time.Second
	auditReceiptOutcomeSuccess = "success"
	auditReceiptDefaultActor   = "system"
	auditReceiptDefaultSurface = "cli"
	auditReceiptEpochBytes     = 16
)

// auditReceiptsMu serializes in-process mode-change write intents, recovery,
// and outbox mutations so a concurrent reconnect drain never discards an active
// write intent before its flag write completes.
var auditReceiptsMu sync.Mutex

type auditReceiptOutbox struct {
	Version        int                       `json:"version"`
	Epoch          string                    `json:"epoch"`
	NextSeq        uint64                    `json:"next-seq"`
	AckedSeq       uint64                    `json:"acked-seq"`
	Lost           uint64                    `json:"lost,omitempty"`
	GapFrom        uint64                    `json:"gap-from,omitempty"`
	GapTo          uint64                    `json:"gap-to,omitempty"`
	Entries        []fleet.AuditReceiptEntry `json:"entries,omitempty"`
	LastOverflowAt time.Time                 `json:"last-overflow-at,omitzero"`
}

type auditModeIntent struct {
	Action        string    `json:"action"`
	Actor         string    `json:"actor"`
	Surface       string    `json:"surface"`
	EventAt       time.Time `json:"event-at"`
	OldManaged    bool      `json:"old-managed"`
	TargetManaged bool      `json:"target-managed"`
}

// recoverAuditReceiptIntent reconciles any leftover recoverable write intent
// stored under zefs.KeyManagedAuditIntent against the current value of
// zefs.KeyInstanceManaged. If the mode transition completed before a crash, the
// receipt is durably appended to zefs.KeyManagedAuditReceipts and the intent is
// removed. If the mode flag still matches OldManaged, the intent is discarded
// without fabricating a receipt.
//
// Safe for concurrent use.
func recoverAuditReceiptIntent(store storage.Storage) (result error) {
	// refuse-by-default: guards drainAuditReceipts -- in-memory test stubs without a store have no intent to recover.
	if store == nil {
		return nil
	}
	auditReceiptsMu.Lock()
	defer auditReceiptsMu.Unlock()

	guard, err := store.AcquireLock(auditReceiptLockKey)
	if err != nil {
		return fmt.Errorf("lock managed audit receipt outbox: %w", err)
	}
	defer func() {
		result = errors.Join(result, guard.Release())
	}()

	return recoverAuditReceiptIntentLocked(guard, time.Now().UTC())
}

// executeManagedModeChange guards a managed-mode flag transition under a
// recoverable write intent so offline disable and enable operations are
// faithfully recorded in zefs.KeyManagedAuditReceipts without fabricating
// receipts for failed transitions or blocking already-committed flag changes.
//
// Safe for concurrent use.
func executeManagedModeChange(
	store storage.Storage,
	targetManaged bool,
	actor, surface string,
	now time.Time,
	writeFlag func() error,
) error {
	if writeFlag == nil {
		return errors.New("managed mode change requires a flag writer")
	}
	// refuse-by-default: guards disableOnDaemonWithAudit -- pure stub unit tests pass a nil store and only run the flag writer.
	if store == nil {
		return writeFlag()
	}
	if now.IsZero() {
		now = time.Now().UTC()
	} else {
		now = now.UTC()
	}

	auditReceiptsMu.Lock()
	defer auditReceiptsMu.Unlock()

	intent, transition, err := prepareModeIntent(store, targetManaged, actor, surface, now)
	if err != nil {
		return err
	}

	if writeErr := writeFlag(); writeErr != nil {
		if transition {
			cleanupModeIntentBestEffort(store)
		}
		return writeErr
	}

	// refuse-by-default: guards disableOnDaemonWithAudit -- a repeated enable or disable that did not change managed mode emits no transition receipt.
	if !transition {
		return nil
	}

	if err := finalizeModeIntent(store, intent, now); err != nil {
		logger().Warn("failed to finalize managed mode audit receipt; write intent retained for recovery",
			"code", CodeDoctorManagedAuditReceiptGap, "action", intent.Action, "error", err)
	}
	return nil
}

// drainAuditReceipts recovers any pending write intent and delivers retained
// offline mode-change receipts (and any unreported overflow gap) to the hub in
// bounded batches of at most fleet.FleetAuditReceiptsBatchMax entries over
// fleet.VerbFleetAuditReceipts. The outbox advances only after receiving a
// matching, in-range hub acknowledgement. Older hubs that answer with an
// unknown-method error leave the outbox untouched and return nil so ordinary
// config synchronization continues unaffected.
//
// Safe for concurrent use.
func drainAuditReceipts(ctx context.Context, mc *rpc.MuxConn, store storage.Storage, timeout time.Duration) error {
	// refuse-by-default: guards runConnection -- absent store or connection means no outbox to drain.
	if store == nil {
		return nil
	}
	if mc == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if timeout <= 0 {
		timeout = defaultReceiptTimeout
	}
	if err := recoverAuditReceiptIntent(store); err != nil {
		return err
	}

	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		batch, batchMaxSeq, sentAckedSeq, hasWork, err := nextReceiptBatch(store)
		if err != nil {
			return err
		}
		// refuse-by-default: guards runConnection -- an empty outbox with no unreported overflow gap has nothing left to deliver.
		if !hasWork {
			return nil
		}

		callCtx, cancel := context.WithTimeout(ctx, timeout)
		raw, rpcErr := mc.CallRPC(callCtx, fleet.VerbFleetAuditReceipts, batch)
		cancel()
		if rpcErr != nil {
			if isUnknownReceiptMethodError(rpcErr) {
				logger().Debug("hub does not support fleet-audit-receipts; keeping local receipt outbox",
					"verb", fleet.VerbFleetAuditReceipts)
				return nil
			}
			return fmt.Errorf("deliver fleet audit receipts: %w", rpcErr)
		}

		ack, err := decodeAuditReceiptAck(raw)
		if err != nil {
			return fmt.Errorf("decode fleet audit receipt ack: %w", err)
		}
		if ack.Epoch != batch.Epoch {
			return fmt.Errorf("fleet audit receipt ack epoch %q does not match outbox epoch %q", ack.Epoch, batch.Epoch)
		}
		if ack.Committed > batchMaxSeq {
			return fmt.Errorf("fleet audit receipt ack committed %d exceeds sent batch max %d", ack.Committed, batchMaxSeq)
		}
		if ack.Committed < sentAckedSeq {
			return fmt.Errorf("fleet audit receipt ack committed %d regressed below %d", ack.Committed, sentAckedSeq)
		}

		advanced, err := applyReceiptAck(store, ack)
		if err != nil {
			return err
		}
		if !advanced {
			return nil
		}
	}
}

// auditReceiptOutboxHealth inspects zefs.KeyManagedAuditReceipts and reports
// cumulative overflow loss, the overflow sequence gap range, the last overflow
// timestamp, and any read or strict-decode error for device doctor reporting.
func auditReceiptOutboxHealth(store storage.Storage) (lost, gapFrom, gapTo uint64, lastOverflowAt time.Time, unreadableErr error) {
	// refuse-by-default: guards checkManagedAuditReceipts -- without a store handle there is no outbox to inspect.
	if store == nil {
		return 0, 0, 0, time.Time{}, nil
	}
	raw, err := store.ReadKey(zefs.KeyManagedAuditReceipts.Pattern)
	if errors.Is(err, fs.ErrNotExist) {
		return 0, 0, 0, time.Time{}, nil
	}
	if err != nil {
		return 0, 0, 0, time.Time{}, err
	}
	outbox, err := decodeAuditReceiptOutbox(raw)
	if err != nil {
		return 0, 0, 0, time.Time{}, err
	}
	return outbox.Lost, outbox.GapFrom, outbox.GapTo, outbox.LastOverflowAt, nil
}

func prepareModeIntent(store storage.Storage, targetManaged bool, actor, surface string, now time.Time) (intent auditModeIntent, transition bool, result error) {
	guard, err := store.AcquireLock(auditReceiptLockKey)
	if err != nil {
		return auditModeIntent{}, false, fmt.Errorf("lock managed audit receipt outbox: %w", err)
	}
	defer func() {
		result = errors.Join(result, guard.Release())
	}()

	if err := recoverAuditReceiptIntentLocked(guard, now); err != nil {
		return auditModeIntent{}, false, err
	}

	oldManaged, err := readManagedFlagLocked(guard)
	if err != nil {
		return auditModeIntent{}, false, fmt.Errorf("read managed mode flag: %w", err)
	}
	if oldManaged == targetManaged {
		return auditModeIntent{}, false, nil
	}

	action := fleet.FleetAuditReceiptActionDisable
	if targetManaged {
		action = fleet.FleetAuditReceiptActionEnable
	}
	intent = auditModeIntent{
		Action:        action,
		Actor:         sanitizeReceiptText(actor, fleet.FleetAuditReceiptActorMax, auditReceiptDefaultActor),
		Surface:       sanitizeReceiptText(surface, fleet.FleetAuditReceiptSurfaceMax, auditReceiptDefaultSurface),
		EventAt:       now,
		OldManaged:    oldManaged,
		TargetManaged: targetManaged,
	}
	encoded, err := json.Marshal(intent)
	if err != nil {
		return auditModeIntent{}, false, fmt.Errorf("marshal managed mode audit intent: %w", err)
	}
	if len(encoded) > zefs.ManagedAuditIntentValueMax {
		return auditModeIntent{}, false, fmt.Errorf("managed mode audit intent exceeds %d bytes (%d)", zefs.ManagedAuditIntentValueMax, len(encoded))
	}
	if err := guard.WriteKey(zefs.KeyManagedAuditIntent.Pattern, encoded); err != nil {
		return auditModeIntent{}, false, fmt.Errorf("write managed mode audit intent: %w", err)
	}
	return intent, true, nil
}

func cleanupModeIntentBestEffort(store storage.Storage) {
	guard, err := store.AcquireLock(auditReceiptLockKey)
	if err != nil {
		logger().Warn("failed to lock managed audit intent for cleanup after flag write error", "code", CodeDoctorManagedAuditReceiptGap, "error", err)
		return
	}
	rmErr := guard.RemoveKey(zefs.KeyManagedAuditIntent.Pattern)
	if rmErr != nil {
		if !errors.Is(rmErr, fs.ErrNotExist) {
			logger().Warn("failed to remove managed audit intent after flag write error", "code", CodeDoctorManagedAuditReceiptGap, "error", rmErr)
		}
	}
	if relErr := guard.Release(); relErr != nil {
		logger().Warn("failed to release managed audit intent cleanup lock", "code", CodeDoctorManagedAuditReceiptGap, "error", relErr)
	}
}

func finalizeModeIntent(store storage.Storage, intent auditModeIntent, now time.Time) (result error) {
	guard, err := store.AcquireLock(auditReceiptLockKey)
	if err != nil {
		return fmt.Errorf("lock managed audit receipt outbox: %w", err)
	}
	defer func() {
		result = errors.Join(result, guard.Release())
	}()

	if err := appendReceiptLocked(guard, intent, now); err != nil {
		return err
	}
	if err := guard.RemoveKey(zefs.KeyManagedAuditIntent.Pattern); err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("remove managed audit intent: %w", err)
		}
	}
	return nil
}

func recoverAuditReceiptIntentLocked(guard storage.WriteGuard, now time.Time) error {
	raw, err := guard.ReadKey(zefs.KeyManagedAuditIntent.Pattern)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read managed audit intent: %w", err)
	}
	intent, err := decodeAuditModeIntent(raw)
	if err != nil {
		return fmt.Errorf("decode managed audit intent: %w", err)
	}
	currentManaged, err := readManagedFlagLocked(guard)
	if err != nil {
		return fmt.Errorf("read managed mode flag during intent recovery: %w", err)
	}
	if currentManaged == intent.TargetManaged {
		if intent.OldManaged != intent.TargetManaged {
			if err := appendReceiptLocked(guard, intent, now); err != nil {
				return err
			}
		}
	}
	if err := guard.RemoveKey(zefs.KeyManagedAuditIntent.Pattern); err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("remove recovered managed audit intent: %w", err)
		}
	}
	return nil
}

func readManagedFlagLocked(guard storage.WriteGuard) (bool, error) {
	raw, err := guard.ReadKey(zefs.KeyInstanceManaged.Pattern)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return storage.ManagedFlagSet(raw), nil
}

func appendReceiptLocked(guard storage.WriteGuard, intent auditModeIntent, now time.Time) error {
	outbox, err := readOrInitOutboxLocked(guard)
	if err != nil {
		return err
	}
	seq := outbox.NextSeq
	outbox.NextSeq++

	eventAt := intent.EventAt.UTC()
	if eventAt.IsZero() {
		eventAt = now.UTC()
	}
	entry := fleet.AuditReceiptEntry{
		Seq:     seq,
		Action:  intent.Action,
		Actor:   sanitizeReceiptText(intent.Actor, fleet.FleetAuditReceiptActorMax, auditReceiptDefaultActor),
		Surface: sanitizeReceiptText(intent.Surface, fleet.FleetAuditReceiptSurfaceMax, auditReceiptDefaultSurface),
		EventAt: eventAt,
		Outcome: auditReceiptOutcomeSuccess,
	}
	if err := entry.Validate(); err != nil {
		return fmt.Errorf("validate managed audit receipt entry: %w", err)
	}

	outbox.Entries = append(outbox.Entries, entry)
	for len(outbox.Entries) > fleet.FleetAuditReceiptsOutboxMax {
		dropped := outbox.Entries[0]
		outbox.Entries = outbox.Entries[1:]
		outbox.Lost++
		if outbox.GapFrom == 0 {
			outbox.GapFrom = dropped.Seq
		} else if outbox.GapTo <= outbox.AckedSeq {
			outbox.GapFrom = dropped.Seq
		}
		if dropped.Seq > outbox.GapTo {
			outbox.GapTo = dropped.Seq
		}
		outbox.LastOverflowAt = now.UTC()
	}

	return writeOutboxLocked(guard, outbox)
}

func readOrInitOutboxLocked(guard storage.WriteGuard) (auditReceiptOutbox, error) {
	raw, err := guard.ReadKey(zefs.KeyManagedAuditReceipts.Pattern)
	if errors.Is(err, fs.ErrNotExist) {
		epoch, genErr := generateAuditReceiptEpoch()
		if genErr != nil {
			return auditReceiptOutbox{}, genErr
		}
		return auditReceiptOutbox{
			Version: auditReceiptOutboxVersion,
			Epoch:   epoch,
			NextSeq: 1,
		}, nil
	}
	if err != nil {
		return auditReceiptOutbox{}, fmt.Errorf("read managed audit receipt outbox: %w", err)
	}
	outbox, err := decodeAuditReceiptOutbox(raw)
	if err != nil {
		return auditReceiptOutbox{}, fmt.Errorf("decode managed audit receipt outbox: %w", err)
	}
	return outbox, nil
}

func writeOutboxLocked(guard storage.WriteGuard, outbox auditReceiptOutbox) error {
	raw, err := json.Marshal(outbox)
	if err != nil {
		return fmt.Errorf("marshal managed audit receipt outbox: %w", err)
	}
	if len(raw) > zefs.ManagedAuditReceiptsValueMax {
		return fmt.Errorf("managed audit receipt outbox exceeds %d bytes (%d)", zefs.ManagedAuditReceiptsValueMax, len(raw))
	}
	if err := guard.WriteKey(zefs.KeyManagedAuditReceipts.Pattern, raw); err != nil {
		return fmt.Errorf("write managed audit receipt outbox: %w", err)
	}
	return nil
}

func nextReceiptBatch(store storage.Storage) (batch fleet.AuditReceiptBatch, batchMaxSeq, sentAckedSeq uint64, hasWork bool, err error) {
	auditReceiptsMu.Lock()
	defer auditReceiptsMu.Unlock()

	raw, readErr := store.ReadKey(zefs.KeyManagedAuditReceipts.Pattern)
	if errors.Is(readErr, fs.ErrNotExist) {
		return fleet.AuditReceiptBatch{}, 0, 0, false, nil
	}
	if readErr != nil {
		return fleet.AuditReceiptBatch{}, 0, 0, false, fmt.Errorf("read managed audit receipt outbox: %w", readErr)
	}
	outbox, decErr := decodeAuditReceiptOutbox(raw)
	if decErr != nil {
		return fleet.AuditReceiptBatch{}, 0, 0, false, fmt.Errorf("decode managed audit receipt outbox: %w", decErr)
	}

	includeGap := false
	if outbox.Lost > 0 {
		if outbox.GapTo > outbox.AckedSeq {
			includeGap = true
		}
	}

	batchMaxSeq = outbox.AckedSeq
	batch = fleet.AuditReceiptBatch{
		Epoch: outbox.Epoch,
	}
	if includeGap {
		batch.Lost = outbox.Lost
		batch.GapFrom = outbox.GapFrom
		batch.GapTo = outbox.GapTo
		if outbox.GapTo > batchMaxSeq {
			batchMaxSeq = outbox.GapTo
		}
	}

	for i := range outbox.Entries {
		entry := outbox.Entries[i]
		if entry.Seq <= outbox.AckedSeq {
			continue
		}
		if includeGap {
			if entry.Seq <= outbox.GapTo {
				continue
			}
		}
		batch.Entries = append(batch.Entries, entry)
		if entry.Seq > batchMaxSeq {
			batchMaxSeq = entry.Seq
		}
		if len(batch.Entries) >= fleet.FleetAuditReceiptsBatchMax {
			break
		}
	}

	if len(batch.Entries) == 0 {
		if !includeGap {
			return fleet.AuditReceiptBatch{}, 0, 0, false, nil
		}
	}

	if err := batch.Validate(); err != nil {
		return fleet.AuditReceiptBatch{}, 0, 0, false, fmt.Errorf("validate outbound audit receipt batch: %w", err)
	}
	return batch, batchMaxSeq, outbox.AckedSeq, true, nil
}

func applyReceiptAck(store storage.Storage, ack fleet.AuditReceiptAck) (advanced bool, result error) {
	auditReceiptsMu.Lock()
	defer auditReceiptsMu.Unlock()

	guard, err := store.AcquireLock(auditReceiptLockKey)
	if err != nil {
		return false, fmt.Errorf("lock managed audit receipt outbox: %w", err)
	}
	defer func() {
		result = errors.Join(result, guard.Release())
	}()

	raw, err := guard.ReadKey(zefs.KeyManagedAuditReceipts.Pattern)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("read managed audit receipt outbox: %w", err)
	}
	outbox, err := decodeAuditReceiptOutbox(raw)
	if err != nil {
		return false, fmt.Errorf("decode managed audit receipt outbox: %w", err)
	}
	if outbox.Epoch != ack.Epoch {
		return false, fmt.Errorf("managed audit receipt outbox epoch changed from %q to %q", ack.Epoch, outbox.Epoch)
	}

	prevAcked := outbox.AckedSeq
	if ack.Committed > outbox.AckedSeq {
		outbox.AckedSeq = ack.Committed
	}

	kept := make([]fleet.AuditReceiptEntry, 0, len(outbox.Entries))
	for i := range outbox.Entries {
		if outbox.Entries[i].Seq > outbox.AckedSeq {
			kept = append(kept, outbox.Entries[i])
		}
	}
	outbox.Entries = kept

	if err := writeOutboxLocked(guard, outbox); err != nil {
		return false, err
	}
	return outbox.AckedSeq > prevAcked, nil
}

func generateAuditReceiptEpoch() (string, error) {
	var raw [auditReceiptEpochBytes]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", fmt.Errorf("generate managed audit receipt epoch: %w", err)
	}
	return hex.EncodeToString(raw[:]), nil
}

func decodeAuditReceiptOutbox(raw []byte) (auditReceiptOutbox, error) {
	var outbox auditReceiptOutbox
	if len(raw) > zefs.ManagedAuditReceiptsValueMax {
		return outbox, fmt.Errorf("audit receipt outbox exceeds %d bytes (%d)", zefs.ManagedAuditReceiptsValueMax, len(raw))
	}
	if !utf8.Valid(raw) {
		return outbox, errors.New("audit receipt outbox contains invalid UTF-8")
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&outbox); err != nil {
		return outbox, err
	}
	if err := dec.Decode(new(any)); !errors.Is(err, io.EOF) {
		return outbox, errors.New("unexpected trailing bytes after audit receipt outbox")
	}
	if outbox.Version != auditReceiptOutboxVersion {
		return outbox, fmt.Errorf("unsupported audit receipt outbox version %d", outbox.Version)
	}
	if outbox.Epoch == "" {
		return outbox, errors.New("audit receipt outbox epoch is empty")
	}
	if len(outbox.Epoch) > fleet.FleetAuditReceiptEpochMax {
		return outbox, fmt.Errorf("audit receipt outbox epoch exceeds %d bytes", fleet.FleetAuditReceiptEpochMax)
	}
	if outbox.NextSeq == 0 {
		return outbox, errors.New("audit receipt outbox next-seq must be positive")
	}
	if len(outbox.Entries) > fleet.FleetAuditReceiptsOutboxMax {
		return outbox, fmt.Errorf("audit receipt outbox entry count %d exceeds %d", len(outbox.Entries), fleet.FleetAuditReceiptsOutboxMax)
	}
	var prevSeq uint64
	for i := range outbox.Entries {
		entry := outbox.Entries[i]
		if err := entry.Validate(); err != nil {
			return outbox, fmt.Errorf("outbox entry %d: %w", i, err)
		}
		if entry.Seq <= prevSeq {
			return outbox, fmt.Errorf("outbox entry %d seq %d is not strictly greater than %d", i, entry.Seq, prevSeq)
		}
		prevSeq = entry.Seq
	}
	return outbox, nil
}

func decodeAuditModeIntent(raw []byte) (auditModeIntent, error) {
	var intent auditModeIntent
	if len(raw) > zefs.ManagedAuditIntentValueMax {
		return intent, fmt.Errorf("audit mode intent exceeds %d bytes (%d)", zefs.ManagedAuditIntentValueMax, len(raw))
	}
	if !utf8.Valid(raw) {
		return intent, errors.New("audit mode intent contains invalid UTF-8")
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&intent); err != nil {
		return intent, err
	}
	if err := dec.Decode(new(any)); !errors.Is(err, io.EOF) {
		return intent, errors.New("unexpected trailing bytes after audit mode intent")
	}
	switch intent.Action {
	case fleet.FleetAuditReceiptActionDisable, fleet.FleetAuditReceiptActionEnable:
	default:
		return intent, fmt.Errorf("invalid audit mode intent action %q", intent.Action)
	}
	if intent.EventAt.IsZero() {
		return intent, errors.New("audit mode intent event-at must be set")
	}
	return intent, nil
}

func decodeAuditReceiptAck(raw json.RawMessage) (fleet.AuditReceiptAck, error) {
	var ack fleet.AuditReceiptAck
	if len(raw) == 0 {
		return ack, errors.New("empty audit receipt ack payload")
	}
	if len(raw) > fleet.FleetAuditReceiptAckBytesMax {
		return ack, fmt.Errorf("audit receipt ack payload exceeds %d bytes (%d)", fleet.FleetAuditReceiptAckBytesMax, len(raw))
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&ack); err != nil {
		return ack, err
	}
	if err := dec.Decode(new(any)); !errors.Is(err, io.EOF) {
		return ack, errors.New("unexpected trailing bytes after audit receipt ack")
	}
	return ack, nil
}

func isUnknownReceiptMethodError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "unknown method")
}

func sanitizeReceiptText(text string, maxBytes int, fallback string) string {
	var b strings.Builder
	for _, r := range text {
		if unicode.IsPrint(r) {
			b.WriteRune(r)
		}
	}
	clean := strings.TrimSpace(b.String())
	if clean == "" {
		return fallback
	}
	if len(clean) <= maxBytes {
		return clean
	}
	limit := maxBytes
	for limit > 0 {
		if utf8.RuneStart(clean[limit]) {
			break
		}
		limit--
	}
	clean = strings.TrimSpace(clean[:limit])
	if clean == "" {
		return fallback
	}
	return clean
}

func localOSActor() string {
	u, err := user.Current()
	if err != nil {
		return "local"
	}
	if u.Username == "" {
		return "local"
	}
	return u.Username
}
