# Spec: config-yang-one-definer-per-node

| Field | Value |
|-------|-------|
| Status | design |
| Scope | config |
| Priority | P1 |
| Release reason | Shipped builds read a config node differently from what its YANG declares. With `ze_cos` built, the schema keeps the CoS module's copy of `interface ethernet`, `veth`, `dummy` and `bridge` and drops the interface module's list-level declarations (help, description, `ze:required`, `ze:suggest`, `unique`, related tools). The help of `system`, `environment`, `service`, `l2tp` and `interface` is the help of whichever module sorts first by name. The cause is the merge of a second definition, which has no guard, so the next shared leaf is replaced in silence too. This is a configuration defect in implemented capabilities (`plan/README.md`, `release/fixes/`, P1), not an absent feature. The defect is fixed first, in its own phase and commit (AC-1). The one-rule conversion that removes the cause follows in the same spec, because it is the same mechanism and the same release category, so `plan/README.md`, "Split fixes from features", calls for no split |
| RFC gaps | - |
| Depends | Moved here from `spec-plugin-external-yang-at-startup` (its D-6 and AC-44) by the owner's "ok" of 2026-11-17. Related: `spec-config-yang-definition-order` (landed) decides the print order of augmented and merged children (`internal/component/config/yang_schema_order.go`, `definitionOrder` and `augmentsInOrder`) |
| Phase | - |
| Handoff | - |
| Design tier | development tier |
| Implementation tier | development tier |
| Updated | 2026-11-26 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Owner decisions (2026-11-24)

| Topic | Decision |
|-------|----------|
| O-1..O-4 (one definer per YANG node across built-in modules) | Enforce one owner per YANG node everywhere (~26 built-in modules switch to `augment`), create always-built `ze-service-conf` (`internal/component/config/service/yang/ze-service-conf.yang`) to own `service`, assign `l2tp/auth` ownership to `ze-l2tp-conf`, and give each shared root clean whole-root `?` help text. |

## Task

**The owner's rule (Thomas, 2026-11-17):** "The only differene should be how they reach the bus". An external plugin and a compiled-in plugin are the same thing to Ze. They differ only in transport, and a rule for external plugins alone must follow from the transport or from arriving after the compiled-in set.

That rule meets this difference. An external `-conf` module that defines a top-level config node another module already defines is refused at the join (`refuseRedefinedRoots`, `config/yang/join.go`). A compiled-in module that does the same is merged in silence by `Schema.Define` (`config/schema.go`). The supervising session recommended refusing a redefinition on both sides. The owner answered "It seems you have all the answers". The implementing agent then found five shared roots, not the one (`service`) the recommendation assumed, and stopped. Shown the five roots, the roughly 26 modules involved and the CoS-over-interface defect, with the recommendation "its own spec, written now as a design, with the enumeration and the interface defect as its first acceptance criterion; the external side keeps refusing a redefinition, which arriving after the compiled-in set justifies", the owner answered: "ok". On 2026-11-24 Thomas confirmed all four design choices (O-1 through O-4): enforce one owner per node everywhere via `augment`, create an always-built `ze-service-conf` module beside `ze-system-conf` to own `service`, place `container auth` in `ze-l2tp-conf`, and give each shared root clean whole-root `?` help text.

Goal: a top-level config node, and every node below it, is defined by exactly one module. Any other module adds to it with `augment`. A second definition in the compiled-in set is refused with the two module names and the `augment` to write. It is never merged and never replaced. Every config the repository carries validates and parses to the same `Tree.ToMap()` as before, and shared roots print their children in `definitionOrder` (owner statements first, then augmented children in augmenting-module name order).

The external join keeps refusing a redefinition. That rule is an arrival rule: an external module arrives after the compiled-in set is fixed. After this spec, both sides are judged by one function.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/config/yang-config-design.md` - module loading, the join's refusals, leafref cross-module wording (`Extensions: ze-extensions.yang`, line 182), the paragraph that says compiled-in modules may share a container by a second definition (`Joining a module that no init() registered`, lines 584–587), and the section claiming plugins attach leaves via same-named containers without importing the owner (`Several modules can declare one node, and the '?' box carries them all`, lines 1843–1871)
  → Constraint: `schemaFromResolved` takes top-level nodes only from modules whose name ends in `-conf`. An augment-only module produces no top-level node, because goyang attaches its augments to the target module's entry tree.
  → Decision: all three sections are updated in AC-8 to the single one-definer-per-node `augment` rule and a table of each shared root and its owner.
- [ ] `docs/architecture/config/syntax.md` - "Child order" table
  → Constraint: `definitionOrder` (`internal/component/config/yang_schema_order.go`) places a block's own statements first, then each augmenting module's statements in `augmentsInOrder` order.
  → Decision: delete the superseded row "Contributed by a second `-conf` module to a top-level container both define", because every cross-module contribution uses `augment`.
- [ ] `docs/architecture/aaa-tacacs.md` - "The schema merge defect this work uncovered"
  → Constraint: that section documents recursive `mergeContainer` and `mergeNode` and anchors `internal/component/config/schema.go -- Define, mergeContainer, mergeNode`.
  → Decision: update the section and its anchor to state that `ze-authz-conf` augments `/sys:system`, `ze-tacacs-conf` augments `/sys:system/authz:authentication`, and `Schema.Define` refuses a second definition.
- [ ] `ai/doors/config-yang.md` - Boundaries
  → Constraint: "Extend a container with `augment`, never with a second definition". The door says that a redefinition replaces the container. The code merges containers (`mergeContainer`) and replaces only the other pairings.
  → Decision: update the door to say a second definition is refused on both sides, never merged and never replaced.
- [ ] `ai/doors/system.md` - a compile tag is a boundary
  → Constraint: always-on code must not import a gated package. An augment imports its target's module, and the generated `register.go` blank-imports the package that registers it. So the target's package must be built in every build that builds the augmenter.
- [ ] `docs/plugin-development/schema.md` - plugin schema authoring guide (currently shows only a standalone root `acme-monitor` and has no `augment` example or shared-root owner table; the actual in-repo caller importing `ze-dns-resolver-conf` to `augment "/dnsresolver:service"` is `internal/test/externalyang/externalyang.go`, `augmentText`)
  → Decision: add an `augment` example and shared-root owner table naming `ze-service-conf` (prefix `svc`) and the other owners in AC-8, and update `internal/test/externalyang/externalyang.go` (`augmentText`) to import `ze-service-conf` and `augment "/svc:service"`.

**Key insights:**
- Five roots are shared and three nested nodes are shared (Enumeration, below). The interface lists are the one place where a list, its key leaf and its list-level declarations are merged, and that merge drops the interface module's declarations whenever CoS is built.
- `ze-bgp-conf` already reaches `environment` by `augment "/hub:environment"`, and `ze-irr-conf` already reaches `system` by `augment "/sys:system"`. The design follows those two modules.
- In goyang, two modules that each define `system` produce two top-level nodes in two namespaces. Ze merges them by local name. So `ze-ssh-conf`'s `augment "/authz:system/..."` and `ze-tacacs-conf`'s leafref `/authz:system/...` name the authz module's own `system`. Both paths change when authz augments `/sys:system`.
- After `spec-config-yang-definition-order` landed, `definitionOrder` (`internal/component/config/yang_schema_order.go`) orders every block by the owner's own statements first, then `augmentsInOrder` (by augmenting module name). Moving ownership from the first-alphabetical module to the real owner therefore reorders `ze config fmt` output on the four shared roots whose owner does not sort first alphabetically (`interface`, `environment`, `system`, `service`), while keeping parsed trees (`ToMap()`) identical.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/config/schema.go` - `Schema.Define`, `mergeContainer`, `mergeNode` (pairing table below)
- [ ] `internal/component/config/yang_schema.go` - `schemaFromResolved` sorts the module names, then calls `Define` for each top-level node of each `-conf` module. `PluginOnlySchema` and `LoadYANGModule` also call `Define`.
- [ ] `internal/component/config/yang_schema_order.go` - `definitionOrder` and `augmentsInOrder` order a block's children by the defining block's own statements first, then each augment sorted by augmenting module name, source name and statement position.
- [ ] `internal/component/config/yang/join.go` - `refuseRedefinedRoots` collects the top-level nodes of the compiled-in `-conf` modules, keeps the first holder of each, does not judge the compiled-in modules against each other, and refuses an external module's top-level node that is already held, naming the holder and the `augment`.
- [ ] `internal/component/aihelp/aihelp.go` - `Services()` walks each `-conf` module's own `environment` entry in module-name order, emitting three separate `api-server` entries (one each from `ze-api-conf` with `token`, `ze-grpc-conf` with `grpc`, and `ze-rest-conf` with `rest`).
- [ ] `internal/component/api/yang/ze-api-conf.yang`, `internal/component/api/grpc/yang/ze-grpc-conf.yang`, `internal/component/api/rest/yang/ze-rest-conf.yang` - three modules each defining `environment/api-server`.
- [ ] `internal/test/externalyang/externalyang.go` - `augmentText` (lines 139–169, used by `test/plugin/external-yang-augment-shared-root.ci` and `validate_corpus_test.go`) imports `ze-dns-resolver-conf` and writes `augment "/dnsresolver:service"`.
- [ ] `internal/plugins/cos/yang/ze-cos-conf.yang`, `internal/component/iface/yang/ze-iface-conf.yang` - the CoS module defines `container interface` with the lists `ethernet`, `veth`, `dummy` and `bridge`, each keyed by its own `leaf name` (type string, CoS help) and omitting `unique "mac/address"` and `unique "mac/match"`. Its comment says "The canonical node is owned by ze-iface-conf.yang".
- [ ] `internal/component/config/system/yang/ze-system-conf.yang`, `internal/component/hub/yang/ze-hub-conf.yang`, `internal/component/l2tp/yang/ze-l2tp-conf.yang`, `internal/component/l2tp/plugins/authlocal/yang/ze-l2tp-auth-local-conf.yang`, `internal/component/l2tp/plugins/authradius/yang/ze-l2tp-auth-radius-conf.yang` - current root definitions, `ze:help` texts, and duplicated `l2tp/auth` container.
- [ ] `internal/component/web/fragment.go`, `internal/component/web/handler_config_entry.go`, `internal/component/web/handler_tools.go`, `internal/component/config/leaf_value.go` - readers of `ListNode.Unique`, `ShortHelp`, `Required`, `Suggest`, `Related`, and `KeyLeaf`.
- [ ] `internal/component/gnmi/path.go`, `internal/component/gnmi/capabilities.go` - gNMI path translation uses local `PathElem.Name` against `config.Schema` without module origins; `Capabilities` lists `loader.ModuleNames()`.
- [ ] `feature-gates.txt` - `ze_cos`, `ze_l2tp`, `ze_radius` (authradius is emitted under `ze_l2tp && ze_radius`), `ze_tacacs`, `ze_ssh`, `ze_web`, `ze_gnmi`, `ze_mcp`, `ze_rest`, `ze_grpc`, `ze_lg`, `ze_ntp`, `ze_bmp`, `ze_as112`, `ze_geodns`, `ze_dhcpserver`, `ze_pxe`. Hub, config/system, iface, api and authz carry no gate.

What `Define` does for each pairing (the module visited first is the first in name order):

| Existing | New | Result |
|----------|-----|--------|
| none | any node | Added |
| container | container | `mergeContainer`. The new module's children are appended. A child both define is passed to `mergeNode`. The container's own fields (help, description, presence and the rest) stay the first module's |
| container | leaf, list, or other | The new node replaces the container in silence |
| leaf, list, or other | any node | The new node replaces the existing one in silence |

What `mergeNode` does below the top level:

| Existing | New | Result |
|----------|-----|--------|
| container | container | Recurses. An empty `Backend` adopts the new one's; `ReplaceWhole` ORs in |
| list | list | The existing list node is kept, its children are merged, and only `Backend` and `ReplaceWhole` are adopted. Every other list-level field of the second module is dropped |
| leaf, or mixed kinds | any node | The first module's node is kept. The second is dropped in silence |

**Behavior to preserve:**
- Every config node keeps its path, its kind and its value type. Every config the repository carries validates and parses to the same `Tree.ToMap()` as before (AC-4).
- A plugin's `ConfigRoots` are hand-declared strings (`register.go` of each plugin), so no plugin's auto-load changes.
- The external join's operator text keeps naming the plugin, the node, the holder and the `augment` (`redefinedRootRefusal`).
- gNMI path translation and non-shared root formatting remain unchanged.

**Behavior to change:**
- A second definition in the compiled-in set is refused (AC-2, AC-5).
- The CoS module adds to the interface module's lists by `augment` and does not redefine them (AC-1).
- The help of a shared root is its owner's clean whole-root text (AC-6).
- `ze config fmt` on the four shared roots whose owner does not sort first alphabetically (`interface`, `environment`, `system`, `service`) prints the owner's own statements first in definition order followed by augmented children in `augmentsInOrder` order (AC-4).
- `ze help ai` (`aihelp.Services()`) collapses the three duplicate `api-server` entries (`ze-api-conf`, `ze-grpc-conf`, `ze-rest-conf`) into one unified `api-server` entry carrying `ze-api-conf`'s help and combined children `grpc`, `rest`, and `token`, while every other environment service stays unchanged (AC-9).

## Enumeration (2026-11-17)

Source: a brace-aware read-only scan of every `*-conf.yang` under `internal`, `pkg` and `cmd` (test data excluded), cross-checked by reading the modules. Content that enters through `uses` of a grouping is not seen by the scan. AC-2's test is the complete census.

| Root | Definers (module `ze-<x>-conf`) | Owner | Owner always built when an augmenter is? |
|------|-------------------------------|-------|-------------------------------------------|
| `environment` | 13: api, bmp, gnmi, grpc, hub, l2tp, lg, mcp, mtu, ntp, rest, ssh, web | `ze-hub-conf`, which `ze-bgp-conf` already augments | Yes, hub carries no gate |
| `system` | 4: authz, radius, system, tacacs | `ze-system-conf`, which `ze-irr-conf` already augments | Yes, config/system carries no gate |
| `interface` | 2: cos, iface | `ze-iface-conf` | Yes, iface carries no gate |
| `l2tp` | 5: l2tp, l2tp-auth-local, l2tp-auth-radius, l2tp-pool, l2tp-shaper | `ze-l2tp-conf` | Yes: the four plugins build only with `ze_l2tp` |
| `service` | 6: as112, dhcp-server, dns-resolver, geodns, image-server, tftp-server | `ze-service-conf` (`internal/component/config/service/yang/ze-service-conf.yang`, prefix `svc`, namespace `urn:ze:service:conf`), always built beside `ze-system-conf` | Yes, always built with no feature gate |

| Nested shared node | Definers | Owner |
|--------------------|----------|-------|
| `environment/api-server` | api, grpc, rest | `ze-api-conf` (no gate). `ze-grpc-conf` and `ze-rest-conf` augment `/hub:environment/api:api-server` |
| `system/authentication` | authz, radius, tacacs | `ze-authz-conf` (no gate), which augments `/sys:system`. `ze-radius-conf` and `ze-tacacs-conf` augment `/sys:system/authz:authentication` |
| `l2tp/auth` | l2tp-auth-local, l2tp-auth-radius | `ze-l2tp-conf` gains `container auth`. `ze-l2tp-auth-local-conf` and `ze-l2tp-auth-radius-conf` augment `/l2tp:l2tp/l2tp:auth` |
| `interface/ethernet`, `veth`, `dummy`, `bridge` (lists, key leaf `name`) | cos, iface | `ze-iface-conf`. `ze-cos-conf` augments each list and its `unit` list |

**The CoS-over-interface defect** (confirmed from code). `schemaFromResolved` visits `ze-cos-conf` before `ze-iface-conf`. So in every build with `ze_cos`, `interface ethernet`, `veth`, `dummy` and `bridge` are the CoS module's list nodes and carry the CoS key leaf `name`. The interface module's list-level `ShortHelp`, `Description`, `Required`, `Suggest`, `Unique`, `Related`, `DisplayKey` and `Listener` are dropped, and only `Backend` and `ReplaceWhole` are adopted. In particular, `ze-iface-conf` declares `unique "mac/address"` and `unique "mac/match"` on `ethernet` and `unique "mac/address"` on `veth` and `bridge`; `internal/component/web/fragment.go` (`collectUniqueFields`) reads `ListNode.Unique` to build the web editor's `ListTable`, so the CoS build drops the list table on those three lists. `nodeDescription` (`fragment.go`) renders CoS's `"Interface-level class-of-service profile bindings."` for `interface` in the web sidebar (and `internal/le/site/testdata/published-configuration.md` published that CoS string as the `interface` root help), and `internal/component/config/leaf_value.go` validates keys against CoS's `KeyLeaf` rather than iface's. The same first-in-name-order rule gives `system` authz's help, `environment` the API module's help, and `service` as112's help. The completer joins every declaration's help on its own (`mergeHelpExts`, journal `silent-fall-through.md` 2026-09-04, fixed), so it is not one of them.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| YANG `augment` across modules | `ze-bgp-conf.yang` (`augment "/hub:environment"`), `ze-irr-conf.yang` (`augment "/sys:system"`), `ze-vrrp-conf.yang` (augments the iface lists) | Adds nodes to another module's tree. goyang attaches them to the target's entry, and the schema reads them there | Extends: every non-owner definer moves to it |
| Definition-order child placement | `internal/component/config/yang_schema_order.go` `definitionOrder`, `augmentsInOrder` | Places a block's own statements first, then each augmenting module's statements sorted by augmenting module name, source name and statement position | Applies unchanged: governs child order on every converted shared root without `mergeContainer` |
| The join's redefinition refusal | `internal/component/config/yang/join.go` `refuseRedefinedRoots`, `redefinedRootRefusal` | Refuses an external `-conf` top-level node that is already held | Extends: becomes the one judge of both compiled-in and external sets. The compiled-in set calls it with no plugin, and the external text is kept |
| The schema merge | `internal/component/config/schema.go` `Schema.Define`, `mergeContainer`, `mergeNode` | Merges same-named containers and lists, and replaces or drops anything else | Replaces: `Define` refuses a second definition. `mergeContainer` and `mergeNode` have no remaining callers (A-4 confirmed) and are deleted |
| Census of the shipped set | `internal/component/config/yang` `TestNoShippedModuleAddsNothing`, `internal/component/config/cli` `TestThisBinaryLinksEveryModuleItsSchemaImports` | Hold the compiled-in set to a join rule; resolve the schema of the config CLI binary | Extends: AC-2's census test follows `TestNoShippedModuleAddsNothing` |
| Config sweep | `internal/component/config/cli/validate_corpus_test.go` `TestRepositoryConfigsValidate` | Validates every Ze config in the repository | Extends: AC-4 pairs it with `Tree.ToMap()` equality and definition-order child checks on the four reordered shared roots |
| Completer help join | `internal/component/cli/completer.go` `mergeAugmentedEntries`, `mergeHelpExts` | Joins the help of every module that declares a node | Does not apply: with one definer, it has one entry to join. It stays for nodes several augments reach |
| Generated YANG glue | `./le yang glue update` | Writes each `yang/` package's `embed.go` and `register.go`, which blank-imports the packages that register a module's imports | Applies: every converted module and new `ze-service-conf` regenerate their glue through it |

## Design documents

| Document | Section | Derived from | Binding statement | Verdict | How this design stands against it |
|----------|---------|--------------|-------------------|---------|-----------------------------------|
| `ai/doors/system.md` | The core knows nothing, and everything registers | the door every change reads; the door covering internal/component/aihelp/aihelp.go | "Always-on code MUST NOT import a gated package." | agrees | Every owner of a shared root augmented by another module (`ze-hub-conf`, `ze-system-conf`, `ze-iface-conf`, new `ze-service-conf`, `ze-api-conf`, `ze-authz-conf`) is always built, and `ze-l2tp-conf` shares the `ze_l2tp` gate of its four augmenters, so no always-on code imports any gated package. |
| `docs/architecture/config/syntax.md` | Child order | the // Design: page of internal/component/config/schema.go; anchored to internal/component/config/schema.go; the // Design: page of internal/component/config/yang_schema.go; anchored to internal/component/config/yang_schema.go | "Contributed by a second `-conf` module to a top-level container both define" | departs | Removing compiled-in top-level container merging eliminates the case where two `-conf` modules define one top-level container. Every cross-module contribution now uses `augment` and follows the `augment` row (`definitionOrder` places the owner's own statements first, then `augmentsInOrder`). AC-8 deletes the superseded row from `syntax.md`. |
| `docs/architecture/aaa-tacacs.md` | The schema merge defect this work uncovered | anchored to internal/component/config/schema.go | "The fix is a recursive `mergeContainer` and `mergeNode`, and it protects any future module that extends a shared container." | departs | This spec deletes `mergeContainer` and `mergeNode` and makes `Schema.Define` refuse a second definition. `ze-authz-conf` augments `/sys:system` and `ze-tacacs-conf` augments `/sys:system/authz:authentication`, eliminating the merge path completely. AC-8 updates this section and its source anchor. |
| `docs/architecture/config/yang-config-design.md` | Joining a module that no `init()` registered | anchored to internal/component/config/schema.go; anchored to internal/component/config/yang_schema.go; the // Design: page of internal/component/config/yang/join.go; anchored to internal/component/config/yang/join.go; anchored to internal/component/aihelp/aihelp.go | "The redefinition refusal judges external modules only. The compiled-in modules may share a top-level container by a second definition, which `Schema.Define` merges: six shipped modules each define `container service`." | departs | By Thomas's 2026-11-17 and 2026-11-24 rulings, one redefinition refusal (`refuseRedefinedRoots`) judges both compiled-in and external `-conf` modules. Every shared root has one owner (`ze-service-conf` for `service`) and every other module uses `augment`. AC-8 rewrites this paragraph and lists the shared root owners. |
| `docs/architecture/config/yang-config-design.md` | Several modules can declare one node, and the `?` box carries them all | anchored to internal/component/config/schema.go; anchored to internal/component/config/yang_schema.go; the // Design: page of internal/component/config/yang/join.go | "A plugin attaches its leaves to a shared node by declaring a container of the same name in its own module. It does not import the module that owns the node. Removing the plugin then removes its leaves and nothing else, which is what plugin self-containment requires. `class-of-service` reaches `interface` that way." | departs | Under one-definer-per-node, plugins never attach leaves by redeclaring a shared container or list; they import the always-built (or same-gated) owner module and use `augment`, so `ze-cos-conf` augments `ze-iface-conf`'s lists instead of redeclaring `interface`, and each shared node has one owner whose `ze:help` and `description` apply (removing lines 1865–1869's rationale that nothing in the schema says which module owns a shared node). AC-8 updates this section. |
| `docs/architecture/config/yang-config-design.md` | Extensions: `ze-extensions.yang` | anchored to internal/component/config/schema.go; anchored to internal/component/config/yang_schema.go; the // Design: page of internal/component/config/yang/join.go | "A section Ze merges across `-conf` modules into one config data node is no exception: a path into another module's section writes that module's prefix, as in `/zb:sec/zb:y`." | departs | Ze no longer merges sections across `-conf` modules; cross-module extensions use `augment` against the single owning module, so leafref paths address the owner's prefix and each augmented child's prefix (for example `/sys:system/authz:authorization/...`). AC-8 updates this sentence in line 182. |
| `docs/architecture/ssh/fixit-bcrypt-hash-credential.md` | Decisions | anchored to internal/component/config/schema.go | "" | silent | Governs local-only bcrypt hash authentication and secret display masking via `Schema.LookupTokenPath`. This spec changes neither `LookupTokenPath` nor credential masking. |
| `docs/architecture/web-interface.md` | YANG Schema Integration | anchored to internal/component/config/schema.go | "The sidebar heading keeps the summary" | agrees | Removing CoS's duplicate `interface` container and lists restores `ze-iface-conf`'s `ShortHelp`, `Description`, and `Unique` constraints on `ContainerNode` and `ListNode`, fixing web sidebar summaries and list tables. |
| `ai/doors/config-yang.md` | Boundaries | the door covering internal/component/config/schema.go; the door covering internal/component/config/yang_schema.go; the door covering internal/component/config/yang/join.go; the door covering internal/component/config/service/yang/ze-service-conf.yang | "**Extend a container with `augment`, never with a second definition.** A redefinition replaces it" | departs | The door already forbids a second definition, but states that a redefinition replaces the container. Under this spec a second definition is refused with an operator error on both compiled-in and external sets, never merged and never replaced. AC-8 updates this sentence. |
| `docs/architecture/core-design.md` | 14a. Commit-Time Backend Capability Gate | anchored to internal/component/config/yang_schema.go | "`getBackendExtension` in `yang_schema.go` mirrors `getOSExtension`; stores the de-duplicated name list on the schema `Node` (`LeafNode.Backend`, `ContainerNode.Backend`, `ListNode.Backend`)." | agrees | `getBackendExtension` and `Backend` storage in `yang_schema.go` stay intact, and removing the CoS-over-iface list merge lets `ze-iface-conf`'s own `ze:backend "kernel"` annotations on `veth` and `bridge` flow straight to `ListNode.Backend` without `mergeNode` adoption. |
| `docs/architecture/hub-architecture.md` | 2. Plugins Define Their Own Schema | anchored to internal/component/config/yang_schema.go | "a command the daemon serves is answered from the set the daemon joined when it started." | agrees | `PublishExternalModules` and the boot-time join flow stay unchanged; `refuseRedefinedRoots` judges external modules at join time with the same function that judges compiled-in modules. |
| `docs/architecture/plugin/plugin-system.md` | One plugin, two ways to reach the bus | anchored to internal/component/config/yang/join.go | "A behaviour that differs between the two kinds with no reason of this kind is a defect. It is fixed, not documented as a limit." | agrees | Refusing a redefined top-level node for external plugins while silently merging compiled-in modules was an unjustified divergence; this spec removes it so one judge enforces one definer per node on both sides. |
| `docs/features/ai-first.md` | Self-Describing Command Reference | the // Design: page of internal/component/aihelp/aihelp.go | "The command and RPC listings are assembled from the binary's plugin registry, YANG schemas and command registrations." | agrees | `aihelp.Services` continues deriving environment services dynamically from the resolved YANG schema, updated to read `ze-hub-conf`'s `environment` entry where all augmented services attach, collapsing today's three duplicate `api-server` entries (`ze-api-conf`, `ze-grpc-conf`, `ze-rest-conf`) into one unified `api-server` entry (`grpc`, `rest`, `token`) while keeping every other environment service unchanged (AC-9). |
| `docs/plugin-development/schema.md` | Registering a Schema | the // Design: page of internal/test/externalyang/externalyang.go | "a module whose name ends in `-conf` adds config roots, one that augments another module extends it" | agrees | Extending a shared config root through `augment` is the rule this spec enforces for compiled-in and external modules alike; AC-8 adds an `augment` example and shared-root owner table naming `ze-service-conf` (`svc`) and the other owners, and `internal/test/externalyang/externalyang.go` (`augmentText`) updates to `augment "/svc:service"`. |
| `ai/doors/le-tooling.md` | Where it sits | the door covering internal/test/externalyang/externalyang.go; the door covering test/plugin/external-yang-augment-shared-root.ci | "Functional `.ci` files live under `test/<suite>/` and run through `./le test <suite>`" | agrees | Updating `internal/test/externalyang/externalyang.go` (`augmentText`) to import `ze-service-conf` (`augment "/svc:service"`) and updating the comment in `test/plugin/external-yang-augment-shared-root.ci` keeps the functional test verifying the live external-plugin augment path against the real `service` owner. |
| `docs/architecture/api/commands.md` | Which surface renders which field | anchored to internal/component/aihelp/aihelp.go | "" | silent | Governs CLI/API command help rendering, RPC metadata, and pipe operators; this spec touches only config `-conf` schema ownership and `aihelp.Services`, leaving command and RPC rendering untouched. |
| `docs/architecture/cli/command-verbs.md` | Read verbs and their guarantee | anchored to internal/component/aihelp/aihelp.go | "" | silent | Governs verb-first CLI command routing and read-only authorization predicates; this spec adds or alters no CLI verbs or authorization rules. |
| `docs/architecture/core-design.md` | Design decisions | anchored to internal/component/config/yang_schema.go | "A commit fails when it configures a feature that programs only the Linux kernel, or that reads only what the Linux kernel sees of the traffic, while the interfaces run on VPP. Two host-side uses stay accepted, so that mixed setups remain possible once VRFs are added: a `fib kernel` block beside a `fib vpp` block, which programs the Linux route table for the router's own traffic, and `sysctl`, which sets the Linux kernel's own tunables. A `fib kernel` block with no `fib vpp` block beside it is refused, for now." | agrees | Converting shared YANG nodes to `augment` keeps every node's path and `ze:backend` annotation intact, preserving commit-time VPP/kernel mismatch refusal unchanged. |
| `docs/architecture/core-design.md` | Design decisions | anchored to internal/component/config/yang_schema.go | "A configuration that holds such a mismatch is refused at daemon start as well as at commit, reload and `ze config validate`, by one verdict that is the same at each." | agrees | This spec does not alter the data-plane mismatch evaluator or how commit, reload, and `ze config validate` invoke it (as modified for daemon start by the 2026-11-26 decision below). |
| `docs/architecture/core-design.md` | Design decisions | anchored to internal/component/config/yang_schema.go | "The VPP firewall verifier, not a YANG annotation, decides whether `connection-state` and `mark-set` are accepted on VPP. The `ze:backend` annotation of `flow-offload` is deleted with theirs; `flow-offload` stays refused on VPP because VPP has no flowtable and the `flowtable` list keeps its `ze:backend "kernel"` annotation." | agrees | Firewall YANG annotations and the VPP firewall verifier are untouched by this spec. |
| `docs/architecture/core-design.md` | Design decisions | anchored to internal/component/config/yang_schema.go | "Every data-plane selector names the plane it programs, `kernel` or `vpp`: `interface backend`, `firewall backend` and `traffic control backend` take `kernel` where they took `netlink`, `nft` and `tc`, and each defaults to `kernel`; the `fib` blocks already name `kernel` and `vpp`. The plane a registration declares and the value a `ze:backend` annotation names are the same word. A second implementation on the same plane, such as eBPF, takes a value of its own when it exists. In Thomas's words (voice-transcribed): "should tc and netfilter be renamed to kernel? so we have vpp and kernel?", then "please do it"." | agrees | All data-plane selector leaves and `ze:backend` annotations keep their `kernel`/`vpp` names unchanged; removing CoS's duplicate interface lists preserves `ze-iface-conf`'s `ze:backend "kernel"` directly. |
| `docs/architecture/core-design.md` | Design decisions | anchored to internal/component/config/yang_schema.go | "BGP egress next-hop rewriting belongs exclusively in the export filter chain, not under session configuration (`session { next-hop }`). Ze automatically appends `next-hop auto` at the end of the export chain when no earlier export filter has already set a next-hop outcome (`self`, `unchanged`, or an explicit address), so ordinary eBGP rewrites to self while iBGP and route-server clients preserve the received next hop. In Thomas's words (voice-transcribed): "I would think that all the rewrite should be in the prefix export and not under configuration. I believe we should automatically add to the export prefix list the auto at the end of the list if not defined with something else. So if no one, for example, used a self or unchanged, we should automatically change it on eBGP. Because otherwise we have two ways to modify packets on the way out to the peer.", then "Unify into the export chain it is the way I want it."" | agrees | BGP policy and export-chain next-hop rewriting are untouched by this spec; `ze-bgp-conf` already augments `/hub:environment`. |
| `docs/architecture/core-design.md` | Design decisions | anchored to internal/component/config/yang_schema.go | "Egress export chains configure next-hop rewriting through one underlying `policychain.Sets` next-hop intent exposed in two authoring forms: `policy { modify <name> { set { next-hop self \| unchanged \| auto \| <ip>; } } }` for matched/conditional rules, and built-in chain references `next-hop:self`, `next-hop:unchanged`, and `next-hop:auto` for peer-wide policy without a wrapper `modify` block. Selected by Thomas on 2026-11-23 ("Option A: Extend policy modify (self\|unchanged\|auto\|<ip>) AND provide built-in next-hop:self / next-hop:unchanged / next-hop:auto chain refs")." | agrees | Policy modify leaves and built-in chain references are untouched by this spec. |
| `docs/architecture/core-design.md` | Design decisions | anchored to internal/component/config/yang_schema.go | "Built-in `next-hop:auto`, `next-hop:self`, and `next-hop:unchanged` are BGP engine steps compiled into `policychain.Chain` (parallel to `loop-detection`), never external plugin RPCs or a `pkg/plugin/rpc.FilterInsertion` tail extension. Config loading appends `next-hop:auto` at the tail of every export chain unless an unconditional `next-hop:self`, `next-hop:unchanged`, or `next-hop:auto` is already present, leaving `Policy.Configured()` false for unconfigured chains; operators override via `next-hop:unchanged`, and `inactive:next-hop:auto` is refused. Selected by Thomas on 2026-11-23 ("Option A: Built-in BGP engine step appended at config compile time (like loop-detection; zero RPC; override via next-hop:unchanged, no inactive:next-hop:auto)")." | agrees | BGP export-chain compilation and built-in engine steps are untouched by this spec. |
| `docs/architecture/core-design.md` | Design decisions | anchored to internal/component/config/yang_schema.go | "Within an export chain, explicit next-hop sets (`self`, `unchanged`, `auto`, or an IP address) use standard last-write-wins folding while the auto-appended trailing `next-hop:auto` acts only when next-hop is still unset. For a whole-UPDATE raw override (`wireOverride`), Ze compares each emitted next-hop field against the received UPDATE (matching `forward_med.go` provenance): trailing `auto` is skipped only when the raw override changed the next-hop bytes or an earlier chain member already selected `unchanged`/`self`/explicit. Selected by Thomas on 2026-11-23 ("Option A: Provenance check on raw overrides (like MED): skip trailing auto only when the raw override changed the next-hop bytes or an earlier member set unchanged/self")." | agrees | BGP export-chain next-hop folding and raw-override provenance are untouched by this spec. |
| `docs/architecture/core-design.md` | Design decisions | anchored to internal/component/config/yang_schema.go | "Explicit next-hop addresses in `policy { modify <name> { set { next-hop <ip>; } } }` and `policychain.Sets.NextHop` support both IPv4 (attr 3) and IPv6/MP_REACH (through `NextHopProfile` and `ValidatePeerGlobalNextHop`) on both export and import chains. Egress-only next-hop modes (`self`, `unchanged`, `auto`), whether written as built-in `next-hop:*` refs or inside `policy { modify }`, are refused on import chains at config load. Selected by Thomas on 2026-11-23 ("Option A: Support IPv4 + IPv6/MP_REACH for explicit next-hop on both export and import; refuse self/unchanged/auto on import at config load")." | agrees | Policy modify next-hop validation for IPv4/IPv6 import and export chains is untouched by this spec. |
| `docs/architecture/core-design.md` | Design decisions | anchored to internal/component/config/yang_schema.go | "Auto-appended trailing `next-hop:auto` applies only to forwarded routes and preserves the originator-chosen next hop on locally originated routes (static config routes, ExaBGP `announce`, API announcements). Explicit export-chain next-hop actions (`next-hop:self`, `modify { set { next-hop ... } }`) apply uniformly to both forwarded and locally originated routes. Selected by Thomas on 2026-11-23 ("Option A: Trailing next-hop:auto applies to forwarded routes only (preserves static/ExaBGP/API explicit next-hop); explicit next-hop:self / modify next-hop applies to both forwarded and originated routes")." | agrees | BGP route origination and export next-hop behavior are untouched by this spec. |
| `docs/architecture/core-design.md` | Design decisions | anchored to internal/component/config/yang_schema.go | "Every automatic filter-chain member Ze inserts — both built-in BGP engine steps (`next-hop:auto` at the export tail, `loop-detection` at the import head) and plugin-declared `FilterInsertions` — is published through one queryable implied-filter seam carrying direction, position, reference and `implied-by` reason so `\| implied` displays every extra chain member alongside the operator's written filter list. In Thomas's words (2026-11-23): "Can you make sure that \| implied show the extra chain members when it displays the filter ?"." | agrees | The implied-filter seam and `\| implied` rendering are untouched by this spec. |
| `docs/architecture/core-design.md` | Design decisions | anchored to internal/component/config/yang_schema.go | "Ze always starts. The configuration of one service never makes `ze start` exit or refuse to start: a service whose configuration cannot be served is not started, ze runs with its other services, and the service is reported with its fix by an error line in the log under a doctor code and by `ze doctor` under the same code. Examples are a web certificate name that does not resolve, a looking-glass `server` entry with no certificate to present, and a looking-glass address list that does not parse. Where a service has several endpoints, only the endpoint that cannot be served stays down and the others serve. A reload whose config cannot be served stays refused as a whole, and the running services keep what they serve. In Thomas's words (voice-transcribed): "we should never not start ze as if we can not bring networking we can not fix things like web cert and the like so the only good answer is to disable the service badly configured and not start it"; asked about a reload, "reload stay refused"; asked "Should only that endpoint stay down while the others serve", "yes". The reason is that a daemon that does not start brings up no networking, so the operator cannot reach the box to correct it." | agrees | The compiled-in duplicate-definition refusal is a build-time schema integrity guard verified by AC-2's census test over the shipped binary, never a runtime refusal triggered by an operator's service configuration; service startup degradation stays untouched. |
| `docs/architecture/core-design.md` | Design decisions | anchored to internal/component/config/yang_schema.go | "The rule of 2026-11-23 above, that a configuration which holds a feature that programs only the Linux kernel while the interfaces run on VPP "is refused at daemon start", is superseded for the start: ze starts, that feature stays off, and `ze doctor` reports the mismatch with its fix. A commit, a reload and `ze config validate` still refuse such a configuration, by the same verdict as before. Thomas was asked whether the rule that ze always starts replaces the refusal at start, "(a) yes, ze starts, that feature stays off and ze doctor reports the mismatch, while commit and reload still refuse it; (b) no", and answered "2a" (voice-transcribed). The reason is the one of the row above: a daemon that does not start brings up no networking, so the operator cannot reach the box to correct the mismatch." | agrees | Startup handling of data-plane mismatches and commit/reload/validate refusal semantics are untouched by this spec; removing CoS's duplicate list definitions ensures `ze-iface-conf`'s `ze:backend "kernel"` annotations remain accurate for those checks. |
| `docs/architecture/plugin/plugin-system.md` | Design decisions | anchored to internal/component/config/yang/join.go | "With no daemon running, a command that parses a config starts each declared external program in query mode, never its run loop, and validates against its declaration" | agrees | Offline external-plugin declaration querying and joining stay unchanged; `refuseRedefinedRoots` judges the joined external modules with the same function that judges compiled-in modules. |
| `docs/architecture/api/commands.md` | Design decisions | anchored to internal/component/aihelp/aihelp.go | "The pipe wins: a command path names the subject and stops; scope, presentation and format are pipe operations after it, selector words included" | agrees | This spec changes no command path or pipe behavior; `aihelp.Services` collapses the three duplicate `api-server` entries into one unified entry (AC-9) while leaving all command paths and pipes untouched. |
| `docs/architecture/api/commands.md` | Design decisions | anchored to internal/component/aihelp/aihelp.go | "A command may override any generic pipe operation that acts on its data, as long as the answer keeps the generic's data format: every data operator of the catalog is overridable, each held to its generic's answer shape; a format, paging, `save` and `log` are the data format or its display, and stay the generic's. In Thomas's words: "I believe it should be possible to overwrite more command as long as their data format is not changed."" | agrees | Pipe operator overrides and command answer shapes are untouched by this spec. |
| `docs/architecture/api/commands.md` | Design decisions | anchored to internal/component/aihelp/aihelp.go | "A pipe operation works on the answer of the command before it. It may narrow the answer, extend it, transform it, or deliver it somewhere, such as saving it to a file. It never changes what the command does" | agrees | Pipe operator semantics are untouched by this spec. |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- The compiled-in module set at `yang.DefaultLoader`, and an external module at `yang.Join`.
- The operator's config text, whose grammar does not change.

### Transformation Path
1. `Loader.Resolve` resolves the modules. A converted module's augment attaches its nodes to the owner's entry tree.
2. `schemaFromResolved` walks the `-conf` modules. Each top-level node now has one definer. A second definition is refused through the shared redefinition check (`refuseRedefinedRoots`), and `Schema.Define` refuses any duplicate root.
3. `definitionOrder` (`internal/component/config/yang_schema_order.go`) orders each block's children by the owner's own statements first, then each augmenting module's statements in `augmentsInOrder` order.
4. The parser, the validator, the editor, the web UI and the printers read the schema as before.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| External plugin to the join | Its `-conf` module augments a shared root with the owner's prefix (`svc` for `ze-service-conf`, `hub` for `ze-hub-conf`, `sys` for `ze-system-conf`, `iface` for `ze-iface-conf`, `l2tp` for `ze-l2tp-conf`) | No: `docs/plugin-development/schema.md` names the owner after AC-8 |
| Build tags | Each augment's target package is built whenever its augmenter is | No: AC-7 |

### Changed-Wire Census

**Search evidence:** scope `internal`, `pkg`, `cmd`, `test`, `docs` (YANG, Go and `.ci`). Terms: `augment "/<p>:(environment|system|interface|service|l2tp)` and `path "/<p>:(...)` for every non-owner prefix; `ze-dns-resolver-conf` and `dnsresolver:service`; `GetEntry("ze-<module>-conf")` for each definer; `Dir["environment|system|service|interface|l2tp"]`; `LoadYANGModule(`; `\.Define(`; `PathElem` / `Origin` in `internal/component/gnmi/`. Positive control: `ze-ssh-conf.yang`'s `augment "/authz:system/authz:authentication/authz:user"` was found by the first term. Findings: one augment (ssh) and one leafref (tacacs) through authz's own `system`; one in-repo external-plugin fixture importing `ze-dns-resolver-conf` to augment `/dnsresolver:service` (`internal/test/externalyang/externalyang.go`, `augmentText`, lines 139–169, described in `test/plugin/external-yang-augment-shared-root.ci`); one product reader of a module's own root (`aihelp.go`); no module-origin usage in `internal/component/gnmi/`; the test readers listed below. The config file grammar does not change, so an operator's config is not a changed wire.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| Schema path of authz's `system` | writer | `ze-authz-conf.yang` top-level `container system` | Scan: authz defines `system` and `system/authentication` | Change: augment `/sys:system`. The nodes keep their paths |
| Schema path of authz's `system` | reader | `ze-ssh-conf.yang` `augment "/authz:system/authz:authentication/authz:user"` | First search term | Change: `/sys:system/authz:authentication/authz:user` |
| Schema path of authz's `system` | reader | `ze-tacacs-conf.yang` leafref `/authz:system/authz:authorization/authz:profile/authz:name` | Second search term | Change: `/sys:system/authz:authorization/authz:profile/authz:name` |
| Each module's own `environment` entry | writer | The `container environment` of each of the 13 definers (`ze-api-conf.yang` ... `ze-web-conf.yang`), turned into that module's entry tree by `Loader.Resolve` and answered by `Resolved.GetEntry(<module>)` (`config/yang/loader.go`) | Scan of the `-conf` modules (Enumeration) | Change: only `ze-hub-conf.yang` writes `environment`. The 12 others write `augment "/hub:environment"`, which `Resolve` attaches to the hub module's entry, so `GetEntry` of those modules no longer holds an `environment` |
| Each module's own `environment` entry | reader | `internal/component/aihelp/aihelp.go` `Services()` | `Dir["environment"]` search | Change: walk `ze-hub-conf`'s `environment` entry, which holds every augmented service. Collapses today's three duplicate `api-server` entries (`ze-api-conf`, `ze-grpc-conf`, `ze-rest-conf`) into one unified `api-server` entry (`grpc`, `rest`, `token`), while keeping every other environment service unchanged (AC-9) |
| Each module's own root entry | writer | The top-level container of each definer in the Enumeration tables, through `Loader.Resolve` into `Resolved.GetEntry(<module>)` | Same scan | Change: a non-owner module's entry holds no top-level node of a shared root. Its nodes are at the owner's entry |
| Each module's own root entry | reader (tests) | `authz`, `l2tp`, `ssh`, `gnmi` `yang/schema_test.go` | `GetEntry` search | Proposed for the judge (Test changes needing the judge) |
| Each module's own root entry | reader | `schemaFromResolved` (`internal/component/config/yang_schema.go`), which walks every `-conf` module's entry and calls `Schema.Define` for each top-level node | Read at the function | Change: it meets each root once, from its owner. A second meeting is the AC-5 refusal |
| A shared root an external module augments | writer | The owner module's top-level container: today `ze-dns-resolver-conf.yang`'s `container service` (and the other shared-root definers); after this spec, always-built `ze-service-conf` (`internal/component/config/service/yang/ze-service-conf.yang`, prefix `svc`) and the other owners of the Enumeration table | `ze-dns-resolver-conf` search; the owner modules | Change: the owner's module name and prefix are what an external augment imports (`ze-service-conf` / `svc` for `service`). AC-8 documents them in `docs/plugin-development/schema.md` |
| A shared root an external module augments | reader (in-repo test fixture) | `internal/test/externalyang/externalyang.go` (`augmentText`, lines 139–169, used by `test/plugin/external-yang-augment-shared-root.ci` and `validate_corpus_test.go`) | `ze-dns-resolver-conf` / `dnsresolver:service` search | Change: replace `import ze-dns-resolver-conf { prefix dnsresolver; }` and `augment "/dnsresolver:service"` with `import ze-service-conf { prefix svc; }` and `augment "/svc:service"`, and update the comment in `test/plugin/external-yang-augment-shared-root.ci` |
| A shared root an external module augments | reader (outside the repository) | An external plugin's `-conf` module, resolved in `yang.Join` (`internal/component/config/yang/join.go`, `resolveJoined`) | `yang.Join` and `docs/plugin-development/schema.md` | Change: AC-8 adds an `augment` example and shared-root owner table naming `ze-service-conf` (`svc`) and the other owners to `docs/plugin-development/schema.md`. Pre-release, so no compatibility is owed |
| The ordered children of a shared root | writer, today | `mergeContainer` (`internal/component/config/schema.go`): visits `-conf` modules in module-name order and appends a later module's children after the first-alphabetical module's | Read at the function | Replaced: `mergeContainer` is deleted; no schema caller remains |
| The ordered children of a shared root | writer, after | `definitionOrder` and `augmentsInOrder` (`internal/component/config/yang_schema_order.go`): places the owner module's own statements first, then each augmenting module's statements in augmenting-module name order | Read at the functions | Change (expected under `spec-config-yang-definition-order`): on `interface`, `environment`, `system`, and `service`, the owner does not sort first alphabetically, so the owner's own children print before earlier-sorting augmenters and `service` sits at `ze-service-conf`'s top-level slot (AC-4) |
| The ordered children of a shared root | reader | every printer (`Serialize`, set form, web) | `ContainerNode.Children()` order | Preserve parsed tree (`ToMap()`) on every config and byte-identical `ze config fmt` on non-shared roots; assert expected `definitionOrder` child position on the four reordered shared roots (AC-4) |

### Integration Points
- `refuseRedefinedRoots` - the shared check for both sets, called by `schemaFromResolved` for the compiled-in set and by `Join` for external modules.
- `redefinedRootRefusal` - keeps the external text. The compiled-in text names the two modules, the node, and the `augment` on its last line.
- `internal/component/config/service/yang` - new always-built YANG package defining `module ze-service-conf` (`container service`), registered via `./le yang glue update` and blank-imported in `internal/component/plugin/all/all.go` via `./le repo update`.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | The schema still comes from goyang's resolved entries through `schemaFromResolved` and `definitionOrder` |
| No unintended coupling (components stay isolated) | Yes | An augment imports only an always-built owner (`ze-hub-conf`, `ze-system-conf`, `ze-iface-conf`, `ze-service-conf`, `ze-api-conf`, `ze-authz-conf`), or an owner under the same gate (`ze-l2tp-conf` under `ze_l2tp`) |
| No duplicated functionality (extends existing, does not recreate) | Yes | One redefinition check for both sets, extending `refuseRedefinedRoots` |
| Zero-copy preserved where applicable (refs, not copies) | N-A | Schema build, no wire path |
| Registration over hardcoding, outbound | Yes | Modules keep registering from `init()`. No list of shared roots is written anywhere: the check finds them |
| Registration over hardcoding, inbound | Yes | Searched `refuseRedefinedRoots`, `schemaFromResolved` and `aihelp.go` for root names: none names a hardcoded list of shared roots, and the new check names none |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Operator-facing surfaces render the schema's list-level fields of `interface ethernet`, `veth`, `dummy`, and `bridge` (`Unique`, `ShortHelp`, `Description`, `Required`, `Suggest`, `Related`, `KeyLeaf`), so the CoS defect reaches an operator | `ListNode` fields in `internal/component/config/schema.go`; `ze-iface-conf.yang` declares `unique "mac/address"` and `unique "mac/match"` on `ethernet` and `unique "mac/address"` on `veth` and `bridge` while `ze-cos-conf.yang` omits them | The defect shows only in the schema value, and AC-1 stays a schema assertion | `git grep` of `*config.ListNode` readers (2026-11-26): `internal/component/web/fragment.go` `collectUniqueFields` reads `ListNode.Unique` to render `ListTable` (dropped under `ze_cos`), `nodeDescription` reads `ContainerNode.ShortHelp` / `ListNode.ShortHelp` for web sidebar tooltips, `handler_config_entry.go` reads `Required`/`Suggest`, `handler_tools.go` reads `Related`, and `internal/component/config/leaf_value.go` validates keys against `KeyLeaf` | confirmed |
| A-2 | Converting a definition to an augment keeps each child's path, kind and value type | goyang attaches augmented nodes to the target entry; `yangToNode` reads the entry | A config stops parsing or changes type | AC-3 and AC-4 at implementation | unvalidated |
| A-3 | The print order of a shared root's children does not change | Originally assumed against the deleted `yangChildOrder` helper before `spec-config-yang-definition-order` landed | `ze config fmt` reorders children on shared roots | Read `internal/component/config/yang_schema_order.go` (`definitionOrder`, `augmentsInOrder`, 2026-11-26): `yangChildOrder` is gone; `definitionOrder` places the owner's own statements first and augments after by augmenting-module name, so moving ownership from the first-alphabetical module to the real owner intentionally reorders children on `interface`, `environment`, `system` and moves top-level `service` to `ze-service-conf`'s slot. AC-4 updated to `ToMap()` equality plus expected `definitionOrder` child placement on those four roots | broken |
| A-4 | No product caller relies on `Schema.Define` merging, apart from `schemaFromResolved` | `Define` callers in `internal/component/config/yang_schema.go` | A caller breaks on the refusal | `git grep -n '\.Define(' -- internal/ pkg/ cmd/` excluding `*_test.go` (2026-11-26): only 4 non-test call sites exist — `yang_schema.go:57` (`PluginOnlySchema`, fresh schema with distinct `FirstPassRoots()`), `yang_schema.go:384` (`schemaFromResolved`), `yang_schema.go:1372` (`LoadYANGModule`, one module's top level), and `internal/test/perfrunner/run.go:990` (fresh schema, single `"fib"` root). None relies on merging | confirmed |
| A-5 | No gNMI or API path carries the module name of a shared root | gNMI serves the config tree | A gNMI client path changes | Read `internal/component/gnmi/path.go` (`pathToSegments`, `segmentsToPath`) and `internal/component/gnmi/capabilities.go` (`loadModels`) (2026-11-26): paths are translated strictly by local `PathElem.Name` against `config.Schema` with no `Path.Origin` or module qualification; `Capabilities` simply enumerates `loader.ModuleNames()` | confirmed |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | An augment's target is missing from a build that compiles the augmenter, so resolve fails at start | `DefaultLoader` error naming an unresolved import | Owners are untagged or share the augmenter's gate (Enumeration). AC-7 resolves the set with no feature tag and with the full feature set |
| R-2 | A converted child changes kind or type | AC-3 red | Revert that module's conversion and fix the augment |
| R-3 | The help text an operator reads or a site golden snapshot changes | AC-6 or site testdata diff | Intended per O-3; AC-6 specifies the exact whole-root `ze:help` texts, and any golden snapshot that captured CoS's broken `interface` help is regenerated |
| R-4 | Another session edits a converted module | Conflict in the module | Convert one root per commit |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A config section is refused at start, or a node loses its type. AC-3 and AC-4 catch both before commit |
| How is it reverted? | One commit per root. Each reverts alone |
| Who else touches this path? | `spec-config-yang-definition-order` (landed: `yang_schema_order.go`); `spec-plugin-external-yang-at-startup` (the join) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| A build with `ze_cos` loads the compiled-in schema | → | `schemaFromResolved` over the converted CoS module | `TestCosAddsToTheInterfaceListsItDoesNotRedefine` |
| `DefaultLoader` and `YANGSchema` at start | → | `schemaFromResolved` calling the shared redefinition check | `TestCompiledInSetHasOneDefinerPerConfigNode` |
| A test module registered with `RegisterModuleForTest` that defines `service` again | → | the shared check refusing in `schemaFromResolved` | `TestSchemaRefusesASecondDefinitionNamingBothModules` |
| An external plugin's module that defines `service` | → | `Join`, through the same check | `TestJoinRefusesTopLevelRedefinition` (existing, unchanged) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | A build with `ze_cos` builds the compiled-in schema and renders the web config view for `interface`, `interface ethernet`, `veth`, `dummy`, and `bridge` | `interface ethernet`, `veth`, `dummy`, and `bridge` carry `ze-iface-conf`'s list-level declarations and key leaf (`ShortHelp`, `Description`, `Required`, `Suggest`, `Unique`, `Related`, `DisplayKey`, `Listener`, `Backend`, `KeyLeaf`) plus the CoS leaves under them; `collectUniqueFields` in `internal/component/web/fragment.go` returns `mac/address` and `mac/match` on `ethernet` and `mac/address` on `veth` and `bridge` (restoring the web `ListTable`); `nodeDescription` returns iface's `"Network interface configuration."` for `interface`. The test is written first and is red today | The CoS leaves (`class-of-service`) stay under the lists and their `unit` lists: CoS adds them by `augment` |
| AC-2 | The compiled-in set at `DefaultLoader`, in the binary that links every module | Every config node is defined by exactly one module. The test goes red when a module defines a node another module defines. It is written red over today's set and turns green when the last root is converted | A node another module reaches by `augment` is one definition: the augmenter defines no node of the same name |
| AC-3 | The five shared subtrees, before and after | Each node at the same path, with the same kind and value type. The expected list is written from the YANG read before the conversion | Help and description of a shared root change only as AC-6 says |
| AC-4 | Every config the repository carries | `TestRepositoryConfigsValidate` passes; every config parses to the exact same `Tree.ToMap()` before and after; on the four shared roots whose owner does not sort first alphabetically (`interface`, `environment`, `system`, `service`), `ze config fmt` prints the owner's own statements first in definition order followed by augmented children in augmenting-module name order (`definitionOrder`/`augmentsInOrder`, matching `docs/architecture/config/syntax.md` "Child order"); every non-shared root's `ze config fmt` output stays byte-identical | Reordering of children inside `interface`, `environment`, `system` and of top-level `service` to `ze-service-conf`'s alphabetical slot is expected under `definitionOrder` once the real owner replaces the first-alphabetical merger; any semantic tree (`ToMap()`) difference is a defect |
| AC-5 | A compiled-in module defines a node another module defines | The schema build fails with an `opererr` operator error that names both modules, the node and the `augment` that fixes it on its last line. Nothing is merged or replaced | A node reached by `augment` |
| AC-6 | `?` on `system`, `environment`, `interface`, `l2tp`, `service` (and `l2tp auth`) | Shows each owner's clean whole-root `ze:help` text: `system` (`ze-system-conf`): `"System-wide settings for identity, access control, DNS, and host management."`; `environment` (`ze-hub-conf`): `"Daemon runtime, logging, CLI, and management service environment settings."`; `interface` (`ze-iface-conf`): `"Network interface configuration."`; `l2tp` (`ze-l2tp-conf`): `"L2TPv2 tunnel and subscriber settings (RFC 2661)."` (with `l2tp auth`: `"PPP authentication for L2TP subscriber sessions."`); `service` (`ze-service-conf`): `"Network services hosted by Ze."` | Augmented children keep their own help |
| AC-7 | The compiled-in set resolved with no feature tag, and with the full feature set | Both answer a `Resolved` and a schema, and the schema of each holds every root its linked modules define, with one definer each (AC-2) | None: an augment whose target is missing in some build is the start failure R-1 names |
| AC-8 | The architecture, door, and plugin-development pages | `ai/doors/config-yang.md` says a second definition is refused on both sides, never merged or replaced. `docs/architecture/config/yang-config-design.md` updates all three affected sections: (1) `Joining a module that no \`init()\` registered` replaces "The redefinition refusal judges external modules only" with the one rule and a table of each shared root and its owner; (2) `Several modules can declare one node, and the \`?\` box carries them all` (lines 1843–1871) replaces the same-named-container / `class-of-service` merge explanation and "nothing in the schema says which module OWNS a shared node" rationale with the one-definer-per-node `augment` model; and (3) `Extensions: \`ze-extensions.yang\`` (line 182) removes the claim that Ze merges sections across `-conf` modules. `docs/architecture/config/syntax.md` removes the superseded "Contributed by a second `-conf` module to a top-level container both define" row from `Child order`. `docs/architecture/aaa-tacacs.md` updates `The schema merge defect this work uncovered` and its source anchor from deleted `mergeContainer`/`mergeNode` to `augment` + refusal. `docs/plugin-development/schema.md` adds an `augment` example and shared-root owner table naming `ze-service-conf` (prefix `svc`) and the other shared-root owners. `./le doc check all` passes over all five | The rest of each page is unchanged. The join's refusals table keeps its redefinition row, because the external refusal stays |
| AC-9 | `ze help ai` services list (`aihelp.Services()`) | Collapses today's three duplicate `api-server` entries (`ze-api-conf`, `ze-grpc-conf`, `ze-rest-conf`) into one unified `api-server` entry carrying `ze-api-conf`'s `ShortHelp` and combined children (`grpc`, `rest`, `token`), while every other environment service keeps the exact same name, order, leaves, and help as before the conversion | Removing the two duplicate `api-server` rows is the intended consequence of unifying `environment/api-server` under `ze-api-conf`; dropping any other service or any child of `api-server` is a defect |
| AC-10 | An external module that defines `service` | Refused at the join as today, with the operator text naming the plugin, `ze-service-conf`, and `augment "/svc:service"`, by the same function (`refuseRedefinedRoots`) that judges the compiled-in set | An external module that augments `/svc:service` is joined |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | An operator with a CoS build opens `interface ethernet` in the web editor or types `interface ?` | schema (`ze-iface-conf` + `ze-cos-conf` augment) → `fragment.go` `collectUniqueFields`/`nodeDescription` and completer | `TestCosAddsToTheInterfaceListsItDoesNotRedefine` (including `Unique` / web `ListTable` and sidebar `ShortHelp` checks) |
| 2 | An operator loads an existing config with `service`, `system`, `environment`, `l2tp` and `interface` sections | file → parser → tree (`ToMap()`) | `TestRepositoryConfigsValidate`; the functional suites listed below |
| 3 | A developer adds a module that redefines `service` | build → `schemaFromResolved` → shared `refuseRedefinedRoots` refusal | `TestSchemaRefusesASecondDefinitionNamingBothModules` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestCosAddsToTheInterfaceListsItDoesNotRedefine` | `internal/plugins/cos/yang/schema_test.go` (`ze_cos`) | AC-1: red today; proves iface list-level fields (`Unique`, `KeyLeaf`, `ShortHelp`, `Description`, `Backend`) and `interface` root help survive with `ze_cos` linked | |
| `TestCompiledInSetHasOneDefinerPerConfigNode` | `internal/component/config/cli/` (the binary that links every module) | AC-2, AC-7 | |
| `TestSharedRootsKeepEveryChildPathAndType` | `internal/component/config/cli/` | AC-3 | |
| `TestRepositoryConfigsKeepParsedTreeAndDefinitionOrder` | `internal/component/config/cli/` | AC-4 (`ToMap()` equality across corpus + `definitionOrder` child position on `interface`, `environment`, `system`, `service`) | |
| `TestSchemaRefusesASecondDefinitionNamingBothModules` | `internal/component/config/yang_schema_test.go` | AC-5 | |
| `TestSharedRootHelpIsTheOwners` | `internal/component/config/cli/` | AC-6 (exact whole-root `ze:help` texts) | |
| `TestAIHelpListsEveryEnvironmentService` | `internal/component/aihelp/` | AC-9 | |
| `TestJoinRefusesTopLevelRedefinition` | `internal/component/config/yang/join_test.go` (existing) | AC-10, unchanged | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N-A | No numeric input | - | - | - |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| The `.ci` files configuring `service {` | `test/parse`, `test/plugin`, `test/reload`, `test/dhcp`, `test/install`, `test/pppoe`, `test/ui` (grep of 2026-11-17: 79 files) | Each config still loads and behaves as before | |
| The `.ci` files configuring `class-of-service` | `test/parse`, `test/plugin`, `test/reload` (17 files) | CoS bindings on interfaces still load and apply, with iface's list declarations in force | |
| The `.ci` files configuring `l2tp {` | `test/l2tp`, `test/parse`, `test/plugin`, `test/pppoe`, `test/reload`, `test/ui` (86 files) | L2TP with local and RADIUS auth still loads | |
| `config-validate-agrees-with-boot.ci` | `test/plugin/` | Offline validation and boot agree over the converted schema | |

### Interop Tests (Scope: protocol)
N-A: no wire protocol changes.

## Files to Modify
- `internal/component/config/schema.go` - `Define` refuses a second definition. `mergeContainer` and `mergeNode` have no callers left and are deleted.
- `internal/component/config/yang_schema.go` - `schemaFromResolved` calls the shared redefinition check (`refuseRedefinedRoots`). Its merge comment and `PluginOnlySchema`'s are rewritten.
- `internal/component/config/yang/join.go` - `refuseRedefinedRoots` judges both compiled-in and external sets.
- `internal/component/aihelp/aihelp.go` - walks `ze-hub-conf`'s `environment` entry (unifying the three `api-server` rows into one entry with `grpc`, `rest`, `token`).
- `internal/test/externalyang/externalyang.go` - `augmentText` switches from `import ze-dns-resolver-conf { prefix dnsresolver; }` / `augment "/dnsresolver:service"` to `import ze-service-conf { prefix svc; }` / `augment "/svc:service"`.
- `test/plugin/external-yang-augment-shared-root.ci` - comment updated from `ze-dns-resolver-conf` / `/dnsresolver:service` to `ze-service-conf` / `/svc:service`.
- The YANG modules of the Enumeration tables, each non-owner converted to `augment` (and glue regenerated via `./le yang glue update`): cos; api, bmp, gnmi, grpc, l2tp, lg, mcp, mtu, ntp, rest, ssh, web (`environment`); authz, radius, tacacs (`system`, plus ssh augment and tacacs leafref updated from `/authz:system/...` to `/sys:system/...`); l2tp-auth-local, l2tp-auth-radius, l2tp-pool, l2tp-shaper (`l2tp`); as112, dhcp-server, dns-resolver, geodns, image-server, tftp-server (`service`); `ze-l2tp-conf` gains `container auth`; `ze-system-conf`, `ze-hub-conf`, `ze-iface-conf`, `ze-l2tp-conf` carry the whole-root `ze:help` texts of AC-6.
- Docs: `ai/doors/config-yang.md`, `docs/architecture/config/yang-config-design.md`, `docs/architecture/config/syntax.md`, `docs/architecture/aaa-tacacs.md`, `docs/plugin-development/schema.md`.

## Files to Create
- `internal/component/config/service/yang/ze-service-conf.yang` - always-built module `ze-service-conf` (namespace `urn:ze:service:conf`, prefix `svc`) owning `container service` beside `internal/component/config/system/yang/ze-system-conf.yang`.
- `internal/component/config/service/yang/doc.go`, `internal/component/config/service/yang/embed.go`, `internal/component/config/service/yang/register.go` - package doc and generated glue from `./le yang glue update` (with composition root updated via `./le repo update`).
- The test files of the Unit Tests table.

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | New `ze-service-conf` and converted modules. No operator-facing config node is added except the owning `service` and `l2tp/auth` containers |
| YANG validation constraints | No | Unchanged: each leaf keeps its type and constraints |
| YANG custom validators | No | Unchanged |
| CLI commands/flags | No | None added |
| CLI grammar (keyword before value) | No | Config grammar unchanged |
| Editor autocomplete | Yes | Help of shared roots (AC-6) |
| Functional test for new RPC/API | N-A | No RPC |
| Pipe completeness | N-A | No command |
| Env var registration | No | `environment` leaves keep their paths and env keys |
| Doctor check for runtime dependencies | N-A | No runtime dependency |
| Doctor code per problem class | N-A | The refusal is a build-time schema error. The default set cannot ship with it, because AC-2 refuses it |
| Prometheus counters/metrics | N-A | None |
| BGP family surface (new SAFI / capability / attribute) | N-A | None |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | Correctness fix |
| 2 | Config syntax changed? | Yes | `docs/architecture/config/syntax.md`: remove superseded second-`-conf`-definer row in "Child order" |
| 3 | CLI command added/changed? | No | - |
| 4 | API/RPC added/changed? | No | - |
| 5 | Plugin added/changed? | Yes | `docs/plugin-development/schema.md`: name `ze-service-conf` (`svc`) and the other root owners |
| 6 | Has a user guide page? | No | - |
| 7 | Wire format changed? | No | - |
| 8 | Plugin SDK/protocol changed? | No | - |
| 9 | RFC behavior implemented, changed, or newly proven? | No | - |
| 10 | Test infrastructure changed? | No | - |
| 11 | Affects daemon comparison? | No | - |
| 12 | Internal architecture changed? | Yes | `docs/architecture/config/yang-config-design.md`, `docs/architecture/aaa-tacacs.md`, `ai/doors/config-yang.md` |
| 13 | Route metadata keys added/changed? | No | - |
| 14 | Prometheus counters added/changed? | No | - |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | - |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Derived by `./le spec citation anchors spec plan/release/fixes/spec-config-yang-one-definer-per-node.md`: `docs/architecture/aaa-tacacs.md` (`Define, mergeContainer, mergeNode`), `docs/architecture/config/syntax.md` (`schemaFromResolved`), `docs/architecture/config/yang-config-design.md`, and `docs/plugin-development/schema.md` are updated in AC-8. Advisory mentions checked and unaffected: `docs/features/mcp-integration.md` and `docs/guide/mcp/overview.md` (mention `aihelp.go` for general MCP `ze_reference` overview, not per-service layout), `docs/guide/web-interface.md` (mentions `schema.go` for general web editor UI, which AC-1 restores), and `docs/plugin-development/commands.md` (mentions `join.go` for command/RPC conflicts, untouched here) |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/plugin-development/schema.md` gains an `augment` example under `service` and a shared-root owner table naming `ze-service-conf` (`svc`) |

## Design presentation

### What changes

Every configuration node in Ze is defined by exactly one YANG module, and every other module that adds children under it uses YANG `augment` instead of defining a second copy of the container or list. On both the compiled-in module set and external plugins' modules, one shared function (`refuseRedefinedRoots`) refuses any second definition naming both modules, the node, and the `augment` statement to write; `Schema.Define` refuses duplicate roots and deletes the silent `mergeContainer`/`mergeNode` merge machinery. An always-built `ze-service-conf` module at `internal/component/config/service/yang/ze-service-conf.yang` (prefix `svc`) owns `container service`, `ze-l2tp-conf` owns `container auth`, and each of the five shared roots (`system`, `environment`, `interface`, `l2tp`, `service`) carries one clean whole-root `ze:help` summary.

### Before and after

1. **CoS build on `interface ethernet`, `veth`, `dummy`, `bridge`:**
   - Before: `ze-cos-conf` sorts before `ze-iface-conf`, so `Schema.Define` keeps CoS's list nodes and drops `ze-iface-conf`'s `unique "mac/address"` / `unique "mac/match"` constraints, `KeyLeaf`, `ShortHelp`, and `Description`. The web config editor renders no `ListTable` for `interface ethernet`, `veth`, or `bridge`, and displays `"Interface-level class-of-service profile bindings."` for `interface` (also leaked into `published-configuration.md`).
   - After: `ze-cos-conf` augments `ze-iface-conf`'s lists and `unit` lists. `interface` keeps `"Network interface configuration."`, and `ethernet`, `veth`, `dummy`, and `bridge` retain all of `ze-iface-conf`'s list-level declarations, key leaves, and web `ListTable` columns alongside the CoS leaves.

2. **`?` help on shared config roots:**
   - Before: whichever module sorted first alphabetically supplied the root's help (`ze-authz-conf` for `system`, `ze-api-conf` for `environment`, `ze-cos-conf` for `interface`, `ze-as112-conf` for `service`; and `ze-hub-conf` itself still read `"ZeBGP environment settings"`).
   - After: each shared root displays its owner's clean whole-root `ze:help` string specified in AC-6 (`system`, `environment`, `interface`, `l2tp`, `service`).

3. **A second module defining an existing top-level config root:**
   - Before: an external plugin redefining `service` was refused by `refuseRedefinedRoots`, whereas a compiled-in module doing the same was silently merged by `Schema.Define` (`mergeContainer`/`mergeNode`), hiding defects like CoS-over-iface.
   - After: both compiled-in and external modules are refused by `refuseRedefinedRoots` with an `opererr` naming both modules, the node, and the `augment` statement on its last line.

4. **External plugin augmenting `service`:**
   - Before: `docs/plugin-development/schema.md` showed only a standalone root (`acme-monitor`) with no `augment` example or shared-root owner table, and the in-repo external-plugin fixture `internal/test/externalyang/externalyang.go` (`augmentText`) imported `ze-dns-resolver-conf` to write `augment "/dnsresolver:service"`.
   - After: `ze-service-conf` (prefix `svc`) is always built beside `ze-system-conf`, `internal/test/externalyang/externalyang.go` (`augmentText`) imports `ze-service-conf` and writes `augment "/svc:service"`, and `docs/plugin-development/schema.md` adds an `augment` example and shared-root owner table naming `ze-service-conf` (`svc`) and the other owners.

5. **`ze config fmt` child order on shared roots:**
   - Before: `mergeContainer` put whichever `-conf` module sorted first alphabetically ahead of the true owner (`api-server` before `daemon`/`log` under `environment`; `authentication` before `host`/`domain` under `system`; CoS's lists before `backend` under `interface`).
   - After: `definitionOrder` (`internal/component/config/yang_schema_order.go`) places each owner's own statements first, followed by augmented children in augmenting-module name order. Every config validates and parses to the exact same `Tree.ToMap()`.

6. **`ze help ai` environment services list (`aihelp.Services()`):**
   - Before: walking each `-conf` module's own `environment` entry emitted three separate `api-server` entries (one each from `ze-api-conf` with `token`, `ze-grpc-conf` with `grpc`, and `ze-rest-conf` with `rest`).
   - After: walking `ze-hub-conf`'s `environment` entry collapses those three duplicate rows into one unified `api-server` entry carrying `ze-api-conf`'s `ShortHelp` and combined children `grpc`, `rest`, and `token`, while every other environment service stays unchanged.

### Alternatives set aside

| Alternative | Why set aside |
|-------------|---------------|
| Option B: convert `service` only and keep container merging for the other four roots | Leaves an allowlist of mergeable roots beside the join's rule (violating Thomas's "only difference is how they reach the bus" rule) and leaves the CoS list defect unguarded unless patched separately. |
| Option C: merge containers on both compiled-in and external sides, refusing only leaf and list clashes | Contradicts Thomas's approved recommendation to refuse redefinitions on both sides, keeps shared-root help dependent on alphabetical module-name order, and still requires converting CoS's lists. |
| Have `ze-dns-resolver-conf` own `service` instead of creating `ze-service-conf` | Forces five gated service plugins (`as112`, `dhcp-server`, `geodns`, `image-server`, `tftp-server`) and external plugins to import an unrelated DNS resolver plugin's module; Thomas chose always-built `ze-service-conf` beside `ze-system-conf` (O-2, 2026-11-24). |
| Have `ze-l2tp-auth-local-conf` or `ze-l2tp-auth-radius-conf` own `l2tp/auth` | Couples one auth plugin to the other across separate gates (`ze_radius`); Thomas placed `container auth` in `ze-l2tp-conf` (O-4, 2026-11-24), which is always built whenever either auth plugin is built. |

### Documents departed from

1. **`docs/architecture/config/syntax.md`**, section `Child order`: departs from the table row `"Contributed by a second \`-conf\` module to a top-level container both define"` (which described merged top-level containers). After this work, AC-8 removes that row because every cross-module contribution enters through `augment` and follows the `augment` row above it.
2. **`docs/architecture/aaa-tacacs.md`**, section `The schema merge defect this work uncovered`: departs from `"The fix is a recursive \`mergeContainer\` and \`mergeNode\`, and it protects any future module that extends a shared container."` (and its anchor to `mergeContainer`/`mergeNode`). After this work, AC-8 updates the section to record that recursive schema merging was replaced by one-definer-per-node `augment` (`ze-authz-conf` augmenting `/sys:system`, `ze-tacacs-conf` augmenting `/sys:system/authz:authentication`) and refusal in `Schema.Define`.
3. **`docs/architecture/config/yang-config-design.md`**, section `Joining a module that no \`init()\` registered`: departs from `"The redefinition refusal judges external modules only. The compiled-in modules may share a top-level container by a second definition, which \`Schema.Define\` merges: six shipped modules each define \`container service\`."`. After this work, AC-8 rewrites the paragraph to state the single rule enforced on both compiled-in and external modules and adds the table of shared roots and their owners.
4. **`docs/architecture/config/yang-config-design.md`**, section `Several modules can declare one node, and the \`?\` box carries them all` (lines 1843–1871): departs from `"A plugin attaches its leaves to a shared node by declaring a container of the same name in its own module. It does not import the module that owns the node. Removing the plugin then removes its leaves and nothing else, which is what plugin self-containment requires. \`class-of-service\` reaches \`interface\` that way."` and the lines 1865–1869 rationale that `"nothing in the schema says which module OWNS a shared node"`. After this work, AC-8 rewrites the section to describe single-owner `augment` (where `ze-cos-conf` augments `ze-iface-conf`'s lists and each shared node has one owner whose `ze:help` and `description` apply).
5. **`docs/architecture/config/yang-config-design.md`**, section `Extensions: \`ze-extensions.yang\`` (line 182): departs from `"A section Ze merges across \`-conf\` modules into one config data node is no exception: a path into another module's section writes that module's prefix, as in \`/zb:sec/zb:y\`."`. After this work, AC-8 updates that sentence because Ze no longer merges sections across `-conf` modules; cross-module leafrefs address the owner's prefix and each augmented child's prefix.
6. **`ai/doors/config-yang.md`**, section `Boundaries`: departs from `"**Extend a container with \`augment\`, never with a second definition.** A redefinition replaces it"`. After this work, AC-8 updates the consequence sentence to state that a second definition is refused on both compiled-in and external sets, never merged and never replaced.

### Choices for Thomas

none (all four owner decisions O-1..O-4 were decided by Thomas on 2026-11-24 and unfolded into this design).

## Implementation Steps

1. **Phase: Wiring and red tests** - write AC-1's and AC-2's tests. Both are red over today's set.
   - Tests: `TestCosAddsToTheInterfaceListsItDoesNotRedefine`, `TestCompiledInSetHasOneDefinerPerConfigNode`, `TestSharedRootsKeepEveryChildPathAndType` (green before, kept green)
   - Files: the test files
   - Verify: AC-1 and AC-2 red, AC-3 green
2. **Phase: The CoS defect (lands first, own commit)** - CoS augments the four iface lists and their `unit` lists, and keeps no `container interface` or key leaf of its own.
   - Tests: AC-1 green (including `Unique` / web `ListTable` and sidebar `ShortHelp`), AC-3 green
3. **Phase: One root per commit** - `environment` (with `api-server`, `ze-hub-conf` whole-root help, `aihelp.Services()` updated to walk `ze-hub-conf`), `system` (with `authentication`, the ssh and tacacs paths, `ze-system-conf` whole-root help), `l2tp` (with `container auth` moved into `ze-l2tp-conf`, whole-root help), `service` (create always-built `internal/component/config/service/yang/ze-service-conf.yang` with prefix `svc`, run `./le yang glue update` and `./le repo update`, convert the six service plugins to `augment "/svc:service"`, and update `internal/test/externalyang/externalyang.go` `augmentText` and the comment in `test/plugin/external-yang-augment-shared-root.ci`).
   - Tests: AC-3, AC-4 (`ToMap()` equality and `definitionOrder` placement), AC-6, AC-9 per root; AC-2 green after the last
4. **Phase: The refusal** - `Schema.Define` refuses duplicate definitions, `mergeContainer` and `mergeNode` are deleted, and `schemaFromResolved` and `yang.Join` call the shared `refuseRedefinedRoots`.
   - Tests: AC-5, AC-10, AC-7
5. **Phase: Pages** - AC-8 (`ai/doors/config-yang.md`, `docs/architecture/config/yang-config-design.md`, `docs/architecture/config/syntax.md`, `docs/architecture/aaa-tacacs.md`, `docs/plugin-development/schema.md`).

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file and symbol |
| Correctness | Each augment's target exists in every build of its augmenter (feature gates) |
| Data flow | No list of shared roots is written: the check finds them |
| Operator errors and doctor codes | The compiled-in refusal is an `opererr` naming both modules, with the `augment` on its last line |
| Rule: no-layering | The merge (`mergeContainer`, `mergeNode`) is deleted, not kept as a fallback |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| One definer per node | `TestCompiledInSetHasOneDefinerPerConfigNode` green |
| CoS defect fixed | `TestCosAddsToTheInterfaceListsItDoesNotRedefine` green under `ze_cos` |
| Configs unchanged semantically and ordered by definition | `TestRepositoryConfigsValidate` and `TestRepositoryConfigsKeepParsedTreeAndDefinitionOrder` green |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | AC-1 restores the interface module's own list declarations (`Unique`, `KeyLeaf`, `Backend`) where CoS is built |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| A converted node changes type | Revert that root's commit, fix the augment |
| Resolve fails in a tag combination | DESIGN: the owner of that root |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Test changes needing the judge

Not edited by this design. Proposed for `ze-test-judge` at implementation, one per test:

| Test | What it proves today | Proposed | Source |
|------|---------------------|----------|--------|
| `config/schema_test.go` `TestMergeListNodePreservesBackend`, `TestMergeListNodeDoesNotWidenBackend` | `mergeNode` adopts a list's `Backend` from the second module | Replaced by AC-1's test: the list has one definer, which carries its own backend | AC-2, owner's rule |
| `authz/yang/schema_test.go`, `l2tp/yang/schema_test.go`, `ssh/yang/schema_test.go`, `gnmi/yang/schema_test.go` (entries read from the module's own root) | The module defines its nodes under its own top-level node | Read the same nodes at the owner's entry, where the augment places them, with the same assertions | goyang places augmented nodes at the target |

## Design Insights

- A redefinition guard on the external side alone left the compiled-in side with no guard, and the defect it would have refused was already shipped (AC-1).
- `spec-config-yang-definition-order` replaced `yangChildOrder` with `definitionOrder` + `augmentsInOrder`, which makes a shared root's child order depend on which module owns the root rather than which module sorts first alphabetically.

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `silent-fall-through.md` | 2026-11-17 | `mergeNode` keeps the CoS copy of the interface lists |

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| The external side keeps refusing a redefinition | Accept it as the compiled-in merge did | An arrival rule: the external module arrives after the compiled-in set is fixed (owner's "ok", 2026-11-17) |
| One check for both sets (O-1) | Convert `service` only (B); merge containers on both sides (C) | One rule, one judge (`refuseRedefinedRoots`); decided by Thomas 2026-11-24 |
| Always-built `ze-service-conf` owns `service` (O-2) | `ze-dns-resolver-conf` owns `service` | Follows `ze-system-conf` under `internal/component/config/`; no plugin module imports another plugin's module; decided by Thomas 2026-11-24 |
| Clean whole-root `ze:help` per shared root (O-3) | Keep each owner's current help (hub's read "ZeBGP environment settings") | Gives operators accurate root-wide `?` descriptions; decided by Thomas 2026-11-24 |
| `ze-l2tp-conf` owns `container auth` (O-4) | One of `l2tp-auth-local` or `l2tp-auth-radius` owns `auth` | `ze-l2tp-conf` is built whenever either auth plugin is built; decided by Thomas 2026-11-24 |
| The defect lands before the conversion | One commit for all | Shipped misbehavior is fixed without waiting for the larger change |

## Known Limitations
- Content that enters a shared root through `uses` of a grouping was not seen by the enumeration scan. AC-2's test over the resolved set is the complete census.

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | BLOCKER / ISSUE / NOTE | [pending implementation] | file:line | - |

### Fixes applied
- [pending implementation]

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] An `rfc/short/` summary exists for every RFC referenced
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints
- [ ] Integration Checklist marks "CLI grammar" when a command is added, "Doctor check" when a runtime dependency is

### Goal Gates (MUST pass)
- [ ] AC-1..AC-10 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass, run in the main checkout by the main thread
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes, and `ai/rules/quality.md` is satisfied
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (or N-A when the feature takes none)
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove plan/release/fixes/spec-config-yang-one-definer-per-node.md` only, in the same `./le commit create` script
