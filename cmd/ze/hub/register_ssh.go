// Design: docs/architecture/hub-architecture.md -- infrastructure server setup
//
// Installs the ssh build + wire implementations into the compile-out seam
// (ssh_infra.go). Compiled only under //go:build ze_ssh; absent the tag this
// init() does not run, the seam stays nil, and the ssh server is dropped from
// the binary (along with service_ssh.go and session_factory.go).

//go:build ze_ssh

package hub

func init() {
	sshBuild = sshBuildImpl
	sshWirePostStart = sshWireImpl
	sshBuildStandalone = sshBuildStandaloneImpl
}
