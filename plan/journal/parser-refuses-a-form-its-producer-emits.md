# parser refuses a form its producer emits

A reader of a tool-generated file accepts only the shapes it has met so far. The
producer emits a legitimate shape the reader never saw, and the reader refuses
it, so a change elsewhere (a dependency pin, a flag) breaks a path that reads
nothing the change touched.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-10-09 | spec-ci-parser-refuses-an-assertion-key-it-does-not-read | `parseVendorModules` (`internal/appliance/instance/vendor.go`), reading `vendor/modules.txt` | Since `66e87707f5` replaces goyang, `modules.txt` has the five-field `# mod v => mod v` header; the parser takes three fields, so every appliance image build fails `unsupported module declaration` (`vpp-hugepages-qemu.ci` red with KVM) | FIXED in "appliance: bind replaced modules from vendor/modules.txt": every `moduleLine` shape parsed, only package-providing records bound |
