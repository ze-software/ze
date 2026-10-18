# Reviewed claims: session fa5bfe15

Each row records a docs claim a reader compared with a change that preserves
behavior. The rules and the check that reads this table are in
`docs/contributing/documentation-testing.md`.

| Doc | Source | Symbol | Commit | Reason |
|-----|--------|--------|--------|--------|
| `docs/contributing/rfc-implementation-guide.md` | `internal/le/rfc/carriers.go` | `carriersFor` | `839819ea2d` | only the Pipeline label strings changed, from the removed `verify current mode full` spelling to `verify current`; the page's carrier table names kinds, runners and tiers, never those labels, so its claims are as true as before |
