# Configuration Migration

Ze keeps the infrastructure that converts a config written in an older format
of its own syntax: `ze config migrate` and the start-time schema evolution run
every conversion a package registers
([syntax](architecture/config/syntax.md), "Schema Evolution and Migration").
The conversion of one format change is written only once Ze has a stable
release, so none is registered today. A file in a form the schema no longer
accepts fails to parse, naming the line and the unknown keyword: edit it to
the current syntax, which the [configuration guide](guide/configuration.md)
describes. The configs in the repository always use the latest format.
<!-- source: internal/component/config/migration/migrate.go -- transformations, Migrate -->
<!-- source: cmd/ze/hub/main_evolve.go -- applyEvolutions -->

An ExaBGP configuration is a different case and keeps its converter:
`ze exabgp migrate` reads ExaBGP syntax and writes Ze syntax
([ExaBGP syntax](architecture/config/exabgp-syntax.md)).

`ze config migrate` applies the registered conversions, then writes the
configuration in either of its two forms, set commands and nested blocks, and
hashes plaintext passwords on the way.

The daemon accepts the same conversion as `show config supplied migrate content
<base64> [name <filename>] [format set|hierarchical]`. It returns the converted
text without opening a caller filename or changing a candidate. The local file
command and the supplied command use the same conversion producer. The browser
discovers this operation from its registered file/paste input descriptor.
<!-- source: internal/component/config/cli/config_supplied_tools.go -- suppliedMigrate -->
<!-- source: internal/component/config/cli/cmd_migrate.go -- configMigrateBytes -->

<!-- source: internal/component/config/cli/cmd_migrate.go -- ze config migrate command -->
<!-- source: internal/component/config/cli/cmd_fmt.go -- ze config fmt command -->
<!-- source: internal/component/config/cli/cmd_validate.go -- ze config validate command -->

`ze config validate --limit environment` checks explicitly set registered
environment values against their declared types, and includes range failures
already recorded by their readers. It returns exit 1 for invalid values and
keeps `-q` silent. `show config environment-validation` returns the same typed
verdict and corrective commands through every command surface. Each route
judges the environment of the process answering it; a local invocation does
not claim to inspect another process's environment.
<!-- source: internal/component/config/cli/config_environment.go -- environmentValidationAnswer -->
<!-- source: internal/component/config/cli/cmd_validate.go -- validateEnvironment -->

## Quick Start

```bash
# Check whether the file matches the current binary
ze config validate myconfig.conf

# Convert an ExaBGP config to Ze syntax
ze exabgp migrate exabgp.conf > myconfig.conf

# Write a Ze config in the set form
ze config migrate -o myconfig.set myconfig.conf

# Format and normalize the hierarchical form
ze config fmt -w myconfig.conf

# Check if formatting is needed
ze config fmt --check myconfig.conf
```

## Commands

### `ze config migrate`

Rewrites a current config, in either form, in the form named:

```bash
# Convert to the set form on stdout
$ ze config migrate config.conf

# Convert to nested blocks
$ ze config migrate format hierarchical config.set

# Write to a new file
$ ze config migrate -o current.conf config.conf

# Read from stdin
cat config.conf | ze config migrate -
```

Flags:

- `format <form>`, typed before the file, sets the output form: `set` by
  default, or `hierarchical`. It is a keyword, not a flag:
  `ze config migrate format hierarchical config.conf`.
- `-o <file>` writes to the specified file.
- `-` reads from stdin and, for `migrate -o -`, writes the result to stdout.
- `--list` names the conversions this binary carries, in the order they run.
- `--dry-run` reports which conversions the file needs, and whether each would
  succeed, without writing anything; it cannot be combined with `-o`.

The input must parse under the current schema, or under the migration schema
that also accepts each registered relocated block and retired leaf at its old
position; with no conversion registered, `ze config migrate` changes the form,
never the syntax. On stderr it lists each conversion applied or not needed. A feature Ze does not implement, such as the
`multi-session` capability or an `operational` block, is not in the Ze grammar,
so it stops the parse with `line N: unknown field`
([syntax changes](architecture/config/deprecated-options.md)).

A `plaintext-password` in the input is written as its bcrypt hash in the
`password` leaf, in both output forms, as `ze config fmt` and a commit do; a
weak password is named on stderr. Both forms write each `ze:sensitive` key
in the `$9$` form, as `ze config fmt` does, and both read a `$9$` string back
as the key it encodes.

### `ze config fmt`

Formats and normalizes current config syntax:

```bash
# Print formatted config to stdout
$ ze config fmt config.conf

# Write back to file
$ ze config fmt -w config.conf

# Check if formatting is needed
$ ze config fmt --check config.conf

# Show diff of changes
$ ze config fmt --diff config.conf
```

Flags:

- no flag prints to stdout.
- `-w` writes the result to the source file, or to stdout when the input is `-`.
- `--check` exits 1 if formatting is needed.
- `--diff` shows a unified diff.

fmt keeps every comment beside the statement it was written beside, and the
schema stamp on the first line, while it puts the statements in canonical
order (`docs/architecture/config/syntax.md`, "Comments"). It keeps the meaning
of every value; blank lines, indentation, quoting and one-line spellings are
the canonical form's.

fmt reads the hierarchical form only. A file in the set or set-meta form, the
form the editor stores, is refused with exit 2, and the message gives the
command that converts it:

```
stored.conf is written in the set form, and ze config fmt formats only the hierarchical form.
Convert it to the hierarchical form first:
  ze config migrate format hierarchical stored.conf
```

fmt never changes what a value means. A value that would not read back as itself when
written bare is written quoted: one holding whitespace, a quote, `#`, `;`, a
brace or a bracket (other than a `:[address]` such as
`redirect:[2001:db8::1]:100`), or starting with `(` or `)`. Inside the quotes, `"`, `\`, a
newline and a tab are written `\"`, `\\`, `\n` and `\t`.

Two kinds of value fmt rewrites. A `plaintext-password` is written as its
bcrypt hash in the `password` leaf, the hash a commit would store, because the
plaintext leaf is never written to a file. A weak password is named on stderr.
A key in a `ze:sensitive` leaf (an MD5 password, a pre-shared key) is never
written in clear: a key given in clear is written in the `$9$` form, and a key
already in the `$9$` form is written back exactly as it was read, so a second
fmt changes nothing. `ze secret decode` reads the key back.
<!-- source: internal/component/config/cli/cmd_fmt.go -- fmtTree, hashPlaintextPasswords -->
<!-- source: internal/component/config/secret_text.go -- encodeSecrets, encodeSecretLeaf -->
<!-- source: internal/component/config/tree.go -- SetStoredSecret -->

## Error Handling

If `ze config migrate` fails, the original file is unchanged and the error
names the line that did not parse. Common causes:

- a keyword from an older pre-release Ze syntax: edit the file to the current
  syntax
- an ExaBGP config: convert it with `ze exabgp migrate`
- file permission errors
