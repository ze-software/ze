---
paths:
  - "*"
---

# Testing Commands

## Required Test Sequence

```bash
make test && make lint && make functional  # Full verification
```

| Target | Command | Purpose |
|--------|---------|---------|
| `make test` | `go test -race -v ./...` | Unit tests |
| `make lint` | `golangci-lint run` | Linting (26 linters, see below) |
| `make vet` | `go vet ./...` | Go vet only (subset of lint) |
| `make functional` | Run qa/tests/* | Functional tests (37) |
| `make ci` | lint + test + build | Full CI check |

### Linters in `make lint`

`golangci-lint` runs 26 linters including `govet`. Key linters:

| Linter | Checks |
|--------|--------|
| `govet` | Suspicious constructs (printf args, struct tags, etc.) |
| `staticcheck` | Static analysis, bugs, simplifications |
| `errcheck` | Unchecked error returns |
| `gosec` | Security issues |
| `gocritic` | Performance (`hugeParam`, `rangeValCopy`), style, diagnostics |
| `prealloc` | Slice preallocation opportunities |
| `exhaustive` | Missing switch cases |
| `dupl` | Duplicate code blocks |

Full list: `errcheck`, `govet`, `ineffassign`, `staticcheck`, `unused`, `gocritic`, `gosec`, `misspell`, `unconvert`, `unparam`, `nakedret`, `prealloc`, `noctx`, `bodyclose`, `dupl`, `errorlint`, `exhaustive`, `forcetypeassert`, `goconst`, `godot`, `nilerr`, `nilnil`, `tparallel`, `wastedassign`, `gofmt`, `goimports`

## Individual Commands

```bash
go test -race ./pkg/bgp/message/... -v       # Single package
go test -race ./... -run TestName -v          # Single test
go test -race -cover ./...                    # Coverage
go test -bench=. -benchmem ./pkg/...          # Benchmarks
```

## Fuzzing

```bash
go test -fuzz=FuzzParseHeader -fuzztime=30s ./pkg/bgp/message/...
go test -fuzz=. -fuzztime=10s ./pkg/bgp/...  # All fuzz tests (CI)
go test -list='Fuzz.*' ./...                  # List fuzz tests
```

Corpus location: `test/data/fuzz/<FuzzName>/`

## Functional Testing

### zebgp-peer (BGP Test Peer)

```bash
zebgp-peer --sink --port 1790              # Accept any, reply keepalive
zebgp-peer --echo --port 1790              # Echo messages back
zebgp-peer --port 1790 qa/encoding/test.msg # Check mode
zebgp-peer --view qa/encoding/test.msg      # View rules only
```

| Flag | Description |
|------|-------------|
| `--port` | Listen port (default: 179) |
| `--sink` | Accept any, reply keepalive |
| `--echo` | Echo messages back |
| `--ipv6` | Bind IPv6 |
| `--asn` | Override ASN (0 = mirror) |

### functional (Test Runner)

```bash
go run ./test/cmd/functional encoding --list      # List tests
go run ./test/cmd/functional encoding --all       # Run all
go run ./test/cmd/functional encoding 0 1 2       # By index
go run ./test/cmd/functional encoding --count 10 0 # Stress test
```

### testpeer (Library)

```go
import "codeberg.org/thomas-mangin/zebgp/pkg/testpeer"

peer := testpeer.New(&testpeer.Config{
    Port: 1790, Sink: true, Output: &bytes.Buffer{},
})
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()
result := peer.Run(ctx)
```

## ExaBGP Compatibility

```bash
# ZeBGP peer against ExaBGP test file
zebgp-peer --port 1790 ../5.0/qa/encoding/api-announce.msg

# ExaBGP against zebgp-peer
cd ../5.0
env exabgp_tcp_port=1790 ./sbin/exabgp etc/exabgp/api-announce.conf
```

## Coverage

```bash
go test -race -coverprofile=coverage.out ./...
go tool cover -html=coverage.out              # HTML report
go tool cover -func=coverage.out              # Summary
```

| Code Type | Target |
|-----------|--------|
| Wire format | 90%+ |
| Public functions | 100% |

## Pre-Commit Checklist

- [ ] `make test` passes
- [ ] `make lint` passes with **zero issues** (fix pre-existing issues first)
- [ ] `make functional` passes
- [ ] User approval

**BLOCKING:** Never commit with ANY lint issues, even pre-existing ones. Fix lint issues first or ask user for guidance.
