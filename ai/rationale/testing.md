# Testing Rationale

Why: `ai/rules/testing.md`

## Why No Throw-Away Tests

Throw-away tests are lost knowledge. Future devs re-investigate same questions. CI doesn't catch regressions.

## Linters in ./le go lint run (26 total)

Key: `govet`, `staticcheck`, `errcheck`, `gosec`, `gocritic` (hugeParam, rangeValCopy), `prealloc`, `exhaustive`, `dupl`.

Full list: `errcheck`, `govet`, `ineffassign`, `staticcheck`, `unused`, `gocritic`, `gosec`, `misspell`, `unconvert`, `unparam`, `nakedret`, `prealloc`, `noctx`, `bodyclose`, `dupl`, `errorlint`, `exhaustive`, `forcetypeassert`, `goconst`, `godot`, `nilerr`, `nilnil`, `tparallel`, `wastedassign`, `gofmt`, `goimports`

## ze-peer Flags

| Flag | Description |
|------|-------------|
| `--port` | Listen port (default: 179) |
| `--sink` | Accept any, reply keepalive |
| `--echo` | Echo messages back |
| `--ipv6` | Bind IPv6 |
| `--asn` | Override ASN (0 = mirror) |

## testpeer Library

```go
import "github.com/ze-software/ze/internal/test/peer"
peer, _ := peer.New(&peer.Config{Port: 1790, Sink: true, Output: &bytes.Buffer{}})
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()
result := peer.Run(ctx)
```

## ExaBGP Compatibility Testing

```bash
ze exabgp plugin /path/to/exabgp-plugin.py
le test peer --port 1790 ../5.0/qa/encoding/api-announce.msg
```

## History moved from rule points (2026-09-26)

- `ai/rules/points/testing/iteration-workflow-blocking/name-a-real-slog-subsystem-in-a-ze-log-key.md`: ", which is why it has recurred three times"
- `ai/rules/points/testing/mutation-testing/an-applied-discrimination-cut-is-marked-so-it-cannot-reach-a-commit.md`: "Seven notes sit at HEAD in `_test.go` files and are right to be there;"
- `ai/rules/points/testing/mutation-testing/an-applied-discrimination-cut-is-marked-so-it-cannot-reach-a-commit.md`: "On 2026-09-06 a session stopped ten agents at once; one had applied `names = names[:1]` and `targets = targets[:1]` to `internal/plugins/imageserver/register.go` and had not yet watched the test fail. Committed as it stood, the product would have shipped the exact defect its own spec existed to fix, wrapped in passing tests. The resuming agent found it by reading the file, and no gate would have."
- `ai/rules/points/testing/mutation-testing/an-applied-discrimination-cut-is-marked-so-it-cannot-reach-a-commit.md`: "A search for the marker MUST cover `_test.go` files too: the first search after that stop excluded them and reported the tree clean."
- `ai/rules/points/testing/rfc-tagged-tests-blocking/reindex-after-moving-a-tagged-test.md`: "Until 2026-09-11 both outputs were owed in the same commit, and a session that regenerated from a shared checkout carried other sessions' tags into its own message."
- `ai/rules/points/testing/temporary-files/use-project-tmp-for-scratch-files.md`: "A fixed name at the `tmp/` root is the failure this replaces: it names the same file for every session in the checkout."
- `ai/rules/points/testing/the-affected-population-is-not-the-edited-population/the-tests-you-write-for-a-change-are-green-by-construction.md`: "Measured on 2026-08-22: `clear_debt` (`internal/le/commit`) changed the argument its `GateRunner` receives from the repo root to the throwaway worktree. Four new tests were green and six existing `TestDebtClear` cases were red, two of them a genuine semantic break. Measured again on 2026-08-23, when `show bgp rib` moved to flat rows: `internal/component/lg` was never edited, and its `extractRoutes` captured the new shape and returned rows unnormalized, so the looking-glass graph answered `No routes found`, which reads as a true answer about an empty RIB."
