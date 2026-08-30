---
kind: directive
level: MUST
stage:
---
- **A closure commit MUST carry a CLEAN `./le spec session review record` artifact that covers every reviewable file in it, and whose hashes still match.** Any edit after the review invalidates the artifact and forces a fresh pass, and a code-free closure still owes one. `CheckReview` in `internal/le/commit/review.go` refuses the commit otherwise; `docs/contributing/spec-workflow.md` says which commit counts as the closure.
- **`review-override <reason>` MUST be an explicit owner decision.** It records a verification-debt row, and an open row refuses the next push.
