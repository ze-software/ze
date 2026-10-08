# Site: the 177 legacy URLs 404, owner decision 2026-10-08

The owner withdrew AC-12 of spec-site-renderers-in-go on
2026-10-08: no redirect stubs and no legacy-URL rewriting, and the paused code
is deleted with the legacy table rather than kept unregistered (no-layering).
The behavior these tests proved no longer exists.

| Test | Reason |
|------|--------|
| TestCheckRefusesAPublishedRouteWithNoProducer | The assertion that the coverage names spec-site-renderers-in-go is removed with `Coverage.Spec`: that field named the open work owning a coverage red, the spec closed with every route written by a producer, and a red now names a real defect. Every behavior assertion stays: the unclaimed route by name, `Red()`, the non-zero exit, and the record kept out of the artifact. |
