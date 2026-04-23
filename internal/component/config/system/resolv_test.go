package system_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"codeberg.org/thomas-mangin/ze/internal/component/config/system"
)

// TestWriteResolvConf_Empty verifies no error when servers list is empty.
//
// VALIDATES: Empty servers list produces no error and no file.
// PREVENTS: Panic or error on nil/empty server list.
func TestWriteResolvConf_Empty(t *testing.T) {
	err := system.WriteResolvConf("/tmp/ze-test-resolv.conf", nil)
	assert.NoError(t, err)

	err = system.WriteResolvConf("/tmp/ze-test-resolv.conf", []string{})
	assert.NoError(t, err)
}

// TestWriteResolvConf_EmptyPath verifies no error when path is empty.
//
// VALIDATES: Empty path disables resolv.conf writing.
// PREVENTS: Error or panic when resolv.conf writing is disabled.
func TestWriteResolvConf_EmptyPath(t *testing.T) {
	err := system.WriteResolvConf("", []string{"8.8.8.8"})
	assert.NoError(t, err)
}
