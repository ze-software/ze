# Verification debt -- commit session 3b09fdcb

Gates that had not run green over these commits when they were made.
Clear rows only through `le commit debt-clear` after the named gate exits 0.

| Date | Session | Subject | Gate owed | Reason | Status |
|------|---------|---------|-----------|--------|--------|
| 2026-08-31 | 3b09fdcb | plan: keep the removed weakened.md rows where they cannot be cleaned | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-09-01 | 3b09fdcb | fix(bgp): name the processes the end-of-rib waits for | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-09-01 | 3b09fdcb | fix(bgp): name the processes the end-of-rib waits for | full native verification over this commit's Go | no full native verification covers this commit's Go | open |
| 2026-09-01 | 3b09fdcb | fix(bgp): name the processes the end-of-rib waits for | discovery-index freshness | the change adds and removes no package: it changes method signatures inside packages ai/PACKAGE-MAP.md already lists | open |
| 2026-09-01 | 3b09fdcb | test: name the check file the draft README actually names | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-09-01 | 3b09fdcb | test: name the check file the draft README actually names | full native verification over this commit's Go | no full native verification covers this commit's Go | open |
| 2026-09-01 | 3b09fdcb | plan: one row for the bmp statistics-timeout defect, not two | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-09-01 | 3b09fdcb | plan: one row for the bmp statistics-timeout defect, not two | independent critical review | this commit closes no spec and carries no code: it merges two journal rows into one. The review gate asks for the artifact of rfcgate-6-supported-extraction-signoff because the surviving row names that spec in its Spec column, which records who found the defect and not what this commit does. That spec belongs to another session and is not mine to review. | open |
| 2026-09-01 | 3b09fdcb | fix(cli): restore the two-path announce grammar the replay dropped | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-09-01 | 3b09fdcb | fix(cli): restore the two-path announce grammar the replay dropped | discovery-index freshness | the change adds and removes no package: one YANG module and two .ci files under paths ai/PACKAGE-MAP.md already lists | open |
| 2026-09-01 | 3b09fdcb | test: the announce action is an alternation, not three optionals | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-09-01 | 3b09fdcb | fix(rfc): drop the imports a disabled body orphans | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-09-01 | 3b09fdcb | fix(rfc): drop the imports a disabled body orphans | full native verification over this commit's Go | no full native verification covers this commit's Go | open |
| 2026-09-01 | 3b09fdcb | fix(rfc): drop the imports a disabled body orphans | discovery-index freshness | the change adds and removes no package: one function and its test inside internal/le/rfc, which ai/PACKAGE-MAP.md already lists | open |
| 2026-09-01 | 3b09fdcb | test(rfc): prove the RFC 2865 walk test discriminates its claims | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-09-01 | 3b09fdcb | plan: the revert route cannot prove a goroutine-served producer | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-09-01 | 3b09fdcb | plan: the revert route cannot prove a goroutine-served producer | independent critical review | this commit closes no spec and carries no code: it is one journal row. The review gate asks for the artifact of rfcgate-6-supported-extraction-signoff because a sibling row in the same class file names that spec, not because this commit does anything for it. | open |
| 2026-09-01 | 3b09fdcb | fix(rfc): attribute a red the break killed the binary with | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-09-01 | 3b09fdcb | fix(rfc): attribute a red the break killed the binary with | full native verification over this commit's Go | no full native verification covers this commit's Go | open |
| 2026-09-01 | 3b09fdcb | fix(rfc): attribute a red the break killed the binary with | discovery-index freshness | the change adds and removes no package: one method and one constant inside internal/le/rfc, which ai/PACKAGE-MAP.md already lists | open |
| 2026-09-01 | 3b09fdcb | test(rfc): prove the RFC 5176 walk test discriminates its claims | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
