# Verification Freshness and Scope

Verification is a statement about one commit and the files the run read. The native verifier records that statement as a certificate and a per-path manifest, then commit preparation asks whether the prospective commit still matches it.

## Certificate

`internal/le/verifyengine.WriteCertificate` writes `tmp/ze-verify.status` and its manifest atomically after a native verification run. The certificate records the mode, commit, time, result, and the tree hash captured for the run. The manifest records each path at the content the stages read.

`./le verify-status check` calls `internal/le/verifyengine.CheckCertificate`. With no paths it compares the whole checkout. Repeated `path <path>` selectors restrict the answer to a prospective commit's files. A changed file, a moved `HEAD`, a missing manifest, or a path that moved while the run was in progress returns STALE.

<!-- source: internal/le/verifyengine/status.go -- WriteCertificate, CheckCertificate -->
<!-- source: internal/le/verifystatus/answer.go -- Answer -->

## One change-set selection

`./le changed packages` and `./le changed group-packages` derive the package scope from the current change set. Non-Go inputs seed the packages that consume them, and every unresolved case widens to `./...`. An empty answer is never used as a successful narrow selection.

The verify runner resolves the selection once and publishes its package and feature-tag answers to the run's artifact directory. Every scoped stage reads those files. This keeps the unit pass and the staticcheck matrix on the same snapshot, and it avoids a second reverse-import walk after another session changes the checkout.

`internal/le/staticcheckfeaturematrix.Answer` retains the all-features and core-only rows, plus the feature-omission rows the selected tags can affect. A negated build constraint counts as a use of the tag because that file compiles in the omission row.

<!-- source: internal/le/changed/actions.go -- Answer -->
<!-- source: internal/le/staticcheckfeaturematrix/actions.go -- Answer -->
<!-- source: internal/le/verifyengine/run.go -- Run, RunMode -->

## Native stage execution

`internal/le/verifyengine.RunMode` executes the ordered stage population and captures each stage result. A red stage does not hide later reds; cancellation stops before another stage starts. Each in-process gate returns a populated `GateResult`, so an omitted registration cannot look like exit zero.

Native `./le` actions are the public interface, while Go-to-Go paths call their package functions. Heavy verification enters through `./le job run label <label> command <argv...>` or the `verify-lock` action rather than starting another copy behind the admission registry.

<!-- source: internal/le/verifyengine/run.go -- GateResult, RunMode -->
<!-- source: internal/le/job/answer.go -- Answer -->
<!-- source: internal/le/verifylock/register.go -- Answer -->

## Failure attribution

A gate knows which files caused its failure. `internal/le/docwiring` publishes that fact as a JSON failure group at the point where the failure is decided. A group carries a kind, a summary, a rerun command, and zero or more related paths. Paths are JSON-encoded, so a crafted filename cannot forge a second group.

The commit package reads those groups for the prospective commit's explicit paths. A path-bearing group is foreign only when every related path lies outside that commit. A population-wide group, a malformed group, or a failure with no path remains charged. The default is therefore fail closed.

Verification debt records an authorised commit that lacked fresh evidence. It follows the commit until the owed gates pass, and open debt blocks a push. `./le commit debt-list` and `./le commit debt-clear` use the same native ledger as commit preparation.

<!-- source: internal/le/docwiring/groups.go -- Group -->
<!-- source: internal/le/commit/actions.go -- Answer -->
<!-- source: internal/le/commit/debt.go -- Debt, ListDebt -->

## Producer contract

A new verification gate returns structured data from its owning `internal/le` package. If the gate can attribute a failure, it emits the group at the failure point; collecting groups at the end loses failures from early returns. The rerun field uses an exact `./le <area> <action>` invocation.

A new scoped input kind joins the `changed` package's native path table and names the packages that read it. A rule that cannot prove a narrow package set widens. It must never select nothing.
