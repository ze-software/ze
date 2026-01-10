# ZeBGP - Claude Instructions

## Commands
- `make test` - Run unit tests
- `make lint` - Run golangci-lint
- `make functional` - Run functional tests (80 tests)

## Workflow
0. **UNDERSTAND FIRST** - No coding until you understand the code structure
1. **DESIGN FIRST** - Search for existing code. Extend, don't duplicate. Think deeply.
2. For BGP code: read RFC from `rfc/` folder first
3. Write test, see it FAIL, implement, see it PASS (TDD)
4. Run `make test && make lint && make functional` before claiming done
5. Only commit when explicitly requested

## Key Rules
- **No code without understanding** - BLOCKING: Do not write any code until you have read relevant existing code and understand the architecture. Search, read, analyze BEFORE proposing changes.
- **Design before code** - Search codebase first. Reuse/extend existing code. Think deeply before implementing.
- **TDD MANDATORY** - Test must exist and fail before implementation
- **RFC compliance** - BGP code must follow RFCs, add `// RFC NNNN` comments
- **Verify before claiming** - Paste command output as proof
- **Git safety** - Never commit/push without explicit request

## Reference Paths
- ExaBGP: `/Users/thomas/Code/github.com/exa-networks/exabgp/main/src/exabgp/`
- RFCs: `rfc/` directory
- RFC summaries: `docs/rfc/`

## Architecture Docs
Read when working on specific areas:
- Wire formats: `docs/architecture/wire/messages.md`
- NLRI types: `docs/architecture/wire/nlri.md`
- Attributes: `docs/architecture/wire/attributes.md`
- Capabilities: `docs/architecture/wire/capabilities.md`
- UPDATE building: `docs/architecture/update-building.md` (Build vs Forward paths)
- Memory pools: `docs/architecture/pool-architecture.md`
- Zero-copy: `docs/architecture/encoding-context.md`
- ExaBGP compat: `docs/exabgp/exabgp-code-map.md`
- FSM: `docs/architecture/behavior/fsm.md`
- API: `docs/architecture/api/architecture.md`
- API Capabilities: `docs/architecture/api/capability-contract.md`
- Config: `docs/architecture/config/syntax.md`

## Style
- Terse, emoji-prefixed status lines
- No fluff, no reassurance
- Paste command output as proof
