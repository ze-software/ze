# Session Start Rationale

Why: `.claude/rules/session-start.md`

## Why Each TOP 6 Rule Exists

- **Rules 1-2** (Read spec, know source) -- Prevent redesigning decisions already made in the spec and writing code that conflicts with existing patterns.
- **Rule 3** (No code without understanding) -- Prevents duplicate code. If you can't name 3 related files, you don't understand the codebase well enough to change it.
- **Rule 4** (TDD: test must FAIL first) -- Catches the case where a test passes immediately, proving it validates nothing.
- **Rule 5** (Preserve existing behavior) -- Prevents inventing new formats when existing ones work fine. Historical example: invented a new JSON format instead of reading `decode.go` and preserving the existing one.
- **Rule 6** (Confirm file paths) -- Prevents editing the wrong file, a common failure mode that wastes entire correction cycles.

## Verification Checks Per Rule

| Rule | Concrete Check |
|------|----------------|
| 1 | `internal/le/spec/session.go current` -> read `plan/<name>` |
| 2 | Check file digests in per-spec session state; re-read full file only when digest insufficient |
| 3 | Can you name 3 related files? |
| 4 | `go test` shows RED before implementation |
| 5 | Document current output format BEFORE changing |
| 6 | Use Glob/Grep to verify target exists and is correct |

## History moved from rule points (2026-09-26)

- From `.claude/rules/session-start.md`, "Style Read (step 2)": "**BLOCKING, every session, whatever the task looks like.** Read `docs/contributing/ze-go-style.md` in full before writing any code. This REPLACES the older instruction in `ai/rules/points/go-standards/directives/read-the-ze-style-guide-before-go-design-or-review.md`, which read the guide only before a Go DESIGN decision, a review, or an argument about how Ze code is written, and told you not to open it for an ordinary edit. That gate was set to save context and it cost more than it saved: a session can write Go all day, never meet one of those three triggers, and never learn that Ze guards with early returns, splits a compound condition, or states an invariant positively."
- From `.claude/rules/session-start.md`, "Style Read (step 2)": "The failure was measured on 2026-08-18. One session wrote `dpd.go`, `detector.go`, `command_registry.go` and `process.go` without opening the guide once, and shipped `if d == nil || !d.awaitReply` and a three-fact error condition. It had a route -- `ai/rules/TRIGGERS.md` lists `go-standards` under \"writing Go in Ze\" -- and did not take it."
- From `.claude/rules/session-start.md`, "Style Read (step 2)": "Two things hid the gap, and neither is a reason to rely on them: `ze-style` is an OUTPUT STYLE (`.claude/output-styles/ze-style.md`), not a skill, so it never appears in the skills listing an agent reads at startup. The native `pretool-writeedit` action (`internal/le/hookruntime/runtime.go`, dispatched via `./le ai hooks pretool-writeedit`) only runs for the `Write`, `Edit`, `MultiEdit`, and `NotebookEdit` tools. Go written through a Bash heredoc reaches it never, and auto mode tells agents to prefer Bash for file changes."
- From `.claude/rules/session-start.md`, "LSP Load (step 1)": "**BLOCKING. Load LSP before any other tool call, regardless of what the task looks like.** The repo has been bitten by sessions that rationalized skipping this step. To close the loophole: every one of the excuses below is **banned reasoning**. If you find yourself thinking any of them, stop and call `ToolSearch query=\"select:LSP\"` first."

  | Banned excuse | Reality |
  |---------------|---------|
  | "The task is shell-only / Makefile-only" | Shell edits drive Go tests. Investigations branch. Load it. |
  | "The task is docs / markdown-only" | Docs describe Go code. You may need to verify a symbol. Load it. |
  | "The task is config / YAML-only" | Config references Go structs. Load it. |
  | "It's a trivial one-file change" | Triviality is judged after reading, not before. Load it. |
  | "LSP is for Go navigation and I won't navigate" | Predicting future tool use is the antipattern. Load it. |
  | "The user will correct me if I need it" | They have. Repeatedly. That is the cost. Load it. |

  "Loading LSP is ~1 tool call and zero-cost if unused. Skipping it costs a round-trip with the user every time you are wrong about what the task needs. The asymmetry is not close."
- From `.claude/rules/session-start.md`, "Empty-result carve-out": "(by design -- a stuck session is the worse failure). The banned excuses above are about SKIPPING the query; issuing it and getting nothing back is not a skip."
- From `.claude/rules/session-start.md`, "A loaded schema is not a working server": "That is what happened on one of the two dev machines: the server was absent there until 2026-08-05, and that machine's transcript store held 33 sessions with no LSP call in any of them (`./le ai tokens` reads `~/.claude/projects/`, so its counts are per-machine and say nothing about the other). The gate could not see it, and by design will not: it lifts on the query text, because a stuck session is the worse failure."
- From `.claude/rules/session-start.md`, "A loaded schema is not a working server": "Working on without a server, having seen it is absent, is the failure this paragraph exists to name."
- From `.claude/rules/session-start.md`, "Mechanical rule": "the first `ToolSearch` / `Bash` / `Read` / `Edit` / anything in a new session must be `ToolSearch query=\"select:LSP\"`. If it is not, you have violated this rule. Apologize, load it, proceed."
- From `.claude/rules/session-start.md`, "Session Focus": "measured 2026-09-15, one session averaged 375k tokens over 335 calls and the main thread was 19% of eight sessions' spend (`ai/rationale/context-economy.md`)."
