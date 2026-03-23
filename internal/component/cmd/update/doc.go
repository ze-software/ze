// Design: docs/architecture/api/commands.md — update verb command handlers
//
// Package update provides the top-level "update" CLI verb for refreshing
// stale data from external sources (PeeringDB, RPKI, IRR, etc.).
//
// Detail: update.go — RPC registration for update verb handlers
package update

import (
	_ "codeberg.org/thomas-mangin/ze/internal/component/cmd/update/schema" // init() registers YANG module
)
