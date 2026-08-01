# 930 -- isis-4-component-config

## Context
Spec `isis-4-component-config` is the wiring backbone of the IS-IS spec set: it
creates the `internal/component/isis/` component, registers it in the plugin
registry, embeds `ze-isis-conf.yang`, resolves the top-level `isis { ... }` config
subtree into typed Go structs, applies YANG defaults, validates the NET/system-id,
declares the IS-IS event namespace, and runs the SDK lifecycle
(verify/configure/apply/start/command) so that an `isis` block reaches a running
engine that opens one L2 circuit per enabled interface. This is the integration
skeleton every runtime sibling (adjacency isis-5, LSDB isis-6, flooding isis-7,
DIS isis-8, SPF isis-9, auth isis-10, redistribution isis-11, IPv6 isis-12, CLI
isis-13) layers on. The component is implemented and the chain config -> schema
-> SDK callbacks -> engine -> transport is closed; the per-circuit goroutines and
PDU handlers this spec installs as stubs are filled by the siblings (most of which
already coexist in the same package tree).

## Decisions
- The component is a `registry.Registration` in `init()` modelled directly on the
  LDP `registerLDP`/`runLDPEngine` pair: `Name "isis"`, `Features "yang"`, the
  embedded `ZeIsisConfYANG`, `ConfigRoots ["isis"]`, `Dependencies ["fib-kernel",
  "sysctl"]`, `RunEngine runISISEngine`, and the `ConfigureEngineLogger /
  ConfigureMetrics / ConfigureEventBus / CLIHandler` hooks. Registration not
  switch-dispatch: core discovers IS-IS through the registry, never imports it
  (the only `isis` import outside the component is the generated `all.go`).
- The NET / system-id `ValidateFn`s live CENTRALLY in
  `internal/component/config/validators_register.go` (`ISISNETValidator()` /
  `ISISSystemIDValidator()` registered as `isis-net` / `isis-system-id`), because
  the config package cannot import the isis component without an import cycle. The
  isis component owns only the CompleteFn guidance, registered from its own
  `registerISIS()` via `configyang.RegisterCompleteFn` (the mac-address
  precedent). This split keeps `ze:validate "isis-net"` self-contained for
  completion while breaking the cycle for validation.
- `OnConfigVerify` parse-checks and stashes a pending config; `OnConfigure`
  (startup-only) stages the active config; `OnConfigApply` (the reload-commit
  step, the only callback that fires on reload) adopts the pending config and
  calls `eng.reconcile`, which journal-diffs interfaces so a metric-only change
  flaps no circuit (AC-8). A config with no NET is "not present" and leaves the
  engine idle (the LDP-with-no-lsr-id precedent), so verify can stage a partial
  config the same way it rejects it; the required-field policy (ErrNoNET,
  ErrSystemIDMismatch) is in `validateConfig`, not `parseISISConfig`.
- The PDU-type receive dispatcher (`server.go`, owner isis-4) keys on the 5-bit
  PDU type (`rf.PDU[4] & 0x1f`) read from the raw frame WITHOUT round-tripping
  through the full header decoder, so a malformed/attacker-controlled PDU is
  bound-checked and dropped (counted), never panicked. Handlers register at
  startup; isis-4 installs the dispatcher, the siblings register the real IIH /
  LSP / CSNP / PSNP handlers. The transport delivers `(RawFrame)` and holds no
  protocol switch.
- System ID is DERIVED from the first NET's 6 octets before the 1-octet NSEL
  (ISO/IEC 10589 6.2) when no explicit `system-id` leaf is given (AC-9); an
  explicit system-id that disagrees with the NET is rejected (ErrSystemIDMismatch).

## Consequences
- A top-level `isis { net ... }` block now validates through the real YANG schema
  and the custom NET validator, and starts the component; `ze plugin` lists
  `isis`. The whole IS-IS feature set is reachable from one config root.
- The YANG carries maximal native validation (range/pattern/enumeration/length)
  on every numeric/enum/id leaf, so out-of-range metric/priority/lifetime and
  bad-enum level are rejected at schema validation before the engine; the custom
  validators handle only NET and system-id where native YANG is insufficient.
- Defaults are mirrored as Go constants AND asserted equal to the YANG defaults by
  `TestISISConfigBoundaries` (reads `yang/ze-isis-conf.yang` from disk), so the
  two cannot drift silently.

## Gotchas / Traps
- The SDK delivers the `isis` subtree as ROOT-WRAPPED JSON (`{"isis": {...}}`) and
  `Tree.ToMap` renders every leaf as a STRING ("10", not 10), keyed lists
  (interfaces, key-chains) as a key->entry MAP (not an array), and a single-element
  leaf-list (`net`) as a BARE SCALAR while a multi-element one is a `[]any`. The
  config resolver has `configNumber` / `configBool` / `configLeafList` / `keyedList`
  coercers precisely for this shape; assuming native JSON numbers or arrays breaks
  the parse. This was assumption A-2 and is the single most load-bearing fact for
  anyone extending the config.
- The redistribute config SOURCE name "isis" must be registered at `init`
  (`isisredistribute.RegisterISISSources()` from `registerISIS`), NOT in
  `OnStarted`: `ze config validate` links in every component but does NOT start the
  engine, so an OnStarted-only registration is too late and `import isis` would
  fail validation. The redistribute CONSUMER is wired in `OnStarted` and uses
  `ReregisterConsumer` (idempotent) so an SDK reconnect that builds a fresh engine
  re-wires rather than failing with ErrConsumerConflict.
- One code, one owner: the raw-socket doctor check + diagnostic code are owned by
  the transport (isis-3), never by the component; isis-4 registers only the two
  config-sanity codes (`doctor-isis-net-missing`, `doctor-isis-system-id-mismatch`)
  and the `isis-config-sanity` doctor check (order 731, just after the isis-3
  raw-socket check at 730). The `ze_isis_*` metric series are likewise registered
  per-owner by the runtime siblings; isis-4 only threads the registry through.
- The `level` field default token is `l1-l2` (kebab), and `parseLevel` falls
  through to `LevelL1L2` for any unrecognised value, so an omitted or empty level
  is the dual-level default. A per-level interface override container uses ZERO as
  "inherit" (no defaults applied), distinct from the circuit-wide leaves which DO
  get defaults.

## Interop validation pending Linux execution
- isis-4 introduces NO wire-protocol behaviour (it is component/config wiring), so
  the spec's Interop Tests section is N/A by design: cross-vendor protocol interop
  is owned by the runtime siblings (isis-5 adjacency, isis-9 convergence, isis-13
  full FRR scenarios). The FRR scenario directories that exist under
  `test/interop/scenarios/isis-*-frr` (auth, convergence, dualstack, p2p,
  lan-dis, redist) belong to those siblings; they are written but their on-the-wire
  execution is pending a Linux/QEMU host (the development host is darwin, where
  AF_PACKET raw L2 is unavailable). For isis-4 itself the proof is unit + build +
  the `test/isis/isis-config.ci` functional test, all run on darwin and passing.
- Note observed at closure: running the whole `ze-test isis` suite on darwin shows
  `isis-flooding` (test 7, owner isis-7) failing a `start-lsp-id` stdout assertion;
  this is a sibling-spec functional test, not isis-4's. The isis-4 functional test
  `isis-config` (test 3) passes in isolation (`ze-test isis 3`, exit 0).

## Files
- `internal/plugins/isis/register.go`: `init()` registration, `runISISEngine`
  SDK lifecycle, central-validator CompleteFn registration, diagnostic-code +
  doctor registration, NET/system-id completions.
- `internal/plugins/isis/config.go`: typed `Config`/`InterfaceConfig`/
  `KeyChainConfig` structs, `parseISISConfig`, default application, `validateConfig`,
  the config-tree coercers, system-id derivation.
- `internal/plugins/isis/events.go`: `Namespace` + `EventSessionUp/Down/LSPChange`
  + typed handles + `eventSink`.
- `internal/plugins/isis/server.go`: the 5-bit PDU-type `dispatcher`, the
  `engine` (open/reconcile/shutdown circuits, journal diff).
- `internal/plugins/isis/yang/ze-isis-conf.yang`: top-level `isis` container with
  `ze:config-root`, every leaf maximally validated; `register.go` / `embed.go` are
  generated glue (`make generate`).
- `internal/component/config/validators_register.go` (+`validators_isis_test.go`):
  central `isis-net` / `isis-system-id` ValidateFns.
- `internal/component/plugin/all/all.go` (generated): imports `isis` + `isis/yang`.
- `internal/test/cli/register.go`, `mk/test-functional.mk`: `test/isis` suite
  registration + `ze-isis-test` target.
- `test/isis/isis-config.ci`: valid `isis` config validates and starts; invalid NET
  rejected.
