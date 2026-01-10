# Spec: async-api-parser

## Status: PLACEHOLDER

## Task

Implement non-blocking API parsing with goroutine-based dispatcher/gatherer pattern.

**Problem:** Current API parsing is synchronous - blocks reactor while parsing routes.

**Solution:** Parser goroutine with parallel workers that preserve ordering.

```
┌─────────┐     ┌────────────────────────────────────────────────┐
│   API   │────▶│              Parser Goroutine                  │
│ Handler │     │  ┌──────────┐                                  │
└─────────┘     │  │Dispatcher│──┬──▶ Parser 1 ──┐               │
                │  └──────────┘  ├──▶ Parser 2 ──┼──▶ Gatherer ──┼──▶ Engine
                │                └──▶ Parser N ──┘   (ordered)   │
                └────────────────────────────────────────────────┘
```

## Required Reading

- [ ] `.claude/zebgp/api/ARCHITECTURE.md` - current API design
- [ ] `.claude/zebgp/api/CAPABILITY_CONTRACT.md` - API contracts
- [ ] `docs/plan/spec-api-command-serial.md` - command sequencing
- [ ] `docs/plan/spec-parser-unification.md` - parser consolidation

**Key insights:**
- TBD - read docs first

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates |
|------|------|-----------|
| `TestDispatcherSequencing` | `pkg/engine/api/dispatcher_test.go` | Sequence numbers assigned correctly |
| `TestGathererOrdering` | `pkg/engine/api/gatherer_test.go` | Results reordered by sequence |
| `TestWorkerPoolParallel` | `pkg/engine/api/worker_test.go` | Multiple parsers run concurrently |
| `TestBackpressure` | `pkg/engine/api/pipeline_test.go` | Slow consumer doesn't lose data |

### Functional Tests
| Test | Location | Scenario |
|------|----------|----------|
| `test-api-async` | `qa/tests/api-async/` | Bulk API commands maintain order |

## Files to Modify

- `pkg/engine/api/dispatcher.go` - new: dispatch logic
- `pkg/engine/api/gatherer.go` - new: ordering logic
- `pkg/engine/api/worker.go` - new: parser worker pool
- `pkg/engine/reactor.go` - integration point
- `pkg/plugin/parse.go` - may need interface changes

## Implementation Steps

1. **Write tests** - Create test files with failing tests
2. **Run tests** - Verify FAIL (paste output)
3. **Implement dispatcher** - Sequencing, distribution
4. **Implement worker pool** - Parallel parsers
5. **Implement gatherer** - Reordering, delivery
6. **Integration** - Wire into reactor
7. **Run tests** - Verify PASS (paste output)
8. **Verify all** - `make lint && make test && make functional`

## Design Decisions

- [ ] Number of parser workers (config? auto-scale?)
- [ ] Buffer sizes for channels
- [ ] Error handling (drop? retry? report?)
- [ ] Backpressure when engine is slow
- [ ] Graceful shutdown

## Channel Types (Draft)

```go
type ParseRequest struct {
    SeqNum  uint64
    Command string
}

type ParseResult struct {
    SeqNum uint64
    Update *Update
    Err    error
}

parseIn  chan ParseRequest   // dispatcher → workers
parseOut chan ParseResult    // workers → gatherer
toEngine chan *Update        // gatherer → engine
```

## RFC Documentation

- N/A - internal architecture, not protocol code

## Checklist

### 🧪 TDD
- [ ] Tests written
- [ ] Tests FAIL (output below)
- [ ] Implementation complete
- [ ] Tests PASS (output below)

### Verification
- [ ] `make lint` passes
- [ ] `make test` passes
- [ ] `make functional` passes

### Documentation
- [ ] Required docs read
- [ ] `.claude/zebgp/api/ARCHITECTURE.md` updated

### Completion
- [ ] Spec moved to `docs/plan/done/NNN-async-api-parser.md`
