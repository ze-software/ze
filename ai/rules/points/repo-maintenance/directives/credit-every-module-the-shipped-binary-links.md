---
kind: directive
level: MUST
stage:
---
**A commit that first imports a third-party module into code that ships MUST carry the module's entry in `website/data/dependencies.json` and the credits data `./le deps credits update` generates from it (`internal/core/credits/data/`), and a commit that changes the declaration, the data, or a credited work's licence or notice file MUST carry data generated from the declaration it leaves.** The `deps credits` change-set check refuses the commit otherwise. What the entry holds is in `docs/contributing/dependency-approval.md`, "What a new dependency owes".
