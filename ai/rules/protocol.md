# Protocol Implementation

**When:** implementing or changing a protocol, an external API, a wire format, or a backend that applies operator config
**Severity:** blocking
**Related:** rfc-compliance, completion, planning, go-standards, architecture, plugins

## Directives

**Code that implements an external API, follows an RFC, matches another project's format, or uses a vendored library MUST carry the upstream reference inline, at the top of the file after the `// Design:` and `// Related:` lines.** An external API names the upstream repository URL, the spec or endpoints file, and the consuming projects. RFC code carries `// RFC NNNN Section X.Y -- see rfc/short/rfcNNNN.md` beside the relevant code. A foreign format names the URL of the format definition, and a vendored library names its version and source URL. An internal ze-to-ze API, standard library usage, and a well-known protocol whose RFC number already sits in a comment need none.

**A new protocol SHOULD follow the subpackage skeleton, and BFD SHOULD be treated as the reference layout:** `packet`, `engine`, `session`, `transport`, `auth`, `cmd`, `api`, `yang`. A protocol at root-package-plus-`yang` size needs none of it. The skeleton is ADVISORY for existing code: no moves, no renames, no gate, and `./le protocol-skeleton report` always exits 0. The modules and how each existing protocol maps to them are in `docs/architecture/protocol-skeleton.md`.
<!-- source: internal/component/bfd -- subpackage layout -->
