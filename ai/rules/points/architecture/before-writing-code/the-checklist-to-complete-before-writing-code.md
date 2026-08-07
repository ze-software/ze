---
kind: directive
level:
stage:
---
- [ ] 1. Read pattern cookbook (touching CLI/web/plugin/config/tests): read `ai/patterns/<domain>.md`. See `ai/INDEX.md` "I Want To..."
- [ ] 2. Grep/Glob for existing implementations, extend if found. Hook `check-existing-patterns.sh` blocks `Write` of a new `.go` under `internal/` when the first type name exists elsewhere. Grep `^type Foo ` first
- [ ] 3. Know source files: use digests if available; read + write digest if not
- [ ] 4. Verify file paths exist (Glob/Grep)
- [ ] 5. Wiring-first check: for every new feature, name the user entry point (CLI command, web route, config leaf, plugin event) and the function where it will be registered. If the entry point doesn't exist yet, it is Phase 1. If it does, name the function you will modify. "Library code someone will call" is not an answer.
- [ ] 6. Buffer-first check (wire encoding): `ai/rules/performance.md`
- [ ] 7. Lazy-first check: can the consumer use existing wire methods? "Design Principles" below, "Lazy over eager"
- [ ] 8. Bulk-edit check: >2 files with same pattern? Change ONE, test, confirm, THEN `scripts/dev/replace.py` (preview before `--apply`). Never assume
- [ ] 9. Sibling call-site audit: adding a guard/fallback/retry to ONE call site? Grep ALL callers; apply same change in the same commit
- [ ] 10. Discovery update check: adding or changing a feature, tool, self-check, verification gate, or test infrastructure? Name the docs/rules/index updates now. See `ai/rules/repo-maintenance.md`
