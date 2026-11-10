# The error inventory

`./le arch error-inventory` lists the places in the code where a message
reaches a user, and says for each one whether it meets
`ai/rules/user-facing-errors.md`: the message names what went wrong, why, and
the exact command that fixes it.

`report` lists every site from the three sources below. `check` and the
commit gate refuse only a plain message a change adds at one of this
package's own writers ("The check and the commit gate").

| Action | What it answers |
|--------|-----------------|
| `report` | Every message site in the tree, one row each, then one count line per surface: `surface=<name> sites=<n> typed=<n> plain-names-command=<n> plain=<n>` |
| `check` | Refuses a change that adds a plain message at a surface writer no other gate judges |

A row carries the surface, the file and line, the owner package, the door
area, the status and the pre-rank. The status is one of three:

| Status | Meaning |
|--------|---------|
| `typed` | The writer receives the operator error type with a fix |
| `plain-names-command` | A plain message whose literal text names a command: `ze ` or `./le ` at a word start followed by a lowercase word, or a config statement (`set `, `delete `) at the start of the text or right after a quote or backtick. It does not meet the rule, because no tool can prove the command fixes the problem |
| `plain` | Any other message |

The pre-rank orders the worklist: 1 for an operator surface with error
severity or a refusal, 2 for an operator surface with warning severity, 3 for
`./le` tooling. The report sorts its rows by pre-rank, then surface, file and
line, and prints each message Go-quoted, so a newline in it stays on its row.
`| json` answers the same rows through the shared renderer.
<!-- source: internal/le/arch/errorinventory/inventory.go -- Row, Status, PreRank -->
<!-- source: internal/le/arch/errorinventory/actions.go -- actions -->
<!-- source: internal/le/arch/errorinventory/report.go -- treeReport, Text -->

## Where the rows come from

The inventory scans nothing another gate already scans. It reads three site
lists and joins them:

| Source | Read through | Surfaces it gives |
|--------|--------------|-------------------|
| operator-error | `archoperatorerror.Sites` | `daemon-command` for a command or RPC handler's refusal, `config-verify` for a config verifier's |
| doctor-coverage | `archdoctorcoverage.Scan` | `log` for a Warn or Error log line, a report raise or a health row; `doctor` for a `diagnostic.Diagnostic` finding |
| writer | this package's writer table (below) | `local-cli`, `mcp`, `rest`, `web`, `gnmi`, `grpc`, `le` |

Two sources can name one site: a registered handler that returns a gRPC
status error is a handler refusal and a writer call on the same line. The
merge keys every row by file and line and keeps one row per pair, from the
first source in the order of the table above. The writer's row of that line
is dropped, so the site is counted once and judged by the gate that owns it.

Each source's own verdict maps onto the three statuses:

| Source verdict | Status |
|----------------|--------|
| operator-error `typed` | `typed` |
| operator-error `plain` or `pass-through` | judged by the returned expression's text: `plain-names-command` when it names a command, `plain` otherwise. A pass-through hands on an error the site did not build, so nothing proves it carries a fix |
| doctor-coverage finding with its `Fix` set | `typed`: the Diagnostic record is the doctor surface's operator error type, and its `Fix` clears the finding |
| any other doctor-coverage site | judged by its message's text. A log line that names a declared doctor code is covered for doctor's purposes: `ze doctor` reports the problem, but the line itself names no fix the code declares. A `"fix"` log attribute is not in the site list, so it is not read |

A handler refusal ranks as a refusal. A log line or report raise ranks by
the level its call names. A health row and a doctor finding carry no level
in the site list, and both are a problem the operator is shown as one, so
they rank as errors, never below the warnings.

Every row carries its owner package, the directory of its file, and the
door pages whose `door-base` covers the file, read by
`hookruntime.DoorsCovering`, the same lookup doctor-coverage uses.

A source that answers fewer sites than the checkout is known to hold is
refused with exit 2, never reported as a cleaner tree: a scan that matched no
registration, logger or writer shape would otherwise answer zero plain
messages. The floors are in `checkoutFloors`, and the action tests replace
them for their fixture trees.
<!-- source: internal/le/arch/errorinventory/inventory.go -- Scan -->
<!-- source: internal/le/arch/errorinventory/merge.go -- mergeRows, operatorErrorStatus, doctorCoverageStatus, doctorCoveragePlace, checkFloor -->
<!-- source: internal/le/arch/errorinventory/actions.go -- checkoutFloors -->

## The surface writers

A surface writer is a call or literal that hands a message to a user on a
surface neither `./le arch operator-error` (registered handler returns and
error answers) nor `./le arch doctor-coverage` (log lines, report raises,
health rows, doctor findings) lists: the local `ze` program's refusals and
stderr writes, MCP error results, REST error bodies, web refusals, gNMI and
gRPC status errors, and `./le` refusals (`leaction.ReportError`, the
`human.Refusal` type) and report findings. A refusal an `./le` action hands
to `Area.Refuse` is not read: the method is called on an imported type the
syntax cannot name. They are
declared once, in the `writers` table of
`internal/le/arch/errorinventory/writers.go`, each with its surface and the
arguments that carry the message. A new writer helper is one row there.

The writers take a string or a plain error, so a site is `typed` when its
message argument was built from the operator error type with a fix: an
`opererr.New` or `opererr.Wrap` call whose fix is not the empty literal, a
name bound to one, a parameter or variable declared `*opererr.Error`, or a
function of the tree that answers only operator errors. Its text reaches the
writer through `.Error()`, a format, a concatenation or a `textbuf` chain.
`.Problem()`, `.Why()` and `.Fix()` each drop the rest of the error, so a
message built from one of them is plain. An operator error held in a
variable of type `error` cannot be seen from the syntax, and reads as plain.
<!-- source: internal/le/arch/errorinventory/writers.go -- writers -->
<!-- source: internal/le/arch/errorinventory/classify.go -- classify, carries, namesCommand -->
<!-- source: internal/le/arch/errorinventory/inventory.go -- preRankOf -->

### Adding a writer

A new helper that hands a message to a user (an answer function on a new
surface, a refusal type in `./le`) is one row in `writers`, and nothing
else in the package changes. The row says:

| Field | What it holds |
|-------|---------------|
| `surface` | the surface the message reaches; a new surface is a new `Surface` value and its count line |
| `shape` | how the writer is called: a function (`shapeCall`), a method on a named receiver (`shapeMethod`, with `receiver`), a print to `os.Stderr` (`shapeStderrPrint`, `shapeStderrMethod`), a composite literal of a type (`shapeLiteral`) |
| `path`, `ident` | the import path and name the call or type resolves to; an unexported helper is matched in its own package |
| `message` | the span of arguments or literal elements that carry the message: from `first` up to but excluding `last`, `last` 0 meaning through the end |
| `within` | optional: limit the row to these directories, placed before a broader row of the same writer |

The row for `./le` action refusals reads `{surface: SurfaceTooling, shape:
shapeCall, path: leactionPath, ident: "ReportError", message: span{0, 1}}`.
Add a case for the new writer to `TestErrorInventoryFindsEachWriter`
(`classify_test.go`) in the same change, so the row is proved to match.

## The check and the commit gate

`./le arch error-inventory check` judges the lines your unpushed commits and
working tree changed, against the same base the other line-scoped `arch`
checks read. `./le commit create` runs the same judgement over the commit it
prepares, because the package registers it into the change-set seam
(`changecheck.Register`, listed in `docs/contributing/committing.md`). The
commit form scans the tree twice, with the carried Go files as the commit
leaves them and as its parent holds them, and refuses a row the commit adds:
a file, writer and message the parent holds fewer of. Moving an old plain
line is not an addition; a new or reworded one is. A commit that carries no
shipped Go file is admitted without a scan.

It refuses a row only when all of these hold:

| Condition | Why |
|-----------|-----|
| The row comes from this package's writer table | A handler refusal is `./le arch operator-error check`'s to judge, and a log line, report raise, health row or doctor finding is `./le arch doctor-coverage check`'s |
| No other source names the same file and line | The merge gives such a line to the gate that owns it, so a registered handler returning a plain gRPC status error is refused by operator-error, never twice |
| Its status is `plain` or `plain-names-command` | Only `typed` meets the rule |
| The change adds it | A site already in the tree is the worklist's, listed by `report`, never refused |

The refusal names each site's file and line, surface, status, writer and
message, then why, then the fix: hand the writer an operator error built
with `opererr.New(problem, why, fix)`, rendered with `.Error()` where the
writer takes a string, or for a `plugin.Response` error answer, built with
`plugin.OperatorErrorResponse`.

The commit form reads the other two gates' sites over the side the commit
leaves (`archoperatorerror.SitesOnSide`, `archdoctorcoverage.ScanSide`), and
only when its own writer scan found a row the commit adds. A commit adding no
plain writer row costs two parses of the tree, the writer scan of each side;
one adding one costs two more, one for each other source, and no source is
scanned twice.
<!-- source: internal/le/arch/errorinventory/report.go -- judgeChange, addedRows, unowned, refused, CheckReport.Text -->
<!-- source: internal/le/arch/errorinventory/inventory.go -- ownedRows -->
<!-- source: internal/le/arch/errorinventory/register.go -- init -->
<!-- source: internal/le/arch/errorinventory/actions.go -- runCheck -->
