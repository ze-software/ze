# Third-Party Credits

## Meta

| Field | Value |
|-------|-------|
| Name | Third-Party Credits |
| Kind | daemon |
| Scope | complete |
| Level | experimental |
| Components | internal/plugins/thanks, internal/core/credits |
| Real-path tests | test/ui/thanks-list.ci, test/ui/thanks-licence.ci, test/plugin/thanks-daemon-list.ci, test/plugin/thanks-daemon-licence-name.ci |
| Docs | docs/guide/command-reference.md |

## Description

`ze thanks` lists every third-party work compiled into the binary that answers, with the version it carries, its licence, its copyright holder, why Ze uses it and where its source is; `ze thanks licence` prints the licence texts the binary carries as one document, narrowed to one work by its `name` pipe operation. The daemon serves both as `show thanks list` and `show thanks licence`. A module is listed when the binary's build information links it, Go itself in every binary at the Go version it was built with, and any other work when the feature carrying it is compiled in; a linked module with no record is listed as `not recorded` and the command exits non-zero. <!-- source: internal/core/credits/credits.go -- carried, document --> <!-- source: internal/plugins/thanks/thanks.go -- listHandler, licenceHandler -->
