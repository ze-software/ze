# Spec: bgp-as-notation

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 4/4 |
| Updated | 2026-09-08 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md`
3. `internal/core/bgp/attribute/text_append.go` - AS-path/ASN text formatting
4. `internal/core/bgp/attribute/text.go` - AS-path text parsing
5. `internal/component/config/schema.go` - uint32 (ASN) config value parsing

## Task

Ze only understands AS numbers in "asplain" form: a plain decimal integer on
both config input and display output. Operators who think in "asdot" notation
(for a 4-byte ASN `X.Y`, where `ASN = X*65536 + Y`, e.g. `1.10` = 65546) cannot
type an ASN in dotted form, and Ze never renders ASNs in dotted form.

Add AS-notation support:
1. Accept an asdot `X.Y` token anywhere an ASN is configured, canonicalising it
   to the underlying uint32.
2. Add a BGP-global `as-notation` option (`asdot` | `asdot+`) that controls how
   ASNs and AS paths are rendered in CLI / looking-glass output. Default stays
   asplain (unchanged output) when the option is absent.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/config/syntax.md` - the JUNOS-like config syntax: blocks, terminators, comments and inheritance
- [ ] `docs/architecture/wire/attributes.md` - the BGP path attribute wire format: header, flags, codes and ASN4 encoding
- [ ] `docs/architecture/plugin/rib-storage-design.md` - the RIB plugin this spec changes a renderer in (`rib.go`, `rib_attr_format.go`)
  → Constraint: the notation is read once at the plugin's config callback and applied at the row builder, so the storage layer keeps holding a uint32.
- [ ] `ai/rules/performance.md` - the attribute text formatters are buffer-first and config-free.
  → Constraint: `core/bgp/attribute` is a leaf package and must NOT import config; the notation choice must be passed in as a parameter, not read from global state.
- [ ] `ai/rules/config.md`, `ai/rules/config.md` - the new BGP-global leaf.
  → Constraint: `as-notation` is a display preference, so it is a YANG leaf (not an env var), scoped to BGP global parameters.

**Key insights:**
- asplain↔asdot is a pure integer<->string transform; the stored/wire value is always uint32.
- Input acceptance (parse `X.Y`) and output rendering (format as `X.Y`) are independent; input is the cheap, high-value half.
- The formatters live in a leaf package with no config access, so display notation must thread through a format parameter/context, not a package global.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/core/bgp/attribute/text_append.go` - `(*ASPath).AppendText` (text_append.go) renders every ASN via `strconv.AppendUint(buf, uint64(asn), 10)` (:158 single, :170 in a list); aggregator ASN at :29. Always base-10 asplain, no notation parameter.
- [ ] `internal/core/bgp/attribute/text.go` - `ParseASPathText` parses each token with `strconv.ParseUint(tok, 10, 32)` (text.go); rejects any token containing a dot.
- [ ] `internal/component/config/schema.go` - config value parse for an ASN (typedef `asn` = uint32) uses `strconv.ParseUint(value, 10, 32)` (schema.go); no dotted form.

**Behavior to preserve:**
- Stored/wire ASN representation stays uint32 everywhere; no change to the on-wire encoding.
- Default output is byte-for-byte identical to today (asplain) when `as-notation` is unset.
- Existing asplain config input keeps working unchanged.

**Behavior to change:**
- ASN config parsing additionally accepts a dotted `X.Y` token and canonicalises to uint32.
- ASN/AS-path display renders in asdot/asdot+ when the BGP-global `as-notation` option selects it.

## Data Flow (MANDATORY)

### Entry Point
- Config input: any ASN leaf (`local-as`, `peer-as`, `session/asn`, aggregator, filter AS lists) typed as either a decimal or a dotted `X.Y` string.
- Config input: new BGP-global `as-notation` leaf (`asdot` | `asdot+`).
- Display output: CLI / looking-glass / RIB-replay rendering of ASNs and AS paths.

### Transformation Path
1. On config parse, an ASN token is normalised: if it contains `.`, split into `X` and `Y`, validate each in range, compute `X*65536 + Y`; else parse as decimal. Result is uint32 stored as today.
2. The `as-notation` leaf is read into BGP global settings and made available to the display layer as a notation mode value (asplain default / asdot / asdot+).
3. At render time, the ASN formatter receives the notation mode and emits either decimal (asplain) or `X.Y` (asdot: dotted only for ASN > 65535; asdot+: dotted for all).
4. AS-path rendering applies the same per-ASN formatter so paths render consistently.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config text ↔ uint32 ASN | `asn.Parse` at `ValidateValue`, `NormalizeLeafValue` writes the decimal form | `TestParseASNAsdot` |
| BGP config ↔ display layer | `parseASNotation` in the rib plugin's `OnConfigure` | `TestParseASNotation` |
| ASN uint32 ↔ output text | `asn.Append(buf, number, notation)`, called by `asPathList` | `TestASPathListRendersConfiguredNotation` |

### Integration Points
- `internal/component/config/schema.go` (or the ASN typedef validation path) - accept dotted input.
- `internal/core/bgp/attribute/text.go` - accept dotted token in `ParseASPathText`.
- `internal/core/bgp/attribute/text_append.go` - notation-aware ASN/AS-path append.
- BGP-global YANG (`parameters`) - new `as-notation` leaf, read into settings and passed to the formatter.

### Architectural Verification
- [ ] No bypassed layers (notation is a parameter to the formatter, not a package global read inside the leaf package)
- [ ] No unintended coupling (`core/bgp/attribute` gains no config import)
- [ ] No duplicated functionality (one shared asdot normaliser and one shared formatter)
- [ ] Registration over hardcoding - the display option is a config leaf read into settings; no per-notation switch is hardcoded into a shared/core package beyond the single formatter helper.

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | All ASN parse paths funnel through the uint32 validator + `ParseASPathText` | schema.go, text.go | additional parse sites accept only decimal | grep every `ParseUint(.*32)` ASN site during audit | broken |
| A-2 | The formatter can receive a notation mode without a config import | buffer-first leaf-package rule | display change is deeper than expected | thread a mode parameter/context in the audit | broken |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Display threading touches many call sites (looking glass, RIB replay, CLI) | formatter callers multiply | ship input-acceptance first (Phase 2); land display (Phase 3) behind the default-off option |
| R-2 | Ambiguous token `1.10` vs an IP-like value in some leaf | parser confusion | only ASN-typed leaves get the asdot normaliser; IP leaves are unaffected |

R-1 landed, and it cost a second implementation pass. The first pass wired the
route rows alone, which left an operator reading 1.10 on one surface and 65546
on six others. The notation now reaches every renderer through one process-wide
value (`asn.Configured`), written where each process loads its configuration.

R-1 also had a consequence the spec did not anticipate: a dotted AS number is
not a JSON number, so a payload written under asdot carries a STRING, and three
in-tree consumers decoded that field as a plain uint32 and answered nothing.
`asn.Number` and `asn.FromJSON` are the readers that fixed them.

R-2 did not land: `TypeASN` marks the ASN leaves and `TestParseASNAsdot` proves
a `TypeUint32` leaf still refuses `1.10`.

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `bgp { session { asn { local 1.10; } } }` | → | `asn.Parse` yields 65546, stored as "65546" | `test/plugin/bgp-as-notation.ci` |
| `bgp { as-notation asdot; }` | → | `asPathList` renders `["1.10","100"]` | `test/plugin/bgp-as-notation.ci` |
| `bgp { as-notation asdot; }` | → | `show bgp peer detail` renders `"remote-as":"1.10"` | `test/plugin/bgp-as-notation.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | config ASN `1.10` | canonicalises to 65546 uint32 |
| AC-2 | config ASN `65546` | still parses (asplain preserved) |
| AC-3 | `as-notation asdot`, render ASN 65546 | outputs `1.10` |
| AC-4 | `as-notation asdot`, render ASN 100 | outputs `100` (2-byte stays plain) |
| AC-5 | `as-notation asdot+`, render ASN 100 | outputs `0.100` (dotted for all) |
| AC-6 | `as-notation` unset | output byte-for-byte identical to today (asplain) |
| AC-7 | invalid asdot `1.99999` (Y out of range) | rejected with a clear error |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | configures a peer AS in asdot and sets asdot display | `asn.Parse` → `NormalizeLeafValue` → rib `OnConfigure` → `asPathList` | `test/plugin/bgp-as-notation.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestParseASNAsdot` | `internal/component/config/schema_test.go` | `X.Y` → uint32, range-checked | PASS |
| `TestValidateASNLeafRange` | `internal/component/config/schema_test.go` | the YANG range is applied to the number an asdot token names | PASS |
| `TestASNTypedefMapsToTypeASN` | `internal/component/config/schema_test.go` | the `asn` typedef is what marks an ASN leaf | PASS |
| `TestParseASPathTextAsdot` | `internal/core/bgp/attribute/text_test.go` | dotted token accepted in AS path | PASS |
| `TestParseASPathTextRejects` | `internal/core/bgp/attribute/text_test.go` | a malformed dotted token is still refused | PASS |
| `TestParseAsdot` / `TestParseRejects` | `internal/core/bgp/asn/asn_test.go` | `X.Y` → uint32 and every boundary refusal | PASS |
| `TestAppendNotation` / `TestAppendRoundTrips` | `internal/core/bgp/asn/asn_test.go` | asplain/asdot/asdot+ rendering, and the round trip back | PASS |
| `TestAppendAllocatesNothing` | `internal/core/bgp/asn/asn_test.go` | the formatter allocates nothing | PASS |
| `TestParseNotation` | `internal/core/bgp/asn/asn_test.go` | the three config tokens, and a typo refused | PASS |
| `TestParseASPathTextForms` | `internal/core/bgp/attribute/text_test.go` | every argument shape the `as-path` word accepts | PASS |
| `TestParseASPathAsdotThroughCommand` | `internal/component/bgp/route/route_builder_parse_test.go` | the `as-path` command word reads every notation, at the entry point | PASS |
| `TestRouteDistinguisherReadsEveryNotation` | `internal/component/bgp/config/asn_notation_test.go` | `rd 1.10:5` and `rd 65546:5` are one route distinguisher; the IPv4 form keeps type 1 | PASS |
| `TestASNSetNameAgreesAcrossBothDerivations` | `internal/component/firewall/plugins/irr/asn_notation_test.go` | the rule and the set owner name one nftables set for every spelling | PASS |
| `TestASNWhoisKeyIsDecimal` / `TestClearASNReadsEveryNotation` | `internal/component/firewall/plugins/irr/asn_notation_test.go` | the whois key stays decimal, and the clear word reads every spelling | PASS |
| `TestRequireASNReadsEveryNotation` | `internal/component/resolve/cmd/resolve_test.go` | the shared reader behind every `resolve` ASN input | PASS |
| `TestResolveRIRReadsEveryNotation` | `internal/component/resolve/cli/asn_notation_test.go` | `ze resolve rir` reads every spelling, through the real dispatch | PASS |
| `TestResolveCymruReadsADottedASNumber` | `internal/component/resolve/cli/asn_notation_test.go` | `ze resolve cymru asn-name` parses the AS number before it reaches DNS | PASS |
| `TestParseRDStringReadsEveryNotation` / `TestParseRDStringAgreesWithTheConfigReader` | `internal/core/bgp/nlri/rd_asn_notation_test.go` | the RD parser every `rd` command word reaches, and its agreement with the config reader | PASS |
| `TestUpdateTextRDReadsEveryNotation` | `internal/component/bgp/plugins/cmd/update/asn_notation_test.go` | the `rd` word of `update text`, at the command's own entry point | PASS |
| `TestExtCommunityAdminReadsEveryNotation` / `TestFlowSpecRedirectReadsEveryNotation` | `internal/core/bgp/attribute/extcomm_asn_notation_test.go` | the extended-community administrator and the flowspec redirect | PASS |
| `TestExtCommunityLSuffixIsOrthogonalToTheSpelling` | `internal/core/bgp/attribute/extcomm_asn_notation_test.go` | `100L` and `0.100L` encode the same bytes, and `1.10L` equals `1.10` | PASS |
| `TestExtendedCommunityReadsEveryNotation` | `internal/component/bgp/config/asn_notation_test.go` | the configured `extended-community` leaf | PASS |
| `TestExtendedCommunityAsdotThroughCommand` | `internal/component/bgp/route/route_builder_parse_test.go` | the `extended-community` command word | PASS |
| `TestASNSelectorReadsEveryNotation` / `TestParseReachesTheASNSelector` | `internal/core/selector/asn_notation_test.go` | the `as<N>` peer selector, and that ParseDefault does not turn it into a peer name | PASS |
| `TestAnalyzeASFlagsReadEveryNotation` | `internal/analyze/asn_notation_test.go` | the four operator-typed AS flags of `ze-analyze` | PASS |
| `TestParseRDStringDeclaredTypeWins` | `internal/core/bgp/nlri/rd_asn_notation_test.go` | a declared RD type survives the round trip through String() | PASS |
| `TestNumberParseGate*` (six) | `internal/le/repository/numberparse_test.go` | the gate: an unlisted file, a file over its count, the 32-bit width, test files skipped, fail-closed with no allowlist, and the baseline still matching this tree | PASS |
| `TestFlowRDStringReadsEveryNotation` | `internal/component/bgp/plugins/nlri/flowspec/asn_notation_test.go` | the third RD reader | PASS |
| `TestMVPNRDStringReadsEveryNotation` / `TestMVPNSourceASReadsEveryNotation` | `internal/component/bgp/plugins/nlri/mvpn/asn_notation_test.go` | the fourth RD reader, and the MVPN `source-as` word | PASS |
| `TestPathPatternReadsEveryNotation` | `internal/component/bgp/plugins/rib/asn_notation_test.go` | `show bgp rib path <pattern>`, validator and matcher together | PASS |
| `TestPolicySelectorReadsEveryNotation` | `internal/component/bgp/plugins/cmd/policy/handler_test.go` | the `AS<n>` peer selector | PASS |
| `TestValidateCommandReadsEveryNotation` | `internal/component/bgp/plugins/rpki/asn_notation_test.go` | `rpki validate <prefix> <asn>` | PASS |
| `TestResolvePeeringDBReadsADottedASNumber` | `internal/component/resolve/cli/asn_notation_test.go` | `ze resolve peeringdb` parses before it reaches the API | PASS |
| `TestInterASRemoteASReadsEveryNotation` / `TestInterASRemoteASRefusesAMalformedASNumber` | `internal/plugins/ospf/asn_notation_test.go` | the RFC 5392 inter-AS Remote AS Number, from config text to the plugin reader | PASS |
| `TestASPathAppendTextStaysAsplain` | `internal/core/bgp/attribute/text_append_test.go` | filter text stays asplain, so a configured filter keeps matching | PASS |
| `TestParseASNotation` | `internal/component/bgp/plugins/rib/rib_asn_notation_test.go` | the leaf selects the notation; a typo refuses the config | PASS |
| `TestASPathListRendersConfiguredNotation` | `internal/component/bgp/plugins/rib/rib_asn_notation_test.go` | asplain writes numbers, asdot and asdot+ write strings | PASS |
| `TestRouteMapASPathHonorsNotation` | `internal/component/bgp/plugins/rib/rib_asn_notation_test.go` | the notation reaches the row builder | PASS |
| `TestAsLeafAnswersThreeStates` | `internal/test/runner/peer_asn_test.go` | the .ci harness reads the same spellings the daemon does | PASS |
| `TestNumberJSONRoundTrip` / `TestNumberUnmarshalsEveryNotation` | `internal/core/bgp/asn/asn_test.go` | a payload written in any notation is read back as the same number | PASS |
| `TestFromJSON` | `internal/core/bgp/asn/asn_test.go` | every decoded shape is read; a value naming no AS number reports absence | PASS |
| `TestConfigureFromBGP` | `internal/core/bgp/asn/asn_test.go` | the leaf selects the notation; a typo refuses the config | PASS |
| `TestPeerRowsFollowTheConfiguredNotation` | `internal/component/bgp/plugins/cmd/peer/asn_notation_test.go` | `show bgp peer detail` writes remote-as and local-as in the notation | PASS |
| `TestROARowsFollowTheConfiguredNotation` / `TestASPACustomerFollowsTheConfiguredNotation` | `internal/component/bgp/plugins/rpki/asn_notation_test.go` | the RPKI rows follow the notation, and an asdot argument is accepted | PASS |
| `TestIRRRowsFollowTheConfiguredNotation` | `internal/component/bgp/plugins/filter_irr/asn_notation_test.go` | the IRR rows follow the notation | PASS |
| `TestDashboardReadsAnAsdotPeerList` / `TestDashboardReadsAnAsplainPeerList` | `internal/component/cli/asn_notation_test.go` | the dashboard decodes and renders both forms | PASS |
| `TestExtractASPathReadsADottedRow` / `TestGraphLabelsFollowTheConfiguredNotation` / `TestResolveASNLooksUpTheDecimalForm` | `internal/component/lg/asn_notation_test.go` | the looking glass reads a dotted row, labels in the notation, and looks names up by the decimal form | PASS |
| `TestPeerCompletionsReadAnAsdotList` / `TestPeerCompletionsReadAnAsplainList` | `internal/plugins/completion/asn_notation_test.go` | peer completion decodes both forms | PASS |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| asdot X | 0..65535 | 65535 | - | 65536 |
| asdot Y | 0..65535 | 65535 | - | 65536 |
| ASN (combined) | 1..4294967295 | 4294967295 | 0 | 4294967296 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `bgp-as-notation` | `test/plugin/bgp-as-notation.ci` | asdot input accepted; asdot display emitted | PASS (RED observed under two forced breaks) |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| asdot is a local notation only; wire ASN is unchanged uint32 | - | - | no on-wire change; interop unaffected | - |

### Future (if deferring any tests)
- None planned.

## Files to Modify
- `internal/component/config/schema.go` - accept dotted ASN input at the uint32/ASN parse path
- `internal/core/bgp/attribute/text.go` - accept dotted token in `ParseASPathText`
- `internal/core/bgp/attribute/text_append.go` - renders through the shared formatter; stays asplain, see Design Insights
- `internal/component/bgp/yang/ze-bgp-conf.yang` - the `as-notation` leaf, directly under `bgp` (there is no `parameters` block)
- `internal/component/bgp/plugins/rib/rib.go`, `rib_attr_format.go` - read the leaf, render the route row

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new leaf) | [ ] yes | `internal/component/bgp/yang/ze-bgp-conf.yang`, leaf `as-notation` |
| Functional test | [ ] yes | `test/plugin/bgp-as-notation.ci` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] yes | `docs/features/bgp-protocol.md`, "AS number notation" |
| 2 | Config syntax changed? | [ ] yes | `docs/guide/configuration.md`, "AS Number Notation" |

## Files to Create
- `internal/core/bgp/asn/asn.go` - the one normaliser and the one formatter
- `internal/component/bgp/plugins/rib/rib_asn_notation.go` - the configured notation
- `test/plugin/bgp-as-notation.ci` - functional test
- (unit tests extend existing test files)

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation Phases below |

### Implementation Phases
1. **Phase: Wiring (MANDATORY FIRST)** - add the `as-notation` leaf (parsed, unused) and a failing `test/ci/bgp-as-notation.ci`.
2. **Phase: Input acceptance** - shared asdot normaliser at the ASN parse sites (config + AS-path text).
   - Tests: `TestParseASNAsdot`, `TestParseASPathTextAsdot`
3. **Phase: Display notation** - thread notation mode into the ASN/AS-path formatter; render asdot/asdot+.
   - Tests: `TestASPathAppendTextNotation`
4. **Functional test** - asdot input + asdot display end to end.
5. **Full verification** → `./le verify current mode full`
6. **Complete spec** → audit, learned summary, two-commit closure.

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N implemented with file:line |
| Correctness | uint32 storage unchanged; default output identical; asdot vs asdot+ boundary at 65535 |
| No leaf-package config import | `core/bgp/attribute` still imports no config |
| Registration over hardcoding | notation read from config into settings; single shared formatter, no scattered per-notation branches |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| asdot input | `go test ./internal/component/config -run Asdot` |
| notation render | `go test ./internal/core/bgp/attribute -run Notation` |
| functional | `test/ci/bgp-as-notation.ci` |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | reject out-of-range X/Y and malformed dotted tokens |

## Mistake Log
### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
| A-1: every ASN parse site funnels through the uint32 validator and `ParseASPathText` | Eight readers parse an AS number: `config.ValidateValue`, `cli.validateYangType`, `attribute.ParseASPathText`, three sites in `update_text.go`, `peer/create.go setASN`, `rib_commands.go parseASNList`, and `runner.asLeaf` | grep for `ParseUint(.*32)` over the ASN surfaces, then a red `.ci` when the harness refused `1.10` | Every one of them now reads `asn.Parse`; the audit cost one extra pass |
| A-3: fixing `ParseASPathText` fixed the `as-path` command word | It fixed nothing an operator reaches. The word went to `Builder.ParseASPath`, a second decimal-only parser, and `ParseASPathText` had no product caller at all | review round 5, `./le repository check` naming the exported symbol with no cross-package non-test caller | The command word now calls `ParseASPathText` and the second parser is deleted. A test written at the fixed function, rather than at the entry point, proved nothing |
| A-4: a grep for the ASN readers finds them all | `ParseRouteDistinguisher` parses its AS half three functions from two that were fixed, in the same file and behind the same caller, and `irrSetMatch` derives an AS-named set without parsing at all | review round 5 | Both are fixed |
| A-5: a sweep that starts at the config leaf terminates | It cannot. A leaf reaches ONE parser, so a second parser of the same syntax is invisible from it: round 5 found two AS-path parsers, round 6 two RD parsers, round 7 four RD parsers. Each time the leaf-side one was already fixed | review rounds 5 to 7, then an enumeration from the parser side | The sweep was inverted: every `ParseUint(..., 10, 32/16)` over an AS field in the tree now carries a verdict, in "Every Text-to-AS-Number Parser In The Tree" above |
| A-2: the notation can be threaded into `text_append.go` as a parameter | It can, and the result has no caller. Every consumer of `(*ASPath).AppendText` is a machine contract | `gopls references` on `AppendText`, then the unwired-symbol rule in `ai/rules/completion.md` | The notation-aware variant was deleted; the display notation is applied in the rib row builder |

## Design Insights
<!-- LIVE -->

- **The spec's two file citations had drifted.** There is no `parameters` block
  under `bgp`, so `as-notation` is a leaf directly under `container bgp`. There
  is no `test/ci/` directory, so the functional test is `test/plugin/`.
- **The notation has to reach EVERY renderer, and one value is how.**
  `asn.Configured()` is a process-wide setting, written where each process
  loads its configuration: `ResolveBGPTree` for the daemon, the rib plugin's
  `OnConfigure` for an out-of-process rib, and `cli.newEditor` for the CLI.
  Threading a parameter to the daemon, four plugins, the CLI and the looking
  glass would have been one value under a dozen names. `Append` and `Text`
  still take the notation as a parameter, so the formatter reads no state.
- **A display notation changes the TYPE of a payload field, and that is what
  broke three consumers.** `1.10` is not a JSON number, so a dotted AS number
  is a string. `cli.dashboardPeer.RemoteAS` and `completion.peerEntry.RemoteAS`
  were plain uint32 and failed the whole decode, which the dashboard shows as
  an empty peer list and completion as no completions. `lg.extractASPath`
  skipped a string outright, so a topology graph under asdot would have dropped
  every AS number of 65536 or more and drawn a path that does not exist. All
  three now read `asn.Number` or `asn.FromJSON`.
- **A lookup key is not a display.** `lg.resolveASN` feeds a Team Cymru DNS
  lookup, whose key is the decimal form. It normalizes its argument rather than
  making each caller do it.
- **`text_append.go` is a machine contract, not a display path.** Every caller
  of `(*ASPath).AppendText` builds filter text a plugin matches, or an
  `update text` command Ze replays to itself (`internal/component/bgp/format.go`,
  `internal/component/bgp/reactor/filter_format.go`,
  `internal/component/bgp/plugins/rpki/rpki.go`). A notation there would stop a
  configured filter matching. A notation-aware variant was written, had no
  cross-package caller, and was deleted rather than left unwired. The file now
  calls `asn.Append` with `NotationPlain`, so one formatter renders every AS
  number in Ze.
- **In Ze a `show` response IS the output**, so a display notation has to change
  the payload. `asPathList.MarshalJSON` writes numbers under asplain, which is
  byte-for-byte what every earlier release wrote, and quoted strings under a
  dotted notation, because `1.10` is not a JSON number.
- **The ASN leaves needed a type of their own.** `ValidateValue` had no way to
  tell an AS number from any other uint32, and R-2 forbids widening uint32. The
  `zt:asn` typedef already existed and `isASNType` already read it, so it now
  maps to a new `TypeASN` rather than to `TypeUint32`.
- **Four write paths, one normaliser.** `NormalizeLeafValue` was
  `normalizeSetValue`, reached by three set paths while the config file parser
  carried its own inline bool branch. It is now exported, carries the ASN case,
  and is called by the file parser and by `cli.Editor.SetValue`, which is the
  editor behind both the CLI and the web form.

## Implementation Summary
### What Was Implemented
- `internal/core/bgp/asn`: `Parse`, `Append`, `Text`, `Notation`, `IsTypedef`.
  One normaliser and one formatter, RFC 5396 Section 2 and Section 3 quoted at
  each rule.
- Config input: `TypeASN`, derived from the `zt:asn` typedef, validated with
  `asn.Parse` and canonicalised by `NormalizeLeafValue` on every write path.
- Command input: `attribute.ParseASPathText`, which is now the parser the
  `as-path` command word reaches (`route.parseCommonAttributeBuilder`);
  `Builder.ParseASPath`, the decimal-only second parser that used to serve that
  word, is deleted. Beside it: `update text` (`as-path`, `origin-as`,
  `aggregator`), `create bgp peer ... asn`, `request bgp rib inject ...
  aspath`, `config.ParseRouteDistinguisher`, `resolve`'s shared `requireASN`
  and `resolve rir`, `update`/`clear firewall irr asn`, and the `.ci` harness
  AS derivation all read `asn.Parse`.
- One nftables set name, two derivations: `firewall.IRRASNName` is the single
  declaration the rule parser and the firewall-irr owner both call, so a dotted
  `source-asn` names one set on both sides.
- Display: the `bgp { as-notation }` leaf, recorded by `asn.Configure` where
  each process loads its configuration, and applied at every renderer an
  operator reads an AS number on: the `show bgp rib` rows (`asPathList`), the
  peer rows of `show bgp` and `show bgp peer detail` (`asn.Number`), the RPKI
  ROA and ASPA rows, the IRR rows, the CLI dashboard, the looking glass tables,
  labels and topology graph, and the web BGP peer and group pages.
- Read decimal only, each with the reason at the producer: the AS half of a
  standard community (RFC 1997 page 3 gives it two octets, and the only dotted
  spelling that fits is `0.Y`, which names the AS number `Y` already names) and
  the Global Administrator of a large community (RFC 8092 Section 5 pins the
  canonical representation to three decimal integers). Both reasons live at
  `attribute.ParseCommunity` and `attribute.ParseLargeCommunity`.
- Excluded on purpose, each with a comment at the producer saying why: the
  filter text and replay command of `attribute.(*ASPath).AppendText`, the
  plugin event stream (`attribute/json.go`), the process-protocol text
  (`bgp/format/text_human.go`), the replay builder (`bgp/format.go`), and the
  ExaBGP bridge (`exabgp/bridge/bridge_event_text.go`).

### Bugs Found/Fixed
Each row of the Review Gate's "Findings fixed" table is a defect this work
found. The two that were reachable by an operator with no dotted AS number
anywhere are the ones to know about: `ParseSingleExtCommunity` chose its IPv4
branch on a search for a dot, and `Builder.ParseASPath` was a second AS-path
parser that had quietly become the one the `as-path` command word reached. Both
carry a test at the entry point now.

### Documentation Updates
- `docs/features/bgp-protocol.md`, "AS number notation": the reader surfaces,
  the fields that stay decimal with the RFC sentence for each, the leaf, the
  JSON string consequence, and the five surfaces that stay asplain. Eight
  `<!-- source: -->` anchors.
- `docs/guide/configuration.md`, "AS Number Notation": already in HEAD, carried
  by another session's commit.
- `docs/architecture/api/birdwatcher-compat.md`: `neighbor_as` stays an integer
  whatever the notation, because the field is a birdwatcher contract.
- `ai/INDEX.md`, `ai/PACKAGE-MAP.md`: `./le repository` now names six checks.
- `./le doc check verify` is not re-run here: `./le repository tree-check`, which
  carries the source-anchor check, passes over this tree.

### Deviations from Plan
- **The sweep was inverted.** The plan reached the parsers from the config
  leaves. Rounds 5, 6 and 7 each found one more parser that way, so the
  enumeration was rebuilt from the PARSER side and written into this spec, and
  `checkNumberParseSites` was added to keep it true. Neither the enumeration nor
  the gate was in the plan.
- **`text_append.go` gained no notation.** The plan threaded a notation
  parameter into `(*ASPath).AppendText`. It was written, had no cross-package
  caller, and was deleted: every caller of that function builds filter text or a
  replay command, so a notation there would stop a configured filter matching.
- **Three product files are written and verified but NOT in this spec's commit.**
  `internal/component/bgp/config/loader_create.go` (the `applyASNotation` call on
  first load), `internal/component/bgp/reactor/reactor_api.go` (`recordASNotation`
  at `SetConfigTree`) and
  `internal/component/bgp/plugins/cmd/peer/peer.go` (`asn.Of` on the two peer row
  builders) each also carry hunks from the BFD-strict and update-delay sessions
  that name symbols HEAD does not hold: `ParseUpdateDelay`,
  `plugin.UpdateDelayStatus`, `Peer.bfdSubState`, `bfdStrictConfigChanged`.
  Committing any of the three alone stops the tree compiling, and
  `plugin.ReactorIntrospector` gained a method, so the closure of what would have
  to ride along is both sibling features whole (`ai/rules/git-safety.md`).
  Four tests that depend on those producers ARE committed and are red until the
  sibling commits land: `TestPeerRowsFollowTheConfiguredNotation`,
  `TestHandlerPeerDetailAllPeers`, `TestTheRunningConfigRecordsTheNotation`, and
  `test/plugin/bgp-as-notation.ci`. `peer_ops_test.go` is committed with one
  assertion red on purpose, because its other assertions belong to `summary.go`,
  which IS in this commit: withholding the file would leave a red against a
  producer that landed.
  Once this commit is in HEAD, `asn.Of` and `applyASNotation` are symbols the
  sibling sessions can safely carry, which is how the four files land.

## Every Text-to-AS-Number Parser In The Tree

Enumerated from the PARSER side, not from the leaves: `ParseUint(x, 10, 32)` and
`(x, 10, 16)` over an AS or administrator field, `Atoi`, `Sscanf`, and any split
on `:` or `.` whose left half becomes an AS number. The walk is
`scratch/r7-candidates.txt` (84 sites over the tree, test files excluded), and
each site below carries a verdict. A leaf-driven sweep cannot terminate, because
a second parser is invisible from the leaf that does not use it: rounds 5, 6 and
7 each found one that way.

### Reads every RFC 5396 spelling

| Parser | File |
|--------|------|
| `asn.Parse` -- the one normalizer every row below calls | `internal/core/bgp/asn/asn.go` |
| `NormalizeLeafValue` (TypeASN) and `ValidateValue` -- every `zt:asn` leaf | `internal/component/config/setparser.go`, `yang_schema.go` |
| `validateYangType` -- the CLI editor's copy of that check | `internal/component/cli/completer_validate.go` |
| `ParseASPathText` -- the `as-path` command word | `internal/core/bgp/attribute/text.go` |
| `ParseASPath`, `ParseAggregator`, `ParseRouteDistinguisher` | `internal/component/bgp/config/routeattr.go` |
| `ParseRDString` -- every command that takes an `rd` word, 19 call sites | `internal/core/bgp/nlri/rd.go` |
| `flowRDStringToBytes` | `internal/component/bgp/plugins/nlri/flowspec/config.go` |
| `rdStringToBytes`, `parseMVPNFields` (`source-as`) | `internal/component/bgp/plugins/nlri/mvpn/config.go`, `encode.go` |
| `parseASNList`, `validatePathPattern` + `matchASPath` (the pair moves together) | `internal/component/bgp/plugins/rib/rib_commands.go`, `rib_pipeline.go` |
| `filterPeersByPolicySelector` -- the `AS<n>` selector | `internal/component/bgp/plugins/cmd/policy/handler.go` |
| `validateCommand` -- `rpki validate <prefix> <asn>` | `internal/component/bgp/plugins/rpki/rpki.go` |
| `addASNs`, `addNth` -- the reject-asn lists | `internal/component/bgp/plugins/filter_path_asn/config.go` |
| `IRRASNName`, `updateASN`, `clearASN` -- one declaration of the AS set name | `internal/component/firewall/config.go`, `plugins/irr/command.go` |
| `ParseExtCommunityAdmin` -- one declaration of the extended-community administrator, called by `ParseSingleExtCommunity`, `FlowSpecRedirect`, `parseExtCommunityASN` and `parseRouteTargetExtCommunity` | `internal/core/bgp/attribute/text.go` |
| `ParseASNSelector` -- one declaration of the `as<N>` peer selector, called by `Parse` and by `cmd/policy`'s peer filter | `internal/core/selector/selector.go` |
| `--peer-asn` and the three `--local-as` readers of the shipped `ze-analyze` | `internal/analyze/filter.go`, `serve.go`, `replay.go`, `inject.go` |
| `requireASN`, `cmdRIR`, `cmdCymru`, `cmdPeeringDB` | `internal/component/resolve/cmd/resolve.go`, `cli/cmd_*.go` |
| `setASN`, the three `update text` readers, `asLeaf` | `.../cmd/peer/create.go`, `.../cmd/update/update_text.go`, `internal/test/runner` |

### Deleted: a second parser of a syntax, with no operator path to it

| Parser | Why it went |
|--------|-------------|
| `attribute.(*Builder).ParseASPath` | The `as-path` word reached it while the fixed `ParseASPathText` had no product caller. |
| `route.parseASPath` | A private copy with only its own test calling it. Its table now drives `ParseASPathText`. |

### Excluded: the field is not a four-byte AS number

| Parser | Reason |
|--------|--------|
| `attribute.ParseCommunity`, `parseSingleCommunity` | RFC 1997 page 3 encodes the AS in the first TWO octets, so the only dotted spelling that fits is `0.Y`, which names the AS number `Y` already names. |
| `attribute.ParseLargeCommunity` | RFC 8092 Section 5 pins the canonical representation to three decimal integers. |
| `parseMUPExtCommunity`, `filter_community.parseStandardWire` | 16-bit administrator, as above. |
| `route.parseTrafficRateFunction` | RFC 8955 Section 7.1: "The first two octets carry the 2-octet id, which can be assigned from a 2-octet AS number." Two octets, and the RFC adds that the value "is purely informational and SHOULD NOT be interpreted by the implementation". |
| `route.parseOriginExtCommunity`, `filter_community.parseExtendedWire` | 2-octet administrator in both their forms. Neither can express a four-byte AS number at all, which is a feature gap rather than a notation gap. |

### Excluded: text Ze writes and reads back, so a notation would break a contract

| Parser | Reason |
|--------|--------|
| `encodeASPathValue`, `encodeAggregatorValue`, `ExtractASPathPrependOps` | Filter text. A dotted AS number stops a configured filter matching. |
| `rs.parseTextOpen`, `rs.parseTextState`, `persist.parsePersistState`, `persist.parsePersistOpen` | Process-protocol text between Ze and its own plugins. |
| `watchdog.parseASPath` | Reads the plugin event stream, which stays asplain. |
| `filter_path_asn.parseASN` | Matches a subject Ze itself wrote; the producer writes decimals. |
| `filter_remove_private_as.rewriteASPathText` | Filter text again. |
| `exabgp/bridge.qualifyExtCommunity`, `exabgp/migration.interfaceSetExtCommunity` | ExaBGP's own text format; that program fixes the spelling. |

### Excluded: text an external protocol defines, which Ze reads rather than writes

| Parser | Reason |
|--------|--------|
| `irr.parseASN` (`internal/component/resolve/irr/client.go`) | Reads an `origin:` field out of a whois RPSL answer. RPSL writes `AS65546`, and no registry emits a dotted spelling, so a reader that accepted one would accept something no server sends. |
| `irr.bareASN` (`internal/component/firewall/plugins/irr/irr.go`) | Strips the `AS` prefix off a name THIS plugin wrote through `firewall.IRRASNName`, which renders decimal. The producer is one function away. |

### Excluded: reads a value the config tree already normalized to decimal

`bgpconfig.globalLocalAS`, `peers.dynamicGroupsFromTree`, `bmp.parseLocalIdentity`,
`as112.parseConfig`, `doctor.checkAS112GlobalOriginCoordination`,
`web.asnNameDecorator.Decorate` and its Cymru variant, `web.communityNameDecorator`.

### Excluded: not product code

`internal/test/peer/expect.go`, `internal/test/mock/rtr`, `internal/test/mock/peeringdb`,
`internal/test/plugins/fakeas112`, `recordOpenAS` in `internal/test/cli/cmd_bgp.go`,
`internal/le/interoplab/bgp/*`, `internal/le/qemu`.
`ike/ipsec.parseSiteToSitePeer` is a false positive of the walk: the field is
`policy-priority`, not an AS number.

### The extended-community administrator: one reader, every spelling

Four parsers read this field, in four packages, each with its own answer to the
same question. They agreed only because all four were wrong in the same
direction. `attribute.ParseExtCommunityAdmin` is now the one declaration, and
all four call it: `attribute.ParseSingleExtCommunity`,
`attribute.FlowSpecRedirect`, `bgpconfig.parseExtCommunityASN` and
`route.parseRouteTargetExtCommunity`.

RFC 5668 Section 2 gives the 4-octet AS specific form a "4-octet Autonomous
System number" in its Global Administrator, and no RFC pins the text form to
decimal, so AC-1 reaches it and the reader takes all three spellings.

`ParseSingleExtCommunity` also chose its IPv4 branch on a search for a dot,
which read `target:1.10:5` as an address. It probes with netip now, as the four
route distinguisher readers do.

**The `L` suffix is orthogonal to the spelling.** It means "encode as four
octets whatever the value", so `65000L`, `0.100L` and `1.10L` all say the same
thing. Above 65535 it is redundant, because the value forces that encoding by
itself, and redundant is ACCEPTED rather than refused: an operator who writes
the suffix everywhere should not have to learn where it stops being needed.
`FlowSpecRedirect` and `parseRouteTargetExtCommunity` gain the suffix by
sharing the reader, and nothing that parsed before changes, because both used
to refuse it.

Two parsers of a NEIGHBORING form stay decimal-only, and the field width is the
reason: `route.parseOriginExtCommunity` and
`filter_community.parseExtendedWire` encode a 2-octet administrator, so the
only dotted spelling that fits is `0.Y`, which names the AS number `Y` already
names. Neither can express a four-byte AS number at all, which is a feature gap
rather than a notation gap.

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Accept an asdot `X.Y` token anywhere an ASN is configured | Done | `asn.Parse` (`internal/core/bgp/asn/asn.go`), reached from `config.ValidateValue` and `config.NormalizeLeafValue` for every `zt:asn` leaf, and from every command reader named in "Every Text-to-AS-Number Parser In The Tree" | The sweep was inverted to run from the parser side, because a leaf-driven sweep cannot terminate (lesson A-5) |
| A BGP-global `as-notation` option controlling ASN and AS-path rendering | Done | `as-notation` leaf (`internal/component/bgp/yang/ze-bgp-conf.yang`), recorded by `asn.Configure` / `asn.ConfigureFromBGP`, read by `asn.Configured` at every renderer | Applied through one process-wide value rather than a threaded parameter, because the renderers span the daemon, four plugins, the CLI, the looking glass and the web UI |
| Default output byte-for-byte unchanged when the option is absent | Done | `Notation` zero value is `NotationPlain`; `asn.AppendJSON` writes a JSON number under it (`internal/core/bgp/asn/asn.go`) | `TestASPathListRendersConfiguredNotation` pins the asplain rendering |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `asn.Parse` dotted branch; `TestParseAsdot`, `TestParseASNAsdot`, `test/plugin/bgp-as-notation.ci` | `1.10` becomes 65546 and is stored as the decimal form by `NormalizeLeafValue` |
| AC-2 | Done | `asn.Parse` undotted branch; `TestParseAsdot` | asplain input is unchanged |
| AC-3 | Done | `Notation.plain` + `asn.Append`; `TestAppendNotation` | asdot writes `1.10` for 65546 |
| AC-4 | Done | `Notation.plain`, `NotationDot` case; `TestAppendNotation` | asdot leaves AS 100 plain (RFC 5396 Section 2) |
| AC-5 | Done | `Notation.plain`, `NotationDotPlus` case; `TestAppendNotation` | asdot+ writes `0.100` |
| AC-6 | Done | `NotationPlain` is the `Notation` zero value; `asn.AppendJSON`; `TestASPathListRendersConfiguredNotation` | The absent leaf writes JSON numbers, as every earlier release did |
| AC-7 | Done | `asn.Parse` field bound `fieldMax`; `TestParseRejects` | `1.99999` is refused with a message naming the 0..65535 bound |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| The whole TDD table above | Pass | as listed per row | Re-run at closure: `internal/core/bgp/asn`, `internal/le/repository`, `internal/component/config`, `internal/core/selector`, `internal/core/bgp/attribute`, `internal/core/bgp/nlri`, `internal/component/bgp/route`, `internal/component/resolve/*`, `internal/plugins/ospf`, `internal/component/firewall/plugins/irr` and the BGP plugin packages all answer `ok` |
| `TestPeerRowsFollowTheConfiguredNotation` | Red in the committed tree | `internal/component/bgp/plugins/cmd/peer/asn_notation_test.go` | Its producer, `handleBgpPeerDetail` in `peer.go`, could not be committed: see Deviations |
| `TestHandlerPeerDetailAllPeers` | Red in the committed tree | `internal/component/bgp/plugins/cmd/peer/peer_ops_test.go` | Same producer. The file is committed anyway, because its other assertions belong to `summary.go`, which IS committed |
| `TestTheRunningConfigRecordsTheNotation` | Red in the committed tree | `internal/component/bgp/reactor/asn_notation_test.go` | Its producer, `recordASNotation` in `reactor_api.go`, could not be committed: see Deviations |
| `bgp-as-notation` functional test | Red in the committed tree | `test/plugin/bgp-as-notation.ci` | Its producer, the `applyASNotation` call in `loader_create.go`, could not be committed: see Deviations |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/core/bgp/asn/asn.go` | Done | The one normalizer and the one formatter |
| `internal/component/bgp/plugins/rib/rib_asn_notation.go` | Done | The out-of-process rib's own record of the notation |
| `test/plugin/bgp-as-notation.ci` | Done | Committed, red until the three deferred files land |
| `internal/component/config/schema.go`, `setparser.go`, `yang_schema.go` | Done | `TypeASN`, `NormalizeLeafValue` |
| `internal/core/bgp/attribute/text.go`, `text_append.go` | Done | `ParseASPathText`, `ParseExtCommunityAdmin`; `AppendText` stays asplain by design |
| `internal/component/bgp/yang/ze-bgp-conf.yang` | Done | The `as-notation` leaf |
| `internal/le/repository/numberparse.go` + allowlist | Changed | Not in the original plan. Added in round 8 so a seventh private AS-number parser cannot land unseen |

### Audit Summary
- **Total items:** 7 acceptance criteria, 3 task requirements, 7 planned files.
- **Done:** all 17.
- **Partial:** none.
- **Skipped:** none.
- **Changed:** 1 (the repository gate, added beyond the plan; recorded in Deviations).

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| An operator can type an AS number in asdot wherever one is configured | functional + unit | `test/plugin/bgp-as-notation.ci` configures `asn { local 1.10; remote 1.10; }` and the daemon starts. Unit: `TestParseASNAsdot` (`internal/component/config`), plus one `asn_notation_test.go` per reader surface (14 files), each driving the reader at its own entry point rather than at `asn.Parse` |
| An operator can type an AS number in asdot at a COMMAND, not only in the config | unit at the entry point | `TestParseASPathAsdotThroughCommand` (`internal/component/bgp/route`), `TestResolveRIRReadsEveryNotation`, `TestUpdateTextRDReadsEveryNotation`, `TestValidateCommandReadsEveryNotation`, `TestASNSelectorReadsEveryNotation`. Lesson A-3 is why these are written at the command word and not at the parser |
| Ze renders AS numbers in the notation the operator selected, on every surface an operator reads one | functional + unit | `bgp-as-notation.ci` asserts `AS-NOTATION OK as-path 1.10 100` and `AS-NOTATION OK remote-as 1.10` at the process boundary, which is two different row builders. Unit: `TestASPathListRendersConfiguredNotation`, `TestPeerRowsFollowTheConfiguredNotation`, `TestROARowsFollowTheConfiguredNotation`, `TestIRRRowsFollowTheConfiguredNotation`, `TestGraphLabelsFollowTheConfiguredNotation`, `TestDashboardReadsAnAsdotPeerList`, `TestPeerCompletionsReadAnAsdotList` |
| Output is unchanged for an operator who sets nothing | unit | `TestASPathListRendersConfiguredNotation` asplain case writes JSON numbers; `TestASPathAppendTextStaysAsplain` pins the filter text; `internal/component/lg/testdata/handler/api-protocols-bgp.txt` is the birdwatcher golden and its `neighbor_as` stays an integer |
| No on-wire change | reasoning at the producer, no scenario owed | `asn` holds no encoder. Every wire path still carries `uint32`, and `attribute.(*ASPath).AppendText` writes `NotationPlain` unconditionally (`internal/core/bgp/attribute/text_append.go`). The Interop row of this spec records why no scenario is owed |
| A seventh private AS-number parser cannot land unnoticed | gate | `checkNumberParseSites` (`internal/le/repository/numberparse.go`), registered in `Run` (`repository.go`) so both `./le repository check` and `./le repository tree-check` run it. `./le repository tree-check` passes over this tree, and `TestNumberParseAllowlistMatchesThisTree` re-verifies the baseline on every unit run |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| none | Every acceptance criterion has product code and a test. Three product files are written and verified but could not be COMMITTED by this session; they are not undone work and they are recorded in Deviations, not here | - |

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/core/bgp/asn/asn.go` | yes | `ls -la` -> 15K |
| `internal/component/bgp/plugins/rib/rib_asn_notation.go` | yes | `ls -la` -> 931 bytes |
| `internal/component/bgp/config/asn_notation.go` | yes | `ls -la` -> 1.9K |
| `internal/le/repository/numberparse.go` | yes | `ls -la` -> 8.3K |
| `internal/le/repository/numberparse-allowlist.txt` | yes | `ls -la` -> 5.8K, 95 file lines |
| `test/plugin/bgp-as-notation.ci` | yes | `ls -la` -> 2.1K |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1, AC-2, AC-7 | `asn.Parse` reads all three spellings and refuses a field above 65535 | `go test -run 'TestParseAsdot|TestParseRejects' ./internal/core/bgp/asn/` -> `--- PASS` for both, `ok ... 0.331s` |
| AC-1 at the config leaf | the `zt:asn` typedef is what marks an ASN leaf, and the YANG range applies to the number the dotted token names | `go test -run 'TestParseASNAsdot|TestValidateASNLeafRange|TestASNTypedefMapsToTypeASN' ./internal/component/config/` -> three `--- PASS` |
| AC-3, AC-4, AC-5 | `asn.Append` renders each notation, and allocates nothing | `go test -run 'TestAppendNotation|TestAppendRoundTrips|TestAppendAllocatesNothing' ./internal/core/bgp/asn/` -> three `--- PASS` |
| AC-6 | an absent leaf selects asplain, and a payload written under it is a JSON number | `go test -run 'TestConfigureFromBGP|TestNumberJSONRoundTrip|TestFromJSON' ./internal/core/bgp/asn/` -> three `--- PASS` |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `bgp { session { asn { local 1.10; } } }` | `test/plugin/bgp-as-notation.ci` | Read at closure. The config block writes `local 1.10` and `remote 1.10`, and the daemon must accept it before any expectation can match |
| `bgp { as-notation asdot; }` -> `asPathList` | `test/plugin/bgp-as-notation.ci` | `expect=stderr:pattern=AS-NOTATION OK as-path 1\.10 100`, written by `bgpASNotation13` from the `show bgp rib received` answer |
| `bgp { as-notation asdot; }` -> `show bgp peer detail` | `test/plugin/bgp-as-notation.ci` | `expect=stderr:pattern=AS-NOTATION OK remote-as 1\.10`, written from a SECOND producer, so the leaf is proven to reach more than one row builder |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | broken | Eight private AS-number parsers existed, not one funnel. Mistake Log row A-1; the full population is "Every Text-to-AS-Number Parser In The Tree" |
| A-2 | broken | A notation parameter on `(*ASPath).AppendText` had no product caller, because every caller of it is a machine contract. Mistake Log row A-2; the variant was deleted rather than left unwired |
| A-3 | broken | Fixing `ParseASPathText` fixed nothing an operator reaches, because the `as-path` command word went to a second parser. Mistake Log row A-3 |
| A-4 | broken | A grep for the readers did not find them all. Mistake Log row A-4 |
| A-5 | broken | A leaf-driven sweep cannot terminate. Mistake Log row A-5, and the reason the parser-side enumeration and `checkNumberParseSites` exist |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| New user-facing feature -> `docs/features/bgp-protocol.md`, "AS number notation" | Anchors naming `asn.Parse`/`Append`/`Number`/`Configured`, `attribute.ParseASPathText`, `nlri.ParseRDString`, the `as-notation` leaf, and `AppendText` staying asplain | Each named symbol read at closure. The page's `ze-analyse` was corrected to `ze-analyze`, which is the name the binary prints (`internal/analyze/aspath.go`) |
| Config syntax changed -> `docs/guide/configuration.md`, "AS Number Notation" | `<!-- source: internal/component/bgp/yang/ze-bgp-conf.yang -- as-notation leaf -->` | Already in HEAD: another session carried this file's hunk in an earlier commit. Verified with `git show HEAD:docs/guide/configuration.md` |
| API contract -> `docs/architecture/api/birdwatcher-compat.md` | The birdwatcher `neighbor_as` stays an integer whatever the notation | `internal/component/lg/handler_api.go` `setNeighborAS`, and the recaptured golden `internal/component/lg/testdata/handler/api-protocols-bgp.txt` |
| Tooling -> `ai/INDEX.md`, `ai/PACKAGE-MAP.md`, `internal/le/repository/{register,actions,repository}.go` | The `./le repository` description now names six checks and the allowlist path | `./le repository tree-check` -> "all checks passed", so the sixth check runs in the gate as well as in the developer command |
| RFC status | No row changed | RFC 5396 is a notation document, not a protocol requirement Ze enforces on the wire; `rfc/full/rfc5396.txt` is fetched and quoted at the producers. No `rfc/short/` summary claims changed |
| Doctor check | Not applicable | The change adds no runtime dependency: no file path, socket, kernel module, port, binary or certificate |

## Core Insight

A display notation changes the TYPE of a payload field, and that is the part
that is invisible from the config leaf. `1.10` is not a JSON number, so every
in-tree consumer of an AS number field had to be able to read a string. Three
could not, and each failed the WHOLE decode rather than one field: the CLI
dashboard showed an empty peer list, shell completion offered nothing, and the
looking glass dropped every AS number of 65536 or more from its topology graph.
A feature that only ever rendered would have shipped with all three broken and
no test red.

## Review Gate

<!-- BLOCKING (ai/rules/planning.md Review Gate). Filled by /ze-implement's /ze-review gate: -->
<!-- the final review before closure, run AFTER the inline critical/security/doc reviews, over the complete diff. -->
<!-- Every BLOCKER and ISSUE (severity > NOTE) must be fixed, then re-run /ze-review. -->
<!-- Loop until the review returns 0 BLOCKER/0 ISSUE (only NOTEs, or nothing). Paste the final clean run. -->
<!-- NOTE-only findings do not block — record them and proceed. -->

### Round 1 scope (fixed before the round ran, 2026-09-08)

The WHOLE diff of this spec, with at least two lenses. Files in scope:
`internal/core/bgp/asn/`, the `as-notation` leaf in
`internal/component/bgp/yang/ze-bgp-conf.yang`, the ASN readers in
`internal/component/config/` and `internal/component/cli/`, and every renderer
the second pass wired or deliberately excluded: `rib_attr_format.go`,
`cmd/peer/`, `plugins/rpki/rpki.go`, `plugins/filter_irr/command.go`,
`plugins/completion/peers.go`, `cli/model_dashboard*`, `internal/component/lg/`,
`internal/component/web/page_bgp_*`, and the excluded text paths in
`internal/core/bgp/attribute/` and `internal/component/bgp/format*`.

Out of scope, because they belong to other sessions in flight in this shared
checkout: `internal/component/bgp/reactor/` BFD and update-delay work,
`internal/component/bgp/fsm/`, the capability packages,
`internal/component/config/transaction/`, `internal/component/iface/`, and the
`plugin/yang` rename.

Always-in-scope classes apply anywhere they are found, per `ai/rules/planning.md`.

### Artifact

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/bgp-as-notation-828bf96b-ed79-41e5-bc2d-b82d08d4124a.md`, 124 code files, verdict clean |
| `./le spec session review check` | `review_gate: OK (124 code files, clean, hashes match ...)` |
| Rounds | 8. Each ran in its own independent context. The closure pass below is a separate phase, not a ninth round: Thomas ruled on 2026-09-09 that no ninth round runs and that closure's own lenses cover the round-8 fixes |
| Rounds reason | Round 7 found a PRODUCT defect: `selector.parseASNSelector` read decimal only, and `ParseDefault` turned its refusal into a peer NAME, so a dotted peer selector matched no peer and reported nothing |
| Owner authorisation | Thomas authorised rounds 6, 7 and 8 individually on 2026-09-09, each after the previous round found one more private AS-number parser. On 2026-09-09 he decided no ninth review round runs and the spec closes, because the round-8 fixes changed no product code and closure's own lenses cover them |
| Reviewer lenses used | Rounds 1-8: spec-completeness, wiring, RFC conformance, silent-failure, and (rounds 5-8) an exhaustive parser-side sweep. Closure: fail-closed/silent-zero over every new producer, and the `docs/contributing/ze-go-style.md` style pass over every changed Go file |

### Findings fixed

Rounds 1 to 8 ran in their own contexts and their findings are recorded in the
scope blocks below, which name what each round fixed. The rows here are the
PRODUCT defects, one per round that found one.

| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | BLOCKER | The notation was recorded during config RESOLUTION, so `ze config validate`, `ze doctor` and a refused web commit each changed what a running daemon rendered | `ResolveBGPTree`, `cli/editor.go` | The write moved to the two points where a candidate BECOMES the running configuration: `applyASNotation` on first load and `SetConfigTree` on reload |
| 2 | BLOCKER | Three in-tree consumers declared the AS number field as a plain uint32, so a dotted payload failed the WHOLE decode: an empty CLI peer list, no shell completions, and a looking-glass topology graph missing every AS number of 65536 or more | `cli.dashboardPeer`, `completion.peerEntry`, `lg.extractASPath` | All three read `asn.Number` or `asn.FromJSON` |
| 3 | BLOCKER | The `as-path` command word reached `Builder.ParseASPath`, a second decimal-only parser, while the fixed `ParseASPathText` had no product caller at all | `route.parseCommonAttributeBuilder` | The word calls `attribute.ParseASPathText`; `Builder.ParseASPath` and `route.parseASPath` are deleted |
| 4 | BLOCKER | Two route distinguisher readers, then four, parsed their AS half decimal-only, three functions from readers already fixed | `config.ParseRouteDistinguisher`, `nlri.ParseRDString`, `flowspec.flowRDStringToBytes`, `mvpn.rdStringToBytes` | All four read `asn.Parse`; `ParseRDString` is the one reader the 19 `rd` command words reach |
| 5 | BLOCKER | Four packages each parsed the extended-community administrator their own way, and agreed only because all four were wrong the same way. `ParseSingleExtCommunity` also chose its IPv4 branch on a search for a dot, so `target:1.10:5` was read as an address | `attribute`, `bgpconfig`, `route` | `attribute.ParseExtCommunityAdmin` is the one reader for all four call sites, and the IPv4 branch probes with `netip` |
| 6 | BLOCKER | `selector.parseASNSelector` read decimal only. `ParseDefault` turns its refusal into a peer NAME, so `as1.10` silently matched no peer and reported nothing | `internal/core/selector/selector.go` | Exported as `ParseASNSelector`, reading `asn.Parse`; `cmd/policy` calls it instead of carrying its own copy |
| 7 | ISSUE | Nothing could stop an eighth private parser landing, because each sweep was only as good as the pattern it searched for | tree-wide | `checkNumberParseSites` (`internal/le/repository/numberparse.go`), an exact-count allowlist of every 32-bit text-to-integer parse in product Go, registered in the repository gate |
| 8 | ISSUE | The gate's own file failed `./le verify lint run`: `sort.Strings` where the repository's ruleguard rule requires `slices.Sort`, two unannotated `os.ReadFile` calls (gosec G304), and a `filepath.Join` argument holding path separators | `internal/le/repository/numberparse.go`, `numberparse_test.go` | Found by this closure pass. `slices.Sort`, two `//nolint:gosec` lines carrying the reason the sibling checks carry, and a joined fixture path |
| 9 | ISSUE | Two conversions became unnecessary when their callee started returning `uint32`, and both carried a now-false `//nolint:gosec // G115` comment claiming a bound that no longer applies | `attribute.ParseSingleExtCommunity`, `rpki.validateCommand` | Found by this closure pass. Both conversions and both stale annotations removed |
| 10 | NOTE | Three comments named the binary `ze-analyse`; the binary prints `ze-analyze` | `internal/analyze/asn_notation_test.go`, `docs/features/bgp-protocol.md` | Corrected, and the test renamed to `TestAnalyzeASFlagsReadEveryNotation` |

### Fixes applied
- `internal/le/repository/numberparse.go`: `sort.Strings` -> `slices.Sort` (two call sites, `sort` import replaced by `slices`), and `//nolint:gosec` with the reason on both `os.ReadFile` calls, matching `repository.go` and `wiring.go`.
- `internal/le/repository/numberparse_test.go`: the fixture path is built with `filepath.Join(root, "internal", "component", "thing", "thing.go")` once and reused.
- `internal/core/bgp/attribute/builder_parse.go`: `binary.BigEndian.PutUint32(ec[2:6], asn)` with the stale `//nolint:gosec // G115` dropped, because `ParseExtCommunityAdmin` returns a `uint32`.
- `internal/component/bgp/plugins/rpki/rpki.go`: `asn.JSONValue(originAS)` with the same stale annotation dropped.
- `internal/analyze/asn_notation_test.go` and `docs/features/bgp-protocol.md`: `ze-analyse` -> `ze-analyze`.


### Round 2 scope (fixed before the round ran, 2026-09-08)

ONLY the fixes round 1 produced, plus the sibling call sites they touched:
`internal/component/bgp/config/asn_notation.go` `applyASNotation` and its two
call sites in `loader_create.go`, the removal of the write from
`ResolveBGPTree` and from `cli/editor.go`, the `asn.Number` struct change in
`internal/core/bgp/asn/asn.go` (`Of`, `Value`, `String`, `MarshalJSON`,
`TextFromJSON`) and every caller that now constructs or decodes one,
`lg/handler_api.go` `asPathNumbers`, the `zt:asn-notated` typedef in
`ze-types.yang` with its uses in `ze-bgp-api.yang` and `ze-peer-cmd.yang`,
`TestASPathAppendTextStaysAsplain`, and the three doc pages touched by the
fixes.

Plus the eight always-in-scope classes, which apply anywhere they are found.

Not in scope: anything round 1 already passed and the fixes did not touch.


### Round 3 scope (fixed before the round ran, 2026-09-08)

ONLY the fixes round 2 produced, plus the sibling call sites they touched:
`reactorAPIAdapter.SetConfigTree` and `recordASNotation`
(`internal/component/bgp/reactor/reactor_api.go`) with the two coordinator call
sites in `internal/component/plugin/server/reload.go`, the removal of the write
from the reload closure in `loader_create.go` and the one surviving
`applyASNotation` call on the first-load path, `lg/handler_api.go`
`setNeighborAS` and the recaptured golden
`internal/component/lg/testdata/handler/api-protocols-bgp.txt`, the YANG
type reversions in `ze-bgp-api.yang` (`notification session-established` leaf
`asn`, `peer-show-statistics` `peer-as`), `Number.UnmarshalJSON`'s null
tolerance, the corrected test docstrings and comments, and
`docs/features/bgp-protocol.md`'s corrected asdot+ example.

Plus the eight always-in-scope classes, which apply anywhere they are found.

Not in scope: anything rounds 1 and 2 cleared that these fixes did not touch.


### Round 4 scope (fixed before the round ran, 2026-09-08)

ONLY the fixes round 3 produced, plus the sibling call sites they touched:
`internal/component/bgp/config/asn_notation_reload_test.go` and the renamed
`TestVerifyingACandidateOnTheTreeBranch...` in
`internal/component/bgp/reactor/asn_notation_test.go` with their docstrings,
`lg/handler_api.go` `setNeighborAS` (now always writing the key) with both
protocol builders and `getVal`, the `peer-show-statistics` `peer-as` leaf back
to `zt:asn-notated` and the `session-established` `asn` leaf's corrected
description in `ze-bgp-api.yang`, `docs/architecture/api/birdwatcher-compat.md`,
the `as-notation` `ze:help` worked example in `ze-bgp-conf.yang`, and the moved
`configRootNameBGP`.

Plus the eight always-in-scope classes, which apply anywhere they are found.

Not in scope: anything rounds 1 to 3 cleared that these fixes did not touch,
including the `SetConfigTree` caller enumeration and the first-load path, both
settled in round 3.


### Round 5 scope (fixed before the round ran, 2026-09-09)

ONLY the fixes round 4 produced, plus the sibling call sites they touched:
the reload guard's call-assertion in `asn_notation_reload_test.go`, the
`peer-show-statistics` output leaf now named `remote-as`, the birdwatcher
contract comments at `asPathNumbers` and `setNeighborAS` with the report-bus
fault, the `session-established asn` description, the blank `bgp/yang` import in
`bgp/config` with its measurement comment, `Number.UnmarshalJSON`'s null
acceptance, `plan/journal/unwired-feature.md`'s new row, and every leaf retyped
in the AC-1 sweep: `reject-asn`'s six leaves, `firewall irr` `source-asn` and
`destination-asn` with the whois key normalisation, the `update`/`clear firewall
irr asn` reader, the seven `resolve` command and API `asn` inputs, and the
parser-level fixes at `config/routeattr.go` `ParseASPath` and `ParseAggregator`.

Plus the eight always-in-scope classes, which apply anywhere they are found.

Not in scope: anything rounds 1 to 4 cleared that these fixes did not touch.


### Round 6 scope (owner-authorised, fixed before the round ran, 2026-09-09)

Thomas authorised this round on 2026-09-09 after round 5 hit the five-round cap
with four product defects outstanding.

ONLY the fixes round 5 produced, plus the sibling call sites they touched: the
deletion of `attribute.Builder.ParseASPath` with `parseCommonAttributeBuilder`'s
`as-path` branch now calling `attribute.ParseASPathText`
(`internal/component/bgp/route/route.go`) and the benchmark caller in
`internal/component/bgp/reactor/filter_format_bench_test.go`,
`ParseRouteDistinguisher`'s AS branch reading `asn.Parse` with its shadowing
rename, `firewall.IRRASNName` as the single declaration with `irrSetMatch` and
the irr plugin's `extractFromBlock` both calling it and the plugin's `asnName`
removed, the two RFC-grounded exclusion reasons written at `ParseCommunity` and
`ParseLargeCommunity` with the spec and `docs/features/bgp-protocol.md`
corrected, `ParseNotation` unexported to `parseNotation`, the corrected
`float64` assertion in `lg/handler_api_test.go`, and the five new
`asn_notation_test.go` files covering the irr set-name convergence and whois
key, `clear firewall irr asn`, `requireASN`, `resolve rir`, `resolve cymru` and
`ospf inter-as remote-as`.

Plus the eight always-in-scope classes, which apply anywhere they are found.

Not in scope: anything rounds 1 to 5 cleared that these fixes did not touch.


### Round 7 scope (owner-authorised, fixed before the round ran, 2026-09-09)

Thomas authorised this round on 2026-09-09, choosing an EXHAUSTIVE
parser-driven sweep over a single-defect fix, because rounds 5 and 6 each found
one more text-to-ASN parser that the leaf-driven sweep had missed.

In scope: `nlri.ParseRDString` and its 19 call sites, the
`docs/features/bgp-protocol.md` sentence about route distinguishers, spec lesson
A-4, and the open-coded IRR element name in `firewall/plugins/irr/command.go`
`updateASN` and `clearASN`. Plus the sweep itself: every function in the tree
that turns text into an AS number, enumerated from the PARSER side, each fixed
or excluded with a recorded verdict, and the enumeration written into the spec.

Plus the eight always-in-scope classes, which apply anywhere they are found.

Not in scope: anything rounds 1 to 6 cleared that these fixes did not touch.


Round 7 scope EXTENDED (2026-09-09), same authorised round: the
extended-community administrator work — `attribute.ParseExtCommunityAdmin` as
the single reader with its four callers (`ParseSingleExtCommunity`,
`FlowSpecRedirect`, `bgpconfig.parseExtCommunityASN`,
`route.parseRouteTargetExtCommunity`), the netip probe replacing the dot search
in `ParseSingleExtCommunity`, the `L`-suffix rule, the two 2-octet exclusions
(`route.parseOriginExtCommunity`, `filter_community.parseExtendedWire`), the
five new tests, the corrected `TestParseExtendedCommunityHex`, and the
enumeration table's replacement section.


### Round 8 scope (owner-authorised, fixed before the round ran, 2026-09-09)

Thomas authorised this round on 2026-09-09, choosing fixes PLUS a permanent
gate over another exhaustion pass, because rounds 5, 6 and 7 each found one more
parser and each sweep was only as good as the pattern it searched for.

ONLY the fixes round 7 produced, plus the sibling call sites they touched:
`selector.ParseASNSelector` exported and reading `asn.Parse` with
`cmd/policy/handler.go` calling it instead of its own copy, the new
`checkNumberParseSites` gate (`internal/le/repository/numberparse.go`) with its
allowlist, its registration in the repository stage, its six unit tests and its
documentation, the `ze-analyze` AS flag readers (`--peer-asn`, three
`--local-as`), the `nlri.ParseRDString` declared-type-wins rule with its
corrected doc comment, the enumeration additions including the new external
protocol text group, the L-suffix test now pinning bytes, and the corrected
`TestParseASNRejection` cases.

Plus the eight always-in-scope classes, which apply anywhere they are found.

Not in scope: anything rounds 1 to 7 cleared that these fixes did not touch.

### Closure pass (2026-09-09, independent context, no product code authored here)

Two lenses over the whole diff, each read at the producer.

**Fail-closed and the silent zero** (`ai/rules/principles.md`). Every new
producer refuses rather than answering a value nobody wrote: `asn.Parse` errors
on both branches, `asn.FromJSON` and `asn.TextFromJSON` answer `false` for a
value naming no AS number, `asn.ConfigureFromBGP` errors on a leaf that is not a
string and refuses an empty one, and `readNumberParseAllowlist` treats a missing
allowlist as an ERROR unless the tree parses nothing. The one zero that IS an
answer, `NotationPlain`, is documented as such and is what RFC 5396 Section 3
recommends. No finding.

**The style pass** (`docs/contributing/ze-go-style.md`, `/ze-review` step 18)
over every changed Go file. The `panic("BUG: unknown AS notation")` in
`Notation.String` and `Notation.plain` is not peer-reachable: a `Notation` is
only ever produced by `parseNotation` or by loading the atomic that
`parseNotation` fills, so no socket reaches it. Three findings, all listed above
as rows 8, 9 and 10 and all fixed here.

`./le verify lint run` after the fixes reports no finding in any file of this
spec's diff. `./le repository tree-check` passes, which is the sixth check
running over its own baseline.

### Final status
- [ ] 0 BLOCKER, 0 ISSUE outstanding: every row above is fixed, and the closure pass's own three findings were fixed in this pass
- [ ] NOTEs recorded above (row 10 is the only one)

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-7 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and passing test
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean
- [ ] `./le verify worktree` passes
- [ ] Feature code integrated (`internal/*`)
- [ ] Documentation Update Checklist answered

### Quality Gates (SHOULD pass)
- [ ] Implementation Audit complete

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (X, Y, combined ASN)
- [ ] Functional tests for end-to-end behavior
