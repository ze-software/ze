# Independent test setup judgment

Unit: `qemu.TestApplianceConfigPushAgentOwnsWorkers`

APPROVE (a): Reusing one SSH agent client on the same accepted connection preserves every input and assertion, including the real signature and identity checks and the refusal after stop, while removing competing background protocol readers introduced by repeated client construction.

The test is absent from HEAD (`git show HEAD:internal/le/test/qemu/install_config_push_agent_test.go` reports that the path is not in HEAD). The reviewed old baseline is the current untracked working-tree test before the proposed repair. The exact proposal is the parent's three substitutions and one local client declaration, retained as `single-client.patch`; it has not been applied.

Preserved proof:

- An actual installer implements `configPushIdentityStarter`, generates a random Ed25519 identity, and starts the real private agent.
- Home and socket stay beneath the owned scratch directory; home, `.ssh`, and socket have no group or other permission bits.
- The accepted Unix socket lists exactly one key, obtains a signer, signs `owned native proof`, verifies that signature, and serves exactly the generated public key.
- Stop must complete without error within the existing two-second bound while the same accepted connection is idle, closing and joining its worker.
- The same connection must reject an agent List after stop, the agent socket must disappear, and HOME and SSH_AUTH_SOCK must remain unchanged.
- The stop and connection cleanup registrations remain unchanged.

Producer evidence: `configPushIdentity` creates a keyring containing the supplied identity and starts `configPushAgent.accept`; `accept` records each connection before starting `serve`; `serve` runs `agent.ServeAgent`; `close` closes the listener and all recorded connections before waiting for all workers. No producer code changes are proposed.

Vendor evidence: `agent.NewClient` selects `newPipeline` for a connection implementing `io.ReadWriteCloser`; `newPipeline` immediately starts `readLoop`. Each pipeline has its own pending queue, while each reader consumes the same connection. A first client's reader can therefore consume a later client's response and wait on its own empty pending queue. `Signers` also calls `List` and binds each signer to its client. One client preserves the real requests and accepted-connection lifecycle while giving the connection one response reader.

No RFC requirement tag exists in this unit. No tests, builds, formatters, gopls commands, project-native commands, or commits were run by this reviewer. Root subsequently reported actual job `b1cbc93e` timing out after ten minutes with this test blocked in `Signers`: the first pipeline reader waits on its empty pending queue, while the second pipeline call waits for its reply and its reader waits for network bytes. That reported stack matches the source diagnosis. This reviewer did not execute or independently inspect that run; its result remains root-observed evidence, and no successful repaired test or full QEMU package run is claimed.

Root owns recording this independent source judgment before editing, under the higher-priority main-only serialized native-command rule. Root reported two import-path spellings and then package-clause spelling `testqemu` were refused without approval; actual native writer `51170` explicitly identified directory-basename spelling `qemu.TestApplianceConfigPushAgentOwnsWorkers`. Corrected writer `5388` is running; this reviewer claims no completed approval until that writer actually succeeds:

`./le test weakened approve unit qemu.TestApplianceConfigPushAgentOwnsWorkers reason "APPROVE (a): Reusing one SSH agent client on the same accepted connection preserves every input and assertion, including the real signature and identity checks and the refusal after stop, while removing competing background protocol readers introduced by repeated client construction."`
