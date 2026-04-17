# 621 -- backend-feature-gate

## Context

A user could write YANG config that their chosen backend could not implement: most visibly,
`interface { backend vpp; bridge ...; tunnel ...; wireguard ... }` was accepted by the parser,
the daemon started, and only at Apply time did `ifacevpp.CreateBridge` / `CreateTunnel` /
`CreateWireguardDevice` return `errNotSupported("<Backend method>")`. The error named a Go
symbol, not the user's YANG path, and the failure arrived minutes or reloads later depending
on the code path. The goal was to reject these configs at commit time with a diagnostic
naming the YANG path, the active backend, and the list of backends that DO implement the
feature -- covering both the daemon reload path and the offline `ze config validate` path,
with a single walker that future components (firewall fw-3, traffic fw-5) can adopt by
adding a `backend` leaf and a one-line call.

## Decisions

- **Chose a declarative YANG extension `ze:backend "<names>"` over a Go-side registry.**
  Annotation lives next to the feature it gates, schema reader mirrors `getOSExtension`
  at the same file, and `ze config validate` works without a running daemon or loaded
  backends because the walker reads only annotations and the parsed tree. A Go-registration
  approach would have required plugin init to run before validation could report anything.
- **Single generic walker in `internal/component/config/backend_gate.go` over per-component copies.**
  `ValidateBackendFeatures(tree, schema, componentRoot, activeBackend, backendLeafPath)` is
  component-agnostic; iface passes `componentRoot="interface"`, firewall will pass `"firewall"`,
  traffic `"traffic-control"`. No per-component walker boilerplate to drift out of sync.
- **Narrowest-annotation-wins via a recursive-return protocol.**
  `walkBackendNode` returns "did any descendant's own annotation accept the active backend?"
  and suppresses the current node's rejection when that's true. Implements AC-9's per-case
  override without special-casing choice/case entries. Default single-level annotation
  (today's iface surface) collapses to the simple "emit at annotated node" rule.
- **Kept ifacevpp's runtime `errNotSupported` returns as defence-in-depth, not removed.**
  If a future annotation gap lets a config through, the runtime check still fails the Apply
  rather than silently mis-configuring. The YANG annotations are the primary diagnostic; the
  runtime returns are the backstop.
- **Added backend-gate to `ze config validate`** (`cmd/ze/config/cmd_validate.go`) as a
  separate table-driven loop over gated components. Spec named this as an incidental benefit
  of the YANG-native approach; surfacing it in the CLI closes AC-8 without a daemon.
- **Firewall and traffic got `leaf backend` only; feature annotations deferred to fw-3 / fw-5.**
  Those components have no `OnConfigVerify` yet, so there is nothing to wire. Shipping the
  leaf now lets their walker call land as a one-liner when the plugins exist. Default values
  (`nft`, `tc`) mean no configuration churn for existing users.

## Consequences

- Every new backend-bearing feature now has a one-line declaration (`ze:backend "<names>";`)
  for its support matrix, consumed by commit-time validation and offline `ze config validate`
  uniformly.
- The firewall and traffic components can adopt the gate once they grow an
  `OnConfigVerify` callback (work owned by `spec-fw-8-lns-gaps.md` Gap 4 for firewall and
  `spec-fw-9-traffic-lifecycle.md` for traffic). The adoption is: (1) add `ze:backend`
  annotations to YANG nodes that vary per backend, (2) call
  `config.ValidateBackendFeaturesJSON` from the component's `OnConfigure` and
  `OnConfigVerify`, (3) add a row to the gated-components table in
  `cmd/ze/config/cmd_validate.go`. Walker needs no changes. The numbering names
  "fw-3" and "fw-5" that earlier drafts of this spec used are already taken by shipped
  work (traffic-netlink backend, CLI) and should not be used.
- `Backend []string` now lives on `LeafNode`, `ContainerNode`, `ListNode`. Code reading
  these nodes for other purposes must ignore the new field (default `nil` = unrestricted).
- The `ze:backend` name is now part of the YANG surface; its extension shape (argument
  = space-separated names) is a public contract once released.
- Consistency test (`TestBackendExtensionNames_AllAnnotationsNameKnownBackends`) holds a
  static allow-list of known backend names (`netlink, vpp, nft, tc`). New backends must be
  added to this list when they register. This is the mechanical check that catches typos
  like `ze:backend "netfilter"` at test time.

## Gotchas

- `ze config validate` did NOT initially run the gate: it calls the parser + YANG tree
  validator but not the plugin's `OnConfigVerify`. Missed this on the first pass; added a
  table-driven loop in `runValidation` to satisfy AC-8.
- Go's `for _, s := range strings.Fields(...)` now triggers `modernize` linter to prefer
  `strings.FieldsSeq` in test code. Used `FieldsSeq` in the consistency guard helper.
- `goyang`'s `Entry.Exts` holds extensions parsed from YANG, but for
  choice/case the extensions on a `case` statement do not propagate to the inner data
  nodes through `flattenChildren`. Annotations intended to override must be placed on the
  inner container, not on the `case` wrapper. The walker's narrowest-wins logic relies on
  finding the annotation on the data node that the config walks into.
- The walker receives a raw JSON `map[string]any` (matching `sdk.ConfigSection.Data`'s
  shape). It is NOT aware of the typed `ifaceConfig` struct. A sibling helper
  `ValidateBackendFeaturesJSON` parses JSON then delegates, keeping the JSON unmarshal out
  of every call site.
- Schema load is cached via `sync.Once` in `iface/register.go`. Tests that want a fresh
  schema after YANG changes must reset the cache or run in a new process; in practice the
  existing test suite builds its own synthetic schema and never touches the cached one.
- Pre-existing schemas from other components still appear in the built schema tree because
  they are registered via their own `init()` chains. The consistency test walks ALL loaded
  modules, not just iface's, so a typo in firewall or traffic YANG fails iface's test too.
  This is intentional coverage.

## Files

- `internal/component/config/yang/modules/ze-extensions.yang` — `extension backend`
- `internal/component/config/yang_schema.go` — `getBackendExtension`, Node population
- `internal/component/config/schema.go` — `Backend []string` on LeafNode/ContainerNode/ListNode
- `internal/component/config/backend_gate.go` — walker (new, ~230 lines)
- `internal/component/config/backend_gate_test.go` — 10 walker tests + consistency guard (new)
- `internal/component/config/yang_schema_test.go` — 6 reader tests
- `internal/component/iface/register.go` — `validateBackendGate` + `OnConfigure` / `OnConfigVerify` wiring
- `internal/component/iface/schema/ze-iface-conf.yang` — `ze:backend "netlink"` on bridge/tunnel/wireguard/veth/mirror
- `internal/component/iface/schema_test.go` — `TestIfaceYANGBackendAnnotations`
- `internal/component/firewall/schema/ze-firewall-conf.yang` — `leaf backend` default `nft`
- `internal/component/traffic/schema/ze-traffic-control-conf.yang` — `leaf backend` default `tc`
- `cmd/ze/config/cmd_validate.go` — backend-gate loop over gated components
- `test/parse/iface-vpp-rejects-bridge.ci`, `iface-vpp-rejects-tunnel.ci`, `iface-vpp-accepts-ethernet.ci`, `iface-netlink-accepts-bridge.ci`, `iface-vpp-aggregates-errors.ci`
- `docs/features.md`, `docs/guide/configuration.md`, `docs/guide/command-reference.md`, `docs/architecture/core-design.md`
