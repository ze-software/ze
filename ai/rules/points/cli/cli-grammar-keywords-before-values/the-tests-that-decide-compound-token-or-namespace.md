---
kind: directive
level: MUST
stage:
---
Whether a hyphenated string is one compound token or a namespace MUST be decided by
these tests, in order:

1. Would you naturally say the two parts separately about the object ("show the
   *health* of *bgp*", "show the *feature* signals for *traffic*")? If yes, they are
   two tokens. The left part becomes a container node so the tree stays object-rooted
   and completion can enumerate the members
   (`docs/architecture/cli/command-namespacing.md`).
2. Is the whole string the actual name of one thing you would never break apart? An
   industry term of art (`as-set`, `graceful-restart`, `segment-routing`,
   `adj-rib-in`, `class-of-service`), a protocol / LSA / object name (`opaque-area`,
   `asbr-summary`, `router-information`), or a single attribute (`asn-name`,
   `max-prefix`, `file-descriptors`). If yes, MUST keep the hyphen.
3. A shared prefix is not proof of a namespace. `flow-export` (NetFlow/IPFIX) and
   `flow-recent` (conntrack ring) share "flow" by accident; they are not
   `show flow {export,recent}`. MUST split only when the prefix is a real object that owns
   every child.
4. A split namespace needs one owning module. If several components share the prefix,
   one module owns the container and the others augment it (as `trafficusage` augments
   `traffic`). A shared parent that multiple plugins reach into breaks plugin
   self-containment, so it MUST NOT be created.
