# Spec: fixit-ci-runner-cannot-test-stdin

| Field | Value |
|-------|-------|
| Status | design |
| Scope | tooling |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-09-06 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

**Bucket: `plan/pre-release/`.** No operator meets this: the `ze` binary reads
standard input correctly, and the defect is in the instrument that judges it. It
is not `plan/` either, because the first release's audit reads the functional
suite's greens, and one whole directive family (`stdin=`) means something other
than what `docs/architecture/testing/ci-format.md` says it means. If Thomas reads
the bucket test as strictly "the release cannot go out until it is done", this
moves to `plan/` and nothing else in the spec changes.

## Task

No `.ci` test can exercise a `ze` command that reads standard input.

A `.ci` author writes `cmd=foreground:seq=1:exec=ze bgp decode pcap -:stdin=capture`
and reads it as "pipe the `capture` block into `ze bgp decode pcap -`". The runner
does something else: it writes the block to a file in the work directory,
substitutes that path for the `-` in argv, and pipes nothing. The command that
runs is the PATH form. Nothing in the output says the line was rewritten.

Two measured consequences, both from the `bgp-pcap-decode` work on 2026-09-06 and
recorded in `plan/journal/green-that-could-not-have-been-red.md` row 160:

| `.ci` | What happened | Why |
|-------|---------------|-----|
| `test/ui/bgp-decode-pcap-stdin.ci` | PASSED, vacuously | `decodePcapInput` opens a path just as happily as stdin, so the assertion held for a reason the test was not written to check. It could not have gone red for the stdin path being broken |
| `test/ui/bgp-decode-stdin-hex.ci` | FAILED, `invalid byte: U+002F '/'` | `cmdDecode` fell through to `decodeHexPacket`, which was handed a filesystem path where hexadecimal was expected |

Neither file was committed. Both are named here as evidence, not as work.

The substitution is not an accident. Its comment says a bare `ze -` daemon launch
needs the config as a FILE, and that is true: the daemon re-reads it on SIGHUP, a
fixture rewrites it by bare name, and a rollback test asserts on it. So this spec
is not "delete the branch". It is "separate the case where `-` means a config file
the runner materializes from the case where `-` means the pipe", and then answer
the harder question underneath: how a `.ci` that silently tests the wrong thing is
DETECTED rather than relied on not to happen.

That question is the reason this spec exists rather than a one-line fix. Three
mechanisms in this runner have been found in one week that let a `.ci` assert
nothing, and a fourth was found writing this spec. All four have one cause: the
runner ANSWERS a directive it did not understand, instead of refusing it.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/testing/ci-format.md` - the `.ci` directive vocabulary this spec changes
  → Constraint: the doc states, under "Stdin Blocks", that a block is "piped to a process's stdin". For `ze <verb> ... -` and for every `ze-peer` line that is false, and the doc says nothing about the file substitution. The doc edit lands in the same work as the code (`ai/rules/documentation.md`), and it must state which form pipes and which does not
  → Decision: the `cmd=` line's key vocabulary is `seq`, `exec`, `stdin`, `timeout`, `exit`, `name`, `signal`. Any new directive this spec adds is one more marker in that set and must be documented in the same table
- [ ] `docs/functional-tests.md` - the suite-by-suite map, already amended for this defect
  → Constraint: the UI row already records "No `.ci` covers the standard-input forms of `ze bgp decode`, and none can". That sentence becomes false when this spec lands and must be rewritten, not appended to
- [ ] `ai/rules/principles.md` - fail-closed, one declaration, registration over enumeration
  → Constraint: "code that cannot answer MUST say so, and MUST NOT return zero, nil, false, empty, or the default in place of an answer". A parser that meets a directive it cannot honor is exactly this case, so a silent fallback is banned and a refusal is owed
  → Constraint: "a new feature MUST register itself and be discovered; it MUST NOT require an edit to a switch, a case, a factory, a field list, or any other central enumeration". This rules OUT Option A below, which is a hand-kept list of `ze` commands inside the runner
- [ ] `ai/rules/testing.md` - what a functional test owes
  → Constraint: a `.ci` proves the user can REACH the feature. A `.ci` that reaches a different feature than the one it names is worse than no `.ci`, because it occupies the coverage slot
- [ ] `ai/rules/cli.md` - the `-` convention
  → Decision: `-` means stdin when reading and stdout when writing, on every command that takes a path. The runner is the only place in the repository where `-` means something else

### RFC Summaries (Scope: protocol)
Not applicable. No wire behavior changes.

**Key insights:** (minimal context to resume after compaction)
- The rewrite lives in one branch of `runTest`'s command loop in `internal/test/runner/runner_exec.go`, guarded by `binName == binNameZe && stdinContent != nil`, and it ends with `stdinContent = nil` so nothing is piped.
- `zeDaemonConfigArgIndex` (`internal/test/runner/runner_exec_util.go`) already tells a daemon-config `-` apart from any other `-`. It returns the argv index for `ze -` and `-1` for `ze bgp decode -`. The discriminator the fix needs already exists and is already tested.
- `ze-peer` is worse and it is a separate branch: it appends a temp file unconditionally, with or without a `-`, so no `ze-peer` line can pipe at all.
- Four things pipe correctly today, all by accident: `ze-test <verb> -` (the branch matches only `binNameZe`), `ze isis decode` and `ze ospf decode` (they read `os.Stdin` directly and take no `-`), and `ze cli -c "show config dump - | json"` (the `-` sits inside a quoted argument, so no argv element equals `-`).
- A sibling session landed `checkKeys` (`internal/test/runner/record_parse_keys.go`), the `!contains` to `reject=stdout:contains=` move, and `checkOutputAssertions` (`internal/test/runner/runner_output_assert.go`) as commit `8c7f0a5bf2` on 2026-09-06, while this spec was being written. The package is clean at that commit. Line numbers in it moved, so cite by symbol.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/test/runner/runner_exec.go` - the command loop. Resolves `binName`, loads `stdinContent` from `rec.StdinBlocks`, then runs two rewrite branches before `exec.CommandContext`. `proc.Stdin` is set only when `stdinContent` is still non-nil
- [ ] `internal/test/runner/runner_exec_util.go` - `zeDaemonConfigArgIndex` returns the argv index of a daemon config argument, skipping the known daemon flags, and `-1` when the first non-flag word is not a config path. `zeDaemonShouldForceFileStorage` reads it too
- [ ] `internal/test/runner/runner_config.go` - `zeConfigFileName` names the substituted file: `ze-bgp.conf` for the first stdin block in a record, `ze-<block>.conf` for each additional one, and the same name again when a block is reused
- [ ] `internal/test/runner/record_parse_cmd.go` - `parseCmdExec` parses the `cmd=` line by MARKER, not by key
- [ ] `internal/test/runner/record_parse.go` - `parseExpect`, `parseReject`, `nextMarker`
- [ ] `internal/test/runner/record_parse_keys.go` - `checkKeys` and `retiredKeys` (read while untracked; landed as `8c7f0a5bf2`)
- [ ] `internal/test/runner/runner_output_assert.go` - `checkOutputAssertions` and `Record.recordStep` (read while untracked; landed as `8c7f0a5bf2`)
- [ ] `internal/test/runner/decoding.go` - `parseCIFile` and `parseDecodeCmdLine`, the `test/decode/` driver
- [ ] `internal/test/runner/accept_only.go` - `isAcceptOnly` and the `test/.accept-only-baseline` ratchet, the only existing detector for a `.ci` that asserts nothing
- [ ] `internal/test/tmpfs/tmpfs.go` - `parseStdinBlock`. A `hex=` block decodes to RAW BYTES, a `text=` block appends a newline, a terminator block is taken verbatim
- [ ] `internal/core/cliio/cliio.go` - `IsStdin`, `ReadFile`, `OpenReader`, `Create`, `WriteFile`, `SwapStreams`, and the `stdinClaimed` fail-closed guard
- [ ] `internal/component/bgp/cli/decode.go` - `cmdDecode` routes `pcap`, then `-`, then a bare hex word
- [ ] `internal/component/bgp/cli/decode_pcap.go` - `decodePcapInput` opens `args[0]` through `cliio.OpenReader`
- [ ] `internal/component/config/cli/editor_stdin.go` - `openEditableConfig` routes the SAVE to stdout for `-`
- [ ] `internal/component/config/cli/cmd_fmt.go` - `-w -` writes the formatted config to stdout, always; a real path is rewritten in place only when it changed
- [ ] `internal/component/config/cli/config_data.go` - `dataHistory` REFUSES `-` with a named error, because history needs on-disk revisions
- [ ] `docs/architecture/testing/ci-format.md` - the published contract for `stdin=`
- [ ] `docs/functional-tests.md` - the suite map and the note already added about this defect

**What the runner does today, by producing function:**

| # | Producer | Behavior |
|---|----------|----------|
| 1 | the command loop in `runner_exec.go`, branch `binName == binNameZePeer && stdinContent != nil` | writes the block to `os.CreateTemp` as `ze-peer-expect-*.msg`, APPENDS the path to argv, sets `stdinContent = nil`. Unconditional: it does not look at argv, so a `-` the author wrote survives as a second positional argument |
| 2 | the command loop in `runner_exec.go`, branch `binName == binNameZe && stdinContent != nil` | scans argv for the FIRST element equal to `-`, writes the block to `rec.WorkDir` under `zeConfigFileName`, and either inserts the `start` verb before it (when the index equals `zeDaemonConfigArgIndex`) or replaces it in place. Then `stdinContent = nil` and `break` |
| 3 | the same loop, after both branches | `proc.Stdin` is assigned only when `stdinContent` is still non-nil. So a matched `-` means nothing is piped |
| 4 | `zeDaemonConfigArgIndex` | for `ze -` returns 0; for `ze bgp decode pcap -` returns `-1`, because `bgp` is not a skipped flag, is not `-`, has no config suffix, holds no path separator and does not start with `.` |
| 5 | `parseCmdExec` | extracts each value from its marker to the next KNOWN marker. An unknown `key=` on a `cmd=` line is not refused: it is swallowed into whichever value precedes it |
| 6 | `parseCIFile` and `parseDecodeCmdLine` (`decoding.go`) | for `test/decode/*.ci` the `exec=` line is never executed. A bespoke parser lifts the message type, family, plugins and `--json` off it, takes the hex from the `stdin=` block as TEXT, and builds its own argv ending in the hex string. The `-` on those lines is decoration |
| 7 | `checkKeys` (`record_parse_keys.go`, landed `8c7f0a5bf2`) | refuses an unknown key inside `expect=` and `reject=`. It is not called from `parseCmdExec`, which is marker-based and has no key map to check |
| 8 | `checkOutputAssertions` (`runner_output_assert.go`, landed `8c7f0a5bf2`) | every stream assertion reads `rec.ClientOutput`, the accumulated stdout AND stderr of every command in the file. The stream named in the directive selects nothing and the command it appears near selects nothing. Documented in that file's comment, not fixed |

**What a `.ci` author writes today, and what they get:**

| The author writes | The author believes | What runs | Verdict |
|-------------------|--------------------|-----------|---------|
| `exec=ze bgp decode pcap -:stdin=capture` | the capture is piped | `ze bgp decode pcap <workdir>/ze-bgp.conf` | passes, proving the path form works |
| `exec=ze bgp decode -:stdin=payload` | the hex lines are piped | `ze bgp decode <workdir>/ze-bgp.conf` | fails, `invalid byte '/'` |
| `exec=ze config fmt -w -:stdin=config` | the formatted config is printed | the temp file is rewritten in place | passes with empty stdout, and the pipeline branch is never entered |
| `exec=ze config history -:stdin=config` | the named refusal is asserted | history runs against a real path | the refusal is unreachable, so the negative test cannot exist |
| `exec=ze-peer --port $PORT -:stdin=peer` | the script is piped | `ze-peer --port $PORT - <tmpfile>`, two positional arguments | unvalidated: I did not run it. `expect.go` opens one path, so the second word is either an argument error or ignored |
| `exec=ze-test engine-steps -:stdin=steps` | the steps are piped | exactly that, stdin piped | works, because neither branch matches `ze-test` |
| `exec=ze isis decode:stdin=payload` | the payload is piped | exactly that, stdin piped | works, because there is no `-` in argv to match |
| `exec=ze cli -c "show config dump - \| json":stdin=config` | the config is piped | exactly that, stdin piped | works, because no argv ELEMENT equals `-` |

**Every `ze` and `ze-peer` command that reads standard input, and what its `-` means:**

Group 1, where `-` means "the config file the runner creates for me". One command,
and the file is load-bearing: the daemon re-reads it on SIGHUP, `action=rewrite:dest=ze-bgp.conf`
addresses it by bare name, a restart reuses it, and a rollback assertion reads
`rollback/ze-bgp-*.conf` beside it.

| Command as typed in a `.ci` | Producer | `.ci` lines today |
|-----------------------------|----------|-------------------|
| `ze -`, and its flagged forms `ze --plugin X -`, `ze --mcp $PORT -`, `ze --web ... -` | `zeDaemonConfigArgIndex` returns the `-`'s own index, so the runner writes the file and inserts the `start` verb. The daemon then loads it through `internal/component/bgp/config/loader.go` | 747 `ze -`, plus 26 `ze --mcp $PORT -`, plus roughly 120 `ze --plugin ... -` |

Group 2, where `-` means "read from the pipe" and the runner substitutes a path
anyway. Every row is a `ze` command whose stdin form no `.ci` can reach.

| Command | Producer | What `-` means there |
|---------|----------|---------------------|
| `ze bgp decode -` | `cmdDecode` routes to `decodeHexStdin` | hexadecimal lines on stdin |
| `ze bgp decode pcap -` | `decodePcapInput` opens `args[0]` via `cliio.OpenReader` | capture bytes on stdin, streamed |
| `ze config validate -` | `cmd_validate.go` reads through `cliio.ReadFile` | the config text on stdin. 423 `.ci` lines, all testing the path form |
| `ze config fmt -`, `ze config fmt -w -`, `--check`, `--diff` | `cmd_fmt.go` | the config on stdin, AND for `-w` a different output sink: stdout unconditionally, rather than an in-place rewrite when changed |
| `ze config set - ...`, `ze config deactivate - ...`, `ze config activate - ...` | `openEditableConfig` (`editor_stdin.go`) | read from stdin and route the SAVE to stdout, turning the command into a pipeline stage |
| `ze config show -`, `ze config dump -`, `ze config graph -`, `ze config fix -`, `ze config completion -`, `ze config import -` | `cmd_show.go`, `cmd_dump.go`, `cmd_graph.go`, `cmd_fix.go`, `cmd_completion.go`, `cmd_import.go` | the config text on stdin. Content-equivalent to the path form, so these are the vacuous-green cases |
| `ze config diff <a> -` | `cmd_diff.go` | one of the two sides on stdin |
| `ze config migrate -`, and its `--output -` | `cmd_migrate.go` | input on stdin, output on stdout |
| `ze config edit -` | `cmd_edit.go` | the config on stdin |
| `ze config rollback <n> -` | `cmd_rollback.go` | the config on stdin |
| `ze config history -` | `dataHistory` (`config_data.go`) | a REFUSAL: "history needs on-disk revision history". The one negative assertion in this group, and it is unreachable |
| `ze doctor <path or ->` | `loadDoctorConfig` (`doctor.go`) | the config to check, on stdin |
| `ze support ... <path or ->` | `support.go`, `opts.ConfigPath` | the config to bundle, on stdin |
| `ze plugin test <config or ->` | `test_cmd.go` | the plugin test config on stdin |
| `ze data <key> <src or ->` | `internal/component/config/storage/cli/main.go` | the blob to store, on stdin |
| `ze tacacs show <path or ->` | `internal/component/tacacs/cli/main.go` | the config on stdin |
| `ze exabgp migrate <config or ->`, `--env <file or ->` | `internal/plugins/exabgp/main.go` | the ExaBGP config or INI environment on stdin |
| `ze appliance init --cert - --key -`, `--manifest -` | `internal/appliance/cmd_init.go` | certificate, key or manifest bytes on stdin. The second `-` in one command fails closed on `ErrStdinClaimed`, which is itself untestable from a `.ci` |
| `ze appliance import <archive or ->` | `internal/appliance/cmd_import.go` | the encrypted archive on stdin |
| `ze appliance config ...` and its callers reading `internal/appliance/config.go` | `cliio.ReadFile(path)` | the appliance config on stdin. Unvalidated: I read the call site, not the flag that supplies the path |
| the crash-dump intent path | `readCrashDumpConfig` (`internal/component/config/system/crashdump.go`) | the config on stdin. Unvalidated as a directly typed CLI form: it is reached from doctor and support rather than from its own verb |

Group 3, `ze-peer`, where the substitution is unconditional and no `-` is involved.

| Command | Producer | What `-` would mean |
|---------|----------|--------------------|
| `ze-peer ...:stdin=<block>` | the runner appends a temp file; `internal/test/peer/expect.go` opens it through `cliio.OpenReader` | `-` would read the expect script from stdin, and `expect.go` supports it. The runner never lets it happen, on 610-plus `.ci` lines |

Group 4, the commands whose stdin a `.ci` CAN reach today, listed so a fix does
not break them.

| Command | Why it works |
|---------|-------------|
| `ze-test peeringdb`, `ze-test rpki`, `ze-test replay -`, `ze-test engine-steps -`, `ze-test mcp` | `binName` is `ze-test`, which neither branch matches |
| `ze isis decode`, `ze ospf decode` | they read `os.Stdin` directly, take no path argument and no `-`, so argv holds nothing to substitute. Noted as a divergence from `ai/rules/cli.md`, not fixed here |
| `ze cli -c "show config dump - \| json"` | the `-` is inside a quoted argument, so no argv element equals `-` |

**Behavior to preserve:**
- `ze -` and its flagged forms keep getting a real file in `rec.WorkDir` under the
  name `zeConfigFileName` chooses, with the `start` verb inserted, chowned for a
  credential-dropped child in netns mode. 747 `.ci` files depend on the file
  existing, and several depend on its exact NAME.
- A second distinct stdin block in one record keeps getting its own
  `ze-<block>.conf`, so a two-daemon test still forms a distinct pair.
- Reusing one block across two `cmd=` lines keeps reusing one file, which is what
  makes a restart-against-the-same-config test work.
- The four working routes in Group 4 keep working.
- `test/decode/*.ci` keeps being driven by `parseCIFile`. Whatever this spec does
  to `runner_exec.go` must not change what that driver builds.

**Behavior to change:**
- A `.ci` that names a stdin block for a `ze` command whose `-` means the pipe
  gets the block piped, or gets a refusal naming the directive and the line. It
  does not get a silent rewrite.
- `ze-peer` stops being a special case that cannot pipe, OR the `.ci` says which
  it wants. Which of these depends on the option Thomas picks.
- The runner refuses a `cmd=` line carrying a key it does not read.
- The runner records what it actually ran, including whether stdin was piped, and
  the report shows it.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A `.ci` file on disk. The `stdin=<name>` block and the
  `cmd=<mode>:seq=N:exec=<command>[:stdin=<name>]` line are the two directives
  this spec governs.
- Format at entry: text. A block is either a terminator block taken verbatim, a
  `hex=` value decoded to raw bytes, or a `text=` value with a newline appended.

### Transformation Path
1. `internal/test/tmpfs/tmpfs.go`, `parseStdinBlock`, turns each block into bytes
   in `Tmpfs.StdinBlocks`.
2. `internal/test/runner/record_parse.go` copies them into `Record.StdinBlocks`,
   and `parseCmdExec` turns each `cmd=` line into a `RunCommand` carrying `Exec`
   and `Stdin`.
3. `internal/test/runner/runner_exec.go` splits `Exec` into argv with
   `splitCommand`, resolves `binName` to a binary path, loads the named block into
   `stdinContent`, expands `$PORT` and `$PORT2` inside it.
4. The two rewrite branches run. Either sets `stdinContent` to nil.
5. `exec.CommandContext` builds the process. `proc.Stdin` is assigned only if
   `stdinContent` survived step 4.
6. The child reads its input: from `os.Stdin` through `cliio.OpenReader` or
   `cliio.ReadFile` when `-` reached it, or from the substituted path otherwise.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| `.ci` text to `Record` | `parseStdinBlock` and `parseCmdExec`, both marker or prefix driven | Yes, both read |
| `Record` to argv | `splitCommand`, then the two rewrite branches | Yes, read |
| runner to child process | `exec.CommandContext`, `proc.Stdin`, the substituted file in `rec.WorkDir` | Yes, read |
| child to input | `internal/core/cliio`, or `os.Stdin` directly for `ze isis decode` and `ze ospf decode` | Yes, read |
| runner to author | the report, `rec.StepTrace`, and the failure text | Partly: I read `recordStep` and `checkOutputAssertions`, not the whole of `report.go` |

### Integration Points
- `zeDaemonConfigArgIndex` already separates a daemon-config `-` from every other
  `-`, and `zeDaemonShouldForceFileStorage` already consumes it. A fix that reuses
  it adds no new discriminator.
- `checkKeys` is the refusal mechanism for `expect=` and
  `reject=`. The `cmd=` equivalent is the same idea against a marker parser.
- `Record.recordStep` is where a "what actually ran" record
  belongs, because it already reaches the report.
- `isAcceptOnly` and `test/.accept-only-baseline` are the existing ratchet shape
  for "this class may shrink and may not grow".

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | to be filled at implementation: the rewrite branch is exactly a bypass, and the fix must remove it rather than add a second one beside it |
| No unintended coupling (components stay isolated) | No | to be filled: Option A would couple the runner to the `ze` command surface, which is why it is rejected below |
| No duplicated functionality (extends existing, does not recreate) | No | to be filled: the fix must reuse `zeDaemonConfigArgIndex` and `checkKeys` rather than write new discriminators |
| Zero-copy preserved where applicable (refs, not copies) | No | not applicable, test tooling off any wire path. State it as N-A with this reason |
| Registration over hardcoding | No | to be filled: no per-command list of `ze` verbs may enter the runner |

## Design Options (OWNER DECISION REQUIRED)

Four ways to separate the two meanings. A is rejected on a rule, not on taste.
B, C and D are live and Thomas picks.

| Option | How it works | What it costs | What it gains |
|--------|-------------|---------------|---------------|
| A. Distinguish by COMMAND | the runner holds a list of `ze` verbs whose `-` is a daemon config, and substitutes only for those | REJECTED. `ai/rules/principles.md` bans a central enumeration a new feature must edit, and this one would have to track every `ze` verb that ever takes a path. It also keeps the runner answering a question the author did not ask | nothing that B or D does not give |
| B. Explicit `.ci` directive | `:stdin=<block>` means what the doc already says, pipe it. A new `cmd=` marker, `:config=<block>`, means materialize a file and substitute the `-` | rewriting roughly 900 `.ci` lines (747 `ze -`, 26 `ze --mcp`, about 120 `ze --plugin`), plus a parse-time refusal for a `ze -` line that still says `stdin=`, plus one more marker in `parseCmdExec` and its doc table | the `.ci` states its own meaning, and no runner heuristic reads argv at all. The largest diff and the only option with nothing left to infer |
| C. Remove the substitution entirely | every `stdin=` block is piped. Daemon-launch tests declare their config as a `tmpfs=` block and name the path in `exec=` | rewriting the same 900 lines AND every fixture that depends on the file's NAME: `action=rewrite:dest=ze-bgp.conf`, the SIGHUP reload tests, the restart-against-the-same-file tests, the rollback assertions on `rollback/ze-bgp-*.conf`, and `zeConfigFileName`'s two-daemon rule. The netns chown of the config file moves to the tmpfs writer | one meaning for `stdin=`, no special case anywhere, and `-` in a `.ci` means what it means in a shell |
| D. Narrow the branch to a true daemon launch | substitute only when the matched `-`'s index equals `zeDaemonConfigArgIndex(args)`. Pipe in every other case | the 423 `ze config validate -` lines flip from the path form to the pipe form. Content-identical, but any assertion naming the file changes, and `ze config fmt -w -` style commands change branch. `ze-peer` still needs its own answer, since it has no `-` to test. Requires a full functional-suite run to prove nothing else moved | the smallest diff by a wide margin, and the discriminator already exists and is already about the daemon rather than about a list of commands |

**Option D is already built for ONE of the two runners (2026-09-06, commit
`d1e6e2d200`), so the decision below is smaller than the table states.** The
parse suite has a second runner with its own copy of the substitution
(`runOneCommand`, `internal/test/runner/parsing.go`), and that copy now narrows
exactly as D describes: it inserts the `start` verb when the matched `-`'s index
equals `zeDaemonConfigArgIndex(args)`, and substitutes in place otherwise. It
was fixed rather than left because the unnarrowed branch had made a security
test vacuous, `test/parse/tacacs-key-required.ci`
(`plan/journal/green-that-could-not-have-been-red.md`).

Two facts this produced, both bearing on D's cost column:

- The full `test/parse` suite ran green over the change, 328/330, with the two
  failures pre-existing and unrelated. So D's "requires a full functional-suite
  run to prove nothing else moved" is discharged for this suite's 330 files.
- The parse suite's `-` substitution is the PATH form for `ze config validate -`
  and stays so under D, because `zeDaemonConfigArgIndex` returns `-1` there.
  The generic runner's 423 lines are the same case, which supports A-1.

What is NOT settled: the generic runner (`runner_exec.go`) is untouched, and
`ze-peer` still has no answer under D.

Two sub-questions ride on the choice and Thomas should answer them together:

| Sub-question | If B | If C | If D |
|--------------|------|------|------|
| `ze-peer` | `:expect-file=<block>` for the current behavior, `:stdin=` pipes | the block is piped and `ze-peer` reads `-`; every `ze-peer` line gains a `-` | unresolved by the discriminator. Needs its own answer: either the B treatment or the C treatment for this one binary |
| the 423 `ze config validate -` lines | they say `stdin=` and start piping | same | same |

## Detection: how a silently-wrong `.ci` is caught

This is the part that matters, and it is why the spec is not one line.

**The common cause, verified at four producers.** The runner answers a directive it
did not understand rather than refusing it.

| # | Instance | Producer | State |
|---|----------|----------|-------|
| 1 | an unrecognised key inside `expect=` was dropped in silence, so ten `not-contains=` lines across seven files asserted nothing | `parseExpect` | FIXED by `checkKeys`, commit `8c7f0a5bf2`, 2026-09-06 |
| 2 | every stream assertion reads the whole file's accumulated stdout and stderr, so the stream named in the directive selects nothing and the command it sits near selects nothing | `checkOutputAssertions` | documented in that function's own comment at `8c7f0a5bf2`, NOT fixed |
| 3 | the `-` rewrite, this spec | the `binName == binNameZe` branch in `runner_exec.go` | not fixed |
| 4 | a `cmd=` line's unknown key is swallowed into the preceding value rather than refused, because `parseCmdExec` is marker-based and `checkKeys` never sees it | `parseCmdExec` with `nextMarker` | found writing this spec, not fixed |

**What the runner owes an author who writes something it cannot honor: a refusal
naming the directive and the line.** Not a silent fallback, and not a best-effort
rewrite. `ai/rules/principles.md` already says so, and the runner has the shape for
it: several parse-time refusals already name the block, the line number and the
directive, and `checkKeys` names the directive and lists the keys it accepts.

Four detection mechanisms, and the spec proposes the first three. The fourth is
named to be rejected on the record.

| ID | Mechanism | Catches | Cost |
|----|-----------|---------|------|
| D1 | key-vocabulary refusal on EVERY directive parser, `cmd=` included. The marker parsers get the same treatment as `checkKeys`: scan for `:<token>=` shapes the marker set does not know, and refuse | a misspelled key, a key from a sibling directive, a key that used to work. Instance 1 and instance 4 | small. `checkKeys` exists; the marker parsers need an unknown-marker scan beside it |
| D2 | the runner records what it RAN, not what was written: the argv actually executed and whether stdin was piped, as a step in `rec.StepTrace`, printed on failure and under `-v` | any rewrite the runner performs, including one nobody has thought of yet. It would have made instance 3 visible the first time anyone read a failure | small. `recordStep` already exists and already reaches the report |
| D3 | a must-fail fixture suite for the runner itself: `.ci` fixtures that MUST go red, run by a Go test that fails if any of them passes. The mirror of the `accept_only` ratchet, aimed at the RUN path rather than the parse path | the whole class. A fixture asserting "stdin was piped" goes red against a runner that substitutes a file, and green after. It is the forced red that `ai/rules/interop-and-goal-validation.md` requires, made permanent | medium. No must-fail harness exists for the run path today; the existing negative tests are Go tests over fixture strings at parse time |
| D4 | a corpus lint that checks each `exec=` line's `-` against the command's declared stdin capability | rejected. It is Option A wearing a lint's clothes: it needs the same central list of `ze` verbs, and `ai/rules/principles.md` bans it | - |

**The strongest form of D3, and the reason it is the answer.** The class is "green
that could not have been red". No static check finds it in general, because the
test is well formed and the assertion is real; only the STIMULUS is wrong. The
only general detector is a forced red, and the only way to keep a forced red is to
store it. So the runner owes a suite of `.ci` fixtures whose whole purpose is to
fail, one per rewrite the runner performs, and a gate that goes red when one of
them passes. That is what would have caught instance 3 on the day the branch was
written, and it is the only one of the four that would catch instance 5.

**A weaker claim, stated as weak.** D1 and D2 catch misspelling and make a rewrite
visible. Neither catches an author who writes a well-formed directive the runner
honors correctly and that nonetheless tests something other than what the comment
above it claims. Nothing mechanical catches that. It is why the discipline in
`plan/journal/green-that-could-not-have-been-red.md` exists: break the thing the
green judges and watch it go red.

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | `zeDaemonConfigArgIndex` returns `-1` for every Group 2 command in the table above | read the function: the first non-flag word decides, and `bgp`, `config`, `doctor`, `support`, `plugin`, `data`, `tacacs`, `exabgp`, `appliance` each fall through to `return -1` | Option D substitutes where it should pipe, on a command nobody listed | a table test over one argv per Group 2 row asserting `-1` | unvalidated |
| A-2 | the 423 `ze config validate -` lines pass identically when the config is piped rather than passed as a path | `cmd_validate.go` reads through `cliio.ReadFile`, which returns the same bytes either way | a suite-wide red that looks like a product defect | run the config suite under the change and diff the pass set | unvalidated |
| A-3 | no `.ci` asserts on the substituted file's NAME in a Group 2 command | the name matters for `action=rewrite:dest=ze-bgp.conf` and the rollback assertions, which are daemon tests | a Group 2 test breaks for a reason unrelated to stdin | grep the suite for `ze-bgp.conf` and `ze-<block>.conf` beside a non-daemon `exec=` | unvalidated |
| A-4 | `ze-peer` given both a `-` and an appended temp file misbehaves | I did not run it. `expect.go` opens one path and the runner appends one | the `ze-peer` sub-question is easier than stated | run one such `.ci` and read the error | unvalidated |
| A-5 | `checkKeys`, `checkOutputAssertions` and the `reject=stdout:contains=` move are at HEAD and will not move again before this spec is implemented | read as uncommitted on 2026-09-06, then observed as commit `8c7f0a5bf2` with the package clean | this spec's D1 duplicates or contradicts a parser change that moved under it | re-read `git log` and `git status` for `internal/test/runner/` at the start of implementation | unvalidated |
| A-6 | `test/decode/*.ci` is unaffected by any change to `runner_exec.go` | `parseCIFile` and `parseDecodeCmdLine` build their own argv from the `exec=` line and never reach the command loop | a change intended for `test/ui/` silently moves 200-plus decode tests | run the decode suite before and after and compare the pass set | unvalidated |
| A-7 | the crash-dump intent path and `internal/appliance/config.go` are not directly typed `ze` verbs taking `-` | I read the call sites, not the flags that supply their paths | two rows of the Group 2 table are wrong about the command spelling | read the flag registration for each | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | a rewrite of roughly 900 `.ci` lines (Options B and C) lands with a mechanical error nobody sees, because the affected tests were already passing for the wrong reason | the suite stays green while a spot check of five rewritten files shows the wrong form | rewrite by script, not by hand, and prove the script on a five-file sample whose runs are read line by line before the bulk pass |
| R-2 | the daemon config file's NAME is depended on somewhere the grep in A-3 misses, so a restart or rollback test breaks late | a reload test reports "sighup reload complete" having changed nothing, which is exactly how the WorkDir defect showed itself before | keep `zeConfigFileName` and its two-daemon rule untouched in Options B and D; in Option C, port the whole naming rule to the `tmpfs=` route before touching any test |
| R-3 | this spec collides with further parser work in the same package, which changed twice in one week | a merge conflict in `record_parse.go`, or two refusal mechanisms for the same key | cite by symbol not by line, re-read `git log` for the package at implementation start, and build D1 on top of `checkKeys` rather than beside it |
| R-4 | D3's must-fail suite becomes a suite that passes for a new reason, which is the same defect one level up | a must-fail fixture that goes red for a message other than the one it names | each must-fail fixture asserts the SPECIFIC failure text, and the gate compares the text, not just the verdict |
| R-5 | fixing stdin re-arms assertions across the suite the way the colon-splitting fix did (203 assertions, 15 suites, one security test that had never run its guard) | a wave of reds in files nobody touched | expect it and read every one as a real finding, not as fallout. Budget for it: this is the second time this class has been repaired |
| R-6 | Option D leaves `ze-peer` unresolved and the spec closes with the biggest group still unable to pipe | the `ze-peer` row of the sub-question table is still empty at review | the `ze-peer` answer is an acceptance criterion, not a follow-up. If it is dropped it becomes its own spec in this bucket, named here |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | the functional suite, which is the evidence every other spec's closure rests on. No shipped binary changes. The worst case is a suite that goes green for a new wrong reason, which is strictly worse than today because today's wrongness is at least recorded |
| How is it reverted? | a single commit revert for Option D. Options B and C rewrite roughly 900 `.ci` files, so the revert is large but mechanical and touches no product code |
| Who else touches this path? | a sibling session changed `record_parse.go`, `record.go`, `accept_only.go`, `peer_contract.go` and `runner_exec.go` and added three files in commit `8c7f0a5bf2` on 2026-09-06, the same day. `plan/immediate/spec-bgp-pcap-decode.md` is in-progress and reached this defect |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| a `.ci` naming a stdin block on a Group 2 `ze` command | → | the command loop's stdin decision in `runner_exec.go` | `TestCIStdinPipesForNonDaemonZeCommand` |
| a `.ci` naming a stdin block on `ze -` | → | the same decision, daemon arm | `TestCIStdinSubstitutesFileForDaemonLaunch` |
| a `.ci` naming a stdin block on `ze-peer` | → | the `binNameZePeer` branch | `TestCIStdinZePeerHonorsDeclaredMode` |
| a `cmd=` line carrying a key the parser does not read | → | the unknown-marker scan beside `parseCmdExec` | `TestCmdUnknownKeyRefused` |
| a run whose argv the runner rewrote | → | `Record.recordStep` and the report | `TestRunReportNamesTheArgvItRan` |
| a must-fail fixture that starts passing | → | the must-fail gate | `TestCIMustFailFixturesAllFail` |
| `ze bgp decode pcap -` driven from a real `.ci` | → | `decodePcapInput` reading piped stdin | `test/ui/bgp-decode-pcap-stdin.ci` |
| `ze bgp decode -` driven from a real `.ci` | → | `decodeHexStdin` | `test/ui/bgp-decode-stdin-hex.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | a `.ci` names a stdin block on a `ze` command whose `-` means the pipe | the block reaches the child's standard input, and no file is substituted into argv |
| AC-2 | a `.ci` names a stdin block on `ze -` or a flagged daemon launch | unchanged: the config is written to the work directory under the name `zeConfigFileName` chooses, argv gains the `start` verb, and the file is chowned in netns mode |
| AC-3 | a `.ci` reuses one stdin block across two daemon launches | unchanged: both read the same file |
| AC-4 | a `.ci` declares two distinct stdin blocks for two concurrent daemons | unchanged: each gets its own `ze-<block>.conf` |
| AC-5 | a `.ci` names a stdin block on a `ze-peer` line | the block reaches ze-peer by the route the `.ci` declares, and a line that declares nothing gets the documented default rather than a silent choice |
| AC-6 | a `.ci` writes a directive the runner cannot honor | the run fails at parse time with a message naming the directive, the line number and what the runner accepts |
| AC-7 | a `cmd=` line carries a key the parser does not read | refused at parse time, naming the key and listing the accepted keys, the way `checkKeys` does for `expect=` |
| AC-8 | any run, verbose or failing | the report states the argv the runner executed and whether standard input was piped |
| AC-9 | `test/ui/bgp-decode-pcap-stdin.ci` runs, with the capture piped | it passes, and it goes RED when the pcap stdin branch of `decodePcapInput` is broken. The red is observed and recorded |
| AC-10 | `test/ui/bgp-decode-stdin-hex.ci` runs, with hex lines piped | it passes, and it goes RED when `decodeHexStdin` is broken. The red is observed and recorded |
| AC-11 | a `.ci` asserts the `ze config history -` refusal | the refusal text is observed, proving a Group 2 negative assertion is now reachable |
| AC-12 | a `.ci` asserts that `ze config fmt -w -` writes the formatted config to standard output | it passes, proving the pipeline branch of `cmd_fmt.go` is reachable, and it would fail against the in-place branch |
| AC-13 | one of the must-fail fixtures starts passing | the gate fails, naming the fixture |
| AC-14 | the whole functional suite runs before and after | the pass set differs only by tests this spec names, and every difference is explained in the closure record |
| AC-15 | `test/decode/*.ci` runs | its pass set is unchanged, proving the `parseCIFile` driver was untouched |
| AC-16 | `docs/architecture/testing/ci-format.md` and `docs/functional-tests.md` are read by an author | they state which form pipes, which form substitutes a file, and how to write each. The existing sentence saying no `.ci` can cover the `ze bgp decode` stdin forms is gone |

## End-to-End User Stories

The user here is a `.ci` author, and the product is the runner.

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | writes a `.ci` piping a capture into `ze bgp decode pcap` | `.ci` to `parseStdinBlock` to `RunCommand` to the stdin decision to `proc.Stdin` to `cliio.OpenReader` to `decodePcapStream` | `test/ui/bgp-decode-pcap-stdin.ci`, with its red observed |
| 2 | writes a `.ci` piping hex lines into `ze bgp decode` | the same path to `decodeHexStdin` | `test/ui/bgp-decode-stdin-hex.ci`, with its red observed |
| 3 | writes a `.ci` asserting a stdin-only refusal | the same path to `dataHistory` | the AC-11 fixture |
| 4 | writes a `.ci` asserting a stdin-only OUTPUT route | the same path to `cmd_fmt.go`'s pipeline branch | the AC-12 fixture |
| 5 | mistypes a directive key | `.ci` to the parser to a refusal | `TestCmdUnknownKeyRefused` |
| 6 | reads a failure report and asks what actually ran | the run to `recordStep` to the report | `TestRunReportNamesTheArgvItRan` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestCIStdinPipesForNonDaemonZeCommand` | `internal/test/runner/runner_exec_test.go` | a Group 2 argv keeps `stdinContent` and sets `proc.Stdin` | |
| `TestCIStdinSubstitutesFileForDaemonLaunch` | `internal/test/runner/runner_exec_test.go` | `ze -` still gets the file, the `start` verb and the chosen name | |
| `TestDaemonConfigArgIndexRejectsEveryGroupTwoCommand` | `internal/test/runner/runner_exec_util_test.go` | A-1: one argv per Group 2 row returns `-1` | |
| `TestCIStdinZePeerHonorsDeclaredMode` | `internal/test/runner/peer_contract_test.go` | AC-5, both routes | |
| `TestCmdUnknownKeyRefused` | `internal/test/runner/record_parse_cmd_test.go` | AC-7, and the message names the key and the accepted set | |
| `TestCmdUnknownKeyNotSwallowedIntoExec` | `internal/test/runner/record_parse_cmd_test.go` | instance 4: the value before the unknown key is not extended over it | |
| `TestRunReportNamesTheArgvItRan` | `internal/test/runner/report_test.go` | AC-8 | |
| `TestCIMustFailFixturesAllFail` | `internal/test/runner/must_fail_test.go` | AC-13, the D3 gate | |
| `TestConfigNameRuleUnchanged` | `internal/test/runner/runner_config_test.go` | AC-3 and AC-4 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| count of `-` elements in one argv | 0 to N | the first one decides today | N/A | two `-` in one argv: state the rule and test it, because `cliio` fails closed on a second stdin claim |
| count of stdin blocks per record | 0 to the parser's limit | unchanged | N/A | unchanged |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `bgp-decode-pcap-stdin` | `test/ui/bgp-decode-pcap-stdin.ci` | an operator pipes a capture into `ze bgp decode pcap -` | |
| `bgp-decode-stdin-hex` | `test/ui/bgp-decode-stdin-hex.ci` | an operator pipes hex lines into `ze bgp decode -` | |
| `config-history-stdin-refused` | `test/ui/config-history-stdin-refused.ci` | an operator pipes a config into `ze config history -` and is told history needs on-disk revisions | |
| `config-fmt-write-stdout` | `test/ui/config-fmt-write-stdout.ci` | an operator pipes a config into `ze config fmt -w -` and gets the formatted config on stdout | |
| the must-fail fixtures | `internal/test/runner/testdata/mustfail/*.ci` | not user-facing: each one proves the runner still refuses or still fails where it must | |

### Interop Tests (Scope: protocol)
Not applicable. Test tooling with no protocol peer and no wire-visible change
(`ai/rules/interop-and-goal-validation.md` names this exemption).

## Files to Modify
- `internal/test/runner/runner_exec.go` - the stdin decision. The two rewrite branches
- `internal/test/runner/runner_exec_util.go` - `zeDaemonConfigArgIndex`, if the chosen option narrows its use
- `internal/test/runner/record_parse_cmd.go` - `parseCmdExec`, the unknown-marker refusal, and any new marker
- `internal/test/runner/record_parse_keys.go` - extend `checkKeys` or its neighbor to serve the marker parsers
- `internal/test/runner/record.go` - any new field the chosen option needs on `RunCommand`
- `internal/test/runner/report.go` - print the executed argv and the stdin verdict
- `internal/test/runner/runner_output_assert.go` - `recordStep`, if the step shape changes
- `docs/architecture/testing/ci-format.md` - the `stdin=` contract, the `cmd=` key table, and the new directive if one is added
- `docs/functional-tests.md` - the UI suite row, which currently says no `.ci` can cover these forms
- `ai/patterns/functional-test.md` - the stdin example, which shows the `-` form
- roughly 900 `.ci` files under `test/` - Options B and C only

## Files to Create
- `internal/test/runner/must_fail_test.go` - the D3 gate
- `internal/test/runner/testdata/mustfail/` - one fixture per rewrite the runner performs
- `test/ui/bgp-decode-pcap-stdin.ci` - AC-9
- `test/ui/bgp-decode-stdin-hex.ci` - AC-10
- `test/ui/config-history-stdin-refused.ci` - AC-11
- `test/ui/config-fmt-write-stdout.ci` - AC-12

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | no config surface changes; the change is inside the test runner |
| YANG validation constraints | N-A | as above |
| YANG custom validators | N-A | as above |
| CLI commands/flags | N-A | no `ze` command changes. The `.ci` directive vocabulary is not a CLI surface |
| CLI grammar (keyword before value) | N-A | as above |
| Editor autocomplete | N-A | as above |
| Functional test for new RPC/API | Yes | the four `test/ui/*.ci` files above, plus the must-fail fixtures |
| Pipe completeness | N-A | no command output changes |
| Env var registration | N-A | no new knob. `ze.tags` is untouched |
| Doctor check for runtime dependencies | N-A | no new file path, socket, port, module, binary or certificate. The substituted config file already exists and its lifecycle is unchanged |
| Prometheus counters/metrics | N-A | test tooling |
| BGP family surface | N-A | no SAFI, capability or attribute |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | N-A | the user is a `.ci` author, covered by rows 10 and 12 |
| 2 | Config syntax changed? | N-A | no `ze` config syntax change |
| 3 | CLI command added/changed? | N-A | no `ze` command changes |
| 4 | API/RPC added/changed? | N-A | none |
| 5 | Plugin added/changed? | N-A | none |
| 6 | Has a user guide page? | N-A | the `.ci` format's page is an architecture page, row 12 |
| 7 | Wire format changed? | N-A | none |
| 8 | Plugin SDK/protocol changed? | N-A | none |
| 9 | RFC behavior implemented, changed, or newly proven? | N-A | no RFC-tagged unit changes. Confirm at implementation that no `rfc/discrimination/` record names a `.ci` this spec rewrites |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md`, whose UI row states the current impossibility, and `ai/patterns/functional-test.md`, whose stdin example shows the `-` form |
| 11 | Affects daemon comparison? | N-A | none |
| 12 | Internal architecture changed? | Yes | `docs/architecture/testing/ci-format.md` (Stdin Blocks, the `cmd=` key table) and `docs/architecture/testing/runner-architecture.md` if the run path's step trace changes |
| 13 | Route metadata keys added/changed? | N-A | none |
| 14 | Prometheus counters added/changed? | N-A | none |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | N-A | none |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | DERIVED at implementation: run `./le spec citation anchors spec plan/pre-release/spec-fixit-ci-runner-cannot-test-stdin.md`. `runner_exec.go`, `record_parse_cmd.go`, `accept_only.go` and `record.go` each carry a `// Design:` header naming `docs/architecture/testing/ci-format.md`, so that page blocks |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `ci-format.md` shows `exec=ze bgp server -:stdin=ze` and `exec=ze -:stdin=ze-bgp`, `docs/functional-tests.md` shows `exec=ze bgp validate -:stdin=config` and `exec=ze bgp decode --json --family <family> -:stdin=payload`, and `ai/patterns/functional-test.md` shows `exec=ze-test decode --family <afi/safi> -:stdin=payload`. Every one is an example of the form this spec changes, and two of them name commands (`ze bgp server`, `ze bgp validate`) whose existence must be re-checked against the current CLI |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- make the decision visible before changing it
   - Tests: `TestRunReportNamesTheArgvItRan`, `TestCIStdinPipesForNonDaemonZeCommand` (expected to fail)
   - Files: `internal/test/runner/runner_exec.go`, `internal/test/runner/report.go`, `internal/test/runner/runner_output_assert.go`
   - Verify: a run records the argv it executed and whether stdin was piped. The pipe test fails, because the substitution still runs. This step alone would have surfaced the defect, so it lands first
2. **Phase: Confirm the discriminator** -- prove A-1 before relying on it
   - Tests: `TestDaemonConfigArgIndexRejectsEveryGroupTwoCommand`
   - Files: `internal/test/runner/runner_exec_util_test.go`
   - Verify: every Group 2 argv returns `-1`, and every Group 1 argv returns the `-`'s index. A row that disagrees is a finding, not a test to adjust
3. **Phase: Apply the option Thomas picked** -- B, C or D
   - Tests: AC-1 through AC-5
   - Files: the runner files above, plus the `.ci` rewrite for B and C
   - Verify: the daemon tests still pass unchanged, and the Group 2 tests now pipe
4. **Phase: Refusal** -- the runner says no rather than guessing
   - Tests: `TestCmdUnknownKeyRefused`, `TestCmdUnknownKeyNotSwallowedIntoExec`
   - Files: `internal/test/runner/record_parse_cmd.go`, `internal/test/runner/record_parse_keys.go`
   - Verify: an unknown `cmd=` key is refused by name, and no value swallows it
5. **Phase: The reachable assertions** -- write the four `.ci` files
   - Tests: AC-9 through AC-12
   - Files: `test/ui/*.ci`
   - Verify: each one is observed RED against the pre-fix runner or the broken producer, then GREEN. Record the red text
6. **Phase: The must-fail gate** -- make the class detectable
   - Tests: `TestCIMustFailFixturesAllFail`
   - Files: `internal/test/runner/must_fail_test.go`, `internal/test/runner/testdata/mustfail/`
   - Verify: each fixture fails with the specific text it names, and the gate goes red when one is made to pass
7. **Phase: Docs** -- the pages that now disagree with the code
   - Files: `docs/architecture/testing/ci-format.md`, `docs/functional-tests.md`, `ai/patterns/functional-test.md`
   - Verify: an author reading only these pages writes a correct stdin test on the first try
8. **Phase: Full suite** -- AC-14 and AC-15
   - Verify: the pass set before and after differs only by the tests this spec names, and every difference is explained

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | every AC has an implementation, and AC-5's `ze-peer` answer is present rather than deferred |
| Feature completeness | a `.ci` author can write a stdin test for every Group 2 command, not only for the two decode forms |
| Correctness | the daemon config file's name, reuse rule, two-daemon rule and netns chown are byte-identical to before |
| No new silent answer | every branch the runner takes on the author's behalf either matches what the `.ci` says or refuses. Grep the diff for a fallback with no error path |
| Discrimination | each of the four new `.ci` files has an observed red, recorded with its text. A green with no observed red is not accepted |
| Rule: `ai/rules/principles.md` | no central enumeration of `ze` verbs entered the runner |
| Rule: `ai/rules/documentation.md` | the doc edits landed with the code, not at closure |
| Rule: `ai/rules/no-layering.md` | the old substitution is DELETED where it is replaced. No mode that keeps both |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| a Group 2 `.ci` pipes | `./le test ui` over `test/ui/bgp-decode-stdin-hex.ci`, plus the recorded red |
| the daemon path is unchanged | the reload, restart and rollback suites pass with no `.ci` edit |
| an unknown `cmd=` key is refused | `go test ./internal/test/runner/ -run TestCmdUnknownKey` |
| the runner reports what it ran | run one failing `.ci` and read the report for the argv line |
| the must-fail gate exists and bites | `go test ./internal/test/runner/ -run TestCIMustFailFixtures`, then make one fixture pass and see red |
| no central command list entered the runner | `git diff` the runner package for a new list or switch naming `ze` verbs |
| docs match | read `ci-format.md`'s Stdin Blocks section against `runner_exec.go` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | a `.ci` is trusted input from the repository, not from a user. The new refusals must not become a way for a malformed `.ci` to hang the runner: every refusal is at parse time and returns an error |
| Resource exhaustion | piping a block rather than writing a file removes a bounded write and adds an unbounded pipe. `cliio.MaxStdinBytes` caps the buffered reads; confirm the streaming readers still have the block-size limit `parseStdinBlock` applies |
| Error leakage | a refusal names the directive, the line and the accepted keys. It must not print the block's contents, which can hold a key or a secret in an appliance test |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| A suite goes red that this spec did not name | it is a re-armed assertion, R-5. Read it as a real finding and route it by `ai/rules/completion.md` |
| A merge conflict with later parser work in this package | re-read `git log` for the package, rebuild on top of `checkKeys`, do not fork it |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- The runner already holds the discriminator this defect needs. `zeDaemonConfigArgIndex`
  answers "is this `-` a daemon config" and the substitution branch does not ask it
  before substituting; it asks it afterwards, only to decide whether to insert the
  `start` verb. The fix in Option D is to move one question one line earlier.
- Every one of the four silent-answer instances is at a PARSER or a REWRITER, never
  at an assertion. The assertions in this runner are honest. What is dishonest is
  the path from the author's text to the process.
- The `test/decode/` driver is the same class at a larger scale: 200-plus `.ci`
  files whose `exec=` line is never executed. Nobody has been misled by it yet
  because those files' `exec=` lines happen to describe what the driver builds. The
  day one stops describing it, the file will still pass.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| OPEN, Thomas decides | B (explicit directive), C (remove the substitution), D (narrow to a true daemon launch) | the three differ by roughly 900 rewritten `.ci` lines and by how much the runner infers. See Design Options |
| Option A is rejected without asking | a per-command list inside the runner | `ai/rules/principles.md` bans a central enumeration a new feature must edit, and it keeps the runner answering an unasked question. This is a rule verdict, not a preference |
| D3 is the detection answer | D1 and D2 alone | D1 catches misspelling and D2 makes a rewrite visible, but only a stored forced red catches the next instance of the class. All three are proposed; D3 is the one that generalizes |
| D4 is rejected | a corpus lint against each command's stdin capability | it is Option A wearing a lint's clothes and needs the same banned list |

## Known Limitations
- `ze isis decode` and `ze ospf decode` read `os.Stdin` directly and accept no `-`,
  so they follow neither `ai/rules/cli.md` nor the `cliio` route. That is a real
  divergence and it is NOT this spec's work: it changes a shipped command's
  argument grammar. One journal row, not a branch here.
- `checkOutputAssertions` reading the whole file's accumulated output (instance 2)
  is not fixed here. It is a distinct mechanism with a distinct repair, it belongs
  to the parser work that just landed, and folding it in would cost this spec its
  single focus. If it is still unfixed at this spec's closure it becomes its own
  spec in this bucket, named here.
- The `test/decode/` driver's unexecuted `exec=` line is named in Design Insights
  and not repaired. It is a third mechanism and a third spec.

## RFC Documentation (Scope: protocol)

Not applicable. No protocol-implementing code changes.

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
- [ ] AC-1..AC-16 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes. It runs every stage against a COMMIT in a throwaway worktree, which is the pre-commit gate (`ai/rules/git-safety.md`). An in-place `./le verify current` is void the moment the tree moves under it
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

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

## Progress, 2026-09-06

Committed in `c31e6a5cb3`. Its three design options await the owner.
