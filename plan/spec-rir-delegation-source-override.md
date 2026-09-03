# Spec: an operator points the RIR delegation fetch at a mirror

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | config |
| Depends | - |
| Phase | 5/5 |
| Deferral shard | - |
| Handoff | - |
| Updated | 2026-09-03 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

`update resolve rir` fetches five delegation files from five registry hosts, and
nothing an operator can write changes where it fetches them from.
`httpDelegationFetch` reads `rirDelegationURLs`, a package variable in
`internal/component/resolve/irr/rir.go`, and no YANG leaf and no `ze.*`
environment variable reaches it.

Two costs follow from that one fact. An appliance behind a mirror, a filtered
network or an air gap cannot refresh the table at all, and its answer to "which
registry holds this AS number" stays frozen at the seed the binary shipped with.
And no functional test can prove a SUCCESSFUL refresh: a test must not fetch tens
of megabytes from five public registries, so `plugin_fixture_resolve_rir.go` says
in its own comment that Ze offers no way to redirect them and settles for forcing
the failure path. The successful path has in-process unit coverage only.

The goal is one operator setting: per registry, the URL its delegation file is
read from, defaulting to the registry's own published file.

## Required Reading

### Architecture Docs
- [ ] `ai/rules/config.md` - which surface a setting belongs to
  → Constraint: "A tunable setting MUST default to a YANG leaf; env-only is the
    exception, reserved for an emergency override, debug instrumentation, a
    bootstrap value read before config parses, or an internal safety cap." A
    mirror URL is none of the four, so it is a YANG leaf
- [ ] `ai/patterns/config-option.md` - the structural template a new leaf follows
  → Constraint: a leaf takes maximum native validation, and a value needing more
    than `range`, `length`, `pattern` or `enumeration` takes `ze:validate` with a
    `ValidateFn`
- [ ] `docs/architecture/resolve.md` - the page that owns this component
  → Constraint: an external service's endpoint sits under `system`, not under
    the protocol that consumes it, and the page already records why
  → Decision: the page carries two defects this work repairs. Its Construction
    section anchors `newResolvers` at `cmd/ze/hub/main.go` when the function is
    in `cmd/ze/hub/main_system.go`, and it says PeeringDB and IRR "are created
    independently with their configured server addresses" when `newResolvers`
    passes `irr.NewIRR("")` and `NewIRR` substitutes `whois.radb.net`
- [ ] `docs/architecture/config/system-update.md`, "The URL rule, and the way it
  was got wrong" - the one written position on plain HTTP in this repo
  → Decision: `ValidateUpdateCheckURL` accepts HTTPS, and plain HTTP only for
    `127.0.0.1` and three exact localhost spellings. This spec takes the same
    rule for the same reason, and the loopback exception is what lets a
    functional test serve fixture files

**Key insights:** (minimal context to resume after compaction)
- Five hosts, five per-registry paths, so no single base substitution reaches
  all five. The setting is per registry or it is wrong.
- `SystemConfig` is built once at startup by `ExtractSystemConfig` and is not
  re-applied to the resolvers on a config commit, so a value injected at startup
  goes stale until restart. A refresh command must read the tree when it runs.
- `rirDelegationURLs` has a SECOND reader: `RenderDelegationTable` writes one
  `# Source:` line per entry into the seed and into every stored copy. A
  redirect that mutates that variable silently rewrites provenance.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/component/resolve/irr/rir.go` - `rirDelegationURLs`,
  `httpDelegationFetch`, `FetchDelegationTable`, `DelegationFetch`,
  `RenderDelegationTable`, `DelegationTable`, `rirNames`.
  → Constraint: `FetchDelegationTable` already takes a `DelegationFetch` seam
    that production leaves nil, so the fetch is parameterized and only its URLs
    are not
  → Constraint: `RenderDelegationTable` reads `rirDelegationURLs` directly for
    its `Source:` lines
- [ ] `internal/component/resolve/cmd/rir.go` - `handleRIRRefresh`, the daemon's
  only production caller of the fetch.
- [ ] `internal/le/ianaasn/ianaasn.go` - `Write`, the other caller, on a
  developer machine with no daemon and no config tree.
  → Decision: the generator keeps the registries' own URLs. A build host has no
    operator config, and the shipped seed should carry registry data
- [ ] `internal/component/config/system/system.go` - `SystemConfig`,
  `ExtractSystemConfig`, and `PeeringDBURL` as the closest precedent.
  → Constraint: `ExtractSystemConfig` reads the whole `system` container into a
    struct, and the hub calls it once at startup
- [ ] `internal/component/bgp/plugins/cmd/peer/prefix_update.go` - an
  operator-initiated handler that opens the config with `cli.NewEditor` and
  calls `ExtractSystemConfig` on that tree at COMMAND time.
  → Decision: this is the precedent `handleRIRRefresh` follows, because it is
    the same kind of handler and it sees a value committed after startup
- [ ] `internal/component/config/system/update.go` - `ValidateUpdateCheckURL`.
- [ ] `internal/test/fixture/plugin_fixture_resolve_rir.go` - the fixture that
  forces the failure path with a closed-port proxy, and says why.

**Behavior to preserve:**
- A daemon with no such configuration fetches exactly the five registry URLs it
  fetches today.
- `./le iana-asn write` keeps reading the registries' own files.
- The refresh stays all or nothing, and still never reports success for a run
  that stored nothing.

**Behavior to change:**
- An operator names a URL per registry, and the refresh reads it.
- The `Source:` lines of a stored table name the URLs that run actually read.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- The `system` container of the configuration, committed by an operator.
- `update resolve rir`, which reads that configuration when it runs.

### Transformation Path
1. The operator commits a URL for one or more of the five registries.
2. `handleRIRRefresh` opens the config tree at command time and extracts the
   delegation sources, as `prefix_update.go` does for the PeeringDB URL.
3. Each source is validated before a byte is fetched: HTTPS, or plain HTTP on
   loopback.
4. The sources are passed INTO `FetchDelegationTable` as an argument. The `irr`
   package holds no configuration and reads no tree.
5. The table carries the URLs it was built from, and `RenderDelegationTable`
   writes those as its `Source:` lines.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config tree ↔ refresh handler | `ExtractSystemConfig` on an editor tree opened at command time | No |
| Handler ↔ `irr` | the sources as a function argument, never a package variable | No |
| Ze ↔ a mirror | HTTPS, or plain HTTP on loopback only | No |
| Stored table ↔ its provenance | the fetched URLs travel in `DelegationTable` | No |

### Integration Points
- `internal/component/config/system/yang/ze-system-conf.yang` - the new nodes.
- `system.SystemConfig` and `ExtractSystemConfig` - the extracted value.
- `irr.FetchDelegationTable` and `irr.RenderDelegationTable` - the argument and
  the provenance.
- `test/plugin/resolve-rir-refresh.ci` - the successful path it cannot reach today.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | No | |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | A refresh must read the config tree when it runs, because `SystemConfig` is not re-applied to the resolvers on commit | `ExtractSystemConfig` is called at hub startup by `main`, and `prefix_update.go` re-reads the tree at command time for the same reason | A startup-injected value serves a stale mirror until restart, with nothing saying so | A functional test that commits a source and refreshes without restarting | confirmed: `TestRefreshReadsTheSourcesCommittedAfterStartup` commits a source after the server is built and the refresh reads it |
| A-2 | A mirror copies each registry's file under its own path, so per-registry URLs are the right shape | The five published URLs sit on five hosts with five paths | A base-plus-filename shape would be simpler for an operator who mirrors the whole tree | Name a real mirror layout before implementing, and say which shape it needs | confirmed by construction: the five files sit on five hosts, so no base substitution reaches them. `TestOneConfiguredSourceLeavesTheOtherFourPublished` pins the per-registry shape |
| A-3 | The loopback exception is enough for a functional test to serve fixture delegation files | `ValidateUpdateCheckURL` accepts `http://127.0.0.1`, and the existing fixture already runs a local server | The `.ci` cannot prove the successful path and AC-4 of the previous spec stays unproven | Write the `.ci` first and watch it reach the fixture server | confirmed: `resolve-rir-refresh.ci` serves the five files from a loopback server and the successful refresh passes |
| A-4 | Nothing else reads `rirDelegationURLs` besides the fetch and the renderer | `gopls references` over the symbol | A third reader changes behavior when the sources become an argument | Run the reference query before editing | confirmed: the two readers were the fetch and the renderer, and both now take the URLs as data. The variable is `publishedDelegation`, read only by `delegationSourceURLs` |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A hostile or stale mirror misattributes AS numbers, and Ze reports the wrong registry with no sign anything is unusual | The stored table's `Source:` lines name a host nobody expects | Provenance travels with the table, the scheme rule keeps the transport authenticated, and `show resolve rir` reports the date the table was generated |
| R-2 | Plain HTTP is accepted somewhere beyond loopback, by a validator that only checks a prefix | A URL such as `http://127.0.0.1.example.com` passing | Take the rule from `ValidateUpdateCheckURL` and test the near-miss spellings explicitly |
| R-3 | A partial override leaves four registry URLs and one mirror, and an operator believes the whole table came from the mirror | The `Source:` lines disagree with the operator's expectation | The setting is per registry by design, and the stored header names every URL actually read |
| R-4 | The provenance change breaks the byte-exactness test that pins the shipped seed | `TestTheShippedSeedIsWhatTheRendererWrites` goes red | The generator passes the registry URLs explicitly, so the shipped bytes do not move |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A refresh reads the wrong host, or refuses a URL an operator legitimately wants. The lookup keeps answering from the shipped seed either way, so no answer is lost |
| How is it reverted? | Single commit revert. The leaf is additive and an absent value is today's behavior |
| Who else touches this path? | `internal/le/ianaasn` calls the same fetch; `test/plugin/resolve-rir-refresh.ci` asserts the failure path today |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| A committed `system` delegation source | → | `ExtractSystemConfig` filling the new field | `TestDelegationSourcesAreExtractedFromTheTree` |
| `update resolve rir` with a source committed | → | `handleRIRRefresh` reading the tree at command time | `TestRefreshReadsTheSourcesCommittedAfterStartup` |
| A source that is not HTTPS and not loopback | → | the validator, before any fetch | `TestANonLoopbackPlainHTTPSourceIsRefused` |
| `update resolve rir` against a local fixture server | → | the whole path, ending in the stored table | `test/plugin/resolve-rir-refresh.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | No delegation source configured | The refresh fetches the five registry URLs, exactly as it does today |
| AC-2 | A source configured for one registry | That registry's file is read from the configured URL and the other four from their published URLs |
| AC-3 | A source configured for a name that is not one of the five registry tokens | The configuration is refused at validation time, naming the tokens that exist |
| AC-4 | A source whose scheme is plain HTTP and whose host is not loopback | Refused before any byte is fetched, and the message names the URL and the rule |
| AC-5 | A source whose host is `127.0.0.1` over plain HTTP | Accepted, which is what a functional test and an on-box mirror both need |
| AC-6 | A refresh whose configured source cannot be reached | Stores nothing, names the URL it could not read, and leaves the previous table answering |
| AC-7 | A successful refresh from a configured source | The stored table's `Source:` lines name the URLs actually read, not the registries' published URLs |
| AC-8 | A source committed after the daemon started, with no restart | The next `update resolve rir` reads it |
| AC-9 | `./le iana-asn write` | Reads the registries' published URLs, and the shipped seed's bytes are unchanged by this work |
| AC-10 | `update resolve rir` driven end to end against a local fixture server | Fetches, parses, stores, and answers the key, the range count and the date, proving the successful path in a functional test for the first time |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Runs an appliance behind a mirror, points the five sources at it, and refreshes | config commit → `handleRIRRefresh` reads the tree → fetch from the mirror → stored table | `test/plugin/resolve-rir-refresh.ci` |
| 2 | Mirrors one registry only, because one is blocked | config commit for one token → four published URLs and one mirror | `TestOneConfiguredSourceLeavesTheOtherFourPublished` |
| 3 | Audits where a stored table came from | `show resolve rir` and the stored table's `Source:` lines | `TestTheStoredTableNamesTheURLsItWasBuiltFrom` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestDelegationSourcesAreExtractedFromTheTree` | `internal/component/config/system/system_test.go` | AC-2, the extraction | |
| `TestAnUnknownRegistryTokenIsRefused` | `internal/component/config/system/system_test.go` | AC-3 | |
| `TestANonLoopbackPlainHTTPSourceIsRefused` | `internal/component/config/system/system_test.go` | AC-4, with the near-miss host spellings of R-2 | |
| `TestALoopbackPlainHTTPSourceIsAccepted` | `internal/component/config/system/system_test.go` | AC-5 | |
| `TestOneConfiguredSourceLeavesTheOtherFourPublished` | `internal/component/resolve/irr/rir_test.go` | AC-2, story 2 | |
| `TestTheStoredTableNamesTheURLsItWasBuiltFrom` | `internal/component/resolve/irr/rir_test.go` | AC-7, story 3 | |
| `TestRefreshReadsTheSourcesCommittedAfterStartup` | `internal/component/resolve/cmd/rir_test.go` | AC-8, A-1 | |
| `TestRefreshNamesAConfiguredSourceItCannotRead` | `internal/component/resolve/cmd/rir_test.go` | AC-6 | |
| `TestTheShippedSeedIsWhatTheRendererWrites` | `internal/component/resolve/irr/rir_test.go` | AC-9, existing test kept green | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Configured sources | 0-5 entries | 5 | N/A | a sixth entry names a token that does not exist (AC-3) |
| URL length | 1-1024 | 1024 | empty string | 1025 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `resolve-rir-refresh` | `test/plugin/resolve-rir-refresh.ci` | the operator refreshes from a mirror and reads the answer back, and the existing failure-path assertions stay | |

### Interop Tests (Scope: protocol)
Not applicable: no wire-visible protocol behavior changes, no protocol peer.

## Files to Modify
- `internal/component/config/system/yang/ze-system-conf.yang` - the new nodes
- `internal/component/config/system/system.go` - the field and its extraction
- `internal/component/config/system/update.go` or a sibling - the scheme rule,
  reused rather than copied
- `internal/component/resolve/irr/rir.go` - the sources as an argument, and the
  provenance carried by the table
- `internal/component/resolve/cmd/rir.go` - the command-time tree read
- `internal/le/ianaasn/ianaasn.go` - passes the published URLs explicitly
- `internal/test/fixture/plugin_fixture_resolve_rir.go` - the successful path
- `test/plugin/resolve-rir-refresh.ci` - the new assertions
- `docs/architecture/resolve.md` - the setting, and the two page defects above
- `docs/guide/configuration.md`, `docs/guide/command-reference.md`,
  `docs/features.md` - the operator-facing description

## Files to Create
- none beyond test fixtures

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `internal/component/config/system/yang/ze-system-conf.yang`, under `system` |
| YANG validation constraints | Yes | the registry key takes an `enumeration` of the five tokens; the URL takes a `length` and a `pattern` |
| YANG custom validators | Yes | the scheme rule needs more than a pattern, so `ze:validate` with a `ValidateFn`, and a `CompleteFn` offering the five tokens |
| CLI commands/flags | No | No new command. `update resolve rir` gains behavior, not arguments |
| CLI grammar (keyword before value) | N-A | No command surface changes |
| Editor autocomplete | Yes | the registry key is an enumeration, so completion is automatic; the URL takes no completion |
| Functional test for new RPC/API | Yes | `test/plugin/resolve-rir-refresh.ci` |
| Pipe completeness | N-A | The refresh already answers structured data |
| Env var registration | No | `ai/rules/config.md` puts this on a YANG leaf; it is not one of the four env-only exceptions |
| Doctor check for runtime dependencies | Yes | a configured source is a runtime dependency an operator can get wrong: a check that the configured URLs are reachable, with a diagnostic code in `internal/core/diagnostic/codes.go` |
| Prometheus counters/metrics | No | The refresh is operator-initiated and reports its own result |
| BGP family surface (new SAFI / capability / attribute) | N-A | No BGP surface |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | DONE: `docs/features.md`, the ASN-to-registry row gains the per-registry source, the scheme rule and the provenance |
| 2 | Config syntax changed? | Yes | DONE: `docs/guide/configuration.md` gains "RIR Delegation Sources" with the block, the two settings and the defaults. `docs/architecture/config/syntax.md` needs no edit: it documents syntax SHAPES, and a keyed list under a container is the shape it already describes |
| 3 | CLI command added/changed? | Yes | DONE: `docs/guide/command-reference.md`, the `update resolve rir` entry names where each file is read from, the scheme rule, the command-time read and the provenance |
| 4 | API/RPC added/changed? | No | The payload of `ze-update:resolve-rir` is unchanged: `key`, `ranges`, `generated` |
| 5 | Plugin added/changed? | No | No plugin surface |
| 6 | Has a user guide page? | Yes | DONE: `docs/guide/cli.md`, the Resolution Commands section gains the mirror paragraph |
| 7 | Wire format changed? | No | No protocol code |
| 8 | Plugin SDK/protocol changed? | No | No SDK surface |
| 9 | RFC behavior implemented, changed, or newly proven? | No | No RFC governs the delegation file format |
| 10 | Test infrastructure changed? | No | The existing fixture gains a server; no harness change |
| 11 | Affects daemon comparison? | No | No comparable behavior changes |
| 12 | Internal architecture changed? | Yes | DONE: `docs/architecture/resolve.md` gains "Where the files are read from" (the setting, the scheme rule, the command-time read, the provenance), and both defects named in Required Reading are repaired: the `newResolvers` anchor now names `main_system.go`, and the Construction paragraph says the IRR resolver takes an empty server and falls back to RADB |
| 13 | Route metadata keys added/changed? | No | No route metadata |
| 14 | Prometheus counters added/changed? | No | None added |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | No new registration |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | DERIVED: run `./le spec citation anchors spec plan/spec-rir-delegation-source-override.md` |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | Verify the `system` examples against the YANG after the leaf lands |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- the setting exists and reaches the handler
   - Tests: `TestDelegationSourcesAreExtractedFromTheTree`, `TestRefreshReadsTheSourcesCommittedAfterStartup`
   - Files: the YANG module, `system.go`, `internal/component/resolve/cmd/rir.go`
   - Verify: a committed source reaches the handler, which still fetches nothing different because the sources are not yet passed down
2. **Phase: validation** -- the scheme rule and the token rule
   - Tests: `TestANonLoopbackPlainHTTPSourceIsRefused`, `TestALoopbackPlainHTTPSourceIsAccepted`, `TestAnUnknownRegistryTokenIsRefused`
   - Files: the validator, reused from the update-check rule rather than copied
   - Verify: the near-miss host spellings of R-2 are refused
3. **Phase: the sources reach the fetch, and the provenance follows them**
   - Tests: `TestOneConfiguredSourceLeavesTheOtherFourPublished`, `TestTheStoredTableNamesTheURLsItWasBuiltFrom`, `TestTheShippedSeedIsWhatTheRendererWrites`
   - Files: `internal/component/resolve/irr/rir.go`, `internal/le/ianaasn/ianaasn.go`
   - Verify: `rirDelegationURLs` has no reader left that decides behavior
4. **Phase: the functional proof** -- the successful refresh, at last
   - Tests: `test/plugin/resolve-rir-refresh.ci`, `internal/test/fixture/plugin_fixture_resolve_rir.go`
   - Files: the fixture serves delegation files over loopback
   - Verify: the scenario goes red when the sources stop reaching the fetch
5. **Phase: the pages and the doctor check**
   - Files: the documentation rows above, the doctor check and its diagnostic code
   - Verify: `./le doc check links`, and the doctor check fails against an unreachable configured source

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at a named file and symbol |
| Correctness | An absent configuration produces byte-identical behavior to today |
| Correctness | The `Source:` header names what was fetched, on every path that writes a table |
| Security | The scheme rule is the update-check rule, not a second spelling of it, and it refuses the near-miss hosts |
| Data flow | The `irr` package still reads no configuration and no tree |
| Naming | The registry tokens are the ones `rirNames` already declares, not a second list |
| Rule: `ai/rules/config.md` | The setting is a YANG leaf with maximum native validation |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| An operator can name a source per registry | `show configuration` after a commit |
| A mirror is reachable over loopback HTTP and nowhere else in plain text | the validator's tests |
| The successful refresh is proven end to end | `resolve-rir-refresh.ci` reaching the fixture server |
| Provenance follows the fetch | the stored table's `Source:` lines in that test |
| The shipped seed is untouched | `TestTheShippedSeedIsWhatTheRendererWrites` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Transport | HTTPS everywhere but loopback, and the near-miss host spellings refused |
| Trust | The table decides registry attribution, so the stored copy records where it came from |
| Untrusted input | A mirror's response is parsed by the same fail-closed parser, with its size cap and its record rules |
| Resource exhaustion | The per-file size cap and the timeout apply to a configured source exactly as to a published one |
| Fail open | A configured source that cannot be read stores nothing and never reports success |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- The redirect and the provenance are one change, not two. A package variable
  read by both the fetch and the renderer cannot be redirected without silently
  rewriting the header that says where the data came from.
- The rule this reused was itself fail-open. `ValidateUpdateCheckURL` compared
  the loopback host by PREFIX, so `http://127.0.0.1.example.com` passed it, and
  it guards a self-update manifest's download URL as well as the operator's own
  leaf. Reusing a guard means reading it: this one was fixed in the same change,
  and the test goes red against the old form.
- The doctor check judges the CONFIGURED URL and fetches nothing. A reachability
  probe would report the network at that second rather than the configuration,
  and a doctor run must not reach five registries.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Per-registry sources, keyed by the token `rirNames` declares | A single base URL with the published paths appended; a `leaf-list` of five URLs | The five files sit on five hosts with five paths, so no base substitution reaches them. A key says which file is which, where a list binds by position, and it allows mirroring one registry that is blocked |
| An absent entry means the registry's published URL | Requiring all five once any is set | Mirroring one blocked registry is the likely case, and an all-or-nothing setting would make an operator restate four URLs Ze already knows |
| The scheme rule is `ValidateUpdateCheckURL`'s | A YANG `pattern 'https?://.*'`, as the IRR PeeringDB leaves use | This table decides which registry Ze names as holding an AS number, which is the same class of trust as an update source. A pattern cannot express the loopback exception |
| The handler reads the tree at command time | Extending `SystemConfig` at startup and holding it in the resolvers | `SystemConfig` is not re-applied on commit, so a startup value would serve a stale mirror until restart, with nothing saying so |
| The sources are an argument to the fetch | A package variable the handler overwrites | A variable is shared mutable state two callers read, and one of them is the provenance header |

## Known Limitations
- No automatic refresh and no schedule: `update resolve rir` stays operator-initiated.
- No mirror health check beyond the doctor check's reachability probe.
- The generator keeps the registries' published URLs, so a developer regenerating
  the shipped seed on a filtered network still cannot run it.

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
- [ ] Every assumption has a Basis and a validation method; every risk row has an early signal
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints
- [ ] Integration Checklist marks "CLI grammar" when a command is added, "Doctor check" when a runtime dependency is

### Goal Gates (MUST pass)
- [ ] AC-1..AC-10 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Deferral shard resolved: no live row without a destination

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/session/review.go`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)

## Implementation Summary

### What Was Implemented
- `system { rir { delegation-source <token> { url } } }` in
  `internal/component/config/system/yang/ze-system-conf.yang`. The key is an
  `enumeration` of the five registry tokens; the URL takes `length "1..1024"`,
  `pattern "https?://.+"` and `ze:validate "delegation-source-url"`.
- `SystemConfig.RIRDelegationSources` and `extractDelegationSources`
  (`internal/component/config/system/system.go`).
- `ValidateFetchURL` and `DelegationSourceValidator`
  (`internal/component/config/validators.go`), registered as
  `delegation-source-url` in `validators_register.go`.
  `system.ValidateUpdateCheckURL` now delegates to `ValidateFetchURL` rather
  than carrying its own copy of the rule.
- `system` added to `validatedSections`
  (`internal/component/config/validate_sections.go`), which is what makes the
  `ze:validate` above RUN.
- `publishedDelegation`, `delegationSourceURLs`, `registryTokens`,
  `sourceStamp` and `DelegationTable.Sources`
  (`internal/component/resolve/irr/rir.go`). `FetchDelegationTable` takes the
  sources map, `RenderDelegationTable` writes the table's own Sources and
  refuses a table carrying none.
- `handleRIRRefresh` and `configuredDelegationSources`
  (`internal/component/resolve/cmd/rir.go`) read the config tree at command
  time and hold every source to `ValidateFetchURL` before a byte is fetched.
- `checkRIRDelegationSources`
  (`internal/component/doctor/checks_rir_sources.go`) and
  `codeDoctorRIRSourceRefused` (`internal/core/diagnostic/codes.go`).
- `internal/test/fixture/plugin_fixture_resolve_rir.go` gained a loopback
  registry server, and `test/plugin/resolve-rir-refresh.ci` now asserts the
  SUCCESSFUL refresh, which no functional test could reach before this spec.
- `test/parse/rir-delegation-source-off-box-rejected.ci` proves the commit-time
  refusal over `ze config validate`.

### Bugs Found/Fixed
- `ValidateUpdateCheckURL` compared the loopback host by PREFIX, so
  `http://127.0.0.1.example.com` passed as loopback. It guards a self-update
  manifest's `download-url` (`selfupdate.go`, `resolveDownloadURL`) as well as
  the operator's own leaf. Fixed by parsing the host. Covered by
  `TestALoopbackPlainHTTPURLIsAcceptedAndALookalikeIsNot`
  (`internal/component/config/validators_test.go`) and by the lookalike cases
  added to `TestSelfUpdateDownloadURLValidation`
  (`internal/component/config/system/selfupdate_test.go`), which is the entry
  point a manifest reaches. Journal row:
  `plan/journal/secret-echoed-to-the-client.md`, which had already recorded the
  unparsed URL as the open half of a 2026-08-18 finding.
- The `ze:validate` on the new leaf did NOT run. `ValidateCustomSections` walks
  `validatedSections`, and `system` was in neither that list nor
  `knownUnwalkedValidatorSections`, so the annotation was inert and
  `TestEveryValidatorSectionIsWalkedOrExcused` was RED. Found at the Review
  Gate. Fixed by adding `system` to the walk. Covered by
  `TestLoadConfigRefusesADelegationSourceOffTheBox`,
  `TestLoadConfigAcceptsALoopbackDelegationSource`
  (`internal/component/config/validate_sections_test.go`) and
  `test/parse/rir-delegation-source-off-box-rejected.ci`. Journal row:
  `plan/journal/gate-excludes-part-of-its-population.md`.
- `handleRIRRefresh`'s own fetch-rule loop had no test at its entry point.
  Covered by `TestRefreshRefusesASourceTheFetchRuleRefuses`
  (`internal/component/resolve/cmd/rir_test.go`).

### Documentation Updates
- `docs/architecture/resolve.md`: new "Where the files are read from" section,
  plus the two page defects Required Reading named. The `newResolvers` anchor
  now names `main_system.go`, and the Construction paragraph says the IRR
  resolver takes an empty server and falls back to RADB.
- `docs/architecture/config/system-update.md`, "The URL rule, and the way it was
  got wrong": rewritten. The page still described the exact-spelling localhost
  rule and said nothing about the `127.0.0.1` prefix hole this work closed.
  Found at the Review Gate; the change that made it wrong is in this same diff.
- `docs/guide/configuration.md`: "RIR Delegation Sources".
- `docs/guide/command-reference.md`: the `update resolve rir` entry.
- `docs/guide/cli.md`: the Resolution Commands mirror paragraph.
- `docs/features.md`: the ASN-to-registry row.
- `ai/INDEX.md`: the `resolve` keyword row now carries "ASN to registry, RIR
  delegation, delegation source, mirror the registry files".

### Deviations from Plan
- The Integration Checklist said the doctor check would test "reachability".
  What was built judges the CONFIGURED URL against the fetch rule and fetches
  nothing. A doctor run must not reach five registries, and an unreachable
  mirror is a fact about the network at that second rather than about the
  configuration. Deliberate; `checks_rir_sources.go` carries the same reasoning
  in its file header.
- The doctor check is NOT WIRED at HEAD. `runChecks`
  (`internal/component/doctor/doctor.go`) is the only way to reach it, and that
  file also holds session d781f6b1's `checkBGPPeersWithoutRole(tree)` call,
  whose definition sits in that session's uncommitted `checks_config.go`.
  Committing the file would leave HEAD unable to compile. The check, its three
  tests and its diagnostic code all land here, and the one `runChecks` line is
  re-applied in the working tree so that session's own commit of `doctor.go`
  completes the wiring. Journal row:
  `plan/journal/concurrent-session-corruption.md`, which also records that both
  central lists lost this spec's hunk to a concurrent read-modify-write during
  the closure.
- Three TDD test names changed: `TestANonLoopbackPlainHTTPSourceIsRefused` and
  `TestALoopbackPlainHTTPSourceIsAccepted` became one table-driven
  `TestALoopbackPlainHTTPURLIsAcceptedAndALookalikeIsNot`, and
  `TestAnUnknownRegistryTokenIsRefused` landed in the `irr` package, where
  `delegationSourceURLs` produces the refusal.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| approach | The `ze:validate` binding was taken as proof the rule runs | `ValidateCustomSections` walks a hand-maintained section list, and `system` was outside it, so the annotation was inert while every unit test around it stayed green | `TestEveryValidatorSectionIsWalkedOrExcused` was RED at the Review Gate | `system` added to `validatedSections`, plus two `LoadConfig` tests and a `test/parse` scenario that drive the rule from the operator's entry point |
| escalation | A page named in Required Reading documented the exact function this work rewrote, and the implementation phase did not edit it | `docs/architecture/config/system-update.md` still described the deleted prefix rule | Review Gate documentation drift check | Page rewritten in this diff. A page a spec READS is the page that spec is most likely to make wrong |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| One operator setting: per registry, the URL its delegation file is read from | Done | `ze-system-conf.yang`, `system/rir/delegation-source` | Keyed by the registry token |
| Defaulting to the registry's own published file | Done | `irr/rir.go`, `delegationSourceURLs` | A registry with no entry keeps `publishedDelegation`'s URL |
| A functional test can prove a SUCCESSFUL refresh | Done | `test/plugin/resolve-rir-refresh.ci` | Loopback registry server in `plugin_fixture_resolve_rir.go` |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestNoDelegationSourceLeavesTheSourcesEmpty`, `delegationSourceURLs(nil)` | The five published URLs, in order |
| AC-2 | Done | `TestOneConfiguredSourceLeavesTheOtherFourPublished`, `TestDelegationSourcesAreExtractedFromTheTree` | |
| AC-3 | Done | YANG `enumeration` on the key; `TestAnUnknownRegistryTokenIsRefused` at the fetch | The fetch errors rather than reading four published files |
| AC-4 | Done | `TestLoadConfigRefusesADelegationSourceOffTheBox`, `TestRefreshRefusesASourceTheFetchRuleRefuses`, `test/parse/rir-delegation-source-off-box-rejected.ci` | Both guards, each at its own entry point |
| AC-5 | Done | `TestLoadConfigAcceptsALoopbackDelegationSource`, `TestALoopbackPlainHTTPURLIsAcceptedAndALookalikeIsNot` | `127.0.0.1`, `::1` and `localhost` |
| AC-6 | Done | `TestRefreshNamesAConfiguredSourceItCannotRead`, `resolve-rir-refresh.ci` | |
| AC-7 | Done | `TestTheStoredTableNamesTheURLsItWasBuiltFrom`, `resolve-rir-refresh.ci` | |
| AC-8 | Done | `TestRefreshReadsTheSourcesCommittedAfterStartup` | |
| AC-9 | Done | `TestTheShippedSeedIsWhatTheRendererWrites`, `TestWriteEmitsAFileTheIRRParserReads` | `Write` passes nil sources |
| AC-10 | Done | `test/plugin/resolve-rir-refresh.ci` | |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestDelegationSourcesAreExtractedFromTheTree` | Done | `config/system/system_test.go` | |
| `TestAnUnknownRegistryTokenIsRefused` | Changed | `resolve/irr/rir_test.go` | Moved to the package that produces the refusal |
| `TestANonLoopbackPlainHTTPSourceIsRefused` | Changed | `config/validators_test.go` | Merged into `TestALoopbackPlainHTTPURLIsAcceptedAndALookalikeIsNot` |
| `TestALoopbackPlainHTTPSourceIsAccepted` | Changed | `config/validators_test.go` | Same test |
| `TestOneConfiguredSourceLeavesTheOtherFourPublished` | Done | `resolve/irr/rir_test.go` | |
| `TestTheStoredTableNamesTheURLsItWasBuiltFrom` | Done | `resolve/irr/rir_test.go` | |
| `TestRefreshReadsTheSourcesCommittedAfterStartup` | Done | `resolve/cmd/rir_test.go` | |
| `TestRefreshNamesAConfiguredSourceItCannotRead` | Done | `resolve/cmd/rir_test.go` | |
| `TestTheShippedSeedIsWhatTheRendererWrites` | Done | `resolve/irr/rir_test.go` | Now a parse-render round trip that includes the Source lines |
| `resolve-rir-refresh` | Done | `test/plugin/resolve-rir-refresh.ci` | |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `config/system/yang/ze-system-conf.yang` | Done | |
| `config/system/system.go` | Done | |
| `config/system/update.go` | Done | Delegates to `ValidateFetchURL` |
| `resolve/irr/rir.go` | Done | |
| `resolve/cmd/rir.go` | Done | |
| `le/ianaasn/ianaasn.go` | Done | Passes nil sources |
| `test/fixture/plugin_fixture_resolve_rir.go` | Done | |
| `test/plugin/resolve-rir-refresh.ci` | Done | |
| `docs/architecture/resolve.md` | Done | |
| `docs/guide/configuration.md`, `command-reference.md`, `docs/features.md` | Done | |
| `config/validators.go`, `validators_register.go` | Changed | The rule lives here, not in `system/update.go`, so `doctor` and `resolve/cmd` reach it without importing `system` |
| `config/validate_sections.go` | Changed | Not in the plan: without it the `ze:validate` never runs |
| `doctor/checks_rir_sources.go`, `core/diagnostic/codes.go` | Partial | Built and tested; the `runChecks` wiring is blocked, see Deviations |
| `test/parse/rir-delegation-source-off-box-rejected.ci` | Changed | Not in the plan: added at the Review Gate |

### Audit Summary
- **Total items:** 10 AC, 3 requirements, 10 planned tests, 14 file rows
- **Done:** every AC and every requirement
- **Partial:** the doctor check's `runChecks` wiring, one line, blocked on another session's uncommitted file
- **Skipped:** none
- **Changed:** recorded in Deviations

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| An appliance behind a mirror can refresh the delegation table | functional | `test/plugin/resolve-rir-refresh.ci`: the daemon reads five files from a loopback server the config names, stores the table, and the lookup then answers LACNIC, which is neither the seed's ARIN nor the pre-stored APNIC |
| A functional test can prove a SUCCESSFUL refresh | functional | The same scenario, `OK: a refresh reads the configured sources and stores what they answer`. Its DISCRIMINATOR names the break: ignore the configured sources in `handleRIRRefresh` and the run reaches the public registries, which the test host cannot |
| The stored table says where its ranges came from | functional | The same scenario asserts `# Source: <base>/delegated-` in the stored blob; `TestTheStoredTableNamesTheURLsItWasBuiltFrom` asserts it over the fetch |
| A mirror is authenticated transport, or the box itself | negative test | `TestALoopbackPlainHTTPURLIsAcceptedAndALookalikeIsNot` refuses eight URLs including `http://127.0.0.1.example.com`; `test/parse/rir-delegation-source-off-box-rejected.ci` refuses one at `ze config validate` and accepts the loopback one |
| Today's behavior is unchanged for a daemon nobody configured | data correctness | `TestTheShippedSeedIsWhatTheRendererWrites` (byte-exact over the shipped seed), `TestNoDelegationSourceLeavesTheSourcesEmpty` |

## Deferrals Resolved

| Row (from the deferral shard) | Final Status | Destination or evidence |
|-------------------------------|--------------|-------------------------|
| none | done | The spec metadata names no deferral shard, and `plan/deferrals/rir-delegation-source-override.md` does not exist |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/rir-delegation-source-override-c7a5e5d1-d594-427d-a3f8-1840c99d83d0.md` |
| `./le spec session review check` | clean |
| Rounds | 3 |
| Reviewer lenses used | wiring + guard audit; documentation drift + removed-behavior + style; round 2 over the fixes; round 3 was the lint gate over round 2's own edit |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | BLOCKER | The leaf's `ze:validate "delegation-source-url"` never ran: `ValidateCustomSections` walks `validatedSections` and `system` was outside it, so the rule refused nothing at commit time and `TestEveryValidatorSectionIsWalkedOrExcused` was RED | `internal/component/config/validate_sections.go`, `validatedSections` | `system` added to the walk, with the measurement recorded above the list. Two `LoadConfig` tests and `test/parse/rir-delegation-source-off-box-rejected.ci` drive the rule from the entry point |
| 2 | ISSUE | `handleRIRRefresh`'s fetch-rule loop was tested only through `ValidateFetchURL`, so nothing proved the handler reaches it | `internal/component/resolve/cmd/rir.go`, `handleRIRRefresh` | `TestRefreshRefusesASourceTheFetchRuleRefuses`, walked red against a disabled loop |
| 3 | ISSUE | The security fix's own entry point, the self-update manifest's `download-url`, had no lookalike-host case | `internal/component/config/system/selfupdate.go`, `resolveDownloadURL` | Two lookalike cases added to `TestSelfUpdateDownloadURLValidation`, walked red against the restored prefix rule |
| 4 | ISSUE | `docs/architecture/config/system-update.md` still described the deleted prefix rule, and it is a page this spec's Required Reading named | `docs/architecture/config/system-update.md` | Section rewritten, with two source anchors |
| 5 | ISSUE | goconst: the five registry tokens were spelled three times each | `internal/component/resolve/irr/rir.go` | `registryRIPE`..`registryLACNIC`, used by `rirWhois`, `rirNames` and `publishedDelegation` |
| 6 | ISSUE | ineffassign: the first `stored` value died when the successful refresh was inserted before the failure phase | `internal/test/fixture/plugin_fixture_resolve_rir.go`, `resolveRIRRefresh` | `resolveRIRStoreCopy` returns only an error; the comparison reads the store back at the moment the comparison starts |
| 7 | NOTE | `configuredDelegationSources` spent a paragraph on a nil-versus-empty distinction its success path does not keep, so the comment said more than the code does | `internal/component/resolve/cmd/rir.go` | Comment rewritten to name the ERROR as the distinguishing answer. The empty map STAYS: round 3 caught that replacing it with `nil` reddened `nilnil`, and the rewritten comment now says why the empty map is the right value rather than leaving the next reader to delete it again |
| 8 | NOTE | `sourceLines` was inserted between `TestWriteEmitsAFileTheIRRParserReads` and its doc comment, so the comment documented the helper | `internal/le/ianaasn/ianaasn_test.go` | Helper moved above the comment block |

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/component/doctor/checks_rir_sources.go` | Yes | `ls -la` reports `2.8K Sep  3 16:23` |
| `test/plugin/resolve-rir-refresh.ci` | Yes | tracked, modified in this diff |
| `test/parse/rir-delegation-source-off-box-rejected.ci` | Yes | created at this closure, ran as scenario 262 of the parse suite |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-4 | A plain-HTTP source off the box is refused before any byte is fetched | `./le functional parse` reports `262/322 PASS rir-delegation-source-off-box-rejected`; `TestRefreshRefusesASourceTheFetchRuleRefuses` passes and FAILS with the handler's loop disabled (`status "done", want "error"`) |
| AC-5 | Plain HTTP on the loopback host is accepted | step 2 of the same scenario, `ze config validate -` exit 0 |
| AC-10 | The successful refresh runs end to end | `./le functional plugin` scenario `resolve-rir-refresh`, with the three new OK lines |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| A committed `system` delegation source | `test/parse/rir-delegation-source-off-box-rejected.ci` | Yes: `ze config validate` refuses the off-box URL and accepts the loopback one, which is `ze:validate` running through `ValidateCustomSections` |
| `update resolve rir` against a local fixture server | `test/plugin/resolve-rir-refresh.ci` | Yes: the fixture serves the five files and the scenario reads the stored table back |
| A source that is not HTTPS and not loopback | `internal/component/resolve/cmd/rir_test.go` | Yes, at `handleRIRRefresh` |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | `TestRefreshReadsTheSourcesCommittedAfterStartup`: the source is written into the config file AFTER `pluginserver.NewServer` and the refresh reads it |
| A-2 | confirmed | `TestOneConfiguredSourceLeavesTheOtherFourPublished` |
| A-3 | confirmed | `test/plugin/resolve-rir-refresh.ci` serves five files over `http://127.0.0.1:<port>` |
| A-4 | confirmed | `rirDelegationURLs` no longer exists; `publishedDelegation` is read by `delegationSourceURLs` and `registryTokens` alone, and the renderer takes `table.Sources` |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| `docs/architecture/resolve.md`, "the IRR client takes an empty server and falls back to `whois.radb.net`" | `cmd/ze/hub/main_system.go`, `newResolvers` calls `irr.NewIRR("")`; `internal/component/resolve/irr/client.go`, `NewIRR` substitutes `whois.radb.net` | Yes |
| `docs/architecture/config/system-update.md`, "plain HTTP only from the host itself: `127.0.0.1`, `::1` and `localhost`" | `internal/component/config/validators.go`, `loopbackFetchHosts` and `ValidateFetchURL` | Yes |
| `docs/guide/configuration.md`, the `delegation-source` block | `ze-system-conf.yang` `list delegation-source { key "registry" }`, and `test/parse/rir-delegation-source-off-box-rejected.ci` parses that exact shape | Yes |
| Category "Doctor check for runtime dependencies" | `checks_rir_sources.go` exists and is tested; the `runChecks` wiring is blocked and is named in Deviations | Partial |

## Core Insight

A `ze:validate` annotation that never runs is spelled exactly like one that
does. `CheckAllValidatorsRegistered` asks whether the validator FUNCTION
exists, never whether the walk reaches it, so a new leaf's rule can pass every
unit test in the package it lives in and refuse nothing an operator writes. The
only proof is a test at the operator's entry point: `LoadConfig`, or
`ze config validate` in a `.ci`.
