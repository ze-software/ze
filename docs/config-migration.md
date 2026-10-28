# Configuration Migration

Ze migrates older pre-release or ExaBGP-shaped configuration files using named
transformations. These transformation names are implementation labels, not
public config version numbers. Ze has not shipped a stable configuration release
yet, so compatibility is date-based and tied to the schema stamp written by the
current binary.

<!-- source: internal/component/config/cli/cmd_migrate.go -- ze config migrate command -->
<!-- source: internal/component/config/cli/cmd_fmt.go -- ze config fmt command -->
<!-- source: internal/component/config/cli/cmd_validate.go -- ze config validate command -->
<!-- source: internal/component/config/migration/migrate.go -- transformation registry -->

## Quick Start

```bash
# Check whether the file matches the current binary
ze config validate myconfig.conf

# Preview migration changes
ze config migrate --dry-run myconfig.conf

# Convert to a new file
ze config migrate -o myconfig-current.conf myconfig.conf

# Format and normalize current syntax
ze config fmt myconfig-current.conf

# Format in place
ze config fmt -w myconfig-current.conf

# Check if formatting is needed
ze config fmt --check myconfig-current.conf
```

## Available Transformations

View the transformation list in the binary you are running:

```bash
$ ze config migrate --list

Available transformations (in order):
  neighbor->peer             Rename 'neighbor' blocks to 'peer'
  peer-glob->template.match  Move glob patterns (10.0.0.0/8) to template.match
  template.neighbor->group   Rename template.neighbor to template.group
  static->announce           Convert 'static' route blocks to 'announce'
  api->new-format            Convert old api syntax to named blocks
  remove-bgp-listen          Remove ExaBGP legacy bgp listen leaf
  remove-tcp-port            Remove environment tcp port
  remove-env-bgp-connect     Remove environment bgp connect
  remove-env-bgp-accept      Remove environment bgp accept
  hub-server-host-to-ip      Rename plugin hub server host to ip
  log-booleans-to-subsystems Convert boolean log topics to subsystem levels
  listener-to-list           Convert flat host+port to server list format
  wrap-bgp-block             Wrap BGP elements in bgp {} block
  template->group            Convert template block to bgp peer-groups
```

## Migration Detection

`ze config migrate --dry-run` reports which named transformations would apply.

Older shapes that trigger migration include:

- `neighbor <IP> { }` at root level
- `peer <glob> { }` at root level, for example `peer * { }`
- `template { neighbor <name> { } }`
- `static { }` route blocks
- old-style `api { processes [...] }` syntax
- ExaBGP environment leaves that Ze does not model as native config

Current syntax uses:

- `bgp { peer <name> { ... } }`
- `bgp { group <name> { ... } }`
- `bgp { group <name> { peer <name> { ... } } }`
- `connection { remote { ip ... } }` for transport
- `session { asn { local ...; remote ...; } }` for BGP ASNs
- `announce { ... }` blocks for local routes
- named API blocks

## Common Transformations

| Older shape | Current shape |
|-------------|---------------|
| `neighbor <IP> { }` | `bgp { peer <name> { } }` |
| `peer * { }` or other root glob | `bgp { group <name> { ... } }` defaults |
| `template { neighbor <name> { } }` | `bgp { group <name> { } }` |
| `local-as` / `peer-as` leaves | `session { asn { local ...; remote ...; } }` |
| flat remote address leaves | `connection { remote { ip ...; } }` |
| `static { }` | `announce { }` |

## Example

Older local file:

```
template {
    neighbor defaults {
        hold-time 90;
    }
}

neighbor 192.0.2.1 {
    inherit defaults;
    local-as 65000;
    peer-as 65001;
}
```

Current native shape:

```
bgp {
    router-id 192.0.2.254
    session { asn { local 65000; } }

    group defaults {
        timer { receive-hold-time 90; }

        peer upstream1 {
            connection {
                remote { ip 192.0.2.1; }
            }
            session {
                asn { remote 65001; }
            }
        }
    }
}
```

## Commands

### `ze config migrate --dry-run`

Shows what would be converted:

```bash
$ ze config migrate --dry-run old.conf
Transformation analysis:
  [done] peer-glob->template.match
  [done] template.neighbor->group
  [pending] neighbor->peer
  [pending] api->new-format

Result: 2 transformation(s) would apply. All would succeed.
```

### `ze config migrate`

Converts a file to the current syntax:

```bash
# List transforms
$ ze config migrate --list

# Preview conversion
$ ze config migrate --dry-run old.conf

# Convert to stdout
$ ze config migrate old.conf

# Write to a new file
$ ze config migrate -o current.conf old.conf

# Read from stdin
cat old.conf | ze config migrate -
```

Flags:

- `--list` shows available transformations.
- `--dry-run` reports what would happen without applying changes.
- `format <form>`, typed before the file, sets the output form: `set` by
  default, or `hierarchical`. It is a keyword, not a flag:
  `ze config migrate format hierarchical config.conf`.
- `-o <file>` writes to the specified file.
- `-` reads from stdin and, for `migrate -o -`, writes the result to stdout.

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

fmt never changes a value. A value that would not read back as itself when
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
<!-- source: internal/component/config/cli/cmd_fmt.go -- encodeSecrets -->
<!-- source: internal/component/config/tree.go -- SetStoredSecret -->

## Unsupported ExaBGP Features

Some ExaBGP features are detected but not implemented in Ze:

| Feature | Location | Result |
|---------|----------|--------|
| `multi-session` | `capability { }` | Warning |
| `operational` | `capability { }` | Warning |
| `operational` block | `peer { }` | Warning |

Warnings do not make the file loadable by themselves. Validate the migrated
output with the same binary that will run it.

## Error Handling

If migration fails:

1. The original file is unchanged.
2. The error names the transformation or syntax that failed.
3. Fix the source file and rerun `ze config migrate --dry-run`.

Common causes:

- invalid syntax in the source config
- conflicting definitions after migration
- file permission errors
