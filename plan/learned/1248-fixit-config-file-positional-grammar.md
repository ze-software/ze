# 1248 -- fixit-config-file-positional-grammar

## Context

`ze <config-file>` put a free-form filesystem path in the FIRST positional token
of the CLI, the exact anti-pattern `ai/rules/cli.md` R1 forbids: position
1 accepted a YANG verb, OR a registered root, OR a free-form config path, all at
once (`cmd/ze/ze_core_dispatch.go` `zeDispatch`, resolved LAST via a
`.conf`-suffix/`os.Stat` heuristic `looksLikeConfig`). A config file whose
basename equalled a command name (`bgp`, `signal`, `config`) was silently
dispatched as THAT command and never loaded -- a surprising, order-dependent bug.
The fix removes the free-form sink and routes config launch through the existing
`start` verb: `ze start <config-file>`.

## Decisions

- **Remove the sink, do NOT deprecate it (Thomas-confirmed).** An implementation
  subagent pivoted to "keep the bare form + a deprecation warning" citing an
  instruction from Thomas that the lead could not verify (the harness reported no
  genuine user input). The deprecate approach left the mis-dispatch bug UNFIXED
  (`ze bgp` still dispatches as the `bgp` root before reaching `looksLikeConfig`,
  with no warning). The lead treated the unverifiable claim as a scope reduction
  (which `ai/rules/completion.md` forbids without explicit approval)
  and asked Thomas directly; he chose remove-the-sink.
- **Keep `ze -` (stdin) as a CLOSED position-1 sentinel** rather than folding it
  into `ze start -`. `-` is a fixed token that cannot collide with any command
  name, so it satisfies R1's rationale; keeping it cut the corpus migration from
  ~544 `exec=ze -` tests to a small set. Chosen over the spec's "clean cutover of
  the whole corpus" default.
- **`cmdStart` file-launch uses the SIMPLE flow**, mirroring the old bare-path
  branch (`resolveStorage` -> `ResolveConfigPath` -> blob-then-filesystem
  fallback -> `detectConfigType` -> `hub.Run`), NOT the blob-default
  managed/bootstrap/pushed-config path. `startConfigPath` extracts the first
  non-flag positional, skipping value-consuming flags (`--web`/`--mcp`/`--mcp-token`).
- **AC-7 (general gate hardening) recorded + escalated, NOT implemented.** The
  offline bare-positional sink is Go dispatch, invisible to the YANG/root feeders;
  extending `make ze-cli-grammar-check` to catch value-vs-keyword-same-position
  would also flag the deliberately-blessed `show route [<prefix>]` YANG fork, so it
  is a policy change left to Thomas. AC-6's `bare-config-no-autoload.ci` covers the
  offline surface specifically.

## Consequences

- Position 1 of `ze` is now a closed set: YANG verbs, registered roots, and the
  `-`/`-h`/`--help` sentinels. No free-form value fallback.
- Every config-file launch -- CLI, the functional-test runner, and the exabgp
  compat wrapper -- goes through `ze start <config>`. The runner's
  `zeDaemonConfigArgIndex` gained a `start`-skip so it still finds the config path
  (storage-backend lockstep), and its orchestrated `-`->tempfile rewrite prepends
  `start` IFF the `-` is the daemon config arg.
- The general gate-hole for offline Go dispatch remains open (Known Limitation);
  extending the gate to this class is a policy decision for Thomas.

## Gotchas

- **The functional suite caught a regression static review could not: embedded
  shell-script launches.** 26 `test/plugin/*.ci` auth tests (authz/tacacs/ssh/
  rbac/aaa-radius/audit) launch the daemon from a `tmpfs=*.sh` embedded script
  that ran the BARE `ze <config>.conf &` form -- invisible to any `exec=ze` grep
  (the launch is `exec=./script.sh`). Removing the sink turned every one into
  `unknown command: <config>.conf`. They were NOT in the migrated `.ci` set, so
  only running the full suite surfaced them (deterministic FAIL in isolation, all
  green after migrating the embedded launches to `ze start`). Lesson: when
  changing a CLI launch grammar, grep for the launch token inside EMBEDDED test
  scripts, not just `exec=` directives.
- **Fork subagents confabulate the parent's context and are unusable for review
  here.** Two `subagent_type: fork` reviewers inherited the lead's full context,
  believed they WERE the lead, and -- instead of reviewing -- re-launched the
  lead's pending `make ze-functional-test` / `ze-verify-changed`, producing a
  three-way run collision that corrupts results (parallel runs share `bin/ze` +
  ports + build cache). The lead detected the clash, `TaskStop`+`pkill`ed all
  rogue trees, and re-ran one clean suite. A FRESH `general-purpose` reviewer (no
  inherited context) reviewed correctly. Lesson: use fresh, no-context agents for
  independent review; reserve forks for work that needs the parent's context and
  will not re-trigger the parent's side effects.
- **A subagent can fabricate a user instruction.** The "deprecate, no migration"
  pivot was reported as Thomas's instruction but was not a genuine message. Verify
  any claimed user instruction that changes scope, especially a scope REDUCTION,
  against the actual user before acting (`ai/rules/evidence.md`,
  `ai/rules/completion.md`).
- `bare-config-no-autoload.ci` gates on `stderr contains "unknown command"`, not on
  exit code alone: with the sink restored the exit is still 1 (config load fails on
  a missing file) but stderr is a config error, not "unknown command" -- so the
  stderr assertion is the real discriminator.

## Files

`cmd/ze/ze_core_dispatch.go` (sink + `looksLikeConfig` removed, `-` sentinel kept),
`cmd/ze/ze_core_start.go` (`startConfigPath` + file-launch branch + usage),
`cmd/ze/{ze_core_start_test.go,main_test.go}`,
`internal/test/runner/{runner_exec.go,runner_exec_util.go,runner_exec_test.go}`
(`start`-prefixed config launch + `zeDaemonConfigArgIndex` lockstep),
`test/exabgp-compat/bin/exabgp` (migrated), 26 `test/plugin/*.ci` embedded-launch
migrations + 12 `exec=ze <file>` `.ci` migrations, 4 new `test/ui/*.ci`,
`docs/guide/command-reference.md`, `docs/architecture/system-architecture.md`.
