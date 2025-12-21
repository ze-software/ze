# Claude Continuation State

**Last Updated:** 2025-12-21

---

## CURRENT PRIORITY

**Config Migration System** - See `plan/config-migration-system.md`

---

## RECENTLY COMPLETED

### Per-Neighbor RIB Config + Peer Globs (Done)

**Plan:** `plan/rib-config-design.md`

**Implemented:**
- Per-neighbor `rib { out { group-updates; auto-commit-delay; max-batch-size; } }`
- Peer glob patterns in config: `peer * { ... }`, `peer 192.168.*.* { ... }`
- API peer glob support: `peer * announce route ...`
- Template inheritance for RIB config
- Legacy `group-updates` backward compatibility

---

## PENDING PLANS

| Plan | Status | Description |
|------|--------|-------------|
| `config-migration-system.md` | Draft | Version detection, migrations, `zebgp config upgrade/fmt` |
| `neighbor-to-peer-rename.md` | Draft | Rename `neighbor` → `peer` in config syntax |

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
