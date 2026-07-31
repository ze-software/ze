# Before Writing Code

**When:** before writing any code, tests, or documentation
**Severity:** blocking

## Directives

Complete before writing any code, tests, or documentation.
Rationale: `ai/rationale/before-writing-code.md`

```
[ ] 1. Read pattern cookbook — touching CLI/web/plugin/config/tests: read `ai/patterns/<domain>.md`. See `ai/INDEX.md` "I Want To..."
[ ] 2. Grep/Glob for existing implementations — extend if found. Hook `check-existing-patterns.sh` blocks `Write` of a new `.go` under `internal/` when the first type name exists elsewhere. Grep `^type Foo ` first
[ ] 3. Know source files — use digests if available; read + write digest if not
[ ] 4. Verify file paths exist (Glob/Grep)
[ ] 5. Wiring-first check — for every new feature, name the user entry point (CLI command, web route, config leaf, plugin event) and the function where it will be registered. If the entry point doesn't exist yet, it is Phase 1. If it does, name the function you will modify. "Library code someone will call" is not an answer.
[ ] 6. Buffer-first check (wire encoding) — `ai/rules/buffer-first.md`
[ ] 7. Lazy-first check — can the consumer use existing wire methods? `design-principles.md` "Lazy over eager"
[ ] 8. Bulk-edit check — >2 files with same pattern? Change ONE, test, confirm, THEN `scripts/dev/replace.py` (preview before `--apply`). Never assume
[ ] 9. Sibling call-site audit — adding a guard/fallback/retry to ONE call site? Grep ALL callers; apply same change in the same commit
[ ] 10. Discovery update check -- adding or changing a feature, tool, self-check, verification gate, or test infrastructure? Name the docs/rules/index updates now. See `ai/rules/discovery-updates.md`
```

Before any spec: READ source, document current behavior, preserve by default.

## Memory Lifecycle Tracing

Before any buffer/pool/allocation, trace the full lifecycle:
where allocated? who holds it? when copied? when released?

Ze lifecycle: allocate at receive (Incoming Peer Pool), share
read-only through forwarding, copy only on egress modification
(Outgoing Peer Pool), release after TCP write.

Acquisition point defines the design: "every dispatch" vs "only on
modification" are fundamentally different. A pool is not a counter.
Look at filter code + `buildModifiedPayload` to see WHERE
modification happens before deciding WHERE buffers come from.

**Red flags:** new file without checking for similar; function that
might duplicate; can't name 3 related files.

## Sibling Call-Site Audit

When you add a guard, fallback, retry, or special case to a call
site of a shared function, grep every other call site in the same
commit and apply the same fix where needed.

Precedent: the blob-store fallback fix (`d029a94d` + `5f66e4f5`) was
added in one call site. Three siblings were missed; five plugin
tests stayed masked for six days.

| Trigger | Action |
|---------|--------|
| `nil` check on a result | Check every other caller for the same nil case |
| Fallback when external system unavailable | Check every other caller of the dependency |
| Retry / backoff | Check every other caller doing the same I/O |
| New error-wrapping context | Check every other caller wrapping the same error |
| Replace direct call with helper | Check every other caller that should use the helper |
| Change/remove how a binary is invoked (launch form, positional, flag) | Grep EVERY invocation site of the bare token (`\bze <arg>`), not just the framework directive (`exec=ze`): `.ci` `exec=`, embedded `tmpfs=*.sh` bodies, helper `.sh`/`.py`, runner launch code, wrapper scripts, docs. Prove with the FULL suite, never a sample -- only it runs the embedded launches (learned 1248) |

```
fn="store.ReadFile"
grep -rn "$fn" internal/ pkg/ cmd/ --include="*.go"
```

For each match: same guard needed? Yes -> fix now. No -> state in
commit message WHY this caller is exempt. Silence is bug bait.

Difference from bulk-edit (item 7): bulk-edit = "same change to N
files I already know about" (discipline). Sibling-audit = "change to
ONE file; which OTHERS need it?" (discovery).
