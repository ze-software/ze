# Handover: ze thanks (third-party credits), ready to close, 2026-11-20

Spec: `plan/release/delivery/spec-thanks-third-party-credits.md`, in-progress,
all seven phases implemented and committed. What is left is closure
(`/ze-close`), in a fresh session. This replaces
`plan/handover/2026-11-20-thanks-third-party-credits.md`.

Per-spec state file, with every phase handoff:
`./le spec state latest spec thanks-third-party-credits`.

## Committed

| Commit | What |
|--------|------|
| `bf60540663` | Generator, tree check, commit gate, rule point `repo-maintenance/directives/credit-every-module-the-shipped-binary-links` |
| `e90ab69e36` | Declaration, generated data (119 works), bundled licence texts, asset copies, the `ze thanks` answer |
| `b97279eb26` | Five functional tests, each seen red against a broken handler and run 80 times under load |
| `099edc4972` | The traffic-usage comments describe the plugin itself |
| `1b17c0d2f8` | The lint tool is declared `GPL-3.0` with no note |
| `7581427e54` | The spec records the owner's rulings |
| `550bf38477` | The Dependencies page (licence, holder, "Also in the shipped binary"), llms.txt licences, `NOTICE`, the legal page; fixtures refreshed with the judge's approval |

## Verified after the last commit

- `./le site build` and `./le site check` both exit 0 into `../gh-pages`, run
  with a launcher built from a commit that holds `550bf38477`. The built
  dependencies page holds the new section, `llms.txt` states the licence on 66
  lines, the legal page names `ze thanks`.
- `./le deps credits update`, `./le deps credits check` and
  `./le web vendor update` change nothing and exit 0.
- `../gh-pages` is regenerated locally only. Nothing is pushed.

## Owner rulings of 2026-11-20 (all in the spec)

- The traffic-usage plugin is Ze's own code: no credit, no mention.
- The root certificate data is credited under the Go module only.
- Where upstream does not say, do not guess: the lint tool is `GPL-3.0`.
- The licence matters more than the holder. Showing the holder is fine and
  stays. `NOTICE` carries no per-work attribution.
- The rendered pages are approved.

## Left for closure

1. The spec's `## Progress` section still says phases 6 and 7 wait for review:
   update it with `550bf38477` and the site build and check.
2. `/ze-close`: deliverables, security and docs checks, the Review Gate, the
   two closing commits.
3. The first handover file may be removed with the spec's closure.

## Reds that are not this work's

- `internal/le/site`: `TestADetailPageStatesWhatAnUndeclaredCommandTakes`,
  `TestTheHomepageRFCCardSaysWhatTheOldFigureCounted`, and the package's
  20 minute limit in `TestBuildRendersEveryPublishedRoute` under machine load.
- `internal/component/web`: `TestWebHandlerGoldenOutput/get-l2tp-session-json`.
- `internal/le/web/vendor`: `TestEveryAssetDirectoryIsDecided` (fleet assets).
- `./le arch tier check`: an IPC schema test import, the PPPoE VPP build tags.

## Tooling defects met, not fixed, not journaled (no owning spec)

| Defect | Evidence |
|--------|----------|
| The Bash hook read the lint tool's name inside a commit message argument as a raw lint run | Refused `./le commit create ... body "...golangci-lint is stated..."`; a body file passed |
| `./le spec design documents` reads only the first path of a line that lists several | First handover |
| The copy hook refused a copy from the job scratch directory as a copy out of a worktree | First handover |
| A full site build started just after a commit runs the previous launcher and publishes the old page with exit 0 | The first `./le site build` after `550bf38477` wrote the old dependencies page; `./le site check` passed on it |
| Go build cache entries vanish under a running compile when another session cleans | "could not import sync ... no such file or directory", twice |
