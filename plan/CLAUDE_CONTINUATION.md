# Claude Continuation State

**Last Updated:** 2025-12-21

---

## CURRENT PRIORITY

**Neighbor→Peer Rename - Phase 2+ (requires migration system)**

✅ Phase 1 complete: v3 syntax (`template.group`, `template.match`, `peer <IP>`)

Next steps:
- `plan/config-migration-system.md` - Migration infrastructure (v1→v2→v3)
- `plan/neighbor-to-peer-rename.md` - Phase 2: migration, Phase 3: refactor

---

## PENDING PLANS

| Plan | Status | Description |
|------|--------|-------------|
| `neighbor-to-peer-rename.md` | **Phase 1 ✅** | Phase 2+ pending: migration, refactor, deprecate |
| `config-migration-system.md` | Draft | v1→v2→v3 migrations, `zebgp config upgrade/fmt` |
| `api-commit-batching.md` | Planning | Commit-based route batching, `commit start/end` API |
| `config-routes-eor.md` | Planning | EOR after config routes, implicit commit |

---

## REFERENCE DOCS

| Doc | Purpose |
|-----|---------|
| `exabgp-alignment.md` | Review decisions (26 ALIGN, 8 KEEP, 2 SKIP) |
| `ARCHITECTURE.md` | Codebase architecture overview |

---

## TEST STATUS

✅ **All unit tests pass** (`make test`)
✅ **Lint clean** (`make lint` - 0 issues)

---

## KEY FILES

| Purpose | File |
|---------|------|
| Config parser | `pkg/config/bgp.go` |
| Peer glob matching | `pkg/config/bgp.go` (IPGlobMatch) |
| API commands | `pkg/api/command.go` |
| Reactor peer matching | `pkg/reactor/reactor.go` (ipGlobMatch) |
| This file | `plan/CLAUDE_CONTINUATION.md` |
| Protocols | `.claude/ESSENTIAL_PROTOCOLS.md` |

---

## NOTES

- All code changes require TDD (test first, show failure, implement, show pass)
- Plans go in `plan/`, protocols go in `.claude/`
- Check ExaBGP reference before implementing BGP features
- **ALWAYS run `make test && make lint` before requesting a commit**
