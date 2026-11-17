# Handover: external plugin schema, from phase 7

Written 2026-11-16. It replaces `42-plugin-external-yang-phase-5.md` for what
comes next. The session that ran phases 5 and 6 is too large to supervise
phase 7.

## Where the work is

| Item | Value |
|------|-------|
| Spec | `plan/post-release/features/spec-plugin-external-yang-at-startup.md`, status in-progress, phases 1 to 6 done, 7 to 9 open |
| Server | Branch `worktree-spec-external-yang`, worktree `.claude/worktrees/spec-external-yang` of the server checkout |
| Laptop | The work is on the laptop's `main`, carried by hand and signed by the owner: `9b45d7f964` (Python example), `c12ae908a5` (phase 5), `c7f081849d` (phase 6 with the owner's ruling) |
| Per-spec state file | `tmp/session/2026-11-15-231106/state/session-state-plugin-external-yang-at-startup-99a3a802-6ea0-45ad-a7f7-a342c1f9baae.md`, untracked, server worktree only: each phase's handoff with file digests |
| Implementation model | The owner asked for Fable for implementation agents |

## What works, and how it was seen

| Behaviour | Evidence |
|-----------|----------|
| A config holding an external plugin's own section loads at `ze start`; the plugin gets it at Stage 2 | Live with `examples/plugin/python/acme_monitor.py`; `test/plugin/external-yang-own-root.ci` |
| Ze enforces the plugin's module (range, mandatory leaf) | Live (interval 11 refused); unit test at the config load; `external-yang-boot-accepts-what-offline-refuses.ci` |
| A commit on a running Ze reaches verify then apply; a verify refusal fails the reload | Live with the Python example over SIGHUP; `external-yang-commit.ci` |
| With no daemon, Ze starts no external plugin and refuses its section as not available | Live behind a marker-writing wrapper; `external-yang-offline-not-available.ci` |
| Failure paths reach `ze doctor` | Stop record for a plugin that never declares; event `doctor-plugin-stop-unfinished` |

## The owner's rulings, not to be re-opened

1. The load order of `docs/architecture/hub-architecture.md`: declarations
   first (the `plugin` block and the command line), run the declared external
   plugins, Stage 1 brings the module, then the full parse.
2. A plugin, its declaration and its module change only when `ze` restarts.
3. "If a user use an external plugin the security is on him."
4. Given 2026-11-16, verbatim: "plugin command requiring external plugins which
   can not be started from the code when no daemon is running to pass it to are
   not available, for command built-in then yes you should run them as we can
   look them up in the yang hardcoded in the binary". With no daemon running,
   Ze starts no external plugin's program. This reversed the spec's earlier
   decision that offline commands read the declaration through declare mode.
5. He agreed that `show plugin declarations config <file>` follows ruling 4,
   and to the order of work below.

## Outstanding items, every one

| # | Item | Detail |
|---|------|--------|
| 1 | Local command to a running daemon | Ruling 4 says "when no daemon is running to pass it to". `ze config validate` never asks the daemon today, so with Ze running the operator still reads "not available" and must open `ze cli` and type `show config validation <file>`. A design is owed to the owner before any code: how the local command finds the daemon, which files the daemon may read. Research was started in the previous session |
| 2 | `show plugin declarations config <file>` | It still runs a config's programs with no daemon. To follow ruling 4. In progress when this was written: check the branch tip |
| 3 | Phase 7, restart rule | A reload whose plugin declarations differ from the published set is refused, naming the restart. Boot must record which plugins declared, so a daemon stops naming every schema-less plugin beside an unknown name |
| 4 | Phase 8, surfaces and doctor | The schema CLI (`getExternalPluginYANG`), the editor's completion (draft `test/draft/editor/external-yang-completion.et`), the three codes `doctor-plugin-schema-undeclared`, `-refused`, `-changed` |
| 5 | Phase 9, measure | Spec Q10. On a host whose performance is known: the owner's Mac, not the server |
| 6 | Drafts still red | `test/draft/plugin/external-yang-query-failed`, `-invalid-refused`, `-clash-refused` (each waits on a schema doctor code), `-declaration-changed` (phase 7), `test/draft/reload/external-yang-reload-refused` |
| 7 | Laptop: `ze config diff`, `dump`, `graph` | Left out of `c7f081849d`: another session was restructuring them, uncommitted, around a supplied-content route. They start no program and refuse an external section as a plain unknown keyword. Add the shared step's call once that work is committed |
| 8 | Laptop: tests not run | `internal/component/plugin/server` did not compile there (another session's request-cancellation edit), so only config, plugin and test/externalyang were tested on the laptop. `cmd/ze` did not build there either |
| 9 | Lint | Not run on phase 6 or on the ruling's commit. Phase 5's two findings were corrected and not re-linted |
| 10 | `TestNoPythonLeRemains` | Refuses the tracked `.py` files of `examples/plugin/python/`. An exemption for that directory needs the test judge. In progress when this was written |
| 11 | Phase 5 leftovers | The early start precedes the `environment` block being applied (spec R-15); `judgeConfigBeforeBoot` and the two `hub.ConfigRefused` closures have no unit test; a held process the manager removed is no longer joined by a later stop |
| 12 | Pushed config | An invalid pushed config that declares an external plugin now ends the process after restoring the previous config. Told to the owner; he did not object |
| 13 | Conformance checker | Owner decision C-1 of the earlier handover is still open: may the daemon run a program a remote caller names. Until then `show plugin conformance run` is local-only and `TestLocalCommandDaemonMatchesLocal` is red for it in `cmd/ze/hub` |
| 14 | Python library | `rollback` never ran against a live Ze. `ze_plugin.py` is 847 lines |
| 15 | Owner decision D-3 | Whether a plugin's own command argument model is part of this spec. Open; blocks nothing before the last phase |

## Reds that predate this work

In `./internal/le/`: `TestNoPythonLeRemains`, `TestCensusActionsDeclareHeavy`,
`TestEveryRegisteredVerbIsInTheVocabulary`, `TestNoNameEmbedsAVerbOrTest`,
`TestHelpAiVerbsRendersTheVocabulary`,
`TestNoAgentSurfaceTellsAnAgentToRunAWholeTreeVerify`. In `./cmd/ze/hub/`:
`TestBuildTag_Gate12_AbsentBinaryDropsSymbols` (an L2TP symbol in the bare
`ze_core` binary).

## Carrying work to the laptop

A commit cannot be created on the laptop from a remote pane: signing fails
there (`gpg failed to sign the data`). The routine that worked twice: fetch the
branch on the laptop, apply the change to the working tree with a script the
laptop pulls from the server, test what builds, prepare the commit with
`./le commit create` (files with other sessions' uncommitted edits go in as
`hunks`), and give the owner the one `bash tmp/commit-...sh` line to run. A
plain cherry-pick refuses because the laptop's `main` holds hundreds of
uncommitted files of other sessions. Before applying, check that no file the
change touches is being restructured, uncommitted, by another session: one
apply broke that session's build for a few minutes and had to be reversed.

## Tooling traps met

| Trap | Effect |
|------|--------|
| The functional runner's `-pattern` matches a test's path, and this worktree's directory is named `spec-external-yang` | `-pattern external-yang` runs all 1205 plugin tests. Use a full test name or the numeric ids from `-list` |
| `./ze` keeps the binary it built once | Remove `bin/ze` to rebuild from the working tree |
| The built `ze` binary was damaged on disk once after running correctly | One 4 MiB block differed from a rebuild; a rebuild works. The crash left `crash/.pending/` files: never build a commit's file list from `git status` without reading it |
| `git status --short` prints the branch header on these machines | Use `--porcelain` in scripts |
| An edit is refused until the main checkout's style guide and doors were read | Tell every agent to read the main-checkout copies first |
| The test judge records its own approvals | `./le test weakened approve` runs from the judge agent; the main thread never records a row for it |
