# CI Testing

Run ALL tests before declaring code ready.

**Current test status:** See `plan/CLAUDE_CONTINUATION.md`.

---

## Required Test Sequence

```bash
make test && make lint  # Tests + linting - BOTH required
```

**Individual targets:**
- `make test` = `go test -race -v ./...` (tests only)
- `make lint` = `golangci-lint run` (linting only)
- `make ci` = lint + test + build (full CI check)
- `make test-all` = unit tests + functional tests (self-check)

**IMPORTANT:** `make test` alone does NOT run lint. Always run BOTH.

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
test/data/fuzz/
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
.PHONY: all build test lint clean test-all self-check ci

all: lint test build        # Default: full check

build:                      # Build binaries
	go build -o bin/zebgp ./cmd/zebgp
	go build -o bin/zebgp-cli ./cmd/zebgp-cli
	go build -o bin/zebgp-decode ./cmd/zebgp-decode

test:                       # Unit tests with race detector
	go test -race -v ./...

lint:                       # Linting
	golangci-lint run

test-all: test self-check   # Unit + functional tests

self-check:                 # Functional tests (ExaBGP compat)
	go run ./test/cmd/self-check --all

ci: lint test build         # Full CI check

clean:
	rm -rf bin/ coverage.out coverage.html
```

---

## CI Workflows

All must pass:
- Linting (golangci-lint)
- Tests with race detector (go test -race)
- Build (go build)

---

## Functional Testing Tools

### zebgp-peer (BGP Test Peer)

A Go port of ExaBGP's `qa/sbin/bgp`. Acts as a BGP peer for testing.

**CLI Usage:**

```bash
# Sink mode - accept any messages, reply keepalive
zebgp-peer --sink --port 1790

# Echo mode - echo messages back
zebgp-peer --echo --port 1790

# Check mode - validate against expected messages
zebgp-peer --port 1790 qa/encoding/test.msg

# View expected rules without running
zebgp-peer --view qa/encoding/test.msg
```

**Flags:**

| Flag | Description |
|------|-------------|
| `--port` | Port to bind (default: 179 or $exabgp_tcp_port) |
| `--sink` | Accept any messages, reply keepalive |
| `--echo` | Echo messages back |
| `--ipv6` | Bind to IPv6 |
| `--asn` | Override ASN (0 = mirror peer) |
| `--view` | Show expected rules and exit |

### testpeer Package (Library)

Use testpeer as a library for integration tests:

```go
import "github.com/exa-networks/zebgp/pkg/testpeer"

// Create test peer
peer := testpeer.New(&testpeer.Config{
    Port:   1790,
    Sink:   true,
    Output: &bytes.Buffer{}, // Silence output
})

// Run in goroutine
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

result := peer.Run(ctx)
if !result.Success {
    t.Fatalf("peer error: %v", result.Error)
}
```

**Config options:**

| Field | Type | Description |
|-------|------|-------------|
| `Port` | int | Listen port |
| `Sink` | bool | Sink mode |
| `Echo` | bool | Echo mode |
| `Expect` | []string | Expected message patterns |
| `IPv6` | bool | Bind IPv6 |
| `ASN` | int | Override ASN |
| `SendUnknownCapability` | bool | Add unknown capability to OPEN |
| `SendDefaultRoute` | bool | Send default route after OPEN |
| `InspectOpenMessage` | bool | Check OPEN against expectations |
| `SendUnknownMessage` | bool | Send invalid message type |
| `Output` | io.Writer | Log output (default: os.Stdout) |

### self-check (Test Runner)

Runs functional tests from `qa/encoding/` directory.

```bash
# List available tests
self-check --list

# Run all tests
self-check --all

# Run specific tests by nick
self-check 0 1 2

# Custom timeout
self-check --timeout 60s --all
```

**Test file structure:**

```
qa/encoding/
├── test-name.ci     # Config file reference
└── test-name.msg    # Expected BGP messages
```

**.ci file format:**
```
config-file.conf
```

**.msg file format:**
```
1:raw:FFFFFFFF...:0017:02:00000000
1:raw:FFFFFFFF...:0030:02:...
```

**Options in .msg files:**
```
option:bind:ipv6
option:open:send-unknown-capability
option:open:inspect-open-message
option:update:send-default-route
option:open:send-unknown-message
option:asn:65001
```

---

## ExaBGP Compatibility Testing

Test ZeBGP against ExaBGP using the same test format:

```bash
# Start zebgp-peer with ExaBGP test file
zebgp-peer --port 1790 ../5.0/qa/encoding/api-announce.msg

# Run ExaBGP against it
cd ../5.0
env exabgp_tcp_port=1790 ./sbin/exabgp etc/exabgp/api-announce.conf
```

**Or use zebgp as client against ExaBGP's bgp tool:**

```bash
# Start ExaBGP's bgp tool
cd ../5.0
env exabgp_tcp_port=1790 python3 qa/sbin/bgp --port 1790 qa/encoding/test.msg

# Run zebgp against it
cd ../ze
env exabgp_tcp_port=1790 zebgp run qa/configs/test.conf
```

**See:** `.claude/zebgp/TEST_INVENTORY.md` for ExaBGP test cases

---

**Updated:** 2025-12-19
