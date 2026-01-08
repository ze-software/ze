Perform a code review of the current branch compared to the base branch.

  ## Instructions

  1. Determine the base branch:
     - Run `git branch -l main master 2>/dev/null | head -1 | tr -d ' '` to detect `main` or `master`
     - Use the detected branch, defaulting to `main` if neither exists
  2. Get the current branch name using `git branch --show-current`
  3. If current branch equals base branch, output: "Cannot review: currently on the base branch. Check out a feature branch first." and stop.
  4. Get the diff using `git diff <base>...HEAD`
  5. If the diff is empty, output: "No changes to review. Branch is up to date with `<base>`." and stop.
  6. For each file with substantive changes, read the full file to understand context around the diff
  7. Produce the report below

  ## Important

  - DO NOT make any changes to the code—no Edit, Write, or Bash commands that modify files
  - DO NOT create files or branches
  - ONLY output the review report as text
  - ALWAYS include file path and line number references in the format `path/to/file.ext:123`
  - For sections with NO findings: output a single line (e.g., "No issues identified.")
  - For sections WITH findings: use the full format with table
  - For non-applicable sections: output "N/A — [brief reason]" (e.g., "N/A — no concurrent code paths affected")
  - For large diffs (20+ files or 500+ lines): prioritize the top 5 most critical findings per section
  - This keeps the report scannable—clean sections are brief, problem areas are detailed

  ## Severity Definitions

  | Severity | Meaning |
  |----------|---------|
  | Critical | Security vulnerability, data loss risk, or production crash |
  | High | Significant bug, major maintainability issue, or degraded reliability |
  | Medium | Should fix before merge, but won't break production |
  | Low | Minor improvement, style, or nitpick |

  ---

  ## Report Structure

  ### Code Review: [branch-name]

  **Comparing:** `[branch-name]` → `[base-branch]`
  **Files Changed:** X | **Insertions:** +Y | **Deletions:** -Z

  ---

  ### Summary

  Provide a 2-3 sentence summary of what these changes accomplish. Then show the status table:

  | Section | Status |
  |---------|--------|
  | Security | ✅ No issues / ⚠️ X issues (highest: [severity]) |
  | Testing | ✅ No issues / ⚠️ X issues |
  | Error Handling | ✅ No issues / ⚠️ X issues |
  | Code Quality | ✅ No issues / ⚠️ X issues |
  | Observability | ✅ No issues / ⚠️ X issues |
  | Concurrency | N/A / ✅ No issues / ⚠️ X issues |
  | Dependencies | N/A / ✅ No issues / ⚠️ X issues |
  | API Compatibility | ✅ No breaking changes / ⚠️ X breaking changes |
  | Documentation | ✅ Adequate / ⚠️ X issues |
  | Deployment | ✅ No concerns / ⚠️ X concerns |

  **Blockers:** [count] Critical, [count] High — or "None"

  ---

  ### Security

  Analyze the changes for security concerns:
  - Input validation and sanitization (SQL injection, XSS, command injection, path traversal)
  - Authentication and authorization checks
  - Sensitive data exposure (credentials, tokens, PII in logs or error messages)
  - Cryptographic issues (weak algorithms, hardcoded secrets, insufficient entropy)
  - Dependency changes introducing known vulnerabilities

  | Location | Severity | Finding |
  |----------|----------|---------|
  | `path/to/file.ext:42` | Critical/High/Medium/Low | Description of the security concern |

  ---

  ### Testing

  Evaluate test coverage and quality:
  - Are new code paths covered by tests?
  - Do tests verify correct behavior, or merely execute code without meaningful assertions?
  - Are edge cases and error conditions tested?
  - Do existing tests still make sense after these changes, or are they now stale?

  | Location | Issue |
  |----------|-------|
  | `path/to/file.ext:42` | Description of testing gap or concern |

  ---

  ### Error Handling

  Evaluate how errors are managed:
  - Are errors caught and handled appropriately (not silently swallowed)?
  - Are error messages clear and actionable for debugging?
  - Are resources cleaned up on failure (connections, file handles, locks)?
  - Are failures recoverable where they should be?
  - Is there appropriate distinction between retryable and terminal errors?

  | Location | Issue |
  |----------|-------|
  | `path/to/file.ext:42` | Description of error handling concern |

  ---

  ### Code Quality

  Evaluate maintainability and clarity:
  - Code duplication that should be extracted
  - Overly complex code (deep nesting, long functions, high cyclomatic complexity)
  - Poor naming (unclear, misleading, or inconsistent identifiers)
  - Dead code, unused imports, commented-out code
  - Magic numbers/strings that should be named constants
  - Inconsistency with existing codebase patterns

  | Location | Issue |
  |----------|-------|
  | `path/to/file.ext:42` | Description of code quality concern |

  ---

  ### Observability

  Check for appropriate operational visibility:
  - Is there sufficient logging for diagnosing production issues?
  - Are log levels appropriate (debug vs info vs warn vs error)?
  - Is sensitive data excluded from logs?
  - Are metrics, traces, or health signals added where needed?
  - Can issues be diagnosed from logs alone, or is critical context missing?

  | Location | Issue |
  |----------|-------|
  | `path/to/file.ext:42` | Description of observability gap |

  ---

  ### Concurrency

  Analyze thread safety and concurrent execution:
  - Race conditions on shared mutable state
  - Deadlock potential from lock ordering
  - Thread safety of data structures
  - Proper use of synchronization primitives
  - Atomic operations where needed

  | Location | Severity | Issue |
  |----------|----------|-------|
  | `path/to/file.ext:42` | High/Medium/Low | Description of concurrency concern |

  Output "N/A — no concurrent code paths affected" if not applicable.

  ---

  ### Dependencies

  Evaluate dependency changes (additions, removals, version bumps):
  - Is each new dependency justified and necessary?
  - Are dependencies actively maintained and reputable?
  - Are there known vulnerabilities in added/updated packages?
  - Are licenses compatible with the project?
  - Do version bumps include breaking changes?

  | Dependency | Change | Concern |
  |------------|--------|---------|
  | `package-name` | Added/Updated/Removed | Description of concern |

  Output "N/A — no dependency changes" if not applicable.

  ---

  ### API Compatibility

  Evaluate backwards compatibility of interface changes:
  - Are public APIs changed in breaking ways?
  - Are function signatures, response formats, or data structures altered?
  - Are deprecated features properly marked with migration guidance?
  - Will existing consumers break?

  | Location | Breaking Change | Impact |
  |----------|-----------------|--------|
  | `path/to/file.ext:42` | Description of the breaking change | Who/what is affected |

  ---

  ### Documentation

  Evaluate documentation adequacy:
  - Are complex or non-obvious code sections commented?
  - Are public APIs documented (parameters, return values, errors)?
  - Are README or other docs updated if behavior changes?
  - Are TODO/FIXME/HACK comments addressed or tracked?

  | Location | Issue |
  |----------|-------|
  | `path/to/file.ext:42` | Description of documentation gap |

  ---

  ### Deployment

  Evaluate rollout risks based on what's visible in the diff:
  - Can this change be rolled back without data loss or corruption?
  - Do database migrations risk locking or data loss?
  - Are there ordering dependencies (deploy X before Y)?
  - Should new functionality be behind a feature flag?
  - Are environment variables or configuration changes required?

  | Concern | Severity | Mitigation |
  |---------|----------|------------|
  | Description | High/Medium/Low | Suggested approach |

  Note: Flag only concerns evident from the code. Infrastructure dependencies may require additional review.

  ---

  ### Other Issues

  Flag significant concerns not covered above (performance, edge cases, potential bugs):

  | Location | Severity | Issue |
  |----------|----------|-------|
  | `path/to/file.ext:42` | High/Medium/Low | Description |

  ---

  ### What's Done Well

  Acknowledge positive aspects of the changes:

  | Location | Observation |
  |----------|-------------|
  | `path/to/file.ext:42` | Description of good practice or well-implemented code |

  ---

  ### Suggestions

  Optional improvements (not blockers, but worth considering):

  | Location | Suggestion |
  |----------|------------|
  | `path/to/file.ext:42` | Description of potential improvement |

  ---

  ### Verdict

  Provide one of:
  - **Approve** — No Critical/High issues; any Medium/Low issues are acceptable to address post-merge
  - **Request Changes** — One or more issues must be resolved before merge (list them)
  - **Needs Discussion** — Architectural concerns, unclear requirements, or trade-offs requiring team input

  **Criteria applied:**
  - Any Critical issue → Request Changes
  - 2+ High issues → Request Changes
  - Unresolved security finding of Medium+ → Request Changes
  - Architectural ambiguity or multiple valid approaches → Needs Discussion
  - Otherwise → Approve

  **Decision:** [Approve / Request Changes / Needs Discussion]

  **Rationale:** [2-3 sentences explaining the decision, referencing the most important findings]
