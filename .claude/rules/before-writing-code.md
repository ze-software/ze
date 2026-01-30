# Before Writing Code

**BLOCKING:** Complete these checks BEFORE writing any code, tests, or documentation.

## 1. Check for Existing Implementation

Before writing anything new:

```bash
# Search for similar tests
grep -r "TestYourFeature\|your_pattern" internal/ test/

# Search for similar functional tests
grep -r "your_pattern" test/

# Search for similar functionality
grep -r "FunctionName\|pattern" internal/
```

**Ask yourself:**
- Does a test for this already exist?
- Does code doing this already exist?
- Can I extend existing code instead of writing new?

**If you find existing code:** STOP. Use it, extend it, or document why new code is needed.

## 2. Check for Duplication

Before writing new code:

```bash
# Find similar patterns
grep -r "similar_function\|similar_pattern" internal/

# Find similar test patterns
grep -r "similar_test" internal/ test/
```

**Ask yourself:**
- Will this duplicate existing functionality?
- Is there a shared utility I should use?
- Should this be a refactor instead of new code?

**Red flags:**
- Writing a new function that does what an existing one does
- Creating a new test file when a test case could be added to an existing file
- Adding a new functional test that tests the same flow as an existing one

## 3. Document New Understanding

Before ending work, if you learned something new about the codebase:

**Update relevant docs:**

| What you learned | Where to document |
|------------------|-------------------|
| Wire format behavior | `docs/architecture/wire/` |
| API behavior | `docs/architecture/api/` |
| FSM/session behavior | `docs/architecture/behavior/` |
| Test patterns | `docs/functional-tests.md` |
| RFC interpretation | `rfc/short/` |

**Format:**
```markdown
## [Topic]

[Brief explanation of the behavior or pattern discovered]

### Example
[Code or wire bytes demonstrating the behavior]
```

## Checklist

Before writing ANY code:

```
[ ] Searched for existing tests covering this functionality
[ ] Searched for existing code doing similar things
[ ] Verified no duplication with functional tests
[ ] Identified if extension is better than new code
```

Before ending work:

```
[ ] New understanding documented in appropriate docs/
[ ] No orphaned or duplicate test files created
```

## Why This Matters

- **Duplicate tests** waste CI time and create maintenance burden
- **Duplicate code** leads to divergent behavior and bugs
- **Undocumented understanding** is lost knowledge

## Examples of Violations

❌ Created `malformed-notification.ci` when `unknown-message.ci` already tests the same flow

❌ Wrote new `ParseFoo()` when `ParseBar()` with minor changes would work

❌ Learned that EOR is sent automatically but didn't document it

## Correct Approach

✅ Search first: `grep -r "send-unknown-message" test/`

✅ Found existing: "unknown-message.ci already tests this"

✅ Decision: "No new test needed" or "Extend existing test"
