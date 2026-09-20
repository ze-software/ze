# IS-IS Component and Config

The IS-IS component is the wiring backbone every runtime layer builds on. It
registers the component, embeds `ze-isis-conf.yang`, resolves the top-level
`isis { ... }` subtree into typed Go structs, applies defaults, validates the NET
and system ID, runs the SDK lifecycle, and installs the PDU receive dispatcher.

| Concern | File |
|---------|------|
| Registration and SDK lifecycle | `register.go` |
| Typed config, parsing, defaults, validation | `config.go` |
| Event namespace and typed handles | `events.go` |
| PDU dispatcher and engine circuit management | `server.go` |
| Config schema | `yang/ze-isis-conf.yang` |

<!-- source: internal/plugins/isis/register.go -- the init registration and the SDK lifecycle hooks -->
<!-- source: internal/plugins/isis/config.go -- parseISISConfig, validateConfig, the tree coercers -->

## Decision: registration, never a switch

The component is a `registry.Registration` created in `init()`, modeled on LDP:
name `isis`, the embedded YANG, `ConfigRoots ["isis"]`, dependencies on
`fib-kernel` and `sysctl`, and the logger, metrics, event-bus and CLI hooks. Core
discovers IS-IS through the registry and never imports it. The only import of the
component outside itself is the generated composition root.

## Decision: the NET validators live centrally, the completion locally

The `ValidateFn` for the NET and the system ID are registered in the config
component (`isis-net`, `isis-system-id`), because the config package cannot
import the IS-IS component without an import cycle. The IS-IS component registers
only the `CompleteFn` guidance from its own registration. This split keeps
`ze:validate "isis-net"` self-contained for completion while breaking the cycle
for validation.

<!-- source: internal/component/config/validators.go -- ISISNETValidator, ISISSystemIDValidator -->

## Decision: three callbacks, and only one fires on reload

`OnConfigVerify` parse-checks and stashes a pending config. `OnConfigure` is
startup-only and stages the active config. `OnConfigApply` is the reload commit
step and the **only** callback that fires on a reload; it adopts the pending
config and calls the engine's reconcile, which journal-diffs interfaces so a
metric-only change flaps no circuit.

## Decision: a commit is written into the running circuit where it can be

The reconcile splits a changed interface in two. `applyCircuitParams` writes the
Hello timers and the DIS priority into the live `circuit.Circuit` and wakes the
per-circuit hello worker, which restarts its tickers at the new period and sends
an IIH at once. The immediate IIH is the protocol half of the change: a neighbor
holds the adjacency only for the holding time the last IIH told it (ISO/IEC 10589
clause 8.2), so a longer interval that waited for its first tick would expire the
adjacency. The metric and the per-level election priority need no write, because
origination and the DIS election read them out of `e.running` every time they
run; the reconcile re-originates once at the end so a metric change does not wait
for the next adjacency transition.

`circuitNeedsRebuild` names the three parameters that decide what the circuit IS:
the kind, the level set, and the address families. Each is read once by
`buildCircuit`, so the reconcile closes the circuit and opens it again, which
flaps every adjacency on the link. Keeping that set at three is the point of the
split.

The circuit is still built once from the NODE-level config (system ID, area
addresses), and the reconcile diffs interfaces only, so a committed `net` or
`system-id` change does not reach a running circuit.

<!-- source: internal/plugins/isis/server.go -- reconcile, circuitNeedsRebuild, circuitParamsEqual -->
<!-- source: internal/plugins/isis/circuits.go -- applyCircuitParams, launchCircuitGoroutine -->

A config with no NET is treated as "not present" and leaves the engine idle,
following the LDP precedent of a missing LSR ID. The required-field policy lives
in `validateConfig`, not in the parser, so verify can stage a partial config the
same way it rejects one.

## Decision: the dispatcher reads the PDU type from the raw frame

The receive dispatcher keys on the low 5 bits of the PDU type octet read straight
from the raw frame, without round-tripping through the full header decoder. A
malformed or attacker-controlled PDU is bounds-checked and dropped with a count,
never panicked on. Handlers register at startup; each runtime layer registers its
own hello, LSP, CSNP or PSNP handler. The transport delivers a raw frame and
holds no protocol switch.

<!-- source: internal/plugins/isis/server.go -- the PDU-type dispatcher and the engine circuit lifecycle -->

## Decision: the system ID is derived from the NET

When no explicit `system-id` leaf is given, the system ID is the first NET's 6
octets before the 1-octet NSEL (ISO/IEC 10589 section 6.2). An explicit system ID
that disagrees with the NET is rejected.

## Constraint: the YANG carries maximal native validation

Every numeric, enum and identifier leaf carries `range`, `pattern`,
`enumeration`, or `length`, so out-of-range metric, priority and lifetime values
and a bad level enum are rejected at schema validation before the engine sees
them. The custom validators handle the NET, the system ID, and every
`auth-key-chain` reference, which are the checks native YANG cannot carry. A
reference is checked against the `key-chains` list of the same config, so a name
that resolves to no chain is refused at commit with the name in the error:
`newKeyStore` cannot tell a dangling name from an unset one, and both would leave
the circuit signing and accepting nothing.

Defaults are mirrored as Go constants **and** asserted equal to the YANG defaults
by a test that reads the YANG file from disk, so the two cannot drift silently.

## Trap: the delivered config shape

The SDK delivers the `isis` subtree **root-wrapped** (`{"isis": {...}}`).
`Tree.ToMap` renders every leaf as a **string** (`"10"`, not `10`), keyed lists
(interfaces, key chains) as a **key-to-entry map** rather than an array, and a
single-element leaf-list (`net`) as a **bare scalar** while a multi-element one
is a slice.

The resolver has coercers for exactly this shape. Assuming native JSON numbers or
arrays breaks the parse silently: the engine reports no NET and idles. This is
the single most load-bearing fact for anyone extending the config.

## Trap: a redistribute source must register at init, not at start

`ze config validate` links in every component but does **not** start the engine.
A redistribute source registered only from `OnStarted` is therefore too late, and
`import isis` fails validation. The source registers from `init()`.

The redistribute **consumer** needs the engine handle, so it registers at
`OnStarted` and uses the re-register call, which is idempotent: an SDK reconnect
that builds a fresh engine re-wires instead of failing with a consumer conflict.

## Trap: level defaults and the per-level override container

The `level` default token is `l1-l2` in kebab case, and the parser falls through
to the dual-level value for any unrecognized string, so an omitted or empty level
is the dual-level default.

A per-level interface override container uses **zero as "inherit"**, with no
defaults applied. That is distinct from the circuit-wide leaves, which do get
defaults.

## Owned diagnostic codes

`doctor-isis-net-missing` and `doctor-isis-system-id-mismatch`, plus the
`isis-config-sanity` check. The raw-socket check and its code stay owned by the
transport (see [`isis-3-l2-transport.md`](isis-3-l2-transport.md)). Each
`ze_isis_*` metric series is registered by the layer that produces it; this layer
only threads the registry through.
