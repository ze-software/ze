# ExaBGP 4.2 and 5.0 recordings

Copied from ExaBGP at commit `685f772e4ede5c818d2ff5c84f4ae040afd37be6`
(github.com/Exa-Networks/exabgp, 2026-10-14). The ExaBGP main pin of the
`exabgp-test` stage (`internal/le/test/functional/exabgp.go`) is now
`f9bfb853b924e97deba646b0bd50693ab07c4f3e`, whose `qa/old/` is
unchanged from that commit (`git diff --stat 685f772e f9bfb853b -- qa/old` is
empty), so these copies are the pin's. Nothing here is edited.

| File | Source at the pin | Read by |
|------|-------------------|---------|
| `commands-5.0.json.gz` | `qa/old/commands-5.0.json`, gzipped here (`gzip -9n`) | `TestOldCommands50` |
| `commands-4.2.json.gz` | `qa/old/commands-4.2.json`, gzipped here | `TestOldCommands42` |
| `responses-5.0.json.gz` | `qa/old/responses-5.0.json.gz`, as is | `TestOldResponses50` |
| `responses-4.2.json.gz` | `qa/old/responses-4.2.json.gz`, as is | `TestOldResponses42` |
| `configs-5.0.json.gz` | `qa/old/configs-5.0.json`, gzipped here | `TestOldResponses50`: the session of each configuration, its first neighbor as 5.0 named it and the OPEN it sent |
| `configs-4.2.json.gz` | `qa/old/configs-4.2.json`, gzipped here | `TestOldResponses42`, as above |
| `neighbor.conf`, `neighbor-4.2.conf` | `qa/old/neighbor.conf`, `qa/old/neighbor-4.2.conf` | the session the recordings were made against |

ExaBGP's `qa/old/recorder.py` made them by running each command and
configuration inside a 4.2 or 5.0 tree; `qa/old/README.md` at the pin says how.
To take them again:

    E=<exabgp checkout>; P=<new pin>
    git -C $E show $P:qa/old/commands-5.0.json | gzip -9n > commands-5.0.json.gz

ExaBGP is under the BSD 3-clause licence, which asks that the copyright notice
be kept with a redistribution: `LICENCE-ExaBGP.txt` is the pin's `LICENCE.txt`.
