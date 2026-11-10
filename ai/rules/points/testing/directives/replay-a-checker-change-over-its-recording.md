---
kind: directive
level: MUST
stage:
---
- **A change to an interop checker whose scenario has a committed recording MUST carry a replay test over that recording (`interoplab.ReplayScenario`), and the change MUST NOT be judged by a lab rerun alone.** The replay judges the checker's logic on the answers a real lab run gave it in seconds and with no Docker, so a wrong checker is found in one run, not one lab pass per guess. A reviewer refuses a checker change whose scenario holds a recording under its lab's `testdata/` and whose diff carries no such test. The layers each test proves, and how a recording is made and reviewed, are in `docs/architecture/testing/interop.md`, "Which layer proves what".
- **A recording MUST be reviewed before it is committed, like any fixture:** it carries environment variable names only, and an answer that holds a secret MUST NOT be committed.
