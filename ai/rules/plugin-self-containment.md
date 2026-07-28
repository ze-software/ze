# Plugin Self-Containment

**When:** adding or moving a plugin's command, schema, help text, or doctor check
**Severity:** blocking

## Directives

**A plugin owns its ENTIRE feature surface. Remove the plugin and every one of
its features disappears; every OTHER plugin and the core keep working.**

This is the load-bearing invariant of the registration architecture. It is the
"delete the folder" test from `ai/rules/plugin-design.md` (Proximity Principle),
stated for the full user-facing surface, not just internal wiring.

## The Removal Test

Deleting a plugin's package directory plus its blank import in
`internal/component/plugin/all/all.go` MUST:

1. **Remove every user-visible feature of that plugin** and nothing else:
   CLI commands, `show`/`set`/`clear`/`delete`/`update` subtrees, RPC registration,
   offline command registration, YANG command schema, help and usage text,
   completion entries, web/looking-glass routes, doctor checks, metrics.
2. **Keep the build green.** No dangling reference, no orphaned command
   spelling, no half-registered command anywhere.
3. **Keep every other plugin and the core fully working.** Removing BGP must
   not break iface, firewall, l2tp, or the generic command plumbing.

If removing a plugin would break a different plugin, the core, or leave a
broken/empty command, the surface is in the wrong package. Move it to the owner.

## What This Forbids

| Anti-pattern | Why it fails the removal test |
|--------------|-------------------------------|
| Plugin command spelling in generic dispatch (`internal/component/plugin/server`) | Deleting the plugin leaves dead BGP/iface knowledge in shared code |
| A plugin's subtree in a central verb schema (e.g. `show bgp ...` in `internal/component/cmd/show/yang/ze-cli-show-cmd.yang`) | Deleting the plugin leaves a `show bgp` branch with no handler |
| Plugin handlers registered from a central verb package (`internal/component/cmd/show`, `internal/component/cmd/delete`, ...) | Deleting the plugin leaves the central package referencing gone symbols |
| Help / usage / inventory strings that hardcode a plugin's commands in a generic package | Deleting the plugin leaves help advertising commands that no longer exist |
| The CLI helper (`cmd/ze/internal/cmdutil`) special-casing a plugin's selectors | Selector handling is generic; per-plugin knowledge belongs to the owner |

## What Shared Code MAY Do

Generic command plumbing carries **selector scope**, not command spelling.
The dispatcher may extract a typed selector value because a YANG `ArgDef`
declares it (`internal/component/plugin/server/command.go`), but it must not
contain the words `peer`, `bgp`, `bfd`, or any plugin's grammar. The
classification rule from `plan/learned/844-command-grammar-ownership-first.md`:
shared dispatch may carry selector scope; it must not own a plugin's command
spelling.

## Finding the Owner: follow the code, not the wire-method namespace

**The `ze-<ns>:` prefix on a WireMethod is a label, not an ownership claim, and is
often a legacy misnomer. Determine the owner by what the handler actually calls,
then place the handler, schema, and registration in that package.**

Trace the handler's real dependencies:

| Command (WireMethod) | What the handler calls | Owner |
|----------------------|------------------------|-------|
| `ze-show:ip-route` / `ze-show:neighbors` / `ze-show:kernel-routes` | `iface.ListKernelRoutes` / `iface.ListNeighbors` (kernel tables via the iface backend) | `internal/component/iface` (NOT central `show`; NOT the BGP RIB) |
| `ze-bgp:pool-stats` | `bgp/plugins/rib/pool` attribute-pool metrics | BGP RIB plugin |
| `ze-bgp:metrics-values` / `ze-bgp:metrics-list` | generic core Prometheus registry (`internal/core/metrics`) | generic, stays central |
| `ze-bgp:subscribe` / `ze-bgp:unsubscribe` | generic `pluginserver` subscription manager | generic, stays central |
| `ze-show:policy-list` | cross-plugin filter-type registry (`registry.FilterTypesMap`) | generic, stays central |

A command is **generic (stays central)** only when it has no single removable
owner: it aggregates a cross-plugin registry, reads a generic core system, or is
process-global (`show warnings`, `show health`, `subscribe`). Everything that
reads one plugin's or component's state belongs to that owner, regardless of the
`ze-<ns>:` label on its WireMethod.

## Where a Plugin's Surface Lives

| Surface | Owner location |
|---------|----------------|
| RPC handler + `pluginserver.RegisterRPCs` | owner package (e.g. `internal/component/<owner>/cmd/` or the plugin's own package) |
| YANG command schema (full path from root, e.g. `show bgp peer ...`) | `<owner>/yang/ze-<x>-cmd.yang`, NEVER `<owner>/cmd/yang/` |
| Offline/root command + handler | owner package via the offline command registry |
| Help / usage / completion | derived from the owner's registry + schema (see `ai/rules/derive-not-hardcode.md`) |
| Doctor check + its unit test | owner package (Proximity Principle) |

Central verb packages (`internal/component/cmd/show`, `internal/component/cmd/delete`, ...) keep ONLY
generic cross-system commands (`show warnings`, `show health`), never a specific
plugin's commands.

### How to carve a command into its owner

1. **Handler:** add `func init() { pluginserver.RegisterRPCs(...) }` + the handler
   in the owner package. If the owner package is already blank-imported (it has a
   `register.go` found by the generator's `pluginDirs`, or sits in `rpcDirs`),
   the registration links with NO generator or manual-island change. The handler
   imports only `plugin` + `pluginserver` (+ the owner's own API), so it does not
   create an import cycle.
2. **Schema (container merge, NOT `augment`):** add `<owner>/yang/ze-<x>-cmd.yang`,
   a standalone module that re-declares the path from the root:
   `container show { container <x> { ... ze:command "ze-show:<x>"; } }`. The YANG
   loader unions same-named top-level containers across all registered modules, so
   the owner module needs no `import`/`augment` of the central schema and has no
   base-module coupling. Give it a unique `namespace`/`prefix` and
   `import ze-extensions`. Add the embed var + `yang.RegisterModule` call. A NEW
   `<owner>/yang/` package whose `register.go` imports `config/yang` is
   auto-discovered, so run `go run scripts/codegen/plugin_imports.go` to refresh
   `internal/component/plugin/all/all.go`.
3. **Schema location:** the command YANG lives in `<owner>/yang/` (top level,
   sibling of `cli`/`cmd`), NEVER nested under `<owner>/cmd/yang`.
4. **Both halves of the invariant:** the owner `yang/` gets a presence test
   asserting its command tokens ARE declared; the central verb schema test bans
   the moved tokens (below).

### Unowned verb roots (multi-owner verbs)

A verb whose subcommands belong to several owners (e.g. `monitor bgp`,
`monitor vpn ipsec`, `monitor ping`) must NOT declare its root container inside
any one plugin. If it does, deleting that plugin deletes the whole verb. The
root lives in a central, plugin-free package `internal/component/cmd/<verb>`
(a `doc.go` that blank-imports its `yang/` subpackage, mirroring `internal/component/cmd/delete`);
each owner container-merges only its own subtree onto that root. Precedent:
`internal/component/cmd/monitor` holds the `container monitor` root, while
`monitor bgp` stays in the BGP plugin and the other subcommands carve out to
their feature owners. The central package holds NO handlers; subcommand handlers
register from their owners.

**The root anchor stays even when it declares zero commands, and is REQUIRED, not
optional.** Once every subcommand of a verb has carved out to an owner, the central
verb schema is a bare `container <verb>` with no `ze:command` leaf of its own. Do
NOT delete it. `internal/component/cmd/clear` is the precedent: `clear interface
counters` (iface), `clear dns cache` (resolve), `clear vpn ipsec sa` (ike),
`clear l2tp ...` (l2tp), and `clear bgp rib ...` (bgp) are all owner-owned, so
`ze-cli-clear-cmd.yang` declares only the bare `container clear` anchor. Owners
attach to that anchor two ways, and the second one has a hard dependency on it:

- **Container-merge:** the owner declares its own `container <verb> { container <noun> ... }`
  and the YANG loader unions same-named roots (iface, resolve, ike use this for `clear`).
  Preferred for new carves (no base-module coupling; see "How to carve" above).
- **Augment:** the owner declares `augment "/<prefix>:<verb>" { container <noun> ... }`
  against the anchor module (l2tp and bgp use this for `clear`, via `augment "/cliclearcmd:clear"`).
  An augment names its target module, so deleting the anchor breaks every augmenting
  owner's build. This is the concrete reason the bare anchor must remain.

The anchor still owns NO command. The central guard test bans each carved token so
a command cannot drift back into the central schema (for `clear`:
`internal/component/cmd/clear/yang/self_containment_test.go`,
`TestClearSchemaHasNoMigratedOwnerCommands`; each owner `yang/` holds the matching
presence test, e.g. `TestResolveCmdSchemaOwnsClearDNSCache`).

### Dedicated feature modules

When one feature spreads across several verbs (a one-shot root command, a `show`
view, a `monitor` stream, a `resolve` variant), give the feature its own module
`internal/component/<feature>` that owns every one of those commands, rather than
scattering them across the verb packages. Create the module if none exists. When
two such modules would share low-level primitives (e.g. ping and traceroute both
build ICMP echo packets and resolve targets), extract those primitives to a
`internal/core/<x>` package (e.g. `internal/core/probe`) so neither feature
module depends on the other or on a central verb package.

### Registration over hardcoding (the CLI client too)

The registration discipline applies to the **CLI client model**, not only the
daemon's command/schema tree. The daemon registers streaming views generically
(`pluginserver.RegisterMonitorProvider(MonitorProvider{Prefix, CreateFn})` plus
`RegisterStreamingHandler(prefix, handler)`, resolved by longest-prefix
`matchesPrefix` in `internal/component/plugin/server/handler.go`); the Bubble Tea
client mirrors this with its own view registry and must not regress into
per-feature hardcoding.

Anti-pattern (removed 2026-07-19): each rich live view (dashboard, traceroute,
ping, traffic) adding its own field + factory + state + dispatch to the core
`cli.Model` (`internal/component/cli/model*.go`), wired one-by-one in
`cmd/ze/hub/session_factory.go` and `internal/component/cli/client/main.go`. Every
new view then edits the core struct in 4-5 places -- the opposite of "the core
discovers features through a registry."

Correct (live in the tree): the client-side view registry in
`internal/component/cli/view_registry.go` (`RegisterView(viewSpec{key, prefix,
matches, start})`, `RegisteredViews()`, longest-prefix `resolveView` copied from
`handler.go` `matchesPrefix`). Each view registers from its own
`register_view_*.go` `init()` (`ViewKeyPing`/`ViewKeyTraceroute`/`ViewKeyDashboard`)
and hangs its session state off the single `Model.activeView` handle plus the
generic `Model.viewFactories` store -- no per-feature field. Consumers iterate
`cli.RegisteredViews()` and inject each factory by key via `SetViewFactory`, not
three typed setters. The `TestModelHasNoPerFeatureViewField` reflection guard
(`internal/component/cli/model_test.go`) fails if a per-feature field returns.

General test for any spec: **a new feature must not require editing a `switch`,
`case`, field list, or factory in a core or shared package -- it registers and is
discovered.** This is the "Registration over hardcoding" review item carried by
`plan/TEMPLATE.md` and warned for by `.claude/hooks/validate-spec.sh`.

## Mechanical Check

A removal-compliance test must exist and run in verification: build (or analyse
the command/schema registries) with a plugin's provider import removed and
assert no command, schema node, help string, or handler reference to that
plugin survives in any generic or central package. See
`ai/rules/discovery-updates.md` — adding this invariant means adding the gate
that enforces it.

The first instance is `TestShowSchemaHasNoBGPPluginCommands`
(`internal/component/cmd/show/yang/self_containment_test.go`): it asserts the
central `show` verb schema declares no part of the `show bgp ...` subtree
(`ze-rib-api:`, `ze-bgp:peer-`, `ze-show:bgp-decode`, `ze-show:bgp-encode`),
because `show bgp rib ...` / `show bgp peer ...` are owned by
`internal/component/bgp/plugins/cmd/{rib,peer}/yang` and the offline
`show bgp decode` / `show bgp encode` diagnostics are owned by
`internal/component/bgp/cli/yang`. The owner half is asserted by
`internal/component/bgp/cli/yang`'s `TestBGPToolsSchemaOwnsDecodeEncode` (the surface moved,
it did not vanish).

Non-BGP owners share one general central guard,
`TestShowSchemaHasNoMigratedOwnerCommands` (same file), whose banned-token map
grows by one entry per carved owner (flow-export, rsvp-te, ldp, policy-routes,
static, vpn-ipsec, vpp, the iface kernel reads, ...); each owner's `yang/`
package holds the matching presence test (e.g. `TestRSVPTECmdSchemaOwnsShowRSVPTE`).
When you carve a new command, add both halves: the banned token here and the
presence assertion in the owner. Extend the same pattern to the other central
verb schemas (`internal/component/cmd/delete`, `internal/component/cmd/set`, ...) as they are made compliant.

## Related

- `ai/rules/plugin-design.md` — Proximity Principle, import rules.
- `ai/patterns/cli-command.md` — owner-owned command registration.
- `ai/rules/cli-grammar.md` — typed selectors, command grammar ownership.
- `plan/learned/844-command-grammar-ownership-first.md` — ownership-before-grammar.
