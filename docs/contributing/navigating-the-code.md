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
2. The `gopls` command line, from any context that has a shell. `./le setup`
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
| Which docs cover this code? | `grep` the basename under its package heading in `ai/CODE-TO-DOCS.md` | every doc citing it. Rows are keyed by BASENAME, so the package heading is what stops a bare `grep peer.go` returning three packages |
| Which `.go` files implement this design doc? | `grep` the doc path in `ai/DOCS-TO-CODE.md` | every file whose `// Design:` header cites that doc, one line each |
| What does this package do? | `grep` the package path in `ai/PACKAGE-MAP.md` | one line, derived from the package doc comment |
| How does this subsystem flow, entry to exit? | `ai/digests/<subsystem>.md` | the flow with `file:line`, the load-bearing files, and the invariants |

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
