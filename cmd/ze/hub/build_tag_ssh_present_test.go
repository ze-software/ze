// Design: ai/rules/plugins.md -- ze_ssh present build validation
//
//go:build ze_ssh

package hub

// VALIDATES: with the ze_ssh build tag (the default ze / ze-appliance feature
// set), the ssh compile-out seam is installed (build + wire + standalone).
// PREVENTS: a regression where ze_ssh is set but ssh is not wired (e.g. the
// register_ssh.go init() is dropped or the tag stops reaching the generator).

import "testing"

func TestBuildTag_SSH_Present(t *testing.T) {
	if sshBuild == nil || sshWirePostStart == nil || sshBuildStandalone == nil {
		t.Fatal("ze_ssh build: ssh seam not installed (sshBuild/sshWirePostStart/sshBuildStandalone)")
	}
}
