# Canonical Examples

Reference implementations for common patterns. Copy these when creating new instances.

## Plugin Patterns

| Pattern | Reference | Notes |
|---------|-----------|-------|
| NLRI plugin (full) | `internal/component/bgp/plugins/nlri/evpn/register.go` | Families, decoder, encoder, CLI handler |
| NLRI plugin (minimal) | `internal/component/bgp/plugins/nlri/vpn/register.go` | Simpler variant, two families |
| RPC registration | `internal/component/cmd/show/show.go` | `pluginserver.RegisterRPCs()` with handler list |
| Plugin with YANG | `internal/component/bgp/plugins/rib/` | Schema subdir, config roots, full lifecycle |
| Plugin with events | `internal/component/bgp/plugins/rib/` | `OnStructuredEvent` for DirectBridge |

## CLI Patterns

| Pattern | Reference | Notes |
|---------|-----------|-------|
| Simple subcommand | `cmd/ze/data/main.go` | Flag parsing, handler map, error suggestions |
| RPC-backed command | `cmd/ze/show/main.go` | Delegates to shared CLI command tree |
| Interactive TUI | `cmd/ze/cli/main.go` | Bubbletea, completion, RPC tree |
| Top-level dispatch | `cmd/ze/main.go` | Switch on arg, global flags, help |

## Config Patterns

| Pattern | Reference | Notes |
|---------|-----------|-------|
| Config parser | `internal/component/config/parser.go` | Tokenizer + YANG schema validation |
| Peer config | `internal/component/config/peers.go` | `PeersFromTree()` -- tree to typed config |
| YANG schema (config) | `internal/component/bgp/schema/ze-bgp-conf.yang` | Container/leaf/list for config |
| YANG schema (API) | `internal/component/bgp/schema/ze-bgp-api.yang` | RPC definitions for commands |

## Wire Patterns

| Pattern | Reference | Notes |
|---------|-----------|-------|
| Buffer-first encoding | `internal/component/bgp/reactor/reactor_wire.go` | Skip-and-backfill, pool buffers |
| Attribute WriteTo | `internal/component/bgp/message/attr/origin.go` | Simple `WriteTo(buf, off) int` |
| NLRI iterator | `internal/component/bgp/message/update/nlri.go` | Zero-copy offset iteration |

## Test Patterns

| Pattern | Reference | Notes |
|---------|-----------|-------|
| Functional test (simple) | `test/decode/flowspec-plugin.ci` | stdin, cmd, JSON expect |
| Functional test (config) | `test/parse/` (any file) | Config file + exit code |
| Functional test (plugin) | `test/plugin/` (any file) | Full peer interaction |
| Editor test | `test/editor/navigation/edit-single.et` | Headless TUI, input/expect |
| Unit test | `internal/component/bgp/message/attr/origin_test.go` | Table-driven, race detector |

## Registration Patterns

| Pattern | Reference | Notes |
|---------|-----------|-------|
| Plugin init() | `internal/component/bgp/plugins/nlri/evpn/register.go` | `registry.Register()` in init |
| All-imports | `internal/component/plugin/all/all.go` | Blank imports triggering init |
| CLI handler in plugin | `internal/component/bgp/plugins/nlri/evpn/register.go` | `cli.BaseConfig` + `cli.RunPlugin` |

## Scripts/Tooling

| Pattern | Reference | Notes |
|---------|-----------|-------|
| Inventory script | `scripts/inventory/inventory.go` | `//go:build ignore`, registry import, markdown+JSON |
| Code generator | `scripts/codegen/plugin_imports.go` | Walk dirs, generate Go source |
