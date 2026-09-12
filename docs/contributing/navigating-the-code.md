# Navigating the Code

How to answer a question about this repository without reading a whole file.
Two capabilities cover almost every question: a symbol server for what the code
IS, and the generated indexes for what it is FOR. Neither replaces the other.

The rule that says WHEN you owe these routes is `ai/rules/context-economy.md`.
This page says how each one works.

## Resolving a Go symbol

There are two routes to one server, and they give the same answers.

1. The agent LSP tool, when the harness serves it. Load it with
   `ToolSearch query="select:LSP"`.
2. The `gopls` command line, from any context that has a shell. `./le setup install`
   installs `gopls` and puts it on PATH (`requiredTools`,
   `internal/le/setup/tools.go`).

An empty `ToolSearch` result means this context has no LSP tool. It does not
mean the capability is absent: run `gopls` instead.

### Which operation answers which question

| Question | LSP tool operation | `gopls` command | What comes back |
|----------|--------------------|-----------------|-----------------|
| What is in this file? | `documentSymbol` | `gopls symbols <file>` | every symbol with its line range: the map you would otherwise read the whole file to build |
| What does this one symbol declare or say? | `goToDefinition`, then `hover` | `gopls definition <file>:<line>:<col>` | the declaration and its doc comment, without the file around it |
| Who calls this? | `findReferences` | `gopls references <file>:<line>:<col>` | every call site as file plus line. `grep` on a common name returns the comments and the string literals too |
| Who calls this, and from inside WHICH function? | `callHierarchy` | `gopls call_hierarchy <file>:<line>:<col>` | each caller's range AND the enclosing function that `references` leaves you to work out |
| Where does a name I can spell actually live? | `workspaceSymbol` | `gopls workspace_symbol <name>` | the file holding it, without guessing a directory |
| Does this file compile, and with what errors? | (diagnostics) | `gopls check <file>` | the type errors for that file. Silence and exit 0 mean clean |

### The two-step recipe

`gopls symbols` prints one line per symbol as `Name Kind <line>:<col>-<line>:<col>`.
That `<line>:<col>` is exactly what `definition`, `references` and
`call_hierarchy` take. Find the symbol in step one, ask about it in step two.
Never guess a position.

```
$ gopls symbols internal/component/bgp/config/resolve.go
ResolveBGPTree Function 43:6-43:20
$ gopls definition internal/component/bgp/config/resolve.go:43:6
.../resolve.go:43:6-43:20: defined here as func ResolveBGPTree(tree *config.Tree) (map[string]any, error)
ResolveBGPTree resolves peer-group inheritance and returns the bgp block as map[string]any.
$ gopls references internal/component/bgp/config/resolve.go:43:6
.../loader_create.go:274:28-42
.../peers.go:53:18-32
```

Positions are 1-based, and the column is the start of the identifier rather
than the start of the line. A `func` declaration puts its name at column 6.

### What it costs

A `symbols` map of a large file is an order of magnitude smaller than the file.
Measure it for any file rather than trusting a frozen number, because both
sides grow:

```
gopls symbols <file> | wc -c ; wc -c < <file>
```

Every invocation starts a fresh server and loads the workspace, so budget
seconds rather than milliseconds. A 60-second timeout is generous. Several
independent `gopls` questions belong in one message, as with any independent
calls.

### Why `gopls mcp` is not registered

Headless `gopls mcp` watches every directory under the workspace root and holds
one open file descriptor per file, because fsnotify uses kqueue on macOS. It
honors no directory filter: `skipDir` in
`golang.org/x/tools/gopls/internal/filewatcher/fsnotify_watcher.go` skips only
names starting with `.` or `_`, and `testdata`. Use the LSP tool or the command
line above.

## Which index answers which question

The symbol routes answer what code IS. These answer what it is FOR. Grep an
index; do not read one. `ai/CODE-TO-DOCS.md` and `ai/DOCS-TO-CODE.md` are each
several hundred kilobytes, so reading either whole costs more than the source
it was meant to save.

| Question | Where the answer is | What comes back |
|----------|---------------------|-----------------|
| What is this file for, and which doc governs it? | the file's own `// Design:` header | the design doc, plus the sibling files that own each detail |
| Which docs cover this code? | `grep` the path in `ai/CODE-TO-DOCS.md` | every doc citing it. Both render shapes carry the full path from the checkout root, so `grep internal/component/bgp/reactor/peer.go` answers that file alone |
| Which `.go` files implement this design doc? | `grep` the doc path in `ai/DOCS-TO-CODE.md` | every file whose `// Design:` header cites that doc, one line each |
| What does this package do? | `grep` the package path in `ai/PACKAGE-MAP.md` | one line, taken from the package doc comment |
| How does this subsystem flow, entry to exit? | `ai/digests/<subsystem>.md` | the flow with `file:line`, the load-bearing files, and the invariants |

<!-- source: internal/le/docstocode/codetodocs_report.go -- renderCodeIndex -->

The three indexes are DERIVED, and git tracks none of them. A `Write` or an
`Edit` to a file that feeds one REMOVES it. A Bash command that names its path
REBUILDS it before that command runs. So a grep reads the tree as it now stands,
never the tree as it stood at the last commit.

A rebuild that fails blocks the command, because a grep of a file nobody built
answers "no match" for a tree the author cannot see.
<!-- source: internal/le/hookruntime/postwrite.go -- postInvalidateDerived -->
<!-- source: internal/le/hookruntime/bash.go -- preMaterializeDerived -->

**Which searches rebuild, and which do not.** The read hook rebuilds when the
command text holds the index's own path, or a directory that contains it with
its trailing slash. `grep -n X ai/PACKAGE-MAP.md` and `grep -rn X ai/` both
rebuild. A search that names neither does not: `rg ResolveBGPTree`,
`grep -rn foo .`, `grep -rn X ai` without the slash, and any `./le` action that
reads an index from inside its own process. After an edit removed the index, one
of those reads a tree without it and answers no match.
<!-- source: internal/le/hookruntime/bash.go -- commandNamesArtifact -->

Name the path when the answer matters. `grep -n X ai/PACKAGE-MAP.md` is the form
that always reads a current index.

**Which writes invalidate, and which do not.** The removal is keyed to the
`Write` and `Edit` TOOLS. A write that reaches the file some other way moves an
input with no hook in the path: `sed -i`, a shell heredoc, `git rebase`, `git
stash pop`, `git checkout`, and `./le repository generate`. Each of those leaves
the index PRESENT and stale, and neither the read hook nor a session start
rebuilds one that is present, so it stays stale until the next `Write` or `Edit`
to one of its inputs. Run the writer yourself after a bulk rewrite:
`./le discovery-index update`, `./le docs-to-code update`,
`./le docs-to-code index-update`.
<!-- source: internal/le/hookruntime/runtime.go -- nativeHookActions -->
<!-- source: internal/le/hookruntime/lifecycle.go -- hookSessionStart -->

Only Bash is intercepted. The `Read` and the `Grep` TOOL reach no hook. One of
them opens an index the last edit removed, and reports a missing file.

Ask the question from Bash, or write the index yourself. The writers are
`./le discovery-index update` for `ai/PACKAGE-MAP.md`, and
`./le docs-to-code update` and `./le docs-to-code index-update` for the other
two.
<!-- source: internal/le/hookruntime/lifecycle.go -- hookSessionStart -->

Every non-test `.go` file carries its own answer in a `// Design: <doc> -- topic`
header. The scan stops after 25 lines (`HeaderLines`, `internal/le/docstocode/docstocode.go`),
so the header block is always in the first 25 lines, and `designLine` in the
same file is what parses it.

A digest orients; it never proves. `ai/digests/*.md` are hand-maintained
(`ai/digests/README.md`), so open the files a digest names before you state what
code does (`ai/rules/evidence.md`).

## Measuring what a session spends

`./le token-economy` reads this machine's Claude Code transcript store and
prints per-session and per-agent-type context costs. The store grows with every
session, so re-run it for current ratios instead of copying an absolute figure
into a rule or a document. `./le token-economy session <id-prefix>` prints the
per-agent-type table for one session, which is the only comparison that holds:
across sessions the always-on preamble changes size and swamps the difference.

Token counts there are characters divided by 3.6, the approximation
`internal/le/tokeneconomy/tokeneconomy.go` uses.
