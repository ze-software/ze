# Verification debt -- commit session 5db0ba0d

Gates that had not run green over these commits when they were made.
Clear rows only through `le commit debt-clear` after the named gate exits 0.

| Date | Session | Subject | Gate owed | Reason | Status |
|------|---------|---------|-----------|--------|--------|
| 2026-09-03 | 5db0ba0d | docs(plan): close the spec that measured the RFC extraction drain (+54 more) | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-09-03 | 5db0ba0d | feat(radius): bound what the NAS-Port-Id template resolves to (+25 more) | full native verification over this commit's Go | no full native verification covers this commit's Go | open |
| 2026-09-03 | 5db0ba0d | feat(radius): report Calling-Station-Id on subscriber accounting | discovery-index freshness | ai/PACKAGE-MAP.md drift is another session's ike/eap move to internal/core/eap and the lg-tls-certificate spec row; this commit adds no package and changes no package doc | open |
| 2026-09-03 | 5db0ba0d | refactor(eap): the EAP peer is a core package, not an IKE one | discovery-index freshness | ai/PACKAGE-MAP.md carries another session's uncommitted lg-tls-certificate rows in 2 of its 4 changed lines; my eap rename is in the working tree and lands with whoever commits that file | open |
| 2026-09-03 | 5db0ba0d | feat(radius): report Acct-Terminate-Cause on the Stop record (+1 more) | discovery-index freshness | ai/PACKAGE-MAP.md drift is another session's ike/eap move to internal/core/eap and the lg-tls-certificate spec row; this commit adds no package | open |
| 2026-09-04 | 5db0ba0d | plan: hand over five agents an account limit stopped mid-flight | discovery-index freshness | ai/PACKAGE-MAP.md holds several sessions' regenerated rows, the eap move and an lg-tls row among them; staging it would carry their work, and this commit adds no package | open |
| 2026-09-04 | 5db0ba0d | fix(interop): the tunnel proof reads each direction separately (+7 more) | discovery-index freshness | ai/PACKAGE-MAP.md holds several other sessions' regenerated rows; this commit adds no package | open |
| 2026-09-04 | 5db0ba0d | feat(radius): the admin backend speaks EAP over a signed exchange | discovery-index freshness | ai/PACKAGE-MAP.md and the rfc requirement indexes hold several other sessions' regenerated rows | open |
| 2026-09-04 | 5db0ba0d | fix(ike): gofmt the imports the eap package move left unsorted | discovery-index freshness | This commit reorders eleven import lines and adds, removes and renames no package, so ai/PACKAGE-MAP.md owes it nothing. That file's one working-tree change is another session's: it drops internal/component/bgp/plugins/redistribute_ingress and adds internal/core/rib/distance, which de739c8b2e introduced. Staging it would carry their work. | open |
| 2026-09-04 | 5db0ba0d | docs(l2tp): the accounting attributes, their causes, and the knob (+2 more) | discovery-index freshness | ai/PACKAGE-MAP.md holds other sessions' regenerated rows; this commit adds no package | open |
