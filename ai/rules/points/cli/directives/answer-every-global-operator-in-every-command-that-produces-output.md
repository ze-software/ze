---
kind: directive
level: MUST
stage:
---
- **Every command that produces output MUST answer every GLOBAL operator on every surface where the catalog makes that operator available.**
  - **Global formats:** Global means independent of answer shape, not execution surface. The six formats and `no-more` act on every answer everywhere.
  - **Local-only save:** `save` is catalogued `local-only`. It MUST be refused by name when a daemon expands a remote SSH or web chain.
  - **Streaming log:** A STREAM operator such as `log` is available only `when-streaming`, where the command keeps answering.
  - **Qualifier preservation:** Published command contracts and every consumer MUST preserve `always`, `with-rows`, and `when-streaming`. They MUST also preserve the independent `local-only` surface restriction. They MUST NOT flatten either qualifier into unconditional support.
  - **Catalog derivation:** The operator catalog (`internal/component/command/pipe_catalog.go`) is the source for class and surface. Each command's availability is derived from that catalog and its declarations, never from a hand-copied list.
