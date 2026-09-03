# Spec: the RIR delegation seed is embedded data, refreshable into zefs, and answerable offline

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | cli |
| Depends | - |
| Phase | 5/5 |
| Deferral shard | - |
| Handoff | - |
| Updated | 2026-09-03 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

The ASN-to-RIR delegation table reaches no production code. `newSeedRIRTable` and
`loadRIRTable` (`internal/component/resolve/irr/rir.go`) have no non-test caller,
so 11,269 lines of committed Go source and the `./le iana-asn write` generator
that writes them answer nobody.

The delegation-file parser is also declared twice, in `irr` and in
`internal/le/ianaasn`, and the two copies have drifted. The runtime copy refuses
a record whose `start + count - 1` overflows a uint32; the generator, which is
the copy that writes the committed table, does not. The generator refuses a run
that parsed no ASN record; the runtime copy returns an empty slice and a nil
error.

Three surfaces state a refresh path that does not exist: the `rir.go` header, the
generated table's header, and the 2026-08-26 row in
`plan/journal/zero-value-as-valid-answer.md` each say `ze update bgp irr`
refreshes this table into zefs. That command refreshes AS-SET prefix lists in
`filter_irr` and never touches a RIR table.

The goal is one lookup an operator can reach, fed by one table with one parser:
the seed ships as an embedded data file, an operator command refreshes it from
the five RIR delegation files into the managed zefs store, and the lookup prefers
the stored copy when it is newer than the shipped seed.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/zefs-format.md` - the store the refresh writes into
  → Constraint: runtime state lives in the managed store under a key registered
    in `pkg/zefs/keys.go` via `MustRegister`, never as a loose file
  → Decision: the new key is `meta/rir/delegation`, alongside the persisted
    runtime-state keys (`KeyDDoSDetectBaseline`, `KeyNTPLastTime`)
- [ ] `docs/architecture/resolve.md` - the page that owns this code
  → Constraint: the page is SILENT on the seed table, the generator and the
    fallback, while `rir_table.go` and `internal/le/ianaasn/ianaasn.go` both
    declare it as their `// Design:` page. This work makes the page wrong twice
    over, so the page gains a seed-table section in the same change
- [ ] `internal/core/statestore/statestore.go` package doc - persistence rule
  → Constraint: the config system opens `database.zefs` ONCE and never reloads.
    A second transient `zefs.Open` in the hub process makes the config store's
    next flush re-encode from its stale tree and DROP every state key
  → Decision: the refresh runs hub-side and writes through `statestore.Put`.
    `PrefixStore.persist` (`internal/component/resolve/irr/store/store.go`) is
    NOT the precedent to copy: it opens its own handle, which is safe only
    because its callers are separate plugin processes
- [ ] `ai/rules/principles.md` - fail closed
  → Constraint: an unreadable or empty table MUST NOT answer "this ASN is
    unallocated". Absence of data and absence of allocation are two answers
- [ ] `ai/rules/cli.md`, `ai/patterns/cli-command.md` - command shape
  → Constraint: keyword before value, and the response is structured data so
    `| json`, `| yaml` and `| table` each render it

**Key insights:** (minimal context to resume after compaction)
- `irr.NewIRR("")` is built in-core by `newResolvers` in
  `cmd/ze/hub/main_system.go`, so anything the `irr` package writes runs inside
  the hub process.
- The registry token to RIR name and whois host mapping is Go data
  (`rirNames`, `rirWhois`, and the `RIR*` / `Whois*` consts in `rir.go`). The
  data file therefore carries the token only, and each hostname stays declared once.
- `iana-asn` has no check twin, deliberately: its input is the network, so a
  checkout cannot be compared against it without asking five registries. That
  exemption is about the INPUT and survives the change of artifact.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/component/resolve/irr/rir.go` - declares `RIREntry`, the interned
  `RIR*` / `Whois*` consts, `rirTable` with `rirForASN` / `whoisForASN` / `Len`,
  the two constructors `newSeedRIRTable` / `loadRIRTable`, the five delegation
  URLs, `fetchDelegation`, `parseDelegation`, `parseDelegationFields`,
  `collapseRanges` and `internRIREntry`.
  → Constraint: `rirForASN` binary-searches a slice sorted by `Start`, so any
    replacement table must stay sorted and non-overlapping
  → Decision: `newSeedRIRTable`, `loadRIRTable`, `rirForASN`, `whoisForASN` and
    `internRIREntry` each have no non-test caller (`gopls references`)
- [ ] `internal/component/resolve/irr/rir_table.go` - 11,269 lines, generated,
  `var seedRIRTable = []RIREntry{...}` naming the interned consts.
- [ ] `internal/le/ianaasn/ianaasn.go` - the generator: `delegationURLs`,
  `rirNames`, `rirWhois`, `rirConstNames`, `whoisConstNames`, `Entry`, `Fetch`,
  `httpFetch`, `parseDelegation`, `collapse`, `tableSource`, `Write`.
  → Constraint: `Write` refuses an empty ASN parse and writes the whole table in
    one call, both deliberate (its header states the reasoning)
- [ ] `internal/component/resolve/irr/client.go` - `NewIRR` fixes one whois
  server for the client's lifetime; every query key is an AS-SET name.
  → Constraint: per-ASN whois routing is NOT reachable from this shape, and is
    out of scope here
- [ ] `internal/component/resolve/cli/main.go`, `cmd_irr.go` - the host CLI
  `ze resolve` with `dns`, `cymru`, `peeringdb`, `irr` subcommands.
  → Decision: `rir` becomes the fifth subcommand, and it needs no network
- [ ] `internal/component/resolve/cmd/resolve.go`,
  `internal/component/resolve/yang/ze-resolve-api.yang` - daemon RPCs registered
  as `ze-resolve:<name>` through `pluginserver.RegisterRPCs`, one YANG `rpc` each.
  → Constraint: a new daemon command is one YANG `rpc` plus one `RPCRegistration`
- [ ] `internal/core/statestore/statestore.go` - `SetStore`, `Put`, `Get`, `Remove`.
  → Constraint: `Put` is best-effort and answers `(false, nil)` when no store is
    registered, so a refresh must report that as "not stored", never as success
- [ ] `pkg/zefs/keys.go` - every key registered through `MustRegister`.
- [ ] `internal/le/fspersistence/fspersistence.go` - the gate that refuses a raw
  `os` write under `internal/component`.
  → Constraint: the refresh must not call `os.WriteFile`; an allowlist entry that
    suppresses nothing fails the gate

**Behavior to preserve:**
- `ze resolve dns|cymru|peeringdb|irr` output and exit codes.
- `filter_irr` and `firewall/plugins/irr` prefix resolution, including the single
  configured whois server and the `update bgp irr ...` commands.
- The `iana-asn` generator's two fail-closed properties: no empty table is ever
  written, and the artifact is written in one call.

**Behavior to change:**
- The seed stops being Go source and becomes an embedded data file.
- The delegation parser exists once, owned by `irr`.
- A lookup becomes reachable: offline from the host CLI, and from the daemon.
- A refresh becomes real: it writes the managed store under a registered key.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `ze resolve rir <asn>` on the host: an AS number as text, no daemon, no network.
- `show resolve rir <asn>` on the daemon: the same question inside the CLI.
- `update resolve rir` on the daemon: the operator's refresh order.

### Transformation Path
1. Read side: the table accessor answers from the store when a stored copy exists
   and its `Generated:` date is newer than the embedded seed's, otherwise from the
   embedded seed.
2. Either source is the same text format, read by the one parser in `irr`.
3. `rirForASN` binary-searches the parsed ranges and answers the entry, or reports
   that the ASN is in no delegated range.
4. Refresh side: fetch the five delegation files, parse each with the same parser,
   sort, collapse, render the same text format, and `statestore.Put` it.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Host CLI ↔ embedded data | `//go:embed` of the seed file, parsed at first use | Yes: `seed.go` `seedDelegation`, `seedTable`. `TestTheEmbeddedSeedParses`, and `test/plugin/resolve-rir-lookup.ci` over the shipped binary |
| Daemon ↔ zefs | `statestore.Put` / `statestore.Get` under `meta/rir/delegation` | Yes: `handleRIRRefresh` writes, `storedDelegation` reads. `TestUpdateResolveRIRWritesTheStoredCopy` |
| Host CLI ↔ zefs | read-only `zefs.Open` of `database.zefs` when it exists | Yes: `storedDelegationFile`. `TestTheHostReadsTheStoreWhileItIsHeldOpen`, and `test/plugin/resolve-rir-refresh.ci` reads it from the host while the daemon holds it |
| Core ↔ registry servers | HTTPS GET of five delegation files, on operator command only | Yes: `httpDelegationFetch`, reached only from `handleRIRRefresh` and `ianaasn.Write`. Never at startup and never on a timer |
| `internal/le` ↔ `internal/component` | `ianaasn` imports the `irr` parser | Yes: `ianaasn.Write` calls `irr.FetchDelegationTable` and `irr.RenderDelegationTable` and holds no format code of its own. `./le tier check` exit 0 |

### Integration Points
- `pluginserver.RegisterRPCs` in `internal/component/resolve/cmd/` - the two new
  daemon commands register the same way the nine existing resolve RPCs do.
- `internal/component/resolve/cli/` - the host subcommand table gains `rir`.
- `pkg/zefs/keys.go` - one new registered key.
- `internal/le/ianaasn` - keeps the fetch and the file write, drops its parser.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | Every lookup enters at `RegistryForASN`, which reads `delegationTable`. Every write goes through `statestore.Put`; `irr` opens no second zefs handle inside the daemon, and `./le fs-persistence check` exits 0 |
| No unintended coupling (components stay isolated) | Yes | `irr` imports `statestore`, `paths` and `pkg/zefs` and nothing new. `ianaasn` (`internal/le`) imports `irr` in the permitted direction, and `./le tier check` exits 0 |
| No duplicated functionality (extends existing, does not recreate) | Yes | This is what the spec exists for. The registry parse, the collapse, the render and the fetch each have ONE implementation in `irr`, and `./le iana-asn write` and `update resolve rir` both reach them through `irr.FetchDelegationTable`. `internal/le/ianaasn` holds no parser, no token map, no collapse, no HTTP client and no fetch type |
| Zero-copy preserved where applicable (refs, not copies) | N-A | No wire path. `rirForASN` binary-searches a slice and returns a pointer into it, `RenderDelegationTable` builds through `textbuf.Buffer`, and the fetch streams through `io.LimitReader` rather than reading five files whole |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | Yes | `register_rir.go` calls `pluginserver.RegisterRPCs`, and `handlerFor` in `rir_test.go` resolves both handlers through `pluginserver.AllBuiltinRPCs` rather than by name, so an unregistered handler fails the test. The zefs key registers through `MustRegister`. No central switch was edited |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | A hub-side write through `statestore.Put` is the only safe way for `irr` to persist, because `irr` is built in-core | `internal/core/statestore/statestore.go` package doc; `newResolvers` in `cmd/ze/hub/main_system.go` | A second `zefs.Open` in the hub drops every state key on the next config flush | `TestConfigWriteDoesNotDropStateKey` shape, plus a functional refresh test that reads the key back after a config commit | confirmed: `handleRIRRefresh` writes through `statestore.Put` and opens no handle of its own, and `storedDelegation` reads through `statestore.Get` whenever a store is registered, so the hub holds exactly one. `TestUpdateResolveRIRWritesTheStoredCopy` round-trips the key through that shared handle. The drop-on-flush failure itself was NOT reproduced: the evidence for "only safe way" is the `statestore` package doc read at `SetStore` and `Put`, and the functional half stays Phase 5's |
| A-2 | A host process may open `database.zefs` read-only while the daemon holds its own handle | `PrefixStore.openExisting` opens transiently from plugin processes today | `ze resolve rir` cannot see a refreshed copy and must answer from the embed alone | Functional test: refresh on the daemon, then read from the host CLI | confirmed: `TestTheHostReadsTheStoreWhileItIsHeldOpen` reads the delegation key while a second handle stays open on the same file, and `TestTheHostLookupAnswersFromANewerStoredCopy` drives the whole host path through `paths.DefaultConfigDir`. `zefs.Open` memory-maps and takes no lock, and `BlobStore.Lock` is a separate call the read path never makes |
| A-3 | The registry token, not the RIR name, is enough in the data file, because names and whois hosts are Go consts | `rirNames`, `rirWhois`, `RIR*` and `Whois*` in `internal/component/resolve/irr/rir.go` | The file must carry four fields per line and the hostname is declared twice | Round-trip test: generator output parsed back equals the ranges it was built from | confirmed: the three-field file parses back to the 11,256 ranges `seedRIRTable` held, equal entry for entry, and `TestARegistryTokenBecomesTheInternedConstants` shows the token alone produces both constants |
| A-4 | Dropping `rir_table.go` costs only a `./le repository generate` run for the `ai/DOCS-TO-CODE.md` row | Research report on `internal/le/docstocode`; NOT read by me at the producing function | A gate goes red after the file disappears | Run `./le repository generate` and the full native verification in the implementing phase | confirmed: after the deletion `./le repository check`, `./le docs-to-code check` and `./le verify lint run` over all 18 flavors each exit 0, and the two indexes are untracked artifacts a native action rewrites |
| A-5 | `iana-asn`'s exemption from the generated-files check survives the artifact change, because the exemption is about the network input | `internal/le/ianaasn/actions.go` header | A check twin is owed and the generator needs a `check` verb | Read `generationChecks` in `internal/le/repository/generate.go` before Phase 3 | confirmed: `generationChecks` names 18 rows and `iana-asn` is not among them, and `./le repository generated-check` after the port names no `iana-asn` row |
| A-6 | The five delegation files stay line-oriented, seven fields separated by a vertical bar | `parseDelegation` in both copies; the RIR extended delegation format | The refresh parses nothing and, by AC-6, writes nothing | The existing parse tests plus a fixture per registry | confirmed against the format Ze parses, NOT against today's published files. `parseDelegationFields` reads a line as seven vertical-bar fields and refuses one with fewer; `eachRegistryOnce` (`internal/le/ianaasn/ianaasn_test.go`) drives a fixture in that shape for each of the five registries; the `TestParseRegistryDelegation*` and `TestCollapseRanges*` cases cover the rest. What no test reads is the LIVE files: `./le iana-asn write` was never run against them in this work, so the seed keeps its 2026-08-16 vintage. If a registry changes the format, AC-6 is what a refresh then answers: it parses no record, stores nothing, and says so |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A corrupt or truncated embedded seed makes every lookup answer "unallocated" | A parse test over the embedded file fails; the lookup answers an error rather than a verdict | The parser errors on a malformed line, the accessor propagates it, and a test asserts a floor on the range count |
| R-2 | A refresh that reaches three registries of five commits a table that reads as "unallocated" for two registries' ASNs | The refresh reports fewer records than the seed | All-or-nothing: any fetch or parse failure writes nothing, as `ianaasn.Write` already does |
| R-3 | A stored copy written by a newer binary in a format this one cannot read | Parse error on the stored blob | Fall back to the embedded seed and report the stored copy as unreadable, never answer from a half-parsed table |
| R-4 | The host CLI reads `database.zefs` while the hub writes it, and sees a torn file | Parse error on the stored blob | Same fallback as R-3; the read is read-only and never blocks the daemon |
| R-5 | An upgrade ships a seed newer than the stored copy, and the stale stored copy keeps answering | A lookup disagrees with a fresh registry answer | Precedence compares `Generated:` dates and the newer wins, so an upgrade takes over automatically |
| R-6 | `statestore.Put` answers `(false, nil)` in filesystem-fallback mode and a refresh reports success having stored nothing | The refresh's own answer says nothing was stored | The handler reports "not stored" as an error, per `ai/rules/principles.md` |
| R-7 | Deleting `rir_table.go` breaks `plan/spec-bcp194-7-transit-asn.md`, which cites it as the shipped-data convention | That spec's `→ Decision:` annotation names a Go literal that no longer exists | The data file keeps `Source:` and `Generated:` header lines, and this spec's closure notes the new shape for that spec to follow |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Nothing user-visible today: the table answers nobody. After this work, a wrong lookup misreports which registry holds an ASN. No routing, filtering or wire behavior depends on it |
| How is it reverted? | Single commit revert. The zefs key is additive and an unread key costs nothing |
| Who else touches this path? | `plan/spec-bcp194-7-transit-asn.md` (design) borrows the shipped-data convention. `filter_irr` and `firewall/plugins/irr` share the `irr` package but not the RIR table |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `ze resolve rir 15169` (host, no daemon, no network) | → | `cmdRIR` in `internal/component/resolve/cli/cmd_rir.go` | `TestResolveRIRAnswersFromTheEmbeddedSeed` |
| `show resolve rir 15169` (daemon) | → | `handleRIRASN` in `internal/component/resolve/cmd/rir.go` | `TestShowResolveRIRReachesTheTable` |
| `update resolve rir` (daemon) | → | `handleRIRRefresh` in `internal/component/resolve/cmd/rir.go` | `TestUpdateResolveRIRWritesTheStoredCopy` |
| `./le iana-asn write` | → | `Write` in `internal/le/ianaasn/ianaasn.go` | `TestWriteEmitsAFileTheIRRParserReads` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `ze resolve rir 15169` with no network and no `database.zefs` | Answers the holding registry and its whois host from the embedded seed, exit 0 |
| AC-2 | `ze resolve rir` for an AS number in no delegated range | Answers that the ASN is in no delegation range, exit 1, and the message distinguishes this from an unreadable table |
| AC-3 | The embedded seed is replaced by a file with a malformed range line | Every lookup reports the seed as unreadable and names the offending line; no lookup answers "unallocated" |
| AC-4 | `update resolve rir` on a daemon with all five registries reachable | Fetches, parses, stores under `meta/rir/delegation`, and reports the record count and the date it stored |
| AC-5 | `update resolve rir` when one of the five fetches fails | Stores nothing, reports the failing URL, and the previously stored copy is unchanged |
| AC-6 | `update resolve rir` when the five responses carry no ASN record | Stores nothing and reports that the run parsed no record |
| AC-7 | A stored copy whose `Generated:` date is newer than the embedded seed's | The lookup answers from the stored copy, on the daemon and from the host CLI |
| AC-8 | A stored copy whose `Generated:` date is older than or equal to the embedded seed's | The lookup answers from the embedded seed, and the stored copy is left in place |
| AC-9 | A stored copy that cannot be parsed | The lookup answers from the embedded seed and reports the stored copy as unreadable |
| AC-10 | `update resolve rir` when no store is registered (filesystem-fallback mode) | Reports that nothing was stored, exit non-zero; success is never reported for an unstored refresh |
| AC-11 | `./le iana-asn write` over fixture responses | Writes the data file with `Generated:` and `Source:` header lines, and the `irr` parser reads it back to the ranges it was built from |
| AC-12 | A delegation record whose `start + count - 1` exceeds a uint32 | Refused by the one parser, on both the generator path and the refresh path |
| AC-13 | The tree after this work | `internal/le/ianaasn` contains no delegation-format parser, no registry-token map and no range-collapse of its own |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Asks which registry holds an AS number, on a laptop with no network | `ze resolve rir` → embedded seed → binary search → answer | `TestResolveRIRAnswersFromTheEmbeddedSeed` |
| 2 | Refreshes the table on an appliance a year after the release, then asks again | `update resolve rir` → five fetches → parse → `statestore.Put` → `show resolve rir` → stored copy | `test/plugin/resolve-rir-refresh.ci` |
| 3 | Asks on the host after the daemon refreshed | `ze resolve rir` → read-only `database.zefs` → stored copy newer than seed → answer | `TestResolveRIRPrefersTheStoredCopy` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestTheEmbeddedSeedParses` | `internal/component/resolve/irr/rir_test.go` | the shipped file parses and holds at least a floor number of ranges | green |
| `TestAMalformedLineIsAnErrorNotAnEmptyTable` | `internal/component/resolve/irr/rir_test.go` | AC-3, fail closed | green |
| `TestAnASNInNoRangeIsDistinctFromAnUnreadableTable` | `internal/component/resolve/irr/rir_test.go` | AC-2, the two answers differ | green |
| `TestTheStoredCopyWinsWhenItIsNewer` | `internal/component/resolve/irr/stored_test.go` | AC-7 | green, and red when the date comparison is inverted |
| `TestTheSeedWinsWhenTheStoredCopyIsNotNewer` | `internal/component/resolve/irr/stored_test.go` | AC-8 | green, and red when the date comparison is inverted |
| `TestAnUnreadableStoredCopyFallsBackToTheSeed` | `internal/component/resolve/irr/stored_test.go` | AC-9 | green |
| `TestTheHostLookupAnswersFromANewerStoredCopy` | `internal/component/resolve/irr/stored_test.go` | AC-7 over the whole host path, user story 3 | green |
| `TestTheHostReadsTheStoreWhileItIsHeldOpen` | `internal/component/resolve/irr/stored_test.go` | A-2, the host read is safe while the daemon holds the file | green |
| `TestAnOversizedRangeIsRefused` | `internal/component/resolve/irr/rir_test.go` | AC-12 | green, and red when the guard is disabled |
| `TestRangesAreSortedAndDisjointAfterCollapse` | `internal/component/resolve/irr/rir_test.go` | the binary search precondition | green |
| `TestWriteEmitsAFileTheIRRParserReads` | `internal/le/ianaasn/ianaasn_test.go` | AC-11 round trip | green |
| `TestAnOversizedRecordReachesNoTable` | `internal/le/ianaasn/ianaasn_test.go` | AC-12 on the generator path, which had no overflow guard of its own | green, and red against the generator's own parser before the port |
| `TestARunThatParsedNothingWritesNothing` | `internal/le/ianaasn/ianaasn_test.go` | AC-6, existing test kept green through the port | green |
| `TestRefreshStoresNothingWhenAFetchFails` | `internal/component/resolve/cmd/rir_test.go` | AC-5 | green |
| `TestRefreshStoresNothingWhenNoRecordIsParsed` | `internal/component/resolve/cmd/rir_test.go` | AC-6 on the refresh path | green |
| `TestUpdateResolveRIRWritesTheStoredCopy` | `internal/component/resolve/cmd/rir_test.go` | AC-4 | green |
| `TestRefreshReportsAnUnstoredWrite` | `internal/component/resolve/cmd/rir_test.go` | AC-10, R-6 | green |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| ASN argument | 0-4294967295 | 4294967295 | negative / non-numeric | 4294967296 |
| Delegation start | 0-4294967295 | 4294967295 | N/A | 4294967296 |
| Delegation start plus count minus one | 0-4294967295 | 4294967295 | count of zero | overflow refused (AC-12) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `resolve-rir-lookup` | `test/plugin/resolve-rir-lookup.ci` | operator asks which registry holds an ASN and pipes the answer through `\| json` | green. AC-1 and AC-2 over both read paths: `ze resolve rir 15169` on the host, and `show resolve rir 15169 \| json` inside the daemon. RED observed when `cmdRIR` returns 0 for an unallocated ASN |
| `resolve-rir-refresh` | `test/plugin/resolve-rir-refresh.ci` | operator refreshes from a fixture registry, then reads the stored answer back | green, with a NARROWER scenario than the row states, and the file says why. A functional test MUST NOT reach the five public registries and Ze publishes no way to redirect the fetch, so the successful refresh stays a unit test. What runs here is AC-5 and AC-7: a stored copy dated after the seed answers on the daemon and on the host, a refresh with the registries out of reach fails naming the file, and the stored blob is byte-identical after. RED observed when `preferStoredDelegation` inverts its date comparison |

### Interop Tests (Scope: protocol)
Not applicable: no wire-visible behavior changes, no protocol peer.

## Files to Modify
- `internal/component/resolve/irr/rir.go` - one parser, the embedded seed, the
  stored-copy precedence, the fail-closed lookup, and the false header comment
- `internal/le/ianaasn/ianaasn.go` - import the `irr` parser, emit the data file,
  drop the second parser, the token maps and the collapse
- `internal/le/ianaasn/report.go` - the report names a data file, not Go source
- `internal/component/resolve/cli/main.go` - `rir` joins the subcommand table
- `internal/component/resolve/cmd/resolve.go` - register the two RPCs
- `internal/component/resolve/yang/ze-resolve-api.yang` - two `rpc` definitions
- `pkg/zefs/keys.go` - register `meta/rir/delegation`
- `docs/architecture/resolve.md` - the seed table section the page never had
- `docs/features.md`, `docs/guide/command-reference.md`,
  `docs/architecture/api/commands.md` - the new commands
- `plan/journal/zero-value-as-valid-answer.md` - correct the 2026-08-26 row,
  which states the table is the resolver's fallback

## Files to Create
- `internal/component/resolve/irr/rir-delegation.txt` - the generated seed data
- `internal/component/resolve/irr/seed.go` - the `//go:embed` declaration and the
  lazily parsed accessor
- `internal/component/resolve/cli/cmd_rir.go` - the host subcommand
- `internal/component/resolve/cmd/rir.go` - the two daemon handlers
- `test/plugin/resolve-rir-lookup.ci`, `test/plugin/resolve-rir-refresh.ci`

## Files to Delete
- `internal/component/resolve/irr/rir_table.go` - 11,269 generated lines,
  replaced by the embedded data file

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `internal/component/resolve/yang/ze-resolve-api.yang`: `rir-asn`, `rir-refresh` |
| YANG validation constraints | Yes | `rir-asn` takes a mandatory uint32 `asn` leaf, matching `cymru-asn-name` |
| YANG custom validators | N-A | No config leaf is added; both RPCs take native types |
| CLI commands/flags | Yes | `internal/component/resolve/cli/cmd_rir.go` (host), `internal/component/resolve/cmd/rir.go` (daemon) |
| CLI grammar (keyword before value) | Yes | `show resolve rir <asn>` and `update resolve rir` keep keyword before value |
| Editor autocomplete | N-A | The one argument is a free uint32; no enumerable value set |
| Functional test for new RPC/API | Yes | `test/plugin/resolve-rir-lookup.ci`, `test/plugin/resolve-rir-refresh.ci` |
| Pipe completeness | Yes | Both handlers answer structured data so `\| json`, `\| yaml` and `\| table` render it |
| Env var registration | N-A | No YANG leaf under `environment/` |
| Doctor check for runtime dependencies | N-A | No new file path, socket, port, module or binary. The zefs file already exists and the network fetch happens on operator command only |
| Prometheus counters/metrics | N-A | No continuous state to observe; the stored copy's date is reported by the command itself |
| BGP family surface (new SAFI / capability / attribute) | N-A | No BGP surface |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | DONE: `docs/features.md` gains the "ASN to Regional Internet Registry" row, naming both lookups, the refresh, the precedence rule and the all-or-nothing guard |
| 2 | Config syntax changed? | No | No YANG config node is added. The two YANG additions are `ze:command` containers, not config nodes |
| 3 | CLI command added/changed? | Yes | DONE: `docs/guide/command-reference.md` gains `### show resolve rir` and `### update resolve rir`. The `ze resolve rir 15169` host example was already there from Phase 1 |
| 4 | API/RPC added/changed? | Yes | DONE: `docs/architecture/api/commands.md` gains the `ze-show:resolve-rir` and `ze-update:resolve-rir` rows with their payloads and their two error shapes |
| 5 | Plugin added/changed? | No | `resolve` is a component. `resolve-cmd` gains two `ze:command` containers and no plugin was added, removed or re-scoped |
| 6 | Has a user guide page? | Yes | DONE: `docs/guide/cli.md` gains a "Resolution Commands" section with both commands and the precedence rule |
| 7 | Wire format changed? | No | No protocol code is touched |
| 8 | Plugin SDK/protocol changed? | No | The RPCs use the existing `pluginserver` registration |
| 9 | RFC behavior implemented, changed, or newly proven? | No | The delegation file format is a registry convention, not an RFC requirement Ze implements |
| 10 | Test infrastructure changed? | No | Existing `.ci` harness and the existing compiled-fixture API. Two scenarios were registered; no runner, format or helper changed |
| 11 | Affects daemon comparison? | No | No competitor-comparable behavior changes |
| 12 | Internal architecture changed? | Yes | DONE: `docs/architecture/resolve.md` gains a "RIR Delegation Table" section: the shipped seed and its generator, the stored copy and its all-or-nothing refresh, the precedence rule, the two read paths as a table, and the three answers a lookup has. It also gains "The two daemon commands" under CLI |
| 13 | Route metadata keys added/changed? | No | No route metadata |
| 14 | Prometheus counters added/changed? | No | None added |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes, and the two files this row names carry nothing to change | `docs/guide/status.md` has no per-command listing: its tables are subsystem-level, and no subsystem changed status. `docs/features/plugins.md` lists plugins by category and names no command of any plugin, so `resolve-cmd` gaining two containers adds no row. The registered command surface is published by `docs/architecture/api/commands.md` (row 4) and `docs/guide/command-reference.md` (row 3), and both are DONE |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes, and nothing is owed | DERIVED, run 2026-09-03: `./le spec citation anchors spec plan/spec-rir-seed-embed-and-zefs-refresh.md` names ONE unnamed document, `docs/architecture/diagnostics/debug-filtering.md`, mentioned by `pkg/zefs/keys.go`. Its anchor there is `<!-- source: pkg/zefs/keys.go -- KeyDebugProfile -->`, about the debug profile key. Adding `KeyRIRDelegation` makes no claim on that page wrong, so no edit is owed. `docs/architecture/resolve.md` is the `// Design:` page of every file this spec touched in `irr` and `ianaasn`, and it is named in row 12 |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | DONE: the `ze resolve` example block in `docs/guide/command-reference.md` carries `ze resolve rir 15169`, and the `ze resolve` table in `docs/architecture/resolve.md` carries the `rir` row. `./le doc check verify` reports "Source anchors: checked 2307 code paths, all references valid" |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- the three entry points exist and are reachable
   - Tests: `TestResolveRIRAnswersFromTheEmbeddedSeed`, `TestShowResolveRIRReachesTheTable`, `TestUpdateResolveRIRWritesTheStoredCopy`
   - Files: `internal/component/resolve/cli/main.go`, `cmd_rir.go`, `internal/component/resolve/cmd/rir.go`, `ze-resolve-api.yang`, `pkg/zefs/keys.go`
   - Verify: the commands dispatch to stubs and the wiring tests fail on the stub answer
2. **Phase: one format, one parser** -- the data file shape and the fail-closed parse
   - Tests: `TestTheEmbeddedSeedParses`, `TestAMalformedLineIsAnErrorNotAnEmptyTable`, `TestAnOversizedRangeIsRefused`, `TestRangesAreSortedAndDisjointAfterCollapse`
   - Files: `internal/component/resolve/irr/rir.go`, `seed.go`, `rir-delegation.txt`
   - Verify: the parser errors where the old one skipped, and the lookup separates "no range" from "unreadable"
3. **Phase: the generator emits the data file** -- and stops parsing
   - Tests: `TestWriteEmitsAFileTheIRRParserReads`, `TestARunThatParsedNothingWritesNothing`
   - Files: `internal/le/ianaasn/ianaasn.go`, `report.go`; delete `rir_table.go`
   - Verify: `./le iana-asn write` reproduces the committed data file from fixtures, and `ianaasn` holds no parser
4. **Phase: the stored copy** -- refresh, precedence, and the two read paths
   - Tests: `TestTheStoredCopyWinsWhenItIsNewer`, `TestTheSeedWinsWhenTheStoredCopyIsNotNewer`, `TestAnUnreadableStoredCopyFallsBackToTheSeed`, `TestRefreshStoresNothingWhenAFetchFails`, `TestRefreshReportsAnUnstoredWrite`
   - Files: `internal/component/resolve/cmd/rir.go`, `internal/component/resolve/irr/rir.go`
   - Verify: a refresh against fixture registries changes the answer, and every failure mode writes nothing
5. **Phase: functional tests and the pages** -- the operator path and the docs
   - Tests: `test/plugin/resolve-rir-lookup.ci`, `test/plugin/resolve-rir-refresh.ci`
   - Files: the documentation rows above, and the journal row correction
   - Verify: `./le repository generate` after the deletion, then the full native verification

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at a named file and symbol |
| Feature completeness | All three user stories run end to end, host and daemon |
| Correctness | "no delegated range" and "table unreadable" are two distinct answers on every path, including the host CLI's exit code |
| Correctness | The refresh is all-or-nothing across the five registries |
| Naming | The data file's header fields (`Generated:`, `Source:`) match what the parser reads and what the precedence compares |
| Data flow | `irr` never writes through a second `zefs.Open`; every write goes through `statestore.Put` |
| Rule: `ai/rules/principles.md` | No path answers a zero where it means "I could not read" |
| Rule: `ai/rules/documentation.md` | `docs/architecture/resolve.md` is edited in the same work, not at closure |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| The 11,269-line Go table is gone | `ls internal/component/resolve/irr/rir_table.go` fails |
| The parser is declared once | `TestWriteEmitsAFileTheIRRParserReads` proves the shared path, and no delegation parse remains in `internal/le/ianaasn` |
| The lookup is reachable | `ze resolve rir 15169` answers with the network down |
| The refresh writes the store | `resolve-rir-refresh.ci` reads the key back |
| The pages agree with the code | `./le doc check links` and `./le spec citation anchors` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | The ASN argument is bounded to uint32 before lookup, as `updateASN` already does for `update bgp irr asn` |
| Untrusted input | The five delegation files are network input: the parser bounds the read size, refuses malformed records, and never allocates on a length the file states |
| Resource exhaustion | The fetch keeps a size cap per file and a timeout; the stored blob has a maximum accepted size |
| Fail open | A refresh that stored nothing must never report success |

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

- Phase 1 correction to Files to Modify: the two daemon commands do NOT belong in
  `internal/component/resolve/yang/ze-resolve-api.yang`. That module carries one
  `rpc` per `ze-resolve:` method only. Every `ze-show:` and `ze-clear:` method of
  this component is a `container` carrying a `ze:command` extension in
  `internal/plugins/resolve-cmd/yang/ze-resolve-cmd.yang`, and `Validate` in
  `internal/le/docvalid/contract.go` cross-checks handlers against the `-cmd`
  modules only. An `rpc` in the api module would have satisfied no gate.
- The generator and the daemon parsed the same registry format independently, and
  each copy held a guard the other lacked. The lesson is not "copy the guard": it
  is that a format has one owner, and the tool that writes the artifact imports it.
- Phase 5 finished that unification, and the thing that had blocked it was one
  published number. `WriteReport.Records` is the pre-collapse record count, which
  says how much of the world a generation run read, and the shared fetch answered
  only the collapsed table. Putting the count INSIDE `DelegationTable` was the
  wrong shape, because a table parsed back from a file has no such count and its
  zero would read as "no records" rather than "never fetched". So
  `FetchDelegationTable` answers the table AND the count, `ianaasn.Write` is four
  statements over it, and the daemon's refresh discards the count with a comment
  saying why an operator does not need it. Three symbols then lost their last
  cross-package caller and were unexported or deleted, which is how the check
  reported that the duplication was really gone.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Line-oriented text data file | Fixed-width binary blob searched in place | Text diffs readably when the seed is refreshed and matches the `feature-gates.txt` shape. Binary would hold about 100 KB against roughly 165 KB and skip the decode, at the cost of a reviewable diff and a second format to maintain |
| The data file carries the registry token only | Carrying the RIR name and whois host per line | Names and hosts are already Go consts, so the file repeating them would declare the same fact twice and grow by roughly a third |
| One format for the embedded seed and the stored copy | JSON for the stored copy | One parser reads both, which is the property this work exists to restore |
| Refresh runs hub-side through `statestore.Put` | A host-side write to `database.zefs` | A second transient handle in the hub process drops every state key on the next config flush (`statestore` package doc) |
| Precedence by `Generated:` date | An explicit format version, or always preferring the stored copy | An upgrade shipping a newer seed must win over a stale stored copy, which a date settles and a version does not |

## Known Limitations
- Per-RIR whois routing for IRR queries is not implemented. `NewIRR` still fixes
  one server, and route objects are commonly registered outside the holder's own
  registry, so routing by holder would shrink the prefix sets `filter_irr`
  depends on. That is a separate spec with its own interop proof.
- The refresh is manual. No timer, no automatic fetch at startup.

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
- [ ] AC-1..AC-13 all demonstrated
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
- The seed stopped being Go source. `internal/component/resolve/irr/rir-delegation.txt`
  carries 11,256 ranges as `<start> <end> <token>` under a comment header, `seed.go`
  embeds it and parses it once through `sync.OnceValues`, and
  `internal/component/resolve/irr/rir_table.go` (11,269 lines) is deleted.
- The delegation format has ONE owner. `parseRegistryDelegation`, `collapseRanges`,
  `RenderDelegationTable` and `FetchDelegationTable` are the `irr` package's, and
  `internal/le/ianaasn` holds no parser, no token map, no collapse, no HTTP client
  and no fetch type: `Write` is four statements over `irr.FetchDelegationTable`.
- The lookup became reachable. `RegistryForASN` is the one entry point, with three
  outcomes. `cmdRIR` answers `ze resolve rir <asn>` on the host with no network and
  no daemon, and `handleRIRASN` answers `show resolve rir <asn>` on the daemon.
- The refresh became real. `handleRIRRefresh` fetches the five registry files,
  renders the same format, and writes it through `statestore.Put` under
  `zefs.KeyRIRDelegation`. It is all or nothing, and it never reports success for a
  run that stored nothing.
- Precedence lives in `stored.go`. `preferStoredDelegation` prefers the stored copy
  while its `Generated:` date is strictly after the seed's, and `storedDelegation`
  reads the config system's own handle inside the daemon and opens `database.zefs`
  read-only in every other process.

### Bugs Found/Fixed
- The two delegation parsers had diverged, and each held a guard the other lacked:
  the runtime copy refused a `start + count - 1` above a uint32 and accepted an
  empty parse, and the generator did the reverse. Both guards now sit on the one
  parser, covered by `TestAnOversizedRangeIsRefused` and `TestAnOversizedRecordReachesNoTable`
  (the second was RED against the generator's own parser before the port) and by
  `TestARunThatParsedNothingWritesNothing`.
- Three surfaces said `ze update bgp irr` refreshes this table. It refreshes AS-SET
  prefix lists in `filter_irr` and never touched a RIR table. The `rir.go` header,
  the generated table's header and the 2026-08-26 row in
  `plan/journal/zero-value-as-valid-answer.md` are each corrected.
- `rirTable` carried a `sync.RWMutex` that guarded nothing, and a doc comment
  ("Thread-safe after loading", "If the table is replaced ... old pointers remain
  valid but stale") describing a load-then-replace design this work removed.
  `parseRIRTable` is the one constructor and no path writes either field
  afterwards. Found at the closure review; the mutex and both stale comments are
  gone, the invariant is stated positively, and
  `go test -race ./internal/component/resolve/...` is green.

### Documentation Updates
- `docs/architecture/resolve.md`: a "RIR Delegation Table" section (the shipped
  seed, the stored copy, which source answers, the two read paths, the three
  answers) and "The two daemon commands" under CLI. Anchors on `seed.go`
  (`seedDelegation`, `seedTable`), `rir.go` (`delegationTableHeader`,
  `RenderDelegationTable`, `FetchDelegationTable`, `RegistryForASN`,
  `ErrASNUnallocated`), `stored.go` (`preferStoredDelegation`, `delegationTable`,
  `storedDelegation`, `storedDelegationFile`), `cmd/rir.go` (`handleRIRASN`,
  `handleRIRRefresh`), `cmd/register_rir.go` and `ianaasn.go` (`Write`).
- `docs/features.md`: the "ASN to Regional Internet Registry" row, with three anchors.
- `docs/guide/command-reference.md`: `### show resolve rir` and `### update resolve rir`,
  plus the `ze resolve rir 15169` example.
- `docs/architecture/api/commands.md`: the `ze-show:resolve-rir` and
  `ze-update:resolve-rir` rows with their payloads and their two error shapes.
- `docs/guide/cli.md`: a "Resolution Commands" section with the precedence rule.
- `ai/INDEX.md` and `ai/PACKAGE-MAP.md`: the `iana-asn` description, which named the
  compiled seed table.
- `./le doc check verify`: source anchors green over 2307 code paths. The drift stage
  names `show resolve rir` and `update resolve rir` in `../gh-pages` only. See
  Pre-Commit Verification for why that publish is not part of this closure.

### Deviations from Plan
- The two daemon commands are declared in
  `internal/plugins/resolve-cmd/yang/ze-resolve-cmd.yang` as `container` nodes
  carrying a `ze:command` extension, NOT as `rpc` nodes in
  `internal/component/resolve/yang/ze-resolve-api.yang` as five sections of the
  spec said. `Validate` in `internal/le/docvalid/contract.go` cross-checks handlers
  against the `-cmd` modules only, so an `rpc` in the api module would have
  satisfied no gate. Recorded in Design Insights at Phase 1.
- The handlers and their `init()` registration are in two files
  (`cmd/rir.go`, `cmd/register_rir.go`), unlike `resolve.go`, because the native
  `pretool-writeedit` check refuses `Register` inside `init()` in a file whose
  basename does not start with `register`.
- AC-4 has no functional test, by decision rather than omission. See Goal Validation.

## Mistake Log

<!-- One table, one place. Ship the `none` row and either replace it or leave it
     deliberately: three separate empty tables produced three separate 67-82%
     untouched rates, because an empty table asks nothing.
     Kind: assumption (a broken A-N) | approach (a route abandoned) | escalation
     (a mistake frequent enough to deserve a rule). -->
| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| assumption | The spec said both daemon commands are declared as `rpc` nodes in `internal/component/resolve/yang/ze-resolve-api.yang`, in Current Behavior, Files to Modify, the Integration Checklist and the Phase 1 file list | That module carries one `rpc` per `ze-resolve:` method only. Every `ze-show:` and `ze-clear:` method of this component is a `container` with a `ze:command` extension in `internal/plugins/resolve-cmd/yang/ze-resolve-cmd.yang` | Phase 1 read the module before writing to it, and `Validate` in `internal/le/docvalid/contract.go` cross-checks handlers against the `-cmd` modules only | Followed the source. Recorded in Design Insights and in Deviations |
| approach | Phase 4 built `irr.FetchDelegationTable`, unified `ianaasn.Write` onto it, then REVERTED the unification and left the two fetch recipes standing | `Write` publishes `WriteReport.Records`, the pre-collapse record count, which the shared recipe did not answer. Phase 4 read that as "the recipes cannot merge" | Phase 5 re-opened it and found the block was the SHAPE of the count, not the count. A table read back from a file has no such count, so the field could not live in `DelegationTable`, but the FUNCTION can answer two facts | `FetchDelegationTable` answers `(DelegationTable, int, error)`. `Write` is four statements over it; the daemon discards the count with a comment saying why an operator does not need it. Three symbols then lost their last cross-package caller and were unexported or deleted, which is how `./le repository check` reported the duplication was really gone |
| approach | The closure review found `rirTable.mu`, a `sync.RWMutex` taken by `rirForASN` and `Len` | Nothing writes `entries` or `generated` after `parseRIRTable` hands the pointer out, so the mutex guarded nothing and its doc comment described a load-then-replace design this work had removed | Reading every writer of the two fields, not the lock sites | Removed the mutex, rewrote both stale comments, replaced an unreachable third `switch` case with `default`. `go test -race` over the resolve packages is green |

## Implementation Audit

<!-- BLOCKING before the learned summary. See ai/rules/completion.md.
     Status: Done (with file:line) | Partial | Skipped | Changed.
     Partial and Skipped both require explicit user approval. -->

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| The seed ships as an embedded data file | Done | `internal/component/resolve/irr/rir-delegation.txt`, embedded by `seedDelegation` in `seed.go` | `rir_table.go` is deleted |
| One lookup an operator can reach | Done | `RegistryForASN` in `internal/component/resolve/irr/rir.go`, reached by `cmdRIR`, `handleRIRASN` | `test/plugin/resolve-rir-lookup.ci` runs both over the shipped binary |
| One table with one parser | Done | `parseRegistryDelegation`, `parseDelegationTable`, `collapseRanges`, `RenderDelegationTable` in `rir.go` | `internal/le/ianaasn` declares none of them |
| An operator command refreshes it into the managed zefs store | Done | `handleRIRRefresh` in `internal/component/resolve/cmd/rir.go` | Writes through `statestore.Put` under `zefs.KeyRIRDelegation` |
| The lookup prefers the stored copy when it is newer than the shipped seed | Done | `preferStoredDelegation` in `internal/component/resolve/irr/stored.go` | Strictly after, so an upgrade with fresher data takes over |
| The three false refresh claims are corrected | Done | the `rir.go` header, `delegationTableHeader`, the 2026-08-26 row in `plan/journal/zero-value-as-valid-answer.md` | None of the three now names `ze update bgp irr` |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestResolveRIRAnswersFromTheEmbeddedSeed`, and `resolve-rir-lookup.ci` over the shipped binary in a fresh config dir | Answers ARIN and whois.arin.net, exit 0 |
| AC-2 | Done | `TestAnASNInNoRangeIsDistinctFromAnUnreadableTable`, `TestResolveRIRSeparatesNoRangeFromNoTable`, `TestShowResolveRIRSeparatesNoRangeFromNoTable`, and `resolve-rir-lookup.ci` on both paths | RED observed when `cmdRIR` returns `exitOK` for an unallocated ASN |
| AC-3 | Done | `TestAMalformedLineIsAnErrorNotAnEmptyTable`, 12 cases, plus `registryForASN` over an unreadable source | `parseDelegationTable` names the line number and the text |
| AC-4 | Done, UNIT ONLY | `TestUpdateResolveRIRWritesTheStoredCopy` over the registered handler, `TestWriteFetchesEveryRegistryAndWritesTheTable` over the generator | No functional test. See Goal Validation for why, and for what would change it |
| AC-5 | Done | `TestRefreshStoresNothingWhenAFetchFails`, and `resolve-rir-refresh.ci` end to end | The error names the delegation file; the stored blob is byte-identical after |
| AC-6 | Done | `TestRefreshStoresNothingWhenNoRecordIsParsed`, `TestARunThatParsedNothingWritesNothing` | One guard, in `FetchDelegationTable`, on both paths |
| AC-7 | Done | `TestTheStoredCopyWinsWhenItIsNewer`, `TestTheHostLookupAnswersFromANewerStoredCopy`, and `resolve-rir-refresh.ci` on the daemon and on the host | RED observed when the date comparison is inverted |
| AC-8 | Done | `TestTheSeedWinsWhenTheStoredCopyIsNotNewer`, older and same-date | The stored copy is left in place |
| AC-9 | Done | `TestAnUnreadableStoredCopyFallsBackToTheSeed` | `preferStoredDelegation` answers the seed AND a note; `delegationTable` logs the note |
| AC-10 | Done | `TestRefreshReportsAnUnstoredWrite` | Non-vacuous since Phase 4: the fetch succeeds from fixtures and only `statestore.Put` answering false produces the error |
| AC-11 | Done | `TestWriteEmitsAFileTheIRRParserReads` | RED before the port: "the irr parser refuses what the generator wrote" |
| AC-12 | Done, both halves | `TestAnOversizedRangeIsRefused`, `TestAnOversizedRecordReachesNoTable` | The second was RED against the generator's own parser before the port |
| AC-13 | Done | `grep -nE "ripencc\|arin\|apnic\|afrinic\|lacnic\|allocated\|assigned\|SplitSeq\|bufio\|strconv" internal/le/ianaasn/*.go` (non-test) answers nothing | And no HTTP client and no fetch type, after the Phase 5 unification |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| Every unit row of the TDD plan | Done | as listed in that table | `go test -count=1 -race ./internal/component/resolve/... ./internal/le/ianaasn/...` exit 0, 10 packages ok |
| `resolve-rir-lookup` | Done | `test/plugin/resolve-rir-lookup.ci` | PASS inside `./le functional gating` |
| `resolve-rir-refresh` | Done | `test/plugin/resolve-rir-refresh.ci` | PASS inside `./le functional gating`, with the narrower scenario its header declares |
| Interop | N-A | - | No wire-visible behavior, no protocol peer |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/component/resolve/irr/rir.go` | Done | One parser, the entry point, the fetch recipe, the render |
| `internal/le/ianaasn/ianaasn.go` | Done | 282 lines to 66; imports the `irr` parser and holds no format code |
| `internal/le/ianaasn/report.go`, `actions.go`, `register.go` | Done | Wording, and `runWrite` takes `irr.DelegationFetch` |
| `internal/component/resolve/cli/main.go` | Done | `rir` in `subcommands`, the `Run` switch, the usage services and examples |
| `internal/component/resolve/cmd/resolve.go` | Changed | The registration is in `register_rir.go`. See Deviations |
| `internal/component/resolve/yang/ze-resolve-api.yang` | Changed | The declarations are in `internal/plugins/resolve-cmd/yang/ze-resolve-cmd.yang`. See Deviations |
| `pkg/zefs/keys.go` | Done | `KeyRIRDelegation`, pattern `meta/rir/delegation` |
| `docs/architecture/resolve.md`, `docs/features.md`, `docs/guide/command-reference.md`, `docs/architecture/api/commands.md` | Done | Plus `docs/guide/cli.md`, `ai/INDEX.md`, `ai/PACKAGE-MAP.md`, which the plan did not name |
| `plan/journal/zero-value-as-valid-answer.md` | Done | The 2026-08-26 row is corrected in place; the defect it records is untouched |
| Files to Create: `rir-delegation.txt`, `seed.go`, `cli/cmd_rir.go`, `cmd/rir.go`, the two `.ci` | Done | Plus `irr/stored.go`, `cmd/register_rir.go`, `internal/test/fixture/plugin_fixture_resolve_rir.go`, `register_resolve_rir.go`, and three test files |
| Files to Delete: `internal/component/resolve/irr/rir_table.go` | Done | `ls` reports no such file |

### Audit Summary
- **Total items:** 35 (6 requirements, 13 AC, 4 test groups, 12 file rows)
- **Done:** 33
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 2 (the two YANG/registration rows, both recorded in Deviations)

## Goal Validation (BLOCKING)

<!-- Maps each goal from the Task section to proof it was achieved. "Tests pass"
     is not evidence for a goal; a named test with its output is.
     See ai/rules/interop-and-goal-validation.md for the required evidence per
     goal type, and for the vacuity traps: a test that would still pass with the
     behavior reverted proves nothing. -->
| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| One lookup an operator can REACH. The table answered nobody: `newSeedRIRTable` and `loadRIRTable` had no non-test caller, so 11,269 committed lines of Go were dead | functional, over the shipped binary | `test/plugin/resolve-rir-lookup.ci`: `ze-test fixture plugin/resolve-rir-lookup` exits 0 and prints four OK lines. The host run is `ze resolve rir 15169` with a fresh temporary config dir, no daemon and no `database.zefs`, and it must print `ARIN` and `whois.arin.net` on stdout. The daemon run is `show resolve rir 15169 \| json` over SSH, unmarshalled into a struct and compared field by field, so prose passing a substring check would fail. A unit test cannot show this goal: the state this spec began in would pass one, because the lookup logic was correct and unreachable |
| The lookup is OFFLINE. An operator with the cable unplugged gets the answer | functional | The same file. `cmdRIR` takes no context and `main.go` gives it no timeout, because the seed is embedded; `resolve-rir-lookup.ci` runs the host binary with no daemon and no network fixture at all, and the assertion is on stdout rather than on an exit code alone |
| "Nobody holds this AS number" and "I could not read the table" stay two answers | functional, both paths, with the RED observed | The same file: `ze resolve rir 4294967294` must exit non-zero AND the stderr must contain `no delegated range`, and `show resolve rir 4294967294` must fail with the same words. Discrimination walk, observed and restored: making `cmdRIR` return `exitOK` for `ErrASNUnallocated` turns `resolve-rir-lookup` RED with "ze resolve rir 4294967294 exited 0", while `resolve-rir-refresh` stays green |
| One table with ONE parser. Two copies had drifted and each held a guard the other lacked | structural, plus the guard proven on both paths | `internal/le/ianaasn` holds no delegation parser, no registry-token map, no collapse, no URL list, no HTTP client and no fetch type (AC-13 grep). `TestAnOversizedRecordReachesNoTable` was RED against the generator's own parser before the port and is green after, which proves the overflow guard now covers the generator path it never covered. `TestARunThatParsedNothingWritesNothing` proves the empty-parse guard still covers it, from `FetchDelegationTable` rather than from a second copy. `./le repository check` names none of my symbols after the unification; before it, `ParseDelegationTable` was reported as an exported symbol with no cross-package non-test caller, which is the check reporting that the duplication was really gone |
| A refresh writes the managed store, and the stored copy then answers | functional, daemon and host, with the RED observed | `test/plugin/resolve-rir-refresh.ci`: a stored copy dated tomorrow hands AS15169 to APNIC where the shipped seed hands it to ARIN, and both `show resolve rir 15169 \| json` on the daemon and `ze resolve rir 15169` on the host must answer APNIC, the host reading `database.zefs` while the daemon holds the file open. Inverting `.After(` to `.Before(` in `preferStoredDelegation` turns it RED with "the answer names ARIN at whois.arin.net, want APNIC at whois.apnic.net", and leaves `resolve-rir-lookup` green. The disagreement between the two sources is deliberate: a lookup reading the wrong source names the wrong registry rather than returning nothing |
| The refresh is ALL OR NOTHING: a partial table must never replace a whole one | functional, byte-for-byte | The same file: with the registries out of reach, `update resolve rir` must FAIL, its output must name the delegation file it could not read, the previously stored copy must still answer, and after the daemon stops the blob under `meta/rir/delegation` must be byte-identical to what was written before the run (`resolveRIRStoredCopyUnchanged`). That last assertion also covers A-1: a config flush during the daemon's life did not drop the state key |
| The successful refresh stores what it fetched (AC-4) | unit, deliberately, and the limit is stated | NOT proven end to end, and this is a decision. A `.ci` MUST NOT fetch the five public delegation files, they are tens of megabytes each, and Ze publishes NO surface that redirects the fetch: `httpDelegationFetch` reads `rirDelegationURLs`, which is a package var no config leaf and no Ze environment variable reaches. What IS proven: `TestUpdateResolveRIRWritesTheStoredCopy` resolves `ze-update:resolve-rir` through `pluginserver.AllBuiltinRPCs` (so an unregistered handler fails it), runs it against a real `zefs.Create` store registered through `statestore.SetStore`, and reads the key back and compares the payload; `TestWriteFetchesEveryRegistryAndWritesTheTable` asserts each of the five tokens was asked for and compares the written bytes against `irr.RenderDelegationTable`. Every SEGMENT of AC-4's path carries a test, and only their successful composition does not: the SSH transport for this command is driven by `resolve-rir-refresh.ci`'s failing run, and the store round trip by the unit test. What would close it is a Ze-published redirect for the five URLs, which an appliance behind a mirror would want for its own sake. Inventing one to serve a test would be an option nobody asked for (`ai/rules/simplicity.md`), so it is named here as separable work rather than added here |
| Interop | N-A | No wire-visible behavior changes and there is no protocol peer. The five delegation files are a registry publishing convention, not a protocol Ze speaks |

## Deferrals Resolved

<!-- Closure must leave no dangling row: deferral_unassigned_problems in
     internal/le/commit/prepare.go WARNS (it does not block) on a live row with no
     destination -- act on the warning here, because nothing else will.
     The spec's own shard is git rm'd at closure ONLY when every row in it is
     terminal; a shard still holding a live row outlives its source spec and
     deferral_shard_removal_problems blocks its removal
     (ai/rules/planning.md). Account for every row here.
     If resolving a row empties a FOREIGN shard (its last live row becomes
     terminal), that shard is now residue and this closure removes it too. -->
| Row (from the deferral shard) | Final Status | Destination or evidence |
|-------------------------------|--------------|-------------------------|
| None. The spec metadata declares `Deferral shard \| -`, and `ls plan/deferrals/rir-seed-embed-and-zefs-refresh.md` reports no such file | n/a | No shard was opened and none is removed. Nothing was deferred: every AC is Done in the Implementation Audit, with AC-4's proof limited to unit tests by the decision recorded in Goal Validation rather than deferred to a later spec |

### Risks carried forward
| ID | Outcome |
|----|---------|
| R-1 | Held. `parseDelegationTable` errors on a malformed line and names it, `seedTable` propagates that error, and `TestTheEmbeddedSeedParses` asserts a floor on the range count |
| R-2 | Held. `FetchDelegationTable` stops on the first registry that fails and never partially stores. `resolve-rir-refresh.ci` asserts the stored blob is byte-identical after a failed run |
| R-3, R-4 | Held. `preferStoredDelegation` answers the seed and a note for a stored copy it cannot parse, and `storedDelegationFile` answers "nothing stored" and logs for a store file that exists and cannot be opened or read |
| R-5 | Held. The comparison is strictly-after, so a shipped seed newer than the stored copy wins |
| R-6 | Held. `handleRIRRefresh` branches on `statestore.Put` answering `(false, nil)` and reports it as a failure to store |
| R-7 | REALIZED, and recorded rather than repaired. `plan/spec-bcp194-7-transit-asn.md` cites `internal/component/resolve/irr/rir_table.go` in its Required Reading as "the one shipped-data convention", and its `→ Decision:` annotation describes a generated GO LITERAL carrying `Source:` and `Generated:` header lines. This closure deletes that file. The convention SURVIVES in the new artifact: `rir-delegation.txt` keeps both header lines and the one deliberate `./le iana-asn write` verb with no check twin, so only the word "Go literal" is now wrong. That spec is UNTRACKED in this checkout, so it belongs to a live session, and editing another session's uncommitted file is not this closure's to do (`ai/rules/principles.md`). R-7's own mitigation is to note the new shape for that spec to follow, and this row plus the "RIR Delegation Table" section of `docs/architecture/resolve.md` is that note. Reported to the main thread to carry to the owning session |

## Review Gate

<!-- BLOCKING (ai/rules/planning.md). The review is INDEPENDENT: reviewer
     subagents or a fresh session over the actual diff, never your own inline
     reasoning about code you just wrote.

     The machine-checked artifact is the deliverable, not this table:
     internal/le/spec/session/review.go record --spec <spec> --rounds <N> ... then check.
     --rounds is the pass count and is required; more than five needs
     --rounds-reason naming the PRODUCT defect a later round found, AND
     --owner-authorised carrying Thomas's word, because more than five passes
     is his decision (owner ruling 2026-08-17). At the cap you stop and ask him;
     you never set that flag on your own initiative. A false statement in this
     record is a NOTE, never a reason for another round (ai/rules/planning.md).
     commit_helper.py runs `review_gate.py check` on the closure commit and
     refuses without a fresh, hash-pinned, CLEAN artifact. Record the artifact
     first; this table exists only to carry what was FOUND and FIXED forward
     into the learned summary. -->

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/rir-seed-embed-and-zefs-refresh-c7a5e5d1-d594-427d-a3f8-1840c99d83d0.md`, 21 files, verdict clean |
| `./le spec session review check` | OK, hashes match |
| Rounds | 2. Round 1 over the complete uncommitted diff found one ISSUE. Round 2 over the fix found nothing, and the fix is new code, which is why it earned its own pass |
| Reviewer lenses used | logic and wiring at the producing functions; security against the spec's checklist and the generic list; `docs/contributing/ze-go-style.md` over every changed Go file, including the panic question; documentation claims re-read against source; shared-checkout attribution of every red the gates reported |

### Findings fixed
<!-- Only BLOCKER and ISSUE. NOTEs do not block: record them and proceed.
     Every fix is new code that needs a fresh pass, so re-run until clean. -->
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | ISSUE | `rirTable` carried a `sync.RWMutex` that guarded nothing, and its doc comment claimed it did. Read every writer of the two fields: `parseRIRTable` is the one production constructor, it fills `entries` and `generated` before it hands the pointer out, and no path writes either field afterwards. The old design had a load-then-replace phase; this work removed it and left the lock and the prose behind. Two comments were stale with it: "Thread-safe after loading", and `rirForASN`'s "If the table is replaced (e.g., by a runtime update), old pointers remain valid but stale". `ai/rules/stale-comments.md` is blocking, and the machinery has no user (`ai/rules/simplicity.md`) | `rirTable`, `rirForASN`, `Len` in `internal/component/resolve/irr/rir.go` | The mutex field and its four lock calls are removed, the `sync` import with them. `rirTable`'s doc now states the invariant positively and names why it holds: one constructor, no writer, and a refresh answering a NEW table rather than mutating one. `rirForASN`'s doc names the sorted-and-disjoint precondition and its two guarantors instead of a replacement that cannot happen. `go test -count=1 -race ./internal/component/resolve/... ./internal/le/ianaasn/...` exits 0 over all ten packages |

### Notes (recorded, not blocking)
| # | Finding | Reading taken |
|---|---------|---------------|
| N-1 | `whoisForASN` and `Len` (`internal/component/resolve/irr/rir.go`) have no non-test caller. `whoisForASN` is superseded by this work, because `RegistryForASN` answers a whole `RIREntry` carrying `Whois` | Left in place. Both predate this spec, the spec's own Current Behavior already records `whoisForASN` as callerless, and neither is exported so `./le repository check` does not flag them. `Len` is read by this spec's own `TestTheEmbeddedSeedParses` floor assertion. Deleting production code and its test at closure, for a symbol no AC names, is scope this closure does not own |
| N-2 | `rirUsage` offers `ze resolve rir 4294967295` as an example, and AS4294967295 is in no delegated range, so the example exits 1 | Left. It demonstrates the second of the two answers the command keeps apart, which is the behavior AC-2 exists for. Changing it is cosmetic |
| N-3 | `RenderDelegationTable` writes its `Source:` lines from the `rirDelegationURLs` package var, not from the URLs the caller's `DelegationFetch` actually read. A test that swaps that var changes the rendered header with it | Correct in production, where `httpDelegationFetch` reads exactly those URLs. It is a test-visible coupling and not a product defect |
| N-4 | The revision statements in `internal/plugins/resolve-cmd/yang/ze-resolve-cmd.yang` are not in date order: 2026-04-04, 2026-08-29, 2026-06-03, then the new 2026-09-02 | Pre-existing disorder, and the new statement is appended last as the module already does. Reordering the earlier two is another author's edit |

## Pre-Commit Verification

<!-- BLOCKING. Do NOT trust the audit above: re-verify independently and paste
     the evidence. For each row run a command (ls, grep, go test -run) now.

     EVERY sub-table needs at least one data row: pre_commit_verification_gaps
     in internal/le/commit/prepare.go checks them one by one and names the empty
     ones. A row in Files Exist is not evidence for AC Verified.
     Not acceptable: "already checked", "should work", a pointer to the audit. -->

### Files Exist (ls)
<!-- Every file in "Files to Create", and every .ci named in Wiring Test and
     Functional Tests. Paste the ls output. -->
| File | Exists | Evidence |
|------|--------|----------|
| `internal/component/resolve/irr/rir-delegation.txt` | Yes | `ls -la` 2026-09-03: 214K |
| `internal/component/resolve/irr/seed.go` | Yes | `ls -la`: 1.2K |
| `internal/component/resolve/irr/stored.go` | Yes | `ls -la`: 5.6K |
| `internal/component/resolve/cli/cmd_rir.go` | Yes | `ls -la`: 2.0K |
| `internal/component/resolve/cmd/rir.go` | Yes | `ls -la`: 4.4K |
| `test/plugin/resolve-rir-lookup.ci` | Yes | `ls -la`: 2.3K |
| `test/plugin/resolve-rir-refresh.ci` | Yes | `ls -la`: 3.5K |
| `internal/component/resolve/irr/rir_table.go` | No, deleted as planned | `ls`: "cannot access ... No such file or directory" |

### AC Verified (grep/test)
<!-- Every AC-N, re-checked. Acceptable: test name + pass output, grep showing
     the call, ls showing the file. -->
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1, AC-2 | The offline lookup answers, and the two error answers stay apart | `go test -count=1 -race ./internal/component/resolve/...` 2026-09-03: `ok internal/component/resolve/cli 2.359s`, `ok internal/component/resolve/cmd 1.800s`, `ok internal/component/resolve/irr 3.231s`. `test/plugin/resolve-rir-lookup.ci` requires four OK lines, two per AC |
| AC-3, AC-12 | The parse fails closed on a malformed line and on an oversized range | Same run, `irr` package green. `parseDelegationTable` returns `DelegationTable{}` and never a partial table on any error path; `parseRegistryDelegation` guards `start+count-1 > math.MaxUint32` and returns `nil` on `scanner.Err()` |
| AC-4, AC-10 | The refresh stores what it fetched, and never reports success for an unstored run | Same run, `ok internal/component/resolve/cmd`. `handleRIRRefresh` branches on both `err` and `!stored` from `statestore.Put` |
| AC-5, AC-6 | Every failure mode stores nothing | Same run. `FetchDelegationTable` returns on the first failing registry and refuses a zero-record run before it sorts |
| AC-7, AC-8, AC-9 | Precedence, and the unreadable stored copy | Same run, `irr` green over `stored_test.go`'s eight tests |
| AC-11 | The generator writes a file the `irr` parser reads | `ok internal/le/ianaasn 2.793s` in the same run |
| AC-13 | `internal/le/ianaasn` holds no format code | `grep -nE "ripencc\|arin\|apnic\|afrinic\|lacnic\|allocated\|assigned\|SplitSeq\|bufio\|strconv\|net/http"` over the four non-test files: exit 1, nothing found |

### Wiring Verified (end-to-end)
<!-- Every Wiring Test row: does the .ci exist AND exercise the claimed path?
     Read the file; do not infer it from its name. -->
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `ze resolve rir 15169` (host, no daemon, no network) | `test/plugin/resolve-rir-lookup.ci` | Yes. Read the file and its driver: `resolveRIRHostAnswers` runs the `ze` binary with `envConfigDir` set to a fresh `os.MkdirTemp` and requires `ARIN` and `whois.arin.net` on stdout with exit 0 |
| `show resolve rir 15169` (daemon) | `test/plugin/resolve-rir-lookup.ci` | Yes. `resolveRIRDaemonAnswers` runs `show resolve rir 15169 \| json` over SSH against a live daemon and `json.Unmarshal`s the answer, so the ASN, the registry and the whois host are each asserted as fields rather than as substrings |
| `update resolve rir` (daemon) | `test/plugin/resolve-rir-refresh.ci` (failure path) plus `TestUpdateResolveRIRWritesTheStoredCopy` (success path) | Partly. Read the file: the `.ci` drives the command over SSH with the registries out of reach and requires a non-zero result naming `delegated-`, then re-reads the stored blob byte for byte. The SUCCESSFUL store is the unit test, which resolves the handler through `pluginserver.AllBuiltinRPCs` and reads the key back from a real `zefs` store. The reason no `.ci` covers the success is in Goal Validation |
| `./le iana-asn write` | none (a developer verb, not an operator path) | Yes, at the unit tier. `TestWriteEmitsAFileTheIRRParserReads` asserts each of the five tokens was fetched and compares the written bytes against `irr.RenderDelegationTable`; `tokenFor` FAILS the test for a URL no fixture registry answers, so a run asking for a sixth file is seen |

### Assumptions Resolved
<!-- Every A-N. `unvalidated` is not a valid final status. A broken assumption
     needs a Mistake Log row and a Deviations entry. -->
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed, with its limit stated | `handleRIRRefresh` writes through `statestore.Put` and opens no handle of its own; `storedDelegation` reads through `statestore.Get` whenever a store is registered, so the hub holds exactly one. `TestUpdateResolveRIRWritesTheStoredCopy` round-trips the key through that shared handle, and `resolve-rir-refresh.ci` asserts the blob is byte-identical after a whole daemon lifetime, which is the functional half the earlier phases owed. The drop-on-flush FAILURE itself was never reproduced: the evidence for "the only safe way" remains the `statestore` package doc read at `SetStore` and `Put` |
| A-2 | confirmed | `TestTheHostReadsTheStoreWhileItIsHeldOpen` reads the delegation key while a second handle stays open on the same file. `resolve-rir-refresh.ci` proves it over the real path: the host `ze resolve rir` answers from `database.zefs` while the daemon holds it |
| A-3 | confirmed | The three-field file parses back to the 11,256 ranges `seedRIRTable` held, equal entry for entry (transient round-trip test, Phase 2, kept at `<scratch>/roundtrip_check_test.go.txt`), and `TestARegistryTokenBecomesTheInternedConstants` shows the token alone produces both constants |
| A-4 | confirmed | After the deletion `./le repository check` names no file of this spec (15 issues 2026-09-03, all in `filterapi`, `filter_aspath_length`, `cli/completer.go`, `cli/validator.go`, `config/yang`, `le/rfc`), and `./le docs-to-code check` is up to date. `ai/DOCS-TO-CODE.md` and `ai/CODE-TO-DOCS.md` are untracked artifacts a native action rewrites |
| A-5 | confirmed | `generationChecks` in `internal/le/repository/generate.go` names 18 rows and `iana-asn` is not among them; `./le repository generated-check` after the port names no `iana-asn` row |
| A-6 | confirmed against the format Ze parses, NOT against today's published files | `parseDelegationFields` reads a line as seven vertical-bar fields and refuses one with fewer; `eachRegistryOnce` drives a fixture in that shape for each of the five registries. `./le iana-asn write` was NEVER run against the live registries in this work, so the seed keeps its 2026-08-16 vintage and no data line changed in any phase. If a registry changes the format, AC-6 is what a refresh then answers: it parses no record, stores nothing, and says so |

### Documentation Verified
<!-- Every Yes in the Documentation checklist: verify the edited claim against
     source. Every No: paste the grep that proves no update was needed. -->
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| Row 1, `docs/features.md`: both lookups, the refresh, the precedence rule and the all-or-nothing guard | Read against `RegistryForASN`, `preferStoredDelegation`, `handleRIRRefresh`, `FetchDelegationTable` | Yes. Three `<!-- source: -->` anchors, each naming a symbol that exists |
| Row 3, `docs/guide/command-reference.md`: `show resolve rir` returns `asn`, `registry`, `whois`, `range-start`, `range-end` | Read against the `plugin.Map` literal in `handleRIRASN` | Yes, all five keys and in that order |
| Row 4, `docs/architecture/api/commands.md`: the refresh answers `key`, `ranges`, `generated` | Read against the `plugin.Map` literal in `handleRIRRefresh` | Yes, all three keys, and `generated` is `time.DateOnly` |
| Row 6, `docs/guide/cli.md`: the precedence rule | Read against `preferStoredDelegation`, which compares strictly-after | Yes |
| Row 12, `docs/architecture/resolve.md`: the two read paths table | Read against `storedDelegation`, which branches on `statestore.Store() != nil`, and `storedDelegationFile`, which `os.Stat`s then `zefs.Open`s read-only | Yes |
| Row 2, 5, 7, 8, 9, 11, 13, 14 answered No | No YANG config node is added (both YANG additions are `ze:command` containers); `resolve` is a component and no plugin was added or re-scoped; no protocol code, no SDK change, no RFC behavior, no daemon comparison, no route metadata, no counter | Yes |
| Row 10 answered No | `grep` over `test/plugin/resolve-rir-*.ci`: each is one `ze-test fixture` line in the existing harness. No runner, format or helper changed; two scenarios were registered through the existing `Register` | Yes |
| Row 15, `docs/guide/status.md` and `docs/features/plugins.md` carry nothing to change | `docs/guide/status.md` tables are subsystem-level and no subsystem changed status; `docs/features/plugins.md` lists plugins by category and names no command of any plugin | Yes |
| Row 16, source anchors on changed files | `./le spec citation anchors` names ONE unnamed document, `docs/architecture/diagnostics/debug-filtering.md`, anchored on `KeyDebugProfile`. Adding `KeyRIRDelegation` makes no claim on that page wrong | Yes, nothing owed |
| `./le doc check verify` | 2026-09-03: "Source anchors: checked 2307 code paths, all references valid". The DRIFT stage names `show resolve rir` and `update resolve rir`, and every such row is under `../gh-pages` | Yes, with the publish deliberately out of scope (below) |

### The publish trees are NOT regenerated, and that is a decision
`../gh-pages` and `../wiki` are stale for the two new commands: `./le doc check
verify` reports about 16 drift rows naming them, all under `../gh-pages`, out of
3600 rows that predate this work (Phase 1 measured the same 3600 before any
command existed). `./le site build` and `./le wiki-catalog update` are what clear
them, and this closure does NOT run them, for two reasons.

They cannot be part of commit A. `validateAddPath` (`internal/le/commit/input.go`)
refuses a path outside the repository, so a sibling worktree's files are a
separate repository and a separate commit by construction.

And running the build now would publish three other sessions' unlanded work.
`../gh-pages` holds 8 uncommitted files and was last published 2026-08-30, so it
is close to clean. The generator writes the whole tree from `docs/`, and `docs/`
currently carries another session's `filter_path_asn` command rows in
`docs/guide/command-reference.md` and `docs/architecture/api/commands.md`, and a
third session's CLI `?`-key work in `docs/guide/cli.md`. A build now would fill a
near-clean shared worktree with documentation for code that is not in `main`
(`ai/rules/principles.md`: another session's work is not mine to carry). The
publish is owed to whoever lands the last of those `docs/` edits, or to a
deliberate publish pass, and it is reported rather than performed.

## Core Insight

One published number kept two parsers alive for four phases.

The duplication was obvious from the first read: two functions called
`parseDelegation`, over one registry format, in two packages, with no call
between them, and each holding a guard the other lacked. The obvious repair,
copying the missing guard into the other copy, is the repair that leaves the
class in place. So the work deleted one parser and imported the other.

It could not, at first. `ianaasn.Write` publishes `WriteReport.Records`, the
count of records the five files yielded BEFORE the collapse, which tells a
developer whether a generation run read the whole world or a fraction of it. The
shared recipe answered the collapsed table and not that count. Phase 4 built the
shared function, unified onto it, and reverted, recording that the count was the
thing to solve first.

The block was not the count. It was where the count was asked to live. Putting it
inside `DelegationTable` fails because a table parsed back from a FILE has no
such count, and its zero would then read as "no records" rather than "never
fetched", which is the zero-as-an-answer defect this repository has paid for most
often. The count is a fact about a RUN, and the table is a fact about the world.
A function may answer two facts; a struct that carries a field only one of its
two producers can fill may not. `FetchDelegationTable` returns
`(DelegationTable, int, error)`, `Write` is four statements over it, and the
daemon's refresh discards the count with a comment saying why an operator does
not want it.

The confirmation came from a gate rather than from a reading: after the
unification, three symbols lost their last cross-package caller, and
`./le repository check` reported them. Exporting a symbol "for the generator" is
a claim that two packages need it. When the check says only a test still calls
it, the duplication is really gone.
