# Spec: config-yang-one-definer-per-node

| Field | Value |
|-------|-------|
| Status | design |
| Scope | config |
| Priority | P1 |
| Release reason | Shipped builds read a config node differently from what its YANG declares. With `ze_cos` built, the schema keeps the CoS module's copy of `interface ethernet`, `veth`, `dummy` and `bridge` and drops the interface module's list-level declarations (help, description, `ze:required`, `ze:suggest`, `unique`, related tools). The help of `system`, `environment`, `service`, `l2tp` and `interface` is the help of whichever module sorts first by name. The cause is the merge of a second definition, which has no guard, so the next shared leaf is replaced in silence too. This is a configuration defect in implemented capabilities (`plan/README.md`, `release/fixes/`, P1), not an absent feature. The defect is fixed first, in its own phase and commit (AC-1). The one-rule conversion that removes the cause follows in the same spec, because it is the same mechanism and the same release category, so `plan/README.md`, "Split fixes from features", calls for no split |
| RFC gaps | - |
| Depends | Moved here from `plan/post-release/features/spec-plugin-external-yang-at-startup.md` (its D-6 and AC-44) by the owner's "ok" of 2026-11-17. Related: `plan/release/fixes/spec-config-yang-definition-order.md` decides the print order of augmented and merged children (its row "Contributed by another `-conf` module to a shared container") |
| Phase | - |
| Handoff | - |
| Design tier | development tier |
| Implementation tier | development tier |
| Updated | 2026-11-17 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

**The owner's rule (Thomas, 2026-11-17):** "The only differene should be how they reach the bus". An external plugin and a compiled-in plugin are the same thing to Ze. They differ only in transport, and a rule for external plugins alone must follow from the transport or from arriving after the compiled-in set.

That rule meets this difference. An external `-conf` module that defines a top-level config node another module already defines is refused at the join (`refuseRedefinedRoots`, `config/yang/join.go`). A compiled-in module that does the same is merged in silence by `Schema.Define` (`config/schema.go`). The supervising session recommended refusing a redefinition on both sides. The owner answered "It seems you have all the answers". The implementing agent then found five shared roots, not the one (`service`) the recommendation assumed, and stopped. Shown the five roots, the roughly 26 modules involved and the CoS-over-interface defect, with the recommendation "its own spec, written now as a design, with the enumeration and the interface defect as its first acceptance criterion; the external side keeps refusing a redefinition, which arriving after the compiled-in set justifies", the owner answered: "ok".

Goal: a top-level config node, and every node below it, is defined by exactly one module. Any other module adds to it with `augment`. A second definition in the compiled-in set is refused with the two module names and the `augment` to write. It is never merged and never replaced. Every config the repository carries parses to the same tree as before.

The external join keeps refusing a redefinition. That rule is an arrival rule: an external module arrives after the compiled-in set is fixed. After this spec, both sides are judged by one function.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/config/yang-config-design.md` - module loading, the join's refusals, and the paragraph that says the compiled-in modules may share a container by a second definition
  → Constraint: `schemaFromResolved` takes top-level nodes only from modules whose name ends in `-conf`. An augment-only module produces no top-level node, because goyang attaches its augments to the target module's entry tree.
  → Decision: the paragraph "The redefinition refusal judges external modules only ... six shipped modules each define `container service`" is rewritten to the one rule. Its "six" understates the scope: five roots are shared.
- [ ] `ai/doors/config-yang.md` - Boundaries
  → Constraint: "Extend a container with `augment`, never with a second definition". The door says that a redefinition replaces the container. The code merges containers (`mergeContainer`) and replaces only the other pairings. The door is corrected with this spec.
- [ ] `ai/doors/system.md` - a compile tag is a boundary
  → Constraint: always-on code must not import a gated package. An augment imports its target's module, and the generated `register.go` blank-imports the package that registers it. So the target's package must be built in every build that builds the augmenter.
- [ ] `docs/plugin-development/schema.md` - tells an author to import the DNS resolver's module to add under `service`
  → Decision: corrected to the module that owns `service` after this spec.

**Key insights:**
- Five roots are shared and three nested nodes are shared (Enumeration, below). The interface lists are the one place where a list, its key leaf and its list-level declarations are merged, and that merge drops the interface module's declarations whenever CoS is built.
- `ze-bgp-conf` already reaches `environment` by `augment "/hub:environment"`, and `ze-irr-conf` already reaches `system` by `augment "/sys:system"`. The design follows those two modules.
- In goyang, two modules that each define `system` produce two top-level nodes in two namespaces. Ze merges them by local name. So `ze-ssh-conf`'s `augment "/authz:system/..."` and `ze-tacacs-conf`'s leafref `/authz:system/...` name the authz module's own `system`. Both paths change when authz augments `/sys:system`.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/config/schema.go` - `Schema.Define`, `mergeContainer`, `mergeNode` (pairing table below)
- [ ] `internal/component/config/yang_schema.go` - `schemaFromResolved` sorts the module names, then calls `Define` for each top-level node of each `-conf` module. `PluginOnlySchema` and `LoadYANGModule` also call `Define`.
- [ ] `internal/component/config/yang/join.go` - `refuseRedefinedRoots` collects the top-level nodes of the compiled-in `-conf` modules, keeps the first holder of each, does not judge the compiled-in modules against each other, and refuses an external module's top-level node that is already held, naming the holder and the `augment`.
- [ ] `internal/component/aihelp/aihelp.go` - the services walk reads each `-conf` module's own `environment` entry, in module-name order, and expects `api-server` to repeat across modules.
- [ ] `internal/plugins/cos/yang/ze-cos-conf.yang`, `internal/component/iface/yang/ze-iface-conf.yang` - the CoS module defines `container interface` with the lists `ethernet`, `veth`, `dummy` and `bridge`, each keyed by its own `leaf name` (type string, CoS help). Its comment says "The canonical node is owned by ze-iface-conf.yang".
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
| container | container | Recurses. An empty `Backend` adopts the new one's |
| list | list | The existing list node is kept, its children are merged, and only `Backend` is adopted. Every other list-level field of the second module is dropped |
| leaf, or mixed kinds | any node | The first module's node is kept. The second is dropped in silence |

**Behavior to preserve:**
- Every config node keeps its path, its kind and its value type. Every config the repository carries parses to the same tree (AC-4).
- A plugin's `ConfigRoots` are hand-declared strings (`register.go` of each plugin), so no plugin's auto-load changes.
- The external join's operator text keeps naming the plugin, the node, the holder and the `augment` (`redefinedRootRefusal`).

**Behavior to change:**
- A second definition in the compiled-in set is refused (AC-2).
- The CoS module adds to the interface module's lists and does not redefine them (AC-1).
- The help of a shared root is its owner's (AC-6).

## Enumeration (2026-11-17)

Source: a brace-aware read-only scan of every `*-conf.yang` under `internal`, `pkg` and `cmd` (test data excluded), cross-checked by reading the modules. Content that enters through `uses` of a grouping is not seen by the scan. AC-2's test is the complete census.

| Root | Definers (module `ze-<x>-conf`) | Owner (recommended) | Owner always built when an augmenter is? |
|------|-------------------------------|---------------------|-------------------------------------------|
| `environment` | 13: api, bmp, gnmi, grpc, hub, l2tp, lg, mcp, mtu, ntp, rest, ssh, web | `ze-hub-conf`, which `ze-bgp-conf` already augments | Yes, hub carries no gate |
| `system` | 4: authz, radius, system, tacacs | `ze-system-conf`, which `ze-irr-conf` already augments | Yes |
| `interface` | 2: cos, iface | `ze-iface-conf` | Yes, iface carries no gate |
| `l2tp` | 5: l2tp, l2tp-auth-local, l2tp-auth-radius, l2tp-pool, l2tp-shaper | `ze-l2tp-conf` | Yes: the four plugins build only with `ze_l2tp` |
| `service` | 6: as112, dhcp-server, dns-resolver, geodns, image-server, tftp-server | none today: only dns-resolver is always built | A new always-built module (O-2) |

| Nested shared node | Definers | Owner (recommended) |
|--------------------|----------|---------------------|
| `environment/api-server` | api, grpc, rest | `ze-api-conf` (no gate). grpc and rest augment it |
| `system/authentication` | authz, radius, tacacs | `ze-authz-conf` (no gate), which augments `/sys:system`. radius and tacacs augment authz's `authentication` |
| `l2tp/auth` | l2tp-auth-local, l2tp-auth-radius | none today. `ze-l2tp-conf` gains `container auth` (O-4) |
| `interface/ethernet`, `veth`, `dummy`, `bridge` (lists, key leaf `name`) | cos, iface | `ze-iface-conf`. cos augments each list |

**The CoS-over-interface defect** (observed from the code, not yet run). `schemaFromResolved` visits `ze-cos-conf` before `ze-iface-conf`. So in every build with `ze_cos`, `interface ethernet`, `veth`, `dummy` and `bridge` are the CoS module's list nodes and carry the CoS key leaf `name`. The interface module's list-level `ShortHelp`, `Description`, `Required`, `Suggest`, `Unique`, `Related`, `DisplayKey` and `Listener` are dropped, and only `Backend` is adopted. The `interface` container's own help is the CoS one, "Interface-level class-of-service profile bindings.", where iface declares "Network interface configuration.". The same first-in-name-order rule gives `system` authz's "System-level settings", `environment` the API module's "Environment settings for API transports", and `service` as112's help. Which surfaces render the schema node's list-level fields is A-1. The completer joins every declaration's help on its own (`mergeHelpExts`, journal `silent-fall-through.md` 2026-09-04, fixed), so it is not one of them.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| YANG `augment` across modules | `ze-bgp-conf.yang` (`augment "/hub:environment"`), `ze-irr-conf.yang` (`augment "/sys:system"`), `ze-vrrp-conf.yang` (augments the iface lists) | Adds nodes to another module's tree. goyang attaches them to the target's entry, and the schema reads them there | Extends: every non-owner definer moves to it |
| The join's redefinition refusal | `config/yang/join.go` `refuseRedefinedRoots`, `redefinedRootRefusal` | Refuses an external `-conf` top-level node that is already held | Extends: it becomes the one judge of both sets. The compiled-in set calls it with no plugin, and the external text is kept |
| The schema merge | `config/schema.go` `Schema.Define`, `mergeContainer`, `mergeNode` | Merges same-named containers and lists, and replaces or drops anything else | Replaces: `Define` refuses a second definition. `mergeContainer` and `mergeNode` lose their schema callers (A-4) |
| Census of the shipped set | `config/yang` `TestNoShippedModuleAddsNothing`, `config/cli` `TestThisBinaryLinksEveryModuleItsSchemaImports` | Hold the compiled-in set to a join rule; resolve the schema of the config CLI binary | Extends: AC-2's census test follows `TestNoShippedModuleAddsNothing` |
| Config sweep | `config/cli/validate_corpus_test.go` `TestRepositoryConfigsValidate` | Validates every Ze config in the repository | Applies unchanged: it is AC-4's proof |
| Completer help join | `cli/completer.go` `mergeAugmentedEntries`, `mergeHelpExts` | Joins the help of every module that declares a node | Does not apply: with one definer, it has one entry to join. It stays for nodes several augments reach |
| Generated YANG glue | `./le yang glue update` | Writes each `yang/` package's `embed.go` and `register.go`, which blank-imports the packages that register a module's imports | Applies: every converted module gains an import |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- The compiled-in module set at `yang.DefaultLoader`, and an external module at `yang.Join`.
- The operator's config text, whose grammar does not change.

### Transformation Path
1. `Loader.Resolve` resolves the modules. A converted module's augment attaches its nodes to the owner's entry tree.
2. `schemaFromResolved` walks the `-conf` modules. Each top-level node now has one definer. A second definition is refused through the shared redefinition check.
3. The parser, the validator, the editor, the web UI and the printers read the schema as before.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| External plugin to the join | Its `-conf` module augments a shared root with the owner's prefix | No: `docs/plugin-development/schema.md` names the owner after AC-8 |
| Build tags | Each augment's target package is built whenever its augmenter is | No: AC-7 |

### Changed-Wire Census

**Search evidence:** scope `internal`, `pkg`, `cmd`, `test`, `docs` (YANG, Go and `.ci`). Terms: `augment "/<p>:(environment|system|interface|service|l2tp)` and `path "/<p>:(...)` for every non-owner prefix; `GetEntry("ze-<module>-conf")` for each definer; `Dir["environment|system|service|interface|l2tp"]`; `LoadYANGModule(`. Positive control: `ze-ssh-conf.yang`'s `augment "/authz:system/authz:authentication/authz:user"` was found by the first term. Findings: one augment (ssh) and one leafref (tacacs) through authz's own `system`; one product reader of a module's own root (`aihelp.go`); the test readers listed below. The config file grammar does not change, so an operator's config is not a changed wire.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| Schema path of authz's `system` | writer | `ze-authz-conf.yang` top-level `container system` | Scan: authz defines `system` and `system/authentication` | Change: augment `/sys:system`. The nodes keep their paths |
| Schema path of authz's `system` | reader | `ze-ssh-conf.yang` `augment "/authz:system/authz:authentication/authz:user"` | First search term | Change: `/sys:system/authz:authentication/authz:user` |
| Schema path of authz's `system` | reader | `ze-tacacs-conf.yang` leafref `/authz:system/authz:authorization/authz:profile/authz:name` | Second search term | Change: `/sys:system/authz:authorization/...` |
| Each module's own `environment` entry | writer | The `container environment` of each of the 13 definers (`ze-api-conf.yang` ... `ze-web-conf.yang`), turned into that module's entry tree by `Loader.Resolve` and answered by `Resolved.GetEntry(<module>)` (`config/yang/loader.go`) | Scan of the `-conf` modules (Enumeration) | Change: only `ze-hub-conf.yang` writes `environment`. The 12 others write `augment "/hub:environment"`, which `Resolve` attaches to the hub module's entry, so `GetEntry` of those modules no longer holds an `environment` |
| Each module's own `environment` entry | reader | `aihelp/aihelp.go` services walk | `Dir["environment"]` search | Change: walk the owner's entry, which holds every augmented service. The service list stays the same (AC-9) |
| Each module's own root entry | writer | The top-level container of each definer in the Enumeration tables, through `Loader.Resolve` into `Resolved.GetEntry(<module>)` | Same scan | Change: a non-owner module's entry holds no top-level node of a shared root. Its nodes are at the owner's entry |
| Each module's own root entry | reader (tests) | `authz`, `l2tp`, `ssh`, `gnmi` `yang/schema_test.go` | `GetEntry` search | Proposed for the judge (Test changes needing the judge) |
| Each module's own root entry | reader | `schemaFromResolved` (`config/yang_schema.go`), which walks every `-conf` module's entry and calls `Schema.Define` for each top-level node | Read at the function | Change: it meets each root once, from its owner. A second meeting is the AC-5 refusal |
| A shared root an external module augments | writer | The owner module's top-level container: today the DNS resolver's `container service` (`ze-dns-resolver-conf.yang`), which `docs/plugin-development/schema.md` names; after this spec, the owner module of O-2 and the other owners of the Enumeration table | `schema.md` text; the owner modules | Change: the owner's module name and prefix are what an external augment imports. The page names them (AC-8) |
| A shared root an external module augments | reader (outside the repository) | An external plugin's `-conf` module, resolved in `yang.Join` (`config/yang/join.go`, `resolveJoined`) | `docs/plugin-development/schema.md` tells authors to import the DNS resolver's module | Change: the page names the owner. Pre-release, so no compatibility is owed |
| The ordered children of a shared root | writer, today | `mergeContainer` (`config/schema.go`): it appends a later module's new children in module-name order | Read at the function | Replaced: `mergeContainer` has no caller left in the schema build |
| The ordered children of a shared root | writer, after | `yangChildOrder` (`config/yang_schema.go`): the owner's own statements first, then the augmented nodes in name order | Read at the function | Preserve the printed order: A-3, AC-4, and `spec-config-yang-definition-order`, which decides one rule for both writers |
| The ordered children of a shared root | reader | every printer (`Serialize`, set form, web) | `ContainerNode.Children()` order | Preserve. AC-4 compares the `ze config fmt` output byte for byte |

### Integration Points
- `refuseRedefinedRoots` - the shared check for both sets, called by `schemaFromResolved` for the compiled-in set and by `Join` for external modules.
- `redefinedRootRefusal` - keeps the external text. The compiled-in text names the two modules and the `augment`.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | The schema still comes from goyang's resolved entries |
| No unintended coupling (components stay isolated) | Yes | An augment imports only an always-built owner, or an owner under the same gate (Enumeration table) |
| No duplicated functionality (extends existing, does not recreate) | Yes | One redefinition check for both sets, extending `refuseRedefinedRoots` |
| Zero-copy preserved where applicable (refs, not copies) | N-A | Schema build, no wire path |
| Registration over hardcoding, outbound | Yes | Modules keep registering from `init()`. No list of shared roots is written anywhere: the check finds them |
| Registration over hardcoding, inbound | Yes | Searched `refuseRedefinedRoots`, `schemaFromResolved` and `aihelp.go` for root names: none names a root, and the new check names none |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Some surface renders the schema's list-level fields of `interface ethernet` (help, description, `ze:suggest`, `ze:required`), so the CoS defect reaches an operator | `ListNode` fields in `schema.go`; the web editor reads `Suggest` and `Required` | The defect shows only in the schema value, and AC-1 stays a schema assertion | Grep the readers of `ListNode.ShortHelp`, `Description`, `Suggest` and `Required` before writing AC-1's test, and add the surface to AC-1 | unvalidated |
| A-2 | Converting a definition to an augment keeps each child's path, kind and value type | goyang attaches augmented nodes to the target entry; `yangToNode` reads the entry | A config stops parsing or changes type | AC-3 and AC-4 | unvalidated |
| A-3 | The print order of a shared root's children does not change | `yangChildOrder` puts nodes not in the owner's own statements after them, in name order. The merge appends them in module-name order | `ze config fmt` reorders a section | AC-4's byte comparison of `ze config fmt` output | unvalidated |
| A-4 | No product caller relies on `Define` merging, apart from `schemaFromResolved` | `Define` callers: `schemaFromResolved`, `PluginOnlySchema` (fresh schema, distinct roots), `LoadYANGModule` (`exabgp/migration/schema.go`) | A caller breaks on the refusal | `gopls references` on `Schema.Define` before phase 3 | unvalidated |
| A-5 | No gNMI or API path carries the module name of a shared root | gNMI serves the config tree | A gNMI client path changes | Grep `internal/component/gnmi` for module-qualified origins before phase 4 | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | An augment's target is missing from a build that compiles the augmenter, so resolve fails at start | `DefaultLoader` error naming an unresolved import | Owners are untagged or share the augmenter's gate (Enumeration). AC-7 resolves the set with no feature tag and with the full feature set |
| R-2 | A converted child changes kind or type | AC-3 red | Revert that module's conversion and fix the augment |
| R-3 | The help text an operator reads changes | AC-6 | Intended. O-3 decides the owner's text |
| R-4 | Another session edits a converted module | Conflict in the module | Convert one root per commit |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A config section is refused at start, or a node loses its type. AC-3 and AC-4 catch both before commit |
| How is it reverted? | One commit per root. Each reverts alone |
| Who else touches this path? | `spec-config-yang-definition-order` (child order); `spec-plugin-external-yang-at-startup` (the join) |

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
| AC-1 | A build with `ze_cos` builds the compiled-in schema | `interface ethernet`, `veth`, `dummy` and `bridge` carry the interface module's list-level declarations and key leaf (help, description, required, suggest, unique, related, display key, listener, backend) and the CoS leaves under them. `interface` carries iface's help. The test is written first and is red today | The CoS leaves stay under the lists: CoS adds them by augment |
| AC-2 | The compiled-in set at `DefaultLoader`, in the binary that links every module | Every config node is defined by exactly one module. The test goes red when a module defines a node another module defines. It is written red over today's set and turns green when the last root is converted | A node another module reaches by `augment` is one definition: the augmenter defines no node of the same name |
| AC-3 | The five shared subtrees, before and after | Each node at the same path, with the same kind and value type. The expected list is written from the YANG read before the conversion | Help and description of a shared root change only as AC-6 says |
| AC-4 | Every config the repository carries | `TestRepositoryConfigsValidate` passes, and the `ze config fmt` output of each config is byte-identical before and after | None: a changed tree is a defect |
| AC-5 | A compiled-in module defines a node another module defines | The schema build fails with an operator error that names both modules, the node and the `augment` that fixes it. Nothing is merged or replaced | A node reached by `augment` |
| AC-6 | `?` on `system`, `environment`, `interface`, `l2tp`, `service` | Shows the owner's help (O-3) | Augmented children keep their own help |
| AC-7 | The compiled-in set resolved with no feature tag, and with the full feature set | Both answer a `Resolved` and a schema, and the schema of each holds every root its linked modules define, with one definer each (AC-2) | None: an augment whose target is missing in some build is the start failure R-1 names |
| AC-8 | The pages | `ai/doors/config-yang.md` says that a second definition is refused, on both sides. `yang-config-design.md` replaces "The redefinition refusal judges external modules only" with the one rule and a table of each shared root and its owner. `docs/plugin-development/schema.md` names the `service` owner's module and prefix in its augment example. `./le doc check all` passes over the three | The rest of each page is unchanged. The join's refusals table keeps its redefinition row, because the external refusal stays |
| AC-9 | `ze help ai` services list | The same services, in the same order, with the same help, as before the conversion | None: the walk changes where it reads, not what it answers |
| AC-10 | An external module that defines `service` | Refused at the join as today, with the operator text naming the plugin, by the same function that judges the compiled-in set | An external module that augments the owner's `service` is joined |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | An operator with a CoS build types `interface ethernet ?` | schema to completer to help | `TestCosAddsToTheInterfaceListsItDoesNotRedefine` plus the surface A-1 names |
| 2 | An operator loads an existing config with `service`, `system`, `environment`, `l2tp` and `interface` sections | file to parser to tree | `TestRepositoryConfigsValidate`; the functional suites listed below |
| 3 | A developer adds a module that redefines `service` | build to schema to refusal | `TestSchemaRefusesASecondDefinitionNamingBothModules` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestCosAddsToTheInterfaceListsItDoesNotRedefine` | `internal/plugins/cos/yang/schema_test.go` (`ze_cos`) | AC-1: red today | |
| `TestCompiledInSetHasOneDefinerPerConfigNode` | `internal/component/config/cli/` (the binary that links every module) | AC-2, AC-7 | |
| `TestSharedRootsKeepEveryChildPathAndType` | `internal/component/config/cli/` | AC-3 | |
| `TestSchemaRefusesASecondDefinitionNamingBothModules` | `internal/component/config/yang_schema_test.go` | AC-5 | |
| `TestSharedRootHelpIsTheOwners` | `internal/component/config/cli/` | AC-6 | |
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
- `internal/component/config/schema.go` - `Define` refuses a second definition. Merge code without callers is deleted.
- `internal/component/config/yang_schema.go` - `schemaFromResolved` calls the shared check. Its merge comment and `PluginOnlySchema`'s are rewritten.
- `internal/component/config/yang/join.go` - `refuseRedefinedRoots` judges both sets.
- `internal/component/aihelp/aihelp.go` - walks the owner's `environment`.
- The YANG modules of the Enumeration tables, each non-owner converted to `augment`: cos; api, bmp, gnmi, grpc, l2tp, lg, mcp, mtu, ntp, rest, ssh, web (`environment`); authz, radius, tacacs (`system`); l2tp-auth-local, l2tp-auth-radius, l2tp-pool, l2tp-shaper (`l2tp`); as112, dhcp-server, dns-resolver, geodns, image-server, tftp-server (`service`); `ze-l2tp-conf` gains `container auth`.
- Docs: `ai/doors/config-yang.md`, `docs/architecture/config/yang-config-design.md`, `docs/plugin-development/schema.md`.

## Files to Create
- The owner module of `service` (O-2), its `embed.go` and `register.go` from `./le yang glue update`.
- The test files of the Unit Tests table.

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | The converted modules. No node is added except the owner containers |
| YANG validation constraints | No | Unchanged: each leaf keeps its type |
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
| 2 | Config syntax changed? | No | Grammar unchanged |
| 3 | CLI command added/changed? | No | - |
| 4 | API/RPC added/changed? | No | - |
| 5 | Plugin added/changed? | Yes | `docs/plugin-development/schema.md`: the owner of `service` and the other roots |
| 6 | Has a user guide page? | No | - |
| 7 | Wire format changed? | No | - |
| 8 | Plugin SDK/protocol changed? | No | - |
| 9 | RFC behavior implemented, changed, or newly proven? | No | - |
| 10 | Test infrastructure changed? | No | - |
| 11 | Affects daemon comparison? | No | - |
| 12 | Internal architecture changed? | Yes | `docs/architecture/config/yang-config-design.md`, `ai/doors/config-yang.md` |
| 13 | Route metadata keys added/changed? | No | - |
| 14 | Prometheus counters added/changed? | No | - |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | - |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Derived by `./le spec citation anchors spec plan/release/fixes/spec-config-yang-one-definer-per-node.md` at implementation |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `schema.md` examples of an augment under `service` |

## Implementation Steps

1. **Phase: Wiring and red tests** - write AC-1's and AC-2's tests. Both are red over today's set.
   - Tests: `TestCosAddsToTheInterfaceListsItDoesNotRedefine`, `TestCompiledInSetHasOneDefinerPerConfigNode`, `TestSharedRootsKeepEveryChildPathAndType` (green before, kept green)
   - Files: the test files
   - Verify: AC-1 and AC-2 red, AC-3 green
2. **Phase: The CoS defect (lands first, own commit)** - CoS augments the four iface lists and the `interface` container, and keeps no key leaf of its own.
   - Tests: AC-1 green, AC-3 green
3. **Phase: One root per commit** - `environment` (with `api-server`), `system` (with `authentication`, the ssh and tacacs paths), `l2tp` (with `auth`), `service` (with its new owner, after O-2).
   - Tests: AC-3, AC-6, AC-9 per root; AC-2 green after the last
4. **Phase: The refusal** - `Define` refuses, and `schemaFromResolved` and the join call the one check.
   - Tests: AC-5, AC-10, AC-7
5. **Phase: Pages** - AC-8.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file and symbol |
| Correctness | Each augment's target exists in every build of its augmenter (feature gates) |
| Data flow | No list of shared roots is written: the check finds them |
| Operator errors and doctor codes | The compiled-in refusal is an `opererr` naming both modules, with the `augment` on its last line |
| Rule: no-layering | The merge is deleted, not kept as a fallback |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| One definer per node | `TestCompiledInSetHasOneDefinerPerConfigNode` green |
| CoS defect fixed | `TestCosAddsToTheInterfaceListsItDoesNotRedefine` green under `ze_cos` |
| Configs unchanged | `TestRepositoryConfigsValidate` and the `ze config fmt` comparison |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | AC-1 restores the interface module's own list declarations where CoS is built |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| A converted node changes type | Revert that root's commit, fix the augment |
| Resolve fails in a tag combination | DESIGN: the owner of that root |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Options

| Option | How | Cost | Forecloses |
|--------|-----|------|------------|
| A (recommended) | One definer per node. Owners: hub (`environment`), system (`system`), iface (`interface`), l2tp (`l2tp`, gaining `container auth`), api (`api-server`), authz (`authentication`, by augmenting `/sys:system`), and a new always-built module for `service`. `Define` refuses, and one function judges both sets | About 26 module edits; the help of a shared root consolidates on its owner; the per-module tests below change through the judge; `aihelp.go` changes; glue regenerated | Nothing the owner's rule keeps |
| B | Convert `service` only, and keep the merge for the other four roots | Smallest edit | The one rule: an allowlist of mergeable roots is a second rule beside the join's, and the CoS list defect stays unless it is fixed separately |
| C | Merge on both sides: drop the join's refusal for containers, and refuse leaf and list clashes on both sides | No module edit for containers | The owner's approved recommendation (refuse on both sides). The help still depends on name order. The CoS lists must still convert, because they are lists |

## Test changes needing the judge

Not edited by this design. Proposed for `ze-test-judge` at implementation, one per test:

| Test | What it proves today | Proposed | Source |
|------|---------------------|----------|--------|
| `config/schema_test.go` `TestMergeListNodePreservesBackend`, `TestMergeListNodeDoesNotWidenBackend` | `mergeNode` adopts a list's `Backend` from the second module | Replaced by AC-1's test: the list has one definer, which carries its own backend | AC-2, owner's rule |
| `authz/yang/schema_test.go`, `l2tp/yang/schema_test.go`, `ssh/yang/schema_test.go`, `gnmi/yang/schema_test.go` (entries read from the module's own root) | The module defines its nodes under its own top-level node | Read the same nodes at the owner's entry, where the augment places them, with the same assertions | goyang places augmented nodes at the target |

## Decisions that are Thomas's (open at design review)

| ID | Decision | Options | Recommendation |
|----|----------|---------|----------------|
| O-1 | The design | A, B, C (Design Options) | A |
| O-2 | Name and home of the module that owns `service` | A new `ze-service-conf` beside `ze-system-conf` (`internal/component/config/`), always built; or the DNS resolver's module as owner, imported by five gated plugins | `ze-service-conf`, following `system`: no plugin's module imports another plugin's |
| O-3 | The help of each shared root | The owner's current text; a new text for the whole root (hub's is "ZeBGP environment settings") | A new text per root, written with AC-6 |
| O-4 | The owner of `l2tp/auth` | `ze-l2tp-conf` gains an empty `container auth`; one of the auth plugins owns it | `ze-l2tp-conf`: it is built whenever either plugin is |

## Design Insights

- A redefinition guard on the external side alone left the compiled-in side with no guard, and the defect it would have refused was already shipped (AC-1).

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `silent-fall-through.md` | 2026-11-17 | `mergeNode` keeps the CoS copy of the interface lists |

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| The external side keeps refusing a redefinition | Accept it as the compiled-in merge did | An arrival rule: the external module arrives after the compiled-in set is fixed (owner's "ok", 2026-11-17) |
| One check for both sets | A second check for the compiled-in set | One rule, one judge (owner's rule) |
| The defect lands before the conversion | One commit for all | Shipped misbehavior is fixed without waiting for the larger change |

## Known Limitations
- Content that enters a shared root through `uses` of a grouping was not seen by the enumeration scan. AC-2's test over the resolved set is the complete census.

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
