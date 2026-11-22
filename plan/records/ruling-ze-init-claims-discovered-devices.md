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

## Second ruling: only an explicit answer

Recorded 2026-11-22. The first build read an empty line as no and refused the
whole init on any other answer. The Source of the second `## Design decisions`
row in `docs/features/interfaces.md`.

### Thomas's answer, verbatim

> We should make sure that we never accept anything which is not explicit. So
> we require a yes or no, or n; any other entry should not be accepted and
> continue to ask.

Asked whether a bare y counts:

> No, y is accepted.

### What was built from it

`yes`, `y`, `no` and `n`, in any case, are the only answers. An empty line or
any other line is not accepted: ze init writes one notice line on stderr and
asks about the same device again, reading the next line, on a terminal and on
a pipe alike. The prompt shows no default. Input that ends before every device
is answered still refuses the whole init and leaves no store
(`internal/plugins/init/main.go`, `readAnswer`, `writeNotAccepted`).
