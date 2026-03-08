//go:build ignore

// Script to add // Design: comments to all eligible Go source files.
// Run: go run scripts/add-design-refs.go
// One-time use — delete after running.
package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// nolintPfx avoids triggering the nolint-abuse hook on string content.
const nolintPfx = "/" + "/nolint"

var dirMapping = map[string][]string{
	"cmd/ze/bgp":                                    {"// Design: docs/architecture/core-design.md — BGP CLI commands"},
	"cmd/ze/cli":                                    {"// Design: docs/architecture/core-design.md — interactive CLI"},
	"cmd/ze/config":                                 {"// Design: docs/architecture/config/syntax.md — config CLI commands"},
	"cmd/ze/exabgp":                                 {"// Design: docs/architecture/core-design.md — external format bridge CLI"},
	"cmd/ze/hub":                                    {"// Design: docs/architecture/hub-architecture.md — hub CLI entry point"},
	"cmd/ze/plugin":                                 {"// Design: docs/architecture/api/process-protocol.md — plugin CLI dispatch"},
	"cmd/ze/schema":                                 {"// Design: docs/architecture/config/yang-config-design.md — schema CLI"},
	"cmd/ze/signal":                                 {"// Design: docs/architecture/behavior/signals.md — signal handling CLI"},
	"cmd/ze/validate":                               {"// Design: docs/architecture/config/yang-config-design.md — validation CLI"},
	"cmd/ze":                                        {"// Design: docs/architecture/system-architecture.md — ze main entry point"},
	"cmd/ze-test":                                   {"// Design: docs/architecture/testing/ci-format.md — test runner CLI"},
	"internal/chaos/engine":                         {"// Design: docs/architecture/chaos-web-dashboard.md — chaos action scheduling"},
	"internal/chaos/guard":                          {"// Design: docs/architecture/chaos-web-dashboard.md — chaos action compatibility guard"},
	"internal/chaos/inprocess":                      {"// Design: docs/architecture/chaos-web-dashboard.md — in-process chaos runner"},
	"internal/chaos/mocknet":                        {"// Design: docs/architecture/chaos-web-dashboard.md — mock network for in-process chaos"},
	"internal/chaos/peer":                           {"// Design: docs/architecture/chaos-web-dashboard.md — BGP peer simulation"},
	"internal/chaos/replay":                         {"// Design: docs/architecture/chaos-web-dashboard.md — event replay and diff"},
	"internal/chaos/report":                         {"// Design: docs/architecture/chaos-web-dashboard.md — chaos reporting and metrics"},
	"internal/chaos/route":                          {"// Design: docs/architecture/chaos-web-dashboard.md — route action scheduling"},
	"internal/chaos/scenario":                       {"// Design: docs/architecture/chaos-web-dashboard.md — scenario generation"},
	"internal/chaos/shrink":                         {"// Design: docs/architecture/chaos-web-dashboard.md — test case shrinking"},
	"internal/chaos/validation":                     {"// Design: docs/architecture/chaos-web-dashboard.md — property-based validation"},
	"internal/chaos/web":                            {"// Design: docs/architecture/chaos-web-dashboard.md — web dashboard UI"},
	"cmd/ze-chaos":                                  {"// Design: docs/architecture/chaos-web-dashboard.md — chaos test orchestrator"},
	"internal/component/bgp/config":                 {"// Design: docs/architecture/config/syntax.md — BGP config extraction and loading"},
	"internal/component/config/editor/testing":      {"// Design: docs/architecture/config/yang-config-design.md — editor test infrastructure"},
	"internal/component/config/editor":              {"// Design: docs/architecture/config/yang-config-design.md — config editor"},
	"internal/component/config/migration":           {"// Design: docs/architecture/config/syntax.md — config migration"},
	"internal/component/config":                     {"// Design: docs/architecture/config/syntax.md — config parsing and loading"},
	"internal/component/config/env":                 {"// Design: docs/architecture/config/environment.md — environment variable handling"},
	"internal/exabgp/bridge":                        {"// Design: docs/architecture/core-design.md — ExaBGP plugin bridge"},
	"internal/exabgp/migration":                     {"// Design: docs/architecture/core-design.md — ExaBGP config migration"},
	"internal/core/hub":                             {"// Design: docs/architecture/hub-architecture.md — hub coordination"},
	"internal/core/ipc":                             {"// Design: docs/architecture/api/ipc_protocol.md — IPC framing and dispatch"},
	"internal/parse":                                {"// Design: docs/architecture/config/syntax.md — parsing helpers"},
	"internal/pidfile":                              {"// Design: docs/architecture/system-architecture.md — PID file management"},
	"internal/plugin/all":                           {"// Design: docs/architecture/api/architecture.md — plugin auto-registration"},
	"internal/plugin/cli":                           {"// Design: docs/architecture/cli/plugin-modes.md — plugin CLI framework"},
	"internal/plugin/registry":                      {"// Design: docs/architecture/api/architecture.md — plugin registry"},
	"internal/plugin":                               {"// Design: docs/architecture/api/process-protocol.md — plugin process management"},
	"internal/plugins/bgp/attribute":                {"// Design: docs/architecture/wire/attributes.md — path attribute encoding"},
	"internal/plugins/bgp/capability":               {"// Design: docs/architecture/wire/capabilities.md — capability negotiation"},
	"internal/plugins/bgp/commit":                   {"// Design: docs/architecture/update-building.md — commit management"},
	"internal/plugins/bgp/context":                  {"// Design: docs/architecture/encoding-context.md — encoding context"},
	"internal/plugins/bgp/filter":                   {"// Design: docs/architecture/core-design.md — route filtering"},
	"internal/plugins/bgp/format":                   {"// Design: docs/architecture/api/json-format.md — message formatting"},
	"internal/plugins/bgp/fsm":                      {"// Design: docs/architecture/behavior/fsm.md — BGP finite state machine"},
	"internal/component/bgp/plugins/bgp-cmd-ops":    {"// Design: docs/architecture/api/commands.md — BGP operational command handlers"},
	"internal/component/bgp/plugins/bgp-cmd-peer":   {"// Design: docs/architecture/api/commands.md — BGP peer command handlers"},
	"internal/component/bgp/plugins/bgp-cmd-update": {"// Design: docs/architecture/api/commands.md — BGP update command handlers"},
	"internal/plugins/bgp/message":                  {"// Design: docs/architecture/wire/messages.md — BGP message types"},
	"internal/plugins/bgp/nlri":                     {"// Design: docs/architecture/wire/nlri.md — NLRI encoding and decoding"},
	"internal/plugins/bgp/reactor":                  {"// Design: docs/architecture/core-design.md — BGP reactor event loop"},
	"internal/plugins/bgp/rib":                      {"// Design: docs/architecture/pool-architecture.md — RIB wire storage"},
	"internal/plugins/bgp/route":                    {"// Design: docs/architecture/route-types.md — route definitions"},
	"internal/plugins/bgp/server":                   {"// Design: docs/architecture/core-design.md — BGP server events and hooks"},
	"internal/plugins/bgp/types":                    {"// Design: docs/architecture/core-design.md — shared BGP types"},
	"internal/plugins/bgp/wire":                     {"// Design: docs/architecture/wire/buffer-writer.md — wire buffer utilities"},
	"internal/plugins/bgp/wireu":                    {"// Design: docs/architecture/wire/messages.md — wire UPDATE lazy parsing"},
	"internal/plugins/bgp-gr": {
		"// Design: docs/architecture/core-design.md — graceful restart plugin",
		"// RFC: rfc/short/rfc4724.md",
	},
	"internal/plugins/bgp-hostname": {"// Design: docs/architecture/core-design.md — hostname capability plugin"},
	"internal/plugins/bgp-llnh": {
		"// Design: docs/architecture/core-design.md — link-local next-hop plugin",
		"// RFC: rfc/short/rfc5549.md",
	},
	"internal/plugins/bgp-rib/storage": {"// Design: docs/architecture/plugin/rib-storage-design.md — RIB storage internals"},
	"internal/plugins/bgp-rib":         {"// Design: docs/architecture/plugin/rib-storage-design.md — RIB plugin"},
	"internal/plugins/bgp-role": {
		"// Design: docs/architecture/core-design.md — BGP role plugin",
		"// RFC: rfc/short/rfc9234.md",
	},
	"internal/plugins/bgp-rs": {"// Design: docs/architecture/core-design.md — route server plugin"},
	"internal/plugins/bgp-nlri-evpn": {
		"// Design: docs/architecture/wire/nlri-evpn.md — EVPN NLRI plugin",
		"// RFC: rfc/short/rfc7432.md",
	},
	"internal/plugins/bgp-nlri-flowspec": {
		"// Design: docs/architecture/wire/nlri-flowspec.md — FlowSpec NLRI plugin",
		"// RFC: rfc/short/rfc5575.md",
	},
	"internal/plugins/bgp-nlri-labeled": {
		"// Design: docs/architecture/wire/nlri.md — labeled unicast NLRI plugin",
		"// RFC: rfc/short/rfc8277.md",
	},
	"internal/plugins/bgp-nlri-ls": {
		"// Design: docs/architecture/wire/nlri-bgpls.md — BGP-LS NLRI plugin",
		"// RFC: rfc/short/rfc7752.md",
	},
	"internal/plugins/bgp-nlri-mup": {
		"// Design: docs/architecture/wire/nlri.md — MUP NLRI plugin",
		"// RFC: rfc/short/draft-ietf-bess-mup-safi.md",
	},
	"internal/plugins/bgp-nlri-mvpn": {"// Design: docs/architecture/wire/nlri.md — MVPN NLRI plugin"},
	"internal/plugins/bgp-nlri-rtc": {
		"// Design: docs/architecture/wire/nlri.md — route target constraint plugin",
		"// RFC: rfc/short/rfc4684.md",
	},
	"internal/plugins/bgp-nlri-vpls": {
		"// Design: docs/architecture/wire/nlri.md — VPLS NLRI plugin",
		"// RFC: rfc/short/rfc4761.md",
	},
	"internal/plugins/bgp-nlri-vpn": {
		"// Design: docs/architecture/wire/nlri.md — VPN NLRI plugin",
		"// RFC: rfc/short/rfc4364.md",
	},
	"internal/plugins/bgp-rib/pool":   {"// Design: docs/architecture/pool-architecture.md — per-attribute pool instances"},
	"internal/component/bgp/attrpool": {"// Design: docs/architecture/pool-architecture.md — attribute and NLRI pools"},
	"internal/selector":               {"// Design: docs/architecture/core-design.md — peer selector"},
	"internal/sim":                    {"// Design: docs/architecture/chaos-web-dashboard.md — simulation infrastructure"},
	"internal/slogutil":               {"// Design: docs/architecture/config/environment.md — structured logging utilities"},
	"internal/source":                 {"// Design: docs/architecture/core-design.md — source registry"},
	"internal/component/bgp/store":    {"// Design: docs/architecture/pool-architecture.md — attribute and NLRI storage"},
	"internal/test/ci":                {"// Design: docs/architecture/testing/ci-format.md — CI test format parsing"},
	"internal/test/decode":            {"// Design: docs/architecture/testing/ci-format.md — decode test helpers"},
	"internal/test/peer":              {"// Design: docs/architecture/testing/ci-format.md — test BGP peer"},
	"internal/test/runner":            {"// Design: docs/architecture/testing/ci-format.md — test runner framework"},
	"internal/core/syslog":            {"// Design: docs/architecture/testing/ci-format.md — syslog test helpers"},
	"internal/test":                   {"// Design: docs/architecture/testing/ci-format.md — test infrastructure"},
	"internal/test/tmpfs":             {"// Design: docs/architecture/system-architecture.md — temporary filesystem management"},
	"internal/component/config/yang":  {"// Design: docs/architecture/config/yang-config-design.md — YANG schema handling"},
	"pkg/plugin/rpc":                  {"// Design: docs/architecture/api/ipc_protocol.md — plugin RPC types"},
	"pkg/plugin/sdk":                  {"// Design: docs/architecture/api/process-protocol.md — plugin SDK"},
	"pkg/plugin":                      {"// Design: docs/architecture/api/process-protocol.md — plugin package"},
	"research":                        {"// Design: (none — research tool)"},
	"scripts":                         {"// Design: (none — build tool)"},
}

func isExempt(base string) bool {
	return strings.HasSuffix(base, "_test.go") ||
		strings.HasSuffix(base, "_gen.go") ||
		base == "register.go" ||
		base == "embed.go" ||
		base == "doc.go"
}

func isGenerated(content string) bool {
	check := content
	if len(check) > 500 {
		check = check[:500]
	}
	return strings.Contains(check, "Code generated") || strings.Contains(check, "DO NOT EDIT")
}

func findDesign(rel string) []string {
	dir := filepath.Dir(rel)
	keys := make([]string, 0, len(dirMapping))
	for k := range dirMapping {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		return len(keys[i]) > len(keys[j])
	})
	for _, k := range keys {
		if dir == k || strings.HasPrefix(dir, k+"/") {
			return dirMapping[k]
		}
	}
	return nil
}

func processFile(path string, designLines []string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	lines := strings.Split(string(content), "\n")

	// Find insertion point (skip //go:build lines and following blank)
	insertAt := 0
	for insertAt < len(lines) {
		trimmed := strings.TrimSpace(lines[insertAt])
		if strings.HasPrefix(trimmed, "//go:build") {
			insertAt++
			continue
		}
		break
	}
	if insertAt > 0 && insertAt < len(lines) && strings.TrimSpace(lines[insertAt]) == "" {
		insertAt++
	}

	// Build insertion block
	toInsert := make([]string, len(designLines))
	copy(toInsert, designLines)

	// Determine separator based on what follows
	if insertAt < len(lines) {
		nextLine := lines[insertAt]
		if strings.HasPrefix(nextLine, "// Package ") ||
			(strings.HasPrefix(nextLine, "// ") &&
				!strings.HasPrefix(nextLine, nolintPfx) &&
				!strings.HasPrefix(nextLine, "//go:")) {
			toInsert = append(toInsert, "//")
		} else if strings.TrimSpace(nextLine) != "" {
			toInsert = append(toInsert, "")
		}
	}

	// Reconstruct file
	result := make([]string, 0, len(lines)+len(toInsert))
	result = append(result, lines[:insertAt]...)
	result = append(result, toInsert...)
	result = append(result, lines[insertAt:]...)

	return os.WriteFile(path, []byte(strings.Join(result, "\n")), 0o644)
}

func main() {
	root := "."
	if len(os.Args) > 1 {
		root = os.Args[1]
	}
	var processed, skipped, unmapped int

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			base := filepath.Base(path)
			if base == ".git" || base == "vendor" || base == "node_modules" || base == ".claude" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		base := filepath.Base(path)
		if isExempt(base) {
			skipped++
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		content, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		contentStr := string(content)
		if isGenerated(contentStr) {
			skipped++
			return nil
		}
		if strings.Contains(contentStr, "// Design:") {
			skipped++
			return nil
		}
		designLines := findDesign(rel)
		if designLines == nil {
			log.Printf("UNMAPPED: %s", rel)
			unmapped++
			return nil
		}
		if err := processFile(path, designLines); err != nil {
			log.Printf("ERROR: %s: %v", rel, err)
			return nil
		}
		processed++
		fmt.Printf("OK: %s\n", rel)
		return nil
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("\nDone: %d processed, %d skipped, %d unmapped\n", processed, skipped, unmapped)
}
