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
# test/parse/my-feature.ci (valid: expect=exit:code=0)
# test/parse/my-error.ci (invalid: expect=exit:code=1 + expect=stderr:contains=)
# stderr can use "regex=" for pattern matching
```

### Why This Matters

- Throw-away tests are lost knowledge
- Future devs will re-investigate the same questions
- CI doesn't catch regressions if tests don't exist
- Functional tests document expected behavior

### When to Add Tests

| Situation | Action |
|-----------|--------|
| Checking valid config parses | Add `.ci` to `test/parse/` with `expect=exit:code=0` |
| Checking invalid config fails | Add `.ci` to `test/parse/` with `expect=exit:code=1` + `expect=stderr:contains=` |
| Checking BGP message encoding | Add to `test/encode/` |
| Checking API behavior | Add to `test/plugin/` |
| Checking wire format decoding | Add to `test/decode/` |

## Functional Test Location

**BLOCKING:** When you need to create a functional test, save it to the repository.

| Test Type | Location | Format |
|-----------|----------|--------|
| BGP encoding tests | `test/encode/<name>.ci` | Config (embedded or ref) + expectations |
| Plugin tests | `test/plugin/<name>.ci` | Config + expectations |
| Parsing tests | `test/parse/<name>.ci` | Embedded config + exit code + optional stderr |
| Decoding tests | `test/decode/<name>.ci` | stdin + cmd + expect=json |
| Unit tests | `internal/<package>/<file>_test.go` | Go test file |

### Creating Functional Tests

1. **Encoding test:** Create `test/encode/<name>.ci` (config can be embedded or separate `.conf`)
2. **Plugin test:** Create `test/plugin/<name>.ci`
3. **Parse test:** Create `test/parse/<name>.ci` with embedded config, `ze bgp validate`, and exit expectations
4. **Decoding test:** Create `test/decode/<name>.ci` with stdin=, cmd=, expect=json: lines
5. **Run:** `make ze-functional-test` or `ze-test bgp <type> --list` to verify

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

✅ Save to `test/` appropriate subdirectory
✅ Verify with `make ze-functional-test`
✅ Commit with the feature

## Required Test Sequence

```bash
make ze-verify   # Development: ze-lint + ze-unit-test + ze-functional-test
make test-all    # Before commit: ze-lint + ze-test
```

| Target | Command | Purpose |
|--------|---------|---------|
| `make ze-unit-test` | `go test -race` (excludes chaos) | Ze unit tests |
| `make ze-lint` | `golangci-lint run` (excludes chaos) | Ze linting (26 linters, see below) |
| `make vet` | `go vet ./...` | Go vet only (subset of lint) |
| `make ze-functional-test` | All functional tests | Encoding, plugin, parsing, decoding |
| `make ze-exabgp-test` | ExaBGP compat tests | Ze encoding matches ExaBGP |
| `make ze-ci` | ze-lint + ze-unit-test + build | Full CI check |

### Linters in `make ze-lint`

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
make ze-fuzz-test                                      # All fuzz tests (10s per package)
make ze-fuzz-one FUZZ=FuzzParseNLRIs TIME=30s          # Single target, longer
go test -list='Fuzz.*' ./...                           # List fuzz tests
```

Corpus location: `test/fuzz/<FuzzName>/`

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
ze-test bgp encode --list      # List tests
ze-test bgp encode --all       # Run all
ze-test bgp encode 0 1 2       # By index
ze-test bgp encode --count 10 0 # Stress test
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

The `internal/exabgp/` library provides Ze ↔ ExaBGP format translation:

```bash
# Run Go tests for exabgp package
go test -v ./internal/exabgp/...

# Use ze exabgp plugin to run ExaBGP plugins with Ze
ze exabgp plugin /path/to/exabgp-plugin.py
```

### In Ze Config

```
process exabgp-compat {
    run "ze exabgp plugin /path/to/exabgp-plugin.py";
}
```

### Testing with ExaBGP

```bash
# Ze peer against ExaBGP test file
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

- [ ] `make ze-unit-test` passes
- [ ] `make ze-lint` passes with **zero issues** (fix pre-existing issues first)
- [ ] `make ze-functional-test` passes
- [ ] User approval

**BLOCKING:** Never commit with ANY lint issues, even pre-existing ones. Fix lint issues first or ask user for guidance.
