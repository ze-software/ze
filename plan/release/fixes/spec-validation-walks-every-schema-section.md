# Spec: validation-walks-every-schema-section

| Field | Value |
|-------|-------|
| Status | design |
| Scope | config |
| Priority | P1 |
| Release reason | A `ze:validate` declared outside a hand list loads, passes the startup check and never runs, so `ze config validate`, reload, editor commit and startup accept a value the schema refuses. That is accepted-but-ineffective configuration in a shipped capability, a release defect (`plan/README.md`, "Split fixes from features") |
| RFC gaps | - |
| Depends | Split from `plan/release/fixes/spec-yang-single-source-of-truth.md` (piece B). Siblings: `plan/release/fixes/spec-yang-defaults-applied-once.md` (piece C), `plan/release/fixes/spec-review-typed-config-decode.md` (piece A, uint64). The choice and `when` half of the mandatory walk stays with `plan/release/fixes/spec-config-yang-when-unique-choice.md` |
| Phase | - |
| Handoff | - |
| Design tier | development tier |
| Implementation tier | development tier |
| Updated | 2026-11-26 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

**Problem.** `validateCustomSectionsSchema` (`internal/component/config/validate_sections.go`) runs `ze:validate`, range, length and pattern checks only inside the 21 top-level sections that `validatedSections` names by hand. Six of those 21 names (`web`, `ssh`, `dns`, `looking-glass`, `mcp`, `managed`) sit under `environment` rather than at the top level, so their iterations do nothing, while 25+ real top-level sections (including `bgp`, `redistribute`, `policy`, `static`, `control-plane-protection`, and `environment`) are omitted from the full walk. A `ze:validate` under any omitted section passes `CheckAllValidatorsRegistered` and never runs. The mandatory check reaches every present section through two extra walks (`mandatoryFailures` and `bgpMandatoryFailures`), which read the same hand list and deliberately drop every non-mandatory validation error.

**Why the list existed (and what remains on HEAD).** Originally three validators blocked widening the list: `AddressFamilyValidator` (omitted engine-builtin families) and two late-populated process send/event validators (`SendMessageValidator` and the receive-event validator, filled by `RegisterPluginSendTypes`/`RegisterPluginEventTypes` after `LoadConfig`). On HEAD, the send/receive process validators and `RegisterPluginSendTypes`/`RegisterPluginEventTypes` have been retired and replaced by the unified bus message validators (`BusMessageNameValidator`, `BusMessageOrNoneValidator`), which already read `events.AllMessages()` (populated in `init()`) and `ExternalBusDeclarations()` (published before parse by `PublishDeclaredSchemas` / `HoldDeclaredPlugins`). Only one validator defect remains: `AddressFamilyValidator` still reads `registry.FamilyMap()` (plugin-registered families only) instead of `registry.AllFamilies()` (plugin plus engine-builtin families), which would refuse builtin families such as `ipv4/unicast` and `ipv6/unicast` across shipped BGP configs.

**Goal.** Delete `validatedSections`, `knownUnwalkedValidatorSections`, `KnownUnwalkedValidatorSections`, `mandatoryFailures`, and the separate `bgpMandatoryFailures` walk so `validateCustomSectionsSchema` walks every top-level section `(*Validator).Sections()` derives from the compiled schema in one pass, keeping `EverySessionCarries` for global `bgp` mandatory leaves (`router-id`, `session/asn/local`). Fix `AddressFamilyValidator` (`ValidateFn` and `CompleteFn`) to read `registry.AllFamilies()` and list known families in its refusal. Fix the SSH config editor validator (`internal/component/cli/validator.go`) so `NewConfigValidator()` wires the custom-validator registry via `config.YANGValidator()` and `editorReportsSectionFailure` stops dropping `bgp` `ze:validate` and other non-duplicate `bgp` failures from `ValidateCustomSections`, ensuring `LoadConfig`, `ze config validate`, and editor `commit`/`commit verify` reach one verdict on `bgp` as well as every other section. Update `ValidatorSectionCoverage` and `TestEveryValidatorSectionIsWalked` so every declaring section is verified against the schema-derived walk with no excuse map, and update the affected architecture docs and doors.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/config/yang-config-design.md`, "Registered Validators", "Where a validator actually runs", and "Startup Integrity Check"
  → Constraint: one walk (`validateCustomSectionsSchema`), shared by `ValidateCustomSections` (`LoadConfig`/`ReadConfig`/reload, `ze config validate`, editor `sectionErrors`) and `excludeInvalidServiceBlocks` (`LoadConfigForStart`); a second walk is the defect the shared function closed
  → Decision: the derivation reads the resolved model, never the YANG source, because a `ze:validate` in a grouping lands wherever the grouping is used
- [ ] `docs/architecture/config/yang-config-design.md`, "Layer 4: The BGP peer pipeline"
  → Constraint: `bgp` cross-plugin rules (role against filter chains, bus requirements, dynamic group resolution) stay in the peer pipeline (`PeersFromConfigTree`); only per-leaf `ze:validate` and YANG type/constraint checks join the shared schema walk
- [ ] `ai/doors/system.md`, "The core knows nothing, and everything registers"
  → Constraint: features register from `init()` and are discovered; never maintain a central hand list of sections or families
- [ ] `ai/doors/config-yang.md`, Traps: "A `ze:validate` can be declared and never run"
  → Constraint: removing `validatedSections` closes the hand-list trap; the typedef-extension trap remains
- [ ] `ai/doors/commands-cli.md`, "Boundaries" and "Traps"
  → Constraint: validation and completion derive from the owner's registration and schema, and all config surfaces (SSH editor, `ze config validate`, daemon load) enforce the same rules
- [ ] `docs/architecture/ospf/ospf-4-component-config.md`, "Constraints on callers"
  → Constraint: removing `validatedSections` removes the documented requirement for a static allow-list entry per new top-level config root
- [ ] `docs/architecture/bgp/filter-path-asn.md`, "Constraints"
  → Constraint: walking `bgp` via `(*Validator).Sections()` enforces YANG bounds (such as the 512-character `length` on `regex`) in `ValidateCustomSections`, superseding the note that `bgp` is outside `validatedSections`

**Key insights:**
- The section population is already derived from the compiled schema by `(*Validator).Sections()` (`internal/component/config/yang/validator_mandatory.go`) and verified by `ValidatorSectionCoverage` (`validate_sections.go`); only the walk itself still iterates the hand list.
- `validateCustomSectionsSchema` is already shared by strict validation (`ValidateCustomSections`) and daemon startup service-block isolation (`excludeInvalidServiceBlocks` via `LoadConfigForStart`), so widening `validateCustomSectionsSchema` automatically preserves Thomas's 2026-11-26 startup rule ("Ze always starts": failing registered service blocks are excluded under a doctor code at `ze start`, while unscoped failures, `ze config validate`, reload, and editor commit refuse strictly).
- Of the three original validator blockers, two (`send-message-type` and `receive-event-type`) were retired when the unified message bus landed; only `AddressFamilyValidator` (`registry.FamilyMap()` vs `registry.AllFamilies()`) remains.

## Current Behavior (MANDATORY)

**Source files read (verified against HEAD on 2026-11-26):**
- [ ] `internal/component/config/validate_sections.go` `validatedSections`, `knownUnwalkedValidatorSections`, `validateCustomSectionsSchema`, `mandatoryFailures`, `ValidatorSectionCoverage` - `validatedSections` holds 21 hand-listed names (six of which -- `web`, `ssh`, `dns`, `looking-glass`, `mcp`, `managed` -- are under `environment` and never match top-level containers). `knownUnwalkedValidatorSections` excuses five sections: `bgp`, `redistribute`, `policy`, `static`, `control-plane-protection`. `validateCustomSectionsSchema` walks `validatedSections`, then appends `mandatoryFailures` (every other section in `validator.Sections()` except `bgp`, keeping only `ErrTypeMissing`), `bgpMandatoryFailures`, and `referenceFailures`. `ValidatorSectionCoverage` derives `Declaring` from `loader.ConfModuleNames()` entries and subtracts `validatedSections` and `knownUnwalkedValidatorSections`.
- [ ] `internal/component/config/validate_mandatory_bgp.go` `bgpMandatoryFailures`, `EverySessionCarries`, `bgpSessionPathOf` - walks `bgp` with `ValidateTreeAllModules("bgp", data)` and discards every error whose `Type != yang.ErrTypeMissing`, filtering global `router-id` and `session/asn/local` through `EverySessionCarries` (per the 2026-10-12 and 2026-10-16 owner rulings).
- [ ] `internal/component/config/validators.go` `AddressFamilyValidator`, `BusMessageNameValidator`, `BusMessageOrNoneValidator`, `RegisterValidators` (`validators_register.go`) - `AddressFamilyValidator` calls `registry.FamilyMap()` in both `ValidateFn` and `CompleteFn` (omitting builtin families) and returns `%q is not a registered address family` without listing known families. The old `SendMessageValidator` and receive-event validator no longer exist; `BusMessageNameValidator` and `BusMessageOrNoneValidator` already read `events.AllMessages()` and `ExternalBusDeclarations()`. Every other `ze:validate` name in the tree is either a pure value/form check, completion-only (`nil` `ValidateFn`), or reads a registry populated in `init()` (`registry.Has`, `redistribute.LookupSource`, `redistevents.ProtocolIDOf`).
- [ ] `internal/component/plugin/registry/registry.go` `FamilyMap`, `AllFamilies`, `RegisterBuiltinFamilies` and `internal/component/bgp/message/register_families.go` `registerBuiltinFamilies` - `RegisterBuiltinFamilies` runs at package initialization (`var _ = registerBuiltinFamilies()`) and populates `builtinFamilies`; `FamilyMap()` returns plugin-registered families only (for decode routing), while `AllFamilies()` returns `builtinFamilies` merged with plugin-registered families.
- [ ] `internal/component/config/loader.go` `LoadConfig`, `LoadConfigForStart`, `ReadConfig` and `internal/component/config/service_blocks.go` `excludeInvalidServiceBlocks` - `LoadConfig`/`ReadConfig` call `refuseInvalidCustomSections` -> `ValidateCustomSections` -> `validateCustomSectionsSchema`; `LoadConfigForStart` calls `excludeInvalidServiceBlocks` -> `validateCustomSectionsSchema`, dropping only failures scoped to a registered service block (such as `environment/web`, `environment/looking-glass`, `bgp/rpki/cache-server[*]`) and refusing unscoped failures identically to `LoadConfig`.
- [ ] `internal/component/config/infra/validator_coverage_test.go` `TestEveryValidatorSectionIsWalkedOrExcused` - asserts every key of `config.KnownUnwalkedValidatorSections()` is present in `cov.Declaring` (catching tagless test runs) and fails on any section in `cov.Unaccounted`.
- [ ] `internal/component/config/validate_sections_test.go` and `internal/component/config/rfc7950_mandatory_sections_test.go` - directly reference the package-private `validatedSections` slice (`TestValidatedSectionsExcludesBGP`, `TestValidatedSectionsIncludesService`, and the `slices.Contains(validatedSections, "bfd")` guard in `TestRFC7950MandatoryOutsideValidatedSections`).
- [ ] `internal/component/config/cli/cmd_validate.go` `runValidation` and `internal/component/cli/validator.go` `NewConfigValidator`, `sectionErrors`, `editorReportsSectionFailure`, `validateWithYANG` (and `internal/component/cli/validator_section_test.go`) - `runValidation` calls `config.ValidateCustomSections` directly. In the SSH config editor, `NewConfigValidator()` builds `yang.NewValidator(resolved)` without calling `SetRegistry(...)`, leaving `v.yangValidator.registry == nil` so `applyCustomValidators` is a no-op during `validateWithYANG`; simultaneously, `editorReportsSectionFailure` (pinned by `validator_section_test.go`) drops every `bgp` failure from `config.ValidateCustomSections` except `yang.ErrTypeReference`, and `validateWithYANG` skips peer-less/dynamic `bgp` groups (`delete(bgpMap, keyGroup)`). As a result, the SSH editor `commit` / `commit verify` silently drops `bgp` `ze:validate` failures that `LoadConfig` and `ze config validate` will refuse once `bgp` enters the schema walk.
- [ ] `internal/component/config/redistribute/yang/ze-redistribute-conf.yang` and `internal/component/config/loader_redistribute.go` - `redistribute` no longer carries `registered-address-family`; its only `ze:validate` is `redistribute-source` on `redistribute/destination/import/source`. `test/parse/redistribute-invalid-source.ci` tests scalar `import rip;` expecting `redistribute: unknown source "rip" under destination "bgp"`.

**Behavior to preserve:**
- Every valid shipped config under `test/` and `etc/` continues to load and validate.
- `bgp` cross-plugin checks keep running in `PeersFromConfigTree`, with the same messages.
- The mandatory walk keeps every refusal and `EverySessionCarries` inheritance acceptance it makes today (`TestRFC7950MandatoryRefusesBGPConfig`, `TestMandatoryBGPPeerAndGroupEntriesAreWalked`, `TestRFC7950MandatoryOutsideValidatedSections`).
- `LoadConfigForStart` (`excludeInvalidServiceBlocks`) continues to isolate failing registered service blocks into `ExcludedBlocks` rather than aborting daemon startup, while refusing unscoped validation failures.
- Sensitive leaf values stay redacted in `SectionValidationError.Message()` across every newly walked section.

**Behavior to change:**
- A `ze:validate`, range, length, or pattern violation under any schema-declared top-level section (including `policy`, `static`, `control-plane-protection`, `redistribute`, `bgp`, `environment`, and any future module's section) is caught by `validateCustomSectionsSchema`: refused by `ze config validate`, `LoadConfig` / reload, and SSH/web editor commit, and either excluded as a failing registered service block or refused if unscoped by `LoadConfigForStart`.
- In `internal/component/cli/validator.go`, `NewConfigValidator()` wires the registered custom validators via `config.YANGValidator()` so `validateWithYANG` enforces `ze:validate` on global `bgp`, standalone peers, and grouped peers, and `sectionErrors` / `editorReportsSectionFailure` stops dropping `bgp` failures from `ValidateCustomSections` whenever `validateWithYANG` did not already refuse `bgp` (covering peer-less/dynamic `bgp` groups and any augmented `bgp` subtree while continuing to always report `yang.ErrTypeReference` and avoiding duplicate errors when `validateWithYANG` already refused `bgp`).
- `AddressFamilyValidator` (`ValidateFn` and `CompleteFn`) reads `registry.AllFamilies()` so engine-builtin families and plugin-registered families are both accepted and completed, and an unknown family refusal names both the offending value and the sorted known families.
- `validatedSections`, `knownUnwalkedValidatorSections`, `KnownUnwalkedValidatorSections`, `mandatoryFailures`, and the standalone `bgpMandatoryFailures` drop-filter are deleted; `ValidatorSectionCoverage` compares `Declaring` against `(*Validator).Sections()` with no excuse map.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| The one validation walk | `internal/component/config/validate_sections.go` `ValidateCustomSections`, `validateCustomSectionsSchema`; `docs/architecture/config/yang-config-design.md`, "Where a validator actually runs" | Runs original-tree validators, custom/type checks over `validatedSections`, mandatory checks over remaining sections, and whole-tree reference checks for `LoadConfig`, `LoadConfigForStart`, `ze config validate`, and editor `sectionErrors` | Extends: same shared function, iterating `(*Validator).Sections()` for every check in one pass instead of splitting across a hand list and mandatory-only fallback walks |
| SSH config editor validator and section-failure filter | `internal/component/cli/validator.go` `NewConfigValidator`, `sectionErrors`, `editorReportsSectionFailure`; `internal/component/cli/validator_section_test.go` | Builds `yang.NewValidator(resolved)` without `SetRegistry` (so `applyCustomValidators` is a no-op during `validateWithYANG`) and drops every `bgp` failure from `ValidateCustomSections` except `yang.ErrTypeReference` | Extends: `NewConfigValidator` uses `config.YANGValidator()` (attaching the custom-validator registry) and `editorReportsSectionFailure` keeps `bgp` failures from `ValidateCustomSections` whenever `validateWithYANG` did not already refuse `bgp` (matching `editorReportsPluginVerifyError`'s `editorRefused` deduplication idiom) so editor commit matches `LoadConfig` and `ze config validate` on `bgp` `ze:validate` rules |
| Schema section derivation | `internal/component/config/yang/validator_mandatory.go` `(*Validator).Sections` | Derives the sorted, deduplicated top-level container names from every loaded `-conf` module; `mandatoryFailures` already iterates it | Extends: becomes the sole section population for the entire walk in `validateCustomSectionsSchema` and for `ValidatorSectionCoverage` |
| BGP global mandatory inheritance filter | `internal/component/config/validate_mandatory_bgp.go` `EverySessionCarries`, `bgpSessionPathOf` | Exempts global `bgp/router-id` and `bgp/session/asn/local` `ErrTypeMissing` errors when every peer/group carries them or no session exists (2026-10-12 and 2026-10-16 owner rulings) | Extends: applied inside the unified `(*Validator).Sections()` loop for `bgp` `ErrTypeMissing` failures; the separate `bgpMandatoryFailures` walk that discarded non-mandatory `bgp` failures is deleted |
| Start-time service-block isolation | `internal/component/config/service_blocks.go` `excludeInvalidServiceBlocks`; `internal/component/config/loader.go` `LoadConfigForStart` | Calls `validateCustomSectionsSchema(tree)` and drops only concrete registered service blocks (`environment/web`, `environment/looking-glass`, `bgp/rpki/cache-server[*]`, etc.) whose validation fails so `ze start` brings up networking | Does not apply (preserved unchanged): because `excludeInvalidServiceBlocks` already calls `validateCustomSectionsSchema`, widening the walk automatically routes newly walked service-block failures (such as under `environment`) through start-time isolation |
| Section coverage derivation and gate | `internal/component/config/validate_sections.go` `ValidatorSectionCoverage`, `subtreeDeclaresValidator`, `annotationRefuses`; `internal/component/config/infra/validator_coverage_test.go` `TestEveryValidatorSectionIsWalkedOrExcused` | Derives every top-level section declaring a refusing `ze:validate` from the resolved model and fails if any declaring section is neither in `validatedSections` nor in `knownUnwalkedValidatorSections` | Extends: compares `Declaring` directly against `validator.Sections()`, removes the excuse map, and renames the gate to `TestEveryValidatorSectionIsWalked` with an explicit linked-section floor |
| Validator existence check | `internal/component/config/yang/validator_registry.go` `CheckAllValidatorsRegistered` | Refuses any `ze:validate` naming an unregistered function at startup | Does not apply: checks function registration rather than section reachability, which this spec does not change |
| Hand lists and mandatory-only fallback | `internal/component/config/validate_sections.go` `validatedSections`, `knownUnwalkedValidatorSections`, `KnownUnwalkedValidatorSections`, `mandatoryFailures` | Enumerate walked sections, excused sections, and run a second mandatory-only pass over non-listed sections | Replaces: all four deleted (`ai/rules/no-layering.md`, `ai/rules/principles.md`) |
| Family registry | `internal/component/plugin/registry/registry.go` `FamilyMap`, `AllFamilies`, `RegisterBuiltinFamilies` | `FamilyMap` holds plugin-registered families only (for decode routing); `AllFamilies` holds builtin plus plugin-registered families (populated at package init) | Extends: `AddressFamilyValidator` (`ValidateFn` and `CompleteFn`) switches from `FamilyMap()` to `AllFamilies()`; `FamilyMap()` stays unchanged for plugin decode routing |
| Unified bus message validation and startup external declarations (already landed) | `internal/component/config/validators.go` `BusMessageNameValidator`, `BusMessageOrNoneValidator`; `internal/core/events` `AllMessages`; `config.PublishDeclaredSchemas` | Retired the old `SendMessageValidator`, receive-event validator, `RegisterPluginSendTypes`, and `RegisterPluginEventTypes`; validates bus message names against `init()`-registered messages and Stage-1 external declarations published before parse | Does not apply (already landed): retires the two late-registry send/event blockers from the earlier draft of this spec; no edits to `internal/component/plugin/resolve.go` are needed |
| BGP peer pipeline | `internal/component/bgp/config/peers.go` `PeersFromConfigTree`; `docs/architecture/config/yang-config-design.md`, "Layer 4: The BGP peer pipeline" | Cross-plugin peer resolution, filter chains, bus requirements, and dynamic groups | Does not apply for cross-plugin rules: Layer 4 keeps cross-plugin peer validation while per-leaf `ze:validate` and type checks run in the shared walk |

## Design documents

| Document | Section | Derived from | Binding statement | Verdict | How this design stands against it |
|----------|---------|--------------|-------------------|---------|-----------------------------------|
| `ai/doors/system.md` | The core knows nothing, and everything registers | the door every change reads | "**A feature registers from `init()` in its own `register.go` and is discovered.** It never needs an edit to a switch, a factory, a field list, a help string or a completion table elsewhere" | agrees | Deleting the hand-maintained `validatedSections` and `knownUnwalkedValidatorSections` lists so `validateCustomSectionsSchema` discovers top-level sections from the compiled schema (`(*Validator).Sections()`), and reading `registry.AllFamilies()` in `AddressFamilyValidator`, removes manual section enumeration in favor of registration-driven discovery. |
| `docs/architecture/config/yang-config-design.md` | Registered Validators | the // Design: page of internal/component/config/validate_sections.go; anchored to internal/component/config/validate_sections.go; the // Design: page of internal/component/config/validate_mandatory_bgp.go; anchored to internal/component/config/validate_mandatory_bgp.go; the // Design: page of internal/component/config/validators.go; anchored to internal/component/config/validators.go; the // Design: page of internal/component/config/rfc7950_mandatory_sections_test.go; the // Design: page of internal/component/cli/validator.go; anchored to internal/component/cli/validator.go; a page this spec modifies | "Yes -- queries `registry.FamilyMap()`" | departs | `AddressFamilyValidator` switches from `registry.FamilyMap()` (plugin-only) to `registry.AllFamilies()` (engine-builtin plus plugin families) so `ipv4/unicast`, `ipv6/unicast`, and other builtin families validate and complete. This spec updates the "Registered Validators" table (and drops the retired `receive-event-type` and `send-message-type` sample rows) in the same change. |
| `docs/architecture/config/yang-config-design.md` | Where a validator actually runs | the // Design: page of internal/component/config/validate_sections.go; anchored to internal/component/config/validate_sections.go; the // Design: page of internal/component/config/validate_mandatory_bgp.go; anchored to internal/component/config/validate_mandatory_bgp.go; the // Design: page of internal/component/config/validators.go; anchored to internal/component/config/validators.go; the // Design: page of internal/component/config/rfc7950_mandatory_sections_test.go; the // Design: page of internal/component/cli/validator.go; anchored to internal/component/cli/validator.go; a page this spec modifies | "Declaring a `ze:validate` is not sufficient to make it run. `ValidateCustomSections` iterates `validatedSections`, a list of top-level section names, and checks only inside those, so an annotation under any other section never executes." | departs | This spec deletes `validatedSections`, `knownUnwalkedValidatorSections`, `mandatoryFailures`, and the standalone `bgpMandatoryFailures` drop-filter so `validateCustomSectionsSchema` iterates `(*Validator).Sections()` directly in one pass, and fixes `internal/component/cli/validator.go` (`NewConfigValidator` using `config.YANGValidator()` + `editorReportsSectionFailure` keeping non-duplicate `bgp` failures) so `LoadConfig`/`LoadConfigForStart`, `ze config validate`, and SSH editor `commit`/`commit verify` truly reach one verdict on `bgp` as well as every other section. The resolved-model coverage derivation (`ValidatorSectionCoverage`), completion-only skipping (`annotationRefuses`), and `EverySessionCarries` for global `bgp` mandatory leaves stay intact. This spec updates this section and its source anchors in the same change. |
| `docs/architecture/config/yang-config-design.md` | Layer 4: The BGP peer pipeline | the // Design: page of internal/component/config/validate_sections.go; anchored to internal/component/config/validate_sections.go; the // Design: page of internal/component/config/validate_mandatory_bgp.go; anchored to internal/component/config/validate_mandatory_bgp.go; the // Design: page of internal/component/config/validators.go; anchored to internal/component/config/validators.go; the // Design: page of internal/component/config/rfc7950_mandatory_sections_test.go; the // Design: page of internal/component/cli/validator.go; anchored to internal/component/cli/validator.go; a page this spec modifies | "`bgp` is deliberately absent from `validatedSections`, because the BGP tree has a deeper walk of its own: `PeersFromConfigTree` resolves the group and peer layers, builds each peer's settings and filter chains, and validates the result." | departs | Per-leaf `ze:validate` and YANG type/constraint checks under `bgp` now run in the shared `(*Validator).Sections()` walk (with `EverySessionCarries` preserving global `router-id` and `session/asn/local` inheritance), while cross-plugin peer pipeline rules (roles, filter chains, bus requirements, dynamic groups) stay in Layer 4 (`PeersFromConfigTree`). This spec updates this section in the same change. |
| `ai/doors/config-yang.md` | Traps | the door covering internal/component/config/validate_sections.go; the door covering internal/component/config/validate_mandatory_bgp.go; the door covering internal/component/config/validators.go; the door covering internal/component/config/validate_sections_test.go; the door covering internal/component/config/rfc7950_mandatory_sections_test.go; the door covering internal/component/config/validators_test.go; the door covering internal/component/config/infra/validator_coverage_test.go; a page this spec modifies | "**A `ze:validate` can be declared and never run.** The walk checks only the sections in `validatedSections`" | departs | Deleting `validatedSections` removes the hand-list half of this trap so every top-level section the compiled schema declares is walked automatically; the typedef-extension trap ("an extension on a typedef does not reach the leaves that use it") remains. This spec updates `Traps` and the `Gates` test name (`TestEveryValidatorSectionIsWalked`) in `ai/doors/config-yang.md`. |
| `docs/architecture/config/system-update.md` | The URL rule, and the way it was got wrong | anchored to internal/component/config/validators.go | "" | silent | Anchored to `ValidateFetchURL` in `validators.go`; covers update backends, version checks, and URL fetch redaction/validation. Silent on `AddressFamilyValidator` and on which top-level sections `ValidateCustomSections` walks, and this spec does not modify `ValidateFetchURL`. |
| `docs/architecture/isis/isis-4-component-config.md` | Decision: the NET validators live centrally, the completion locally | anchored to internal/component/config/validators.go | "" | silent | Anchored to `ISISNETValidator` and `ISISSystemIDValidator` in `validators.go`; covers IS-IS component config and validators. Silent on `AddressFamilyValidator` and on the top-level section walk list (`isis` was already walked), and this spec does not modify IS-IS validators. |
| `docs/architecture/ospf/ospf-4-component-config.md` | Constraints on callers | anchored to internal/component/config/validators.go; a page this spec modifies | "`ze config validate` needs a static allow-list entry for each new top-level config root. A new YANG module alone does not make it validate through the CLI." | departs | Deleting `validatedSections` removes the static allow-list so any new top-level config root a YANG `-conf` module declares is automatically validated through `(*Validator).Sections()` by `ValidateCustomSections` and `ze config validate`. `ospfRouterIDValidator` and `ospfAreaIDValidator` remain unchanged in `validators.go`. This spec updates `Constraints on callers` in the same change. |
| `docs/architecture/resolve.md` | Where the files are read from | anchored to internal/component/config/validators.go | "" | silent | Anchored to `ValidateFetchURL` and `DelegationSourceValidator` in `validators.go`; covers external data resolution (DNS, Cymru, PeeringDB, IRR, RIR). Silent on `AddressFamilyValidator` and on which sections `ValidateCustomSections` walks, and this spec does not modify either validator. |
| `docs/architecture/hub-architecture.md` | 3. Hub Validates Against Combined Schema | anchored to internal/component/cli/validator.go | "A failure is a `reference` error (diagnostic `config-yang-reference`) naming the value, the path, and what to configure, and the editor's commit reports it for every section, bgp included." | agrees | `editorReportsSectionFailure` continues to report `yang.ErrTypeReference` unconditionally for every section including `bgp`, while also keeping non-duplicate `bgp` failures from `ValidateCustomSections` whenever `validateWithYANG` did not already refuse `bgp`. |
| `docs/architecture/plugin/plugin-system.md` | Address family registration | anchored to internal/component/cli/validator.go | "The four RFC 4760 base families (`IPv4Unicast`, `IPv6Unicast`, `IPv4Multicast`, `IPv6Multicast`) live in `internal/core/family/registry.go`. Every other family is owned by its plugin's `types.go`." | agrees | Switching `AddressFamilyValidator` to `registry.AllFamilies()` validates and completes both the engine-builtin RFC 4760 base families (registered via `RegisterBuiltinFamilies`) and every plugin-owned family, while leaving `editorReportsPluginVerifyError` and `pluginVerifyTransitionError` in `internal/component/cli/validator.go` untouched. |
| `docs/architecture/plugin/plugin-system.md` | Design decisions | anchored to internal/component/cli/validator.go | "With no daemon running, a command that parses a config starts each declared external program in query mode, never its run loop, and validates against its declaration" | agrees | Offline external-plugin declaration querying and joining stay untouched; any top-level container an external plugin's `-conf` module adds to the joined schema is now automatically included in `(*Validator).Sections()` and walked by `validateCustomSectionsSchema`. |
| `ai/doors/commands-cli.md` | Boundaries | the door covering internal/component/cli/validator.go; the door covering internal/component/cli/validator_section_test.go | "**Help, usage and completion are derived** from the owner's registration and schema, never written in a second place." | agrees | Wiring `config.YANGValidator()` into `NewConfigValidator()` and switching `AddressFamilyValidator().CompleteFn` to `registry.AllFamilies()` derives both editor validation and completion from the shared schema and validator registry rather than a registry-less validator copy or plugin-only map. |
| `docs/architecture/bgp/filter-path-asn.md` | Constraints | a page this spec modifies | "The 512-character `length` on `regex` is reported by `ValidateTreeAllModules`, and `bgp` is deliberately outside the `validatedSections` list that `ValidateCustomSections` walks (`internal/component/config/validate_sections.go`)." | departs | Deleting `validatedSections` and iterating `(*Validator).Sections()` brings `bgp` (and `policy`) into `ValidateCustomSections`, so YANG `length` bounds under `bgp` are now enforced by the shared walk in addition to the plugin's own config parse. This spec updates `Constraints` in `docs/architecture/bgp/filter-path-asn.md` in the same change. |
| `ai/doors/le-tooling.md` | Boundaries | the door covering test/parse/validate-walks-every-section.ci | "**A bare `go test` omits the feature tags**, so plugins do not register and unrelated tests fail. Run one package through `./le test unit pkg path <dir>/ [run <regex>] [vet]`, which takes the tags from the toolchain." | agrees | Unit verification runs through `./le test unit pkg` with the full feature tag set so `TestEveryValidatorSectionIsWalked` links every gated plugin's YANG, and the new `.ci` functional test under `test/parse/` needs no Linux kernel feature and runs via `./le test functional parse`. |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A config file or candidate tree at `ze start` (`LoadConfigForStart`), SIGHUP reload (`LoadConfig`), offline `ze config validate` (`runValidation`), or an SSH/web editor `commit` / `commit verify` (`sectionErrors`).

### Transformation Path
1. Parse config text to `*Tree` and prune inactive nodes (`LoadConfig`, `LoadConfigForStart`, `runValidation`, `sectionErrors`).
2. `ValidateCustomSections` runs registered original-tree validators, then calls `validateCustomSectionsSchema(tree)` (`LoadConfigForStart` calls `excludeInvalidServiceBlocks(tree)`, which iterates `validateOriginalTreeAll` + `validateCustomSectionsSchema` to isolate registered service blocks).
3. `validateCustomSectionsSchema` obtains the cached `*yang.Validator`, iterates every top-level section in `validator.Sections()` that exists in `tree`, runs `validator.ValidateTreeAllModules(section, container.ToMap())` (applying custom `ze:validate` validators, YANG range/length/pattern/type checks, and RFC 7950 Section 7.6.5 mandatory checks in one pass), filters `bgp` `ErrTypeMissing` results through `EverySessionCarries(data, ve.Path)`, and appends `referenceFailures(validator, tree)`.
4. `AddressFamilyValidator` queries `registry.AllFamilies()` (builtin + plugin families) during the walk and completion.
5. Remaining failures are tagged for sensitivity (`isSensitiveLeaf`) and returned as `[]SectionValidationError`, rendered by `SectionValidationError.Message()` / `joinSectionRefusals` (strict doors) or grouped into `ExcludedBlock` entries for registered service blocks at `ze start`.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config to plugin registry | `AddressFamilyValidator` reads `registry.AllFamilies()`, populated at `init()` by `RegisterBuiltinFamilies` and plugin `registry.Register` | No |
| Config schema to validator walk | `validateCustomSectionsSchema` reads `(*yang.Validator).Sections()`, derived from loaded `-conf` YANG modules | No |

### Changed-Wire Census

No wire changes: this spec modifies only which in-process YANG top-level sections `validateCustomSectionsSchema` iterates, which in-process registry accessor (`registry.AllFamilies()` instead of `registry.FamilyMap()`) `AddressFamilyValidator` queries, and how the SSH editor's in-process `ConfigValidator` wires `config.YANGValidator()` and filters `bgp` section failures in `editorReportsSectionFailure`. No BGP wire bytes, config file syntax, RPC method, JSON payload, or plugin IPC protocol field is added or changed.

**Search evidence:** Searched `internal/`, `cmd/`, and `pkg/` for `ValidateCustomSections`, `validateCustomSectionsSchema`, `validatedSections`, `SectionValidationError`, `AddressFamilyValidator`, `FamilyMap`, `NewConfigValidator`, and `editorReportsSectionFailure` (positive controls found: `LoadConfig` in `internal/component/config/loader.go`, `excludeInvalidServiceBlocks` in `internal/component/config/service_blocks.go`, `runValidation` in `internal/component/config/cli/cmd_validate.go`, `NewConfigValidator` / `sectionErrors` / `editorReportsSectionFailure` in `internal/component/cli/validator.go`, and `RegisterValidators` in `internal/component/config/validators_register.go`). Every caller is in-process Go; `registry.FamilyMap()` itself is untouched so plugin decode-routing callers (`internal/component/bgp/reactor/reactor.go`, `internal/component/bgp/cli/`, `internal/component/bgp/format/`) retain their exact plugin-only map.

### Integration Points
- `LoadConfig` / `ReadConfig` (`internal/component/config/loader.go`) - unchanged caller of `refuseInvalidCustomSections` -> `ValidateCustomSections`.
- `LoadConfigForStart` (`internal/component/config/loader.go`) / `excludeInvalidServiceBlocks` (`internal/component/config/service_blocks.go`) - unchanged caller of `validateCustomSectionsSchema`; automatically benefits from walking `environment` and all other sections while isolating failing registered service blocks.
- `runValidation` (`internal/component/config/cli/cmd_validate.go`) - unchanged caller of `ValidateCustomSections`.
- `ConfigValidator` (`internal/component/cli/validator.go`) - updated in this spec (`NewConfigValidator` uses `config.YANGValidator()`, and `sectionErrors` / `editorReportsSectionFailure` stops dropping `bgp` failures from `ValidateCustomSections` when `validateWithYANG` did not already refuse `bgp`) so SSH editor `commit` / `commit verify` matches `LoadConfig` and `ze config validate`.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | One shared walk (`validateCustomSectionsSchema`) serves all callers (`ValidateCustomSections` and `excludeInvalidServiceBlocks`), and `NewConfigValidator` shares `config.YANGValidator()` with registered custom validators |
| No unintended coupling (components stay isolated) | No | `AddressFamilyValidator` reads `registry.AllFamilies()` in leaf package `plugin/registry`; `config` still imports no BGP or plugin implementation package; `cli` already imports `config` |
| No duplicated functionality (extends existing, does not recreate) | No | `(*Validator).Sections()`, `config.YANGValidator()`, and `ValidatorSectionCoverage` extended; `validatedSections`, `knownUnwalkedValidatorSections`, `mandatoryFailures`, standalone `bgpMandatoryFailures`, and the registry-less `yang.NewValidator(resolved)` call in `NewConfigValidator` deleted |
| Zero-copy preserved where applicable (refs, not copies) | No | Config validation is control-plane load time, not a wire hot path |
| Registration over hardcoding, outbound: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | No | Builtin families already register via `registry.RegisterBuiltinFamilies` and plugins via `registry.Register`; `AddressFamilyValidator` reads `registry.AllFamilies()` |
| Registration over hardcoding, inbound: no existing switch, seed map, validator, parser, runner, help string, or completion table has to learn this feature's name. Evidence names every list that was searched for the names this feature introduces, and the registry each one now derives from (`ai/rules/principles.md`) | No | Searched `internal/` and `cmd/` for `validatedSections` and `knownUnwalkedValidatorSections`; both hand lists are deleted and replaced by `(*Validator).Sections()`. `bgpSessionPathOf` stays by owner ruling (2026-10-16) for global BGP mandatory leaves |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Fixing `AddressFamilyValidator` to read `registry.AllFamilies()` removes the family-validation blocker across shipped BGP configs (the two old send/event blockers are already retired by the unified message bus on HEAD) | `validate_sections.go` comment (742 family sites across shipped configs) and HEAD inspection of `validators.go` / `validators_register.go` (showing `BusMessageNameValidator` already reads `events.AllMessages()` and `ExternalBusDeclarations()`) | Another validator or YANG constraint newly refuses a valid shipped config under `bgp`, `policy`, `static`, `control-plane-protection`, `redistribute`, or `environment` | Running `parse`, `install`, `ui`, `reload`, and `plugin` functional suites after widening the walk | unvalidated |
| A-2 | The four `policy route ... rule ... from` leaves in shipped configs either hold valid prefixes/ports or are genuinely invalid test fixtures | `knownUnwalkedValidatorSections["policy"]` comment in `validate_sections.go` | A valid shipped policy config is refused because a validator is too strict | Measurement pass across all shipped configs under `test/` and `etc/` in Phase 4 | unvalidated |
| A-3 | Walking `redistribute` in `validateCustomSectionsSchema` preserves the expected refusal on `test/parse/redistribute-invalid-source.ci` (scalar `import rip;`) or reveals how `Tree.ToMap()` lowers scalar vs block `import` entries | `ze-redistribute-conf.yang` (`ze:validate "redistribute-source"` on `import/source`) and `test/parse/redistribute-invalid-source.ci` | `RedistributeSourceValidator` fires before `RefuseRedistributeRules` with a different wording on `redistribute-invalid-source.ci` | Running `./le test functional parse` in Phase 4 and inspecting the exact refusal path | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Newly walking `environment`, `bgp`, `policy`, `static`, `control-plane-protection`, or `redistribute` exposes a pre-existing invalid fixture in `test/` or a latent validator bug | Failures during Phase 4 measurement across `parse`, `install`, `ui`, `reload`, `editor`, and `plugin` suites | Classify each failure at its producer: fix genuinely invalid fixture configs (not by weakening assertions) or fix any validator that rejects valid YANG-compliant input |
| R-2 | Double refusal for a `bgp` leaf that both `ValidateCustomSections` and the BGP peer pipeline (`ValidateBGPPeersWithPolicy` in `cmd_validate.go`) or editor `validateWithYANG` check | Duplicated lines in `ze config validate` or SSH editor validation output for a single invalid BGP leaf (such as `router-id 0.0.0.0` or an unregistered `family`) | Deduplicate in `editorReportsSectionFailure` via `bgpEditorRefused` for the editor path; measure `ze config validate` during Phase 4 and deduplicate at the single producing boundary without dropping cross-plugin checks |
| R-3 | Editing or replacing unit tests that referenced `validatedSections` (`TestValidatedSectionsExcludesBGP`, `TestValidatedSectionsIncludesService`, and the `slices.Contains(validatedSections, "bfd")` guard in `TestRFC7950MandatoryOutsideValidatedSections`) or updating `TestEditorReportsSectionFailure` triggers the test-weakening gate | `./le test weakened` or pre-commit check flags changed or removed assertions | Replace with stronger schema-derived assertions (`TestValidateCustomSectionsCoversEverySchemaSection`, `TestEveryValidatorSectionIsWalked`), keep every positive/negative RFC 7950 assertion in `TestRFC7950MandatoryOutsideValidatedSections`, keep both deduplicated (`bgpEditorRefused == true`) and non-deduplicated (`bgpEditorRefused == false`) assertions in `TestEditorReportsSectionFailure`, and obtain `ze-test-judge` approval for any retired hand-list test |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A valid config could be refused at validate, reload, commit, or start (or a registered service block could be falsely excluded at start) |
| How is it reverted? | Single commit revert |
| Who else touches this path? | `plan/release/fixes/spec-config-yang-when-unique-choice.md` (mandatory choice and `when`), `plan/release/fixes/spec-yang-defaults-applied-once.md`, `plan/release/fixes/spec-config-yang-one-definer-per-node.md`, `plan/post-release/engineering/spec-gates-share-one-semantic-population.md` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `ze config validate` on a config with a `policy route ... rule ... from` prefix its `ze:validate` refuses, and a peer configuring builtin `family ipv4/unicast` | → | `validateCustomSectionsSchema` iterating `(*Validator).Sections()` and `AddressFamilyValidator` reading `registry.AllFamilies()` | `test/parse/validate-walks-every-section.ci` |
| `LoadConfig` on a config exercising a formerly unwalked section and a test module's new top-level section | → | `ValidateCustomSections` -> `validateCustomSectionsSchema` -> `(*Validator).Sections()` | `TestValidateCustomSectionsCoversEverySchemaSection` |
| SSH config editor `ConfigValidator.Validate` / `ValidateTransition` on a `bgp` peer and a peer-less/dynamic `bgp` group violating `ze:validate` | → | `NewConfigValidator` (`config.YANGValidator()`) and `sectionErrors` / `editorReportsSectionFailure` | `TestEditorValidatorRefusesBGPCustomValidation` |
| `AddressFamilyValidator` validating and completing builtin and plugin families | → | `AddressFamilyValidator` reading `registry.AllFamilies()` | `TestAddressFamilyValidatorAcceptsBuiltinFamilies` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | A `ze:validate`, range, length, or pattern failure under any schema-declared top-level section, reproduced under `policy` (e.g. `policy route ... rule ... from source-address`), `static`, `control-plane-protection`, `redistribute`, `bgp` (both on a peer and in a peer-less/dynamic `bgp` group), `environment`, and a test module's new top-level section | `validateCustomSectionsSchema` iterates `(*Validator).Sections()` directly (`validatedSections`, `knownUnwalkedValidatorSections`, and `KnownUnwalkedValidatorSections` are deleted), and in `internal/component/cli/validator.go` `NewConfigValidator` uses `config.YANGValidator()` (attaching the custom-validator registry) while `sectionErrors` / `editorReportsSectionFailure` reports `bgp` failures from `ValidateCustomSections` whenever `validateWithYANG` did not already refuse `bgp` (and always reports `yang.ErrTypeReference`). `ze config validate`, `LoadConfig` / reload, and SSH/web editor commit refuse each failure naming the section, path, and reason (with sensitive leaves redacted); `LoadConfigForStart` isolates failures under registered service blocks into `ExcludedBlocks` and refuses unscoped failures identically to `LoadConfig` | Completion-only validators (`ValidateFn == nil`, registered via `RegisterSuggestion` or empty `CustomValidator`) refuse nothing by design (`annotationRefuses`). Sections absent from the config file are not walked (`LYD_VALIDATE_PRESENT` reading recorded against `RFC7950-7.6.5-1`) |
| AC-2 | The section coverage derivation and gate over a full-feature-tag binary | `ValidatorSectionCoverage` derives `Declaring` from the resolved model and checks every declaring section against `(*Validator).Sections()` with no excuse map; `TestEveryValidatorSectionIsWalked` (renamed from `TestEveryValidatorSectionIsWalkedOrExcused`) asserts expected declaring sections (`bgp`, `policy`, `static`, `control-plane-protection`, `redistribute`, `service`) are present in `Declaring` and passes with `Unaccounted` empty | A section compiled out by a disabled `ze_<feature>` build tag is absent from that build's schema; the test explicitly asserts the expected declaring sections are linked so a bare tagless run fails loudly instead of passing vacuously |
| AC-3 | `AddressFamilyValidator` (`ValidateFn` and `CompleteFn`) asked about an engine-builtin family (such as `ipv4/unicast`, `ipv6/unicast`), a plugin-registered family, and an unknown family name | Both engine-builtin and plugin-registered families are accepted by `ValidateFn` and returned (sorted) by `CompleteFn` via `registry.AllFamilies()`; an unknown family is refused with an error naming both the offending value and the sorted known families | None: `registry.AllFamilies()` is the single complete family inventory for validation and completion (`registry.FamilyMap()` remains plugin-only for decode routing) |
| AC-4 | The RFC 7950 Section 7.6.5 mandatory check after `mandatoryFailures` and standalone `bgpMandatoryFailures` are folded into the single `(*Validator).Sections()` walk | Every missing mandatory leaf and mandatory choice under any present top-level section refuses on `ze config validate`, `LoadConfig`, and editor commit without duplicate errors, while `EverySessionCarries` continues to exempt global `bgp/router-id` and `bgp/session/asn/local` when every peer/group supplies them or when `bgp` holds no sessions (`TestRFC7950MandatoryRefusesBGPConfig`, `TestMandatoryBGPPeerAndGroupEntriesAreWalked`, `TestRFC7950MandatoryOutsideValidatedSections` all pass) | The choice-case `when`/`unique` evaluation gaps stay with `plan/release/fixes/spec-config-yang-when-unique-choice.md`; peer-level mandatory inheritance from a group stays refused pending a separate owner ruling (`validate_mandatory_bgp.go`) |
| AC-5 | Shipped valid configs across `test/` and `etc/` validated after the walk widens to all schema sections | Every valid shipped config continues to load and pass validation (`parse`, `install`, `ui`, `editor`, `reload`, and `plugin` suites pass); any shipped fixture that carried a genuinely schema-invalid value in a formerly unwalked section is corrected at the fixture, and any validator defect exposed during measurement is fixed in its validator | None: no valid shipped config may remain broken and no section may be re-excused |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Writes a malformed match prefix in a `policy route ... rule ... from` block | config file → `ze config validate` / `LoadConfig` → `ValidateCustomSections` → `validateCustomSectionsSchema` → refusal naming `policy`, leaf path, and reason | `test/parse/validate-walks-every-section.ci` |
| 2 | Configures `family ipv4/unicast` on a BGP peer and runs `ze config validate` | config file → `LoadConfig` / `runValidation` → `validateCustomSectionsSchema` → `AddressFamilyValidator` (`registry.AllFamilies()`) → accepted | `test/parse/validate-walks-every-section.ci` |
| 3 | Configures an unknown family `family ipv4/bogus` on a BGP peer or dynamic group in the SSH editor or config file | config file / editor candidate → `ze config validate` / `ConfigValidator.ValidateTransition` → `AddressFamilyValidator` (`registry.AllFamilies()`) → refusal naming `ipv4/bogus` and known families | `test/parse/validate-walks-every-section.ci`, `TestEditorValidatorRefusesBGPCustomValidation` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestValidateCustomSectionsCoversEverySchemaSection` | `internal/component/config/validate_sections_test.go` | AC-1: `LoadConfig` / `ValidateCustomSections` refuses a `ze:validate` failure in `policy`, `static`, `control-plane-protection`, `bgp`, and a test-registered `-conf` module's new top-level section without any hand list | |
| `TestEditorReportsSectionFailure` (updated), `TestEditorValidatorRefusesBGPCustomValidation` | `internal/component/cli/validator_section_test.go` | AC-1: `editorReportsSectionFailure` keeps `bgp` failures when `validateWithYANG` did not already refuse `bgp` (and always keeps `ErrTypeReference`), and `NewConfigValidator().Validate` / `ValidateTransition` refuse a `bgp` `ze:validate` failure on both a peer and a peer-less/dynamic group | |
| `TestEveryValidatorSectionIsWalked` | `internal/component/config/infra/validator_coverage_test.go` | AC-2: asserts expected declaring sections are linked and `cov.Unaccounted` is empty with no excuse map | |
| `TestAddressFamilyValidatorAcceptsBuiltinFamilies` | `internal/component/config/validators_test.go` | AC-3: builtin families registered via `RegisterBuiltinFamilies` and plugin families registered via `Register` are both accepted and completed; unknown family refusal names the value and sorted known families | |
| `TestRFC7950MandatoryOutsideValidatedSections` (updated), `TestRFC7950MandatoryRefusesBGPConfig`, `TestMandatoryBGPPeerAndGroupEntriesAreWalked` | `internal/component/config/rfc7950_mandatory_sections_test.go`, `internal/component/config/validate_mandatory_bgp_test.go` | AC-4: mandatory checks and `EverySessionCarries` work through the unified walk with no duplicate reports | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N-A | this spec adds no numeric input or bound | N/A | N/A | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `validate-walks-every-section` | `test/parse/validate-walks-every-section.ci` | `ze config validate` refuses a malformed `policy route ... rule ... from source-address`, accepts builtin `family ipv4/unicast` on a BGP peer, and refuses an unknown `family` naming the bad value and known families | |

### Interop Tests (Scope: protocol)
N-A: config validation only, no protocol wire behavior changes.

## Files to Modify
- `internal/component/config/validate_sections.go` - delete `validatedSections`, `knownUnwalkedValidatorSections`, `KnownUnwalkedValidatorSections`, and `mandatoryFailures`; iterate `validator.Sections()` directly in `validateCustomSectionsSchema` (filtering `bgp` `ErrTypeMissing` via `EverySessionCarries`); update `ValidatorSectionCoverage` to check `Declaring` against `validator.Sections()` with no excuse map
- `internal/component/config/validate_mandatory_bgp.go` - delete the standalone `bgpMandatoryFailures` walk that discarded non-mandatory `bgp` failures; keep `EverySessionCarries` and `bgpSessionPathOf` for the unified walk in `validateCustomSectionsSchema`
- `internal/component/config/validators.go` - switch `AddressFamilyValidator` (`ValidateFn` and `CompleteFn`) from `registry.FamilyMap()` to `registry.AllFamilies()`, and include sorted known families in the refusal error
- `internal/component/cli/validator.go` - wire `config.YANGValidator()` (carrying the registered custom validators) into `NewConfigValidator()` instead of a registry-less `yang.NewValidator(resolved)`, and update `sectionErrors` / `editorReportsSectionFailure` to keep `bgp` failures from `ValidateCustomSections` whenever `validateWithYANG` did not already refuse `bgp` (always keeping `yang.ErrTypeReference`)
- `internal/component/cli/validator_section_test.go` - update `TestEditorReportsSectionFailure` for both `bgpEditorRefused` polarities and add `TestEditorValidatorRefusesBGPCustomValidation` covering peer and peer-less/dynamic `bgp` group `ze:validate` refusals
- `internal/component/config/validate_sections_test.go` - replace `TestValidatedSectionsExcludesBGP` and `TestValidatedSectionsIncludesService` (which referenced deleted `validatedSections`) with `TestValidateCustomSectionsCoversEverySchemaSection`
- `internal/component/config/rfc7950_mandatory_sections_test.go` - remove the deleted `validatedSections` guard in `TestRFC7950MandatoryOutsideValidatedSections` while preserving the `bfd` mandatory refusal and acceptance assertions
- `internal/component/config/validators_test.go` - add `TestAddressFamilyValidatorAcceptsBuiltinFamilies` covering builtin families, plugin families, completion, and known-families refusal wording
- `internal/component/config/infra/validator_coverage_test.go` - rename `TestEveryValidatorSectionIsWalkedOrExcused` to `TestEveryValidatorSectionIsWalked` and assert expected declaring sections are linked with zero unaccounted sections and no excuse map
- `docs/architecture/config/yang-config-design.md` - update "Registered Validators", "Where a validator actually runs" (including the editor `NewConfigValidator` / `sectionErrors` behavior), and "Layer 4: The BGP peer pipeline" (and their `<!-- source: -->` anchors)
- `docs/architecture/ospf/ospf-4-component-config.md` - update "Constraints on callers" to remove the retired static allow-list requirement
- `docs/architecture/bgp/filter-path-asn.md` - update "Constraints" to remove the retired claim that `bgp` sits outside `validatedSections`
- `docs/features/configuration.md` - update the `<!-- source: -->` anchors that named deleted `mandatoryFailures` and `bgpMandatoryFailures`
- `ai/doors/config-yang.md` - update the `Traps` entry on `validatedSections` (keeping the typedef-extension trap) and the `Gates` test name (`TestEveryValidatorSectionIsWalked`)

## Files to Create
- `test/parse/validate-walks-every-section.ci` - functional proof of AC-1 and AC-3

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | No YANG node added or renamed |
| YANG validation constraints | No | Existing YANG constraints start running across all sections |
| YANG custom validators | Yes | `internal/component/config/validators.go` (`AddressFamilyValidator`), `internal/component/cli/validator.go` (`NewConfigValidator` attaches custom-validator registry via `config.YANGValidator()`) |
| CLI commands/flags | No | No CLI command or flag added or changed |
| CLI grammar (keyword before value) | N-A | No command added or changed |
| Editor autocomplete | Yes | `AddressFamilyValidator().CompleteFn` switches from `registry.FamilyMap()` to `registry.AllFamilies()` so builtin families appear in editor completion |
| Functional test for new RPC/API | N-A | No RPC or API added |
| Pipe completeness | N-A | No command output shape changed |
| Env var registration | No | No environment variable added |
| Doctor check for runtime dependencies | No | No new runtime dependency |
| Doctor code per problem class | No | Refusals reuse existing config validation and service-block exclusion doctor codes |
| Prometheus counters/metrics | No | No new metric |
| BGP family surface | No | Validation and completion read existing `registry.AllFamilies()`; no SAFI, capability, or attribute added |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | Defect fix in existing config validation |
| 2 | Config syntax changed? | No | Grammar unchanged |
| 3 | CLI command added/changed? | No | Command surface unchanged |
| 4 | API/RPC added/changed? | No | No RPC change |
| 5 | Plugin added/changed? | No | No plugin change |
| 6 | Has a user guide page? | No | `docs/guide/configuration.md` already promises one walk for `ze config validate` and daemon load |
| 7 | Wire format changed? | No | No wire change |
| 8 | Plugin SDK/protocol changed? | No | No SDK or IPC protocol change |
| 9 | RFC behavior implemented, changed, or newly proven? | No | Existing RFC 7950 Section 7.6.5 mandatory behavior preserved unchanged |
| 10 | Test infrastructure changed? | No | Standard `.ci` and Go unit tests |
| 11 | Affects daemon comparison? | No | Internal validation coverage fix |
| 12 | Internal architecture changed? | Yes | `docs/architecture/config/yang-config-design.md` ("Registered Validators", "Where a validator actually runs", "Layer 4: The BGP peer pipeline"), `docs/architecture/ospf/ospf-4-component-config.md` ("Constraints on callers"), and `docs/architecture/bgp/filter-path-asn.md` ("Constraints") |
| 13 | Route metadata keys added/changed? | No | None |
| 14 | Prometheus counters added/changed? | No | None |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | `registry.AllFamilies()` already holds the registered inventory |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Derived with `./le spec citation anchors spec plan/release/fixes/spec-validation-walks-every-schema-section.md`. Updated in Files to Modify: `docs/architecture/config/yang-config-design.md`, `docs/architecture/ospf/ospf-4-component-config.md`, `docs/architecture/bgp/filter-path-asn.md`, `docs/features/configuration.md`. Anchored to changed files and unaffected: `docs/DESIGN.md` (`refuseInvalidCustomSections` and `ValidateCustomSections` stay intact), `docs/architecture/config/system-update.md` (`ValidateFetchURL` untouched), `docs/architecture/hub-architecture.md` (`editorReportsSectionFailure` still reports `ErrTypeReference` for every section including `bgp`), `docs/architecture/isis/isis-4-component-config.md` (`ISISNETValidator`/`ISISSystemIDValidator` untouched), `docs/architecture/plugin/plugin-system.md` (`editorReportsPluginVerifyError` and `pluginVerifyTransitionError` untouched), `docs/architecture/resolve.md` (`ValidateFetchURL`/`DelegationSourceValidator` untouched), `docs/features/interfaces.md` (`MACAddressValidator`/`OSDeviceNameValidator` untouched), `docs/guide/config-editor.md` (`ConfigValidationError.Text`, `refusalIssue`, `missingFieldIssue` untouched), `docs/guide/configuration.md` (`ValidateCustomSections`, `ValidateFetchURL`, `crashMemoryImageValidator`, `MACAddressValidator` untouched), `docs/guide/isis.md` (`ISISHostnameValidator` untouched), `docs/guide/plugins.md` (`ExternalPluginNameValidator` untouched) |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `ai/doors/config-yang.md` trap text on `validatedSections` and gate test name |

## Design presentation

### What changes

Config validation stops reading a hand-maintained list of top-level sections (`validatedSections`) and an excuse map (`knownUnwalkedValidatorSections`). Instead, `validateCustomSectionsSchema` walks every top-level section `(*Validator).Sections()` derives from the compiled YANG schema in one pass, applying `ze:validate` custom validators, YANG range/length/pattern checks, and RFC 7950 mandatory checks (preserving `EverySessionCarries` for global `bgp/router-id` and `bgp/session/asn/local`). `AddressFamilyValidator` switches from `registry.FamilyMap()` (plugin-only) to `registry.AllFamilies()` (engine-builtin plus plugin families) and names the known families on refusal. In the SSH config editor (`internal/component/cli/validator.go`), `NewConfigValidator()` wires the shared `config.YANGValidator()` (carrying the custom-validator registry) and `sectionErrors` / `editorReportsSectionFailure` stops dropping non-duplicate `bgp` failures from `ValidateCustomSections`, so `LoadConfig`, `ze config validate`, and SSH editor `commit`/`commit verify` reach one verdict on `bgp` as well as every other section. The section coverage test `TestEveryValidatorSectionIsWalked` verifies that every declaring section is walked with no excuse map left.

### Before and after

1. **Invalid leaf under a formerly unwalked section (`policy`, `static`, `control-plane-protection`, `redistribute`, `environment`, or a new module's section):**
   - Before: `ze config validate` accepts `policy { route r1 { rule 10 { from { source-address not-a-prefix; } } } }` with exit 0 because `policy` is absent from `validatedSections`, even though the leaf carries `ze:validate "ipv4-prefix|ipv6-prefix|set-ref"`.
   - After: `ze config validate`, SIGHUP reload, editor commit, and unscoped startup refuse it naming `policy`, the leaf path, and the validation error; at `ze start`, failures scoped to a registered service block (such as under `environment/web`) are isolated into `ExcludedBlocks` under `LoadConfigForStart`.

2. **BGP address family validation and completion (and SSH editor `bgp` `ze:validate` enforcement):**
   - Before: `AddressFamilyValidator` queries `registry.FamilyMap()`, omitting engine-builtin families (`ipv4/unicast`, `ipv6/unicast`, etc.) and refusing unknown families without listing known ones; in the SSH editor, `NewConfigValidator` builds `yang.NewValidator(resolved)` without `SetRegistry` (making `applyCustomValidators` a no-op during `validateWithYANG`) while `editorReportsSectionFailure` drops all non-reference `bgp` failures from `ValidateCustomSections`.
   - After: `AddressFamilyValidator` queries `registry.AllFamilies()`, accepting and completing both builtin and plugin-registered families and listing sorted known families on refusal; `NewConfigValidator` uses `config.YANGValidator()` and `editorReportsSectionFailure` keeps `bgp` failures from `ValidateCustomSections` whenever `validateWithYANG` did not already refuse `bgp`, so SSH editor `commit`/`commit verify` matches `LoadConfig` and `ze config validate` on both BGP peers and peer-less/dynamic BGP groups.

3. **Adding a new YANG `-conf` module with `ze:validate`:**
   - Before: a new top-level section's `ze:validate` is inert until someone adds the section name to `validatedSections` (or excuses it in `knownUnwalkedValidatorSections`).
   - After: `(*Validator).Sections()` includes the new top-level container automatically, so `ValidateCustomSections` runs its validators on day one with zero central list edits.

### Alternatives set aside

| Alternative | Why set aside |
|-------------|---------------|
| Keep `validatedSections` and append `policy`, `static`, `control-plane-protection`, `bgp`, `redistribute`, and `environment` one by one | A hand list beside `(*Validator).Sections()` is a second declaration of the schema's top-level sections (`ai/rules/principles.md`) that drifts whenever a new module lands and still leaves six dead non-top-level names in the list. |
| Keep `knownUnwalkedValidatorSections` excusing `bgp` and `redistribute` while widening only `policy`, `static`, and `control-plane-protection` | Leaves `AddressFamilyValidator` broken for builtin families and keeps two separate mandatory-only fallback walks (`mandatoryFailures` and `bgpMandatoryFailures`) beside the main walk. |
| Leave `internal/component/cli/validator.go` untouched while adding `bgp` to the shared walk | `NewConfigValidator` lacks `SetRegistry` and `editorReportsSectionFailure` drops non-reference `bgp` failures, so SSH editor `commit`/`commit verify` would silently accept `bgp` `ze:validate` failures that `LoadConfig` and `ze config validate` refuse, breaking the one-verdict contract in AC-1 and `yang-config-design.md`. |
| Change `registry.FamilyMap()` itself to include engine-builtin families | `registry.FamilyMap()` is documented and tested as the plugin-only decode-routing map (`internal/component/bgp/reactor/reactor.go`, `internal/component/bgp/cli/decode_mp_registry_test.go`, `internal/component/plugin/all/all_test.go`); `registry.AllFamilies()` already exists for the complete builtin + plugin set. |

### Documents departed from

1. **`docs/architecture/config/yang-config-design.md`**, section `Registered Validators`: departs from `"Yes -- queries \`registry.FamilyMap()\`"` for `registered-address-family`. After this work, the row states that `registered-address-family` validates and completes against `registry.AllFamilies()` (builtin plus plugin families), and the retired `receive-event-type` and `send-message-type` sample rows are removed.
2. **`docs/architecture/config/yang-config-design.md`**, section `Where a validator actually runs`: departs from `"Declaring a \`ze:validate\` is not sufficient to make it run. \`ValidateCustomSections\` iterates \`validatedSections\`, a list of top-level section names, and checks only inside those, so an annotation under any other section never executes."`. After this work, the section states that `validateCustomSectionsSchema` walks every top-level section `(*Validator).Sections()` derives in one pass (including mandatory checks with `EverySessionCarries` for global `bgp` leaves), `NewConfigValidator` shares `config.YANGValidator()` while `sectionErrors` reports non-duplicate `bgp` failures so all three callers reach one verdict on `bgp` as well, and `TestEveryValidatorSectionIsWalked` checks coverage with no excuse map.
3. **`docs/architecture/config/yang-config-design.md`**, section `Layer 4: The BGP peer pipeline`: departs from `"\`bgp\` is deliberately absent from \`validatedSections\`, because the BGP tree has a deeper walk of its own: \`PeersFromConfigTree\` resolves the group and peer layers, builds each peer's settings and filter chains, and validates the result."`. After this work, the section states that per-leaf `ze:validate` and YANG type/constraint checks under `bgp` run in the shared walk while cross-plugin peer pipeline rules stay in Layer 4 (`PeersFromConfigTree`).
4. **`ai/doors/config-yang.md`**, section `Traps`: departs from `"**A \`ze:validate\` can be declared and never run.** The walk checks only the sections in \`validatedSections\`"`. After this work, the trap removes the retired `validatedSections` hand-list note (keeping the typedef-extension trap) and `Gates` names `TestEveryValidatorSectionIsWalked`.
5. **`docs/architecture/ospf/ospf-4-component-config.md`**, section `Constraints on callers`: departs from `"\`ze config validate\` needs a static allow-list entry for each new top-level config root. A new YANG module alone does not make it validate through the CLI."`. After this work, that bullet is updated to state that `ValidateCustomSections` derives top-level config roots from the compiled YANG schema (`(*Validator).Sections()`), so a new `-conf` YANG module validates automatically with no allow-list edit.
6. **`docs/architecture/bgp/filter-path-asn.md`**, section `Constraints`: departs from `"The 512-character \`length\` on \`regex\` is reported by \`ValidateTreeAllModules\`, and \`bgp\` is deliberately outside the \`validatedSections\` list that \`ValidateCustomSections\` walks (\`internal/component/config/validate_sections.go\`)."`. After this work, the paragraph states that `ValidateCustomSections` walks `bgp` and `policy` through `(*Validator).Sections()`, so the YANG `length` bound is enforced by the shared walk in addition to the plugin's own config parse.

### Choices for Thomas

none

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- write `test/parse/validate-walks-every-section.ci`, `TestValidateCustomSectionsCoversEverySchemaSection` (`internal/component/config/validate_sections_test.go`), and `TestEditorValidatorRefusesBGPCustomValidation` (`internal/component/cli/validator_section_test.go`); observe all three fail against the current `validatedSections` list, `registry.FamilyMap()`, and registry-less editor `NewConfigValidator` / `editorReportsSectionFailure`.
2. **Phase: AddressFamilyValidator** -- update `AddressFamilyValidator` (`ValidateFn` and `CompleteFn`) in `internal/component/config/validators.go` to read `registry.AllFamilies()` and list sorted known families on refusal; add `TestAddressFamilyValidatorAcceptsBuiltinFamilies` in `internal/component/config/validators_test.go` (AC-3).
3. **Phase: Unified schema walk, editor wiring, and coverage gate** -- in `internal/component/config/validate_sections.go` and `internal/component/config/validate_mandatory_bgp.go`, delete `validatedSections`, `knownUnwalkedValidatorSections`, `KnownUnwalkedValidatorSections`, `mandatoryFailures`, and standalone `bgpMandatoryFailures`; iterate `validator.Sections()` in `validateCustomSectionsSchema` with `EverySessionCarries` filtering `bgp` `ErrTypeMissing` errors; in `internal/component/cli/validator.go`, wire `config.YANGValidator()` into `NewConfigValidator()` and update `sectionErrors` / `editorReportsSectionFailure` to keep `bgp` failures from `ValidateCustomSections` when `validateWithYANG` did not already refuse `bgp` (and always keep `yang.ErrTypeReference`); update `ValidatorSectionCoverage`, `TestEveryValidatorSectionIsWalked`, `validate_sections_test.go`, `rfc7950_mandatory_sections_test.go`, and `validator_section_test.go` (AC-1, AC-2, AC-4; obtain `ze-test-judge` approval if replacing tests of the deleted `validatedSections` triggers the test-weakening gate).
4. **Phase: Measurement** -- run the targeted unit tests and the `parse`, `install`, `ui`, `editor`, `reload`, and `plugin` functional suites (main thread); classify every newly refused config at its producer (fix genuinely schema-invalid test fixtures or fix any validator rejecting valid YANG input) and check for R-2 duplicate BGP refusals on `ze config validate` (AC-5).
5. **Phase: Docs** -- update `docs/architecture/config/yang-config-design.md`, `docs/architecture/ospf/ospf-4-component-config.md`, `docs/architecture/bgp/filter-path-asn.md`, `docs/features/configuration.md`, and `ai/doors/config-yang.md`, and verify source anchors with `./le doc check all`.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-1..AC-5 has an implementation at file and symbol |
| Correctness | No valid shipped config is newly refused; every refusal names the section, leaf, and reason; `AddressFamilyValidator` names known families on refusal |
| Data flow | One walk (`validateCustomSectionsSchema`) serves both strict callers (`ValidateCustomSections`) and startup service-block isolation (`excludeInvalidServiceBlocks`), and SSH editor `NewConfigValidator` / `sectionErrors` enforces the same `bgp` `ze:validate` rules without double reporting |
| Rule: no-layering | No residual section allowlist or excuse map anywhere in `internal/component/config/` |
| Mandatory inheritance | `EverySessionCarries` still exempts global `bgp/router-id` and `bgp/session/asn/local` without dropping non-mandatory `bgp` failures or duplicating mandatory errors |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Hand lists and mandatory-only fallback deleted | `grep -rn "validatedSections\|knownUnwalkedValidatorSections\|mandatoryFailures\|bgpMandatoryFailures" internal/component/config/` returns nothing |
| Every declaring section walked | `./le test unit pkg path ./internal/component/config/infra/ run TestEveryValidatorSectionIsWalked` |
| Builtin and plugin families accepted and completed | `./le test unit pkg path ./internal/component/config/ run TestAddressFamilyValidator` |
| SSH editor enforces `bgp` `ze:validate` on peers and peer-less groups | `./le test unit pkg path ./internal/component/cli/ run TestEditor` |
| End-to-end functional proof | `./le test functional parse` (`test/parse/validate-walks-every-section.ci`) |
| Doc anchors valid | `./le doc check all` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | A sensitive leaf's refusal stays redacted (`SectionValidationError.Message`) in newly walked sections (including `environment`, `bgp`, `policy`, `static`, `control-plane-protection`, and `redistribute`), and `excludeInvalidServiceBlocks` continues storing `ve.Message()` rather than raw unredacted values |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Shipped config refused during measurement | Decide wrong fixture or wrong validator at the producer; never re-add an excuse map |
| Duplicate BGP refusal on `ze config validate` (R-2) | Deduplicate at the single producing boundary without weakening Layer 4 cross-plugin checks |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `gate-excludes-part-of-its-population.md` | 2026-09-02 | `validatedSections` hand list: `ze:validate` outside it never runs |
| `gate-excludes-part-of-its-population.md` | 2026-08-11 | section hand list beside the schema-derived sections (now `validatedSections`) |
| `gate-excludes-part-of-its-population.md` | 2026-08-11 | three validators answer from a registry not holding the asked value |

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Walk every section `(*Validator).Sections()` derives in `validateCustomSectionsSchema` | Keep `validatedSections` and widen it section by section | A hand list is a second declaration of the schema's sections (`ai/rules/principles.md`) and requires editing core config code whenever a new module lands |
| Switch `AddressFamilyValidator` to `registry.AllFamilies()` and keep `registry.FamilyMap()` plugin-only | Include builtin families in `registry.FamilyMap()` | `registry.FamilyMap()` is the plugin decode-routing map; `registry.AllFamilies()` already holds builtin plus plugin families |
| Wire `config.YANGValidator()` into `NewConfigValidator()` and keep non-duplicate `bgp` failures in `editorReportsSectionFailure` | Leave editor validator registry-less and drop all non-reference `bgp` failures | Prevents SSH editor `commit`/`commit verify` from silently accepting `bgp` `ze:validate` failures that `LoadConfig` and `ze config validate` refuse |
| Fold `mandatoryFailures` and `bgpMandatoryFailures` into the single `(*Validator).Sections()` loop while preserving `EverySessionCarries` | Keep a separate `bgp` mandatory pass beside the main walk | One walk per section prevents walking `bgp` twice, preserves the 2026-10-12 and 2026-10-16 owner inheritance rulings, and stops dropping non-mandatory `bgp` failures |

## Known Limitations
- Choice, `when`, and `unique` evaluation stay with `plan/release/fixes/spec-config-yang-when-unique-choice.md`.
- Typedef-level `ze:validate` extension propagation stays documented as a trap in `ai/doors/config-yang.md` (extensions attach to leaves/groupings rather than typedefs in goyang).

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
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints

### Goal Gates (MUST pass)
- [ ] AC-1..AC-5 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] Targeted tests pass, run in the main checkout by the main thread: `./le test unit pkg path ./internal/component/config/`, `./le test unit pkg path ./internal/component/config/infra/`, `./le test unit pkg path ./internal/component/cli/`, `./le test functional parse`, `./le test functional reload`, `./le test functional ui`, `./le test functional editor`, `./le test functional install`, `./le test functional plugin`, `./le doc check all`, and the suites `./le repo changed scope print both` reaches
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests N-A: no wire change

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] **Commit A:** code + tests + docs + edited spec + journal rows marked `fixed:`
- [ ] **Commit B:** `remove plan/release/fixes/spec-validation-walks-every-schema-section.md` only
