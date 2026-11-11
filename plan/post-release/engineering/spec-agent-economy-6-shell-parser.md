# Spec: agent-economy-6-shell-parser

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | tooling |
| Priority | P1 |
| Release reason | Release-independent development tooling: it changes how the pre-tool Bash hook reads an agent's command line and changes no shipped binary. |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-11 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Child 6 of `plan/post-release/engineering/spec-agent-economy-0-umbrella.md`.
Owner order, 2026-11-11, in reply to the offer to vendor a real bash parser
(`mvdan.cc/sh`, the parser behind shfmt) in place of the hand-written splitter
and syntax allow-list of the pre-tool Bash hook: "incorporate all the
improvements you have identified".
→ Decision: Status moved from `design` to `ready` on that order (2026-11-11): Thomas ordered the work itself, so the design needs no further approval, and the umbrella's child-claim rule admits the claim at the in-progress count plus one.

Child 1 closed with the hook reading a command line through a splitter it
wrote itself (`lineCommands`) and trusting that split only inside a byte-level
allow-list of shell syntax (`modeledSyntax`). Six review rounds of child 1
found the same class each time: the hand splitter read a line differently
from bash. The allow-list closed the class by refusing to read most of bash.
The cost is paid on every call an agent makes:

1. **A harmless line outside the allow-list needs the identity line and keeps
   every guard's text verdict.** A `#` inside a quoted grep pattern, a brace,
   a `!`, a here-document, `[[`, a parenthesis, a backslash outside quotes, a
   UTF-8 pattern, an empty quoted word, a `$` before a closing quote and a
   redirection before the words are all refused (child 1, Known Limitations).
   On 2026-11-11 this spec's own author had a `git show ... | grep -n '^##'`
   refused for the quoted `#`.
2. **A loop or a conditional made only of readers is not a reader line.** No
   reader judge accepts `for`, `do`, `then` or `while`, so the text verdict
   stands, and its split at quoted separators refuses a `grep` whose pattern
   holds `go vet` after a bar (open journal row, 2026-11-11,
   `plan/journal/hook-existing-patterns-false-positive.md`).
3. **A heavy job behind a keyword is not seen at all.** The text judgment
   reads the first word of each piece, so `for p in a b; do go test ./$p;
   done` and `( go test ./x )` read as the commands `do` and `(` and pass.
4. **A line that starts with the agent's own identity line is never a reader
   line**, because `export` is no reader, so an agent that types its line in
   front of a grep gets the text verdict on the grep's pattern.
5. **The word walk that guards the identity after the agent's line has
   holes** (strongest-tier review of child 1's identity exemption,
   2026-11-11): a `.` after `time` or `then` is skipped as a number, so
   `time . ./env.sh` sources a file unseen; `/usr/bin/env -i` and
   `timeout -s KILL 60 env -i` are not read as `env`.

The goal: the hook reads a command line with one real bash parser. A line the
parse proves to be made only of readers, through loops, conditionals, groups
and substitutions, runs as typed and lifts each guard's text refusal. A heavy
job the line really runs is refused wherever the line runs it. Every other
line keeps the verdict it has at HEAD or a stricter one. The hand splitter and
the byte allow-list are deleted, not kept beside the parser.

Smaller findings ride here so the hook source seal is re-derived once: the
raw-job refusal advises the long form without `quiet`, two launcher loops
compile a regular expression on every pass, one test's doc comment and one
page sentence describe a start-text sentence the test asserts absent, the
identity tests miss rows for half the environment tools, and the fork test
reads an environment variable nobody has shown the harness scopes to a fork.

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md` - the constraints every change keeps
  → Constraint: checks register in one place and are discovered. The parse is read by the checks already in `nativeHookActions`; no check and no list is added outside it.
- [ ] `ai/doors/le-tooling.md` - the door for the hooks
  → Constraint: the hooks run on every Bash call of every session, so a hook that refuses wrongly or panics blocks every session at once. The parser is third-party code on that path: a panic inside it is contained and read as a line that did not parse.
  → Constraint: a shipped `ze` never links an `internal/le` package (`TestNormalZeLinksNoInternalLe`), so the parser module reaches `le` only.
  → Constraint: the shared `le` is built from HEAD. An uncommitted hook edit is judged with a named build, and reaches other agents only when committed.
- [ ] `ai/rules/no-layering.md` - replacing X with Y
  → Constraint: `lineCommands`, `lineWords`, `modeledSyntax` and `unmodeledSyntax` are deleted in the commit that puts the parse in their place. No caller keeps the hand splitter for a line the parser refuses.
- [ ] `docs/contributing/dependency-approval.md` - who may import a third-party package
  → Constraint: the module lands in a vendor commit of its own (`go.mod`, `go.sum`, `vendor/`), as `bfc6dfe868` did for circl. The commit that first imports it carries a `dependency-ledger.txt` row only Thomas writes, with `./le deps approve` from his terminal.
  → Decision: one package is imported, `mvdan.cc/sh/v3/syntax`, by one directory, `internal/le/hookruntime`. One ledger row.
- [ ] `.claude/hooks/README.md` and `docs/contributing/running-commands.md`, "Session identity, and what each agent owns" - the pages that describe the splitter and the allow-list
  → Constraint: both pages name `lineCommands` and `modeledSyntax` and list the allow-list byte by byte. They are rewritten in the commit that deletes those symbols.
  → Constraint: both pages say `.`, `source` and `set` are refused after the agent's line "past a reserved word or a wrapper such as `time` or `nice`". The code does not do that for `.` (finding 5 of the Task), so the code is the defect.
- [ ] Child 1's record: `spec-agent-economy-1-call-shape`, closed; read it from the commit before its closure, `391cc12bc6^`
  → Decision (phase 2, Design Insights): "the text judgement is each guard's default verdict, unchanged from HEAD; readerLine and the reader+build line may only lift it." Kept: only a line proven to be readers lifts a text refusal.
  → Decision (Mistake Log): editing the text judgement cleared lines HEAD refused, and opening the string a shell is handed reopened the hole for every program that runs a string. Kept: this spec changes neither. A string is never opened.
  → Decision (Key Design Decisions): the agent types its identity line and the hook checks it. Kept: the hook returns no changed input.
  → Decision (Known Limitations): the check catches a mistake and is no sandbox. Kept: a script file, `eval` and a variable holding a command stay opaque.
- [ ] `ai/rules/user-facing-errors.md`, `ai/rules/commands.md`
  → Constraint: every refusal names what is wrong, why, and the exact command that fixes it. A heavy command's advised form takes `quiet`.

**Key insights:**
- The parser changes what the hook can prove, not what it must refuse. Proving a line is made of readers is the only way a verdict loosens.
- A node-kind allow-list replaces the byte allow-list. The byte list protected a splitter that could misread bash. The node list states which constructs can run only readers; it protects nothing, it is the definition of a reader line.
- On a line that is not a reader line, HEAD's text functions run unchanged over the unchanged text, so "HEAD's verdict or stricter" holds by construction, and a recorded table of HEAD's verdicts proves it again by test.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/le/hookruntime/shellline.go` - `lineCommands` is the one hand splitter: it walks the line character by character and yields each command's words (quotes removed), its text (`segments`), which words the shell expands (`expands`), whether its text holds an unquoted `>` (`redirects`), the operator that ends it (`separators`) and whether the line holds a command substitution (`substitutes`). It recognises no keyword: `do`, `then` and a leading assignment are the first word of their command. A redirection's target is a word of its command. `lineWords` flattens it. `readerLine` answers a line inside `modeledSyntax` that parses whole, holds no substitution, and whose every command is a `readerCommand` that writes no file. `writesFile` asks `writesOnlyDevNull` over `shellWords` of the segment.
- [ ] `internal/le/hookruntime/shellsyntax.go` - `unmodeledSyntax` is the byte allow-list: printable ASCII; one-line single- and double-quoted strings holding no `#`; plain `$NAME` and `${NAME}` inside double quotes only; the separators `;`, `&&`, `||`, `|` and a newline; a redirection only after the last word of its command; no empty quoted word. It answers the first construct outside the list in words a refusal carries. `dollarStep` answers "locale quoting" for a `$` before a closing double quote, which bash keeps as a literal `$`. `modeledSyntax` is its boolean.
- [ ] `internal/le/hookruntime/readers.go` - `readerCommands` maps 26 command words to a judge over `(arguments, expands)`. A judge is an allow-list of options and argument shapes; a nil judge takes any argument, and its comment names the GNU option sets without saying the table assumes them. `readerCommand` requires a command word the shell does not expand. A judge sees a redirection's target as an argument, so `[ -f x ] 2>/dev/null` and `uniq -c 2>/dev/null` are refused.
- [ ] `internal/le/hookruntime/bash.go` - `commandSegments` splits a line at every `&&`, `||`, `;`, newline, `|`, `|&` and `&`, quoted ones included, and is the text judgment of `bashRawHeavy` and `bashDocumentRead`. `bashLossyPipe` splits with three regular expressions it compiles on each call. `shellWords` splits one segment at whitespace with quotes removed. `programWords` strips assignments and launchers and compiles a regular expression inside its launcher loop. `bashTestDeletion` judges `lineWords` of the whole line. `gitVerbRun` and `insideQuotes` judge git verbs by text position and call neither splitter.
- [ ] `internal/le/hookruntime/rawheavy.go` - `rawHeavy` judges one segment's first word past assignments and the launchers `bash`, `sh`, `perl`, `ruby`, `sudo`, `time`, `nice`, `env`, `timeout`, compiling a regular expression on every pass of the launcher loop. `packageRefusal` advises the short package action when it expresses the words exactly, and otherwise `admittedCommand(label, false, kept)`, the long form without `quiet`; the lint arm does the same. `rawAdvice` rebuilds the advice from `lineCommands` on a modeled line and keeps the text refusal on any other.
- [ ] `internal/le/hookruntime/lossypipe.go` - `lossyPipeRefusal` advises `admittedCommand(label, true, words)`, the quiet form. `advisedSegment` picks the advised command from `lineCommands` on a modeled line, else the text segment.
- [ ] `internal/le/hookruntime/gobuild.go` - `goBuildLine` judges a build by its words only on a reader+build line, reading the build's arguments with `shellWords` over `line.segments` because the splitter's own words split `2>&1`. `cdFollowed` reads `line.separators`. `goBuildText` is the text judgment.
- [ ] `internal/le/hookruntime/session.go` - `commandOutsideReaders` names what makes a line need the identity line, in the order: unmodeled syntax, unparsable, substitution, first command outside the reader set. `takesAnotherIdentity` and `remainderChangesEnvironment` judge what follows the agent's line by `lineCommands` on a modeled line and by `textChangesEnvironment` on any other. `changesEnvironment` walks a command's words: it skips a word of `transparentWords` (the reserved words and the wrappers `time`, `command`, `builtin`, `nice`, `nohup`, `timeout`, `xargs`, `stdbuf`), then skips every word that starts with `-` or is made only of the characters `0123456789.smhd`, and answers whether the first word left is in `environmentTools`, compared whole. A lone `.` is made only of those characters, so it is skipped; `KILL` after `-s` is the first word left; `/usr/bin/env` is not the word `env`. `set` is in `environmentTools` with every option. `rawWords` is a second hand word splitter that keeps quotes as typed. `bashAgentIdentity` and `commandIdentityPrefix` read a payload with no agent id as a fork when the hook's environment holds `CLAUDE_CODE_FORK_SUBAGENT`.
- [ ] `internal/le/ai/rules/session_coverage.go`, `internal/le/spec/model.go` - each `TranscriptPath` reads `CLAUDE_CODE_FORK_SUBAGENT` to pick the session's transcript when no session id is in the environment. They run inside an `le` action, hold no hook payload, and refuse nothing.
- [ ] `internal/le/hookruntime/systemtmp.go` - `namesSystemTmp` judges words on a reader line (`systemTmpWord`) and the `systemTmp` pattern on any other.
- [ ] `internal/le/hookruntime/testdeletion.go` - `plainRmArguments` narrows the deletion guard on a modeled line of plain `rm` commands and readers.
- [ ] `internal/le/hookruntime/runtime.go` - `Run` builds one `context` per call, runs every check of `nativeHookActions`, and recovers a panic by answering exit 0.
- [ ] `internal/le/hookruntime/lifecycle.go` - `hookSubagentContext` prints the identity line; with no agent id it prints the session part and says that part is the whole line. It never says a refusal prints more.
- [ ] `internal/le/hookruntime/lifecycle_test.go` - the doc comment of `TestSubagentStartCarriesTheIdentityLine` says a payload with no agent id "says that the first refusal prints the full line"; its rows assert the words "first refusal" absent. `docs/contributing/running-commands.md` carries the same stale sentence.
- [ ] `internal/le/hookruntime/session_identity_test.go` - `TestSubagentChangingTheEnvironmentAfterItsLineIsRefused` holds no row for `source`, `.`, `set`, `typeset`, `readonly` or `local`. The rows for `eval`, `command`, `builtin` and `then`, in it and in its two siblings, each carry a literal identity variable name, which the mention rule refuses by itself, so they prove nothing about the form. No row puts an environment tool behind `if`, `elif`, `else`, `do`, `while`, `until`, `nohup`, `timeout`, `xargs` or `stdbuf`.
- [ ] `internal/le/ai/hooks/parity.go`, `parity_catalog.go` - 83 bash rows judged by `bashCode`, a hand model of the main thread's verdict that runs no row through `internal/le/hookruntime`; `parityCatalogDigest` seals the rows.
- [ ] `internal/le/ai/hooks/fixtures.go` - `hookSourcesDigest` seals every non-test Go file of the packages in `hookSourcePackages`, `internal/le/hookruntime` among them. `vendor/` is outside it.
- [ ] `go.mod`, `vendor/modules.txt` - `mvdan.cc/gofumpt`, `mvdan.cc/unparam` and `mvdan.cc/xurls/v2` are present as indirect modules. `mvdan.cc/sh` is absent from both, from `dependency-ledger.txt` and from the local module cache. The module declares `go 1.27.3`.

**Behavior to preserve:**
- The hook never changes a command. A subagent's command that could start `le` carries the agent's exact identity line, checked and never added.
- Every guard refuses the command it names, and on a line that is not a reader line every refusal HEAD gives stands.
- A script, `eval`, a string handed to a shell and a variable holding a command stay opaque: nothing opens them.
- The reader judges keep their signature and their verdicts.
- `Run` fails open on a panic and on malformed input. The main thread is never asked for the identity line.
- The budget, the git guards, the governed-write guard, the poll-loop guard and the scratch guard are untouched.

**Behavior to change:**
- The hook reads a line's commands from one bash parse. The hand splitter and the byte allow-list are deleted.
- A reader line may hold loops, conditionals, groups, subshells, negation, substitutions and here-documents, when everything they run is a reader.
- The agent's own exact identity line in front of a reader line leaves it a reader line for the guards.
- A heavy job that is a real command of the line is refused at any depth.
- A refusal's advised command is built from the parsed words on every line that parses.
- After the agent's line, an environment tool is found behind any reserved word and any wrapper, by its base name; `set` with options that change no variable passes.
- The raw-job refusal advises the quiet form. No launcher loop compiles a regular expression.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Hand splitter | `internal/le/hookruntime/shellline.go`, `lineCommands`, `lineWords`, `shellLine` | Splits a line into commands, words, expansions, separators | Replaces: deleted. One parse (`mvdan.cc/sh/v3/syntax`, bash variant) yields the statement tree every caller reads. No caller keeps the splitter for a line the parser refuses |
| Byte allow-list of syntax | `internal/le/hookruntime/shellsyntax.go`, `modeledSyntax`, `unmodeledSyntax`, `syntaxCommand` | Bounds where the splitter's answer is trusted | Replaces: the file is deleted. Its job, protecting a splitter that misreads bash, has no successor. The reader-line predicate states which node kinds it admits, and names the first one it does not in the refusal, as `unmodeledSyntax` does today |
| Reader line | `internal/le/hookruntime/shellline.go`, `readerLine`, `writesFile` | The only line on which a guard lifts its text refusal | Extends: same rule, same callers, read from the statement tree, so it reaches every construct whose commands are all readers. A redirection is a node of its statement, so its target is read exactly, no judge sees it as an argument, and `writesFile` no longer re-splits text |
| Reader set and judges | `internal/le/hookruntime/readers.go`, `readerCommands`, `readerCommand` | Declares which commands run nothing they are handed | Extends: the table and the judge signature stay. `read` joins with a judge. The `[[` conditional is judged from its parsed operators. The table's comment states the GNU assumption. Nothing else in the table moves |
| Identity check | `internal/le/hookruntime/session.go`, `bashAgentIdentity`, `commandOutsideReaders`, `takesAnotherIdentity` | Exempts a reader line, checks the typed line, refuses a second identity | Extends: same three outcomes. The exemption and the after-line rules read the statement tree; `rawWords` goes, because an assignment is a node with a name |
| After-line word walk | `internal/le/hookruntime/session.go`, `changesEnvironment`, `transparentWords`, `environmentTools` | Finds an environment tool past reserved words and wrappers | Extends: the two tables stay the declaration. Reserved words leave `transparentWords`, because the tree carries them as structure; the wrappers gain their value-taking options; a skipped value holds a digit; a tool is matched by base name; `set` is judged by its options |
| Fork detection in the hook | `internal/le/hookruntime/session.go`, `bashAgentIdentity`, `commandIdentityPrefix` | Reads `CLAUDE_CODE_FORK_SUBAGENT` from the hook's environment | Extends, after a probe: the probe records what a fork's and the main thread's hook see. If the variable is set for the main thread too, a fork is read from the payload |
| Fork detection for the transcript | `internal/le/ai/rules/session_coverage.go` and `internal/le/spec/model.go`, `TranscriptPath` | Reads the same variable to pick a transcript, with no payload | Does not apply: they hold no payload to read a fork from and refuse nothing. The probe's record says whether their test is sound, and a fault there is a journal row, not this spec's change |
| Text judgment of the raw-job and piped-command guards | `internal/le/hookruntime/bash.go`, `commandSegments`, `shellWords`, `programWords`, `commandExpensive`, the split in `bashLossyPipe`; `rawheavy.go`, `rawHeavy` | Each guard's default verdict, quote-blind | Extends: kept unchanged as the verdict of a line no parse proves to be readers, per child 1's decision. Added beside it on a parsed line: the same word judgment over each real command of the tree. It is not a fallback for the parser: it judges text a non-reader may run, which no parse can clear |
| Text judgment of the build, `/tmp` and deletion guards | `gobuild.go` `goBuildText`; `bash.go` `systemTmp`, `bashTestDeletion` | Whole-line patterns and a line-wide word list | Extends: the two patterns are unchanged. The deletion guard's word list came from `lineWords`, which is deleted, so it is re-derived from the tree and held to HEAD's verdicts by the recorded table |
| Advice from the modeled line | `rawheavy.go` `rawAdvice`; `lossypipe.go` `advisedSegment` | Rebuild the advised command from `lineCommands` on a modeled line, else from the text segment | Replaces: the advice is the real command's words on every line that parses. The text segment remains only for a line that does not parse |
| Admitted replacement text | `bash.go` `admittedCommand`, `quotedWord` | Writes `./le job run label <label> [quiet] command <argv>` | Extends: `packageRefusal` and the lint arm pass `quiet`, as `lossyPipeRefusal` does |
| Build judgment | `gobuild.go` `goBuildLine`, `cdFollowed`, `judgeGoBuild` | Judges a build by its words on a reader+build line | Extends: `judgeGoBuild` and everything under it are untouched. `goBuildLine` reads the build's arguments, its redirections and the position of a `cd` from the tree |
| Git verb guards | `bash.go` `gitVerbRun`, `insideQuotes`, `gitInvocation` | Judge a git verb by its text position, with a quote exemption of their own | Does not apply: they call neither splitter, and they enforce a hard ban. Moving a hard ban onto a new reading is a decision of its own, not a consequence of this replacement |
| Governed-write taint, poll-loop and scratch guards | `bash_governed_taint.go`; `bash.go` `bashPollLoop`, `bashScratch`, `bashGovernedWrite` | Text patterns over the whole line | Does not apply: none reads the splitter, and none lifts on a reader line |
| Check registry | `internal/le/hookruntime/runtime.go`, `nativeHookActions`, `context` | The authority for every native check; one context per call | Extends: no check joins. The parse is made once per hook call, held by the context, and shared by the checks that read it |
| Parity golden and fixture sites | `internal/le/ai/hooks/parity_catalog.go`, `parity.go` `bashCode`, `fixtures.go` | 83 model-judged bash rows; behavioral sites that run the hook | Extends: rows and sites join. Replaces `bashCode`: every bash row is judged by the hook's own `pretool-bash` action (AC-36), so a row proves its verdict |
| Source seal | `fixtures.go` `hookSourcesDigest`, `parity.go` `parityCatalogDigest` | Detect a change to the hook sources and the rows | Extends: both are re-derived once, in the last code phase |
| Dependency ledger | `dependency-ledger.txt`, `./le deps approve`, `docs/contributing/dependency-approval.md` | Declares who may import which third-party package | Extends: one row, written by Thomas |
| Vendor-only commit | commit `bfc6dfe868` (circl) | The precedent for a module that lands before its first use | Extends: the same shape for `mvdan.cc/sh/v3` |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- The harness sends the PreToolUse payload of a Bash call to `le ai hooks pretool-bash`. The command is one string.

### Transformation Path
1. `Run` decodes the payload and builds the context.
2. The first check that needs it parses the command once, as bash, and the context keeps the result: a statement tree, or the fact that the line did not parse. A panic inside the parser is recovered there and recorded as "did not parse".
3. The reader-line predicate walks the tree. It answers yes only when every node is of an admitted kind and every simple command is a reader its judge admits that writes no file. Otherwise it names the first thing outside.
4. Each guard asks the predicate. On a reader line the build, piped-command, raw-job, `/tmp` and deletion guards lift, as today. On any other line each guard gives its text verdict over the raw line, unchanged, and the raw-job and piped-command guards also judge each real command of the tree.
5. The identity check exempts a reader line, passes the agent's exact line, and judges what follows that line from the tree.
6. A refusal builds its advised command from the parsed words of the command it refused.
7. The hook answers with an exit code and its messages. It writes no changed input.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Harness ↔ hook | JSON payload in; exit code and stderr text out | Yes, `Run`, unchanged |
| Hook ↔ parser | One call with the command string; a tree or an error back | No: phase 2 |

### Changed-Wire Census

No wire changes format. The payload, the exit codes and the absence of changed
input are as child 1 left them. Two texts an agent reads change wording.

**Search evidence:** `grep -rn "lineCommands(\|lineWords(\|shellWords(\|commandSegments(\|modeledSyntax(\|unmodeledSyntax(\|readerLine(\|rawWords(\|programWords("` over `internal/le`, non-test Go. Every caller lies in `internal/le/hookruntime` (`bash.go`, `gobuild.go`, `lossypipe.go`, `rawheavy.go`, `readers.go`, `session.go`, `shellline.go`, `shellsyntax.go`, `systemtmp.go`, `testdeletion.go`). Positive control: `internal/le/test/stressrepro/run.go` appears for a `shellWords` of its own, a different function in a different package, so the search reaches outside the hook package and finds no other reader. `grep -n "mvdan" go.mod vendor/modules.txt` finds three modules, none of them `sh`. `grep -rn "CLAUDE_CODE_FORK_SUBAGENT"` over non-test Go finds `session.go` twice, `internal/le/ai/rules/session_coverage.go` and `internal/le/spec/model.go`.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| Hook verdict (exit code, stderr) | writer | `internal/le/hookruntime/runtime.go`, `Run` | read at source; no edit planned to `Run`'s output | Preserved |
| Hook verdict (exit code, stderr) | reader | the Claude Code harness | outside the repository | Preserved |
| Identity refusal text | writer | `internal/le/hookruntime/session.go`, `commandOutsideReaders` | read at source: the reason today is an `unmodeledSyntax` construct | Changed: the reason names the parsed construct or command that is outside the reader line. The line to type is unchanged |
| Identity refusal text | reader | the refused agent; `internal/le/hookruntime/session_identity_test.go` | `TestUnmodeledSyntaxNamesEveryConstructOutsideTheAllowList` and its siblings assert the old reasons | Changed with the judge's approval (Test changes, below) |
| Raw-job refusal text | writer | `internal/le/hookruntime/rawheavy.go`, `packageRefusal`, `rawHeavy`; `bash.go`, `bashRawHeavy` | read at source: `admittedCommand(label, false, kept)` and the generic-form line | Changed: the advised long form and the generic form carry `quiet` |
| Raw-job refusal text | reader | the refused agent; `rawheavy_test.go`, `refusal_messages_test.go`, `bash_test.go` | grep for `quiet` in the first two finds no assertion on it | Changed: the tests that pin the `Use:` line assert the quiet form |

### Integration Points
- `nativeHookActions` - the same checks, in the same order; the budget stays last.
- `readerCommands` - the predicate asks it for every simple command.
- `admittedCommand`, `quotedWord` - write the advised command from parsed words.
- `leroot.Admits`, `RegisterPackageRuns` - unchanged inputs of the cost and short-form judgments.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | every check reads the one parse the context holds; nothing reads a second split of a line the parse covers |
| No unintended coupling (components stay isolated) | Yes | one package imports the parser, `internal/le/hookruntime`; no product package links `internal/le` |
| No duplicated functionality (extends existing, does not recreate) | Yes | the hand splitter, the byte allow-list and `rawWords` are deleted in the commit that adds the parse; the text judgment stays as the verdict it already is |
| Zero-copy preserved where applicable (refs, not copies) | Yes | not applicable: a hook judges one short command line |
| Registration over hardcoding, outbound | Yes | no new check, command or handler |
| Registration over hardcoding, inbound | Yes | the reader set stays one declared table, and the admitted node kinds one declared table beside it. A node kind or a command absent from its table is outside the reader line and is refused with its fix, so a construct nobody listed is never passed unjudged. Searched for the names this spec introduces: `readerCommands`, `environmentTools`, `transparentWords`, the launcher lists of `rawHeavy` and `programWords`; `transparentWords` loses its reserved words, which the tree now carries as structure |

## Design

### The reader line, from the tree

A line is a reader line when it parses as bash and every node of its tree is
in this table. Anything else is outside, and the predicate names it.

| Node | Admitted when |
|------|---------------|
| Simple command | Its command word is text the shell does not expand, a member of `readerCommands`, and its judge admits its arguments; it carries no assignment in front; none of its redirections writes a file |
| Bare assignment | The name is lower-case letters, digits and `_`, and the hook's own environment holds no variable of that name. Its value is judged as a word. This keeps `PATH`, `IFS`, `CDPATH`, `GIT_*` and every exported name outside |
| Pipeline, `&&`, `\|\|`, `;`, newline, `\|&` | Both sides are admitted |
| Negation `!`, `time` | The command it prefixes is admitted |
| Group `{ }`, subshell `( )` | Every statement inside is admitted |
| `if`, `while`, `until` | The conditions and the bodies are admitted |
| `for name in words` | The name passes the bare-assignment rule; the body is admitted |
| `case` | The subject and the patterns are words; every body is admitted |
| `[[ ]]` | Its operators are file and string tests, `=`, `==`, `!=`, `=~`, `<`, `>`, `!`, `&&`, `\|\|` and parentheses. The arithmetic comparisons (`-eq` and the rest), `-v` and `-R` are outside: bash evaluates their operands as expressions. Its words may expand, because no expansion becomes an operator inside `[[` |
| Command substitution `$( )`, backticks, process substitution `<( )` | Every statement inside is admitted. The word that holds it counts as expanded, except a word that is one whole process substitution, which is an operand whose value is a path the shell picks |
| Parameter expansion | Plain `$name` and `${name}`, the special parameters, and `${name}` with a default, an alternative, a prefix or suffix removal, a case change, a replacement or a length. An index, a slice, an assigning default, an indirection and an `@` operator are outside: they evaluate arithmetic, assign, or expand a prompt string |
| Redirection | Input from a file, a here-string or a here-document; output to `/dev/null` or onto another descriptor, wherever it is written in the command. Any other output is a file write, as today |
| Here-document body | Data to the reader that receives it. A body the shell expands is walked for substitutions like any word |

Outside, by kind: a function definition, `declare`, `export`, `local`,
`readonly`, `typeset`, `let`, an arithmetic command or expansion, a C-style
`for`, `select`, a coprocess, a background `&`, an assignment in front of a
command, an ANSI-C or locale quoted string (its value is not read), and
everything not in the table.

The judges still receive `(arguments, expands)`, and an argument is never a
redirection's operator or target. An argument's value is its parts with the
quoting removed: a backslash outside quotes drops and keeps the next
character; inside double quotes a backslash drops only before `$`, a backtick,
`"` and `\`; a single-quoted part is taken whole; a `$` the parser reads as a
literal (before a closing quote) is that character. A word with a parameter
expansion, a substitution, an unquoted glob character, a brace expansion or a
leading `~` is marked expanded. That is the only hand-written reading of shell
text left, it works inside one word the parser already delimited, and
`TestWordValuesMatchBash` compares it with what bash passes.

`read` joins the reader set: only `-r`, and names that pass the
bare-assignment rule.

### The agent's own line in front

When a subagent's command starts with its exact identity line, the guards ask
the reader-line question of what follows it. The two exports are compared
byte for byte with the line built from the payload, as `bashAgentIdentity`
already compares them, and no reader reads either variable (child 1, A-3).
The identity check itself is unchanged: the line passes it by carrying the
agent's line.

### After the agent's line

What follows the agent's line is judged command by command from the tree, at
every depth. A command that is an admitted reader writing no file is skipped,
as today. For every other command:

| Rule | Detail |
|------|--------|
| A declaration node (`export`, `declare`, `typeset`, `readonly`, `local`) | Refused, whatever it names |
| An assignment whose name is not plain, or a command word that holds `=` | Refused, as today |
| The command word, past wrappers | The wrappers are `command`, `builtin`, `nice`, `nohup`, `timeout`, `xargs`, `stdbuf`, each with its declared value-taking options (`timeout -s` and `-k`, `nice -n`, `stdbuf -i`, `-o` and `-e`, the value options of `xargs`). After a wrapper the walk skips an option, the one value of a value-taking option, and a duration, which holds a digit. The first word left is the command word. A lone `.` is a command word |
| The command word is an environment tool | Matched by base name, so `/usr/bin/env` is `env`. Refused: `unset`, `env`, `exec`, `sudo`, `eval`, `source`, `.` |
| `set` | Passes with only the options `-e`, `-u`, `-x`, `-o pipefail`, `-o errexit`, `-o nounset`, `-o xtrace` and their `+` forms, which change no variable. Every other `set` is refused: `-a` exports later assignments, `-k` moves assignments into the environment, and operands replace the positional parameters |
| The mention rule | Unchanged: a command that names either identity variable outside an admitted reader is refused; the mention is read in the command's own source text |

A remainder that does not parse is judged by `textChangesEnvironment`, as
today, with the same digit and base-name rules.

### A line that is not a reader line

Each guard's text judgment runs over the raw line, as at HEAD: same function,
same input. Nothing a non-reader line holds is cleared by the parse, because a
command outside the reader set may run any string it is handed.

On such a line that parses, two guards also judge the tree:

| Guard | Added judgment | Why it can only add refusals |
|-------|----------------|------------------------------|
| `bashRawHeavy` | Each simple command of the tree, at any depth, by the word judgment `rawHeavy` applies to a segment: assignments from the command's own assignment nodes, the same launchers, the same `ZE_ADMIT_RAW` escape | It is OR-ed with the text verdict |
| `bashLossyPipe` | Each pipeline of the tree: an expensive command (`commandExpensive`'s word rule) with a lossy filter later in the same pipeline | It is OR-ed with the text verdict |

A line that does not parse is not a reader line, needs the identity line, and
is judged by its text alone. The deletion guard refuses it when its text runs
`rm`, as today.

### What is deleted

`shellsyntax.go` whole. From `shellline.go`: `lineCommands`, `lineWords`,
`shellLine` and their helpers. From `session.go`: `rawWords`, and the reserved
words of `transparentWords`. From `rawheavy.go` and `lossypipe.go`: the
modeled-or-text choice in `rawAdvice` and `advisedSegment`. The helpers other
files still call (`byteAt`, `nameStart`, `nameEnd`) move to the file that
calls them.

### Recorded verdicts

Before the replacement, a test records HEAD's verdict of each guard and of the
identity check for every line of a fixed corpus into a committed data file.
After it, the same test holds the new hook to that file.

| Field | Type | Description |
|-------|------|-------------|
| `line` | string | The command, as an agent types it |
| `caller` | string | `main`, `subagent`, or `fork` for a line a source test judged as a fork; a subagent row is judged without the identity line |
| `refused-by` | list of strings | The checks that refuse the line at HEAD, by name; empty when it passes |
| `reader-line` | boolean | Hand-set to true only on a row this spec loosens; such a row names the test that proves its commands are readers. It waives only the recorded refusals of the checks a reader line can lift (those whose code reaches the reader-set judgment); every other recorded refusal still holds |
| `proof` | string | Present only with `reader-line`: the name of a test, listed in `corpusSourceTests` or `readerLineProofs`, that judged this exact line and passed while the recorder ran |

The corpus: the 83 bash parity rows; the command of every Bash fixture site;
the rows of `TestGuardsKeepTheTextVerdictOfHead`,
`TestGuardsKeepTheTextVerdictBesideAReaderItsJudgeRefuses`,
`TestGuardsJudgeALineThatCanRunAStringByItsText`,
`TestGuardsKeepTheTextVerdictOutsideTheModeledSyntax`,
`TestGuardsKeepTheTextVerdictWhereTheSplitMovesAWord`,
`TestGuardsSeeACommandHandedToAShell`, the `TestTestDeletion` tests and the
`session_identity_test.go` tables; and every construct row this spec names.
The rule the test enforces: a check that refused a row at HEAD refuses it
now, unless the row is marked a reader line and the check is one a reader
line can lift. A new refusal is allowed. One class of row is recorded after its
fix rather than from HEAD: the after-line rows `set -o pipefail` and its
siblings, which AC-31 loosens on purpose and which name that criterion.

### Parity golden rows

No existing bash row changes its expected code. Nine rows join, 83 become 92.
From AC-36 on, the golden is judged by the hook itself, so each row proves its
verdict; the unit test in the last column proves the mechanism behind it. Each verdict "at HEAD" below is derived by
reading the text judgment and is confirmed when the recorded table is written.

| Row | HEAD | After | Direction | Proof |
|-----|------|-------|-----------|-------|
| `for f in a b; do grep -n "x\|go vet ./y" "$f"; done` | 2 (raw job) | 0 | looser | `TestReaderLineReachesThroughConstructs`, journal row |
| `while read -r f; do grep -c 'x;go test ./y' "$f"; done < list` | 2 (raw job) | 0 | looser | same test |
| `for f in a b; do grep -c 'x; go test ./y \| tail -1' "$f"; done` | 2 (piped) | 0 | looser | same test |
| `for f in a b; do grep -n 'go build ./cmd/ze' "$f"; done` | 2 (build) | 0 | looser | same test |
| `for f in a b; do grep -n 'cat /tmp/x' "$f"; done` | 2 (`/tmp`) | 0 | looser | same test |
| `for p in a b; do go test ./internal/$p; done` | 0 | 2 | stricter | `TestHeavyJobIsRefusedWhereverTheLineRunsIt` |
| `if true; then go vet ./internal/x; fi` | 0 | 2 | stricter | same test |
| `( go test ./internal/x )` | 0 | 2 | stricter | same test |
| `for p in a b; do ./le test unit all \| tail -3; done` | 0 | 2 | stricter | `TestLossyPipeIsRefusedInsideAConstruct` |

Each looser row has a fixture site that runs the real hook as a subagent
without the identity line and expects exit 0. Passing there is the reader
proof: the exemption and the lift ask the same predicate.

### Test changes

Tests that pin the byte allow-list change their expectation, because the
construct they refuse is now read. Each goes to an independent `ze-test-judge`
before the edit, with this spec's acceptance criterion as the source that
shows the old expectation superseded:
`TestSubagentLineOutsideTheModeledSyntaxNeedsTheIdentityLine`,
`TestUnmodeledSyntaxNamesEveryConstructOutsideTheAllowList`,
`TestLineCommandsKeepsABackslashBashKeepsInDoubleQuotes`,
`TestSubagentRedirectionBeforeAWordNeedsTheIdentityLine`,
`TestSubagentEmptyQuotedWordNeedsTheIdentityLine`,
`TestGuardsKeepTheTextVerdictOutsideTheModeledSyntax`,
`TestGuardsKeepTheTextVerdictWhereTheSplitMovesAWord`, and the tests that
assert the raw-job `Use:` line. A row of those tests that holds a non-reader
keeps its expectation: only rows whose every command is a reader move. Rows
added to the identity tests (AC-27) lower nothing and need no judge.

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | `mvdan.cc/sh/v3/syntax` imports only the standard library, so the vendor tree holds one package | The author's knowledge of the module; not read here, the module is not in the local cache | More packages are vendored and each import needs its ledger row | Phase 1 reads the import blocks of the vendored package | unvalidated |
| A-2 | The parser reads the constructs of the reader table the way bash does | It is the parser behind shfmt | A line bash runs differently is judged a reader line | `TestWordValuesMatchBash` and `TestParsedCommandsMatchBash` run each corpus line through bash and compare the commands and words | unvalidated |
| A-3 | The Bash tool runs bash | The hand splitter assumed the same; the development host is Linux | On a shell with another grammar the predicate judges a line the shell reads differently | Phase 2 records the shell of one real Bash call into scratch | validated (2026-11-11, phase 2 probe record A-3): the harness runs the command as `/bin/bash -c 'source <shell snapshot> ... && shopt -u extglob ... && eval '<command>' < /dev/null && pwd -P >\| <file>'`, `/proc/$$/exe` is `/usr/bin/bash`, `BASH_VERSION=5.2.21(1)-release`; the snapshot defines `grep` and `find` as functions over ugrep and bfs, which the `readerCommands` comment records |
| A-4 | One parse per call costs far less than the fork that starts the hook | A command line is short | Hook latency grows on every Bash call | `BenchmarkHookParse` reports allocations per operation; the main thread reads them | validated: 26 and 30 allocations, about 6 KB, per parse (BenchmarkHookParse, 2026-11-11); a fork and exec of le costs far more |
| A-5 | No reader reads `ZE_AGENT_ID` or `CLAUDE_CODE_SESSION_ID` | Child 1, A-3, validated at closure by its census | The identity line in front of readers changes what they do | The census grep, re-run in phase 3 | unvalidated |
| A-6 | The lines named "0 at HEAD" in the parity table pass today | Derived by reading `rawHeavy` and `programWords` | A row is not a tightening, only a confirmation | The recorded table, written from HEAD in phase 2 | unvalidated |
| A-7 | A vendor commit whose module has no importer yet passes the gates a commit meets | `bfc6dfe868` landed that way | Phase 1 and phase 3's first commit must land together, which the owner rule forbids | Phase 1 itself | unvalidated |
| A-8 | The harness sets `CLAUDE_CODE_FORK_SUBAGENT` only in a fork's hook environment | `plan/journal/fixture-encodes-an-impossible-state.md`, 2026-08-16: a fixture inherited the variable from the invoking harness, which shows a process can carry it without being a fork. Not observed for a main thread | The main thread is read as a fork and asked for an identity line: it fails closed, in every session | The phase 2 probe: a fork agent and the main thread each record the payload fields and that variable as their hook sees them | validated (2026-11-11, AC-28 probe record) as "the main thread's environment does not carry the variable", so the failure this row fears does not occur. The stated basis is not observed either: a fork's environment does not carry it, and a fork is told apart by its payload, which carries `agent_id`. Both observations are of the Bash shell, which inherits the harness process environment the hook inherits; the hook's own environment was not read directly |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The predicate admits a construct that can run something other than a reader | A subagent's `le` call runs without the identity line | The node table is an allow-list; every row of it has a refusing sibling in `TestReaderLineRefusesWhatCanRunAnythingElse`; strongest-tier review of phase 3 |
| R-2 | The parser panics or loops on a hostile line and the hook fails open through `Run` | A command passes every guard | The parse call recovers a panic and answers "did not parse"; `FuzzShellParse` |
| R-3 | A verdict loosens on a line that is not a reader line | A piped `go test` passes | The text judgment is untouched code over untouched input; the recorded table refuses any lost refusal |
| R-4 | The added tree judgment refuses a line agents use correctly | Refusals of `./le job run ... command go test` or of a reader line | The tree judgment reads command words only, never an argument of another command; the two survivors are rows of the recorded table |
| R-5 | The word-value reading disagrees with bash | A judge admits an option bash does not pass | `TestWordValuesMatchBash`; a word whose value the reading cannot give (ANSI-C quoting) is outside the reader line |
| R-6 | The expectation changes lower what the tests prove | The judge refuses | Each change goes to `ze-test-judge`; a refusal stands and the row keeps its old expectation |
| R-7 | Upstream changes the grammar in a later release | A re-vendor changes a verdict | The module is vendored at one version; the recorded table and the bash comparison run on every hook package test |
| R-8 | The phase 3 package is too large for one agent | The agent reaches its call budget mid-replacement | The deletion and the replacement are one commit by rule; a continuation agent carries the same package, and the main thread re-cuts only at a compiling boundary |
| R-9 | A wrapper's value-taking option is missing from its declared list | An environment tool behind it passes | `TestEnvironmentToolIsFoundBehindEveryWrapper` holds one row per wrapper option, and the review reads each wrapper's manual against the list |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Every session at once. Too strict: reader lines are refused and every agent pays a turn per call. Too loose: a subagent's `le` call acts as the main thread, or a heavy job runs unadmitted |
| How is it reverted? | `./le rollback` moves the build aside at once. Each phase is one commit or a short run of commits, reverted by a script prepared for the owner. The vendor commit can stay: nothing imports it once the code is reverted |
| Who else touches this path? | `plan/post-release/engineering/spec-agent-economy-4-supervision.md` edits `lifecycle.go` and not the splitter. The three text-judgement rows, the parity golden row and the `shapeFinding` row moved here from `plan/post-release/engineering/spec-tooling-one-off-defects.md` (AC-33 to AC-37), so no other spec edits these guards, the golden or `writers.go` for them |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `le ai hooks pretool-bash`, subagent payload, a `for` loop of readers with no identity line | → | the parse and the reader-line predicate, through `Run` | `TestSubagentReaderLoopPassesUnchanged` |
| `le ai hooks pretool-bash`, main payload, a `for` loop of raw `go test` | → | the tree judgment of `bashRawHeavy`, through `Run` | `TestHeavyJobIsRefusedWhereverTheLineRunsIt` |
| `le ai hooks pretool-bash`, a line with an unclosed quote | → | the parse failure path, through `Run` | `TestUnparsedLineKeepsTheTextVerdictAndNeedsTheIdentityLine` |
| `le ai hooks pretool-bash`, subagent payload, the agent's line then a reader loop holding a quoted `go vet` | → | the guards' reader-line question after the identity line, through `Run` | `TestIdentityLineBeforeAReaderLineStillLifts` |
| `le ai hooks pretool-bash`, subagent payload, the agent's line then `time . x` | → | the after-line walk, through `Run` | `TestEnvironmentToolIsFoundBehindEveryWrapper` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | The phase 1 commit | It carries `go.mod`, `go.sum` and the `vendor/` tree of `mvdan.cc/sh/v3` at its latest stable release, with `vendor/modules.txt`, and builds | No Go file of Ze, no ledger row and no spec edit ride in it: the owner rule gives a new module a commit of its own |
| AC-2 | The commit that first imports the parser | It carries the `dependency-ledger.txt` row for `mvdan.cc/sh/v3/syntax` in `internal/le/hookruntime` that Thomas wrote with `./le deps approve`, and the commit gate admits it | No other package of the module and no other importer: an approval covers its approved use only |
| AC-3 | The hook package after phase 3 | One function parses a command line, the context holds its result for the whole hook call, and every check that reads commands reads that result | `commandSegments`, `shellWords`, `programWords` and the guards' patterns stay: they are the text verdict of a line no parse proves to be readers |
| AC-4 | A search of `internal/le/hookruntime` for `lineCommands`, `lineWords`, `modeledSyntax`, `unmodeledSyntax`, `syntaxCommand` and `rawWords` | Each name is gone from the code, the tests and the two pages that described it, and `shellsyntax.go` no longer exists | The word-value reading stays: it reads one word the parser delimited, and a test compares it with bash |
| AC-5 | A subagent line with no identity line: `for f in a b; do grep -n "x\|go vet ./y" "$f"; done` | The hook exits 0 with empty standard output, and no guard refuses it | The same loop with `go vet ./y` as a command of its body is refused (AC-9): it runs the vet |
| AC-6 | Reader lines through each admitted construct: `if`, `while read -r`, `until`, `case`, `[[ -f "$f" ]]`, a group, a subshell, `!`, `time`, a `$(...)` of readers, a here-document fed to a reader, `diff <(sort a) <(sort b)` | Each passes as typed for a subagent and lifts every guard's text refusal | A construct whose body holds one command outside the reader set keeps the text verdict and needs the line: the line can then run anything |
| AC-7 | Reader lines the byte allow-list or the splitter refused: a quoted `#`, a brace that is text, a `!` inside quotes, a backslash outside quotes, a UTF-8 pattern, an empty quoted word, `grep -n "foo$" f`, a redirection before the words, `[ -f x ] 2>/dev/null && cat x`, `sort f \| uniq -c 2>/dev/null` | Each passes as typed for a subagent | A string in ANSI-C or locale quoting keeps the line outside, and its refusal says so in those words: its value is not read |
| AC-8 | A line holding a node outside the table: a function definition, `export` or `declare`, `let` or an arithmetic expansion, an array index or a slice, a background `&`, a coprocess, an assignment in front of a reader, a bare assignment to an upper-case or exported name, `[[ a -eq b ]]`, `read` with an option other than `-r` | A subagent is refused with the identity line to type, and the message names the construct in plain words; each guard gives its text verdict | With the agent's line in front it passes the identity check as today: the line is what such a command needs |
| AC-9 | A heavy job that is a command of the line, behind a keyword or inside a group, subshell, negation or substitution: `for p in a b; do go test ./$p; done`, `if true; then go vet ./x; fi`, `( go test ./x )`, `! golangci-lint run`, `x=$(go test ./x)` | Refused as a raw job, with the admitted form of that command | `./le job run label unit-pkg quiet command go test ./x` passes: `go test` there is an argument of `le`. A real `ZE_ADMIT_RAW="<reason>"` assignment in front of the job passes it, as today |
| AC-10 | An expensive command piped through a lossy filter inside a construct: `for p in a b; do ./le test unit all \| tail -3; done` | Refused as a lossy pipe | A reader piped through a filter inside the loop passes: no reader is expensive |
| AC-11 | Every row of the recorded table | Each check that refused the row at HEAD refuses it now | A row marked a reader line passes, with the test that proves its commands are readers named in the row; the `set` rows of AC-31 pass and name that criterion. A new refusal is allowed: stricter is permitted |
| AC-12 | A line that does not parse as bash: an unclosed quote, an unterminated here-document, a stray `fi` | It is no reader line: a subagent needs the identity line, every guard gives its text verdict, and the deletion guard refuses it when its text runs `rm` | All of HEAD's refusals on such a line: a hook that cannot read a line cannot clear it |
| AC-13 | A parser that panics on its input | The parse answers "did not parse", the call is judged as in AC-12, and the hook reports no failure of its own | `Run` still fails open on a panic anywhere else: that rule protects every session from a broken check |
| AC-14 | A subagent line that starts with its exact identity line, followed by a reader line holding a quoted `go vet`, `go build` or `/tmp` path | Every guard lifts, as it would without the line in front | A line with another agent's identity line, or one that sets either variable after the line, is refused by the identity check as today |
| AC-15 | A refused piped or raw command on a line that parses, whatever else the line holds: `go test -run 'TestA\|TestB' ./x \| tail -5` after a quoted `#`, or inside a group | The advised command holds the refused command's own words, each written back as one shell word | On a line that does not parse the advice is built from the text segment, as today: there is nothing else to read |
| AC-16 | A build on a reader+build line whose other commands are reader constructs, and a redirection written before the build's words | Judged by its words, with the verdicts `judgeGoBuild` gives today | A build inside a construct, behind any assignment but `GOFLAGS`, with an expanded word or with `-toolexec`, `-overlay` or `-modfile` keeps the text judgment; a `cd` anywhere but alone at the top level, or leftmost in a top-level `&&` chain, leaves the directory unknown |
| AC-17 | The `/tmp` guard on a reader line with constructs: `for f in /tmp/a; do cat "$f"; done` | Refused: the guard judges every word of the tree, loop lists, case words, assignment values and redirection targets included | A here-document body and a here-string are data and are not judged; a `/tmp` under a literal parent passes as today |
| AC-18 | The deletion guard: `rm -f scratch && grep x internal/a_test.go`, and every line-wide row of the recorded table | The plain-`rm` line passes on the `rm` arguments alone; every line-wide refusal of HEAD stands | An `rm` inside a construct, with an expanded word, or beside a `cd` keeps the line-wide judgment |
| AC-19 | After the agent's own line: `export X=1`, `declare -x X`, `env X=1 ./le x`, a command naming either identity variable outside an admitted reader | Refused as taking another identity, by the node's kind and the command word | `FOO=bar ./le x`, `cd x && ./le x`, `grep ZE_AGENT_ID f` pass, as today |
| AC-20 | A raw `go test` or `go vet` the short action cannot express, and a raw `golangci-lint run` | The `Use:` line shows `./le job run label <label> quiet command <argv>`, and the generic-form line carries `quiet` too | The short package action is still advised where it expresses the words exactly: it is quiet by itself |
| AC-21 | `rawHeavy` and `programWords` on a segment with a launcher | The numeric-argument pattern is compiled once, at package load, and both read it | The patterns `bashLossyPipe`, `bashPollLoop` and `bashTestDeletion` compile per call stay: they are outside the two launcher loops, and one compile per hook call is not a loop |
| AC-22 | The doc comment above `TestSubagentStartCarriesTheIdentityLine`, and the sentence of `docs/contributing/running-commands.md` that says a start payload with no agent id gets "a note that the first refusal prints the rest" | Both say what `hookSubagentContext` writes and the test asserts: with no agent id the start text carries the session part alone, which is the whole line, and the words "first refusal" are absent | The test body is unchanged |
| AC-23 | The open row of 2026-11-11 in `plan/journal/hook-existing-patterns-false-positive.md` (`rawHeavy` on a `for` loop) | This spec's Owned journal rows table lists it and the table of `plan/post-release/engineering/spec-tooling-one-off-defects.md` no longer does, both in the commit that lands this spec; the row's Fix cell is closed by the commit that proves AC-5 | None. The five rows that moved here from that spec on Thomas's decision (2026-11-11) are owned here too, and each is closed by the commit that proves its AC: the three `bash-guard-misses-a-command-shape` rows by AC-33, AC-34 and AC-35, the parity-golden row by AC-36, the `shapeFinding` row by AC-37 |
| AC-24 | `./le ai hooks unit` after the last code phase | Green: `hookSourcesDigest` and `parityCatalogDigest` are each re-derived once, with a dated comment naming this spec, and the bash rows are 92 | Commits before it name the stale seal in their body: a red holds no commit |
| AC-25 | `.claude/hooks/README.md`, `docs/contributing/running-commands.md`, `plan/learned/HOOK-FRICTION.md` and the subagent start text | Each states the reader line as the parser gives it, with the table of admitted constructs, and its source anchors name files that exist | A sentence that was already true stays as written |
| AC-26 | After the agent's own line, a `.` behind a wrapper or a reserved word: `time . ./env.sh; ./le x`, `if true; then . ./env.sh; fi; ./le x`, `command . x`, `builtin . x` | Each is refused as taking another identity: a word the walk skips as a duration holds a digit, so a lone `.` is the command word | `timeout 5 ./le x`, `timeout 1.5s ./le x` and `nice -n 5 ./le x` pass: `5` and `1.5s` hold a digit and `./le` is no environment tool |
| AC-27 | The identity tests, with rows that carry no literal identity variable name: `source x`, `. x`, `set -a`, `eval x`, `typeset -x A`, `readonly A`, `local A`, `command export A=1`, `if true; then export A=1; fi`, `timeout 5 env -i le x`, `time . x`, and an environment tool behind each of `if`, `elif`, `else`, `do`, `while`, `until`, `nohup`, `timeout`, `xargs` and `stdbuf` | Every row is refused as taking another identity, so each form is proven by its form and not by the mention rule | The existing rows that carry a literal name stay: they prove the mention rule |
| AC-28 | A probe in which a fork agent and the main thread each record, into scratch, the payload fields their pre-tool hook receives and whether its environment holds `CLAUDE_CODE_FORK_SUBAGENT` | The record is kept in the spec's closure sections and settles A-8. When it shows the variable in the main thread's hook, `bashAgentIdentity` and `commandIdentityPrefix` read a fork from the payload field the record shows only a fork carries, and a main-thread command passes with no identity line while that variable is set | When the record shows the variable only in a fork's hook, the hook stays as it is: the environment is then the harness's own signal. When no payload field tells a fork apart, the finding goes to Thomas with the record. The two `TranscriptPath` readers are not changed here: they hold no payload |
| AC-29 | After the agent's own line: `timeout -s KILL 60 env -i ./le x`, `/usr/bin/env -i ./le x`, `nice -n 5 /usr/bin/sudo le x`, `stdbuf -o L env -i le x` | Each is refused as taking another identity: a wrapper's value-taking option takes one value, and a tool is matched by its base name | `timeout -s KILL 60 ./le x` passes: the command behind the wrapper is `./le` |
| AC-30 | A subagent line with no identity line whose refusal names a construct | The words name the construct bash would read there, and no message names a construct the line does not hold; `grep -n "foo$" f` is not refused at all (AC-7) | The refusal still ends with the identity line to type |
| AC-31 | After the agent's own line: `set -o pipefail; ./le x`, `set -e`, `set -eu`, `set -x`, `set +e` | Each passes the identity check: these options change no variable | `set -a`, `set -k`, `set -- a b`, `set -o allexport` and any other `set` are refused as today: they export later assignments or replace the positional parameters |
| AC-32 | The comment of `readerCommands` | It says the nil judges assume the GNU option sets of grep and coreutils, and names what a host with another grep changes: a judge is owed before the nil entry is trusted there | The table's entries are unchanged |
| AC-33 | The text judgement of `bashLossyPipe` and `bashRawHeavy` (`programWords`, `rawHeavy`) on a launcher with an option that takes a value: `bash -o pipefail -c './le test unit all \| tail -5'`, `bash -O extglob -c 'go test ./x'`, `timeout -s KILL 60 go test ./x`, `nice -n 5 go test ./x`, `sudo -u u go test ./x` | Each gets the verdict the same line gets without the option and its value. The launcher loops skip a value-taking option together with its one value. Both loops read one declared table of each launcher's value-taking options (journal row `bash-guard-misses-a-command-shape`, 2026-11-11, launcher option value) | A launcher option that takes no value is skipped alone, as today. The recorded table holds: the fix only adds refusals |
| AC-34 | `bashRootBuild`'s text judgement (`goBuildText`) on `go build -toolexec=x -o bin/y ./cmd/y`, `go build -overlay=o.json -o bin/y ./cmd/y`, `go build -modfile=m -o bin/y ./cmd/y`, the spaced forms (`-toolexec x`), and the same flags in a `GOFLAGS` assignment | Refused. The message names the flag, says it makes the toolchain run or read what the line names, and gives the `go build -o bin/<name> ./cmd/<name>` form (journal row `bash-guard-misses-a-command-shape`, 2026-11-11, `-toolexec`) | `go build -o bin/ze ./cmd/ze` passes. A reader line whose quoted pattern names `-toolexec` still lifts: the lift belongs to the reader line |
| AC-35 | `bashSystemTmp`'s text pattern `systemTmp` on `cat ${v}/tmp`, `cat ${v}/tmp/x` and `ls ${HOME}/tmp`, unquoted | Refused. `}` is in the pattern's boundary class, so a parent the shell can expand to nothing counts as a boundary before `/tmp` (journal row `bash-guard-misses-a-command-shape`, 2026-11-11, `}` boundary) | `cat x/tmp/y` and `cat tmp/x` pass, as today: a literal parent keeps the path inside it |
| AC-36 | `./le ai hooks unit` over the bash parity golden (`parity_catalog.go` BASH_GOLDEN) | Each row is judged by `hookruntime`'s own `pretool-bash` action over a main-thread payload, through an exported entry `aihooks` calls, and its expected code is compared with that verdict. `bashCode`, and the regular expressions only it reads, are deleted (no layering). Breaking `bashRootBuild` reddens the `go build ./...` row (journal row `green-that-could-not-have-been-red`, 2026-11-11) | A row whose expected code disagrees with the hook is either a hook defect, which is fixed, or a wrong row, which changes only with a `ze-test-judge` approval. The nine rows of the Parity golden rows table then prove their verdict through the hook |
| AC-37 | The error inventory (`internal/le/arch/errorinventory/writers.go`, `shapeFinding`) on a `Finding` composite literal under `internal/le` that sets no message field, such as `Finding{Path: p, Package: k, Name: n}` | It is not judged a message writer, and the inventory reports no plain message with text `""` for it. The message fields are declared once, beside `shapeFinding` (journal row `hook-existing-patterns-false-positive`, 2026-11-11, `shapeFinding`). Test: `TestIdentityOnlyFindingIsNotAMessage` in that package | A `Finding` literal that sets a message field is judged a message writer, as today |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestGuardVerdictsOfHeadStand` | `internal/le/hookruntime/verdicts_test.go` | AC-11: written first, from HEAD; then holds the new hook to the recorded table | |
| `TestReaderLineRowWaivesOnlyTheChecksAReaderLineLifts` | `internal/le/hookruntime/verdicts_test.go` | AC-11: a reader-line mark keeps every refusal of a check a reader line cannot lift, and its proof judged that exact line | |
| `TestReaderLiftableChecksAreTheReaderGuards` | `internal/le/hookruntime/verdicts_test.go` | AC-11: the liftable set derived from the source holds the reader guards and none of the git, poll-loop, scratch or governed-write guards | |
| `TestSubagentReaderLoopPassesUnchanged` | `internal/le/hookruntime/session_identity_test.go` | AC-5 through `Run` | |
| `TestReaderLineReachesThroughConstructs` | `internal/le/hookruntime/readerline_test.go` | AC-6, AC-7, one row per construct, each naming the commands the predicate admitted | |
| `TestReaderLineRefusesWhatCanRunAnythingElse` | `internal/le/hookruntime/readerline_test.go` | AC-8, AC-30, one row per node outside the table, each asserting the named construct | |
| `TestHeavyJobIsRefusedWhereverTheLineRunsIt` | `internal/le/hookruntime/rawheavy_test.go` | AC-9, with both survivors as rows | |
| `TestLossyPipeIsRefusedInsideAConstruct` | `internal/le/hookruntime/bash_test.go` | AC-10 | |
| `TestUnparsedLineKeepsTheTextVerdictAndNeedsTheIdentityLine` | `internal/le/hookruntime/shellparse_test.go` | AC-12 through `Run` | |
| `TestParserPanicIsAnUnparsedLine` | `internal/le/hookruntime/shellparse_test.go` | AC-13, with a parse function that panics | |
| `TestIdentityLineBeforeAReaderLineStillLifts` | `internal/le/hookruntime/session_identity_test.go` | AC-14 through `Run` | |
| `TestRefusalAdvisesTheParsedCommand` | `internal/le/hookruntime/refusal_messages_test.go` | AC-15 | |
| `TestRootBuildReadsTheBuildFromTheTree` | `internal/le/hookruntime/bash_test.go` | AC-16 | |
| `TestSystemTmpJudgesEveryWordOfAReaderLine` | `internal/le/hookruntime/bash_test.go` | AC-17 | |
| `TestTestDeletionReadsThePlainRmFromTheTree` | `internal/le/hookruntime/bash_test.go` | AC-18 | |
| `TestAnotherIdentityIsJudgedByTheNode` | `internal/le/hookruntime/session_identity_test.go` | AC-19 | |
| `TestRawRefusalAdvisesTheQuietForm` | `internal/le/hookruntime/rawheavy_test.go` | AC-20 | |
| `TestSubagentChangingTheEnvironmentAfterItsLineIsRefused` (rows added) | `internal/le/hookruntime/session_identity_test.go` | AC-27: the eleven rows and the ten keyword and wrapper rows, none naming an identity variable | |
| `TestEnvironmentToolIsFoundBehindEveryWrapper` | `internal/le/hookruntime/session_identity_test.go` | AC-26, AC-29, through `Run`, one row per wrapper and per value-taking option, with the passing survivors | |
| `TestSetOptionsThatChangeNoVariablePass` | `internal/le/hookruntime/session_identity_test.go` | AC-31, both directions | |
| `TestMainThreadIsNeverReadAsAFork` | `internal/le/hookruntime/session_identity_test.go` | AC-28, written when the probe shows the variable in the main thread's hook: a main payload with the variable set passes | |
| `TestWordValuesMatchBash` | `internal/le/hookruntime/shellparse_test.go` | A-2, R-5: for each corpus line with no expansion, the words bash passes equal the values the hook reads | |
| `TestParsedCommandsMatchBash` | `internal/le/hookruntime/shellparse_test.go` | A-2: for each corpus line of readers, the command words bash runs equal the ones the tree yields | |
| `TestHandSplitterIsGone` | `internal/le/hookruntime/shellparse_test.go` | AC-3, AC-4: the package declares none of the deleted names, and one file imports the parser | |
| `FuzzShellParse` | `internal/le/hookruntime/shellparse_test.go` | R-2: no input makes the parse call panic or the predicate answer yes for a line holding a command outside the reader set | |
| `BenchmarkHookParse` | `internal/le/hookruntime/shellparse_test.go` | A-4: allocations per parse of a typical line | |
| `TestLauncherValueOptionsAreSkippedWithTheirValue` | `internal/le/hookruntime/rawheavy_test.go` | AC-33, both loops, with the no-value survivor | |
| `TestRootBuildRefusesToolchainFlagsByText` | `internal/le/hookruntime/bash_test.go` | AC-34, with the `-o bin/` survivor | |
| `TestSystemTmpTakesABraceAsABoundary` | `internal/le/hookruntime/bash_test.go` | AC-35, with the literal-parent survivors | |
| `TestBashGoldenIsJudgedByTheHook` | `internal/le/ai/hooks/parity_test.go` | AC-36: a row's code is the hook's verdict | |
| `TestIdentityOnlyFindingIsNotAMessage` | `internal/le/arch/errorinventory/` | AC-37, with the message-field survivor; the `shapeFinding` row of `docs/contributing/error-inventory.md` (the `shape` field table) is reworded in the same commit to say a literal counts only when it sets a message field | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| none | N/A | N/A | N/A | N/A |

The feature takes no numeric input.

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `./le ai hooks unit` | `internal/le/ai/hooks/fixtures.go`, `parity_catalog.go` | Every golden row and behavioral site, with a site per looser parity row that runs the hook as a subagent with no identity line, and the nine parity rows, every bash row judged by the hook itself (AC-36) | |
| `./le test functional runner` | `test/runner/` | Unchanged: the runner primitives still pass with the hook package rebuilt | |
| Live probe | a research agent spawned after the phase 3 build of HEAD is published | Its reader loops and a quoted `#` run without the identity line, and a loop of raw `go test` is refused with the admitted form | |
| Fork probe | a fork agent and the main thread, phase 2 | AC-28's record | |

## Files to Modify
- `go.mod`, `go.sum`, `vendor/modules.txt` - the module (phase 1)
- `dependency-ledger.txt` - one row, written by `./le deps approve`
- `internal/le/hookruntime/shellline.go` - the splitter goes; `readerLine` asks the predicate
- `internal/le/hookruntime/readers.go` - `read` joins the table; the comment states the GNU assumption
- `internal/le/hookruntime/session.go` - `commandOutsideReaders`, `takesAnotherIdentity`, `remainderChangesEnvironment`, `changesEnvironment` and its two tables; `rawWords` goes; the fork test, if the probe says so
- `internal/le/hookruntime/bash.go` - `bashLossyPipe`, `bashRawHeavy`, `bashTestDeletion` ask the tree beside their text verdict; `programWords` reads a hoisted pattern
- `internal/le/hookruntime/rawheavy.go` - the quiet form, the hoisted pattern, advice from parsed words
- `internal/le/hookruntime/lossypipe.go` - advice from parsed words
- `internal/le/hookruntime/gobuild.go` - `goBuildLine` reads the tree
- `internal/le/hookruntime/systemtmp.go`, `testdeletion.go` - read the tree
- `internal/le/hookruntime/runtime.go` - the context holds the parse
- `internal/le/hookruntime/lifecycle.go` - the start text's sentence on readers, if AC-25 finds it wrong
- `internal/le/hookruntime/lifecycle_test.go` - the stale comment (AC-22)
- `internal/le/hookruntime/bash_test.go`, `session_identity_test.go`, `rawheavy_test.go`, `refusal_messages_test.go` - the expectation changes the judge approves, and the new tests and rows
- `internal/le/ai/hooks/parity_catalog.go`, `parity.go`, `fixtures.go`, `fixture_catalog.go` - rows, the golden judged by the hook with `bashCode` deleted (AC-36), sites, seals
- `internal/le/arch/errorinventory/writers.go` - `shapeFinding` judges only a literal that sets a message field (AC-37)
- `.claude/hooks/README.md`, `docs/contributing/running-commands.md`, `plan/learned/HOOK-FRICTION.md` - the pages
- `plan/journal/hook-existing-patterns-false-positive.md` - the row's Fix cell
- `plan/post-release/engineering/spec-tooling-one-off-defects.md` - its owned-rows table loses the row
- `plan/post-release/engineering/spec-agent-economy-0-umbrella.md` - the Children table gains child 6

## Files to Create
- `vendor/mvdan.cc/sh/v3/` - the vendored package and its licence (phase 1)
- `internal/le/hookruntime/shellparse.go` - the one parse, the panic containment, the word-value reading
- `internal/le/hookruntime/readerline.go` - the admitted-node table and the predicate
- `internal/le/hookruntime/shellparse_test.go`, `readerline_test.go`, `verdicts_test.go` - the tests above
- `internal/le/hookruntime/testdata/guard-verdicts-head.jsonl` - the recorded table

Deleted: `internal/le/hookruntime/shellsyntax.go`, through `remove` in the commit that replaces it.

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | tooling: no RPC, no config |
| YANG validation constraints | N-A | no leaf |
| YANG custom validators | N-A | no leaf |
| CLI commands/flags | No | no `le` command or flag is added or changed |
| CLI grammar (keyword before value) | N-A | no command |
| Editor autocomplete | N-A | no config |
| Functional test for new RPC/API | N-A | no RPC |
| Pipe completeness | N-A | no command output |
| Env var registration | No | the hook reads its environment for the bare-assignment rule by name lookup only and registers no variable |
| Doctor check for runtime dependencies | N-A | `ze doctor` reports on the shipped binary; the parser is linked into `le` only |
| Doctor code per problem class | N-A | a hook refusal is not a product problem class |
| Prometheus counters/metrics | N-A | no runtime state |
| BGP family surface (new SAFI / capability / attribute) | N-A | no protocol |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | development tooling; no `features/` entry describes the hook |
| 2 | Config syntax changed? | No | none |
| 3 | CLI command added/changed? | No | none |
| 4 | API/RPC added/changed? | No | none |
| 5 | Plugin added/changed? | No | none |
| 6 | Has a user guide page? | Yes | `docs/contributing/running-commands.md`, "Session identity, and what each agent owns": the outcome table, the allow-list paragraph, the after-line paragraph (wrappers, base name, `set`), the stale start-text sentence (AC-22) and the source anchors to `shellline.go` and `shellsyntax.go` |
| 7 | Wire format changed? | No | none |
| 8 | Plugin SDK/protocol changed? | No | none |
| 9 | RFC behavior implemented, changed, or newly proven? | No | none |
| 10 | Test infrastructure changed? | No | the hook golden's count is stated in code comments, not in `docs/functional-tests.md` |
| 11 | Affects daemon comparison? | No | none |
| 12 | Internal architecture changed? | Yes | `.claude/hooks/README.md`: the guard rows, the paragraph on the text judgment and the reader line, the identity row and their source anchors. `docs/contributing/dependency-approval.md` is unaffected: it names no module list |
| 13 | Route metadata keys added/changed? | No | none |
| 14 | Prometheus counters added/changed? | No | none |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | none |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Derived at implementation with `./le spec citation anchors spec plan/post-release/engineering/spec-agent-economy-6-shell-parser.md`. Known from a grep of `docs/`, `ai/` and `.claude/hooks/README.md` for the deleted names: `docs/contributing/running-commands.md` and `.claude/hooks/README.md` only. `docs/architecture/core-design.md` is declared by the `// Design:` header of the hook files and names none of the deleted symbols. `docs/contributing/committing.md` is declared by the header of `lifecycle_test.go` and is unaffected: this spec edits one doc comment in that file (AC-22), which says nothing about committing |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | the example lines of both pages (`cat x{#}; ./le x`, `<<<cat ./le x`) describe splitter faults that no longer exist; they move to `plan/learned/HOOK-FRICTION.md` as history |

## Implementation Steps

The spec itself lands first, in a plan-only commit by the main thread that
also moves the journal row's ownership (AC-23) and adds child 6 to the
umbrella's Children table.

1. **Phase: Vendor, the module alone** -- development tier
   - Work: add `mvdan.cc/sh/v3` at its latest stable release and vendor it. The vendor tree is populated with the import present in the working tree; the commit carries `go.mod`, `go.sum` and `vendor/` only.
   - Verify: A-1 (the import blocks of the vendored package), A-7. The main thread runs the module commands, which need the network, and `./le repo compiles check`.
   - → Done (2026-11-11): `1fdd96d4d6`, `mvdan.cc/sh/v3` v3.14.1. Two packages are vendored: `syntax` and `fileutil`, which `syntax` imports. It was vendored with `./le setup install`, because a bare `go mod vendor` drops the four reviewed vendor patches. The commit also carries the `dependency-ledger.txt` row Thomas approved and the removal of four stale go-abnf fuzz files that the v0.5.4 vendoring no longer writes.
   - → Constraint (A-1, read from the vendored import blocks): A-1 is wrong as worded and its consequence does not follow. `vendor/mvdan.cc/sh/v3/syntax/*.go` import the standard library (bufio, bytes, fmt, io, iter, math, math/bits, reflect, slices, strconv, strings, text/tabwriter, unicode, unicode/utf8) plus `mvdan.cc/sh/v3/fileutil`, and `fileutil/file.go` imports only io/fs, regexp and strings. So the vendor tree holds two packages, not one. No second ledger row is owed: the ledger lists the third-party packages a Ze directory imports, Ze imports `syntax` alone, and `fileutil` is imported only by `syntax`. A-1 is settled as invalidated with no change to the design.
   - Gate before phase 2's import: Thomas runs `./le deps approve package mvdan.cc/sh/v3/syntax importer internal/le/hookruntime reason "bash parser for the pre-tool Bash hook"`.
2. **Phase: Wiring (MANDATORY FIRST of the code)** -- development tier; strongest-tier review
   - → Decision (2026-11-11, main thread): the fixes below (AC-20, AC-21, AC-22, AC-26, AC-27, AC-32) land first, in a commit of their own, and the recorded HEAD-verdict table (`TestGuardVerdictsOfHeadStand`) lands in the next commit, recorded from the tree that commit produces. The fix commit moves two verdicts of the after-line walk (`wrapperNumber`), and neither loses a refusal HEAD gives: a lone `.` after a wrapper is now the command word, so it is refused (AC-26); and a digit-less word made only of `s`, `m`, `h`, `d` and `.` after a wrapper (`timeout s ./le x`), which HEAD skipped as a duration, is now the command word, which is what bash runs. A table recorded after it therefore still holds every refusal HEAD gives.
   - First commit, no parser, the first hook-touching commit (e7e1d95731): the fixes that need no parser, each with its test failing first: the lone `.` (AC-26) and the rows owed (AC-27), because that hole is open in every session today; the quiet form (AC-20); the hoisted pattern (AC-21); the stale comment and page sentence (AC-22); the `readerCommands` comment (AC-32).
   - → Decision (2026-11-11, agent a35909a020a742ffb): the second commit is `internal/le/hookruntime/verdicts_test.go` plus the table its `-args -update` run writes. The corpus is collected, never copied. (1) The listed source tests run as subtests of `TestGuardVerdictsOfHeadStand` while a recorder is on: `guardVerdict` and `runHook` add each line they judge, with the test that judged it, so computed rows are recorded as judged. The list is `corpusSourceTests`: the six named guard tests, every `TestTestDeletion` test of `bash_test.go` (the deletion guard's word list is rewritten in phase 3), and every test of `session_identity_test.go`; `holdSourcesComplete` checks the last two against their files' syntax trees. The fork variable is pinned empty while they run, so a fork row comes from a source that set it, never from the runner's environment. (2) The parity rows and Bash fixture values of `internal/le/ai/hooks` are read with go/ast, because that package imports this one. These are the `name:` of every BASH_GOLDEN fixture, held to the table's `expected:` count, and the `allow` and `refuse` of every `fixtureCategories` row whose `categoryProbes` entry has `tool: "Bash"`. A shape the reading does not model fails the test: a non-literal (a `tool` named by a constant is read from that constant's literal), a `wrap`, a count mismatch, no Bash probe at all. (3) `specConstructLines`, the spec's named rows, without the AC-31 `set` rows, which are recorded after their fix. Every line is judged as `main` and `subagent`, and as `fork` when a source judged it so. Each pretool-bash check is asked alone, in registry order, over `goBuildTree`, with `GOFLAGS`, `CDPATH` and the fork variable fixed. `bashCallBudget` is left out. The plain run is red on a corpus key with no row; on a recorded refusal the check no longer gives, where a `reader-line` row waives only the checks whose code reaches `readerCommand`, the reader-set judgment (`readerLiftableChecks`, derived from the package source: today `bashRootBuild`, `bashLossyPipe`, `bashRawHeavy`, `bashSystemTmp`, `bashTestDeletion`, `bashAgentIdentity`), so a git, poll-loop, scratch or governed-write refusal holds on every row; on a duplicate key; on a `reader-line` whose `proof` is not a test of `corpusSourceTests` or `readerLineProofs` that judged that exact line and passed; and on a table holding fewer rows than `verdictsTableRowsMin`, the count its last update left, so a table cut short cannot be re-recorded from the tree in hand; an update run over a missing table is refused, because the table is committed. `-update` appends rows for missing keys only, through `mergeVerdictRows`, which keeps every recorded row as it stands (`TestVerdictTableUpdateKeepsEveryRecordedRow`), and then runs the same judgment.
   - Pre-parser commits after the recorded table, each with its test failing first: the text-judgement tightenings AC-33 (launcher value-taking options), AC-34 (`-toolexec`, `-overlay`, `-modfile` in `goBuildText`) and AC-35 (`}` in `systemTmp`), in one hook commit; AC-37 (`shapeFinding`) in a commit of its own. Every one only adds refusals, so `TestGuardVerdictsOfHeadStand` stays green; the update run then appends their rows.
   - The fork probe (AC-28) runs here; its record decides whether the hook's fork test changes, in a commit of its own.
   - → Probe record, AC-28 (2026-11-11), the main-thread side only: `CLAUDE_CODE_FORK_SUBAGENT` is unset in the main thread's environment, and the pre-tool hook inherits that environment, so the main thread's hook does not see the variable. The fork side (2026-11-11): a fork agent of this session ran `./le spec current` with no identity line, and the pre-tool hook refused it and asked for the full line with an agent id (`export ZE_AGENT_ID=<the fork's agent id>; export CLAUDE_CODE_SESSION_ID=<session>;`), so a fork's payload carries `agent_id` and the hook judges a fork as a subagent. The fork's Bash shell environment does not hold `CLAUDE_CODE_FORK_SUBAGENT` (`fork=unset`). Outcome under AC-28: neither branch that changes the hook applies, because the variable is not in the main thread's hook environment and a payload field, `agent_id`, does tell a fork apart. `bashAgentIdentity` and `commandIdentityPrefix` stay as they are, and `TestMainThreadIsNeverReadAsAFork` is not written. A fork of this harness never reaches the hook's no-agent-id fork branch. Caveat: the hook's own environment was not read directly; both observations are of the Bash shell, which inherits the same harness process environment the hook does. Side observation: the fork's reader line `echo "fork=${CLAUDE_CODE_FORK_SUBAGENT-unset} ..."` was refused for the `${...}` default operator, the false refusal phase 3 removes. The record of both sides still goes into the closure sections.
   - Second commit: `shellparse.go` with the parse, the panic containment and the word-value reading; the wiring tests of the table above fail; the bash comparison tests and the fuzz target pass over the corpus. The parse has one production reader from this commit: `commandOutsideReaders` names a line that does not parse from it.
   - → Decision (2026-11-11, agent a78cddc7144618861), the parse step (AC-3, AC-12, AC-13): the parser is reached through an injectable seam, `parseBashLine` (`shellparse.go`), so a test can put a panicking parser in its place, and `parseCommandLine` recovers in line: a parser panic is answered as a line that did not parse, never as Run's own recover, which would answer exit 0 and let an unread line pass every check. One parse cache serves the whole hook call: `newContext` gives the context a shared `*shellParse`, every check's copy of the context holds the same pointer, and the first check that asks (`ctx.commandParse`) parses the line once. `commandOutsideReaders(command, parse)` asks the not-parsed reason last, after every reason the splitter already gives, so no refusal message that exists today changes. The guards' half of AC-12 (a line that does not parse keeps each guard's text verdict) goes to phase 3, where the guards read the tree. Tests: `TestCommandIsParsedOncePerHookCall`, `TestUnparsedLineKeepsTheTextVerdictAndNeedsTheIdentityLine`, `TestParserPanicIsAnUnparsedLine` (`shellparse_test.go`). What this commit still owes before phase 2 closes: the word-value reading, `TestWordValuesMatchBash`, `TestParsedCommandsMatchBash`, `FuzzShellParse`, `BenchmarkHookParse`, and the A-3 record.
   - → Decision (2026-11-11, agent af284921fd61e9998), the word-value reading (R-5): one function, `readWord(*syntax.Word) (wordValue, error)` in `shellparse.go`. `wordValue` holds the text with quoting removed and `expands`; the text is the argument bash passes only when `expands` is false. A word whose value cannot be read answers a `wordNotRead` error ("its value is not read: ..."), never an empty text: ANSI-C `$'...'`, locale `$"..."`, and any part kind the function does not list. The quote removal relies on the parser: mvdan's lexer joins each backslash-newline outside single quotes before a `Lit` is built (`rune`, `escNewl`, `vendor/mvdan.cc/sh/v3/syntax/lexer.go`), and keeps every other backslash in `Lit.Value`, so the reading drops a backslash outside quotes and keeps the next character, and inside double quotes drops it only before `$`, a backtick, `"`, `\` and a newline. The parser never produces `BraceExp` unless `SplitBraces` is called, which rewrites the shared tree, so brace expansion is read from the unquoted literals instead: an unquoted `{`, then an unquoted `,` or `..`, then an unquoted `}`. That test is wider than bash's and marks some words bash leaves alone, which is the safe side. A `[` marks a pattern only with a later unquoted `]`. A tilde is a prefix at the word's start, and after `=` or `:` in a word shaped as an assignment, which bash expands even as an argument outside POSIX mode. The function has no production caller in phase 2; phase 3 wires it into the predicate. Its tests: `TestWordValuesMatchBash`, `TestParsedCommandsMatchBash`, `FuzzShellParse`, `BenchmarkHookParse` (`shellparse_test.go`). The bash comparisons run each corpus line through `bash -c` with extglob off and the line handed to eval, under an empty environment whose `PATH` names a missing directory, with every command word the tree names defined as a function that records its argv and `command_not_found_handle` recording any other, so no corpus program runs. `lookBash` fails the test when bash is missing rather than skipping: the edit hook refuses a new `t.Skip` as a weakening, and production is Linux, where bash is present.
   - → Probe record, A-3 (2026-11-11, observed by the main thread from a real Bash tool call): the harness runs the command as `/bin/bash -c 'source <shell snapshot> ... && shopt -u extglob ... && eval '<command>' < /dev/null && pwd -P >| <file>'`. `/proc/$$/exe` is `/usr/bin/bash` and `BASH_VERSION=5.2.21(1)-release`. So the command is bash's, read by eval (which parses and runs one complete command at a time) with extglob off and stdin from /dev/null. The snapshot sets `expand_aliases` but defines no alias (it unaliases find, grep and pkill), and it defines shell functions: `grep`, which runs ugrep (embedded in the claude binary) with `-G --ignore-files --hidden -I --exclude-dir=...` and routes any argument matching `-*-filter*|-*-pager*|-*-view*|-*-format-open*|-*-config*|---*|-@*|-*-save-config*|-[Zz]*|-[!-]*[Zz]*|--null|--null-data` to `command grep`, GNU grep, which rejects the ugrep-only ones; `find`, which runs bfs with `-S dfs -regextype findutils-default`; `pkill`; and `rg` only when no rg is installed. The ugrep options that run a program and that the function does not route are `-Q`/`--query`, interactive, whose view key runs an editor, observed to stop with "no ANSI terminal keyboard detected", exit 2 and no output when stdin is not a terminal, as under the Bash tool; and `--index`, which reads index files only. Consequence: grep's nil judge holds on this host because of that routing and the missing terminal, not because grep is GNU; find's judge is an allow-list of primaries, so bfs receives no unlisted action. egrep and fgrep are not shadowed and reach GNU grep. The `readerCommands` comment records this (AC-32), and the table's entries are unchanged.
   - → Decision (2026-11-11, main thread), a carriage return: `TestWordValuesMatchBash` found the one disagreement between the tree and bash over 440 corpus lines. The lexer drops a CR that comes before an LF (`rune`, `vendor/mvdan.cc/sh/v3/syntax/lexer.go`), and bash keeps it in the word (`cat x\r` passes `x\r`). `parseCommandLine` therefore answers any line holding a CR as a line that did not parse, so it is no reader line and keeps every text verdict. No refusal message changes: the byte allow-list refuses the byte first. `FuzzShellParse` holds it (a CR line never parses) and `TestUnparsedLineKeepsTheTextVerdictAndNeedsTheIdentityLine` has its row. Red walk, all three red and restored: the reading keeping a backslash bash drops reddens 13 rows of `TestWordValuesMatchBash`; a tree cut to its first statement reddens `TestParsedCommandsMatchBash` (bash runs a command the tree does not name); a parse answering both a tree and a failure reddens `FuzzShellParse`. A-4: `BenchmarkHookParse` reports 26 and 30 allocations (about 6 KB) per parse of a pipeline and a reader loop; no time figure is taken on this host. A 60 s fuzz run (13 871 inputs) found nothing.
   - Verify: `./le test unit pkg path ./internal/le/hookruntime/`; A-3 recorded.
3. **Phase: The reader line from the tree** -- development tier; strongest-tier review
   - Work: delete `shellsyntax.go`, `lineCommands`, `lineWords`, `shellLine` and `rawWords` and put the predicate in their place for every caller: the identity exemption, the four lifts, the plain-`rm` narrowing, the build guard's reader+build line, the after-line rules with the wrapper options, the base-name match and `set` (AC-29, AC-31), the `/tmp` word judgment, the advice. `read` and `[[` join. The identity line in front (AC-14). One commit: the tree does not compile between the deletion and the replacement.
   - Tests: AC-3 to AC-8, AC-11, AC-12, AC-14 to AC-19, AC-29 to AC-31. The expectation changes go to `ze-test-judge` before the edit.
   - Pages: `running-commands.md` and `.claude/hooks/README.md` in the same commit.
   - Verify: the hook package tests; `./le doc check all`; the live probe after the build is published.
4. **Phase: Real commands at any depth** -- development tier; strongest-tier review
   - Work: the tree judgment of `bashRawHeavy` and `bashLossyPipe` (AC-9, AC-10), OR-ed with the text verdict.
   - Tests: `TestHeavyJobIsRefusedWhereverTheLineRunsIt`, `TestLossyPipeIsRefusedInsideAConstruct`, the recorded table.
   - Pages: the two guard rows of `.claude/hooks/README.md`.
5. **Phase: Fixtures, seal, history** -- development tier
   - Work: the nine parity rows; the golden judged by the hook's `pretool-bash` action and `bashCode` deleted (AC-36); the fixture sites, both digests re-derived once (AC-24), the `HOOK-FRICTION.md` entry, the journal row's Fix cell.
   - Verify: `./le ai hooks unit`, `./le go vet-platforms darwin`.

### Model tier per phase

Tiers, not model names (`ai/META-DECISIONS.md`).

| Phase | Implementation | Design and review | Why |
|-------|----------------|-------------------|-----|
| This spec | - | strongest | The node table decides what runs without identity in every session |
| 1 Vendor | development | development | Mechanical; the gates are loud |
| 2 Wiring | development | strongest | The recorded table and the word-value reading are what every later proof rests on, and the `.` fix closes an identity hole |
| 3 Reader line | development | strongest | A node admitted by mistake is silent: an `le` call acts as the main thread |
| 4 Depth | development | strongest | It adds refusals to every session's calls |
| 5 Fixtures | development | development | Rows and digests; the unit run fails loudly |
| Test-change judging | - | development | Independence of context matters more than tier |

### Red walk

Each proof is walked red once by breaking the producer, and the red is
recorded in the closure sections.

| Break | Test that must go red |
|-------|-----------------------|
| The predicate admits a function definition | `TestReaderLineRefusesWhatCanRunAnythingElse` |
| The predicate admits an assignment in front of a reader | same test |
| The predicate skips the body of a `for` | `FuzzShellParse` seed row, `TestReaderLineRefusesWhatCanRunAnythingElse` |
| The predicate skips the statements of a `$( )` | same tests |
| `[[` admits `-eq` | same test |
| The bare-assignment rule admits `PATH` | same test |
| The predicate refuses `for` | `TestSubagentReaderLoopPassesUnchanged`, `TestReaderLineReachesThroughConstructs` |
| A judge is handed a redirection's target as an argument | the `uniq -c 2>/dev/null` row of `TestReaderLineReachesThroughConstructs` |
| A parse error is read as an empty line of readers | `TestUnparsedLineKeepsTheTextVerdictAndNeedsTheIdentityLine` |
| The panic containment is removed | `TestParserPanicIsAnUnparsedLine` |
| The text verdict is skipped on a parsed non-reader line | `TestGuardVerdictsOfHeadStand` |
| The tree judgment reads only top-level commands | `TestHeavyJobIsRefusedWhereverTheLineRunsIt` |
| The tree judgment reads arguments as commands | the `./le job run ... command go test` survivor row of the same test |
| The word-value reading keeps a backslash bash drops | `TestWordValuesMatchBash` |
| The guards judge the identity line's exports as non-readers | `TestIdentityLineBeforeAReaderLineStillLifts` |
| The skipped-value rule drops its digit requirement | the `time . x` row of `TestEnvironmentToolIsFoundBehindEveryWrapper` and of the AC-27 rows |
| A tool is matched by the whole word again | the `/usr/bin/env -i` row of `TestEnvironmentToolIsFoundBehindEveryWrapper` |
| `timeout -s` takes no value | the `timeout -s KILL 60 env -i` row of the same test |
| `environmentTools` loses `source` | the `source x` row of AC-27 |
| `set -a` joins the passing options | `TestSetOptionsThatChangeNoVariablePass` |
| `packageRefusal` passes `false` again | `TestRawRefusalAdvisesTheQuietForm` |
| `lineCommands` is restored beside the parse | `TestHandSplitterIsGone` |

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at a named symbol |
| Correctness | For every row of the node table: what can the construct run besides its listed children, and is each of those walked? |
| No layering | No caller reads a hand split of a line the parse covered; the deleted names are gone from code, tests and pages |
| Verdict direction | Every row of the recorded table that changed is marked a reader line or names AC-31, and names its proof; no row was regenerated |
| Opaque stays opaque | No code path parses the content of a quoted string, a here-document handed to a non-reader, or a variable's value |
| Fail direction | A parse error, a panic, an unknown node kind and a word with no readable value each end on the refusing side |
| Wrapper options | Each wrapper's value-taking options, read from its manual, against the declared list |
| Rule: `ai/rules/user-facing-errors.md` | Each new refusal reason names the construct in plain words and ends with the line to type or the admitted form |
| Rule: `ai/rules/stale-comments.md` | Every comment that describes the splitter or the modeled syntax is rewritten or removed |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| The module is vendored, one package | `grep -n "mvdan.cc/sh" vendor/modules.txt go.mod` |
| The ledger row exists | `grep -n "mvdan.cc/sh/v3/syntax" dependency-ledger.txt` |
| The hand splitter is gone | `grep -rn "lineCommands\|modeledSyntax\|unmodeledSyntax\|rawWords" internal/le/hookruntime docs .claude/hooks/README.md` answers nothing |
| The hook package passes | `./le test unit pkg path ./internal/le/hookruntime/` |
| The hook golden passes | `./le ai hooks unit` |
| The journal row is owned here and closed | the commit gate's owned-rows check, and the row's Fix cell |
| The fork probe's record | the closure section that holds it |
| The pages agree with the code | `./le doc check all` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | The command string is untrusted text for the parser: a panic, an input that parses slowly, a very long line. The parse is bounded by the line and contained |
| Authorization that could fail open | The reader-line predicate is the authorization to run without identity, and the after-line walk is the authorization to keep it. The predicate answers yes only from an allow-list. The walk's wrapper options are a declared list, and a missing one is the known way it fails open (R-9) |
| Injection | The advised command is written from parsed words through `quotedWord`, one shell word each, so following the advice runs the words the agent typed and nothing else |
| Supply chain | One third-party package, vendored at a fixed version, imported by one directory, linked into `le` only |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| A recorded-table row loses a refusal | The code is wrong, not the row: fix the predicate or the text path. A row is edited only to mark a proven reader line |
| The bash comparison disagrees | Read the parser's node for that line; narrow the node table or the word-value reading. If the parser misreads bash, the construct leaves the table |
| The judge refuses an expectation change | The row keeps its expectation and the line stays outside the reader line |
| A-1 is wrong | Stop phase 1 and report the packages to the main thread: each needs its ledger row |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- The two constraints given for this spec, "every guard keeps HEAD's verdict or a stricter one, and only a reader line lifts it" and "end the quote-blind split in `commandSegments`", meet only one way. On a line that is not a reader line, a quoted string may be run by the command that receives it, so the first constraint forbids clearing it, and the quote-blind split is exactly the judgment that does not clear it. The split therefore ends where it was wrong: on every line the parse proves to be readers, which now includes loops and conditionals, and in the advice. It stays as the verdict of text a non-reader may run.
- Replacing the whole-line text judgment by a judgment of each parsed string was examined and rejected. It would loosen only unquoted text that is no command position (a comment, a case pattern's bar), which no agent types by mistake, and it would re-derive the text verdict over new inputs, which is the edit that opened six holes in child 1's phase 2.
- Three of the review's notes are not fixes to write but consequences of the tree: a `$` before a closing quote is a literal the parser reads as one, a redirection is a node and never an argument, and a reserved word is structure and never a word to skip. Each still has its row, so the consequence is proven and not assumed.

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `hook-existing-patterns-false-positive` | 2026-11-11 | `rawHeavy` on a `for` loop: a quoted `go vet` alternative read as a raw run |
| `hook-existing-patterns-false-positive` | 2026-11-11 | `bashTestDeletion` line-wide words: a test path `cmp` read beside a plain `rm` refused as a deletion (AC-18) |
| `bash-guard-misses-a-command-shape` | 2026-11-11 | `bashLossyPipe`, `bashRawHeavy`: a launcher's option value read as the command word |
| `bash-guard-misses-a-command-shape` | 2026-11-11 | `bashRootBuild`: `-toolexec`, `-overlay`, `-modfile` pass the text judgement |
| `bash-guard-misses-a-command-shape` | 2026-11-11 | `bashSystemTmp`: no `}` boundary in `systemTmp` |
| `green-that-could-not-have-been-red` | 2026-11-11 | bash parity golden judged by the hand model `bashCode` |
| `hook-existing-patterns-false-positive` | 2026-11-11 | error-inventory `shapeFinding`: an identity-only `Finding` literal judged as a plain message |
| `failure-report-names-a-command-that-does-not-exist` | 2026-11-11 | raw-job advice read the redirection `2>` as a package path (AC-15) |

The last five rows came here from `plan/post-release/engineering/spec-tooling-one-off-defects.md` on Thomas's decision (2026-11-11). Thomas's words: "complete all the work, it does not matter if we do via a spec-agent-economy-6-shell-parser spec". The criteria that fix them:

| Row | Fixed by | Phase |
|-----|----------|-------|
| launcher option value read as the command word | AC-33 | 2, a pre-parser commit |
| `-toolexec`, `-overlay`, `-modfile` | AC-34 | 2, a pre-parser commit |
| no `}` boundary in `systemTmp` | AC-35 | 2, a pre-parser commit |
| parity golden judged by `bashCode` | AC-36 | 5 |
| `shapeFinding` identity-only literal | AC-37 | 2, a pre-parser commit of its own (no hook code) |

The row moves here from `plan/post-release/engineering/spec-tooling-one-off-defects.md`
in the commit that lands this spec: that spec's table drops the entry and this
table holds it, so the row is owned at every commit.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| The lift stays line-wide: only a line whose every command is a reader lifts a text refusal | Lift per command: clear a reader's quoted arguments on a mixed line | A reader's text reaches a later non-reader through `$_`, a pipe or a file, which child 1's review showed (`echo 'x -toolexec=...'; go build -tags $_`). Forecloses: `./le x; grep 'a\|go vet b' f` stays refused. Open question 3 |
| The text judgment runs unchanged over the raw line on every non-reader line, and the tree judgment is OR-ed with it | Judge only the tree and each quoted string | "HEAD or stricter" then holds by construction. Forecloses: a guarded name in a comment or a case pattern on a non-reader line stays refused |
| A node-kind allow-list defines the reader line | Admit everything the parser accepts and deny known dangers | A deny-list missed fourteen forms in child 1. Forecloses: a harmless construct nobody listed (a function of readers, `$((1+2))`) needs the identity line until it earns a row |
| Arithmetic is outside the reader line everywhere | Admit arithmetic over literals | Bash evaluates a variable's value as an expression in every arithmetic context, and that value can hold a subscript that runs a command. Forecloses: counters in reader loops |
| An assignment in front of a reader is outside; a bare assignment needs a lower-case name absent from the environment | Admit any assignment | `GIT_EXTERNAL_DIFF=x git diff` and `PATH=. cat f` run a program. Forecloses: `LC_ALL=C sort f` needs the identity line |
| The agent's own exact line in front leaves a reader line a reader line for the guards | Keep `export` a non-reader | The two exports are byte-compared with the payload's line and no reader reads them. Forecloses nothing; it removes the refusal an agent meets when it types its line out of habit |
| Import `syntax` only and read a word's value in the hook | Import `expand` and let the library unquote | One ledger row and one vendored package; the reading is three rules inside a word the parser delimited, compared with bash by test. Forecloses: ANSI-C quoted words on a reader line |
| No string is opened | Parse the string handed to `bash -c`, `eval`, `ssh` | Child 1 deleted that after two reviews: every program that runs a string reopened it. Forecloses: `bash -O extglob -c 'go test ./x'` stays unseen, with its owner |
| Wrappers declare their value-taking options, and a skipped duration holds a digit | Refuse any command after a wrapper that holds an environment tool's name anywhere | The declared list reads the command the wrapper runs, as the shell does. The whole-command match would refuse `timeout 5 grep env f`. Forecloses: an option missing from the list hides a tool until its row exists (R-9) |
| `set` passes with five named options | Keep every `set` refused; or drop `set` from the tools | `set -o pipefail` in front of a pipeline is what agents type, and it changes no variable; `set -a` and operands do change what later commands see. Forecloses: `set -o posix`, `set -f` and the rest need a row to pass |
| The lone-`.` fix and its rows land in phase 2's first commit, before the parser | Fix it with the tree in phase 3 | It is an open hole in the identity check, a one-condition fix, and the rows prove it either way |
| The git guards are not moved to the tree | Judge `git commit` as a parsed command | They read neither splitter and enforce a hard ban. Forecloses: their quote handling stays hand-written |
| HEAD's verdicts are recorded as data before the replacement | Keep a frozen copy of the old splitter in a test as the oracle | Data states the verdict; a frozen copy keeps deleted code alive. Forecloses: a line absent from the corpus is not protected by it |
| The parity golden gains rows and runs them through the hook (AC-36) | Keep the hand model `bashCode` | A model stays green when the hook moves (the `go build ./...` row did). Thomas's decision of 2026-11-11 put the row here. Forecloses: nothing |

## Known Limitations
- A mixed line keeps the text verdict: one command outside the reader set and every quoted separator on the line is read as a command boundary again. The identity line in front is the one exception.
- Arithmetic, arrays, functions, `declare` and an assignment in front of a command are outside the reader line. A subagent types its line for them.
- A string in ANSI-C or locale quoting puts its line outside the reader line.
- The text judgment is kept as the verdict of a non-reader line. Its three known gaps are closed here as tightenings: a launcher's option value read as the command word (AC-33), `-toolexec` with `-o bin/` (AC-34), an unquoted `${v}/tmp` (AC-35). A gap found later is a new row.
- The check still catches a mistake and is no sandbox: a script the agent writes runs unseen.
- The bash comparison tests need a bash on the machine that runs the hook package tests.
- The reader table's nil judges assume GNU grep and coreutils. On a host with other tools the table is trusted as written until a judge is added (AC-32 states it where the table is declared).

## Open questions for Thomas

| # | Question | Evidence | Default taken |
|---|----------|----------|---------------|
| 1 | Not a question, an action only you can take: approve the import before phase 2's second commit | `docs/contributing/dependency-approval.md`: `./le commit create` refuses a third-party import with no ledger row, and only `./le deps approve` from your terminal writes one. Command: `./le deps approve package mvdan.cc/sh/v3/syntax importer internal/le/hookruntime reason "bash parser for the pre-tool Bash hook"` | None possible. Phase 1 and the first commit of phase 2 run without it; the parser import waits |
| 2 | Answered: the bash parity golden runs its rows through the hook | Thomas's decision of 2026-11-11 moved the `green-that-could-not-have-been-red` row here ("complete all the work") | Yes: AC-36, phase 5. `bashCode` is deleted, and the nine rows added here prove their verdict through the hook |
| 3 | May a reader's quoted arguments be cleared on a mixed line too, such as `./le spec current; grep 'a\|go vet b' f`? | Child 1's rule, set by its review and not by you: only a whole line of readers lifts. It keeps that grep refused. The known way a reader's text reaches a later command is `$_`, a pipe or a file | No: the lift stays line-wide |

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
- [ ] AC-1..AC-37 all demonstrated
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass, run in the main checkout: the hook package tests and `./le ai hooks unit`. No agent runs any `./le verify` command
- [ ] Feature code integrated (`internal/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes, and `ai/rules/quality.md` is satisfied
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (N-A: the feature takes none)
- [ ] Functional `.ci` tests for end-to-end behavior (N-A: the hook's functional run is `./le ai hooks unit`)
- [ ] Interop tests for protocol features (N-A: tooling with no protocol peer)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove plan/post-release/engineering/spec-agent-economy-6-shell-parser.md` only, in the same `./le commit create` script
