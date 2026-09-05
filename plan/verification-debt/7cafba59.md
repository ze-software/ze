# Verification debt -- commit session 7cafba59

Gates that had not run green over these commits when they were made.
Clear rows only through `le commit debt-clear` after the named gate exits 0.

| Date | Session | Subject | Gate owed | Reason | Status |
|------|---------|---------|-----------|--------|--------|
| 2026-09-05 | 7cafba59 | give the le namespace surface the three proofs it was missing | full native verification (not FRESH-green) | unit tests green for internal/le/cligrammar (bar the two foreign --flag-in-YANG findings in internal/component/hub/yang/ze-hub-conf.yang, which are red without these files too), internal/le/verify, internal/le/leroot; the new .ci passes under bin/ze-test ui. The three red cases in internal/test/fixture are foreign and were proven red with my two files moved aside | open |
| 2026-09-05 | 7cafba59 | give the le namespace surface the three proofs it was missing | full native verification over this commit's Go | no full native verification covers this commit's Go | open |
