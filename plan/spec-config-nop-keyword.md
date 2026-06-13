# Spec: config-nop-keyword

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 3/3 |
| Updated | 2026-06-13 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `docs/architecture/config/syntax.md` - config format reference
4. `internal/component/config/serialize_set.go` - set-format serialization
5. `internal/component/config/setparser.go` - set-format parsing

## Task

Replace the two-line `set`+`inactive` deactivation model in the set-format config
with a single-line `set`/`nop` toggle. Every set-format line starts with either
`set`, `nop`, or `delete` (`set` and `nop` are both 3 characters). Toggling a
leaf's activation state is a 3-byte in-place edit with no line insertions,
deletions, or file size change. "nop" means "no operation": the config entry
exists in the file but produces no operational effect.

The hierarchical format's `inactive:` prefix and YANG tree representation are
unaffected. No changes to hierarchical parsing or presentation.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/config/syntax.md` - defines set format, inactive prefix, format detection
  -> Decision: set format uses `inactive <path>` as a separate line; hierarchical uses `inactive: <field>`
  -> Constraint: `DetectFormat` must recognize `nop` as a set-format keyword
  -> Constraint: hierarchical format `inactive:` is unchanged (user requirement)

**Key insights:**
- Current inactive model: `set <path> <value>` + separate `inactive <path>` line = 2 lines per deactivated leaf
- Proposed: `nop <path> <value>` = 1 line, same length as active `set` line, togglable by editing 3 bytes
- `inactive:` in hierarchical format and YANG tree representation are separate mechanisms, both unchanged

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/config/serialize_set.go` (~1020L) - emits `set <path> <value>` lines + `inactive <path>` lines for deactivated nodes. ~30 sites emit `"set "`, ~6 sites emit `"inactive "`. Functions: `emitSetInactive`, `emitSetInactiveStructural`, `emitInactiveMemberLines`, `splitInactiveMembers`. `FilterSetByPath` matches both `set` and `inactive` prefixes.
- [ ] `internal/component/config/setparser.go` (~440L) - parses `set`, `delete`, `inactive` commands. Constants: `cmdSet`, `cmdDelete`, `cmdInactive`. `parseInactive` handles leaf, container, list-entry, and leaf-list-member deactivation.
- [ ] `internal/component/config/setparser_meta.go` (~230L) - `parseLineWithMeta` dispatches same three commands; `cmdInactive` has no metadata of its own.
- [ ] `internal/component/config/serialize_annotated.go` (~850L) - annotated set-format serializer. 13 sites emit `"set "`. Hierarchical-format section uses `"inactive: "` (with colon, unchanged). Set-format section does not currently emit `inactive` lines.
- [ ] `internal/component/config/tree.go` (~120L) - `inactive bool` on Tree (container/list-entry level), `inactiveValues map[string]bool` (leaf level). `SetInactive`, `IsInactive`, `SetLeafInactive`, `IsLeafInactive`.
- [ ] `internal/component/config/change_file.go` (~210L) - `tb.Str("set ").Str(pc.Path)...` for change lines.

**Behavior to preserve:**
- Hierarchical format's `inactive:` prefix (unrelated mechanism, explicitly unchanged per user)
- YANG tree representation and parsing (explicitly unchanged per user)
- Tree-level `inactive` / `inactiveValues` internal model (unchanged)
- `delete` command (unchanged)
- Round-trip: parse -> tree -> serialize produces identical output
- Deactivated leaf-list members: per-member granularity
- Container/list-entry deactivation: marks subtree as non-operational

**Behavior to change:**
- Set-format serialization: emit `nop <path> <value>` instead of `set <path> <value>` + `inactive <path>`
- Set-format parsing: accept `nop` as a command that sets-and-deactivates in one step
- Backward compat: continue parsing `inactive` from old config files; on save, migrate to `nop`
- Leaf-list member deactivation: deactivated members become `nop <path> <member>` lines (individual lines, not bracket form)

## Data Flow (MANDATORY)

### Entry Point
- Config file on disk (set format) or CLI `set`/`nop` command

### Transformation Path
1. File read -> `SetParser.Parse()` or `SetParser.ParseWithMeta()` -> tokenize each line
2. First token dispatches: `set` -> `parseSet`, `delete` -> `parseDelete`, `nop` -> `parseNop` (new), `inactive` -> `parseInactive` (backward compat)
3. `parseNop` walks the schema path exactly like `parseSet`, then marks the resolved node inactive
4. Tree populated with values + inactive markers
5. On save: `SerializeSet` / `SerializeSetMeta` walks tree in schema order
6. At each leaf: if `tree.inactiveValues[name]`, emit `nop` prefix instead of `set`
7. At each container/list-entry: if `tree.IsInactive()`, structural `nop <container-path>` line emitted

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| File <-> Parser | text lines with `set`/`nop`/`delete` prefix | [ ] |
| Parser <-> Tree | `SetInactive()` / `SetLeafInactive()` calls | [ ] |
| Tree <-> Serializer | `IsInactive()` / `IsLeafInactive()` checks | [ ] |

### Integration Points
- `FilterSetByPath` - must match `nop` lines same as `set` lines
- `DetectFormat` - must recognize `nop` as a set-format keyword
- CLI compare/diff - must treat `set`<->`nop` as activation state change
- `change_file.go` - change lines may need `nop` prefix for deactivated changes

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable (uses refs, not copies)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | No external tool parses `inactive <path>` lines from Ze config files | Ze is the only consumer of its own config format | External tools break | User confirmation | unvalidated |
| A-2 | Leaf-list bracket form can be split to individual lines without semantic change | `ValueOrArrayNode` parsing accepts both forms | Round-trip produces different (but equivalent) output for deactivated leaf-lists | Test round-trip | unvalidated |
| A-3 | Container deactivation can be represented by a structural `nop <container-path>` line with children retaining their own `set`/`nop` state | Matches Junos `deactivate` semantics | Parse complexity | Design discussion | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Old config files with `inactive` lines must still load | Test with saved configs | Parser keeps `cmdInactive` as backward-compat path; remove after 2 releases |
| R-2 | Bracket-form leaf-lists with per-member deactivation must serialize as individual lines, changing output format | Diff test fails | Accept format change for deactivated members only; active-only lists keep bracket form |

## Key Design Decisions

### D-1: Keyword choice

Chose `nop` over `off`, `rem`, `nil`, `not`, `del`.

`nop` = "no operation". The config line exists but produces no operational effect.
Precise semantics, familiar to network engineers, no collision with existing keywords.
`off` was considered but `nop` is more precise: the line is not "off" (removed), it
is present but inert. `del` collides with `delete`. `not` suggests logical negation.
`rem` means "remark" (comment), which is a different concept.

### D-2: Leaf deactivation

Current:
```
set bgp router-id 10.0.0.1
inactive bgp router-id
```

Proposed:
```
nop bgp router-id 10.0.0.1
```

One line. Toggle by editing bytes 0-2.

### D-3: Leaf-list member deactivation

Current:
```
set system name-server [ 8.8.8.8 1.1.1.1 ]
inactive system name-server 1.1.1.1
```

Proposed:
```
set system name-server 8.8.8.8
nop system name-server 1.1.1.1
```

Each member on its own line. Active members use `set`, deactivated members use `nop`.
Bracket form is only used when all members are active (no deactivated members).

### D-4: Container/list-entry deactivation (NEEDS USER INPUT)

Option A: Structural `nop` marker line + children retain their own `set`/`nop` state:
```
nop bgp
set bgp router-id 10.0.0.1
set bgp neighbor 192.0.2.1 peer-as 65001
```
Pro: activating one child is just changing that child to `set`. Deactivating
the whole container is changing the structural line. Matches Junos semantics.
Con: `nop bgp` has no value (structural-only line).

Option B: All descendant lines become `nop`:
```
nop bgp router-id 10.0.0.1
nop bgp neighbor 192.0.2.1 peer-as 65001
```
Pro: every line is self-describing, no structural marker needed.
Con: toggling a container means editing every descendant line. Activating
one child while container is deactivated is ambiguous.

Recommendation: Option A. The structural `nop <container-path>` line is the
activation toggle. Children retain their own `set`/`nop` state independently.

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| Config file with `nop` lines | -> | `SetParser.parseLine` dispatches to `parseNop` | `TestParseNopLeaf` |
| Tree with inactive leaf | -> | `serializeSetChild` emits `nop` prefix | `TestSerializeSetNopLeaf` |
| Old config with `inactive` lines | -> | `SetParser.parseLine` dispatches to `parseInactive` (backward compat) | `TestParseInactiveBackwardCompat` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `nop bgp router-id 10.0.0.1` parsed | Tree has `router-id` = `10.0.0.1` marked inactive |
| AC-2 | Tree with inactive leaf serialized | Output line is `nop bgp router-id 10.0.0.1` (no separate `inactive` line) |
| AC-3 | `nop` line parsed + serialized | Round-trip produces identical output |
| AC-4 | Old `set` + `inactive` config loaded | Parses correctly (backward compat) |
| AC-5 | Old `set` + `inactive` config loaded, then saved | Output uses `nop` lines (migration on save) |
| AC-6 | Leaf-list with one deactivated member | Active members as `set` lines, deactivated as `nop` lines (individual lines) |
| AC-7 | `DetectFormat` on file containing `nop` lines | Returns `FormatSet` |
| AC-8 | `FilterSetByPath` with `nop` lines | Matches `nop` lines same as `set` lines |
| AC-9 | Container marked inactive, serialized | Structural `nop <container-path>` line emitted; children retain their own `set`/`nop` |
| AC-10 | Annotated/meta format with inactive nodes | Uses `nop` prefix instead of `set` + separate `inactive` |

## Files to Modify
- `internal/component/config/serialize_set.go` - replace `emitSetInactive`/`emitSetInactiveStructural`/`emitInactiveMemberLines` with inline `nop`/`set` dispatch at every emission site
- `internal/component/config/setparser.go` - add `cmdNop` constant and `parseNop` handler
- `internal/component/config/setparser_meta.go` - add `cmdNop` dispatch in `parseLineWithMeta`
- `internal/component/config/serialize_annotated.go` - `nop`/`set` dispatch at set-format emission sites only
- `internal/component/config/change_file.go` - handle `nop` prefix for deactivated changes
- `docs/architecture/config/syntax.md` - document `nop` keyword, update set-format examples

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs/config) | No | N/A (set-format only, YANG unchanged) |
| CLI commands/flags | Maybe | CLI `deactivate`/`activate` commands may show `nop` in output |
| Editor autocomplete | No | N/A |
| Functional test for new RPC/API | No | N/A |
| Doc updates | Yes | `docs/architecture/config/syntax.md` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | - |
| 2 | Config syntax changed? | Yes | `docs/architecture/config/syntax.md` - document `nop` keyword in set-format section |
| 3 | CLI command added/changed? | No | - |
| 7 | Wire format changed? | No | - |

## Files to Create
- None (all changes are to existing files)
- Test additions go in existing `*_test.go` files

## Known Limitations
- Bracket-form leaf-lists with per-member deactivation must serialize as individual lines (format change, semantically equivalent)
- Backward compat: `inactive` keyword parsed but never emitted in set format. One-way migration on save.
- Hierarchical format's `inactive:` prefix and YANG tree representation are unchanged (explicit user requirement)
