# Understand Before Coding

## Core Principle

**You are NOT ALLOWED to write any code until you understand the existing code structure.**

This is a BLOCKING requirement. No exceptions.

## Before Writing Any Code

Complete these steps IN ORDER:

```
[ ] 1. Search codebase for related code
      - Use Grep/Glob to find similar patterns
      - Look for existing implementations of similar features
      - Find where related functionality lives

[ ] 2. Read the relevant source files
      - Understand current implementation
      - Note the patterns used
      - Identify extension points

[ ] 3. Check architecture docs
      - Read docs matching task keywords
      - Understand design decisions
      - Note constraints and requirements

[ ] 4. Understand data flow
      - How does data enter the system?
      - What transformations occur?
      - Where does it exit?

[ ] 5. Identify reuse opportunities
      - Can you extend existing code?
      - Are there shared utilities to use?
      - What patterns should you follow?
```

## Verification Questions

Before writing code, you MUST be able to answer:

1. **What existing code relates to this task?**
   - File paths and function names
   - How your code will interact with it

2. **What patterns does the codebase use?**
   - Naming conventions
   - Error handling style
   - Testing patterns

3. **How will your changes integrate?**
   - What calls your new code?
   - What does your new code call?
   - What data structures are shared?

## Red Flags

Stop and investigate further if:

- You're about to create a new file without checking for similar existing files
- You're writing a utility function without searching for existing utilities
- You don't know what package your code belongs in
- You can't name 3 existing files your code relates to

## Why This Matters

Skipping this step leads to:

- **Duplicate code** - Reimplementing existing functionality
- **Inconsistent patterns** - Code that doesn't fit the architecture
- **Integration issues** - Changes that break existing functionality
- **Wasted effort** - Rewriting code that needs to match existing patterns

## The Right Approach

```
❌ Wrong: "I'll write a new parser for X"
✅ Right: "Let me search for existing parsers to understand the pattern"

❌ Wrong: "I'll create internal/newfeature/feature.go"
✅ Right: "Let me find where similar features live and follow that structure"

❌ Wrong: "This needs a new struct for Y"
✅ Right: "Let me check if there's an existing struct I should extend"
```
