| Test | Reason |
|---|---|
| bgp-srpolicy-bsid-srv6-decode | Nothing left the suite. The file could not be parsed by the decode suite, which runs one cmd= and one expect=json: per file, so it ran nothing. Its second case (Length 20 shown malformed) moved to bgp-srpolicy-bsid-srv6-long-decode.ci, and each case's two expectations (exit 0, a stdout substring) became one full expect=json: of the decode output, which holds the same sub-TLV object and asserts every other field too. |
