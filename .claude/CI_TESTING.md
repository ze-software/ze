# CI Testing

Run ALL tests before declaring code ready.

---

## Required Test Sequence

```bash
make test  # ALL tests, exits on first failure
```

**This runs:**
1. `golangci-lint run` - Linting
2. `go test -race ./...` - All tests with race detector

---

## Individual Commands (For Debugging)

```bash
# Linting only
make lint

# All tests
go test -race ./...

# Specific package
go test -race ./internal/pool/... -v

# Single test
go test -race ./internal/pool/... -run TestInternDeduplication -v

# With debug build tags
go test -race -tags=debug ./internal/pool/... -v

# Benchmarks
go test -bench=. -benchmem ./internal/pool/...

# Build check
make build
```

---

## Fuzzing (MANDATORY for Wire Format)

**All parsers MUST have fuzz tests.**

```bash
# Run specific fuzz test for 30 seconds
go test -fuzz=FuzzParseHeader -fuzztime=30s ./pkg/bgp/message/...

# Run fuzz test until stopped (Ctrl+C)
go test -fuzz=FuzzParseHeader ./pkg/bgp/message/...

# Run all fuzz tests briefly (CI smoke test)
go test -fuzz=. -fuzztime=10s ./pkg/bgp/...

# List available fuzz tests
go test -list='Fuzz.*' ./...
```

### Fuzz Test Requirements

| Parser | Fuzz Test Required |
|--------|-------------------|
| **Wire Format (Network)** | |
| Message header | FuzzParseHeader |
| OPEN message | FuzzParseOpen |
| UPDATE message | FuzzParseUpdate |
| NOTIFICATION | FuzzParseNotification |
| Attributes | FuzzParseAttribute |
| NLRI (each type) | FuzzParse<Type> |
| Capabilities | FuzzParseCapability |
| **Configuration (User Input)** | |
| Config file tokenizer | FuzzTokenize |
| Config file parser | FuzzParseConfig |
| Neighbor definition | FuzzParseNeighbor |
| Route definition | FuzzParseRoute |
| **API (External Input)** | |
| CLI commands | FuzzParseCommand |
| JSON input | FuzzParseJSON |

### Fuzz Corpus Location

```
testdata/fuzz/
├── FuzzParseHeader/
├── FuzzParseUpdate/
└── FuzzParseNLRI/
```

---

## Pre-Commit Checklist

- [ ] `make test` passes all tests
- [ ] `git status` reviewed
- [ ] User approval

**If ANY unchecked: DO NOT COMMIT**

---

## Test Coverage

```bash
# Coverage for specific package
go test -race -cover ./internal/pool/...

# Coverage with HTML report
go test -race -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Coverage for specific package with details
go test -race -coverprofile=coverage.out -covermode=atomic ./internal/pool/...
go tool cover -func=coverage.out
```

**Coverage thresholds:**
- Critical code (pool, wire parsing): aim for 90%+
- Public functions: 100% coverage required

---

## Race Detector

**ALWAYS run tests with race detector:**

```bash
go test -race ./...
```

The race detector catches:
- Data races
- Concurrent map access
- Improper synchronization

**Any race detected = test failure**

---

## Debugging Test Failures

### Verbose output

```bash
go test -race -v ./pkg/bgp/message/... 2>&1 | tee /tmp/test.log
```

### Run single failing test

```bash
go test -race ./pkg/bgp/message/... -run TestOpenPack -v
```

### Debug with delve

```bash
dlv test ./internal/pool/... -- -test.run TestInternDeduplication
```

---

## Makefile Targets

```makefile
.PHONY: build test lint clean

build:
	go build ./...

test: lint
	go test -race ./...

lint:
	golangci-lint run

clean:
	go clean -testcache
```

---

## CI Workflows

All must pass:
- Linting (golangci-lint)
- Tests with race detector (go test -race)
- Build (go build)

---

## ExaBGP Compatibility Testing

Once wire format is implemented, test against ExaBGP:

```bash
# Start ExaBGP as peer
cd ../main
./sbin/exabgp etc/exabgp/test-config.conf

# Run zebgp against it
cd ../ze
./bin/zebgp --config test.conf
```

**See:** `.claude/zebgp/TEST_INVENTORY.md` for ExaBGP test cases

---

**Updated:** 2025-12-19
