# Ze Go Style: simplicity and debt

The detail behind "Simplicity and debt" in
[the Go style core](../ze-go-style.md), which states the rule.

Only the plugin API contract freezes, and only after the first release.

Existing modules in `go.mod` may be used in full. The experimental circl
cryptography module remains restricted to the package/importer pairs Thomas
approved. A new module lands in a dependency-only commit before its first
use. The commit gate reads HEAD's module declarations and the exact approval
ledger. It reads the carried module identity and vendor inventory, so an
uncarried working-tree edit cannot hide an added third-party import; see
[dependency approval](../dependency-approval.md). Dependencies are
vendored, so a build never reaches the network.
<!-- source: internal/le/commit/dependencies.go -- dependencyRefusal -->

Repository tooling is Go under `internal/le/`: an `./le <area> <action>` backed
by a callable package, with fixtures under its `testdata/` or
`internal/test/fixture`. Python stays only where an external Python program is
the subject.
<!-- source: internal/le/register.go -- native tooling composition root -->
