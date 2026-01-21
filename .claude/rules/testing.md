---
paths:
  - "*"
---

# Testing Commands

## No Throw-Away Tests

**BLOCKING:** Never write temporary/throw-away test code to verify behavior.

If you need to check that code behaves correctly:
1. **Add a functional test** - permanent, runs in CI
2. **Add a unit test** - if testing internal logic

### Examples

❌ **Wrong:** Write a quick `main()` to test config parsing
```go
// /tmp/test_config.go - FORBIDDEN
func main() {
    cfg, err := config.Load("test.conf")
    fmt.Println(cfg.Peers[0].Capabilities.RouteRefresh)
}
```

✅ **Right:** Add to functional tests
```
# test/data/parse/valid/my-feature.conf (positive test)
# test/data/parse/invalid/my-error.conf + .expect (negative test)
# .expect can use "regex:" prefix for pattern matching
```

### Why This Matters

- Throw-away tests are lost knowledge
- Future devs will re-investigate the same questions
- CI doesn't catch regressions if tests don't exist
- Functional tests document expected behavior

### When to Add Tests

| Situation | Action |
|-----------|--------|
| Checking valid config parses | Add to `test/data/parse/valid/` |
| Checking invalid config fails | Add to `test/data/parse/invalid/` with `.expect` |
| Checking BGP message encoding | Add to `test/data/encode/` |
| Checking API behavior | Add to `test/data/plugin/` |
| Checking wire format decoding | Add to `test/decode/` |

## Functional Test Location

**BLOCKING:** When you need to create a functional test, save it to the repository.

| Test Type | Location | Format |
|-----------|----------|--------|
| BGP encoding tests | `test/data/encode/<name>.conf` + `<name>.ci` | Config + expectations |
| Plugin tests | `test/data/plugin/<name>.conf` + `<name>.ci` | Config + expectations |
| Parsing tests (valid) | `test/data/parse/valid/<name>.conf` | Config only |
| Parsing tests (invalid) | `test/data/parse/invalid/<name>.conf` + `<name>.expect` | Config + error |
| Decoding tests | `test/decode/<name>.ci` | stdin + cmd + expect=json |
| Unit tests | `internal/<package>/<file>_test.go` | Go test file |

### Creating Functional Tests

1. **Encoding test:** Create `test/data/encode/<name>.conf` and `test/data/encode/<name>.ci`
2. **Plugin test:** Create `test/data/plugin/<name>.conf` and `test/data/plugin/<name>.ci`
3. **Decoding test:** Create `test/decode/<name>.ci` with stdin=, cmd=, expect=json: lines
4. **Run:** `make functional` or `ze-test run <type> --list` to verify

### CI File Format (.ci)

```
# Comments start with #
# Each line is a hex-encoded BGP message expected in sequence
FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF002D010400...
FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304...
```

### NEVER Create Temporary Test Files

❌ `/tmp/test_something.go`
❌ Ad-hoc scripts that aren't committed
❌ Manual testing without saving the test

✅ Save to `test/data/` appropriate subdirectory
✅ Verify with `make functional`
✅ Commit with the feature

## Required Test Sequence

```bash
make test && make lint && make functional  # Full verification
```

| Target | Command | Purpose |
|--------|---------|---------|
| `make test` | `go test -race -v ./...` | Unit tests |
| `make lint` | `golangci-lint run` | Linting (26 linters, see below) |
| `make vet` | `go vet ./...` | Go vet only (subset of lint) |
| `make functional` | All functional tests | Encoding, plugin, parsing, decoding |
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
go test -race ./internal/bgp/message/... -v       # Single package
go test -race ./... -run TestName -v          # Single test
go test -race -cover ./...                    # Coverage
go test -bench=. -benchmem ./internal/...          # Benchmarks
```

## Fuzzing

```bash
go test -fuzz=FuzzParseHeader -fuzztime=30s ./internal/bgp/message/...
go test -fuzz=. -fuzztime=10s ./internal/bgp/...  # All fuzz tests (CI)
go test -list='Fuzz.*' ./...                  # List fuzz tests
```

Corpus location: `test/data/fuzz/<FuzzName>/`

## Functional Testing

### ze-peer (BGP Test Peer)

```bash
ze-peer --sink --port 1790              # Accept any, reply keepalive
ze-peer --echo --port 1790              # Echo messages back
ze-peer --port 1790 qa/encoding/test.msg # Check mode
ze-peer --view qa/encoding/test.msg      # View rules only
```

| Flag | Description |
|------|-------------|
| `--port` | Listen port (default: 179) |
| `--sink` | Accept any, reply keepalive |
| `--echo` | Echo messages back |
| `--ipv6` | Bind IPv6 |
| `--asn` | Override ASN (0 = mirror) |

### ze-test (Test Runner)

```bash
ze-test run encoding --list      # List tests
ze-test run encoding --all       # Run all
ze-test run encoding 0 1 2       # By index
ze-test run encoding --count 10 0 # Stress test
```

### testpeer (Library)

```go
import "codeberg.org/thomas-mangin/ze/internal/test/peer"

peer, err := peer.New(&peer.Config{
    Port: 1790, Sink: true, Output: &bytes.Buffer{},
})
if err != nil {
    log.Fatal(err)
}
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()
result := peer.Run(ctx)
```

## ExaBGP Compatibility

The `internal/exabgp/` library provides ZeBGP ↔ ExaBGP format translation:

```bash
# Run Go tests for exabgp package
go test -v ./internal/exabgp/...

# Use ze exabgp plugin to run ExaBGP plugins with ZeBGP
ze exabgp plugin /path/to/exabgp-plugin.py
```

### In ZeBGP Config

```
process exabgp-compat {
    run "ze exabgp plugin /path/to/exabgp-plugin.py";
}
```

### Testing with ExaBGP

```bash
# ZeBGP peer against ExaBGP test file
ze-peer --port 1790 ../5.0/qa/encoding/api-announce.msg

# ExaBGP against ze-peer
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
