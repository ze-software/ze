# OTA push through the gokrazy updater

`ze appliance push` streams a built image to a running device and switches its
A/B root partition. It speaks the gokrazy update protocol through the
`github.com/gokrazy/updater` module rather than a raw HTTP PUT.

<!-- source: internal/appliance/cmd_push.go -- runPush, pushOne, doPushUpdater, authTransport -->

## Decisions

| Decision | Reason |
|----------|--------|
| Authentication is injected as an `http.RoundTripper` wrapper | the updater makes its own requests, a feature probe among them. Credentials on the stream call alone would fail the probe |
| TLS verifies against the stored `cert.pem` only | devices carry self-signed certificates, so the system CA pool cannot help |
| HTTP basic auth with an empty user and the token as the password | this is the scheme the gokrazy update API expects |
| `--testboot` uses the updater's testboot path rather than switch | the device reverts on its own when the new root fails to boot |
| `--no-reboot` streams and switches without rebooting | batch updates decide their own reboot order |
| Updater errors are mapped back onto the existing protocol-error type | the "unreachable versus protocol error" distinction survives the library change |
| The push itself is an injectable function variable | tests drive the full protocol against a mock, with no device |

## Trap

The push mock has to speak the whole protocol, not just the upload: features,
root, switch, testboot, and reboot. A mock that answers only the stream endpoint
passes while the feature probe is broken, because the probe is a separate
request.

## Related

- `remote-operations.md` for fleet-wide push and the parallel worker pool
- `self-update.md` for the device pulling its own update instead
