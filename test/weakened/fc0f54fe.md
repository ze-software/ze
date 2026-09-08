# Test weakenings this commit accepts

Six helpers left a `_test.go`, and what replaced them reads strictly more than
they did.

`tunnel_endpoint_lint_test.go` declared a reader for the ze configuration a
`.ci` embeds: `namedBlocks`, `firstBlock`, `leafInBlock`, `leaf`,
`atTokenStart` and `braceBody`. This commit needs the same reader in production
code, because `declarePeerAS` (`internal/test/runner/peer_asn.go`) derives each
ze-peer's AS from that configuration, and a helper declared in a `_test.go` is
not reachable from one. They moved to `internal/test/runner/ci_config_read.go`,
in the same package and in the same commit.

The audit reads that as six test bodies replaced with the empty string, because
it sees the declarations leave the test file and does not follow them into the
production file beside it.

**The six were then REPLACED, in this same commit, by a reader that finds
everything they found and more.** They matched `keyword + " "` at a token start,
so a leaf written `local<TAB>65000` was not found, a block written
`peer<TAB>name {` was not found, and a `#` comment holding a brace unbalanced
the brace counter. Each of those came back as `""`, indistinguishable from a
configuration that declares nothing, which is the defect the spec this commit
belongs to exists to remove. `ci_config_read.go` now TOKENIZES the text with
ze's own separator set (copied from `readWord` and `skipWhitespaceAndComments`,
`internal/component/config/tokenizer.go`) and REFUSES a text it cannot cut,
rather than answering empty.

Nothing the lint asserted was relaxed. `tunnelEndpointClaims` reads the same
four values through the new API, and its blindness guard is stronger than
before: `remote/ip` is `mandatory true`, and the reader can now say "this block
is not there" separately from "this block declares nothing", which is the
distinction that guard turns on.

The lint the file exists for, `TestCITunnelEndpointsAreUnique`, is untouched and
passes. The replacement reader gained its own tests
(`internal/test/runner/ci_config_read_test.go`), a second caller, and a corpus
gate: `TestPeerASDerivationReachesEveryCIFile` runs it over every `.ci` in the
tree.

| Test | Reason |
|------|--------|
| namedBlocks | Moved out of `internal/test/runner/tunnel_endpoint_lint_test.go` in this commit and replaced by `(configFile).blocks` in `internal/test/runner/ci_config_read.go`, which finds every block the old matcher found plus those written with a tab between the keyword and the name. |
| firstBlock | Same move, replaced by `(configFile).topLevel`, which additionally scopes the lookup to the block's own level so a container never reads a nested block's declarations as its own. |
| leafInBlock | Same move, replaced by `(configFile).blockIP`, dropping a `name` parameter every one of its three call sites passed `"ip"`. It now reports whether the leaf was found, which the tunnel lint needs because `remote/ip` is `mandatory true`. |
| leaf | Same move, replaced by `(configFile).leaf`, which reports found/not-found rather than `""` and is scoped to the block's own level. |
| atTokenStart | Same move. Its job is now done by tokenization: after the text is cut into tokens there is no "matched inside another word" case left to guard against. |
| braceBody | Same move. Its job is now done by `(configFile).blockAt` over tokens, which cannot be unbalanced by a brace inside a `#` comment, because the tokenizer consumes the comment first. |
