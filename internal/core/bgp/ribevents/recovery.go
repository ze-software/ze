// Design: docs/architecture/plugin/rib-storage-design.md -- source-owned DOWN recovery.
package ribevents

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"net/netip"
	"strconv"
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
	// Limit bounds each key's survivors, and zero bounds nothing. On a
	// SentAddPath destination it also selects the answer: zero keeps WITHDRAW
	// plus Sent with no election, while a nonzero Limit asks for an election
	// over every eligible candidate and at most Limit survivors in election
	// order, with Sent empty, because the caller's own admission state names
	// what the destination holds. It is the destination's PATHS-LIMIT bound
	// for the family, so it shares that number's type.
	Limit uint16 `json:"limit"`
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

// recoveryLimitKeyword introduces the optional limit pair in the command form.
const recoveryLimitKeyword = "limit"

// recoveryUsage is the `request bgp rib recovery` argument form, quoted by
// every refusal so the caller sees the shape it must write.
const recoveryUsage = "usage: request bgp rib recovery <destination> <family> <add-path> <sent-add-path> " +
	"[limit <paths>] <nlri-hex>=<excluded,...> [<nlri-hex>=<excluded,...> ...]"

// AppendRecoveryArgs writes request in the `request bgp rib recovery` form
// that ParseRecoveryArgs reads, so the reactor's IPC writer and the RIB's
// command parser share one declaration of it. The limit pair is written only
// for a nonzero Limit: an unlimited request carries none.
// Draft-abraitis-idr-addpath-paths-limit Section 1.
func AppendRecoveryArgs(args []string, request *RecoveryRequest) []string {
	args = append(args, request.Destination.String(), request.Family.String(),
		strconv.FormatBool(request.AddPath), strconv.FormatBool(request.SentAddPath))
	if request.Limit > 0 {
		args = append(args, recoveryLimitKeyword, strconv.Itoa(int(request.Limit)))
	}
	for i := range request.Keys {
		args = append(args, FormatRecoveryKey(&request.Keys[i]))
	}
	return args
}

// ParseRecoveryArgs reads AppendRecoveryArgs's form. A request names at least
// one key, and a stated limit is 1 to 65535: zero is written by omitting the
// pair, so "limit 0" is a malformed request, never an unbounded one.
// Draft-abraitis-idr-addpath-paths-limit Section 1.
func ParseRecoveryArgs(args []string) (RecoveryRequest, error) {
	if len(args) < 5 {
		return RecoveryRequest{}, fmt.Errorf("request bgp rib recovery needs at least 5 arguments, got %d: %s", len(args), recoveryUsage)
	}
	destination, err := netip.ParseAddr(args[0])
	if err != nil {
		return RecoveryRequest{}, fmt.Errorf("recovery destination: %w", err)
	}
	fam, known := family.LookupFamily(args[1])
	if !known {
		return RecoveryRequest{}, fmt.Errorf("recovery: unknown family %q", args[1])
	}
	addPath, err := strconv.ParseBool(args[2])
	if err != nil {
		return RecoveryRequest{}, fmt.Errorf("recovery ADD-PATH: %w", err)
	}
	sentAddPath, err := strconv.ParseBool(args[3])
	if err != nil {
		return RecoveryRequest{}, fmt.Errorf("recovery sent ADD-PATH: %w", err)
	}
	request := RecoveryRequest{Destination: destination, Family: fam, AddPath: addPath, SentAddPath: sentAddPath}
	keys := args[4:]
	if keys[0] == recoveryLimitKeyword {
		if request.Limit, err = parseRecoveryLimit(keys[1:]); err != nil {
			return RecoveryRequest{}, err
		}
		keys = keys[2:]
	}
	if len(keys) == 0 {
		return RecoveryRequest{}, fmt.Errorf("request bgp rib recovery names no key: %s", recoveryUsage)
	}
	request.Keys = make([]RecoveryKey, 0, len(keys))
	for _, encoded := range keys {
		key, err := ParseRecoveryKey(encoded)
		if err != nil {
			return RecoveryRequest{}, err
		}
		request.Keys = append(request.Keys, key)
	}
	return request, nil
}

// parseRecoveryLimit reads the value after the limit keyword.
func parseRecoveryLimit(rest []string) (uint16, error) {
	if len(rest) == 0 {
		return 0, fmt.Errorf("recovery limit has no value: %s", recoveryUsage)
	}
	limit, err := strconv.ParseUint(rest[0], 10, 16)
	if err != nil {
		return 0, fmt.Errorf("recovery limit %q is not a path count from 1 to 65535: %w", rest[0], err)
	}
	if limit == 0 {
		return 0, fmt.Errorf("recovery limit 0 is not a path count: omit the limit pair for an unbounded request")
	}
	return uint16(limit), nil
}

var errRecoveryAnswer = errors.New("recovery answer")

// CheckRecoveryAnswer refuses an answer that does not decide every requested
// key exactly once, in request order, or that answers a limited request with
// more survivors than its Limit. A missing outcome is a lookup failure, never
// read as "no survivor".
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
		// Draft-abraitis-idr-addpath-paths-limit Section 1: a limited answer
		// relays at most the destination's bound.
		if limit := int(request.Limit); limit > 0 && len(outcome.Survivors) > limit {
			return fmt.Errorf("%w: key %d has %d survivors over limit %d", errRecoveryAnswer, i, len(outcome.Survivors), limit)
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
