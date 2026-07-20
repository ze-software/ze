# Impact Analysis

**When:** Before modifying a file, check what else needs to change
**Severity:** blocking

## Directives

Before modifying a file, check what else needs to change.
Changes to certain file types have predictable ripple effects.

## By File Type

### YANG Schema (`*.yang`)

| What changed | Also update |
|---|---|
| New leaf/container | Config parser that reads the tree (grep `GetContainer`, `GetChild` for the path) |
| New leaf/container | Validator if validation rules apply |
| New leaf/container | CLI completion if the command references the schema |
| Renamed path | `scripts/dev/yang_move.py` handles slash paths, set commands, brace blocks, GetContainer chains |
| New `environment/` leaf | `env.MustRegister()` in the component's config loader |
| New `ze:listener` | Conflict detection via `FindListenerConflict` |
| New `ze:command` | RPC handler + `make ze-doc-test` |

### Registration (`register.go`, `init()`)

| What changed | Also update |
|---|---|
| New plugin | `make generate` (updates `all.go`), `TestAllPluginsRegistered` count |
| New family | `family.MustRegister()`, NLRI decoder/encoder registration |
| New capability | Capability codec registration |
| New event type | `Registration.EventTypes` field |
| Renamed name | See `ai/rules/plugin-design.md` "Renaming a Registered Name" for full grep |

### Go Source (`*.go` under `internal/`)

| What changed | Also check |
|---|---|
| New exported symbol | Wiring: who calls it? (`ai/rules/wiring-completeness.md`) |
| Modified function signature | All callers (LSP findReferences or grep) |
| New goroutine | `ai/rules/goroutine-lifecycle.md`, cleanup on shutdown |
| New `make([]byte, N)` on wire path | Pool-backed alternative (`ai/rules/buffer-first.md`) |
| New `fmt.Sprintf` | Append-based alternative (`ai/rules/no-sprintf-alloc.md`) |
| Guard/fallback added | Sibling call-site audit (`ai/rules/before-writing-code.md`) |
| Error return ignored | Hook `block-ignored-errors.sh` will reject |

### Functional Test (`*.ci`)

| What changed | Also check |
|---|---|
| New test file | Correct directory (`ai/rules/testing.md` test directories table) |
| Python observer | No `sys.exit(1)`, use `runtime_fail` (`ai/rules/testing.md` observer section) |
| Config in `tmpfs=` | Parse test validates syntax |

### Go Source → Documentation

When changing code, check `ai/CODE-TO-DOCS.md` for docs that reference the file.
Update any claims that are now wrong. Regenerate: `make ze-doc-index`.

### Documentation (`docs/`)

| What changed | Also check |
|---|---|
| New factual claim | Source anchor: `<!-- source: path -- symbol -->` |
| Feature count/list | `make ze-doc-test` validates against live registry |
| Changed config syntax | `docs/guide/configuration.md` and `docs/architecture/config/syntax.md` |

### Spec (`plan/spec-*.md`)

| What changed | Also check |
|---|---|
| Status change | per-session marker via `scripts/dev/spec-session.sh` |
| AC added/removed | Wiring test table, audit table |
| Design decision | Annotate with `-> Decision:` for post-compaction recovery |

## Quick Grep Patterns

```bash
# Who calls this function?
grep -rn "FunctionName" internal/ cmd/ --include="*.go" | grep -v "_test.go"

# Who reads this YANG path?
grep -rn "path/to/leaf" internal/ --include="*.go"

# Who references this registered name?
grep -rn "plugin-name" internal/ pkg/ cmd/ test/ docs/ plan/ .claude/

# Who imports this package?
grep -rn "codeberg.org/thomas-mangin/ze/internal/component/foo" internal/ cmd/ --include="*.go"
```
