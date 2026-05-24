# Architecture Decisions

This directory holds durable architecture decisions for Ze. Use it for choices that future work must respect: accepted designs, rejected alternatives, and deferred decisions with clear revisit triggers.

<!-- source: ai/rules/planning.md - Plan File Location, Writing Learned Summaries -->

Use `plan/spec-*.md` for work to implement, and `plan/learned/NNN-*.md` for summaries of completed implementation work. Decision records stay here because they describe why the architecture has a shape, even when no implementation is happening now.

<!-- source: ai/rules/planning.md - Spec Rules, Writing Learned Summaries -->

## Index

| ID | Decision | Status | Summary |
|----|----------|--------|---------|
| 001 | [Defer pull-model metrics collector](001-pull-model-metrics.md) | deferred | Keep the push-model `metrics.Registry` path for Ze Prometheus metrics; revisit only when external-plugin metrics or another trigger forces the shape. |

<!-- source: docs/architecture/decisions/001-pull-model-metrics.md - Decision -->

## Format

Each decision should include:

| Section | Purpose |
|---------|---------|
| Status | `accepted`, `deferred`, `rejected`, or `superseded` |
| Context | The concrete state of the code or system when the decision was made |
| Decision | The choice, including rejected alternatives where useful |
| Consequences | What future work must preserve, avoid, or revisit |
| Revisit Triggers | Conditions that make the decision worth reopening |

<!-- source: docs/architecture/decisions/001-pull-model-metrics.md - metadata and Decision -->
