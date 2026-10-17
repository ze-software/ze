// Design: docs/architecture/diagnostics/path-mtu.md -- the reference address defaults to the first name server
// Related: system.go -- ExtractSystemConfig, the reader of the name-server leaf-list

package system

import (
	"slices"
	"sync/atomic"
)

// publishedNameServers holds the system { name-server } leaf-list of the
// config the daemon runs. The hub publishes it at start and again on every
// accepted reload, so a reader that has no config tree of its own (show mtu, which
// defaults its reference address to the first name server) sees the value
// the running config holds. A nil pointer means nothing was published yet,
// which is a different answer from a config that names no server.
//
//nolint:gochecknoglobals // Process-wide publication of running config, written by the hub.
var publishedNameServers atomic.Pointer[[]string]

// PublishNameServers records the name servers of the running config. The hub
// MUST call it after every load and every accepted reload, including one whose
// config names no server, so a removed leaf-list does not leave the old servers
// published. The slice is copied, so the caller keeps ownership. Safe for
// concurrent use.
func PublishNameServers(servers []string) {
	published := slices.Clone(servers)
	publishedNameServers.Store(&published)
}

// NameServers answers the name servers PublishNameServers last recorded, in
// config order, and true. It answers false when nothing was published yet,
// so a caller can tell "the config names no server" from "no config was
// loaded". The slice is a copy the caller may keep. Safe for concurrent use.
func NameServers() ([]string, bool) {
	published := publishedNameServers.Load()
	if published == nil {
		return nil, false
	}
	return slices.Clone(*published), true
}
