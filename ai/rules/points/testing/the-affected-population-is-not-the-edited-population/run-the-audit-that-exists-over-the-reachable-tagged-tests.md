---
kind: directive
level: MUST
stage:
---
**The audit that reads whether a test still enforces what it names already exists, so MUST run it over the reachable set rather than write a new one.** `/ze-rfc-audit` records a verdict per requirement, and `check_audit_freshness` (`internal/le/rfc/rfc.go`) invalidates that verdict when the tagged test changes. What is missing is a trigger and a scope: nothing routes a semantics change to that audit, and its population is edited files.

**A fixture carrying no RFC tag has no audit, so MUST read its header against its config yourself.** A header that names the rail, the plugin or the topology under test is the assertion the runner cannot check. When the change contradicts that header, the fixture is now testing something else and MUST be fixed in the same work, never left green.
