// Design: docs/architecture/ospf/ospf-ext-9-graceful-restart.md -- GR restart-fact non-volatile storage.
// Related: auth_keystore.go -- shared daemon state client.
// RFC: rfc/short/rfc3623.md sec 2.1 (store the restart fact + grace period in NVS),
//
//	rfc/short/rfc5187.md sec 3.1 (LSA-ID->prefix preservation) / sec 3.2 (Interface-ID
//	preservation), persisted alongside the fact for the OSPFv3 family.
//
// Restart facts are sent through the daemon-owned state RPC, keyed per engine.
package ospf

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// grRestartFactKeyPrefix is the key prefix for a GR restart fact. The full key
// appends a per-engine suffix (address family + instance) so each engine owns its own fact.
const grRestartFactKeyPrefix = "meta/ospf/gr-fact-"

// restartFact is the RFC 3623 sec 2.1 persisted restart record (plus the RFC 5187 sec 3.1 /
// sec 3.2 OSPFv3 preservation maps). It is JSON-encoded (a small, occasional, non-wire blob).
type restartFact struct {
	// Restarting is true while a planned graceful restart is in flight. A cleared fact
	// (Restarting false) or one whose GraceEndUnix has passed is ignored on resume (R-10).
	Restarting bool `json:"restarting"`
	// GraceEndUnix is the wall-clock deadline (Unix seconds) by which the grace period ends.
	GraceEndUnix int64 `json:"grace-end-unix"`
	// Reason is the RFC 3623 sec A restart reason (0 unknown, 1 software restart, 2 reload,
	// 3 switch to redundant CP).
	Reason uint8 `json:"reason"`
	// Expected are the pre-restart Full-adjacency neighbor Router IDs (dotted). The
	// restarter exits when every one of them re-reaches Full (RFC 3623 sec 2.2 trigger 1).
	Expected []string `json:"expected,omitempty"`
	// InterfaceIDs preserves the RFC 5187 sec 3.2 OSPFv3 Interface ID per interface name.
	InterfaceIDs map[string]uint32 `json:"interface-ids,omitempty"`
	// PrefixLSIDs preserves the RFC 5187 sec 3.1 LSA-ID -> prefix correspondence: prefix
	// string -> the arbitrary 32-bit LSA ID assigned to it.
	PrefixLSIDs map[string]uint32 `json:"prefix-lsids,omitempty"`
}

// expired reports whether the fact's grace window has already closed at now: a stale fact a
// resume must ignore and boot normally (RFC 3623, R-10).
func (f restartFact) expired(now time.Time) bool {
	return now.Unix() >= f.GraceEndUnix
}

// active reports whether the fact represents an in-flight restart whose grace window is still
// open, so the resumed engine should enter in-restart mode.
func (f restartFact) active(now time.Time) bool {
	return f.Restarting && !f.expired(now)
}

// writeRestartFact waits for the daemon's durable acknowledgement.
func writeRestartFact(ctx context.Context, client stateClient, key string, f restartFact) error {
	data, err := json.Marshal(f)
	if err != nil {
		return err
	}
	return client.StatePut(ctx, key, data)
}

// readRestartFact separates a cold boot from unreadable or corrupt restart data.
func readRestartFact(ctx context.Context, client stateClient, key string) (restartFact, bool, error) {
	data, found, err := client.StateGet(ctx, key)
	if err != nil {
		return restartFact{}, false, err
	}
	if !found {
		return restartFact{}, false, nil
	}
	var f restartFact
	if err := json.Unmarshal(data, &f); err != nil {
		return restartFact{}, false, &rpc.StateError{Status: rpc.StateCorrupt, Message: fmt.Sprintf("decode restart fact: %v", err)}
	}
	return f, true, nil
}

// clearRestartFact records that no restart is in flight (written on GR exit). It overwrites
// rather than deletes so a later resume reads an explicit not-restarting fact.
func clearRestartFact(ctx context.Context, client stateClient, key string) error {
	return writeRestartFact(ctx, client, key, restartFact{Restarting: false})
}
