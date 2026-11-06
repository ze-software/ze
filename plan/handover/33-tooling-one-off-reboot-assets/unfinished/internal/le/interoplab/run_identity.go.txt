// Design: docs/architecture/testing/interop.md -- inherited launch ownership.
// Related: orphans.go -- processIdentity and CleanupOwnedOrphans.
package interoplab

import (
	"errors"

	"github.com/ze-software/ze/internal/core/env"
)

const runIdentityKey = labelLaunch
const runIdentityVariable = "ZE_LAB_LAUNCH"

var _ = env.MustRegister(env.EnvEntry{
	Key:         runIdentityKey,
	Type:        "string",
	Default:     "",
	Private:     true,
	Description: "internal launch identity inherited by test children for their launcher's cleanup",
})

// launchIdentity retains the root launcher's id through descendants. The
// existing run id stays unique per process for per-process resource cleanup.
func (o ownerIdentity) launchIdentity() string {
	if o.launch != "" {
		return o.launch
	}
	return o.run
}

// ChildEnvironment passes this client's launch identity to the child it launches.
// The caller MUST pass this environment to its child and MUST use this same
// Docker client for CleanupOwnedOrphans after the child has exited.
func (d *Docker) ChildEnvironment(environ []string) ([]string, error) {
	launch := d.owner.launchIdentity()
	if launch == "" {
		return nil, errors.New("the Docker client has no launch identity, so child resources cannot be attributed. Fix: create the client with NewDocker before launching tests")
	}
	return append(env.Without(environ, runIdentityKey), runIdentityVariable+"="+launch), nil
}
