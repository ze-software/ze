// Design: docs/architecture/appliance/build-artifacts.md -- the runtime kernel
// doctor check asks which builder this host would use.
// Related: driver.go -- selectBuilder, the one backend choice.

package kernelbuilder

// DoctorSourceName is this file's name. Nothing in it runs during a kernel
// build, so the kernel cache variant leaves it out of the builder source hash
// (kernelBuilderSources in internal/appliance/cache.go): an edit here must not
// make every cached kernel stale and cost a cold rebuild.
const DoctorSourceName = "doctor.go"

// UsableBuilder answers the backend a kernel build for arch would use when no
// backend is named (Docker first, then QEMU with Go), or an error when this
// host has neither. It runs nothing: `ze doctor` asks it before any build.
func UsableBuilder(arch string) (string, error) {
	return selectBuilder("", arch)
}
