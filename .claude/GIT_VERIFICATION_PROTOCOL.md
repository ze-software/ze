# Git Verification Protocol

---

## Core Rule

**NEVER commit or push without EXPLICIT user request.**

User must say one of:
- "commit"
- "make a commit"
- "git commit"
- "push"
- "git push"

**Completing work is NOT permission to commit.**

---

## Before ANY Git Operation

```bash
git status && git log --oneline -5
```

Check for:
- Unexpected modified files
- Staged changes you didn't make
- Current branch is correct

---

## Commit Workflow

### 1. User Explicitly Requests Commit

Wait for: "commit this", "make a commit", etc.

### 2. Check State

```bash
git status
git diff --staged
```

### 3. Add Files

```bash
git add <specific-files>
# Or if user says "commit all":
git add -A
```

### 4. Create Commit

```bash
git commit -m "$(cat <<'EOF'
<type>: <description>

<body if needed>

Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude <noreply@anthropic.com>
EOF
)"
```

**Commit types:**
- `feat`: New feature
- `fix`: Bug fix
- `refactor`: Code change that neither fixes bug nor adds feature
- `test`: Adding or updating tests
- `docs`: Documentation only
- `chore`: Build process, tooling, etc.

### 5. Verify

```bash
git log --oneline -3
git status
```

---

## Push Workflow

### 1. User Explicitly Requests Push

Wait for: "push", "push it", "git push"

### 2. Check State

```bash
git status
git log origin/main..HEAD --oneline
```

### 3. Push

```bash
git push origin <branch>
```

### 4. Verify

```bash
git status
```

---

## Forbidden Without Permission

### MUST Ask Before:

- `git reset` (any form: --soft, --mixed, --hard)
- `git revert`
- `git checkout -- <file>`
- `git restore` (to discard changes)
- `git stash drop`
- `git push --force`
- Deleting branches

### How to Ask:

```
I need to run `git reset --hard HEAD~1`. This will discard the last commit.
May I proceed? (yes/no)
```

---

## Work Preservation

### Before Any Destructive Operation

Save current work:

```bash
git diff > .claude/backups/work-$(date +%Y%m%d-%H%M%S).patch
```

### Recovery

```bash
# Apply saved patch
git apply .claude/backups/<filename>.patch

# Or check reflog
git reflog
git checkout <sha>
```

---

## Branch Operations

### Creating Branch

```bash
git checkout -b <branch-name>
```

### Switching Branch

```bash
# Check for uncommitted changes first
git status

# If clean:
git checkout <branch>

# If dirty: ask user what to do
```

---

## Quick Reference

| Action | Requires |
|--------|----------|
| `git add` | Implied by commit request |
| `git commit` | Explicit "commit" from user |
| `git push` | Explicit "push" from user |
| `git reset` | Explicit permission |
| `git revert` | Explicit permission |
| `git checkout -- file` | Explicit permission |

---

## Enforcement Checklist

Before committing:

- [ ] User explicitly requested commit
- [ ] `git status` shows expected files
- [ ] `git diff --staged` reviewed
- [ ] Tests pass (`make test`)
- [ ] Commit message follows convention

Before pushing:

- [ ] User explicitly requested push
- [ ] `git log origin/main..HEAD` shows expected commits
- [ ] Branch is correct

---

**Updated:** 2025-12-19
