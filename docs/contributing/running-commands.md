# Running Development Commands

<!-- source: internal/le/go/toolchain, internal/le/repo/changed, internal/le/test/functional, internal/le/session, internal/le/job, internal/le/hookruntime, internal/le/verify, internal/le/scratch -->

How the `./le` action surface, the session scratch tree, and the Bash guard
behave. The obligations that follow from this page are `ai/rules/commands.md`.

## Finding a command and what it takes

`./le '|' json` answers le's whole surface as one document. It names every
registered area, the group it renders under, its `short-help` and its
`description`. An area that registered an action table also carries each action,
with its verb, its one-line `summary`, its whole purpose, its write flag and its keyword grammar. Each keyword carries its `kind` (such
as `switch`, `path`, `uint` or `enum`), its `help` line, whether it is
`required` or may `repeat`, and an enum's `values`. The bar is quoted because a shell
consumes an unquoted one. `| yaml` and `| table` render the same payload.

`./le` with no argument prints that manifest as the root help page, and
`./le help` and `./le --help` print the same bytes. All three write to stdout
and exit 0. The one exception is a person at a terminal: when stdin and stdout
are both terminals, a bare `./le` opens the menu instead (below), so an agent,
a pipe, CI or a redirection always gets the page. The page has one block for the single-word commands and one per
namespace, its heading followed by the namespace's short summary; each line is
the name, the area's group as a tag in brackets, and the area's short summary
whole. Every short text le prints (an area's or a namespace's summary, an
action's `Summary`, a keyword's `Help`, a value's line) is declared as one line
of at most 60 characters, and every namespace, area and action also declares a
long description, one to three sentences, that its own page and the menu print. Every page wraps to the terminal's width, or
to 100 columns when stdout is a pipe, so a page read through a pipe is the same
on every host.

`./le <area> <verb> --help` (or `./le help <area> <verb>`) prints one action's
page on stdout: its whole purpose, its usage line, and one line per keyword
giving its kind, whether it is required or repeats, and its help. The dispatcher
renders it from the registered action table and calls no handler. The help word
therefore starts no work, whatever the area does with its first argument. In the
usage line a required keyword prints bare, an optional one prints inside
brackets, and a keyword that repeats carries a trailing ellipsis.
`./le help <area> <verb> <keyword>` prints one keyword's page, with the meaning
of its kind and its values. When a line is refused, stderr carries the error,
then that one action's usage and keyword lines, never the area's other actions.

The bare word `help` that a declared keyword introduced is that keyword's VALUE,
and the action runs. `./le repo rewrite replace file <path> old beta new help`
replaces `beta` with the word `help`. The registered table tells a value from a
question, so the two never collide in an area that declares one.

A value never begins with a dash. le follows GNU option syntax, and
`ai/rules/cli.md` declares le's whole option set: `--help`, `-h`, `--version`
and `-V`. A dash opens a long option, or a cluster of one-letter short options.
`-help` is therefore not the word `help` with one dash. It is `-h` followed by
`-e`, `-l` and `-p`: a help request, then three options le does not declare.

An action that dispatches through its action table refuses a dash-leading word
in every value slot, the last one included, and answers 2. `./le verify status
check path --help path internal` refuses and reads no path. `./le repo rewrite
replace file <path> old beta new --help apply` refuses and writes nothing.
`./le test stress-repro run suite -help` refuses the cluster by name and starts
no burn. Only the bare word, which carries no dash, can be data.

Four spellings ask for help, and the set is closed: `help`, `--help`, `-h` and
`-help`. A trailing one asks the question where it stands in place of the next
keyword, so `./le test stress-repro run --help` and `./le test stress-repro run
suite bgp --help` print the action's page and run nothing. Every area declares
a table, or a pass-through one for an area whose words are another program's:
le refuses to start while one declares neither, naming the area. It refuses
as well while an area, an action, a keyword or a value an enum declares has no
text, naming the item and the package the text goes in (`docs/architecture/core-design.md`,
"Every word le offers says what it does").

Every other option travels on. An area with a table refuses it, except after
a keyword of kind `argv`, which ends the grammar: every word after it is
another program's line, handed over as typed. `command <argv...>` is that
keyword, so `./le job run label x command echo -html=cover.out` prints
`-html=cover.out`, and `./le job run label encode-list command bin/le test bgp
encode --list` hands `--list` to the child. A pass-through area
(`leroot.RegisterForwarding`) hands every word after its name to its program,
a trailing help spelling included: `./le mrt statistics --help` is the MRT
analyzer's own `statistics --help`.

A shape rule cannot replace the four words. Only the grammar an area publishes
tells le its own option from one you forward to a child.

<!-- source: internal/le/le/root/manifest.go -- Manifest -->
<!-- source: internal/le/le/root/dispatch.go -- Dispatch, helpTrailing -->
<!-- source: internal/le/le/action/parameter.go -- parameterForm, parseArguments -->
<!-- source: internal/le/le/root/leroot.go -- requireEveryAreaHasATable -->
<!-- source: internal/le/le/root/namespacesummary.go -- requireEveryNamespaceHasASummary -->

### At a terminal: the menu, and prompting for a missing value

With stdin and stdout both terminals, a bare `./le` opens the drill-down menu
ze opens for a bare `ze` (`docs/architecture/cli/tui-launcher.md`), over le's
derived command tree: the first screen lists every first word with what it is
for, in the declared short summary shown whole in the right column. Enter on a
namespace or an area opens its members under a header holding that node's long
description, wrapped to the screen, and Enter on an action runs it. `ze`'s own
menu prints no such header and is unchanged. Typing
filters the level, Left and Backspace go back, Escape goes back or quits, and
`q` quits while no filter is typed. Quitting runs nothing and exits 0.

At a terminal, a line that names an action but leaves out a value the action
declares required is not refused: le asks for each missing value in turn, the
positional one first, with Tab offering what `le completion words` offers at
that place, and the paths on disk for a path or a spec file. A directory Tab
cannot list (it is a file, or cannot be read) is named on stderr once the
prompt closes, with the command that shows why, so an empty offer is never
read as an empty directory. Optional values are never asked. Enter on an
empty answer keeps asking. Once every value is given, le prints the whole line
to stderr after `running: `, so it is never read as the command's output, and
runs it:

```
$ ./le spec claim
spec (the spec file this session takes): plan/post-release/engineering/spec-le-cli-usability.md
running: le spec claim spec plan/post-release/engineering/spec-le-cli-usability.md
```

A line chosen at the menu is echoed the same way, whether or not anything was
asked. Ctrl-C or Ctrl-D at a prompt runs nothing and exits 2. Off a terminal
nothing is asked, because a prompt on a pipe would wait forever: the line runs
and is refused with that one action's usage, as above. A pass-through area and
a forwarding action are never prompted, since their words are another
program's. A prompt never answers the whole-tree gate: a full verification
still asks its own terminal question once the line is complete
(`internal/le/le/human`). `ze le` behaves identically under its own name.

<!-- source: internal/le/le/root/prompt.go -- fillLine, missingIn, candidatesFor, pathsFrom, noteUnlisted, openMenu -->
<!-- source: internal/le/le/action/leaction.go -- Missing -->
<!-- source: internal/component/cli/menu/prompt.go -- Ask -->

### The whole surface for an agent: `le help ai`

`./le help ai` prints the sections it answers. Each section is a word:
`commands` lists every namespace with what it is for, then every area, each action and each parameter with its kind, its
values, whether it is required and whether it repeats, and the program a
pass-through area hands its words to, and every hidden area (one Ze spawns,
absent from help pages and completion) marked hidden; `groups` the purpose tags; `verbs` every
verb in use; `exit-codes` what each code means and who answers another
program's; `launcher` the words the `./le` script reads before le starts
(below); `all` every section. Render it with the pipe layer, never a flag:
`./le help ai commands | json`, `./le help ai all | yaml`. A dash-leading word
(`--json`, `--ai`) is refused with exit 2 and the refusal names
`le help ai | json`; so is a section that does not exist. `ai` is the one
reserved word after `help`, so the `ai` namespace's own page is
`./le ai help`. `ze le help ai` answers the same, its program reading `ze le`.

<!-- source: internal/le/le/root/helpai.go -- helpAI, answerHelpAI -->
<!-- source: internal/component/aihelp/reference/reference.go -- Reference -->

### What le's exit codes mean

Every le command answers one of four codes, so a script or an agent reads the
kind of failure from the code alone. `./le help ai exit-codes` prints the same
table from its one declaration.

| Code | Meaning |
|------|---------|
| 0 | the action did what it names, or a help question was answered |
| 1 | the action ran and found a problem (a gate's violations, a review that is not clean), or the operation it ran failed |
| 2 | usage error: an unknown command or action, a word a namespace does not hold, a stray or dash-leading word, a missing or malformed value, a retired spelling, a refused rendering flag |
| 3 | could not run: the checkout, a file or a program the action needs is absent or unreadable, or a `\| save` could not write its file |

The table's parser makes every usage refusal before the action runs: a value
its kind does not accept (`max-areas many`) and a required value left out on
a pipe are refused there, with the one action's usage. Two kinds of command
are exempt. `le ai hooks <hook>` answers the hook protocol, 0 to allow and 2 to
block, because Claude Code reads 2 from a hook as "block". A forwarding
command (`le test fixture`, `le test peer`, every `mock` area, every
pass-through area) answers its program's own code.

<!-- source: internal/le/le/root/exitcode.go -- exitCodes -->
<!-- source: internal/le/le/action/parameter.go -- parseLine, checkValue -->

A retired spelling is refused before le parses anything, with one line
naming the spelling that does the same work now, the words typed after it
carried over: `error: le verify lock was renamed: run le job run`. The rename
map is declared once, in `internal/le/le/retired`, and `le doc check
retired-commands` reads the same map to find callers that still name an old
spelling.

<!-- source: internal/le/le/root/dispatch.go -- refuseRetired -->
<!-- source: internal/le/le/retired/retired.go -- Match -->

### The words the launcher reads first

The `./le` script reads some words before le starts, so they come first on
the line: `--name <name>` and `--name=<name>` build the working tree as
`bin/le-<name>/le` and run it, the value `session` naming this session's
build; `rollback` sets the newest build aside. They are declared once in
`leroot` (`rollback` by its own area), the root page lists them in its
`Launcher` block, `./le help ai launcher` lists them, and completion offers
them at the first word and `session` after `--name`. A test reads the words
from the script itself and fails when the script and the declaration differ.

<!-- source: internal/le/le/root/launcher.go -- launcherWords, launcherSection, afterLauncher -->
<!-- source: internal/le/le/root/launcher_test.go -- scriptWords -->

### Verb vocabulary

le's action words form one closed vocabulary, declared once in
`internal/le/le/action/vocabulary.go`. An action word is either a verb of the
vocabulary, performing that verb's one meaning, or a name (a suite, scenario,
platform, shell, artifact or selection, such as `test functional bgp`) that
declares the verb it performs in `Does`. Three rules hold it closed:

- a vocabulary verb never declares a different verb;
- no hyphen part of a name is a vocabulary verb, `write`, `generate` or
  `test`, so `condensed-update` and `ipsec-mobike-test` are refused;
- `all` is the one name every area may use, meaning every member in table
  order.

The `le ai hooks <name>` words that `.claude/settings.json` calls are frozen:
the rules do not judge them, and a test holds the frozen set equal to the
settings file's calls. Each verb has a kind: `report` reads and prints, exit 0
whenever it answered; `gate` reads and exits 1 on a defect; `regenerate`
rewrites a committed artifact from its source; `record` appends to a ledger;
`render` produces a published artifact this repository does not commit; `run`
has side effects outside the tree's committed files. `update` regenerates a
committed artifact; `write` and `generate` are not verbs.

The tests in `internal/le/vocabulary_test.go` judge every registered action.
While the renames of `spec-le-cli-usability` Phase 4 land in batches, the
findings the tree still holds are listed in
`internal/le/testdata/vocabulary-owed.txt`, each with the rename or
declaration that clears it. A new finding fails, and a listed finding the tree
no longer holds fails too, so the list only shrinks.

<!-- source: internal/le/le/action/vocabulary.go -- vocabulary, ForbiddenNamePart -->
<!-- source: internal/le/vocabulary_test.go -- vocabularyFindings, judgeAction -->

### Install completion

One script completes both `./le` and `le` (when the checkout is on your
`PATH`). Load it in the current shell, from the checkout root:

```
eval "$(./le completion bash)"     # bash
source <(./le completion zsh)      # zsh
./le completion fish | source      # fish
```

To keep it, write it where your shell looks: `./le completion bash >
~/.local/share/bash-completion/completions/le`, `./le completion zsh >
"${fpath[1]}/_le"`, or `./le completion fish > ~/.config/fish/completions/le.fish`.
Each script's first lines say the same.

On Tab the script runs `<the typed word's directory>/bin/le completion words
<line>`, with stderr discarded. `bin/le` is the dispatcher's installed copy, and
it recognises that form: it execs the newest build in `bin/builds/` with no
input-key check, no build and no rollback, so a keystroke costs one directory
listing, and with no build it completes nothing.
The `completion` area is a shell callback, so le runs no cache-link repair and
no before-dispatch hook for it. It offers areas, then actions, then the
keywords still open: a keyword already given is not offered again unless it
repeats, a retired keyword is never offered, and an enum's values come only
after their keyword. A pass-through area completes nothing after its name,
because its program owns those words. A typed word le does not know is refused:
`./le completion words nosuch ''` names it on stderr and exits 2, so the shell
offers nothing while a person running it by hand learns which word was wrong.
After a `git pull` the next `./le` call builds the new HEAD in the background,
and completion knows the new commands once that build lands.

<!-- source: internal/le/completion/completion.go -- table, answerWords -->
<!-- source: internal/le/le/root/complete.go -- Complete, afterVerb -->
<!-- source: internal/plugins/completion/script/script.go -- Generate, WriteRecord -->
<!-- source: internal/le/register.go -- run -->

## Launcher builds do not overwrite running binaries

`./le` is a dispatcher. It runs the newest build of le in `bin/builds/`, one
directory per build named `<UTC commit time>-<short sha>`, so a byte-order
listing sorts the newest last. `bin/le` is a copy of the same script, which
`./le` installs by rename whenever the two differ, and it chooses exactly as
`./le` does. Go code reads the same names through `gotoolchain.ParseBuildName`
and `gotoolchain.RunnableBuild`, which the store trim uses to judge builds.

Builds follow HEAD, never the working tree. Each build records its input key:
the git object id of `internal cmd pkg api vendor go.mod go.sum
feature-gates.txt feature-tags` at the commit it was built from. When HEAD's key
differs from the newest build's, the call starts one detached build and still
runs the newest; later calls see the lock (`bin/builds/.building`, holding the
builder's host and pid) and run the newest too. A lock from this host is
reclaimed once its pid is dead. A lock from another host (a QEMU guest sharing
the checkout over 9p, which cannot see the host's pids) holds until it is thirty
minutes old, well past the two minutes a cold build takes, and is then
reclaimed. The builder exports HEAD with `git archive` into the lock directory, compiles with
the patched GOROOT into `bin/builds/.staging-<pid>/le`, smoke-tests it (`le
help`, `le spec wip`, `le spec current`, with `ZE_REPO_ROOT` naming the
checkout), and renames the directory into place. A commit that changes no input
reuses the newest build, so it starts nothing. With no build at all, the first
call builds HEAD in the foreground and the others wait for it. When
`bin/builds` or its lock cannot be created at all (a read-only `bin/`, a full
disk), that call refuses at once with the path, the cause and the fix, because
no wait would end it.

A build that fails to compile or fails its smoke test is never published. Its
log becomes `bin/builds/.failed-<commit>.log`, the next caller reads one line
naming the commit and the log, and that commit is not built again. A build
that does not start (not executable, the wrong platform, or failing the smoke
test a hand-placed build gets before its first run) is moved aside to
`bin/builds/.failed-<name>` and the call runs the next newest, saying so in one
line. `./le rollback` moves the newest build aside the same way on demand, and
the inputs of a build moved aside are not rebuilt, except when it was the only
build: the next call then finds no build and builds HEAD in the foreground,
because some le must run. Nothing in this path
deletes: a process running a build keeps its file.
`rollback` is registered in le's command tree, so `./le help` lists it and
completion offers it, but the move is the launcher's: a build run by its path,
or as a `--name` build, refuses `rollback` with exit 2 and names `./le rollback`.
<!-- source: internal/le/rollback/rollback.go -- Answer, refusal -->

A machine that shares the checkout with another platform, a QEMU guest
mounting it, finds the newest build's header is not its own, and its builds are
named `<time>-<sha>-<GOOS>-<GOARCH>` in Go's words. `--update` is gone, and
says so with exit 2.

`--name <name>` rebuilds `bin/le-<name>/le` from the working tree on every
invocation and exports `ZE_LE_BUILD_NAME` so nested calls select the same
private build; `--name session` names it `s-<first 8 of
CLAUDE_CODE_SESSION_ID>`, and refuses with exit 2 when the variable is unset.
Every publish, a build of HEAD and a named rebuild alike, writes a complete
sibling and renames it onto the target, so a process already running keeps its
old inode and no caller opens a partially written file. The final filename stays
`le`, preserving personality dispatch. Preparation failures, including a missing
or empty feature manifest, stop before the compiler runs.

A missing `go` stops it too. The script refuses before git and before `go
build`, and once a build exists `gotoolchain.New` refuses before any le action
starts a Go command. Both name the PATH searched, the version go.mod pins, and
how to put Go on PATH. The usual cause is a non-interactive shell, such as `ssh
host command`, that never read the profile adding Go to PATH.

<!-- source: le -- build_le, build_head, export_commit, choose_build, start_candidate, set_aside, take_lock -->
<!-- source: internal/le/go/toolchain/builds.go -- BuildsDir, ParseBuildName, RunnableBuild -->
<!-- source: internal/le/go/toolchain/gotoolchain.go -- New, goMissing -->

Every Go command le starts compiles against Ze's patched standard library
(`docs/contributing/patching-the-standard-library.md`). le's root handler calls
`gen.Activate` (`internal/le/go/stdpatch/gen`) before it dispatches: it builds the patched GOROOT from the
toolchain go.mod pins, on first use, and sets `GOROOT` to it, `GOTOOLCHAIN=local`
and the tree's `bin` first on `PATH` in le's own environment, so every child
inherits them. `./le go stdpatch goroot` prints that tree; a plain `go` command
outside le uses it with
`export GOROOT="$(./le go stdpatch goroot)" GOTOOLCHAIN=local PATH="$GOROOT/bin:$PATH"`.
A refusal of the generator (a stale review stamp, a patch that no longer
applies) leaves the dispatch working and is the error of every action that
compiles through `gotoolchain.New`. `./le go patch-stamps check` compares every
patch's review stamp, standard-library and vendor, with the version it names,
and `./le verify` runs it in both modes.
<!-- source: internal/le/register.go -- activatePatchedGOROOT -->
<!-- source: internal/le/go/patchstamps/patchstamps.go -- runPatchStampsCheck -->
<!-- source: internal/le/go/stdpatch/gen/stdpatch.go -- Activate -->

A named build does not stay forever. The hourly store trim (see "When the disk
is full") removes `bin/le-<name>/` once nothing in it was written for a day
and no running process carries the path in its argv; the next
`--name <name>` call rebuilds it, at the cost of one warm build. A process
already running from it keeps its inode. `bin/le`, `bin/ze` and every other
`bin/ze*` never reach the removal, and neither does a name the script's own
name check would refuse. A `bin/le-<OS>-<architecture>/` left by the launcher
before `bin/builds` (`bin/le-Linux-aarch64/`) is judged the same way as a named
build and goes once idle: the launcher writes no such directory any more.

<!-- source: internal/le/go/toolchain/gotoolchain.go -- NamedLauncherDir, NamedLauncherName -->
<!-- source: internal/le/scratch/livetrim.go -- trimLaunchers, treeIdle, pathNamedInArgv -->

## A bare `go test` is not `./le test unit`

Ze compiles features out behind build tags (`//go:build ze_isis`, `ze_ospf`,
`ze_ldp`, `ze_rsvpte`, `ze_web`, `ze_ssh`, and the rest). `internal/le/go/toolchain`
derives the feature set from `feature-gates.txt` and passes it to every native
unit and verification action. Omit those tags and the plugins never register, so
their validators, listeners and schema vanish, and unrelated tests fail.

The failure is a phantom red with a plausible but false cause. Its symptom is a
test asserting on something another feature registers (a listener, a validator, a
plugin name, a wire method, a schema entry) failing with a message that a thing is
*missing* or *not produced*. Check the tags before believing it.

A `git archive HEAD` scratch tree carries the same trap: a bare run there
reproduces the same mistake and confirms a red that does not exist.

To scope a run to one package and keep the tags, read the tag list out of
`feature-gates.txt` and pass it with `-tags`, prefixed by `ze_core`:

```
tags="ze_core $(awk '$1 ~ /^ze_/ {print $1}' feature-gates.txt | sort -u | tr '\n' ' ')"
```

## What `./le test unit all` covers

`all` runs `./...` under the feature tag set and the race detector, then each
group whose own build tags compile its test files out of that run. Today that
second part is the `installer` group, because `internal/install`'s tests carry
`ze_installer` and no feature-tag run selects them.

The six named verbs (`bgp`, `core`, `plugins`, `config`, `cli`, `installer`) are
targeted SUBSETS, for iterating inside the one you changed. A green group says
nothing about the rest of the tree. Neither did `all` until 2026-09-03, when it
swept those six verbs and nothing else: a session that ran it read green over
every other component and over all of `internal/le`, `internal/appliance`,
`internal/test`, `pkg` and `cmd`.

`./le test unit` typed alone lists the verbs and starts no run.

## A functional suite is not the runner binary

`./le test functional <suite>` builds an isolated bare-named binary pair into the
session scratch directory (`internal/le/test/functional/binaries.go`). The daemon
carries the test-only tag set, and the suite runs with `LE_TEST_NO_BUILD=1`,
and `ZE_BIN` pointing at that set, as `<set>/le test <suite>`
(`BinarySet.Environment`).

Running `le test` directly skips all of that. The runner can then
rebuild a daemon without the test-only surface, so a fixture times out for a
build-population reason and the failure looks like the code under test. The
`--server` and `--client` hints the runner prints on failure inherit the same
gap: they re-run the same non-equivalent launch.

| Want | Use |
|------|-----|
| A whole suite | `./le test functional plugin` (`./le test functional list` names every suite, and `./le test functional preview` says which ones a gating run would start) |
| One test, iterating | The owning compiled fixture's Go test, then rerun the whole `./le test functional <suite>` |
| A kernel-dependent suite in the VM | `./le test qemu netns suites <comma-separated-suites>` |

## A functional run checks the host before it starts

A functional fixture binds loopback addresses that `./le setup` adds and a
reboot removes. A host without them used to fail at the first fixture that binds
one, which in a verification is two hours in. So every command that can start a
functional suite checks the host first, before it builds or runs anything:

| Command | When it checks |
|---------|----------------|
| `./le test functional gating`, `./le test functional <suite>`, `./le test functional exabgp` | Always |
| `./le verify worktree`, `./le verify current` (Thomas's release check) | When the population holds a functional stage |
| `./le verify worktree all of <m>` (Thomas's release check) | When a piece the sweep will run holds a functional stage, for every such piece before the first starts |
| `./le feature run feature <id>`, `./le feature run due <days>`, `./le site build refresh` | When a feature's real-path tests hold a `.ci` that runs on this host, before its first test runs. A Go test, an editor test and a `.ci` whose home is the QEMU guest bind none of the addresses. Each due or stale feature is judged on its own, so every one is refused and none runs |
| `./le rfc record` | When the tagged unit's carrier is a functional `.ci`, before the break is built. An escape (`route no-break`) runs nothing and is not checked, and neither is `./le rfc discriminate`, which only reads records |

A missing address refuses the run with a non-zero exit. The refusal names each
missing address and the fix: `./le setup install`, or the `sudo` line for each
address.
It never warns and continues. The address set and the readiness test are the ones
`./le setup check` reports as `loopback-addresses` (`docs/guide/developer-setup.md`),
so `./le setup check` says the same thing before you start.

<!-- source: internal/le/setup/system.go -- FunctionalHostPreflight -->
<!-- source: internal/le/verify/preflight.go -- HostPreflight -->
<!-- source: internal/le/test/functional/actions.go -- hostReady -->
<!-- source: internal/le/feature/recordrun.go -- repoRunner.preflight -->
<!-- source: internal/le/rfc/discriminate_action.go -- observeFor -->

## The lint gate

The native per-edit hook (`internal/le/hookruntime/postwrite.go`) judges changed
lines only. Package-wide analysis is what finds the rest:

- functions and variables left unused by a refactor in another file
- import cycles introduced by a cross-package change
- type mismatches from an interface change
- constants and vars that became unreferenced

`./le go lint run` lints the full tree through each selected build. An explicit
`scope "<packages>"` limits the same matrix to those package patterns.
golangci-lint analyzes one GOOS, one GOARCH and one tag set per invocation, so a
file outside that build is unchecked. The flavor matrix that closes that hole
is `testing.md`, "The builds the linter reads"; its rows are `flavorMatrix` in
`internal/le/go/lint/matrix.go`.
<!-- source: internal/le/go/lint/verifylint.go -- Plan, planScope, plan -->
<!-- source: internal/le/go/lint/actions.go -- runRunner -->

Before querying the matrix, after each flavor query, and after full-tree
coverage enumeration, the planner compares the shared lint input fingerprint.
Source content and build tags, nonignored file additions and deletions, lint
configuration, and module and feature manifests participate. A changed
fingerprint or a failed measurement refuses the plan before any lint child
starts; rerun after source edits stop. It does not retry or remove packages to
obtain a plan. These are observation checks, not an atomic filesystem snapshot.
<!-- source: internal/le/go/lint/verifylint.go -- plan, checkPlanningInputs -->
<!-- source: internal/le/job/treehash.go -- InputHash, writeReadPaths, lintIgnores -->

## The changed-set selector

`./le repo changed scope` is the one selector, and `./le verify changed` and
`./le verify current fast`, Thomas's release check, reuse its answer. It reports the changed packages plus two levels of their
importers (`defaultDepth`, `internal/le/repo/changed/selector.go`), and the feature
tags the change can reach.

```
./le repo changed scope print both
./le repo changed scope print packages paths-from FILE
```

A non-Go path seeds the Go packages whose tests read it, so a `.ci` file or a
rule point selects the native tooling packages rather than nothing.

Every route that fails to narrow WIDENS to `./...` and names its reason on stderr
(`widen`, `internal/le/repo/changed/scope.go`). The committed half of the change
set starts at the anchor, the merge base of HEAD and its upstream, or of HEAD
and `refs/remotes/origin/HEAD` when the branch tracks none (`ResolveAnchor`,
`internal/le/repo/changed/anchor.go`). No verify run can move it, so a red or
interrupted run never widens the next scoped answer. A checkout with neither
ref is refused with exit 2 and the command that fixes it, never widened:
`git branch --set-upstream-to=origin/<branch>` on a branch, and
`git remote set-head origin --auto` with HEAD detached, as in a verify worktree
(`noUpstreamError`). The contract is
`../architecture/testing/verify-freshness-scope.md`.

One directory answers with the whole tree by design. Every file in
`cmd/ze-installer` carries `//go:build linux && ze_installer`, so `go list` under
the unit tag set reports no package there, and `seedPackages`
(`internal/le/repo/changed/selector.go`) has nothing narrower to name. The wide answer
is what makes the `ze_installer` lint flavor run at all.

A scoped run also judges fewer Staticcheck feature-matrix rows. `scopeMatrix`
(`internal/le/go/staticcheck/staticcheckfeaturematrix.go`) keeps the two
rows that omit no feature tag, plus one row per tag the change reached: 3 of 38
for a `ze_ssh`-local change, with 36 feature tags declared in `feature-gates.txt`.
Those two rows are `all_features` and `core_only`, and `validateScoped` refuses
any scope that subtracts one of them. `./le go staticcheck check`
typed on its own judges every row, because only a verify run publishes the
feature-tag answer that `ZE_VERIFY_SCOPE_TAGS` (`ScopeTagsKey`) names.

Whatever rows the scope leaves are then CUT across six stages,
`check part 1 of 6` through `check part 6 of 6` (`staticcheckParts`,
`internal/le/verify/engine/stages.go`). `Matrix.Part` deals the rows round robin,
so each row is judged by exactly one piece and the pieces together judge them
all. A CI red names the piece it came from, and that command reproduces it:

```
./le go staticcheck check part 3 of 6
```

A piece dealt no row says so and passes: its rows are judged by a sibling piece
of the same run. One run is bounded at 90 seconds for each row it judges, so the
bound follows the size of the piece.

Suite selection narrows only where the recorded suite map answers.
`selectSuites` (`internal/le/test/functional/suitemap.go`) runs every suite
while the map is being recorded, when the map cannot be read, and when the
change-set selector refused or widened; otherwise it keeps the suites the map
records as reaching a changed package. `go list -deps ./cmd/ze` links most of
the module, so the map is measured, not derived from imports. The contract is
`../architecture/testing/verify-freshness-scope.md`, "What a suite REACHED".
<!-- source: internal/le/test/functional/suitemap.go -- selectSuites -->

## Scratch files

`tmp/` is keyed per CHECKOUT, not per session (`internal/le/scratch/scratch.go`),
so every concurrent session in the tree shares it. A fixed name at the `tmp/` root
(`tmp/out.log`, `tmp/stdout`, `tmp/gotest.log`) collides with a sibling session
writing the same name, and nothing removes it when either session ends.

A file written directly at the `tmp/` root is refused on both surfaces that create
one: `bashScratch` in `internal/le/hookruntime/bash.go` and the Write/Edit path
check in `internal/le/hookruntime/writeedit.go` both call `isAdHocScratch`
(`internal/le/hookruntime/runtime.go`). A path carrying a directory component
passes, and so do the root names that are shared by design: `ze-verify*`,
`commit-*`, `delete-*`, `mutation*`, `test-timings*`.

```
dir=$(./le session scratch ensure)          # <session-dir>/scratch/, created for you
./le test unit all > "$dir/unit.log" 2>&1
```

A session directory outlives its session, so a log written today is there
tomorrow. Nothing removes it at session end or from a hook. `./le session clean`
removes only session directories whose owners are provably gone, and the hourly
store trim (see "When the disk is full") removes a directory only when those same
rules call it dead AND its mtime is more than a day old. Inside a directory it
keeps, the trim removes a `testbin-pid-<pid>-<label>/` that is more than six hours
old and whose run is over: the pid is not running, or the process now running
under that pid started after the directory was made. A process that has exited
and is not yet reaped (a zombie) is not running, here and in every rule that
reads the process table (`internal/le/procscan`). A `testbin-<suffix>/`
(an explicit `ze.suffix`) leaves only with its session; a suffix starting with
`pid-` is refused, because it would name a throwaway set. When the process table
shows no Claude CLI, as inside a QEMU guest or a container, the trim cannot tell a
live session from a dead one and removes none of these. Artifacts that are
already session-keyed, and the shared-by-design ones (`tmp/ze-verify.*`, and
the durable Go build cache `internal/le/go/toolchain` assigns), stay where they
are.
<!-- source: internal/le/test/functional/binaries.go -- binaryRoot, throwawaySetPrefix -->

## Session identity, and what each agent owns

Every `./le` command resolves one harness session id: `CLAUDE_CODE_SESSION_ID`
first, then the `--session-id` of an ancestor process, then the access token, and
last a minted id (`resolveSessionID`, `internal/le/le/path/session.go`). The id
names the dated session directory and its `scratch/`.

A subagent runs under its parent's session. The pretool Bash hook prefixes each
of its calls with `export ZE_AGENT_ID=<agent_id>; export
CLAUDE_CODE_SESSION_ID=<parent>` from the hook payload
(`writeParentSessionPrefix`, `internal/le/hookruntime/session.go`). An agent id
that cannot name a file blocks the Bash call (hook exit 2) rather than run it
under the parent's id alone. A payload that names an agent but carries an
absent or unsafe `session_id` blocks the same way, and the pretool Write/Edit
weakening check refuses its edits (`writeWeakening`,
`internal/le/hookruntime/writeedit.go`), because either one would otherwise
act as the main thread. The budget counts every agent id this identity accepts
(`lepath.ValidAgentID`). A fork subagent (`CLAUDE_CODE_FORK_SUBAGENT`, no
`agent_id` in its payload) gets only the session id, so it shares the main
thread's identity and everything below. Every agent of a session shares the
session directory and its scratch, and owns what follows:

| Artifact | Keyed on | Main thread's file | An agent's file | Producer |
|----------|----------|--------------------|-----------------|----------|
| Commit namespace (8 hex): the commit script and the message | session and agent | `tmp/commit-session-id-<session>` | `tmp/commit-session-id-<session>+<agent>` | `lepath.CommitSession` |
| Spec claim | session and agent | `tmp/session/.session-<session>` | `tmp/session/.session-<session>+<agent>` | `lepath.SpecClaimMarker` |
| RFC approval | the session alone, because the `ze-test-judge` verdict serves the main thread, whichever agent records it | `tmp/commit-rfc-approved-<ns>.md`, `<ns>` the main thread's namespace | the main thread's file | `lepath.SessionCommitSession` |
| Test weakening approval | the session alone, for the same reason | `tmp/commit-weakening-approved-<ns>.md`, `<ns>` the main thread's namespace | the main thread's file | `lepath.SessionCommitSession` |

A session id and an agent id hold only letters, digits, `.`, `_` and `-`, so the
`+` splits a name one way only, and no session can pose as another's agent.
The session cleaner removes a dead session's agent markers with its own
(`lepath.MarkerOwnedBy`).

An agent that claimed no spec reads its parent's claim, and its `release`
removes only its own claim. Nothing an agent writes for a commit has to be
carried by its parent: the owner's approvals are keyed on the main thread's
namespace, and the commit carries them as trailers, so a main thread commits
its subagents' work with no file of theirs in it.

The shared scratch still means one file name is one file for every agent. A tool
that writes into it picks a per-run name (`cover-<pid>.out`, the discrimination
`run-*/` directory in `writeOverlay`). A file you write there yourself carries
a name of your own.
<!-- source: internal/le/le/path/commitsession.go -- CommitSession, SessionCommitSession -->
<!-- source: internal/le/le/path/session.go -- SpecClaimMarker, MarkerOwnedBy -->
<!-- source: internal/le/hookruntime/session.go -- writeParentSessionPrefix -->
<!-- source: internal/le/test/weakened/approve.go -- ApprovalPath, approveAnswer -->

## When the storage stalls

The Linux development VM keeps the checkout on a network drive that can freeze
for several seconds at a time (owner, 2026-08-31). A stall is not a code defect
and it is not the full-disk case below. It arrives as a test that talks to a
local socket failing on a timeout, several at once, each at exactly its
deadline: a RADIUS CoA run showed eight together at `no response: read udp4
...: i/o timeout`, all at 2.00s. A stage that takes minutes longer than its
neighbours for no visible reason is the same cause.

The tell that separates a stall from a defect is REPRODUCIBILITY, and it costs
one re-run to read. A stall does not survive one: the same command, unchanged,
comes back green. The full-disk case does survive, and `df -h cache/go-cache`
answers it. A real defect survives every run.

Two things follow. A timeout red measured while another session held a lint run
or a job is evidence about the machine, not about the code, so re-run it once
before you write anything down. And a failure you tried to reproduce and could
not is the one kind `ai/rules/completion.md` lets you RECORD instead of fix, so
its journal row carries the reproduction attempt and says the storage stalled.

## Where a Go test keeps its temp dirs

Every Go test le starts, through a native unit action, a verify unit stage or
`./le job run ... command go test`, runs with `GOTMPDIR=<home>/.zt`. Go 1.27's
`t.TempDir()` creates beneath `GOTMPDIR`, and the go command writes its build
work directory there, test binaries included, so both sit on the device that
holds your home rather than under `TMPDIR`. A work directory holds 0.5-0.7G
while a package builds, so that room now comes from the home device. On the
Linux VM `TMPDIR` is `~/.cache/tmp`, on a second disk whose `fsync` under
concurrent writers was measured ten times slower: the
storage tests took 271s there against 75s on the home device, and whole
packages hit the 20m timeout (`plan/journal/full-disk-false-red.md`,
2026-10-16). `TMPDIR` itself is left alone, because the socket helpers that
call `os.MkdirTemp("")` chose it for path length.

The base must stay short. A socket under `t.TempDir()` adds up to 79 bytes to
it, and `sun_path` holds 107, so a base longer than 23 bytes is refused. A
home that does not resolve, a base that long, or a directory a test cannot
write leaves `GOTMPDIR` unset; le then writes one line to stderr naming the
reason and its fix, and the run keeps Go's default. The directory is created
`0700` on first use. `df -h ~/.zt` answers how much room it has. Like
`GOCACHE`, a `GOTMPDIR` you exported yourself is overridden on a test run.

Every le-driven Go command (`go test`, `go vet`, `go list`, the functional
binary builds, golangci-lint and Staticcheck) runs with `-trimpath` in
`GOFLAGS`. Go hashes the package directory into every compile action ID
unless the flag is set, so without it each fresh verify worktree compiled the
module cold. `TestASecondCheckoutPathReusesTheFirstOnesTestResult`
(`internal/le/go/toolchain`) proves the reuse: one module tested from two
paths that share a cache answers `(cached)` the second time, and does not
without the flag. The entry is composed from the value Go would use anyway (your
exported `GOFLAGS`, else `go env GOFLAGS`, which reads a `go env -w` setting),
so your own flags are kept and `-trimpath` is never doubled. If Go cannot say
what that value is, le writes one line to stderr and runs the command without
the flag. The `go env GOFLAGS` query runs under the caller's signal root where
it holds one (`EnvironmentContext`, and `job` for a `go test` child): after a
Ctrl-C or a SIGTERM has ended the verb it starts no go process and prints no
line, because the command it would have served does not start either. The tracked-build gate keeps the flag in its own argv, because it
does not use the toolchain environment. Two consequences: `runtime.Caller`
answers a module-path-prefixed name, so a test finds the repository from its
working directory (`os.Getwd`, the package directory) and never from its own
source file; and the first le run after the flag landed rebuilt every cache
once.
<!-- source: internal/le/go/toolchain/gotoolchain.go -- Environment, EnvironmentContext, TrimpathGoFlags, EffectiveGoFlags -->

A reused test result is only as good as the environment it was recorded
under, and Go's test cache sees little of it. It hashes only the variables a
test read through `os.Getenv` or `os.LookupEnv`; a read through `os.Environ`
logs nothing (`computeTestInputsID`, `cmd/go/internal/test/test.go`), and <!-- doc-links: ignore (Go toolchain source, outside this repository) -->
`internal/core/env` builds its cache from `os.Environ`. Every Ze setting a test
reads through `env.Get` is therefore invisible to it: change one in your shell
and the next run may still answer `(cached)`. Run with `-count=1` after you
change one. `TestTheTestCacheSeesGetenvButNotEnviron` shows this.

le makes sure the gate never changes such a value itself, so two
`./le verify worktree` runs Thomas starts over one commit from the same shell hand every test
the same environment:

- `GOCACHE`, `GOLANGCI_LINT_CACHE` and the patched `GOROOT` (with its `bin`
  first on `PATH`) are named where the worktree's links land, never through
  the worktree (`gen.Landing`, used by `Overrides` and `gen.CacheDir`). A
  checkout with no links keeps its own spelling.
- A test child does not inherit `ze.repo.root`, `ze.le.build.name` or the
  gate's change scope (`ze.verify.scope.packages`, `ze.verify.scope.tags`),
  which name files under one run's worktree.
- `PWD` is the package directory, set by `go test` itself. It differs between
  worktrees, but `os.Getwd` and `os.Getenv` log their read of it, so a test
  that uses it misses the cache instead of reusing a stale answer.

`TestTwoWorktreesGiveATestChildOneEnvironment` (`internal/le/go/toolchain`)
proves it: two worktrees laid out as the gate lays them out give one test
environment, and a package that scans `os.Environ` is answered `(cached)` in
the second with the verdict a `-count=1` rerun gives. The one value that still
differs between runs is `ze.run.job`, the job registry entry of the run that
holds the slot: a nested admission reads it to run inside its parent. A test
child keeps it on purpose. Its bytes differ, but the only answer a test can
draw from it is "inside a live parent", which holds in every gate run, so a
cached verdict cannot differ from a rerun's because of it. Stripping it would
make a test that admits through the real registry queue behind the gate's own
slot, and that can deadlock.
<!-- source: internal/le/job/job.go -- Admission.insideParent -->
<!-- source: internal/le/go/stdpatch/gen/stdpatch.go -- Landing, CacheDir -->
<!-- source: internal/le/go/toolchain/gotoolchain.go -- Overrides, EnvironmentContext -->

Under `sudo -E`, the test runs as root with your `HOME`, so root creates
root-owned entries in `~/.zt` that your next run cannot use or remove. Give
them back with `sudo chown -R $USER ~/.zt`.

A test binary removes its own temp dirs when it ends, and the go command its
`go-build*` work directory, so what stays under `~/.zt` is what a killed run
left. The hourly store trim removes those on its `test-temp` row (see "When
the disk is full"), and `./le scratch trim` does the same on demand: a
`go-build*` entry more than 2 hours old, and any other entry more than 6 hours
old. A work directory's mtime moves each time a package build starts or ends,
so 2 hours is safe while packages build; once a test binary runs, its work
directory gets no new child and the binary still executes from it (a fuzz
coordinator, and every test that re-executes its own binary), so one test
binary running longer than 2 hours can lose it. A `t.TempDir()` parent's
mtime moves only when the test makes another temp dir. A run given a
`-timeout` or `-fuzztime` longer than either age, or `-timeout 0`, can lose
its work directory or its parent.

A killed permission test can leave a directory chmodded to `0o000`. The trim
gives each directory of an old entry back to its owner (`0700`) before it
removes it, so such a leftover goes with no error. An old entry it cannot
open is one this user does not own, which a `sudo -E` run leaves; the row
error then names `sudo chown -R $USER ~/.zt`. A young entry it cannot read
is a run still holding it: the row names it as contention and keeps it, and
the row stays green.
<!-- source: internal/le/go/toolchain/gotoolchain.go -- TestTempDir, TestTempBase, testTempPathMax, makeTestTempDir, Overrides -->
<!-- source: internal/le/job/process.go -- commandEnvironment -->
<!-- source: internal/le/scratch/livetrim.go -- testTempRow, trimTestTemp, openTestTempEntry, testTempOpenFailure, goBuildIdleAge, testTempIdleAge -->

## When another session cleans the cache under you

`cache/go-cache/...: no such file or directory`, or `cache entry not found`, on
files that are there, is a cache entry removed under a running build (owner,
2026-08-31). Sessions share this checkout and they share the build caches. Both
cleaners, the store trim and `./le scratch clean` by default, remove only
entries older than 3h. An entry is safe for 2h after Go last looked it up: Go
refreshes an entry's mtime when it uses one, but at most once an hour, and the
floor is 3h. An entry a build last looked up more than 2h ago can be removed,
and the build's next lookup misses and recompiles it rather than failing. So a
cleaner can cause this error only for a build that looked an entry up and had
not opened it 2h later; a single retry clears that, because the retry
repopulates what it needs.
<!-- source: internal/le/scratch/cachetrim.go -- cacheEntryAgeFloor -->
<!-- source: internal/le/scratch/cacheclean.go -- cleanOldCacheEntries -->

The floor and the hourly refresh are both fixed: `cacheEntryAgeFloor` in le,
`mtimeInterval` in Go's `cmd/go/internal/cache`. <!-- doc-links: ignore (Go toolchain source, outside this repository) -->

The full empty, `./le scratch clean all`, is the cause that does not
clear so easily: emptied mid-run, the staticcheck matrix lost its parts 2 and 3
on 2026-10-21. It refuses while another session's build may be using the
caches. It names each process: pid, command line, and the job label when the
process holds a `./le job run` or verify slot. Wait, or run the default. The
check is a snapshot taken before the empty starts, so a build that starts while
the empty runs is not seen and can still lose its entries.

Do not read it as a full disk: that case says `no space left on device` and
survives a retry. Do not read it as a code defect either. Retry the command
once.

Store trim charges allocated blocks for directory entries as well as files.
An `ENOTEMPTY` directory remains charged; `ENOENT` releases its recorded bytes
without counting a removal by this run. Contention fixtures budget the measured
retained directory plus the newest file, rather than assuming an empty
directory uses no blocks.
<!-- source: internal/le/scratch/cachetrim.go -- treeBytes, allocatedBytes, walkCacheSubdirectory, removeOldest -->

## When the disk is full

A full cache disk is read as a code defect often enough to have its own class
file. `plan/journal/full-disk-false-red.md` carries one row for each occurrence,
and that file is the count: no page repeats it, because a copy goes stale on the
next row. It arrives as a wave of unrelated failures:

- Packages that do not import each other fail to build.
- The linker says `mapping output file failed: no space left on device`.
- A verification stage reports `cache entry not found`.
- A whole functional suite goes red at once.

Read the device that holds the CACHE, by naming the cache path itself:

```
df -h cache/go-cache          # follows cache/ to the device that fills, linked or not
df -h ~/.cache/ze/go-cache    # the shared per-user cache verify worktrees write
df -h cache/golangci-lint-cache # the lint cache, under cache/ with the others
findmnt -T cache/go-cache     # Linux: which device that path is on
```

`df` on the checkout ROOT can answer about the wrong device. The per-user cache
target is `$XDG_CACHE_HOME/ze`, or `~/.cache/ze` (`internal/le/scratch/scratch.go`,
`cacheTarget`), and that target is frequently its own filesystem. Every le run
in a Ze checkout makes the checkout's `cache/` the symlink to it before anything
can create `cache/` as a real directory. A checkout is a tree holding `go.mod`
beside `feature-gates.txt` (`lepath.IsCheckout`): a tree `ZE_REPO_ROOT` names
without them, such as the published site, the wiki or a test's copy of a few
subtrees, builds no Ze and gets no link. So `cache/go-cache`, which every le action writes,
and `~/.cache/ze/go-cache`, which every verify worktree writes through its own
link (see "A verify worktree shares that cache" below), are ONE cache. The
step runs in three places, because the `le` and `ze` scripts build their binary
before any le binary exists: the `le` script's `ensure_cache_link` ahead of
`build_le`'s `mkdir`, the same function in the root `ze` script ahead of the
`GOCACHE` it builds `bin/ze` through, and `ensureCacheLink` in le's `run`
(`internal/le/register.go`), ahead of the patched GOROOT that creates
`cache/stdpatch`. The last covers a `bin/le` started directly, such as in a
git-archive export. `TestLauncherLinkMatchesCacheTarget` (`internal/le/scratch`)
runs both scripts' copies, compares the link each makes with `cacheTarget`, and
fails when the two function bodies differ.

What each run finds, and does:

| `cache/` is | The run |
|-------------|---------|
| Absent | Creates the link, silently |
| The link to the target, with or without a trailing slash | Nothing: one `Lstat`, one `Readlink`, no process |
| A real directory | Renames it to `cache.stale-<UTC timestamp>-<pid>` beside it, creates the link, and prints one stderr line naming both paths and the copy's size. Nothing is deleted, and stdout stays the action's own |
| A link to another place | Nothing, silently: a QEMU guest sees the host's link dangling, and the `le` script's fallback to `tmp/go-cache` handles that |

Two le runs that find the same real directory both succeed: the one whose
rename finds nothing to move, or finds the link already made, takes that as
done. Go's cache is content addressed and written by rename, so a build running
through the old directory during the move can fail once, and never gets a wrong
entry: an entry it wrote before the move is gone from the path it reads back,
and it stops with `open .../cache/go-cache/<xx>/<id>-d: no such file or
directory`. Run it again: it builds into the per-user cache. The stderr line
the repair printed names the move, so that failure is attributable. `./le scratch ensure` and `./le scratch migrate` still link
`tmp/`; for `cache/`, `scratch ensure` makes the same repair, and `migrate` moves
the entries into the target instead. Naming each cache path rather than the
checkout is still what makes `df` answer correctly.
<!-- source: internal/le/scratch/scratch.go -- EnsureCache, moveRealCacheAside, repairedResult, Announce -->
<!-- source: internal/le/register.go -- run, ensureCacheLink -->
<!-- source: internal/le/le/path/lepath.go -- IsCheckout -->
<!-- source: le -- ensure_cache_link, build_le -->
<!-- source: ze -- ensure_cache_link -->

The built kernels under the same per-user target, `~/.cache/ze/runtime-kernel/`
and `~/.cache/ze/installer-kernel/`, are NOT a reclamation target, and no
cleanup removes them: not `./le scratch clean`, not the store trim, and
not a hand `rm` while recovering a full disk. A cold kernel build takes about
thirty minutes, the arm64 kernel is built on the Mac and the amd64 kernel on the
Linux host, and neither host can rebuild the other's. The only thing that
removes an entry is the eviction after a newer build, which keeps the two newest
entries of each architecture and never the entry the current tree resolves to
(`evictKeepN`, `internal/appliance/cache.go`).
The clean and the trim both take their targets from one list, and that list
skips any target that lies within a kernel cache or holds one, whatever path it
resolved to, so a `scratch clean` row for such a target reads as a skip that names
the kernel cache. `TestCleanTargetsNeverReachTheKernelCache` and
`TestCleanTargetsSkipATargetHoldingTheKernelCache` (`internal/le/scratch`) fail
when a clean or trim target reaches into either directory.
<!-- source: internal/appliance/cache.go -- KernelCacheNamespaces, evictKeepN -->
<!-- source: internal/le/scratch/cacheclean.go -- cleanTargets, pathWithin -->

`stat -f` is NOT the command to reach for, and the 2026-09-12 and 2026-09-13
rows in the class file are both that mistake. On macOS `stat -f` takes a FORMAT
string, so `stat -f cache/go-cache` prints `cache/go-cache` back and diagnoses
nothing. The GNU filesystem mode exists only on Linux, and even there APFS-style
purgeable accounting is what made one reading report terabytes free on a volume
at 98 percent.

Five caches fill, and emptying one leaves the others full. The checkout's
`cache/` links to the per-user target, so its Go cache `cache/go-cache`, its
golangci-lint cache `cache/golangci-lint-cache` and its Staticcheck matrix cache
`cache/staticcheck-matrix` (`gostaticcheck.CacheDir`) are the host's one copy of
each, shared by every checkout and verify worktree. The lint cache is not a Go
cache, so no `go clean` reaches it; it held 9.5G on 2026-09-13, when it still
sat under `tmp/` on the checkout's own device. Staticcheck trims its own cache
only once a day and keeps five days of entries, so it held 9.3G on 2026-10-23.
The ambient cache is the one a bare `go` command writes, and the bootstrap
cache `tmp/go-cache` is the `le` launcher's fallback where `cache/` cannot be
created. The launcher can still write it on any host, so its row stays on a
linked host too, where it is normally a skip because the directory does not
exist: a cache no row declares is one the cleaner walks past. Each
`cache.stale-*` copy the link repair left is a row of its own too, named
`stale-1`, `stale-2` and so on in repair order, oldest first, so two copies are
two rows the report and the trim tell apart; the row's path carries the stamp.

Reach for the safe tools first, in this order, because other sessions are
building against these caches while you read this:

1. `./le scratch trim` trims the caches to their budgets now (see below).
2. `./le scratch clean` cleans every cache. By default it removes every
   cache entry older than 3h, the store trim's floor, and keeps the younger
   ones. It is safe while other sessions build: an entry is safe for 2h after Go
   last looked it up (Go refreshes an entry's mtime at most hourly; the floor is
   3h), and a build that later misses an older entry recompiles it.
3. `./le scratch clean all` empties every cache outright. Use it only when no
   build runs: it refuses, and exits 1, while it sees any `go`, `compile`,
   `link`, `vet`, `asm`, `cgo`, `cover`, `staticcheck`, `golangci-lint` or
   compiled test binary (`*.test`) process, or any process holding a slot in
   this checkout's job registry (`./le job run`, a verify run). The refusal
   names each one with its pid, command line and job label. A process table
   or registry it cannot read refuses too, because it then cannot tell whether
   a build runs. The check is a snapshot before the empty starts: a build that
   starts during the empty is not seen, and can fail with `cache entry not
   found`. It reads program names, not each process's environment, so a
   build in an unrelated checkout also refuses it. That is the right answer,
   because the ambient cache it empties is the default every `go` command
   without its own GOCACHE writes.

The default prints one row for each cache, in this shape:

```
$ ./le scratch clean
checkout    /path/to/checkout/cache/go-cache                 removed 81.3G (402118 entries), kept 7.7G younger than 3h0m0s, freed 81.2G, free 115.4G
ambient     SKIP     /Users/thomas/.cache/ze/go-cache: the machine default is the checkout cache, which this run already cleaned
bootstrap   SKIP     /path/to/checkout/tmp/go-cache: does not exist, so it holds nothing to trim
lint        /path/to/checkout/cache/golangci-lint-cache      removed 8.8G (90214 entries), kept 0.7G younger than 3h0m0s, freed 8.8G, free 124.0G
staticcheck /path/to/checkout/cache/staticcheck-matrix       removed 8.1G (51022 entries), kept 1.2G younger than 3h0m0s, freed 8.1G, free 132.1G
stale-1     /path/to/checkout/cache.stale-20261024T101500Z-4242 removed 20.0G (1 entries), kept 0.0G younger than 3h0m0s, freed 20.0G, free 152.1G
```

`removed` is the size of the entries past the floor that are gone after the run:
the ones it deleted, and any a concurrent trim deleted first, because both
returned that space. The entry count in brackets is only what this run deleted.
`kept` is what the floor kept, and `freed` and `free` are the device's own
reading before and after, taken on the cache path. `all` prints the same rows
without the `removed` and `kept` parts. A cache that does not exist yet is a
skip, not a refusal. `| json` renders each row with the keys `removed`,
`entries-removed` and `kept-young` beside `freed`.

The ambient row is a SKIP when `go env GOCACHE` lands in the same directory as
the checkout cache, compared through the link rather than as spelled, which is
the case on a host where `go env -w GOCACHE` names the shared cache. When
either path exists but cannot be resolved (a directory this user cannot
search), the row is an error naming that path and nothing cleans it, because
le cannot tell whether it is the checkout cache.
<!-- source: internal/le/scratch/target.go -- SameDirectory -->
A stale
copy is removed whole once the repair stamp in its name is older than the 3h
floor, and kept whole before that, because a build already running when the
repair moved `cache/` aside may still read it. A name with no stamp is refused
rather than given a guessed age.

The patched GOROOT trees under `cache/stdpatch/` (or `tmp/stdpatch/` where
`cache/` is not usable) are not emptied, because a build in another session may
be compiling with `GOROOT` inside one. Each key gets a `KEEP` row with its size.
Both modes drop the two kinds no build can be using, each with a `DROP` row: a
key whose `VERSION` or `src/go.mod` link names nothing on this machine (the
package manager replaced the toolchain in place; such a tree also made
`go list ./...` walk into its `goroot/src`), and a `tmp-*` staging directory
more than three hours old, which a killed tree build left
(`patching-the-standard-library.md`):

```
stdpatch           KEEP     /path/to/checkout/cache/stdpatch/94a7…             27.0M, a patched GOROOT a running build may use
stdpatch           DROP     /path/to/checkout/cache/stdpatch/398f…             26.8M, its toolchain links name nothing on this machine, so no build can use it
```

The CHECKOUT cache is `cache/go-cache`. Every le action writes it, because
`Overrides` (`internal/le/go/toolchain/gotoolchain.go`) points GOCACHE there, and
`gotoolchain.GoCache` names it; the `cache/` link carries it to the per-user
target, which every verify worktree reaches through its own `cache/` link, so
no second row names it. The AMBIENT cache is the one a bare `go` command writes
outside le. The action asks `go env GOCACHE` for that path with the inherited
override removed. The BOOTSTRAP cache is `tmp/go-cache`
(`gotoolchain.BootstrapCache`), which only the `le` launcher writes, and only
when `cache/` cannot be created. The LINT cache is `cache/golangci-lint-cache`,
which `Overrides` names through `gotoolchain.LintCache`. It keeps Go's entry
layout, so the default mode walks it with the same age rule; `all` empties it by
deleting the directory, because no `go clean` owns it. The STATICCHECK cache
keeps the same layout, written by Staticcheck's copy of Go's cache code
(`honnef.co/go/tools/lintcmd/cache`, `DiskCache.fileName`), so the default mode
walks it the same way and `all` deletes it. `all` is the equivalent of this by
hand, which carries the same hazard and none of the refusal, so do not type it
while a build runs:

```
go clean -cache                                    # the ambient cache
env GOCACHE="$PWD/cache/go-cache" go clean -cache   # the checkout cache
env GOCACHE="$PWD/tmp/go-cache" go clean -cache     # the bootstrap cache
rm -rf cache/golangci-lint-cache                   # the lint cache
rm -rf cache/staticcheck-matrix                    # the staticcheck cache
rm -rf cache.stale-*                               # the stale copies
```
<!-- source: internal/le/scratch/cacheclean.go -- CleanCaches, cleanTargets, cleanOldCacheEntries, cleanStaleCopy, patchedTrees -->
<!-- source: internal/le/scratch/cachetrim.go -- trimStaleCopy -->
<!-- source: internal/le/scratch/cacheuse.go -- refuseFullEmpty, cacheUsers, cacheReaders -->
<!-- source: internal/le/job/holders.go -- Holders -->

The clean costs recompilation, which makes the next run slow once, so le also
bounds every cache but the ambient one by itself, a little at a time. Every le
invocation reads `tmp/store-trim/stamp`. When the stamp is an hour old, one
invocation takes `tmp/store-trim/stamp.lock`, rewrites the stamp and starts
`le scratch trim background` as a detached child, so no command, hook
calls included, waits for the walk. The child trims:

| Caches | Budget | Default |
|--------|--------|---------|
| checkout and bootstrap, as ONE union | `ze.le.store.go-cache-budget` | `20G`, one total across the two |
| lint | `ze.le.store.lint-cache-budget` | `10G` |
| staticcheck | `ze.le.store.staticcheck-cache-budget` | `5G` |
| each `cache.stale-*` copy | none: removed whole once its repair stamp is past the 3h floor | - |

Each pass removes `-a` and `-d` entries from the 256 two-hex-digit
subdirectories, oldest mtime first, until the total is at or under the budget.
In the Go pass that order runs across all three caches at once, so a cache that
is under 20G on its own can still lose its coldest entries to the others' use.
The Staticcheck pass does the same across its two caches, and its own daily
trim, which keeps five days, still runs when a matrix run closes the cache.
Nothing else in a cache directory is touched, and no `go clean` runs. No entry
younger than three hours is removed, whatever the budget: when that is what
stands between the cache and its budget, the report says the budget is unmet
and how many bytes the floor kept. An entry a peer removed first counts as gone
but not as removed by this run. The ambient cache is not le's, and Go's own
five-day trim keeps it: its row names the path and is a skip.

A budget is a whole number followed by `M` or `G`, from `1M` to `1048576G`. Any
other spelling, a bare number included, refuses that budget's caches by name and
leaves them untouched, while the rest are trimmed. `ze.le.store.trim=off` stops
le from starting a trim by itself, and the functional test runner gives every
test child that setting (`docs/architecture/testing/runner-architecture.md`).
`./le scratch trim` runs one in the
foreground whenever asked, ignoring the stamp, and prints one line for each
budget and one row for each cache (`| json` renders the same report). The
background child writes that report to `tmp/store-trim/last.log`, which holds
the latest run. The action exits 1 when a budget, a cache path or the
`ze.le.store.trim` value was refused; a contended entry or an unmet budget is
reported and exits 0.
<!-- source: internal/le/scratch/storetrim.go -- storeTrimBeforeDispatch, startTrimWhenDue, trimStores, trimReport -->
<!-- source: internal/le/scratch/cachetrim.go -- trimCaches, removeOldest, walkGoFormatCache -->

After the caches, the same child trims the stores a live session may be using,
by age and liveness and never by size: named launchers (`launchers`, see
"Launcher builds do not overwrite running binaries"), session directories
(`sessions`) and orphaned testbins (`testbins`, both in "Scratch files"), the
launcher's builds (`builds`) and what agents leave directly in `bin/`
(`agent-files`). It scans the process table once for all of them. When that
table shows no Claude CLI, every one of these rows says `no session process
visible` and nothing is removed from them; the caches are trimmed either way. A
row counts what it removed and what it kept, and an error in one of these rows
also makes the action exit 1. With no `tmp/session/`, or when the session
directories cannot be judged, the `testbins` row is a skip that says which.

Every rule below also needs that no running process names the entry's path in
its argv, as whole names (`bin/gomu` does not protect `bin/gomu-rfc`), so a
command still running from a superseded build keeps its file.

| Row | Entry | Removed once |
|-----|-------|--------------|
| `builds` | `bin/builds/<name>/` | it is older than the newest two runnable builds of its platform; the newest, which le runs, and the one `./le rollback` falls back to always stay |
| `builds` | `.failed-<name>[.<pid>]/`, a rolled-back build | a newer build of its platform landed |
| `builds` | `.failed-<commit>.log`, `.reported` | a build of another commit landed after it, so HEAD moved on |
| `builds` | `.staging-<pid>/` | nothing in it was written for an hour |
| `builds` | `.previous-legacy/` | a build of this checkout's own platform exists |
| `builds` | `.building/`, anything else | never: the launcher reclaims a lock whose owner died |
| `agent-files` | any `bin/` entry but `le`, `le-*`, `le.*`, `ze*` and `builds` | nothing in it (its newest mtime, the whole tree for a directory) was written for a day; a symbolic link is never removed |

The last row, `test-temp`, is the per-user test temp base `~/.zt` (see "Where a
Go test keeps its temp dirs"). Age alone judges it, with or without a Claude
CLI in the process table, because every checkout and session shares the base
and a run still writing an entry is younger than the floor: a `go-build*` work
directory more than 2 hours old and any other entry more than 6 hours old is
removed. A home with no usable base makes the row a skip that says
why.
<!-- source: internal/le/scratch/livetrim.go -- trimLiveness, trimSessions, trimTestbins, trimBuilds, trimAgentFiles, pathNamedInArgv, trimTestTemp -->
<!-- source: internal/le/session/reap.go -- Judge -->
<!-- source: internal/le/procscan/procscan.go -- Scan -->

### A verify worktree shares that cache

Thomas's release check, `./le verify worktree`, extracts a detached worktree under `tmp/verify-worktree`
and links its `cache/` to the per-user target, before any stage runs
(`internal/le/verify`, `sharedCacheLink`, calling `EnsureCache` in
`internal/le/scratch`). That target is the checkout's own cache only when the
checkout's `cache/` is linked to it too; otherwise it is the second, shared
cache that `scratch clean` reports on its `shared` row. `tmp/` is NOT linked with it: that one is per-checkout,
and the worktree's own `tmp/verify` is where the run writes the stage logs it
copies out afterwards.

Without that link `Overrides` resolves GOCACHE to `<worktree>/cache/go-cache`
(`internal/le/go/toolchain/gotoolchain.go`), so each run compiled from cold into a
private cache and deleted it unread. Two worktrees measured on 2026-09-03 held
7.4 GiB and 6.7 GiB of private cache against a 0.6 GiB source tree, which is
about 90 percent of each worktree, rebuilt once per run.

The run prints what it did with the link, and the line names the shared path:

```
verify-worktree: shared build cache: created  cache -> /Users/thomas/Unix/cache/ze
```

A cache that cannot be linked is reported on that line and the run continues
against a cold one. The link is speed and disk, never correctness.

The run also fills the worktree's `gokrazy/modcache` from the checkout's
(`sharedModcacheLink`, `internal/le/verify/modcache.go`). Git tracks only the
gokrazy init source there, and `ze appliance build` resolves its modules with
`GOPROXY=off`, so a worktree carrying only tracked files failed every appliance
build with "looks unpopulated" whatever the checkout held. Each entry the
worktree lacks becomes a symlink to the checkout's; a tracked file is never
replaced, and every link lands on a path the cache's `.gitignore` ignores. The
line reads:

```
verify-worktree: gokrazy module cache: linked 23 entries from <checkout>/gokrazy/modcache
```

A checkout with no cache is reported on that line with `./le setup install` in
that checkout as the fix, and the run continues: the appliance stage then fails
with its own cause.

### What the gate is holding

A verify worktree that carries uncommitted changes is never deleted, whoever
abandoned it (`sweepAbandoned`, `internal/le/verify/cleanup.go`). The run names
each one it kept, with the path, the age, the disk it holds, and the shape of
its dirt:

```
verify-worktree: 20260903T091500-p123-r1-abcdef012345 was abandoned but holds
uncommitted changes, so it is left alone: /path/to/it, 8h12m0s old, 8.3G,
0 modified, 0 untracked, 264 deleted
```

The shape decides what to do. Modified or untracked content exists only there,
so removing the worktree destroys it. Deletions alone are a tree somebody
emptied, and git restores every one of them from the commit the worktree is
detached at. Remove one by hand with `git worktree remove --force <path>` after
reading that line, never on a schedule.

### A run that judged nothing

Thomas's release check, `./le verify worktree`, answers five statuses, and they are not interchangeable:

| Status | Meaning |
|--------|---------|
| 0 | Every stage ran and the tree passed |
| 1 | A stage judged the tree and found it wrong |
| 2 | The run itself broke before it could judge |
| 3 | The run reached no verdict: a stage could not judge its subject, or a full device defeated the run |
| 130 | An interrupt stopped the run, between stages or during one |

A full device is recognized by its typed error at the write site, never by
matching text (`Defeated`, `internal/le/verify/engine/run.go`). Status 3 says
the tree was never cleared, so it is neither a pass to build on nor a red to
debug: free the device and run it again.

The verdict line is the LAST line the run prints, and it states the status the
process exits with:

```
verify-worktree: full exit=3
```

The run publishes its certificate to this checkout's `tmp/ze-verify.status`
before it removes the worktree, so `./le verify status check` answers about the
run that just ended (`../architecture/testing/verify-freshness-scope.md`).

## Session binaries

Native test actions build their binaries inside the current session's private
directory, under bare names, so a sibling session cannot overwrite the binary
under test. Ask the owning action for the path rather than writing `bin/ze`.

The directory carries the session id, so the file name does not. That is what
keeps argv[0] personality dispatch working (`defaultDispatch`,
`cmd/ze/dispatch.go`, looks the base name up as a root, so a file named `le` runs
le) and lets a `.ci` test exec `ze` by bare name off one PATH entry. A binary's location also decides where
`ze` resolves its config and database (`ConfigDirFromBinary`,
`internal/core/paths/paths.go`), so a session's `ze` reads `<session-dir>/etc/ze`
and the repository's `etc/ze` belongs to the human alone.
`internal/test/sessionpath` is what a `.ci` test uses to find it.

That directory is LOOKED UP, never recomputed: every consumer takes the single
directory matching `tmp/session/????-??-??-<id>`, and names a new one with today's
date only on a miss. Recomputing from today's date would move a session's
directory at midnight and orphan the binaries it is running.

The session-local `etc/ze` is seeded once by
`./le session seed-store binary <session-bin>/ze`. `seedStore`
(`internal/le/session/seed.go`) validates that the binary belongs to the current
session directory. Credentials are generated per session: user `admin`, and a
random password at `<session-dir>/etc/ze/.dev-password`, mode 0600. A later seed
preserves the existing store and does not rotate the credentials.
The seeder runs `ze init --seed`, which writes a `database.zefs` artifact and
skips interface discovery on the development host. It then runs
`ze init --from` to import that artifact into the live `etc/ze/database` tree.
A `database.zefs` that is still in `etc/ze` stops the seed with the
`ze init --from` repair, because ze refuses to open a store beside a blob.

## Verify logs

Each verify run writes its own directory under `tmp/verify/<mode>-<random>/`
(`internal/le/verify/engine/run.go`), so concurrent runs never collide. It holds
one log per stage plus the combined `ze-verify.log`. The latest run is also
published at four stable paths (`internal/le/verify/engine/artifacts.go`):

| Path | Holds |
|------|-------|
| `tmp/ze-verify.log` | the combined log of the latest run |
| `tmp/ze-verify-failures.log` | the human failure index |
| `tmp/ze-verify-failures.json` | the machine failure index |
| `tmp/ze-verify-full.json` | the latest FULL-mode machine index, kept when a fast run publishes a cheaper result |

Piping such a run through `head` or `grep` loses the failure line and costs a
re-run. Run it clean, then read the log with paging.

## Asking a run in flight whether it reddened your file

A full pass takes tens of minutes, and most of it is two whole-tree Go
analyses. Waiting for the published index above to answer "did this run redden a
file of mine" costs that whole hour, and the answer is already on disk: each
stage writes its own log, carrying its `VERIFY FAILURE GROUP:` declaration, the
moment that stage ends.

```
./le verify reds file internal/component/bgp/reactor/peer.go
```

It reads the newest run directory under `tmp/verify/`, whether or not the run
has finished, and answers one of four verdicts
(`internal/le/verify/reds.go`, `verdictOf`):

| Verdict | Means | Exit |
|---------|-------|------|
| `named` | a stage that has reported declared a red naming this path | 1 |
| `undetermined` | nothing that reported names it, and something is unknown: stages still to report, or a red that named no file | 1 |
| `not-named` | every stage reported, every red among them named its files, and none is this path | 0 |
| `no-run` | no run directory exists, so nothing has judged anything | 1 |

**It is a query, never a certificate.** Only `not-named` exits 0, and it demands
a whole run. Every answer carries `stages`, `reported` and `pending`, so a
reader can see how much of the run has judged nothing yet. No red refuses a
commit: `./le commit create` (`checkVerificationGates` in
`internal/le/commit/prepare.go`) reports the verification certificate and reads
no failure index, because a red prevents a release, never a commit
(`ai/META-DECISIONS.md`, "Ze is pre-release"). `./le commit push` only derives
the verification debt from the recorded verify runs
(`ai/rules/precommit-verify.md`). This action tells you whether to keep working
or to go and fix something, and which reds to name in your commit body, and it
starts no run.

The answer is structured data, so `| json`, `| yaml` and `| table` each render
it.

The pretool-bash guard refuses that pipe for the areas that RUN something:
`verify`, `verify lock`, `verify deps`, `verify lint`, `functional`,
`integration`, `qemu` and `test-unit` (`heavyArea` in
`internal/le/hookruntime/bash.go`). It reads the two-word area name, so
`./le verify status check` and `./le verify summary` stay pipeable: they read
and write the verification certificate and start no run.

## Why one owner runs the suites

The reason is attribution, not speed and not memory. Suites share the build
cache, the TCP ports, and the `ze` processes they start. A run taken while
another suite is running therefore produces a red that belongs to nobody: a
killed process and a real defect read the same in a log. The repository-wide
verify lock says this for one target; the one-owner rule says it for every suite
and names who holds the right to run one.

## How long a verify takes, and when a slow one is broken

Never take a timeout from a duration written in a rule. How long a full pass takes
depends on the machine and on what else that machine is doing, so the figures in
two different rules are different hardware rather than a contradiction. `ticket.Release`
(`internal/le/job/job.go`) appends the real elapsed seconds to
`tmp/.ze-verify-duration.txt` for the machine you are on, and `tmp/` is gitignored,
so that file is the only per-machine record there is. Read it as an expectation,
never as a threshold.

A slow run is not a broken run, and there is no threshold to raise. A waiter breaks
a holder's slot only when that holder is DEAD, or when it has made no progress for
the stall window: `scanAndClaim` (`internal/le/job/registry.go`) judges progress by
the mtime of the job's log, never by elapsed time. `ZE_JOB_STALL_SECONDS`
(`StallKey`) sets that window, defaults to 1800 seconds, and is bounded to 60..3600
(`StallMin`, `StallMax`, `internal/le/job/job.go`). A value outside the range is
refused before the job starts.

## Stage times, and stopping at the first red

Every verification run appends one line per stage it reached to
`tmp/ze-verify-stage-times.tsv` at the checkout, green run or red, worktree or
in place: the stage, its seconds, its exit, the mode, the commit and its start,
tab-separated. A worktree run writes to the checkout's file, not the
worktree's, because the worktree is removed when the run ends. Each run's lines
go out in one write on a file opened for append, so two sessions appending at
once never interleave inside a line. The reader keeps the latest line for each
stage, skips any line that is not six columns with finite, non-negative
seconds, an integer exit and an RFC 3339 start, and the run's combined log says
how many it skipped. Nothing trims the file.
<!-- source: internal/le/verify/engine/stagetimes.go -- StageTimesPath, ReadStageCosts, AppendStageTimes -->

An uncut run reads that history before its first stage and runs the stages it
has no time for first, in declared order, so a new stage is measured the first
time it exists, then the rest from cheapest to dearest, a tie keeping declared
order. The combined log's `Order:` line says which order the run used. A cut
run (`part <i> of <n>`) keeps the dealt declared order, and `./le verify list`
always prints the declared order, because CI deals its six shards from that
list.
<!-- source: internal/le/verify/engine/run.go -- planOrder -->
<!-- source: internal/le/verify/engine/stagetimes.go -- costOrder -->

`first-red` stops the run after the first stage that exits non-zero. It is an
opt-in: without it a red stage never hides a later red.

Thomas runs `./le verify current fast first-red` in place over the
changed-files stages, `./le verify current first-red` over the full population
in a fresh worktree at HEAD, and `./le verify worktree first-red` for the same
run naming a commit with `commit <rev>`.

All three are release checks that start only at a terminal ("A full
verification is Thomas's to start", below).

A stopped run is never a pass. Its exit is the stopping stage's (1, or 3 when
that stage could not judge), its certificate is STALE for every path list, the
failure index names the stage under `stopped-at` and lists every stage of its
population it did not start with `reached: false`. The run
prints `stopped at <stage>, <n> stages not run` and the command that reruns
them: `le verify current fast` for the fast population, whose stop leaves the
full-only stages to `le verify worktree`; a stopped worktree run names
`le verify worktree commit <sha>` as a release check Thomas starts, so an agent
gives him that command.

Every run's index names the whole full population, and a stage the run did not
hold (a piece of a cut run, a fast run, a run that broke) carries the verdict
the checkout's earlier index recorded for it, marked `outside-run` with
`carried-from` naming the commit and run that judged it. A red therefore stays
red in the index until a run reaches that stage. A stage's own rerun shows
the red but publishes no index. A stage no run has judged is `outside-run`
with no verdict: not running a stage is the derived verification debt's
business, not a red. A stage an interrupt cut short is the same: its exit is
what its process died with, not a verdict, so the index writes it
`interrupted` with no exit and no group (or carries the earlier verdict for
it), with its own rerun as the command that judges it. So a green
`verify current fast` leaves no red for the stages it drops, and a green
`le verify worktree all of <m>` sweep Thomas starts over every piece leaves
none either. No commit is refused for anything the index holds. A
missing or malformed earlier index is treated as empty, and the index's
`carry-note` (the `Earlier index:` line of the text index) says so. The merge
runs under a lock on `tmp/ze-verify-failures.lock`, because sessions publish
into the same `tmp/`.
`./le verify reds` over a stopped run answers `undetermined` and says the
pending stages will never report. A first-red run is separate work in the job
registry, so it never attaches to a whole run of the same commit, nor a whole
run to it.
<!-- source: internal/le/verify/lifecycle.go -- worktreeArgv, firstRedWithPart -->
<!-- source: internal/le/verify/engine/artifacts.go -- carryForward -->

Thomas runs `./le verify worktree part <i> of <m>` for one dealt piece, every stage of it,
and `./le verify worktree all of <m>` for every piece not yet proven over one
commit, each in its own worktree. The piece that completes 1..m over one commit
writes the green record (`docs/architecture/testing/verify-freshness-scope.md`,
"The green record and the cut"). A sweep whose every piece passed but that
wrote no green record (a concurrent run rewrote the certificate, the tree
moved) exits 1, because nothing covers a push: its verdict line ends
`every piece passed but no green record was written, so no push is covered`,
then names what the sweep itself reads: the skipped suites under
`ZE_SKIP_SUITES`, or the pieces the parts record holds, and the sweep to run
again. A parts record that already holds every piece with no green record
naming the commit (the green write failed after the last piece was recorded)
runs no piece: the sweep writes the green record from the parts record, and
when that write fails too it prints the reason on the line before its verdict.
`first-red` with `part` or `all` is refused
with exit 2, because a piece counts towards the green record only when every
stage of it runs.
<!-- source: internal/le/verify/sweep.go -- Sweep -->

## A full verification is Thomas's to start

`./le verify worktree` (with any of `commit`, `keep`, `first-red`,
`part <i> of <m>`, `all of <m>`), `./le verify current`, `current fast`, and the
host side of `./le verify evidence release-candidate` are release checks Thomas starts. Each
declares `WholeTree` where it registers (`leaction.Action.WholeTree`), and the
dispatcher refuses the line with exit 2 unless stdin and stdout are both
terminals (`Area.admit`, `internal/le/le/action/leaction.go`, calling
`human.RequireTerminal`, `internal/le/le/human/human.go`). The refusal comes
after the keyword grammar, so a mistyped keyword still gets its usage text, and
before the body, so no admission, job slot, worktree or container exists yet. A
tool call reaches le through pipes; Thomas's shell reaches it with his terminal.

The refusal names which stream is not a terminal, says why (a full run loads
the whole shared machine, and only Thomas knows when that is safe), then gives
two lines: the exact command this invocation was given, to hand to Thomas, and
`./le verify changed`, the scoped run that is the agent's own evidence. A
terminal question that errors is refused the same way, naming the error.

No keyword, flag or environment variable skips the gate.
`TestEveryWholeTreeVerbIsGated` (`internal/le/verify/gate_test.go`) walks the
registered verbs and fails on one that starts a run without the declaration, or
on a gated verb that gains a keyword nobody reviewed. A Go test that drives a
gated verb installs `leaction.SetWholeTreeAdmission`; a functional fixture that
drives one runs it under a pseudo-terminal (`terminalCommand`, or
`terminalCommandStreams` when its assertions read stderr apart from stdout,
`internal/test/fixture/misc_fixture_runner.go`), standing where Thomas stands.
The release-candidate container gets `docker run -t` for the inner
`./le verify current` of the release check Thomas starts
(`docs/architecture/testing/interop.md`). The gate stops a
tool call, not a program that fakes a terminal; the rule that an agent never
runs a full verification covers that case.

## One verify at a time

Parallel verify runs share the build cache, the ports, and the test binaries. An
admitted job runs now, queues behind the jobs already in flight, or attaches to an
equivalent run. The actions that admit themselves today are `./le go lint run`,
`./le job run`, and every harness RUNNER under `le test`: the 24 suites plus
`bgp`, `editor`, `exabgp`, `vpp` and `web`. A runner takes the slot, then runs
itself again as a child inside it, so the slot's log grows with the run's output
(`RunnerAnswer`, `internal/le/test/harnesstool/harnesstool.go`). A harness HELPER
tool (`peer`, `rpki`, a mock, `fixture`) never admits, because the suite that
started it already holds the slot. `./le test stress-repro` holds ONE slot for
its whole run and names it as the parent of its parallel `le test <suite>`
children, so each repetition runs inside that slot rather than attaching to the
first child's verdict (`admitRun`, `internal/le/test/stressrepro/run.go`). With
no checkout, in a container, a runner runs unadmitted. A QEMU guest names a stand-in for the host's slot as its
parent (`guestJobParent`, `internal/le/test/qemu/guestle.go`), because the guest
sees the host's registry through `/workspace` but not the host's processes. The
pretool hook reads a runner as heavy for the lossy-pipe refusal, and a helper as
light, from the same registration (`leroot.Admits`). Anything else is admitted by
typing it after `./le job run label <label> command`. The rest of the heavy
population joins in
`plan/post-release/engineering/spec-native-action-job-admission.md`, so a second `./le verify current
fast` that Thomas starts does NOT block on the first: only its lint stage does.

`ZE_RUN_SLOTS` (`SlotsKey`) sets the slot count. It defaults to one slot per core
share this machine holds (`defaultSlots`, `internal/le/job/job.go`), which is four
on the 32-core development box, because `internal/le/go/toolchain` already caps each
job at a quarter of the cores (`CoresPerJob`, which is the same number it uses for
`GOMAXPROCS` and for the linter's `-j`).

Admission state is one file per running job, `tmp/.ze-jobs/<label>.<pid>.job`. There
is no `tmp/.ze-verify.lock`: nothing takes that flock. The only flock in the
registry is held for the length of one scan (`registryLock`,
`internal/le/job/registry.go`). A job started INSIDE another job's slot runs straight
through instead of queueing behind its own parent, which is how every stage of a
verify run runs.

## Testing one package while you develop it

`./le job run label unit-pkg command <argv...>` is the supported route. It takes
the admission slot, so one heavy job runs while the peers queue, and it tees the
child's merged output to the job log.

Everything after the `command` keyword is the child's argv, and `job` passes it
through unchanged (its kind is `argv`, `KindArgv` in
`internal/le/le/action/kind.go`; `Admission.Run`,
`internal/le/job/job.go`). The DISPATCHER reads the line first. It answers a
trailing `help`, `--help`, `-h` or `-help` with a usage page, and it does not
run the child (`asksForUsage`, `internal/le/le/root/dispatch.go`). Every other
option travels on, so `command echo -html=cover.out` reaches the child.

The command adds no build tags, no `-race`, no
package pattern and no timeout of its own, so write each of them yourself. The
`PKG=` and `RUN=` spellings belong to `./le test fuzz`, which declares them as
argument aliases; `go test` reads `PKG=./x` as an import path and refuses it.

For a direct `go test` child, the job boundary selects `CGO_ENABLED=1` when the
Go build flags enable `-race`, and `CGO_ENABLED=0` otherwise. This overrides the
launcher's build-only `CGO_ENABLED=0`; no `env` wrapper is needed. Effective
`GOFLAGS` is applied first using Go's whole-field quoting, then explicit argv
wins. If argv does not determine race mode and the OS `GOFLAGS` is absent or
empty, the job asks the selected Go executable for `go env GOFLAGS` in the same
directory/environment, preserving any leading `-C`. This includes persistent
`GOENV` configuration and its platform-specific default location without a
second configuration-file parser. A failed lookup stops the command rather
than inventing a CGO mode. Explicit race flags and nonempty OS `GOFLAGS` need
no configuration subprocess.

An explicit `-race=false` disables the detector and repeated race flags
use the last value. Flags after `-args`, or in a positional test-argument tail
after the completed package list, belong to the test binary rather than the
Go build. Values of other Go flags, such as the pattern in `-run -race`, do not
enable instrumentation. The child's argv and `GOFLAGS` are unchanged and Go
still validates them.

The same direct-child boundary removes every spelling of the outer checkout
root and named-build identity, while preserving the parent job setting. Keep
`go test` as the direct command (an optional leading Go `-C` is supported);
`job` does not parse shell commands or `env` wrappers.
<!-- source: internal/le/job/process.go -- commandEnvironment, goTestRace, goFlagsRace -->
<!-- source: internal/le/go/toolchain/gotoolchain.go -- EffectiveGoFlags -->

Carry the feature tags from the recipe at the top of this page, or the run
judges a tree in which no gated plugin registers.

```
tags="ze_core $(awk '$1 ~ /^ze_/ {print $1}' feature-gates.txt | sort -u | tr '\n' ' ')"
./le job run label unit-pkg quiet command go test -race -tags "$tags" ./internal/core/eap
./le job run label unit-pkg quiet command go test -race -tags "$tags" -run TestEAPTLS ./internal/component/ike/...
```

Drop `-race` while iterating if you must. A package tested without `-race` has
not been tested the way the gate tests it, so put it back before the end.

### `quiet`, and the pattern it replaces

`quiet` sits between the label and the `command` keyword. It sends the child's
merged output to `tmp/session/<date>-<id>/scratch/job-<label>.log` instead of
the terminal, and answers three things: the exit code, that log's path, and up
to twenty failure lines from it, each with its line number.

```
job unit-pkg: exit 1, log tmp/session/2026-09-01-abc/scratch/job-unit-pkg.log
412:--- FAIL: TestEAPIdentity (0.00s)
418:FAIL	github.com/ze-software/ze/internal/core/eap	0.310s
```

Which lines those are is one declaration, `internal/le/runlog`, shared with the
verification failure index, so a stage log and a quiet job name the same lines.

Write no redirect of your own, and no `grep` over the result. A session that
composes a scratch path, a redirect, an exit-code echo and a `grep` is writing
this command by hand, and each hand-written copy picks a different set of lines
to keep.

The job log under `tmp/.ze-jobs/` is not that file. It is the breaker's
liveness evidence and a follower's replay source, and `ticket.Release`
(`internal/le/job/registry.go`) removes it when the job ends, so a reader who
arrives after the run finds nothing. The quiet log is the session's own and
stays until the session's scratch directory goes. Two runs of one label in one
session overwrite it.

An ordinary run still streams to the terminal, which is what every wrapped
recipe needs.

## Which native action owns the documentation gate

`./le doc check all` and `./le repo check generated` are separate actions.
`internal/le/doc/wiring.Verify` owns the ordered documentation gate, including the
`internal/le/doc/yangcontract` command and drift checks, the `internal/le/doc/check` links,
and RFC freshness. `internal/le/repo` owns the generated repository artifacts.

## Waiting for another session's job

`tmp/.ze-jobs/<label>.<pid>.job` holds one entry per running job, with its label,
pid and log (`JobsDir`, `internal/le/job/job.go`). It is a registry, not a lock.
`tmp/.ze-verify.lock.owner` (`OwnerFile`) is a copy of ONE entry, so read the
directory when more than one job can run. `./le verify status check` reports the
last verify's verdict.

Raw heavy work that no registered action owns is admitted with
`./le job run label <label> command <argv...>`. One job runs and its peers queue
or attach; the child's exit status is preserved, so the command inside remains the
command being judged. The child runs inside that job's slot: its environment names
the job's entry as `ZE_RUN_JOB`, with every other spelling of `ze.run.job` the
caller carried removed, because `env.Get` prefers a dotted spelling and a child
that read an older parent would queue behind its own (`childEnviron`). A cheap subcommand of a heavy tool needs no admission:
`golangci-lint config verify` runs no analysis. A one-off that must not queue
states its reason in the command, as `ZE_ADMIT_RAW="<reason>"`. An empty reason
admits nothing, and the reason that is there lands in the transcript, which is
what makes the escape auditable by reading the session.
<!-- source: internal/le/job/job.go -- Admission.childEnviron -->

## The Bash guard matches your command text

`internal/le/hookruntime/bash.go` judges the command STRING. It reads a command
POSITION rather than a substring, so a verb at the start of the command or after a
separator is a run, and a verb inside quotes is prose: a search pattern, an echo,
or a commit message explaining the rule. `gitVerbRun` decides it, and the quote
exemption is withdrawn for a command that hands a string to another shell to run
(`bash -c`, `eval`), which is the one place quotes open a command position.

Git's pre-verb options are stripped before the comparison (`gitInvocation`), so
`git -C /other/tree commit` and `git -c commit.gpgsign=false commit` are refused
exactly as the bare verb is.

Two guards read the result. `bashDestructiveGit` refuses the verbs that discard
work or publish it, staging included, and names `./le commit create` as the route
that works. `bashBranchMove` refuses the verbs that create, switch, rename, delete
or integrate a branch, and says the branch is the user's to move; `git branch`
with no mutating flag is a read and passes.

The guard is still coarse where quoting cannot help: an unquoted verb in a
pipeline reads as a run. One extra round-trip costs less than one real bare
staging or commit verb in a shared checkout. Running the scan through the harness
`Grep` tool avoids the question, because the query never enters a command line.

The same guard refuses a write to `plan/` or `ai/rules/` from Bash
(`bashGovernedWrite`), because the Write and Edit tools are where the document
checks in `internal/le/hookruntime/writeedit.go` run. The interpreter tier
covers Perl, Ruby, Python, Node, Deno and Bun payloads. It refuses one that
names a governed tree beside a write call: a write mode passed to `open`, a
write method, `writeFileSync`, or a rename, copy or move. A payload that only
reads passes. The tier over-matches on purpose, so a heredoc that merely NAMES those trees beside a write
primitive is refused too. A wrong refusal is answered with
`ZE_ADMIT_GOVERNED_WRITE="<reason>"`, never by rewording the command.

A path that reaches the writer at run time is followed as a taint
(`internal/le/hookruntime/bash_governed_taint.go`). A variable is tainted when
it is assigned a governed path, when a `for` loop iterates over one, or when a
`while read` loop is fed from a command that lists or searches a governed tree
(`find plan`, `git ls-files ai/rules`, `done < <(find plan ...)`). A write
through a tainted `$f` or `${f}` is refused: an in-place editor, a redirect,
`tee`, or a `cp` or `mv` target. The guard also refuses an in-place editor run
by `xargs` or `find -exec` after a command that names a governed tree, one
handed its files by `$(find plan ...)`, and, after a `cd` or `pushd` into a
governed tree, any in-place editor or a relative redirect, `tee`, copy or move
target. A read into scratch passes, because the scratch variable never held a
governed path: `grep x plan/a.md > "$S/out"` is free. A file list read from a
file that is not itself governed, such as `sed -i x $(cat list.txt)`, is not
seen. Reading stays free: `grep`, `cat`, `sed -n` and `./le commit create file plan/spec-x.md
dry-run` bind on the write, not on the path.

## What a fork and a poll cost

On macOS each `fork+exec` costs about 4 to 5 ms. A loop over 400 files running one
`grep` per iteration spends about 2 seconds on fork overhead before any real work.
A second command per iteration doubles it, and a nested loop makes it quadratic.
One recursive `grep`, one glob, or one `find -exec +` is a single fork.

A poll loop's cost is not the fork. It is the wake and its lifetime: an abandoned
loop keeps taking CPU long after anybody wants its answer, and that contention is
what makes concurrent QEMU, Docker and verification work fail.

| Waiting for | Mechanism |
|-------------|-----------|
| A command this session launched in the background | Nothing. The completion notification is the wake-up |
| A file or log line one of your own commands will produce | ONE bounded loop in the background, wrapped in `timeout`. It notifies once, then it is gone |
| A repeated event (every ERROR line, every CI step) | A monitor with a deadline, never a persistent one |
| Another session's heavy job to free a slot | Do other work, and read `tmp/.ze-jobs/`. Never a watcher |
| Nothing in particular | Do not wait at all |
