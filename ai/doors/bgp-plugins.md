# The BGP plugins door

<!-- door-base: internal/component/bgp/plugins -->

Read `ai/doors/system.md` first, and `ai/doors/bgp-engine.md` for the engine
these plugins extend. This door covers `internal/component/bgp/plugins/*`:

- the RIBs (`rib`, `adj_rib_in`)
- the route server and reflector (`rs`, `rr`)
- `gr`
- the NLRI codecs under `nlri/`
- the `filter_*` plugins
- `rpki`, `bmp`, `role`
- the command plugins under `cmd/`
- the rest

How the RIB stores and selects routes is in `ai/digests/rib.md`. How a plugin
talks to the engine is in `ai/digests/plugin-transport.md`.

## Where it sits

- **Each is an edge plugin of the BGP component.** It registers from `init()`
  in its own `register.go` and is discovered through the generated composition
  root. Nothing names it in a list (`ai/patterns/plugin.md`,
  `ai/rules/plugins.md`). They are gated with BGP by `ze_bgp`. `bmp` also needs
  `ze_bmp` (`feature-gates.txt`).
- **Every one can run in another process.** A plugin runs internal (a
  goroutine) or external (a forked subprocess over TLS RPC), and the SDK hides
  which (`docs/architecture/plugin/plugin-system.md`, "The process boundary").
  Where behavior must differ, the plugin asks: `rib` and `adj_rib_in` branch on
  `p.IsInternal()` (`rib/rib.go`, `adj_rib_in/rib.go`).
- **The in-tree plugins hold no privilege over a third-party one** on the
  protocol: the same `#<id> <verb> [json]` RPC, the same five stages, the same
  events (`docs/guide/plugins.md`, "Writing External Plugins"). The exceptions
  are the compiled-in hooks listed under Boundaries.

## Who depends on it from outside

| Outside party | Contract | Page |
|---------------|----------|------|
| A third-party BGP plugin author | The process protocol, the Stage 1 `declare-registration` (families, filters, cache consumer, claims, `signals-session-ready`), capability injection, the event and command formats. The in-tree plugins are the examples such an author reads | `docs/plugin-development/protocol.md`, `docs/architecture/api/process-protocol.md`, `docs/architecture/api/capability-contract.md` |
| An operator | Each plugin's YANG config, its commands, the `<type>:<name>` filter reference (`Registration.FilterTypes`), and the alias form `internal rs { use bgp-rs }` | `docs/guide/plugins.md`, `docs/guide/configuration.md` |
| Another plugin | Route metadata keys a plugin sets or reads, and the dependency order on state events (`bgp-gr` before `bgp-rib`) | `docs/architecture/meta/README.md`, `docs/architecture/plugin/plugin-relationships.md` |
| A protocol peer or server | The RFCs these plugins implement: a GR peer, an RPKI cache, a BMP collector, a route-server client | `rfc/short/`, `docs/guide/graceful-restart.md`, `docs/guide/rpki.md`, `docs/guide/bmp.md` |
| The public catalog | Registration metadata is published as the plugin catalog | `docs/architecture/plugin/plugin-system.md`, "Registration metadata feeds the website catalog" |

## Boundaries

- **Reach the engine through the bus, DirectBridge or a text command, never a
  plain call into `reactor` or another component.** Internal, it works. External,
  it changes the subprocess's own copy and nothing else (`./le plugin boundary
  check`). A plugin never imports another plugin's implementation (`./le arch
  tier check`).
- **A plugin declares twice, and both must agree.** `registry.Registration` is
  read by anything that links the composition root. The `sdk.Registration`
  passed to `p.Run` is what a running daemon receives at Stage 1
  (`plugin-system.md`, "The process boundary", and `./le plugin declarations check`).
- **Some hooks exist only in-process.** Know which before you rely on one for
  a feature a third party must also reach:

| Hook | In-process only? | Source |
|------|------------------|--------|
| `PeerUpBarrier` | Yes. `registry.RequiresPeerUpBarrier` reads the compiled registry, and Stage 1 has no field for it | `internal/component/plugin/registry/registry.go`, `pkg/plugin/rpc/types.go` |
| `SignalsSessionReady` | No. An external process declares it at Stage 1, answered through `registry.SetRuntimeSessionReady` | `registry.go` `SignalsSessionReady` |
| `filterapi.Register` filters | Yes, Go function pointers. An external filter uses `filter-update` | `internal/component/bgp/filterapi/filterapi.go` `Register` |
| `InProcessNLRIDecoder`, `InProcessNLRIEncoder` and the other `InProcess*` fields | Fast path. An external plugin answers through `OnDecodeNLRI`/`OnEncodeNLRI` | `registry.Registration` comments |

- **A barrier or a report is about timing, never about permission.** The
  peer-up barrier holds the End-of-RIB at most 2 s. The session-ready report
  never holds it (owner ruling, 2026-09-18). Both are in `plugin-system.md`,
  "Peer-up barrier" and "Session-ready reports".
- **A family or a capability is declared, never enumerated:**
  `family.MustRegister`, `Registration.Families` and `CapabilityCodes`
  (`plugin-system.md`, "Address family registration", and `ai/patterns/bgp-family.md`).

## Read before code

| Read | It answers |
|------|------------|
| `docs/architecture/plugin/plugin-system.md` | Registration fields, startup phases, claims, the barrier, session-ready, the process boundary |
| `docs/guide/plugins.md` | What an operator configures for each BGP plugin, and how an external plugin connects |
| `docs/architecture/api/architecture.md` | The engine and plugin split, `attach process`, the startup barrier |
| `docs/architecture/api/process-protocol.md` | Every RPC and event payload a plugin sends or receives |
| `docs/architecture/plugin/rib-storage-design.md`, `ai/digests/rib.md` | How `bgp-rib` stores routes and runs best path |
| `docs/architecture/meta/README.md` | The route metadata keys a plugin can set or read |
| `ai/digests/plugin-transport.md` | The handshake, EventBus and DirectBridge path for one plugin |
| `ai/patterns/plugin.md`, `ai/patterns/bgp-family.md` | The file layout and registration of a new plugin or family |
| `ai/rules/plugins.md` | Declarations as pure values, cross-boundary types, registration-based dispatch |

## Traps

- **A plugin that owns config and registers no `OnConfigApply` refuses every
  commit.** The SDK binds the missing callback to `refuseConfigApply`
  (`pkg/plugin/sdk/sdk_callbacks.go`). The RECURRING-PATTERNS entry "An
  unregistered plugin callback answers OK" describes the behavior before that
  fix.
- **A capability plugin must declare both the decoder and the codes.** With
  one of the two, it registers and is never called. **Its YANG needs one augment
  per template scope.** GR needed four (`plan/learned/DESIGN-HISTORY.md`,
  "Plugin system: architecture", load-bearing invariants).
- **A subscription delivers future events only.** A late subscriber queries
  and reconciles. There is no replay (same page).
- **Replay on reconnect is selective, never bulk:** a bulk replay re-announces
  routes the operator withdrew (same page, `bgp-watchdog`).
- **A forwarder keeps per-source FIFO order and never drops a cached UPDATE**,
  because the cache ack is cumulative (same page, "BGP engine: wire encoding
  and RIB").
- **stdout carries the protocol.** A plugin logs to stderr only (same page).
- **A registration whose engine does nothing still auto-loads.**
  `bgp-nlri-srpolicy` returned 0 from `RunEngine` and broke startup for any
  config that named its family (`plan/journal/unwired-feature.md`, 2026-10-17,
  and `plan/journal/inert-registration-offered-as-a-choice.md`).
- **A queue of pooled-buffer copies is a hot-path allocation** (BMP,
  `plan/journal/handoff-queue-copies-the-pooled-buffer.md`).
- **A new plugin breaks two hand-kept test lists**
  (`plan/learned/RECURRING-PATTERNS.md`, "Plugin list hardcoded in two test files").

## Gates

| Command | It judges |
|---------|-----------|
| `./le plugin boundary check` | No unguarded in-process call that breaks when the plugin runs external |
| `./le plugin declarations check` | The two declarations agree |
| `./le plugin imports check`, `./le arch tier check` | The composition root is current. No plugin imports another's implementation |
| `./le arch enumeration check` | No literal in the change restates what a registry holds |
| `./le test plugin-external` | A registered engine plugin run as an external process, refusing or warning where it must |
| `./le test functional plugin`, `./le test functional reload` | Plugin startup, events and reload through the real daemon |
| `./le rfc check` | RFC rows and tags for the protocols these plugins implement |
| `./le job run label <label> command go test -tags ze_bgp <package>` | The scoped package test. Add `ze_bmp` for `bmp` |
