# Plugin Design Rationale

Why: `.claude/rules/plugin-design.md`

## Why Registry Is a Leaf Package

The registry has zero plugin dependencies. Plugin packages import the registry (not vice versa). The `all` package uses blank imports to trigger `init()` registration. This prevents import cycles and allows plugins to be added without modifying infrastructure.

## Why Infrastructure Must Not Import Plugins

Registry exists as an indirection layer. Direct imports create coupling and import cycles. Known violations still exist in: `cmd/ze/bgp/encode.go`, `reactor/reactor.go`, `message/update_build.go`, `config/loader.go` — to be fixed via `registry.EncodeNLRIByFamily(family, args)`.

## SDK Callbacks Available

| Callback | Purpose | Used by |
|----------|---------|---------|
| `OnEvent` | Receive BGP events | RIB, RR, GR, Hostname |
| `OnConfigure` | Receive config sections | GR, Hostname |
| `OnDecodeNLRI` | Decode NLRI hex for family | EVPN, VPN, BGP-LS, FlowSpec |
| `OnEncodeNLRI` | Encode NLRI from args | FlowSpec |
| `OnDecodeCapability` | Decode capability hex | Hostname |
| `OnExecuteCommand` | Handle API commands | RIB, RR |
| `OnShareRegistry` | Receive registry info | (general) |
| `OnBye` | Shutdown notification | (general) |
| `OnStarted` | Post-startup hook | (general) |

## SDK Engine Calls (plugin → engine)

| Method | Purpose |
|--------|---------|
| `p.UpdateRoute(ctx, peer, command)` | Send route update/forward/withdraw |
| `p.DecodeNLRI(ctx, family, hex)` | Decode NLRI via registry |
| `p.EncodeNLRI(ctx, family, args)` | Encode NLRI via registry |
| `p.DecodeMPReach(ctx, hex, addPath)` | Decode MP_REACH_NLRI |
| `p.DecodeMPUnreach(ctx, hex, addPath)` | Decode MP_UNREACH_NLRI |
| `p.DecodeUpdate(ctx, hex, addPath)` | Decode full UPDATE body |
| `p.SubscribeEvents(ctx, events, families, peer)` | Subscribe to events |
| `p.UnsubscribeEvents(ctx)` | Clear subscriptions |

## Standard Plugin Flags

`RunPlugin()` provides: `--log-level`, `--yang`, `--features`, `--decode`, `--nlri`, `--capa`, `--text`.

## Decode Protocol (CLI mode)

stdin: `decode capability <code> <hex>` or `decode nlri <family> <hex>`
stdout: `decoded json <json>` or `decoded unknown`
Separate from YANG RPC protocol — CLI `--decode` flag only.

## Auto-Populated Maps (from registry)

CLI dispatch, plugin runners, YANG schemas, config roots, family→plugin, capability→plugin, in-process decoders, NLRI decoder/encoder by family — all populated from registry at init time.

## Logger Pattern

```go
var logger = slogutil.DiscardLogger()
func SetLogger(l *slog.Logger) { if l != nil { logger = l } }
```

## Entry Point Pattern

`sdk.NewWithConn()` → register callbacks → `p.Run(ctx, Registration{...})` → 5-stage protocol + event loop.
