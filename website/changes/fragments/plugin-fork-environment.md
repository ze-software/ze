---
spec: plugin-fork-environment
topic: Plugins
feature: plugins
found-by: review
---

## Before

A plugin started in another process could miss settings the daemon held under
dotted or hyphenated environment names and silently use defaults. Conflicting
case spellings could also make the child read a different value from the daemon.

## After

Forked plugins and declaration queries receive shell-safe names for those
settings. The dotted value takes precedence in both processes, and registered
uppercase aliases keep their spelling. The daemon's own connection credentials
still replace inherited credentials for a live plugin.

## Corrects

none

## Exposure

none

## Upstream

none

## Asked by

none
