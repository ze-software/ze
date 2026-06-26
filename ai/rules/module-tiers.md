# Module Tiers (core / component / plugin)

**When:** creating a new package under `internal/`, or deciding whether something
belongs in `internal/core/`, `internal/component/`, or `internal/plugins/`.

**Where a Go package lives under `internal/` is decided by dependency direction,
not by size or age. Three tiers, two mechanical axes. New code MUST land in the
correct tier; an engine in the wrong tier fails `make ze-verify`.**

This generalizes `ai/rules/plugin-self-containment.md` (the "delete the folder"
test) into a placement rule that code can audit.

## The Three Tiers

| Tier | Home | What it is | Examples |
|------|------|------------|----------|
| **core / infra** | `internal/core/` | A library you cannot "run as a plugin." Foundational; no config-driven lifecycle. | family, events, metrics, diagnostic, bufpool |
| **component** | `internal/component/` | A platform plugin: other plugins/components depend on it or plug into it. | bgp, iface, firewall, traffic, vpp |
| **plugin (edge)** | `internal/plugins/` | An edge plugin: a config-driven engine that nothing else depends on. | ntp, static, dhcpserver, l2tp-auth-* |

## The Two Axes

| Axis | Mechanical test |
|------|-----------------|
| **A. Is it a config-driven engine?** | does it call `sdk.NewWithConn(`? |
| **B. Does a feature depend on it?** | does any `.go` file under `internal/component/` or `internal/plugins/` (excluding its own subtree, the generated composition root, `cmd/ze` dispatch, `internal/core`, `internal/chaos`, `internal/test`, and `_test.go`) import it? |

Decision:

- **not an engine** → it is **core** infra. It belongs in `internal/core/`. (Today
  many libraries still sit under `internal/component/`; moving them is deferred —
  see "Scope of enforcement".)
- **engine + a feature depends on it** → **component** (`internal/component/`).
  It is a platform other plugins build on. BGP is the archetype: its sub-plugins
  and other code plug into it.
- **engine + nothing depends on it** → **edge plugin** (`internal/plugins/`).
  IS-IS, LDP, RSVP-TE are edge protocols: they consume services (iface, the RIB)
  but nothing consumes them.

The RIB stays **component** because edge protocols install routes through it.

## Authoring rule (read before creating a package)

Decide the tier by the two axes BEFORE you pick a directory:

1. Pure library, no `sdk.NewWithConn`, no plugin lifecycle → `internal/core/<x>`.
2. Engine that other plugins will depend on → `internal/component/<x>`.
3. Engine that is a self-contained leaf feature → `internal/plugins/<x>`.

A **sub-plugin of an existing subsystem** (e.g. a BGP capability or NLRI codec)
goes under that subsystem's own plugin namespace (`internal/component/bgp/plugins/<x>`),
not at the top level. Those nested namespaces are listed in the generator's
`pluginDirs` (`scripts/codegen/plugin_imports.go`).

## Scope of enforcement (Path C)

Only the **engine-placement** rule is enforced today, because it is the only part
that is fully mechanical:

> A config-driven engine (`sdk.NewWithConn`) at a top-level subsystem MUST be in
> `internal/component/` if a feature depends on it, else in `internal/plugins/`.

The **core vs component-vs-host** distinction for non-engine libraries is REPORTED
by `scripts/dev/dep_audit.py` (advisory) but NOT gated. The "is this wired as a
plugin?" half of that distinction is now mechanical: the advisory reads the
composition roots (the generated `all.go` plus `cmd/ze` dispatch) to tell a wired
plugin from a genuine core candidate, so every plugin shape -- `*-cmd` verb
providers, doctor checks, `RegisterBackend`, `RegisterRPCs` -- is recognized without
a per-mechanism registration grep that used to mis-send `*-cmd` dirs to core
(umbrella blocker B-1). What still blocks *enforcing* core/composition is structural,
not the plugin signal: BGP fuses a codec library with its engine (B-2) and
host-services need a tier decision (B-3) -- see `plan/spec-tiers-0-umbrella.md`
"Phase 5 Hardening Analysis". There is **no permanent allowlist**.

## Disable-ability (compile-out)

Axis B also decides whether a feature can be **compiled out** of the binary. A
feature is compile-out-able exactly when nothing always-compiled depends on it: it
is reached ONLY through build-tag-gated registration. A direct functional `import`
from always-on (untagged) code pins the package into every binary and defeats the
compile-out; only a blank/gated registration import can be dropped by a build tag.

Two shapes exist. **Listener services** (looking-glass: `ze_lg`, web:
`ze_web`, MCP: `ze_mcp`) plug into the construction registry
(`cmd/ze/hub/service_registry.go`): a gated `service_<x>.go` +
`register_<x>.go` registers a factory and any listener-migrator wiring the hub
iterates. MCP fits because its `MCPServerHandle` is already `Reconfigurable` +
`Shutdown`; the command metadata it shares with the always-on API is kept
neutral (`command_meta.go`) so API does not pull mcp back into every binary.
Web also has a nil-able standalone seam (`web_infra.go`) for
`ze start --web` so the always-on CLI path does not import `internal/component/web`.
**Dedicated seams** cover services whose construction shape does not fit the
registry: ssh (`ze_ssh`) uses `ssh_infra.go` for the shared startup and
standalone paths; gNMI (`ze_gnmi`) uses `gnmi_infra.go` for rich constructor
inputs plus the reload notification hook; the REST/gRPC API uses `api_infra.go`
with TWO independent hooks (`ze_rest` / `ze_grpc`) so an operator can ship
gRPC-without-REST or vice-versa, sharing an always-on engine/session builder
(`buildAPIShared`). The shared `api-server { token }` YANG base stays always-on
and each transport's `rest{}`/`grpc{}` container is contributed by a gated YANG
module (`api/rest/yang`, `api/grpc/yang`) via Ze's same-named-container merge, so
a compiled-out transport's config block is rejected as unknown. Telemetry
(`ze_telemetry`) is the first **core-level** seam: its hook
`metrics.StartExporter` lives in always-on `internal/core/metrics` (not the hub)
because two start sites in different components -- the hub standalone path and the
bgp reactor path (`internal/component/bgp/config`) -- both read it; the hub's
gated `register_telemetry.go` wires the gated `internal/component/telemetry/exporter`
(and its Netdata `collector` sidecar) into the seam. Only the Prometheus HTTP
exporter compiles out; the metric COLLECTION registry (`PrometheusRegistry`,
`Registry`, the `NopRegistry` dummy) stays always-on so the ~60 packages that
record `ze_*` metrics keep working with the exporter gated. Both shapes gate
their direct package and YANG schema imports into generated `all_ze_<feature>.go`
files via `plugin_imports.go`. `make ze` / `ze-appliance` pass the default-on feature tags
(`ZE_FEATURES` in the Makefile); `ze-stripped` omits them for a smaller, hardened
binary. See `plan/spec-feature-gate-0-umbrella.md`.

**Rule:** a compile-out-able feature (gated by `//go:build ze_<feature>`) MUST NOT
be directly imported by always-on code. Reach it through the construction
registry or a seam (`ssh_infra.go` / `gnmi_infra.go` style) in another gated file.
`dep_audit.py` enumerates these gated packages (`DISABLEABLE`); the gate flags
any always-on, non-test importer. Gates are declared in ONE place: `<tag> <pkg>`
lines in the repo-root `feature-gates.txt` manifest. A feature may reuse one tag
for sidecar packages that must vanish with it. `ZE_FEATURES` (Makefile),
`TestBuildTags()` (`internal/test/runner`), `featureTags` (`plugin_imports.go`),
and `DISABLEABLE` (`dep_audit.py`) all DERIVE from it; only `.golangci.yml`
build-tags is edited by hand (static YAML), and `dep_audit.py --check` fails on
its drift. Full procedure and the two registration shapes:
`ai/rules/feature-gate-registration.md`.

## The gate

`scripts/dev/dep_audit.py --check` enforces the engine-placement rule AND the
disable-ability rule, and runs in `make ze-verify` (target `ze-tier-check`). It:

- parses `pluginDirs` from `scripts/codegen/plugin_imports.go` to exclude nested
  sub-plugin namespaces (so `bgp/plugins/*` are never flagged);
- fails (exit 2) on any **new** misplaced engine, naming the dir and its required
  tier, pointing here;
- fails on a **stale** baseline entry (one no longer misplaced), forcing cleanup;
- fails (exit 2) if a `DISABLEABLE` feature is imported by always-on (untagged,
  non-test) code, naming the file and the build tag it needs.

### Migration baseline (transitional, NOT an allowlist)

`scripts/dev/tier_migration_baseline.txt` holds the engines that are currently in
the wrong tier and scheduled to move (child specs `spec-tiers-2`/`-3`). Each row is
annotated with the child spec that removes it. The gate fails on new violations and
on stale entries, so the file can only shrink. **An empty baseline = full
engine-placement enforcement with zero exceptions.** Regenerate after a move with
`scripts/dev/dep_audit.py --write-baseline`.

## Related

- `ai/rules/plugin-self-containment.md` — the delete-the-folder invariant.
- `ai/rules/plugin-design.md` — registration patterns, Proximity Principle.
- `scripts/dev/dep_audit.py` — the reverse-dependency report + the placement gate.
- `plan/spec-tiers-0-umbrella.md` — the taxonomy, the reorg plan, the hardening analysis.
