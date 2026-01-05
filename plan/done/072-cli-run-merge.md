# Spec: cli-run-merge

## Task
Move the zebgp run command under cli --run

## Required Reading (MUST complete before implementation)

- [x] `.claude/INDEX.md` - doc index (checked for relevant docs)
- [x] `.claude/zebgp/api/ARCHITECTURE.md` - API server internals (low relevance)
- [x] `plan/spec-*.md` - no existing related specs
- [x] `cmd/zebgp/main.go` - current command dispatch
- [x] `cmd/zebgp/cli.go` - bubbletea interactive CLI
- [x] `cmd/zebgp/run.go` - current run command
- [x] `cmd/zebgp/run_test.go` - existing tests

**Key insights:**
- `run.go` defines `cliClient` type used by both `run.go` and `cli.go`
- `cmdCLI()` already uses `cliClient` for socket communication
- `cmdRun()` has `-i` for interactive, stdin piping, and single command modes
- Merge is straightforward: add `--run` flag to `cmdCLI()`, consolidate client code

## Files to Modify
- `cmd/zebgp/cli.go` - add `--run` flag, merge run logic
- `cmd/zebgp/main.go` - remove `run` case, update help
- `cmd/zebgp/run.go` - DELETE (merge into cli.go)
- `cmd/zebgp/run_test.go` - move to cli_test.go or delete

## Current State
- Tests: passing
- Last commit: be51d1f

## Implementation Steps
1. Write test for `cli --run` behavior
2. See test fail
3. Add `--run` flag to `cmdCLI()` in cli.go
4. Move `cliClient` methods from run.go to cli.go
5. See test pass
6. Remove `run` case from main.go
7. Delete run.go
8. Update help text in main.go
9. Run `make test && make lint && make functional`

## Design Decision
- `zebgp cli --run "peer list"` executes single command
- `zebgp cli` launches interactive bubbletea
- `zebgp run` removed entirely (breaking change, user approved)

## Checklist
- [x] Required docs read
- [x] Test fails first
- [x] Test passes after impl
- [x] make test passes
- [x] make lint passes (pre-existing deprecated warnings only)
- [x] make functional passes (test C failure pre-existing)
- [x] Update help text
