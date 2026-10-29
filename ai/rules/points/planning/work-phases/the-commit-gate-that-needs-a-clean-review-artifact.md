---
kind: directive
level: MUST
stage:
---
**A closure commit MUST carry a clean `./le spec review record` artifact covering every reviewable file of the spec's whole change set, whose hashes still match.** The change set is the closure commit, the earlier blocks of its script, and every commit carrying the line `Spec-work: <stem>`, which `./le commit create` writes from the session's claim. An edit to a reviewable file after the review invalidates it and forces a fresh pass, and a code-free closure still owes one. Markdown is never reviewable (`IsReviewCode`, `internal/le/spec/review.go`), so fixing a review NOTE or filling the closure sections in the spec under `plan/` leaves a clean review valid. `checkVerificationGates` (`internal/le/commit/prepare.go`) refuses the commit otherwise, and `review-override <reason>` is an explicit owner decision that writes a `Review-owed:` trailer, which refuses the push until a clean review, a recorded Review Gate or the owner's attestation answers it.
**The artifact proves a fresh, hash-pinned, covering review exists, and it MUST NOT be read as proof that an independent context did the reviewing.** A commit made without a claim, or before the trailer existed, carries no `Spec-work:` line and is not covered by it.
