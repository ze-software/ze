# Handover 46: looking glass search fix, in a worktree

Written 2026-11-18 by the session that implemented
`plan/release/fixes/spec-lg-search-matches-what-the-pages-promise.md`. It passed
the context threshold before the last commits landed.

## Where the work is

| Item | Value |
|------|-------|
| Machine | the server (`claude`) |
| Worktree | `.claude/worktrees/lg-search-matches-pages`, inside the main checkout |
| Branch | `lg-search-matches-pages`, no upstream |
| Based on | `c49fd96ac6`. `main` has since moved to `89d8dd6a9c` or later (laptop commits plus the amended feature spec) |
| Spec | `plan/release/fixes/spec-lg-search-matches-what-the-pages-promise.md`, status `in-progress`, phase 4/4, claimed by the writing session (`./le spec release` it or claim it anew) |
| Owner order | Thomas ordered the implementation done in a worktree, so edits and test runs happen there |
| Companion spec | `plan/post-release/features/spec-lg-peer-history-origin-search-and-named-nodes.md` on `main`, status `design`, with three choices waiting for Thomas |

The Bash hook refuses, from a session entered into the worktree, any command
that names git together with another checkout. Leave the worktree session
(`ExitWorktree`, keep) to query `main` or the laptop.

## Landed on the branch

| Commit | Content |
|--------|---------|
| `47784969e7` | `bgp-rib`: `covering <address>` and `exact <prefix>` selectors of `show bgp rib received`, read from the per-peer prefix trie; `store.Store.Covering`; docs; tests |

## Written and proven, not yet committed

The looking glass half: `internal/component/lg/` (`prefix_lookup.go`,
`aspath_pattern.go`, handlers, help, stylesheet, tests, captures),
`internal/test/fixture/lg_covering_lookup.go` and `lg_aspath_search.go`,
`test/plugin/lg-covering-lookup.ci`, `lg-aspath-search.ci`, `lg-ui-pages.ci`,
the looking glass docs, and the spec.

| Proof | Result |
|-------|--------|
| `./le test unit pkg path ./internal/component/lg/` | green |
| `./le go lint run scope "./internal/component/lg/..."` | clean |
| The three functional tests, run one at a time with `./le --name session test bgp plugin --pattern <name>` | pass |
| Red proof | `lg-covering-lookup` and `lg-aspath-search` fail when the looking glass sends the old text `prefix` filter and the spaced AS path |
| `./le test stress-repro run suite "plugin --draft" test <name> any-failure iterations 80` | no failure, for all three, before promotion |
| Test judge | every changed expectation approved; rows in `tmp/commit-weakening-approved-c581069e.md`; `./le test weakened check` clean |

The commit is prepared as two files in the session scratch directory of the
worktree, `tmp/session/2026-11-18-120834/scratch/`: `lg-commit-body.txt` (the
message body, with its `Test-judge:` lines) and `lg-commit-files.txt` (the
file list). The command is:

`./le commit create subject "lg: address, prefix and AS path searches do what the pages say" body-file <body> file-list <list>`

It was refused once, by `./le arch error-inventory check`: three `http.Error`
sites hand the writer an error the gate cannot see is an operator error. They
are the engine-failure answers of `handleGraph` and `handleUIRouteDetail`, and
the bad-prefix answer of `handleUIRouteDetail`. An agent was fixing those when
this was written. Its edits change looking glass files after the proofs above,
so rerun the package test and lint, and if it changed a capture or a test
expectation, send that to a `ze-test-judge` agent before committing.

## Update, later the same day

- The selector fix is committed: `b379368e79`. `--pattern` now matches a
  test's id and name only, and refuses a pattern that matches nothing. `--list`
  still prints the whole suite whatever `--pattern` says, which the fix left
  alone.
- The three error-inventory sites are corrected, and the correction went
  wider: `engineRefusal` in `internal/component/lg/server.go` gives every
  engine-failure answer of the looking glass three lines (problem, reason,
  fix), on the pages and in the birdwatcher JSON `error` body, whose first
  line is unchanged. `docs/architecture/api/birdwatcher-compat.md` section 3.1
  and `docs/guide/looking-glass.md` say so. Outside clients see the longer
  text.
- That left five expectations on the old one-line text: two assertions in
  `handler_api_dispatch_test.go`, the `ask` table test in `handler_ui_test.go`,
  `test/plugin/lg-dispatch-refused.ci` and `test/web/lg-stream-error.wb`. An
  agent was updating them, through draft copies for the two functional files.
- Owed before the looking glass commit: a `ze-test-judge` verdict on those
  five, the looking glass package test and lint, the draft runs and promotion
  of the two functional files, `./le arch operator-error check`, and the
  prepared `./le commit create` line. Add
  `internal/component/lg/server.go`, the two promoted functional files and
  anything else the agent names to `lg-commit-files.txt`, and a sentence on
  the engine-failure answers to `lg-commit-body.txt`.

The section below describes the state before this update.

## In flight when this was written

| Agent task | State | Owed after it reports |
|------------|-------|-----------------------|
| The three error-inventory sites | editing `internal/component/lg/` | the checks named above, then the looking glass commit |
| `--pattern` selector fix | editing `internal/test/runner/selection.go`, `internal/test/cli/` help texts and `cmd_bgp_selection_test.go`, `docs/functional-tests.md`, `docs/contributing/running-commands.md` | `./le test unit pkg path ./internal/test/cli/ run 'TestBgp(Pattern|Positional|Unmatched|Selection)'`, `./le test unit pkg path ./internal/test/runner/ run 'TestTestsSelect'`, `./le go lint run scope "./internal/test/..."`, then `./le --name session test bgp plugin --pattern lg- --list` (expect about 17 tests, never the whole suite), then a commit of its own |

The selector defect: `recordMatches` in `internal/test/runner/selection.go`
matched a test's absolute file path as well as its id and name, so in a
checkout whose path contains the pattern text every test matched. In this
worktree `--pattern lg-` selected all 1276 plugin tests. Until the fix is
committed, never pass `--pattern lg-` here: give the whole test name.

## Left to do, in order

1. Commit the looking glass piece (above).
2. Commit the selector fix.
3. Commit `internal/component/web/l2tp_detail_templ.go` alone. Template
   regeneration rewrote it: the committed file is stale on `main`, from
   another session's work. It is not part of this spec.
4. Rebase the branch onto the current `main` (`git pull --rebase
   --no-autostash <main checkout> main` inside the worktree), rerun the scoped
   tests, and tell Thomas the branch is ready. He lands it with `git rebase
   lg-search-matches-pages`.
5. Read the whole branch diff, then hand the spec to `/ze-close` in a fresh
   session for the Review Gate and closure. This session authored the work and
   must not review it.

## Reds that are not from this work

They arrived with the laptop's in-progress commits. The first two were
compared against the unchanged head; the others were attributed by reading
what each test covers.

| Red | Evidence |
|-----|----------|
| 30 tests of `internal/component/bgp/plugins/rib` (ASPA retention, FlowSpec, RIB source identity, `TestRFC9117AuthorizationUsesLongestCoveringRoute`, others) | same 30 on the unchanged head in the main checkout |
| One lint finding in `rib/storage/pathset.go` (`nestingReduce`) | file untouched by the branch |
| `TestClearBgpRibBareIsRefused` in `bgp/plugins/cmd/rib` | the branch adds a pipe filter and schema descriptions only |
| Three tests of `internal/test/fixture` (`TestToolFixturesReachTheirProducer`, `TestRulesFixtureRootAliasesReachOnlyOwnedTree`, `TestPluginObserversWaitForTheScriptedPeer`) | none names the new fixtures |
| `./le doc check all`: `show data encode` has no schema entry and no long description; two stale file references in `docs/research/l2tpv2-ze-integration.md` | none in a page the branch touches |

## Facts worth keeping

- The laptop's `main` is usually dirty and ahead of origin. To bring a server
  commit to it: bundle the laptop's new commits to the server, `git pull
  --rebase --no-autostash <bundle> main` on the server (clean tree), then
  `git pull --ff-only --no-rebase claude:Code/github.com/ze-software/ze/main
  main` on the laptop. Thomas approved that route on 2026-11-18, for a
  fast-forward whose files are untouched on the laptop.
- A subagent of a session that is entered into a worktree is confined to that
  worktree: its edits to the main checkout are refused.
- The edit hooks count a subagent's reads of the style guide and the doors
  only at the main checkout's paths, not the worktree's.
