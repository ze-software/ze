# parser refuses a form its producer emits

A reader of a tool-generated file accepts only the shapes it has met so far. The
producer emits a legitimate shape the reader never saw, and the reader refuses
it, so a change elsewhere (a dependency pin, a flag) breaks a path that reads
nothing the change touched.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-10-14 | spec-bgp-sr-policy-rfc-defects (design review r3 B-1) | ExaBGP bridge `convertAnnounceSRPolicy` (`internal/exabgp/bridge/bridge_command.go`) -> `parseSRPolicySection` (`plugins/cmd/update/update_text_srpolicy.go`) | The bridge appends ExaBGP's leftover tokens (`preference 100` ...) to `nlri ipv4/sr-policy add`; the parser takes only distinguisher/color/endpoint and refuses the rest (`sr-policy: unknown keyword`), so any announce carrying an attribute fails | open: owned by spec-api-sr-policy-attributes (owner, 2026-10-14) |
| 2026-10-13 | - (enum-refusal design review r13) | session change file: `writeThroughSet` -> `serializeSetMetaExtraValues` -> `ParseChangeFile` (`walkAndSet`) | Session `set <list> <key> <key-leaf> <v>` stores the key leaf as an extra value, serialized as a line `walkAndSet` refuses (`unknown field`), so later write-throughs and commits fail "does not parse". Serialize+parse probed, chain inferred. Also reached, unverified: web CLI bar (`handleCLISet`), API config sessions. | open: spec-config-enum-refusal fixes the SSH editor and web terminal halves (refused at set). |
| 2026-10-09 | spec-ci-parser-refuses-an-assertion-key-it-does-not-read | `parseVendorModules` (`internal/appliance/instance/vendor.go`), reading `vendor/modules.txt` | Since `66e87707f5` replaces goyang, `modules.txt` has the five-field `# mod v => mod v` header; the parser takes three fields, so every appliance image build fails `unsupported module declaration` (`vpp-hugepages-qemu.ci` red with KVM) | FIXED in "appliance: bind replaced modules from vendor/modules.txt": every `moduleLine` shape parsed, only package-providing records bound |
