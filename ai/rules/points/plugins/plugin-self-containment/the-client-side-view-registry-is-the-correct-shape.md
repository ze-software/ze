---
kind: note
level:
stage:
---
Correct (live in the tree): the client-side view registry in
`internal/component/cli/view_registry.go` (`RegisterView(viewSpec{key, prefix,
matches, start})`, `RegisteredViews()`, longest-prefix `resolveView` copied from
`handler.go` `matchesPrefix`). Each view registers from its own
`register_view_*.go` `init()` (`ViewKeyPing`/`ViewKeyTraceroute`/`ViewKeyDashboard`)
and hangs its session state off the single `Model.activeView` handle plus the
generic `Model.viewFactories` store, with no per-feature field. Consumers iterate
`cli.RegisteredViews()` and inject each factory by key via `SetViewFactory`, not
three typed setters. The `TestModelHasNoPerFeatureViewField` reflection guard
(`internal/component/cli/model_test.go`) fails if a per-feature field returns.
