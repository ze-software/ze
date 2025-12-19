# Error Recovery Protocol

What to do when mistakes happen.

---

## Step 1: STOP

Do not try to fix immediately. Assess first.

---

## Step 2: Preserve Current State

```bash
# Save all changes
git diff > .claude/backups/error-$(date +%Y%m%d-%H%M%S).patch
git diff --staged >> .claude/backups/error-$(date +%Y%m%d-%H%M%S).patch
```

---

## Step 3: Identify the Problem

### Code Not Compiling

```bash
go build ./...
# Read error message carefully
```

### Tests Failing

```bash
go test ./... -v 2>&1 | head -50
# Identify which test, what assertion
```

### Wrong Approach Taken

Document what was tried and why it failed before changing course.

---

## Step 4: Ask User

Present options:

```
Tests failing after change to X.

Options:
(a) Continue debugging - I see the issue is Y
(b) Revert to last working state
(c) Save current work and try different approach

Which?
```

**Wait for response before destructive actions.**

---

## Common Recovery Scenarios

### Scenario: Tests Were Passing, Now Failing

```bash
# 1. Save current state
git diff > .claude/backups/failing-$(date +%Y%m%d-%H%M%S).patch

# 2. Find what changed
git diff HEAD~1

# 3. Ask user before reverting
```

### Scenario: Wrong File Edited

```bash
# 1. Save the change
git diff path/to/file > .claude/backups/wrong-file.patch

# 2. Restore original
git checkout -- path/to/file

# 3. Apply to correct file (manually adapt)
```

### Scenario: Commit Made to Wrong Branch

```bash
# 1. Note the commit SHA
git log --oneline -3

# 2. Ask user before fixing
# Options: cherry-pick to correct branch, reset, etc.
```

### Scenario: Build Broken

```bash
# 1. Identify the error
go build ./... 2>&1

# 2. Check recent changes
git diff HEAD~1

# 3. Fix incrementally, test after each fix
```

---

## Recovery Tools

### Git Reflog

Find lost commits:

```bash
git reflog
# Shows all recent HEAD positions
git checkout <sha>  # to inspect
git cherry-pick <sha>  # to recover
```

### Saved Patches

```bash
# List backups
ls -la .claude/backups/

# Apply a patch
git apply .claude/backups/<filename>.patch

# Or view it first
cat .claude/backups/<filename>.patch
```

### Git Stash

```bash
# View stashes
git stash list

# Apply most recent
git stash pop

# Apply specific stash
git stash apply stash@{n}
```

---

## Prevention Checklist

Before making significant changes:

- [ ] Tests pass currently (`make test`)
- [ ] Git state is clean (`git status`)
- [ ] Understanding of what needs to change is clear

After each significant change:

- [ ] Code compiles (`go build ./...`)
- [ ] Related tests pass
- [ ] Consider saving checkpoint if complex

---

## When to Ask for Help

Ask user when:

- Multiple approaches have failed
- Unsure which direction to take
- Error message is unclear
- Change scope is expanding significantly

Format:

```
Hit a blocker: <description>

Tried:
1. <approach 1> - failed because <reason>
2. <approach 2> - failed because <reason>

Options I see:
(a) <option>
(b) <option>

Recommendation: <your recommendation>

Which approach?
```

---

## Anti-Patterns

**Don't:**
- Make more changes to "fix" without understanding the problem
- Delete or revert without saving first
- Claim success after recovery without re-running all tests
- Hide that a mistake was made

**Do:**
- Admit the mistake clearly
- Save state before attempting recovery
- Test thoroughly after recovery
- Document what went wrong for future reference

---

**Updated:** 2025-12-19
