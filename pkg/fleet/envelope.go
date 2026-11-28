// Design: docs/architecture/fleet-config.md — managed config RPC payloads
// Related: version.go — version hash computation
// Related: chain.go — SHA-1 configuration hash chain folding
// Related: diff.go — deterministic line diff/patch payloads
// Related: report.go — inventory and health report payloads and producer registry

package fleet

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"time"
	"unicode"
	"unicode/utf8"
)

// RPC verb constants for managed config protocol.
const (
	VerbConfigFetch   = "config-fetch"
	VerbConfigChanged = "config-changed"
	VerbConfigAck     = "config-ack"
	// VerbConfigHeld is sent by a client that holds: it carries the
	// configuration the client runs up to its hub. A hub built before the verb
	// existed answers it with an unknown-method error.
	VerbConfigHeld = "config-held"
	VerbPing       = "ping"
	// VerbInventoryReport is sent by a managed client to push its bounded
	// hardware and software summary to its hub.
	VerbInventoryReport = "inventory-report"
	// VerbHealthReport is sent by a managed client to push its component health
	// statuses to its hub.
	VerbHealthReport = "health-report"
	// VerbSampleReport is sent by a managed client to push its periodic fixed
	// set of scalar metric readings to its hub.
	VerbSampleReport = "sample-report"
	// VerbFleetAuditReceipts is sent by a managed client to deliver locally
	// recorded offline fleet mode-change audit receipts to its hub.
	VerbFleetAuditReceipts = "fleet-audit-receipts"
)

// Fetch delivery modes, resync requests, and rejection classifications carried
// on managed configuration RPC payloads.
const (
	// FetchModeFull indicates ConfigFetchResponse carries a full base64 config.
	FetchModeFull = "full"
	// FetchModeDiff indicates ConfigFetchResponse carries a base64 line diff
	// over BaseVersion.
	FetchModeDiff = "diff"

	// ResyncModeDiff requests a diff-based resync on chain mismatch first.
	ResyncModeDiff = "diff"
	// ResyncModeFull requests a full configuration replay fallback.
	ResyncModeFull = "full"

	// RejectKindVerify marks a ConfigAck refusal caused by YANG verification.
	RejectKindVerify = "verify"
	// RejectKindCommit marks a ConfigAck refusal caused by staging or commit failure.
	RejectKindCommit = "commit"
	// RejectKindResync marks a ConfigAck refusal caused by diff/chain resync failure.
	RejectKindResync = "resync"
)

// ManagedReply is the shared reply payload carried on managed RPC answers
// (ping, config-ack, config-held, config-changed) so both peers compare their
// expected SHA-1 configuration hash chain head on every reply.
type ManagedReply struct {
	ChainHash string `json:"chain-hash,omitempty"`
}

// DecodeManagedReply decodes a ManagedReply from raw JSON. An empty or
// whitespace-only payload (a legacy empty ok reply from an older peer) returns
// an empty ManagedReply with no error.
func DecodeManagedReply(raw json.RawMessage) (ManagedReply, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return ManagedReply{}, nil
	}
	var reply ManagedReply
	if err := json.Unmarshal(trimmed, &reply); err != nil {
		return ManagedReply{}, fmt.Errorf("fleet: decode managed reply: %w", err)
	}
	return reply, nil
}

// PingRequest is sent by a managed client on VerbPing heartbeats carrying its
// current SHA-1 configuration hash chain head.
type PingRequest struct {
	ChainHash string `json:"chain-hash,omitempty"`
}

// ConfigFetchRequest is sent by a managed client to request its config.
// Version is the client's current config hash, or empty on first boot.
// ChainHash and Resync are optional: ChainHash carries the client's current
// SHA-1 chain head, and Resync requests ResyncModeDiff or ResyncModeFull.
type ConfigFetchRequest struct {
	Version   string `json:"version"`
	ChainHash string `json:"chain-hash,omitempty"`
	Resync    string `json:"resync,omitempty"`
}

// ConfigFetchResponse is the hub's reply to a config-fetch request.
// If the client's version matches, Status is "current" and Config is empty.
// Otherwise, Version is the new hash and Config is base64-encoded config bytes
// (or Diff when Mode is FetchModeDiff).
//
// HeldVersion, DiscardVersion, SampleIntervalSeconds, Mode, BaseVersion, Diff,
// Reanchor, and ChainHash are optional, so a client built before them decodes
// the answer as before and ignores them.
// HeldVersion is the version hash of the configuration the hub holds as this
// client's held copy, and is absent when the hub holds none. DiscardVersion is
// present only when the hub's operator decided that a client running exactly
// that version drops it and applies Config. SampleIntervalSeconds advertises
// the hub's configured metric sample interval in seconds on both changed and
// current replies; zero (omitted) disables sampling. Mode selects FetchModeFull
// (or empty) versus FetchModeDiff, with BaseVersion and base64 Diff carrying
// the line patch, Reanchor signaling a FoldFull base re-anchor, and ChainHash
// carrying the hub's expected 40-hex SHA-1 chain head.
type ConfigFetchResponse struct {
	Version               string `json:"version,omitempty"`
	Config                string `json:"config,omitempty"`
	Status                string `json:"status,omitempty"`
	HeldVersion           string `json:"held-version,omitempty"`
	DiscardVersion        string `json:"discard-version,omitempty"`
	SampleIntervalSeconds int    `json:"sample-interval-seconds,omitempty"`
	Mode                  string `json:"mode,omitempty"`
	BaseVersion           string `json:"base-version,omitempty"`
	Diff                  string `json:"diff,omitempty"`
	Reanchor              bool   `json:"reanchor,omitempty"`
	ChainHash             string `json:"chain-hash,omitempty"`
}

// ConfigChanged is sent by the hub to notify a connected client
// that a new config version is available.
type ConfigChanged struct {
	Version   string `json:"version"`
	ChainHash string `json:"chain-hash,omitempty"`
}

// ConfigAck is sent by the client after receiving and processing a config.
// OK is true if the config was accepted, false if validation or commit failed.
// Error describes the rejection reason when OK is false.
//
// Holding, ChainHash, and RejectKind are optional: Holding says the client
// refused because it holds a configuration of its own; ChainHash reports the
// client's SHA-1 chain head; RejectKind classifies non-OK refusals as
// RejectKindVerify, RejectKindCommit, or RejectKindResync.
type ConfigAck struct {
	Version    string `json:"version"`
	OK         bool   `json:"ok"`
	Error      string `json:"error,omitempty"`
	Holding    bool   `json:"holding,omitempty"`
	ChainHash  string `json:"chain-hash,omitempty"`
	RejectKind string `json:"reject-kind,omitempty"`
}

// ConfigHeld is the payload of config-held: the configuration a holding client
// runs. It names no client: the hub takes the name from the authenticated
// session. Version is the version hash of the decoded Config bytes, which the
// hub recomputes and never trusts. Config is the base64-encoded configuration.
// EditedAt is when that configuration became active on the client, and is
// absent when the client's store records no such time. ChainHash optionally
// carries the client's SHA-1 chain head at hold time.
type ConfigHeld struct {
	Version   string    `json:"version"`
	Config    string    `json:"config"`
	EditedAt  time.Time `json:"edited-at,omitzero"`
	ChainHash string    `json:"chain-hash,omitempty"`
}

// HeldConfigMax is the largest configuration, in raw bytes before base64, that
// a client sends in config-held and that a hub keeps. A client sends no
// larger one, and a hub refuses a larger one whatever the client claims. The
// bound is set by the client's notification loop, which answers no
// config-changed while an upload runs, not by the store.
const HeldConfigMax = 1 << 20

// AckErrorMax is the maximum number of UTF-8 bytes a managed client sends in
// ConfigAck.Error. Longer messages are cut on a UTF-8 rune boundary and marked
// with AckErrorTruncatedSuffix so every legal client request stays well below
// the hub's managed read maximum.
const AckErrorMax = 1024

// AckErrorTruncatedSuffix marks a ConfigAck.Error value that was shortened to
// fit AckErrorMax.
const AckErrorTruncatedSuffix = "... (truncated)"

// BoundAckError shortens text to at most AckErrorMax bytes on a UTF-8 rune
// boundary and appends AckErrorTruncatedSuffix when truncated.
func BoundAckError(text string) string {
	if len(text) <= AckErrorMax {
		return text
	}
	limit := AckErrorMax - len(AckErrorTruncatedSuffix)
	for limit > 0 && !utf8.RuneStart(text[limit]) {
		limit--
	}
	return text[:limit] + AckErrorTruncatedSuffix
}

// HeldRefusedCode is the error code of a hub that takes config-held and
// refused this one copy: too large, a hash that is not the hash of the bytes,
// or a configuration that does not parse. A client then sends no copy of that
// version again, and may send a copy of another one. An error answer with any
// other code, or none, is a hub that does not take config-held at all: a hub
// built before the verb answers it as an unknown method.
const HeldRefusedCode = "config-held-refused"

// Bounds and action identifiers for device-side offline fleet audit receipts
// delivered over VerbFleetAuditReceipts.
const (
	FleetAuditReceiptsOutboxMax    = 1024
	FleetAuditReceiptsBatchMax     = 32
	FleetAuditReceiptEntryBytesMax = 1024
	FleetAuditReceiptBatchBytesMax = 64 << 10
	FleetAuditReceiptAckBytesMax   = 4 << 10
	FleetAuditReceiptEpochMax      = 64
	FleetAuditReceiptActorMax      = 128
	FleetAuditReceiptSurfaceMax    = 32

	FleetAuditReceiptActionDisable = "fleet.disable"
	FleetAuditReceiptActionEnable  = "fleet.enable"

	fleetAuditReceiptOutcomeSuccess = "success"
)

// AuditReceiptEntry is one locally observed fleet mode transition recorded by a
// managed device in its durable receipt outbox for later delivery to the hub.
type AuditReceiptEntry struct {
	Seq     uint64    `json:"seq"`
	Action  string    `json:"action"`
	Actor   string    `json:"actor"`
	Surface string    `json:"surface"`
	EventAt time.Time `json:"event-at"`
	Outcome string    `json:"outcome"`
}

// Validate checks every field and the serialized byte bound of an
// AuditReceiptEntry.
func (e AuditReceiptEntry) Validate() error {
	if e.Seq == 0 {
		return errors.New("fleet: audit receipt seq must be positive")
	}
	switch e.Action {
	case FleetAuditReceiptActionDisable, FleetAuditReceiptActionEnable:
	default:
		return fmt.Errorf("fleet: invalid audit receipt action %q", e.Action)
	}
	if e.EventAt.IsZero() {
		return errors.New("fleet: audit receipt event-at must be set")
	}
	if e.Outcome != fleetAuditReceiptOutcomeSuccess {
		return fmt.Errorf("fleet: invalid audit receipt outcome %q", e.Outcome)
	}
	if !validBoundedPrintable(e.Actor, FleetAuditReceiptActorMax) {
		return errors.New("fleet: audit receipt actor must be non-empty printable UTF-8 within 128 bytes")
	}
	if !validBoundedPrintable(e.Surface, FleetAuditReceiptSurfaceMax) {
		return errors.New("fleet: audit receipt surface must be non-empty printable UTF-8 within 32 bytes")
	}
	raw, err := json.Marshal(e)
	if err != nil {
		return fmt.Errorf("fleet: marshal audit receipt entry: %w", err)
	}
	if len(raw) > FleetAuditReceiptEntryBytesMax {
		return fmt.Errorf("fleet: audit receipt entry exceeds %d bytes (%d)", FleetAuditReceiptEntryBytesMax, len(raw))
	}
	return nil
}

// AuditReceiptBatch is the request payload of VerbFleetAuditReceipts carrying
// an ordered slice of device-side audit receipts and optional overflow-gap
// accounting for one persistent device epoch.
type AuditReceiptBatch struct {
	Epoch   string              `json:"epoch"`
	Entries []AuditReceiptEntry `json:"entries,omitempty"`
	Lost    uint64              `json:"lost,omitempty"`
	GapFrom uint64              `json:"gap-from,omitempty"`
	GapTo   uint64              `json:"gap-to,omitempty"`
}

// Validate checks epoch syntax, batch size limits, overflow-gap consistency,
// strictly increasing entry sequences, and every contained entry.
func (b AuditReceiptBatch) Validate() error {
	if !validAuditReceiptEpoch(b.Epoch) {
		return errors.New("fleet: audit receipt epoch must be non-empty printable ASCII without whitespace within 64 bytes")
	}
	if len(b.Entries) > FleetAuditReceiptsBatchMax {
		return fmt.Errorf("fleet: audit receipt batch exceeds %d entries (%d)", FleetAuditReceiptsBatchMax, len(b.Entries))
	}
	if b.Lost == 0 {
		if b.GapFrom != 0 {
			return errors.New("fleet: audit receipt gap-from requires lost > 0")
		}
		if b.GapTo != 0 {
			return errors.New("fleet: audit receipt gap-to requires lost > 0")
		}
		if len(b.Entries) == 0 {
			return errors.New("fleet: audit receipt batch must contain entries or a declared loss gap")
		}
	} else {
		if b.GapFrom == 0 {
			return errors.New("fleet: audit receipt gap-from must be positive when lost > 0")
		}
		if b.GapTo < b.GapFrom {
			return errors.New("fleet: audit receipt gap-to must be >= gap-from when lost > 0")
		}
		if len(b.Entries) > 0 {
			if b.Entries[0].Seq <= b.GapTo {
				return errors.New("fleet: audit receipt first entry seq must follow gap-to")
			}
		}
	}
	var prevSeq uint64
	for i := range b.Entries {
		entry := b.Entries[i]
		if err := entry.Validate(); err != nil {
			return fmt.Errorf("entry %d: %w", i, err)
		}
		if i > 0 {
			if entry.Seq <= prevSeq {
				return fmt.Errorf("fleet: audit receipt entries must have strictly increasing seq (got %d after %d)", entry.Seq, prevSeq)
			}
		}
		prevSeq = entry.Seq
	}
	return nil
}

// AuditReceiptAck is the hub's response payload for VerbFleetAuditReceipts
// acknowledging the highest contiguous device receipt sequence durably
// committed for Epoch.
type AuditReceiptAck struct {
	Epoch     string `json:"epoch"`
	Committed uint64 `json:"committed"`
}

func validAuditReceiptEpoch(epoch string) bool {
	if epoch == "" {
		return false
	}
	if len(epoch) > FleetAuditReceiptEpochMax {
		return false
	}
	for i := range len(epoch) {
		ch := epoch[i]
		if ch < 0x21 {
			return false
		}
		if ch > 0x7e {
			return false
		}
	}
	return true
}

func validBoundedPrintable(text string, maxBytes int) bool {
	if text == "" {
		return false
	}
	if len(text) > maxBytes {
		return false
	}
	if !utf8.ValidString(text) {
		return false
	}
	for _, r := range text {
		if !unicode.IsPrint(r) {
			return false
		}
	}
	return true
}
