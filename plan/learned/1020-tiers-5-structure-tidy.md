# 1020 - tiers-5: non-engine tier taxonomy, B-2 extraction, domain clustering

## Context

The module-tier umbrella (spec-tiers-0-umbrella) established an enforced engine gate
(Path C) but left non-engine classification advisory-only. Three blockers remained:
B-1 (plugin discovery, resolved in 979), B-2 (BGP/iface/vpp/ike fuse library+engine
code so "imports bgp" can't distinguish codec use from engine dependence), and B-3
(framework/host packages don't fit the "not engine -> core" simplification). The
directory structure gave no signal about which packages form isolated domains.

## Decisions

- Resolved B-3 by expanding `ai/rules/architecture.md` with four non-engine
  categories (framework, host-service, domain-library, planned-violation) and a
  machine-readable manifest (`scripts/dev/tier_non_engine_categories.txt`) that
  `dep_audit.py --check` enforces. This replaces the advisory-only report with
  declared, reviewable human judgment. No hidden allowlist.

- Resolved B-2 partially: extracted 6 BGP library subpackages to `internal/core/bgp/`
  (wire, capability, events, context, nlri, attribute) in dependency order. Three
  subpackages stay in the engine: `message` (imports plugin/registry), `types` and
  `wireu` (deeply fused with reactor/RIB). Also extracted `iface/events` and
  `vpp/events` to core.

- Clustered BNG (l2tp + ppp + pppoe + pppoeclient + subscriber + 7 edge plugins
  under `component/l2tp/`) and VPN (ipsec under `component/ike/`). These are
  genuinely isolated domains: external dependents are only plugin registration + one
  web UI page. Did NOT cluster AAA (platform infra consumed by api, bgp, ssh, web),
  traffic, firewall, or CoS (global interface QoS behavior).

- Kept `pki` flat at `component/pki/` (categorized as framework, not domain-library)
  because it serves as shared certificate infrastructure, not VPN-only.

- Extended `migrate_module.py` for core and nested component-domain destinations
  before performing any moves (the existing tool only handled top-level
  component<->plugins relocations).

## Consequences

- `dep_audit.py --check` now enforces both engine placement (mechanical) AND
  non-engine categories (manifest-backed). 28 manifest rows classify every
  non-engine, non-registered package. A new unclassified placement fails the gate.

- `DOMAIN_LIBRARY_PREFIXES` in dep_audit.py is hardcoded to `internal/component/l2tp`
  and `internal/component/ike`. Adding a third domain cluster requires updating this
  tuple. This is the one place that doesn't derive from the manifest.

- The BGP extraction reduces false engine-dependence: importing
  `internal/core/bgp/attribute` is now mechanically distinguishable from importing
  the BGP engine. But `bgp/message` still fuses library use with component-tier
  wiring (its plugin/registry import), so complete separation needs a future
  decoupling spec.

- BNG plugin names dropped the `l2tp` prefix when nesting (`l2tpauthlocal` ->
  `authlocal`, `l2tppool` -> `pool`, `l2tpshaper` -> `shaper`). The nesting
  provides the context. BGP plugins use underscores (`filter_aspath`); BNG plugins
  use concatenated names (`authlocal`). This is a cosmetic inconsistency between the
  two namespaces, not a functional issue.

## Gotchas

- BGP library extraction is layered, not batch. The dependency graph means wire and
  capability must extract first (leaf), then context and nlri (one dep each), then
  attribute (two deps). Attempting a batch move breaks the build.

- `ike/dataplane` looks extractable from its name but its VPP backend imports
  `internal/component/vpp`. It stays in-place until an interface/backend split.

- `setup_features_*.go` files under `cmd/ze/` are registration importers but were
  not recognized as such until this work extended `is_registration_importer`. Without
  that fix, connect/local/provision/systemd appear as "shared libraries" in the
  advisory report.

## Files

- `ai/rules/architecture.md` (expanded with non-engine categories and clustering)
- `scripts/dev/dep_audit.py` (non-engine gate + setup_features recognition)
- `scripts/dev/tier_non_engine_categories.txt` (28-row manifest, created)
- `scripts/dev/migrate_module.py` (core + nested move support)
- `scripts/codegen/plugin_imports.go` (pluginDirs + nestedPluginDomains)
- `internal/core/bgp/{wire,capability,events,context,nlri,attribute}` (extracted)
- `internal/core/{iface/events,vpp/events,audit}` (extracted/moved)
- `internal/component/l2tp/{ppp,pppoe,pppoeclient,subscriber,plugins/*}` (clustered)
- `internal/component/ike/ipsec/` (clustered)
