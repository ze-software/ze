# Spec: AS Path Length Validation

## Task
Implement AS path length validation

## Embedded Protocol Requirements

### Default Rules (ALL tasks)
- Tests MUST exist and FAIL before implementation code exists
- Run `make test && make lint` before claiming done
- NEVER discard uncommitted work without explicit user permission
- Verify before claiming: run commands, paste output as proof

### From TDD_ENFORCEMENT.md
- Every test MUST document VALIDATES and PREVENTS
- If test passes before implementation: TEST IS WRONG
- Write ONLY enough code to make the test pass
- Show test failure output BEFORE writing implementation

### From CODING_STANDARDS.md
- NEVER ignore errors (`f, _ := ...` is WRONG)
- Use `fmt.Errorf` with `%w` for error wrapping
- `golangci-lint` must pass with zero issues
- Go 1.21+ required

### From zebgp/wire/ATTRIBUTES.md
- AS_PATH is type code 2, flags 0x40 (Well-known Mandatory)
- Attribute header: flags byte, type byte, length (1 or 2 bytes)
- Extended Length flag (0x10) means 2-byte length field

## Codebase Context
- Check existing AS_PATH parsing: `pkg/bgp/attribute/aspath.go`
- Check existing validation patterns: `pkg/bgp/attribute/`
- Check ExaBGP reference: `../main/src/exabgp/bgp/message/update/attribute/aspath.py`

## Implementation Steps
1. Write test for AS path length validation (with VALIDATES/PREVENTS)
2. Run test → verify it FAILS
3. Implement validation logic
4. Run test → verify it PASSES
5. Run `make test && make lint`

## Verification Checklist
- [ ] Tests written and shown to FAIL first
- [ ] Implementation makes tests pass
- [ ] `make test` passes
- [ ] `make lint` passes
- [ ] AS path length edge cases covered (0, max, overflow)
