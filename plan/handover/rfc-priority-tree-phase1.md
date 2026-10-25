# Handoff: rfc-priority-tree, Phase 1 onwards

**Spec:** `plan/release/delivery/spec-rfc-priority-tree.md` (Status in-progress, claimed by the 2026-10-25 session; the next session claims it again with `./le spec claim spec plan/release/delivery/spec-rfc-priority-tree.md`)
**Branch:** main
**Goal:** Order RFC coverage work by use-case profile and by a dependency tree derived from the RFC texts, report a mandatory and an optional coverage figure per RFC, per profile and per technology aggregate, name the next gap with `./le rfc next`, and publish one website page per profile and aggregate.

The owner rule behind it is in `ai/META-DECISIONS.md`, Goal and scope ("RFC work is ordered by use-case profile and by the RFC dependency tree"). Every design decision is in the spec's "Agreed shape", "Research gate decisions" and "Key Design Decisions" tables; do not reopen them.

## Status

### Done (committed)

| Commit | What |
|--------|------|
| `cebd86cd48`, `00b9273a2a` | Owner rule in `ai/META-DECISIONS.md`: release profiles need 100% mandatory coverage (MUST, MUST NOT, SHALL, SHALL NOT, REQUIRED), proven by a recorded break; optional (SHOULD, SHOULD NOT, RECOMMENDED, NOT RECOMMENDED, MAY, OPTIONAL) is a second figure, improved after |
| `d47d2705ba` | The spec, design approved |
| `1facce9b44` | `{updated-by: <ID>; why}` row marker: parser (`parseUpdatedBy`, `summary.go`), `Meta.Updates` from the Meta `Updates` cell (`updatesFrom`, `meta.go`), `checkUpdatedBy` (`check_core.go`), selftest `summary/updated-by`, tests in `internal/le/rfc/updatedby_test.go`, docs in `docs/contributing/rfc-conformance-gates.md` ("The updated-by marker") and `ai/skills/ze-rfc.md`. `Requirement` kept at 152 bytes (`RollupState` is `uint8`, `Derived` beside `Ticked`). `rfc/short/rfc2966.md` Updates cell corrected to None |
| `fb3027cc78` | The 13 rfc4271 rows RFC 7606 rewrites carry `{updated-by}` (list in the spec, section "First `{updated-by}` set") |

Evidence for `1facce9b44`: `go test -race ./internal/le/rfc` ok 287.844s; scoped lint leaves 7 findings, all pre-existing in `discriminate.go`, `discriminate_action.go`, `discriminate_observe.go`, `discriminate_progress_test.go`.

### In progress
- Nothing uncommitted from this work.

### Remaining (spec phases)

| Phase | Work |
|-------|------|
| 1 (rest) | `rfc/profiles/` parser and files (8 use-case profiles, about 28 aggregates); `Support | <section> <rank>` becomes `Support | <aggregate>` in all 204 summaries; delete `statusSections` (`internal/le/rfc/render_ledger.go`) and the rank (`placeRows` rank refusal goes); 36 summaries with `Support | -` gain an `Aggregate` cell; coverage function (`profilecoverage.go`) with mandatory and optional figures and "not walked"; update the `bgp-base 10` fixtures (`render_test.go`, `site/rfcdetail_test.go`, `rfc/fixture.go`, `selftest_core.go`, `health/collect_rfc_test.go`, `check_test.go`) |
| 2 | `internal/le/rfc/graph/`: normative-reference and header parser, `rfc/graph.tsv` via `./le rfc index-update`, cycle components, levels; header-vs-Meta link check and the 24 corrections; switch `checkUpdatedBy` from the Meta `Updates` cell to the graph |
| 3 | `./le rfc tree`, `./le rfc next` in `internal/le/rfc/actions.go`; status page ordered by aggregate, level, RFC number |
| 4 | Ledger JSON gains graph, profiles, figures; `rfc-profiles` producer (`internal/le/site/rfcprofiles.go`); `./le site build` and `./le site check` on `../gh-pages` |
| 5 | `RFC gaps` spec Meta field (`specstatus.go`, `parseMetadata`), roadmap Profiles column, template row |
| Closure | `/ze-review` to 0 BLOCKER/0 ISSUE, `plan/templates/TEMPLATE-CLOSURE.md`, `./le verify changed`, closing commits A and B |

### Not done, and who owns it

| Item | Owner |
|------|-------|
| `{updated-by}` markers for the other updater pairs (summaries declare about 127 Updates links) | This spec's goal needs them for the release profiles; do them per release profile after Phase 3, when `./le rfc tree profile <name>` shows which pairs the profile reaches. Each pair needs the row-by-row reading against `rfc/full/` that rfc4271/rfc7606 got |
| LAN DNS resolver, DHCPv6 server, NAT and NTP RFCs for fttx-edge | Feature specs not yet written; the owner decides whether they run |
| IKE virtual-IP pool has no non-test caller | `spec-ike-virtual-ip-assignment` (release fixes) |
| `./le verify changed` debt on this checkout | Owed before any push; the push itself needs the owner's order |

## Files Already Handled (don't re-read)

- `internal/le/rfc/meta.go`: `readSupport` parses `-` or `<section> <rank>`; `checkRowCells` picks the table shape from `Brief`; `updatesFrom` reads the Updates cell
- `internal/le/rfc/render_ledger.go`: `statusSections` (10 keys, Heading, Intro, `Brief`), `placeRows` (rank clash refusal, rank sort), `renderStatusPage`
- `internal/le/rfc/provenshare.go`: denominator rule of 2026-09-02 (not-applicable and lower-layer stay in; `{rollup}` out)
- `internal/le/rfc/check_core.go`: `checkSuperseded`, `checkUpdatedBy`
- `internal/le/rfc/rfc.go`: `gatedLevels` is the mandatory set; `Requirement` layout (152 bytes, rangeValCopy threshold is 160)
- `internal/le/site/rfcledger.go`: per-row proof in `rfcLedgerRequirement.Covers[].Proof` (`rfcProofsByCover`); lift that predicate into `internal/le/rfc` in Phase 1
- `internal/le/spec/roadmap/roadmap.go`, `internal/le/spec/status/specstatus.go`: roadmap rows from spec Meta; no RFC link today
- Research results are in the spec's "Research findings" table; the reference-parser traps are listed there

## Traps met in this session

- The shared `./le` follows HEAD and is rebuilt in the background from committed code. A summary annotation the committed parser does not know makes the pretool hook refuse Bash calls naming `rfc/`, `docs/` or `internal/le/rfc/` for every session. Commit a parser change first, confirm the rebuilt `./le rfc check` accepts the new syntax, then commit the data. For Phase 1 the Support migration and the parser change must land in ONE commit, because the old parser refuses the new cell and the new parser refuses the old one; prepare the migrated summaries in the session scratch directory and copy them in just before `./le commit create`.
- Lint caps findings at 10 per linter per run; a clean-looking list can hide more of the same kind.
- `go test` under `./le job run` needs the feature tags: `tags="ze_core $(awk '$1 ~ /^ze_/ {print $1}' feature-gates.txt | sort -u | tr '\n' ' ')"`.
- `./le go lint run scope "./internal/le/rfc/..."` refuses while another session edits sources; retry.

## Edits (first steps of Phase 1)

EDIT 1: spec metadata table, `Phase` row
- Replace: `| Phase | 0/5 |` -> `| Phase | 1/5 |`

EDIT 2: new file `rfc/profiles/route-server.md`
- Add: `# Profile: route-server`, blank line, then a two-column `| Field | Value |` table with rows:
  `| Kind | use-case |`,
  `| Title | IXP route server |`,
  `| Release | yes |`,
  `| Roots | rfc4271, rfc4760, rfc6793, rfc7947, rfc9234, rfc1997, rfc8092, rfc7999, rfc6811, rfc8210, rfc9582, rfc7911, rfc7454 |`,
  `| Absent | RFC 7948, RFC 8212, RFC 8893 |`

EDIT 3: new file `rfc/profiles/lns.md`
- Add: same shape, `Kind use-case`, `Title L2TP network server`, `Release yes`,
  `Roots rfc2661, rfc1661, rfc1332, rfc1877, rfc5072, rfc1334, rfc1994, rfc2759, rfc2865, rfc2866, rfc2869, rfc5176`,
  `Absent RFC 1570, RFC 2867, RFC 2868, RFC 3162, RFC 4818, RFC 6911, RFC 3145, RFC 4861`

EDIT 4: new file `rfc/profiles/fttx-edge.md`
- Add: same shape, `Kind use-case`, `Title FTTx edge: customers to the internet over L2TP, local DNS for the LAN`, `Release yes`,
  `Roots rfc2661, rfc2516, rfc1661, rfc1332, rfc5072, rfc1877, rfc1994, rfc2131, rfc2132, rfc9915, rfc1035, rfc2181, rfc4035, rfc4862`,
  `Absent RFC 7084, RFC 3022, RFC 4787, RFC 5382, RFC 5905, RFC 8106, RFC 4638, RFC 4861`

EDIT 5: spec, section `## Design Insights`, after its existing two bullets
- Add: "Profile file format: a `| Field | Value |` table. Use-case profiles carry Kind (use-case), Title, Release (yes/no), Roots (comma list of stems), Absent (comma list of RFC numbers not yet in rfc/short). Technology aggregates carry Kind (technology), Title, Heading, Intro, Shape (wide/brief, replacing the `Brief` flag of `statusSections`), Release (n/a)."

Then, in order: the remaining five use-case profiles from the spec's "Initial profiles" table (route-collector yes, looking-glass yes, vpn-endpoint yes, vpn-concentrator no, ddos-flowspec no); the aggregate files; the profile parser tests first (`TestProfileUnknownRootRefused`, `TestCorpusProfilesParse`), then the parser. The Support migration commit described in the traps section follows.

## Then

    tags="ze_core $(awk '$1 ~ /^ze_/ {print $1}' feature-gates.txt | sort -u | tr '\n' ' ')"
    dir=$(./le session scratch ensure)
    ./le job run label unit-pkg quiet command go test -race -timeout 20m -tags "$tags" ./internal/le/rfc > "$dir/rfc-test.log" 2>&1
    ./le rfc check > "$dir/rfccheck.log" 2>&1

Read both logs. Expected in `./le rfc check`: no finding naming `rfc/profiles/`; pre-existing reds from other sessions (discrimination fingerprints, the SHIFTED verdict on RFC4271-5.1.2-3) are not this work's.
