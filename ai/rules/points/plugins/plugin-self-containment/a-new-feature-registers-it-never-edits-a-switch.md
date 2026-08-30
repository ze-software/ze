---
kind: directive
level: MUST NOT
stage:
---
- **A new feature MUST NOT require editing a `switch`, a `case`, a field list or a factory in a core or shared package: it registers and is discovered.** This holds for the CLI client model as much as for the daemon's command and schema tree. `plan/TEMPLATE.md` carries it as a review item, and the client-side view registry is described in `docs/architecture/command-ownership.md`, "Registration Over Hardcoding".
