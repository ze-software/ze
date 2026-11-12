# OTA push through the gokrazy updater

`ze appliance push` reads a built GPT disk image, streams its root filesystem
to the running device's inactive root partition, updates the shared boot
filesystem and boot code, then switches the root partition. It speaks the gokrazy update protocol through the
`github.com/gokrazy/updater` module rather than a raw HTTP PUT.

<!-- source: internal/appliance/cmd_push.go -- runPush, pushOne, doPushUpdater, authTransport -->

## Decisions

| Decision | Reason |
|----------|--------|
| Authentication is injected as an `http.RoundTripper` wrapper | the updater makes its own requests, a feature probe among them. Credentials on the stream call alone would fail the probe |
| TLS verifies against the stored `cert.pem` only | no public authority signed the device certificate, so the system CA pool cannot help. The file holds the device certificate and the appliance root that issued it, and the pool trusts the ISSUER, so a certificate reissued by `ze appliance replace-cert` still verifies |
| HTTP basic auth with user `gokrazy` and the stored token as the password | the pinned gokrazy authentication handler requires that username |
| Validate GPT checksums, partition bounds, FAT and SquashFS lengths before contacting the device | a full disk image is not a root filesystem; malformed or truncated input must not start an update |
| Stream bounded root, `bootonly`, and MBR boot-code payloads in that order | root goes to the inactive slot; `bootonly` rewrites both command-line files with the running device's active root identity before switch or testboot |
| Write only the first 440 MBR bytes | boot code must match the new FAT layout; the target disk signature, partition table, GPT and persistent partition must remain intact |
| Refuse HTTP redirects and bound the update to 30 minutes | update credentials must not travel to a redirected destination, and an unreachable target must not wait forever |
| `--testboot` uses the updater's testboot path rather than switch | the device reverts on its own when the new root fails to boot |
| `--no-reboot` streams and switches without rebooting | batch updates decide their own reboot order |
| Updater errors are mapped back onto the existing protocol-error type | the "unreachable versus protocol error" distinction survives the library change |
| The push itself is an injectable function variable | tests drive the full protocol against a mock, with no device |

The A/B guarantee applies to the root filesystem. The boot filesystem is shared
by both root slots, so an interrupted boot write still requires recovery media.
`--testboot` tests the inactive root before making it permanent; it does not
create a second boot partition. Keep a known-good installer image and a backup
of the appliance's configuration and secrets.
<!-- source: internal/appliance/push_image.go -- readPushImage, readPushPartitions, readPushBoot, readPushRoot -->
<!-- source: internal/appliance/cmd_push.go -- doPushUpdater, refusePushRedirect -->

## Trap

The reboot response acknowledges an accepted request before supervised services
stop. Ze can own the management interface, so waiting to send the response until
after Ze stops can leave the client waiting on a connection the device has lost.
The device still completes synchronous service shutdown and unmounts persistent
storage before it performs the power operation. A successful push reports reboot
acceptance, not a healthy restarted device. Verify management access, the running
version and the retained configuration after restart. The QEMU OTA proof checks
fresh authenticated status and SSH before it reports the updated runtime.
<!-- source: gokrazy/modcache/github.com/gokrazy/gokrazy@v0.0.0-20260703061218-a4a45a20149d/update.go -- powerHandlerWithShutdown -->
<!-- source: internal/appliance/cmd_push.go -- doPushUpdater -->
<!-- source: internal/le/test/qemu/install_ota.go -- executeApplianceOTA, await -->


The push mock has to speak the whole protocol, not just the upload: features,
root, bootonly, mbr, switch, testboot, and reboot. A mock that answers only the stream endpoint
passes while the feature probe is broken, because the probe is a separate
request.

Named image builds put the same update token into gokrazy's prepared `Update`
configuration that `pushOne` reads on the host. TLS material is written to
private temporary files and selected through `Update.UseTLS`, the upstream
consumer's custom certificate path. The prepared configuration and TLS files
have mode `0600` and are removed when the builder returns. Secrets
are not command arguments and the checked-in instance is not changed.
<!-- source: internal/appliance/build_update.go -- readGokUpdate, writeGokUpdate -->
<!-- source: internal/appliance/cmd_build.go -- buildOne, runGokBuild -->

## Related

- `remote-operations.md` for fleet-wide push and the parallel worker pool
- `self-update.md` for the device pulling its own update instead
