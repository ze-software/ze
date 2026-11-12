# Handover: one-page site build, and why the full build is slow

Written 2026-11-12 at the close of the session that changed the front page
and the activity page. The page changes are committed and approved by the
owner. Two things cross the session boundary.

## 1. `le site build producer <name>`: written, works by hand, not committed

The owner asked why the tooling cannot render one page, and asked for the
answer to be in the binary's help.

### State

| Item | State |
|------|-------|
| `internal/le/site/buildproducer.go` (new) | In the working tree, uncommitted |
| `internal/le/site/actions.go`, `build.go`, `producer.go` | Modified, uncommitted |
| `internal/le/le/action/page.go`, `usage_test.go` | Modified, uncommitted: a keyword's help page lists the candidates its `Source` answers |
| `website/AI.md` section "Reviewing a page while you edit it" | Uncommitted hunk |
| `docs/contributing/gh-pages.md`, `running-commands.md` | Uncommitted hunks |
| Tests of the feature | Held out of the tree, see below |
| Independent review | Not done |

Exercised by hand in the session: `./le --name session help site build`,
`./le --name session help site build producer` (lists the 24 producers), and
`./le --name session site build producer activity producer home`, which wrote
those two routes and left every other page untouched.

No unit test of the feature has run.

### The blocker

`buildproducer_test.go` redeclares `stubBuildClock` (already in
`footer_test.go`) and `readArtifact` (already in `assets_build_test.go`), so
the `site` package's tests do not compile with it present. The fix is to
delete those two definitions and nothing else.

- The edit hook refuses the deletion as a weakening of `site.stubBuildClock`
  and `site.readArtifact`, although HEAD never held the file.
- An independent `ze-test-judge` agent approved both removals: the surviving
  helpers have the same signature, and `readArtifact`'s failure message is the
  stronger one.
- `./le test weakened approve unit site.stubBuildClock reason ...` refuses to
  record the verdict: "approval unit site.stubBuildClock is ambiguous across
  carriers" (`ambiguousApprovalCarrier`, `internal/le/rfc/approvalunits.go`).

The defect is owned by
`plan/post-release/engineering/spec-weakening-check-judges-units-head-never-held.md`.
The approve command's inability to name a carrier is a second face of it and
may not be in that spec yet: check, and add it there if it is missing.

The test file is kept unchanged at
`plan/handover/site-one-page-build/buildproducer_test.go.held`. It was moved
out of `internal/le/site/` so the package compiles for every session.

### Next steps

1. Once the hook or the approve command is fixed, copy the held file back to
   `internal/le/site/buildproducer_test.go` and delete the two duplicate
   helpers.
2. Run:
   - `./le job run label le-test quiet command ./le test unit pkg path ./internal/le/le/action/ run 'TestKeywordPage'`
   - `./le job run label le-test quiet command ./le test unit pkg path ./internal/le/site/`
3. Independent review, then commit the feature with its docs.
4. Remove this handover and its directory.

### Known limits of the one-page build

- It skips the daemon compile and the staging of sources, so a change to a
  stylesheet, a script, a copied asset, or the command catalog and YANG tree
  still needs the full build.
- It is not seconds yet for a page whose text lives in Go: `--name session`
  rebuilds le from the working tree on each call, and the activity producer
  measures the whole git history on each run.
- A plain `./le site build` renders Go-held page text from committed HEAD, so
  a full build during review puts back the committed text of every page whose
  uncommitted edit is in Go. Re-render those with `producer <name>`.

## 2. Why `./le site build` is not seconds: decisions waiting for the owner

A read-only analysis in the session ranked the causes from the code's
structure. No timing was taken: this machine's time figures are not
reportable. Three causes need the owner's ruling; none has been acted on.

| # | What the build does | The choice |
|---|---------------------|------------|
| 1 | `rfc.Check` runs inside the rfc-compliance producer (`internal/le/site/rfccompliance.go`, `collectRFCCompliance`) | It contradicts "building the website renders data; it never runs a check". Render a recorded check result, or drop the verdict section |
| 2 | Compiles the `ze` daemon from the working tree and runs it twice for the command catalog and the YANG tree (`refreshNativeSurfaces`, `yangcontract.BuildDaemon`) | Publish from a committed data file, or from a daemon built once per commit |
| 3 | A feature-freshness pass before the build (`internal/le/site/freshness.go`), several hundred git processes | Keep it in the build, or move it to CI and `le feature check` |

Accidental costs that give nothing up when removed:

- The RFC corpus is collected seven times per build; nothing shares the
  reading.
- `publishPluginRegistry` reads every Go blob from git, about 200 MB, in one
  buffered `git cat-file --batch` under a 30-second limit
  (`internal/le/repo/objects/objects.go`). That limit killed one build in the
  session.
- Repository activity is measured twice, once for a frozen talk.
- The 158 MB artifact is copied out and then back over itself.
- Every page is written about three times; about 7 of 1,107 actually change.

Verified in source by the supervising session: items 1 and 2 and the
30-second limit. The rest is the analysis agent's reading.

### Decision 1 is taken

The owner has decided decision 1: the website's RFC data is recorded once, in
a git-ignored folder of the website source tree (`website/recorded/`), so it is
never pushed, and every RFC page and published RFC data file is generated from
it. The whole recording is regenerated in three cases: a build finds it absent
or more than a week old by the saved time it records, `./le site rfc-data` is
run, or the weekly update runs, which calls `./le site rfc-data` before its
site build. The rule is in `ai/META-DECISIONS.md`, "Website design". The design
and its open questions are in
`plan/post-release/engineering/spec-site-rfc-recorded-data.md`, which is the
home of decision 1. Reading the source for that spec found eight RFC corpus
collections per full build, not seven: the docs producer's `rfc.StatusPage` is
the eighth, and `derived.EnsureAll` adds a ninth when an RFC derived artifact
is absent.

## 3. Open from the page work

- The tests that cover the approved page changes pass: `./le test unit pkg
  path ./internal/le/site/ run 'Activity|TestHomeOpening|TestHomeIntro'`,
  run after the last page commit.
- The whole `site` package run at closure ended red, with the one-page build's
  uncommitted code in the tree. None of the failures names text this
  session's page commits changed; whether each is older than the session or
  comes from another session's work was not traced:

  | Test | What it reported |
  |------|------------------|
  | `TestTheComplianceIndexLinksEveryStemOfThisCheckout` | Still running when the package hit its 20-minute limit |
  | `TestHomeHeroDerivesFromTheManifest` | No `hero-start-action` link to the quickstart web tour in `homeTemplate`. Red before this session's first edit |
  | `TestTheHomepageProofStripCarriesItsStatSpans` | 14 proof-strip stat spans, want 9 |
  | `TestAFactTheSnapshotLostStopsTheHomepage` | The refusal names `repo.go_source_lines_display`, not the token the test expects |
  | `TestTheSharedHeaderReadsAsThePublishedHeader` | Quickstart caption: got "Two BGP peers in under five minutes", fixture wants "Two local BGP peers and a received route" |
  | `TestTheLabISOSizeFallsBackUntilAReleaseCarriesOne` | The lab page source lacks the ISO size sentence |
- Nothing was pushed. `../gh-pages` is regenerated locally and uncommitted.
