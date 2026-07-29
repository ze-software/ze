// Design: plan/learned/1044-ospf-ext-9-graceful-restart.md -- GR restart-fact non-volatile storage.
// Related: auth_keystore.go -- the pkg/zefs blob-store pattern this reuses.
// RFC: rfc/short/rfc3623.md sec 2.1 (store the restart fact + grace period in NVS),
//
//	rfc/short/rfc5187.md sec 3.1 (LSA-ID->prefix preservation) / sec 3.2 (Interface-ID
//	preservation), persisted alongside the fact for the OSPFv3 family.
//
// The blob is stored via the same pkg/zefs blob store as the RFC 7474 boot count
// (auth_keystore.go), keyed per engine (address family + instance) so a v4 and a v6 engine
// in one process do not clobber each other's fact.
package ospf

import (
	"encoding/json"
	"io/fs"
	"path/filepath"

	"time"

	"github.com/ze-software/ze/internal/core/paths"

	"github.com/ze-software/ze/pkg/zefs"
)

// grRestartFactKeyPrefix is the zefs blob key prefix for a GR restart-fact. The full key
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

// grBlobStore is the minimal read/write seam over the zefs blob store, so the NVS functions
// are unit-testable against an in-memory fake (it matches the bootCountStore shape).
type grBlobStore interface {
	ReadFile(name string) ([]byte, error)
	WriteFile(name string, data []byte, perm fs.FileMode) error
}

// openGRStore opens the shared OSPF zefs blob store for a GR read/write. The caller owns the
// returned store and MUST Close it. It mirrors openBootCountStore but returns the raw store so
// the GR path can perform multiple operations (write on prepare, read on resume, clear on
// exit) rather than the single-write closing wrapper the boot count uses.
func openGRStore() (*zefs.BlobStore, bool) {
	// pinnedStateDir, not paths.DefaultConfigDir: unpinned, that resolves the
	// binary-relative etc/ze every `ze` on the host shares. See its doc.
	dir := pinnedStateDir(paths.DefaultConfigDir)
	if dir == "" {
		return nil, false
	}
	store, err := zefs.Open(filepath.Join(dir, "database.zefs"))
	if err != nil {
		return nil, false
	}
	return store, true
}

// writeRestartFact persists the restart fact under key. Non-wire JSON; the blob is small and
// written only on a prepare/exit transition, never per packet.
func writeRestartFact(store grBlobStore, key string, f restartFact) error {
	data, err := json.Marshal(f)
	if err != nil {
		return err
	}
	return store.WriteFile(key, data, 0)
}

// readRestartFact reads the restart fact under key. The bool is false when no fact is stored
// (a normal cold boot) or the blob is unreadable/corrupt (treated as no fact: boot normally,
// per the R-10 / preservation-integrity safety rule).
func readRestartFact(store grBlobStore, key string) (restartFact, bool) {
	data, err := store.ReadFile(key)
	if err != nil || len(data) == 0 {
		return restartFact{}, false
	}
	var f restartFact
	if err := json.Unmarshal(data, &f); err != nil {
		return restartFact{}, false
	}
	return f, true
}

// clearRestartFact records that no restart is in flight (written on GR exit). It overwrites
// rather than deletes so a later resume reads an explicit not-restarting fact.
func clearRestartFact(store grBlobStore, key string) error {
	return writeRestartFact(store, key, restartFact{Restarting: false})
}
