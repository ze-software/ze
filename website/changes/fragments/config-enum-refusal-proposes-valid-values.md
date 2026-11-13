---
spec: config-enum-refusal-proposes-valid-values
topic: Config
feature: configuration
found-by: review
---

## Before

A refused configuration value could leave you with a short error, a missing
setting path, or a correction that could not be entered from your current
position. Some editors accepted values that the configuration parser refused.

## After

Configuration refusals name the setting and show the accepted values with
their help, the allowed bounds, or the required format. Each refusal ends with
a correction for the surface you used. Sensitive values stay masked. The SSH
editor, web editors, configuration files and APIs use the same schema checks.

In an editor, `top <command>` runs from the root and keeps your position unless
the command moves it. Printed corrections preserve quoted names and literal
pipes. A refused commit keeps your pending changes available for repair.

## Corrects

none

## Exposure

none

## Upstream

none

## Asked by

none
