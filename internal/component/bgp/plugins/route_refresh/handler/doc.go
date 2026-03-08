// Design: docs/architecture/api/commands.md — BGP route refresh command handlers
// Overview: ../register.go — bgp-route-refresh SDK plugin registration
//
// Package handler provides route refresh command handlers for the plugin server.
// These handlers send ROUTE-REFRESH messages (RFC 2918) and enhanced markers (RFC 7313).
//
// Each handler self-registers via init() + pluginserver.RegisterRPCs().
package handler

import (
	_ "codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/route_refresh/schema" // init() registers YANG module
)
