---
kind: directive
level: MUST
stage:
rationale: ai/rationale/user-facing-errors.md
---
- **Every `Warn` or `Error` log line and every report-bus raise MUST name, under its `code` attribute, a doctor code that a registered doctor check declares, and a transient problem MUST also be raised on the report bus under that code.** The `code` key is reserved for the doctor code: a protocol code or an exit code MUST go under another key. A line that reports nothing wrong MUST be logged at `Info` or `Debug`, never at `Warn`. `./le arch doctor-coverage check` refuses a site a change adds without a covered code, and `./le arch doctor-coverage report` lists the existing ones; what counts as a site and how its code resolves are in `docs/architecture/doctor-and-health-checks.md`, "Every problem site names a doctor code".
