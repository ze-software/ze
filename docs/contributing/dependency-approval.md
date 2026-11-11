# Approving a third-party dependency

Modules already declared in `go.mod` may be used in full, including a new
package or importing directory. The exception is the experimental
`github.com/cloudflare/circl` cryptography module: approving
`github.com/cloudflare/circl/sign/ed448` for `internal/core/ed448` approves
nothing else of circl, and no other directory. Each new circl use goes to
Thomas first (`ai/META-DECISIONS.md`, "Code and dependencies").

A new module still lands in its own dependency commit: `go.mod`, `go.sum` and
its `vendor/` tree, before the commit that first uses it. An uncommitted
`go.mod` addition does not authorize a new importing commit.

## The ledger

`go.mod` declares the modules available for full use. `dependency-ledger.txt`,
beside it, records exact package/importer approvals where use-specific approval
is required. Each row holds three tab-separated fields:

| Field | Meaning |
|-------|---------|
| package | The import path of one third-party package, never a whole module |
| importer | The checkout-relative directory whose Go files may import it, `.` for the root |
| reason | Why Ze needs it there, one line |

A line starting with `#` is a comment. A malformed or repeated row is an error
naming its line, because a row the reader skipped would withdraw an approval in
silence. `go.mod` and `vendor/modules.txt` say which modules are present.
Existing ordinary-module rows remain in the ledger, but a new package or
importer of those modules needs no new row.

The first ledger was seeded from the tree: every third-party import in use that
day carries the reason `in use when the ledger was seeded`.

<!-- source: internal/le/deps/deps.go -- Parse, Row, LedgerRel -->
<!-- source: internal/le/deps/policy.go -- ParsePolicy, packageProviders -->

## What counts as a third-party import

An import path is third party when its first element holds a dot, which no
standard-library path does, and it is not under the module path `go.mod`
declares. Imports are read with the Go parser in imports-only mode, so a file
built under one feature tag is judged like any other: it still links what it
imports.

The importer is the file's directory. A `_test.go` file is judged like any
other file of its directory: a test-only dependency is still vendored, still
supply chain, and still Thomas's to approve.

Not judged:

| Path | Why |
|------|-----|
| `vendor/` | The dependencies themselves |
| `testdata/` | The go command never builds it |
| A directory holding its own `go.mod` (`examples/plugin/go`, the gokrazy build directories) | Another module, whose imports Ze does not link |

<!-- source: internal/le/deps/deps.go -- ThirdParty, Imports, Judged, JudgedWith, Importer -->

## The commit gate

`./le commit create` refuses a commit that:

- carries a Go file adding a third-party import that neither an existing
  ordinary module nor an exact ledger approval permits;
- adds a ledger row that `tmp/dependency-approvals.txt` does not hold, the
  record only `./le deps approve` writes;
- removes the ledger.

It judges only what the commit adds. The existing-module allowance reads
`go.mod` from HEAD, so a working-tree module addition, including one carried
with the first use, does not qualify. Circl always needs an exact ledger row.
Reuse is bound to the exact package provider in the prospective
`vendor/modules.txt`, so a new nested module does not inherit its parent's
allowance. The root module identity, vendor inventory and nested-module
boundaries come from the carried content, or HEAD for a file not carried;
removed boundaries are absent. An uncarried `go.mod` cannot hide an import.
An import the file already held at HEAD, or one it drops, asks nothing. The
ledger is read as the commit leaves it: the carried copy when the commit
carries the ledger, HEAD's otherwise, so a use-specific approval cannot come
from an uncarried row. A commit that creates the ledger (HEAD
holds none) is the seed and owes no record; since removing the ledger is
refused, that happens once.

The refusal names the file, the package, the vendored module that provides it
(or says no vendored module does), the importer, and the command Thomas runs.
It answers exit 1. A file the gate cannot read answers exit 3.

<!-- source: internal/le/commit/dependencies.go -- dependencyRefusal, carriedLedger, addedImports -->
<!-- source: internal/le/commit/prepare.go -- Create -->

## Approving a use

Where use-specific approval is required, only Thomas approves, from his own
terminal:

```
./le deps approve package github.com/cloudflare/circl/sign/ed448 importer internal/core/ed448 reason "Ed448 signatures for IKEv2 AUTH"
```

The command refuses unless stdin and stdout are both interactive terminals,
before it reads or writes anything, and the refusal prints the command line for
Thomas (`human.RequireAuthorization`, the gate `commit push` and owner
attestations share). It refuses the standard library and Ze's own packages,
which need no approval. Admitted, it writes the row into the ledger, sorted,
replacing the reason of an existing row for the same package and importer, and
appends the same row to `tmp/dependency-approvals.txt`. The agent then carries
`dependency-ledger.txt` in the commit that adds the import.

An agent never writes either file by hand. The gate stops a tool call, not a
determined wrapper: like every owner-only action, a program that fakes a
terminal or writes the record passes it.

Removing a row needs no approval: it narrows what is allowed.

<!-- source: internal/le/deps/approve.go -- admit, validApproval, Approve, Recorded -->
<!-- source: internal/le/deps/actions.go -- approveAnswer -->

## Checking the whole tree

`./le deps check` parses every Go file git tracks or would track and refuses,
exit 1, any third-party import that neither a module declared in the current
`go.mod` nor an exact ledger approval permits. Circl remains use-specific.
The ordinary-module allowance uses the current vendored package inventory;
an absent inventory grants no module-wide reuse.
Rows no file uses any more are listed as stale, without failing. It runs in the
full verification (`fullStages`).

<!-- source: internal/le/deps/check.go -- Check, goFiles -->
<!-- source: internal/le/verify/engine/stages.go -- fullStages -->
