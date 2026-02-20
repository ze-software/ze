# Testing Rationale

Why: `.claude/rules/testing.md`

## Why No Throw-Away Tests

Throw-away tests are lost knowledge. Future devs re-investigate same questions. CI doesn't catch regressions.

## Linters in make ze-lint (26 total)

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
import "codeberg.org/thomas-mangin/ze/internal/test/peer"
peer, _ := peer.New(&peer.Config{Port: 1790, Sink: true, Output: &bytes.Buffer{}})
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()
result := peer.Run(ctx)
```

## ExaBGP Compatibility Testing

```bash
ze exabgp plugin /path/to/exabgp-plugin.py
ze-peer --port 1790 ../5.0/qa/encoding/api-announce.msg
```
