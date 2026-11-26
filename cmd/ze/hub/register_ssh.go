// Design: docs/architecture/hub-architecture.md -- infrastructure server setup
//
// Installs the ssh build + wire implementations into the compile-out seam
// (ssh_infra.go). Compiled only under //go:build ze_ssh; absent the tag this
// init() does not run, the seam stays nil, and the ssh server is dropped from
// the binary (along with service_ssh.go and session_factory.go).

//go:build ze_ssh

package hub

import zeconfig "github.com/ze-software/ze/internal/component/config"

func init() {
	if err := zeconfig.RegisterServiceBlock("environment/ssh", "SSH server"); err != nil {
		panic("BUG: hub-ssh: service block registration: " + err.Error())
	}
	sshBuild = sshBuildImpl
	sshWirePostStart = sshWireImpl
	sshBuildStandalone = sshBuildStandaloneImpl
}
