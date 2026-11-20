# Handover: ze thanks (third-party credits), 2026-11-20

Spec: `plan/release/delivery/spec-thanks-third-party-credits.md`, in-progress,
Phase 5 of 7. Design approved by Thomas on 2026-11-20 (digest 5acc408a7f6a).
WIP cap exception for this spec only, his, recorded in the spec.
Sibling skeleton: `plan/release/delivery/spec-appliance-image-third-party-notices.md`.
Per-spec state file with every phase handoff:
`tmp/session/2026-11-20-045524/state/session-state-thanks-third-party-credits-80b76754-7771-4e7f-a191-4ad037837da8.md`
(`./le spec state latest spec thanks-third-party-credits` finds it).

This session stopped at the context threshold, mid Phase 5, with the working
tree compiling and two agents still to report. Nothing below is closed.

## Committed

| Commit | What |
|--------|------|
| `43b3d27123` | Phase 2: licence texts for htmx, uPlot, swagger-ui, Go, ExaBGP, the IANA statement; the vendored web check refuses a directory with no licence file |
| `4096f645dc` | Phase 1: `ze thanks`, `ze thanks licence`, daemon paths, the `name` pipe operation, the empty registry, both specs |
| `32e51d3548` | A fix of its own: `./le repo update` called four areas by retired names |

## Uncommitted, in the working tree

| Piece | Files | State |
|-------|-------|-------|
| Phase 3, the declaration | `website/data/dependencies.json` | 66 curated + 45 linked modules, 22 works, upstream corrections applied. Every declared holder verified against the work's own files by the main thread |
| Bundled library licences | `third_party/swagger-ui-bundled/` (14 texts + README) | Downloaded from unpkg, byte-identical to the research copies |
| Phase 4, generator, check, gate | `internal/le/deps/credits/`, `internal/le/register.go`, `internal/le/deps/check.go`, `approve.go`, `policy.go`, `internal/le/repo/compiles/matrix.go`, `docs/architecture/third-party-credits.md`, `docs/contributing/dependency-approval.md`, `ai/INDEX.md`, `.golangci.yml` (misspell exemption for `licence`) | Package tests were green before the continuation's edits; not re-run since. Lint not re-run |
| Generated data | `internal/core/credits/data/` (119 works, 101 texts) | Written by the working tree's generator through a temporary driver; must be regenerated after the open fixes below |
| Phase 5, the answer | `internal/core/credits/`, `internal/plugins/thanks/`, presence declarations in `internal/component/web/render.go`, `lg/embed.go`, `api/rest/embed.go`, `api/rest/server.go`, `internal/plugins/container/seccomp/profile.go`, `internal/exabgp/bridge/bridge_event.go`, `internal/core/ipregistry/registry.go`, `internal/core/portname/services_table.go`, `internal/thirdparty/fat/credits.go`, `docs/guide/command-reference.md`, `docs/guide/api.md`, `features/third-party-credits.md` | Only `internal/core/credits` was run: green except the two phase 1 tests below |
| Asset consumer copies | `htmx.LICENSE`, `uPlot.LICENSE` in the web, lg and chaos asset directories; three swagger-ui files in `internal/component/api/rest/assets/` | Synced by the working tree's tool; its check was green |
| Draft functional tests | `test/draft/ui/thanks-list.ci`, `thanks-licence.ci`, `pipe-local-command.ci`, `test/draft/plugin/thanks-daemon-list.ci`, `thanks-daemon-licence-name.ci` (git-ignored) | Never run |
| Temporary driver | `internal/le/deps/credits/_credgen/main.go` | Remove once `./le` has rebuilt from a commit that carries the generator; it exists only because the launcher could not rebuild |

## Agents still to report when this session stopped

| Agent | Task | Where its result lands |
|-------|------|------------------------|
| Generator continuation | Fix two check refusals: four `credits.DeclarePresent` literals in `internal/component/api/rest/embed.go` still use the banner titles (the works are now named deep-extend, fast-json-patch, dompurify, use-sync-external-store; an unknown name panics at start-up), and `holderLine` reads a quoted title in `third_party/iana/README.md` as a holder | Its handoff is appended to the state file |
| Test judge | Verdict on removing `TestRowsRefuseWithoutData` and `TestLicenceRefusesWithoutData` (`internal/core/credits/credits_test.go`), and on whether `LicenceDocument()` answering an empty document with a nil error, in a binary that carries nothing, is a silent empty answer | Recorded through the approval route; a refusal names the change owed |

## Next steps, in order

1. Read the state file's last handoffs; confirm both fixes above are in the tree and read the judge's verdict.
2. Regenerate and check, through the temporary driver while `./le` is stale:
   `./le job run label credgen quiet command go run ./internal/le/deps/credits/_credgen update`, then the same with `check`. Expected: no refusal.
3. Run, one at a time: `./le test unit pkg path` over `./internal/le/deps/credits/`, `./internal/le/deps/`, `./internal/core/credits/`, `./internal/plugins/thanks/`, `./internal/component/api/rest/`, `./internal/component/web/`, `./internal/component/lg/`, `./internal/thirdparty/fat/`, `./internal/exabgp/bridge/`, `./internal/core/ipregistry/`, `./internal/core/portname/`; then lint over the same packages, `./le arch tier check`, `./le doc check all`, `./le feature check`.
4. Run the drafts (`./le test ui --draft --pattern thanks`, `./le test plugin --draft --pattern thanks`, `./le test ui --draft --pattern pipe-local-command`), fix, then promote them per `ai/skills/ze-test.md`. Promoting `pipe-local-command.ci` turns `TestCommittedScenarioLaunchesOnlyTheCompiledHelper` green.
5. Commit order matters: the commit gate in the generator refuses a commit that carries the declaration or the data unless they match, so the generator lands first (tool code and its pages), then one commit with the declaration, the generated data, the bundled licence texts, the asset copies and Phase 5.
6. Once `./le` has rebuilt, run `./le deps credits update` and `./le web vendor update` for real: both must report nothing to change. Then remove the temporary driver.
7. Phase 6, the website page (`internal/le/site/dependencies.go`, `derived.go`): licence and copyright columns, the section "Also in the shipped binary", the licence in each `llms.txt` line. Website rule: render for Thomas's review into `../gh-pages`, tests and commit after he approves. `./le site build` runs the build of HEAD, so the renderer change must be committed before it can be shown; say so to him.
8. Phase 7: `NOTICE`, `website/legal/legal.md`, remaining pages. Then `/ze-close` in a fresh session.

## Blocked outside this work

- The committed tree does not compile (another session's L2TP and PPP commits), so the `./le` launcher cannot rebuild and still runs a build from before this spec's commits. Until it does, `./le deps credits ...` does not exist and `./le web vendor update` lacks the new rows.

## Decisions waiting for Thomas

| Work | What was found | Decision |
|------|----------------|----------|
| The traffic-usage plugin, ported from `lan-bandwidth-exporter` | Upstream is Exa's internal repository (authors Isaac French and Laurence Syree); no licence file; its only licence statement is the string "GPL" in its eBPF program, which Ze's port repeats | How to credit it and under what licence. It is not declared, so `ze thanks` does not list it |
| The NSS root certificate data inside `golang.org/x/crypto/x509roots/fallback` | The module states only Go's licence and says the data is parsed from the NSS trust store | Credit it under the Go module only, or add Mozilla's terms once read upstream |
| `golangci-lint` (development tool, not shipped) | Upstream says GPL version 3 and nowhere "only" or "or later" | Recorded as upstream states it; confirm that is how the website should show it |

## Defects found on the way

| Defect | State |
|--------|-------|
| `./le spec design documents` read only the first path of a line listing several files | Confirmed; not in the journal (a row needs an owning spec) |
| The copy hook refused a copy from the job scratch directory as "a copy out of a worktree" | False refusal; not worked around; not in the journal |
| `TestEveryAssetDirectoryIsDecided` is red on `internal/component/web/fleetui/assets` | Another session's (fleet interface); named in commit `43b3d27123` |
| gosec G204 in `GoFiles` of `internal/le/deps/check.go` | Pre-existing line; the source fix is to set the command's directory instead of passing `-C` |
| `LicenceDocument()` may answer an empty document with no error in a binary that carries no work | With the test judge |
