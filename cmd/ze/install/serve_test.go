package install

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRemoteConfigGeneration(t *testing.T) {
	cfg := generateConfig(configParams{
		iface:       "eth0",
		network:     "10.0.0.0/24",
		image:       "/images/gokrazy.img",
		serverIP:    "10.0.0.1",
		sshUsername: "admin",
		sshPassHash: "$2a$10$testhash",
	})

	assert.Contains(t, cfg, "service {")
	assert.Contains(t, cfg, "dhcp-server {")
	assert.Contains(t, cfg, "listen-interface eth0;")
	assert.Contains(t, cfg, "subnet 10.0.0.0/24 {")
	assert.Contains(t, cfg, "pxe {")
	assert.Contains(t, cfg, "tftp-server {")
	assert.Contains(t, cfg, "image-server {")
	assert.Contains(t, cfg, "default-router 10.0.0.1;")

	// NUL sentinel is written by forkAndServe, not generateConfig.
	// Verify config does not contain NUL.
	assert.NotContains(t, cfg, "\x00")
}

func TestRunRemote_MissingFlags(t *testing.T) {
	code := runRemote([]string{})
	assert.Equal(t, 1, code, "should fail with no flags")
}

func TestRunRemote_Help(t *testing.T) {
	code := runRemote([]string{"-h"})
	assert.Equal(t, 0, code, "help should exit 0")
}
