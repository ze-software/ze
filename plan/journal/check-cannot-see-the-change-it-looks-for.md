# Check cannot see the change it looks for

A check runs, answers, and is believed. Both of its inputs were narrowed before
the comparison, so the difference it exists to announce cannot appear in either
one. Nothing is silent and nothing is red: the check prints a confident line
about values that no longer carry the fact.

This is the input-side twin of `green-that-could-not-have-been-red`. There the
observation itself cannot come out otherwise. Here the observation is fine, and
each of the two things observed was replaced by a stand-in first.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-09-02 | - | le vendor-web update-report | The report that announces a newer vendored web asset cannot announce htmx 4.0.0, released five days earlier. Both inputs were narrowed. semverRE read only X.Y.Z from MANIFEST.md, so the vendored 4.0.0-beta6 read as 4.0.0. fetchLatestNpmVersion asked for the latest dist-tag alone, and htmx published 4.0.0 under next while latest stayed on 2.0.10. The two narrowings printed "4.0.0 -> 2.0.10 available", a recommendation to downgrade, over an upgrade the tree owed | fixed on the spot, as code related to the htmx 4.0.0 vendoring. semverRE keeps the prerelease suffix. newestPublishedRelease reads every dist-tag and answers the newest release among them. laterRelease orders a prerelease before the release sharing its triple. The rendering then separates an upgrade from a tree that runs ahead of every release |
