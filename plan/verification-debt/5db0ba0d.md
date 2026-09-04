# Verification debt -- commit session 5db0ba0d

Gates that had not run green over these commits when they were made.
Clear rows only through `le commit debt-clear` after the named gate exits 0.

| Date | Session | Subject | Gate owed | Reason | Status |
|------|---------|---------|-----------|--------|--------|
| 2026-09-03 | 5db0ba0d | docs(plan): close the spec that measured the RFC extraction drain | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-09-03 | 5db0ba0d | docs(plan): remove the closed RFC drain quota spec | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-09-03 | 5db0ba0d | docs(plan): drop the resurrected pending-sync spec | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-09-03 | 5db0ba0d | docs(radius): name the transports the admin path actually reaches | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-09-03 | 5db0ba0d | spec(radius): settle CHAP and EAP admin auth as buildable designs | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-09-03 | 5db0ba0d | feat(radius): bound what the NAS-Port-Id template resolves to | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-09-03 | 5db0ba0d | feat(radius): bound what the NAS-Port-Id template resolves to | full native verification over this commit's Go | no full native verification covers this commit's Go | open |
| 2026-09-03 | 5db0ba0d | spec: close spec-radius-subscriber-attributes | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-09-03 | 5db0ba0d | spec(l2tp): settle the four accounting attributes as options | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-09-03 | 5db0ba0d | spec(radius): emit all four accounting attributes, no knob | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-09-03 | 5db0ba0d | rfc: enrol RFC 3579, RADIUS support for EAP | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-09-03 | 5db0ba0d | spec(radius): let an operator suppress an accounting attribute | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-09-03 | 5db0ba0d | feat(radius): stamp Event-Timestamp on subscriber accounting records | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-09-03 | 5db0ba0d | feat(radius): stamp Event-Timestamp on subscriber accounting records | full native verification over this commit's Go | no full native verification covers this commit's Go | open |
| 2026-09-03 | 5db0ba0d | feat(radius): report Calling-Station-Id on subscriber accounting | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-09-03 | 5db0ba0d | feat(radius): report Calling-Station-Id on subscriber accounting | full native verification over this commit's Go | no full native verification covers this commit's Go | open |
| 2026-09-03 | 5db0ba0d | feat(radius): report Calling-Station-Id on subscriber accounting | discovery-index freshness | ai/PACKAGE-MAP.md drift is another session's ike/eap move to internal/core/eap and the lg-tls-certificate spec row; this commit adds no package and changes no package doc | open |
| 2026-09-03 | 5db0ba0d | refactor(eap): the EAP peer is a core package, not an IKE one | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-09-03 | 5db0ba0d | refactor(eap): the EAP peer is a core package, not an IKE one | full native verification over this commit's Go | no full native verification covers this commit's Go | open |
| 2026-09-03 | 5db0ba0d | refactor(eap): the EAP peer is a core package, not an IKE one | discovery-index freshness | ai/PACKAGE-MAP.md carries another session's uncommitted lg-tls-certificate rows in 2 of its 4 changed lines; my eap rename is in the working tree and lands with whoever commits that file | open |
| 2026-09-03 | 5db0ba0d | feat(radius): give the admin backend a CHAP credential beside PAP | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-09-03 | 5db0ba0d | feat(radius): give the admin backend a CHAP credential beside PAP | full native verification over this commit's Go | no full native verification covers this commit's Go | open |
| 2026-09-03 | 5db0ba0d | fix(bmp): the Loc-RIB emulated peer takes its identity from config | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-09-03 | 5db0ba0d | fix(bmp): the Loc-RIB emulated peer takes its identity from config | full native verification over this commit's Go | no full native verification covers this commit's Go | open |
| 2026-09-03 | 5db0ba0d | spec(radius): prove RADIUS against a server ze did not write | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-09-03 | 5db0ba0d | feat(radius): report Acct-Terminate-Cause on the Stop record | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-09-03 | 5db0ba0d | feat(radius): report Acct-Terminate-Cause on the Stop record | full native verification over this commit's Go | no full native verification covers this commit's Go | open |
| 2026-09-03 | 5db0ba0d | feat(radius): report Acct-Terminate-Cause on the Stop record | discovery-index freshness | ai/PACKAGE-MAP.md drift is another session's ike/eap move to internal/core/eap and the lg-tls-certificate spec row; this commit adds no package | open |
| 2026-09-04 | 5db0ba0d | feat(radius): report Acct-Terminate-Cause on the Stop record | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-09-04 | 5db0ba0d | feat(radius): report Acct-Terminate-Cause on the Stop record | full native verification over this commit's Go | no full native verification covers this commit's Go | open |
| 2026-09-04 | 5db0ba0d | feat(radius): report Acct-Terminate-Cause on the Stop record | discovery-index freshness | ai/PACKAGE-MAP.md drift is another session's ike/eap move to internal/core/eap and the lg-tls-certificate spec row; this commit adds no package | open |
| 2026-09-04 | 5db0ba0d | plan: hand over five agents an account limit stopped mid-flight | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-09-04 | 5db0ba0d | plan: hand over five agents an account limit stopped mid-flight | discovery-index freshness | ai/PACKAGE-MAP.md holds several sessions' regenerated rows, the eap move and an lg-tls row among them; staging it would carry their work, and this commit adds no package | open |
| 2026-09-04 | 5db0ba0d | fix(interop): the tunnel proof reads each direction separately | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-09-04 | 5db0ba0d | fix(interop): the tunnel proof reads each direction separately | full native verification over this commit's Go | no full native verification covers this commit's Go | open |
| 2026-09-04 | 5db0ba0d | fix(interop): the tunnel proof reads each direction separately | discovery-index freshness | ai/PACKAGE-MAP.md holds several other sessions' regenerated rows; this commit adds no package | open |
| 2026-09-04 | 5db0ba0d | test(bmp): prove Loc-RIB against two receivers ze did not write | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-09-04 | 5db0ba0d | test(bmp): prove Loc-RIB against two receivers ze did not write | full native verification over this commit's Go | no full native verification covers this commit's Go | open |
| 2026-09-04 | 5db0ba0d | test(bmp): prove Loc-RIB against two receivers ze did not write | discovery-index freshness | ai/PACKAGE-MAP.md holds several other sessions' regenerated rows; this commit adds no package | open |
