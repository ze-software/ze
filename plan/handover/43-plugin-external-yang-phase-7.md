# Handover: external plugin schema, from phase 7

Written 2026-11-16. It replaces `42-plugin-external-yang-phase-5.md` for what
comes next. The session that ran phases 5 and 6 is too large to supervise
phase 7.

**Read the last section first.** "State at the end of the session,
2026-11-18" is the current state, and it alone says what to do next. Every
section above it is the history of the session, kept for its reasons and
evidence. Where a section above says something is pending, owed,
uncommitted, planned, being written, being built or held on a branch, the
last section supersedes it.
The owner's rulings below and the tooling traps still hold.

## Where the work is (superseded: history of 2026-11-16)

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

## Outstanding items, every one (superseded: the list of 2026-11-16, every item since done or moved)

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

## Added 2026-11-17: `ze config validate` asks a running ze, and the owner's decision on it

Since this handover was first written:

| Commit | Where | Content |
|--------|-------|---------|
| `33e690ac64` | Server branch; on the laptop as `d4fb5888c7` | `show plugin declarations config <file>` starts no program with no daemon (item 2 done); the Python example is exempt from `TestNoPythonLeRemains` by an exact two-file list (item 10 done, the test stays red on three tests of other work); offline and boot functional tests in their final form |
| `a0656f1c59` | Server branch only, HELD | `ze config validate` asks a running ze first (spec phase 10, item 1); the `ze:keyword-only` argument statement; SSH exec keeps the payload of a refused command |

`a0656f1c59` is proven on the server (unit tests of twelve packages, three
functional tests, live with the Python example). It is not carried to the
laptop, because its daemon side duplicates work another session is doing there.

**Owner decision, 2026-11-17: "yes, keep theirs and point your client at it".**
Another session on the laptop is building, uncommitted when this was written,
`show config supplied validation|dump|fmt|graph|diff`: the caller sends a
config's bytes and the daemon judges them with the existing producers. That is
the daemon side. From `a0656f1c59` only the client side is kept.

What the switch is, to do once that work is committed on the laptop:

| Step | Detail |
|------|--------|
| Remove | The `content` leaf added to `show config validation` (`ze-config-cli-cmd.yang`) and the content read in `dataValidate` (`config_data.go`), with `TestShowConfigValidationJudgesContent` |
| Point the client | `judgeConfig` (`config/cli/validate_route.go`) sends `show config supplied validation content <base64> name <label>` in place of `show config validation file <name> content <base64>` |
| Their contract, read on the laptop | `content` is mandatory, 4 to 261,120 padded base64 characters (195,840 bytes), same pattern as the existing content leaves. `name` is optional, a filename label with no directory and no control character, default `supplied.conf`; the client passes the base name of the operator's file so diagnostics name it. The handler is `validationBytesAnswer(data, name)`, the producer of the file form, so the answer keeps the `ValidateResult` shape the client draws. It refuses positional words itself, and declares its content argument through `registry.Meta.SuppliedInput` |
| Check before relying on it | That the supplied validation, inside a daemon, judges with the module set boot published (an external plugin's section accepted). The functional test `external-yang-validate-asks-running-ze` proves it end to end once pointed at their command |
| Keep | `sshclient.ReachDaemon` and doctor's use of it, the refusal texts (`DaemonUnanswered`, `Unanswered`), `ze config validate --user`, the SSH payload fix with its unit tests, `ze:keyword-only` on the two `update data` content leaves. Consider declaring `ze:keyword-only` on their `supplied-content` grouping with that session's agreement |
| Then | The same client route for `fmt`, `dump`, `diff` and `graph` uses their four other supplied commands: no new daemon side is needed |
| Where | On the laptop, in a detached worktree at the commit that holds their work, not in the shared checkout; the server cannot test it until that commit reaches it |

The spec's phase 10 text still describes the `content` leaf on
`show config validation`: correct it in the same work as the switch.

Tooling notes from this round: an apply to the laptop's working tree was denied
to the agent by the permission system once; the owner ran the apply script
himself. A page edited by another session between assessment and apply made the
plain patch refuse; the apply script now merges such a file three-way and
refuses before writing anything on a real conflict.

## Added 2026-11-17, later: the switch and phase 7 are done and on the laptop

This section replaces what the one above says is pending.

| Piece | Laptop `main` | Server |
|-------|---------------|--------|
| `ze config validate` asks a running ze first, through `show config supplied validation` (item 1, spec phase 10) | `552c08a28a` | `09ad03b19d` on `worktree-validate-client` |
| Phase 7, the restart rule (item 3) | `2210546edb` | `350a334e1c` on `worktree-validate-client`; first built as `b4a5be2c4c` on `worktree-spec-external-yang` |

The session doing `show config supplied` (Codex) confirmed its contract in
writing, said the command, arguments and answer shape will not change, and
took the `diff`, `dump` and `graph` gap itself (item 7): it corrects those
producers to read `config.SchemaWithDeclaredPlugins`.

**Where to work from now.** The laptop's `main` is the truth. The server branch
`worktree-spec-external-yang` has diverged from it: it still holds `a0656f1c59`
with the duplicate `content` leaf, and none of the laptop's other work. Build
further phases in `.claude/worktrees/validate-client` (branch
`worktree-validate-client`), or in a fresh worktree made the same way: on the
laptop, `git bundle create` of `main` from the newest commit the server holds,
`scp` to the server, `git fetch` the bundle there, `git worktree add -b <branch>
<path> FETCH_HEAD`. Run every `./le` there with the working directory set to
that worktree (`env -C <worktree> ./le ...`): invoked by absolute path from
another worktree, the launcher tests the directory it was called from.

**What is left of the spec:** phase 8 (the schema CLI `getExternalPluginYANG`,
the editor's completion with the draft `test/draft/editor/external-yang-completion.et`,
the codes `doctor-plugin-schema-undeclared` and `-refused` that the drafts
`external-yang-query-failed`, `-invalid-refused` and `-clash-refused` wait for;
`doctor-plugin-schema-changed` exists since phase 7), phase 9 (the measurement,
on the owner's Mac), and the same client route for `fmt`, `dump`, `diff` and
`graph` through their supplied commands. Those drafts are git-ignored and exist
only in the server worktree `spec-external-yang`.

**For the owner to pass to the supplied-config session:** its `supplied-content`
grouping does not declare `ze:keyword-only` (added by `552c08a28a`), so a stray
word that is valid base64 binds to `content` before the handler's check of
positional words.

**Reds seen on the laptop's history at `0402601f9d`, not from this work:** in
`plugin/server`, `TestRecognizesCommandUsesDispatchRegistrations`,
`TestRecognizesCommandTracksHiddenPluginsAndSubsystems`,
`TestRelayStoredRouteFromAnUnnamedCallerStaysUnnamed`,
`TestRelayStoredRouteJSONTransport`,
`TestHandleDispatchCommandRPCPreservesPluginIdentity`; in `cmd/ze/hub`, the
`TestGraphicalImport` tests, `TestWebCLIViewActionRouteRefusesUnknownHandle`,
and `TestLocalCommandDaemonMatchesLocal` on the supplied commands (a help hint
printed locally and omitted by the daemon route; a later commit on `main`
appears to fix it); in `config/cli`, the config sweep on 28 configs refused as
`config-bgp-peer` "incomplete destination grants"; in `test/reload`,
`bgp-attach-no-route-refresh-restarts` and `bgp-attach-sent-direction-restarts`
(their reload completes, then a `changed=ProcessBindings` line is missing).

**This server's disk stalls.** A live reload on a disk-backed directory was
seen waiting on the store's boot record and on a file write barrier; on a
memory-backed directory (`/dev/shm`) the same reload completes at once. Run
live daemon checks there. One hub test,
`TestInfraSSHOwnerDrainsBeforeStoreCloseOnDaemonShutdown`, is intermittent on
this host.

## Added 2026-11-17, phase 8: surfaces and doctor, written and not run

Built in the worktree `.claude/worktrees/yang-phase8` (branch
`worktree-yang-phase8`, from the laptop's `main` at `2210546edb`). Nothing was
run: the supervising session runs the owed commands. The spec's step 8 holds
the full account.

| Piece | State |
|-------|-------|
| Schema CLI | `getExternalPluginYANG` deleted. The daemon's schema commands answer an external module from the published set; with no daemon `ze schema` answers compiled-in modules and says the external module is not available, fix `ze cli` |
| Doctor codes | `doctor-plugin-schema-undeclared` and `doctor-plugin-schema-refused` declared, registered and raised by boot through `pluginserver.ReportUndeclaredSchema` and `ReportRefusedSchema` |
| Claims check | Counts the roots each running external plugin declared (`pluginserver.ExternalConfigRoots`) |
| Editor | No product change needed; a Go test proves completion from the published set |
| Drafts | `external-yang-query-failed`, `-invalid-refused`, `-clash-refused` corrected to the current texts; `external-yang-show-config` new |

What is left of phase 8, for its continuation:

| # | Item | Detail |
|---|------|--------|
| 1 | Three tracked tests no longer compile | `TestGetExternalPluginYANG`, `TestExternalPluginEmptySpec` and `TestExternalPluginNonexistent` (`internal/component/config/schema/cli/main_test.go`) call the deleted run of a program with `--yang`. Their removal needs the test judge; until then the package's tests do not build |
| 2 | The editor's functional test | `test/draft/editor/external-yang-completion.et` cannot run: the `.et` runner has no daemon and starts no plugin, so it never holds the module. It needs an `.et` option that publishes a fixture module, or a driver that types into the daemon's SSH session |
| 3 | `show schema`, `show yang` and save in the daemon | No functional test. `ze cli -c` answers these local-data commands in its own process, so the test needs a caller that dispatches through the daemon |
| 4 | AC-6 and AC-7 halves | "The internal owner of `service` keeps working" and "the config under its root stays loaded" have no functional test yet |
| 5 | A refused module after the failed start | Nothing reports it again: the daemon is not running, and a local `ze doctor` starts no plugin. Read on the refusal and in the log only. Whether a record that survives the failed start is wanted is the owner's to say |
| 6 | A hook refusal met | The edit hook refused a change to the `switch` in `Run` of the schema CLI ("switch-based command dispatch"). The switch is unchanged; the notice is written where the commands build their registry (`commandLineRegistry`) |

## Added 2026-11-17, phase 8 after its first test run

The table of leftovers above is superseded by this section. Nothing here is
committed; the supervising session runs the owed commands and commits.

What landed in the worktree, and where each piece is seen:

| Piece | Where it is observable |
|-------|------------------------|
| Schema codes after a failed start | Boot writes the refusal to the last-stop record (`RecordRefusedSchemaStop`, `RecordUndeclaredSchemaStop`). The local `ze doctor` reports it under `doctor-plugin-schema-refused` or `doctor-plugin-schema-undeclared` until ze next starts cleanly. Drafts `external-yang-refused-doctor` (green), `external-yang-undeclared-doctor` |
| Empty `--plugin` | `ze schema` refuses it with exit 1, fix `ze show plugin list` |
| `ze schema` with an external plugin and no daemon | Answers compiled-in modules, exit 0, and warns on stderr. The why now names the two commands a running ze has, typed in a `ze cli` session: `show schema list` lists the module, `show yang tree config` shows its nodes |
| The daemon's schema commands | `show schema list` and `show yang tree config` hold the external module. The daemon has no `show schema show <module>` and no command that prints a module's YANG text. Draft `test/draft/ui/external-yang-daemon-schema.ci` |
| The daemon's editor | Offers the external root and its leaves, shows a leaf's description on `?` after Tab, refuses a value outside the range with the range. Draft `test/draft/ui/external-yang-session-editor.ci`, which supersedes the `.et` draft |
| The owner of a shared container | `show dns resolver` holds the listener the config gives it beside the external augment. Draft `test/draft/plugin/external-yang-augment-owner-keeps-section.ci` |

Green in the first run: unit packages `core/diagnostic` and `plugin/doctor`;
drafts `external-yang-query-failed`, `-invalid-refused`, `-clash-refused`,
`-refused-doctor`, `-show-config`, `-changed-keeps-config`. Red and corrected,
not run again: the schema CLI package (a helper declared twice), the two UI
drafts above.

What is left:

| # | Item | Detail |
|---|------|--------|
| 1 | Run and promote the drafts | The two UI drafts, the owner-keeps-section draft and `external-yang-undeclared-doctor` after their next run; each promotion follows the stress run of the `ze-test` skill |
| 2 | AC-13, save | No functional test: a commit typed in the daemon's session, the stored config holding the external block, and a second start that parses it. The session driver (`sessionScreenAfterKeys`) can type it; the commit grammar in the daemon's window and the second start were not written |
| 3 | The vacuous exit assertion | `await=stderr` beside `expect=exit:code=` compares the peers' wait, never the daemon's exit (journal `green-that-could-not-have-been-red.md`, 2026-11-17). Drafts of `external-yang-own-root`, `-command-line` and `-augment-shared-root` add `then=stop`; each replaces a tracked test, so each needs the test judge. `external-yang-boot-accepts-what-offline-refuses.ci` holds three commands and is not converted, and five tracked tests of other work hold the same pair |
| 4 | Phase 9, measure | Spec Q10, on a host whose performance is known |
| 5 | `ze config fmt`, `dump`, `diff`, `graph` | The client route that sends these commands to a running ze, as `ze config validate` has since phase 10, is not built. Another session was working on `cmd_diff.go`, `cmd_dump.go`, `cmd_graph.go` and `config_supplied.go`; this phase did not touch them |
| 6 | A page that disagrees with the code | `docs/guide/command-reference.md`, near "Command help at the prompt", says `ze cli` with no command runs the interactive model in the client process. `runInteractiveSession` opens the daemon's session in command mode (`sshclient.RunInteractive`). Not corrected here: it is about pipe aliases, outside this phase's files |

## Added 2026-11-17: what phase 9 will find, seen on the server

The owner asked to see the start-time difference on the server. The server is a
VM on a host of unknown speed, so these are NOT figures to record or compare
across machines (`ai/META-DECISIONS.md`, "Test systems and labs"); they compare
two binaries side by side, runs interleaved, on a memory-backed directory. Time
from launching `ze start` to "Ze running.", median of 12 to 15 runs:

| Config | Before this work (`92cc99189b`) | After phase 7 | Added |
|--------|---------------------------------|---------------|-------|
| No external plugin | 429 ms | 431 ms | nothing |
| Go fixture plugin declared | 524 ms | 839 ms | about 315 ms |
| Python example declared | 500 ms | 799 ms | about 300 ms |

The claim of the spec holds: a config with no external plugin pays nothing.
With one, about 300 ms is added whatever the plugin is, so it is not the
plugin's own start moving onto the critical path. Timestamps of the plugin's
launch, its connection and the ready line show where: the plugin connects as
fast as before (about 140 ms after its launch), and the time from its
connection to ready went from about 10 ms to about 400 ms.

Confirmed by a CPU profile later that day (13 starts merged for each case, the
same host, so indicative too). With no plugin, `yang.resolveDefault` takes
about 290 ms of a start. With the Go fixture plugin, the same plus
`joinDeclaredPlugins` → `PublishExternalModules` → `Join` → `resolveJoined`,
about 275 ms: the module set is built a second time.

This paragraph first said the fix would be to read the `plugin` and
`environment` blocks with a schema of those two modules alone and build the
full schema once, after the plugins declared. Research showed that is not the
fix, for two reasons. The first build is needed anyway: readers run before the
publish in every start (the spec's A-5 names `cli.YANGCommandTree`, which
builds the YANG state before the daemon's first pass). And the join needs the
texts of every compiled-in module: `resolveJoined` parses each one again
(`sourcedModules.reloaded`) and runs a full `Resolve`, checks included, because
an external module imports and augments them. A smaller first schema would not
remove the second build. The figure on a known host is owed by the build-time
spec (closing section below).

Trap met while measuring: a script that stops its daemon with a name-wide
signal (every process named `ze`) would stop other sessions' daemons, and the
permission system refuses it. Stop the child the script started, by its pid.

## Reds that predate this work

In `./internal/le/`: `TestNoPythonLeRemains`, `TestCensusActionsDeclareHeavy`,
`TestEveryRegisteredVerbIsInTheVocabulary`, `TestNoNameEmbedsAVerbOrTest`,
`TestHelpAiVerbsRendersTheVocabulary`,
`TestNoAgentSurfaceTellsAnAgentToRunAWholeTreeVerify`. In `./cmd/ze/hub/`:
`TestBuildTag_Gate12_AbsentBinaryDropsSymbols` (an L2TP symbol in the bare
`ze_core` binary).

## Carrying work to the laptop (superseded by "The carry routine" in the last section)

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

## Added 2026-11-17, closing: phase 9 is closed, and where the optimization lives

**Phase 9 is closed.** The owner's order, 2026-11-17: "Complete in that case,
write a spec for that optimization and complete the work we have done there.
We can start that optimization in a different tree closed phase nine." The
spec's step 9 holds the indicative figures and where the cost is; its Q10 and
R-19 are answered; its header says phase 9 of 10. No figure went into
`docs/plugin-development/schema.md`, because none comes from a known host.

**The optimization lives in an existing spec, amended, not a new one:**
`plan/post-release/engineering/spec-config-yang-compiled-at-build-time.md`
(status `design`). It now owns the second schema build of a start with an
external plugin, and it gained:

| Added there | What |
|-------------|------|
| P0, AC-19 | A staged benchmark over every registered module, before any implementation |
| P1, AC-15 | `checkStructure` walks each module once and stops reading struct tags per node |
| P2, AC-16 | The join checks the external modules only, and names a failing module without a resolve per module |
| P3, AC-17 | `loadLCPDefaults` reads the resolved set; whether a start without L2TP or PPPoE reaches it is to verify |
| AC-18 | A start with one external plugin costs about one schema build, shown on the owner's Mac. This closes R-19 and Q10 |

Start that work in a tree of its own, as he ordered. P0 to P3 wait on nothing.

**One owner decision is open there (Q11) and blocks C1 to C3.** That spec
records his decision of 2026-10-16: generated Go literals per module, no blob,
no per-tag-set output, no fallback. On 2026-11-17, before the spec was found
again, he answered "3" to three ways put to him: a generator before each
build, an embedded file, a fingerprint check at start and a build from text on
a mismatch. That is a departure from his recorded decision. Q11 holds the
comparison and the supervising session's recommendation, which is to keep the
recorded design. It is his to answer; the spec stays at `design` until he does.

**What the external-plugin spec still owes** is its section "What this spec
still owes before it can close": the client route for `ze config fmt`, `dump`,
`diff` and `graph`; a functional test of save for AC-13; owner decision D-3;
the test rows still marked not run; the superseded `.et` draft; the vacuous
exit assertion in four tests; `CollectPluginYANG` and the old text road; R-15,
R-18 and R-20; the `.ci` runs of A-2 and A-4; the Architectural Verification
rows; the documentation checklist; and closure.

**Also corrected in this round:** the Abandoned-approaches entry of
`plan/learned/DESIGN-HISTORY.md` that called build-time generation rejected;
the comment in `cmd/ze/hub/infra_setup.go` that said `YANGSchema()` re-parses
on every call; and the section "Joining a module that no `init()` registered"
of `docs/architecture/config/yang-config-design.md`, which said nothing in the
start path calls the join.

## Added 2026-11-17, finishing: what was built last, what remains, and D-3

This section replaces "What the external-plugin spec still owes" above. The
owner, shown that list, answered: "Okay, do it all."

**Where.** Worktree `.claude/worktrees/yang-finish`, branch
`worktree-yang-finish`, from the laptop's `main` at `e7e5900786`. Everything
in the table is uncommitted there; the main thread commits it once the pending
runs are read.

| Piece | State | Seen |
|-------|-------|------|
| `ze config fmt`, `dump`, `diff`, `graph` ask a running ze | Built: one route shared with `validate` (`config/cli` `daemon_route.go`, `inspect_route.go`, `validate_route.go`), each through `show config supplied <sub>`, `--user` on the four | `external-yang-inspect-asks-running-ze` and `external-yang-inspect-no-daemon`, 3 of 3 each; `external-yang-validate-asks-running-ze` and `external-yang-offline-not-available` still pass |
| Save (spec AC-13) | `test/ui/external-yang-session-save.ci`: a leaf set and committed in a session reaches the plugin, and after a stop and a start on the same directory the plugin is configured with the new value | Passes |
| The `environment` block (spec R-15, a regression of phase 5 that no test covered) | Fixed: an early-started external plugin gets the block's settings for its own process (`config.EnvironmentEntries`, `env.WithDefaults`, through `HoldDeclaredPlugins`, `StartDeclared`, `StartEach`, `Process.SetConfigEnvironment`) | `external-yang-environment-block`, 3 of 3 |
| A module that adds under `plugin` or `environment` (spec R-18) | Fixed: the join refuses it (`refuseFirstPassAddition`, `yang.FirstPassRoots`). `TestJoinedAugmentReachesCompiledInContainer` now uses `system`, approved by the test judge before the edit | Unit tests of `config` and `config/yang` |
| `ze:decorate` and `ze:backend` (spec R-20) | Not built. Recorded as a limit of AC-11: journal row `gate-excludes-part-of-its-population.md` 2026-11-17, owned by the spec | - |

Unit packages green in that tree: `core/env`, `core/slogutil`, `config`,
`config/yang`, `plugin/process`, `plugin/manager`; `plugin/server` but for six
tests of other work (the five named above and
`TestUsageRendersTheDeclaredValues`). Still running when this was written, so
not known: `test/fixture`, `config/cli`, `cmd/ze/hub`, and lint.

**The spec was brought up to date in the same round** (bookkeeping only): the
test rows name tracked paths and the commit that records each run; an evidence
table sits under the acceptance criteria, which gained AC-26 to AC-29 for the
work above; the Architectural Verification table and a state table for the
documentation checklist are filled; the still-owes section lists what is done
and what is left. No external-yang draft is left under `test/draft/` in that
tree. The `.et` draft `external-yang-completion.et` is superseded by
`test/ui/external-yang-session-editor.ci`, was never promoted, is git-ignored
and exists only in the server worktree `spec-external-yang`.

**What remains before the spec can close**, in the spec's own order:

| # | Item | Whose |
|---|------|-------|
| 1 | Read the pending runs and lint, then commit the work above | Main thread |
| 2 | D-3, below | The owner |
| 3 | AC-11: decided 2026-11-17, the owner narrowed it to `ze:validate` and `ze:reference` ("make it a fix to do post-release"); the `ze:decorate` and `ze:backend` check and its journal row are `plan/post-release/features/spec-plugin-external-yang-judges-decorate-and-backend.md`. Still owed here: the functional test of the narrowed AC-11 (spec still-owes row 10) | A code agent; the main thread runs it |
| 4 | Remove `CollectPluginYANG`, `YANGSchemaWithPlugins` and `YANGValidatorWithPlugins`. The spec's "Leftover code" table says what each caller does today and what its removal takes; the tests that pass a fixture module move to `JoinExternalModules` and `SchemaOf`, under the test judge | A code agent |
| 5 | `external-yang-boot-accepts-what-offline-refuses.ci` still pairs `await=stderr` with an exit assertion that proves nothing | A code agent and the test judge |
| 6 | Pages with no statement of this work found by search: `features/plugins.md`, `docs/guide/plugins.md`, `docs/functional-tests.md`, `docs/architecture/plugin/plugin-system.md`, `ai/digests/hub-engine.md`, `docs/plugin-development/README.md`; then `./le spec citation anchors` and `./le doc check all` | A prose agent; the main thread runs the checks |
| 7 | Journal rows: `green-that-could-not-have-been-red.md` 2026-11-17 needs an owner that outlives the spec (the runner still accepts the pair, and five tests of other work hold it); `unwired-feature.md` 2026-08-13 is marked `fixed:` at closure | Closure |
| 8 | Runs with no record: `cmd/ze/hub` `schemaless_plugins_test.go`; the publish and keeper tests of `cli/client`, `cli/testing`, `gnmi`, `iface`, `firewall`, `traffic` | Main thread |
| 9 | `/ze-close` | A closing agent |

One deliverable of the spec does not hold: `reg.PluginSchema` is written by
`registrationFromRPC` and read by no non-test code. Its reader is the command
argument model, so it follows D-3.

**D-3, the one decision that blocks closure.** Is the argument model of a
plugin's own commands part of this spec?

| Answer | What it changes |
|--------|-----------------|
| No | No existing spec can take it (searched `plan/`; the nearest, `plan/release/fixes/spec-declared-commands-without-leaves.md`, is a release fix for commands compiled into `ze`). A new spec in `plan/post-release/features/` takes the item, the journal row `unwired-feature.md` 2026-11-02 and the missing reader of `reg.PluginSchema`. Closure proceeds once items 1 and 3 to 8 are done |
| Yes | Phase 11, with a design to put to the owner first: one declaration carries a config module and a command module, a change to the published `SchemaDecl`. New acceptance criteria and functional tests. The spec stays in progress until it is built |

## Phase 11, approved 2026-11-17

This section replaces the D-3 item of the list above: the owner answered it,
and the answer was "Yes".

**Where the code is.** The laptop's `main` holds `81567bab05` and
`f98ca55702`. The server branch `worktree-yang-finish` (worktree
`.claude/worktrees/yang-finish`) has its tip at `c40ca5fb09`. Implementation
continues from this handover; the spec's phase 11 under Implementation Steps
is the plan.

**The owner's words, verbatim.**

| When | What he said |
|------|--------------|
| On D-3 | "Yes make the plugin define its command. You can change the API and interface between the plugin and ze if it must or helps" |
| Shown a five-point design | "The yang should already allow to define command ?? or I thought it did" |
| Shown what lifting the two blockers needs, and two optional improvements to leave out | "lift these two then :-)" |

**What reshaped the design.** His second sentence is correct. A compiled-in
plugin defines its commands in a `-cmd` module, one `ze:command` node per
command and one leaf per argument. Help, completion and argument checking
come from there: `BuildCommandTree`, `rebuildCommandArgDefs` inside
`yang.PublishExternalModules`, and `command.ValidateModelArgs` before
`routeToProcess`. Nothing new is invented. Two things alone stop an external
plugin from using it: the join refuses a module that is not `-conf` and
augments nothing (`refuseIdleModules`), and the Stage 1 declaration carries
one YANG text.

**The approved scope, in order.**

| Part | What | Why |
|------|------|-----|
| First | Settle R-24 and R-25 at the code: whether command nodes joined after the first tree build are checked (`BuildCommandTree`'s `validateOnce`), and which tree an SSH session completes from. Run the wire census again | The research could not verify either |
| 11.1 | The Stage 1 `schema` becomes a list of modules: wire, SDK, engine YANG, the Python library, every in-tree plugin and fixture | A plugin brings a config module and a command module |
| 11.2 | The join accepts a `-cmd` module, and refuses one whose command path the tree already holds | It reaches the existing command tree safely |
| 11.3 | The restart check covers every module of the list; a command the plugin declares names a node of its command module | The owner's restart-only ruling |
| 11.4 | `ze <verb> ...` from the shell asks a running Ze for a path the local tree does not hold; with none running, "not available", fix `ze start` | Today the local verb tree answers "unknown command" even when Ze runs; the owner's no-daemon ruling |
| 11.5 | Pages and example: five departures the approval covers (the spec's Documentation Update Checklist), the Python example's command with an argument | The pages say a plugin brings one module, and that a handler checks its own arguments |

Acceptance criteria AC-30 to AC-40, each with a planned functional test
`test/plugin/external-yang-command-*.ci` through the fixture plugin or the
Python example.

**Left out by the same approval, wanted later and not declined** (the spec's
Work Not Done): removing help text and arguments from the Stage 1 command
declaration so the YANG is the only source (54 Go files; until then, where
both exist, the YANG wins); passing arguments to the plugin's handler by name;
API parameters derived from a command node's leaves when it has no `-api` rpc.
No existing spec fits the first two; the third's nearest home is part D of
`plan/release/fixes/spec-yang-single-source-of-truth.md`.

**Open point for the main session to raise with the owner (spec P-1).** An
external command declared in a `-cmd` module, with no `-api` rpc, shows no
parameters on MCP, REST or gRPC: `buildParamMeta` (`cmd/ze/hub/command_meta.go`)
reads rpc input only. The answers are (a) the plugin also brings an `-api`
module, declaring each argument twice; (b) `buildParamMeta` reads the command
node's leaves, which changes compiled-in commands with no rpc too; (c) leave
it, and the pages say so. Each changes only AC-36's surfaces and the pages.
It does not hold parts 11.1 to 11.4.

**Two later rulings, same day (2026-11-17).** They supersede the open point
above.

| Ruling | His words | What follows |
|--------|-----------|--------------|
| P-1, API parameters | "yes, like current internal plugin, the exact same way." | An external plugin may bring an `-api` module in its list, its command nodes point at its rpcs with `ze:rpc`, and the pair is judged as a compiled-in pair is. `buildParamMeta` reads it unchanged. Nothing is derived from leaves. New AC-41, test `external-yang-command-api-parameters.ci` (planned). The Work Not Done item about deriving parameters from leaves is dropped for external plugins; the general question stays in part D of `spec-yang-single-source-of-truth.md` |
| The governing rule | "The only differene should be how they reach the bus" | A compiled-in and an external plugin are the same thing to Ze and differ only in transport. A rule for external plugins alone needs a transport reason (Stage 1, fixed until restart, nothing to ask with no `ze` running) or an arrival reason (a name that already exists). Any other difference is a defect. Written into `docs/architecture/plugin/plugin-system.md`, "One plugin, two ways to reach the bus", with one line in `ai/doors/plugin-infra.md`. Every phase 11 step is tested against it |

The spec's table "To re-examine under the transport-only rule" (under Key
Design Decisions) lists six rows that have no transport reason: the
top-level redefinition refusal, the idle-module refusal, the stricter
`ze:validate` and `ze:reference` verdict, the AC-40 Stage 1 node check, the
"YANG wins" help rule, and the command path clash. Each is settled by the
rule or brought to the owner before closure.

## State at 2026-11-17, end of the second part of phase 11 (superseded by the last section)

This section replaced the lists above for what came next on 2026-11-17. Its
"What remains" table and its "being written" and "being built" items are
done: see the last section. The spec's
"What this spec still owes before it can close" is the full record; this is
the summary for the next session.

**Commits.** Server branch `worktree-yang-finish`, worktree
`.claude/worktrees/yang-finish`. The laptop twins are as the supervising
session reported them; they cannot be read from the server.

| Server | Laptop `main` | What |
|--------|---------------|------|
| `a529b75fd9` | `81567bab05` | `ze config fmt`, `dump`, `diff`, `graph` ask a running ze; the save test; R-15 and R-18 |
| `c40ca5fb09` | `f98ca55702` | The command-line text road kept; AC-11 narrowed; the boot test split |
| `adc03a69ac` | `af22d6f7be` | Phase 11, first step: `schemas` list, strict decoder `rpc.DecodeDeclaration`, the join takes external `-cmd` and `-api` modules |
| `6f127569c1` | `c370011e55` | Phase 11, second part: Stage 1 check `command-nodes-served`, shell form, restart and clash refusals, `moduleUseOf`, the examples, the pages |

**What is built and proven** (three runs each on a real daemon, per the
commit bodies): `external-yang-command-argument`, `-schema-key-refused`,
`-command-shell-running`, `-command-shell-no-daemon`,
`-command-restart-changed`, `-command-path-clash`, `-command-python`. AC-32 to
AC-37 are proven, AC-38 for `| json`, AC-39 for its argument part. AC-40 is
narrowed to the compiled-in rule (every node a plugin brings is reached by a
command it declares) and carried at unit level. AC-35 is corrected: a plugin
restarted with a changed module stays down and its commands answer "unknown
command" until ze restarts.

**What remains.**

| Item | State |
|------|-------|
| Tests of help and completion in a session (AC-30, AC-31), `\| count` (AC-38), the MCP, REST and gRPC parameters (AC-41), and `external-yang-unheld-registration-refused.ci` (AC-11) | Being written by another agent |
| AC-42 to AC-45, the four decisions below | Being built by three agents |
| A build of the Go example (no gate compiles it) | Owed |
| The registration's `PluginSchemas` field, written and never read | Delete it or give it a reader |
| The uncached schema rebuild on the text road; the six pages of still-owes row 13 | Owed |
| An owner for journal row `green-that-could-not-have-been-red.md` 2026-11-17 that outlives the spec | Owed before commit B |
| The Review Gate in an independent context, then `/ze-close` | After all of the above |

**The four decisions.** Put to the owner with the supervising session's
recommendations: remove `pkg/plugin/plugin.go`; remove the six empty
compiled-in modules in this phase; refuse a redefined top-level node on both
sides and convert the six `service` definers to `augment`; refuse an unheld
`ze:validate` or `ze:reference` on both sides. His answer, verbatim: "It
seems you have all the answers." The supervising session reads that as
approval of the four, told him so, and proceeds; it is not an explicit
approval of each. The spec's Key Design Decisions, "Decided 2026-11-17",
holds the table and the stop conditions given to the implementing agents.

**The redefinition rule moved to its own spec (D-6, AC-44).** The agent
building it found five shared top-level roots (`environment`, `system`,
`interface`, `l2tp`, `service`) across about 26 compiled-in modules, not one,
and a shipped defect: with `ze_cos` built, the schema keeps the CoS copy of the
interface lists and drops the interface module's own list declarations
(journal `silent-fall-through.md` 2026-11-17, read and not run). It stopped
before any edit. Shown the facts, the owner answered "ok" to the
recommendation: a spec of its own, written now as a design, with the defect
as its first acceptance criterion. It is
`plan/release/fixes/spec-config-yang-one-definer-per-node.md` (P1, `design`;
open decisions O-1 to O-4 for his design review). This spec keeps
`refuseRedefinedRoots` for external modules as an arrival rule, and AC-44 is
marked moved, not met.

**Journal.** The new row `error-names-no-fix-command.md` 2026-11-17 (the
missing-argument refusal) is owned by
`plan/release/fixes/spec-web-cli-config-one-off-defects.md`, AC-31 there.

**A lead, not a defect record.** The ExaBGP bridge's Ze-facing startup
(`internal/exabgp/bridge/bridge.go`, `startupProtocol`, `sendDeclarations`)
appears to write the retired text declaration lines, and no daemon reader of
them was found while `pkg/plugin/plugin.go` was examined. Nobody investigated
it: whoever next works on the bridge reads what the daemon does with those
lines before anything else.

**The carry routine, as it works now.**

1. On the server, write an apply script for the commit; the laptop pulls and
   runs it on its working tree.
2. When a file the commit touches holds another session's adjacent
   uncommitted edit on the laptop, the apply refuses that file. The file is
   then merged by hand on the server into a `resolved` copy, and the apply uses
   it only if the laptop's copy is still unchanged.
3. The test-judge approvals and the commit preparation (`./le commit create`)
   run in the owner's shell, because approvals are keyed to the shell session.
4. The owner runs the printed script and signs.

**Traps met today.**

| Trap | What to do |
|------|------------|
| A bare `go test` lacks the build tags | `./le test unit pkg path <dir>` |
| Lint refuses when its inputs change during the run | Run it when no agent is editing the files it reads |
| Agents stop at 100 tool calls | Plan each package so a continuation can take it over |
| The Go edit hook refuses until the style guide and the doors are read in the session | Name them in every code agent's brief, from the main checkout |

## State at the end of the session, 2026-11-18

This section is the current state. A fresh session can act on it alone, with
the spec. Every statement is read from `git log`, the commit bodies and the
tree on the server, except the laptop facts, which the supervising session
reported.

### Where everything is

| Item | Value |
|------|-------|
| The work | All on the laptop's `main`. The last carry is `7504b8c5d2`: one commit holding the server commits `9e0949eadb`, `ca5d2d0588` (plan), `6448a74128` (external commands on every surface, one rule: 84 files, 18 of them deleted) and `0dc2d4f173` (the regenerated provider list). Earlier carries: `a529b75fd9` as `81567bab05`, `c40ca5fb09` as `f98ca55702`, `adc03a69ac` as `af22d6f7be`, `6f127569c1` as `c370011e55` |
| Checked | The supervising session compared every file this work changed since its base `e7e5900786` (213 files) with the laptop's committed `main`: 202 identical, 11 differing only by other sessions' later commits, and in each of the 11 every line this work added is present |
| Server | Nothing uncommitted. The branch `worktree-yang-finish` (worktree `.claude/worktrees/yang-finish`) and the older branches `worktree-spec-external-yang`, `worktree-validate-client` and `worktree-yang-phase8` are redundant copies. Nothing on them is needed. Removing them is the owner's call |
| Spec | `plan/post-release/features/spec-plugin-external-yang-at-startup.md`, `in-progress`, phases 1 to 11 built. Its "What remains before closure, in order" is the plan, and its gap table G-1 to G-11 names what closure will check that is not in place |
| Claim | Held by session `99a3a802-6ea0-45ad-a7f7-a342c1f9baae` in the server worktree. The supervising session releases it when it ends. The next session claims the spec where it works: `./le spec claim plan/post-release/features/spec-plugin-external-yang-at-startup.md` |

### What is built

Every acceptance criterion the spec still owns is built; the spec's evidence
tables name each carrier and the commit that records its runs.

| Criteria | State |
|----------|-------|
| AC-1 to AC-29 | Proven, some at unit level by design (the spec's table under Acceptance Criteria) |
| AC-30, help | The YANG help wins over the Stage 1 help, for every plugin (`external-yang-command-help.ci`). **Not met in one point:** no session surface shows an argument leaf's own `ze:help`, for any command, compiled-in included (`TreeCompleter.matchChildren`). It needs a design of its own and no spec holds it |
| AC-31, AC-38 | Completion and pipes proven (`-completion.ci`, `-pipes.ci`). `\| count` is refused on a one-document answer exactly as on a compiled-in command with the same shape; `\| text \| count` works on both |
| AC-41 | MCP and REST proven (`-api-parameters.ci`); gRPC has no functional harness and is carried by unit tests of the shared source. Product fix: a command only a plugin serves was listed with no parameters (`buildCommandMeta`) |
| AC-11, AC-45 | An unheld `ze:validate` refuses the start, external (`external-yang-unheld-registration-refused.ci`) and compiled-in (`refuseWronglyBuilt`, doctor code `doctor-build-registration-unheld`). No shipped build-tag combination can hold an unheld name. The compiled-in refusal sits at daemon start only, by the supervising session's choice |
| AC-42, AC-43 | `pkg/plugin/plugin.go` removed; the six empty modules removed; an empty module refused for an external plugin too; `TestNoShippedModuleAddsNothing` green |
| AC-44 | Moved to `plan/release/fixes/spec-config-yang-one-definer-per-node.md` (`design`) |
| AC-46, AC-47 | Found and fixed outside the subject: a command sent over SSH in one shot lost its quotes (`execMiddleware`, `exec_quotes_test.go`); `ze plugin <name> --yang` for a plugin with no YANG now says so |

The owner's decisions of 2026-11-17, answered "It seems you have all the
answers.": D-5a (remove `pkg/plugin/plugin.go`) was later confirmed
explicitly, after he was shown the 33 deleted tests of the dead type: "it iz
ok, i was only unclear". D-5b (the six empty modules) and D-7 (unheld names)
still rest on the supervising session's reading of his first answer. D-6
moved to its own spec by his "ok".

The independent test judge ruled on every existing test this work changed or
deleted: 38 `Weakening-approved:` trailers on `6448a74128`, and a
`Test-judge:` paragraph for the snapshot on both commits. One process lapse:
two test changes, the renamed join test and the deletion of the dead type's
test file, reached the commit gate without a prior ruling; the gate refused,
and the judge ruled before the commit.

### Next steps, in order

| # | Step | Command or skill |
|---|------|------------------|
| 1 | Work on the laptop's `main`, where the work is. Check the checkout is not behind its upstream before the first edit | `git fetch`, then `git rev-list --count HEAD..@{u}` |
| 2 | Claim the spec | `./le spec claim plan/post-release/features/spec-plugin-external-yang-at-startup.md` |
| 3 | Check the change set the review must cover. The review artifact covers the commits that carry `Spec-work: plugin-external-yang-at-startup`; whether the laptop's carry commits (`7504b8c5d2` and the earlier four) carry that trailer was not read from the server | `git log -1 --format=%B 7504b8c5d2` |
| 4 | The Review Gate on the whole spec, by an independent agent, round 1 over the whole change set with at least two lenses, its scope written into the spec's Review Gate section first | `/ze-review` with `/ze-review-spec` in agents, or `/ze-review-deep` in the main thread (`ai/rules/planning.md`) |
| 5 | The fixes the review asks for, each with its scoped tests run by the main thread, then a review of only the fixes, until a round finds no BLOCKER and no ISSUE | `/ze-implement` discipline for each fix; `./le test unit pkg path <dir>`; `./le go lint run scope "<pkg>"` |
| 6 | The gaps G-1 to G-11 of the spec: decide each as a fix before closure or a Work Not Done row in an existing spec, and ask the owner whether a new spec for the argument leaf help runs | The spec, "What remains before closure, in order" |
| 7 | Closure: the closure template, the news fragment, commits A and B from one script | `/ze-close` in one agent |

### Open items and their homes

| Item | Home |
|------|------|
| An argument leaf's own `ze:help` in a session (AC-30) | **None yet.** A new spec is needed; the spec's Work Not Done row and still-owes row 22 say so |
| The refusal of a missing argument names no fix, for every command | `plan/release/fixes/spec-web-cli-config-one-off-defects.md`, AC-31 there |
| The compiled-in redefinition rule and the CoS-over-interface defect | `plan/release/fixes/spec-config-yang-one-definer-per-node.md` |
| `ze:decorate` and `ze:backend` names of an external module | `plan/post-release/features/spec-plugin-external-yang-judges-decorate-and-backend.md` |
| The second schema build of a start with an external plugin, and its figure | `plan/post-release/engineering/spec-config-yang-compiled-at-build-time.md` |
| `PluginSchemas` on the registration, written and never read; the uncached schema build of a command-line selection; the Go example built by no gate; AC-39's help and shell parts on the Python example; the six pages of the documentation checklist | This spec, gaps G-3 to G-5 |
| Journal row `green-that-could-not-have-been-red.md` 2026-11-17 | This spec, until an owner that outlives it is found (gap G-8) |

### Leads, not defects

Nobody investigated these. Each goes to whoever next works on its area.

| Lead | What is known |
|------|---------------|
| The ExaBGP bridge's Ze-facing startup | `internal/exabgp/bridge/bridge.go` (`startupProtocol`, `sendDeclarations`) still writes the retired text declaration lines, which no daemon reader was found for |
| A stale citation on the website | `website/compare/nos.md` cites a line range of the removed module `ze-cli-log-cmd.yang`; it should cite `internal/plugins/log/yang/ze-log-cmd.yang` |
| Push debt on the laptop | The commit tool printed `debt=3 person act(s) owed` at the last preparation there. The supervising session could not establish what the three acts are; it concerns a push |
| Another session's work on the laptop | Uncommitted work in `internal/component/config/cli`: a new `register_artifact_maintenance.go` and about a dozen modified files, `cmd_validate.go` and `config_data.go` among them, which this work changed earlier. Its tree did not build at that moment |

### The carry routine

Needed only if work is done on the server again. It supersedes "Carrying
work to the laptop" above.

1. On the server, commit on a branch of a worktree, and write an apply script
   for the commit; the laptop pulls the script and runs it on its working tree.
2. When a file the commit touches holds another session's adjacent
   uncommitted edit on the laptop, the apply refuses that file. Merge it by
   hand on the server into a `resolved` copy; the apply uses it only if the
   laptop's copy is still unchanged.
3. The test-judge approvals and the commit preparation (`./le commit create`)
   run in the owner's shell, because approvals are keyed to the shell session.
4. The owner runs the printed script and signs. Then compare every changed
   file with the laptop's committed `main`, as was done for `7504b8c5d2`.

### Traps of the session

| Trap | What to do |
|------|------------|
| The commit gate refuses a test change that has no prior ruling of the test judge | Run `/ze-test-judge` on every changed or deleted existing test before the commit is prepared |
| A generator reads the disk, so the commit that deletes files cannot carry the generators' output | Write the generated files by hand to the generators' output in that commit, and carry the generators' own output in the next, as `6448a74128` and `0dc2d4f173` did |
| A snapshot test refreshed to turn a red green is a weakening | Regenerate it only by the test's own `-update` switch, with the judge's ruling on the exact difference |
| A bare `go test` lacks the build tags | `./le test unit pkg path <dir>` |
| Lint refuses when its inputs change during the run | Run it when no agent is editing the files it reads |
| Agents stop at 100 tool calls | Plan each package so a continuation can take it over |
| An edit is refused until the style guide and the doors are read in the session | Name them in every agent's brief, from the main checkout |
| `-pattern` of the functional runner matches a test's path | Use a full test name or the numeric ids from `-list` |
| This server's disk stalls under a live reload | Run live daemon checks on a memory-backed directory (`/dev/shm`) |
