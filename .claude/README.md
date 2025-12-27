# .claude/ Directory Index

## Core Protocol (Read Every Session)

- **ESSENTIAL_PROTOCOLS.md** - All protocols consolidated (~985 lines)
  - Session start/end checklists
  - TDD, verification, git safety, error recovery, refactoring
  - RFC compliance, ExaBGP reference, coding standards summary

## Reference Files

| File | Purpose |
|------|---------|
| CODING_STANDARDS.md | Go style guide (detailed) |
| TDD_ENFORCEMENT.md | TDD workflow (detailed) |
| TESTING_PROTOCOL.md | Test commands reference |
| CI_TESTING.md | CI commands reference |
| RFC_DOCUMENTATION_PROTOCOL.md | Wire format documentation |
| DOCUMENTATION_PLACEMENT_GUIDE.md | Where to put docs |
| FILE_NAMING_CONVENTIONS.md | Naming patterns |

## Subdirectories

| Directory | Contents |
|-----------|----------|
| commands/ | Custom skills (`/prep`, etc.) |
| hooks/ | Auto-linter hook |
| output-styles/ | Communication style (zebgp.md) |
| backups/ | Work preservation (patches) |
| zebgp/ | Codebase reference docs |

## zebgp/ Reference Docs

| Subdirectory | Topics |
|--------------|--------|
| api/ | API commands, JSON format |
| behavior/ | FSM, signals |
| config/ | Configuration syntax, environment |
| edge-cases/ | ADD-PATH, AS4, extended messages |
| wire/ | Message formats, attributes, NLRI, capabilities |

## Quick Start

1. Read `ESSENTIAL_PROTOCOLS.md` at session start
2. Use `/prep <task>` for non-trivial tasks
3. Run `make test && make lint` before claiming done

---

**Updated:** 2025-12-27
