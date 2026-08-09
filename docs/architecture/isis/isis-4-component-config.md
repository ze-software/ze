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
them. The custom validators handle only the NET and the system ID, where native
YANG is insufficient.

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
