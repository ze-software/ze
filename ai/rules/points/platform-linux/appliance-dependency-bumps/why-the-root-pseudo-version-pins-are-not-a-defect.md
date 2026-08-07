---
kind: note
level:
stage:
---
Separate from the builddir concern: five **root** `go.mod` direct dependencies are
pinned to pseudo-versions (`v0.0.0-<date>-<hash>`) rather than semver tags. This is
**not a defect**. It was verified (2026-07-21, `spec-fixit-supply-chain-hardening`
AC-4) that **none of these upstreams publish any semver tag**: `go list -m -versions`
and `proxy.golang.org/<mod>/@v/list` return an empty version list for every one, and
`@latest` resolves to a pseudo-version. There is nothing to move the pin to.

The list was six until 2026-08-07. `github.com/charmbracelet/ssh` left it because
upstream MOVED the module rather than tagging it: the same code now publishes as
`charm.land/ssh`, which carries semver, and the root pin is `charm.land/ssh v0.4.2`.
A module that disappears from this table has either been tagged or been moved. Find
out which before you re-add a row.
