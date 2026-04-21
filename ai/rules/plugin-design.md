# Plugin Design

**BLOCKING:** All plugins MUST follow these patterns.
Rationale: `ai/rationale/plugin-design.md`
Structural template: `ai/patterns/plugin.md`

## Architecture

| Layer | Location | Purpose |
|-------|----------|---------|
| Registry | `internal/component/plugin/registry/` | Central registry (leaf package, no plugin deps) |
| Family registry | `internal/core/family/` | Cross-component address-family registration (`Family`/`AFI`/`SAFI` types + `family.MustRegister`) |
| Public SDK | `pkg/plugin/sdk/` | Callback abstraction for external plugins |
| RPC Types | `pkg/plugin/rpc/` | Shared YANG RPC types + `MuxConn` for concurrent RPCs |
| Internal | `internal/component/bgp/plugins/<name>/` | Plugin implementations + `register.go` |
| All imports | `internal/component/plugin/all/` | Blank imports triggering all `init()` |
| CLI shared | `internal/component/plugin/cli/` | `PluginConfig` + `RunPlugin()` |

## SDK Is Generic (BLOCKING)

**The SDK (`pkg/plugin/sdk/`) must never contain plugin-specific code.** Adding or removing
a callback type requires only one `On*` method in `sdk_callbacks.go` that registers a
handler in the callback map. The event loops, dispatch logic, and transport layers are
callback-agnostic -- they dispatch through `map[string]callbackHandler` without knowing
what callbacks exist.

| Rule | Meaning |
|------|---------|
| No switch/case on method names in event loops | Dispatch is map lookup, not enumeration |
| No transport-specific handler methods | One handler per callback, used by both pipe and bridge |
| Bye is the only special case | It terminates the loop -- checked by method name, not by handler signature |
| Adding a callback = one On* method | Zero changes to sdk_dispatch.go or event loop code |

## Proximity Principle (BLOCKING)

**Related code belongs together.** The "delete the folder" test is a mechanical check for proximity.

| Rule | Meaning |
|------|---------|
| All code for a concern in its folder | Commands, handlers, registration, logic — not scattered across packages |
| No external references to internals | Infrastructure, reactor, other units never import a specific plugin/command module |
| Blank import is the only coupling | A single `_ "package"` triggers init(); removing it cleanly disables the unit |
| Engine core works without any command module | Reactor, FSM, wire layer must function without CLI command handlers |

## YANG Is Required (BLOCKING)

**All RPCs need YANG registration for the CLI.** Any command handler without a YANG schema is a structural issue to fix, not a different category. There is no "command module" — everything with RPCs is a plugin and lives under `plugins/<name>/`.

| Registration | YANG | Location |
|-------------|------|----------|
| `registry.Register()` (SDK) | Required | `plugins/<name>/` |
| `pluginserver.RegisterRPCs()` (engine-side) | Required | `plugins/<name>/` |

**Anti-pattern:** Placing command handlers in reactor/ (couples engine core to command surface), in a separate handler/ package (middleman), or in a `command/` folder (formalizes missing YANG as acceptable). Commands belong in `plugins/` with YANG schemas.

## Import Rules (BLOCKING)

Infrastructure MUST NOT import plugin implementations directly -- use registry lookups.
Plugins MUST NOT import sibling plugin packages -- use text commands via DispatchCommand.

- `internal/component/plugin/`, `internal/component/bgp/`, `internal/component/config/`, `cmd/ze/` -> registry
- NLRI decoding: `registry.NLRIDecoder(family)` -> `func(hex) (json, error)`
- NLRI encoding: `registry.NLRIEncoder(family)` -> `func(args) (hex, error)`
- Plugin `register.go` and `internal/component/plugin/all/all.go` blank imports: allowed
- Schema imports (`<plugin>/schema/`): allowed (data, not logic)
- Test imports: tolerated

## Plugin Boundary Naming (BLOCKING)

When a plugin sends commands to the engine via DispatchCommand, name the helper
for what it does (dispatch a command), not where it sends it (to a specific plugin).
The engine routes commands by prefix -- the caller should not encode the destination
in function names, variable names, or type names.

| Banned | Use Instead |
|--------|-------------|
| `dispatchRIBCommand` | `dispatchCommand` |
| `sendToRIB` | `dispatchCommand` |
| `ribClient` | `sdk.DispatchCommand` directly |

## 5-Stage Protocol

| Stage | Direction | RPC |
|-------|-----------|-----|
| 1. Declaration | Plugin→Engine (A) | `ze-plugin-engine:declare-registration` |
| 2. Config | Engine→Plugin (B) | `ze-plugin-callback:configure` |
| 3. Capability | Plugin→Engine (A) | `ze-plugin-engine:declare-capabilities` |
| 4. Registry | Engine→Plugin (B) | `ze-plugin-callback:share-registry` |
| 5. Ready | Plugin→Engine (A) | `ze-plugin-engine:ready`, enter event loop |
| Post | Engine→Plugin (B) | `ze-plugin-callback:post-startup` — sent once after every startup phase completes and both the plugin registry and dispatcher command registry are frozen |

After Stage 5: SDK wraps Socket A in `MuxConn` for concurrent RPCs. Engine dispatches Socket A requests in goroutines. Wire format: `#<id> <verb> [<json>]\n` (see `docs/architecture/api/wire-format.md`).

## OnStarted vs OnAllPluginsReady (BLOCKING)

Stages 1-5 run per-phase. The engine loads plugins across up to five phases
(config-path auto-load → explicit → family → event-type → send-type) serially,
so a plugin's `OnStarted` fires after its own handshake but potentially before
plugins in later phases are loaded.

| Where to put it | Rule |
|-----------------|------|
| `OnStarted(fn)` | Local setup only: start long-lived goroutines, register subscriptions, initialise per-plugin state. |
| `OnAllPluginsReady(fn)` | Any `DispatchCommand` targeting another plugin's command at startup. The callback fires via the event loop once the dispatcher command registry is frozen, so cross-plugin dispatch is guaranteed to resolve. |

`bgp-rpki` is the reference example: the `adj-rib-in enable-validation` dispatch lives in `OnAllPluginsReady` (`internal/component/bgp/plugins/rpki/rpki.go`). Putting it in `OnStarted` used to fail with "unknown command" whenever `bgp-adj-rib-in` loaded in Phase 2 while bgp-rpki auto-loaded in Phase 1.

## Registration Fields

| Field | Type | Required | Purpose |
|-------|------|----------|---------|
| `Name` | string | Yes | Plugin name |
| `Description` | string | Yes | Human-readable description |
| `RunEngine` | func(Conn, Conn) int | Yes | Engine-mode entry point |
| `CLIHandler` | func([]string) int | Yes | CLI dispatch handler |
| `Families` | []string | No | Address families ("afi/safi") |
| `CapabilityCodes` | []uint8 | No | Capability codes decoded |
| `ConfigRoots` | []string | No | Config roots plugin wants |
| `Dependencies` | []string | No | Plugin names that MUST also be loaded. Missing name → `ErrMissingDependency`. |
| `OptionalDependencies` | []string | No | Plugin names the owner uses when loaded but can run without. Missing name is silently skipped (no error). Owner must handle runtime absence gracefully. |
| `YANG` | string | No | YANG schema content |
| `InProcessNLRIDecoder` | func | No | NLRI decode |
| `InProcessNLRIEncoder` | func | No | NLRI encode |
| `EventTypes` | []string | No | Event types this plugin produces (registered at startup) |
| `SendTypes` | []string | No | Send types this plugin enables (e.g., ["enhanced-refresh"]). Registered dynamically at startup. |
| `Features` | string | No | Space-separated flags ("nlri yang capa") |

## Optional Dependencies

Plugins that USE another plugin when it is present but can run without it
declare the relationship as `OptionalDependencies` rather than `Dependencies`.
Example: `bgp-rs` uses `bgp-adj-rib-in` for replay-on-peer-up when present,
and disables replay (with a one-shot WARN log) when it is not.

| Field | Semantics |
|-------|-----------|
| `Dependencies` | Hard. Resolver returns `ErrMissingDependency` when the named plugin is not registered. Startup fails. |
| `OptionalDependencies` | Soft. Resolver pulls the plugin in if registered, silently skips if not. No error. Owner MUST detect absence at run time and fall back cleanly. |

Validation at registration is the same for both fields: empty string or
self-dependency is rejected.

Cycle detection and `TopologicalTiers` walk both kinds of edges when BOTH
endpoints appear in the resolved name set, so startup ordering is preserved
whenever the optional dep is actually present.

Graceful fallback is the owner's responsibility. The pattern used by `bgp-rs`:

1. Dispatch the command targeting the optional dep normally.
2. If the response returns the engine's `ErrUnknownCommand` (propagated as a
   string across the plugin IPC boundary), treat it as the "plugin absent"
   signal.
3. Use `sync.Once` to log one `WARN` per process lifetime; skip the feature
   (e.g. replay convergence loop) and continue with the rest of the flow.

## Family Registration (BLOCKING)

NLRI plugins MUST register the address families they handle via
`family.MustRegister(afi, safi, afiStr, safiStr)` at package init time. The four
RFC 4760 base families (`IPv4Unicast`, `IPv6Unicast`, `IPv4Multicast`,
`IPv6Multicast`) live in `internal/core/family/registry.go` itself; everything
else is owned by the plugin's `types.go`.

| Rule | Detail |
|------|--------|
| One canonical name per family | No aliases. The `afiStr/safiStr` arguments form the canonical `<afi>/<safi>` string. |
| Registration is fatal on conflict | `family.MustRegister` panics if AFI or SAFI numbers collide with a different name. Same name + same numbers is a no-op. |
| Plugin owns the SAFI name | `vpn` plugin chose `mpls-vpn`; `flowspec` plugin chose `flow`. The plugin is the authority. |
| External plugins use the protocol | Forked plugins declare families in `declare-registration` (Stage 1) with AFI/SAFI numbers; the engine forwards to `family.RegisterFamily` via `registerPluginFamilies` in `plugin/server/startup.go`. See `docs/architecture/api/process-protocol.md`. |
| Test packages call `family.RegisterTestFamilies()` | If a test exercises a SAFI not registered by an internal plugin, register it via the helper in `internal/core/family/testfamilies.go`. |

## Runtime Filter Declaration (planned -- stage 1 wire protocol)

External plugins can declare named route filters at stage 1 via `declare-registration`.
This is runtime IPC, not compile-time registration. Filter fields are stored in
`PluginRegistration` (`internal/component/plugin/registration.go`), not `Registration`.

| Field | Type | Purpose |
|-------|------|---------|
| `filters[].name` | string | Filter name (config references as `<plugin>:<name>`) |
| `filters[].direction` | enum | import, export, both |
| `filters[].attributes` | []string | Attribute names to receive |
| `filters[].raw` | bool | Include raw wire bytes; REQUIRED for non-CIDR families |
| `filters[].on-error` | enum | reject (fail-closed) or accept (fail-open) |
| `filters[].overrides` | []string | Default filters this filter replaces |

See `plan/learned/479-redistribution-filter.md` for the full design.

### Non-CIDR Families (BLOCKING for filter plugin authors)

The engine's text-mode filter protocol inlines NLRI prefixes only for the
"CIDR-family" set: IPv4/IPv6 unicast, multicast, and mpls-label. For every
other family -- EVPN, Flowspec, VPN, BGP-LS, MVPN, MUP, RTC, and any
future non-CIDR family -- the text protocol emits a marker-only block of
the form `nlri <family> <op>` with no prefixes. A filter plugin that
needs per-NLRI decisions on a non-CIDR family MUST declare `raw=true`
and parse `FilterUpdateInput.Raw` itself.

| Family set | Text protocol emits | Filter plugin requirement |
|------------|---------------------|--------------------------|
| CIDR (ipv4/ipv6 unicast, multicast, mpls-label) | `nlri <family> <op> <prefix>...` | `raw=false` is sufficient |
| Non-CIDR (EVPN, Flowspec, VPN, BGP-LS, MVPN, MUP, RTC, ...) | `nlri <family> <op>` (marker only) | `raw=true` REQUIRED for per-NLRI decisions |

See `docs/architecture/api/process-protocol.md` "Non-CIDR Families in the
Filter Text Protocol" for the full contract and
`internal/component/bgp/reactor/filter_format.go` (`isCIDRFamily`,
`formatMPBlock`) for the implementation.

## Renaming a Registered Name (BLOCKING)

A plugin or subsystem name is not a single string. It appears in many places
that all have to agree, and most of them are loose strings (config keys, log
keys, dispatch keys, env vars) that no compiler will catch.

The `938df51d` fix exists because BGP-as-plugin Phase 2 registered subsystems
as `bgp-gr` / `bgp-rib` etc., but config and `ze.log.*` env vars expected
`bgp.gr` / `bgp.rib`. Log levels were silently never applied. Six days passed
before review caught it.

**Before changing any registered name (plugin name, subsystem name, log
subsystem, dispatch key, command prefix, family canonical name), grep for
EVERY consumer in the table below.** A diff that updates only one of these
locations is incomplete by definition.

| Consumer | Where to grep | Looks like |
|----------|--------------|-----------|
| Plugin registration | `internal/component/bgp/plugins/*/register.go`, `internal/component/plugin/all/all.go` | `Name: "bgp-gr"` |
| Subsystem logger | `internal/core/slogutil/`, `slogutil.Logger("...")` calls | `slogutil.Logger("bgp.gr")` |
| Env var registration | `env.MustRegister("ze.log.bgp.gr", ...)` | `ze.log.<name>` |
| YANG config keys | `internal/component/*/schema/*.yang`, `grouping`/`container` names | `container gr { ... }` |
| Config consumer | `internal/component/bgp/config/`, anything that does string-keyed lookups in the parsed tree | `tree["bgp"]["gr"]` |
| Dispatch keys | `dispatchCommand("bgp gr ...")`, command prefix matching | `"bgp gr"` |
| Test fixtures | `test/**/*.ci`, `test/**/*.conf`, env vars in tests | `option=env:var=ze.log.bgp.gr` |
| Documentation | `docs/`, `<!-- source: -->` anchors | text references |
| Learned summaries | `plan/learned/*.md` | text references |

**Mechanical check before committing the rename:**

```
old_name="bgp-gr"  # what you are renaming away from
new_name="bgp.gr"  # what you are renaming to
# Show every place that still mentions the old name
grep -rn "$old_name" internal/ pkg/ cmd/ test/ docs/ plan/ .claude/ 2>/dev/null
```

Every match is either a deliberate keep (vendored code, history, learned
summary) or a bug. Do not commit until each match is resolved.

**Naming convention:** subsystem and log keys use dots (`bgp.gr`, `bgp.rib`).
Plugin names registered with `registry.Register()` use hyphens (`bgp-gr`,
`bgp-rib`). The two are NOT the same string. The hub canonicalizes hyphen ->
dot for in-process subsystem names (`938df51d`). When you add a new plugin,
register it with the hyphen form AND make sure every config / log / env
consumer uses the dot form (or the canonicalized form, depending on which
side of the hub it lives on).

## New Plugin Checklist

```
[ ] Create internal/plugins/<name>/<name>.go (package-level logger with SetLogger)
[ ] Create internal/plugins/<name>/register.go (init() → registry.Register())
[ ] Run make generate (regenerates all.go)
[ ] Update TestAllPluginsRegistered expected count
[ ] Add YANG schema if config support (schema/ subdir)
[ ] Add EventTypes if plugin produces custom event types (e.g., ["update-rpki"])
[ ] Add functional tests in test/plugin/
[ ] If plugin sets/reads route metadata: register keys in docs/architecture/meta/README.md, create docs/architecture/meta/<name>.md (see template there)
```

Auto-populated: CLI dispatch, plugin runners, YANG schemas, config roots, family/capability maps, decoder maps.

## Invocation Modes

| Mode | Syntax | Implementation |
|------|--------|----------------|
| Fork (default) | `pluginname` | Subprocess via exec, TLS connect-back |
| Internal | `ze.pluginname` | Goroutine + net.Pipe + DirectBridge (hot path bypasses pipes) |
| Direct | `ze-pluginname` | Sync in-process call |
| Path | `/path/to/binary` | External binary, TLS connect-back |

## Transport

| Plugin type | Transport | Auth | Config |
|-------------|-----------|------|--------|
| Internal (goroutine) | `net.Pipe()` then DirectBridge | N/A | implicit |
| External (local) | TLS over TCP (single connection) | Per-plugin token via `ZE_PLUGIN_HUB_TOKEN` env + cert pinning via `ZE_PLUGIN_CERT_FP` | `plugin { hub { server <name> { host ...; port ...; secret ...; } } }` |
| External (remote) | TLS over TCP (single connection) | Token via out-of-band config | `plugin { hub { server <name> { host ...; port ...; secret ...; } } }` |

External plugins connect back to the engine's TLS listener. Auth is stage 0: `#0 auth {"token":"...","name":"..."}`. Each forked plugin receives a unique random token bound to its name. A plugin cannot use its token to impersonate another. The token is cleared from the OS environment after first read (`Secret: true` on the env registration). The engine also passes its TLS certificate SHA-256 fingerprint via `ZE_PLUGIN_CERT_FP` so the SDK verifies the server identity during the TLS handshake.

## DirectBridge: Choosing the Right Communication Pattern (BLOCKING)

DirectBridge (`pkg/plugin/rpc/bridge.go`) provides typed direct function calls
between the engine and internal plugins, bypassing JSON serialization and socket
I/O entirely. It supports multiple communication patterns. Before designing any
new core-to-plugin communication, read DirectBridge and check whether it already
covers your use case.

Design history: `plan/learned/294-inprocess-direct-transport.md`

| Pattern | Mechanism | Use when |
|---------|-----------|----------|
| Async broadcast (one-to-many) | EventBus (`pkg/ze/eventbus.go`) | A component notifies zero or more listeners about a state change. No return value needed. Example: `(l2tp, session-down)`, `(bgp-rib, best-change)`. |
| Sync request/response (one-to-one) | DirectBridge typed handler | Core calls a plugin function with typed args and waits for a typed result. Example: `ForwardCached`, `DispatchCommand`, `EmitEvent`. |
| Structured event delivery | DirectBridge `DeliverStructured` | Engine delivers pre-parsed event data to internal plugins (zero JSON). Example: `StructuredEvent` for BGP UPDATEs. |
| Text command dispatch | `DispatchCommand` (via bridge or pipe) | Plugin sends a text command to the engine's command registry. Slow path for ad-hoc or external callers. |

**Anti-pattern:** Proposing a new direct-call mechanism when DirectBridge already
provides typed handler slots. The bridge struct has `Set*`/`Has*`/call triplets
for each fast-path handler. Adding a new one follows the same pattern (function
type + `atomic.Bool` + `Set`/`Has`/call methods).

**Anti-pattern:** Using EventBus for request/response. EventBus is pub/sub with
no return channel. Emitting a request event and subscribing for a response event
adds complexity (correlation IDs, timeouts, two event registrations) that a
direct function call avoids entirely.

## Structured Event Delivery (DirectBridge)

Internal plugins that register `OnStructuredEvent` receive `*rpc.StructuredEvent` instead of formatted text. The engine delivers pre-extracted peer metadata + `RawMessage` pointer, eliminating JSON formatting on the engine side and `ParseEvent` on the plugin side.

| Event type | StructuredEvent fields | RawMessage |
|------------|----------------------|------------|
| UPDATE (received/sent) | PeerAddress, PeerAS, LocalAS, Direction, MessageID, Meta | Set — carries `AttrsWire` (lazy attributes) + `WireUpdate` (zero-copy sections) |
| State (up/down) | PeerAddress, PeerAS, State, Reason | nil |
| OPEN, NOTIFICATION, REFRESH | PeerAddress, PeerAS, Direction, MessageID | Set — carries `RawBytes` for wire decoding |

Plugins read attributes via `AttrsWire.Get(code)` (lazy, per-attribute) and NLRIs via `WireUpdate.NLRI()` / `MPReach()` / `MPUnreach()` (zero-copy byte slices). External/forked plugins continue receiving JSON text via `OnEvent`.

## EventBus Typed Payloads (BLOCKING)

`pkg/ze/eventbus.go` carries `payload any`. New events MUST be declared via
`events.Register[T](namespace, eventType)` (typed) or
`events.RegisterSignal(namespace, eventType)` (no payload). Producers call
`Handle.Emit(bus, payload)` and consumers call
`Handle.Subscribe(bus, func(T))`; the registry is the single source of
truth for the payload type.

**Test-stub convention.** Every test file that defines a private mock of
`ze.EventBus` MUST add a compile-time check on the same file:

```go
var _ ze.EventBus = (*<stubName>)(nil)
```

Without this line, an interface change (e.g. `Emit` gaining `any`) compiles
the stub against an outdated signature and only fails when the test
actually constructs the stub. The current 8 stub files
(`pkg/ze/ze_test.go`, `internal/plugins/{sysrib,ntp,ifacedhcp}/*_test.go`,
`internal/plugins/ifacenetlink/monitor_linux_test.go`,
`internal/component/iface/{migrate_linux,integration_helpers_linux,config}_test.go`,
`internal/component/plugin/{server,manager}/*_test.go`,
`internal/component/bgp/plugins/rib/rib_bestchange_test.go`) all carry
this assertion. New stubs without it should fail review.

Subscribers MUST type-assert via the typed handle (`Event[T].Subscribe`)
rather than calling `bus.Subscribe` directly. The handle's wrapper logs a
warn on type mismatch; raw `bus.Subscribe` callers swallow mismatches
silently. The legacy `events.AsString` shim exists only for events that
have not yet migrated to a typed handle and is not for new code.
