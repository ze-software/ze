// Design: docs/architecture/plugin/rib-storage-design.md -- source-owned DOWN recovery.
package ribevents

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"net/netip"
	"strings"
	"sync/atomic"

	"github.com/ze-software/ze/internal/core/family"
)

// RecoveryRequest names one destination and the keys it asks the RIB to decide.
// NLRIs retain native announcement framing; AddPath records identifier presence.
// Each key carries its own excluded-source set, so a departure (the departed
// source excluded) and an owner withdrawal (nobody excluded) differ only in
// data: there is no mode field.
type RecoveryRequest struct {
	Destination netip.Addr    `json:"destination"`
	Family      family.Family `json:"family"`
	Keys        []RecoveryKey `json:"keys"`
	AddPath     bool          `json:"add-path"`
	// SentAddPath selects the destination's negotiated sent identity space.
	SentAddPath bool `json:"sent-add-path"`
}

// RecoveryKey is one route identity and the sources whose candidates the
// election skips for it. An empty set keeps every source eligible, including
// the withdrawing owner's other received paths.
type RecoveryKey struct {
	NLRI     []byte       `json:"nlri"`
	Excluded []netip.Addr `json:"excluded"`
}

// RecoveryOutcome is the RIB's one answer for one requested key. Exactly one
// of Withdraw and Survivors holds: WITHDRAW when no candidate remains,
// SURVIVORS otherwise. Sent is filled only for a SentAddPath request: every
// destination-framed identifier an excluded source owns at the destination
// for the key's prefix. The RIB holds no session generation, so it cannot
// tell a reconnected session's identifier from the departed one's; the
// reactor's writer withdraws only an identifier no current session owns. A
// non-ADD-PATH key leaves Sent empty, because the writer's owed record names
// the one advertisement the decision replaces or withdraws.
type RecoveryOutcome struct {
	NLRI     []byte `json:"nlri"`
	Withdraw bool   `json:"withdraw"`
	// Survivors are in election order: repeated SelectBest over the remainder,
	// because the decision comparator is not transitive under MED and cannot sort.
	Survivors []RecoveryRoute `json:"survivors"`
	Sent      [][]byte        `json:"sent"`
}

// RecoveryRoute owns its bytes. It carries the surviving source's framing, its
// own attributes, its received message ID and its received path identity.
// MessageID MUST be set for every candidate: the recovery write's sent event
// reports it as the entry's source message ID.
type RecoveryRoute struct {
	Source     netip.Addr `json:"source"`
	NLRI       []byte     `json:"nlri"`
	Attributes []byte     `json:"attributes"`
	NextHop    []byte     `json:"next-hop"`
	MessageID  uint64     `json:"message-id"`
	// PathID and AddPath name the path as the source sent it (RFC 7911), so
	// a consumer compares a candidate without re-deriving it from NLRI framing.
	PathID  uint32 `json:"path-id"`
	AddPath bool   `json:"add-path"`
}

// FormatRecoveryKey writes one key as a command argument:
// "<nlri-hex>=<excluded>,<excluded>", with nothing after "=" for an empty set.
func FormatRecoveryKey(key *RecoveryKey) string {
	var b strings.Builder
	b.WriteString(hex.EncodeToString(key.NLRI))
	b.WriteByte('=')
	for i, excluded := range key.Excluded {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(excluded.String())
	}
	return b.String()
}

// ParseRecoveryKey reads FormatRecoveryKey's form.
func ParseRecoveryKey(encoded string) (RecoveryKey, error) {
	nlriHex, sources, found := strings.Cut(encoded, "=")
	if !found {
		return RecoveryKey{}, fmt.Errorf("recovery key %q: want <nlri-hex>=<excluded,...>", encoded)
	}
	raw, err := hex.DecodeString(nlriHex)
	if err != nil {
		return RecoveryKey{}, fmt.Errorf("recovery key NLRI: %w", err)
	}
	if len(raw) == 0 {
		return RecoveryKey{}, fmt.Errorf("recovery key %q: empty NLRI", encoded)
	}
	key := RecoveryKey{NLRI: raw}
	if sources == "" {
		return key, nil
	}
	for source := range strings.SplitSeq(sources, ",") {
		addr, err := netip.ParseAddr(source)
		if err != nil {
			return RecoveryKey{}, fmt.Errorf("recovery key excluded source: %w", err)
		}
		key.Excluded = append(key.Excluded, addr)
	}
	return key, nil
}

var errRecoveryAnswer = errors.New("recovery answer")

// CheckRecoveryAnswer refuses an answer that does not decide every requested
// key exactly once, in request order. A missing outcome is a lookup failure,
// never read as "no survivor".
func CheckRecoveryAnswer(request *RecoveryRequest, outcomes []RecoveryOutcome) error {
	if len(outcomes) != len(request.Keys) {
		return fmt.Errorf("%w: %d outcomes for %d keys", errRecoveryAnswer, len(outcomes), len(request.Keys))
	}
	for i := range outcomes {
		outcome := &outcomes[i]
		if !bytes.Equal(outcome.NLRI, request.Keys[i].NLRI) {
			return fmt.Errorf("%w: outcome %d names another key", errRecoveryAnswer, i)
		}
		if outcome.Withdraw {
			if len(outcome.Survivors) != 0 {
				return fmt.Errorf("%w: key %d is both withdrawn and replaced", errRecoveryAnswer, i)
			}
			continue
		}
		if len(outcome.Survivors) == 0 {
			return fmt.Errorf("%w: key %d has no outcome", errRecoveryAnswer, i)
		}
	}
	return nil
}

// RecoveryOwner publishes one selecting RIB. Its owner MUST Close after event
// delivery stops. Lookup MUST be safe for concurrent use and MUST NOT call the
// forwarding engine. Selection never runs under a destination write lock.
type RecoveryOwner struct {
	Lookup func(RecoveryRequest) ([]RecoveryOutcome, error)
}

var recoveryOwner atomic.Pointer[RecoveryOwner]

// PublishRecovery installs the active RIB without coupling forwarding to its
// plugin package. The caller MUST Close the returned owner after stopping it.
func PublishRecovery(lookup func(RecoveryRequest) ([]RecoveryOutcome, error)) *RecoveryOwner {
	owner := &RecoveryOwner{Lookup: lookup}
	recoveryOwner.Store(owner)
	return owner
}

// Close removes only this owner's publication, never a replacement instance.
func (owner *RecoveryOwner) Close() {
	recoveryOwner.CompareAndSwap(owner, nil)
}

// RecoveryProvider returns the active in-process RIB. Nil selects the same
// producer over command IPC; it never means no surviving route exists.
func RecoveryProvider() *RecoveryOwner { return recoveryOwner.Load() }
