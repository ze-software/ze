# .claude/ Directory Index

## Structure

```
.claude/
├── rules/              # Auto-loaded by file path
│   ├── planning.md     # Pre-implementation planning (*)
│   ├── tdd.md          # TDD rules (**/*.go)
│   ├── go-standards.md # Go coding standards (**/*.go)
│   ├── rfc-compliance.md # RFC rules (internal/bgp/**/*.go)
│   ├── config-design.md # Config design rules
│   └── git-safety.md   # Git protection (*)
├── hooks/              # Automation scripts
│   ├── session-start.sh # Git status check (SessionStart)
│   └── auto_linter.sh   # Lint on file write (PostToolUse)
├── commands/           # Custom skills
│   └── prep.md         # /prep task specification
├── output-styles/      # Communication style
│   └── zebgp.md        # Terse emoji-prefixed
├── zebgp/              # Architecture reference docs
│   ├── wire/           # Wire format docs
│   ├── behavior/       # FSM, signals
│   ├── api/            # API architecture
│   └── config/         # Config syntax
├── backups/            # Work preservation
├── INDEX.md            # Doc navigation
├── ESSENTIAL_PROTOCOLS.md # Reference (slim)
└── settings.json       # Hooks, permissions
```

## Quick Start

1. Rules auto-load based on file path
2. Hooks automate git check and linting
3. Read `INDEX.md` to find architecture docs
4. Run `make lint && make test && make functional` before claiming done

---

**Updated:** 2025-12-31
