# Spec: Text Command Format Unification

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `docs/architecture/api/commands.md` - command vocabulary and dispatch
4. `docs/architecture/api/text-format.md` - unified grammar reference
5. `internal/plugins/bgp/handler/update_text.go` - main command parser
6. `internal/plugins/bgp/textparse/scanner.go` - shared TextScanner

## Task

Refactor the `update text` command parser to a flat-attribute grammar with keyword aliases. Remove the accumulator model (set/add/del on attributes, mid-stream modification). Introduce short/long keyword aliases so API output is compact and config output is readable.

Parent spec: `spec-utp-0-umbrella.md`.
Depends on: `spec-utp-1-event-format.md` (completed: `docs/plan/done/302-utp-1-event-format.md`).
Next: `spec-utp-3-handshake.md`.

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x] -- checkboxes are template markers, not progress trackers. -->
- [ ] `docs/architecture/api/text-format.md` - unified grammar reference
  -> Decision: text format uses uniform header "peer <addr> asn <n> ..." with comma-separated lists, no brackets
  -> Constraint: event parser uses TextScanner from textparse/ -- keyword tables should be shared
- [ ] `docs/architecture/api/text-parser.md` - shared TextScanner design
  -> Decision: TextScanner is zero-alloc, whitespace-delimited, no quote handling
  -> Constraint: quote handling stays in command.go:tokenize(), not in TextScanner
- [ ] `docs/architecture/api/process-protocol.md` - command dispatch architecture
  -> Constraint: commands enter via Dispatch() which calls tokenize() then longest-match handler lookup
  -> Constraint: handler signature is func(ctx *CommandContext, args []string) (*Response, error) -- args are pre-tokenized
- [ ] `docs/architecture/api/commands.md` - command vocabulary
  -> Constraint: target-first syntax: "bgp peer <selector> update text ..."
  -> Constraint: update dispatches by encoding: text/hex/b64

### Source Files
- [ ] `internal/plugins/bgp/handler/update_text.go` (1357L) - accumulator parser
  -> Constraint: ParseUpdateText(args []string) is the entry point, returns *UpdateTextResult
  -> Constraint: parsedAttrs struct has applySet/applyAdd/applyDel -- these are being REMOVED
  -> Constraint: parseCommonAttributeText() parses attribute values by keyword -- KEEP this, just remove mode handling
  -> Constraint: parseBracketedListText() handles 4 bracket styles -- KEEP for transition (AC-3), simplify later
  -> Constraint: snapshot() builds wire-format attributes via Builder -- KEEP, called once instead of per-section
- [ ] `internal/plugins/bgp/handler/update_text_nlri.go` (341L) - NLRI section parsing
  -> Constraint: parseNLRISection() handles in-NLRI modifiers (rd, label without 'set') -- already aligned
  -> Constraint: parseNLRI() dispatches by family SAFI -- KEEP as-is
  -> Constraint: "prefix" keyword already accepted as skip token (line 173)
- [ ] `internal/plugins/bgp/handler/update_text_evpn.go` (357L) - EVPN parsing
  -> Constraint: keyword-value parsing (mac, ip, prefix, esi, etag, gateway, rd, label) -- no set/add/del on these
- [ ] `internal/plugins/bgp/handler/update_text_flowspec.go` (165L) - FlowSpec parsing
  -> Constraint: component tokens collected until boundary, passed to registry encoder -- no set/add/del
- [ ] `internal/plugins/bgp/handler/update_text_vpls.go` (168L) - VPLS parsing
  -> Constraint: keyword-value parsing (rd, ve-id, ve-block-offset, ve-block-size, label-base) -- no set/add/del
- [ ] `internal/plugin/command.go` (452L) - tokenizer + dispatcher
  -> Constraint: tokenize() handles quoted strings and backslash escaping -- KEEP in command.go
  -> Constraint: Dispatch() does peer selector extraction then longest-match lookup -- unchanged
- [ ] `internal/plugins/bgp/handler/update_wire.go` - hex/b64 mode boundary
  -> Constraint: kwSelf = "self" defined here -- used by nhop parsing
- [ ] `internal/plugins/bgp/textparse/scanner.go` (55L) - shared TextScanner
  -> Constraint: whitespace-delimited, no quote handling, zero-alloc
- [ ] `internal/plugins/bgp-rs/server.go` (1250+ lines) - event parser
  -> Constraint: topLevelKeywords map (line 1022) defines keyword boundaries -- should move to shared package
  -> Constraint: parseTextNLRIOps() uses key-dispatch loop with TextScanner -- pattern model for command parser
- [ ] `internal/plugins/bgp/handler/update_text_test.go` (3928L) - test suite
  -> Constraint: 130+ test functions all use old set/add/del syntax -- ALL must be migrated

**Key insights:**
- Command parser operates on pre-tokenized []string from command.go:tokenize(). Event parser uses TextScanner on raw strings. These serve different needs (quotes vs no-quotes) and should NOT be unified into one tokenizer.
- The real sharing opportunity is keyword tables (what's a boundary keyword, what's an attribute keyword) and the NLRI parsing pattern (family -> optional modifiers -> add/del -> collect NLRIs).
- The accumulator model (set/add/del on attributes, mid-stream modification between NLRI sections) is being REMOVED. Attributes are flat declarations, add/del only for NLRI (MP_REACH vs MP_UNREACH).
- Short/long keyword aliases enable compact API output and readable config output.
- EVPN, FlowSpec, and VPLS parsers already use flat keyword-value parsing internally -- they don't use set/add/del on their section-local keywords. The main change is at the top level.

## Current Behavior (MANDATORY)

**Source files read:** (all read during research)
- [ ] `handler/update_text.go` - accumulator model with set/add/del on attributes
- [ ] `handler/update_text_nlri.go` - NLRI section parsing with path-information accumulator
- [ ] `handler/update_text_evpn.go` - EVPN flat keyword-value parsing (already aligned)
- [ ] `handler/update_text_flowspec.go` - FlowSpec component parsing (already aligned)
- [ ] `handler/update_text_vpls.go` - VPLS flat keyword-value parsing (already aligned)
- [ ] `plugin/command.go` - tokenizer with quotes, dispatcher
- [ ] `bgp-rs/server.go` - event parser with TextScanner and topLevelKeywords

**Behavior to preserve:**
- `nlri <family> add <prefix>+` and `nlri <family> del <prefix>+` syntax
- In-NLRI modifiers: `rd <value>`, `label <value>` (without 'set') inside nlri sections
- `nlri <family> eor` for End-of-RIB markers (RFC 4724)
- `watchdog <name>` in NLRI sections
- EVPN route-type parsing: `mac-ip`, `ip-prefix`, `multicast` with field keywords
- FlowSpec component parsing: `destination`, `source`, `protocol`, `then` etc.
- VPLS field parsing: `ve-id`, `ve-block-offset`, `ve-block-size`, `label-base`
- `tokenize()` quote handling in command.go
- `handleUpdate()` dispatch to text/hex/b64 handlers
- `DispatchNLRIGroups()` reactor interface
- All existing functional tests in `test/`

**Behavior to change:**
- Remove `set`/`add`/`del` keywords on attributes: `origin set igp` -> `origin igp`
- Remove mid-stream attribute modification: attributes declared once before NLRI sections
- Remove `parsedAttrs.applySet/applyAdd/applyDel` accumulator methods
- Remove `Clear*` and `Del*Expected` fields from parsedAttrs
- Remove `parsePerAttributeSection()` mode handling
- Remove top-level `path-information set <id>` accumulator
- Rename `nhop` to accept `next-hop`/`next` (alias system)
- Add keyword alias resolution
- Add `path-id` as per-NLRI-section modifier (already works like rd/label in-NLRI modifiers)
- List format: comma-separated primary, brackets accepted for transition

## Data Flow (MANDATORY)

### Entry Point
- CLI: `ze cli --run "bgp peer * update text origin igp next 1.2.3.4 nlri ipv4/unicast add 10.0.0.0/24"`
- Plugin stdout: same text line read by engine

### Transformation Path
1. `command.go:tokenize()` splits into []string (handles quoted strings)
2. `Dispatch()` extracts peer selector, finds handler by longest-match
3. `handleUpdate()` dispatches by encoding (text/hex/b64)
4. `handleUpdateText()` calls `ParseUpdateText(args)`
5. `ParseUpdateText()`: resolve aliases -> parse flat attributes -> parse NLRI sections -> build UpdateTextResult
6. `DispatchNLRIGroups()` calls reactor AnnounceNLRIBatch/WithdrawNLRIBatch
7. Reactor builds wire UPDATE, sends to peers

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| CLI -> JSON-RPC | `ze-bgp:peer-update` method | [ ] |
| Plugin -> engine | stdout text line -> Dispatch() | [ ] |
| Command parser -> reactor | AnnounceNLRIBatch/WithdrawNLRIBatch | [ ] |
| Keyword alias -> canonical | textparse.ResolveAlias() at parse entry | [ ] |

### Integration Points
- `textparse/keywords.go` (NEW) consumed by handler/update_text.go and bgp-rs/server.go
- `ParseUpdateText()` consumed by handleUpdateText() in same package
- `parsedAttrs.snapshot()` consumed by ParseUpdateText() for wire attribute building
- `format/text.go` uses alias table for short-form output

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (textparse/ is leaf package, no handler imports)
- [ ] No duplicated functionality (keyword tables shared, not duplicated)
- [ ] Zero-copy preserved where applicable (attribute builder pattern unchanged)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `bgp peer * update text origin igp next 1.2.3.4 nlri ...` | -> | ParseUpdateText with alias resolution | TestParseUpdateText_FlatAttributes |
| `bgp peer * update text path 65001,65002 nlri ...` | -> | Alias `path` resolved to `as-path` | TestParseUpdateText_ShortAlias_Path |
| `bgp peer * update text s-com 65001:100 nlri ...` | -> | Alias `s-com` resolved to `community` | TestParseUpdateText_ShortAlias_Community |
| `bgp peer * update text nlri ipv4/unicast path-id 42 add 10.0.0.0/24` | -> | path-id as per-section modifier | TestParseUpdateText_PathIDModifier |
| `bgp peer * update text origin set igp ...` (old syntax) | -> | Error with migration hint | TestParseUpdateText_RejectSetKeyword |
| `bgp peer * update text as-path [65001 65002] nlri ...` (old brackets) | -> | parseBracketedListText still works | TestParseUpdateText_BracketCompat |
| handleUpdateText -> reactor | -> | DispatchNLRIGroups | TestHandleUpdateText_SimpleAnnounce (existing, updated) |
| Event formatter | -> | short aliases in output | TestFormatTextUpdate_ShortAliases |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `next` or `next-hop` keyword | Both accepted; API prints `next`, config prints `next-hop` |
| AC-2 | `path 65001,65002` (short alias + comma list) | Parsed as AS_PATH [65001, 65002] |
| AC-3 | `as-path [65001 65002]` (old brackets) | Still accepted during transition |
| AC-4 | `nlri ipv4/unicast add 10.0.0.0/24` | Creates MP_REACH_NLRI (unchanged) |
| AC-5 | `nlri ipv4/unicast path-id 42 add 10.0.0.0/24` | Path-ID 42 applied to NLRI |
| AC-6 | All short aliases: `next`, `pref`, `path`, `s-com`, `l-com`, `e-com`, `path-id` | Accepted and resolved to canonical form |
| AC-7 | `origin igp next 1.2.3.4 nlri ...` (flat, no `set`) | Parsed correctly |
| AC-8 | `origin set igp` (old `set` syntax) | Rejected with error: "set keyword removed; use: origin igp" |
| AC-9 | Attributes after first NLRI section | Rejected: "attributes must precede all nlri sections" |
| AC-10 | `route-distinguisher` (long form of `rd`) | Accepted, resolved to `rd` |
| AC-11 | `short-community` (consistency alias) | Accepted, resolved to `community` |
| AC-12 | Existing functional tests | All pass after grammar migration |
| AC-13 | Event text formatter | Uses short aliases: `next`, `path`, `pref`, `s-com`, `l-com`, `e-com` |

## Keyword Alias Table

All keywords are always lowercase.

| Long (config prints) | Short (API prints) | Also accepts | Notes |
|---|---|---|---|
| `next-hop` | `next` | both | was `nhop` |
| `local-preference` | `pref` | both | |
| `as-path` | `path` | both | |
| `community` | `s-com` | `short-community` | consistency alias |
| `large-community` | `l-com` | both | |
| `extended-community` | `e-com` | both | |
| `path-information` | `path-id` | both | per-section modifier now |
| `route-distinguisher` | `rd` | both | `rd` always printed |

Already short (no alias): `origin`, `med`, `rd`, `label`, `nlri`, `watchdog`

### Grammar

Supersedes the old accumulator grammar.

| Production | Definition |
|---|---|
| update-text | attribute* nlri-section+ |
| attribute | attr-name value |
| attr-name | `origin` / `next-hop` / `next` / `as-path` / `path` / `med` / `local-preference` / `pref` / `community` / `s-com` / `large-community` / `l-com` / `extended-community` / `e-com` / (and all other accepted aliases) |
| value | single token or comma-separated list |
| nlri-section | `nlri` family [path-id id] nlri-ops |
| nlri-ops | (add nlri-entries)+ / (del nlri-entries)+ |
| nlri-entries | (per-family NLRI format: prefix for INET, keyword-value for EVPN/FlowSpec/VPLS) |

**add/del are NLRI-only keywords** mapping to MP_REACH_NLRI (announce) / MP_UNREACH_NLRI (withdraw). They do NOT apply to attributes.

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| TestParseUpdateText_FlatAttributes | handler/update_text_test.go | AC-7: flat origin/next-hop/as-path without set | |
| TestParseUpdateText_RejectSetKeyword | handler/update_text_test.go | AC-8: set keyword produces error with hint | |
| TestParseUpdateText_RejectMidStreamAttrs | handler/update_text_test.go | AC-9: attrs after nlri section rejected | |
| TestParseUpdateText_ShortAlias_Next | handler/update_text_test.go | AC-1: `next` resolved to next-hop | |
| TestParseUpdateText_ShortAlias_Path | handler/update_text_test.go | AC-2: `path 65001,65002` parsed correctly | |
| TestParseUpdateText_ShortAlias_Pref | handler/update_text_test.go | AC-6: `pref` resolved to local-preference | |
| TestParseUpdateText_ShortAlias_SCom | handler/update_text_test.go | AC-6: `s-com` resolved to community | |
| TestParseUpdateText_ShortAlias_LCom | handler/update_text_test.go | AC-6: `l-com` resolved to large-community | |
| TestParseUpdateText_ShortAlias_ECom | handler/update_text_test.go | AC-6: `e-com` resolved to extended-community | |
| TestParseUpdateText_LongAlias_RD | handler/update_text_test.go | AC-10: `route-distinguisher` accepted | |
| TestParseUpdateText_ShortCommunityAlias | handler/update_text_test.go | AC-11: `short-community` accepted | |
| TestParseUpdateText_BracketCompat | handler/update_text_test.go | AC-3: old brackets still parsed | |
| TestParseUpdateText_PathIDModifier | handler/update_text_test.go | AC-5: path-id as per-section modifier | |
| TestResolveAlias | textparse/keywords_test.go | Alias table correctness | |
| TestIsAttributeKeyword | textparse/keywords_test.go | Keyword classification | |
| TestFormatTextUpdate_ShortAliases | format/text_test.go | AC-13: event output uses short forms | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| path-id | 0-4294967295 | 4294967295 | N/A (uint32) | 4294967296 |
| label | 0-1048575 | 1048575 | N/A | 1048576 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| test-update-text-flat | test/encode/*.ci | CLI sends flat-attribute update, routes announced | |
| test-update-text-alias | test/encode/*.ci | CLI sends short-alias update, routes announced | |
| test-update-text-reject-set | test/parse/*.ci | Old set syntax produces clear error | |

### Future (deferred)
- Property test: roundtrip event format -> command re-parse (requires utp-3 alignment)

## Files to Modify
- `internal/plugins/bgp/handler/update_text.go` - remove accumulator model, flat attributes, alias resolution
- `internal/plugins/bgp/handler/update_text_nlri.go` - add path-id modifier, update boundary keywords
- `internal/plugins/bgp/handler/update_text_evpn.go` - minor: remove mode variable if present at top level
- `internal/plugins/bgp/handler/update_text_flowspec.go` - minor: same
- `internal/plugins/bgp/handler/update_text_vpls.go` - minor: same
- `internal/plugins/bgp/handler/update_text_test.go` - migrate all tests to new grammar
- `internal/plugins/bgp/format/text.go` - use short aliases in API output
- `internal/plugins/bgp-rs/server.go` - import shared keyword tables from textparse/
- `docs/architecture/api/commands.md` - update examples and grammar
- `docs/architecture/api/text-format.md` - update format reference

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | No | N/A (no new RPCs) |
| RPC count in architecture docs | No | N/A |
| CLI commands/flags | No | dispatch unchanged |
| CLI usage/help text | No | N/A |
| API commands doc | Yes | `docs/architecture/api/commands.md` |
| Plugin SDK docs | No | N/A |
| Functional test for new RPC/API | Yes | `test/encode/*.ci` |

## Files to Create
- `internal/plugins/bgp/textparse/keywords.go` - keyword constants, alias map, resolution function
- `internal/plugins/bgp/textparse/keywords_test.go` - alias and keyword tests
- `test/encode/update-text-flat.ci` - functional test for flat grammar
- `test/encode/update-text-alias.ci` - functional test for short aliases

## Implementation Steps

Each step ends with a **Self-Critical Review**. Fix issues before proceeding.

### Phase 1: Shared keyword infrastructure
1. Create `textparse/keywords.go` with alias table and ResolveAlias function
2. Write tests in `textparse/keywords_test.go` for alias resolution
3. Run tests -> verify PASS

### Phase 2: Remove accumulator model
4. Write new tests for flat grammar (TestParseUpdateText_FlatAttributes, TestParseUpdateText_RejectSetKeyword, TestParseUpdateText_RejectMidStreamAttrs)
5. Run tests -> verify FAIL
6. Refactor ParseUpdateText(): remove set/add/del on attributes, flat parsing
7. Remove parsedAttrs.applySet/applyAdd/applyDel, Clear*, Del*Expected fields
8. Remove parsePerAttributeSection() mode handling -- simplify to flat keyword-value
9. Run new tests -> verify PASS

### Phase 3: Alias resolution
10. Write alias tests (TestParseUpdateText_ShortAlias_*)
11. Run -> verify FAIL
12. Add alias resolution at ParseUpdateText entry (resolve each token before matching)
13. Rename nhop references to use next-hop/next alias system
14. Run -> verify PASS

### Phase 4: Path-ID modifier
15. Write TestParseUpdateText_PathIDModifier
16. Run -> verify FAIL
17. Move path-id to per-NLRI-section modifier (like existing rd/label in-NLRI modifiers)
18. Remove parsePathInfoSection (top-level accumulator)
19. Run -> verify PASS

### Phase 5: Migrate existing tests
20. Update all 130+ existing tests to new flat grammar (batch by category)
21. Run `make ze-unit-test` after each batch
22. Verify all tests pass

### Phase 6: Event formatter
23. Write TestFormatTextUpdate_ShortAliases
24. Run -> verify FAIL
25. Update format/text.go to use short aliases in API text output
26. Run -> verify PASS

### Phase 7: Shared keyword tables in bgp-rs
27. Update bgp-rs/server.go to import topLevelKeywords from textparse/
28. Remove local keyword map definition
29. Run `make ze-verify`

### Phase 8: Documentation and functional tests
30. Update docs/architecture/api/commands.md
31. Update docs/architecture/api/text-format.md
32. Create functional tests
33. Run `make ze-verify` -- paste FULL output

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix syntax/types in current phase |
| New test fails wrong reason | Fix test, re-run |
| Existing test fails after migration | Check grammar translation, fix test or parser |
| Functional test fails | Check AC; if AC wrong -> redesign; if correct -> fix impl |
| bgp-rs event parsing breaks | Check shared keyword table compatibility |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights
<!-- LIVE -- write IMMEDIATELY when you learn something -->
- The accumulator model (set/add/del on attributes) was overengineered. Attributes are declared once; only NLRI operations need add/del (MP_REACH vs MP_UNREACH).
- Short/long aliases are a general pattern: API output is compact for wire speed, config output is readable for humans. Both contexts accept both forms.
- The "shared tokenizer" from the umbrella spec should be "shared keyword tables" -- TextScanner and tokenize() serve fundamentally different needs (raw string scanning vs quoted input splitting).
- EVPN, FlowSpec, and VPLS parsers already use flat keyword-value parsing internally. The accumulator model only existed at the top level.

## RFC Documentation

No new RFC constraints. Existing RFC references (RFC 4271 UPDATE, RFC 4724 EOR, RFC 4760 MP extensions, RFC 8955 FlowSpec, RFC 7432 EVPN, RFC 4761 VPLS) are preserved in the family-specific parsers.

## Implementation Summary

### What Was Implemented
- Removed accumulator model (set/add/del on attributes) from ParseUpdateText — flat attribute grammar
- Created `textparse/keywords.go` with shared keyword constants, alias tables, and resolution functions
- Added alias resolution to command parser (short/long/legacy forms all accepted)
- Moved path-id from top-level accumulator to per-NLRI-section modifier
- Updated event formatter (`format/text.go`) to output short aliases (`path`, `next`, `pref`, `s-com`, `l-com`, `e-com`)
- Updated event parser (`bgp-rs/server.go`) to use shared keyword tables from `textparse/`
- Updated `FormatRouteCommand()` in `internal/plugin/bgp/shared/format.go` to use flat grammar
- Migrated all existing tests (unit and functional) to new grammar
- Created 2 new functional tests: `update-text-flat.ci` and `update-text-alias.ci`

### Bugs Found/Fixed
- `FormatRouteCommand()` in `shared/format.go` still used old `set` syntax — caused plugin route replay failures
- Python plugin scripts in `test/plugin/*.ci` `tmpfs=` blocks still used `set` — missed in initial batch migration
- Unused `testHasMED()` function in test file flagged by linter after test migration

### Documentation Updates
- `docs/architecture/api/commands.md` — updated all examples to flat grammar, added keyword aliases table
- `docs/architecture/api/text-format.md` — updated BNF grammar, attribute formats table, examples; reorganized "Proposed" section

### Deviations from Plan
- `test-update-text-reject-set` functional test not created (unit test `TestParseUpdateText_RejectSetKeyword` covers AC-8; test framework doesn't easily test API error responses)
- `FormatRouteCommand()` update not in original spec (discovered during functional testing — internal producer must match parser changes)

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Remove set/add/del on attributes | ✅ Done | handler/update_text.go:548 | Error with migration hint |
| Flat attribute parsing | ✅ Done | handler/update_text.go:530-564 | keyword + value, no mode |
| Keyword alias resolution | ✅ Done | handler/update_text.go:525 | textparse.ResolveAlias() |
| path-id as per-NLRI modifier | ✅ Done | handler/update_text_nlri.go:76 | Inside parseNLRISection |
| Shared keyword tables | ✅ Done | textparse/keywords.go | Consumed by handler, format, bgp-rs |
| Event formatter short aliases | ✅ Done | format/text.go:formatAttributeText() | Uses textparse.Alias* constants |
| bgp-rs shared keywords | ✅ Done | bgp-rs/server.go | Removed local maps, uses textparse.* |
| Migrate existing tests | ✅ Done | handler/update_text_test.go + 64 .ci files | All 130+ tests migrated |
| Documentation updates | ✅ Done | commands.md, text-format.md | BNF, examples, alias table |
| FormatRouteCommand flat grammar | ✅ Done | shared/format.go | Discovered during testing |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | ✅ Done | TestParseUpdateText_ShortAlias_Next | `next` and `next-hop` both accepted |
| AC-2 | ✅ Done | TestParseUpdateText_ShortAlias_Path | `path 65001,65002` parsed correctly |
| AC-3 | ✅ Done | TestParseUpdateText_BracketCompat | Old brackets still accepted |
| AC-4 | ✅ Done | TestParseUpdateText_FlatAttributes (existing) | `nlri ipv4/unicast add` unchanged |
| AC-5 | ✅ Done | TestParseUpdateText_PathIDModifier | path-id as per-section modifier |
| AC-6 | ✅ Done | TestParseUpdateText_ShortAlias_* (6 tests) | All short aliases accepted |
| AC-7 | ✅ Done | TestParseUpdateText_FlatAttributes | Flat origin/next-hop without set |
| AC-8 | ✅ Done | TestParseUpdateText_RejectSetKeyword | Error with migration hint |
| AC-9 | ✅ Done | TestParseUpdateText_RejectMidStreamAttrs | Attrs after nlri rejected |
| AC-10 | ✅ Done | TestParseUpdateText_LongAlias_RD | `route-distinguisher` accepted |
| AC-11 | ✅ Done | TestParseUpdateText_ShortCommunityAlias | `short-community` accepted |
| AC-12 | ✅ Done | make ze-verify (all pass) | All existing tests pass |
| AC-13 | ✅ Done | TestFormatTextUpdate_ShortAliases | Short aliases in event output |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| TestParseUpdateText_FlatAttributes | ✅ Done | handler/update_text_test.go | AC-7 |
| TestParseUpdateText_RejectSetKeyword | ✅ Done | handler/update_text_test.go | AC-8 |
| TestParseUpdateText_RejectMidStreamAttrs | ✅ Done | handler/update_text_test.go | AC-9 |
| TestParseUpdateText_ShortAlias_Next | ✅ Done | handler/update_text_test.go | AC-1 |
| TestParseUpdateText_ShortAlias_Path | ✅ Done | handler/update_text_test.go | AC-2 |
| TestParseUpdateText_ShortAlias_Pref | ✅ Done | handler/update_text_test.go | AC-6 |
| TestParseUpdateText_ShortAlias_SCom | ✅ Done | handler/update_text_test.go | AC-6 |
| TestParseUpdateText_ShortAlias_LCom | ✅ Done | handler/update_text_test.go | AC-6 |
| TestParseUpdateText_ShortAlias_ECom | ✅ Done | handler/update_text_test.go | AC-6 |
| TestParseUpdateText_LongAlias_RD | ✅ Done | handler/update_text_test.go | AC-10 |
| TestParseUpdateText_ShortCommunityAlias | ✅ Done | handler/update_text_test.go | AC-11 |
| TestParseUpdateText_BracketCompat | ✅ Done | handler/update_text_test.go | AC-3 |
| TestParseUpdateText_PathIDModifier | ✅ Done | handler/update_text_test.go | AC-5 |
| TestResolveAlias | ✅ Done | textparse/keywords_test.go | Alias table correctness |
| TestIsAttributeKeyword | ✅ Done | textparse/keywords_test.go | Keyword classification |
| TestFormatTextUpdate_ShortAliases | ✅ Done | format/text_test.go | AC-13 |
| update-text-flat.ci | ✅ Done | test/encode/update-text-flat.ci | Flat grammar end-to-end |
| update-text-alias.ci | ✅ Done | test/encode/update-text-alias.ci | Short aliases end-to-end |
| test-update-text-reject-set | ❌ Skipped | — | Unit test covers AC-8; functional test framework doesn't support API error assertions |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| handler/update_text.go | ✅ Modified | Removed accumulator, flat parsing, alias resolution |
| handler/update_text_nlri.go | ✅ Modified | path-id per-section modifier |
| handler/update_text_test.go | ✅ Modified | All tests migrated + new tests |
| format/text.go | ✅ Modified | Short aliases in output |
| format/text_test.go | ✅ Modified | New alias tests + updated expectations |
| bgp-rs/server.go | ✅ Modified | Shared keyword tables |
| docs/architecture/api/commands.md | ✅ Modified | Updated examples, alias table |
| docs/architecture/api/text-format.md | ✅ Modified | BNF, formats, examples |
| textparse/keywords.go | ✅ Created | Shared keyword infrastructure |
| textparse/keywords_test.go | ✅ Created | Alias and keyword tests |
| test/encode/update-text-flat.ci | ✅ Created | Flat grammar functional test |
| test/encode/update-text-alias.ci | ✅ Created | Short alias functional test |
| shared/format.go | 🔄 Changed | Not in plan — discovered during testing |
| shared/format_test.go | 🔄 Changed | Updated for flat grammar |
| bgp-rib/rib_test.go | 🔄 Changed | Updated replay test expectations |
| 64 .ci test files | ✅ Modified | Migrated from set syntax |

### Audit Summary
- **Total items:** 48
- **Done:** 46
- **Partial:** 0
- **Skipped:** 1 (reject-set functional test — AC-8 covered by unit test)
- **Changed:** 3 (shared/format.go, shared/format_test.go, bgp-rib/rib_test.go — not in plan, discovered during testing)

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-13 all demonstrated
- [ ] Wiring Test table complete -- every row has a concrete test name, none deferred
- [ ] `make ze-unit-test` passes
- [ ] `make ze-functional-test` passes
- [ ] Feature code integrated (internal/*)
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes (all 6 checks in rules/quality.md -- no failures)

### Quality Gates (SHOULD pass -- defer with user approval)
- [ ] `make ze-lint` passes
- [ ] RFC constraint comments added
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction (3+ use cases?)
- [ ] No speculative features (needed NOW?)
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING -- before ANY commit)
- [ ] Critical Review passes -- all 6 checks in rules/quality.md documented pass in spec
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled (every requirement, AC, test, file has status + location)
- [ ] Spec moved to `docs/plan/done/NNN-utp-2-command-format.md`
- [ ] Spec included in commit
