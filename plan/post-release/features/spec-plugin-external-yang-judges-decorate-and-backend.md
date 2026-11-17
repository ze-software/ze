# Spec: plugin-external-yang-judges-decorate-and-backend

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | plugin |
| Priority | P3 |
| Release reason | Completes a post-release feature, the YANG of an external plugin joined at `ze start`: the owner made this check a fix to do after the release (2026-11-17). |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Design tier | development tier |
| Implementation tier | development tier |
| Updated | 2026-11-17 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

At `ze start` the join (`yang.Join`, `internal/component/config/yang/join.go`)
refuses an external plugin's module whose `ze:validate` or `ze:reference` names
something the daemon does not hold. It does not judge the two other extensions
that name a registration: `ze:decorate` (a display decorator of the web UI) and
`ze:backend` (the backends of `interface`, `firewall` and `traffic control`
that support a node). This spec makes the join judge those two names the same
way.

**What an author meets today.** An author writes `ze:decorate "asn-nmae"` (a
typo of `asn-name`) on a leaf, or `ze:backend "netlnk"` on a node the module
augments into `interface`. `ze start` joins the module and says nothing.

| Extension | What follows a name Ze does not hold |
|-----------|--------------------------------------|
| `ze:decorate` | The web UI shows the value with no decoration and no message: `DecoratorRegistry.Get` answers no decorator for the name, and `resolveField` ignores the miss |
| `ze:backend`, on a node under `interface`, `firewall` or `traffic control` | A config that uses the node is refused at commit with `feature not supported by backend "netlink" (supported: netlnk)`, so the operator reads a backend limit where the fault is a typo in a plugin's module; completion hides the node |
| `ze:backend`, on a node under any other root | Nothing reads it: no component's backend gate walks that root |

A `ze:validate` or `ze:reference` name that Ze does not hold refuses the start
instead, naming the plugin, the module, the extension, the name and the schema
path (`unheldRegistrationRefusal`), and the start ends with
`doctor-plugin-schema-refused`.

**Origin.** Acceptance criterion AC-11 of
`plan/post-release/features/spec-plugin-external-yang-at-startup.md` asked for
every daemon-side registration to be judged. The join could judge only two:
the decorator and backend registries live in packages that import `config`, so
`config/yang` cannot read them. Shown that limit and the two options (build it
now, or narrow AC-11), Thomas answered on 2026-11-17: "make it a fix to do
post-release". AC-11 there was narrowed to `ze:validate` and `ze:reference`,
and its journal row moved to this spec.

**Goal.** At `ze start`, an external module that names a decorator or a backend
this build cannot honour is refused with the refusal the other two extensions
give, before the full parse.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/config/yang-config-design.md` - "Joining an external module" (the registration check), and the `ze:` extension table
  → Decision: the join judges each name against the registry its own extension reads, and judges only the entries an external module wrote
  → Constraint: `config/yang` imports no component; an owner reaches it by registering from `init()`
- [ ] `ai/doors/system.md` - "The core knows nothing, and everything registers", "A compile tag is a boundary"
  → Constraint: always-on code never imports a gated package, and a build without the tag answers that the feature is absent
- [ ] `ai/doors/config-yang.md`, `ai/doors/plugin-infra.md`
  → Constraint: nothing may parse the full config, or build a value that keeps the module set, before the publish
- [ ] `docs/plugin-development/schema.md` - "Extensions that name a registration": what an author is told today

**Key insights:**
- The two registries hold different kinds of names. Backends register from `init()` of each backend plugin, per component. Decorators are registered on an instance when the web server starts, and two of the three only when their resolver exists.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/config/yang/join.go` - `Join` (its doc comment says the two extensions are not judged), `refuseUnheldRegistrations` (walks every entry an external module wrote), `unheldRegistration` (judges `ze:validate` against the `ValidatorRegistry` and `ze:reference` against `referenceSources`), `unheldRegistrationRefusal`
- [ ] `internal/component/config/yang/reference_source.go` - `RegisterReferenceSource`: the owner registers a name from `init()`, so `config` imports no component; `referenceSources` is written during `init()` and read-only after
- [ ] `internal/component/web/decorator.go` - `DecoratorRegistry`: an instance, `Register` adds a `Decorator` by its `Name()`, `Get` answers nil for a name it does not hold
- [ ] `cmd/ze/hub/service_web.go` - the web server start builds one `DecoratorRegistry`: `asn-name` only with the Team Cymru resolver, `reverse-dns` only with the DNS resolver, `community-name` always
- [ ] `internal/component/iface/backend.go`, `internal/component/firewall/backend.go`, `internal/component/traffic/backend.go` - `RegisterBackend(name, factory)`, called from `init()` of each backend plugin: `netlink` and `vpp` for `iface`, `nft` and `vpp` for `firewall`, `tc` and `vpp` for `traffic`. The `vpp` backends are gated by `ze_vpp` (`feature-gates.txt`); `internal/component/web` is gated by `ze_web`
- [ ] `internal/component/config/backend_gate.go` - `validateBackendFeatures`: at commit, a node whose `ze:backend` list excludes the active backend is refused, unless a descendant's own list accepts it. Called by `iface`, `firewall` and `traffic` for their roots `interface`, `firewall` and `traffic/control`, over `config.YANGSchema()`, which holds the external modules once they are published
- [ ] `internal/component/cli/completer.go` - `backendRoots`, `backendAllowed`: completion hides a node no active backend matches

**Behavior to preserve:**
- The compiled-in modules are not judged by the join: a shipped module may name `vpp` in a build without `ze_vpp` (`ze:backend "netlink vpp"` in the interface modules)
- A start that declares no external plugin resolves nothing more (`Join` answers the compiled-in set itself)
- The `ze:validate` and `ze:reference` judgments, and the form of their refusal

**Behavior to change:**
- An external module whose `ze:decorate` or `ze:backend` names something this build cannot honour refuses the start, with the refusal `ze:validate` gives

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Reference source registration | `config/yang/reference_source.go` `RegisterReferenceSource` | The component that owns a set of names registers it into `config/yang` from `init()`, so the schema package imports no component | Extends: decorator names and backend names reach `config/yang` the same way, registered by their owners |
| The join's registration check | `config/yang/join.go` `refuseUnheldRegistrations`, `unheldRegistration` | Judges `ze:validate` and `ze:reference` on every entry an external module wrote, augments into compiled-in containers included | Extends: two more judgments in the same walk, with the same refusal |
| Validator registry | `config.RegisterValidators`, `yang.ValidatorRegistry`, passed to `Join` | Holds the `ze:validate` functions | Does not apply: a decorator or a backend is not a validator |
| Web decorators | `web.DecoratorRegistry`, filled in `cmd/ze/hub/service_web.go` | The run-time lookup of a decorator by name, per web server | Extends: the web component also declares from `init()` the names it can provide. The instance stays the run-time lookup. The design must keep one declaration of each name (`ai/rules/principles.md`) |
| Component backends | `RegisterBackend` of `iface`, `firewall`, `traffic` | Each backend plugin registers its factory from `init()` | Extends: each registration also reaches `config/yang` with its component, so the join can judge a name against the component whose root holds the node |
| Backend gate and completion | `config/backend_gate.go` `validateBackendFeatures`, `cli/completer.go` `backendAllowed` | Read `ze:backend` at commit and in completion | Does not change: they keep reading the names; the join only refuses names no backend answers to |
| Feature gates | `feature-gates.txt` (`ze_web`, `ze_vpp`), `docs/architecture/plugin/feature-gates.md` | A package compiled out runs no `init()`, so its names are not registered | Applies: what the join does with such a name is decision D-1 |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `ze start` with an external plugin declared in the `plugin` block or on the command line
- The plugin's Stage 1 declaration carries the module text (`SchemaDecl.YANGText`)

### Transformation Path
1. The early Stage 1 reads the declaration (`pluginserver.HoldDeclaredPlugins`)
2. `yang.Join` resolves the compiled-in set with the external modules
3. `refuseUnheldRegistrations` walks the joined entries and judges each one an external module wrote
4. A refusal ends the start and records `doctor-plugin-schema-refused`

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Plugin process to hub, at boot | The Stage 1 declaration, unchanged | No |
| Owner component to `config/yang` | Registration from `init()` | No |

### Changed-Wire Census

**Search evidence:** No wire changes: the declaration and the module text are read as today, and the new output is a refusal that `unheldRegistrationRefusal` already formats. To be confirmed at design.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| [format] | [role] | [source site] | [search finding] | [disposition] |

### Integration Points
- `unheldRegistration` - gains the two judgments
- `web` and the three backend registries - each registers its names into `config/yang`

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | To be judged at design |
| No unintended coupling (components stay isolated) | No | To be judged at design |
| No duplicated functionality (extends existing, does not recreate) | No | To be judged at design |
| Zero-copy preserved where applicable (refs, not copies) | No | Not applicable: start-time check |
| Registration over hardcoding, outbound | No | To be judged at design |
| Registration over hardcoding, inbound | No | To be judged at design |

## Open Owner Questions

| # | Question | Recommendation |
|---|----------|----------------|
| D-1 | In a build compiled without the component that owns a name (no `ze_web`, so no decorator; no `ze_vpp`, so no `vpp` backend; a platform without the netlink backends), is an external module that names that decorator or backend refused or accepted at start? | Refuse, in both cases (the supervising session's recommendation) |

### D-1: what each answer changes

| Answer | What the author meets | What it changes in the build |
|--------|-----------------------|------------------------------|
| Refuse (recommended) | `ze start` refuses the module, naming the plugin, the module, the extension, the name and the path, as for an unknown `ze:validate`. A build without `ze_web` refuses every external module that names a decorator. A module that lists `ze:backend "netlink vpp"` is refused in a build without `ze_vpp`, although the node works with `netlink`. An author who targets several builds ships the module without the name, or one module per build | The check reads only what `init()` registered. No package has to know the names of a component that is not built |
| Accept | The module is joined, and the name has no effect in that build: with no web there is no page to decorate, and the gate never selects a backend the build lacks. Only a name that no build of Ze provides, such as a typo, is refused | The join needs every name any build can hold, declared in code every build compiles: either a central list of decorator and backend names beside their owners, which "everything registers" forbids (`ai/doors/system.md`), or each owner splits its names out of its gated package into one that stays in every build |

Why refuse: it is the rule `ze:validate` and `ze:reference` already follow (a
validator or a reference source registered by a component that is not built is
not held, and the module is refused), and a module naming something this build
cannot honour should say so at startup.

### Design notes for the design phase

These follow from the code, are not owner questions, and are settled by the
design that is put to the owner:

| # | Point | Proposed answer, and why |
|---|-------|--------------------------|
| N-1 | A decorator whose resolver is not configured (`asn-name` with no Team Cymru resolver) | Held: the build can provide it, and whether the resolver is set is run-time state, not a fault of the module. A shipped module names `asn-name` in every build with web |
| N-2 | Which backends a `ze:backend` name is judged against | Those registered by the component whose root holds the node: `vpp` of `firewall` says nothing about a node under `interface` |
| N-3 | A `ze:backend` on a node under a root no component's gate reads | To be read at design against the rule of D-1: nothing in the build honours it |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Every backend name is registered from `init()` before the join runs | `RegisterBackend` callers in `internal/plugins/{iface,firewall,traffic}/*/register.go` | A held backend would be refused | Grep of the non-test callers at design | unvalidated |
| A-2 | The join runs before any web server starts, so the decorator instance cannot be the source | `runYANGConfig` order (`cmd/ze/hub/main.go`), `service_web.go` | The names must come from `init()` either way | Read at design | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A decorator name is declared twice: by `init()` and by the decorator's `Name()` | Two lists that can disagree | One declaration, with the other derived or checked by a test |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A start with an external plugin is refused for a name the build does hold |
| How is it reverted? | A single commit revert |
| Who else touches this path? | `spec-plugin-external-yang-at-startup` (the join), the web and backend owners |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `ze start` with an external plugin whose module names an unknown decorator | → | `unheldRegistration` | `test/plugin/external-yang-unheld-decorator-refused.ci` |
| `ze start` with an external plugin whose module names an unknown backend under `interface` | → | `unheldRegistration` | `test/plugin/external-yang-unheld-backend-refused.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | `ze start` runs a real external plugin (a mode of the fixture `le test fixture plugin/external-yang`) whose module carries `ze:decorate` naming a decorator this build does not provide | The start is refused before the full parse, naming the plugin, the module, `ze:decorate`, the name and the schema path; `ze doctor` then reports `doctor-plugin-schema-refused` | A module naming a decorator the build provides (`asn-name`) is joined, also when its resolver is not configured (N-1) |
| AC-2 | The same, with `ze:backend` on a node the module augments into `interface`, naming a backend no `iface` backend of this build registered | Refused the same way, naming `ze:backend` and the name | A module naming `netlink` on that node is joined in a build that registers `netlink` |
| AC-3 | A build without the owning component (`ze_web` off, or `ze_vpp` off) and a module that names its decorator or backend | As D-1 decides. With the recommended answer, refused as in AC-1 and AC-2 | With the other answer, joined, and only a name no build provides is refused |
| AC-4 | The compiled-in set alone, in any build, and a start that declares no external plugin | Loads as today: a shipped module that names a backend the build lacks still loads, and no more is resolved | Every compiled-in module: the join judges external modules only |
| AC-5 | An author reads `docs/plugin-development/schema.md` | It states that each of the four extensions refuses the start when it names something Ze does not hold | None: the page describes the behavior this spec ships |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestJoinRefusesUnheldDecorator` | `internal/component/config/yang/join_test.go` | AC-1 | |
| `TestJoinAcceptsHeldDecorator` | same | AC-1 survivor | |
| `TestJoinRefusesUnheldBackend` | same | AC-2 | |
| `TestJoinAcceptsHeldBackend` | same | AC-2 survivor | |

### Boundary Tests (numeric inputs)
N-A: no numeric input.

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `external-yang-unheld-decorator-refused` | `test/plugin/external-yang-unheld-decorator-refused.ci` | AC-1: the start is refused with exit 1 and the message names the plugin, `ze:decorate` and the name, as `external-yang-clash-refused.ci` asserts its refusal | |
| `external-yang-unheld-backend-refused` | `test/plugin/external-yang-unheld-backend-refused.ci` | AC-2 | |

### Interop Tests (Scope: protocol)
N-A: no protocol peer.

## Files to Modify
- `internal/component/config/yang/join.go` - the two judgments, and the doc comment of `Join`
- `internal/component/web/` - declare the decorator names from `init()`
- `internal/component/iface/backend.go`, `internal/component/firewall/backend.go`, `internal/component/traffic/backend.go` - each registration reaches `config/yang`
- `internal/test/fixture/plugin_fixture_external_yang.go` - fixture modes whose module names an unknown decorator and an unknown backend
- `docs/plugin-development/schema.md`, `docs/architecture/config/yang-config-design.md` - the registration check

## Files to Create
- `test/plugin/external-yang-unheld-decorator-refused.ci`, `test/plugin/external-yang-unheld-backend-refused.ci`

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 6 | Has a user guide page? | Yes | `docs/plugin-development/schema.md`, "Extensions that name a registration" |
| 12 | Internal architecture changed? | Yes | `docs/architecture/config/yang-config-design.md`, the registration check of the join |

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `gate-excludes-part-of-its-population.md` | 2026-11-17 | The join refuses an unheld `ze:validate` or `ze:reference` name of an external module and does not judge `ze:decorate` or `ze:backend` (moved 2026-11-17 from `spec-plugin-external-yang-at-startup`, whose AC-11 the owner narrowed) |

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|

## Known Limitations
- [To be written at design]
