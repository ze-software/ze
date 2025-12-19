# PRE-FLIGHT CHECKLIST

**MANDATORY - Run BEFORE starting ANY work.**

---

## 1. Protocols Read

- [ ] ESSENTIAL_PROTOCOLS.md
- [ ] output-styles/zebgp.md (communication style)
- [ ] GIT_VERIFICATION_PROTOCOL.md
- [ ] CODING_STANDARDS.md
- [ ] TESTING_PROTOCOL.md

**If ANY unchecked: STOP. Read them.**

---

## 2. Git State Verified

```bash
git status && git diff && git diff --staged
```

**Paste output above. If modified/staged files: ASK user before proceeding.**

---

## 3. Plan State Check

```bash
ls -la plan/
```

- [ ] Listed active plan files
- [ ] Checked status emoji in each plan header (🔄/📋/✅/⏸️)
- [ ] Reported to user: "Active plans: [list with status]"
- [ ] Asked user: "Which plan (if any) are we working on today?"

**If working on a plan:** Keep it updated throughout session (see ESSENTIAL_PROTOCOLS.md § Plan Update Triggers)

---

## 4. Codebase References (For New Features/Refactoring)

**If task involves new features, major changes, or unfamiliar areas:**

- [ ] Read relevant codebase reference docs
  - Adding NLRI type → zebgp/wire/NLRI.md, zebgp/EXABGP_CODE_MAP.md
  - Adding attribute → zebgp/wire/ATTRIBUTES.md
  - Understanding pools → zebgp/POOL_ARCHITECTURE.md
  - Edge cases → zebgp/edge-cases/*.md

**Skip this if:** Simple bug fix, known area, documentation-only work

---

## 5. Ready to Work

- [ ] All protocols read
- [ ] Git state checked
- [ ] Plan state checked
- [ ] User informed of any pre-existing changes
- [ ] No assumptions made
- [ ] Relevant codebase references reviewed (if applicable)

**If ANY unchecked: STOP.**

---

**This checklist BLOCKS starting work. Complete it first.**
