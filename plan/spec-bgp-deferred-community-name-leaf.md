# Spec: bgp-deferred-community-name-leaf

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | - |
| Phase | - |
| Updated | 2026-07-16 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `internal/component/config/yang_schema.go` - `ze:decorate` extraction and leaf vs leaf-list routing
4. `internal/component/web/fragment.go` - decorator name plumbed into the rendered field

## Task

Attach the working `community-name` web decorator to the BGP community config
leaf, so a well-known community value renders with its RFC 1997 name (for example
`65535:65281` shows as `no-export`).

**Provenance:** deferred from `plan/spec-followup-bgp-feature.md` item 4, recorded
2026-07-08 at that item's completion. That spec has since been closed and removed
from disk (commit `7f60301d1`, "spec: close followup-bgp-feature (all items done
or re-deferred)"), so this file is the only remaining home for the work.

**The recorded reason was wrong, and re-verification on 2026-07-16 replaced it.**
The deferral said the work was "blocked on a BGP YANG community leaf existing".
A community leaf does exist, and it predates the deferral:

| Half of the original claim | Verified status |
|---------------------------|-----------------|
| "decorator registered in `service_web.go` and functional" | HOLDS. Registered at `cmd/ze/hub/service_web.go` (`decorators.Register(zeweb.NewCommunityNameDecorator())`), implemented at `internal/component/web/decorator_community.go`, unit-tested in `internal/component/web/decorator_community_test.go` |
| "no community leaf exists in the BGP YANG to attach it to" | FALSE. `internal/component/bgp/yang/ze-bgp-conf.yang` declares `leaf-list community` under `list update > container attribute`. It landed 2026-06-08 in commit `8973d902d`, a month before the deferral was recorded |

**The real blocker is different, and it is why a naive fix fails silently.** The
decorator plumbing only supports a single-valued leaf, and the community leaf is a
leaf-list:

| Fact | Evidence |
|------|----------|
| `Decorate` is a field on `LeafNode` only | `internal/component/config/schema.go` |
| It is assigned in exactly one place, `yangToLeaf` | `internal/component/config/yang_schema.go` |
| `yangToLeaf` is only reached for a non-leaf-list leaf; a leaf-list returns `ValueOrArray`, a different node type with no `Decorate` field | `internal/component/config/yang_schema.go` |
| The web render reads `Decorate` off a `*config.LeafNode` | `internal/component/web/fragment.go` (`buildFieldMeta`) |
| Every leaf decorated today is a plain `leaf`, never a leaf-list | `ze-bgp-conf.yang`, `:312`, `:448`, `:453` (three `asn-name`, one `reverse-dns`) |

So adding `ze:decorate "community-name"` to the community leaf-list today would
parse, validate, and then be silently dropped: `getDecorateExtension` would never
be called for that entry. The work is to teach the decorator path about
leaf-lists (decorating each element), then attach the decorator. That a
`ze:decorate` on a leaf-list is accepted and ignored rather than rejected is
itself worth fixing per `ai/rules/evidence.md`.

### Landed 2026-08-05: the fail-open half only

The last sentence above is DONE and the rest of this spec is not. `yangToNode`
(`internal/component/config/yang_schema.go`) now refuses a `ze:decorate` on any
multi-valued leaf through `recordSchemaBuildError`, the accumulator `ze:related`
already used for errors that cannot travel the `yangTo*` signature chain. The
message names the decorator and the path. `isLeafListNode` decides which entries
count, mirroring the branches that build `MultiLeafNode`, `BracketLeafListNode`
and `ValueOrArrayNode`.

Refusing nothing that worked: no leaf-list in the tree carries `ze:decorate`. All
four uses (`asn-name` three times, `reverse-dns` once, all in
`internal/component/bgp/yang/ze-bgp-conf.yang`) are on plain leaves and are
unaffected, which `TestDecorateOnSingleLeafStillAccepted` pins.

Three tests, both claims mutation-verified. Deleting the guard fails
`TestDecorateOnLeafListRejectedAtLoad` on every syntax; refusing on leaf-list
shape without reading the decorator fails
`TestLeafListWithoutDecorateIsNotRejected`.

**What is still owed, and it is the whole point of this spec.** The decorator is
not attached and a community still renders as `65535:65281` rather than
`no-export`. Doing it needs, in order:

| Step | Where |
|------|-------|
| A `Decorate` field on the multi-valued node types | `internal/component/config/schema.go` |
| Populate it at the four leaf-list branches | `internal/component/config/yang_schema.go` |
| A render path that emits a field for a leaf-list and decorates EACH element | `internal/component/web/fragment.go`. `populateFragmentFields` today builds a `FieldMeta` for `*config.LeafNode` alone and produces nothing at all for a leaf-list, so this is a new path rather than an added field |
| `ze:decorate "community-name"` on the leaf-list | `internal/component/bgp/yang/ze-bgp-conf.yang` |
| Delete the guard above, and its three tests | `internal/component/config/yang_schema.go` |

The guard is deliberately the thing that stops step 4 landing on its own and
looking like it worked.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/web-interface.md` - decorator registry and display-time enrichment
  → Constraint: decorators resolve at render time by name; a missing decorator degrades gracefully rather than erroring
- [ ] `ai/rules/evidence.md` - a silently ignored `ze:decorate` is a guard that fails open
  → Constraint: an unsupported `ze:decorate` target should be reported, not dropped

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc1997.md` - BGP Communities Attribute, the well-known values the decorator names
  → Constraint: only well-known communities have a bare name; an ordinary community renders as `ASN:value` and needs no annotation

**Key insights:**
- `decorator_community.go`: the decorator returns "" for anything that still renders with a colon, so ordinary communities are left alone
- `decorator_community.go`: non-numeric or non-`ASN:value` input returns "" rather than an error (graceful degradation)
- The decorator needs no external resolver: the mapping is the in-process well-known registry (`decorator_community.go`)

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/web/decorator_community.go` - maps a standard community in `ASN:value` form to its well-known name; returns "" when there is none
- [ ] `cmd/ze/hub/service_web.go` - registers the community-name decorator into the render registry (lines 289-300)
- [ ] `internal/component/config/yang_schema.go` - routes leaf vs leaf-list (331-344); sets `Decorate` only in `yangToLeaf` (575); reads the extension in `getDecorateExtension` (397-406)
- [ ] `internal/component/config/schema.go` - `Decorate` field lives on `LeafNode` (line 139)
- [ ] `internal/component/web/fragment.go` - `buildFieldMeta` copies `leaf.Decorate` into `FieldMeta.DecoratorName` (447-454)
- [ ] `internal/component/bgp/yang/ze-bgp-conf.yang` - `leaf-list community` at line 237; existing decorated leaves at 88, 312, 448, 453

**Behavior to preserve:**
- The three `asn-name` leaves and the one `reverse-dns` leaf keep decorating exactly as today
- Graceful degradation: a value the decorator cannot parse renders undecorated, never errors
- An ordinary (non-well-known) community keeps rendering as `ASN:value` with no annotation
- The community leaf-list keeps accepting both a single value and bracket-list syntax (`ValueOrArray`, `yang_schema.go`)
- Decorators stay optional: a nil registry disables decoration (`render.go`)

**Behavior to change:**
- Decorator support extends to leaf-list nodes, annotating each element
- `ze:decorate "community-name"` attaches to the community leaf-list
- An unsupported `ze:decorate` target is reported at schema build rather than silently ignored

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- YANG schema build: a `ze:decorate` extension on a config entry, read by `getDecorateExtension` (`internal/component/config/yang_schema.go`)
- Web render of a config field carrying a value (the `bgp update attribute community` path)

### Transformation Path
1. The YANG parser produces a goyang entry for `leaf-list community` (`ze-bgp-conf.yang`)
2. `yangToSchemaNode` branches on kind: a leaf-list without `ze:syntax` becomes `ValueOrArray` and never reaches `yangToLeaf` (`yang_schema.go`)
3. For a plain leaf, `yangToLeaf` copies the extension argument into `node.Decorate` (`yang_schema.go`)
4. The web fragment builder copies `leaf.Decorate` into `FieldMeta.DecoratorName` for a `*config.LeafNode` (`fragment.go`)
5. The renderer resolves that name against the decorator registry and appends the annotation at display time (`render.go`, `render.go`)

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| YANG source → config schema | `ze:decorate` extension read into a schema node field | [ ] |
| Config schema → web fragment | `LeafNode.Decorate` copied to `FieldMeta.DecoratorName` | [ ] |
| Web fragment → decorator registry | Lookup by decorator name at render time | [ ] |
| Leaf-list schema node → decorator | (does not exist today; this spec must create it) | [ ] |

### Integration Points
- `internal/component/config/schema.go` - where a decorate-capable node field must live for leaf-lists
- `internal/component/config/yang_schema.go` - the leaf vs leaf-list routing that currently drops the extension
- `internal/component/web/fragment.go` - the `*config.LeafNode` signature that must also accept a leaf-list node
- `internal/component/web/decorator.go` - the `Decorator` interface, unchanged by this work

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable (uses refs, not copies)
- [ ] Registration over hardcoding: the decorator stays registry-registered and resolved by name; no per-decorator switch is added to the render path

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Decorating each element of a leaf-list is the desired UX | The decorator annotates one community value at a time (`decorator_community.go`) | A per-list summary annotation is wanted instead | User confirmation at pickup | unvalidated |
| A-2 | Only the standard community leaf-list needs this decorator | Large and extended communities use different formats the decorator returns "" for | Those leaves need their own decorators | Check the decorator against each format | unvalidated |
| A-3 | No other leaf-list in any YANG carries a `ze:decorate` today | Only four decorated entries exist, all plain leaves (`ze-bgp-conf.yang`, `:312`, `:448`, `:453`) | The silent-drop bug already affects shipped config surface | Grep `ze:decorate` across `internal/**/*.yang` | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Adding a `Decorate` field to a second node type duplicates state across schema nodes | The same field copied into three node structs | Prefer a shared accessor or interface over per-struct fields |
| R-2 | Making an unsupported `ze:decorate` a build error breaks an existing YANG | Schema build errors after the change | Audit every `ze:decorate` first; the four known entries are all plain leaves |

## Wiring Test (MANDATORY)
| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `ze:decorate` on a leaf-list in YANG | -> | Schema build carries the decorator name onto the leaf-list node | (fill during design) |
| Web render of a configured community value | -> | community-name annotation appears next to the value | (fill during design) |
| `ze:decorate` naming an unknown or unsupported target | -> | Schema build reports it instead of dropping it | (fill during design) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A well-known community (`65535:65281`) is configured and rendered | The value shows its RFC 1997 name (`no-export`) |
| AC-2 | An ordinary community (`65000:100`) is configured and rendered | The value shows with no annotation |
| AC-3 | Several communities are configured in one leaf-list | Each element is annotated independently |
| AC-4 | A `ze:decorate` names an unsupported target | The schema build reports it rather than ignoring it |
| AC-5 | The existing `asn-name` and `reverse-dns` leaves are rendered | Behavior is unchanged from today |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestLeafListCarriesDecorateExtension` | `internal/component/config/yang_schema_test.go` | AC-1: a `ze:decorate` on a leaf-list survives the schema build | |
| `TestFieldMetaDecoratesLeafListElements` | `internal/component/web/fragment_test.go` | AC-3: every element of a leaf-list gets its own annotation | |
| `TestDecorateUnsupportedTargetReported` | `internal/component/config/yang_schema_test.go` | AC-4: an unsupported target is reported, not dropped | |
| `TestExistingLeafDecoratorsUnchanged` | `internal/component/config/yang_schema_test.go` | AC-5: `asn-name` and `reverse-dns` still resolve | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `web-community-name-decorate` | `test/ui/*.ci` | Operator configures `no-export` on an update block and sees the well-known name in the web config view | |

### Interop Tests (MANDATORY for protocol features)
Not applicable: this is a display-time annotation with no wire protocol behavior.

## Files to Modify
- `internal/component/config/yang_schema.go` - carry `ze:decorate` onto leaf-list nodes; report unsupported targets
- `internal/component/config/schema.go` - decorate-capable leaf-list node
- `internal/component/web/fragment.go` - build field meta for a decorated leaf-list
- `internal/component/bgp/yang/ze-bgp-conf.yang` - attach `ze:decorate "community-name"` to the community leaf-list (line 237)
- `docs/architecture/web-interface.md` - document that decorators apply to leaf-lists

## Implementation Steps

1. **Phase: Audit.** Grep every `ze:decorate` across `internal/**/*.yang` and confirm which node kinds are targeted (R-2)
2. **Phase: Wiring (MANDATORY FIRST).** Write the failing test that a `ze:decorate` on a leaf-list survives the schema build
3. **Phase: Schema.** Carry the decorator name onto the leaf-list node; report unsupported targets instead of dropping them
4. **Phase: Render.** Annotate each leaf-list element in the web fragment builder
5. **Phase: Attach.** Add `ze:decorate "community-name"` to the community leaf-list
6. **Functional test** → prove the annotation appears in the web config view
7. **Full verification** → `make ze-verify`

## RFC Documentation

The decorator names RFC 1997 well-known communities. No new enforcing code is
added, so no new RFC constraint comments are expected beyond what
`decorator_community.go` already carries.

## Known Limitations
- Scope is the standard community leaf-list; large and extended communities have different formats and would need their own decorators (A-2)
- Display-time only: no wire behavior changes, so no interop coverage applies

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-5 all demonstrated
- [ ] Wiring Test table complete: every row has a concrete test name
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`)
- [ ] Registration over hardcoding verified
- [ ] Documentation Update Checklist answered Yes/No with source evidence

### Quality Gates (SHOULD pass, defer with user approval)
- [ ] Every `ze:decorate` in the repo audited against the supported node kinds
- [ ] `docs/architecture/web-interface.md` updated

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Functional tests for end-to-end behavior
