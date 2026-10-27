# A command name breaks the CLI grammar

A new command is named on its own, without reading the names already beside it.
Two siblings that share a left segment before a hyphen turn that segment into a
namespace, and `./le cli grammar` R9 then asks for the hyphen to become a token
boundary. The name ships because the grammar gate is run by nobody who adds a
command.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-10-24 | spec-docker-hosts-run-the-ze-kernel | `le setup docker-host` (`internal/le/setup/dockerhost/actions.go`), `le setup docker-kernel` (`internal/le/setup/dockerkernel/dockerkernel.go`) | `./le cli grammar` exits 1 with two R9 findings: both roots share the left segment `setup docker`, which `leNamespaces` (`internal/le/cli/grammar/cligrammar.go`) makes a namespace, so each is told to split. docker-kernel landed in 4bc6b410dd, docker-host in 22e1b9e8e7 | fixed: b209677569 split both into `setup docker host` and `kernel`; three-word dispatch (D-P4-depth3) serves them |
| 2026-10-27 | spec-add-path-limit-send-receive (verify changed triage at e6474aa200) | `le site terminal-demo readme-carrier` (`791c21c9ee`), `le test deployment vpn-kernel-test` and `le test qemu vpn-kernel-test` (`22f782e1c2`) | `TestEveryActionWordIsAVerbOrADeclaringName` and `TestNoNameEmbedsAVerbOrTest` (`internal/le/cli/grammar`) refuse the three: none is a declared verb or name, and two embed `test` | open: rename into the verb vocabulary of `docs/contributing/running-commands.md` |
| 2026-10-27 | spec-add-path-limit-send-receive (verify changed triage at e6474aa200) | `ze appliance kernel --jobs` (`internal/appliance/cmd_kernel.go`, `b7e2ace8ed`) | `TestTheRealCheckoutPassesAndWasRead` grammar F4: the flag is parsed but never declared through `registry.RegisterCommandFlags`, so completion does not offer it | open: declare the flag beside its parse |
