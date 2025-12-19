# Mandatory Refactoring Protocol

**When to read:** Before renaming/restructuring functions, types, or packages
**Prerequisites:** ESSENTIAL_PROTOCOLS.md (verification, testing requirements)
**Size:** 3 KB

---

## Quick Summary

- ONE function/type at a time (never batch refactoring)
- ALL tests MUST pass at EVERY step (100% pass rate always)
- Paste proof after each step before proceeding
- Commit only when passing (one function = one commit)

**See enforcement checklist below for step-by-step workflow.**

---

## PHASE 0: PLANNING

Write numbered steps. Each MUST have:
```
Step N: [Action] [What] in [Where]
  Files: [exact paths]
  Verification: [exact command]
  Expected: "PASS"
```

**Vague:** "Rename methods"
**Specific:** "Rename Pool.Get() to Pool.Lookup() in internal/pool/pool.go"

**Plan Requirements:**
- [ ] Every step numbered
- [ ] Exact file paths
- [ ] Verification command
- [ ] Expected output
- [ ] Final step: full test suite
- [ ] No vague language

**Get user approval. DO NOT proceed without approval.**

---

## PHASE 1-N: EXECUTION

```
=== STEP N ===
[Make changes]
Verification: [run command]
OUTPUT:
[PASTE EXACT OUTPUT - NO SUMMARY]
Result: PASS
=== STEP N COMPLETE ===
```

**Rules:**
- Announce step
- Complete ONLY that step
- Run verification
- PASTE EXACT OUTPUT
- Stop if failures

**NEVER:**
- Skip verification
- Batch steps
- Summarize output
- Proceed with failures

---

## PHASE FINAL: PRE-COMMIT

**Before ANY commit:**

```bash
make test
```
**PASTE OUTPUT - all tests passed**

**Checklist:**
- [ ] `make test` passed (proof pasted)
- [ ] `git status` reviewed
- [ ] User approval

**If ANY unchecked: DO NOT COMMIT**

---

## ONE FUNCTION AT A TIME

**MANDATORY: ONE function/type per step. No batching.**

**Why:**
- Immediate feedback
- Easy debugging
- Surgical rollback
- Always working

**Good:** "Step 1: Pool.Get() → Pool.Lookup()" "Step 2: Handle.Valid() → Handle.IsValid()"
**Bad:** "Step 1: Rename all pool methods"

**No exceptions.**

### All Tests Always Pass

**100% pass rate at every step.**

If tests fail:
1. STOP
2. ANALYZE why THIS change failed
3. FIX it
4. RETEST full suite
5. PROCEED only when all pass

---

## GIT STRATEGY

**Commit messages:**
```
refactor: rename Pool.Get to Pool.Lookup
```

**When to commit:**
- Function renamed + all call sites + ALL tests pass + linting passes
**NEVER:**
- Tests failing, partial work, "will fix next"

**One function = one commit.**

---

## Go-Specific Considerations

### Interface Changes

When refactoring interfaces:
1. Update interface definition
2. Update ALL implementations (compiler will find them)
3. Update ALL call sites
4. Run tests

### Package Renames

When renaming packages:
1. Update go.mod if needed
2. Update ALL import paths
3. Run `go mod tidy`
4. Run tests

### Type Renames

When renaming types:
1. Use IDE refactoring if available (gopls)
2. Otherwise: search with `grep -rn "TypeName" .`
3. Update all references
4. Run tests

---

## ENFORCEMENT

For EACH step before proceeding to next:
```
=== STEP N ===
Verification: <command>
OUTPUT:
<FULL OUTPUT PASTED - NO SUMMARY>
Result: PASS
```
- [ ] Output pasted (not summarized)
- [ ] ALL tests passed (not "most tests")

**If ANY unchecked: STOP. Fix current step.**

Before ANY commit:
- [ ] `make test` run
- [ ] Output pasted showing all tests passed
- [ ] User approval obtained

**If ANY unchecked: DO NOT COMMIT.**

---

## VIOLATION DETECTION

**If I do these, I'm violating:**
- Batch multiple steps together
- Summarize test output instead of pasting
- Proceed with ANY test failures
- Skip verification command
- Commit without full test suite passing

**Auto-fix:** Stop. Run verification. Paste output. Wait for pass before next step.

---

## See Also

- TESTING_PROTOCOL.md - Test requirements at each step
- GIT_VERIFICATION_PROTOCOL.md - Git workflow during refactoring
- RFC_DOCUMENTATION_PROTOCOL.md - Document wire formats before changing protocol code

---

**Updated:** 2025-12-19
