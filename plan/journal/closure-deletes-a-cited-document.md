| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-08-10 | fixit-spec-closure-leaves-dangling-spec-citations | workflow | Spec closure removes the spec file. Every sibling citation of it then points at nothing. The gate that finds them existed and was correct. No closure step fed it, so it drifted to 15 red rows | a BLOCKING step before commit B. The author who removes the spec names and repairs its citers, inside commit A |
