# Owner ruling: `ze init` claims every discovered device

Recorded 2026-11-22. The Source of the `## Design decisions` row in
`docs/features/interfaces.md`.

## The question

`ze init`'s default profile writes every device interface discovery finds into
`file/active/<name>.conf`, with nothing that excludes a device another program
owns (`docker0`, `br-*`, `virbr0`, container veths, `wg-quick` devices). A root
ze then manages each. Should `ze init` claim devices another program owns?

## Thomas's answer, verbatim

> claim when using init: yes by default, perhaps with --confirm you could have
> the user confirm each one

## What was built from it

The default is unchanged: every discovered device is written. `ze init
--confirm` asks about each discovered device the config can hold and writes
only the ones answered yes (`internal/plugins/init/main.go`,
`confirmInterfaces`).
